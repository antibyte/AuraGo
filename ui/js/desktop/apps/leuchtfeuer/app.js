/* Leuchtfeuer window. One harbour, three parts, no network beyond the asset route. */
(function (root) {
    const LOCALES = new Set(['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh']);
    const SESSION = 'aurago.leuchtfeuer.v1';
    const PACKS = '/api/game-maker/asset-packs/';
    const sessions = new Map();

    function element(tag, className, text) {
        const node = document.createElement(tag);
        if (className) node.className = className;
        if (text !== undefined) node.textContent = text;
        return node;
    }

    function render(host, windowId, ctx) {
        dispose(windowId);
        ctx = ctx || {};
        const controller = new AbortController();
        let generation = 0;
        let disposed = false;
        let view = null;
        let sim = null;
        let world = null;
        let texts = {};
        let userPaused = false;
        let wantInteract = false;
        let soundOn = false;
        let saveTimer = 0;
        const locale = LOCALES.has(String(ctx.lang || root.SYSTEM_LANG || document.documentElement.lang || 'de').slice(0, 2).toLowerCase())
            ? String(ctx.lang || root.SYSTEM_LANG || document.documentElement.lang || 'de').slice(0, 2).toLowerCase()
            : 'de';

        const shell = element('section', 'lf-app');
        shell.tabIndex = 0;
        shell.setAttribute('aria-label', ctx.t ? ctx.t('desktop.app_leuchtfeuer') : 'Leuchtfeuer');
        shell.setAttribute('lang', locale);
        const stage = element('div', 'lf-stage');
        const hud = element('div', 'lf-hud');
        const panel = element('div', 'lf-panel');
        const goal = element('p', 'lf-goal');
        const parts = element('p', 'lf-parts');
        const notice = element('p', 'lf-notice');
        notice.setAttribute('role', 'status');
        const controls = element('p', 'lf-controls');
        panel.append(goal, parts, notice, controls);
        const pauseButton = element('button', 'lf-pause-button');
        pauseButton.type = 'button';
        hud.append(panel, pauseButton);
        const touch = element('div', 'lf-touch');
        const loading = element('div', 'lf-card lf-loading');
        const loadingText = element('p');
        loading.append(loadingText);
        const errorCard = element('div', 'lf-card lf-error');
        errorCard.hidden = true;
        const errorText = element('p');
        const retry = element('button');
        retry.type = 'button';
        errorCard.append(errorText, retry);
        const winCard = element('div', 'lf-card lf-win');
        winCard.hidden = true;
        const winTitle = element('h2');
        const winBody = element('p');
        const winRestart = element('button');
        winRestart.type = 'button';
        winCard.append(winTitle, winBody, winRestart);
        const dialog = element('dialog', 'lf-dialog');
        const pauseTitle = element('h2');
        const resume = element('button');
        resume.type = 'button';
        const restart = element('button');
        restart.type = 'button';
        const sound = element('button');
        sound.type = 'button';
        sound.setAttribute('aria-pressed', 'false');
        const actions = element('div', 'lf-actions');
        actions.append(resume, restart, sound);
        dialog.append(pauseTitle, actions);
        shell.append(stage, hud, touch, loading, errorCard, winCard, dialog);
        host.replaceChildren(shell);

        function t(key) {
            const value = texts[key];
            return typeof value === 'string' ? value : key;
        }
        function fill(key, parameters) {
            return t(key).replace(/\{([a-zA-Z_]+)\}/g, function (match, name) {
                return Object.prototype.hasOwnProperty.call(parameters, name) ? String(parameters[name]) : match;
            });
        }
        function applyCopy() {
            goal.textContent = t('leuchtfeuer.goal');
            controls.textContent = t('leuchtfeuer.controls');
            pauseButton.textContent = t('leuchtfeuer.pause.title');
            loadingText.textContent = t('leuchtfeuer.loading');
            errorText.textContent = t('leuchtfeuer.error.assets');
            retry.textContent = t('leuchtfeuer.error.retry');
            winTitle.textContent = t('leuchtfeuer.win.title');
            winBody.textContent = t('leuchtfeuer.win.body');
            winRestart.textContent = t('leuchtfeuer.pause.restart');
            pauseTitle.textContent = t('leuchtfeuer.pause.title');
            resume.textContent = t('leuchtfeuer.pause.resume');
            restart.textContent = t('leuchtfeuer.pause.restart');
            paintSound();
            paintTouch();
        }
        function paintSound() {
            sound.textContent = soundOn ? t('leuchtfeuer.sound.on') : t('leuchtfeuer.sound.off');
            sound.setAttribute('aria-pressed', soundOn ? 'true' : 'false');
        }
        function paintTouch() {
            touch.replaceChildren();
            [['touch.forward', 'KeyW'], ['touch.back', 'KeyS'], ['touch.left', 'KeyA'], ['touch.right', 'KeyD']].forEach(function (pair) {
                const button = element('button', '', t('leuchtfeuer.' + pair[0]));
                button.type = 'button';
                button.addEventListener('pointerdown', function (event) {
                    event.preventDefault();
                    view && view.hold(pair[1], true);
                });
                button.addEventListener('pointerup', function () { view && view.hold(pair[1], false); });
                button.addEventListener('pointerleave', function () { view && view.hold(pair[1], false); });
                touch.append(button);
            });
            const act = element('button', '', t('leuchtfeuer.touch.act'));
            act.type = 'button';
            act.addEventListener('click', function () { wantInteract = true; });
            touch.append(act);
        }
        function paintHud() {
            if (!sim) return;
            parts.replaceChildren();
            const label = element('span', '', t('leuchtfeuer.hud.parts') + ': ');
            parts.append(label);
            ['glass', 'wick', 'oil'].forEach(function (part, index) {
                if (index) parts.append(element('span', '', ', '));
                const name = element('span', sim.parts[part] ? 'lf-part-on' : '', t('leuchtfeuer.part.' + part));
                parts.append(name);
            });
            if (sim.notice === 'leuchtfeuer.notice.missing') {
                const names = (sim.missing || []).map(function (part) { return t('leuchtfeuer.part.' + part); }).join(', ');
                notice.textContent = fill('leuchtfeuer.notice.missing', { parts: names });
            } else if (sim.notice) notice.textContent = t(sim.notice);
            else notice.textContent = '';
            shell.dataset.lfX = String(sim.x);
            shell.dataset.lfZ = String(sim.z);
            shell.dataset.lfLit = sim.lit ? '1' : '0';
            shell.dataset.lfNotice = sim.notice || '';
            shell.dataset.lfParts = ['glass', 'wick', 'oil'].filter(function (part) { return sim.parts[part]; }).join(',');
            winCard.hidden = !sim.lit;
        }
        function paused() {
            const active = !ctx.isActive || ctx.isActive();
            return disposed || userPaused || !active || document.hidden || !errorCard.hidden || !loading.hidden || dialog.open || (sim && sim.lit);
        }
        function syncPause() {
            if (view) view.setPaused(paused());
            shell.classList.toggle('lf-paused', userPaused || dialog.open);
        }
        function layout() {
            shell.classList.toggle('lf-narrow', shell.clientWidth < 760);
        }
        function readSession() {
            try {
                const raw = root.sessionStorage.getItem(SESSION);
                if (!raw || raw.length > 4096) return null;
                return JSON.parse(raw);
            } catch (err) { return null; }
        }
        function writeSession() {
            if (!sim) return;
            try { root.sessionStorage.setItem(SESSION, JSON.stringify(root.LeuchtfeuerSim.exportState(sim))); } catch (err) { /* private mode */ }
        }
        function showError() {
            loading.hidden = true;
            errorCard.hidden = false;
            syncPause();
        }
        async function dictionary() {
            const response = await fetch('/lang/leuchtfeuer/' + locale + '.json?v=' + encodeURIComponent(root.BUILD_VERSION || 'dev'), {
                signal: controller.signal, credentials: 'same-origin'
            });
            if (!response.ok) throw new Error('locale');
            const source = await response.text();
            if (source.length > 256 * 1024) throw new Error('locale');
            const parsed = JSON.parse(source);
            if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') throw new Error('locale');
            Object.keys(parsed).forEach(function (key) {
                if (!key.startsWith('leuchtfeuer.') || typeof parsed[key] !== 'string') throw new Error('locale');
            });
            return parsed;
        }
        async function manifest(id) {
            const response = await fetch(PACKS + id + '/manifest.json', { signal: controller.signal, credentials: 'same-origin', cache: 'no-store' });
            if (!response.ok) throw new Error('assets');
            return response.json();
        }
        async function boot() {
            const ticket = ++generation;
            loading.hidden = false;
            errorCard.hidden = true;
            winCard.hidden = true;
            loadingText.textContent = '…';
            syncPause();
            try {
                const loaded = await Promise.all([
                    dictionary(),
                    manifest('aurago-pirates-3d'),
                    manifest('aurago-effects'),
                    manifest('aurago-sounds')
                ]);
                if (disposed || ticket !== generation) return;
                texts = loaded[0];
                applyCopy();
                world = root.LeuchtfeuerWorld.createWorld(loaded[1]);
                sim = root.LeuchtfeuerSim.createSim(world);
                const lampSpot = world.spots.find(function (spot) { return spot.id === 'lamp'; });
                if (lampSpot) shell.dataset.lfLamp = lampSpot.x + ',' + lampSpot.z;
                root.LeuchtfeuerSim.importState(sim, readSession());
                const config = root.LeuchtfeuerAudio.presentationConfig(loaded[2], loaded[3]);
                view = await root.LeuchtfeuerView.mount(stage, world, sim, {
                    signal: controller.signal,
                    manifest: loaded[1],
                    config: config,
                    isActive: function () { return !ctx.isActive || ctx.isActive(); },
                    takeInteract: function () {
                        const pressed = wantInteract;
                        wantInteract = false;
                        return pressed;
                    },
                    onFrame: function () { if (!disposed) { paintHud(); syncPause(); } },
                    onLost: function () { if (!disposed) showError(); }
                });
                if (disposed || ticket !== generation) { view.dispose(); view = null; return; }
                view.setMuted(!soundOn);
                loading.hidden = true;
                paintHud();
                layout();
                syncPause();
                shell.focus({ preventScroll: true });
            } catch (err) {
                if (disposed || ticket !== generation || (err && err.name === 'AbortError')) return;
                shell.dataset.lfBoot = err && err.message ? String(err.message).slice(0, 500) : 'boot';
                showError();
            }
        }
        function beginRestart() {
            try { root.sessionStorage.removeItem(SESSION); } catch (err) { /* private mode */ }
            if (sim) root.LeuchtfeuerSim.importState(sim, null);
            userPaused = false;
            if (dialog.open) dialog.close();
            winCard.hidden = true;
            paintHud();
            syncPause();
        }
        function onKey(event) {
            if (disposed || event.repeat) return;
            if (ctx.isActive && !ctx.isActive()) return;
            if (event.code === 'Escape') {
                event.preventDefault();
                if (dialog.open) dialog.close();
                else if (view) dialog.showModal();
                userPaused = dialog.open;
                syncPause();
                return;
            }
            if (event.code === 'KeyE' && !dialog.open) wantInteract = true;
        }
        pauseButton.addEventListener('click', function () {
            if (!view) return;
            dialog.showModal();
            userPaused = true;
            syncPause();
        });
        resume.addEventListener('click', function (event) {
            userPaused = false;
            if (dialog.open) dialog.close();
            syncPause();
            if (soundOn && view) view.unlock(event);
            shell.focus({ preventScroll: true });
        });
        restart.addEventListener('click', beginRestart);
        winRestart.addEventListener('click', beginRestart);
        sound.addEventListener('click', function (event) {
            soundOn = !soundOn;
            if (view) {
                view.setMuted(!soundOn);
                if (soundOn && !paused()) view.unlock(event);
            }
            paintSound();
        });
        retry.addEventListener('click', function () { boot(); });
        dialog.addEventListener('cancel', function (event) {
            event.preventDefault();
            userPaused = false;
            dialog.close();
            syncPause();
        });
        shell.addEventListener('keydown', onKey);
        document.addEventListener('visibilitychange', syncPause);
        const observer = new root.ResizeObserver(layout);
        observer.observe(shell);
        saveTimer = root.setInterval(writeSession, 1000);
        sessions.set(windowId, function () {
            if (disposed) return;
            disposed = true;
            controller.abort();
            root.clearInterval(saveTimer);
            observer.disconnect();
            document.removeEventListener('visibilitychange', syncPause);
            writeSession();
            if (view) view.dispose();
            if (dialog.open) dialog.close();
        });
        boot();
    }

    function dispose(windowId) {
        const close = sessions.get(windowId);
        if (!close) return;
        sessions.delete(windowId);
        close();
    }

    root.LeuchtfeuerApp = { render: render, dispose: dispose };
})(typeof window !== 'undefined' ? window : globalThis);
