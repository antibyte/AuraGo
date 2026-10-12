package cydcreator

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"slices"
	"strings"
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

// TestRenderAssetsUsesPixelSize pins that the descriptor header and the
// comment describe the pixels that are written, not the layout's numbers.
func TestRenderAssetsUsesPixelSize(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	for i := range img.Pix {
		img.Pix[i] = 0xFF
	}
	got := renderAssets([]genImage{{asset: Asset{ID: "a_x", File: "assets/x.png", W: 9, H: 9}, img: img}})
	for _, want := range []string{
		"// a_x: assets/x.png, 2x1, RGB565, 4 bytes\n",
		"    {LV_IMAGE_HEADER_MAGIC, LV_COLOR_FORMAT_RGB565, 0, 2, 1, 4, 0},\n    4,\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("ui_assets.cpp lacks %q:\n%s", want, got)
		}
	}
}

// TestEncodeImageAlphaPlaneOrder pins RGB565A8 for more than one row: the whole
// color plane (row by row), then the whole alpha plane.
func TestEncodeImageAlphaPlaneOrder(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 0xFF, A: 0x11}) // 0xF800
	img.SetNRGBA(1, 0, color.NRGBA{G: 0xFF, A: 0x22}) // 0x07E0
	img.SetNRGBA(0, 1, color.NRGBA{B: 0xFF, A: 0x33}) // 0x001F
	img.SetNRGBA(1, 1, color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x44})
	want := []byte{0x00, 0xF8, 0xE0, 0x07, 0x1F, 0x00, 0xFF, 0xFF, 0x11, 0x22, 0x33, 0x44}
	if got := encodeImage(img, true); !slices.Equal(got, want) {
		t.Errorf("2x2 RGB565A8 = % X, want % X", got, want)
	}
}

func TestInspectAssetSides(t *testing.T) {
	for name, dims := range map[string][2]int{"widest": {maxWidgetSide, 1}, "tallest": {1, maxWidgetSide}} {
		w, h, _, _, err := InspectAsset(encodePNG(t, image.NewNRGBA(image.Rect(0, 0, dims[0], dims[1]))))
		if err != nil || w != dims[0] || h != dims[1] {
			t.Errorf("%s: InspectAsset = %d %d %v, want %dx%d", name, w, h, err, dims[0], dims[1])
		}
	}
	_, _, _, _, err := InspectAsset(encodePNG(t, image.NewNRGBA(image.Rect(0, 0, 1, maxWidgetSide+1))))
	if ErrorCode(err) != CodeBadRequest || !strings.Contains(err.Error(), "must be within") {
		t.Errorf("too tall: err = %v, want CYD_BAD_REQUEST (size limit)", err)
	}
}

// TestInspectAssetPalettedTransparency: a palette entry with alpha < 255 (PNG
// tRNS chunk) makes the picture RGB565A8, and the color stays straight.
func TestInspectAssetPalettedTransparency(t *testing.T) {
	pal := color.Palette{color.NRGBA{R: 0xFF, A: 0xFF}, color.NRGBA{G: 0xFF, A: 0x80}}
	img := image.NewPaletted(image.Rect(0, 0, 2, 1), pal)
	img.SetColorIndex(0, 0, 0)
	img.SetColorIndex(1, 0, 1)
	data := encodePNG(t, img)
	w, h, alpha, size, err := InspectAsset(data)
	if err != nil || w != 2 || h != 1 || !alpha || size != 2*1*3 {
		t.Fatalf("InspectAsset(paletted with tRNS) = %d %d %v %d %v, want 2 1 true 6", w, h, alpha, size, err)
	}
	decoded, err := decodeAsset(data)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := encodeImage(decoded, true), []byte{0x00, 0xF8, 0xE0, 0x07, 0xFF, 0x80}; !slices.Equal(got, want) {
		t.Errorf("paletted RGB565A8 = % X, want % X", got, want)
	}
	opaquePal := color.Palette{color.NRGBA{R: 0xFF, A: 0xFF}, color.NRGBA{B: 0xFF, A: 0xFF}}
	if _, _, alpha, size, err := InspectAsset(encodePNG(t, image.NewPaletted(image.Rect(0, 0, 2, 1), opaquePal))); err != nil || alpha || size != 4 {
		t.Errorf("InspectAsset(opaque paletted) = %v %d %v, want false 4", alpha, size, err)
	}
}

