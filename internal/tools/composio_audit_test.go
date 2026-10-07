package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestComposioAllowlistCannotOverrideReadOnly(t *testing.T) {
	no := false
	for _, slug := range []string{"GMAIL_SEND_EMAIL", "GMAIL_GET_AND_TRASH_EMAIL", "GMAIL_CANCEL_SEND", "SLACK_BAN_USER"} {
		cfg := ComposioPolicyConfig{Enabled: true, ReadOnly: true, AllowDestructive: true, Toolkits: []ComposioToolkitPolicy{{Slug: "gmail", Enabled: true, ReadOnly: &no, AllowedToolSlugs: []string{slug}}}}
		if EvaluateComposioToolPolicy(cfg, ComposioToolInfo{Slug: slug, ToolkitSlug: "gmail"}).Allowed {
			t.Errorf("read-only bypass: %s", slug)
		}
	}
}

func TestComposioCustomKeyNeverFollowsForeignRedirect(t *testing.T) {
	var received atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { received.Add(1) }))
	defer destination.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer origin.Close()
	c := NewComposioClient(ComposioClientConfig{BaseURL: origin.URL, APIKey: "fixture-private-token"})
	if _, err := c.GetTool(context.Background(), "GMAIL_GET_EMAIL"); err == nil {
		t.Fatal("foreign redirect accepted")
	}
	if received.Load() != 0 {
		t.Fatal("API key forwarded to a different origin")
	}
}
