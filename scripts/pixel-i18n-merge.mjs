// Merges the translated Pixel strings into ui/lang/desktop/<lang>.json with a minimal text diff.
//
//   node scripts/pixel-i18n-merge.mjs                 merge all 15 languages (and sync en.json with the English sources)
//   node scripts/pixel-i18n-merge.mjs --dry-run       validate and report, write nothing
//   node scripts/pixel-i18n-merge.mjs --lang de,fr    only these languages
//   node scripts/pixel-i18n-merge.mjs --parts-dir D --lang-dir D2   read the parts / lang files from other folders
//
// Inputs (docs/superpowers/plans/i18n): src-part1..4.json (key -> English) and <lang>.part1..4.json (key -> translation).
// For every language file:
//  - a key that already exists keeps its line and position; only the value text is replaced (nothing when it is equal);
//  - new keys are appended before the closing brace, sorted among themselves, the comma of the former last entry is
//    fixed; no other line is touched and the newline style of the file (LF or CRLF) is kept.
// Nothing is written unless every part of every selected language passes: all four parts present and valid JSON, key set
// equal to the source part, no empty value, same {x} / {{x}} placeholders as the English. The run is idempotent.
// en.json is only synced in place (the four reworded filter names); a source key that en.json lacks is an error there
// (add keys to en.json with scripts/pixel-i18n-extract.mjs --write).
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const LANGS = ['cs', 'da', 'de', 'el', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const PARTS = [1, 2, 3, 4];
// Existing entries that are harmonised with the new translations (the key must already exist in the file).
const OVERRIDES = {
    sv: { 'pixel.highlights': 'Högdagrar' }
};

const args = process.argv.slice(2);
const flagValue = name => { const i = args.indexOf(name); return i >= 0 ? args[i + 1] : ''; };
const dryRun = args.includes('--dry-run');
const partsDir = path.resolve(flagValue('--parts-dir') || path.join(root, 'docs', 'superpowers', 'plans', 'i18n'));
const langDir = path.resolve(flagValue('--lang-dir') || path.join(root, 'ui', 'lang', 'desktop'));
const only = flagValue('--lang');
const selected = only ? only.split(',').map(s => s.trim()).filter(Boolean) : LANGS.concat('en');
const unknown = selected.filter(l => l !== 'en' && !LANGS.includes(l));
if (unknown.length) { console.error('unknown language: ' + unknown.join(', ')); process.exit(1); }

const errors = [];
const fail = message => errors.push(message);

function readPart(name) {
    const file = path.join(partsDir, name);
    let data;
    try { data = JSON.parse(fs.readFileSync(file, 'utf8')); } catch (err) { fail(name + ': ' + (err.code === 'ENOENT' ? 'missing' : 'invalid JSON (' + err.message + ')')); return null; }
    if (!data || typeof data !== 'object' || Array.isArray(data) || Object.values(data).some(v => typeof v !== 'string')) {
        fail(name + ': not a flat key -> string object');
        return null;
    }
    return data;
}

const placeholders = text => (text.match(/\{\{[^{}]*\}\}|\{[^{}]*\}/g) || []).sort().join(' ');
const sample = list => list.slice(0, 5).join(', ') + (list.length > 5 ? ' (+' + (list.length - 5) + ' more)' : '');

// --- sources --------------------------------------------------------------------------------------------------------

const source = {};
const sourceParts = {};
PARTS.forEach(i => {
    const data = readPart('src-part' + i + '.json');
    if (!data) return;
    sourceParts[i] = data;
    Object.keys(data).forEach(key => {
        if (key in source) fail('src-part' + i + '.json: key ' + key + ' also in another part');
        source[key] = data[key];
    });
});

// --- translations per language --------------------------------------------------------------------------------------

function loadTranslations(lang) {
    if (lang === 'en') return Object.assign({}, source);
    const merged = {};
    PARTS.forEach(i => {
        const name = lang + '.part' + i + '.json';
        const data = readPart(name);
        const src = sourceParts[i];
        if (!data || !src) return;
        const missing = Object.keys(src).filter(k => !(k in data));
        const extra = Object.keys(data).filter(k => !(k in src));
        if (missing.length) fail(name + ': missing keys ' + sample(missing));
        if (extra.length) fail(name + ': keys not in the source ' + sample(extra));
        const empty = Object.keys(src).filter(k => k in data && !data[k].trim());
        if (empty.length) fail(name + ': empty values ' + sample(empty));
        const drift = Object.keys(src).filter(k => k in data && placeholders(src[k]) !== placeholders(data[k]));
        if (drift.length) fail(name + ': placeholders differ from the English in ' + sample(drift.map(k => k + ' [' + placeholders(src[k]) + ' vs ' + placeholders(data[k]) + ']')));
        Object.keys(src).forEach(k => { if (k in data) merged[k] = data[k]; });
    });
    return merged;
}

// --- minimal text merge ---------------------------------------------------------------------------------------------

const ENTRY = /^(  ")((?:[^"\\]|\\.)*)(": )("(?:[^"\\]|\\.)*")(,?)$/;

function mergeText(text, translations, allowAppend, mustExist) {
    const eol = text.includes('\r\n') ? '\r\n' : '\n';
    const lines = text.split(eol);
    if (lines.some(line => line.includes('\n') || line.includes('\r'))) throw new Error('mixed line endings');
    if (lines[0] !== '{') throw new Error('does not start with a lone "{"');
    let end = lines.length - 1;
    while (end > 0 && lines[end] === '') end--;
    if (lines[end] !== '}') throw new Error('does not end with a lone "}"');
    const index = new Map();
    for (let i = 1; i < end; i++) {
        const m = ENTRY.exec(lines[i]);
        if (!m) throw new Error('line ' + (i + 1) + ' is not a plain "key": "value" entry');
        const key = JSON.parse('"' + m[2] + '"');
        if (index.has(key)) throw new Error('duplicate key ' + key);
        index.set(key, i);
    }
    if (end > 1 && lines[end - 1].endsWith(',')) throw new Error('trailing comma before the closing brace');

    const keys = Object.keys(translations);
    mustExist.forEach(key => { if (!index.has(key)) throw new Error('key to harmonise is missing: ' + key); });
    let replaced = 0, same = 0;
    keys.forEach(key => {
        if (!index.has(key)) return;
        const i = index.get(key);
        const m = ENTRY.exec(lines[i]);
        if (JSON.parse(m[4]) === translations[key]) { same++; return; }
        lines[i] = m[1] + m[2] + m[3] + JSON.stringify(translations[key]) + m[5];
        replaced++;
    });
    const fresh = keys.filter(key => !index.has(key)).sort((a, b) => (a < b ? -1 : a > b ? 1 : 0));
    if (fresh.length) {
        if (!allowAppend) throw new Error('keys missing in the file: ' + sample(fresh));
        if (end > 1) lines[end - 1] += ',';
        lines.splice(end, 0, ...fresh.map((key, n) => '  ' + JSON.stringify(key) + ': ' + JSON.stringify(translations[key]) + (n < fresh.length - 1 ? ',' : '')));
    }
    const result = lines.join(eol);

    // Self-check: the result parses, carries every translation and kept everything else.
    const before = JSON.parse(text), after = JSON.parse(result);
    if (Object.keys(after).length !== Object.keys(before).length + fresh.length) throw new Error('key count after the merge is wrong');
    Object.keys(before).forEach(key => {
        const expected = key in translations ? translations[key] : before[key];
        if (after[key] !== expected) throw new Error('self-check failed for ' + key);
    });
    fresh.forEach(key => { if (after[key] !== translations[key]) throw new Error('self-check failed for ' + key); });
    return { text: result, replaced, same, appended: fresh.length, eol: eol === '\r\n' ? 'CRLF' : 'LF' };
}

// --- run ------------------------------------------------------------------------------------------------------------

const plans = [];
selected.forEach(lang => {
    const translations = loadTranslations(lang);
    if (errors.length) return;
    const file = path.join(langDir, lang + '.json');
    let text;
    try { text = fs.readFileSync(file, 'utf8'); } catch (err) { fail(lang + '.json: ' + err.message); return; }
    const overrides = OVERRIDES[lang] || {};
    try {
        const result = mergeText(text, Object.assign({}, translations, overrides), lang !== 'en', Object.keys(overrides));
        plans.push({ lang, file, original: text, ...result, overrides: Object.keys(overrides).length });
    } catch (err) {
        fail(lang + '.json: ' + err.message);
    }
});

if (errors.length) {
    console.error('pixel-i18n-merge: refused, nothing written:');
    errors.forEach(message => console.error('  ' + message));
    process.exit(1);
}

let written = 0;
plans.forEach(plan => {
    const changed = plan.text !== plan.original;
    if (changed && !dryRun) { fs.writeFileSync(plan.file, plan.text); written++; }
    console.log('  ' + plan.lang.padEnd(3) + ' ' + plan.eol.padEnd(4) + ' replaced ' + String(plan.replaced).padStart(2) + ', unchanged ' + String(plan.same).padStart(2)
        + ', appended ' + String(plan.appended).padStart(4) + (plan.overrides ? ' (incl. ' + plan.overrides + ' harmonised)' : '') + (changed ? '' : '  [no change]'));
});
console.log('pixel-i18n-merge: ' + plans.length + ' files checked, ' + (dryRun ? 'dry run, nothing written' : written + ' written'));
