package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// HAHTTPError reports a non-2xx Home Assistant answer without its body (bodies may echo request data).
type HAHTTPError struct{ Status int }

func (e *HAHTTPError) Error() string { return fmt.Sprintf("home assistant answered HTTP %d", e.Status) }

func haValidEndpoint(endpoint string) error {
	if !strings.HasPrefix(endpoint, "/api/") || strings.Contains(endpoint, "://") || strings.Contains(endpoint, "/../") || strings.HasSuffix(endpoint, "/..") {
		return errors.New("invalid Home Assistant endpoint")
	}
	return nil
}

// HARequestJSON calls the Home Assistant REST API through the shared client policy and decodes a
// 2xx JSON answer into out (nil out skips decoding). body is JSON-encoded when not nil.
func HARequestJSON(ctx context.Context, cfg HAConfig, method, endpoint string, body, out any) error {
	if err := haValidEndpoint(endpoint); err != nil {
		return err
	}
	payload := ""
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode Home Assistant request: %w", err)
		}
		payload = string(raw)
	}
	data, code, _, err := haDo(ctx, cfg, method, endpoint, payload, maxHTTPResponseSize)
	if err != nil {
		return err
	}
	if code < 200 || code > 299 {
		return &HAHTTPError{Status: code}
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode Home Assistant answer: %w", err)
	}
	return nil
}

// HAGetBytes reads a binary resource (camera_proxy) up to max bytes and returns its Content-Type.
func HAGetBytes(ctx context.Context, cfg HAConfig, endpoint string, max int64) ([]byte, string, error) {
	if err := haValidEndpoint(endpoint); err != nil {
		return nil, "", err
	}
	data, code, header, err := haDo(ctx, cfg, http.MethodGet, endpoint, "", max)
	if err != nil {
		return nil, "", err
	}
	if code < 200 || code > 299 {
		return nil, "", &HAHTTPError{Status: code}
	}
	return data, header.Get("Content-Type"), nil
}
