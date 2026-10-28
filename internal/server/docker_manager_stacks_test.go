package server

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerstacks"
)

func TestDockerManagerStackReads(t *testing.T) {
	f := newDMStackFixture(t)
	f.fake.helpers = dockerstacks.HelperStatus{APIVersion: "1.45", APISupported: true, RunnerImage: dockerstacks.RunnerImageTag, IOContainer: "ok"}
	f.fake.listing = dockerstacks.Listing{
		HostID: "local", StacksDir: "/opt/stacks", Cached: true,
		Stacks: []dockerstacks.StackSummary{{Name: "web", HostID: "local", Status: dockerstacks.StatusRunning}},
	}
	f.fake.detail = dockerstacks.StackDetail{StackSummary: dockerstacks.StackSummary{Name: "web", HostID: "local"}}

	code, body := f.do(http.MethodGet, "/hosts/local/helpers", nil)
	if helpers := dmObject(t, body, "helpers"); code != http.StatusOK || helpers["api_version"] != "1.45" || helpers["io_container"] != "ok" {
		t.Fatalf("helpers = %d %s, want the helper status", code, f.lastRaw)
	}

	// The listing's members sit at the top level; empty lists encode as [].
	code, body = f.do(http.MethodGet, "/hosts/local/stacks?refresh=1", nil)
	if code != http.StatusOK || body["stacks_dir"] != "/opt/stacks" || body["cached"] != true || !f.fake.called("List local refresh") {
		t.Fatalf("list = %d %s, want the listing at top level and refresh passed on", code, f.lastRaw)
	}
	if stacks := dmArray(t, body, "stacks"); len(stacks) != 1 || stacks[0]["name"] != "web" {
		t.Fatalf("list stacks = %s, want web", f.lastRaw)
	}
	if len(dmArray(t, body, "external")) != 0 || len(dmArray(t, body, "ignored")) != 0 {
		t.Fatalf("list = %s, want external and ignored as empty arrays", f.lastRaw)
	}
	f.do(http.MethodGet, "/hosts/local/stacks", nil)
	if !f.fake.called("List local") {
		t.Fatal("a list without ?refresh=1 asked for a refresh")
	}

	code, body = f.do(http.MethodGet, "/hosts/local/stacks/web", nil)
	if stack := dmObject(t, body, "stack"); code != http.StatusOK || stack["name"] != "web" || !f.fake.called("Get local web") {
		t.Fatalf("get = %d %s, want the stack", code, f.lastRaw)
	}
	code, body = f.do(http.MethodGet, "/hosts/local/stacks/web/volumes", nil)
	if code != http.StatusOK || len(dmArray(t, body, "volumes")) != 0 || !f.fake.called("ProjectVolumes local web") {
		t.Fatalf("volumes = %d %s, want an empty volumes array", code, f.lastRaw)
	}

	// Failures keep their codes and fields.
	f.fake.errs["List"] = &dockerstacks.Error{Code: dockerstacks.CodeHostNotPrepared, Message: "not prepared", Fields: map[string]any{"helpers": f.fake.helpers}}
	code, body = f.do(http.MethodGet, "/hosts/local/stacks", nil)
	f.expectError(code, body, http.StatusConflict, "host_not_prepared")
	if dmObject(t, body, "helpers")["runner_image"] != dockerstacks.RunnerImageTag {
		t.Fatalf("host_not_prepared answer = %s, want the helper status", f.lastRaw)
	}
	f.fake.errs["Get"] = &dockerstacks.Error{Code: dockerstacks.CodeStackNotFound, Message: "no such stack"}
	code, body = f.do(http.MethodGet, "/hosts/local/stacks/nope", nil)
	f.expectError(code, body, http.StatusNotFound, "stack_not_found")
	f.fake.errs["Helpers"] = dockerhost.ErrHostNotFound
	code, body = f.do(http.MethodGet, "/hosts/h_000000000000/helpers", nil)
	f.expectError(code, body, http.StatusNotFound, "host_not_found")

	code, body = f.do(http.MethodPut, "/hosts/local/stacks", nil)
	f.expectError(code, body, http.StatusMethodNotAllowed, "method_not_allowed")
	code, body = f.do(http.MethodGet, "/hosts/local/stacks/web/unknown", nil)
	f.expectError(code, body, http.StatusNotFound, "not_found")
}

