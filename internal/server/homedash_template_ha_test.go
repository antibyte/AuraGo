package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/homedash"
	"aurago/internal/homedash/sources/hass"
	"aurago/internal/homedash/sources/hass/hasstest"
	"aurago/internal/homedash/sourcetest"
)

func haEntity(native, area, areaID string, kind homedash.Kind, caps ...homedash.CapabilitySpec) homedash.Entity {
	e := sourcetest.Entity(homedash.MakeEntityID(hass.SourceID, native), kind, caps...)
	e.Name, e.Area, e.AreaID = native, area, areaID
	return e
}

func allowedCap(c homedash.Capability) homedash.CapabilitySpec {
	return homedash.CapabilitySpec{Name: c, Allowed: true}
}

func pageSummary(d homedash.Dashboard) []string {
	var out []string
	for _, p := range d.Pages {
		for _, w := range p.Widgets {
			out = append(out, fmt.Sprintf("%s|%s|%s/%s=%s", p.ID, p.Name, w.Type, w.Variant, w.Bindings["primary"]))
		}
	}
	return out
}

func TestHomeDashHATemplateBuilder(t *testing.T) {
	sel := allowedCap(homedash.CapSelectOption)
	sel.Options = []string{"a", "b", "c"}
	big := allowedCap(homedash.CapSelectOption)
	big.Options = []string{"1", "2", "3", "4", "5"}
	box := haEntity("number.boiler", "", "", homedash.KindNumber, allowedCap(homedash.CapSetValue))
	box.State.Attributes = map[string]any{"mode": "box"}
	blocked := haEntity("script.night", "", "", homedash.KindScript, homedash.CapabilitySpec{Name: homedash.CapActivate, BlockedBy: "home_assistant.allowed_services"})
	list := []homedash.Entity{
		haEntity("switch.b", "Kitchen", "kitchen", homedash.KindSwitch, allowedCap(homedash.CapToggle)),
		haEntity("light.a", "Kitchen", "kitchen", homedash.KindLight, allowedCap(homedash.CapToggle), allowedCap(homedash.CapBrightness)),
		haEntity("light.c", "Bath", "bath", homedash.KindLight, allowedCap(homedash.CapToggle)),
		haEntity("select.s", "Bath", "bath", homedash.KindSelect, sel),
		haEntity("input_select.big", "", "", homedash.KindSelect, big),
		box, blocked,
		haEntity("sensor.t", "Kitchen", "kitchen", homedash.KindSensor),
		sourcetest.Entity("fake:light.x", homedash.KindLight, allowedCap(homedash.CapToggle)),
	}
	doc, omitted, ok := homeDashTemplateHA(list, "Home", "Other", 12, 400)
	want := []string{
		"bath|Bath|light/tile=ha:light.c", "bath|Bath|select/segmented=ha:select.s",
		"kitchen|Kitchen|light/slider=ha:light.a", "kitchen|Kitchen|switch/tile=ha:switch.b",
		"unassigned|Other|select/list=ha:input_select.big", "unassigned|Other|number/stepper=ha:number.boiler",
	}
	if !ok || omitted != 0 || !slices.Equal(pageSummary(doc), want) {
		t.Fatalf("template = %v omitted=%d ok=%v", pageSummary(doc), omitted, ok)
	}
	cat := homedash.Catalog()
	doc.Slug = "home"
	homedash.Normalize(&doc)
	homedash.Autopack(&doc, cat)
	if rep := homedash.Validate(doc, cat, nil); !rep.OK() {
		t.Fatalf("issues = %+v", rep.Issues)
	}

	var many []homedash.Entity
	for i := 0; i < 30; i++ {
		many = append(many, haEntity(fmt.Sprintf("light.l%02d", i), "Hall", "hall", homedash.KindLight, allowedCap(homedash.CapToggle)))
	}
	if doc, omitted, _ = homeDashTemplateHA(many, "H", "O", 12, 400); len(doc.Pages) != 1 || len(doc.Pages[0].Widgets) != 24 || omitted != 6 {
		t.Fatalf("per-page cap: %d widgets, omitted %d", len(doc.Pages[0].Widgets), omitted)
	}
	var areas []homedash.Entity
	for i := 0; i < 14; i++ {
		areas = append(areas, haEntity(fmt.Sprintf("light.a%02d", i), fmt.Sprintf("Area %02d", i), fmt.Sprintf("a%02d", i), homedash.KindLight, allowedCap(homedash.CapToggle)))
	}
	areas = append(areas, haEntity("light.free", "", "", homedash.KindLight, allowedCap(homedash.CapToggle)))
	if doc, omitted, _ = homeDashTemplateHA(areas, "H", "O", 12, 400); len(doc.Pages) != 12 || omitted != 3 {
		t.Fatalf("page cap: %d pages, omitted %d", len(doc.Pages), omitted)
	}
	if doc, omitted, _ = homeDashTemplateHA(many, "H", "O", 12, 5); len(doc.Pages[0].Widgets) != 5 || omitted != 25 {
		t.Fatalf("total cap: %d widgets, omitted %d", len(doc.Pages[0].Widgets), omitted)
	}
	if _, _, ok := homeDashTemplateHA([]homedash.Entity{blocked}, "H", "O", 12, 400); ok {
		t.Fatal("nothing qualifies, but ok")
	}
}

