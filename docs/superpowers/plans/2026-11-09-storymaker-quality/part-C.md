# Part C

Tasks of batch C: missing German umlauts repaired where text is stored, the browser made ready before the agent's check, and the reason of an unavailable compile check carried to the log, the check event, the open check and the job card. Rules, commands and the order are in the plan index. Decisions are in [design.md](design.md), table "Batch C".

Batch C is independent of A and B. It touches `internal/storymaker` (a data file and two small files), `internal/server` (the job), `storymaker-agent.js` and 16 locale files. Task C1 reads the real run of 2026-11-08 (`testdata/stories/agent-firewall-run.json`). If batch A has not landed yet, create that file from `evidence/firewall-run.story.json` as A1 step 1 says (batch A creates the identical file, so the two do not conflict). C's other tests use the existing test rig and need nothing from batch A.

<!-- group C1 -->

### Task C1: The umlaut map and the repair function
**Batch:** C  **Depends on:** none
**Files:**
Create `internal/storymaker/data/de_umlauts.json`
Create `internal/storymaker/studio_umlauts.go`
Test `internal/storymaker/studio_umlauts_test.go`

Weak models write German without umlauts ("pruft", "durfen", "wahrend"). Captions and speech carry the error and nothing flags it. The repair replaces whole words from a curated map, keeps the case, and applies to language `de` only. A word is in the map only when its ASCII form is not itself a German word: "wurde", "fuhren", "losen", "losung", "wahlen", "andern" and "hohe" are real words and stay out. The map has no `ue` keys: the older fixtures write `schuetzt` and `prueft` and must stay untouched. It has no `ss` to `ß` entries either, because `ss` is correct Swiss spelling.

- [ ] **Step 1: Write the failing test.** Create `internal/storymaker/studio_umlauts_test.go`:

```go
package storymaker

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"unicode"
)

func TestRepairUmlautsRestoresWholeWordsAndKeepsTheCase(t *testing.T) {
	text := "Eine {1}Firewall pruft jeden Strom. Pruft sie? PRUFT! Sie durfen wahrend der Zeit personlichen Schutz."
	got, fixes := RepairUmlauts(text, "de")
	want := "Eine {1}Firewall prüft jeden Strom. Prüft sie? PRÜFT! Sie dürfen während der Zeit persönlichen Schutz."
	if got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	counts := map[string]int{}
	for _, f := range fixes {
		counts[f.From+">"+f.To] += f.Count
	}
	if counts["pruft>prüft"] != 1 || counts["Pruft>Prüft"] != 1 || counts["PRUFT>PRÜFT"] != 1 || counts["durfen>dürfen"] != 1 || counts["wahrend>während"] != 1 || counts["personlichen>persönlichen"] != 1 {
		t.Fatalf("fixes = %+v", fixes)
	}
}

func TestRepairUmlautsTouchesOnlyWholeWordsOfGerman(t *testing.T) {
	for _, tc := range []struct{ text, language string }{
		{"Eine Firewall prueft und schuetzt das Netz.", "de"},      // the ue spelling is a spelling, not a loss
		{"Unpruft und Pruftest sind keine Wörter der Liste.", "de"}, // whole words only
		{"Die Firewall pruft jedes Paket.", "en"},                  // another language is never touched
		{"Die Firewall pruft jedes Paket.", ""},                    // no language, no repair
		{"Furcht und Uberfluss bleiben.", "de"},                    // fur and uber inside longer words
		{"Das war schon, wurde aber fuhren und losen.", "de"},      // real German words stay
	} {
		if got, fixes := RepairUmlauts(tc.text, tc.language); got != tc.text || len(fixes) != 0 {
			t.Errorf("RepairUmlauts(%q, %q) = %q, %v: must be unchanged", tc.text, tc.language, got, fixes)
		}
	}
	if got, _ := RepairUmlauts("Er pruft das.", "de-AT"); got != "Er prüft das." {
		t.Errorf("a regional tag has the base language de, got %q", got)
	}
	if got, fixes := RepairUmlauts("", "de"); got != "" || fixes != nil {
		t.Errorf("empty text = %q, %v", got, fixes)
	}
}

// Every entry of the map is checked: a lower-case ASCII key, a lower-case value that differs from it only by umlauts, no
// real German word as key.
func TestUmlautMapEntriesAreSound(t *testing.T) {
	var entries map[string]string
	if err := json.Unmarshal(deUmlautsJSON, &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) < 30 {
		t.Fatalf("the map has only %d entries", len(entries))
	}
	replacer := strings.NewReplacer("ä", "a", "ö", "o", "ü", "u")
	// ASCII spellings that are German words of their own: they must never be keys.
	realWords := []string{"wurde", "wurden", "fuhren", "losen", "losung", "losungen", "wahlen", "andern", "hohe", "schon"}
	for key, value := range entries {
		if key != strings.ToLower(key) || value != strings.ToLower(value) {
			t.Errorf("%q: keys and values are lower case", key)
		}
		for _, r := range key {
			if r > unicode.MaxASCII || !unicode.IsLetter(r) {
				t.Errorf("%q: a key is plain ASCII letters", key)
				break
			}
		}
		if replacer.Replace(value) != key {
			t.Errorf("%q -> %q: the value must differ from the key only by umlauts", key, value)
		}
		if !strings.ContainsAny(value, "äöü") {
			t.Errorf("%q -> %q: the value holds no umlaut", key, value)
		}
	}
	for _, word := range realWords {
		if _, bad := entries[word]; bad {
			t.Errorf("%q is a real German word and must not be a key", word)
		}
	}
	for _, key := range []string{"pruft", "durfen", "wahrend", "personlichen", "stromen"} {
		if entries[key] == "" {
			t.Errorf("the words of the real run need an entry: %q", key)
		}
	}
}

// The real run of 2026-11-08 had five such words in its narration. The repair fixes them and leaves the rest alone.
func TestRepairUmlautsOnTheRealFirewallRun(t *testing.T) {
	raw, err := os.ReadFile("testdata/stories/agent-firewall-run.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Scenes []struct{ ID, Narration string }
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, sc := range doc.Scenes {
		fixed, fixes := RepairUmlauts(sc.Narration, "de")
		for _, f := range fixes {
			total += f.Count
		}
		if strings.Contains(fixed, "pruft") || strings.Contains(fixed, "durfen") || strings.Contains(fixed, "wahrend") {
			t.Errorf("scene %s still has a missing umlaut: %q", sc.ID, fixed)
		}
	}
	if total < 5 {
		t.Errorf("the run has at least 5 missing umlauts, repaired %d", total)
	}
}
```
- [ ] **Step 2: Run to see it fail:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '^(TestRepairUmlauts|TestUmlautMap)' -count=1`. Expected: FAIL to build (`RepairUmlauts`, `deUmlautsJSON` are not defined).
- [ ] **Step 3: Create `internal/storymaker/data/de_umlauts.json`:**

```json
{
  "pruft": "prüft",
  "prufen": "prüfen",
  "prufung": "prüfung",
  "uberprufen": "überprüfen",
  "uberpruft": "überprüft",
  "durfen": "dürfen",
  "wahrend": "während",
  "personlich": "persönlich",
  "personliche": "persönliche",
  "personlichen": "persönlichen",
  "stromen": "strömen",
  "fur": "für",
  "uberall": "überall",
  "ubrigens": "übrigens",
  "ubertragen": "übertragen",
  "ubersicht": "übersicht",
  "ubersichtlich": "übersichtlich",
  "moglich": "möglich",
  "unmoglich": "unmöglich",
  "moglichkeit": "möglichkeit",
  "konnen": "können",
  "konnte": "könnte",
  "konnten": "könnten",
  "mussen": "müssen",
  "mochte": "möchte",
  "mochten": "möchten",
  "fuhrt": "führt",
  "zuruck": "zurück",
  "naturlich": "natürlich",
  "taglich": "täglich",
  "spater": "später",
  "fruher": "früher",
  "gunstig": "günstig",
  "kunftig": "künftig",
  "schadlich": "schädlich",
  "gerat": "gerät",
  "gerate": "geräte",
  "zuverlassig": "zuverlässig",
  "fahigkeit": "fähigkeit",
  "verschlusselung": "verschlüsselung",
  "verschlusselt": "verschlüsselt",
  "schlussel": "schlüssel",
  "geschutzt": "geschützt",
  "beschutzen": "beschützen",
  "schutzen": "schützen",
  "unterstutzen": "unterstützen",
  "nutzlich": "nützlich",
  "gefahrlich": "gefährlich",
  "einfuhrung": "einführung",
  "gebuhr": "gebühr",
  "hauptsachlich": "hauptsächlich",
  "tatsachlich": "tatsächlich",
  "hochstens": "höchstens",
  "verandern": "verändern",
  "anderung": "änderung",
  "sicherheitslucke": "sicherheitslücke",
  "lucke": "lücke"
}
```
The map holds `fur` and `mochte` on purpose: neither is a German word. It has no `uber` alone (`uberall`, `ubertragen` and the like are in): a bare "uber" appears in English text and brand names.
- [ ] **Step 4: Create `internal/storymaker/studio_umlauts.go`:**

```go
package storymaker

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
	"unicode"
)

