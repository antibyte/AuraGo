# Part D: Storyboard entry, validation, measuring and app codes

> Generated from the drafted and tested commits 0831f87f9..152499ce2. Apply the tasks in order. "replace ... with" blocks are exact: the old text occurs once in the file before the task.

### Task D1: model types and codes

**Files:**
- Create: `internal/storymaker/blocks_model_test.go`
- Modify: `internal/storymaker/defaults.go`
- Modify: `internal/storymaker/model.go`

- [ ] **Step 1: Write the failing test**

Create `internal/storymaker/blocks_model_test.go`:

```go
package storymaker

import (
	"bytes"
	"reflect"
	"testing"
)

const blocksStorySrc = `{"schema":"storymaker/1","title":"B","format":{"aspect":"16:9","fps":30},"style":{"palette":"sunny","font":"default","captions":true},"cast":[],"scenes":[{"id":"s1","narration":"Anfrage Server {1}x","beats":[
 {"do":"show","diagram":{"layout":"flow","flow":true,"nodes":[{"id":"req","label":"Anfrage","icon":"icon.send","cue":1},{"id":"srv","label":"Server","points":["a","b"]}]},"at":"full","entry":"draw"},
 {"do":"show","icon":"icon.cpu","style":"card","text":"CPU","at":"board","entry":"draw"},
 {"do":"show","chart":"bar","data":{"labels":["Warten","Rechnen"],"values":[90,10],"unit":"%","title":"Zeit"},"at":"full","entry":"fade"}]}]}`

func TestDecodeStoryKeepsDiagrams(t *testing.T) {
	s := mustDecode(t, blocksStorySrc)
	first, err := CanonicalJSON(s)
	if err != nil {
		t.Fatal(err)
	}
	again, err := CanonicalJSON(mustDecode(t, string(first)))
	if err != nil || !bytes.Equal(first, again) {
		t.Fatalf("canonical form is not stable:\n%s\n%s", first, again)
	}
	b := s.Scenes[0].Beats
	if b[0].Diagram == nil || b[0].Diagram.Layout != "flow" || !b[0].Diagram.Flow || len(b[0].Diagram.Nodes) != 2 ||
		b[0].Diagram.Nodes[0].Cue != 1 || b[0].Diagram.Nodes[0].Icon != "icon.send" || len(b[0].Diagram.Nodes[1].Points) != 2 {
		t.Fatalf("diagram lost: %+v", b[0].Diagram)
	}
	if b[2].Data == nil || b[2].Data.Unit != "%" || b[2].Data.Title != "Zeit" {
		t.Fatalf("chart unit or title lost: %+v", b[2].Data)
	}
	if got, want := deriveShowIDs(&s.Scenes[0]), []string{"diagram-1", "icon-cpu-1", "chart-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("show ids = %v, want %v", got, want)
	}
	if BlockAreas[len(BlockAreas)-1] != "board" || len(BlockAreas) != 6 {
		t.Fatalf("BlockAreas = %v", BlockAreas)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run '(?i)(TestDecodeStoryKeepsDiagrams)' -count=1`
Expected: build failed, Beat.Diagram, ChartData.Unit/Title and BlockAreas undefined

- [ ] **Step 3: Implement**

In `internal/storymaker/defaults.go`, replace:

```go
	switch {
	case b.Item != "":
		return assetSlug(b.Item)
	case b.Text != "":
		return "text"
	case b.Icon != "":
		return cleanSlug(b.Icon, "icon")
	case b.Chart != "":
		return "chart"
	}
	return "show"
}
```

with:

```go
	switch {
	case b.Item != "":
		return assetSlug(b.Item)
	case b.Diagram != nil:
		return "diagram"
	case b.Icon != "":
		return cleanSlug(b.Icon, "icon")
	case b.Chart != "":
		return "chart"
	case b.Text != "":
		return "text"
	}
	return "show"
}
```

In `internal/storymaker/model.go`, replace:

```go
	CodeOffscreen        = "offscreen"
	CodeActorPropOverlap = "actor_prop_overlap"
	CodeSceneStatic      = "scene_static"
)

// Issue codes of the measured picture checks (layout boxes, stills, frame probe, player lint). The check emits them, never
// Validate. The two hard codes are model-fixable warnings that block an outcome while left; the others are advice or an open
// check.
```

with:

```go
	CodeOffscreen        = "offscreen"
	CodeActorPropOverlap = "actor_prop_overlap"
	CodeSceneStatic      = "scene_static"
	CodeBlockTooSmall    = "block_too_small" // a block box is below the minimum height or its label below the minimum size
)

// Codes of the block advice and of the fallback board. The advice codes are warnings of the advice lint, scene_fallback is
// an open check of the job.
const (
	CodeLabelLong     = "label_long"     // warning, advice: node label > 3 words or headline > 8 words
	CodeNoBlocks      = "no_blocks"      // warning, advice: film of >= 5 scenes without a block
	CodeBlockRepeat   = "block_repeat"   // warning, advice: the same block key in 3 scenes in a row
	CodeSceneFallback = "scene_fallback" // open check of the job: the scene got the fallback board
)

// BlockAreas are the named areas a block can be shown at.
var BlockAreas = []string{"full", "left", "right", "top", "center", "board"}

// Issue codes of the measured picture checks (layout boxes, stills, frame probe, player lint). The check emits them, never
// Validate. The two hard codes are model-fixable warnings that block an outcome while left; the others are advice or an open
// check.
```

In `internal/storymaker/model.go`, replace:

```go
type ChartData struct {
	Labels []string  `json:"labels"`
	Values []float64 `json:"values"`
}

// Beat is one timed action of a storyboard scene. Per-kind required fields are checked by
```

with:

```go
type ChartData struct {
	Labels []string  `json:"labels"`
	Values []float64 `json:"values"`
	Unit   string    `json:"unit,omitempty"`
	Title  string    `json:"title,omitempty"`
}

// Diagram is a diagram block of a show beat.
type Diagram struct {
	Layout string        `json:"layout"` // flow | cycle | hub | stack | compare
	Nodes  []DiagramNode `json:"nodes"`
	Flow   bool          `json:"flow,omitempty"`
}

// DiagramNode is one node of a Diagram.
type DiagramNode struct {
	ID     string   `json:"id"`
	Label  string   `json:"label"`
	Icon   string   `json:"icon,omitempty"`
	Points []string `json:"points,omitempty"` // compare only
	Cue    int      `json:"cue,omitempty"`    // narration marker; set by the Studio from cue_phrase
}

// Beat is one timed action of a storyboard scene. Per-kind required fields are checked by
```

In `internal/storymaker/model.go`, replace:

```go
	Icon       string     `json:"icon,omitempty"`
	Chart      string     `json:"chart,omitempty"`
	Data       *ChartData `json:"data,omitempty"`
	At         *Position  `json:"at,omitempty"`
	Entry      string     `json:"entry,omitempty"`
	Scale      float64    `json:"scale,omitempty"` // 0.5 to 2, a show beat with an item only; 0 = unset
```

with:

```go
	Icon       string     `json:"icon,omitempty"`
	Chart      string     `json:"chart,omitempty"`
	Data       *ChartData `json:"data,omitempty"`
	Diagram    *Diagram   `json:"diagram,omitempty"`
	At         *Position  `json:"at,omitempty"`
	Entry      string     `json:"entry,omitempty"`
	Scale      float64    `json:"scale,omitempty"` // 0.5 to 2, a show beat with an item only; 0 = unset
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker -run '(?i)(TestDecodeStoryKeepsDiagrams)' -count=1`
Expected: TestDecodeStoryKeepsDiagrams PASS, whole internal/storymaker package ok

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/blocks_model_test.go internal/storymaker/defaults.go internal/storymaker/model.go
git commit -m "storymaker: model types and codes"
```

### Task D2: Go validates block shows

**Files:**
- Modify: `internal/storymaker/model.go`
- Modify: `internal/storymaker/validate.go`
- Create: `internal/storymaker/validate_blocks.go`
- Create: `internal/storymaker/validate_blocks_test.go`
- Modify: `internal/storymaker/validate_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/storymaker/validate_blocks_test.go`:

```go
package storymaker

import "testing"

func blockScene(beat string) string {
	return `{"id":"s1","narration":"Anfrage {1} Server {2}","beats":[` + beat + `]}`
}

func TestValidateBlocks(t *testing.T) {
	valid := []string{
		`{"do":"show","icon":"icon.cpu","style":"card","text":"CPU","at":"board","entry":"draw"}`,
		`{"do":"show","icon":"icon.cpu","at":{"x":50,"y":50},"entry":"pop"}`,
		`{"do":"show","diagram":{"layout":"flow","flow":true,"nodes":[{"id":"req","label":"Anfrage","icon":"icon.send","cue":1},{"id":"srv","label":"Server","icon":"icon.server","cue":2}]},"at":"full","entry":"draw"}`,
		`{"do":"show","diagram":{"layout":"compare","nodes":[{"id":"a","label":"A","points":["x","y","z"]},{"id":"b","label":"B"}]},"at":"left","entry":"fade"}`,
		`{"do":"show","chart":"bar","data":{"labels":["Warten","Rechnen"],"values":[90,10],"unit":"%","title":"Zeit"},"at":"full","entry":"fade"}`,
		`{"do":"show","text":"Kleine Modelle sind *schnell*","style":"headline","at":"top","entry":"fade"}`,
		`{"do":"show","text":"Laden\nRechnen\nAntworten","style":"list","at":"right","entry":"fade"}`,
		`{"do":"chart","chart":"bar","data":{"labels":["a"],"values":[1]},"at":"right"}`,
	}
	for _, beat := range valid {
		if issues := validateScenes(t, blockScene(beat)); len(issues) != 0 {
			t.Errorf("%s:%s", beat, dump(issues))
		}
	}
	const bp = "/scenes/0/beats/0"
	chart := func(typ, data string) string {
		return `{"do":"show","chart":"` + typ + `","data":` + data + `,"at":"full","entry":"fade"}`
	}
	node := func(layout, nodes string) string {
		return `{"do":"show","diagram":{"layout":"` + layout + `","nodes":[` + nodes + `]},"at":"full","entry":"fade"}`
	}
	two := `{"id":"a","label":"A"},{"id":"b","label":"B"}`
	long := "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
	invalid := []struct {
		name, beat, code, severity, path string
	}{
		{"unknown icon", `{"do":"show","icon":"warning","at":"center","entry":"pop"}`, CodeUnknownAsset, SeverityError, bp + "/icon"},
		{"icon style", `{"do":"show","icon":"icon.cpu","style":"big","at":"center","entry":"pop"}`, CodeInvalidValue, SeverityError, bp + "/style"},
		{"label without card", `{"do":"show","icon":"icon.cpu","style":"badge","text":"CPU","at":"center","entry":"pop"}`, CodeInvalidValue, SeverityWarning, bp + "/text"},
		{"icon and chart", `{"do":"show","icon":"icon.cpu","chart":"bar","data":{"labels":["a"],"values":[1]},"at":"full","entry":"fade"}`, CodeInvalidValue, SeverityError, bp + "/item"},
		{"headline too long", `{"do":"show","text":"` + long + `","style":"headline","at":"top","entry":"fade"}`, CodeLimitExceeded, SeverityError, bp + "/text"},
		{"headline stars", `{"do":"show","text":"a *b","style":"headline","at":"top","entry":"fade"}`, CodeInvalidValue, SeverityWarning, bp + "/text"},
		{"list of five", `{"do":"show","text":"a\nb\nc\nd\ne","style":"list","at":"right","entry":"fade"}`, CodeLimitExceeded, SeverityError, bp + "/text"},
		{"list line long", `{"do":"show","text":"` + long + `","style":"list","at":"right","entry":"fade"}`, CodeInvalidValue, SeverityWarning, bp + "/text"},
		{"chart type", chart("donut", `{"labels":["a"],"values":[1]}`), CodeInvalidValue, SeverityError, bp + "/chart"},
		{"chart without data", `{"do":"show","chart":"bar","at":"full","entry":"fade"}`, CodeMissingField, SeverityError, bp + "/data"},
		{"chart lengths", chart("bar", `{"labels":["a","b"],"values":[1]}`), CodeInvalidValue, SeverityError, bp + "/data"},
		{"chart count", chart("pie", `{"labels":["a"],"values":[1]}`), CodeLimitExceeded, SeverityError, bp + "/data"},
		{"pie negative", chart("pie", `{"labels":["a","b"],"values":[3,-1]}`), CodeInvalidValue, SeverityError, bp + "/data/values"},
		{"pie zero sum", chart("pie", `{"labels":["a","b"],"values":[0,0]}`), CodeInvalidValue, SeverityError, bp + "/data/values"},
		{"chart label", chart("bar", `{"labels":["`+long+`"],"values":[1]}`), CodeInvalidValue, SeverityWarning, bp + "/data/labels/0"},
		{"chart unit", chart("bar", `{"labels":["a"],"values":[1],"unit":"123456789"}`), CodeInvalidValue, SeverityWarning, bp + "/data/unit"},
		{"chart title", chart("bar", `{"labels":["a"],"values":[1],"title":"`+long+`"}`), CodeInvalidValue, SeverityWarning, bp + "/data/title"},
		{"legacy chart type", `{"do":"chart","chart":"donut","data":{"labels":["a"],"values":[1]},"at":"right"}`, CodeInvalidValue, SeverityError, bp + "/chart"},
		{"legacy chart data", `{"do":"chart","chart":"bar","at":"right"}`, CodeMissingField, SeverityError, bp + "/data"},
		{"diagram layout", node("ring", two), CodeInvalidValue, SeverityError, bp + "/diagram/layout"},
		{"diagram count", node("flow", `{"id":"a","label":"A"}`), CodeLimitExceeded, SeverityError, bp + "/diagram/nodes"},
		{"node slug", node("flow", `{"id":"A b","label":"A"},{"id":"b","label":"B"}`), CodeInvalidValue, SeverityError, bp + "/diagram/nodes/0/id"},
		{"node duplicate", node("flow", `{"id":"a","label":"A"},{"id":"a","label":"B"}`), CodeDuplicateID, SeverityError, bp + "/diagram/nodes/1/id"},
		{"node label empty", node("flow", `{"id":"a","label":""},{"id":"b","label":"B"}`), CodeMissingField, SeverityError, bp + "/diagram/nodes/0/label"},
		{"node label long", node("flow", `{"id":"a","label":"`+long+`"},{"id":"b","label":"B"}`), CodeLimitExceeded, SeverityError, bp + "/diagram/nodes/0/label"},
		{"node icon", node("flow", `{"id":"a","label":"A","icon":"icon.nope"},{"id":"b","label":"B"}`), CodeUnknownAsset, SeverityError, bp + "/diagram/nodes/0/icon"},
		{"points on flow", node("flow", `{"id":"a","label":"A","points":["x"]},{"id":"b","label":"B"}`), CodeInvalidValue, SeverityWarning, bp + "/diagram/nodes/0/points"},
		{"four points", node("compare", `{"id":"a","label":"A","points":["1","2","3","4"]},{"id":"b","label":"B"}`), CodeLimitExceeded, SeverityError, bp + "/diagram/nodes/0/points"},
		{"node cue", node("flow", `{"id":"a","label":"A","cue":9},{"id":"b","label":"B"}`), CodeUnknownCue, SeverityError, bp + "/diagram/nodes/0/cue"},
		{"draw on a chart", `{"do":"show","chart":"bar","data":{"labels":["a"],"values":[1]},"at":"full","entry":"draw"}`, CodeInvalidValue, SeverityWarning, bp + "/entry"},
		{"draw on a text", `{"do":"show","text":"Hi","at":"center","entry":"draw"}`, CodeInvalidValue, SeverityWarning, bp + "/entry"},
		{"entry value", `{"do":"show","text":"Hi","at":"center","entry":"spin"}`, CodeInvalidValue, SeverityError, bp + "/entry"},
		{"block area", `{"do":"show","icon":"icon.cpu","at":"middle","entry":"pop"}`, CodeInvalidValue, SeverityError, bp + "/at"},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			expectIssue(t, validateScenes(t, blockScene(tc.beat)), tc.code, tc.severity, tc.path)
		})
	}
	// The old notes are gone: icons and charts play.
	for _, beat := range []string{valid[0], valid[4], valid[7]} {
		for _, is := range validateScenes(t, blockScene(beat)) {
			if is.Code == CodeNotSupportedYet {
				t.Errorf("%s: %+v", beat, is)
			}
		}
	}
}
```

In `internal/storymaker/validate_test.go`, replace:

```go
		// The phase 1 support matrix.
		{"transition not played yet", `{"id":"s1","transition":"slide-right"}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/transition"},
		{"transition zoom", `{"id":"s1","transition":"zoom"}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/transition"},
		{"chart beat", `{"id":"s1","beats":[{"do":"chart","chart":"bar","data":{"labels":["a"],"values":[1]},"at":"right"}]}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/beats/0"},
		{"show with icon", `{"id":"s1","beats":[{"do":"show","icon":"warning","at":"center","entry":"pop"}]}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/beats/0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expectIssue(t, validateScenes(t, tc.scenes), tc.code, tc.severity, tc.path)
		})
	}
	// Supported names raise nothing.
	clean := `{"id":"s1","transition":"wipe","actors":[{"cast":"anna","pose":"typing","expression":"happy"}],"beats":[` +
		`{"do":"gesture","who":"anna","gesture":"point","count":2,"ease":"back-out"},{"do":"show","text":"Hi","style":"title","at":"center","entry":"fade"}]}`
```

with:

```go
		// The phase 1 support matrix.
		{"transition not played yet", `{"id":"s1","transition":"slide-right"}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/transition"},
		{"transition zoom", `{"id":"s1","transition":"zoom"}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/transition"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expectIssue(t, validateScenes(t, tc.scenes), tc.code, tc.severity, tc.path)
		})
	}
	// The chart beat plays now; an unknown icon is an error.
	expectIssue(t, validateScenes(t, `{"id":"s1","beats":[{"do":"show","icon":"warning","at":"center","entry":"pop"}]}`), CodeUnknownAsset, SeverityError, "/scenes/0/beats/0/icon")
	if issues := validateScenes(t, `{"id":"s1","beats":[{"do":"chart","chart":"bar","data":{"labels":["a"],"values":[1]},"at":"right"}]}`); len(issues) != 0 {
		t.Errorf("chart beat: %s", dump(issues))
	}
	// Supported names raise nothing.
	clean := `{"id":"s1","transition":"wipe","actors":[{"cast":"anna","pose":"typing","expression":"happy"}],"beats":[` +
		`{"do":"gesture","who":"anna","gesture":"point","count":2,"ease":"back-out"},{"do":"show","text":"Hi","style":"title","at":"center","entry":"fade"}]}`
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run '(?i)(TestValidateBlocks|TestValidate)' -count=1`
Expected: TestValidateBlocks subtests fail (no icon, headline, list, chart or diagram rules; not_supported_yet reported)

- [ ] **Step 3: Implement**

In `internal/storymaker/model.go`, replace:

```go
// Phase 1 plays these beats (show only with item or text) and transitions; everything else
// is accepted, ignored by the player and reported as a minor not_supported_yet issue.
var (
	phase1Beats       = nameSet("sfx", "say", "enter", "exit", "walk", "move", "pose", "gesture", "emote", "look", "show", "hide", "highlight", "camera", "wait")
	phase1Transitions = nameSet("cut", "fade", "slide-left", "wipe")
)
```

with:

```go
// Phase 1 plays these beats (show only with item or text) and transitions; everything else
// is accepted, ignored by the player and reported as a minor not_supported_yet issue.
var (
	phase1Beats       = nameSet("sfx", "say", "enter", "exit", "walk", "move", "pose", "gesture", "emote", "look", "show", "hide", "highlight", "camera", "chart", "wait")
	phase1Transitions = nameSet("cut", "fade", "slide-left", "wipe")
)
```

In `internal/storymaker/validate.go`, replace:

```go
			v.errorf(CodeInvalidValue, bb+"/amount", "The camera amount must not be negative.", `Use "amount": 1.2.`)
		}
	case "chart":
		switch b.Chart {
		case "bar", "line", "pie", "counter":
		case "":
			v.missing(bb, b, "chart", " (bar, line, pie or counter)")
		default:
			v.errorf(CodeInvalidValue, bb+"/chart", fmt.Sprintf("A chart is bar, line, pie or counter, not %q.", clip(b.Chart, 20)), `Use "chart": "bar".`)
		}
		v.chartData(bb, b)
		needPos("at", b.At)
	case "sfx":
		if b.Sound == "" {
			v.missing(bb, b, "sound", " (an sfx id)")
```

with:

```go
			v.errorf(CodeInvalidValue, bb+"/amount", "The camera amount must not be negative.", `Use "amount": 1.2.`)
		}
	case "chart":
		v.chartBlock(bb, b)
		if b.At.IsZero() {
			v.missing(bb, b, "at", " (an area name or {x, y})")
		} else {
			v.blockAt(bb+"/at", b.At, env)
		}
	case "sfx":
		if b.Sound == "" {
			v.missing(bb, b, "sound", " (an sfx id)")
```

In `internal/storymaker/validate.go`, replace:

```go
}

