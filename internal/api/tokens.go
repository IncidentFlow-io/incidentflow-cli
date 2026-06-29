package api

import (
	"context"
	"fmt"
)

type CreateTokenRequest struct {
	Name        string `json:"name,omitempty"`
	ClusterName string `json:"cluster_name,omitempty"`
}

type TokenItem struct {
	ID          string  `json:"id"`
	Prefix      string  `json:"prefix"`
	Status      string  `json:"status"`
	Name        *string `json:"name"`
	ClusterName *string `json:"cluster_name"`
	CreatedAt   string  `json:"created_at"`
}

type CreateTokenResponse struct {
	RegistrationToken string    `json:"registration_token"`
	TokenItem         TokenItem `json:"token_item"`
}

func (c *Client) CreateRegistrationToken(ctx context.Context, req CreateTokenRequest) (*CreateTokenResponse, error) {
	var resp CreateTokenResponse
	if err := c.do(ctx, "POST", "/api/v1/agents/registration-tokens", req, &resp); err != nil {
		return nil, fmt.Errorf("creating registration token: %w", err)
	}
	return &resp, nil
}

func (c *Client) RevokeRegistrationToken(ctx context.Context, tokenID string) error {
	path := fmt.Sprintf("/api/v1/agents/registration-tokens/%s", tokenID)
	return c.do(ctx, "DELETE", path, nil, nil)
}
