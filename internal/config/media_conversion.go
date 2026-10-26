package config

import (
	"fmt"
	"strings"
)

const (
	// MediaConversionMinTimeoutSeconds is the shortest conversion timeout an
	// operator may configure. 0 keeps the built-in default.
	MediaConversionMinTimeoutSeconds = 5
	// MediaConversionMaxTimeoutSeconds is the longest conversion timeout an
	// operator may configure.
	MediaConversionMaxTimeoutSeconds = 1800
	// MediaConversionMaxFileSizeMB is the largest accepted input size cap.
	// 0 means unlimited.
	MediaConversionMaxFileSizeMB = 102400
	// MediaConversionMaxBinaryPathLength bounds a configured binary path.
	MediaConversionMaxBinaryPathLength = 1024
)

// NormalizeMediaConversionConfig clamps media conversion settings into the
// supported range. It runs while loading the configuration so that an
// out-of-range value in config.yaml cannot block the startup.
func NormalizeMediaConversionConfig(cfg *MediaConversionConfig) {
	if cfg == nil {
		return
	}
	if cfg.TimeoutSeconds != 0 {
		if cfg.TimeoutSeconds < MediaConversionMinTimeoutSeconds {
			cfg.TimeoutSeconds = MediaConversionMinTimeoutSeconds
		} else if cfg.TimeoutSeconds > MediaConversionMaxTimeoutSeconds {
			cfg.TimeoutSeconds = MediaConversionMaxTimeoutSeconds
		}
	}
	if cfg.MaxFileSizeMB < 0 {
		cfg.MaxFileSizeMB = 0
	} else if cfg.MaxFileSizeMB > MediaConversionMaxFileSizeMB {
		cfg.MaxFileSizeMB = MediaConversionMaxFileSizeMB
	}
}

// ValidateMediaConversionConfig rejects media conversion settings that the
// tool cannot honor. It guards the configuration API, where a rejected value
// must be reported instead of silently corrected.
func ValidateMediaConversionConfig(cfg *MediaConversionConfig) error {
	if cfg == nil {
		return nil
	}
	if cfg.TimeoutSeconds != 0 && (cfg.TimeoutSeconds < MediaConversionMinTimeoutSeconds || cfg.TimeoutSeconds > MediaConversionMaxTimeoutSeconds) {
		return fmt.Errorf("media conversion timeout_seconds must be between %d and %d", MediaConversionMinTimeoutSeconds, MediaConversionMaxTimeoutSeconds)
	}
	if cfg.MaxFileSizeMB < 0 || cfg.MaxFileSizeMB > MediaConversionMaxFileSizeMB {
		return fmt.Errorf("media conversion max_file_size_mb must be between 0 and %d", MediaConversionMaxFileSizeMB)
	}
	for _, binary := range []struct {
		name string
		path string
	}{
		{"ffmpeg_path", cfg.FFmpegPath},
		{"imagemagick_path", cfg.ImageMagickPath},
	} {
		if err := validateMediaConversionBinaryPath(binary.name, binary.path); err != nil {
			return err
		}
	}
	return nil
}

func validateMediaConversionBinaryPath(name, path string) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil
	}
	if len(trimmed) > MediaConversionMaxBinaryPathLength {
		return fmt.Errorf("media conversion %s must not exceed %d characters", name, MediaConversionMaxBinaryPathLength)
	}
	if strings.ContainsAny(trimmed, "\x00\r\n") {
		return fmt.Errorf("media conversion %s must not contain control characters", name)
	}
	return nil
}
