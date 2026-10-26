package qrcode

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"testing"
)

func jpegEXIFFixture(t *testing.T, orientation int, order binary.ByteOrder) ([]byte, image.Image) {
	t.Helper()
	source := image.NewNRGBA(image.Rect(0, 0, 32, 16))
	palette := []color.NRGBA{{R: 255, A: 255}, {G: 255, A: 255}, {B: 255, A: 255}, {R: 255, G: 255, A: 255}}
	for i, c := range palette {
		x, y := (i%2)*16, (i/2)*8
		draw.Draw(source, image.Rect(x, y, x+16, y+8), image.NewUniform(c), image.Point{}, draw.Src)
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, source, &jpeg.Options{Quality: 98}); err != nil {
		t.Fatal(err)
	}
	baseline, _, err := decodeRaster(context.Background(), b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	// Invalid GPS/next-IFD pointers verify that orientation never follows them.
	const privateMetadata = "PRIVATE-GPS-METADATA-IS-STRIPPED"
	tiff := make([]byte, 8+2+24+4+len(privateMetadata))
	if order == binary.LittleEndian {
		copy(tiff, "II")
	} else {
		copy(tiff, "MM")
	}
	order.PutUint16(tiff[2:], 42)
	order.PutUint32(tiff[4:], 8)
	order.PutUint16(tiff[8:], 2)
	entry := tiff[10:]
	order.PutUint16(entry, 0x112)
	order.PutUint16(entry[2:], 3)
	order.PutUint32(entry[4:], 1)
	order.PutUint16(entry[8:], uint16(orientation))
	gps := tiff[22:]
	order.PutUint16(gps, 0x8825)
	order.PutUint16(gps[2:], 4)
	order.PutUint32(gps[4:], 1)
	order.PutUint32(gps[8:], 0xfffffff0)
	order.PutUint32(tiff[34:], 0xfffffff0)
	copy(tiff[38:], privateMetadata)
	payload := append([]byte("Exif\x00\x00"), tiff...)
	segment := []byte{0xff, 0xe1, 0, 0}
	binary.BigEndian.PutUint16(segment[2:], uint16(len(payload)+2))
	raw := append(bytes.Clone(b.Bytes()[:2]), segment...)
	raw = append(raw, payload...)
	raw = append(raw, b.Bytes()[2:]...)
	return raw, baseline
}

func TestNormalizeJPEGEXIFOrientationAndMetadata(t *testing.T) {
	arrangements := [][4]int{{0, 1, 2, 3}, {1, 0, 3, 2}, {3, 2, 1, 0}, {2, 3, 0, 1}, {0, 2, 1, 3}, {2, 0, 3, 1}, {3, 1, 2, 0}, {1, 3, 0, 2}}
	for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		for orientation := 1; orientation <= 8; orientation++ {
			raw, baseline := jpegEXIFFixture(t, orientation, order)
			if got := jpegOrientation(raw); got != orientation {
				t.Fatalf("orientation tag=%d want=%d", got, orientation)
			}
			resource, err := NormalizeImage(context.Background(), "image/jpeg", raw)
			if err != nil {
				t.Fatal(err)
			}
			if resource.MIME != "image/png" || bytes.Contains(resource.Data, []byte("PRIVATE-GPS-METADATA")) || bytes.Contains(resource.Data, []byte("Exif\x00\x00")) {
				t.Fatal("metadata survived PNG normalization")
			}
			normalized, _, err := decodeRaster(context.Background(), resource.Data)
			if err != nil {
				t.Fatal(err)
			}
			w, h := 32, 16
			if orientation >= 5 {
				w, h = h, w
			}
			if normalized.Bounds().Dx() != w || normalized.Bounds().Dy() != h {
				t.Fatalf("orientation%d dimensions: %v", orientation, normalized.Bounds())
			}
			for quadrant, index := range arrangements[orientation-1] {
				sx, sy := (index%2)*16+8, (index/2)*8+4
				dx, dy := (quadrant%2)*w/2+w/4, (quadrant/2)*h/2+h/4
				want := color.NRGBAModel.Convert(baseline.At(sx, sy))
				got := color.NRGBAModel.Convert(normalized.At(dx, dy))
				if got != want {
					t.Fatalf("orientation%d quadrant%d pixels differ", orientation, quadrant)
				}
			}
		}
	}
}
func TestJPEGOrientationMalformedMetadataAndCancellation(t *testing.T) {
	raw, _ := jpegEXIFFixture(t, 6, binary.LittleEndian)
	for _, data := range [][]byte{nil, []byte{0xff, 0xd8}, raw[:12], {0xff, 0xd8, 0xff, 0xe1, 0, 1}, {0xff, 0xd8, 0xff, 0xe1, 0xff, 0xff}} {
		if got := jpegOrientation(data); got != 1 {
			t.Fatal("malformed metadata changed orientation")
		}
	}
	for _, orientation := range []int{0, 9, 65535} {
		data, _ := jpegEXIFFixture(t, orientation, binary.BigEndian)
		if got := jpegOrientation(data); got != 1 {
			t.Fatal("invalid orientation changed image")
		}
	}
	offset := bytes.Index(raw, []byte("Exif\x00\x00")) + 6
	badOffset := bytes.Clone(raw)
	binary.LittleEndian.PutUint32(badOffset[offset+4:], 0xfffffff0)
	badCount := bytes.Clone(raw)
	binary.LittleEndian.PutUint16(badCount[offset+8:], 0xffff)
	for _, data := range [][]byte{badOffset, badCount} {
		if got := jpegOrientation(data); got != 1 {
			t.Fatal("out-of-bounds EXIF was followed")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := orientImage(ctx, image.NewNRGBA(image.Rect(0, 0, 4, 2)), 6); !errors.Is(err, context.Canceled) {
		t.Fatal("orientation ignored cancellation")
	}
}
func FuzzJPEGOrientation(f *testing.F) {
	f.Add([]byte{0xff, 0xd8, 0xff, 0xd9})
	f.Add([]byte("Exif\x00\x00II"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 65536 {
			t.Skip()
		}
		got := jpegOrientation(data)
		if got < 1 || got > 8 {
			t.Fatal("orientation outside bounded range")
		}
	})
}