func haTemplateFake() *sourcetest.FakeSource {
	return sourcetest.New(hass.SourceID,
		haEntity("light.kitchen", "Kitchen", "kitchen", homedash.KindLight, allowedCap(homedash.CapToggle), allowedCap(homedash.CapBrightness)),
		haEntity("switch.coffee", "Kitchen", "kitchen", homedash.KindSwitch, allowedCap(homedash.CapToggle)))
}

type hdHATemplateAnswer struct {
	Dashboard homedash.Dashboard `json:"dashboard"`
	Warnings  []homedash.Issue   `json:"warnings"`
	Omitted   *int               `json:"omitted"`
	Code      string             `json:"code"`
	Source    string             `json:"source"`
}

func TestHomeDashHATemplateRoute(t *testing.T) {
	env := newHomeDashTestEnv(t, nil, fakeHomeDashSpec(haTemplateFake()))
	body := map[string]any{"template": "home_assistant", "name": "Zuhause", "unassigned_title": "Ohne Raum"}
	w := env.do(t, env.admin, http.MethodPost, "dashboards", body)
	var a hdHATemplateAnswer
	hdDecode(t, w, &a)
	if w.Code != http.StatusCreated || a.Omitted == nil || *a.Omitted != 0 || len(a.Warnings) != 0 || a.Dashboard.Slug != "home-assistant" || a.Dashboard.Name != "Zuhause" ||
		!slices.Equal(pageSummary(a.Dashboard), []string{"kitchen|Kitchen|light/slider=ha:light.kitchen", "kitchen|Kitchen|switch/tile=ha:switch.coffee"}) {
		t.Fatalf("template = %d %s", w.Code, w.Body.String())
	}
	hdDecode(t, env.do(t, env.admin, http.MethodPost, "dashboards", body), &a)
	if a.Dashboard.Slug != "home-assistant-2" {
		t.Fatalf("second slug = %q", a.Dashboard.Slug)
	}
	if w := env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"template": "system", "unassigned_title": "x"}); w.Code != http.StatusBadRequest {
		t.Fatalf("system with unassigned_title = %d", w.Code)
	}
	var sys hdHATemplateAnswer
	hdDecode(t, env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"template": "system"}), &sys)
	if sys.Omitted != nil {
		t.Fatal("omitted belongs to the home_assistant answer only")
	}
}

func TestHomeDashHATemplateErrors(t *testing.T) {
	env := newHomeDashTestEnv(t, nil)
	var a hdHATemplateAnswer
	w := env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"template": "home_assistant"})
	hdDecode(t, w, &a)
	if w.Code != http.StatusServiceUnavailable || a.Code != "source_unavailable" || a.Source != "ha" {
		t.Fatalf("no HA source = %d %s", w.Code, w.Body.String())
	}
	for _, bad := range []map[string]any{
		{"template": "home_assistant", "name": "http://x.example"},
		{"template": "home_assistant", "unassigned_title": "a\u0007b"},
		{"template": "home_assistant", "unassigned_title": strings.Repeat("x", 41)},
		{"template": "home_assistant", "name": strings.Repeat("x", 81)},
	} {
		if w := env.do(t, env.admin, http.MethodPost, "dashboards", bad); w.Code != http.StatusUnprocessableEntity || hdCode(t, w) != homedash.CodeInvalidDocument {
			t.Fatalf("%v = %d %s", bad, w.Code, w.Body.String())
		}
	}
	sensors := sourcetest.New(hass.SourceID, haEntity("sensor.t", "Kitchen", "kitchen", homedash.KindSensor))
	env2 := newHomeDashTestEnv(t, nil, fakeHomeDashSpec(sensors))
	w = env2.do(t, env2.admin, http.MethodPost, "dashboards", map[string]any{"template": "home_assistant"})
	hdDecode(t, w, &a)
	if w.Code != http.StatusUnprocessableEntity || a.Code != "template_empty" {
		t.Fatalf("nothing controllable = %d %s", w.Code, w.Body.String())
	}
}

func bootTemplates(t *testing.T, env *homeDashTestEnv) []string {
	t.Helper()
	var boot struct {
		Templates []string `json:"templates"`
	}
	hdDecode(t, env.do(t, env.admin, http.MethodGet, "bootstrap", nil), &boot)
	return boot.Templates
}

