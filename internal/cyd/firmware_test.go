package cyd

import (
	"testing"
)

func TestEncodeFactoryBlobRoundTrip(t *testing.T) {
	buf, err := EncodeFactoryBlob("https://192.168.1.9:8443", "aura_ABCDEFGHJ")
	if err != nil {
		t.Fatal(err)
	}
	if len(buf) != FactorySize {
		t.Fatalf("len=%d", len(buf))
	}
	url, token, ok := DecodeFactoryBlob(buf)
	if !ok {
		t.Fatal("decode failed")
	}
	if url != "https://192.168.1.9:8443" || token != "aura_ABCDEFGHJ" {
		t.Fatalf("url=%q token=%q", url, token)
	}
}

func TestDiscoverFirmwareFindsPack(t *testing.T) {
	got := DiscoverFirmware()
	var pack VariantInfo
	for _, v := range got {
		if v.ID == "cyd" {
			pack = v
		}
	}
	if !pack.Available {
		t.Fatalf("cyd not available: %+v", pack)
	}
	if pack.Version == "" {
		t.Fatal("missing bundled version")
	}
	data, err := FirmwareBytes("cyd", "firmware.bin")
	if err != nil || len(data) < 1024 {
		t.Fatalf("firmware.bin: err=%v len=%d", err, len(data))
	}
	if _, err := FirmwareBytes("cyd", "../secret.bin"); err == nil {
		t.Fatal("path traversal must be rejected")
	}
}