func (v *validator) show(bb string, b *Beat, refs *sceneRefs, explicit map[string]bool, env string) {
	kinds := 0
	for _, set := range []bool{b.Item != "", b.Text != "", b.Icon != "", b.Chart != ""} {
		if set {
			kinds++
		}
	}
	switch {
	case kinds == 0:
		v.errorf(CodeMissingField, bb+"/item", `A "show" beat needs one of "item", "text", "icon" or "chart".`, "Add \"item\". Example: "+beatExamples["show"])
	case kinds > 1:
		v.errorf(CodeInvalidValue, bb+"/item", `A "show" beat shows one thing: use only one of "item", "text", "icon" or "chart".`, "Split it into two show beats.")
	}
	if b.Item != "" {
		v.checkAsset(bb+"/item", "item", b.Item, roleProp)
	}
	if b.Text != "" && b.Style != "" {
		v.bareName(bb+"/style", &b.Style, specTextStyle)
	}
	if b.Icon != "" || b.Chart != "" {
		v.notYet(bb, "A show beat with an icon or a chart is accepted but not shown yet.")
	}
	if b.At.IsZero() {
		v.missing(bb, b, "at", " (a spot name or {x, y})")
	} else {
		v.position(bb+"/at", b.At, env)
	}
	switch b.Entry {
	case "pop", "fade", "slide", "none":
	case "":
		v.missing(bb, b, "entry", " (pop, fade, slide or none)")
	default:
		v.errorf(CodeInvalidValue, bb+"/entry", fmt.Sprintf("An entry is pop, fade, slide or none, not %q.", clip(b.Entry, 20)), `Use "entry": "pop".`)
	}
	if b.ID != "" {
		switch {
```

with:

```go
}

func (v *validator) show(bb string, b *Beat, refs *sceneRefs, explicit map[string]bool, env string) {
	switch kinds := showKinds(b); {
	case kinds == 0:
		v.errorf(CodeMissingField, bb+"/item", `A "show" beat needs one of "item", "text", "icon", "chart" or "diagram".`, "Add \"item\". Example: "+beatExamples["show"])
	case kinds > 1:
		v.errorf(CodeInvalidValue, bb+"/item", `A "show" beat shows one thing: use only one of "item", "text", "icon", "chart" or "diagram".`, "Split it into two show beats.")
	}
	kind := blockKind(b)
	if b.Item != "" {
		v.checkAsset(bb+"/item", "item", b.Item, roleProp)
	}
	if kind != "" {
		v.block(bb, b, kind, refs)
	} else if b.Text != "" && b.Style != "" && b.Icon == "" {
		v.bareName(bb+"/style", &b.Style, specTextStyle)
	}
	switch {
	case b.At.IsZero():
		v.missing(bb, b, "at", " (a spot name or {x, y})")
	case kind != "":
		v.blockAt(bb+"/at", b.At, env)
	default:
		v.position(bb+"/at", b.At, env)
	}
	switch b.Entry {
	case "pop", "fade", "slide", "none":
	case "draw":
		if kind != "icon" && kind != "diagram" {
			v.warnf(CodeInvalidValue, bb+"/entry", "draw applies to icons and diagrams; the player fades it in.", `Use "entry": "fade".`)
		}
	case "":
		v.missing(bb, b, "entry", " (pop, fade, slide, none or draw)")
	default:
		v.errorf(CodeInvalidValue, bb+"/entry", fmt.Sprintf("An entry is pop, fade, slide, none or draw, not %q.", clip(b.Entry, 20)), `Use "entry": "pop".`)
	}
	if b.ID != "" {
		switch {
```

In `internal/storymaker/validate.go`, replace:

```go
	}
}

func (v *validator) chartData(bb string, b *Beat) {
	d := b.Data
	switch {
	case d == nil:
		v.missing(bb, b, "data", " ({labels, values})")
	case len(d.Labels) == 0 || len(d.Labels) != len(d.Values):
		v.errorf(CodeInvalidValue, bb+"/data", "Chart data needs the same number (at least one) of labels and values.", `Use "data": {"labels":["Mo","Di"],"values":[2,5]}.`)
	case len(d.Labels) > 24:
		v.errorf(CodeLimitExceeded, bb+"/data", fmt.Sprintf("The chart has %d entries; the limit is 24.", len(d.Labels)), "Show fewer values.")
	}
}

// checkTarget validates a beat target: a cast id, an instance id, an item id of the scene, a
// spot or "camera".
func (v *validator) checkTarget(path, target string, refs *sceneRefs) {
```

with:

```go
	}
}

// checkTarget validates a beat target: a cast id, an instance id, an item id of the scene, a
// spot or "camera".
func (v *validator) checkTarget(path, target string, refs *sceneRefs) {
```

Create `internal/storymaker/validate_blocks.go`:

```go
package storymaker

import (
	"fmt"
	"math"
	"strings"
)

// Block limits repeat ui/js/storymaker/blocks.js (LAYOUTS, CHARTS, ICON_STYLES, LIST_MAX, POINTS_MAX); change both together.
var (
	diagramLayouts = map[string][2]int{"flow": {2, 5}, "cycle": {3, 6}, "hub": {3, 7}, "stack": {2, 5}, "compare": {2, 2}}
	chartRanges    = map[string][2]int{"bar": {1, 8}, "line": {2, 24}, "pie": {2, 6}, "counter": {1, 1}}
	iconStyles     = []string{"line", "badge", "card"}
)

const (
	blockListMax      = 4
	blockPointsMax    = 3
	blockHeadlineMax  = 80
	blockListLineMax  = 40
	blockChartLabel   = 24
	blockChartUnit    = 8
	blockChartTitle   = 40
	blockNodeLabelMax = 40
)

// blockKind says which block a show beat makes: icon, diagram, chart, headline or list, or "" for an item or a plain text.
func blockKind(b *Beat) string {
	switch {
	case b.Item != "":
		return ""
	case b.Diagram != nil:
		return "diagram"
	case b.Icon != "":
		return "icon"
	case b.Chart != "":
		return "chart"
	case b.Text != "" && (b.Style == "headline" || b.Style == "list"):
		return b.Style
	}
	return ""
}

// showKinds counts what a show beat shows. A text next to an icon is the card label and does not count.
func showKinds(b *Beat) int {
	n := 0
	for _, set := range []bool{b.Item != "", b.Text != "" && b.Icon == "", b.Icon != "", b.Chart != "", b.Diagram != nil} {
		if set {
			n++
		}
	}
	return n
}

// block validates the fields of a block show beat.
func (v *validator) block(bb string, b *Beat, kind string, refs *sceneRefs) {
	switch kind {
	case "icon":
		v.iconBlock(bb, b)
	case "diagram":
		v.diagramBlock(bb, b, refs)
	case "chart":
		v.chartBlock(bb, b)
	case "headline":
		if n := runeLen(b.Text); n > blockHeadlineMax {
			v.errorf(CodeLimitExceeded, bb+"/text", fmt.Sprintf("The headline has %d characters; the limit is %d.", n, blockHeadlineMax), "Shorten the headline.")
		}
		if n := strings.Count(b.Text, "*"); n != 0 && n != 2 {
			v.warnf(CodeInvalidValue, bb+"/text", "A headline marks one word with a pair of * signs, for example Small models are *fast*.", "Use exactly two * around one word, or none.")
		}
	case "list":
		lines := 0
		for _, line := range strings.Split(b.Text, "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			lines++
			if runeLen(line) > blockListLineMax {
				v.warnf(CodeInvalidValue, bb+"/text", fmt.Sprintf("A list line has %d characters; more than %d does not fit.", runeLen(line), blockListLineMax), "Shorten the line.")
			}
		}
		if lines > blockListMax {
			v.errorf(CodeLimitExceeded, bb+"/text", fmt.Sprintf("The list has %d lines; the limit is %d.", lines, blockListMax), "Keep at most 4 lines, one per line break.")
		}
	}
}

func (v *validator) iconBlock(bb string, b *Beat) {
	v.checkIcon(bb+"/icon", b.Icon)
	if b.Style != "" && !containsName(iconStyles, b.Style) {
		v.errorf(CodeInvalidValue, bb+"/style", fmt.Sprintf("An icon style is line, badge or card, not %q.", clip(b.Style, 20)), `Use "style": "card".`)
	}
	if b.Text != "" && b.Style != "card" {
		v.warnf(CodeInvalidValue, bb+"/text", "Only the card style of an icon shows a label.", `Use "style": "card", or remove "text".`)
	}
}

// checkIcon reports an icon id the catalog does not know.
func (v *validator) checkIcon(path, id string) {
	if v.cat == nil {
		return
	}
	if _, ok := v.cat.Icon(id); ok {
		return
	}
	fix := "Search the catalog with kind icon."
	if alts := closestNames(id, v.cat.IconIDs(), 5); len(alts) > 0 {
		fix = "Use one of: " + listOf(alts) + ". Example: " + exampleJSON("icon", alts[0])
	}
	v.errorf(CodeUnknownAsset, path, fmt.Sprintf("The icon %q is not in the catalog.", clip(id, 60)), fix)
}

// chartBlock validates the chart type and data of a show beat or of the legacy chart beat.
func (v *validator) chartBlock(bb string, b *Beat) {
	rng, known := chartRanges[b.Chart]
	switch {
	case b.Chart == "":
		v.missing(bb, b, "chart", " (bar, line, pie or counter)")
	case !known:
		v.errorf(CodeInvalidValue, bb+"/chart", fmt.Sprintf("A chart is bar, line, pie or counter, not %q.", clip(b.Chart, 20)), `Use "chart": "bar".`)
	}
	d := b.Data
	if d == nil {
		v.missing(bb, b, "data", " ({labels, values})")
		return
	}
	n := len(d.Labels)
	switch {
	case n == 0 || n != len(d.Values):
		v.errorf(CodeInvalidValue, bb+"/data", "Chart data needs the same number (at least one) of labels and values.", `Use "data": {"labels":["Mo","Di"],"values":[2,5]}.`)
		return
	case known && (n < rng[0] || n > rng[1]):
		v.errorf(CodeLimitExceeded, bb+"/data", fmt.Sprintf("A %s chart takes %d to %d values, not %d.", b.Chart, rng[0], rng[1], n), "Show "+fmt.Sprintf("%d to %d", rng[0], rng[1])+" values.")
	case n > 24:
		v.errorf(CodeLimitExceeded, bb+"/data", fmt.Sprintf("The chart has %d entries; the limit is 24.", n), "Show fewer values.")
	}
	if b.Chart == "pie" {
		sum, negative := 0.0, false
		for _, x := range d.Values {
			sum += x
			negative = negative || x < 0 || math.IsNaN(x)
		}
		if negative || sum <= 0 {
			v.errorf(CodeInvalidValue, bb+"/data/values", "A pie chart needs values that are not negative and add up to more than 0.", "Use positive values.")
		}
	}
	for k, l := range d.Labels {
		if runeLen(l) > blockChartLabel {
			v.warnf(CodeInvalidValue, fmt.Sprintf("%s/data/labels/%d", bb, k), fmt.Sprintf("A chart label has %d characters; more than %d does not fit.", runeLen(l), blockChartLabel), "Shorten the label.")
		}
	}
	if runeLen(d.Unit) > blockChartUnit {
		v.warnf(CodeInvalidValue, bb+"/data/unit", fmt.Sprintf("A chart unit longer than %d characters does not fit.", blockChartUnit), `Use a short unit such as "%".`)
	}
	if runeLen(d.Title) > blockChartTitle {
		v.warnf(CodeInvalidValue, bb+"/data/title", fmt.Sprintf("A chart title longer than %d characters does not fit.", blockChartTitle), "Shorten the title.")
	}
}

func (v *validator) diagramBlock(bb string, b *Beat, refs *sceneRefs) {
	d := b.Diagram
	rng, known := diagramLayouts[d.Layout]
	if !known {
		v.errorf(CodeInvalidValue, bb+"/diagram/layout", fmt.Sprintf("A diagram layout is flow, cycle, hub, stack or compare, not %q.", clip(d.Layout, 20)), `Use "layout": "flow".`)
	} else if n := len(d.Nodes); n < rng[0] || n > rng[1] {
		v.errorf(CodeLimitExceeded, bb+"/diagram/nodes", fmt.Sprintf("A %s diagram takes %d to %d nodes, not %d.", d.Layout, rng[0], rng[1], n), fmt.Sprintf("Use %d to %d nodes.", rng[0], rng[1]))
	}
	seen := map[string]bool{}
	for k := range d.Nodes {
		n := &d.Nodes[k]
		np := fmt.Sprintf("%s/diagram/nodes/%d", bb, k)
		switch {
		case !validSlug(n.ID):
			v.errorf(CodeInvalidValue, np+"/id", fmt.Sprintf("The node id %q is not a slug.", clip(n.ID, 40)), "Use lowercase letters, digits and dashes, for example srv.")
		case seen[n.ID]:
			v.errorf(CodeDuplicateID, np+"/id", fmt.Sprintf("The node id %q is used twice in this diagram.", n.ID), "Give each node its own id.")
		default:
			seen[n.ID] = true
		}
		switch l := runeLen(n.Label); {
		case strings.TrimSpace(n.Label) == "":
			v.errorf(CodeMissingField, np+"/label", "A diagram node needs a label.", `Add "label": "Server".`)
		case l > blockNodeLabelMax:
			v.errorf(CodeLimitExceeded, np+"/label", fmt.Sprintf("The node label has %d characters; the limit is %d.", l, blockNodeLabelMax), "Use one or two words.")
		}
		if n.Icon != "" {
			v.checkIcon(np+"/icon", n.Icon)
		}
		switch {
		case len(n.Points) > blockPointsMax:
			v.errorf(CodeLimitExceeded, np+"/points", fmt.Sprintf("A node has %d points; the limit is %d.", len(n.Points), blockPointsMax), "Keep at most 3 points.")
		case len(n.Points) > 0 && d.Layout != "compare":
			v.warnf(CodeInvalidValue, np+"/points", "Only the compare layout shows points.", `Remove "points" or use "layout": "compare".`)
		}
		if n.Cue != 0 {
			switch {
			case n.Cue < 0:
				v.errorf(CodeInvalidValue, np+"/cue", "A cue is a narration marker number, 1 or more.", `Use "cue": 1 and put {1} into the narration.`)
			case !refs.markers[n.Cue]:
				v.errorf(CodeUnknownCue, np+"/cue", fmt.Sprintf("The narration has no marker {%d}.", n.Cue), v.cueFix(refs, n.Cue))
			}
		}
	}
}

// blockAt validates the position of a block: a point or a name of BlockAreas. Blocks skip the environment spot check.
func (v *validator) blockAt(path string, p *Position, env string) {
	if p != nil && !p.Point && p.Spot != "" && len(p.Invalid) == 0 {
		if !containsName(BlockAreas, p.Spot) {
			v.errorf(CodeInvalidValue, path, fmt.Sprintf("A block is shown at %s, not %q.", listOf(BlockAreas), clip(p.Spot, 40)), `Use "at": "full".`)
		}
		return
	}
	v.position(path, p, env)
}
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker -run '(?i)(TestValidateBlocks|TestValidate)' -count=1`
Expected: PASS, whole internal/storymaker package ok. phase1Beats gains "chart" here (moved from D1 to keep D1 green).

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/model.go internal/storymaker/validate.go internal/storymaker/validate_blocks.go internal/storymaker/validate_blocks_test.go internal/storymaker/validate_test.go
git commit -m "storymaker: Go validates block shows"
```

### Task D3: highlight on a diagram node

**Files:**
- Modify: `internal/storymaker/validate.go`
- Modify: `internal/storymaker/validate_blocks_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/validate_blocks_test.go`, replace:

```go
package storymaker

import "testing"

func blockScene(beat string) string {
	return `{"id":"s1","narration":"Anfrage {1} Server {2}","beats":[` + beat + `]}`
```

with:

```go
package storymaker

import (
	"strings"
	"testing"
)

func blockScene(beat string) string {
	return `{"id":"s1","narration":"Anfrage {1} Server {2}","beats":[` + beat + `]}`
```

In `internal/storymaker/validate_blocks_test.go`, replace:

```go
		}
	}
}
```

with:

```go
		}
	}
}

func TestValidateNodeHighlight(t *testing.T) {
	diagram := `{"do":"show","diagram":{"layout":"flow","nodes":[{"id":"req","label":"A"},{"id":"srv","label":"B"}]},"at":"full","entry":"fade"}`
	scene := func(target string) string {
		return `{"id":"s1","props":[{"item":"prop.lock"}],"beats":[` + diagram + `,{"do":"highlight","target":"` + target + `","style":"pulse"}]}`
	}
	for _, target := range []string{"diagram-1.srv", "diagram-1", "prop.lock"} {
		if issues := validateScenes(t, scene(target)); len(issues) != 0 {
			t.Errorf("%s:%s", target, dump(issues))
		}
	}
	is := expectIssue(t, validateScenes(t, scene("diagram-1.zzz")), CodeUnknownName, SeverityError, "/scenes/0/beats/1/target")
	if !strings.Contains(is.Message, "zzz") || !strings.Contains(is.Message, "diagram-1") {
		t.Errorf("message: %s", is.Message)
	}
	expectIssue(t, validateScenes(t, scene("nodiagram.srv")), CodeUnknownName, SeverityError, "/scenes/0/beats/1/target")
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run '(?i)(TestValidateNodeHighlight)' -count=1`
Expected: diagram-1.srv reported as unknown_name (target not in this scene)

- [ ] **Step 3: Implement**

In `internal/storymaker/validate.go`, replace:

```go
	spots      map[string]Spot
	spotsKnown bool
	markers    map[int]bool
}

func (v *validator) scene(i int, sc *Scene, env string, seen map[string]int) {
```

with:

```go
	spots      map[string]Spot
	spotsKnown bool
	markers    map[int]bool
	diagrams   map[string][]string // diagram instance id -> node ids
}

func (v *validator) scene(i int, sc *Scene, env string, seen map[string]int) {
```

In `internal/storymaker/validate.go`, replace:

```go

// refsOf collects the instance ids, items, spots and cue markers of a scene.
func (v *validator) refsOf(sc *Scene, env string) *sceneRefs {
	r := &sceneRefs{instances: map[string]bool{}, items: map[string][]string{}, markers: map[int]bool{}}
	for _, p := range sc.Props {
		if p.ID != "" {
			r.instances[p.ID] = true
```

with:

```go

// refsOf collects the instance ids, items, spots and cue markers of a scene.
func (v *validator) refsOf(sc *Scene, env string) *sceneRefs {
	r := &sceneRefs{instances: map[string]bool{}, items: map[string][]string{}, markers: map[int]bool{}, diagrams: map[string][]string{}}
	for _, p := range sc.Props {
		if p.ID != "" {
			r.instances[p.ID] = true
```

In `internal/storymaker/validate.go`, replace:

```go
		if b.Item != "" {
			r.items[b.Item] = append(r.items[b.Item], id)
		}
	}
	if v.cat != nil {
		r.spots, r.spotsKnown = v.cat.Spots(env, v.s.Format.Aspect)
```

with:

```go
		if b.Item != "" {
			r.items[b.Item] = append(r.items[b.Item], id)
		}
		if b.Diagram != nil && id != "" {
			for _, n := range b.Diagram.Nodes {
				r.diagrams[id] = append(r.diagrams[id], n.ID)
			}
		}
	}
	if v.cat != nil {
		r.spots, r.spotsKnown = v.cat.Spots(env, v.s.Format.Aspect)
```

In `internal/storymaker/validate.go`, replace:

```go
	} else if v.cat == nil && validSlug(target) {
		return // spots cannot be checked without a catalog
	}
	pool := append([]string{"camera"}, v.castIDs...)
	for id := range refs.instances {
		pool = append(pool, id)
```

with:

```go
	} else if v.cat == nil && validSlug(target) {
		return // spots cannot be checked without a catalog
	}
	if i := strings.LastIndex(target, "."); i > 0 {
		if nodes, ok := refs.diagrams[target[:i]]; ok {
			if containsName(nodes, target[i+1:]) {
				return
			}
			v.errorf(CodeUnknownName, path, fmt.Sprintf("The diagram %s has no node %q.", target[:i], clip(target[i+1:], 40)),
				"Use one of its nodes: "+listOf(nodes)+". Example: "+exampleJSON("target", target[:i]+"."+nodes[0]))
			return
		}
	}
	pool := append([]string{"camera"}, v.castIDs...)
	for id := range refs.instances {
		pool = append(pool, id)
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker -run '(?i)(TestValidateNodeHighlight)' -count=1`
Expected: PASS, whole internal/storymaker package ok

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/validate.go internal/storymaker/validate_blocks_test.go
git commit -m "storymaker: highlight on a diagram node"
```

### Task D4: cue_phrase on diagram nodes

**Files:**
- Modify: `internal/storymaker/cuephrase.go`
- Modify: `internal/storymaker/cuephrase_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/cuephrase_test.go`, replace:

```go
		if c.fields["narration"] != narration || c.fields["beats"].([]any)[0].(map[string]any)["cue_phrase"] == nil {
			t.Errorf("%s: a refusal changed the fields: %v", c.name, c.fields)
		}
	}
}
```

with:

```go
		if c.fields["narration"] != narration || c.fields["beats"].([]any)[0].(map[string]any)["cue_phrase"] == nil {
			t.Errorf("%s: a refusal changed the fields: %v", c.name, c.fields)
		}
	}
}

