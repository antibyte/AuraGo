// EasyDrag undo affordances: the header's Undo and Redo buttons (touch has no Ctrl+Z) and the toast
// that offers to take a deletion back ("Alpha deleted · Undo"). The editor hands over the header group
// and the body; both parts follow the model shown through ed.bus (model, steps-deleted).
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    // TOAST_MS is how long the toast stays while neither the pointer nor the focus is on it.
    const TOAST_MS = 8000;

    // create fills opts.host (the header's history group) and adds the toast to opts.toastHost.
    // opts: {host, toastHost, undo(), redo(), focusCanvas()}; undo and redo are interact's, which
    // announce what they did. Returns {sync, hideToast, dispose}.
    function create(ed, opts) {
        const core = ED.core;
        const { t, esc } = ed;
        const bag = core.bag();
        const host = opts.host;
        const hint = key => ' (' + core.shortcut(key) + ')';
        host.setAttribute('aria-label', t('easydrag.ui.history_label'));
        host.innerHTML = '<button type="button" class="ed-icon-btn" data-ed-history="undo" aria-label="' + esc(t('easydrag.ui.undo')) + '" title="' + esc(t('easydrag.ui.undo') + hint('Ctrl+Z')) + '">' + core.icon('undo') + '</button>' +
            '<button type="button" class="ed-icon-btn" data-ed-history="redo" aria-label="' + esc(t('easydrag.ui.redo')) + '" title="' + esc(t('easydrag.ui.redo') + hint('Ctrl+Shift+Z')) + '">' + core.icon('redo') + '</button>';
        const undoBtn = host.querySelector('[data-ed-history="undo"]');
        const redoBtn = host.querySelector('[data-ed-history="redo"]');
        const toast = core.el('<div class="ed-toast" data-ed-toast hidden><span class="ed-toast-text" data-ed-toast-text></span>' +
            '<button type="button" class="ed-btn ed-btn--small" data-ed-toast-undo>' + core.icon('undo') + '<span>' + esc(t('easydrag.ui.undo')) + '</span></button>' +
            '<button type="button" class="ed-icon-btn" data-ed-toast-close aria-label="' + esc(t('easydrag.ui.close')) + '" title="' + esc(t('easydrag.ui.close')) + '">' + core.icon('x') + '</button></div>');
        toast.hidden = true;
        opts.toastHost.appendChild(toast);
        let toastVersion = -1;
        let toastTimer = 0;

        // Disabled buttons would drop the focus to the page: the buttons say aria-disabled instead,
        // and a click on them does nothing.
        function setOff(button, off) {
            if (off) button.setAttribute('aria-disabled', 'true');
            else button.removeAttribute('aria-disabled');
        }

        // sync follows the model shown: off in the run view and while there is nothing to undo or
        // redo; a read-only desktop shows no group at all.
        function sync() {
            const ro = !!(ed.readonly || ed.runView);
            host.hidden = !!ed.readonly;
            setOff(undoBtn, ro || !ed.model.canUndo());
            setOff(redoBtn, ro || !ed.model.canRedo());
        }

        function arm() {
            clearTimeout(toastTimer);
            toastTimer = setTimeout(hideToast, TOAST_MS);
        }

        function hideToast() {
            clearTimeout(toastTimer);
            toastTimer = 0;
            toastVersion = -1;
            if (toast.hidden) return;
            const hadFocus = toast.contains(document.activeElement);
            toast.hidden = true;
            if (hadFocus) opts.focusCanvas();
        }

        // showDeleted offers to take back the deletion just made ({names}). The canvas live region has
        // said it already, so the toast is no live region. Its Undo acts only while the document is the
        // one right after the deletion (toastVersion); any other change closes the toast.
        function showDeleted(payload) {
            const names = (payload && payload.names) || [];
            if (!names.length || ed.readonly || ed.runView) return;
            toast.querySelector('[data-ed-toast-text]').textContent = core.stepsText(t, 'easydrag.ui.announce_deleted_one', 'easydrag.ui.announce_deleted_many', names);
            toastVersion = ed.model.version;
            toast.hidden = false;
            arm();
        }

        bag.listen(host, 'click', (event) => {
            const b = event.target.closest('[data-ed-history]');
            if (!b || b.getAttribute('aria-disabled') === 'true') return;
            if (b.dataset.edHistory === 'undo') opts.undo(); else opts.redo();
            sync();
        });
        bag.listen(toast, 'click', (event) => {
            if (event.target.closest('[data-ed-toast-close]')) { hideToast(); return; }
            if (!event.target.closest('[data-ed-toast-undo]')) return;
            const current = toastVersion !== -1 && ed.model.version === toastVersion;
            hideToast();
            if (current) opts.undo();
        });
        bag.listen(toast, 'focusin', () => clearTimeout(toastTimer));
        bag.listen(toast, 'focusout', () => { if (!toast.hidden) arm(); });
        bag.listen(toast, 'pointerenter', () => clearTimeout(toastTimer));
        bag.listen(toast, 'pointerleave', () => { if (!toast.hidden) arm(); });
        bag.add(ed.bus.on('model', () => {
            sync();
            if (!toast.hidden && ed.model.version !== toastVersion) hideToast();
        }));
        bag.add(ed.bus.on('steps-deleted', showDeleted));
        bag.add(() => { clearTimeout(toastTimer); toast.remove(); });
        sync();

        return { sync, hideToast, dispose() { bag.dispose(); } };
    }

    ED.history = { create };
})();
