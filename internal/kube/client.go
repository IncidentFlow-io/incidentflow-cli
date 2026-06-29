package kube

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	clientset *kubernetes.Clientset
}

func NewClient() (*Client, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		rules, &clientcmd.ConfigOverrides{},
	).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf(
			"no Kubernetes context found.\n\nRun:\n  kubectl config get-contexts\n  kubectl config use-context <context>\n\nError: %w",
			err,
		)
	}

	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating kube client: %w", err)
	}

	return &Client{clientset: cs}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return fmt.Errorf("Kubernetes API unreachable: %w", err)
	}
	return nil
}

func (c *Client) Clientset() *kubernetes.Clientset {
	return c.clientset
}
