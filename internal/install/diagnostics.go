package install

import (
	"context"
	"fmt"

	"github.com/incidentflow/incidentflow-cli/internal/output"
)

type DiagnosticsOptions struct {
	ClusterName string
	Namespace   string
	ReleaseName string
}

func (i *Installer) Diagnose(ctx context.Context, opts DiagnosticsOptions) error {
	if opts.Namespace == "" {
		opts.Namespace = "incidentflow-agent"
	}
	if opts.ReleaseName == "" {
		opts.ReleaseName = "incidentflow-k8s-agent"
	}

	output.Header(fmt.Sprintf("Diagnostics: %s", opts.ClusterName))

	// Helm release status
	output.Info("Helm release:")
	rs, err := i.Helm.Status(ctx, opts.ReleaseName, opts.Namespace)
	if err != nil {
		output.Error(fmt.Sprintf("  %s", err))
	} else {
		output.Step(fmt.Sprintf("  %s  %s  %s", rs.Name, rs.Namespace, rs.Status))
	}

	// Deployment status
	output.Info("\nDeployment:")
	status, err := i.Kube.GetDeploymentStatus(ctx, opts.Namespace, opts.ReleaseName)
	if err != nil {
		output.Error(fmt.Sprintf("  %s", err))
	} else {
		output.Step(fmt.Sprintf("  %s", status))
	}

	// Pod status
	output.Info("\nPods:")
	pods, err := i.Kube.GetPodStatus(ctx, opts.Namespace, "app.kubernetes.io/name=incidentflow-k8s-agent")
	if err != nil {
		output.Error(fmt.Sprintf("  %s", err))
	} else if len(pods) == 0 {
		output.Step("  no pods found")
	} else {
		for _, p := range pods {
			output.Step(p)
		}
	}

	// Warning events
	output.Info("\nWarning events:")
	events, err := i.Kube.GetEvents(ctx, opts.Namespace)
	if err != nil {
		output.Error(fmt.Sprintf("  %s", err))
	} else if len(events) == 0 {
		output.Step("  no warnings")
	} else {
		for _, e := range events {
			output.Step(e)
		}
	}

	// Platform API status
	output.Info("\nPlatform status:")
	agentStatus, err := i.API.GetAgentStatus(ctx, opts.ClusterName)
	if err != nil {
		output.Error(fmt.Sprintf("  %s", err))
	} else {
		output.Step(fmt.Sprintf("  Status: %s", agentStatus.Status))
		if !agentStatus.LastHeartbeat.IsZero() {
			output.Step(fmt.Sprintf("  Last heartbeat: %s", output.FormatDuration(agentStatus.LastHeartbeat)))
		}
		output.Step(fmt.Sprintf("  Agent version: %s", agentStatus.AgentVersion))
	}

	return nil
}
