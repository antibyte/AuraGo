package ecalc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
)

// testdata/vectors/pinouts/<pinout id>.json is the independent check of one pinout item: the pin types as counted from
// the source, and spot checks of pins against the standard or the datasheet.
//
//	{"pinout": "usb_a", "variants": [{"variant": "", "counts": {"power": 1, "ground": 1, "data": 2}, "pins": [
//	  {"number": "1", "name": "VBUS", "type": "power", "ref": "USB 2.0 specification, table 6-1"}, ...]}]}
//
// There is one entry per variant ("variant": "" for an item without variants). counts holds the number of pins of every
// type (a type without pins may be left out); the sum is the pin count of the item. pins holds at least three spot checks
// per variant (all pins when there are fewer), each with the source in ref. A vector file without an item and an item
// without a vector file both fail.
type pinoutVectorPin struct {
	Number string `json:"number"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Ref    string `json:"ref"`
}

type pinoutVectorVariant struct {
	Variant string            `json:"variant"`
	Counts  map[string]int    `json:"counts"`
	Pins    []pinoutVectorPin `json:"pins"`
}

type pinoutVectors struct {
	Pinout   string                `json:"pinout"`
	Variants []pinoutVectorVariant `json:"variants"`
}

// minSpotChecks is the number of pins that a vector checks per variant, or all of them when there are fewer.
const minSpotChecks = 3

// checkPinoutVectors compares the vector files of vecs (a directory of <id>.json files) with the pinouts of the catalog and
// returns every problem found.
func checkPinoutVectors(cat *Catalog, vecs fs.FS) []string {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }

	entries, err := fs.ReadDir(vecs, ".")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return []string{err.Error()}
	}
	files := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		files[id] = true
		p := cat.Pinout(id)
		if p == nil {
			add("%s: there is no pinout %q (an orphan vector file)", e.Name(), id)
			continue
		}
		data, err := fs.ReadFile(vecs, e.Name())
		if err != nil {
			add("%s: %v", e.Name(), err)
			continue
		}
		var v pinoutVectors
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&v); err != nil {
			add("%s: %v", e.Name(), err)
			continue
		}
		for _, msg := range checkPinoutVector(p, v) {
			add("%s: %s", e.Name(), msg)
		}
	}
	for _, p := range cat.Pinouts {
		if !files[p.ID] {
			add("pinout %s has no vector file %s.json", p.ID, p.ID)
		}
	}
	sort.Strings(problems)
	return problems
}

// checkPinoutVector checks one vector file against its pinout.
func checkPinoutVector(p *Pinout, v pinoutVectors) []string {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }
	if v.Pinout != p.ID {
		add("pinout is %q, want %q", v.Pinout, p.ID)
	}
	want := []string{""}
	if len(p.Variants) > 0 {
		want = nil
		for _, variant := range p.Variants {
			want = append(want, variant.ID)
		}
	}
	seen := map[string]bool{}
	for _, ve := range v.Variants {
		label := "variant " + ve.Variant
		if ve.Variant == "" {
			label = "the item"
		}
		if seen[ve.Variant] {
			add("%s appears twice", label)
			continue
		}
		seen[ve.Variant] = true
		pins, _, ok := p.PinsOf(ve.Variant)
		if !ok || (ve.Variant == "" && len(p.Variants) > 0) {
			add("%s: the pinout has no such variant (variants: %s)", label, strings.Join(want, ", "))
			continue
		}
		byNumber := map[string]Pin{}
		actual := map[string]int{}
		for _, pin := range pins {
			byNumber[pin.Number] = pin
			actual[pin.Type]++
		}
		for typ, n := range ve.Counts {
			if !contains(PinTypes, typ) {
				add("%s: counts has the unknown type %q", label, typ)
			} else if n < 0 {
				add("%s: counts %s is negative", label, typ)
			}
		}
		for _, typ := range PinTypes {
			if ve.Counts[typ] != actual[typ] {
				add("%s: counts %s = %d, but the pinout has %d", label, typ, ve.Counts[typ], actual[typ])
			}
		}
		if need := min(minSpotChecks, len(pins)); len(ve.Pins) < need {
			add("%s: %d spot checks, want at least %d", label, len(ve.Pins), need)
		}
		checked := map[string]bool{}
		for _, sp := range ve.Pins {
			pin, found := byNumber[sp.Number]
			switch {
			case checked[sp.Number]:
				add("%s: pin %q is checked twice", label, sp.Number)
			case !found:
				add("%s: the pinout has no pin %q", label, sp.Number)
			case pin.Name != sp.Name:
				add("%s: pin %s is named %q, the vector says %q", label, sp.Number, pin.Name, sp.Name)
			case pin.Type != sp.Type:
				add("%s: pin %s has the type %q, the vector says %q", label, sp.Number, pin.Type, sp.Type)
			}
			checked[sp.Number] = true
			if strings.TrimSpace(sp.Ref) == "" {
				add("%s: pin %s has no ref (the source of the check)", label, sp.Number)
			}
		}
	}
	for _, id := range want {
		if !seen[id] {
			if id == "" {
				add("the vector has no entry for the item (variant \"\")")
			} else {
				add("the vector has no entry for variant %s", id)
			}
		}
	}
	return problems
}

// The real vectors: every pinout has a file and every file a pinout. Without items the check passes trivially.
func TestPinoutVectors(t *testing.T) {
	cat, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	if problems := checkPinoutVectors(cat, os.DirFS("testdata/vectors/pinouts")); len(problems) > 0 {
		t.Errorf("the pinout vectors disagree with the catalog:\n%s", strings.Join(problems, "\n"))
	}
}

const goodVector = `{"pinout": "t_conn", "variants": [{"variant": "", "counts": {"power": 1, "ground": 1, "data": 1}, "pins": [
  {"number": "1", "name": "VCC", "type": "power", "ref": "Datasheet table 1"},
  {"number": "2", "name": "GND", "type": "ground", "ref": "Datasheet table 1"},
  {"number": "3", "name": "TX", "type": "data", "ref": "Datasheet table 1"}]}]}`

// A fixture proves that the check can fail: every rule has a vector that it refuses.
func TestPinoutVectorChecksCanFail(t *testing.T) {
	three := pinLine("1", "A", "power", "") + pinLine("2", "B", "ground", "") + pinLine("3", "C", "data", "")
	cat := loadPinouts(t, basePinout+strings.Replace(variantsOf(three, three), "t_conn", "t_var", 1))
	good := fstest.MapFS{"t_conn.json": {Data: []byte(goodVector)}, "t_var.json": {Data: []byte(goodVariantVector)}}
	if problems := checkPinoutVectors(cat, good); len(problems) > 0 {
		t.Fatalf("the good vectors fail: %v", problems)
	}

	with := func(name, data string) fstest.MapFS {
		fsys := fstest.MapFS{"t_conn.json": {Data: []byte(goodVector)}, "t_var.json": {Data: []byte(goodVariantVector)}}
		if data == "" {
			delete(fsys, name)
		} else {
			fsys[name] = &fstest.MapFile{Data: []byte(data)}
		}
		return fsys
	}
	cases := []struct {
		name string
		fsys fstest.MapFS
		want string
	}{
		{"an item without a file", with("t_conn.json", ""), "pinout t_conn has no vector file t_conn.json"},
		{"an orphan file", with("ghost.json", goodVector), `there is no pinout "ghost"`},
		{"another pinout id inside", with("t_conn.json", strings.Replace(goodVector, `"pinout": "t_conn"`, `"pinout": "x"`, 1)), `pinout is "x", want "t_conn"`},
		{"an unknown field", with("t_conn.json", strings.Replace(goodVector, `"pinout"`, `"colour": 1, "pinout"`, 1)), `unknown field "colour"`},
		{"broken json", with("t_conn.json", `{`), "unexpected EOF"},
		{"counts that do not match", with("t_conn.json", strings.Replace(goodVector, `"power": 1`, `"power": 2`, 1)), "counts power = 2, but the pinout has 1"},
		{"counts without a type", with("t_conn.json", strings.Replace(goodVector, `"data": 1`, `"signal": 0`, 1)), "counts data = 0, but the pinout has 1"},
		{"counts with an unknown type", with("t_conn.json", strings.Replace(goodVector, `"data": 1`, `"data": 1, "bus": 1`, 1)), `unknown type "bus"`},
		{"too few spot checks", with("t_conn.json", strings.Replace(goodVector, `,
  {"number": "3", "name": "TX", "type": "data", "ref": "Datasheet table 1"}`, "", 1)), "2 spot checks, want at least 3"},
		{"a spot check of a missing pin", with("t_conn.json", strings.Replace(goodVector, `"number": "3"`, `"number": "9"`, 1)), `the pinout has no pin "9"`},
		{"a spot check with the wrong name", with("t_conn.json", strings.Replace(goodVector, `"name": "TX"`, `"name": "RX"`, 1)), `pin 3 is named "TX", the vector says "RX"`},
		{"a spot check with the wrong type", with("t_conn.json", strings.Replace(goodVector, `"type": "data"`, `"type": "signal"`, 1)), `pin 3 has the type "data", the vector says "signal"`},
		{"a spot check without ref", with("t_conn.json", strings.Replace(goodVector, `"ref": "Datasheet table 1"}]`, `"ref": ""}]`, 1)), "pin 3 has no ref"},
		{"a pin checked twice", with("t_conn.json", strings.Replace(goodVector, `"number": "2"`, `"number": "1"`, 1)), `pin "1" is checked twice`},
		{"a variant entry missing", with("t_var.json", strings.Replace(goodVariantVector, `{"variant": "bb"`, `{"variant": "aa"`, 1)), "variant aa appears twice"},
		{"the item entry for a variants item", with("t_var.json", strings.ReplaceAll(goodVariantVector, `"variant": "aa"`, `"variant": ""`)), "the pinout has no such variant (variants: aa, bb)"},
		{"no entry for a variant", with("t_var.json", onlyVariantAA), "no entry for variant bb"},
		{"no entry for the item", with("t_conn.json", `{"pinout": "t_conn", "variants": []}`), `no entry for the item`},
	}
	for _, c := range cases {
		problems := strings.Join(checkPinoutVectors(cat, c.fsys), "\n")
		if !strings.Contains(problems, c.want) {
			t.Errorf("%s: problems = %q, want it to contain %q", c.name, problems, c.want)
		}
	}
}

const goodVariantVector = `{"pinout": "t_var", "variants": [
 {"variant": "aa", "counts": {"power": 1, "ground": 1, "data": 1}, "pins": [
  {"number": "1", "name": "A", "type": "power", "ref": "R"}, {"number": "2", "name": "B", "type": "ground", "ref": "R"}, {"number": "3", "name": "C", "type": "data", "ref": "R"}]},
 {"variant": "bb", "counts": {"power": 1, "ground": 1, "data": 1}, "pins": [
  {"number": "1", "name": "A", "type": "power", "ref": "R"}, {"number": "2", "name": "B", "type": "ground", "ref": "R"}, {"number": "3", "name": "C", "type": "data", "ref": "R"}]}]}`

// onlyVariantAA lacks the entry of variant bb.
const onlyVariantAA = `{"pinout": "t_var", "variants": [
 {"variant": "aa", "counts": {"power": 1, "ground": 1, "data": 1}, "pins": [
  {"number": "1", "name": "A", "type": "power", "ref": "R"}, {"number": "2", "name": "B", "type": "ground", "ref": "R"}, {"number": "3", "name": "C", "type": "data", "ref": "R"}]}]}`
