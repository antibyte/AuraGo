    // Quick Connect RDP client glue, bundled inside the Desktop shell IIFE
    // (scripts/build-ui-bundles.js). Drives the vendored IronRDP web client
    // (<iron-remote-desktop> + the RDP backend, /js/vendor/ironrdp/) against the
    // RDCleanPath gateway GET /api/desktop/rdp. The server issues a one-time ticket
    // (POST /api/desktop/rdp/ticket) that doubles as the gateway auth token.
    // renderQuickConnect in quickconnect-launchpad-chat.js owns the session bar and
    // placeholders; this file owns the library, the config and the error mapping.
    // Credentials live only in local variables: never logged, never put in the DOM.
    window.QuickConnectRDP = (() => {
        const CORE_URL = '/js/vendor/ironrdp/iron-remote-desktop.js';
        const BACKEND_URL = '/js/vendor/ironrdp/iron-remote-desktop-rdp.js';
        const READY_TIMEOUT_MS = 15000;
        const MIN_WIDTH = 640;
        const MIN_HEIGHT = 480;
        const MAX_SIDE = 8192;
        // IronErrorKind values of @devolutions/iron-remote-desktop.
        const KIND_KEYS = {
            0: 'desktop.qc_rdp_error_general',
            1: 'desktop.qc_rdp_error_wrong_password',
            2: 'desktop.qc_rdp_error_logon_failure',
            3: 'desktop.qc_rdp_error_access_denied',
            4: 'desktop.qc_rdp_error_gateway',
            5: 'desktop.qc_rdp_error_proxy',
            6: 'desktop.qc_rdp_error_negotiation'
        };
        // Gateway code for "server certificate differs from the pinned one".
        const CERT_CHANGED_CODE = 16384;
        const CERT_CHANGED_KEY = 'desktop.qc_rdp_error_cert_changed';
        const SCALES = { fit: 'fit', full: 'full', real: 'real', native: 'real' };
        let libraryPromise = null;

        // Imports both modules once and initialises the wasm backend once. A failed
        // load is not cached so a later Reconnect can retry.
        function preload() {
            if (!libraryPromise) {
                libraryPromise = (async () => {
                    await import(CORE_URL);
                    const backend = await import(BACKEND_URL);
                    await backend.init('WARN');
                    return backend;
                })();
                libraryPromise.catch(() => { libraryPromise = null; });
            }
            return libraryPromise;
        }

        // Windows rejects bad NLA credentials by hanging up: the client then reports
        // a General error whose text mentions the WebSocket closing.
        function mentionsSocketClose(error) {
            let text = '';
            try { text += ' ' + String(error.message || ''); } catch (_) {}
            try { if (typeof error.backtrace === 'function') text += ' ' + String(error.backtrace() || ''); } catch (_) {}
            try { text += ' ' + String(error); } catch (_) {}
            return /web\s?socket[\s\S]{0,60}clos|clos[\s\S]{0,60}web\s?socket/i.test(text);
        }

        // `connected` tells whether the session ever reached the connected state.
        function rdpErrorKey(error, connected) {
            if (!error || typeof error.kind !== 'function') return 'desktop.qc_rdp_error_general';
            let kind = 0;
            try { kind = error.kind(); } catch (_) { return 'desktop.qc_rdp_error_general'; }
            if (kind === 0 && !connected && mentionsSocketClose(error)) return 'desktop.qc_rdp_error_remote_closed';
            if (kind !== 4) return KIND_KEYS[kind] || KIND_KEYS[0];
            let details = null;
            try { details = typeof error.rdcleanpathDetails === 'function' ? error.rdcleanpathDetails() : null; } catch (_) { details = null; }
            const status = details && details.httpStatusCode;
            const code = details && details.errorCode;
            if (code === CERT_CHANGED_CODE || status === 409) return CERT_CHANGED_KEY;
            if (status === 401 || status === 403 || code === 100) return 'desktop.qc_rdp_error_ticket';
            if (code === 103) return 'desktop.qc_rdp_error_unreachable';
            if (code === 104) return 'desktop.qc_rdp_error_dns';
            if (code === 105) return 'desktop.qc_rdp_error_tls';
            return KIND_KEYS[4];
        }

        // First line of the IronRDP error text (backtrace or message), trimmed. Library
        // errors never carry the password; this is shown next to the localized message.
        function errorDetail(error) {
            let text = '';
            try { if (error && typeof error.backtrace === 'function') text = String(error.backtrace() || ''); } catch (_) {}
            if (!text.trim()) { try { text = String((error && error.message) || error || ''); } catch (_) {} }
            const line = text.split(/[\x0d\x0a]+/).map(part => part.trim()).find(Boolean) || '';
            return line.length > 200 ? line.slice(0, 200) : line;
        }

        function failWith(fail, error, connected) {
            const key = rdpErrorKey(error, connected);
            const detail = errorDetail(error);
            console.warn('[QuickConnect RDP]', key, detail);
            fail(key, detail);
        }

        function desktopSize(container) {
            const clamp = (value, min) => {
                const rounded = Math.min(MAX_SIDE, Math.max(min, Math.round(value || 0)));
                return rounded - (rounded % 2);
            };
            return { width: clamp(container.clientWidth, MIN_WIDTH), height: clamp(container.clientHeight, MIN_HEIGHT) };
        }

        function waitForReady(element) {
            return new Promise((resolve, reject) => {
                const timer = setTimeout(() => reject(new Error('ready timeout')), READY_TIMEOUT_MS);
                element.addEventListener('ready', event => {
                    clearTimeout(timer);
                    const ui = event && event.detail && event.detail.irgUserInteraction;
                    if (ui) resolve(ui); else reject(new Error('missing user interaction'));
                }, { once: true });
            });
        }

        // Starts a session inside `container`. Returns the handle at once; the
        // outcome is reported through callbacks. Exactly one of onError / onClose
        // fires, once, unless shutdown() ran first (then neither does).
        function connect(container, opts) {
            const options = opts || {};
            const callbacks = {
                status: typeof options.onStatus === 'function' ? options.onStatus : () => {},
                error: typeof options.onError === 'function' ? options.onError : () => {},
                close: typeof options.onClose === 'function' ? options.onClose : () => {}
            };
            let element = null;
            let ui = null;
            let ended = false;
            let observer = null;
            let resizeTimer = 0;
            let lastSize = null;
            let connected = false;

            const teardown = () => {
                clearTimeout(resizeTimer);
                if (observer) { observer.disconnect(); observer = null; }
                const current = ui;
                ui = null;
                if (current) { try { current.shutdown(); } catch (_) {} }
                if (element) { element.remove(); element = null; }
            };
            const finish = (fire) => {
                if (ended) return;
                ended = true;
                teardown();
                fire();
            };
            const fail = (key, detail) => finish(() => { callbacks.status('error'); callbacks.error(key, detail || ''); });

            const handle = {
                scale(mode) {
                    const value = SCALES[mode];
                    if (value && element) element.scale = value;
                },
                ctrlAltDel() { if (ui) { try { ui.ctrlAltDel(); } catch (_) {} } },
                meta() { if (ui) { try { ui.metaKey(); } catch (_) {} } },
                shutdown() {
                    ended = true;
                    teardown();
                }
            };

            (async () => {
                let password = options.password || '';
                options.password = '';
                const ticket = options.ticket || '';
                options.ticket = '';
                callbacks.status('connecting');
                let backend;
                try {
                    backend = await preload();
                } catch (_) {
                    password = '';
                    fail('desktop.qc_rdp_error_unavailable');
                    return;
                }
                if (ended) { password = ''; return; }
                try {
                    element = document.createElement('iron-remote-desktop');
                    element.style.cssText = 'display:block;width:100%;height:100%';
                    element.module = backend.Backend;
                    element.scale = 'fit';
                    element.flexcenter = 'true';
                    const ready = waitForReady(element);
                    container.appendChild(element);
                    ui = await ready;
                    if (ended) { password = ''; teardown(); return; }
                    const size = desktopSize(container);
                    lastSize = size;
                    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
                    const proxy = proto + '//' + location.host + '/api/desktop/rdp?device_id=' + encodeURIComponent(options.deviceId || '');
                    let builder = ui.configBuilder()
                        .withUsername(options.username || '')
                        .withPassword(password)
                        .withDestination((options.host || '') + ':' + (options.port || 3389))
                        .withProxyAddress(proxy)
                        .withAuthToken(ticket)
                        .withDesktopSize(size)
                        .withExtension(backend.displayControl(true));
                    if (options.domain) builder = builder.withServerDomain(options.domain);
                    const config = builder.build();
                    password = '';
                    ui.setVisibility(true);
                    const info = await ui.connect(config);
                    if (ended) { teardown(); return; }
                    connected = true;
                    callbacks.status('connected');
                    if (typeof ResizeObserver === 'function') {
                        observer = new ResizeObserver(() => {
                            clearTimeout(resizeTimer);
                            resizeTimer = setTimeout(() => {
                                if (ended || !ui) return;
                                const next = desktopSize(container);
                                if (lastSize && Math.abs(next.width - lastSize.width) < 4 && Math.abs(next.height - lastSize.height) < 4) return;
                                lastSize = next;
                                try { ui.resize(next.width, next.height); } catch (_) {}
                            }, 400);
                        });
                        observer.observe(container);
                    }
                    let reason = '';
                    try {
                        const termination = await info.run();
                        reason = termination && typeof termination.reason === 'function' ? String(termination.reason() || '') : '';
                    } catch (error) {
                        if (!ended) failWith(fail, error, connected);
                        return;
                    }
                    finish(() => { callbacks.status('disconnected'); callbacks.close(reason); });
                } catch (error) {
                    password = '';
                    if (!ended) failWith(fail, error, connected);
                }
            })();
            return handle;
        }

        function toolbarMarkup(t) {
            return `<div class="vd-qc-vnc-toolbar" data-rdp-toolbar>
                <span class="vd-qc-vnc-status" data-rdp-status data-state="connecting">${esc(t('desktop.qc_rdp_status_connecting'))}</span>
                <div class="vd-qc-vnc-tools" role="group" aria-label="${esc(t('desktop.qc_rdp_toolbar_label'))}">
                    <button class="vd-qc-btn vd-qc-btn-sm active" type="button" data-rdp-scale="fit" title="${esc(t('desktop.qc_vnc_scale_fit'))}">${iconMarkup('maximize', 'F', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.qc_vnc_scale_fit'))}</span></button>
                    <button class="vd-qc-btn vd-qc-btn-sm" type="button" data-rdp-scale="real" title="${esc(t('desktop.qc_vnc_scale_native'))}">${iconMarkup('monitor', 'N', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.qc_vnc_scale_native'))}</span></button>
                    <button class="vd-qc-btn vd-qc-btn-sm" type="button" data-rdp-action="ctrl-alt-del" title="${esc(t('desktop.qc_vnc_ctrl_alt_del'))}">${iconMarkup('keyboard', 'K', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.qc_vnc_ctrl_alt_del'))}</span></button>
                    <button class="vd-qc-btn vd-qc-btn-sm" type="button" data-rdp-action="meta" title="${esc(t('desktop.qc_rdp_win_key'))}">${iconMarkup('layout-grid', 'W', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.qc_rdp_win_key'))}</span></button>
                    <button class="vd-qc-btn vd-qc-btn-sm" type="button" data-rdp-action="reconnect" title="${esc(t('desktop.qc_reconnect'))}">${iconMarkup('refresh', 'R', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.qc_reconnect'))}</span></button>
                </div>
            </div>`;
        }

        // getSession returns the live handle (or null while connecting).
        function wireToolbar(root, getSession, onReconnect) {
            root.querySelectorAll('[data-rdp-scale]').forEach(btn => {
                btn.addEventListener('click', () => {
                    const session = getSession();
                    if (!session) return;
                    root.querySelectorAll('[data-rdp-scale]').forEach(item => item.classList.toggle('active', item === btn));
                    session.scale(btn.dataset.rdpScale);
                });
            });
            const bind = (action, run) => {
                const btn = root.querySelector('[data-rdp-action="' + action + '"]');
                if (btn) btn.addEventListener('click', run);
            };
            bind('ctrl-alt-del', () => { const session = getSession(); if (session) session.ctrlAltDel(); });
            bind('meta', () => { const session = getSession(); if (session) session.meta(); });
            bind('reconnect', onReconnect);
        }

        function setStatus(root, state, t) {
            const status = root && root.querySelector ? root.querySelector('[data-rdp-status]') : null;
            if (!status) return;
            const known = ['connecting', 'connected', 'disconnected', 'error'].includes(state) ? state : 'error';
            status.dataset.state = known;
            status.textContent = t('desktop.qc_rdp_status_' + known);
        }

        // Masked password prompt for devices without a stored password. Resolves with
        // the typed password, or null on Esc/Cancel. `track(cancel)` registers a
        // canceller (window close, device switch) and returns an unregister function.
        function promptPassword(opts) {
            const { root, t, deviceName, track } = opts;
            return new Promise(resolve => {
                const overlay = document.createElement('div');
                overlay.className = 'vd-qc-modal-overlay';
                overlay.innerHTML = `<div class="vd-qc-confirm">
                    <div class="vd-qc-confirm-title">${esc(t('desktop.qc_rdp_password_title'))}</div>
                    <div class="vd-qc-confirm-msg">${esc(deviceName || '')}</div>
                    <label class="vd-qc-label">${esc(t('desktop.qc_rdp_password_label'))}
                        <input class="vd-qc-input" type="password" name="rdp-password" autocomplete="off" spellcheck="false">
                    </label>
                    <div class="vd-qc-confirm-actions">
                        <button class="vd-qc-btn vd-qc-btn-secondary" type="button" data-action="cancel">${iconMarkup('x', 'X', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.cancel'))}</span></button>
                        <button class="vd-qc-btn vd-qc-btn-primary" type="button" data-action="ok">${iconMarkup('monitor', 'C', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.qc_rdp_password_connect'))}</span></button>
                    </div>
                </div>`;
                root.appendChild(overlay);
                const input = overlay.querySelector('input[name="rdp-password"]');
                const cancel = () => finish(null);
                const untrack = typeof track === 'function' ? track(cancel) : () => {};
                const release = QuickConnectConsole.openDialog(overlay, { onCancel: cancel, initialFocus: input });
                let done = false;
                function finish(value) {
                    if (done) return;
                    done = true;
                    untrack();
                    release();
                    overlay.remove();
                    resolve(value);
                }
                overlay.querySelector('[data-action="cancel"]').addEventListener('click', cancel);
                overlay.querySelector('[data-action="ok"]').addEventListener('click', () => finish(input.value));
                input.addEventListener('keydown', event => {
                    if (event.key === 'Enter' && !event.repeat && !event.isComposing) {
                        event.preventDefault();
                        finish(input.value);
                    }
                });
            });
        }

        // Adds the "Trust new certificate" action to an error placeholder shown for
        // a changed server certificate. The reset only happens after the user
        // confirms in the app's own confirm dialog; then the session reconnects.
        // opts: { box, t, deviceId, confirm(title, message, options), post(url), notify(text), onTrusted() }
        function addTrustAction(opts) {
            const { box, t, deviceId, confirm, post, notify, onTrusted } = opts;
            const actions = box && box.querySelector ? box.querySelector('.vd-qc-reconnect-actions') : null;
            if (!actions) return;
            const button = document.createElement('button');
            button.className = 'vd-qc-btn vd-qc-btn-secondary vd-qc-reconnect-btn';
            button.type = 'button';
            button.dataset.action = 'trust-new-cert';
            button.innerHTML = `${iconMarkup('save', 'T', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.qc_rdp_trust_new_cert'))}</span>`;
            button.addEventListener('click', async () => {
                const ok = await confirm(t('desktop.qc_rdp_trust_new_cert'), t(CERT_CHANGED_KEY), {
                    okLabel: t('desktop.qc_rdp_trust_new_cert'), okIcon: 'save', okClass: 'vd-qc-btn-primary'
                });
                if (!ok) return;
                try {
                    await post('/api/desktop/rdp/pin/reset?device_id=' + encodeURIComponent(deviceId));
                } catch (err) {
                    notify(t('desktop.qc_rdp_error_ticket_failed') + ': ' + (err && err.message || ''));
                    return;
                }
                onTrusted();
            });
            actions.insertBefore(button, actions.firstChild);
        }

        return { preload, connect, rdpErrorKey, toolbarMarkup, wireToolbar, setStatus, promptPassword, addTrustAction };
    })();
