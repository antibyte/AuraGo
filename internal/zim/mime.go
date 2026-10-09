package zim

import "bytes"

const (
	maxMimeListBytes = 64 << 10
	maxMimeTypes     = 0xFFFD // 0xFFFD..0xFFFF are reserved dirent markers
)

// parseMimeList decodes the zero-terminated MIME type strings that follow
// the header; an empty string ends the list.
func parseMimeList(b []byte) ([]string, error) {
	var types []string
	for p := 0; ; {
		if p >= len(b) {
			return nil, errCorrupt("unterminated MIME type list")
		}
		if b[p] == 0 {
			return types, nil
		}
		n := bytes.IndexByte(b[p:], 0)
		if n < 0 {
			return nil, errCorrupt("unterminated MIME type list")
		}
		if len(types) == maxMimeTypes {
			return nil, errCorrupt("more than %d MIME types", maxMimeTypes)
		}
		types = append(types, string(b[p:p+n]))
		p += n + 1
	}
}
