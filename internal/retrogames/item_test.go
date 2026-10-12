package retrogames

import (
	"encoding/json"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestFlexTextHugeArrayStaysCappedAndCheap(t *testing.T) {
	const elements = 1_000_000
	var b strings.Builder
	b.Grow(elements*5 + 32)
	b.WriteString(`{"title":[`)
	for i := 0; i < elements; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`"ab"`)
	}
	b.WriteString(`]}`)
	input := []byte(b.String())
	b.Reset()

	var v struct {
		Title FlexText `json:"title"`
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	if err := json.Unmarshal(input, &v); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	runtime.ReadMemStats(&after)

	if len(v.Title) != maxFlexValues {
		t.Fatalf("len = %d, want %d", len(v.Title), maxFlexValues)
	}
	const limit = 32 << 20
	delta := after.TotalAlloc - before.TotalAlloc
	t.Logf("decoding %d elements allocated %d KiB", elements, delta>>10)
	if delta > limit {
		t.Fatalf("decoding a %d-element array allocated %d MiB, want under %d MiB", elements, delta>>20, limit>>20)
	}
}

func TestFlexTextStreamingEdgeCases(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"top-level string", `"a"`, "a"},
		{"top-level number", `12`, "12"},
		{"top-level null", `null`, ""},
		{"top-level bool", `true`, ""},
		{"top-level object", `{"a":["x"],"b":"y"}`, ""},
		{"empty array", `[]`, ""},
		{"object inside array is skipped", `["a",{"k":["x",{"d":"y"}],"j":"z"},"b"]`, "a,b"},
		{"array inside object inside array is skipped", `[{"k":[[["x"]]]},"b"]`, "b"},
		{"skipped deep array keeps siblings in sync", `["a",[[[["x",["y"]]]]],"b"]`, "a,b"},
		{"nested within depth", `["a",["b",["c"]]]`, "a,b,c"},
		{"nested beyond depth", `["a",["b",["c",["d",["e"]]]]]`, "a,b,c"},
		{"whitespace", " [ 1 , \"x\" ,\n [ true , 2 ] ] ", "1,x,2"},
		{"escaped string", "[\"a\x5cu00e9\"]", "aé"},
	}
	for _, tc := range cases {
		var f FlexText
		if err := f.UnmarshalJSON([]byte(tc.raw)); err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if got := f.Join(","); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
	for _, raw := range []string{``, `[`, `["a"`, `["a",`, `{"a":`, `[1,}`, `{"a":["b"]`, `nul`} {
		var f FlexText
		if err := f.UnmarshalJSON([]byte(raw)); err == nil {
			t.Errorf("UnmarshalJSON(%q) should fail", raw)
		}
	}
}

func TestFlexTextDeeplyNestedArrayIsLinearAndDepthLimited(t *testing.T) {
	const depth = 5000
	raw := `{"nested":` + strings.Repeat("[", depth) + `"x"` + strings.Repeat("]", depth) + `}`
	var v struct {
		Nested FlexText `json:"nested"`
	}
	start := time.Now()
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("deep nesting took %v", elapsed)
	}
	if len(v.Nested) != 0 {
		t.Fatalf("values beyond the depth limit were kept: %q", []string(v.Nested))
	}
}

func TestFlexTextDepthBoundary(t *testing.T) {
	var v struct {
		Within  FlexText `json:"within"`
		Beyond  FlexText `json:"beyond"`
		Mixed   FlexText `json:"mixed"`
		Numbers FlexText `json:"numbers"`
	}
	raw := `{"within":[[["a"]]],"beyond":[[[["b"]]]],"mixed":["m",[[[["n"]]]],"o"],"numbers":[[[1.5e3]]]}`
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got := v.Within.Join(","); got != "a" {
		t.Errorf("within = %q", got)
	}
	if got := v.Beyond.Join(","); got != "" {
		t.Errorf("beyond = %q", got)
	}
	if got := v.Mixed.Join(","); got != "m,o" {
		t.Errorf("mixed = %q", got)
	}
	if got := v.Numbers.First(); got != "1.5e3" {
		t.Errorf("numbers = %q", got)
	}
}

