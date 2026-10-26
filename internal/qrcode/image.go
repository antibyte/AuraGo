package qrcode

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

type limitedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, errors.New("output limit")
	}
	return b.Buffer.Write(p)
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.r.Read(p)
}
func imageDimensions(w, h int) bool {
	return w > 0 && h > 0 && w <= 32768 && h <= 32768 && int64(w)*int64(h) <= MaxInputPixels
}
func decodeRaster(ctx context.Context, data []byte) (image.Image, string, error) {
	cfg, format, e := image.DecodeConfig(contextReader{ctx, bytes.NewReader(data)})
	if e != nil || !imageDimensions(cfg.Width, cfg.Height) {
		return nil, "", invalid("image", "dimensions", "image format or dimensions are invalid; limit is 32 megapixels")
	}
	if !oneOf(format, "png", "jpeg", "gif", "webp") {
		return nil, "", invalid("image", "format", "supported raster formats are PNG, JPEG, GIF and WebP")
	}
	im, _, e := image.Decode(contextReader{ctx, bytes.NewReader(data)})
	if e != nil {
		return nil, "", invalid("image", "decode", "image could not be decoded")
	}
	if e = ctx.Err(); e != nil {
		return nil, "", e
	}
	return im, format, nil
}
func encodePNG(im image.Image, limit int) ([]byte, error) {
	b := &limitedBuffer{limit: limit}
	if e := png.Encode(b, im); e != nil {
		return nil, invalid("image", "limit", "normalized image exceeds its byte limit")
	}
	return b.Bytes(), nil
}

// NormalizeImage accepts known raster formats and a restricted static SVG profile.
// JPEG orientation is applied; animation uses its first frame and metadata is discarded.
func NormalizeImage(ctx context.Context, mime string, data []byte) (resource ImageResource, err error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	defer func() {
		if recover() != nil {
			resource = ImageResource{}
			err = invalid("image", "decode", "invalid image data")
		}
	}()
	if e := ctx.Err(); e != nil {
		return resource, e
	}
	if len(data) == 0 || len(data) > MaxResourceBytes {
		return resource, invalid("image", "limit", "image must contain 1 byte to 10 MiB")
	}
	mime = strings.ToLower(strings.TrimSpace(strings.Split(mime, ";")[0]))
	trimmed := bytes.TrimSpace(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}))
	var im image.Image
	orientation := 1
	if mime == "image/svg+xml" || len(trimmed) > 0 && trimmed[0] == '<' {
		if mime != "" && mime != "image/svg+xml" {
			return resource, invalid("image", "mime", "declared image type does not match its bytes")
		}
		im, err = rasterizeSVG(ctx, trimmed, false)
	} else {
		var format string
		im, format, err = decodeRaster(ctx, data)
		if err == nil && format == "jpeg" {
			orientation = jpegOrientation(data)
		}
		expected := map[string]string{"png": "image/png", "jpeg": "image/jpeg", "gif": "image/gif", "webp": "image/webp"}[format]
		if err == nil && mime != "" && mime != expected {
			return resource, invalid("image", "mime", "declared image type does not match its bytes")
		}
	}
	if err != nil {
		return resource, err
	}
	b := im.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > 2048 || h > 2048 {
		scale := math.Min(2048/float64(w), 2048/float64(h))
		w, h = max(1, int(float64(w)*scale)), max(1, int(float64(h)*scale))
		dst := image.NewNRGBA(image.Rect(0, 0, w, h))
		xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), im, b, draw.Src, nil)
		im = dst
	}
	if orientation != 1 {
		im, err = orientImage(ctx, im, orientation)
		if err != nil {
			return resource, err
		}
	}
	raw, e := encodePNG(im, MaxResourceBytes)
	if e != nil {
		return resource, e
	}
	if e = ctx.Err(); e != nil {
		return resource, e
	}
	return ImageResource{MIME: "image/png", Data: raw}, nil
}

type affine [6]float64

