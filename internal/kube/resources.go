package kube

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Client) WaitForDeployment(ctx context.Context, namespace, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		d, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil && deploymentReady(d) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}

	return fmt.Errorf("deployment %s/%s did not become ready within %s", namespace, name, timeout)
}

func deploymentReady(d *appsv1.Deployment) bool {
	desired := int32(1)
	if d.Spec.Replicas != nil {
		desired = *d.Spec.Replicas
	}
	return d.Status.ReadyReplicas >= desired
}

func (c *Client) GetDeploymentStatus(ctx context.Context, namespace, name string) (string, error) {
	d, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	desired := int32(1)
	if d.Spec.Replicas != nil {
		desired = *d.Spec.Replicas
	}

	return fmt.Sprintf("%d/%d ready", d.Status.ReadyReplicas, desired), nil
}

func (c *Client) GetPodStatus(ctx context.Context, namespace, labelSelector string) ([]string, error) {
	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	var lines []string
	for _, p := range pods.Items {
		lines = append(lines, fmt.Sprintf("  %s  %s", p.Name, string(p.Status.Phase)))
	}
	return lines, nil
}

func (c *Client) GetEvents(ctx context.Context, namespace string) ([]string, error) {
	events, err := c.clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var lines []string
	for _, e := range events.Items {
		if e.Type == "Warning" {
			lines = append(lines, fmt.Sprintf("  [%s] %s: %s", e.Reason, e.InvolvedObject.Name, e.Message))
		}
	}
	return lines, nil
}
