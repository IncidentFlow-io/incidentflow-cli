package api

import (
	"context"
	"fmt"
	"time"
)

type AgentStatus struct {
	ClusterName   string    `json:"cluster_name"`
	Status        string    `json:"status"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	AgentVersion  string    `json:"agent_version"`
	Namespace     string    `json:"namespace"`
}

type ListAgentsResponse struct {
	Agents []AgentStatus `json:"agents"`
}

func (c *Client) GetAgentStatus(ctx context.Context, clusterName string) (*AgentStatus, error) {
	var resp struct {
		Clusters []struct {
			Name          string    `json:"name"`
			AgentStatus   string    `json:"agent_status"`
			LastHeartbeat time.Time `json:"last_heartbeat_at"`
			AgentVersion  string    `json:"agent_version"`
		} `json:"clusters"`
	}
	if err := c.do(ctx, "GET", "/api/v1/agents/clusters", nil, &resp); err != nil {
		return nil, fmt.Errorf("getting agent status: %w", err)
	}
	for _, cl := range resp.Clusters {
		if cl.Name == clusterName {
			return &AgentStatus{
				ClusterName:   cl.Name,
				Status:        cl.AgentStatus,
				LastHeartbeat: cl.LastHeartbeat,
				AgentVersion:  cl.AgentVersion,
			}, nil
		}
	}
	return nil, fmt.Errorf("cluster %q not found", clusterName)
}

func (c *Client) ListAgents(ctx context.Context) ([]AgentStatus, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/agents", c.workspace)
	var resp ListAgentsResponse
	if err := c.do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, fmt.Errorf("listing agents: %w", err)
	}
	return resp.Agents, nil
}

func (c *Client) DeleteAgent(ctx context.Context, clusterName string) error {
	path := fmt.Sprintf("/api/v1/workspaces/%s/agents/%s", c.workspace, clusterName)
	return c.do(ctx, "DELETE", path, nil, nil)
}
