/* Reproducible pack step for the System World v3 kit.
 *
 * Blender writes uncompressed GLBs plus manifest.raw.json into the ignored
 * scratch directory disposable/system-world/v3-raw/ (assets/system-world/
 * build_world3.py). This script compresses every GLB with the pinned
 * gltfpack (meshopt, no simplification), validates the packed bytes with the
 * Khronos validator, checks the kit contract and writes ui/3d/system-world/v3/
 * with a manifest that pins bytes and SHA-256 of the packed files.
 *
 * -vtf keeps window room coordinates (TEXCOORD_0, up to about 70) as floats; quantised texture
 * coordinates would be normalised to 0..1 because the kit has no textures to carry the transform.
 *
 * Run: node scripts/build-system-world-v3.mjs [--check]
 * --check never writes. It verifies the committed kit and, when the raw
 * directory exists, that repacking reproduces the committed files byte for byte.
 */
import fs from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import os from 'node:os';
import path from 'node:path';
import validator from 'gltf-validator';

const check = process.argv.includes('--check');
const rawDir = 'disposable/system-world/v3-raw';
const outDir = 'ui/3d/system-world/v3';
const modelRoot = 'ui/3d/system-world';
const sourceDir = 'assets/system-world';
const PACK_VERSION = 'gltfpack@1.3.0';
const PACK_FLAGS = ['-cc', '-kn', '-km', '-ke', '-kv', '-vtf'];
const PACK_LABEL = PACK_VERSION + ' ' + PACK_FLAGS.join(' ');
const CEILING = 40 * 1024 * 1024;
const FILE_RE = /^[a-z0-9-]+\.lod[0-2]\.glb$/;
const CONCURRENCY = 3;

const sha = data => createHash('sha256').update(data).digest('hex');
const lfSha = data => sha(Buffer.from(data.toString('latin1').replaceAll('\r\n', '\n'), 'latin1'));
const exists = p => fs.stat(p).then(() => true, () => false);
const fail = message => { throw new Error(message); };

async function size(dir) {
  let total = 0;
  for (const f of await fs.readdir(dir, { withFileTypes: true })) {
    const p = path.join(dir, f.name);
    total += f.isDirectory() ? await size(p) : (await fs.stat(p)).size;
  }
  return total;
}

function parseGlb(bytes, label) {
  if (bytes.length < 20 || bytes.readUInt32LE(0) !== 0x46546c67) fail(label + ': not a GLB');
  const jlen = bytes.readUInt32LE(12);
  return JSON.parse(bytes.subarray(20, 20 + jlen).toString('utf8'));
}

function triangleCount(doc) {
  let n = 0;
  for (const mesh of doc.meshes ?? []) for (const p of mesh.primitives)
    n += (p.indices !== undefined ? doc.accessors[p.indices].count : doc.accessors[p.attributes.POSITION].count) / 3;
  return n;
}

let tempCounter = 0;
function gltfpack(rawFile) {
  const temp = path.join(os.tmpdir(), 'aurago-v3-' + process.pid + '-' + (tempCounter++) + '.glb');
  const win = process.platform === 'win32';
  const q = p => win ? '"' + p + '"' : p;
  const run = spawnSync(win ? 'npx.cmd' : 'npx',
    ['--yes', PACK_VERSION, '-i', q(rawFile), '-o', q(temp), ...PACK_FLAGS],
    { encoding: 'utf8', shell: win });
  // gltfpack's wasm CLI can abort inside libuv at process exit on Windows
  // after it wrote the complete output. The bytes are still validated below.
  const exitCrash = /UV_HANDLE_CLOSING/.test(String(run.stderr) + String(run.stdout));
  if (run.status !== 0 && !exitCrash) fail('gltfpack failed for ' + rawFile + ': ' + (run.error?.message || run.stderr || run.stdout));
  return fs.readFile(temp).catch(() => fail('gltfpack wrote no output for ' + rawFile)).finally(() => fs.unlink(temp).catch(() => {}));
}

function checkContract(doc, asset, file) {
  if (!doc.extensionsRequired?.includes('EXT_meshopt_compression')) fail(file + ': EXT_meshopt_compression is not required');
  if (doc.images?.length || doc.textures?.length) fail(file + ': kit must stay texture-free');
  if (doc.buffers?.some(b => b.uri) || doc.images?.some(i => i.uri)) fail(file + ': external URIs are not allowed');
  for (const mesh of doc.meshes ?? []) for (const prim of mesh.primitives) {
    for (const attr of ['POSITION', 'NORMAL', 'COLOR_0'])
      if (prim.attributes?.[attr] === undefined) fail(file + ': primitive without ' + attr);
  }
  for (const material of doc.materials ?? [])
    if (!String(material.name || '').startsWith('city.')) fail(file + ': material "' + material.name + '" is not city.*');
  const names = new Set((doc.nodes ?? []).map(n => n.name));
  for (const component of asset.components ?? [])
    if (!names.has(component)) fail(file + ': component node missing: ' + component);
}

async function packOne(asset, lod) {
  if (!FILE_RE.test(lod.file)) fail('Bad file name: ' + lod.file);
  const bytes = await gltfpack(path.join(rawDir, lod.file));
  const result = await validator.validateBytes(new Uint8Array(bytes), { uri: lod.file, maxIssues: 50 });
  if (result.issues.numErrors) fail(lod.file + ': Khronos validator: ' + JSON.stringify(result.issues.messages));
  const doc = parseGlb(bytes, lod.file);
  checkContract(doc, asset, lod.file);
  const tris = triangleCount(doc);
  if (tris !== lod.triangles) fail(lod.file + ': gltfpack changed the triangle count (' + lod.triangles + ' -> ' + tris + '); no simplification is allowed');
  return bytes;
}

