package helm

import (
	"fmt"
	"os/exec"
	"strings"
)

type Client struct {
	helmBin string
}

func NewClient() (*Client, error) {
	bin, err := exec.LookPath("helm")
	if err != nil {
		return nil, fmt.Errorf(
			"Helm is not installed.\n\nInstall Helm 3 and run again:\n  https://helm.sh/docs/intro/install/",
		)
	}
	return &Client{helmBin: bin}, nil
}

func (c *Client) Version() (string, error) {
	out, err := exec.Command(c.helmBin, "version", "--short").Output()
	if err != nil {
		return "", fmt.Errorf("helm version: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
