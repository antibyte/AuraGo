package qrcode

import (
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"math"
	"strings"
	"time"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode/decoder"
	"github.com/makiuchi-d/gozxing/qrcode/encoder"
	xdraw "golang.org/x/image/draw"
)

var renderAdmission = make(chan struct{}, 2)

type sceneItem struct {
	path geometryPath
	fill color.NRGBA
	im   image.Image
	raw  []byte
	rect image.Rectangle
}
type qrScene struct {
	width, height int
	items         []sceneItem
}

func (s *qrScene) add(p geometryPath, c color.NRGBA) {
	if len(p) > 0 && c.A > 0 {
		s.items = append(s.items, sceneItem{path: p, fill: c})
	}
}
func (s *qrScene) addImage(im image.Image, raw []byte, rect image.Rectangle) {
	s.items = append(s.items, sceneItem{im: im, raw: raw, rect: rect})
}
func (s qrScene) raster(ctx context.Context) (*image.NRGBA, error) {
	dst := image.NewNRGBA(image.Rect(0, 0, s.width, s.height))
	for _, item := range s.items {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if item.im != nil {
			xdraw.ApproxBiLinear.Scale(dst, item.rect, item.im, item.im.Bounds(), draw.Over, nil)
		} else {
			if err := paintPath(ctx, dst, item.path, item.fill); err != nil {
				return nil, err
			}
		}
	}
	return dst, nil
}
func (s qrScene) svg() ([]byte, error) {
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, s.width, s.height, s.width, s.height)
	for _, item := range s.items {
		if item.im != nil {
			fmt.Fprintf(&b, `<image x="%d" y="%d" width="%d" height="%d" href="data:image/png;base64,%s"/>`, item.rect.Min.X, item.rect.Min.Y, item.rect.Dx(), item.rect.Dy(), base64.StdEncoding.EncodeToString(item.raw))
		} else {
			fmt.Fprintf(&b, `<path fill="%s" fill-opacity="%.6f" d="%s"/>`, svgColor(item.fill), float64(item.fill.A)/255, item.path.svg())
		}
		if b.Len() > 32<<20 {
			return nil, invalid("output", "limit", "SVG exceeds 32 MiB")
		}
	}
	b.WriteString("</svg>\n")
	return []byte(b.String()), nil
}
func colorValue(s string) color.NRGBA { _, c, _ := parseColor(s); return c }
func flatten(im image.Image, c color.NRGBA) *image.NRGBA {
	dst := image.NewNRGBA(im.Bounds())
	draw.Draw(dst, dst.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), im, im.Bounds().Min, draw.Over)
	return dst
}

