// Checks of the generated catalog against contract section 3. The build refuses to write when anything fails;
// the Go test ui/storymaker_catalog_test.go repeats the schema checks on the committed files.

import { parsePath } from './pathdata.mjs';
import { versionOf, PALETTES } from './catalog.mjs';
import { NAMES as ALL_NAMES, LANGS, nameKey } from './names.mjs';
import { RASTER_NAMES } from './raster-names.mjs';
import { Ctx, characterSvg } from './svg.mjs';
import { MOUTH_NAMES } from './face.mjs';
import { GLYPHS_SCHEMA, decodeRanges, rangesCover } from './glyphs.mjs';

const NAMES = Object.fromEntries(Object.entries(ALL_NAMES).filter(([id]) => !Object.hasOwn(RASTER_NAMES, id)));

export const REQUIRED = {
  character: ['char.office-woman-1', 'char.office-man-1', 'char.it-tech-1', 'char.senior-man-1'],
  prop: ['laptop', 'phone', 'monitor', 'server', 'router', 'cloud', 'database', 'lock', 'key', 'shield', 'mail', 'document', 'coffee-cup', 'desk', 'chair', 'plant', 'check', 'cross', 'warning', 'idea', 'question', 'arrow-right', 'gear'].map((n) => `prop.${n}`),
  environment: ['env.plain-stage', 'env.office', 'env.home-office', 'env.abstract-tech'],
  pose: ['stand', 'sit', 'typing', 'point-left', 'point-right', 'wave', 'think', 'present', 'arms-crossed', 'hands-on-hips'].map((n) => `pose.${n}`),
  gesture: ['point', 'wave', 'shrug', 'think', 'present', 'thumbs-up', 'celebrate', 'facepalm', 'count'].map((n) => `gesture.${n}`),
  expression: ['neutral', 'happy', 'laugh', 'sad', 'angry', 'surprised', 'confused', 'thinking', 'worried', 'determined'].map((n) => `expr.${n}`),
  mouth: ['rest', 'ai', 'e', 'o', 'u', 'mbp', 'fv'].map((n) => `mouth.${n}`),
  cycle: ['cycle.walk', 'cycle.idle'],
  text_style: ['text.title', 'text.caption', 'text.label'],
  transition: ['trans.cut', 'trans.fade', 'trans.slide-left', 'trans.wipe'],
  sfx: ['whoosh', 'pop', 'click', 'ding', 'success', 'error', 'typing'].map((n) => `sfx.${n}`),
};
const PREFIX = { character: 'char.', prop: 'prop.', environment: 'env.', pose: 'pose.', gesture: 'gesture.', expression: 'expr.', mouth: 'mouth.', cycle: 'cycle.', text_style: 'text.', transition: 'trans.', sfx: 'sfx.' };
const BONES = ['root', 'hips', 'torso', 'neck', 'head', 'upper_arm_l', 'lower_arm_l', 'upper_arm_r', 'lower_arm_r', 'upper_leg_l', 'lower_leg_l', 'upper_leg_r', 'lower_leg_r'];
const SLOT_PARAMS = ['body', 'skin', 'hair', 'hair_back', 'eyes', 'brows', 'mouth', 'hand', 'top', 'bottom', 'shoes', 'glasses', 'facial_hair', 'accessory'];
const VIEWS = ['front', 'three_quarter', 'profile'];
const CHAR_TOKENS = ['skin', 'hair', 'top', 'bottom', 'shoes', 'outline', 'eye_white', 'iris', 'lip'];
const PALETTE_TOKENS = ['bg', 'bg2', 'fg', 'accent', 'accent2', 'text', 'outline'];

