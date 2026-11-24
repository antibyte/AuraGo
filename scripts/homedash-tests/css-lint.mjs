// HomeDash stylesheet and source guard (visual part 9.4). lintCss/lintJs return readable violations; [] is clean.
export const TOKEN_BLOCK = /^(?:\.hd-root(?:\[data-hd-(?:palette|scheme)="\w+"\])?|\[data-hd-family="\w+"\])$/;
export const MOTION_GATE = '.hd-root:not([data-hd-motion="reduced"]):not([data-hd-motion="none"])';
const COLOUR = /#[0-9a-fA-F]{3,8}\b|\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch|color)\(/i;
const NAMED = /(^|[\s,(])(white|black|red|green|blue|gray|grey|silver|orange|yellow|purple|pink|navy|teal|maroon|olive|lime|aqua|fuchsia|gold|brown|cyan|magenta)(?=$|[\s,)])/i;
const COLOR_PROPS = /^(color|background|background-color|background-image|border|border-(top|right|bottom|left)|border(-(top|right|bottom|left))?-color|outline|outline-color|fill|stroke|box-shadow|text-shadow|caret-color|accent-color|text-decoration-color|column-rule|stop-color|flood-color)$/;
const MOTION = /^(transition|transition-property|transition-duration|animation|animation-name|animation-duration)$/;
export function leafRules(text) { const rules = [], stack = []; let buf = '';
  for (const ch of text) { if (ch === '{') { stack.push(buf.trim()); buf = ''; } else if (ch === '}') { const head = stack.pop() || ''; if (buf.trim()) rules.push({ selector: head, body: buf.trim(), context: [...stack] }); buf = ''; } else buf += ch; }
  return rules; }
export function splitSelectors(sel) { const out = []; let depth = 0, cur = '';
  for (const ch of sel) { if (ch === '(') depth++; if (ch === ')') depth--; if (ch === ',' && !depth) { out.push(cur.trim()); cur = ''; } else cur += ch; }
  if (cur.trim()) out.push(cur.trim()); return out; }
