package cydcreator

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg" // PNG and JPEG are the only accepted asset formats
	_ "image/png"
	"strings"
)

// genImage is one decoded project asset, ready to be written as an LVGL 9
// image descriptor.
type genImage struct {
	asset Asset
	img   image.Image
	alpha bool
}

func (im genImage) symbol() string { return imageSymbol(im.asset.ID) }

// imageSymbol is the public descriptor name of an asset (ui.h declares it).
func imageSymbol(assetID string) string { return "ui_img_" + assetID }

// assetHeader reads only the header of a PNG or JPEG: other formats (even if
// another package registered a decoder) and pictures above maxWidgetSide
// pixels per side are rejected without reading any pixel.
func assetHeader(data []byte) (image.Config, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return image.Config{}, NewError(CodeBadRequest, "the image is not a readable PNG or JPEG file", nil)
	}
	if format != "png" && format != "jpeg" {
		return image.Config{}, NewError(CodeBadRequest, fmt.Sprintf("image format %q is not supported (PNG or JPEG only)", format),
			map[string]any{"format": format})
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxWidgetSide || cfg.Height > maxWidgetSide {
		return image.Config{}, NewError(CodeBadRequest, fmt.Sprintf("image size %dx%d must be within 1..%d pixels per side", cfg.Width, cfg.Height, maxWidgetSide),
			map[string]any{"width": cfg.Width, "height": cfg.Height, "limit": maxWidgetSide})
	}
	return cfg, nil
}

// decodeAsset decodes a PNG or JPEG after checking its header (assetHeader).
func decodeAsset(data []byte) (image.Image, error) {
	if _, err := assetHeader(data); err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, NewError(CodeBadRequest, "the image data is damaged: "+err.Error(), nil)
	}
	return img, nil
}

// imageHasAlpha reports whether any pixel is not fully opaque.
func imageHasAlpha(img image.Image) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0xFFFF {
				return true
			}
		}
	}
	return false
}

// imageDataSize is the LVGL byte size of a decoded image: RGB565 = w*h*2,
// RGB565A8 = w*h*3.
func imageDataSize(w, h int, alpha bool) int64 {
	per := int64(2)
	if alpha {
		per = 3
	}
	return int64(w) * int64(h) * per
}

func assetDataSize(a Asset, alpha bool) int64 {
	if a.Encoding == "a8" {
		return int64(a.W) * int64(a.H)
	}
	return imageDataSize(a.W, a.H, alpha)
}

func encodeMask(img image.Image) []byte {
	b := img.Bounds()
	out := make([]byte, 0, b.Dx()*b.Dy())
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			out = append(out, byte(a>>8))
		}
	}
	return out
}

// InspectAsset decodes PNG/JPEG headers and pixels: size, whether any pixel
// is not opaque, and the decoded LVGL byte size. Errors are *Error
// CYD_BAD_REQUEST.
func InspectAsset(data []byte) (w, h int, alpha bool, decodedBytes int64, err error) {
	img, err := decodeAsset(data)
	if err != nil {
		return 0, 0, false, 0, err
	}
	b := img.Bounds()
	alpha = imageHasAlpha(img)
	return b.Dx(), b.Dy(), alpha, imageDataSize(b.Dx(), b.Dy(), alpha), nil
}

// encodeImage converts pixels to LVGL 9 RGB565 (little-endian, LVGL's native
// layout for LV_COLOR_DEPTH 16, see lv_color_to_u16) or RGB565A8 (the color
// plane followed by an 8-bit alpha plane, straight alpha).
func encodeImage(img image.Image, alpha bool) []byte {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := make([]byte, 0, imageDataSize(w, h, alpha))
	var alphas []byte
	if alpha {
		alphas = make([]byte, 0, w*h)
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			v := uint16(c.R&0xF8)<<8 | uint16(c.G&0xFC)<<3 | uint16(c.B)>>3
			out = append(out, byte(v), byte(v>>8))
			if alpha {
				alphas = append(alphas, c.A)
			}
		}
	}
	return append(out, alphas...)
}

// assetError scopes an asset problem to its asset id: "asset" is merged into
// the error's details (the other keys stay) and "asset <id>: " prefixes the
// message. Errors that are not *Error pass through unchanged.
func assetError(id string, err error) error {
	var e *Error
	if !errors.As(err, &e) || e == nil {
		return err
	}
	if e.Details == nil {
		e.Details = map[string]any{}
	}
	e.Details["asset"] = id
	e.Message = "asset " + id + ": " + e.Message
	return err
}

// maxBoardDisplay returns the largest long and the largest short display side
// over all board profiles: an asset that fits no supported display is refused
// before it is decoded.
func maxBoardDisplay() (long, short int, err error) {
	boards, err := Boards()
	if err != nil {
		return 0, 0, err
	}
	for _, b := range boards {
		long = max(long, b.Display.Width, b.Display.Height)
		short = max(short, min(b.Display.Width, b.Display.Height))
	}
	return long, short, nil
}

