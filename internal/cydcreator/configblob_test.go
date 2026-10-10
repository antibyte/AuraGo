package cydcreator

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

func sampleDeviceConfig() DeviceConfig {
	return DeviceConfig{
		URL:        "https://aurago.lan:8443/base",
		DeviceID:   "ccd_0123456789ab",
		Token:      "cyd.tok_ABCdef-123_~+/=",
		CertSHA256: strings.Repeat("ab", 32),
	}
}

func TestDeviceConfigRoundTrip(t *testing.T) {
	cfg := sampleDeviceConfig()
	blob, err := EncodeDeviceConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(blob) != ConfigSize {
		t.Fatalf("blob is %d bytes, want %d", len(blob), ConfigSize)
	}
	if string(blob[0:4]) != "AGCC" || blob[4] != 1 || blob[5] != 0 {
		t.Fatalf("header = % x", blob[0:8])
	}
	n := int(binary.LittleEndian.Uint16(blob[6:8]))
	payload := string(blob[8 : 8+n])
	want := `{"url":"https://aurago.lan:8443/base","device_id":"ccd_0123456789ab",` +
		`"token":"cyd.tok_ABCdef-123_~+/=","cert_sha256":"` + strings.Repeat("ab", 32) + `"}`
	if payload != want {
		t.Fatalf("payload =\n%s\nwant\n%s", payload, want)
	}
	if !bytes.Equal(blob[8+n:], make([]byte, ConfigSize-8-n)) {
		t.Fatal("padding is not zero")
	}
	got, err := DecodeDeviceConfig(blob)
	if err != nil {
		t.Fatal(err)
	}
	if got != cfg {
		t.Fatalf("decoded %+v, want %+v", got, cfg)
	}
}

func TestDeviceConfigHTTPWithoutCert(t *testing.T) {
	cfg := DeviceConfig{URL: "http://192.168.1.10:8088", DeviceID: "ccd_1", Token: "t"}
	blob, err := EncodeDeviceConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := DecodeDeviceConfig(blob); err != nil || got != cfg {
		t.Fatalf("decode = %+v, %v", got, err)
	}
}

func TestDeviceConfigPayloadIsNotHTMLEscaped(t *testing.T) {
	cfg := DeviceConfig{URL: "http://aurago.lan:8088/a&b", DeviceID: "ccd_1", Token: "t"}
	blob, err := EncodeDeviceConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	n := int(binary.LittleEndian.Uint16(blob[6:8]))
	if payload := string(blob[8 : 8+n]); !strings.Contains(payload, `"http://aurago.lan:8088/a&b"`) {
		t.Fatalf("payload escaped the URL: %s", payload)
	}
	if got, err := DecodeDeviceConfig(blob); err != nil || got != cfg {
		t.Fatalf("decode = %+v, %v", got, err)
	}
}

func TestDeviceConfigPortBounds(t *testing.T) {
	for _, u := range []string{"http://x.example:1", "http://x.example:65535", "http://x.example:08080", "http://x.example"} {
		if _, err := EncodeDeviceConfig(DeviceConfig{URL: u, DeviceID: "ccd_1", Token: "t"}); err != nil {
			t.Errorf("%s rejected: %v", u, err)
		}
	}
}

func TestDeviceConfigMaxPayload(t *testing.T) {
	base := DeviceConfig{URL: "https://a.example", DeviceID: "ccd_1", Token: "x", CertSHA256: strings.Repeat("ab", 32)}
	blob, err := EncodeDeviceConfig(base)
	if err != nil {
		t.Fatal(err)
	}
	baseLen := int(binary.LittleEndian.Uint16(blob[6:8]))
	exact := base
	exact.Token = strings.Repeat("x", 1+ConfigMaxPayload-baseLen)
	blob, err = EncodeDeviceConfig(exact)
	if err != nil {
		t.Fatalf("payload of exactly %d bytes rejected: %v", ConfigMaxPayload, err)
	}
	if n := binary.LittleEndian.Uint16(blob[6:8]); n != ConfigMaxPayload {
		t.Fatalf("payload length %d, want %d", n, ConfigMaxPayload)
	}
	if _, err := DecodeDeviceConfig(blob); err != nil {
		t.Fatalf("decode max payload: %v", err)
	}
	over := exact
	over.Token += "x"
	if _, err := EncodeDeviceConfig(over); ErrorCode(err) != CodeBadRequest {
		t.Fatalf("payload of %d bytes: err = %v, want CYD_BAD_REQUEST", ConfigMaxPayload+1, err)
	}
}

