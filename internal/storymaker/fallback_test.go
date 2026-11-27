package storymaker

import (
	"context"
	"strings"
	"testing"
)

func TestFallbackHeadline(t *testing.T) {
	long := strings.Repeat("x", 60)
	for _, tc := range []struct{ narration, title, want string }{
		{"Eine Firewall prüft jedes Paket am Eingang deines Netzes. Danach…", "Titel", "Eine Firewall prüft jedes Paket am Eingang deines"},
		{"Was ist ein Agent? Er plant.", "Titel", "Was ist ein Agent?"},
		{"Stopp! Hier {1}geht es nicht weiter.", "Titel", "Stopp!"},
		{"Version 3.5 ist da: neu und schnell.", "Titel", "Version 3.5 ist da"},
		{"Erstens, zweitens, drittens, viertens, fünftens, sechstens, siebtens, achtens, neuntens", "Titel", "Erstens, zweitens, drittens, viertens, fünftens, sechstens, siebtens, achtens"},
		{"Eins zwei drei vier – fünf sechs sieben acht neun", "Titel", "Eins zwei drei vier – fünf sechs sieben"},
		{"Eins –", "Titel", "Eins"},
		{long + " Ende.", "Titel", strings.Repeat("x", 40) + " Ende"},
		{"", "Wie eine Firewall schützt", "Wie eine Firewall schützt"},
		{"{1}", "Titel", "Titel"},
		{"", "", ""},
		{"Eine *markierte* Aussage.", "Titel", "Eine markierte Aussage"},
	} {
		if got := FallbackHeadline(tc.narration, tc.title); got != tc.want {
			t.Errorf("FallbackHeadline(%q, %q) = %q, want %q", tc.narration, tc.title, got, tc.want)
		}
	}
	// Astral characters count two UTF-16 units each, as the validator and the player count them.
	emoji := strings.TrimSpace(strings.Repeat(strings.Repeat("🔥", 12)+" ", 6))
	if got := FallbackHeadline(emoji, ""); got == "" || jsLen(got) > blockHeadlineMax {
		t.Errorf("an emoji headline %q has %d units, want 1 to %d", got, jsLen(got), blockHeadlineMax)
	}
	// 8 words of 40 characters must still fit the 80 characters of a headline.
	words := strings.TrimSpace(strings.Repeat(strings.Repeat("w", 40)+" ", 8))
	if got := FallbackHeadline(words, ""); len([]rune(got)) > blockHeadlineMax {
		t.Errorf("a headline of %d characters does not fit", len([]rune(got)))
	}
}

func TestApplyFallbackBoard(t *testing.T) {
	filled := func() *Scene {
		return &Scene{ID: "s1", Kind: "code", Narration: "N", Environment: "env.office", Actors: []Actor{{Cast: "anna"}}, Props: []Prop{{Item: "prop.lock"}}, Beats: []Beat{{Do: "wait", Seconds: 1}}}
	}
	sc := filled()
	if src := ApplyFallbackBoard(sc, "Was ist ein Agent?", ModeCatalog); src != "" {
		t.Errorf("catalog source = %q", src)
	}
	if sc.Environment != BoardEnvironmentID || sc.Kind != "" || len(sc.Actors) != 0 || len(sc.Props) != 0 || sc.Narration != "N" {
		t.Errorf("catalog scene = %+v", sc)
	}
	if len(sc.Beats) != 1 || sc.Beats[0].Do != "show" || sc.Beats[0].Text != "Was ist ein Agent?" || sc.Beats[0].Style != "headline" ||
		sc.Beats[0].At == nil || sc.Beats[0].At.Spot != "full" || sc.Beats[0].Entry != "fade" {
		t.Errorf("catalog beats = %+v", sc.Beats)
	}

	sc = filled()
	src := ApplyFallbackBoard(sc, `Er sagt "Hallo" <b>`, ModeFree)
	want := "export default function scene(api) {\n  api.headline(\"Er sagt \\\"Hallo\\\" \\u003cb\\u003e\", { at: 'full', entry: 'fade' });\n}\n"
	if src != want {
		t.Errorf("free source = %q, want %q", src, want)
	}
	if sc.Kind != "code" || sc.Environment != BoardEnvironmentID || sc.Actors != nil || sc.Props != nil || sc.Beats != nil || sc.Narration != "N" {
		t.Errorf("free scene = %+v", sc)
	}
}

