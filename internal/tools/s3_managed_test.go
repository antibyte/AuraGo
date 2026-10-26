package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

func managedS3Fixture(t *testing.T, handler http.HandlerFunc) (*config.Config, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	c := &config.Config{}
	c.S3.Enabled = true
	c.S3.Provider = config.S3ProviderRustFS
	c.RustFS.Enabled, c.RustFS.AgentAccess = true, true
	c.RustFS.Bucket = "aurago"
	c.RustFS.Permissions = config.RustFSPermissions{AllowRead: true, AllowWrite: true, AllowDelete: true, AllowedLocations: []config.RustFSLocation{{Bucket: "aurago", Prefixes: []string{"agent/"}}}}
	base := t.TempDir()
	c.Directories.WorkspaceDir = filepath.Join(base, "workspace", "workdir")
	c.Directories.DataDir = filepath.Join(base, "data")
	c.Tools.DocumentCreator.OutputDir = filepath.Join(base, "documents")
	c.Agent.AllowFilesystemWrite = true
	for _, dir := range []string{c.Directories.WorkspaceDir, c.Directories.DataDir, c.Tools.DocumentCreator.OutputDir} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	c.AcquireS3 = func(ctx context.Context, snapshot *config.Config, _ bool) (*config.RustFSS3Lease, error) {
		owner, cancel := context.WithCancel(ctx)
		return &config.RustFSS3Lease{Context: owner, Managed: true, Endpoint: server.URL, Region: "us-east-1", Bucket: "aurago", AccessKey: "s3-managed-access-fixture", SecretKey: "s3-managed-secret-fixture", Permissions: snapshot.Clone().RustFS.Permissions, Release: cancel, Check: owner.Err, Publish: func(commit func() error) error {
			if err := owner.Err(); err != nil {
				return err
			}
			return commit()
		}}, nil
	}
	c.S3Availability = func(*config.Config) config.FeatureAvailability { return config.FeatureAvailability{Available: true} }
	return c, server
}

func decodeS3Result(t *testing.T, raw string) s3Result {
	t.Helper()
	var result s3Result
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestS3ManagedPermissionMatrixAndBothCopyEnds(t *testing.T) {
	operations := []struct {
		op               string
		read, write, del bool
	}{{"list_buckets", true, false, false}, {"list_objects", true, false, false}, {"download", true, false, false}, {"upload", false, true, false}, {"delete", false, false, true}, {"copy", true, true, false}, {"move", true, true, true}}
	for bits := 0; bits < 8; bits++ {
		for _, op := range operations {
			t.Run(fmt.Sprintf("%s-%d", op.op, bits), func(t *testing.T) {
				permissions := config.RustFSPermissions{AllowRead: bits&1 != 0, AllowWrite: bits&2 != 0, AllowDelete: bits&4 != 0, AllowedLocations: []config.RustFSLocation{{Bucket: "aurago", Prefixes: []string{"agent/"}}}}
				c := &config.Config{RustFS: config.RustFSConfig{Permissions: permissions}}
				cfg := S3Config{Config: c, lease: &config.RustFSS3Lease{Managed: true, Permissions: permissions}}
				_, err := authorizeS3Operation(cfg, op.op, "aurago", "agent/source", "agent/", "aurago", "agent/dest")
				allowed := (!op.read || permissions.AllowRead) && (!op.write || permissions.AllowWrite) && (!op.del || permissions.AllowDelete)
				if (err == nil) != allowed {
					t.Fatalf("permission result %v; want allowed=%v", err, allowed)
				}
				if op.op == "copy" || op.op == "move" {
					if _, err := authorizeS3Operation(cfg, op.op, "aurago", "agent/source", "", "aurago", "agent2/dest"); err == nil {
						t.Fatal("destination prefix escaped")
					}
					if _, err := authorizeS3Operation(cfg, op.op, "other", "agent/source", "", "aurago", "agent/dest"); err == nil {
						t.Fatal("source bucket escaped")
					}
				}
			})
		}
	}
}

func TestS3ManagedRefusalsNeverUseExternalCredentials(t *testing.T) {
	var requests atomic.Int32
	c, _ := managedS3Fixture(t, func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.WriteHeader(500) })
	c.S3.AccessKey, c.S3.SecretKey = "external-unused-access", "external-unused-secret"
	for _, tc := range []struct {
		name   string
		change func(*config.Config)
	}{
		{"missing runtime", func(c *config.Config) { c.AcquireS3 = nil }},
		{"agent disabled", func(c *config.Config) { c.RustFS.AgentAccess = false }},
		{"no locations", func(c *config.Config) { c.RustFS.Permissions.AllowedLocations = nil }},
		{"readonly", func(c *config.Config) { c.S3.ReadOnly = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := c.Clone()
			tc.change(candidate)
			result := decodeS3Result(t, ExecuteS3Context(context.Background(), S3ConfigFromConfig(candidate), "upload", "aurago", "agent/file", "absent", "", "", ""))
			if result.Status == "success" {
				t.Fatal("managed denial fell through")
			}
		})
	}
	if requests.Load() != 0 {
		t.Fatal("refused request reached a data endpoint")
	}
}

