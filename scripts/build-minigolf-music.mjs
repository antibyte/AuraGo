import { createHash, webcrypto } from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const sourceDir = path.join(root, 'assets/minigolf/music');
const outputDir = path.join(root, 'ui/3d/minigolf/music');
const write = process.argv.includes('--write');
const check = process.argv.includes('--check');
if (write === check) throw new Error('Choose exactly one of --write or --check.');

const sources = [
  {
    id: 'a-simple-trifle', title: 'A Simple Trifle', author: 'jestar',
    page: 'https://opengameart.org/content/a-simple-trifle',
    download: 'https://opengameart.org/sites/default/files/ASimpleTrifle.mid',
    file: 'a-simple-trifle.mid'
  },
  {
    id: 'piano-emotional-solo-139', title: 'Piano Emotional Solo 139', author: 'Tozan',
    page: 'https://opengameart.org/content/piano-emotional-solo-139',
    download: 'https://opengameart.org/sites/default/files/pianoemo139a.mid',
    file: 'piano-emotional-solo-139.mid'
  },
  {
    id: 'crystal-tune-bright-1', title: 'Crystal Tune Bright 1', author: 'Tozan',
    page: 'https://opengameart.org/content/crystal-tune-bright-1',
    download: 'https://opengameart.org/sites/default/files/piuray2.mid',
    file: 'crystal-tune-bright-1.mid'
  }
];

function loadImporter() {
  const sandbox = { console, crypto: webcrypto, TextEncoder, performance, setTimeout, clearTimeout };
  sandbox.globalThis = sandbox;
  sandbox.window = sandbox;
  sandbox.self = sandbox;
  const context = vm.createContext(sandbox);
  const run = file => vm.runInContext(fs.readFileSync(path.join(root, file), 'utf8'), context, { filename: file });
  run('ui/js/vendor/synth-studio/gm-data.js');
  run('ui/js/desktop/apps/synth-studio-presets.js');
  run('ui/js/vendor/synth-studio/tone-midi-2.0.28.bundle.js');
  run('ui/js/desktop/apps/synth-studio-model.js');
  sandbox.SynthStudioMidiLib = { Midi: sandbox.Midi };
  run('ui/js/desktop/apps/synth-studio-midi.js');
  return { importFile: sandbox.SynthStudioMIDI.importFile, validate: sandbox.SynthStudioModel.validate };
}

const { importFile, validate } = loadImporter();
const outputs = new Map();
const tracks = [];
for (const source of sources) {
  const bytes = fs.readFileSync(path.join(sourceDir, source.file));
  const project = importFile(bytes);
  project.name = source.title;
  let noteCount = 0;
  project.tracks.forEach((track, trackIndex) => {
    track.id = `${source.id}-track-${trackIndex + 1}`;
    track.clips.forEach((clip, clipIndex) => {
      clip.id = `${source.id}-clip-${trackIndex + 1}-${clipIndex + 1}`;
      clip.notes.forEach((note, noteIndex) => {
        note.id = `${source.id}-note-${trackIndex + 1}-${clipIndex + 1}-${noteIndex + 1}`;
        noteCount++;
      });
    });
  });
  const safeProject = validate(project);
  if (!safeProject.tracks.length || safeProject.tracks.length > 16 || noteCount > 50_000) {
    throw new Error(`${source.title} exceeds Synth Studio MIDI import limits.`);
  }
  const lastTick = Math.max(0, ...safeProject.tracks.flatMap(track => track.clips.flatMap(clip =>
    clip.notes.map(note => clip.start + note.start + note.duration))));
  const projectFile = `${source.id}.project.json`;
  outputs.set(projectFile, `${JSON.stringify(safeProject, null, 2)}\n`);
  tracks.push({
    id: source.id,
    title: source.title,
    author: source.author,
    source: source.page,
    download: source.download,
    license: 'CC0',
    licenseUrl: 'https://creativecommons.org/publicdomain/zero/1.0/',
    original: source.file,
    sha256: createHash('sha256').update(bytes).digest('hex'),
    project: projectFile,
    tempo: safeProject.tempo,
    trackCount: safeProject.tracks.length,
    noteCount,
    durationSeconds: Number((lastTick * 60 / safeProject.tempo / safeProject.ppq).toFixed(2))
  });
}

outputs.set('manifest.json', `${JSON.stringify({ version: 1, tracks }, null, 2)}\n`);
let mismatch = false;
for (const [name, content] of outputs) {
  const target = path.join(outputDir, name);
  if (check) {
    if (!fs.existsSync(target) || fs.readFileSync(target, 'utf8') !== content) {
      console.error(`Out of date: ui/3d/minigolf/music/${name}`);
      mismatch = true;
    }
  } else {
    fs.mkdirSync(outputDir, { recursive: true });
    fs.writeFileSync(target, content);
  }
}
if (mismatch) process.exitCode = 1;
else console.log(`${check ? 'Checked' : 'Built'} ${tracks.length} deterministic minigolf MIDI projects.`);
