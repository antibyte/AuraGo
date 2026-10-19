// Original Windows XP-inspired artwork for AuraGo's desktop icon selector.
// The backend preferred icon catalog is the source of truth for manifest coverage.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const check = process.argv.includes('--check');
const papirus = JSON.parse(fs.readFileSync(path.join(root, 'ui/img/papirus/manifest.json'), 'utf8'));
const xpSystemIcons = new Set([
  'folder', 'folder-open', 'folder-plus', 'folder-symbolic', 'desktop', 'desktop-symbolic',
  'monitor', 'monitor-symbolic', 'trash', 'trash-empty', 'trash-full', 'settings',
  'settings-symbolic', 'home', 'documents'
]);
const archetypes = {
  application: '<rect x="8" y="11" width="48" height="42" rx="4" fill="url(#pane)" stroke="#476886" stroke-width="2"/><path d="M9 16H55V24H9Z" fill="url(#blue)"/><circle cx="49" cy="20" r="1.5" fill="#fff"/><rect x="15" y="30" width="13" height="13" rx="2" fill="#f7c84c"/><rect x="34" y="30" width="13" height="13" rx="2" fill="#61be59"/><path d="M16 48H47" stroke="#fff" stroke-width="2"/>',
  folder: '<path d="M8 17Q8 13 12 13H27L33 19H52Q57 19 57 24V47Q57 52 52 52H12Q8 52 8 47Z" fill="url(#folder)" stroke="#a66b12" stroke-width="2"/><path d="M8 25H57L53 46Q52 50 48 50H13Q9 50 9 46Z" fill="url(#gold)" stroke="#d59422"/><path d="M11 28H53" stroke="#fff2ad" stroke-width="2" opacity=".9"/>',
  computer: '<rect x="7" y="9" width="50" height="34" rx="4" fill="#d9e7f4" stroke="#536d88" stroke-width="2"/><rect x="11" y="13" width="42" height="26" rx="2" fill="url(#screen)"/><path d="M12 34Q24 21 35 33T52 28V38H12Z" fill="#42a83d"/><rect x="29" y="43" width="7" height="8" fill="#a9bdcf"/><path d="M22 53H43" stroke="#5d758b" stroke-width="3" stroke-linecap="round"/>',
  recycle_bin: '<path d="M16 22H49L46 53Q45 57 41 57H24Q20 57 19 53Z" fill="url(#glass)" stroke="#59768a" stroke-width="2"/><path d="M13 19H52M24 15H41" stroke="#59768a" stroke-width="3" stroke-linecap="round"/><path d="M24 30L29 27L31 31M40 31L43 36L39 38M30 45L25 43L26 39M25 32L20 36L22 41M38 43L34 47L30 44" fill="none" stroke="#1688b0" stroke-width="2.5" stroke-linejoin="round"/>',
  settings: '<path d="M34 8L37 13L43 14L48 11L53 16L50 21L52 27L58 30V37L52 39L50 45L53 50L48 55L43 52L37 54L34 60H27L25 54L19 52L14 55L9 50L12 45L10 39L4 37V30L10 27L12 21L9 16L14 11L19 14L25 13L27 8Z" transform="translate(1 -2) scale(.95)" fill="url(#silver)" stroke="#687f95" stroke-width="2"/><circle cx="32" cy="33" r="9" fill="#eaf4fd" stroke="#53718c" stroke-width="3"/><circle cx="32" cy="33" r="4" fill="url(#blue)"/>',
  browser: '<circle cx="32" cy="33" r="24" fill="url(#blue)" stroke="#315b8a" stroke-width="2"/><path d="M9 33H55M32 9C21 19 21 47 32 57M32 9C43 19 43 47 32 57M15 19H49M15 47H49" fill="none" stroke="#dff3ff" stroke-width="2"/><path d="M12 40Q27 31 48 16" fill="none" stroke="#91d55f" stroke-width="4" stroke-linecap="round"/>',
  documents: '<path d="M17 12H43L52 21V52Q52 56 48 56H17Q13 56 13 52V16Q13 12 17 12Z" fill="#f8fbff" stroke="#6b8298" stroke-width="2"/><path d="M42 12V22H52" fill="#b8d3ec" stroke="#6b8298" stroke-width="2"/><path d="M21 29H43M21 36H43M21 43H39" stroke="#5896c9" stroke-width="3" stroke-linecap="round"/>',
  terminal: '<rect x="7" y="10" width="50" height="42" rx="4" fill="#dfeaf5" stroke="#536d88" stroke-width="2"/><path d="M11 14H53V46H11Z" fill="#14243b"/><path d="M17 24L23 29L17 34M27 35H37" fill="none" stroke="#a5e66c" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/><path d="M26 52V57M38 52V57M20 58H44" stroke="#637e98" stroke-width="3" stroke-linecap="round"/>',
  mail: '<path d="M8 18Q8 14 12 14H52Q56 14 56 18V48Q56 52 52 52H12Q8 52 8 48Z" fill="url(#pane)" stroke="#54728d" stroke-width="2"/><path d="M10 19L32 37L54 19" fill="none" stroke="#347cb7" stroke-width="5" stroke-linejoin="round"/><path d="M10 48L26 34M54 48L38 34" fill="none" stroke="#8bb3d2" stroke-width="2"/>',
  chat: '<path d="M10 13H49Q55 13 55 19V42Q55 48 49 48H33L20 57V48H10Q5 48 5 42V19Q5 13 10 13Z" fill="url(#blue)" stroke="#315d92" stroke-width="2"/><circle cx="19" cy="31" r="2.4" fill="#fff"/><circle cx="30" cy="31" r="2.4" fill="#fff"/><circle cx="41" cy="31" r="2.4" fill="#fff"/>',
  network: '<circle cx="21" cy="31" r="10" fill="url(#blue)" stroke="#315d92" stroke-width="2"/><circle cx="46" cy="17" r="8" fill="url(#green)" stroke="#4c813e" stroke-width="2"/><circle cx="47" cy="47" r="8" fill="url(#gold)" stroke="#ad7922" stroke-width="2"/><path d="M30 27L39 21M30 36L40 43" stroke="#536f8b" stroke-width="4"/><circle cx="21" cy="31" r="3" fill="#fff"/>',
  database: '<ellipse cx="32" cy="17" rx="20" ry="8" fill="#8dc8ec" stroke="#456d8b" stroke-width="2"/><path d="M12 17V45Q12 53 32 53T52 45V17" fill="url(#blue)" stroke="#456d8b" stroke-width="2"/><path d="M12 28Q32 40 52 28M12 39Q32 51 52 39" fill="none" stroke="#dff3ff" stroke-width="2"/>',
  audio: '<path d="M11 27H21L36 15V51L21 39H11Z" fill="url(#blue)" stroke="#345b82" stroke-width="2"/><path d="M42 25Q52 33 42 41M46 19Q62 33 46 47" fill="none" stroke="#4c9c4b" stroke-width="4" stroke-linecap="round"/>',
  image: '<rect x="8" y="11" width="48" height="42" rx="4" fill="#f7fbff" stroke="#607b95" stroke-width="2"/><circle cx="42" cy="22" r="5" fill="#f3c64e"/><path d="M11 46L24 31L33 40L40 32L53 47V50H11Z" fill="url(#green)" stroke="#478044" stroke-width="2"/>',
  video: '<rect x="8" y="15" width="48" height="36" rx="5" fill="url(#blue)" stroke="#315b8a" stroke-width="2"/><path d="M27 24L42 33L27 42Z" fill="#fff4c4" stroke="#936c26" stroke-width="1.5"/><path d="M13 20H21M13 27H18M13 34H18M13 41H18" stroke="#b7d7ef" stroke-width="2"/>',
  games: '<path d="M14 22Q17 17 23 18L29 21H36L42 18Q48 17 51 22L56 42Q58 50 51 51L41 42H24L14 51Q7 50 9 42Z" fill="url(#silver)" stroke="#506b86" stroke-width="2"/><path d="M19 29V39M14 34H24" stroke="#276ba8" stroke-width="3" stroke-linecap="round"/><circle cx="43" cy="30" r="2.5" fill="#e34f3e"/><circle cx="48" cy="36" r="2.5" fill="#54a34a"/>',
  phone: '<path d="M19 10Q14 9 12 15Q10 38 33 52Q48 60 53 50L48 40L39 38L34 43Q25 39 21 29L25 23Z" fill="url(#green)" stroke="#45763d" stroke-width="3" stroke-linejoin="round"/>',
  security: '<path d="M32 8L53 16V31Q53 47 32 57Q11 47 11 31V16Z" fill="url(#blue)" stroke="#315c8c" stroke-width="2"/><path d="M21 32L29 40L44 24" fill="none" stroke="#f7ffdf" stroke-width="5" stroke-linecap="round" stroke-linejoin="round"/>',
  calendar: '<rect x="9" y="12" width="46" height="44" rx="4" fill="#fff" stroke="#59738d" stroke-width="2"/><path d="M9 17Q9 12 14 12H50Q55 12 55 17V25H9Z" fill="url(#red)"/><path d="M20 8V18M44 8V18" stroke="#fff" stroke-width="4" stroke-linecap="round"/><path d="M18 32H24M30 32H36M42 32H48M18 42H24M30 42H36M42 42H48" stroke="#4381b6" stroke-width="4" stroke-linecap="round"/>',
  package: '<path d="M8 22L32 10L56 22V47L32 59L8 47Z" fill="url(#gold)" stroke="#966428" stroke-width="2"/><path d="M8 22L32 34L56 22M32 34V59" fill="none" stroke="#fff0b2" stroke-width="3"/><path d="M20 16L44 28" stroke="#fff7d8" stroke-width="2"/>',
  home: '<path d="M7 31L32 10L57 31L52 36L48 32V54H17V32L12 36Z" fill="url(#red)" stroke="#8c5231" stroke-width="2"/><rect x="27" y="37" width="11" height="17" fill="#fff5d9" stroke="#976c43" stroke-width="2"/><rect x="19" y="34" width="6" height="7" fill="#8fd3f1"/><rect x="41" y="34" width="6" height="7" fill="#8fd3f1"/>',
  search: '<circle cx="27" cy="27" r="17" fill="url(#blue)" stroke="#315d8b" stroke-width="3"/><circle cx="27" cy="27" r="10" fill="#eaf8ff" stroke="#87b7d7" stroke-width="2"/><path d="M40 40L55 55" stroke="#627b92" stroke-width="7" stroke-linecap="round"/><path d="M18 21Q23 15 31 19" fill="none" stroke="#fff" stroke-width="3" stroke-linecap="round"/>',
  calculator: '<rect x="14" y="7" width="36" height="51" rx="4" fill="url(#silver)" stroke="#536d86" stroke-width="2"/><rect x="20" y="13" width="24" height="10" rx="2" fill="#a8d6ec" stroke="#5c8299"/><path d="M21 31H26M32 31H37M42 31H46M21 40H26M32 40H37M42 40H46M21 49H26M32 49H37M42 49H46" stroke="#3479ad" stroke-width="4" stroke-linecap="round"/>',
  books: '<path d="M11 20L43 13L47 45L15 51Z" fill="url(#blue)" stroke="#315b86" stroke-width="2"/><path d="M20 16L51 21L47 53L16 49Z" fill="url(#red)" stroke="#925333" stroke-width="2"/><path d="M25 23L42 26M22 43L44 47" stroke="#fff1cd" stroke-width="3"/>',
  printer: '<path d="M17 12H47V26H17ZM13 25H51Q56 25 56 30V45H47V54H17V45H8V30Q8 25 13 25Z" fill="url(#silver)" stroke="#566f87" stroke-width="2"/><path d="M18 41H46V51H18Z" fill="#fff" stroke="#6a8195"/><circle cx="48" cy="32" r="2" fill="#49a347"/>',
  cloud: '<path d="M20 48Q9 48 9 38Q9 28 20 27Q23 15 35 15Q49 15 50 29Q58 31 58 39Q58 48 48 48Z" fill="url(#pane)" stroke="#6890ab" stroke-width="2"/><path d="M22 39Q29 32 36 39T50 37" fill="none" stroke="#62a34b" stroke-width="3"/>',
  camera: '<rect x="8" y="19" width="48" height="34" rx="5" fill="url(#silver)" stroke="#526f89" stroke-width="2"/><path d="M19 19L23 12H40L45 19" fill="#63839f" stroke="#526f89" stroke-width="2"/><circle cx="32" cy="36" r="12" fill="url(#blue)" stroke="#315b89" stroke-width="3"/><circle cx="32" cy="36" r="5" fill="#dff7ff"/>',
  notes: '<path d="M15 8H49V56H15Z" fill="url(#gold)" stroke="#a3782f" stroke-width="2"/><path d="M15 16H49M22 26H42M22 34H42M22 42H38" stroke="#fff6d4" stroke-width="3" stroke-linecap="round"/><path d="M41 49L54 36L58 40L45 53L39 55Z" fill="#2871ad" stroke="#19507f" stroke-width="1.5"/>',
  weather: '<circle cx="32" cy="29" r="12" fill="url(#gold)" stroke="#b07b1a" stroke-width="2"/><path d="M32 7V11M32 47V51M10 29H14M50 29H54M16 13L19 16M45 42L48 45M48 13L45 16M19 42L16 45" stroke="#e2a727" stroke-width="3" stroke-linecap="round"/><path d="M13 50Q13 42 22 42Q25 33 34 37Q41 36 44 43Q52 43 52 50Z" fill="url(#pane)" stroke="#6687a1" stroke-width="2"/>',
  map: '<path d="M8 17L25 10L41 17L56 10V48L40 56L24 49L8 56Z" fill="url(#pane)" stroke="#5a758d" stroke-width="2"/><path d="M24 13V48M41 18V53" stroke="#6285a0" stroke-width="2"/><path d="M15 38Q21 23 31 35T49 26" fill="none" stroke="#4ba24b" stroke-width="4"/><circle cx="31" cy="35" r="4" fill="#e44d38" stroke="#fff" stroke-width="2"/>',
  archive: '<path d="M9 13H55V25H9ZM14 25H50V53H14Z" fill="url(#gold)" stroke="#966a31" stroke-width="2"/><path d="M25 33H39V38H25Z" fill="#fff4cb" stroke="#8f6c35"/><path d="M18 44H46" stroke="#c48c38" stroke-width="2"/>',
  globe: '<circle cx="32" cy="32" r="23" fill="url(#blue)" stroke="#315b8a" stroke-width="2"/><path d="M9 32H55M32 9C20 18 20 46 32 55M32 9C44 18 44 46 32 55M14 21H50M14 43H50" fill="none" stroke="#dff3ff" stroke-width="2"/><path d="M17 18Q34 11 48 20" fill="none" stroke="#fff" stroke-width="3" opacity=".8"/>',
  generic: '<rect x="9" y="11" width="46" height="42" rx="5" fill="url(#pane)" stroke="#536f8a" stroke-width="2"/><path d="M10 16H54V24H10Z" fill="url(#blue)"/><path d="M18 44L28 32L38 40L45 31L53 44Z" fill="url(#green)" stroke="#4e803d" stroke-width="1.5"/><circle cx="21" cy="30" r="4" fill="url(#gold)" stroke="#ad792a"/>'
};

