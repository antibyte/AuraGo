package ecalc

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// The pinout queries run against fixture pinouts (a MapFS next to the real units, categories and pin styles), never
// against the real items: a large board-like item with repeated names and long texts, an item with two variants whose
// pins have wires and notes, and a small DIP without views.

// pinoutBoardFixture is a 40-pin board: GND on every fifth pin, long function texts and a note on every pin.
func pinoutBoardFixture() string {
	var left, right, pins strings.Builder
	for n := 1; n <= 40; n++ {
		name, typ := fmt.Sprintf("GPIO%d", n), "io"
		if n%5 == 0 {
			name, typ = "GND", "ground"
		}
		list := &left
		if n > 20 {
			list = &right
		}
		if list.Len() > 0 {
			list.WriteString(", ")
		}
		fmt.Fprintf(list, `"%d"`, n)
		fmt.Fprintf(&pins, `    - {number: "%d", name: %s, function: {en: "General purpose line %d of the header with a deliberately long description so that the page is large", de: "Allzweckleitung %d des Headers"}, type: %s, notes: {en: "Note for pin %d: a deliberately long hint that the bound test has to shorten before it drops rows.", de: "Hinweis %d"}}`+"\n", n, name, n, n, typ, n, n)
	}
	return strings.ReplaceAll(`- id: t_board
  kind: pinout
  category: pinouts
  group: boards
  name: NAME16
  doc: {en: "A large test board. Second sentence.", de: "Ein grosses Testboard."}
  notes: {en: "Board notes: 3V3 logic only.", de: "Boardhinweise: nur 3V3."}
  tags: [board, testboard]
  source: "Fixture datasheet"
  shape:
    type: board
    left: [`+left.String()+`]
    right: [`+right.String()+`]
    connector: bottom
  pins:
`+pins.String(), "NAME16", flow16("Test board"))
}

// pinoutVariantsFixture is a two-variant RJ45-like item with wires and notes.
var pinoutVariantsFixture = strings.ReplaceAll(`- id: t_rj
  kind: pinout
  category: pinouts
  group: data
  name: NAME16
  doc: {en: "Two wiring variants.", de: "Zwei Belegungen."}
  notes: {en: "Pair 1 is the centre pair.", de: "Paar 1 ist das mittlere."}
  tags: [rj, ethernet]
  source: "Fixture standard"
  views:
    plug: {en: "Male connector (pins)", de: "Stecker (Stifte)"}
    socket: {en: "Female connector (sockets)", de: "Buchse"}
  shape:
    type: rows
    outline: rj45
    contact: blade
    drawn: socket
    rows:
      - {pins: ["1", "2", "3", "4"]}
  variants:
    - id: t568a
      name: {en: "T568A", de: "T568A"}
      pins:
        - {number: "1", name: "TX+", function: {en: "Transmit +", de: "Senden +"}, type: data, wire: white_green}
        - {number: "2", name: "TX-", function: {en: "Transmit -", de: "Senden -"}, type: data, wire: green, notes: {en: "Twisted with pin 1.", de: "Verdrillt mit Pin 1."}}
        - {number: "3", name: "RX+", function: {en: "Receive +", de: "Empfangen +"}, type: data, wire: white_orange}
        - {number: "4", name: "GND", function: {en: "Ground", de: "Masse"}, type: ground}
    - id: t568b
      name: {en: "T568B", de: "T568B"}
      pins:
        - {number: "1", name: "TX+", function: {en: "Transmit +", de: "Senden +"}, type: data, wire: white_orange}
        - {number: "2", name: "TX-", function: {en: "Transmit -", de: "Senden -"}, type: data, wire: orange}
        - {number: "3", name: "RX+", function: {en: "Receive +", de: "Empfangen +"}, type: data, wire: white_green}
        - {number: "4", name: "GND", function: {en: "Ground", de: "Masse"}, type: ground}
`, "NAME16", flow16("Test jack"))

