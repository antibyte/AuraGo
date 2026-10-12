package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

func TestHuggingFacePolicyAllowsPublicReadsWithoutToken(t *testing.T) {
	cfg := config.HuggingFaceConfig{
		Enabled:        true,
		ReadOnly:       true,
		MaxDatasetRows: 100,
	}
	req := HuggingFaceRequest{Operation: "search_models", Query: "bert", Limit: 5}

	if err := EvaluateHuggingFacePolicy(cfg, req, ""); err != nil {
		t.Fatalf("EvaluateHuggingFacePolicy() error = %v", err)
	}
}

func TestHuggingFacePolicyBlocksWritesWhenReadOnly(t *testing.T) {
	cfg := config.HuggingFaceConfig{Enabled: true, ReadOnly: true, AllowWrites: true}
	req := HuggingFaceRequest{Operation: "create_repo", RepoID: "owner/repo"}

	err := EvaluateHuggingFacePolicy(cfg, req, "hf_token")
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("expected read-only error, got %v", err)
	}
}

func TestHuggingFacePolicyBlocksJobsByDefault(t *testing.T) {
	cfg := config.HuggingFaceConfig{
		Enabled:         true,
		ReadOnly:        false,
		AllowedHardware: []string{"cpu-basic"},
	}
	req := HuggingFaceRequest{Operation: "job_run_python", Hardware: "cpu-basic"}

	err := EvaluateHuggingFacePolicy(cfg, req, "hf_token")
	if err == nil || !strings.Contains(err.Error(), "allow_jobs") {
		t.Fatalf("expected allow_jobs error, got %v", err)
	}
}

func TestHuggingFacePolicyAllowsAuthenticatedJobReadsInReadOnlyMode(t *testing.T) {
	cfg := config.HuggingFaceConfig{Enabled: true, ReadOnly: true, AllowJobs: true, AllowedHardware: []string{"a10g-small"}}
	for _, operation := range []string{"jobs_list", "job_get", "job_logs"} {
		if err := EvaluateHuggingFacePolicy(cfg, HuggingFaceRequest{Operation: operation}, "hf_token"); err != nil {
			t.Fatalf("%s blocked in read-only mode: %v", operation, err)
		}
		if err := EvaluateHuggingFacePolicy(cfg, HuggingFaceRequest{Operation: operation}, ""); err == nil {
			t.Fatalf("%s must require a token", operation)
		}
	}
}

func TestRunHuggingFacePassesConfiguredJobNamespace(t *testing.T) {
	var sawNamespace bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawNamespace = r.URL.Path == "/api/jobs/team"
		if r.Header.Get("Authorization") != "Bearer hf_token" {
			t.Fatalf("missing bearer token")
		}
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()
	cfg := config.HuggingFaceConfig{
		Enabled: true, ReadOnly: true, AllowJobs: true,
		JobsBaseURL: server.URL + "/api/jobs", JobNamespace: "team",
	}
	output := RunHuggingFace(context.Background(), cfg, "hf_token", t.TempDir(), t.TempDir(), HuggingFaceRequest{Operation: "jobs_list"})
	if !sawNamespace || !strings.Contains(output, `"status":"success"`) {
		t.Fatalf("namespace was not passed to Jobs client: path=%v output=%s", sawNamespace, output)
	}
}

func TestHuggingFacePolicyKeepsJobMutationsBehindReadOnly(t *testing.T) {
	cfg := config.HuggingFaceConfig{Enabled: true, ReadOnly: true, AllowJobs: true, AllowedHardware: []string{"cpu-basic"}}
	for _, operation := range []string{"job_run_python", "job_cancel"} {
		err := EvaluateHuggingFacePolicy(cfg, HuggingFaceRequest{Operation: operation}, "hf_token")
		if err == nil || !strings.Contains(err.Error(), "read-only") {
			t.Fatalf("%s should be blocked in read-only mode: %v", operation, err)
		}
	}
}

func TestHuggingFacePolicyDoesNotApplyHardwareGateToCancel(t *testing.T) {
	cfg := config.HuggingFaceConfig{Enabled: true, ReadOnly: false, AllowJobs: true, AllowedHardware: []string{"a10g-small"}}
	if err := EvaluateHuggingFacePolicy(cfg, HuggingFaceRequest{Operation: "job_cancel"}, "hf_token"); err != nil {
		t.Fatalf("job_cancel unexpectedly used the hardware gate: %v", err)
	}
}

