package cli

import (
	"context"
	"fmt"

	"github.com/incidentflow/incidentflow-cli/internal/api"
	"github.com/incidentflow/incidentflow-cli/internal/config"
	"github.com/incidentflow/incidentflow-cli/internal/helm"
	"github.com/incidentflow/incidentflow-cli/internal/install"
	"github.com/incidentflow/incidentflow-cli/internal/kube"
	"github.com/incidentflow/incidentflow-cli/internal/output"
	"github.com/spf13/cobra"
)

var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Manage Kubernetes clusters",
}

var clusterInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install IncidentFlow Agent into a Kubernetes cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		flags := cmd.Flags()
		name, _ := flags.GetString("name")
		displayName, _ := flags.GetString("display-name")
		namespace, _ := flags.GetString("namespace")
		platformURL, _ := flags.GetString("platform-url")
		gatewayURL, _ := flags.GetString("gateway-url")
		chartRef, _ := flags.GetString("chart")
		chartVersion, _ := flags.GetString("chart-version")
		releaseName, _ := flags.GetString("release")
		yes, _ := flags.GetBool("yes")
		dryRun, _ := flags.GetBool("dry-run")
		forceDiff, _ := flags.GetBool("diff")
		noDiff, _ := flags.GetBool("no-diff")
		printCommand, _ := flags.GetBool("print-command")
		jsonOut, _ := flags.GetBool("json")

		if jsonOut {
			output.SetJSON(true)
		}

		cfg, err := requireAuth()
		if err != nil {
			return err
		}

		if platformURL == "" {
			platformURL = cfg.APIURL
		}

		kubeClient, err := kube.NewClient()
		if err != nil {
			return err
		}

		helmClient, err := helm.NewClient()
		if err != nil {
			return err
		}

		installer := &install.Installer{
			API:  api.NewClient(cfg.APIURL, cfg.Token, cfg.Workspace),
			Kube: kubeClient,
			Helm: helmClient,
		}

		result, err := installer.InstallCluster(context.Background(), install.Options{
			ClusterName:  name,
			DisplayName:  displayName,
			Namespace:    namespace,
			PlatformURL:  platformURL,
			GatewayURL:   gatewayURL,
			ChartRef:     chartRef,
			ChartVersion: chartVersion,
			ReleaseName:  releaseName,
			Yes:          yes,
			DryRun:       dryRun,
			ForceDiff:    forceDiff,
			NoDiff:       noDiff,
			PrintCommand: printCommand,
			ConfigEnv:    cfg.Env,
			ConfigAppURL: cfg.AppURL,
		})
		if err != nil {
			return err
		}

		if result != nil && output.IsJSON() {
			return output.JSON(result)
		}
		return nil
	},
}

var clusterStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of registered clusters",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")

		cfg, err := requireAuth()
		if err != nil {
			return err
		}

		client := api.NewClient(cfg.APIURL, cfg.Token, cfg.Workspace)

		if name != "" {
			status, err := client.GetAgentStatus(context.Background(), name)
			if err != nil {
				return err
			}
			printAgentStatus(status)
			return nil
		}

		agents, err := client.ListAgents(context.Background())
		if err != nil {
			return err
		}

		if len(agents) == 0 {
			output.Info("No clusters registered.\n\nRun:\n  incidentflow cluster install --name <name>")
			return nil
		}

		if output.IsJSON() {
			return output.JSON(agents)
		}

		for _, a := range agents {
			printAgentStatus(&a)
			fmt.Println()
		}
		return nil
	},
}

var clusterUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall IncidentFlow Agent from a cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		namespace, _ := cmd.Flags().GetString("namespace")
		releaseName, _ := cmd.Flags().GetString("release")
		revoke, _ := cmd.Flags().GetBool("revoke-credentials")

		if name == "" {
			return fmt.Errorf("--name is required")
		}
		if namespace == "" {
			namespace = config.DefaultNamespace
		}
		if releaseName == "" {
			releaseName = config.DefaultRelease
		}

		cfg, err := requireAuth()
		if err != nil {
			return err
		}

		helmClient, err := helm.NewClient()
		if err != nil {
			return err
		}

		output.Header(fmt.Sprintf("Uninstalling IncidentFlow Agent from %s", name))

		if err := helmClient.Uninstall(context.Background(), releaseName, namespace); err != nil {
			return err
		}
		output.Success("Helm release removed")

		if revoke {
			apiClient := api.NewClient(cfg.APIURL, cfg.Token, cfg.Workspace)
			if err := apiClient.DeleteAgent(context.Background(), name); err != nil {
				output.Error(fmt.Sprintf("Could not deregister cluster: %s", err))
			} else {
				output.Success("Cluster deregistered")
			}
		}

		output.Info(fmt.Sprintf("\nCluster %s uninstalled.", name))
		return nil
	},
}

