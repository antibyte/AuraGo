/* Runenpfad cutscene overlay (global `RunenpfadCutsceneUi.create`): the DOM around the cutscene canvas.
 * Builds DOM with createElement/textContent only. Owns the overlay (canvas, title, typewriter subtitle, Next and Skip
 * buttons, progress pips), the keys (Enter, Space, ArrowRight advance; Escape skips; Tab stays inside), pointer and
 * tap advance, the music override and the UI sound cues. The overlay and every listener exist only while a play is
 * open. Options: { host, t, onDone, audio, music }; `audio.play(cue)` and `music.setOverride(theme|null)` are optional. */
(function (root) {
    'use strict';

    const GUARD_MS = 300;       // ignore advance input right after opening (the key that started the stage)
    const MAX_PX_W = 1920;
    const MAX_PX_H = 1080;

    function el(tag, cls, text) {
        const n = root.document.createElement(tag);
        if (cls) n.className = cls;
        if (text != null) n.textContent = text;
        return n;
    }

    function create(options) {
        const opt = options || {};
        const host = opt.host || null;
        const t = typeof opt.t === 'function' ? opt.t : function (k) { return k; };
        const perf = root.performance;

        let disposed = false;
        let open = false;
        let st = null;
        let ids = [];
        let doneCb = null;
        let ov = null;
        let canvas = null;
        let view = null;
        let titleEl = null;
        let srEl = null;
        let shownEl = null;
        let restEl = null;
        let nextBtn = null;
        let skipBtn = null;
        let pipsEl = null;
        let ro = null;
        let raf = 0;
        let openedAt = 0;
        let lastNow = 0;
        let driven = false;      // true once the app calls frame() itself
        let prevFocus = null;
        let lineKey = '';
        let sceneId = '';
        let chars = [];
        let shown = -1;
        let panelsBefore = [];
        let reduceCache = false;

        function C() { return root.RunenpfadCutscenes || null; }

        function reduced() {
            return !!((root.document.body && root.document.body.dataset && root.document.body.dataset.animations === 'false') ||
                (typeof root.matchMedia === 'function' && root.matchMedia('(prefers-reduced-motion: reduce)').matches));
        }

        function cue(name) {
            try { if (opt.audio && typeof opt.audio.play === 'function') opt.audio.play(name); } catch (err) { /* sound is optional */ }
        }

        function setMusic(theme) {
            try { if (opt.music && typeof opt.music.setOverride === 'function') opt.music.setOverride(theme || null); } catch (err) { /* music is optional */ }
        }

        // ---- layout ----
        function fit() {
            if (!ov || !canvas) return;
            const cw = canvas.clientWidth;
            const ch = canvas.clientHeight;
            if (!(cw > 0 && ch > 0)) return;
            const dpr = Math.min(root.devicePixelRatio || 1, 2);
            const k = Math.min(1, MAX_PX_W / (cw * dpr), MAX_PX_H / (ch * dpr));
            const w = Math.max(2, Math.round(cw * dpr * k));
            const h = Math.max(2, Math.round(ch * dpr * k));
            if (canvas.width !== w) canvas.width = w;
            if (canvas.height !== h) canvas.height = h;
        }

        function buildPips() {
            pipsEl.replaceChildren();
            let total = 0;
            panelsBefore = [];
            const cut = C();
            ids.forEach(function (id) {
                panelsBefore.push(total);
                total += cut && cut.SCENES[id] ? cut.SCENES[id].panels.length : 0;
            });
            for (let i = 0; i < total; i++) pipsEl.append(el('i'));
        }

        function paintPips() {
            const idx = (panelsBefore[st.sceneIndex] || 0) + st.panelIndex;
            const list = pipsEl.children;
            for (let i = 0; i < list.length; i++) list[i].className = i < idx ? 'is-done' : (i === idx ? 'is-on' : '');
            pipsEl.setAttribute('aria-label', (idx + 1) + ' / ' + list.length);
        }

        // ---- text ----
        function setTyped(n) {
            if (n === shown) return;
            shown = n;
            shownEl.textContent = chars.slice(0, n).join('');
            restEl.textContent = chars.slice(n).join('');
            ov.classList.toggle('is-typing', n < chars.length);
        }

        // Re-reads the current line from the player state when it changed (new line, panel or scene).
        function syncLine() {
            const cut = C();
            const cur = cut ? cut.current(st) : null;
            if (!cur) return;
            const key = cur.scene.id + '|' + st.panelIndex + '|' + st.lineIndex;
            if (key === lineKey) return;
            lineKey = key;
            if (cur.scene.id !== sceneId) {
                sceneId = cur.scene.id;
                titleEl.textContent = t(cur.scene.titleKey);
                ov.setAttribute('aria-label', titleEl.textContent);
                setMusic(cur.scene.music);
            }
            const full = t(cur.lineKey);
            chars = Array.from(full);
            srEl.textContent = full;
            cut.setLineLength(st, chars.length);
            if (reduceCache) cut.completeLine(st);
            shown = -1;
            setTyped(Math.min(st.chars, chars.length));
            paintPips();
        }

        // ---- flow ----
        function tick(now) {
            if (!open || !st) return;
            const cut = C();
            if (!cut) return;
            const dt = lastNow ? (now - lastNow) / 1000 : 0;
            lastNow = now;
            reduceCache = reduced();
            cut.step(st, dt);
            if (reduceCache) cut.completeLine(st);
            syncLine();
            setTyped(Math.min(st.chars, chars.length));
            if (view) view.render(st, now, reduceCache);
        }

        function loop(ts) {
            raf = 0;
            if (!open) return;
            if (!driven) tick(ts);
            raf = root.requestAnimationFrame(loop);
        }

        function userAdvance() {
            if (!open || !st) return;
            if (perf.now() - openedAt < GUARD_MS) return;
            const cut = C();
            const r = cut.advance(st);
            if (r === 'done') {
                finish(false);
                return;
            }
            if (r === 'line' || r === 'panel' || r === 'scene') cue('cut_next');
            syncLine();
            setTyped(Math.min(st.chars, chars.length));
        }

        function userSkip() {
            if (!open) return;
            finish(true);
        }

        // ---- input ----
        function onKey(e) {
            if (!open) return;
            const k = e.key;
            const onBtn = !!(e.target && typeof e.target.closest === 'function' && e.target.closest('.rp-cut-btn'));
            if (k === 'Enter' || k === ' ' || k === 'Spacebar' || k === 'ArrowRight') {
                e.stopPropagation();
                if (onBtn && (k === 'Enter' || k === ' ' || k === 'Spacebar')) return; // the button's own click does the work
                e.preventDefault();
                if (!e.repeat) userAdvance();
                return;
            }
            e.stopPropagation();
            if (k === 'Escape') {
                e.preventDefault();
                if (!e.repeat) userSkip();
            } else if (k === 'Tab') {
                e.preventDefault();
                const target = root.document.activeElement === nextBtn ? skipBtn : nextBtn;
                target.focus({ preventScroll: true });
            }
        }

        function onPointer(e) {
            if (!open) return;
            if (e.pointerType === 'mouse' && e.button > 0) return;
            if (e.target && typeof e.target.closest === 'function' && e.target.closest('.rp-cut-btn')) return;
            userAdvance();
        }

        function onFocusIn(e) {
            if (open && ov && !ov.contains(e.target)) ov.focus({ preventScroll: true });
        }

        function onResize() { fit(); }

        // ---- open and close ----
        function teardown() {
            open = false;
            if (raf && typeof root.cancelAnimationFrame === 'function') root.cancelAnimationFrame(raf);
            raf = 0;
            const doc = root.document;
            doc.removeEventListener('keydown', onKey, true);
            doc.removeEventListener('focusin', onFocusIn, true);
            if (ro) { try { ro.disconnect(); } catch (err) { /* ignore */ } ro = null; }
            if (root.removeEventListener) root.removeEventListener('resize', onResize);
            if (view) { try { view.dispose(); } catch (err) { /* ignore */ } view = null; }
            if (ov) {
                ov.removeEventListener('pointerdown', onPointer);
                if (ov.parentNode) ov.parentNode.removeChild(ov);
            }
            const back = prevFocus;
            prevFocus = null;
            ov = canvas = titleEl = srEl = shownEl = restEl = nextBtn = skipBtn = pipsEl = null;
            if (back && typeof back.focus === 'function' && doc.contains(back)) { try { back.focus({ preventScroll: true }); } catch (err) { /* ignore */ } }
        }

        function finish(skipped) {
            if (!open) return;
            const list = ids.slice();
            const cb = doneCb;
            doneCb = null;
            teardown();
            setMusic(null);
            cue('cut_close');
            st = null;
            if (typeof cb === 'function') {
                try { cb(list, { skipped: !!skipped }); } catch (err) { /* the caller's problem must not leave the overlay open */ }
            }
        }

        function play(sceneIds, onDone) {
            if (disposed || !root.document) return;
            if (open) finish(true);
            const cut = C();
            const cb = typeof onDone === 'function' ? onDone : opt.onDone;
            st = cut ? cut.create(sceneIds) : null;
            if (!st || st.done || !host) {
                st = null;
                if (typeof cb === 'function') { try { cb([], { skipped: false }); } catch (err) { /* ignore */ } }
                return;
            }
            ids = st.ids.slice();
            doneCb = cb;
            const doc = root.document;
            ov = el('div', 'rp-cut');
            if (host === doc.body || host === doc.documentElement) ov.classList.add('is-fixed');
            ov.tabIndex = -1;
            ov.setAttribute('role', 'dialog');
            ov.setAttribute('aria-modal', 'true');
            canvas = el('canvas', 'rp-cut-canvas');
            canvas.width = 640;
            canvas.height = 360;
            canvas.setAttribute('aria-hidden', 'true');
            const top = el('div', 'rp-cut-top');
            titleEl = el('span', 'rp-cut-title');
            skipBtn = el('button', 'rp-cut-btn rp-cut-skip', t('runenpfad.cut.skip'));
            skipBtn.type = 'button';
            top.append(titleEl, skipBtn);
            const sub = el('div', 'rp-cut-sub');
            const text = el('p', 'rp-cut-text');
            text.setAttribute('aria-live', 'polite');
            srEl = el('span', 'rp-cut-sr');
            shownEl = el('span', 'rp-cut-shown');
            restEl = el('span', 'rp-cut-rest');
            shownEl.setAttribute('aria-hidden', 'true');
            restEl.setAttribute('aria-hidden', 'true');
            text.append(srEl, shownEl, restEl);
            const bar = el('div', 'rp-cut-bar');
            pipsEl = el('span', 'rp-cut-pips');
            pipsEl.setAttribute('role', 'img');
            nextBtn = el('button', 'rp-cut-btn rp-cut-next', t('runenpfad.cut.next'));
            nextBtn.type = 'button';
            bar.append(pipsEl, nextBtn);
            sub.append(text, bar);
            ov.append(canvas, top, sub);

            skipBtn.addEventListener('click', userSkip);
            nextBtn.addEventListener('click', userAdvance);
            ov.addEventListener('pointerdown', onPointer);
            doc.addEventListener('keydown', onKey, true);
            doc.addEventListener('focusin', onFocusIn, true);
            prevFocus = doc.activeElement && doc.activeElement !== doc.body ? doc.activeElement : null;

            host.append(ov);
            open = true;
            driven = false;
            lastNow = 0;
            lineKey = '';
            sceneId = '';
            chars = [];
            shown = -1;
            openedAt = perf.now();
            reduceCache = reduced();
            buildPips();
            const A = root.RunenpfadCutsceneView;
            try { view = A ? A.create(canvas, { t: t }) : null; } catch (err) { view = null; }
            if (view) view.prime(ids);
            if (typeof root.ResizeObserver === 'function') {
                ro = new root.ResizeObserver(onResize);
                ro.observe(ov);
            } else if (root.addEventListener) {
                root.addEventListener('resize', onResize);
            }
            fit();
            syncLine();
            tick(perf.now());
            cue('cut_open');
            try { ov.focus({ preventScroll: true }); } catch (err) { /* ignore */ }
            // fade in on the next frame (transition only, no CSS animation)
            if (typeof root.requestAnimationFrame === 'function') {
                const shell = ov;
                root.requestAnimationFrame(function () { if (ov === shell) shell.classList.add('is-open'); });
                raf = root.requestAnimationFrame(loop);
            } else {
                ov.classList.add('is-open');
            }
        }

        // Called by the app every animation frame while open; without it the overlay drives itself.
        function frame(nowMs) {
            if (!open) return;
            if (!driven) { driven = true; lastNow = 0; }
            tick(+nowMs || perf.now());
        }

        function close() { finish(true); }

        function dispose() {
            disposed = true;
            doneCb = null;
            if (open) {
                teardown();
                setMusic(null);
                st = null;
            }
        }

        return { play: play, isOpen: function () { return open; }, frame: frame, close: close, dispose: dispose };
    }

    root.RunenpfadCutsceneUi = { create: create };
})(typeof window !== 'undefined' ? window : globalThis);
