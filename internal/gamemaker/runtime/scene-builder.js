// The canonical scene contract is deliberately small.  This adapter keeps scene
// data separate from the gameplay source so a local scene regeneration does not
// overwrite custom code.  It accepts the current names and the old objects/at
// shape for projects created before scene.json existed.

const MAX_SCENE_ITEMS = 4096;
const MAX_AUDIT_NODES = 128;
const MAX_AUDIT_ROLES = 64;
const MAX_AUDIT_FAULTS = 16;
const MAX_EVENT_TRACE = 64;
const MAX_STATE_KEYS = 64;
const KNOWN_COLLIDERS = new Set(['box', 'circle', 'capsule', 'mesh']);
const KNOWN_MECHANICS = new Set([
  'movement', 'camera', 'health', 'collect', 'destroy', 'reach', 'survive',
  'checkpoint', 'patrol', 'chase', 'keepdistance', 'damage', 'projectile',
  'waves', 'wave', 'inventory', 'dialogue', 'unlock', 'trigger',
]);
const finite = (value, fallback = 0) => Number.isFinite(Number(value)) ? Number(value) : fallback;
const clamp = (value, min, max) => Math.max(min, Math.min(max, value));
const array = (value) => Array.isArray(value) ? value : [];
const object = (value) => value && typeof value === 'object' && !Array.isArray(value) ? value : {};

function parseJSON(value, fallback = {}) {
  if (value && typeof value === 'object') return object(value);
  if (typeof value !== 'string' || !value.trim()) return fallback;
  try {
    const parsed = JSON.parse(value);
    return object(parsed);
  } catch {
    return fallback;
  }
}

function normalizeScale(value, dimension) {
  const defaults = dimension === '3d' ? [1, 1, 1] : [1, 1];
  const v = Array.isArray(value) ? value : object(value);
  const values = dimension === '3d'
    ? [v[0] ?? v.x, v[1] ?? v.y, v[2] ?? v.z]
    : [v[0] ?? v.x, v[1] ?? v.y];
  return values.map((item, index) => {
    const n = Number(item);
    // Fixed Vec3 fields encode an omitted scale as [0,0,0].
    return Number.isFinite(n) && Math.abs(n) > 1e-9 ? n : defaults[index];
  });
}

function boundsFromValue(raw, dimension, fallback = null) {
  const source = object(raw?.bounds || raw?.world_bounds || raw?.worldBounds || raw);
  const hasMin = Array.isArray(source.min) || (source.min && typeof source.min === 'object');
  const hasMax = Array.isArray(source.max) || (source.max && typeof source.max === 'object');
  if (!hasMin || !hasMax) return fallback;
  return { min: position(source.min, dimension), max: position(source.max, dimension), explicit: true };
}

function normalizeCollider(raw, dimension) {
  const source = object(raw);
  const extents = source.extents || source.size || [1, 1, dimension === '3d' ? 1 : 0];
  return {
    ...source,
    id: String(source.id || '').trim(),
    node_id: String(source.node_id || source.nodeId || '').trim(),
    shape: String(source.shape || 'box').toLowerCase(),
    extents: position(extents, dimension),
    offset: position(source.offset, dimension),
    sensor: source.sensor === true,
  };
}

function normalizeZone(raw, index, dimension) {
  const source = object(raw);
  return {
    ...source,
    id: String(source.id || 'zone-' + (index + 1)).trim() || 'zone-' + (index + 1),
    kind: String(source.kind || '').trim().toLowerCase(),
    bounds: boundsFromValue(source, dimension, {
      min: position(undefined, dimension),
      max: position(undefined, dimension),
      explicit: false,
    }),
    level_id: String(source.level_id || source.levelId || '').trim(),
    region_id: String(source.region_id || source.regionId || '').trim(),
    properties: object(source.properties),
    active: source.active !== false,
  };
}

function normalizeRoute(raw, index, dimension) {
  const source = object(raw);
  return {
    ...source,
    id: String(source.id || 'route-' + (index + 1)).trim() || 'route-' + (index + 1),
    from: String(source.from || '').trim(),
    to: String(source.to || '').trim(),
    waypoints: array(source.waypoints || source.points || source.route).map(item => position(item, dimension)),
  };
}

function position(value, dimension) {
  const v = Array.isArray(value) ? value : object(value);
  if (Array.isArray(v)) {
    return dimension === '3d'
      ? [finite(v[0]), finite(v[1]), finite(v[2])]
      : [finite(v[0]), finite(v[1])];
  }
  return dimension === '3d'
    ? [finite(v.x ?? v[0]), finite(v.y ?? v[1]), finite(v.z ?? v[2])]
    : [finite(v.x ?? v[0]), finite(v.y ?? v[1])];
}

function bounds(raw, dimension, viewport) {
  const source = object(raw.world_bounds || raw.worldBounds || raw.world || raw.bounds);
  const min = position(source.min || source.minimum, dimension);
  const max = position(source.max || source.maximum, dimension);
  const hasMinMax = (Array.isArray(source.min) || source.min && source.min.x !== undefined) &&
    (Array.isArray(source.max) || source.max && source.max.x !== undefined);
  if (hasMinMax) return { min, max, explicit: true };
  if (dimension === '3d') {
    return { min: [-50, 0, -20], max: [50, 30, 180], explicit: false };
  }
  return {
    min: [0, 0],
    max: [finite(viewport?.width, 960), finite(viewport?.height, 540)],
    explicit: false,
  };
}

function nodePosition(raw, dimension) {
  const transform = object(raw.transform);
  return position(raw.at || raw.position || transform.position || raw.origin, dimension);
}

function nodeSize(raw, dimension) {
  const size = raw.size || raw.dimensions || raw.extents || raw.collider?.size || raw.collider?.extents;
  const positive = (value, fallback, minimum) => {
    const number = Number(value);
    return Number.isFinite(number) && number > 0 ? Math.max(minimum, number) : fallback;
  };
  if (Array.isArray(size)) return dimension === '3d'
    ? [positive(size[0], 1, .01), positive(size[1], 1, .01), positive(size[2], 1, .01)]
    : [positive(size[0], 32, 1), positive(size[1], 32, 1)];
  const s = object(size);
  return dimension === '3d'
    ? [positive(s.x ?? s.width, 1, .01), positive(s.y ?? s.height, 1, .01), positive(s.z ?? s.depth, 1, .01)]
    : [positive(s.x ?? s.width, 32, 1), positive(s.y ?? s.height, 32, 1)];
}

function behaviorList(raw) {
  const source = raw.behaviors ?? raw.behaviour ?? raw.behavior ?? raw.rules ?? [];
  if (typeof source === 'string') return [{ type: source }];
  if (Array.isArray(source)) return source.map(item => typeof item === 'string' ? { type: item } : object(item));
  if (source && typeof source === 'object' && (source.type || source.kind)) return [source];
  if (source && typeof source === 'object') {
    return Object.entries(source).flatMap(([type, value]) => {
      if (value === false || value == null) return [];
      return [{ type, ...(value === true ? {} : object(value)) }];
    });
  }  return [];
}

function behaviorType(item) {
  return String(item?.type || item?.kind || '').trim().toLowerCase();
}

function mergeBehaviors(...sources) {
  const result = [];
  for (const source of sources) {
    for (const item of array(source)) {
      const candidate = typeof item === 'string' ? { type: item } : object(item);
      const type = behaviorType(candidate);
      if (!type) continue;
      const existing = result.find(value => behaviorType(value) === type);
      if (existing) Object.assign(existing, candidate);
      else result.push({ ...candidate, type });
    }
  }
  return result;
}