func identity() affine { return affine{1, 0, 0, 1, 0, 0} }
func (m affine) point(x, y float64) (float64, float64) {
	return m[0]*x + m[2]*y + m[4], m[1]*x + m[3]*y + m[5]
}
func (m affine) mul(n affine) affine {
	return affine{m[0]*n[0] + m[2]*n[1], m[1]*n[0] + m[3]*n[1], m[0]*n[2] + m[2]*n[3], m[1]*n[2] + m[3]*n[3], m[0]*n[4] + m[2]*n[5] + m[4], m[1]*n[4] + m[3]*n[5] + m[5]}
}
func transformPath(p geometryPath, m affine) geometryPath {
	q := make(geometryPath, len(p))
	copy(q, p)
	for i := range q {
		for j := 0; j < commandArgs(q[i].op); j += 2 {
			q[i].a[j], q[i].a[j+1] = m.point(q[i].a[j], q[i].a[j+1])
		}
	}
	return q
}

type numberScanner struct {
	s string
	i int
}

func (s *numberScanner) skip() {
	for s.i < len(s.s) && (s.s[s.i] == ',' || unicode.IsSpace(rune(s.s[s.i]))) {
		s.i++
	}
}
func (s *numberScanner) number() (float64, error) {
	s.skip()
	start := s.i
	if s.i < len(s.s) && (s.s[s.i] == '+' || s.s[s.i] == '-') {
		s.i++
	}
	digits := 0
	for s.i < len(s.s) && s.s[s.i] >= '0' && s.s[s.i] <= '9' {
		s.i++
		digits++
	}
	if s.i < len(s.s) && s.s[s.i] == '.' {
		s.i++
		for s.i < len(s.s) && s.s[s.i] >= '0' && s.s[s.i] <= '9' {
			s.i++
			digits++
		}
	}
	if digits == 0 {
		return 0, errors.New("number")
	}
	if s.i < len(s.s) && (s.s[s.i] == 'e' || s.s[s.i] == 'E') {
		s.i++
		if s.i < len(s.s) && (s.s[s.i] == '+' || s.s[s.i] == '-') {
			s.i++
		}
		exp := s.i
		for s.i < len(s.s) && s.s[s.i] >= '0' && s.s[s.i] <= '9' {
			s.i++
		}
		if exp == s.i {
			return 0, errors.New("exponent")
		}
	}
	v, e := strconv.ParseFloat(s.s[start:s.i], 64)
	if e != nil || !finite(v) || math.Abs(v) > 1e6 {
		return 0, errors.New("number range")
	}
	return v, nil
}
func numberList(s string, limit int) ([]float64, error) {
	scan := numberScanner{s: s}
	values := []float64{}
	for {
		scan.skip()
		if scan.i == len(s) {
			return values, nil
		}
		if len(values) >= limit {
			return nil, errors.New("number count")
		}
		v, e := scan.number()
		if e != nil {
			return nil, e
		}
		values = append(values, v)
	}
}
func svgLength(s string) (float64, error) {
	factor := 1.0
	for _, u := range []struct {
		s string
		f float64
	}{{"px", 1}, {"mm", 96 / 25.4}, {"cm", 96 / 2.54}, {"in", 96}, {"pt", 96 / 72}} {
		if strings.HasSuffix(s, u.s) {
			s = strings.TrimSuffix(s, u.s)
			factor = u.f
			break
		}
	}
	n, e := numberList(s, 1)
	if e != nil || len(n) != 1 {
		return 0, errors.New("length")
	}
	return n[0] * factor, nil
}
func parseTransform(s string) (affine, error) {
	m := identity()
	for strings.TrimSpace(s) != "" {
		s = strings.TrimSpace(s)
		k := strings.IndexByte(s, '(')
		end := strings.IndexByte(s, ')')
		if k < 1 || end < k {
			return m, errors.New("transform")
		}
		name := strings.TrimSpace(s[:k])
		v, e := numberList(s[k+1:end], 6)
		if e != nil {
			return m, e
		}
		n := identity()
		switch name {
		case "matrix":
			if len(v) != 6 {
				return m, errors.New("matrix")
			}
			copy(n[:], v)
		case "translate":
			if len(v) < 1 || len(v) > 2 {
				return m, errors.New("translate")
			}
			n[4] = v[0]
			if len(v) == 2 {
				n[5] = v[1]
			}
		case "scale":
			if len(v) < 1 || len(v) > 2 {
				return m, errors.New("scale")
			}
			n[0], n[3] = v[0], v[0]
			if len(v) == 2 {
				n[3] = v[1]
			}
		case "rotate":
			if len(v) != 1 && len(v) != 3 {
				return m, errors.New("rotate")
			}
			angle := v[0] * math.Pi / 180
			c, si := math.Cos(angle), math.Sin(angle)
			n = affine{c, si, -si, c, 0, 0}
			if len(v) == 3 {
				n = affine{1, 0, 0, 1, v[1], v[2]}.mul(n).mul(affine{1, 0, 0, 1, -v[1], -v[2]})
			}
		default:
			return m, errors.New("transform option")
		}
		m = m.mul(n)
		for _, x := range m {
			if !finite(x) || math.Abs(x) > 1e6 {
				return m, errors.New("transform range")
			}
		}
		s = s[end+1:]
	}
	return m, nil
}

