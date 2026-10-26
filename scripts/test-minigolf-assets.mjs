// Checks the AuraGolf runtime assets (manifest version 2): budgets, the course GLBs with their collision nodes,
// placements, mechanics, obstacles, prop kits and menu previews.
//
//   node scripts/test-minigolf-assets.mjs
//
// Environment:
//   AURAGO_MINIGOLF_DIR=<dir>   asset folder (default ui/3d/minigolf)
//   AURAGO_MINIGOLF_PARTIAL=1   accept a subset of the 18 courses (staging builds): missing previews are not an
//                               error and the "every kit prop is placed" rule is only reported
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFile, readdir, stat } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import validator from 'gltf-validator';
import { colliderDigest, readColliders } from '../ui/js/desktop/apps/minigolf-collision.js';
import { obstaclePose, validateMechanic, validateObstacle } from '../ui/js/desktop/apps/minigolf-mechanics.js';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const assetRoot = path.resolve(root, process.env.AURAGO_MINIGOLF_DIR || 'ui/3d/minigolf');
const partial = process.env.AURAGO_MINIGOLF_PARTIAL === '1';
const manifestBytes = await readFile(path.join(assetRoot, 'manifest.json'));
const manifest = JSON.parse(manifestBytes.toString('utf8'));

const KIB = 1024, MIB = 1024 * 1024;
const WORLDS = ['pirate', 'temple', 'moon'];
const FIRST_COURSE = { pirate: 1, temple: 7, moon: 13 };
const GRAVITY = { pirate: 9.81, temple: 9.81, moon: 4.0 };
const COURSE_COUNT = 18, BALL_RADIUS = 0.16;
// Largest sizes a manifest, course, kit, prop or preview may reach (P3 design, section 8).
const BUDGET = { manifest: 256 * KIB, courseBytes: 1.25 * MIB, courseTriangles: 36_000, meshNodes: 32, placements: 220, kitBytes: 3 * MIB, kitTriangles: 60_000, prop: 15_000, hero: 20_000, preview: 80 * KIB, package: 32 * MIB };
const REQUIRED_FIELDS = ['id', 'number', 'world', 'par', 'model', 'sha256', 'bytes', 'triangles', 'tee', 'cup', 'cupRadius', 'gravity', 'cycle', 'length', 'bounds', 'water', 'collision', 'obstacles', 'mechanics', 'path', 'corridors', 'props', 'island'];
const COLLIDER_NODE = { floor: ['grass', 'sand'], rail: ['wood', 'stone', 'metal'] };
// Placement rules of kit/placement.py: distance of a prop's centre to the island rim (EDGE plus its share of the footprint radius).
const EDGE = 0.3, RIM_TOLERANCE = 0.01;
const RIM_SHARE_BY_NAME = { fern: 0.5, bush: 0.5, tuft: 0.5, seaweed: 0.4, shell: 0.6, crab: 0.6, starfish: 0.6 };
const rimShare = (name, tags) => RIM_SHARE_BY_NAME[name] ?? (tags.includes('tall') ? 0.6 : tags.includes('hero') ? 0.85 : 1.0);
const idOf = number => `${WORLDS.filter(world => number >= FIRST_COURSE[world]).pop()}-${String(number).padStart(2, '0')}`;

assert.equal(manifest.version, 2);
assert.ok(manifestBytes.length <= BUDGET.manifest, `manifest is ${manifestBytes.length} bytes (max ${BUDGET.manifest})`);
const numbers = manifest.courses.map(course => course.number);
if (partial) {
  assert.ok(numbers.length >= 1 && numbers.length <= COURSE_COUNT, `${numbers.length} courses`);
  assert.deepEqual(numbers, [...new Set(numbers)].sort((a, b) => a - b), 'course numbers ascend');
} else {
  assert.equal(manifest.courses.length, COURSE_COUNT);
  assert.deepEqual(WORLDS.map(world => manifest.courses.filter(course => course.world === world).length), [6, 6, 6]);
  assert.deepEqual(numbers, Array.from({ length: COURSE_COUNT }, (_, i) => i + 1));
}