const decls = body => body.split(';').map(r => { const i = r.indexOf(':'); return i < 0 ? null : [r.slice(0, i).trim().toLowerCase(), r.slice(i + 1).trim()]; }).filter(d => d && d[0]);
export function lintCss(text, { file = 'css', kind = 'part' } = {}) {
  const v = [], css = text.replace(/\/\*[\s\S]*?\*\//g, ''), lines = text.split('\n').length;
  if (kind !== 'lab' && lines > 900) v.push(`${file}: ${lines} lines, want at most 900`);
  if (/light-dark\(/i.test(css)) v.push(`${file}: uses light-dark(); select scheme values with [data-hd-scheme]`);
  if (/\[data-theme\b/.test(css)) v.push(`${file}: keys on [data-theme]; read the bridge tokens`);
  if (/(^|[^a-z-])cc-[a-z]/i.test(css)) v.push(`${file}: uses the cc- prefix`);
  for (const m of css.matchAll(/url\(\s*(?:"([^"]*)"|'([^']*)'|([^)\s]*))\s*\)/g)) { const u = m[1] ?? m[2] ?? m[3] ?? '';
    const rest = u.slice(18).replace(/http:\/\/www\.w3\.org\/2000\/svg/g, '');
    if (!(u.startsWith('/fonts/') || (u.startsWith('data:image/svg+xml') && u.length <= 4096 && !/script|foreignObject|https?:|\/\//i.test(rest)))) v.push(`${file}: url(${u.slice(0, 60)}) is not allowed`); }
  for (const rule of leafRules(css)) {
    const sel = rule.selector, parts = splitSelectors(sel), ds = decls(rule.body);
    if (sel.startsWith('@font-face')) { const f = ds.find(d => d[0] === 'font-family'); if (!f || !/^["']HomeDash /.test(f[1])) v.push(`${file}: @font-face family ${f ? f[1] : '(none)'} must start with "HomeDash "`); continue; }
    if (rule.context.some(h => h.startsWith('@keyframes'))) { for (const [n] of ds) if (n !== 'transform' && n !== 'opacity') v.push(`${file}: keyframes animate ${n}; only transform and opacity`); continue; }
    if (kind !== 'lab') for (const p of parts) for (const m of p.replace(/\[[^\]]*\]/g, '').matchAll(/\.(-?[_a-zA-Z][\w-]*)/g)) if (!m[1].startsWith('hd-')) v.push(`${file}: class .${m[1]} in "${p}" does not start with hd-`);
    const tokenBlock = parts.every(p => TOKEN_BLOCK.test(p));
    for (const [name, value] of ds) {
      if (/!important/i.test(value) && !parts.every(p => /\[hidden\]$/.test(p))) v.push(`${file}: !important in ${sel} { ${name} }`);
      if (name.startsWith('--')) {
        if (kind !== 'lab' && !name.startsWith('--hd-')) v.push(`${file}: custom property ${name} outside the --hd- namespace`);
        if ((COLOUR.test(value) || NAMED.test(value)) && !tokenBlock) v.push(`${file}: literal colour in ${name} outside a token block (${sel})`);
        continue;
      }
      if (COLOUR.test(value)) v.push(`${file}: literal colour in ${sel} { ${name}: ${value} }`);
      if (COLOR_PROPS.test(name) && NAMED.test(value)) v.push(`${file}: named colour in ${sel} { ${name}: ${value} }`);
      if (name === 'text-transform' && /uppercase/i.test(value)) v.push(`${file}: uppercase text in ${sel}`);
      if (MOTION.test(name) && !/^(none|0m?s)\b/i.test(value) && !(rule.context.some(h => /prefers-reduced-motion\s*:\s*no-preference/.test(h)) && parts.every(p => p.includes(MOTION_GATE)))) v.push(`${file}: ungated motion in ${sel} { ${name} }`);
      if (name === 'transition' || name === 'transition-property') {
        const props = name === 'transition' ? splitSelectors(value).map(s => s.trim().split(/\s+/)[0]) : value.split(',').map(s => s.trim());
        for (const p of props) if (p !== 'none' && !['transform', 'opacity', 'stroke-dasharray'].includes(p)) v.push(`${file}: transition of ${p}; only transform, opacity and stroke-dasharray`);
      }
      if (/^(-webkit-)?backdrop-filter$/.test(name) && value !== 'none') {
        if (!parts.every(p => /\.hd-(frame|sheet)(?=[.:[]|$)[^\s>+~]*$/.test(p))) v.push(`${file}: backdrop-filter outside .hd-frame and .hd-sheet (${sel})`);
        for (const m of value.matchAll(/blur\(\s*([\d.]+)px/g)) if (Number(m[1]) > 24) v.push(`${file}: blur ${m[1]}px over 24px`);
      }
      if (kind === 'family' && /^(stroke-width|border-radius|box-shadow)$/.test(name) && !/var\(--hd-/.test(value) && !/^(0|none)$/.test(value)) v.push(`${file}: ${name} in ${sel} must use a --hd- token`);
    }
  }
  return v;
}
const JS_RULES = [[/\.innerHTML\b/, 'innerHTML'], [/\.outerHTML\b/, 'outerHTML'], [/insertAdjacentHTML/, 'insertAdjacentHTML'], [/\beval\s*\(/, 'eval'], [/new\s+Function\s*\(/, 'new Function'],
  [/document\.write/, 'document.write'], [/(^|[^.\w])(alert|confirm|prompt)\s*\(/, 'browser dialogs'], [/setAttribute\(\s*['"]style['"]/, "setAttribute('style'"], [/\.style\.cssText/, 'style.cssText'],
  [/\.style\s*=[^=]/, 'style assignment'], [/setAttribute\(\s*['"]on[a-z]+['"]/, 'inline event handler attribute']];
export function lintJs(text, file) { return JS_RULES.filter(([re]) => re.test(text)).map(([, what]) => `${file}: uses ${what}`); }
