package newspaper

import (
	"fmt"
	"io/fs"
	"unicode"

	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"
)

// Select a complete font pair so mixed text and headings keep every character.
func pdfFonts(text string, resources fs.FS) ([]byte, []byte, error) {
	var unsupported rune
	for attempt := 0; attempt < 2; attempt++ {
		pair := [2][]byte{goregular.TTF, gobold.TTF}
		if attempt == 1 {
			for i, name := range []string{"NotoSansSC-Regular.ttf", "NotoSansSC-Bold.ttf"} {
				data, err := fs.ReadFile(resources, name)
				if err != nil {
					return nil, nil, fmt.Errorf("load Newspaper PDF font %s: %w", name, err)
				}
				pair[i] = data
			}
		}
		covered := true
		for _, data := range pair {
			font, err := sfnt.Parse(data)
			if err != nil {
				return nil, nil, fmt.Errorf("parse Newspaper PDF font: %w", err)
			}
			for _, char := range text {
				if unicode.IsSpace(char) {
					continue
				}
				// gofpdf uses BMP glyphs and does not shape complex scripts.
				glyph, err := font.GlyphIndex(nil, char)
				if err != nil || glyph == 0 || char > 0xffff || unicode.In(char, unicode.Arabic, unicode.Devanagari, unicode.Hebrew) {
					unsupported, covered = char, false
					break
				}
			}
			if !covered {
				break
			}
		}
		if covered {
			return pair[0], pair[1], nil
		}
	}
	return nil, nil, fmt.Errorf("PDF font cannot render U+%04X", unsupported)
}