//go:embed data/de_umlauts.json
var deUmlautsJSON []byte

// UmlautFix is one repaired word: how it was written, what it became, and how often.
type UmlautFix struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Count int    `json:"count"`
}

// umlautNote is the sentence that travels with a repair in a tool answer.
const umlautNote = "Missing umlauts were restored in the text you sent. Write ä, ö, ü and ß yourself next time."

var (
	umlautOnce sync.Once
	umlautMap  map[string]string
)

// germanUmlautMap is the curated map of ASCII spellings that lost an umlaut to the word with it. The file is embedded and a
// test checks every entry, so a decode error means a broken build and leaves the map empty (no repair).
func germanUmlautMap() map[string]string {
	umlautOnce.Do(func() {
		umlautMap = map[string]string{}
		_ = json.Unmarshal(deUmlautsJSON, &umlautMap)
	})
	return umlautMap
}

// RepairUmlauts restores the umlauts that a weak model left out of German text. It replaces whole words only, keeps the case
// (pruft, Pruft, PRUFT) and leaves every other character, cue markers included, as it is. Only language de (any regional
// tag with base de) is touched, and only words of the curated map, whose ASCII spelling is not a German word. It returns the
// text and the distinct repairs; with nothing to repair it returns the text as given and nil.
func RepairUmlauts(text, language string) (string, []UmlautFix) {
	if text == "" || LanguageBase(language) != "de" {
		return text, nil
	}
	words := germanUmlautMap()
	if len(words) == 0 {
		return text, nil
	}
	runes := []rune(text)
	var out strings.Builder
	var fixes []UmlautFix
	for i := 0; i < len(runes); {
		if !unicode.IsLetter(runes[i]) {
			out.WriteRune(runes[i])
			i++
			continue
		}
		j := i
		for j < len(runes) && unicode.IsLetter(runes[j]) {
			j++
		}
		word := string(runes[i:j])
		if fixed, ok := repairUmlautWord(words, word); ok {
			fixes = addUmlautFix(fixes, UmlautFix{From: word, To: fixed, Count: 1})
			word = fixed
		}
		out.WriteString(word)
		i = j
	}
	if len(fixes) == 0 {
		return text, nil
	}
	return out.String(), fixes
}

// repairUmlautWord looks one word up and gives the repaired word in the case of the original.
func repairUmlautWord(words map[string]string, word string) (string, bool) {
	to, ok := words[strings.ToLower(word)]
	if !ok {
		return "", false
	}
	letters := []rune(word)
	switch {
	case len(letters) > 1 && word == strings.ToUpper(word):
		return strings.ToUpper(to), true
	case unicode.IsUpper(letters[0]):
		r := []rune(to)
		r[0] = unicode.ToUpper(r[0])
		return string(r), true
	}
	return to, true
}

