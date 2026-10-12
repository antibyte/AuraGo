package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"aurago/internal/config"
)

// go2RTCDetectionProjection renders a detection config as the generic map the
// config UI receives; JSON keys equal the YAML keys and zone points are [x, y] arrays,
// so the map can be written back to the YAML file unchanged.
func go2RTCDetectionProjection(value interface{}) map[string]interface{} {
	// The {} fallbacks are unreachable for loaded settings: json.Marshal fails only on
	// NaN/Inf, which config.Load rejects. If one were reached, the camera's block would
	// be projected empty and the next config page save would reset it to the defaults.
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

func go2RTCDetectionDecodeError(field, message string) error {
	return &config.Go2RTCDetectionFieldError{Field: field, Message: message}
}

// decodeGo2RTCStreamDetection decodes a per-camera detection object that replaces the
// camera's block. Like the YAML decoder (contract C5) it pre-fills the default set, so
// a missing key gets its default and an explicit value is kept, also in nested actions.
// Decoding is strict, and every error names its field ("detection", "detection.…"):
// null is rejected everywhere (it never means "default" or 0; a NaN from a zone editor
// arrives as null), as are unknown fields, wrong types and trailing data, and each zone
// point must be exactly [x, y] (encoding/json would silently pad or cut a [2]float64).
func decodeGo2RTCStreamDetection(raw json.RawMessage) (config.Go2RTCStreamDetectionConfig, error) {
	var generic interface{}
	pre := json.NewDecoder(bytes.NewReader(raw))
	pre.UseNumber()
	if err := pre.Decode(&generic); err != nil || !go2RTCDetectionJSONEnded(pre) {
		return config.Go2RTCStreamDetectionConfig{}, go2RTCDetectionDecodeError("detection", "must be one JSON object")
	}
	if err := checkGo2RTCDetectionJSON(generic, reflect.TypeOf(config.Go2RTCStreamDetectionConfig{}), "detection"); err != nil {
		return config.Go2RTCStreamDetectionConfig{}, err
	}
	// The default set's zone slice is empty and new, so decoding cannot merge into or
	// alias zones of another configuration.
	out := config.DefaultGo2RTCStreamDetection(false)
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&out); err != nil {
		return config.Go2RTCStreamDetectionConfig{}, go2RTCDetectionJSONError(err)
	}
	if !go2RTCDetectionJSONEnded(decoder) {
		return config.Go2RTCStreamDetectionConfig{}, go2RTCDetectionDecodeError("detection", "must be one JSON object")
	}
	return out, nil
}

// go2RTCDetectionJSONEnded reports whether nothing but whitespace follows the value
// the decoder read.
func go2RTCDetectionJSONEnded(decoder *json.Decoder) bool {
	var trailing json.RawMessage
	return errors.Is(decoder.Decode(&trailing), io.EOF)
}

// checkGo2RTCDetectionJSON walks a generically decoded value (json.Number for numbers)
// against the Go type it will be decoded into. It rejects null, unknown or differently
// cased keys, objects and lists of the wrong shape and points that are not [x, y],
// with the field path. Scalar types are left to the typed decode.
//
// Limit: a duplicate object key reaches the walker only with its last value, while the
// typed decode merges every occurrence ({"actions":{"desktop":null},"actions":{}} is
// accepted as the defaults). JSON.stringify never writes duplicate keys, so this is
// accepted instead of walking tokens.
func checkGo2RTCDetectionJSON(value interface{}, typ reflect.Type, path string) error {
	if typ.Kind() == reflect.Array {
		// The only arrays are zone points ([2]float64).
		list, ok := value.([]interface{})
		if !ok || len(list) != typ.Len() {
			return go2RTCDetectionDecodeError(path, "must be [x, y]")
		}
		for _, item := range list {
			if _, ok := item.(json.Number); !ok {
				return go2RTCDetectionDecodeError(path, "must be [x, y]")
			}
		}
		return nil
	}
	if value == nil {
		return go2RTCDetectionDecodeError(path, "must not be null")
	}
	switch typ.Kind() {
	case reflect.Struct:
		object, ok := value.(map[string]interface{})
		if !ok {
			return go2RTCDetectionDecodeError(path, "must be an object")
		}
		fields := go2RTCDetectionJSONFields(typ)
		keys := make([]string, 0, len(object))
		for key := range object {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fieldType, known := fields[key]
			if !known {
				return go2RTCDetectionUnknownKey(path, key)
			}
			if err := checkGo2RTCDetectionJSON(object[key], fieldType, path+"."+key); err != nil {
				return err
			}
		}
	case reflect.Slice:
		list, ok := value.([]interface{})
		if !ok {
			return go2RTCDetectionDecodeError(path, "must be a list")
		}
		for i, item := range list {
			if err := checkGo2RTCDetectionJSON(item, typ.Elem(), fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

// go2RTCDetectionUnknownKey reports key of the object at path as unknown; an empty key
// is named in the message so the path does not end in an empty segment.
func go2RTCDetectionUnknownKey(path, key string) error {
	if key == "" {
		return go2RTCDetectionDecodeError(path, `"" is not a known setting`)
	}
	return go2RTCDetectionDecodeError(path+"."+key, "is not a known setting")
}

// go2RTCDetectionJSONFields maps the JSON names of typ's exported fields to their types.
func go2RTCDetectionJSONFields(typ reflect.Type) map[string]reflect.Type {
	fields := make(map[string]reflect.Type, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		fields[name] = field.Type
	}
	return fields
}

var go2RTCDetectionPointField = regexp.MustCompile(`^detection\.zones\[\d+\]\.points\[\d+\]`)

// go2RTCDetectionJSONError turns an error of the typed detection decode into a field
// error. encoding/json names the field of a type error as a dotted path with array
// indices as segments ("zones.0.points.1.1"); an unknown field error names only the key.
func go2RTCDetectionJSONError(err error) error {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) && typeErr.Field != "" {
		field := "detection"
		for _, segment := range strings.Split(typeErr.Field, ".") {
			if segment != "" && strings.Trim(segment, "0123456789") == "" {
				field += "[" + segment + "]"
			} else {
				field += "." + segment
			}
		}
		if point := go2RTCDetectionPointField.FindString(field); point != "" {
			return go2RTCDetectionDecodeError(point, "must be [x, y]")
		}
		return go2RTCDetectionDecodeError(field, go2RTCDetectionTypeMessage(typeErr.Type))
	}
	if err != nil {
		if key, found := strings.CutPrefix(err.Error(), `json: unknown field "`); found {
			return go2RTCDetectionUnknownKey("detection", strings.TrimSuffix(key, `"`))
		}
	}
	return go2RTCDetectionDecodeError("detection", "must be a detection object")
}

func go2RTCDetectionTypeMessage(typ reflect.Type) string {
	if typ == nil {
		return "has the wrong type"
	}
	switch typ.Kind() {
	case reflect.Bool:
		return "must be true or false"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "must be a whole number"
	case reflect.Float32, reflect.Float64:
		return "must be a number"
	case reflect.String:
		return "must be text"
	case reflect.Struct, reflect.Map:
		return "must be an object"
	case reflect.Slice, reflect.Array:
		return "must be a list"
	}
	return "has the wrong type"
}
