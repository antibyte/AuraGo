// The four starter environments (1920 x 1080 design frame). Layers are drawn in array order; parallax runs from
// 0 (fixed sky) to 1 (floor). Day colours follow the palette where possible ($bg, $bg2, $accent, $accent2); the night
// tokens override bg/bg2, the text colour (light: the player draws texts with these tokens first) and the local tokens.
// Local tokens exist in both variants. The dark abstract-tech environment carries bg2 and text in both variants.

import {
  fill, shade, light, line, linear, radial, ellipse, circle, rect, roundRect, roundRect4, poly, M, L, C, Z, A, rng,
} from './util.mjs';

const W = 1920;
const H = 1080;

const BASE_SPOTS = {
  'far-left': { x: 12, y: 88 },
  left: { x: 30, y: 88 },
  center: { x: 50, y: 88 },
  right: { x: 70, y: 88 },
  'far-right': { x: 88, y: 88 },
};

/** Spots, floor and aspect overrides per environment (design frame percent; y is the feet line). */
export const ENV_META = {
  'plain-stage': {
    spots: { ...BASE_SPOTS, board: { x: 74, y: 62 } },
    floor: { y_near: 96, y_far: 70, scale_near: 1.05, scale_far: 0.8 },
    aspects: {
      '9:16': { focus_x: 50, spots: { 'far-left': { x: 38, y: 88 }, left: { x: 42, y: 88 }, right: { x: 58, y: 88 }, 'far-right': { x: 62, y: 88 }, board: { x: 56, y: 62 } } },
      '1:1': { focus_x: 50, spots: { 'far-left': { x: 25, y: 88 }, 'far-right': { x: 75, y: 88 }, board: { x: 68, y: 62 } } },
    },
  },
  office: {
    spots: { ...BASE_SPOTS, desk: { x: 36, y: 84, face: 'right' }, door: { x: 90, y: 82, face: 'left' }, board: { x: 70, y: 60 }, window: { x: 18, y: 82 } },
    floor: { y_near: 96, y_far: 72, scale_near: 1.05, scale_far: 0.82 },
    aspects: {
      '9:16': { focus_x: 44, spots: { 'far-left': { x: 33, y: 88 }, left: { x: 37, y: 88 }, center: { x: 44, y: 88 }, right: { x: 51, y: 88 }, 'far-right': { x: 55, y: 88 }, door: { x: 56, y: 82, face: 'left' }, board: { x: 56, y: 60 }, window: { x: 33, y: 82 } } },
      '1:1': { focus_x: 45, spots: { 'far-left': { x: 24, y: 88 }, center: { x: 45, y: 88 }, right: { x: 60, y: 88 }, 'far-right': { x: 66, y: 88 }, door: { x: 66, y: 82, face: 'left' }, board: { x: 62, y: 60 }, window: { x: 24, y: 82 } } },
    },
  },
  'home-office': {
    spots: { ...BASE_SPOTS, desk: { x: 36, y: 84, face: 'right' }, door: { x: 90, y: 82, face: 'left' }, window: { x: 14, y: 82 } },
    floor: { y_near: 96, y_far: 72, scale_near: 1.05, scale_far: 0.82 },
    aspects: {
      '9:16': { focus_x: 44, spots: { 'far-left': { x: 33, y: 88 }, left: { x: 37, y: 88 }, center: { x: 44, y: 88 }, right: { x: 51, y: 88 }, 'far-right': { x: 55, y: 88 }, door: { x: 56, y: 82, face: 'left' }, window: { x: 33, y: 82 } } },
      '1:1': { focus_x: 45, spots: { 'far-left': { x: 24, y: 88 }, center: { x: 45, y: 88 }, right: { x: 60, y: 88 }, 'far-right': { x: 66, y: 88 }, door: { x: 66, y: 82, face: 'left' }, window: { x: 24, y: 82 } } },
    },
  },
  'abstract-tech': {
    spots: { ...BASE_SPOTS, board: { x: 70, y: 58 } },
    floor: { y_near: 96, y_far: 72, scale_near: 1.05, scale_far: 0.82 },
    aspects: {
      '9:16': { focus_x: 50, spots: { 'far-left': { x: 38, y: 88 }, left: { x: 42, y: 88 }, right: { x: 58, y: 88 }, 'far-right': { x: 62, y: 88 }, board: { x: 56, y: 58 } } },
      '1:1': { focus_x: 50, spots: { 'far-left': { x: 25, y: 88 }, 'far-right': { x: 75, y: 88 }, board: { x: 68, y: 58 } } },
    },
  },
};

