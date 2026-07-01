package tools

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestS3ReadOnlyBlocksDirectMutations(t *testing.T) {
	cfg := S3Config{AccessKey: "test-access", SecretKey: "test-secret", ReadOnly: true}

	for name, got := range map[string]string{
		"upload": ExecuteS3(cfg, "upload", "bucket", "key", "local.txt", "", "", ""),
		"delete": ExecuteS3(cfg, "delete", "bucket", "key", "", "", "", ""),
		"copy":   ExecuteS3(cfg, "copy", "bucket", "key", "", "", "bucket", "copy-key"),
		"move":   ExecuteS3(cfg, "move", "bucket", "key", "", "", "bucket", "move-key"),
	} {
		t.Run(name, func(t *testing.T) {
			var result s3Result
			if err := json.Unmarshal([]byte(got), &result); err != nil {
				t.Fatalf("decode result: %v\nraw=%s", err, got)
			}
			if result.Status != "error" || !strings.Contains(result.Message, "read-only mode") {
				t.Fatalf("response = %+v, want read-only denial", result)
			}
		})
	}
}

func TestS3EndpointRejectsHTTPUnlessInsecure(t *testing.T) {
	cfg := S3Config{AccessKey: "test-access", SecretKey: "test-secret", Endpoint: "http://minio.local:9000"}
	if _, err := newS3Client(cfg); err == nil || !strings.Contains(err.Error(), "s3.insecure=true") {
		t.Fatalf("newS3Client(http, insecure=false) error = %v, want insecure opt-in error", err)
	}

	cfg.Insecure = true
	if _, err := newS3Client(cfg); err != nil {
		t.Fatalf("newS3Client(http, insecure=true) error = %v, want nil", err)
	}

	cfg.Endpoint = "https://s3.amazonaws.com"
	cfg.Insecure = false
	if _, err := newS3Client(cfg); err != nil {
		t.Fatalf("newS3Client(https) error = %v, want nil", err)
	}
}

func TestS3ResolveDownloadDestinationRequiresWriteAndWorkspace(t *testing.T) {
	tmp := t.TempDir()
	workspaceDir := filepath.Join(tmp, "agent_workspace", "workdir")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	cfg := S3Config{WorkspaceDir: workspaceDir}

	if _, err := resolveS3DownloadDestination(cfg, "reports/out.txt", ""); err == nil || !strings.Contains(err.Error(), "filesystem write") {
		t.Fatalf("resolve without write permission error = %v, want filesystem write denial", err)
	}

	cfg.AllowFilesystemWrite = true
	got, err := resolveS3DownloadDestination(cfg, "reports/out.txt", "")
	if err != nil {
		t.Fatalf("resolve empty local_path: %v", err)
	}
	if want := filepath.Join(workspaceDir, "out.txt"); got != want {
		t.Fatalf("empty local_path resolved to %q, want %q", got, want)
	}

	got, err = resolveS3DownloadDestination(cfg, "reports/out.txt", "workdir/downloads/out.txt")
	if err != nil {
		t.Fatalf("resolve workdir local_path: %v", err)
	}
	if want := filepath.Join(workspaceDir, "downloads", "out.txt"); got != want {
		t.Fatalf("workdir local_path resolved to %q, want %q", got, want)
	}

	_, err = resolveS3DownloadDestination(cfg, "reports/out.txt", "workdir")
	if err == nil || !strings.Contains(err.Error(), "directory") {
		t.Fatalf("resolve workspace directory destination error = %v, want directory denial", err)
	}

	_, err = resolveS3DownloadDestination(cfg, "reports/out.txt", filepath.Join(tmp, "outside.txt"))
	if err == nil || !strings.Contains(err.Error(), "workspace") {
		t.Fatalf("resolve outside workspace error = %v, want workspace confinement error", err)
	}
}

func TestS3ResolveUploadSourceAllowsOnlyWorkspaceOrData(t *testing.T) {
	tmp := t.TempDir()
	workspaceDir := filepath.Join(tmp, "agent_workspace", "workdir")
	dataDir := filepath.Join(tmp, "data")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("create data dir: %v", err)
	}
	workspaceFile := filepath.Join(workspaceDir, "upload.txt")
	dataFile := filepath.Join(dataDir, "media", "upload.txt")
	if err := os.WriteFile(workspaceFile, []byte("workspace"), 0o644); err != nil {
		t.Fatalf("write workspace file: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(dataFile), 0o755); err != nil {
		t.Fatalf("create data child dir: %v", err)
	}
	if err := os.WriteFile(dataFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("write data file: %v", err)
	}

	cfg := S3Config{WorkspaceDir: workspaceDir, DataDir: dataDir}
	got, err := resolveS3UploadSource(cfg, "workdir/upload.txt")
	if err != nil {
		t.Fatalf("resolve workspace upload source: %v", err)
	}
	if got != workspaceFile {
		t.Fatalf("workspace upload source = %q, want %q", got, workspaceFile)
	}

	got, err = resolveS3UploadSource(cfg, dataFile)
	if err != nil {
		t.Fatalf("resolve data upload source: %v", err)
	}
	if got != dataFile {
		t.Fatalf("data upload source = %q, want %q", got, dataFile)
	}

	outsideFile := filepath.Join(tmp, "outside.txt")
	if err := os.WriteFile(outsideFile, []byte("outside"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	_, err = resolveS3UploadSource(cfg, outsideFile)
	if err == nil || !strings.Contains(err.Error(), "workspace or data") {
		t.Fatalf("resolve outside upload source error = %v, want confinement error", err)
	}
}

func TestWriteS3DownloadAtomicKeepsExistingFileOnCopyError(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(dest, []byte("original"), 0o644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	_, err := writeS3DownloadAtomic(dest, failingReader{})
	if err == nil || !strings.Contains(err.Error(), "copy") {
		t.Fatalf("writeS3DownloadAtomic error = %v, want copy error", err)
	}

	content, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read original after failure: %v", err)
	}
	if string(content) != "original" {
		t.Fatalf("destination content = %q, want original", content)
	}

	matches, err := filepath.Glob(filepath.Join(dir, ".out.txt.*.tmp"))
	if err != nil {
		t.Fatalf("glob temp files: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temp files left behind: %v", matches)
	}
}

func TestResolveS3DestinationBucketDefaultsToSource(t *testing.T) {
	if got := resolveS3DestinationBucket("source-bucket", ""); got != "source-bucket" {
		t.Fatalf("default destination bucket = %q, want source-bucket", got)
	}
	if got := resolveS3DestinationBucket("source-bucket", "archive-bucket"); got != "archive-bucket" {
		t.Fatalf("explicit destination bucket = %q, want archive-bucket", got)
	}
}

type failingReader struct{}

func (failingReader) Read(_ []byte) (int, error) {
	return 0, errors.New("boom")
}