// Largest possible distance of (x, z) inside the rim: the rim is the superellipse scaled by at most 1 +/- wobble.
function rimReserve(island, x, z) {
  const dx = x - island.center[0], dz = z - island.center[1], theta = Math.atan2(dz, dx);
  const [hx, hz] = island.half, n = island.power;
  const ellipse = (Math.abs(Math.cos(theta) / hx) ** n + Math.abs(Math.sin(theta) / hz) ** n) ** (-1 / n);
  return ellipse * (1 + island.wobble) - Math.hypot(dx, dz);
}
function pathDistance(points, x, z) {
  let best = Infinity;
  for (let i = 1; i < points.length; i++) {
    const [ax, , az] = points[i - 1], [bx, , bz] = points[i], vx = bx - ax, vz = bz - az;
    const t = Math.max(0, Math.min(1, ((x - ax) * vx + (z - az) * vz) / Math.max(1e-9, vx * vx + vz * vz)));
    best = Math.min(best, Math.hypot(x - ax - t * vx, z - az - t * vz));
  }
  return best;
}
const polylineLength = points => points.slice(1).reduce((sum, point, i) => sum + Math.hypot(...point.map((value, axis) => value - points[i][axis])), 0);
const close3 = (actual, expected, message, eps = 0.0001) => actual.forEach((value, i) => assert.ok(Math.abs(value - expected[i]) < eps, `${message}[${i}]: ${value} != ${expected[i]}`));
const vec3 = value => Array.isArray(value) && value.length === 3 && value.every(Number.isFinite);
const inside = (bounds, point) => point.every((value, axis) => value >= bounds.min[axis] && value <= bounds.max[axis]);

