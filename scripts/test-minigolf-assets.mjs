import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFile, readdir, stat } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import validator from 'gltf-validator';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const assetRoot = path.join(root, 'ui/3d/minigolf');
const manifestBytes = await readFile(path.join(assetRoot, 'manifest.json'));
const manifest = JSON.parse(manifestBytes.toString('utf8'));
const p1Colliders = JSON.parse(await readFile(path.join(root, 'scripts/fixtures/minigolf-p1-colliders.json'), 'utf8'));
assert.equal(manifest.version, 1);
assert.equal(manifest.ballRadius, 0.16);
assert.equal(manifest.courses.length, 18);
assert.deepEqual(['garden', 'coast', 'night'].map(world => manifest.courses.filter(course => course.world === world).length), [6, 6, 6]);
assert.deepEqual(manifest.courses.map(course => course.number), Array.from({ length: 18 }, (_, i) => i + 1));

// P2 dresses the courses as three worlds; the world ids stay as they are until P3.
const LOOK_OF = { coast: 'pirate', garden: 'temple', night: 'moon' };
const KIB = 1024, MIB = 1024 * 1024;
// Largest sizes a course, kit or prop may reach (spec section 6).
const BUDGET = { courseBytes: 768 * KIB, courseTriangles: 24_000, meshNodes: 24, kitBytes: 3 * MIB, kitTriangles: 60_000, prop: 15_000, hero: 20_000, preview: 80 * KIB, package: 32 * MIB };
function pathDistance(points, x, z) {
  let best = Infinity;
  for (let i = 1; i < points.length; i++) {
    const [ax, , az] = points[i - 1], [bx, , bz] = points[i], vx = bx - ax, vz = bz - az;
    const t = Math.max(0, Math.min(1, ((x - ax) * vx + (z - az) * vz) / Math.max(1e-9, vx * vx + vz * vz)));
    best = Math.min(best, Math.hypot(x - ax - t * vx, z - az - t * vz));
  }
  return best;
}

// Manifest collision vertices are read directly from exported float32 GLB
// accessors, so shortest-roundtrip decimal strings compare the same values
// without coarse coordinate rounding or half-cent precision edge cases.
const vertexKey = point => point.map(value => Number(value).toPrecision(17)).join(',');
const triangleKey = points => points.map(vertexKey).sort().join('|');
const close3 = (actual, expected, message) => actual.forEach((value, i) => assert.ok(Math.abs(value - expected[i]) < 0.0001, `${message}[${i}]: ${value} != ${expected[i]}`));
function sampleCenterlineHeight(positions, marker, courseId, markerName) {
  const exact = positions.filter(point => Math.abs(point[0] - marker[0]) < 0.0001 && Math.abs(point[2] - marker[2]) < 0.0001);
  if (exact.length) return exact.reduce((sum, point) => sum + point[1], 0) / exact.length;
  const nearby = positions.filter(point => Math.hypot(point[0] - marker[0], point[2] - marker[2]) <= 0.9);
  let bestDistance = Infinity, samples = [];
  for (let i = 0; i < nearby.length; i++) for (let j = i + 1; j < nearby.length; j++) {
    const a = nearby[i], b = nearby[j];
    if (Math.hypot(a[0] + b[0] - 2 * marker[0], a[2] + b[2] - 2 * marker[2]) > 0.0002) continue;
    const distance = Math.hypot(a[0] - marker[0], a[2] - marker[2]) + Math.hypot(b[0] - marker[0], b[2] - marker[2]);
    if (distance < bestDistance - 0.0001) { bestDistance = distance; samples = [(a[1] + b[1]) / 2]; }
    else if (Math.abs(distance - bestDistance) < 0.0001) samples.push((a[1] + b[1]) / 2);
  }
  assert.ok(samples.length, `${courseId}: ${markerName} must have a symmetric exported fairway sample`);
  return samples.reduce((sum, height) => sum + height, 0) / samples.length;
}

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
      const triangles = [];
      if ((primitive.mode ?? 4) === 4) for (let i = 0; i + 2 < indices.length; i += 3) triangles.push(triangleKey([positions[indices[i]], positions[indices[i + 1]], positions[indices[i + 2]]]));
      return { positions, triangles };
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

function colliderTriangles(collider) {
  const vertices = Array.from({ length: collider.vertices.length / 3 }, (_, i) => collider.vertices.slice(i * 3, i * 3 + 3));
  return Array.from({ length: collider.indices.length / 3 }, (_, i) => {
    const tri = collider.indices.slice(i * 3, i * 3 + 3).map(index => {
      assert.ok(Number.isInteger(index) && index >= 0 && index < vertices.length, `${collider.surface}: collider index outside vertex list`);
      return vertices[index];
    });
    return { key: triangleKey(tri), points: tri };
  });
}

