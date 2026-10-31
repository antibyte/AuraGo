package server

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestKaggleSSEReplayAndCredentialRevocation(t *testing.T) {
	s, _, _ := testDesktopPermissionServer(t)
	s.Cfg.Directories.DataDir = t.TempDir()
	s.Cfg.VirtualDesktop.Enabled = true
	s.Cfg.Kaggle = config.DefaultKaggleConfig()
	b, err := s.ensureKaggleServices()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.closeKaggleServices)
	first, err := b.Platform.Store.Emit(context.Background(), "cell", map[string]string{"execution_id": "before-cursor"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = b.Platform.Store.Emit(context.Background(), "cell", map[string]string{"execution_id": "after-cursor"}); err != nil {
		t.Fatal(err)
	}
	token, meta, err := s.TokenManager.Create("Kaggle SSE fixture", []string{"desktop:read"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	origin := httptest.NewServer(http.HandlerFunc(s.handleKaggle))
	defer origin.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r, _ := http.NewRequestWithContext(ctx, "GET", origin.URL+"/api/kaggle/events", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Last-Event-ID", strconv.FormatInt(first, 10))
	response, err := origin.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal(response.Status)
	}
	scan := bufio.NewScanner(response.Body)
	found := false
	for scan.Scan() {
		line := scan.Text()
		if strings.Contains(line, "before-cursor") {
			t.Fatal("replayed an already consumed event")
		}
		if strings.Contains(line, "after-cursor") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("missing replayed event", scan.Err())
	}
	if !s.kaggleStreamAuthorized(r) {
		t.Fatal("valid stream credential rejected")
	}
	if err = s.TokenManager.Delete(meta.ID); err != nil {
		t.Fatal(err)
	}
	if s.kaggleStreamAuthorized(r) {
		t.Fatal("revoked stream credential remains valid")
	}
}

func TestKaggleHTTPScopesNotebookConflictsAndVault(t *testing.T) {
	s, read, write := testDesktopPermissionServer(t)
	s.Cfg.Directories.DataDir = t.TempDir()
	s.Cfg.VirtualDesktop.Enabled = true
	s.Cfg.Kaggle = config.DefaultKaggleConfig()
	s.Cfg.Kaggle.Enabled = true
	s.Cfg.Agent.AllowFilesystemWrite = true
	s.Cfg.Agent.AllowNetworkRequests = true
	t.Cleanup(s.closeKaggleServices)
	call := func(method, path, token, body, revision string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/kaggle/"+path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if revision == "*" {
			r.Header.Set("If-None-Match", "*")
		} else if revision != "" {
			r.Header.Set("If-Match", revision)
		}
		w := httptest.NewRecorder()
		s.handleKaggle(w, r)
		return w
	}
	if got := call("GET", "status", "", "", ""); got.Code != 401 {
		t.Fatal(got.Code)
	}
	if got := call("POST", "runtime", read, `{"operation":"status"}`, ""); got.Code != 200 {
		t.Fatalf("semantic read POST: %d %s", got.Code, got.Body.String())
	}
	if got := call("POST", "runtime", read, `{"operation":"create_project","title":"no"}`, ""); got.Code != 403 {
		t.Fatal("read scope admitted creation", got.Code)
	}
	created := call("POST", "projects", write, `{"title":"Notebook"}`, "")
	if created.Code != 200 {
		t.Fatal(created.Code, created.Body.String())
	}
	var project struct {
		ID string `json:"id"`
	}
	json.Unmarshal(created.Body.Bytes(), &project)
	path := "projects/" + project.ID + "/notebooks/test.ipynb"
	notebook := `{"nbformat":4,"nbformat_minor":5,"metadata":{"unknown":{"keep":true}},"cells":[{"id":"one","cell_type":"code","source":"x=41","metadata":{},"outputs":[],"execution_count":null}]}`
	saved := call("PUT", path, write, notebook, "*")
	if saved.Code != 200 {
		t.Fatal(saved.Code, saved.Body.String())
	}
	if got := call("PUT", path, write, notebook, "*"); got.Code != 409 {
		t.Fatal("create collision", got.Code)
	}
	if got := call("PUT", path, write, notebook, `"stale"`); got.Code != 409 {
		t.Fatal("stale revision overwrite", got.Code)
	}
	if got := call("GET", path, read, "", ""); got.Code != 200 || !strings.Contains(got.Body.String(), "unknown") {
		t.Fatal("roundtrip", got.Code)
	}
	s.Cfg.VirtualDesktop.ReadOnly = true
	if got := call("PUT", path, write, notebook, saved.Header().Get("ETag")); got.Code != 403 {
		t.Fatal("desktop readonly bypass", got.Code)
	}
	if got := call("POST", "runtime", write, `{"operation":"start","project_id":"`+project.ID+`","file":"test.ipynb"}`, ""); got.Code != 403 {
		t.Fatal("readonly kernel start", got.Code)
	}
	admin, _, err := s.TokenManager.Create("Kaggle fixture", []string{"admin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := call("POST", "credentials", write, `{"token":"fixture-kaggle-secret"}`, ""); got.Code != 403 {
		t.Fatal("non-admin credentials", got.Code)
	}
	if got := call("POST", "credentials", admin, `{"token":"fixture-kaggle-secret"}`, ""); got.Code != 200 || strings.Contains(got.Body.String(), "fixture-kaggle-secret") {
		t.Fatal("credential response", got.Code)
	}
	token, err := s.Vault.ReadSecret("kaggle_api_token")
	if err != nil || token != "fixture-kaggle-secret" {
		t.Fatal("Vault persistence")
	}
	if got := call("GET", "status", read, "", ""); strings.Contains(got.Body.String(), token) {
		t.Fatal("status leaked credentials")
	}
	if got := call("POST", "credentials", admin, `{"token":""}`, ""); got.Code != 200 {
		t.Fatal("explicit secret removal", got.Code)
	}
}

func TestKaggleHTTPGrantMatrixDoesNotContactRemote(t *testing.T) {
	s, read, write := testDesktopPermissionServer(t)
	s.Cfg.Directories.DataDir = t.TempDir()
	s.Cfg.Kaggle = config.DefaultKaggleConfig()
	s.Cfg.Kaggle.Enabled = true
	s.Cfg.Agent.AllowFilesystemWrite = true
	s.Cfg.Agent.AllowNetworkRequests = true
	s.Vault.WriteSecret("kaggle_api_token", "fixture-denied-token")
	t.Cleanup(s.closeKaggleServices)
	for _, tc := range []struct{ domain, operation string }{
		{"datasets", "create_dataset"}, {"datasets", "delete_dataset"}, {"models", "create_model"}, {"notebooks", "save_kernel"}, {"competitions", "create_submission"}, {"benchmarks", "batch_schedule_benchmark_task_runs"},
	} {
		t.Run(tc.operation, func(t *testing.T) {
			body := `{"domain":"` + tc.domain + `","operation":"` + tc.operation + `","parameters":{}}`
			for _, token := range []string{read, write} {
				r := httptest.NewRequest("POST", "/api/kaggle/operations", strings.NewReader(body))
				r.Header.Set("Authorization", "Bearer "+token)
				w := httptest.NewRecorder()
				s.handleKaggle(w, r)
				if w.Code < 400 {
					t.Fatal("mutation admitted", w.Body.String())
				}
				if token == write && !strings.Contains(w.Body.String(), "read-only") {
					t.Fatal("did not fail locally", w.Body.String())
				}
			}
		})
	}
}
