package atlaspdf

import (
	"strconv"
	"strings"
	"unicode"

	"aurago/internal/infra"

	"golang.org/x/image/font/sfnt"
)

type rgb struct{ R, G, B int }

func hexColor(v int) rgb { return rgb{v >> 16 & 255, v >> 8 & 255, v & 255} }

// mix blends a towards b by t (0..1).
func mix(a, b rgb, t float64) rgb {
	f := func(x, y int) int { return int(float64(x)*(1-t) + float64(y)*t + 0.5) }
	return rgb{f(a.R, b.R), f(a.G, b.G), f(a.B, b.B)}
}

var (
	colWhite  = hexColor(0xFFFFFF)
	colInk    = hexColor(0x111827)
	colText2  = hexColor(0x4B5563)
	colText3  = hexColor(0x8A93A3)
	colHair   = hexColor(0xD9DEE7)
	colAccent = hexColor(0x4F46E5)
	colGrid   = hexColor(0xE3E7EE)
	colShadow = hexColor(0xEEF0F4)
	colZebra  = hexColor(0xF8FAFC)
	colHead   = hexColor(0xEEF1F6)
	colGroup  = hexColor(0xF3F5F9)
)

func kindColor(k infra.NodeKind) rgb {
	switch k {
	case infra.NodeVM:
		return hexColor(0x7C3AED)
	case infra.NodeContainer:
		return hexColor(0x0891B2)
	case infra.NodeService:
		return hexColor(0xD97706)
	case infra.NodeNetwork:
		return hexColor(0x059669)
	}
	return hexColor(0x4F46E5)
}

type regionStyle struct {
	fill, stroke rgb
	dashed       bool
}

func regionStyleOf(k infra.NodeKind) regionStyle {
	switch k {
	case infra.NodeRoom:
		return regionStyle{hexColor(0xEEF6FF), hexColor(0xB9D4F2), false}
	case infra.NodeRack:
		return regionStyle{hexColor(0xF6F4F0), hexColor(0xD6CFC2), false}
	case infra.NodeGroup:
		return regionStyle{hexColor(0xFFFFFF), hexColor(0xC9CFDA), true}
	}
	return regionStyle{hexColor(0xF5F7FB), hexColor(0xCBD3E1), false}
}

func powerColor(p string) rgb {
	switch p {
	case infra.PowerRunning:
		return hexColor(0x16A34A)
	case infra.PowerPaused:
		return hexColor(0xF59E0B)
	case infra.PowerStopped:
		return hexColor(0x9CA3AF)
	}
	return hexColor(0xD1D5DB)
}

type edgeStyle struct {
	col  rgb
	wPt  float64
	dash []float64 // mm; nil is solid
}

func edgeStyleOf(k infra.EdgeKind) edgeStyle {
	switch k {
	case infra.EdgeEthernet:
		return edgeStyle{hexColor(0x64748B), 0.45, nil}
	case infra.EdgeFiber:
		return edgeStyle{hexColor(0xEA580C), 0.6, nil}
	case infra.EdgeWiFi:
		return edgeStyle{hexColor(0x0EA5E9), 0.45, []float64{1.4, 1.0}}
	case infra.EdgeDependsOn:
		return edgeStyle{hexColor(0x9CA3AF), 0.35, []float64{0.3, 0.8}}
	case infra.EdgePoweredBy:
		return edgeStyle{hexColor(0xDC2626), 0.35, []float64{0.8, 0.8}}
	case infra.EdgeMemberOf:
		return edgeStyle{hexColor(0xCBD5E1), 0.3, nil}
	}
	return edgeStyle{hexColor(0x94A3B8), 0.35, nil}
}

// coverage reports which runes a font set can print.
type coverage struct {
	fonts []*sfnt.Font
	buf   sfnt.Buffer
	cache map[rune]bool
}

func newCoverage(data ...[]byte) (*coverage, error) {
	c := &coverage{cache: map[rune]bool{}}
	for _, d := range data {
		f, err := sfnt.Parse(d)
		if err != nil {
			return nil, err
		}
		c.fonts = append(c.fonts, f)
	}
	return c, nil
}

// has is true when every font has a glyph for r. gofpdf prints BMP glyphs
// unshaped, so complex scripts count as uncovered.
func (c *coverage) has(r rune) bool {
	if unicode.IsSpace(r) {
		return true
	}
	if v, ok := c.cache[r]; ok {
		return v
	}
	ok := r <= 0xFFFF && !unicode.In(r, unicode.Arabic, unicode.Devanagari, unicode.Hebrew, unicode.Thai, unicode.Bengali, unicode.Tamil)
	for _, f := range c.fonts {
		if !ok {
			break
		}
		g, err := f.GlyphIndex(&c.buf, r)
		ok = err == nil && g != 0
	}
	c.cache[r] = ok
	return ok
}

func (c *coverage) all(s string) bool {
	for _, r := range s {
		if !c.has(r) {
			return false
		}
	}
	return true
}

// clean makes s printable: whitespace is collapsed, control characters dropped
// and uncovered runes become '?'.
func (c *coverage) clean(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsSpace(r):
			b.WriteByte(' ')
		case unicode.IsControl(r):
		case c.has(r):
			b.WriteRune(r)
		default:
			b.WriteByte('?')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func itoa(n int) string { return strconv.Itoa(n) }