func parseSVGPath(data string, limit int) (geometryPath, error) {
	s := numberScanner{s: data}
	var p geometryPath
	var cmd, prev byte
	var x, y, startX, startY, cx, cy, qx, qy float64
	for {
		s.skip()
		if s.i >= len(data) {
			break
		}
		b := data[s.i]
		if b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' {
			cmd = b
			s.i++
		} else if cmd == 0 {
			return nil, errors.New("path command")
		}
		rel := cmd >= 'a' && cmd <= 'z'
		op := cmd
		if rel {
			op -= 32
		}
		if op == 'Z' {
			p.add('Z')
			x, y = startX, startY
			prev = op
			cmd = 0
			continue
		}
		n := 0
		switch op {
		case 'M', 'L', 'T':
			n = 2
		case 'H', 'V':
			n = 1
		case 'Q', 'S':
			n = 4
		case 'C':
			n = 6
		case 'A':
			n = 7
		default:
			return nil, errors.New("path command")
		}
		a := make([]float64, n)
		for i := range a {
			v, e := s.number()
			if e != nil {
				return nil, e
			}
			a[i] = v
		}
		if op == 'A' {
			if a[3] != 0 && a[3] != 1 || a[4] != 0 && a[4] != 1 {
				return nil, errors.New("arc flags")
			}
			if rel {
				a[5] += x
				a[6] += y
			}
			appendArc(&p, x, y, a[0], a[1], a[2], a[3] == 1, a[4] == 1, a[5], a[6])
			x, y = a[5], a[6]
		} else if op == 'H' {
			if rel {
				a[0] += x
			}
			x = a[0]
			p.add('L', x, y)
		} else if op == 'V' {
			if rel {
				a[0] += y
			}
			y = a[0]
			p.add('L', x, y)
		} else {
			if rel {
				for i := 0; i < len(a); i += 2 {
					a[i] += x
					a[i+1] += y
				}
			}
			switch op {
			case 'M':
				x, y = a[0], a[1]
				startX, startY = x, y
				p.add('M', x, y)
				if rel {
					cmd = 'l'
				} else {
					cmd = 'L'
				}
			case 'L':
				x, y = a[0], a[1]
				p.add('L', x, y)
			case 'C':
				p.add('C', a...)
				cx, cy = a[2], a[3]
				x, y = a[4], a[5]
			case 'S':
				sx, sy := x, y
				if prev == 'C' || prev == 'S' {
					sx, sy = 2*x-cx, 2*y-cy
				}
				p.add('C', sx, sy, a[0], a[1], a[2], a[3])
				cx, cy = a[0], a[1]
				x, y = a[2], a[3]
			case 'Q':
				p.add('Q', a...)
				qx, qy = a[0], a[1]
				x, y = a[2], a[3]
			case 'T':
				sx, sy := x, y
				if prev == 'Q' || prev == 'T' {
					sx, sy = 2*x-qx, 2*y-qy
				}
				p.add('Q', sx, sy, a[0], a[1])
				qx, qy = sx, sy
				x, y = a[0], a[1]
			}
		}
		prev = op
		if len(p) > limit {
			return nil, errors.New("path complexity")
		}
	}
	if len(p) > 0 && p[0].op != 'M' {
		return nil, errors.New("path start")
	}
	return p, nil
}
func appendArc(p *geometryPath, x1, y1, rx, ry, rotation float64, large, sweep bool, x2, y2 float64) {
	rx, ry = math.Abs(rx), math.Abs(ry)
	if x1 == x2 && y1 == y2 {
		return
	}
	if rx == 0 || ry == 0 {
		p.add('L', x2, y2)
		return
	}
	phi := math.Mod(rotation, 360) * math.Pi / 180
	c, s := math.Cos(phi), math.Sin(phi)
	dx, dy := (x1-x2)/2, (y1-y2)/2
	xp, yp := c*dx+s*dy, -s*dx+c*dy
	lambda := xp*xp/(rx*rx) + yp*yp/(ry*ry)
	if lambda > 1 {
		scale := math.Sqrt(lambda)
		rx *= scale
		ry *= scale
	}
	den := rx*rx*yp*yp + ry*ry*xp*xp
	factor := 0.0
	if den > 0 {
		factor = math.Sqrt(math.Max(0, (rx*rx*ry*ry-den)/den))
	}
	if large == sweep {
		factor = -factor
	}
	cxp, cyp := factor*rx*yp/ry, -factor*ry*xp/rx
	centerX, centerY := c*cxp-s*cyp+(x1+x2)/2, s*cxp+c*cyp+(y1+y2)/2
	ux, uy := (xp-cxp)/rx, (yp-cyp)/ry
	vx, vy := (-xp-cxp)/rx, (-yp-cyp)/ry
	theta := math.Atan2(uy, ux)
	delta := math.Atan2(ux*vy-uy*vx, ux*vx+uy*vy)
	if !sweep && delta > 0 {
		delta -= 2 * math.Pi
	}
	if sweep && delta < 0 {
		delta += 2 * math.Pi
	}
	parts := max(1, int(math.Ceil(math.Abs(delta)/(math.Pi/2))))
	step := delta / float64(parts)
	point := func(t float64) (float64, float64) {
		return centerX + c*rx*math.Cos(t) - s*ry*math.Sin(t), centerY + s*rx*math.Cos(t) + c*ry*math.Sin(t)
	}
	deriv := func(t float64) (float64, float64) {
		return -c*rx*math.Sin(t) - s*ry*math.Cos(t), -s*rx*math.Sin(t) + c*ry*math.Cos(t)
	}
	for i := 0; i < parts; i++ {
		a, b := theta+float64(i)*step, theta+float64(i+1)*step
		k := 4.0 / 3 * math.Tan(step/4)
		ax, ay := point(a)
		bx, by := point(b)
		adx, ady := deriv(a)
		bdx, bdy := deriv(b)
		p.add('C', ax+k*adx, ay+k*ady, bx-k*bdx, by-k*bdy, bx, by)
	}
}