// addUmlautFix adds a repair to a list, merging the counts of the same pair.
func addUmlautFix(list []UmlautFix, fix UmlautFix) []UmlautFix {
	for i := range list {
		if list[i].From == fix.From && list[i].To == fix.To {
			list[i].Count += fix.Count
			return list
		}
	}
	return append(list, fix)
}
```
- [ ] **Step 5: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '^(TestRepairUmlauts|TestUmlautMap)' -count=1 -v`. Expected: PASS. A failing `TestUmlautMapEntriesAreSound` names the entry: fix the entry in the JSON, not the test, unless the test itself is wrong about the word.
- [ ] **Step 6: Commit:** `gofmt -w` the two Go files; `git add internal/storymaker/data/de_umlauts.json internal/storymaker/studio_umlauts.go internal/storymaker/studio_umlauts_test.go`; message `feat(storymaker): curated German umlaut repair`.

<!-- group C2 -->

### Task C2: Repair at storage, and tell the model
**Batch:** C  **Depends on:** C1
**Files:**
Modify `internal/storymaker/studio.go`
Modify `internal/storymaker/studio_story_scenes.go`
Modify `internal/storymaker/studio_story.go`
Modify `internal/storymaker/studio_umlauts.go`
Test `internal/storymaker/studio_umlauts_store_test.go`

The repair runs in `writeScene` (so `put_scene` and `patch_scene` pass it, `dry_run` does not) after the fields are built and before they are evaluated and saved, and in `studioOutline` before the word count. It covers the narration and the `text` of `show` and `say` beats. The tool answer carries `repaired` and `repaired_note`, so the model learns, and the Studio counts the repairs for the job report.

- [ ] **Step 1: Impact:** `writeScene`, `runSceneOp`, `studioOutline`, `Studio` (struct). `writeScene` has one caller (`runSceneOp`). Report the risk.
- [ ] **Step 2: Implement.**

**Edit** `internal/storymaker/studio.go`: two fields in the `Studio` struct, after `castSaved`.
Old:
```go
castSaved       bool
```
New:
```go
castSaved       bool
	pendingRepairs  []UmlautFix // umlaut repairs of the call in progress; takeRepairs hands them out
	repairs         int         // umlaut repairs of the job so far, for the job report
```

**Edit** `internal/storymaker/studio_story_scenes.go`: `runSceneOp` hands the repairs to the answer.
Old:
```go
		return s.writeScene(ctx, op, sceneID, args)
```
New:
```go
		res := s.writeScene(ctx, op, sceneID, args)
		return withRepairs(res, s.takeRepairs())
```

**Edit** `internal/storymaker/studio_story_scenes.go`: the repair in `writeScene`, between the field building and the evaluation. The two lines of Old are the ones in `writeScene`; if `dryRunScene` has the same two lines, take the pair that follows the `corrections_exhausted` check.
Old:
```go
	ev := s.evaluateSceneWrite(story, sceneID, fields)
	if ev.errors == 0 {
```
New:
```go
	s.repairSceneFields(fields)
	ev := s.evaluateSceneWrite(story, sceneID, fields)
	if ev.errors == 0 {
```

**Edit** `internal/storymaker/studio_story.go`: the outline.
Old:
```go
		scenes = append(scenes, Scene{ID: id, Narration: strings.TrimSpace(row.Narration)})
```
New:
```go
		scenes = append(scenes, Scene{ID: id, Narration: s.repairText(strings.TrimSpace(row.Narration))})
```
Old:
```go
	raw, refused := studioDataJSON(args, outlineExample)
```
New:
```go
	s.pendingRepairs = nil
	raw, refused := studioDataJSON(args, outlineExample)
```
Old:
```go
	answer["next_action"] = "The outline is saved. This phase is done: stop."
```
New:
```go
	if fixes := s.takeRepairs(); len(fixes) > 0 {
		answer["repaired"] = fixes
		answer["repaired_note"] = umlautNote
	}
	answer["next_action"] = "The outline is saved. This phase is done: stop."
```

**Edit** `internal/storymaker/studio_umlauts.go`: add the Studio side. Replace the import block and append the methods.
Old:
```go
import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
	"unicode"
)
```
New:
```go
import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
	"unicode"
)

// repairText restores the umlauts of one text in the job's language and remembers what it repaired for the call's answer.
func (s *Studio) repairText(text string) string {
	fixed, fixes := RepairUmlauts(text, s.job.Language)
	for _, f := range fixes {
		s.pendingRepairs = addUmlautFix(s.pendingRepairs, f)
	}
	return fixed
}

// repairSceneFields repairs the narration and the text of show and say beats in the fields of a scene write, in place.
func (s *Studio) repairSceneFields(fields map[string]any) {
	if narration, ok := fields["narration"].(string); ok {
		fields["narration"] = s.repairText(narration)
	}
	beats, _ := fields["beats"].([]any)
	for _, item := range beats {
		beat, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if do, _ := beat["do"].(string); do != "show" && do != "say" {
			continue
		}
		if text, ok := beat["text"].(string); ok {
			beat["text"] = s.repairText(text)
		}
	}
}

// takeRepairs hands out the repairs of the call in progress and counts them for the job. A refused call leaves its repairs
// pending, and the next call that takes them is the one that saved the text.
func (s *Studio) takeRepairs() []UmlautFix {
	fixes := s.pendingRepairs
	s.pendingRepairs = nil
	for _, f := range fixes {
		s.repairs += f.Count
	}
	return fixes
}

// Repairs is the number of umlauts restored in this job's stored text so far.
func (s *Studio) Repairs() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.repairs
}

// withRepairs adds the repairs to a tool answer so that the model sees what changed in its text.
func withRepairs(res StudioResult, fixes []UmlautFix) StudioResult {
	if len(fixes) == 0 {
		return res
	}
	var body map[string]any
	if json.Unmarshal(res.JSON, &body) != nil {
		return res
	}
	body["repaired"] = fixes
	body["repaired_note"] = umlautNote
	data, err := json.Marshal(body)
	if err != nil {
		return res
	}
	res.JSON = data
	return res
}
```
`repairSceneFields` leaves `fields` alone when it has no narration or beats. The map is the one `sceneWriteFields` built from the tool call or from the saved scene, so a patch that keeps an older narration repairs that narration too.
- [ ] **Step 3: Test.** Create `internal/storymaker/studio_umlauts_store_test.go`:

