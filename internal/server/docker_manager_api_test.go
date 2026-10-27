package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/dockerhost"
)

func TestDockerManagerEngineFailureMapping(t *testing.T) {
	engine := func(status int) error {
		return &dockerhost.StatusError{Status: status, Err: fmt.Errorf("Docker API returned %d: engine said no", status)}
	}
	for _, tc := range []struct {
		name     string
		err      error
		notFound string
		status   int
		code     string
	}{
		{"engine 404 with a not-found code", engine(404), "container_not_found", http.StatusNotFound, "container_not_found"},
		{"engine 404 without a code", engine(404), "", http.StatusBadGateway, "docker_error"},
		{"engine 409", engine(409), "container_not_found", http.StatusBadGateway, "docker_error"},
		{"wrapped engine status", fmt.Errorf("stop: %w", engine(500)), "", http.StatusBadGateway, "docker_error"},
		{"nothing answered", errors.New("dial tcp 10.0.0.5:2375: connect: connection refused"), "", http.StatusBadGateway, "host_unreachable"},
		{"rejected action", dockerhost.ErrInvalidAction, "", http.StatusBadRequest, "invalid_action"},
		{"rejected container reference", fmt.Errorf("inspect: %w", dockerhost.ErrInvalidContainerRef), "", http.StatusBadRequest, "bad_request"},
	} {
		rec := httptest.NewRecorder()
		(&dmRequest{w: rec}).engineFailure(tc.err, tc.notFound)
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: answer %q is not JSON: %v", tc.name, rec.Body.String(), err)
		}
		if rec.Code != tc.status || body["status"] != "error" || body["code"] != tc.code {
			t.Errorf("%s: answer = %d %s, want %d error %q", tc.name, rec.Code, rec.Body.String(), tc.status, tc.code)
		}
		if reason, _ := body["reason"].(string); tc.code == "host_unreachable" && reason == "" {
			t.Errorf("%s: answer %s carries no reason", tc.name, rec.Body.String())
		}
	}
}

func TestDockerManagerWithoutRegistryAnswers503(t *testing.T) {
	s := testContainerServer(true, false) // no initDockerHosts: the store could not be opened
	rec := httptest.NewRecorder()
	handleDockerManager(s)(rec, httptest.NewRequest(http.MethodGet, "/api/docker-manager/hosts", nil))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "docker_hosts_unavailable") {
		t.Fatalf("answer = %d %s, want 503 docker_hosts_unavailable", rec.Code, rec.Body.String())
	}
}

func TestDockerManagerDisabledAnswers503(t *testing.T) {
	f := newDMFixture(t)
	f.setDocker(false, false)
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/hosts"},
		{http.MethodPost, "/hosts/test"},
		{http.MethodGet, "/nests"},
		{http.MethodGet, "/hosts/local/containers"},
		{http.MethodDelete, "/hosts/local/containers/web1"},
	} {
		code, body := f.do(tc.method, tc.path, nil)
		dmExpectError(t, f, code, body, http.StatusServiceUnavailable, "docker_disabled")
	}
}

func TestDockerManagerUnknownRoutesAnswerJSON(t *testing.T) {
	f := newDMFixture(t)
	for _, path := range []string{"/", "/nope", "/hosts/local/unknown", "/hosts//containers"} {
		code, body := f.do(http.MethodGet, path, nil)
		dmExpectError(t, f, code, body, http.StatusNotFound, "not_found")
	}
}
