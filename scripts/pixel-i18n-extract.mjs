// Collects every Pixel UI translation key together with its English text and reports the keys that
// ui/lang/desktop/en.json does not have yet. Run it after adding or changing Pixel strings:
//
//   node scripts/pixel-i18n-extract.mjs                 report (exit 1 when keys are missing or have no English text)
//   node scripts/pixel-i18n-extract.mjs --out FILE      also write the missing keys as sorted JSON (key -> English)
//   node scripts/pixel-i18n-extract.mjs --write         insert the missing keys into ui/lang/desktop/en.json
//   node scripts/pixel-i18n-extract.mjs --verbose       list every key, conflict and exercise error
//
// Two sources are merged:
//  1. static: a small JS tokenizer over ui/js/desktop/apps/pixel*.js collects every literal 'pixel.*' key and the English
//     texts that sit next to a key (helper calls such as tx/tr, [key, English] tuples, and object literals that carry
//     labelKey/key plus fallback/label/fb/title in the same literal). Plain t('pixel.key') calls carry no text: the repo
//     lint forbids inline fallbacks there, the English text lives in en.json;
//  2. dynamic: the real scripts run in the fake DOM of scripts/pixel-test-dom.mjs; every call of the window's t() is
//     recorded (panels, menus, commands, tools, dialogs are exercised) and the registries (filters, generators, presets,
//     params, options, blend modes, menus, tools, panels) are walked for { labelKey, fallback } objects. This is what
//     finds the keys that are built at run time (gen_*, cmd_gen_*, gp_*, go_*, gpr_*, fxp_*, fxo_*, filter_*, blend_*,
//     gradient_preset_*, cutout_*, sel_*, layer_*, ...).
// Keys that are used but have no English text in code, in a runtime call or in en.json are listed separately (exit 1): add
// the key with its English text to ui/lang/desktop/en.json, then the translations with scripts/pixel-i18n-merge.mjs.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { installFakeDom, makeEl, loadPixelApp } from './pixel-test-dom.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const appDir = path.join(root, 'ui', 'js', 'desktop', 'apps');
const enPath = path.join(root, 'ui', 'lang', 'desktop', 'en.json');
const args = process.argv.slice(2);
const verbose = args.includes('--verbose');
const writeEn = args.includes('--write');
const outIndex = args.indexOf('--out');
const outFile = outIndex >= 0 ? args[outIndex + 1] : '';

// A key never ends with '_' or '.': 'pixel.blend_' is the prefix of a key that is built at run time.
const KEY_RE = /^(?:pixel|stickers|desktop)\.[A-Za-z0-9_]*[A-Za-z0-9](?:\.[A-Za-z0-9_]*[A-Za-z0-9])*$/;
// Fallbacks that are only an id ('dodge-burn', 'all') are what the code shows for tools and categories it has no text for.
const idish = text => /^[a-z0-9]+(?:[-_][a-z0-9]+)*$/.test(text);
const KIND_RANK = { 'literal pair': 3, object: 3, extra: 3, 'registry object': 2, 'runtime call': 1 };
const rankOf = pair => (idish(pair.en) ? 0 : KIND_RANK[pair.kind.startsWith('object') ? 'object' : pair.kind] || 1);
// localStorage keys that look like translation keys.
const NOT_I18N = new Set(['pixel.recent_commands', 'pixel.filter_favorites', 'pixel.swatches', 'pixel.dock_split']);
// Keys the code builds in ways no scan can see (kept here on purpose, with their English text).
const EXTRA = { 'desktop.file_dialog_ora': 'OpenRaster project (.ora)' };

// --- static scan ----------------------------------------------------------------------------------------------------

function decodeEscape(src, i) {
    const c = src[i + 1];
    const simple = { n: '\n', t: '\t', r: '\r', b: '\b', f: '\f', v: '\v', '0': '\0' };
    if (c in simple) return [simple[c], 2];
    if (c === 'x') return [String.fromCharCode(parseInt(src.slice(i + 2, i + 4), 16)), 4];
    if (c === 'u') {
        if (src[i + 2] === '{') {
            const end = src.indexOf('}', i);
            return [String.fromCodePoint(parseInt(src.slice(i + 3, end), 16)), end - i + 1];
        }
        return [String.fromCharCode(parseInt(src.slice(i + 2, i + 6), 16)), 6];
    }
    if (c === '\n') return ['', 2];
    return [c, 2];
}