type svgState struct {
	matrix  affine
	fill    color.NRGBA
	none    bool
	opacity float64
}

func rasterizeSVG(ctx context.Context, data []byte, generated bool) (image.Image, error) {
	if len(data) > MaxResourceBytes && !generated {
		return nil, invalid("image", "limit", "SVG exceeds the input limit")
	}
	decoder := xml.NewDecoder(contextReader{ctx, bytes.NewReader(data)})
	stack := []svgState{}
	state := svgState{matrix: identity(), fill: color.NRGBA{A: 255}, opacity: 1}
	var dst *image.NRGBA
	nodes, segments := 0, 0
	nodeLimit, segmentLimit := 2048, 30000
	if generated {
		nodeLimit, segmentLimit = 10000, 400000
	}
	rootClosed := false
	bad := func() (image.Image, error) {
		return nil, invalid("image", "svg_profile", "SVG must use bounded static filled geometry; scripts, styles, strokes, text, external references and entities are unsupported")
	}
	for {
		token, e := decoder.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return bad()
		}
		if e = ctx.Err(); e != nil {
			return nil, e
		}
		switch t := token.(type) {
		case xml.Directive:
			return bad()
		case xml.ProcInst:
			if t.Target != "xml" || dst != nil {
				return bad()
			}
		case xml.CharData:
			if strings.TrimSpace(string(t)) != "" {
				return bad()
			}
		case xml.StartElement:
			nodes++
			if nodes > nodeLimit || len(stack) > 16 || rootClosed || t.Name.Space != "" && t.Name.Space != "http://www.w3.org/2000/svg" {
				return bad()
			}
			name := t.Name.Local
			if !oneOf(name, "svg", "g", "rect", "circle", "ellipse", "path", "polygon", "polyline") && !(generated && name == "image") {
				return bad()
			}
			attrs := map[string]string{}
			for _, a := range t.Attr {
				key := a.Name.Local
				if a.Name.Space != "" && a.Name.Space != "xmlns" {
					return bad()
				}
				if _, exists := attrs[key]; exists {
					return bad()
				}
				attrs[key] = a.Value
			}
			allowed := " fill fill-opacity opacity transform id "
			switch name {
			case "svg":
				allowed += " width height viewBox xmlns version preserveAspectRatio "
			case "rect":
				allowed += " x y width height rx ry "
			case "circle":
				allowed += " cx cy r "
			case "ellipse":
				allowed += " cx cy rx ry "
			case "path":
				allowed += " d fill-rule "
			case "polygon", "polyline":
				allowed += " points "
			case "image":
				allowed += " x y width height href preserveAspectRatio "
			}
			for k := range attrs {
				if !strings.Contains(allowed, " "+k+" ") {
					return bad()
				}
			}
			next := state
			if fill, ok := attrs["fill"]; ok {
				next.none = fill == "none"
				if !next.none {
					_, col, e := parseColor(fill)
					if e != nil {
						return bad()
					}
					next.fill = col
				}
			}
			for _, key := range []string{"opacity", "fill-opacity"} {
				if val, ok := attrs[key]; ok {
					v, e := numberList(val, 1)
					if e != nil || len(v) != 1 || v[0] < 0 || v[0] > 1 {
						return bad()
					}
					next.opacity *= v[0]
				}
			}
			if (name == "svg" || name == "g") && (attrs["opacity"] != "" || attrs["fill-opacity"] != "") {
				return bad()
			}
			if tr := attrs["transform"]; tr != "" {
				m, e := parseTransform(tr)
				if e != nil {
					return bad()
				}
				next.matrix = next.matrix.mul(m)
			}
			stack = append(stack, state)
			state = next
			get := func(k string) (float64, error) {
				if attrs[k] == "" {
					return 0, nil
				}
				return svgLength(attrs[k])
			}
			if name == "svg" {
				if dst != nil || len(stack) != 1 {
					return bad()
				}
				w, e1 := get("width")
				h, e2 := get("height")
				view := []float64{}
				if attrs["viewBox"] != "" {
					var e error
					view, e = numberList(attrs["viewBox"], 4)
					if e != nil || len(view) != 4 || view[2] <= 0 || view[3] <= 0 {
						return bad()
					}
					if w == 0 {
						w = view[2]
					}
					if h == 0 {
						h = view[3]
					}
				}
				if e1 != nil || e2 != nil || w <= 0 || h <= 0 || w > 32768 || h > 32768 || w*h > MaxInputPixels {
					return bad()
				}
				originalW, originalH := w, h
				if !generated {
					scale := math.Min(1, math.Min(2048/w, 2048/h))
					w *= scale
					h *= scale
				}
				width, height := max(1, int(math.Ceil(w))), max(1, int(math.Ceil(h)))
				if generated && (width > MaxOutputDimension || height > MaxOutputDimension) {
					return bad()
				}
				dst = image.NewNRGBA(image.Rect(0, 0, width, height))
				if len(view) == 4 {
					sx, sy := w/view[2], h/view[3]
					if attrs["preserveAspectRatio"] != "none" {
						if a := attrs["preserveAspectRatio"]; a != "" && a != "xMidYMid meet" && a != "xMidYMid" {
							return bad()
						}
						scale := math.Min(sx, sy)
						state.matrix = state.matrix.mul(affine{scale, 0, 0, scale, (w-view[2]*scale)/2 - view[0]*scale, (h-view[3]*scale)/2 - view[1]*scale})
					} else {
						state.matrix = state.matrix.mul(affine{sx, 0, 0, sy, -view[0] * sx, -view[1] * sy})
					}
				} else if w != originalW || h != originalH {
					state.matrix = state.matrix.mul(affine{w / originalW, 0, 0, h / originalH, 0, 0})
				}
				continue
			}
			if dst == nil {
				return bad()
			}
			if name == "g" {
				continue
			}
			var p geometryPath
			switch name {
			case "rect":
				x, e1 := get("x")
				y, e2 := get("y")
				w, e3 := get("width")
				h, e4 := get("height")
				rx, e5 := get("rx")
				ry, e6 := get("ry")
				if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil || e6 != nil || w < 0 || h < 0 || rx < 0 || ry < 0 {
					return bad()
				}
				if rx != 0 && ry != 0 && rx != ry {
					return bad()
				}
				p = roundedRectangle(x, y, w, h, uniformRadii(math.Max(rx, ry)))
			case "circle", "ellipse":
				cx, e1 := get("cx")
				cy, e2 := get("cy")
				rx, e3 := get("r")
				ry := rx
				if name == "ellipse" {
					rx, e3 = get("rx")
					ry, e = get("ry")
				}
				if e1 != nil || e2 != nil || e3 != nil || e != nil || rx < 0 || ry < 0 {
					return bad()
				}
				if rx > 0 && ry > 0 {
					p = transformPath(circle(0, 0, 1), affine{rx, 0, 0, ry, cx, cy})
				}
			case "path":
				if attrs["fill-rule"] != "" && attrs["fill-rule"] != "nonzero" {
					return bad()
				}
				var e error
				p, e = parseSVGPath(attrs["d"], segmentLimit-segments)
				if e != nil {
					return bad()
				}
			case "polygon", "polyline":
				v, e := numberList(attrs["points"], segmentLimit*2)
				if e != nil || len(v) < 4 || len(v)%2 != 0 {
					return bad()
				}
				for i := 0; i < len(v); i += 2 {
					op := byte('L')
					if i == 0 {
						op = 'M'
					}
					p.add(op, v[i], v[i+1])
				}
				p.add('Z')
			case "image":
				href := attrs["href"]
				const prefix = "data:image/png;base64,"
				if !strings.HasPrefix(href, prefix) {
					return bad()
				}
				raw, e := base64.StdEncoding.DecodeString(strings.TrimPrefix(href, prefix))
				if e != nil || len(raw) > 32<<20 {
					return bad()
				}
				im, format, e := decodeRaster(ctx, raw)
				if e != nil || format != "png" {
					return bad()
				}
				x, e1 := get("x")
				y, e2 := get("y")
				w, e3 := get("width")
				h, e4 := get("height")
				if e1 != nil || e2 != nil || e3 != nil || e4 != nil || w <= 0 || h <= 0 || state.matrix != identity() || state.opacity != 1 {
					return bad()
				}
				rect := image.Rect(int(x), int(y), int(x+w), int(y+h))
				xdraw.ApproxBiLinear.Scale(dst, rect, im, im.Bounds(), draw.Over, nil)
				continue
			}
			segments += len(p)
			if segments > segmentLimit {
				return bad()
			}
			p = transformPath(p, state.matrix)
			for _, cmd := range p {
				for i := 0; i < commandArgs(cmd.op); i++ {
					if !finite(cmd.a[i]) || math.Abs(cmd.a[i]) > 1e7 {
						return bad()
					}
				}
			}
			if !state.none {
				c := state.fill
				c.A = uint8(float64(c.A) * state.opacity)
				if err := paintPath(ctx, dst, p, c); err != nil {
					return nil, err
				}
			}
		case xml.EndElement:
			if len(stack) == 0 {
				return bad()
			}
			state = stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				rootClosed = true
			}
		}
	}
	if dst == nil || !rootClosed || len(stack) != 0 {
		return bad()
	}
	return dst, nil
}
