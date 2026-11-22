package media

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func assertEmptyDir(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	if len(entries) != 0 {
		t.Fatalf("%s holds %d file(s), want none", dir, len(entries))
	}
}

func smallPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for x := 0; x < 4; x++ {
		for y := 0; y < 4; y++ {
			img.Set(x, y, color.RGBA{R: uint8(60 * x), G: uint8(60 * y), B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	return buf.Bytes()
}

// headerOnlyPNG builds a PNG that declares width x height but carries no image
// data: signature, IHDR (8-bit truecolor) with a correct CRC, then IEND.
func headerOnlyPNG(width, height uint32) []byte {
	chunk := func(kind string, data []byte) []byte {
		out := make([]byte, 0, 12+len(data))
		out = binary.BigEndian.AppendUint32(out, uint32(len(data)))
		out = append(out, kind...)
		out = append(out, data...)
		return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(append([]byte(kind), data...)))
	}
	ihdr := binary.BigEndian.AppendUint32(nil, width)
	ihdr = binary.BigEndian.AppendUint32(ihdr, height)
	ihdr = append(ihdr, 8, 2, 0, 0, 0)
	out := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}
	out = append(out, chunk("IHDR", ihdr)...)
	return append(out, chunk("IEND", nil)...)
}

func TestSanitizeImageFilePixelCap(t *testing.T) {
	write := func(t *testing.T, data []byte) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "in.png")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("write input: %v", err)
		}
		return path
	}

	t.Run("oversized canvas is refused before decode", func(t *testing.T) {
		destDir := t.TempDir()
		_, err := SanitizeImageFile(write(t, headerOnlyPNG(20000, 10000)), destDir)
		if err == nil {
			t.Fatal("a 20000x10000 PNG header was accepted")
		}
		if !strings.Contains(err.Error(), "20000x10000") || !strings.Contains(err.Error(), "100000000") {
			t.Fatalf("error = %q, want the dimensions and the limit", err)
		}
		if strings.Contains(err.Error(), "decode failed") {
			t.Fatalf("error = %q, the image was decoded instead of refused from its header", err)
		}
		assertEmptyDir(t, destDir)
	})

	t.Run("limit is inclusive", func(t *testing.T) {
		// 10000x10000 is exactly maxImagePixels: it passes the cap and then fails
		// to decode (no image data), which proves the cap did not fire.
		_, err := SanitizeImageFile(write(t, headerOnlyPNG(10000, 10000)), t.TempDir())
		if err == nil {
			t.Fatal("a header-only PNG decoded")
		}
		if strings.Contains(err.Error(), "exceed") || !strings.Contains(err.Error(), "decode failed") {
			t.Fatalf("error = %q, want a decode failure below the cap", err)
		}
	})

	t.Run("one pixel over the limit", func(t *testing.T) {
		_, err := SanitizeImageFile(write(t, headerOnlyPNG(10001, 10000)), t.TempDir())
		if err == nil || !strings.Contains(err.Error(), "exceed") {
			t.Fatalf("error = %v, want the pixel cap", err)
		}
	})

	t.Run("normal image passes", func(t *testing.T) {
		destDir := t.TempDir()
		path, err := SanitizeImageFile(write(t, smallPNG(t)), destDir)
		if err != nil {
			t.Fatalf("SanitizeImageFile: %v", err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("sanitized file missing: %v", err)
		}
	})
}