// ---------------------------------------------------------------- plain stage
function plainStage() {
  const horizon = 756;
  const sky = [
    linear(rect(0, 0, W, H), [[0, '$bg'], [0.7, '$bg2'], [1, '$bg2']], 0, 0, 0, horizon),
    fill(circle(300, 250, 230), '#ffffff', 0.3),
    fill(circle(1640, 190, 150), '$accent2', 0.12),
    fill(circle(1500, 420, 270), '#ffffff', 0.18),
    fill(circle(520, 520, 90), '$accent', 0.08),
    fill(circle(1180, 120, 34), '$accent', 0.16),
    fill(circle(150, 600, 52), '$accent2', 0.14),
  ];
  const r = rng(3);
  for (let i = 0; i < 18; i++) sky.push(fill(circle(80 + r() * 1760, 40 + r() * 220, 2 + r() * 3), '$star', 0.7));
  const back = [
    fill(`${M(0, 660)}${C(300, 600, 620, 640, 900, 664)}${C(1200, 690, 1500, 620, 1920, 650)}${L(1920, horizon)}${L(0, horizon)}${Z}`, '$bg2'),
    shade(`${M(0, 660)}${C(300, 600, 620, 640, 900, 664)}${C(1200, 690, 1500, 620, 1920, 650)}${L(1920, horizon)}${L(0, horizon)}${Z}`, 0.05),
    fill(`${M(0, 700)}${C(400, 660, 800, 700, 1200, 700)}${C(1500, 700, 1700, 670, 1920, 690)}${L(1920, horizon)}${L(0, horizon)}${Z}`, '$bg2'),
    shade(`${M(0, 700)}${C(400, 660, 800, 700, 1200, 700)}${C(1500, 700, 1700, 670, 1920, 690)}${L(1920, horizon)}${L(0, horizon)}${Z}`, 0.08),
  ];
  const floor = [
    fill(rect(0, horizon, W, H - horizon), '$bg2'),
    shade(rect(0, horizon, W, H - horizon), 0.07),
    fill(ellipse(960, 900, 760, 120), '#ffffff', 0.3),
    fill(ellipse(960, 904, 520, 76), '#ffffff', 0.22),
    shade(rect(0, horizon, W, 4), 0.1),
    shade(rect(0, 1010, W, 70), 0.06),
  ];
  return {
    layers: [{ name: 'sky', parallax: 0, shapes: sky }, { name: 'hills', parallax: 0.3, shapes: back }, { name: 'floor', parallax: 1, shapes: floor }],
    tokens: {
      day: { star: '#ffffff' },
      night: { bg: '#1b2142', bg2: '#2c356b', text: '#f2f4ff', star: '#dfe6ff' },
    },
  };
}

// ---------------------------------------------------------------- shared furniture for the two offices
function chair(x, color) {
  const s = [];
  s.push(shade(ellipse(x + 80, 905, 100, 12), 0.14));
  s.push(fill(roundRect(x - 12, 560, 34, 236, 16), color));
  s.push(shade(roundRect4(x + 6, 560, 16, 236, [0, 16, 16, 0]), 0.14));
  s.push(fill(roundRect(x - 12, 774, 164, 36, 16), color));
  s.push(shade(roundRect4(x - 12, 796, 164, 14, [0, 0, 16, 16]), 0.16));
  s.push(fill(rect(x + 60, 808, 16, 66), '#6b7690'));
  s.push(fill(poly([[x - 6, 902], [x + 60, 874], [x + 76, 874], [x + 146, 902], [x + 146, 912], [x - 6, 912]]), '#4b5573'));
  for (const dx of [0, 70, 140]) s.push(fill(circle(x + dx, 914, 10), '#2b3350'));
  return s;
}

function laptopSide(x, y) {
  return [
    fill(roundRect(x, y - 12, 160, 12, 5), '#c9d0dc'),
    shade(roundRect4(x, y - 5, 160, 5, [0, 0, 5, 5]), 0.2),
    fill(poly([[x + 146, y - 10], [x + 160, y - 10], [x + 196, y - 118], [x + 182, y - 122]]), '#9aa4b8'),
    fill(poly([[x + 148, y - 12], [x + 154, y - 12], [x + 188, y - 114], [x + 183, y - 116]]), '$accent2', 0.9),
    light(rect(x + 8, y - 11, 120, 3), 0.4),
  ];
}

