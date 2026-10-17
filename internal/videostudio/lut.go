package videostudio

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

// 3D LUTs (spec §5.7, plan index §4.20). This file holds:
//   - the limits (ParseCube never reads more than MaxLUTBytes+1 bytes; MaxLUTAssetsPerProject also counts pending
//     .cube imports);
//   - the .cube parser that every import, probe and render goes through;
//   - trilinear sampling;
//   - the writer that stages tables for FFmpeg's lut3d;
//   - the eight built-in looks.
//
// FFmpeg never reads a user's file: a render stages a Go-written copy of the parsed numbers (writeStagedLUT, render.go).
// The JS mirror is VideoStudioPicture.parseCube and sampleLUT (parity topic "lut").

const (
	// MaxLUTBytes bounds an uploaded .cube file.
	MaxLUTBytes = 16 << 20
	// MinLUTGrid and MaxLUTGrid bound the LUT_3D_SIZE of a .cube file (Asset.LUTSize).
	MinLUTGrid = 2
	MaxLUTGrid = 65
	// MaxLUTAssetsPerProject bounds the lut assets of one project.
	MaxLUTAssetsPerProject = 32
	// BuiltinLookGrid is the grid size of the generated built-in looks.
	BuiltinLookGrid = 33
)

// The .cube grammar's limits (index §4.20) and the prefix of built-in LUT refs.
const (
	maxCubeLineBytes  = 1024
	maxCubeTitleBytes = 256
	maxCubeValue      = 16
	builtinLUTPrefix  = "builtin:"
)

// LUT3D is a parsed 3D table. Data holds Size³ RGB triplets, red fastest: entry (r, g, b) starts at
// ((b·Size+g)·Size+r)·3. Each input channel is clamped to [DomainMin, DomainMax] ([0, 1] by default) before lookup.
// Tables returned by BuiltinLook are shared and must not be modified.
type LUT3D struct {
	Size                 int
	DomainMin, DomainMax [3]float64
	Data                 []float32
}

// LUTError is a .cube file the parser refuses. Line is 1-based; a problem found at the end of the file names the line
// after the last. Reason is fixed English text and never quotes the file.
type LUTError struct {
	Line   int
	Reason string
}

func (e *LUTError) Error() string { return fmt.Sprintf("LUT line %d: %s", e.Line, e.Reason) }

func lutError(line int, reason string) error { return &LUTError{Line: line, Reason: reason} }

// lutByteCounter counts the bytes ParseCube read, so the size limit holds while reading.
type lutByteCounter struct {
	reader io.Reader
	read   int64
}

func (c *lutByteCounter) Read(p []byte) (int, error) {
	n, err := c.reader.Read(p)
	c.read += int64(n)
	return n, err
}

// ParseCube reads a .cube table and enforces the rules of index §4.20 while it reads:
//   - At most MaxLUTBytes+1 bytes are read. Lines hold at most 1024 bytes (the scanner cannot buffer more) and are
//     UTF-8, with LF or CRLF; a leading BOM is skipped.
//   - Blank lines and # comments are skipped. Fields are separated by spaces and tabs.
//   - Keywords: TITLE (once, at most 256 bytes) and LUT_3D_SIZE (once, 2..65); DOMAIN_MIN and DOMAIN_MAX, or
//     LUT_3D_INPUT_RANGE lo hi (once, finite, |v| <= 16, min < max on every channel). All keywords come before the
//     data; LUT_1D_* and any other keyword are refused.
//   - Data lines have exactly three decimal numbers within ±16. There are exactly Size³ of them, and reading stops at
//     the first line past the grid.
//
// Content problems are *LUTError; a failing reader gives a wrapped I/O error.
func ParseCube(r io.Reader) (*LUT3D, error) {
	counter := &lutByteCounter{reader: io.LimitReader(r, MaxLUTBytes+1)}
	scanner := bufio.NewScanner(counter)
	scanner.Buffer(make([]byte, 0, maxCubeLineBytes+2), maxCubeLineBytes+2)
	p := cubeParser{domainMax: [3]float64{1, 1, 1}}
	line := 0
	for scanner.Scan() {
		line++
		if counter.read > MaxLUTBytes {
			return nil, lutError(line, "the file is larger than 16 MiB")
		}
		if err := p.line(line, scanner.Bytes()); err != nil {
			return nil, err
		}
	}
	if counter.read > MaxLUTBytes {
		return nil, lutError(line+1, "the file is larger than 16 MiB")
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return nil, lutError(line+1, "line longer than 1024 bytes")
		}
		return nil, fmt.Errorf("read LUT: %w", err)
	}
	return p.finish(line + 1)
}

