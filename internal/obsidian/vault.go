package obsidian

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ListFiles lists files and directories at the given vault path.
// Pass empty string for vault root.
func (c *Client) ListFiles(ctx context.Context, directory string) ([]FileEntry, error) {
	endpoint := "/vault/"
	if directory != "" {
		endpoint = "/vault/" + encodePath(directory) + "/"
	}

	resp, err := c.request(ctx, http.MethodGet, endpoint, nil, map[string]string{
		"Accept": "application/json",
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

	var result struct {
		Files []string `json:"files"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		// Try as array of FileEntry
		var entries []FileEntry
		if err2 := json.Unmarshal(body, &entries); err2 != nil {
			return nil, fmt.Errorf("decode file list: %w", err)
		}
		return entries, nil
	}

	entries := make([]FileEntry, 0, len(result.Files))
	for _, f := range result.Files {
		t := "file"
		if len(f) > 0 && f[len(f)-1] == '/' {
			t = "directory"
		}
		entries = append(entries, FileEntry{Path: f, Type: t})
	}
	return entries, nil
}

// ReadNote reads a note and returns it with metadata.
func (c *Client) ReadNote(ctx context.Context, path string) (*NoteJSON, error) {
	endpoint := "/vault/" + encodePath(path)

	resp, err := c.request(ctx, http.MethodGet, endpoint, nil, map[string]string{
		"Accept": "application/vnd.olrapi.note+json",
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return decodeNoteResponse(resp, path)
}

// ReadNoteSection reads a specific section of a note using sub-document targeting.
func (c *Client) ReadNoteSection(ctx context.Context, path, targetType, target string) (*NoteJSON, error) {
	endpoint := "/vault/" + encodePath(path)

	headers := map[string]string{
		"Accept":      "application/vnd.olrapi.note+json",
		"Target-Type": targetType,
		"Target":      target,
	}

	resp, err := c.request(ctx, http.MethodGet, endpoint, nil, headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return decodeNoteResponse(resp, path)
}

// CreateNote creates a new note with the given content.
func (c *Client) CreateNote(ctx context.Context, path, content string) error {
	endpoint := "/vault/" + encodePath(path)

	resp, err := c.request(ctx, http.MethodPost, endpoint, bytes.NewReader([]byte(content)), map[string]string{
		"Content-Type": "text/markdown",
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// UpdateNote replaces the entire content of an existing note.
func (c *Client) UpdateNote(ctx context.Context, path, content string) error {
	endpoint := "/vault/" + encodePath(path)

	resp, err := c.request(ctx, http.MethodPut, endpoint, bytes.NewReader([]byte(content)), map[string]string{
		"Content-Type": "text/markdown",
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// PatchNote patches a note using the specified operation on a target.
func (c *Client) PatchNote(ctx context.Context, path, content, targetType, target, operation string) error {
	endpoint := "/vault/" + encodePath(path)

	headers := map[string]string{
		"Content-Type":             "text/markdown",
		"Operation":                operation,
		"Create-Target-If-Missing": "true",
	}
	if targetType != "" {
		headers["Target-Type"] = targetType
	}
	if target != "" {
		headers["Target"] = target
	}

	resp, err := c.request(ctx, http.MethodPatch, endpoint, bytes.NewReader([]byte(content)), headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// DeleteNote deletes a note from the vault.
func (c *Client) DeleteNote(ctx context.Context, path string) error {
	endpoint := "/vault/" + encodePath(path)

	resp, err := c.request(ctx, http.MethodDelete, endpoint, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetDocumentMap returns the structural map of a note (headings, blocks, frontmatter keys).
func (c *Client) GetDocumentMap(ctx context.Context, path string) ([]DocumentMapEntry, error) {
	endpoint := "/vault/" + encodePath(path)

	resp, err := c.request(ctx, http.MethodGet, endpoint, nil, map[string]string{
		"Accept": "application/vnd.olrapi.document-map+json",
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

	var entries []DocumentMapEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("decode document map: %w", err)
	}
	return entries, nil
}
