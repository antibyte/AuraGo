package obsidian

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ReadPeriodicNote reads the current or specific periodic note.
func (c *Client) ReadPeriodicNote(ctx context.Context, period string) (*NoteJSON, error) {
	endpoint := "/periodic/" + encodePath(period) + "/"

	resp, err := c.request(ctx, http.MethodGet, endpoint, nil, map[string]string{
		"Accept": "application/vnd.olrapi.note+json",
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var note NoteJSON
	if err := json.Unmarshal(body, &note); err != nil {
		return nil, fmt.Errorf("decode periodic note: %w", err)
	}
	return &note, nil
}

// CreatePeriodicNote creates or updates the current periodic note.
func (c *Client) CreatePeriodicNote(ctx context.Context, period, content string) error {
	endpoint := "/periodic/" + encodePath(period) + "/"

	resp, err := c.request(ctx, http.MethodPost, endpoint, bytes.NewReader([]byte(content)), map[string]string{
		"Content-Type": "text/markdown",
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// PatchPeriodicNote appends/prepends/replaces content on the current periodic note.
func (c *Client) PatchPeriodicNote(ctx context.Context, period, content, operation string) error {
	endpoint := "/periodic/" + encodePath(period) + "/"

	headers := map[string]string{
		"Content-Type": "text/markdown",
		"Operation":    operation,
	}

	resp, err := c.request(ctx, http.MethodPatch, endpoint, bytes.NewReader([]byte(content)), headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