function mug(x, y, color) {
  return [fill(roundRect(x, y - 40, 36, 40, 7), color), shade(roundRect4(x + 18, y - 40, 18, 40, [0, 7, 7, 0]), 0.14), line(`${M(x + 36, y - 30)}${C(x + 54, y - 30, x + 54, y - 10, x + 34, y - 10)}`, color, 7), fill(ellipse(x + 18, y - 40, 15, 4), '#5a3a2a')];
}

function deskLamp(x, y, light1) {
  const s = [];
  s.push(fill(ellipse(x, y, 30, 7), '#4b5573'));
  s.push(line(`${M(x, y - 4)}${L(x + 6, y - 90)}${L(x - 34, y - 140)}`, '#4b5573', 8));
  s.push(fill(`${M(x - 62, y - 126)}${L(x - 14, y - 160)}${L(x + 2, y - 128)}${L(x - 44, y - 106)}${Z}`, light1));
  s.push(fill(`${M(x - 44, y - 106)}${L(x - 120, y + 8)}${L(x + 10, y + 8)}${L(x + 2, y - 128)}${Z}`, '$lampglow', 0.16));
  return s;
}

function plantPot(x, y, s = 1) {
  const out = [];
  const leaves = [[-70, 170, '#2fa86a'], [64, 176, '#2fa86a'], [-32, 220, '#38c27d'], [30, 226, '#38c27d'], [0, 250, '#46d28a']];
  for (const [ang, len, color] of leaves) {
    const a = (ang * Math.PI) / 180;
    const dx = Math.sin(a) * len * s;
    const dy = -Math.cos(a) * len * s;
    const nx = -Math.cos(a) * len * 0.2 * s;
    const ny = -Math.sin(a) * len * 0.2 * s;
    out.push(fill(`${M(x, y - 70 * s)}${C(x + dx * 0.4 + nx, y - 70 * s + dy * 0.4 + ny, x + dx * 0.85 + nx, y - 70 * s + dy * 0.85 + ny, x + dx, y - 70 * s + dy)}${C(x + dx * 0.85 - nx, y - 70 * s + dy * 0.85 - ny, x + dx * 0.4 - nx, y - 70 * s + dy * 0.4 - ny, x, y - 70 * s)}${Z}`, color));
  }
  out.push(fill(`${M(x - 52 * s, y - 80 * s)}${L(x + 52 * s, y - 80 * s)}${L(x + 42 * s, y - 8 * s)}${L(x - 42 * s, y - 8 * s)}${Z}`, '$accent'));
  out.push(shade(`${M(x, y - 80 * s)}${L(x + 52 * s, y - 80 * s)}${L(x + 42 * s, y - 8 * s)}${L(x, y - 8 * s)}${Z}`, 0.14));
  return out;
}

function windowCity(x, y, w, h) {
  const s = [];
  s.push(fill(roundRect(x, y, w, h, 12), '$frame'));
  const gx = x + 20;
  const gy = y + 20;
  const gw = w - 40;
  const gh = h - 40;
  s.push(linear(rect(gx, gy, gw, gh), [[0, '$sky'], [1, '$sky2']], 0, gy, 0, gy + gh));
  s.push(fill(circle(gx + gw * 0.76, gy + 80, 34), '$orb'));
  const r = rng(11);
  let bx = gx;
  while (bx < gx + gw) {
    const bw = 40 + Math.floor(r() * 50);
    const bh = 90 + Math.floor(r() * 170);
    const width = Math.min(bw, gx + gw - bx);
    s.push(fill(rect(bx, gy + gh - bh, width, bh), r() > 0.5 ? '$city' : '$city2'));
    for (let wy = gy + gh - bh + 14; wy < gy + gh - 16; wy += 28) for (let wx = bx + 8; wx < bx + width - 12; wx += 22) if (r() > 0.45) s.push(fill(rect(wx, wy, 10, 14), '$lit'));
    bx += width + 4;
  }
  s.push(fill(rect(x + w / 2 - 4, gy, 8, gh), '$frame'));
  s.push(fill(rect(gx, y + h * 0.5 - 4, gw, 8), '$frame'));
  s.push(light(poly([[gx, gy], [gx + gw * 0.5, gy], [gx, gy + gh * 0.6]]), 0.12));
  s.push(fill(roundRect(x - 20, y + h - 4, w + 40, 20, 6), '$frame'));
  s.push(shade(roundRect4(x - 20, y + h + 8, w + 40, 8, [0, 0, 6, 6]), 0.16));
  return s;
}