function tokenize(src) {
    const toks = [];
    let i = 0, line = 1;
    const regexAllowedAfter = prev => !prev
        || (prev.t === 'p' && ![')', ']', '}'].includes(prev.v))
        || prev.t === 'op'
        || (prev.t === 'id' && /^(?:return|typeof|case|in|of|delete|void|throw|new|else|do|yield|await)$/.test(prev.v));

    function readString(quote) {
        const startLine = line;
        let text = '';
        i++;
        while (i < src.length && src[i] !== quote) {
            if (src[i] === '\\') { const [s, n] = decodeEscape(src, i); text += s; if (src[i + 1] === '\n') line++; i += n; continue; }
            if (src[i] === '\n') break;
            text += src[i++];
        }
        i++;
        toks.push({ t: 'str', v: text, line: startLine });
    }

    function scan(stopOnBrace) {
        let depth = 0;
        while (i < src.length) {
            const c = src[i];
            if (c === '\n') { line++; i++; continue; }
            if (/\s/.test(c)) { i++; continue; }
            if (c === '/' && src[i + 1] === '/') { while (i < src.length && src[i] !== '\n') i++; continue; }
            if (c === '/' && src[i + 1] === '*') {
                const e = src.indexOf('*/', i + 2);
                const end = e < 0 ? src.length : e + 2;
                line += (src.slice(i, end).match(/\n/g) || []).length;
                i = end;
                continue;
            }
            if (c === '"' || c === "'") { readString(c); continue; }
            if (c === '`') {
                const startLine = line;
                let text = '', hasExpr = false;
                i++;
                while (i < src.length && src[i] !== '`') {
                    if (src[i] === '\\') { const [s, n] = decodeEscape(src, i); text += s; i += n; continue; }
                    if (src[i] === '$' && src[i + 1] === '{') { hasExpr = true; i += 2; scan(true); continue; }
                    if (src[i] === '\n') line++;
                    text += src[i++];
                }
                i++;
                toks.push(hasExpr ? { t: 'tpl', line: startLine } : { t: 'str', v: text, line: startLine });
                continue;
            }
            if (c === '/' && regexAllowedAfter(toks[toks.length - 1])) {
                i++;
                let inClass = false;
                while (i < src.length && (src[i] !== '/' || inClass)) {
                    if (src[i] === '\\') i++;
                    else if (src[i] === '[') inClass = true;
                    else if (src[i] === ']') inClass = false;
                    else if (src[i] === '\n') break;
                    i++;
                }
                i++;
                while (i < src.length && /[a-z]/i.test(src[i])) i++;
                toks.push({ t: 're', line });
                continue;
            }
            if (/[A-Za-z_$]/.test(c)) {
                let j = i + 1;
                while (j < src.length && /[\w$]/.test(src[j])) j++;
                toks.push({ t: 'id', v: src.slice(i, j), line });
                i = j;
                continue;
            }
            if (/[0-9]/.test(c)) {
                let j = i + 1;
                while (j < src.length && /[\w.]/.test(src[j])) j++;
                toks.push({ t: 'num', v: src.slice(i, j), line });
                i = j;
                continue;
            }
            if (c === '{') { depth++; toks.push({ t: 'p', v: c, line }); i++; continue; }
            if (c === '}') {
                if (stopOnBrace && depth === 0) { i++; return; }
                depth--;
                toks.push({ t: 'p', v: c, line });
                i++;
                continue;
            }
            if ('()[],:;.?'.includes(c)) { toks.push({ t: 'p', v: c, line }); i++; continue; }
            let j = i + 1;
            while (j < src.length && '+-*%<>=!&|^~/'.includes(src[j]) && '+-*%<>=!&|^~/'.includes(c)) j++;
            toks.push({ t: 'op', v: src.slice(i, j), line });
            i = j;
        }
    }
    scan(false);
    return toks;
}

const isP = (tok, v) => !!tok && tok.t === 'p' && tok.v === v;
const isEnglish = tok => !!tok && tok.t === 'str' && tok.v.trim() !== '' && !KEY_RE.test(tok.v);

// Properties of an object literal whose value is a plain string: { name: value }.
function stringProps(toks, open, close) {
    const props = {};
    let depth = 0;
    for (let k = open + 1; k < close; k++) {
        const tok = toks[k];
        if (tok.t === 'p' && '([{'.includes(tok.v)) depth++;
        else if (tok.t === 'p' && ')]}'.includes(tok.v)) depth--;
        else if (depth === 0 && (tok.t === 'id' || tok.t === 'str') && isP(toks[k + 1], ':') && toks[k + 2] && toks[k + 2].t === 'str'
            && (isP(toks[k + 3], ',') || isP(toks[k + 3], '}'))) {
            props[tok.v] = toks[k + 2].v;
        }
    }
    return props;
}

