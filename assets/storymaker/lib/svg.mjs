// A small renderer that follows contract section 3a literally, producing SVG.
// It is used for the committed thumbnails and for the review sheets, so the art is judged under the same
// transform and variant rules the player (part C) implements for Canvas2D.
//
//   world(b) = world(parent) . T(rest[view].x, rest[view].y) . R(rest[view].angle + pose.bones[b] + sum deltas[b])
//
// Angles are degrees, clockwise positive (y points down). Slot shapes are in the bone's local coordinates.
// Facing left is scale(-1, 1) about the root x after the view is chosen. Strokes are drawn with round caps.

import { n, n2 } from './util.mjs';

export const VIEWS = ['front', 'three_quarter', 'profile'];

// ---------------------------------------------------------------- matrices (SVG order: [a b c d e f])
const IDENT = [1, 0, 0, 1, 0, 0];
const mul = (m, k) => [
  m[0] * k[0] + m[2] * k[1], m[1] * k[0] + m[3] * k[1],
  m[0] * k[2] + m[2] * k[3], m[1] * k[2] + m[3] * k[3],
  m[0] * k[4] + m[2] * k[5] + m[4], m[1] * k[4] + m[3] * k[5] + m[5],
];
const T = (x, y) => [1, 0, 0, 1, x, y];
const R = (degrees) => {
  const a = (degrees * Math.PI) / 180;
  return [Math.cos(a), Math.sin(a), -Math.sin(a), Math.cos(a), 0, 0];
};
const matStr = (m) => `matrix(${m.map((v) => Math.round(v * 10000) / 10000).join(' ')})`;

/** World matrices of all bones for a view, body type, pose offsets and extra deltas (gesture/cycle). */
export function boneWorlds(rig, bodyName, view, poseBones = {}, deltas = {}) {
  const body = rig.bodies?.[bodyName] || {};
  const worlds = new Map();
  for (const bone of rig.bones) {
    const ov = body.bones?.[bone.name]?.rest?.[view] || {};
    const rest = { ...bone.rest[view], ...ov };
    const angle = (rest.angle || 0) + (poseBones[bone.name] || 0) + (deltas[bone.name] || 0);
    const local = mul(T(rest.x || 0, rest.y || 0), R(angle));
    worlds.set(bone.name, bone.parent ? mul(worlds.get(bone.parent), local) : local);
  }
  return worlds;
}

// ---------------------------------------------------------------- keys (gestures and cycles)
/** Linear blend of key frames {t, bones, hands?, offset_y?} at time t (0..1). Missing bones count as 0. */
export function evalKeys(keys, t) {
  let i = 0;
  while (i < keys.length - 2 && t > keys[i + 1].t) i++;
  const k0 = keys[i];
  const k1 = keys[Math.min(i + 1, keys.length - 1)];
  const span = k1.t - k0.t;
  const u = span > 0 ? Math.min(1, Math.max(0, (t - k0.t) / span)) : 0;
  const bones = {};
  for (const name of new Set([...Object.keys(k0.bones || {}), ...Object.keys(k1.bones || {})])) {
    const a = k0.bones?.[name] || 0;
    const b = k1.bones?.[name] || 0;
    bones[name] = a + (b - a) * u;
  }
  let hands = {};
  for (const k of keys) if (k.t <= t && k.hands) hands = { ...hands, ...k.hands };
  const oy = (k0.offset_y || 0) + ((k1.offset_y || 0) - (k0.offset_y || 0)) * u;
  return { bones, hands, offset_y: oy };
}

// ---------------------------------------------------------------- colours and shapes
export class Ctx {
  constructor() {
    this.gradients = [];
    this.gid = 0;
    this.unresolved = new Set();
    this.missing = new Set();
  }
}

function colourOf(c, maps, ctx) {
  if (typeof c !== 'string') return c;
  if (c[0] !== '$') return c;
  const key = c.slice(1);
  for (const m of maps) if (m && Object.prototype.hasOwnProperty.call(m, key)) return m[key];
  ctx.unresolved.add(c);
  return '#ff00ff';
}

export function shapeSvg(shape, maps, ctx) {
  let fillAttr = 'none';
  if (shape.gradient) {
    const g = shape.gradient;
    const id = `g${ctx.gid++}`;
    const stops = g.stops.map(([o, c]) => `<stop offset="${o}" stop-color="${colourOf(c, maps, ctx)}"/>`).join('');
    ctx.gradients.push(
      g.type === 'radial'
        ? `<radialGradient id="${id}" gradientUnits="userSpaceOnUse" cx="${g.cx}" cy="${g.cy}" r="${g.r}">${stops}</radialGradient>`
        : `<linearGradient id="${id}" gradientUnits="userSpaceOnUse" x1="${g.x1}" y1="${g.y1}" x2="${g.x2}" y2="${g.y2}">${stops}</linearGradient>`,
    );
    fillAttr = `url(#${id})`;
  } else if (shape.fill) {
    fillAttr = colourOf(shape.fill, maps, ctx);
  }
  let attrs = `d="${shape.d}" fill="${fillAttr}"`;
  if (shape.stroke) attrs += ` stroke="${colourOf(shape.stroke, maps, ctx)}" stroke-width="${shape.stroke_width ?? 1}" stroke-linecap="round" stroke-linejoin="round"`;
  if (shape.opacity !== undefined && shape.opacity !== 1) attrs += ` opacity="${n2(shape.opacity)}"`;
  return `<path ${attrs}/>`;
}