// cubeParser holds the state of one ParseCube run.
type cubeParser struct {
	size, count           int
	title, minSet, maxSet bool
	domainChecked         bool
	domainLine            int
	domainMin, domainMax  [3]float64
	data                  []float32
}

func (p *cubeParser) line(number int, raw []byte) error {
	if number == 1 {
		raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))
	}
	if len(raw) > maxCubeLineBytes {
		return lutError(number, "line longer than 1024 bytes")
	}
	if !utf8.Valid(raw) {
		return lutError(number, "the text is not UTF-8")
	}
	text := strings.Trim(string(raw), " \t")
	if text == "" || text[0] == '#' {
		return nil
	}
	fields := strings.FieldsFunc(text, func(r rune) bool { return r == ' ' || r == '\t' })
	if first := fields[0][0]; first >= 'A' && first <= 'Z' || first >= 'a' && first <= 'z' {
		return p.keyword(number, text, fields)
	}
	return p.entry(number, fields)
}

func (p *cubeParser) keyword(number int, text string, fields []string) error {
	if p.count > 0 {
		return lutError(number, "keyword after the data")
	}
	switch fields[0] {
	case "TITLE":
		if p.title {
			return lutError(number, "TITLE appears twice")
		}
		p.title = true
		if len(strings.TrimLeft(text[len("TITLE"):], " \t")) > maxCubeTitleBytes {
			return lutError(number, "TITLE longer than 256 bytes")
		}
		return nil
	case "LUT_3D_SIZE":
		if p.size != 0 {
			return lutError(number, "LUT_3D_SIZE appears twice")
		}
		if len(fields) != 2 || !cubeInteger(fields[1]) {
			return lutError(number, "LUT_3D_SIZE needs one whole number")
		}
		size, err := strconv.Atoi(fields[1])
		if err != nil || size < MinLUTGrid || size > MaxLUTGrid {
			return lutError(number, "LUT_3D_SIZE must be 2 to 65")
		}
		p.size = size
		p.data = make([]float32, 0, 3*size*size*size)
		return nil
	case "DOMAIN_MIN", "DOMAIN_MAX":
		isMin := fields[0] == "DOMAIN_MIN"
		if isMin && p.minSet || !isMin && p.maxSet {
			return lutError(number, "the input domain is given twice")
		}
		var values [3]float64
		if err := cubeNumbers(number, fields[1:], values[:]); err != nil {
			return err
		}
		if isMin {
			p.domainMin, p.minSet = values, true
		} else {
			p.domainMax, p.maxSet = values, true
		}
		p.domainLine = number
		return nil
	case "LUT_3D_INPUT_RANGE":
		if p.minSet || p.maxSet {
			return lutError(number, "the input domain is given twice")
		}
		var values [2]float64
		if err := cubeNumbers(number, fields[1:], values[:]); err != nil {
			return err
		}
		p.domainMin = [3]float64{values[0], values[0], values[0]}
		p.domainMax = [3]float64{values[1], values[1], values[1]}
		p.minSet, p.maxSet, p.domainLine = true, true, number
		return nil
	case "LUT_1D_SIZE", "LUT_1D_INPUT_RANGE":
		return lutError(number, "1D LUTs are not supported")
	}
	return lutError(number, "unknown keyword")
}

