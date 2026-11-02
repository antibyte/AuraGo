// The 23 starter props. Flat shapes with a second tone, coloured from palette tokens ($accent, $accent2, $fg, $bg2)
// and a few fixed neutrals so they read on both starter palettes. Every prop stands on the bottom centre of its
// viewbox (anchor 0.5, 1) and is sized by its height as a fraction of the stage height.

import {
  fill, shade, light, line, linear, ellipse, circle, rect, roundRect, roundRect4, poly, ribbon, M, L, C, Z, A,
  rad,
} from './util.mjs';

const WHITE = '#ffffff';
const PAPER = '#f7f8fb';
const STEEL = '#c9d0dc';
const STEEL_DARK = '#8f99ae';
const INK = '#2b3350';
const GREEN = '#34c77b';
const RED = '#ef5b5b';
const YELLOW = '#f7b731';

const groundShadow = (cx, y, rx) => shade(ellipse(cx, y, rx, rx * 0.12), 0.12);

function laptop() {
  const s = [];
  s.push(groundShadow(120, 166, 100));
  s.push(fill(roundRect(36, 8, 168, 112, 9), '$fg'));
  s.push(fill(roundRect(44, 16, 152, 96, 4), '#eef3fb'));
  s.push(linear(roundRect(44, 16, 152, 96, 4), [[0, '$accent2'], [1, '$accent']], 44, 16, 196, 112, 0.22));
  // chart on the screen
  for (const [i, h] of [30, 52, 38, 66].entries()) s.push(fill(roundRect(62 + i * 28, 100 - h, 18, h, 3), i % 2 ? '$accent' : '$accent2'));
  s.push(fill(roundRect(62, 28, 52, 7, 3), '$fg', 0.55));
  s.push(fill(roundRect(62, 40, 34, 7, 3), '$fg', 0.3));
  s.push(fill(poly([[16, 124], [224, 124], [236, 152], [4, 152]]), '#d7dce6'));
  s.push(fill(roundRect4(4, 148, 232, 12, [0, 0, 8, 8]), STEEL_DARK));
  s.push(fill(roundRect(98, 128, 44, 9, 4), STEEL, 0.9));
  s.push(shade(poly([[120, 124], [224, 124], [236, 152], [120, 152]]), 0.06));
  s.push(light(roundRect(48, 18, 70, 6, 3), 0.35));
  return { viewbox: [0, 0, 240, 170], shapes: s };
}

function phone() {
  const s = [];
  s.push(groundShadow(50, 196, 38));
  s.push(fill(roundRect(8, 4, 84, 188, 16), '$fg'));
  s.push(fill(roundRect(14, 14, 72, 168, 9), '#f4f7fd'));
  s.push(fill(roundRect(36, 18, 28, 6, 3), '$fg'));
  s.push(fill(roundRect(22, 36, 40, 22, 9), '$accent2'));
  s.push(fill(roundRect(38, 66, 40, 22, 9), '$accent'));
  s.push(fill(roundRect(22, 96, 52, 22, 9), '$accent2', 0.8));
  for (let i = 0; i < 3; i++) s.push(fill(roundRect(22 + i * 20, 140, 16, 16, 5), i === 1 ? '$accent' : '$fg', i === 1 ? 1 : 0.25));
  s.push(fill(roundRect(36, 168, 28, 4, 2), '$fg', 0.35));
  s.push(light(poly([[14, 14], [60, 14], [14, 70]]), 0.22));
  return { viewbox: [0, 0, 100, 200], shapes: s };
}

