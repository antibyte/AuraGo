// HomeDash popover sheet (visual system 3; D5 list select). listbox(o) opens one non-modal listbox for a trigger: anchored below (above when no room)
// when the root is at least 600 px wide, a bottom sheet otherwise; one popover per root. APG listbox keys; a pointer press outside closes; focus returns
// to the trigger (Tab and Escape included). The panel is .hd-sheet.hd-widget[data-kind=listbox] with the trigger's family (SCR-M3-3). Geometry is read once
// at open. o = {root, anchor, options: [{id, label}], value, family, label, onPick(id), onClose()}. isOpen(root) lets a trigger toggle: a press on the trigger
// never closes the popover by itself, the trigger's own click decides.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.icons) throw new Error('hd-popover.js: load hd-core.js and hd-icons.js first');
    const { el, setAttr, setVars, uid } = E.core, OPEN = new WeakMap(), ROW = 44, PAD = 8, MAX_H = 352, NARROW = 600;
    function listbox(o) {
        const root = o.root, prev = OPEN.get(root); if (prev) prev.close(false);
        const id = uid('hd-lb'), life = E.core.bag(), doc = root.ownerDocument;
        const layer = Array.from(root.children).find(n => n.classList && n.classList.contains('hd-pop-layer')) || root.appendChild(el('div', { class: 'hd-pop-layer' }));
        const list = el('ul', { class: 'hd-listbox', role: 'listbox', id, tabindex: '0', 'aria-label': o.label || null });
        const items = o.options.map((x, i) => el('li', { class: 'hd-option', role: 'option', id: id + '-' + i, 'aria-selected': x.id === o.value ? 'true' : 'false', 'data-hd-option': x.id }, [E.icons.icon('check'), el('span', { class: 'hd-option-label', text: x.label })]));
        list.append(...items);
        const panel = el('div', { class: 'hd-sheet hd-widget', 'data-kind': 'listbox', 'data-hd-family': o.family || 'glass' }, list); layer.appendChild(panel);
        const rr = root.getBoundingClientRect(), ar = o.anchor.getBoundingClientRect(), narrow = rr.width < NARROW, h = Math.min(MAX_H, items.length * ROW + 2 * PAD), w = Math.min(rr.width - 16, Math.max(ar.width, 200));
        let y = ar.bottom - rr.top + 4; if (y + h > rr.height && ar.top - rr.top - 4 - h >= 0) y = ar.top - rr.top - 4 - h;
        const x = Math.max(8, Math.min(ar.left - rr.left, rr.width - w - 8));
        setAttr(panel, 'data-hd-pop', narrow ? 'sheet' : 'anchored');
        setVars(panel, { '--hd-pop-x': Math.round(x) + 'px', '--hd-pop-y': Math.round(y) + 'px', '--hd-pop-w': Math.round(narrow ? rr.width : w) + 'px', '--hd-pop-h': h + 'px' });
        let active = Math.max(0, o.options.findIndex(v => v.id === o.value)), done = false, typed = '', typedAt = 0;
        const activate = i => {
            active = Math.max(0, Math.min(items.length - 1, i)); setAttr(list, 'aria-activedescendant', items[active] ? items[active].id : null);
            items.forEach((n, k) => setAttr(n, 'data-hd-active', k === active));
            if (items[active] && typeof items[active].scrollIntoView === 'function') items[active].scrollIntoView({ block: 'nearest' });
        };
        const api = { el: panel, close };
        function close(refocus) {
            if (done) return; done = true; life.dispose(); panel.remove(); if (OPEN.get(root) === api) OPEN.delete(root);
            if (refocus !== false && o.anchor.focus) o.anchor.focus();
            if (o.onClose) o.onClose();
        }
        function pick(i) { const v = o.options[i]; if (!v) return; close(true); o.onPick(v.id); }
        life.on(list, 'keydown', ev => {
            const k = ev.key, nav = { ArrowDown: active + 1, ArrowUp: active - 1, Home: 0, End: items.length - 1 };
            if (Object.prototype.hasOwnProperty.call(nav, k)) { ev.preventDefault(); activate(nav[k]); return; }
            if (k === 'Enter' || k === ' ') { ev.preventDefault(); pick(active); return; }
            if (k === 'Escape') { ev.preventDefault(); ev.stopPropagation(); close(true); return; }
            if (k === 'Tab') { close(true); return; }   // no preventDefault: the Tab moves on from the trigger
            if (k.length === 1 && !ev.ctrlKey && !ev.metaKey && !ev.altKey) {
                const t = Date.now(); typed = (t - typedAt < 700 ? typed : '') + k.toLowerCase(); typedAt = t;
                const i = o.options.findIndex(v => v.label.toLowerCase().startsWith(typed)); if (i >= 0) activate(i);
            }
        });
        life.on(list, 'click', ev => { const n = ev.target && ev.target.closest ? ev.target.closest('.hd-option') : null; if (n) pick(items.indexOf(n)); });
        life.on(doc, 'pointerdown', ev => { if (!panel.contains(ev.target) && !o.anchor.contains(ev.target)) close(false); }, { capture: true });
        OPEN.set(root, api); activate(active); list.focus(); return api;
    }
    E.popover = Object.freeze({ listbox, isOpen: root => OPEN.has(root), close(root) { const p = OPEN.get(root); if (p) p.close(false); } });
})();
