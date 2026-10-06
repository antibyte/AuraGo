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
        const effectMarkup = styles.effectControls.map(function (effect, index) {
            const heading = index === 0 ? 'image' : index === 8 ? 'glass' : '';
            return (heading ? '<h3>' + ctx.t('desktop.terminal_effects_' + heading) + '</h3>' : '') +
                '<label class="vd-terminal-effect"><span>' + ctx.t('desktop.terminal_effects_' + effect.key) + '</span>' +
                '<output aria-hidden="true" data-terminal-effect-output="' + effect.key + '"></output>' +
                '<input type="range" data-terminal-effect="' + effect.key + '" min="' + (effect.min || 0) +
                '" max="' + (effect.max || 100) + '" step="1"></label>';
        }).join('');
        host.innerHTML = '<div class="vd-terminal-app" data-terminal-state="desktop.loading" data-terminal-style="' + initial + '">' +
            '<div class="vd-terminal-toolbar">' +
            '<span class="vd-terminal-status vd-terminal-toolbar-status" data-terminal-status role="status">' + ctx.t('desktop.loading') + '</span>' +
            '<div class="vd-terminal-toolbar-actions">' +
            '<label class="vd-terminal-style-label">' +
            '<span class="vd-sr-only">' + ctx.t('desktop.terminal_style') + '</span>' +
            '<select data-terminal-style aria-label="' + ctx.t('desktop.terminal_style') + '">' + optionMarkup(ctx) + '</select>' +
            '</label>' +
            '<button type="button" data-terminal-effects aria-haspopup="dialog">' + ctx.t('desktop.terminal_effects') + '</button>' +
            '<button type="button" data-terminal-audio aria-pressed="' + (muted ? 'false' : 'true') + '">' +
            ctx.t(muted ? 'desktop.terminal_audio_off' : 'desktop.terminal_audio_on') +
            '</button>' +
            '</div></div>' +
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
        const inst = { root: host };

        if (select) select.value = initial;
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
            if (fit && typeof fit.fit === 'function') fit.fit();
            if (crt && typeof crt.resize === 'function') crt.resize();
        }

        function onKeyDown(event) {
            if (!audio || effectsDialog.open || !host.contains(document.activeElement)) return;
            audio.playKey(event);
        }

        function cleanup() {
            document.removeEventListener('keydown', onKeyDown, true);
            if (observer) observer.disconnect();
            effectsObserver.disconnect();
            motionQuery.removeEventListener('change', syncEffectsMenu);
            if (effectsDialog.open) effectsDialog.close();
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
