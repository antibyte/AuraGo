package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/localwiki"
	"aurago/internal/tools"
)

type agentWikiLibrary struct {
	query string
	limit int
	read  localwiki.ReadRequest
}

func (l *agentWikiLibrary) Edition() localwiki.Edition {
	return localwiki.Edition{Language: "de", Variant: localwiki.VariantNoPic, Date: "2026-10"}
}

func (l *agentWikiLibrary) Fulltext() bool { return true }

func (l *agentWikiLibrary) Search(_ context.Context, query string, limit, _ int) (localwiki.SearchResult, error) {
	l.query, l.limit = query, limit
	return localwiki.SearchResult{Edition: l.Edition(), Fulltext: true, Results: []localwiki.SearchHit{{Ref: localwiki.Ref{Title: "Berlin", Path: "Berlin"}, Snippet: "Berlin ist die Hauptstadt."}}}, nil
}

func (l *agentWikiLibrary) Read(_ context.Context, req localwiki.ReadRequest) (localwiki.Article, error) {
	l.read = req
	return localwiki.Article{Ref: localwiki.Ref{Title: "Berlin", Path: "Berlin"}, Content: "# Berlin"}, nil
}

type agentWikiSource struct {
	lib  *agentWikiLibrary
	open bool
}

func (s *agentWikiSource) AcquireLibrary() (tools.LocalWikipediaLibrary, func(), bool) {
	if !s.open {
		return nil, nil, false
	}
	return s.lib, func() {}, true
}

// useAgentWikiSource publishes a fake edition source; tests using it must not run in parallel.
func useAgentWikiSource(t *testing.T, src tools.LocalWikipediaSource) {
	t.Helper()
	tools.SetLocalWikipediaSource(src)
	t.Cleanup(func() { tools.SetLocalWikipediaSource(nil) })
}

func localWikipediaTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.LocalWikipedia.Enabled = true
	cfg.LocalWikipedia.AgentAccess = true
	return cfg
}

func TestDispatchLocalWikipediaDecodesArguments(t *testing.T) {
	lib := &agentWikiLibrary{}
	useAgentWikiSource(t, &agentWikiSource{lib: lib, open: true})
	dc := &DispatchContext{Cfg: localWikipediaTestConfig(), Logger: testLogger}

	out, ok := dispatchPlatform(context.Background(), ToolCall{Action: "local_wikipedia", Params: map[string]interface{}{
		"operation": "search", "query": "Hauptstadt Deutschland", "limit": float64(3),
	}}, dc)
	if !ok || !strings.Contains(out, `"status":"success"`) || lib.query != "Hauptstadt Deutschland" || lib.limit != 3 {
		t.Fatalf("search: handled=%v out=%s query=%q limit=%d", ok, out, lib.query, lib.limit)
	}

	out, _ = dispatchPlatform(context.Background(), ToolCall{Action: "local_wikipedia", Params: map[string]interface{}{
		"operation": "read", "path": "Berlin", "section": float64(2), "offset": float64(8000),
	}}, dc)
	if want := (localwiki.ReadRequest{Path: "Berlin", Section: "2", Offset: 8000}); lib.read != want {
		t.Fatalf("read request = %+v, want %+v (out %s)", lib.read, want, out)
	}

	_, _ = dispatchPlatform(context.Background(), ToolCall{Action: "local_wikipedia", Operation: "read", Title: "Berlin", Params: map[string]interface{}{"section": " Geschichte "}}, dc)
	if want := (localwiki.ReadRequest{Title: "Berlin", Section: "Geschichte"}); lib.read != want {
		t.Fatalf("text-mode read request = %+v, want %+v", lib.read, want)
	}

	_, _ = dispatchPlatform(context.Background(), ToolCall{Action: "local_wikipedia", Operation: "read", Path: "Berlin", Offset: 16000}, dc)
	if want := (localwiki.ReadRequest{Path: "Berlin", Offset: 16000}); lib.read != want {
		t.Fatalf("text-mode paged read request = %+v, want %+v", lib.read, want)
	}
}

func TestDispatchLocalWikipediaHonoursAgentAccess(t *testing.T) {
	useAgentWikiSource(t, &agentWikiSource{lib: &agentWikiLibrary{}, open: true})
	cfg := localWikipediaTestConfig()
	cfg.LocalWikipedia.AgentAccess = false
	out, ok := dispatchPlatform(context.Background(), ToolCall{Action: "local_wikipedia", Operation: "search", Query: "Berlin"}, &DispatchContext{Cfg: cfg, Logger: testLogger})
	if !ok || !strings.Contains(out, `"status":"policy_denied"`) {
		t.Fatalf("handled=%v out=%s", ok, out)
	}
	if got := classifyLegacyToolResult(out); got != ToolResultDenied {
		t.Fatalf("result status = %v, want denied", got)
	}
}

func TestDecodeLocalWikipediaNumbersAreWholeOrText(t *testing.T) {
	for _, tc := range []struct {
		section any
		want    string
	}{
		{float64(3), "3"}, {float64(0), "0"}, {2.5, "2.5"}, {1e300, "1e+300"}, {float64(-1), "-1"},
		{json.Number("4"), "4"}, {7, "7"}, {" Geschichte ", "Geschichte"}, {nil, ""}, {true, ""},
	} {
		if got := decodeLocalWikipediaArgs(ToolCall{Params: map[string]interface{}{"section": tc.section}}).Section; got != tc.want {
			t.Errorf("section %#v decoded as %q, want %q", tc.section, got, tc.want)
		}
	}
	for _, tc := range []struct {
		offset any
		want   int
	}{
		{float64(8000), 8000}, {2.5, -1}, {1e300, -1}, {float64(-3), -1}, {json.Number("16000"), 16000}, {json.Number("x"), -1},
	} {
		if got := decodeLocalWikipediaArgs(ToolCall{Params: map[string]interface{}{"offset": tc.offset}}).Offset; got != tc.want {
			t.Errorf("offset %#v decoded as %d, want %d", tc.offset, got, tc.want)
		}
	}
	if got := decodeLocalWikipediaArgs(ToolCall{Offset: 24000}).Offset; got != 24000 {
		t.Errorf("text-mode offset = %d", got)
	}
	useAgentWikiSource(t, &agentWikiSource{lib: &agentWikiLibrary{}, open: true})
	out, _ := dispatchPlatform(context.Background(), ToolCall{Action: "local_wikipedia", Params: map[string]interface{}{"operation": "read", "path": "Berlin", "offset": 2.5}}, &DispatchContext{Cfg: localWikipediaTestConfig(), Logger: testLogger})
	if !strings.Contains(out, `"code":"invalid_request"`) {
		t.Fatalf("fractional offset accepted: %s", out)
	}
}
