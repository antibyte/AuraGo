// Review sheets: the cast, the parameter variety, props and environments rendered with the section 3a renderer
// (svg.mjs). Used by `build-storymaker-catalog.mjs --review <dir>`; the SVG sheets are screenshotted with Chrome.

import { Ctx, placedCharacter, svgDoc, evalKeys, propSvg, environmentSvg } from './svg.mjs';
import { CHARACTERS, EXPRESSIONS, PALETTES } from './catalog.mjs';
import { POSES, GESTURES, CYCLES } from './motion.mjs';
import { PROPS } from './props.mjs';
import { ENVIRONMENTS, ENV_META } from './environments.mjs';
import { HAIR_STYLES } from './hair.mjs';
import { FACIAL_HAIR, GLASSES, ACCESSORIES } from './extras.mjs';
import { OUTFITS } from './clothes.mjs';
import { BODY_NAMES } from './dims.mjs';

const PAL = PALETTES.sunny;
const label = (x, y, text, size = 26) => `<text x="${x}" y="${y}" font-family="sans-serif" font-size="${size}" fill="#1f2a44" stroke="#ffffff" stroke-width="${Math.max(2, size / 7)}" paint-order="stroke">${text}</text>`;
const cell = (x, y, w, h, viewBox, inner) => `<svg x="${x}" y="${y}" width="${w}" height="${h}" viewBox="${viewBox.join(' ')}">${inner}</svg>`;
const FULL = [-230, -1030, 460, 1090];
const HEAD = [-135, -1010, 270, 270];
const NEUTRAL = { eyes: 'neutral', brows: 'neutral', mouth: 'rest' };

function actor(rig, params, ctx, opts) {
  return placedCharacter(rig, params, PAL, ctx, { x: 0, y: 0, scale: 1, facing: 'right', expression: NEUTRAL, ...opts });
}

/** All starters in three views, expressions, poses, gestures and the walk cycle. */
export function castSheet(rig) {
  const ctx = new Ctx();
  let inner = '';
  let y = 0;
  const chars = Object.entries(CHARACTERS);
  // row 1: four characters in front / three quarter / profile
  const cw = 230;
  const ch = 545;
  chars.forEach(([name, c], ci) => {
    ['front', 'three_quarter', 'profile'].forEach((view, vi) => {
      inner += cell((ci * 3 + vi) * cw, y, cw, ch, [-230, -1030, 460, 1090], actor(rig, c.params, ctx, { view }));
    });
    inner += label(ci * 3 * cw + 8, y + 20, name, 20);
  });
  y += ch;
  const width = cw * 12;
  // rows 2-3: expressions on the four characters (front and three quarter)
  const exprNames = Object.keys(EXPRESSIONS);
  const ew = 176;
  const eh = 176;
  chars.forEach(([name, c], ci) => {
    exprNames.forEach((en, ei) => {
      const e = EXPRESSIONS[en];
      const x = (ei % 10) * ew + (ci % 2) * 0;
      const yy = y + (ci * 2) * eh;
      inner += cell(x, yy, ew, eh, HEAD, actor(rig, c.params, ctx, { view: 'front', expression: e }));
      inner += cell(x, yy + eh, ew, eh, HEAD, actor(rig, c.params, ctx, { view: 'three_quarter', expression: e }));
    });
  });
  y += chars.length * 2 * eh;
  // poses and gestures on the first character
  const pw = 230;
  const ph = 424;
  const cells = [];
  for (const [n, p] of Object.entries(POSES)) cells.push({ n, view: p.view || 'three_quarter', pose: p });
  for (const [n, g] of Object.entries(GESTURES)) {
    const peak = g.keys.reduce((a, b) => (Object.keys(b.bones).length > Object.keys(a.bones).length ? b : a), g.keys[0]);
    cells.push({ n: `gesture ${n}`, view: 'three_quarter', pose: POSES.stand, deltas: peak.bones, hands: peak.hands });
  }
  for (const t of [0, 0.25, 0.5, 0.75]) {
    const k = evalKeys(CYCLES.walk.keys, t);
    cells.push({ n: `walk ${t}`, view: 'profile', pose: POSES.stand, deltas: k.bones, offset_y: k.offset_y });
  }
  const per = Math.floor(width / pw);
  cells.forEach((c, i) => {
    const x = (i % per) * pw;
    const yy = y + Math.floor(i / per) * ph;
    inner += cell(x, yy + 24, pw, ph - 24, FULL, actor(rig, chars[0][1].params, ctx, { view: c.view, pose: c.pose, deltas: c.deltas, hands: c.hands, offset_y: c.offset_y }));
    inner += label(x + 6, yy + 18, `${c.n} (${c.view.replace('three_quarter', '3/4')})`, 17);
  });
  y += Math.ceil(cells.length / per) * ph;
  return { svg: svgDoc([0, 0, width, y], inner, ctx, { background: PAL.bg }), width, height: y };
}