let totalBytes = 0, totalTriangles = 0, warningCount = 0;
for (const course of manifest.courses) {
  assert.match(course.id, /^(garden|coast|night)-\d{2}$/);
  assert.ok(course.par >= 2 && course.par <= 4, course.id);
  assert.equal(course.model, `course-${String(course.number).padStart(2, '0')}.glb`);
  // P2 dresses courses without touching physics: the decoded collider data must
  // stay identical to the P1 state (P3 replaces this fixture with new courses).
  assert.equal(createHash('sha256').update(JSON.stringify(course.colliders)).digest('hex'), p1Colliders[course.id], `${course.id}: collision differs from the P1 fingerprint`);
  const bytes = await readFile(path.join(assetRoot, course.model));
  assert.equal(bytes.length, course.bytes, `${course.id}: byte count`);
  assert.equal(createHash('sha256').update(bytes).digest('hex'), course.sha256, `${course.id}: SHA-256`);
  const glb = parseGLB(bytes);
  assert.equal(glb.triangleCount, course.triangles, `${course.id}: triangle count`);
  assert.ok(course.bytes <= BUDGET.courseBytes, `${course.id}: course GLB exceeds ${BUDGET.courseBytes / KIB} KiB (${course.bytes})`);
  assert.ok(course.triangles <= BUDGET.courseTriangles, `${course.id}: triangle budget (${course.triangles})`);
  const meshNodes = glb.json.nodes.filter(node => node.mesh !== undefined).length;
  assert.ok(meshNodes <= BUDGET.meshNodes, `${course.id}: ${meshNodes} mesh nodes (max ${BUDGET.meshNodes})`);
  assert.equal(course.look, LOOK_OF[course.world], `${course.id}: look`);
  assert.ok(Array.isArray(course.path) && course.path.length >= 17 && course.path.length <= 33, `${course.id}: path samples`);
  assert.ok(Math.hypot(course.path[0][0] - course.tee[0], course.path[0][2] - course.tee[2]) <= .3, `${course.id}: path starts at the tee`);
  // The lane runs a little past the hole (the cup sits at 91 % of it), so the path may end up to a metre behind the cup.
  assert.ok(Math.hypot(course.path.at(-1)[0] - course.cup[0], course.path.at(-1)[2] - course.cup[2]) <= 1.0, `${course.id}: path ends at the cup`);
  for (const name of ['rail-left', 'rail-right']) assert.equal(glb.nodeByName.get(name)?.node.extras?.aurago_hidden, true, `${course.id}: ${name} hidden at runtime`);
  for (const name of ['rail-trim-left', 'rail-trim-right']) assert.ok(glb.nodeByName.has(name), `${course.id}: missing ${name}`);
  const catalog = manifest.kits?.[course.look]?.props;
  assert.ok(catalog, `${course.id}: kit catalog for ${course.look}`);
  assert.ok(course.props.length > 0 && course.props.length <= 160, `${course.id}: ${course.props.length} placements`);
  for (const [name, x, y, z, yaw, scale] of course.props) {
    const prop = catalog[name];
    assert.ok(prop, `${course.id}: unknown prop ${name}`);
    assert.ok([x, y, z, yaw, scale].every(Number.isFinite) && scale > 0, `${course.id}: ${name} placement numbers`);
    assert.ok(Math.hypot(x, z) <= 6.0, `${course.id}: ${name} outside the island`);
    if (!prop.tags.includes('span')) assert.ok(pathDistance(course.path, x, z) >= 1.0 + prop.radius * scale - .05, `${course.id}: ${name} blocks the lane corridor`);
  }
  for (const name of ['flag', 'ball-template', 'putter-template']) assert.ok(glb.nodeByName.has(name), `${course.id}: missing ${name} node`);
  for (const key of ['tee', 'cup']) {
    assert.equal(course[key].length, 3, `${course.id}: ${key}`);
    assert.ok(course[key].every(Number.isFinite), `${course.id}: ${key} must be finite`);
  }
  const laneVertices = ['surface-grass', 'surface-sand'].flatMap(name => glb.geometryByNode.get(name) ?? []).flatMap(mesh => mesh.positions);
  for (const [markerName, marker, heightOffset] of [['tee', course.tee, manifest.ballRadius], ['cup', course.cup, 0]]) {
    const surfaceY = sampleCenterlineHeight(laneVertices, marker, course.id, markerName);
    assert.ok(Math.abs(marker[1] - heightOffset - surfaceY) < 0.0002, `${course.id}: ${markerName} height must match exported fairway + ${heightOffset} m`);
  }
  assert.ok(course.bounds.min.every((v, i) => Number.isFinite(v) && v < course.bounds.max[i]), `${course.id}: bounds`);
  assert.ok(course.water.length > 0, `${course.id}: water volume`);

  for (const name of ['ball-template', 'putter-template']) {
    const template = glb.nodeByName.get(name);
    close3(template.node.translation ?? [0, 0, 0], [0, 0, 0], `${course.id}: ${name} local origin`);
    assert.ok(!glb.parents.has(template.index), `${course.id}: ${name} must be a top-level template`);
  }
  const ballVertices = (glb.geometryByNode.get('ball-template') ?? []).flatMap(mesh => mesh.positions);
  assert.ok(ballVertices.length >= 12, `${course.id}: ball template geometry`);
  for (const point of ballVertices) assert.ok(Math.abs(Math.hypot(...point) - manifest.ballRadius) < 0.0002, `${course.id}: ball template radius`);
  const putterVertices = (glb.geometryByNode.get('putter-template') ?? []).flatMap(mesh => mesh.positions);
  assert.ok(putterVertices.length >= 12, `${course.id}: putter template geometry`);
  const putterY = putterVertices.map(point => point[1]);
  close3([Math.min(...putterY), Math.max(...putterY), 0], [0, 0.82, 0], `${course.id}: putter local shaft extent`);

  for (const obstacle of course.obstacles) {
    const actual = glb.nodeByName.get(obstacle.node);
    assert.ok(actual, `${course.id}: missing obstacle node ${obstacle.node}`);
    assert.ok(!glb.parents.has(actual.index), `${course.id}: obstacle node must be top-level`);
    close3(actual.node.translation ?? [0, 0, 0], obstacle.position, `${course.id}: obstacle transform ${obstacle.node}`);
    assert.ok(['rotate', 'slide'].includes(obstacle.kind));
    assert.ok(obstacle.amplitude > 0 && obstacle.speed > 0);
    assert.ok(Array.isArray(obstacle.colliders) && obstacle.colliders.length > 0, `${course.id}: ${obstacle.node} compound collider list`);
    for (const part of obstacle.colliders) {
      assert.equal(part.position.length, 3, `${course.id}: ${obstacle.node} collider position`);
      assert.equal(part.halfExtents.length, 3, `${course.id}: ${obstacle.node} collider half-extents`);
      assert.ok(part.halfExtents.every(value => Number.isFinite(value) && value > 0), `${course.id}: ${obstacle.node} collider dimensions`);
      if (part.rotation) assert.equal(part.rotation.length, 4, `${course.id}: ${obstacle.node} collider quaternion`);
    }
    const subtree = [actual.index];
    for (let i = 0; i < subtree.length; i++) subtree.push(...(glb.json.nodes[subtree[i]].children ?? []));
    const componentNodes = subtree.map(index => glb.json.nodes[index]).filter(node => node.mesh !== undefined);
    assert.equal(componentNodes.length, obstacle.colliders.length, `${course.id}: ${obstacle.node} visual and compound-part count`);
    const unusedParts = new Set(obstacle.colliders.keys());
    for (const component of componentNodes) {
      const points = (glb.geometryByNode.get(component.name) ?? []).flatMap(mesh => mesh.positions);
      assert.ok(points.length, `${course.id}: ${obstacle.node} component geometry`);
      const extents = [0, 1, 2].map(axis => Math.max(...points.map(point => Math.abs(point[axis] - obstacle.position[axis]))));
      const partIndex = [...unusedParts].find(index => obstacle.colliders[index].halfExtents.every((value, axis) => Math.abs(value - extents[axis]) < 0.0001));
      assert.notEqual(partIndex, undefined, `${course.id}: ${obstacle.node} collider dimensions match exported geometry`);
      unusedParts.delete(partIndex);
    }
    assert.equal(unusedParts.size, 0, `${course.id}: all ${obstacle.node} collider parts represented`);
  }

  // Every declared fixed-collision triangle must also exist on the matching
  // exported visual mesh. Moving obstacles have their own nodes and no fixed
  // collider entry. The cup opening has no collision floor.
  const expected = new Map();
  for (const collider of course.colliders) {
    assert.equal(collider.type, 'mesh');
    assert.equal(collider.vertices.length % 3, 0);
    assert.equal(collider.indices.length % 3, 0);
    const list = expected.get(collider.surface) ?? [];
    list.push(...colliderTriangles(collider));
    expected.set(collider.surface, list);
  }
  for (const [surface, triangles] of expected) {
    const names = surface === 'wood' ? ['rail-left', 'rail-right'] : surface === 'stone' ? ['surface-stone', 'rail-left', 'rail-right'] : [`surface-${surface}`];
    const actual = names.flatMap(name => glb.geometryByNode.get(name) ?? []).flatMap(mesh => mesh.triangles);
    assert.ok(actual.length, `${course.id}: no visual geometry for ${surface} collider`);
    assert.equal(actual.length, triangles.length, `${course.id}: ${surface} collider and visual triangle counts`);
    const actualCounts = new Map();
    for (const triangle of actual) actualCounts.set(triangle, (actualCounts.get(triangle) ?? 0) + 1);
    for (const triangle of triangles) {
      const count = actualCounts.get(triangle.key) ?? 0;
      assert.ok(count > 0, `${course.id}: ${surface} collider triangle ${triangle.key} differs from its visible mesh`);
      actualCounts.set(triangle.key, count - 1);
    }
    assert.ok([...actualCounts.values()].every(count => count === 0), `${course.id}: unmatched visible ${surface} triangles`);
  }
  for (const collider of course.colliders) for (const triangle of colliderTriangles(collider)) {
    const center = triangle.points[0].map((v, axis) => (v + triangle.points[1][axis] + triangle.points[2][axis]) / 3);
    const horizontal = Math.hypot(center[0] - course.cup[0], center[2] - course.cup[2]);
    if (center[1] > course.cup[1] - .12 && center[1] < course.cup[1] + .24) assert.ok(horizontal >= course.cupRadius * .70, `${course.id}: solid collision triangle closes cup`);
  }

  const result = await validator.validateBytes(bytes, { uri: course.model, maxIssues: 100, externalResourceFunction: () => Promise.reject(Error('external resource forbidden')) });
  assert.equal(result.issues.numErrors, 0, `${course.id}: glTF validation errors`);
  warningCount += result.issues.numWarnings;
  totalBytes += bytes.length;
  totalTriangles += course.triangles;
}
const runtimeBytes = totalBytes + manifestBytes.length;

