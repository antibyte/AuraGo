    // Find and replace for the active editor. The widget floats over the editor like the one
    // in Visual Studio, works for both the CodeMirror view and the plain textarea fallback and
    // keeps the match counter in sync while the user types.

    function findTarget() {
        const tab = activeTab();
        if (!tab) return null;
        const view = tab.focusedView || tab.view;
        if (!view) return null;
        if (view.state && view.state.doc && state.cmModule && state.cmModule.EditorView) {
            const cm = state.cmModule;
            return {
                kind: 'codemirror',
                view,
                text: () => view.state.doc.toString(),
                // The editor must not grab the focus here: the find box keeps it while
                // the user types, exactly like in Visual Studio.
                reveal: pos => {
                    view.dispatch({
                        selection: { anchor: pos },
                        effects: cm.EditorView.scrollIntoView(pos, { y: 'center' })
                    });
                },
                select: (from, to) => {
                    view.dispatch({ selection: { anchor: from, head: to } });
                },
                replaceRange: (from, to, insert) => {
                    view.dispatch({ changes: { from, to, insert } });
                },
                replaceRanges: (ranges, insert) => {
                    view.dispatch({ changes: ranges.map(range => ({ from: range.from, to: range.to, insert })) });
                }
            };
        }
        if (view.textarea) {
            const textarea = view.textarea;
            const notify = () => {
                try { textarea.dispatchEvent(new Event('input', { bubbles: true })); } catch (_) {}
            };
            return {
                kind: 'textarea',
                view,
                text: () => textarea.value,
                reveal: pos => {
                    textarea.setSelectionRange(pos, pos);
                },
                select: (from, to) => {
                    textarea.setSelectionRange(from, to);
                },
                replaceRange: (from, to, insert) => {
                    textarea.value = textarea.value.slice(0, from) + insert + textarea.value.slice(to);
                    notify();
                },
                replaceRanges: (ranges, insert) => {
                    let out = '';
                    let cursor = 0;
                    ranges.forEach(range => {
                        out += textarea.value.slice(cursor, range.from) + insert;
                        cursor = range.to;
                    });
                    out += textarea.value.slice(cursor);
                    textarea.value = out;
                    notify();
                }
            };
        }
        return null;
    }

    function escapeRegExp(value) {
        return String(value).replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    }

    function findMatches(text, query, options) {
        const needle = String(query || '');
        if (!needle) return [];
        const flags = options.caseSensitive ? 'g' : 'gi';
        let pattern;
        try {
            if (options.regex) pattern = new RegExp(needle, flags);
            else if (options.wholeWord) pattern = new RegExp(`\\b${escapeRegExp(needle)}\\b`, flags);
            else pattern = new RegExp(escapeRegExp(needle), flags);
        } catch (_) {
            return [];
        }
        const matches = [];
        let match;
        while ((match = pattern.exec(text)) !== null) {
            if (match[0] === '') { pattern.lastIndex += 1; continue; }
            matches.push({ from: match.index, to: match.index + match[0].length });
            if (matches.length >= 5000) break;
        }
        return matches;
    }

    function findOptions() {
        return {
            caseSensitive: !!state.findCaseSensitive,
            wholeWord: !!state.findWholeWord,
            regex: !!state.findRegex
        };
    }

    function refreshFindMatches(preferDirection) {
        const target = findTarget();
        if (!target) {
            state.findMatches = [];
            state.findIndex = -1;
            renderFindWidget();
            return;
        }
        const text = target.text();
        const matches = findMatches(text, state.findQuery, findOptions());
        state.findMatches = matches;
        if (!matches.length) {
            state.findIndex = -1;
        } else if (preferDirection === 'next') {
            const cursor = findCursorOffset(target);
            const found = matches.findIndex(match => match.from > cursor);
            state.findIndex = found === -1 ? 0 : found;
        } else if (preferDirection === 'previous') {
            const cursor = findCursorOffset(target);
            let found = -1;
            for (let i = matches.length - 1; i >= 0; i -= 1) {
                if (matches[i].from < cursor) { found = i; break; }
            }
            state.findIndex = found === -1 ? matches.length - 1 : found;
        } else {
            state.findIndex = Math.min(Math.max(0, state.findIndex), matches.length - 1);
        }
        renderFindWidget();
        if (matches.length && state.findIndex >= 0) {
            const match = matches[state.findIndex];
            target.select(match.from, match.to);
            target.reveal(match.from);
        }
    }

    function findCursorOffset(target) {
        try {
            if (target.kind === 'codemirror') return target.view.state.selection.main.head;
            return target.view.textarea.selectionStart;
        } catch (_) {
            return 0;
        }
    }

    function findMatchLabel() {
        const total = (state.findMatches || []).length;
        if (!state.findQuery) return tr('codeStudio.findNoQuery', 'No results');
        if (!total) return tr('codeStudio.findNoResults', 'No results');
        return tr('codeStudio.findCounter', '{{current}} of {{total}}', {
            current: (state.findIndex || 0) + 1,
            total
        });
    }

    function renderFindWidget() {
        const widget = shellPart('[data-find-widget]');
        if (!widget) return;
        widget.hidden = !state.findOpen;
        if (!state.findOpen) return;
        // The widget floats over the editor, not over the tab bar.
        const editor = shellPart('[data-editor]');
        const main = shellPart('.code-studio-main');
        if (editor && main) {
            const delta = editor.getBoundingClientRect().top - main.getBoundingClientRect().top;
            widget.style.top = Math.max(0, delta + 8) + 'px';
        }
        const options = findOptions();
        if (!widget.querySelector('[data-find-input]')) {
            widget.innerHTML = `
            <div class="cs-find-row">
                <button type="button" class="cs-find-toggle" data-find-option="case" title="${esc(tr('codeStudio.findCase', 'Match case'))}">Aa</button>
                <button type="button" class="cs-find-toggle" data-find-option="whole" title="${esc(tr('codeStudio.findWholeWord', 'Match whole word'))}">ab</button>
                <button type="button" class="cs-find-toggle" data-find-option="regex" title="${esc(tr('codeStudio.findRegex', 'Use regular expression'))}">.*</button>
                <input type="text" class="cs-find-input" data-find-input value="${esc(state.findQuery || '')}" placeholder="${esc(tr('codeStudio.findPlaceholder', 'Find'))}" spellcheck="false" autocomplete="off">
                <span class="cs-find-count" data-find-count>${esc(findMatchLabel())}</span>
                <button type="button" class="cs-find-btn" data-find-prev title="${esc(tr('codeStudio.findPrevious', 'Previous match'))}">↑</button>
                <button type="button" class="cs-find-btn" data-find-next title="${esc(tr('codeStudio.findNext', 'Next match'))}">↓</button>
                <button type="button" class="cs-find-btn" data-find-replace-toggle title="${esc(tr('codeStudio.findToggleReplace', 'Toggle replace'))}">⇄</button>
                <button type="button" class="cs-find-btn" data-find-close title="${esc(tr('desktop.close', 'Close'))}">×</button>
            </div>
            <div class="cs-find-replace-row" data-find-replace-row${state.findReplaceOpen ? '' : ' hidden'}>
                <input type="text" class="cs-find-input" data-find-replace value="${esc(state.findReplaceText || '')}" placeholder="${esc(tr('codeStudio.replacePlaceholder', 'Replace'))}" spellcheck="false" autocomplete="off">
                <button type="button" class="cs-find-btn labelled" data-find-replace-one title="${esc(tr('codeStudio.replaceOne', 'Replace'))}">${esc(tr('codeStudio.replaceOne', 'Replace'))}</button>
                <button type="button" class="cs-find-btn labelled" data-find-replace-all title="${esc(tr('codeStudio.replaceAll', 'Replace All'))}">${esc(tr('codeStudio.replaceAll', 'Replace All'))}</button>
            </div>`;
            wireFindWidget(widget);
        }
        // The widget is only patched from here on so typing never loses the caret.
        const counter = widget.querySelector('[data-find-count]');
        if (counter) counter.textContent = findMatchLabel();
        const queryInput = widget.querySelector('[data-find-input]');
        if (queryInput && queryInput !== document.activeElement && queryInput.value !== (state.findQuery || '')) {
            queryInput.value = state.findQuery || '';
        }
        const replaceField = widget.querySelector('[data-find-replace]');
        if (replaceField && replaceField !== document.activeElement && replaceField.value !== (state.findReplaceText || '')) {
            replaceField.value = state.findReplaceText || '';
        }
        widget.querySelectorAll('[data-find-option]').forEach(btn => {
            const key = btn.dataset.findOption;
            const active = key === 'case' ? options.caseSensitive : (key === 'whole' ? options.wholeWord : options.regex);
            btn.classList.toggle('active', !!active);
        });
        const replaceRow = widget.querySelector('[data-find-replace-row]');
        if (replaceRow) replaceRow.hidden = !state.findReplaceOpen;
    }

    function wireFindWidget(widget) {
        const input = widget.querySelector('[data-find-input]');
        const replaceInput = widget.querySelector('[data-find-replace]');
        if (input) {
            input.addEventListener('input', bind(event => {
                state.findQuery = event.target.value;
                refreshFindMatches('next');
            }));
            input.addEventListener('keydown', bind(event => {
                if (event.key === 'Enter') {
                    event.preventDefault();
                    refreshFindMatches(event.shiftKey ? 'previous' : 'next');
                } else if (event.key === 'Escape') {
                    event.preventDefault();
                    closeFindWidget();
                }
            }));
        }
        if (replaceInput) {
            replaceInput.addEventListener('input', bind(event => { state.findReplaceText = event.target.value; }));
            replaceInput.addEventListener('keydown', bind(event => {
                if (event.key === 'Enter') {
                    event.preventDefault();
                    replaceCurrentMatch();
                } else if (event.key === 'Escape') {
                    event.preventDefault();
                    closeFindWidget();
                }
            }));
        }
        widget.querySelectorAll('[data-find-option]').forEach(btn => {
            btn.addEventListener('click', bind(() => {
                const key = btn.dataset.findOption;
                if (key === 'case') state.findCaseSensitive = !state.findCaseSensitive;
                if (key === 'whole') state.findWholeWord = !state.findWholeWord;
                if (key === 'regex') state.findRegex = !state.findRegex;
                refreshFindMatches('next');
            }));
        });
        const prev = widget.querySelector('[data-find-prev]');
        if (prev) prev.addEventListener('click', bind(() => refreshFindMatches('previous')));
        const next = widget.querySelector('[data-find-next]');
        if (next) next.addEventListener('click', bind(() => refreshFindMatches('next')));
        const close = widget.querySelector('[data-find-close]');
        if (close) close.addEventListener('click', bind(() => closeFindWidget()));
        const toggleReplace = widget.querySelector('[data-find-replace-toggle]');
        if (toggleReplace) toggleReplace.addEventListener('click', bind(() => {
            state.findReplaceOpen = !state.findReplaceOpen;
            renderFindWidget();
            const field = shellPart(state.findReplaceOpen ? '[data-find-replace]' : '[data-find-input]');
            if (field) field.focus();
        }));
        const replaceOne = widget.querySelector('[data-find-replace-one]');
        if (replaceOne) replaceOne.addEventListener('click', bind(() => replaceCurrentMatch()));
        const replaceAll = widget.querySelector('[data-find-replace-all]');
        if (replaceAll) replaceAll.addEventListener('click', bind(() => replaceAllMatches()));
    }

    function openFindWidget(showReplace) {
        state.findOpen = true;
        if (showReplace) state.findReplaceOpen = true;
        const tab = activeTab();
        const target = findTarget();
        if (target && tab) {
            const selection = target.kind === 'codemirror'
                ? target.view.state.selection.main
                : { from: target.view.textarea.selectionStart, to: target.view.textarea.selectionEnd };
            const text = target.text();
            if (!state.findQuery && selection && selection.from !== selection.to) {
                state.findQuery = text.slice(selection.from, selection.to);
            }
        }
        renderFindWidget();
        refreshFindMatches('next');
        const input = shellPart('[data-find-input]');
        if (input) { input.focus(); input.select(); }
    }

    function closeFindWidget() {
        state.findOpen = false;
        state.findMatches = [];
        state.findIndex = -1;
        renderFindWidget();
        // The editor takes the focus back, like in Visual Studio.
        const tab = activeTab();
        const view = tab && (tab.focusedView || tab.view);
        if (!view) return;
        if (view.textarea && typeof view.textarea.focus === 'function') view.textarea.focus();
        else if (typeof view.focus === 'function') view.focus();
    }

    function replaceCurrentMatch() {
        const target = findTarget();
        const matches = state.findMatches || [];
        if (!target || !matches.length) return;
        const index = Math.min(Math.max(0, state.findIndex), matches.length - 1);
        const match = matches[index];
        const insert = state.findReplaceText || '';
        target.replaceRange(match.from, match.to, insert);
        state.findIndex = index;
        refreshFindMatches('next');
    }

    function replaceAllMatches() {
        const target = findTarget();
        if (!target) return;
        // The document may have changed since the last search, so the ranges are
        // recalculated before anything is rewritten.
        const matches = findMatches(target.text(), state.findQuery, findOptions());
        if (!matches.length) {
            refreshFindMatches('next');
            return;
        }
        const insert = state.findReplaceText || '';
        target.replaceRanges(matches, insert);
        state.findMatches = matches;
        refreshFindMatches('next');
        flashStatus(tr('codeStudio.replaceDone', '{{count}} replacements', { count: matches.length }));
    }