function door(x, y, w, h) {
  return [
    fill(roundRect(x - 16, y - 16, w + 32, h + 16, 8), '$frame'),
    fill(rect(x, y, w, h), '$wood'),
    fill(roundRect(x + 22, y + 26, w - 44, h * 0.42, 6), '$wood2'),
    fill(roundRect(x + 22, y + h * 0.5, w - 44, h * 0.42, 6), '$wood2'),
    shade(rect(x + w * 0.55, y, w * 0.45, h), 0.08),
    fill(circle(x + w - 26, y + h * 0.5, 9), '#e8c25a'),
  ];
}

// ---------------------------------------------------------------- office
function office() {
  const base = 777;
  const wall = [
    fill(rect(0, 0, W, base), '$bg'),
    fill(rect(0, 640, W, base - 640), '$wall2'),
    fill(rect(0, 632, W, 10), '$wood'),
    fill(rect(0, base - 14, W, 14), '$wood2'),
    ...windowCity(150, 150, 400, 470),
    // framed pictures
    fill(roundRect(640, 220, 170, 130, 8), '$wood'),
    fill(roundRect(652, 232, 146, 106, 4), '$bg2'),
    fill(circle(700, 280, 22), '$accent'),
    fill(poly([[724, 322], [760, 262], [790, 322]]), '$accent2'),
    fill(roundRect(836, 250, 90, 100, 8), '$wood'),
    fill(roundRect(846, 260, 70, 80, 4), '$bg2'),
    fill(circle(881, 300, 18), '$accent2'),
    // clock above the door
    fill(circle(1536, 190, 54), '$frame'),
    fill(circle(1536, 190, 44), '$bg'),
    line(`${M(1536, 190)}${L(1536, 158)}`, '$ink', 5),
    line(`${M(1536, 190)}${L(1560, 198)}`, '$ink', 5),
    fill(circle(1536, 190, 5), '$ink'),
    // whiteboard
    fill(roundRect(1040, 170, 610, 498, 14), '$boardframe'),
    fill(roundRect(1054, 184, 582, 470, 8), '$board'),
    light(poly([[1054, 184], [1280, 184], [1054, 400]]), 0.5),
    fill(roundRect(1250, 658, 190, 14, 6), '$boardframe'),
    fill(roundRect(1280, 650, 36, 10, 3), '$accent'),
    fill(roundRect(1326, 650, 36, 10, 3), '$accent2'),
    ...door(1650, 236, 184, 541),
  ];
  const floor = [
    fill(rect(0, base, W, H - base), '$floor'),
    ...Array.from({ length: 9 }, (_, i) => fill(rect(0, base + 34 + i * 34, W, 3), '$floor2')),
    ...Array.from({ length: 30 }, (_, i) => fill(rect(((i * 197) % 1900) + 10, base + 34 + (i % 9) * 34, 3, 34), '$floor2')),
    fill(roundRect(540, 846, 880, 170, 26), '$accent2', 0.22),
    fill(roundRect(560, 862, 840, 138, 18), '$accent2', 0.14),
    shade(rect(0, base, W, 6), 0.1),
    shade(rect(0, 1010, W, 70), 0.06),
  ];
  const furniture = [
    ...plantPot(120, 880, 1),
    ...chair(630, '$accent'),
    // desk: top plate, drawer unit on the right, open knee space on the left
    shade(ellipse(1050, 912, 330, 14), 0.14),
    fill(roundRect(780, 698, 540, 28, 8), '$wood'),
    light(roundRect(788, 700, 524, 6, 3), 0.3),
    fill(rect(780, 718, 540, 10), '$wood2'),
    fill(roundRect(1126, 726, 190, 182, 6), '$wood2'),
    fill(roundRect(1138, 738, 166, 76, 6), '$wood'),
    fill(roundRect(1138, 820, 166, 76, 6), '$wood'),
    fill(roundRect(1196, 768, 50, 10, 5), '#6f4f33'),
    fill(roundRect(1196, 850, 50, 10, 5), '#6f4f33'),
    fill(roundRect(794, 726, 18, 182, 4), '$wood2'),
    ...laptopSide(880, 698),
    ...mug(1118, 698, '$accent'),
    ...deskLamp(1262, 698, '$accent'),
  ];
  return {
    layers: [{ name: 'wall', parallax: 0.1, shapes: wall }, { name: 'floor', parallax: 1, shapes: floor }, { name: 'furniture', parallax: 0.7, shapes: furniture }],
    tokens: {
      day: { wall2: '#efe2cf', wood: '#c9a27a', wood2: '#b58a62', frame: '#ffffff', sky: '#a9dcf3', sky2: '#e6f6fd', city: '#8eb2cc', city2: '#7399b6', lit: '#a9c6da', orb: '#fff3b8', floor: '#d9b894', floor2: '#c8a37c', board: '#ffffff', boardframe: '#c3cad8', ink: '#2b3350', lampglow: '#ffffff' },
      night: { bg: '#232a4d', bg2: '#2f3865', text: '#f2f4ff', wall2: '#1d2442', wood: '#7a5a44', wood2: '#664a38', frame: '#c9d0ea', sky: '#141a3a', sky2: '#27306a', city: '#1e2650', city2: '#2a3470', lit: '#ffd98a', orb: '#f4f1de', floor: '#3a3b5c', floor2: '#2f304e', board: '#e9edf7', boardframe: '#8c95b3', ink: '#0f1226', lampglow: '#ffd98a' },
    },
  };
}

