package install

import (
	"fmt"
	"strings"
)

// Plan describes what an installation will do before any mutations happen.
type Plan struct {
	Environment       EnvironmentInfo `json:"environment"`
	Kubernetes        KubernetesInfo  `json:"kubernetes"`
	Agent             AgentInfo       `json:"agent"`
	Resources         []Resource      `json:"resources"`
	HelmDiffAvailable bool            `json:"helm_diff_available"`
	HelmDiff          string          `json:"helm_diff,omitempty"`
}

type EnvironmentInfo struct {
	Env    string `json:"env"`
	AppURL string `json:"app_url"`
	APIURL string `json:"api_url"`
}

type KubernetesInfo struct {
	Context   string `json:"context"`
	Namespace string `json:"namespace"`
	Release   string `json:"release"`
}

type AgentInfo struct {
	ClusterName string `json:"cluster_name"`
	ChartRef    string `json:"chart"`
	Version     string `json:"version"`
	PlatformURL string `json:"platform_url"`
	GatewayURL  string `json:"gateway_url"`
}

type Resource struct {
	Action string `json:"action"` // "+" create, "~" update
	Kind   string `json:"kind"`
	Name   string `json:"name"`
}

func defaultResources(releaseName, namespace string) []Resource {
	return []Resource{
		{"+", "Namespace", namespace},
		{"+", "ServiceAccount", releaseName},
		{"+", "ConfigMap", releaseName + "-config"},
		{"+", "Secret", "incidentflow-agent-credentials"},
		{"+", "Deployment", releaseName},
		{"+", "ClusterRole", releaseName},
		{"+", "ClusterRoleBinding", releaseName},
	}
}

func (p *Plan) Print() {
	col := func(label, value string) {
		fmt.Printf("    %-16s %s\n", label, value)
	}

	fmt.Println()
	fmt.Println("Installation Plan")
	fmt.Println()

	fmt.Println("  Environment")
	col("Env:", p.Environment.Env)
	col("App URL:", p.Environment.AppURL)
	col("API URL:", p.Environment.APIURL)
	fmt.Println()

	fmt.Println("  Kubernetes")
	col("Context:", p.Kubernetes.Context)
	col("Namespace:", p.Kubernetes.Namespace)
	col("Release:", p.Kubernetes.Release)
	fmt.Println()

	fmt.Println("  Agent")
	col("Cluster name:", p.Agent.ClusterName)
	col("Chart:", p.Agent.ChartRef)
	col("Version:", p.Agent.Version)
	col("Platform URL:", p.Agent.PlatformURL)
	col("Gateway URL:", p.Agent.GatewayURL)
	fmt.Println()

	fmt.Println("  Resources that may be created or updated:")
	for _, r := range p.Resources {
		fmt.Printf("    \033[32m%s\033[0m %-24s %s\n", r.Action, r.Kind, r.Name)
	}
	fmt.Println()
}

func (p *Plan) PrintDiff() {
	fmt.Println("Helm Diff")
	fmt.Println()

	if !p.HelmDiffAvailable {
		fmt.Println("  helm-diff plugin not installed. Skipping detailed diff.")
		fmt.Println("  Install it with:")
		fmt.Println("    helm plugin install https://github.com/databus23/helm-diff")
		fmt.Println()
		return
	}

	if strings.TrimSpace(p.HelmDiff) == "" {
		fmt.Println("  No changes detected.")
		fmt.Println()
		return
	}

	for _, line := range strings.Split(strings.TrimRight(p.HelmDiff, "\n"), "\n") {
		fmt.Println("  " + line)
	}
	fmt.Println()
}
