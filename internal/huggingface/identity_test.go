package huggingface

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestRepositoryIdentityAndPathFailBeforeNetwork(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.Write([]byte(`{}`)) }))
	defer server.Close()
	client := NewClient(ClientConfig{HubBaseURL: server.URL})
	for _, id := range []string{"org/../../other/repo", "org/%252e%252e/other", "org/repo?x=y", "/org/repo", "org//repo", "org/repo.git", "org/repo ", "org\\repo"} {
		if _, err := client.GetModel(context.Background(), id); err == nil {
			t.Errorf("read accepted %q", id)
		}
		if _, err := client.CreateDiscussion(context.Background(), DiscussionOptions{RepoID: id}); err == nil {
			t.Errorf("write accepted %q", id)
		}
	}
	for _, base := range []string{"ftp://example.com", "https:///nohost", "https://name:pass@example.com", "https://example.com?secret=x", "https://example.com#frag"} {
		if _, err := buildURL(base, "/api/models", nil); err == nil {
			t.Errorf("invalid base accepted %q", base)
		}
	}
	for _, endpoint := range []string{"/api/models/org/repo/../other", "/api/%252e%252e/secret", "/api/models/org\\repo"} {
		if _, err := buildURL(server.URL, endpoint, nil); err == nil {
			t.Errorf("invalid path accepted %q", endpoint)
		}
	}
	if requests.Load() != 0 {
		t.Fatal("invalid identity reached HTTP")
	}
	for _, id := range []string{"gpt2", "org/Model-v1.2_3"} {
		if got, err := CanonicalRepoID(id); err != nil || got != id {
			t.Fatalf("valid identity changed: %q %v", got, err)
		}
	}
}

func TestAPIRedirectDoesNotForwardCredentials(t *testing.T) {
	var forwarded atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded.Add(1) }))
	defer foreign.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, foreign.URL, http.StatusTemporaryRedirect)
	}))
	defer origin.Close()
	client := NewClient(ClientConfig{HubBaseURL: origin.URL, Token: "fixture-private-token"})
	if _, err := client.WhoAmI(context.Background()); err == nil {
		t.Fatal("foreign redirect accepted")
	}
	if forwarded.Load() != 0 {
		t.Fatal("redirect reached untrusted origin")
	}
	if _, err := client.DownloadFile(context.Background(), DownloadFileOptions{RepoID: "org/repo", Path: "file", Destination: t.TempDir() + "/file"}); err == nil {
		t.Fatal("private download redirect accepted")
	}
	if forwarded.Load() != 0 {
		t.Fatal("download redirect escaped destination policy")
	}
}
