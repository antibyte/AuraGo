package config

import (
	"strings"
	"testing"
)

func TestValidateMediaConversionConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		cfg     MediaConversionConfig
		wantErr string
	}{
		{"defaults", MediaConversionConfig{}, ""},
		{"valid timeout", MediaConversionConfig{TimeoutSeconds: 120}, ""},
		{"minimum timeout", MediaConversionConfig{TimeoutSeconds: MediaConversionMinTimeoutSeconds}, ""},
		{"maximum timeout", MediaConversionConfig{TimeoutSeconds: MediaConversionMaxTimeoutSeconds}, ""},
		{"timeout too small", MediaConversionConfig{TimeoutSeconds: 1}, "timeout_seconds"},
		{"timeout too large", MediaConversionConfig{TimeoutSeconds: 999999}, "timeout_seconds"},
		{"size cap negative", MediaConversionConfig{MaxFileSizeMB: -1}, "max_file_size_mb"},
		{"size cap too large", MediaConversionConfig{MaxFileSizeMB: MediaConversionMaxFileSizeMB + 1}, "max_file_size_mb"},
		{"size cap unlimited", MediaConversionConfig{MaxFileSizeMB: 0}, ""},
		{"binary path with newline", MediaConversionConfig{FFmpegPath: "C:\\ffmpeg\n.exe"}, "ffmpeg_path"},
		{"binary path too long", MediaConversionConfig{ImageMagickPath: strings.Repeat("a", MediaConversionMaxBinaryPathLength+1)}, "imagemagick_path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateMediaConversionConfig(&tc.cfg)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateMediaConversionConfig = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateMediaConversionConfig = nil, want an error mentioning %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want it to mention %q", err, tc.wantErr)
			}
		})
	}
}

func TestNormalizeMediaConversionConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		cfg          MediaConversionConfig
		wantTimeout  int
		wantSizeCap  int
	}{
		{"timeout too large", MediaConversionConfig{TimeoutSeconds: 999999}, MediaConversionMaxTimeoutSeconds, 0},
		{"timeout too small", MediaConversionConfig{TimeoutSeconds: 1}, MediaConversionMinTimeoutSeconds, 0},
		{"timeout unset keeps default", MediaConversionConfig{}, 0, 0},
		{"size cap negative", MediaConversionConfig{MaxFileSizeMB: -5}, 0, 0},
		{"size cap too large", MediaConversionConfig{MaxFileSizeMB: MediaConversionMaxFileSizeMB + 1}, 0, MediaConversionMaxFileSizeMB},
		{"valid values stay", MediaConversionConfig{TimeoutSeconds: 120, MaxFileSizeMB: 500}, 120, 500},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.cfg
			NormalizeMediaConversionConfig(&cfg)
			if cfg.TimeoutSeconds != tc.wantTimeout {
				t.Fatalf("timeout = %d, want %d", cfg.TimeoutSeconds, tc.wantTimeout)
			}
			if cfg.MaxFileSizeMB != tc.wantSizeCap {
				t.Fatalf("size cap = %d, want %d", cfg.MaxFileSizeMB, tc.wantSizeCap)
			}
		})
	}
}
