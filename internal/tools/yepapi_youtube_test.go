package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type recordingYepAPIClient struct {
	srv       *httptest.Server
	api       *YepAPIClient
	calls     int
	lastLimit int
}

func (r *recordingYepAPIClient) client() *YepAPIClient {
	if r.api != nil {
		return r.api
	}
	r.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		var payload map[string]interface{}
		_ = json.Unmarshal(body, &payload)
		r.calls++
		if limit, ok := payload["limit"].(float64); ok {
			r.lastLimit = int(limit)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"data":{}}`))
	}))
	r.api = NewYepAPIClientWithBaseURL("test-key", r.srv.URL)
	return r.api
}

func TestYepAPIYouTubeRejectsLooseIDsAndCapsLimit(t *testing.T) {
	client := &recordingYepAPIClient{}
	t.Cleanup(func() {
		if client.srv != nil {
			client.srv.Close()
		}
	})

	_, err := DispatchYepAPIYouTube(context.Background(), client.client(), "video", map[string]interface{}{"video_id": "not a video"})
	if err != nil || client.calls != 0 {
		t.Fatalf("loose video id reached the provider, err=%v calls=%d", err, client.calls)
	}
	_, err = DispatchYepAPIYouTube(context.Background(), client.client(), "comments", map[string]interface{}{
		"video_id": "dQw4w9WgXcQ",
		"limit":    float64(500),
	})
	if err != nil || client.lastLimit != 25 {
		t.Fatalf("comments limit = %d, err = %v", client.lastLimit, err)
	}
	client.calls = 0
	client.lastLimit = 0
	_, err = DispatchYepAPIYouTube(context.Background(), client.client(), "search", map[string]interface{}{
		"query": "golang",
		"limit": float64(500),
	})
	if err != nil || client.lastLimit != 25 {
		t.Fatalf("search limit = %d, err = %v", client.lastLimit, err)
	}
	_, err = DispatchYepAPIYouTube(context.Background(), client.client(), "resolve", map[string]interface{}{
		"url": "https://example.com/watch?v=dQw4w9WgXcQ",
	})
	if err != nil || client.calls != 1 {
		t.Fatalf("non-YouTube resolve was sent, calls=%d err=%v", client.calls, err)
	}
}
