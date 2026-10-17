package videostudio

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// lutTestCube writes a .cube text with size³ entries, red fastest. value maps an entry's grid coordinates (0..1) to its
// output; numbers carry six decimals and every line ends in newline. Later tasks' tests reuse it.
func lutTestCube(size int, value func(r, g, b float64) [3]float64, newline string) string {
	var text strings.Builder
	fmt.Fprintf(&text, "LUT_3D_SIZE %d%s", size, newline)
	step := 1 / float64(size-1)
	for b := range size {
		for g := range size {
			for r := range size {
				v := value(float64(r)*step, float64(g)*step, float64(b)*step)
				fmt.Fprintf(&text, "%s %s %s%s", strconv.FormatFloat(v[0], 'f', 6, 64), strconv.FormatFloat(v[1], 'f', 6, 64), strconv.FormatFloat(v[2], 'f', 6, 64), newline)
			}
		}
	}
	return text.String()
}

func lutIdentity(r, g, b float64) [3]float64 { return [3]float64{r, g, b} }

// The parser rules of index §4.20. Every refusal is a *LUTError on the given line. A problem found at the end of the
// file names the line after the last one, and a domain problem names the last domain keyword.
func TestParseCubeRules(t *testing.T) {
	identity := lutTestCube(2, lutIdentity, "\n")
	data := strings.TrimPrefix(identity, "LUT_3D_SIZE 2\n")
	seven := identity[:strings.LastIndex(strings.TrimSuffix(identity, "\n"), "\n")+1]
	for _, c := range []struct {
		name, text string
		line       int // 0: accepted
	}{
		{"LF", identity, 0},
		{"CRLF, BOM, comments, blank lines, title and domain", "\xef\xbb\xbf# comment\r\n\r\nTITLE \"look\"\r\nLUT_3D_SIZE 2\r\nDOMAIN_MIN 0 0 0\r\nDOMAIN_MAX 1 1 1\r\n" + strings.ReplaceAll(data, "\n", "\r\n") + "# end\r\n", 0},
		{"tabs and spaces", "\tLUT_3D_SIZE \t 2 \n" + strings.ReplaceAll(data, " ", "\t "), 0},
		{"input range", "LUT_3D_INPUT_RANGE -0.5 2\n" + identity, 0},
		{"values at ±16", "LUT_3D_SIZE 2\n" + strings.Repeat("16 -16 +.5e1\n", 8), 0},
		{"grid 65", lutTestCube(65, lutIdentity, "\n"), 0},
		{"1024-byte comment", "#" + strings.Repeat("x", 1023) + "\n" + identity, 0},
		{"1024-byte comment with CRLF", "#" + strings.Repeat("x", 1023) + "\r\n" + identity, 0},
		{"256-byte title", "TITLE " + strings.Repeat("t", 256) + "\n" + identity, 0},
		{"empty file", "", 1},
		{"grid 1", "LUT_3D_SIZE 1\n", 1},
		{"grid 66", "LUT_3D_SIZE 66\n", 1},
		{"grid not a whole number", "LUT_3D_SIZE 2.0\n", 1},
		{"grid without a number", "LUT_3D_SIZE\n", 1},
		{"1D", "LUT_1D_SIZE 16\n", 1},
		{"1D input range", "LUT_1D_INPUT_RANGE 0 1\n", 1},
		{"missing size", "# only a comment\n", 2},
		{"data before the size", "0 0 0\nLUT_3D_SIZE 2\n", 1},
		{"seven data lines", seven, 9},
		{"nine data lines", identity + "1 1 1\n", 10},
		{"NaN", "LUT_3D_SIZE 2\n0 NaN 0\n", 2},
		{"Inf", "LUT_3D_SIZE 2\ninf 0 0\n", 2},
		{"overflow", "LUT_3D_SIZE 2\n1e999 0 0\n", 2},
		{"above 16", "LUT_3D_SIZE 2\n0 16.000001 0\n", 2},
		{"below -16", "LUT_3D_SIZE 2\n-17 0 0\n", 2},
		{"two numbers", "LUT_3D_SIZE 2\n0 0\n", 2},
		{"four numbers", "LUT_3D_SIZE 2\n0 0 0 0\n", 2},
		{"comma separated", "LUT_3D_SIZE 2\n0,0,0\n", 2},
		{"hexadecimal float", "LUT_3D_SIZE 2\n0x1p-2 0 0\n", 2},
		{"inline comment", "LUT_3D_SIZE 2\n0 0 0 # x\n", 2},
		{"domain min not below max", "LUT_3D_SIZE 2\nDOMAIN_MIN 0 0 0\nDOMAIN_MAX 1 0 1\n" + data, 3},
		{"domain min above max, no data", "DOMAIN_MIN 2 2 2\nLUT_3D_SIZE 2\n", 1},
		{"domain value above 16", "DOMAIN_MAX 17 1 1\n", 1},
		{"domain twice", "DOMAIN_MIN 0 0 0\nLUT_3D_INPUT_RANGE 0 1\n", 2},
		{"domain with two numbers", "DOMAIN_MIN 0 0\n", 1},
		{"unknown keyword", "LUT_3D_SIZE 2\nLUT_3D_FOO 1\n", 2},
		{"lowercase keyword", "lut_3d_size 2\n", 1},
		{"size twice", "LUT_3D_SIZE 2\nLUT_3D_SIZE 2\n", 2},
		{"keyword after the data", "LUT_3D_SIZE 2\n0 0 0\nTITLE \"late\"\n", 3},
		{"title twice", "TITLE a\nTITLE b\n", 2},
		{"257-byte title", "TITLE " + strings.Repeat("t", 257) + "\n", 1},
		{"1025-byte line", "LUT_3D_SIZE 2\n#" + strings.Repeat("x", 1024) + "\n", 2},
		{"line beyond the scanner buffer", "LUT_3D_SIZE 2\n#" + strings.Repeat("x", 4000) + "\n", 2},
		{"not UTF-8", "LUT_3D_SIZE 2\n# \xff\n", 2},
		{"CRLF, too few lines", "LUT_3D_SIZE 2\r\n0 0 0\r\n", 3},
	} {
		table, err := ParseCube(strings.NewReader(c.text))
		if c.line == 0 {
			if err != nil || table == nil {
				t.Errorf("%s: refused: %v", c.name, err)
			}
			continue
		}
		var lutErr *LUTError
		if !errors.As(err, &lutErr) || lutErr.Line != c.line || lutErr.Reason == "" || table != nil {
			t.Errorf("%s: error %v (table %v), want a LUTError on line %d", c.name, err, table != nil, c.line)
		}
	}
	// Accepted tables: red runs fastest, and the domain comes from the keywords.
	parsed, err := ParseCube(strings.NewReader("LUT_3D_INPUT_RANGE -0.5 2\n" + identity))
	if err != nil || parsed.Size != 2 || parsed.DomainMin != [3]float64{-0.5, -0.5, -0.5} || parsed.DomainMax != [3]float64{2, 2, 2} ||
		!reflect.DeepEqual(parsed.Data, []float32{0, 0, 0, 1, 0, 0, 0, 1, 0, 1, 1, 0, 0, 0, 1, 1, 0, 1, 0, 1, 1, 1, 1, 1}) {
		t.Fatalf("input range table = %+v, %v", parsed, err)
	}
	crlf, err := ParseCube(strings.NewReader("\xef\xbb\xbfLUT_3D_SIZE 2\r\n" + strings.ReplaceAll(data, "\n", "\r\n")))
	if err != nil || !reflect.DeepEqual(crlf.Data, parsed.Data) || crlf.DomainMin != [3]float64{} || crlf.DomainMax != [3]float64{1, 1, 1} {
		t.Fatalf("CRLF table = %+v, %v", crlf, err)
	}
	big, err := ParseCube(strings.NewReader(lutTestCube(65, lutIdentity, "\n")))
	if err != nil || big.Size != 65 || len(big.Data) != 3*65*65*65 || big.Data[3] != float32(1.0/64) {
		t.Fatalf("grid 65: size %d, %d numbers, %v", big.Size, len(big.Data), err)
	}
}

