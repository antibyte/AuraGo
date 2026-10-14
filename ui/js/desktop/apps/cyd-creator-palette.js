// CYD Creator designer palette: widget entries grouped like the catalog (inst.caps.widgets), with a
// search field. Pointer: drag an entry onto the screen (interact.startPaletteDrag); a press without
// movement adds it in the middle. Keyboard: Enter/Space on an entry adds it in the middle, the
// designer's keys (Delete, undo, arrows) work from an entry, and Escape cancels a running palette
// drag. While read-only, locked by an agent job (the designer's 'lock'
// signal) or on the Wi-Fi system screen (it takes no widgets) every entry is disabled.
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    const GROUPS = ['basic', 'input', 'display', 'structure', 'composite'];
    const GLYPHS = {
        label: 'Aa', button: '▭', image: '▨', panel: '▢', line: '─', switch: '◐', checkbox: '☑', slider: '⊸', arc: '◠',
        dropdown: '▾', roller: '⇕', textarea: '⌨', spinbox: '±', keypad: '⌗', bar: '▬', spinner: '◌', led: '●',
        chart: '∿', gauge: '◔', table: '▦', qrcode: '▣', tabview: '⊟', list: '☰', msgbox: '▤', canvas: '▧', statusbar: '▔'
    };
    const PAGE_KEYS = new Set(['Enter', 'Delete', 'Backspace', 'F2']);

    function mount(d, host) {
        const { h, dom } = P.render;
        const t = d.t;
        const bag = P.util.bag();
        const specs = d.inst.caps.widgets || [];
        const groups = GROUPS.concat(Array.from(new Set(specs.map(s => s.group))).filter(g => !GROUPS.includes(g)));
        // Catalog names: translated, else the catalog's own type and group ids.
        const label = d.label;
        const groupLabel = g => t.or('cyd_creator.group.' + g, g);
        const state = () => (d.readonly() ? 'readonly' : '') + (d.isSystem() ? ' system' : '');
        let search = null;
        let list = null;
        let shown = null;
        const root = dom(h('section', { class: 'cydc-palette', 'aria-label': t('cyd_creator.designer.palette_title') },
            h('div', { class: 'cydc-panel-head' }, h('h3', {}, t('cyd_creator.designer.palette_title'))),
            h('input', { type: 'search', class: 'cydc-palette-search', placeholder: t('cyd_creator.designer.palette_search'), 'aria-label': t('cyd_creator.designer.palette_search'), ref: el => { search = el; } }),
            h('div', { class: 'cydc-palette-list', ref: el => { list = el; } }),
            h('p', { class: 'cydc-panel-hint' }, t('cyd_creator.designer.palette_hint'))));
        host.appendChild(root);

        function render() {
            const q = String(search.value || '').trim().toLowerCase();
            const ro = d.readonly();
            const off = ro || d.isSystem();
            // A focused entry keeps the focus across a re-render; a disabled one hands it to the search
            // field, never to the page (the desktop acts on its selected icons from the page's keys).
            const active = document.activeElement;
            const focused = active && active !== list && list.contains(active) && active.dataset ? active.dataset.ccPalette || '' : '';
            const made = new Map();
            const match = s => !q || s.type.includes(q) || label(s.type).toLowerCase().includes(q);
            const sections = groups.map(g => {
                const items = specs.filter(s => s.group === g && match(s));
                if (!items.length) return null;
                return h('div', { class: 'cydc-palette-group', role: 'group', 'aria-label': groupLabel(g) },
                    h('h4', {}, groupLabel(g)),
                    h('div', { class: 'cydc-palette-items' }, items.map(s => h('button', {
                        type: 'button', class: 'cydc-palette-item', 'data-cc-palette': s.type, disabled: off,
                        title: t('cyd_creator.designer.palette_item_hint', { name: label(s.type) }),
                        ref: el => { made.set(s.type, el); }
                    }, h('span', { class: 'cydc-palette-glyph', 'aria-hidden': 'true' }, GLYPHS[s.type] || '◆'), h('span', { class: 'cydc-palette-name' }, label(s.type))))));
            }).filter(Boolean);
            list.replaceChildren(...(sections.length ? sections.map(n => dom(n)) : [dom(h('p', { class: 'cydc-panel-empty' }, t('cyd_creator.designer.palette_empty')))]));
            root.classList.toggle('is-readonly', ro);
            root.classList.toggle('is-system', d.isSystem());
            shown = state();
            if (focused) {
                const next = made.get(focused);
                (next && !next.disabled ? next : search).focus({ preventScroll: true });
            }
        }
        // sync re-renders when read-only, the lock or the system screen changed what the entries allow.
        function sync() { if (state() !== shown) render(); }

        bag.on(list, 'pointerdown', event => {
            const item = event.target && event.target.closest ? event.target.closest('[data-cc-palette]') : null;
            if (!item || item.disabled || !d.interact) return;
            d.interact.startPaletteDrag(item.dataset.ccPalette, event);
        });
        // Mouse presses are handled by the pointer path above; a click with detail 0 comes from the keyboard.
        bag.on(list, 'click', event => {
            const item = event.target && event.target.closest ? event.target.closest('[data-cc-palette]') : null;
            if (!item || item.disabled || event.detail !== 0) return;
            d.commands.addAtCenter(item.dataset.ccPalette);
        });
        // Keys of the palette: during a palette drag they go to the gesture (Escape cancels it; the
        // pressed entry may hold the focus). Otherwise the search field keeps its keys and an entry
        // passes the designer's keys on (after a keyboard add, Delete or Ctrl+Z act on the new
        // widget). Enter (the entry's own click), Delete, Backspace and F2 never leave the palette:
        // the desktop opens, deletes or renames its selected icons from them.
        bag.on(root, 'keydown', event => {
            const ix = d.interact;
            const field = event.target === search;
            if (ix && (ix.gestureKey(event) || (!field && ix.handleKey(event)))) { event.preventDefault(); event.stopPropagation(); return; }
            if (!field && PAGE_KEYS.has(event.key)) event.stopPropagation();
        });
        bag.on(search, 'input', render);
        bag.add(d.bus.on('lock', sync));
        bag.add(d.bus.on('screen', sync));
        render();
        return { root, render, dispose() { bag.dispose(); root.remove(); } };
    }

    P.designerPalette = { mount, GLYPHS };
})();
