import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const categories = ['drums', 'bass', 'leads', 'pads', 'keys'];
const prefixes = ['drum', 'bass', 'lead', 'pad', 'key'];
const presets = categories.map((category, i) => ({ id: `${prefixes[i]}-one`, category }));
const gm = Array.from({ length: 128 }, (_, i) => ({ id: `gm-${i}`, category: 'gm' }));
const byId = new Map([...presets, ...gm, { id: 'drums-kit' }, { id: 'gm-drums' }].map(item => [item.id, item]));
const runtime = { console, crypto: globalThis.crypto, performance: globalThis.performance, TextEncoder };
runtime.globalThis = runtime;
runtime.window = runtime;
runtime.SynthStudioPresets = { presets, gm, get: id => byId.get(id) };

function loadModel() {
    const module = { exports: {} };
    vm.runInNewContext(fs.readFileSync('ui/js/desktop/apps/synth-studio-model.js', 'utf8'), {
        module, exports: module.exports, globalThis: runtime, window: runtime, console, TextEncoder
    });
    return module.exports;
}

function extractFunction(source, name) {
    const match = new RegExp(`(?:async\\s+)?function\\s+${name}\\s*\\(`).exec(source);
    assert.ok(match, `Could not find production function ${name}`);
    const start = match.index;
    const open = source.indexOf('{', match.index + match[0].length);
    let depth = 0, quote = '', escaped = false, lineComment = false, blockComment = false;
    for (let i = open; i < source.length; i++) {
        const ch = source[i], next = source[i + 1];
        if (lineComment) { if (ch === '\n') lineComment = false; continue; }
        if (blockComment) { if (ch === '*' && next === '/') { blockComment = false; i++; } continue; }
        if (quote) {
            if (escaped) escaped = false;
            else if (ch === '\\') escaped = true;
            else if (ch === quote) quote = '';
            continue;
        }
        if (ch === '/' && next === '/') { lineComment = true; i++; continue; }
        if (ch === '/' && next === '*') { blockComment = true; i++; continue; }
        if (ch === '"' || ch === "'" || ch === '`') { quote = ch; continue; }
        if (ch === '{') depth++;
        else if (ch === '}' && --depth === 0) return source.slice(start, i + 1);
    }
    throw new Error(`Unclosed production function ${name}`);
}

function bindRecordingFunctions(state) {
    const source = fs.readFileSync('ui/js/desktop/apps/synth-studio.js', 'utf8');
    const eventCount = source.slice(source.indexOf('const eventCount='), source.indexOf(';', source.indexOf('const eventCount=')) + 1);
    const names = ['utf8ByteLength', 'position', 'stop', 'panic', 'recordTick', 'stopForRecordingLimit', 'eventBytes', 'noteOn', 'noteOff', 'controller', 'startRecording', 'finishRecording'];
    const functions = names.map(name => extractFunction(source, name)).join('\n');
    return new Function('state', `with (state) { ${eventCount}\n${functions}\nreturn { startRecording, noteOn, controller, finishRecording }; }`)(state);
}

const M = loadModel();
const BAR = 1920, PPQ = 480, MAX_TICK = 10_000_000, MAX_EVENTS = 50_000, MAX_CLIPS = 256, MAX_PROJECT_BYTES = 5 * 1024 * 1024, RECORDING_TICKS = BAR * 64;

function projectWithClipCount(count) {
    const project = M.create('Recording bounds');
    const track = M.addTrack(project, 'gm-0');
    for (let i = 0; i < count; i++) {
        track.clips.push({ id: `clip-${i}`, name: 'Clip', start: i * BAR, length: BAR, notes: [], controllers: [] });
    }
    return M.validate(project);
}

function projectNearEventLimit() {
    const project = M.create('Event boundary');
    const track = M.addTrack(project, 'gm-0');
    track.clips.push({
        id: 'existing-clip', name: 'Existing', start: 0, length: BAR, notes: [],
        controllers: Array.from({ length: MAX_EVENTS - 1 }, () => ({ tick: 0, type: 'sustain', value: 0 }))
    });
    return M.validate(project);
}

function harness(initial) {
    const project = M.validate(initial);
    const selection = project.tracks[0].id;
    const elements = new Map();
    const element = selector => {
        if (!elements.has(selector)) elements.set(selector, { checked: true, textContent: '', classList: { add() {}, remove() {} } });
        return elements.get(selector);
    };
    const state = {
        M, BAR, PPQ, MAX_TICK, MAX_EVENTS, MAX_CLIPS, MAX_PROJECT_BYTES, RECORDING_TICKS, TextEncoder,
        currentProject: project, recording: null, playing: false, cursor: 0, disposed: false, generation: 0,
        ready: true, busy: false, selectedInstrument: 'gm-0', countIn: false, quantize: 120, latency: 0, metronome: false,
        storageState: {}, live: new Map(), errors: [], playCalls: 0,
        performance: globalThis.performance,
        project() { return this.currentProject; },
        track() { return this.currentProject.tracks.find(t => t.id === selection); },
        readonly() { return false; }, tr(key) { return key; }, find: element,
        editor: { selection: () => ({ trackId: selection }), setPlayhead() {}, select() {} },
        history: M.history(project), storage: { changed() {} }, refresh() {}, status() {},
        fail(error) { this.errors.push(typeof error === 'string' ? error : error?.code); },
        insert() {},
        audio: {
            unlock: () => Promise.resolve(),
            stop: () => 0,
            play() { state.playCalls++; return Promise.resolve(); },
            currentTick: () => 0,
            noteOn: () => () => {},
            addController() {}, panic() {}
        }
    };
    const api = bindRecordingFunctions(state);
    return { state, api };
}

// A full clip lane must reject before creating a temporary or committed take.
{
    const { state, api } = harness(projectWithClipCount(MAX_CLIPS));
    await api.startRecording();
    assert.deepEqual(state.errors, ['E_PROJECT_LIMIT']);
    assert.equal(state.playCalls, 0);
    assert.equal(state.currentProject.tracks[0].clips.length, MAX_CLIPS);
}

// The final event slot is reserved by a held note. A following controller hits
// the cap, stops recording, and must commit that held note before reporting it.
{
    const { state, api } = harness(projectNearEventLimit());
    await api.startRecording();
    assert.equal(state.recording?.phase, 'record');
    await api.noteOn(60, 0.8, globalThis.performance.now(), 1);
    assert.equal(state.recording.used, 1);
    api.controller({ type: 'modulation', value: 0.5, time: globalThis.performance.now() });
    assert.deepEqual(state.errors, ['E_PROJECT_LIMIT']);
    assert.equal(state.recording, null);
    const clips = state.currentProject.tracks[0].clips;
    assert.equal(clips.length, 2);
    assert.equal(clips[1].notes.length, 1);
    assert.equal(clips[1].notes[0].pitch, 60);
    assert.equal(clips.reduce((total, clip) => total + clip.notes.length + clip.controllers.length, 0), MAX_EVENTS);
}

console.log('Synth Studio recording-boundary checks passed.');
