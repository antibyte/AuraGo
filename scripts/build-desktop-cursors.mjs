// Original AuraGo vector artwork. Run with --check to verify the shipped assets.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const check = process.argv.includes('--check');
const palettes = {
  amiga: { ink: '#101823', edge: '#ffffff', top: '#ffffff', bottom: '#b9c4d2', accent: '#f06428', bg: '#2769b0', pixel: true },
  cyberpunk: { ink: '#061723', edge: '#b9ffff', top: '#fff778', bottom: '#d5be12', accent: '#29e6ff', bg: '#070b18' },
  black_gloss: { ink: '#07090f', edge: '#e0e8ff', top: '#c8d3e7', bottom: '#171e2e', accent: '#ffffff', bg: '#101116' },
  crazy: { ink: '#2c104d', edge: '#fff5a9', top: '#ff9bfb', bottom: '#b24bff', accent: '#83ffcb', bg: '#26113b' },
  lollipop: { ink: '#842e62', edge: '#ffffff', top: '#fff9fd', bottom: '#ffb8d9', accent: '#c2186b', bg: '#fff4f9' },
  organic: { ink: '#29180f', edge: '#f6e8cc', top: '#d5ad67', bottom: '#70472c', accent: '#ffe8ac', bg: '#22160f' },
  electronic_dreams: { ink: '#08232d', edge: '#e4fff8', top: '#a5eadc', bottom: '#277d8d', accent: '#ffca82', bg: '#071c25' },
  windows_xp: { ink: '#102c62', edge: '#ffffff', top: '#f9fcff', bottom: '#70a9ed', accent: '#ffcf4c', bg: '#164a91' },
  green_slime: { ink: '#103d1e', edge: '#f3ffd3', top: '#c7ff52', bottom: '#4f9d20', accent: '#fff179', bg: '#15361d' },
  voxel_world: { ink: '#30221a', edge: '#fff1cb', top: '#e7b86a', bottom: '#9a5b35', accent: '#79c85a', bg: '#29251f', pixel: true }
};
const arrows = {
  amiga: 'M4 3H7V6H10V9H13V12H16V15H19V18H13V21H16V27H12V23H9V19H7V23H4Z',
  cyberpunk: 'M4 3L25 20L16 21L12 29L4 3Z',
  black_gloss: 'M4 3L25 20L16 21L21 29L16 32L11 23L5 28Z',
  crazy: 'M4 3Q13 8 28 18L18 21L21 31L15 33L11 23L4 29Z',
  lollipop: 'M4 3Q7 2 10 6L25 19Q27 22 23 22L17 22L21 29Q22 33 18 33Q16 33 15 30L11 24L6 28Q3 30 4 25Z',
  organic: 'M4 3C17 4 29 12 25 22C22 28 15 25 12 22L8 29C7 20 3 13 4 3Z',
  electronic_dreams: 'M4 3L27 19L18 21L13 31L4 3Z',
  windows_xp: 'M4 3L27 19L18 21L14 31L4 3Z',
  green_slime: 'M4 3Q10 7 15 12L27 21L18 22L12 31L9 22L4 3Z',
  voxel_world: 'M4 3H10V9H16V15H22V21H29V27H18V22H13V16H8V10H4Z'
};
const shapes = {
  pointer: 'M16 4C16 1 21 1 21 4V13C22 10 26 11 26 14C28 12 31 14 31 17C34 16 36 18 35 22L32 31Q30 35 25 35H20Q17 35 15 32L8 22C6 19 9 16 12 19L16 23Z',
  text: 'M12 5H28V9H23V31H28V35H12V31H17V9H12Z',
  grab: 'M13 17V9C13 6 17 6 17 9V16V5C17 2 22 2 22 5V16V8C22 5 27 5 27 8V18V12C27 9 32 9 32 12V25C32 32 28 35 21 35C17 35 14 33 12 29L6 21C4 18 7 15 10 18L13 22Z',
  grabbing: 'M10 17C8 12 12 9 15 13C14 8 20 7 21 12C22 8 27 9 27 14C30 11 34 14 32 19L30 29Q28 34 21 34Q13 34 10 28L6 21C4 17 8 15 11 19Z',
  move: 'M20 3L27 11H23V17H29V13L37 20L29 27V23H23V29H27L20 37L13 29H17V23H11V27L3 20L11 13V17H17V11H13Z',
  'ew-resize': 'M3 20L12 11V17H28V11L37 20L28 29V23H12V29Z',
  'ns-resize': 'M20 3L29 12H23V28H29L20 37L11 28H17V12H11Z'
};
// Diagonals share the precise centre/hotspot of the horizontal resize cursor.
shapes['nesw-resize'] = shapes['ew-resize'];
shapes['nwse-resize'] = shapes['ew-resize'];
const roles = ['default', 'pointer', 'text', 'grab', 'grabbing', 'move', 'ew-resize', 'ns-resize', 'nesw-resize', 'nwse-resize', 'not-allowed', 'wait'];
const outputs = new Map();

