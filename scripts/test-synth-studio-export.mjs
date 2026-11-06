import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

const runtime = { console, Blob, TextEncoder, TextDecoder, crypto: globalThis.crypto, setTimeout, clearTimeout };
runtime.window = runtime;
runtime.globalThis = runtime;
runtime.SynthStudioPresets = {
    presets: [{ id: 'test-lead', category: 'leads' }],
    gm: Array.from({ length: 128 }, (_, program) => ({ id: 'gm-' + program, category: 'gm' })),
    get: id => ({ id, category: String(id).startsWith('drum') ? 'drums' : 'gm' })
};
vm.createContext(runtime);
vm.runInContext(readFileSync('ui/js/desktop/apps/synth-studio-model.js', 'utf8'), runtime);
vm.runInContext(readFileSync('ui/js/desktop/apps/synth-studio-export.js', 'utf8'), runtime);

const Model = runtime.SynthStudioModel;
const Export = runtime.SynthStudioExport;
assert.equal(Export.crc32(new TextEncoder().encode('123456789')), 0xcbf43926, 'CRC-32 matches the standard check value');

const project = Model.create('Stem Test');
const source = Model.addTrack(project, 'gm-0');
const drums = Model.addTrack(project, 'drums-kit');
const sourceClip = Model.createClip('gm-0');
sourceClip.notes = [{ id: 'note-lead', pitch: 64, start: 0, duration: 480, velocity: 0.8 }];
source.clips = [sourceClip];
const drumClip = Model.createClip('drums-kit');
drumClip.notes = [{ id: 'note-kick', pitch: 36, start: 0, duration: 120, velocity: 0.9 }];
drums.clips = [drumClip];
const stem = Export.stemProject(project, source.id);
assert.equal(stem.tracks.find(track => track.id === source.id).solo, true, 'the stem solos its track');
assert.equal(stem.tracks.find(track => track.id === source.id).mute, false, 'the stem track is unmuted');
assert.ok(stem.tracks.filter(track => track.id !== source.id).every(track => track.mute), 'other tracks are muted in a stem');
assert.equal(stem.master.volume, 1, 'stems bypass the master level');
assert.equal(stem.master.ratio, 1, 'stems bypass master compression');
assert.equal(stem.master.plateSize, project.master.plateSize, 'stems keep the project reverb character');
assert.equal(stem.master.echoSync, project.master.echoSync, 'stems keep the project echo time');
assert.equal(Model.validate(stem).name, project.name, 'a stem project validates');

const calls = [];
runtime.SynthStudioAudio = {
    async renderWav(value) {
        calls.push(value);
        return new Blob([new Uint8Array([1, 2, 3])], { type: 'audio/wav' });
    }
};
const progress = [];
const files = await Export.renderStems(project, { onProgress: (done, total, name) => progress.push([done, total, name]) });
assert.ok(files.length > 0, 'renderStems returns one file per audible track');
assert.equal(calls.length, files.length, 'every stem is rendered once');
assert.ok(files.every(file => file.name.endsWith('.wav')), 'stem entries use WAV filenames');
assert.equal(progress.length, files.length, 'progress fires for every stem');
assert.deepEqual(progress.at(-1).slice(0, 2), [files.length, files.length]);

const zip = await Export.zip([
    { name: '01 Drums.wav', blob: new Blob([new Uint8Array([1, 2, 3])]) },
    { name: '02 Bass.wav', blob: new Blob([new Uint8Array([4, 5])]) }
]);
const bytes = new Uint8Array(await zip.arrayBuffer());
const view = new DataView(bytes.buffer);
assert.equal(view.getUint32(0, true), 0x04034b50, 'the archive starts with a local file header');
assert.equal(view.getUint32(bytes.length - 22, true), 0x06054b50, 'the archive ends with an end record');
assert.ok(Buffer.from(bytes).includes(Buffer.from('01 Drums.wav')), 'local headers carry UTF-8 names');
assert.ok(Buffer.from(bytes).includes(Buffer.from('02 Bass.wav')), 'all entries are named');
let central = 0;
for (let i = 0; i < bytes.length - 4; i++) if (view.getUint32(i, true) === 0x02014b50) central++;
assert.equal(central, 2, 'the central directory contains every entry');
await assert.rejects(Export.zip([]), /no files/i);

console.log('Synth Studio stem export: CRC, stem isolation, ZIP layout and progress pass');
