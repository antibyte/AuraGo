package desktop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

const SerialProfilesSetting = "quick_connect.serial_profiles"

const defaultSerialProfiles = `{"version":1,"profiles":[]}`

type serialProfileDocument struct {
	Version  int             `json:"version"`
	Profiles []serialProfile `json:"profiles"`
}

type serialProfile struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Source       string          `json:"source"`
	Port         string          `json:"port"`
	USBVendorID  *uint16         `json:"usb_vendor_id,omitempty"`
	USBProductID *uint16         `json:"usb_product_id,omitempty"`
	Options      json.RawMessage `json:"options"`
}

type serialOptions struct {
	BaudRate    int
	DataBits    int
	StopBits    int
	Parity      string
	FlowControl string
	LocalEcho   bool
	LineEnding  string
	DTR         bool
	RTS         bool
}

func defaultSerialOptions() serialOptions {
	return serialOptions{BaudRate: 115200, DataBits: 8, StopBits: 1, Parity: "none", FlowControl: "none", LineEnding: "cr"}
}

func validateSerialProfiles(value string) error {
	if len(value) > 64<<10 || !utf8.ValidString(value) {
		return fmt.Errorf("invalid desktop setting value for %s", SerialProfilesSetting)
	}
	dec := json.NewDecoder(strings.NewReader(value))
	dec.DisallowUnknownFields()
	var doc serialProfileDocument
	if err := dec.Decode(&doc); err != nil {
		return fmt.Errorf("invalid desktop setting value for %s", SerialProfilesSetting)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF || doc.Version != 1 || doc.Profiles == nil || len(doc.Profiles) > 64 {
		return fmt.Errorf("invalid desktop setting value for %s", SerialProfilesSetting)
	}
	seen := make(map[string]struct{}, len(doc.Profiles))
	for _, profile := range doc.Profiles {
		if !validSerialProfileID(profile.ID) || !validSerialProfileName(profile.Name) || len(profile.Port) > 256 || !validSerialProfileText(profile.Port) {
			return fmt.Errorf("invalid desktop setting value for %s", SerialProfilesSetting)
		}
		if _, duplicate := seen[profile.ID]; duplicate {
			return fmt.Errorf("invalid desktop setting value for %s", SerialProfilesSetting)
		}
		seen[profile.ID] = struct{}{}
		if profile.Source != "browser" && profile.Source != "host" {
			return fmt.Errorf("invalid desktop setting value for %s", SerialProfilesSetting)
		}
		if profile.Source == "browser" {
			if profile.Port != "" {
				return fmt.Errorf("invalid desktop setting value for %s", SerialProfilesSetting)
			}
		} else if profile.Port == "" || strings.TrimSpace(profile.Port) != profile.Port || profile.USBVendorID != nil || profile.USBProductID != nil {
			return fmt.Errorf("invalid desktop setting value for %s", SerialProfilesSetting)
		}
		if profile.USBProductID != nil && profile.USBVendorID == nil {
			return fmt.Errorf("invalid desktop setting value for %s", SerialProfilesSetting)
		}
		if _, err := parseSerialOptions(profile.Options, profile.Source); err != nil {
			return fmt.Errorf("invalid desktop setting value for %s", SerialProfilesSetting)
		}
	}
	return nil
}

func validSerialProfileID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, r := range id {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

func validSerialProfileName(name string) bool {
	if strings.TrimSpace(name) == "" || utf8.RuneCountInString(name) > 80 {
		return false
	}
	return validSerialProfileText(name)
}

func validSerialProfileText(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func parseSerialOptions(raw json.RawMessage, source string) (serialOptions, error) {
	options := defaultSerialOptions()
	if len(raw) == 0 || !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) {
		return options, fmt.Errorf("options must be an object")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return options, fmt.Errorf("invalid options")
	}
	for key := range fields {
		switch key {
		case "baud_rate", "data_bits", "stop_bits", "parity", "flow_control", "local_echo", "line_ending", "dtr", "rts":
		default:
			return options, fmt.Errorf("unknown serial option")
		}
	}
	if err := decodeSerialOption(fields, "baud_rate", &options.BaudRate); err != nil {
		return options, err
	}
	if err := decodeSerialOption(fields, "data_bits", &options.DataBits); err != nil {
		return options, err
	}
	if err := decodeSerialOption(fields, "stop_bits", &options.StopBits); err != nil {
		return options, err
	}
	if err := decodeSerialOption(fields, "parity", &options.Parity); err != nil {
		return options, err
	}
	if err := decodeSerialOption(fields, "flow_control", &options.FlowControl); err != nil {
		return options, err
	}
	if err := decodeSerialOption(fields, "local_echo", &options.LocalEcho); err != nil {
		return options, err
	}
	if err := decodeSerialOption(fields, "line_ending", &options.LineEnding); err != nil {
		return options, err
	}
	if err := decodeSerialOption(fields, "dtr", &options.DTR); err != nil {
		return options, err
	}
	if err := decodeSerialOption(fields, "rts", &options.RTS); err != nil {
		return options, err
	}
	if options.BaudRate < 1 || options.BaudRate > 4_000_000 || (options.DataBits != 7 && options.DataBits != 8) || (options.StopBits != 1 && options.StopBits != 2) {
		return options, fmt.Errorf("serial numeric option out of range")
	}
	switch options.Parity {
	case "none", "even", "odd":
	default:
		return options, fmt.Errorf("invalid parity")
	}
	switch options.FlowControl {
	case "none":
	case "hardware":
		if source != "browser" {
			return options, fmt.Errorf("host serial does not support hardware flow control")
		}
	default:
		return options, fmt.Errorf("invalid flow control")
	}
	switch options.LineEnding {
	case "cr", "lf", "crlf", "none":
	default:
		return options, fmt.Errorf("invalid line ending")
	}
	return options, nil
}

func decodeSerialOption[T any](fields map[string]json.RawMessage, key string, target *T) error {
	if raw, ok := fields[key]; ok {
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return fmt.Errorf("invalid serial option %s", key)
		}
		if err := json.Unmarshal(raw, target); err != nil {
			return fmt.Errorf("invalid serial option %s", key)
		}
	}
	return nil
}