// ---------------------------------------------------------------- home office
function bookshelf(x, y, w, h) {
  const s = [fill(roundRect(x, y, w, h, 8), '$wood2'), fill(rect(x + 12, y + 12, w - 24, h - 24), '$shelfback')];
  const r = rng(5);
  const colors = ['$accent', '$accent2', '#f2b544', '#6c7bd6', '#e86f6f', '#58b99a'];
  const rows = 5;
  const rowH = (h - 24) / rows;
  for (let i = 0; i < rows; i++) {
    const by = y + 12 + (i + 1) * rowH;
    let bx = x + 16;
    while (bx < x + w - 40) {
      const bw = 14 + Math.floor(r() * 16);
      const bh = rowH - 14 - Math.floor(r() * 26);
      s.push(fill(rect(bx, by - bh, bw, bh), colors[Math.floor(r() * colors.length)]));
      bx += bw + 2;
    }
    s.push(fill(rect(x + 12, by, w - 24, 8), '$wood'));
  }
  return s;
}

function homeOffice() {
  const base = 777;
  const wall = [
    fill(rect(0, 0, W, base), '$wallh'),
    ...Array.from({ length: 24 }, (_, i) => fill(rect(40 + i * 80, 0, 3, base), '$wallh2', 0.6)),
    fill(rect(0, base - 18, W, 18), '$wood2'),
    // window with curtains
    fill(roundRect(110, 130, 360, 470, 12), '$frame'),
    linear(rect(128, 148, 324, 434), [[0, '$sky'], [1, '$sky2']], 0, 148, 0, 582),
    fill(circle(360, 240, 32), '$orb'),
    fill(`${M(128, 520)}${C(210, 470, 300, 500, 452, 480)}${L(452, 582)}${L(128, 582)}${Z}`, '$hill'),
    fill(circle(230, 440, 46), '$tree'),
    fill(rect(224, 470, 12, 110), '$wood2'),
    fill(rect(285, 148, 8, 434), '$frame'),
    fill(roundRect(70, 100, 440, 14, 7), '$wood2'),
    fill(`${M(84, 114)}${C(150, 200, 130, 400, 168, 600)}${L(84, 600)}${Z}`, '$accent'),
    fill(`${M(496, 114)}${C(430, 200, 450, 400, 412, 600)}${L(496, 600)}${Z}`, '$accent'),
    shade(`${M(112, 130)}${C(146, 240, 138, 420, 168, 600)}${L(146, 600)}${C(120, 420, 124, 240, 98, 130)}${Z}`, 0.1),
    shade(`${M(468, 130)}${C(434, 240, 442, 420, 412, 600)}${L(434, 600)}${C(460, 420, 456, 240, 482, 130)}${Z}`, 0.1),
    // shelf with books and a plant over the desk
    fill(roundRect(780, 300, 480, 18, 6), '$wood'),
    fill(rect(820, 300, 10, 40), '$wood2'),
    fill(rect(1210, 300, 10, 40), '$wood2'),
    fill(rect(840, 226, 22, 74), '$accent2'),
    fill(rect(864, 240, 18, 60), '$accent'),
    fill(rect(884, 232, 24, 68), '#f2b544'),
    fill(roundRect(1100, 246, 70, 54, 8), '$accent'),
    fill(circle(1118, 232, 24), '#38c27d'),
    fill(circle(1150, 224, 28), '#2fa86a'),
    fill(circle(1136, 200, 22), '#46d28a'),
    // framed art
    fill(roundRect(800, 400, 150, 190, 8), '$wood'),
    fill(roundRect(814, 414, 122, 162, 4), '$bg'),
    fill(circle(875, 470, 30), '$accent2'),
    fill(poly([[826, 566], [875, 500], [924, 566]]), '$accent'),
    fill(roundRect(980, 410, 220, 120, 8), '$wood'),
    fill(roundRect(992, 422, 196, 96, 4), '$bg2'),
    line(`${M(1004, 500)}${L(1050, 466)}${L(1096, 486)}${L(1176, 438)}`, '$accent', 6),
    ...bookshelf(1330, 90, 250, 687),
    ...door(1650, 236, 184, 541),
  ];
  const floor = [
    fill(rect(0, base, W, H - base), '$floor'),
    ...Array.from({ length: 10 }, (_, i) => fill(rect(0, base + 30 + i * 31, W, 3), '$floor2')),
    ...Array.from({ length: 34 }, (_, i) => fill(rect(((i * 173) % 1880) + 10, base + 30 + (i % 10) * 31, 3, 31), '$floor2')),
    fill(ellipse(900, 940, 520, 70), '$accent', 0.22),
    fill(ellipse(900, 940, 440, 54), '$accent', 0.14),
    fill(poly([[200, base], [420, base], [640, 1000], [60, 1000]]), '$sun', 0.22),
    shade(rect(0, base, W, 6), 0.1),
    shade(rect(0, 1010, W, 70), 0.06),
  ];
  const furniture = [
    ...chair(630, '$accent2'),
    shade(ellipse(1050, 912, 330, 14), 0.14),
    fill(roundRect(780, 698, 540, 24, 8), '$deskwood'),
    light(roundRect(788, 700, 524, 6, 3), 0.3),
    fill(roundRect(1290, 722, 20, 186, 4), '$deskwood2'),
    fill(roundRect(798, 722, 20, 186, 4), '$deskwood2'),
    fill(roundRect(820, 726, 460, 34, 6), '$deskwood2'),
    ...laptopSide(880, 698),
    ...mug(1118, 698, '$accent2'),
    ...deskLamp(1250, 698, '$accent'),
    ...plantPot(1210, 906, 0.8),
  ];
  return {
    layers: [{ name: 'wall', parallax: 0.1, shapes: wall }, { name: 'floor', parallax: 1, shapes: floor }, { name: 'furniture', parallax: 0.7, shapes: furniture }],
    tokens: {
      day: { wallh: '#f3e6d6', wallh2: '#ead9c3', wood: '#c9a27a', wood2: '#b58a62', deskwood: '#e7cfae', deskwood2: '#d3b48c', frame: '#ffffff', sky: '#bfe6f6', sky2: '#eaf8fd', hill: '#8fcf9a', tree: '#52b788', orb: '#fff3b8', floor: '#d8b48c', floor2: '#c79f77', shelfback: '#efe0c9', sun: '#fff1b8', lampglow: '#ffffff' },
      night: { bg: '#2c2540', bg2: '#3d3560', text: '#f2f4ff', wallh: '#2d2744', wallh2: '#352e52', wood: '#6e5340', wood2: '#5c4535', deskwood: '#8a6f58', deskwood2: '#74594a', frame: '#c4c0de', sky: '#14193a', sky2: '#2a2f6a', hill: '#1f3b4a', tree: '#19424a', orb: '#f4f1de', floor: '#3b3354', floor2: '#322b49', shelfback: '#241f38', sun: '#8d8fe0', lampglow: '#ffd98a' },
    },
  };
}