function normalizePlacement(raw, logical, dimension, index) {
  const source = object(raw);
  const logicalSource = object(logical);
  const id = String(source.id || 'placement-' + (index + 1)).trim() || 'placement-' + (index + 1);
  const nodeID = String(source.node_id || source.nodeId || logicalSource.id || '').trim();
  const transform = object(source.transform);
  return {
    ...source,
    id,
    node_id: nodeID,
    asset_id: String(source.asset_id || source.assetId || '').trim(),
    asset_role: String(source.asset_role || source.role || '').trim(),
    behavior: source.behavior ?? '',
    position: position(source.position ?? source.at ?? logicalSource.position ?? logicalSource.at, dimension),
    rotation: transform.rotation ?? source.rotation,
    scale: normalizeScale(transform.scale ?? source.scale, dimension),
    level_id: String(source.level_id || source.levelId || logicalSource.level_id || '').trim(),
    region_id: String(source.region_id || source.regionId || logicalSource.region_id || '').trim(),
  };
}

function normalizeNode(raw, index, dimension, used) {
  const source = object(raw);
  const placementRows = array(source.__placements);
  const placementBehaviors = array(source.__placementBehaviors);
  const asset = object(source.asset || source.visual);
  const firstPlacement = object(placementRows[0]);
  const baseID = String(source.id || source.key || source.name || firstPlacement.node_id || 'node-' + (index + 1)).trim() || 'node-' + (index + 1);
  let id = baseID;
  let suffix = 2;
  while (used.has(id)) id = baseID + '-' + suffix++;
  used.add(id);
  const transform = object(source.transform);
  const role = String(source.role || source.asset_role || firstPlacement.asset_role || asset.role || '').trim();
  const assetID = String(source.asset_id || source.assetId || firstPlacement.asset_id || asset.id || asset.asset_id || '').trim();
  const collider = source.collider === false || !source.collider ? null : normalizeCollider(source.collider, dimension);
  const node = {
    ...source,
    id,
    role,
    asset_role: role,
    asset_id: assetID,
    asset_roles: placementRows.map(item => String(item.asset_role || '').trim()).filter(Boolean),
    at: nodePosition(source, dimension),
    rotation: transform.rotation ?? source.rotation,
    scale: normalizeScale(transform.scale ?? source.scale, dimension),
    size: nodeSize(source, dimension),
    collider,
    behaviors: mergeBehaviors(behaviorList(source), placementBehaviors),
    placements: placementRows,
    tags: array(source.tags).map(String),
    active: source.active !== false,
    properties: object(source.properties),
  };
  node.required_role = String(source.required_role || source.requiredRole || asset.required_role || '').trim();
  node.material = String(source.material || collider?.material || '').trim();
  delete node.__placements;
  delete node.__placementBehaviors;
  return node;
}
export function normalizeScene(raw, dimension = '2d', viewport = {}) {
  const source = object(raw);
  const used = new Set();
  const levels = array(source.levels);
  const activeLevels = new Set(levels.filter(level => level.active !== false).map(level => String(level.id || '')));
  const colliderByNode = new Map();
  const allColliders = array(source.colliders);
  const colliders = allColliders.slice(0, MAX_SCENE_ITEMS).map(item => normalizeCollider(item, dimension));
  for (const collider of colliders) {
    if (!collider.node_id) continue;
    if (!colliderByNode.has(collider.node_id)) colliderByNode.set(collider.node_id, []);
    colliderByNode.get(collider.node_id).push(collider);
  }
  const logicalNodes = array(source.nodes || source.objects || source.entities);
  const allPlacements = array(source.placements);
  const placements = allPlacements.slice(0, MAX_SCENE_ITEMS);
  const byID = new Map(logicalNodes.map(node => [String(node.id || ''), node]));
  const placementGroups = new Map();
  const placementOrder = [];
  for (const placement of placements) {
    const nodeID = String(placement.node_id || placement.nodeId || '').trim();
    if (!placementGroups.has(nodeID)) {
      placementGroups.set(nodeID, []);
      placementOrder.push(nodeID);
    }
    placementGroups.get(nodeID).push(placement);
  }

  const sourceNodes = [];
  const placedIDs = new Set();
  for (const nodeID of placementOrder) {
    const group = placementGroups.get(nodeID) || [];
    const logical = object(byID.get(nodeID));
    const first = object(group[0]);
    const merged = { ...logical };
    merged.id = logical.id || nodeID || first.id || 'placement-' + (sourceNodes.length + 1);
    merged.position = logical.position ?? logical.at ?? first.position ?? first.at;
    merged.role = logical.role || logical.asset_role || first.asset_role;
    merged.asset_role = logical.asset_role || first.asset_role;
    merged.asset_id = logical.asset_id || logical.assetId || first.asset_id || first.assetId;
    merged.properties = { ...object(logical.properties), ...object(first.properties) };
    merged.colliders = colliderByNode.get(nodeID) || [];
    merged.collider = merged.colliders[0] || logical.collider;
    merged.__placements = group.map((item, index) => normalizePlacement(item, logical, dimension, index));
    merged.__placementBehaviors = group.flatMap(item => behaviorList({ behavior: item.behavior }));
    sourceNodes.push(merged);
    if (logical.id) placedIDs.add(String(logical.id));
  }
  for (const logical of logicalNodes) {
    if (!placedIDs.has(String(logical.id || ''))) sourceNodes.push(logical);
  }

  const activeSourceNodes = sourceNodes.filter(node => {
    const levelID = String(node.level_id || node.levelId || '').trim();
    return !activeLevels.size || !levelID || activeLevels.has(levelID);
  });
  const nodes = activeSourceNodes.slice(0, MAX_SCENE_ITEMS).map((item, index) => normalizeNode({
    ...object(item),
    colliders: array(item.colliders).length ? item.colliders : colliderByNode.get(String(item.node_id || item.id || '')) || [],
    collider: (object(item).collider && Object.keys(object(item).collider).length ? item.collider : null) ||
      array(item.colliders)[0] || colliderByNode.get(String(item.node_id || item.id || ''))?.[0] || null,
  }, index, dimension, used));
  const routeSource = array(source.routes).length ? array(source.routes) : Object.values(object(source.routes));
  const routes = Object.fromEntries(routeSource.slice(0, MAX_SCENE_ITEMS).map((route, index) => {
    const normalized = normalizeRoute(route, index, dimension);
    return [normalized.id, normalized];
  }));
  const zones = array(source.zones).slice(0, MAX_SCENE_ITEMS).map((zone, index) => normalizeZone(zone, index, dimension)).filter(zone => {
    return zone.active !== false && (!activeLevels.size || !zone.level_id || activeLevels.has(zone.level_id));
  });
  const metadata = object(source.metadata);
  const mechanics = object(source.mechanics || metadata.mechanics);

  return {
    version: finite(source.schema_version ?? source.version, 1),
    dimension,
    id: String(source.id || source.name || 'main'),
    seed: finite(source.seed),
    world_bounds: bounds(source, dimension, viewport),
    camera_bounds: source.camera_bounds || source.cameraBounds
      ? bounds({ world_bounds: source.camera_bounds || source.cameraBounds }, dimension, viewport)
      : null,
    nodes,
    colliders,
    zones,
    routes,
    regions: array(source.regions).slice(0, MAX_SCENE_ITEMS),
    attachments: array(source.attachments).slice(0, MAX_SCENE_ITEMS),
    goals: array(source.goals),
    rules: object(source.rules),
    mechanics,
    metadata,
    debug: object(source.debug),
    truncated: activeSourceNodes.length > MAX_SCENE_ITEMS ||
      allPlacements.length > MAX_SCENE_ITEMS ||
      colliders.length < array(source.colliders).length ||
      routeSource.length > MAX_SCENE_ITEMS ||
      array(source.regions).length > MAX_SCENE_ITEMS ||
      array(source.attachments).length > MAX_SCENE_ITEMS ||
      array(source.zones).length > MAX_SCENE_ITEMS,
  };
}
function behavior(node, type) {
  return array(node?.behaviors).find(item => behaviorType(item) === type);
}