func TestHomeDashBootstrapAdvertisesHATemplate(t *testing.T) {
	env := newHomeDashTestEnv(t, nil, fakeHomeDashSpec(haTemplateFake()))
	sourcetest.WaitFor(t, 3*time.Second, func() bool { return slices.Contains(bootTemplates(t, env), "home_assistant") },
		"home_assistant listed after the background catalog refresh")
	sensors := sourcetest.New(hass.SourceID, haEntity("sensor.t", "Kitchen", "kitchen", homedash.KindSensor))
	env2 := newHomeDashTestEnv(t, nil, fakeHomeDashSpec(sensors))
	env2.do(t, env2.admin, http.MethodGet, "entities?source=ha", nil) // catalog loaded
	if got := bootTemplates(t, env2); !slices.Equal(got, []string{"system"}) {
		t.Fatalf("templates = %v, want only system", got)
	}
}

// haTemplateContractJSON is the pages part of the contract section 5 example (demo home).
const haTemplateContractJSON = `{"schema_version":1,"slug":"home-assistant","name":"Zuhause","revision":1,"approved_revision":1,"saved_by":"user",
"look":{"family":"glass","palette":{"mode":"theme"},"backdrop":{"kind":"theme"},"density":"comfortable","radius_scale":1},
"grid":{"row_unit":"auto","gap":null,"fit":"scroll"},
"pages":[
 {"id":"kitchen","name":"Kitchen","compact":true,"widgets":[
  {"id":"w1","type":"light","variant":"slider","bindings":{"primary":"ha:light.kitchen"},"options":{},"actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false},
  {"id":"w2","type":"switch","variant":"tile","bindings":{"primary":"ha:switch.coffee"},"options":{},"actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false},
  {"id":"w3","type":"select","variant":"segmented","bindings":{"primary":"ha:select.heater_mode"},"options":{},"actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false}],
  "layouts":{"lg":[{"id":"w1","x":0,"y":0,"w":4,"h":1},{"id":"w2","x":4,"y":0,"w":2,"h":2},{"id":"w3","x":6,"y":0,"w":3,"h":1}]}},
 {"id":"living_room","name":"Living room","compact":true,"widgets":[
  {"id":"w4","type":"light","variant":"tile","bindings":{"primary":"ha:light.sofa"},"options":{},"actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false},
  {"id":"w5","type":"action","variant":"button","bindings":{"primary":"ha:scene.movie"},"options":{},"actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false},
  {"id":"w6","type":"number","variant":"slider","bindings":{"primary":"ha:input_number.volume"},"options":{},"actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false}],
  "layouts":{"lg":[{"id":"w4","x":0,"y":0,"w":2,"h":2},{"id":"w5","x":2,"y":0,"w":2,"h":1},{"id":"w6","x":4,"y":0,"w":2,"h":1}]}},
 {"id":"unassigned","name":"Ohne Raum","compact":true,"widgets":[
  {"id":"w7","type":"switch","variant":"tile","bindings":{"primary":"ha:input_boolean.vacation_mode"},"options":{},"actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false},
  {"id":"w8","type":"action","variant":"button","bindings":{"primary":"ha:button.doorbell_test"},"options":{},"actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false},
  {"id":"w9","type":"select","variant":"list","bindings":{"primary":"ha:input_select.radio"},"options":{},"actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false},
  {"id":"w10","type":"number","variant":"stepper","bindings":{"primary":"ha:number.boiler_target"},"options":{},"actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false}],
  "layouts":{"lg":[{"id":"w7","x":0,"y":0,"w":2,"h":2},{"id":"w8","x":2,"y":0,"w":2,"h":1},{"id":"w9","x":4,"y":0,"w":2,"h":1},{"id":"w10","x":6,"y":0,"w":2,"h":1}]}}],
"kiosk":{"start_page":"kitchen","idle_dim_seconds":120,"idle_dim_level":0.3,"page_rotation_seconds":0,"screen_saver":"clock","show_clock":true}}`