var pinoutDipFixture = strings.ReplaceAll(`- id: t_dip
  kind: pinout
  category: pinouts
  group: ics
  name: NAME16
  tags: [dip, timer]
  source: "Fixture datasheet"
  shape:
    type: dip
    pins: 4
  pins:
    - {number: "1", name: GND, function: {en: "Ground", de: "Masse"}, type: ground}
    - {number: "2", name: IN, function: {en: "Input", de: "Eingang"}, type: signal}
    - {number: "3", name: OUT, function: {en: "Output", de: "Ausgang"}, type: signal}
    - {number: "4", name: VCC, function: {en: "Supply", de: "Versorgung"}, type: power}
`, "NAME16", flow16("Test timer"))

func pinoutQueryCatalog(t *testing.T) *Catalog {
	t.Helper()
	cat, err := Load(pinTreeFS(t, map[string]string{
		"pinouts/a_board.yaml": pinoutBoardFixture(), "pinouts/b_rj.yaml": pinoutVariantsFixture, "pinouts/c_dip.yaml": pinoutDipFixture,
		"tables/amp.yaml": queryTables,
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cat
}

func pinNumbers(pins []PinAnswer) string {
	out := make([]string, len(pins))
	for i, p := range pins {
		out[i] = p.Number
	}
	return strings.Join(out, ",")
}

func TestQueryPinoutListsAPageOfPins(t *testing.T) {
	cat := pinoutQueryCatalog(t)
	res, err := cat.QueryPinout(" T_BOARD ", PinoutRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if res.ID != "t_board" || res.Name != "Test board" || res.Group != "boards" || res.Total != 40 || res.Returned != 40 || res.Truncated || res.NextOffset != 0 ||
		res.Variant != "" || len(res.Variants) != 0 || res.Found != nil || res.Source != "Fixture datasheet" || res.Notes != "Board notes: 3V3 logic only." {
		t.Fatalf("result = %+v", res)
	}
	if res.Connector != "Board, main connector at the bottom, 40 pins" || res.View != "Board seen from the component side, portrait, main connector at the bottom." {
		t.Errorf("connector %q, view %q", res.Connector, res.View)
	}
	if p := res.Pins[4]; p.Number != "5" || p.Name != "GND" || p.Type != "ground" || p.Function == "" || !strings.HasPrefix(p.Notes, "Note for pin 5") || p.Wire != "" {
		t.Errorf("pin 5 = %+v", p)
	}

	// Paging: offset and limit; the continuation is said.
	res, err = cat.QueryPinout("t_board", PinoutRequest{Offset: 10, Limit: 5})
	if err != nil || pinNumbers(res.Pins) != "11,12,13,14,15" || res.Total != 40 || res.Offset != 10 || res.Returned != 5 || !res.Truncated || res.NextOffset != 15 {
		t.Fatalf("page = %v %+v", err, res)
	}
	// The limit is 1 to 50 (default 50): zero and too large values mean 50, an offset past the end lists nothing.
	for _, limit := range []int{0, -3, 51, 1000} {
		if res, _ = cat.QueryPinout("t_board", PinoutRequest{Limit: limit}); res.Returned != 40 {
			t.Errorf("limit %d returned %d pins", limit, res.Returned)
		}
	}
	if res, _ = cat.QueryPinout("t_board", PinoutRequest{Offset: 99}); res.Returned != 0 || res.Total != 40 || res.Truncated || len(res.Pins) != 0 || res.Pins == nil {
		t.Errorf("offset past the end = %+v", res)
	}
	// The pins are English and a pinout page is always the same JSON shape: pins is a list even when empty.
	raw, _ := json.Marshal(res)
	if !strings.Contains(string(raw), `"pins":[]`) {
		t.Errorf("an empty page must carry pins: %s", raw)
	}
}

func TestQueryPinoutPinMatchesNumbersAndNamesExactly(t *testing.T) {
	cat := pinoutQueryCatalog(t)
	// Several pins share a name: all of them come back, in pin order.
	res, err := cat.QueryPinout("t_board", PinoutRequest{Pin: "gnd"})
	if err != nil || pinNumbers(res.Pins) != "5,10,15,20,25,30,35,40" || res.Total != 8 || res.Found == nil || !*res.Found {
		t.Fatalf("GND = %v %+v", err, res)
	}
	// A number matches the number only, and the name exactly: GPIO1 is not GPIO11.
	for pin, want := range map[string]string{"7": "7", " 12 ": "12", "GPIO1": "1", "gpio11": "11", "GPIO": ""} {
		res, err := cat.QueryPinout("t_board", PinoutRequest{Pin: pin})
		if err != nil || pinNumbers(res.Pins) != want {
			t.Errorf("pin %q = %v %s, want %q", pin, err, pinNumbers(res.Pins), want)
		}
	}
	// A pin that matches nothing is an answer: found false, the variants (none here) and a note, not an error.
	res, err = cat.QueryPinout("t_board", PinoutRequest{Pin: "SDA"})
	if err != nil || res.Found == nil || *res.Found || res.Total != 0 || res.Returned != 0 || res.Pins == nil || len(res.Pins) != 0 || !strings.Contains(res.Note, `"SDA"`) {
		t.Fatalf("SDA = %v %+v", err, res)
	}
	raw, _ := json.Marshal(res)
	if !strings.Contains(string(raw), `"found":false`) {
		t.Errorf("found:false must be in the JSON: %s", raw)
	}
	// pin and filter both have to match; an overlong pin is found:false without echoing it.
	if res, _ = cat.QueryPinout("t_board", PinoutRequest{Pin: "GND", Filter: "pin 25"}); pinNumbers(res.Pins) != "25" {
		t.Errorf("GND + filter = %s", pinNumbers(res.Pins))
	}
	res, err = cat.QueryPinout("t_board", PinoutRequest{Pin: strings.Repeat("x", 5000)})
	if err != nil || *res.Found || strings.Contains(res.Note, "xxxx") || len(res.Note) > 300 {
		t.Errorf("long pin = %v %q", err, res.Note)
	}
}

func TestQueryPinoutVariants(t *testing.T) {
	cat := pinoutQueryCatalog(t)
	// Without variant the first one is used; the variants are listed.
	res, err := cat.QueryPinout("t_rj", PinoutRequest{})
	if err != nil || res.Variant != "t568a" || res.VariantName != "T568A" || !slices.Equal(res.Variants, []string{"t568a", "t568b"}) || res.Total != 4 {
		t.Fatalf("default variant = %v %+v", err, res)
	}
	if p := res.Pins[0]; p.Wire != "white-green" || p.Name != "TX+" || p.Type != "data" {
		t.Errorf("pin 1 = %+v (the wire is the English colour name)", p)
	}
	if res.Pins[1].Wire != "green" || res.Pins[1].Notes != "Twisted with pin 1." || res.Pins[3].Wire != "" {
		t.Errorf("pins = %+v", res.Pins)
	}
	res, err = cat.QueryPinout("t_rj", PinoutRequest{Variant: " T568B ", Pin: "1"})
	if err != nil || res.Variant != "t568b" || res.Pins[0].Wire != "white-orange" {
		t.Fatalf("t568b = %v %+v", err, res)
	}
	// An unknown variant is an error that lists the ids; so is a variant of an item that has none.
	_, err = cat.QueryPinout("t_rj", PinoutRequest{Variant: "t568c"})
	wantQueryError(t, err, CodeUnknownVariant, `no variant "t568c"`, "t568a, t568b")
	_, err = cat.QueryPinout("t_dip", PinoutRequest{Variant: "a"})
	wantQueryError(t, err, CodeUnknownVariant, "t_dip has no variants")
	// A pin that is in no variant answers found:false and still lists the variants.
	res, err = cat.QueryPinout("t_rj", PinoutRequest{Pin: "SDA"})
	if err != nil || res.Found == nil || *res.Found || !slices.Equal(res.Variants, []string{"t568a", "t568b"}) || !strings.Contains(res.Note, "t568a, t568b") {
		t.Errorf("SDA in a variant item = %v %+v", err, res)
	}
}

func TestQueryPinoutFilterAndErrors(t *testing.T) {
	cat := pinoutQueryCatalog(t)
	// The filter looks at number, name, function and notes, in English and German, ignoring case.
	for filter, want := range map[string]string{"gpio3": "3,31,32,33,34,36,37,38,39", "allzweckleitung 7 ": "7", "HINWEIS 40": "40", "Note for pin 1:": "1"} {
		res, err := cat.QueryPinout("t_board", PinoutRequest{Filter: filter})
		if err != nil || pinNumbers(res.Pins) != want {
			t.Errorf("filter %q = %v %s, want %s", filter, err, pinNumbers(res.Pins), want)
		}
	}
	res, _ := cat.QueryPinout("t_board", PinoutRequest{Filter: "zzz"})
	if res.Total != 0 || res.Note != "No pin matches the filter." || res.Found != nil {
		t.Errorf("filter without a match = %+v", res)
	}
	_, err := cat.QueryPinout("t_board", PinoutRequest{Filter: strings.Repeat("a", MaxFilterLength+1)})
	wantQueryError(t, err, CodeInvalidFilter, "at most 128 characters")

	// Ids: case-insensitive, the id of another kind says what it is, an unknown id suggests the pinouts only.
	_, err = cat.QueryPinout("t_amp", PinoutRequest{})
	wantQueryError(t, err, CodeNotAPinout, "t_amp is a table, not a pinout")
	_, err = cat.QueryPinout("board", PinoutRequest{})
	qe := wantQueryError(t, err, CodeUnknownID, `unknown pinout "board"`, "did you mean")
	for _, id := range suggestedIDs(qe.Message) {
		if kind, _ := cat.Item(id); kind != "pinout" {
			t.Errorf("only pinouts are suggested, got the %s %s", kind, id)
		}
	}
	_, err = cat.QueryPinout("", PinoutRequest{})
	wantQueryError(t, err, CodeUnknownID, "id is required", "pinout id")
	_, err = cat.QueryPinout(strings.Repeat("x", MaxIDLength+1), PinoutRequest{})
	wantQueryError(t, err, CodeUnknownID, "longer than 256 characters")
	// The other kinds say what a pinout is.
	_, err = cat.DescribeTable("t_board")
	wantQueryError(t, err, CodeNotATable, "t_board is a pinout, not a table", "pinout reads its pins")
	_, err = cat.Describe("t_board")
	wantQueryError(t, err, CodeNotACalculator, "t_board is a pinout")
	_, err = cat.DescribeCode("t_board")
	wantQueryError(t, err, CodeNotACode, "t_board is a pinout")
}

func TestDescribePinout(t *testing.T) {
	cat := pinoutQueryCatalog(t)
	d, err := cat.DescribePinout("T_RJ")
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != "t_rj" || d.Kind != "pinout" || d.Category != "pinouts" || d.Group != "data" || d.Name != "Test jack" || d.Doc != "Two wiring variants." || d.Notes != "Pair 1 is the centre pair." ||
		d.Source != "Fixture standard" || d.PinCount != 4 || d.Counts["data"] != 3 || d.Counts["ground"] != 1 || len(d.Counts) != 2 {
		t.Fatalf("description = %+v", d)
	}
	if d.Connector != "RJ45 connector, 1 row, blade contacts, 4 pins" || d.View != "Rows as seen looking at the mating face of the female connector (sockets)." {
		t.Errorf("connector %q, view %q", d.Connector, d.View)
	}
	if len(d.Variants) != 2 || d.Variants[0].ID != "t568a" || d.Variants[1].Name != "T568B" || d.Variants[1].Counts["data"] != 3 {
		t.Errorf("variants = %+v", d.Variants)
	}
	raw, _ := json.Marshal(d)
	if strings.Contains(string(raw), `"pins"`) || strings.Contains(string(raw), "Transmit") {
		t.Errorf("describe lists no pins: %s", raw)
	}
	dip, _ := cat.DescribePinout("t_dip")
	if dip.Connector != "DIP package, 4 pins" || dip.View != "Package seen from above: notch at the top, pin 1 at the top left, numbers run counter-clockwise." || len(dip.Variants) != 0 || dip.Counts["signal"] != 2 {
		t.Errorf("dip = %+v", dip)
	}
	if _, err := cat.DescribePinout("t_amp"); AsQueryError(err) == nil || AsQueryError(err).Code != CodeNotAPinout {
		t.Errorf("describe of a table = %v", err)
	}
}

// Every outline of the loader has an English name, and the view sentence of every shape type is one sentence.
func TestPinoutTextsCoverEveryOutlineAndShape(t *testing.T) {
	for _, o := range PinoutOutlines {
		if pinoutOutlineNames[o] == "" {
			t.Errorf("outline %s has no English name", o)
		}
	}
	if len(pinoutOutlineNames) != len(PinoutOutlines) {
		t.Errorf("%d names for %d outlines", len(pinoutOutlineNames), len(PinoutOutlines))
	}
	views := map[string]Text{"plug": {"en": "Male connector (pins)"}, "socket": {"en": "Female connector (sockets)"}}
	for _, c := range []struct {
		shape PinoutShape
		view  string
	}{
		{PinoutShape{Type: "rows", Drawn: "plug"}, "Rows as seen looking at the mating face of the male connector (pins)."},
		{PinoutShape{Type: "rows", Drawn: "socket"}, "Rows as seen looking at the mating face of the female connector (sockets)."},
		{PinoutShape{Type: "circular", Drawn: "plug"}, "Pin positions as seen looking at the mating face of the male connector (pins)."},
		{PinoutShape{Type: "to", Package: "to92"}, "TO-92 seen from the front (flat side), leads pointing down, pins left to right."},
		{PinoutShape{Type: "to", Package: "to220"}, "TO-220 seen from the front (label side), leads pointing down, pins left to right."},
		{PinoutShape{Type: "jack"}, "Plug seen from the side, tip at the left, segments from tip to sleeve."},
		{PinoutShape{Type: "board", Connector: "none"}, "Board seen from the component side, portrait, without a main connector."},
		{PinoutShape{Type: "board", Connector: "top"}, "Board seen from the component side, portrait, main connector at the top."},
	} {
		if got := pinoutView(&Pinout{Shape: c.shape, Views: views}); got != c.view {
			t.Errorf("%s/%s view = %q, want %q", c.shape.Type, c.shape.Drawn, got, c.view)
		}
	}
	// A caption with a leading acronym keeps its capitals.
	if got := pinoutView(&Pinout{Shape: PinoutShape{Type: "rows", Drawn: "plug"}, Views: map[string]Text{"plug": {"en": "USB plug."}}}); got != "Rows as seen looking at the mating face of the USB plug." {
		t.Errorf("acronym caption = %q", got)
	}
}

func TestSearchOverviewAndItemCountCoverPinouts(t *testing.T) {
	cat := pinoutQueryCatalog(t)
	if want := len(cat.Calculators) + len(cat.Tables) + len(cat.Codes) + 3; cat.ItemCount() != want {
		t.Errorf("ItemCount = %d, want %d", cat.ItemCount(), want)
	}
	pinoutsCount := 0
	for _, c := range cat.Overview() {
		if c.ID == "pinouts" {
			pinoutsCount = c.Count
		}
	}
	if pinoutsCount != 3 {
		t.Errorf("Overview counts %d pinouts, want 3", pinoutsCount)
	}
	hits := cat.SearchAll("ethernet")
	if len(hits) == 0 || hits[0].ID != "t_rj" || hits[0].Kind != "pinout" || hits[0].Category != "pinouts" || hits[0].Summary != "Two wiring variants." {
		t.Fatalf("hits = %+v", hits)
	}
	// An item without a doc text summarizes its connector.
	hits = cat.SearchAll("timer")
	if len(hits) == 0 || hits[0].ID != "t_dip" || hits[0].Summary != "DIP package, 4 pins." {
		t.Fatalf("dip hits = %+v", hits)
	}
	// ResolveItem answers pinout; the catalog without pinouts is unchanged.
	if kind, id, err := cat.ResolveItem("T_Rj"); err != nil || kind != "pinout" || id != "t_rj" {
		t.Errorf("ResolveItem = %q %q %v", kind, id, err)
	}
	_, _, err := cat.ResolveItem("rj")
	wantQueryError(t, err, CodeUnknownID, "calculator, table, code or pinout", "t_rj")
	real := queryCatalog(t)
	if real.ItemCount() != len(real.Calculators)+len(real.Tables)+len(real.Codes)+len(real.Pinouts) {
		t.Errorf("ItemCount of the real catalog = %d", real.ItemCount())
	}
}
