package render

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-rod/rod/lib/proto"
)

func goodReport() map[string]any {
	return map[string]any{
		"frame_count": 300, "fps": 30, "width": 1920, "height": 1080, "duration_ms": 10000, "catalog_version": "70d5cf51a9a9",
		"scenes": []any{
			map[string]any{"id": "s1", "start_frame": 0, "end_frame": 130, "kind": "storyboard"},
			map[string]any{"id": "s2", "start_frame": 115, "end_frame": 210, "kind": "storyboard"},
			map[string]any{"id": "s3", "start_frame": 195, "end_frame": 300, "kind": "code"},
		},
		"audio_events": []any{}, "captions": []any{}, "checks": []any{}, "unverified": []any{},
		"issues": []any{map[string]any{"severity": "warning", "path": "/scenes/0/narration", "code": "narration_missing", "message": "estimated"}},
	}
}

func reportExpectation() reportExpect {
	return reportExpect{FPS: 30, Width: 1920, Height: 1080, MaxFrames: 27000, CatalogVersion: "70d5cf51a9a9", SceneIDs: []string{"s1", "s2", "s3"}}
}

func TestParseReportAcceptsAConsistentReport(t *testing.T) {
	body, _ := json.Marshal(goodReport())
	rep, refusal := parseReport(body, reportExpectation())
	if refusal != nil || rep.FrameCount != 300 || len(rep.Scenes) != 3 || len(rep.Issues) != 1 {
		t.Fatalf("parseReport = %+v, %v", rep, refusal)
	}
	// Unknown fields are ignored.
	m := goodReport()
	m["future_field"] = map[string]any{"x": 1}
	body, _ = json.Marshal(m)
	if _, refusal := parseReport(body, reportExpectation()); refusal != nil {
		t.Fatalf("unknown field refused: %v", refusal)
	}
}

func TestParseReportRefusals(t *testing.T) {
	scenes := func(m map[string]any) []any { return m["scenes"].([]any) }
	scene := func(m map[string]any, i int) map[string]any { return scenes(m)[i].(map[string]any) }
	cases := map[string]struct {
		mut  func(m map[string]any)
		code string
		text string
	}{
		"fps":                {func(m map[string]any) { m["fps"] = 24 }, CodeRenderFailed, "24 fps"},
		"width":              {func(m map[string]any) { m["width"] = 1280 }, CodeRenderFailed, "1280x1080"},
		"catalog version":    {func(m map[string]any) { m["catalog_version"] = "other" }, CodeRenderFailed, "catalog version"},
		"too many frames":    {func(m map[string]any) { m["frame_count"] = 27001; scene(m, 2)["end_frame"] = 27001 }, CodeInvalidStory, "27001 frames"},
		"no frames":          {func(m map[string]any) { m["frame_count"] = 0 }, CodeRenderFailed, "no frames"},
		"scene count":        {func(m map[string]any) { m["scenes"] = scenes(m)[:2] }, CodeRenderFailed, "2 scenes"},
		"scene order":        {func(m map[string]any) { scene(m, 0)["id"], scene(m, 1)["id"] = "s2", "s1" }, CodeRenderFailed, `scene 0 is "s2"`},
		"empty range":        {func(m map[string]any) { scene(m, 1)["end_frame"] = 115 }, CodeRenderFailed, "invalid range"},
		"past the end":       {func(m map[string]any) { scene(m, 1)["end_frame"] = 301 }, CodeRenderFailed, "invalid range"},
		"negative start":     {func(m map[string]any) { scene(m, 0)["start_frame"] = -1 }, CodeRenderFailed, "invalid range"},
		"start not forward":  {func(m map[string]any) { scene(m, 1)["start_frame"] = 0 }, CodeRenderFailed, "not monotonic"},
		"frame count vs end": {func(m map[string]any) { m["frame_count"] = 310 }, CodeRenderFailed, "last scene end"},
		"error issue": {func(m map[string]any) {
			m["issues"] = []any{map[string]any{"severity": "error", "scene_id": "s2", "code": "unknown_asset", "message": "no such prop"}}
		}, CodeInvalidStory, "Scene s2: no such prop"},
		"code error": {func(m map[string]any) {
			m["issues"] = []any{map[string]any{"severity": "error", "scene_id": "s3", "code": "deadline", "message": "scene took too long"}}
		}, CodeCodeError, "too long"},
	}
	for name, tc := range cases {
		m := goodReport()
		tc.mut(m)
		body, _ := json.Marshal(m)
		_, refusal := parseReport(body, reportExpectation())
		if refusal == nil || refusal.Code != tc.code || !strings.Contains(refusal.Message, tc.text) {
			t.Errorf("%s: refusal = %+v", name, refusal)
		}
	}
	// An end that goes back behind the previous scene's end is not monotonic; equal ends are fine.
	m := goodReport()
	scene(m, 0)["end_frame"] = 220
	scene(m, 1)["end_frame"] = 210
	body, _ := json.Marshal(m)
	if _, refusal := parseReport(body, reportExpectation()); refusal == nil || !strings.Contains(refusal.Message, "not monotonic") {
		t.Errorf("end going back: %v", refusal)
	}
	if _, refusal := parseReport([]byte(`{"frame_count":`), reportExpectation()); refusal == nil || refusal.Code != CodeRenderFailed {
		t.Errorf("broken JSON: %v", refusal)
	}
}

