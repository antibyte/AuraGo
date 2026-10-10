package cydcreator

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"regexp"
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
type DeviceConfig struct {
	URL        string `json:"url"`
	DeviceID   string `json:"device_id"`
	Token      string `json:"token"`
	CertSHA256 string `json:"cert_sha256"` // lowercase hex of the DER leaf cert; "" for http://
}

var (
	configTokenPattern = regexp.MustCompile(`^[A-Za-z0-9._~+/=-]+$`)
	configCertPattern  = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

func (c DeviceConfig) validate() error {
	u, err := url.Parse(c.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		strings.ContainsAny(c.URL, " \t\r\n\"\\") {
		return configError("url must be an absolute http(s) URL")
	}
	if !validElementID(c.DeviceID) {
		return configError("device_id is invalid")
	}
	if !configTokenPattern.MatchString(c.Token) {
		return configError("token is empty or contains characters outside [A-Za-z0-9._~+/=-]")
	}
	if c.CertSHA256 != "" && !configCertPattern.MatchString(c.CertSHA256) {
		return configError("cert_sha256 must be empty or 64 lowercase hex characters")
	}
	if u.Scheme == "http" && c.CertSHA256 != "" {
		return configError("cert_sha256 must be empty for an http:// URL")
	}
	return nil
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

// DecodeDeviceConfig parses a cydcc blob with the same rules the firmware applies.
func DecodeDeviceConfig(b []byte) (DeviceConfig, error) {
	if len(b) != ConfigSize {
		return DeviceConfig{}, configError(fmt.Sprintf("blob is %d bytes, want %d", len(b), ConfigSize))
	}
	if string(b[0:4]) != ConfigMagic {
		return DeviceConfig{}, configError("bad magic")
	}
	if b[4] != ConfigVersion || b[5] != 0 {
		return DeviceConfig{}, configError(fmt.Sprintf("unsupported version %d", b[4]))
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
	dec := json.NewDecoder(bytes.NewReader(b[8 : 8+n]))
	dec.DisallowUnknownFields()
	var c DeviceConfig
	if err := dec.Decode(&c); err != nil {
		return DeviceConfig{}, configError("payload: " + err.Error())
	}
	if _, err := dec.Token(); err != io.EOF {
		return DeviceConfig{}, configError("payload has trailing data")
	}
	if err := c.validate(); err != nil {
		return DeviceConfig{}, err
	}
	return c, nil
}
