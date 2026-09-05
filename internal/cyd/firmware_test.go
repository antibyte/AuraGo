package cyd

import (
	"os"
	"path/filepath"
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
	root := t.TempDir()
	dir := filepath.Join(root, "cyd")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"bootloader.bin", "partitions.bin", "boot_app0.bin", "firmware.bin"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "version.txt"), []byte("0.2.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CYD_FIRMWARE_DIR", root)
	got := DiscoverFirmware("")
	var cyd VariantInfo
	for _, v := range got {
		if v.ID == "cyd" {
			cyd = v
		}
	}
	if !cyd.Available {
		t.Fatalf("cyd not available: %+v", cyd)
	}
	if cyd.Version != "0.2.1" {
		t.Fatalf("version=%q", cyd.Version)
	}
	path := FirmwareFilePath("", "cyd", "firmware.bin")
	if path == "" {
		t.Fatal("missing firmware path")
	}
	if FirmwareFilePath("", "cyd", "../secret.bin") != "" {
		t.Fatal("path traversal must be rejected")
	}
}
