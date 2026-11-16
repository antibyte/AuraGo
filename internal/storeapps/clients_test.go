package storeapps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNodeREDProjectsFlowsAndTriggersOnlyExistingPayload(t *testing.T) {
	triggers := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer node-token" {
			t.Error("missing credential")
		}
		switch r.URL.Path {
		case "/auth/login":
			fmt.Fprint(w, `{"type":"credentials"}`)
		case "/flows":
			fmt.Fprint(w, `[{"id":"flow","type":"tab","label":"Lights"},{"id":"allowed","type":"inject","z":"flow","name":"Start","payload":"hidden-payload"},{"id":"other","type":"inject","z":"flow"},{"id":"disabled","type":"inject","z":"flow","d":true},{"id":"secret","type":"function","z":"flow","func":"sensitive-code"}]`)
		case "/inject/allowed":
			triggers++
			body, _ := io.ReadAll(r.Body)
			if r.Method != "POST" || len(body) != 0 {
				t.Error("injected a payload override")
			}
			fmt.Fprint(w, "OK")
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	c := Connection{URL: server.URL, Credential: "node-token", AllowedIDs: []string{"allowed", "disabled"}}
	if err := c.NodeREDProbe(t.Context()); err != nil || triggers != 0 {
		t.Fatalf("probe performed an action: %v", err)
	}
	flows, err := c.NodeREDFlows(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(flows)
	if len(flows) != 1 || len(flows[0].Injects) != 1 || strings.Contains(string(data), "hidden-payload") || strings.Contains(string(data), "sensitive-code") {
		t.Fatal("flow projection leaked configuration")
	}
	for _, id := range []string{"other", "disabled", "../allowed"} {
		if _, err := c.NodeREDTrigger(t.Context(), id); err == nil {
			t.Fatalf("triggered %q", id)
		}
	}
	result, err := c.NodeREDTrigger(t.Context(), "allowed")
	if err != nil || triggers != 1 || result.(map[string]any)["completed"] != false {
		t.Fatalf("trigger receipt: %v %v", result, err)
	}
}

func TestOliveTinValidatesDefinitionsAndExecutionOwnership(t *testing.T) {
	starts, validations := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer olive-token" {
			t.Error("invalid Connect request")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		switch strings.TrimPrefix(r.URL.Path, oliveTinAPI) {
		case "Init":
			fmt.Fprint(w, `{"authenticatedUser":"automation","currentVersion":"3"}`)
		case "GetActionBinding":
			fmt.Fprint(w, `{"action":{"bindingId":"backup","title":"Backup","canExec":true,"arguments":[{"name":"target","type":"ascii","defaultValue":"hidden-default","choices":[{"value":"local","title":"Local"}]}]}}`)
		case "ValidateArgumentType":
			validations++
			fmt.Fprint(w, `{"valid":true}`)
		case "StartAction":
			starts++
			args := body["arguments"].([]any)
			if len(args) != 1 || args[0].(map[string]any)["value"] != "local" {
				t.Error("changed action arguments")
			}
			fmt.Fprint(w, `{"executionTrackingId":"execution"}`)
		case "ExecutionStatus":
			fmt.Fprint(w, `{"logEntry":{"bindingId":"backup","executionTrackingId":"execution","executionStarted":true,"executionFinished":false}}`)
		default:
			t.Error("arbitrary RPC requested")
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	c := Connection{URL: server.URL, Credential: "olive-token", AllowedIDs: []string{"backup"}}
	if err := c.OliveTinProbe(t.Context()); err != nil {
		t.Fatal(err)
	}
	listed, err := c.OliveTinActions(t.Context(), 0, 25)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(listed)
	if strings.Contains(string(raw), "hidden-default") || starts != 0 {
		t.Fatal("listing exposed defaults or started an action")
	}
	for _, args := range [][]OliveTinArgument{{{Name: "target", Value: "elsewhere"}}, {{Name: "target", Value: "local"}, {Name: "shell", Value: "command"}}} {
		if _, err := c.OliveTinExecute(t.Context(), "backup", args); err == nil {
			t.Fatal("invalid arguments accepted")
		}
	}
	if _, err := c.OliveTinExecute(t.Context(), "other", nil); err == nil || starts != 0 {
		t.Fatal("ungranted action reached API")
	}
	if _, err := c.OliveTinExecute(t.Context(), "backup", []OliveTinArgument{{Name: "target", Value: "local"}}); err != nil || starts != 1 || validations < 1 {
		t.Fatalf("execute: %v", err)
	}
	if _, err := c.OliveTinStatus(t.Context(), "backup", "other-execution"); err == nil {
		t.Fatal("foreign execution accepted")
	}
	if _, err := c.OliveTinStatus(t.Context(), "backup", "execution"); err != nil {
		t.Fatal(err)
	}
}

func TestStoreAppMutationIsNotReplayedAfterDisconnect(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = conn.Close()
	}))
	defer server.Close()
	c := Connection{URL: server.URL, Credential: "token"}
	if err := c.request(context.Background(), "POST", "/action", map[string]string{"input": "value"}, true, nil); err != ErrUncertain || calls != 1 {
		t.Fatalf("uncertain action: %v, calls=%d", err, calls)
	}
}
