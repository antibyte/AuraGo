package qrcode

import (
	"bytes"
	"context"
	"crypto/sha256"
	"math"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/math/fixed"
)

type cachedFont struct {
	sum  [32]byte
	font *font.Font
}

var fontCache = struct {
	sync.Mutex
	entries map[string]cachedFont
}{entries: map[string]cachedFont{}}

func loadFont(ctx context.Context, id string, provider FontProvider) (*font.Face, error) {
	var raw []byte
	switch id {
	case "sans":
		raw = goregular.TTF
	case "mono":
		raw = gomono.TTF
	default:
		if provider == nil {
			return nil, invalid("frame.font", "font_unavailable", "caption requires a verified local fallback font")
		}
		var err error
		raw, err = provider(ctx, id)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, invalid("frame.font", "font_unavailable", "verified local caption font is unavailable")
		}
	}
	if len(raw) == 0 || len(raw) > 24<<20 {
		return nil, invalid("frame.font", "font_limit", "font resource is empty or exceeds 24 MiB")
	}
	sum := sha256.Sum256(raw)
	fontCache.Lock()
	defer fontCache.Unlock()
	if item, ok := fontCache.entries[id]; ok && item.sum == sum {
		return font.NewFace(item.font), nil
	}
	face, err := font.ParseTTF(bytes.NewReader(raw))
	if err != nil {
		return nil, invalid("frame.font", "font_invalid", "local caption font could not be parsed")
	}
	fontCache.entries[id] = cachedFont{sum, face.Font}
	return face, nil
}

type captionFontMap []*font.Face

func (f captionFontMap) ResolveFace(r rune) *font.Face {
	for _, face := range f {
		if _, ok := face.NominalGlyph(r); ok {
			return face
		}
	}
	return f[0]
}
func fallbackFor(r rune) string {
	switch {
	case unicode.In(r, unicode.Arabic, unicode.Hebrew):
		return "noto-arabic"
	case unicode.In(r, unicode.Devanagari):
		return "noto-devanagari"
	case unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) || r >= 0x3000 && r <= 0x303f || r >= 0xff00 && r <= 0xffef:
		return "noto-cjk"
	default:
		return "noto-emoji"
	}
}
func captionFonts(ctx context.Context, text, id string, provider FontProvider) (captionFontMap, error) {
	face, err := loadFont(ctx, id, provider)
	if err != nil {
		return nil, err
	}
	faces := captionFontMap{face}
	seen := map[string]bool{id: true}
	if id != "sans" {
		face, err = loadFont(ctx, "sans", nil)
		if err != nil {
			return nil, err
		}
		faces = append(faces, face)
		seen["sans"] = true
	}
	for _, r := range text {
		if r == '\n' || r == '\r' || r == '\t' || unicode.Is(unicode.Cf, r) {
			continue
		}
		has := false
		for _, f := range faces {
			if _, ok := f.NominalGlyph(r); ok {
				has = true
				break
			}
		}
		if has {
			continue
		}
		fallback := fallbackFor(r)
		if !seen[fallback] {
			f, err := loadFont(ctx, fallback, provider)
			if err != nil {
				return nil, err
			}
			faces = append(faces, f)
			seen[fallback] = true
		}
		has = false
		for _, f := range faces {
			if _, ok := f.NominalGlyph(r); ok {
				has = true
				break
			}
		}
		if !has {
			return nil, invalid("frame.text", "missing_glyph", "caption contains a character not supported by the local font catalog")
		}
	}
	return faces, nil
}

type captionLine struct {
	path          geometryPath
	width, height float64
}
type captionLayout struct {
	lines         []captionLine
	width, height float64
}

