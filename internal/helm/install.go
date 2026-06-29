package helm

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type InstallOptions struct {
	ReleaseName string
	ChartRef    string
	Version     string // chart version; empty or "latest" means no --version flag
	Namespace   string
	Set         map[string]string
	Wait        bool
	Timeout     string
}

func (c *Client) Upgrade(ctx context.Context, opts InstallOptions) error {
	args := []string{
		"upgrade", "--install", opts.ReleaseName, opts.ChartRef,
		"--namespace", opts.Namespace,
		"--create-namespace",
	}

	if opts.Version != "" && opts.Version != "latest" {
		args = append(args, "--version", opts.Version)
	}

	for k, v := range opts.Set {
		args = append(args, "--set", fmt.Sprintf("%s=%s", k, v))
	}

	if opts.Wait {
		args = append(args, "--wait")
		if opts.Timeout != "" {
			args = append(args, "--timeout", opts.Timeout)
		}
	}

	cmd := exec.CommandContext(ctx, c.helmBin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("helm upgrade failed:\n%s", string(out))
	}
	return nil
}

func (c *Client) Uninstall(ctx context.Context, releaseName, namespace string) error {
	cmd := exec.CommandContext(ctx, c.helmBin,
		"uninstall", releaseName, "--namespace", namespace,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("helm uninstall failed:\n%s", string(out))
	}
	return nil
}

type ReleaseStatus struct {
	Name      string
	Namespace string
	Status    string
	Chart     string
	Revision  string
}

func (c *Client) Status(ctx context.Context, releaseName, namespace string) (*ReleaseStatus, error) {
	cmd := exec.CommandContext(ctx, c.helmBin,
		"status", releaseName, "--namespace", namespace, "--output", "json",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("helm release %q not found in namespace %q", releaseName, namespace)
	}

	// Parse minimal fields without importing helm SDK
	status := string(out)
	rs := &ReleaseStatus{
		Name:      releaseName,
		Namespace: namespace,
	}

	if v := extractJSON(status, "\"status\":"); v != "" {
		rs.Status = v
	}
	if v := extractJSON(status, "\"chart\":"); v != "" {
		rs.Chart = v
	}

	return rs, nil
}

func extractJSON(s, key string) string {
	idx := strings.Index(s, key)
	if idx < 0 {
		return ""
	}
	rest := s[idx+len(key):]
	rest = strings.TrimSpace(rest)
	if len(rest) == 0 || rest[0] != '"' {
		return ""
	}
	end := strings.Index(rest[1:], "\"")
	if end < 0 {
		return ""
	}
	return rest[1 : end+1]
}