// One prop kit per world: valid, embedded, one top-level node per prop with vertex colours, within budget.
let kitBytes = 0;
for (const look of ['pirate', 'temple', 'moon']) {
  const kit = manifest.kits[look];
  const bytes = await readFile(path.join(assetRoot, kit.model));
  assert.equal(kit.model, `props-${look}.glb`);
  assert.equal(bytes.length, kit.bytes, `${look} kit bytes`);
  assert.equal(createHash('sha256').update(bytes).digest('hex'), kit.sha256, `${look} kit SHA-256`);
  assert.ok(bytes.length <= BUDGET.kitBytes, `${look} kit exceeds ${BUDGET.kitBytes / MIB} MiB`);
  const glb = parseGLB(bytes);
  assert.equal(glb.triangleCount, kit.triangles, `${look} kit triangle count`);
  assert.ok(kit.triangles <= BUDGET.kitTriangles, `${look} kit triangle budget`);
  assert.ok(glb.json.materials.length <= 8, `${look} kit uses ${glb.json.materials.length} materials (max 8)`);
  for (const [name, prop] of Object.entries(kit.props)) {
    const node = glb.nodeByName.get(`prop-${look}-${name}`);
    assert.ok(node && !glb.parents.has(node.index), `${look}: prop-${look}-${name} must be a top-level node`);
    const hero = prop.tags.includes('hero');
    const mesh = glb.json.meshes[node.node.mesh];
    assert.ok(mesh.primitives.length <= (hero ? 4 : 3), `${look}/${name}: ${mesh.primitives.length} primitives`);
    for (const primitive of mesh.primitives) assert.ok(primitive.attributes.COLOR_0 !== undefined, `${look}/${name}: COLOR_0`);
    assert.ok(prop.triangles <= (hero ? BUDGET.hero : BUDGET.prop), `${look}/${name}: ${prop.triangles} triangles`);
    assert.deepEqual(node.node.extras?.aurago_prop?.tags, prop.tags, `${look}/${name}: node extras`);
  }
  const result = await validator.validateBytes(bytes, { uri: kit.model, maxIssues: 100, externalResourceFunction: () => Promise.reject(Error('external resource forbidden')) });
  assert.equal(result.issues.numErrors, 0, `${look} kit: glTF validation errors`);
  kitBytes += bytes.length;
}

async function treeBytes(dir) {
  let sum = 0;
  for (const entry of await readdir(dir, { withFileTypes: true })) sum += entry.isDirectory() ? await treeBytes(path.join(dir, entry.name)) : (await stat(path.join(dir, entry.name))).size;
  return sum;
}
const packageBytes = await treeBytes(assetRoot);
assert.ok(packageBytes <= BUDGET.package, `AuraGolf runtime assets exceed ${BUDGET.package / MIB} MiB: ${packageBytes}`);
console.log(`Mini golf: 18 courses, ${totalTriangles.toLocaleString()} triangles, ${totalBytes.toLocaleString()} GLB bytes + ${manifestBytes.length.toLocaleString()} manifest bytes = ${runtimeBytes.toLocaleString()} course bytes, 3 kits ${kitBytes.toLocaleString()} bytes, ${packageBytes.toLocaleString()} bytes in total, ${warningCount} glTF warnings (${validator.version()})`);
