// Assembles the catalog (storymaker-catalog/1): manifest entries, rig, props, environments, thumbnails.
// buildOutputs() returns a Map of repo-relative output paths (below ui/img/storymaker/) to file contents.

import crypto from 'node:crypto';
import { buildRig } from './rig.mjs';
import { PROPS } from './props.mjs';
import { ENVIRONMENTS, ENV_META } from './environments.mjs';
import { POSES, GESTURES, CYCLES } from './motion.mjs';
import { nameKey } from './names.mjs';
import { Ctx, svgDoc, placedCharacter, propSvg, environmentSvg, evalKeys, shapesSvg } from './svg.mjs';
import { roundRect, fill, line, poly, circle } from './util.mjs';
import { buildGlyphs, glyphsText } from './glyphs.mjs';

export const STAGE = { width: 1920, height: 1080, safe: { x: 5, y: 5, w: 90, h: 90 } };

export const PALETTES = {
  sunny: { bg: '#fff7e6', bg2: '#ffe0a3', fg: '#1f2a44', accent: '#ff7a45', accent2: '#36cfc9', text: '#1f2a44', outline: '#1f2a44' },
  calm: { bg: '#eaf4f4', bg2: '#cfe7ef', fg: '#1b3a4b', accent: '#3a7ca5', accent2: '#81c3d7', text: '#16425b', outline: '#16425b' },
};
const THUMB_PALETTE = PALETTES.sunny;

export const FONTS = [
  { name: 'default', family: 'Geist', weight: 400, file: '/fonts/Geist-Regular.woff2', licence: 'OFL-1.1' },
  { name: 'default', family: 'Geist', weight: 500, file: '/fonts/Geist-Medium.woff2', licence: 'OFL-1.1' },
  { name: 'default', family: 'Geist', weight: 600, file: '/fonts/Geist-SemiBold.woff2', licence: 'OFL-1.1' },
  { name: 'default', family: 'Geist', weight: 700, file: '/fonts/Geist-Bold.woff2', licence: 'OFL-1.1' },
];

export const CHARACTERS = {
  'office-woman-1': {
    slug: 'anna', tags: ['woman', 'office', 'business', 'employee', 'presenter'],
    params: { body: 'average', skin: 'medium', hair: 'bun', hair_color: 'dark-brown', eyes: 'round', brows: 'soft', facial_hair: 'none', glasses: 'none', outfit: 'business', accessory: 'none' },
  },
  'office-man-1': {
    slug: 'ben', tags: ['man', 'office', 'business', 'suit', 'manager'],
    params: { body: 'broad', skin: 'light', hair: 'side-part', hair_color: 'chestnut', eyes: 'almond', brows: 'bold', facial_hair: 'stubble', glasses: 'none', outfit: 'suit', accessory: 'none' },
  },
  'it-tech-1': {
    slug: 'ida', tags: ['it', 'tech', 'support', 'admin', 'headset', 'glasses'],
    params: { body: 'slim', skin: 'brown', hair: 'messy', hair_color: 'black', eyes: 'big', brows: 'thin', facial_hair: 'none', glasses: 'square', outfit: 'it', accessory: 'headset' },
  },
  'senior-man-1': {
    slug: 'otto', tags: ['senior', 'elderly', 'man', 'retired', 'grandfather'],
    params: { body: 'elder', skin: 'tan', hair: 'balding', hair_color: 'white', eyes: 'almond', brows: 'soft', facial_hair: 'mustache', glasses: 'round', outfit: 'cardigan', accessory: 'none' },
  },
};

// expression -> eye, brow and mouth variants of the rig
export const EXPRESSIONS = {
  neutral: { eyes: 'neutral', brows: 'neutral', mouth: 'rest', tags: ['neutral', 'calm'] },
  happy: { eyes: 'smile', brows: 'neutral', mouth: 'smile', tags: ['happy', 'smile', 'pleased'] },
  laugh: { eyes: 'laugh', brows: 'raised', mouth: 'laugh', tags: ['laugh', 'joy', 'funny'] },
  sad: { eyes: 'sad', brows: 'sad', mouth: 'frown', tags: ['sad', 'unhappy', 'cry'] },
  angry: { eyes: 'angry', brows: 'angry', mouth: 'grit', tags: ['angry', 'mad', 'furious'] },
  surprised: { eyes: 'wide', brows: 'raised', mouth: 'o', tags: ['surprised', 'shock', 'amazed'] },
  confused: { eyes: 'neutral', brows: 'confused', mouth: 'squiggle', tags: ['confused', 'puzzled', 'unsure'] },
  thinking: { eyes: 'lookup', brows: 'think', mouth: 'side', tags: ['thinking', 'wonder', 'ponder'] },
  worried: { eyes: 'worried', brows: 'worried', mouth: 'wobble', tags: ['worried', 'anxious', 'nervous'] },
  determined: { eyes: 'narrow', brows: 'determined', mouth: 'firm', tags: ['determined', 'focused', 'resolute'] },
};

