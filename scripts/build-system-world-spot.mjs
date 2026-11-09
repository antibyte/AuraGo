/* Reproducible compact companion for the System World city.
 *
 * The pinned BSD-3-Clause source (9.5 MB of untextured float32 geometry) is
 * verified by SHA-256 in ignored scratch, compressed with meshopt and reduced
 * to a city-scale triangle budget. The bit-exact source is never shipped.
 * Run: node scripts/build-system-world-spot.mjs [--check]
 *
 * Fetch the source first (see assets/system-world/spot-source.json):
 *   curl -L --fail -o disposable/system-world/boston_dynamics_spot-spot.glb <pinned URL>
 */
import fs from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import os from 'node:os';
import path from 'node:path';
import validator from 'gltf-validator';

const root = process.cwd();
const check = process.argv.includes('--check');
const meta = JSON.parse(await fs.readFile('assets/system-world/spot-source.json', 'utf8'));
const source = path.join(root, meta.scratch_path);
const outDir = 'ui/3d/system-world';
const ratio = 0.55;
const JOINT_NAMES = ['body', 'fl_hip', 'fr_hip', 'hl_hip', 'hr_hip',
  'fl_uleg', 'fr_uleg', 'hl_uleg', 'hr_uleg',
  'fl_lleg', 'fr_lleg', 'hl_lleg', 'hr_lleg'];
const sha = data => createHash('sha256').update(data).digest('hex');
async function size(dir) {
  let total = 0;
  for (const f of await fs.readdir(dir, { withFileTypes: true })) {
    const p = path.join(dir, f.name);
    total += f.isDirectory() ? await size(p) : (await fs.stat(p)).size;
  }
  return total;
}

let raw;
try { raw = await fs.readFile(source); }
catch { throw Error('Missing pinned source. Run: ' + meta.download); }
if (raw.length !== meta.bytes || sha(raw) !== meta.sha256) throw Error('Pinned Spot source hash drift');

const temp = path.join(os.tmpdir(), 'aurago-spot-' + process.pid + '.glb');
const npx = process.platform === 'win32' ? 'npx.cmd' : 'npx';
const run = spawnSync(npx, ['--yes', 'gltfpack', '-i', source, '-o', temp,
  '-cc', '-kn', '-km', '-si', String(ratio), '-sa'],
  { encoding: 'utf8', shell: process.platform === 'win32' });
if (run.status !== 0) throw Error('gltfpack failed: ' + (run.error?.message || run.stderr || run.stdout));
const bytes = await fs.readFile(temp);
await fs.unlink(temp).catch(() => {});

const result = await validator.validateBytes(new Uint8Array(bytes), { uri: 'spot.glb', maxIssues: 20 });
if (result.issues.numErrors) throw Error('Khronos validator: ' + JSON.stringify(result.issues.messages));
const jlen = bytes.readUInt32LE(12);
const doc = JSON.parse(bytes.subarray(20, 20 + jlen));
const names = doc.nodes.map(n => n.name || '');
const missing = JOINT_NAMES.filter(j => !names.some(n => n === j || n.startsWith(j + '.')));
if (missing.length) throw Error('Joint hierarchy lost: ' + missing.join(','));
for (const ext of ['EXT_meshopt_compression', 'KHR_mesh_quantization'])
  if (!doc.extensionsRequired?.includes(ext)) throw Error('Missing required extension: ' + ext);
if (doc.images?.length || doc.textures?.length) throw Error('Runtime derivative must stay texture-free');
if (doc.buffers?.some(b => b.uri)) throw Error('No external buffer URIs');

let triangles = 0;
for (const mesh of doc.meshes) for (const prim of mesh.primitives) {
  const ext = prim.extensions?.EXT_meshopt_compression;
  triangles += (ext ? ext.count : doc.accessors[prim.indices].count) / 3;
}
const manifest = {
  id: 'spot', license: meta.license, copyright: meta.copyright,
  provenance: { dataset: meta.dataset, commit: meta.commit, url: meta.url,
    upstream: meta.upstream, upstream_path: meta.upstream_path,
    license_url: meta.license_url, source_bytes: meta.bytes, source_sha256: meta.sha256 },
  source_triangles: meta.triangles, runtime_triangles: triangles,
  build: 'gltfpack -cc -kn -km -si ' + ratio + ' -sa',
  compression: doc.extensionsRequired, y_up: true, units: 'metres',
  file: 'spot.glb', bytes: bytes.length, sha256: sha(bytes),
  joints: JOINT_NAMES, validator_errors: 0,
};
const licence = await fs.readFile('licences/texts/system-world-spot-BSD-3-Clause.txt', 'utf8');
const manifestText = JSON.stringify(manifest, null, 2) + '\n';
const shipped = await fs.readFile(outDir + '/spot.glb').catch(() => null);
const shippedManifest = await fs.readFile(outDir + '/spot.json', 'utf8').catch(() => '');
const shippedLicence = await fs.readFile(outDir + '/spot-LICENSE.txt', 'utf8').catch(() => '');

if (check) {
  if (!shipped || sha(shipped) !== manifest.sha256) throw Error('Rebuild ui/3d/system-world/spot.glb');
  if (shippedManifest !== manifestText) throw Error('Rebuild ui/3d/system-world/spot.json');
  if (shippedLicence !== licence) throw Error('Rebuild ui/3d/system-world/spot-LICENSE.txt');
} else {
  await fs.writeFile(outDir + '/spot.glb', bytes);
  await fs.writeFile(outDir + '/spot.json', manifestText);
  await fs.writeFile(outDir + '/spot-LICENSE.txt', licence);
}
const total = await size(outDir);
if (total >= 40 * 1024 * 1024) throw Error('Model directory exceeds the 40 MiB ceiling: ' + total);
console.log('System World companion ' + (check ? 'verified' : 'built') + ': ' +
  bytes.length + ' bytes, ' + triangles + ' triangles, model directory ' + total + ' bytes');
