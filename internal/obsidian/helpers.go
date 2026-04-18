package obsidian

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// decodeJSONResponse reads and decodes a JSON response body.
func decodeJSONResponse(resp *http.Response, result interface{}) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	if result != nil && len(body) > 0 {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// decodeNoteResponse reads a JSON response and decodes it into a NoteJSON.
// It handles 404 as "not found" and sets the path on successful decode.
func decodeNoteResponse(resp *http.Response, path string) (*NoteJSON, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("note not found: %s", path)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var note NoteJSON
	if err := json.Unmarshal(body, &note); err != nil {
		return nil, fmt.Errorf("decode note: %w", err)
	}
	note.Path = path
	return &note, nil
}