func TestDockerManagerStackRoutesAnswer503WithoutTheService(t *testing.T) {
	f := newDMStackFixture(t)
	f.noStacks = true
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/hosts/local/helpers"},
		{http.MethodPost, "/hosts/local/prepare"},
		{http.MethodGet, "/hosts/local/stacks"},
		{http.MethodPost, "/hosts/local/stacks"},
		{http.MethodGet, "/hosts/local/stacks/web"},
		{http.MethodDelete, "/hosts/local/stacks/web"},
		{http.MethodPut, "/hosts/local/stacks/web/files"},
		{http.MethodGet, "/hosts/local/stacks/web/volumes"},
		{http.MethodPost, "/hosts/local/stacks/web/ops"},
		{http.MethodPost, "/hosts/local/projects/shop/restart"},
	} {
		code, body := f.do(tc.method, tc.path, map[string]any{"name": "web", "compose": "x", "kind": "up"})
		f.expectError(code, body, http.StatusServiceUnavailable, "stacks_unavailable")
	}
}

func TestDockerManagerStackCreateAndDeploy(t *testing.T) {
	f := newDMStackFixture(t)
	f.fake.detail = dockerstacks.StackDetail{StackSummary: dockerstacks.StackSummary{Name: "web", HostID: "local", Status: dockerstacks.StatusDraft}}
	f.fake.operation = dockerstacks.Operation{ID: "op_0123456789abcdef", HostID: "local", Stack: "web", Kind: dockerstacks.KindUp, State: dockerstacks.StateQueued}

	code, body := f.do(http.MethodPost, "/hosts/local/stacks", map[string]any{"name": "web", "compose": "services: {}\n", "env": "A=1\n", "deploy": true})
	if code != http.StatusCreated || dmObject(t, body, "stack")["name"] != "web" || dmObject(t, body, "operation")["id"] != "op_0123456789abcdef" {
		t.Fatalf("create + deploy = %d %s, want 201 with the stack and the up operation", code, f.lastRaw)
	}
	if got := f.fake.create; got.Name != "web" || got.Compose != "services: {}\n" || got.Env == nil || *got.Env != "A=1\n" {
		t.Fatalf("service received %+v, want the posted name, compose and env", got)
	}
	if !f.fake.called("Start local web up") || f.fake.opts.Volumes {
		t.Fatalf("calls = %v, want the up operation without volumes", f.fake.calls)
	}

	// Without deploy nothing starts, and the answer has no operation; an absent env stays nil.
	f.fake.calls = nil
	code, body = f.do(http.MethodPost, "/hosts/local/stacks", map[string]any{"name": "web", "compose": "services: {}\n"})
	if code != http.StatusCreated || body["operation"] != nil || f.fake.create.Env != nil || f.fake.called("Start local web up") {
		t.Fatalf("create = %d %s (env %v), want a draft: no operation, no env", code, f.lastRaw, f.fake.create.Env)
	}

	// The files exist but the operation could not start: still a 201, with the reason.
	f.fake.errs["Start"] = &dockerstacks.BusyError{OperationID: "op_aaaaaaaaaaaaaaaa", Kind: dockerstacks.KindDelete}
	code, body = f.do(http.MethodPost, "/hosts/local/stacks", map[string]any{"name": "web", "compose": "x", "deploy": true})
	if deploy := dmObject(t, body, "deploy_error"); code != http.StatusCreated || deploy["code"] != "operation_in_progress" || body["operation"] != nil {
		t.Fatalf("create with a failing start = %d %s, want 201 with deploy_error", code, f.lastRaw)
	}
	f.fake.errs["Start"] = errors.New("database is locked")
	code, body = f.do(http.MethodPost, "/hosts/local/stacks", map[string]any{"name": "web", "compose": "x", "deploy": true})
	if deploy := dmObject(t, body, "deploy_error"); code != http.StatusCreated || deploy["code"] != "internal_error" || strings.Contains(f.lastRaw, "locked") {
		t.Fatalf("create with a broken journal = %d %s, want 201 with a generic deploy_error", code, f.lastRaw)
	}
	delete(f.fake.errs, "Start")

	f.fake.errs["Create"] = &dockerstacks.Error{Code: dockerstacks.CodeStackExists, Message: "exists"}
	code, body = f.do(http.MethodPost, "/hosts/local/stacks", map[string]any{"name": "web", "compose": "x"})
	f.expectError(code, body, http.StatusConflict, "stack_exists")
	delete(f.fake.errs, "Create")

	code, body = f.do(http.MethodPost, "/hosts/local/stacks", strings.Repeat("{", 3))
	f.expectError(code, body, http.StatusBadRequest, "bad_request")
	f.fake.calls = nil
	code, body = f.do(http.MethodPost, "/hosts/local/stacks", map[string]any{"name": "web", "compose": strings.Repeat("a", dmStackBodyLimit+1)})
	f.expectError(code, body, http.StatusRequestEntityTooLarge, "file_too_large")
	if f.fake.callCount() != 0 {
		t.Fatalf("calls = %v, an oversized body must not reach the service", f.fake.calls)
	}
}