func TestInspectAsset16Bit(t *testing.T) {
	img := image.NewNRGBA64(image.Rect(0, 0, 2, 1))
	img.SetNRGBA64(0, 0, color.NRGBA64{R: 0xFFFF, G: 0x8000, B: 0, A: 0xFFFF})
	img.SetNRGBA64(1, 0, color.NRGBA64{R: 0x1234, G: 0x5678, B: 0x9ABC, A: 0xFFFF})
	data := encodePNG(t, img)
	w, h, alpha, size, err := InspectAsset(data)
	if err != nil || w != 2 || h != 1 || alpha || size != 4 {
		t.Fatalf("InspectAsset(16-bit opaque) = %d %d %v %d %v, want 2 1 false 4", w, h, alpha, size, err)
	}
	decoded, err := decodeAsset(data)
	if err != nil {
		t.Fatal(err)
	}
	// The high byte of each 16-bit channel feeds the 5-6-5 truncation.
	if got, want := encodeImage(decoded, false), []byte{0x00, 0xFC, 0xB3, 0x12}; !slices.Equal(got, want) {
		t.Errorf("16-bit RGB565 = % X, want % X", got, want)
	}
	img.SetNRGBA64(1, 0, color.NRGBA64{R: 0x1234, G: 0x5678, B: 0x9ABC, A: 0xFFFE})
	if _, _, alpha, size, err := InspectAsset(encodePNG(t, img)); err != nil || !alpha || size != 6 {
		t.Errorf("InspectAsset(16-bit, alpha 0xFFFE) = %v %d %v, want true 6", alpha, size, err)
	}
}

// headerOnlyPNG is the PNG signature and a valid IHDR chunk and nothing else:
// image.DecodeConfig reads the size, image.Decode fails (no pixel data), so a
// test that expects a size error can tell it did not come from decoding.
func headerOnlyPNG(w, h int) []byte {
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], uint32(w))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(h))
	ihdr[8] = 16 // bit depth
	ihdr[9] = 6  // RGBA
	chunk := append([]byte("IHDR"), ihdr...)
	var n [4]byte
	var buf bytes.Buffer
	buf.WriteString("\x89PNG\r\n\x1a\n")
	binary.BigEndian.PutUint32(n[:], uint32(len(ihdr)))
	buf.Write(n[:])
	buf.Write(chunk)
	binary.BigEndian.PutUint32(n[:], crc32.ChecksumIEEE(chunk))
	buf.Write(n[:])
	return buf.Bytes()
}

// assetsOf builds a project with the given sized assets (ids a_0, a_1, ...) and
// their header-only PNGs.
func assetsOf(sizes ...[2]int) (Project, map[string][]byte) {
	var p Project
	data := map[string][]byte{}
	for i, s := range sizes {
		id := fmt.Sprintf("a_%d", i)
		p.Assets = append(p.Assets, Asset{ID: id, File: "assets/" + id + ".png", W: s[0], H: s[1]})
		data[id] = headerOnlyPNG(s[0], s[1])
	}
	return p, data
}

func wantAssetError(t *testing.T, err error, asset, reason string) *Error {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) || e.Code != CodeBadRequest {
		t.Fatalf("err = %v, want CYD_BAD_REQUEST", err)
	}
	if e.Details["asset"] != asset {
		t.Errorf("details.asset = %v, want %s (%v)", e.Details["asset"], asset, err)
	}
	if reason != "" && e.Details["reason"] != reason {
		t.Errorf("details.reason = %v, want %s (%v)", e.Details["reason"], reason, err)
	}
	if strings.Contains(e.Message, "damaged") {
		t.Errorf("error came from decoding, not from the header check: %v", err)
	}
	return e
}