func TestS3ManagedListingsAreScopedAndBoundedAcrossPages(t *testing.T) {
	for _, byteLimit := range []int{4 << 20, 1200} {
		t.Run(fmt.Sprint(byteLimit), func(t *testing.T) {
			var calls atomic.Int32
			c, _ := managedS3Fixture(t, func(w http.ResponseWriter, r *http.Request) {
				page := calls.Add(1)
				if r.Method != "GET" || r.URL.Query().Get("prefix") != "agent/" {
					t.Error("listing was not prefix scoped")
					w.WriteHeader(403)
					return
				}
				w.Header().Set("Content-Type", "application/xml")
				fmt.Fprint(w, `<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Name>aurago</Name><Prefix>agent/</Prefix><IsTruncated>true</IsTruncated><NextContinuationToken>next-`+fmt.Sprint(page)+`</NextContinuationToken>`)
				for i := 0; i < 600; i++ {
					fmt.Fprintf(w, "<Contents><Key>agent/%d-%04d</Key><Size>1</Size></Contents>", page, i)
				}
				fmt.Fprint(w, "</ListBucketResult>")
			})
			c.Agent.ToolOutputLimit = byteLimit
			raw := ExecuteS3Context(context.Background(), S3ConfigFromConfig(c), "list_objects", "", "", "", "", "", "")
			result := decodeS3Result(t, raw)
			objects, ok := result.Data.([]any)
			if result.Status != "success" || !ok || !result.Truncated {
				t.Fatalf("bounded listing failed: %s", raw)
			}
			if len(raw) > byteLimit || len(objects) > 1000 || calls.Load() > 2 {
				t.Fatalf("listing escaped bounds: bytes=%d objects=%d pages=%d", len(raw), len(objects), calls.Load())
			}
			if byteLimit > 1200 && len(objects) != 1000 {
				t.Fatalf("count cap returned %d objects", len(objects))
			}
		})
	}
}

func TestS3ManagedListingsRejectAmbiguousOrForbiddenPrefixes(t *testing.T) {
	var requests atomic.Int32
	c, _ := managedS3Fixture(t, func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.WriteHeader(500) })
	c.RustFS.Permissions.AllowedLocations[0].Prefixes = []string{"agent/", "second/"}
	for _, prefix := range []string{"", "agent2/", "Agent/", "agent%2F"} {
		result := decodeS3Result(t, ExecuteS3Context(context.Background(), S3ConfigFromConfig(c), "list_objects", "aurago", "", "", prefix, "", ""))
		if result.Code != "s3_permission_denied" {
			t.Fatalf("prefix %q not denied", prefix)
		}
	}
	result := decodeS3Result(t, ExecuteS3Context(context.Background(), S3ConfigFromConfig(c), "list_buckets", "", "", "", "", "", ""))
	if result.Status != "success" || len(result.Data.([]any)) != 1 || requests.Load() != 0 {
		t.Fatal("bucket listing queried account-wide storage")
	}
}