function artwork(id, role, suffix = '') {
  const p = palettes[id], g = 'material' + suffix;
  const defs = `<defs><linearGradient id="${g}" x1="0" y1="0" x2=".65" y2="1"><stop stop-color="${p.top}"/><stop offset=".43" stop-color="${p.top}"/><stop offset=".48" stop-color="${p.bottom}"/><stop offset="1" stop-color="${p.bottom}"/></linearGradient></defs>`;
  const fill = p.pixel ? p.top : `url(#${g})`;
  let geometry;
  if (role === 'not-allowed') geometry = '<circle cx="20" cy="20" r="12"/><path d="M11 11L29 29" fill="none"/>';
  else if (role === 'wait') geometry = '<path d="M20 5A15 15 0 1 1 5 20H11A9 9 0 1 0 20 11Z"/>';
  else geometry = `<path d="${role === 'default' ? arrows[id] : shapes[role]}"/>`;
  const rotate = role === 'nesw-resize' ? ' transform="rotate(-45 20 20)"' : role === 'nwse-resize' ? ' transform="rotate(45 20 20)"' : '';
  // A light outer contour and dark inner edge keep the silhouette readable on any wallpaper.
  const silhouette = `<g${rotate} stroke-linejoin="${p.pixel ? 'miter' : 'round'}" stroke-linecap="round"><g fill="${fill}" stroke="${p.edge}" stroke-width="4">${geometry}</g><g fill="${fill}" stroke="${p.ink}" stroke-width="1.7">${geometry}</g></g>`;
  let detail = '';
  if (role === 'default') {
    const details = {
      amiga: '<path d="M6 7V19H8V12L13 17H16Z"/>',
      cyberpunk: '<path d="M8 9L20 18H14L12 23Z" fill="none"/><path d="M7 20L9 26M19 11L25 16" fill="none"/>',
      black_gloss: '<path d="M7 8L19 18H13L9 22Z" fill="#ffffff" opacity=".55" stroke="none"/>',
      crazy: '<path d="M9 10L22 17L15 19L17 28" fill="none"/><circle cx="26" cy="28" r="2" stroke="none"/>',
      lollipop: '<path d="M8 10L18 20M8 17L14 23M16 25L20 30" fill="none" stroke-width="2.5"/>',
      organic: '<path d="M7 7Q15 13 20 21M12 12L19 11M15 16L14 21" fill="none"/>',
      electronic_dreams: '<path d="M8 9H12V6H16V13H20V10H25M9 17H13V21H17V17H22" fill="none" stroke-width="1.8"/>',
      windows_xp: '<path d="M7 8L18 16L13 17L11 23Z" fill="none"/><path d="M19 25H27M23 21V29" stroke-width="1.7"/>',
      green_slime: '<path d="M10 12Q13 8 17 12Q21 16 24 12M13 22Q17 26 22 22" fill="none" stroke-width="2"/><circle cx="25" cy="10" r="2" stroke="none"/>',
      voxel_world: '<path d="M8 10H13V15H8ZM15 15H20V20H15ZM21 21H26V26H21Z" stroke="none"/>'
    };
    detail = `<g fill="${p.accent}" stroke="${p.accent}" stroke-width="1.5" stroke-linecap="round">${details[id]}</g>`;
  } else if (role === 'pointer' || role === 'grab' || role === 'grabbing') {
    detail = `<path d="M18 29H28" stroke="${p.accent}" stroke-width="3" stroke-linecap="round"/>`;
    if (id === 'organic') detail += `<path d="M20 24Q18 17 26 18Q27 25 20 24Z" fill="${p.accent}"/>`;
    if (id === 'lollipop') detail += `<path d="M20 18C16 13 12 21 22 25C32 17 24 13 20 18Z" fill="${p.accent}"/>`;
    if (id === 'cyberpunk') detail += `<path d="M21 18H27M21 22H29" stroke="${p.accent}" stroke-width="1.5"/>`;
  } else detail = `<circle cx="20" cy="20" r="2" fill="${p.accent}"/>`;
  return defs + silhouette + detail;
}

