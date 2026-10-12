package cydcreator

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func newTestProject(t *testing.T) Project {
	t.Helper()
	p, err := NewProject("Wetterstation", "esp32-2432s028r", "night", "de", "CET-1CEST,M3.5.0,M10.5.0/3", &SeqIDs{})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestNewProjectCanonicalJSON(t *testing.T) {
	data, err := MarshalProject(newTestProject(t))
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "format": 1,
  "template_version": "1.0.0",
  "name": "Wetterstation",
  "board": "esp32-2432s028r",
  "orientation": "landscape",
  "theme": "night",
  "language": "de",
  "timezone": "CET-1CEST,M3.5.0,M10.5.0/3",
  "settings": {
    "wifi": "optional",
    "aurago_link": false,
    "libraries": [],
    "backlight": 100,
    "statusbar_clock": false
  },
  "start_screen": "s_0001",
  "screens": [
    {
      "id": "s_0001",
      "name": "home",
      "title": "Home",
      "transition": "none",
      "style": {},
      "widgets": []
    }
  ],
  "assets": []
}
`
	if string(data) != want {
		t.Fatalf("canonical JSON =\n%s\nwant\n%s", data, want)
	}
}

func TestNewProjectDefaultsAndErrors(t *testing.T) {
	p, err := NewProject("  Demo  ", "esp32-2432s028r", "clear", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Demo" || p.Language != "en" || p.Timezone != "UTC0" || !validElementID(p.StartScreen) {
		t.Fatalf("defaults: %+v", p)
	}
	cases := []struct {
		name, project, board, theme, lang string
	}{
		{"empty name", " ", "esp32-2432s028r", "clear", "en"},
		{"long name", strings.Repeat("x", 65), "esp32-2432s028r", "clear", "en"},
		{"control char", "a\nb", "esp32-2432s028r", "clear", "en"},
		{"unknown board", "x", "esp32-nope", "clear", "en"},
		{"unknown theme", "x", "esp32-2432s028r", "klar", "en"},
		{"unknown language", "x", "esp32-2432s028r", "clear", "tlh"},
	}
	for _, tc := range cases {
		if _, err := NewProject(tc.project, tc.board, tc.theme, tc.lang, "", &SeqIDs{}); ErrorCode(err) != CodeBadRequest {
			t.Errorf("%s: err = %v, want CYD_BAD_REQUEST", tc.name, err)
		}
	}
}

func sampleLayoutJSON() string {
	return `{
  "format": 1,
  "template_version": "1.0.0",
  "name": "Demo <&>",
  "board": "esp32-2432s028r",
  "orientation": "landscape",
  "theme": "night",
  "language": "de",
  "timezone": "UTC0",
  "settings": {
    "wifi": "optional",
    "aurago_link": false,
    "libraries": [
      "arduinojson"
    ],
    "backlight": 80,
    "statusbar_clock": true
  },
  "start_screen": "s_1",
  "screens": [
    {
      "id": "s_1",
      "name": "home",
      "title": "Start",
      "transition": "fade",
      "style": {
        "bg_color": "#101010",
        "bg_opacity": 200
      },
      "widgets": [
        {
          "id": "w_7f3a",
          "type": "label",
          "name": "lbl_temp",
          "x": 12,
          "y": 40,
          "w": 140,
          "h": 36,
          "props": {
            "align": "center",
            "size": 28,
            "text": "22,5 °C"
          },
          "style": {},
          "events": [
            {
              "on": "clicked",
              "action": {
                "type": "goto_screen",
                "screen": "s_2",
                "transition": "slide_left"
              }
            }
          ]
        },
        {
          "id": "w_p1",
          "type": "panel",
          "x": 0,
          "y": 100,
          "w": 160,
          "h": 100,
          "hidden": true,
          "style": {},
          "children": [
            {
              "id": "w_c1",
              "type": "button",
              "name": "btn_ok",
              "x": 4,
              "y": 4,
              "w": 80,
              "h": 30,
              "style": {
                "radius": 0
              }
            }
          ]
        }
      ]
    },
    {
      "id": "s_2",
      "name": "details",
      "title": "Details",
      "transition": "none",
      "style": {},
      "widgets": []
    }
  ],
  "assets": [
    {
      "id": "a_sun",
      "file": "assets/sun.png",
      "w": 48,
      "h": 48,
      "alpha": true
    }
  ]
}
`
}

func TestParseMarshalRoundTrip(t *testing.T) {
	p, err := ParseProject([]byte(sampleLayoutJSON()))
	if err != nil {
		t.Fatal(err)
	}
	out, err := MarshalProject(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != sampleLayoutJSON() {
		t.Fatalf("round trip changed the document:\n%s", out)
	}
	if p.Screens[0].Widgets[1].Children[0].Style.Radius == nil || *p.Screens[0].Widgets[1].Children[0].Style.Radius != 0 {
		t.Fatal("explicit radius 0 lost")
	}
	bom := append([]byte("\xEF\xBB\xBF"), sampleLayoutJSON()...)
	if _, err := ParseProject(bom); err != nil {
		t.Fatalf("BOM rejected: %v", err)
	}
}

func TestParseProjectRejects(t *testing.T) {
	good := sampleLayoutJSON()
	cases := map[string]string{
		"not json":         "{",
		"unknown field":    strings.Replace(good, `"format": 1,`, `"format": 1, "colour": "x",`, 1),
		"unknown nested":   strings.Replace(good, `"hidden": true,`, `"hidden": true, "opacity": 3,`, 1),
		"wrong type":       strings.Replace(good, `"x": 12,`, `"x": "12",`, 1),
		"trailing data":    good + "{}",
		"format 2":         strings.Replace(good, `"format": 1,`, `"format": 2,`, 1),
		"missing format":   strings.Replace(good, `"format": 1,`, ``, 1),
		"empty widget id":  strings.Replace(good, `"id": "w_c1"`, `"id": ""`, 1),
		"bad screen id":    strings.Replace(good, `"id": "s_2"`, `"id": "s 2"`, 1),
		"bad asset id":     strings.Replace(good, `"id": "a_sun"`, `"id": "../sun"`, 1),
		"oversized layout": good[:len(good)-2] + strings.Repeat(" ", MaxLayoutBytes) + "}\n",
	}
	for name, doc := range cases {
		_, err := ParseProject([]byte(doc))
		if ErrorCode(err) != CodeInvalidLayout {
			t.Errorf("%s: err = %v, want CYD_INVALID_LAYOUT", name, err)
			continue
		}
		if reason, _ := err.(*Error).Details["reason"].(string); reason == "" {
			t.Errorf("%s: details.reason missing", name)
		}
	}
	_, err := ParseProject([]byte(strings.Replace(good, `"id": "w_c1"`, `"id": ""`, 1)))
	if e := asLayoutError(t, err); e != nil && e.Details["path"] != "screens/0/widgets/1/children/0/id" {
		t.Errorf("invalid id path = %v", e.Details["path"])
	}
}

// asLayoutError returns err as *Error CYD_INVALID_LAYOUT, or fails the test and
// returns nil when err is nil or something else.
func asLayoutError(t *testing.T, err error) *Error {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) || e == nil {
		t.Errorf("err = %v, want *Error", err)
		return nil
	}
	if e.Code != CodeInvalidLayout {
		t.Errorf("code = %s, want CYD_INVALID_LAYOUT (%v)", e.Code, e)
		return nil
	}
	return e
}

// layoutWithWidgets wraps a JSON array of widgets into a minimal one-screen layout.
func layoutWithWidgets(widgetsJSON string) string {
	return `{"format":1,"template_version":"1.0.0","name":"x","board":"esp32-2432s028r","orientation":"landscape",` +
		`"theme":"clear","language":"en","timezone":"UTC0","settings":{"wifi":"off","aurago_link":false,"libraries":[],` +
		`"backlight":100,"statusbar_clock":false},"start_screen":"s_1","screens":[{"id":"s_1","name":"home","title":"",` +
		`"transition":"none","style":{},"widgets":` + widgetsJSON + `}],"assets":[]}`
}

// nestedChildren returns a widget array whose single widget has `depth` panel
// ancestors, i.e. depth+1 widget levels.
func nestedChildren(depth int) string {
	w := `{"id":"w_leaf","type":"panel","x":0,"y":0,"w":1,"h":1}`
	for i := 0; i < depth; i++ {
		w = fmt.Sprintf(`{"id":"w_%d","type":"panel","x":0,"y":0,"w":1,"h":1,"children":[%s]}`, i, w)
	}
	return "[" + w + "]"
}

// propsNestedArrays returns a one-widget array whose props value is an array
// nested `levels` deep. Container depth of the value's innermost array is
// 6 (root, screens, screen, widgets, widget, props) + levels.
func propsNestedArrays(levels int) string {
	v := strings.Repeat("[", levels) + strings.Repeat("]", levels)
	return `[{"id":"w_1","type":"label","x":0,"y":0,"w":1,"h":1,"props":{"a":` + v + `}}]`
}

func manyWidgets(n int) string {
	var b strings.Builder
	b.WriteByte('[')
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"id":"w_%d","type":"label","x":0,"y":0,"w":1,"h":1}`, i)
	}
	b.WriteByte(']')
	return b.String()
}

