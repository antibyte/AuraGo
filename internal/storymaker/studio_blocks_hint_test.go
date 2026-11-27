package storymaker

import (
	"context"
	"testing"
)

func TestSuggestedBlock(t *testing.T) {
	for _, tc := range []struct{ narration, language, want string }{
		{"Zuerst {1}prüft sie das Paket, dann leitet sie es weiter.", "de", "flow"},
		{"First we check the packet, then we send it on.", "en", "flow"},
		{"90 Prozent aller Pakete sind harmlos.", "de", "chart"},
		{"Nine of ten packets pass, that is 90%.", "en-US", "chart"},
		{"Das ist ein Kreislauf ohne Ende.", "de", "cycle"},
		{"Think of it as a loop.", "en", "cycle"},
		{"Das Netz besteht aus drei Schichten.", "de", "stack"},
		{"The stack consists of layers.", "en", "stack"},
		{"Regeln statt Zufall.", "de", "compare"},
		{"Cache versus disk.", "en", "compare"},
		{"Im Gegensatz zu früher prüft sie alles.", "de", "compare"},
		{"Es gab 2024 viele Angriffe.", "de", "chart"},
		{"Hallo Welt", "de", ""},
		{"Zuerst prüfen, dann senden.", "fr", ""},
		{"Zuerst prüfen, dann senden.", "", ""},
		{"Schritt 1 ist leicht.", "de", ""},
	} {
		if got := SuggestedBlock(tc.narration, tc.language); got != tc.want {
			t.Errorf("SuggestedBlock(%q, %q) = %q, want %q", tc.narration, tc.language, got, tc.want)
		}
	}
}

func TestFillPacketSuggestsABlock(t *testing.T) {
	ctx := context.Background()
	narrate := func(ps *ProjectStore, id, text string) {
		t.Helper()
		data, etag, err := ps.LoadStory(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		st := mustDecode(t, string(data))
		st.Scenes[0].Narration = text
		out, err := CanonicalJSON(st)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ps.SaveStory(ctx, id, out, etag); err != nil {
			t.Fatal(err)
		}
	}

	catalog, ps, id := newStudioWriteRig(t, nil)
	catalog.SetPhase(PhaseFill, "s1")
	narrate(ps, id, "Zuerst prüft sie das Paket, dann leitet sie es weiter.")
	packet, err := catalog.Packet(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if dir, _ := packet["direction"].(map[string]any); dir["suggested_block"] != "flow" {
		t.Errorf("catalog direction = %v", packet["direction"])
	}
	catalog.SetPhase(PhaseFill, "s2")
	if packet, err = catalog.Packet(ctx, false); err != nil {
		t.Fatal(err)
	}
	if dir, _ := packet["direction"].(map[string]any); dir["suggested_block"] != nil {
		t.Errorf("a scene without a hint carries none: %v", dir)
	}

	free, fps, fid := newFreeStudioRig(t)
	free.SetPhase(PhaseFill, "s1")
	narrate(fps, fid, "Der Zyklus ist eine Schleife.")
	packet, err = free.Packet(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if packet["suggested_block"] != "cycle" {
		t.Errorf("free packet suggested_block = %v", packet["suggested_block"])
	}
}
