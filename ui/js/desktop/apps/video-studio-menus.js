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
    // Colour labels in a second menu; "Back" returns to the clip menu.
    function openLabelMenu(s, clipId, x, y) {
        const colors = P.LABEL_COLORS || [];
        const items = [{ label: tr(s, 'labelNone', 'No label'), run: () => P.setLabel(s, '') }]
            .concat(colors.map(color => ({ label: tr(s, 'color_' + color, color), run: () => P.setLabel(s, color) })))
            .concat([{ separator: true }, { label: tr(s, 'back', 'Back'), run: () => openClipMenu(s, clipId, x, y) }]);
        return openMenu(s, x, y, items, tr(s, 'labelMenu', 'Color label…'), () => s.q(`[data-timeline] [data-clip-id="${CSS.escape(clipId)}"]`));
    }
    // An unselected clip selects its link group; a clip already in the selection becomes its primary clip, so the
    // clip-specific items (P.clipCapabilities) describe the clip under the pointer.
    function openClipMenu(s, clipId, x, y) {
        const found = s.project && T().clipFor(s.project, clipId);
        if (!found) return null;
        if (!P.isSelected(s, clipId)) { P.setSelection(s, P.linkedIds(s, [clipId], false), clipId); P.renderTimeline(s); }
        else if (s.selectedClipId !== clipId) { P.setSelection(s, P.selectedIds(s), clipId); P.renderTimeline(s); }
        const ro = !!s.readonly, caps = P.clipCapabilities(s), canPaste = !ro && P.clipboardReady(s), key = name => tr(s, 'key' + name, name);
        // Split acts on every selected clip under the playhead (as landed in 1a), not only the clicked one.
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
            { label: tr(s, caps.linked ? 'unlink' : 'link', caps.linked ? 'Unlink' : 'Link'), shortcut: key('Ctrl') + '+L', icon: caps.linked ? 'unlink' : 'link', disabled: ro || !caps.linkable, run: () => P.toggleLink(s) },
            { label: tr(s, 'speed', 'Speed…'), icon: 'speed', disabled: ro || !caps.retimable, run: () => P.focusSpeed(s) },
            { label: tr(s, 'labelMenu', 'Color label…'), icon: 'label', disabled: ro, run: () => openLabelMenu(s, clipId, x, y) },
            { label: tr(s, found.clip.disabled ? 'enable' : 'disable', found.clip.disabled ? 'Enable' : 'Disable'), shortcut: key('Shift') + '+E', icon: found.clip.disabled ? 'eye' : 'eyeOff', disabled: ro, run: () => P.toggleDisabled(s) },
            { separator: true },
            { label: tr(s, 'detachAudio', 'Detach audio'), icon: 'speaker', disabled: ro || !caps.detachable, run: () => P.detachAudio(s) },
            { label: tr(s, 'reattachAudio', 'Reattach audio'), icon: 'speaker', disabled: ro || !caps.reattachable, run: () => P.reattachAudio(s) },
            { label: tr(s, 'reverse', 'Reverse'), icon: 'reverse', disabled: ro || !caps.reversible, run: () => P.clipCommand(s, 'reverse') },
            { label: tr(s, 'freezeFrame', 'Freeze frame'), icon: 'freeze', disabled: ro || !caps.freezable, run: () => P.clipCommand(s, 'freeze') },
            { label: tr(s, 'exportFrame', 'Export frame as PNG'), icon: 'download', disabled: ro || !caps.exportable, run: () => P.clipCommand(s, 'export-frame') },
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
