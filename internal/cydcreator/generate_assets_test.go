package cydcreator

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"slices"
	"testing"
)

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestInspectAsset(t *testing.T) {
	opaque := image.NewNRGBA(image.Rect(0, 0, 4, 3))
	for i := range opaque.Pix {
		opaque.Pix[i] = 0xFF
	}
	translucent := image.NewNRGBA(image.Rect(0, 0, 5, 2))
	for i := range translucent.Pix {
		translucent.Pix[i] = 0xFF
	}
	translucent.SetNRGBA(4, 1, color.NRGBA{R: 10, G: 20, B: 30, A: 254})
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, opaque, nil); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		data  []byte
		w, h  int
		alpha bool
		size  int64
	}{
		{"opaque png", encodePNG(t, opaque), 4, 3, false, 4 * 3 * 2},
		{"png with alpha", encodePNG(t, translucent), 5, 2, true, 5 * 2 * 3},
		{"jpeg", jpg.Bytes(), 4, 3, false, 4 * 3 * 2},
	}
	for _, tc := range cases {
		w, h, alpha, size, err := InspectAsset(tc.data)
		if err != nil || w != tc.w || h != tc.h || alpha != tc.alpha || size != tc.size {
			t.Errorf("%s: InspectAsset = %d %d %v %d %v, want %d %d %v %d", tc.name, w, h, alpha, size, err, tc.w, tc.h, tc.alpha, tc.size)
		}
	}
	var gifData bytes.Buffer
	if err := gif.Encode(&gifData, opaque, nil); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		"garbage":   []byte("definitely not an image"),
		"gif":       gifData.Bytes(),
		"too wide":  encodePNG(t, image.NewNRGBA(image.Rect(0, 0, maxWidgetSide+1, 1))),
		"truncated": encodePNG(t, opaque)[:40],
	} {
		if _, _, _, _, err := InspectAsset(data); ErrorCode(err) != CodeBadRequest {
			t.Errorf("%s: err = %v, want CYD_BAD_REQUEST", name, err)
		}
	}
}

// TestEncodeImagePixels pins the LVGL 9 byte layout: RGB565 little-endian
// (lv_color_to_u16: rrrrrggg gggbbbbb, low byte first), RGB565A8 = all color
// pixels, then one alpha byte per pixel; colors are not premultiplied.
func TestEncodeImagePixels(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 0xFF, G: 0x00, B: 0x00, A: 0xFF}) // 0xF800
	img.SetNRGBA(1, 0, color.NRGBA{R: 0x00, G: 0xFF, B: 0x00, A: 0x80}) // 0x07E0, straight color despite alpha
	img.SetNRGBA(2, 0, color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 0x00}) // 0x11AA
	if got, want := encodeImage(img, false), []byte{0x00, 0xF8, 0xE0, 0x07, 0xAA, 0x11}; !slices.Equal(got, want) {
		t.Errorf("RGB565 = % X, want % X", got, want)
	}
	if got, want := encodeImage(img, true), []byte{0x00, 0xF8, 0xE0, 0x07, 0xAA, 0x11, 0xFF, 0x80, 0x00}; !slices.Equal(got, want) {
		t.Errorf("RGB565A8 = % X, want % X", got, want)
	}
	// Paletted and gray images go through the same conversion.
	gray := image.NewGray(image.Rect(0, 0, 1, 1))
	gray.SetGray(0, 0, color.Gray{Y: 0x80})
	if got, want := encodeImage(gray, false), []byte{0x10, 0x84}; !slices.Equal(got, want) {
		t.Errorf("gray = % X, want % X", got, want)
	}
}

func TestRenderAssets(t *testing.T) {
	if got := renderAssets(nil); got != genHeader("ui_assets.cpp")+
		"// Images as LVGL 9 image descriptors (RGB565, or RGB565A8 with alpha).\n#include \"ui.h\"\n\n// This project has no images.\n" {
		t.Errorf("empty ui_assets.cpp:\n%s", got)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 0xFF, A: 0xFF})
	img.SetNRGBA(1, 0, color.NRGBA{B: 0xFF, A: 0x40})
	got := renderAssets([]genImage{{asset: Asset{ID: "a_dot", File: "assets/dot.png", W: 2, H: 1, Alpha: true}, img: img, alpha: true}})
	want := genHeader("ui_assets.cpp") + `// Images as LVGL 9 image descriptors (RGB565, or RGB565A8 with alpha).
#include "ui.h"

// The positional header initializers follow the little-endian field order.
#if LV_BIG_ENDIAN_SYSTEM
#error "ui_assets.cpp expects the little-endian lv_image_header_t layout"
#endif

// a_dot: assets/dot.png, 2x1, RGB565A8, 6 bytes
static const uint8_t ui__img_a_dot_map[] __attribute__((aligned(4))) = {
  0x00, 0xF8, 0x1F, 0x00, 0xFF, 0x40,
};

const lv_image_dsc_t ui_img_a_dot = {
    {LV_IMAGE_HEADER_MAGIC, LV_COLOR_FORMAT_RGB565A8, 0, 2, 1, 4, 0},
    6,
    ui__img_a_dot_map,
    NULL,
    NULL,
};
`
	if got != want {
		t.Errorf("ui_assets.cpp:\n%s", firstDiff(want, got))
	}
}
