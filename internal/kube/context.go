package kube

import (
	"fmt"
	"os"

	"k8s.io/client-go/tools/clientcmd"
)

type ContextInfo struct {
	Name    string
	Server  string
	Current bool
}

func CurrentContext() (string, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	cfg, err := rules.Load()
	if err != nil {
		return "", fmt.Errorf("loading kubeconfig: %w", err)
	}

	if cfg.CurrentContext == "" {
		return "", fmt.Errorf(
			"no Kubernetes context found.\n\nRun:\n  kubectl config get-contexts\n  kubectl config use-context <context>",
		)
	}

	return cfg.CurrentContext, nil
}

func ListContexts() ([]ContextInfo, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	cfg, err := rules.Load()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig: %w", err)
	}

	var out []ContextInfo
	for name, ctx := range cfg.Contexts {
		cluster := cfg.Clusters[ctx.Cluster]
		server := ""
		if cluster != nil {
			server = cluster.Server
		}
		out = append(out, ContextInfo{
			Name:    name,
			Server:  server,
			Current: name == cfg.CurrentContext,
		})
	}
	return out, nil
}

func KubeconfigPath() string {
	if v := os.Getenv("KUBECONFIG"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return home + "/.kube/config"
}
