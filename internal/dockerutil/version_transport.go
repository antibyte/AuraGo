package dockerutil

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// VersionTransport negotiates before sending any versioned request. It never
// replays the requested operation, including after an uncertain mutation.
type VersionTransport struct {
	base    http.RoundTripper
	mu      sync.Mutex
	version string
	loading chan struct{}
}

func NewVersionTransport(base http.RoundTripper) *VersionTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &VersionTransport{base: base}
}

func (t *VersionTransport) CloseIdleConnections() {
	if c, ok := t.base.(interface{ CloseIdleConnections() }); ok {
		c.CloseIdleConnections()
	}
}

func dockerMinorVersion(value string) (int, error) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "v")
	parts := strings.Split(value, ".")
	if len(parts) != 2 || parts[0] != "1" || parts[1] == "" || strings.Trim(parts[1], "0123456789") != "" {
		return 0, fmt.Errorf("unsupported Docker API version %q", value)
	}
	n, err := strconv.Atoi(parts[1])
	return n, err
}

func (t *VersionTransport) versionFor(ctx context.Context, origin *url.URL) (string, error) {
	for {
		t.mu.Lock()
		if t.version != "" {
			v := t.version
			t.mu.Unlock()
			return v, nil
		}
		if pending := t.loading; pending != nil {
			t.mu.Unlock()
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-pending:
				continue
			}
		}
		t.loading = make(chan struct{})
		t.mu.Unlock()
		v, err := t.probeVersion(ctx, origin)
		t.mu.Lock()
		if err == nil {
			t.version = v
		}
		close(t.loading)
		t.loading = nil
		t.mu.Unlock()
		return v, err
	}
}

func (t *VersionTransport) probeVersion(ctx context.Context, origin *url.URL) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	u := *origin
	u.Path, u.RawPath, u.RawQuery, u.Fragment = "/version", "", "", ""
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return "", fmt.Errorf("negotiate Docker API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("negotiate Docker API: /version returned HTTP %d", resp.StatusCode)
	}
	var info struct {
		API     string `json:"ApiVersion"`
		Minimum string `json:"MinAPIVersion"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&info); err != nil {
		return "", fmt.Errorf("decode Docker API version: %w", err)
	}
	server, err := dockerMinorVersion(info.API)
	if err != nil {
		return "", err
	}
	minimum := 25 // managed containers require Engine Init support
	if info.Minimum != "" {
		min, err := dockerMinorVersion(info.Minimum)
		if err != nil {
			return "", err
		}
		if min > minimum {
			minimum = min
		}
	}
	client, _ := dockerMinorVersion(APIVersion)
	selected := min(client, server)
	if selected < minimum {
		return "", fmt.Errorf("Docker API has no supported version (server %s, minimum %s, client %s)", info.API, info.Minimum, APIVersion)
	}
	return fmt.Sprintf("v1.%d", selected), nil
}

func (t *VersionTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	prefix := "/" + APIVersion + "/"
	if !strings.HasPrefix(req.URL.Path, prefix) {
		return t.base.RoundTrip(req)
	}
	version, err := t.versionFor(req.Context(), req.URL)
	if err != nil {
		return nil, err
	}
	clone := req.Clone(req.Context())
	clone.URL.Path = "/" + version + "/" + strings.TrimPrefix(req.URL.Path, prefix)
	if clone.URL.RawPath != "" {
		clone.URL.RawPath = strings.Replace(clone.URL.RawPath, prefix, "/"+version+"/", 1)
	}
	resp, err := t.base.RoundTrip(clone)
	if staleVersionSignal(req.Context(), resp, err) {
		t.invalidate(version)
	}
	return resp, err
}

// invalidate forgets the negotiated version so the next request probes again.
// It clears only the value this request used, so a newer negotiation by a
// concurrent request is kept. It never resends the current request.
func (t *VersionTransport) invalidate(version string) {
	t.mu.Lock()
	if t.version == version {
		t.version = ""
	}
	t.mu.Unlock()
}

// staleVersionSignal reports outcomes that can follow an Engine restart with a
// different API range: a transport failure the caller did not cause by
// cancelling, or HTTP 400, which the Engine returns for an unsupported API
// version before any handler runs. The response body is never read here.
func staleVersionSignal(ctx context.Context, resp *http.Response, err error) bool {
	if err != nil {
		return ctx.Err() == nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
	}
	return resp != nil && resp.StatusCode == http.StatusBadRequest
}

// NegotiatedAPIVersion supports raw upgraded streams using the same policy.
func NegotiatedAPIVersion(ctx context.Context, host string) (string, error) {
	c := NewClient(host, 5*time.Second)
	defer c.CloseIdleConnections()
	u, _ := url.Parse("http://docker")
	return c.httpClient.Transport.(*VersionTransport).versionFor(ctx, u)
}