// decodeProjectAssets decodes every asset of the project from in.Assets.
// Missing data, an unreadable file or a size that differs from the layout
// are *Error CYD_BAD_REQUEST (the stored asset and the layout disagree).
//
// The layout is not validated yet when this runs (an imported project is
// untrusted) and decoded pixels are far larger than the files (a 16-bit RGBA
// PNG of 2048x2048 is ~37 KB compressed but ~64 MiB decoded). Pass 1 therefore
// reads only the headers of all assets and refuses, before any pixel is
// decoded: a header that differs from the layout, an image larger than the
// largest supported display (CYD_BAD_REQUEST, details.reason "dimensions"),
// and a project whose images would need more than AssetBudgetLarge even as
// opaque RGB565 (details.reason "budget"; Validate checks the exact budget of
// the board afterwards). That bounds pass 2, the full decode, to about 4M A8
// pixels in total.
func decodeProjectAssets(p Project, data map[string][]byte) ([]genImage, map[string]int64, error) {
	var maxLong, maxShort int
	if len(p.Assets) > 0 {
		var err error
		if maxLong, maxShort, err = maxBoardDisplay(); err != nil {
			return nil, nil, err
		}
	}
	var minBytes int64
	for _, a := range p.Assets {
		if a.Encoding != "" && a.Encoding != "a8" {
			return nil, nil, assetError(a.ID, fmt.Errorf("unsupported image encoding"))
		}
		raw, ok := data[a.ID]
		if !ok {
			return nil, nil, NewError(CodeBadRequest, fmt.Sprintf("asset %s has no image data", a.ID), map[string]any{"asset": a.ID})
		}
		cfg, err := assetHeader(raw)
		if err != nil {
			return nil, nil, assetError(a.ID, err)
		}
		if cfg.Width != a.W || cfg.Height != a.H {
			return nil, nil, NewError(CodeBadRequest, fmt.Sprintf("asset %s is %dx%d but the layout says %dx%d", a.ID, cfg.Width, cfg.Height, a.W, a.H),
				map[string]any{"asset": a.ID})
		}
		if max(cfg.Width, cfg.Height) > maxLong || min(cfg.Width, cfg.Height) > maxShort {
			return nil, nil, NewError(CodeBadRequest,
				fmt.Sprintf("asset %s is %dx%d, larger than the largest supported display (%dx%d)", a.ID, cfg.Width, cfg.Height, maxLong, maxShort),
				map[string]any{"asset": a.ID, "reason": "dimensions", "width": cfg.Width, "height": cfg.Height, "max_width": maxLong, "max_height": maxShort})
		}
		minBytes += assetDataSize(a, false)
		if minBytes > AssetBudgetLarge {
			return nil, nil, NewError(CodeBadRequest,
				fmt.Sprintf("the images need at least %d bytes, more than the largest budget of %d bytes (asset %s)", minBytes, int64(AssetBudgetLarge), a.ID),
				map[string]any{"asset": a.ID, "reason": "budget", "needed": minBytes, "budget": int64(AssetBudgetLarge)})
		}
	}
	images := make([]genImage, 0, len(p.Assets))
	sizes := make(map[string]int64, len(p.Assets))
	for _, a := range p.Assets {
		img, err := decodeAsset(data[a.ID])
		if err != nil {
			return nil, nil, assetError(a.ID, err)
		}
		b := img.Bounds()
		if b.Dx() != a.W || b.Dy() != a.H {
			return nil, nil, NewError(CodeBadRequest, fmt.Sprintf("asset %s is %dx%d but the layout says %dx%d", a.ID, b.Dx(), b.Dy(), a.W, a.H),
				map[string]any{"asset": a.ID})
		}
		alpha := imageHasAlpha(img)
		images = append(images, genImage{asset: a, img: img, alpha: alpha})
		sizes[a.ID] = assetDataSize(a, alpha)
	}
	return images, sizes, nil
}

// renderAssets writes src/generated/ui_assets.cpp.
func renderAssets(images []genImage) string {
	var w codeWriter
	w.raw(genHeader("ui_assets.cpp"))
	w.line(0, "// Images as LVGL 9 image descriptors (RGB565, or RGB565A8 with alpha).")
	w.line(0, `#include "ui.h"`)
	w.line(0, "")
	if len(images) == 0 {
		w.line(0, "// This project has no images.")
		return w.String()
	}
	w.line(0, "// The positional header initializers follow the little-endian field order.")
	w.line(0, "#if LV_BIG_ENDIAN_SYSTEM")
	w.line(0, `#error "ui_assets.cpp expects the little-endian lv_image_header_t layout"`)
	w.line(0, "#endif")
	for _, im := range images {
		a := im.asset
		cf, format := "LV_COLOR_FORMAT_RGB565", "RGB565"
		if im.alpha {
			cf, format = "LV_COLOR_FORMAT_RGB565A8", "RGB565A8"
		}
		// Size and stride come from the pixels that are written, like the
		// data itself (decodeProjectAssets guarantees they equal the layout).
		bounds := im.img.Bounds()
		iw, ih := bounds.Dx(), bounds.Dy()
		data := encodeImage(im.img, im.alpha)
		stride := iw * 2
		if a.Encoding == "a8" {
			cf, format, stride = "LV_COLOR_FORMAT_A8", "A8", iw
			data = encodeMask(im.img)
		}
		mapName := "ui__img_" + a.ID + "_map"
		w.line(0, "")
		w.line(0, "// %s: %s, %dx%d, %s, %d bytes", a.ID, cComment(a.File), iw, ih, format, len(data))
		w.line(0, "static const uint8_t %s[] __attribute__((aligned(4))) = {", mapName)
		for i := 0; i < len(data); i += 16 {
			end := min(i+16, len(data))
			parts := make([]string, 0, end-i)
			for _, c := range data[i:end] {
				parts = append(parts, fmt.Sprintf("0x%02X", c))
			}
			w.line(1, "%s,", strings.Join(parts, ", "))
		}
		w.line(0, "};")
		w.line(0, "")
		w.line(0, "const lv_image_dsc_t %s = {", im.symbol())
		w.line(2, "{LV_IMAGE_HEADER_MAGIC, %s, 0, %d, %d, %d, 0},", cf, iw, ih, stride)
		w.line(2, "%d,", len(data))
		w.line(2, "%s,", mapName)
		w.line(2, "NULL,")
		w.line(2, "NULL,")
		w.line(0, "};")
	}
	return w.String()
}