const ENGLISH_PROPS = {
    labelKey: ['labelFallback', 'fallback', 'label', 'fb', 'name', 'text'],
    key: ['fb', 'title', 'fallback', 'label', 'text']
};

function scanFile(file) {
    const toks = tokenize(fs.readFileSync(file, 'utf8'));
    const rel = path.basename(file);
    const pairs = [], uses = [];
    const stack = [], closeOf = new Map(), objectOf = new Map();
    toks.forEach((tok, k) => {
        if (tok.t !== 'p') return;
        if ('([{'.includes(tok.v)) stack.push(k);
        else if (')]}'.includes(tok.v)) { const open = stack.pop(); if (open !== undefined) closeOf.set(open, k); }
    });
    stack.length = 0;
    toks.forEach((tok, k) => {
        if (tok.t === 'p' && '([{'.includes(tok.v)) stack.push(k);
        else if (tok.t === 'p' && ')]}'.includes(tok.v)) stack.pop();
        if (tok.t !== 'str' || !KEY_RE.test(tok.v)) return;
        if (toks[k + 1] && toks[k + 1].t === 'op' && toks[k + 1].v === '+') return; // 'pixel.cat_' + id
        uses.push({ key: tok.v, file: rel, line: tok.line });
        // key, 'English' followed by ) , ] or }
        if (isP(toks[k + 1], ',') && isEnglish(toks[k + 2]) && (isP(toks[k + 3], ')') || isP(toks[k + 3], ',') || isP(toks[k + 3], ']') || isP(toks[k + 3], '}'))) {
            pairs.push({ key: tok.v, en: toks[k + 2].v, file: rel, line: tok.line, kind: 'literal pair' });
        }
        // labelKey: 'pixel.x' / key: 'pixel.x' inside an object literal that also carries the English text
        const name = toks[k - 2];
        if (isP(toks[k - 1], ':') && name && (name.t === 'id' || name.t === 'str') && (name.v === 'labelKey' || name.v === 'key' || /Key$/.test(name.v))) {
            const open = stack[stack.length - 1];
            if (open !== undefined && toks[open].v === '{' && closeOf.has(open)) {
                if (!objectOf.has(open)) objectOf.set(open, stringProps(toks, open, closeOf.get(open)));
                const props = objectOf.get(open);
                const prefix = name.v.replace(/Key$/, '');
                const names = ENGLISH_PROPS[name.v] || [prefix + 'Fallback', prefix];
                const found = names.find(n => typeof props[n] === 'string' && props[n].trim() !== '' && !KEY_RE.test(props[n]));
                if (found) pairs.push({ key: tok.v, en: props[found], file: rel, line: tok.line, kind: 'object ' + name.v + '+' + found });
            }
        }
    });
    return { pairs, uses };
}

function scanStatic() {
    const files = fs.readdirSync(appDir).filter(name => /^pixel.*\.js$/.test(name) && !/^pixel-(?:lib-|worker)/.test(name)).sort();
    const pairs = [], uses = [];
    files.forEach(name => {
        const result = scanFile(path.join(appDir, name));
        pairs.push(...result.pairs);
        uses.push(...result.uses);
    });
    return { pairs, uses, files };
}

// --- dynamic scan ---------------------------------------------------------------------------------------------------