func TestFlexTextKeepsAtMostMaxFlexValues(t *testing.T) {
	parts := make([]string, 1000)
	for i := range parts {
		parts[i] = "s" + strconv.Itoa(i)
	}
	raw, err := json.Marshal(map[string][]string{"flat": parts})
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Flat FlexText `json:"flat"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(v.Flat) != 64 {
		t.Fatalf("len = %d, want 64", len(v.Flat))
	}
	if v.Flat[0] != "s0" || v.Flat[63] != "s63" {
		t.Fatalf("kept the wrong values: first %q last %q", v.Flat[0], v.Flat[63])
	}
}

func TestFlexTextIgnoresBooleansAndRejectsBadJSON(t *testing.T) {
	var v struct {
		Flag  FlexText `json:"flag"`
		Mixed FlexText `json:"mixed"`
	}
	if err := json.Unmarshal([]byte(`{"flag":true,"mixed":[false,"a",null,true,2]}`), &v); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if v.Flag.First() != "" || v.Mixed.Join(",") != "a,2" {
		t.Fatalf("flag=%q mixed=%q", v.Flag.First(), v.Mixed.Join(","))
	}
	var f FlexText
	if err := f.UnmarshalJSON([]byte(`[1,`)); err == nil {
		t.Fatal("truncated JSON should return an error")
	}
}

func TestFlexTextDecodesStringsNumbersAndArrays(t *testing.T) {
	var v struct {
		Year    FlexText `json:"year"`
		YearS   FlexText `json:"year_s"`
		Creator FlexText `json:"creator"`
		Subject FlexText `json:"subject"`
		Object  FlexText `json:"object"`
		Nested  FlexText `json:"nested"`
	}
	raw := `{"year":1990,"year_s":"1991","creator":["A","B"],"subject":null,"object":{"a":1},"nested":[["x"],"y",3]}`
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if v.Year.First() != "1990" || v.YearS.First() != "1991" {
		t.Fatalf("years = %q %q", v.Year.First(), v.YearS.First())
	}
	if v.Creator.Join(", ") != "A, B" {
		t.Fatalf("creator = %q", v.Creator.Join(", "))
	}
	if v.Subject.First() != "" || v.Object.First() != "" {
		t.Fatalf("null/object should be empty: %q %q", v.Subject.First(), v.Object.First())
	}
	if v.Nested.Join(",") != "x,y,3" {
		t.Fatalf("nested = %q", v.Nested.Join(","))
	}
}

func TestNormalizeItem(t *testing.T) {
	dos, _ := SourceByKey(PlatformMSDOS)
	g, ok := normalizeItem(dos, Item{
		Identifier: "msdos_Oregon_Trail_The_1990",
		Title:      FlexText{"Oregon Trail, The"},
		Year:       FlexText{"1990"},
		Creator:    FlexText{"MECC", "Broderbund"},
		Downloads:  FlexText{"13973895"},
		Emulator:   FlexText{"dosbox"},
		Subject:    FlexText{"educational", "trail"},
	})
	if !ok {
		t.Fatal("normalizeItem rejected a valid item")
	}
	if g.Title != "The Oregon Trail" || g.Year != 1990 || g.Creator != "MECC, Broderbund" || g.Platform != PlatformMSDOS || g.System != "MS-DOS" || g.Emulator != "dosbox" || g.Downloads != 13973895 {
		t.Fatalf("game = %+v", g)
	}
	if !strings.Contains(g.Tags, "Oregon Trail, The") || !strings.Contains(g.Tags, "educational") {
		t.Fatalf("tags = %q", g.Tags)
	}

	console, _ := SourceByKey(PlatformConsole)
	g, ok = normalizeItem(console, Item{Identifier: "sg_Golden_Axe_Rev_1_1989_Sega", Title: FlexText{"Golden Axe Rev 1"}, Emulator: FlexText{"genesis"}, Downloads: FlexText{"1.5e3"}})
	if !ok || g.System != "Mega Drive / Genesis" || g.Downloads != 1500 {
		t.Fatalf("console game = %+v ok=%v", g, ok)
	}

	amiga, _ := SourceByKey(PlatformAmiga)
	g, ok = normalizeItem(amiga, Item{Identifier: "Turrican_II", Title: FlexText{" Turrican II   The Final Fight (1991)(Rainbow Arts)[cr Defjam]"}, Emulator: FlexText{"sae-a500p"}})
	if !ok || g.Title != "Turrican II The Final Fight" || g.Year != 1991 {
		t.Fatalf("tosec year fallback = %+v", g)
	}
}

func TestNormalizeItemRejectsIncompleteRecords(t *testing.T) {
	dos, _ := SourceByKey(PlatformMSDOS)
	cases := []Item{
		{Identifier: "../etc", Title: FlexText{"x"}, Emulator: FlexText{"dosbox"}},
		{Identifier: "ok_id", Title: nil, Emulator: FlexText{"dosbox"}},
		{Identifier: "ok_id", Title: FlexText{"x"}},
	}
	for i, it := range cases {
		if _, ok := normalizeItem(dos, it); ok {
			t.Errorf("case %d accepted: %+v", i, it)
		}
	}
}

func TestNormalizeItemRejectsBooleanEmulator(t *testing.T) {
	dos, _ := SourceByKey(PlatformMSDOS)
	var it Item
	if err := json.Unmarshal([]byte(`{"identifier":"ok_id","title":"x","emulator":false}`), &it); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got := it.Emulator.First(); got != "" {
		t.Fatalf("Emulator.First() = %q, want empty", got)
	}
	if _, ok := normalizeItem(dos, it); ok {
		t.Fatal("item with a boolean emulator was accepted")
	}
}

func TestNormalizeItemTitleUsesFirstValueAndTagsKeepAll(t *testing.T) {
	dos, _ := SourceByKey(PlatformMSDOS)
	g, ok := normalizeItem(dos, Item{Identifier: "foo", Title: FlexText{"Foo", "Foo (alt)"}, Emulator: FlexText{"dosbox"}})
	if !ok {
		t.Fatal("rejected")
	}
	if g.Title != "Foo" {
		t.Fatalf("title = %q, want %q", g.Title, "Foo")
	}
	if !strings.Contains(g.Tags, "Foo (alt)") {
		t.Fatalf("tags = %q, want it to contain the alternate title", g.Tags)
	}
}

func TestNormalizeItemRejectsMalformedEmulator(t *testing.T) {
	dos, _ := SourceByKey(PlatformMSDOS)
	for _, emu := range []string{"dos box", "dos/box", "dosbox!", "dös", ""} {
		if _, ok := normalizeItem(dos, Item{Identifier: "ok_id", Title: FlexText{"x"}, Emulator: FlexText{emu}}); ok {
			t.Errorf("emulator %q accepted", emu)
		}
	}
	for _, emu := range []string{"dosbox", "DOSBox", " dosbox ", "sae-a500p", "np2.v2", "a_b"} {
		if _, ok := normalizeItem(dos, Item{Identifier: "ok_id", Title: FlexText{"x"}, Emulator: FlexText{emu}}); !ok {
			t.Errorf("emulator %q rejected", emu)
		}
	}
}

func TestNormalizeItemCapsLongText(t *testing.T) {
	dos, _ := SourceByKey(PlatformMSDOS)
	g, ok := normalizeItem(dos, Item{Identifier: "long", Title: FlexText{strings.Repeat("é", 300)}, Creator: FlexText{strings.Repeat("c", 300)}, Emulator: FlexText{"dosbox"}, Subject: FlexText{strings.Repeat("s ", 500)}})
	if !ok {
		t.Fatal("rejected")
	}
	if len([]rune(g.Title)) > maxTitleRunes || len([]rune(g.Creator)) > maxCreatorRunes || len([]rune(g.Tags)) > maxTagsRunes {
		t.Fatalf("lengths title=%d creator=%d tags=%d", len([]rune(g.Title)), len([]rune(g.Creator)), len([]rune(g.Tags)))
	}
}

func TestPopRank(t *testing.T) {
	cases := map[int64]int{0: 0, 1: 1, 2: 2, 1023: 10, 1024: 11}
	for in, want := range cases {
		if got := popRank(in); got != want {
			t.Errorf("popRank(%d) = %d, want %d", in, got, want)
		}
	}
}