```go
package storymaker

import (
	"context"
	"testing"
)

func storedScene(t *testing.T, ps *ProjectStore, projectID, sceneID string) Scene {
	t.Helper()
	raw, _, err := ps.LoadStory(context.Background(), projectID)
	if err != nil {
		t.Fatal(err)
	}
	s := mustDecode(t, string(raw))
	for _, sc := range s.Scenes {
		if sc.ID == sceneID {
			return sc
		}
	}
	t.Fatalf("no scene %s", sceneID)
	return Scene{}
}

func TestSceneWriteRepairsUmlautsAndTellsTheModel(t *testing.T) {
	studio, ps, projectID := newStudioWriteRig(t, nil)
	studio.SetPhase(PhaseFill, "s1")
	put := studioWriteCall(t, studio, ToolStory, "put_scene",
		`{"scene_id":"s1","data":{"environment":"env.office","speaker":"anna","narration":"Eine {1}Firewall pruft jedes Paket.","actors":[{"cast":"anna","at":"left","pose":"present"}],"beats":[{"do":"show","text":"Pruft alles","at":"right","entry":"pop","cue":1}]}}`)
	if put["status"] != "ok" {
		t.Fatalf("put_scene = %v", put)
	}
	fixes, _ := put["repaired"].([]any)
	if len(fixes) != 2 || put["repaired_note"] == nil {
		t.Fatalf("the answer must carry repaired and a note: %v", put)
	}
	sc := storedScene(t, ps, projectID, "s1")
	if sc.Narration != "Eine {1}Firewall prüft jedes Paket." || sc.Beats[0].Text != "Prüft alles" {
		t.Fatalf("stored narration %q, beat text %q", sc.Narration, sc.Beats[0].Text)
	}
	if studio.Repairs() != 2 {
		t.Fatalf("Repairs = %d, want 2", studio.Repairs())
	}

	// patch_scene repairs the new narration too.
	patch := studioWriteCall(t, studio, ToolStory, "patch_scene", `{"scene_id":"s1","data":{"narration":"Sie durfen {1}wahrend der Zeit durch."}}`)
	if patch["status"] != "ok" || patch["repaired"] == nil {
		t.Fatalf("patch_scene = %v", patch)
	}
	if got := storedScene(t, ps, projectID, "s1").Narration; got != "Sie dürfen {1}während der Zeit durch." {
		t.Fatalf("stored narration %q", got)
	}
	if studio.Repairs() != 4 {
		t.Fatalf("Repairs = %d, want 4", studio.Repairs())
	}

	// A text without a loss has no repair note and does not change the count.
	clean := studioWriteCall(t, studio, ToolStory, "patch_scene", `{"scene_id":"s1","data":{"narration":"Eine {1}Firewall prüft jedes Paket."}}`)
	if clean["repaired"] != nil {
		t.Fatalf("no repair expected: %v", clean)
	}
	if studio.Repairs() != 4 {
		t.Fatalf("Repairs = %d, want 4", studio.Repairs())
	}
}

func TestSceneWriteLeavesOtherLanguagesAndUeSpellingsAlone(t *testing.T) {
	studio, ps, projectID := newStudioWriteRig(t, nil)
	studio.SetPhase(PhaseFill, "s1")
	studio.job.Language = "en"
	put := studioWriteCall(t, studio, ToolStory, "put_scene",
		`{"scene_id":"s1","data":{"environment":"env.office","speaker":"anna","narration":"The firewall pruft each {1}packet.","actors":[{"cast":"anna","at":"left","pose":"present"}],"beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop","cue":1}]}}`)
	if put["status"] != "ok" || put["repaired"] != nil {
		t.Fatalf("an English job repairs nothing: %v", put)
	}
	if got := storedScene(t, ps, projectID, "s1").Narration; got != "The firewall pruft each {1}packet." {
		t.Fatalf("stored narration %q", got)
	}
	studio.job.Language = "de"
	put = studioWriteCall(t, studio, ToolStory, "patch_scene", `{"scene_id":"s1","data":{"narration":"Eine {1}Firewall prueft und schuetzt das Netz."}}`)
	if put["repaired"] != nil {
		t.Fatalf("the ue spelling is left alone: %v", put)
	}
}

func TestOutlineRepairsUmlautsBeforeTheWordCount(t *testing.T) {
	studio, ps, projectID := newStudioWriteRig(t, nil)
	studio.SetPhase(PhaseOutline, "")
	out := studioWriteCall(t, studio, ToolStory, "outline",
		`{"data":{"template":"concept_explainer","target_seconds":60,"scenes":[{"id":"s1","narration":"Eine Firewall pruft jedes Paket."},{"id":"s2","narration":"Sie darf Pakete durchlassen, die erlaubt sind."},{"id":"s3","narration":"Das schutzt dich wahrend des ganzen Tages."}]}}`)
	if out["status"] != "ok" || out["repaired"] == nil {
		t.Fatalf("outline = %v", out)
	}
	if got := storedScene(t, ps, projectID, "s1").Narration; got != "Eine Firewall prüft jedes Paket." {
		t.Fatalf("stored outline narration %q", got)
	}
	if got := storedScene(t, ps, projectID, "s3").Narration; got != "Das schutzt dich während des ganzen Tages." {
		t.Fatalf("stored outline narration %q (schutzt is a word of its own and stays)", got)
	}
	if studio.Repairs() != 2 {
		t.Fatalf("Repairs = %d, want 2", studio.Repairs())
	}
}
```
"schutzt" is not in the map, on purpose: "schützt" is the correct form, but the ASCII "schutzt" is not in the curated list, and the test shows that a word outside the list is left as it is. If you add `schutzt` to the map later, change the last expectation.
- [ ] **Step 4: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '^(TestSceneWrite|TestOutline|TestStudio)' -count=1 -v`. Expected: PASS, including the older `TestStudioPacketContinuityAndSuggestions`, which writes `schuetzt` and expects it unchanged. If `studioWriteCall` needs another args shape for `outline`, read `studio_story_outline_test.go` and copy its call.
- [ ] **Step 5: Commit:** `gofmt -w` the touched Go files; `git add internal/storymaker/studio.go internal/storymaker/studio_story_scenes.go internal/storymaker/studio_story.go internal/storymaker/studio_umlauts.go internal/storymaker/studio_umlauts_store_test.go`; message `feat(storymaker): restore missing umlauts when scene text is stored`.

<!-- group C3 -->

### Task C3: The reason of an unavailable compile, in the report
**Batch:** C  **Depends on:** none
**Files:**
Modify `internal/storymaker/studio_check.go`
Modify `internal/storymaker/agentjob.go`
Modify `internal/storymaker/agentjob_test.go`
Modify `internal/server/storymaker_agent.go`
Test `internal/storymaker/studio_check_reason_test.go`

Today an unavailable compile check shows `compile_unverified` and nothing else: the reason (no browser, a timeout, a crash) is dropped (`compilecheck.go:63-66`). A closed code now travels: `CheckReport.CompileReason`, the `compile_reason` field of the `check` event, and `Detail` on the open check. The browser's own status text can hold paths, so it goes to the log only (task C4).

- [ ] **Step 1: Impact:** `CheckReport`, `runCheck`, `DecideOutcome`, `AgentCheck`, `emitCheck`. Report callers and risk.
- [ ] **Step 2: Implement.**

**Edit** `internal/storymaker/studio_check.go`: the field (the spaces align it with its neighbours; run `gofmt -w` after).
Old:
```go
Compile                     string
```
New:
```go
Compile                     string
	CompileReason               string // the closed code of why the compile check was unavailable, "" otherwise