async function scanRuntime() {
    installFakeDom();
    const pairs = [], uses = [], warnings = [];
    const record = (key, en, kind) => {
        if (typeof key !== 'string' || !KEY_RE.test(key)) return;
        uses.push({ key, file: '(runtime)', line: 0 });
        if (typeof en === 'string' && en.trim() !== '' && !KEY_RE.test(en)) pairs.push({ key, en, file: '(runtime)', line: 0, kind });
    };
    const realError = console.error;
    console.error = (...a) => { warnings.push('console.error: ' + a.map(x => (x && x.message) || String(x)).join(' ')); };
    const restoreConsole = () => { console.error = realError; };
    const Pixel = loadPixelApp();
    const original = Pixel.makeTranslator;
    Pixel.makeTranslator = base => {
        const inner = original(base);
        return function t(key, second, third) {
            record(key, typeof second === 'string' ? second : undefined, 'runtime call');
            return inner(key, second, third);
        };
    };
    let rt = null;
    Pixel.registerModule('i18n-capture', r => { rt = r; });
    let menus = [];
    const host = makeEl('div');
    window.PixelApp.render(host, 'i18n', {
        esc: v => String(v), t: key => key, api: async () => ({}), iconMarkup: () => '', notify: () => {},
        setWindowMenus: (id, m) => { menus = m; }, readonly: false
    });
    if (!rt) { restoreConsole(); throw new Error('no runtime captured'); }
    const attempt = (label, fn) => {
        try { return fn(); } catch (err) { warnings.push(label + ': ' + ((err && err.message) || err)); return undefined; }
    };

    // Objects that carry labelKey + an English text anywhere in the registries and the window state.
    const seen = new WeakSet();
    const englishOf = obj => ['fallback', 'label', 'fb', 'name', 'text', 'title'].map(n => obj[n]).find(v => typeof v === 'string' && v.trim() !== '' && !KEY_RE.test(v));
    function walk(value, depth) {
        if (!value || typeof value !== 'object' || depth > 7 || seen.has(value)) return;
        if (value.tagName || typeof value.nodeType === 'number' || value === globalThis || value === host) return;
        seen.add(value);
        let keys = [];
        try { keys = Object.keys(value); } catch (_) { return; }
        if (typeof value.labelKey === 'string') record(value.labelKey, englishOf(value), 'registry object');
        if (typeof value.key === 'string' && KEY_RE.test(value.key)) record(value.key, englishOf(value), 'registry object');
        keys.forEach(k => {
            let child;
            try { child = value[k]; } catch (_) { return; }
            if (typeof child === 'string' && KEY_RE.test(child) && /Key$/.test(k)) record(child, englishOf(value), 'registry object');
            else walk(child, depth + 1);
        });
    }

    attempt('newBlankCanvas', () => rt.newBlankCanvas(64, 48));
    attempt('listTools', () => Pixel.listTools().forEach(def => {
        walk(def, 0);
        attempt('toolLabel ' + def.id, () => rt.toolLabel(def.id));
        attempt('tool options ' + def.id, () => { rt.setActiveTool(def.id); if (rt.activeTool !== def.id) rt.setActiveTool(def.id); });
        attempt('tool options html ' + def.id, () => typeof def.options === 'function' && def.options(rt));
    }));
    attempt('deactivate tool', () => { if (rt.activeTool) rt.setActiveTool(rt.activeTool); });
    attempt('listPanels', () => Pixel.listPanels().forEach(def => {
        walk(def, 0);
        attempt('panel build ' + def.id, () => def.build(rt));
        attempt('showPanel ' + def.id, () => rt.showPanel(def.id));
    }));
    attempt('menus', () => { rt.rebuildMenus(); walk(menus, 0); walk(Pixel.composeMenus(rt), 0); });
    attempt('commands', () => walk(rt.listCommands(), 0));
    attempt('tool rail', () => rt.buildToolRailHTML());
    attempt('shortcuts modal', () => rt.showShortcutsModal());
    attempt('new image dialog', () => rt.showNewImageDialog());
    attempt('options bar', () => rt.renderOptionsBar());
    attempt('layers panel', () => { rt.addLayer(); rt.refreshLayerPanel(); });
    attempt('registry data', () => walk(Pixel, 0));
    attempt('window state', () => walk(rt, 0));
    attempt('lib data', () => walk(globalThis.AuraPixelLib, 0));
    restoreConsole();
    return { pairs, uses, warnings };
}

// --- merge and report -----------------------------------------------------------------------------------------------

// Best text: highest source rank (literal pairs and object literals beat runtime calls, bare ids come last), then the
// most frequent, then the first found.
function pickEnglish(candidates) {
    const best = Math.max(...candidates.map(rankOf));
    const counts = new Map();
    candidates.filter(c => rankOf(c) === best).forEach(c => counts.set(c.en, (counts.get(c.en) || 0) + 1));
    return Array.from(counts).sort((a, b) => b[1] - a[1])[0][0];
}

function applyToEnJson(missing) {
    let lines = fs.readFileSync(enPath, 'utf8').split('\n');
    const parse = line => { const m = /^  "((?:[^"\\]|\\.)*)": /.exec(line); return m ? JSON.parse('"' + m[1] + '"') : null; };
    Object.keys(missing).sort().forEach(key => {
        const entries = lines.map(parse);
        let best = -1, bestLen = -1;
        entries.forEach((existing, index) => {
            if (existing === null) return;
            let n = 0;
            while (n < existing.length && n < key.length && existing[n] === key[n]) n++;
            if (n > bestLen) { bestLen = n; best = index; }
        });
        const prefix = key.slice(0, Math.max(bestLen, 1));
        let from = best, to = best;
        while (from > 0 && entries[from - 1] !== null && entries[from - 1].startsWith(prefix)) from--;
        while (to < entries.length - 1 && entries[to + 1] !== null && entries[to + 1].startsWith(prefix)) to++;
        let at = from;
        for (let k = from; k <= to; k++) if (entries[k] < key) at = k + 1;
        if (at > to + 1) at = to + 1;
        lines.splice(at, 0, '  ' + JSON.stringify(key) + ': ' + JSON.stringify(missing[key]) + ',');
    });
    fs.writeFileSync(enPath, lines.join('\n'));
    JSON.parse(fs.readFileSync(enPath, 'utf8'));
}

