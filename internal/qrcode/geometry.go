package qrcode

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"strconv"
	"strings"

	"golang.org/x/image/vector"
)

type pathCommand struct {
	op byte
	a  [6]float64
}
type geometryPath []pathCommand

func (p *geometryPath) add(op byte, a ...float64) {
	var c pathCommand
	c.op = op
	copy(c.a[:], a)
	*p = append(*p, c)
}
func (p geometryPath) translated(x, y float64) geometryPath {
	q := make(geometryPath, len(p))
	copy(q, p)
	for i := range q {
		for j := 0; j < commandArgs(q[i].op); j += 2 {
			q[i].a[j] += x
			q[i].a[j+1] += y
		}
	}
	return q
}
func commandArgs(op byte) int {
	switch op {
	case 'M', 'L':
		return 2
	case 'Q':
		return 4
	case 'C':
		return 6
	}
	return 0
}
func (p geometryPath) svg() string {
	var b strings.Builder
	for _, c := range p {
		b.WriteByte(c.op)
		for j := 0; j < commandArgs(c.op); j++ {
			if j > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(strconv.FormatFloat(c.a[j], 'f', 4, 64))
		}
	}
	return b.String()
}
func rectangle(x, y, w, h float64) geometryPath {
	var p geometryPath
	p.add('M', x, y)
	p.add('L', x+w, y)
	p.add('L', x+w, y+h)
	p.add('L', x, y+h)
	p.add('Z')
	return p
}
func roundedRectangle(x, y, w, h float64, r [4]float64) geometryPath {
	for i := range r {
		r[i] = math.Min(r[i], math.Min(w, h)/2)
	}
	if r == [4]float64{} {
		return rectangle(x, y, w, h)
	}
	var p geometryPath
	p.add('M', x+r[0], y)
	p.add('L', x+w-r[1], y)
	p.add('Q', x+w, y, x+w, y+r[1])
	p.add('L', x+w, y+h-r[2])
	p.add('Q', x+w, y+h, x+w-r[2], y+h)
	p.add('L', x+r[3], y+h)
	p.add('Q', x, y+h, x, y+h-r[3])
	p.add('L', x, y+r[0])
	p.add('Q', x, y, x+r[0], y)
	p.add('Z')
	return p
}
func uniformRadii(r float64) [4]float64 { return [4]float64{r, r, r, r} }
func circle(cx, cy, r float64) geometryPath {
	const k = .5522847498307936
	var p geometryPath
	p.add('M', cx+r, cy)
	p.add('C', cx+r, cy+k*r, cx+k*r, cy+r, cx, cy+r)
	p.add('C', cx-k*r, cy+r, cx-r, cy+k*r, cx-r, cy)
	p.add('C', cx-r, cy-k*r, cx-k*r, cy-r, cx, cy-r)
	p.add('C', cx+k*r, cy-r, cx+r, cy-k*r, cx+r, cy)
	p.add('Z')
	return p
}
func mirrorX(p geometryPath, axis float64) geometryPath {
	q := make(geometryPath, len(p))
	copy(q, p)
	for i := range q {
		for j := 0; j < commandArgs(q[i].op); j += 2 {
			q[i].a[j] = 2*axis - q[i].a[j]
		}
	}
	return q
}
func paintPath(ctx context.Context, dst draw.Image, p geometryPath, c color.NRGBA) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.A == 0 || len(p) == 0 {
		return nil
	}
	bounds := dst.Bounds()
	r := vector.NewRasterizer(bounds.Dx(), bounds.Dy())
	for i, v := range p {
		if i&63 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		a := v.a
		switch v.op {
		case 'M':
			r.MoveTo(float32(a[0]), float32(a[1]))
		case 'L':
			r.LineTo(float32(a[0]), float32(a[1]))
		case 'Q':
			r.QuadTo(float32(a[0]), float32(a[1]), float32(a[2]), float32(a[3]))
		case 'C':
			r.CubeTo(float32(a[0]), float32(a[1]), float32(a[2]), float32(a[3]), float32(a[4]), float32(a[5]))
		case 'Z':
			r.ClosePath()
		}
	}
	r.Draw(dst, bounds, image.NewUniform(c), image.Point{})
	return ctx.Err()
}
func pathBounds(p geometryPath) (float64, float64, float64, float64) {
	minx, miny, maxx, maxy := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, c := range p {
		for i := 0; i < commandArgs(c.op); i += 2 {
			minx = math.Min(minx, c.a[i])
			maxx = math.Max(maxx, c.a[i])
			miny = math.Min(miny, c.a[i+1])
			maxy = math.Max(maxy, c.a[i+1])
		}
	}
	return minx, miny, maxx, maxy
}
func pathRing(x, y, w, h, border, radius float64) geometryPath {
	outer := roundedRectangle(x, y, w, h, uniformRadii(radius))
	inner := roundedRectangle(x+border, y+border, w-2*border, h-2*border, uniformRadii(math.Max(0, radius-border)))
	return append(outer, mirrorX(inner, x+w/2)...)
}
func svgColor(c color.NRGBA) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }
