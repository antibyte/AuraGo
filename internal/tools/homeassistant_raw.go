package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// HAHTTPError reports a non-2xx Home Assistant answer without its body (bodies may echo request data).
type HAHTTPError struct{ Status int }

func (e *HAHTTPError) Error() string { return fmt.Sprintf("home assistant answered HTTP %d", e.Status) }

// haRawMaxBytes caps binary reads through HAGetBytes (the design's snapshot cap).
const haRawMaxBytes = 8 << 20

var errHAInvalidEndpoint = errors.New("invalid Home Assistant endpoint")

func haValidEndpoint(endpoint string) error {
	if endpoint == "" || strings.ContainsAny(endpoint, string(rune(0x5c))+"#") {
		return errHAInvalidEndpoint
	}
	for i := 0; i < len(endpoint); i++ {
		if c := endpoint[i]; c <= 0x20 || c == 0x7f {
			return errHAInvalidEndpoint
		}
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "" || u.Host != "" || u.Opaque != "" || u.User != nil || u.Fragment != "" {
		return errHAInvalidEndpoint
	}
	if !strings.HasPrefix(u.Path, "/api/") {
		return errHAInvalidEndpoint
	}
	segments := strings.Split(u.Path, "/")
	for i, seg := range segments[1:] {
		last := i == len(segments)-2
		if seg == "." || seg == ".." || (seg == "" && !last) {
			return errHAInvalidEndpoint
		}
	}
	escaped := strings.ToLower(u.EscapedPath())
	if strings.Contains(escaped, "%2f") || strings.Contains(escaped, "%5c") {
		return errHAInvalidEndpoint
	}
	return nil
}

// HARequestJSON performs a read-only GET against the Home Assistant REST API through the shared
// client policy and decodes a 2xx JSON answer into out (nil out skips decoding). Any other method
// or a request body is rejected before the network; writes must go through HACallServiceContext,
// which enforces read-only mode and the service policy.
func HARequestJSON(ctx context.Context, cfg HAConfig, method, endpoint string, body, out any) error {
	if method != http.MethodGet {
		return errors.New("home assistant raw requests are read-only")
	}
	if body != nil {
		return errors.New("home assistant raw requests take no body")
	}
	if err := haValidEndpoint(endpoint); err != nil {
		return err
	}
	data, code, _, err := haDo(ctx, cfg, method, endpoint, "", maxHTTPResponseSize)
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
	if max <= 0 || max > haRawMaxBytes {
		return nil, "", errors.New("invalid Home Assistant response size limit")
	}
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
