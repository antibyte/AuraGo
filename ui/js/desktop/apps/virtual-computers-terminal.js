(function () {
    'use strict';

    function esc(value) {
        return String(value == null ? '' : value).replace(/[&<>'"]/g, ch => ({
            '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;'
        }[ch]));
    }

    function mount(container, options) {
        if (!container) throw new Error('Terminal container is required');
        const settings = options || {};
        const t = typeof settings.t === 'function' ? settings.t : key => key;
        const notify = typeof settings.notify === 'function' ? settings.notify : function () {};
        const onClose = typeof settings.onClose === 'function' ? settings.onClose : function () {};
        let terminal = null;
        let fitAddon = null;
        let socket = null;
        let dataSubscription = null;
        let resizeObserver = null;
        let fitTimer = null;
        let disposed = false;
        let connectionFailed = false;

        container.classList.add('vc-terminal-session');
        container.dataset.machineId = String(settings.machineId || '');
        container.innerHTML = `
            <div class="vc-terminal-toolbar" role="toolbar" aria-label="${esc(t('desktop.virtual_computers_terminal'))}">
                <span class="vc-terminal-status" data-role="terminal-status" aria-live="polite"></span>
                <span class="vc-terminal-copy-hint">${esc(t('desktop.virtual_computers_terminal_copy_hint'))}</span>
                <div class="vc-terminal-actions">
                    <button type="button" class="vc-terminal-tool" data-terminal-action="reconnect">${esc(t('desktop.virtual_computers_terminal_reconnect'))}</button>
                    <button type="button" class="vc-terminal-tool" data-terminal-action="disconnect">${esc(t('desktop.virtual_computers_terminal_disconnect'))}</button>
                </div>
            </div>
            <div class="vc-terminal-stage" data-role="terminal-stage"></div>`;

        const stage = container.querySelector('[data-role="terminal-stage"]');
        const status = container.querySelector('[data-role="terminal-status"]');
        const reconnectButton = container.querySelector('[data-terminal-action="reconnect"]');
        const disconnectButton = container.querySelector('[data-terminal-action="disconnect"]');
        const encoder = new TextEncoder();

        function setStatus(key, state) {
            if (!status) return;
            status.textContent = t(key);
            status.dataset.state = state || '';
        }

        function clearFitTimer() {
            if (fitTimer == null) return;
            clearTimeout(fitTimer);
            fitTimer = null;
        }

        function fit() {
            if (disposed || !fitAddon) return;
            try { fitAddon.fit(); } catch (_) {}
        }

        function scheduleFit() {
            clearFitTimer();
            fitTimer = setTimeout(() => {
                fitTimer = null;
                fit();
            }, 40);
        }

        function detachSocket(current) {
            if (!current) return;
            current.removeEventListener?.('open', handleOpen);
            current.removeEventListener?.('message', handleMessage);
            current.removeEventListener?.('error', handleError);
            current.removeEventListener?.('close', handleClose);
        }

        function disconnectConnection() {
            const current = socket;
            socket = null;
            if (!current) return;
            detachSocket(current);
            try { current.close(); } catch (_) {}
        }

        function handleOpen(event) {
            if (disposed || (event && event.currentTarget && event.currentTarget !== socket)) return;
            connectionFailed = false;
            setStatus('desktop.virtual_computers_terminal_connected', 'connected');
            scheduleFit();
            terminal?.focus();
        }

        function handleMessage(event) {
            if (disposed || !terminal) return;
            const data = event.data;
            if (data instanceof ArrayBuffer) {
                terminal.write(new Uint8Array(data));
                return;
            }
            if (ArrayBuffer.isView(data)) {
                terminal.write(new Uint8Array(data.buffer, data.byteOffset, data.byteLength));
                return;
            }
            if (typeof data === 'string') terminal.write(data);
        }

        function handleError() {
            if (disposed) return;
            connectionFailed = true;
            const message = t('desktop.virtual_computers_terminal_error');
            setStatus('desktop.virtual_computers_terminal_error', 'error');
            notify(message, 'error');
        }

        function handleClose(event) {
            const current = event && event.currentTarget ? event.currentTarget : socket;
            if (current && current !== socket) return;
            if (socket) detachSocket(socket);
            socket = null;
            if (disposed || connectionFailed) return;
            setStatus('desktop.virtual_computers_terminal_disconnected', 'disconnected');
        }

        function connect() {
            if (disposed) return;
            disconnectConnection();
            connectionFailed = false;
            setStatus('desktop.virtual_computers_terminal_connecting', 'connecting');
            try {
                const nextSocket = new window.WebSocket(settings.url);
                socket = nextSocket;
                nextSocket.binaryType = 'arraybuffer';
                nextSocket.addEventListener('open', handleOpen);
                nextSocket.addEventListener('message', handleMessage);
                nextSocket.addEventListener('error', handleError);
                nextSocket.addEventListener('close', handleClose);
            } catch (_) {
                handleError();
            }
        }

        function isCopyShortcut(event) {
            const key = String(event.key || '').toLowerCase();
            return key === 'c' && ((event.ctrlKey && event.shiftKey) || event.metaKey);
        }

        function handleTerminalKey(event) {
            if (!isCopyShortcut(event) || !terminal || !terminal.hasSelection()) return true;
            const selection = terminal.getSelection();
            if (selection && navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
                const copy = navigator.clipboard.writeText(selection);
                if (copy && typeof copy.catch === 'function') copy.catch(function () {});
            }
            return false;
        }

        function handleReconnect() {
            connect();
        }

        function handleDisconnect() {
            disconnect();
            onClose();
        }

        function disconnect() {
            if (disposed) return;
            disposed = true;
            clearFitTimer();
            disconnectConnection();
            reconnectButton?.removeEventListener?.('click', handleReconnect);
            disconnectButton?.removeEventListener?.('click', handleDisconnect);
            if (resizeObserver) {
                resizeObserver.disconnect();
                resizeObserver = null;
            }
            if (dataSubscription) {
                try { dataSubscription.dispose(); } catch (_) {}
                dataSubscription = null;
            }
            if (terminal) {
                terminal.dispose();
                terminal = null;
            }
            fitAddon = null;
        }

        function reconnect() {
            connect();
        }

        if (!stage || !settings.url || !window.Terminal || !window.FitAddon || !window.FitAddon.FitAddon || !window.WebSocket || typeof ResizeObserver !== 'function') {
            const message = t('desktop.virtual_computers_terminal_unavailable');
            setStatus('desktop.virtual_computers_terminal_unavailable', 'error');
            notify(message, 'error');
            throw new Error(message);
        }

        try {
            terminal = new window.Terminal({
                cursorBlink: true,
                convertEol: true,
                fontFamily: '"Cascadia Mono", "SFMono-Regular", Consolas, monospace',
                fontSize: 13,
                scrollback: 5000,
                theme: { background: '#101419', foreground: '#e9eef4', cursor: '#7ec8ff' }
            });
            fitAddon = new window.FitAddon.FitAddon();
            terminal.loadAddon(fitAddon);
            terminal.open(stage);
            terminal.attachCustomKeyEventHandler(handleTerminalKey);
            dataSubscription = terminal.onData(data => {
                if (!socket || socket.readyState !== window.WebSocket.OPEN) return;
                socket.send(encoder.encode(data));
            });
            resizeObserver = new ResizeObserver(scheduleFit);
            resizeObserver.observe(container);
            reconnectButton?.addEventListener('click', handleReconnect);
            disconnectButton?.addEventListener('click', handleDisconnect);
            scheduleFit();
            connect();
        } catch (error) {
            disconnect();
            throw error;
        }

        return { disconnect, reconnect, fit };
    }

    window.VirtualComputersTerminal = { mount };
}());
