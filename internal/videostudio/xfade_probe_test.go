package videostudio

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The xfade probe fixture (plan 2e Task 10; index §4.12, §4.24) is the transition truth of phase 2e.
//   - Inputs: two Go-generated pattern frames (xfadePatterns), written as raw gbrap and read with -f rawvideo, so every
//     FFmpeg version sees the same bytes.
//   - Each registry transition runs through xfade alone: offset 0, duration D/25, 4 filter threads, rawvideo gbrap out.
//     Version equality is checked at this level because premultiply and unpremultiply round differently on 5.1.9 and
//     8.1.1 (risk 32); the goldens cover the full pipeline on 8.1.1.
//   - A row holds the SHA-256 of one overlap frame (excluded pixels zeroed) and its samples [x, y, r, g, b, a]:
//     a 4×3 grid, both pixels of every D1 edge (indices in edges), and the D3 quirk pixels.
//   - AURAGO_UPDATE_PARITY=1 records the fixture on FFmpeg 8.x (TestXfadeProbesMatchFixture).
//
// Readers:
//   - TestXfadeModelMatchesFixture: the Go float32 model of §4.24, no FFmpeg needed;
//   - TestXfadeProbesMatchFixture: the local FFmpeg;
//   - ui TestDesktopVideoStudioTransitionsGLBrowser: the GL shaders, which rebuild the patterns from the header.
var xfadeProbePath = filepath.Join("testdata", "xfade", "probes.json")

// xfadeProbeMajor is the FFmpeg major version that records the fixture. The spike found all 24 transitions
// bit-exact against the float32 model on 8.1.1.
const xfadeProbeMajor = 8

// xfadePattern is one probe frame.
//   - Colour plane c (g, b, r) is (Planes[c][0]·x + Planes[c][1]·y + Planes[c][2]) mod 256.
//   - The alpha variant has α = 64 + ((Alpha[0]·x + Alpha[1]·y + Alpha[2]) mod 192) and colour floor(plane·α/255),
//     the premultiplied form in which xfade sees the export's layers.
//   - The opaque variant has α = 255.
type xfadePattern struct {
	Planes [3][3]int `json:"planes"`
	Alpha  [3]int    `json:"alpha"`
}

type xfadeProbeRow struct {
	Type    string   `json:"type"`    // registry transition id
	Size    [2]int   `json:"size"`    // width, height
	D       int      `json:"D"`       // overlap frames
	I       int      `json:"i"`       // overlap frame
	Variant string   `json:"variant"` // "opaque" or "alpha"
	SHA256  string   `json:"sha256"`  // planar gbrap frame, excluded pixels zeroed
	Samples [][6]int `json:"samples"` // [x, y, r, g, b, a]
	Edges   []int    `json:"edges,omitempty"`
}

type xfadeProbeFile struct {
	Comment  string                  `json:"comment"`
	FFmpeg   string                  `json:"ffmpeg"` // first line of -version of the recording binary
	Major    int                     `json:"major"`
	Patterns map[string]xfadePattern `json:"patterns"` // "a" outgoing, "b" incoming
	Rows     []xfadeProbeRow         `json:"rows"`
}

// xfadeProbePatterns are the two probe frames.
//   - Every plane moves by at least 1 per pixel in x or y, so a sample read one index off shows.
//   - B's red plane is A's plus 128, so the opaque layers differ by 128 in red at every pixel. A GL sample within
//     2/255 therefore also pins which layer a pixel shows.
var xfadeProbePatterns = map[string]xfadePattern{
	"a": {Planes: [3][3]int{{5, 3, 0}, {2, 7, 40}, {11, 1, 90}}, Alpha: [3]int{3, 5, 0}},
	"b": {Planes: [3][3]int{{3, 9, 200}, {7, 2, 120}, {11, 1, 218}}, Alpha: [3]int{5, 2, 96}},
}

// xfadeProbeCombos are the recorded geometries:
//   - an even 16:9 frame at D = 20, opaque and alpha;
//   - an odd frame at D = 7, where 1/7 is not a float32.
//
// Frames 0 and D − 1 carry the quirks (§4.24 D3); the others sample the motion.
var xfadeProbeCombos = []struct {
	variant       string
	width, height int
	duration      int
	frames        []int
}{
	{"opaque", 96, 54, 20, []int{0, 5, 10, 15, 19}},
	{"alpha", 96, 54, 20, []int{0, 5, 15, 19}},
	{"opaque", 75, 43, 7, []int{0, 3, 4, 6}},
}

