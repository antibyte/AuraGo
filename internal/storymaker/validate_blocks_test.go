package storymaker

import (
	"strings"
	"testing"
)

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