func TestEncodeDeviceConfigRejects(t *testing.T) {
	cases := map[string]struct {
		mutate func(*DeviceConfig)
		want   string // substring of the error message, pins the rule that fired
	}{
		"empty url":        {func(c *DeviceConfig) { c.URL = "" }, "start with http"},
		"ftp url":          {func(c *DeviceConfig) { c.URL = "ftp://x.example" }, "start with http"},
		"relative url":     {func(c *DeviceConfig) { c.URL = "/api" }, "start with http"},
		"uppercase scheme": {func(c *DeviceConfig) { c.URL = "HTTPS://x.example" }, "start with http"},
		"mixed scheme":     {func(c *DeviceConfig) { c.URL = "Http://x.example" }, "start with http"},
		"no host":          {func(c *DeviceConfig) { c.URL = "https:///base" }, "with a host"},
		"url with quote":   {func(c *DeviceConfig) { c.URL = `https://x.example/"` }, "printable ASCII"},
		"url with backslash": {func(c *DeviceConfig) {
			c.URL = `https://x.example\base`
		}, "printable ASCII"},
		"url with space":   {func(c *DeviceConfig) { c.URL = "https://x.example/a b" }, "printable ASCII"},
		"url with control": {func(c *DeviceConfig) { c.URL = "https://x.example/\x01" }, "printable ASCII"},
		"url with newline": {func(c *DeviceConfig) { c.URL = "https://x.example/\n" }, "printable ASCII"},
		"url with DEL":     {func(c *DeviceConfig) { c.URL = "https://x.example/\x7f" }, "printable ASCII"},
		"url non-ascii":    {func(c *DeviceConfig) { c.URL = "https://x.example/ä" }, "printable ASCII"},
		"url with query":   {func(c *DeviceConfig) { c.URL = "https://x.example/?a=1" }, "credentials, a query"},
		"url empty query":  {func(c *DeviceConfig) { c.URL = "https://x.example/?" }, "credentials, a query"},
		"url fragment":     {func(c *DeviceConfig) { c.URL = "https://x.example/#f" }, "credentials, a query"},
		"url empty fragment": {func(c *DeviceConfig) {
			c.URL = "https://x.example/#"
		}, "credentials, a query"},
		"url userinfo":       {func(c *DeviceConfig) { c.URL = "https://u:p@x.example" }, "credentials, a query"},
		"url user only":      {func(c *DeviceConfig) { c.URL = "https://u@x.example" }, "credentials, a query"},
		"url empty userinfo": {func(c *DeviceConfig) { c.URL = "https://@x.example" }, "credentials, a query"},
		"ipv6 literal":       {func(c *DeviceConfig) { c.URL = "https://[::1]:8443" }, "IPv6"},
		"ipv6 without port":  {func(c *DeviceConfig) { c.URL = "https://[fe80::1]/base" }, "IPv6"},
		"port zero":          {func(c *DeviceConfig) { c.URL = "https://x.example:0" }, "port"},
		"port too big":       {func(c *DeviceConfig) { c.URL = "https://x.example:65536" }, "port"},
		"port huge":          {func(c *DeviceConfig) { c.URL = "https://x.example:99999999999999999999" }, "port"},
		"empty port":         {func(c *DeviceConfig) { c.URL = "https://x.example:" }, "port"},
		"empty port http": {func(c *DeviceConfig) {
			c.URL = "http://x.example:"
			c.CertSHA256 = ""
		}, "port"},
		"empty host with port": {func(c *DeviceConfig) { c.URL = "https://:8443/base" }, "host name"},
		"empty host http": {func(c *DeviceConfig) {
			c.URL = "http://:80"
			c.CertSHA256 = ""
		}, "host name"},
		"empty device":     {func(c *DeviceConfig) { c.DeviceID = "" }, "device_id"},
		"bad device":       {func(c *DeviceConfig) { c.DeviceID = "ccd 1" }, "device_id"},
		"empty token":      {func(c *DeviceConfig) { c.Token = "" }, "token"},
		"token with space": {func(c *DeviceConfig) { c.Token = "a b" }, "token"},
		"short cert":       {func(c *DeviceConfig) { c.CertSHA256 = "abcd" }, "64 lowercase hex"},
		"upper cert":       {func(c *DeviceConfig) { c.CertSHA256 = strings.Repeat("AB", 32) }, "64 lowercase hex"},
		"https without cert": {func(c *DeviceConfig) {
			c.CertSHA256 = ""
		}, "https URL needs cert_sha256"},
		"cert on http": {func(c *DeviceConfig) {
			c.URL = "http://x.example"
		}, "empty for an http:// URL"},
	}
	for name, tc := range cases {
		cfg := sampleDeviceConfig()
		tc.mutate(&cfg)
		_, err := EncodeDeviceConfig(cfg)
		if ErrorCode(err) != CodeBadRequest {
			t.Errorf("%s: err = %v, want CYD_BAD_REQUEST", name, err)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %q, want it to mention %q", name, err, tc.want)
		}
	}
}