func layoutCaption(ctx context.Context, f Frame, width int, provider FontProvider) (captionLayout, error) {
	var result captionLayout
	if f.Text == "" {
		return result, nil
	}
	faces, err := captionFonts(ctx, f.Text, f.Font, provider)
	if err != nil {
		return result, err
	}
	var shaper shaping.HarfbuzzShaper
	var segmenter shaping.Segmenter
	for _, paragraph := range strings.Split(strings.ReplaceAll(strings.ReplaceAll(f.Text, "\r\n", "\n"), "\r", "\n"), "\n") {
		if e := ctx.Err(); e != nil {
			return result, e
		}
		if paragraph == "" {
			h := float64(f.FontSize) * 1.4
			result.lines = append(result.lines, captionLine{height: h})
			result.height += h
			continue
		}
		runes := []rune(strings.ReplaceAll(paragraph, "\t", "    "))
		direction := di.DirectionLTR
		for _, r := range runes {
			if unicode.In(r, unicode.Arabic, unicode.Hebrew) {
				direction = di.DirectionRTL
				break
			}
			if unicode.IsLetter(r) {
				break
			}
		}
		runs := segmenter.Split(shaping.Input{Text: runes, RunStart: 0, RunEnd: len(runes), Direction: direction, Size: fixed.I(f.FontSize)}, faces)
		outputs := make([]shaping.Output, len(runs))
		for i, input := range runs {
			outputs[i] = shaper.Shape(input)
		}
		var wrapper shaping.LineWrapper
		lines, truncated := wrapper.WrapParagraph(shaping.WrapConfig{Direction: direction, TruncateAfterLines: 128}, width, runes, shaping.NewSliceIterator(outputs))
		if truncated != 0 {
			return result, invalid("frame.text", "caption_limit", "caption needs more than 128 lines")
		}
		for _, line := range lines {
			sort.SliceStable(line, func(i, j int) bool { return line[i].VisualIndex < line[j].VisualIndex })
			ascent, descent, gap := float64(f.FontSize), 0.0, 0.0
			for _, out := range line {
				ascent = math.Max(ascent, float64(out.LineBounds.Ascent)/64)
				descent = math.Min(descent, float64(out.LineBounds.Descent)/64)
				gap = math.Max(gap, float64(out.LineBounds.Gap)/64)
			}
			height := math.Ceil(ascent - descent + math.Max(gap, float64(f.FontSize)*.15))
			var p geometryPath
			advance := 0.0
			for _, out := range line {
				scale := float64(out.Size) / 64 / float64(out.Face.Upem())
				for _, glyph := range out.Glyphs {
					if glyph.GlyphID == 0 {
						return result, invalid("frame.text", "missing_glyph", "caption contains an unsupported glyph")
					}
					outline, ok := out.Face.GlyphDataOutline(glyph.GlyphID)
					if !ok {
						return result, invalid("frame.text", "glyph_outline", "caption glyph has no supported vector outline")
					}
					x, y := advance+float64(glyph.XOffset)/64, ascent-float64(glyph.YOffset)/64
					for _, seg := range outline.Segments {
						args := []float64{}
						for _, point := range seg.ArgsSlice() {
							args = append(args, x+float64(point.X)*scale, y-float64(point.Y)*scale)
						}
						switch seg.Op {
						case ot.SegmentOpMoveTo:
							p.add('M', args...)
						case ot.SegmentOpLineTo:
							p.add('L', args...)
						case ot.SegmentOpQuadTo:
							p.add('Q', args...)
						case ot.SegmentOpCubeTo:
							p.add('C', args...)
						}
					}
					advance += float64(glyph.XAdvance) / 64
				}
			}
			minx, miny, maxx, maxy := pathBounds(p)
			if len(p) == 0 {
				minx, miny, maxx, maxy = 0, 0, advance, height
			}
			shiftx, shifty := -math.Min(0, minx), -math.Min(0, miny)
			p = p.translated(shiftx, shifty)
			actualWidth := math.Max(advance+shiftx, maxx+shiftx)
			height = math.Max(height+shifty, maxy+shifty)
			if actualWidth > float64(width)+1 {
				return result, invalid("frame.text", "caption_width", "caption glyph is wider than the available frame")
			}
			result.lines = append(result.lines, captionLine{p, actualWidth, height})
			result.width = math.Max(result.width, actualWidth)
			result.height += height
			if len(result.lines) > 128 || result.height > MaxOutputDimension {
				return result, invalid("frame.text", "caption_limit", "caption exceeds output bounds")
			}
		}
	}
	return result, nil
}