function behaviorConfig(node, type) {
  const item = behavior(node, type);
  if (!item) return null;
  const properties = object(node.properties);
  const configured = object(properties[type] || properties[type === 'wave' ? 'waves' : type]);
  return { ...configured, ...item, type };
}

function actualPosition(record, dimension) {
  if (!record) return [0, 0, 0];
  let p = record.object || record.position || record;
  if (p?.object) p = p.object;
  if (dimension === '3d' && p?.position) p = p.position;
  if (Array.isArray(p)) return position(p, dimension).concat(dimension === '2d' ? [0] : []);
  return dimension === '3d'
    ? [finite(p?.x), finite(p?.y), finite(p?.z)]
    : [finite(p?.x), finite(p?.y), 0];
}

function distance(a, b, dimension) {
  const pa = actualPosition(a, dimension);
  const pb = actualPosition(b, dimension);
  return Math.hypot(...pa.map((value, index) => value - pb[index]));
}

function segmentDistance(point, start, end) {
  const delta = end.map((value, index) => value - start[index]);
  const lengthSquared = delta.reduce((sum, value) => sum + value * value, 0);
  const t = lengthSquared > 1e-9
    ? clamp(point.reduce((sum, value, index) => sum + (value - start[index]) * delta[index], 0) / lengthSquared, 0, 1)
    : 0;
  return Math.hypot(...point.map((value, index) => value - (start[index] + delta[index] * t)));
}

function pointInsideBounds(point, boundsValue, dimension) {
  if (!boundsValue || !Array.isArray(boundsValue.min) || !Array.isArray(boundsValue.max)) return false;
  const p = position(point, dimension);
  return p.every((value, index) => value >= boundsValue.min[index] && value <= boundsValue.max[index]);
}

