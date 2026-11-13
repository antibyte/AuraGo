// EasyDrag undo affordances: the header's Undo and Redo buttons (touch has no Ctrl+Z) and the toast
// that offers to take a deletion back ("Alpha deleted · Undo") (ED.kit.toast). The editor hands over the header group
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
        // toastVersion is the model version right after the deletion: the toast's Undo acts only on that one.
        let toastVersion = -1;
        const toast = ED.kit.toast({
            status: 'ok', host: opts.toastHost, start: false, duration: TOAST_MS, closeLabel: t('easydrag.ui.close'),
            action: { label: t('easydrag.ui.undo'), icon: 'undo', onClick: () => {
                const current = toastVersion !== -1 && ed.model.version === toastVersion;
                toastVersion = -1;
                if (current) opts.undo();
            } },
            onHide: ({ hadFocus }) => { if (hadFocus) opts.focusCanvas(); }
        });

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

        function hideToast() { toast.hide(); toastVersion = -1; }

        // showDeleted offers to take back the deletion just made ({names}). The canvas live region has
        // said it already, so the toast is no live region. Its Undo acts only while the document is the
        // one right after the deletion (toastVersion); any other change closes the toast.
        function showDeleted(payload) {
            const names = (payload && payload.names) || [];
            if (!names.length || ed.readonly || ed.runView) return;
            toastVersion = ed.model.version;
            toast.show(core.stepsText(t, 'easydrag.ui.announce_deleted_one', 'easydrag.ui.announce_deleted_many', names));
        }

        bag.listen(host, 'click', (event) => {
            const b = event.target.closest('[data-ed-history]');
            if (!b || b.getAttribute('aria-disabled') === 'true') return;
            if (b.dataset.edHistory === 'undo') opts.undo(); else opts.redo();
            sync();
        });
        bag.add(ed.bus.on('model', () => {
            sync();
            if (toast.isShown() && ed.model.version !== toastVersion) hideToast();
        }));
        bag.add(ed.bus.on('steps-deleted', showDeleted));
        bag.add(() => toast.dispose());
        sync();

        return { sync, hideToast, dispose() { bag.dispose(); } };
    }

    ED.history = { create };
})();