function soundArtwork(id) {
  const p = palettes[id];
  const emblems = {
    amiga: '<rect x="22" y="24" width="36" height="36" rx="1"/><path d="M28 18V24M38 18V24M48 18V24M28 60V66M38 60V66M48 60V66M16 30H22M16 42H22M16 54H22M58 30H64M58 42H64M58 54H64M29 46V35H35V46H41V35H48"/>',
    cyberpunk: '<path d="M40 13L68 42L40 71L12 42ZM28 34L40 22L52 34M27 52L40 63L53 52M24 42H34L40 32L46 53L52 42H70"/><circle cx="70" cy="42" r="3"/>',
    black_gloss: '<path d="M22 59Q29 51 29 36C29 18 51 18 51 36Q51 51 58 59Z"/><path d="M35 65Q40 72 45 65M35 29Q40 24 45 29"/><path d="M19 24Q12 42 19 53M61 24Q68 42 61 53" opacity=".35"/>',
    crazy: '<circle cx="32" cy="39" r="19"/><circle cx="57" cy="58" r="12"/><circle cx="59" cy="22" r="7"/><path d="M23 34Q27 26 35 29M51 55L57 50M56 20L60 18"/>',
    lollipop: '<circle cx="37" cy="34" r="22"/><path d="M47 54L60 72M37 15C59 22 41 52 24 38C11 24 48 15 47 35C44 46 26 37 34 29C39 25 43 33 37 34"/>',
    organic: '<path d="M19 58C12 28 40 17 62 18C64 43 44 64 19 58ZM19 58L49 30M33 44L32 30M42 35L54 35M17 65L12 71"/><path d="M49 62Q59 57 66 48M54 70Q67 64 73 53" opacity=".4"/>',
    electronic_dreams: '<path d="M18 47H27V34H36V51H45V39H54V29H63M18 59H27V54H36V62H45V56H54V48H63"/><circle cx="65" cy="27" r="3"/>',
    windows_xp: '<rect x="23" y="24" width="34" height="34" rx="4"/><path d="M40 24V58M23 41H57M40 18L47 25M40 18L33 25"/>',
    green_slime: '<path d="M40 14C34 25 23 36 23 47A17 17 0 0 0 57 47C57 36 46 25 40 14Z"/><path d="M33 49Q38 56 47 49M34 39Q38 35 42 39"/>',
    voxel_world: '<path d="M20 25H38V43H20ZM42 23H60V41H42ZM31 46H49V64H31ZM52 45H67V60H52Z"/><path d="M24 29H34V39H24ZM46 27H56V37H46ZM35 50H45V60H35Z"/>'
  };
  const waveform = Array.from({length:29},(_,i)=>{
    const height = 4 + 31 * Math.pow(Math.sin((i+1)*0.61),2) * Math.sin(Math.PI*(i+1)/30);
    return `<path d="M${105+i*5.7} ${44-height}V${44+height}" opacity="${(0.4+0.6*Math.sin(Math.PI*(i+1)/30)).toFixed(2)}"/>`;
  }).join('');
  return `<svg xmlns="http://www.w3.org/2000/svg" width="288" height="88" viewBox="0 0 288 88"><rect width="288" height="88" rx="12" fill="${p.bg}"/><g stroke="${p.top}" stroke-width="1" opacity=".12"><path d="M94 12V76M104 44H276M104 24H276M104 64H276"/></g><g fill="none" stroke="${p.accent}" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round">${emblems[id]}</g><g stroke="${p.pixel ? p.top : p.bottom}" stroke-width="3" stroke-linecap="${p.pixel?'square':'round'}">${waveform}</g></svg>\n`;
}

for (const id of Object.keys(palettes)) {
  outputs.set(`ui/img/themes/sounds/${id}.svg`, soundArtwork(id));
  for (const role of roles) {
    outputs.set(`ui/img/cursors/${id}/${role}.svg`, `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 40 40">${artwork(id, role)}</svg>\n`);
  }
  const tiles = ['default', 'pointer', 'text', 'grab', 'ew-resize', 'wait'];
  outputs.set(`ui/img/cursors/${id}/preview.svg`, `<svg xmlns="http://www.w3.org/2000/svg" width="288" height="88" viewBox="0 0 288 88"><rect width="288" height="88" rx="12" fill="${palettes[id].bg}"/>${tiles.map((role, i) => `<g transform="translate(${12 + i * 46} 24)">${artwork(id, role, String(i))}</g>`).join('')}</svg>\n`);
}
let drift = false;
for (const [relative, contents] of outputs) {
  const file = path.join(root, relative);
  if (check) {
    if (!fs.existsSync(file) || fs.readFileSync(file, 'utf8') !== contents) { console.error('Stale cursor artwork:', relative); drift = true; }
  } else { fs.mkdirSync(path.dirname(file), { recursive: true }); fs.writeFileSync(file, contents); }
}
if (drift) process.exitCode = 1;
else console.log(`${check ? 'Verified' : 'Generated'} ${outputs.size} original desktop theme assets.`);
