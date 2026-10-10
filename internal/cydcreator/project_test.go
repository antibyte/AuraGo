package cydcreator

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
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
	if got := err.(*Error).Details["path"]; got != "screens/0/widgets/1/children/0/id" {
		t.Errorf("invalid id path = %v", got)
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