func TestDockerManagerStackFilesOperationsAndDelete(t *testing.T) {
	f := newDMStackFixture(t)
	f.fake.operation = dockerstacks.Operation{ID: "op_0123456789abcdef", HostID: "local", Stack: "web", State: dockerstacks.StateQueued}
	f.fake.revisions = dockerstacks.FileRevisions{Compose: "rev-compose", Env: "rev-env"}

	// Files: both members reach the service with their base revisions.
	code, body := f.do(http.MethodPut, "/hosts/local/stacks/web/files", map[string]any{
		"compose": map[string]any{"content": "services: {}\n", "revision": "r1"},
		"env":     map[string]any{"content": "A=1\n", "revision": ""},
	})
	revisions := dmObject(t, body, "revisions")
	if code != http.StatusOK || revisions["compose"] != "rev-compose" || revisions["env"] != "rev-env" {
		t.Fatalf("files = %d %s, want the new revisions", code, f.lastRaw)
	}
	if got := f.fake.files; got.Compose == nil || got.Compose.Revision != "r1" || got.Env == nil || got.Env.Content != "A=1\n" {
		t.Fatalf("service received %+v, want both files with their revisions", got)
	}
	f.fake.errs["WriteFiles"] = &dockerstacks.ConflictError{File: "compose", Current: dockerstacks.FileContent{Name: "compose.yaml", Content: "on host", Revision: "r2", Exists: true}}
	code, body = f.do(http.MethodPut, "/hosts/local/stacks/web/files", map[string]any{"compose": map[string]any{"content": "mine", "revision": "r1"}})
	f.expectError(code, body, http.StatusConflict, "revision_conflict")
	if body["file"] != "compose" || dmObject(t, body, "current")["content"] != "on host" {
		t.Fatalf("conflict answer = %s, want the file and what is on the host", f.lastRaw)
	}
	delete(f.fake.errs, "WriteFiles")

	// Operations: kind and the volumes flag reach the service; delete has its own route.
	code, body = f.do(http.MethodPost, "/hosts/local/stacks/web/ops", map[string]any{"kind": "down", "volumes": true})
	if code != http.StatusAccepted || dmObject(t, body, "operation")["id"] != "op_0123456789abcdef" || f.fake.kind != "down" || !f.fake.opts.Volumes {
		t.Fatalf("ops = %d %s (kind %q, %+v), want 202 and a down with volumes", code, f.lastRaw, f.fake.kind, f.fake.opts)
	}
	f.fake.calls = nil
	code, body = f.do(http.MethodPost, "/hosts/local/stacks/web/ops", map[string]any{"kind": "delete"})
	f.expectError(code, body, http.StatusBadRequest, "invalid_operation")
	if f.fake.callCount() != 0 {
		t.Fatalf("calls = %v, ops must refuse delete before the service", f.fake.calls)
	}
	f.fake.errs["Start"] = &dockerstacks.BusyError{OperationID: "op_bbbbbbbbbbbbbbbb", Kind: "up"}
	code, body = f.do(http.MethodPost, "/hosts/local/stacks/web/ops", map[string]any{"kind": "restart"})
	f.expectError(code, body, http.StatusConflict, "operation_in_progress")
	if body["operation_id"] != "op_bbbbbbbbbbbbbbbb" || body["kind"] != "up" {
		t.Fatalf("busy answer = %s, want the running operation", f.lastRaw)
	}
	f.fake.errs["Start"] = errors.New("database is locked")
	code, body = f.do(http.MethodPost, "/hosts/local/stacks/web/ops", map[string]any{"kind": "up"})
	f.expectError(code, body, http.StatusInternalServerError, "internal_error")
	delete(f.fake.errs, "Start")

	// Delete: DELETE on the stack, volumes only on request.
	code, body = f.do(http.MethodDelete, "/hosts/local/stacks/web?volumes=1", nil)
	if code != http.StatusAccepted || f.fake.kind != dockerstacks.KindDelete || !f.fake.opts.Volumes || dmObject(t, body, "operation")["id"] == "" {
		t.Fatalf("delete = %d %s (kind %q, %+v), want 202 and a delete with volumes", code, f.lastRaw, f.fake.kind, f.fake.opts)
	}
	code, _ = f.do(http.MethodDelete, "/hosts/local/stacks/web", nil)
	if code != http.StatusAccepted || f.fake.opts.Volumes {
		t.Fatalf("delete without ?volumes = %d (%+v), want 202 and no volumes", code, f.fake.opts)
	}

	// Prepare.
	code, body = f.do(http.MethodPost, "/hosts/local/prepare", nil)
	if code != http.StatusAccepted || dmObject(t, body, "operation")["id"] != "op_0123456789abcdef" || !f.fake.called("Prepare local") {
		t.Fatalf("prepare = %d %s, want 202 with the operation", code, f.lastRaw)
	}
	f.fake.errs["Prepare"] = dockerhost.ErrHostNotFound
	code, body = f.do(http.MethodPost, "/hosts/h_000000000000/prepare", nil)
	f.expectError(code, body, http.StatusNotFound, "host_not_found")
}

