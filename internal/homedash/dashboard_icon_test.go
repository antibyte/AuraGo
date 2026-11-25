package homedash_test

import (
	"encoding/json"
	"strings"
	"testing"

	"aurago/internal/homedash"
)

func TestWidgetIconIsOptionalAndValidated(t *testing.T) {
	raw := `{"name":"I","slug":"i","pages":[{"id":"main","name":"Main","widgets":[{"id":"w1","type":"clock","variant":"digital","icon":"timer","bindings":{},"options":{}}]}]}`
	doc, err := homedash.DecodeDashboard([]byte(raw))
	if err != nil || doc.Pages[0].Widgets[0].Icon != "timer" {
		t.Fatalf("decode = %+v %v", doc, err)
	}
	cat := homedash.Catalog()
	homedash.Normalize(&doc)
	homedash.Autopack(&doc, cat)
	if rep := homedash.Validate(doc, cat, nil); !rep.OK() {
		t.Fatalf("issues = %+v", rep.Issues)
	}
	doc.Pages[0].Widgets[0].Icon = "Bad Icon"
	rep := homedash.Validate(doc, cat, nil)
	if len(rep.Issues) != 1 || rep.Issues[0].Path != "/pages/0/widgets/0/icon" || rep.Issues[0].Code != homedash.IssueBadFormat {
		t.Fatalf("issues = %+v", rep.Issues)
	}
	out, _ := json.Marshal(homedash.Widget{ID: "w", Type: "clock", Variant: "digital"})
	if strings.Contains(string(out), `"icon"`) {
		t.Fatalf("empty icon must be omitted: %s", out)
	}
}

func TestUpdateWidgetPatchSetsAndKeepsIcon(t *testing.T) {
	raw := `{"name":"I","slug":"i","pages":[{"id":"main","name":"Main","widgets":[{"id":"w1","type":"clock","variant":"digital","bindings":{},"options":{}}]}]}`
	doc, err := homedash.DecodeDashboard([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	doc, err = homedash.ApplyPatch(doc, []homedash.PatchOp{{Op: "update_widget", WidgetID: "w1", Widget: &homedash.Widget{Icon: "cpu"}}})
	if err != nil || doc.Pages[0].Widgets[0].Icon != "cpu" {
		t.Fatalf("icon patch = %+v %v", doc.Pages[0].Widgets, err)
	}
	doc, err = homedash.ApplyPatch(doc, []homedash.PatchOp{{Op: "update_widget", WidgetID: "w1", Widget: &homedash.Widget{Title: "Clock"}}})
	if err != nil || doc.Pages[0].Widgets[0].Icon != "cpu" || doc.Pages[0].Widgets[0].Title != "Clock" {
		t.Fatalf("title patch dropped the icon: %+v %v", doc.Pages[0].Widgets, err)
	}
}