// ---------------------------------------------------------------- abstract tech
function abstractTech() {
  const horizon = 777;
  const sky = [
    linear(rect(0, 0, W, H), [[0, '$t1'], [1, '$t2']], 0, 0, 0, horizon),
    radial(circle(1500, 330, 760), [[0, '$accent2'], [1, '$t1']], 1500, 330, 760, 0.35),
    radial(circle(260, 640, 520), [[0, '$accent'], [1, '$t1']], 260, 640, 520, 0.22),
  ];
  const r = rng(21);
  for (let i = 0; i < 70; i++) sky.push(fill(circle(r() * W, r() * 640, 1 + r() * 2.4), '#ffffff', 0.25 + r() * 0.45));
  const back = [];
  const nodes = [];
  for (let i = 0; i < 26; i++) nodes.push([80 + r() * 1760, 60 + r() * 640]);
  nodes.forEach(([x, y], i) => {
    const near = nodes.map(([nx, ny], j) => [Math.hypot(nx - x, ny - y), j]).filter(([d, j]) => j > i && d < 320).sort((a, b) => a[0] - b[0]).slice(0, 2);
    for (const [, j] of near) back.push(line(`${M(x, y)}${L(nodes[j][0], nodes[j][1])}`, '$grid', 2, 0.35));
  });
  nodes.forEach(([x, y], i) => {
    back.push(fill(circle(x, y, 14 + (i % 3) * 4), '$accent2', 0.12));
    back.push(fill(circle(x, y, 5 + (i % 3) * 1.5), i % 4 === 0 ? '$accent' : '$accent2', 0.9));
  });
  const mid = [];
  // floating cards
  for (const [x, y, w, h, c] of [[120, 160, 230, 150, '$accent2'], [1560, 520, 250, 160, '$accent'], [300, 520, 200, 120, '$accent2']]) {
    mid.push(fill(roundRect(x, y, w, h, 16), '$panel', 0.55));
    mid.push(line(roundRect(x, y, w, h, 16), c, 3, 0.8));
    for (let i = 0; i < 3; i++) mid.push(fill(roundRect(x + 18, y + 22 + i * 26, w * (0.7 - i * 0.16), 10, 5), c, 0.7 - i * 0.15));
  }
  // HUD board
  const board = [
    fill(roundRect(1040, 170, 610, 480, 22), '$panel', 0.5),
    line(roundRect(1040, 170, 610, 480, 22), '$accent2', 4, 0.9),
    line(`${M(1040, 230)}${L(1040, 190)}${A(20, 20, 0, 0, 1, 1060, 170)}${L(1100, 170)}`, '#ffffff', 6, 0.9),
    line(`${M(1650, 590)}${L(1650, 630)}${A(20, 20, 0, 0, 1, 1630, 650)}${L(1590, 650)}`, '#ffffff', 6, 0.9),
    fill(roundRect(1068, 196, 180, 14, 7), '$accent2', 0.7),
    fill(circle(1618, 204, 8), '$accent'),
  ];
  const floor = [
    fill(rect(0, horizon, W, H - horizon), '$panel'),
    linear(rect(0, horizon, W, H - horizon), [[0, '$accent2'], [1, '$panel']], 0, horizon, 0, H, 0.25),
  ];
  for (let i = -14; i <= 14; i++) floor.push(line(`${M(960 + i * 40, horizon)}${L(960 + i * 360, H)}`, '$grid', 2, 0.55));
  let y = horizon;
  let step = 8;
  while (y < H) {
    floor.push(line(`${M(0, y)}${L(W, y)}`, '$grid', 2, 0.45));
    y += step;
    step *= 1.38;
  }
  floor.push(fill(rect(0, horizon - 2, W, 5), '$accent2', 0.9));
  floor.push(fill(rect(0, horizon - 14, W, 12), '$accent2', 0.18));
  return {
    layers: [{ name: 'sky', parallax: 0, shapes: sky }, { name: 'network', parallax: 0.3, shapes: back }, { name: 'cards', parallax: 0.5, shapes: mid }, { name: 'floor', parallax: 1, shapes: floor }, { name: 'hud', parallax: 0.7, shapes: board }],
    tokens: {
      day: { bg2: '#0e1a47', text: '#f4f7ff', t1: '#16295f', t2: '#2c58c8', grid: '#7fbaff', panel: '#0e1a47' },
      night: { bg2: '#070b22', text: '#f4f7ff', t1: '#090d2b', t2: '#1b2160', grid: '#4f7dff', panel: '#070b22' },
    },
  };
}

export const ENVIRONMENTS = {
  'plain-stage': plainStage,
  office,
  'home-office': homeOffice,
  'abstract-tech': abstractTech,
};