```

**Edit** `internal/storymaker/studio_check.go`: fill it in `runCheck`.
Old:
```go
		TargetSeconds: s.job.TargetSeconds, Compile: state,
```
New:
```go
		TargetSeconds: s.job.TargetSeconds, Compile: state, CompileReason: compileReasonCode(summary, state),
```

**Edit** `internal/storymaker/studio_check.go`: the helper, before `compileForCheck`.
Old:
```go
// compileForCheck returns the compile result of this story version. A result is kept per etag,
```
New:
```go
// CompileReasonNone is the reason code when no compile check is wired in, or when the reason text is not a code.
const CompileReasonNone = "no_compile"

var compileReasonPattern = regexp.MustCompile(`^[a-z][a-z_]{0,39}$`)

// compileReasonCode is the closed code of why a compile check was unavailable (browser_unavailable, compile_timeout,
// browser_crashed, compile_setup, no_compile), or "" when it ran. The render package gives its own code; any other text
// counts as no_compile, so free text never leaves this function.
func compileReasonCode(summary CompileSummary, state string) string {
	if state != "unavailable" {
		return ""
	}
	if compileReasonPattern.MatchString(summary.Reason) {
		return summary.Reason
	}
	return CompileReasonNone
}

// compileForCheck returns the compile result of this story version. A result is kept per etag,
```
Add `"regexp"` to the import block of `studio_check.go` (it has `context`, `fmt`, `math`, `slices`).

**Edit** `internal/storymaker/agentjob.go`: `AgentCheck` gets a detail.
Old:
```go
SceneID string `json:"scene_id,omitempty"`
```
New:
```go
SceneID string `json:"scene_id,omitempty"`
	// Detail is a closed code that explains the check, for compile_unverified the reason (browser_unavailable and so on).
	Detail string `json:"detail,omitempty"`
```
The Old line is the one in `type AgentCheck struct` (single spaces between the words). `AgentJobInfo` also has a `SceneID` field, but it is written with aligned spacing and does not match.

**Edit** `internal/storymaker/agentjob.go`: `DecideOutcome` adds the detail to `compile_unverified`.
Old:
```go
		add(outcomeCheckCompileUnverified, "")
```
New:
```go
		add(outcomeCheckCompileUnverified, "")
		for i := range checks {
			if checks[i].Code == outcomeCheckCompileUnverified {
				checks[i].Detail = in.Check.CompileReason
			}
		}
```

**Edit** `internal/server/storymaker_agent.go`: the event.
Old:
```go
"rounds_left": report.RoundsLeft, "compile": report.Compile})
```
New:
```go
"rounds_left": report.RoundsLeft, "compile": report.Compile, "compile_reason": report.CompileReason})
```
- [ ] **Step 3: Test.** Create `internal/storymaker/studio_check_reason_test.go`:

```go
package storymaker

import (
	"context"
	"testing"
)

func TestCompileReasonCodeIsClosed(t *testing.T) {
	for _, tc := range []struct {
		reason, state, want string
	}{
		{"browser_unavailable", "unavailable", "browser_unavailable"},
		{"compile_timeout", "unavailable", "compile_timeout"},
		{"no browser in tests", "unavailable", CompileReasonNone},
		{"The pinned Chromium could not be downloaded: C:\\Users\\x", "unavailable", CompileReasonNone},
		{"", "unavailable", CompileReasonNone},
		{"browser_unavailable", "ok", ""},
		{"browser_unavailable", "failed", ""},
	} {
		if got := compileReasonCode(CompileSummary{Reason: tc.reason}, tc.state); got != tc.want {
			t.Errorf("compileReasonCode(%q, %q) = %q, want %q", tc.reason, tc.state, got, tc.want)
		}
	}
}

