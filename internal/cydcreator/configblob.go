package cydcreator

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Config partition (cydcc) layout, shared byte-for-byte with the firmware:
//
//	bytes 0-3  magic "AGCC"
//	byte  4    version (1)
//	byte  5    reserved, 0
//	bytes 6-7  little-endian uint16 payload length (1..ConfigMaxPayload)
//	bytes 8-   JSON payload {"url","device_id","token","cert_sha256"}, then zero padding
const (
	ConfigMagic      = "AGCC"
	ConfigVersion    = 1
	ConfigSize       = 4096
	ConfigMaxPayload = 4088
)

// DeviceConfig is the payload the template's AuraGo link reads at boot.
//
// Token is a bearer secret; never log it. String, GoString and LogValue redact
// it, so printing the struct with fmt or slog is safe; JSON encoding (the blob
// itself) still carries the token on purpose.
type DeviceConfig struct {
	URL        string `json:"url"`
	DeviceID   string `json:"device_id"`
	Token      string `json:"token"`
	CertSHA256 string `json:"cert_sha256"` // lowercase hex of the DER leaf cert; "" for http://
}

const redactedToken = "***"

// String implements fmt.Stringer with the token redacted.
func (c DeviceConfig) String() string {
	return fmt.Sprintf("DeviceConfig{URL:%s DeviceID:%s Token:%s CertSHA256:%s}",
		c.URL, c.DeviceID, redactedToken, c.CertSHA256)
}

// GoString implements fmt.GoStringer so %#v does not leak the token either.
func (c DeviceConfig) GoString() string { return c.String() }

// LogValue implements slog.LogValuer with the token redacted.
func (c DeviceConfig) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("url", c.URL),
		slog.String("device_id", c.DeviceID),
		slog.String("token", redactedToken),
		slog.String("cert_sha256", c.CertSHA256),
	)
}

var (
	configTokenPattern = regexp.MustCompile(`^[A-Za-z0-9._~+/=-]+$`)
	configCertPattern  = regexp.MustCompile(`^[0-9a-f]{64}$`)
	deviceHostPattern  = regexp.MustCompile(`^[A-Za-z0-9.-]+$`)
)

// validate is the single gate for the encoder and the decoder. The URL and
// pin checks are deviceURLProblem and deviceCertProblem, which
// ValidateDeviceURL shares, so a device URL accepted anywhere always encodes.
func (c DeviceConfig) validate() error {
	if problem := deviceURLProblem(c.URL); problem != "" {
		return configError(problem)
	}
	if !validElementID(c.DeviceID) {
		return configError("device_id is invalid")
	}
	if !configTokenPattern.MatchString(c.Token) {
		return configError("token is empty or contains characters outside [A-Za-z0-9._~+/=-]")
	}
	if problem := deviceCertProblem(c.URL, c.CertSHA256); problem != "" {
		return configError(problem)
	}
	return nil
}

// deviceURLProblem returns why rawURL cannot be a device's base URL, or "".
// The firmware builds request URLs by string joining, checks the literal
// prefix "https://", splits host and port with lastIndexOf(':') and casts the
// port to uint16, so a URL is only accepted when it is plain ASCII, has no
// credentials, query or fragment, no IPv6 literal, a host of letters, digits,
// dots and hyphens and a port in 1..65535.
func deviceURLProblem(rawURL string) string {
	if !strings.HasPrefix(rawURL, "https://") && !strings.HasPrefix(rawURL, "http://") {
		return "url must start with http:// or https://"
	}
	for i := 0; i < len(rawURL); i++ {
		if b := rawURL[i]; b < 0x21 || b > 0x7E || b == '"' || b == '\\' {
			return "url must be printable ASCII without quotes or backslashes"
		}
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return "url must be an absolute http(s) URL with a host"
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" ||
		u.ForceQuery || strings.ContainsAny(rawURL, "?#") {
		return "url must not contain credentials, a query or a fragment"
	}
	if strings.HasPrefix(u.Host, "[") {
		return "url must not use an IPv6 literal host"
	}
	if u.Hostname() == "" { // "http://:80": url.Parse accepts it, the device cannot connect to it
		return "url must have a host name"
	}
	// The firmware connects to the raw text between "//" and the last ':' (host
	// names and IPv4 only): a %-escape, a sub-delimiter or a second colon that
	// url.Parse tolerates would reach the device's DNS lookup as is.
	if !deviceHostPattern.MatchString(u.Hostname()) {
		return "url host must be a host name or an IPv4 address (letters, digits, dots and hyphens)"
	}
	if strings.HasSuffix(u.Host, ":") {
		return "url port must be 1..65535"
	}
	if port := u.Port(); port != "" {
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return "url port must be 1..65535"
		}
	}
	return ""
}

// deviceCertProblem returns why certSHA256 cannot be the pin of a device
// using rawURL, or "": https needs the lowercase hex SHA-256 of the DER leaf
// certificate, http must not carry one.
func deviceCertProblem(rawURL, certSHA256 string) string {
	https := strings.HasPrefix(rawURL, "https://")
	if certSHA256 != "" && !configCertPattern.MatchString(certSHA256) {
		return "cert_sha256 must be empty or 64 lowercase hex characters"
	}
	if https && certSHA256 == "" {
		return "an https URL needs cert_sha256"
	}
	if !https && certSHA256 != "" {
		return "cert_sha256 must be empty for an http:// URL"
	}
	return ""
}