func TestS3ManagedUploadUsesProtectedRootedInputsAndKeepsLegacyData(t *testing.T) {
	var uploaded atomic.Int32
	c, server := managedS3Fixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Error("unexpected upload method")
			w.WriteHeader(500)
			return
		}
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			t.Error(err)
		}
		uploaded.Add(1)
		w.Header().Set("ETag", `"d41d8cd98f00b204e9800998ecf8427e"`)
	})
	workspace := filepath.Join(c.Directories.WorkspaceDir, "upload.txt")
	data := filepath.Join(c.Directories.DataDir, "backup.txt")
	sensitive := filepath.Join(c.Directories.WorkspaceDir, "private.env")
	for _, path := range []string{workspace, data, sensitive} {
		if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		path    string
		allowed bool
	}{{workspace, true}, {"workdir/upload.txt", true}, {data, false}, {sensitive, false}} {
		got := decodeS3Result(t, ExecuteS3Context(context.Background(), S3ConfigFromConfig(c), "upload", "aurago", "agent/upload", tc.path, "", "", ""))
		if (got.Status == "success") != tc.allowed {
			t.Fatalf("managed source %s: %s", filepath.Base(tc.path), got.Message)
		}
	}
	external := S3Config{Endpoint: server.URL, Region: "us-east-1", Bucket: "aurago", AccessKey: "external-fixture-access", SecretKey: "external-fixture-secret", UsePathStyle: true, Insecure: true, WorkspaceDir: c.Directories.WorkspaceDir, DataDir: c.Directories.DataDir}
	if got := decodeS3Result(t, ExecuteS3(external, "upload", "aurago", "backup", data, "", "", "")); got.Status != "success" {
		t.Fatalf("legacy data upload failed: %s", got.Message)
	}
	if uploaded.Load() != 3 {
		t.Fatal("unexpected upload count")
	}
}

func TestS3DownloadPublicationRejectsOverflowTruncationAndRevocation(t *testing.T) {
	for _, kind := range []string{"overflow", "truncated", "read-error", "revoked", "success"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			dest := filepath.Join(dir, "out.txt")
			if err := os.WriteFile(dest, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			cfg := S3Config{WorkspaceDir: dir, MaxDownloadBytes: 4}
			var source io.Reader = strings.NewReader("data")
			expected := int64(4)
			switch kind {
			case "overflow":
				source = strings.NewReader("large")
			case "truncated":
				source = strings.NewReader("bad")
			case "read-error":
				source = failingReader{}
			case "revoked":
				cfg.lease = &config.RustFSS3Lease{Publish: func(func() error) error { return errors.New("revoked") }}
			}
			_, err := writeS3DownloadRooted(cfg, dest, source, expected)
			if (err == nil) != (kind == "success") {
				t.Fatalf("publication result: %v", err)
			}
			data, readErr := os.ReadFile(dest)
			if readErr != nil {
				t.Fatal(readErr)
			}
			want := "original"
			if kind == "success" {
				want = "data"
			}
			if string(data) != want {
				t.Fatal("failed transfer replaced destination")
			}
			files, _ := filepath.Glob(filepath.Join(dir, ".aurago_edit_*"))
			if len(files) != 0 {
				t.Fatal("transfer staging leaked")
			}
		})
	}
}

func TestS3MoveRechecksAuthorizationBeforeDelete(t *testing.T) {
	var deletes atomic.Int32
	var revoke context.CancelFunc
	c, _ := managedS3Fixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			deletes.Add(1)
			return
		}
		if r.Method == "PUT" {
			fmt.Fprint(w, `<CopyObjectResult><ETag>"d41d8cd98f00b204e9800998ecf8427e"</ETag><LastModified>2026-10-26T00:00:00Z</LastModified></CopyObjectResult>`)
			revoke()
			return
		}
		w.WriteHeader(500)
	})
	original := c.AcquireS3
	c.AcquireS3 = func(ctx context.Context, c *config.Config, agent bool) (*config.RustFSS3Lease, error) {
		owner, cancel := context.WithCancel(ctx)
		revoke = cancel
		return original(owner, c, agent)
	}
	result := decodeS3Result(t, ExecuteS3Context(context.Background(), S3ConfigFromConfig(c), "move", "aurago", "agent/source", "", "", "aurago", "agent/dest"))
	if result.Status == "success" || deletes.Load() != 0 {
		t.Fatal("move deleted after revocation")
	}
}

