// Searchable combobox for Infra (app links, places, hosts, connection ends).
// ARIA 1.2 combobox: focus stays in the input, aria-activedescendant names the
// active option. Popover mode floats in the shell layer; inline mode lives in a form.
(function () {
    'use strict';

    function create(app) {
        const { store, t, esc, model } = app;
        let counter = 0, popover = null;
        const inlines = new Set();
        function optionsMarkup(prefix, result) {
            const flat = [];
            const html = (result.groups || []).filter(group => group.options.length).map((group, g) => {
                const heading = `${prefix}-g${g}`;
                const options = group.options.map(option => {
                    const index = flat.length, id = `${prefix}-o${index}`;
                    flat.push({ ...option, domId: id });
                    const attrs = Object.entries(option.attrs || {}).map(([key, value]) => ` ${key}="${esc(value)}"`).join('');
                    return `<div class="infra-picker-option" role="option" id="${id}" aria-selected="false" data-picker-option="${index}"${attrs}>${option.icon ? app.icon(option.icon) : ''}<span class="infra-picker-label">${esc(option.label)}</span>${option.meta ? `<small class="infra-picker-meta">${esc(option.meta)}</small>` : ''}</div>`;
                }).join('');
                const more = group.more ? `<div class="infra-picker-more">${esc(model.fill(t('desktop.infra_picker_more'), { count: group.more }))}</div>` : '';
                return `<div class="infra-picker-group" role="group" aria-labelledby="${heading}"><div class="infra-picker-heading" id="${heading}">${esc(group.label)}</div>${options}${more}</div>`;
            }).join('');
            return { html, flat };
        }
        function controller({ input, list, note, empty, source, onPick, onEscape }) {
            let flat = [], active = -1;
            const setActive = next => {
                active = next;
                for (const option of list.querySelectorAll('[data-picker-option]')) { const on = Number(option.dataset.pickerOption) === next; option.classList.toggle('is-active', on); option.setAttribute('aria-selected', String(on)); }
                const current = flat[next];
                input.setAttribute('aria-activedescendant', current ? current.domId : '');
                if (current) list.querySelector(`[id="${current.domId}"]`)?.scrollIntoView?.({ block: 'nearest' });
            };
            const refresh = () => {
                const result = source(input.value) || { groups: [] };
                const built = optionsMarkup(list.id, result);
                flat = built.flat; list.innerHTML = built.html;
                empty.hidden = flat.length > 0;
                note.textContent = result.note || ''; note.hidden = !result.note;
                setActive(flat.length ? 0 : -1);
            };
            const pick = index => { const option = flat[index]; if (option) onPick(option); };
            const onKey = event => {
                if (event.key === 'ArrowDown' || event.key === 'ArrowUp') { event.preventDefault(); if (flat.length) setActive((active + (event.key === 'ArrowDown' ? 1 : -1) + flat.length) % flat.length); }
                else if (event.key === 'Enter') { event.preventDefault(); if (active >= 0) pick(active); } // never submits the surrounding form
                else if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); onEscape(); }
            };
            const onInput = () => refresh();
            const keepFocus = event => { if (event.target.closest('[data-picker-option]')) event.preventDefault(); };
            const onClick = event => { const option = event.target.closest('[data-picker-option]'); if (option) pick(Number(option.dataset.pickerOption)); };
            input.addEventListener('keydown', onKey); input.addEventListener('input', onInput);
            list.addEventListener('pointerdown', keepFocus); list.addEventListener('mousedown', keepFocus); list.addEventListener('click', onClick);
            return { refresh, destroy() { input.removeEventListener('keydown', onKey); input.removeEventListener('input', onInput); list.removeEventListener('pointerdown', keepFocus); list.removeEventListener('mousedown', keepFocus); list.removeEventListener('click', onClick); } };
        }
        const skeleton = (id, label, placeholder, expanded) => `<input type="text" class="infra-picker-input" data-picker-input role="combobox" aria-expanded="${expanded}" aria-controls="${id}" aria-autocomplete="list" aria-activedescendant="" aria-label="${esc(label)}" placeholder="${esc(placeholder)}" autocomplete="off" spellcheck="false"><div class="infra-picker-list" id="${id}" role="listbox" aria-label="${esc(label)}" data-picker-list ${expanded ? '' : 'hidden'}></div><p class="infra-picker-note" data-picker-note hidden></p><p class="infra-picker-empty" data-picker-empty hidden>${esc(t('desktop.infra_picker_no_match'))}</p>`;
        function place(el, rect) {
            const box = app.root.getBoundingClientRect(), width = el.offsetWidth || 320, below = rect.bottom - box.top + 6;
            el.style.left = `${Math.max(8, Math.min(rect.left - box.left, box.width - width - 8))}px`;
            el.style.top = `${below}px`;
            if (below + el.offsetHeight > box.height - 8) el.style.top = `${Math.max(8, rect.top - box.top - el.offsetHeight - 6)}px`;
        }
        // open shows a popover under anchor (or under rect when the anchor may be repainted).
        function open({ anchor = null, rect = null, label, placeholder = '', source, onPick, onClose = () => {} }) {
            close({ restoreFocus: false });
            const layer = app.q('[data-layer]');
            if (!layer) return;
            const id = `infra-picker-${app.safeId}-${++counter}`, el = document.createElement('div');
            el.className = 'infra-popover infra-picker'; el.dataset.picker = '';
            el.setAttribute('role', 'dialog'); el.setAttribute('aria-label', label);
            el.innerHTML = skeleton(id, label, placeholder || label, true);
            layer.append(el);
            const input = el.querySelector('[data-picker-input]');
            const ctl = controller({ input, list: el.querySelector('[data-picker-list]'), note: el.querySelector('[data-picker-note]'), empty: el.querySelector('[data-picker-empty]'), source, onPick: option => { close(); onPick(option); }, onEscape: () => close() });
            ctl.refresh();
            place(el, rect || anchor?.getBoundingClientRect() || app.root.getBoundingClientRect());
            const outside = event => { if (!el.contains(event.target)) close({ restoreFocus: false }); };
            const focusout = event => { if (event.relatedTarget && !el.contains(event.relatedTarget)) close({ restoreFocus: false }); };
            document.addEventListener('pointerdown', outside, true);
            el.addEventListener('focusout', focusout);
            popover = { el, ctl, anchor, onClose, outside, focusout };
            input.focus({ preventScroll: true });
        }
        function close({ restoreFocus = true } = {}) {
            if (!popover) return;
            const { el, ctl, anchor, onClose, outside, focusout } = popover;
            popover = null;
            document.removeEventListener('pointerdown', outside, true);
            el.removeEventListener('focusout', focusout);
            ctl.destroy(); el.remove();
            if (restoreFocus && anchor?.isConnected) anchor.focus({ preventScroll: true });
            onClose();
        }
        // attachInline turns container into an inline combobox: {focus, refresh({clear}), destroy}.
        function attachInline(container, { label, placeholder = '', source, onPick }) {
            const id = `infra-picker-${app.safeId}-${++counter}`;
            container.classList.add('infra-picker', 'infra-picker-inline'); container.dataset.picker = '';
            container.innerHTML = skeleton(id, label, placeholder || label, false);
            const input = container.querySelector('[data-picker-input]'), list = container.querySelector('[data-picker-list]');
            const note = container.querySelector('[data-picker-note]'), empty = container.querySelector('[data-picker-empty]');
            const setOpen = isOpen => { list.hidden = !isOpen; input.setAttribute('aria-expanded', String(isOpen)); if (!isOpen) { input.setAttribute('aria-activedescendant', ''); note.hidden = true; empty.hidden = true; } };
            const ctl = controller({ input, list, note, empty, source, onPick: option => { onPick(option); input.value = ''; ctl.refresh(); input.focus({ preventScroll: true }); }, onEscape: () => setOpen(false) });
            const show = () => { if (list.hidden) { setOpen(true); ctl.refresh(); } };
            const onKey = event => { if (event.key === 'ArrowDown') show(); };
            const onFocusOut = event => { if (!container.contains(event.relatedTarget)) setOpen(false); };
            input.addEventListener('focus', show); input.addEventListener('input', show); input.addEventListener('keydown', onKey);
            container.addEventListener('focusout', onFocusOut);
            const handle = {
                focus() { input.focus({ preventScroll: true }); show(); },
                refresh({ clear = false } = {}) { if (clear) input.value = ''; if (!list.hidden) ctl.refresh(); },
                isOpen: () => !list.hidden,
                destroy() { ctl.destroy(); input.removeEventListener('focus', show); input.removeEventListener('input', show); input.removeEventListener('keydown', onKey); container.removeEventListener('focusout', onFocusOut); inlines.delete(handle); },
            };
            inlines.add(handle);
            return handle;
        }
        const nodeOption = node => ({ id: node.id, label: node.name, meta: [app.labelForKind(node.kind), node.profile?.ip_address].filter(Boolean).join(' · '), icon: node.icon, attrs: { 'data-node-option': node.id } });
        function assetSource(accept, preferred = []) {
            return query => {
                const q = String(query || '').trim().toLocaleLowerCase();
                const nodes = store.state.nodes.filter(node => accept(node) && (!q || store.searchableText(node).includes(q)));
                const first = preferred.map(id => nodes.find(node => node.id === id)).filter(Boolean);
                const rest = nodes.filter(node => !preferred.includes(node.id)).sort((a, b) => model.naturalCompare(a.name, b.name));
                const groups = first.length ? [{ id: 'suggested', label: t('desktop.infra_picker_suggested'), options: first.map(nodeOption) }] : [];
                groups.push({ id: 'all', label: t('desktop.infra_link_all'), options: rest.slice(0, 100).map(nodeOption), more: Math.max(0, rest.length - 100) });
                return { groups };
            };
        }
        const openAssetPicker = ({ anchor = null, rect = null, label, accept, preferred = [], onPick }) => open({ anchor, rect, label, placeholder: t('desktop.infra_pick_asset'), source: assetSource(accept, preferred), onPick });
        // openPlacePicker offers "No place" plus every place except exclude and its descendants.
        function openPlacePicker({ anchor = null, rect = null, exclude = [], onPick }) {
            const parentOf = id => store.nodeById(id)?.parent_id || '';
            const blocked = node => exclude.includes(node.id) || exclude.some(id => model.isDescendant(node.id, id, parentOf));
            open({ anchor, rect, label: t('desktop.infra_move_to_place'), placeholder: t('desktop.infra_pick_place'), onPick: option => onPick(option.id), source: query => {
                const q = String(query || '').trim().toLocaleLowerCase();
                const places = store.state.nodes.filter(node => model.isContainer(node.kind) && !blocked(node))
                    .map(node => ({ node, path: model.placePath(node.id, store.nodeById, { includeSelf: true }).map(item => item.name).join(' › ') }))
                    .filter(item => !q || item.path.toLocaleLowerCase().includes(q)).sort((a, b) => model.naturalCompare(a.path, b.path));
                const options = [...(q ? [] : [{ id: '', label: t('desktop.infra_no_place'), attrs: { 'data-place-option': '' } }]), ...places.slice(0, 100).map(item => ({ id: item.node.id, label: item.path, icon: item.node.icon, attrs: { 'data-place-option': item.node.id } }))];
                return { groups: [{ id: 'places', label: t('desktop.infra_place'), options, more: Math.max(0, places.length - 100) }] };
            } });
        }
        return { open, close, isOpen: () => !!popover, attachInline, assetSource, openAssetPicker, openPlacePicker, dispose() { close({ restoreFocus: false }); for (const handle of [...inlines]) handle.destroy(); } };
    }

    window.InfraAppPicker = { create };
})();
