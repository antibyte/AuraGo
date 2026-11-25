/* Code Studio IDE package 2C: the pure half of the language server actions. Nothing here touches the DOM or the
   studio state, so the Code Studio bundle and lsp-edits.test.cjs (node) load the same file. It turns an LSP
   WorkspaceEdit into per-file change lists, applies LSP TextEdits to plain text and groups reference locations for
   the References panel. Positions are LSP { line, character } with UTF-16 characters, the unit of JavaScript strings
   and of CodeMirror offsets. Paths come from CodeStudioLSPProtocol.pathForUri (lsp-protocol.js, plan 2B); nothing
   outside /workspace passes. */
(function (root, factory) {
    'use strict';
    const api = factory(() => {
        if (root && root.CodeStudioLSPProtocol) return root.CodeStudioLSPProtocol;
        if (typeof CodeStudioLSPProtocol !== 'undefined') return CodeStudioLSPProtocol;
        return null;
    });
    if (typeof module === 'object' && module && module.exports) module.exports = api;
    if (root) root.CodeStudioLSPEdits = api;
})(typeof window !== 'undefined' ? window : null, getProtocol => {
    'use strict';

    const WORKSPACE_PREFIX = '/workspace/';

    function editError(code) {
        const error = new Error('LSP text edit rejected: ' + code);
        error.editError = code;
        return error;
    }

    // '/workspace/...' for a URI inside the workspace, null for anything else (other roots, empty, '.' or '..' parts).
    function workspacePath(uri) {
        if (typeof uri !== 'string' || !uri) return null;
        const protocol = getProtocol();
        const path = protocol && typeof protocol.pathForUri === 'function' ? protocol.pathForUri(uri) : null;
        if (typeof path !== 'string' || !path.startsWith(WORKSPACE_PREFIX)) return null;
        if (path.slice(1).split('/').some(part => part === '' || part === '.' || part === '..')) return null;
        return path;
    }

    // Offsets where lines start. LSP ends a line at \n, \r\n or a lone \r.
    function lineStarts(text) {
        const starts = [0];
        for (let index = 0; index < text.length; index += 1) {
            const code = text.charCodeAt(index);
            if (code === 13) {
                if (text.charCodeAt(index + 1) === 10) index += 1;
                starts.push(index + 1);
            } else if (code === 10) {
                starts.push(index + 1);
            }
        }
        return starts;
    }

    // Offset just before the line break of a line; the end of the text for the last line.
    function lineEnd(text, starts, line) {
        if (line + 1 >= starts.length) return text.length;
        let end = starts[line + 1];
        if (text.charCodeAt(end - 1) === 10) end -= 1;
        if (end > starts[line] && text.charCodeAt(end - 1) === 13) end -= 1;
        return end;
    }

    // LSP position to an offset. A character past the line end means the line end (LSP 3.17); the line after the
    // last one, at character 0, means the end of the text.
    function offsetAt(text, starts, position) {
        const line = position && position.line;
        const character = position && position.character;
        if (!Number.isInteger(line) || !Number.isInteger(character) || line < 0 || character < 0) throw editError('invalid');
        if (line >= starts.length) {
            if (line === starts.length && character === 0) return text.length;
            throw editError('range');
        }
        return Math.min(starts[line] + character, lineEnd(text, starts, line));
    }

    // TextEdits to CodeMirror style changes: ascending, offsets into the original text. Inserts at one position merge
    // in array order, and they may precede one replacement at the same start. Overlapping edits are rejected.
    function textEditsToChanges(text, edits) {
        const source = String(text == null ? '' : text);
        if (!Array.isArray(edits)) throw editError('invalid');
        const starts = lineStarts(source);
        const list = edits.map((edit, index) => {
            if (!edit || !edit.range || typeof edit.newText !== 'string') throw editError('invalid');
            const from = offsetAt(source, starts, edit.range.start);
            const to = offsetAt(source, starts, edit.range.end);
            if (to < from) throw editError('range');
            return { from, to, insert: edit.newText, index };
        });
        list.sort((a, b) => a.from - b.from || Number(a.to > a.from) - Number(b.to > b.from) || a.index - b.index);
        const changes = [];
        list.forEach(change => {
            const last = changes[changes.length - 1];
            if (last && change.from < last.to) throw editError('overlap');
            if (last && last.from === change.from && last.to === last.from) {
                last.to = change.to;
                last.insert += change.insert;
                return;
            }
            changes.push({ from: change.from, to: change.to, insert: change.insert });
        });
        return changes;
    }

    // Applies ascending, non-overlapping changes from the end of the text, so earlier offsets stay valid.
    function applyChanges(text, changes) {
        const parts = [];
        let cursor = text.length;
        for (let index = changes.length - 1; index >= 0; index -= 1) {
            const change = changes[index];
            parts.push(text.slice(change.to, cursor), change.insert);
            cursor = change.from;
        }
        parts.push(text.slice(0, cursor));
        return parts.reverse().join('');
    }

    function applyTextEdits(text, edits) {
        const source = String(text == null ? '' : text);
        return applyChanges(source, textEditsToChanges(source, edits));
    }

    // Batches of one file (several TextDocumentEdits for it) apply one after the other. Every step's offsets refer to
    // the text the previous step produced, which a CodeMirror transaction spec marks with sequential: true.
    function planTextEdits(text, batches) {
        let current = String(text == null ? '' : text);
        const steps = [];
        (Array.isArray(batches) ? batches : []).forEach(batch => {
            const changes = textEditsToChanges(current, batch);
            if (!changes.length) return;
            steps.push(changes);
            current = applyChanges(current, changes);
        });
        return { text: current, steps };
    }

    function isTextEdit(edit) {
        return !!edit && !!edit.range && !!edit.range.start && !!edit.range.end && typeof edit.newText === 'string';
    }

    // A WorkspaceEdit as { files, skipped }.
    // files: [{ path, uri, version, batches, edits, count }] in the order the edit names them. edits lists every
    // TextEdit of the file (for counts and previews); batches keeps the TextDocumentEdits apart for planTextEdits.
    // skipped: [{ uri, path, reason }] with reason 'create', 'rename' or 'delete' (file operations are not supported),
    // 'depends' (a text edit for a file such an operation touches), 'outside' (not under /workspace) or 'invalid'.
    // documentChanges win over changes, as the protocol says.
    function workspaceEditToFileChanges(edit) {
        const files = [];
        const skipped = [];
        const byPath = new Map();
        if (!edit || typeof edit !== 'object') return { files, skipped };
        const skip = (uri, reason, extra) => {
            skipped.push(Object.assign({ uri: typeof uri === 'string' ? uri : '', path: workspacePath(uri) || '', reason }, extra || {}));
        };
        const add = (uri, edits, version) => {
            const path = workspacePath(uri);
            if (!path) {
                skip(uri, 'outside');
                return;
            }
            if (!Array.isArray(edits) || !edits.every(isTextEdit)) {
                skip(uri, 'invalid');
                return;
            }
            if (!edits.length) return;
            let file = byPath.get(path);
            if (!file) {
                file = { path, uri, version: Number.isInteger(version) ? version : null, batches: [], edits: [], count: 0 };
                byPath.set(path, file);
                files.push(file);
            }
            file.batches.push(edits.slice());
            for (const item of edits) file.edits.push(item);
            file.count += edits.length;
        };
        if (Array.isArray(edit.documentChanges)) {
            // First pass: every file an operation touches, whatever the order of the changes.
            const touched = new Set();
            edit.documentChanges.forEach(change => {
                if (!change || typeof change !== 'object') return;
                if (change.kind === 'create' || change.kind === 'delete') touched.add(change.uri);
                else if (change.kind === 'rename') {
                    touched.add(change.oldUri);
                    touched.add(change.newUri);
                }
            });
            edit.documentChanges.forEach(change => {
                if (!change || typeof change !== 'object') {
                    skip('', 'invalid');
                    return;
                }
                if (change.kind === 'create' || change.kind === 'delete') {
                    skip(change.uri, change.kind);
                    return;
                }
                if (change.kind === 'rename') {
                    skip(change.oldUri, 'rename', { newPath: workspacePath(change.newUri) || '' });
                    return;
                }
                const document = change.textDocument;
                if (!document || typeof document.uri !== 'string') {
                    skip('', 'invalid');
                    return;
                }
                if (touched.has(document.uri)) {
                    skip(document.uri, 'depends');
                    return;
                }
                add(document.uri, change.edits, document.version);
            });
        } else if (edit.changes && typeof edit.changes === 'object') {
            Object.keys(edit.changes).forEach(uri => add(uri, edit.changes[uri], null));
        }
        return { files, skipped };
    }

    function wholeNumber(value) {
        const number = Math.floor(Number(value));
        return Number.isFinite(number) && number > 0 ? number : 0;
    }

    // Reference locations grouped by file: { groups: [{ path, uri, items: [{ line, character, endLine, endCharacter }] }],
    // outside, total }. Files sort by path, references by position, duplicates collapse. outside counts the results
    // that are not under /workspace (the standard library, for example).
    function referencesToGroups(locations) {
        const groups = [];
        const byPath = new Map();
        const seen = new Set();
        let outside = 0;
        (Array.isArray(locations) ? locations : []).forEach(location => {
            if (!location || typeof location.uri !== 'string' || !location.range || !location.range.start) return;
            const path = workspacePath(location.uri);
            if (!path) {
                outside += 1;
                return;
            }
            const start = location.range.start;
            const end = location.range.end || start;
            const item = {
                line: wholeNumber(start.line),
                character: wholeNumber(start.character),
                endLine: wholeNumber(end.line),
                endCharacter: wholeNumber(end.character)
            };
            const key = [path, item.line, item.character, item.endLine, item.endCharacter].join('|');
            if (seen.has(key)) return;
            seen.add(key);
            let group = byPath.get(path);
            if (!group) {
                group = { path, uri: location.uri, items: [] };
                byPath.set(path, group);
                groups.push(group);
            }
            group.items.push(item);
        });
        groups.sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : 0));
        groups.forEach(group => group.items.sort((a, b) => a.line - b.line || a.character - b.character));
        return { groups, outside, total: groups.reduce((sum, group) => sum + group.items.length, 0) };
    }

    // The line of a reference for the panel: indentation dropped, a long prefix shortened, the range split out.
    // Returns { before, match, after }, or null when the line does not exist.
    function referencePreview(text, item, maxLength) {
        const source = String(text == null ? '' : text);
        const starts = lineStarts(source);
        if (!item || !Number.isInteger(item.line) || item.line < 0 || item.line >= starts.length) return null;
        const lineText = source.slice(starts[item.line], lineEnd(source, starts, item.line));
        const indent = lineText.length - lineText.replace(/^\s+/, '').length;
        const from = Math.min(Math.max(Number(item.character) || 0, indent), lineText.length);
        const to = item.endLine === item.line ? Math.min(Math.max(Number(item.endCharacter) || 0, from), lineText.length) : lineText.length;
        const limit = maxLength > 0 ? maxLength : 160;
        let before = lineText.slice(indent, from);
        if (before.length > 40) before = '…' + before.slice(-39);
        const match = lineText.slice(from, to);
        const after = lineText.slice(to, to + Math.max(0, limit - before.length - match.length));
        return { before, match, after };
    }

    return { workspaceEditToFileChanges, textEditsToChanges, applyTextEdits, planTextEdits, referencesToGroups, referencePreview };
});
