    function renderTerminal() {
        const terminal = shellPart('[data-terminal]');
        if (!terminal) return;
        const sessionTabs = (state.terminalSessions || []).map((session, index) => `
            <button type="button" class="cs-terminal-tab${index === (state.activeTerminalSession || 0) ? ' active' : ''}" data-terminal-tab="${index}">
                <span>${esc(session.name || 'Shell ' + (index + 1))}</span>
                <span class="cs-terminal-tab-close" data-terminal-close="${index}">\u00d7</span>
            </button>`).join('');
        const activeIdx = state.activeTerminalSession || 0;
        terminal.innerHTML = `<div class="cs-terminal-resize" data-terminal-resize></div>
            <div class="cs-terminal-head">
                <div class="cs-terminal-tabs">
                    ${sessionTabs || `<button type="button" class="cs-terminal-tab active" data-terminal-tab="0"><span>${esc(tr('codeStudio.terminal', 'Terminal'))}</span></button>`}
                    <button type="button" class="cs-terminal-add" data-terminal-add title="${esc(tr('codeStudio.newTerminal', 'New Terminal'))}">+</button>
                </div>
                <span data-terminal-state>${esc(tr('codeStudio.stopped', 'Stopped'))}</span>
            </div><div class="cs-terminal-screen" data-terminal-screen></div>`;
        wireTerminalResize();
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
        state.terminalSessions = [{ name: 'Shell 1', term: null, ws: null }];
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
            term.writeln('Code Studio - Shell ' + (index + 1));
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
        const screen = shellPart('[data-terminal-screen]');
        if (screen) screen.innerHTML = '';
        const session = state.terminalSessions[index];
        if (session && session.term) {
            const scr = shellPart('[data-terminal-screen]');
            if (scr) session.term.open(scr);
            if (session.fitAddon) session.fitAddon.fit();
            else if (state.fitAddon) state.fitAddon.fit();
        }
        state.terminal = session?.term || null;
        state.ws = session?.ws || null;
        renderTerminal();
    }

    function addTerminalSession() {
        if (!state.terminalSessions) state.terminalSessions = [];
        const index = state.terminalSessions.length;
        state.terminalSessions.push({ name: 'Shell ' + (index + 1), term: null, ws: null });
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
            state.terminalSessions.push({ name: 'Shell 1', term: null, ws: null });
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