// deviceURLHint names the setting that fixes a device URL AuraGo derived but a
// device cannot use (for example an IPv6 address or a wildcard bind).
const deviceURLHint = "cyd_creator.ota.public_url"

// ValidateDeviceURL checks the base URL a device is flashed with and its
// certificate pin by exactly the rule the cydcc blob enforces
// (DeviceConfig.validate shares both checks), so minting never fails later
// inside EncodeDeviceConfig. A violation is CYD_BAD_REQUEST with
// details.reason "device_url" or "cert_sha256" and details.hint
// "cyd_creator.ota.public_url".
func ValidateDeviceURL(rawURL, certSHA256 string) error {
	if problem := deviceURLProblem(rawURL); problem != "" {
		return deviceURLError("device_url", problem)
	}
	if problem := deviceCertProblem(rawURL, certSHA256); problem != "" {
		return deviceURLError("cert_sha256", problem)
	}
	return nil
}

func deviceURLError(reason, problem string) *Error {
	return NewError(CodeBadRequest, "device URL: "+problem, map[string]any{"reason": reason, "hint": deviceURLHint})
}

func configError(reason string) *Error {
	return NewError(CodeBadRequest, "device config: "+reason, nil)
}

// EncodeDeviceConfig returns exactly ConfigSize bytes for the cydcc partition.
func EncodeDeviceConfig(c DeviceConfig) ([]byte, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	var payload bytes.Buffer
	enc := json.NewEncoder(&payload)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(c); err != nil {
		return nil, fmt.Errorf("encode device config: %w", err)
	}
	body := bytes.TrimSuffix(payload.Bytes(), []byte("\n"))
	if len(body) > ConfigMaxPayload {
		return nil, NewError(CodeBadRequest,
			fmt.Sprintf("device config: payload is %d bytes, maximum %d", len(body), ConfigMaxPayload),
			map[string]any{"limit": ConfigMaxPayload})
	}
	out := make([]byte, ConfigSize)
	copy(out[0:4], ConfigMagic)
	out[4] = ConfigVersion
	binary.LittleEndian.PutUint16(out[6:8], uint16(len(body)))
	copy(out[8:], body)
	return out, nil
}

// DecodeDeviceConfig parses a cydcc blob. It is stricter than encoding/json on
// purpose: the payload must be one JSON object with exactly the four keys
// url, device_id, token and cert_sha256 (case-sensitive, each once, each a
// string), nothing after the object, and the fields must pass the same
// validation as EncodeDeviceConfig.
func DecodeDeviceConfig(b []byte) (DeviceConfig, error) {
	if len(b) != ConfigSize {
		return DeviceConfig{}, configError(fmt.Sprintf("blob is %d bytes, want %d", len(b), ConfigSize))
	}
	if string(b[0:4]) != ConfigMagic {
		return DeviceConfig{}, configError("bad magic")
	}
	if b[4] != ConfigVersion {
		return DeviceConfig{}, configError(fmt.Sprintf("unsupported version %d", b[4]))
	}
	if b[5] != 0 {
		return DeviceConfig{}, configError(fmt.Sprintf("reserved byte is %d, want 0", b[5]))
	}
	n := int(binary.LittleEndian.Uint16(b[6:8]))
	if n < 1 || n > ConfigMaxPayload {
		return DeviceConfig{}, configError(fmt.Sprintf("payload length %d out of range", n))
	}
	for _, rest := range b[8+n:] {
		if rest != 0 {
			return DeviceConfig{}, configError("non-zero bytes after the payload")
		}
	}
	c, err := decodeConfigPayload(b[8 : 8+n])
	if err != nil {
		return DeviceConfig{}, err
	}
	if err := c.validate(); err != nil {
		return DeviceConfig{}, err
	}
	return c, nil
}

// decodeConfigPayload reads the JSON object token by token so unknown keys,
// case variants and duplicate keys are rejected instead of silently merged.
func decodeConfigPayload(payload []byte) (DeviceConfig, error) {
	dec := json.NewDecoder(bytes.NewReader(payload))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return DeviceConfig{}, configError("payload must be a JSON object")
	}
	var c DeviceConfig
	seen := map[string]bool{}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return DeviceConfig{}, configError("payload: " + err.Error())
		}
		key, _ := keyTok.(string)
		var field *string
		switch key {
		case "url":
			field = &c.URL
		case "device_id":
			field = &c.DeviceID
		case "token":
			field = &c.Token
		case "cert_sha256":
			field = &c.CertSHA256
		default:
			return DeviceConfig{}, configError(fmt.Sprintf("payload has unknown field %q", key))
		}
		if seen[key] {
			return DeviceConfig{}, configError(fmt.Sprintf("payload repeats field %q", key))
		}
		seen[key] = true
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return DeviceConfig{}, configError("payload: " + err.Error())
		}
		if len(raw) == 0 || raw[0] != '"' || json.Unmarshal(raw, field) != nil {
			return DeviceConfig{}, configError(fmt.Sprintf("payload field %q must be a string", key))
		}
	}
	if _, err := dec.Token(); err != nil { // the closing brace
		return DeviceConfig{}, configError("payload: " + err.Error())
	}
	if _, err := dec.Token(); err != io.EOF {
		return DeviceConfig{}, configError("payload has trailing data")
	}
	for _, key := range []string{"url", "device_id", "token", "cert_sha256"} {
		if !seen[key] {
			return DeviceConfig{}, configError(fmt.Sprintf("payload is missing field %q", key))
		}
	}
	return c, nil
}