export const MOUTHS = ['rest', 'ai', 'e', 'o', 'u', 'mbp', 'fv'];

const POSE_TAGS = {
  stand: ['stand', 'idle'], sit: ['sit', 'seated', 'chair'], typing: ['typing', 'computer', 'sit', 'keyboard'],
  'point-left': ['point', 'left', 'show'], 'point-right': ['point', 'right', 'show'], wave: ['wave', 'hello', 'greet'],
  think: ['think', 'ponder', 'chin'], present: ['present', 'offer', 'show'], 'arms-crossed': ['arms crossed', 'skeptical', 'wait'],
  'hands-on-hips': ['hands on hips', 'confident', 'proud'],
};
const GESTURE_TAGS = {
  point: ['point', 'show'], wave: ['wave', 'hello'], shrug: ['shrug', 'unsure', 'dont know'], think: ['think', 'ponder'],
  present: ['present', 'offer'], 'thumbs-up': ['thumbs up', 'approve', 'good'], celebrate: ['celebrate', 'cheer', 'success'],
  facepalm: ['facepalm', 'oops', 'mistake'], count: ['count', 'number', 'list'],
};

const TEXT_STYLES = {
  title: { font: 'default', size: 0.07, weight: 700, color: '$text', align: 'center', max_lines: 2, tags: ['title', 'heading', 'headline'] },
  caption: { font: 'default', size: 0.042, weight: 500, color: '#ffffff', box: { fill: '#1f2a44d9', radius: 0.012, pad: 0.012 }, align: 'center', max_lines: 2, tags: ['caption', 'subtitle', 'lower third'] },
  label: { font: 'default', size: 0.034, weight: 600, color: '$text', box: { fill: '$bg2', radius: 0.02, pad: 0.01 }, align: 'center', max_lines: 2, tags: ['label', 'tag', 'small text'] },
  speech: { font: 'default', size: 0.042, weight: 500, color: '#1f2a44', box: { fill: '#ffffff', radius: 0.02, pad: 0.014 }, align: 'center', max_lines: 4, tags: ['speech', 'bubble', 'dialogue'] },
  thought: { font: 'default', size: 0.042, weight: 500, color: '#1f2a44', box: { fill: '#ffffff', radius: 0.04, pad: 0.014 }, align: 'center', max_lines: 4, tags: ['thought', 'bubble', 'inner voice'] },
  subtitle: { font: 'default', size: 0.042, weight: 500, color: '#ffffff', box: { fill: '#1f2a44d9', radius: 0.012, pad: 0.012 }, align: 'center', max_lines: 2, tags: ['subtitle', 'burned captions'] },
};

const TEXT_USAGE = {
  title: { do: 'show', text: 'Hallo', at: 'center' },
  speech: { do: 'say', who: 'anna', text: 'Hallo', style: 'speech' },
  thought: { do: 'say', who: 'anna', text: 'Hallo', style: 'thought' },
  subtitle: {},
};

const TRANSITIONS = { cut: { duration: 0, tags: ['cut', 'instant'] }, fade: { duration: 0.5, tags: ['fade', 'dissolve'] }, 'slide-left': { duration: 0.5, tags: ['slide', 'push'] }, wipe: { duration: 0.5, tags: ['wipe', 'reveal'] } };

const ENV_TAGS = {
  'plain-stage': ['plain', 'stage', 'neutral', 'gradient'],
  office: ['office', 'work', 'desk', 'whiteboard', 'workplace'],
  'home-office': ['home', 'office', 'remote', 'desk', 'bookshelf'],
  'abstract-tech': ['tech', 'abstract', 'network', 'digital', 'cyber'],
};

function entry(id, kind, name, tags, usage, extra) {
  return { id, kind, name, name_key: nameKey(id), tags, thumb: `thumbs/${id}.svg`, usage_example: usage, ...extra };
}

