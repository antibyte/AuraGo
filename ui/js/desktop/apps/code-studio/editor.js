    function renderEditor() {
        state.diffRequest = null;
        const editor = shellPart('[data-editor]');
        if (!editor) return;
        const tab = activeTab();
        editor.classList.remove('code-studio-split', 'split-right', 'split-down');
        editor.style.gridTemplateColumns = '';
        editor.style.gridTemplateRows = '';
        if (!tab) {
            state.openTabs.forEach(destroyTabView);
            editor.innerHTML = `<div class="cs-editor-empty">
                <div class="cs-empty-icon">{ }</div>
                <div class="cs-empty-title">${esc(tr('codeStudio.welcome', 'Welcome to Code Studio'))}</div>
                <div class="cs-empty-hint">${esc(tr('codeStudio.welcomeHint', 'Open a file from the sidebar or press Ctrl+Shift+P to open the Command Palette'))}</div>
                <div class="cs-empty-keys">
                    <span><kbd>Ctrl+P</kbd> ${esc(tr('codeStudio.quickOpen', 'Quick open'))}</span>
                    <span><kbd>Ctrl+N</kbd> ${esc(tr('codeStudio.newFile', 'New File'))}</span>
                    <span><kbd>?</kbd> ${esc(tr('codeStudio.keyboardShortcuts', 'Keyboard Shortcuts'))}</span>
                </div>
            </div>`;
            editor.oncontextmenu = null;
            editor.onwheel = null;
            highlightActiveTreeRow();
            return;
        }
        state.openTabs.forEach(openTab => {
            if (openTab !== tab) destroyTabView(openTab);
        });
        destroyTabView(tab);
        editor.innerHTML = '';
        if (state.splitMode) {
            renderSplitPanes(editor, tab);
        } else {
            tab.view = createEditorView(editor, tab, null);
            tab.views = [tab.view];
        }
        editor.oncontextmenu = bind(event => {
            event.preventDefault();
            showCodeActionMenu(event.clientX, event.clientY);
        });
        editor.onwheel = bind(event => {
            if (!event.ctrlKey && !event.metaKey) return;
            event.preventDefault();
            adjustEditorZoom(event.deltaY < 0 ? 1 : -1);
        });
        highlightActiveTreeRow();
    }

    function createEditorView(container, tab, link) {
        return state.editorType === 'codemirror'
            ? createCodeMirrorEditor(container, tab, link)
            : createTextareaEditor(container, tab, link);
    }

    function usesLightEditorTheme() {
        const body = document.body;
        if (!body) return false;
        if (body.dataset.theme === 'fruity') return body.dataset.fruityMode !== 'dark';
        return body.dataset.theme === 'light';
    }

    function codeMirrorHighlightExtensions(cm) {
        if (usesLightEditorTheme()) {
            return cm.syntaxHighlighting && cm.defaultHighlightStyle
                ? [cm.syntaxHighlighting(cm.defaultHighlightStyle)]
                : [];
        }
        return cm.oneDark ? [cm.oneDark] : [];
    }

    function syncLinkedView(link, update) {
        if (!link || !update.docChanged || link.syncing) return;
        const other = (link.views || []).find(view => view && view !== update.view);
        if (!other || !other.state) return;
        link.syncing = true;
        try {
            update.transactions.forEach(transaction => {
                if (transaction.docChanged) other.dispatch({ changes: transaction.changes });
            });
        } finally {
            link.syncing = false;
        }
    }

    function createCodeMirrorEditor(container, tab, link) {
        const cm = state.cmModule;
        if (!cm || !cm.EditorState || !cm.EditorView) return createTextareaEditor(container, tab, link);
        const light = usesLightEditorTheme();
        const extensions = [
            cm.lineNumbers && cm.lineNumbers(),
            cm.highlightActiveLineGutter && cm.highlightActiveLineGutter(),
            cm.highlightSpecialChars && cm.highlightSpecialChars(),
            cm.history && cm.history(),
            cm.drawSelection && cm.drawSelection(),
            cm.dropCursor && cm.dropCursor(),
            cm.highlightActiveLine && cm.highlightActiveLine(),
            cm.EditorState.allowMultipleSelections && cm.EditorState.allowMultipleSelections.of(true),
            cm.indentUnit && cm.indentUnit.of('    '),
            cm.EditorView.lineWrapping,
            ...codeMirrorHighlightExtensions(cm),
            cm.closeBrackets && cm.closeBrackets(),
            cm.autocompletion && cm.autocompletion(),
            cm.rectangularSelection && cm.rectangularSelection(),
            cm.crosshairCursor && cm.crosshairCursor(),
            cm.highlightSelectionMatches && cm.highlightSelectionMatches(),
            languageExtension(cm, tab.language),
            cm.keymap && cm.keymap.of([
                cm.indentWithTab,
                ...(cm.closeBracketsKeymap || []),
                ...(cm.defaultKeymap || []),
                ...(cm.searchKeymap || []),
                ...(cm.historyKeymap || []),
                ...(cm.completionKeymap || []),
                ...(cm.lintKeymap || []),
                { key: 'Ctrl-s', run: bind(() => { saveCurrentFile(); return true; }) },
                { key: 'F5', run: bind(() => { runCurrentFile(); return true; }) }
            ].filter(Boolean)),
            cm.EditorView.theme({
                '&': {
                    fontSize: 'var(--cs-editor-font-size, 12px)',
                    fontFamily: 'var(--cs-mono-font)',
                    backgroundColor: 'var(--cs-panel-soft)',
                    color: 'var(--cs-text)'
                },
                '.cm-scroller': {
                    fontFamily: 'var(--cs-mono-font)'
                },
                '.cm-gutters': {
                    background: 'var(--cs-panel)',
                    color: 'var(--cs-muted)',
                    borderRight: '1px solid var(--cs-border-subtle)'
                },
                '.cm-activeLineGutter': {
                    background: 'var(--cs-accent-soft)'
                },
                '.cm-activeLine': {
                    background: 'var(--cs-accent-faint)'
                },
                '.cm-matchingBracket': {
                    background: 'var(--cs-accent-soft)',
                    outline: '1px solid var(--cs-accent-glow)'
                },
                '.cm-selectionBackground': {
                    background: 'var(--cs-selection) !important'
                },
                '&.cm-focused .cm-selectionBackground': {
                    background: 'var(--cs-selection-strong) !important'
                },
                '.cm-cursor': {
                    borderLeftColor: 'var(--cs-accent)',
                    borderLeftWidth: '2px'
                },
                '.cm-indentGuide': {
                    borderLeft: '1px solid var(--cs-border-subtle)'
                }
            }, { dark: !light }),
            link ? cm.EditorView.updateListener.of(bind(update => syncLinkedView(link, update))) : null,
            cm.EditorView.updateListener.of(bind(update => {
                if (update.selectionSet && !update.docChanged) {
                    renderStatus();
                    return;
                }
                if (!update.docChanged) return;
                tab.modified = true;
                tab.content = update.state.doc.toString();
                renderTabs();
                renderStatus();
            }))
        ].filter(Boolean);
        return new cm.EditorView({
            state: cm.EditorState.create({ doc: tab.content, extensions }),
            parent: container
        });
    }

    function createTextareaEditor(container, tab, link) {
        const wrapper = document.createElement('div');
        wrapper.className = 'cs-textarea-wrap';
        const textarea = document.createElement('textarea');
        textarea.className = 'code-studio-textarea';
        textarea.value = tab.content;
        textarea.spellcheck = false;
        const preview = document.createElement('pre');
        preview.className = 'code-studio-preview hljs';
        wrapper.appendChild(textarea);
        wrapper.appendChild(preview);
        container.appendChild(wrapper);
        const view = { textarea, getValue: () => textarea.value, setValue: value => { textarea.value = value; updatePreview(); } };
        const updatePreview = bind(() => {
            tab.content = textarea.value;
            tab.modified = true;
            preview.textContent = textarea.value;
            if (window.hljs && tab.language) {
                try {
                    preview.innerHTML = window.hljs.highlight(textarea.value, { language: tab.language, ignoreIllegals: true }).value;
                } catch (_) {}
            }
            if (link && !link.syncing) {
                link.syncing = true;
                try {
                    (link.views || []).forEach(other => {
                        if (other && other !== view && other.textarea && other.textarea.value !== textarea.value) other.setValue(textarea.value);
                    });
                } finally {
                    link.syncing = false;
                }
            }
            renderTabs();
            renderStatus();
        });
        textarea.addEventListener('input', updatePreview);
        textarea.addEventListener('keyup', bind(() => renderStatus()));
        textarea.addEventListener('click', bind(() => renderStatus()));
        textarea.addEventListener('keydown', bind(event => {
            if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
                event.preventDefault();
                saveCurrentFile();
            }
            if (event.key === 'Tab') {
                event.preventDefault();
                const start = textarea.selectionStart;
                const end = textarea.selectionEnd;
                textarea.value = textarea.value.slice(0, start) + '    ' + textarea.value.slice(end);
                textarea.selectionStart = textarea.selectionEnd = start + 4;
                updatePreview();
            }
        }));
        updatePreview();
        tab.modified = false;
        return view;
    }
