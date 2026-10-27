package server

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerstacks"
)

const dmLintCompose = "services:\n  web:\n    image: nginx:1.27\n    restart: always\n    ports:\n      - \"8080:80\"\n"

// dmDiagnosticCodes lists the codes of a lint answer.
func dmDiagnosticCodes(t *testing.T, body map[string]any) []string {
	t.Helper()
	var codes []string
	for _, d := range dmArray(t, body, "diagnostics") {
		codes = append(codes, d["code"].(string))
	}
	return codes
}

func TestDockerManagerLintReadsThePortsOfTheHost(t *testing.T) {
	f := newDMStackFixture(t)
	f.fake.ports = []dockerstacks.PublishedPort{{Port: 8080, Protocol: "tcp", Container: "other", Project: "shop"}}

	code, body := f.do(http.MethodPost, "/hosts/local/lint", map[string]any{"compose": dmLintCompose, "env": "", "stack": "web"})
	if code != http.StatusOK || !f.fake.called("PublishedPorts local web") || body["ports_unchecked"] != nil {
		t.Fatalf("lint = %d %s, want 200 with the ports of the host read for stack web", code, f.lastRaw)
	}
	if codes := dmDiagnosticCodes(t, body); len(codes) != 1 || codes[0] != "port_conflict" {
		t.Fatalf("diagnostics = %v, want only port_conflict", codes)
	}
	services := dmArray(t, dmObject(t, body, "preview"), "services")
	if ports, _ := services[0]["ports"].([]any); len(ports) != 1 || ports[0].(map[string]any)["conflict"] != "other" {
		t.Fatalf("preview = %s, want the conflicting container named on the port", f.lastRaw)
	}

	// A host that cannot be read: lint without port conflicts, and say so.
	f.fake.errs["PublishedPorts"] = errors.New("dial tcp 10.0.0.5:2375: connect: connection refused")
	code, body = f.do(http.MethodPost, "/hosts/local/lint", map[string]any{"compose": dmLintCompose})
	if code != http.StatusOK || body["ports_unchecked"] != true || len(dmDiagnosticCodes(t, body)) != 0 {
		t.Fatalf("lint on an unreachable host = %d %s, want 200, no diagnostics and ports_unchecked", code, f.lastRaw)
	}
	// So does a server whose stacks service is not open.
	f.noStacks = true
	code, body = f.do(http.MethodPost, "/hosts/local/lint", map[string]any{"compose": dmLintCompose})
	if code != http.StatusOK || body["ports_unchecked"] != true {
		t.Fatalf("lint without the stacks service = %d %s, want 200 with ports_unchecked", code, f.lastRaw)
	}
	f.noStacks = false

	// A wrong host id is the caller's mistake, not a missing ports check.
	f.fake.errs["PublishedPorts"] = dockerhost.ErrHostNotFound
	code, body = f.do(http.MethodPost, "/hosts/h_000000000000/lint", map[string]any{"compose": dmLintCompose})
	f.expectError(code, body, http.StatusNotFound, "host_not_found")
	delete(f.fake.errs, "PublishedPorts")

	// Syntax errors come back as diagnostics with their line, not as an HTTP error; lint changes nothing, so
	// read-only mode does not block it.
	f.readOnly = true
	code, body = f.do(http.MethodPost, "/hosts/local/lint", map[string]any{"compose": "a: [\n"})
	diagnostics := dmArray(t, body, "diagnostics")
	if code != http.StatusOK || len(diagnostics) != 1 || diagnostics[0]["code"] != "yaml_syntax" || diagnostics[0]["line"] != float64(1) {
		t.Fatalf("lint of broken YAML = %d %s, want one yaml_syntax diagnostic on line 1", code, f.lastRaw)
	}
	code, body = f.do(http.MethodGet, "/hosts/local/lint", nil)
	f.expectError(code, body, http.StatusMethodNotAllowed, "method_not_allowed")
	code, body = f.do(http.MethodPost, "/hosts/local/lint", strings.Repeat("{", 3))
	f.expectError(code, body, http.StatusBadRequest, "bad_request")
}

func TestDockerManagerConvertDockerRun(t *testing.T) {
	f := newDMStackFixture(t)
	f.readOnly = true // converting changes nothing
	f.noStacks = true // and needs no service

	code, body := f.do(http.MethodPost, "/convert/docker-run", map[string]any{"command": "docker run -d --name web --rm -p 8080:80 nginx:1.27"})
	if code != http.StatusOK || body["service"] != "web" || !strings.Contains(body["compose"].(string), "image: nginx:1.27") {
		t.Fatalf("convert = %d %s, want service web and the compose text", code, f.lastRaw)
	}
	warnings := dmArray(t, body, "warnings")
	if len(warnings) != 1 || warnings[0]["code"] != "flag_ignored" || warnings[0]["flag"] != "--rm" {
		t.Fatalf("warnings = %v, want --rm reported as ignored", warnings)
	}

	code, body = f.do(http.MethodPost, "/convert/docker-run", map[string]any{"command": "ls -la"})
	f.expectError(code, body, http.StatusBadRequest, "convert_failed")
	code, body = f.do(http.MethodPost, "/convert/docker-run", map[string]any{"command": "docker run " + strings.Repeat("a", dmConvertMaxBytes)})
	f.expectError(code, body, http.StatusBadRequest, "convert_failed")
	code, body = f.do(http.MethodGet, "/convert/docker-run", nil)
	f.expectError(code, body, http.StatusMethodNotAllowed, "method_not_allowed")
	code, body = f.do(http.MethodPost, "/convert/other", nil)
	f.expectError(code, body, http.StatusNotFound, "not_found")
}
