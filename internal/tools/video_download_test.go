package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestParseYtDlpSearch(t *testing.T) {
	output := strings.Join([]string{
		`{"id":"dQw4w9WgXcQ","title":"Example","uploader":"Uploader","duration":42,"view_count":1000}`,
		`warning: ignored`,
		`{"id":"def456","title":"Second","url":"https://example.test/video","thumbnail":"https://img.test/t.jpg"}`,
	}, "\n")

	items := parseYtDlpSearch(output)
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].URL != "https://www.youtube.com/watch?v=dQw4w9WgXcQ" {
		t.Fatalf("first URL = %q", items[0].URL)
	}
	if items[1].URL != "https://example.test/video" {
		t.Fatalf("second URL = %q", items[1].URL)
	}
}

func TestParseYtDlpSearchTruncatesLongDescription(t *testing.T) {
	longDescription := strings.Repeat("a", 260)
	output := `{"id":"abc123","title":"Example","description":"` + longDescription + `"}`

	items := parseYtDlpSearch(output)
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if len(items[0].Description) > 203 {
		t.Fatalf("description length = %d, want <= 203", len(items[0].Description))
	}
	if !strings.HasSuffix(items[0].Description, "...") {
		t.Fatalf("description should end with truncation marker: %q", items[0].Description)
	}
}

func TestResolveDownloadedFilePathMapsContainerPath(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "video.mp4")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	cfg := &config.Config{}
	cfg.Tools.VideoDownload.Mode = "docker"

	got, err := resolveDownloadedFilePath(cfg, dir, "/downloads/video.mp4\n")
	if err != nil {
		t.Fatalf("resolveDownloadedFilePath() error = %v", err)
	}
	if got != filePath {
		t.Fatalf("path = %q, want %q", got, filePath)
	}
}

func TestResolveDownloadedFilePathRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "clip.mp4")
	if err := os.Symlink(outside, link); err != nil {
		t.Skip("symlinks are not available here")
	}
	cfg := &config.Config{}
	cfg.Tools.VideoDownload.Mode = "native"
	if _, err := resolveDownloadedFilePath(cfg, dir, link); err == nil {
		t.Fatal("symlink path was accepted")
	}
}

func TestResolveDownloadedFilePathRejectsTraversalOutsideDownloadDir(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Clean(filepath.Join(dir, "..", "escape.mp4"))
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	cfg := &config.Config{}
	cfg.Tools.VideoDownload.Mode = "docker"

	got, err := resolveDownloadedFilePath(cfg, dir, "/downloads/../escape.mp4\n")
	if err == nil {
		t.Fatalf("resolveDownloadedFilePath() = %q, want traversal error", got)
	}
	if strings.Contains(filepath.Clean(got), filepath.Base(outside)) {
		t.Fatalf("resolver returned escaped path %q", got)
	}
}

func TestBuildYtDlpDownloadArgsUsesModeSpecificOutputDir(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.VideoDownload.MaxFileSizeMB = 500
	req := VideoDownloadRequest{Format: "video"}

	dockerArgs := buildYtDlpDownloadArgs(cfg, req, "https://youtu.be/dQw4w9WgXcQ", "video", videoDownloadContainerDir)
	if !strings.Contains(strings.Join(dockerArgs, " "), "/downloads/%(title).200B") {
		t.Fatalf("docker args should use container download dir: %v", dockerArgs)
	}
	if !strings.Contains(strings.Join(dockerArgs, " "), "--max-filesize 500M") {
		t.Fatalf("docker args should enforce yt-dlp max filesize: %v", dockerArgs)
	}

	nativeDir := filepath.Join(t.TempDir(), "downloads")
	nativeArgs := buildYtDlpDownloadArgs(cfg, req, "https://youtu.be/dQw4w9WgXcQ", "video", nativeDir)
	if !strings.Contains(strings.Join(nativeArgs, " "), filepath.ToSlash(nativeDir)+"/%(title).200B") {
		t.Fatalf("native args should use host download dir: %v", nativeArgs)
	}
}

func TestValidateVideoDownloadURLRejectsUnsafeSchemes(t *testing.T) {
	for _, raw := range []string{
		"file:///etc/passwd",
		"javascript:alert(1)",
		"ftp://example.com/video.mp4",
		"not a url",
	} {
		t.Run(raw, func(t *testing.T) {
			if err := validateVideoDownloadURL(raw); err == nil {
				t.Fatalf("validateVideoDownloadURL(%q) succeeded, want error", raw)
			}
		})
	}
	if err := validateVideoDownloadURL("https://www.youtube.com/watch?v=dQw4w9WgXcQ"); err != nil {
		t.Fatalf("validateVideoDownloadURL(https) error = %v", err)
	}
}

func TestValidateVideoDownloadURLRejectsInternalTargets(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		"https://127.0.0.1/video.mp4",
		"http://169.254.169.254/latest/meta-data",
		"https://user:secret@www.youtube.com/watch?v=dQw4w9WgXcQ",
		"https://192.168.1.10/cam.m3u8",
	} {
		t.Run(raw, func(t *testing.T) {
			if err := validateVideoDownloadURL(raw); err == nil {
				t.Fatalf("validateVideoDownloadURL(%q) succeeded, want error", raw)
			}
		})
	}
}

