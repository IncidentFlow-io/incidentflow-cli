package install

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/incidentflow/incidentflow-cli/internal/api"
	"github.com/incidentflow/incidentflow-cli/internal/helm"
	"github.com/incidentflow/incidentflow-cli/internal/kube"
	"github.com/incidentflow/incidentflow-cli/internal/output"
)

// APIClient is the subset of api.Client used by the installer.
type APIClient interface {
	CreateRegistrationToken(ctx context.Context, req api.CreateTokenRequest) (*api.CreateTokenResponse, error)
	GetAgentStatus(ctx context.Context, clusterName string) (*api.AgentStatus, error)
}

// KubeClient is the subset of kube.Client used by the installer.
type KubeClient interface {
	Ping(ctx context.Context) error
	CheckPermissions(ctx context.Context) error
	WaitForDeployment(ctx context.Context, namespace, name string, timeout time.Duration) error
	GetDeploymentStatus(ctx context.Context, namespace, name string) (string, error)
	GetPodStatus(ctx context.Context, namespace, labelSelector string) ([]string, error)
	GetEvents(ctx context.Context, namespace string) ([]string, error)
}

// HelmClientIface is the subset of helm.Client used by the installer.
type HelmClientIface interface {
	Version() (string, error)
	ShowChart(ctx context.Context, chartRef, version string) error
	HasDiffPlugin() bool
	Diff(ctx context.Context, opts helm.InstallOptions) (string, error)
	Upgrade(ctx context.Context, opts helm.InstallOptions) error
	Uninstall(ctx context.Context, releaseName, namespace string) error
	Status(ctx context.Context, releaseName, namespace string) (*helm.ReleaseStatus, error)
}

type Installer struct {
	API    APIClient
	Kube   KubeClient
	Helm   HelmClientIface
	stdin  io.Reader // overridden in tests; defaults to os.Stdin
}

