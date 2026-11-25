// HomeDash in-root modal sheet (F2; design-frontend 6). create(container) builds one E.prim.sheet layer inside `container`.
// open({title, text: [paragraphs], actions: [{id, label, tone, autofocus}], kind}) resolves the chosen action id, or null for Escape, a click
// on the backdrop or close(). While open, the container's other children are inert, Tab and Shift+Tab stay in the dialog, and focus returns to
// the element that had it. The confirm helper (title, text, confirmLabel, cancelLabel, tone) resolves true or false; a danger one starts on Cancel.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.prim || !E.prim.sheet) throw new Error('hd-sheet.js: load hd-core.js and hd-prim-displays.js first');
    const { el, setAttr } = E.core, FOCUSABLE = 'button, [href], input, select, textarea, [tabindex]';
    function create(container) {
        const doc = container.ownerDocument, sheet = E.prim.sheet({ kind: 'confirm' }), life = E.core.bag(), dialog = sheet.el.querySelector('.hd-sheet');
        container.appendChild(sheet.el);
        let resolveOpen = null, opener = null, inerted = [];
        const focusables = () => Array.from(sheet.el.querySelectorAll(FOCUSABLE)).filter(n => !n.hasAttribute('disabled') && n.getAttribute('tabindex') !== '-1');
        function close(value) {
            if (!resolveOpen) return;
            const done = resolveOpen; resolveOpen = null;
            sheet.set({ open: false });
            for (const n of inerted) n.removeAttribute('inert'); inerted = [];
            sheet.foot.replaceChildren(); sheet.body.replaceChildren();
            if (opener && typeof opener.focus === 'function' && opener.isConnected !== false) opener.focus();
            opener = null; done(value === undefined ? null : value);
        }
        function open(spec) {
            close(null);
            const s = spec || {};
            sheet.set({ open: true, title: s.title || '' }); setAttr(dialog, 'data-kind', s.kind || 'confirm');
            for (const line of [].concat(s.text || [])) sheet.body.appendChild(el('p', { class: 'hd-sheet-text', text: line }));
            let first = null;
            for (const a of s.actions || []) {
                const b = el('button', { type: 'button', class: 'hd-sheet-btn', 'data-action': a.id, 'data-tone': a.tone || 'default', text: a.label });
                sheet.foot.appendChild(b); if (a.autofocus || !first) first = b;
            }
            opener = doc.activeElement || null;
            inerted = Array.from(container.children).filter(n => n !== sheet.el && !n.hasAttribute('inert'));
            for (const n of inerted) n.setAttribute('inert', '');
            return new Promise(resolve => { resolveOpen = resolve; if (first) first.focus(); });
        }
        life.on(sheet.el, 'click', ev => {
            const b = ev.target && ev.target.closest ? ev.target.closest('.hd-sheet-btn') : null;
            if (b) close(b.getAttribute('data-action')); else if (ev.target === sheet.el) close(null);
        });
        life.on(sheet.el, 'keydown', ev => {
            if (!resolveOpen) return;
            if (ev.key === 'Escape') { ev.preventDefault(); ev.stopPropagation(); close(null); return; }
            if (ev.key !== 'Tab') return;
            const list = focusables(); if (!list.length) return;
            const i = list.indexOf(doc.activeElement), next = ev.shiftKey ? (i <= 0 ? list.length - 1 : i - 1) : (i === list.length - 1 ? 0 : i + 1);
            ev.preventDefault(); list[next].focus();
        });
        return Object.freeze({
            el: sheet.el, open, close, isOpen: () => !!resolveOpen,
            confirm: spec => open({ title: spec.title, text: spec.text, kind: 'confirm', actions: [{ id: 'cancel', label: spec.cancelLabel, tone: 'default' },
                { id: 'confirm', label: spec.confirmLabel, tone: spec.tone || 'primary', autofocus: spec.tone !== 'danger' }] }).then(v => v === 'confirm'),
            destroy() { close(null); life.dispose(); sheet.destroy(); }
        });
    }
    E.sheet = Object.freeze({ create });
})();
