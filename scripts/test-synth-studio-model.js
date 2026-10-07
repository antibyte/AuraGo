'use strict';

import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const categories = ['drums', 'bass', 'leads', 'pads', 'keys'];
const presets = categories.flatMap((category, index) => [
    { id: `${['drum', 'bass', 'lead', 'pad', 'key'][index]}-one`, category },
    { id: `${['drum', 'bass', 'lead', 'pad', 'key'][index]}-two`, category }
]);
const gm = Array.from({ length: 128 }, (_, program) => ({ id: `gm-${program}`, category: 'gm', program }));
const byId = new Map([...presets, ...gm, { id: 'drums-kit', category: 'drums' }, { id: 'gm-drums', category: 'gm' }].map(item => [item.id, item]));
const runtime = { console, crypto: globalThis.crypto, performance: globalThis.performance };
runtime.globalThis = runtime;
runtime.window = runtime;
runtime.SynthStudioPresets = { presets, gm, get: id => byId.get(id) };

function loadClassicModule(file, target = runtime) {
    const module = { exports: {} };
    vm.runInNewContext(fs.readFileSync(file, 'utf8'), { module, exports: module.exports, globalThis: target, window: target, console }, { filename: file });
    return module.exports;
}

const Model = loadClassicModule('ui/js/desktop/apps/synth-studio-model.js');

runtime.self = runtime;
vm.runInNewContext(fs.readFileSync('ui/js/vendor/synth-studio/tone-midi-2.0.28.bundle.js', 'utf8'), runtime);
runtime.SynthStudioMidiLib = { Midi: runtime.Midi };
runtime.SynthStudioModel = Model;
const MIDI = loadClassicModule('ui/js/desktop/apps/synth-studio-midi.js');

function equalJSON(actual, expected, message) {
    assert.equal(JSON.stringify(actual), JSON.stringify(expected), message);
}

function code(expected) {
    return error => error && error.code === expected;
}

function expectCode(fn, expected) {
    assert.throws(fn, code(expected));
}

function midiFixture() {
    const project = Model.create('Round trip');
    project.tempo = 97;
    const bass = Model.addTrack(project, 'bass-one');
    bass.clips.push({
        id: 'bass-clip', name: 'Bass', start: 240, length: 1920,
        notes: [{ id: 'bass-note', pitch: 41, start: 120, duration: 360, velocity: 0.73 }],
        controllers: [
            { tick: 0, type: 'sustain', value: 0.75 },
            { tick: 240, type: 'modulation', value: 0.25 },
            { tick: 360, type: 'bend', value: -0.5 }
        ]
    });
    const drums = Model.addTrack(project, 'drums-kit');
    drums.clips.push({
        id: 'drum-clip', name: 'Kit', start: 0, length: 1920,
        notes: [{ id: 'drum-note', pitch: 70, start: 0, duration: 90, velocity: 0.9 }], controllers: []
    });
    for (const instrument of ['lead-one', 'pad-one', 'key-one']) {
        const track = Model.addTrack(project, instrument);
        track.clips.push({
            id: `${instrument}-clip`, name: instrument, start: 0, length: 1920,
            notes: [{ id: `${instrument}-note`, pitch: 64, start: 60, duration: 240, velocity: 0.6 }], controllers: []
        });
    }
    return project;
}

function typeZeroFixture() {
    const track = [
        0x00, 0xff, 0x03, 0x03, 0x4d, 0x49, 0x44,
        0x00, 0xff, 0x51, 0x03, 0x07, 0xa1, 0x20,
        0x00, 0xff, 0x58, 0x04, 0x04, 0x02, 0x18, 0x08,
        0x00, 0x90, 0x3c, 0x64,
        0x83, 0x60, 0x80, 0x3c, 0x00,
        0x00, 0xff, 0x2f, 0x00
    ];
    return Uint8Array.from([
        0x4d, 0x54, 0x68, 0x64, 0, 0, 0, 6, 0, 0, 0, 1, 1, 0xe0,
        0x4d, 0x54, 0x72, 0x6b, 0, 0, 0, track.length,
        ...track
    ]);
}