// xfadeProbeEdgedTypes are the hard-edged registry transitions; each needs rows with edge samples.
var xfadeProbeEdgedTypes = []string{"wipeleft", "wiperight", "wipeup", "wipedown", "slideleft", "slideright", "slideup", "slidedown", "circlecrop", "rectcrop", "pixelize", "squeezeh"}

// xfadePatterns returns the outgoing (a) and incoming (b) probe frames of a variant as planar gbrap: planes g, b, r and
// a, each width·height bytes (index §4.12).
func xfadePatterns(variant string, width, height int) (a, b []byte) {
	return xfadePatternFrame(xfadeProbePatterns["a"], variant, width, height), xfadePatternFrame(xfadeProbePatterns["b"], variant, width, height)
}

func xfadePatternFrame(p xfadePattern, variant string, width, height int) []byte {
	n := width * height
	frame := make([]byte, 4*n)
	for y := range height {
		for x := range width {
			k := y*width + x
			alpha := 255
			if variant == "alpha" {
				alpha = 64 + (p.Alpha[0]*x+p.Alpha[1]*y+p.Alpha[2])%192
			}
			for plane, c := range p.Planes {
				v := (c[0]*x + c[1]*y + c[2]) % 256
				if variant == "alpha" {
					v = v * alpha / 255
				}
				frame[plane*n+k] = byte(v)
			}
			frame[3*n+k] = byte(alpha)
		}
	}
	return frame
}

// xfadeProbeExcluded reports the pixels whose xfade value is undefined: column 0 of slideleft and row 0 of slideup
// read out of range at overlap frame 0, on every FFmpeg version (§4.24 D3). They are zeroed before hashing and never
// sampled.
func xfadeProbeExcluded(xfade string, i, x, y int) bool {
	return i == 0 && ((xfade == "slideleft" && x == 0) || (xfade == "slideup" && y == 0))
}

// xfadeProbeZeroExcluded zeroes the excluded pixels of a planar gbrap frame in all four planes.
func xfadeProbeZeroExcluded(frame []byte, xfade string, i, width, height int) {
	n := width * height
	for y := range height {
		for x := range width {
			if xfadeProbeExcluded(xfade, i, x, y) {
				for plane := range 4 {
					frame[plane*n+y*width+x] = 0
				}
			}
		}
	}
}

// xfadeProbeModel is the Go float32 model of overlap frame i of D: the §4.24 per-pixel rules, ported from the spike's
// models.js EXACT.
//   - Every per-frame number comes from transitionFrameAt (D1), so the model checks Task 2's parameters too.
//   - Every float operation is rounded to float32 by an explicit conversion, so no fused multiply-add changes a value.
//   - The 8-bit store truncates.
//
// a, b and the result are planar gbrap frames of width × height; excluded pixels are 0.
func xfadeProbeModel(spec TransitionSpec, a, b []byte, width, height, i, D int) []byte {
	fr := transitionFrameAt(spec.ID, i, D, width, height)
	n := width * height
	frame := make([]byte, 4*n)
	for plane := range 4 {
		for y := range height {
			for x := range width {
				if !xfadeProbeExcluded(spec.XFade, i, x, y) {
					frame[plane*n+y*width+x] = xfadeProbePixel(spec.XFade, fr, a, b, plane, x, y, width, height)
				}
			}
		}
	}
	return frame
}