func TestHuggingFacePolicyRequiresTwoPartTokenInjectionOptIn(t *testing.T) {
	cfg := config.HuggingFaceConfig{
		Enabled: true, ReadOnly: false, AllowJobs: true, AllowedHardware: []string{"cpu-basic"},
	}
	req := HuggingFaceRequest{Operation: "job_run_python", Script: "print('ok')", InjectToken: true}
	if err := EvaluateHuggingFacePolicy(cfg, req, "hf_token"); err == nil || !strings.Contains(err.Error(), "allow_job_token_injection") {
		t.Fatalf("expected config token-injection gate, got %v", err)
	}
	cfg.AllowJobTokenInjection = true
	if err := EvaluateHuggingFacePolicy(cfg, req, "hf_token"); err != nil {
		t.Fatalf("two-part token opt-in was rejected: %v", err)
	}
	withToken := hfJobRunOptions(cfg, req, "hf_token")
	if withToken.Secrets["HF_TOKEN"] != "hf_token" {
		t.Fatalf("HF_TOKEN was not injected after both opt-ins: %#v", withToken.Secrets)
	}
	withoutToken := hfJobRunOptions(cfg, HuggingFaceRequest{}, "hf_token")
	if len(withoutToken.Secrets) != 0 {
		t.Fatalf("HF_TOKEN was injected without request opt-in: %#v", withoutToken.Secrets)
	}
}

func TestHuggingFacePolicyRequiresHardwareAllowlist(t *testing.T) {
	cfg := config.HuggingFaceConfig{
		Enabled:         true,
		ReadOnly:        false,
		AllowJobs:       true,
		AllowedHardware: []string{"cpu-basic"},
	}
	req := HuggingFaceRequest{Operation: "job_run_container", Hardware: "a10g-small"}

	err := EvaluateHuggingFacePolicy(cfg, req, "hf_token")
	if err == nil || !strings.Contains(err.Error(), "hardware") {
		t.Fatalf("expected hardware allowlist error, got %v", err)
	}
}

func TestHuggingFacePolicyRejectsOversizedDatasetRows(t *testing.T) {
	cfg := config.HuggingFaceConfig{Enabled: true, ReadOnly: true, MaxDatasetRows: 50}
	req := HuggingFaceRequest{Operation: "dataset_rows", Dataset: "org/data", Split: "train", Length: 100}

	err := EvaluateHuggingFacePolicy(cfg, req, "")
	if err == nil || !strings.Contains(err.Error(), "max_dataset_rows") {
		t.Fatalf("expected max_dataset_rows error, got %v", err)
	}
}

func TestHuggingFaceWorkspacePathsRejectEscapes(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "agent_workspace", "workdir")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	outside := filepath.Join(root, "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatalf("write outside fixture: %v", err)
	}
	inside := filepath.Join(workspace, "inside.txt")
	if err := os.WriteFile(inside, []byte("workspace"), 0o600); err != nil {
		t.Fatalf("write inside fixture: %v", err)
	}

	if _, err := validateHuggingFaceUploadSource(workspace, outside, 1); err == nil {
		t.Fatal("expected absolute path outside workspace to be rejected")
	}
	if _, err := validateHuggingFaceUploadSource(workspace, inside, 1); err == nil {
		t.Fatal("expected absolute path inside workspace to be rejected")
	}
	if _, err := resolveHuggingFaceWorkspaceFile(workspace, filepath.Join("..", "..", "outside.txt")); err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
}

func TestHuggingFaceWorkspacePathsRejectSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "agent_workspace", "workdir")
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	link := filepath.Join(workspace, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := resolveHuggingFaceWorkspaceFile(workspace, filepath.Join("link", "file.txt")); err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}

func TestHuggingFaceUploadSourceRejectsDirectoriesAndSize(t *testing.T) {
	workspace := t.TempDir()
	if _, err := validateHuggingFaceUploadSource(workspace, ".", 1); err == nil {
		t.Fatal("expected directory upload to be rejected")
	}
	large := filepath.Join(workspace, "large.bin")
	file, err := os.Create(large)
	if err != nil {
		t.Fatalf("create large fixture: %v", err)
	}
	if err := file.Truncate(1024*1024 + 1); err != nil {
		_ = file.Close()
		t.Fatalf("truncate large fixture: %v", err)
	}
	_ = file.Close()
	if _, err := validateHuggingFaceUploadSource(workspace, "large.bin", 1); err == nil {
		t.Fatal("expected one MB upload limit to reject fixture")
	}
	if _, err := validateHuggingFaceUploadSource(workspace, "large.bin", 0); err != nil {
		t.Fatalf("unexpected default upload limit error: %v", err)
	}
}