func TestStudioFallbackBoards(t *testing.T) {
	ctx := context.Background()
	check := func(t *testing.T, st *Story, cat *Catalog) {
		t.Helper()
		if issues := Validate(st, cat, DefaultLimits()); HasErrors(issues) {
			t.Errorf("the saved story has errors: %s", dump(issues))
		}
	}

	t.Run("catalog", func(t *testing.T) {
		studio, ps, id := newStudioWriteRig(t, nil)
		got, err := studio.FallbackBoards(ctx, []string{"s1", "s2", "ghost"})
		if err != nil || len(got) != 2 || got[0] != "s1" || got[1] != "s2" {
			t.Fatalf("FallbackBoards = %v, %v", got, err)
		}
		data, _, err := ps.LoadStory(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		st := mustDecode(t, string(data))
		for _, sc := range st.Scenes {
			if sc.Environment != BoardEnvironmentID || len(sc.Beats) != 1 || sc.Beats[0].Style != "headline" || sc.Kind == "code" {
				t.Errorf("scene %s = %+v", sc.ID, sc)
			}
		}
		if st.Scenes[1].Beats[0].Text != "Sie prueft jedes Paket" {
			t.Errorf("headline = %q", st.Scenes[1].Beats[0].Text)
		}
		check(t, &st, loadMini(t))
		// A filled scene is not touched: the same call finds nothing to do.
		if got, err = studio.FallbackBoards(ctx, []string{"s1", "s2"}); err != nil || len(got) != 0 {
			t.Errorf("second call = %v, %v", got, err)
		}
	})

	t.Run("free", func(t *testing.T) {
		studio, ps, id := newFreeStudioRig(t)
		got, err := studio.FallbackBoards(ctx, []string{"s2"})
		if err != nil || len(got) != 1 || got[0] != "s2" {
			t.Fatalf("FallbackBoards = %v, %v", got, err)
		}
		src, _, err := ps.LoadCodeSource(ctx, id, "s2")
		if err != nil || !strings.Contains(string(src), `api.headline("Sie prueft jedes Paket"`) {
			t.Fatalf("source = %q, %v", src, err)
		}
		if bundle, err := ps.LoadCodeBundle(ctx, id, "s2"); err != nil || len(bundle) == 0 {
			t.Errorf("bundle = %d bytes, %v", len(bundle), err)
		}
		data, _, err := ps.LoadStory(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		st := mustDecode(t, string(data))
		if st.Scenes[1].Kind != "code" || st.Scenes[1].Environment != BoardEnvironmentID || len(st.Scenes[1].Beats) != 0 || st.Scenes[0].Kind == "code" {
			t.Errorf("scenes = %+v", st.Scenes)
		}
		check(t, &st, loadReal(t))
		// A source the model saved but never activated is replaced.
		if _, res, err := ps.SaveCodeScene(ctx, id, "s1", []byte("export default function scene(api) { api.duration(3); }\n"), MustNotExist); err != nil || len(res.Errors) != 0 {
			t.Fatalf("SaveCodeScene: %v %+v", err, res)
		}
		if got, err = studio.FallbackBoards(ctx, []string{"s1"}); err != nil || len(got) != 1 {
			t.Fatalf("FallbackBoards over a saved source = %v, %v", got, err)
		}
		if src, _, _ = ps.LoadCodeSource(ctx, id, "s1"); !strings.Contains(string(src), "api.headline(") {
			t.Errorf("source = %q", src)
		}
	})

	t.Run("read-only", func(t *testing.T) {
		studio, ps, id := fixtureStudio(t, func() bool { return true })
		before, _, err := ps.LoadStory(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := studio.FallbackBoards(ctx, []string{"s1"}); got != nil || err != nil {
			t.Errorf("read-only FallbackBoards = %v, %v", got, err)
		}
		after, _, _ := ps.LoadStory(ctx, id)
		if string(before) != string(after) {
			t.Error("a read-only job changed the story")
		}
	})
}
