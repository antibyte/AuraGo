package server

import (
	"encoding/json"
	"strconv"
	"strings"

	"aurago/internal/config"
)

// mediaConversionPatchSection returns the media_conversion keys a patch changes.
func mediaConversionPatchSection(patch map[string]interface{}) map[string]interface{} {
	if patch == nil {
		return nil
	}
	toolsSection, ok := patch["tools"].(map[string]interface{})
	if !ok {
		return nil
	}
	mediaSection, ok := toolsSection["media_conversion"].(map[string]interface{})
	if !ok || len(mediaSection) == 0 {
		return nil
	}
	return mediaSection
}

// validateMediaConversionPatch rejects only the media conversion values that the
// patch itself changes.
//
// The configuration API must refuse an out-of-range timeout or size cap that the
// caller just wrote, but a value that already sits in config.yaml must not block
// an unrelated save: config.Load clamps it at startup, and the operator can fix
// it whenever the media conversion section is edited.
func validateMediaConversionPatch(patch map[string]interface{}) error {
	changed := mediaConversionPatchSection(patch)
	if changed == nil {
		return nil
	}
	patched := config.MediaConversionConfig{}
	for key, value := range changed {
		switch key {
		case "timeout_seconds":
			patched.TimeoutSeconds = configYAMLIntValue(value)
		case "max_file_size_mb":
			patched.MaxFileSizeMB = configYAMLIntValue(value)
		case "ffmpeg_path":
			patched.FFmpegPath = configYAMLStringValue(value)
		case "imagemagick_path":
			patched.ImageMagickPath = configYAMLStringValue(value)
		}
	}
	return config.ValidateMediaConversionConfig(&patched)
}

func configYAMLIntValue(value interface{}) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0
		}
		return int(parsed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func configYAMLStringValue(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}
