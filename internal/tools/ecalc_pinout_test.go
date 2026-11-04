package tools

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"aurago/internal/ecalc"
)

// The pinout operation of the ecalc tool is tested on a fixture catalog (a MapFS next to the real units, categories and
// pin styles), never on the real pinouts: a 40-pin board with long texts and a note on every pin, an item with two
// variants whose pins have wires and notes, and a small DIP without views.

func ecalcPinoutName(v string) string {
	parts := make([]string, len(ecalc.Locales))
	for i, l := range ecalc.Locales {
		parts[i] = l + `: "` + v + `"`
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func ecalcFixtureBoard() string {
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
		fmt.Fprintf(&pins, `    - {number: "%d", name: %s, function: {en: "General purpose line %d of the header, described at length so that a page of fifty pins is large", de: "Allzweckleitung %d"}, type: %s, notes: {en: "Note for pin %d: a long hint about voltage levels, pull resistors and alternate functions of the pin.", de: "Hinweis %d"}}`+"\n", n, name, n, n, typ, n, n)
	}
	return `- id: t_board
  kind: pinout
  category: pinouts
  group: boards
  name: ` + ecalcPinoutName("Test board") + `
  doc: {en: "A large test board. Second sentence.", de: "Ein grosses Testboard."}
  notes: {en: "Board notes: 3V3 logic only. ` + strings.Repeat("A long remark about the board. ", 20) + `", de: "Boardhinweise."}
  tags: [board, testboard]
  source: "Fixture datasheet ` + strings.Repeat("with a very long title ", 20) + `"
  shape:
    type: board
    left: [` + left.String() + `]
    right: [` + right.String() + `]
    connector: bottom
  pins:
` + pins.String()
}

const ecalcFixtureJack = `- id: t_rj
  kind: pinout
  category: pinouts
  group: data
  name: NAME
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
`

const ecalcFixtureDip = `- id: t_dip
  kind: pinout
  category: pinouts
  group: ics
  name: NAME
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
`

// ecalcPinoutCatalog loads the fixture pinouts with the real units, categories and pin styles.
func ecalcPinoutCatalog(t *testing.T) *ecalc.Catalog {
	t.Helper()
	fsys := fstest.MapFS{}
	for _, name := range []string{"units.yaml", "categories.yaml", "pinstyles.yaml"} {
		data, err := os.ReadFile("../ecalc/catalog/" + name)
		if err != nil {
			t.Fatal(err)
		}
		fsys[name] = &fstest.MapFile{Data: data}
	}
	fsys["pinouts/a_board.yaml"] = &fstest.MapFile{Data: []byte(ecalcFixtureBoard())}
	fsys["pinouts/b_rj.yaml"] = &fstest.MapFile{Data: []byte(strings.Replace(ecalcFixtureJack, "NAME", ecalcPinoutName("Test jack"), 1))}
	fsys["pinouts/c_dip.yaml"] = &fstest.MapFile{Data: []byte(strings.Replace(ecalcFixtureDip, "NAME", ecalcPinoutName("Test timer"), 1))}
	cat, err := ecalc.Load(fsys)
	if err != nil {
		t.Fatalf("fixture catalog: %v", err)
	}
	return cat
}

func ecalcRunOn(t *testing.T, cat *ecalc.Catalog, req ECalcRequest) (map[string]any, VirtualDesktopExecution) {
	t.Helper()
	exec := executeECalc(ecalcDesktopConfig(), cat, req)
	return ecalcDecode(t, exec), exec
}

func pinNumberList(out map[string]any) []string {
	var numbers []string
	pins, _ := out["pins"].([]any)
	for _, p := range pins {
		numbers = append(numbers, p.(map[string]any)["number"].(string))
	}
	return numbers
}

func TestECalcPinoutAnswersPinsVariantsAndNotFound(t *testing.T) {
	cat := ecalcPinoutCatalog(t)

	// Variants: the first is the default, wires are English colour names, the variant ids are listed.
	out, exec := ecalcRunOn(t, cat, ECalcRequest{Operation: "pinout", ID: "T_RJ"})
	pins, _ := out["pins"].([]any)
	if out["status"] != "ok" || out["operation"] != "pinout" || out["id"] != "t_rj" || out["name"] != "Test jack" || out["group"] != "data" || out["variant"] != "t568a" ||
		!slices.Equal(anyStrings(out["variants"]), []string{"t568a", "t568b"}) || out["total"] != float64(4) || out["returned"] != float64(4) || len(pins) != 4 ||
		out["connector"] != "RJ45 connector, 1 row, blade contacts, 4 pins" || out["view"] != "Rows as seen looking at the mating face of the female connector (sockets)." ||
		out["notes"] != "Pair 1 is the centre pair." || out["source"] != "Fixture standard" || exec.Event != nil {
		t.Fatalf("output = %s", exec.Output)
	}
	if first := pins[0].(map[string]any); first["wire"] != "white-green" || first["name"] != "TX+" || first["type"] != "data" || first["function"] != "Transmit +" {
		t.Errorf("pin 1 = %v", first)
	}
	out, _ = ecalcRunOn(t, cat, ECalcRequest{Operation: "pinout", ID: "t_rj", Variant: "t568b", Pin: "tx+"})
	if numbers := pinNumberList(out); !slices.Equal(numbers, []string{"1"}) || out["variant"] != "t568b" || out["found"] != true || out["pins"].([]any)[0].(map[string]any)["wire"] != "white-orange" {
		t.Errorf("t568b pin tx+ = %v", out)
	}

	// Several pins share a name: all of them are returned.
	out, _ = ecalcRunOn(t, cat, ECalcRequest{Operation: "pinout", ID: "t_board", Pin: "gnd"})
	if numbers := pinNumberList(out); !slices.Equal(numbers, []string{"5", "10", "15", "20", "25", "30", "35", "40"}) || out["found"] != true || out["total"] != float64(8) {
		t.Errorf("GND = %v", out)
	}

	// A pin that matches nothing is found:false with the variants, not an error.
	out, _ = ecalcRunOn(t, cat, ECalcRequest{Operation: "pinout", ID: "t_rj", Pin: "SDA"})
	if out["status"] != "ok" || out["found"] != false || out["code"] != nil || out["total"] != float64(0) || !slices.Equal(anyStrings(out["variants"]), []string{"t568a", "t568b"}) {
		t.Errorf("SDA = %v", out)
	}
	if pins, ok := out["pins"].([]any); !ok || len(pins) != 0 {
		t.Errorf("found:false carries an empty pins list: %v", out["pins"])
	}

	// An unknown variant is an error that lists the ids; filter and paging work.
	out, _ = ecalcRunOn(t, cat, ECalcRequest{Operation: "pinout", ID: "t_rj", Variant: "t568c"})
	if out["status"] != "error" || out["code"] != "unknown_variant" || !strings.Contains(out["message"].(string), "t568a, t568b") {
		t.Errorf("unknown variant = %v", out)
	}
	out, _ = ecalcRunOn(t, cat, ECalcRequest{Operation: "pinout", ID: "t_board", Filter: "allzweckleitung 3", Offset: 1, Limit: 2})
	if numbers := pinNumberList(out); !slices.Equal(numbers, []string{"30", "31"}) || out["total"] != float64(11) || out["truncated"] != true || out["next_offset"] != float64(3) {
		t.Errorf("filtered page = %v", out)
	}
	// The id of a table says what it is (the fixture catalog has no tables, the shipped one does).
	if out := ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "pinout", ID: "awg"})); out["code"] != "not_a_pinout" || !strings.Contains(out["message"].(string), "awg is a table, not a pinout") {
		t.Errorf("pinout on a table = %v", out)
	}
	for _, c := range []struct {
		req  ECalcRequest
		code string
	}{
		{ECalcRequest{Operation: "pinout"}, "unknown_id"},
		{ECalcRequest{Operation: "pinout", ID: "t_rjj"}, "unknown_id"},
		{ECalcRequest{Operation: "pinout", ID: "t_dip", Variant: "a"}, "unknown_variant"},
		{ECalcRequest{Operation: "pinout", ID: "t_dip", Filter: strings.Repeat("x", 129)}, "invalid_filter"},
	} {
		if out, _ := ecalcRunOn(t, cat, c.req); out["status"] != "error" || out["code"] != c.code {
			t.Errorf("%+v = %v, want code %s", c.req, out, c.code)
		}
	}
}

