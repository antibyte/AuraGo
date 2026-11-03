package launchpad

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"aurago/internal/security"
)

type iconRoundTripFunc func(*http.Request) (*http.Response, error)

func (f iconRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDownloadIconRefusesPrivateDestinationsEvenWithOverride(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	for _, host := range []string{"127.0.0.1", "10.1.2.3", "192.168.1.1", "169.254.169.254", "[::1]", "[fc00::1]", "[fe80::1]", "[::ffff:127.0.0.1]"} {
		if _, err := DownloadIconContext(t.Context(), t.TempDir(), "http://"+host+"/icon.png", "test"); err == nil {
			t.Fatalf("accepted %s", host)
		}
	}
}

func TestDownloadIconRevalidatesRedirectsAndBoundsResponses(t *testing.T) {
	for _, tc := range []struct {
		name, location, body string
		redirects            int
		wantError            bool
	}{
		{"public", "", "png", 0, false},
		{"private redirect", "http://127.0.0.1/private", "", 1, true},
		{"ipv6 redirect", "http://[::1]/private", "", 1, true},
		{"ten redirects", "/next.png", "png", 10, false},
		{"eleven redirects", "/next.png", "png", 11, true},
		{"oversize", "", strings.Repeat("x", maxIconSize+1), 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests, validations := 0, 0
			factory := func(ctx context.Context, raw string, timeout time.Duration) (*http.Client, error) {
				validations++
				client, err := security.NewStrictPublicHTTPClientForURLContext(ctx, raw, timeout)
				if err != nil {
					return nil, err
				}
				client.Transport = iconRoundTripFunc(func(r *http.Request) (*http.Response, error) {
					requests++
					header := http.Header{"Content-Type": []string{"image/png"}}
					status := http.StatusOK
					if requests <= tc.redirects {
						status = 302
						header.Set("Location", tc.location)
					}
					return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(tc.body)), Request: r}, nil
				})
				return client, nil
			}
			_, err := downloadIcon(t.Context(), t.TempDir(), "https://8.8.8.8/icon.png", "test", factory)
			if (err != nil) != tc.wantError {
				t.Fatalf("error=%v", err)
			}
			if !tc.wantError && validations != tc.redirects+1 {
				t.Fatal("redirect skipped validation")
			}
			if strings.Contains(tc.name, "private") && requests != 1 {
				t.Fatal("private redirect contacted")
			}
		})
	}
}

func TestDownloadIconCancellationReachesTransport(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	entered := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		_, err := downloadIcon(ctx, t.TempDir(), "https://8.8.8.8/icon.png", "test", func(context.Context, string, time.Duration) (*http.Client, error) {
			return &http.Client{Transport: iconRoundTripFunc(func(r *http.Request) (*http.Response, error) {
				close(entered)
				<-r.Context().Done()
				return nil, r.Context().Err()
			})}, nil
		})
		finished <- err
	}()
	<-entered
	cancel()
	if err := <-finished; err == nil {
		t.Fatal("cancelled download succeeded")
	}
}
