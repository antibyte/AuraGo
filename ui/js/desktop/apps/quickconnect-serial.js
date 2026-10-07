    // Quick Connect serial session controller: profiles, capture, terminal and
    // browser/host connection lifecycle for one Quick Connect window. The profile
    // model, markup and device I/O live in quickconnect-serial-{model,views,transport}.js.
    window.QuickConnectSerial = (() => {
        const { STORAGE_KEY, MAX_PROFILES, MAX_PROFILE_BYTES, parseHex, formatHex, parseProfiles, normalizeProfile, errorKey, readProfileDraft } = QuickConnectSerialModel;
        const { shellMarkup, profileListMarkup, portOptionsMarkup, profileFormMarkup } = QuickConnectSerialViews;
        const MAX_CAPTURE_BYTES = 10 * 1024 * 1024;
        const MAX_CAPTURE_RECORDS = 10000;
        const MAX_PENDING_RX_BYTES = 1024 * 1024;
        const MAX_PENDING_TX_BYTES = 256 * 1024;
        const MAX_SEND_BYTES = 64 * 1024;

        function create(options) {
            const opts = options || {};
            const list = opts.list || opts.sidebar;
            const content = opts.content;
            const searchInput = opts.searchInput;
            const request = opts.api || api;
            const translate = opts.t || t;
            const getBootstrap = opts.getBootstrap || (() => (opts.context && opts.context.state && opts.context.state.bootstrap) || {});
            const ownerWindow = opts.window || window;
            const browserNavigator = opts.navigator || ownerWindow.navigator || navigator;
            const Socket = opts.WebSocket || ownerWindow.WebSocket;
            const TerminalCtor = opts.Terminal || ownerWindow.Terminal;
            const FitAddonCtor = opts.FitAddon || ownerWindow.FitAddon;
            const confirmDialog = opts.confirmDialog || (async () => false);
            const onSessionStart = opts.onSessionStart || (() => null);
            const onSessionEnd = opts.onSessionEnd || (() => {});
            const { queueBrowserControl, sendHostControl, closeHostSocket, closePort } = QuickConnectSerialTransport.create({ Socket, ownerWindow });
            let profiles = [];
            let selectedId = '';
            let loaded = false;
            let loading = false;
            let loadGeneration = 0;
            let ports = [];
            let portsLoading = false;
            let portsError = false;
            let portsGeneration = 0;
            let portsAbort = null;
            let mounted = false;
            let disposed = false;
            let connection = null;
            let connectionStop = Promise.resolve();
            let connectionGeneration = 0;
            let chooserPending = false;
            let chooserPendingGeneration = 0;
            let chooserGeneration = 0;
            let connectionAttemptGeneration = 0;
            let pendingAttemptSource = '';
            let authEnded = false;
            let terminal = null;
            let terminalInputDisposable = null;
            let terminalActivityListeners = null;
            let fitAddon = null;
            let resizeObserver = null;
            let receiveMode = 'text';
            let displaySessionId = 0;
            let displayGeneration = 0;
            let displayQueueBytes = 0;
            let capture = [];
            let captureBytes = 0;
            let captureDropped = false;
            let policySnapshot = null;

            function secureContext() {
                if (typeof ownerWindow.isSecureContext === 'boolean') return ownerWindow.isSecureContext;
                if (typeof globalThis.isSecureContext === 'boolean') return globalThis.isSecureContext;
                return true;
            }

            function tr(key) { return translate(key); }
            function bootstrap() { return policySnapshot || getBootstrap() || {}; }
            function readonly() { const b = bootstrap(); return authEnded || window._logoutInProgress === true || !!b.readonly || b.enabled === false; }
            function allowed(source) {
                const b = bootstrap();
                return !readonly() && (source === 'browser' ? b.serial_browser_enabled === true : b.serial_host_enabled === true);
            }
            function sourceDiagnostic(profile) {
                if (readonly()) return tr('desktop.qc_serial_read_only');
                if (!allowed(profile.source)) return tr('desktop.qc_serial_permission_disabled');
                if (profile.source === 'browser') {
                    if (!secureContext()) return tr('desktop.qc_serial_secure_context_required');
                    const policy = document.permissionsPolicy || document.featurePolicy;
                    if (policy && typeof policy.allowsFeature === 'function' && !policy.allowsFeature('serial')) return tr('desktop.qc_serial_browser_permission_denied');
                    const serial = browserNavigator && browserNavigator.serial;
                    return !serial || typeof serial.requestPort !== 'function' ? tr('desktop.qc_serial_browser_unsupported') : '';
                }
                if (portsLoading) return tr('desktop.loading');
                if (portsError) return tr('desktop.load_failed');
                const port = ports.find(item => item.name === profile.port);
                if (!port) return tr('desktop.qc_serial_unavailable_port');
                if (port.busy && !(connection && connection.source === 'host' && connection.profileId === profile.id)) return tr('desktop.qc_serial_port_busy');
                return '';
            }
            function serialErrorText(code, fallback) { return tr(errorKey(code, fallback)); }
            function setStatus(message, kind) {
                const status = content && content.querySelector('[data-serial-status]');
                if (!status) return;
                status.textContent = message || '';
                status.dataset.state = kind || 'ready';
            }
            function writeStatus(message, kind) { setStatus(message, kind); }
            function isCurrent(conn) {
                if (disposed || !conn || connection !== conn || conn.generation !== connectionGeneration) return false;
                if (conn.isOwnerCurrent) {
                    let current = false;
                    try { current = conn.isOwnerCurrent() === true; } catch (_) {}
                    if (!current) {
                        void stopConnection(conn, 'switch');
                        return false;
                    }
                }
                return true;
            }
            function updateButtons() {
                const root = content && content.querySelector('[data-qc-serial-app]');
                if (!root) return;
                const locked = readonly();
                root.querySelectorAll('[data-serial-save], [data-serial-create], [data-serial-delete]').forEach(button => { button.disabled = locked; });
                root.querySelectorAll('[data-serial-connect]').forEach(button => {
                    const profile = profiles.find(item => item.id === button.dataset.serialConnect);
                    button.disabled = locked || !profile || !allowed(profile.source);
                });
                const ready = !!(connection && connection.ready);
                const disconnect = root.querySelector('[data-serial-disconnect]');
                if (disconnect) disconnect.disabled = !connection;
                const breakButton = root.querySelector('[data-serial-break]');
                if (breakButton) breakButton.disabled = !ready;
                const dtr = root.querySelector('[data-serial-dtr]');
                const rts = root.querySelector('[data-serial-rts]');
                if (dtr) dtr.disabled = !ready;
                if (rts) rts.disabled = !ready || (connection.source === 'browser' && connection.profile.options.flow_control === 'hardware');
                const send = root.querySelector('[data-serial-send]');
                if (send) send.disabled = !ready || locked;
                const input = root.querySelector('[data-serial-input]');
                if (input) input.disabled = !ready || locked;
            }

            function startSessionTimers(conn) {
                const b = bootstrap();
                const timeout = (value, fallback) => Number.isFinite(Number(value)) && Number(value) > 0 ? Number(value) : fallback;
                conn.maxTimer = ownerWindow.setTimeout(() => { void stopConnection(conn, 'timeout'); }, timeout(b.remote_max_session_minutes, 60) * 60 * 1000);
                touchUserActivity(conn);
            }

            function touchUserActivity(conn) {
                if (!isCurrent(conn)) return;
                if (conn.idleTimer) ownerWindow.clearTimeout(conn.idleTimer);
                const b = bootstrap();
                const minutes = Number(b.remote_idle_timeout_minutes);
                const delay = Number.isFinite(minutes) && minutes > 0 ? minutes : 5;
                conn.idleTimer = ownerWindow.setTimeout(() => { void stopConnection(conn, 'timeout'); }, delay * 60 * 1000);
            }

            function clearSessionTimers(conn) {
                if (!conn) return;
                for (const key of ['idleTimer', 'maxTimer', 'breakTimer', 'deviceTimer']) {
                    if (conn[key]) ownerWindow.clearTimeout(conn[key]);
                    conn[key] = null;
                }
                conn.breakGeneration = (conn.breakGeneration || 0) + 1;
            }

            async function loadProfiles() {
                const generation = ++loadGeneration;
                loading = true;
                try {
                    const body = await request('/api/desktop/settings');
                    if (disposed || generation !== loadGeneration) return;
                    profiles = parseProfiles(body && body.settings && body.settings[STORAGE_KEY]);
                    if (!profiles.some(profile => profile.id === selectedId)) selectedId = profiles[0] && profiles[0].id || '';
                    loaded = true;
                } catch (_) {
                    if (disposed || generation !== loadGeneration) return;
                    profiles = [];
                    loaded = true;
                    writeStatus(tr('desktop.load_failed'), 'error');
                } finally {
                    if (!disposed && generation === loadGeneration) {
                        loading = false;
                        renderList();
                        renderEditor();
                        updateButtons();
                    }
                }
            }

            function ensureShell() {
                if (!list || !content || disposed) return false;
                if (content.querySelector('[data-qc-serial-app]')) {
                    mounted = true;
                    return true;
                }
                content.innerHTML = shellMarkup(tr);
                mounted = true;
                wireShell();
                ensureTerminal();
                if (!loaded && !loading) void loadProfiles();
                if (allowed('host') && !portsLoading) void refreshPorts();
                return true;
            }

            function wireShell() {
                const root = content.querySelector('[data-qc-serial-app]');
                if (!root) return;
                root.querySelector('[data-serial-disconnect]').addEventListener('click', () => { void disconnect('manual'); });
                root.querySelector('[data-serial-clear]').addEventListener('click', clearCapture);
                root.querySelector('[data-serial-export]').addEventListener('click', exportCapture);
                const rxMode = root.querySelector('[data-serial-rx-mode]');
                rxMode.value = receiveMode;
                rxMode.addEventListener('change', event => { setReceiveMode(event.target.value); });
                root.querySelector('[data-serial-send-mode]').addEventListener('change', event => {
                    const lineEnding = root.querySelector('[data-serial-line-ending]');
                    lineEnding.disabled = event.target.value === 'hex';
                    root.querySelector('[data-serial-input]').placeholder = event.target.value === 'hex' ? tr('desktop.qc_serial_hex_placeholder') : tr('desktop.qc_serial_input_placeholder');
                });
                root.querySelector('[data-serial-form]').addEventListener('submit', event => { event.preventDefault(); void sendInput(); });
                root.querySelector('[data-serial-break]').addEventListener('click', () => { void sendBreak(); });
                root.querySelector('[data-serial-dtr]').addEventListener('change', event => { void setSignal('dtr', event.target.checked); });
                root.querySelector('[data-serial-rts]').addEventListener('change', event => { void setSignal('rts', event.target.checked); });
            }

            function ensureTerminal() {
                if (terminal || !TerminalCtor || !FitAddonCtor || !content) return;
                const mount = content.querySelector('[data-serial-terminal]');
                if (!mount) return;
                terminal = new TerminalCtor({
                    theme: { background: '#0d1117', foreground: '#c9d1d9', cursor: '#58a6ff', selectionBackground: 'rgba(88, 166, 255, 0.3)' },
                    fontFamily: "'Cascadia Code', 'JetBrains Mono', 'Fira Code', 'Consolas', monospace",
                    fontSize: 13,
                    cursorBlink: true,
                    disableStdin: false,
                    scrollback: 5000,
                    convertEol: true
                });
                fitAddon = new FitAddonCtor.FitAddon();
                terminal.loadAddon(fitAddon);
                terminal.open(mount);
                if (typeof terminal.onData === 'function') {
                    const inputTerminal = terminal;
                    const inputGeneration = displayGeneration;
                    terminalInputDisposable = terminal.onData(data => {
                        if (terminal !== inputTerminal || displayGeneration !== inputGeneration) return;
                        const conn = connection;
                        if (!conn || !conn.ready || readonly() || !isCurrent(conn)) return;
                        const root = content && content.querySelector('[data-qc-serial-app]');
                        const ending = root && root.querySelector('[data-serial-line-ending]')?.value || 'cr';
                        const endings = { none: '', cr: '\r', lf: '\n', crlf: '\r\n' };
                        const text = String(data).replace(/\r\n|\r|\n/g, endings[ending] || '');
                        if (text) void queueTransmit(conn, new TextEncoder().encode(text), 'text', false);
                    });
                }
                terminalActivityListeners = event => {
                    if (event.isTrusted !== true) return;
                    const conn = connection;
                    if (conn && conn.ready) touchUserActivity(conn);
                };
                if (typeof mount.addEventListener === 'function') {
                    mount.addEventListener('keydown', terminalActivityListeners);
                    mount.addEventListener('paste', terminalActivityListeners);
                }
                resizeObserver = typeof ResizeObserver === 'function' ? new ResizeObserver(() => fitTerminal()) : null;
                if (resizeObserver) resizeObserver.observe(mount);
                ownerWindow.setTimeout(() => fitTerminal(), 0);
            }

            function fitTerminal() {
                if (fitAddon) { try { fitAddon.fit(); } catch (_) {} }
            }

            function disposeTerminal() {
                if (terminalInputDisposable) {
                    try { terminalInputDisposable.dispose(); } catch (_) {}
                    terminalInputDisposable = null;
                }
                if (terminalActivityListeners) {
                    const mount = content && content.querySelector('[data-serial-terminal]');
                    if (mount && typeof mount.removeEventListener === 'function') {
                        mount.removeEventListener('keydown', terminalActivityListeners);
                        mount.removeEventListener('paste', terminalActivityListeners);
                    }
                    terminalActivityListeners = null;
                }
                if (resizeObserver) { resizeObserver.disconnect(); resizeObserver = null; }
                if (terminal) { try { terminal.dispose(); } catch (_) {} terminal = null; }
                fitAddon = null;
            }

            function render() {
                if (!ensureShell()) return;
                renderList();
                renderEditor();
                updateButtons();
            }

            function renderList() {
                if (!list) return;
                const query = String(searchInput && searchInput.value || '').trim().toLowerCase();
                const filtered = profiles.filter(profile => !query || `${profile.name} ${profile.source} ${profile.port}`.toLowerCase().includes(query));
                list.innerHTML = profileListMarkup({ profiles: filtered, selectedId, loading, readonly: readonly(), isConnectable: profile => allowed(profile.source) && !readonly(), diagnose: sourceDiagnostic, tr });
                list.querySelector('[data-serial-create]')?.addEventListener('click', () => { selectedId = ''; renderEditor(); });
                list.querySelectorAll('[data-serial-profile]').forEach(button => button.addEventListener('click', () => { selectedId = button.dataset.serialProfile; renderList(); renderEditor(); }));
                list.querySelectorAll('[data-serial-connect]').forEach(button => button.addEventListener('click', () => { void connectProfile(button.dataset.serialConnect); }));
                list.querySelectorAll('[data-serial-delete]').forEach(button => button.addEventListener('click', () => { void deleteProfile(button.dataset.serialDelete); }));
            }

            function renderEditor() {
                if (!content || !content.querySelector('[data-qc-serial-app]')) return;
                const editor = content.querySelector('[data-serial-editor]');
                if (!editor) return;
                const current = profiles.find(profile => profile.id === selectedId);
                editor.innerHTML = profileFormMarkup({ current, readonly: readonly(), ports, portsError, tr });
                const form = editor.querySelector('[data-serial-profile-form]');
                form.addEventListener('submit', event => { event.preventDefault(); void saveProfile(form); });
                form.querySelector('[name="source"]').addEventListener('change', event => {
                    const nextSource = event.target.value;
                    connectionAttemptGeneration++;
                    pendingAttemptSource = '';
                    if (chooserPending) chooserGeneration++;
                    if (connection && connection.profileId === selectedId && connection.source !== nextSource) void disconnect('switch');
                    form.querySelector('[data-serial-browser-fields]').hidden = nextSource !== 'browser';
                    form.querySelector('[data-serial-host-fields]').hidden = nextSource !== 'host';
                    const hardware = form.querySelector('[name="flow_control"] option[value="hardware"]');
                    hardware.disabled = nextSource === 'host';
                    hardware.textContent = `${tr('desktop.qc_serial_hardware')}${nextSource === 'host' ? ` — ${tr('desktop.qc_serial_hardware_host_unsupported')}` : ''}`;
                    if (nextSource === 'host' && form.querySelector('[name="flow_control"]').value === 'hardware') form.querySelector('[name="flow_control"]').value = 'none';
                    form.querySelector('[name="rts"]').disabled = readonly() || (nextSource === 'browser' && form.querySelector('[name="flow_control"]').value === 'hardware');
                    if (nextSource === 'host' && allowed('host') && !ports.length && !portsLoading) void refreshPorts();
                });
                form.querySelector('[name="baud_preset"]').addEventListener('change', event => {
                    form.querySelector('[data-custom-baud]').hidden = event.target.value !== 'custom';
                });
                form.querySelector('[name="flow_control"]').addEventListener('change', event => {
                    const rts = form.querySelector('[name="rts"]');
                    rts.disabled = readonly() || (form.elements.source.value === 'browser' && event.target.value === 'hardware');
                });
                form.querySelector('[data-serial-refresh-ports]')?.addEventListener('click', () => { void refreshPorts(); });
                if (connection) syncSignalControls();
            }

            function updateHostPortControls() {
                const form = content && content.querySelector('[data-serial-profile-form]');
                if (!form || form.elements.source.value !== 'host') return;
                const port = form.elements.port;
                const selected = port.value || (profiles.find(item => item.id === selectedId) || {}).port || '';
                port.innerHTML = portOptionsMarkup(selected, ports, portsError, tr);
                if (selected) port.value = selected;
                const error = form.querySelector('[data-serial-port-error]');
                if (error) {
                    error.hidden = !portsError;
                    error.textContent = portsError ? tr('desktop.load_failed') : '';
                }
            }

            function resetTerminalDisplay() {
                displayGeneration++;
                displayQueueBytes = 0;
                disposeTerminal();
                ensureTerminal();
                const note = content && content.querySelector('[data-serial-capture-note]');
                if (note && !captureDropped) { note.hidden = true; note.textContent = ''; }
            }

            function terminalWrite(output, conn) {
                const size = typeof output === 'string' ? output.length : output.byteLength;
                if (!terminal || displayQueueBytes + size > MAX_PENDING_RX_BYTES) {
                    showCaptureNote(tr('desktop.qc_serial_render_limit'));
                    return false;
                }
                displayQueueBytes += size;
                const token = displayGeneration;
                terminal.write(output, () => {
                    if (token === displayGeneration) displayQueueBytes = Math.max(0, displayQueueBytes - size);
                });
                return true;
            }

            function setReceiveMode(value) {
                receiveMode = value === 'hex' ? 'hex' : 'text';
                if (connection) connection.rxMode = receiveMode;
                resetTerminalDisplay();
                showCaptureNote(captureDropped ? tr('desktop.qc_serial_capture_limit') : tr('desktop.qc_serial_mode_changed'));
            }

            async function refreshPorts() {
                if (!allowed('host') || disposed) return;
                if (portsAbort) portsAbort.abort();
                portsAbort = new AbortController();
                const generation = ++portsGeneration;
                portsLoading = true;
                portsError = false;
                try {
                    const body = await request('/api/desktop/serial/ports', { signal: portsAbort.signal });
                    if (disposed || generation !== portsGeneration) return;
                    ports = Array.isArray(body && body.ports) ? body.ports.filter(port => port && typeof port.name === 'string').map(port => ({ name: port.name, busy: port.busy === true })) : [];
                    if (connection && connection.source === 'host' && !ports.some(port => port.name === connection.profile.port)) void disconnect('remote');
                } catch (_) {
                    if (disposed || generation !== portsGeneration) return;
                    portsError = true;
                } finally {
                    if (!disposed && generation === portsGeneration) {
                        portsLoading = false;
                        updateHostPortControls();
                        renderList();
                    }
                }
            }

            async function saveProfile(form) {
                if (readonly()) return;
                const status = form.querySelector('[data-serial-form-status]');
                let profile;
                try {
                    profile = readProfileDraft(form, selectedId, tr);
                    if (!profiles.some(item => item.id === profile.id) && profiles.length >= MAX_PROFILES) throw new Error(tr('desktop.qc_serial_no_profiles'));
                    const next = profiles.filter(item => item.id !== profile.id).concat(profile);
                    const serialized = JSON.stringify({ version: 1, profiles: next });
                    if (new TextEncoder().encode(serialized).byteLength > MAX_PROFILE_BYTES) throw new Error(tr('desktop.qc_serial_save_profile'));
                    const body = await request('/api/desktop/settings', {
                        method: 'PUT',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ key: STORAGE_KEY, value: serialized })
                    });
                    if (disposed) return;
                    profiles = next;
                    selectedId = profile.id;
                    const b = bootstrap();
                    if (b.settings && typeof b.settings === 'object') b.settings[STORAGE_KEY] = serialized;
                    if (status) status.textContent = tr('desktop.qc_serial_saved');
                    renderList();
                    updateButtons();
                    void body;
                } catch (error) {
                    if (status) status.textContent = error && error.message ? error.message : tr('desktop.qc_serial_open_failed');
                }
            }

            async function deleteProfile(id) {
                if (readonly()) return;
                const profile = profiles.find(item => item.id === id);
                if (!profile) return;
                const message = tr('desktop.qc_delete_confirm_msg').replace('{{name}}', profile.name);
                if (!await confirmDialog(tr('desktop.qc_serial_delete_profile'), message)) return;
                if (connection && connection.profileId === id) await disconnect('switch');
                const next = profiles.filter(item => item.id !== id);
                try {
                    const serialized = JSON.stringify({ version: 1, profiles: next });
                    await request('/api/desktop/settings', {
                        method: 'PUT',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ key: STORAGE_KEY, value: serialized })
                    });
                    profiles = next;
                    if (selectedId === id) selectedId = profiles[0] && profiles[0].id || '';
                    renderList();
                    renderEditor();
                    updateButtons();
                } catch (_) {
                    writeStatus(tr('desktop.qc_serial_open_failed'), 'error');
                }
            }

            async function connectProfile(id) {
                if (disposed || chooserPending || readonly()) return;
                const profile = profiles.find(item => item.id === id);
                if (!profile) return;
                if (!allowed(profile.source)) {
                    writeStatus(tr('desktop.qc_serial_permission_disabled'), 'error');
                    return;
                }
                const attempt = ++connectionAttemptGeneration;
                pendingAttemptSource = profile.source;
                if (profile.source === 'browser') {
                    const diagnostic = sourceDiagnostic(profile);
                    if (diagnostic) {
                        pendingAttemptSource = '';
                        writeStatus(diagnostic, 'error');
                        return;
                    }
                    const serial = browserNavigator && browserNavigator.serial;
                    const filters = [];
                    if (profile.usb_vendor_id !== undefined || profile.usb_product_id !== undefined) {
                        const filter = {};
                        if (profile.usb_vendor_id !== undefined) filter.usbVendorId = profile.usb_vendor_id;
                        if (profile.usb_product_id !== undefined) filter.usbProductId = profile.usb_product_id;
                        filters.push(filter);
                    }
                    const chooser = ++chooserGeneration;
                    chooserPending = true;
                    chooserPendingGeneration = chooser;
                    let port;
                    try {
                        // Keep requestPort in the trusted click stack; awaiting it is safe.
                        const choice = serial.requestPort(filters.length ? { filters } : {});
                        port = await choice;
                    } catch (error) {
                        if (attempt === connectionAttemptGeneration) pendingAttemptSource = '';
                        if (chooser === chooserGeneration && !disposed) {
                            const cancelled = error && error.name === 'NotFoundError';
                            writeStatus(tr(cancelled ? 'desktop.qc_serial_cancelled' : error && (error.name === 'NotAllowedError' || error.name === 'SecurityError') ? 'desktop.qc_serial_browser_permission_denied' : 'desktop.qc_serial_open_failed'), cancelled ? 'ready' : 'error');
                        }
                        return;
                    } finally {
                        if (chooserPendingGeneration === chooser) chooserPending = false;
                    }
                    if (disposed || attempt !== connectionAttemptGeneration || chooser !== chooserGeneration || !allowed('browser')) {
                        if (attempt === connectionAttemptGeneration) pendingAttemptSource = '';
                        return;
                    }
                    await startBrowserConnection(profile, port, attempt);
                    if (attempt === connectionAttemptGeneration) pendingAttemptSource = '';
                    return;
                }
                await startHostConnection(profile, attempt);
                if (attempt === connectionAttemptGeneration) pendingAttemptSource = '';
            }

            async function startBrowserConnection(profile, port, attempt) {
                await disconnect('switch');
                if (disposed || attempt !== connectionAttemptGeneration || !allowed('browser')) return;
                const owner = onSessionStart({ source: 'browser', profile });
                const conn = { generation: ++connectionGeneration, source: 'browser', profileId: profile.id, profile, port, owner, isOwnerCurrent: typeof owner === 'function' ? owner : null, reader: null, writer: null, rxMode: 'text', signals: { dtr: profile.options.dtr, rts: profile.options.rts }, pendingTxBytes: 0, sendChain: Promise.resolve(), controlChain: Promise.resolve(), closed: false };
                connection = conn;
                conn.rxMode = receiveMode;
                displaySessionId = conn.generation;
                resetTerminalDisplay();
                startSessionTimers(conn);
                selectedId = profile.id;
                const lineEnding = content && content.querySelector('[data-serial-line-ending]');
                if (lineEnding) lineEnding.value = profile.options.line_ending;
                renderEditor();
                ensureTerminal();
                updateButtons();
                writeStatus(tr('desktop.qc_serial_choose_port'), 'connecting');
                try {
                    const serialOptions = {
                        baudRate: profile.options.baud_rate,
                        dataBits: profile.options.data_bits,
                        stopBits: profile.options.stop_bits,
                        parity: profile.options.parity,
                        bufferSize: 65536,
                        flowControl: profile.options.flow_control
                    };
                    conn.openPromise = Promise.resolve().then(() => port.open(serialOptions)).then(() => { conn.opened = true; });
                    await conn.openPromise;
                    if (!isCurrent(conn)) { await closePort(conn); return; }
                    conn.reader = port.readable.getReader();
                    conn.writer = port.writable.getWriter();
                    const serial = browserNavigator && browserNavigator.serial;
                    if (serial && typeof serial.addEventListener === 'function') {
                        conn.serialDisconnectListener = event => {
                            const removedPort = event && (event.port || (event.target && event.target.port) || event.target);
                            if ((removedPort === conn.port || !removedPort) && isCurrent(conn)) {
                                conn.preserveStatus = true;
                                writeStatus(tr('desktop.qc_serial_connection_lost'), 'error');
                                void stopConnection(conn, 'remote');
                            }
                        };
                        serial.addEventListener('disconnect', conn.serialDisconnectListener);
                        conn.serialEventTarget = serial;
                        if (typeof port.addEventListener === 'function') port.addEventListener('disconnect', conn.serialDisconnectListener);
                    }
                    const signals = { dataTerminalReady: conn.signals.dtr };
                    if (profile.options.flow_control !== 'hardware') signals.requestToSend = conn.signals.rts;
                    await queueBrowserControl(conn, signals);
                    if (!isCurrent(conn)) { await closePort(conn); return; }
                    conn.ready = true;
                    syncSignalControls();
                    writeStatus(tr('desktop.qc_serial_connected'), 'connected');
                    void readBrowser(conn);
                } catch (error) {
                    if (isCurrent(conn)) {
                        const key = error && error.name === 'NotAllowedError' ? 'desktop.qc_serial_browser_permission_denied' : error && error.name === 'NotFoundError' ? 'desktop.qc_serial_unavailable_port' : 'desktop.qc_serial_open_failed';
                        conn.preserveStatus = true;
                        writeStatus(tr(key), 'error');
                        await stopConnection(conn, 'error');
                    } else await closePort(conn);
                }
            }

            async function startHostConnection(profile, attempt) {
                if (!Socket) { writeStatus(tr('desktop.qc_serial_open_failed'), 'error'); return; }
                await disconnect('switch');
                if (disposed || attempt !== connectionAttemptGeneration || !allowed('host')) return;
                const owner = onSessionStart({ source: 'host', profile });
                const conn = { generation: ++connectionGeneration, source: 'host', profileId: profile.id, profile, owner, isOwnerCurrent: typeof owner === 'function' ? owner : null, socket: null, rxMode: 'text', signals: { dtr: profile.options.dtr, rts: profile.options.rts }, pendingTxBytes: 0, sendChain: Promise.resolve(), closed: false };
                connection = conn;
                conn.rxMode = receiveMode;
                displaySessionId = conn.generation;
                resetTerminalDisplay();
                startSessionTimers(conn);
                selectedId = profile.id;
                const lineEnding = content && content.querySelector('[data-serial-line-ending]');
                if (lineEnding) lineEnding.value = profile.options.line_ending;
                renderEditor();
                ensureTerminal();
                updateButtons();
                writeStatus(tr('desktop.qc_serial_choose_port'), 'connecting');
                try {
                    const available = await request('/api/desktop/serial/ports');
                    if (!isCurrent(conn)) return;
                    const port = (available && available.ports || []).find(item => item && item.name === profile.port);
                    if (!port || port.busy) {
                        const error = new Error('');
                        error.code = port ? 'port_busy' : 'port_not_found';
                        throw error;
                    }
                    const currentLocation = ownerWindow.location || window.location;
                    const scheme = currentLocation.protocol === 'https:' ? 'wss:' : 'ws:';
                    const socket = new Socket(`${scheme}//${currentLocation.host}/api/desktop/serial/connect`);
                    socket.binaryType = 'arraybuffer';
                    conn.socket = socket;
                    socket.addEventListener('open', () => {
                        if (!isCurrent(conn)) { try { socket.close(); } catch (_) {} return; }
                        try { socket.send(JSON.stringify({ type: 'open', port: profile.port, options: { ...profile.options } })); }
                        catch (_) { conn.preserveStatus = true; writeStatus(tr('desktop.qc_serial_connection_lost'), 'error'); void stopConnection(conn, 'remote'); }
                    });
                    socket.addEventListener('message', event => { if (isCurrent(conn)) handleHostMessage(conn, event.data); });
                    socket.addEventListener('error', () => { if (isCurrent(conn)) { conn.preserveStatus = true; writeStatus(tr('desktop.qc_serial_connection_lost'), 'error'); void stopConnection(conn, 'remote'); } });
                    socket.addEventListener('close', () => { if (isCurrent(conn)) void stopConnection(conn, 'remote'); });
                } catch (error) {
                    if (isCurrent(conn)) {
                        const code = error && (error.code || error.body && (error.body.code || error.body.error));
                        conn.preserveStatus = true;
                        writeStatus(serialErrorText(code), 'error');
                        await stopConnection(conn, 'error');
                    }
                }
            }

            async function handleHostMessage(conn, data) {
                if (typeof data !== 'string') {
                    const bytes = data instanceof ArrayBuffer ? new Uint8Array(data) : data instanceof Blob ? new Uint8Array(await data.arrayBuffer()) : new Uint8Array(data);
                    if (isCurrent(conn)) receive(conn, bytes);
                    return;
                }
                let message;
                try { message = JSON.parse(data); } catch (_) { return; }
                const status = message.status || message.type;
                if (status === 'connected') {
                    conn.ready = true;
                    writeStatus(tr('desktop.qc_serial_connected'), 'connected');
                    syncSignalControls();
                    if (!conn.deviceTimer) scheduleHostDeviceCheck(conn);
                }
                else if (status === 'disconnected') {
                    conn.preserveStatus = true;
                    writeStatus(serialErrorText(message.code, 'desktop.qc_serial_connection_lost'), 'error');
                    void stopConnection(conn, 'remote');
                } else if (status === 'error') {
                    conn.preserveStatus = true;
                    writeStatus(serialErrorText(message.code), 'error');
                    void stopConnection(conn, 'error');
                }
            }

            function scheduleHostDeviceCheck(conn) {
                conn.deviceTimer = ownerWindow.setTimeout(async () => {
                    conn.deviceTimer = null;
                    if (!isCurrent(conn)) return;
                    try {
                        const body = await request('/api/desktop/serial/ports');
                        if (!isCurrent(conn)) return;
                        const present = Array.isArray(body && body.ports) && body.ports.some(port => port && port.name === conn.profile.port);
                        if (!present) {
                            conn.preserveStatus = true;
                            writeStatus(tr('desktop.qc_serial_unavailable_port'), 'error');
                            await stopConnection(conn, 'remote');
                            return;
                        }
                    } catch (_) {
                        if (!isCurrent(conn)) return;
                    }
                    if (isCurrent(conn)) scheduleHostDeviceCheck(conn);
                }, 15000);
            }

            async function readBrowser(conn) {
                try {
                    while (isCurrent(conn)) {
                        const result = await conn.reader.read();
                        if (!isCurrent(conn) || result.done) break;
                        if (result.value && result.value.byteLength) receive(conn, result.value);
                    }
                    if (isCurrent(conn)) await stopConnection(conn, 'remote');
                } catch (error) {
                    if (isCurrent(conn)) {
                        conn.preserveStatus = true;
                        writeStatus(tr('desktop.qc_serial_connection_lost'), 'error');
                        await stopConnection(conn, 'error');
                    }
                }
            }

            function receive(conn, bytes) {
                if (!isCurrent(conn) || !bytes || !bytes.byteLength) return;
                const copy = Uint8Array.from(bytes);
                addCapture('RX', copy, conn);
                const output = conn.rxMode === 'hex' ? `${formatHex(copy)}\r\n` : copy;
                terminalWrite(output, conn);
            }

            function captureRecord(direction, bytes, conn) {
                addCapture(direction, bytes, conn);
            }

            function addCapture(direction, bytes, conn) {
                const input = Uint8Array.from(bytes || []);
                let data = input;
                if (data.byteLength > MAX_CAPTURE_BYTES) {
                    data = data.slice(data.byteLength - MAX_CAPTURE_BYTES);
                    captureDropped = true;
                }
                const cost = data.byteLength + 32;
                capture.push({ at: Date.now(), direction, session: conn ? conn.generation : displaySessionId, data, cost });
                captureBytes += cost;
                while (capture.length > MAX_CAPTURE_RECORDS || captureBytes > MAX_CAPTURE_BYTES) {
                    const dropped = capture.shift();
                    if (dropped) captureBytes -= dropped.cost;
                    captureDropped = true;
                }
                if (captureDropped) showCaptureNote(tr('desktop.qc_serial_capture_limit'));
            }

            function showCaptureNote(message) {
                const note = content && content.querySelector('[data-serial-capture-note]');
                if (!note) return;
                note.hidden = false;
                note.textContent = message;
            }

            function clearCapture() {
                capture = [];
                captureBytes = 0;
                captureDropped = false;
                resetTerminalDisplay();
                const note = content && content.querySelector('[data-serial-capture-note]');
                if (note) { note.hidden = true; note.textContent = ''; }
            }

            function exportCapture() {
                const rows = capture.map(record => `${new Date(record.at).toISOString()}\t${record.direction}\t${formatHex(record.data)}`);
                const contentText = `${captureDropped ? '# Older capture data was dropped.\n' : ''}${rows.join('\n')}${rows.length ? '\n' : ''}`;
                const blob = new Blob([contentText], { type: 'text/plain;charset=utf-8' });
                const url = URL.createObjectURL(blob);
                const link = document.createElement('a');
                link.href = url;
                link.download = `serial-capture-${new Date().toISOString().replace(/[:.]/g, '-')}.txt`;
                link.click();
                setTimeout(() => URL.revokeObjectURL(url), 0);
            }

            function queueTransmit(conn, bytes, mode, explicitActivity) {
                if (!isCurrent(conn) || !conn.ready || readonly()) return Promise.resolve(false);
                if (!bytes || !bytes.byteLength || bytes.byteLength > MAX_SEND_BYTES) {
                    writeStatus(tr('desktop.qc_serial_invalid_hex'), 'error');
                    return Promise.resolve(false);
                }
                if (conn.pendingTxBytes + bytes.byteLength > MAX_PENDING_TX_BYTES) {
                    writeStatus(tr('desktop.qc_serial_connection_lost'), 'error');
                    return Promise.resolve(false);
                }
                const copy = Uint8Array.from(bytes);
                conn.pendingTxBytes += copy.byteLength;
                conn.sendChain = conn.sendChain.then(async () => {
                    if (!isCurrent(conn) || !conn.ready) return false;
                    if (conn.source === 'browser') {
                        if (!conn.writer) throw new Error('writer_unavailable');
                        await conn.writer.write(copy);
                    } else {
                        if (!conn.socket || conn.socket.readyState !== Socket.OPEN || conn.socket.bufferedAmount + copy.byteLength > MAX_PENDING_TX_BYTES) throw new Error('write_unavailable');
                        conn.socket.send(copy.buffer);
                    }
                    captureRecord('TX', copy, conn);
                    if (!isCurrent(conn)) return true;
                    if (conn.profile.options.local_echo) terminalWrite(mode === 'hex' ? `${formatHex(copy)}\r\n` : copy, conn);
                    if (explicitActivity) touchUserActivity(conn);
                    return true;
                }).catch(() => {
                    if (isCurrent(conn)) writeStatus(tr('desktop.qc_serial_connection_lost'), 'error');
                    return false;
                }).finally(() => { conn.pendingTxBytes = Math.max(0, conn.pendingTxBytes - copy.byteLength); });
                return conn.sendChain;
            }

            async function sendInput() {
                const root = content && content.querySelector('[data-qc-serial-app]');
                const input = root && root.querySelector('[data-serial-input]');
                const conn = connection;
                if (!conn || !conn.ready || !input || readonly()) return;
                const original = input.value;
                const mode = root.querySelector('[data-serial-send-mode]').value;
                let bytes;
                if (mode === 'hex') {
                    bytes = parseHex(original);
                    if (!bytes) { writeStatus(tr('desktop.qc_serial_invalid_hex'), 'error'); return; }
                } else {
                    const endings = { none: '', cr: '\r', lf: '\n', crlf: '\r\n' };
                    const suffix = endings[root.querySelector('[data-serial-line-ending]').value] || '';
                    bytes = new TextEncoder().encode(original + suffix);
                }
                if (!bytes.byteLength || bytes.byteLength > MAX_SEND_BYTES) {
                    writeStatus(tr('desktop.qc_serial_invalid_hex'), 'error');
                    return;
                }
                if (await queueTransmit(conn, bytes, mode, true) && input.value === original) input.value = '';
            }

            async function sendBreak() {
                const conn = connection;
                if (!isCurrent(conn) || !conn.ready) return;
                if (conn.breakTimer) ownerWindow.clearTimeout(conn.breakTimer);
                const generation = conn.breakGeneration = (conn.breakGeneration || 0) + 1;
                try {
                    if (conn.source === 'browser') {
                        await queueBrowserControl(conn, { break: true });
                        if (!isCurrent(conn) || conn.breakGeneration !== generation) return;
                        conn.breakTimer = ownerWindow.setTimeout(async () => {
                            conn.breakTimer = null;
                            if (connection !== conn || conn.closed || conn.breakGeneration !== generation) return;
                            try { await queueBrowserControl(conn, { break: false }); } catch (_) {}
                        }, 250);
                    } else {
                        sendHostControl(conn, { type: 'break' });
                    }
                    touchUserActivity(conn);
                } catch (_) {
                    if (connection === conn) writeStatus(tr('desktop.qc_serial_open_failed'), 'error');
                }
            }

            async function setSignal(signal, value) {
                const conn = connection;
                if (!isCurrent(conn) || !conn.ready) return;
                try {
                    const signals = { ...conn.signals, [signal]: value };
                    if (conn.source === 'browser') {
                        await queueBrowserControl(conn, signal === 'dtr' ? { dataTerminalReady: value } : { requestToSend: value });
                    } else {
                        sendHostControl(conn, { type: 'signals', dtr: signals.dtr, rts: signals.rts });
                    }
                    if (!isCurrent(conn)) return;
                    conn.signals = signals;
                    touchUserActivity(conn);
                } catch (_) {
                    if (connection === conn) writeStatus(tr('desktop.qc_serial_open_failed'), 'error');
                }
            }

            function syncSignalControls() {
                const root = content && content.querySelector('[data-qc-serial-app]');
                if (!root || !connection) return;
                const dtr = root.querySelector('[data-serial-dtr]');
                const rts = root.querySelector('[data-serial-rts]');
                if (dtr) dtr.checked = !!connection.signals.dtr;
                if (rts) rts.checked = !!connection.signals.rts;
                updateButtons();
            }

            async function disconnect(reason) {
                if (reason !== 'switch') {
                    connectionAttemptGeneration++;
                    pendingAttemptSource = '';
                }
                chooserGeneration++;
                const conn = connection;
                if (conn) await stopConnection(conn, reason || 'manual');
                else await connectionStop;
            }

            function stopConnection(conn, reason) {
                if (!conn || connection !== conn) return conn && conn.stopPromise || connectionStop;
                if (conn.stopPromise) return conn.stopPromise;
                connection = null;
                connectionGeneration++;
                conn.closed = true;
                clearSessionTimers(conn);
                updateButtons();
                const stopGeneration = connectionGeneration;
                conn.stopPromise = (async () => {
                    if (conn.source === 'host') await closeHostSocket(conn);
                    else await closePort(conn);
                    updateButtons();
                    if (!disposed && !connection && connectionGeneration === stopGeneration && !conn.preserveStatus) {
                        if (reason === 'remote') writeStatus(tr('desktop.qc_serial_connection_lost'), 'error');
                        else if (reason !== 'error') writeStatus(tr('desktop.qc_serial_disconnected'), 'ready');
                    }
                    if (!conn.sessionEnded) {
                        conn.sessionEnded = true;
                        try { onSessionEnd(conn.owner, reason); } catch (_) {}
                    }
                })();
                connectionStop = conn.stopPromise.catch(() => {});
                return conn.stopPromise;
            }

            function onPolicy(event) {
                policySnapshot = Object.assign({}, getBootstrap() || {}, event && event.detail || {});
                if (pendingAttemptSource && !allowed(pendingAttemptSource)) {
                    connectionAttemptGeneration++;
                    pendingAttemptSource = '';
                    if (chooserPending) chooserGeneration++;
                }
                const source = connection && connection.source;
                if (source && !allowed(source)) void disconnect('policy');
                updateButtons();
            }

            function onAuthEnded() {
                authEnded = true;
                connectionAttemptGeneration++;
                pendingAttemptSource = '';
                chooserGeneration++;
                updateButtons();
                void disconnect('auth');
            }
            function onPageHide() { void disconnect('pagehide'); }

            function dispose() {
                if (disposed) return;
                disposed = true;
                loadGeneration++;
                chooserGeneration++;
                connectionAttemptGeneration++;
                pendingAttemptSource = '';
                portsGeneration++;
                if (portsAbort) portsAbort.abort();
                document.removeEventListener('aurago:desktop-policy', onPolicy);
                document.removeEventListener('aurago:auth-ended', onAuthEnded);
                window.removeEventListener('pagehide', onPageHide);
                displayGeneration++;
                displayQueueBytes = 0;
                void stopConnection(connection, 'dispose');
                disposeTerminal();
                if (list) list.replaceChildren();
                if (content && content.querySelector('[data-qc-serial-app]')) content.replaceChildren();
                mounted = false;
            }

            document.addEventListener('aurago:desktop-policy', onPolicy);
            document.addEventListener('aurago:auth-ended', onAuthEnded);
            window.addEventListener('pagehide', onPageHide);

            return {
                render,
                connectProfile,
                disconnect,
                dispose,
                get connected() { return !!connection; }
            };
        }

        return { create, parseHex, formatHex, parseProfiles, normalizeProfile };
    })();
