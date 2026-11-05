// Task A1: the 25 source WAVs of the Storymaker sound effects and their provenance table.
// Run: node --test assets/storymaker/test/sfx.test.mjs

import test from 'node:test';
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const sfxDir = path.join(here, '..', 'sfx');
const readme = path.join(here, '..', 'README.md');

// Frozen catalog sfx entry: 4 + 4 + 4 + 3 + 2 + 2 + 6 = 25 takes.
const TAKES = { whoosh: 4, pop: 4, click: 4, ding: 3, success: 2, error: 2, typing: 6 };
const names = Object.entries(TAKES).flatMap(([n, k]) => Array.from({ length: k }, (_, i) => `${n}_${i + 1}.wav`));

function wavFormat(buf) {
  assert.equal(buf.toString('latin1', 0, 4), 'RIFF');
  assert.equal(buf.toString('latin1', 8, 12), 'WAVE');
  const out = { dataBytes: 0 };
  for (let at = 12; at + 8 <= buf.length;) {
    const id = buf.toString('latin1', at, at + 4);
    const size = buf.readUInt32LE(at + 4);
    if (id === 'fmt ') {
      out.format = buf.readUInt16LE(at + 8);
      out.channels = buf.readUInt16LE(at + 10);
      out.rate = buf.readUInt32LE(at + 12);
      out.bits = buf.readUInt16LE(at + 22);
    } else if (id === 'data') {
      out.dataBytes = size;
    }
    at += 8 + size + (size % 2);
  }
  return out;
}

function readmeRows() {
  const rows = new Map();
  const pattern = /^\| `sfx\/([a-z]+_\d+\.wav)` \| (\d+) \| ([0-9a-f]{64}) \|\r?$/gm;
  for (const m of fs.readFileSync(readme, 'utf8').matchAll(pattern)) rows.set(m[1], { bytes: Number(m[2]), sha256: m[3] });
  return rows;
}

test('the folder holds the 25 takes as 48 kHz mono PCM16', () => {
  assert.equal(names.length, 25);
  assert.deepEqual(fs.readdirSync(sfxDir).sort(), [...names].sort());
  for (const name of names) {
    const f = wavFormat(fs.readFileSync(path.join(sfxDir, name)));
    assert.deepEqual({ format: f.format, channels: f.channels, rate: f.rate, bits: f.bits }, { format: 1, channels: 1, rate: 48000, bits: 16 }, name);
    assert.ok(f.dataBytes > 0, name);
  }
});

test('every WAV has the sha256 and the size of its README row', () => {
  const rows = readmeRows();
  assert.deepEqual([...rows.keys()].sort(), [...names].sort());
  for (const name of names) {
    const bytes = fs.readFileSync(path.join(sfxDir, name));
    assert.equal(crypto.createHash('sha256').update(bytes).digest('hex'), rows.get(name).sha256, name);
    assert.equal(bytes.length, rows.get(name).bytes, name);
  }
});