function parseGLB(bytes) {
  assert.equal(bytes.toString('ascii', 0, 4), 'glTF');
  assert.equal(bytes.readUInt32LE(4), 2);
  assert.equal(bytes.readUInt32LE(8), bytes.length);
  let offset = 12, json, binary;
  while (offset < bytes.length) {
    const length = bytes.readUInt32LE(offset), type = bytes.readUInt32LE(offset + 4);
    offset += 8;
    const chunk = bytes.subarray(offset, offset + length);
    if (type === 0x4e4f534a) json = JSON.parse(chunk.toString('utf8').replace(/[\u0000 ]+$/, ''));
    if (type === 0x004e4942) binary = chunk;
    offset += length;
  }
  assert.ok(json && binary, 'GLB must contain embedded JSON and binary chunks');
  assert.ok(json.buffers.every(buffer => !buffer.uri), 'GLB buffers must stay embedded');
  assert.ok((json.images ?? []).every(image => !image.uri), 'GLB images must stay embedded');
  const parents = new Map();
  for (const [parent, node] of json.nodes.entries()) for (const child of node.children ?? []) parents.set(child, parent);
  const transformLocal = (node, point) => {
    if (node.matrix) {
      const m = node.matrix;
      return [m[0] * point[0] + m[4] * point[1] + m[8] * point[2] + m[12], m[1] * point[0] + m[5] * point[1] + m[9] * point[2] + m[13], m[2] * point[0] + m[6] * point[1] + m[10] * point[2] + m[14]];
    }
    const s = node.scale ?? [1, 1, 1], q = node.rotation ?? [0, 0, 0, 1], t = node.translation ?? [0, 0, 0];
    const x = point[0] * s[0], y = point[1] * s[1], z = point[2] * s[2];
    const [qx, qy, qz, qw] = q;
    const tx = 2 * (qy * z - qz * y), ty = 2 * (qz * x - qx * z), tz = 2 * (qx * y - qy * x);
    return [x + qw * tx + qy * tz - qz * ty + t[0], y + qw * ty + qz * tx - qx * tz + t[1], z + qw * tz + qx * ty - qy * tx + t[2]];
  };
  const worldPoint = (index, point) => {
    let value = point, current = index;
    while (current !== undefined) { value = transformLocal(json.nodes[current], value); current = parents.get(current); }
    return value;
  };
  const readAccessor = index => {
    const accessor = json.accessors[index], view = json.bufferViews[accessor.bufferView];
    const component = {
      5121: { bytes: 1, read: (offset) => binary.readUInt8(offset) },
      5123: { bytes: 2, read: (offset) => binary.readUInt16LE(offset) },
      5125: { bytes: 4, read: (offset) => binary.readUInt32LE(offset) },
      5126: { bytes: 4, read: (offset) => binary.readFloatLE(offset) }
    }[accessor.componentType];
    assert.ok(component, `unsupported accessor component type ${accessor.componentType}`);
    if (accessor.type === 'VEC3') assert.equal(accessor.componentType, 5126, 'positions must be float32');
    const components = accessor.type === 'VEC3' ? 3 : accessor.type === 'SCALAR' ? 1 : 0;
    assert.ok(components, `unsupported accessor type ${accessor.type}`);
    const stride = view.byteStride ?? components * component.bytes;
    const start = (view.byteOffset ?? 0) + (accessor.byteOffset ?? 0);
    const out = [];
    for (let i = 0; i < accessor.count; i++) {
      const values = [];
      for (let c = 0; c < components; c++) values.push(component.read(start + i * stride + c * component.bytes));
      out.push(values);
    }
    return out;
  };
  const nodeByName = new Map();
  const geometryByNode = new Map();
  for (const [index, node] of json.nodes.entries()) {
    if (!node.name) continue;
    nodeByName.set(node.name, { node, index });
    if (node.mesh === undefined) continue;
    const meshes = json.meshes[node.mesh].primitives.map(primitive => {
      const positions = readAccessor(primitive.attributes.POSITION).map(point => worldPoint(index, point));
      const indices = primitive.indices === undefined ? positions.map((_, i) => i) : readAccessor(primitive.indices).map(value => value[0]);
      const faces = [];
      if ((primitive.mode ?? 4) === 4) for (let i = 0; i + 2 < indices.length; i += 3) faces.push([positions[indices[i]], positions[indices[i + 1]], positions[indices[i + 2]]]);
      return { positions, faces };
    });
    geometryByNode.set(node.name, meshes);
  }
  const triangleCount = json.meshes.flatMap(mesh => mesh.primitives).reduce((sum, primitive) => {
    if ((primitive.mode ?? 4) !== 4) return sum;
    const accessor = json.accessors[primitive.indices ?? primitive.attributes.POSITION];
    return sum + Math.floor(accessor.count / 3);
  }, 0);
  return { json, nodeByName, parents, geometryByNode, triangleCount };
}

// The flat tee ring is joined into the scenery nodes, so it is found by shape: all three vertices on the inner or
// outer circle, at the ring's height. Its material is single sided, hence the faces must point up (+Y), or the
// runtime culls them seen from above.
const RING_TOLERANCE = 0.002;
function flatRingFaces(glb, centre, inner, outer, y) {
  const onCircle = point => {
    const radius = Math.hypot(point[0] - centre[0], point[2] - centre[2]);
    return Math.abs(radius - inner) < RING_TOLERANCE || Math.abs(radius - outer) < RING_TOLERANCE;
  };
  return [...glb.geometryByNode].filter(([name]) => name.startsWith('scenery-')).flatMap(([, meshes]) => meshes)
    .flatMap(mesh => mesh.faces).filter(face => face.every(point => onCircle(point) && Math.abs(point[1] - y) < RING_TOLERANCE));
}
const faceNormalY = ([a, b, c]) => (b[2] - a[2]) * (c[0] - a[0]) - (b[0] - a[0]) * (c[2] - a[2]);

