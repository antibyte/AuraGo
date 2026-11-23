package kaggle

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
)

func fixtureClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	return &Client{http: server.Client(), base: server.URL + "/", credentials: Credentials{Mode: "token", Token: "fixture-token"}}
}
func TestKaggleContractsAndTypedRequests(t *testing.T) {
	for _, op := range Catalog() {
		if op.Schema == nil {
			t.Fatal(op.Name)
		}
		if _, _, err := Decode(op, []byte(`{}`)); err != nil {
			t.Fatalf("%s: %v", op.Name, err)
		}
	}
	op, _ := Lookup("datasets", "list_datasets")
	for _, raw := range []string{`{"page":"2"}`, `{"page":1.5}`, `{"endpoint":"https://example.com"}`, `{"sortBy":"unknown"}`, `null`, `{} {}`} {
		if _, _, err := Decode(op, []byte(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	_, typed, err := Decode(op, []byte(`{"search":"cats","page":2,"pageSize":20,"pageToken":"next"}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := typed.(*ApiListDatasetsRequest); !ok {
		t.Fatalf("untyped request %T", typed)
	}
	if _, err = Lookup("forums", "create_topic"); err == nil {
		t.Fatal("forum writes exposed")
	}
	if _, err = Lookup("benchmarks", "delete_benchmark_task"); err == nil {
		t.Fatal("unsupported delete exposed")
	}
}
func TestKaggleAuthenticationErrorsAndNoMutationReplay(t *testing.T) {
	for _, code := range []int{401, 403, 429, 503} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			var n atomic.Int32
			c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
				n.Add(1)
				if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer fixture-token" {
					t.Error("incorrect wire authentication")
				}
				w.Header().Set("Retry-After", "7")
				w.WriteHeader(code)
				io.WriteString(w, `{"error":"fixture-token"}`)
			})
			op, _ := Lookup("datasets", "create_dataset")
			_, err := c.Call(context.Background(), op, &ApiCreateDatasetRequest{})
			var api *APIError
			if !errors.As(err, &api) || api.Status != code || api.Uncertain != (code >= 500) {
				t.Fatalf("bad error: %#v", err)
			}
			if n.Load() != 1 || strings.Contains(err.Error(), "fixture-token") {
				t.Fatal("retry or credential leak")
			}
		})
	}
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != "user" || p != "legacy" {
			t.Error("legacy auth")
		}
		io.WriteString(w, `{}`)
	})
	c.credentials = Credentials{Mode: "legacy", Username: "user", Key: "legacy"}
	op, _ := Lookup("notebooks", "get_accelerator_quota_statistics")
	if _, err := c.Call(context.Background(), op, struct{}{}); err != nil {
		t.Fatal(err)
	}
}
func TestKaggleTimeoutUnknownAndRedirectCredentialBoundary(t *testing.T) {
	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	release := make(chan struct{})
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		cancel() // Cancel after the server accepts the mutation, independent of scheduling speed.
		<-release
		io.WriteString(w, `{}`)
	})
	op, _ := Lookup("models", "create_model")
	_, err := c.Call(ctx, op, struct{}{})
	close(release)
	var api *APIError
	if !errors.As(err, &api) || !api.Uncertain || calls.Load() != 1 {
		t.Fatalf("%v calls=%d", err, calls.Load())
	}
	if _, err = storageClient(context.Background(), "https://127.0.0.1/private", time.Second); err == nil {
		t.Fatal("private storage destination allowed")
	}
	if _, err = storageClient(context.Background(), "http://example.com/file", time.Second); err == nil {
		t.Fatal("plaintext storage allowed")
	}
}
func TestKagglePermissionMatrix(t *testing.T) {
	c := config.DefaultKaggleConfig()
	c.Enabled = true
	a := Access{Network: true, Write: true, Admin: true}
	for _, op := range Catalog() {
		p := map[string]any{"kernelExecutionType": "QUICK_SAVE"}
		err := CheckPolicy(c, a, op, p, "owner")
		allowed := op.Grant == "read" || op.Grant == "cancel"
		if (err == nil) != allowed {
			t.Fatalf("read-only %s: %v", op.Name, err)
		}
	}
	c.ReadOnly = false
	c.AllowUpdate = true
	c.AllowCreate = true
	op, _ := Lookup("notebooks", "save_kernel")
	for _, p := range []map[string]any{{"kernelExecutionType": "SAVE_AND_RUN_ALL"}, {"kernelExecutionType": "QUICK_SAVE", "enableGpu": true}, {"kernelExecutionType": "QUICK_SAVE", "enableInternet": true}, {"kernelExecutionType": "QUICK_SAVE", "isPrivate": false}, {"kernelExecutionType": "QUICK_SAVE", "slug": "stranger/book"}} {
		if CheckPolicy(c, a, op, p, "owner") == nil {
			t.Fatalf("denied grant bypass: %v", p)
		}
	}
	op, _ = Lookup("datasets", "download_dataset")
	c.AllowDownload = true
	if err := CheckPolicy(c, a, op, nil, "owner"); err != nil {
		t.Fatal(err)
	}
	a.Write = false
	if CheckPolicy(c, a, op, nil, "owner") == nil {
		t.Fatal("download bypassed global write")
	}
}
func newTestService(t *testing.T) *Service {
	t.Helper()
	c := config.DefaultKaggleConfig()
	c.Enabled = true
	c.Token = "fixture-token"
	s, err := NewService(context.Background(), t.TempDir(), func() Snapshot { return Snapshot{c, Access{true, true, true, true, true}} })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func TestKaggleDurableAdmissionAndConflict(t *testing.T) {
	s := newTestService(t)
	j := Job{ID: newID(), Account: "owner", Domain: "datasets", Operation: "create_dataset", Hash: "one", Request: []byte(`{}`)}
	a, fresh, err := s.Store.admit(context.Background(), j)
	if err != nil || !fresh {
		t.Fatal(err)
	}
	b, fresh, err := s.Store.admit(context.Background(), j)
	if err != nil || fresh || a.ID != b.ID {
		t.Fatal("replayed admission")
	}
	j.Hash = "different"
	if _, _, err = s.Store.admit(context.Background(), j); err == nil {
		t.Fatal("conflicting request accepted")
	}
	j.Hash = "one"
	j.Account = "other"
	if _, _, err = s.Store.admit(context.Background(), j); err == nil {
		t.Fatal("cross-account admission")
	}
}
func TestKaggleTransferAtomicCancellationTraversalAndLimits(t *testing.T) {
	s := newTestService(t)
	p, err := s.CreateProject(context.Background(), "Test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.WriteFile(context.Background(), p.ID, "data.csv", strings.NewReader("original"), nil); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = s.WriteFile(ctx, p.ID, "data.csv", strings.NewReader("replaced"), nil); err == nil {
		t.Fatal("canceled publication")
	}
	f, err := s.OpenFile(p.ID, "data.csv")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(f)
	f.Close()
	if string(b) != "original" {
		t.Fatal("original lost")
	}
	for _, name := range []string{"../secret", "/secret", "C:\\secret", "a/b", "a\\b", ".hidden"} {
		if _, err = s.WriteFile(context.Background(), p.ID, name, strings.NewReader("x"), nil); err == nil {
			t.Fatalf("unsafe %q", name)
		}
	}
	deny := func() error { return errors.New("revoked") }
	if _, err = s.WriteFile(context.Background(), p.ID, "deny", strings.NewReader("x"), deny); err == nil {
		t.Fatal("revocation ignored")
	}
	if err = os.Symlink(filepath.Join(s.Root, p.ID, "data.csv"), filepath.Join(s.Root, p.ID, "link")); err == nil {
		if _, err = s.OpenFile(p.ID, "link"); err == nil {
			t.Fatal("symlink opened")
		}
	}
}
func TestKaggleEventsAndUploadTokensStayPrivate(t *testing.T) {
	s := newTestService(t)
	id, err := s.Store.Emit(context.Background(), "cell", map[string]any{"text": "first"})
	if err != nil {
		t.Fatal(err)
	}
	s.Store.Emit(context.Background(), "cell", map[string]any{"text": "second"})
	events, err := s.Store.Events(context.Background(), id)
	if err != nil || len(events) != 1 || !bytes.Contains(events[0].Data, []byte("second")) {
		t.Fatal("SSE replay cursor failed")
	}
	raw := cleanOutput([]byte(`{"token":"secret","createUrl":"signed","nested":{"downloadUrl":"hidden"},"pageToken":"next"}`))
	if strings.Contains(string(raw), "secret") || strings.Contains(string(raw), "signed") {
		t.Fatal("opaque credentials escaped")
	}
}
func TestKaggleExistingPublicResourceRequiresPublish(t *testing.T) {
	s := newTestService(t)
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"isPrivate":false}`) })
	cfg := s.current().Config
	cfg.AllowUpdate = true
	op, _ := Lookup("datasets", "update_dataset_metadata")
	err := s.checkResource(context.Background(), c, cfg, op, map[string]any{"ownerSlug": "owner", "datasetSlug": "public"}, "owner")
	if err == nil {
		t.Fatal("public mutation allowed without publication")
	}
	cfg.AllowPublish = true
	if err = s.checkResource(context.Background(), c, cfg, op, map[string]any{"ownerSlug": "owner", "datasetSlug": "public"}, "owner"); err != nil {
		t.Fatal(err)
	}
}

func TestKaggleUnknownOutcomePersistsWithoutReplay(t *testing.T) {
	dir := t.TempDir()
	s, err := NewService(context.Background(), dir, func() Snapshot { return Snapshot{Config: config.DefaultKaggleConfig()} })
	if err != nil {
		t.Fatal(err)
	}
	id := newID()
	s.Store.admit(context.Background(), Job{ID: id, Account: "owner", Request: json.RawMessage(`{}`), Hash: "h"})
	s.Store.update(context.Background(), id, "running", 0, nil, "")
	s.Close()
	s, err = NewService(context.Background(), dir, func() Snapshot { return Snapshot{Config: config.DefaultKaggleConfig()} })
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	j, err := s.Store.Get(context.Background(), id)
	if err != nil || j.Status != "unknown" {
		t.Fatalf("%+v %v", j, err)
	}
}

func TestKaggleMachineShapeGrants(t *testing.T) {
	base := config.DefaultKaggleConfig()
	base.Enabled, base.ReadOnly = true, false
	base.AllowCreate, base.AllowRemoteExecute, base.AllowGPU = true, true, true
	base.AllowedMachineShapes = []string{"NvidiaTeslaT4"}
	a := Access{Network: true, Write: true}
	op, _ := Lookup("notebooks", "save_kernel")
	params := func(extra map[string]any) map[string]any {
		p := map[string]any{"kernelExecutionType": "SAVE_AND_RUN_ALL", "enableGpu": true}
		for k, v := range extra {
			if v == nil {
				delete(p, k)
			} else {
				p[k] = v
			}
		}
		return p
	}
	for _, tc := range []struct {
		name   string
		mutate func(*config.KaggleConfig)
		params map[string]any
		ok     bool
	}{
		{"listed shape with GPU", nil, params(map[string]any{"machineShape": "NvidiaTeslaT4"}), true},
		{"no shape", nil, params(nil), true},
		{"empty shape", nil, params(map[string]any{"machineShape": ""}), true},
		{"quick save with shape and GPU", nil, params(map[string]any{"kernelExecutionType": "QUICK_SAVE", "machineShape": "NvidiaTeslaT4"}), true},
		{"second listed shape", func(c *config.KaggleConfig) { c.AllowedMachineShapes = []string{"NvidiaTeslaT4", "NvidiaL4"} }, params(map[string]any{"machineShape": "NvidiaL4"}), true},
		{"shape without enableGpu", nil, params(map[string]any{"machineShape": "NvidiaTeslaT4", "enableGpu": nil}), false},
		{"shape with enableGpu false", nil, params(map[string]any{"machineShape": "NvidiaTeslaT4", "enableGpu": false}), false},
		{"shape without GPU grant", func(c *config.KaggleConfig) { c.AllowGPU = false }, params(map[string]any{"machineShape": "NvidiaTeslaT4"}), false},
		{"known shape not listed", nil, params(map[string]any{"machineShape": "NvidiaL4"}), false},
		{"unknown shape", nil, params(map[string]any{"machineShape": "NvidiaA100"}), false},
		{"wrong case", nil, params(map[string]any{"machineShape": "nvidiateslat4"}), false},
		{"empty allow-list", func(c *config.KaggleConfig) { c.AllowedMachineShapes = nil }, params(map[string]any{"machineShape": "NvidiaTeslaT4"}), false},
		{"docker image with listed shape", nil, params(map[string]any{"machineShape": "NvidiaTeslaT4", "dockerImage": "gcr.io/kaggle-gpu-images/python"}), false},
		{"docker image alone", nil, params(map[string]any{"dockerImage": "gcr.io/kaggle-gpu-images/python"}), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := base
			c.AllowedMachineShapes = append([]string(nil), base.AllowedMachineShapes...)
			if tc.mutate != nil {
				tc.mutate(&c)
			}
			err := CheckGrants(c, a, op, tc.params)
			if (err == nil) != tc.ok {
				t.Fatalf("CheckGrants(%v) = %v, want ok=%v", tc.params, err, tc.ok)
			}
		})
	}
}

func TestKaggleMachineShapeSnapshotIntersection(t *testing.T) {
	current := config.KaggleConfig{AllowedMachineShapes: []string{"NvidiaTeslaT4", "NvidiaL4"}}
	for _, tc := range []struct{ snapshot, want []string }{
		{[]string{"NvidiaL4"}, []string{"NvidiaL4"}},
		{[]string{"NvidiaL4", "NvidiaTeslaT4"}, []string{"NvidiaTeslaT4", "NvidiaL4"}},
		{nil, []string{}},
	} {
		got := intersectConfig(current, config.KaggleConfig{AllowedMachineShapes: tc.snapshot}).AllowedMachineShapes
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("intersect with %v = %v, want %v", tc.snapshot, got, tc.want)
		}
	}
	// Revoking a shape in the current config also revokes it for a running snapshot.
	revoked := intersectConfig(config.KaggleConfig{AllowedMachineShapes: []string{"NvidiaL4"}}, current).AllowedMachineShapes
	if !reflect.DeepEqual(revoked, []string{"NvidiaL4"}) {
		t.Fatalf("revocation ignored: %v", revoked)
	}
	snap := config.KaggleConfig{AllowedMachineShapes: []string{"NvidiaTeslaT4"}}
	ctx := WithConfigSnapshot(context.Background(), snap)
	snap.AllowedMachineShapes[0] = "NvidiaL4"
	held := ctx.Value(grantSnapshotKey{}).(config.KaggleConfig)
	if held.AllowedMachineShapes[0] != "NvidiaTeslaT4" {
		t.Fatal("WithConfigSnapshot shares the machine-shape slice with its caller")
	}
}

