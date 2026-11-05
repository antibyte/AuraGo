package render

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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

var (
	pieceHashA = strings.Repeat("a", 64)
	pieceHashB = strings.Repeat("b", 64)
	pieceHashC = strings.Repeat("c", 64)
	pieceHashD = strings.Repeat("d", 64)
)

// measuredReport is goodReport with a three-piece narration in s1 and a one-piece dialogue in s2.
// goodReport has 300 frames at 30 fps, so filmMS is round(300*1000/30) = 10000.
func measuredReport() map[string]any {
	m := goodReport()
	m["audio_events"] = []any{
		map[string]any{"kind": "narration", "ref": pieceHashA, "start_ms": 500, "scene_id": "s1", "block_id": "s1:n", "piece": 0},
		map[string]any{"kind": "narration", "ref": pieceHashB, "start_ms": 1500, "scene_id": "s1", "block_id": "s1:n", "piece": 1},
		map[string]any{"kind": "narration", "ref": pieceHashC, "start_ms": 2500, "scene_id": "s1", "block_id": "s1:n", "piece": 2},
		map[string]any{"kind": "dialogue", "ref": pieceHashD, "start_ms": 5000, "scene_id": "s2", "block_id": "s2:b1", "piece": 0},
	}
	m["captions"] = []any{
		map[string]any{"start_ms": 500, "end_ms": 1500, "lines": []any{"Der erste Satz."}, "scene_id": "s1", "block_id": "s1:n"},
		map[string]any{"start_ms": 1500, "end_ms": 3500, "lines": []any{"Der zweite Satz.", "Der dritte"}, "scene_id": "s1", "block_id": "s1:n"},
	}
	return m
}

func measuredExpect() reportExpect {
	want := reportExpectation()
	want.Blocks = map[string]reportBlock{
		"s1:n":  {SceneID: "s1", Kind: "narration", Hashes: []string{pieceHashA, pieceHashB, pieceHashC}, Text: captionKey("Der erste Satz. Der zweite Satz. Der dritte Satz.")},
		"s2:b1": {SceneID: "s2", Kind: "dialogue", Hashes: []string{pieceHashD}, Text: captionKey("Hallo!")},
	}
	want.MaxEvents = 4 // three pieces of s1:n plus one of s2:b1
	return want
}

func measuredEvents(m map[string]any) []any   { return m["audio_events"].([]any) }
func measuredCaptions(m map[string]any) []any { return m["captions"].([]any) }
func measuredEvent(m map[string]any, i int) map[string]any {
	return measuredEvents(m)[i].(map[string]any)
}
func measuredCaption(m map[string]any, i int) map[string]any {
	return measuredCaptions(m)[i].(map[string]any)
}