func TestCuePhraseOnDiagramNodes(t *testing.T) {
	diagram := func(nodes ...map[string]any) map[string]any {
		list := make([]any, len(nodes))
		for i, n := range nodes {
			list[i] = n
		}
		return map[string]any{"layout": "flow", "nodes": list}
	}
	fields := cueFields("Die Anfrage geht zum Server und zurueck.",
		map[string]any{"do": "show", "diagram": diagram(
			map[string]any{"id": "req", "label": "Anfrage", "cue_phrase": "Anfrage"},
			map[string]any{"id": "srv", "label": "Server", "cue_phrase": "Server"})})
	if notes, refusal := resolveCuePhrases(fields); refusal != nil || len(notes) != 0 {
		t.Fatalf("notes %v, refusal %+v", notes, refusal)
	}
	if got, want := fields["narration"], "Die {1}Anfrage geht zum {2}Server und zurueck."; got != want {
		t.Fatalf("narration = %q, want %q", got, want)
	}
	nodes := cueNodes(fields["beats"].([]any)[0].(map[string]any))
	for k, want := range []int{1, 2} {
		if n := nodes[k].(map[string]any); n["cue"] != want || n["cue_phrase"] != nil || n["label"] == nil {
			t.Fatalf("node %d = %v", k, n)
		}
	}
	bad := cueFields("Die Anfrage geht.", map[string]any{"do": "show", "diagram": diagram(
		map[string]any{"id": "req", "label": "A", "cue_phrase": "Anfrage"},
		map[string]any{"id": "srv", "label": "S", "cue_phrase": "Nirgends"})})
	_, refusal := resolveCuePhrases(bad)
	if refusal == nil || refusal.Code != "cue_phrase_not_found" || !strings.Contains(refusal.Message, "beats[0].diagram.nodes[1]") {
		t.Fatalf("refusal = %+v", refusal)
	}
	conflict := cueFields("Die Anfrage geht.", map[string]any{"do": "show", "diagram": diagram(
		map[string]any{"id": "req", "label": "A", "cue": float64(1), "cue_phrase": "Anfrage"})})
	if _, refusal := resolveCuePhrases(conflict); refusal == nil || refusal.Code != "cue_phrase_conflict" || !strings.Contains(refusal.Message, "beats[0].diagram.nodes[0]") {
		t.Fatalf("conflict refusal = %+v", refusal)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run '(?i)(TestCuePhraseOnDiagramNodes)' -count=1`
Expected: build failed (cueNodes undefined); cue_phrase on nodes was left in place

- [ ] **Step 3: Implement**

In `internal/storymaker/cuephrase.go`, replace:

```go
// cueTrailingMarker finds a marker that ends a text.
var cueTrailingMarker = regexp.MustCompile(`\{(\d{1,3})\}$`)

// resolveCuePhrases rewrites fields in place: every beat with cue_phrase gets a {n} marker in fields["narration"] and
// cue: n, and loses cue_phrase. notes say when a phrase occurred more than once.
func resolveCuePhrases(fields map[string]any) (notes []string, refusal *cuePhraseRefusal) {
	beats, _ := fields["beats"].([]any)
	if !slices.ContainsFunc(beats, func(b any) bool { m, ok := b.(map[string]any); return ok && m["cue_phrase"] != nil }) {
		return nil, nil
	}
	for i, b := range beats {
		if m, ok := b.(map[string]any); ok && m["cue_phrase"] != nil {
			if refusal := checkCuePhraseBeat(i, m); refusal != nil {
				return nil, refusal
			}
		}
	}
	narration, _ := fields["narration"].(string)
	taken := map[int]bool{}
	for _, m := range narrationMarker.FindAllStringSubmatch(narration, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil {
			taken[n] = true
		}
	}
	for _, b := range beats {
		if m, ok := b.(map[string]any); ok {
			taken[cueNumber(m["cue"])] = true
		}
	}
	out := make([]any, len(beats))
	for i, b := range beats {
		m, ok := b.(map[string]any)
		if !ok || m["cue_phrase"] == nil {
			out[i] = b
			continue
		}
		phrase := strings.TrimSpace(m["cue_phrase"].(string))
		tokens := cueTokens(narration)
		phraseTokens := cueTokens(phrase)
		hits := cuePhraseMatches(tokens, phraseTokens)
		if len(hits) == 0 {
			return nil, cuePhraseNotFound(phrase, narration, tokens, phraseTokens)
		}
		if len(hits) > 1 {
			notes = append(notes, fmt.Sprintf("The phrase %q occurs %d times in the narration; the first one is cued.", phrase, len(hits)))
```

with:

```go
// cueTrailingMarker finds a marker that ends a text.
var cueTrailingMarker = regexp.MustCompile(`\{(\d{1,3})\}$`)

// resolveCuePhrases rewrites fields in place: every beat and every diagram node with cue_phrase gets a {n} marker in
// fields["narration"] and cue: n, and loses cue_phrase. notes say when a phrase occurred more than once.
func resolveCuePhrases(fields map[string]any) (notes []string, refusal *cuePhraseRefusal) {
	beats, _ := fields["beats"].([]any)
	if !slices.ContainsFunc(beats, func(b any) bool {
		m, ok := b.(map[string]any)
		return ok && (m["cue_phrase"] != nil || slices.ContainsFunc(cueNodes(m), func(n any) bool { nm, ok := n.(map[string]any); return ok && nm["cue_phrase"] != nil }))
	}) {
		return nil, nil
	}
	taken := map[int]bool{}
	for i, b := range beats {
		m, ok := b.(map[string]any)
		if !ok {
			continue
		}
		taken[cueNumber(m["cue"])] = true
		if m["cue_phrase"] != nil {
			if refusal := checkCuePhraseBeat(fmt.Sprintf("beats[%d]", i), m); refusal != nil {
				return nil, refusal
			}
		}
		for k, n := range cueNodes(m) {
			if nm, ok := n.(map[string]any); ok {
				taken[cueNumber(nm["cue"])] = true
				if nm["cue_phrase"] != nil {
					if refusal := checkCuePhraseBeat(fmt.Sprintf("beats[%d].diagram.nodes[%d]", i, k), nm); refusal != nil {
						return nil, refusal
					}
				}
			}
		}
	}
	narration, _ := fields["narration"].(string)
	for _, m := range narrationMarker.FindAllStringSubmatch(narration, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil {
			taken[n] = true
		}
	}
	// resolve gives m its marker: a clone without cue_phrase, or m itself when it has none.
	resolve := func(m map[string]any, where string) (map[string]any, *cuePhraseRefusal) {
		if m["cue_phrase"] == nil {
			return m, nil
		}
		phrase := strings.TrimSpace(m["cue_phrase"].(string))
		tokens := cueTokens(narration)
		phraseTokens := cueTokens(phrase)
		hits := cuePhraseMatches(tokens, phraseTokens)
		if len(hits) == 0 {
			refusal := cuePhraseNotFound(phrase, narration, tokens, phraseTokens)
			refusal.Message = where + ": " + refusal.Message
			return nil, refusal
		}
		if len(hits) > 1 {
			notes = append(notes, fmt.Sprintf("The phrase %q occurs %d times in the narration; the first one is cued.", phrase, len(hits)))
```

In `internal/storymaker/cuephrase.go`, replace:

```go
		clone := maps.Clone(m)
		delete(clone, "cue_phrase")
		clone["cue"] = n
		out[i] = clone
	}
	fields["narration"] = narration
	fields["beats"] = out
	return notes, nil
}

// cueNumber reads a decoded JSON number; anything else is 0.
func cueNumber(v any) int {
	switch n := v.(type) {
```

with:

```go
		clone := maps.Clone(m)
		delete(clone, "cue_phrase")
		clone["cue"] = n
		return clone, nil
	}
	out := make([]any, len(beats))
	for i, b := range beats {
		m, ok := b.(map[string]any)
		if !ok {
			out[i] = b
			continue
		}
		m, refusal := resolve(m, fmt.Sprintf("beats[%d]", i))
		if refusal != nil {
			return nil, refusal
		}
		if nodes := cueNodes(m); slices.ContainsFunc(nodes, func(n any) bool { nm, ok := n.(map[string]any); return ok && nm["cue_phrase"] != nil }) {
			next := make([]any, len(nodes))
			for k, n := range nodes {
				nm, ok := n.(map[string]any)
				if !ok {
					next[k] = n
					continue
				}
				if next[k], refusal = resolve(nm, fmt.Sprintf("beats[%d].diagram.nodes[%d]", i, k)); refusal != nil {
					return nil, refusal
				}
			}
			diagram := maps.Clone(m["diagram"].(map[string]any))
			diagram["nodes"] = next
			m = maps.Clone(m)
			m["diagram"] = diagram
		}
		out[i] = m
	}
	fields["narration"] = narration
	fields["beats"] = out
	return notes, nil
}

// cueNodes returns the nodes of the diagram of a decoded beat, or nil.
func cueNodes(beat map[string]any) []any {
	d, _ := beat["diagram"].(map[string]any)
	nodes, _ := d["nodes"].([]any)
	return nodes
}

// cueNumber reads a decoded JSON number; anything else is 0.
func cueNumber(v any) int {
	switch n := v.(type) {
```

In `internal/storymaker/cuephrase.go`, replace:

```go
	return 0
}

// checkCuePhraseBeat refuses a beat whose cue_phrase cannot be used: not text, empty, too long, without a word, or next
// to a cue. beat is the index of the beat in the scene.
func checkCuePhraseBeat(beat int, m map[string]any) *cuePhraseRefusal {
	where := fmt.Sprintf("beats[%d]", beat)
	phrase, isText := m["cue_phrase"].(string)
	phrase = strings.TrimSpace(phrase)
	switch {
```

with:

```go
	return 0
}

// checkCuePhraseBeat refuses a beat or diagram node whose cue_phrase cannot be used: not text, empty, too long, without a
// word, or next to a cue. where names it, for example beats[0] or beats[0].diagram.nodes[1].
func checkCuePhraseBeat(where string, m map[string]any) *cuePhraseRefusal {
	phrase, isText := m["cue_phrase"].(string)
	phrase = strings.TrimSpace(phrase)
	switch {
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker -run '(?i)(TestCuePhraseOnDiagramNodes)' -count=1`
Expected: PASS, whole internal/storymaker package ok. A not-found refusal now names its beat or node (beats[0].diagram.nodes[1]: ...).

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/cuephrase.go internal/storymaker/cuephrase_test.go
git commit -m "storymaker: cue_phrase on diagram nodes"
```

### Task D5: beats play icons, charts, diagrams, headlines and lists

**Files:**
- Modify: `scripts/storymaker-bundle-parts.js`
- Modify: `scripts/storymaker-tests/04-compile.mjs`
- Create: `scripts/storymaker-tests/34-block-beats.mjs`
- Create: `ui/js/storymaker/beats-blocks.js`
- Modify: `ui/js/storymaker/beats.js`
- Modify: `ui/js/storymaker/compile.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/04-compile.mjs`, replace:

```js
    const codes = (path, code) => issues.filter(i => i.path === path && i.code === code);
    assert.equal(codes('/scenes/0/beats/0', 'not_supported_yet').length, 0, 'highlight is played');
    assert.equal(codes('/scenes/0/beats/2', 'not_supported_yet').length, 0, 'the sfx beat plays');
    assert.equal(codes('/scenes/0/beats/3', 'not_supported_yet').length, 1, 'the chart beat');
    assert.equal(codes('/scenes/0/beats/1/text', 'narration_missing').length, 1, 'a say without audio is estimated');
    assert.equal(codes('/scenes/0/beats/4', 'not_supported_yet').length, 1, 'show with an icon');
    assert.equal(codes('/scenes/0/beats/5', 'not_supported_yet').length, 1, 'show with a chart');
    assert.ok(issues.every(i => i.code !== 'not_supported_yet' || i.severity === 'minor'));
    assert.equal(codes('/scenes/0/actors/0/pose', 'not_supported_yet').length, 1, 'a known pose that the catalog lacks');
    assert.equal(codes('/scenes/0/actors/0/expression', 'not_supported_yet').length, 1);
```

with:

```js
    const codes = (path, code) => issues.filter(i => i.path === path && i.code === code);
    assert.equal(codes('/scenes/0/beats/0', 'not_supported_yet').length, 0, 'highlight is played');
    assert.equal(codes('/scenes/0/beats/2', 'not_supported_yet').length, 0, 'the sfx beat plays');
    assert.equal(codes('/scenes/0/beats/3', 'not_supported_yet').length, 0, 'the chart beat plays');
    assert.equal(codes('/scenes/0/beats/3/data', 'invalid_value').length, 1, 'but its data is empty');
    assert.equal(codes('/scenes/0/beats/1/text', 'narration_missing').length, 1, 'a say without audio is estimated');
    assert.equal(codes('/scenes/0/beats/4', 'not_supported_yet').length, 0, 'show with an icon plays');
    assert.equal(codes('/scenes/0/beats/4/icon', 'unknown_asset').length, 1, 'but icon.x is unknown');
    assert.equal(codes('/scenes/0/beats/5', 'not_supported_yet').length, 0, 'show with a chart plays');
    assert.equal(codes('/scenes/0/beats/5/data', 'missing_field').length, 1, 'but it has no data');
    assert.ok(issues.every(i => i.code !== 'not_supported_yet' || i.severity === 'minor'));
    assert.equal(codes('/scenes/0/actors/0/pose', 'not_supported_yet').length, 1, 'a known pose that the catalog lacks');
    assert.equal(codes('/scenes/0/actors/0/expression', 'not_supported_yet').length, 1);
```

Create `scripts/storymaker-tests/34-block-beats.mjs`:

```js
// Batch D5: storyboard beats that show icons, diagrams, charts, headlines and lists, and the highlight of a diagram node.
import assert from 'node:assert/strict';
import { plain } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, catalog } = t;
const SM = t.core();

const flow = { layout: 'flow', flow: true, nodes: [{ id: 'req', label: 'Anfrage', icon: 'icon.send', cue: 1 }, { id: 'srv', label: 'Server', icon: 'icon.server', cue: 2 }] };
const BEATS = [
    { do: 'show', icon: 'icon.cpu', style: 'card', text: 'CPU', at: 'board', entry: 'draw' },
    { do: 'show', diagram: flow, at: 'full', entry: 'draw' },
    { do: 'show', chart: 'bar', data: { labels: ['Warten', 'Rechnen'], values: [90, 10], unit: '%', title: 'Zeit' }, at: 'full', entry: 'fade' },
    { do: 'show', text: 'Kleine Modelle sind *schnell*', style: 'headline', at: 'top', entry: 'fade' },
    { do: 'show', text: 'Laden\nRechnen\nAntworten', style: 'list', at: 'right', entry: 'fade' }
];
const compileScene = (beats, extra = {}) => SM.compile({ schema: 'storymaker/1', cast: [], scenes: [{ id: 'a', narration: 'Die Anfrage {1} geht zum Server {2} und zurueck.', ...extra, beats }] }, {}, catalog);
const blocksOf = out => out.timeline.scenes[0].entities.filter(e => e.type === 'block');
const issuesOf = (out, code) => plain(out.report.issues).filter(i => !code || i.code === code);

check('block beats: every show kind makes one block with its kind and area', () => {
    const out = compileScene(BEATS), blocks = blocksOf(out);
    assert.deepEqual(plain(blocks.map(b => b.block.kind)), ['icon', 'diagram', 'chart', 'headline', 'list']);
    assert.deepEqual(plain(blocks.map(b => b.id)), ['icon-cpu-1', 'diagram-1', 'chart-1', 'text-1', 'text-2']);
    const stage = SM.stage.create({ aspect: '16:9', env: null });
    ['board', 'full', 'full', 'top', 'right'].forEach((at, i) => {
        const a = SM.blocks.area(stage, null, 'day', at);
        near(blocks[i].w, a.w, 1e-6); near(blocks[i].h, a.h, 1e-6);
    });
    assert.deepEqual(issuesOf(out).filter(i => i.severity !== 'minor' && i.code !== 'narration_missing'), []);
    assert.ok(blocks.every(b => b.alpha.at(60) === 1), 'every block is visible at the end');
    assert.ok(blocks.every(b => b.alpha.init === 0), 'and hidden before its beat');
});

check('block beats: the node cues become the reveal times, without cues the player spreads the nodes', () => {
    const out = compileScene([BEATS[1]]), e = blocksOf(out)[0], cues = out.timeline.scenes[0].narration.cueList;
    assert.equal(e.reveals.length, 2);
    assert.ok(e.reveals[1] > e.reveals[0]);
    assert.equal(cues.length, 2);
    cues.forEach((c, k) => near(e.reveals[k], out.timeline.scenes[0].narration.start + c.at_ms / 1000, 1e-3, 'node ' + k + ' appears on its marker'));
    const plainFlow = { layout: 'flow', nodes: [{ id: 'a', label: 'A' }, { id: 'b', label: 'B' }] };
    const spread = blocksOf(compileScene([{ do: 'show', diagram: plainFlow, at: 'full', entry: 'draw' }]))[0];
    assert.equal(spread.reveals, null);
    near(spread.dur, SM.blocks.revealSeconds(spread.block), 1e-9);
    const none = blocksOf(compileScene([{ do: 'show', diagram: plainFlow, at: 'full', entry: 'none' }]))[0];
    assert.equal(none.dur, 0);
});

check('block beats: highlight with diagram.node marks the node, the legacy chart beat plays as a fade', () => {
    const out = compileScene([BEATS[1], { do: 'highlight', target: 'diagram-1.srv', style: 'pulse' },
        { do: 'chart', chart: 'bar', data: { labels: ['a'], values: [1] }, at: 'right' }, { do: 'highlight', target: 'diagram-1.zzz' }]);
    const [d, c] = blocksOf(out);
    assert.equal(d.marks.list.length, 1);
    assert.equal(d.marks.list[0].data.node, 'srv');
    assert.equal(c.block.kind, 'chart');
    assert.equal(c.entry, 'fade');
    const bad = issuesOf(out, 'unknown_name');
    assert.equal(bad.length, 1);
    assert.equal(bad[0].path, '/scenes/0/beats/3/target');
});

check('block beats: the needs hold the block fonts and a broken block is reported and skipped', () => {
    const fonts = [...catalog.fonts, { name: 'default', family: 'Storymaker Default', weight: 700, file: '/fonts/placeholder-bold.woff2', licence: 'OFL-1.1' }];
    const out = SM.compile({ schema: 'storymaker/1', cast: [], scenes: [{ id: 'a', beats: BEATS }] }, {}, { ...catalog, fonts });
    assert.ok(plain(out.needs).includes('font:default/700'), 'the 700 face');
    const broken = compileScene([{ do: 'show', icon: 'icon.nope', at: 'full', entry: 'draw' }, { do: 'show', chart: 'bar', at: 'full', entry: 'fade' },
        { do: 'chart', chart: 'bar', data: { labels: [], values: [] }, at: 'right' }, { do: 'show', icon: 'icon.cpu', at: 'middle', entry: 'pop' }]);
    assert.equal(blocksOf(broken).length, 1, 'only the icon at an unknown area is kept, at full');
    const codes = issuesOf(broken).filter(i => i.path.includes('/beats/')).map(i => i.path.replace('/scenes/0/beats/', '') + ' ' + i.code);
    assert.deepEqual(codes, ['0/icon unknown_asset', '1/data missing_field', '2/data invalid_value', '3/at unknown_spot']);
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `npm run test:storymaker`
Expected: not_supported_yet for chart and icon beats; block beats made no entity (34-block-beats.mjs, 04-compile.mjs)

- [ ] **Step 3: Implement**

In `scripts/storymaker-bundle-parts.js`, replace:

```js
  'ui/js/storymaker/blocks.js',
  'ui/js/storymaker/blocks-draw.js',
  'ui/js/storymaker/blocks-plans.js',
  'ui/js/storymaker/compose.js',
  'ui/js/storymaker/beats.js',
  'ui/js/storymaker/lint.js',
```

with:

```js
  'ui/js/storymaker/blocks.js',
  'ui/js/storymaker/blocks-draw.js',
  'ui/js/storymaker/blocks-plans.js',
  'ui/js/storymaker/beats-blocks.js',
  'ui/js/storymaker/compose.js',
  'ui/js/storymaker/beats.js',
  'ui/js/storymaker/lint.js',
```

Create `ui/js/storymaker/beats-blocks.js`:

```js
// Storymaker player: block beats. A show beat with an icon, a diagram, a chart, a headline or a list (and the legacy chart
// beat) makes one block entity (blocks.js). This file builds the definition from the beat, checks it, resolves the area and
// gives the diagram nodes their reveal times; beats.js schedules the beat like any other show. Classic script; publishes
// StorymakerCore.beatBlocks.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    function isText(b) { return b.text !== undefined && b.text !== null; }

    // isBlockBeat(b): true for a beat that shows a block. An item always wins, as in the validator.
    function isBlockBeat(b) {
        if (!b || typeof b !== 'object') return false;
        if (b.do === 'chart') return true;
        if (b.do !== 'show' || b.item) return false;
        return !!(b.icon || b.chart || b.diagram || (isText(b) && (b.style === 'headline' || b.style === 'list')));
    }

    function kindOf(b) {
        if (b.diagram) return 'diagram';
        if (b.icon) return 'icon';
        if (b.chart) return 'chart';
        return b.style === 'list' ? 'list' : 'headline';
    }

    // slugOf(b): the base of the instance id, as Go's showSlug names it (icon.cpu becomes icon-cpu).
    function slugOf(b, kind) {
        if (kind === 'icon') return String(b.icon).toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 40) || 'icon';
        return kind === 'headline' || kind === 'list' ? 'text.text' : kind;
    }

    // defOf(b, kind, id): the block definition of a beat, without area and timing.
    function defOf(b, kind, id) {
        var def = { id: id, kind: kind };
        if (kind === 'icon') {
            def.icon = b.icon;
            if (b.style) def.style = b.style;
            if (isText(b) && b.text !== '') def.label = b.text;
        } else if (kind === 'diagram') {
            var d = b.diagram && typeof b.diagram === 'object' ? b.diagram : {};
            def.diagram = { layout: d.layout, nodes: (Array.isArray(d.nodes) ? d.nodes : []).map(function (n) {
                var out = n && typeof n === 'object' ? { id: n.id, label: n.label } : n;
                if (out && n.icon) out.icon = n.icon;
                if (out && n.points !== undefined) out.points = n.points;
                return out;
            }) };
            if (d.flow === true) def.diagram.flow = true;
        } else if (kind === 'chart') {
            var data = b.data || {};
            def.chart = { type: b.chart, labels: data.labels, values: data.values };
            if (data.unit !== undefined) def.chart.unit = data.unit;
            if (data.title !== undefined) def.chart.title = data.title;
        } else def.text = b.text;
        return def;
    }

    // issueOf(problem): the issue code and the beat field of a failed SM.blocks.check.
    function issueOf(p) {
        var head = String(p.field).split(/[.\[]/)[0], last = String(p.field).split('.').pop();
        if (head === 'icon' || last === 'icon') return { code: 'unknown_asset', field: head === 'diagram' ? 'diagram' : 'icon' };
        return { code: 'invalid_value', field: head === 'chart' ? 'data' : head };
    }

    // target(c, b, j, h): the block entity of a beat, hidden and registered, or null (with a warning) when the beat cannot be
    // shown. h = {warn, beatPath, instanceId, register}. The timing is set by reveals() when the beat is scheduled.
    function target(c, b, j, h) {
        var kind = b.do === 'chart' ? 'chart' : kindOf(b);
        if (kind === 'chart' && (b.data === undefined || b.data === null)) {
            h.warn('warning', h.beatPath(j, 'data'), 'missing_field', 'A chart needs data {labels, values}.');
            return null;
        }
        var probe = defOf(b, kind, 'block'), bad = SM.blocks.check(probe, c.cat);
        if (bad) {
            var is = issueOf(bad);
            h.warn('warning', h.beatPath(j, is.field), is.code, bad.message + ' The beat is ignored.');
            return null;
        }
        var at = b.at === undefined || b.at === null ? 'full' : b.at, area = SM.blocks.area(c.stage, c.env, c.variant, at);
        if (!area) {
            h.warn('warning', h.beatPath(j, 'at'), 'unknown_spot', 'Unknown block area ' + JSON.stringify(at) + '; the player uses "full".');
            area = SM.blocks.area(c.stage, c.env, c.variant, 'full');
        }
        var def = defOf(b, kind, h.instanceId(slugOf(b, kind), b.id));
        def.area = area;
        def.hidden = true;
        def.entry = b.entry || (kind === 'icon' || kind === 'diagram' ? 'draw' : 'fade');
        def.t0 = 0;
        def.dur = 0;
        var e = SM.blocks.create(def);
        h.register(e, null);
        return e;
    }

    // reveals(c, b, def, t0, dur): the reveal times of the diagram nodes, scene-local seconds, or null when the player spreads
    // the items over dur by itself. A node with a cue appears on its marker.
    function reveals(c, b, def, t0, dur) {
        if (def.kind !== 'diagram' || !b.diagram || !Array.isArray(b.diagram.nodes)) return null;
        var nodes = b.diagram.nodes, cues = c.plan && c.plan.narration && c.plan.narration.cues || {}, any = false, out = [];
        for (var k = 0; k < nodes.length; k++) {
            var t = nodes[k] && nodes[k].cue !== undefined && nodes[k].cue !== null ? cues[nodes[k].cue] : undefined;
            if (typeof t === 'number' && isFinite(t)) any = true;
            else t = t0 + k * dur / Math.max(1, nodes.length);
            out.push(t);
        }
        return any ? out : null;
    }

    // highlightNode(c, b, j, t0, dur, find): handles a highlight whose target is "<block id>.<node id>". It returns false when
    // the target is no such reference, so the caller reports the unknown name.
    function highlightNode(c, b, j, t0, dur, find) {
        var target = String(b.target || ''), cut = target.lastIndexOf('.');
        if (cut <= 0) return false;
        var e = find(target.slice(0, cut));
        if (!e || e.type !== 'block' || e.block.kind !== 'diagram') return false;
        var id = target.slice(cut + 1);
        var known = e.block.diagram.nodes.some(function (n) { return n.id === id; });
        if (!known) {
            c.issues.push(SM.plan.issue('warning', c.scene.id, '/scenes/' + c.index + '/beats/' + j + '/target', 'unknown_name', 'The diagram ' + e.id + ' has no node "' + id + '".'));
            return true;
        }
        e.marks.add(t0, t0 + dur, { node: id, style: b.style === 'outline' || b.style === 'arrow' ? b.style : 'pulse' });
        return true;
    }

    // texts(e): every text a block draws, for the glyph needs of the film.
    function texts(e) {
        var b = e.block, out = [];
        if (b.label) out.push(b.label);
        if (b.text) out.push(b.text.replace(/\*/g, ''));
        if (b.diagram) b.diagram.nodes.forEach(function (n) { out.push(n.label); (n.points || []).forEach(function (p) { out.push(p); }); });
        if (b.chart) { out.push(b.chart.unit || '', b.chart.title || ''); b.chart.labels.forEach(function (l) { out.push(l); }); }
        return out;
    }

    SM.beatBlocks = { isBlockBeat: isBlockBeat, target: target, reveals: reveals, highlightNode: highlightNode, texts: texts };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

In `ui/js/storymaker/beats.js`, replace:

```js
// Storymaker player: storyboard scenes. buildStoryboard turns one scene (actors, props, camera, beats) into entities
// with tracks. The scheduler: a beat on another target starts at 0.4 of the anchor, a beat on the same target follows it,
// "with": "prev" runs together with the previous beat, and a cued arrival lands on its marker. Timing defaults and role
// eases live in motion.js and are scaled by style.rhythm. Beats: enter exit walk move pose gesture emote look show (item
// and text) hide camera wait; sfx lands by its take's impact; the other beats are reported as minor not_supported_yet and
// ignored. Classic script; publishes StorymakerCore.beats.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});
```

with:

```js
// Storymaker player: storyboard scenes. buildStoryboard turns one scene (actors, props, camera, beats) into entities
// with tracks. The scheduler: a beat on another target starts at 0.4 of the anchor, a beat on the same target follows it,
// "with": "prev" runs together with the previous beat, and a cued arrival lands on its marker. Timing defaults and role
// eases live in motion.js and are scaled by style.rhythm. Beats: enter exit walk move pose gesture emote look show (item,
// text and blocks: beats-blocks.js) chart (the legacy show of a chart) hide highlight camera wait; sfx lands by its take's
// impact; the other beats are reported as minor not_supported_yet and ignored. Classic script; publishes StorymakerCore.beats.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});
```

In `ui/js/storymaker/beats.js`, replace:

```js

        function entryAnim(e, b, t0, dur, pos) {
            var entry = b.entry || 'fade', fade = SM.motion.fadeEase(b);
            if (entry === 'none' || dur <= 0) { e.alpha.add(t0, t0, 1, 'linear'); return; }
            if (entry === 'pop' && e.type !== 'text') {
                e.alpha.add(t0, t0 + dur, 1, fade);
                e.scale.add(t0, t0 + dur, 1, easeOf(b), 0.6);
```

with:

```js

        function entryAnim(e, b, t0, dur, pos) {
            var entry = b.entry || 'fade', fade = SM.motion.fadeEase(b);
            if (entry === 'none' || dur <= 0 || (entry === 'draw' && e.type === 'block')) { e.alpha.add(t0, t0, 1, 'linear'); return; }
            if (entry === 'pop' && e.type !== 'text') {
                e.alpha.add(t0, t0 + dur, 1, fade);
                e.scale.add(t0, t0 + dur, 1, easeOf(b), 0.6);
```

In `ui/js/storymaker/beats.js`, replace:

```js
                }
            },
            show: {
                dur: function (b) { return (b.entry || 'fade') === 'none' ? 0 : DEF.show * f; },
                run: function (b, j, t0, dur, tgt) { if (tgt) entryAnim(tgt, b, t0, dur, { x: tgt.x.init }); }
            },
            hide: {
                dur: function () { return DEF.hide * f; },
```

with:

```js
                }
            },
            show: {
                dur: function (b, floor, tgt) {
                    if ((b.entry || 'fade') === 'none') return 0;
                    return tgt && tgt.type === 'block' ? SM.blocks.revealSeconds(tgt.block) * f : DEF.show * f;
                },
                run: function (b, j, t0, dur, tgt) {
                    if (!tgt) return;
                    if (tgt.type === 'block') { tgt.t0 = t0; tgt.dur = dur; tgt.reveals = SM.beatBlocks.reveals(c, b, tgt.block, t0, dur); }
                    entryAnim(tgt, b, t0, dur, { x: tgt.x.init });
                }
            },
            hide: {
                dur: function () { return DEF.hide * f; },
```

In `ui/js/storymaker/beats.js`, replace:

```js
                dur: function () { return SM.motion.DEFAULTS.highlight * SM.motion.factor(c.story.style); },
                run: function (b, j, t0, dur) {
                    var e = find(b.target);
                    if (!e) { warn('warning', beatPath(j, 'target'), 'unknown_name', 'Nothing named "' + b.target + '" to highlight.'); return; }
                    e.highlights.add(t0, t0 + dur, { style: b.style === 'outline' || b.style === 'arrow' ? b.style : 'pulse' });
                }
```

with:

```js
                dur: function () { return SM.motion.DEFAULTS.highlight * SM.motion.factor(c.story.style); },
                run: function (b, j, t0, dur) {
                    var e = find(b.target);
                    if (!e && SM.beatBlocks.highlightNode(c, b, j, t0, dur, find)) return;
                    if (!e) { warn('warning', beatPath(j, 'target'), 'unknown_name', 'Nothing named "' + b.target + '" to highlight.'); return; }
                    e.highlights.add(t0, t0 + dur, { style: b.style === 'outline' || b.style === 'arrow' ? b.style : 'pulse' });
                }
```

In `ui/js/storymaker/beats.js`, replace:

```js
            }
        };

        var typingBeats = [];

        kinds.sfx = {
```

with:

```js
            }
        };

        kinds.chart = kinds.show;

        var typingBeats = [];

        kinds.sfx = {
```

In `ui/js/storymaker/beats.js`, replace:

```js
        // showTarget(b, j): the instance a show beat creates. It must exist before the beat is placed, because the scheduler
        // compares instances. Returns null when the beat names nothing it can show.
        function showTarget(b, j) {
            if (b.text !== undefined && b.text !== null && b.item === undefined) {
                var pos = textPosition(b.at, beatPath(j, 'at'));
                var style = b.style && cat.textStyles[b.style] ? b.style : 'title';
```

with:

```js
        // showTarget(b, j): the instance a show beat creates. It must exist before the beat is placed, because the scheduler
        // compares instances. Returns null when the beat names nothing it can show.
        function showTarget(b, j) {
            if (SM.beatBlocks.isBlockBeat(b)) return SM.beatBlocks.target(c, b, j, { warn: warn, beatPath: beatPath, instanceId: instanceId, register: register });
            if (b.text !== undefined && b.text !== null && b.item === undefined) {
                var pos = textPosition(b.at, beatPath(j, 'at'));
                var style = b.style && cat.textStyles[b.style] ? b.style : 'title';
```

In `ui/js/storymaker/beats.js`, replace:

```js
        // targetOf(b, j): the instance a beat acts on, compared by identity. A show's target is the instance it creates. A
        // string that resolves to no instance (a camera on a spot) has none.
        function targetOf(b, j) {
            if (b.do === 'show') return showTarget(b, j);
            return b.do === 'wait' || b.do === 'sfx' ? null : find(b.who || b.target);
        }
```

with:

```js
        // targetOf(b, j): the instance a beat acts on, compared by identity. A show's target is the instance it creates. A
        // string that resolves to no instance (a camera on a spot) has none.
        function targetOf(b, j) {
            if (b.do === 'show' || b.do === 'chart') return showTarget(b, j);
            return b.do === 'wait' || b.do === 'sfx' ? null : find(b.who || b.target);
        }
```

In `ui/js/storymaker/beats.js`, replace:

```js
                warn('minor', beatPath(j), 'not_supported_yet', 'Beat "' + b.do + '" is not supported yet; it is ignored.');
                return;
            }
            if (b.do === 'show' && (b.chart || b.icon)) {
                warn('minor', beatPath(j), 'not_supported_yet', 'Show with ' + (b.chart ? 'a chart' : 'an icon') + ' is not supported yet; the beat is ignored.');
                return;
            }
            var tgt = targetOf(b, j);
            var floor = tgt ? (lastEnd.get(tgt) || 0) : 0;
            // a walk measures its distance from the target's position at floor: only same-target beats move that track
            var dur = typeof b.duration === 'number' && b.duration >= 0 ? b.duration : spec.dur(b, floor);
            // impact: the chosen take's impact for an sfx, 0 for a sound the catalog lacks and for every other beat
            var delay = b.delay > 0 ? b.delay : 0, lead = SM.motion.leadOf(b, dur), impact = b.do === 'sfx' ? sfxImpact(b, j) : 0, cueT, byPrev = false, t;
            if (b.cue !== undefined && b.cue !== null) {
```

with:

```js
                warn('minor', beatPath(j), 'not_supported_yet', 'Beat "' + b.do + '" is not supported yet; it is ignored.');
                return;
            }
            var tgt = targetOf(b, j);
            if (!tgt && SM.beatBlocks.isBlockBeat(b)) return;   // a block that failed its check is skipped like an unsupported beat
            var floor = tgt ? (lastEnd.get(tgt) || 0) : 0;
            // a walk measures its distance from the target's position at floor: only same-target beats move that track
            var dur = typeof b.duration === 'number' && b.duration >= 0 ? b.duration : spec.dur(b, floor, tgt);
            // impact: the chosen take's impact for an sfx, 0 for a sound the catalog lacks and for every other beat
            var delay = b.delay > 0 ? b.delay : 0, lead = SM.motion.leadOf(b, dur), impact = b.do === 'sfx' ? sfxImpact(b, j) : 0, cueT, byPrev = false, t;
            if (b.cue !== undefined && b.cue !== null) {
```

In `ui/js/storymaker/compile.js`, replace:

```js
                    issues.push(P.issue('minor', sc.id, '/scenes/' + i + '/beats/' + j, 'not_supported_yet', 'Beats of a code scene are ignored; the code scene draws everything.'));
                });
            } else {
                built = SM.beats.buildStoryboard({ story: story, scene: sc, index: i, plan: plan, cat: cat, fmt: fmt, stage: stage, issues: issues, needs: needs, castMap: castMap, lastTake: lastTake });
            }

            // duration (scene-local seconds, origin = start of the entry transition)
```

with:

```js
                    issues.push(P.issue('minor', sc.id, '/scenes/' + i + '/beats/' + j, 'not_supported_yet', 'Beats of a code scene are ignored; the code scene draws everything.'));
                });
            } else {
                built = SM.beats.buildStoryboard({ story: story, scene: sc, index: i, plan: plan, cat: cat, fmt: fmt, stage: stage, issues: issues, needs: needs, castMap: castMap, lastTake: lastTake, env: envEntry, variant: variant });
            }

            // duration (scene-local seconds, origin = start of the entry transition)
```

In `ui/js/storymaker/compile.js`, replace:

```js

            // fonts in use
            scene.entities.forEach(function (e) {
                if (e.type === 'actor') {
                    var character = cat.byId[e.character];
                    if (character && character.file) needs['catalog:' + character.file] = true;
```

with:

```js

            // fonts in use
            scene.entities.forEach(function (e) {
                if (e.type === 'block') {
                    // blocks draw in the film font at 600 and 700; their texts need the fallback faces like any text
                    [600, 700].forEach(function (w) { var k = fontKey(cat, styleFont, w) || fontKey(cat, 'default', w); if (k) needs[k] = true; });
                    SM.beatBlocks.texts(e).forEach(needGlyphs);
                }
                if (e.type === 'actor') {
                    var character = cat.byId[e.character];
                    if (character && character.file) needs['catalog:' + character.file] = true;
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `npm run test:storymaker`
Expected: 300 passed, 0 failed; bundles rebuilt, check:ui ok

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-bundle-parts.js scripts/storymaker-tests/04-compile.mjs scripts/storymaker-tests/34-block-beats.mjs ui/js/storymaker/beats-blocks.js ui/js/storymaker/beats.js ui/js/storymaker/compile.js ui/js/storymaker/player.bundle.js
git commit -m "storymaker: beats play icons, charts, diagrams, headlines and lists"
```

### Task D6: the layout run measures blocks, in code scenes too

**Files:**
- Modify: `scripts/storymaker-tests/30-layout.mjs`
- Modify: `ui/js/storymaker/compile.js`
- Modify: `ui/js/storymaker/host-core.js`
- Modify: `ui/js/storymaker/host.bundle.js` (generated)
- Modify: `ui/js/storymaker/layout.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)
- Modify: `ui/js/storymaker/worker-scene.js`

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/30-layout.mjs`, replace:

```js
    near(wide[1] + wide[3], 0.92, 0.01, '16:9 bottom');
    near(wide[0] + wide[2] / 2, 0.5, 0.002, '16:9 stays centred');
});
```

with:

```js
    near(wide[1] + wide[3], 0.92, 0.01, '16:9 bottom');
    near(wide[0] + wide[2] / 2, 0.5, 0.002, '16:9 stays centred');
});

// ---- Batch D6: block boxes, also in code scenes
const flowBeat = { do: 'show', diagram: { layout: 'flow', nodes: [{ id: 'a', label: 'Anfrage' }, { id: 'b', label: 'Server' }] }, at: 'full', entry: 'draw' };
const noActors = beats => film({}, { actors: [], beats });

check('layout: a diagram at full gives a block with its kind, layout, label size and a box inside the frame', () => {
    const b = measure(noActors([flowBeat]))[0];
    assert.equal(b.blocks.length, 1);
    const [d] = b.blocks;
    assert.deepEqual(plain([d.id, d.kind, d.layout]), ['diagram-1', 'diagram', 'flow']);
    assert.ok(d.label > 0 && d.label < 0.2, 'label ' + d.label);
    assert.ok(d.box.every(v => v >= 0 && v <= 1) && d.box[2] > 0 && d.box[3] > 0 && d.box[0] + d.box[2] <= 1 && d.box[1] + d.box[3] <= 1, 'box ' + d.box);
    assert.deepEqual(plain(measure(film())[0].blocks), [], 'a scene without blocks has an empty list');
});

check('layout: a scene whose only change is a block is not static', () => {
    assert.equal(measure(noActors([flowBeat]))[0].static, false);
    assert.equal(measure(noActors([]))[0].static, true);
});

check('layout: a code scene measures its blocks under the code camera, holds after the last one and survives a bad payload', () => {
    const stage = SM.stage.create({ aspect: '16:9', env: null });
    const head = SM.blocks.create({ id: 'headline-1', kind: 'headline', text: 'Hallo *Welt*', area: SM.blocks.area(stage, null, 'day', 'top'), t0: 2, dur: 1, entry: 'fade', hidden: true });
    head.alpha.add(2, 3, 1, 'ease-out');
    const wire = JSON.parse(JSON.stringify(SM.blocks.serialize([head], SM.camera.create(1, 960, 540))));
    const run = blocks => {
        const two = film();
        two.scenes.push({ id: 's2', kind: 'code', narration: 'Ein Code.' });
        const tl = SM.compile(two, {}, t.catalog, { codeScenes: { s2: { duration: 4, blocks } } }).timeline;
        return { code: SM.layout.measure(tl, renderAssets())[1], sc: tl.scenes[1], tl };
    };
    const ok = run(wire), D = (ok.sc.end_frame - ok.sc.start_frame) / ok.tl.fps;
    assert.equal(ok.code.code, true);
    assert.equal(ok.code.blocks.length, 1);
    assert.deepEqual(plain([ok.code.blocks[0].id, ok.code.blocks[0].kind, ok.code.blocks[0].layout]), ['headline-1', 'headline', '']);
    assert.ok(ok.code.hold_s >= 3.5 - 1e-6 && ok.code.hold_s <= D, 'hold ' + ok.code.hold_s + ' of ' + D);
    const bad = run({ ...wire, blocks: [{ ...wire.blocks[0], w: 'wide' }] });
    assert.equal(bad.code.code, true);
    assert.deepEqual(plain(bad.code.blocks), []);
    assert.deepEqual(plain(run(null).code.blocks), []);
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `npm run test:storymaker`
Expected: no blocks key in the layout block (3 new 30-layout checks fail)

- [ ] **Step 3: Implement**

In `ui/js/storymaker/compile.js`, replace:

```js
                id: sc.id, index: i, kind: isCode ? 'code' : 'storyboard', start_frame: startFrame, end_frame: startFrame + dFrames,
                tf: plan.tf, transition: plan.transition, requested_transition: plan.requested, env: envEntry, variant: variant,
                narration: n, entities: built.entities, camera: built.camera, framing: built.framing,
                code: codeInfo ? { error: codeInfo.error || null, duration: codeInfo.duration || null } : null, stages: {}
            };
            scenes.push(scene);
            if (i === 0) firstScene = { id: sc.id, narrated: n.words > 0, log: built.log || [] };
```

with:

```js
                id: sc.id, index: i, kind: isCode ? 'code' : 'storyboard', start_frame: startFrame, end_frame: startFrame + dFrames,
                tf: plan.tf, transition: plan.transition, requested_transition: plan.requested, env: envEntry, variant: variant,
                narration: n, entities: built.entities, camera: built.camera, framing: built.framing,
                code: codeInfo ? { error: codeInfo.error || null, duration: codeInfo.duration || null } : null, stages: {},
                codeLayout: codeInfo && codeInfo.blocks ? SM.blocks.revive(codeInfo.blocks, cat) : null
            };
            scenes.push(scene);
            if (i === 0) firstScene = { id: sc.id, narrated: n.words > 0, log: built.log || [] };
```

In `ui/js/storymaker/host-core.js`, replace:

```js
            var reply = await channel.call({ type: 'build', opts: opts }, left);
            if (generation !== this.loadId) { channel.kill(fail('cancelled', 'superseded')); return { error: fail('cancelled', 'superseded') }; }
            this.sceneChannels[id] = channel;
            return { duration: reply.duration, explicit: reply.explicit === true, needs: reply.needs || [], issues: reply.issues || [] };
        } catch (err) {
            if (channel) channel.kill(err);
            var e = asError(err);
```

with:

```js
            var reply = await channel.call({ type: 'build', opts: opts }, left);
            if (generation !== this.loadId) { channel.kill(fail('cancelled', 'superseded')); return { error: fail('cancelled', 'superseded') }; }
            this.sceneChannels[id] = channel;
            return { duration: reply.duration, explicit: reply.explicit === true, needs: reply.needs || [], issues: reply.issues || [], blocks: reply.blocks || null };
        } catch (err) {
            if (channel) channel.kill(err);
            var e = asError(err);
```

In `ui/js/storymaker/host-core.js`, replace:

```js
                self.codeErrors[cid] = built.error.message;
                env.onError({ code: built.error.code, message: built.error.message, scene_id: cid, line: built.error.line, column: built.error.column });
            } else {
                codeScenes[cid] = { duration: built.duration, explicit: built.explicit };
                sceneNeeds[cid] = built.needs;
                built.issues.forEach(function (i) { sceneIssues.push(i); });
                self.codeSources[cid] = { msg: msg, plan: plan };
```

with:

```js
                self.codeErrors[cid] = built.error.message;
                env.onError({ code: built.error.code, message: built.error.message, scene_id: cid, line: built.error.line, column: built.error.column });
            } else {
                codeScenes[cid] = { duration: built.duration, explicit: built.explicit, blocks: built.blocks };
                sceneNeeds[cid] = built.needs;
                built.issues.forEach(function (i) { sceneIssues.push(i); });
                self.codeSources[cid] = { msg: msg, plan: plan };
```

In `ui/js/storymaker/layout.js`, replace:

```js
// its last cue arrival and its last motion, plus half a second), and reports where the actors, the shown props, the texts and
// the burned caption are, as boxes in fractions of the output frame. Go turns the boxes into findings. It reads the compiled
// timeline and the loaded art and draws nothing; the one canvas it uses only measures text. Classic script; publishes
// StorymakerCore.layout. A code scene gets a block with code true and no boxes of its own.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});
```

with:

```js
// its last cue arrival and its last motion, plus half a second), and reports where the actors, the shown props, the texts and
// the burned caption are, as boxes in fractions of the output frame. Go turns the boxes into findings. It reads the compiled
// timeline and the loaded art and draws nothing; the one canvas it uses only measures text. Classic script; publishes
// StorymakerCore.layout. A code scene gets a block with code true and the boxes of its explainer blocks (sc.codeLayout, from the
// scene worker); raw drawing in a code scene is never measured.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});
```

In `ui/js/storymaker/layout.js`, replace:

```js
    // holdSeconds(timeline, index): the scene-local time of the sample, clamped to the last frame in which the scene is the
    // one on screen (the next scene's entry transition overlaps its end).
    // A code scene has no stage entities of the storyboard: it is sampled at the larger of 70 % of its length and half a second
    // after its last cue, and never after its last frame.
    function holdSeconds(timeline, index) {
        var sc = timeline.scenes[index], next = timeline.scenes[index + 1], fps = timeline.fps, last = 0;
        var cues = sc.narration && sc.narration.cueList || [];
        for (var i = 0; i < cues.length; i++) if (cues[i].at_ms / 1000 > last) last = cues[i].at_ms / 1000;
        var lastFrame = Math.min(sc.end_frame, next ? next.start_frame : sc.end_frame) - 1;
        var limit = Math.max(0, (lastFrame - sc.start_frame) / fps);
        if (sc.kind === 'code') return Math.min(Math.max(CODE_HOLD_SHARE * (sc.end_frame - sc.start_frame) / fps, last + HOLD_AFTER), limit);
        for (var k = 0; k < sc.entities.length; k++) { var end = SM.scene.entityEnd(sc.entities[k]); if (end > last) last = end; }
        return Math.min(last + HOLD_AFTER, limit);
    }
```

with:

```js
    // holdSeconds(timeline, index): the scene-local time of the sample, clamped to the last frame in which the scene is the
    // one on screen (the next scene's entry transition overlaps its end).
    // A code scene has no stage entities of the storyboard: it is sampled at the larger of 70 % of its length and half a second
    // after its last cue or the end of its last block, and never after its last frame.
    function holdSeconds(timeline, index) {
        var sc = timeline.scenes[index], next = timeline.scenes[index + 1], fps = timeline.fps, last = 0;
        var cues = sc.narration && sc.narration.cueList || [];
        for (var i = 0; i < cues.length; i++) if (cues[i].at_ms / 1000 > last) last = cues[i].at_ms / 1000;
        var lastFrame = Math.min(sc.end_frame, next ? next.start_frame : sc.end_frame) - 1;
        var limit = Math.max(0, (lastFrame - sc.start_frame) / fps);
        if (sc.kind === 'code') {
            var held = sc.codeLayout ? sc.codeLayout.entities : [];
            for (var h = 0; h < held.length; h++) { var he = SM.scene.entityEnd(held[h]); if (he > last) last = he; }
            return Math.min(Math.max(CODE_HOLD_SHARE * (sc.end_frame - sc.start_frame) / fps, last + HOLD_AFTER), limit);
        }
        for (var k = 0; k < sc.entities.length; k++) { var end = SM.scene.entityEnd(sc.entities[k]); if (end > last) last = end; }
        return Math.min(last + HOLD_AFTER, limit);
    }
```

In `ui/js/storymaker/layout.js`, replace:

```js
        var out = [];
        for (var i = 0; i < sc.entities.length; i++) {
            var e = sc.entities[i];
            if (e.type !== 'actor' && e.type !== 'prop' && e.type !== 'text' && e.type !== 'shape') continue;
            var s = SM.scene.evalEntity(e, t);
            out.push([
                e.id, Math.round(s.x / EPS), Math.round(s.y / EPS), Math.round(s.alpha * 50), Math.round(s.scale * 50), Math.round(s.rot * 50),
                typeof s.pose === 'string' ? s.pose : (s.pose && s.pose.b) || '', s.expression || '', s.gesture && s.gesture.name || ''
```

with:

```js
        var out = [];
        for (var i = 0; i < sc.entities.length; i++) {
            var e = sc.entities[i];
            if (e.type !== 'actor' && e.type !== 'prop' && e.type !== 'text' && e.type !== 'shape' && e.type !== 'block') continue;
            var s = SM.scene.evalEntity(e, t);
            if (e.type === 'block') { out.push([e.id, SM.blocks.cacheKey(s), Math.round(SM.blocks.itemProgress(s, 0) * 50), Math.round(SM.blocks.itemProgress(s, Math.max(0, SM.blocks.itemCount(s.block) - 1)) * 50)]); continue; }
            out.push([
                e.id, Math.round(s.x / EPS), Math.round(s.y / EPS), Math.round(s.alpha * 50), Math.round(s.scale * 50), Math.round(s.rot * 50),
                typeof s.pose === 'string' ? s.pose : (s.pose && s.pose.b) || '', s.expression || '', s.gesture && s.gesture.name || ''
```

In `ui/js/storymaker/layout.js`, replace:

```js
        return JSON.stringify(out);
    }

    function measureScene(timeline, index, rc, ctx) {
        var sc = timeline.scenes[index], W = timeline.width, H = timeline.height, fps = timeline.fps;
        var t = holdSeconds(timeline, index), stage = SM.render.stageFor(sc, rc);
        var win = stage.cameraWindow(SM.camera.at(sc.camera, t)), base = stage.baseWindow();
        var code = sc.kind === 'code';
        var block = { id: sc.id, index: sc.index, hold_s: round4(t), code: code, actors: [], props: [], texts: [], caption: null, missing: 0, static: false };
        for (var i = 0; i < sc.entities.length && !code; i++) {
            var e = sc.entities[i], st = SM.scene.evalEntity(e, t);
            if (!(st.alpha > VISIBLE)) continue;
```

with:

```js
        return JSON.stringify(out);
    }

    // layoutOf(block): the diagram layout, the chart type or the icon style; "" for a headline and a list.
    function layoutOf(b) {
        if (b.kind === 'diagram') return b.diagram.layout;
        if (b.kind === 'chart') return b.chart.type;
        return b.kind === 'icon' ? b.style || 'line' : '';
    }

    // blockBoxes(entities, camera, t, rc, ctx, stage, W, H): the boxes of the visible blocks under a camera at scene time t.
    function blockBoxes(entities, camera, t, rc, ctx, stage, W, H) {
        var out = [], win = camera ? stage.cameraWindow(SM.camera.at(camera, t)) : stage.baseWindow();
        for (var i = 0; i < entities.length && ctx; i++) {
            var st = SM.scene.evalEntity(entities[i], t);
            if (entities[i].type !== 'block' || !(st.alpha > VISIBLE)) continue;
            var m = SM.blocks.measure(ctx, st, rc, stage, win);
            if (m) out.push({ id: st.id, kind: st.block.kind, layout: layoutOf(st.block), label: round4(m.labelPx / Math.min(W, H)), box: frameBox(m.box, win, stage, W, H) });
        }
        return out;
    }

    function measureScene(timeline, index, rc, ctx) {
        var sc = timeline.scenes[index], W = timeline.width, H = timeline.height, fps = timeline.fps;
        var t = holdSeconds(timeline, index), stage = SM.render.stageFor(sc, rc);
        var win = stage.cameraWindow(SM.camera.at(sc.camera, t)), base = stage.baseWindow();
        var code = sc.kind === 'code';
        var block = { id: sc.id, index: sc.index, hold_s: round4(t), code: code, actors: [], props: [], texts: [], blocks: [], caption: null, missing: 0, static: false };
        for (var i = 0; i < sc.entities.length && !code; i++) {
            var e = sc.entities[i], st = SM.scene.evalEntity(e, t);
            if (!(st.alpha > VISIBLE)) continue;
```

In `ui/js/storymaker/layout.js`, replace:

```js
                if (tb) block.texts.push({ id: e.id, box: pixelBox(tb, W, H) });
            }
        }
        if (timeline.style.captions && timeline.captions && timeline.captions.length && ctx) {
            var cap = SM.captions.at(timeline.captions, sc.start_frame * 1000 / fps + t * 1000);
            if (cap) block.caption = pixelBox(SM.render.captionBox(ctx, cap, rc), W, H);
```

with:

```js
                if (tb) block.texts.push({ id: e.id, box: pixelBox(tb, W, H) });
            }
        }
        block.blocks = code ? (sc.codeLayout ? blockBoxes(sc.codeLayout.entities, sc.codeLayout.camera, t, rc, ctx, stage, W, H) : []) : blockBoxes(sc.entities, sc.camera, t, rc, ctx, stage, W, H);
        if (timeline.style.captions && timeline.captions && timeline.captions.length && ctx) {
            var cap = SM.captions.at(timeline.captions, sc.start_frame * 1000 / fps + t * 1000);
            if (cap) block.caption = pixelBox(SM.render.captionBox(ctx, cap, rc), W, H);
```

In `ui/js/storymaker/worker-scene.js`, replace:

```js
                var built;
                try { built = SM.codescene.build(fn, msg.opts); } catch (err) { throw sceneError(err, 'Scene "' + msg.opts.sceneId + '"'); }
                this.built = built;
                return { payload: { duration: built.duration, explicit: built.explicit, needs: built.needs, issues: built.issues } };
            }
            case 'assets': {
                var assets = msg.assets || {}, k;
```

with:

```js
                var built;
                try { built = SM.codescene.build(fn, msg.opts); } catch (err) { throw sceneError(err, 'Scene "' + msg.opts.sceneId + '"'); }
                this.built = built;
                return { payload: { duration: built.duration, explicit: built.explicit, needs: built.needs, issues: built.issues, blocks: SM.blocks.serialize(built.entities, built.camera) } };
            }
            case 'assets': {
                var assets = msg.assets || {}, k;
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `npm run test:storymaker`
Expected: 303 passed, 0 failed; bundles rebuilt, check:ui ok

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/30-layout.mjs ui/js/storymaker/compile.js ui/js/storymaker/host-core.js ui/js/storymaker/host.bundle.js ui/js/storymaker/layout.js ui/js/storymaker/player.bundle.js ui/js/storymaker/worker-scene.js
git commit -m "storymaker: the layout run measures blocks, in code scenes too"
```

### Task D7: Go layout rules for blocks

**Files:**
- Modify: `internal/storymaker/layout.go`
- Modify: `internal/storymaker/layout_test.go`
- Modify: `internal/storymaker/render/compilecheck.go`
- Modify: `internal/storymaker/render/compilecheck_layout_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/layout_test.go`, replace:

```go
		t.Fatalf("a storyboard scene reported %v", got)
	}
}
```

with:

```go
		t.Fatalf("a storyboard scene reported %v", got)
	}
}

func TestLayoutIssuesForBlocks(t *testing.T) {
	caption := LayoutBox{0.2, 0.794, 0.6, 0.126}
	sc := layoutScene("s1", 0)
	sc.Caption = &caption
	sc.Actors = []LayoutActor{{ID: "anna", Box: LayoutBox{0.5, 0.2, 0.1, 0.4}}}
	sc.Blocks = []LayoutBlock{
		{ID: "covered", Kind: "chart", Layout: "bar", Label: 0.03, Box: LayoutBox{0.3, 0.5, 0.3, 0.4}},
		{ID: "outside", Kind: "list", Label: 0.03, Box: LayoutBox{0.8, 0.1, 0.4, 0.3}},
		{ID: "tiny", Kind: "headline", Label: 0.03, Box: LayoutBox{0.1, 0.1, 0.3, 0.05}},
		{ID: "hidden", Kind: "diagram", Layout: "flow", Label: 0.03, Box: LayoutBox{0.45, 0.2, 0.2, 0.3}},
		{ID: "label", Kind: "diagram", Layout: "flow", Label: 0.02, Box: LayoutBox{0.05, 0.55, 0.3, 0.2}},
		{ID: "fine", Kind: "diagram", Layout: "flow", Label: 0.03, Box: LayoutBox{0.05, 0.05, 0.3, 0.4}},
	}
	got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{})
	want := []string{CodeCaptionOverlap, CodeOffscreen, CodeBlockTooSmall, CodeActorPropOverlap, CodeBlockTooSmall}
	if !slices.Equal(layoutCodes(got), want) {
		t.Fatalf("issues = %v, want %v", layoutCodes(got), want)
	}
	for _, is := range got {
		if is.Severity != SeverityWarning || is.Fix == "" || !IssueLayout(is.Code) || !IssueModelFixable(is) || strings.Contains(is.Fix, "api.safe()") {
			t.Errorf("a storyboard block finding is a warning with a patch_scene fix: %+v", is)
		}
	}
	if !strings.Contains(got[0].Message, "covered") || !strings.Contains(got[2].Fix, `"full"`) {
		t.Errorf("messages: %+v", got)
	}
	// The caption rule holds without composition v2, and a code scene names api.safe().
	sc.Code = true
	for _, is := range LayoutIssues([]LayoutScene{sc}, LayoutOptions{}) {
		if !strings.Contains(is.Fix, "api.safe()") || !strings.Contains(is.Fix, "put_code") {
			t.Errorf("a code scene fix names api.safe(): %+v", is)
		}
	}
	sc.Blocks = []LayoutBlock{{ID: "ok", Kind: "chart", Label: 0.03, Box: LayoutBox{0.1, 0.1, 0.5, 0.5}}}
	sc.Actors = nil
	if got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{}); len(got) != 0 {
		t.Fatalf("a block inside the safe area with label 0.03: %v", layoutCodes(got))
	}
}
```

In `internal/storymaker/render/compilecheck_layout_test.go`, replace:

```go
		t.Fatalf("the first 64 are kept: %+v", second.Actors[63])
	}
}
```

with:

```go
		t.Fatalf("the first 64 are kept: %+v", second.Actors[63])
	}
}

func TestSummarizeCompileReportBoundsBlocks(t *testing.T) {
	want := reportExpectation()
	want.Prefix, want.Token = "/r/job-7f3a/", "tok9d2c"
	box := []float64{0.1, 0.1, 0.4, 0.4}
	blocks := make([]any, 0, 70)
	for i := 0; i < 70; i++ {
		blocks = append(blocks, map[string]any{"id": fmt.Sprintf("diagram-%d", i), "kind": "diagram", "layout": "flow", "label": 0.03, "box": box})
	}
	blocks[0] = map[string]any{"id": "d" + want.Token + "1" + strings.Repeat("x", 80), "kind": "chart" + want.Token, "layout": "bar", "label": 0.03, "box": box}
	m := goodReport()
	m["layout"] = []any{map[string]any{"id": "s1", "index": 0, "hold_s": 1, "actors": []any{}, "props": []any{}, "texts": []any{}, "blocks": blocks}}
	body, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	got := summarizeCompileReport(body, want, 0)
	if !got.Available || len(got.Layout) != 1 || len(got.Layout[0].Blocks) != 64 {
		t.Fatalf("70 blocks are cut to 64: %+v", got.Layout)
	}
	first := got.Layout[0].Blocks[0]
	if strings.Contains(first.ID, want.Token) || len(first.ID) > 64 || first.Kind != "chart" || first.Layout != "bar" || first.Label != 0.03 || first.Box != [4]float64{0.1, 0.1, 0.4, 0.4} {
		t.Fatalf("the first block is cleaned and keeps its geometry: %+v", first)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker/... -run '(?i)(TestLayoutIssuesForBlocks|TestSummarizeCompileReport)' -count=1`
Expected: build failed, LayoutBlock and LayoutScene.Blocks undefined

- [ ] **Step 3: Implement**

In `internal/storymaker/layout.go`, replace:

```go
	Box LayoutBox `json:"box"`
}

// LayoutScene is the measured layout of one scene at its hold frame. A code scene (Code) has no boxes of its own, because its
// raw drawing is never measured; it is listed so that stills, the contact sheet and the checks see its hold frame. Caption is the burned caption of that
// moment, nil when none is on screen. Missing counts boxes the player could not measure for lack of art. Static says that
// nothing on the stage changed between the first frame and the hold frame, apart from the idle cycle.
type LayoutScene struct {
```

with:

```go
	Box LayoutBox `json:"box"`
}

// LayoutBlock is the box of one explainer block (icon, diagram, chart, headline or list). Layout is the diagram layout, the chart
// type or the icon style, "" for a headline and a list. Label is the smallest label of the block as a share of the short side
// of the frame, 0 when it has no label.
type LayoutBlock struct {
	ID     string    `json:"id"`
	Kind   string    `json:"kind"`
	Layout string    `json:"layout"`
	Label  float64   `json:"label"`
	Box    LayoutBox `json:"box"`
}

// LayoutScene is the measured layout of one scene at its hold frame. A code scene (Code) has no actor, prop or text boxes,
// because its raw drawing is never measured; it carries the boxes of its explainer blocks and is listed so that stills, the
// contact sheet and the checks see its hold frame. Caption is the burned caption of that
// moment, nil when none is on screen. Missing counts boxes the player could not measure for lack of art. Static says that
// nothing on the stage changed between the first frame and the hold frame, apart from the idle cycle.
type LayoutScene struct {
```

In `internal/storymaker/layout.go`, replace:

```go
	Actors  []LayoutActor `json:"actors"`
	Props   []LayoutProp  `json:"props"`
	Texts   []LayoutText  `json:"texts"`
	Caption *LayoutBox    `json:"caption"`
	Missing int           `json:"missing"`
	Static  bool          `json:"static"`
```

with:

```go
	Actors  []LayoutActor `json:"actors"`
	Props   []LayoutProp  `json:"props"`
	Texts   []LayoutText  `json:"texts"`
	Blocks  []LayoutBlock `json:"blocks"`
	Caption *LayoutBox    `json:"caption"`
	Missing int           `json:"missing"`
	Static  bool          `json:"static"`
```

In `internal/storymaker/layout.go`, replace:

```go
	layoutZoneShare     = 0.02 // texts and the caption: this share of the box inside the safe zone is too much
	layoutOverlapShare  = 0.35 // of the smaller box
	layoutScaleMax      = 2.0  // the validated range of a beat scale
)

// LayoutOptions say which rules apply. CaptionRule is true for a composition v2 film with captions on in 16:9 or 1:1: the
```

with:

```go
	layoutZoneShare     = 0.02 // texts and the caption: this share of the box inside the safe zone is too much
	layoutOverlapShare  = 0.35 // of the smaller box
	layoutScaleMax      = 2.0  // the validated range of a beat scale

	layoutBlockMinHeight    = 0.10  // a block box below this share of the frame height is too small
	layoutBlockMinLabel     = 0.025 // a label below this share of the short side is too small
	layoutBlockCaptionShare = 0.02  // this share of a block box under the caption counts as covered
)

// LayoutOptions say which rules apply. CaptionRule is true for a composition v2 film with captions on in 16:9 or 1:1: the
```

In `internal/storymaker/layout.go`, replace:

```go
// block an outcome.
func IssueLayout(code string) bool {
	switch code {
	case CodeCaptionOverlap, CodePropTooSmall, CodeOffscreen, CodeActorPropOverlap, CodeSceneStatic, CodeScenesLookAlike:
		return true
	}
	return false
```

with:

```go
// block an outcome.
func IssueLayout(code string) bool {
	switch code {
	case CodeCaptionOverlap, CodePropTooSmall, CodeOffscreen, CodeActorPropOverlap, CodeSceneStatic, CodeScenesLookAlike, CodeBlockTooSmall:
		return true
	}
	return false
```

In `internal/storymaker/layout.go`, replace:

```go
			add(CodeTextUnsafe, fmt.Sprintf("The caption lies in the margin that platform overlays cover in scene %s.", sc.ID),
				`Shorten the narration of the scene so the caption has fewer lines.`)
		}
		for _, a := range sc.Actors {
			for _, p := range sc.Props {
				if !p.Back && overlapShare(a.Box, p.Box) > layoutOverlapShare {
```

with:

```go
			add(CodeTextUnsafe, fmt.Sprintf("The caption lies in the margin that platform overlays cover in scene %s.", sc.ID),
				`Shorten the narration of the scene so the caption has fewer lines.`)
		}
		blockFix := func(storyboard string) string {
			if sc.Code {
				return "Keep it inside api.safe(): change at in the source with put_code."
			}
			return storyboard
		}
		const blockAreas = `Show it at "full", "left", "right", "top" or "board" with patch_scene and the whole beats list.`
		for _, b := range sc.Blocks {
			if sc.Caption != nil && area(b.Box) > 0 && intersection(b.Box, *sc.Caption)/area(b.Box) > layoutBlockCaptionShare {
				add(CodeCaptionOverlap, fmt.Sprintf("The caption covers the block %s in scene %s.", b.ID, sc.ID), blockFix(blockAreas))
			}
			if outsideShare(b.Box) > layoutTextOutside {
				add(CodeOffscreen, fmt.Sprintf("The block %s is partly outside the frame in scene %s.", b.ID, sc.ID), blockFix(blockAreas))
			}
			if b.Box[3] < layoutBlockMinHeight || (b.Label > 0 && b.Label < layoutBlockMinLabel) {
				add(CodeBlockTooSmall, fmt.Sprintf("The block %s is too small to read in scene %s.", b.ID, sc.ID),
					blockFix(`Show it at "full" or on a bigger area, or give it fewer nodes or shorter labels.`))
			}
			for _, a := range sc.Actors {
				if overlapShare(a.Box, b.Box) > layoutOverlapShare {
					add(CodeActorPropOverlap, fmt.Sprintf("%s hides the block %s in scene %s.", a.ID, b.ID, sc.ID), blockFix(blockAreas))
				}
			}
		}
		for _, a := range sc.Actors {
			for _, p := range sc.Props {
				if !p.Back && overlapShare(a.Box, p.Box) > layoutOverlapShare {
```

In `internal/storymaker/render/compilecheck.go`, replace:

```go
	}
}

// layoutMaxBoxes bounds the actors, props and texts the page may report for one scene. A story holds far fewer.
const layoutMaxBoxes = 64

// cleanLayoutScene removes the render secrets from the ids the page wrote into a layout block and bounds their length and
```

with:

```go
	}
}

// layoutMaxBoxes bounds the actors, props, texts and blocks the page may report for one scene. A story holds far fewer.
const layoutMaxBoxes = 64

// cleanLayoutScene removes the render secrets from the ids the page wrote into a layout block and bounds their length and
```

In `internal/storymaker/render/compilecheck.go`, replace:

```go
	for i := range block.Texts {
		block.Texts[i].ID = want.clean(block.Texts[i].ID, 60)
	}
	return block
}
```

with:

```go
	for i := range block.Texts {
		block.Texts[i].ID = want.clean(block.Texts[i].ID, 60)
	}
	block.Blocks = slices.Clone(block.Blocks[:min(len(block.Blocks), layoutMaxBoxes)])
	for i := range block.Blocks {
		block.Blocks[i].ID = want.clean(block.Blocks[i].ID, 60)
		block.Blocks[i].Kind = want.clean(block.Blocks[i].Kind, 20)
		block.Blocks[i].Layout = want.clean(block.Blocks[i].Layout, 20)
	}
	return block
}
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker/... -run '(?i)(TestLayoutIssuesForBlocks|TestSummarizeCompileReport)' -count=1`
Expected: ok internal/storymaker and render; whole internal/storymaker/... ok

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/layout.go internal/storymaker/layout_test.go internal/storymaker/render/compilecheck.go internal/storymaker/render/compilecheck_layout_test.go
git commit -m "storymaker: Go layout rules for blocks"
```

### Task D8: the agent's beat schema has the block fields

**Files:**
- Modify: `internal/agent/native_tools_storymaker.go`
- Modify: `internal/agent/native_tools_storymaker_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/agent/native_tools_storymaker_test.go`, replace:

```go
func TestStorymakerBeatSchemaOffersCuePhrase(t *testing.T) {
	names := storymakerCatalogNames(t)
	for _, compact := range []bool{false, true} {
		story := storymakerToolParams(StorymakerPhaseToolSchemas(storymaker.PhaseFill, compact, names), storymaker.ToolStory)
		phrase := storymakerSchemaAt(story, "data", "beats", "[]", "cue_phrase")
		if phrase == nil || phrase["type"] != "string" || phrase["maxLength"] != 60 || phrase["description"] == "" {
			t.Errorf("compact=%v cue_phrase = %v, want a described string of at most 60 characters", compact, phrase)
		}
		if storymakerSchemaAt(story, "data", "beats", "[]", "cue") == nil {
			t.Errorf("compact=%v beat lost cue", compact)
		}
	}
}
```

with:

```go
func TestStorymakerBeatSchemaOffersCuePhrase(t *testing.T) {
	names := storymakerCatalogNames(t)
	for _, compact := range []bool{false, true} {
		story := storymakerToolParams(StorymakerPhaseToolSchemas(storymaker.PhaseFill, compact, names), storymaker.ToolStory)
		phrase := storymakerSchemaAt(story, "data", "beats", "[]", "cue_phrase")
		if phrase == nil || phrase["type"] != "string" || phrase["maxLength"] != 60 || (phrase["description"] == nil) != compact {
			t.Errorf("compact=%v cue_phrase = %v, want a described string of at most 60 characters", compact, phrase)
		}
		if storymakerSchemaAt(story, "data", "beats", "[]", "cue") == nil {
			t.Errorf("compact=%v beat lost cue", compact)
		}
	}
}

func TestStorymakerBeatSchemaHasBlocks(t *testing.T) {
	names := storymakerCatalogNames(t)
	for _, compact := range []bool{false, true} {
		story := storymakerToolParams(StorymakerPhaseToolSchemas(storymaker.PhaseFill, compact, names), storymaker.ToolStory)
		beat := func(path ...string) map[string]interface{} {
			return storymakerSchemaAt(story, append([]string{"data", "beats", "[]"}, path...)...)
		}
		paths := [][]string{
			{"icon"}, {"chart"}, {"data", "labels"}, {"data", "values"}, {"data", "unit"},
			{"diagram", "layout"}, {"diagram", "nodes", "[]", "id"}, {"diagram", "nodes", "[]", "label"},
			{"diagram", "nodes", "[]", "icon"}, {"diagram", "nodes", "[]", "cue_phrase"},
		}
		if !compact {
			// the compact schema stays at 900 tokens: no chart title, no compare points, no flow switch, no required lists
			paths = append(paths, []string{"data", "title"}, []string{"diagram", "flow"}, []string{"diagram", "nodes", "[]", "points"})
		}
		for _, path := range paths {
			if beat(path...) == nil {
				t.Errorf("compact=%v beat lacks %s", compact, strings.Join(path, "."))
			}
		}
		if got, _ := beat("chart")["enum"].([]string); !slices.Equal(got, []string{"bar", "line", "pie", "counter"}) {
			t.Errorf("compact=%v chart enum = %v", compact, got)
		}
		if got, _ := beat("diagram", "layout")["enum"].([]string); !slices.Equal(got, []string{"flow", "cycle", "hub", "stack", "compare"}) {
			t.Errorf("compact=%v layout enum = %v", compact, got)
		}
		if entry, _ := beat("entry")["enum"].([]string); !slices.Contains(entry, "draw") {
			t.Errorf("compact=%v entry enum = %v", compact, entry)
		}
		if do, _ := beat("do")["enum"].([]string); slices.Contains(do, "chart") {
			t.Errorf("compact=%v do must not offer the legacy chart beat", compact)
		}
		if req, _ := beat("diagram")["required"].([]string); !compact && !slices.Equal(req, []string{"layout", "nodes"}) {
			t.Errorf("diagram required = %v", req)
		}
		if req, _ := beat("diagram", "nodes", "[]")["required"].([]string); !compact && !slices.Equal(req, []string{"id", "label"}) {
			t.Errorf("node required = %v", req)
		}
		if beat("diagram", "nodes", "[]", "cue_phrase")["maxLength"] != 60 {
			t.Errorf("compact=%v node cue_phrase maxLength = %v", compact, beat("diagram", "nodes", "[]", "cue_phrase")["maxLength"])
		}
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/agent -run '(?i)storymaker' -count=1`
Expected: TestStorymakerBeatSchemaHasBlocks: beat lacks icon, chart, data.*, diagram.* (all fields)

- [ ] **Step 3: Implement**

In `internal/agent/native_tools_storymaker.go`, replace:

```go
		"name":      storymakerTyped("string"),
		"character": character,
	}
	properties := map[string]interface{}{
		"title": storymakerTyped("string"),
		"cast":  storymakerArray(schema(member)),
		"style": schema(map[string]interface{}{
			"rhythm":   storymakerEnum("calm", "standard", "lively"),
			"auto_sfx": storymakerTyped("boolean"),
		}),
	}
```

with:

```go
		"name":      storymakerTyped("string"),
		"character": character,
	}
	rhythm := storymakerEnum("calm", "standard", "lively")
	if compact {
		rhythm = storymakerTyped("string")
	}
	properties := map[string]interface{}{
		"title": storymakerTyped("string"),
		"cast":  storymakerArray(schema(member)),
		"style": schema(map[string]interface{}{
			"rhythm":   rhythm,
			"auto_sfx": storymakerTyped("boolean"),
		}),
	}
```

In `internal/agent/native_tools_storymaker.go`, replace:

```go
		// Names remain in the phase packet and beat enums; avoid duplicating them.
		pose, expression = storymakerTyped("string"), storymakerTyped("string")
	}
	return map[string]interface{}{
		"kind":        storymakerEnum("storyboard", "code"),
		"source":      map[string]interface{}{"type": "string", "maxLength": storymaker.StudioCodeBytes, "description": "put_code only"},
		"environment": environment,
		"variant":     storymakerEnum("day", "night"),
		"narration":   storymakerTyped("string"),
		"speaker":     storymakerTyped("string"),
		"transition":  storymakerEnum("cut", "fade", "slide-left", "wipe"),
		"duration":    prop("string", "auto or seconds"),
		"camera": schema(map[string]interface{}{
			"framing": storymakerEnum("wide", "medium", "close"),
			"target":  storymakerTyped("string"),
		}),
		"actors": storymakerArray(schema(map[string]interface{}{
			"cast":       storymakerTyped("string"),
			"at":         storymakerTyped("string"),
			"pose":       pose,
			"facing":     storymakerEnum("left", "right", "auto"),
			"expression": expression,
			"scale":      storymakerTyped("number"),
		})),
```

with:

```go
		// Names remain in the phase packet and beat enums; avoid duplicating them.
		pose, expression = storymakerTyped("string"), storymakerTyped("string")
	}
	source := map[string]interface{}{"type": "string", "maxLength": storymaker.StudioCodeBytes}
	variant, transition, duration := storymakerEnum("day", "night"), storymakerEnum("cut", "fade", "slide-left", "wipe"), storymakerTyped("string")
	framing, facing, layer := storymakerEnum("wide", "medium", "close"), storymakerEnum("left", "right", "auto"), storymakerEnum("back", "front")
	kind := storymakerEnum("storyboard", "code")
	if compact {
		kind = storymakerTyped("string")
		// The compact schema has no descriptions and leaves short closed names to the validator, which lists the valid ones.
		variant, transition, framing, facing, layer = storymakerTyped("string"), storymakerTyped("string"), storymakerTyped("string"), storymakerTyped("string"), storymakerTyped("string")
	} else {
		source["description"] = "put_code only"
		duration = prop("string", "auto or seconds")
	}
	return map[string]interface{}{
		"kind":        kind,
		"source":      source,
		"environment": environment,
		"variant":     variant,
		"narration":   storymakerTyped("string"),
		"speaker":     storymakerTyped("string"),
		"transition":  transition,
		"duration":    duration,
		"camera": schema(map[string]interface{}{
			"framing": framing,
			"target":  storymakerTyped("string"),
		}),
		"actors": storymakerArray(schema(map[string]interface{}{
			"cast":       storymakerTyped("string"),
			"at":         storymakerTyped("string"),
			"pose":       pose,
			"facing":     facing,
			"expression": expression,
			"scale":      storymakerTyped("number"),
		})),
```

In `internal/agent/native_tools_storymaker.go`, replace:

```go
			"id":    storymakerTyped("string"),
			"item":  storymakerTyped("string"),
			"at":    storymakerTyped("string"),
			"layer": storymakerEnum("back", "front"),
			"scale": storymakerTyped("number"),
		})),
		"beats": storymakerArray(storymakerBeatSchema(compact, names)),
```

with:

```go
			"id":    storymakerTyped("string"),
			"item":  storymakerTyped("string"),
			"at":    storymakerTyped("string"),
			"layer": layer,
			"scale": storymakerTyped("number"),
		})),
		"beats": storymakerArray(storymakerBeatSchema(compact, names)),
```

In `internal/agent/native_tools_storymaker.go`, replace:

```go
}

func storymakerBeatSchema(compact bool, names StorymakerNames) map[string]interface{} {
	properties := map[string]interface{}{
		"do":         storymakerEnum("enter", "exit", "walk", "move", "pose", "gesture", "emote", "look", "say", "show", "hide", "highlight", "camera", "sfx", "wait"),
		"who":        storymakerTyped("string"),
```

with:

```go
}

func storymakerBeatSchema(compact bool, names StorymakerNames) map[string]interface{} {
	gesture, expression, ease := storymakerNames(names.Gestures), storymakerNames(names.Expressions), storymakerEnum("linear", "ease-in", "ease-out", "ease-in-out", "back-out", "bounce-out")
	if compact {
		// The names travel in the phase packet and the validator lists the closest ones; the compact schema keeps its 900 tokens.
		gesture, expression, ease = storymakerTyped("string"), storymakerTyped("string"), storymakerTyped("string")
	}
	op, with := storymakerEnum("zoom", "pan", "focus", "shake"), storymakerEnum("prev")
	cuePhrase := map[string]interface{}{"type": "string", "maxLength": 60}
	chartData := map[string]interface{}{
		"labels": storymakerArray(storymakerTyped("string")),
		"values": storymakerArray(storymakerTyped("number")),
		"unit":   storymakerTyped("string"),
	}
	node := map[string]interface{}{
		"id":         storymakerTyped("string"),
		"label":      storymakerTyped("string"),
		"icon":       storymakerTyped("string"),
		"cue_phrase": map[string]interface{}{"type": "string", "maxLength": 60},
	}
	diagramFields := map[string]interface{}{"layout": storymakerEnum("flow", "cycle", "hub", "stack", "compare")}
	var nodeRequired, diagramRequired []string
	if compact {
		op, with = storymakerTyped("string"), storymakerTyped("string")
	} else {
		cuePhrase["description"] = "narration words to cue on"
		chartData["title"] = storymakerTyped("string")
		node["points"] = storymakerArray(storymakerTyped("string"))
		diagramFields["flow"] = storymakerTyped("boolean")
		nodeRequired, diagramRequired = []string{"id", "label"}, []string{"layout", "nodes"}
	}
	diagramFields["nodes"] = storymakerArray(schema(node, nodeRequired...))
	diagram := schema(diagramFields, diagramRequired...)
	properties := map[string]interface{}{
		"do":         storymakerEnum("enter", "exit", "walk", "move", "pose", "gesture", "emote", "look", "say", "show", "hide", "highlight", "camera", "sfx", "wait"),
		"who":        storymakerTyped("string"),
```

In `internal/agent/native_tools_storymaker.go`, replace:

```go
		"to":         storymakerTyped("string"),
		"at":         storymakerTyped("string"),
		"pose":       storymakerNames(names.Poses),
		"gesture":    storymakerNames(names.Gestures),
		"expression": storymakerNames(names.Expressions),
		"text":       storymakerTyped("string"),
		"style":      storymakerTyped("string"),
		"item":       storymakerTyped("string"),
		"entry":      storymakerEnum("pop", "fade", "slide", "none"),
		"op":         storymakerEnum("zoom", "pan", "focus", "shake"),
		"amount":     storymakerTyped("number"),
		"seconds":    storymakerTyped("number"),
		"with":       storymakerEnum("prev"),
		"cue":        storymakerTyped("integer"),
		"cue_phrase": map[string]interface{}{"type": "string", "maxLength": 60, "description": "narration words to cue on"},
	}
	properties["count"] = storymakerTyped("integer")
	properties["id"] = storymakerTyped("string")
	properties["sound"] = storymakerTyped("string")
	properties["delay"] = storymakerTyped("number")
	properties["duration"] = storymakerTyped("number")
	properties["ease"] = storymakerEnum("linear", "ease-in", "ease-out", "ease-in-out", "back-out", "bounce-out")
	properties["scale"] = storymakerTyped("number")
	return schema(properties, "do")
}
```

with:

```go
		"to":         storymakerTyped("string"),
		"at":         storymakerTyped("string"),
		"pose":       storymakerNames(names.Poses),
		"gesture":    gesture,
		"expression": expression,
		"text":       storymakerTyped("string"),
		"style":      storymakerTyped("string"),
		"item":       storymakerTyped("string"),
		"icon":       storymakerTyped("string"),
		"chart":      storymakerEnum("bar", "line", "pie", "counter"),
		"data":       schema(chartData),
		"diagram":    diagram,
		"entry":      storymakerEnum("pop", "fade", "slide", "none", "draw"),
		"op":         op,
		"amount":     storymakerTyped("number"),
		"seconds":    storymakerTyped("number"),
		"with":       with,
		"cue":        storymakerTyped("integer"),
		"cue_phrase": cuePhrase,
	}
	properties["count"] = storymakerTyped("integer")
	properties["id"] = storymakerTyped("string")
	properties["sound"] = storymakerTyped("string")
	properties["delay"] = storymakerTyped("number")
	properties["duration"] = storymakerTyped("number")
	properties["ease"] = ease
	properties["scale"] = storymakerTyped("number")
	return schema(properties, "do")
}
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/agent -run '(?i)storymaker' -count=1`
Expected: PASS; compact story schema 800 (fill), 780 (check), 897 (revise) tokens, bound 900 kept

- [ ] **Step 5: Commit**

```bash
git add internal/agent/native_tools_storymaker.go internal/agent/native_tools_storymaker_test.go
git commit -m "storymaker: the agent's beat schema has the block fields"
```

### Task D9: the app knows the new codes

**Files:**
- Modify: `ui/desktop_storymaker_guards_test.go`
- Modify: `ui/js/desktop/apps/storymaker-agent.js`
- Modify: `ui/js/desktop/apps/storymaker-core.js`
- Modify: `ui/lang/storymaker/cs.json`
- Modify: `ui/lang/storymaker/da.json`
- Modify: `ui/lang/storymaker/de.json`
- Modify: `ui/lang/storymaker/el.json`
- Modify: `ui/lang/storymaker/en.json`
- Modify: `ui/lang/storymaker/es.json`
- Modify: `ui/lang/storymaker/fr.json`
- Modify: `ui/lang/storymaker/hi.json`
- Modify: `ui/lang/storymaker/it.json`
- Modify: `ui/lang/storymaker/ja.json`
- Modify: `ui/lang/storymaker/nl.json`
- Modify: `ui/lang/storymaker/no.json`
- Modify: `ui/lang/storymaker/pl.json`
- Modify: `ui/lang/storymaker/pt.json`
- Modify: `ui/lang/storymaker/sv.json`
- Modify: `ui/lang/storymaker/zh.json`

- [ ] **Step 1: Write the failing test**

In `ui/desktop_storymaker_guards_test.go`, replace:

```go
		"storymaker.export.phase_":  storymakerList(t, exporter, "PHASES"),
		"storymaker.export.source_": {"config", "system", "pinned"},
	}
	// The list of issue codes is the contract's: thirty-four codes, no more, no less.
	want := strings.Fields("invalid_json unknown_field schema_mismatch too_large limit_exceeded missing_field invalid_value duplicate_id unknown_name unknown_asset unknown_spot unknown_cast unknown_cue canonicalised not_supported_yet narration_missing code_error deadline layout_unverified line_unread dead_stretch first_frame_empty text_too_small fade_between_worlds entry_monotony transition_variety cue_late glyph_missing text_offframe text_unsafe text_contrast flash flash_unverified slow_hook")
	if strings.Join(families["storymaker.issue."], " ") != strings.Join(want, " ") {
		t.Errorf("ISSUE_CODES = %v, want the 34 codes of the contract", families["storymaker.issue."])
	}
	// A narrate job ends with these codes, so the app needs a sentence for each. Batch 2C adds more, so this is no exact list.
	jobCodes := " " + strings.Join(families["storymaker.job_error."], " ") + " "
```

with:

```go
		"storymaker.export.phase_":  storymakerList(t, exporter, "PHASES"),
		"storymaker.export.source_": {"config", "system", "pinned"},
	}
	// The list of issue codes is the contract's: thirty-eight codes, no more, no less.
	want := strings.Fields("invalid_json unknown_field schema_mismatch too_large limit_exceeded missing_field invalid_value duplicate_id unknown_name unknown_asset unknown_spot unknown_cast unknown_cue canonicalised not_supported_yet narration_missing code_error deadline layout_unverified line_unread dead_stretch first_frame_empty text_too_small fade_between_worlds entry_monotony transition_variety cue_late glyph_missing text_offframe text_unsafe text_contrast flash flash_unverified slow_hook block_too_small label_long no_blocks block_repeat")
	if strings.Join(families["storymaker.issue."], " ") != strings.Join(want, " ") {
		t.Errorf("ISSUE_CODES = %v, want the 38 codes of the contract", families["storymaker.issue."])
	}
	// A narrate job ends with these codes, so the app needs a sentence for each. Batch 2C adds more, so this is no exact list.
	jobCodes := " " + strings.Join(families["storymaker.job_error."], " ") + " "
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./ui -run '(?i)(TestStorymakerAppUsesOnlyKeysItHas|TestStorymakerLocalesCoverAllSixteenLanguages|TestStorymakerAppTranslationsAreTranslated)' -count=1`
Expected: ISSUE_CODES has 34 codes, want the 38 codes of the contract

- [ ] **Step 3: Implement**

In `ui/js/desktop/apps/storymaker-agent.js`, replace:

```js
        offscreen: 'storymaker.agent.check_layout_found',
        actor_prop_overlap: 'storymaker.agent.check_layout_found',
        scene_static: 'storymaker.agent.check_layout_found',
        environment_monotony: 'storymaker.agent.check_craft',
        framing_monotony: 'storymaker.agent.check_craft',
        gesture_monotony: 'storymaker.agent.check_craft',
```

with:

```js
        offscreen: 'storymaker.agent.check_layout_found',
        actor_prop_overlap: 'storymaker.agent.check_layout_found',
        scene_static: 'storymaker.agent.check_layout_found',
        block_too_small: 'storymaker.agent.check_layout_found',
        scene_fallback: 'storymaker.agent.check_scene_fallback',
        environment_monotony: 'storymaker.agent.check_craft',
        framing_monotony: 'storymaker.agent.check_craft',
        gesture_monotony: 'storymaker.agent.check_craft',
```

In `ui/js/desktop/apps/storymaker-core.js`, replace:

```js
        return t('storymaker.job_error.unknown');
    }

    const ISSUE_CODES = ['invalid_json', 'unknown_field', 'schema_mismatch', 'too_large', 'limit_exceeded', 'missing_field', 'invalid_value', 'duplicate_id', 'unknown_name', 'unknown_asset', 'unknown_spot', 'unknown_cast', 'unknown_cue', 'canonicalised', 'not_supported_yet', 'narration_missing', 'code_error', 'deadline', 'layout_unverified', 'line_unread', 'dead_stretch', 'first_frame_empty', 'text_too_small', 'fade_between_worlds', 'entry_monotony', 'transition_variety', 'cue_late', 'glyph_missing', 'text_offframe', 'text_unsafe', 'text_contrast', 'flash', 'flash_unverified', 'slow_hook'];

    function issueTitle(t, code) {
        if (ISSUE_CODES.indexOf(code) >= 0) return t('storymaker.issue.' + code);
```

with:

```js
        return t('storymaker.job_error.unknown');
    }

    const ISSUE_CODES = ['invalid_json', 'unknown_field', 'schema_mismatch', 'too_large', 'limit_exceeded', 'missing_field', 'invalid_value', 'duplicate_id', 'unknown_name', 'unknown_asset', 'unknown_spot', 'unknown_cast', 'unknown_cue', 'canonicalised', 'not_supported_yet', 'narration_missing', 'code_error', 'deadline', 'layout_unverified', 'line_unread', 'dead_stretch', 'first_frame_empty', 'text_too_small', 'fade_between_worlds', 'entry_monotony', 'transition_variety', 'cue_late', 'glyph_missing', 'text_offframe', 'text_unsafe', 'text_contrast', 'flash', 'flash_unverified', 'slow_hook', 'block_too_small', 'label_long', 'no_blocks', 'block_repeat'];

    function issueTitle(t, code) {
        if (ISSUE_CODES.indexOf(code) >= 0) return t('storymaker.issue.' + code);
```

In `ui/lang/storymaker/cs.json`, replace:

```json
  "storymaker.issue.flash": "Příliš rychlé blikání",
  "storymaker.issue.flash_unverified": "Blikání není zcela zkontrolováno",
  "storymaker.issue.slow_hook": "Pomalý začátek",
  "storymaker.issue.unknown": "Problém",
  "storymaker.code.title": "Kód",
  "storymaker.code.files": "Soubory",
```

with:

```json
  "storymaker.issue.flash": "Příliš rychlé blikání",
  "storymaker.issue.flash_unverified": "Blikání není zcela zkontrolováno",
  "storymaker.issue.slow_hook": "Pomalý začátek",
  "storymaker.issue.block_too_small": "Blok je příliš malý",
  "storymaker.issue.label_long": "Popisek je příliš dlouhý",
  "storymaker.issue.no_blocks": "Žádné vysvětlující bloky",
  "storymaker.issue.block_repeat": "Stejný blok třikrát",
  "storymaker.issue.unknown": "Problém",
  "storymaker.code.title": "Kód",
  "storymaker.code.files": "Soubory",
```

In `ui/lang/storymaker/cs.json`, replace:

```json
  "storymaker.agent.check_corrections_exhausted": "Scéně došly pokusy o opravu. Požádejte autora, ať ji dokončí.",
  "storymaker.agent.check_model_error": "Model přestal odpovídat. Požádejte autora, ať příběh dokončí.",
  "storymaker.agent.check_scene_unfilled": "Tato scéna ještě potřebuje práci. Požádejte autora, ať ji dokončí.",
  "storymaker.agent.check_compile_failed": "Náhled nedokázal tento film připravit. Požádejte autora o opravu, nebo se podívejte do seznamu problémů.",
  "storymaker.agent.check_over_target": "Film je delší, než jste chtěli. Požádejte autora, ať ho zkrátí.",
  "storymaker.agent.check_craft": "Film působí v jednom ohledu stále jednotvárně: prostředí, záběr, gesta nebo obsazení. Prohlédněte si ho a případně upravte scény.",
```

with:

```json
  "storymaker.agent.check_corrections_exhausted": "Scéně došly pokusy o opravu. Požádejte autora, ať ji dokončí.",
  "storymaker.agent.check_model_error": "Model přestal odpovídat. Požádejte autora, ať příběh dokončí.",
  "storymaker.agent.check_scene_unfilled": "Tato scéna ještě potřebuje práci. Požádejte autora, ať ji dokončí.",
  "storymaker.agent.check_scene_fallback": "Tato scéna dostala jednoduchou tabuli s nadpisem, protože ji autor nedokázal dokončit. Uprav příběh, aby dostala skutečný obraz.",
  "storymaker.agent.check_compile_failed": "Náhled nedokázal tento film připravit. Požádejte autora o opravu, nebo se podívejte do seznamu problémů.",
  "storymaker.agent.check_over_target": "Film je delší, než jste chtěli. Požádejte autora, ať ho zkrátí.",
  "storymaker.agent.check_craft": "Film působí v jednom ohledu stále jednotvárně: prostředí, záběr, gesta nebo obsazení. Prohlédněte si ho a případně upravte scény.",
```

In `ui/lang/storymaker/da.json`, replace:

```json
  "storymaker.issue.flash": "Blinker for hurtigt",
  "storymaker.issue.flash_unverified": "Blink er ikke helt kontrolleret",
  "storymaker.issue.slow_hook": "Langsom start",
  "storymaker.issue.unknown": "Problem",
  "storymaker.code.title": "Kode",
  "storymaker.code.files": "Filer",
```

with:

```json
  "storymaker.issue.flash": "Blinker for hurtigt",
  "storymaker.issue.flash_unverified": "Blink er ikke helt kontrolleret",
  "storymaker.issue.slow_hook": "Langsom start",
  "storymaker.issue.block_too_small": "Blokken er for lille",
  "storymaker.issue.label_long": "Etiketten er for lang",
  "storymaker.issue.no_blocks": "Ingen forklarende blokke",
  "storymaker.issue.block_repeat": "Samme blok tre gange",
  "storymaker.issue.unknown": "Problem",
  "storymaker.code.title": "Kode",
  "storymaker.code.files": "Filer",
```

In `ui/lang/storymaker/da.json`, replace:

```json
  "storymaker.agent.check_corrections_exhausted": "En scene har brugt alle rettelsesforsøg. Bed forfatteren om at gøre den færdig.",
  "storymaker.agent.check_model_error": "Modellen holdt op med at svare. Bed forfatteren om at gøre historien færdig.",
  "storymaker.agent.check_scene_unfilled": "Denne scene mangler stadig arbejde. Bed forfatteren om at gøre den færdig.",
  "storymaker.agent.check_compile_failed": "Forhåndsvisningen kunne ikke forberede filmen. Bed forfatteren om at rette den, eller se listen over problemer.",
  "storymaker.agent.check_over_target": "Filmen er længere, end du bad om. Bed forfatteren om at forkorte den.",
  "storymaker.agent.check_craft": "Filmen virker stadig ensartet på ét punkt: miljøer, kamera, gestus eller rollebesætning. Se den igennem og justér scener, hvis du vil.",
```

with:

```json
  "storymaker.agent.check_corrections_exhausted": "En scene har brugt alle rettelsesforsøg. Bed forfatteren om at gøre den færdig.",
  "storymaker.agent.check_model_error": "Modellen holdt op med at svare. Bed forfatteren om at gøre historien færdig.",
  "storymaker.agent.check_scene_unfilled": "Denne scene mangler stadig arbejde. Bed forfatteren om at gøre den færdig.",
  "storymaker.agent.check_scene_fallback": "Denne scene fik en enkel tavle med en overskrift, fordi forfatteren ikke kunne gøre den færdig. Gennemgå historien, så den får et rigtigt billede.",
  "storymaker.agent.check_compile_failed": "Forhåndsvisningen kunne ikke forberede filmen. Bed forfatteren om at rette den, eller se listen over problemer.",
  "storymaker.agent.check_over_target": "Filmen er længere, end du bad om. Bed forfatteren om at forkorte den.",
  "storymaker.agent.check_craft": "Filmen virker stadig ensartet på ét punkt: miljøer, kamera, gestus eller rollebesætning. Se den igennem og justér scener, hvis du vil.",
```

In `ui/lang/storymaker/de.json`, replace:

```json
  "storymaker.issue.flash": "Zu schnelles Blitzen",
  "storymaker.issue.flash_unverified": "Blitzen nicht vollständig geprüft",
  "storymaker.issue.slow_hook": "Langsamer Einstieg",
  "storymaker.issue.unknown": "Problem",
  "storymaker.code.title": "Code",
  "storymaker.code.files": "Dateien",
```

with:

```json
  "storymaker.issue.flash": "Zu schnelles Blitzen",
  "storymaker.issue.flash_unverified": "Blitzen nicht vollständig geprüft",
  "storymaker.issue.slow_hook": "Langsamer Einstieg",
  "storymaker.issue.block_too_small": "Baustein zu klein",
  "storymaker.issue.label_long": "Beschriftung zu lang",
  "storymaker.issue.no_blocks": "Keine Erklärbausteine",
  "storymaker.issue.block_repeat": "Dreimal derselbe Baustein",
  "storymaker.issue.unknown": "Problem",
  "storymaker.code.title": "Code",
  "storymaker.code.files": "Dateien",
```

In `ui/lang/storymaker/de.json`, replace:

```json
  "storymaker.agent.check_corrections_exhausted": "Eine Szene hat alle Korrekturversuche verbraucht. Bitte den Autor, sie fertigzustellen.",
  "storymaker.agent.check_model_error": "Das Modell hat aufgehört zu antworten. Bitte den Autor, die Geschichte fertigzustellen.",
  "storymaker.agent.check_scene_unfilled": "Diese Szene braucht noch Arbeit. Bitte den Autor, sie fertigzustellen.",
  "storymaker.agent.check_compile_failed": "Die Vorschau konnte diesen Film nicht vorbereiten. Bitte den Autor um eine Korrektur, oder sieh in die Problemliste.",
  "storymaker.agent.check_over_target": "Der Film ist länger, als du wolltest. Bitte den Autor, ihn zu kürzen.",
  "storymaker.agent.check_craft": "Der Film wirkt in einem Punkt noch gleichförmig: Umgebungen, Kamera, Gesten oder Besetzung. Sieh ihn dir an und passe Szenen an, wenn du magst.",
```

with:

```json
  "storymaker.agent.check_corrections_exhausted": "Eine Szene hat alle Korrekturversuche verbraucht. Bitte den Autor, sie fertigzustellen.",
  "storymaker.agent.check_model_error": "Das Modell hat aufgehört zu antworten. Bitte den Autor, die Geschichte fertigzustellen.",
  "storymaker.agent.check_scene_unfilled": "Diese Szene braucht noch Arbeit. Bitte den Autor, sie fertigzustellen.",
  "storymaker.agent.check_scene_fallback": "Diese Szene hat eine einfache Tafel mit einer Überschrift bekommen, weil der Autor sie nicht fertig bekam. Überarbeite die Geschichte, damit sie ein richtiges Bild bekommt.",
  "storymaker.agent.check_compile_failed": "Die Vorschau konnte diesen Film nicht vorbereiten. Bitte den Autor um eine Korrektur, oder sieh in die Problemliste.",
  "storymaker.agent.check_over_target": "Der Film ist länger, als du wolltest. Bitte den Autor, ihn zu kürzen.",
  "storymaker.agent.check_craft": "Der Film wirkt in einem Punkt noch gleichförmig: Umgebungen, Kamera, Gesten oder Besetzung. Sieh ihn dir an und passe Szenen an, wenn du magst.",
```

In `ui/lang/storymaker/el.json`, replace:

```json
  "storymaker.issue.flash": "Πολύ γρήγορες αναλαμπές",
  "storymaker.issue.flash_unverified": "Οι αναλαμπές δεν ελέγχθηκαν πλήρως",
  "storymaker.issue.slow_hook": "Αργή αρχή",
  "storymaker.issue.unknown": "Πρόβλημα",
  "storymaker.code.title": "Κώδικας",
  "storymaker.code.files": "Αρχεία",
```

with:

```json
  "storymaker.issue.flash": "Πολύ γρήγορες αναλαμπές",
  "storymaker.issue.flash_unverified": "Οι αναλαμπές δεν ελέγχθηκαν πλήρως",
  "storymaker.issue.slow_hook": "Αργή αρχή",
  "storymaker.issue.block_too_small": "Το μπλοκ είναι πολύ μικρό",
  "storymaker.issue.label_long": "Η ετικέτα είναι πολύ μεγάλη",
  "storymaker.issue.no_blocks": "Χωρίς επεξηγηματικά μπλοκ",
  "storymaker.issue.block_repeat": "Το ίδιο μπλοκ τρεις φορές",
  "storymaker.issue.unknown": "Πρόβλημα",
  "storymaker.code.title": "Κώδικας",
  "storymaker.code.files": "Αρχεία",
```

In `ui/lang/storymaker/el.json`, replace:

```json
  "storymaker.agent.check_corrections_exhausted": "Μια σκηνή εξάντλησε τις προσπάθειες διόρθωσης. Ζητήστε από τον συγγραφέα να την ολοκληρώσει.",
  "storymaker.agent.check_model_error": "Το μοντέλο σταμάτησε να απαντά. Ζητήστε από τον συγγραφέα να ολοκληρώσει την ιστορία.",
  "storymaker.agent.check_scene_unfilled": "Αυτή η σκηνή χρειάζεται ακόμη δουλειά. Ζητήστε από τον συγγραφέα να την ολοκληρώσει.",
  "storymaker.agent.check_compile_failed": "Η προεπισκόπηση δεν μπόρεσε να προετοιμάσει την ταινία. Ζητήστε διόρθωση από τον συγγραφέα ή δείτε τη λίστα προβλημάτων.",
  "storymaker.agent.check_over_target": "Η ταινία είναι μεγαλύτερη από όσο ζητήσατε. Ζητήστε από τον συγγραφέα να τη συντομεύσει.",
  "storymaker.agent.check_craft": "Η ταινία φαίνεται ακόμη μονότονη σε ένα σημείο: χώροι, κάδρο, χειρονομίες ή διανομή. Δείτε την μία φορά και προσαρμόστε σκηνές αν θέλετε.",
```

with:

```json
  "storymaker.agent.check_corrections_exhausted": "Μια σκηνή εξάντλησε τις προσπάθειες διόρθωσης. Ζητήστε από τον συγγραφέα να την ολοκληρώσει.",
  "storymaker.agent.check_model_error": "Το μοντέλο σταμάτησε να απαντά. Ζητήστε από τον συγγραφέα να ολοκληρώσει την ιστορία.",
  "storymaker.agent.check_scene_unfilled": "Αυτή η σκηνή χρειάζεται ακόμη δουλειά. Ζητήστε από τον συγγραφέα να την ολοκληρώσει.",
  "storymaker.agent.check_scene_fallback": "Αυτή η σκηνή πήρε έναν απλό πίνακα με τίτλο, επειδή ο συγγραφέας δεν κατάφερε να την ολοκληρώσει. Αναθεώρησε την ιστορία για να αποκτήσει πραγματική εικόνα.",
  "storymaker.agent.check_compile_failed": "Η προεπισκόπηση δεν μπόρεσε να προετοιμάσει την ταινία. Ζητήστε διόρθωση από τον συγγραφέα ή δείτε τη λίστα προβλημάτων.",
  "storymaker.agent.check_over_target": "Η ταινία είναι μεγαλύτερη από όσο ζητήσατε. Ζητήστε από τον συγγραφέα να τη συντομεύσει.",
  "storymaker.agent.check_craft": "Η ταινία φαίνεται ακόμη μονότονη σε ένα σημείο: χώροι, κάδρο, χειρονομίες ή διανομή. Δείτε την μία φορά και προσαρμόστε σκηνές αν θέλετε.",
```

In `ui/lang/storymaker/en.json`, replace:

```json
  "storymaker.issue.flash": "Flashing too fast",
  "storymaker.issue.flash_unverified": "Flashing not fully checked",
  "storymaker.issue.slow_hook": "Slow start",
  "storymaker.issue.unknown": "Issue",
  "storymaker.code.title": "Code",
  "storymaker.code.files": "Files",
```

with:

```json
  "storymaker.issue.flash": "Flashing too fast",
  "storymaker.issue.flash_unverified": "Flashing not fully checked",
  "storymaker.issue.slow_hook": "Slow start",
  "storymaker.issue.block_too_small": "Block too small",
  "storymaker.issue.label_long": "Label too long",
  "storymaker.issue.no_blocks": "No explainer blocks",
  "storymaker.issue.block_repeat": "Same block three times",
  "storymaker.issue.unknown": "Issue",
  "storymaker.code.title": "Code",
  "storymaker.code.files": "Files",
```

In `ui/lang/storymaker/en.json`, replace:

```json
  "storymaker.agent.check_corrections_exhausted": "A scene ran out of correction attempts. Ask the author to finish it.",
  "storymaker.agent.check_model_error": "The model stopped answering. Ask the author to finish the story.",
  "storymaker.agent.check_scene_unfilled": "This scene still needs work. Ask the author to finish it.",
  "storymaker.agent.check_compile_failed": "The preview could not prepare this film. Ask the author to fix it, or check the issues list.",
  "storymaker.agent.check_over_target": "The film is longer than you asked for. Ask the author to shorten it.",
  "storymaker.agent.check_craft": "The film still looks alike in one respect: environments, framing, gestures or cast. Watch it once and adjust scenes if you like.",
```

with:

```json
  "storymaker.agent.check_corrections_exhausted": "A scene ran out of correction attempts. Ask the author to finish it.",
  "storymaker.agent.check_model_error": "The model stopped answering. Ask the author to finish the story.",
  "storymaker.agent.check_scene_unfilled": "This scene still needs work. Ask the author to finish it.",
  "storymaker.agent.check_scene_fallback": "This scene got a plain board with a headline because the author could not finish it. Revise the story to give it a real picture.",
  "storymaker.agent.check_compile_failed": "The preview could not prepare this film. Ask the author to fix it, or check the issues list.",
  "storymaker.agent.check_over_target": "The film is longer than you asked for. Ask the author to shorten it.",
  "storymaker.agent.check_craft": "The film still looks alike in one respect: environments, framing, gestures or cast. Watch it once and adjust scenes if you like.",
```

In `ui/lang/storymaker/es.json`, replace:

```json
  "storymaker.issue.flash": "Destellos demasiado rápidos",
  "storymaker.issue.flash_unverified": "Destellos no comprobados del todo",
  "storymaker.issue.slow_hook": "Inicio lento",
  "storymaker.issue.unknown": "Problema",
  "storymaker.code.title": "Código",
  "storymaker.code.files": "Archivos",
```

with:

```json
  "storymaker.issue.flash": "Destellos demasiado rápidos",
  "storymaker.issue.flash_unverified": "Destellos no comprobados del todo",
  "storymaker.issue.slow_hook": "Inicio lento",
  "storymaker.issue.block_too_small": "Bloque demasiado pequeño",
  "storymaker.issue.label_long": "Etiqueta demasiado larga",
  "storymaker.issue.no_blocks": "Sin bloques explicativos",
  "storymaker.issue.block_repeat": "El mismo bloque tres veces",
  "storymaker.issue.unknown": "Problema",
  "storymaker.code.title": "Código",
  "storymaker.code.files": "Archivos",
```

In `ui/lang/storymaker/es.json`, replace:

```json
  "storymaker.agent.check_corrections_exhausted": "Una escena agotó sus intentos de corrección. Pide al autor que la termine.",
  "storymaker.agent.check_model_error": "El modelo dejó de responder. Pide al autor que termine la historia.",
  "storymaker.agent.check_scene_unfilled": "Esta escena aún necesita trabajo. Pide al autor que la termine.",
  "storymaker.agent.check_compile_failed": "La vista previa no pudo preparar esta película. Pide al autor que la corrija o mira la lista de problemas.",
  "storymaker.agent.check_over_target": "La película es más larga de lo que pediste. Pide al autor que la acorte.",
  "storymaker.agent.check_craft": "La película aún resulta uniforme en un aspecto: entornos, encuadre, gestos o reparto. Míralo una vez y ajusta escenas si quieres.",
```

with:

```json
  "storymaker.agent.check_corrections_exhausted": "Una escena agotó sus intentos de corrección. Pide al autor que la termine.",
  "storymaker.agent.check_model_error": "El modelo dejó de responder. Pide al autor que termine la historia.",
  "storymaker.agent.check_scene_unfilled": "Esta escena aún necesita trabajo. Pide al autor que la termine.",
  "storymaker.agent.check_scene_fallback": "Esta escena recibió una pizarra sencilla con un titular porque el autor no pudo terminarla. Revisa la historia para darle una imagen de verdad.",
  "storymaker.agent.check_compile_failed": "La vista previa no pudo preparar esta película. Pide al autor que la corrija o mira la lista de problemas.",
  "storymaker.agent.check_over_target": "La película es más larga de lo que pediste. Pide al autor que la acorte.",
  "storymaker.agent.check_craft": "La película aún resulta uniforme en un aspecto: entornos, encuadre, gestos o reparto. Míralo una vez y ajusta escenas si quieres.",
```

In `ui/lang/storymaker/fr.json`, replace:

```json
  "storymaker.issue.flash": "Clignotement trop rapide",
  "storymaker.issue.flash_unverified": "Clignotement pas entièrement vérifié",
  "storymaker.issue.slow_hook": "Début trop lent",
  "storymaker.issue.unknown": "Problème",
  "storymaker.code.title": "Code",
  "storymaker.code.files": "Fichiers",
```

with:

```json
  "storymaker.issue.flash": "Clignotement trop rapide",
  "storymaker.issue.flash_unverified": "Clignotement pas entièrement vérifié",
  "storymaker.issue.slow_hook": "Début trop lent",
  "storymaker.issue.block_too_small": "Bloc trop petit",
  "storymaker.issue.label_long": "Libellé trop long",
  "storymaker.issue.no_blocks": "Aucun bloc explicatif",
  "storymaker.issue.block_repeat": "Le même bloc trois fois",
  "storymaker.issue.unknown": "Problème",
  "storymaker.code.title": "Code",
  "storymaker.code.files": "Fichiers",
```

In `ui/lang/storymaker/fr.json`, replace:

```json
  "storymaker.agent.check_corrections_exhausted": "Une scène a épuisé ses tentatives de correction. Demandez à l'auteur de la terminer.",
  "storymaker.agent.check_model_error": "Le modèle a cessé de répondre. Demandez à l'auteur de terminer l'histoire.",
  "storymaker.agent.check_scene_unfilled": "Cette scène demande encore du travail. Demandez à l'auteur de la terminer.",
  "storymaker.agent.check_compile_failed": "L'aperçu n'a pas pu préparer ce film. Demandez une correction à l'auteur, ou consultez la liste des problèmes.",
  "storymaker.agent.check_over_target": "Le film est plus long que demandé. Demandez à l'auteur de le raccourcir.",
  "storymaker.agent.check_craft": "Le film paraît encore uniforme sur un point : décors, cadrage, gestes ou distribution. Regardez-le une fois et ajustez des scènes si vous le souhaitez.",
```

with:

```json
  "storymaker.agent.check_corrections_exhausted": "Une scène a épuisé ses tentatives de correction. Demandez à l'auteur de la terminer.",
  "storymaker.agent.check_model_error": "Le modèle a cessé de répondre. Demandez à l'auteur de terminer l'histoire.",
  "storymaker.agent.check_scene_unfilled": "Cette scène demande encore du travail. Demandez à l'auteur de la terminer.",
  "storymaker.agent.check_scene_fallback": "Cette scène a reçu un tableau simple avec un titre, car l'auteur n'a pas pu la terminer. Révisez l'histoire pour lui donner une vraie image.",
  "storymaker.agent.check_compile_failed": "L'aperçu n'a pas pu préparer ce film. Demandez une correction à l'auteur, ou consultez la liste des problèmes.",
  "storymaker.agent.check_over_target": "Le film est plus long que demandé. Demandez à l'auteur de le raccourcir.",
  "storymaker.agent.check_craft": "Le film paraît encore uniforme sur un point : décors, cadrage, gestes ou distribution. Regardez-le une fois et ajustez des scènes si vous le souhaitez.",
```

In `ui/lang/storymaker/hi.json`, replace:

```json
  "storymaker.issue.flash": "बहुत तेज़ चमक",
  "storymaker.issue.flash_unverified": "चमक की पूरी जाँच नहीं हुई",
  "storymaker.issue.slow_hook": "धीमी शुरुआत",
  "storymaker.issue.unknown": "समस्या",
  "storymaker.code.title": "कोड",
  "storymaker.code.files": "फ़ाइलें",
```

with:

```json
  "storymaker.issue.flash": "बहुत तेज़ चमक",
  "storymaker.issue.flash_unverified": "चमक की पूरी जाँच नहीं हुई",
  "storymaker.issue.slow_hook": "धीमी शुरुआत",
  "storymaker.issue.block_too_small": "ब्लॉक बहुत छोटा है",
  "storymaker.issue.label_long": "लेबल बहुत लंबा है",
  "storymaker.issue.no_blocks": "कोई समझाने वाला ब्लॉक नहीं",
  "storymaker.issue.block_repeat": "एक ही ब्लॉक तीन बार",
  "storymaker.issue.unknown": "समस्या",
  "storymaker.code.title": "कोड",
  "storymaker.code.files": "फ़ाइलें",
```

In `ui/lang/storymaker/hi.json`, replace:

```json
  "storymaker.agent.check_corrections_exhausted": "एक दृश्य के सुधार प्रयास समाप्त हो गए। लेखक से इसे पूरा करने को कहें।",
  "storymaker.agent.check_model_error": "मॉडल ने जवाब देना बंद कर दिया। लेखक से कहानी पूरी करने को कहें।",
  "storymaker.agent.check_scene_unfilled": "इस दृश्य पर अभी काम बाकी है। लेखक से इसे पूरा करने को कहें।",
  "storymaker.agent.check_compile_failed": "पूर्वावलोकन इस फ़िल्म को तैयार नहीं कर सका। लेखक से इसे ठीक करने को कहें, या समस्याओं की सूची देखें।",
  "storymaker.agent.check_over_target": "फ़िल्म आपके अनुरोध से लंबी है। लेखक से इसे छोटा करने को कहें।",
  "storymaker.agent.check_craft": "फ़िल्म अब भी एक बात में एक जैसी लगती है: माहौल, फ़्रेमिंग, हाव-भाव या किरदार। इसे एक बार देखें और चाहें तो दृश्य बदलें।",
```

with:

```json
  "storymaker.agent.check_corrections_exhausted": "एक दृश्य के सुधार प्रयास समाप्त हो गए। लेखक से इसे पूरा करने को कहें।",
  "storymaker.agent.check_model_error": "मॉडल ने जवाब देना बंद कर दिया। लेखक से कहानी पूरी करने को कहें।",
  "storymaker.agent.check_scene_unfilled": "इस दृश्य पर अभी काम बाकी है। लेखक से इसे पूरा करने को कहें।",
  "storymaker.agent.check_scene_fallback": "लेखक इस दृश्य को पूरा नहीं कर सका, इसलिए इसे शीर्षक वाला एक साधारण बोर्ड मिला। कहानी को संशोधित करें ताकि इसे असली तस्वीर मिले।",
  "storymaker.agent.check_compile_failed": "पूर्वावलोकन इस फ़िल्म को तैयार नहीं कर सका। लेखक से इसे ठीक करने को कहें, या समस्याओं की सूची देखें।",
  "storymaker.agent.check_over_target": "फ़िल्म आपके अनुरोध से लंबी है। लेखक से इसे छोटा करने को कहें।",
  "storymaker.agent.check_craft": "फ़िल्म अब भी एक बात में एक जैसी लगती है: माहौल, फ़्रेमिंग, हाव-भाव या किरदार। इसे एक बार देखें और चाहें तो दृश्य बदलें।",
```

In `ui/lang/storymaker/it.json`, replace:

```json
  "storymaker.issue.flash": "Lampeggio troppo rapido",
  "storymaker.issue.flash_unverified": "Lampeggio non verificato del tutto",
  "storymaker.issue.slow_hook": "Inizio lento",
  "storymaker.issue.unknown": "Problema",
  "storymaker.code.title": "Codice",
  "storymaker.code.files": "File",
```

with:

```json
  "storymaker.issue.flash": "Lampeggio troppo rapido",
  "storymaker.issue.flash_unverified": "Lampeggio non verificato del tutto",
  "storymaker.issue.slow_hook": "Inizio lento",
  "storymaker.issue.block_too_small": "Blocco troppo piccolo",
  "storymaker.issue.label_long": "Etichetta troppo lunga",
  "storymaker.issue.no_blocks": "Nessun blocco esplicativo",
  "storymaker.issue.block_repeat": "Lo stesso blocco tre volte",
  "storymaker.issue.unknown": "Problema",
  "storymaker.code.title": "Codice",
  "storymaker.code.files": "File",
```

In `ui/lang/storymaker/it.json`, replace:

```json
  "storymaker.agent.check_corrections_exhausted": "Una scena ha esaurito i tentativi di correzione. Chiedi all'autore di completarla.",
  "storymaker.agent.check_model_error": "Il modello ha smesso di rispondere. Chiedi all'autore di completare la storia.",
  "storymaker.agent.check_scene_unfilled": "Questa scena richiede ancora lavoro. Chiedi all'autore di completarla.",
  "storymaker.agent.check_compile_failed": "L'anteprima non è riuscita a preparare questo film. Chiedi una correzione all'autore, oppure guarda l'elenco dei problemi.",
  "storymaker.agent.check_over_target": "Il film è più lungo di quanto richiesto. Chiedi all'autore di accorciarlo.",
  "storymaker.agent.check_craft": "Il film appare ancora uniforme sotto un aspetto: ambienti, inquadratura, gesti o cast. Guardalo una volta e modifica le scene se vuoi.",
```

with:

```json
  "storymaker.agent.check_corrections_exhausted": "Una scena ha esaurito i tentativi di correzione. Chiedi all'autore di completarla.",
  "storymaker.agent.check_model_error": "Il modello ha smesso di rispondere. Chiedi all'autore di completare la storia.",
  "storymaker.agent.check_scene_unfilled": "Questa scena richiede ancora lavoro. Chiedi all'autore di completarla.",
  "storymaker.agent.check_scene_fallback": "Questa scena ha ricevuto una lavagna semplice con un titolo perché l'autore non è riuscito a completarla. Rivedi la storia per darle un'immagine vera.",
  "storymaker.agent.check_compile_failed": "L'anteprima non è riuscita a preparare questo film. Chiedi una correzione all'autore, oppure guarda l'elenco dei problemi.",
  "storymaker.agent.check_over_target": "Il film è più lungo di quanto richiesto. Chiedi all'autore di accorciarlo.",
  "storymaker.agent.check_craft": "Il film appare ancora uniforme sotto un aspetto: ambienti, inquadratura, gesti o cast. Guardalo una volta e modifica le scene se vuoi.",
```

In `ui/lang/storymaker/ja.json`, replace:

```json
  "storymaker.issue.flash": "点滅が速すぎます",
  "storymaker.issue.flash_unverified": "点滅を完全には確認できていません",
  "storymaker.issue.slow_hook": "出だしが遅い",
  "storymaker.issue.unknown": "問題",
  "storymaker.code.title": "コード",
  "storymaker.code.files": "ファイル",
```

with:

```json
  "storymaker.issue.flash": "点滅が速すぎます",
  "storymaker.issue.flash_unverified": "点滅を完全には確認できていません",
  "storymaker.issue.slow_hook": "出だしが遅い",
  "storymaker.issue.block_too_small": "ブロックが小さすぎます",
  "storymaker.issue.label_long": "ラベルが長すぎます",
  "storymaker.issue.no_blocks": "解説ブロックがありません",
  "storymaker.issue.block_repeat": "同じブロックが3回続いています",
  "storymaker.issue.unknown": "問題",
  "storymaker.code.title": "コード",
  "storymaker.code.files": "ファイル",
```

In `ui/lang/storymaker/ja.json`, replace:

```json
  "storymaker.agent.check_corrections_exhausted": "シーンの修正回数が上限に達しました。作家に仕上げを依頼してください。",
  "storymaker.agent.check_model_error": "モデルが応答しなくなりました。作家にストーリーの仕上げを依頼してください。",
  "storymaker.agent.check_scene_unfilled": "このシーンにはまだ作業が必要です。作家に仕上げを依頼してください。",
  "storymaker.agent.check_compile_failed": "プレビューでこの映像を準備できませんでした。作家に修正を依頼するか、問題の一覧を確認してください。",
  "storymaker.agent.check_over_target": "映像が希望より長くなっています。作家に短縮を依頼してください。",
  "storymaker.agent.check_craft": "動画はまだ、環境・カメラ・ジェスチャー・キャストのいずれかで単調に見えます。一度ご覧になり、必要ならシーンを調整してください。",
```

with:

```json
  "storymaker.agent.check_corrections_exhausted": "シーンの修正回数が上限に達しました。作家に仕上げを依頼してください。",
  "storymaker.agent.check_model_error": "モデルが応答しなくなりました。作家にストーリーの仕上げを依頼してください。",
  "storymaker.agent.check_scene_unfilled": "このシーンにはまだ作業が必要です。作家に仕上げを依頼してください。",
  "storymaker.agent.check_scene_fallback": "作成者がこのシーンを完成できなかったため、見出し付きのシンプルなボードが入りました。本物の絵になるようストーリーを見直してください。",
  "storymaker.agent.check_compile_failed": "プレビューでこの映像を準備できませんでした。作家に修正を依頼するか、問題の一覧を確認してください。",
  "storymaker.agent.check_over_target": "映像が希望より長くなっています。作家に短縮を依頼してください。",
  "storymaker.agent.check_craft": "動画はまだ、環境・カメラ・ジェスチャー・キャストのいずれかで単調に見えます。一度ご覧になり、必要ならシーンを調整してください。",
```

In `ui/lang/storymaker/nl.json`, replace:

```json
  "storymaker.issue.flash": "Flitsen te snel",
  "storymaker.issue.flash_unverified": "Flitsen niet volledig gecontroleerd",
  "storymaker.issue.slow_hook": "Trage start",
  "storymaker.issue.unknown": "Probleem",
  "storymaker.code.title": "Code",
  "storymaker.code.files": "Bestanden",
```

with:

```json
  "storymaker.issue.flash": "Flitsen te snel",
  "storymaker.issue.flash_unverified": "Flitsen niet volledig gecontroleerd",
  "storymaker.issue.slow_hook": "Trage start",
  "storymaker.issue.block_too_small": "Blok te klein",
  "storymaker.issue.label_long": "Label te lang",
  "storymaker.issue.no_blocks": "Geen uitlegblokken",
  "storymaker.issue.block_repeat": "Hetzelfde blok drie keer",
  "storymaker.issue.unknown": "Probleem",
  "storymaker.code.title": "Code",
  "storymaker.code.files": "Bestanden",
```

In `ui/lang/storymaker/nl.json`, replace:

```json
  "storymaker.agent.check_corrections_exhausted": "Een scène heeft geen correctiepogingen meer. Vraag de auteur om hem af te maken.",
  "storymaker.agent.check_model_error": "Het model antwoordt niet meer. Vraag de auteur om het verhaal af te maken.",
  "storymaker.agent.check_scene_unfilled": "Deze scène heeft nog werk nodig. Vraag de auteur om hem af te maken.",
  "storymaker.agent.check_compile_failed": "De preview kon deze film niet voorbereiden. Vraag de auteur om een correctie, of bekijk de lijst met problemen.",
  "storymaker.agent.check_over_target": "De film is langer dan je vroeg. Vraag de auteur om hem in te korten.",
  "storymaker.agent.check_craft": "De film oogt nog op één punt eentonig: omgevingen, camera, gebaren of cast. Bekijk hem eenmaal en pas scènes aan als je wilt.",
```

with:

```json
  "storymaker.agent.check_corrections_exhausted": "Een scène heeft geen correctiepogingen meer. Vraag de auteur om hem af te maken.",
  "storymaker.agent.check_model_error": "Het model antwoordt niet meer. Vraag de auteur om het verhaal af te maken.",
  "storymaker.agent.check_scene_unfilled": "Deze scène heeft nog werk nodig. Vraag de auteur om hem af te maken.",
  "storymaker.agent.check_scene_fallback": "Deze scène kreeg een eenvoudig bord met een kop omdat de auteur hem niet kon afmaken. Herzie het verhaal om er een echt beeld aan te geven.",
  "storymaker.agent.check_compile_failed": "De preview kon deze film niet voorbereiden. Vraag de auteur om een correctie, of bekijk de lijst met problemen.",
  "storymaker.agent.check_over_target": "De film is langer dan je vroeg. Vraag de auteur om hem in te korten.",
  "storymaker.agent.check_craft": "De film oogt nog op één punt eentonig: omgevingen, camera, gebaren of cast. Bekijk hem eenmaal en pas scènes aan als je wilt.",
```

In `ui/lang/storymaker/no.json`, replace:

```json
  "storymaker.issue.flash": "Blinker for raskt",
  "storymaker.issue.flash_unverified": "Blinking er ikke helt kontrollert",
  "storymaker.issue.slow_hook": "Treg start",
  "storymaker.issue.unknown": "Problem",
  "storymaker.code.title": "Kode",
  "storymaker.code.files": "Filer",
```

with:

```json
  "storymaker.issue.flash": "Blinker for raskt",
  "storymaker.issue.flash_unverified": "Blinking er ikke helt kontrollert",
  "storymaker.issue.slow_hook": "Treg start",
  "storymaker.issue.block_too_small": "Blokken er for liten",
  "storymaker.issue.label_long": "Etiketten er for lang",
  "storymaker.issue.no_blocks": "Ingen forklarende blokker",
  "storymaker.issue.block_repeat": "Samme blokk tre ganger",
  "storymaker.issue.unknown": "Problem",
  "storymaker.code.title": "Kode",
  "storymaker.code.files": "Filer",
```

In `ui/lang/storymaker/no.json`, replace:

```json
  "storymaker.agent.check_corrections_exhausted": "En scene har brukt opp korrigeringsforsøkene. Be forfatteren om å fullføre den.",
  "storymaker.agent.check_model_error": "Modellen sluttet å svare. Be forfatteren om å fullføre historien.",
  "storymaker.agent.check_scene_unfilled": "Denne scenen trenger fortsatt arbeid. Be forfatteren om å fullføre den.",
  "storymaker.agent.check_compile_failed": "Forhåndsvisningen kunne ikke forberede filmen. Be forfatteren om å rette den, eller se listen over problemer.",
  "storymaker.agent.check_over_target": "Filmen er lengre enn du ba om. Be forfatteren om å korte den ned.",
  "storymaker.agent.check_craft": "Filmen virker fortsatt ensartet på ett punkt: omgivelser, kamera, bevegelser eller rollebesetning. Se den én gang og juster scener om du vil.",
```

with:

```json
  "storymaker.agent.check_corrections_exhausted": "En scene har brukt opp korrigeringsforsøkene. Be forfatteren om å fullføre den.",
  "storymaker.agent.check_model_error": "Modellen sluttet å svare. Be forfatteren om å fullføre historien.",
  "storymaker.agent.check_scene_unfilled": "Denne scenen trenger fortsatt arbeid. Be forfatteren om å fullføre den.",
  "storymaker.agent.check_scene_fallback": "Denne scenen fikk en enkel tavle med en overskrift fordi forfatteren ikke fikk gjort den ferdig. Revider historien slik at den får et ekte bilde.",
  "storymaker.agent.check_compile_failed": "Forhåndsvisningen kunne ikke forberede filmen. Be forfatteren om å rette den, eller se listen over problemer.",
  "storymaker.agent.check_over_target": "Filmen er lengre enn du ba om. Be forfatteren om å korte den ned.",
  "storymaker.agent.check_craft": "Filmen virker fortsatt ensartet på ett punkt: omgivelser, kamera, bevegelser eller rollebesetning. Se den én gang og juster scener om du vil.",
```

In `ui/lang/storymaker/pl.json`, replace:

```json
  "storymaker.issue.flash": "Zbyt szybkie migotanie",
  "storymaker.issue.flash_unverified": "Migotanie nie w pełni sprawdzone",
  "storymaker.issue.slow_hook": "Wolny początek",
  "storymaker.issue.unknown": "Problem",
  "storymaker.code.title": "Kod",
  "storymaker.code.files": "Pliki",
```

with:

```json
  "storymaker.issue.flash": "Zbyt szybkie migotanie",
  "storymaker.issue.flash_unverified": "Migotanie nie w pełni sprawdzone",
  "storymaker.issue.slow_hook": "Wolny początek",
  "storymaker.issue.block_too_small": "Blok za mały",
  "storymaker.issue.label_long": "Etykieta za długa",
  "storymaker.issue.no_blocks": "Brak bloków objaśniających",
  "storymaker.issue.block_repeat": "Ten sam blok trzy razy",
  "storymaker.issue.unknown": "Problem",
  "storymaker.code.title": "Kod",
  "storymaker.code.files": "Pliki",
```

In `ui/lang/storymaker/pl.json`, replace:

```json
  "storymaker.agent.check_corrections_exhausted": "Scena wyczerpała próby poprawek. Poproś autora o jej dokończenie.",
  "storymaker.agent.check_model_error": "Model przestał odpowiadać. Poproś autora o dokończenie historii.",
  "storymaker.agent.check_scene_unfilled": "Ta scena wciąż wymaga pracy. Poproś autora o jej dokończenie.",
  "storymaker.agent.check_compile_failed": "Podgląd nie mógł przygotować tego filmu. Poproś autora o poprawkę albo sprawdź listę problemów.",
  "storymaker.agent.check_over_target": "Film jest dłuższy, niż prosiłeś. Poproś autora o skrócenie.",
  "storymaker.agent.check_craft": "Film nadal wygląda jednolicie pod jednym względem: otoczenie, kadr, gesty lub obsada. Obejrzyj go raz i w razie potrzeby popraw sceny.",
```

with:

```json
  "storymaker.agent.check_corrections_exhausted": "Scena wyczerpała próby poprawek. Poproś autora o jej dokończenie.",
  "storymaker.agent.check_model_error": "Model przestał odpowiadać. Poproś autora o dokończenie historii.",
  "storymaker.agent.check_scene_unfilled": "Ta scena wciąż wymaga pracy. Poproś autora o jej dokończenie.",
  "storymaker.agent.check_scene_fallback": "Ta scena dostała prostą tablicę z nagłówkiem, bo autor nie zdołał jej dokończyć. Zmień historię, aby nadać jej prawdziwy obraz.",
  "storymaker.agent.check_compile_failed": "Podgląd nie mógł przygotować tego filmu. Poproś autora o poprawkę albo sprawdź listę problemów.",
  "storymaker.agent.check_over_target": "Film jest dłuższy, niż prosiłeś. Poproś autora o skrócenie.",
  "storymaker.agent.check_craft": "Film nadal wygląda jednolicie pod jednym względem: otoczenie, kadr, gesty lub obsada. Obejrzyj go raz i w razie potrzeby popraw sceny.",
```

In `ui/lang/storymaker/pt.json`, replace:

```json
  "storymaker.issue.flash": "Piscar demasiado rápido",
  "storymaker.issue.flash_unverified": "Piscar não totalmente verificado",
  "storymaker.issue.slow_hook": "Início lento",
  "storymaker.issue.unknown": "Problema",
  "storymaker.code.title": "Código",
  "storymaker.code.files": "Ficheiros",
```

with:

```json
  "storymaker.issue.flash": "Piscar demasiado rápido",
  "storymaker.issue.flash_unverified": "Piscar não totalmente verificado",
  "storymaker.issue.slow_hook": "Início lento",
  "storymaker.issue.block_too_small": "Bloco pequeno demais",
  "storymaker.issue.label_long": "Rótulo longo demais",
  "storymaker.issue.no_blocks": "Sem blocos explicativos",
  "storymaker.issue.block_repeat": "O mesmo bloco três vezes",
  "storymaker.issue.unknown": "Problema",
  "storymaker.code.title": "Código",
  "storymaker.code.files": "Ficheiros",
```

In `ui/lang/storymaker/pt.json`, replace:

```json
  "storymaker.agent.check_corrections_exhausted": "Uma cena esgotou as tentativas de correção. Peça ao autor que a termine.",
  "storymaker.agent.check_model_error": "O modelo deixou de responder. Peça ao autor que termine a história.",
  "storymaker.agent.check_scene_unfilled": "Esta cena ainda precisa de trabalho. Peça ao autor que a termine.",
  "storymaker.agent.check_compile_failed": "A pré-visualização não conseguiu preparar este filme. Peça uma correção ao autor, ou veja a lista de problemas.",
  "storymaker.agent.check_over_target": "O filme é mais longo do que pediu. Peça ao autor que o encurte.",
  "storymaker.agent.check_craft": "O filme ainda parece uniforme num aspeto: cenários, enquadramento, gestos ou elenco. Veja-o uma vez e ajuste cenas se quiser.",
```

with:

```json
  "storymaker.agent.check_corrections_exhausted": "Uma cena esgotou as tentativas de correção. Peça ao autor que a termine.",
  "storymaker.agent.check_model_error": "O modelo deixou de responder. Peça ao autor que termine a história.",
  "storymaker.agent.check_scene_unfilled": "Esta cena ainda precisa de trabalho. Peça ao autor que a termine.",
  "storymaker.agent.check_scene_fallback": "Esta cena recebeu um quadro simples com um título porque o autor não conseguiu concluí-la. Revise a história para dar-lhe uma imagem de verdade.",
  "storymaker.agent.check_compile_failed": "A pré-visualização não conseguiu preparar este filme. Peça uma correção ao autor, ou veja a lista de problemas.",
  "storymaker.agent.check_over_target": "O filme é mais longo do que pediu. Peça ao autor que o encurte.",
  "storymaker.agent.check_craft": "O filme ainda parece uniforme num aspeto: cenários, enquadramento, gestos ou elenco. Veja-o uma vez e ajuste cenas se quiser.",
```

In `ui/lang/storymaker/sv.json`, replace:

```json
  "storymaker.issue.flash": "Blinkar för snabbt",
  "storymaker.issue.flash_unverified": "Blinkning inte helt kontrollerad",
  "storymaker.issue.slow_hook": "Långsam start",
  "storymaker.issue.unknown": "Problem",
  "storymaker.code.title": "Kod",
  "storymaker.code.files": "Filer",
```

with:

```json
  "storymaker.issue.flash": "Blinkar för snabbt",
  "storymaker.issue.flash_unverified": "Blinkning inte helt kontrollerad",
  "storymaker.issue.slow_hook": "Långsam start",
  "storymaker.issue.block_too_small": "Blocket är för litet",
  "storymaker.issue.label_long": "Etiketten är för lång",
  "storymaker.issue.no_blocks": "Inga förklarande block",
  "storymaker.issue.block_repeat": "Samma block tre gånger",
  "storymaker.issue.unknown": "Problem",
  "storymaker.code.title": "Kod",
  "storymaker.code.files": "Filer",
```

In `ui/lang/storymaker/sv.json`, replace:

```json
  "storymaker.agent.check_corrections_exhausted": "En scen har inga korrigeringsförsök kvar. Be författaren göra klart den.",
  "storymaker.agent.check_model_error": "Modellen slutade svara. Be författaren göra klart berättelsen.",
  "storymaker.agent.check_scene_unfilled": "Den här scenen behöver fortfarande arbete. Be författaren göra klart den.",
  "storymaker.agent.check_compile_failed": "Förhandsvisningen kunde inte förbereda filmen. Be författaren rätta den, eller se listan över problem.",
  "storymaker.agent.check_over_target": "Filmen är längre än du bad om. Be författaren korta den.",
  "storymaker.agent.check_craft": "Filmen känns fortfarande likartad på en punkt: miljöer, kamera, gester eller rollbesättning. Titta på den en gång och justera scener om du vill.",
```

with:

```json
  "storymaker.agent.check_corrections_exhausted": "En scen har inga korrigeringsförsök kvar. Be författaren göra klart den.",
  "storymaker.agent.check_model_error": "Modellen slutade svara. Be författaren göra klart berättelsen.",
  "storymaker.agent.check_scene_unfilled": "Den här scenen behöver fortfarande arbete. Be författaren göra klart den.",
  "storymaker.agent.check_scene_fallback": "Den här scenen fick en enkel tavla med en rubrik eftersom författaren inte kunde göra klart den. Revidera berättelsen så att den får en riktig bild.",
  "storymaker.agent.check_compile_failed": "Förhandsvisningen kunde inte förbereda filmen. Be författaren rätta den, eller se listan över problem.",
  "storymaker.agent.check_over_target": "Filmen är längre än du bad om. Be författaren korta den.",
  "storymaker.agent.check_craft": "Filmen känns fortfarande likartad på en punkt: miljöer, kamera, gester eller rollbesättning. Titta på den en gång och justera scener om du vill.",
```

In `ui/lang/storymaker/zh.json`, replace:

```json
  "storymaker.issue.flash": "闪烁过快",
  "storymaker.issue.flash_unverified": "闪烁未完全检查",
  "storymaker.issue.slow_hook": "开头太慢",
  "storymaker.issue.unknown": "问题",
  "storymaker.code.title": "代码",
  "storymaker.code.files": "文件",
```

with:

```json
  "storymaker.issue.flash": "闪烁过快",
  "storymaker.issue.flash_unverified": "闪烁未完全检查",
  "storymaker.issue.slow_hook": "开头太慢",
  "storymaker.issue.block_too_small": "模块太小",
  "storymaker.issue.label_long": "标签太长",
  "storymaker.issue.no_blocks": "没有讲解模块",
  "storymaker.issue.block_repeat": "同一模块连续出现三次",
  "storymaker.issue.unknown": "问题",
  "storymaker.code.title": "代码",
  "storymaker.code.files": "文件",
```

In `ui/lang/storymaker/zh.json`, replace:

```json
  "storymaker.agent.check_corrections_exhausted": "某个场景的修正次数已用完。请让作者完成它。",
  "storymaker.agent.check_model_error": "模型停止了响应。请让作者完成故事。",
  "storymaker.agent.check_scene_unfilled": "这个场景仍需处理。请让作者完成它。",
  "storymaker.agent.check_compile_failed": "预览无法准备这部影片。请让作者修复，或查看问题列表。",
  "storymaker.agent.check_over_target": "影片比你要求的长。请让作者缩短。",
  "storymaker.agent.check_craft": "影片在某一方面仍显单调：场景、镜头、手势或角色。请观看一次，如有需要再调整场景。",
```

with:

```json
  "storymaker.agent.check_corrections_exhausted": "某个场景的修正次数已用完。请让作者完成它。",
  "storymaker.agent.check_model_error": "模型停止了响应。请让作者完成故事。",
  "storymaker.agent.check_scene_unfilled": "这个场景仍需处理。请让作者完成它。",
  "storymaker.agent.check_scene_fallback": "作者未能完成这个场景，因此它得到了一块带标题的简单板。请修改故事，为它配上真正的画面。",
  "storymaker.agent.check_compile_failed": "预览无法准备这部影片。请让作者修复，或查看问题列表。",
  "storymaker.agent.check_over_target": "影片比你要求的长。请让作者缩短。",
  "storymaker.agent.check_craft": "影片在某一方面仍显单调：场景、镜头、手势或角色。请观看一次，如有需要再调整场景。",
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./ui -run '(?i)(TestStorymakerAppUsesOnlyKeysItHas|TestStorymakerLocalesCoverAllSixteenLanguages|TestStorymakerAppTranslationsAreTranslated)' -count=1`
Expected: PASS; go test ./ui -run '(?i)storymaker' ok; check:ui ok

- [ ] **Step 5: Commit**

```bash
git add ui/desktop_storymaker_guards_test.go ui/js/desktop/apps/storymaker-agent.js ui/js/desktop/apps/storymaker-core.js ui/lang/storymaker/cs.json ui/lang/storymaker/da.json ui/lang/storymaker/de.json ui/lang/storymaker/el.json ui/lang/storymaker/en.json ui/lang/storymaker/es.json ui/lang/storymaker/fr.json ui/lang/storymaker/hi.json ui/lang/storymaker/it.json ui/lang/storymaker/ja.json ui/lang/storymaker/nl.json ui/lang/storymaker/no.json ui/lang/storymaker/pl.json ui/lang/storymaker/pt.json ui/lang/storymaker/sv.json ui/lang/storymaker/zh.json
git commit -m "storymaker: the app knows the new codes"
```

### Task D10: Go refuses the blocks the player cannot draw; ids are counted alike

**Files:**
- Modify: `internal/storymaker/blocks_model_test.go`
- Modify: `internal/storymaker/defaults.go`
- Modify: `internal/storymaker/validate.go`
- Modify: `internal/storymaker/validate_blocks.go`
- Modify: `internal/storymaker/validate_blocks_test.go`
- Modify: `scripts/storymaker-tests/34-block-beats.mjs`
- Modify: `ui/js/storymaker/beats-blocks.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/blocks_model_test.go`, replace:

```go
		t.Fatalf("BlockAreas = %v", BlockAreas)
	}
}
```

with:

```go
		t.Fatalf("BlockAreas = %v", BlockAreas)
	}
}

// The player numbers the legacy chart beat like a show chart (beats-blocks.js: instanceId("chart")), so Go must too.
func TestDeriveShowIDsCountsLegacyCharts(t *testing.T) {
	data := `"data":{"labels":["a"],"values":[1]}`
	s := mustDecode(t, `{"schema":"storymaker/1","title":"B","format":{"aspect":"16:9","fps":30},"style":{"palette":"sunny","font":"default","captions":true},"cast":[],"scenes":[{"id":"s1","narration":"x","beats":[
 {"do":"chart","chart":"bar",`+data+`,"at":"right"},
 {"do":"show","chart":"bar",`+data+`,"at":"full","entry":"fade"},
 {"do":"chart","chart":"bar",`+data+`,"at":"left"}]}]}`)
	if got, want := deriveShowIDs(&s.Scenes[0]), []string{"chart-1", "chart-2", "chart-3"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("show ids = %v, want %v", got, want)
	}
}
```

In `internal/storymaker/validate_blocks_test.go`, replace:

```go
		`{"do":"show","text":"Kleine Modelle sind *schnell*","style":"headline","at":"top","entry":"fade"}`,
		`{"do":"show","text":"Laden\nRechnen\nAntworten","style":"list","at":"right","entry":"fade"}`,
		`{"do":"chart","chart":"bar","data":{"labels":["a"],"values":[1]},"at":"right"}`,
	}
	for _, beat := range valid {
		if issues := validateScenes(t, blockScene(beat)); len(issues) != 0 {
```

with:

```go
		`{"do":"show","text":"Kleine Modelle sind *schnell*","style":"headline","at":"top","entry":"fade"}`,
		`{"do":"show","text":"Laden\nRechnen\nAntworten","style":"list","at":"right","entry":"fade"}`,
		`{"do":"chart","chart":"bar","data":{"labels":["a"],"values":[1]},"at":"right"}`,
		// The limits themselves are legal, and a label next to an icon of any style is a card label.
		`{"do":"show","icon":"icon.cpu","style":"badge","text":"` + strings.Repeat("x", 40) + `","at":"center","entry":"pop"}`,
		`{"do":"show","text":"` + strings.Repeat("x", 40) + `\n` + strings.Repeat("y", 40) + `","style":"list","at":"right","entry":"fade"}`,
		`{"do":"show","chart":"bar","data":{"labels":["` + strings.Repeat("x", 24) + `"],"values":[1],"unit":"12345678","title":"` + strings.Repeat("t", 40) + `"},"at":"full","entry":"fade"}`,
		`{"do":"show","diagram":{"layout":"compare","nodes":[{"id":"a","label":"A","points":["` + strings.Repeat("p", 40) + `"]},{"id":"b","label":"B"}]},"at":"left","entry":"fade"}`,
	}
	for _, beat := range valid {
		if issues := validateScenes(t, blockScene(beat)); len(issues) != 0 {
```

In `internal/storymaker/validate_blocks_test.go`, replace:

```go
	}{
		{"unknown icon", `{"do":"show","icon":"warning","at":"center","entry":"pop"}`, CodeUnknownAsset, SeverityError, bp + "/icon"},
		{"icon style", `{"do":"show","icon":"icon.cpu","style":"big","at":"center","entry":"pop"}`, CodeInvalidValue, SeverityError, bp + "/style"},
		{"label without card", `{"do":"show","icon":"icon.cpu","style":"badge","text":"CPU","at":"center","entry":"pop"}`, CodeInvalidValue, SeverityWarning, bp + "/text"},
		{"icon and chart", `{"do":"show","icon":"icon.cpu","chart":"bar","data":{"labels":["a"],"values":[1]},"at":"full","entry":"fade"}`, CodeInvalidValue, SeverityError, bp + "/item"},
		{"headline too long", `{"do":"show","text":"` + long + `","style":"headline","at":"top","entry":"fade"}`, CodeLimitExceeded, SeverityError, bp + "/text"},
		{"headline stars", `{"do":"show","text":"a *b","style":"headline","at":"top","entry":"fade"}`, CodeInvalidValue, SeverityWarning, bp + "/text"},
		{"list of five", `{"do":"show","text":"a\nb\nc\nd\ne","style":"list","at":"right","entry":"fade"}`, CodeLimitExceeded, SeverityError, bp + "/text"},
		{"list line long", `{"do":"show","text":"` + long + `","style":"list","at":"right","entry":"fade"}`, CodeInvalidValue, SeverityWarning, bp + "/text"},
		{"chart type", chart("donut", `{"labels":["a"],"values":[1]}`), CodeInvalidValue, SeverityError, bp + "/chart"},
		{"chart without data", `{"do":"show","chart":"bar","at":"full","entry":"fade"}`, CodeMissingField, SeverityError, bp + "/data"},
		{"chart lengths", chart("bar", `{"labels":["a","b"],"values":[1]}`), CodeInvalidValue, SeverityError, bp + "/data"},
		{"chart count", chart("pie", `{"labels":["a"],"values":[1]}`), CodeLimitExceeded, SeverityError, bp + "/data"},
		{"pie negative", chart("pie", `{"labels":["a","b"],"values":[3,-1]}`), CodeInvalidValue, SeverityError, bp + "/data/values"},
		{"pie zero sum", chart("pie", `{"labels":["a","b"],"values":[0,0]}`), CodeInvalidValue, SeverityError, bp + "/data/values"},
		{"chart label", chart("bar", `{"labels":["`+long+`"],"values":[1]}`), CodeInvalidValue, SeverityWarning, bp + "/data/labels/0"},
		{"chart unit", chart("bar", `{"labels":["a"],"values":[1],"unit":"123456789"}`), CodeInvalidValue, SeverityWarning, bp + "/data/unit"},
		{"chart title", chart("bar", `{"labels":["a"],"values":[1],"title":"`+long+`"}`), CodeInvalidValue, SeverityWarning, bp + "/data/title"},
		{"legacy chart type", `{"do":"chart","chart":"donut","data":{"labels":["a"],"values":[1]},"at":"right"}`, CodeInvalidValue, SeverityError, bp + "/chart"},
		{"legacy chart data", `{"do":"chart","chart":"bar","at":"right"}`, CodeMissingField, SeverityError, bp + "/data"},
		{"diagram layout", node("ring", two), CodeInvalidValue, SeverityError, bp + "/diagram/layout"},
		{"diagram count", node("flow", `{"id":"a","label":"A"}`), CodeLimitExceeded, SeverityError, bp + "/diagram/nodes"},
		{"node slug", node("flow", `{"id":"A b","label":"A"},{"id":"b","label":"B"}`), CodeInvalidValue, SeverityError, bp + "/diagram/nodes/0/id"},
```

with:

```go
	}{
		{"unknown icon", `{"do":"show","icon":"warning","at":"center","entry":"pop"}`, CodeUnknownAsset, SeverityError, bp + "/icon"},
		{"icon style", `{"do":"show","icon":"icon.cpu","style":"big","at":"center","entry":"pop"}`, CodeInvalidValue, SeverityError, bp + "/style"},
		{"icon and chart", `{"do":"show","icon":"icon.cpu","chart":"bar","data":{"labels":["a"],"values":[1]},"at":"full","entry":"fade"}`, CodeInvalidValue, SeverityError, bp + "/item"},
		{"headline too long", `{"do":"show","text":"` + long + `","style":"headline","at":"top","entry":"fade"}`, CodeLimitExceeded, SeverityError, bp + "/text"},
		{"headline stars", `{"do":"show","text":"a *b","style":"headline","at":"top","entry":"fade"}`, CodeInvalidValue, SeverityWarning, bp + "/text"},
		{"list of five", `{"do":"show","text":"a\nb\nc\nd\ne","style":"list","at":"right","entry":"fade"}`, CodeLimitExceeded, SeverityError, bp + "/text"},
		{"list line long", `{"do":"show","text":"` + long + `","style":"list","at":"right","entry":"fade"}`, CodeLimitExceeded, SeverityError, bp + "/text"},
		{"chart type", chart("donut", `{"labels":["a"],"values":[1]}`), CodeInvalidValue, SeverityError, bp + "/chart"},
		{"chart without data", `{"do":"show","chart":"bar","at":"full","entry":"fade"}`, CodeMissingField, SeverityError, bp + "/data"},
		{"chart lengths", chart("bar", `{"labels":["a","b"],"values":[1]}`), CodeInvalidValue, SeverityError, bp + "/data"},
		{"chart count", chart("pie", `{"labels":["a"],"values":[1]}`), CodeLimitExceeded, SeverityError, bp + "/data"},
		{"pie negative", chart("pie", `{"labels":["a","b"],"values":[3,-1]}`), CodeInvalidValue, SeverityError, bp + "/data/values"},
		{"pie zero sum", chart("pie", `{"labels":["a","b"],"values":[0,0]}`), CodeInvalidValue, SeverityError, bp + "/data/values"},
		{"chart label", chart("bar", `{"labels":["`+long+`"],"values":[1]}`), CodeLimitExceeded, SeverityError, bp + "/data/labels/0"},
		{"chart unit", chart("bar", `{"labels":["a"],"values":[1],"unit":"123456789"}`), CodeLimitExceeded, SeverityError, bp + "/data/unit"},
		{"chart title", chart("bar", `{"labels":["a"],"values":[1],"title":"`+long+`"}`), CodeLimitExceeded, SeverityError, bp + "/data/title"},
		{"legacy chart type", `{"do":"chart","chart":"donut","data":{"labels":["a"],"values":[1]},"at":"right"}`, CodeInvalidValue, SeverityError, bp + "/chart"},
		{"legacy chart data", `{"do":"chart","chart":"bar","at":"right"}`, CodeMissingField, SeverityError, bp + "/data"},
		{"icon label long", `{"do":"show","icon":"icon.cpu","style":"card","text":"` + long + `","at":"center","entry":"pop"}`, CodeLimitExceeded, SeverityError, bp + "/text"},
		{"icon label long without style", `{"do":"show","icon":"icon.cpu","text":"` + long + `","at":"center","entry":"pop"}`, CodeLimitExceeded, SeverityError, bp + "/text"},
		{"headline without a word", `{"do":"show","text":"**","style":"headline","at":"top","entry":"fade"}`, CodeInvalidValue, SeverityError, bp + "/text"},
		{"list without a line", `{"do":"show","text":" \n ","style":"list","at":"right","entry":"fade"}`, CodeInvalidValue, SeverityError, bp + "/text"},
		{"point empty", node("compare", `{"id":"a","label":"A","points":[""]},{"id":"b","label":"B"}`), CodeLimitExceeded, SeverityError, bp + "/diagram/nodes/0/points/0"},
		{"point long", node("compare", `{"id":"a","label":"A","points":["`+long+`"]},{"id":"b","label":"B"}`), CodeLimitExceeded, SeverityError, bp + "/diagram/nodes/0/points/0"},
		{"point long on flow", node("flow", `{"id":"a","label":"A","points":["`+long+`"]},{"id":"b","label":"B"}`), CodeLimitExceeded, SeverityError, bp + "/diagram/nodes/0/points/0"},
		{"diagram layout", node("ring", two), CodeInvalidValue, SeverityError, bp + "/diagram/layout"},
		{"diagram count", node("flow", `{"id":"a","label":"A"}`), CodeLimitExceeded, SeverityError, bp + "/diagram/nodes"},
		{"node slug", node("flow", `{"id":"A b","label":"A"},{"id":"b","label":"B"}`), CodeInvalidValue, SeverityError, bp + "/diagram/nodes/0/id"},
```

In `scripts/storymaker-tests/34-block-beats.mjs`, replace:

```js
    const codes = issuesOf(broken).filter(i => i.path.includes('/beats/')).map(i => i.path.replace('/scenes/0/beats/', '') + ' ' + i.code);
    assert.deepEqual(codes, ['0/icon unknown_asset', '1/data missing_field', '2/data invalid_value', '3/at unknown_spot']);
});
```

with:

```js
    const codes = issuesOf(broken).filter(i => i.path.includes('/beats/')).map(i => i.path.replace('/scenes/0/beats/', '') + ' ' + i.code);
    assert.deepEqual(codes, ['0/icon unknown_asset', '1/data missing_field', '2/data invalid_value', '3/at unknown_spot']);
});

check('block beats: a dropped block still uses up its number, as in Go, and the legacy chart is counted', () => {
    const oneNode = { layout: 'flow', nodes: [{ id: 'a', label: 'A' }] };
    const out = compileScene([{ do: 'show', diagram: oneNode, at: 'full', entry: 'draw' }, { do: 'show', diagram: flow, at: 'full', entry: 'draw' },
        { do: 'highlight', target: 'diagram-2.srv', style: 'pulse' }]);
    const [d] = blocksOf(out);
    assert.equal(blocksOf(out).length, 1, 'the first diagram has one node and is dropped');
    assert.equal(d.id, 'diagram-2');
    assert.equal(d.marks.list.length, 1);
    assert.equal(issuesOf(out, 'unknown_name').length, 0);
    const data = { labels: ['a'], values: [1] };
    const charts = compileScene([{ do: 'chart', chart: 'bar', data, at: 'right' }, { do: 'show', chart: 'bar', data, at: 'full', entry: 'fade' }]);
    assert.deepEqual(plain(blocksOf(charts).map(b => b.id)), ['chart-1', 'chart-2']);
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run '(?i)(TestValidateBlocks|TestDeriveShowIDs|TestDecodeStoryKeepsDiagrams)' -count=1 ; npm run test:storymaker`
Expected: TestValidateBlocks: 11 rows fail (chart label/unit/title, list line, icon label, headline and list without text, point empty/long); TestDeriveShowIDsCountsLegacyCharts; JS: a dropped first diagram left the second as diagram-1

- [ ] **Step 3: Implement**

In `internal/storymaker/defaults.go`, replace:

```go
}

// deriveShowIDs returns, aligned with sc.Beats, the instance id a show beat without an id
// would get (<item-slug>-<n>), and "" for every other beat. It skips ids already taken by props
// and by beats with an explicit id, so ids stay unique.
func deriveShowIDs(sc *Scene) []string {
	if len(sc.Beats) == 0 {
		return nil
```

with:

```go
}

// deriveShowIDs returns, aligned with sc.Beats, the instance id a show beat without an id
// would get (<item-slug>-<n>), and "" for every other beat. The legacy chart beat counts as a
// show chart, as in the player. It skips ids already taken by props and by beats with an
// explicit id, so ids stay unique.
func deriveShowIDs(sc *Scene) []string {
	if len(sc.Beats) == 0 {
		return nil
```

In `internal/storymaker/defaults.go`, replace:

```go
	counters := map[string]int{}
	out := make([]string, len(sc.Beats))
	for j, b := range sc.Beats {
		if b.Do != "show" || b.ID != "" {
			continue
		}
		slug := showSlug(b)
		n := counters[slug] + 1
		id := slug + "-" + strconv.Itoa(n)
		for used[id] {
```

with:

```go
	counters := map[string]int{}
	out := make([]string, len(sc.Beats))
	for j, b := range sc.Beats {
		if (b.Do != "show" && b.Do != "chart") || b.ID != "" {
			continue
		}
		slug := showSlug(b)
		if b.Do == "chart" {
			slug = "chart"
		}
		n := counters[slug] + 1
		id := slug + "-" + strconv.Itoa(n)
		for used[id] {
```

In `internal/storymaker/validate.go`, replace:

```go
	}
	derived := deriveShowIDs(sc)
	for j, b := range sc.Beats {
		if b.Do != "show" {
			continue
		}
		id := b.ID
```

with:

```go
	}
	derived := deriveShowIDs(sc)
	for j, b := range sc.Beats {
		if b.Do != "show" && b.Do != "chart" {
			continue
		}
		id := b.ID
```

In `internal/storymaker/validate_blocks.go`, replace:

```go
	"fmt"
	"math"
	"strings"
)

// Block limits repeat ui/js/storymaker/blocks.js (LAYOUTS, CHARTS, ICON_STYLES, LIST_MAX, POINTS_MAX); change both together.
```

with:

```go
	"fmt"
	"math"
	"strings"
	"unicode"
	"unicode/utf16"
)

// Block limits repeat ui/js/storymaker/blocks.js (LAYOUTS, CHARTS, ICON_STYLES, LIST_MAX, POINTS_MAX); change both together.
```

In `internal/storymaker/validate_blocks.go`, replace:

```go
	blockNodeLabelMax = 40
)

// blockKind says which block a show beat makes: icon, diagram, chart, headline or list, or "" for an item or a plain text.
func blockKind(b *Beat) string {
	switch {
```

with:

```go
	blockNodeLabelMax = 40
)

// jsLen counts UTF-16 units, as String.length does in blocks.js, so Go and the player refuse the same texts.
func jsLen(s string) int { return len(utf16.Encode([]rune(s))) }

// blockKind says which block a show beat makes: icon, diagram, chart, headline or list, or "" for an item or a plain text.
func blockKind(b *Beat) string {
	switch {
```

In `internal/storymaker/validate_blocks.go`, replace:

```go
	case "chart":
		v.chartBlock(bb, b)
	case "headline":
		if n := runeLen(b.Text); n > blockHeadlineMax {
			v.errorf(CodeLimitExceeded, bb+"/text", fmt.Sprintf("The headline has %d characters; the limit is %d.", n, blockHeadlineMax), "Shorten the headline.")
		}
		if n := strings.Count(b.Text, "*"); n != 0 && n != 2 {
```

with:

```go
	case "chart":
		v.chartBlock(bb, b)
	case "headline":
		if strings.TrimSpace(strings.ReplaceAll(b.Text, "*", "")) == "" {
			v.errorf(CodeInvalidValue, bb+"/text", "The headline has no word to show.", "Write a short headline.")
		}
		if n := jsLen(b.Text); n > blockHeadlineMax {
			v.errorf(CodeLimitExceeded, bb+"/text", fmt.Sprintf("The headline has %d characters; the limit is %d.", n, blockHeadlineMax), "Shorten the headline.")
		}
		if n := strings.Count(b.Text, "*"); n != 0 && n != 2 {
```

In `internal/storymaker/validate_blocks.go`, replace:

```go
				continue
			}
			lines++
			if runeLen(line) > blockListLineMax {
				v.warnf(CodeInvalidValue, bb+"/text", fmt.Sprintf("A list line has %d characters; more than %d does not fit.", runeLen(line), blockListLineMax), "Shorten the line.")
			}
		}
		if lines > blockListMax {
			v.errorf(CodeLimitExceeded, bb+"/text", fmt.Sprintf("The list has %d lines; the limit is %d.", lines, blockListMax), "Keep at most 4 lines, one per line break.")
		}
```

with:

```go
				continue
			}
			lines++
			if n := jsLen(strings.TrimRightFunc(line, unicode.IsSpace)); n > blockListLineMax {
				v.errorf(CodeLimitExceeded, bb+"/text", fmt.Sprintf("A list line has %d characters; the limit is %d.", n, blockListLineMax), "Shorten the line.")
			}
		}
		if lines == 0 {
			v.errorf(CodeInvalidValue, bb+"/text", "The list has no line to show.", "Write one to four lines, one per line break.")
		}
		if lines > blockListMax {
			v.errorf(CodeLimitExceeded, bb+"/text", fmt.Sprintf("The list has %d lines; the limit is %d.", lines, blockListMax), "Keep at most 4 lines, one per line break.")
		}
```

In `internal/storymaker/validate_blocks.go`, replace:

```go
	if b.Style != "" && !containsName(iconStyles, b.Style) {
		v.errorf(CodeInvalidValue, bb+"/style", fmt.Sprintf("An icon style is line, badge or card, not %q.", clip(b.Style, 20)), `Use "style": "card".`)
	}
	if b.Text != "" && b.Style != "card" {
		v.warnf(CodeInvalidValue, bb+"/text", "Only the card style of an icon shows a label.", `Use "style": "card", or remove "text".`)
	}
}
```

with:

```go
	if b.Style != "" && !containsName(iconStyles, b.Style) {
		v.errorf(CodeInvalidValue, bb+"/style", fmt.Sprintf("An icon style is line, badge or card, not %q.", clip(b.Style, 20)), `Use "style": "card".`)
	}
	// A text next to an icon is its card label, whatever the style.
	if n := jsLen(b.Text); n > blockNodeLabelMax {
		v.errorf(CodeLimitExceeded, bb+"/text", fmt.Sprintf("The icon label has %d characters; the limit is %d.", n, blockNodeLabelMax), "Use one or two words.")
	}
}
```

In `internal/storymaker/validate_blocks.go`, replace:

```go
		}
	}
	for k, l := range d.Labels {
		if runeLen(l) > blockChartLabel {
			v.warnf(CodeInvalidValue, fmt.Sprintf("%s/data/labels/%d", bb, k), fmt.Sprintf("A chart label has %d characters; more than %d does not fit.", runeLen(l), blockChartLabel), "Shorten the label.")
		}
	}
	if runeLen(d.Unit) > blockChartUnit {
		v.warnf(CodeInvalidValue, bb+"/data/unit", fmt.Sprintf("A chart unit longer than %d characters does not fit.", blockChartUnit), `Use a short unit such as "%".`)
	}
	if runeLen(d.Title) > blockChartTitle {
		v.warnf(CodeInvalidValue, bb+"/data/title", fmt.Sprintf("A chart title longer than %d characters does not fit.", blockChartTitle), "Shorten the title.")
	}
}
```

with:

```go
		}
	}
	for k, l := range d.Labels {
		if n := jsLen(l); n > blockChartLabel {
			v.errorf(CodeLimitExceeded, fmt.Sprintf("%s/data/labels/%d", bb, k), fmt.Sprintf("A chart label has %d characters; the limit is %d.", n, blockChartLabel), "Shorten the label.")
		}
	}
	if jsLen(d.Unit) > blockChartUnit {
		v.errorf(CodeLimitExceeded, bb+"/data/unit", fmt.Sprintf("A chart unit longer than %d characters does not fit.", blockChartUnit), `Use a short unit such as "%".`)
	}
	if jsLen(d.Title) > blockChartTitle {
		v.errorf(CodeLimitExceeded, bb+"/data/title", fmt.Sprintf("A chart title longer than %d characters does not fit.", blockChartTitle), "Shorten the title.")
	}
}
```

In `internal/storymaker/validate_blocks.go`, replace:

```go
		default:
			seen[n.ID] = true
		}
		switch l := runeLen(n.Label); {
		case strings.TrimSpace(n.Label) == "":
			v.errorf(CodeMissingField, np+"/label", "A diagram node needs a label.", `Add "label": "Server".`)
		case l > blockNodeLabelMax:
```

with:

```go
		default:
			seen[n.ID] = true
		}
		switch l := jsLen(n.Label); {
		case strings.TrimSpace(n.Label) == "":
			v.errorf(CodeMissingField, np+"/label", "A diagram node needs a label.", `Add "label": "Server".`)
		case l > blockNodeLabelMax:
```

In `internal/storymaker/validate_blocks.go`, replace:

```go
		if n.Icon != "" {
			v.checkIcon(np+"/icon", n.Icon)
		}
		switch {
		case len(n.Points) > blockPointsMax:
			v.errorf(CodeLimitExceeded, np+"/points", fmt.Sprintf("A node has %d points; the limit is %d.", len(n.Points), blockPointsMax), "Keep at most 3 points.")
```

with:

```go
		if n.Icon != "" {
			v.checkIcon(np+"/icon", n.Icon)
		}
		for k, pt := range n.Points {
			if l := jsLen(pt); l < 1 || l > blockNodeLabelMax {
				v.errorf(CodeLimitExceeded, fmt.Sprintf("%s/points/%d", np, k), fmt.Sprintf("A point has %d characters; it needs 1 to %d.", l, blockNodeLabelMax), "Use a short, non-empty point.")
			}
		}
		switch {
		case len(n.Points) > blockPointsMax:
			v.errorf(CodeLimitExceeded, np+"/points", fmt.Sprintf("A node has %d points; the limit is %d.", len(n.Points), blockPointsMax), "Keep at most 3 points.")
```

In `ui/js/storymaker/beats-blocks.js`, replace:

```js
    // shown. h = {warn, beatPath, instanceId, register}. The timing is set by reveals() when the beat is scheduled.
    function target(c, b, j, h) {
        var kind = b.do === 'chart' ? 'chart' : kindOf(b);
        if (kind === 'chart' && (b.data === undefined || b.data === null)) {
            h.warn('warning', h.beatPath(j, 'data'), 'missing_field', 'A chart needs data {labels, values}.');
            return null;
```

with:

```js
    // shown. h = {warn, beatPath, instanceId, register}. The timing is set by reveals() when the beat is scheduled.
    function target(c, b, j, h) {
        var kind = b.do === 'chart' ? 'chart' : kindOf(b);
        // The instance id comes first, as in Go's deriveShowIDs: a beat that is dropped still uses up its number.
        var id = h.instanceId(slugOf(b, kind), b.id);
        if (kind === 'chart' && (b.data === undefined || b.data === null)) {
            h.warn('warning', h.beatPath(j, 'data'), 'missing_field', 'A chart needs data {labels, values}.');
            return null;
```

In `ui/js/storymaker/beats-blocks.js`, replace:

```js
            h.warn('warning', h.beatPath(j, 'at'), 'unknown_spot', 'Unknown block area ' + JSON.stringify(at) + '; the player uses "full".');
            area = SM.blocks.area(c.stage, c.env, c.variant, 'full');
        }
        var def = defOf(b, kind, h.instanceId(slugOf(b, kind), b.id));
        def.area = area;
        def.hidden = true;
        def.entry = b.entry || (kind === 'icon' || kind === 'diagram' ? 'draw' : 'fade');
```

with:

```js
            h.warn('warning', h.beatPath(j, 'at'), 'unknown_spot', 'Unknown block area ' + JSON.stringify(at) + '; the player uses "full".');
            area = SM.blocks.area(c.stage, c.env, c.variant, 'full');
        }
        var def = defOf(b, kind, id);
        def.area = area;
        def.hidden = true;
        def.entry = b.entry || (kind === 'icon' || kind === 'diagram' ? 'draw' : 'fade');
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `go test ./internal/storymaker -run '(?i)(TestValidateBlocks|TestDeriveShowIDs|TestDecodeStoryKeepsDiagrams)' -count=1 ; npm run test:storymaker`
Expected: PASS; npm run test:storymaker 313 passed, 0 failed; build:ui + check:ui ok

- [ ] **Step 6: Commit**

```bash
git add internal/storymaker/blocks_model_test.go internal/storymaker/defaults.go internal/storymaker/validate.go internal/storymaker/validate_blocks.go internal/storymaker/validate_blocks_test.go scripts/storymaker-tests/34-block-beats.mjs ui/js/storymaker/beats-blocks.js ui/js/storymaker/player.bundle.js
git commit -m "storymaker: Go refuses the blocks the player cannot draw; ids are counted alike"
```

### Task D11: block_too_small judges diagrams and charts by height, text by its size

**Files:**
- Modify: `internal/storymaker/layout.go`
- Modify: `internal/storymaker/layout_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/layout_test.go`, replace:

```go
	sc.Blocks = []LayoutBlock{
		{ID: "covered", Kind: "chart", Layout: "bar", Label: 0.03, Box: LayoutBox{0.3, 0.5, 0.3, 0.4}},
		{ID: "outside", Kind: "list", Label: 0.03, Box: LayoutBox{0.8, 0.1, 0.4, 0.3}},
		{ID: "tiny", Kind: "headline", Label: 0.03, Box: LayoutBox{0.1, 0.1, 0.3, 0.05}},
		{ID: "hidden", Kind: "diagram", Layout: "flow", Label: 0.03, Box: LayoutBox{0.45, 0.2, 0.2, 0.3}},
		{ID: "label", Kind: "diagram", Layout: "flow", Label: 0.02, Box: LayoutBox{0.05, 0.55, 0.3, 0.2}},
		{ID: "fine", Kind: "diagram", Layout: "flow", Label: 0.03, Box: LayoutBox{0.05, 0.05, 0.3, 0.4}},
```

with:

```go
	sc.Blocks = []LayoutBlock{
		{ID: "covered", Kind: "chart", Layout: "bar", Label: 0.03, Box: LayoutBox{0.3, 0.5, 0.3, 0.4}},
		{ID: "outside", Kind: "list", Label: 0.03, Box: LayoutBox{0.8, 0.1, 0.4, 0.3}},
		{ID: "tiny", Kind: "diagram", Layout: "flow", Label: 0.03, Box: LayoutBox{0.1, 0.1, 0.3, 0.06}},
		{ID: "one line", Kind: "headline", Label: 0.03, Box: LayoutBox{0.1, 0.02, 0.3, 0.06}},
		{ID: "hidden", Kind: "diagram", Layout: "flow", Label: 0.03, Box: LayoutBox{0.45, 0.2, 0.2, 0.3}},
		{ID: "label", Kind: "diagram", Layout: "flow", Label: 0.02, Box: LayoutBox{0.05, 0.55, 0.3, 0.2}},
		{ID: "fine", Kind: "diagram", Layout: "flow", Label: 0.03, Box: LayoutBox{0.05, 0.05, 0.3, 0.4}},
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run '(?i)(TestLayoutIssuesForBlocks)' -count=1`
Expected: issues = [caption_overlap offscreen block_too_small block_too_small actor_prop_overlap block_too_small], the one-line headline (box height 0.06, label 0.03) was flagged

- [ ] **Step 3: Implement**

In `internal/storymaker/layout.go`, replace:

```go
			if outsideShare(b.Box) > layoutTextOutside {
				add(CodeOffscreen, fmt.Sprintf("The block %s is partly outside the frame in scene %s.", b.ID, sc.ID), blockFix(blockAreas))
			}
			if b.Box[3] < layoutBlockMinHeight || (b.Label > 0 && b.Label < layoutBlockMinLabel) {
				add(CodeBlockTooSmall, fmt.Sprintf("The block %s is too small to read in scene %s.", b.ID, sc.ID),
					blockFix(`Show it at "full" or on a bigger area, or give it fewer nodes or shorter labels.`))
			}
```

with:

```go
			if outsideShare(b.Box) > layoutTextOutside {
				add(CodeOffscreen, fmt.Sprintf("The block %s is partly outside the frame in scene %s.", b.ID, sc.ID), blockFix(blockAreas))
			}
			// A diagram or chart needs height to be read; a headline, list or icon is as short as its text, so only its label size counts.
			tallKind := b.Kind == "diagram" || b.Kind == "chart"
			if (tallKind && b.Box[3] < layoutBlockMinHeight) || (b.Label > 0 && b.Label < layoutBlockMinLabel) {
				add(CodeBlockTooSmall, fmt.Sprintf("The block %s is too small to read in scene %s.", b.ID, sc.ID),
					blockFix(`Show it at "full" or on a bigger area, or give it fewer nodes or shorter labels.`))
			}
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker -run '(?i)(TestLayoutIssuesForBlocks)' -count=1`
Expected: PASS; the 0.06 high diagram still gives block_too_small

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/layout.go internal/storymaker/layout_test.go
git commit -m "storymaker: block_too_small judges diagrams and charts by height, text by its size"
```

### Task D12: a code scene's block payload stays within the revive limits

**Files:**
- Modify: `scripts/storymaker-tests/33-blocks.mjs`
- Modify: `ui/js/storymaker/blocks.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/33-blocks.mjs`, replace:

```js
    const xOf = (time, s) => parseFloat(paint(def, time).find(l => l.startsWith('fillText(' + s + ',')).slice(10 + s.length).split(',')[0]);
    assert.ok(xOf(timeFor(0.3), 'Laden') < xOf(5, 'Laden'), 'the first line is left of its final place while it slides');
});
```

with:

```js
    const xOf = (time, s) => parseFloat(paint(def, time).find(l => l.startsWith('fillText(' + s + ',')).slice(10 + s.length).split(',')[0]);
    assert.ok(xOf(timeFor(0.3), 'Laden') < xOf(5, 'Laden'), 'the first line is left of its final place while it slides');
});

check('blocks: serialize keeps what revive accepts, 70 camera shakes and 70 marks do not drop the blocks', () => {
    const e = B.create({ id: 'text-1', kind: 'headline', text: 'Hi', area: { x: 0, y: 0, w: 800, h: 200, surface: 'panel' } });
    const cam = SM.camera.create(1, 960, 540);
    for (let k = 69; k >= 0; k--) SM.camera.addShake(cam, k, k + 0.5, 5);
    for (let k = 0; k < 70; k++) { e.marks.add(k, k + 1, { node: 'a', style: 'pulse' }); e.x.add(k, k + 0.5, 100 + k, 'linear'); }
    const wire = JSON.parse(JSON.stringify(B.serialize([e], cam)));
    assert.equal(wire.camera.shakes.length, 64);
    assert.equal(wire.camera.shakes[0].t0, 0, 'the first in time order, not in call order');
    assert.equal(wire.blocks[0].marks.length, 64);
    assert.equal(wire.blocks[0].x.segs.length, 64);
    const back = B.revive(wire, cat);
    assert.notEqual(back, null);
    assert.equal(back.entities.length, 1);
    const many = Array.from({ length: 40 }, (_, k) => B.create({ id: 'text-' + k, kind: 'headline', text: 'Hi', area: { x: 0, y: 0, w: 800, h: 200, surface: 'panel' } }));
    assert.equal(B.revive(JSON.parse(JSON.stringify(B.serialize(many, null))), cat).entities.length, 32);
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `npm run test:storymaker`
Expected: the new 33-blocks check failed: serialize kept 70 camera shakes, revive refused them and dropped the block

- [ ] **Step 3: Implement**

In `ui/js/storymaker/blocks.js`, replace:

```js

    function trackOut(track) {
        track.finalize();
        return { init: track.init, segs: track.segs.map(function (s) { return { t0: s.t0, t1: s.t1, from: s.from, to: s.to, ease: s.ease, log: !!s.log }; }) };
    }

    function listOut(list, keys) {
        return list.list.map(function (it) {
            var o = { t0: it.t0, t1: it.t1 };
            keys.forEach(function (k) { o[k] = it.data[k]; });
            return o;
        });
    }

    // serialize(entities, camera) = a plain payload of the block entities and the camera, or null without blocks.
    function serialize(entities, camera) {
        var list = (entities || []).filter(function (e) { return e && e.type === 'block'; });
        if (!list.length) return null;
        var cam = null;
        if (camera) cam = { zoom: trackOut(camera.zoom), cx: trackOut(camera.cx), cy: trackOut(camera.cy), shakes: (camera.shakes || []).map(function (s) { return { t0: s.t0, t1: s.t1, amp: s.amp }; }) };
        return {
            v: 1, camera: cam,
            blocks: list.map(function (e) {
                return {
                    id: e.id, block: e.block, w: e.w, h: e.h, surface: e.surface, t0: e.t0, dur: e.dur, entry: e.entry, reveals: e.reveals,
                    x: trackOut(e.x), y: trackOut(e.y), alpha: trackOut(e.alpha), scale: trackOut(e.scale), rot: trackOut(e.rot),
```

with:

```js

    function trackOut(track) {
        track.finalize();
        return { init: track.init, segs: track.segs.slice(0, MAX_SEGS).map(function (s) { return { t0: s.t0, t1: s.t1, from: s.from, to: s.to, ease: s.ease, log: !!s.log }; }) };
    }

    // earliest(items): the first MAX_MARKS items in time order; revive refuses more, and that would drop every block box of the scene.
    function earliest(items) {
        return items.map(function (it, i) { return { it: it, i: i }; }).sort(function (a, b) { return a.it.t0 - b.it.t0 || a.i - b.i; }).slice(0, MAX_MARKS).map(function (w) { return w.it; });
    }

    function listOut(list, keys) {
        return earliest(list.list).map(function (it) {
            var o = { t0: it.t0, t1: it.t1 };
            keys.forEach(function (k) { o[k] = it.data[k]; });
            return o;
        });
    }

    // serialize(entities, camera) = a plain payload of the block entities and the camera, or null without blocks. The payload only
    // measures code-scene blocks, so it keeps at most the counts that revive accepts (first in time order).
    function serialize(entities, camera) {
        var list = (entities || []).filter(function (e) { return e && e.type === 'block'; });
        if (!list.length) return null;
        var cam = null;
        if (camera) cam = { zoom: trackOut(camera.zoom), cx: trackOut(camera.cx), cy: trackOut(camera.cy), shakes: earliest(camera.shakes || []).map(function (s) { return { t0: s.t0, t1: s.t1, amp: s.amp }; }) };
        return {
            v: 1, camera: cam,
            blocks: list.slice(0, MAX_BLOCKS).map(function (e) {
                return {
                    id: e.id, block: e.block, w: e.w, h: e.h, surface: e.surface, t0: e.t0, dur: e.dur, entry: e.entry, reveals: e.reveals,
                    x: trackOut(e.x), y: trackOut(e.y), alpha: trackOut(e.alpha), scale: trackOut(e.scale), rot: trackOut(e.rot),
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `npm run test:storymaker`
Expected: 314 passed, 0 failed; build:ui + check:ui ok

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/33-blocks.mjs ui/js/storymaker/blocks.js ui/js/storymaker/player.bundle.js
git commit -m "storymaker: a code scene's block payload stays within the revive limits"
```