func TestHuggingFaceLedgerFailureProducesSafeWarning(t *testing.T) {
	blockingFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blockingFile, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := map[string]interface{}{"id": "job-1"}
	err := recordHuggingFaceJob(context.Background(), blockingFile, "job_run_python", "cpu-basic", HuggingFaceRequest{}, result, nil)
	if err == nil {
		t.Fatal("expected local ledger failure")
	}
	attachHuggingFaceLedgerWarning(result, err)
	if result["ledger_warning"] == nil || strings.Contains(result["ledger_warning"].(string), blockingFile) {
		t.Fatalf("unsafe or missing ledger warning: %#v", result)
	}
}

func TestHuggingFaceLedgerScrubsProviderResponse(t *testing.T) {
	const token = "hf_audit_token_0123456789"
	security.RegisterSensitive(token)

	t.Run("response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/jobs/team" || r.Header.Get("Authorization") != "Bearer "+token {
				t.Fatalf("request = %s %s auth=%q", r.Method, r.URL.Path, r.Header.Get("Authorization"))
			}
			_, _ = w.Write([]byte(`{"id":"job-1","note":"` + token + `"}`))
		}))
		defer server.Close()
		dataDir := t.TempDir()
		out := runHuggingFaceJobFixture(t, server.URL, dataDir, token)
		if !strings.Contains(out, `"status":"success"`) {
			t.Fatalf("output = %s", out)
		}
		job := mustHuggingFaceLedgerJob(t, dataDir)
		if strings.Contains(job.ResponseJSON, token) || !strings.Contains(job.ResponseJSON, "[redacted]") || strings.Contains(job.LastError, token) || strings.Contains(job.RequestJSON, "secret-script") {
			t.Fatalf("ledger leaked: %#v", job)
		}
	})

	t.Run("error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"` + token + `"}`))
		}))
		defer server.Close()
		dataDir := t.TempDir()
		out := runHuggingFaceJobFixture(t, server.URL, dataDir, token)
		if !strings.Contains(out, `"status":"error"`) {
			t.Fatalf("output = %s", out)
		}
		job := mustHuggingFaceLedgerJob(t, dataDir)
		if strings.Contains(job.LastError, token) || !strings.Contains(job.LastError, "[redacted]") || strings.Contains(job.ResponseJSON, token) || strings.Contains(job.RequestJSON, "secret-script") {
			t.Fatalf("ledger leaked: %#v", job)
		}
	})
}

func TestHuggingFaceDownloadPublishesThroughWorkspaceRoot(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(workspace, "huggingface")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("0123456789abcdef0123456789abcdef"))
	}))
	defer server.Close()
	out := RunHuggingFace(context.Background(), config.HuggingFaceConfig{
		Enabled: true, ReadOnly: true, HubBaseURL: server.URL,
	}, "", workspace, t.TempDir(), HuggingFaceRequest{Operation: "download_file", RepoID: "org/model", Path: "file.bin"})
	if !strings.Contains(out, `"status":"error"`) {
		t.Fatalf("expected download through the workspace symlink to fail, got %s", out)
	}
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("outside directory gained %d entries", len(entries))
	}
}

func TestHuggingFaceAllowlistedDownloadSendsToken(t *testing.T) {
	const token = "hf_audit_token_0123456789"
	const payload = "0123456789abcdef0123456789abcdef"
	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(payload))
	}))
	defer server.Close()
	workspace := t.TempDir()
	out := RunHuggingFace(context.Background(), config.HuggingFaceConfig{
		Enabled: true, ReadOnly: true, HubBaseURL: server.URL, AllowedRepos: []string{"allowed/model"},
	}, token, workspace, t.TempDir(), HuggingFaceRequest{
		Operation: "download_file", RepoID: "allowed/model", Path: "file.bin", Destination: "models/file.bin",
	})
	if auth != "Bearer "+token || !strings.Contains(out, `"status":"success"`) {
		t.Fatalf("auth=%q output=%s", auth, out)
	}
	published := filepath.Join(workspace, "models", "file.bin")
	got, err := os.ReadFile(published)
	if err != nil || string(got) != payload {
		t.Fatalf("published file = %q, err=%v", got, err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(published)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("mode = %v, err=%v", info, err)
		}
	}
}

