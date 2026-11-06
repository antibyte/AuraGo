package ui

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/videostudio"
)

// storymakerHandoffPrelude loads the app core, the model and the handoff module into one context. The core gives S.uuid and
// S.errors, the model gives S.model.exportSize, and no module touches the DOM while it loads.
const storymakerHandoffPrelude = `
'use strict';
const fs = require('fs');
const vm = require('vm');
const path = require('path');
const dir = process.argv[2];
const context = { console };
context.globalThis = context; context.window = context;
context.document = { documentElement: { lang: 'en' } };
vm.createContext(context);
for (const name of ['storymaker-core.js', 'storymaker-model.js', 'storymaker-handoff.js']) {
  vm.runInContext(fs.readFileSync(path.join(dir, name), 'utf8'), context, { filename: name });
}
const S = context.StorymakerParts;
const eq = (a, b, what) => { const x = JSON.stringify(a), y = JSON.stringify(b); if (x !== y) throw new Error(what + ': got ' + x + ', want ' + y); };
`

// runStorymakerHandoffNode runs the prelude plus body in Node and returns what the script printed.
func runStorymakerHandoffNode(t *testing.T, body string) []byte {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	script := filepath.Join(t.TempDir(), "storymaker-handoff-check.js")
	if err := os.WriteFile(script, []byte(storymakerHandoffPrelude+body), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, err := filepath.Abs(filepath.Join("js", "desktop", "apps"))
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(node, script, dir)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("node check failed: %v\n%s", err, stderr.String())
	}
	return stdout.Bytes()
}

// decodeStorymakerHandoffProject reads a project the way the PUT handler does (unknown fields refused) and runs the Go validator.
func decodeStorymakerHandoffProject(t *testing.T, raw json.RawMessage) videostudio.Project {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var project videostudio.Project
	if err := decoder.Decode(&project); err != nil {
		t.Fatalf("the project does not decode strictly: %v\n%s", err, raw)
	}
	if err := videostudio.Validate(project); err != nil {
		t.Fatalf("videostudio.Validate refuses the project: %v\n%s", err, raw)
	}
	return project
}

// The tracks from Node pass the strict decoder and the Go validator; durations never pass the asset or the film; a silent stem has
// no track.
func TestStorymakerHandoffTracksInNode(t *testing.T) {
	t.Parallel()
	out := runStorymakerHandoffNode(t, storymakerHandoffTracksScript)
	var printed struct {
		Full   json.RawMessage `json:"full"`
		Silent json.RawMessage `json:"silent"`
	}
	if err := json.Unmarshal(out, &printed); err != nil {
		t.Fatalf("node output is not JSON: %v\n%s", err, out)
	}
	full := decodeStorymakerHandoffProject(t, printed.Full)
	silent := decodeStorymakerHandoffProject(t, printed.Silent)

	// The film has 250 frames, so each duration is min(asset frames, 250): video 300 -> 250, narration 180 -> 180,
	// music 400 -> 250, sfx 250 -> 250.
	want := []struct {
		kind     videostudio.TrackKind
		name     string
		asset    string
		duration int
	}{
		{videostudio.TrackVideo, "Video track", "a-video", 250},
		{videostudio.TrackAudio, "Narration track", "a-narration", 180},
		{videostudio.TrackAudio, "Music track", "a-music", 250},
		{videostudio.TrackAudio, "Effects track", "a-sfx", 250},
	}
	if len(full.Tracks) != len(want) {
		t.Fatalf("got %d tracks, want %d", len(full.Tracks), len(want))
	}
	for i, w := range want {
		track := full.Tracks[i]
		if track.Kind != w.kind || track.Name != w.name || len(track.Clips) != 1 || !strings.HasPrefix(track.ID, "track-") {
			t.Fatalf("track %d = %+v", i, track)
		}
		clip := track.Clips[0]
		if clip.AssetID != w.asset || clip.Duration != w.duration || clip.Start != 0 || clip.Offset != 0 || !strings.HasPrefix(clip.ID, "clip-") {
			t.Errorf("track %d clip = %+v", i, clip)
		}
		// Only the picture detaches its audio: the stems carry the sound.
		if clip.AudioDetached != (w.kind == videostudio.TrackVideo) {
			t.Errorf("track %d audio_detached = %v", i, clip.AudioDetached)
		}
	}

	// Music and sfx were silent, so their stems do not exist and they get no track.
	if len(silent.Tracks) != 2 || silent.Tracks[0].Kind != videostudio.TrackVideo || silent.Tracks[1].Clips[0].AssetID != "a-narration" {
		t.Fatalf("tracks without silent stems = %+v", silent.Tracks)
	}
}

const storymakerHandoffTracksScript = `
const audio = (id, frames) => ({ id, name: id + '.wav', path: 'media/' + id + '.wav', kind: 'audio', duration_frames: frames, duration_us: Math.round(frames / 30 * 1e6), has_audio: true, codec: 'pcm_s16le', size_bytes: 1000 });
const project = {
  version: 3, name: 'Firewall', width: 1920, height: 1080, fps: 30,
  assets: [
    { id: 'a-video', name: 'film.mp4', path: 'media/a-video.mp4', kind: 'video', duration_frames: 300, duration_us: 10000000, video_duration_us: 10000000, width: 1920, height: 1080, has_audio: true, source_fps: '30/1', codec: 'h264', size_bytes: 1000 },
    audio('a-narration', 180), audio('a-music', 400), audio('a-sfx', 250)
  ],
  tracks: []
};
const names = { video: 'Video track', narration: 'Narration track', music: 'Music track', sfx: 'Effects track' };
const full = S.handoff.buildTracks(project, { video: 'a-video', narration: 'a-narration', music: 'a-music', sfx: 'a-sfx' }, 250, names);
const silent = S.handoff.buildTracks(project, { video: 'a-video', narration: 'a-narration' }, 250, names);
console.log(JSON.stringify({ full: Object.assign({}, project, { tracks: full }), silent: Object.assign({}, project, { tracks: silent }) }));
`