// xfadeProbePixel is one plane value of the model at (x, y), by xfade name (§4.24 "Per-pixel rules"). A and B are the
// outgoing and incoming plane values; mix32(a, b, m) weighs its first argument by m.
func xfadeProbePixel(xfade string, fr transitionFrame, a, b []byte, plane, x, y, width, height int) byte {
	n := width * height
	at := func(frame []byte, px, py int) byte { return frame[plane*n+py*width+px] }
	va, vb := float32(at(a, x, y)), float32(at(b, x, y))
	alpha := plane == 3
	fill := byte(0) // the opaque black of circlecrop and rectcrop
	if alpha {
		fill = 255
	}
	switch xfade {
	case "fade":
		return xfadeProbeStore(mix32(va, vb, fr.P))
	case "fadeblack", "fadewhite", "fadegrays":
		var ka, kb float32 // fadeblack's colour planes: K = 0
		switch {
		case xfade == "fadewhite" || (xfade == "fadeblack" && alpha):
			ka, kb = 255, 255
		case xfade == "fadegrays" && alpha:
			ka, kb = va, vb // the alpha "grey" is the pixel's own alpha
		case xfade == "fadegrays":
			ka, kb = xfadeProbeGrey(a, x, y, width, height), xfadeProbeGrey(b, x, y, width, height)
		}
		return xfadeProbeStore(mix32(mix32(va, ka, fr.U), mix32(kb, vb, fr.V), fr.P))
	case "pixelize":
		sx, sy := x, y
		if fr.Sq > 0 {
			sx, sy = xfadeProbePixelizeIndex(x, width, fr.Sq), xfadeProbePixelizeIndex(y, height, fr.Sq)
		}
		return xfadeProbeStore(mix32(float32(at(a, sx, sy)), float32(at(b, sx, sy)), fr.P))
	case "wipeleft": // the user's wiperight: B enters from the right
		if x > fr.Edge {
			return at(b, x, y)
		}
		return at(a, x, y)
	case "wiperight": // the user's wipeleft: B grows from the left; column 0 already shows B at i = 0
		if x > fr.Edge {
			return at(a, x, y)
		}
		return at(b, x, y)
	case "wipeup":
		if y > fr.Edge {
			return at(b, x, y)
		}
		return at(a, x, y)
	case "wipedown":
		if y > fr.Edge {
			return at(a, x, y)
		}
		return at(b, x, y)
	case "slideleft", "slideright", "slideup", "slidedown":
		horizontal := xfade == "slideleft" || xfade == "slideright"
		pos, size := y, height
		if horizontal {
			pos, size = x, width
		}
		var z int
		var inB bool
		if xfade == "slideleft" || xfade == "slideup" {
			z = pos - fr.Shift
			inB = z >= 0 && z < size
		} else {
			z = pos + fr.Shift
			inB = z > 0 && z < size
		}
		zz := z % size // C's remainder, then the wrap of models.js
		if z < 0 {
			zz += size
		}
		if zz >= size {
			return 0 // the excluded out-of-range read (slideleft, slideup at i = 0)
		}
		sx, sy := x, zz
		if horizontal {
			sx, sy = zz, y
		}
		if inB {
			return at(b, sx, sy)
		}
		return at(a, sx, sy)
	case "smoothleft", "smoothright", "diagtl", "diagbr":
		t := smoothstep32(0, 1, float32(float32(1+xfadeProbeSoft(xfade, x, y, width, height))-fr.C))
		return xfadeProbeStore(mix32(vb, va, t))
	case "radial":
		// atan2 rounded to float32, minus the float64 k, rounded again: xfade's float − double expression.
		angle := float32(math.Atan2(float64(x-fr.CX), float64(y-fr.CY)))
		return xfadeProbeStore(mix32(vb, va, smoothstep32(0, 1, float32(float64(angle)-fr.K))))
	case "circleopen", "circleclose":
		t := smoothstep32(0, 1, float32(float32(xfadeProbeDistance(x-fr.CX, y-fr.CY)/fr.R)+fr.C))
		if xfade == "circleopen" {
			return xfadeProbeStore(mix32(va, vb, t))
		}
		return xfadeProbeStore(mix32(vb, va, t))
	case "circlecrop":
		if fr.Radius < xfadeProbeDistance(x-fr.CX, y-fr.CY) {
			return fill
		}
		if fr.P < 0.5 {
			return at(b, x, y)
		}
		return at(a, x, y)
	case "rectcrop":
		// The centre is the plane's (W >> 1, H >> 1); the GL shader takes it from u_size.
		if absInt(x-width>>1) < fr.ZW && absInt(y-height>>1) < fr.ZH {
			if fr.P < 0.5 {
				return at(b, x, y)
			}
			return at(a, x, y)
		}
		return fill
	case "squeezeh":
		z := xfadeProbeSqueezeZ(y, height, fr.P)
		if z < 0 || z > 1 {
			return at(b, x, y)
		}
		return at(a, x, int(math.RoundToEven(float64(float32(z*float32(height-1))))))
	case "zoomin":
		ix := min(width-1, max(0, int(math.Ceil(float64(xfadeProbeZoomCoord(x, width, fr.Zf))))))
		iy := min(height-1, max(0, int(math.Ceil(float64(xfadeProbeZoomCoord(y, height, fr.Zf))))))
		return xfadeProbeStore(mix32(float32(at(a, ix, iy)), vb, fr.W))
	}
	panic("xfade probe model: no rule for " + xfade)
}

// xfadeProbeStore is xfade's 8-bit store: the float32 value truncated.
func xfadeProbeStore(v float32) byte { return byte(int(v)) }

// xfadeProbeGrey is fadegrays' grey of a pixel: trunc((G + B + R)/3) of the three colour planes.
func xfadeProbeGrey(frame []byte, x, y, width, height int) float32 {
	n, k := width*height, y*width+x
	return float32((int(frame[k]) + int(frame[n+k]) + int(frame[2*n+k])) / 3)
}