function monitor() {
  const s = [];
  s.push(groundShadow(120, 196, 90));
  s.push(fill(roundRect(76, 176, 88, 14, 6), STEEL_DARK));
  s.push(fill(poly([[104, 144], [136, 144], [142, 178], [98, 178]]), STEEL));
  s.push(fill(roundRect(8, 6, 224, 142, 10), '$fg'));
  s.push(fill(roundRect(16, 14, 208, 124, 5), '#eef3fb'));
  s.push(linear(roundRect(16, 14, 208, 124, 5), [[0, '$accent2'], [1, '$accent']], 16, 14, 224, 138, 0.2));
  s.push(fill(roundRect(30, 28, 70, 8, 4), '$fg', 0.45));
  s.push(fill(roundRect(30, 44, 46, 8, 4), '$fg', 0.25));
  for (const [i, h] of [28, 50, 36, 70, 58].entries()) s.push(fill(roundRect(104 + i * 22, 126 - h, 14, h, 3), i % 2 ? '$accent' : '$accent2'));
  s.push(line(poly([[30, 112], [52, 92], [72, 100], [92, 70]], false), '$accent', 5));
  s.push(light(poly([[16, 14], [110, 14], [16, 96]]), 0.18));
  return { viewbox: [0, 0, 240, 200], shapes: s };
}

function server() {
  const s = [];
  s.push(groundShadow(70, 216, 58));
  s.push(fill(roundRect(14, 6, 112, 206, 8), '#3a4261'));
  for (let i = 0; i < 4; i++) {
    const y = 16 + i * 48;
    s.push(fill(roundRect(22, y, 96, 40, 5), '#4a5478'));
    s.push(fill(roundRect(30, y + 8, 56, 6, 3), '#2b3350'));
    s.push(fill(roundRect(30, y + 20, 56, 6, 3), '#2b3350'));
    s.push(fill(circle(100, y + 14, 5), i % 2 ? '$accent2' : GREEN));
    s.push(fill(circle(100, y + 28, 5), i === 2 ? '$accent' : '$accent2', 0.8));
    s.push(light(rect(22, y, 96, 4), 0.1));
  }
  s.push(shade(roundRect4(84, 6, 42, 206, [0, 8, 8, 0]), 0.14));
  return { viewbox: [0, 0, 140, 220], shapes: s };
}

function router() {
  const s = [];
  s.push(groundShadow(110, 134, 90));
  s.push(fill(roundRect(24, 14, 10, 62, 5), '$fg'));
  s.push(fill(roundRect(186, 14, 10, 62, 5), '$fg'));
  s.push(fill(roundRect(8, 70, 204, 56, 14), '$accent'));
  s.push(shade(roundRect4(8, 100, 204, 26, [0, 0, 14, 14]), 0.16));
  s.push(light(roundRect(18, 76, 184, 8, 4), 0.25));
  for (let i = 0; i < 4; i++) s.push(fill(circle(48 + i * 24, 98, 5.5), i === 3 ? '$accent2' : WHITE, i === 3 ? 1 : 0.92));
  s.push(fill(roundRect(150, 90, 40, 8, 4), WHITE, 0.55));
  // signal arcs
  for (const [i, r] of [14, 26, 38].entries()) s.push(line(`${M(110 - r, 58 - r * 0.2)}${A(r, r, 0, 0, 1, 110 + r, 58 - r * 0.2)}`, '$accent2', 5, 0.95 - i * 0.2));
  return { viewbox: [0, 0, 220, 140], shapes: s };
}

function cloud() {
  const s = [];
  const body = (dy = 0, color = '$accent2', opacity) => [
    fill(circle(76, 98 + dy, 40), color, opacity), fill(circle(124, 70 + dy, 54), color, opacity), fill(circle(176, 100 + dy, 42), color, opacity),
    fill(roundRect(44, 100 + dy, 170, 40, 20), color, opacity),
  ];
  s.push(...body());
  s.push(shade(roundRect4(44, 120, 170, 20, [0, 0, 20, 20]), 0.1));
  s.push(light(`${M(88, 60)}${A(30, 30, 0, 0, 1, 130, 28)}${A(36, 36, 0, 0, 0, 96, 60)}${Z}`, 0.35));
  return { viewbox: [0, 0, 240, 160], shapes: s };
}