// lutCountingReader counts the bytes ParseCube pulled from its source.
type lutCountingReader struct {
	reader io.Reader
	read   int64
}

func (r *lutCountingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.read += int64(n)
	return n, err
}

// lutRepeatReader repeats chunk forever.
type lutRepeatReader struct {
	chunk  []byte
	offset int
}

func (r *lutRepeatReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = r.chunk[r.offset]
		r.offset = (r.offset + 1) % len(r.chunk)
	}
	return len(p), nil
}

// The limits hold while ParseCube reads. An endless stream of comments stops after MaxLUTBytes+1 bytes, and endless
// data lines stop at the first line past the grid, long before the size limit.
func TestParseCubeStopsAtLimit(t *testing.T) {
	comments := &lutCountingReader{reader: io.MultiReader(strings.NewReader("LUT_3D_SIZE 2\n"), &lutRepeatReader{chunk: []byte("# padding\n")})}
	_, err := ParseCube(comments)
	var lutErr *LUTError
	if !errors.As(err, &lutErr) || !strings.Contains(lutErr.Reason, "16 MiB") || lutErr.Line < 2 {
		t.Fatalf("endless comments: %v", err)
	}
	if comments.read > MaxLUTBytes+1 {
		t.Fatalf("ParseCube read %d bytes, the limit is %d", comments.read, MaxLUTBytes+1)
	}
	entries := &lutCountingReader{reader: io.MultiReader(strings.NewReader("LUT_3D_SIZE 2\n"), &lutRepeatReader{chunk: []byte("0 0 0\n")})}
	if _, err := ParseCube(entries); !errors.As(err, &lutErr) || lutErr.Line != 10 {
		t.Fatalf("endless data lines: %v, want a LUTError on line 10", err)
	}
	if entries.read > 4096 {
		t.Fatalf("endless data lines: ParseCube read %d bytes before it stopped", entries.read)
	}
}