// blobWithPayload returns a valid cydcc blob header around an arbitrary payload.
func blobWithPayload(t *testing.T, payload string) []byte {
	t.Helper()
	b := make([]byte, ConfigSize)
	copy(b[0:4], ConfigMagic)
	b[4] = ConfigVersion
	binary.LittleEndian.PutUint16(b[6:8], uint16(len(payload)))
	copy(b[8:], payload)
	return b
}

func TestDecodeDeviceConfigRejects(t *testing.T) {
	good, err := EncodeDeviceConfig(sampleDeviceConfig())
	if err != nil {
		t.Fatal(err)
	}
	n := int(binary.LittleEndian.Uint16(good[6:8]))
	cases := map[string]func([]byte) []byte{
		"short":          func(b []byte) []byte { return b[:100] },
		"bad magic":      func(b []byte) []byte { b[0] = 'X'; return b },
		"bad version":    func(b []byte) []byte { b[4] = 2; return b },
		"reserved byte":  func(b []byte) []byte { b[5] = 1; return b },
		"zero length":    func(b []byte) []byte { binary.LittleEndian.PutUint16(b[6:8], 0); return b },
		"length too big": func(b []byte) []byte { binary.LittleEndian.PutUint16(b[6:8], ConfigMaxPayload+1); return b },
		"garbage after":  func(b []byte) []byte { b[8+n+5] = 1; return b },
	}
	for name, mutate := range cases {
		blob := mutate(append([]byte(nil), good...))
		if _, err := DecodeDeviceConfig(blob); ErrorCode(err) != CodeBadRequest {
			t.Errorf("%s: err = %v, want CYD_BAD_REQUEST", name, err)
		}
	}
}

func TestDecodeDeviceConfigHeaderMessages(t *testing.T) {
	good, err := EncodeDeviceConfig(sampleDeviceConfig())
	if err != nil {
		t.Fatal(err)
	}
	reserved := append([]byte(nil), good...)
	reserved[5] = 1
	_, err = DecodeDeviceConfig(reserved)
	if ErrorCode(err) != CodeBadRequest || !strings.Contains(err.Error(), "reserved byte") ||
		strings.Contains(err.Error(), "version") {
		t.Errorf("reserved byte err = %v, want its own message", err)
	}
	version := append([]byte(nil), good...)
	version[4] = 2
	_, err = DecodeDeviceConfig(version)
	if ErrorCode(err) != CodeBadRequest || !strings.Contains(err.Error(), "unsupported version 2") {
		t.Errorf("version err = %v, want unsupported version 2", err)
	}
}

