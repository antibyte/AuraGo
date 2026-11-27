// Lucide icon data for Storymaker: converts icon-nodes.json elements to path data on the 24 grid.
// Reads only icon-nodes.json, tags.json and LICENSE of a lucide-static package; it never runs package code.

import fs from 'node:fs';
import path from 'node:path';

export const LUCIDE_VERSION = '1.82.0';
export const LUCIDE_SOURCE = `lucide-static@${LUCIDE_VERSION}`;
export const LUCIDE_INTEGRITY = 'sha512-Xzzpfkx06hHXjXoLWOk1RWg52OhfaRcIOIK72hCbRty8Jq29WQUV4J4Z9GvjAKDoPxGf7tlcqBbJX4qrr1U8+w==';
export const ICON_SCHEMA = 'storymaker-icons/1';
export const ALLOW_LIST_SCHEMA = 'storymaker-icon-allow-list/1';
export const CATEGORIES = ['devices', 'development', 'connectivity', 'security', 'science', 'medical', 'finance', 'charts', 'communication', 'people', 'nature', 'transport', 'weather', 'files', 'arrows', 'home', 'education', 'general'];
export const PATH_CHARS = /^[MmLlHhVvCcSsQqTtAaZz0-9 .,eE-]+$/;

export const fmt = (n) => String(Math.round(n * 1000) / 1000);

const num = (attrs, key, tag) => {
    const v = Number(attrs[key]);
    if (attrs[key] === undefined || !Number.isFinite(v)) throw new Error(`${tag}: attribute ${key} is missing or not a number`);
    return v;
};

const points = (attrs, tag) => {
    const v = String(attrs.points ?? '').trim().split(/[\s,]+/).map(Number);
    if (v.length < 4 || v.length % 2 || v.some((n) => !Number.isFinite(n))) throw new Error(`${tag}: bad points`);
    const out = [];
    for (let i = 0; i < v.length; i += 2) out.push(`${i ? 'L' : 'M'}${fmt(v[i])} ${fmt(v[i + 1])}`);
    return out.join('');
};

const ellipse = (cx, cy, rx, ry) => `M${fmt(cx - rx)} ${fmt(cy)}A${fmt(rx)} ${fmt(ry)} 0 1 0 ${fmt(cx + rx)} ${fmt(cy)}A${fmt(rx)} ${fmt(ry)} 0 1 0 ${fmt(cx - rx)} ${fmt(cy)}Z`;

function nodeToPath([tag, attrs]) {
    if ('transform' in attrs) throw new Error(`${tag}: transform is not supported`);
    switch (tag) {
        case 'path': return String(attrs.d ?? '').trim();
        case 'circle': {
            const r = num(attrs, 'r', tag);
            return ellipse(num(attrs, 'cx', tag), num(attrs, 'cy', tag), r, r);
        }
        case 'ellipse': return ellipse(num(attrs, 'cx', tag), num(attrs, 'cy', tag), num(attrs, 'rx', tag), num(attrs, 'ry', tag));
        case 'rect': {
            const x = num(attrs, 'x', tag), y = num(attrs, 'y', tag), w = num(attrs, 'width', tag), h = num(attrs, 'height', tag);
            // Each radius defaults to the other one as written, then each is clamped to its own half side.
            const rawRx = attrs.rx === undefined ? undefined : num(attrs, 'rx', tag);
            const rawRy = attrs.ry === undefined ? undefined : num(attrs, 'ry', tag);
            const rx = Math.min(rawRx ?? rawRy ?? 0, w / 2);
            const ry = Math.min(rawRy ?? rawRx ?? 0, h / 2);
            if (!rx && !ry) return `M${fmt(x)} ${fmt(y)}H${fmt(x + w)}V${fmt(y + h)}H${fmt(x)}Z`;
            const a = `A${fmt(rx)} ${fmt(ry)} 0 0 1 `;
            return `M${fmt(x + rx)} ${fmt(y)}H${fmt(x + w - rx)}${a}${fmt(x + w)} ${fmt(y + ry)}V${fmt(y + h - ry)}${a}${fmt(x + w - rx)} ${fmt(y + h)}H${fmt(x + rx)}${a}${fmt(x)} ${fmt(y + h - ry)}V${fmt(y + ry)}${a}${fmt(x + rx)} ${fmt(y)}Z`;
        }
        case 'line': return `M${fmt(num(attrs, 'x1', tag))} ${fmt(num(attrs, 'y1', tag))}L${fmt(num(attrs, 'x2', tag))} ${fmt(num(attrs, 'y2', tag))}`;
        case 'polyline': return points(attrs, tag);
        case 'polygon': return `${points(attrs, tag)}Z`;
        default: throw new Error(`${tag}: element is not supported`);
    }
}