func TestCheckReportCarriesTheCompileReason(t *testing.T) {
	studio, _, _ := newStudioWriteRig(t, func(context.Context, string) (CompileSummary, error) {
		return CompileSummary{Available: false, Reason: "browser_unavailable"}, nil
	})
	report, err := studio.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Compile != "unavailable" || report.CompileReason != "browser_unavailable" {
		t.Fatalf("compile %q reason %q", report.Compile, report.CompileReason)
	}
	outcome, checks := DecideOutcome(OutcomeInput{Progress: StudioProgress{ScenesTotal: 2, ScenesDone: 2}, Check: report})
	found := false
	for _, c := range checks {
		if c.Code == "compile_unverified" && c.Detail == "browser_unavailable" {
			found = true
		}
	}
	if !found || outcome != OutcomeReadyUnverified {
		t.Fatalf("outcome %s, checks %+v: compile_unverified must carry its reason", outcome, checks)
	}

	none, _, _ := newStudioWriteRig(t, nil)
	report, err = none.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.CompileReason == "" || report.CompileReason == "no browser in tests" {
		t.Fatalf("free text must not become the reason: %q", report.CompileReason)
	}
}
```
The rig's default compile function answers `Reason: "no browser in tests"`, so the last check shows the closed code `no_compile`.
- [ ] **Step 4: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '^(TestCompileReason|TestCheckReport|TestDecideOutcome|TestStudioCheck)' -count=1`. Expected: PASS. The old table row `environment never downgrades` expects `{Code: "compile_unverified"}` without a detail: it passes because its `Check` has no `CompileReason`. Then `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go build ./internal/server`.
- [ ] **Step 5: Commit:** `gofmt -w` the touched Go files; `git add internal/storymaker/studio_check.go internal/storymaker/agentjob.go internal/storymaker/studio_check_reason_test.go internal/server/storymaker_agent.go`; message `feat(storymaker): carry the reason of an unavailable compile check`.

<!-- group C4 -->

### Task C4: Make the browser ready before the check
**Batch:** C  **Depends on:** C3
**Files:**
Modify `internal/server/storymaker_agent.go`
Modify `internal/server/storymaker_agent_events.go`
Test `internal/server/storymaker_agent_browser_test.go`

The pinned Chromium is downloaded only by an export (`render.go:437`), so on a box that never exported, the agent's check always answers `compile: "unavailable"`. The job now calls the same `Browsers.Ensure` before a check round. There is no config key for the download: the gate is that the desktop is not read-only (a download writes to the data folder; the job already checked `Storymaker.Enabled` and `VirtualDesktop.Enabled` at its start). The download is bounded by `render.timeout_seconds`, as the export's is. A failure is logged with the browser's own status text and the job goes on without the compile check.

- [ ] **Step 1: Impact:** `runCheckRound`, `storymakerAgentJob`. `runCheckRound` is called by `runCreate` and `runRevise`.
- [ ] **Step 2: Implement.**

**Edit** `internal/server/storymaker_agent_events.go`: a progress event type.
Old:
```go
	agentEventCheck   = "check"
```
New:
```go
	agentEventCheck   = "check"
	agentEventBrowser = "browser"
```
(`gofmt` realigns the block.)

**Edit** `internal/server/storymaker_agent.go`: two fields of the job, after `before`.
Old:
```go
	before     *storymaker.Story
```
New:
```go
	before     *storymaker.Story

	browserReady    bool // the browser for the compile check answered ready
	browserAttempts int  // Ensure calls of this job, at most browserMaxAttempts
```

**Edit** `internal/server/storymaker_agent.go`: the call and the method.
Old:
```go
report, err := j.check(ctx)
```
New:
```go
j.ensureBrowser(ctx)
	report, err := j.check(ctx)
```
(The Old line occurs once, in `runCheckRound`; `finalCheck` calls `fresh.Check`.)
Old:
```go
// runCheckRound is the check step: one model loop, only if the report asks for one. Advice gets one chance per job.
```
New:
```go
// storymakerBrowserMaxAttempts is how often one job asks for the browser: the first check round and one later round.
const storymakerBrowserMaxAttempts = 2

// ensureBrowser makes the browser for the compile check ready, downloading the pinned Chromium when only that is left, as
// the export does. It is bound by render.timeout_seconds. A read-only desktop gets no download, a stopped job logs nothing,
// and a failure is logged with the browser's own status text (which can name paths, so it never goes into an event or the
// job document). The job then goes on, and the check reports compile_unverified with the reason code.
func (j *storymakerAgentJob) ensureBrowser(ctx context.Context) {
	if j.browserReady || j.browserAttempts >= storymakerBrowserMaxAttempts || ctx.Err() != nil || j.rt.browsers == nil {
		return
	}
	if j.deps.ReadOnly != nil && j.deps.ReadOnly() {
		return
	}
	j.browserAttempts++
	cfg := j.route.cfg
	limit := time.Duration(cfg.Storymaker.Render.TimeoutSeconds) * time.Second
	if limit <= 0 {
		limit = time.Hour
	}
	setup, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	last := time.Now()
	status, _ := j.rt.browsers.Ensure(setup, cfg.Storymaker.Render.BrowserPath, func(done, total int64) {
		if total > 0 && time.Since(last) >= 2*time.Second {
			last = time.Now()
			j.emit(agentEventBrowser, map[string]any{"done": done, "total": total})
		}
	})
	if ctx.Err() != nil {
		return // a stopped job is no download failure
	}
	if status.Ready {
		j.browserReady = true
		return
	}
	j.logger.Warn("[Storymaker] The browser for the agent's compile check is not ready", "job_id", j.job.ID, "source", status.Source, "can_download", status.CanDownload, "issue", status.Issue)
}

// runCheckRound is the check step: one model loop, only if the report asks for one. Advice gets one chance per job.
```
`storymaker_agent.go` imports `time` already (it uses `time.Duration`); if the compiler says otherwise, add it.
- [ ] **Step 3: Test.** The scripted-job tests run with a browser stub that cannot start (`newSMScriptEnv` stubs `NewBrowsersWithProbe`, `storymaker_agent_script_test.go:264-295`), so `Ensure` returns not ready, and the job must still finish and say why. Read `smScriptRunHappyJob` (`storymaker_agent_script_test.go`, called at line 417) and add these assertions where it reads the finished job's agent block, for both tool modes:

```go
	// The check could not run a browser, so the open checks carry the reason, closed.
	var compileCheck *storymaker.AgentCheck
	for i := range final.Agent.OpenChecks {
		if final.Agent.OpenChecks[i].Code == "compile_unverified" {
			compileCheck = &final.Agent.OpenChecks[i]
		}
	}
	if compileCheck == nil || compileCheck.Detail != "browser_unavailable" {
		t.Fatalf("compile_unverified must carry the reason browser_unavailable: %+v", final.Agent.OpenChecks)
	}
```
Adapt `final` to the variable that helper uses for the finished job. If the helper has no such variable, create `internal/server/storymaker_agent_browser_test.go` with a test that builds the environment with `newSMScriptEnv`, runs the happy job through `smScriptEnqueue` and `smScriptWait`, and makes the same assertion.