class FakeInput {
    constructor(id, name) { this.id = id; this.name = name; this.state = 'connected'; this.listeners = new Set(); this.openCount = 0; this.closeCount = 0; }
    addEventListener(type, fn) { if (type === 'midimessage') this.listeners.add(fn); }
    removeEventListener(type, fn) { if (type === 'midimessage') this.listeners.delete(fn); }
    async open() { this.openCount++; return this; }
    async close() { this.closeCount++; return this; }
    send(data, receivedTime = 123.5) { for (const fn of this.listeners) fn({ data: Uint8Array.from(data), receivedTime }); }
}

class FakeAccess {
    constructor(inputs) { this.inputs = new Map(inputs.map(input => [input.id, input])); this.listeners = new Set(); }
    addEventListener(type, fn) { if (type === 'statechange') this.listeners.add(fn); }
    removeEventListener(type, fn) { if (type === 'statechange') this.listeners.delete(fn); }
    stateChange(port) { for (const fn of this.listeners) fn({ port }); }
}

async function testMidiInput() {
    const input = new FakeInput('keyboard-1', 'Keyboard');
    const access = new FakeAccess([input]);
    let requestOptions;
    const descriptor = Object.getOwnPropertyDescriptor(runtime, 'navigator');
    Object.defineProperty(runtime, 'navigator', {
        configurable: true,
        value: { requestMIDIAccess: async options => { requestOptions = options; return access; } }
    });
    try {
        const notesOn = [], notesOff = [], controls = [], states = [];
        const device = MIDI.create({ onNoteOn: e => notesOn.push(e), onNoteOff: e => notesOff.push(e), onController: e => controls.push(e), onState: e => states.push(e) });
        equalJSON(await device.connect(), [{ id: 'keyboard-1', name: 'Keyboard' }]);
        equalJSON(requestOptions, { sysex: false });
        await device.select('keyboard-1');
        assert.equal(input.openCount, 1);
        input.send([0x90, 60, 127]);
        input.send([0xb0, 64, 127]);
        input.send([0xe0, 0, 64]);
        equalJSON(notesOn.map(e => [e.pitch, e.velocity, e.time]), [[60, 1, 123.5]]);
        equalJSON(controls.map(e => e.type), ['sustain', 'bend']);
        input.state = 'disconnected';
        access.stateChange(input);
        assert.equal(input.listeners.size, 0);
        assert.equal(notesOff.length, 1);
        assert.equal(notesOff[0].released, true);
        assert.equal(states.at(-1).status, 'disconnected');
        device.dispose();
        assert.equal(access.listeners.size, 0);
    } finally {
        if (descriptor) Object.defineProperty(runtime, 'navigator', descriptor);
        else delete runtime.navigator;
    }

    const saved = Object.getOwnPropertyDescriptor(runtime, 'navigator');
    Object.defineProperty(runtime, 'navigator', { configurable: true, value: { requestMIDIAccess: async () => { throw Object.assign(new Error('blocked'), { name: 'NotAllowedError' }); } } });
    try {
        const states = [];
        const device = MIDI.create({ onState: state => states.push(state) });
        await assert.rejects(device.connect(), code('E_MIDI_DENIED'));
        assert.equal(states.at(-1).status, 'denied');
    } finally {
        if (saved) Object.defineProperty(runtime, 'navigator', saved);
        else delete runtime.navigator;
    }

    const unsupportedDescriptor = Object.getOwnPropertyDescriptor(runtime, 'navigator');
    Object.defineProperty(runtime, 'navigator', { configurable: true, value: {} });
    try {
        const states = [];
        const device = MIDI.create({ onState: state => states.push(state) });
        await assert.rejects(device.connect(), code('E_MIDI_UNSUPPORTED'));
        assert.equal(states.at(-1).status, 'unsupported');
    } finally {
        if (unsupportedDescriptor) Object.defineProperty(runtime, 'navigator', unsupportedDescriptor);
        else delete runtime.navigator;
    }
}