func TestParseProjectBoundsNestingAndArrays(t *testing.T) {
	const depthReason = "nesting deeper than 32 levels"
	const arrayReason = "array with more than 4096 elements"
	reject := map[string]struct {
		doc, reason string
		limit       int
	}{
		"children nested 40 levels":  {layoutWithWidgets(nestedChildren(40)), depthReason, maxLayoutDepth},
		"props value nested 40":      {layoutWithWidgets(propsNestedArrays(40)), depthReason, maxLayoutDepth},
		"props value one level over": {layoutWithWidgets(propsNestedArrays(27)), depthReason, maxLayoutDepth},
		"5000 empty widgets":         {layoutWithWidgets("[" + strings.Repeat("{},", 4999) + "{}]"), arrayReason, maxLayoutArrayElems},
		"4097 widgets":               {layoutWithWidgets(manyWidgets(4097)), arrayReason, maxLayoutArrayElems},
		"deep after BOM":             {"\xEF\xBB\xBF" + layoutWithWidgets(nestedChildren(40)), depthReason, maxLayoutDepth},
	}
	for name, tc := range reject {
		_, err := ParseProject([]byte(tc.doc))
		e := asLayoutError(t, err)
		if e == nil {
			t.Errorf("%s: not rejected as a layout error", name)
			continue
		}
		if reason, _ := e.Details["reason"].(string); reason != tc.reason {
			t.Errorf("%s: reason = %q, want %q", name, reason, tc.reason)
		}
		if e.Details["limit"] != tc.limit {
			t.Errorf("%s: details.limit = %v, want %d", name, e.Details["limit"], tc.limit)
		}
	}

	// Brackets, commas and an escaped quote inside a string must not count; the
	// final \\ is an escaped backslash, so the quote after it closes the string.
	tricky := strings.Repeat("[", 100) + `\"` + strings.Repeat(",", 5000) + strings.Repeat("]", 100) + `\\`
	accept := map[string]string{
		"exactly 32 levels":     layoutWithWidgets(propsNestedArrays(26)),
		"exactly 4096 elements": layoutWithWidgets(manyWidgets(4096)),
		"structure characters inside strings": layoutWithWidgets(
			`[{"id":"w_1","type":"label","x":0,"y":0,"w":1,"h":1,"props":{"text":"` + tricky + `"}}]`),
		"normal layout": sampleLayoutJSON(),
	}
	for name, doc := range accept {
		if _, err := ParseProject([]byte(doc)); err != nil {
			t.Errorf("%s: rejected: %v", name, err)
		}
	}
}

