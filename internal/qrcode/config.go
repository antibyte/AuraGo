package qrcode

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"io"
	"math"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrVerification = errors.New("QR verification failed; choose a safer presentation or a larger size")
var ErrBatchInvalid = errors.New("CSV contains invalid rows; no archive was produced")

type ValidationError struct{ Field, Code, Message string }

func (e *ValidationError) Error() string        { return e.Field + ": " + e.Message }
func invalid(field, code, message string) error { return &ValidationError{field, code, message} }
func warning(code, message string) Diagnostic   { return Diagnostic{code, "warning", message} }
func oneOf(s string, values ...string) bool {
	for _, v := range values {
		if s == v {
			return true
		}
	}
	return false
}
func defaultString(s *string, v string) {
	if *s == "" {
		*s = v
	}
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// Normalize validates the complete spec and strips image metadata by re-encoding
// all imported resources. It does not change the caller's struct or byte slices.
func Normalize(ctx context.Context, s QRSpec) (QRSpec, []Diagnostic, error) {
	if err := ctx.Err(); err != nil {
		return QRSpec{}, nil, err
	}
	if s.SchemaVersion == 0 {
		s.SchemaVersion = SchemaVersion
	}
	if s.SchemaVersion != SchemaVersion {
		return QRSpec{}, nil, invalid("schema_version", "unsupported_version", "unsupported QR schema version")
	}
	if s.Content.Calendar != nil {
		calendar := *s.Content.Calendar
		s.Content.Calendar = &calendar
		if calendar.CreatedAt == "" {
			calendar.CreatedAt = time.Now().UTC().Format(time.RFC3339)
		} else if stamp, err := time.Parse(time.RFC3339, calendar.CreatedAt); err == nil {
			calendar.CreatedAt = stamp.UTC().Format(time.RFC3339)
		}
	}
	if _, err := Payload(s.Content); err != nil {
		return QRSpec{}, nil, err
	}
	st := &s.Style
	if st.Size == 0 {
		st.Size = 512
	}
	if st.Size < 64 || st.Size > MaxOutputDimension {
		return QRSpec{}, nil, invalid("style.size", "range", "size must be between 64 and 4096 pixels")
	}
	q := 4
	if st.QuietZone != nil {
		q = *st.QuietZone
	}
	if q < 0 || q > 32 {
		return QRSpec{}, nil, invalid("style.quiet_zone", "range", "quiet zone must be between 0 and 32 modules")
	}
	st.QuietZone = &q
	if s.Content.Type == "epc" {
		defaultString(&st.ErrorCorrection, "M")
	} else {
		defaultString(&st.ErrorCorrection, "H")
	}
	if !oneOf(st.ErrorCorrection, "L", "M", "Q", "H") {
		return QRSpec{}, nil, invalid("style.error_correction", "option", "unsupported error correction level")
	}
	if s.Content.Type == "epc" && st.ErrorCorrection != "M" {
		return QRSpec{}, nil, invalid("style.error_correction", "epc_correction", "EPC requires correction level M")
	}
	defaultString(&st.ModuleShape, "square")
	defaultString(&st.FinderShape, "square")
	defaultString(&st.CenterShape, "square")
	if !oneOf(st.ModuleShape, "square", "rounded", "extra-rounded", "classy", "classy-rounded", "dots") {
		return QRSpec{}, nil, invalid("style.module_shape", "option", "unsupported module shape")
	}
	if !oneOf(st.FinderShape, "square", "rounded", "extra-rounded", "dot") {
		return QRSpec{}, nil, invalid("style.finder_shape", "option", "unsupported finder shape")
	}
	if !oneOf(st.CenterShape, "square", "rounded", "dot") {
		return QRSpec{}, nil, invalid("style.center_shape", "option", "unsupported finder center shape")
	}
	for _, p := range []struct {
		v          *string
		def, field string
	}{{&st.ModuleColor, "#000000", "style.module_color"}, {&st.FinderColor, "#000000", "style.finder_color"}, {&st.CenterColor, "#000000", "style.center_color"}, {&st.BackgroundColor, "#ffffff", "style.background_color"}, {&s.Output.JPEGBackground, "#ffffff", "output.jpeg_background"}} {
		defaultString(p.v, p.def)
		v, _, err := parseColor(*p.v)
		if err != nil {
			return QRSpec{}, nil, invalid(p.field, "color", "use a hexadecimal RGB or RGBA color")
		}
		*p.v = v
	}
	if !finite(st.BackgroundRadius) || st.BackgroundRadius < 0 || st.BackgroundRadius > float64(st.Size)/2 {
		return QRSpec{}, nil, invalid("style.background_radius", "range", "background radius exceeds QR bounds")
	}
	defaultString(&s.Output.Format, "png")
	defaultString(&s.Output.TextMode, "unicode_half")
	if s.Output.Quality == 0 {
		s.Output.Quality = 90
	}
	if !oneOf(s.Output.Format, "png", "jpeg", "svg", "ascii", "unicode", "unicode_half", "txt", "markdown") {
		return QRSpec{}, nil, invalid("output.format", "option", "unsupported output format")
	}
	if !oneOf(s.Output.TextMode, "ascii", "unicode", "unicode_half") {
		return QRSpec{}, nil, invalid("output.text_mode", "option", "unsupported text representation")
	}
	if s.Output.Quality < 30 || s.Output.Quality > 100 {
		return QRSpec{}, nil, invalid("output.quality", "range", "JPEG quality must be between 30 and 100")
	}
	_, jc, _ := parseColor(s.Output.JPEGBackground)
	if jc.A != 255 {
		return QRSpec{}, nil, invalid("output.jpeg_background", "alpha", "JPEG background must be opaque")
	}
	diagnostics := []Diagnostic{}
	numbers := []string{}
	if s.Content.Telephone != nil {
		numbers = append(numbers, s.Content.Telephone.Number)
	}
	if s.Content.SMS != nil {
		numbers = append(numbers, s.Content.SMS.Number)
	}
	if v := s.Content.VCard; v != nil {
		numbers = append(numbers, v.Phone, v.PhoneWork, v.PhonePrivate, v.PhoneMobile)
	}
	for _, number := range numbers {
		if number != "" {
			_, bareLocal, _ := phoneSubscriber(number)
			if bareLocal {
				diagnostics = append(diagnostics, warning("local_phone", "Local phone numbers depend on the scanner phone context; no country is inferred."))
				break
			}
		}
	}
	if q < 4 {
		diagnostics = append(diagnostics, warning("quiet_zone", "A quiet zone of at least four modules is recommended."))
	}
	_, bg, _ := parseColor(st.BackgroundColor)
	for _, v := range []string{st.ModuleColor, st.FinderColor, st.CenterColor} {
		_, fg, _ := parseColor(v)
		if contrast(fg, bg) < 4.5 {
			diagnostics = append(diagnostics, warning("low_contrast", "The QR colors have low contrast; use a high-contrast preset."))
			break
		}
	}
	if bg.A < 255 {
		diagnostics = append(diagnostics, warning("transparent_background", "Transparency is verified against white; the insertion surface can change readability."))
	}
	if s.Logo != nil {
		l := *s.Logo
		s.Logo = &l
		if l.SizePercent == 0 {
			l.SizePercent = 16
		}
		if !finite(l.SizePercent) || l.SizePercent < 1 || l.SizePercent > 30 || l.Margin < 0 || l.Margin > 128 {
			return QRSpec{}, nil, invalid("logo", "range", "logo size must be 1–30 percent with margin 0–128 pixels")
		}
		resource, err := NormalizeImage(ctx, l.Image.MIME, l.Image.Data)
		if err != nil {
			return QRSpec{}, nil, err
		}
		l.Image = resource
		*s.Logo = l
		if l.SizePercent > 20 {
			diagnostics = append(diagnostics, warning("logo_coverage", "A large logo can prevent scanning."))
		}
		if st.ErrorCorrection != "H" {
			diagnostics = append(diagnostics, warning("logo_correction", "High error correction is recommended when adding a logo."))
		}
	}
	if s.Frame != nil {
		f := *s.Frame
		s.Frame = &f
		defaultString(&f.Position, "bottom")
		defaultString(&f.Font, "sans")
		if !oneOf(f.Position, "top", "bottom", "left", "right") {
			return QRSpec{}, nil, invalid("frame.position", "option", "unsupported caption position")
		}
		if !validFontID(f.Font) {
			return QRSpec{}, nil, invalid("frame.font", "option", "unsupported local font")
		}
		if !utf8.ValidString(f.Text) || len(f.Text) > 8192 || utf8.RuneCountInString(f.Text) > 2048 || strings.ContainsRune(f.Text, 0) {
			return QRSpec{}, nil, invalid("frame.text", "limit", "caption is invalid or exceeds 2048 characters")
		}
		if f.FontSize == 0 {
			f.FontSize = 24
		}
		if f.SideWidth == 0 {
			f.SideWidth = 180
		}
		if f.FontSize < 8 || f.FontSize > 256 || f.SideWidth < 32 || f.SideWidth > 2048 || f.Padding < 0 || f.Padding > 512 || f.BorderWidth < 0 || f.BorderWidth > 128 || !finite(f.BorderRadius) || f.BorderRadius < 0 || f.BorderRadius > 2048 {
			return QRSpec{}, nil, invalid("frame", "range", "frame dimensions exceed their supported range")
		}
		for _, p := range []struct {
			v   *string
			def string
		}{{&f.TextColor, "#000000"}, {&f.BackgroundColor, "#ffffff"}, {&f.BorderColor, "#000000"}} {
			defaultString(p.v, p.def)
			v, _, err := parseColor(*p.v)
			if err != nil {
				return QRSpec{}, nil, invalid("frame", "color", "use hexadecimal frame colors")
			}
			*p.v = v
		}
		if f.BackgroundImage != nil {
			r, err := NormalizeImage(ctx, f.BackgroundImage.MIME, f.BackgroundImage.Data)
			if err != nil {
				return QRSpec{}, nil, err
			}
			f.BackgroundImage = &r
		}
		*s.Frame = f
	}
	if err := ctx.Err(); err != nil {
		return QRSpec{}, nil, err
	}
	return s, diagnostics, nil
}

func parseColor(s string) (string, color.NRGBA, error) {
	switch strings.ToLower(s) {
	case "transparent":
		s = "#00000000"
	case "white":
		s = "#ffffff"
	case "black":
		s = "#000000"
	}
	if !strings.HasPrefix(s, "#") {
		return "", color.NRGBA{}, errors.New("color")
	}
	h := s[1:]
	if len(h) == 3 || len(h) == 4 {
		var b strings.Builder
		for _, c := range h {
			b.WriteRune(c)
			b.WriteRune(c)
		}
		h = b.String()
	}
	if len(h) != 6 && len(h) != 8 {
		return "", color.NRGBA{}, errors.New("color")
	}
	b, err := hex.DecodeString(h)
	if err != nil {
		return "", color.NRGBA{}, err
	}
	c := color.NRGBA{R: b[0], G: b[1], B: b[2], A: 255}
	if len(b) == 4 {
		c.A = b[3]
	}
	return "#" + strings.ToLower(h), c, nil
}
func luminance(c color.NRGBA) float64 {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	a := float64(c.A) / 255
	linear := func(v float64) float64 {
		v = v*a + 1 - a
		if v <= .04045 {
			return v / 12.92
		}
		return math.Pow((v+.055)/1.055, 2.4)
	}
	return .2126*linear(r) + .7152*linear(g) + .0722*linear(b)
}
func contrast(a, b color.NRGBA) float64 {
	x, y := luminance(a), luminance(b)
	return (math.Max(x, y) + .05) / (math.Min(x, y) + .05)
}

// ParseConfig rejects unknown/duplicate properties and excessive JSON depth.
func ParseConfig(ctx context.Context, data []byte) (QRSpec, error) {
	var s QRSpec
	if len(data) == 0 || len(data) > MaxResourceBytes || !utf8.Valid(data) {
		return s, invalid("config", "limit", "configuration exceeds 10 MiB")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	tokens := 0
	if err := checkJSONValue(d, 0, &tokens); err != nil {
		return s, invalid("config", "json", "configuration must be bounded JSON without duplicate properties")
	}
	if _, err := d.Token(); err != io.EOF {
		return s, invalid("config", "json", "unexpected trailing JSON")
	}
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&s); err != nil {
		return QRSpec{}, invalid("config", "schema", "configuration contains an unknown field or invalid value")
	}
	if s.SchemaVersion != SchemaVersion {
		return QRSpec{}, invalid("schema_version", "unsupported_version", "configuration must declare schema version 1")
	}
	s, _, err := Normalize(ctx, s)
	return s, err
}
func checkJSONValue(d *json.Decoder, depth int, count *int) error {
	*count++
	if depth > 16 || *count > 20000 {
		return errors.New("JSON limit")
	}
	t, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			s, ok := key.(string)
			if !ok || seen[s] {
				return errors.New("duplicate key")
			}
			seen[s] = true
			if err := checkJSONValue(d, depth+1, count); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := checkJSONValue(d, depth+1, count); err != nil {
				return err
			}
		}
	default:
		return errors.New("unexpected delimiter")
	}
	_, err = d.Token()
	return err
}
func ExportConfig(ctx context.Context, s QRSpec) ([]byte, error) {
	s, _, err := Normalize(ctx, s)
	if err != nil {
		return nil, err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode QR configuration: %w", err)
	}
	if len(b) > MaxResourceBytes {
		return nil, invalid("config", "limit", "configuration exceeds 10 MiB")
	}
	return append(b, '\n'), nil
}

