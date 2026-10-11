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

// decodeAsset decodes a PNG or JPEG after checking its header: other formats
// (even if another package registered a decoder) and pictures above
// maxWidgetSide pixels per side are rejected before the pixels are read.
func decodeAsset(data []byte) (image.Image, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, NewError(CodeBadRequest, "the image is not a readable PNG or JPEG file", nil)
	}
	if format != "png" && format != "jpeg" {
		return nil, NewError(CodeBadRequest, fmt.Sprintf("image format %q is not supported (PNG or JPEG only)", format),
			map[string]any{"format": format})
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxWidgetSide || cfg.Height > maxWidgetSide {
		return nil, NewError(CodeBadRequest, fmt.Sprintf("image size %dx%d must be within 1..%d pixels per side", cfg.Width, cfg.Height, maxWidgetSide),
			map[string]any{"width": cfg.Width, "height": cfg.Height, "limit": maxWidgetSide})
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

// decodeProjectAssets decodes every asset of the project from in.Assets.
// Missing data, an unreadable file or a size that differs from the layout
// are *Error CYD_BAD_REQUEST (the stored asset and the layout disagree).
func decodeProjectAssets(p Project, data map[string][]byte) ([]genImage, map[string]int64, error) {
	images := make([]genImage, 0, len(p.Assets))
	sizes := make(map[string]int64, len(p.Assets))
	for _, a := range p.Assets {
		raw, ok := data[a.ID]
		if !ok {
			return nil, nil, NewError(CodeBadRequest, fmt.Sprintf("asset %s has no image data", a.ID), map[string]any{"asset": a.ID})
		}
		img, err := decodeAsset(raw)
		if err != nil {
			var e *Error
			if errors.As(err, &e) {
				e.Details = map[string]any{"asset": a.ID}
				e.Message = "asset " + a.ID + ": " + e.Message
			}
			return nil, nil, err
		}
		b := img.Bounds()
		if b.Dx() != a.W || b.Dy() != a.H {
			return nil, nil, NewError(CodeBadRequest, fmt.Sprintf("asset %s is %dx%d but the layout says %dx%d", a.ID, b.Dx(), b.Dy(), a.W, a.H),
				map[string]any{"asset": a.ID})
		}
		alpha := imageHasAlpha(img)
		images = append(images, genImage{asset: a, img: img, alpha: alpha})
		sizes[a.ID] = imageDataSize(a.W, a.H, alpha)
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
		data := encodeImage(im.img, im.alpha)
		mapName := "ui__img_" + a.ID + "_map"
		w.line(0, "")
		w.line(0, "// %s: %s, %dx%d, %s, %d bytes", a.ID, cComment(a.File), a.W, a.H, format, len(data))
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
		w.line(2, "{LV_IMAGE_HEADER_MAGIC, %s, 0, %d, %d, %d, 0},", cf, a.W, a.H, a.W*2)
		w.line(2, "%d,", len(data))
		w.line(2, "%s,", mapName)
		w.line(2, "NULL,")
		w.line(2, "NULL,")
		w.line(0, "};")
	}
	return w.String()
}