func TestWriteCubeRoundTrip(t *testing.T) {
	values := func(r, g, b float64) [3]float64 { return [3]float64{r*r - 0.123456, g, 15.5 * b} }
	text := "TITLE \"round trip\"\nLUT_3D_SIZE 3\nDOMAIN_MIN -0.5 0 0.25\nDOMAIN_MAX 1.5 2 0.75\n" + strings.TrimPrefix(lutTestCube(3, values, "\n"), "LUT_3D_SIZE 3\n")
	original, err := ParseCube(strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	var first bytes.Buffer
	if err := WriteCube(&first, original); err != nil {
		t.Fatal(err)
	}
	written := first.String()
	// No title, the size first, the domain in Go's shortest exact form, then %.6f triplets.
	if !strings.HasPrefix(written, "LUT_3D_SIZE 3\nDOMAIN_MIN -0.5 0 0.25\nDOMAIN_MAX 1.5 2 0.75\n-0.123456 0.000000 0.000000\n0.126544 0.000000 0.000000\n") || strings.Contains(written, "TITLE") {
		t.Fatalf("written cube:\n%s", written[:min(len(written), 200)])
	}
	parsed, err := ParseCube(strings.NewReader(written))
	if err != nil || !reflect.DeepEqual(parsed, original) {
		t.Fatalf("parse back: %v, equal %v", err, reflect.DeepEqual(parsed, original))
	}
	var second bytes.Buffer
	if err := WriteCube(&second, parsed); err != nil || second.String() != written {
		t.Fatalf("writing again changed the text (%v)", err)
	}
	// A unit domain writes no DOMAIN lines, and negative zero is written as 0.000000.
	unit := &LUT3D{Size: 2, DomainMax: [3]float64{1, 1, 1}, Data: make([]float32, 24)}
	unit.Data[0] = float32(math.Copysign(0, -1))
	var plain bytes.Buffer
	if err := WriteCube(&plain, unit); err != nil || !strings.HasPrefix(plain.String(), "LUT_3D_SIZE 2\n0.000000 0.000000 0.000000\n") || strings.Contains(plain.String(), "DOMAIN") {
		t.Fatalf("unit cube (%v):\n%s", err, plain.String())
	}
	// WriteCube refuses what ParseCube would refuse.
	for name, bad := range map[string]*LUT3D{
		"nil":         nil,
		"grid 1":      {Size: 1, DomainMax: [3]float64{1, 1, 1}, Data: make([]float32, 3)},
		"short data":  {Size: 2, DomainMax: [3]float64{1, 1, 1}, Data: make([]float32, 21)},
		"NaN":         {Size: 2, DomainMax: [3]float64{1, 1, 1}, Data: append(make([]float32, 23), float32(math.NaN()))},
		"above 16":    {Size: 2, DomainMax: [3]float64{1, 1, 1}, Data: append(make([]float32, 23), 17)},
		"flat domain": {Size: 2, DomainMin: [3]float64{1, 0, 0}, DomainMax: [3]float64{1, 1, 1}, Data: make([]float32, 24)},
	} {
		if err := WriteCube(io.Discard, bad); err == nil {
			t.Errorf("%s: written", name)
		}
	}
	// unitDomainLUT keeps a unit-domain table and resamples any other domain on the same grid over [0, 1].
	if unitDomainLUT(unit) != unit {
		t.Fatal("a unit-domain table was copied")
	}
	wide, err := ParseCube(strings.NewReader("LUT_3D_INPUT_RANGE 0 2\n" + lutTestCube(2, func(r, g, b float64) [3]float64 { return [3]float64{2 * r, 2 * g, 2 * b} }, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	resampled := unitDomainLUT(wide)
	identity, _ := ParseCube(strings.NewReader(lutTestCube(2, lutIdentity, "\n")))
	if resampled.Size != 2 || resampled.DomainMin != [3]float64{} || resampled.DomainMax != [3]float64{1, 1, 1} || !reflect.DeepEqual(resampled.Data, identity.Data) {
		t.Fatalf("resampled = %+v, want the unit identity", resampled)
	}
}

// Sample clamps every channel to the domain; NaN reads as the domain minimum (JS sampleLUT does the same).
func TestLUTSampleClamps(t *testing.T) {
	identity, err := ParseCube(strings.NewReader(lutTestCube(2, lutIdentity, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	if got := identity.Sample([3]float64{math.NaN(), 0.5, 2}); got != [3]float64{0, 0.5, 1} {
		t.Fatalf("Sample(NaN, 0.5, 2) = %v", got)
	}
	if got := identity.Sample([3]float64{-1, 1, 0.25}); got != [3]float64{0, 1, 0.25} {
		t.Fatalf("Sample(-1, 1, 0.25) = %v", got)
	}
}

func TestLUTFilesAndRefs(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "Look.CUBE")
	text := lutTestCube(17, lutIdentity, "\n")
	if err := os.WriteFile(good, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	if table, err := LoadLUTFile(good); err != nil || table.Size != 17 {
		t.Fatalf("LoadLUTFile = %v, %v", table, err)
	}
	asset, err := InspectLUTFile(good)
	if err != nil || !reflect.DeepEqual(asset, Asset{Name: "Look.CUBE", Kind: AssetLUT, LUTSize: 17, SizeBytes: int64(len(text))}) {
		t.Fatalf("InspectLUTFile = %+v, %v", asset, err)
	}
	// A directory or a missing file is refused, and so is a file above the limit (its size is checked before reading).
	for _, path := range []string{dir, filepath.Join(dir, "missing.cube")} {
		if _, err := LoadLUTFile(path); err == nil {
			t.Errorf("LoadLUTFile(%s) accepted", path)
		}
	}
	huge := filepath.Join(dir, "huge.cube")
	file, err := os.Create(huge)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(MaxLUTBytes + 1); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	var lutErr *LUTError
	if _, err := LoadLUTFile(huge); !errors.As(err, &lutErr) || lutErr.Line != 1 || !strings.Contains(lutErr.Reason, "16 MiB") {
		t.Fatalf("a 16 MiB + 1 file: %v", err)
	}
	if runtime.GOOS != "windows" {
		link := filepath.Join(dir, "link.cube")
		if err := os.Symlink(good, link); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadLUTFile(link); err == nil {
			t.Fatal("a symlink was loaded")
		}
	}
	for name, want := range map[string]bool{"look.cube": true, "LOOK.CUBE": true, "a.Cube": true, "look.cube.txt": false, "cube": false, "look.cub": false, "": false} {
		if IsLUTFileName(name) != want {
			t.Errorf("IsLUTFileName(%q) = %v", name, !want)
		}
	}
	for _, c := range []struct {
		ref, builtin, asset string
		ok                  bool
	}{
		{"", "", "", true},
		{"builtin:teal_orange", "teal_orange", "", true},
		{"builtin:sepia", "", "", false},
		{"builtin:", "", "", false},
		{"123e4567-e89b-12d3-a456-426614174000", "", "123e4567-e89b-12d3-a456-426614174000", true},
		{" padded", "", "", false},
		{strings.Repeat("a", 129), "", "", false},
	} {
		builtin, assetID, ok := ParseLUTRef(c.ref)
		if builtin != c.builtin || assetID != c.asset || ok != c.ok {
			t.Errorf("ParseLUTRef(%q) = %q, %q, %v", c.ref, builtin, assetID, ok)
		}
	}
}

// lookCubeSHA256 pins the .cube bytes of every built-in look. Fill it in by running
//
//	AURAGO_PRINT_LOOK_HASHES=1 go test ./internal/videostudio -run TestBuiltinLooksDeterministic -count=1 -v
//
// and copying the eight logged lines here. A changed hash means a changed look, so update it only on purpose.
var lookCubeSHA256 = map[string]string{
	"warm":          "5c52d6fd6d80ca1d57544d22f074ccc2f75199ff0403f2ab5fafa7a9bbea85fd",
	"cool":          "9e59529b5cc76e6100c84007f80952b5b5d369d458d544ba56d7dca8929a1fba",
	"teal_orange":   "43c0b9b62a1f1cc30a8d2d1a23dafaec829247f631da90b5cc1c10c4c747fd54",
	"mono":          "3274c52c0500930e8a0575ebd46f175ee0faaa28001a3658e5331a0978a7f33b",
	"faded_film":    "0afe13b4684fb118c00584c104dc2efb5acd63450bebf1e69b2b3a894090eb5b",
	"high_contrast": "96b49c632c02fc8fe28e68ab9c9d68fd5fb3c761455931344f56850a63787188",
	"vintage":       "13fe37d5fbeb9887bbe6b670b17bd7abf54e39f45e133e18f2e74f79d4a41c0b",
	"bleach_bypass": "0f54cf5d823a13ee860ad0abefc94e22576a53a1a145180a577572bd8b64cb71",
}

func TestBuiltinLooksDeterministic(t *testing.T) {
	looks := EffectRegistry().Looks
	if len(looks) != 8 {
		t.Fatalf("%d registry looks, want 8", len(looks))
	}
	printHashes := os.Getenv("AURAGO_PRINT_LOOK_HASHES") == "1"
	const n = BuiltinLookGrid
	index := func(r, g, b int) int { return ((b*n+g)*n + r) * 3 }
	for _, look := range looks {
		table, ok := BuiltinLook(look.ID)
		cube, okCube := BuiltinLookCube(look.ID)
		if !ok || !okCube || table.Size != n || len(table.Data) != 3*n*n*n || table.DomainMin != [3]float64{} || table.DomainMax != [3]float64{1, 1, 1} {
			t.Fatalf("look %q: table %v (%v), cube %v", look.ID, table != nil, ok, okCube)
		}
		if again, _ := BuiltinLook(look.ID); again != table {
			t.Fatalf("look %q is computed again instead of remembered", look.ID)
		}
		var written bytes.Buffer
		if err := WriteCube(&written, table); err != nil || !bytes.Equal(written.Bytes(), cube) {
			t.Fatalf("look %q: WriteCube differs from BuiltinLookCube (%v)", look.ID, err)
		}
		if parsed, err := ParseCube(bytes.NewReader(cube)); err != nil || !reflect.DeepEqual(parsed, table) {
			t.Fatalf("look %q: the served cube does not parse to the table (%v)", look.ID, err)
		}
		// One grid point against the colour formula, rounded to the six decimals the cube holds.
		params, ok := builtinLookParams(look.ID)
		if !ok {
			t.Fatalf("look %q has no parameters", look.ID)
		}
		raw := applyColor(params, [3]float64{0.25, 0.5, 0.75})
		for c := range 3 {
			want, _ := strconv.ParseFloat(string(appendCubeNumber(nil, raw[c])), 64)
			if got := table.Data[index(8, 16, 24)+c]; got != float32(want) {
				t.Fatalf("look %q at (0.25, 0.5, 0.75) channel %d = %v, want %v", look.ID, c, got, want)
			}
		}
		sum := sha256.Sum256(cube)
		hash := hex.EncodeToString(sum[:])
		switch {
		case printHashes:
			t.Logf("%q: %q,", look.ID, hash)
		case lookCubeSHA256[look.ID] == "":
			t.Errorf("look %q has no pinned sha256; run with AURAGO_PRINT_LOOK_HASHES=1 and copy the logged line", look.ID)
		case lookCubeSHA256[look.ID] != hash:
			t.Errorf("look %q changed: sha256 %s, pinned %s", look.ID, hash, lookCubeSHA256[look.ID])
		}
	}
	mono, _ := BuiltinLook("mono")
	for i := 0; i < len(mono.Data); i += 3 {
		if mono.Data[i] != mono.Data[i+1] || mono.Data[i+1] != mono.Data[i+2] {
			t.Fatalf("mono entry %d has unequal channels %v", i/3, mono.Data[i:i+3])
		}
	}
	warm, _ := BuiltinLook("warm")
	if grey := index(16, 16, 16); !(warm.Data[grey] > warm.Data[grey+2]) {
		t.Fatalf("warm mid grey = %v, want red above blue", warm.Data[grey:grey+3])
	}
	for _, id := range []string{"", "sepia", "builtin:warm", "WARM", "warm.cube"} {
		if _, ok := BuiltinLook(id); ok {
			t.Errorf("BuiltinLook(%q) found a look", id)
		}
		if _, ok := BuiltinLookCube(id); ok {
			t.Errorf("BuiltinLookCube(%q) found a look", id)
		}
	}
	// The served bytes are a copy: changing them does not change the next answer.
	cube, _ := BuiltinLookCube("cool")
	cube[0] = 'X'
	if again, _ := BuiltinLookCube("cool"); !bytes.HasPrefix(again, []byte("LUT_3D_SIZE 33\n")) {
		t.Fatal("BuiltinLookCube returned its memo")
	}
}

// FuzzParseCube runs its seeds in the normal suite. Every input either parses into a complete table inside the limits
// that writes stably, or fails with a LUTError that carries a line. Nothing panics.
func FuzzParseCube(f *testing.F) {
	identity := lutTestCube(2, lutIdentity, "\n")
	for _, seed := range []string{
		identity,
		"\xef\xbb\xbfTITLE \"seed\"\r\n# comment\r\n" + lutTestCube(2, lutIdentity, "\r\n"),
		"LUT_3D_INPUT_RANGE 0 2\n" + identity,
		"DOMAIN_MIN -1 0 0.5\nDOMAIN_MAX 1 1 1\n" + identity,
		"LUT_3D_SIZE 2\n0 0 0\n",
		"LUT_1D_SIZE 4\n",
		"LUT_3D_SIZE 65\n",
		"0 0 0\n",
		"",
		"\xff\xfe\x00A",
		"LUT_3D_SIZE 2\n1e999 0 0\n",
		"LUT_3D_SIZE 2\n" + strings.Repeat("16 -16 0.0000004\n", 8),
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		table, err := ParseCube(bytes.NewReader(data))
		if err != nil {
			var lutErr *LUTError
			if !errors.As(err, &lutErr) || lutErr.Line < 1 || lutErr.Reason == "" || table != nil {
				t.Fatalf("refusal without a line: %v (table %v)", err, table != nil)
			}
			return
		}
		if table.Size < MinLUTGrid || table.Size > MaxLUTGrid || len(table.Data) != 3*table.Size*table.Size*table.Size {
			t.Fatalf("table of grid %d with %d numbers", table.Size, len(table.Data))
		}
		for c := range 3 {
			if !(table.DomainMin[c] < table.DomainMax[c]) || math.Abs(table.DomainMin[c]) > 16 || math.Abs(table.DomainMax[c]) > 16 {
				t.Fatalf("domain %v..%v", table.DomainMin, table.DomainMax)
			}
		}
		for _, v := range table.Data {
			if !finite(float64(v)) || math.Abs(float64(v)) > 16 {
				t.Fatalf("value %v", v)
			}
		}
		var first, second bytes.Buffer
		if err := WriteCube(&first, table); err != nil {
			t.Fatalf("write: %v", err)
		}
		again, err := ParseCube(bytes.NewReader(first.Bytes()))
		if err != nil {
			t.Fatalf("the written table does not parse: %v", err)
		}
		if err := WriteCube(&second, again); err != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
			t.Fatalf("writing is not stable after one round trip (%v)", err)
		}
		_ = table.Sample([3]float64{0.5, -1, math.NaN()})
	})
}
