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
	"aurago/internal/dockerstacks"
)

func TestDockerManagerStackFailureMapping(t *testing.T) {
	engine := func(status int) error {
		return &dockerhost.StatusError{Status: status, Err: fmt.Errorf("Docker API returned %d: engine said no", status)}
	}
	coded := func(code string, fields map[string]any) error {
		return fmt.Errorf("list stacks: %w", &dockerstacks.Error{Code: code, Message: "message of " + code, Fields: fields})
	}
	helpers := dockerstacks.HelperStatus{APIVersion: "1.45", APISupported: true, RunnerImage: dockerstacks.RunnerImageTag, IOContainer: "missing"}
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
		field  string // a member the answer must carry
	}{
		{"name invalid", coded(dockerstacks.CodeStackNameInvalid, nil), http.StatusBadRequest, "stack_name_invalid", ""},
		{"stack missing", coded(dockerstacks.CodeStackNotFound, nil), http.StatusNotFound, "stack_not_found", ""},
		{"stack exists", coded(dockerstacks.CodeStackExists, nil), http.StatusConflict, "stack_exists", ""},
		{"operation missing", coded(dockerstacks.CodeOperationNotFound, nil), http.StatusNotFound, "operation_not_found", ""},
		{"invalid operation", coded(dockerstacks.CodeInvalidOperation, nil), http.StatusBadRequest, "invalid_operation", ""},
		{"file too large", coded(dockerstacks.CodeFileTooLarge, nil), http.StatusRequestEntityTooLarge, "file_too_large", ""},
		{"host not prepared", coded(dockerstacks.CodeHostNotPrepared, map[string]any{"helpers": helpers}), http.StatusConflict, "host_not_prepared", "helpers"},
		{"runner image missing", coded(dockerstacks.CodeRunnerImageMissing, map[string]any{"command": "docker pull " + dockerstacks.RunnerImageTag}), http.StatusConflict, "runner_image_missing", "command"},
		{"api too old", coded(dockerstacks.CodeAPITooOld, map[string]any{"api_version": "1.40"}), http.StatusConflict, "api_too_old", "api_version"},
		{"stacks dir", coded(dockerstacks.CodeStacksDirInvalid, nil), http.StatusConflict, "stacks_dir_unavailable", ""},
		{"project conflict", coded(dockerstacks.CodeProjectConflict, map[string]any{"working_dir": "/home/andi/web"}), http.StatusConflict, "project_conflict", "working_dir"},
		{"convert failed", coded(dockerstacks.CodeConvertFailed, nil), http.StatusBadRequest, "convert_failed", ""},
		{"revision conflict", fmt.Errorf("write: %w", &dockerstacks.ConflictError{File: "compose", Current: dockerstacks.FileContent{Name: "compose.yaml", Content: "services: {}", Revision: "abc", Exists: true}}), http.StatusConflict, "revision_conflict", "current"},
		{"busy", &dockerstacks.BusyError{OperationID: "op_0123456789abcdef", Kind: "up"}, http.StatusConflict, "operation_in_progress", "operation_id"},
		{"unknown host", dockerhost.ErrHostNotFound, http.StatusNotFound, "host_not_found", ""},
		{"engine refused", fmt.Errorf("list: %w", engine(500)), http.StatusBadGateway, "docker_error", ""},
		{"nothing answered", errors.New("dial tcp 10.0.0.5:2375: connect: connection refused"), http.StatusBadGateway, "host_unreachable", "reason"},
		{"unknown code", coded("brand_new_code", nil), http.StatusInternalServerError, "brand_new_code", ""},
	} {
		rec := httptest.NewRecorder()
		(&dmRequest{s: &Server{}, w: rec}).stackFailure(tc.err)
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: answer %q is not JSON: %v", tc.name, rec.Body.String(), err)
		}
		if rec.Code != tc.status || body["status"] != "error" || body["code"] != tc.code {
			t.Errorf("%s: answer = %d %s, want %d error %q", tc.name, rec.Code, rec.Body.String(), tc.status, tc.code)
		}
		if tc.field != "" && body[tc.field] == nil {
			t.Errorf("%s: answer %s lacks %q", tc.name, rec.Body.String(), tc.field)
		}
	}
}

func TestDockerManagerStackFailureFieldsKeepTheEnvelope(t *testing.T) {
	err := &dockerstacks.Error{Code: dockerstacks.CodeHostNotPrepared, Message: "not prepared", Fields: map[string]any{
		"status": "ok", "code": "other", "message": "other", "helpers": dockerstacks.HelperStatus{},
	}}
	rec := httptest.NewRecorder()
	(&dmRequest{s: &Server{}, w: rec}).stackFailure(err)
	var body map[string]any
	if json.Unmarshal(rec.Body.Bytes(), &body) != nil || body["status"] != "error" || body["code"] != "host_not_prepared" || body["message"] != "not prepared" {
		t.Fatalf("answer = %s, want the envelope members of the error, not its fields", rec.Body.String())
	}
}

func TestDockerManagerRevisionConflictCarriesTheHostFile(t *testing.T) {
	conflict := &dockerstacks.ConflictError{File: "env", Current: dockerstacks.FileContent{Name: ".env", Content: "A=1\n", Revision: "r1", Exists: true}}
	rec := httptest.NewRecorder()
	(&dmRequest{s: &Server{}, w: rec}).stackFailure(conflict)
	var body struct {
		File    string                   `json:"file"`
		Current dockerstacks.FileContent `json:"current"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.File != "env" || body.Current != conflict.Current {
		t.Fatalf("answer = %s (%v), want file env and the current .env content", rec.Body.String(), err)
	}
}

func TestDockerManagerOperationFailureIsNeverAnUnreachableHost(t *testing.T) {
	rec := httptest.NewRecorder()
	(&dmRequest{s: &Server{}, w: rec}).operationFailure(errors.New("database is locked"))
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), `"internal_error"`) {
		t.Fatalf("answer = %d %s, want 500 internal_error", rec.Code, rec.Body.String())
	}
}
