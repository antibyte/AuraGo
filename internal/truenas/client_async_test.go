package truenas

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientMutationsAcceptAcceptedStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		call   func(context.Context, *Client) error
	}{
		{
			name:   "post",
			method: http.MethodPost,
			call: func(ctx context.Context, c *Client) error {
				return c.Post(ctx, "/job", map[string]string{"name": "scrub"}, nil)
			},
		},
		{
			name:   "put",
			method: http.MethodPut,
			call: func(ctx context.Context, c *Client) error {
				return c.Put(ctx, "/job/1", map[string]string{"state": "queued"})
			},
		},
		{
			name:   "delete",
			method: http.MethodDelete,
			call: func(ctx context.Context, c *Client) error {
				return c.Delete(ctx, "/job/1")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.method {
					t.Fatalf("method = %s, want %s", r.Method, tt.method)
				}
				w.WriteHeader(http.StatusAccepted)
			}))
			defer server.Close()

			client := &Client{
				baseURL:    server.URL,
				apiKey:     "test-key",
				httpClient: server.Client(),
			}

			if err := tt.call(context.Background(), client); err != nil {
				t.Fatalf("%s returned error for 202 Accepted: %v", tt.name, err)
			}
		})
	}
}
