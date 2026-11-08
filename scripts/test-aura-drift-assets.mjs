// Verifies the Aura Drift Blender model pack shipped in ui/3d/aura-drift/v1 (contract: plan section 3).
// No dependencies: parses the GLB containers directly. Adapted from test-desktop-theme-orbit-assets.mjs.
//
// AURAGO_AURA_DRIFT_ASSETS_PARTIAL=1  skips the exact-file-set check (parallel authors, partial packs).
// AURAGO_AURA_DRIFT_PACK_DIR=<abs>    checks another folder (scratch builds) instead of the shipped pack.
import { createHash } from 'node:crypto';
import { readFileSync, readdirSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const packDir = process.env.AURAGO_AURA_DRIFT_PACK_DIR || path.join(root, 'ui', '3d', 'aura-drift', 'v1');
const PARTIAL = process.env.AURAGO_AURA_DRIFT_ASSETS_PARTIAL === '1';
const KB = 1024;
const MIB = 1048576;
const PACK_BUDGET_BYTES = 18 * MIB;

const CAR_IDS = ['vektor', 'kobalt', 'lumen', 'tundra', 'softeis', 'maehkoenig', 'couch', 'bertha'];
const CAR_NODES = ['body', 'wheel_fl', 'wheel_fr', 'wheel_rl', 'wheel_rr'];
const CAR_ANCHORS = [
  'anchor_exhaust_l', 'anchor_exhaust_r', 'anchor_head_l', 'anchor_head_r', 'anchor_tail_l', 'anchor_tail_r', 'anchor_weapon',
];
const BODY_MATERIALS = ['paint', 'paint_accent', 'trim', 'glass', 'chrome', 'head_light', 'brake_light'];
const CAR_CHILDREN = {
  softeis: ['cone'],
  maehkoenig: ['blade', 'gnome'],
  couch: ['lamp', 'cushion'],
};
const LEGS = ['leg_fl', 'leg_fr', 'leg_rl', 'leg_rr'];

// name -> {kind, nodes: required top-level nodes, children: {node: [children]}, maxTriangles, maxBytes}
const FILES = {};
for (const id of CAR_IDS) {
  FILES[`car-${id}.glb`] = { kind: 'car', id, maxTriangles: 30000, maxBytes: 1.25 * MIB };
}
FILES['items.glb'] = {
  kind: 'items',
  nodes: ['crate', 'gnome_mine', 'rubber_duck', 'rocket', 'bratwurst', 'boxing_glove', 'disco_ball', 'harpoon', 'air_horn', 'frost_grenade'].map(n => `item-${n}`),
  maxTriangles: 12000,
  maxBytes: 450 * KB,
};
FILES['track.glb'] = {
  kind: 'track',
  nodes: ['ramp', 'boost_pad', 'cone', 'hay_bale', 'barrel', 'tyre_stack', 'barrier_segment', 'gantry', 'rumble_bars'].map(n => `track-${n}`),
  children: { 'track-gantry': ['light_1', 'light_2', 'light_3', 'light_4', 'light_5'] },
  maxTriangles: 12000,
  maxBytes: 450 * KB,
};
FILES['animals.glb'] = {
  kind: 'animals',
  nodes: ['cow', 'sheep', 'penguin', 'crab', 'llama'].map(n => `animal-${n}`),
  maxNodeTriangles: 1800,
  maxTriangles: Infinity,
  maxBytes: 320 * KB,
};
FILES['shared.glb'] = {
  kind: 'shared',
  nodes: ['grandstand', 'fan_camp', 'hoarding', 'marshal_post', 'gnome_statue', 'hotdog_stand', 'camper_van', 'inflatable_arch', 'road_sign', 'balloon_bunch', 'photo_tower', 'windsock', 'rain_shelter'].map(n => `prop-${n}`),
  children: {
    'prop-hoarding': ['face'],
    'prop-road_sign': ['face'],
    'prop-grandstand': ['frame', 'crowd', 'crowd_arms'],
    'prop-fan_camp': ['frame', 'crowd', 'crowd_arms'],
    'prop-marshal_post': ['frame', 'crowd', 'crowd_arms'],
    'prop-hotdog_stand': ['frame', 'crowd', 'crowd_arms'],
    'prop-camper_van': ['frame', 'crowd', 'crowd_arms'],
  },
  maxTriangles: 36000,
  maxBytes: Math.floor(1.75 * MIB),
};
const SCENERY = {
  alpine: ['pine_a', 'pine_b', 'rock_a', 'rock_b', 'chalet', 'cablecar_pylon', 'flower_meadow', 'hay_barn', 'trail_marker', 'snow_gauge'],
  desert: ['cactus_a', 'cactus_b', 'mesa_rock', 'water_tower', 'windmill_pump', 'rock_arch', 'palm_oasis', 'sunshade', 'anemometer'],
  harbour: ['container_stack', 'crane', 'lighthouse', 'fishing_boat', 'neon_sign', 'tugboat', 'buoy_cluster', 'signal_mast', 'storm_lantern'],
  salt: ['salt_crystal', 'ufo', 'rover', 'flag', 'radar_dome', 'geyser_pool'],
  forest: ['oak', 'birch', 'mushroom_cluster', 'log_pile', 'hunting_hide', 'fern_glade', 'watermill'],
  industry: ['silo', 'smokestack', 'pipes', 'forklift', 'pylon', 'cooling_tower', 'chem_tank'],
  glacier: ['ice_block', 'ice_arch', 'snow_pine', 'igloo', 'research_station', 'ice_spire', 'snow_cat', 'snow_fence'],
  metro: ['skyscraper_a', 'skyscraper_b', 'skyscraper_c', 'billboard', 'street_lamp', 'food_truck', 'taxi', 'metro_entrance', 'arcade_kiosk', 'holo_arrow'],
};
const SCENERY_CHILDREN = { desert: { 'prop-windmill_pump': ['rotor'] }, metro: { 'prop-billboard': ['face'] } };
for (const [biome, props] of Object.entries(SCENERY)) {
  FILES[`scenery-${biome}.glb`] = {
    kind: 'scenery',
    biome,
    nodes: props.map(n => `prop-${n}`),
    children: SCENERY_CHILDREN[biome] || {},
    maxTriangles: 30000,
    maxBytes: Math.floor(1.1 * MIB),
  };
}
const PROP_BUDGET = { scatter: 450, default: 1200, hero: 4500, crowd: 14000 };

const failures = [];
function fail(message) {
  failures.push(message);
}

function readFile(name) {
  try {
    return readFileSync(path.join(packDir, name));
  } catch (error) {
    fail(`${name}: cannot read (${error.code || error.message}); run the Blender generator`);
    return null;
  }
}

function parseGLB(name, bytes) {
  if (bytes.length < 20) return fail(`${name}: file is too short to be a GLB`), null;
  if (bytes.toString('ascii', 0, 4) !== 'glTF') return fail(`${name}: missing glTF magic`), null;
  if (bytes.readUInt32LE(4) !== 2) return fail(`${name}: GLB version ${bytes.readUInt32LE(4)} is not 2`), null;
  if (bytes.readUInt32LE(8) !== bytes.length) {
    return fail(`${name}: GLB header length ${bytes.readUInt32LE(8)} != file size ${bytes.length}`), null;
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
        return fail(`${name}: JSON chunk does not parse (${error.message})`), null;
      }
    }
    offset += length;
  }
  return fail(`${name}: no JSON chunk`), null;
}

