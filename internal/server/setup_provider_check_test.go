package server

import "testing"

func TestSetupProbeStatusCode(t *testing.T) {
	t.Parallel()
	cases := map[int]string{200: "", 204: "", 401: "key_invalid", 403: "key_invalid", 429: "rate_limited", 404: "provider_error", 500: "provider_error"}
	for status, want := range cases {
		if got := setupProbeStatusCode(status); got != want {
			t.Errorf("status %d = %q, want %q", status, got, want)
		}
	}
}

func TestSetupModelListContains(t *testing.T) {
	t.Parallel()
	openai := []byte(`{"data":[{"id":"gpt-4o"},{"id":"google/gemini-3.1-flash-lite"}]}`)
	if c, f := setupModelListContains(openai, "openrouter", "google/gemini-3.1-flash-lite"); !c || !f {
		t.Fatalf("openrouter hit = %v %v", c, f)
	}
	if c, f := setupModelListContains(openai, "openai", "missing"); !c || f {
		t.Fatalf("openai miss = %v %v", c, f)
	}
	ollama := []byte(`{"models":[{"name":"llama3:latest","model":"llama3:latest"},{"name":"qwen:7b","model":"qwen:7b"}]}`)
	if c, f := setupModelListContains(ollama, "ollama", "llama3"); !c || !f {
		t.Fatalf("ollama :latest = %v %v", c, f)
	}
	if c, f := setupModelListContains(ollama, "ollama", "qwen:7b"); !c || !f {
		t.Fatalf("ollama exact = %v %v", c, f)
	}
	if c, f := setupModelListContains(ollama, "ollama", "nope"); !c || f {
		t.Fatalf("ollama miss = %v %v", c, f)
	}
	paged := []byte(`{"data":[{"id":"claude-haiku-5-5"}],"has_more":true}`)
	if c, _ := setupModelListContains(paged, "anthropic", "claude-sonnet-5-5"); c {
		t.Fatal("a miss on a partial page must not be judged")
	}
	for _, body := range []string{"", "not json", `{"data":[]}`, `{"other":1}`} {
		if c, _ := setupModelListContains([]byte(body), "openai", "x"); c {
			t.Errorf("body %q must not be judged", body)
		}
	}
}

func TestParseOpenRouterKeyInfo(t *testing.T) {
	t.Parallel()
	rem, free, ok := parseOpenRouterKeyInfo([]byte(`{"data":{"limit":10,"limit_remaining":3.5,"usage":6.5,"is_free_tier":false}}`))
	if !ok || rem == nil || *rem != 3.5 || free {
		t.Fatalf("number = %v %v %v", rem, free, ok)
	}
	rem, free, ok = parseOpenRouterKeyInfo([]byte(`{"data":{"limit":null,"limit_remaining":null,"usage":1,"is_free_tier":true}}`))
	if !ok || rem != nil || !free {
		t.Fatalf("null = %v %v %v", rem, free, ok)
	}
	if _, _, ok := parseOpenRouterKeyInfo([]byte(`{}`)); ok {
		t.Fatal("empty object must not parse as key info")
	}
}