func TestEgressGuardDecisions(t *testing.T) {
	g := newEgressGuard("http://127.0.0.1:41000/0123456789abcdef0123456789abcdef")
	base := "http://127.0.0.1:41000/0123456789abcdef0123456789abcdef"
	for _, tc := range []struct {
		url      string
		resource proto.NetworkResourceType
		want     bool
	}{
		{base + "/", proto.NetworkResourceTypeDocument, true},
		{base + "/boot.json", proto.NetworkResourceTypeFetch, true},
		{base + "/frame?n=3", proto.NetworkResourceTypeFetch, true},
		{base + "/js/storymaker/host.bundle.js", proto.NetworkResourceTypeScript, true},
		{"blob:null/6c1f", proto.NetworkResourceTypeScript, true},
		{"data:text/plain;base64,AAAA", proto.NetworkResourceTypeFetch, true},
		{"blob:null/6c1f", proto.NetworkResourceTypeDocument, false},
		{"data:text/html,<script>fetch(1)</script>", proto.NetworkResourceTypeDocument, false},
		{"http://127.0.0.1:41000/", proto.NetworkResourceTypeDocument, false},
		{"http://127.0.0.1:41000/favicon.ico", proto.NetworkResourceTypeImage, false},
		{"http://127.0.0.1:41001/0123456789abcdef0123456789abcdef/", proto.NetworkResourceTypeDocument, false},
		{"http://127.0.0.1:41000/0123456789abcdef0123456789abcdee/", proto.NetworkResourceTypeFetch, false},
		{base, proto.NetworkResourceTypeDocument, false}, // no trailing slash: not the base's page
		{"http://127.0.0.1:41000@evil.example/", proto.NetworkResourceTypeDocument, false},
		{"https://example.com/", proto.NetworkResourceTypeFetch, false},
		{"ws://127.0.0.1:41000/0123456789abcdef0123456789abcdef/x", proto.NetworkResourceTypeWebSocket, false},
		{"file:///etc/passwd", proto.NetworkResourceTypeDocument, false},
		{"", proto.NetworkResourceTypeFetch, false},
	} {
		if got := g.allowed(tc.url, tc.resource); got != tc.want {
			t.Errorf("allowed(%q, %s) = %v, want %v", tc.url, tc.resource, got, tc.want)
		}
	}
	g.noteBlocked("https://example.com/secret?token=1")
	g.noteBlocked("not a url at all, but long enough to be clipped somewhere around forty characters")
	if g.Blocked() != 2 || len(g.BlockedHosts()) != 2 || g.BlockedHosts()[0] != "https://example.com" || len(g.BlockedHosts()[1]) > 40 {
		t.Errorf("blocked = %d %v", g.Blocked(), g.BlockedHosts())
	}
	patterns := g.blockPatterns()
	if patterns[0]["urlPattern"] != base+"/*" || patterns[0]["block"] != false {
		t.Errorf("the allow rule must come first: %v", patterns[0])
	}
	for _, p := range patterns[1:] {
		if p["block"] != true {
			t.Errorf("%v does not block", p)
		}
	}
}