function shapeProblems(where, shapes, known, out) {
  shapes.forEach((s, i) => {
    const p = parsePath(s.d);
    if (!p.ok) out.push(`${where}[${i}]: bad path data: ${p.error}`);
    const colours = [s.fill, s.stroke, ...(s.gradient ? s.gradient.stops.map((st) => st[1]) : [])].filter((c) => typeof c === 'string');
    if (!s.fill && !s.stroke && !s.gradient) out.push(`${where}[${i}]: shape draws nothing`);
    for (const c of colours) {
      if (c[0] === '$' && !known.has(c.slice(1))) out.push(`${where}[${i}]: unknown token ${c}`);
      if (c[0] !== '$' && !/^#[0-9a-fA-F]{6}([0-9a-fA-F]{2})?$/.test(c)) out.push(`${where}[${i}]: bad colour ${c}`);
    }
    if (s.opacity !== undefined && !(s.opacity >= 0 && s.opacity <= 1)) out.push(`${where}[${i}]: opacity out of range`);
  });
}

function glyphProblems(data, add) {
  if (data.schema !== GLYPHS_SCHEMA) add(`glyphs.json: schema must be ${GLYPHS_SCHEMA}`);
  const ids = new Set();
  for (const f of data.faces) {
    if (ids.has(f.id)) add(`glyphs.json: duplicate face ${f.id}`);
    ids.add(f.id);
    if (!f.family || !f.weight || !f.file.startsWith('/fonts/')) add(`glyphs.json: face ${f.id} needs a family, a weight and a /fonts/ file`);
    if (!f.d.length || f.d.length % 2) add(`glyphs.json: face ${f.id} needs pairs of range values`);
    for (let i = 0; i < f.d.length; i += 2) {
      if (f.d[i] < (i ? 1 : 0) || f.d[i + 1] < 0) add(`glyphs.json: face ${f.id} is not monotonic at value ${i}`);
    }
  }
  if (!data.faces.some((f) => rangesCover(decodeRanges(f.d), 0x2026))) add('glyphs.json: U+2026 is not covered by any face');
}

export function validateOutputs(built, { maxProblems = 40 } = {}) {
  const out = [];
  const { files, manifest, rig, entries } = built;
  const add = (m) => { if (out.length < maxProblems) out.push(m); };

  if (manifest.schema !== 'storymaker-catalog/1') add('schema must be storymaker-catalog/1');
  if (manifest.version !== versionOf(files)) add('manifest version does not match the output files');
  if (!/^[0-9a-f]{12}$/.test(manifest.version)) add('version must be 12 hex digits');
  const ids = new Set();
  for (const e of entries) {
    if (ids.has(e.id)) add(`duplicate id ${e.id}`);
    ids.add(e.id);
    if (!e.id.startsWith(PREFIX[e.kind] || '\0')) add(`${e.id}: prefix does not match kind ${e.kind}`);
    if (e.name_key !== nameKey(e.id)) add(`${e.id}: wrong name_key`);
    if (!NAMES[e.id]) add(`${e.id}: no names`);
    if (e.kind !== 'sfx' && !files.has(e.thumb)) add(`${e.id}: missing thumbnail ${e.thumb}`);
    if (!Array.isArray(e.tags) || !e.tags.length) add(`${e.id}: no tags`);
    if (typeof e.usage_example !== 'object') add(`${e.id}: usage_example must be an object`);
    if (e.file && !files.has(e.file)) add(`${e.id}: missing file ${e.file}`);
  }
  for (const [kind, list] of Object.entries(REQUIRED)) for (const id of list) if (!ids.has(id)) add(`missing required ${kind} ${id}`);
  for (const p of manifest.palettes) {
    if (!NAMES[p.id]) add(`${p.id}: no names`);
    for (const t of PALETTE_TOKENS) if (!/^#[0-9a-f]{6}$/.test(p.tokens[t] || '')) add(`${p.id}: token ${t} missing`);
  }
  if (!manifest.palettes.some((p) => p.name === 'sunny') || !manifest.palettes.some((p) => p.name === 'calm')) add('palettes sunny and calm are required');

  // rig
  const rigTokens = new Set([...CHAR_TOKENS, ...PALETTE_TOKENS]);
  if (rig.bones.map((b) => b.name).join() !== BONES.join()) add('rig bones differ from the contract list');
  for (const b of rig.bones) for (const v of VIEWS) if (!b.rest[v]) add(`bone ${b.name} lacks rest.${v}`);
  for (const s of rig.slots) {
    if (!BONES.includes(s.bone)) add(`slot ${s.name}: unknown bone`);
    if (!SLOT_PARAMS.includes(s.param)) add(`slot ${s.name}: unknown param ${s.param}`);
    for (const v of VIEWS) if (!Number.isInteger(s.z[v])) add(`slot ${s.name}: z.${v} missing`);
    if (!rig.variants[s.name]) add(`slot ${s.name}: no variants`);
  }
  for (const [slot, keys] of Object.entries(rig.variants)) {
    for (const [key, views] of Object.entries(keys)) {
      for (const v of VIEWS) {
        if (!Array.isArray(views[v])) add(`variant ${slot}/${key} lacks ${v}`);
        else shapeProblems(`rig ${slot}/${key}/${v}`, views[v], rigTokens, out);
      }
    }
  }
  for (const name of Object.keys(rig.bodies)) if (!rig.params.body.includes(name)) add(`body ${name} not in params.body`);
  for (const b of rig.params.body) if (!rig.bodies[b]) add(`params.body lists ${b} without a body entry`);

  // every starter character, every view, every expression and hand renders without missing variants or tokens
  for (const e of entries.filter((x) => x.kind === 'character')) {
    for (const v of VIEWS) {
      for (const ex of entries.filter((x) => x.kind === 'expression')) {
        for (const hand of ['open', 'fist', 'point', 'hold']) {
          const ctx = new Ctx();
          characterSvg(rig, e.params, PALETTES.sunny, ctx, { view: v, pose: { bones: {}, hands: { l: hand, r: hand } }, expression: ex });
          if (ctx.missing.size) add(`${e.id} ${v} ${ex.id}: missing variants ${[...ctx.missing].join(',')}`);
          if (ctx.unresolved.size) add(`${e.id} ${v}: unresolved tokens ${[...ctx.unresolved].join(',')}`);
        }
      }
      for (const m of MOUTH_NAMES) {
        const ctx = new Ctx();
        characterSvg(rig, e.params, PALETTES.sunny, ctx, { view: v, mouth: m });
        if (ctx.missing.size) add(`${e.id} ${v} mouth ${m}: missing variants`);
      }
    }
    for (const [k, list] of Object.entries({ body: rig.params.body, hair: rig.params.hair, eyes: rig.params.eyes, brows: rig.params.brows, glasses: rig.params.glasses, facial_hair: rig.params.facial_hair, accessory: rig.params.accessory })) {
      if (!list.includes(e.params[k])) add(`${e.id}: ${k}=${e.params[k]} not in the parameter list`);
    }
    if (!rig.params.skin[e.params.skin]) add(`${e.id}: unknown skin`);
    if (!rig.params.hair_color[e.params.hair_color]) add(`${e.id}: unknown hair colour`);
    if (!rig.params.outfit[e.params.outfit]) add(`${e.id}: unknown outfit`);
  }
  // every parameter value has shapes (so a future character can use it)
  for (const style of rig.params.hair) if (!rig.variants.hair[style] || !rig.variants.hair_back[style]) add(`hair style ${style} has no variants`);
  for (const [name, o] of Object.entries(rig.params.outfit)) {
    for (const [slot, key] of [['top_det', o.top], ['sleeve_u_l', o.top], ['pelvis_det', o.bottom], ['leg_det_u_l', o.bottom], ['foot_l', o.shoes]]) if (!rig.variants[slot]?.[key]) add(`outfit ${name}: ${slot}/${key} missing`);
    for (const c of ['top', 'bottom', 'shoes']) if (!/^#[0-9a-f]{6}$/.test(o.colors[c])) add(`outfit ${name}: colour ${c}`);
  }

  // poses, gestures, cycles reference known bones, hands and views
  const checkBones = (where, bones) => { for (const [b, deg] of Object.entries(bones || {})) { if (!BONES.includes(b)) add(`${where}: unknown bone ${b}`); if (!Number.isFinite(deg)) add(`${where}: bad angle`); } };
  const checkHands = (where, hands) => { for (const [side, h] of Object.entries(hands || {})) if (!['l', 'r'].includes(side) || !['open', 'fist', 'point', 'hold'].includes(h)) add(`${where}: bad hands`); };
  for (const e of entries) {
    if (e.kind === 'pose') { checkBones(e.id, e.bones); checkHands(e.id, e.hands); if (e.view && !VIEWS.includes(e.view)) add(`${e.id}: bad view`); }
    if (e.kind === 'gesture' || e.kind === 'cycle') {
      if (!(e.keys[0].t === 0 && e.keys[e.keys.length - 1].t === 1)) add(`${e.id}: keys must run from t=0 to t=1`);
      e.keys.forEach((k, i) => { checkBones(`${e.id} key ${i}`, k.bones); checkHands(`${e.id} key ${i}`, k.hands); if (i && k.t < e.keys[i - 1].t) add(`${e.id}: keys out of order`); });
      if (e.kind === 'gesture') {
        const first = JSON.stringify(e.keys[0].bones);
        const last = JSON.stringify(e.keys[e.keys.length - 1].bones);
        if (first !== last) add(`${e.id}: a gesture must return to its first key`);
        if (!(e.duration > 0)) add(`${e.id}: duration`);
      } else if (!(e.period > 0)) add(`${e.id}: period`);
    }
    if (e.kind === 'expression') {
      if (!rig.variants.eyes[`round/${e.eyes}`] || !rig.variants.brows[`soft/${e.brows}`] || !rig.variants.mouth[e.mouth]) add(`${e.id}: eyes/brows/mouth variant missing in the rig`);
    }
    if (e.kind === 'mouth' && !rig.variants.mouth[e.shape]) add(`${e.id}: rig has no mouth ${e.shape}`);
  }

  // sfx: at least two takes, each a copied WAV; file is the first take
  for (const e of entries.filter((x) => x.kind === 'sfx')) {
    if (e.thumb !== '') add(`${e.id}: an sfx entry has no thumbnail`);
    if (!Array.isArray(e.takes) || e.takes.length < 2) {
      add(`${e.id}: needs at least two takes`);
    } else {
      if (e.file !== e.takes[0].file) add(`${e.id}: file must equal takes[0].file`);
      for (const t of e.takes) if (!Buffer.isBuffer(files.get(t.file))) add(`${e.id}: missing take ${t.file}`);
    }
    if (e.align !== 'peak' && e.align !== 'onset') add(`${e.id}: align must be peak or onset`);
    if (!e.source || !e.source.kind || !e.source.basis) add(`${e.id}: source needs kind and basis`);
  }

  // props: palette tokens only
  for (const e of entries.filter((x) => x.kind === 'prop')) {
    const f = JSON.parse(files.get(e.file));
    shapeProblems(e.file, f.shapes, new Set(PALETTE_TOKENS), out);
    if (!(e.size.h > 0 && e.size.h < 1)) add(`${e.id}: size.h`);
    if (f.viewbox.length !== 4 || !(f.viewbox[2] > 0 && f.viewbox[3] > 0)) add(`${e.file}: viewbox`);
  }
  // environments
  const BASE = ['far-left', 'left', 'center', 'right', 'far-right'];
  const EXTRA = { 'env.plain-stage': ['board'], 'env.office': ['desk', 'door', 'board', 'window'], 'env.home-office': ['desk', 'door', 'window'], 'env.abstract-tech': ['board'] };
  // a 9:16 or 1:1 output shows a vertical slice of the 16:9 design frame, as wide as the output is at the design height
  const cropHalf = { '9:16': (100 * ((1080 * 9) / 16)) / 1920 / 2, '1:1': (100 * 1080) / 1920 / 2 };
  for (const e of entries.filter((x) => x.kind === 'environment')) {
    const f = JSON.parse(files.get(e.file));
    for (const v of ['day', 'night']) if (!f.tokens[v]) add(`${e.file}: tokens.${v} missing`);
    const dayKeys = Object.keys(f.tokens.day);
    const nightKeys = Object.keys(f.tokens.night);
    const known = new Set(PALETTE_TOKENS);
    for (const k of dayKeys) known.add(k);
    for (const k of nightKeys) if (!PALETTE_TOKENS.includes(k) && !dayKeys.includes(k)) add(`${e.file}: night token ${k} has no day value`);
    for (const k of dayKeys) if (!nightKeys.includes(k)) add(`${e.file}: day token ${k} has no night value`);
    for (const l of f.layers) {
      if (!(l.parallax >= 0 && l.parallax <= 1)) add(`${e.file}: layer ${l.name} parallax`);
      shapeProblems(`${e.file} layer ${l.name}`, l.shapes, known, out);
    }
    for (const s of [...BASE, ...EXTRA[e.id]]) if (!e.spots[s]) add(`${e.id}: spot ${s} missing`);
    for (const [name, sp] of Object.entries(e.spots)) if (!(sp.x >= 0 && sp.x <= 100 && sp.y >= 0 && sp.y <= 100)) add(`${e.id}: spot ${name} out of range`);
    const fl = e.floor;
    if (!(fl.y_far < fl.y_near && fl.scale_far < fl.scale_near)) add(`${e.id}: floor`);
    if (JSON.stringify(e.variants) !== '["day","night"]') add(`${e.id}: variants`);
    for (const asp of ['9:16', '1:1']) {
      const a = e.aspects?.[asp];
      if (!a || typeof a.focus_x !== 'number') { add(`${e.id}: aspect ${asp} override missing`); continue; }
      const half = cropHalf[asp];
      for (const name of Object.keys(e.spots)) {
        const x = (a.spots?.[name] || e.spots[name]).x;
        if (x < a.focus_x - half + 3 || x > a.focus_x + half - 3) add(`${e.id}: spot ${name} (x=${x}) lies outside the ${asp} crop around ${a.focus_x}`);
      }
    }
  }
  // thumbnails are svg and parse as path data
  for (const [p, text] of files) {
    if (!p.startsWith('thumbs/')) continue;
    if (!text.startsWith('<svg ') || /<script|<image|href=/i.test(text)) add(`${p}: not a plain svg`);
    for (const m of text.matchAll(/ d="([^"]*)"/g)) { const r = parsePath(m[1]); if (!r.ok) { add(`${p}: bad path: ${r.error}`); break; } }
  }
  // names
  for (const [id, row] of Object.entries(NAMES)) {
    if (row.length !== LANGS.length || row.some((s) => typeof s !== 'string' || !s.trim())) add(`names ${id}: needs ${LANGS.length} non-empty strings`);
    if (!ids.has(id) && !manifest.palettes.some((p) => p.id === id)) add(`names ${id}: no such entry`);
  }
  LANGS.forEach((lang, i) => {
    if (lang === 'en') return;
    const same = Object.values(NAMES).filter((row) => row[i] === row[LANGS.indexOf('en')]).length;
    if (same > Object.keys(NAMES).length * 0.35) add(`names ${lang}: ${same} of ${Object.keys(NAMES).length} are copies of the English name`);
  });
  // fallback coverage
  if (manifest.glyphs?.file !== 'glyphs.json') add('the manifest must name glyphs.json');
  const glyphText = files.get('glyphs.json');
  if (typeof glyphText !== 'string') add('glyphs.json is missing');
  else glyphProblems(JSON.parse(glyphText), add);
  return out;
}
