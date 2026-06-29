// Package mcp exposes IncidentFlow installer logic as MCP tool handlers.
// All cluster management tools call the same Go installer package used by the CLI —
// there is no duplicated installation logic here.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/incidentflow/incidentflow-cli/internal/api"
	"github.com/incidentflow/incidentflow-cli/internal/config"
	"github.com/incidentflow/incidentflow-cli/internal/helm"
	"github.com/incidentflow/incidentflow-cli/internal/install"
	"github.com/incidentflow/incidentflow-cli/internal/kube"
)

// ToolHandler is a generic MCP tool handler function.
type ToolHandler func(ctx context.Context, params json.RawMessage) (any, error)

// Registry returns all registered MCP tool handlers.
func Registry(cfg *config.Config) map[string]ToolHandler {
	return map[string]ToolHandler{
		"incidentflow.install_cluster":         installCluster(cfg),
		"incidentflow.cluster_status":          clusterStatus(cfg),
		"incidentflow.list_clusters":           listClusters(cfg),
		"incidentflow.uninstall_cluster":       uninstallCluster(cfg),
		"incidentflow.diagnose_cluster":        diagnoseCluster(cfg),
		"incidentflow.upgrade_agent":           upgradeAgent(cfg),
		"incidentflow.rotate_agent_credentials": rotateCredentials(cfg),
	}
}

func newInstaller(cfg *config.Config) (*install.Installer, error) {
	kubeClient, err := kube.NewClient()
	if err != nil {
		return nil, err
	}
	helmClient, err := helm.NewClient()
	if err != nil {
		return nil, err
	}
	return &install.Installer{
		API:  api.NewClient(cfg.APIURL, cfg.Token, cfg.Workspace),
		Kube: kubeClient,
		Helm: helmClient,
	}, nil
}

type installClusterParams struct {
	ClusterName string `json:"cluster_name"`
	DisplayName string `json:"display_name"`
	Namespace   string `json:"namespace"`
	Wait        bool   `json:"wait"`
}

func installCluster(cfg *config.Config) ToolHandler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		var p installClusterParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}

		installer, err := newInstaller(cfg)
		if err != nil {
			return nil, err
		}

		result, err := installer.InstallCluster(ctx, install.Options{
			ClusterName: p.ClusterName,
			DisplayName: p.DisplayName,
			Namespace:   p.Namespace,
			Yes:         true, // MCP always skips confirmation; caller must obtain user approval first
		})
		if err != nil {
			return nil, err
		}

		return result, nil
	}
}

type clusterNameParams struct {
	ClusterName string `json:"cluster_name"`
}

func clusterStatus(cfg *config.Config) ToolHandler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		var p clusterNameParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}

		client := api.NewClient(cfg.APIURL, cfg.Token, cfg.Workspace)
		return client.GetAgentStatus(ctx, p.ClusterName)
	}
}

func listClusters(cfg *config.Config) ToolHandler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		client := api.NewClient(cfg.APIURL, cfg.Token, cfg.Workspace)
		return client.ListAgents(ctx)
	}
}

type uninstallParams struct {
	ClusterName       string `json:"cluster_name"`
	Namespace         string `json:"namespace"`
	RevokeCredentials bool   `json:"revoke_credentials"`
}

func uninstallCluster(cfg *config.Config) ToolHandler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		var p uninstallParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}

		namespace := p.Namespace
		if namespace == "" {
			namespace = config.DefaultNamespace
		}

		helmClient, err := helm.NewClient()
		if err != nil {
			return nil, err
		}

		if err := helmClient.Uninstall(ctx, config.DefaultRelease, namespace); err != nil {
			return nil, err
		}

		if p.RevokeCredentials {
			client := api.NewClient(cfg.APIURL, cfg.Token, cfg.Workspace)
			_ = client.DeleteAgent(ctx, p.ClusterName)
		}

		return map[string]string{"status": "uninstalled", "cluster_name": p.ClusterName}, nil
	}
}

func diagnoseCluster(cfg *config.Config) ToolHandler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		var p clusterNameParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}

		installer, err := newInstaller(cfg)
		if err != nil {
			return nil, err
		}

		// Diagnose outputs to stdout; for MCP we also return platform status
		_ = installer.Diagnose(ctx, install.DiagnosticsOptions{
			ClusterName: p.ClusterName,
		})

		client := api.NewClient(cfg.APIURL, cfg.Token, cfg.Workspace)
		return client.GetAgentStatus(ctx, p.ClusterName)
	}
}

func upgradeAgent(cfg *config.Config) ToolHandler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			ClusterName string `json:"cluster_name"`
			Namespace   string `json:"namespace"`
			ChartRef    string `json:"chart_ref"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}

		namespace := p.Namespace
		if namespace == "" {
			namespace = config.DefaultNamespace
		}
		chartRef := p.ChartRef
		if chartRef == "" {
			chartRef = config.DefaultChartRef
		}

		helmClient, err := helm.NewClient()
		if err != nil {
			return nil, err
		}

		if err := helmClient.Upgrade(ctx, helm.InstallOptions{
			ReleaseName: config.DefaultRelease,
			ChartRef:    chartRef,
			Namespace:   namespace,
		}); err != nil {
			return nil, err
		}

		return map[string]string{"status": "upgraded", "cluster_name": p.ClusterName}, nil
	}
}

func rotateCredentials(cfg *config.Config) ToolHandler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		var p clusterNameParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}

		client := api.NewClient(cfg.APIURL, cfg.Token, cfg.Workspace)

		tokenResp, err := client.CreateRegistrationToken(ctx, api.CreateTokenRequest{
			ClusterName: p.ClusterName,
		})
		if err != nil {
			return nil, err
		}

		return map[string]string{
			"cluster_name": p.ClusterName,
			"token_id":     tokenResp.TokenItem.ID,
			"status":       "rotated",
		}, nil
	}
}