// Heights of the visible floor triangles (every floor-* node except the cup tube) that cover (x, z) in plan view.
function floorHeightsAt(faces, x, z) {
  const heights = [];
  for (const [a, b, c] of faces) {
    const d = (b[2] - c[2]) * (a[0] - c[0]) + (c[0] - b[0]) * (a[2] - c[2]);
    if (Math.abs(d) < 1e-12) continue;
    const u = ((b[2] - c[2]) * (x - c[0]) + (c[0] - b[0]) * (z - c[2])) / d, v = ((c[2] - a[2]) * (x - c[0]) + (a[0] - c[0]) * (z - c[2])) / d, w = 1 - u - v;
    if (u >= -1e-6 && v >= -1e-6 && w >= -1e-6) heights.push(u * a[1] + v * b[1] + w * c[1]);
  }
  return heights;
}

let totalBytes = 0, totalTriangles = 0, warningCount = 0;
const placedProps = new Map(WORLDS.map(world => [world, new Set()]));
for (const course of manifest.courses) {
  for (const field of REQUIRED_FIELDS) assert.ok(field in course, `${course.id}: manifest field ${field}`);
  assert.equal(course.colliders, undefined, `${course.id}: manifest v2 carries no collider arrays`);
  assert.equal(course.look, undefined, `${course.id}: manifest v2 has no look (the world is the look)`);
  assert.equal(course.id, idOf(course.number), `${course.id}: id of course ${course.number}`);
  assert.equal(course.world, course.id.split('-')[0], `${course.id}: world`);
  assert.ok(course.par >= 2 && course.par <= 5, `${course.id}: par ${course.par}`);
  assert.equal(course.model, `course-${String(course.number).padStart(2, '0')}.glb`);
  assert.equal(course.gravity, GRAVITY[course.world], `${course.id}: gravity`);
  assert.ok(Number.isFinite(course.length) && course.length >= 10 && course.length <= 35, `${course.id}: length ${course.length}`);
  assert.ok(Math.abs(polylineLength(course.path) - course.length) <= Math.max(0.5, course.length * 0.03), `${course.id}: length ${course.length} vs path ${polylineLength(course.path).toFixed(2)}`);
  const bytes = await readFile(path.join(assetRoot, course.model));
  assert.equal(bytes.length, course.bytes, `${course.id}: byte count`);
  assert.equal(createHash('sha256').update(bytes).digest('hex'), course.sha256, `${course.id}: SHA-256`);
  const glb = parseGLB(bytes);
  assert.equal(glb.triangleCount, course.triangles, `${course.id}: triangle count`);
  // A numbered name ("scenery-matte.001") means a datablock of another course or part was still around at export time.
  const numbered = [...glb.json.nodes, ...glb.json.meshes].map(entry => entry.name).filter(name => /\.\d{3}$/.test(name ?? ''));
  assert.deepEqual(numbered, [], `${course.id}: numbered node or mesh names`);
  assert.ok(course.bytes <= BUDGET.courseBytes, `${course.id}: course GLB exceeds ${BUDGET.courseBytes / KIB} KiB (${course.bytes})`);
  assert.ok(course.triangles <= BUDGET.courseTriangles, `${course.id}: triangle budget (${course.triangles})`);
  const meshNodes = glb.json.nodes.filter(node => node.mesh !== undefined).length;
  assert.ok(meshNodes <= BUDGET.meshNodes, `${course.id}: ${meshNodes} mesh nodes (max ${BUDGET.meshNodes})`);

  // Collision lives in the GLB: visible floor-* nodes and hidden rail-* nodes, welded per surface, fingerprinted by the manifest.
  const colliderNodes = glb.json.nodes.filter(node => node.extras?.aurago_collider !== undefined);
  assert.equal(course.collision.nodes, colliderNodes.length, `${course.id}: collision node count`);
  let floorNodes = 0, railNodes = 0;
  for (const node of colliderNodes) {
    const kind = node.name.split('-')[0], surface = node.extras.aurago_collider;
    assert.ok(COLLIDER_NODE[kind]?.includes(surface), `${course.id}: collision node ${node.name} (${surface}) must be floor-* grass|sand or rail-* wood|stone|metal`);
    assert.equal(node.mesh !== undefined, true, `${course.id}: ${node.name} is a mesh`);
    if (kind === 'floor') { floorNodes++; assert.notEqual(node.extras.aurago_hidden, true, `${course.id}: ${node.name} is visible`); }
    else { railNodes++; assert.equal(node.extras.aurago_hidden, true, `${course.id}: ${node.name} is hidden at runtime`); }
  }
  assert.ok(floorNodes >= 1 && railNodes >= 1, `${course.id}: floor and rail collision nodes`);
  assert.ok(glb.nodeByName.has('rail-trim'), `${course.id}: missing rail-trim`);
  assert.equal(glb.nodeByName.get('rail-trim').node.extras?.aurago_collider, undefined, `${course.id}: rail-trim is plain decor`);
  const groups = readColliders(bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength));
  assert.equal(groups.reduce((sum, group) => sum + group.indices.length / 3, 0), course.collision.triangles, `${course.id}: welded collision triangle count`);
  assert.equal(await colliderDigest(groups), course.collision.digest, `${course.id}: collision digest of the GLB equals the manifest`);

  // Tee, cup, bounds and the routes.
  for (const key of ['tee', 'cup']) assert.ok(vec3(course[key]), `${course.id}: ${key} must be three finite numbers`);
  assert.ok(Number.isFinite(course.cupRadius) && course.cupRadius > 0.2 && course.cupRadius < 0.5, `${course.id}: cup radius`);
  assert.ok(course.bounds.min.every((v, i) => Number.isFinite(v) && v < course.bounds.max[i]), `${course.id}: bounds`);
  assert.ok(inside(course.bounds, course.tee) && inside(course.bounds, course.cup), `${course.id}: tee and cup inside bounds`);
  assert.ok(Array.isArray(course.water) && course.water.every(volume => vec3(volume.min) && vec3(volume.max) && volume.min.every((v, i) => v < volume.max[i])), `${course.id}: water volumes`);
  if (course.world !== 'moon') assert.ok(course.water.length > 0, `${course.id}: sea water volume`);
  assert.ok(Array.isArray(course.path) && course.path.length >= 8 && course.path.every(vec3), `${course.id}: path points`);
  assert.ok(Math.hypot(course.path[0][0] - course.tee[0], course.path[0][2] - course.tee[2]) <= 0.3, `${course.id}: path starts at the tee`);
  assert.ok(Math.hypot(course.path.at(-1)[0] - course.cup[0], course.path.at(-1)[2] - course.cup[2]) <= 1.0, `${course.id}: path ends at the cup`);
  assert.ok(Array.isArray(course.corridors) && course.corridors.length >= 1 && course.corridors.every(line => Array.isArray(line) && line.length >= 2 && line.every(vec3)), `${course.id}: corridors`);
  assert.deepEqual(course.corridors[0], course.path, `${course.id}: the first corridor is the main route`);
  // A branch leaves the main route and rejoins it (a fork) or ends on it.
  for (const [index, line] of course.corridors.slice(1).entries()) for (const [label, point] of [['starts', line[0]], ['ends', line.at(-1)]]) {
    assert.ok(pathDistance(course.path, point[0], point[2]) <= 0.5, `${course.id}: corridor ${index + 1} ${label} off the main route`);
  }

  // Open cup: no visible floor triangle covers the hole, except the cup tube itself.
  const floorFaces = [...glb.json.nodes.keys()].map(index => glb.json.nodes[index]).filter(node => node.name?.startsWith('floor-') && node.name !== 'floor-cup')
    .flatMap(node => glb.geometryByNode.get(node.name)).flatMap(mesh => mesh.faces);
  assert.ok(glb.nodeByName.has('floor-cup'), `${course.id}: missing the floor-cup tube`);
  for (const face of floorFaces) {
    const centre = [0, 1, 2].map(axis => (face[0][axis] + face[1][axis] + face[2][axis]) / 3);
    assert.ok(Math.hypot(centre[0] - course.cup[0], centre[2] - course.cup[2]) >= course.cupRadius, `${course.id}: a floor triangle closes the cup at (${centre.map(v => v.toFixed(2))})`);
  }
  // The tee sits one ball radius above the visible floor.
  const heights = floorHeightsAt(floorFaces, course.tee[0], course.tee[2]);
  assert.ok(heights.length > 0, `${course.id}: no floor under the tee`);
  assert.ok(heights.some(height => Math.abs(course.tee[1] - BALL_RADIUS - height) < 0.002), `${course.id}: the tee must lie ${BALL_RADIUS} m above the floor (floor ${heights.map(h => h.toFixed(3))}, tee ${course.tee[1]})`);
  const ring = flatRingFaces(glb, course.tee, 0.10, 0.18, course.tee[1] - BALL_RADIUS + 0.012);
  assert.equal(ring.length, 24, `${course.id}: tee ring triangles`);
  assert.ok(ring.every(face => faceNormalY(face) > 0), `${course.id}: tee ring must face up (single-sided material)`);

  // Templates and the flag, as the runtime reads them.
  for (const name of ['flag', 'ball-template', 'putter-template']) assert.ok(glb.nodeByName.has(name), `${course.id}: missing ${name} node`);
  for (const name of ['ball-template', 'putter-template']) {
    const template = glb.nodeByName.get(name);
    close3(template.node.translation ?? [0, 0, 0], [0, 0, 0], `${course.id}: ${name} local origin`);
    assert.ok(!glb.parents.has(template.index), `${course.id}: ${name} must be a top-level template`);
  }
  const ballVertices = (glb.geometryByNode.get('ball-template') ?? []).flatMap(mesh => mesh.positions);
  assert.ok(ballVertices.length >= 12, `${course.id}: ball template geometry`);
  for (const point of ballVertices) assert.ok(Math.abs(Math.hypot(...point) - BALL_RADIUS) < 0.0002, `${course.id}: ball template radius`);
  const putterY = (glb.geometryByNode.get('putter-template') ?? []).flatMap(mesh => mesh.positions).map(point => point[1]);
  assert.ok(putterY.length >= 12, `${course.id}: putter template geometry`);
  close3([Math.min(...putterY), Math.max(...putterY), 0], [0, 0.82, 0], `${course.id}: putter local shaft extent`);

  // Island and placements.
  const catalog = manifest.kits?.[course.world]?.props;
  assert.ok(catalog, `${course.id}: kit catalog for ${course.world}`);
  assert.ok(course.props.length > 0 && course.props.length <= BUDGET.placements, `${course.id}: ${course.props.length} placements`);
  const island = course.island;
  assert.ok(island && island.center.length === 2 && island.half.length === 2, `${course.id}: island outline`);
  assert.ok([...island.center, ...island.half, island.power, island.wobble].every(Number.isFinite), `${course.id}: island numbers`);
  assert.ok(island.half.every(value => value > 3 && value <= 20) && island.power >= 2 && island.wobble >= 0 && island.wobble < 0.1, `${course.id}: island outline range`);
  for (const [name, x, y, z, yaw, scale] of course.props) {
    const prop = catalog[name];
    assert.ok(prop, `${course.id}: unknown prop ${name}`);
    assert.ok([x, y, z, yaw, scale].every(Number.isFinite) && scale > 0, `${course.id}: ${name} placement numbers`);
    placedProps.get(course.world).add(name);
    const reserve = rimReserve(island, x, z), needed = EDGE + rimShare(name, prop.tags) * prop.radius * scale;
    assert.ok(reserve >= needed - RIM_TOLERANCE, `${course.id}: ${name} at (${x}, ${z}) reaches over the island rim (${reserve.toFixed(2)} m inside, needs ${needed.toFixed(2)} m)`);
    if (!prop.tags.includes('span')) for (const [index, line] of course.corridors.entries()) {
      assert.ok(pathDistance(line, x, z) >= 1.0 + prop.radius * scale - .05, `${course.id}: ${name} blocks corridor ${index}`);
    }
    if (prop.tags.includes('hero')) assert.ok(scale >= 0.9, `${course.id}: hero ${name} scaled down to ${scale}`);
  }

  // Mechanics and obstacles: valid, unique, inside bounds; obstacle nodes start in their time-0 pose; one shared cycle.
  assert.ok(Array.isArray(course.mechanics) && Array.isArray(course.obstacles), `${course.id}: mechanics and obstacles are lists`);
  const used = new Set();
  for (const mechanic of course.mechanics) {
    validateMechanic(mechanic);
    assert.ok(!used.has(mechanic.id), `${course.id}: duplicate id ${mechanic.id}`); used.add(mechanic.id);
    for (const key of ['position', 'exit', 'muzzle']) if (mechanic[key]) assert.ok(inside(course.bounds, mechanic[key]), `${course.id}: ${mechanic.id} ${key} outside bounds`);
  }
  for (const obstacle of course.obstacles) {
    validateObstacle(obstacle);
    assert.ok(!used.has(obstacle.id), `${course.id}: duplicate id ${obstacle.id}`); used.add(obstacle.id);
    assert.equal(obstacle.node, `obstacle-${obstacle.id}`, `${course.id}: obstacle node name`);
    assert.ok(inside(course.bounds, obstacle.position), `${course.id}: ${obstacle.id} outside bounds`);
    const actual = glb.nodeByName.get(obstacle.node);
    assert.ok(actual, `${course.id}: missing obstacle node ${obstacle.node}`);
    assert.ok(!glb.parents.has(actual.index), `${course.id}: ${obstacle.node} must be a top-level node`);
    const pose = obstaclePose(obstacle, 0), rotation = actual.node.rotation ?? [0, 0, 0, 1];
    close3(actual.node.translation ?? [0, 0, 0], pose.position, `${course.id}: ${obstacle.node} position equals obstaclePose(spec, 0)`);
    const sign = Math.sign(rotation.reduce((sum, value, i) => sum + value * pose.rotation[i], 0)) || 1;
    close3(rotation.map(value => value * sign), pose.rotation, `${course.id}: ${obstacle.node} rotation equals obstaclePose(spec, 0)`);
    assert.ok(Number.isFinite(course.cycle) && course.cycle > 0, `${course.id}: a course with obstacles needs a cycle`);
    const turns = course.cycle * obstacle.speed / (2 * Math.PI);
    assert.ok(turns >= 0.999 && Math.abs(turns - Math.round(turns)) < 1e-3, `${course.id}: ${obstacle.id} runs ${turns} periods per cycle (needs a whole number)`);
  }
  if (!course.obstacles.length) assert.equal(course.cycle, null, `${course.id}: no obstacles, no cycle`);

  const result = await validator.validateBytes(bytes, { uri: course.model, maxIssues: 100, externalResourceFunction: () => Promise.reject(Error('external resource forbidden')) });
  assert.equal(result.issues.numErrors, 0, `${course.id}: glTF validation errors`);
  warningCount += result.issues.numWarnings;
  totalBytes += bytes.length;
  totalTriangles += course.triangles;
}
const runtimeBytes = totalBytes + manifestBytes.length;