func anyStrings(v any) []string {
	var out []string
	list, _ := v.([]any)
	for _, s := range list {
		out = append(out, s.(string))
	}
	return out
}

func TestECalcSearchDescribeAndOpenCoverPinouts(t *testing.T) {
	cat := ecalcPinoutCatalog(t)
	out, _ := ecalcRunOn(t, cat, ECalcRequest{Operation: "search"})
	cats, _ := out["categories"].([]any)
	listed := false
	for _, c := range cats {
		if c.(map[string]any)["id"] == "pinouts" {
			listed = c.(map[string]any)["count"] == float64(3)
		}
	}
	if out["total"] != float64(cat.ItemCount()) || !listed || cat.ItemCount() != len(cat.Calculators)+len(cat.Tables)+len(cat.Codes)+3 {
		t.Errorf("search without a query = %v", out)
	}
	out, _ = ecalcRunOn(t, cat, ECalcRequest{Operation: "search", Query: "ethernet"})
	results, _ := out["results"].([]any)
	if len(results) == 0 || results[0].(map[string]any)["id"] != "t_rj" || results[0].(map[string]any)["kind"] != "pinout" || results[0].(map[string]any)["category"] != "pinouts" {
		t.Fatalf("search = %v", out)
	}

	// describe: group, connector, view, pin count, variants, counts per type, notes and source, but no pins.
	out, exec := ecalcRunOn(t, cat, ECalcRequest{Operation: "describe", ID: "T_RJ"})
	variants, _ := out["variants"].([]any)
	counts, _ := out["counts"].(map[string]any)
	if out["status"] != "ok" || out["operation"] != "describe" || out["kind"] != "pinout" || out["group"] != "data" || out["pin_count"] != float64(4) || len(variants) != 2 ||
		counts["data"] != float64(3) || counts["ground"] != float64(1) || out["notes"] != "Pair 1 is the centre pair." || out["source"] != "Fixture standard" ||
		out["connector"] == nil || out["view"] == nil || out["pins"] != nil || exec.Event != nil {
		t.Fatalf("describe = %s", exec.Output)
	}
	if out, _ = ecalcRunOn(t, cat, ECalcRequest{Operation: "calculate", ID: "t_rj", Inputs: ecalcPairs("x", "1")}); out["code"] != "not_a_calculator" || !strings.Contains(out["message"].(string), "t_rj is a pinout") {
		t.Errorf("calculate on a pinout = %v", out)
	}
	if out, _ = ecalcRunOn(t, cat, ECalcRequest{Operation: "table", ID: "t_rj"}); out["code"] != "not_a_table" || !strings.Contains(out["message"].(string), "pinout reads its pins") {
		t.Errorf("table on a pinout = %v", out)
	}

	// open works with the id of a pinout and takes no values.
	exec = executeECalc(ecalcDesktopConfig(), cat, ECalcRequest{Operation: "open", ID: "t_rj"})
	out = ecalcDecode(t, exec)
	if out["status"] != "ok" || out["page"] != "t_rj" || exec.Event == nil || !strings.Contains(out["message"].(string), "Test jack pinout") || !strings.Contains(out["message"].(string), "Use pinout to read its pins") {
		t.Fatalf("open = %s", exec.Output)
	}
	if out, _ = ecalcRunOn(t, cat, ECalcRequest{Operation: "open", ID: "t_rj", Inputs: ecalcPairs("x", "1")}); out["code"] != "not_a_calculator" {
		t.Errorf("open with values = %v", out)
	}
}