// Render verifies the exact raster bytes, or the serialized SVG's rasterization,
// before returning an artifact. It never publishes a failed or partial result.
func Render(ctx context.Context, spec QRSpec, options ...RenderOptions) (result Result, err error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	defer func() {
		if recover() != nil {
			result = Result{}
			if e := ctx.Err(); e != nil {
				err = e
			} else {
				err = invalid("output", "render", "QR rendering failed on invalid data")
			}
		}
	}()
	select {
	case renderAdmission <- struct{}{}:
		defer func() { <-renderAdmission }()
	case <-ctx.Done():
		return result, ctx.Err()
	}
	if len(options) > 1 {
		return result, invalid("options", "option", "only one renderer options value is supported")
	}
	var opts RenderOptions
	if len(options) == 1 {
		opts = options[0]
	}
	s, diagnostics, err := Normalize(ctx, spec)
	if err != nil {
		return result, err
	}
	payload, err := Payload(s.Content)
	if err != nil {
		return result, err
	}
	ec, _ := decoder.ErrorCorrectionLevel_ValueOf(s.Style.ErrorCorrection)
	code, e := encoder.Encoder_encode(payload, ec, map[gozxing.EncodeHintType]interface{}{gozxing.EncodeHintType_CHARACTER_SET: "UTF-8"})
	if e != nil {
		return result, invalid("content", "capacity", "payload exceeds QR capacity at the requested correction level")
	}
	version := code.GetVersion().GetVersionNumber()
	if s.Content.Type == "epc" && version > 13 {
		return result, invalid("content.epc", "capacity", "EPC requires QR version 13 or lower")
	}
	matrix := code.GetMatrix()
	n := matrix.GetWidth()
	pitch := s.Style.Size / (n + 2**s.Style.QuietZone)
	if pitch < 1 {
		return result, invalid("style.size", "module_pitch", "output is too small for the selected payload and quiet zone")
	}
	if pitch < 3 {
		diagnostics = append(diagnostics, warning("module_pitch", "Modules are smaller than three pixels; a larger size is recommended."))
	}
	metadata := Metadata{QRVersion: version, Modules: n, ModulePitch: float64(pitch), Diagnostics: diagnostics}
	if oneOf(s.Output.Format, "ascii", "unicode", "unicode_half", "txt", "markdown") {
		mode := s.Output.Format
		if mode == "txt" || mode == "markdown" {
			mode = s.Output.TextMode
		}
		text := matrixText(matrix, *s.Style.QuietZone, mode)
		if s.Output.Format == "markdown" {
			text = "```text\n" + text + "```\n"
			metadata.MIME = "text/markdown; charset=utf-8"
			metadata.Extension = "md"
		} else {
			metadata.MIME = "text/plain; charset=utf-8"
			metadata.Extension = "txt"
		}
		if err := verifyMatrix(ctx, matrix, payload); err != nil {
			return result, err
		}
		metadata.Verified = true
		metadata.Width = n + 2**s.Style.QuietZone
		metadata.Height = metadata.Width
		return Result{Data: []byte(text), Metadata: metadata}, nil
	}
	scene, err := layoutScene(ctx, s, matrix, pitch, opts.FontProvider)
	if err != nil {
		return result, err
	}
	metadata.Width, metadata.Height = scene.width, scene.height
	im, err := scene.raster(ctx)
	if err != nil {
		return result, err
	}
	var raw []byte
	switch s.Output.Format {
	case "png":
		raw, err = encodePNG(im, 32<<20)
		metadata.MIME = "image/png"
		metadata.Extension = "png"
	case "jpeg":
		buf := &limitedBuffer{limit: 32 << 20}
		err = jpeg.Encode(buf, flatten(im, colorValue(s.Output.JPEGBackground)), &jpeg.Options{Quality: s.Output.Quality})
		raw = buf.Bytes()
		metadata.MIME = "image/jpeg"
		metadata.Extension = "jpg"
	case "svg":
		raw, err = scene.svg()
		metadata.MIME = "image/svg+xml"
		metadata.Extension = "svg"
	}
	if err != nil {
		return result, invalid("output", "limit", "encoded artifact exceeds its supported byte limit")
	}
	if e := ctx.Err(); e != nil {
		return result, e
	}
	var verificationImage image.Image
	if s.Output.Format == "svg" {
		verificationImage, err = rasterizeSVG(ctx, raw, true)
	} else {
		verificationImage, _, err = decodeRaster(ctx, raw)
	}
	if err != nil {
		return result, ErrVerification
	}
	if err = verifyImage(ctx, verificationImage, payload); err != nil {
		return result, err
	}
	metadata.Verified = true
	return Result{Data: raw, Metadata: metadata}, nil
}

