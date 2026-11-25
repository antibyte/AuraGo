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