// xfadeProbeDistance is hypotf of an integer offset: the exact root rounded to float32.
func xfadeProbeDistance(dx, dy int) float32 { return float32(math.Sqrt(float64(dx*dx + dy*dy))) }

// xfadeProbeSoft is the position term e of smoothleft, smoothright, diagtl and diagbr.
func xfadeProbeSoft(xfade string, x, y, width, height int) float32 {
	w, h := float32(width), float32(height)
	switch xfade {
	case "smoothleft":
		return float32(float32(x) / w)
	case "smoothright":
		return float32(float32(width-1-x) / w)
	case "diagtl":
		return float32(float32(float32(float32(x)/w)*float32(y)) / h)
	default: // diagbr
		return float32(float32(float32(float32(width-1-x)/w)*float32(height-1-y)) / h)
	}
}

// xfadeProbePixelizeIndex is pixelize's sample index min(size − 1, trunc((floor(c/sq) + 0.5)·sq)).
func xfadeProbePixelizeIndex(c, size int, sq float32) int {
	q := float32(float32(c) / sq)
	s := float32(float32(float32(math.Floor(float64(q)))+0.5) * sq)
	return min(size-1, int(s))
}

// xfadeProbeZoomCoord is zoomin's coordinate before ceil: ((c/N − 0.5)·zf + 0.5)·(N − 1).
func xfadeProbeZoomCoord(c, size int, zf float32) float32 {
	t := float32(float32(c) / float32(size))
	t = float32(t - 0.5)
	t = float32(t * zf)
	t = float32(t + 0.5)
	return float32(t * float32(size-1))
}

// xfadeProbeSqueezeZ is squeezeh's z = 0.5 + (y/H − 0.5)/P.
func xfadeProbeSqueezeZ(y, height int, P float32) float32 {
	t := float32(float32(y) / float32(height))
	t = float32(t - 0.5)
	t = float32(t / P)
	return float32(0.5 + t)
}

// xfadeProbeStable reports whether a pixel's value is safe from the GL rounding that GLSL ES allows: division within
// 2.5 ULP, square roots, possible multiply-add fusion. A pixel is unsafe when one of these lies within 1e-3 of the
// step it is compared with:
//   - a float that decides a pixelize, zoomin or squeezeh index;
//   - the circlecrop distance, against the radius.
//
// Exact zeros stay safe, and so does zoomin at zf = 0, where every step is exact.
func xfadeProbeStable(xfade string, fr transitionFrame, width, height, x, y int) bool {
	near := func(v float64) bool { return v != 0 && math.Abs(v-math.Round(v)) < 1e-3 }
	switch xfade {
	case "pixelize":
		if fr.Sq <= 0 {
			return true
		}
		for _, c := range []int{x, y} {
			q := float32(float32(c) / fr.Sq)
			s := float32(float32(float32(math.Floor(float64(q)))+0.5) * fr.Sq)
			if near(float64(q)) || near(float64(s)) {
				return false
			}
		}
	case "zoomin":
		// At zf = 0 (P below 0.5) the coordinate is 0.5·(N − 1): every step is exact, so an integer result is a
		// stable index in any IEEE evaluation, whatever the fusion or division rounding.
		if fr.Zf != 0 && (near(float64(xfadeProbeZoomCoord(x, width, fr.Zf))) || near(float64(xfadeProbeZoomCoord(y, height, fr.Zf)))) {
			return false
		}
	case "squeezeh":
		z := xfadeProbeSqueezeZ(y, height, fr.P)
		row := float64(float32(z * float32(height-1)))
		if math.Abs(float64(z)) < 1e-4 || math.Abs(float64(z)-1) < 1e-4 || math.Abs(row-math.Floor(row)-0.5) < 1e-3 {
			return false
		}
	case "circlecrop":
		if math.Abs(math.Hypot(float64(x-(width>>1)), float64(y-(height>>1)))-float64(fr.Radius)) < 1e-3 {
			return false
		}
	}
	return true
}

