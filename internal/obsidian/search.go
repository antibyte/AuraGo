package obsidian

import (
	"context"
	"fmt"
)

// SearchSimple performs a full-text search across the vault.
func (c *Client) SearchSimple(ctx context.Context, query string, contextLength int) ([]SimpleSearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("search query is required")
	}

	if contextLength <= 0 {
		contextLength = 100
	}

	endpoint := fmt.Sprintf("/search/simple/?query=%s&contextLength=%d", encodePath(query), contextLength)

	resp, err := c.request(ctx, "POST", endpoint, nil, map[string]string{
		"Accept": "application/json",
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var results []SimpleSearchResult
	if err := decodeJSONResponse(resp, &results); err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	return results, nil
}

// SearchDataview performs a Dataview DQL query.
func (c *Client) SearchDataview(ctx context.Context, dqlQuery string) (interface{}, error) {
	if dqlQuery == "" {
		return nil, fmt.Errorf("DQL query is required")
	}

	body := map[string]string{"query": dqlQuery}

	var result interface{}
	if err := c.postJSON(ctx, "/search/", body, &result); err != nil {
		return nil, fmt.Errorf("dataview search: %w", err)
	}
	return result, nil
}
