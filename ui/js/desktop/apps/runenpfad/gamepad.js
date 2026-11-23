/* Runenpfad gamepad bridge (global `RunenpfadGamepad.create`): reads the first standard-mapping pad and feeds the
 * app real key events through `press(code)` / `release(code)`. While a dialog is open the pad walks its focus
 * instead. Options: { root, press, release, activeDialog(), cardOpen(), reduced() }. Never throws. */
(function (root) {
    'use strict';

    const DEAD = 0.35;
    const UP_DEAD = 0.7;
    const GAME_KEYS = ['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Space', 'KeyJ', 'KeyK', 'KeyQ', 'KeyE', 'KeyP'];
    // Standard mapping: A jump, B special, X attack, Y heal, LB / RB talk, Start pause.
    const BUTTON_KEYS = { 0: 'Space', 1: 'KeyK', 2: 'KeyJ', 3: 'KeyQ', 4: 'KeyE', 5: 'KeyE', 9: 'KeyP', 12: 'ArrowUp', 13: 'ArrowDown', 14: 'ArrowLeft', 15: 'ArrowRight' };
    const RUMBLE = { player_hurt: [0.6, 140], slam_land: [0.8, 180], boss_slam: [0.8, 180] };
    const FOCUSABLE = 'button:not(:disabled), input:not(:disabled), select:not(:disabled)';

    function create(o) {
        o = o || {};
        const host = o.root || null;
        let held = Object.create(null); // game keys currently pressed through the app
        let prev = Object.create(null); // logical state of the last poll (edge detection)
        let hold = Object.create(null); // keys swallowed until they are released (a click must not also jump)
        let pad = null;
        let sawPad = false;
        let wasLocked = false;
        let onConnect = null;

        function nav() { return root.navigator || (typeof navigator !== 'undefined' ? navigator : null); }
        function doc() { return root.document || (typeof document !== 'undefined' ? document : null); }

        function fire(fn, code) {
            try { if (typeof fn === 'function') fn(code); } catch (err) { /* a bad handler must not stop the game */ }
        }

        function firstPad() {
            let list = null;
            try { const n = nav(); list = n && typeof n.getGamepads === 'function' ? n.getGamepads() : null; } catch (err) { list = null; }
            if (!list) return null;
            for (let i = 0; i < list.length; i++) {
                const p = list[i];
                if (p && p.connected !== false && p.mapping === 'standard') return p;
            }
            return null;
        }

        // The logical keys the pad asks for right now.
        function sample(p) {
            const cur = Object.create(null);
            const ax = p.axes || [];
            const x = Number(ax[0]) || 0;
            const y = Number(ax[1]) || 0;
            const btn = p.buttons || [];
            Object.keys(BUTTON_KEYS).forEach(function (i) {
                const b = btn[i];
                if (b && (b.pressed || b.value > 0.5)) cur[BUTTON_KEYS[i]] = true;
            });
            if (x < -DEAD) cur.ArrowLeft = true;
            if (x > DEAD) cur.ArrowRight = true;
            if (y > DEAD) cur.ArrowDown = true;
            // Up is jump, talk and climb at once: only a clear, mostly vertical push counts.
            if (y < -UP_DEAD && Math.abs(y) > Math.abs(x)) cur.ArrowUp = true;
            return cur;
        }

        function releaseAll() {
            Object.keys(held).forEach(function (k) { if (held[k]) fire(o.release, k); });
            held = Object.create(null);
        }

        function tap(code) { fire(o.press, code); fire(o.release, code); }

        function focusables(dialog) {
            return Array.prototype.filter.call(dialog.querySelectorAll(FOCUSABLE), function (n) {
                return !n.hidden && !(n.closest && n.closest('[hidden]'));
            });
        }

        function step(dialog, dir, horizontal) {
            const d = doc();
            const cur = d ? d.activeElement : null;
            if (horizontal && cur && cur.type === 'range') {
                const stepBy = Number(cur.step) || 5;
                const lo = Number(cur.min) || 0;
                const hi = cur.max === '' || cur.max === undefined ? 100 : Number(cur.max);
                cur.value = String(Math.min(hi, Math.max(lo, (Number(cur.value) || 0) + dir * stepBy)));
                if (typeof cur.dispatchEvent === 'function' && typeof root.Event === 'function') {
                    cur.dispatchEvent(new root.Event('input', { bubbles: true }));
                    cur.dispatchEvent(new root.Event('change', { bubbles: true }));
                }
                return;
            }
            const list = focusables(dialog);
            if (!list.length) return;
            const at = list.indexOf(cur);
            const next = at < 0 ? (dir > 0 ? 0 : list.length - 1) : (at + dir + list.length) % list.length;
            list[next].focus({ preventScroll: true });
        }

        function dialogInput(dialog, cur) {
            const edge = function (k) { return cur[k] && !prev[k]; };
            if (edge('ArrowLeft')) step(dialog, -1, true);
            if (edge('ArrowRight')) step(dialog, 1, true);
            if (edge('ArrowUp')) step(dialog, -1, false);
            if (edge('ArrowDown')) step(dialog, 1, false);
            if (edge('Space')) {
                const d = doc();
                if (d && d.activeElement && typeof d.activeElement.click === 'function') d.activeElement.click();
            }
            if (edge('KeyK')) tap('Escape');
            if (edge('KeyP')) tap('KeyP');
        }

        function poll() {
            try {
                pad = firstPad();
                if (!pad) {
                    releaseAll();
                    prev = Object.create(null);
                    return;
                }
                sawPad = true;
                const cur = sample(pad);
                const dialog = typeof o.activeDialog === 'function' ? o.activeDialog() : null;
                const card = !dialog && typeof o.cardOpen === 'function' && !!o.cardOpen();
                const locked = !!dialog || card;
                if (dialog) {
                    releaseAll();
                    dialogInput(dialog, cur);
                    GAME_KEYS.forEach(function (k) { if (cur[k]) hold[k] = true; });
                } else {
                    if (wasLocked) GAME_KEYS.forEach(function (k) { if (cur[k]) hold[k] = true; });
                    GAME_KEYS.forEach(function (k) {
                        if (!cur[k]) hold[k] = false;
                        const want = !!cur[k] && !hold[k];
                        if (card && k === 'KeyK' && cur[k] && !prev[k]) tap('Escape'); // a round card closes with Escape
                        if (want && !held[k]) { held[k] = true; fire(o.press, k); }
                        else if (!want && held[k]) { held[k] = false; fire(o.release, k); }
                    });
                }
                wasLocked = locked;
                prev = cur;
            } catch (err) { /* never throw out of the frame loop */ }
        }

        function onEvents(events) {
            try {
                if (!events || !pad || (typeof o.reduced === 'function' && o.reduced())) return;
                const act = pad.vibrationActuator;
                if (!act || typeof act.playEffect !== 'function') return;
                let best = null;
                for (let i = 0; i < events.length; i++) {
                    const r = RUMBLE[events[i] && events[i].type];
                    if (r && (!best || r[0] > best[0])) best = r;
                }
                if (!best) return;
                const res = act.playEffect('dual-rumble', { startDelay: 0, duration: best[1], weakMagnitude: best[0] * 0.5, strongMagnitude: best[0] });
                if (res && typeof res.catch === 'function') res.catch(function () {});
            } catch (err) { /* rumble is a bonus */ }
        }

        if (host && typeof host.addEventListener === 'function' && root.addEventListener) {
            onConnect = function () { sawPad = true; };
            root.addEventListener('gamepadconnected', onConnect);
        }

        function dispose() {
            releaseAll();
            if (onConnect && root.removeEventListener) root.removeEventListener('gamepadconnected', onConnect);
            onConnect = null;
        }

        return { poll: poll, onEvents: onEvents, seen: function () { return sawPad; }, dispose: dispose };
    }

    root.RunenpfadGamepad = { create: create };
})(typeof window !== 'undefined' ? window : globalThis);