const staticScan = scanStatic();
const runtimeScan = await scanRuntime();
const en = JSON.parse(fs.readFileSync(enPath, 'utf8'));

const pairs = staticScan.pairs.concat(runtimeScan.pairs);
const usedKeys = new Set(staticScan.uses.concat(runtimeScan.uses).map(u => u.key).filter(key => !NOT_I18N.has(key)));
Object.keys(EXTRA).forEach(key => { usedKeys.add(key); pairs.push({ key, en: EXTRA[key], file: '(extra)', line: 0, kind: 'extra' }); });
const byKey = new Map();
pairs.forEach(p => { if (!byKey.has(p.key)) byKey.set(p.key, []); byKey.get(p.key).push(p); });

const missing = {}, noEnglish = [], conflicts = [], differs = [];
Array.from(usedKeys).sort().forEach(key => {
    const candidates = byKey.get(key) || [];
    const distinct = new Set(candidates.map(c => c.en));
    const real = new Set(candidates.filter(c => rankOf(c) > 0).map(c => c.en));
    if (real.size > 1 && !(key in en)) conflicts.push({ key, texts: Array.from(real), where: candidates.filter(c => rankOf(c) > 0).map(c => c.file + ':' + c.line).filter((w, i, all) => all.indexOf(w) === i).slice(0, 4) });
    if (key in en) {
        if (real.size && !real.has(en[key])) differs.push({ key, en: en[key], code: Array.from(real)[0] });
        return;
    }
    if (!candidates.length) noEnglish.push(key);
    else missing[key] = pickEnglish(candidates);
});

const sortedMissing = Object.fromEntries(Object.keys(missing).sort().map(key => [key, missing[key]]));
console.log('pixel-i18n-extract: ' + staticScan.files.length + ' scripts, ' + usedKeys.size + ' keys used (' + staticScan.pairs.length + ' static pairs, ' + runtimeScan.pairs.length + ' runtime pairs)');
console.log('  already in en.json: ' + (usedKeys.size - Object.keys(missing).length - noEnglish.length) + ', missing with English: ' + Object.keys(missing).length + ', missing without English: ' + noEnglish.length);
if (conflicts.length) {
    console.log('  MISSING KEYS WITH SEVERAL ENGLISH TEXTS (' + conflicts.length + '; make the call sites agree):');
    conflicts.forEach(c => console.log('    ' + c.key + ': ' + c.texts.map(t => JSON.stringify(t)).join(' | ') + '  [' + c.where.join(', ') + ']'));
}
if (differs.length && verbose) {
    console.log('  keys whose code text differs from en.json (' + differs.length + '):');
    differs.forEach(d => console.log('    ' + d.key + ': en.json ' + JSON.stringify(d.en) + ' / code ' + JSON.stringify(d.code)));
} else if (differs.length) {
    console.log('  ' + differs.length + ' keys have another English text in code than in en.json (--verbose lists them)');
}
if (verbose) runtimeScan.warnings.forEach(w => console.log('  exercise: ' + w));
if (noEnglish.length) {
    console.log('  USED WITHOUT ANY ENGLISH TEXT (add a fallback at the call site):');
    noEnglish.forEach(key => console.log('    ' + key));
}
if (verbose || (!writeEn && !outFile)) {
    Object.keys(sortedMissing).forEach(key => console.log('  + ' + key + ' = ' + JSON.stringify(sortedMissing[key])));
}
if (outFile) {
    fs.writeFileSync(path.resolve(outFile), JSON.stringify(sortedMissing, null, 2) + '\n');
    console.log('  wrote ' + Object.keys(sortedMissing).length + ' keys to ' + outFile);
}
if (writeEn && Object.keys(sortedMissing).length) {
    applyToEnJson(sortedMissing);
    console.log('  added ' + Object.keys(sortedMissing).length + ' keys to ' + path.relative(root, enPath));
}
process.exitCode = noEnglish.length || (!writeEn && Object.keys(sortedMissing).length) ? 1 : 0;
setTimeout(() => process.exit(process.exitCode), 20);
