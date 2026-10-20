package server

import (
	"fmt"
	"strings"

	"aurago/internal/config"
	"aurago/internal/tools"
)

type go2RTCMediaPatch struct {
	Tuya                *go2RTCTuyaCameraRequest `json:"tuya"`
	SubSource           *string                  `json:"sub_source"`
	SnapshotSource      *string                  `json:"snapshot_source"`
	ClearSubSource      bool                     `json:"clear_sub_source"`
	ClearSnapshotSource bool                     `json:"clear_snapshot_source"`
	PreviewSource       *string                  `json:"preview_source"`
	DetectionSource     *string                  `json:"detection_source"`
	SnapshotMode        *string                  `json:"snapshot_mode"`
	CodecProfile        *string                  `json:"codec_profile"`
	Transport           *string                  `json:"transport"`
	PlaybackMode        *string                  `json:"playback_mode"`
	Preload             *bool                    `json:"preload"`
	TalkEnabled         *bool                    `json:"talk_enabled"`
}

func (p go2RTCMediaPatch) apply(s *config.Go2RTCStreamConfig) {
	if p.Tuya != nil {
		main := p.Tuya.MainResolution
		if main == "" {
			main = "hd"
		}
		s.Tuya = &config.Go2RTCTuyaBinding{AccountID: p.Tuya.AccountID, MainResolution: main, SubResolution: p.Tuya.SubResolution}
	}
	for dst, src := range map[*string]*string{&s.PreviewSource: p.PreviewSource, &s.DetectionSource: p.DetectionSource, &s.SnapshotMode: p.SnapshotMode, &s.CodecProfile: p.CodecProfile, &s.Transport: p.Transport, &s.PlaybackMode: p.PlaybackMode} {
		if src != nil {
			*dst = *src
		}
	}
	if p.Preload != nil {
		s.Preload = *p.Preload
	}
	if p.TalkEnabled != nil {
		s.TalkEnabled = *p.TalkEnabled
	}
}

func (p go2RTCMediaPatch) changes(id string) ([]go2RTCSourceChange, error) {
	item := map[string]interface{}{"id": id, "clear_sub_source": p.ClearSubSource, "clear_snapshot_source": p.ClearSnapshotSource}
	if p.SubSource != nil {
		item["sub_source"] = *p.SubSource
	}
	if p.SnapshotSource != nil {
		item["snapshot_source"] = *p.SnapshotSource
	}
	return go2RTCSourceChanges([]interface{}{item}, false)
}

func (p go2RTCMediaPatch) present() bool {
	if p.Tuya != nil {
		return true
	}
	return p.SubSource != nil || p.SnapshotSource != nil || p.ClearSubSource || p.ClearSnapshotSource || p.PreviewSource != nil || p.DetectionSource != nil || p.SnapshotMode != nil || p.CodecProfile != nil || p.Transport != nil || p.PlaybackMode != nil || p.Preload != nil || p.TalkEnabled != nil
}

func go2RTCSourceChanges(items []interface{}, strip bool) ([]go2RTCSourceChange, error) {
	var changes []go2RTCSourceChange
	seen := map[string]bool{}
	for _, item := range items {
		stream, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("camera must be an object")
		}
		id, _ := stream["id"].(string)
		if tools.ValidateGo2RTCStreamID(id) != nil || seen[id] {
			return nil, fmt.Errorf("camera IDs must be valid and unique")
		}
		seen[id] = true
		for _, role := range []string{"main", "sub", "snapshot"} {
			field := "source"
			if role != "main" {
				field = role + "_source"
			}
			clear := false
			if raw, exists := stream["clear_"+field]; exists {
				var valid bool
				clear, valid = raw.(bool)
				if !valid {
					return nil, fmt.Errorf("clear_%s must be a boolean", field)
				}
			}
			if clear && role == "main" {
				return nil, fmt.Errorf("the main source cannot be cleared")
			}
			if raw, exists := stream[field]; exists {
				value, ok := raw.(string)
				if !ok {
					return nil, fmt.Errorf("%s must be a string", field)
				}
				value = strings.TrimSpace(value)
				if value != "" && value != "••••••••" {
					if clear {
						return nil, fmt.Errorf("replace and clear %s cannot be combined", field)
					}
					if err := tools.ValidateGo2RTCSource(value); err != nil {
						return nil, fmt.Errorf("invalid %s for camera %q", field, id)
					}
					changes = append(changes, go2RTCSourceChange{ID: id, Role: role, Value: value})
				}
			}
			if clear {
				changes = append(changes, go2RTCSourceChange{ID: id, Role: role, Delete: true})
			}
			if strip {
				delete(stream, field)
				delete(stream, field+"_configured")
				delete(stream, "clear_"+field)
			}
		}
	}
	return changes, nil
}

func go2RTCMediaProjection(stream config.Go2RTCStreamConfig) map[string]interface{} {
	stream.Detection = config.NormalizeGo2RTCStreamDetection(stream.Detection)
	return go2RTCDetectionProjection(stream)
}

func go2RTCSavedMediaFields(dst *config.Go2RTCStreamConfig, payload map[string]interface{}) error {
	// Only known non-secret media fields are merged; decode into the saved block
	// so omitted fields retain their values and explicit false remains false.
	for key, value := range payload {
		switch key {
		case "preview_source", "detection_source", "snapshot_mode", "codec_profile", "transport", "playback_mode":
			s, ok := value.(string)
			if !ok {
				return fmt.Errorf("%s must be a string", key)
			}
			switch key {
			case "preview_source":
				dst.PreviewSource = s
			case "detection_source":
				dst.DetectionSource = s
			case "snapshot_mode":
				dst.SnapshotMode = s
			case "codec_profile":
				dst.CodecProfile = s
			case "transport":
				dst.Transport = s
			case "playback_mode":
				dst.PlaybackMode = s
			}
		case "preload", "talk_enabled":
			b, ok := value.(bool)
			if !ok {
				return fmt.Errorf("%s must be a boolean", key)
			}
			if key == "preload" {
				dst.Preload = b
			} else {
				dst.TalkEnabled = b
			}
		}
	}
	return nil
}