Add a unit test for the gate that does not need a scripted job, in `internal/server/storymaker_agent_browser_test.go`:

```go
package server

import (
	"context"
	"testing"
)

// A job that has already seen a ready browser, or that used its attempts, or whose context is cancelled, never calls Ensure.
func TestEnsureBrowserSkipsWhenItShould(t *testing.T) {
	j := &storymakerAgentJob{browserReady: true}
	j.ensureBrowser(context.Background()) // would panic on the nil runtime if it went on

	j = &storymakerAgentJob{browserAttempts: storymakerBrowserMaxAttempts}
	j.ensureBrowser(context.Background())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	j = &storymakerAgentJob{}
	j.ensureBrowser(ctx)
	if j.browserAttempts != 0 {
		t.Fatalf("a cancelled job must not count an attempt: %d", j.browserAttempts)
	}
}
```
- [ ] **Step 4: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/server -run '(TestEnsureBrowser|TestStorymakerAgentScripted)' -count=1 -timeout 20m`. Expected: PASS.
- [ ] **Step 5: Commit:** `gofmt -w` the touched Go files; `git add internal/server/storymaker_agent.go internal/server/storymaker_agent_events.go internal/server/storymaker_agent_browser_test.go` and the scripted test file if you changed it; message `feat(storymaker): make the browser ready before the agent's compile check`.

<!-- group C5 -->

### Task C5: Repairs in the job report, and the job card
**Batch:** C  **Depends on:** C2, C3
**Files:**
Modify `internal/storymaker/agentjob.go`
Modify `internal/server/storymaker_agent.go`
Modify `ui/js/desktop/apps/storymaker-agent.js`
Modify `ui/lang/storymaker/*.json` (16 files)

- [ ] **Step 1: Impact:** `AgentJobInfo`, `publishOutcome`. Report callers.
- [ ] **Step 2: Implement the counter.**

**Edit** `internal/storymaker/agentjob.go`: the field, after `Tokens`.
Old:
```go
	Tokens          int          `json:"tokens,omitempty"`
```
New:
```go
	Tokens          int          `json:"tokens,omitempty"`
	Repairs         int          `json:"repairs,omitempty"` // umlauts restored in the stored text of this job
```

**Edit** `internal/server/storymaker_agent.go` in `publishOutcome`:
Old:
```go
		info.Outcome, info.OpenChecks = outcome, checks
```
New:
```go
		info.Outcome, info.OpenChecks = outcome, checks
		info.Repairs = j.studio.Repairs()
```
The job's first Studio holds the count (`finalCheck` builds a second one only for the check), and `publishOutcome` already reads `j.studio.Progress`.
- [ ] **Step 3: The job card shows the reason.** `storymaker-agent.js` is a CRLF file: write `<scratch>/edit-C5-agent-js.mjs` and run it from the worktree root:

```js
import { edit } from './crlf-edit.mjs';
const F = 'ui/js/desktop/apps/storymaker-agent.js';

edit(F,
  "    const CHECK_KEYS = {\n",
  "    // The reason of an unavailable compile check (open check compile_unverified, field detail); any other code adds nothing.\n" +
  "    const COMPILE_REASON_KEYS = {\n" +
  "        browser_unavailable: 'storymaker.agent.compile_reason_browser_unavailable',\n" +
  "        compile_timeout: 'storymaker.agent.compile_reason_compile_timeout',\n" +
  "        browser_crashed: 'storymaker.agent.compile_reason_browser_crashed'\n" +
  "    };\n" +
  "    const CHECK_KEYS = {\n");

edit(F,
  "                el('span', { text: t(key) }));\n",
  "                el('span', { text: t(key) + (check.code === 'compile_unverified' && COMPILE_REASON_KEYS[check.detail] ? ' ' + t(COMPILE_REASON_KEYS[check.detail]) : '') }));\n");
console.log('storymaker-agent.js edited');
```
If batch A has landed before this task, `CHECK_KEYS` ends with the four craft entries: that does not matter for these two edits. The `check.detail` text is a closed code and is only used as a lookup key, never shown.
- [ ] **Step 4: The three sentences in 16 languages.** Write `<scratch>/add-C5-keys.mjs` and run it from the worktree root. It inserts three lines after the line with `"storymaker.agent.check_compile_unverified"` in each `ui/lang/storymaker/<lang>.json`, keeps the file's line endings and refuses to run twice:

