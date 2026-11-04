package ecalc

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
)

// pinTreeFS is treeFS plus the real pinstyles.yaml (a catalog with pinouts needs it) and the given files.
func pinTreeFS(t *testing.T, files map[string]string) fstest.MapFS {
	t.Helper()
	fsys := treeFS(t, files)
	data, err := os.ReadFile("catalog/pinstyles.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := fsys["pinstyles.yaml"]; !ok {
		fsys["pinstyles.yaml"] = &fstest.MapFile{Data: data}
	}
	return fsys
}

// fixtureItem writes one pinout item with a valid header; views, shape and pins are the indented YAML blocks of the rest.
func fixtureItem(id, views, shape, pins string) string {
	return strings.ReplaceAll(`- id: `+id+`
  kind: pinout
  category: pinouts
  group: data
  name: NAME16
  doc: {en: "Doc.", de: "Doku."}
  notes: {en: "Note.", de: "Hinweis."}
  tags: [test]
  source: "Datasheet"
`+views+`  shape:
`+shape+pins, "NAME16", flow16("T"))
}

const fixtureViews = `  views:
    plug: {en: "Male", de: "Stecker"}
    socket: {en: "Female", de: "Buchse"}
`

// pinLine writes one pin of a fixture.
func pinLine(number, name, typ, extra string) string {
	if extra != "" {
		extra = ", " + extra
	}
	return `    - {number: "` + number + `", name: ` + name + `, function: {en: "Func ` + name + `", de: "Funktion ` + name + `"}, type: ` + typ + extra + `}
`
}

// basePinout is a valid "rows" item: a rect connector with a key, three pins, a wire colour and a pin note.
var basePinout = fixtureItem("t_conn", fixtureViews, `    type: rows
    outline: rect
    contact: round
    drawn: plug
    key: top
    rows:
      - {pins: ["1", "2"]}
      - {pins: ["3"], offset: 0.5}
`, `  pins:
`+pinLine("1", "VCC", "power", "wire: red")+
	pinLine("2", "GND", "ground", `notes: {en: "Shield too.", de: "Auch Schirm."}`)+
	pinLine("3", "TX", "data", ""))