function database() {
  const s = [];
  s.push(groundShadow(80, 196, 66));
  const disc = (y, color) => {
    s.push(fill(`${M(14, y)}${L(14, y + 42)}${A(66, 20, 0, 0, 0, 146, y + 42)}${L(146, y)}${Z}`, color));
    s.push(shade(`${M(90, y)}${L(146, y)}${L(146, y + 42)}${A(66, 20, 0, 0, 1, 90, y + 62)}${Z}`, 0.12));
    s.push(fill(ellipse(80, y, 66, 20), color));
    s.push(light(ellipse(80, y, 66, 20), 0.2));
    s.push(fill(circle(124, y + 28, 4.5), WHITE, 0.9));
  };
  disc(116, '$accent');
  disc(66, '$accent2');
  disc(16, '$accent');
  return { viewbox: [0, 0, 160, 200], shapes: s };
}

function lock() {
  const s = [];
  s.push(groundShadow(60, 146, 44));
  s.push(fill(`${M(34, 66)}${L(34, 44)}${A(26, 28, 0, 0, 1, 86, 44)}${L(86, 66)}${L(72, 66)}${L(72, 44)}${A(12, 14, 0, 0, 0, 48, 44)}${L(48, 66)}${Z}`, '#9aa4b8'));
  s.push(fill(roundRect(14, 64, 92, 80, 14), '$accent'));
  s.push(shade(roundRect4(60, 64, 46, 80, [0, 14, 14, 0]), 0.14));
  s.push(light(roundRect(22, 70, 76, 8, 4), 0.3));
  s.push(fill(circle(60, 96, 11), '$fg'));
  s.push(fill(poly([[55, 100], [65, 100], [68, 124], [52, 124]]), '$fg'));
  return { viewbox: [0, 0, 120, 150], shapes: s };
}

function key() {
  const s = [];
  s.push(fill(circle(52, 52, 38), '$accent'));
  s.push(fill(circle(52, 52, 16), '$bg'));
  s.push(shade(`${M(52, 14)}${A(38, 38, 0, 0, 1, 52, 90)}${L(52, 52)}${Z}`, 0.12));
  s.push(fill(roundRect(84, 44, 118, 16, 8), '$accent'));
  s.push(fill(roundRect(152, 58, 14, 28, 5), '$accent'));
  s.push(fill(roundRect(176, 58, 14, 20, 5), '$accent'));
  s.push(shade(roundRect(84, 52, 118, 8, 4), 0.14));
  s.push(light(`${M(30, 36)}${A(26, 26, 0, 0, 1, 56, 18)}${L(56, 24)}${A(20, 20, 0, 0, 0, 36, 40)}${Z}`, 0.3));
  return { viewbox: [0, 0, 210, 104], shapes: s };
}

function shield() {
  const s = [];
  const body = `${M(80, 8)}${C(100, 24, 130, 28, 148, 28)}${L(148, 84)}${C(148, 128, 116, 160, 80, 178)}${C(44, 160, 12, 128, 12, 84)}${L(12, 28)}${C(30, 28, 60, 24, 80, 8)}${Z}`;
  s.push(fill(body, '$accent2'));
  s.push(shade(`${M(80, 8)}${C(100, 24, 130, 28, 148, 28)}${L(148, 84)}${C(148, 128, 116, 160, 80, 178)}${Z}`, 0.14));
  s.push(fill(`${M(80, 24)}${C(96, 36, 118, 40, 134, 41)}${L(134, 84)}${C(134, 120, 108, 148, 80, 164)}${C(52, 148, 26, 120, 26, 84)}${L(26, 41)}${C(42, 40, 64, 36, 80, 24)}${Z}`, WHITE, 0.2));
  s.push({ d: ribbon([[50, 88], [72, 112], [112, 66]], 16), fill: WHITE });
  return { viewbox: [0, 0, 160, 190], shapes: s };
}

