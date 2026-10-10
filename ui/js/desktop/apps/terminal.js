(function () {
    'use strict';

    const instances = new Map();
    const LAST_ENTRY_KEY = 'aurago.desktop.terminal.retronet.last';
    const HANGUP_KEY = '\x1d';
    // C64 sessions (Contract C): black screen, cursor and border in C64 light blue (colour 14, the power-on text).
    const C64_THEME = { background: '#000000', cursor: '#706DEB', cursorAccent: '#000000' };
    const C64_BORDER = '#706DEB';
    // Written once, right before the first board byte of a C64 call: the server's PETSCII engine owns the 40x25
    // screen from then on and its model starts blank with the cursor at row 25, column 1. The coordinator writes its
    // own lines (dial, CONNECT, key hint) only before this point and nothing during the call.
    const C64_FRESH_SCREEN = '\x1b[0m\x1b[2J\x1b[25;1H';
    // Normal screen buffer, then DECSTR (attributes, cursor, scroll region, wrap, insert, focus and paste
    // reporting) and mouse reporting off: after a session only real keys reach the result prompt.
    const PLAIN_SCREEN = '\x1b[?1047l\x1b[!p\x1b[?1000l\x1b[?1002l\x1b[?1003l\x1b[?1006l';
    // Keys typed while a service hangs up must not dismiss its result before it can be read.
    const RESULT_GRACE_MS = 400;
    const RESULT_REASONS = [
        'refused', 'limit', 'timeout', 'dns', 'blocked', 'remote_closed', 'idle', 'max_duration',
        'disabled', 'hostkey_mismatch', 'hostkey_rejected', 'server_shutdown', 'local_hangup', 'lost'
    ];

    function escapeHTML(value) {
        return String(value == null ? '' : value).replace(/[&<>"']/g, function (m) {
            return ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[m];
        });
    }

    function styleLabel(ctx, id) {
        const key = ({
            modern: 'desktop.terminal_style_modern',
            amber: 'desktop.terminal_style_amber',
            green: 'desktop.terminal_style_green',
            apple2: 'desktop.terminal_style_apple2',
            commodore64: 'desktop.terminal_style_commodore64',
            ibm3278: 'desktop.terminal_style_ibm3278',
            vintage: 'desktop.terminal_style_vintage',
            'mono-green': 'desktop.terminal_style_mono_green',
            'transparent-green': 'desktop.terminal_style_transparent_green'
        })[id] || 'desktop.terminal_style_modern';
        return ctx.t(key);
    }

    function optionMarkup(ctx) {
        const esc = typeof ctx.esc === 'function' ? ctx.esc : escapeHTML;
        return window.TerminalStyles.ids.map(function (id) {
            return '<option value="' + id + '">' + esc(styleLabel(ctx, id)) + '</option>';
        }).join('');
    }

    // Retro-Net needs every module of Contracts C and E, including the shared TerminalText helpers and the geometry.
    function retroNetModulesLoaded() {
        return !!(window.TerminalText && window.TerminalModem && window.TerminalRetroNetDirectory &&
            window.TerminalRetroNetSession && window.TerminalRetroNetEntries && window.TerminalGeometry);
    }

    // Interpolates {{name}} itself: ctx.t() is called without params, so `$` in server text (host-key type and
    // fingerprint) is never a replacement pattern of the shared t().
    function translate(ctx, key, params) {
        let text = String(ctx.t(key));
        Object.keys(params || {}).forEach(function (name) {
            text = text.split('{{' + name + '}}').join(String(params[name]));
        });
        return text;
    }

    // Text the coordinator writes itself reaches xterm without control characters (Retro-Net only,
    // so the shared TerminalText module is loaded whenever this runs).
    function plain(text) {
        return window.TerminalText.printable(text);
    }

    function saveLastEntry(id) {
        try {
            window.localStorage.setItem(LAST_ENTRY_KEY, String(id));
        } catch (e) {}
    }

    function render(host, windowId, ctx) {
        if (!host) return;
        dispose(windowId);
        const styles = window.TerminalStyles;
        const esc = typeof ctx.esc === 'function' ? ctx.esc : escapeHTML;
        const tr = function (key, params) { return translate(ctx, key, params); };
        const getBootstrap = typeof ctx.getBootstrap === 'function' ? ctx.getBootstrap : function () { return {}; };
        const retroNetReady = retroNetModulesLoaded();
        const initial = styles ? styles.load() : 'modern';
        const muted = window.TerminalAudio ? window.TerminalAudio.loadMuted() : false;
        const effectMarkup = styles.effectControls.map(function (effect, index) {
            const heading = index === 0 ? 'image' : index === 8 ? 'glass' : '';
            return (heading ? '<h3>' + ctx.t('desktop.terminal_effects_' + heading) + '</h3>' : '') +
                '<label class="vd-terminal-effect"><span>' + ctx.t('desktop.terminal_effects_' + effect.key) + '</span>' +
                '<output aria-hidden="true" data-terminal-effect-output="' + effect.key + '"></output>' +
                '<input type="range" data-terminal-effect="' + effect.key + '" min="' + (effect.min || 0) +
                '" max="' + (effect.max || 100) + '" step="1"></label>';
        }).join('');
        const baudMarkup = retroNetReady ? window.TerminalModem.BAUD_RATES.map(function (rate) {
            const label = rate ? tr('desktop.terminal_retronet_baud_rate', { rate: rate }) : tr('desktop.terminal_retronet_baud_off');
            return '<option value="' + rate + '">' + esc(label) + '</option>';
        }).join('') : '';
        host.innerHTML = '<div class="vd-terminal-app" data-terminal-state="desktop.loading" data-terminal-mode="shell" data-terminal-style="' + initial + '">' +
            '<div class="vd-terminal-toolbar">' +
            '<span class="vd-terminal-status vd-terminal-toolbar-status" data-terminal-status role="status">' + ctx.t('desktop.loading') + '</span>' +
            '<div class="vd-terminal-toolbar-actions">' +
            '<button type="button" data-terminal-retronet-action="directory" hidden></button>' +
            '<label class="vd-terminal-baud-label" data-terminal-baud-label hidden>' +
            '<span class="vd-sr-only">' + esc(tr('desktop.terminal_retronet_baud')) + '</span>' +
            '<select data-terminal-baud aria-label="' + esc(tr('desktop.terminal_retronet_baud')) + '">' + baudMarkup + '</select>' +
            '</label>' +
            '<label class="vd-terminal-style-label">' +
            '<span class="vd-sr-only">' + ctx.t('desktop.terminal_style') + '</span>' +
            '<select data-terminal-style aria-label="' + ctx.t('desktop.terminal_style') + '">' + optionMarkup(ctx) + '</select>' +
            '</label>' +
            '<button type="button" data-terminal-effects aria-haspopup="dialog">' + ctx.t('desktop.terminal_effects') + '</button>' +
            '<button type="button" data-terminal-audio aria-pressed="' + (muted ? 'false' : 'true') + '">' +
            ctx.t(muted ? 'desktop.terminal_audio_off' : 'desktop.terminal_audio_on') +
            '</button>' +
            '</div></div>' +
            '<div class="vd-sr-only" data-terminal-announce aria-live="polite" aria-atomic="true"></div>' +
            '<div class="vd-terminal-stage">' +
            '<div class="vd-terminal-bezel" data-terminal-bezel>' +
            '<div class="vd-terminal-screen" data-terminal-screen><div class="vd-terminal-reflection" aria-hidden="true"></div></div>' +
            '<div class="vd-terminal-hardware" aria-hidden="true">' +
            '<span class="vd-terminal-maker">AuraGo</span>' +
            '<span class="vd-terminal-vents"></span>' +
            '<span class="vd-terminal-power"><span class="vd-terminal-led"></span><span>⏻</span></span>' +
            '</div>' +
            '</div></div>' +
            '<dialog class="vd-terminal-effects-dialog" data-terminal-effects-dialog aria-label="' + ctx.t('desktop.terminal_effects') + '">' +
            '<header><div><h2>' + ctx.t('desktop.terminal_effects') + '</h2><p data-terminal-effects-style></p></div>' +
            '<button type="button" data-terminal-effects-close aria-label="' + ctx.t('desktop.close') + '">×</button></header>' +
            '<p data-terminal-effects-retro hidden>' + ctx.t('desktop.terminal_effects_retro_hint') + '</p>' +
            '<p data-terminal-effects-fallback hidden>' + ctx.t('desktop.terminal_effects_fallback_hint') + '</p>' +
            '<p data-terminal-effects-motion hidden>' + ctx.t('desktop.terminal_effects_motion_hint') + '</p>' +
            '<fieldset class="vd-terminal-effects-grid">' + effectMarkup + '</fieldset>' +
            '<footer><p>' + ctx.t('desktop.terminal_effects_saved_hint') + '</p>' +
            '<button type="button" data-terminal-effects-reset>' + ctx.t('desktop.terminal_effects_reset') + '</button></footer></dialog></div>';

        const root = host.querySelector('.vd-terminal-app');
        const screen = host.querySelector('[data-terminal-screen]');
        const status = host.querySelector('[data-terminal-status]');
        const select = host.querySelector('select[data-terminal-style]');
        const styleBox = host.querySelector('.vd-terminal-style-label');
        const audioBtn = host.querySelector('[data-terminal-audio]');
        const effectsBtn = host.querySelector('[data-terminal-effects]');
        const effectsDialog = host.querySelector('[data-terminal-effects-dialog]');
        const retroBtn = host.querySelector('[data-terminal-retronet-action]');
        const baudLabel = host.querySelector('[data-terminal-baud-label]');
        const baudSelect = host.querySelector('select[data-terminal-baud]');
        const announcer = host.querySelector('[data-terminal-announce]');
        const motionQuery = window.matchMedia('(prefers-reduced-motion: reduce)');
        let effectValues = styles.loadEffects(initial);
        let ws = null;
        let term = null;
        let fit = null;
        let canvasAddon = null;
        let crt = null;
        let audio = window.TerminalAudio ? window.TerminalAudio.create() : null;
        let observer = null;
        let styleRevision = 0;
        let mode = 'shell';
        let retroNetOn = false;
        let directory = null;
        let modem = null;
        let retro = null;
        let geometry = null;
        let c64Look = null;
        let resultAt = 0;
        const inst = { root: host };

        if (select) select.value = initial;
        if (baudSelect && retroNetReady) baudSelect.value = String(window.TerminalModem.loadBaud());
        if (audio) audio.setMuted(muted);

        function setStatus(key) {
            root.setAttribute('data-terminal-state', key);
            if (status) status.textContent = ctx.t(key);
        }

        function currentProfile() {
            return styles.profile(root.getAttribute('data-terminal-style'), effectValues);
        }

        // What the CRT paints around the grid: the C64 border colour while the C64 look is on.
        function crtProfile(profile) {
            if (!c64Look) return profile;
            return Object.assign({}, profile, { theme: Object.assign({}, profile.theme, { background: C64_BORDER }) });
        }

        // The fit of the current geometry: the fit addon, or the pinned grid's font fit.
        function refitScreen() {
            if (geometry) geometry.refit();
            else if (fit && typeof fit.fit === 'function') fit.fit();
        }

        function syncEffectsMenu() {
            const profile = currentProfile();
            const fallback = root.getAttribute('data-terminal-fallback') === 'css';
            effectsDialog.querySelector('[data-terminal-effects-style]').textContent = styleLabel(ctx, profile.id);
            effectsDialog.querySelector('fieldset').disabled = !profile.retro;
            effectsDialog.querySelector('[data-terminal-effects-reset]').disabled = !profile.retro;
            effectsDialog.querySelector('[data-terminal-effects-retro]').hidden = profile.retro;
            effectsDialog.querySelector('[data-terminal-effects-fallback]').hidden = !profile.retro || !fallback;
            effectsDialog.querySelector('[data-terminal-effects-motion]').hidden = !profile.retro || !(motionQuery.matches || document.body.dataset.animations === 'false');
            styles.effectControls.forEach(function (effect) {
                const input = effectsDialog.querySelector('[data-terminal-effect="' + effect.key + '"]');
                input.value = effectValues[effect.key];
                input.setAttribute('aria-valuetext', effectValues[effect.key] + '%');
                input.disabled = fallback && !!effect.advanced;
                effectsDialog.querySelector('[data-terminal-effect-output="' + effect.key + '"]').textContent = effectValues[effect.key] + '%';
            });
        }

        function applyEffects() {
            const profile = currentProfile();
            const c = profile.crt;
            const strength = profile.retro ? c.intensity : 0;
            root.style.setProperty('--vd-term-reflection', c.reflection * strength);
            root.style.setProperty('--vd-term-brightness', 1 + (c.brightness - 1) * strength);
            root.style.setProperty('--vd-term-bloom', c.bloom * strength * 4 + 'px');
            root.style.setProperty('--vd-term-bloom-color', c.bloom * strength ? 'var(--vd-term-ink)' : 'transparent');
            root.style.setProperty('--vd-term-scan', c.scan * strength * 0.23);
            root.style.setProperty('--vd-term-vignette', c.vignette * strength);
            if (crt) crt.setProfile(crtProfile(profile));
            syncEffectsMenu();
        }

        function syncAudioButton() {
            if (!audioBtn) return;
            const profile = currentProfile();
            const isMuted = window.TerminalAudio ? window.TerminalAudio.loadMuted() : true;
            audioBtn.hidden = !profile.retro;
            audioBtn.setAttribute('aria-pressed', profile.retro && !isMuted ? 'true' : 'false');
            audioBtn.textContent = ctx.t(isMuted ? 'desktop.terminal_audio_off' : 'desktop.terminal_audio_on');
        }

        function ensureCanvas(enable) {
            if (!term || !window.WebglAddon || !window.WebglAddon.WebglAddon) return;
            if (enable && !canvasAddon) {
                try {
                    canvasAddon = new window.WebglAddon.WebglAddon(true);
                    term.loadAddon(canvasAddon);
                    canvasAddon.onContextLoss(() => {
                        if (canvasAddon) canvasAddon.dispose();
                        canvasAddon = null;
                        root.setAttribute('data-terminal-fallback', 'css');
                    });
                } catch (e) {
                    canvasAddon = null;
                }
            }
            if (!enable && canvasAddon && typeof canvasAddon.dispose === 'function') {
                try { canvasAddon.dispose(); } catch (e) {}
                canvasAddon = null;
            }
        }

        // persist false: the temporary C64 look and its undo never change the saved style.
        async function applyStyle(id, persist) {
            const revision = ++styleRevision;
            const next = persist === false ? styles.normalize(id) : styles.save(id);
            effectValues = styles.loadEffects(next);
            if (select) select.value = next;
            root.setAttribute('data-terminal-style', next);
            root.removeAttribute('data-terminal-fallback');
            let profile = currentProfile();
            if (document.fonts && typeof document.fonts.load === 'function') {
                const family = String(profile.fontFamily || '').split(',')[0].trim() || 'monospace';
                const spec = String(profile.fontSize || 13) + 'px ' + family;
                if (typeof document.fonts.check !== 'function' || !document.fonts.check(spec)) {
                    try { await document.fonts.load(spec); } catch (_) { /* Keep the fallback font usable. */ }
                }
            }
            if (!term || revision !== styleRevision) return;
            profile = currentProfile();
            styles.applyXterm(term, profile);
            if (c64Look) term.options.theme = Object.assign({}, profile.theme, C64_THEME);
            // A pinned grid gets its font back at once; the fit itself follows below.
            if (geometry && geometry.mode() !== 'fit') geometry.refit();
            ensureCanvas(profile.retro);
            if (profile.retro) {
                if (!crt && window.TerminalCrt) {
                    crt = window.TerminalCrt.create({
                        host: root,
                        screen: screen,
                        term: term,
                        getWindowEl: function () { return host.closest('.vd-window'); }
                    });
                }
                if (crt) {
                    crt.setProfile(crtProfile(profile));
                    crt.setEnabled(true);
                    crt.resize();
                    if (typeof crt.usesFallback === 'function' && crt.usesFallback()) {
                        root.setAttribute('data-terminal-fallback', 'css');
                    }
                }
            } else if (crt) {
                crt.setEnabled(false);
                crt.dispose();
                crt = null;
                root.removeAttribute('data-terminal-fallback');
            }
            if (audio) audio.setProfile(profile);
            applyEffects();
            syncAudioButton();
            refitScreen();
            if (crt && typeof crt.resize === 'function') crt.resize();
        }

        function onKeyDown(event) {
            if (!audio || effectsDialog.open || !host.contains(document.activeElement)) return;
            if (document.activeElement.closest && document.activeElement.closest('dialog')) return;
            audio.playKey(event);
        }

        // ---- Retro-Net coordination: exactly one live socket (shell or Retro-Net) ----

        function retroNetAllowed(overrides) {
            const b = Object.assign({}, getBootstrap() || {}, overrides || {});
            return retroNetReady && b.retronet_enabled === true && b.readonly !== true && b.enabled !== false;
        }

        function syncRetroToolbar() {
            const busy = mode === 'dialing' || mode === 'retro';
            if (baudLabel) baudLabel.hidden = !retroNetOn;
            if (!retroBtn) return;
            retroBtn.hidden = mode === 'directory' || (!retroNetOn && !busy);
            retroBtn.setAttribute('data-terminal-retronet-action', busy ? 'hangup' : 'directory');
            retroBtn.textContent = busy ? tr('desktop.terminal_retronet_hangup') : tr('desktop.terminal_retronet_directory');
        }

        function setMode(next) {
            mode = next;
            root.setAttribute('data-terminal-mode', next);
            syncRetroToolbar();
        }

        function announce(text) {
            if (announcer) announcer.textContent = String(text || '');
        }

        function writeDim(text) {
            if (term) term.write('\x1b[2m' + plain(text) + '\x1b[0m\r\n');
        }

        // term.reset() acts at once, but output written before it is still queued and would land on the
        // fresh screen (the directory's hidden cursor, wrap off, alternate screen). RIS in the write queue
        // resets after it; xterm keeps the cursor hidden across resets, so it is shown explicitly.
        function resetScreen() {
            if (!term) return;
            term.reset();
            term.write('\x1bc\x1b[?25h');
        }

        function closeShell() {
            const sock = ws;
            ws = null;
            if (!sock) return;
            sock.onopen = null;
            sock.onmessage = null;
            sock.onerror = null;
            sock.onclose = null;
            if (sock.readyState !== WebSocket.CLOSED) {
                try { sock.close(); } catch (e) {}
            }
        }

        // Today's Code Studio shell socket; the only WebSocket this file creates.
        function openShell() {
            leaveDirectory();
            stopRetro();
            setGeometry('fit');
            closeShell();
            if (!term) return;
            term.options.convertEol = true;
            setMode('shell');
            setStatus('desktop.loading');
            const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
            const sock = new WebSocket(protocol + '//' + location.host + '/api/code-studio/terminal');
            ws = sock;
            sock.binaryType = 'arraybuffer';
            sock.onopen = function () {
                if (ws === sock) setStatus('desktop.terminal_running');
            };
            sock.onmessage = function (event) {
                if (ws !== sock || !term) return;
                if (event.data instanceof ArrayBuffer) term.write(new Uint8Array(event.data));
                else term.write(String(event.data));
            };
            sock.onerror = function () {
                if (ws === sock) setStatus('desktop.terminal_unavailable');
            };
            sock.onclose = function () {
                if (ws !== sock) return;
                setStatus('desktop.terminal_stopped');
                // With Retro-Net on, the ended shell waits for a key and returns to the directory.
                if (retroNetOn && term && mode === 'shell') {
                    term.write(PLAIN_SCREEN + '\r\n');
                    writeDim(tr('desktop.terminal_retronet_press_key'));
                    announce(plain(tr('desktop.terminal_retronet_press_key')));
                    enterResult();
                }
            };
        }

        // Own-entry dialogs belong to the directory: removed, not closed (close() reopens a dialog whose save
        // is pending), whenever the directory goes away, e.g. after HTTP 403 or a policy switch.
        function removeEntryDialogs() {
            root.querySelectorAll('dialog[data-terminal-retronet-dialog]').forEach(function (dialog) { dialog.remove(); });
        }

        function leaveDirectory() {
            removeEntryDialogs();
            if (!directory) return;
            directory.dispose();
            directory = null;
        }

        // HTTP 403 (Retro-Net switched off server-side) falls back to the shell; other errors stay in the
        // directory's error view, where R retries.
        function loadDirectory(current, preferId) {
            current.load(preferId).catch(function (err) {
                if (directory !== current || !err || err.status !== 403) return;
                retroNetOn = false;
                resetScreen();
                openShell();
            });
        }

        function showDirectory() {
            if (!term) return;
            if (!retroNetOn) {
                resetScreen();
                openShell();
                return;
            }
            stopRetro();
            closeShell();
            leaveDirectory();
            setGeometry('fit');
            resetScreen();
            term.options.convertEol = false;
            setMode('directory');
            setStatus('desktop.terminal_directory');
            directory = window.TerminalRetroNetDirectory.create({
                term: term,
                t: ctx.t,
                api: ctx.api,
                announce: announce,
                canEdit: function () { return (getBootstrap() || {}).readonly !== true; },
                onDial: dial,
                onEdit: editEntry,
                onDelete: deleteEntry
            });
            loadDirectory(directory);
            term.focus();
        }

        function ownEntries() {
            return directory ? directory.entries().filter(function (entry) { return entry && entry.own; }) : [];
        }

        function editEntry(entry) {
            const current = directory;
            if (!current) return;
            window.TerminalRetroNetEntries.open({
                host: root,
                t: ctx.t,
                esc: esc,
                api: ctx.api,
                entry: entry || null,
                ownEntries: ownEntries(),
                onSaved: function (entries, saved) {
                    if (directory === current) loadDirectory(current, saved && saved.id);
                }
            });
        }

        function deleteEntry(entry) {
            const current = directory;
            if (!current || !entry) return;
            window.TerminalRetroNetEntries.confirmDelete({
                host: root,
                t: ctx.t,
                esc: esc,
                api: ctx.api,
                entry: entry,
                ownEntries: ownEntries(),
                onSaved: function () {
                    if (directory === current) loadDirectory(current);
                }
            });
        }

        function dial(entry) {
            if (!entry || !term) return;
            saveLastEntry(entry.id);
            leaveDirectory();
            if (entry.id === window.TerminalRetroNetDirectory.LOCAL_SHELL_ID) {
                resetScreen();
                openShell();
                return;
            }
            startRetro(entry);
        }

        // The modem animation and the socket run in parallel; data waits for both. Runs synchronously from
        // the Enter key or click, so the modem can create its AudioContext inside the user gesture.
        function startRetro(entry) {
            stopRetro();
            closeShell();
            resetScreen();
            term.options.convertEol = false;
            const c64 = entry.kind === 'c64';
            const pinned = c64 ? 'c64' : entry.protocol === 'telnet' && entry.kind === 'bbs' ? 'bbs' : 'fit';
            setGeometry(pinned);
            const grid = window.TerminalGeometry.MODES[pinned];
            setMode('dialing');
            setStatus('desktop.terminal_dialing');
            if (!modem) {
                modem = window.TerminalModem.create({
                    term: term,
                    t: ctx.t,
                    getProfile: currentProfile,
                    isMuted: function () { return window.TerminalAudio ? window.TerminalAudio.loadMuted() : true; }
                });
            }
            const run = { entry: entry, c64: c64, fresh: c64, buffer: [], buffered: 0, connected: null, animationDone: false, live: false, ended: false, hostKey: false, session: null, throttle: null };
            retro = run;
            run.throttle = window.TerminalModem.createThrottle(function (bytes) {
                if (!term || retro !== run) return;
                if (run.fresh) {
                    run.fresh = false;
                    term.write(C64_FRESH_SCREEN);
                }
                term.write(bytes);
            });
            run.session = window.TerminalRetroNetSession.open({
                term: term,
                entry: entry,
                cols: grid ? grid.cols : term.cols,
                rows: grid ? grid.rows : term.rows,
                t: ctx.t,
                onControl: function (control) { onRetroControl(run, control); },
                onData: function (bytes) { onRetroData(run, bytes); },
                onClose: function () { onRetroClose(run); }
            });
            modem.dial(entry.host, entry.id).then(function () {
                if (retro !== run || run.ended) return;
                run.animationDone = true;
                goLive(run);
            });
        }

        // Data before CONNECT waits for the dial animation, at most the throttle's queue limit: a service that
        // sends more cuts the animation short. Data never precedes "connected", so the drop only bounds a bad server.
        function onRetroData(run, bytes) {
            if (retro !== run || run.ended) return;
            if (!run.live && run.buffered + bytes.length > window.TerminalModem.MAX_QUEUE_BYTES) {
                if (modem) modem.skip();
                run.animationDone = true;
                goLive(run);
                if (!run.live) return;
            }
            if (run.live) run.throttle.push(bytes);
            else {
                run.buffer.push(bytes);
                run.buffered += bytes.length;
            }
        }

        function onRetroControl(run, control) {
            if (retro !== run || run.ended || !control) return;
            if (control.type === 'connected') {
                run.connected = control;
                goLive(run);
            } else if (control.type === 'hostkey_prompt') {
                promptHostKey(run, control);
            } else if (control.type === 'result') {
                endRetro(run, String(control.code || 'NO CARRIER'), String(control.reason || 'remote_closed'), true);
            }
        }

        function onRetroClose(run) {
            if (retro !== run || run.ended) return;
            endRetro(run, 'NO CARRIER', 'lost', true);
        }

        function goLive(run) {
            if (retro !== run || run.ended || run.live || run.hostKey || !run.animationDone || !run.connected) return;
            run.live = true;
            const baud = window.TerminalModem.loadBaud();
            run.throttle.setBaud(baud);
            modem.connectLine(baud);
            if ((run.connected.protocol || run.entry.protocol) === 'telnet') writeDim(tr('desktop.terminal_retronet_telnet_notice'));
            if (run.c64) window.TerminalText.wrapHints(tr('desktop.terminal_retronet_c64_keys'), term.cols - 1).forEach(writeDim);
            announce(plain(tr('desktop.terminal_retronet_hangup_hint') + (run.c64 ? ' ' + tr('desktop.terminal_retronet_c64_keys') : '')));
            setMode('retro');
            setStatus('desktop.terminal_connected');
            const pending = run.buffer;
            run.buffer = [];
            run.buffered = 0;
            pending.forEach(function (bytes) { run.throttle.push(bytes); });
        }

        // SSH first contact (a sub-state of dialing): fingerprint and a localized yes/no question.
        function promptHostKey(run, control) {
            run.hostKey = true;
            if (modem) modem.skip();
            const title = tr('desktop.terminal_retronet_hostkey_title');
            const fingerprint = tr('desktop.terminal_retronet_hostkey_fingerprint', {
                type: window.TerminalText.printable(control.key_type),
                fingerprint: window.TerminalText.printable(control.fingerprint)
            });
            const question = tr('desktop.terminal_retronet_hostkey_question', {
                yes: tr('desktop.terminal_retronet_hostkey_yes'),
                no: tr('desktop.terminal_retronet_hostkey_no')
            });
            term.write('\r\n' + plain(title) + '\r\n' + plain(fingerprint) + '\r\n' + plain(question) + ' ');
            announce(plain(title + ' ' + fingerprint + ' ' + question));
        }

        // The session module's rule: localized letters first (they win a collision), then ASCII y/n.
        function answerHostKey(run, data) {
            const accept = window.TerminalRetroNetSession.hostKeyAnswer(data, tr('desktop.terminal_retronet_hostkey_yes'), tr('desktop.terminal_retronet_hostkey_no'));
            if (accept === null) return;
            run.hostKey = false;
            term.write(plain(accept ? tr('desktop.terminal_retronet_hostkey_yes') : tr('desktop.terminal_retronet_hostkey_no')) + '\r\n');
            run.session.hostKeyDecision(accept);
            goLive(run);
        }

        function enterResult() {
            resultAt = window.performance.now();
            setMode('result');
        }

        function showResult(code, reasonCode) {
            if (!term) return;
            const reason = RESULT_REASONS.indexOf(reasonCode) >= 0 ? reasonCode : 'remote_closed';
            // The server ends calls with "disabled" before the policy change reaches the page: treat it as
            // Retro-Net off now, so the hint and the next key both lead to the shell.
            if (reason === 'disabled') retroNetOn = false;
            const hayes = window.TerminalText.printable(code).replace(/[^A-Z ]/g, '') || 'NO CARRIER';
            const explanation = tr('desktop.terminal_retronet_result_' + reason);
            const next = retroNetOn ? tr('desktop.terminal_retronet_press_key') : tr('desktop.terminal_retronet_press_key_shell');
            term.write(PLAIN_SCREEN + '\r\n' + hayes + '\r\n');
            writeDim(explanation);
            term.write('\x1b[2m' + plain(next) + '\x1b[0m');
            announce(plain(hayes + '. ' + explanation + ' ' + next));
            enterResult();
            setStatus('desktop.terminal_stopped');
        }

        function endRetro(run, code, reason, remote) {
            if (retro !== run || run.ended) return;
            // A service that answers and hangs up during the dial (a short "all nodes busy" banner): the
            // dial is skipped and the buffered output shown before the result, as live data would be.
            if (remote && run.connected && !run.live && !run.hostKey) {
                if (modem) modem.skip();
                run.animationDone = true;
                goLive(run);
            }
            run.ended = true;
            if (modem) modem.skip();
            if (remote) run.session.dispose();
            else run.session.hangup();
            if (remote && run.live) run.throttle.flush();
            run.throttle.dispose();
            retro = null;
            showResult(code, reason);
        }

        function stopRetro() {
            const run = retro;
            if (!run) return;
            retro = null;
            run.ended = true;
            if (modem) modem.skip();
            if (run.session) run.session.dispose();
            if (run.throttle) run.throttle.dispose();
        }

        function hangup() {
            if (retro) {
                endRetro(retro, 'NO CARRIER', 'local_hangup', false);
                return;
            }
            if (mode === 'shell' && retroNetOn) showDirectory();
        }

        function afterResult() {
            if (retroNetOn) {
                showDirectory();
                return;
            }
            setGeometry('fit');
            resetScreen();
            openShell();
        }

        // Single input dispatcher for the one xterm instance. Ctrl+] is intercepted here and never
        // reaches a Retro-Net service (with Retro-Net off it reaches the shell as today).
        function onTermData(data) {
            if (mode === 'directory') {
                if (directory) directory.handleData(data);
                return;
            }
            if (mode === 'shell') {
                if (retroNetOn && data === HANGUP_KEY) {
                    showDirectory();
                    return;
                }
                if (ws && ws.readyState === WebSocket.OPEN) ws.send(data);
                return;
            }
            if (mode === 'result') {
                if (window.performance.now() - resultAt >= RESULT_GRACE_MS) afterResult();
                return;
            }
            const run = retro;
            if (!run) return;
            if (data.indexOf(HANGUP_KEY) >= 0) {
                hangup();
                return;
            }
            if (mode === 'dialing') {
                if (run.hostKey) answerHostKey(run, data);
                else if (modem) modem.skip();
                return;
            }
            if (run.session) run.session.send(data);
        }

        // C64 colour keys (Contract C), only during a live C64 call: the private sequence goes to the board, and
        // neither xterm nor the page (browser shortcuts) sees the key.
        function onTermKey(event) {
            const run = retro;
            if (!run || !run.c64 || mode !== 'retro') return true;
            const sequence = window.TerminalRetroNetSession.c64ColorKey(event);
            if (!sequence) return true;
            event.preventDefault();
            if (event.type === 'keydown' && !event.repeat) run.session.send(sequence);
            return false;
        }

        // BEL from a C64 board: a short beep under the key-click rules (retro style, not muted, motion allowed).
        function onTermBell() {
            if (audio && retro && retro.c64 && mode === 'retro') audio.beep();
        }

        function onTermResize(size) {
            if (mode === 'directory' && directory) {
                directory.render();
                return;
            }
            if (retro && retro.session && (!geometry || geometry.mode() === 'fit')) retro.session.resize(size.cols, size.rows);
        }

        // Explicit readonly true, enabled false or retronet_enabled false switch Retro-Net off; retronet_enabled
        // true (merged over the bootstrap) can switch it on. Missing fields mean "unchanged": the desktop-socket
        // close event carries the serial flags alone.
        function onPolicy(event) {
            const detail = (event && event.detail) || {};
            const off = detail.readonly === true || detail.enabled === false || detail.retronet_enabled === false;
            if (!off && detail.retronet_enabled !== true) return;
            const next = off ? false : retroNetAllowed(detail);
            if (next === retroNetOn) return;
            retroNetOn = next;
            syncRetroToolbar();
            // Live sessions are ended by the server (NO CARRIER / disabled) within about a second.
            if (!next && term && mode === 'directory') {
                resetScreen();
                openShell();
            }
        }

        // ---- Screen geometry (terminal-geometry.js) and the temporary C64 look ----

        // The style select is locked while the C64 look is on; the tooltip says why.
        function lockStyle(locked) {
            [select, styleBox].forEach(function (node) {
                if (!node) return;
                if (locked) node.title = ctx.t('desktop.terminal_retronet_style_locked');
                else node.removeAttribute('title');
            });
            if (select) select.disabled = locked;
        }

        // The commodore64 style without saving it, a black screen and a blinking block cursor.
        function enterC64Look() {
            if (c64Look || !term) return;
            c64Look = { style: root.getAttribute('data-terminal-style'), cursorStyle: term.options.cursorStyle, cursorBlink: term.options.cursorBlink };
            lockStyle(true);
            term.options.cursorStyle = 'block';
            term.options.cursorBlink = true;
            term.options.theme = Object.assign({}, styles.profile('commodore64').theme, C64_THEME);
            applyStyle('commodore64', false);
        }

        // Back to the user's style (still the saved one), cursor and an unlocked style select.
        function leaveC64Look() {
            const look = c64Look;
            if (!look) return;
            c64Look = null;
            lockStyle(false);
            if (term) {
                term.options.cursorStyle = look.cursorStyle;
                term.options.cursorBlink = look.cursorBlink;
            }
            applyStyle(look.style, false);
        }

        // 'fit' | 'bbs' | 'c64' for the next screen; the C64 look belongs to 'c64' only.
        function setGeometry(next) {
            if (next !== 'c64') leaveC64Look();
            if (geometry) geometry.set(next);
            if (next === 'c64') enterC64Look();
        }

        function cleanup() {
            document.removeEventListener('keydown', onKeyDown, true);
            document.removeEventListener('aurago:desktop-policy', onPolicy);
            if (observer) observer.disconnect();
            effectsObserver.disconnect();
            motionQuery.removeEventListener('change', syncEffectsMenu);
            if (effectsDialog.open) effectsDialog.close();
            // Removed, not closed: close() reopens an entry dialog whose save is still pending.
            removeEntryDialogs();
            if (geometry) geometry.dispose();
            // The C64 look is never saved: dropping it leaves the stored style untouched.
            c64Look = null;
            if (modem) modem.dispose();
            leaveDirectory();
            stopRetro();
            closeShell();
            if (crt) crt.dispose();
            if (audio) audio.dispose();
            if (canvasAddon && typeof canvasAddon.dispose === 'function') {
                try { canvasAddon.dispose(); } catch (e) {}
            }
            if (term && typeof term.dispose === 'function') term.dispose();
            ws = null;
            term = null;
            fit = null;
            geometry = null;
            canvasAddon = null;
            crt = null;
            audio = null;
            observer = null;
            modem = null;
        }

        inst.cleanup = cleanup;
        instances.set(windowId, inst);
        if (typeof ctx.registerWindowCleanup === 'function') ctx.registerWindowCleanup(windowId, cleanup);

        if (select) {
            select.addEventListener('change', function () {
                applyStyle(select.value);
            });
        }
        if (audioBtn) {
            audioBtn.addEventListener('click', function () {
                const next = !(window.TerminalAudio && window.TerminalAudio.loadMuted());
                if (audio) audio.setMuted(next);
                else if (window.TerminalAudio) window.TerminalAudio.saveMuted(next);
                syncAudioButton();
            });
        }
        if (retroBtn) {
            retroBtn.addEventListener('click', function () {
                if (mode === 'dialing' || mode === 'retro') hangup();
                else showDirectory();
                if (term) term.focus();
            });
        }
        if (baudSelect && retroNetReady) {
            baudSelect.addEventListener('change', function () {
                const rate = window.TerminalModem.saveBaud(baudSelect.value);
                baudSelect.value = String(rate);
                if (retro && retro.live) retro.throttle.setBaud(rate);
            });
        }
        document.addEventListener('keydown', onKeyDown, true);
        const effectsObserver = new MutationObserver(syncEffectsMenu);
        effectsObserver.observe(root, { attributes: true, attributeFilter: ['data-terminal-fallback'] });
        effectsObserver.observe(document.body, { attributes: true, attributeFilter: ['data-animations'] });
        motionQuery.addEventListener('change', syncEffectsMenu);
        effectsBtn.addEventListener('click', function () { syncEffectsMenu(); effectsDialog.showModal(); });
        effectsDialog.querySelector('[data-terminal-effects-close]').addEventListener('click', function () { effectsDialog.close(); });
        effectsDialog.addEventListener('input', function (event) {
            const key = event.target.dataset.terminalEffect;
            if (!key || event.target.disabled || !currentProfile().retro) return;
            effectValues[key] = Number(event.target.value);
            effectValues = styles.saveEffects(root.dataset.terminalStyle, effectValues);
            applyEffects();
        });
        effectsDialog.querySelector('[data-terminal-effects-reset]').addEventListener('click', function () {
            effectValues = styles.resetEffects(root.dataset.terminalStyle);
            applyEffects();
        });
        applyEffects();

        if (!window.Terminal) {
            setStatus('desktop.terminal_unavailable');
            syncAudioButton();
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
            fit = new window.FitAddon.FitAddon();
            term.loadAddon(fit);
            fit.fit();
            window.setTimeout(function () { if (fit && (!geometry || geometry.mode() === 'fit')) fit.fit(); }, 80);
        }
        if (window.TerminalGeometry) {
            geometry = window.TerminalGeometry.create({
                term: term,
                screen: screen,
                root: root,
                fit: fit,
                getProfile: currentProfile,
                onFitted: function () { if (crt) crt.resize(); }
            });
        }
        if (typeof ResizeObserver === 'function') {
            observer = new ResizeObserver(function () {
                refitScreen();
                if (crt) crt.resize();
            });
            observer.observe(screen);
        }
        term.onData(onTermData);
        term.onResize(onTermResize);
        term.onBell(onTermBell);
        term.attachCustomKeyEventHandler(onTermKey);
        retroNetOn = retroNetAllowed();
        document.addEventListener('aurago:desktop-policy', onPolicy);
        // Start mode: directory when Retro-Net is on and no path context; otherwise today's shell.
        // "Open Terminal Here" always passes a string path, '' for the workspace root.
        if (retroNetOn && typeof ctx.path !== 'string') showDirectory();
        else openShell();

        applyStyle(initial);
    }

    function dispose(windowId) {
        if (windowId) {
            const inst = instances.get(windowId);
            if (inst && inst.cleanup) inst.cleanup();
            instances.delete(windowId);
            return;
        }
        instances.forEach(function (inst) {
            if (inst && inst.cleanup) inst.cleanup();
        });
        instances.clear();
    }

    window.TerminalApp = { render, dispose };
})();