function readHashChannel() {
  const locationValue = globalThis.location || globalThis.window?.location;
  const hash = String(locationValue?.hash || '');
  if (!hash) return '';
  const query = hash.charAt(0) === '#' ? hash.slice(1) : hash;
  try {
    const params = new URLSearchParams(query);
    const value = params.get('gm-channel') || params.get('channel') || '';
    if (value) return String(value).trim();
  } catch {
    // Fall through to the bounded parser below.
  }
  const match = query.match(/(?:^|[&#])(?:gm-channel|channel)=([^&#]+)/i);
  if (!match) return '';
  try {
    return decodeURIComponent(match[1]).trim();
  } catch {
    return String(match[1]).trim();
  }
}

function readChannel() {
  return readHashChannel() ||
    String(globalThis.__AURAGO_STUDIO_CHANNEL__ || globalThis.__AURAGO_GAME_TEST__?.channel || '').trim();
}

function parseMechanics(raw) {
  if (!raw) return {};
  if (typeof raw === 'string') return parseJSON(raw, {});
  return object(raw);
}

function addBehavior(node, candidate) {
  const type = behaviorType(candidate);
  if (!type) return;
  const existing = node.behaviors.find(item => behaviorType(item) === type);
  if (existing) Object.assign(existing, candidate, { type });
  else node.behaviors.push({ ...candidate, type });
}

function mechanicBehavior(block, params) {
  const kind = String(block.kind || '').trim().toLowerCase();
  const config = { ...params };
  delete config.target;
  delete config.node_id;
  delete config.nodeId;
  delete config.role;
  delete config.enabled;
  let type = kind;
  if (kind === 'movement') {
    const mode = String(config.mode || config.behavior || config.movement || '').trim().toLowerCase();
    delete config.behavior;
    delete config.movement;
    delete config.type;
    if (mode) config.mode = mode;
    type = 'movement';
  }
  if (!type) return null;
  if (block.value !== undefined && block.value !== null) {
    const value = block.value;
    if (type === 'health') config.health = finite(value, 1);
    else if (type === 'damage' || type === 'inventory') config.amount = finite(value, 1);
    else if (type === 'survive') config.duration = finite(value, 0);
    else if (type === 'dialogue') config.text = String(value);
    else if (type === 'unlock') config.key = String(value);
    else if (type === 'collect') config.points = finite(value, 0);
    else if (type === 'destroy' || type === 'reach') config.win_when_cleared = value === true;
  }
  return { ...config, type };
}

function applyMechanics(scene, raw, report) {
  const mechanics = parseMechanics(raw);
  scene.mechanics = { ...object(scene.mechanics), ...mechanics };
  for (const block of array(mechanics.blocks)) {
    const source = object(block);
    if (source.enabled === false) continue;
    const kind = String(source.kind || '').trim().toLowerCase();
    if (!KNOWN_MECHANICS.has(kind)) {
      report('mechanic_unknown:' + (kind || 'empty'));
      continue;
    }
    const params = typeof source.params === 'string' ? parseJSON(source.params, null) : object(source.params);
    if (params === null) {
      report('mechanic_params_invalid:' + kind);
      continue;
    }
    if (kind === 'camera') {
      scene.mechanics.camera = [...array(scene.mechanics.camera), { ...source, ...params }];
      continue;
    }
    const targetID = String(source.target || source.node_id || source.nodeId || params.target || '').trim();
    const role = String(source.role || params.role || '').trim();
    const targets = targetID
      ? scene.nodes.filter(node => node.id === targetID)
      : role
        ? scene.nodes.filter(node => node.role === role || array(node.asset_roles).includes(role))
        : [];
    if (!targets.length) {
      report('mechanic_target_missing:' + (targetID || role || kind));
      continue;
    }
    const candidate = mechanicBehavior(source, params);
    if (!candidate || !KNOWN_MECHANICS.has(behaviorType(candidate))) {
      report('mechanic_behavior_missing:' + kind);
      continue;
    }
    if (kind === 'projectile') continue;
    for (const node of targets) addBehavior(node, candidate);
  }
}

function normalizeEventBindings(raw) {
  const bindings = new Map();
  for (const item of array(raw)) {
    const source = object(item);
    const event = String(source.event || '').trim().toLowerCase();
    if (!event) continue;
    const row = {};
    if (source.effect !== undefined) row.effect = source.effect;
    if (source.sound !== undefined) row.sound = source.sound;
    if (!bindings.has(event)) bindings.set(event, []);
    if (Object.keys(row).length) bindings.get(event).push(row);
  }
  return bindings;
}

function isGoalNode(node) {
  return node.active !== false && node.behaviors.some(item =>
    ['collect', 'reach', 'destroy', 'goal'].includes(behaviorType(item)));
}

function colliderExtents(node, dimension) {
  const collider = object(node?.collider);
  const raw = collider.extents || node?.size;
  const values = position(raw, dimension);
  if (collider.extents) return values.map(value => Math.max(.001, Math.abs(value)));
  return values.map(value => Math.max(.001, Math.abs(value) / 2));
}

function solidNode(node) {
  return node?.active !== false && node?.properties?.solid !== false &&
    node?.collider && node.collider.sensor !== true &&
    node.kind !== 'player' && node.properties?.player !== true;
}

function overlapAABB(node, point, delta, dimension) {
  const extents = colliderExtents(node, dimension);
  const offset = position(node.collider?.offset, dimension);
  const center = node.at.map((value, index) => value + offset[index]);
  const target = point.map((value, index) => value + delta[index]);
  return target.every((value, index) => Math.abs(value - center[index]) <= extents[index]);
}
function withinWorldBounds(node, record, delta, dimension, world) {
  if (!world?.explicit || node?.properties?.allow_overhang === true || node?.properties?.decorative === true) return true;
  const current = actualPosition(record, dimension);
  const target = current.map((value, index) => value + finite(delta[index]));
  const extents = colliderExtents(node, dimension);
  const count = dimension === '3d' ? 3 : 2;
  for (let index = 0; index < count; index++) {
    if (target[index] - extents[index] < world.min[index] ||
      target[index] + extents[index] > world.max[index]) return false;
  }
  return true;
}
function makeController(scene, host, dimension) {
  const records = new Map();
  const events = Object.create(null);
  const faults = [];
  const eventTrace = [];
  const zoneStates = new Map();
  const actionNext = new Map();
  const usedNodeIDs = new Set(scene.nodes.map(node => String(node.id)));
  const mechanicsSource = host.mechanics || scene.mechanics;
  const eventBindings = normalizeEventBindings(parseMechanics(mechanicsSource).events);
  let debugListener = null;
  let elapsed = 0;
  let debugEnabled = false;
  let debugOverlay = null;
  let disposed = false;
  const state = host.state || {};

  function fault(code) {
    const value = String(code).slice(0, 160);
    if (faults.length < MAX_AUDIT_FAULTS && !faults.includes(value)) faults.push(value);
  }

  applyMechanics(scene, mechanicsSource, fault);
  for (const [event, bindings] of eventBindings) {
    if (typeof host.presentation?.bindEvent !== 'function') {
      fault('event_binding_missing:' + event);
      continue;
    }
    for (const binding of bindings.slice(0, 16)) {
      const accepted = host.presentation.bindEvent(event, binding);
      if (accepted === false) fault('event_binding_rejected:' + event);
    }
  }
  for (const node of scene.nodes) {
    if (!String(node.id || '').trim()) fault('node_id_missing');
    if (!Array.isArray(node.at) || node.at.some(value => !Number.isFinite(value))) fault('node_position_invalid:' + node.id);
    if (!Array.isArray(node.size) || node.size.some(value => !Number.isFinite(value) || value <= 0)) fault('node_size_invalid:' + node.id);
    if (node.collider) {
      if (!KNOWN_COLLIDERS.has(node.collider.shape)) fault('collider_shape_invalid:' + node.id);
      if (node.collider.extents.some(value => !Number.isFinite(value) || value <= 0)) fault('collider_extents_invalid:' + node.id);
    }
  }
  if (scene.truncated) fault('scene_items_truncated');
  for (const route of Object.values(scene.routes)) {
    if (!route.from || !route.to || route.waypoints.length < 2) fault('route_invalid:' + route.id);
    if (route.from && !usedNodeIDs.has(route.from)) fault('route_from_missing:' + route.id);
    if (route.to && !usedNodeIDs.has(route.to)) fault('route_to_missing:' + route.id);
  }
  for (const item of scene.attachments) {
    if (!item.id || !item.node_id || !item.socket) fault('attachment_invalid:' + (item.id || 'unknown'));
    if (item.node_id && !usedNodeIDs.has(item.node_id)) fault('attachment_node_missing:' + item.id);
  }
  for (const zone of scene.zones) {
    if (!['spawn', 'goal', 'loss', 'trigger'].includes(zone.kind)) fault('zone_kind_invalid:' + zone.id);
    if (!zone.bounds?.explicit) fault('zone_bounds_invalid:' + zone.id);
    zoneStates.set(zone.id, { inside: false, next: 0 });
  }

  const mechanics = parseMechanics(mechanicsSource);
  const playerNode = scene.nodes.find(node => node.kind === 'player' || node.properties?.player === true);
  const playerHealth = playerNode ? behaviorConfig(playerNode, 'health') : null;
  const configuredLives = finite(mechanics.lives ?? scene.rules.lives ?? scene.lives, NaN);
  const configuredHealth = finite(mechanics.health ?? scene.rules.health ?? scene.health ?? playerHealth?.health, NaN);
  if (Number.isFinite(configuredLives)) state.lives = configuredLives;
  if (Number.isFinite(configuredHealth)) state.health = configuredHealth;
  const initialGoalRemaining = scene.nodes.filter(isGoalNode).length;
  if (!Number.isFinite(state.goal_remaining) || state.goal_remaining === 0) state.goal_remaining = initialGoalRemaining;
  if (!Number.isFinite(state.outcome)) state.outcome = 0;
  state.hit_events = finite(state.hit_events);
  state.pickup_events = finite(state.pickup_events);
  state.win_events = finite(state.win_events);
  state.lose_events = finite(state.lose_events);
  if (!state.inventory || typeof state.inventory !== 'object') state.inventory = {};
  if (!state.unlocks || typeof state.unlocks !== 'object') state.unlocks = {};
  if (typeof state.dialogue !== 'string') state.dialogue = '';
  state.checkpoint_index = Math.max(0, Math.trunc(finite(state.checkpoint_index, 0)));

  const controller = {
    scene,
    nodes: records,
    debug: { get enabled() { return debugEnabled; }, set enabled(value) { debugEnabled = !!value; } },
    audit,
    snapshot: audit,
    emit,
    hit,
    pickup,
    win,
    lose,
    spawnProjectile,
    projectileHit,
    action,
    attachment,
    reset,
    step,
    render,
    dispose() {
      disposed = true;
      if (debugListener && typeof window !== 'undefined') window.removeEventListener('message', debugListener);
      if (debugOverlay?.destroy) debugOverlay.destroy();
      debugOverlay = null;
      for (const record of records.values()) {
        if (record.dynamic) removeRuntimeNode(record.node, record.object, 'dispose', true);
      }
      records.clear();
    },
  };
  function fault(code) {
    const value = String(code).slice(0, 160);
    if (faults.length < 16 && !faults.includes(value)) faults.push(value);
  }

  function emit(type, node, detail = {}) {
    const key = String(type || 'event').toLowerCase().replace(/[^a-z0-9_:-]/g, '_');
    events[key] = finite(events[key]) + 1;
    eventTrace.push({ type: key, id: node?.id || '', at: Math.round(elapsed * 1000) / 1000 });
    if (eventTrace.length > MAX_EVENT_TRACE) eventTrace.shift();
    const bindings = eventBindings.get(key);
    const payload = bindings?.length ? { ...object(detail), bindings: bindings.slice(0, 16) } : detail;
    host.event?.(key, node, payload);
  }
  function setOutcome(value, eventName, node, detail) {
    if (state.outcome !== 0) return false;
    state.outcome = value === 'won' ? 1 : 2;
    if (eventName === 'win') state.win_events++;
    if (eventName === 'lose') state.lose_events++;
    if (eventName) emit(eventName, node, detail);
    host.end?.(value === 'won');
    return true;
  }

  function win(node, detail) {
    return setOutcome('won', 'win', node, detail);
  }

  function lose(node, detail) {
    return setOutcome('lost', 'lose', node, detail);
  }

  function boundedIncrement(map, key, amount) {
    const normalized = String(key).slice(0, 64);
    if (!Object.prototype.hasOwnProperty.call(map, normalized) && Object.keys(map).length >= MAX_STATE_KEYS) {
      fault('state_key_limit');
      return;
    }
    map[normalized] = finite(map[normalized]) + amount;
  }

  function completeGoal(record) {
    if (!record.goalCounted) return;
    record.goalCounted = false;
    state.goal_remaining = Math.max(0, finite(state.goal_remaining) - 1);
  }

  function removeRuntimeNode(node, objectValue, reason, dynamic = false) {
    if (dynamic && typeof host.destroyNode === 'function') host.destroyNode(node, objectValue, reason);
    else host.removeNode?.(node, objectValue, reason);
  }

  function pickup(node, detail = {}) {
    const record = records.get(node?.id);
    if (!record || !record.active) return false;
    record.active = false;
    state.pickup_events++;
    const collect = { ...object(node.properties), ...object(behaviorConfig(node, 'collect')), ...object(detail) };
    state.score = finite(state.score) + Math.max(0, finite(collect.points, 0));
    completeGoal(record);
    emit('pickup', node, detail);
    removeRuntimeNode(node, record.object, 'pickup', record.dynamic);
    if (collect.win_when_cleared === true && state.goal_remaining === 0) win(node);
    return true;
  }

  function hit(nodeID, detail = {}) {
    const record = records.get(String(nodeID));
    if (!record || !record.active) return false;
    const node = record.node;
    const hitBehavior = behaviorConfig(node, 'destroy') || behaviorConfig(node, 'health');
    if (!hitBehavior) {
      fault('hit_target_not_damageable:' + node.id);
      return false;
    }
    record.health = Math.max(0, finite(record.health, finite(hitBehavior.health, 1)) - 1);
    state.hit_events++;
    state.hits = finite(state.hits) + 1;
    emit('hit', node, detail);
    const destroy = behaviorConfig(node, 'destroy');
    if (destroy && record.health <= 0) {
      record.active = false;
      completeGoal(record);
      emit('destroy', node, detail);
      removeRuntimeNode(node, record.object, 'destroy', record.dynamic);
      if (destroy.win_when_cleared && state.goal_remaining === 0) win(node);
    }
    return true;
  }

  function projectileHit(projectileID, targetID, detail = {}) {
    const target = String(targetID || '');
    if (!target) return false;
    const didHit = hit(target, { ...object(detail), projectile_id: String(projectileID || '') });
    if (!didHit) return false;
    const projectile = records.get(String(projectileID || ''));
    if (projectile?.active && behaviorConfig(projectile.node, 'projectile')?.pierce !== true) {
      projectile.active = false;
      removeRuntimeNode(projectile.node, projectile.object, 'projectile_hit', projectile.dynamic);
    }
    emit('projectile_hit', projectile?.node || { id: String(projectileID || '') }, { target_id: target });
    return true;
  }

  function applyBehavior(node, record, item) {
    const type = behaviorType(item);
    if (type === 'collect' || type === 'pickup') pickup(node, item);
    else if (type === 'reach' || type === 'goal') {
      completeGoal(record);
      if (state.goal_remaining === 0 && item.win_when_cleared !== false) win(node, item);
    } else if (type === 'checkpoint') {
      const sequence = Array.isArray(item.sequence) ? item.sequence : array(node.properties?.sequence);
      const index = Math.max(0, Math.trunc(finite(state.checkpoint_index, 0)));
      const expected = sequence.length ? String(sequence[index] || '') : '';
      if (expected && expected !== node.id && expected !== String(item.id || '')) {
        emit('checkpoint_miss', node, { expected, index });
        return;
      }
      state.checkpoint_index = sequence.length ? index + 1 : index;
      state.checkpoint = node.id;
      emit('checkpoint', node, sequence.length ? { ...object(item), index } : item);
      if (sequence.length && index + 1 >= sequence.length && item.win_when_cleared === true) {
        completeGoal(record);
        if (state.goal_remaining === 0) win(node, item);
      }
    } else if (type === 'trigger') emit(String(item.event || 'trigger'), node, item);
    else if (type === 'dialogue') {
      state.dialogue = String(item.text || item.line || item.message || '').slice(0, 256);
      emit('dialogue', node, item);
    } else if (type === 'unlock') {
      const key = String(item.key || item.id || node.id).slice(0, 64);
      state.unlocks[key] = true;
      emit('unlock', node, item);
    } else if (type === 'inventory') {
      const key = String(item.key || item.item || node.id);
      boundedIncrement(state.inventory, key, Math.max(1, finite(item.amount, 1)));
      emit('inventory', node, item);
    } else if (type === 'damage') {
      const amount = Math.max(1, finite(item.amount, 1));
      if (Number.isFinite(state.health)) state.health = Math.max(0, state.health - amount);
      else state.lives = finite(state.lives, 0) - amount;
      emit('damage', node, item);
      if ((Number.isFinite(state.health) && state.health <= 0) ||
        (!Number.isFinite(state.health) && state.lives <= 0)) lose(node, item);
    }
  }
  function contact(node, record, player) {
    const cooldown = Math.max(0, finite(node.contact_cooldown ?? node.cooldown, 0));
    if (elapsed < record.nextContact) return;
    const touching = host.overlap
      ? !!host.overlap(node, record.object, player)
      : distance(record, player, dimension) <= finite(node.contact_radius ?? node.radius, 1.2);
    if (!touching) return;
    const oneShot = node.behaviors.some(item => ['inventory', 'dialogue', 'unlock', 'checkpoint', 'trigger'].includes(behaviorType(item)) && item.repeat !== true) && node.properties?.repeat !== true;
    if (oneShot && record.contactConsumed) return;
    record.nextContact = elapsed + cooldown;
    if (oneShot) record.contactConsumed = true;
    for (const item of node.behaviors) applyBehavior(node, record, item);
  }
  function moveRecord(node, record, delta) {
    if (!record.active || !delta.some(value => Math.abs(value) > 1e-9)) return true;
    const context = { scene, records, controller };
    if (typeof host.canMove === 'function') {
      if (host.canMove(node, record.object, delta, context) === false) {
        emit('blocked', node, { delta });
        return false;
      }
    } else {
      if (!withinWorldBounds(node, record, delta, dimension, scene.world_bounds)) {
        emit('blocked', node, { delta, reason: 'world_bounds' });
        return false;
      }
      for (const obstacle of records.values()) {
        if (obstacle === record || !obstacle.active || !solidNode(obstacle.node)) continue;
        const current = actualPosition(record, dimension);
        if (overlapAABB(obstacle.node, current, delta, dimension)) {
          emit('blocked', node, { delta, obstacle_id: obstacle.node.id });
          return false;
        }
      }
    }
    return host.move?.(node, record.object, delta, context) !== false;
  }

  function moveNode(node, record, dt) {
    if (node.kind === 'player' || node.properties?.player === true) return;
    const movement = node.behaviors.find(item =>
      behaviorType(item) === 'movement' || ['patrol', 'chase', 'keepdistance'].includes(behaviorType(item)));
    if (!movement || !record.active) return;
    const type = behaviorType(movement) === 'movement'
      ? String(movement.mode || movement.behavior || movement.movement || '').trim().toLowerCase()
      : behaviorType(movement);
    if (type === 'patrol') {
      const route = scene.routes[movement.route_id || movement.routeId];
      const points = array(movement.points || movement.route || route?.waypoints || route);
      if (points.length < 2) {
        fault('patrol_route_missing:' + node.id);
        return;
      }
      const target = position(points[record.routeIndex % points.length], dimension);
      const current = actualPosition(record, dimension);
      const speed = Math.max(0, finite(movement.speed, 1));
      const delta = target.map((value, index) => value - current[index]);
      const length = Math.hypot(...delta);
      if (length < .1) record.routeIndex = (record.routeIndex + 1) % points.length;
      else moveRecord(node, record, delta.map(value => value / length * speed * dt));
    } else if (type === 'chase' || type === 'keepdistance') {
      const player = host.player?.();
      if (!player) return;
      const current = actualPosition(record, dimension);
      const playerObject = player?.object || player;
      const target = dimension === '3d'
        ? actualPosition({ object: playerObject }, dimension)
        : [finite(playerObject?.x), finite(playerObject?.y), 0];
      const delta = target.map((value, index) => value - current[index]);
      const length = Math.hypot(...delta);
      const desired = Math.max(0, finite(movement.distance, finite(movement.min_distance, 4)));
      const shouldMove = type === 'chase' ? length > .1 : Math.abs(length - desired) > .2;
      if (shouldMove && length > .001) {
        const direction = type === 'keepdistance' && length < desired ? -1 : 1;
        moveRecord(node, record, delta.map(value => value / length * direction * Math.max(0, finite(movement.speed, 1)) * dt));
      }
    }
  }

  function projectileTargetIDs(projectileRecord) {
    const result = [];
    const projectileNode = projectileRecord.node;
    const projectilePosition = actualPosition(projectileRecord, dimension);
    for (const target of records.values()) {
      if (target === projectileRecord || !target.active || target.node.kind === 'player' || target.node.properties?.player === true) continue;
      if (!behaviorConfig(target.node, 'destroy') && !behaviorConfig(target.node, 'health')) continue;
      const targetPosition = actualPosition(target, dimension);
      const radius = finite(projectileNode.properties?.radius, 1) + finite(target.node.properties?.radius, 1);
      const separation = Math.hypot(...projectilePosition.map((value, index) => value - targetPosition[index]));
      if (separation <= radius || overlapAABB(target.node, projectilePosition, [0, 0, 0], dimension)) result.push(target.node.id);
    }
    return result;
  }

  function updateProjectile(record, delta) {
    const projectile = behaviorConfig(record.node, 'projectile');
    if (!projectile) return;
    const externalHits = host.stepProjectile?.(record.node, record.object, delta, controller);
    const hitIDs = [];
    if (typeof externalHits === 'string') hitIDs.push(externalHits);
    else if (Array.isArray(externalHits)) hitIDs.push(...externalHits);
    else if (externalHits && typeof externalHits === 'object') {
      if (externalHits.target_id) hitIDs.push(externalHits.target_id);
      hitIDs.push(...array(externalHits.target_ids));
    }
    const direction = position(projectile.direction || record.node.properties?.direction, dimension);
    const speed = Math.max(0, finite(projectile.speed, 0));
    if (!hitIDs.length && speed > 0 && Math.hypot(...direction) > .001) {
      const length = Math.hypot(...direction);
      moveRecord(record.node, record, direction.map(value => value / length * speed * delta));
      hitIDs.push(...projectileTargetIDs(record));
    }
    for (const targetID of hitIDs.slice(0, 16)) projectileHit(record.node.id, targetID, { source: 'projectile' });
  }
  function spawnResults(value) {
    if (Array.isArray(value)) return value;
    if (Array.isArray(value?.objects)) return value.objects;
    return value ? [value] : [];
  }

  function dynamicNode(template, sourceNode, id, positionOverride) {
    const source = object(template);
    const raw = {
      ...source,
      id,
      position: positionOverride || source.position || source.at || sourceNode.at,
      properties: { ...object(sourceNode.properties), ...object(source.properties) },
    };
    return normalizeNode(raw, scene.nodes.length + records.size, dimension, usedNodeIDs);
  }

  function registerCreated(node, created, dynamic = false) {
    const made = spawnResults(created);
    if (!made.length) {
      fault('node_spawn_failed:' + node.id);
      return [];
    }
    const ids = [];
    for (const result of made) {
      const actual = result?.object !== undefined ? result.object : result;
      if (!actual) {
        fault('node_spawn_failed:' + node.id);
        continue;
      }
      let recordNode = node;
      if (ids.length) recordNode = normalizeNode({ ...node, id: node.id + '-' + (ids.length + 1) }, scene.nodes.length + records.size, dimension, usedNodeIDs);
      const record = {
        node: recordNode,
        object: actual,
        active: true,
        dynamic,
        start: recordNode.at.slice(),
        createdAt: elapsed,
        nextContact: 0,
        contactConsumed: false,
        routeIndex: 0,
        waveIndex: 0,
        nextWave: 0,
        goalCounted: isGoalNode(recordNode),
        initialHealth: Math.max(1, finite((recordNode.kind === 'player' || recordNode.properties?.player === true) && Number.isFinite(configuredHealth) ? configuredHealth : behaviorConfig(recordNode, 'health')?.health, finite(behaviorConfig(recordNode, 'destroy')?.health, 1))),
        health: 1,
      };
      record.health = record.initialHealth;
      records.set(recordNode.id, record);
      ids.push(recordNode.id);
      if (dynamic && record.goalCounted) state.goal_remaining = finite(state.goal_remaining) + 1;
      const actualRole = String(typeof host.actualRole === 'function' ? host.actualRole(recordNode, actual) || '' : '').trim();
      if (recordNode.required_role && actualRole !== recordNode.required_role) fault('required_role_mismatch:' + recordNode.id);
      if (recordNode.required_role && !actualRole) fault('required_role_missing:' + recordNode.id);
      host.onNode?.(recordNode, actual, record, controller);
      for (const placement of array(recordNode.placements)) {
        host.attachVisual?.(recordNode, placement, actual, record, controller);
      }
    }
    return ids;
  }

  function spawnWave(sourceNode, wave, index) {
    const templateID = String(wave.node_id || wave.nodeId || wave.template_id || wave.templateId || wave.spawn_node || '').trim();
    const template = object(wave.node || scene.nodes.find(node => node.id === templateID) || sourceNode);
    const id = String(template.id || sourceNode.id) + '-wave-' + (index + 1);
    const node = dynamicNode(template, sourceNode, id, wave.position);
    const spawned = typeof host.spawnWave === 'function'
      ? host.spawnWave(sourceNode, wave, index, node, controller)
      : host.createNode?.(node, scene.nodes.length + records.size);
    const ids = registerCreated(node, spawned, true);
    if (!ids.length) fault('wave_spawn_failed:' + sourceNode.id);
    return ids;
  }

  function attachment(id) {
    const attachmentID = String(id || '').trim();
    const item = scene.attachments.find(value => value.id === attachmentID);
    if (!item) {
      fault('attachment_missing:' + attachmentID);
      return null;
    }
    const record = records.get(item.node_id);
    if (!record) {
      fault('attachment_node_missing:' + item.node_id);
      return null;
    }
    if (typeof host.resolveAttachment !== 'function') {
      fault('attachment_resolver_missing:' + item.id);
      return null;
    }
    const resolved = host.resolveAttachment(item, record.object, record.node);
    if (!resolved) {
      fault('attachment_unresolved:' + item.id);
      return null;
    }
    return resolved;
  }
  function action(detail = {}) {
    if (disposed || state.outcome !== 0) return 0;
    const player = host.player?.();
    const playerObject = player?.object || player;
    const playerNode = scene.nodes.find(node =>
      node.kind === 'player' || node.properties?.player === true);
    const playerRecord = playerNode ? records.get(playerNode.id) : null;
    if (!playerNode || !playerRecord?.active || !playerObject) return 0;
    const mechanics = parseMechanics(mechanicsSource);
    let handled = 0;
    for (const [index, rawBlock] of array(mechanics.blocks).entries()) {
      const block = object(rawBlock);
      if (block.enabled === false) continue;
      const kind = String(block.kind || '').trim().toLowerCase();
      const params = typeof block.params === 'string' ? parseJSON(block.params, null) : object(block.params);
      if (params === null) {
        if (kind === 'projectile' || kind === 'trigger') fault('mechanic_params_invalid:' + kind);
        continue;
      }
      const actionRequested = kind === 'projectile' || kind === 'trigger' ||
        block.on_action === true || params.on_action === true;
      if (!actionRequested) continue;
      const targetID = String(block.target || block.node_id || block.nodeId || params.target || '').trim();
      const role = String(block.role || params.role || '').trim();
      const targetsPlayer = targetID
        ? targetID === playerNode.id
        : role
          ? playerNode.role === role || array(playerNode.asset_roles).includes(role)
          : false;
      if (!targetsPlayer) continue;
      const actionID = String(block.id || kind + '-' + index).slice(0, 96);
      const cooldown = Math.max(0, finite(params.cooldown ?? block.cooldown, 0));
      if (elapsed < finite(actionNext.get(actionID), 0)) continue;
      if (kind === 'projectile') {
        const nested = object(params.projectile);
        const projectile = { ...nested, ...params };
        delete projectile.projectile;
        delete projectile.target;
        delete projectile.role;
        delete projectile.enabled;
        delete projectile.on_action;
        delete projectile.cooldown;
        projectile.position = projectile.position || actualPosition(playerObject, dimension);
        projectile.id = projectile.id || block.id || 'projectile';
        const id = spawnProjectile(projectile, {
          ...object(detail),
          source: 'action',
          mechanic_id: actionID,
        });
        if (!id) continue;
        handled++;
        actionNext.set(actionID, elapsed + cooldown);
      } else {
        const candidate = mechanicBehavior(block, params);
        if (!candidate) {
          fault('mechanic_behavior_missing:' + kind);
          continue;
        }
        applyBehavior(playerNode, playerRecord, candidate);
        handled++;
        actionNext.set(actionID, elapsed + cooldown);
      }
    }
    return handled;
  }
  function spawnProjectile(spec = {}, detail = {}) {
    const request = typeof spec === 'string' ? { node_id: spec } : object(spec);
    const templateID = String(request.node_id || request.nodeId || request.template_id || request.templateId || '').trim();
    const template = object(request.node || scene.nodes.find(node => node.id === templateID) || {});
    const requestBehavior = object(request.behavior);
    const sourceBehavior = {
      ...(typeof template.behavior === 'object' ? template.behavior : {}),
      ...requestBehavior,
    };
    for (const key of ['direction', 'speed', 'ttl', 'lifetime', 'pierce']) {
      if (request[key] !== undefined) sourceBehavior[key] = request[key];
    }
    const source = template.id ? {
      ...template,
      behavior: sourceBehavior,
      properties: { ...object(template.properties), ...object(request.properties) },
    } : {
      id: 'projectile',
      kind: 'projectile',
      position: request.position,
      behavior: { type: 'projectile', ...sourceBehavior },
      properties: object(request.properties),
    };
    const id = String(request.id || source.id || 'projectile') + '-' + (records.size + 1);
    const node = dynamicNode({ ...source, behavior: source.behavior || 'projectile', properties: { ...object(source.properties), ...request.properties } }, source, id, request.position);
    const projectile = behaviorConfig(node, 'projectile') || { type: 'projectile' };
    const spawned = typeof host.spawnProjectile === 'function'
      ? host.spawnProjectile(node, projectile, detail, controller)
      : host.createNode?.(node, scene.nodes.length + records.size);
    const ids = registerCreated(node, spawned, true);
    if (!ids.length) fault('projectile_spawn_failed:' + id);
    return ids[0] || '';
  }
  function processZones() {
    const player = host.player?.();
    if (!player) return;
    const playerObject = player?.object || player;
    for (const item of scene.attachments) {
    if (!item.id || !item.node_id || !item.socket) fault('attachment_invalid:' + (item.id || 'unknown'));
    if (item.node_id && !usedNodeIDs.has(item.node_id)) fault('attachment_node_missing:' + item.id);
  }
  for (const zone of scene.zones) {
      if (state.outcome !== 0) break;
      const zoneState = zoneStates.get(zone.id) || { inside: false, next: 0 };
      const touching = host.zoneOverlap
        ? !!host.zoneOverlap(zone, playerObject, { scene, controller })
        : pointInsideBounds(actualPosition({ object: playerObject }, dimension), zone.bounds, dimension);
      const entered = touching && !zoneState.inside;
      const repeat = zone.properties.repeat === true;
      if (touching && (entered || repeat) && elapsed >= zoneState.next) {
        zoneState.next = elapsed + Math.max(0, finite(zone.properties.cooldown, 0));
        if (zone.kind === 'spawn') {
          emit('spawn', zone, { zone_id: zone.id });
          if (zone.properties.auto_spawn === true) host.spawnPlayer?.(zone, playerObject, zone.bounds, controller);
        } else if (zone.kind === 'goal') {
          emit('goal', zone, { zone_id: zone.id });
          if (zone.properties.win_when_entered !== false) win(zone);
        } else if (zone.kind === 'loss') {
          emit('loss', zone, { zone_id: zone.id });
          const damage = finite(zone.properties.damage ?? zone.properties.lives, NaN);
          if (Number.isFinite(damage) && Number.isFinite(state.lives)) {
            state.lives -= Math.max(1, damage);
            emit('damage', zone, { amount: Math.max(1, damage) });
            if (state.lives <= 0) lose(zone);
          } else {
            lose(zone);
          }
        } else if (zone.kind === 'trigger') {
          emit(String(zone.properties.event || 'trigger'), zone, { zone_id: zone.id, properties: zone.properties });
          for (const item of behaviorList({ behavior: zone.properties.behavior || zone.properties.behaviors })) {
            applyBehavior(zone, { node: zone, active: true }, item);
          }
        }
      }
      zoneState.inside = touching;
      zoneStates.set(zone.id, zoneState);
    }
  }
  function step(dt = 0) {
    if (disposed || state.outcome !== 0) return;
    const delta = clamp(finite(dt) / 1000, 0, .25);
    elapsed += delta;
    for (const record of [...records.values()]) {
      if (!record.active) continue;
      const projectile = behaviorConfig(record.node, 'projectile');
      const ttl = finite(record.node.ttl ?? record.node.lifetime ?? projectile?.ttl, NaN);
      if (Number.isFinite(ttl) && elapsed - record.createdAt >= ttl) {
        record.active = false;
        emit('expired', record.node);
        removeRuntimeNode(record.node, record.object, 'ttl', record.dynamic);
        continue;
      }
      if (projectile) {
        const externalHits = host.stepProjectile?.(record.node, record.object, delta, controller);
        const hitIDs = typeof externalHits === 'string' ? [externalHits] :
          Array.isArray(externalHits) ? externalHits :
          externalHits?.target_id ? [externalHits.target_id, ...array(externalHits.target_ids)] : [];
        const projectileConfig = behaviorConfig(record.node, 'projectile') || {};
        const direction = position(projectileConfig.direction || record.node.properties?.direction, dimension);
        const speed = Math.max(0, finite(projectileConfig.speed, 0));
        if (!hitIDs.length && speed > 0 && Math.hypot(...direction) > .001) {
          const length = Math.hypot(...direction);
          const start = actualPosition(record, dimension);
          moveRecord(record.node, record, direction.map(value => value / length * speed * delta));
          const end = actualPosition(record, dimension);
          for (const target of records.values()) {
            if (target === record || !target.active || target.node.kind === 'player' || target.node.properties?.player === true) continue;
            if (!behaviorConfig(target.node, 'destroy') && !behaviorConfig(target.node, 'health')) continue;
            const targetPoint = actualPosition(target, dimension);
            const radius = finite(record.node.properties?.radius, 1) + finite(target.node.properties?.radius, 1);
            if (segmentDistance(targetPoint, start, end) <= radius) hitIDs.push(target.node.id);
          }
        }
        for (const targetID of hitIDs.slice(0, 16)) projectileHit(record.node.id, targetID, { source: 'projectile' });
      } else {
        moveNode(record.node, record, delta);
        const player = host.player?.();
        if (player && record.node.kind !== 'player' && record.node.properties?.player !== true) {
          contact(record.node, record, player);
        }
      }
    }
    for (const record of [...records.values()]) {
      if (!record.active) continue;
      const wave = behaviorConfig(record.node, 'waves') || behaviorConfig(record.node, 'wave');
      if (!wave) continue;
      const interval = Math.max(.01, finite(wave.interval, finite(wave.every, 1)));
      const rawLimit = Number(wave.count ?? wave.waves);
      const limit = Number.isFinite(rawLimit) ? Math.max(0, rawLimit) : 1;
      if (record.waveIndex < limit && elapsed >= record.nextWave) {
        spawnWave(record.node, wave, record.waveIndex);
        record.waveIndex++;
        record.nextWave = elapsed + interval;
        emit('wave', record.node, { index: record.waveIndex });
      }
    }
    processZones();
    for (const node of scene.nodes) {
      const survive = behaviorConfig(node, 'survive');
      if (survive && elapsed >= Math.max(0, finite(survive.duration, finite(survive.seconds, 0)))) win(node, survive);
    }
    host.step?.(delta, controller);
    syncState();
  }
  function syncState() {
    state.remaining = [...records.values()].filter(record => record.active).length;
    state.outcome = state.outcome === 1 ? 1 : state.outcome === 2 ? 2 : 0;
  }

  function reset() {
    elapsed = 0;
    for (const record of [...records.values()]) {
      if (record.dynamic) {
        removeRuntimeNode(record.node, record.object, 'reset', true);
        records.delete(record.node.id);
        continue;
      }
      record.active = true;
      record.health = record.initialHealth;
      record.nextContact = 0;
      record.contactConsumed = false;
      record.routeIndex = 0;
      record.waveIndex = 0;
      record.nextWave = 0;
      record.goalCounted = isGoalNode(record.node);
      host.resetNode?.(record.node, record.object, record.start);
    }
    state.outcome = 0;
    state.score = 0;
    state.hits = 0;
    state.actions = 0;
    state.hit_events = 0;
    state.pickup_events = 0;
    state.win_events = 0;
    state.lose_events = 0;
    state.goal_remaining = initialGoalRemaining;
    if (Number.isFinite(configuredLives)) state.lives = configuredLives;
    if (Number.isFinite(configuredHealth)) state.health = configuredHealth;
    state.inventory = {};
    state.unlocks = {};
    state.dialogue = '';
    state.checkpoint = '';
    state.checkpoint_index = 0;
    for (const zoneState of zoneStates.values()) {
      zoneState.inside = false;
      zoneState.next = 0;
    }
    eventsReset();
    actionNext.clear();
    host.reset?.(controller);
  }
  function eventsReset() {
    for (const key of Object.keys(events)) delete events[key];
    eventTrace.length = 0;
  }

  function audit() {
    const roleRows = new Map();
    for (const record of records.values()) {
      const actualAssets = array(host.actualAssets?.(record.node, record.object));
      const rows = actualAssets.length ? actualAssets : [{
        role: typeof host.actualRole === 'function' ? host.actualRole(record.node, record.object) : '',
        asset_id: typeof host.actualAssetID === 'function' ? host.actualAssetID(record.node, record.object) : '',
      }];
      for (const asset of rows) {
        const role = String(asset?.role || '').trim();
        if (!role) continue;
        const assetID = String(asset?.asset_id || '').trim();
        const key = role + '\u0000' + assetID;
        const row = roleRows.get(key) || { role, asset_id: assetID, count: 0 };
        row.count++;
        roleRows.set(key, row);
      }
    }
    return {
      outcome: state.outcome === 1 ? 'won' : state.outcome === 2 ? 'lost' : 'playing',
      roles: [...roleRows.values()].slice(0, MAX_AUDIT_ROLES),
      events: { ...events },
      event_trace: eventTrace.slice(-32),
      faults: faults.slice(0, MAX_AUDIT_FAULTS),
      nodes: [...records.values()].slice(0, MAX_AUDIT_NODES).map(record => {
        const p = actualPosition(record, dimension);
        return { id: record.node.id, active: !!record.active, health: record.health, x: p[0], y: p[1], z: p[2] };
      }),
      resource_counts: object(host.resourceCounts?.()),
      inventory: Object.fromEntries(Object.entries(object(state.inventory)).slice(0, 32).map(([key, value]) => [String(key).slice(0, 64), finite(value)])),
      unlocks: Object.keys(object(state.unlocks)).slice(0, 32).map(key => String(key).slice(0, 64)),
      dialogue: String(state.dialogue || '').slice(0, 256),
      presentation: host.presentation?.audit?.(),
    };
  }
  function render() {
    if (!debugEnabled) {
      if (debugOverlay?.clear) debugOverlay.clear();
      return;
    }
    host.renderDebug?.(controller, debugOverlay);
  }

  function bindDebug() {
    if (typeof window === 'undefined' || !window.parent || window.parent === window) return;
    debugListener = event => {
      const data = event.data;
      const channel = readChannel();
      const expectedOrigin = String(globalThis.__AURAGO_STUDIO_ORIGIN__ || '').trim();
      if (event.source !== window.parent || !channel || !data || data.source !== 'aurago-studio' ||
        data.type !== 'scene-debug' || data.channel !== channel || typeof data.enabled !== 'boolean' ||
        (expectedOrigin && event.origin && event.origin !== expectedOrigin)) return;
      debugEnabled = data.enabled;
      host.debugChanged?.(debugEnabled);
    };
    window.addEventListener('message', debugListener);
  }
  bindDebug();
  scene.nodes.forEach((node, index) => {
    if (node.active === false) return;
    const created = host.createNode?.(node, index);
    registerCreated(node, created, false);
  });  syncState();
  return controller;
}

export function createScene2D(raw, host = {}) {
  const scene = normalizeScene(raw, '2d', host.viewport || {});
  return makeController(scene, host, '2d');
}

export function createScene3D(raw, host = {}) {
  const scene = normalizeScene(raw, '3d', host.viewport || {});
  return makeController(scene, host, '3d');
}

export function hasSceneNodes(raw) {
  const source = object(raw);
  return array(source.nodes || source.objects || source.entities).length > 0 ||
    array(source.placements).length > 0 ||
    array(source.regions).length > 0 ||
    array(source.colliders).length > 0 ||
    array(source.zones).length > 0 ||
    array(source.routes).length > 0 ||
    array(source.attachments).length > 0;
}
