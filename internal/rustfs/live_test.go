package rustfs

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"aurago/internal/dockerutil"
	"aurago/internal/security"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/minio/minio-go/v7"
)

func waitOperation(t *testing.T, m *Manager, action Action) {
	t.Helper()
	operation, err := m.Begin(action)
	if err != nil {
		t.Fatalf("begin %s: %s", action, ErrorCode(err))
	}
	deadline := time.NewTimer(4 * time.Minute)
	defer deadline.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		operation, _ = m.Operation(operation.ID)
		switch operation.State {
		case "succeeded":
			return
		case "failed", "cancelled", "unknown":
			m.mu.Lock()
			detail := m.jobs[operation.ID].err
			m.mu.Unlock()
			t.Fatalf("%s failed in %s: %s", action, operation.Phase, security.Scrub(detail.Error()))
		}
		select {
		case <-deadline.C:
			t.Fatalf("%s timed out", action)
		case <-ticker.C:
		}
	}
}
func assertDeniedS3(t *testing.T, err error) {
	t.Helper()
	if minio.ToErrorResponse(err).Code != "AccessDenied" {
		t.Fatalf("wanted AccessDenied, got %s", minio.ToErrorResponse(err).Code)
	}
}

// TestLiveRustFS is explicitly opt-in and touches only its unique labeled fixture resources.
func TestLiveRustFS(t *testing.T) {
	if os.Getenv("AURAGO_RUSTFS_LIVE") != "1" {
		t.Skip("set AURAGO_RUSTFS_LIVE=1 for isolated Docker acceptance")
	}
	cfg := testConfig()
	cfg.Docker.Host = os.Getenv("DOCKER_HOST")
	cfg.Runtime.IsDocker = os.Getenv("AURAGO_RUSTFS_CONTAINER") == "1"
	suffix := strings.ToLower(rand.Text()[:12])
	names := resourceNames{"aurago-rustfs-it-" + suffix, "aurago-rustfs-seed-it-" + suffix, "aurago_rustfs_data_it_" + suffix, "aurago_rustfs_credentials_it_" + suffix}
	vault := &memoryVault{}
	engine := dockerutil.NewClient(cfg.Docker.Host, 3*time.Minute)
	m := newManager(context.Background(), cfg, vault, quietLogger(), engine, nil, names)
	t.Cleanup(func() {
		closeManager(t, m)
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		m.mu.Lock()
		s := m.state
		m.mu.Unlock()
		for name, role := range map[string]string{names.container: "runtime", names.seed: "credential-seed"} {
			var c containerInfo
			code, err := engine.DoJSON(ctx, http.MethodGet, "containers/"+name+"/json", nil, &c)
			if code == http.StatusNotFound {
				continue
			}
			if err != nil || !owns(c.Config.Labels, s, role) {
				t.Error("fixture container cleanup ownership failed")
				continue
			}
			if _, err = engine.DoJSON(ctx, http.MethodDelete, "containers/"+url.PathEscape(c.ID)+"?force=true", nil, nil); err != nil {
				t.Error("fixture container cleanup failed")
			}
		}
		for name, role := range map[string]string{names.data: "data", names.credentials: "credentials"} {
			var v volumeInfo
			code, err := engine.DoJSON(ctx, http.MethodGet, "volumes/"+name, nil, &v)
			if code == http.StatusNotFound {
				continue
			}
			if err != nil || !owns(v.Labels, s, role) {
				t.Error("fixture volume cleanup ownership failed")
				continue
			}
			if _, err = engine.DoJSON(ctx, http.MethodDelete, "volumes/"+name, nil, nil); err != nil {
				t.Error("fixture volume cleanup failed")
			}
		}
	})
	waitOperation(t, m, ActionInstall)
	status := m.Status()
	if !status.Ready || !status.Provisioned || status.AccessState != "ready" {
		t.Fatalf("runtime state %s/%s", status.State, status.AccessState)
	}
	lease, err := m.AcquireS3(context.Background(), cfg, true)
	if err != nil {
		t.Fatal(ErrorCode(err))
	}
	defer lease.Release()
	machine, err := newDataClient(lease.Endpoint, lease.AccessKey, lease.SecretKey, nil)
	if err != nil {
		t.Fatal(ErrorCode(err))
	}
	m.mu.Lock()
	s := m.state
	m.mu.Unlock()
	root, err := newDataClient(s.Endpoint, s.RootAccess, s.RootSecret, nil)
	if err != nil {
		t.Fatal(ErrorCode(err))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	_, err = machine.PutObject(ctx, cfg.RustFS.Bucket, "agent/persisted.txt", strings.NewReader("fixture"), 7, minio.PutObjectOptions{})
	if err != nil {
		t.Fatalf("allowed put: %s", minio.ToErrorResponse(err).Code)
	}
	object, err := machine.GetObject(ctx, cfg.RustFS.Bucket, "agent/persisted.txt", minio.GetObjectOptions{})
	if err != nil {
		t.Fatal("allowed get failed")
	}
	data, err := io.ReadAll(object)
	_ = object.Close()
	if err != nil || string(data) != "fixture" {
		t.Fatal("object bytes differ")
	}
	_, err = root.PutObject(ctx, cfg.RustFS.Bucket, "outside.txt", strings.NewReader("private"), 7, minio.PutObjectOptions{})
	if err != nil {
		t.Fatal("root fixture write failed")
	}
	_, err = machine.StatObject(ctx, cfg.RustFS.Bucket, "outside.txt", minio.StatObjectOptions{})
	assertDeniedS3(t, err)
	_, err = machine.PutObject(ctx, cfg.RustFS.Bucket, "outside-write.txt", strings.NewReader("x"), 1, minio.PutObjectOptions{})
	assertDeniedS3(t, err)
	foreign := "aurago-foreign-" + suffix
	if err = root.MakeBucket(ctx, foreign, minio.MakeBucketOptions{Region: Region}); err != nil {
		t.Fatal("foreign fixture bucket create failed")
	}
	_, err = machine.StatObject(ctx, foreign, "secret.txt", minio.StatObjectOptions{})
	assertDeniedS3(t, err)
	admin, err := newAdminClient(s.Endpoint, s.ChildAccess, s.ChildSecret, nil)
	if err != nil {
		t.Fatal(ErrorCode(err))
	}
	for _, operation := range []struct {
		method, name string
		query        url.Values
		body         any
	}{
		{http.MethodGet, "list-users", nil, nil},
		{http.MethodPut, "add-user", url.Values{"accessKey": {s.ParentAccess}}, map[string]string{"secretKey": s.ParentSecret, "status": "enabled"}},
		{http.MethodPut, "add-canned-policy", url.Values{"name": {s.PolicyName}}, map[string]any{"Version": "2012-10-17", "Statement": []any{}}},
		{http.MethodPut, "add-service-account", nil, map[string]string{"targetUser": s.ParentAccess}},
		{http.MethodPost, "account/password", nil, map[string]string{"current_secret_key": s.ParentSecret, "new_secret_key": s.ChildSecret}},
		{http.MethodPost, "account/mfa/enroll", nil, map[string]string{}},
	} {
		code, _ := admin.native(ctx, operation.method, operation.name, operation.query, operation.body, nil)
		if code != http.StatusBadRequest && code != http.StatusForbidden && code != http.StatusUnauthorized {
			t.Errorf("machine could invoke %s (HTTP %d)", operation.name, code)
		}
	}
	form := []byte("Action=AssumeRole&Version=2011-06-15&DurationSeconds=900")
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.Endpoint+"/", bytes.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	sum := sha256.Sum256(form)
	digest := hex.EncodeToString(sum[:])
	req.Header.Set("X-Amz-Content-Sha256", digest)
	if err = v4.NewSigner().SignHTTP(ctx, aws.Credentials{AccessKeyID: s.ChildAccess, SecretAccessKey: s.ChildSecret}, req, digest, "s3", Region, time.Now()); err != nil {
		t.Fatal("STS signing failed")
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal("STS request failed")
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("machine STS not denied: HTTP %d", response.StatusCode)
	}
	// Actual Copy/Get/Delete and paginated listings must obey the same scoped IAM.
	_, err = machine.CopyObject(ctx, minio.CopyDestOptions{Bucket: cfg.RustFS.Bucket, Object: "agent/copied.txt"}, minio.CopySrcOptions{Bucket: cfg.RustFS.Bucket, Object: "agent/persisted.txt"})
	if err != nil {
		t.Fatalf("allowed copy: %s", minio.ToErrorResponse(err).Code)
	}
	_, err = machine.CopyObject(ctx, minio.CopyDestOptions{Bucket: cfg.RustFS.Bucket, Object: "agent/denied-copy.txt"}, minio.CopySrcOptions{Bucket: cfg.RustFS.Bucket, Object: "outside.txt"})
	assertDeniedS3(t, err)
	if err = machine.RemoveObject(ctx, cfg.RustFS.Bucket, "agent/copied.txt", minio.RemoveObjectOptions{}); err != nil {
		t.Fatalf("allowed delete: %s", minio.ToErrorResponse(err).Code)
	}
	_, err = machine.PutObject(ctx, cfg.RustFS.Bucket, "agent/Grüße %20#.txt", strings.NewReader("encoded"), 7, minio.PutObjectOptions{})
	if err != nil {
		t.Fatalf("encoded key: %s", minio.ToErrorResponse(err).Code)
	}
	listed := 0
	for item := range machine.ListObjects(ctx, cfg.RustFS.Bucket, minio.ListObjectsOptions{Prefix: "agent/", Recursive: true, MaxKeys: 1}) {
		if item.Err != nil {
			t.Fatalf("bounded list: %s", minio.ToErrorResponse(item.Err).Code)
		}
		if !strings.HasPrefix(item.Key, "agent/") {
			t.Fatal("listing leaked a forbidden prefix")
		}
		listed++
	}
	if listed < 2 {
		t.Fatal("pagination lost allowed objects")
	}
	// The real installed MinIO client must complete multipart operations under the scoped policy.
	payload := bytes.Repeat([]byte("m"), 17<<20)
	_, err = machine.PutObject(ctx, cfg.RustFS.Bucket, "agent/multipart.bin", bytes.NewReader(payload), int64(len(payload)), minio.PutObjectOptions{PartSize: 5 << 20})
	if err != nil {
		t.Fatalf("multipart upload failed: %s", minio.ToErrorResponse(err).Code)
	}
	next := copyConfig(cfg)
	next.S3.ReadOnly = true
	if err = m.PrepareReconfigure(ctx, next); err != nil {
		t.Fatalf("prepare readonly: %s", ErrorCode(err))
	}
	if _, barrierErr := m.Begin(ActionStart); ErrorCode(barrierErr) != "configuration_pending" {
		t.Fatal("prepared IAM could be reenabled before config publication")
	}
	if lease.Check() == nil {
		t.Fatal("old local lease survived revocation")
	}
	_, err = machine.PutObject(ctx, cfg.RustFS.Bucket, "agent/revoked.txt", strings.NewReader("x"), 1, minio.PutObjectOptions{})
	if code := minio.ToErrorResponse(err).Code; code != "AccessDenied" && code != "InvalidAccessKeyId" {
		t.Fatalf("revoked credential error: %s", code)
	}
	m.Configure(next)
	until := time.Now().Add(3 * time.Minute)
	for time.Now().Before(until) {
		if m.Status().AccessState == "ready" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if m.Status().AccessState != "ready" {
		t.Fatalf("readonly reconciliation: %s", m.Status().Reason)
	}
	_, err = machine.PutObject(ctx, cfg.RustFS.Bucket, "agent/readonly.txt", strings.NewReader("x"), 1, minio.PutObjectOptions{})
	assertDeniedS3(t, err)
	// Explicit-region writes need no bucket-discovery/read grant.
	writeOnly := copyConfig(cfg)
	writeOnly.RustFS.Permissions.AllowRead = false
	writeOnly.RustFS.Permissions.AllowDelete = false
	if err = m.PrepareReconfigure(ctx, writeOnly); err != nil {
		t.Fatal(ErrorCode(err))
	}
	m.Configure(writeOnly)
	until = time.Now().Add(30 * time.Second)
	for time.Now().Before(until) {
		if m.Status().AccessState == "ready" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	_, err = machine.PutObject(ctx, cfg.RustFS.Bucket, "agent/write-only.txt", strings.NewReader("write"), 5, minio.PutObjectOptions{})
	if err != nil {
		t.Fatalf("write-only upload: %s", minio.ToErrorResponse(err).Code)
	}
	_, err = machine.StatObject(ctx, cfg.RustFS.Bucket, "agent/persisted.txt", minio.StatObjectOptions{})
	assertDeniedS3(t, err)
	if err = m.PrepareReconfigure(ctx, next); err != nil {
		t.Fatal(ErrorCode(err))
	}
	m.Configure(next)
	until = time.Now().Add(30 * time.Second)
	for time.Now().Before(until) {
		if m.Status().AccessState == "ready" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	waitOperation(t, m, ActionStop)
	waitOperation(t, m, ActionRemove)
	waitOperation(t, m, ActionStart)
	object, err = machine.GetObject(ctx, cfg.RustFS.Bucket, "agent/persisted.txt", minio.GetObjectOptions{})
	if err != nil {
		t.Fatal("retained get failed")
	}
	data, err = io.ReadAll(object)
	_ = object.Close()
	if err != nil || string(data) != "fixture" {
		t.Fatal("data lost after runtime recreation")
	}
	// A new AuraGo process must reconcile tighter saved rights before admitting data access.
	closeManager(t, m)
	tightened := copyConfig(next)
	tightened.RustFS.Permissions.AllowedLocations[0].Prefixes = []string{"agent/private/"}
	m = newManager(context.Background(), tightened, vault, quietLogger(), engine, nil, names)
	until = time.Now().Add(30 * time.Second)
	for time.Now().Before(until) {
		if m.Status().AccessState == "ready" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if m.Status().AccessState != "ready" {
		t.Fatalf("startup IAM reconciliation failed: %s", m.Status().Reason)
	}
	_, err = machine.StatObject(ctx, cfg.RustFS.Bucket, "agent/persisted.txt", minio.StatObjectOptions{})
	assertDeniedS3(t, err)
	// Saved disable intent stops an established runtime; later process restarts preserve the stop.
	closeManager(t, m)
	disabled := copyConfig(tightened)
	disabled.RustFS.Enabled = false
	m = newManager(context.Background(), disabled, vault, quietLogger(), engine, nil, names)
	until = time.Now().Add(30 * time.Second)
	for time.Now().Before(until) {
		if m.Status().State == "stopped" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if m.Status().State != "stopped" {
		t.Fatalf("startup disable not confirmed: %s", m.Status().Reason)
	}
	closeManager(t, m)
	m = newManager(context.Background(), tightened, vault, quietLogger(), engine, nil, names)
	if _, err = m.Preflight(ctx); err != nil {
		t.Fatal(ErrorCode(err))
	}
	if m.Status().Running {
		t.Fatal("explicit stopped intent was lost")
	}
	waitOperation(t, m, ActionStart)
	// Missing initialized IAM is a recovery condition, never a request to create another identity.
	m.mu.Lock()
	s = m.state
	m.mu.Unlock()
	rootAdmin, err := m.rootAdmin(s)
	if err != nil {
		t.Fatal(ErrorCode(err))
	}
	if err = rootAdmin.mutation(ctx, http.MethodDelete, "delete-service-account", url.Values{"accessKey": {s.ChildAccess}}, nil, nil); err != nil {
		t.Fatal("fixture child deletion failed")
	}
	if _, err = m.Preflight(ctx); ErrorCode(err) != "recovery_required" {
		t.Fatal("missing initialized child was accepted")
	}
	attempted, err := m.Begin(ActionStart)
	if err != nil {
		t.Fatal(ErrorCode(err))
	}
	until = time.Now().Add(30 * time.Second)
	for time.Now().Before(until) {
		attempted, _ = m.Operation(attempted.ID)
		if attempted.State == "failed" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if attempted.Error != "recovery_required" {
		t.Fatalf("lost child recovery status: %s", attempted.Error)
	}
	code, _ := rootAdmin.native(ctx, http.MethodGet, "info-service-account", url.Values{"accessKey": {s.ChildAccess}}, nil, nil)
	if code != http.StatusNotFound {
		t.Fatal("missing child was silently recreated")
	}
	t.Log("native IAM, scoped S3/multipart, privilege denial, live/startup revocation, stopped intent, retained data and lost-IAM recovery passed")
}