// checkDomain refuses a domain whose minimum is not below its maximum on every channel. The error names the line of
// the last domain keyword.
func (p *cubeParser) checkDomain() error {
	p.domainChecked = true
	for c := range 3 {
		if !(p.domainMin[c] < p.domainMax[c]) {
			return lutError(p.domainLine, "DOMAIN_MIN must be below DOMAIN_MAX")
		}
	}
	return nil
}

func (p *cubeParser) entry(number int, fields []string) error {
	if p.size == 0 {
		return lutError(number, "data before LUT_3D_SIZE")
	}
	if !p.domainChecked {
		if err := p.checkDomain(); err != nil {
			return err
		}
	}
	if p.count == p.size*p.size*p.size {
		return lutError(number, "more data lines than the grid holds")
	}
	var values [3]float64
	if err := cubeNumbers(number, fields, values[:]); err != nil {
		return err
	}
	p.data = append(p.data, float32(values[0]), float32(values[1]), float32(values[2]))
	p.count++
	return nil
}

func (p *cubeParser) finish(end int) (*LUT3D, error) {
	if p.size == 0 {
		return nil, lutError(end, "missing LUT_3D_SIZE")
	}
	if !p.domainChecked {
		if err := p.checkDomain(); err != nil {
			return nil, err
		}
	}
	if want := p.size * p.size * p.size; p.count != want {
		return nil, lutError(end, fmt.Sprintf("expected %d data lines, found %d", want, p.count))
	}
	return &LUT3D{Size: p.size, DomainMin: p.domainMin, DomainMax: p.domainMax, Data: p.data}, nil
}

// cubeNumbers reads exactly len(out) decimal numbers within ±16 from fields.
func cubeNumbers(number int, fields []string, out []float64) error {
	if len(fields) != len(out) {
		return lutError(number, fmt.Sprintf("expected %d numbers", len(out)))
	}
	for i, field := range fields {
		if !cubeNumber(field) {
			return lutError(number, "invalid number")
		}
		// The grammar admits decimal syntax only; an overflow parses to ±Inf and is refused below.
		value, _ := strconv.ParseFloat(field, 64)
		if !finite(value) || math.Abs(value) > maxCubeValue {
			return lutError(number, "number outside -16..16")
		}
		out[i] = value
	}
	return nil
}

// cubeNumber reports whether s is a decimal number:
//   - an optional sign;
//   - digits with an optional fraction, or a fraction alone;
//   - an optional exponent.
//
// It rejects NaN, Inf, hexadecimal floats and underscores. The JS mirror uses /^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$/.
func cubeNumber(s string) bool {
	digit := func(i int) bool { return i < len(s) && s[i] >= '0' && s[i] <= '9' }
	i, digits := 0, 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	for ; digit(i); i++ {
		digits++
	}
	if i < len(s) && s[i] == '.' {
		for i++; digit(i); i++ {
			digits++
		}
	}
	if digits == 0 {
		return false
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		exponent := 0
		for ; digit(i); i++ {
			exponent++
		}
		if exponent == 0 {
			return false
		}
	}
	return i == len(s)
}

