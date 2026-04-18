package obsidian

import (
	"bytes"
	"context"
	"fmt"
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

	var note NoteJSON
	if err := decodeJSONResponse(resp, &note); err != nil {
		return nil, fmt.Errorf("read periodic note: %w", err)
	}
	// API returns the note with populated path
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

	return decodeJSONResponse(resp, nil)
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

	return decodeJSONResponse(resp, nil)
}
