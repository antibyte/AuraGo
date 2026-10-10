package cydcreator

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func sampleDeviceConfig() DeviceConfig {
	return DeviceConfig{
		URL:        "https://aurago.lan:8443/base?x=1&y=2",
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
	want := `{"url":"https://aurago.lan:8443/base?x=1&y=2","device_id":"ccd_0123456789ab",` +
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

func TestDeviceConfigMaxPayload(t *testing.T) {
	base := DeviceConfig{URL: "https://a.example", DeviceID: "ccd_1", Token: "x"}
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
	cases := map[string]func(*DeviceConfig){
		"empty url":        func(c *DeviceConfig) { c.URL = "" },
		"ftp url":          func(c *DeviceConfig) { c.URL = "ftp://x.example" },
		"relative url":     func(c *DeviceConfig) { c.URL = "/api" },
		"url with quote":   func(c *DeviceConfig) { c.URL = `https://x.example/"` },
		"empty device":     func(c *DeviceConfig) { c.DeviceID = "" },
		"bad device":       func(c *DeviceConfig) { c.DeviceID = "ccd 1" },
		"empty token":      func(c *DeviceConfig) { c.Token = "" },
		"token with space": func(c *DeviceConfig) { c.Token = "a b" },
		"short cert":       func(c *DeviceConfig) { c.CertSHA256 = "abcd" },
		"upper cert":       func(c *DeviceConfig) { c.CertSHA256 = strings.Repeat("AB", 32) },
		"cert on http": func(c *DeviceConfig) {
			c.URL = "http://x.example"
		},
	}
	for name, mutate := range cases {
		cfg := sampleDeviceConfig()
		mutate(&cfg)
		if _, err := EncodeDeviceConfig(cfg); ErrorCode(err) != CodeBadRequest {
			t.Errorf("%s: err = %v, want CYD_BAD_REQUEST", name, err)
		}
	}
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
		"unknown field": func(b []byte) []byte {
			body := []byte(`{"url":"http://x.example","device_id":"d1","token":"t","cert_sha256":"","x":1}`)
			copy(b[8:], body)
			for i := 8 + len(body); i < len(b); i++ {
				b[i] = 0
			}
			binary.LittleEndian.PutUint16(b[6:8], uint16(len(body)))
			return b
		},
	}
	for name, mutate := range cases {
		blob := mutate(append([]byte(nil), good...))
		if _, err := DecodeDeviceConfig(blob); ErrorCode(err) != CodeBadRequest {
			t.Errorf("%s: err = %v, want CYD_BAD_REQUEST", name, err)
		}
	}
}
