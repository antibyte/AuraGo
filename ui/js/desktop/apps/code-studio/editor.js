    function renderEditor() {
        const editor = shellPart('[data-editor]');
        if (!editor) return;
        const tab = activeTab();
        if (!tab) {
            state.openTabs.forEach(destroyTabView);
            editor.innerHTML = `<div class="cs-editor-empty">
                <div class="cs-empty-icon">{ }</div>
                <div class="cs-empty-title">${esc(tr('codeStudio.welcome', 'Welcome to Code Studio'))}</div>
                <div class="cs-empty-hint">${esc(tr('codeStudio.welcomeHint', 'Open a file from the sidebar or press Ctrl+Shift+P to open the Command Palette'))}</div>
            </div>`;
            return;
        }
        state.openTabs.forEach(openTab => {
            if (openTab !== tab) destroyTabView(openTab);
        });
        destroyTabView(tab);
        editor.innerHTML = '';
        tab.view = state.editorType === 'codemirror'
            ? createCodeMirrorEditor(editor, tab)
            : createTextareaEditor(editor, tab);
        editor.oncontextmenu = bind(event => {
            event.preventDefault();
            showCodeActionMenu(event.clientX, event.clientY);
        });
    }

    function createCodeMirrorEditor(container, tab) {
        const cm = state.cmModule;
        if (!cm || !cm.EditorState || !cm.EditorView) return createTextareaEditor(container, tab);
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
            cm.oneDark,
            cm.closeBrackets && cm.closeBrackets(),
            cm.autocompletion && cm.autocompletion(),
            cm.rectangularSelection && cm.rectangularSelection(),
            cm.crosshairCursor && cm.crosshairCursor(),
            cm.highlightSelectionMatches && cm.highlightSelectionMatches(),
            cm.syntaxHighlighting && cm.defaultHighlightStyle && cm.syntaxHighlighting(cm.defaultHighlightStyle),
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
                    fontSize: 'var(--cs-editor-font-size, 13px)',
                    fontFamily: 'var(--cs-mono-font)'
                },
                '.cm-scroller': {
                    fontFamily: 'var(--cs-mono-font)'
                },
                '.cm-gutters': {
                    background: 'var(--cs-panel)',
                    borderRight: '1px solid var(--cs-border-subtle)'
                },
                '.cm-activeLineGutter': {
                    background: 'var(--cs-accent-soft)'
                },
                '.cm-activeLine': {
                    background: 'rgba(62, 198, 181, 0.04)'
                },
                '.cm-matchingBracket': {
                    background: 'var(--cs-accent-soft)',
                    outline: '1px solid var(--cs-accent-glow)'
                },
                '.cm-selectionBackground': {
                    background: 'rgba(62, 198, 181, 0.18) !important'
                },
                '&.cm-focused .cm-selectionBackground': {
                    background: 'rgba(62, 198, 181, 0.22) !important'
                },
                '.cm-cursor': {
                    borderLeftColor: 'var(--cs-accent)',
                    borderLeftWidth: '2px'
                },
                '.cm-indentGuide': {
                    borderLeft: '1px solid var(--cs-border-subtle)'
                }
            }),
            cm.EditorView.updateListener.of(bind(update => {
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

    function createTextareaEditor(container, tab) {
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
        const updatePreview = bind(() => {
            tab.content = textarea.value;
            tab.modified = true;
            preview.textContent = textarea.value;
            if (window.hljs && tab.language) {
                try {
                    preview.innerHTML = window.hljs.highlight(textarea.value, { language: tab.language, ignoreIllegals: true }).value;
                } catch (_) {}
            }
            renderTabs();
            renderStatus();
        });
        textarea.addEventListener('input', updatePreview);
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
        return { textarea, getValue: () => textarea.value, setValue: value => { textarea.value = value; updatePreview(); } };
    }