// TestDecodeProjectAssetsHeaderPass: images that cannot be used are refused
// from their headers alone, before any pixel is decoded (the data of these
// PNGs is missing, so a decode attempt would report "damaged").
func TestDecodeProjectAssetsHeaderPass(t *testing.T) {
	// Precondition: the crafted header is readable but not decodable.
	if _, err := decodeAsset(headerOnlyPNG(8, 8)); err == nil || !strings.Contains(err.Error(), "damaged") {
		t.Fatalf("headerOnlyPNG must fail the full decode, got %v", err)
	}
	for name, size := range map[string][2]int{
		"2048x2048":      {2048, 2048},
		"long side":      {801, 10},
		"long side tall": {10, 801},
		"short side":     {481, 481},
	} {
		p, data := assetsOf(size)
		_, _, err := decodeProjectAssets(p, data)
		e := wantAssetError(t, err, "a_0", "dimensions")
		if e.Details["width"] != size[0] || e.Details["height"] != size[1] || e.Details["max_width"] != 800 || e.Details["max_height"] != 480 {
			t.Errorf("%s: details = %v", name, e.Details)
		}
	}
	// The largest supported display itself passes the header pass (and then
	// fails the decode: the test data has no pixels).
	for _, size := range [][2]int{{800, 480}, {480, 800}} {
		p, data := assetsOf(size)
		if _, _, err := decodeProjectAssets(p, data); err == nil || !strings.Contains(err.Error(), "damaged") {
			t.Errorf("%v: err = %v, want the decode failure of the header-only PNG", size, err)
		}
	}

	// Budget: 800x480 needs 768000 bytes as RGB565 at least; five fit into the
	// 4 MiB budget, the sixth does not, and that is refused before the first
	// image is decoded (every image alone is a valid display-sized picture).
	sizes := make([][2]int, 6)
	for i := range sizes {
		sizes[i] = [2]int{800, 480}
	}
	p, data := assetsOf(sizes[:5]...)
	if _, _, err := decodeProjectAssets(p, data); err == nil || !strings.Contains(err.Error(), "damaged") {
		t.Errorf("5 assets: err = %v, want the decode failure (budget not exceeded)", err)
	}
	p, data = assetsOf(sizes...)
	_, _, err := decodeProjectAssets(p, data)
	e := wantAssetError(t, err, "a_5", "budget")
	if e.Details["needed"] != int64(6*800*480*2) || e.Details["budget"] != int64(AssetBudgetLarge) {
		t.Errorf("budget details = %v", e.Details)
	}
}

func TestDecodeProjectAssetsSizeMismatchBeforeDecode(t *testing.T) {
	p, data := assetsOf([2]int{10, 10})
	data["a_0"] = headerOnlyPNG(20, 20)
	_, _, err := decodeProjectAssets(p, data)
	e := wantAssetError(t, err, "a_0", "")
	if !strings.Contains(e.Message, "is 20x20 but the layout says 10x10") {
		t.Errorf("message = %q", e.Message)
	}
}