/** All manifest entries in a stable order. */
export function buildEntries() {
  const out = [];
  for (const [n, c] of Object.entries(CHARACTERS)) {
    const id = `char.${n}`;
    out.push(entry(id, 'character', id, c.tags, { cast: [{ id: c.slug, character: id }] }, { params: c.params }));
  }
  for (const [n, p] of Object.entries(PROPS)) {
    const id = `prop.${n}`;
    const usage = { do: 'show', item: id, at: 'right', entry: n === 'shield' || n === 'cloud' ? 'fade' : 'pop' };
    const extra = { file: `props/${n}.json`, size: { h: p.h }, anchor: { x: 0.5, y: 1 } };
    if (p.hold) extra.hold = true;
    out.push(entry(id, 'prop', id, p.tags, usage, extra));
  }
  for (const n of Object.keys(ENVIRONMENTS)) {
    const id = `env.${n}`;
    const m = ENV_META[n];
    out.push(entry(id, 'environment', id, ENV_TAGS[n], { environment: id }, { file: `environments/${n}.json`, spots: m.spots, floor: m.floor, variants: ['day', 'night'], aspects: m.aspects }));
  }
  for (const [n, p] of Object.entries(POSES)) {
    const extra = { bones: p.bones };
    if (p.hands) extra.hands = p.hands;
    if (p.view) extra.view = p.view;
    if (p.seated) extra.seated = true;
    if (p.offset_y) extra.offset_y = p.offset_y;
    out.push(entry(`pose.${n}`, 'pose', n, POSE_TAGS[n], { pose: n }, extra));
  }
  for (const [n, g] of Object.entries(GESTURES)) {
    out.push(entry(`gesture.${n}`, 'gesture', n, GESTURE_TAGS[n], { do: 'gesture', who: 'anna', gesture: n }, { duration: g.duration, keys: g.keys }));
  }
  for (const [n, e] of Object.entries(EXPRESSIONS)) {
    out.push(entry(`expr.${n}`, 'expression', n, e.tags, n === 'neutral' ? { expression: n } : { do: 'emote', who: 'anna', expression: n }, { eyes: e.eyes, brows: e.brows, mouth: e.mouth }));
  }
  for (const m of MOUTHS) out.push(entry(`mouth.${m}`, 'mouth', m, ['mouth', 'lip sync', m], {}, { shape: m }));
  out.push(entry('cycle.walk', 'cycle', 'walk', ['walk', 'move'], { do: 'walk', who: 'anna', to: 'center' }, { period: CYCLES.walk.period, speed: CYCLES.walk.speed, keys: CYCLES.walk.keys }));
  out.push(entry('cycle.idle', 'cycle', 'idle', ['idle', 'breathing'], {}, { period: CYCLES.idle.period, keys: CYCLES.idle.keys }));
  for (const [n, t] of Object.entries(TEXT_STYLES)) {
    const { tags, ...rest } = t;
    const usage = TEXT_USAGE[n] || { do: 'show', text: 'Hallo', at: 'center', style: n };
    out.push(entry(`text.${n}`, 'text_style', n, tags, usage, rest));
  }
  for (const [n, t] of Object.entries(TRANSITIONS)) out.push(entry(`trans.${n}`, 'transition', n, t.tags, { transition: n }, { duration: t.duration }));
  return out;
}

export function buildPalettes() {
  return Object.entries(PALETTES).map(([name, tokens]) => ({ id: `palette.${name}`, name, name_key: nameKey(`palette.${name}`), tokens }));
}

// ---------------------------------------------------------------- thumbnails
const REF = CHARACTERS['office-woman-1'].params;

function charThumb(rig, params, opts, viewBox, size) {
  const ctx = new Ctx();
  const g = placedCharacter(rig, params, THUMB_PALETTE, ctx, { x: 0, y: 0, scale: 1, facing: 'right', ...opts });
  return svgDoc(viewBox, g, ctx, { width: size[0], height: size[1] });
}

