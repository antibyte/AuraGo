package videostudio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestValidateMarkersAndInOut(t *testing.T) {
	base := func() Project {
		project := validProject()
		project.Markers = []Marker{{ID: "m1", Frame: 10}, {ID: "m2", Frame: 20, Duration: 30, Name: "Range", Color: "teal", Note: "note"}}
		in, out := 5, 100
		project.InPoint, project.OutPoint = &in, &out
		return project
	}
	if err := Validate(base()); err != nil {
		t.Fatalf("valid markers rejected: %v", err)
	}
	cases := map[string]func(*Project){
		"duplicate id":         func(p *Project) { p.Markers[1].ID = "m1" },
		"negative frame":       func(p *Project) { p.Markers[0].Frame = -1 },
		"range past the limit": func(p *Project) { p.Markers[1].Duration = MaxDurationFrames(*p) },
		"unknown colour":       func(p *Project) { p.Markers[0].Color = "mauve" },
		"long name":            func(p *Project) { p.Markers[0].Name = strings.Repeat("n", MaxMarkerNameRunes+1) },
		"long note":            func(p *Project) { p.Markers[0].Note = strings.Repeat("n", MaxMarkerNoteRunes+1) },
		"too many markers": func(p *Project) {
			p.Markers = nil
			for i := 0; i <= MaxMarkers; i++ {
				p.Markers = append(p.Markers, Marker{ID: fmt.Sprintf("m%d", i), Frame: i})
			}
		},
		"in not before out": func(p *Project) { in := 100; p.InPoint = &in },
		"out at zero":       func(p *Project) { out := 0; p.OutPoint = &out },
		"in past the end":   func(p *Project) { in := MaxDurationFrames(*p); p.InPoint = &in },
	}
	for name, mutate := range cases {
		project := base()
		mutate(&project)
		if err := Validate(project); err == nil {
			t.Errorf("%s: Validate accepted the project", name)
		}
	}
}

func TestMarkerJSONFollowsOmitEmpty(t *testing.T) {
	project := validProject()
	data, err := json.Marshal(project)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("markers")) || bytes.Contains(data, []byte("in_point")) {
		t.Fatalf("empty marker fields were serialized: %s", data)
	}
	zero := 0
	project.InPoint = &zero
	project.Markers = []Marker{{ID: "m", Frame: 0}}
	data, err = json.Marshal(project)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"in_point":0`)) || !bytes.Contains(data, []byte(`"markers":[{"id":"m","frame":0}]`)) {
		t.Fatalf("an in point of 0 or a point marker was dropped: %s", data)
	}
}
