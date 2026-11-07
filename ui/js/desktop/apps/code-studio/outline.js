    // Document outline. A real IDE shows the symbols of the active file next to the tree so the
    // user can navigate a large file without scrolling. The scanner is intentionally simple:
    // it recognises the declaration shapes of the languages the studio can edit.

    const OUTLINE_RULES = {
        javascript: [
            { kind: 'function', regex: /^\s*(?:export\s+)?(?:default\s+)?(?:async\s+)?function\s*\*?\s*([A-Za-z_$][\w$]*)/ },
            { kind: 'class', regex: /^\s*(?:export\s+)?(?:default\s+)?(?:abstract\s+)?class\s+([A-Za-z_$][\w$]*)/ },
            { kind: 'function', regex: /^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*(?:async\s*)?(?:\([^)]*\)|[A-Za-z_$][\w$]*)\s*=>/ },
            { kind: 'variable', regex: /^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)/ }
        ],
        python: [
            { kind: 'class', regex: /^\s*class\s+([A-Za-z_][\w]*)/ },
            { kind: 'function', regex: /^\s*(?:async\s+)?def\s+([A-Za-z_][\w]*)/ }
        ],
        go: [
            { kind: 'function', regex: /^\s*func\s+(?:\([^)]*\)\s*)?([A-Za-z_][\w]*)/ },
            { kind: 'type', regex: /^\s*type\s+([A-Za-z_][\w]*)/ },
            { kind: 'variable', regex: /^\s*(?:const|var)\s+([A-Za-z_][\w]*)/ }
        ],
        c: [
            { kind: 'function', regex: /^\s*(?:static\s+|inline\s+|extern\s+)*[A-Za-z_][\w\s*]*?\b([A-Za-z_][\w]*)\s*\([^;]*\)\s*\{?\s*$/ },
            { kind: 'type', regex: /^\s*typedef\s+.*\b([A-Za-z_][\w]*)\s*;/ },
            { kind: 'variable', regex: /^\s*(?:#define\s+)([A-Za-z_][\w]*)/ }
        ],
        rust: [
            { kind: 'function', regex: /^\s*(?:pub(?:\([^)]*\))?\s+)?(?:async\s+)?fn\s+([A-Za-z_][\w]*)/ },
            { kind: 'type', regex: /^\s*(?:pub(?:\([^)]*\))?\s+)?(?:struct|enum|trait|type)\s+([A-Za-z_][\w]*)/ },
            { kind: 'type', regex: /^\s*impl(?:<[^>]*>)?\s+([A-Za-z_][\w]*)/ }
        ],
        json: [
            { kind: 'variable', regex: /^\s*"([^"]+)"\s*:/ }
        ],
        css: [
            { kind: 'type', regex: /^\s*([.#]?[A-Za-z_][\w-]*)\s*\{/ }
        ]
    };

    function outlineLanguage(tab) {
        if (!tab) return '';
        const language = tab.language || languageForPath(tab.path);
        return OUTLINE_RULES[language] ? language : '';
    }

    function scanDocumentSymbols(tab) {
        const code = editorValue(tab);
        if (!code) return [];
        const language = outlineLanguage(tab);
        if (!language) return [];
        const rules = OUTLINE_RULES[language];
        const lines = code.split('\n');
        const symbols = [];
        let depth = 0;
        lines.forEach((raw, index) => {
            const text = raw.trim();
            if (!text || text.startsWith('//') || text.startsWith('#') && language === 'python' && !/^#\s*def/.test(text)) {
                depth += countBrackets(raw);
                return;
            }
            const level = language === 'python' ? indentLevel(raw) : depth;
            for (const rule of rules) {
                const match = rule.regex.exec(raw);
                if (!match) continue;
                symbols.push({
                    name: match[1],
                    kind: rule.kind,
                    line: index + 1,
                    column: raw.length - raw.trimStart().length + 1,
                    level: Math.max(0, level)
                });
                break;
            }
            depth += countBrackets(raw);
        });
        return symbols;
    }

    function indentLevel(raw) {
        const match = /^\s*/.exec(raw);
        return Math.floor((match ? match[0].length : 0) / 4);
    }

    function countBrackets(raw) {
        const clean = stripCodeNoise(raw);
        let depth = 0;
        for (const char of clean) {
            if (char === '{') depth += 1;
            else if (char === '}') depth -= 1;
        }
        return depth;
    }

    function outlineSymbolIcon(kind) {
        if (kind === 'function') return 'ƒ';
        if (kind === 'class') return 'C';
        if (kind === 'type') return 'T';
        if (kind === 'variable') return '◇';
        return '•';
    }

    function renderOutline() {
        const sidebar = shellPart('[data-sidebar]');
        if (!sidebar) return;
        const tab = activeTab();
        const symbols = tab && tab.kind !== 'diff' ? scanDocumentSymbols(tab) : [];
        const head = `<div class="cs-sidebar-head">
            <div class="cs-sidebar-title">
                <strong>${esc(tr('codeStudio.outline', 'Outline'))}</strong>
            </div>
            <div class="cs-sidebar-path" data-sidebar-path>${esc(tab ? tab.path : tr('codeStudio.noFileOpen', 'No file open'))}</div>
        </div>`;
        const body = symbols.length
            ? `<div class="cs-outline" data-outline role="tree" aria-label="${esc(tr('codeStudio.outline', 'Outline'))}">${symbols.map((symbol, index) => `
                <button type="button" class="cs-outline-item" data-outline-index="${index}" data-outline-line="${symbol.line}" style="padding-left:${6 + symbol.level * 14}px" title="${esc(symbol.name)}">
                    <span class="cs-outline-icon">${esc(outlineSymbolIcon(symbol.kind))}</span>
                    <span class="cs-outline-name">${esc(symbol.name)}</span>
                    <span class="cs-outline-kind">${esc(symbol.kind)}</span>
                </button>`).join('')}</div>`
            : `<div class="cs-empty">${esc(tab ? tr('codeStudio.noOutline', 'No symbols found in this file.') : tr('codeStudio.noFileOpen', 'No file open'))}</div>`;
        sidebar.innerHTML = `${head}${body}<div class="cs-sidebar-resize" data-sidebar-resize></div>`;
        wireSidebarHead(sidebar);
        wireSidebarResize();
        sidebar.querySelectorAll('[data-outline-index]').forEach(btn => {
            btn.addEventListener('click', bind(() => {
                const symbol = symbols[Number(btn.dataset.outlineIndex)];
                if (symbol) jumpToLine(symbol.line, symbol.column);
            }));
        });
    }

    function jumpToLine(line, column) {
        const tab = activeTab();
        if (!tab) return;
        const lineNumber = Math.max(1, Number(line) || 1);
        const columnNumber = Math.max(1, Number(column) || 1);
        if (tab.view && tab.view.state && tab.view.state.doc && state.cmModule && state.cmModule.EditorView) {
            const doc = tab.view.state.doc;
            const docLine = doc.line(Math.min(lineNumber, doc.lines));
            const pos = Math.min(docLine.to, docLine.from + columnNumber - 1);
            tab.view.dispatch({
                selection: { anchor: pos },
                effects: state.cmModule.EditorView.scrollIntoView(pos, { y: 'center' })
            });
            tab.view.focus();
        } else if (tab.view && tab.view.textarea) {
            const rows = tab.view.textarea.value.split('\n');
            const offset = rows.slice(0, lineNumber - 1).join('\n').length + (lineNumber > 1 ? 1 : 0);
            tab.view.textarea.focus();
            tab.view.textarea.setSelectionRange(offset, offset);
        }
        renderStatus();
    }