function primitiveTriangles(json, primitive) {
  if (primitive.mode !== undefined && primitive.mode !== 4) return 0;
  if (primitive.indices !== undefined) return json.accessors[primitive.indices].count / 3;
  return json.accessors[primitive.attributes.POSITION].count / 3;
}
const meshTriangles = (json, mesh) => (mesh.primitives || []).reduce((sum, p) => sum + primitiveTriangles(json, p), 0);

// Triangles of a node and all its descendants (a shared mesh counts per placement).
function nodeTriangles(json, index) {
  const node = json.nodes[index];
  let total = node.mesh !== undefined ? meshTriangles(json, json.meshes[node.mesh]) : 0;
  for (const child of node.children || []) total += nodeTriangles(json, child);
  return total;
}

function topLevel(json) {
  const scene = (json.scenes || [])[json.scene || 0] || { nodes: [] };
  const map = new Map();
  for (const index of scene.nodes || []) map.set(json.nodes[index].name, index);
  return map;
}

function childNames(json, index) {
  return new Set((json.nodes[index].children || []).map(child => json.nodes[child].name));
}

function materialNames(json, index) {
  const node = json.nodes[index];
  const names = new Set();
  if (node.mesh === undefined) return names;
  for (const primitive of json.meshes[node.mesh].primitives || []) {
    const material = (json.materials || [])[primitive.material];
    names.add(material ? material.name : '(none)');
  }
  return names;
}