function mail() {
  const s = [];
  s.push(groundShadow(100, 146, 80));
  s.push(fill(roundRect(8, 24, 184, 118, 12), '#f2f4fa'));
  s.push(shade(roundRect4(8, 84, 184, 58, [0, 0, 12, 12]), 0.07));
  s.push(fill(poly([[8, 38], [100, 100], [192, 38], [192, 142], [8, 142]]).replace(/Z$/, 'Z'), '#e3e8f3'));
  s.push(fill(`${M(8, 36)}${C(8, 28, 14, 24, 22, 24)}${L(178, 24)}${C(186, 24, 192, 28, 192, 36)}${L(100, 92)}${Z}`, '$accent'));
  s.push(shade(`${M(100, 92)}${L(192, 36)}${L(192, 44)}${L(100, 100)}${Z}`, 0.15));
  s.push(fill(circle(100, 92, 9), '$accent2'));
  return { viewbox: [0, 0, 200, 150], shapes: s };
}

function document() {
  const s = [];
  s.push(groundShadow(75, 198, 58));
  s.push(fill(`${M(14, 8)}${L(100, 8)}${L(136, 44)}${L(136, 188)}${L(14, 188)}${Z}`, PAPER));
  s.push(fill(`${M(100, 8)}${L(136, 44)}${L(100, 44)}${Z}`, '#d9dfec'));
  s.push(shade(`${M(110, 8)}${L(136, 8)}${L(136, 188)}${L(110, 188)}${Z}`, 0.04));
  s.push(fill(roundRect(28, 24, 50, 9, 4), '$accent'));
  for (let i = 0; i < 7; i++) s.push(fill(roundRect(28, 58 + i * 17, i % 3 === 2 ? 56 : 92, 7, 3.5), '$fg', i % 3 === 2 ? 0.2 : 0.3));
  s.push(fill(circle(104, 156, 14), '$accent2'));
  s.push({ d: ribbon([[98, 156], [102, 161], [110, 150]], 4), fill: WHITE });
  return { viewbox: [0, 0, 150, 200], shapes: s };
}

function coffeeCup() {
  const s = [];
  s.push(groundShadow(78, 160, 62));
  for (const x of [58, 82, 106]) s.push({ d: ribbon([[x, 44], [x - 8, 32], [x + 6, 20], [x - 4, 6]], 5), fill: '$fg', opacity: 0.18 });
  s.push(fill(ellipse(80, 150, 70, 12), '#e1e6f0'));
  s.push(fill(`${M(28, 56)}${L(132, 56)}${L(122, 134)}${A(10, 10, 0, 0, 1, 112, 144)}${L(48, 144)}${A(10, 10, 0, 0, 1, 38, 134)}${Z}`, '$accent'));
  s.push(shade(`${M(90, 56)}${L(132, 56)}${L(122, 134)}${A(10, 10, 0, 0, 1, 112, 144)}${L(84, 144)}${Z}`, 0.14));
  s.push(fill(rect(28, 56, 104, 14), WHITE, 0.22));
  s.push(fill(ellipse(80, 56, 52, 9), '#5a3a2a'));
  s.push(light(ellipse(70, 54, 22, 3.4), 0.25));
  s.push(line(`${M(130, 70)}${C(160, 68, 160, 112, 124, 114)}`, '$accent', 11));
  return { viewbox: [0, 0, 160, 166], shapes: s };
}

function desk() {
  const s = [];
  s.push(groundShadow(210, 192, 190));
  s.push(fill(roundRect(10, 20, 400, 22, 7), '#b98a5e'));
  s.push(fill(rect(10, 34, 400, 8), '#8f6a45'));
  s.push(light(roundRect(18, 22, 384, 5, 2.5), 0.3));
  s.push(fill(roundRect(26, 42, 16, 142, 3), '#8f6a45'));
  s.push(fill(roundRect(378, 42, 16, 142, 3), '#8f6a45'));
  s.push(fill(roundRect(250, 42, 128, 104, 6), '#a67a50'));
  for (const y of [50, 88]) {
    s.push(fill(roundRect(258, y, 112, 32, 5), '#c4966a'));
    s.push(fill(roundRect(298, y + 11, 32, 8, 4), '#6f4f33'));
  }
  return { viewbox: [0, 0, 420, 196], shapes: s };
}