// InstallCluster is the single source of truth for agent installation.
// Used by both the CLI and the MCP tool.
//
// Phases:
//
//	A — Preflight checks (always run, read-only)
//	B — Installation plan + optional helm diff (always run, read-only)
//	C — Confirmation prompt (skipped when opts.Yes or opts.DryRun)
//	D — Mutations: token creation, helm upgrade, rollout wait
func (i *Installer) InstallCluster(ctx context.Context, opts Options) (*Result, error) {
	opts.applyDefaults()

	if opts.ClusterName == "" {
		return nil, fmt.Errorf("--name is required")
	}

	output.Header("Installing IncidentFlow Agent")

	// ── Phase A: Preflight ──────────────────────────────────────────────────

	kubeCtx, err := kube.CurrentContext()
	if err != nil {
		return nil, err
	}
	output.Success(fmt.Sprintf("Kubernetes context detected: %s", kubeCtx))

	if err := i.Kube.Ping(ctx); err != nil {
		return nil, err
	}
	output.Success("Kubernetes API reachable")

	helmVer, err := i.Helm.Version()
	if err != nil {
		return nil, err
	}
	output.Success(fmt.Sprintf("Helm detected: %s", helmVer))

	if err := i.Kube.CheckPermissions(ctx); err != nil {
		return nil, err
	}
	output.Success("Permissions verified")

	if err := i.Helm.ShowChart(ctx, opts.ChartRef, opts.ChartVersion); err != nil {
		return nil, err
	}
	output.Success("Helm chart found")

	// ── Phase B: Plan ───────────────────────────────────────────────────────

	version := opts.ChartVersion
	if version == "" {
		version = "latest"
	}

	plan := &Plan{
		Environment: EnvironmentInfo{
			Env:    opts.ConfigEnv,
			AppURL: opts.ConfigAppURL,
			APIURL: opts.PlatformURL,
		},
		Kubernetes: KubernetesInfo{
			Context:   kubeCtx,
			Namespace: opts.Namespace,
			Release:   opts.ReleaseName,
		},
		Agent: AgentInfo{
			ClusterName: opts.ClusterName,
			ChartRef:    opts.ChartRef,
			Version:     version,
			PlatformURL: opts.PlatformURL,
			GatewayURL:  opts.GatewayURL,
		},
		Resources: defaultResources(opts.ReleaseName, opts.Namespace),
	}

	if !output.IsJSON() {
		plan.Print()
	}

	// Helm diff — uses a placeholder token so no real token is ever created here.
	if opts.ForceDiff && !i.Helm.HasDiffPlugin() {
		return nil, fmt.Errorf(
			"helm-diff plugin is not installed.\n\n" +
				"Install it with:\n  helm plugin install https://github.com/databus23/helm-diff",
		)
	}

	plan.HelmDiffAvailable = i.Helm.HasDiffPlugin()
	showDiff := !opts.NoDiff

	if showDiff {
		if plan.HelmDiffAvailable {
			diffOpts := helm.InstallOptions{
				ReleaseName: opts.ReleaseName,
				ChartRef:    opts.ChartRef,
				Version:     opts.ChartVersion,
				Namespace:   opts.Namespace,
				Set: map[string]string{
					"agent.clusterName":       opts.ClusterName,
					"agent.platformUrl":       opts.PlatformURL,
					"agent.gatewayUrl":        opts.GatewayURL,
					"agent.registrationToken": "***PLACEHOLDER***",
				},
			}
			if diffOut, err := i.Helm.Diff(ctx, diffOpts); err == nil {
				plan.HelmDiff = diffOut
			} else if !output.IsJSON() {
				output.Info(fmt.Sprintf("Warning: helm diff failed: %s\n", err))
			}
		}

		if !output.IsJSON() {
			plan.PrintDiff()
		}
	}

	// Print equivalent helm command with token masked.
	if opts.PrintCommand && !output.IsJSON() {
		printHelmCommand(opts)
	}

	// Dry run: output plan and exit before any mutations.
	if opts.DryRun {
		if output.IsJSON() {
			return nil, output.JSON(plan)
		}
		output.Info("Dry run complete. No resources were created or modified.")
		return nil, nil
	}

	// ── Phase C: Confirmation ───────────────────────────────────────────────

	if !opts.Yes && !output.IsJSON() {
		r := i.stdin
		if r == nil {
			r = os.Stdin
		}
		confirmed, err := promptConfirm(r, "Continue with installation? [y/N]: ")
		if err != nil {
			return nil, err
		}
		if !confirmed {
			fmt.Println()
			fmt.Println("Installation cancelled.")
			fmt.Println("No registration token was created.")
			fmt.Println("No Kubernetes resources were changed.")
			return nil, nil
		}
		fmt.Println()
	}

	// ── Phase D: Mutations ──────────────────────────────────────────────────

	tokenResp, err := i.API.CreateRegistrationToken(ctx, api.CreateTokenRequest{
		ClusterName: opts.ClusterName,
		Name:        opts.DisplayName,
	})
	if err != nil {
		return nil, err
	}
	output.Success("Registration token created")

	helmOpts := helm.InstallOptions{
		ReleaseName: opts.ReleaseName,
		ChartRef:    opts.ChartRef,
		Version:     opts.ChartVersion,
		Namespace:   opts.Namespace,
		Wait:        false, // we handle waiting ourselves below
		Set: map[string]string{
			"agent.clusterName":       opts.ClusterName,
			"agent.platformUrl":       opts.PlatformURL,
			"agent.gatewayUrl":        opts.GatewayURL,
			"agent.registrationToken": tokenResp.RegistrationToken,
		},
	}

	if err := i.Helm.Upgrade(ctx, helmOpts); err != nil {
		return nil, err
	}
	output.Success("Helm release installed")

	timeout := time.Duration(opts.TimeoutSeconds) * time.Second

	if err := i.Kube.WaitForDeployment(ctx, opts.Namespace, opts.ReleaseName, timeout); err != nil {
		return nil, fmt.Errorf(
			"agent was installed but deployment did not become ready.\n\nRun:\n  incidentflow cluster diagnose --name %s",
			opts.ClusterName,
		)
	}
	output.Success("Deployment available")

	agentStatus, err := i.pollUntilOnline(ctx, opts.ClusterName, timeout)
	if err != nil {
		return nil, fmt.Errorf(
			"agent was installed but did not become Online.\n\nRun:\n  incidentflow cluster diagnose --name %s",
			opts.ClusterName,
		)
	}
	output.Success("Agent registered")
	output.Success("Heartbeat received")

	output.Info(fmt.Sprintf("\nCluster %s is Online.", opts.ClusterName))

	return &Result{
		ClusterName:   opts.ClusterName,
		Namespace:     opts.Namespace,
		ReleaseName:   opts.ReleaseName,
		TokenID:       tokenResp.TokenItem.ID,
		Connected:     true,
		LastHeartbeat: agentStatus.LastHeartbeat.Format(time.RFC3339),
		AgentVersion:  agentStatus.AgentVersion,
		Status:        agentStatus.Status,
	}, nil
}

func promptConfirm(r io.Reader, prompt string) (bool, error) {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(r)
	if scanner.Scan() {
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		return answer == "y" || answer == "yes", nil
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	return false, nil
}

// printHelmCommand prints the equivalent helm command with the token masked.
// Never prints real tokens.
func printHelmCommand(opts Options) {
	fmt.Println()
	fmt.Println("Equivalent Helm command:")
	fmt.Println()
	fmt.Printf("  helm upgrade --install %s %s \\\n", opts.ReleaseName, opts.ChartRef)
	fmt.Printf("    --namespace %s \\\n", opts.Namespace)
	fmt.Printf("    --create-namespace \\\n")
	if opts.ChartVersion != "" && opts.ChartVersion != "latest" {
		fmt.Printf("    --version %s \\\n", opts.ChartVersion)
	}
	fmt.Printf("    --set agent.clusterName=%s \\\n", opts.ClusterName)
	fmt.Printf("    --set agent.platformUrl=%s \\\n", opts.PlatformURL)
	fmt.Printf("    --set agent.gatewayUrl=%s \\\n", opts.GatewayURL)
	fmt.Printf("    --set agent.registrationToken=<TOKEN_MASKED>\n")
	fmt.Println()
}

func (i *Installer) pollUntilOnline(ctx context.Context, clusterName string, timeout time.Duration) (*api.AgentStatus, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		status, err := i.API.GetAgentStatus(ctx, clusterName)
		if err == nil && status.Status == "online" {
			return status, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}

	return nil, fmt.Errorf("timeout waiting for cluster %s to become Online", clusterName)
}
