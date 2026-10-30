package decisions

import (
	"bytes"
	"encoding/binary"
	"image"
)

func animatedDecisionPNG(data []byte) bool {
	for offset := 8; offset+12 <= len(data); {
		size := uint64(binary.BigEndian.Uint32(data[offset:]))
		if size+12 > uint64(len(data)-offset) {
			return false
		}
		if string(data[offset+4:offset+8]) == "acTL" {
			return true
		}
		offset += int(size) + 12
	}
	return false
}

// Read only IFD0's bounded orientation tag; never follow thumbnail/data pointers.
func jpegOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xff || data[1] != 0xd8 {
		return 1
	}
	for offset := 2; offset+4 <= len(data); {
		if data[offset] != 0xff {
			return 1
		}
		marker := data[offset+1]
		if marker == 0xda || marker == 0xd9 {
			return 1
		}
		if marker == 0xff {
			offset++
			continue
		}
		size := int(binary.BigEndian.Uint16(data[offset+2:]))
		if size < 2 || size > len(data)-offset-2 {
			return 1
		}
		payload := data[offset+4 : offset+2+size]
		offset += size + 2
		if marker != 0xe1 || !bytes.HasPrefix(payload, []byte("Exif\x00\x00")) {
			continue
		}
		tiff := payload[6:]
		if len(tiff) < 8 {
			return 1
		}
		var order binary.ByteOrder
		switch string(tiff[:2]) {
		case "II":
			order = binary.LittleEndian
		case "MM":
			order = binary.BigEndian
		default:
			return 1
		}
		if order.Uint16(tiff[2:]) != 42 {
			return 1
		}
		start := uint64(order.Uint32(tiff[4:]))
		if start+2 > uint64(len(tiff)) {
			return 1
		}
		count := uint64(order.Uint16(tiff[start:]))
		if start+2+count*12 > uint64(len(tiff)) {
			return 1
		}
		for i := uint64(0); i < count; i++ {
			entry := tiff[start+2+i*12:]
			if order.Uint16(entry) == 0x112 && order.Uint16(entry[2:]) == 3 && order.Uint32(entry[4:]) == 1 {
				orientation := int(order.Uint16(entry[8:]))
				if orientation >= 1 && orientation <= 8 {
					return orientation
				}
			}
		}
	}
	return 1
}

func orientDecisionImage(source image.Image, orientation int) image.Image {
	if orientation == 1 {
		return source
	}
	bounds := source.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	width, height := w, h
	if orientation >= 5 {
		width, height = h, w
	}
	result := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			sx, sy := x, y
			switch orientation {
			case 2:
				sx = w - 1 - x
			case 3:
				sx, sy = w-1-x, h-1-y
			case 4:
				sy = h - 1 - y
			case 5:
				sx, sy = y, x
			case 6:
				sx, sy = y, h-1-x
			case 7:
				sx, sy = w-1-y, h-1-x
			case 8:
				sx, sy = w-1-y, x
			}
			result.Set(x, y, source.At(bounds.Min.X+sx, bounds.Min.Y+sy))
		}
	}
	return result
}

// OrientImage applies the EXIF orientation found in the original JPEG bytes,
// so a caller that re-encodes the picture does not lose it.
func OrientImage(source image.Image, original []byte) image.Image {
	return orientDecisionImage(source, jpegOrientation(original))
}