function textThumb(name) {
  const ctx = new Ctx();
  const maps = [THUMB_PALETTE];
  let shapes;
  if (name === 'title') {
    shapes = [fill(roundRect(0, 0, 320, 180, 10), '$bg'), fill(roundRect(52, 60, 216, 26, 8), '$text'), fill(roundRect(92, 100, 136, 18, 6), '$accent')];
  } else if (name === 'speech') {
    shapes = [fill(roundRect(0, 0, 320, 180, 10), '$bg'), fill(roundRect(52, 34, 216, 84, 18), '#ffffff'), fill(poly([[116, 116], [156, 116], [104, 152]]), '#ffffff'), fill(roundRect(78, 62, 164, 10, 5), '#1f2a44'), fill(roundRect(78, 84, 108, 10, 5), '#1f2a44', 0.8)];
  } else if (name === 'thought') {
    shapes = [fill(roundRect(0, 0, 320, 180, 10), '$bg'), fill(roundRect(52, 30, 216, 84, 36), '#ffffff'), fill(circle(124, 126, 12), '#ffffff'), fill(circle(106, 146, 8), '#ffffff'), fill(circle(94, 162, 5), '#ffffff'), fill(roundRect(82, 58, 156, 10, 5), '#1f2a44'), fill(roundRect(82, 80, 100, 10, 5), '#1f2a44', 0.8)];
  } else if (name === 'subtitle') {
    shapes = [fill(roundRect(0, 0, 320, 180, 10), '$bg'), fill(circle(86, 62, 30), '$accent2', 0.5), fill(roundRect(36, 126, 248, 36, 8), '#1f2a44', 0.85), fill(roundRect(58, 140, 204, 8, 4), '#ffffff')];
  } else if (name === 'caption') {
    shapes = [fill(roundRect(0, 0, 320, 180, 10), '$bg'), fill(circle(250, 60, 34), '$accent2', 0.5), fill(roundRect(44, 112, 232, 44, 10), '#1f2a44', 0.85), fill(roundRect(64, 124, 192, 8, 4), '#ffffff'), fill(roundRect(94, 140, 132, 8, 4), '#ffffff', 0.8)];
  } else {
    shapes = [fill(roundRect(0, 0, 320, 180, 10), '$bg'), fill(roundRect(96, 62, 128, 40, 20), '$bg2'), fill(roundRect(114, 76, 92, 12, 6), '$text'), line('M160 104L160 140', '$accent', 4), fill(circle(160, 146, 8), '$accent')];
  }
  return svgDoc([0, 0, 320, 180], shapesSvg(shapes, maps, ctx), ctx, { width: 320, height: 180 });
}

function transThumb(name) {
  const ctx = new Ctx();
  const maps = [THUMB_PALETTE];
  const A = '$accent2';
  const B = '$accent';
  let shapes;
  if (name === 'cut') {
    shapes = [fill(roundRect(0, 0, 320, 180, 10), A), fill('M190 0L310 0A10 10 0 0 1 320 10L320 170A10 10 0 0 1 310 180L130 180Z', B), line('M190 0L130 180', '#ffffff', 5)];
  } else if (name === 'fade') {
    shapes = [{ d: roundRect(0, 0, 320, 180, 10), gradient: { type: 'linear', stops: [[0, A], [1, B]], x1: 0, y1: 0, x2: 320, y2: 0 } }];
  } else if (name === 'slide-left') {
    shapes = [fill(roundRect(0, 0, 320, 180, 10), A), fill('M150 0L310 0A10 10 0 0 1 320 10L320 170A10 10 0 0 1 310 180L150 180Z', B), line('M110 90L50 90M74 66L50 90L74 114', '#ffffff', 8)];
  } else {
    shapes = [fill(roundRect(0, 0, 320, 180, 10), A), fill('M170 0L310 0A10 10 0 0 1 320 10L320 170A10 10 0 0 1 310 180L170 180Z', B), fill(poly([[160, 0], [184, 0], [184, 180], [160, 180]]), '#ffffff', 0.85), line('M138 90L84 90M106 68L84 90L106 112', '#ffffff', 8)];
  }
  return svgDoc([0, 0, 320, 180], shapesSvg(shapes, maps, ctx), ctx, { width: 320, height: 180 });
}