func TestHuggingFaceDownloadRefusesProtectedNames(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte("secret"))
	}))
	defer server.Close()
	workspace := t.TempDir()
	out := RunHuggingFace(context.Background(), config.HuggingFaceConfig{
		Enabled: true, ReadOnly: true, HubBaseURL: server.URL,
	}, "", workspace, t.TempDir(), HuggingFaceRequest{
		Operation: "download_file", RepoID: "org/model", Path: "file.bin", Destination: ".env",
	})
	if hits.Load() != 0 || !strings.Contains(out, "protected") {
		t.Fatalf("hits=%d output=%s", hits.Load(), out)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".env")); !os.IsNotExist(err) {
		t.Fatalf("protected file exists: %v", err)
	}
}

func TestHuggingFaceReadOnlyAllowsDownload(t *testing.T) {
	cfg := config.HuggingFaceConfig{Enabled: true, ReadOnly: true}
	if err := EvaluateHuggingFacePolicy(cfg, HuggingFaceRequest{Operation: "download_file", RepoID: "gpt2", Path: "file"}, ""); err != nil {
		t.Fatal(err)
	}
}

func TestHuggingFacePublicNamedReadOmitsToken(t *testing.T) {
	const token = "hf_audit_token_0123456789"
	var auth string
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		auth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"id":"gpt2"}`))
	}))
	defer server.Close()
	for _, id := range []string{"org/private", "gpt2"} {
		hits.Store(0)
		auth = ""
		out := RunHuggingFace(context.Background(), config.HuggingFaceConfig{
			Enabled: true, ReadOnly: true, HubBaseURL: server.URL,
		}, token, t.TempDir(), t.TempDir(), HuggingFaceRequest{Operation: "get_model", RepoID: id})
		if hits.Load() != 1 || auth != "" || !strings.Contains(out, `"status":"success"`) {
			t.Fatalf("%s hits=%d auth=%q output=%s", id, hits.Load(), auth, out)
		}
	}
}

func TestHuggingFaceAllowlistRejectsNamedReadBeforeHTTP(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{"id":"x"}`))
	}))
	defer server.Close()
	cfg := config.HuggingFaceConfig{
		Enabled: true, ReadOnly: true, HubBaseURL: server.URL, AllowedNamespaces: []string{"allowed"},
	}
	for _, id := range []string{"other/private", "gpt2"} {
		out := RunHuggingFace(context.Background(), cfg, "hf_audit_token_0123456789", t.TempDir(), t.TempDir(), HuggingFaceRequest{Operation: "get_model", RepoID: id})
		if hits.Load() != 0 || !strings.Contains(out, `"status":"error"`) || strings.Contains(out, "write") {
			t.Fatalf("id %s hits=%d output=%s", id, hits.Load(), out)
		}
		if id == "gpt2" && !strings.Contains(out, "namespace/repository") {
			t.Fatalf("single-segment read used the wrong rejection: %s", out)
		}
	}
}