export function shapesSvg(shapes, maps, ctx) {
  return shapes.map((s) => shapeSvg(s, maps, ctx)).join('');
}

// ---------------------------------------------------------------- characters
/** Colour maps for a character (the token rules of section 3a). */
export function characterMaps(rig, params, palette) {
  const outfit = rig.params.outfit[params.outfit];
  return [
    {
      skin: rig.params.skin[params.skin],
      hair: rig.params.hair_color[params.hair_color],
      top: outfit.colors.top,
      bottom: outfit.colors.bottom,
      shoes: outfit.colors.shoes,
    },
    rig.params.fixed,
    palette,
  ];
}

function slotKey(rig, slot, params, pose, expression, mouth) {
  const outfit = rig.params.outfit[params.outfit];
  switch (slot.param) {
    case 'body': return params.body;
    case 'skin': return params.skin;
    case 'hair': case 'hair_back': return params.hair;
    case 'eyes': return `${params.eyes}/${expression.eyes}`;
    case 'brows': return `${params.brows}/${expression.brows}`;
    case 'mouth': return mouth || expression.mouth;
    case 'hand': return slot.name === 'hand_l' ? (pose.hands?.l || 'open') : (pose.hands?.r || 'open');
    case 'top': case 'bottom': case 'shoes': return outfit[slot.param];
    case 'glasses': case 'facial_hair': case 'accessory': return params[slot.param];
    default: return undefined;
  }
}

/**
 * Draw a character in rig space (root at the origin). Returns the SVG fragment; the caller wraps it in the
 * placement transform. pose = {bones, hands}; deltas = extra bone deltas (gesture/cycle); expression = {eyes, brows, mouth}.
 */
export function characterSvg(rig, params, palette, ctx, opts = {}) {
  const view = opts.view || 'three_quarter';
  const pose = opts.pose || { bones: {} };
  const deltas = opts.deltas || {};
  const expression = opts.expression || { eyes: 'neutral', brows: 'neutral', mouth: 'rest' };
  const hands = { ...(pose.hands || {}), ...(opts.hands || {}) };
  const worlds = boneWorlds(rig, params.body, view, pose.bones || {}, deltas);
  const maps = characterMaps(rig, params, palette);
  const order = rig.slots.map((s, i) => ({ s, i })).sort((a, b) => a.s.z[view] - b.s.z[view] || a.i - b.i);
  let out = '';
  for (const { s: slot } of order) {
    const key = slotKey(rig, slot, params, { ...pose, hands }, expression, opts.mouth);
    if (key === undefined || key === 'none') continue;
    let variant = rig.variants[slot.name]?.[key];
    if (!variant && slot.param === 'eyes') variant = rig.variants[slot.name]?.[`${params.eyes}/neutral`];
    if (!variant && slot.param === 'brows') variant = rig.variants[slot.name]?.[`${params.brows}/neutral`];
    if (!variant) { ctx.missing.add(`${slot.name}:${key}`); continue; }
    const shapes = variant[view];
    if (!shapes || !shapes.length) continue;
    out += `<g transform="${matStr(worlds.get(slot.bone))}">${shapesSvg(shapes, maps, ctx)}</g>`;
  }
  return out;
}

/** A complete character placed at (x, y) (feet) with screen scale s; facing 'left' mirrors about the root. */
export function placedCharacter(rig, params, palette, ctx, opts = {}) {
  const body = rig.bodies?.[params.body] || { scale: 1 };
  const s = (opts.scale ?? 1) * (body.scale ?? 1);
  const oy = (opts.offset_y || 0) + (opts.pose?.offset_y || 0);
  const sx = opts.facing === 'left' ? -s : s;
  const inner = characterSvg(rig, params, palette, ctx, opts);
  return `<g transform="translate(${n(opts.x || 0)} ${n(opts.y || 0)}) scale(${n2(sx)} ${n2(s)}) translate(0 ${n(oy)})">${inner}</g>`;
}

// ---------------------------------------------------------------- documents
export function svgDoc(viewBox, inner, ctx, { width, height, background } = {}) {
  const [x, y, w, h] = viewBox;
  const defs = ctx && ctx.gradients.length ? `<defs>${ctx.gradients.join('')}</defs>` : '';
  const bg = background ? `<rect x="${x}" y="${y}" width="${w}" height="${h}" fill="${background}"/>` : '';
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="${x} ${y} ${w} ${h}" width="${width ?? w}" height="${height ?? h}">${defs}${bg}${inner}</svg>\n`;
}

/** Props resolve their colours from palette tokens only. */
export function propSvg(prop, palette, ctx) {
  return shapesSvg(prop.shapes, [palette], ctx);
}

/** Environments resolve from env.tokens[variant], then palette tokens. */
export function environmentSvg(env, variant, palette, ctx) {
  const maps = [env.tokens[variant], palette];
  return env.layers.map((layer) => `<g data-layer="${layer.name}">${shapesSvg(layer.shapes, maps, ctx)}</g>`).join('');
}