func TestDockerManagerProjectActions(t *testing.T) {
	f := newDMStackFixture(t)
	for _, action := range []string{"start", "stop", "restart"} {
		code, body := f.do(http.MethodPost, "/hosts/local/projects/shop/"+action, nil)
		if code != http.StatusOK || body["status"] != "ok" || !f.fake.called("ProjectAction local shop "+action) {
			t.Fatalf("%s = %d %s, want 200 ok and the service call", action, code, f.lastRaw)
		}
	}
	code, body := f.do(http.MethodPost, "/hosts/local/projects/shop/pause", nil)
	f.expectError(code, body, http.StatusBadRequest, "invalid_action")
	code, body = f.do(http.MethodGet, "/hosts/local/projects/shop/start", nil)
	f.expectError(code, body, http.StatusMethodNotAllowed, "method_not_allowed")

	f.fake.errs["ProjectAction"] = &dockerstacks.BusyError{OperationID: "op_cccccccccccccccc", Kind: "up"}
	code, body = f.do(http.MethodPost, "/hosts/local/projects/shop/stop", nil)
	f.expectError(code, body, http.StatusConflict, "operation_in_progress")
	f.fake.errs["ProjectAction"] = &dockerstacks.Error{Code: dockerstacks.CodeStackNotFound, Message: "no such project"}
	code, body = f.do(http.MethodPost, "/hosts/local/projects/gone/stop", nil)
	f.expectError(code, body, http.StatusNotFound, "stack_not_found")
	f.fake.errs["ProjectAction"] = &dockerhost.StatusError{Status: 500, Err: errors.New("Docker API returned 500: boom")}
	code, body = f.do(http.MethodPost, "/hosts/local/projects/shop/stop", nil)
	f.expectError(code, body, http.StatusBadGateway, "docker_error")
}

func TestDockerManagerStackMutationsAreRefusedInReadOnlyMode(t *testing.T) {
	f := newDMStackFixture(t)
	f.readOnly = true
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/hosts/local/prepare"},
		{http.MethodPost, "/hosts/local/stacks"},
		{http.MethodPut, "/hosts/local/stacks/web/files"},
		{http.MethodPost, "/hosts/local/stacks/web/ops"},
		{http.MethodDelete, "/hosts/local/stacks/web"},
		{http.MethodPost, "/hosts/local/projects/shop/restart"},
	} {
		code, body := f.do(tc.method, tc.path, map[string]any{"name": "web", "compose": "x", "kind": "up"})
		f.expectError(code, body, http.StatusForbidden, "read_only")
	}
	if f.fake.callCount() != 0 {
		t.Fatalf("calls = %v, a refused mutation must not reach the service", f.fake.calls)
	}
	// Reads stay available.
	for _, path := range []string{"/hosts/local/helpers", "/hosts/local/stacks", "/hosts/local/stacks/web", "/hosts/local/stacks/web/volumes"} {
		if code, _ := f.do(http.MethodGet, path, nil); code != http.StatusOK {
			t.Fatalf("GET %s = %d %s in read-only mode, want 200", path, code, f.lastRaw)
		}
	}
}
