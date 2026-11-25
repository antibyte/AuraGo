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

    // --- Project replace: studio side (state, DOM, editors and the file API). ---

    const PROJECT_REPLACE_INVALID_TEXT = String.fromCharCode(0xfffd);

    // Diff and commit tabs carry the path of a file too; they are never replace targets.
    function projectReplaceIsVirtual(tab) {
        if (typeof isVirtualTab === 'function') return isVirtualTab(tab);
        return !!tab && (tab.kind === 'diff' || tab.kind === 'commit');
    }

    function projectReplaceTab(path) {
        const wanted = normalizeCodeStudioPath(path);
        return state.openTabs.find(tab => tab && tab.path === wanted && !projectReplaceIsVirtual(tab)) || null;
    }

    function projectReplaceViews(tab) {
        const views = [tab.view, tab.secondaryView, ...(Array.isArray(tab.views) ? tab.views : [])].filter(Boolean);
        return views.filter((view, index) => views.indexOf(view) === index);
    }

    function projectReplaceView(tab) {
        const views = projectReplaceViews(tab);
        return views.find(view => view.state && view.state.doc) || views.find(view => typeof view.setValue === 'function') || null;
    }

    // A tab without a live view keeps its CodeMirror state (with the undo history) in the snapshot
    // that renderEditor restores when the tab is shown again.
    function projectReplaceSnapshot(tab) {
        if (projectReplaceView(tab)) return null;
        const snapshot = tab.editorSnapshot;
        return snapshot && snapshot.state && snapshot.state.doc ? snapshot : null;
    }

    function projectReplaceTabText(tab) {
        const snapshot = projectReplaceSnapshot(tab);
        return snapshot ? snapshot.state.doc.toString() : editorValue(tab);
    }

    // Results for open files come from their buffers: the server only sees the files on disk, and a
    // replace needs the line numbers of the buffer it changes. Files the server did not list stay out.
    function overlayOpenTabResults(results) {
        const list = Array.isArray(results) ? results : [];
        const regex = buildReplaceRegExp(state.searchQuery, state.searchOptions);
        if (!regex) return list;
        const open = new Map();
        state.openTabs.forEach(tab => {
            if (tab && !projectReplaceIsVirtual(tab) && !open.has(tab.path)) open.set(tab.path, tab);
        });
        if (!open.size) return list;
        const out = [];
        const done = new Set();
        list.forEach(result => {
            const path = normalizeCodeStudioPath(result.path);
            const tab = open.get(path);
            if (!tab) {
                out.push(result);
            } else if (!done.has(path)) {
                done.add(path);
                out.push(...bufferSearchResults(result.path, projectReplaceTabText(tab), regex, 200));
            }
        });
        return out;
    }

    function projectReplaceJob() {
        const options = state.searchOptions || {};
        const regex = buildReplaceRegExp(state.searchQuery, options);
        return regex ? { regex, replacement: String(state.searchReplaceText || ''), regexMode: !!options.regex } : null;
    }

    // The result list grouped by file. Every row keeps its segments, so the preview and the occurrence
    // numbers come from the same computation as the write.
    function projectReplaceGroups(job) {
        const groups = [];
        const byPath = new Map();
        (state.searchResults || []).forEach(result => {
            const path = normalizeCodeStudioPath(result.path);
            let group = byPath.get(path);
            if (!group) {
                group = { path, label: searchResultLabel(path), rows: [], count: 0 };
                byPath.set(path, group);
                groups.push(group);
            }
            const line = Number(result.line) || 0;
            if (group.rows.some(row => row.line === line)) return;
            const preview = String(result.preview == null ? '' : result.preview).replace(/\r$/, '');
            const segments = replaceLineSegments(preview, job);
            const matches = segments.filter(segment => segment.match).length;
            group.rows.push({ line, preview, segments, matches });
            group.count += matches;
        });
        return groups;
    }

    function projectReplaceTotals(groups) {
        return groups.reduce((totals, group) => ({
            count: totals.count + group.count,
            files: totals.files + (group.count ? 1 : 0)
        }), { count: 0, files: 0 });
    }

    function projectReplaceRowMarkup() {
        const label = tr('codeStudio.replacePlaceholder', 'Replace');
        return `<div class="cs-search-replace-row" data-search-replace-row>
            <input name="replace" data-search-replace-input value="${esc(state.searchReplaceText || '')}" placeholder="${esc(label)}" aria-label="${esc(label)}" autocomplete="off" spellcheck="false">
            <button type="button" class="cs-button" data-search-replace-all>${buttonIcon('edit', 'R')}<span>${esc(tr('codeStudio.replaceAll', 'Replace All'))}</span></button>
            <span class="cs-search-replace-hint" data-search-replace-hint aria-live="polite"></span>
        </div>`;
    }

    function projectReplaceResultsMarkup() {
        const job = projectReplaceJob();
        const replaceLabel = tr('codeStudio.replaceOne', 'Replace');
        const fileLabel = tr('codeStudio.replaceInFile', 'Replace in File');
        return projectReplaceGroups(job).map(group => {
            const head = `<div class="cs-replace-file" data-replace-file="${esc(group.path)}">
                <span class="cs-replace-file-name" title="${esc(group.path)}"><strong>${esc(group.label)}</strong><em class="cs-replace-count">${esc(group.count)}</em></span>
                <button type="button" class="cs-replace-action" data-replace-file-action data-replace-path="${esc(group.path)}" data-replace-count="${esc(group.count)}" title="${esc(fileLabel)}">${esc(fileLabel)}</button>
            </div>`;
            const rows = group.rows.map(row => {
                const open = (code, tail) => `<button type="button" class="cs-replace-preview" data-search-path="${esc(group.path)}" data-search-line="${esc(row.line)}" title="${esc(group.label)}:${esc(row.line)}"><em class="cs-search-line">${esc(row.line)}</em><code>${code}</code>${tail || ''}</button>`;
                if (!row.matches) {
                    const tail = job ? `<span class="cs-replace-nomatch">${esc(tr('codeStudio.replaceNoMatchLine', 'no match'))}</span>` : '';
                    return `<div class="cs-replace-match is-nomatch">${open(esc(row.preview.trim()), tail)}</div>`;
                }
                let html = '';
                for (let occurrence = 0; occurrence < row.matches; occurrence += 1) {
                    const parts = replaceMatchPreview(row.segments, occurrence);
                    const code = `${esc(parts.before)}<del class="cs-replace-old">${esc(parts.old)}</del><ins class="cs-replace-new">${esc(parts.next)}</ins>${esc(parts.after)}`;
                    html += `<div class="cs-replace-match">${open(code)}<button type="button" class="cs-replace-action" data-replace-one data-replace-path="${esc(group.path)}" data-replace-line="${esc(row.line)}" data-replace-occurrence="${occurrence}" title="${esc(replaceLabel)}">${esc(replaceLabel)}</button></div>`;
                }
                return html;
            }).join('');
            return head + rows;
        }).join('');
    }

    function wireProjectReplaceControls(panel) {
        const toggle = panel.querySelector('[data-search-replace-toggle]');
        if (toggle) toggle.addEventListener('click', bind(() => toggleProjectReplace()));
        const input = panel.querySelector('[data-search-replace-input]');
        if (input) {
            input.addEventListener('input', bind(event => {
                state.searchReplaceText = event.target.value;
                renderSearchResults();
            }));
        }
        const all = panel.querySelector('[data-search-replace-all]');
        if (all) all.addEventListener('click', bind(() => projectReplaceAll()));
    }

    function wireProjectReplaceResults(box) {
        box.querySelectorAll('[data-replace-file-action]').forEach(button => {
            button.addEventListener('click', bind(() => projectReplaceFile(button.dataset.replacePath)));
        });
        box.querySelectorAll('[data-replace-one]').forEach(button => {
            button.addEventListener('click', bind(() => projectReplaceMatch(
                button.dataset.replacePath,
                Number(button.dataset.replaceLine),
                Number(button.dataset.replaceOccurrence)
            )));
        });
    }

    function toggleProjectReplace() {
        state.searchReplaceOpen = !state.searchReplaceOpen;
        renderSearchPanel();
        const field = shellPart(state.searchReplaceOpen ? '[data-search-replace-input]' : '[data-search] input[name="q"]');
        if (field) field.focus();
    }

    function projectReplaceHint(job, writable) {
        if (!writable) return tr('codeStudio.replaceReadOnly', 'Read-only: replacing is turned off.');
        if (state.searchQuery && !job) return tr('codeStudio.replaceInvalidRegex', 'This expression is not valid JavaScript regex syntax.');
        if (state.searchTruncated) return tr('codeStudio.replaceTruncatedHint', 'More than 200 results: narrow the search to use Replace All. Replace in File still works for the listed files.');
        if (job && job.regexMode) return tr('codeStudio.replaceGroupsHint', 'Use $1, $2 for capture groups.');
        return '';
    }

    // Runs after every result render and from updatePolicyControls, so read-only switches land at once.
    function updateProjectReplaceControls() {
        const root = studioRoot();
        const panel = root && root.querySelector('[data-search]');
        if (!panel || panel.hidden || !state.searchReplaceOpen) return;
        const writable = canWrite();
        const busy = !!state.projectReplaceBusy;
        const job = projectReplaceJob();
        const totals = job && state.searchPerformed ? projectReplaceTotals(projectReplaceGroups(job)) : { count: 0, files: 0 };
        const input = panel.querySelector('[data-search-replace-input]');
        if (input) input.disabled = !writable;
        const all = panel.querySelector('[data-search-replace-all]');
        if (all) {
            all.disabled = !writable || busy || !job || !!state.searchTruncated || totals.count === 0;
            all.title = state.searchTruncated
                ? tr('codeStudio.replaceTruncatedHint', 'More than 200 results: narrow the search to use Replace All. Replace in File still works for the listed files.')
                : tr('codeStudio.replaceAll', 'Replace All');
        }
        panel.querySelectorAll('[data-replace-file-action], [data-replace-one]').forEach(button => {
            button.disabled = !writable || busy || !job || button.dataset.replaceCount === '0';
        });
        const hint = panel.querySelector('[data-search-replace-hint]');
        if (hint) hint.textContent = projectReplaceHint(job, writable);
    }

    function projectReplaceJobFile(group) {
        return {
            path: group.path,
            label: group.label,
            targets: group.rows.filter(row => row.matches > 0).map(row => ({ line: row.line, preview: row.preview, occurrence: null }))
        };
    }

    function projectReplaceReady() {
        if (!canWrite() || state.projectReplaceBusy) return null;
        const job = projectReplaceJob();
        if (!job) flashStatus(tr('codeStudio.replaceInvalidRegex', 'This expression is not valid JavaScript regex syntax.'));
        return job;
    }

    async function projectReplaceAll() {
        const target = state;
        const job = projectReplaceReady();
        if (!job || target.searchTruncated) return;
        const groups = projectReplaceGroups(job);
        const totals = projectReplaceTotals(groups);
        if (!totals.count) {
            flashStatus(tr('codeStudio.replaceNothing', 'Nothing to replace.'));
            return;
        }
        const confirmed = await confirmValue(
            tr('codeStudio.replaceAllConfirm', 'Replace {{count}} occurrences in {{files}} files?', { count: totals.count, files: totals.files }),
            { confirmLabel: tr('codeStudio.replaceAll', 'Replace All'), confirmIcon: 'edit' }
        );
        if (!confirmed || !isLiveInstance(target)) return;
        await runProjectReplace(target, job, groups.filter(group => group.count > 0).map(projectReplaceJobFile), groups.filter(group => !group.count), 'all');
    }

    async function projectReplaceFile(path) {
        const target = state;
        const job = projectReplaceReady();
        if (!job) return;
        const group = projectReplaceGroups(job).find(item => item.path === normalizeCodeStudioPath(path));
        if (!group) return;
        if (!group.count) {
            flashStatus(tr('codeStudio.replaceNothing', 'Nothing to replace.'));
            return;
        }
        await runProjectReplace(target, job, [projectReplaceJobFile(group)], [], 'file');
    }

    // One match: the exact line and the occurrence index on that line, re-checked before anything changes.
    async function projectReplaceMatch(path, line, occurrence) {
        const target = state;
        const job = projectReplaceReady();
        if (!job) return;
        const group = projectReplaceGroups(job).find(item => item.path === normalizeCodeStudioPath(path));
        const row = group && group.rows.find(item => item.line === line);
        if (!row || !Number.isInteger(occurrence) || occurrence < 0 || occurrence >= row.matches) return;
        await runProjectReplace(target, job, [{ path: group.path, label: group.label, targets: [{ line, preview: row.preview, occurrence }] }], [], 'one');
    }

    // Files one after the other; a failing file is reported and the loop goes on.
    async function runProjectReplace(target, job, files, noMatchGroups, mode) {
        if (!canWrite(target) || target.projectReplaceBusy) return null;
        target.projectReplaceBusy = true;
        const report = { mode, count: 0, files: 0, buffers: 0, stale: 0, skipped: [] };
        (noMatchGroups || []).forEach(group => report.skipped.push(projectReplacePlanSkip({ status: 'nomatch' }, group).message));
        runWithInstance(target, () => {
            updateProjectReplaceControls();
            renderStatus(tr('codeStudio.replaceRunning', 'Replacing...'));
        });
        try {
            for (const file of files) {
                if (!isLiveInstance(target)) return null;
                let outcome;
                try {
                    outcome = await projectReplaceInFile(target, file, job);
                } catch (err) {
                    outcome = projectReplaceFailure(file, err);
                }
                if (outcome.status === 'written' || outcome.status === 'buffer') {
                    report.count += outcome.count;
                    report.files += 1;
                    if (outcome.status === 'buffer') report.buffers += 1;
                } else {
                    if (outcome.status === 'stale') report.stale += 1;
                    report.skipped.push(outcome.message);
                }
            }
        } finally {
            target.projectReplaceBusy = false;
        }
        if (!isLiveInstance(target)) return null;
        runWithInstance(target, () => finishProjectReplace(report));
        await runAsyncStep(target, rerunProjectSearch);
        return report;
    }

    async function projectReplaceInFile(target, file, job) {
        const open = runWithInstance(target, () => projectReplaceTab(file.path));
        if (open) return runWithInstance(target, () => projectReplaceInTab(open, file, job));
        let content;
        try {
            const result = await apiClient.file(file.path);
            content = result && typeof result.content === 'string' ? result.content : '';
        } catch (err) {
            return projectReplaceFailure(file, err);
        }
        if (!isLiveInstance(target)) return projectReplaceFailure(file, null);
        // A tab opened while the file was read takes the change in its buffer instead.
        const opened = runWithInstance(target, () => projectReplaceTab(file.path));
        if (opened) return runWithInstance(target, () => projectReplaceInTab(opened, file, job));
        if (content.includes(PROJECT_REPLACE_INVALID_TEXT)) {
            return projectReplaceSkip('error', 'codeStudio.replaceSkippedEncoding', '{{path}}: not plain UTF-8 text, skipped', file.label);
        }
        const plan = planFileReplace(content, file.targets, job);
        if (plan.status !== 'ok') return projectReplacePlanSkip(plan, file);
        if (!canWrite(target)) return { status: 'error', message: file.label + ': ' + tr('codeStudio.replaceReadOnly', 'Read-only: replacing is turned off.') };
        try {
            await apiClient.writeFile(file.path, plan.content);
        } catch (err) {
            return projectReplaceFailure(file, err);
        }
        // IDE package 2b: the language servers re-read the closed file the replace just wrote.
        if (isLiveInstance(target)) runWithInstance(target, () => lspNotifyFilesChanged([{ path: file.path, type: 2 }]));
        const late = isLiveInstance(target) ? runWithInstance(target, () => projectReplaceTab(file.path)) : null;
        if (late) await reloadCleanTabFromDisk(target, late);
        return { status: 'written', count: plan.count };
    }

    function projectReplaceInTab(tab, file, job) {
        if (!canWrite()) return { status: 'error', message: file.label + ': ' + tr('codeStudio.replaceReadOnly', 'Read-only: replacing is turned off.') };
        if (tab.pathMutation) return projectReplacePlanSkip({ status: 'stale' }, file);
        const snapshot = projectReplaceSnapshot(tab);
        const plan = planFileReplace(projectReplaceTabText(tab), file.targets, job);
        if (plan.status !== 'ok') return projectReplacePlanSkip(plan, file);
        applyProjectReplaceToTab(tab, plan, snapshot);
        return { status: 'buffer', count: plan.count };
    }

    // One transaction is one undo step; the editor's update listener marks the tab modified. In a split
    // the two views of one file are linked (syncLinkedView), so one dispatch reaches both; any view that
    // still differs afterwards gets the same changes, so every live view ends with the new text.
    function applyProjectReplaceToTab(tab, plan, snapshot) {
        const view = projectReplaceView(tab);
        if (view && view.state && view.state.doc) {
            view.dispatch({ changes: plan.changes, userEvent: 'input.replace' });
            projectReplaceViews(tab).forEach(other => {
                if (other === view) return;
                if (other.state && other.state.doc && other.state.doc.toString() !== plan.content) {
                    other.dispatch({ changes: plan.changes, userEvent: 'input.replace' });
                } else if (!other.state && typeof other.setValue === 'function') {
                    other.setValue(plan.content);
                }
            });
            return;
        }
        if (view && typeof view.setValue === 'function') {
            view.setValue(plan.content);
            return;
        }
        if (snapshot) snapshot.state = snapshot.state.update({ changes: plan.changes, userEvent: 'input.replace' }).state;
        tab.secondaryEditorSnapshot = null;
        updateTabContent(tab, plan.content);
    }

    function projectReplaceSkip(status, key, fallback, label) {
        return { status, message: tr(key, fallback, { path: label }) };
    }

    function projectReplacePlanSkip(plan, file) {
        if (plan.status === 'stale') {
            return projectReplaceSkip('stale', 'codeStudio.replaceSkippedStale', '{{path}}: changed since the search, skipped', file.label);
        }
        return projectReplaceSkip('nomatch', 'codeStudio.replaceSkippedNoMatch', '{{path}}: the JavaScript expression finds no match, skipped', file.label);
    }

    // The server answers 413 for files above its size cap, on GET and on PUT.
    function projectReplaceFailure(file, err) {
        if (err && err.status === 413) {
            return projectReplaceSkip('error', 'codeStudio.replaceSkippedTooLarge', '{{path}}: larger than the file size limit, skipped', file.label);
        }
        return { status: 'error', message: file.label + ': ' + (err && err.message ? err.message : String(err || '')) };
    }

    function finishProjectReplace(report) {
        state.projectReplaceReport = report;
        const summary = tr('codeStudio.replaceSummary', 'Replaced {{count}} occurrences in {{files}} files.', { count: report.count, files: report.files });
        const lines = [summary];
        if (report.buffers) lines.push(tr('codeStudio.replaceUnsavedNote', '{{count}} open files were changed in the editor and are not saved yet.', { count: report.buffers }));
        appendOutput(lines.concat(report.skipped).join('\n'), tr('codeStudio.replaceOne', 'Replace'));
        if (report.files > report.buffers) scheduleGitRefresh();
        renderTabs();
        renderWindowMenus();
        renderStatus();
        updateProjectReplaceControls();
        if (report.mode === 'one' && report.stale) {
            flashStatus(tr('codeStudio.replaceStale', 'The file changed since the search. Searching again.'));
        } else if (report.skipped.length) {
            setActivePanel('output');
            flashStatus(tr('codeStudio.replaceSomeSkipped', '{{count}} files skipped, see Output.', { count: report.skipped.length }));
        } else {
            flashStatus(summary);
        }
    }

    // Runs the last search again with its own options, not with whatever the form shows now.
    function rerunProjectSearch() {
        if (!state.searchQuery) return undefined;
        const options = state.searchOptions || {};
        const data = new FormData();
        data.set('q', state.searchQuery);
        if (options.case) data.set('case', 'on');
        if (options.whole) data.set('whole', 'on');
        if (options.regex) data.set('regex', 'on');
        data.set('include', options.include || '');
        data.set('exclude', options.exclude || '');
        return runSearch(data);
    }
