/* Aura Drift - guard against undefined constants.
 *
 * A constant that is used but never declared only fails when the code path runs,
 * and a throwing frame takes the whole 3D renderer down (the shell falls back to
 * Canvas 2D). The boost glow of the HUD used a bare TAU that no module scope
 * had, so the first nitro press killed the 3D view.
 *
 * This check reads every Aura Drift module and reports identifiers that look
 * like constants, are used, but are declared nowhere in their own file.
 */
import { readFileSync, readdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = dirname(fileURLToPath(import.meta.url));
/* An optional first argument points at another apps folder (used by the checks
 * themselves to run the scan against a known-bad copy). */
const APPS = process.argv[2] ? process.argv[2] : join(HERE, '..', 'ui', 'js', 'desktop', 'apps');

const GLOBALS = new Set([
    'Math', 'JSON', 'window', 'document', 'console', 'Number', 'Object', 'Array', 'String', 'Boolean',
    'Infinity', 'NaN', 'undefined', 'Promise', 'Date', 'RegExp', 'Error', 'TypeError', 'RangeError',
    'Function', 'Symbol', 'Map', 'Set', 'WeakMap', 'WeakSet', 'Proxy', 'Reflect', 'BigInt',
    'isFinite', 'isNaN', 'parseFloat', 'parseInt', 'setTimeout', 'clearTimeout', 'setInterval',
    'clearInterval', 'requestAnimationFrame', 'cancelAnimationFrame', 'performance', 'fetch',
    'ArrayBuffer', 'DataView', 'Float32Array', 'Float64Array', 'Int8Array', 'Int16Array', 'Int32Array',
    'Uint8Array', 'Uint8ClampedArray', 'Uint16Array', 'Uint32Array', 'TextDecoder', 'TextEncoder',
    'Image', 'ImageData', 'OffscreenCanvas', 'AudioContext', 'CustomEvent', 'Event', 'DOMPoint',
    'Blob', 'URL', 'globalThis', 'self', 'navigator', 'location', 'history', 'localStorage',
    'sessionStorage', 'structuredClone', 'queueMicrotask', 'Intl', 'WeakRef'
]);

/* Remove comments, strings and template literals. Regex literals are cut to
 * nothing: a replacement that carries an identifier would hide a declaration. */
function strip(src) {
    let out = '';
    let i = 0;
    const n = src.length;
    const regexAllowed = () => {
        for (let k = out.length - 1; k >= 0; k--) {
            const ch = out[k];
            if (ch === ' ' || ch === '\t' || ch === '\n' || ch === '\r') continue;
            return '(,=:[!&|?{};+-*%~^'.includes(ch) || /\b(?:return|typeof|instanceof|in|of|new|delete|void|case|do|else)$/.test(out.slice(Math.max(0, k - 12), k));
        }
        return true;
    };
    while (i < n) {
        const ch = src[i], nx = src[i + 1];
        if (ch === '/' && nx === '/') { while (i < n && src[i] !== '\n') i++; continue; }
        if (ch === '/' && nx === '*') {
            i += 2; let nl = 0;
            while (i < n && !(src[i] === '*' && src[i + 1] === '/')) { if (src[i] === '\n') nl++; i++; }
            i += 2; out += ' '.repeat(1) + '\n'.repeat(nl); continue;
        }
        if (ch === '"' || ch === "'") {
            i++;
            while (i < n && src[i] !== ch) { if (src[i] === '\\') i++; i++; }
            i++; out += '""'; continue;
        }
        if (ch === '`') {
            i++;
            let depth = 0, nl = 0;
            while (i < n) {
                if (src[i] === '\\') { i += 2; continue; }
                if (src[i] === '\n') nl++;
                if (src[i] === '$' && src[i + 1] === '{') { depth++; i += 2; continue; }
                if (depth > 0 && src[i] === '}') { depth--; i++; continue; }
                if (depth === 0 && src[i] === '`') break;
                i++;
            }
            i++; out += '""' + '\n'.repeat(nl); continue;
        }
        if (ch === '/' && regexAllowed()) {
            i++;
            let inClass = false;
            while (i < n) {
                if (src[i] === '\\') { i += 2; continue; }
                if (src[i] === '[') inClass = true;
                else if (src[i] === ']') inClass = false;
                else if (src[i] === '/' && !inClass) break;
                else if (src[i] === '\n') break;
                i++;
            }
            i++;
            while (i < n && /[gimsuyd]/.test(src[i])) i++;
            out += ' '; continue;
        }
        out += ch; i++;
    }
    /* Keys of an object literal are names, not variables: { PUFF: 0 } */
    return out.replace(/([{,])\s*([A-Za-z_$][\w$]*)\s*:/g, '$1 :');
}

/* All names a file declares: const/let/var (also multi-declarator lists and
 * destructuring), function and class names, parameters and catch bindings. */
function declared(src) {
    const out = new Set();
    const add = n => { if (/^[A-Za-z_$][\w$]*$/.test(n)) out.add(n); };

    /* Split a declaration list at top-level commas. */
    const splitTop = text => {
        const parts = [];
        let depth = 0, buf = '';
        for (const ch of text) {
            if (ch === '{' || ch === '[' || ch === '(') depth++;
            else if (ch === '}' || ch === ']' || ch === ')') depth--;
            if (ch === ',' && depth === 0) { parts.push(buf); buf = ''; continue; }
            buf += ch;
        }
        if (buf.trim()) parts.push(buf);
        return parts;
    };
    /* Names inside a destructuring pattern: { a, b: c, d = 1, ...rest } */
    const patternNames = text => {
        for (const part of splitTop(text)) {
            const t = part.trim();
            if (!t) continue;
            const key = t.split(':').pop().split('=')[0].trim().replace(/^\.\.\./, '');
            add(key);
        }
    };

    const re = /\b(?:const|let|var)\s+/g;
    let m;
    while ((m = re.exec(src))) {
        let i = re.lastIndex, depth = 0, buf = '';
        while (i < src.length) {
            const ch = src[i];
            if (ch === '{' || ch === '[' || ch === '(') depth++;
            else if (ch === '}' || ch === ']' || ch === ')') {
                if (depth === 0) break;
                depth--;
            } else if ((ch === ';' || ch === '\n') && depth === 0) break;
            buf += ch; i++;
        }
        for (const part of splitTop(buf)) {
            const t = part.trim();
            if (!t) continue;
            if (t[0] === '{' || t[0] === '[') {
                const close = t[0] === '{' ? '}' : ']';
                const end = t.indexOf(close);
                if (end > 0) patternNames(t.slice(1, end));
                continue;
            }
            const head = t.split('=')[0].split(':')[0].trim().replace(/^\.\.\./, '');
            add(head);
        }
    }
    const reFn = /\bfunction\s*([A-Za-z_$][\w$]*)?\s*\(([^)]*)\)/g;
    while ((m = reFn.exec(src))) {
        add(m[1] || '');
        for (const p of m[2].split(',')) add(p.split('=')[0].trim().replace(/^\.\.\./, ''));
    }
    const reArrow = /\(([^()]*)\)\s*=>/g;
    while ((m = reArrow.exec(src))) for (const p of m[1].split(',')) add(p.split('=')[0].trim().replace(/^\.\.\./, ''));
    const reArrow1 = /(?<![.\w$])([A-Za-z_$][\w$]*)\s*=>/g;
    while ((m = reArrow1.exec(src))) add(m[1]);
    const reClass = /\bclass\s+([A-Za-z_$][\w$]*)/g;
    while ((m = reClass.exec(src))) add(m[1]);
    const reCatch = /\bcatch\s*\(\s*([A-Za-z_$][\w$]*)/g;
    while ((m = reCatch.exec(src))) add(m[1]);
    return out;
}

const files = readdirSync(APPS).filter(f => /^aura-drift.*\.js$/.test(f)).sort();
const problems = [];

for (const file of files) {
    const src = readFileSync(join(APPS, file), 'utf8');
    const code = strip(src);
    const known = declared(src);
    const seen = new Map();
    const re = /(?<![.\w$])([A-Za-z_$][\w$]*)/g;
    let m;
    while ((m = re.exec(code))) {
        const n = m[1];
        if (!/^[A-Z][A-Z0-9_]*$/.test(n)) continue;      /* only constants */
        if (known.has(n) || GLOBALS.has(n)) continue;
        if (!seen.has(n)) seen.set(n, code.slice(0, m.index).split('\n').length);
    }
    for (const [name, line] of seen) {
        /* Die Zeilennummer aus der Rohquelle nehmen, das Strippen verschiebt Zeilen. */
        const at = src.search(new RegExp(`(?<![.\\w$])${name}\\b`));
        problems.push({ file, name, line: at < 0 ? line : src.slice(0, at).split('\n').length });
    }
}

if (problems.length) {
    console.error('ok   no undefined constants in the Aura Drift modules');
    for (const p of problems) console.error(`FAIL ${p.file}:${p.line}  ${p.name} is used but never declared`);
    process.exit(1);
}
console.log(`ok   every constant in ${files.length} Aura Drift modules is declared (${files.length} files)`);
