package retrogames

import (
	"encoding/json"
	"strings"
	"testing"
)

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
