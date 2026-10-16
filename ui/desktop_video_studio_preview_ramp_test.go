package ui

import (
	"os/exec"
	"testing"
)

// TestVideoStudioPreviewRampRules checks the ramp-aware media timing of the preview (plan 2b, Task 7; index §4.18):
// the rules sourceSeconds, mediaRate and audioAllowed, and a mounted preview whose ramped clip seeks to offset + S(t),
// plays with playbackRate = speedAt x rate on every paint and keeps its linked sound silent.
func TestVideoStudioPreviewRampRules(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Video Studio preview checks")
	}
	script := `
const assert = require('node:assert/strict');
global.window = {};
global.location = {origin: 'http://localhost'};
require('./js/desktop/apps/video-studio-time.js');
require('./js/desktop/apps/video-studio-effects.js');
const media = [], contexts = [];
class MediaElement {
  constructor() { this.src = ''; this.paused = true; this.currentTime = 0; this.playbackRate = 1; this.readyState = 2; this.videoWidth = 320; this.videoHeight = 180; }
  addEventListener() {}
  load() {}
  pause() { this.paused = true; }
  play() { this.paused = false; return Promise.resolve(); }
  removeAttribute(name) { if (name === 'src') this.src = ''; }
}
class VideoElement extends MediaElement {}
global.HTMLMediaElement = MediaElement;
global.HTMLVideoElement = VideoElement;
global.Image = class { addEventListener() {} removeAttribute() {} };
const drawing = {fillRect() {}, save() {}, translate() {}, rotate() {}, beginPath() {}, rect() {}, clip() {}, drawImage() {}, restore() {}, clearRect() {},
  getImageData(x, y, w, h) { return {width: w, height: h, data: new Uint8ClampedArray(w * h * 4)}; }};
global.document = {createElement(tag) {
  if (tag === 'canvas') return {width: 0, height: 0, getContext: () => drawing};
  const element = new VideoElement(); media.push(element); return element;
}};
class AudioNode { constructor() { this.gain = {value: -1}; } connect() {} disconnect() {} }
window.AudioContext = class {
  constructor() { this.state = 'running'; this.destination = {}; this.gains = []; contexts.push(this); }
  createMediaElementSource() { return new AudioNode(); }
  createGain() { const node = new AudioNode(); this.gains.push(node); return node; }
  resume() { return Promise.resolve(); }
  close() { return Promise.resolve(); }
};
let clock = 1000, pending = null;
global.performance = {now: () => clock};
global.requestAnimationFrame = callback => { pending = callback; return 1; };
global.cancelAnimationFrame = () => {};
const runTo = ms => { clock = ms; const callback = pending; pending = null; if (callback) callback(ms); };
require('./js/desktop/apps/video-studio-timeline.js');
require('./js/desktop/apps/video-studio-painter2d.js');
require('./js/desktop/apps/video-studio-preview.js');
const FX = window.VideoStudioEffects, R = window.VideoStudioPreview.rules;
const near = (got, want, message) => assert(Math.abs(got - want) < 1e-9, message + ': got ' + got + ', want ' + want);

// Ramp 1x -> 3x over clip frames 0..60, one linear piece: speed(t) = 1 + t/30, S(t) = t + t*t/60; 3x after frame 60.
const ramp = () => [{f: 0, speed: 1, e: 'linear'}, {f: 60, speed: 3, e: 'linear'}];
const ramped = {id: 'r', asset_id: 'v', start: 30, offset: 12, duration: 90, ramp: ramp()};
near(R.sourceSeconds(ramped, 30, 30), 12 / 30, 'the first frame shows the offset');
near(R.sourceSeconds(ramped, 60, 30), (12 + 45) / 30, 'mid-ramp the position is offset + S(30)');
near(R.sourceSeconds(ramped, 100, 30), (12 + 120 + 30) / 30, 'after the last point the speed stays 3x');
near(R.sourceSeconds(ramped, 75, 30), FX.sourceAt(ramped, 75) / 30, 'the content anchor the keyframes use');
near(R.mediaRate(ramped, 30, 1), 1, 'the media clock starts at the first point speed');
near(R.mediaRate(ramped, 60, 1), 2, 'and follows the instantaneous speed');
near(R.mediaRate(ramped, 60, 2), 4, 'times the shuttle rate');
near(R.mediaRate(ramped, 100, 1), 3, 'constant after the last point');
assert.equal(R.mediaRate(Object.assign({}, ramped, {ramp: [{f: 0, speed: 8, e: 'linear'}, {f: 10, speed: 16, e: 'linear'}]}), 60, 8), 16, 'clamped to the 16x browsers accept');
const videoTrack = {id: 'v1', kind: 'video'}, audioTrack = {id: 'a1', kind: 'audio'}, video = {id: 'v', kind: 'video', has_audio: true};
assert.equal(R.audioAllowed(videoTrack, ramped, video), false, 'the linked sound of a ramped video-track clip is muted (Go audibleClip agrees)');
assert.equal(R.audioAllowed(videoTrack, Object.assign({}, ramped, {ramp: undefined}), video), true, 'the same clip without a ramp sounds');
assert.equal(R.audioAllowed(audioTrack, {id: 'ra', asset_id: 'v', start: 30, offset: 12, duration: 90}, video), true, 'its detached 1x audio clip still sounds');

// A mounted preview: paused seeks land on offset + S(t), forward playback sets speedAt x rate on every paint, and the
// linked sound stays silent.
const asset = {id: 'v', kind: 'video', media_url: '/media/v.mp4', has_audio: true, duration_frames: 600, width: 320, height: 180};
const clip = {id: 'r', asset_id: 'v', start: 0, offset: 0, duration: 90, x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, volume: 1,
  fade_in: 0, fade_out: 0, fit: 'contain', text: '', text_style: null, transition: null, ramp: ramp()};
const project = {version: 3, name: 'Ramp', width: 1280, height: 720, fps: 30, assets: [asset],
  tracks: [{id: 'v1', kind: 'video', name: 'V1', muted: false, hidden: false, locked: false, clips: [clip]}]};
assert(window.VideoStudioTimeline.validTimeline(project), 'the ramped fixture is a valid timeline');
const preview = window.VideoStudioPreview.mount({width: 0, height: 0, getContext: () => drawing}, () => project, () => 0, () => {}, () => {});
const element = media.at(-1);
preview.seek(30);
near(element.currentTime, 45 / 30, 'a paused seek shows source frame S(30) = 45');
near(element.playbackRate, 2, 'paused media carries the clip speed at the frame');
preview.play();
runTo(1500);
assert.equal(preview.getFrame(), 45, 'half a second at 30 fps');
near(element.playbackRate, 2.5, 'forward playback sets speedAt(45) on the paint');
assert.equal(contexts.at(-1).gains[0].gain.value, 0, 'the linked sound of the ramped clip is silent');
runTo(1600);
near(element.playbackRate, 2.6, 'and again on the next paint (frame 48)');
preview.setRate(2);
near(element.playbackRate, 5.2, 'the shuttle rate scales the instantaneous speed');
preview.pause();
near(element.playbackRate, 2.6, 'pausing returns to the clip speed at the frame');
preview.dispose();
`
	if output, err := exec.Command(node, "-e", script).CombinedOutput(); err != nil {
		t.Fatalf("preview ramp checks: %v\n%s", err, output)
	}
}
