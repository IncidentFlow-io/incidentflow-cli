package api

import (
	"context"
	"encoding/json"
	"fmt"
)

// WhoAmI verifies credentials against /api/v1/auth/me (used as fallback with --token flag).
type WhoAmIResponse struct {
	UserID string `json:"id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

func (c *Client) WhoAmI(ctx context.Context) (*WhoAmIResponse, error) {
	var resp struct {
		User WhoAmIResponse `json:"user"`
	}
	if err := c.do(ctx, "GET", "/api/v1/auth/me", nil, &resp); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}
	return &resp.User, nil
}

// --- Device login flow ---

type DeviceStartResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type DeviceTokenResponse struct {
	Status    string          `json:"status"`
	RawTokens json.RawMessage `json:"tokens,omitempty"`
	RawUser   json.RawMessage `json:"user,omitempty"`
	Workspace *struct {
		ID   string `json:"id"`
		Slug string `json:"slug"`
		Name string `json:"name"`
	} `json:"workspace,omitempty"`
}

type TokenPair struct {
	AccessToken        string `json:"access_token"`
	RefreshToken       string `json:"refresh_token"`
	ExpiresIn          int    `json:"expires_in"`
	RefreshExpiresIn   int    `json:"refresh_expires_in"`
}

type DeviceApprovedResult struct {
	Tokens    TokenPair
	Email     string
	Name      string
	Workspace string // slug or name
}

func (c *Client) DeviceStart(ctx context.Context) (*DeviceStartResponse, error) {
	var resp DeviceStartResponse
	if err := c.do(ctx, "POST", "/api/cli/device/start", struct{}{}, &resp); err != nil {
		return nil, fmt.Errorf("starting device login: %w", err)
	}
	return &resp, nil
}

func (c *Client) DevicePoll(ctx context.Context, deviceCode string) (*DeviceTokenResponse, error) {
	var resp DeviceTokenResponse
	body := map[string]string{"device_code": deviceCode}
	if err := c.do(ctx, "POST", "/api/cli/device/token", body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (r *DeviceTokenResponse) Approved() (*DeviceApprovedResult, error) {
	if r.Status != "approved" {
		return nil, nil
	}

	var tokens TokenPair
	if err := json.Unmarshal(r.RawTokens, &tokens); err != nil {
		return nil, fmt.Errorf("parsing tokens: %w", err)
	}

	var user struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if len(r.RawUser) > 0 {
		_ = json.Unmarshal(r.RawUser, &user)
	}

	workspace := ""
	if r.Workspace != nil {
		workspace = r.Workspace.Slug
		if workspace == "" {
			workspace = r.Workspace.Name
		}
	}

	return &DeviceApprovedResult{
		Tokens:    tokens,
		Email:     user.Email,
		Name:      user.Name,
		Workspace: workspace,
	}, nil
}
