package install

import (
	"github.com/incidentflow/incidentflow-cli/internal/config"
)

// Options controls the install process (single source of truth for CLI + MCP).
type Options struct {
	ClusterName    string
	DisplayName    string
	Namespace      string
	PlatformURL    string
	GatewayURL     string
	ChartRef       string
	ChartVersion   string
	ReleaseName    string
	TimeoutSeconds int

	// UX control
	Yes          bool // skip confirmation prompt (CI / MCP / --yes flag)
	DryRun       bool // Phase A+B only; no mutations, no token created
	ForceDiff    bool // require helm-diff plugin; error if missing
	NoDiff       bool // skip diff even if plugin is installed
	PrintCommand bool // print equivalent helm command with token masked

	// Populated from loaded config — set by caller before passing to InstallCluster.
	ConfigEnv    string
	ConfigAppURL string
}

// Result is returned after a successful install (JSON-serialisable).
type Result struct {
	ClusterName   string `json:"cluster_name"`
	Namespace     string `json:"namespace"`
	ReleaseName   string `json:"release_name"`
	TokenID       string `json:"token_id"`
	Connected     bool   `json:"connected"`
	LastHeartbeat string `json:"last_heartbeat,omitempty"`
	AgentVersion  string `json:"agent_version,omitempty"`
	Status        string `json:"status"`
}

func (o *Options) applyDefaults() {
	if o.Namespace == "" {
		o.Namespace = config.DefaultNamespace
	}
	if o.PlatformURL == "" {
		o.PlatformURL = config.DefaultAPIURL()
	}
	if o.GatewayURL == "" {
		o.GatewayURL = config.DefaultGatewayURL
	}
	if o.ChartRef == "" {
		o.ChartRef = config.DefaultChartRef
	}
	if o.ReleaseName == "" {
		o.ReleaseName = config.DefaultRelease
	}
	if o.TimeoutSeconds == 0 {
		o.TimeoutSeconds = 300
	}
	if o.DisplayName == "" {
		o.DisplayName = o.ClusterName
	}
	if o.ChartVersion == "" {
		o.ChartVersion = "latest"
	}
}