/** Every value of the parameter lists on the reference character. */
export function varietySheet(rig) {
  const ctx = new Ctx();
  let inner = '';
  const base = CHARACTERS['office-woman-1'].params;
  const man = CHARACTERS['office-man-1'].params;
  let y = 0;
  const hw = 170;
  const hh = 170;
  const row = (title, items, make, view = 'three_quarter', box = HEAD, w = hw, h = hh, perRow = 12) => {
    inner += label(6, y + 24, title, 22);
    y += 30;
    items.forEach((it, i) => {
      const xx = (i % perRow) * w;
      const yy = y + Math.floor(i / perRow) * (h + 22);
      inner += cell(xx, yy, w, h, box, actor(rig, make(it), ctx, { view }));
      inner += label(xx + 4, yy + h + 16, it, 15);
    });
    y += Math.ceil(items.length / perRow) * (h + 22) + 8;
  };
  const width = hw * 12;
  row('hair (front)', HAIR_STYLES, (v) => ({ ...base, hair: v }), 'front');
  row('hair (three quarter)', HAIR_STYLES, (v) => ({ ...base, hair: v }), 'three_quarter');
  row('hair (profile)', HAIR_STYLES, (v) => ({ ...base, hair: v }), 'profile');
  row('hair colours and skin tones', ['black', 'dark-brown', 'brown', 'chestnut', 'blonde', 'red', 'grey', 'white'], (v) => ({ ...base, hair_color: v, hair: 'short' }), 'front');
  row('skin', ['porcelain', 'light', 'tan', 'olive', 'medium', 'brown', 'dark', 'deep'], (v) => ({ ...base, skin: v }), 'front');
  row('facial hair (front)', FACIAL_HAIR, (v) => ({ ...man, facial_hair: v }), 'front');
  row('facial hair (three quarter)', FACIAL_HAIR, (v) => ({ ...man, facial_hair: v }), 'three_quarter');
  row('facial hair (profile)', FACIAL_HAIR, (v) => ({ ...man, facial_hair: v }), 'profile');
  row('glasses', GLASSES, (v) => ({ ...base, glasses: v }), 'front');
  row('glasses (three quarter)', GLASSES, (v) => ({ ...base, glasses: v }), 'three_quarter');
  row('accessory (front)', ACCESSORIES, (v) => ({ ...base, hair: 'short', accessory: v }), 'front');
  row('accessory (three quarter)', ACCESSORIES, (v) => ({ ...base, hair: 'short', accessory: v }), 'three_quarter');
  row('accessory (profile)', ACCESSORIES, (v) => ({ ...base, hair: 'short', accessory: v }), 'profile');
  row('eyes / brows', ['round', 'almond', 'big'], (v) => ({ ...base, eyes: v, brows: v === 'round' ? 'soft' : v === 'almond' ? 'bold' : 'thin' }), 'front');
  // outfits and bodies are full-body rows
  const fw = 200;
  const fh = 480;
  inner += label(6, y + 24, 'outfits (three quarter, front)', 22);
  y += 30;
  Object.keys(OUTFITS).forEach((o, i) => {
    inner += cell(i * fw * 2, y, fw, fh, FULL, actor(rig, { ...base, outfit: o }, ctx, { view: 'three_quarter' }));
    inner += cell(i * fw * 2 + fw, y, fw, fh, FULL, actor(rig, { ...base, outfit: o }, ctx, { view: 'front' }));
    inner += label(i * fw * 2 + 4, y + fh + 16, o, 15);
  });
  y += fh + 30;
  inner += label(6, y + 24, 'bodies (front, three quarter, profile), outfit casual', 22);
  y += 30;
  BODY_NAMES.forEach((b, i) => {
    ['front', 'three_quarter', 'profile'].forEach((view, vi) => {
      inner += cell(i * fw * 2 + vi * (fw * 0.62), y, fw * 0.62, fh * 0.8, FULL, actor(rig, { ...base, body: b, outfit: 'casual' }, ctx, { view }));
    });
    inner += label(i * fw * 2 + 4, y + fh * 0.8 + 16, b, 15);
  });
  y += fh * 0.8 + 30;
  return { svg: svgDoc([0, 0, width, y], inner, ctx, { background: PAL.bg }), width, height: y };
}