// xfadeProbePositions picks the sample pixels of one row:
//   - a 4×3 grid;
//   - for the hard-edged types, both pixels of every D1 edge; their indices go to edges;
//   - the quirk pixels of §4.24 D3, sampled at every frame: rectcrop's frame-0 border and radial's last-frame wedge.
//
// Excluded and unstable pixels are skipped. An edge counts only when both of its pixels can be sampled.
func xfadeProbePositions(spec TransitionSpec, fr transitionFrame, width, height, i int) (points [][2]int, edges []int) {
	usable := func(x, y int) bool {
		return x >= 0 && y >= 0 && x < width && y < height && !xfadeProbeExcluded(spec.XFade, i, x, y) && xfadeProbeStable(spec.XFade, fr, width, height, x, y)
	}
	index := map[[2]int]int{}
	add := func(x, y int) int {
		if !usable(x, y) {
			return -1
		}
		point := [2]int{x, y}
		if k, ok := index[point]; ok {
			return k
		}
		index[point] = len(points)
		points = append(points, point)
		return index[point]
	}
	pair := func(x0, y0, x1, y1 int) bool {
		if !usable(x0, y0) || !usable(x1, y1) {
			return false
		}
		for _, k := range []int{add(x0, y0), add(x1, y1)} {
			if !slices.Contains(edges, k) {
				edges = append(edges, k)
			}
		}
		return true
	}
	for gy := range 3 {
		for gx := range 4 {
			add((2*gx+1)*width/8, (2*gy+1)*height/6)
		}
	}
	cx, cy := width>>1, height>>1
	switch spec.XFade {
	case "wipeleft", "wiperight":
		pair(fr.Edge, cy, fr.Edge+1, cy)
	case "wipeup", "wipedown":
		pair(cx, fr.Edge, cx, fr.Edge+1)
	case "slideleft":
		pair(fr.Shift-1, cy, fr.Shift, cy)
	case "slideright":
		pair(width-fr.Shift-1, cy, width-fr.Shift, cy)
	case "slideup":
		pair(cx, fr.Shift-1, cx, fr.Shift)
	case "slidedown":
		pair(cx, height-fr.Shift-1, cx, height-fr.Shift)
	case "circlecrop":
		r := int(fr.Radius)
		pair(cx+r, cy, cx+r+1, cy)
	case "rectcrop":
		if fr.ZW > 0 && fr.ZH > 0 {
			pair(cx+fr.ZW-1, cy, cx+fr.ZW, cy)
			pair(cx-fr.ZW, cy, cx-fr.ZW+1, cy)
			pair(cx, cy+fr.ZH-1, cx, cy+fr.ZH)
			pair(cx, cy-fr.ZH, cx, cy-fr.ZH+1)
		}
		add(0, cy)
		add(cx, 0)
		add(1, cy)
	case "pixelize":
		if fr.Sq > 0 {
			block := func(x int) float64 { return math.Floor(float64(float32(float32(x) / fr.Sq))) }
			for x := 1; x < width; x++ {
				if block(x) != block(x-1) && pair(x-1, cy, x, cy) {
					break
				}
			}
		}
	case "squeezeh":
		outside := func(y int) bool { z := xfadeProbeSqueezeZ(y, height, fr.P); return z < 0 || z > 1 }
		for y := 1; y < height; y++ {
			if outside(y) != outside(y-1) {
				pair(cx, y-1, cx, y)
			}
		}
	case "radial":
		add(cx-1, 0)
		add(cx+1, 0)
		add(cx-2, 1)
	}
	return points, edges
}

func xfadeProbeHash(frame []byte) string {
	sum := sha256.Sum256(frame)
	return hex.EncodeToString(sum[:])
}

// xfadeProbeSample is one sample [x, y, r, g, b, a] of a planar gbrap frame.
func xfadeProbeSample(frame []byte, width, height, x, y int) [6]int {
	n, k := width*height, y*width+x
	return [6]int{x, y, int(frame[2*n+k]), int(frame[k]), int(frame[n+k]), int(frame[3*n+k])}
}

func xfadeProbeKey(typ, variant string, width, height, D, i int) string {
	return fmt.Sprintf("%s %s %dx%d D=%d i=%d", typ, variant, width, height, D, i)
}

// xfadeProbeSampleDiff lists the fixture samples a frame does not reproduce, for failure messages.
func xfadeProbeSampleDiff(frame []byte, row xfadeProbeRow) string {
	var parts []string
	for _, s := range row.Samples {
		got := xfadeProbeSample(frame, row.Size[0], row.Size[1], s[0], s[1])
		if got != s {
			parts = append(parts, fmt.Sprintf("(%d, %d) %v, fixture %v", s[0], s[1], got[2:], s[2:]))
		}
	}
	if len(parts) == 0 {
		return "every sample agrees; the difference lies elsewhere in the frame"
	}
	if len(parts) > 4 {
		parts = append(parts[:4], fmt.Sprintf("… %d more", len(parts)-4))
	}
	return strings.Join(parts, "; ")
}

// ffmpegVersionLine is the first line of `ffmpeg -version`, recorded in the fixture header.
func ffmpegVersionLine(t *testing.T, ffmpeg string) string {
	t.Helper()
	out, err := exec.Command(ffmpeg, "-hide_banner", "-version").Output()
	if err != nil {
		t.Fatalf("ffmpeg -version: %v", err)
	}
	first, _, _ := strings.Cut(string(out), "\n")
	return strings.TrimSpace(first)
}