func TestParseReportRefusesBadAudioEventsAndCaptions(t *testing.T) {
	cases := []struct {
		name string
		mut  func(m map[string]any)
		text string
	}{
		{"unknown block", func(m map[string]any) { measuredEvent(m, 0)["block_id"] = "s9:n" }, "unknown block"},
		{"wrong hash", func(m map[string]any) { measuredEvent(m, 1)["ref"] = pieceHashA }, "does not match"},
		{"duplicate piece", func(m map[string]any) {
			measuredEvent(m, 1)["piece"] = 0
			measuredEvent(m, 1)["ref"] = pieceHashA
		}, "twice"},
		{"gap in the piece prefix", func(m map[string]any) {
			e := measuredEvents(m)
			m["audio_events"] = []any{e[0], e[2], e[3]}
		}, "gap"},
		{"a narration block with no event", func(m map[string]any) { m["audio_events"] = measuredEvents(m)[3:] }, "no audio event"},
		{"empty audio_events with measured blocks", func(m map[string]any) { m["audio_events"] = []any{} }, "no audio event"},
		{"unknown kind", func(m map[string]any) { measuredEvent(m, 0)["kind"] = "music" }, "unknown kind"},
		// filmMS is 10000 and the bound is strict.
		{"a time at the film end", func(m map[string]any) { measuredEvent(m, 3)["start_ms"] = 10000 }, "outside the film"},
		// |10002-10000| = 2, over the tolerance of 1.
		{"duration_ms off by 2", func(m map[string]any) { m["duration_ms"] = 10002 }, "duration_ms"},
		{"over MaxEvents", func(m map[string]any) {
			m["audio_events"] = append(measuredEvents(m), map[string]any{"kind": "narration", "ref": pieceHashA, "start_ms": 600, "scene_id": "s1", "block_id": "s1:n", "piece": 0})
		}, "too many"},
		{"over 8000 captions", func(m map[string]any) {
			caps := make([]any, maxReportCaptions+1)
			for i := range caps {
				caps[i] = measuredCaption(m, 0)
			}
			m["captions"] = caps
		}, "8001 captions"},
		{"a caption with 0 lines", func(m map[string]any) { measuredCaption(m, 0)["lines"] = []any{} }, "1 or 2 lines"},
		{"an empty line", func(m map[string]any) { measuredCaption(m, 0)["lines"] = []any{" \t"} }, "empty line"},
		// 257 UTF-16 units, one over the cap.
		{"a long line", func(m map[string]any) { measuredCaption(m, 0)["lines"] = []any{strings.Repeat("a", 257)} }, "256"},
		{"a caption not in its block", func(m map[string]any) { measuredCaption(m, 0)["lines"] = []any{"Ein fremder Satz."} }, "not in the text"},
		{"a caption of an unknown block", func(m map[string]any) { measuredCaption(m, 0)["block_id"] = "s9:n" }, "unknown block"},
		{"a caption past the film", func(m map[string]any) { measuredCaption(m, 1)["end_ms"] = 10001 }, "outside the film"},
	}
	for _, tc := range cases {
		m := measuredReport()
		tc.mut(m)
		body, _ := json.Marshal(m)
		_, refusal := parseReport(body, measuredExpect())
		if refusal == nil || refusal.Code != CodeRenderFailed || !strings.Contains(refusal.Message, tc.text) {
			t.Errorf("%s: refusal = %+v", tc.name, refusal)
		}
	}
}

func TestParseReportAcceptsHonestAudioReports(t *testing.T) {
	accept := func(name string, want reportExpect, mut func(m map[string]any)) compileReport {
		t.Helper()
		m := measuredReport()
		if mut != nil {
			mut(m)
		}
		body, _ := json.Marshal(m)
		rep, refusal := parseReport(body, want)
		if refusal != nil {
			t.Errorf("%s: refused: %v", name, refusal)
		}
		return rep
	}
	if rep := accept("the full report", measuredExpect(), nil); len(rep.AudioEvents) != 4 || len(rep.Captions) != 2 || rep.AudioEvents[3].BlockID != "s2:b1" {
		t.Errorf("rep = %+v", rep)
	}
	accept("duration_ms off by 1", measuredExpect(), func(m map[string]any) { m["duration_ms"] = 10001 })
	accept("a dialogue block with no event", measuredExpect(), func(m map[string]any) { m["audio_events"] = measuredEvents(m)[:3] })
	accept("a film end that dropped the last piece", measuredExpect(), func(m map[string]any) {
		e := measuredEvents(m)
		m["audio_events"] = []any{e[0], e[1], e[3]}
	})
	accept("a JS gain", measuredExpect(), func(m map[string]any) { measuredEvent(m, 0)["gain"] = 99 })

	withControl := measuredExpect()
	block := withControl.Blocks["s1:n"]
	block.Text = captionKey("Der erste\x01 Satz. Der zweite Satz. Der dritte Satz.")
	withControl.Blocks["s1:n"] = block
	accept("a narration with U+0001", withControl, func(m map[string]any) {
		measuredCaption(m, 0)["lines"] = []any{"Der erste\x01 Satz."}
	})

	estimated := measuredExpect()
	estimated.CaptionText = map[string]string{"s3:n": captionKey("Noch ohne Audio.")}
	accept("a caption of an estimated block", estimated, func(m map[string]any) {
		m["captions"] = append(measuredCaptions(m), map[string]any{"start_ms": 8000, "end_ms": 9000, "lines": []any{"Noch ohne Audio."}, "scene_id": "s3", "block_id": "s3:n"})
	})
	// An estimated block still takes no audio event.
	m := measuredReport()
	event := measuredEvent(m, 3)
	event["kind"], event["block_id"], event["scene_id"] = "narration", "s3:n", "s3"
	body, _ := json.Marshal(m)
	if _, refusal := parseReport(body, estimated); refusal == nil || !strings.Contains(refusal.Message, "unknown block") {
		t.Errorf("an event of an estimated block: %+v", refusal)
	}
}