// nodes: [[tag, attrs], ...] -> {paths, fills}; fills are the elements with a fill other than none (drawn filled and stroked).
export function nodesToPaths(nodes) {
    const paths = [], fills = [];
    for (const node of nodes) {
        const d = nodeToPath(node);
        paths.push(d);
        const fill = node[1].fill;
        if (fill !== undefined && fill !== 'none') fills.push(d);
    }
    return { paths, fills };
}

const label = (name) => name.split('-');

// allowList: {schema, source, categories: {name: [ids]}} -> {json, notice, count, problems}
export function buildIconData(pkgDir, allowList) {
    const read = (f) => fs.readFileSync(path.join(pkgDir, f), 'utf8');
    // The notice names the pinned version and integrity, so the package must be that version.
    const problems = [];
    try {
        const meta = JSON.parse(read('package.json'));
        if (meta.name !== 'lucide-static' || meta.version !== LUCIDE_VERSION) {
            problems.push(`package.json is ${meta.name}@${meta.version}, expected ${LUCIDE_SOURCE}`);
        }
    } catch (e) { problems.push(`package.json cannot be read: ${e.message}`); }
    if (problems.length) return { json: '', notice: '', count: 0, problems, license: '' };
    const nodes = JSON.parse(read('icon-nodes.json'));
    const tags = JSON.parse(read('tags.json'));
    const license = read('LICENSE').replace(/\r\n/g, '\n');
    const cats = new Map();
    if (allowList.schema !== ALLOW_LIST_SCHEMA) problems.push(`allow-list schema is ${allowList.schema}`);
    for (const cat of CATEGORIES) for (const id of allowList.categories?.[cat] ?? []) {
        if (!cats.has(id)) cats.set(id, []);
        if (!cats.get(id).includes(cat)) cats.get(id).push(cat);
    }
    for (const cat of Object.keys(allowList.categories ?? {})) if (!CATEGORIES.includes(cat)) problems.push(`unknown category ${cat}`);
    const lines = [];
    for (const name of [...cats.keys()].sort()) {
        if (!Object.prototype.hasOwnProperty.call(nodes, name)) { problems.push(`allow-list id ${name} is not in the package`); continue; }
        let conv;
        try { conv = nodesToPaths(nodes[name]); } catch (e) { problems.push(`${name}: ${e.message}`); continue; }
        const icon = { id: `icon.${name}`, paths: conv.paths };
        if (conv.fills.length) icon.fills = conv.fills;
        icon.tags = [...new Set([...(tags[name] ?? []).slice(0, 20), ...label(name), ...cats.get(name)])];
        if (!/^icon\.[a-z0-9-]{1,60}$/.test(icon.id)) problems.push(`${name}: bad id`);
        if (icon.paths.length < 1 || icon.paths.length > 40) problems.push(`${name}: ${icon.paths.length} paths`);
        for (const p of icon.paths) if (p.length > 2000 || !PATH_CHARS.test(p)) problems.push(`${name}: bad path data ${p.slice(0, 40)}`);
        if (icon.tags.length > 32) problems.push(`${name}: ${icon.tags.length} tags`);
        lines.push(JSON.stringify(icon));
    }
    const json = `{"schema":"${ICON_SCHEMA}","source":"${LUCIDE_SOURCE}","grid":24,"stroke":2,"icons":[\n${lines.join(',\n')}\n]}\n`;
    if (Buffer.byteLength(json) > 256 * 1024) problems.push(`lucide.json is ${Buffer.byteLength(json)} bytes`);
    const notice = `Storymaker icons: ${lines.length} icons from Lucide (lucide-static ${LUCIDE_VERSION}, https://lucide.dev), ISC License.\nSome Lucide icons derive from Feather (Cole Bemis), MIT License; both texts follow.\nPackage integrity: ${LUCIDE_INTEGRITY}\nConverted to path data on the 24 grid by scripts/build-storymaker-icons.mjs from icon-nodes.json and tags.json.\n\n${license}`;
    return { json, notice, count: lines.length, problems, license };
}
