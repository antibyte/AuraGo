// Loads the desktop sound sources in bundle order into this Node process, exactly as the browser concatenates them:
// every file attaches to `self`, which is globalThis here. Runs once per process (the files define globals).
import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';
import { desktopSoundCoreParts, desktopSoundThemeParts, desktopSoundHostPart } from './desktop-sound-bundle-parts.js';

export const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
let loaded = null;

export function loadDesktopSounds({ themes = 'all', strict = true, requireAll = false } = {}) {
  if (loaded) throw new Error('loadDesktopSounds runs once per process');
  globalThis.self = globalThis;
  globalThis.DesktopSoundEngineStrict = strict; // invalid themes throw instead of being skipped
  const missing = [];
  const run = rel => {
    const file = path.join(repoRoot, rel);
    if (!existsSync(file)) { missing.push(rel); return false; }
    vm.runInThisContext(readFileSync(file, 'utf8').replace(/^﻿/, ''), { filename: file });
    return true;
  };
  desktopSoundCoreParts.forEach(run);
  const ids = themes === 'all' ? Object.keys(desktopSoundThemeParts) : themes === 'none' ? [] : themes;
  for (const id of ids) {
    if (!desktopSoundThemeParts[id]) throw new Error('unknown sound theme ' + id);
    run(desktopSoundThemeParts[id]);
  }
  run(desktopSoundHostPart);
  if (requireAll && missing.length) throw new Error('missing sound sources: ' + missing.join(', '));
  loaded = { DSP: globalThis.DesktopSoundDSP, Instruments: globalThis.DesktopSoundInstruments, Engine: globalThis.DesktopSoundEngine, missing };
  return loaded;
}