function svg(iconName, contents) {
  return `<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64"><defs>
<linearGradient id="pane" x2="0" y2="1"><stop stop-color="#ffffff"/><stop offset="1" stop-color="#b9d5ed"/></linearGradient>
<linearGradient id="blue" x2="0" y2="1"><stop stop-color="#69b9f4"/><stop offset=".48" stop-color="#287cce"/><stop offset="1" stop-color="#15559f"/></linearGradient>
<linearGradient id="green" x2="0" y2="1"><stop stop-color="#b9eb74"/><stop offset="1" stop-color="#4c9a3d"/></linearGradient>
<linearGradient id="folder" x2="0" y2="1"><stop stop-color="#ffe880"/><stop offset="1" stop-color="#eba827"/></linearGradient>
<linearGradient id="gold" x2="0" y2="1"><stop stop-color="#ffe58a"/><stop offset="1" stop-color="#db9823"/></linearGradient>
<linearGradient id="silver" x2="0" y2="1"><stop stop-color="#f8fbfe"/><stop offset="1" stop-color="#9eb3c7"/></linearGradient>
<linearGradient id="glass" x2="0" y2="1"><stop stop-color="#ecfcff" stop-opacity=".9"/><stop offset="1" stop-color="#8bc1d6" stop-opacity=".62"/></linearGradient>
<linearGradient id="screen" x2="0" y2="1"><stop stop-color="#71c2f0"/><stop offset="1" stop-color="#145998"/></linearGradient>
<linearGradient id="red" x2="0" y2="1"><stop stop-color="#f7915e"/><stop offset="1" stop-color="#c34828"/></linearGradient>
</defs><g>${contents}</g><path d="M12 13Q20 8 ${iconName === 'folder' ? 49 : 52} 12L50 17Q29 14 13 21Z" fill="#fff" opacity=".22"/></svg>\n`;
}