// One prop kit per world: valid, embedded, one top-level node per prop with vertex colours, within budget.
let kitBytes = 0;
for (const world of WORLDS) {
  const kit = manifest.kits[world];
  const bytes = await readFile(path.join(assetRoot, kit.model));
  assert.equal(kit.model, `props-${world}.glb`);
  assert.equal(bytes.length, kit.bytes, `${world} kit bytes`);
  assert.equal(createHash('sha256').update(bytes).digest('hex'), kit.sha256, `${world} kit SHA-256`);
  assert.ok(bytes.length <= BUDGET.kitBytes, `${world} kit exceeds ${BUDGET.kitBytes / MIB} MiB`);
  const glb = parseGLB(bytes);
  assert.equal(glb.triangleCount, kit.triangles, `${world} kit triangle count`);
  assert.ok(kit.triangles <= BUDGET.kitTriangles, `${world} kit triangle budget`);
  assert.ok(glb.json.materials.length <= 8, `${world} kit uses ${glb.json.materials.length} materials (max 8)`);
  for (const [name, prop] of Object.entries(kit.props)) {
    const node = glb.nodeByName.get(`prop-${world}-${name}`);
    assert.ok(node && !glb.parents.has(node.index), `${world}: prop-${world}-${name} must be a top-level node`);
    const hero = prop.tags.includes('hero');
    const mesh = glb.json.meshes[node.node.mesh];
    assert.ok(mesh.primitives.length <= (hero ? 4 : 3), `${world}/${name}: ${mesh.primitives.length} primitives`);
    for (const primitive of mesh.primitives) assert.ok(primitive.attributes.COLOR_0 !== undefined, `${world}/${name}: COLOR_0`);
    assert.ok(prop.triangles <= (hero ? BUDGET.hero : BUDGET.prop), `${world}/${name}: ${prop.triangles} triangles`);
    assert.deepEqual(node.node.extras?.aurago_prop?.tags, prop.tags, `${world}/${name}: node extras`);
  }
  const result = await validator.validateBytes(bytes, { uri: kit.model, maxIssues: 100, externalResourceFunction: () => Promise.reject(Error('external resource forbidden')) });
  assert.equal(result.issues.numErrors, 0, `${world} kit: glTF validation errors`);
  kitBytes += bytes.length;
}

