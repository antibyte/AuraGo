package cyd

import (
	"bytes"
	"fmt"
	"image/png"
)

// PortraitSide is the RGB565 sprite the Cheap Yellow Display draws on Home.
// The source is the same /img/personas/<key>.png AgoDesk receives as avatar_image_url.
const PortraitSide = 72

// EncodePortraitRGB565 scales a PNG to side×side and returns little-endian RGB565.
func EncodePortraitRGB565(pngData []byte, side int) ([]byte, error) {
	if side < 8 || side > 96 {
		return nil, fmt.Errorf("portrait side %d out of range", side)
	}
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	if b.Dx() < 1 || b.Dy() < 1 {
		return nil, fmt.Errorf("portrait is empty")
	}
	out := make([]byte, side*side*2)
	for y := 0; y < side; y++ {
		y0 := b.Min.Y + (y*b.Dy())/side
		y1 := b.Min.Y + ((y+1)*b.Dy())/side
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < side; x++ {
			x0 := b.Min.X + (x*b.Dx())/side
			x1 := b.Min.X + ((x+1)*b.Dx())/side
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var rs, gs, bs, n uint32
			for sy := y0; sy < y1 && sy < b.Max.Y; sy++ {
				for sx := x0; sx < x1 && sx < b.Max.X; sx++ {
					r, g, bl, _ := img.At(sx, sy).RGBA()
					rs += r
					gs += g
					bs += bl
					n++
				}
			}
			if n == 0 {
				continue
			}
			v := uint16((rs/n>>11)<<11 | (gs/n>>10)<<5 | (bs / n >> 11))
			i := (y*side + x) * 2
			out[i] = byte(v)
			out[i+1] = byte(v >> 8)
		}
	}
	return out, nil
}
