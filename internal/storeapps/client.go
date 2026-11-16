package storeapps

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"aurago/internal/httporigin"
	"aurago/internal/security"
)

var ErrUncertain = errors.New("Request outcome unknown; check execution status before considering another action")

type Connection struct {
	URL, Username, Credential      string
	AllowedIDs                     []string
	Source, CertificateFingerprint string
}

func (c Connection) Allowed(id string) bool { return id != "" && slices.Contains(c.AllowedIDs, id) }

func (c Connection) request(ctx context.Context, method, path string, body any, mutation bool, result any) error {
	return c.jsonRequest(ctx, method, path, body, mutation, "Authorization", "Bearer "+c.Credential, result)
}

func (c Connection) jsonRequest(ctx context.Context, method, path string, body any, mutation bool, authKey, authValue string, result any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil || len(data) > 256<<10 {
			return errors.New("Invalid or oversized Store request")
		}
		reader = bytes.NewReader(data)
	}
	data, err := c.exchange(ctx, method, path, reader, "application/json", authKey, authValue, mutation, 2<<20)
	if err != nil {
		return err
	}
	if result != nil && json.Unmarshal(data, result) != nil {
		if mutation {
			return ErrUncertain
		}
		return errors.New("Store app returned an incompatible response; update the app")
	}
	return nil
}

func (c Connection) exchange(ctx context.Context, method, path string, reader io.Reader, contentType, authKey, authValue string, mutation bool, maxBytes int64) ([]byte, error) {
	if err := security.ValidateHTTPBaseURL(c.URL); err != nil {
		return nil, errors.New("Invalid Store app address")
	}
	if c.Credential == "" {
		return nil, errors.New("Store app credential is missing")
	}
	security.RegisterSensitive(c.Credential)
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.URL, "/")+path, reader)
	if err != nil {
		return nil, errors.New("Invalid Store app request")
	}
	if authKey != "" {
		req.Header.Set(authKey, authValue)
	}
	req.Header.Set("Content-Type", contentType)
	client := httporigin.NewClient(5 * time.Minute)
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		if mutation {
			return nil, ErrUncertain
		}
		return nil, errors.New("Store app request failed or was cancelled")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if mutation && resp.StatusCode >= 500 {
			return nil, ErrUncertain
		}
		return nil, fmt.Errorf("Store app rejected the request (HTTP %d)", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil || int64(len(data)) > maxBytes {
		if mutation {
			return nil, ErrUncertain
		}
		return nil, errors.New("Store app response is incomplete or exceeds its size limit")
	}
	return data, nil
}
