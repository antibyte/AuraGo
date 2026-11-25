// HomeDash input (P1, decision D5; design-frontend 6, 10). Pure maths for keyboard and pointer values, and binders on pointer events (never HTML5
// drag and drop). press(): buttons (activation through the native click, so Enter and Space work; 500 ms hold; a move of 4 px or more swallows the
// click). gesture(): draggable controls (pointer capture, 4 px between tap and drag, hold). keys(): Arrow +-step, PageUp/PageDown +-10 % of the range
// (at least one step), Home/End, Enter/Space toggle where a control has one, Shift+Enter and ContextMenu run hold. repeat(): one step per click,
// repeats while held (400 ms, then 80 ms). Primary pointer only; a mouse gesture also ends on a move without a pressed button. touch-action is CSS.
// Values snap to a grid anchored at min that never passes max (contract M3 section 4). Timers, clock and window are injectable.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core) throw new Error('hd-input.js: load hd-core.js first');
    const THRESHOLD_PX = 4, HOLD_MS = 500, REPEAT_FIRST_MS = 400, REPEAT_MS = 80, SWALLOW_MS = 400, DEAD_ZONE_DEG = 15, DIAL_PX = 220;
    const finite = v => typeof v === 'number' && Number.isFinite(v);
    const places = n => { const s = String(n), i = s.indexOf('.'); return i < 0 ? 0 : Math.min(6, s.length - i - 1); };
    function range(r) { const x = r || {}, min = finite(x.min) ? x.min : 0, max = finite(x.max) && x.max > min ? x.max : min + 100, step = finite(x.step) && x.step > 0 ? x.step : 1;
        return Object.freeze({ min, max, step, top: min + Math.floor((max - min) / step + 1e-9) * step, dp: Math.max(places(step), places(min)) }); }
    const fix = (v, dp) => Number(v.toFixed(dp));
    function snap(value, r) { const g = range(r); if (!finite(value)) return null; const k = Math.round((Math.min(g.max, Math.max(g.min, value)) - g.min) / g.step); return fix(Math.min(g.top, g.min + k * g.step), g.dp); }
    function page(r) { const g = range(r); return Math.max(g.step, Math.round((g.max - g.min) / 10 / g.step) * g.step); }
    function keyValue(key, value, r) { const g = range(r), v = finite(value) ? value : g.min;
        switch (key) { case 'ArrowRight': case 'ArrowUp': return snap(v + g.step, g); case 'ArrowLeft': case 'ArrowDown': return snap(v - g.step, g);
            case 'PageUp': return snap(v + page(g), g); case 'PageDown': return snap(v - page(g), g); case 'Home': return fix(g.min, g.dp); case 'End': return fix(g.top, g.dp); default: return null; } }
    function fraction(x, y, rect, axis) { if (!rect) return 0; const f = axis === 'y' ? (rect.height > 0 ? (rect.top + rect.height - y) / rect.height : 0) : (rect.width > 0 ? (x - rect.left) / rect.width : 0); return Math.min(1, Math.max(0, f)); }
    const valueAt = (f, r) => { const g = range(r); return snap(g.min + Math.min(1, Math.max(0, f)) * (g.max - g.min), g); };
    // Angles from 12 o'clock clockwise; the arc starts at `start` (-135) and sweeps `sweep` (270). Within 15 deg past an end the value pins to it; deeper in the gap the last fraction stays.
    function dialFraction(x, y, rect, last, start, sweep) { const s0 = finite(start) ? start : -135, sw = finite(sweep) ? sweep : 270, cx = rect.left + rect.width / 2, cy = rect.top + rect.height / 2;
        const rel = ((Math.atan2(x - cx, cy - y) * 180 / Math.PI + 360) % 360 - s0 + 720) % 360;
        if (rel <= sw) return rel / sw; if (rel - sw <= DEAD_ZONE_DEG) return 1; if (360 - rel <= DEAD_ZONE_DEG) return 0; return finite(last) ? last : (rel - sw < 360 - rel ? 1 : 0); }
    const verticalFraction = (from, dy) => Math.min(1, Math.max(0, from - dy / DIAL_PX));
    const isHoldKey = ev => !!ev && ((ev.key === 'Enter' && ev.shiftKey) || ev.key === 'ContextMenu');
    const stop = ev => { if (ev && typeof ev.preventDefault === 'function') ev.preventDefault(); };
    const eat = ev => { stop(ev); if (ev && typeof ev.stopImmediatePropagation === 'function') ev.stopImmediatePropagation(); };   // a swallowed click reaches no other listener on the element
    const primary = ev => ev.isPrimary !== false && !(ev.pointerType === 'mouse' && ev.button > 0);
    function env(el, o) { const doc = el.ownerDocument; return { win: o.win || (doc && doc.defaultView) || el, setT: o.setTimeout || ((fn, ms) => setTimeout(fn, ms)), clearT: o.clearTimeout || (id => clearTimeout(id)), now: o.now || (() => Date.now()), life: E.core.bag() }; }
    const allowed = (o, ev) => { if (!o.enabled || o.enabled()) return true; if (o.onBlocked) o.onBlocked(ev); return false; };
    function press(el, opts) { const o = opts || {}, ctx = env(el, o); let g = null, swallowAt = -Infinity, heldAt = -Infinity;
        function drop() { const s = g; g = null; if (s) { if (s.timer !== null) ctx.clearT(s.timer); s.bag.dispose(); } return s; }
        function end(cancelled) { const s = drop(); if (s && !cancelled && (s.moved || s.held)) swallowAt = ctx.now(); }   // a cancel is followed by no click
        function hold(ev) { heldAt = ctx.now(); o.onHold(ev); } // never gated by enabled: more-info works when read-only (rule 11)
        ctx.life.on(el, 'pointerdown', ev => { if (!primary(ev)) return; drop(); swallowAt = -Infinity; // a stale gesture (lost pointerup) ends silently
            g = { id: ev.pointerId, type: ev.pointerType, x: ev.clientX, y: ev.clientY, moved: false, held: false, timer: null, bag: E.core.bag() };
            const s = g, cap = { capture: true }, mine = e => g === s && e.pointerId === s.id;
            if (o.onHold) s.timer = ctx.setT(() => { s.timer = null; if (g === s && !s.moved) { s.held = true; hold(null); } }, HOLD_MS);
            s.bag.on(ctx.win, 'pointermove', e => { if (!mine(e)) return;
                if (s.type === 'mouse' && e.buttons === 0) { drop(); return; } // the button came up outside the window: no click follows
                if (!s.moved && Math.hypot(e.clientX - s.x, e.clientY - s.y) >= THRESHOLD_PX) { s.moved = true; if (s.timer !== null) { ctx.clearT(s.timer); s.timer = null; } } }, cap);
            s.bag.on(ctx.win, 'pointerup', e => { if (mine(e)) end(false); }, cap);
            s.bag.on(ctx.win, 'pointercancel', e => { if (mine(e)) end(true); }, cap); });
        ctx.life.on(el, 'click', ev => { if (ctx.now() - swallowAt < SWALLOW_MS) { swallowAt = -Infinity; eat(ev); return; } // swallow one click only
            if (o.onActivate && allowed(o, ev)) o.onActivate(ev); }, { capture: true });
        ctx.life.on(el, 'keydown', ev => { if (ev.defaultPrevented === true || ev.altKey || ev.ctrlKey || ev.metaKey || !o.onHold || !isHoldKey(ev)) return; stop(ev); hold(ev); });
        ctx.life.on(el, 'contextmenu', ev => { if ((g && g.type !== 'mouse') || ctx.now() - heldAt < SWALLOW_MS) stop(ev); });
        ctx.life.add(drop);
        return () => ctx.life.dispose(); }
    // gesture() binds no keyboard: keys() owns it (pass onHold to keys() too, e.g. the light tile and bindLinear), so Shift+Enter runs hold once.
    function gesture(el, opts) { const o = opts || {}, ctx = env(el, o); let g = null, swallowAt = -Infinity;
        const rect = () => (o.rect ? o.rect() : typeof el.getBoundingClientRect === 'function' ? el.getBoundingClientRect() : null);
        const point = ev => ({ x: ev.clientX, y: ev.clientY, rect: rect(), type: ev.type });
        function drop() { const s = g; g = null; if (!s) return null;
            if (s.timer !== null) ctx.clearT(s.timer); s.bag.dispose(); if (s.drag) el.removeAttribute('data-hd-dragging');
            if (s.captured && typeof el.releasePointerCapture === 'function') { try { el.releasePointerCapture(s.id); } catch (err) { /* released with the pointerup */ } }
            return s; }
        // how: 'up'; 'cancel' (pointercancel or a replaced gesture; its coordinates may be 0/0, so a drag ends at the last move point);
        // 'lost' (mouse move without a button: the up happened outside the window, no click follows). A started drag always ends in onDragEnd (rule 6 commits).
        function finish(ev, how) { const s = drop(); if (!s) return; const cancelled = how === 'cancel';
            if (how === 'up' && (s.held || s.moved)) swallowAt = ctx.now();   // 'cancel' and 'lost' are followed by no click
            if (s.drag) { if (o.onDragEnd) o.onDragEnd(cancelled ? s.last : point(ev), s.start, cancelled); return; }
            if (how === 'up' && !s.held && !s.moved && o.onTap && allowed(o, ev)) o.onTap(s.start); }
        ctx.life.on(el, 'pointerdown', ev => { if (!primary(ev)) return; if (g) finish(null, 'cancel'); swallowAt = -Infinity;
            if (ev.pointerType === 'mouse') { stop(ev); if (typeof el.focus === 'function') el.focus({ preventScroll: true }); }
            const start = point(ev);
            g = { id: ev.pointerId, type: ev.pointerType, start, last: start, moved: false, held: false, drag: false, captured: false, timer: null, bag: E.core.bag() };
            const s = g, cap = { capture: true }, mine = e => g === s && e.pointerId === s.id;
            if (typeof el.setPointerCapture === 'function') { try { el.setPointerCapture(ev.pointerId); s.captured = true; } catch (err) { /* synthetic pointer: the window listeners carry it */ } }
            if (o.onHold) s.timer = ctx.setT(() => { s.timer = null; if (g === s && !s.moved) { s.held = true; o.onHold(null); } }, HOLD_MS);
            s.bag.on(ctx.win, 'pointermove', e => { if (!mine(e)) return;
                if (s.type === 'mouse' && e.buttons === 0) { finish(e, 'lost'); return; }
                if (s.held) return;
                if (!s.moved) { if (Math.hypot(e.clientX - start.x, e.clientY - start.y) < THRESHOLD_PX) return;
                    s.moved = true; if (s.timer !== null) { ctx.clearT(s.timer); s.timer = null; }
                    s.drag = allowed(o, e); if (s.drag) { el.setAttribute('data-hd-dragging', 'true'); if (o.onDragStart) o.onDragStart(start); } }
                if (s.drag && g === s) { s.last = point(e); if (o.onDrag) o.onDrag(s.last, start); } }, cap);
            s.bag.on(ctx.win, 'pointerup', e => { if (mine(e)) finish(e, 'up'); }, cap);
            s.bag.on(ctx.win, 'pointercancel', e => { if (mine(e)) finish(e, 'cancel'); }, cap); });
        ctx.life.on(el, 'click', ev => { if (ctx.now() - swallowAt < SWALLOW_MS) { swallowAt = -Infinity; eat(ev); } }, { capture: true });
        ctx.life.add(drop);
        return () => ctx.life.dispose(); }
    function keys(el, opts) { const o = opts || {}, life = E.core.bag();
        life.on(el, 'keydown', ev => {
            if (ev.defaultPrevented === true || ev.altKey || ev.ctrlKey || ev.metaKey) return;
            if (isHoldKey(ev)) { if (o.onHold) { stop(ev); o.onHold(ev); } return; }
            if ((ev.key === 'Enter' || ev.key === ' ') && o.onToggle) { stop(ev); if (allowed(o, ev)) o.onToggle(ev); return; }
            if (!o.range || !o.onKey) return;
            const cur = o.get ? o.get() : null, next = keyValue(ev.key, cur, o.range()); if (next === null) return;
            stop(ev); if (allowed(o, ev) && next !== cur) o.onKey(next, ev.key); });
        return () => life.dispose(); }
    function repeat(el, opts) { const o = opts || {}, ctx = env(el, o); let g = null, swallowAt = -Infinity;
        function halt(swallow) { const s = g; g = null; if (!s) return; if (s.timer !== null) ctx.clearT(s.timer); s.bag.dispose(); if (swallow && s.repeated) swallowAt = ctx.now(); }
        ctx.life.on(el, 'pointerdown', ev => { if (!primary(ev)) return; halt(false); swallowAt = -Infinity;
            g = { id: ev.pointerId, type: ev.pointerType, repeated: false, timer: null, bag: E.core.bag() };
            const s = g, cap = { capture: true }, mine = e => g === s && e.pointerId === s.id;
            const tick = () => { s.timer = null; if (g !== s) return; s.repeated = true; // set first: the click after release is swallowed even when blocked
                if (!o.onStep || !allowed(o, null)) return; // blocked: announced once, the chain stops
                o.onStep(); if (g === s) s.timer = ctx.setT(tick, REPEAT_MS); };
            s.timer = ctx.setT(tick, REPEAT_FIRST_MS);
            s.bag.on(ctx.win, 'pointerup', e => { if (mine(e)) halt(true); }, cap);
            s.bag.on(ctx.win, 'pointercancel', e => { if (mine(e)) halt(false); }, cap);   // no click follows a cancel
            s.bag.on(ctx.win, 'pointermove', e => { if (mine(e) && s.type === 'mouse' && e.buttons === 0) halt(false); }, cap);
            s.bag.on(el, 'pointerleave', e => { if (mine(e)) halt(false); }); }); // the up lands elsewhere: no click to swallow
        ctx.life.on(el, 'click', ev => { if (ctx.now() - swallowAt < SWALLOW_MS) { swallowAt = -Infinity; eat(ev); return; } if (o.onStep && allowed(o, ev)) o.onStep(); }, { capture: true });
        ctx.life.on(el, 'contextmenu', ev => { if (g && g.type !== 'mouse') stop(ev); });
        ctx.life.add(() => halt(false));
        return () => ctx.life.dispose(); }
    E.input = Object.freeze({ THRESHOLD_PX, HOLD_MS, REPEAT_FIRST_MS, REPEAT_MS, DEAD_ZONE_DEG, DIAL_PX, range, snap, page, keyValue, fraction, valueAt, dialFraction, verticalFraction, isHoldKey, press, gesture, keys, repeat });
})();
