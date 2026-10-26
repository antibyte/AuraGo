package qrcode

import (
	"context"
	"image"
	"image/color"
	"time"
	"unicode/utf8"

	"github.com/makiuchi-d/gozxing"
	zxqr "github.com/makiuchi-d/gozxing/qrcode"
)

type contextImage struct {
	image.Image
	ctx   context.Context
	reads uint32
}

func (im *contextImage) At(x, y int) color.Color {
	im.reads++
	if im.reads&65535 == 0 {
		if err := im.ctx.Err(); err != nil {
			panic(err)
		}
	}
	return im.Image.At(x, y)
}
func decodeQRImage(ctx context.Context, im image.Image) (text string, err error) {
	defer func() {
		if recover() != nil {
			text = ""
			if e := ctx.Err(); e != nil {
				err = e
			} else {
				err = invalid("image", "no_qr", "no readable QR code was found")
			}
		}
	}()
	if e := ctx.Err(); e != nil {
		return "", e
	}
	bitmap, e := gozxing.NewBinaryBitmapFromImage(&contextImage{Image: flatten(im, color.NRGBA{R: 255, G: 255, B: 255, A: 255}), ctx: ctx})
	if e != nil {
		return "", invalid("image", "decode", "image could not be prepared for scanning")
	}
	var result *gozxing.Result
	// The general detector can reject dense, axis-aligned symbols; the upstream
	// pure-symbol path still decodes and checks the actual image pixels.
	for _, hints := range []map[gozxing.DecodeHintType]interface{}{{gozxing.DecodeHintType_TRY_HARDER: true}, {gozxing.DecodeHintType_PURE_BARCODE: true}} {
		if e := ctx.Err(); e != nil {
			return "", e
		}
		result, e = readQRBitmap(bitmap, hints)
		if e == nil {
			break
		}
	}
	if e != nil {
		return "", invalid("image", "no_qr", "no readable QR code was found")
	}
	if e = ctx.Err(); e != nil {
		return "", e
	}
	text = result.GetText()
	if len(text) > MaxPayloadBytes || !utf8.ValidString(text) {
		return "", invalid("image", "payload", "decoded payload exceeds supported text bounds")
	}
	return text, nil
}
func verifyImage(ctx context.Context, im image.Image, payload string) error {
	text, e := decodeQRImage(ctx, im)
	if e != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return ErrVerification
	}
	if text != payload {
		return ErrVerification
	}
	return nil
}

// Decode reads one QR code from bounded image bytes and never executes it.
func Decode(ctx context.Context, data []byte) (result ScanResult, err error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	defer func() {
		if recover() != nil {
			result = ScanResult{}
			if e := ctx.Err(); e != nil {
				err = e
			} else {
				err = invalid("image", "decode", "invalid image data")
			}
		}
	}()
	if len(data) == 0 || len(data) > MaxResourceBytes {
		return result, invalid("image", "limit", "scan image must contain 1 byte to 10 MiB")
	}
	if e := ctx.Err(); e != nil {
		return result, e
	}
	im, _, err := decodeRaster(ctx, data)
	if err != nil {
		return result, err
	}
	text, err := decodeQRImage(ctx, im)
	if err != nil {
		return result, err
	}
	return Classify(text), nil
}

func readQRBitmap(bitmap *gozxing.BinaryBitmap, hints map[gozxing.DecodeHintType]interface{}) (result *gozxing.Result, err error) {
	defer func() {
		if recover() != nil {
			result = nil
			err = invalid("image", "no_qr", "no readable QR code was found")
		}
	}()
	return zxqr.NewQRCodeReader().Decode(bitmap, hints)
}
