(function () {
    'use strict';

    const instances = new Map();

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
        const esc = typeof ctx.esc === 'function' ? ctx.esc : function (s) {
            return String(s || '').replace(/[&<>"']/g, function (m) {
                return ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[m];
            });
        };
        return window.TerminalStyles.ids.map(function (id) {
            return '<option value="' + id + '">' + esc(styleLabel(ctx, id)) + '</option>';
        }).join('');
    }

    function render(host, windowId, ctx) {
        if (!host) return;
        dispose(windowId);
        const styles = window.TerminalStyles;
        const initial = styles ? styles.load() : 'modern';
        const muted = window.TerminalAudio ? window.TerminalAudio.loadMuted() : false;
        host.innerHTML = '<div class="vd-terminal-app" data-terminal-style="' + initial + '">' +
            '<div class="vd-terminal-toolbar">' +
            '<span class="vd-terminal-status vd-terminal-toolbar-status" data-terminal-status>' + ctx.t('desktop.loading') + '</span>' +
            '<div class="vd-terminal-toolbar-actions">' +
            '<label class="vd-terminal-style-label">' +
            '<span class="vd-sr-only">' + ctx.t('desktop.terminal_style') + '</span>' +
            '<select data-terminal-style aria-label="' + ctx.t('desktop.terminal_style') + '">' + optionMarkup(ctx) + '</select>' +
            '</label>' +
            '<button type="button" data-terminal-audio aria-pressed="' + (muted ? 'false' : 'true') + '">' +
            ctx.t(muted ? 'desktop.terminal_audio_off' : 'desktop.terminal_audio_on') +
            '</button>' +
            '</div></div>' +
            '<div class="vd-terminal-stage">' +
            '<div class="vd-terminal-bezel" data-terminal-bezel>' +
            '<div class="vd-terminal-screen" data-terminal-screen></div>' +
            '</div></div></div>';

        const root = host.querySelector('.vd-terminal-app');
        const screen = host.querySelector('[data-terminal-screen]');
        const status = host.querySelector('[data-terminal-status]');
        const select = host.querySelector('select[data-terminal-style]');
        const audioBtn = host.querySelector('[data-terminal-audio]');
        let ws = null;
        let term = null;
        let fit = null;
        let canvasAddon = null;
        let crt = null;
        let audio = window.TerminalAudio ? window.TerminalAudio.create() : null;
        let observer = null;
        const inst = { root: host };

        if (select) select.value = initial;
        if (audio) audio.setMuted(muted);

        function setStatus(key) {
            if (status) status.textContent = ctx.t(key);
        }

        function currentProfile() {
            return styles ? styles.profile(root.getAttribute('data-terminal-style')) : { id: 'modern', retro: false };
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
            if (!term || !window.CanvasAddon || !window.CanvasAddon.CanvasAddon) return;
            if (enable && !canvasAddon) {
                try {
                    canvasAddon = new window.CanvasAddon.CanvasAddon();
                    term.loadAddon(canvasAddon);
                } catch (e) {
                    canvasAddon = null;
                }
            }
            if (!enable && canvasAddon && typeof canvasAddon.dispose === 'function') {
                try { canvasAddon.dispose(); } catch (e) {}
                canvasAddon = null;
            }
        }

        function applyStyle(id) {
            const next = styles.save(id);
            root.setAttribute('data-terminal-style', next);
            root.removeAttribute('data-terminal-fallback');
            const profile = styles.profile(next);
            styles.applyXterm(term, profile);
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
            syncAudioButton();
            function scheduleFit() {
                if (!term) return;
                if (fit && typeof fit.fit === 'function') fit.fit();
                if (crt && typeof crt.resize === 'function') crt.resize();
            }
            scheduleFit();
            if (document.fonts && typeof document.fonts.load === 'function') {
                const family = String(profile.fontFamily || '').split(',')[0].trim() || 'monospace';
                const spec = String(profile.fontSize || 13) + 'px ' + family;
                document.fonts.load(spec).then(scheduleFit, scheduleFit);
            }
        }

        function onKeyDown(event) {
            if (!audio || !host.contains(document.activeElement)) return;
            audio.playKey(event);
        }

        function cleanup() {
            document.removeEventListener('keydown', onKeyDown, true);
            if (observer) observer.disconnect();
            if (ws && ws.readyState !== WebSocket.CLOSED) ws.close();
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
        document.addEventListener('keydown', onKeyDown, true);

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
            window.setTimeout(function () { if (fit) fit.fit(); }, 80);
        }
        if (typeof ResizeObserver === 'function') {
            observer = new ResizeObserver(function () {
                if (fit) fit.fit();
                if (crt) crt.resize();
            });
            observer.observe(screen);
        }

        const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
        ws = new WebSocket(protocol + '//' + location.host + '/api/code-studio/terminal');
        ws.binaryType = 'arraybuffer';
        ws.onopen = function () {
            setStatus('desktop.terminal_running');
            term.onData(function (data) {
                if (ws && ws.readyState === WebSocket.OPEN) ws.send(data);
            });
        };
        ws.onmessage = function (event) {
            if (!term) return;
            if (event.data instanceof ArrayBuffer) term.write(new Uint8Array(event.data));
            else term.write(String(event.data));
        };
        ws.onerror = function () { setStatus('desktop.terminal_unavailable'); };
        ws.onclose = function () { setStatus('desktop.terminal_stopped'); };

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
