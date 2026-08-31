(function () {
    'use strict';

    window.TerminalApp = {
        render(host, windowId, ctx) {
            if (!host) return;
            host.innerHTML = `<div class="vd-terminal-app">
                <div class="vd-terminal-toolbar">
                    <span class="vd-terminal-status" data-terminal-status>${ctx.t('desktop.loading')}</span>
                </div>
                <div class="vd-terminal-screen" data-terminal-screen></div>
            </div>`;
            const screen = host.querySelector('[data-terminal-screen]');
            const status = host.querySelector('[data-terminal-status]');
            let ws = null;
            let term = null;
            const cleanup = () => {
                if (ws && ws.readyState !== WebSocket.CLOSED) ws.close();
                if (term && typeof term.dispose === 'function') term.dispose();
            };
            if (typeof ctx.registerWindowCleanup === 'function') ctx.registerWindowCleanup(windowId, cleanup);
            if (!window.Terminal) {
                if (status) status.textContent = ctx.t('desktop.terminal_unavailable');
                return;
            }
            term = new window.Terminal({
                cursorBlink: true,
                convertEol: true,
                fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
                fontSize: 13
            });
            term.open(screen);
            if (window.FitAddon && window.FitAddon.FitAddon) {
                const fit = new window.FitAddon.FitAddon();
                term.loadAddon(fit);
                fit.fit();
                window.setTimeout(() => fit.fit(), 80);
            }
            const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
            ws = new WebSocket(protocol + '//' + location.host + '/api/code-studio/terminal');
            ws.binaryType = 'arraybuffer';
            ws.onopen = () => {
                if (status) status.textContent = ctx.t('desktop.terminal_running');
                term.onData(data => {
                    if (ws.readyState === WebSocket.OPEN) ws.send(data);
                });
            };
            ws.onmessage = event => {
                if (event.data instanceof ArrayBuffer) term.write(new Uint8Array(event.data));
                else term.write(String(event.data));
            };
            ws.onerror = () => {
                if (status) status.textContent = ctx.t('desktop.terminal_unavailable');
            };
            ws.onclose = () => {
                if (status) status.textContent = ctx.t('desktop.terminal_stopped');
            };
        },
        dispose() {}
    };
})();
