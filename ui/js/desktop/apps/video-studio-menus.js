(function () {
    'use strict';

    // Context menus (role="menu") for timeline clips and lane space. openMenu is shared by later menus
    // (track headers, markers, library items).
    const P = window.VideoStudioParts;
    const E = () => window.VideoStudioEdit;
    const T = (...args) => P.T(...args);
    const esc = (...args) => P.esc(...args);
    const icon = (...args) => P.icon(...args);
    const tr = (...args) => P.tr(...args);

    function closeMenu(s, returnFocus) {
        const menu = s.menu;
        if (!menu) return;
        s.menu = null;
        menu.abort.abort();
        menu.el.remove();
        if (!returnFocus) return;
        const target = menu.focusTarget ? menu.focusTarget() : null;
        if (target && target.isConnected) target.focus({ preventScroll: true });
        else s.app.focus({ preventScroll: true });
    }
    // items: { label, shortcut, icon, disabled, run } or { separator: true }; x and y are client coordinates;
    // focusTarget() returns the element that gets focus back when the menu closes (default: the editor).
    function openMenu(s, x, y, items, label, focusTarget) {
        closeMenu(s, false);
        const el = document.createElement('div');
        el.className = 'vs-context-menu';
        el.setAttribute('role', 'menu');
        el.setAttribute('aria-label', label);
        el.innerHTML = items.map((item, index) => (item.separator ? '<hr role="separator">' : `<button type="button" role="menuitem" data-menu-index="${index}" ${item.disabled ? 'disabled' : ''}>${item.icon ? icon(item.icon, 15) : '<span class="vs-menu-gap"></span>'}<span>${esc(s, item.label)}</span>${item.shortcut ? `<kbd>${esc(s, item.shortcut)}</kbd>` : ''}</button>`)).join('');
        s.app.append(el);
        const app = s.app.getBoundingClientRect(), box = el.getBoundingClientRect();
        el.style.left = Math.max(4, Math.min(x - app.left, app.width - box.width - 4)) + 'px';
        el.style.top = Math.max(4, Math.min(y - app.top, app.height - box.height - 4)) + 'px';
        const abort = new AbortController(), signal = abort.signal;
        s.menu = { el, abort, focusTarget: focusTarget || null };
        const enabled = () => Array.from(el.querySelectorAll('button:not(:disabled)'));
        el.addEventListener('click', event => {
            const button = event.target.closest('[data-menu-index]');
            if (!button || button.disabled) return;
            closeMenu(s, true);
            items[Number(button.dataset.menuIndex)].run();
        }, { signal });
        el.addEventListener('keydown', event => {
            const list = enabled(), index = list.indexOf(document.activeElement);
            if (event.key === 'ArrowDown' || event.key === 'ArrowUp') list[(index + (event.key === 'ArrowDown' ? 1 : list.length - 1) + list.length) % list.length]?.focus();
            else if (event.key === 'Home') list[0]?.focus();
            else if (event.key === 'End') list[list.length - 1]?.focus();
            else if (event.key === 'Escape' || event.key === 'Tab') closeMenu(s, true);
            else return;
            event.preventDefault(); event.stopPropagation();
        }, { signal });
        window.addEventListener('pointerdown', event => { if (!el.contains(event.target)) closeMenu(s, false); }, { signal, capture: true });
        window.addEventListener('blur', () => closeMenu(s, false), { signal });
        window.addEventListener('resize', () => closeMenu(s, false), { signal });
        enabled()[0]?.focus({ preventScroll: true });
        return el;
    }
    function openClipMenu(s, clipId, x, y) {
        const found = s.project && T().clipFor(s.project, clipId);
        if (!found) return null;
        if (!P.isSelected(s, clipId)) { P.setSelection(s, [clipId], clipId); P.renderTimeline(s); }
        const ro = !!s.readonly, canPaste = !ro && P.clipboardReady(s), key = name => tr(s, 'key' + name, name);
        const inside = P.selectedIds(s).some(id => { const hit = T().clipFor(s.project, id); return !!hit && s.frame > hit.clip.start && s.frame < hit.clip.start + hit.clip.duration; });
        const items = [
            { label: tr(s, 'split', 'Split'), shortcut: 'S', icon: 'scissors', disabled: ro || !inside, run: () => P.splitSelection(s) },
            { label: tr(s, 'duplicate', 'Duplicate'), shortcut: key('Ctrl') + '+D', icon: 'copy', disabled: ro, run: () => P.duplicateSelection(s) },
            { separator: true },
            { label: tr(s, 'cut', 'Cut'), shortcut: key('Ctrl') + '+X', disabled: ro, run: () => P.cutSelection(s) },
            { label: tr(s, 'copyClips', 'Copy'), shortcut: key('Ctrl') + '+C', run: () => P.copySelection(s) },
            { label: tr(s, 'paste', 'Paste'), shortcut: key('Ctrl') + '+V', disabled: !canPaste, run: () => P.pasteClipboard(s, 'overwrite') },
            { label: tr(s, 'pasteInsert', 'Paste insert'), shortcut: key('Ctrl') + '+' + key('Shift') + '+V', disabled: !canPaste, run: () => P.pasteClipboard(s, 'insert') },
            { label: tr(s, 'pasteAttributes', 'Paste attributes…'), shortcut: key('Ctrl') + '+' + key('Alt') + '+V', disabled: !canPaste, run: () => P.openPasteAttributes(s) },
            { separator: true },
            { label: tr(s, 'selectForward', 'Select forward on track'), run: () => P.selectForward(s, found.track.id, found.clip.start) },
            { separator: true },
            { label: tr(s, 'delete', 'Delete'), shortcut: key('Del'), icon: 'trash', disabled: ro, run: () => P.deleteSelection(s, false) },
            { label: tr(s, 'rippleDelete', 'Ripple delete'), shortcut: key('Shift') + '+' + key('Del'), disabled: ro, run: () => P.deleteSelection(s, true) }
        ];
        return openMenu(s, x, y, items, tr(s, 'clipMenu', 'Clip actions'), () => s.q(`[data-timeline] [data-clip-id="${CSS.escape(clipId)}"]`));
    }
    function openGapMenu(s, trackId, frame, x, y) {
        if (!s.project) return null;
        const ro = !!s.readonly, gap = E().gapAt(s.project, trackId, frame);
        const items = [
            { label: tr(s, 'rippleDeleteGap', 'Delete gap'), disabled: ro || !gap, run: () => P.deleteGap(s, trackId, frame) },
            { label: tr(s, 'pasteHere', 'Paste here'), disabled: ro || !P.clipboardReady(s), run: () => P.pasteClipboard(s, 'overwrite', frame, trackId) },
            { separator: true },
            { label: tr(s, 'selectForward', 'Select forward on track'), run: () => P.selectForward(s, trackId, frame) },
            { label: tr(s, 'selectAll', 'Select all'), run: () => P.selectAll(s) }
        ];
        return openMenu(s, x, y, items, tr(s, 'gapMenu', 'Track actions'), null);
    }

    Object.assign(P, { openMenu, closeMenu, openClipMenu, openGapMenu });
})();
