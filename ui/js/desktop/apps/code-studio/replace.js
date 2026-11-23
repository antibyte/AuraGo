    // Project-wide replace for the search panel. The server search (grep) lists the matching lines;
    // the replace itself runs here with a JavaScript RegExp built from the same options. The preview
    // and the write both go through replaceLineSegments, so the preview shows exactly what lands in a
    // file. Open files change in their editor buffer as one undoable transaction and are not written;
    // closed files are read with GET /api/code-studio/file and written back with PUT.

    // --- Project replace: pure helpers. No DOM and no studio state; scripts/test-code-studio-replace.mjs runs them in a VM. ---

    // Same options as the search: literal text is escaped (escapeRegExp from find.js), whole word wraps
    // the pattern in \b and the case switch decides the i flag. Returns null for an empty query or for a
    // pattern JavaScript cannot compile.
    function buildReplaceRegExp(query, options) {
        const needle = String(query || '');
        if (!needle) return null;
        const settings = options || {};
        let source = settings.regex ? needle : escapeRegExp(needle);
        if (settings.whole) source = settings.regex ? '\\b(?:' + source + ')\\b' : '\\b' + source + '\\b';
        try {
            return new RegExp(source, settings.case ? 'g' : 'gi');
        } catch (_) {
            return null;
        }
    }

    // GetSubstitution of String.prototype.replace: $$, $&, $`, $', $1 to $99 and $<name>.
    function expandReplaceTemplate(template, match, line) {
        const text = String(template == null ? '' : template);
        const source = String(line == null ? '' : line);
        const captures = match.length - 1;
        const groups = match.groups;
        const isDigit = ch => ch !== undefined && ch >= '0' && ch <= '9';
        let out = '';
        for (let i = 0; i < text.length; i += 1) {
            const ch = text[i];
            const next = text[i + 1];
            if (ch !== '$' || next === undefined) {
                out += ch;
            } else if (next === '$') {
                out += '$';
                i += 1;
            } else if (next === '&') {
                out += match[0];
                i += 1;
            } else if (next === '`') {
                out += source.slice(0, match.index);
                i += 1;
            } else if (next === "'") {
                out += source.slice(match.index + match[0].length);
                i += 1;
            } else if (isDigit(next)) {
                const two = isDigit(text[i + 2]) ? Number(next + text[i + 2]) : 0;
                const one = Number(next);
                if (two >= 1 && two <= captures) {
                    out += match[two] == null ? '' : match[two];
                    i += 2;
                } else if (one >= 1 && one <= captures) {
                    out += match[one] == null ? '' : match[one];
                    i += 1;
                } else {
                    out += '$';
                }
            } else if (next === '<' && groups && text.indexOf('>', i + 2) !== -1) {
                const close = text.indexOf('>', i + 2);
                const value = groups[text.slice(i + 2, close)];
                out += value == null ? '' : String(value);
                i = close;
            } else {
                out += '$';
            }
        }
        return out;
    }

    // Splits one line into plain text and matches; every match carries the text that replaces it.
    // job = { regex, replacement, regexMode }. Zero-length matches are skipped like the in-file find does.
    function replaceLineSegments(line, job) {
        const text = String(line == null ? '' : line);
        if (!job || !job.regex) return text ? [{ text }] : [];
        const flags = job.regex.flags.includes('g') ? job.regex.flags : job.regex.flags + 'g';
        const pattern = new RegExp(job.regex.source, flags);
        const replacement = String(job.replacement == null ? '' : job.replacement);
        const segments = [];
        let cursor = 0;
        let found = 0;
        let match;
        while (found < 5000 && (match = pattern.exec(text)) !== null) {
            if (match[0] === '') {
                pattern.lastIndex += 1;
                continue;
            }
            if (match.index > cursor) segments.push({ text: text.slice(cursor, match.index) });
            segments.push({
                match: true,
                from: match.index,
                to: match.index + match[0].length,
                text: match[0],
                insert: job.regexMode ? expandReplaceTemplate(replacement, match, text) : replacement
            });
            cursor = match.index + match[0].length;
            found += 1;
        }
        if (cursor < text.length) segments.push({ text: text.slice(cursor) });
        return segments;
    }

    function replaceSegmentsText(segments) {
        return (segments || []).map(segment => (segment.match ? segment.insert : segment.text)).join('');
    }

    // The parts one result row shows for one occurrence: the text before it (indentation dropped,
    // long prefixes shortened), the old text, the new text and the rest of the line. Other matches on
    // the same line stay plain.
    function replaceMatchPreview(segments, occurrence) {
        let before = '';
        let after = '';
        let found = null;
        let seen = 0;
        (segments || []).forEach(segment => {
            if (segment.match && !found && seen === occurrence) {
                found = segment;
                seen += 1;
                return;
            }
            if (segment.match) seen += 1;
            if (found) after += segment.text;
            else before += segment.text;
        });
        before = before.replace(/^\s+/, '');
        if (before.length > 40) before = '…' + before.slice(-39);
        return { before, old: found ? found.text : '', next: found ? found.insert : '', after };
    }

    // The server sends the line as grep printed it; indentation and a trailing CR do not count.
    function sameSearchLine(lineText, preview) {
        return String(lineText == null ? '' : lineText).trim() === String(preview == null ? '' : preview).trim();
    }

    // Plans the replacements for one file. targets: [{ line, preview, occurrence }]; occurrence null
    // means every match on that line. Each target line must still read like its search result, else
    // the whole file is stale. Returns { status: 'ok' | 'stale' | 'nomatch', changes, count, content };
    // changes are ascending offsets into content, ready for one CodeMirror transaction. CRLF stays.
    function planFileReplace(content, targets, job) {
        const text = String(content == null ? '' : content);
        const stale = { status: 'stale', changes: [], count: 0, content: text };
        const starts = [0];
        for (let i = 0; i < text.length; i += 1) {
            if (text.charCodeAt(i) === 10) starts.push(i + 1);
        }
        const changes = [];
        for (const target of targets || []) {
            const lineNumber = Number(target && target.line);
            if (!Number.isInteger(lineNumber) || lineNumber < 1 || lineNumber > starts.length) return stale;
            const from = starts[lineNumber - 1];
            let end = lineNumber < starts.length ? starts[lineNumber] - 1 : text.length;
            if (end > from && text.charCodeAt(end - 1) === 13) end -= 1;
            const lineText = text.slice(from, end);
            if (!sameSearchLine(lineText, target.preview)) return stale;
            const matches = replaceLineSegments(lineText, job).filter(segment => segment.match);
            let picked = matches;
            if (target.occurrence !== null && target.occurrence !== undefined) {
                const chosen = matches[Number(target.occurrence)];
                if (!chosen) return stale;
                picked = [chosen];
            }
            picked.forEach(segment => changes.push({ from: from + segment.from, to: from + segment.to, insert: segment.insert }));
        }
        changes.sort((a, b) => a.from - b.from || a.to - b.to);
        const applied = [];
        let out = '';
        let cursor = 0;
        changes.forEach(change => {
            if (change.from < cursor) return;
            out += text.slice(cursor, change.from) + change.insert;
            cursor = change.to;
            applied.push(change);
        });
        if (!applied.length) return { status: 'nomatch', changes: [], count: 0, content: text };
        return { status: 'ok', changes: applied, count: applied.length, content: out + text.slice(cursor) };
    }

    // Search results for an open buffer: the lines where the RegExp finds a non-empty match.
    function bufferSearchResults(path, content, regex, limit) {
        const found = [];
        if (!regex) return found;
        const max = Number(limit) > 0 ? Number(limit) : 200;
        const probe = { regex, replacement: '', regexMode: false };
        const lines = String(content == null ? '' : content).split('\n');
        for (let index = 0; index < lines.length && found.length < max; index += 1) {
            const line = lines[index].endsWith('\r') ? lines[index].slice(0, -1) : lines[index];
            if (replaceLineSegments(line, probe).some(segment => segment.match)) found.push({ path, line: index + 1, preview: line });
        }
        return found;
    }

    // --- End of the pure project replace helpers. ---
