package render

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/storymaker"
)

func sfxMiniCatalog(t *testing.T) *storymaker.Catalog {
	t.Helper()
	cat, err := storymaker.LoadCatalog(os.DirFS("../testdata/catalog-mini"), "catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func sfxReportEvent(ref string, startMS, take int) map[string]any {
	return map[string]any{"kind": "sfx", "ref": ref, "start_ms": startMS, "scene_id": "s1", "take": take}
}

func TestParseReportChecksSFXEvents(t *testing.T) {
	cat := sfxMiniCatalog(t)
	entry, ok := cat.Lookup("sfx.pop")
	if !ok {
		t.Fatal("the mini catalog has no sfx.pop")
	}
	want := reportExpectation()
	want.Catalog = cat
	want.MaxEvents = 8
	parse := func(ev map[string]any) *reportRefusal {
		m := goodReport()
		m["audio_events"] = []any{ev}
		body, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		_, refusal := parseReport(body, want)
		return refusal
	}
	if refusal := parse(sfxReportEvent("sfx.pop", 100, len(entry.Takes)-1)); refusal != nil {
		t.Fatalf("the last take was refused: %+v", refusal)
	}
	for name, ev := range map[string]map[string]any{
		"take too large": sfxReportEvent("sfx.pop", 100, len(entry.Takes)),
		"negative take":  sfxReportEvent("sfx.pop", 100, -1),
		"unknown sound":  sfxReportEvent("sfx.nope", 100, 0),
		"not a sound":    sfxReportEvent("prop.lock", 100, 0),
	} {
		if refusal := parse(ev); refusal == nil || refusal.Code != CodeRenderFailed {
			t.Errorf("%s: refusal = %+v", name, refusal)
		}
	}
}

func TestParseReportEventCeiling(t *testing.T) {
	want := reportExpectation()
	want.Catalog = sfxMiniCatalog(t)
	want.MaxEvents = 1 << 20 // the ceiling wins over a larger bound
	build := func(n int) []byte {
		events := make([]any, n)
		for i := range events {
			events[i] = sfxReportEvent("sfx.pop", i%10000, 0) // goodReport lasts 10000 ms
		}
		m := goodReport()
		m["audio_events"] = events
		body, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	if _, refusal := parseReport(build(32768), want); refusal != nil {
		t.Fatalf("32768 events: %+v", refusal)
	}
	if _, refusal := parseReport(build(32769), want); refusal == nil || refusal.Code != CodeRenderFailed {
		t.Fatalf("32769 events: %+v", refusal)
	}
}

func TestSFXEventBound(t *testing.T) {
	cat := sfxMiniCatalog(t)
	story := storymaker.Story{Scenes: []storymaker.Scene{
		{ID: "a", Beats: []storymaker.Beat{
			{Do: "sfx", Sound: "sfx.typing"},              // floor(1200*1.25/90)+1 = floor(16.67)+1 = 17
			{Do: "sfx", Sound: "sfx.pop"},                 // 1
			{Do: "sfx", Sound: "sfx.typing", Duration: 3}, // max(3000, 1500): floor(3000/90)+1 = 33+1 = 34
			{Do: "show", Text: "x", Entry: "pop"},         // 1 with auto_sfx
			{Do: "show", Text: "y", Entry: "fade"},        // 0
		}},
		{ID: "c", Kind: "code", Beats: []storymaker.Beat{{Do: "sfx", Sound: "sfx.pop"}}}, // a code scene's beats never run
	}}
	if got := sfxEventBound(&story, cat); got != 17+1+34 {
		t.Errorf("without auto_sfx: %d", got)
	}
	story.Style.AutoSFX = true
	if got := sfxEventBound(&story, cat); got != 17+1+34+1 {
		t.Errorf("with auto_sfx: %d", got)
	}
}

func TestMaxRhythmFactorMatchesMotion(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(uiDirForTests, "js", "storymaker", "motion.js"))
	if err != nil {
		t.Fatal(err)
	}
	table := regexp.MustCompile(`RHYTHM\s*=\s*\{([^}]*)\}`).FindSubmatch(src)
	if table == nil {
		t.Fatal("motion.js has no RHYTHM table")
	}
	largest := 0.0
	for _, m := range regexp.MustCompile(`:\s*([0-9.]+)`).FindAllSubmatch(table[1], -1) {
		v, err := strconv.ParseFloat(string(m[1]), 64)
		if err != nil {
			t.Fatal(err)
		}
		largest = max(largest, v)
	}
	if largest != maxRhythmFactor {
		t.Fatalf("largest RHYTHM value %v, maxRhythmFactor %v", largest, maxRhythmFactor)
	}
}

func TestReportAcceptsRealTypingAndAutoSFX(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required for the real compile")
	}
	cat, err := storymaker.LoadCatalog(os.DirFS(uiDirForTests), "img/storymaker/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	scene := func(id string) string {
		return `{"id":"` + id + `","environment":"env.plain-stage","transition":"cut","beats":[` +
			`{"do":"sfx","sound":"sfx.typing"},{"do":"show","text":"Eins","at":"center","entry":"pop"}]}`
	}
	storyJSON := `{"schema":"storymaker/1","title":"T","format":{"aspect":"16:9","fps":30},` +
		`"style":{"palette":"sunny","font":"default","captions":true,"rhythm":"calm","auto_sfx":true},"voice":null,"music":null,"cast":[],"scenes":[` +
		strings.Join([]string{scene("s1"), scene("s2"), scene("s3"), scene("s4")}, ",") + `]}`
	story, _, err := storymaker.DecodeStory([]byte(storyJSON), storymaker.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}

	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	script := `import fs from 'node:fs';
import { loadPlayer, realCatalog } from './scripts/storymaker-test-support.mjs';
const story = JSON.parse(fs.readFileSync(0, 'utf8'));
process.stdout.write(JSON.stringify(loadPlayer().StorymakerCore.compile(story, {}, realCatalog()).report));`
	cmd := exec.Command(node, "--input-type=module", "-e", script)
	cmd.Dir = root
	cmd.Stdin = strings.NewReader(storyJSON)
	body, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			t.Fatalf("node: %v\n%s", err, ee.Stderr)
		}
		t.Fatalf("node: %v", err)
	}

	bound := sfxEventBound(&story, cat)
	if bound != 4*(17+1) { // each scene: floor(1200*1.25/90)+1 = 17 keys and one auto pop
		t.Fatalf("bound %d", bound)
	}
	want := reportExpect{FPS: 30, Width: 1920, Height: 1080, MaxFrames: 27000, CatalogVersion: cat.Version(),
		SceneIDs: []string{"s1", "s2", "s3", "s4"}, Catalog: cat, SFXEvents: bound}
	rep, refusal := parseReport(body, want)
	if refusal != nil {
		t.Fatalf("a real compile was refused: %+v", refusal)
	}

	typing, ok := cat.Lookup("sfx.typing")
	if !ok {
		t.Fatal("the real catalog has no sfx.typing")
	}
	keys, pops := map[string]int{}, map[string]int{}
	lastX := 0.0
	for _, ev := range rep.AudioEvents {
		switch ev.Ref {
		case "sfx.typing":
			keys[ev.SceneID]++
			if ev.SceneID == "s1" {
				lastX = max(lastX, float64(ev.StartMS)+typing.Takes[ev.Take].ImpactMS)
			}
		case "sfx.pop":
			pops[ev.SceneID]++
		}
	}
	for _, id := range want.SceneIDs {
		// Calm lasts 1500 ms. x starts at most 13.5 ms in and grows by 90 to 140 ms, so at least 11 keys and at most 17.
		if keys[id] < 11 || keys[id] > 17 || pops[id] != 1 {
			t.Errorf("scene %s: %d keys, %d pops", id, keys[id], pops[id])
		}
	}
	// The loop stops only when x reaches 1500 ms, with a step of at most 140 ms, so the last x is at least 1360 ms.
	// Rounding start_ms takes off at most 1 ms. This is past the 1200 ms a standard rhythm would stop at.
	if lastX < 1359 {
		t.Errorf("the calm typing beat stopped early: last key at %v ms", lastX)
	}
}
