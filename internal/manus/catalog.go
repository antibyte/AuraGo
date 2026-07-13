package manus

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

// Project is a Manus project available to the authenticated account.
type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	CreatedAt   int64  `json:"created_at"`
	Instruction string `json:"instruction"`
}

// Connector is an installed Manus connector.
type Connector struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// Skill is a Manus skill available to the account or selected project.
type Skill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	OwnerType   string `json:"owner_type"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

// ListProjects returns the authenticated account's project catalog.
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	var response struct {
		Data []Project `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/v2/project.list", nil, nil, &response); err != nil {
		return nil, err
	}
	return response.Data, nil
}

// ListConnectors returns the authenticated account's installed connectors.
func (c *Client) ListConnectors(ctx context.Context) ([]Connector, error) {
	var response struct {
		Data []Connector `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/v2/connector.list", nil, nil, &response); err != nil {
		return nil, err
	}
	return response.Data, nil
}

// ListSkills returns global skills and, optionally, project-specific skills.
func (c *Client) ListSkills(ctx context.Context, projectID string) ([]Skill, error) {
	query := url.Values{}
	if projectID = strings.TrimSpace(projectID); projectID != "" {
		query.Set("project_id", projectID)
	}
	var response struct {
		Data []Skill `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/v2/skill.list", query, nil, &response); err != nil {
		return nil, err
	}
	return response.Data, nil
}