func validFontID(id string) bool {
	return oneOf(id, "sans", "serif", "mono", "noto-arabic", "noto-devanagari", "noto-cjk", "noto-emoji")
}
func Presets() Catalog {
	q := 4
	base := Style{Size: 512, QuietZone: &q, ErrorCorrection: "H", ModuleShape: "square", FinderShape: "square", CenterShape: "square", ModuleColor: "#000000", FinderColor: "#000000", CenterColor: "#000000", BackgroundColor: "#ffffff"}
	classic := base
	soft := base
	soft.ModuleShape = "rounded"
	soft.FinderShape = "rounded"
	soft.CenterShape = "rounded"
	soft.ModuleColor = "#17324d"
	soft.FinderColor = "#17324d"
	soft.CenterColor = "#17324d"
	forest := soft
	forest.ModuleShape = "extra-rounded"
	forest.FinderShape = "extra-rounded"
	forest.ModuleColor = "#163c2b"
	forest.FinderColor = "#163c2b"
	forest.CenterColor = "#163c2b"
	dots := base
	dots.ModuleShape = "dots"
	dots.FinderShape = "dot"
	dots.CenterShape = "dot"
	label := Frame{Position: "bottom", Font: "sans", FontSize: 24, SideWidth: 180, Padding: 16, TextColor: "#000000", BackgroundColor: "#ffffff", BorderColor: "#000000", BorderWidth: 2, BorderRadius: 12}
	sidebar := label
	sidebar.Position = "left"
	sidebar.BorderRadius = 0
	return Catalog{SchemaVersion: SchemaVersion, ContentTypes: []string{"text", "url", "email", "telephone", "sms", "wifi", "vcard", "location", "calendar", "epc"}, ModuleShapes: []string{"square", "rounded", "extra-rounded", "classy", "classy-rounded", "dots"}, FinderShapes: []string{"square", "rounded", "extra-rounded", "dot"}, CenterShapes: []string{"square", "rounded", "dot"}, FramePositions: []string{"top", "bottom", "left", "right"}, OutputFormats: []string{"png", "jpeg", "svg", "ascii", "unicode", "unicode_half", "txt", "markdown"}, Fonts: []FontInfo{{"sans", "Go Sans", ""}, {"serif", "Liberation Serif", "LiberationSerif-Regular.ttf"}, {"mono", "Go Mono", ""}, {"noto-arabic", "Noto Sans Arabic", "NotoSansArabic.ttf"}, {"noto-devanagari", "Noto Sans Devanagari", "NotoSansDevanagari.ttf"}, {"noto-cjk", "Noto Sans CJK", "NotoSansCJKsc.otf"}, {"noto-emoji", "Noto Emoji", "NotoEmoji.ttf"}}, Presets: []Preset{{"classic", "Classic", Presentation{Style: classic}}, {"soft", "Soft", Presentation{Style: soft}}, {"forest", "Forest", Presentation{Style: forest}}, {"dots", "Dots", Presentation{Style: dots}}, {"label", "Label", Presentation{Style: base, Frame: &label}}, {"sidebar", "Sidebar", Presentation{Style: base, Frame: &sidebar}}}}
}
func RandomPresentation() (Presentation, error) {
	c := Presets()
	i, err := rand.Int(rand.Reader, big.NewInt(int64(len(c.Presets))))
	if err != nil {
		return Presentation{}, fmt.Errorf("choose QR presentation: %w", err)
	}
	p := c.Presets[i.Int64()].Presentation
	shape, err := rand.Int(rand.Reader, big.NewInt(int64(len(c.ModuleShapes))))
	if err != nil {
		return Presentation{}, err
	}
	p.Style.ModuleShape = c.ModuleShapes[shape.Int64()]
	return p, nil
}