async function testMidiGeneration() {
    let resolveAccess;
    const descriptor = Object.getOwnPropertyDescriptor(runtime, 'navigator');
    Object.defineProperty(runtime, 'navigator', { configurable: true, value: { requestMIDIAccess: () => new Promise(resolve => { resolveAccess = resolve; }) } });
    try {
        const states = [];
        const device = MIDI.create({ onState: state => states.push(state) });
        const pending = device.connect();
        device.disconnect();
        const lateAccess = new FakeAccess([new FakeInput('late', 'Late')]);
        resolveAccess(lateAccess);
        equalJSON(await pending, []);
        assert.equal(lateAccess.listeners.size, 0, 'a late permission result is not attached after disconnect');
        assert.equal(states.at(-1).status, 'disconnected');
    } finally {
        if (descriptor) Object.defineProperty(runtime, 'navigator', descriptor);
        else delete runtime.navigator;
    }

    class DeferredInput extends FakeInput {
        open() { this.openCount++; return new Promise(resolve => { this.finishOpen = () => resolve(this); }); }
    }
    const first = new DeferredInput('first', 'First');
    const second = new DeferredInput('second', 'Second');
    const access = new FakeAccess([first, second]);
    const saved = Object.getOwnPropertyDescriptor(runtime, 'navigator');
    Object.defineProperty(runtime, 'navigator', { configurable: true, value: { requestMIDIAccess: async () => access } });
    try {
        const seen = [];
        const device = MIDI.create({ onNoteOn: event => seen.push(event.pitch) });
        await device.connect();
        const selectingFirst = device.select('first');
        const selectingSecond = device.select('second');
        first.finishOpen();
        second.finishOpen();
        await Promise.all([selectingFirst, selectingSecond]);
        first.send([0x90, 60, 100]);
        second.send([0x90, 62, 100]);
        equalJSON(seen, [62], 'only the latest selected input is active');
        assert.equal(first.listeners.size, 0);
        assert.equal(second.listeners.size, 1);
        device.dispose();
        assert.equal(second.listeners.size, 0, 'dispose removes the selected input handler');
    } finally {
        if (saved) Object.defineProperty(runtime, 'navigator', saved);
        else delete runtime.navigator;
    }
}