func TestCaptionKeyRemovesJSSpaceAndControls(t *testing.T) {
	// \xc2\xa0 is U+00A0 and \xe3\x80\x80 is U+3000, both JS whitespace.
	if got := captionKey(" Der\xc2\xa0erste\x01\tSatz\xe3\x80\x80. "); got != "DerersteSatz." {
		t.Errorf("captionKey = %q", got)
	}
}

// The plan's own blocks reach parseReport: a report without events cannot defeat a measured narration.
func TestRunRefusesAReportThatDropsMeasuredNarration(t *testing.T) {
	fx := newRunFixture(t)
	ctx := context.Background()
	if _, err := fx.projects.SaveStory(ctx, fx.projectID, []byte(narratedStoryJSON), ""); err != nil {
		t.Fatal(err)
	}
	plan, err := fx.runner.Prepare(ctx, fx.projectID, "", fx.settings)
	if err != nil {
		t.Fatal(err)
	}
	seedPieces(t, fx, plan.Blocks)
	fx.playing(playOptions{frames: 12}) // the fake page posts an empty audio_events
	job := fx.render(t)
	if job.Status != StatusFailed || job.Error == nil || job.Error.Code != CodeRenderFailed || !strings.Contains(job.Error.Message, "no audio event") {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
}

// The page fills captions for estimated blocks too, and a narrated story without audio still exports silently.
func TestRunAcceptsCaptionsOfAnEstimatedBlock(t *testing.T) {
	fx := newRunFixture(t)
	if _, err := fx.projects.SaveStory(context.Background(), fx.projectID, []byte(narratedStoryJSON), ""); err != nil {
		t.Fatal(err)
	}
	// 12 frames at 30 fps give duration_ms 400, so the caption ends with the film.
	fx.playing(playOptions{frames: 12, mutateRep: func(rep map[string]any) {
		rep["captions"] = []any{map[string]any{"start_ms": 0, "end_ms": 400, "lines": []any{"Hallo Welt, das ist der erste Satz."}, "scene_id": "s1", "block_id": "s1:n"}}
	}})
	if job := fx.render(t); job.Status != StatusSucceeded {
		t.Fatalf("job = %+v %+v", job, job.Error)
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

// The same file is pinned by scripts/storymaker-tests/04-compile.mjs, so this is the wire check between compile.js and parseReport.
func TestParseReportAcceptsTheMeasuredFixture(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "report_measured.json"))
	if err != nil {
		t.Fatal(err)
	}
	hash := func(c string) string { return strings.Repeat(c, 64) }
	rep, refusal := parseReport(body, reportExpect{
		FPS: 30, Width: 1920, Height: 1080, MaxFrames: 27000, CatalogVersion: "mini00000000", SceneIDs: []string{"s1", "s2"},
		MaxEvents: 3,
		Blocks: map[string]reportBlock{
			"s1:n": {SceneID: "s1", Kind: "narration", Hashes: []string{hash("a"), hash("b")}, Text: captionKey("Das Paket kommt heute an. Die Firewall prueft es genau.")},
			"s2:n": {SceneID: "s2", Kind: "narration", Hashes: []string{hash("c")}, Text: captionKey("Das ist das Ende vom Film.")},
		},
	})
	if refusal != nil {
		t.Fatalf("the fixture was refused: %+v", refusal)
	}
	if len(rep.AudioEvents) != 3 || rep.AudioEvents[1].Piece != 1 || len(rep.Captions) != 2 || rep.Captions[1].BlockID != "s2:n" {
		t.Fatalf("parsed report = %+v", rep)
	}
	if rep.CuePrecision == nil || *rep.CuePrecision != 1 {
		t.Fatalf("cue_precision = %v", rep.CuePrecision)
	}
}

// A text the page wrote must reach the refusal without the render prefix and the token, also when a clip would
// have cut through the prefix: the scrub runs before the clip.
func TestParseReportRefusalsCarryNoSecrets(t *testing.T) {
	const prefix, token = "/pfx-9Zq4Lm7Vw2Xk8", "tok-Hd3Rt6Yb1Nc5"
	want := reportExpectation()
	want.Prefix, want.Token = prefix, token
	// The prefix starts about 390 bytes into the text, so a clip at 400 would keep only a part of it.
	cutting := strings.Repeat("x", 360) + "http://127.0.0.1:5555" + prefix + "/story.json " + token
	cases := map[string]func(m map[string]any){
		"issue message": func(m map[string]any) {
			m["issues"] = []any{map[string]any{"severity": "error", "scene_id": "s3", "code": "code_error", "message": "fetch failed: http://127.0.0.1:5555" + prefix + "/x?t=" + token}}
		},
		"issue scene id": func(m map[string]any) {
			m["issues"] = []any{map[string]any{"severity": "error", "scene_id": "http://127.0.0.1:5555" + prefix, "code": "unknown_asset", "message": "bad"}}
		},
		"issue cut by clip": func(m map[string]any) {
			m["issues"] = []any{map[string]any{"severity": "error", "scene_id": "s3", "code": "code_error", "message": cutting}}
		},
		"catalog version": func(m map[string]any) { m["catalog_version"] = prefix + strings.Repeat("v", 600) + token },
		"catalog version cut by clip": func(m map[string]any) {
			m["catalog_version"] = strings.Repeat("v", 345) + prefix + token
		},
		"scene id": func(m map[string]any) { m["scenes"].([]any)[0].(map[string]any)["id"] = prefix + token },
	}
	for name, mut := range cases {
		m := goodReport()
		mut(m)
		body, _ := json.Marshal(m)
		_, refusal := parseReport(body, want)
		if refusal == nil {
			t.Errorf("%s: not refused", name)
			continue
		}
		if len(refusal.Message) > 460 {
			t.Errorf("%s: message has %d bytes, it is not clipped", name, len(refusal.Message))
		}
		for _, frag := range []string{"pfx-", "9Zq4", "Lm7V", "Xk8", "tok-", "Hd3R", "Yb1N", "Rt6Y"} {
			if strings.Contains(refusal.Message, frag) {
				t.Errorf("%s: the message keeps %q: %q", name, frag, refusal.Message)
			}
		}
	}

	// The same text through the run hook reaches neither the job record nor the answer.
	run := &renderRun{r: &Runner{}, prefix: prefix, token: token, fps: 30, width: 1920, height: 1080, frameLimit: 27000, catVersion: "70d5cf51a9a9",
		plan: &Plan{}, failCh: make(chan *JobError, 1)}
	m := goodReport()
	cases["issue cut by clip"](m)
	body, _ := json.Marshal(m)
	ans := run.onReport(body)
	failed := <-run.failCh
	if ans.Start || failed == nil || strings.Contains(ans.Message+failed.Message, "Zq4") || strings.Contains(ans.Message+failed.Message, "Hd3R") || failed.Message != ans.Message {
		t.Errorf("onReport: answer %+v, job error %+v", ans, failed)
	}
}
