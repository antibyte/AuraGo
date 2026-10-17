package ui

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The range a range check renders (index §4.15): in/out clamped to the enabled end, else 5 s around the playhead;
// more than 60 s is refused, never clamped. Also the failure strings of the new job codes.
func TestVideoStudioRangeRenderRange(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Video Studio range checks")
	}
	script := `
const assert = require('node:assert/strict');
global.window = {};
require('./js/desktop/apps/video-studio-time.js');
require('./js/desktop/apps/video-studio-effects.js');
require('./js/desktop/apps/video-studio-timeline.js');
require('./js/desktop/apps/video-studio-core.js');
require('./js/desktop/apps/video-studio-jobs.js');
require('./js/desktop/apps/video-studio-transport.js');
require('./js/desktop/apps/video-studio-rangerender.js');
const P = window.VideoStudioParts;
const clip = (start, duration, extra) => Object.assign({id: 'c' + start, asset_id: 'a', start, offset: 0, duration}, extra || {});
const session = (clips, project, frame) => ({frame: frame || 0, project: Object.assign({fps: 30, tracks: [{id: 'v', kind: 'video', clips}]}, project || {})});
const range = s => P.rangeRenderRange(s);

// P.enabledEnd is the renderer's length (Go's EnabledDuration): disabled clips do not count.
assert.equal(P.enabledEnd({tracks: [{clips: [clip(0, 100), clip(100, 50, {disabled: true})]}]}), 100);
assert.equal(P.enabledEnd({tracks: []}), 0);
// Both points set: the in/out range, clamped to the enabled end.
assert.deepEqual(range(session([clip(0, 600)], {in_point: 100, out_point: 900})), {start: 100, end: 600});
assert.deepEqual(range(session([clip(0, 3000)], {in_point: 30, out_point: 90})), {start: 30, end: 90});
// Up to exactly 60 s; one frame more is refused.
assert.deepEqual(range(session([clip(0, 3000)], {in_point: 0, out_point: 1800})), {start: 0, end: 1800});
assert.equal(range(session([clip(0, 3000)], {in_point: 0, out_point: 1801})).error, 'too_long');
// 29.97 fps: 60 s are 1798 frames (Go's MaxRangeRenderFrames).
assert.deepEqual(range(session([clip(0, 3000)], {ntsc: true, in_point: 0, out_point: 1798})), {start: 0, end: 1798});
assert.equal(range(session([clip(0, 3000)], {ntsc: true, in_point: 0, out_point: 1799})).error, 'too_long');
// One point only, or points past the enabled end: 5 s around the playhead instead.
assert.deepEqual(range(session([clip(0, 600)], {in_point: 100}, 300)), {start: 225, end: 375});
assert.deepEqual(range(session([clip(0, 600)], {in_point: 700, out_point: 800}, 300)), {start: 225, end: 375});
// 5 s (150 frames at 30 fps) around the playhead at the start, the middle and the end.
assert.deepEqual(range(session([clip(0, 600)], null, 0)), {start: 0, end: 150});
assert.deepEqual(range(session([clip(0, 600)], null, 300)), {start: 225, end: 375});
assert.deepEqual(range(session([clip(0, 600)], null, 600)), {start: 450, end: 600});
assert.deepEqual(range(session([clip(0, 600)], null, 900)), {start: 450, end: 600}, 'a playhead past the end');
assert.deepEqual(range(session([clip(0, 100)], null, 50)), {start: 0, end: 100}, 'a timeline shorter than 5 s');
assert.deepEqual(range(session([clip(0, 1000)], {fps: 25}, 500)), {start: 438, end: 563}, '125 frames at 25 fps');
// Nothing to render.
assert.equal(range(session([])).error, 'empty');
assert.equal(range(session([clip(0, 90, {disabled: true})])).error, 'empty');
// Failure strings of the new codes.
assert.equal(P.jobFailure({status: 'failed', error: 'range_render_failed'})[0], 'rangeFailed');
assert.equal(P.jobFailure({status: 'failed', error: 'range_too_long'})[0], 'rangeTooLong');
assert.equal(P.jobFailure({status: 'failed', error: 'feature_unavailable'})[0], 'featureUnavailable');
assert.equal(P.jobFailure({status: 'failed', error: 'source_aspect_extreme'})[0], 'sourceAspectExtreme');
assert.equal(P.jobFailure({status: 'failed', error: 'render_segment_failed', result: {failed_from_seconds: 1, failed_to_seconds: 2}})[0], 'renderSegmentFailed');
`
	if output, err := exec.Command(node, "-e", script).CombinedOutput(); err != nil {
		t.Fatalf("range render checks: %v\n%s", err, output)
	}
}

// Every desktop locale carries the range check strings, keeps the placeholders and translates the sentences.
func TestVideoStudioRangeRenderLocales(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("lang", "desktop", "*.json"))
	if err != nil || len(files) != 16 {
		t.Fatalf("desktop locales: %d files, %v", len(files), err)
	}
	keys := []string{"rangeRender", "rangeRendering", "rangeLive", "rangeRendered", "rangeClose", "rangeRetry", "rangeFailed", "rangeTooLong", "job_range_render", "featureUnavailable", "sourceAspectExtreme"}
	locales := make(map[string]map[string]string, len(files))
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		locales[strings.TrimSuffix(filepath.Base(file), ".json")] = values
	}
	for lang, values := range locales {
		for _, key := range keys {
			if strings.TrimSpace(values["videoStudio."+key]) == "" {
				t.Errorf("%s lacks videoStudio.%s", lang, key)
			}
		}
		if text := values["videoStudio.rangeRendering"]; !strings.Contains(text, "{{range}}") || !strings.Contains(text, "{{percent}}") {
			t.Errorf("%s videoStudio.rangeRendering lost a placeholder: %q", lang, text)
		}
		if lang == "en" {
			continue
		}
		for _, key := range []string{"rangeRender", "rangeFailed", "rangeTooLong", "featureUnavailable", "sourceAspectExtreme"} {
			if values["videoStudio."+key] == locales["en"]["videoStudio."+key] {
				t.Errorf("%s copies the English videoStudio.%s", lang, key)
			}
		}
	}
}