const depthScale = (meta, y) => {
  const f = meta.floor;
  return f.scale_far + ((y - f.y_far) / (f.y_near - f.y_far)) * (f.scale_near - f.scale_far);
};

/** Props on both palettes, then the environments in day and night with actors standing on the spots. */
export function envPropSheet(rig) {
  const ctx = new Ctx();
  let inner = '';
  let y = 0;
  const names = Object.keys(PROPS);
  for (const pname of ['sunny', 'calm']) {
    const pal = PALETTES[pname];
    inner += `<rect x="0" y="${y}" width="1920" height="${Math.ceil(names.length / 8) * 250}" fill="${pal.bg}"/>`;
    names.forEach((n, i) => {
      const p = PROPS[n].build();
      const [, , w, h] = p.viewbox;
      const s = Math.min(200 / w, 200 / h);
      const x = (i % 8) * 240 + 20;
      const yy = y + Math.floor(i / 8) * 250 + 10;
      inner += `<g transform="translate(${x + (200 - w * s) / 2} ${yy + 200 - h * s}) scale(${s})">${propSvg(p, pal, ctx)}</g>${label(x, yy + 226, n, 16)}`;
    });
    y += Math.ceil(names.length / 8) * 250;
  }
  const ew = 960;
  const eh = 540;
  let i = 0;
  const envNames = Object.keys(ENVIRONMENTS);
  for (const pname of ['sunny', 'calm']) {
    const pal = PALETTES[pname];
    for (const variant of ['day', 'night']) {
      for (const n of envNames) {
        const env = ENVIRONMENTS[n]();
        const meta = ENV_META[n];
        let g = environmentSvg(env, variant, pal, ctx);
        const place = (spot, ch, pose, facing) => {
          const sp = meta.spots[spot];
          const sc = (rig.height * 1080 * depthScale(meta, sp.y)) / 1000;
          return placedCharacter(rig, CHARACTERS[ch].params, pal, ctx, { view: 'three_quarter', x: (sp.x / 100) * 1920, y: (sp.y / 100) * 1080, scale: sc, facing, pose, offset_y: pose.offset_y || 0, expression: NEUTRAL });
        };
        g += place(meta.spots.desk ? 'desk' : 'left', 'office-woman-1', meta.spots.desk ? POSES.typing : POSES.stand, 'right');
        g += place('center', 'office-man-1', POSES['point-right'], 'right');
        g += place('right', 'it-tech-1', POSES.stand, 'left');
        g += place('far-right', 'senior-man-1', POSES.present, 'left');
        inner += cell((i % 2) * ew, y + Math.floor(i / 2) * eh, ew, eh, [0, 0, 1920, 1080], g);
        inner += label((i % 2) * ew + 10, y + Math.floor(i / 2) * eh + 30, `${n} ${variant} ${pname}`, 26);
        i++;
      }
    }
  }
  y += Math.ceil(i / 2) * eh;
  return { svg: svgDoc([0, 0, 1920, y], inner, ctx, { background: '#888' }), width: 1920, height: y };
}
