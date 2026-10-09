(function () {
    'use strict';

    const instances = new Map();
    const LAST_ENTRY_KEY = 'aurago.desktop.terminal.retronet.last';
    const HANGUP_KEY = '\x1d';
    const VGA_FONT = '"Aura VGA", "Aura Terminal", ui-monospace, monospace';
    const VGA_FONT_SPEC = '16px "Aura VGA"';
    // Normal screen buffer, then DECSTR (attributes, cursor, scroll region, wrap, insert, focus and paste
    // reporting) and mouse reporting off: after a session only real keys reach the result prompt.
    const PLAIN_SCREEN = '\x1b[?1047l\x1b[!p\x1b[?1000l\x1b[?1002l\x1b[?1003l\x1b[?1006l';
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

    // Retro-Net needs every module of Contract E, including the shared TerminalText helpers.
    function retroNetModulesLoaded() {
        return !!(window.TerminalText && window.TerminalModem && window.TerminalRetroNetDirectory &&
            window.TerminalRetroNetSession && window.TerminalRetroNetEntries);
    }

    function translate(ctx, key, params) {
        let text = String(ctx.t(key, params));
        if (params) {
            Object.keys(params).forEach(function (name) {
                text = text.split('{{' + name + '}}').join(String(params[name]));
            });
        }
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

    function nextFrame() {
        return new Promise(function (resolve) {
            window.requestAnimationFrame(function () { resolve(); });
        });
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
        let bbsMode = false;
        let bbsRevision = 0;
        let bbsFrame = 0;
        let bbsFontSize = 16;
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
            if (crt) crt.setProfile(profile);
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

        async function applyStyle(id) {
            const revision = ++styleRevision;
            const next = styles.save(id);
            effectValues = styles.loadEffects(next);
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
            if (bbsMode) applyBbsOptions();
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
                    crt.setProfile(profile);
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
            if (bbsMode) scheduleBbsFit();
            else if (fit && typeof fit.fit === 'function') fit.fit();
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
            leaveBbsMode();
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
                    setMode('result');
                }
            };
        }

        function leaveDirectory() {
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
            leaveBbsMode();
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
            const bbs = entry.protocol === 'telnet' && entry.kind === 'bbs';
            if (bbs) enterBbsMode();
            else leaveBbsMode();
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
            const run = { entry: entry, buffer: [], connected: null, animationDone: false, live: false, ended: false, hostKey: false, session: null, throttle: null };
            retro = run;
            run.throttle = window.TerminalModem.createThrottle(function (bytes) {
                if (term && retro === run) term.write(bytes);
            });
            run.session = window.TerminalRetroNetSession.open({
                term: term,
                entry: entry,
                cols: bbs ? 80 : term.cols,
                rows: bbs ? 25 : term.rows,
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

        function onRetroData(run, bytes) {
            if (retro !== run || run.ended) return;
            if (run.live) run.throttle.push(bytes);
            else run.buffer.push(bytes);
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
            announce(plain(tr('desktop.terminal_retronet_hangup_hint')));
            setMode('retro');
            setStatus('desktop.terminal_connected');
            const pending = run.buffer;
            run.buffer = [];
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

        function showResult(code, reasonCode) {
            if (!term) return;
            const reason = RESULT_REASONS.indexOf(reasonCode) >= 0 ? reasonCode : 'remote_closed';
            const hayes = window.TerminalText.printable(code).replace(/[^A-Z ]/g, '') || 'NO CARRIER';
            const explanation = tr('desktop.terminal_retronet_result_' + reason);
            const next = retroNetOn ? tr('desktop.terminal_retronet_press_key') : tr('desktop.terminal_retronet_press_key_shell');
            term.write(PLAIN_SCREEN + '\r\n' + hayes + '\r\n');
            writeDim(explanation);
            term.write('\x1b[2m' + plain(next) + '\x1b[0m');
            announce(plain(hayes + '. ' + explanation + ' ' + next));
            setMode('result');
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
            leaveBbsMode();
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
                afterResult();
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

        function onTermResize(size) {
            if (mode === 'directory' && directory) {
                directory.render();
                return;
            }
            if (retro && retro.session && !bbsMode) retro.session.resize(size.cols, size.rows);
        }

        // Only an explicit retronet_enabled changes Retro-Net: the desktop-socket close event carries the
        // serial flags alone and means "unchanged".
        function onPolicy(event) {
            const detail = (event && event.detail) || {};
            if (typeof detail.retronet_enabled !== 'boolean') return;
            const next = retroNetAllowed(detail);
            if (next === retroNetOn) return;
            retroNetOn = next;
            syncRetroToolbar();
            // Live sessions are ended by the server (NO CARRIER / disabled) within about a second.
            if (!next && term && mode === 'directory') {
                resetScreen();
                openShell();
            }
        }

        // ---- 80x25 BBS geometry with the VGA font ----

        function applyBbsOptions() {
            term.options.fontFamily = VGA_FONT;
            term.options.lineHeight = 1;
            term.options.fontSize = bbsFontSize;
            term.options.scrollback = 0;
        }

        function enterBbsMode() {
            if (!term) return;
            bbsMode = true;
            bbsRevision += 1;
            const revision = bbsRevision;
            root.setAttribute('data-terminal-geometry', 'bbs');
            applyBbsOptions();
            term.resize(80, 25);
            const ready = document.fonts && typeof document.fonts.load === 'function'
                ? document.fonts.load(VGA_FONT_SPEC).catch(function () { return []; })
                : Promise.resolve([]);
            ready.then(function () {
                if (bbsMode && revision === bbsRevision) fitBbsFont().catch(function () {});
            });
        }

        function leaveBbsMode() {
            if (!bbsMode) return;
            bbsMode = false;
            bbsRevision += 1;
            if (bbsFrame) window.cancelAnimationFrame(bbsFrame);
            bbsFrame = 0;
            root.removeAttribute('data-terminal-geometry');
            if (!term) return;
            if (term.element) {
                term.element.style.padding = '';
                const screenEl = term.element.querySelector('.xterm-screen');
                if (screenEl) screenEl.style.background = '';
            }
            term.options.scrollback = 1000;
            styles.applyXterm(term, currentProfile());
            if (fit) fit.fit();
            if (crt) crt.resize();
        }

        function scheduleBbsFit() {
            if (!bbsMode || bbsFrame) return;
            bbsFrame = window.requestAnimationFrame(function () {
                bbsFrame = 0;
                fitBbsFont().catch(function () {});
            });
        }

        // The style's own padding and the glass size, read with the letterbox padding removed and restored
        // in the same task: a rendered change would fire the screen's ResizeObserver and refit endlessly.
        function bbsSpace(element) {
            const sides = ['paddingTop', 'paddingRight', 'paddingBottom', 'paddingLeft'];
            const kept = sides.map(function (side) { return element.style[side]; });
            element.style.padding = '';
            const style = window.getComputedStyle(element);
            const pad = {
                left: parseFloat(style.paddingLeft) || 0,
                right: parseFloat(style.paddingRight) || 0,
                top: parseFloat(style.paddingTop) || 0,
                bottom: parseFloat(style.paddingBottom) || 0
            };
            const space = { pad: pad, width: screen.clientWidth - pad.left - pad.right, height: screen.clientHeight - pad.top - pad.bottom };
            sides.forEach(function (side, index) { element.style[side] = kept[index]; });
            return space;
        }

        // Largest font size in 0.5 px steps at which the 80x25 grid fits; centered via padding, written only
        // when it changes.
        async function fitBbsFont() {
            if (!bbsMode || !term || !term.element) return;
            const revision = bbsRevision;
            const element = term.element;
            const screenEl = element.querySelector('.xterm-screen');
            if (!screenEl) return;
            const space = bbsSpace(element);
            const pad = space.pad;
            const availW = space.width;
            const availH = space.height;
            if (availW <= 0 || availH <= 0) return;
            // Px437 8x16: a cell is 0.5em wide and 1em high, so 80x25 needs 40em x 25em.
            let size = Math.max(6, Math.floor(Math.min(availW / 40, availH / 25) * 2) / 2);
            let rect = null;
            for (let attempt = 0; attempt < 12; attempt += 1) {
                term.options.fontSize = size;
                await nextFrame();
                if (!bbsMode || revision !== bbsRevision || !term) return;
                rect = screenEl.getBoundingClientRect();
                if ((rect.width <= availW + 0.5 && rect.height <= availH + 0.5) || size <= 6) break;
                size -= 0.5;
            }
            const resized = bbsFontSize !== term.options.fontSize;
            bbsFontSize = term.options.fontSize;
            const extraX = Math.max(0, (availW - rect.width) / 2);
            const extraY = Math.max(0, (availH - rect.height) / 2);
            const px = function (value) { return Math.round(value * 100) / 100 + 'px'; };
            const next = {
                paddingLeft: px(pad.left + extraX),
                paddingRight: px(pad.right + extraX),
                paddingTop: px(pad.top + extraY),
                paddingBottom: px(pad.bottom + extraY)
            };
            let moved = false;
            Object.keys(next).forEach(function (side) {
                if (element.style[side] === next[side]) return;
                element.style[side] = next[side];
                moved = true;
            });
            screenEl.style.background = (term.options.theme && term.options.theme.background) || '';
            if (crt && (resized || moved)) crt.resize();
        }

        function cleanup() {
            document.removeEventListener('keydown', onKeyDown, true);
            document.removeEventListener('aurago:desktop-policy', onPolicy);
            if (observer) observer.disconnect();
            effectsObserver.disconnect();
            motionQuery.removeEventListener('change', syncEffectsMenu);
            if (effectsDialog.open) effectsDialog.close();
            // Removed, not closed: close() reopens an entry dialog whose save is still pending.
            root.querySelectorAll('dialog[data-terminal-retronet-dialog]').forEach(function (dialog) { dialog.remove(); });
            if (bbsFrame) window.cancelAnimationFrame(bbsFrame);
            bbsFrame = 0;
            bbsMode = false;
            bbsRevision += 1;
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
            window.setTimeout(function () { if (fit && !bbsMode) fit.fit(); }, 80);
        }
        if (typeof ResizeObserver === 'function') {
            observer = new ResizeObserver(function () {
                if (bbsMode) scheduleBbsFit();
                else if (fit) fit.fit();
                if (crt) crt.resize();
            });
            observer.observe(screen);
        }
        term.onData(onTermData);
        term.onResize(onTermResize);
        retroNetOn = retroNetAllowed();
        document.addEventListener('aurago:desktop-policy', onPolicy);
        // Start mode: directory when Retro-Net is on and no path context; otherwise today's shell.
        if (retroNetOn && !ctx.path) showDirectory();
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