function chair() {
  const s = [];
  s.push(groundShadow(90, 216, 70));
  s.push(fill(roundRect(24, 6, 36, 118, 16), '$accent'));
  s.push(shade(roundRect4(42, 6, 18, 118, [0, 16, 16, 0]), 0.14));
  s.push(fill(roundRect(24, 106, 112, 26, 12), '$accent'));
  s.push(shade(roundRect4(24, 120, 112, 12, [0, 0, 12, 12]), 0.16));
  s.push(fill(rect(70, 130, 10, 54), STEEL_DARK));
  s.push(fill(poly([[28, 200], [70, 184], [80, 184], [128, 200], [128, 208], [28, 208]]), '#6b7690'));
  for (const x of [34, 78, 122]) s.push(fill(circle(x, 210, 7), INK));
  s.push(fill(roundRect(110, 112, 24, 8, 4), '#6b7690'));
  return { viewbox: [0, 0, 160, 220], shapes: s };
}

function plant() {
  const s = [];
  s.push(groundShadow(70, 196, 50));
  const leaf = (x, y, ang, len, color, op) => {
    const a = rad(ang);
    const dx = Math.sin(a);
    const dy = -Math.cos(a);
    const tipx = x + dx * len;
    const tipy = y + dy * len;
    const nx = -dy * len * 0.32;
    const ny = dx * len * 0.32;
    s.push(fill(`${M(x, y)}${C(x + dx * len * 0.35 + nx, y + dy * len * 0.35 + ny, x + dx * len * 0.8 + nx * 0.8, y + dy * len * 0.8 + ny * 0.8, tipx, tipy)}${C(x + dx * len * 0.8 - nx * 0.8, y + dy * len * 0.8 - ny * 0.8, x + dx * len * 0.35 - nx, y + dy * len * 0.35 - ny, x, y)}${Z}`, color, op));
    s.push({ d: ribbon([[x, y], [x + dx * len * 0.5, y + dy * len * 0.5], [tipx, tipy]], 2.4), fill: WHITE, opacity: 0.35 });
  };
  for (const [a, l, c] of [[-62, 84, '#2fa86a'], [58, 88, '#2fa86a'], [-30, 108, '#38c27d'], [28, 112, '#38c27d'], [0, 120, '#46d28a']]) leaf(70, 118, a, l, c);
  s.push(fill(`${M(30, 112)}${L(110, 112)}${L(100, 190)}${A(8, 8, 0, 0, 1, 92, 196)}${L(48, 196)}${A(8, 8, 0, 0, 1, 40, 190)}${Z}`, '$accent'));
  s.push(fill(roundRect(24, 104, 92, 16, 6), '$accent'));
  s.push(shade(`${M(76, 104)}${L(116, 104)}${L(116, 120)}${L(110, 112)}${L(100, 190)}${A(8, 8, 0, 0, 1, 92, 196)}${L(76, 196)}${Z}`, 0.14));
  s.push(light(roundRect(30, 106, 52, 4, 2), 0.3));
  return { viewbox: [0, 0, 140, 200], shapes: s };
}

const badge = (color, glyph) => {
  const s = [fill(circle(80, 80, 70), color), shade(`${M(80, 10)}${A(70, 70, 0, 0, 1, 80, 150)}${Z}`, 0.12), light(`${M(34, 52)}${A(52, 52, 0, 0, 1, 90, 26)}${A(60, 60, 0, 0, 0, 40, 60)}${Z}`, 0.22)];
  s.push(...glyph);
  return { viewbox: [0, 0, 160, 160], shapes: s };
};

function check() {
  return badge(GREEN, [{ d: ribbon([[46, 82], [70, 106], [114, 56]], 18), fill: WHITE }]);
}

function cross() {
  return badge(RED, [{ d: ribbon([[54, 54], [106, 106]], 18), fill: WHITE }, { d: ribbon([[106, 54], [54, 106]], 18), fill: WHITE }]);
}