func TestResolveVideoDownloadDirRejectsAbsolutePathOutsideProject(t *testing.T) {
	projectRoot := t.TempDir()
	workspace := filepath.Join(projectRoot, "agent_workspace", "workdir")
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = workspace
	cfg.Tools.VideoDownload.DownloadDir = filepath.Join(t.TempDir(), "external-downloads")

	if got, err := resolveVideoDownloadDir(cfg); err == nil {
		t.Fatalf("resolveVideoDownloadDir() = %q, want external path rejection", got)
	}
}

func TestResolveVideoDownloadDirRejectsRelativeTraversalOutsideProject(t *testing.T) {
	projectRoot := t.TempDir()
	workspace := filepath.Join(projectRoot, "agent_workspace", "workdir")
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = workspace
	cfg.Tools.VideoDownload.DownloadDir = "../escape"

	if got, err := resolveVideoDownloadDir(cfg); err == nil {
		t.Fatalf("resolveVideoDownloadDir() = %q, want traversal rejection", got)
	}
}

func TestEnforceVideoDownloadSize(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.VideoDownload.MaxFileSizeMB = 1
	if err := enforceVideoDownloadSize(cfg, 1024*1024); err != nil {
		t.Fatalf("size at limit returned error: %v", err)
	}
	if err := enforceVideoDownloadSize(cfg, 1024*1024+1); err == nil {
		t.Fatal("expected error for oversized file")
	}
}

func TestVideoDownloadModeDefaultsToDocker(t *testing.T) {
	cfg := &config.Config{}
	if got := videoDownloadMode(cfg); got != "docker" {
		t.Fatalf("mode = %q, want docker", got)
	}
	cfg.Tools.VideoDownload.Mode = "native"
	if got := videoDownloadMode(cfg); got != "native" {
		t.Fatalf("mode = %q, want native", got)
	}
}

func TestDispatchVideoDownloadRequiresExplicitDownloadPermission(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.VideoDownload.Enabled = true
	cfg.Tools.VideoDownload.AllowDownload = false

	raw := DispatchVideoDownload(context.Background(), cfg, nil, VideoDownloadRequest{
		Operation: "download",
		URL:       "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
	}, nil)

	var result videoDownloadResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode result: %v\n%s", err, raw)
	}
	if result.Status != "error" {
		t.Fatalf("status = %q, want error", result.Status)
	}
	if !strings.Contains(result.Message, "allow_download") {
		t.Fatalf("message = %q, want allow_download guidance", result.Message)
	}
}

func TestDispatchVideoDownloadRequiresExplicitTranscribePermission(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.VideoDownload.Enabled = true
	cfg.Tools.VideoDownload.AllowTranscribe = false

	raw := DispatchVideoDownload(context.Background(), cfg, nil, VideoDownloadRequest{
		Operation: "transcribe",
		URL:       "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
	}, nil)

	var result videoDownloadResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode result: %v\n%s", err, raw)
	}
	if result.Status != "error" {
		t.Fatalf("status = %q, want error", result.Status)
	}
	if !strings.Contains(result.Message, "allow_transcribe") {
		t.Fatalf("message = %q, want allow_transcribe guidance", result.Message)
	}
}

func TestVideoDownloadHostConfigIsBounded(t *testing.T) {
	t.Parallel()
	host := videoDownloadHostConfig(`C:\downloads`)
	if host["Memory"] != int64(512<<20) || host["NanoCpus"] != int64(1_000_000_000) || host["PidsLimit"] != int64(64) {
		t.Fatalf("limits = %#v", host)
	}
	if host["ReadonlyRootfs"] != true {
		t.Fatal("root filesystem must be read-only")
	}
	opts, _ := host["SecurityOpt"].([]string)
	if len(opts) != 1 || opts[0] != "no-new-privileges:true" {
		t.Fatalf("SecurityOpt = %#v", host["SecurityOpt"])
	}
	tmpfs, _ := host["Tmpfs"].(map[string]string)
	if tmpfs["/tmp"] != "rw,nosuid,nodev,size=64m" {
		t.Fatalf("Tmpfs = %#v", tmpfs)
	}
}

func TestVideoDownloadResultOmitsHostPathAndBoundsText(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("ä", 250)
	raw := videoDownloadJSON(videoDownloadResult{
		Status:   "ok",
		FilePath: `C:\secret\clip.mp4`,
		WebPath:  "/files/downloads/clip.mp4",
		Info:     compactVideoInfo(map[string]interface{}{"description": long, "title": "clip"}),
		Message:  commandErrMessage(fmt.Errorf("yt-dlp failed"), long),
	})
	if strings.Contains(raw, `C:\secret`) || strings.Contains(raw, "file_path") {
		t.Fatalf("host path leaked: %s", raw)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatal(err)
	}
	info := decoded["info"].(map[string]interface{})
	if got := []rune(info["description"].(string)); len(got) > 203 {
		t.Fatalf("description runes = %d", len(got))
	}
}
