package kube

import (
	"context"
	"fmt"

	authv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Permission struct {
	Verb     string
	Resource string
	Group    string
}

var requiredPermissions = []Permission{
	{Verb: "create", Resource: "namespaces", Group: ""},
	{Verb: "create", Resource: "deployments", Group: "apps"},
	{Verb: "create", Resource: "secrets", Group: ""},
	{Verb: "create", Resource: "serviceaccounts", Group: ""},
	{Verb: "create", Resource: "clusterroles", Group: "rbac.authorization.k8s.io"},
	{Verb: "create", Resource: "clusterrolebindings", Group: "rbac.authorization.k8s.io"},
}

func (c *Client) CheckPermissions(ctx context.Context) error {
	var missing []string

	for _, p := range requiredPermissions {
		sar := &authv1.SelfSubjectAccessReview{
			Spec: authv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &authv1.ResourceAttributes{
					Verb:     p.Verb,
					Resource: p.Resource,
					Group:    p.Group,
				},
			},
		}

		result, err := c.clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(
			ctx, sar, metav1.CreateOptions{},
		)
		if err != nil {
			return fmt.Errorf("checking permissions: %w", err)
		}

		if !result.Status.Allowed {
			missing = append(missing, fmt.Sprintf("  - %s %s", p.Verb, p.Resource))
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf(
			"IncidentFlow could not create resources in the cluster.\n\nRequired permissions:\n%s\n\n"+
				"Contact your cluster administrator to grant these permissions.",
			joinLines(missing),
		)
	}

	return nil
}

func joinLines(lines []string) string {
	out := ""
	for _, l := range lines {
		out += l + "\n"
	}
	return out
}
