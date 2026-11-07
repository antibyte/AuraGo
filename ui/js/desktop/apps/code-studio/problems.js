    // Problem tracking. Diagnostics come from two places: cheap client side checks that run
    // while the user types, and a real project check that runs the toolchain inside the
    // container. Both end up in the same list so the panel, the inline squiggles and the
    // status bar counters always agree.

    const BRACKET_LANGUAGES = ['javascript', 'go', 'c', 'rust', 'css'];
    const MAX_PROBLEMS = 500;
    // Sources of the cheap client side checks. They are replaced whenever the
    // document changes, so they must not collide with the container checks.
    const CLIENT_PROBLEM_SOURCES = ['syntax', 'json'];

    // A slash starts a regular expression literal only where an operand is expected.
    // Everywhere else it is a division, so the scanner never eats real code.
    const REGEX_PRECEDING_CHARS = new Set(['(', ',', '=', ':', '[', '!', '&', '|', '?', '{', '}', ';', '+', '-', '*', '%', '~', '^', '<', '>']);
    const REGEX_PRECEDING_WORDS = new Set(['return', 'typeof', 'instanceof', 'in', 'of', 'new', 'delete', 'void', 'case', 'do', 'else', 'yield', 'await']);

    function regexCanStartAfter(output) {
        const trimmed = String(output || '').replace(/\s+$/, '');
        if (!trimmed) return true;
        const last = trimmed[trimmed.length - 1];
        if (/[A-Za-z_$]/.test(last)) {
            const word = /([A-Za-z_$][\w$]*)$/.exec(trimmed);
            return !!word && REGEX_PRECEDING_WORDS.has(word[1]);
        }
        return REGEX_PRECEDING_CHARS.has(last);
    }

    function problemCounts() {
        const problems = state.problems || [];
        let errors = 0;
        let warnings = 0;
        problems.forEach(problem => {
            if (problem.severity === 'error') errors += 1;
            else if (problem.severity === 'warning') warnings += 1;
        });
        return { errors, warnings };
    }

    function normalizeProblem(problem) {
        const severity = ['error', 'warning', 'info'].includes(problem.severity) ? problem.severity : 'error';
        return {
            file: String(problem.file || ''),
            line: Math.max(1, Number(problem.line) || 1),
            column: Math.max(1, Number(problem.column) || 1),
            severity,
            message: String(problem.message || '').trim(),
            source: String(problem.source || '')
        };
    }

    function setProblems(problems) {
        state.problems = (problems || []).map(normalizeProblem).slice(0, MAX_PROBLEMS);
        renderBottomPanel();
        applyDiagnosticsToOpenTabs();
        renderStatus();
    }

    function addProblems(problems) {
        if (!problems || !problems.length) return;
        const existing = new Set((state.problems || []).map(problem => `${problem.file}|${problem.line}|${problem.column}|${problem.message}`));
        const merged = (state.problems || []).slice();
        problems.map(normalizeProblem).forEach(problem => {
            const key = `${problem.file}|${problem.line}|${problem.column}|${problem.message}`;
            if (existing.has(key)) return;
            existing.add(key);
            merged.push(problem);
        });
        state.problems = merged.slice(0, MAX_PROBLEMS);
        renderBottomPanel();
        applyDiagnosticsToOpenTabs();
        renderStatus();
    }

    function clearProblems() {
        setProblems([]);
    }

    function problemsForFile(path) {
        const target = String(path || '');
        // Only findings that name this exact file. Workspace wide findings stay in the list
        // but must not paint a squiggle into every open document.
        return (state.problems || []).filter(problem => !!problem.file && problem.file === target);
    }

    // Compilers report paths relative to the project root, so they are anchored at the
    // workspace before they are opened or matched against a tab.
    function normalizeDiagnosticPath(file, base) {
        const value = String(file || '').trim().replace(/\\/g, '/');
        if (!value) return '';
        if (value.startsWith('/')) return value;
        const root = String(base || WORKSPACE_ROOT).replace(/\/+$/, '');
        return `${root}/${value.replace(/^\.\//, '')}`;
    }

    function cursorLineNumber(tab) {
        if (!tab) return 1;
        const view = tab.focusedView || tab.view;
        if (view && view.state && view.state.doc && view.state.selection) {
            try {
                return view.state.doc.lineAt(view.state.selection.main.head).number;
            } catch (_) {
                return 1;
            }
        }
        if (view && view.textarea) {
            return view.textarea.value.slice(0, view.textarea.selectionStart).split('\n').length;
        }
        return 1;
    }

    function renderProblemsPanel() {
        const panel = shellPart('[data-problems-panel]');
        if (!panel) return;
        const problems = state.problems || [];
        if (!problems.length) {
            panel.innerHTML = `<div class="cs-empty">${esc(state.diagnosticsRunning
                ? tr('codeStudio.diagnosticsRunning', 'Checking...')
                : tr('codeStudio.noProblems', 'No problems have been detected in the workspace.'))}</div>`;
            return;
        }
        const counts = problemCounts();
        const groups = [];
        problems.forEach(problem => {
            const key = problem.file || tr('codeStudio.problemWorkspace', 'Workspace');
            let group = groups.find(item => item.key === key);
            if (!group) {
                group = { key, label: key, items: [] };
                groups.push(group);
            }
            group.items.push(problem);
        });
        const summary = `<div class="cs-problems-summary">
            <span class="cs-problems-count error">${esc(tr('codeStudio.problemErrors', '{{count}} errors', { count: counts.errors }))}</span>
            <span class="cs-problems-count warning">${esc(tr('codeStudio.problemWarnings', '{{count}} warnings', { count: counts.warnings }))}</span>
        </div>`;
        const body = groups.map((group, groupIndex) => `
            <div class="cs-problems-group">
                <button type="button" class="cs-problems-file" data-problem-group="${groupIndex}">
                    <span class="cs-problems-caret">▾</span>
                    <span>${esc(group.label)}</span>
                    <span class="cs-problems-file-count">${esc(String(group.items.length))}</span>
                </button>
                ${group.items.map((problem, index) => `
                    <button type="button" class="cs-problem cs-problem-${esc(problem.severity)}" data-problem-index="${groupIndex}|${index}">
                        <span class="cs-problem-icon">${problem.severity === 'warning' ? '⚠' : (problem.severity === 'info' ? 'ℹ' : '✖')}</span>
                        <span class="cs-problem-message">${esc(problem.message)}</span>
                        ${problem.source ? `<span class="cs-problem-source">${esc(problem.source)}</span>` : ''}
                    </button>`).join('')}
            </div>`).join('');
        panel.innerHTML = summary + body;
        panel.querySelectorAll('[data-problem-group]').forEach(btn => {
            btn.addEventListener('click', bind(() => {
                const group = btn.closest('.cs-problems-group');
                if (group) group.classList.toggle('collapsed');
            }));
        });
        panel.querySelectorAll('[data-problem-index]').forEach(btn => {
            btn.addEventListener('click', bind(() => {
                const [groupIndex, index] = btn.dataset.problemIndex.split('|').map(Number);
                const group = groups[groupIndex];
                const problem = group && group.items[index];
                if (problem) jumpToProblem(problem);
            }));
        });
    }

    async function jumpToProblem(problem) {
        if (!problem) return;
        const target = state;
        if (!isLiveInstance(target)) return;
        if (problem.file && problem.file !== WORKSPACE_ROOT) {
            await runAsyncStep(target, () => openFile(problem.file));
            if (!isLiveInstance(target)) return;
        }
        runWithInstance(target, () => {
            const tab = activeTab();
            if (!tab) return;
            const view = tab.view;
            if (view && view.state && view.state.doc && state.cmModule && state.cmModule.EditorView) {
                const doc = view.state.doc;
                // The compiler may report a line that the current buffer does not have
                // (an older revision, a generated file), so it is clamped.
                const line = Math.min(Math.max(1, problem.line || 1), doc.lines);
                const column = Math.max(1, problem.column || 1);
                const docLine = doc.line(line);
                const pos = Math.min(docLine.to, docLine.from + column - 1);
                view.dispatch({
                    selection: { anchor: pos },
                    effects: state.cmModule.EditorView.scrollIntoView(pos, { y: 'center' })
                });
                view.focus();
            } else if (view && view.textarea) {
                const lines = view.textarea.value.split('\n');
                const line = Math.min(Math.max(1, problem.line || 1), lines.length);
                const offset = lines.slice(0, line - 1).join('\n').length + (line > 1 ? 1 : 0);
                view.textarea.focus();
                view.textarea.setSelectionRange(offset, offset);
            }
            renderStatus();
        });
    }

    function nextProblem(direction) {
        const all = (state.problems || []).slice().sort((a, b) => {
            if (a.file !== b.file) return String(a.file).localeCompare(String(b.file));
            if (a.line !== b.line) return a.line - b.line;
            return a.column - b.column;
        });
        if (!all.length) return;
        const tab = activeTab();
        const currentFile = tab ? tab.path : '';
        // Problems of the open file come first, the workspace wide ones are the fallback.
        const inFile = all.filter(problem => problem.file && problem.file === currentFile);
        const list = inFile.length ? inFile : all;
        const currentLine = cursorLineNumber(tab);
        let candidate = null;
        if (direction > 0) {
            candidate = list.find(problem => problem.line > currentLine) || list[0];
        } else {
            const earlier = list.filter(problem => problem.line < currentLine);
            candidate = earlier.length ? earlier[earlier.length - 1] : list[list.length - 1];
        }
        jumpToProblem(candidate);
    }

    // --- Client side checks -------------------------------------------------------

    function stripCodeNoise(code) {
        // Removes comments, string bodies and regular expression literals so bracket
        // counting never trips over them.
        let out = '';
        let i = 0;
        const length = code.length;
        while (i < length) {
            const char = code[i];
            const next = code[i + 1];
            if (char === '/' && next === '/') {
                while (i < length && code[i] !== '\n') i += 1;
                continue;
            }
            if (char === '/' && next === '*') {
                i += 2;
                while (i < length && !(code[i] === '*' && code[i + 1] === '/')) i += 1;
                i += 2;
                continue;
            }
            if (char === '/' && regexCanStartAfter(out)) {
                i += 1;
                let inClass = false;
                while (i < length) {
                    if (code[i] === '\\') { i += 2; continue; }
                    if (code[i] === '[') inClass = true;
                    else if (code[i] === ']') inClass = false;
                    else if (code[i] === '/' && !inClass) { i += 1; break; }
                    else if (code[i] === '\n') break;
                    i += 1;
                }
                out += '""';
                continue;
            }
            if (char === '"' || char === "'" || char === '`') {
                const quote = char;
                i += 1;
                while (i < length) {
                    if (code[i] === '\\') { i += 2; continue; }
                    if (code[i] === quote) { i += 1; break; }
                    if (quote !== '`' && code[i] === '\n') break;
                    i += 1;
                }
                out += '""';
                continue;
            }
            out += char;
            i += 1;
        }
        return out;
    }

    function checkBrackets(code) {
        const pairs = { ')': '(', ']': '[', '}': '{' };
        const openers = new Set(['(', '[', '{']);
        const stack = [];
        let line = 1;
        let column = 1;
        const clean = stripCodeNoise(code);
        for (let i = 0; i < clean.length; i += 1) {
            const char = clean[i];
            if (char === '\n') { line += 1; column = 1; continue; }
            if (openers.has(char)) stack.push({ char, line, column });
            else if (pairs[char]) {
                const top = stack.pop();
                if (!top) {
                    return [{ severity: 'error', line, column, message: tr('codeStudio.problemUnexpectedBracket', 'Unexpected closing bracket \'{{char}}\'', { char }), source: 'syntax' }];
                }
                if (top.char !== pairs[char]) {
                    return [{ severity: 'error', line: top.line, column: top.column, message: tr('codeStudio.problemMismatchedBracket', 'Mismatched bracket: expected \'{{expected}}\' but found \'{{found}}\'', { expected: top.char, found: char }), source: 'syntax' }];
                }
            }
            column += 1;
        }
        const top = stack.pop();
        if (top) {
            return [{ severity: 'error', line: top.line, column: top.column, message: tr('codeStudio.problemUnclosedBracket', 'Unclosed bracket \'{{char}}\'', { char: top.char }), source: 'syntax' }];
        }
        return [];
    }

    function checkJson(code) {
        try {
            JSON.parse(code);
            return [];
        } catch (err) {
            const message = String(err && err.message ? err.message : err);
            const position = /position (\d+)/i.exec(message);
            let line = 1;
            let column = 1;
            if (position) {
                const offset = Math.min(Number(position[1]) || 0, code.length);
                const before = code.slice(0, offset);
                line = before.split('\n').length;
                column = offset - before.lastIndexOf('\n');
            } else {
                const lineMatch = /line (\d+)/i.exec(message);
                if (lineMatch) line = Number(lineMatch[1]) || 1;
            }
            return [{ severity: 'error', line, column, message, source: 'json' }];
        }
    }

    function checkDocumentContent(tab) {
        if (!tab || tab.kind === 'diff') return [];
        const code = editorValue(tab);
        if (!code) return [];
        const language = tab.language || languageForPath(tab.path);
        if (language === 'json') return checkJson(code);
        if (BRACKET_LANGUAGES.includes(language)) return checkBrackets(code);
        return [];
    }

    function checkActiveDocument() {
        const tab = activeTab();
        if (!tab) return;
        const found = checkDocumentContent(tab);
        // The client checks of this document are dropped, the findings of the
        // container check and of the other files stay untouched.
        const others = (state.problems || []).filter(problem => !(CLIENT_PROBLEM_SOURCES.includes(problem.source) && problem.file === tab.path));
        const mine = found.map(problem => normalizeProblem({ ...problem, file: tab.path }));
        setProblems([...others, ...mine]);
    }

    // Typing triggers a check after a short pause so the problem list does not flicker.
    function scheduleDocumentCheck() {
        const instance = state;
        if (!instance) return;
        if (instance.checkTimer) clearTimeout(instance.checkTimer);
        instance.checkTimer = setTimeout(bindInstance(instance, () => {
            instance.checkTimer = null;
            if (!isLiveInstance(instance)) return;
            checkActiveDocument();
        }), 600);
    }

    // --- Container side project check ----------------------------------------------

    function detectProjectCheck() {
        const files = (state.files || []).map(entry => entry.name || '').concat(Object.values(state.treeCache || {}).flat().map(entry => entry.name || ''));
        const has = name => files.includes(name);
        const tab = activeTab();
        const path = tab ? tab.path : '';
        const ext = String(path).split('.').pop().toLowerCase();
        const lang = tab ? (tab.language || languageForPath(tab.path)) : '';
        // A project wide check is preferred when the project marker matches the open file.
        if (lang === 'go' && has('go.mod')) return { command: 'go build ./...', source: 'go build' };
        if (lang === 'rust' && has('Cargo.toml')) return { command: 'cargo check --message-format short', source: 'cargo check' };
        // Otherwise check the open file with the toolchain that understands it.
        if (ext === 'ts' || ext === 'tsx') return { command: `npx --no-install tsc --noEmit ${shellQuotePath(path)}`, source: 'tsc' };
        if (lang === 'go' && path) return { command: `go vet ${shellQuotePath(path)}`, source: 'go vet' };
        if (lang === 'rust' && path) return { command: `rustc --emit=metadata -o /dev/null ${shellQuotePath(path)}`, source: 'rustc' };
        if ((ext === 'js' || ext === 'mjs' || ext === 'cjs') && path) return { command: `node --check ${shellQuotePath(path)}`, source: 'node' };
        if (lang === 'python' && path) return { command: `python3 -m py_compile ${shellQuotePath(path)}`, source: 'py_compile' };
        if ((ext === 'c' || ext === 'h') && path) return { command: `gcc -fsyntax-only -std=c17 ${shellQuotePath(path)}`, source: 'gcc' };
        if (lang === 'json' && path) return { command: `python3 -m json.tool ${shellQuotePath(path)}`, source: 'json.tool' };
        // Finally fall back to whatever the project itself offers.
        if (has('go.mod')) return { command: 'go build ./...', source: 'go build' };
        if (has('Cargo.toml')) return { command: 'cargo check --message-format short', source: 'cargo check' };
        if (has('tsconfig.json')) return { command: 'npx --no-install tsc --noEmit', source: 'tsc' };
        if (has('requirements.txt') || has('pyproject.toml')) return { command: 'python3 -m compileall -q .', source: 'compileall' };
        if (has('package.json')) return { command: 'npm test --silent', source: 'npm test' };
        return null;
    }

    function shellQuotePath(path) {
        return "'" + String(path).replaceAll("'", "'\"'\"'") + "'";
    }

    function parseDiagnosticOutput(output, source, base) {
        const problems = [];
        const lines = String(output || '').split('\n');
        const push = (file, line, column, severity, message) => {
            if (!message) return;
            problems.push(normalizeProblem({ file: normalizeDiagnosticPath(file, base), line, column, severity, message, source }));
        };
        for (let i = 0; i < lines.length; i += 1) {
            const raw = lines[i];
            const line = raw.trim();
            if (!line) continue;
            // file:line:col: severity: message   (gcc, clang, tsc with --pretty)
            let match = /^(.*?):(\d+):(\d+):\s*(error|warning|note|info)\s*:\s*(.*)$/i.exec(line);
            if (match) {
                push(match[1], Number(match[2]), Number(match[3]), match[4].toLowerCase() === 'note' ? 'info' : match[4].toLowerCase(), match[5]);
                continue;
            }
            // file:line:col: message              (go build, go vet)
            match = /^(.*?):(\d+):(\d+):\s*(.*)$/.exec(line);
            if (match && !/^\s*(https?:\/\/)/.test(line)) {
                push(match[1], Number(match[2]), Number(match[3]), 'error', match[4]);
                continue;
            }
            // file:line: severity: message
            match = /^(.*?):(\d+):\s*(error|warning)\s*:\s*(.*)$/i.exec(line);
            if (match) {
                push(match[1], Number(match[2]), 1, match[3].toLowerCase(), match[4]);
                continue;
            }
            // file(line,col): severity message    (tsc)
            match = /^(.*?)\((\d+),(\d+)\):\s*(error|warning)\s+(TS\d+:)?\s*(.*)$/i.exec(line);
            if (match) {
                push(match[1], Number(match[2]), Number(match[3]), match[4].toLowerCase(), match[6]);
                continue;
            }
            // file(line): severity message
            match = /^(.*?)\((\d+)\):\s*(error|warning)\s+(TS\d+:)?\s*(.*)$/i.exec(line);
            if (match) {
                push(match[1], Number(match[2]), 1, match[3].toLowerCase(), match[5]);
                continue;
            }
            // Python traceback: File "path", line N
            match = /^\s*File "([^"]+)",\s*line\s*(\d+)/.exec(raw);
            if (match) {
                const detail = lines.slice(i + 1).find(entry => /^\s+\S/.test(entry) && !/^\s*File "/.test(entry));
                const text = (detail || '').trim();
                push(match[1], Number(match[2]), 1, 'error', text || tr('codeStudio.problemSyntaxError', 'Syntax error'));
                continue;
            }
            // Rust: error[E0308]: message  +  --> file:line:col
            match = /^error(?:\[(\w+)\])?:\s*(.*)$/i.exec(line);
            if (match) {
                const pointer = lines.slice(i + 1, i + 4).map(entry => /^\s*-->\s*(.*?):(\d+):(\d+)/.exec(entry)).find(Boolean);
                if (pointer) push(pointer[1], Number(pointer[2]), Number(pointer[3]), 'error', match[2]);
                else push('', 1, 1, 'error', match[2]);
                continue;
            }
            // eslint style: path on its own line, then "  12:5  error  message"
            if (/^\/\S+/.test(line) || /^[A-Za-z]:[\\/]/.test(line)) {
                const detail = lines.slice(i + 1).find(entry => /^\s+\d+:\d+\s+(error|warning)\s+.*$/.test(entry));
                const detailMatch = detail && /^\s+(\d+):(\d+)\s+(error|warning)\s+(.*)$/.exec(detail);
                if (detailMatch) push(line, Number(detailMatch[1]), Number(detailMatch[2]), detailMatch[3].toLowerCase(), detailMatch[4]);
                continue;
            }
            // eslint compact style: path:12:5: message  (already covered above)
        }
        return problems;
    }

    async function runDiagnostics() {
        const target = state;
        if (!isLiveInstance(target) || target.diagnosticsRunning) return;
        if (!canWrite(target)) {
            flashStatus(tr('codeStudio.diagnosticsReadOnly', 'Checks need write access to the container.'));
            return;
        }
        const plan = detectProjectCheck();
        if (!plan) {
            flashStatus(tr('codeStudio.diagnosticsUnsupported', 'No supported project check found for this workspace.'));
            return;
        }
        target.diagnosticsRunning = true;
        runWithInstance(target, () => {
            renderBottomPanel();
            renderStatus(tr('codeStudio.diagnosticsRunning', 'Checking...'));
        });
        const request = Symbol();
        target.diagnosticsRequest = request;
        try {
            const result = await apiClient.exec(plan.command, WORKSPACE_ROOT, 180);
            if (!isLiveInstance(target) || target.diagnosticsRequest !== request) return;
            runWithInstance(target, () => {
                const output = [result && result.output ? result.output : ''].join('\n');
                appendOutput(output, plan.source);
                const parsed = parseDiagnosticOutput(output, plan.source, WORKSPACE_ROOT);
                const others = (state.problems || []).filter(problem => problem.source !== plan.source);
                setProblems([...others, ...parsed]);
                state.diagnosticsRunning = false;
                state.lastDiagnosticsAt = Date.now();
                renderBottomPanel();
                const counts = problemCounts();
                flashStatus(tr('codeStudio.diagnosticsDone', '{{errors}} errors, {{warnings}} warnings', {
                    errors: counts.errors,
                    warnings: counts.warnings
                }));
            });
        } catch (err) {
            if (!isLiveInstance(target) || target.diagnosticsRequest !== request) return;
            runWithInstance(target, () => {
                state.diagnosticsRunning = false;
                renderBottomPanel();
                showOperationError(err);
            });
        }
    }

    // --- Inline decorations --------------------------------------------------------

    function applyDiagnosticsToOpenTabs() {
        (state.openTabs || []).forEach(tab => applyDiagnosticsToTab(tab));
    }

    function applyDiagnosticsToTab(tab) {
        if (!tab || !tab.view) return;
        const cm = state.cmModule;
        if (!cm || !cm.setDiagnostics || !cm.EditorState) return;
        const findings = problemsForFile(tab.path);
        const view = tab.view;
        if (!view.state || !view.state.doc) return;
        const doc = view.state.doc;
        const diagnostics = findings.map(problem => {
            const lineNumber = Math.min(Math.max(1, problem.line), doc.lines);
            const docLine = doc.line(lineNumber);
            const from = Math.min(docLine.to, docLine.from + Math.max(0, problem.column - 1));
            const to = Math.min(docLine.to, Math.max(from + 1, docLine.from + Math.max(0, problem.column - 1)));
            return {
                from,
                to,
                severity: problem.severity === 'warning' ? 'warning' : (problem.severity === 'info' ? 'info' : 'error'),
                message: problem.message,
                source: problem.source || undefined
            };
        }).filter(diagnostic => diagnostic.to > diagnostic.from);
        try {
            // setDiagnostics is a helper that turns the findings into a transaction spec.
            view.dispatch(cm.setDiagnostics(view.state, diagnostics));
        } catch (_) {}
    }