func TestHuggingFaceAllowlistsRequireBothLists(t *testing.T) {
	const token = "hf_audit_token_0123456789"
	var hits atomic.Int32
	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		auth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"id":"allowed/model"}`))
	}))
	defer server.Close()
	rejected := []struct {
		repos, namespaces []string
		repoID, want      string
	}{
		{[]string{"other/model"}, []string{"allowed"}, "other/model", "allowed_namespaces"},
		{[]string{"allowed/other"}, []string{"allowed"}, "allowed/model", "allowed_repos"},
	}
	for _, tc := range rejected {
		cfg := config.HuggingFaceConfig{
			Enabled: true, ReadOnly: true, HubBaseURL: server.URL,
			AllowedRepos: tc.repos, AllowedNamespaces: tc.namespaces,
		}
		out := RunHuggingFace(context.Background(), cfg, token, t.TempDir(), t.TempDir(), HuggingFaceRequest{Operation: "get_model", RepoID: tc.repoID})
		if hits.Load() != 0 || !strings.Contains(out, `"status":"error"`) || !strings.Contains(out, tc.want) {
			t.Fatalf("repo %s lists %#v %#v hits=%d output=%s", tc.repoID, tc.repos, tc.namespaces, hits.Load(), out)
		}
	}
	hits.Store(0)
	auth = ""
	cfg := config.HuggingFaceConfig{
		Enabled: true, ReadOnly: true, HubBaseURL: server.URL,
		AllowedRepos: []string{"allowed/model"}, AllowedNamespaces: []string{"allowed"},
	}
	out := RunHuggingFace(context.Background(), cfg, token, t.TempDir(), t.TempDir(), HuggingFaceRequest{Operation: "get_model", RepoID: "allowed/model"})
	if hits.Load() != 1 || auth != "Bearer "+token || !strings.Contains(out, `"status":"success"`) {
		t.Fatalf("both lists hits=%d auth=%q output=%s", hits.Load(), auth, out)
	}
}

func TestHuggingFaceDatasetReadsFollowAllowlist(t *testing.T) {
	const token = "hf_audit_token_0123456789"
	var auth string
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		auth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"rows":[]}`))
	}))
	defer server.Close()
	cfg := config.HuggingFaceConfig{Enabled: true, ReadOnly: true, DatasetBaseURL: server.URL}
	out := RunHuggingFace(context.Background(), cfg, token, t.TempDir(), t.TempDir(), HuggingFaceRequest{
		Operation: "dataset_rows", Dataset: "org/data", Split: "train", Length: 1,
	})
	if hits.Load() != 1 || auth != "" || !strings.Contains(out, `"status":"success"`) {
		t.Fatalf("public dataset hits=%d auth=%q output=%s", hits.Load(), auth, out)
	}

	hits.Store(0)
	cfg.AllowedNamespaces = []string{"allowed"}
	out = RunHuggingFace(context.Background(), cfg, token, t.TempDir(), t.TempDir(), HuggingFaceRequest{
		Operation: "dataset_rows", Dataset: "other/private", Split: "train", Length: 1,
	})
	if hits.Load() != 0 || !strings.Contains(out, `"status":"error"`) {
		t.Fatalf("rejected dataset hits=%d output=%s", hits.Load(), out)
	}

	auth = ""
	out = RunHuggingFace(context.Background(), cfg, token, t.TempDir(), t.TempDir(), HuggingFaceRequest{
		Operation: "dataset_rows", Dataset: "allowed/data", Split: "train", Length: 1,
	})
	if hits.Load() != 1 || auth != "Bearer "+token || !strings.Contains(out, `"status":"success"`) {
		t.Fatalf("allowlisted dataset hits=%d auth=%q output=%s", hits.Load(), auth, out)
	}

	hits.Store(0)
	out = RunHuggingFace(context.Background(), cfg, token, t.TempDir(), t.TempDir(), HuggingFaceRequest{
		Operation: "dataset_rows", Dataset: "org/../../other", Split: "train", Length: 1,
	})
	if hits.Load() != 0 || !strings.Contains(out, `"status":"error"`) {
		t.Fatalf("invalid dataset hits=%d output=%s", hits.Load(), out)
	}
}

func TestHuggingFaceSearchOmitsTokenWhenAllowlistSet(t *testing.T) {
	const token = "hf_audit_token_0123456789"
	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()
	cfg := config.HuggingFaceConfig{
		Enabled: true, ReadOnly: true, HubBaseURL: server.URL, AllowedNamespaces: []string{"allowed"},
	}
	for _, operation := range []string{"search_models", "search_papers", "daily_papers"} {
		auth = ""
		out := RunHuggingFace(context.Background(), cfg, token, t.TempDir(), t.TempDir(), HuggingFaceRequest{Operation: operation, Query: "bert"})
		if auth != "" || !strings.Contains(out, `"status":"success"`) {
			t.Fatalf("%s auth=%q output=%s", operation, auth, out)
		}
	}
}

func runHuggingFaceJobFixture(t *testing.T, jobsURL, dataDir, token string) string {
	t.Helper()
	return RunHuggingFace(context.Background(), config.HuggingFaceConfig{
		Enabled: true, ReadOnly: false, AllowJobs: true, AllowedHardware: []string{"cpu-basic"},
		JobsBaseURL: jobsURL + "/api/jobs", JobNamespace: "team",
	}, token, t.TempDir(), dataDir, HuggingFaceRequest{
		Operation: "job_run_python", Script: "print('secret-script')", Hardware: "cpu-basic",
	})
}

func mustHuggingFaceLedgerJob(t *testing.T, dataDir string) hfJobRecord {
	t.Helper()
	jobs, err := HuggingFaceLedgerJobs(context.Background(), dataDir, 10)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("jobs = %#v, err=%v", jobs, err)
	}
	return hfJobRecord{
		RequestJSON: jobs[0].RequestJSON, ResponseJSON: jobs[0].ResponseJSON, LastError: jobs[0].LastError,
	}
}

type hfJobRecord struct {
	RequestJSON  string
	ResponseJSON string
	LastError    string
}