// Every prop of a world's kit stands in at least one course of that world.
const unplaced = WORLDS.flatMap(world => Object.keys(manifest.kits[world].props).filter(name => !placedProps.get(world).has(name)).map(name => `${world}/${name}`));
if (partial) { if (unplaced.length) console.log(`note: kit props not placed yet (partial run): ${unplaced.join(', ')}`); }
else assert.deepEqual(unplaced, [], 'kit props that no course places');

// The menu shows one rendered preview per world.
const worldIds = manifest.courses.map(course => course.world).filter((world, i, all) => all.indexOf(world) === i);
assert.deepEqual(manifest.worlds, (partial ? worldIds : WORLDS).map(id => ({ id, preview: `previews/${id}.webp` })));
for (const world of manifest.worlds) {
  const image = await readFile(path.join(assetRoot, world.preview)).catch(error => { if (partial && error.code === 'ENOENT') return null; throw error; });
  if (!image) continue;
  assert.equal(image.toString('ascii', 0, 4) + image.toString('ascii', 8, 12), 'RIFFWEBP', `${world.preview}: WebP`);
  assert.ok(image.length <= BUDGET.preview, `${world.preview}: ${image.length} bytes`);
}

async function treeBytes(dir) {
  let sum = 0;
  for (const entry of await readdir(dir, { withFileTypes: true })) sum += entry.isDirectory() ? await treeBytes(path.join(dir, entry.name)) : (await stat(path.join(dir, entry.name))).size;
  return sum;
}
const packageBytes = await treeBytes(assetRoot);
assert.ok(packageBytes <= BUDGET.package, `AuraGolf runtime assets exceed ${BUDGET.package / MIB} MiB: ${packageBytes}`);
console.log(`Mini golf: ${manifest.courses.length} courses${partial ? ' (partial)' : ''}, ${totalTriangles.toLocaleString()} triangles, ${totalBytes.toLocaleString()} GLB bytes + ${manifestBytes.length.toLocaleString()} manifest bytes = ${runtimeBytes.toLocaleString()} course bytes, 3 kits ${kitBytes.toLocaleString()} bytes, ${packageBytes.toLocaleString()} bytes in total, ${warningCount} glTF warnings (${validator.version()})`);
