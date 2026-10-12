package server

import (
	"bytes"
	"encoding/json"
	"fmt"

	"aurago/internal/config"
)

// go2RTCDetectionProjection renders a detection config as the generic map the
// config UI receives; JSON keys equal the YAML keys and zone points are [x, y] arrays,
// so the map can be written back to the YAML file unchanged.
func go2RTCDetectionProjection(value interface{}) map[string]interface{} {
	encoded, err := json.Marshal(value)
	if err != nil {
		return map[string]interface{}{}
	}
	var out map[string]interface{}
	if err := json.Unmarshal(encoded, &out); err != nil || out == nil {
		return map[string]interface{}{}
	}
	return out
}

// validateGo2RTCDetectionSettings normalizes a copy exactly like config.Load and
// validates it, so a file config.Load would reject is never written. cfg must be the
// direct decode of the YAML (or built in code), never a YAML/JSON round-trip clone:
// a zone point that was not [x, y] is recorded only on the directly decoded zone.
func validateGo2RTCDetectionSettings(cfg config.Go2RTCConfig) error {
	// cfg is a copy; NormalizeGo2RTCDetection gives it new Streams and Schedule
	// slices, so the caller's slices are not modified.
	config.NormalizeGo2RTCDetection(&cfg)
	return config.ValidateGo2RTCDetection(cfg)
}

// decodeGo2RTCStreamDetection decodes a per-camera detection object that replaces the
// camera's block. Like the YAML decoder (contract C5) it pre-fills the default set, so
// a missing key gets its default and an explicit value is kept, also in nested actions.
// Decoding is strict: unknown fields, wrong types and trailing data are rejected, and
// each zone point must be exactly [x, y] (encoding/json would silently pad or cut a
// [2]float64). Field paths in errors start with "detection".
func decodeGo2RTCStreamDetection(raw json.RawMessage) (config.Go2RTCStreamDetectionConfig, error) {
	var shape struct {
		Zones []struct {
			Points [][]float64 `json:"points"`
		} `json:"zones"`
	}
	if err := json.Unmarshal(raw, &shape); err != nil {
		return config.Go2RTCStreamDetectionConfig{}, &config.Go2RTCDetectionFieldError{Field: "detection", Message: "must be a detection object"}
	}
	for i, zone := range shape.Zones {
		for j, point := range zone.Points {
			if len(point) != 2 {
				return config.Go2RTCStreamDetectionConfig{}, &config.Go2RTCDetectionFieldError{
					Field: fmt.Sprintf("detection.zones[%d].points[%d]", i, j), Message: "must be [x, y]",
				}
			}
		}
	}
	// The default set's zone slice is empty and new, so decoding cannot merge into or
	// alias zones of another configuration.
	out := config.DefaultGo2RTCStreamDetection(false)
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&out); err != nil {
		return config.Go2RTCStreamDetectionConfig{}, &config.Go2RTCDetectionFieldError{Field: "detection", Message: "has an unknown field or a value of the wrong type"}
	}
	return out, nil
}
