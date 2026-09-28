    function shellName(index) {
        return tr('codeStudio.shell_n', 'Shell {{n}}', { n: index + 1 });
    }

    function renderTerminal() {
        const terminal = shellPart('[data-terminal]');
        if (!terminal) return;
        const sessionTabs = (state.terminalSessions || []).map((session, index) => `
            <button type="button" class="cs-terminal-tab${index === (state.activeTerminalSession || 0) ? ' active' : ''}" data-terminal-tab="${index}">
                <span>${esc(session.name || shellName(index))}</span>
                <span class="cs-terminal-tab-close" data-terminal-close="${index}" title="${esc(tr('desktop.close', 'Close'))}">×</span>
            </button>`).join('');
        terminal.innerHTML = `<div class="cs-terminal-resize" data-terminal-resize></div>
            <div class="cs-terminal-head">
                <div class="cs-terminal-tabs">
                    ${sessionTabs || `<button type="button" class="cs-terminal-tab active" data-terminal-tab="0"><span>${esc(tr('codeStudio.terminal', 'Terminal'))}</span></button>`}
                    <button type="button" class="cs-terminal-add" data-terminal-add title="${esc(tr('codeStudio.newTerminal', 'New Terminal'))}">+</button>
                </div>
                <span class="cs-terminal-state" data-terminal-state>${esc(tr('codeStudio.stopped', 'Stopped'))}</span>
                <button type="button" class="cs-terminal-hide" data-terminal-hide title="${esc(tr('codeStudio.toggleTerminal', 'Toggle Terminal'))}">${iconMarkup('x', 'X', 'cs-icon-button-icon', 12)}</button>
            </div><div class="cs-terminal-screen" data-terminal-screen></div>`;
        wireTerminalResize();
        wireTerminalObserver(terminal);
        terminal.querySelectorAll('[data-terminal-tab]').forEach(btn => {
            btn.addEventListener('click', bind(() => switchTerminalSession(Number(btn.dataset.terminalTab))));
        });
        terminal.querySelectorAll('[data-terminal-close]').forEach(btn => {
            btn.addEventListener('click', bind(event => {
                event.stopPropagation();
                closeTerminalSession(Number(btn.dataset.terminalClose));
            }));
        });
        const addBtn = terminal.querySelector('[data-terminal-add]');
        if (addBtn) addBtn.addEventListener('click', bind(() => addTerminalSession()));
        const hideBtn = terminal.querySelector('[data-terminal-hide]');
        if (hideBtn) hideBtn.addEventListener('click', bind(() => toggleTerminal()));
    }

    function wireTerminalObserver(terminal) {
        if (state.terminalObserver || typeof ResizeObserver !== 'function') return;
        const instance = state;
        const observer = new ResizeObserver(bindInstance(instance, () => scheduleTerminalFit()));
        observer.observe(terminal);
        state.terminalObserver = observer;
        instance.disposers.push(() => observer.disconnect());
    }

    function scheduleTerminalFit() {
        const instance = state;
        if (!instance || instance.terminalFitScheduled) return;
        instance.terminalFitScheduled = true;
        const run = bindInstance(instance, () => {
            instance.terminalFitScheduled = false;
            refitTerminal();
        });
        if (typeof window.requestAnimationFrame === 'function') window.requestAnimationFrame(run);
        else setTimeout(run, 16);
    }

    function activeTerminalSession() {
        const sessions = state.terminalSessions || [];
        return sessions[state.activeTerminalSession || 0] || null;
    }

    function refitTerminal() {
        if (!state.terminalVisible || state.zenMode) return;
        const session = activeTerminalSession();
        const fitAddon = (session && session.fitAddon) || state.fitAddon;
        if (!fitAddon) return;
        const screen = shellPart('[data-terminal-screen]');
        if (!screen || !screen.clientHeight || !screen.clientWidth) return;
        try { fitAddon.fit(); } catch (_) { return; }
        sendTerminalResize(session);
    }

    function sendTerminalResize(session) {
        const term = (session && session.term) || state.terminal;
        const ws = (session && session.ws) || state.ws;
        if (!term || !ws || typeof WebSocket === 'undefined' || ws.readyState !== WebSocket.OPEN) return;
        const cols = Number(term.cols) || 0;
        const rows = Number(term.rows) || 0;
        if (!cols || !rows) return;
        if (session && session.lastCols === cols && session.lastRows === rows) return;
        if (session) {
            session.lastCols = cols;
            session.lastRows = rows;
        }
        try { ws.send(JSON.stringify({ type: 'resize', cols, rows })); } catch (_) {}
    }

    function wireTerminalResize() {
        const handle = shellPart('[data-terminal-resize]');
        if (!handle) return;
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
            const newHeight = Math.max(80, Math.min(600, startHeight + delta));
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

    function connectTerminal() {
        const screen = shellPart('[data-terminal-screen]');
        const label = shellPart('[data-terminal-state]');
        if (!screen || !window.Terminal) {
            if (screen) screen.textContent = tr('codeStudio.terminalUnavailable', 'Terminal unavailable');
            return;
        }
        state.terminalSessions = [{ name: shellName(0), term: null, ws: null }];
        state.activeTerminalSession = 0;
        connectTerminalSession(0, screen, label);
    }

    function connectTerminalSession(index, screen, label) {
        if (!screen) screen = shellPart('[data-terminal-screen]');
        if (!label) label = shellPart('[data-terminal-state]');
        if (!screen || !window.Terminal) return;
        try {
            const term = new window.Terminal({ cursorBlink: true, convertEol: true, fontFamily: "'Cascadia Code', 'JetBrains Mono', 'SF Mono', 'Fira Code', Consolas, monospace", fontSize: 13 });
            const instance = state;
            let terminalDisposed = false;
            instance.disposers.push(() => {
                if (terminalDisposed) return;
                terminalDisposed = true;
                if (term && typeof term.dispose === 'function') term.dispose();
            });
            if (window.FitAddon && window.FitAddon.FitAddon) {
                const fitAddon = new window.FitAddon.FitAddon();
                term.loadAddon(fitAddon);
                if (index === 0) state.fitAddon = fitAddon;
                if (state.terminalSessions[index]) state.terminalSessions[index].fitAddon = fitAddon;
            }
            term.open(screen);
            if (state.terminalSessions[index]) state.terminalSessions[index].term = term;
            if (index === 0) state.terminal = term;
            const fitTarget = state.terminalSessions[index]?.fitAddon || state.fitAddon;
            if (fitTarget) fitTarget.fit();
            term.writeln(tr('codeStudio.title', 'Code Studio') + ' - ' + shellName(index));
            const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
            const ws = new WebSocket(protocol + '//' + location.host + '/api/code-studio/terminal');
            ws.binaryType = 'arraybuffer';
            if (state.terminalSessions[index]) state.terminalSessions[index].ws = ws;
            if (index === 0) state.ws = ws;
            ws.onopen = bindInstance(instance, () => {
                if (label && index === (state.activeTerminalSession || 0)) label.textContent = tr('codeStudio.running', 'Running...');
                const termDataDispose = term.onData(bindInstance(instance, data => ws.readyState === WebSocket.OPEN && ws.send(data)));
                if (termDataDispose && typeof termDataDispose.dispose === 'function') {
                    instance.disposers.push(() => termDataDispose.dispose());
                }
                sendTerminalResize(state.terminalSessions[index]);
            });
            ws.onmessage = bindInstance(instance, event => {
                if (event.data instanceof ArrayBuffer) term.write(new Uint8Array(event.data));
                else term.write(String(event.data));
            });
            ws.onerror = bindInstance(instance, () => {
                if (label && index === (state.activeTerminalSession || 0)) label.textContent = tr('codeStudio.terminalUnavailable', 'Terminal unavailable');
            });
            ws.onclose = bindInstance(instance, () => {
                if (label && index === (state.activeTerminalSession || 0)) label.textContent = tr('codeStudio.stopped', 'Stopped');
            });
        } catch (err) {
            screen.textContent = tr('codeStudio.terminalUnavailable', 'Terminal unavailable');
        }
    }

    function switchTerminalSession(index) {
        if (!state.terminalSessions || index < 0 || index >= state.terminalSessions.length) return;
        state.activeTerminalSession = index;
        const session = state.terminalSessions[index];
        renderTerminal();
        mountActiveTerminalSession(session);
    }

    function mountActiveTerminalSession(session) {
        const screen = shellPart('[data-terminal-screen]');
        const label = shellPart('[data-terminal-state]');
        if (screen) screen.innerHTML = '';
        if (session && session.term && screen) {
            session.term.open(screen);
            if (session.fitAddon) session.fitAddon.fit();
            else if (state.fitAddon) state.fitAddon.fit();
            session.term.focus();
        }
        state.terminal = session?.term || null;
        state.ws = session?.ws || null;
        if (label && session && session.ws) {
            const open = typeof WebSocket !== 'undefined' && session.ws.readyState === WebSocket.OPEN;
            label.textContent = open ? tr('codeStudio.running', 'Running...') : tr('codeStudio.stopped', 'Stopped');
        }
        sendTerminalResize(session);
    }

    function addTerminalSession() {
        if (!state.terminalSessions) state.terminalSessions = [];
        const index = state.terminalSessions.length;
        state.terminalSessions.push({ name: shellName(index), term: null, ws: null });
        state.activeTerminalSession = index;
        renderTerminal();
        const screen = shellPart('[data-terminal-screen]');
        const label = shellPart('[data-terminal-state]');
        if (screen) screen.innerHTML = '';
        connectTerminalSession(index, screen, label);
    }

    function closeTerminalSession(index) {
        if (!state.terminalSessions || index < 0 || index >= state.terminalSessions.length) return;
        const session = state.terminalSessions[index];
        if (session) {
            if (session.ws && session.ws.readyState !== WebSocket.CLOSED) session.ws.close();
            if (session.term && typeof session.term.dispose === 'function') session.term.dispose();
        }
        state.terminalSessions.splice(index, 1);
        if (!state.terminalSessions.length) {
            state.terminalSessions.push({ name: shellName(0), term: null, ws: null });
            state.activeTerminalSession = 0;
            renderTerminal();
            const screen = shellPart('[data-terminal-screen]');
            const label = shellPart('[data-terminal-state]');
            if (screen) screen.innerHTML = '';
            connectTerminalSession(0, screen, label);
        } else {
            state.activeTerminalSession = Math.min(index, state.terminalSessions.length - 1);
            switchTerminalSession(state.activeTerminalSession);
        }
    }