// renderXfadeProbe runs xfade alone over D frames of the two probe patterns and returns the D output frames as planar
// gbrap. The setup:
//   - each input is D copies of its pattern frame, raw gbrap at 25 fps;
//   - xfade runs at offset 0 with duration D/25 (whole frames), on 4 filter threads.
func renderXfadeProbe(t *testing.T, ctx context.Context, ffmpeg, dir, xfade, variant string, width, height, D int) [][]byte {
	t.Helper()
	a, b := xfadePatterns(variant, width, height)
	size := fmt.Sprintf("%dx%d", width, height)
	args := []string{"-hide_banner", "-nostdin", "-v", "error", "-filter_threads", "4", "-filter_complex_threads", "4"}
	for k, frame := range [][]byte{a, b} {
		path := filepath.Join(dir, fmt.Sprintf("%c-%s-%s-%d.raw", 'a'+k, variant, size, D))
		if _, err := os.Stat(path); err != nil {
			if err := os.WriteFile(path, bytes.Repeat(frame, D), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		args = append(args, "-f", "rawvideo", "-pixel_format", "gbrap", "-video_size", size, "-framerate", "25", "-i", path)
	}
	args = append(args,
		"-filter_complex", fmt.Sprintf("[0:v][1:v]xfade=transition=%s:duration=%s:offset=0", xfade, strconv.FormatFloat(float64(D)/25, 'f', -1, 64)),
		"-frames:v", strconv.Itoa(D), "-f", "rawvideo", "-pix_fmt", "gbrap", "pipe:1")
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("xfade %s %s %s D=%d: %v: %s", xfade, variant, size, D, err, stderr.String())
	}
	frameSize := 4 * width * height
	if stdout.Len() != D*frameSize {
		t.Fatalf("xfade %s %s %s D=%d: %d bytes, want %d frames of %d", xfade, variant, size, D, stdout.Len(), D, frameSize)
	}
	frames := make([][]byte, D)
	for k := range frames {
		frames[k] = stdout.Bytes()[k*frameSize : (k+1)*frameSize]
	}
	return frames
}

func readXfadeProbes(t *testing.T) xfadeProbeFile {
	t.Helper()
	data, err := os.ReadFile(xfadeProbePath)
	if err != nil {
		t.Fatalf("read %s (record it with AURAGO_UPDATE_PARITY=1 on FFmpeg 8.x): %v", xfadeProbePath, err)
	}
	var file xfadeProbeFile
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatalf("decode %s: %v", xfadeProbePath, err)
	}
	if len(file.Rows) == 0 {
		t.Fatalf("%s has no rows", xfadeProbePath)
	}
	return file
}

