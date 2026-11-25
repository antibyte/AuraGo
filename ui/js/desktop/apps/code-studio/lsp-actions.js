    // IDE package 2C: language server actions in the Code Studio editor.
    // - Find references (Shift+F12) fills a References tab in the bottom panel, grouped by file with line previews.
    // - Rename (F2 in the editor) asks for the new name in a small box at the symbol and previews the edit.
    // - Quick fixes (Ctrl+. or the lightbulb in the gutter) list the server's code actions for the line.
    // Every edit goes through one applier: open tabs change in their editor buffer as one undoable step and stay
    // unsaved; closed files are read and written through the file API. The pure helpers are in lsp-edits.js
    // (CodeStudioLSPEdits). The connection, the requests and the editor hook come from lsp.js (plan 2B). Every
    // top-level name here starts with lspAct or LSP_ACT_, so nothing clashes with lsp.js in the shared bundle closure;
    // the hooks that panel.js and shortcuts.js call (ensureReferencesPanelParts, renderReferencesPanel, clearReferences
    // and the three palette commands) are the only exceptions.

    const LSP_ACT_TIMEOUT_MS = 15000;
    // Problems published by a language server carry the server's name as their source (lsp.js, spec section 3).
    const LSP_ACT_SOURCES = new Set(['gopls', 'typescript', 'pyright', 'clangd']);
    const LSP_ACT_SEVERITY = { error: 1, warning: 2, info: 3 };
    const LSP_ACT_INVALID_TEXT = String.fromCharCode(0xfffd);
    const LSP_ACT_PREVIEW_FILES = 60;
    const LSP_ACT_BULB_SVG = '<svg viewBox="0 0 16 16" width="12" height="12" fill="currentColor" aria-hidden="true">'
        + '<path d="M8 1.5a4.5 4.5 0 0 0-2.62 8.16c.39.28.62.72.62 1.2v.64h4v-.64c0-.48.23-.92.62-1.2A4.5 4.5 0 0 0 8 1.5z"/>'
        + '<path d="M6 12.5h4v.75c0 .69-.56 1.25-1.25 1.25h-1.5C6.56 14.5 6 13.94 6 13.25z"/></svg>';

    // --- Shared helpers ---

    function lspActTabShows(tab, view) {
        return !!tab && !!view && (tab.view === view || tab.secondaryView === view || (tab.views || []).includes(view));
    }

    // The Code Studio window that shows a view. Keymaps, gutters and palette commands may run outside the studio's
    // instance context, so the owner is found through the tabs of every open window.
    function lspActInstanceForView(view) {
        for (const instance of instances.values()) {
            if ((instance.openTabs || []).some(tab => lspActTabShows(tab, view))) return instance;
        }
        return null;
    }

    // Starts an action for a view inside its window's context and returns true when the view belongs to a window.
    // The action is async; a failure ends as a status notice.
    function lspActStart(view, action) {
        const instance = lspActInstanceForView(view);
        if (!instance) return false;
        let pending;
        try {
            pending = runWithInstance(instance, () => action(view));
        } catch (err) {
            pending = Promise.reject(err);
        }
        Promise.resolve(pending).catch(err => {
            if (isLiveInstance(instance)) runWithInstance(instance, () => lspActReportError(err));
        });
        return true;
    }

    // The file tab, path, language and URI of an editor view whose file has a language server.
    function lspActContext(view) {
        if (!view || !view.state || !view.state.doc) return null;
        const tab = state.openTabs.find(item => isSourceTab(item) && lspActTabShows(item, view));
        const language = tab ? CodeStudioLSPProtocol.languageForPath(tab.path) : null;
        return language ? { tab, path: tab.path, language, uri: CodeStudioLSPProtocol.uriForPath(tab.path) } : null;
    }

    function lspActOpenTab(path) {
        return state.openTabs.find(tab => isSourceTab(tab) && tab.path === path) || null;
    }

    // The view that owns the undo history comes first: a split of one file links its second view to tab.view.
    function lspActLiveView(tab) {
        const views = [tab.view, tab.secondaryView].filter(Boolean);
        return views.find(view => view.state && view.state.doc) || views.find(view => typeof view.setValue === 'function') || null;
    }

    // The document of an open tab as an edit would see it: the live editor, a saved editor state or the plain content.
    function lspActDocKey(tab) {
        const view = lspActLiveView(tab);
        if (view && view.state && view.state.doc) return view.state.doc;
        if (view && typeof view.getValue === 'function') return view.getValue();
        const snapshot = tab.editorSnapshot;
        if (snapshot && snapshot.state && snapshot.state.doc) return snapshot.state.doc;
        return String(tab.content || '');
    }

    function lspActSameDoc(before, now) {
        if (before === now) return true;
        if (before == null || now == null) return false;
        return (typeof before === 'string' || typeof now === 'string') && String(before) === String(now);
    }

    // What every open file looked like when a request went out; an edit for a buffer that changed since is stale.
    function lspActCaptureDocs() {
        return new Map(state.openTabs.filter(isSourceTab).map(tab => [tab, lspActDocKey(tab)]));
    }

    // LSP position to a CodeMirror offset, clamped into the document.
    function lspActOffset(doc, position) {
        const lineIndex = Math.min(Math.max(0, Math.floor(Number(position && position.line) || 0)), doc.lines - 1);
        const line = doc.line(lineIndex + 1);
        return line.from + Math.min(Math.max(0, Math.floor(Number(position && position.character) || 0)), line.length);
    }

    function lspActErrorText(err) {
        if (!err) return '';
        if (typeof err === 'string') return err;
        return String(err.message || err.code || err);
    }

    // lspRequest rejects with Error('timeout'); the client's own timer says "Request timed out".
    function lspActIsTimeout(err) {
        const text = lspActErrorText(err);
        return text === 'timeout' || /timed out/i.test(text);
    }

    function lspActReportError(err) {
        flashStatus(lspActIsTimeout(err)
            ? tr('codeStudio.lspTimeout', 'The language server did not answer in time. Try again.')
            : tr('codeStudio.lspRequestFailed', 'Language server error: {{message}}', { message: lspActErrorText(err) }), 5000);
    }

    function lspActClearStatus(target) {
        if (!isLiveInstance(target)) return;
        runWithInstance(target, () => {
            if (target.statusTimer) clearTimeout(target.statusTimer);
            target.statusTimer = null;
            target.statusMessage = '';
            renderStatus();
        });
    }

    // The window's client for a language, or null with a notice when its server is not ready.
    async function lspActClient(target, language) {
        let client = null;
        if (runWithInstance(target, () => lspClientState(language)) === 'ready') {
            try {
                client = await runWithInstance(target, () => lspEnsureClient(language));
            } catch (_) {
                client = null;
            }
        }
        if (!isLiveInstance(target)) return null;
        if (!client) runWithInstance(target, () => flashStatus(tr('codeStudio.lspNotReady', 'No language server is ready for this file.')));
        return client;
    }

    // Requests with the 15 s budget of references and rename, sent from the window that asks.
    function lspActRequest(target, language, method, params) {
        return runWithInstance(target, () => lspRequest(language, method, params, LSP_ACT_TIMEOUT_MS));
    }

    function lspActReadOnlyText() {
        return tr('codeStudio.lspReadOnly', 'Read-only: code changes are turned off.');
    }

    function lspActLabel(path, fallback) {
        if (path) return searchResultLabel(path);
        return String(fallback || '') || tr('codeStudio.problemWorkspace', 'Workspace');
    }

    function lspActSkipMessage(item) {
        const label = lspActLabel(item.path, item.uri);
        if (item.reason === 'outside') return tr('codeStudio.lspSkipOutside', '{{path}}: outside the workspace', { path: label });
        if (item.reason === 'invalid') return tr('codeStudio.lspSkipInvalid', '{{path}}: the server sent an invalid change', { path: label });
        return tr('codeStudio.lspSkipFileOperation', '{{path}}: creating, renaming or deleting files is not supported', { path: label });
    }

    // The server answers 413 for files above its size cap; lsp-edits.js marks rejected edits with editError.
    function lspActFailure(label, err) {
        if (err && err.status === 413) return tr('codeStudio.replaceSkippedTooLarge', '{{path}}: larger than the file size limit, skipped', { path: label });
        if (err && err.editError) return tr('codeStudio.lspFailEdits', '{{path}}: the server sent overlapping or out-of-range changes', { path: label });
        return label + ': ' + lspActErrorText(err);
    }

    // --- Preview and result dialog ---

    // Lists the files of an edit with their change counts before anything is applied, and after Apply the files that
    // failed. Mounted on the body like the other Code Studio dialogs; Escape and the backdrop cancel the preview.
    // options: { title, files, skipped } for the preview, or { title, result: { summary, note, failures } }.
    // Returns { decision, showResult, close }; decision resolves to true on Apply.
    function lspActDialog(target, options) {
        const settings = options || {};
        const title = settings.title || tr('codeStudio.lspPreviewTitle', 'Apply these changes?');
        const overlay = document.createElement('div');
        overlay.className = 'cs-modal-backdrop';
        document.body.appendChild(overlay);
        const releaseFocus = trapDialogFocus(overlay);
        let mode = 'preview';
        let closed = false;
        let unregister = () => {};
        let resolveDecision = () => {};
        const decision = new Promise(resolve => { resolveDecision = resolve; });
        const onKey = event => {
            if (event.key !== 'Escape' || mode === 'busy') return;
            event.preventDefault();
            event.stopPropagation();
            close();
        };
        function close() {
            if (closed) return;
            closed = true;
            resolveDecision(false);
            unregister();
            overlay.removeEventListener('keydown', onKey);
            overlay.remove();
            releaseFocus();
        }
        const list = (items, className) => (items.length
            ? `<ul class="${className}">${items.map(text => `<li>${esc(text)}</li>`).join('')}</ul>`
            : '');
        const frame = (body, actions) => `<div class="cs-modal cs-lsp-dialog" role="dialog" aria-modal="true" aria-label="${esc(title)}">
            <h3 class="cs-lsp-dialog-title" title="${esc(title)}">${esc(title)}</h3>
            ${body}
            <div class="cs-modal-actions">${actions}</div>
        </div>`;
        function showPreview() {
            const files = settings.files || [];
            const skipped = settings.skipped || [];
            const total = files.reduce((sum, file) => sum + file.count, 0);
            const rows = files.map(file => {
                const open = runWithInstance(target, () => !!lspActOpenTab(file.path));
                const where = open
                    ? tr('codeStudio.lspPreviewOpenTab', 'open tab, stays unsaved')
                    : tr('codeStudio.lspPreviewDisk', 'written to disk');
                return `<li><span class="cs-lsp-dialog-path" title="${esc(file.path)}">${esc(searchResultLabel(file.path))}</span><em class="cs-replace-count">${esc(file.count)}</em><span class="cs-lsp-dialog-where${open ? ' is-open' : ''}">${esc(where)}</span></li>`;
            }).join('');
            const skippedPart = skipped.length
                ? `<p class="cs-lsp-dialog-note">${esc(tr('codeStudio.lspPreviewSkipped', 'These parts are not applied:'))}</p>${list(skipped, 'cs-lsp-dialog-skipped')}`
                : '';
            overlay.innerHTML = frame(
                `<p class="cs-lsp-dialog-summary">${esc(tr('codeStudio.lspPreviewSummary', '{{count}} changes in {{files}} files', { count: total, files: files.length }))}</p><ul class="cs-lsp-dialog-files">${rows}</ul>${skippedPart}`,
                `<button type="button" class="cs-button" data-lsp-cancel>${buttonIcon('x', 'X')}<span>${esc(tr('desktop.cancel', 'Cancel'))}</span></button><button type="button" class="cs-button primary" data-lsp-apply>${buttonIcon('check-square', 'Y')}<span>${esc(tr('codeStudio.lspPreviewApply', 'Apply'))}</span></button>`
            );
            overlay.querySelector('[data-lsp-cancel]').addEventListener('click', close);
            const apply = overlay.querySelector('[data-lsp-apply]');
            apply.addEventListener('click', () => {
                mode = 'busy';
                overlay.querySelectorAll('button').forEach(button => { button.disabled = true; });
                resolveDecision(true);
            });
            apply.focus();
        }
        function showResult(result) {
            if (closed) return;
            mode = 'result';
            const failures = result.failures || [];
            const failedPart = failures.length
                ? `<p class="cs-lsp-dialog-note is-error">${esc(tr('codeStudio.lspResultFailed', 'These files were not changed:'))}</p>${list(failures, 'cs-lsp-dialog-skipped')}`
                : '';
            overlay.innerHTML = frame(
                `<p class="cs-lsp-dialog-summary">${esc(result.summary || '')}</p>${result.note ? `<p class="cs-lsp-dialog-note">${esc(result.note)}</p>` : ''}${failedPart}`,
                `<button type="button" class="cs-button primary" data-lsp-close>${buttonIcon('check-square', 'Y')}<span>${esc(tr('desktop.close', 'Close'))}</span></button>`
            );
            const button = overlay.querySelector('[data-lsp-close]');
            button.addEventListener('click', close);
            button.focus();
        }
        runWithInstance(target, () => { unregister = registerDisposer(close); });
        overlay.addEventListener('keydown', onKey);
        overlay.addEventListener('mousedown', event => {
            if (event.target === overlay && mode !== 'busy') close();
        });
        if (settings.result) showResult(settings.result);
        else showPreview();
        return { decision, showResult, close };
    }

    // --- WorkspaceEdit applier ---

    // Applies a WorkspaceEdit. options: { mode: 'rename' | 'fix' | 'server', title, originPath, docsBefore, focusView }.
    // A rename always shows the preview. Other edits show it when they touch another file than originPath (the file
    // the user is in) or carry parts that cannot be applied. Resolves to { applied, cancelled, reason }.
    async function lspActApplyWorkspaceEdit(target, edit, options) {
        const settings = options || {};
        if (!canWrite(target)) {
            if (isLiveInstance(target)) runWithInstance(target, () => flashStatus(lspActReadOnlyText()));
            return { applied: false, reason: 'Code Studio is read-only.' };
        }
        if (target.lspActBusy) {
            runWithInstance(target, () => flashStatus(tr('codeStudio.lspWorking', 'Waiting for the language server...')));
            return { applied: false, reason: 'Another edit is being applied.' };
        }
        const plan = CodeStudioLSPEdits.workspaceEditToFileChanges(edit);
        const skipped = plan.skipped.map(lspActSkipMessage);
        if (!plan.files.length) {
            if (skipped.length) {
                runWithInstance(target, () => lspActDialog(target, { title: settings.title, result: {
                    summary: tr('codeStudio.lspResultSummary', 'Applied {{count}} changes in {{files}} files.', { count: 0, files: 0 }),
                    failures: skipped
                } }));
                return { applied: false, reason: 'The edit only contains changes that cannot be applied.' };
            }
            runWithInstance(target, () => flashStatus(tr('codeStudio.lspNoChanges', 'Nothing to change.')));
            return { applied: true };
        }
        const direct = settings.mode !== 'rename' && !skipped.length && plan.files.length === 1
            && !!settings.originPath && plan.files[0].path === settings.originPath;
        target.lspActBusy = true;
        let dialog = null;
        let report;
        try {
            if (!direct) {
                dialog = runWithInstance(target, () => lspActDialog(target, { title: settings.title, files: plan.files, skipped }));
                const confirmed = await dialog.decision;
                if (!confirmed || !canWrite(target)) {
                    dialog.close();
                    if (confirmed && isLiveInstance(target)) runWithInstance(target, () => flashStatus(lspActReadOnlyText()));
                    return { applied: false, cancelled: true, reason: 'The user cancelled the edit.' };
                }
            }
            report = await lspActApplyFiles(target, plan.files, settings);
        } finally {
            target.lspActBusy = false;
        }
        report.failures = skipped.concat(report.failures);
        if (isLiveInstance(target)) runWithInstance(target, () => lspActFinish(target, report, settings, dialog));
        else if (dialog) dialog.close();
        return report.failures.length ? { applied: false, reason: report.failures.join('\n') } : { applied: true };
    }

    // Files one after the other: one file's failure never stops the others, and what was applied stays applied.
    async function lspActApplyFiles(target, files, settings) {
        const report = { count: 0, files: 0, buffers: 0, written: [], failures: [] };
        for (const file of files) {
            if (!isLiveInstance(target)) break;
            const label = searchResultLabel(file.path);
            let outcome;
            try {
                outcome = await lspActApplyFile(target, file, label, settings);
            } catch (err) {
                outcome = { ok: false, message: lspActFailure(label, err) };
            }
            if (outcome.ok) {
                report.count += file.count;
                report.files += 1;
                if (outcome.buffer) report.buffers += 1;
                else report.written.push(file.path);
            } else {
                report.failures.push(outcome.message);
            }
        }
        return report;
    }

    // An open tab takes the change in its buffer. A closed file is read, changed and written back with PUT.
    async function lspActApplyFile(target, file, label, settings) {
        const open = runWithInstance(target, () => lspActOpenTab(file.path));
        if (open) return runWithInstance(target, () => lspActApplyToTab(open, file, label, settings));
        let content;
        try {
            const result = await apiClient.file(file.path);
            content = result && typeof result.content === 'string' ? result.content : '';
        } catch (err) {
            return { ok: false, message: lspActFailure(label, err) };
        }
        if (!isLiveInstance(target)) return { ok: false, message: label };
        // A tab opened while the file was read takes the change in its buffer instead.
        const opened = runWithInstance(target, () => lspActOpenTab(file.path));
        if (opened) return runWithInstance(target, () => lspActApplyToTab(opened, file, label, settings));
        // Text with U+FFFD is not valid UTF-8; writing it back would corrupt the file.
        if (content.includes(LSP_ACT_INVALID_TEXT)) {
            return { ok: false, message: tr('codeStudio.replaceSkippedEncoding', '{{path}}: not plain UTF-8 text, skipped', { path: label }) };
        }
        let next;
        try {
            next = CodeStudioLSPEdits.planTextEdits(content, file.batches).text;
        } catch (err) {
            return { ok: false, message: lspActFailure(label, err) };
        }
        if (!canWrite(target)) return { ok: false, message: label + ': ' + lspActReadOnlyText() };
        try {
            await apiClient.writeFile(file.path, next);
        } catch (err) {
            return { ok: false, message: lspActFailure(label, err) };
        }
        // A tab opened after the write re-reads the file while its buffer is untouched.
        const late = isLiveInstance(target) ? runWithInstance(target, () => lspActOpenTab(file.path)) : null;
        if (late) await reloadCleanTabFromDisk(target, late);
        return { ok: true, buffer: false };
    }

    // Open files change in their buffer and stay unsaved.
    // - A live editor gets one transaction: one undo step that never joins typing. A split of the same file follows
    //   through its linked view, so the change is dispatched once, on tab.view.
    // - The textarea fallback takes the new text.
    // - A tab without a live editor changes its saved editor state, which the next render restores together with the
    //   undo history. With unsaved changes it is refused: the server may have computed the edit on the saved file.
    function lspActApplyToTab(tab, file, label, settings) {
        if (!canWrite()) return { ok: false, message: label + ': ' + lspActReadOnlyText() };
        const stale = { ok: false, message: tr('codeStudio.lspFailStale', '{{path}}: changed while the language server was working', { path: label }) };
        if (tab.pathMutation) return stale;
        if (settings.docsBefore && settings.docsBefore.has(tab) && !lspActSameDoc(settings.docsBefore.get(tab), lspActDocKey(tab))) return stale;
        try {
            const view = lspActLiveView(tab);
            if (view && view.state && view.state.doc) {
                const plan = CodeStudioLSPEdits.planTextEdits(view.state.doc.toString(), file.batches);
                if (plan.steps.length) view.dispatch(...lspActSpecs(plan.steps));
                return { ok: true, buffer: true };
            }
            if (view && typeof view.setValue === 'function') {
                view.setValue(CodeStudioLSPEdits.planTextEdits(view.getValue(), file.batches).text);
                return { ok: true, buffer: true };
            }
            if (tab.modified) {
                return { ok: false, message: tr('codeStudio.lspFailHiddenUnsaved', '{{path}}: unsaved changes in a hidden tab; save it and try again', { path: label }) };
            }
            const snapshot = tab.editorSnapshot && tab.editorSnapshot.state && tab.editorSnapshot.state.doc ? tab.editorSnapshot : null;
            const plan = CodeStudioLSPEdits.planTextEdits(snapshot ? snapshot.state.doc.toString() : String(tab.content || ''), file.batches);
            if (snapshot && plan.steps.length) snapshot.state = snapshot.state.update(...lspActSpecs(plan.steps)).state;
            updateTabContent(tab, plan.text);
            return { ok: true, buffer: true };
        } catch (err) {
            return { ok: false, message: lspActFailure(label, err) };
        }
    }

    // Transaction specs for the steps of planTextEdits: later steps refer to the text the earlier ones produced.
    function lspActSpecs(steps) {
        const cm = state.cmModule;
        return steps.map((changes, index) => {
            const spec = { changes, userEvent: 'input.lsp' };
            if (index > 0) spec.sequential = true;
            else if (cm && cm.isolateHistory) spec.annotations = cm.isolateHistory.of('full');
            return spec;
        });
    }

    function lspActFinish(target, report, settings, dialog) {
        if (report.written.length) {
            lspNotifyFilesChanged(report.written.map(path => ({ path, type: 2 })));
            scheduleGitRefresh();
        }
        renderTabs();
        renderStatus();
        const summary = tr('codeStudio.lspResultSummary', 'Applied {{count}} changes in {{files}} files.', { count: report.count, files: report.files });
        const note = report.buffers && report.files > 1
            ? tr('codeStudio.replaceUnsavedNote', '{{count}} open files were changed in the editor and are not saved yet.', { count: report.buffers })
            : '';
        if (report.failures.length) {
            const result = { summary, note, failures: report.failures };
            if (dialog) dialog.showResult(result);
            else lspActDialog(target, { title: settings.title, result });
            return;
        }
        if (dialog) dialog.close();
        flashStatus(note ? summary + ' ' + note : summary);
        const view = settings.focusView;
        if (view && view.dom && view.dom.isConnected) view.focus();
    }

    // --- Server-sent workspace/applyEdit ---

    // gopls and typescript-language-server send workspace/applyEdit while they run a command. It goes through the same
    // applier and previews when it touches another file than the active one.
    async function lspActServerApplyEdit(_language, params) {
        const target = isLiveInstance(state) ? state : currentInstance();
        if (!target) return { applied: false, failureReason: 'Code Studio is not open.' };
        target.lspActServerEdits = (target.lspActServerEdits || 0) + 1;
        const origin = runWithInstance(target, () => {
            const tab = activeTab();
            return isSourceTab(tab) ? tab.path : '';
        });
        const title = params && typeof params.label === 'string' && params.label.trim() ? params.label : '';
        const outcome = await runWithInstance(target, () => lspActApplyWorkspaceEdit(target, params && params.edit, { mode: 'server', title, originPath: origin }));
        return outcome.applied ? { applied: true } : { applied: false, failureReason: outcome.reason || 'The edit was not applied.' };
    }

    lspOnServerRequest('workspace/applyEdit', lspActServerApplyEdit);

    // --- Floating boxes at the cursor (rename box and quick-fix menu) ---

    // Closes open Code Studio menus the way core.js does, so only one floating box is open at a time.
    function lspActCloseMenus() {
        document.querySelectorAll('.cs-context-menu').forEach(menu => {
            if (typeof menu.__codeStudioCleanup === 'function') menu.__codeStudioCleanup();
            else menu.remove();
        });
    }

    // Puts a fixed box under a document position, kept inside the viewport.
    function lspActPlace(element, view, pos, gap) {
        const coords = view.coordsAtPos(pos) || view.contentDOM.getBoundingClientRect();
        const rect = element.getBoundingClientRect();
        element.style.left = Math.max(8, Math.min(coords.left, window.innerWidth - rect.width - 8)) + 'px';
        element.style.top = Math.max(8, Math.min(coords.bottom + gap, window.innerHeight - rect.height - 8)) + 'px';
    }

    // --- Find references (Shift+F12) and the References tab of the bottom panel ---

    async function lspActFindReferences(view) {
        const target = state;
        const ctx = lspActContext(view);
        if (!ctx) return;
        const head = view.state.selection.main.head;
        const word = view.state.wordAt(head);
        if (!word) {
            flashStatus(tr('codeStudio.lspNoSymbol', 'There is no symbol at the cursor.'));
            return;
        }
        const symbol = view.state.doc.sliceString(word.from, word.to);
        const position = CodeStudioLSPProtocol.offsetToPosition(view.state.doc, head);
        const client = await lspActClient(target, ctx.language);
        if (!client) return;
        const request = Symbol('references');
        runWithInstance(target, () => {
            target.lspActReferences = { request, status: 'loading', symbol, groups: [], total: 0, outside: 0, message: '' };
            setActivePanel('references');
        });
        let result;
        try {
            result = await lspActRequest(target, ctx.language, 'textDocument/references', {
                textDocument: { uri: ctx.uri },
                position,
                context: { includeDeclaration: true }
            });
        } catch (err) {
            lspActUpdateReferences(target, request, {
                status: 'error',
                message: lspActIsTimeout(err)
                    ? tr('codeStudio.lspTimeout', 'The language server did not answer in time. Try again.')
                    : tr('codeStudio.lspRequestFailed', 'Language server error: {{message}}', { message: lspActErrorText(err) })
            });
            return;
        }
        if (!lspActReferencesCurrent(target, request)) return;
        const grouped = CodeStudioLSPEdits.referencesToGroups(result);
        const groups = grouped.groups.map(group => ({
            path: group.path,
            label: searchResultLabel(group.path),
            collapsed: false,
            items: group.items.map(item => Object.assign({ preview: null }, item))
        }));
        await lspActFillPreviews(target, groups);
        lspActUpdateReferences(target, request, { status: 'ready', groups, total: grouped.total, outside: grouped.outside });
    }

    function lspActReferencesCurrent(target, request) {
        return isLiveInstance(target) && !!target.lspActReferences && target.lspActReferences.request === request;
    }

    function lspActUpdateReferences(target, request, changes) {
        if (!lspActReferencesCurrent(target, request)) return;
        runWithInstance(target, () => {
            target.lspActReferences = Object.assign({}, target.lspActReferences, changes);
            renderReferencesPanel();
        });
    }

    // Line previews come from the open buffer of a file, otherwise from the file API, six files at a time and at most
    // LSP_ACT_PREVIEW_FILES files; the others keep their line numbers.
    async function lspActFillPreviews(target, groups) {
        const pending = [];
        groups.forEach(group => {
            const text = runWithInstance(target, () => {
                const tab = lspActOpenTab(group.path);
                return tab ? editorValue(tab) : null;
            });
            if (text !== null) lspActSetPreviews(group, text);
            else if (pending.length < LSP_ACT_PREVIEW_FILES) pending.push(group);
        });
        for (let index = 0; index < pending.length; index += 6) {
            await Promise.all(pending.slice(index, index + 6).map(async group => {
                try {
                    const result = await apiClient.file(group.path);
                    if (result && typeof result.content === 'string') lspActSetPreviews(group, result.content);
                } catch (_) {
                    // A file that cannot be read keeps its line numbers without a preview.
                }
            }));
            if (!isLiveInstance(target)) return;
        }
    }

    function lspActSetPreviews(group, text) {
        group.items.forEach(item => { item.preview = CodeStudioLSPEdits.referencePreview(text, item, 160); });
    }

    // The References tab joins the bottom panel when it is first needed, so the panel keeps its three standard views
    // until then. renderBottomPanel (panel.js) calls this before it marks the active view.
    function ensureReferencesPanelParts(panel) {
        if (!panel || (!state.lspActReferences && state.activePanel !== 'references')) return;
        if (panel.querySelector('[data-panel-tab="references"]')) return;
        const tabs = panel.querySelector('[data-panel-tabs]');
        const body = panel.querySelector('.cs-panel-body');
        if (!tabs || !body) return;
        const tab = document.createElement('button');
        tab.type = 'button';
        tab.className = 'cs-panel-tab';
        tab.setAttribute('role', 'tab');
        tab.setAttribute('aria-selected', 'false');
        tab.dataset.panelTab = 'references';
        tab.innerHTML = `<span>${esc(tr('codeStudio.panelReferences', 'References'))}</span>`;
        tab.addEventListener('click', bind(() => setActivePanel('references')));
        tabs.insertBefore(tab, tabs.querySelector('[data-panel-tab="terminal"]'));
        const pane = document.createElement('div');
        pane.className = 'cs-panel-pane';
        pane.dataset.panelPane = 'references';
        pane.setAttribute('data-references-panel', '');
        pane.hidden = true;
        body.insertBefore(pane, body.querySelector('[data-panel-pane="terminal"]'));
    }

    // Re-renders only when the result changed (renderBottomPanel runs on every problems update) or when forced.
    function renderReferencesPanel(force) {
        const pane = shellPart('[data-references-panel]');
        if (!pane) return;
        const data = state.lspActReferences || null;
        if (!force && pane.lspActShown === data) return;
        pane.lspActShown = data;
        const message = (text, extraClass) => `<div class="cs-empty${extraClass ? ' ' + extraClass : ''}">${esc(text)}</div>`;
        if (!data) {
            pane.innerHTML = message(tr('codeStudio.lspReferencesEmpty', 'Place the cursor on a symbol and press Shift+F12 to list its references.'));
            return;
        }
        if (data.status === 'loading') {
            pane.innerHTML = message(tr('codeStudio.lspReferencesLoading', 'Finding references to {{symbol}}...', { symbol: data.symbol }));
            return;
        }
        if (data.status === 'error') {
            pane.innerHTML = message(data.message, 'cs-references-error');
            return;
        }
        const outside = data.outside
            ? `<span class="cs-references-outside">${esc(tr('codeStudio.lspReferencesOutside', '{{count}} more results are outside the workspace and not listed.', { count: data.outside }))}</span>`
            : '';
        if (!data.total) {
            pane.innerHTML = message(tr('codeStudio.lspReferencesNone', 'No references found for {{symbol}}.', { symbol: data.symbol }))
                + (outside ? `<div class="cs-references-summary">${outside}</div>` : '');
            return;
        }
        const summary = `<div class="cs-references-summary"><span>${esc(tr('codeStudio.lspReferencesSummary', '{{count}} references to {{symbol}} in {{files}} files', { count: data.total, symbol: data.symbol, files: data.groups.length }))}</span>${outside}</div>`;
        const groups = data.groups.map((group, groupIndex) => {
            const rows = group.collapsed ? '' : group.items.map((item, index) => {
                const preview = item.preview;
                const code = preview ? `${esc(preview.before)}<mark>${esc(preview.match)}</mark>${esc(preview.after)}` : '';
                return `<button type="button" class="cs-reference" data-ref="${groupIndex}|${index}" title="${esc(group.label)}:${esc(item.line + 1)}"><em class="cs-search-line">${esc(item.line + 1)}</em><code>${code}</code></button>`;
            }).join('');
            return `<div class="cs-references-group${group.collapsed ? ' collapsed' : ''}">
                <button type="button" class="cs-references-file" data-ref-group="${groupIndex}" aria-expanded="${group.collapsed ? 'false' : 'true'}" title="${esc(group.path)}">
                    <span class="cs-problems-caret" aria-hidden="true">${group.collapsed ? '▸' : '▾'}</span>
                    <strong>${esc(group.label)}</strong>
                    <span class="cs-references-count">${esc(group.items.length)}</span>
                </button>${rows}
            </div>`;
        }).join('');
        pane.innerHTML = summary + groups;
        pane.querySelectorAll('[data-ref-group]').forEach(button => {
            button.addEventListener('click', bind(() => {
                const index = Number(button.dataset.refGroup);
                const group = data.groups[index];
                if (!group) return;
                group.collapsed = !group.collapsed;
                renderReferencesPanel(true);
                const again = pane.querySelector(`[data-ref-group="${index}"]`);
                if (again) again.focus();
            }));
        });
        pane.querySelectorAll('[data-ref]').forEach(button => {
            button.addEventListener('click', bind(() => {
                const [groupIndex, index] = button.dataset.ref.split('|').map(Number);
                const group = data.groups[groupIndex];
                const item = group && group.items[index];
                if (item) lspActOpenLocation(group.path, item.line, item.character);
            }));
        });
    }

    function clearReferences() {
        state.lspActReferences = null;
        renderReferencesPanel(true);
    }

    // Opens a reference: the file's tab comes forward (or opens) and the cursor lands on the reference.
    async function lspActOpenLocation(path, line, character) {
        const target = state;
        if (!isLiveInstance(target)) return;
        try {
            await runAsyncStep(target, () => openFile(path));
        } catch (err) {
            if (isLiveInstance(target)) runWithInstance(target, () => showOperationError(err));
            return;
        }
        if (!isLiveInstance(target)) return;
        runWithInstance(target, () => {
            const tab = activeTab();
            const view = tab && tab.path === normalizeCodeStudioPath(path) ? focusedEditorView(tab) : null;
            if (!view || !view.state || !view.state.doc || !state.cmModule || !state.cmModule.EditorView) return;
            const pos = lspActOffset(view.state.doc, { line, character });
            view.dispatch({ selection: { anchor: pos }, effects: state.cmModule.EditorView.scrollIntoView(pos, { y: 'center' }) });
            view.focus();
            renderStatus();
        });
    }

    // --- Rename symbol (F2 in the editor; F2 in the file tree still renames files) ---

    async function lspActRename(view) {
        const target = state;
        const ctx = lspActContext(view);
        if (!ctx) return;
        if (!canWrite(target)) {
            flashStatus(lspActReadOnlyText());
            return;
        }
        const client = await lspActClient(target, ctx.language);
        if (!client || !view.dom.isConnected) return;
        const capabilities = client.serverCapabilities || null;
        const provider = capabilities ? capabilities.renameProvider : true;
        if (!provider) {
            runWithInstance(target, () => flashStatus(tr('codeStudio.lspCannotRename', 'This symbol cannot be renamed.')));
            return;
        }
        const doc = view.state.doc;
        const head = view.state.selection.main.head;
        const position = CodeStudioLSPProtocol.offsetToPosition(doc, head);
        const word = view.state.wordAt(head);
        let range = word ? { from: word.from, to: word.to } : null;
        let current = word ? doc.sliceString(word.from, word.to) : '';
        if (typeof provider === 'object' && provider.prepareProvider) {
            let prepared;
            try {
                prepared = await lspActRequest(target, ctx.language, 'textDocument/prepareRename', { textDocument: { uri: ctx.uri }, position });
            } catch (err) {
                if (isLiveInstance(target)) runWithInstance(target, () => lspActReportError(err));
                return;
            }
            if (!isLiveInstance(target) || view.state.doc !== doc) return;
            if (!prepared) {
                runWithInstance(target, () => flashStatus(tr('codeStudio.lspCannotRename', 'This symbol cannot be renamed.')));
                return;
            }
            // prepareRename answers a Range, { range, placeholder } or { defaultBehavior: true } (keep the word).
            const preparedRange = prepared.range && prepared.range.start ? prepared.range : (prepared.start && prepared.end ? prepared : null);
            if (preparedRange) {
                range = { from: lspActOffset(doc, preparedRange.start), to: lspActOffset(doc, preparedRange.end) };
                current = typeof prepared.placeholder === 'string' ? prepared.placeholder : doc.sliceString(range.from, range.to);
            }
        }
        if (!range || !current) {
            runWithInstance(target, () => flashStatus(tr('codeStudio.lspNoSymbol', 'There is no symbol at the cursor.')));
            return;
        }
        const newName = await runWithInstance(target, () => lspActRenameInput(target, view, range.from, current));
        if (!newName || newName === current || !isLiveInstance(target) || !view.dom.isConnected || view.state.doc !== doc) return;
        const docsBefore = runWithInstance(target, lspActCaptureDocs);
        runWithInstance(target, () => flashStatus(tr('codeStudio.lspWorking', 'Waiting for the language server...'), LSP_ACT_TIMEOUT_MS));
        let edit;
        try {
            edit = await lspActRequest(target, ctx.language, 'textDocument/rename', { textDocument: { uri: ctx.uri }, position, newName });
        } catch (err) {
            if (isLiveInstance(target)) runWithInstance(target, () => lspActReportError(err));
            return;
        }
        if (!isLiveInstance(target)) return;
        lspActClearStatus(target);
        if (!edit) {
            runWithInstance(target, () => flashStatus(tr('codeStudio.lspNoChanges', 'Nothing to change.')));
            return;
        }
        await runWithInstance(target, () => lspActApplyWorkspaceEdit(target, edit, {
            mode: 'rename',
            title: tr('codeStudio.lspPreviewRename', 'Rename {{from}} to {{to}}', { from: current, to: newName }),
            originPath: ctx.path,
            docsBefore,
            focusView: view
        }));
    }

    // The new name goes into a small box at the symbol: Enter renames, Escape or a click elsewhere cancels.
    // Resolves to the trimmed name or null.
    function lspActRenameInput(target, view, pos, value) {
        return new Promise(resolve => {
            lspActCloseMenus();
            const label = tr('codeStudio.lspRenameLabel', 'New name');
            const box = document.createElement('div');
            box.className = 'cs-context-menu cs-lsp-rename-box';
            box.setAttribute('role', 'dialog');
            box.setAttribute('aria-label', label);
            box.innerHTML = `<input type="text" value="${esc(value)}" aria-label="${esc(label)}" title="${esc(label)}" autocomplete="off" spellcheck="false" autocapitalize="off" enterkeyhint="done">`;
            document.body.appendChild(box);
            lspActPlace(box, view, pos, 4);
            const input = box.querySelector('input');
            let done = false;
            let unregister = () => {};
            const finish = result => {
                if (done) return;
                done = true;
                unregister();
                box.remove();
                if (view.dom.isConnected) view.focus();
                resolve(result);
            };
            box.__codeStudioCleanup = () => finish(null);
            runWithInstance(target, () => { unregister = registerDisposer(() => finish(null)); });
            input.addEventListener('keydown', event => {
                if (event.isComposing) return;
                if (event.key === 'Enter') {
                    event.preventDefault();
                    event.stopPropagation();
                    finish(input.value.trim() || null);
                } else if (event.key === 'Escape') {
                    event.preventDefault();
                    event.stopPropagation();
                    finish(null);
                }
            });
            input.addEventListener('blur', () => setTimeout(() => finish(null), 0));
            input.focus();
            input.select();
        });
    }

    // --- Quick fixes (Ctrl+. and the lightbulb): the server's code actions for the line ---

    // LSP diagnostics of one line for the codeAction context. A problem that kept the server's own diagnostic
    // (problem.lsp, see the contract notes) sends it unchanged; otherwise the diagnostic is rebuilt from the problem.
    function lspActLineDiagnostics(path, doc, lineNumber) {
        return problemsForFile(path)
            .filter(problem => LSP_ACT_SOURCES.has(problem.source))
            .filter(problem => {
                const range = problem.lsp && problem.lsp.range;
                const first = range && range.start ? range.start.line + 1 : problem.line;
                const last = range && range.end ? range.end.line + 1 : first;
                return first <= lineNumber && lineNumber <= last;
            })
            .map(problem => lspActProblemDiagnostic(problem, doc));
    }

    function lspActProblemDiagnostic(problem, doc) {
        const raw = problem.lsp;
        if (raw && raw.range && raw.range.start && raw.range.end && typeof raw.message === 'string') return raw;
        const line = Math.max(0, (Number(problem.line) || 1) - 1);
        const text = line < doc.lines ? doc.line(line + 1).text : '';
        const character = Math.min(Math.max(0, (Number(problem.column) || 1) - 1), text.length);
        const word = /^[\p{L}\p{N}_$]+/u.exec(text.slice(character));
        const end = { line, character: Math.min(text.length, character + (word ? word[0].length : 1)) };
        const diagnostic = { range: { start: { line, character }, end }, severity: LSP_ACT_SEVERITY[problem.severity] || 1, message: String(problem.message || '') };
        if (problem.code !== undefined && problem.code !== null && problem.code !== '') diagnostic.code = problem.code;
        return diagnostic;
    }

    // Commands and code actions in one list: preferred actions first, then quick fixes, other actions, refactorings and
    // source actions, each group in the server's order.
    function lspActNormalizeActions(result) {
        const rank = kind => {
            if (kind === 'quickfix' || kind.startsWith('quickfix.')) return 0;
            if (kind.startsWith('refactor')) return 2;
            if (kind.startsWith('source')) return 3;
            return 1;
        };
        return (Array.isArray(result) ? result : [])
            .filter(item => item && typeof item.title === 'string' && item.title.trim())
            .map((item, index) => {
                const command = typeof item.command === 'string';
                return {
                    raw: item,
                    command,
                    title: item.title,
                    kind: command ? '' : String(item.kind || ''),
                    preferred: !command && item.isPreferred === true,
                    disabled: !command && item.disabled ? String(item.disabled.reason || item.title) : '',
                    index
                };
            })
            .sort((a, b) => (Number(b.preferred) - Number(a.preferred)) || (rank(a.kind) - rank(b.kind)) || (a.index - b.index));
    }

    // wholeLine: the lightbulb asks for the whole line; Ctrl+. asks for the selection.
    async function lspActQuickFixes(view, pos, wholeLine) {
        const target = state;
        const ctx = lspActContext(view);
        if (!ctx) return;
        if (!canWrite(target)) {
            flashStatus(lspActReadOnlyText());
            return;
        }
        const client = await lspActClient(target, ctx.language);
        if (!client || !view.dom.isConnected) return;
        const doc = view.state.doc;
        const line = doc.lineAt(Math.min(Math.max(0, pos), doc.length));
        const selection = view.state.selection.main;
        const from = wholeLine ? line.from : selection.from;
        const to = wholeLine ? line.to : selection.to;
        const range = { start: CodeStudioLSPProtocol.offsetToPosition(doc, from), end: CodeStudioLSPProtocol.offsetToPosition(doc, to) };
        const diagnostics = runWithInstance(target, () => lspActLineDiagnostics(ctx.path, doc, line.number));
        const docsBefore = runWithInstance(target, lspActCaptureDocs);
        runWithInstance(target, () => flashStatus(tr('codeStudio.lspWorking', 'Waiting for the language server...'), LSP_ACT_TIMEOUT_MS));
        let result;
        try {
            result = await lspActRequest(target, ctx.language, 'textDocument/codeAction', {
                textDocument: { uri: ctx.uri },
                range,
                context: { diagnostics, triggerKind: 1 }
            });
        } catch (err) {
            if (isLiveInstance(target)) runWithInstance(target, () => lspActReportError(err));
            return;
        }
        if (!isLiveInstance(target)) return;
        lspActClearStatus(target);
        const actions = lspActNormalizeActions(result);
        if (!actions.length) {
            runWithInstance(target, () => flashStatus(tr('codeStudio.lspQuickFixNone', 'No quick fixes available here.')));
            return;
        }
        if (!view.dom.isConnected) return;
        const run = Object.assign({}, ctx, { client, view, docsBefore });
        runWithInstance(target, () => lspActShowFixMenu(target, view, wholeLine ? line.from : selection.head, actions, action => {
            let pending;
            try {
                pending = runWithInstance(target, () => lspActRunCodeAction(target, run, action));
            } catch (err) {
                pending = Promise.reject(err);
            }
            Promise.resolve(pending).catch(err => {
                if (isLiveInstance(target)) runWithInstance(target, () => lspActReportError(err));
            });
        }));
    }

    // The quick-fix menu at the cursor: arrow keys, Home and End move, Enter runs, Escape, Tab or a click elsewhere closes.
    function lspActShowFixMenu(target, view, pos, actions, onPick) {
        lspActCloseMenus();
        const menu = document.createElement('div');
        menu.className = 'cs-context-menu cs-lsp-fix-menu';
        menu.setAttribute('role', 'menu');
        menu.setAttribute('aria-label', tr('codeStudio.lspQuickFix', 'Quick Fix...'));
        menu.innerHTML = actions.map((action, index) => {
            const kind = action.kind && action.kind !== 'quickfix' ? `<kbd>${esc(action.kind)}</kbd>` : '';
            return `<button type="button" role="menuitem" tabindex="-1" data-fix-index="${index}"${action.disabled ? ' disabled' : ''} title="${esc(action.disabled || action.title)}"><span>${esc(action.title)}</span>${kind}</button>`;
        }).join('');
        document.body.appendChild(menu);
        lspActPlace(menu, view, pos, 2);
        const items = Array.from(menu.querySelectorAll('button:not(:disabled)'));
        let closed = false;
        let unregister = () => {};
        const onOutside = event => {
            if (!menu.contains(event.target)) close(false);
        };
        function close(refocus) {
            if (closed) return;
            closed = true;
            unregister();
            document.removeEventListener('mousedown', onOutside, true);
            menu.remove();
            if (refocus && view.dom.isConnected) view.focus();
        }
        const move = step => {
            if (!items.length) return;
            const index = items.indexOf(document.activeElement);
            const next = index < 0 ? (step > 0 ? 0 : items.length - 1) : (index + step + items.length) % items.length;
            items[next].focus();
        };
        menu.__codeStudioCleanup = () => close(false);
        runWithInstance(target, () => { unregister = registerDisposer(() => close(false)); });
        menu.addEventListener('keydown', event => {
            if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
                event.preventDefault();
                move(event.key === 'ArrowDown' ? 1 : -1);
            } else if (event.key === 'Home' || event.key === 'End') {
                event.preventDefault();
                if (items.length) items[event.key === 'Home' ? 0 : items.length - 1].focus();
            } else if (event.key === 'Escape' || event.key === 'Tab') {
                event.preventDefault();
                event.stopPropagation();
                close(true);
            }
        });
        menu.querySelectorAll('[data-fix-index]').forEach(button => {
            button.addEventListener('click', () => {
                const action = actions[Number(button.dataset.fixIndex)];
                close(true);
                if (action && !action.disabled) onPick(action);
            });
        });
        setTimeout(() => {
            if (!closed) document.addEventListener('mousedown', onOutside, true);
        }, 0);
        if (items.length) {
            items[0].focus();
        } else {
            menu.tabIndex = -1;
            menu.focus();
        }
    }

    // A code action applies its edit first and then runs its command, as the protocol orders it. A plain command goes
    // straight to workspace/executeCommand; the server answers with workspace/applyEdit when it changes files.
    async function lspActRunCodeAction(target, ctx, action) {
        if (!isLiveInstance(target)) return;
        if (action.command) {
            await lspActExecuteCommand(target, ctx.language, action.raw);
            return;
        }
        let item = action.raw;
        const provider = ctx.client && ctx.client.serverCapabilities ? ctx.client.serverCapabilities.codeActionProvider : null;
        if (!item.edit && !item.command && item.data !== undefined && provider && typeof provider === 'object' && provider.resolveProvider) {
            try {
                item = (await lspActRequest(target, ctx.language, 'codeAction/resolve', item)) || item;
            } catch (err) {
                if (isLiveInstance(target)) runWithInstance(target, () => lspActReportError(err));
                return;
            }
            if (!isLiveInstance(target)) return;
        }
        const command = item.command && typeof item.command.command === 'string' ? item.command : null;
        if (!item.edit && !command) {
            runWithInstance(target, () => flashStatus(tr('codeStudio.lspNoChanges', 'Nothing to change.')));
            return;
        }
        if (item.edit) {
            const outcome = await runWithInstance(target, () => lspActApplyWorkspaceEdit(target, item.edit, {
                mode: 'fix',
                title: item.title,
                originPath: ctx.path,
                docsBefore: ctx.docsBefore,
                focusView: ctx.view
            }));
            if (!outcome.applied) return;
        }
        if (command) await lspActExecuteCommand(target, ctx.language, command);
    }

    async function lspActExecuteCommand(target, language, command) {
        const before = target.lspActServerEdits || 0;
        const params = { command: command.command };
        if (Array.isArray(command.arguments)) params.arguments = command.arguments;
        try {
            await lspActRequest(target, language, 'workspace/executeCommand', params);
        } catch (err) {
            // While the server waits for a previewed workspace/applyEdit the command itself may time out; that edit
            // reports its own result.
            if (!isLiveInstance(target) || (lspActIsTimeout(err) && (target.lspActServerEdits || 0) !== before)) return;
            runWithInstance(target, () => lspActReportError(err));
        }
    }

    // --- Lightbulb gutter and the editor hook ---

    let lspActBulbType = null;
    let lspActBulbBase = null;

    // GutterMarker comes from the dynamically imported CodeMirror bundle, so the subclass is built on first use.
    function lspActBulbClass(cm) {
        if (lspActBulbType && lspActBulbBase === cm.GutterMarker) return lspActBulbType;
        lspActBulbBase = cm.GutterMarker;
        lspActBulbType = class LspActBulbMarker extends cm.GutterMarker {
            eq(other) {
                return other instanceof LspActBulbMarker;
            }

            toDOM() {
                const element = document.createElement('span');
                element.className = 'cs-lsp-bulb';
                element.title = tr('codeStudio.lspQuickFix', 'Quick Fix...') + ' (Ctrl+.)';
                element.innerHTML = LSP_ACT_BULB_SVG;
                return element;
            }
        };
        return lspActBulbType;
    }

    // Line numbers with language server problems in the file a view shows, cached per problem list. A view that is not
    // attached to its tab yet (the gutter runs while the editor is built) asks for a few more updates.
    function lspActBulbLines(view, cache) {
        if (!cache.instance || !isLiveInstance(cache.instance)) cache.instance = lspActInstanceForView(view);
        if (!cache.instance) {
            if (!cache.retry && cache.retries < 3) {
                cache.retry = true;
                cache.retries += 1;
                setTimeout(() => {
                    cache.retry = false;
                    if (view.dom.isConnected) {
                        try { view.dispatch({}); } catch (_) {}
                    }
                }, 0);
            }
            return null;
        }
        return runWithInstance(cache.instance, () => {
            const tab = state.openTabs.find(item => isSourceTab(item) && lspActTabShows(item, view));
            if (!tab) return null;
            if (cache.problems !== state.problems || cache.path !== tab.path) {
                cache.problems = state.problems;
                cache.path = tab.path;
                cache.lines = new Set(problemsForFile(tab.path).filter(problem => LSP_ACT_SOURCES.has(problem.source)).map(problem => problem.line));
            }
            return cache.lines;
        });
    }

    function lspActBulbGutter(cm) {
        if (typeof cm.gutter !== 'function' || typeof cm.GutterMarker !== 'function') return [];
        const Marker = lspActBulbClass(cm);
        const marker = new Marker();
        const cache = { instance: null, problems: null, path: '', lines: null, retry: false, retries: 0 };
        const hasBulb = (view, from) => {
            const lines = lspActBulbLines(view, cache);
            return !!lines && lines.has(view.state.doc.lineAt(from).number);
        };
        return [cm.gutter({
            class: 'cs-lsp-bulb-gutter',
            lineMarker: (view, line) => (hasBulb(view, line.from) ? marker : null),
            lineMarkerChange: update => update.docChanged || update.transactions.length > 0,
            initialSpacer: () => marker,
            domEventHandlers: {
                mousedown: (view, line, event) => {
                    if (!hasBulb(view, line.from)) return false;
                    event.preventDefault();
                    return lspActStart(view, editor => lspActQuickFixes(editor, line.from, true));
                }
            }
        })];
    }

    // lsp.js appends these extensions to every editor whose file has a language server (contract: lspActionExtensionFactories).
    function lspActEditorExtensions(cm) {
        if (!cm || !cm.keymap) return [];
        const keys = cm.keymap.of([
            { key: 'Shift-F12', preventDefault: true, run: view => lspActStart(view, lspActFindReferences) },
            { key: 'F2', preventDefault: true, run: view => lspActStart(view, lspActRename) },
            { key: 'Mod-.', preventDefault: true, run: view => lspActStart(view, editor => lspActQuickFixes(editor, editor.state.selection.main.head, false)) }
        ]);
        return [cm.Prec && typeof cm.Prec.high === 'function' ? cm.Prec.high(keys) : keys, lspActBulbGutter(cm)];
    }

    lspActionExtensionFactories.push(lspActEditorExtensions);

    // --- Command palette entries (studioCommandTable in shortcuts.js) ---

    function lspActPaletteAction(action) {
        const tab = activeTab();
        const view = isSourceTab(tab) ? focusedEditorView(tab) : null;
        if (!view || !lspActContext(view)) {
            flashStatus(tr('codeStudio.lspNotReady', 'No language server is ready for this file.'));
            return;
        }
        view.focus();
        lspActStart(view, action);
    }

    function lspFindReferencesCommand() {
        lspActPaletteAction(lspActFindReferences);
    }

    function lspRenameSymbolCommand() {
        lspActPaletteAction(lspActRename);
    }

    function lspQuickFixCommand() {
        lspActPaletteAction(editor => lspActQuickFixes(editor, editor.state.selection.main.head, false));
    }