func layoutScene(ctx context.Context, s QRSpec, matrix *encoder.ByteMatrix, pitch int, provider FontProvider) (qrScene, error) {
	scene := qrScene{width: s.Style.Size, height: s.Style.Size}
	var caption captionLayout
	var err error
	qrX, qrY := 0, 0
	captionX, captionY := 0.0, 0.0
	if s.Frame != nil {
		f := *s.Frame
		inset := f.Padding + f.BorderWidth
		available := s.Style.Size
		if oneOf(f.Position, "left", "right") {
			available = f.SideWidth
		}
		caption, err = layoutCaption(ctx, f, available, provider)
		if err != nil {
			return scene, err
		}
		scene.width += 2 * inset
		scene.height += 2 * inset
		qrX, qrY = inset, inset
		extra := 0
		if len(caption.lines) > 0 {
			if oneOf(f.Position, "top", "bottom") {
				extra = int(math.Ceil(caption.height)) + f.Padding
				scene.height += extra
				captionX = float64(inset)
				captionY = float64(inset)
				if f.Position == "top" {
					qrY += extra
				} else {
					captionY = float64(inset + s.Style.Size + f.Padding)
				}
			} else {
				extra = f.SideWidth + f.Padding
				scene.width += extra
				scene.height = max(scene.height, int(math.Ceil(caption.height))+2*inset)
				qrY = (scene.height - s.Style.Size) / 2
				captionY = (float64(scene.height) - caption.height) / 2
				captionX = float64(inset)
				if f.Position == "left" {
					qrX += extra
				} else {
					captionX = float64(inset + s.Style.Size + f.Padding)
				}
			}
		}
	}
	if scene.width > MaxOutputDimension || scene.height > MaxOutputDimension {
		return scene, invalid("frame", "output_bounds", "framed output must fit within 4096 by 4096 pixels")
	}
	if s.Frame != nil {
		f := s.Frame
		if f.BorderRadius > float64(min(scene.width, scene.height))/2 {
			return scene, invalid("frame.border_radius", "range", "frame border radius exceeds its output bounds")
		}
		scene.add(roundedRectangle(0, 0, float64(scene.width), float64(scene.height), uniformRadii(f.BorderRadius)), colorValue(f.BackgroundColor))
		if f.BackgroundImage != nil {
			im, _, e := decodeRaster(ctx, f.BackgroundImage.Data)
			if e != nil {
				return scene, e
			}
			dst := image.NewNRGBA(image.Rect(0, 0, scene.width, scene.height))
			xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), im, im.Bounds(), draw.Src, nil)
			mask := image.NewAlpha(dst.Bounds())
			if err := paintPath(ctx, mask, roundedRectangle(0, 0, float64(scene.width), float64(scene.height), uniformRadii(f.BorderRadius)), color.NRGBA{A: 255}); err != nil {
				return scene, err
			}
			clipped := image.NewNRGBA(dst.Bounds())
			draw.DrawMask(clipped, clipped.Bounds(), dst, image.Point{}, mask, image.Point{}, draw.Src)
			raw, e := encodePNG(clipped, 32<<20)
			if e != nil {
				return scene, e
			}
			scene.addImage(clipped, raw, clipped.Bounds())
		}
		if f.BorderWidth > 0 {
			scene.add(pathRing(0, 0, float64(scene.width), float64(scene.height), float64(f.BorderWidth), f.BorderRadius), colorValue(f.BorderColor))
		}
	}
	scene.add(roundedRectangle(float64(qrX), float64(qrY), float64(s.Style.Size), float64(s.Style.Size), uniformRadii(s.Style.BackgroundRadius)), colorValue(s.Style.BackgroundColor))
	n := matrix.GetWidth()
	originX, originY := qrX+(s.Style.Size-n*pitch)/2, qrY+(s.Style.Size-n*pitch)/2
	p := float64(pitch)
	var logoRect image.Rectangle
	var logoImage image.Image
	var logoRaw []byte
	if s.Logo != nil {
		logoImage, _, err = decodeRaster(ctx, s.Logo.Image.Data)
		if err != nil {
			return scene, err
		}
		width := max(1, int(float64(n*pitch)*s.Logo.SizePercent/100))
		height := width
		bounds := logoImage.Bounds()
		if bounds.Dx() > bounds.Dy() {
			height = max(1, width*bounds.Dy()/bounds.Dx())
		} else {
			width = max(1, height*bounds.Dx()/bounds.Dy())
		}
		logoRect = image.Rect(originX+(n*pitch-width)/2, originY+(n*pitch-height)/2, originX+(n*pitch-width)/2+width, originY+(n*pitch-height)/2+height)
		logoRaw = s.Logo.Image.Data
	}
	logoClear := logoRect
	if s.Logo != nil {
		logoClear = logoClear.Inset(-s.Logo.Margin)
	}
	var modules geometryPath
	dark := func(x, y int) bool { return x >= 0 && y >= 0 && x < n && y < n && matrix.Get(x, y) == 1 }
	finder := func(x, y int) bool { return x < 7 && y < 7 || x >= n-7 && y < 7 || x < 7 && y >= n-7 }
	for y := 0; y < n; y++ {
		if y%16 == 0 {
			if e := ctx.Err(); e != nil {
				return scene, e
			}
		}
		for x := 0; x < n; x++ {
			if !dark(x, y) || finder(x, y) {
				continue
			}
			left, top := originX+x*pitch, originY+y*pitch
			rect := image.Rect(left, top, left+pitch, top+pitch)
			if s.Logo != nil && s.Logo.HideBackgroundModules && rect.Overlaps(logoClear) {
				continue
			}
			var path geometryPath
			switch s.Style.ModuleShape {
			case "square":
				path = rectangle(float64(left), float64(top), p, p)
			case "dots":
				path = circle(float64(left)+p/2, float64(top)+p/2, p*.5)
			default:
				r := uniformRadii(p * .5)
				if s.Style.ModuleShape == "rounded" {
					r = uniformRadii(p * .28)
				}
				if s.Style.ModuleShape == "classy" {
					r = [4]float64{p * .5, 0, p * .5, 0}
				}
				if s.Style.ModuleShape == "classy-rounded" {
					r = [4]float64{p * .5, p * .18, p * .5, p * .18}
				}
				if dark(x-1, y) || dark(x, y-1) {
					r[0] = 0
				}
				if dark(x+1, y) || dark(x, y-1) {
					r[1] = 0
				}
				if dark(x+1, y) || dark(x, y+1) {
					r[2] = 0
				}
				if dark(x-1, y) || dark(x, y+1) {
					r[3] = 0
				}
				path = roundedRectangle(float64(left), float64(top), p, p, r)
			}
			modules = append(modules, path...)
		}
	}
	scene.add(modules, colorValue(s.Style.ModuleColor))
	for _, pos := range [][2]int{{0, 0}, {n - 7, 0}, {0, n - 7}} {
		x, y := float64(originX+pos[0]*pitch), float64(originY+pos[1]*pitch)
		var ring geometryPath
		switch s.Style.FinderShape {
		case "dot":
			ring = circle(x+3.5*p, y+3.5*p, 3.5*p)
			ring = append(ring, mirrorX(circle(x+3.5*p, y+3.5*p, 2.5*p), x+3.5*p)...)
		case "rounded":
			ring = pathRing(x, y, 7*p, 7*p, p, p)
		case "extra-rounded":
			ring = pathRing(x, y, 7*p, 7*p, p, 2*p)
		default:
			ring = pathRing(x, y, 7*p, 7*p, p, 0)
		}
		scene.add(ring, colorValue(s.Style.FinderColor))
		var center geometryPath
		switch s.Style.CenterShape {
		case "dot":
			center = circle(x+3.5*p, y+3.5*p, 1.5*p)
		case "rounded":
			center = roundedRectangle(x+2*p, y+2*p, 3*p, 3*p, uniformRadii(p*.75))
		default:
			center = rectangle(x+2*p, y+2*p, 3*p, 3*p)
		}
		scene.add(center, colorValue(s.Style.CenterColor))
	}
	if s.Logo != nil {
		scene.addImage(logoImage, logoRaw, logoRect)
	}
	if s.Frame != nil {
		f := s.Frame
		available := float64(s.Style.Size)
		if oneOf(f.Position, "left", "right") {
			available = float64(f.SideWidth)
		}
		for _, line := range caption.lines {
			scene.add(line.path.translated(captionX+(available-line.width)/2, captionY), colorValue(f.TextColor))
			captionY += line.height
		}
	}
	return scene, nil
}