// writeXfadeProbes records the fixture from this FFmpeg (AURAGO_UPDATE_PARITY=1). Only an FFmpeg 8.x that offers every
// registry transition may record it: that is the version the spike verified bit-exact (index §4.24).
func writeXfadeProbes(t *testing.T, ctx context.Context, ffmpeg string, major int) {
	t.Helper()
	if major != xfadeProbeMajor {
		t.Fatalf("AURAGO_UPDATE_PARITY=1: the xfade probe fixture is recorded on FFmpeg %d.x; this binary is %d", xfadeProbeMajor, major)
	}
	missing, err := ProbeCapabilities(ctx, ffmpeg)
	if err != nil {
		t.Fatalf("ProbeCapabilities: %v", err)
	}
	for _, spec := range registryTransitions {
		if slices.Contains(missing, "xfade:"+spec.XFade) {
			t.Fatalf("AURAGO_UPDATE_PARITY=1: this FFmpeg lacks xfade:%s; the fixture needs all %d transitions", spec.XFade, len(registryTransitions))
		}
	}
	file := xfadeProbeFile{
		Comment:  "xfade probe fixture (plan 2e Task 10, index §4.12, §4.24): FFmpeg's xfade alone on the Go patterns of xfade_probe_test.go. Record with AURAGO_UPDATE_PARITY=1 go test ./internal/videostudio -run '^TestXfadeProbesMatchFixture$' on FFmpeg 8.x.",
		FFmpeg:   ffmpegVersionLine(t, ffmpeg),
		Major:    major,
		Patterns: xfadeProbePatterns,
	}
	dir := t.TempDir()
	for _, spec := range registryTransitions {
		for _, combo := range xfadeProbeCombos {
			frames := renderXfadeProbe(t, ctx, ffmpeg, dir, spec.XFade, combo.variant, combo.width, combo.height, combo.duration)
			for _, i := range combo.frames {
				frame := slices.Clone(frames[i])
				xfadeProbeZeroExcluded(frame, spec.XFade, i, combo.width, combo.height)
				fr := transitionFrameAt(spec.ID, i, combo.duration, combo.width, combo.height)
				points, edges := xfadeProbePositions(spec, fr, combo.width, combo.height, i)
				row := xfadeProbeRow{Type: string(spec.ID), Size: [2]int{combo.width, combo.height}, D: combo.duration, I: i, Variant: combo.variant, SHA256: xfadeProbeHash(frame), Edges: edges}
				for _, p := range points {
					row.Samples = append(row.Samples, xfadeProbeSample(frame, combo.width, combo.height, p[0], p[1]))
				}
				file.Rows = append(file.Rows, row)
			}
		}
	}
	var buf bytes.Buffer
	buf.WriteString("{\n")
	for _, field := range []struct {
		key   string
		value any
	}{{"comment", file.Comment}, {"ffmpeg", file.FFmpeg}, {"major", file.Major}, {"patterns", file.Patterns}} {
		data, err := json.Marshal(field.value)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&buf, "  %q: %s,\n", field.key, data)
	}
	buf.WriteString("  \"rows\": [\n")
	for k, row := range file.Rows {
		line, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		buf.WriteString("    ")
		buf.Write(line)
		if k < len(file.Rows)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("  ]\n}\n")
	if err := os.MkdirAll(filepath.Dir(xfadeProbePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(xfadeProbePath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s: %d rows from %s (%d bytes)", xfadeProbePath, len(file.Rows), file.FFmpeg, buf.Len())
}

// TestXfadeProbesMatchFixture runs every fixture row through the local FFmpeg (index §4.12, §4.24):
//   - on the fixture's major version (8.x), every row is hash-exact;
//   - below it (the 5.1.9 container), radial may move values by 1 (libm atan2f): every byte within ±1 of the model,
//     at most 0.1 % of them, and every sample within ±1. Every other row is hash-exact;
//   - a row whose xfade transition ProbeCapabilities lists as missing is skipped by name. This is fadegrays wherever
//     the D2 self-test fails (5.1.9).
//
// AURAGO_UPDATE_PARITY=1 records the fixture first (writeXfadeProbes).
func TestXfadeProbesMatchFixture(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	major := ffmpegMajor(t, ffmpeg)
	if os.Getenv("AURAGO_UPDATE_PARITY") == "1" {
		writeXfadeProbes(t, ctx, ffmpeg, major)
	}
	file := readXfadeProbes(t)
	missing, err := ProbeCapabilities(ctx, ffmpeg)
	if err != nil {
		t.Fatalf("ProbeCapabilities: %v", err)
	}
	strict := major == file.Major
	type render struct {
		xfade, variant   string
		width, height, D int
	}
	dir, frames := t.TempDir(), map[render][][]byte{}
	checked, tolerated, skipped := 0, 0, map[string]bool{}
	for _, row := range file.Rows {
		key := xfadeProbeKey(row.Type, row.Variant, row.Size[0], row.Size[1], row.D, row.I)
		spec, ok := TransitionByID(TransitionType(row.Type))
		if !ok {
			t.Errorf("%s: not a registry transition", key)
			continue
		}
		if slices.Contains(missing, "xfade:"+spec.XFade) {
			skipped[spec.XFade] = true
			continue
		}
		width, height := row.Size[0], row.Size[1]
		r := render{spec.XFade, row.Variant, width, height, row.D}
		if frames[r] == nil {
			frames[r] = renderXfadeProbe(t, ctx, ffmpeg, dir, spec.XFade, row.Variant, width, height, row.D)
		}
		got := slices.Clone(frames[r][row.I])
		xfadeProbeZeroExcluded(got, spec.XFade, row.I, width, height)
		checked++
		if xfadeProbeHash(got) == row.SHA256 {
			continue
		}
		if strict || spec.XFade != "radial" {
			t.Errorf("%s: FFmpeg %d differs from the fixture (recorded on %s): %s", key, major, file.FFmpeg, xfadeProbeSampleDiff(got, row))
			continue
		}
		a, b := xfadePatterns(row.Variant, width, height)
		model := xfadeProbeModel(spec, a, b, width, height, row.I, row.D)
		differ, worst := 0, 0
		for k := range got {
			if d := absInt(int(got[k]) - int(model[k])); d > 0 {
				differ++
				worst = max(worst, d)
			}
		}
		for _, s := range row.Samples {
			v := xfadeProbeSample(got, width, height, s[0], s[1])
			for c := 2; c < 6; c++ {
				worst = max(worst, absInt(v[c]-s[c]))
			}
		}
		if worst > 1 || differ*1000 > len(got) {
			t.Errorf("%s: FFmpeg %d radial is off by up to %d in %d of %d bytes; below FFmpeg %d only ±1 in at most 0.1 %% is accepted", key, major, worst, differ, len(got), file.Major)
			continue
		}
		tolerated++
		t.Logf("%s: %d bytes ±1 against the model (radial, libm atan2f on FFmpeg %d)", key, differ, major)
	}
	for name := range skipped {
		t.Logf("%s skipped: ProbeCapabilities lists xfade:%s as missing on this FFmpeg (fadegrays: the D2 self-test)", name, name)
	}
	if checked == 0 {
		t.Fatal("no fixture row ran")
	}
	t.Logf("FFmpeg %d (fixture: %s): %d of %d rows checked, %d radial rows within ±1, strict=%v", major, file.FFmpeg, checked, len(file.Rows), tolerated, strict)
}

// TestXfadeModelMatchesFixture checks the Go float32 model (transitionFrameAt + the §4.24 per-pixel rules) against
// FFmpeg 8.1.1's recorded truth, without FFmpeg. Each row's model frame hashes like the recording and reproduces its
// samples. It also pins:
//   - the fixture's coverage: every registry transition × combo × frame, once;
//   - the header patterns, which JS rebuilds;
//   - the 128-level red separation of the opaque layers;
//   - edge samples for every hard-edged type;
//   - fadegraysModel (the D2 self-test's model) = this model on the opaque fadegrays rows.
func TestXfadeModelMatchesFixture(t *testing.T) {
	file := readXfadeProbes(t)
	if file.Major < xfadeProbeMajor {
		t.Fatalf("the fixture was recorded on FFmpeg %d (%s), want %d.x", file.Major, file.FFmpeg, xfadeProbeMajor)
	}
	if !reflect.DeepEqual(file.Patterns, xfadeProbePatterns) {
		t.Fatalf("fixture patterns %v differ from xfadeProbePatterns %v: re-record the fixture", file.Patterns, xfadeProbePatterns)
	}
	for _, combo := range xfadeProbeCombos {
		a, b := xfadePatterns("opaque", combo.width, combo.height)
		n := combo.width * combo.height
		for k := range n {
			if d := absInt(int(a[2*n+k]) - int(b[2*n+k])); d != 128 {
				t.Fatalf("%dx%d pixel %d: red differs by %d, want 128 (the GL test reads the A/B choice from it)", combo.width, combo.height, k, d)
			}
		}
	}
	want := map[string]bool{}
	for _, spec := range registryTransitions {
		for _, combo := range xfadeProbeCombos {
			for _, i := range combo.frames {
				want[xfadeProbeKey(string(spec.ID), combo.variant, combo.width, combo.height, combo.duration, i)] = true
			}
		}
	}
	edged := map[string]bool{}
	for _, row := range file.Rows {
		key := xfadeProbeKey(row.Type, row.Variant, row.Size[0], row.Size[1], row.D, row.I)
		if !want[key] {
			t.Errorf("unexpected or duplicate row %s", key)
			continue
		}
		delete(want, key)
		spec, ok := TransitionByID(TransitionType(row.Type))
		if !ok {
			t.Errorf("%s: not a registry transition", key)
			continue
		}
		if len(row.Samples) < 8 {
			t.Errorf("%s: %d samples, want at least 8", key, len(row.Samples))
		}
		if len(row.Edges) > 0 {
			edged[row.Type] = true
		}
		width, height := row.Size[0], row.Size[1]
		a, b := xfadePatterns(row.Variant, width, height)
		model := xfadeProbeModel(spec, a, b, width, height, row.I, row.D)
		if xfadeProbeHash(model) != row.SHA256 {
			t.Errorf("%s: the Go model (transitionFrameAt + §4.24) differs from FFmpeg %d: %s", key, file.Major, xfadeProbeSampleDiff(model, row))
		}
		if spec.XFade == "fadegrays" && row.Variant == "opaque" {
			n := width * height
			frames := fadegraysModel(a[:3*n], b[:3*n], width, height, row.D)
			if len(frames) != row.D || !bytes.Equal(frames[row.I], model[:3*n]) {
				t.Errorf("%s: fadegraysModel (the D2 self-test's model) differs from the probe model", key)
			}
		}
	}
	for key := range want {
		t.Errorf("missing row %s", key)
	}
	for _, typ := range xfadeProbeEdgedTypes {
		if !edged[typ] {
			t.Errorf("no %s row has edge samples", typ)
		}
	}
}
