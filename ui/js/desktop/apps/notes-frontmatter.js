(function () {
    'use strict';

    // Minimal YAML frontmatter support for the Notes app.
    // Contract: only the tags line is ever rewritten; all other frontmatter
    // keys are preserved verbatim (including unknown keys and original line
    // endings). Malformed blocks are treated as plain markdown.

    const OPEN_FENCE = '---';

    function detectEol(content) {
        const idx = String(content || '').indexOf('\r\n');
        return idx === -1 ? '\n' : '\r\n';
    }

    function splitLines(block) {
        return String(block || '').split(/\r?\n/);
    }

    function joinLines(lines, eol) {
        return lines.join(eol);
    }

    // Returns {present, valid, tags, title, blockStart, blockEnd, bodyStart, eol, lines}
    // blockStart/blockEnd are line indexes of the fences; bodyStart is the line
    // index where the markdown body begins.
    function parse(content) {
        const raw = String(content || '');
        const eol = detectEol(raw);
        const lines = splitLines(raw);
        const result = { present: false, valid: false, tags: [], aliases: [], title: '', eol, lines };
        if (lines[0] === undefined || lines[0].trim() !== OPEN_FENCE) return result;
        result.present = true;
        let end = -1;
        for (let i = 1; i < lines.length; i++) {
            if (lines[i].trim() === OPEN_FENCE) { end = i; break; }
        }
        if (end === -1) return result; // unterminated: treat as plain markdown
        result.valid = true;
        result.blockStart = 0;
        result.blockEnd = end;
        result.bodyStart = end + 1;
        for (let i = 1; i < end; i++) {
            const line = lines[i];
            const match = line.match(/^([A-Za-z0-9_-]+)\s*:\s*(.*)$/);
            if (!match) continue;
            const key = match[1].toLowerCase();
            const value = match[2].trim();
            if (key === 'tags') {
                result.tags = parseTags(value);
                if (!value) for (let j=i+1;j<end;j++) {
                    const item=/^\s+-\s+(.+)$/.exec(lines[j]);
                    if (!item) break;
                    result.tags.push(...parseTags(item[1]));
                }
            }
            else if ((key === 'aliases' || key === 'alias') && !result.aliases.length) result.aliases = readListAt(lines, i, end).values;
            else if (key === 'title' && !result.title) result.title = value.replace(/^["']|["']$/g, '');
        }
        return result;
    }

    function parseTags(value) {
        let v = String(value || '').trim();
        if (!v) return [];
        try { const parsed=JSON.parse(v.startsWith('[')?v:'['+v+']'); if(Array.isArray(parsed))return parsed.filter(tag=>typeof tag==='string').map(tag=>tag.toLowerCase()); } catch (_) {}
        if (v.startsWith('[') && v.endsWith(']')) v = v.slice(1, -1);
        return v.split(',')
            .map(tag => tag.trim().replace(/^["']|["']$/g, '').toLowerCase())
            .filter(tag => tag.length > 0);
    }

    function serializeTags(tags) {
        return (tags || []).map(tag=>/^[\p{L}\p{N}_-]+$/u.test(tag)?tag:JSON.stringify(tag)).join(', ');
    }

    function parseList(value) {
        let v = String(value || '').trim();
        if (!v) return [];
        try { const parsed=JSON.parse(v.startsWith('[')?v:'['+v+']'); if(Array.isArray(parsed))return parsed.filter(x=>typeof x==='string').map(x=>x.trim()).filter(Boolean); } catch (_) {}
        if (v.startsWith('[') && v.endsWith(']')) v = v.slice(1, -1);
        return v.split(',')
            .map(x => x.trim().replace(/^["']|["']$/g, ''))
            .filter(x => x.length > 0);
    }

    const KEY_LINE = /^([A-Za-z0-9_-]+)\s*:\s*(.*)$/;

    // Reads the values of the list property at line index i (inline or block).
    function readListAt(lines, i, end) {
        const match = KEY_LINE.exec(lines[i]);
        const value = match[2].trim();
        const out = parseList(value);
        let count = 1;
        if (!value) {
            while (i + count < end && /^\s+-\s+/.test(lines[i + count])) {
                const item = /^\s+-\s+(.+)$/.exec(lines[i + count]);
                if (item) out.push(...parseList(item[1]));
                count++;
            }
        }
        return { values: out, count, inline: value };
    }

    function findKey(parsed, key) {
        const want = String(key).toLowerCase();
        for (let i = 1; i < parsed.blockEnd; i++) {
            const match = KEY_LINE.exec(parsed.lines[i]);
            if (match && match[1].toLowerCase() === want) return i;
        }
        return -1;
    }

    function listValues(content, key) {
        const parsed = parse(content);
        if (!parsed.valid) return [];
        const idx = findKey(parsed, key);
        if (idx === -1) return [];
        return readListAt(parsed.lines, idx, parsed.blockEnd).values;
    }

    function fields(content) {
        const parsed = parse(content);
        if (!parsed.valid) return [];
        const out = [];
        for (let i = 1; i < parsed.blockEnd; i++) {
            const match = KEY_LINE.exec(parsed.lines[i]);
            if (!match) continue;
            const info = readListAt(parsed.lines, i, parsed.blockEnd);
            const list = info.inline ? info.inline.startsWith('[') : info.count > 1;
            out.push({ key: match[1], value: list ? info.values.join(', ') : info.inline, list });
            i += info.count - 1;
        }
        return out;
    }

    // Rewrites only the given key's line (and its block-list items). Preserves
    // every other frontmatter line verbatim, keeps the original bracket style
    // when replacing an existing line, and prepends a fresh block when no
    // frontmatter exists.
    function updateList(content, key, values) {
        const next = (values || []).map(v => String(v).trim()).filter(Boolean);
        const parsed = parse(content);
        const eol = parsed.eol;
        if (!parsed.present || !parsed.valid) {
            if (!next.length) return String(content || '');
            // No blank line after the closing fence: dropping the block again must restore the note byte for byte.
            return [OPEN_FENCE, key + ': ' + serializeTags(next), OPEN_FENCE].join(eol) + eol + String(content || '');
        }
        const lines = parsed.lines.slice();
        const idx = findKey(parsed, key);
        let bracketStyle = false;
        let count = 1;
        let lineKey = key;
        if (idx !== -1) {
            const info = readListAt(lines, idx, parsed.blockEnd);
            bracketStyle = info.inline.startsWith('[');
            count = info.count;
            lineKey = KEY_LINE.exec(lines[idx])[1];
        }
        if (!next.length) {
            if (idx === -1) return joinLines(lines, eol);
            lines.splice(idx, count);
            if (parsed.blockEnd - count <= 1) {
                // frontmatter became empty: drop the whole block
                return joinLines(lines.slice(parsed.blockEnd - count + 1), eol);
            }
            return joinLines(lines, eol);
        }
        const value = bracketStyle ? '[' + serializeTags(next) + ']' : serializeTags(next);
        if (idx === -1) lines.splice(1, 0, key + ': ' + value);
        else lines.splice(idx, count, lineKey + ': ' + value);
        return joinLines(lines, eol);
    }

    function updateTags(content, tags) {
        return updateList(content, 'tags', (tags || []).map(tag => String(tag).trim().toLowerCase()).filter(Boolean));
    }

    function strip(content) {
        const parsed = parse(content);
        if (!parsed.present || !parsed.valid) return String(content || '');
        return joinLines(parsed.lines.slice(parsed.bodyStart), parsed.eol);
    }

    function firstBodyLine(body) {
        const lines = splitLines(body);
        for (const line of lines) {
            const trimmed = line.trim();
            if (trimmed) return trimmed;
        }
        return '';
    }

    function deriveTitle(content, fallbackName) {
        const parsed = parse(content);
        if (parsed.title) return parsed.title;
        const body = strip(content);
        const lines = splitLines(body);
        for (let i = 0; i < Math.min(lines.length, 80); i++) {
            const heading = lines[i].match(/^#{1,6}\s+(.+?)\s*#*$/);
            if (heading && heading[1].trim()) return heading[1].trim();
        }
        const first = firstBodyLine(body);
        if (first) return first.replace(/^[>\-*+]\s*/, '').slice(0, 120);
        const name = String(fallbackName || '');
        if (name) return name.replace(/\.md$/i, '');
        return '';
    }

    const api = { parse, updateTags, strip, deriveTitle, listValues, updateList, fields };
    if (typeof window !== 'undefined') window.NotesFrontmatter = api;
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
})();