// layoutWithObjectCount builds a layout of exactly total JSON objects spread
// over nine screens (so no array reaches maxLayoutArrayElems). widget is the
// text of one widget object.
func layoutWithObjectCount(total int, widget string) string {
	const screens = 9
	widgets := total - (2 + 2*screens) // root, settings, and a screen plus its style per screen
	var b strings.Builder
	b.WriteString(`{"format":1,"template_version":"1.0.0","name":"x","board":"esp32-2432s028r","orientation":"landscape",` +
		`"theme":"clear","language":"en","timezone":"UTC0","settings":{"wifi":"off","aurago_link":false,"libraries":[],` +
		`"backlight":100,"statusbar_clock":false},"start_screen":"s_0","screens":[`)
	for si := 0; si < screens; si++ {
		n := widgets / screens
		if si < widgets%screens {
			n++
		}
		if si > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"id":"s_%d","name":"s%d","title":"","transition":"none","style":{},"widgets":[`, si, si)
		for wi := 0; wi < n; wi++ {
			if wi > 0 {
				b.WriteByte(',')
			}
			b.WriteString(widget)
		}
		b.WriteString(`]}`)
	}
	b.WriteString(`],"assets":[]}`)
	return b.String()
}

func TestParseProjectBoundsTotalObjects(t *testing.T) {
	// 9 screens x 4000 empty widgets: every array stays under 4096 elements,
	// but the document holds about 36k objects.
	var screens []string
	for i := 0; i < 9; i++ {
		screens = append(screens, fmt.Sprintf(
			`{"id":"s_%d","name":"s%d","title":"","transition":"none","style":{},"widgets":[%s{}]}`,
			i, i, strings.Repeat("{},", 3999)))
	}
	doc := `{"format":1,"template_version":"1.0.0","name":"x","board":"esp32-2432s028r","orientation":"landscape",` +
		`"theme":"clear","language":"en","timezone":"UTC0","settings":{"wifi":"off","aurago_link":false,"libraries":[],` +
		`"backlight":100,"statusbar_clock":false},"start_screen":"s_0","screens":[` + strings.Join(screens, ",") + `],"assets":[]}`
	_, err := ParseProject([]byte(doc))
	e := asLayoutError(t, err)
	if e == nil {
		t.Fatal("36k objects not rejected as a layout error")
	}
	if reason, _ := e.Details["reason"].(string); reason != "more than 32768 objects" {
		t.Errorf("reason = %q", reason)
	}
	if e.Details["limit"] != maxLayoutObjects {
		t.Errorf("details.limit = %v, want %d", e.Details["limit"], maxLayoutObjects)
	}

	// Exactly maxLayoutObjects objects pass the scan; one more does not. Braces
	// inside strings never count.
	const widget = `{"id":"w_1"}`
	if _, err := ParseProject([]byte(layoutWithObjectCount(maxLayoutObjects, widget))); err != nil {
		t.Errorf("exactly %d objects rejected: %v", maxLayoutObjects, err)
	}
	_, err = ParseProject([]byte(layoutWithObjectCount(maxLayoutObjects+1, widget)))
	if e := asLayoutError(t, err); e != nil {
		if reason, _ := e.Details["reason"].(string); reason != "more than 32768 objects" {
			t.Errorf("one over: reason = %q", reason)
		}
	}
	braces := layoutWithWidgets(`[{"id":"w_1","type":"label","x":0,"y":0,"w":1,"h":1,"props":{"text":"` +
		strings.Repeat("{", maxLayoutObjects+10) + `"}}]`)
	if _, err := ParseProject([]byte(braces)); err != nil {
		t.Errorf("braces inside a string counted as objects: %v", err)
	}
}

func TestParseProjectMalformedPastShapeScanStillRejected(t *testing.T) {
	// Mismatched brackets and a dangling string pass the shape scan, never the decoder.
	for name, doc := range map[string]string{
		"mismatched":        `{"format":1,"screens":[}`,
		"unterminated":      `{"format":1,"name":"abc`,
		"commas outside []": `{"format":1,,,,}`,
	} {
		_, err := ParseProject([]byte(doc))
		if e := asLayoutError(t, err); e == nil {
			t.Errorf("%s: not rejected as a layout error", name)
		}
	}
}

func TestMarshalProjectRejectsOversizeCanonical(t *testing.T) {
	text := strings.Repeat("x", 300)
	p := newTestProject(t)
	p.Screens = nil
	for si := 0; si < MaxScreens; si++ {
		s := Screen{ID: fmt.Sprintf("s_%d", si), Name: fmt.Sprintf("s%d", si)}
		for wi := 0; wi < MaxWidgetsPerScreen; wi++ {
			s.Widgets = append(s.Widgets, Widget{ID: fmt.Sprintf("w_%d_%d", si, wi), Type: "label", W: 1, H: 1,
				Props: map[string]any{"text": text}})
		}
		p.Screens = append(p.Screens, s)
	}
	out, err := MarshalProject(p)
	e := asLayoutError(t, err)
	if e == nil {
		t.Fatalf("oversize canonical layout accepted (%d bytes)", len(out))
	}
	if out != nil {
		t.Errorf("output returned alongside the error: %d bytes", len(out))
	}
	bytesGot, _ := e.Details["bytes"].(int)
	if bytesGot <= MaxLayoutBytes || e.Details["limit"] != MaxLayoutBytes {
		t.Errorf("details = %v, want bytes > %d and limit %d", e.Details, MaxLayoutBytes, MaxLayoutBytes)
	}
	if reason, _ := e.Details["reason"].(string); !strings.HasPrefix(reason, "canonical layout is ") {
		t.Errorf("reason = %q", reason)
	}

	// The same shape without the long texts is a realistic maximum and must
	// marshal and parse back.
	for si := range p.Screens {
		for wi := range p.Screens[si].Widgets {
			p.Screens[si].Widgets[wi].Props = nil
		}
	}
	out, err = MarshalProject(p)
	if err != nil {
		t.Fatalf("max-shape layout without props: %v", err)
	}
	if _, err := ParseProject(out); err != nil {
		t.Fatalf("max-shape layout does not parse back (%d bytes): %v", len(out), err)
	}
}

func TestLayoutErrorClipsReason(t *testing.T) {
	// An unknown key is echoed by the decoder; a clipped reason must stay valid
	// UTF-8 even when the cut falls inside a multi-byte rune.
	for name, key := range map[string]string{
		"ascii key":      strings.Repeat("k", 1_000_000),
		"two-byte key":   strings.Repeat("ä", 400_000),
		"three-byte key": strings.Repeat("€", 300_000),
	} {
		doc := `{"format":1,"` + key + `":1}`
		if len(doc) > MaxLayoutBytes {
			t.Fatalf("%s: test document exceeds the layout limit", name)
		}
		_, err := ParseProject([]byte(doc))
		e := asLayoutError(t, err)
		if e == nil {
			continue
		}
		reason, _ := e.Details["reason"].(string)
		if len(reason) > maxDiagMessageBytes+len("…") || !strings.HasSuffix(reason, "…") || !utf8.ValidString(reason) {
			t.Errorf("%s: reason len %d, suffix %q, valid utf8 %v", name, len(reason), reason[len(reason)-3:], utf8.ValidString(reason))
		}
		if len(e.Message) > len("invalid layout: ")+maxDiagMessageBytes+len("…") || !utf8.ValidString(e.Message) {
			t.Errorf("%s: message len %d, valid utf8 %v", name, len(e.Message), utf8.ValidString(e.Message))
		}
	}
	// Short reasons pass through untouched.
	if got := clipReason("short"); got != "short" {
		t.Errorf("clipReason(short) = %q", got)
	}
}

func TestMarshalProjectSortsPropsKeysAndIsIdempotent(t *testing.T) {
	doc := layoutWithWidgets(`[{"id":"w_1","type":"label","x":1,"y":2,"w":3,"h":4,` +
		`"props":{"zeta":1,"alpha":{"y":2,"b":[{"q":1,"a":2}],"a":3}}}]`)
	p, err := ParseProject([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	out, err := MarshalProject(p)
	if err != nil {
		t.Fatal(err)
	}
	wantProps := `          "props": {
            "alpha": {
              "a": 3,
              "b": [
                {
                  "a": 2,
                  "q": 1
                }
              ],
              "y": 2
            },
            "zeta": 1
          },
`
	if !strings.Contains(string(out), wantProps) {
		t.Fatalf("props keys not sorted canonically:\n%s", out)
	}
	again, err := ParseProject(out)
	if err != nil {
		t.Fatal(err)
	}
	out2, err := MarshalProject(again)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, out2) {
		t.Fatalf("canonical form is not idempotent:\n%s\nvs\n%s", out, out2)
	}
}

func TestParseProjectNormalizesNulls(t *testing.T) {
	doc := `{"format":1,"template_version":"1.0.0","name":"x","board":"esp32-2432s028r","orientation":"landscape",
	"theme":"clear","language":"en","timezone":"UTC0","settings":{"wifi":"off","aurago_link":false,"libraries":null,
	"backlight":100,"statusbar_clock":false},"start_screen":"s_1","screens":[{"id":"s_1","name":"home","title":"",
	"transition":"none","style":{},"widgets":null}],"assets":null}`
	p, err := ParseProject([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if p.Assets == nil || p.Settings.Libraries == nil || p.Screens[0].Widgets == nil {
		t.Fatalf("nil slices survived: %+v", p)
	}
}

func TestMarshalProjectDoesNotMutateInput(t *testing.T) {
	p := Project{Format: 1, Screens: []Screen{{ID: "s_1", Name: "home"}}}
	out, err := MarshalProject(p)
	if err != nil {
		t.Fatal(err)
	}
	if p.Screens[0].Widgets != nil || p.Assets != nil {
		t.Fatal("MarshalProject modified its input")
	}
	if !bytes.Contains(out, []byte(`"widgets": []`)) || !bytes.HasSuffix(out, []byte("}\n")) {
		t.Fatalf("output:\n%s", out)
	}
}

func TestFindWidget(t *testing.T) {
	p, err := ParseProject([]byte(sampleLayoutJSON()))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		key    string
		screen int
		path   []int
		ok     bool
	}{
		{"w_7f3a", 0, []int{0}, true},
		{"lbl_temp", 0, []int{0}, true},
		{"w_c1", 0, []int{1, 0}, true},
		{"btn_ok", 0, []int{1, 0}, true},
		{"nope", -1, nil, false},
		{"", -1, nil, false},
	}
	for _, tc := range cases {
		si, path, ok := p.FindWidget(tc.key)
		if si != tc.screen || !reflect.DeepEqual(path, tc.path) || ok != tc.ok {
			t.Errorf("FindWidget(%q) = %d %v %v, want %d %v %v", tc.key, si, path, ok, tc.screen, tc.path, tc.ok)
		}
	}
	// ids win over names: a widget named like another widget's id
	p.Screens[1].Widgets = append(p.Screens[1].Widgets, Widget{ID: "w_x", Type: "label", Name: "w_c1"})
	if si, path, _ := p.FindWidget("w_c1"); si != 0 || !reflect.DeepEqual(path, []int{1, 0}) {
		t.Errorf("id lookup did not win: %d %v", si, path)
	}
}

func TestWalkWidgets(t *testing.T) {
	p, err := ParseProject([]byte(sampleLayoutJSON()))
	if err != nil {
		t.Fatal(err)
	}
	var visited []string
	p.WalkWidgets(func(s *Screen, parent *Widget, w *Widget) bool {
		pid := ""
		if parent != nil {
			pid = parent.ID
		}
		visited = append(visited, s.ID+"/"+pid+"/"+w.ID)
		return true
	})
	want := []string{"s_1//w_7f3a", "s_1//w_p1", "s_1/w_p1/w_c1"}
	if !reflect.DeepEqual(visited, want) {
		t.Fatalf("visited %v, want %v", visited, want)
	}
	count := 0
	p.WalkWidgets(func(*Screen, *Widget, *Widget) bool { count++; return false })
	if count != 1 {
		t.Fatalf("walk did not stop: %d", count)
	}
	p.WalkWidgets(func(_ *Screen, _ *Widget, w *Widget) bool { w.X += 1; return true })
	if p.Screens[0].Widgets[1].Children[0].X != 5 {
		t.Fatal("WalkWidgets pointers do not alias the project")
	}
}