function checkCar(name, rule, json, top) {
  for (const required of [...CAR_NODES, ...CAR_ANCHORS]) {
    if (!top.has(required)) fail(`${name}: required top-level node "${required}" is missing`);
  }
  for (const anchor of CAR_ANCHORS) {
    if (top.has(anchor) && json.nodes[top.get(anchor)].mesh !== undefined) fail(`${name}: "${anchor}" must be an empty`);
  }
  if (top.has('body')) {
    const used = materialNames(json, top.get('body'));
    const required = json.images?.length ? ['paint', 'trim', 'head_light', 'brake_light'] : BODY_MATERIALS;
    for (const material of required) {
      if (!used.has(material)) fail(`${name}: body mesh lacks material "${material}"`);
    }
    for (const material of used) {
      if (!BODY_MATERIALS.includes(material)) fail(`${name}: body mesh has unexpected material "${material}"`);
    }
    if (used.size > 7) fail(`${name}: body mesh uses ${used.size} materials (at most 7)`);
    const children = childNames(json, top.get('body'));
    for (const child of CAR_CHILDREN[rule.id] || []) {
      if (!children.has(child)) fail(`${name}: body lacks the animated child "${child}"`);
    }
  }
  for (const wheel of CAR_NODES.slice(1)) {
    if (!top.has(wheel)) continue;
    const node = json.nodes[top.get(wheel)];
    if (node.mesh === undefined) fail(`${name}: "${wheel}" carries no mesh`);
    const used = [...materialNames(json, top.get(wheel))];
    if (used.length !== 1 || used[0] !== 'tyre') fail(`${name}: "${wheel}" must use exactly the material "tyre" (has ${used.join(', ')})`);
    // wheels rest on y = 0: the centre sits one radius above the ground (translation y > 0.2)
    const y = (node.translation || [0, 0, 0])[1];
    if (!(y > 0.2 && y < 1.5)) fail(`${name}: "${wheel}" centre height ${y} looks wrong (wheels rest on y=0)`);
  }
  for (const material of ['head_light', 'brake_light']) {
    const entry = (json.materials || []).find(m => m.name === material);
    if (!entry) continue;
    if (!(entry.emissiveFactor || [0, 0, 0]).some(value => value > 0)) fail(`${name}: "${material}" needs a non-zero emissiveFactor`);
  }
  const allowed = new Set([...BODY_MATERIALS, 'tyre']);
  for (const material of json.materials || []) {
    if (!allowed.has(material.name)) fail(`${name}: unexpected material "${material.name}"`);
  }
  // nose on +X: the head anchors must be ahead of the tail anchors
  if (top.has('anchor_head_r') && top.has('anchor_tail_r')) {
    const head = json.nodes[top.get('anchor_head_r')].translation || [0, 0, 0];
    const tail = json.nodes[top.get('anchor_tail_r')].translation || [0, 0, 0];
    if (!(head[0] > tail[0])) fail(`${name}: head anchor is not ahead of the tail anchor (nose must point to +X)`);
    if (!(head[2] > 0)) fail(`${name}: anchor_head_r must sit on the right side (+Z)`);
  }
}

