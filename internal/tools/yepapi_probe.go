package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Probe checks the documented free model catalog without creating a billable
// search or generation. Public catalogs do not establish API-key validity.
func (c *YepAPIClient) Probe(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/ai/models", nil)
	if err != nil {
		return fmt.Errorf("build YepAPI probe: %w", err)
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("YepAPI probe: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("YepAPI probe returned HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (2<<20)+1))
	if err != nil {
		return fmt.Errorf("read YepAPI probe: %w", err)
	}
	if len(raw) > 2<<20 || !json.Valid(raw) {
		return fmt.Errorf("invalid YepAPI catalog response")
	}
	var result struct {
		OK    *bool           `json:"ok"`
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("decode YepAPI catalog: %w", err)
	}
	if (result.OK != nil && !*result.OK) || (len(result.Error) > 0 && string(result.Error) != "null") {
		return fmt.Errorf("YepAPI catalog reported an error")
	}
	return nil
}
