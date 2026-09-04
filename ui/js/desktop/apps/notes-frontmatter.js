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
        const result = { present: false, valid: false, tags: [], title: '', eol, lines };
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
            if (key === 'tags') result.tags = parseTags(value);
            else if (key === 'title' && !result.title) result.title = value.replace(/^["']|["']$/g, '');
        }
        return result;
    }

    function parseTags(value) {
        let v = String(value || '').trim();
        if (!v) return [];
        if (v.startsWith('[') && v.endsWith(']')) v = v.slice(1, -1);
        return v.split(',')
            .map(tag => tag.trim().replace(/^["']|["']$/g, '').toLowerCase())
            .filter(tag => tag.length > 0);
    }

    function serializeTags(tags) {
        return (tags || []).join(', ');
    }

    // Rewrites only the tags line. Preserves every other frontmatter line
    // verbatim, keeps the original bracket style when replacing an existing
    // tags line, and prepends a fresh block when no frontmatter exists.
    function updateTags(content, tags) {
        const nextTags = (tags || []).map(tag => String(tag).trim().toLowerCase()).filter(Boolean);
        const parsed = parse(content);
        const eol = parsed.eol;
        if (!parsed.present || !parsed.valid) {
            if (!nextTags.length) return String(content || '');
            const block = [OPEN_FENCE, 'tags: ' + serializeTags(nextTags), OPEN_FENCE, ''].join(eol);
            return block + eol + String(content || '');
        }
        const lines = parsed.lines.slice();
        let tagsLineIdx = -1;
        let bracketStyle = false;
        for (let i = 1; i < parsed.blockEnd; i++) {
            const match = lines[i].match(/^([A-Za-z0-9_-]+)\s*:\s*(.*)$/);
            if (match && match[1].toLowerCase() === 'tags') {
                tagsLineIdx = i;
                bracketStyle = match[2].trim().startsWith('[');
                break;
            }
        }
        if (!nextTags.length) {
            if (tagsLineIdx === -1) return joinLines(lines, eol);
            lines.splice(tagsLineIdx, 1);
            if (parsed.blockEnd - 1 <= 1) {
                // frontmatter became empty: drop the whole block
                // (the closing fence shifted one line up after the splice)
                const rest = lines.slice(parsed.blockEnd);
                return joinLines(rest, eol);
            }
            return joinLines(lines, eol);
        }
        const value = bracketStyle
            ? '[' + serializeTags(nextTags) + ']'
            : serializeTags(nextTags);
        if (tagsLineIdx === -1) lines.splice(1, 0, 'tags: ' + value);
        else lines[tagsLineIdx] = 'tags: ' + value;
        return joinLines(lines, eol);
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

    window.NotesFrontmatter = { parse, updateTags, strip, deriveTitle };
})();