var clusterDiagnoseCmd = &cobra.Command{
	Use:   "diagnose",
	Short: "Run diagnostics on a cluster agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		namespace, _ := cmd.Flags().GetString("namespace")
		releaseName, _ := cmd.Flags().GetString("release")

		if name == "" {
			return fmt.Errorf("--name is required")
		}

		cfg, err := requireAuth()
		if err != nil {
			return err
		}

		kubeClient, err := kube.NewClient()
		if err != nil {
			return err
		}

		helmClient, err := helm.NewClient()
		if err != nil {
			return err
		}

		installer := &install.Installer{
			API:  api.NewClient(cfg.APIURL, cfg.Token, cfg.Workspace),
			Kube: kubeClient,
			Helm: helmClient,
		}

		return installer.Diagnose(context.Background(), install.DiagnosticsOptions{
			ClusterName: name,
			Namespace:   namespace,
			ReleaseName: releaseName,
		})
	},
}

func init() {
	clusterCmd.AddCommand(clusterInstallCmd)
	clusterCmd.AddCommand(clusterStatusCmd)
	clusterCmd.AddCommand(clusterUninstallCmd)
	clusterCmd.AddCommand(clusterDiagnoseCmd)

	clusterInstallCmd.Flags().String("name", "", "Cluster name (required)")
	clusterInstallCmd.Flags().String("display-name", "", "Human-readable display name")
	clusterInstallCmd.Flags().String("namespace", config.DefaultNamespace, "Kubernetes namespace")
	clusterInstallCmd.Flags().String("release", config.DefaultRelease, "Helm release name")
	clusterInstallCmd.Flags().String("platform-url", "", "IncidentFlow platform API URL")
	clusterInstallCmd.Flags().String("gateway-url", config.DefaultGatewayURL, "IncidentFlow gateway WebSocket URL")
	clusterInstallCmd.Flags().String("chart", config.DefaultChartRef, "Helm chart reference")
	clusterInstallCmd.Flags().String("chart-version", "", "Helm chart version (default: latest)")
	clusterInstallCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt (for CI/automation/MCP)")
	clusterInstallCmd.Flags().Bool("dry-run", false, "Show plan and exit without making any changes")
	clusterInstallCmd.Flags().Bool("diff", false, "Force helm diff (error if helm-diff plugin is missing)")
	clusterInstallCmd.Flags().Bool("no-diff", false, "Skip helm diff even if plugin is installed")
	clusterInstallCmd.Flags().Bool("print-command", false, "Print equivalent helm command (token masked)")
	clusterInstallCmd.Flags().Bool("json", false, "Output JSON")
	_ = clusterInstallCmd.MarkFlagRequired("name")

	clusterStatusCmd.Flags().String("name", "", "Cluster name (optional, lists all if omitted)")

	clusterUninstallCmd.Flags().String("name", "", "Cluster name (required)")
	clusterUninstallCmd.Flags().String("namespace", config.DefaultNamespace, "Kubernetes namespace")
	clusterUninstallCmd.Flags().String("release", config.DefaultRelease, "Helm release name")
	clusterUninstallCmd.Flags().Bool("revoke-credentials", false, "Also deregister cluster from platform")
	_ = clusterUninstallCmd.MarkFlagRequired("name")

	clusterDiagnoseCmd.Flags().String("name", "", "Cluster name (required)")
	clusterDiagnoseCmd.Flags().String("namespace", config.DefaultNamespace, "Kubernetes namespace")
	clusterDiagnoseCmd.Flags().String("release", config.DefaultRelease, "Helm release name")
	_ = clusterDiagnoseCmd.MarkFlagRequired("name")
}

func requireAuth() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if !cfg.IsAuthenticated() {
		return nil, fmt.Errorf("not logged in. Run:\n  incidentflow login")
	}
	return cfg, nil
}

func printAgentStatus(a *api.AgentStatus) {
	if output.IsJSON() {
		_ = output.JSON(a)
		return
	}
	fmt.Printf("Cluster:        %s\n", a.ClusterName)
	fmt.Printf("Status:         %s\n", a.Status)
	if !a.LastHeartbeat.IsZero() {
		fmt.Printf("Last heartbeat: %s\n", output.FormatDuration(a.LastHeartbeat))
	}
	if a.AgentVersion != "" {
		fmt.Printf("Agent version:  %s\n", a.AgentVersion)
	}
	if a.Namespace != "" {
		fmt.Printf("Namespace:      %s\n", a.Namespace)
	}
}
