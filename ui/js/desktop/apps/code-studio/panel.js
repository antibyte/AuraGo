    // Bottom panel with Visual Studio style tabs (PROBLEMS / OUTPUT / TERMINAL).
    // The terminal lives inside the panel as one of its views, so the panel keeps a single
    // vertical slot and the user never loses the terminal while switching to diagnostics.
    const PANEL_VIEWS = ['problems', 'output', 'terminal'];

    function activePanelView() {
        return PANEL_VIEWS.includes(state.activePanel) ? state.activePanel : 'terminal';
    }

    function renderBottomPanel() {
        const panel = shellPart('[data-panel]');
        if (!panel) return;
        panel.hidden = !state.panelVisible;
        const root = studioRoot();
        if (root) {
            root.dataset.panel = state.panelVisible ? 'visible' : 'hidden';
            // Legacy hook: the terminal stylesheet keys off data-terminal.
            root.dataset.terminal = state.panelVisible ? 'visible' : 'hidden';
        }
        const active = activePanelView();
        panel.querySelectorAll('[data-panel-tab]').forEach(tab => {
            const name = tab.dataset.panelTab;
            const selected = name === active;
            tab.classList.toggle('active', selected);
            tab.setAttribute('aria-selected', selected ? 'true' : 'false');
        });
        panel.querySelectorAll('[data-panel-pane]').forEach(pane => {
            pane.hidden = pane.dataset.panelPane !== active;
        });
        const problemsBadge = panel.querySelector('[data-panel-badge="problems"]');
        if (problemsBadge) {
            const counts = problemCounts();
            const total = counts.errors + counts.warnings;
            problemsBadge.textContent = total ? String(total) : '';
            problemsBadge.hidden = !total;
            problemsBadge.classList.toggle('error', counts.errors > 0);
        }
        const runBtn = panel.querySelector('[data-panel-action="diagnostics"]');
        if (runBtn) {
            runBtn.disabled = !canWrite() || state.diagnosticsRunning;
            runBtn.textContent = state.diagnosticsRunning
                ? tr('codeStudio.diagnosticsRunning', 'Checking...')
                : tr('codeStudio.runDiagnostics', 'Run Checks');
        }
        const clearBtn = panel.querySelector('[data-panel-action="clear"]');
        if (clearBtn) {
            // The terminal owns its own scrollback, so clearing only applies to the
            // problems and the output view.
            clearBtn.disabled = active === 'terminal';
        }
        renderProblemsPanel();
        renderOutputPanel();
        if (active === 'terminal') renderTerminal();
    }

    function setActivePanel(view) {
        const next = PANEL_VIEWS.includes(view) ? view : 'terminal';
        let changed = state.activePanel !== next;
        state.activePanel = next;
        if (!state.panelVisible) {
            state.panelVisible = true;
            changed = true;
        }
        renderBottomPanel();
        if (changed) saveState();
        if (next === 'terminal') {
            if (state.fitAddon) setTimeout(bind(() => state.fitAddon.fit()), 50);
        }
    }

    function togglePanel(view) {
        const next = PANEL_VIEWS.includes(view) ? view : activePanelView();
        if (state.panelVisible && activePanelView() === next) {
            state.panelVisible = false;
            renderBottomPanel();
            saveState();
            return;
        }
        setActivePanel(next);
    }

    function toggleTerminal() {
        togglePanel('terminal');
    }

    function wireBottomPanelEvents() {
        const panel = shellPart('[data-panel]');
        if (!panel || panel.dataset.wired === 'true') return;
        panel.dataset.wired = 'true';
        panel.querySelectorAll('[data-panel-tab]').forEach(btn => {
            btn.addEventListener('click', bind(() => setActivePanel(btn.dataset.panelTab)));
        });
        panel.querySelectorAll('[data-panel-action]').forEach(btn => {
            btn.addEventListener('click', bind(() => {
                const action = btn.dataset.panelAction;
                if (action === 'close') {
                    state.panelVisible = false;
                    renderBottomPanel();
                    saveState();
                } else if (action === 'clear') {
                    if (activePanelView() === 'problems') clearProblems();
                    else if (activePanelView() === 'output') clearOutput();
                } else if (action === 'diagnostics') {
                    runDiagnostics();
                }
            }));
        });
    }

    // The panel owns the drag handle that resizes the whole bottom area.
    function wirePanelResize() {
        const handle = shellPart('[data-panel-resize]');
        if (!handle || handle.dataset.wired === 'true') return;
        handle.dataset.wired = 'true';
        let startY = 0;
        let startHeight = 0;
        const onPointerDown = bind(event => {
            event.preventDefault();
            const root = studioRoot();
            if (!root) return;
            startHeight = parseInt(root.style.getPropertyValue('--cs-terminal-height')) || state.terminalHeight || 220;
            startY = event.clientY;
            handle.classList.add('dragging');
            handle.setPointerCapture(event.pointerId);
            handle.addEventListener('pointermove', onPointerMove);
            handle.addEventListener('pointerup', onPointerUp);
            handle.addEventListener('pointercancel', onPointerUp);
        });
        const onPointerMove = bind(event => {
            const delta = startY - event.clientY;
            const newHeight = Math.max(MIN_TERMINAL_HEIGHT, Math.min(MAX_TERMINAL_HEIGHT, startHeight + delta));
            const root = studioRoot();
            if (root) root.style.setProperty('--cs-terminal-height', newHeight + 'px');
            state.terminalHeight = newHeight;
        });
        const onPointerUp = bind(event => {
            handle.classList.remove('dragging');
            handle.releasePointerCapture(event.pointerId);
            handle.removeEventListener('pointermove', onPointerMove);
            handle.removeEventListener('pointerup', onPointerUp);
            handle.removeEventListener('pointercancel', onPointerUp);
            saveState();
            if (state.fitAddon) setTimeout(bind(() => state.fitAddon.fit()), 50);
            scheduleTerminalFit();
        });
        handle.addEventListener('pointerdown', onPointerDown);
    }

    function appendOutput(text, source) {
        const lines = String(text == null ? '' : text).split('\n');
        const target = state;
        runWithInstance(target, () => {
            const entries = state.outputLines || (state.outputLines = []);
            lines.forEach(line => {
                if (!line && !entries.length) return;
                entries.push({ text: line, source: source || '', at: Date.now() });
            });
            while (entries.length > 500) entries.shift();
            if (source) state.outputSource = source;
            if (state.activePanel === 'output') renderOutputPanel();
        });
    }

    function clearOutput() {
        state.outputLines = [];
        renderOutputPanel();
    }

    function renderOutputPanel() {
        const panel = shellPart('[data-output-panel]');
        if (!panel) return;
        const entries = state.outputLines || [];
        panel.innerHTML = entries.length
            ? entries.map(entry => `<div class="cs-output-line"><span class="cs-output-source">${esc(entry.source || '')}</span><span class="cs-output-text">${esc(entry.text)}</span></div>`).join('')
            : `<div class="cs-empty">${esc(tr('codeStudio.noOutput', 'No output yet. Run the program or the checks to see its output here.'))}</div>`;
        panel.scrollTop = panel.scrollHeight;
    }
