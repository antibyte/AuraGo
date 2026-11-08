package storymaker

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/png"
	"testing"
	"testing/fstest"
)

func TestCatalogDeliveryChecksBytesHashTypeAndDimensions(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 4, 3))); err != nil {
		t.Fatal(err)
	}
	data := encoded.Bytes()
	hash := sha256.Sum256(data)
	meta := CatalogFile{Path: "textures/actor.png", Type: "image/png", Bytes: int64(len(data)), SHA256: hex.EncodeToString(hash[:]), Width: 4, Height: 3}
	for _, test := range []struct {
		name   string
		change func(*CatalogFile, *[]byte)
		fail   bool
	}{
		{"valid", func(*CatalogFile, *[]byte) {}, false},
		{"changed pixels", func(_ *CatalogFile, b *[]byte) { (*b)[len(*b)-1] ^= 1 }, true},
		{"truncated", func(_ *CatalogFile, b *[]byte) { *b = (*b)[:len(*b)-1] }, true},
		{"extra bytes", func(_ *CatalogFile, b *[]byte) { *b = append(*b, 0) }, true},
		{"dimension mismatch", func(m *CatalogFile, _ *[]byte) { m.Width++ }, true},
		{"type mismatch", func(m *CatalogFile, _ *[]byte) { m.Type = "image/webp" }, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, b := meta, append([]byte(nil), data...)
			test.change(&m, &b)
			c := &Catalog{Schema: CatalogSchema, files: []CatalogFile{m}}
			err := c.VerifyFiles(fstest.MapFS{"catalog/" + m.Path: &fstest.MapFile{Data: b}}, "catalog")
			if (err != nil) != test.fail {
				t.Fatalf("VerifyFiles error = %v, want failure %v", err, test.fail)
			}
		})
	}
}

func TestCatalogDeliveryManifestRejectsUnsafeOrUnboundedMetadata(t *testing.T) {
	valid := CatalogFile{Path: "texture.png", Type: "image/png", Bytes: 30, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Width: 4, Height: 3}
	for _, test := range []struct {
		name   string
		change func(*CatalogFile)
	}{
		{"traversal", func(f *CatalogFile) { f.Path = "../outside.png" }},
		{"absolute", func(f *CatalogFile) { f.Path = "/outside.png" }},
		{"bad hash", func(f *CatalogFile) { f.SHA256 = "bad" }},
		{"oversized", func(f *CatalogFile) { f.Bytes = CatalogTextureLimit + 1 }},
		{"zero bytes", func(f *CatalogFile) { f.Bytes = 0 }},
		{"decode bomb", func(f *CatalogFile) { f.Width = 100000 }},
		{"oversized atlas", func(f *CatalogFile) { f.Width = 2049 }},
		{"oversized foreground atlas", func(f *CatalogFile) { f.Path = "textures/environments/foreground-day.png"; f.Width = 2049 }},
		{"oversized thumbnail", func(f *CatalogFile) { f.Path = "thumbs/actor.png"; f.Width = 257 }},
		{"invalid type", func(f *CatalogFile) { f.Type = "image/svg+xml" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := valid
			test.change(&f)
			c := &Catalog{Schema: CatalogSchema, Rig: RigRef{File: f.Path}, files: []CatalogFile{f}}
			var errors []string
			c.checkFiles(func(s string, _ ...any) { errors = append(errors, s) })
			if len(errors) == 0 {
				t.Fatal("invalid metadata was accepted")
			}
		})
	}
	c := &Catalog{files: []CatalogFile{valid}}
	copy := c.DeliveryFiles()
	copy[0].Path = "changed"
	if c.files[0].Path != valid.Path {
		t.Fatal("DeliveryFiles exposed mutable catalog storage")
	}
}
