package render

import (
	"bytes"
	"encoding/binary"
	"errors"
)

// Frame transports (storymaker.render.transport).
const (
	TransportPNG        = "png"
	TransportWebP       = "webp"
	TransportRaw        = "raw"
	TransportScreenshot = "screenshot"
)

// Frame headers. The ingest checks them before a frame reaches FFmpeg; the tests read them too.
var (
	pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	pngTrailer   = []byte{0, 0, 0, 0, 'I', 'E', 'N', 'D', 0xae, 0x42, 0x60, 0x82}
)

// pngSize reads the IHDR and checks that the file is complete (it ends with the IEND chunk), so a
// truncated body never desynchronises the image2pipe demuxer.
func pngSize(data []byte) (int, int, error) {
	if len(data) < 33+len(pngTrailer) || !bytes.Equal(data[:8], pngSignature) {
		return 0, 0, errors.New("the frame is not a PNG")
	}
	if binary.BigEndian.Uint32(data[8:12]) != 13 || string(data[12:16]) != "IHDR" {
		return 0, 0, errors.New("the PNG has no IHDR chunk first")
	}
	if !bytes.Equal(data[len(data)-len(pngTrailer):], pngTrailer) {
		return 0, 0, errors.New("the PNG is truncated")
	}
	return int(binary.BigEndian.Uint32(data[16:20])), int(binary.BigEndian.Uint32(data[20:24])), nil
}

// webpSize reads the canvas size of a RIFF WebP (lossy VP8, lossless VP8L or extended VP8X) and checks
// that the RIFF length matches the body.
func webpSize(data []byte) (int, int, error) {
	if len(data) < 30 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return 0, 0, errors.New("the frame is not a WebP")
	}
	if riff := int(binary.LittleEndian.Uint32(data[4:8])); riff+8 != len(data) && riff+9 != len(data) {
		return 0, 0, errors.New("the WebP is truncated")
	}
	switch string(data[12:16]) {
	case "VP8X":
		w := 1 + (int(data[24]) | int(data[25])<<8 | int(data[26])<<16)
		h := 1 + (int(data[27]) | int(data[28])<<8 | int(data[29])<<16)
		return w, h, nil
	case "VP8L":
		if data[20] != 0x2f {
			return 0, 0, errors.New("the WebP lossless header is invalid")
		}
		bits := binary.LittleEndian.Uint32(data[21:25])
		return int(bits&0x3fff) + 1, int((bits>>14)&0x3fff) + 1, nil
	case "VP8 ":
		if data[23] != 0x9d || data[24] != 0x01 || data[25] != 0x2a {
			return 0, 0, errors.New("the WebP lossy header is invalid")
		}
		return int(binary.LittleEndian.Uint16(data[26:28]) & 0x3fff), int(binary.LittleEndian.Uint16(data[28:30]) & 0x3fff), nil
	}
	return 0, 0, errors.New("the WebP has an unknown first chunk")
}