function checkPieces(name, rule, json, top) {
  for (const required of rule.nodes || []) {
    if (!top.has(required)) {
      fail(`${name}: required node "${required}" is missing`);
      continue;
    }
    const index = top.get(required);
    const wanted = (rule.children || {})[required] || [];
    const have = childNames(json, index);
    for (const child of wanted) if (!have.has(child)) fail(`${name}: "${required}" lacks the child "${child}"`);
    if (rule.kind === 'animals') {
      for (const part of ['body', 'head', 'tail', ...LEGS]) {
        if (!have.has(part)) fail(`${name}: "${required}" lacks the child "${part}"`);
      }
    }
    const count = nodeTriangles(json, index);
    if (rule.maxNodeTriangles && count > rule.maxNodeTriangles) fail(`${name}: "${required}" has ${count} triangles, over ${rule.maxNodeTriangles}`);
  }
}

function checkUVs(name, json) {
  const faceMeshes = new Set();
  for (const node of json.nodes || []) if (node.name === 'face' && node.mesh !== undefined) faceMeshes.add(node.mesh);
  (json.meshes || []).forEach((mesh, index) => {
    for (const primitive of mesh.primitives || []) {
      const has = primitive.attributes.TEXCOORD_0 !== undefined;
      const textured = json.materials?.[primitive.material]?.pbrMetallicRoughness?.baseColorTexture !== undefined;
      if (has && !faceMeshes.has(index) && !json.images?.length) fail(`${name}: mesh "${mesh.name}" has UVs without retained artwork`);
      if (textured && !has) fail(`${name}: textured mesh "${mesh.name}" lacks UVs`);
    }
  });
  for (const node of json.nodes || []) {
    if (node.name === 'face' && node.mesh !== undefined) {
      const ok = (json.meshes[node.mesh].primitives || []).every(p => p.attributes.TEXCOORD_0 !== undefined);
      if (!ok) fail(`${name}: "face" mesh needs UVs (0 to 1)`);
    }
  }
}

let manifest = null;
const manifestBytes = readFile('manifest.json');
if (manifestBytes) {
  try {
    manifest = JSON.parse(manifestBytes.toString('utf8'));
  } catch (error) {
    fail(`manifest.json: does not parse (${error.message})`);
  }
}
readFile('LICENSE.txt');

const present = readdirSync(packDir).filter(name => name.endsWith('.glb')).sort();
for (const name of present) if (!FILES[name]) fail(`${name}: not part of the pack contract`);
if (!PARTIAL) {
  for (const name of Object.keys(FILES)) if (!present.includes(name)) fail(`${name}: missing (set AURAGO_AURA_DRIFT_ASSETS_PARTIAL=1 for partial packs)`);
}
if (manifest) {
  if (manifest.version !== 1) fail(`manifest.json: version ${manifest.version} is not 1`);
  const listed = (manifest.files || []).map(entry => entry.name).sort();
  if (JSON.stringify(listed) !== JSON.stringify(present)) {
    fail(`manifest.json: lists [${listed.join(', ')}] but the folder holds [${present.join(', ')}]; run generate.py --only manifest`);
  }
}

