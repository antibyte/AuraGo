package flows

import (
	"errors"
	"reflect"
	"testing"
)

type fakeSecrets map[string]string

func (f fakeSecrets) ReadSecret(key string) (string, error) {
	v, ok := f[key]
	if !ok {
		return "", errors.New("missing")
	}
	return v, nil
}

func webRegistry(t *testing.T, env CatalogEnv) *Registry {
	t.Helper()
	reg := NewRegistry()
	if err := registerWebNodes(reg, env); err != nil {
		t.Fatal(err)
	}
	return reg
}

func TestWebSearchProviders(t *testing.T) {
	brave := &fakeTools{respond: toolReply(`{"status":"success","query":"ki","result_count":1,"results":[{"title":"<external_data>\nA &amp; B\n</external_data>","url":"https://a","description":"d","published":"2026-10-01"}]}`)}
	withBrave := lookupDef(t, webRegistry(t, StaticEnv{BraveSearchTool: {}, DDGSearchTool: {}}), TypeWebSearch)
	res, err := execDef(withBrave, map[string]any{"query": "ki", "count": 50.0}, &Services{Tools: brave})
	if err != nil {
		t.Fatalf("brave: %v", err)
	}
	want := []any{map[string]any{"title": "A & B", "url": "https://a", "snippet": "d", "published": "2026-10-01"}}
	if !reflect.DeepEqual(res.Output["results"], want) || res.Output["provider"] != "brave" || res.Output["count"] != 1.0 || res.ItemCount != 1 {
		t.Fatalf("brave output = %#v", res.Output)
	}
	if req := brave.last(t); req.Tool != BraveSearchTool || req.Args["count"] != 20 || req.Args["query"] != "ki" {
		t.Fatalf("brave request = %+v", req)
	}

	ddg := &fakeTools{respond: toolReply(`{"status":"success","results":[{"title":"T","link":"https://t","snippet":"s"}]}`)}
	onlyDDG := lookupDef(t, webRegistry(t, StaticEnv{DDGSearchTool: {}}), TypeWebSearch)
	res, err = execDef(onlyDDG, map[string]any{"query": "ki"}, &Services{Tools: ddg})
	if err != nil || res.Output["provider"] != "duckduckgo" {
		t.Fatalf("ddg = %#v, %v", res.Output, err)
	}
	if req := ddg.last(t); req.Tool != DDGSearchTool || req.Args["max_results"] != 5 {
		t.Fatalf("ddg request = %+v", req)
	}
	if r := res.Output["results"].([]any)[0].(map[string]any); r["url"] != "https://t" || r["snippet"] != "s" {
		t.Fatalf("ddg result = %#v", r)
	}

	ddg.respond = toolReply(`{"status":"error","message":"No results found for query"}`)
	res, err = execDef(onlyDDG, map[string]any{"query": "zzz"}, &Services{Tools: ddg})
	if err != nil || len(res.Output["results"].([]any)) != 0 {
		t.Fatalf("no results must be an empty list, got %#v, %v", res.Output, err)
	}
	if _, err := execDef(onlyDDG, map[string]any{"query": " "}, &Services{Tools: ddg}); asNodeError(err).Code != "FLOW_PARAM_INVALID" {
		t.Fatalf("empty query = %v", err)
	}
	if a := lookupDef(t, webRegistry(t, StaticEnv{BraveSearchTool: {}}), TypeWebSearch).Availability(); a.State != AvailableState {
		t.Fatalf("one provider is enough: %+v", a)
	}
}

func TestWebRead(t *testing.T) {
	tools := &fakeTools{respond: toolReply(`{"status":"success","mode":"rss","title":"Feed","content":"md","items":[{"title":"a"},{"title":"b"}]}`)}
	def := lookupDef(t, webRegistry(t, nil), TypeWebRead)
	res, err := execDef(def, map[string]any{"url": "https://x/feed", "mode": "rss"}, &Services{Tools: tools})
	if err != nil || res.Output["title"] != "Feed" || res.Output["content"] != "md" || len(res.Output["items"].([]any)) != 2 || res.ItemCount != 2 {
		t.Fatalf("read = %#v, %v", res.Output, err)
	}
	if args := tools.last(t).Args; args["url"] != "https://x/feed" || args["mode"] != "rss" {
		t.Fatalf("args = %#v", args)
	}
}

func TestHTTPRequest(t *testing.T) {
	tools := &fakeTools{respond: toolReply(`{"status":"success","status_code":200,"headers":{"content-type":"application/json"},"body":"{\"ok\":true}"}`)}
	def := lookupDef(t, webRegistry(t, nil), TypeHTTPRequest)
	svc := &Services{Tools: tools, Secrets: fakeSecrets{"api_token": "s3cret"}}
	res, err := execDef(def, map[string]any{"url": "https://api", "headers": map[string]any{"X-A": "1"}, "auth_secret": "api_token"}, svc)
	if err != nil || res.Output["status_code"] != 200.0 || res.Output["ok"] != true || !reflect.DeepEqual(res.Output["json"], map[string]any{"ok": true}) {
		t.Fatalf("GET = %#v, %v", res.Output, err)
	}
	args := tools.last(t).Args
	if args["method"] != "GET" || args["url"] != "https://api" || args["body"] != nil ||
		!reflect.DeepEqual(args["headers"], map[string]any{"X-A": "1", "Authorization": "Bearer s3cret"}) {
		t.Fatalf("GET args = %#v", args)
	}
	if _, err := execDef(def, map[string]any{"method": "POST", "url": "https://api", "body": map[string]any{"a": 1.0}}, svc); err != nil {
		t.Fatal(err)
	}
	if args := tools.last(t).Args; args["body"] != `{"a":1}` {
		t.Fatalf("POST body = %#v", args["body"])
	}

	tools.respond = toolReply(`{"status":"error","status_code":404,"body":"nope"}`)
	if _, err := execDef(def, map[string]any{"url": "https://api"}, svc); asNodeError(err).Code != "FLOW_HTTP_STATUS" {
		t.Fatalf("404 = %v", err)
	}
	res, err = execDef(def, map[string]any{"url": "https://api", "fail_on_error": false}, svc)
	if err != nil || res.Output["ok"] != false || res.Output["body"] != "nope" {
		t.Fatalf("404 tolerated = %#v, %v", res.Output, err)
	}
	if _, err := execDef(def, map[string]any{"url": "https://api", "auth_secret": "missing"}, svc); asNodeError(err).Code != "FLOW_SECRET_UNAVAILABLE" {
		t.Fatalf("missing secret = %v", err)
	}
	tools.respond = toolReply(`{"status":"error","message":"URL validation failed"}`)
	if _, err := execDef(def, map[string]any{"url": "http://127.0.0.1"}, svc); asNodeError(err).Code != "FLOW_TOOL_ERROR" {
		t.Fatalf("tool error = %v", err)
	}
	if len(def.EffectsOf(&Node{Params: map[string]any{"method": "GET"}})) != 0 ||
		!reflect.DeepEqual(def.EffectsOf(&Node{Params: map[string]any{"method": "POST"}}), []Effect{EffectSendsMessage}) {
		t.Fatal("HTTP effects mismatch")
	}
}