function warning() {
  const s = [];
  const tri = `${M(80, 14)}${C(86, 14, 90, 17, 93, 22)}${L(150, 120)}${C(156, 130, 150, 142, 138, 142)}${L(22, 142)}${C(10, 142, 4, 130, 10, 120)}${L(67, 22)}${C(70, 17, 74, 14, 80, 14)}${Z}`;
  s.push(fill(tri, YELLOW));
  s.push(shade(`${M(80, 14)}${C(86, 14, 90, 17, 93, 22)}${L(150, 120)}${C(156, 130, 150, 142, 138, 142)}${L(80, 142)}${Z}`, 0.1));
  s.push(fill(roundRect(72, 54, 16, 50, 8), '$fg'));
  s.push(fill(circle(80, 122, 9), '$fg'));
  return { viewbox: [0, 0, 160, 150], shapes: s };
}

function idea() {
  const s = [];
  for (let i = 0; i < 7; i++) {
    const a = rad(-120 + i * 40);
    s.push({ d: ribbon([[80 + Math.sin(a) * 66, 76 - Math.cos(a) * 66], [80 + Math.sin(a) * 86, 76 - Math.cos(a) * 86]], 7), fill: YELLOW, opacity: 0.9 });
  }
  s.push(fill(`${M(52, 118)}${C(36, 100, 30, 84, 34, 66)}${C(40, 36, 64, 18, 80, 18)}${C(96, 18, 120, 36, 126, 66)}${C(130, 84, 124, 100, 108, 118)}${C(104, 126, 102, 132, 102, 140)}${L(58, 140)}${C(58, 132, 56, 126, 52, 118)}${Z}`, YELLOW));
  s.push(shade(`${M(80, 18)}${C(96, 18, 120, 36, 126, 66)}${C(130, 84, 124, 100, 108, 118)}${C(104, 126, 102, 132, 102, 140)}${L(80, 140)}${Z}`, 0.1));
  s.push(light(`${M(50, 60)}${C(52, 44, 62, 34, 74, 30)}${C(64, 40, 58, 52, 58, 66)}${Z}`, 0.5));
  s.push(fill(roundRect(56, 142, 48, 11, 5), '#9aa4b8'));
  s.push(fill(roundRect(60, 156, 40, 11, 5), '#7d88a0'));
  s.push(fill(roundRect4(68, 168, 24, 10, [0, 0, 5, 5]), '#6b7690'));
  s.push({ d: ribbon([[66, 126], [66, 96], [80, 82], [94, 96], [94, 126]], 4), fill: '$fg', opacity: 0.35 });
  return { viewbox: [0, 0, 160, 182], shapes: s };
}

function question() {
  const s = [];
  s.push(fill(`${M(80, 8)}${C(122, 8, 152, 36, 152, 74)}${C(152, 112, 122, 138, 86, 138)}${L(56, 164)}${L(60, 136)}${C(32, 128, 8, 106, 8, 74)}${C(8, 36, 38, 8, 80, 8)}${Z}`, '$accent'));
  s.push(shade(`${M(80, 8)}${C(122, 8, 152, 36, 152, 74)}${C(152, 112, 122, 138, 86, 138)}${L(80, 138)}${Z}`, 0.12));
  s.push(light(`${M(24, 60)}${C(28, 34, 50, 20, 76, 18)}${C(52, 28, 38, 44, 36, 70)}${Z}`, 0.22));
  s.push(line(`${M(58, 56)}${C(58, 40, 102, 40, 102, 62)}${C(102, 78, 80, 78, 80, 94)}`, WHITE, 14));
  s.push(fill(circle(80, 116, 8.5), WHITE));
  return { viewbox: [0, 0, 160, 166], shapes: s };
}

function arrowRight() {
  const s = [];
  s.push(fill(`${M(10, 54)}${L(118, 54)}${L(118, 22)}${L(190, 80)}${L(118, 138)}${L(118, 106)}${L(10, 106)}${Z}`, '$accent'));
  s.push(shade(`${M(10, 80)}${L(190, 80)}${L(118, 138)}${L(118, 106)}${L(10, 106)}${Z}`, 0.14));
  s.push(light(`${M(14, 58)}${L(122, 58)}${L(122, 30)}${L(150, 52)}${L(14, 52)}${Z}`, 0.25));
  return { viewbox: [0, 0, 200, 160], shapes: s };
}

