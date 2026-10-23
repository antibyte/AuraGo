package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

func TestValidateMediaConversionPatchRejectsOutOfRangeValues(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		patch   map[string]interface{}
		wantErr string
	}{
		{
			name: "timeout too large",
			patch: map[string]interface{}{
				"tools": map[string]interface{}{
					"media_conversion": map[string]interface{}{"timeout_seconds": 999999},
				},
			},
			wantErr: "timeout_seconds",
		},
		{
			// The configuration API decodes JSON, so every number arrives as a
			// float64. The same value must be refused on that path too.
			name: "timeout too large as a JSON number",
			patch: map[string]interface{}{
				"tools": map[string]interface{}{
					"media_conversion": map[string]interface{}{"timeout_seconds": float64(999999)},
				},
			},
			wantErr: "timeout_seconds",
		},
		{
			name: "size cap too large as a JSON number",
			patch: map[string]interface{}{
				"tools": map[string]interface{}{
					"media_conversion": map[string]interface{}{"max_file_size_mb": float64(999999)},
				},
			},
			wantErr: "max_file_size_mb",
		},
		{
			name: "timeout too small",
			patch: map[string]interface{}{
				"tools": map[string]interface{}{
					"media_conversion": map[string]interface{}{"timeout_seconds": 1},
				},
			},
			wantErr: "timeout_seconds",
		},
		{
			name: "size cap negative",
			patch: map[string]interface{}{
				"tools": map[string]interface{}{
					"media_conversion": map[string]interface{}{"max_file_size_mb": -1},
				},
			},
			wantErr: "max_file_size_mb",
		},
		{
			name: "binary path with a control character",
			patch: map[string]interface{}{
				"tools": map[string]interface{}{
					"media_conversion": map[string]interface{}{"ffmpeg_path": "C:\\ffmpeg\n.exe"},
				},
			},
			wantErr: "ffmpeg_path",
		},
		{
			name: "valid timeout",
			patch: map[string]interface{}{
				"tools": map[string]interface{}{
					"media_conversion": map[string]interface{}{"timeout_seconds": 300},
				},
			},
		},
		{
			name: "unrelated section",
			patch: map[string]interface{}{
				"tools": map[string]interface{}{
					"other_tool": map[string]interface{}{"timeout_seconds": 999999},
				},
			},
		},
		{
			name:  "empty patch",
			patch: map[string]interface{}{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMediaConversionPatch(tc.patch)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("validateMediaConversionPatch = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateMediaConversionPatch = nil, want an error mentioning %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want it to mention %q", err, tc.wantErr)
			}
		})
	}
}

func TestValidateMediaConversionPatchIgnoresPreExistingValues(t *testing.T) {
	t.Parallel()

	// A value that already sits in config.yaml must not block a save that does
	// not touch the media conversion section.
	patch := map[string]interface{}{
		"tools": map[string]interface{}{
			"other_tool": map[string]interface{}{"enabled": true},
		},
	}
	if err := validateMediaConversionPatch(patch); err != nil {
		t.Fatalf("an unrelated save must not be blocked: %v", err)
	}

	// A patch that changes the section is validated against the patched value.
	mediaPatch := map[string]interface{}{
		"tools": map[string]interface{}{
			"media_conversion": map[string]interface{}{"timeout_seconds": 60},
		},
	}
	if err := validateMediaConversionPatch(mediaPatch); err != nil {
		t.Fatalf("a valid patched value must be accepted: %v", err)
	}
}

// A hand-edited config.yaml with an out-of-range media conversion value must
// not make every other configuration save fail, and a save that writes an
// out-of-range value must be refused.
func TestHandleUpdateConfigValidatesMediaConversionPatch(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	raw := "agent:\n  system_language: Deutsch\ntools:\n  media_conversion:\n    enabled: true\n    timeout_seconds: 999999\n"
	if err := os.WriteFile(configPath, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	vault, err := security.NewVault(strings.Repeat("57", 32), filepath.Join(tmpDir, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	loaded.ConfigPath = configPath
	s := &Server{Cfg: loaded, Logger: slog.Default(), Vault: vault}
	save := func(payload string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		handleUpdateConfig(s).ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(payload)))
		return recorder
	}

	// The out-of-range value in config.yaml is clamped by config.Load and must
	// not block a save that does not touch the media conversion section.
	if got := save(`{"agent":{"system_language":"English"}}`); got.Code != http.StatusOK {
		t.Fatalf("unrelated save = %d %s, want 200", got.Code, got.Body.String())
	}
	// A save that writes an out-of-range value is refused.
	if got := save(`{"tools":{"media_conversion":{"timeout_seconds":999999}}}`); got.Code != http.StatusBadRequest {
		t.Fatalf("out-of-range save = %d %s, want 400", got.Code, got.Body.String())
	}
	// A valid value is accepted.
	if got := save(`{"tools":{"media_conversion":{"timeout_seconds":300}}}`); got.Code != http.StatusOK {
		t.Fatalf("valid save = %d %s, want 200", got.Code, got.Body.String())
	}
}
