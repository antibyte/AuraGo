    // Quick Connect serial device I/O for one connection record: ordered Web
    // Serial control signals, host WebSocket control frames and idempotent
    // port/socket teardown. Bundled inside the Desktop shell IIFE before
    // quickconnect-serial.js, which owns session state and generation fencing.
    const QuickConnectSerialTransport = (() => {
        function create({ Socket, ownerWindow }) {
            // Serializes setSignals calls; force bypasses the queue limit and the
            // closed check so teardown can always lower DTR/RTS and clear Break.
            function queueBrowserControl(conn, signals, force) {
                conn.pendingControlCount = conn.pendingControlCount || 0;
                if (!force && conn.pendingControlCount >= 8) return Promise.reject(new Error('control_queue_full'));
                conn.pendingControlCount++;
                const operation = (conn.controlChain || Promise.resolve()).catch(() => {}).then(() => {
                    if (conn.closed && !force) return;
                    return conn.port.setSignals(signals);
                });
                conn.controlChain = operation.finally(() => { conn.pendingControlCount = Math.max(0, conn.pendingControlCount - 1); });
                return conn.controlChain;
            }

            function sendHostControl(conn, message) {
                if (!conn.socket || conn.socket.readyState !== Socket.OPEN) throw new Error('serial_socket_unavailable');
                conn.socket.send(JSON.stringify(message));
            }

            function closeHostSocket(conn) {
                const socket = conn && conn.socket;
                if (!socket) return Promise.resolve();
                if (conn.socketClosePromise) return conn.socketClosePromise;
                if (Socket.CLOSED !== undefined && socket.readyState === Socket.CLOSED) return Promise.resolve();
                conn.socketClosePromise = new Promise(resolve => {
                    let finished = false;
                    const finish = () => {
                        if (finished) return;
                        finished = true;
                        if (conn.socketCloseTimer) ownerWindow.clearTimeout(conn.socketCloseTimer);
                        conn.socketCloseTimer = null;
                        resolve();
                    };
                    try { socket.addEventListener('close', finish, { once: true }); } catch (_) {}
                    if (socket.readyState === Socket.OPEN) {
                        try { sendHostControl(conn, { type: 'disconnect' }); } catch (_) {}
                    }
                    try { socket.close(); } catch (_) { finish(); return; }
                    const timer = ownerWindow.setTimeout(finish, 1500);
                    if (finished) ownerWindow.clearTimeout(timer);
                    else conn.socketCloseTimer = timer;
                });
                return conn.socketClosePromise;
            }

            function closePort(conn) {
                if (!conn || !conn.port) return Promise.resolve();
                if (conn.closePromise) return conn.closePromise;
                conn.closePromise = (async () => {
                    if (conn.breakTimer) ownerWindow.clearTimeout(conn.breakTimer);
                    conn.breakTimer = null;
                    if (conn.openPromise) { try { await conn.openPromise; } catch (_) {} }
                    if (conn.opened) {
                        const signals = { dataTerminalReady: false, break: false };
                        if (conn.profile.options.flow_control !== 'hardware') signals.requestToSend = false;
                        try { await queueBrowserControl(conn, signals, true); } catch (_) {}
                    }
                    if (conn.serialDisconnectListener) {
                        try { conn.serialEventTarget && conn.serialEventTarget.removeEventListener('disconnect', conn.serialDisconnectListener); } catch (_) {}
                        try { conn.port.removeEventListener('disconnect', conn.serialDisconnectListener); } catch (_) {}
                        conn.serialDisconnectListener = null;
                    }
                    if (conn.reader) {
                        try { await conn.reader.cancel(); } catch (_) {}
                        try { conn.reader.releaseLock(); } catch (_) {}
                        conn.reader = null;
                    }
                    if (conn.writer) {
                        try { await conn.writer.abort(); } catch (_) {}
                        try { conn.writer.releaseLock(); } catch (_) {}
                        conn.writer = null;
                    }
                    try { await conn.port.close(); } catch (_) {}
                })();
                return conn.closePromise;
            }

            return { queueBrowserControl, sendHostControl, closeHostSocket, closePort };
        }

        return { create };
    })();