function gear() {
  const s = [];
  const teeth = 8;
  const pts = [];
  for (let i = 0; i < teeth; i++) {
    const base = (i / teeth) * 360 - 22.5;
    for (const [da, r] of [[0, 58], [8, 58], [15, 78], [30, 78], [37, 58]]) {
      const a = rad(base + da);
      pts.push([80 + Math.sin(a) * r, 80 - Math.cos(a) * r]);
    }
  }
  const body = poly(pts);
  s.push(fill(body, '#8d97ad'));
  s.push(fill(circle(80, 80, 52), '#a4adc0'));
  s.push(shade(`${M(80, 28)}${A(52, 52, 0, 0, 1, 80, 132)}${Z}`, 0.12));
  s.push(fill(circle(80, 80, 24), '$bg'));
  s.push(shade(circle(80, 80, 24), 0.12));
  s.push(light(`${M(36, 60)}${A(48, 48, 0, 0, 1, 72, 32)}${L(72, 40)}${A(40, 40, 0, 0, 0, 44, 62)}${Z}`, 0.3));
  return { viewbox: [0, 0, 160, 160], shapes: s };
}

/** name -> {build, h (fraction of the stage height), tags, hold} */
export const PROPS = {
  laptop: { build: laptop, h: 0.1, tags: ['computer', 'notebook', 'work', 'office'] },
  phone: { build: phone, h: 0.11, tags: ['mobile', 'smartphone', 'call', 'app'] },
  monitor: { build: monitor, h: 0.16, tags: ['screen', 'display', 'computer', 'desktop'] },
  server: { build: server, h: 0.26, tags: ['rack', 'datacenter', 'hosting', 'it'] },
  router: { build: router, h: 0.09, tags: ['network', 'wifi', 'internet', 'gateway'] },
  cloud: { build: cloud, h: 0.14, tags: ['internet', 'storage', 'online', 'hosting'] },
  database: { build: database, h: 0.17, tags: ['data', 'storage', 'sql', 'records'] },
  lock: { build: lock, h: 0.14, tags: ['security', 'lock', 'padlock', 'protection'] },
  key: { build: key, h: 0.08, tags: ['security', 'password', 'access', 'unlock'] },
  shield: { build: shield, h: 0.16, tags: ['security', 'shield', 'protection', 'firewall'] },
  mail: { build: mail, h: 0.1, tags: ['email', 'message', 'letter', 'phishing'] },
  document: { build: document, h: 0.16, tags: ['file', 'paper', 'report', 'contract'] },
  'coffee-cup': { build: coffeeCup, h: 0.08, tags: ['coffee', 'break', 'drink', 'cup'], hold: true },
  desk: { build: desk, h: 0.2, tags: ['table', 'furniture', 'office', 'workplace'] },
  chair: { build: chair, h: 0.22, tags: ['seat', 'furniture', 'office', 'sit'] },
  plant: { build: plant, h: 0.2, tags: ['nature', 'decor', 'green', 'pot'] },
  check: { build: check, h: 0.1, tags: ['yes', 'ok', 'correct', 'success'] },
  cross: { build: cross, h: 0.1, tags: ['no', 'wrong', 'error', 'denied'] },
  warning: { build: warning, h: 0.12, tags: ['alert', 'danger', 'attention', 'caution'] },
  idea: { build: idea, h: 0.14, tags: ['light bulb', 'tip', 'insight', 'thought'] },
  question: { build: question, h: 0.13, tags: ['ask', 'unknown', 'help', 'why'] },
  'arrow-right': { build: arrowRight, h: 0.08, tags: ['direction', 'next', 'flow', 'then'] },
  gear: { build: gear, h: 0.12, tags: ['settings', 'process', 'engine', 'configuration'] },
};