func TestKaggleRemoveFilePathSafety(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	p, err := s.CreateProject(ctx, "Staging")
	if err != nil {
		t.Fatal(err)
	}
	const name = "r0123456789ab-train.jsonl"
	if _, err = s.WriteFile(ctx, p.ID, name, strings.NewReader("{}\n"), nil); err != nil {
		t.Fatal(err)
	}
	if err = s.RemoveFile(p.ID, name); err != nil {
		t.Fatal(err)
	}
	if files, _ := s.Files(p.ID); len(files) != 0 {
		t.Fatalf("file not removed: %v", files)
	}
	if err = s.RemoveFile(p.ID, name); err != nil {
		t.Fatalf("missing file must not be an error: %v", err)
	}
	for _, bad := range []string{"", ".", "..", "../secret", "a/b", "a\b", ".hidden", "CON", "C:x"} {
		if s.RemoveFile(p.ID, bad) == nil {
			t.Fatalf("unsafe name %q accepted", bad)
		}
	}
	if s.RemoveFile("not-a-project", "x.txt") == nil {
		t.Fatal("invalid project ID accepted")
	}
	dir := filepath.Join(s.Root, p.ID, "folder")
	if err = os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if s.RemoveFile(p.ID, "folder") == nil {
		t.Fatal("directory removed")
	}
	if _, err = os.Stat(dir); err != nil {
		t.Fatal("directory lost")
	}
	target := filepath.Join(s.Root, p.ID, "keep.txt")
	if err = os.WriteFile(target, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Symlink creation needs a privilege on Windows; the existing transfer test uses the same guard.
	if err = os.Symlink(target, filepath.Join(s.Root, p.ID, "link.txt")); err == nil {
		if s.RemoveFile(p.ID, "link.txt") == nil {
			t.Fatal("symlink removed")
		}
		if _, err = os.Stat(target); err != nil {
			t.Fatal("symlink target lost")
		}
	}
}