/** One SVG per entry id. */
export function buildThumbs(rig, entries) {
  const out = new Map();
  for (const e of entries) {
    const id = e.id;
    let svg;
    switch (e.kind) {
      case 'character':
        svg = charThumb(rig, e.params, { view: 'three_quarter' }, [-200, -1025, 400, 575], [220, 316]);
        break;
      case 'prop': {
        const p = PROPS[id.slice(5)].build();
        const ctx = new Ctx();
        const inner = propSvg(p, THUMB_PALETTE, ctx);
        const [, , w, h] = p.viewbox;
        svg = svgDoc(p.viewbox, inner, ctx, { width: Math.round((160 * w) / h), height: 160 });
        break;
      }
      case 'environment': {
        const env = ENVIRONMENTS[id.slice(4)]();
        const ctx = new Ctx();
        const inner = environmentSvg(env, 'day', THUMB_PALETTE, ctx);
        svg = svgDoc([0, 0, 1920, 1080], inner, ctx, { width: 320, height: 180 });
        break;
      }
      case 'pose': {
        const p = POSES[e.name];
        // seated poses are cropped lower, standing ones from the head to the knees
        const drop = p.offset_y || 0;
        const h = drop ? 1075 - drop : 720;
        svg = charThumb(rig, REF, { view: p.view || 'three_quarter', pose: p, offset_y: drop }, [-260, -1025 + drop, 520, h], [200, Math.round((200 * h) / 520)]);
        break;
      }
      case 'gesture': {
        const g = GESTURES[e.name];
        const peak = g.keys.reduce((a, b) => (Object.keys(b.bones).length > Object.keys(a.bones).length ? b : a), g.keys[0]);
        svg = charThumb(rig, REF, { view: 'three_quarter', pose: POSES.stand, deltas: peak.bones, hands: peak.hands || {} }, [-260, -1025, 520, 720], [200, 277]);
        break;
      }
      case 'expression': {
        svg = charThumb(rig, REF, { view: 'front', expression: { eyes: e.eyes, brows: e.brows, mouth: e.mouth } }, [-135, -1010, 270, 270], [180, 180]);
        break;
      }
      case 'mouth':
        svg = charThumb(rig, REF, { view: 'front', mouth: e.shape }, [-110, -895, 220, 190], [180, 155]);
        break;
      case 'cycle': {
        if (e.name === 'walk') {
          const k = evalKeys(e.keys, 0);
          svg = charThumb(rig, REF, { view: 'profile', pose: POSES.stand, deltas: k.bones, offset_y: k.offset_y }, [-260, -1025, 520, 1075], [200, 413]);
        } else {
          svg = charThumb(rig, REF, { view: 'three_quarter', pose: POSES.stand }, [-260, -1025, 520, 1075], [200, 413]);
        }
        break;
      }
      case 'text_style':
        svg = textThumb(e.name);
        break;
      case 'transition':
        svg = transThumb(e.name);
        break;
      default:
        throw new Error(`no thumbnail for ${e.kind}`);
    }
    out.set(`thumbs/${id}.svg`, svg);
  }
  return out;
}

// ---------------------------------------------------------------- manifest and hashing
const lf = (s) => s.replace(/\r\n/g, '\n');

/** True when the bytes on disk match an expected output: a Buffer byte for byte, text after LF normalisation. */
export function sameOutput(onDisk, expected) {
  return Buffer.isBuffer(expected) ? onDisk.equals(expected) : lf(onDisk.toString('utf8')) === expected;
}

/** sha256 over every output except catalog.json in sorted path order, text LF-normalised and Buffers raw, first 12 hex. */
export function versionOf(files) {
  const h = crypto.createHash('sha256');
  for (const p of [...files.keys()].filter((k) => k !== 'catalog.json').sort()) {
    const content = files.get(p);
    h.update(Buffer.isBuffer(content) ? content : Buffer.from(lf(content), 'utf8'));
  }
  return h.digest('hex').slice(0, 12);
}

function manifestText(manifest) {
  const lines = [];
  const j = (v) => JSON.stringify(v);
  lines.push('{' + [`"schema":${j(manifest.schema)}`, `"version":${j(manifest.version)}`, `"stage":${j(manifest.stage)}`, `"rig":${j(manifest.rig)}`, `"glyphs":${j(manifest.glyphs)}`].join(',') + ',');
  lines.push(` "fonts":[\n${manifest.fonts.map((f) => '  ' + j(f)).join(',\n')}\n ],`);
  lines.push(` "palettes":[\n${manifest.palettes.map((p) => '  ' + j(p)).join(',\n')}\n ],`);
  lines.push(` "entries":[\n${manifest.entries.map((e) => '  ' + j(e)).join(',\n')}\n ]}`);
  return lines.join('\n') + '\n';
}

/** Every output file below ui/img/storymaker/ as a Map(relative path -> text). */
export function buildOutputs() {
  const rig = buildRig();
  const entries = buildEntries();
  const files = new Map();
  files.set('rig/human.json', JSON.stringify(rig) + '\n');
  for (const [n, p] of Object.entries(PROPS)) {
    const b = p.build();
    files.set(`props/${n}.json`, JSON.stringify({ viewbox: b.viewbox, shapes: b.shapes }) + '\n');
  }
  for (const [n, f] of Object.entries(ENVIRONMENTS)) {
    const e = f();
    files.set(`environments/${n}.json`, JSON.stringify({ viewbox: [0, 0, 1920, 1080], layers: e.layers, tokens: e.tokens }) + '\n');
  }
  for (const [p, svg] of buildThumbs(rig, entries)) files.set(p, svg);
  files.set('glyphs.json', glyphsText(buildGlyphs()));
  const manifest = {
    schema: 'storymaker-catalog/1',
    version: versionOf(files),
    stage: STAGE,
    rig: { id: 'rig.human/1', file: 'rig/human.json' },
    glyphs: { file: 'glyphs.json' },
    fonts: FONTS,
    palettes: buildPalettes(),
    entries,
  };
  files.set('catalog.json', manifestText(manifest));
  return { files, manifest, rig, entries };
}