async function mapLimited(items, limit, fn) {
  const results = new Array(items.length);
  let next = 0;
  await Promise.all(Array.from({ length: Math.min(limit, items.length) }, async () => {
    while (next < items.length) { const i = next++; results[i] = await fn(items[i], i); }
  }));
  return results;
}

function checkTriangles(manifest) {
  for (const asset of manifest.assets) {
    if (asset.lods.map(l => l.level).join() !== '0,1,2') fail(asset.id + ': needs exactly LOD 0, 1, 2');
    let previous = Infinity;
    for (const lod of asset.lods) {
      if (!(lod.triangles <= previous)) fail(asset.id + ': triangles increase at LOD ' + lod.level);
      previous = lod.triangles;
    }
  }
}

async function checkSources(manifest) {
  for (const [name, hash] of Object.entries(manifest.sources ?? {})) {
    const data = await fs.readFile(path.join(sourceDir, name)).catch(() => fail('Missing authoring source: ' + name));
    if (lfSha(data) !== hash) fail('Authoring provenance drift: ' + name);
  }
}

async function checkCeiling() {
  const total = await size(modelRoot);
  if (total >= CEILING) fail('Model directory exceeds the 40 MiB ceiling: ' + total);
  return total;
}

if (check) {
  const manifest = JSON.parse(await fs.readFile(path.join(outDir, 'manifest.json'), 'utf8'));
  checkTriangles(manifest);
  await checkSources(manifest);
  const listed = new Set();
  let total = 0;
  for (const asset of manifest.assets) for (const lod of asset.lods) {
    listed.add(lod.file);
    const data = await fs.readFile(path.join(outDir, lod.file)).catch(() => fail('Missing file: ' + lod.file));
    if (data.length !== lod.bytes) fail(lod.file + ': size differs from the manifest');
    if (sha(data) !== lod.sha256) fail(lod.file + ': sha256 differs from the manifest');
    const checked = await validator.validateBytes(new Uint8Array(data), { uri: lod.file, maxIssues: 50 });
    if (checked.issues.numErrors) fail(lod.file + ': Khronos validator: ' + JSON.stringify(checked.issues.messages));
    checkContract(parseGlb(data, lod.file), asset, lod.file);
    total += data.length;
  }
  for (const name of await fs.readdir(outDir)) if (name.endsWith('.glb') && !listed.has(name)) fail('Unlisted GLB: ' + name);
  if (manifest.total_model_bytes !== total) fail('total_model_bytes differs from the files');
  let repacked = 'skipped (no ' + rawDir + ')';
  if (await exists(rawDir)) {
    const jobs = manifest.assets.flatMap(asset => asset.lods.map(lod => ({ asset, lod })));
    await mapLimited(jobs, CONCURRENCY, async ({ asset, lod }) => {
      const bytes = await packOne(asset, lod);
      if (sha(bytes) !== lod.sha256) fail(lod.file + ': repack does not reproduce the committed file');
    });
    repacked = 'reproduced ' + jobs.length + ' files';
  } else console.log('Repack comparison skipped: ' + rawDir + ' does not exist');
  const dir = await checkCeiling();
  console.log('System World v3 verified: ' + manifest.assets.length + ' designs, ' + listed.size + ' files, ' +
    total + ' bytes, repack ' + repacked + ', model directory ' + dir + ' bytes');
} else {
  const raw = JSON.parse(await fs.readFile(path.join(rawDir, 'manifest.raw.json'), 'utf8'));
  if (raw.schema_version !== 3) fail('Raw manifest must have schema_version 3');
  checkTriangles(raw);
  await checkSources(raw);
  const jobs = raw.assets.flatMap(asset => asset.lods.map(lod => ({ asset, lod })));
  const packed = await mapLimited(jobs, CONCURRENCY, ({ asset, lod }) => packOne(asset, lod));

  const manifest = structuredClone(raw);
  let total = 0, index = 0;
  for (const asset of manifest.assets) for (const lod of asset.lods) {
    const bytes = packed[index++];
    lod.bytes = bytes.length;
    lod.sha256 = sha(bytes);
    total += bytes.length;
  }
  manifest.compression = 'EXT_meshopt_compression';
  manifest.pack = PACK_LABEL;
  manifest.total_model_bytes = total;
  manifest.validator_errors = 0;

  await fs.mkdir(outDir, { recursive: true });
  const listed = new Set(jobs.map(j => j.lod.file));
  for (const name of await fs.readdir(outDir)) if (name.endsWith('.glb') && !listed.has(name)) await fs.unlink(path.join(outDir, name));
  await Promise.all(jobs.map((job, i) => fs.writeFile(path.join(outDir, job.lod.file), packed[i])));
  await fs.writeFile(path.join(outDir, 'LICENSE.txt'), (await fs.readFile('LICENSE', 'utf8')).replaceAll(String.fromCharCode(13, 10), String.fromCharCode(10)));
  await fs.writeFile(path.join(outDir, 'manifest.json'), JSON.stringify(manifest, null, 2) + '\n');
  const dir = await checkCeiling();
  console.log('System World v3 built: ' + manifest.assets.length + ' designs, ' + jobs.length + ' files, ' +
    total + ' bytes, model directory ' + dir + ' bytes');
}