function archetypeFor(name) {
  if (/^(?:folder|downloads?$|home)$/.test(name)) return name === 'home' ? 'home' : 'folder';
  if (/^(?:desktop|desktop-symbolic|monitor|monitor-symbolic|apps|apps-symbolic)$/.test(name)) return name.startsWith('apps') ? 'application' : 'computer';
  if (/^trash/.test(name)) return 'recycle_bin';
  if (/^(?:settings|settings-symbolic|sliders|tools|contrast)$/.test(name)) return 'settings';
  if (/^(?:browser|globe|web|open-webui)$/.test(name)) return 'browser';
  if (/^(?:file|documents|book|bookshelf|writer|pdf|spreadsheet|text|markdown|json|xml|yaml|css|html|javascript|python|editor)$/.test(name)) return name === 'book' || name === 'bookshelf' ? 'books' : 'documents';
  if (/^(?:terminal|terminal-app|termix|code|code-studio|commandcode|openscad|nasscad)$/.test(name)) return 'terminal';
  if (/^(?:mail|email)$/.test(name)) return 'mail';
  if (/^(?:chat|agent-chat|live-speech|phone|phone-symbolic)$/.test(name)) return name === 'phone' || name === 'live-speech' ? 'phone' : 'chat';
  if (/^(?:network|mesh|bluetooth|cloud|backup|server|dozzle|n8n|node-red|olivetin|ha-switchboard|ruview)$/.test(name)) return name === 'cloud' || name === 'backup' ? 'cloud' : 'network';
  if (/^(?:database|analytics)$/.test(name)) return name === 'analytics' ? 'calculator' : 'database';
  if (/^(?:audio|audio-player|personal-radio|radio|sound-symbolic|music|speaker|headphones)$/.test(name)) return 'audio';
  if (/^(?:image|gallery|gallery-action-.*|camera|video|video-studio)$/.test(name)) return name === 'video' || name === 'video-studio' ? 'video' : name === 'camera' ? 'camera' : 'image';
  if (/^(?:games|chess|quakejs|romm|run|pixel|cyd-creator|easydrag|looper|galaxa-deluxe)$/.test(name)) return 'games';
  if (/^(?:calendar|weather|map|home|search|calculator|printer|archive|package|zipper|notes|folder-open|folder-plus|file-plus|download|downloads|upload|copy|clipboard|scissors|save|edit|refresh|undo|redo|check-square|chevron-.*|arrow-up|maximize|minus|square|sort|grid|columns|list|eye|eye-off|info|info-symbolic|key|help|star|heart|users|cube|screensaver-symbolic|launchpad|software-store|forms|gods-eye-view|leafy|layerling|nasscad|tresor)$/.test(name)) {
    const choose = { calendar: 'calendar', weather: 'weather', map: 'map', search: 'search', calculator: 'calculator', printer: 'printer', archive: 'archive', package: 'package', zipper: 'package', notes: 'notes' };
    return choose[name] || 'generic';
  }
  return 'generic';
}

