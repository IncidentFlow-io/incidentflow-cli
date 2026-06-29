package helm

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// ShowChart validates the chart ref is reachable before any mutations happen.
func (c *Client) ShowChart(ctx context.Context, chartRef, version string) error {
	args := []string{"show", "chart", chartRef}
	if version != "" && version != "latest" {
		args = append(args, "--version", version)
	}
	cmd := exec.CommandContext(ctx, c.helmBin, args...)
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf(
			"Helm chart was not found.\nChart: %s\nCheck that the chart is published and the chart reference is correct.",
			chartRef,
		)
	}
	return nil
}

// HasDiffPlugin reports whether the helm-diff plugin is installed.
func (c *Client) HasDiffPlugin() bool {
	out, err := exec.Command(c.helmBin, "plugin", "list").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "diff")
}

// Diff runs `helm diff upgrade --install` and returns the output.
// The caller must pass a placeholder for any secret --set values.
// --suppress-secrets masks Secret data in the diff output.
func (c *Client) Diff(ctx context.Context, opts InstallOptions) (string, error) {
	args := []string{
		"diff", "upgrade", "--install",
		opts.ReleaseName, opts.ChartRef,
		"--namespace", opts.Namespace,
		"--no-color",
		"--suppress-secrets",
	}
	for k, v := range opts.Set {
		args = append(args, "--set", fmt.Sprintf("%s=%s", k, v))
	}
	if opts.Version != "" && opts.Version != "latest" {
		args = append(args, "--version", opts.Version)
	}
	cmd := exec.CommandContext(ctx, c.helmBin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("helm diff failed:\n%s", string(out))
	}
	return string(out), nil
}