func loadPinouts(t *testing.T, src string) *Catalog {
	t.Helper()
	cat, err := Load(pinTreeFS(t, map[string]string{"pinouts/p.yaml": src}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cat
}

func TestLoadPinoutItem(t *testing.T) {
	cat := loadPinouts(t, basePinout)
	p := cat.Pinout("t_conn")
	if p == nil || len(cat.Pinouts) != 1 || p.Category != "pinouts" || p.Group != "data" || p.Source != "Datasheet" || p.Name["en"] != "T" {
		t.Fatalf("unexpected pinout %+v", p)
	}
	if kind, ok := cat.Item("t_conn"); !ok || kind != "pinout" || cat.Calculator("t_conn") != nil || cat.Table("t_conn") != nil || cat.Code("t_conn") != nil {
		t.Errorf("Item(t_conn) = %q, %v", kind, ok)
	}
	if p.Views["plug"]["de"] != "Stecker" || p.Views["socket"]["en"] != "Female" {
		t.Errorf("Views = %v", p.Views)
	}
	s := p.Shape
	if s.Type != "rows" || s.Outline != "rect" || s.Contact != "round" || s.Drawn != "plug" || s.Key != "top" ||
		!reflect.DeepEqual(s.Rows, []PinoutRow{{Pins: []string{"1", "2"}}, {Pins: []string{"3"}, Offset: 0.5}}) {
		t.Errorf("Shape = %+v", s)
	}
	if !reflect.DeepEqual(s.Numbers(), []string{"1", "2", "3"}) {
		t.Errorf("Numbers() = %v", s.Numbers())
	}
	if len(p.Pins) != 3 || p.Pins[0].Wire != "red" || p.Pins[1].Notes["de"] != "Auch Schirm." || p.Pins[2].Type != "data" || p.Variants != nil {
		t.Errorf("Pins = %+v", p.Pins)
	}
	pins, id, ok := p.PinsOf("")
	if !ok || id != "" || len(pins) != 3 {
		t.Errorf("PinsOf(\"\") = %d pins, %q, %v", len(pins), id, ok)
	}
	if _, _, ok := p.PinsOf("x"); ok {
		t.Error("PinsOf(x) must fail for an item without variants")
	}
	if !reflect.DeepEqual(PinoutGroups, []string{"data", "audio", "pc_power", "boards", "debug", "ics"}) {
		t.Errorf("PinoutGroups = %v", PinoutGroups)
	}
	if !sort.StringsAreSorted(PinoutOutlines) || len(PinoutOutlines) != 16 || !reflect.DeepEqual(PinTypes, []string{"power", "ground", "signal", "data", "io", "nc"}) {
		t.Errorf("PinoutOutlines = %v, PinTypes = %v", PinoutOutlines, PinTypes)
	}
}

func TestPinoutVariants(t *testing.T) {
	src := fixtureItem("t_jack", "", `    type: jack
    segments: ["T", "R", "S"]
`, `  variants:
    - id: first
      name: {en: "First", de: "Erste"}
      pins:
`+indent(pinLine("T", "L", "signal", "")+pinLine("R", "R", "signal", "")+pinLine("S", "GND", "ground", ""), "    ")+
		`    - id: second
      name: {en: "Second", de: "Zweite"}
      pins:
`+indent(pinLine("T", "L", "signal", "")+pinLine("R", "GND", "ground", "")+pinLine("S", "R", "signal", ""), "    "))
	p := loadPinouts(t, src).Pinout("t_jack")
	if len(p.Variants) != 2 || p.Pins != nil || p.Variants[1].Name["de"] != "Zweite" {
		t.Fatalf("variants = %+v, pins = %+v", p.Variants, p.Pins)
	}
	pins, id, ok := p.PinsOf("")
	if !ok || id != "first" || pins[2].Name != "GND" {
		t.Errorf("PinsOf(\"\") = %v, %q, %v; want the first variant", pins, id, ok)
	}
	pins, id, ok = p.PinsOf("second")
	if !ok || id != "second" || pins[2].Name != "R" {
		t.Errorf("PinsOf(second) = %v, %q, %v", pins, id, ok)
	}
	if pins, id, ok = p.PinsOf("nope"); ok || pins != nil || id != "" {
		t.Errorf("PinsOf(nope) = %v, %q, %v", pins, id, ok)
	}
}

// indent prefixes every line of s.
func indent(s, prefix string) string {
	lines := strings.SplitAfter(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = prefix + l
		}
	}
	return strings.Join(lines, "")
}

// variantsOf builds an item with two variants whose pin lists are given.
func variantsOf(a, b string) string {
	return fixtureItem("t_conn", fixtureViews, `    type: rows
    outline: rect
    contact: round
    drawn: plug
    rows:
      - {pins: ["1", "2", "3"]}
`, `  variants:
    - id: aa
      name: {en: "A", de: "A"}
      pins:
`+indent(a, "    ")+`    - id: bb
      name: {en: "B", de: "B"}
      pins:
`+indent(b, "    "))
}

func TestLoadRejectsBrokenPinouts(t *testing.T) {
	three := pinLine("1", "A", "power", "") + pinLine("2", "B", "ground", "") + pinLine("3", "C", "data", "")
	otherThree := pinLine("1", "A", "power", "") + pinLine("2", "B", "ground", "") + pinLine("9", "C", "data", "")
	cases := []struct{ name, src, want string }{
		// item fields
		{"wrong kind", mutate(t, basePinout, "kind: pinout", "kind: table"), "kind must be pinout in pinouts/"},
		{"wrong category", mutate(t, basePinout, "category: pinouts", "category: tables"), "category must be pinouts"},
		{"unknown group", mutate(t, basePinout, "group: data", "group: wires"), "group must be one of data, audio, pc_power, boards, debug, ics"},
		{"bad id", mutate(t, basePinout, "id: t_conn", "id: T-conn"), "id must match"},
		{"missing name locale", mutate(t, basePinout, `, zh: "T"}`, `}`), `missing "zh" text`},
		{"doc without german", mutate(t, basePinout, `doc: {en: "Doc.", de: "Doku."}`, `doc: {en: "Doc."}`), `doc: missing "de" text`},
		{"notes without german", mutate(t, basePinout, `notes: {en: "Note.", de: "Hinweis."}`, `notes: {en: "Note."}`), `notes: missing "de" text`},
		{"no source", mutate(t, basePinout, `source: "Datasheet"`, `source: ""`), "source is required"},
		{"no tags", mutate(t, basePinout, `tags: [test]`, `tags: []`), "at least one tag is required"},
		{"unknown field", mutate(t, basePinout, `tags: [test]`, "tags: [test]\n  colour: red"), "colour"},
		// views
		{"views missing", mutate(t, basePinout, fixtureViews, ""), "views are required when the shape is drawn as plug or socket"},
		{"views without a socket", mutate(t, basePinout, `    socket: {en: "Female", de: "Buchse"}`+"\n", ""), `views: missing "socket" caption`},
		{"views without german", mutate(t, basePinout, `plug: {en: "Male", de: "Stecker"}`, `plug: {en: "Male"}`), `views.plug: missing "de" text`},
		{"unknown view", mutate(t, basePinout, `    socket:`, `    side: {en: "S", de: "S"}`+"\n    socket:"), `unknown view "side"`},
		{"views on a dip", fixtureItem("t_conn", fixtureViews, "    type: dip\n    pins: 4\n", "  pins:\n"+pinLine("1", "A", "power", "")+pinLine("2", "B", "power", "")+pinLine("3", "C", "power", "")+pinLine("4", "D", "power", "")), "views are only for shapes that are drawn as plug or socket"},
		// pins
		{"pins and variants", basePinout + "  variants:\n    - {id: aa, name: {en: A, de: A}, pins: []}\n    - {id: bb, name: {en: B, de: B}, pins: []}\n", "use either pins or variants, not both"},
		{"neither pins nor variants", mutate(t, basePinout, "  pins:\n"+pinLine("1", "VCC", "power", "wire: red")+pinLine("2", "GND", "ground", `notes: {en: "Shield too.", de: "Auch Schirm."}`)+pinLine("3", "TX", "data", ""), ""), "pins or variants are required"},
		{"duplicate number", mutate(t, basePinout, `number: "2"`, `number: "1"`), `pin "1": number is used twice`},
		{"empty number", mutate(t, basePinout, `number: "2"`, `number: ""`), "number must be 1 to 8 characters"},
		{"long number", mutate(t, basePinout, `number: "2"`, `number: "123456789"`), "number must be 1 to 8 characters"},
		{"long name", mutate(t, basePinout, `name: GND`, `name: ABCDEFGHIJKLMNOPQRSTUVWXY`), "name must be 1 to 24 characters"},
		{"empty name", mutate(t, basePinout, `name: GND`, `name: ""`), "name must be 1 to 24 characters"},
		{"function without german", mutate(t, basePinout, `de: "Funktion TX"`, `de: ""`), `function: missing "de" text`},
		{"pin notes without german", mutate(t, basePinout, `de: "Auch Schirm."`, `de: ""`), `notes: missing "de" text`},
		{"unknown type", mutate(t, basePinout, `type: data`, `type: bus`), `type must be one of power, ground, signal, data, io, nc, got "bus"`},
		{"unknown wire", mutate(t, basePinout, `wire: red`, `wire: teal`), `wire "teal" is not in pinstyles.yaml`},
		{"unknown pin field", mutate(t, basePinout, `type: data`, `type: data, colour: red`), "colour"},
		// shape coverage
		{"pin not drawn", mutate(t, basePinout, `{pins: ["3"], offset: 0.5}`, `{pins: [null], offset: 0.5}`), "has no pins"},
		{"pin missing in the shape", mutate(t, basePinout, `{pins: ["1", "2"]}`, `{pins: ["1"]}`), `pin "2" is not drawn`},
		{"shape position without a pin", mutate(t, basePinout, `{pins: ["3"], offset: 0.5}`, `{pins: ["3", "4"], offset: 0.5}`), `position "4" names no pin`},
		{"pin drawn twice", mutate(t, basePinout, `{pins: ["3"], offset: 0.5}`, `{pins: ["3", "1"], offset: 0.5}`), `pin "1" is drawn more than once`},
		// variants
		{"one variant", strings.Replace(variantsOf(three, three), "    - id: bb\n      name: {en: \"B\", de: \"B\"}\n      pins:\n"+indent(three, "    "), "", 1), "needs at least 2 variants"},
		{"variant id twice", strings.Replace(variantsOf(three, three), "id: bb", "id: aa", 1), `variant id "aa" is used twice`},
		{"bad variant id", strings.Replace(variantsOf(three, three), "id: bb", "id: B-b", 1), `variant id "B-b" must match`},
		{"variant name without german", strings.Replace(variantsOf(three, three), `name: {en: "B", de: "B"}`, `name: {en: "B"}`, 1), `variant bb: name: missing "de" text`},
		{"variants with other numbers", variantsOf(three, otherThree), "variant bb has other pin numbers than variant aa (missing 3; extra 9)"},
		{"variant pin broken", variantsOf(three, strings.Replace(three, "type: data", "type: bus", 1)), "variant bb: pins: pin \"3\": type must be one of"},
		// shapes
		{"unknown shape type", mutate(t, basePinout, "type: rows", "type: ring"), "shape: type must be one of rows, dip, to, circular, jack, board"},
		{"field of another type", mutate(t, basePinout, "    key: top", "    key: top\n    segments: [a, b]"), "shape: field segments is not used by type rows"},
		{"rows without outline", mutate(t, basePinout, "    outline: rect\n", ""), "shape: type rows needs outline"},
		{"unknown outline", mutate(t, basePinout, "outline: rect", "outline: oval"), `shape: outline must be one of displayport, dsub,`},
		{"unknown contact", mutate(t, basePinout, "contact: round", "contact: hex"), "shape: contact must be one of round, square, blade"},
		{"unknown drawn", mutate(t, basePinout, "drawn: plug", "drawn: top"), "shape: drawn must be one of plug, socket"},
		{"unknown key", mutate(t, basePinout, "key: top", "key: middle"), "shape: key must be one of none, top, bottom, left, right"},
		{"key on a connector without rect", mutate(t, basePinout, "outline: rect", "outline: dsub"), "shape: key top needs the outline rect"},
		{"offset too large", mutate(t, basePinout, "offset: 0.5", "offset: 2.5"), "offset must be from 0 to 2 pitches"},
		{"negative offset", mutate(t, basePinout, "offset: 0.5", "offset: -1"), "offset must be from 0 to 2 pitches"},
		{"empty rows", mutate(t, basePinout, "    rows:\n      - {pins: [\"1\", \"2\"]}\n      - {pins: [\"3\"], offset: 0.5}\n", "    rows: []\n"), "shape: rows must not be empty"},
		{"empty pin number in a row", mutate(t, basePinout, `{pins: ["1", "2"]}`, `{pins: ["1", ""]}`), "has an empty pin number"},
		{"dip odd", dipFixture(5), "an even number of pins from 4 to 64"},
		{"dip too small", dipFixture(2), "an even number of pins from 4 to 64"},
		{"dip too large", dipFixture(66), "an even number of pins from 4 to 64"},
		{"dip not numbered 1..N", strings.Replace(dipFixture(4), `number: "4"`, `number: "A4"`, 1), `shape: pin "A4" is not drawn`},
		{"dip with a package", strings.Replace(dipFixture(4), "    pins: 4\n", "    pins: 4\n    package: to92\n", 1), "field package is not used by type dip"},
		{"to bad package", toFixture("to247", `["B", "C", "E"]`), "shape: package must be one of to92, to220"},
		{"to one pin", toFixture("to92", `["B"]`), "a to shape has 2 to 5 pins"},
		{"to pins as a number", toFixture("to92", `3`), "pins of a to shape must be the list of pin numbers"},
		{"jack one segment", jackFixture(`["T"]`), "a jack has 2 to 5 segments"},
		{"jack six segments", jackFixture(`["1", "2", "3", "4", "5", "6"]`), "a jack has 2 to 5 segments"},
		{"circular without drawn", strings.Replace(circularFixture(), "    drawn: socket\n", "", 1), "shape: type circular needs drawn"},
		{"circular key out of range", strings.Replace(circularFixture(), "key: 0", "key: 360", 1), "key angle must be 0 or more and less than 360"},
		{"circular key text", strings.Replace(circularFixture(), "key: 0", "key: top", 1), "key must be an angle in degrees or null"},
		{"circular angle out of range", strings.Replace(circularFixture(), "angle: 45", "angle: 400", 1), "angle is required, 0 or more and less than 360"},
		{"circular radius out of range", strings.Replace(circularFixture(), "radius: 0.7, angle: 315", "radius: 1.5, angle: 315", 1), "radius is required, from 0 to 1"},
		{"circular without radius", strings.Replace(circularFixture(), ", radius: 0}", "}", 1), "radius is required"},
		{"circular without positions", fixtureItem("t_din", fixtureViews, "    type: circular\n    contact: round\n    drawn: socket\n    positions: []\n", "  pins:\n"+pinLine("1", "L", "signal", "")), "positions must not be empty"},
		{"board without sides", strings.Replace(boardFixture(), "    left: [\"1\", null, \"2\"]\n    right: [\"3\", \"4\"]\n", "", 1), "a board needs at least one of left, right, top and bottom"},
		{"board bad connector", strings.Replace(boardFixture(), "connector: bottom", "connector: middle", 1), "shape: connector must be one of top, bottom, left, right, none"},
		{"board inner without label", strings.Replace(boardFixture(), "label: ICSP", "label: \"\"", 1), "inner 1 needs a label"},
		{"board with a plug", strings.Replace(boardFixture(), "    connector:", "    drawn: plug\n    connector:", 1), "field drawn is not used by type board"},
	}
	for _, c := range cases {
		_, err := Load(pinTreeFS(t, map[string]string{"pinouts/p.yaml": c.src}))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v, want it to contain %q", c.name, err, c.want)
		}
	}

	// The id space is shared with the other kinds.
	_, err := Load(pinTreeFS(t, map[string]string{"tables/t.yaml": baseTable, "pinouts/p.yaml": strings.Replace(basePinout, "id: t_conn", "id: t_sec", 1)}))
	if err == nil || !strings.Contains(err.Error(), "id is already used in tables/t.yaml") {
		t.Errorf("pinout with the id of a table: error = %v", err)
	}
	_, err = Load(pinTreeFS(t, map[string]string{"pinouts/p.yaml": basePinout, "pinouts/q.yaml": basePinout}))
	if err == nil || !strings.Contains(err.Error(), "id is already used in pinouts/p.yaml") {
		t.Errorf("two pinouts with one id: error = %v", err)
	}
	// A pinout does not belong in the other directories, nor a table in pinouts/; an empty file is an error.
	_, err = Load(pinTreeFS(t, map[string]string{"codes/k.yaml": basePinout}))
	if err == nil || !strings.Contains(err.Error(), "field shape not found in type ecalc.rawCode") {
		t.Errorf("pinout in codes/: error = %v", err)
	}
	_, err = Load(pinTreeFS(t, map[string]string{"pinouts/p.yaml": baseTable}))
	if err == nil || !strings.Contains(err.Error(), "field columns not found in type ecalc.rawPinout") {
		t.Errorf("table in pinouts/: error = %v", err)
	}
	_, err = Load(pinTreeFS(t, map[string]string{"pinouts/p.yaml": ""}))
	if err == nil || !strings.Contains(err.Error(), "the file is empty") {
		t.Errorf("empty file: error = %v", err)
	}
	// Pinouts need pinstyles.yaml.
	fsys := pinTreeFS(t, map[string]string{"pinouts/p.yaml": basePinout})
	delete(fsys, "pinstyles.yaml")
	_, err = Load(fsys)
	if err == nil || !strings.Contains(err.Error(), "pinstyles.yaml") {
		t.Errorf("pinouts without pinstyles.yaml: error = %v", err)
	}
}

