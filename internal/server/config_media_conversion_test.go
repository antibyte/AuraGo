package server

import (
	"strings"
	"testing"
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