func TestS3ManagedProbeUsesPermittedListingWithoutWrites(t *testing.T) {
	c, _ := managedS3Fixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Query().Get("prefix") != "agent/" || r.URL.Query().Get("max-keys") != "1" {
			t.Error("probe was not a bounded permitted listing")
			w.WriteHeader(403)
			return
		}
		fmt.Fprint(w, `<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Name>aurago</Name><IsTruncated>false</IsTruncated></ListBucketResult>`)
	})
	if err := CheckS3Connection(context.Background(), S3ConfigFromConfig(c)); err != nil {
		t.Fatal(err)
	}
	c.RustFS.Permissions.AllowRead = false
	if err := CheckS3Connection(context.Background(), S3ConfigFromConfig(c)); !errors.Is(err, ErrS3ReadVerificationUnavailable) {
		t.Fatalf("write-only probe: %v", err)
	}
}

func TestS3ExternalSnapshotCannotPublishAfterProviderChange(t *testing.T) {
	c := &config.Config{}
	c.S3.Enabled = true
	c.S3.Provider = "external"
	current := c.Clone()
	c.AuthorizationSnapshots = func() (*config.Config, *config.Config) { return c, current }
	cfg, release, err := acquireS3Config(context.Background(), S3ConfigFromConfig(c), true)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	current.S3.Provider = "rustfs"
	called := false
	if err := cfg.publish(func() error { called = true; return nil }); err == nil || called {
		t.Fatal("external request published after source switch")
	}
}

func TestS3ManagedSavedProbeWithoutReadSkipsRuntimeAdmission(t *testing.T) {
	for _, change := range []func(*config.Config){func(c *config.Config) { c.RustFS.AgentAccess = false }, func(c *config.Config) { c.RustFS.Permissions.AllowRead = false }, func(c *config.Config) { c.RustFS.Permissions.AllowedLocations[0].Prefixes = nil }} {
		c, _ := managedS3Fixture(t, func(http.ResponseWriter, *http.Request) { t.Fatal("probe sent HTTP without readable scope") })
		c.AcquireS3 = func(context.Context, *config.Config, bool) (*config.RustFSS3Lease, error) {
			t.Fatal("probe acquired credentials without readable scope")
			return nil, errors.New("not ready")
		}
		c.S3Availability = func(*config.Config) config.FeatureAvailability {
			return config.FeatureAvailability{Reason: "not ready"}
		}
		change(c)
		if err := CheckS3Connection(context.Background(), S3ConfigFromConfig(c)); !errors.Is(err, ErrS3ReadVerificationUnavailable) {
			t.Fatalf("saved probe error=%v", err)
		}
	}
}

func TestS3ManagedClientDoesNotReplayRetryableMutation(t *testing.T) {
	var requests atomic.Int32
	c, _ := managedS3Fixture(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, `<Error><Code>SlowDown</Code><Message>retry later</Message></Error>`)
	})
	result := decodeS3Result(t, ExecuteS3Context(context.Background(), S3ConfigFromConfig(c), "delete", "aurago", "agent/source", "", "", "", ""))
	if result.Status == "success" || requests.Load() != 1 {
		t.Fatalf("managed mutation replayed %d times", requests.Load())
	}
}

func TestS3ResultsScrubRegisteredCredentialsInObjectMetadata(t *testing.T) {
	secret := "s3-output-metadata-fixture-secret"
	security.RegisterSensitive(secret)
	raw := s3Encode(s3Result{Status: "success", Data: map[string]string{"key": secret}})
	if strings.Contains(raw, secret) || !json.Valid([]byte(raw)) {
		t.Fatal("S3 result leaked a registered credential or broke JSON")
	}
}

func TestS3DownloadHonorsRunScopedFilesystemGrant(t *testing.T) {
	cfg := S3Config{WorkspaceDir: t.TempDir(), AllowFilesystemWrite: true, ctx: WithRuntimePermissions(context.Background(), RuntimePermissions{AllowFilesystemWrite: false})}
	if _, err := resolveS3DownloadDestination(cfg, "agent/file", ""); err == nil {
		t.Fatal("S3 download widened the run filesystem grant")
	}
}