// Every problem of an item is reported at once, with the file and the item.
func TestPinoutProblemsAreReportedTogether(t *testing.T) {
	src := mutate(t, basePinout, "group: data", "group: wires", "type: data", "type: bus", "wire: red", "wire: teal", `source: "Datasheet"`, `source: ""`)
	_, err := Load(pinTreeFS(t, map[string]string{"pinouts/p.yaml": src}))
	if err == nil {
		t.Fatal("want errors")
	}
	for _, want := range []string{"pinouts/p.yaml: t_conn: group must be", "t_conn: source is required", `pin "3": type must be`, `pin "1": wire "teal"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q:\n%v", want, err)
		}
	}
}

func dipFixture(n int) string {
	shape := "    type: dip\n    pins: " + itoa(n) + "\n"
	pins := "  pins:\n"
	for i := 1; i <= n && i <= 8; i++ {
		pins += pinLine(itoa(i), "P"+itoa(i), "io", "")
	}
	return fixtureItem("t_dip", "", shape, pins)
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func toFixture(pkg, pins string) string {
	return fixtureItem("t_to", "", "    type: to\n    package: "+pkg+"\n    pins: "+pins+"\n",
		"  pins:\n"+pinLine("B", "B", "signal", "")+pinLine("C", "C", "signal", "")+pinLine("E", "E", "ground", ""))
}

func jackFixture(segments string) string {
	return fixtureItem("t_jack", "", "    type: jack\n    segments: "+segments+"\n",
		"  pins:\n"+pinLine("T", "L", "signal", "")+pinLine("R", "R", "signal", "")+pinLine("S", "GND", "ground", ""))
}

func circularFixture() string {
	return fixtureItem("t_din", fixtureViews, `    type: circular
    contact: round
    drawn: socket
    key: 0
    positions:
      - {pin: "1", angle: 45, radius: 0.7}
      - {pin: "2", radius: 0.7, angle: 315}
      - {pin: "3", angle: 0, radius: 0}
`, "  pins:\n"+pinLine("1", "L", "signal", "")+pinLine("2", "R", "signal", "")+pinLine("3", "GND", "ground", ""))
}

func boardFixture() string {
	return fixtureItem("t_board", "", `    type: board
    left: ["1", null, "2"]
    right: ["3", "4"]
    connector: bottom
    inner:
      - label: ICSP
        rows:
          - {pins: ["5", "6"]}
          - {pins: ["7", "8"], offset: 1}
`, "  pins:\n"+pinLine("1", "D0", "io", "")+pinLine("2", "D1", "io", "")+pinLine("3", "VIN", "power", "")+pinLine("4", "GND", "ground", "")+
		pinLine("5", "MISO", "data", "")+pinLine("6", "SCK", "data", "")+pinLine("7", "RST", "signal", "")+pinLine("8", "NC", "nc", ""))
}

// jsonItems loads the fixtures and returns the pinout items of the catalog JSON in a language.
func jsonItems(t *testing.T, cat *Catalog, lang string) (items []map[string]any, doc map[string]any) {
	t.Helper()
	body, _, err := cat.LocalizedJSON(lang)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	for _, it := range doc["items"].([]any) {
		items = append(items, it.(map[string]any))
	}
	return items, doc
}

// One item per shape type pins the JSON of a shape: every field is always there, with "", null or [] when unused.
func TestPinoutJSONShapes(t *testing.T) {
	all := basePinout + dipFixture(4) + toFixture("to92", `["B", "C", "E"]`) + circularFixture() + jackFixture(`["T", "R", "S"]`) + boardFixture()
	// dipFixture(4) lists 4 pins, to92 and the others 3 or more; the ids are distinct.
	cat := loadPinouts(t, all)
	items, doc := jsonItems(t, cat, "de")
	if len(items) != 6 {
		t.Fatalf("%d items, want 6", len(items))
	}
	types, wires := doc["pin_types"].([]any), doc["wires"].([]any)
	if len(types) != 6 || len(wires) != 15 {
		t.Errorf("pin_types %d, wires %d", len(types), len(wires))
	}

	row := func(offset float64, pins ...any) map[string]any {
		return map[string]any{"pins": append([]any{}, pins...), "offset": offset}
	}
	shape := func(over map[string]any) map[string]any {
		out := map[string]any{
			"type": "", "outline": "", "contact": "", "drawn": "", "key": nil, "rows": []any{}, "pins": nil, "package": "",
			"positions": []any{}, "segments": []any{}, "left": []any{}, "right": []any{}, "top": []any{}, "bottom": []any{},
			"connector": "", "inner": []any{},
		}
		for k, v := range over {
			out[k] = v
		}
		return out
	}
	want := map[string]map[string]any{
		"t_conn": shape(map[string]any{"type": "rows", "outline": "rect", "contact": "round", "drawn": "plug", "key": "top",
			"rows": []any{row(0, "1", "2"), row(0.5, "3")}}),
		"t_dip": shape(map[string]any{"type": "dip", "drawn": "top", "pins": float64(4)}),
		"t_to":  shape(map[string]any{"type": "to", "drawn": "front", "package": "to92", "pins": []any{"B", "C", "E"}}),
		"t_din": shape(map[string]any{"type": "circular", "contact": "round", "drawn": "socket", "key": float64(0),
			"positions": []any{
				map[string]any{"pin": "1", "angle": float64(45), "radius": 0.7},
				map[string]any{"pin": "2", "angle": float64(315), "radius": 0.7},
				map[string]any{"pin": "3", "angle": float64(0), "radius": float64(0)}}}),
		"t_jack": shape(map[string]any{"type": "jack", "drawn": "side", "segments": []any{"T", "R", "S"}}),
		"t_board": shape(map[string]any{"type": "board", "drawn": "top", "connector": "bottom",
			"left": []any{"1", nil, "2"}, "right": []any{"3", "4"},
			"inner": []any{map[string]any{"label": "ICSP", "rows": []any{row(0, "5", "6"), row(1, "7", "8")}}}}),
	}
	for _, it := range items {
		id := it["id"].(string)
		if len(it) != 14 {
			t.Errorf("%s has %d keys, want 14: %v", id, len(it), it)
		}
		if it["kind"] != "pinout" || it["category"] != "pinouts" || it["group"] != "data" || it["name"] != "T" || it["name_en"] != "T" ||
			it["doc"] != "Doku." || it["notes"] != "Hinweis." || it["source"] != "Datasheet" || !reflect.DeepEqual(it["tags"], []any{"test"}) {
			t.Errorf("%s: item = %v", id, it)
		}
		if !reflect.DeepEqual(it["shape"], any(want[id])) {
			t.Errorf("%s: shape = %#v\nwant %#v", id, it["shape"], want[id])
		}
		if !reflect.DeepEqual(it["variants"], []any{}) {
			t.Errorf("%s: variants = %#v, want []", id, it["variants"])
		}
		switch id {
		case "t_conn", "t_din":
			if !reflect.DeepEqual(it["views"], map[string]any{"plug": "Stecker", "socket": "Buchse"}) {
				t.Errorf("%s: views = %#v", id, it["views"])
			}
		default:
			if v, ok := it["views"]; !ok || v != nil {
				t.Errorf("%s: views = %#v, want null", id, v)
			}
		}
	}

	// The pins of the first item: all six keys, German texts, "" for notes and wire when unset.
	pins := items[0]["pins"].([]any)
	if len(pins) != 3 {
		t.Fatalf("pins = %v", pins)
	}
	wantPin := []map[string]any{
		{"number": "1", "name": "VCC", "function": "Funktion VCC", "type": "power", "notes": "", "wire": "red"},
		{"number": "2", "name": "GND", "function": "Funktion GND", "type": "ground", "notes": "Auch Schirm.", "wire": ""},
		{"number": "3", "name": "TX", "function": "Funktion TX", "type": "data", "notes": "", "wire": ""},
	}
	for i, p := range pins {
		if !reflect.DeepEqual(p, any(wantPin[i])) {
			t.Errorf("pin %d = %#v, want %#v", i, p, wantPin[i])
		}
	}

	// A language without texts falls back to English.
	fr, _ := jsonItems(t, cat, "fr")
	if fr[0]["doc"] != "Doc." || fr[0]["pins"].([]any)[0].(map[string]any)["function"] != "Func VCC" ||
		fr[0]["views"].(map[string]any)["plug"] != "Male" {
		t.Errorf("French fallback: %v", fr[0])
	}
}

func TestPinoutJSONVariantsAndStyles(t *testing.T) {
	three := pinLine("1", "A", "power", "") + pinLine("2", "B", "ground", "") + pinLine("3", "C", "data", "")
	cat := loadPinouts(t, variantsOf(three, strings.Replace(three, `name: A`, `name: Z`, 1)))
	items, doc := jsonItems(t, cat, "en")
	it := items[0]
	if !reflect.DeepEqual(it["pins"], []any{}) {
		t.Errorf("an item with variants has pins = %#v, want []", it["pins"])
	}
	variants := it["variants"].([]any)
	if len(variants) != 2 || variants[1].(map[string]any)["id"] != "bb" || variants[1].(map[string]any)["name"] != "B" ||
		len(variants[1].(map[string]any)["pins"].([]any)) != 3 {
		t.Errorf("variants = %#v", variants)
	}
	types := doc["pin_types"].([]any)
	first := types[0].(map[string]any)
	if len(types) != 6 || first["id"] != "power" || first["hex"] != "#c62828" || first["ink"] != "#ffffff" || first["name"] != "Power" || len(first) != 4 {
		t.Errorf("pin_types = %v", types)
	}
	wires := doc["wires"].([]any)
	red, wg := wires[2].(map[string]any), wires[12].(map[string]any)
	if len(wires) != 15 || red["id"] != "red" || red["stripe"] != "" || wg["id"] != "white_green" || wg["stripe"] != "#2e7d32" || wg["name"] != "white-green" || len(red) != 4 {
		t.Errorf("wires = %v", wires)
	}
	_, deDoc := jsonItems(t, cat, "de")
	if deDoc["wires"].([]any)[12].(map[string]any)["name"] != "weiß-grün" || deDoc["pin_types"].([]any)[1].(map[string]any)["name"] != "Masse" {
		t.Errorf("German style names: %v", deDoc["wires"])
	}
}

// Without pinouts the pin styles still load, and without pinstyles.yaml the lists are empty arrays.
func TestCatalogWithoutPinouts(t *testing.T) {
	cat, err := Load(treeFS(t, nil))
	if err != nil {
		t.Fatalf("Load without pinstyles.yaml: %v", err)
	}
	_, doc := jsonItems(t, cat, "en")
	if !reflect.DeepEqual(doc["pin_types"], []any{}) || !reflect.DeepEqual(doc["wires"], []any{}) {
		t.Errorf("pin_types = %#v, wires = %#v, want empty arrays", doc["pin_types"], doc["wires"])
	}
	if cat.Pinout("x") != nil || cat.PinType("power") != nil || cat.Wire("red") != nil || len(cat.Pinouts) != 0 {
		t.Error("an empty catalog must have no pinouts and no styles")
	}
}