function iconPathFor(name) {
  // Keep OS chrome in the new XP style while app logos retain their established identity.
  if (!xpSystemIcons.has(name) && papirus.icons && papirus.icons[name]) return papirus.icons[name];
  return `img/windows-xp/icons/${archetypeFor(name)}.svg`;
}

function preferredNames() {
  const source = fs.readFileSync(path.join(root, 'internal/desktop/types.go'), 'utf8');
  const block = source.match(/var desktopPreferredIconNames\s*=\s*\[\]string\s*\{([\s\S]*?)\n\}/);
  if (!block) throw new Error('Could not read desktopPreferredIconNames from internal/desktop/types.go');
  return [...block[1].matchAll(/"([^"\n]+)"/g)].map(match => match[1]);
}

function aliases() {
  const source = fs.readFileSync(path.join(root, 'internal/desktop/types.go'), 'utf8');
  const block = source.match(/var desktopIconAliases\s*=\s*map\[string\]string\s*\{([\s\S]*?)\n\}/);
  if (!block) throw new Error('Could not read desktopIconAliases from internal/desktop/types.go');
  return Object.fromEntries([...block[1].matchAll(/"([^"\n]+)"\s*:\s*"([^"\n]+)"/g)].map(match => [match[1], match[2]]).sort((a, b) => a[0].localeCompare(b[0])));
}

const names = preferredNames();
const manifest = {
  name: 'AuraGo Windows XP-inspired icon pack',
  version: 1,
  source: 'Original AuraGo SVG artwork',
  fallback: 'img/windows-xp/icons/generic.svg',
  icons: Object.fromEntries(names.map(name => [name, iconPathFor(name)])),
  aliases: aliases()
};
const outputs = new Map(Object.entries(archetypes).map(([name, contents]) => [`ui/img/windows-xp/icons/${name}.svg`, svg(name, contents)]));
outputs.set('ui/img/windows-xp/manifest.json', JSON.stringify(manifest, null, 2) + '\n');

let drift = false;
for (const [relative, contents] of outputs) {
  const file = path.join(root, relative);
  if (check) {
    if (!fs.existsSync(file) || fs.readFileSync(file, 'utf8') !== contents) {
      console.error('Stale Windows XP icon artwork:', relative);
      drift = true;
    }
  } else {
    fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, contents);
  }
}
if (drift) process.exitCode = 1;
else console.log(`${check ? 'Verified' : 'Generated'} ${names.length} preferred Windows XP icon mappings and ${outputs.size - 1} original SVG motifs.`);