// TestHomeDashHATemplateMatchesContractExample builds the template from the demo home through the real
// Home Assistant adapter and compares the answer with contract section 5 (ignoring id and updated_at).
// The demo's script.goodnight (policy), input_boolean.guest_mode (hidden), sensor and camera (not W1) and
// the alarm panel (excluded) must not appear and must not count as omitted.
func TestHomeDashHATemplateMatchesContractExample(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	env := newHomeDashTestEnv(t, func(cfg *config.Config) {
		cfg.HomeAssistant.Enabled, cfg.HomeAssistant.URL, cfg.HomeAssistant.AccessToken = true, ha.URL, ha.Token
		cfg.HomeDash.Sources.HomeAssistant.Enabled = true
	}, homeDashHASpec(hass.FactoryOptions{}))
	w := env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"template": "home_assistant", "name": "Zuhause", "unassigned_title": "Ohne Raum"})
	if w.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", w.Code, w.Body.String())
	}
	var got struct {
		Dashboard map[string]any `json:"dashboard"`
		Warnings  []any          `json:"warnings"`
		Omitted   *int           `json:"omitted"`
	}
	hdDecode(t, w, &got)
	if got.Omitted == nil || *got.Omitted != 0 || len(got.Warnings) != 0 {
		t.Fatalf("omitted = %v warnings = %v", got.Omitted, got.Warnings)
	}
	for _, k := range []string{"id", "updated_at"} {
		if _, ok := got.Dashboard[k]; !ok {
			t.Fatalf("dashboard lacks %q", k)
		}
		delete(got.Dashboard, k)
	}
	var want map[string]any
	if err := json.Unmarshal([]byte(haTemplateContractJSON), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Dashboard, want) {
		g, _ := json.MarshalIndent(got.Dashboard, "", " ")
		t.Fatalf("template differs from the contract example: %s", g)
	}
}

// stalledHAFake is a Home Assistant source whose catalog answers only after a long delay.
func stalledHAFake() *sourcetest.FakeSource {
	f := haTemplateFake()
	f.CatalogDelay = 1500 * time.Millisecond
	return f
}

func TestHomeDashBootstrapNeverWaitsForHomeAssistant(t *testing.T) {
	fake := stalledHAFake()
	env := newHomeDashTestEnv(t, nil, fakeHomeDashSpec(fake))
	start := time.Now()
	for i := 0; i < 3; i++ {
		if got := bootTemplates(t, env); !slices.Equal(got, []string{"system"}) {
			t.Fatalf("templates = %v while the catalog stalls", got)
		}
	}
	if d := time.Since(start); d > 700*time.Millisecond {
		t.Fatalf("three bootstraps took %v: bootstrap waited for Home Assistant", d)
	}
	time.Sleep(100 * time.Millisecond)
	if n := fake.CatalogCalls(); n != 1 {
		t.Fatalf("catalog fetches = %d, want exactly one background refresh at a time", n)
	}
}

func TestHomeDashBootstrapRefreshEndsWithTheService(t *testing.T) {
	env := newHomeDashTestEnv(t, nil, fakeHomeDashSpec(stalledHAFake()))
	bootTemplates(t, env)
	if !env.svc.haProbe.Load() {
		t.Fatal("no background refresh started")
	}
	env.s.closeHomeDash()
	sourcetest.WaitFor(t, 5*time.Second, func() bool { return !env.svc.haProbe.Load() }, "the background refresh to end after the service closed")
}

func TestHomeDashHAPageIDsRespectTheContractLimit(t *testing.T) {
	tog := allowedCap(homedash.CapToggle)
	long := strings.Repeat("x", 40)
	prefix := strings.Repeat("y", 28)
	list := []homedash.Entity{
		haEntity("light.a", "A", long, homedash.KindLight, tog),
		haEntity("light.b", "B", prefix+"1", homedash.KindLight, tog),
		haEntity("light.c", "C", prefix+"2", homedash.KindLight, tog),
		haEntity("light.d", "D", prefix+"3", homedash.KindLight, tog),
		haEntity("light.e", "E", "---", homedash.KindLight, tog),
		haEntity("light.f", "F", "\u00e4\u00f6", homedash.KindLight, tog),
		haEntity("light.g", "G", "unassigned", homedash.KindLight, tog),
		haEntity("light.h", "", "", homedash.KindLight, tog),
	}
	doc, _, ok := homeDashTemplateHA(list, "Home", "Other", 12, 400)
	if !ok {
		t.Fatal("no template")
	}
	seen := map[string]bool{}
	var ids []string
	for _, p := range doc.Pages {
		ids = append(ids, p.ID)
		if len(p.ID) > 28 || seen[p.ID] {
			t.Fatalf("page id %q too long or duplicated in %v", p.ID, ids)
		}
		seen[p.ID] = true
	}
	want := []string{long[:28], prefix, prefix[:26] + "-2", prefix[:26] + "-3", "area", "area-2", "unassigned-2", "unassigned"}
	if !slices.Equal(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
}

func TestHomeDashHAUnassignedPageKeepsItsIDAgainstAnArea(t *testing.T) {
	tog := allowedCap(homedash.CapToggle)
	doc, _, _ := homeDashTemplateHA([]homedash.Entity{
		haEntity("light.a", "Zone", "unassigned", homedash.KindLight, tog),
		haEntity("light.b", "", "", homedash.KindLight, tog),
	}, "Home", "Other", 12, 400)
	if len(doc.Pages) != 2 || doc.Pages[0].ID != "unassigned-2" || doc.Pages[1].ID != "unassigned" || doc.Pages[1].Name != "Other" {
		t.Fatalf("pages = %+v", doc.Pages)
	}
}