// The outer bound of the tool (ecalc_budget.go) refuses a result that is over the budget. A page of pins can always be
// shortened, so it must never get that answer, for the fixture pinouts and for every pinout the catalog will ever hold:
// the loop over ecalc.Default().Pinouts is empty until the content exists and then checks every real item, every variant,
// with and without a pin, at the default budget and at the smallest budgets the agent can hand over.
func TestECalcPinoutPagesStayWithinTheBound(t *testing.T) {
	shipped, err := ecalc.Default()
	if err != nil {
		t.Fatal(err)
	}
	for name, cat := range map[string]*ecalc.Catalog{"fixture": ecalcPinoutCatalog(t), "real": shipped} {
		if name == "fixture" && len(cat.Pinouts) != 3 {
			t.Fatalf("the fixture catalog has %d pinouts", len(cat.Pinouts))
		}
		for _, p := range cat.Pinouts {
			variants := []string{""}
			for _, v := range p.Variants {
				variants = append(variants, v.ID)
			}
			for _, variant := range variants {
				pins, _, _ := p.PinsOf(variant)
				requests := []ECalcRequest{
					{Operation: "pinout", ID: p.ID, Variant: variant, Limit: 50},
					{Operation: "pinout", ID: p.ID, Variant: variant, Pin: pins[0].Name, Limit: 50},
					{Operation: "pinout", ID: p.ID, Variant: variant, Pin: pins[len(pins)-1].Number},
					{Operation: "describe", ID: p.ID},
				}
				for _, outputBudget := range []int{0, 6000, 2100, 1} {
					budget := ecalcBudget(outputBudget)
					for _, req := range requests {
						req.OutputBudget = outputBudget
						exec := executeECalc(nil, cat, req)
						out := ecalcDecode(t, exec)
						if out["status"] != "ok" || out["code"] != nil || len(exec.Output) > budget {
							t.Errorf("%s %s/%s with budget %d: %d bytes, status %v, code %v: %.300s", name, req.Operation, p.ID, budget, len(exec.Output), out["status"], out["code"], exec.Output)
							continue
						}
						if req.Operation != "pinout" {
							continue
						}
						// A shortened page says where to continue and carries at least the one pin a named pin asks for.
						listed, _ := out["pins"].([]any)
						if out["returned"] != float64(len(listed)) || (out["truncated"] == true && len(listed) > 0 && out["next_offset"] != float64(len(listed))) {
							t.Errorf("%s %s with budget %d: returned %v of %d pins, truncated %v, next_offset %v", name, p.ID, budget, out["returned"], len(listed), out["truncated"], out["next_offset"])
						}
						if req.Pin != "" && out["found"] != true {
							t.Errorf("%s %s pin %q is not found", name, p.ID, req.Pin)
						}
					}
				}
			}
		}
	}

	// The 40-pin board with a note on every pin: the default page loses the notes first and then pins; at the smallest
	// budget a page is a few pins and the next page starts right after them.
	cat := ecalcPinoutCatalog(t)
	full, _ := ecalcRunOn(t, cat, ECalcRequest{Operation: "pinout", ID: "t_board", OutputBudget: 1 << 20})
	if full["returned"] != float64(40) || full["truncated"] != nil {
		t.Fatalf("the full page = %v", full["returned"])
	}
	small, exec := ecalcRunOn(t, cat, ECalcRequest{Operation: "pinout", ID: "t_board", OutputBudget: 2100})
	listed, _ := small["pins"].([]any)
	if len(listed) < 3 || len(listed) >= 40 || small["total"] != float64(40) || small["truncated"] != true || small["next_offset"] != float64(len(listed)) ||
		!strings.Contains(small["note"].(string), "pin notes left out") || strings.Contains(exec.Output, "Note for pin") {
		t.Fatalf("a page at the smallest budget: %d pins, %s", len(listed), exec.Output)
	}
	next, _ := ecalcRunOn(t, cat, ECalcRequest{Operation: "pinout", ID: "t_board", Offset: len(listed), Limit: 3, OutputBudget: 2100})
	if next["offset"] != float64(len(listed)) || next["returned"] != float64(3) {
		t.Errorf("the next page = %v", next)
	}
}