func matrixText(matrix *encoder.ByteMatrix, quiet int, mode string) string {
	n := matrix.GetWidth()
	size := n + 2*quiet
	dark := func(x, y int) bool {
		return x >= quiet && y >= quiet && x < n+quiet && y < n+quiet && matrix.Get(x-quiet, y-quiet) == 1
	}
	var b strings.Builder
	if mode == "unicode_half" {
		for y := 0; y < size; y += 2 {
			for x := 0; x < size; x++ {
				a, c := dark(x, y), dark(x, y+1)
				switch {
				case a && c:
					b.WriteRune('█')
				case a:
					b.WriteRune('▀')
				case c:
					b.WriteRune('▄')
				default:
					b.WriteByte(' ')
				}
			}
			b.WriteByte('\n')
		}
	} else {
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				if dark(x, y) {
					if mode == "ascii" {
						b.WriteString("##")
					} else {
						b.WriteString("██")
					}
				} else {
					b.WriteString("  ")
				}
			}
			b.WriteByte('\n')
		}
	}
	return b.String()
}
func verifyMatrix(ctx context.Context, m *encoder.ByteMatrix, payload string) error {
	size := (m.GetWidth() + 8) * 4
	im := image.NewNRGBA(image.Rect(0, 0, size, size))
	draw.Draw(im, im.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	for y := 0; y < m.GetHeight(); y++ {
		for x := 0; x < m.GetWidth(); x++ {
			if m.Get(x, y) == 1 {
				draw.Draw(im, image.Rect((x+4)*4, (y+4)*4, (x+5)*4, (y+5)*4), image.NewUniform(color.Black), image.Point{}, draw.Src)
			}
		}
	}
	return verifyImage(ctx, im, payload)
}