async function main() {
    const project = Model.create('Test');
    const clean = Model.validate(project);
    clean.name = 'changed';
    assert.equal(project.name, 'Test', 'validation returns an isolated clone');
    expectCode(() => Model.validate({ ...project, extra: 'script' }), 'E_PROJECT_FIELD_UNKNOWN');
    expectCode(() => Model.validate({ ...project, tempo: Infinity }), 'E_PROJECT_NUMBER');
    expectCode(() => Model.validate({ ...project, tempo: 39 }), 'E_PROJECT_NUMBER');
    expectCode(() => Model.validate({ ...project, tempo: 241 }), 'E_PROJECT_NUMBER');
    assert.equal(Model.validate({ ...project, tempo: 40 }).tempo, 40);
    assert.equal(Model.validate({ ...project, tempo: 240 }).tempo, 240);
    expectCode(() => Model.validate({ ...project, version: 2 }), 'E_PROJECT_VERSION');
    expectCode(() => Model.validate({ ...project, name: '<bad\nname>' }), 'E_PROJECT_TEXT');
    expectCode(() => Model.validate({ ...project, tracks: [null] }), 'E_PROJECT_TYPE');
    expectCode(() => Model.validate('{bad json'), 'E_PROJECT_JSON');
    expectCode(() => Model.validate('"' + 'é'.repeat(2_621_441) + '"'), 'E_PROJECT_SIZE');
    expectCode(() => Model.create('x'.repeat(97)), 'E_PROJECT_TEXT');
    expectCode(() => Model.addTrack(project, 'unknown-preset'), 'E_PROJECT_INSTRUMENT');
    const noCryptoRuntime = { SynthStudioPresets: runtime.SynthStudioPresets };
    const noCryptoModel = loadClassicModule('ui/js/desktop/apps/synth-studio-model.js', noCryptoRuntime);
    const fallbackIds = [noCryptoModel.newID('track'), noCryptoModel.newID('clip')];
    assert.ok(fallbackIds.every(value => /^[A-Za-z0-9_-]{1,100}$/.test(value)));
    assert.notEqual(fallbackIds[0], fallbackIds[1]);

    for (let i = 0; i < 15; i++) Model.addTrack(project, 'gm-0');
    Model.addTrack(project, 'drums-kit');
    expectCode(() => Model.addTrack(project, 'gm-1'), 'E_PROJECT_LIMIT');
    expectCode(() => Model.addTrack(project, 'gm-drums'), 'E_PROJECT_LIMIT');
    assert.equal(Model.validate(project).tracks.length, 16);

    const demo = Model.demo();
    assert.equal(demo.tracks.length, 5);
    equalJSON(demo.tracks.map(track => track.clips.map(clip => clip.start)), Array.from({ length: 5 }, () => [0, 1920, 3840, 5760]));
    equalJSON([...new Set(demo.tracks.map(track => byId.get(track.instrument).category))], categories);
    assert.ok(demo.tracks.every(track => track.clips.length === 4 && track.clips.every(clip => clip.length === Model.BAR)));
    assert.equal(Model.patterns.length, 10);
    equalJSON(Model.patterns.map(pattern => pattern.name), ['Rock beat', 'Offbeat groove', 'Octave bass', 'Walking bass', 'Stepped melody', 'Echo melody', 'Open chord', 'Rising chord', 'Chord rhythm', 'Arpeggio']);
    equalJSON(categories, [...new Set(Model.patterns.map(pattern => pattern.category))]);
    equalJSON(categories.map(category => Model.patterns.filter(pattern => pattern.category === category).length), [2, 2, 2, 2, 2]);
    const patternCopy = Model.cloneClip(Model.patterns[0].clip, 1920);
    assert.notEqual(patternCopy.id, Model.patterns[0].clip.id);
    assert.notEqual(patternCopy.notes[0].id, Model.patterns[0].clip.notes[0].id);
    assert.equal(patternCopy.start, 1920);

    const eventClip = { id: 'clip-a', name: 'Events', start: 0, length: 480, notes: [
        { id: 'n1', pitch: 60, start: 0, duration: 120, velocity: 0.8 },
        { id: 'n2', pitch: 60, start: 120, duration: 120, velocity: 0.8 }
    ], controllers: [{ tick: 120, type: 'sustain', value: 1 }] };
    function projectWithClip(clip) {
        const project = Model.create('Clip validation');
        Model.addTrack(project, 'gm-0').clips.push(clip);
        return project;
    }
    const noteAtClipEnd = { ...eventClip, notes: [...eventClip.notes, { id: 'n3', pitch: 62, start: 480, duration: 1, velocity: 0.8 }] };
    expectCode(() => Model.validate(projectWithClip(noteAtClipEnd)), 'E_PROJECT_TICK');
    const notePastClipEnd = { ...eventClip, length: 239 };
    expectCode(() => Model.validate(projectWithClip(notePastClipEnd)), 'E_PROJECT_TICK');
    const controllerAtClipEnd = { ...eventClip, controllers: [...eventClip.controllers, { tick: 480, type: 'modulation', value: 0.5 }] };
    expectCode(() => Model.validate(projectWithClip(controllerAtClipEnd)), 'E_PROJECT_TICK');
    equalJSON(Model.events(eventClip).filter(event => event.tick === 120).map(event => event.type), ['noteOff', 'controller', 'noteOn']);
    assert.equal(Model.endTick({ ...eventClip, start: 960 }), 1440);
    const duplicate = Model.create('Duplicate');
    const duplicateTrack = Model.addTrack(duplicate, 'gm-0');
    duplicateTrack.clips.push({ ...eventClip, notes: [eventClip.notes[0], { ...eventClip.notes[1], id: 'n1' }] });
    expectCode(() => Model.validate(duplicate), 'E_PROJECT_ID_DUPLICATE');

    const history = Model.history(Model.create('History'), 2);
    const first = history.current();
    first.name = 'First edit';
    history.commit(first);
    assert.equal(history.canUndo(), true);
    history.undo();
    assert.equal(history.current().name, 'History');
    history.redo();
    assert.equal(history.current().name, 'First edit');
    const next = history.current();
    next.name = 'Second edit';
    history.commit(next);
    const last = history.current();
    last.name = 'Third edit';
    history.commit(last);
    history.undo();
    assert.equal(history.current().name, 'Second edit', 'history retains only the bounded tail');

    const midiBytes = MIDI.exportFile(midiFixture());
    assert.ok(ArrayBuffer.isView(midiBytes));
    assert.equal((midiBytes[8] << 8) | midiBytes[9], 1, 'exports Standard MIDI Type 1');
    const exported = new runtime.Midi(midiBytes);
    equalJSON(exported.tracks.map(track => [track.instrument.number, track.channel]), [[38, 0], [0, 9], [81, 1], [89, 2], [4, 3]], 'custom synth patches use documented GM approximations and drums use channel 10');
    const imported = MIDI.importFile(Uint8Array.from(midiBytes));
    assert.ok(Math.abs(imported.tempo - 97) < 0.001);
    const importedBass = imported.tracks.find(track => track.instrument === 'gm-38');
    const importedDrums = imported.tracks.find(track => track.instrument === 'gm-drums');
    assert.ok(importedBass && importedDrums);
    equalJSON(importedDrums.clips[0].notes.map(note => note.pitch), [70]);
    assert.ok(Math.abs(importedBass.clips[0].notes[0].velocity - 0.73) < 0.01);
    equalJSON([importedBass.clips[0].notes[0].start, importedBass.clips[0].notes[0].duration], [360, 360], 'MIDI note timing and duration survive roundtrip');
    equalJSON(importedBass.clips[0].controllers.map(event => event.type).sort(), ['bend', 'modulation', 'sustain']);
    assert.ok(Math.abs(importedBass.clips[0].controllers.find(event => event.type === 'bend').value + 0.5) < 0.01);
    assert.ok(Math.abs(importedBass.clips[0].controllers.find(event => event.type === 'sustain').value - 0.75) < 0.01);
    const typeZero = MIDI.importFile(typeZeroFixture());
    assert.equal(typeZero.tracks[0].instrument, 'gm-0');
    equalJSON([typeZero.tracks[0].clips[0].notes[0].pitch, typeZero.tracks[0].clips[0].notes[0].duration], [60, 480]);
    const controllerAtBarEnd = new runtime.Midi();
    controllerAtBarEnd.header.setTempo(120);
    controllerAtBarEnd.addTrack().addCC({ number: 64, ticks: Model.BAR, value: 0.5 });
    const boundaryImport = MIDI.importFile(controllerAtBarEnd.toArray());
    assert.equal(boundaryImport.tracks[0].clips[0].length, Model.BAR * 2, 'MIDI clip extends past controllers at a bar boundary');

    expectCode(() => MIDI.importFile(new Uint8Array(13)), 'E_MIDI_SIZE');
    const formatTwo = Uint8Array.from([0x4d, 0x54, 0x68, 0x64, 0, 0, 0, 6, 0, 2, 0, 1, 1, 0xe0]);
    expectCode(() => MIDI.importFile(formatTwo), 'E_MIDI_FORMAT_2');
    const smpte = Uint8Array.from([0x4d, 0x54, 0x68, 0x64, 0, 0, 0, 6, 0, 0, 0, 1, 0xe7, 0x28]);
    expectCode(() => MIDI.importFile(smpte), 'E_MIDI_SMPTE');

    const changedTempo = new runtime.Midi();
    changedTempo.header.tempos = [{ ticks: 0, bpm: 120 }, { ticks: 480, bpm: 90 }];
    changedTempo.header.update();
    changedTempo.addTrack().addNote({ midi: 60, ticks: 0, durationTicks: 120, velocity: 0.8 });
    expectCode(() => MIDI.importFile(changedTempo.toArray()), 'E_MIDI_TEMPO_CHANGES');
    const outOfRangeTempo = new runtime.Midi();
    outOfRangeTempo.header.setTempo(250);
    outOfRangeTempo.addTrack().addNote({ midi: 60, ticks: 0, durationTicks: 120, velocity: 0.8 });
    expectCode(() => MIDI.importFile(outOfRangeTempo.toArray()), 'E_MIDI_TEMPO');
    const unsupportedCC = new runtime.Midi();
    unsupportedCC.header.setTempo(120);
    unsupportedCC.addTrack().addCC({ number: 7, ticks: 0, value: 0.5 });
    expectCode(() => MIDI.importFile(unsupportedCC.toArray()), 'E_MIDI_CONTROLLER_UNSUPPORTED');

    const changedSignature = new runtime.Midi();
    changedSignature.header.timeSignatures = [{ ticks: 0, timeSignature: [3, 4] }];
    changedSignature.header.update();
    changedSignature.addTrack().addNote({ midi: 60, ticks: 0, durationTicks: 120, velocity: 0.8 });
    expectCode(() => MIDI.importFile(changedSignature.toArray()), 'E_MIDI_TIME_SIGNATURE');

    await testMidiInput();
    await testMidiGeneration();
    process.stdout.write('Synth Studio model/MIDI checks passed.\n');
}

main().catch(error => {
    console.error(error);
    process.exitCode = 1;
});