```js
import fs from 'node:fs';
const AFTER = '"storymaker.agent.check_compile_unverified"';
const T = {
  en: ['No browser could be started here.', 'The measuring run took too long.', 'The browser stopped while measuring.'],
  de: ['Hier konnte kein Browser gestartet werden.', 'Der Messlauf hat zu lange gedauert.', 'Der Browser ist beim Messen abgestürzt.'],
  cs: ['Zde se nepodařilo spustit prohlížeč.', 'Měření trvalo příliš dlouho.', 'Prohlížeč se při měření zastavil.'],
  da: ['Der kunne ikke startes nogen browser her.', 'Målingen tog for lang tid.', 'Browseren holdt op under målingen.'],
  el: ['Δεν ήταν δυνατή η εκκίνηση προγράμματος περιήγησης εδώ.', 'Η μέτρηση διήρκεσε υπερβολικά.', 'Το πρόγραμμα περιήγησης σταμάτησε κατά τη μέτρηση.'],
  es: ['Aquí no se pudo iniciar ningún navegador.', 'La medición tardó demasiado.', 'El navegador se detuvo durante la medición.'],
  fr: ["Aucun navigateur n'a pu démarrer ici.", 'La mesure a pris trop de temps.', "Le navigateur s'est arrêté pendant la mesure."],
  hi: ['यहाँ कोई ब्राउज़र शुरू नहीं हो सका।', 'माप में बहुत अधिक समय लगा।', 'माप के दौरान ब्राउज़र रुक गया।'],
  it: ['Qui non è stato possibile avviare alcun browser.', 'La misurazione ha richiesto troppo tempo.', 'Il browser si è fermato durante la misurazione.'],
  ja: ['ここではブラウザーを起動できませんでした。', '計測に時間がかかりすぎました。', '計測中にブラウザーが停止しました。'],
  nl: ['Hier kon geen browser worden gestart.', 'De meting duurde te lang.', 'De browser stopte tijdens het meten.'],
  no: ['Ingen nettleser kunne startes her.', 'Målingen tok for lang tid.', 'Nettleseren stoppet under målingen.'],
  pl: ['Nie udało się tu uruchomić przeglądarki.', 'Pomiar trwał zbyt długo.', 'Przeglądarka zatrzymała się podczas pomiaru.'],
  pt: ['Não foi possível iniciar nenhum navegador aqui.', 'A medição demorou demasiado.', 'O navegador parou durante a medição.'],
  sv: ['Ingen webbläsare kunde startas här.', 'Mätningen tog för lång tid.', 'Webbläsaren stoppade under mätningen.'],
  zh: ['此处无法启动浏览器。', '测量耗时过长。', '测量期间浏览器已停止。'],
};
const KEYS = ['storymaker.agent.compile_reason_browser_unavailable', 'storymaker.agent.compile_reason_compile_timeout', 'storymaker.agent.compile_reason_browser_crashed'];
for (const [lang, texts] of Object.entries(T)) {
  const file = `ui/lang/storymaker/${lang}.json`;
  const source = fs.readFileSync(file, 'utf8');
  if (source.includes(KEYS[0])) throw new Error(`${file} already has the keys`);
  const eol = source.includes('\r\n') ? '\r\n' : '\n';
  const lines = source.split(eol);
  const at = lines.findIndex((line) => line.includes(AFTER));
  if (at < 0) throw new Error(`${file} lacks ${AFTER}`);
  lines.splice(at + 1, 0, ...KEYS.map((key, i) => `  "${key}": ${JSON.stringify(texts[i])},`));
  fs.writeFileSync(file, lines.join(eol));
  JSON.parse(fs.readFileSync(file, 'utf8'));
}
console.log('added 3 keys to', Object.keys(T).length, 'files');
```
Expected output: `added 3 keys to 16 files`. The line after `check_compile_unverified` is another key, so the commas keep the JSON valid.
- [ ] **Step 5: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(TestStorymakerLocalesCoverAllSixteenLanguages|TestStorymakerAppTranslationsAreTranslated|TestStorymakerAppUsesOnlyKeysItHas|TestStorymakerSourcesAreSafeAndSmall)' -count=1`, then `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker ./internal/server -run 'Storymaker|AgentJob|DecideOutcome' -count=1 -timeout 20m`. Expected: PASS. If `ui/js/desktop/apps/storymaker-agent.js` is part of a generated bundle, run `npm run build:ui` and `npm run check:ui`, and commit the rebuilt bundle with this task.
- [ ] **Step 6: Commit:** `git add internal/storymaker/agentjob.go internal/server/storymaker_agent.go ui/js/desktop/apps/storymaker-agent.js ui/lang/storymaker` plus any rebuilt bundle `git status` lists (by name); message `feat(storymaker): job report counts repairs, job card names why the compile check did not run`.

<!-- group C6 -->

### Task C6: Documents
**Batch:** C  **Depends on:** C4
**Files:**
Modify `internal/storymaker/AGENTS.md`
Modify `internal/storymaker/render/AGENTS.md`
Modify `internal/server/AGENTS.md`

- [ ] **Step 1: The compile contract changes.** `internal/storymaker/AGENTS.md` (line 111 as read on 2026-11-09):
Old:
```
- `render.Runner.CompileCheck` reuses `snapshot`, the listener, the strict proxy and the page, captures the report and answers it with an error, so the page stops. It needs no FFmpeg, is bounded to 60 s and never downloads Chromium. Without a browser the job reports `compile_unverified` and still finishes.
```
New:
```
- `render.Runner.CompileCheck` reuses `snapshot`, the listener, the strict proxy and the page, captures the report and answers it with an error, so the page stops. It needs no FFmpeg, is bounded to 60 s and never downloads Chromium itself: the agent job calls `Browsers.Ensure` before a check round (`ensureBrowser`, at most twice per job, bounded by `render.timeout_seconds`, not on a read-only desktop). Without a browser the job reports `compile_unverified` with a closed reason code in `Detail` (`browser_unavailable`, `compile_timeout`, `browser_crashed`, `compile_setup`, `no_compile`) and still finishes. The browser's own status text goes to the log only.
- `RepairUmlauts` (`studio_umlauts.go`, map in `data/de_umlauts.json`) restores missing German umlauts in the narration and in the text of show and say beats when `put_scene`, `patch_scene` and `outline` store them, for job language `de` only. Only words whose ASCII spelling is not a German word go into the map; `TestUmlautMapEntriesAreSound` checks every entry. The job report counts them (`AgentJobInfo.Repairs`).
```
- [ ] **Step 2: The browser provisioning rule.** In `internal/storymaker/render/AGENTS.md` run `grep -n "never by a status request\|download" internal/storymaker/render/AGENTS.md`. The lines about provisioning (around lines 50 and 51) say that the download is made only by the first export. Add to that bullet: `The agent job also calls \`Browsers.Ensure\` before a check round, so a first agent run downloads the pinned Chromium; a status request still never does.`
- [ ] **Step 3: The server note.** In `internal/server/AGENTS.md`, in the Storymaker section (around lines 253 to 259), add: `- The agent job makes the browser ready before each check round (\`ensureBrowser\`) and logs a failure with the browser's status text; the \`check\` event carries \`compile_reason\` (a closed code) and the \`browser\` event reports a download.`
- [ ] **Step 4: Commit:** `git add internal/storymaker/AGENTS.md internal/storymaker/render/AGENTS.md internal/server/AGENTS.md`; message `docs(storymaker): browser readiness, compile reasons and umlaut repair`.

## Batch C end

Run the batch matrix of the index from the batch C worktree. Add one real check that unit tests cannot give: the first agent run on a box without the pinned Chromium downloads it. That is the user's acceptance on aurago-test (index, "Acceptance"); a run needs no new setting, and the `browser` events in the job's event file show the download.
