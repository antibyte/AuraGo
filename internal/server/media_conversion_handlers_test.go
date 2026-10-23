package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestValidateMediaConversionBinaryPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	converter := filepath.Join(dir, "ffmpeg")
	if err := os.WriteFile(converter, []byte("binary"), 0o750); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(dir, "payload.exe")
	if err := os.WriteFile(foreign, []byte("binary"), 0o750); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		path    string
		wantErr string
	}{
		{"empty is allowed", "", ""},
		{"approved converter", converter, ""},
		{"missing file", filepath.Join(dir, "missing"), "was not found"},
		{"directory", dir, "not a regular file"},
		{"foreign binary", foreign, "not an approved media converter"},
		{"control characters", "ffmpeg\n.exe", "control characters"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMediaConversionBinaryPath(tc.path)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("validateMediaConversionBinaryPath(%q) = %v, want nil", tc.path, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateMediaConversionBinaryPath(%q) = nil, want an error", tc.path)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want it to mention %q", err, tc.wantErr)
			}
		})
	}
}

func TestValidateMediaConversionBinaryPathAcceptsBareNameOnPath(t *testing.T) {
	// A bare name is resolved on PATH exactly like the tool resolves it, so the
	// documented default must not be refused. t.Setenv forbids t.Parallel.
	binDir := t.TempDir()
	for _, name := range []string{"ffmpeg", "ffmpeg.exe"} {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte("binary"), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("Path", binDir+string(os.PathListSeparator)+os.Getenv("Path"))

	if err := validateMediaConversionBinaryPath("ffmpeg"); err != nil {
		t.Fatalf("a converter on PATH must be accepted: %v", err)
	}
}

func TestValidateMediaConversionBinaryPathAcceptsVersionedName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	versioned := filepath.Join(dir, "ffmpeg-7.0.exe")
	if err := os.WriteFile(versioned, []byte("binary"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := validateMediaConversionBinaryPath(versioned); err != nil {
		t.Fatalf("a versioned ffmpeg must be accepted: %v", err)
	}
}

func TestValidateMediaConversionBinaryPathRejectsWindowsSystemConvert(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the convert.exe trap only exists on Windows")
	}
	t.Parallel()

	dir := t.TempDir()
	convert := filepath.Join(dir, "convert.exe")
	if err := os.WriteFile(convert, []byte("binary"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := validateMediaConversionBinaryPath(convert); err == nil {
		t.Fatal("the Windows system convert.exe must be refused")
	}
}

func TestHandleMediaConversionTestRejectsForeignBinary(t *testing.T) {
	t.Parallel()

	s := &Server{Cfg: &config.Config{}}
	s.Cfg.Tools.MediaConversion.Enabled = true

	foreign := filepath.Join(t.TempDir(), "payload.exe")
	if err := os.WriteFile(foreign, []byte("not a converter"), 0o750); err != nil {
		t.Fatal(err)
	}

	body := fmt.Sprintf(`{"media_conversion":{"enabled":true,"ffmpeg_path":%q}}`, foreign)
	req := httptest.NewRequest(http.MethodPost, "/api/media-conversion/test", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handleMediaConversionTest(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "not an approved media converter") {
		t.Fatalf("body = %q, want the rejection reason", rec.Body.String())
	}
}

func TestHandleMediaConversionTestRejectsMissingBinary(t *testing.T) {
	t.Parallel()

	s := &Server{Cfg: &config.Config{}}
	s.Cfg.Tools.MediaConversion.Enabled = true

	missing := filepath.Join(t.TempDir(), "ffmpeg")
	body := fmt.Sprintf(`{"media_conversion":{"enabled":true,"ffmpeg_path":%q}}`, missing)
	req := httptest.NewRequest(http.MethodPost, "/api/media-conversion/test", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handleMediaConversionTest(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "was not found") {
		t.Fatalf("body = %q, want the not-found reason", rec.Body.String())
	}
}

func TestHandleMediaConversionTestReportsDisabledTool(t *testing.T) {
	t.Parallel()

	s := &Server{Cfg: &config.Config{}}
	s.Cfg.Tools.MediaConversion.Enabled = false

	req := httptest.NewRequest(http.MethodPost, "/api/media-conversion/test", strings.NewReader("{}"))
	rec := httptest.NewRecorder()

	handleMediaConversionTest(s).ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), "disabled") {
		t.Fatalf("body = %q, want the disabled message", rec.Body.String())
	}
}

func TestHandleMediaConversionTestRequiresAdmin(t *testing.T) {
	t.Parallel()

	s := &Server{Cfg: &config.Config{}}
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.SessionSecret = "admin-session-secret"
	s.Cfg.Tools.MediaConversion.Enabled = true

	req := httptest.NewRequest(http.MethodPost, "/api/media-conversion/test", strings.NewReader("{}"))
	rec := httptest.NewRecorder()

	requireAdmin(s, handleMediaConversionTest(s)).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMediaConversionTestPathIsAdminProtected(t *testing.T) {
	t.Parallel()

	if !isAdminProtectedPath("/api/media-conversion/test") {
		t.Fatal("the media conversion health probe must be admin protected")
	}
}