func TestDecodeDeviceConfigPayloadRules(t *testing.T) {
	const httpBody = `{"url":"http://x.example","device_id":"d1","token":"t","cert_sha256":""}`
	if _, err := DecodeDeviceConfig(blobWithPayload(t, httpBody)); err != nil {
		t.Fatalf("baseline payload rejected: %v", err)
	}
	pin := strings.Repeat("ab", 32)
	cases := map[string]struct{ body, want string }{
		"unknown field": {
			`{"url":"http://x.example","device_id":"d1","token":"t","cert_sha256":"","x":1}`, "unknown field"},
		"case-variant key": {
			`{"URL":"http://x.example","device_id":"d1","token":"t","cert_sha256":""}`, "unknown field"},
		"case-variant extra key": {
			`{"url":"http://x.example","device_id":"d1","token":"t","cert_sha256":"","Token":"u"}`, "unknown field"},
		"duplicate key": {
			`{"url":"http://x.example","url":"http://y.example","device_id":"d1","token":"t","cert_sha256":""}`, "repeats"},
		"missing key": {
			`{"url":"http://x.example","device_id":"d1","token":"t"}`, "missing field"},
		"null value": {
			`{"url":"http://x.example","device_id":"d1","token":"t","cert_sha256":null}`, "must be a string"},
		"number value": {
			`{"url":"http://x.example","device_id":"d1","token":1,"cert_sha256":""}`, "must be a string"},
		"trailing object":  {httpBody + `{}`, "trailing data"},
		"trailing garbage": {httpBody + `x`, "trailing data"},
		"trailing value":   {httpBody + ` 1`, "trailing data"},
		"not an object":    {`[1]`, "JSON object"},
		"truncated":        {`{"url":"http://x.example"`, "payload:"},
		"url with query": {
			`{"url":"https://x.example/?a=1","device_id":"d1","token":"t","cert_sha256":"` + pin + `"}`, "query"},
		"https without pin": {
			`{"url":"https://x.example","device_id":"d1","token":"t","cert_sha256":""}`, "needs cert_sha256"},
		"ipv6 host": {
			`{"url":"http://[::1]:80","device_id":"d1","token":"t","cert_sha256":""}`, "IPv6"},
		"port zero": {
			`{"url":"http://x.example:0","device_id":"d1","token":"t","cert_sha256":""}`, "port"},
		"empty host": {
			`{"url":"http://:80","device_id":"d1","token":"t","cert_sha256":""}`, "host name"},
		"uppercase scheme": {
			`{"url":"HTTP://x.example","device_id":"d1","token":"t","cert_sha256":""}`, "start with http"},
	}
	for name, tc := range cases {
		_, err := DecodeDeviceConfig(blobWithPayload(t, tc.body))
		if ErrorCode(err) != CodeBadRequest {
			t.Errorf("%s: err = %v, want CYD_BAD_REQUEST", name, err)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %q, want it to mention %q", name, err, tc.want)
		}
	}
}

func TestDeviceConfigRedactsToken(t *testing.T) {
	cfg := sampleDeviceConfig()
	for _, out := range []string{
		fmt.Sprintf("%v", cfg),
		fmt.Sprintf("%+v", cfg),
		fmt.Sprintf("%s", cfg),
		fmt.Sprintf("%#v", cfg),
		fmt.Sprintf("%v", &cfg),
		fmt.Sprintf("%+v", &cfg),
		cfg.String(),
	} {
		if strings.Contains(out, cfg.Token) {
			t.Errorf("formatted config leaks the token: %s", out)
		}
		if !strings.Contains(out, "***") || !strings.Contains(out, cfg.DeviceID) {
			t.Errorf("formatted config = %s, want redacted token and the device id", out)
		}
	}

	for name, newHandler := range map[string]func(*bytes.Buffer) slog.Handler{
		"text": func(b *bytes.Buffer) slog.Handler { return slog.NewTextHandler(b, nil) },
		"json": func(b *bytes.Buffer) slog.Handler { return slog.NewJSONHandler(b, nil) },
	} {
		var buf bytes.Buffer
		slog.New(newHandler(&buf)).Info("device paired", "config", cfg)
		out := buf.String()
		if strings.Contains(out, cfg.Token) {
			t.Errorf("%s slog output leaks the token: %s", name, out)
		}
		if !strings.Contains(out, "***") || !strings.Contains(out, cfg.DeviceID) {
			t.Errorf("%s slog output = %s, want redacted token and the device id", name, out)
		}
	}
}

func TestDeviceConfigRedactionKeepsBlobToken(t *testing.T) {
	cfg := sampleDeviceConfig()
	blob, err := EncodeDeviceConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(blob, []byte(cfg.Token)) {
		t.Fatal("the cydcc blob must still carry the real token")
	}
}
