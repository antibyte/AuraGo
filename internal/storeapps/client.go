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
	if err := security.ValidateHTTPBaseURL(c.URL); err != nil {
		return errors.New("Invalid Store app address")
	}
	if c.Credential == "" {
		return errors.New("Store app credential is missing")
	}
	security.RegisterSensitive(c.Credential)
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil || len(data) > 256<<10 {
			return errors.New("Invalid or oversized Store request")
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.URL, "/")+path, reader)
	if err != nil {
		return errors.New("Invalid Store app request")
	}
	req.Header.Set("Authorization", "Bearer "+c.Credential)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	client := httporigin.NewClient(30 * time.Second)
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		if mutation {
			return ErrUncertain
		}
		return errors.New("Store app request failed or was cancelled")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if mutation && resp.StatusCode >= 500 {
			return ErrUncertain
		}
		return fmt.Errorf("Store app rejected the request (HTTP %d)", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (2<<20)+1))
	if err != nil || len(data) > 2<<20 || (result != nil && json.Unmarshal(data, result) != nil) {
		if mutation {
			return ErrUncertain
		}
		return errors.New("Store app response is incomplete, incompatible or exceeds 2 MiB")
	}
	return nil
}