// cubeInteger reports whether s is one or more decimal digits.
func cubeInteger(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// Sample is the trilinear value of the table at rgb (index §4.20). Each channel is clamped to the domain (NaN reads as
// the domain minimum) and mapped onto the grid. The intensity mix is the caller's: (1−k)·in + k·Sample(in).
// JS: VideoStudioPicture.sampleLUT.
func (l *LUT3D) Sample(rgb [3]float64) [3]float64 {
	n := l.Size
	var base [3]int
	var frac [3]float64
	for c := range 3 {
		low, high := l.DomainMin[c], l.DomainMax[c]
		v := rgb[c]
		if !(v > low) {
			v = low
		} else if v > high {
			v = high
		}
		position := (v - low) / (high - low) * float64(n-1)
		index := int(math.Floor(position))
		if index > n-2 {
			index = n - 2
		}
		base[c], frac[c] = index, position-float64(index)
	}
	at := func(r, g, b, c int) float64 { return float64(l.Data[((b*n+g)*n+r)*3+c]) }
	lerp := func(a, b, t float64) float64 { return a + (b-a)*t }
	r, g, b := base[0], base[1], base[2]
	var out [3]float64
	for c := range 3 {
		c00 := lerp(at(r, g, b, c), at(r+1, g, b, c), frac[0])
		c10 := lerp(at(r, g+1, b, c), at(r+1, g+1, b, c), frac[0])
		c01 := lerp(at(r, g, b+1, c), at(r+1, g, b+1, c), frac[0])
		c11 := lerp(at(r, g+1, b+1, c), at(r+1, g+1, b+1, c), frac[0])
		out[c] = lerp(lerp(c00, c10, frac[1]), lerp(c01, c11, frac[1]), frac[2])
	}
	return out
}

// WriteCube writes a table in the .cube form that FFmpeg's lut3d and ParseCube read:
//   - LUT_3D_SIZE;
//   - DOMAIN_MIN and DOMAIN_MAX only when the domain is not [0, 1], with Go's shortest exact numbers;
//   - one line per entry with three %.6f numbers.
//
// It writes no TITLE and no comment, and it refuses a table that ParseCube would refuse.
func WriteCube(w io.Writer, l *LUT3D) error {
	if err := checkLUT(l); err != nil {
		return err
	}
	out := bufio.NewWriterSize(w, 64<<10)
	fmt.Fprintf(out, "LUT_3D_SIZE %d\n", l.Size)
	if l.DomainMin != [3]float64{0, 0, 0} || l.DomainMax != [3]float64{1, 1, 1} {
		fmt.Fprintf(out, "DOMAIN_MIN %s %s %s\n", domainNumber(l.DomainMin[0]), domainNumber(l.DomainMin[1]), domainNumber(l.DomainMin[2]))
		fmt.Fprintf(out, "DOMAIN_MAX %s %s %s\n", domainNumber(l.DomainMax[0]), domainNumber(l.DomainMax[1]), domainNumber(l.DomainMax[2]))
	}
	line := make([]byte, 0, 40)
	for i := 0; i < len(l.Data); i += 3 {
		line = appendCubeNumber(line[:0], float64(l.Data[i]))
		line = append(line, ' ')
		line = appendCubeNumber(line, float64(l.Data[i+1]))
		line = append(line, ' ')
		line = appendCubeNumber(line, float64(l.Data[i+2]))
		line = append(line, '\n')
		if _, err := out.Write(line); err != nil {
			return err
		}
	}
	return out.Flush()
}

// checkLUT refuses tables that are incomplete, out of range or that have a domain that is not ordered.
func checkLUT(l *LUT3D) error {
	if l == nil || l.Size < MinLUTGrid || l.Size > MaxLUTGrid || len(l.Data) != 3*l.Size*l.Size*l.Size {
		return fmt.Errorf("LUT table is incomplete")
	}
	for c := range 3 {
		low, high := l.DomainMin[c], l.DomainMax[c]
		if !finite(low) || !finite(high) || math.Abs(low) > maxCubeValue || math.Abs(high) > maxCubeValue || !(low < high) {
			return fmt.Errorf("LUT table has an invalid input domain")
		}
	}
	for _, v := range l.Data {
		if value := float64(v); !finite(value) || math.Abs(value) > maxCubeValue {
			return fmt.Errorf("LUT table holds a value outside -16..16")
		}
	}
	return nil
}

// appendCubeNumber appends v as %.6f; "-0.000000" is written as "0.000000".
func appendCubeNumber(buf []byte, v float64) []byte {
	start := len(buf)
	buf = strconv.AppendFloat(buf, v, 'f', 6, 64)
	if string(buf[start:]) == "-0.000000" {
		buf = append(buf[:start], "0.000000"...)
	}
	return buf
}

// domainNumber writes a domain bound exactly (shortest form, so parsing it back gives the same float64).
func domainNumber(v float64) string {
	if v == 0 {
		v = 0
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// unitDomainLUT returns l when its domain is [0, 1] on every channel. Otherwise it returns the table resampled with
// Sample on the same grid over [0, 1]. Staged LUTs are written this way (index §4.20): FFmpeg's lut3d then needs no
// DOMAIN handling, and the 8-bit inputs it sees all lie in [0, 1].
func unitDomainLUT(l *LUT3D) *LUT3D {
	if l.DomainMin == [3]float64{0, 0, 0} && l.DomainMax == [3]float64{1, 1, 1} {
		return l
	}
	n := l.Size
	out := &LUT3D{Size: n, DomainMax: [3]float64{1, 1, 1}, Data: make([]float32, 0, len(l.Data))}
	step := 1 / float64(n-1)
	for b := range n {
		for g := range n {
			for r := range n {
				v := l.Sample([3]float64{float64(r) * step, float64(g) * step, float64(b) * step})
				out.Data = append(out.Data, float32(v[0]), float32(v[1]), float32(v[2]))
			}
		}
	}
	return out
}

// LoadLUTFile parses a regular local .cube file. A symlink or a file above MaxLUTBytes is refused before reading.
func LoadLUTFile(path string) (*LUT3D, error) {
	table, _, err := loadLUT(path)
	return table, err
}

// InspectLUTFile parses a staged .cube file and describes it as a lut asset {kind, lut_size, size_bytes} for the
// import's probe job.
func InspectLUTFile(path string) (Asset, error) {
	abs, err := ensureRegularLocalFile(path)
	if err != nil {
		return Asset{}, fmt.Errorf("open LUT file: %w", err)
	}
	table, size, err := loadLUT(abs)
	if err != nil {
		return Asset{}, err
	}
	return Asset{Name: filepath.Base(abs), Kind: AssetLUT, LUTSize: table.Size, SizeBytes: size}, nil
}

func loadLUT(path string) (*LUT3D, int64, error) {
	abs, err := ensureRegularLocalFile(path)
	if err != nil {
		return nil, 0, fmt.Errorf("open LUT file: %w", err)
	}
	file, err := os.Open(abs)
	if err != nil {
		return nil, 0, fmt.Errorf("open LUT file: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, 0, fmt.Errorf("open LUT file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, 0, fmt.Errorf("open LUT file: not a regular file")
	}
	if info.Size() > MaxLUTBytes {
		return nil, 0, lutError(1, "the file is larger than 16 MiB")
	}
	table, err := ParseCube(bufio.NewReaderSize(file, 64<<10))
	if err != nil {
		return nil, 0, err
	}
	return table, info.Size(), nil
}

// IsLUTFileName reports a .cube file name (the extension is case-insensitive).
func IsLUTFileName(name string) bool { return strings.EqualFold(filepath.Ext(name), ".cube") }

// ParseLUTRef splits a lut effect's "lut" parameter (index §4.2):
//   - "" (no table) gives ok with nothing else;
//   - "builtin:<id>" gives the look id when it is a registry look;
//   - any other valid identifier is a project asset id. Validate checks that the asset exists and is of kind lut.
func ParseLUTRef(ref string) (builtin, assetID string, ok bool) {
	if ref == "" {
		return "", "", true
	}
	if id, found := strings.CutPrefix(ref, builtinLUTPrefix); found {
		if _, known := LookByID(id); known {
			return id, "", true
		}
		return "", "", false
	}
	if !validIdentifier(ref) {
		return "", "", false
	}
	return "", ref, true
}

// lookWheel is the colour-wheel position at radius and angle (degrees): x = r·cos θ, y = r·sin θ (plan decision 2).
func lookWheel(radius, degrees float64) (x, y float64) {
	theta := degrees * math.Pi / 180
	return radius * math.Cos(theta), radius * math.Sin(theta)
}

// builtinLookParams are the fixed colour parameters of the built-in looks (index §4.20). The ids are the registry's.
func builtinLookParams(id string) (colorParams, bool) {
	p := defaultColorParams()
	switch id {
	case "warm":
		p.Temperature, p.Tint, p.Saturation = 7600, 0.05, 1.05
	case "cool":
		p.Temperature, p.Tint, p.Saturation = 5400, -0.05, 0.97
	case "teal_orange":
		p.Contrast, p.Saturation = 1.1, 1.15
		p.LiftX, p.LiftY = lookWheel(0.6, 190)
		p.GainX, p.GainY = lookWheel(0.5, 30)
	case "mono":
		p.Saturation, p.Contrast = 0, 1.05
	case "faded_film":
		p.Contrast, p.Lift, p.Gain, p.Saturation, p.Temperature = 0.85, 0.06, 0.95, 0.8, 6900
	case "high_contrast":
		p.Contrast, p.Saturation = 1.4, 1.1
	case "vintage":
		p.Temperature, p.Saturation, p.Contrast, p.Lift = 7300, 0.75, 0.95, 0.04
		p.LiftX, p.LiftY = lookWheel(0.35, 75)
		p.GainX, p.GainY = lookWheel(0.25, 45)
	case "bleach_bypass":
		p.Saturation, p.Contrast, p.Gamma = 0.4, 1.35, 0.95
	default:
		return colorParams{}, false
	}
	return p, true
}

// builtinLookEntry is one generated look: its table and its .cube bytes (the bytes WriteCube writes for the table).
type builtinLookEntry struct {
	lut  *LUT3D
	cube []byte
}

// builtinLooks remembers every look generated in this process; a look is a pure function of its id.
var builtinLooks = struct {
	sync.Mutex
	byID map[string]builtinLookEntry
}{byID: map[string]builtinLookEntry{}}

func builtinLook(id string) (builtinLookEntry, bool) {
	if _, known := LookByID(id); !known {
		return builtinLookEntry{}, false
	}
	params, ok := builtinLookParams(id)
	if !ok {
		return builtinLookEntry{}, false
	}
	builtinLooks.Lock()
	defer builtinLooks.Unlock()
	if entry, ok := builtinLooks.byID[id]; ok {
		return entry, true
	}
	const n = BuiltinLookGrid
	var text bytes.Buffer
	text.Grow(n*n*n*27 + 16)
	fmt.Fprintf(&text, "LUT_3D_SIZE %d\n", n)
	step := 1 / float64(n-1)
	line := make([]byte, 0, 40)
	for b := range n {
		for g := range n {
			for r := range n {
				out := applyColor(params, [3]float64{float64(r) * step, float64(g) * step, float64(b) * step})
				line = appendCubeNumber(line[:0], out[0])
				line = append(line, ' ')
				line = appendCubeNumber(line, out[1])
				line = append(line, ' ')
				line = appendCubeNumber(line, out[2])
				text.Write(append(line, '\n'))
			}
		}
	}
	// Parsing the text gives the table every reader of the served .cube gets, so preview and export use one table.
	table, err := ParseCube(bytes.NewReader(text.Bytes()))
	if err != nil {
		return builtinLookEntry{}, false
	}
	entry := builtinLookEntry{lut: table, cube: text.Bytes()}
	builtinLooks.byID[id] = entry
	return entry, true
}

// BuiltinLook returns a built-in look's table: the §4.20 colour formula with the look's fixed parameters, sampled on a
// 33³ grid of [0, 1]³. The table is shared; do not modify it.
func BuiltinLook(id string) (*LUT3D, bool) {
	entry, ok := builtinLook(id)
	return entry.lut, ok
}

// BuiltinLookCube returns a copy of a built-in look's .cube bytes (the built-in endpoint serves them).
func BuiltinLookCube(id string) ([]byte, bool) {
	entry, ok := builtinLook(id)
	if !ok {
		return nil, false
	}
	return bytes.Clone(entry.cube), true
}