func TestDecodeProjectAssetsErrors(t *testing.T) {
	opaque := image.NewNRGBA(image.Rect(0, 0, 4, 3))
	for i := range opaque.Pix {
		opaque.Pix[i] = 0xFF
	}
	var gifData bytes.Buffer
	if err := gif.Encode(&gifData, opaque, nil); err != nil {
		t.Fatal(err)
	}
	asset := func(w, h int) Project {
		return Project{Assets: []Asset{{ID: "a_x", File: "assets/x.png", W: w, H: h}}}
	}
	t.Run("missing data", func(t *testing.T) {
		_, _, err := decodeProjectAssets(asset(4, 3), map[string][]byte{"other": nil})
		e := wantAssetError(t, err, "a_x", "")
		if !strings.Contains(e.Message, "asset a_x has no image data") {
			t.Errorf("message = %q", e.Message)
		}
	})
	t.Run("layout size differs", func(t *testing.T) {
		_, _, err := decodeProjectAssets(asset(5, 5), map[string][]byte{"a_x": encodePNG(t, opaque)})
		e := wantAssetError(t, err, "a_x", "")
		if !strings.Contains(e.Message, "asset a_x is 4x3 but the layout says 5x5") {
			t.Errorf("message = %q", e.Message)
		}
	})
	t.Run("garbage", func(t *testing.T) {
		_, _, err := decodeProjectAssets(asset(4, 3), map[string][]byte{"a_x": []byte("not an image")})
		e := wantAssetError(t, err, "a_x", "")
		if !strings.HasPrefix(e.Message, "asset a_x: ") {
			t.Errorf("message = %q, want the \"asset a_x: \" prefix", e.Message)
		}
	})
	t.Run("truncated data", func(t *testing.T) {
		data := encodePNG(t, opaque)
		_, _, err := decodeProjectAssets(asset(4, 3), map[string][]byte{"a_x": data[:len(data)-12]})
		var e *Error
		if !errors.As(err, &e) || e.Code != CodeBadRequest || !strings.HasPrefix(e.Message, "asset a_x: ") || e.Details["asset"] != "a_x" {
			t.Errorf("err = %v, want CYD_BAD_REQUEST with the asset prefix and details", err)
		}
	})
	t.Run("details are merged", func(t *testing.T) {
		_, _, err := decodeProjectAssets(asset(4, 3), map[string][]byte{"a_x": gifData.Bytes()})
		e := wantAssetError(t, err, "a_x", "")
		if e.Details["format"] != "gif" || !strings.HasPrefix(e.Message, "asset a_x: image format") {
			t.Errorf("gif: details = %v, message = %q (format must stay in the details)", e.Details, e.Message)
		}
		_, _, err = decodeProjectAssets(asset(maxWidgetSide+1, 1), map[string][]byte{"a_x": encodePNG(t, image.NewNRGBA(image.Rect(0, 0, maxWidgetSide+1, 1)))})
		e = wantAssetError(t, err, "a_x", "")
		if e.Details["width"] != maxWidgetSide+1 || e.Details["height"] != 1 || e.Details["limit"] != maxWidgetSide {
			t.Errorf("too wide: details = %v (width/height/limit must stay)", e.Details)
		}
	})
}

func TestDecodeProjectAssets(t *testing.T) {
	opaque := image.NewNRGBA(image.Rect(0, 0, 4, 3))
	for i := range opaque.Pix {
		opaque.Pix[i] = 0xFF
	}
	translucent := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	translucent.SetNRGBA(1, 1, color.NRGBA{R: 1, G: 2, B: 3, A: 0x7F})
	p := Project{Assets: []Asset{
		{ID: "a_one", File: "assets/one.png", W: 4, H: 3},
		{ID: "a_two", File: "assets/two.png", W: 2, H: 2},
	}}
	images, sizes, err := decodeProjectAssets(p, map[string][]byte{
		"a_one": encodePNG(t, opaque), "a_two": encodePNG(t, translucent), "unused": []byte("ignored"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 2 || images[0].asset.ID != "a_one" || images[0].alpha || images[1].asset.ID != "a_two" || !images[1].alpha {
		t.Errorf("images = %+v", images)
	}
	if sizes["a_one"] != 4*3*2 || sizes["a_two"] != 2*2*3 || len(sizes) != 2 {
		t.Errorf("sizes = %v, want a_one 24 and a_two 12", sizes)
	}
	if images, sizes, err := decodeProjectAssets(Project{}, nil); err != nil || len(images) != 0 || len(sizes) != 0 {
		t.Errorf("no assets: %v %v %v", images, sizes, err)
	}
}