let total = 0;
for (const name of present) {
  const rule = FILES[name];
  if (!rule) continue;
  const bytes = readFile(name);
  if (!bytes) continue;
  total += bytes.length;
  if (bytes.length > rule.maxBytes) fail(`${name}: ${bytes.length} bytes exceed the ${rule.maxBytes} byte budget`);

  const json = parseGLB(name, bytes);
  if (!json) continue;
  for (const key of ['animations', 'skins', ...(rule.kind === 'car' ? [] : ['images', 'textures', 'samplers'])]) {
    if (json[key] && json[key].length) fail(`${name}: must not contain ${key}`);
  }
  if (json.images?.length) {
    if (json.images.length !== 1 || json.textures?.length !== 1) fail(`${name}: expected one shared paint atlas`);
    for (const image of json.images) {
      const view = json.bufferViews?.[image.bufferView];
      if (image.uri || image.mimeType !== 'image/webp' || !view || view.byteLength > 350 * KB) fail(`${name}: paint atlas must be embedded WebP, at most 350 KiB`);
    }
    for (const material of json.materials || []) {
      if (material.pbrMetallicRoughness?.baseColorTexture && material.alphaMode && material.alphaMode !== 'OPAQUE') fail(`${name}: paint mask must not enable transparency`);
    }
  }

  const nodes = json.nodes || [];
  const meshes = json.meshes || [];
  const unique = meshes.reduce((sum, mesh) => sum + meshTriangles(json, mesh), 0);
  let placed = 0;
  for (const node of nodes) if (node.mesh !== undefined) placed += meshTriangles(json, meshes[node.mesh]);
  const counted = Math.max(unique, placed);
  if (counted > rule.maxTriangles) fail(`${name}: ${counted} triangles exceed the ${rule.maxTriangles} budget`);

  const entry = manifest && (manifest.files || []).find(file => file.name === name);
  if (manifest && !entry) {
    fail(`${name}: not listed in manifest.json`);
  } else if (entry) {
    if (entry.bytes !== bytes.length) fail(`${name}: manifest bytes ${entry.bytes} != actual ${bytes.length}`);
    if (entry.sha256 !== createHash('sha256').update(bytes).digest('hex')) fail(`${name}: manifest sha256 does not match the file`);
    if (entry.triangles !== counted) fail(`${name}: manifest triangles ${entry.triangles} != counted ${counted}`);
    if (entry.kind !== rule.kind) fail(`${name}: manifest kind "${entry.kind}" != "${rule.kind}"`);
  }

  const top = topLevel(json);
  if (rule.kind === 'car') {
    checkCar(name, rule, json, top);
    const car = manifest && manifest.cars && manifest.cars[rule.id];
    if (manifest && !car) fail(`${name}: manifest.cars.${rule.id} is missing`);
    else if (car) {
      for (const key of ['length', 'width', 'height', 'wheelRadius', 'wheelbase']) {
        if (typeof car[key] !== 'number' || !(car[key] > 0)) fail(`${name}: manifest.cars.${rule.id}.${key} must be a positive number`);
      }
      if (car.file !== name) fail(`${name}: manifest.cars.${rule.id}.file is "${car.file}"`);
    }
  } else {
    checkPieces(name, rule, json, top);
  }
  checkUVs(name, json);

  if (rule.kind === 'scenery' || rule.kind === 'shared') {
    const group = rule.kind === 'shared' ? 'shared' : rule.biome;
    const props = (manifest && manifest.props && manifest.props[group]) || {};
    for (const required of rule.nodes) {
      const key = required.slice('prop-'.length);
      const prop = props[key];
      if (manifest && !prop) {
        fail(`${name}: manifest.props.${group}.${key} is missing`);
        continue;
      }
      if (!top.has(required) || !prop) continue;
      const tags = prop.tags || [];
      const budget = tags.includes('crowd') ? PROP_BUDGET.crowd : tags.includes('hero') ? PROP_BUDGET.hero : tags.includes('scatter') ? PROP_BUDGET.scatter : PROP_BUDGET.default;
      const count = nodeTriangles(json, top.get(required));
      if (count > budget) fail(`${name}: "${required}" has ${count} triangles, over the ${budget} budget for tags [${tags.join(', ')}]`);
      if (prop.triangles !== count) fail(`${name}: manifest triangles for ${required} (${prop.triangles}) != counted ${count}`);
    }
  }
}

if (total > PACK_BUDGET_BYTES) fail(`pack is ${total} bytes, over the ${PACK_BUDGET_BYTES} byte budget`);
if (manifest && manifest.totalBytes !== total) fail(`manifest.json: totalBytes ${manifest.totalBytes} != ${total}`);

if (failures.length) {
  console.error('Aura Drift model pack check failed:');
  for (const message of failures) console.error(`  - ${message}`);
  process.exit(1);
}
console.log(`Verified Aura Drift model pack: ${present.length} GLB files, ${total} bytes${PARTIAL ? ' (partial mode)' : ''}`);
