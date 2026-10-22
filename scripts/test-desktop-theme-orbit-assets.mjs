// Verifies the Orbit desktop theme Blender model pack shipped in ui/3d/desktop-theme-orbit/v1.
// No dependencies: parses the GLB containers directly and checks the contract the runtime relies on.
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const packDir = path.join(root, 'ui', '3d', 'desktop-theme-orbit', 'v1');
const BUDGET_BYTES = 1572864;

const FILES = {
  'station.glb': {
    nodes: ['RingRotor', 'BeaconRed', 'BeaconGreen', 'DockPort'],
    maxTriangles: 26000,
    emissive: true,
  },
  'shuttle.glb': {
    nodes: ['EngineGlow', 'Exhaust'],
    maxTriangles: 7000,
    emissive: true,
  },
  'satellite.glb': {
    nodes: ['Dish'],
    maxTriangles: 5000,
    emissive: false,
  },
  'asteroids.glb': {
    nodes: ['Rock0', 'Rock1', 'Rock2', 'Rock3', 'Rock4'],
    maxTriangles: Infinity,
    maxNodeTriangles: 800,
    emissive: false,
  },
};

const failures = [];
function fail(message) {
  failures.push(message);
}

function readPack(name) {
  try {
    return readFileSync(path.join(packDir, name));
  } catch (error) {
    fail(`${name}: cannot read (${error.code || error.message}); run the Blender generator`);
    return null;
  }
}

function parseGLB(name, bytes) {
  if (bytes.length < 20) {
    fail(`${name}: file is too short to be a GLB`);
    return null;
  }
  if (bytes.toString('ascii', 0, 4) !== 'glTF') {
    fail(`${name}: missing glTF magic`);
    return null;
  }
  if (bytes.readUInt32LE(4) !== 2) {
    fail(`${name}: GLB version ${bytes.readUInt32LE(4)} is not 2`);
    return null;
  }
  if (bytes.readUInt32LE(8) !== bytes.length) {
    fail(`${name}: GLB header length ${bytes.readUInt32LE(8)} != file size ${bytes.length}`);
    return null;
  }
  let offset = 12;
  while (offset + 8 <= bytes.length) {
    const length = bytes.readUInt32LE(offset);
    const type = bytes.readUInt32LE(offset + 4);
    offset += 8;
    if (type === 0x4e4f534a) {
      try {
        return JSON.parse(bytes.toString('utf8', offset, offset + length).replace(/[\u0000 ]+$/, ''));
      } catch (error) {
        fail(`${name}: JSON chunk does not parse (${error.message})`);
        return null;
      }
    }
    offset += length;
  }
  fail(`${name}: no JSON chunk`);
  return null;
}

function primitiveTriangles(json, primitive) {
  if (primitive.mode !== undefined && primitive.mode !== 4) return 0;
  if (primitive.indices !== undefined) return json.accessors[primitive.indices].count / 3;
  return json.accessors[primitive.attributes.POSITION].count / 3;
}

function meshTriangles(json, mesh) {
  return (mesh.primitives || []).reduce((sum, primitive) => sum + primitiveTriangles(json, primitive), 0);
}

let manifest = null;
const manifestBytes = readPack('manifest.json');
if (manifestBytes) {
  try {
    manifest = JSON.parse(manifestBytes.toString('utf8'));
  } catch (error) {
    fail(`manifest.json: does not parse (${error.message})`);
  }
}
if (manifest) {
  if (manifest.version !== 1) fail(`manifest.json: version ${manifest.version} is not 1`);
  const listed = (manifest.files || []).map(entry => entry.name).sort();
  const expected = Object.keys(FILES).sort();
  if (JSON.stringify(listed) !== JSON.stringify(expected)) {
    fail(`manifest.json: lists [${listed.join(', ')}] but must list exactly [${expected.join(', ')}]`);
  }
}
readPack('LICENSE.txt');

let total = 0;
for (const [name, rule] of Object.entries(FILES)) {
  const bytes = readPack(name);
  if (!bytes) continue;
  total += bytes.length;

  const entry = manifest && (manifest.files || []).find(file => file.name === name);
  if (manifest && !entry) {
    fail(`${name}: not listed in manifest.json`);
  } else if (entry) {
    if (entry.bytes !== bytes.length) fail(`${name}: manifest bytes ${entry.bytes} != actual ${bytes.length}`);
    const digest = createHash('sha256').update(bytes).digest('hex');
    if (entry.sha256 !== digest) fail(`${name}: manifest sha256 does not match the file`);
  }

  const json = parseGLB(name, bytes);
  if (!json) continue;

  for (const key of ['images', 'textures', 'samplers']) {
    if (json[key] && json[key].length) fail(`${name}: must not contain ${key} (the runtime has no blob image path)`);
  }

  const nodes = json.nodes || [];
  const names = new Set(nodes.map(node => node.name));
  for (const required of rule.nodes) {
    if (!names.has(required)) fail(`${name}: required node "${required}" is missing`);
  }

  const meshes = json.meshes || [];
  const triangles = meshes.reduce((sum, mesh) => sum + meshTriangles(json, mesh), 0);
  // Triangles actually placed in the scene graph (a mesh referenced twice counts twice).
  let placed = 0;
  for (const node of nodes) {
    if (node.mesh !== undefined) placed += meshTriangles(json, meshes[node.mesh]);
  }
  const counted = Math.max(triangles, placed);
  if (counted > rule.maxTriangles) fail(`${name}: ${counted} triangles exceed the ${rule.maxTriangles} budget`);
  if (entry && entry.triangles !== undefined && entry.triangles !== counted) {
    fail(`${name}: manifest triangles ${entry.triangles} != counted ${counted}`);
  }
  if (rule.maxNodeTriangles) {
    for (const required of rule.nodes) {
      const node = nodes.find(candidate => candidate.name === required);
      if (!node) continue;
      if (node.mesh === undefined) {
        fail(`${name}: node "${required}" carries no mesh`);
        continue;
      }
      const count = meshTriangles(json, meshes[node.mesh]);
      if (count > rule.maxNodeTriangles) {
        fail(`${name}: "${required}" has ${count} triangles, over the ${rule.maxNodeTriangles} per-rock budget`);
      }
      if (count < 100) fail(`${name}: "${required}" has only ${count} triangles`);
    }
  }

  if (rule.emissive) {
    const glowing = (json.materials || []).filter(material => {
      if (!/Emissive/.test(material.name || '')) return false;
      const factor = material.emissiveFactor || [0, 0, 0];
      return factor.some(value => value > 0);
    });
    if (!glowing.length) fail(`${name}: needs a material named *Emissive* with a non-zero emissiveFactor`);
  }
}

if (total > BUDGET_BYTES) fail(`pack is ${total} bytes, over the ${BUDGET_BYTES} byte budget`);

if (failures.length) {
  console.error('Orbit model pack check failed:');
  for (const message of failures) console.error(`  - ${message}`);
  process.exit(1);
}
console.log(`Verified orbit model pack (${total} bytes)`);
