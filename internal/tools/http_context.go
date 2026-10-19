package tools

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"
)

// Native calls carry their own context. Shared clients/transports never store it.
func nativeHTTPContext(contexts ...context.Context) context.Context {
	if len(contexts) > 0 && contexts[0] != nil {
		return contexts[0]
	}
	return context.Background()
}

func lockToolHTTPContext(ctx context.Context, mu *sync.Mutex) error {
	for !mu.TryLock() {
		if err := waitToolHTTP(ctx, 10*time.Millisecond); err != nil {
			return err
		}
	}
	if err := nativeHTTPContext(ctx).Err(); err != nil {
		mu.Unlock()
		return err
	}
	return nil
}

func waitToolHTTP(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-nativeHTTPContext(ctx).Done():
		return nativeHTTPContext(ctx).Err()
	case <-timer.C:
		return nil
	}
}

func newToolHTTPRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	return http.NewRequestWithContext(nativeHTTPContext(ctx), method, url, body)
}

func getToolHTTP(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	req, err := newToolHTTPRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func postToolHTTP(ctx context.Context, client *http.Client, url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := newToolHTTPRequest(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return client.Do(req)
}
