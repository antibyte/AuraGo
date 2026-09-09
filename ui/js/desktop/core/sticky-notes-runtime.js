    function updateStickyNoteCard(card, widget) {
        if (!card.classList.contains('vd-sticky-note')) {
            cleanupWidgetCard(card);
            card.innerHTML = `<div class="vd-sticky-paper" aria-hidden="true"></div>
                <div class="vd-sticky-text vd-scroll" tabindex="0"></div>
                <button type="button" class="vd-sticky-menu" aria-haspopup="menu">···</button>`;
            card.querySelector('button').addEventListener('click', event => {
                event.stopPropagation();
                const bounds = event.currentTarget.getBoundingClientRect();
                showStickyNoteMenu(bounds.right, bounds.bottom, card._widgetData);
                const menu = state.contextMenu;
                if (menu) menu.querySelector('button:not(:disabled)')?.focus();
            });
            card.addEventListener('dblclick', event => {
                if (!event.target.closest('button')) editStickyNote(card._widgetData);
            });
        }
        card.className = 'vd-widget vd-sticky-note';
        card._widgetData = widget;
        card.dataset.widgetId = widget.id;
        card.removeAttribute('title');
        delete card.dataset.widgetAutoSize;
        const config = widgetConfig(widget);
        const text = typeof config.text === 'string' ? config.text : '';
        card.querySelector('.vd-sticky-text').textContent = text || t('desktop.sticky_placeholder');
        card.querySelector('.vd-sticky-text').setAttribute('aria-label', t('desktop.sticky_note'));
        card.querySelector('button').setAttribute('aria-label', t('desktop.sticky_actions'));
        // Derive paper wear from the persistent identity, so refreshes never change it.
        let seed = 0;
        for (const char of widget.id) seed = (Math.imul(seed, 31) + char.charCodeAt(0)) >>> 0;
        seed = Math.imul(seed ^ (seed >>> 16), 2246822507) >>> 0;
        card.dataset.paper = String(seed % 4);
        card.style.setProperty('--sticky-angle', ((seed % 601) / 100 - 3) + 'deg');
        card.style.setProperty('--sticky-hue', String(48 + seed % 10));
        card.style.setProperty('--sticky-fold', (16 + seed % 15) + 'px');
        card.style.setProperty('--sticky-crease', (22 + seed % 48) + '%');
        const size = Math.min(220, Math.max(140, ($('vd-workspace').clientWidth || window.innerWidth) - 32));
        const pos = clampToWorkspace(Number(widget.x) || 16, Number(widget.y) || 16, size, 220);
        card.style.left = pos.x + 'px';
        card.style.top = pos.y + 'px';
        card.style.width = size + 'px';
        card.style.height = '220px';
        return false;
    }

    function showStickyNoteMenu(x, y, widget) {
        const readonly = desktopReadonly();
        showContextMenu(x, y, [
            { label: t('desktop.edit'), icon: 'edit', fallback: 'E', disabled: readonly, action: () => editStickyNote(widget) },
            { label: t('desktop.fm.duplicate'), icon: 'copy', fallback: '+', disabled: readonly, action: () => editStickyNote(null, x + 24, y + 24, widgetConfig(widget).text) },
            { separator: true },
            { label: t('desktop.delete'), icon: 'trash', fallback: 'X', disabled: readonly, action: async () => {
                if (desktopReadonly() || !await confirmDialog(t('desktop.notes_delete_confirm'))) return;
                try {
                    await api('/api/desktop/widgets?id=' + encodeURIComponent(widget.id), { method: 'DELETE' });
                    await loadBootstrap();
                } catch (_) {
                    showDesktopNotification({ title: t('desktop.notification'), message: t('desktop.widget_update_failed') });
                }
            } }
        ]);
    }

    function editStickyNote(widget, clientX, clientY, initialText) {
        if (desktopReadonly()) return;
        closeContextMenu();
        const previousFocus = document.activeElement;
        const workspace = $('vd-workspace').getBoundingClientRect();
        const pos = clampToWorkspace((clientX ?? workspace.left + 40) - workspace.left, (clientY ?? workspace.top + 40) - workspace.top, 220, 220);
        const record = widget || {
            id: 'sticky-' + Date.now().toString(36) + '-' + crypto.getRandomValues(new Uint32Array(1))[0].toString(36),
            title: t('desktop.sticky_note'), type: 'sticky-note', icon: 'notes',
            x: Math.round(pos.x), y: Math.round(pos.y), w: 220, h: 220, visible: true, builtin: false,
            config: { auto_size: false }
        };
        const dialog = document.createElement('dialog');
        dialog.className = 'vd-sticky-editor';
        dialog.setAttribute('aria-labelledby', 'vd-sticky-editor-title');
        dialog.innerHTML = `<form>
            <h2 id="vd-sticky-editor-title">${esc(t(widget ? 'desktop.edit' : 'desktop.sticky_add'))}</h2>
            <label for="vd-sticky-input">${esc(t('desktop.sticky_note'))}</label>
            <textarea id="vd-sticky-input" maxlength="4000" rows="9" placeholder="${esc(t('desktop.sticky_placeholder'))}"></textarea>
            <p class="vd-sticky-error" role="status"></p>
            <div class="vd-modal-actions">
                <button class="vd-button" type="button" data-cancel>${esc(t('desktop.cancel'))}</button>
                <button class="vd-button vd-button-primary" type="submit">${esc(t('desktop.save'))}</button>
            </div>
        </form>`;
        const input = dialog.querySelector('textarea');
        input.value = typeof initialText === 'string' ? initialText : (widgetConfig(record).text || '');
        const submit = dialog.querySelector('[type="submit"]');
        const cancel = dialog.querySelector('[data-cancel]');
        let saving = false;
        cancel.addEventListener('click', () => dialog.close());
        dialog.addEventListener('cancel', event => { if (saving) event.preventDefault(); });
        dialog.addEventListener('close', () => {
            dialog.remove();
            if (previousFocus?.isConnected) previousFocus.focus();
        });
        dialog.querySelector('form').addEventListener('submit', async event => {
            event.preventDefault();
            if (saving || desktopReadonly()) return;
            saving = true;
            submit.disabled = cancel.disabled = true;
            input.readOnly = true;
            dialog.querySelector('[role="status"]').textContent = '';
            try {
                const current = widget ? (state.bootstrap?.all_widgets || []).find(item => item.id === widget.id) : record;
                if (!current) throw new Error('Sticky note no longer exists');
                await persistWidgetConfig(current, { text: input.value, auto_size: false }, { skipReload: true });
                dialog.close();
                await loadBootstrap();
            } catch (_) {
                if (dialog.isConnected) dialog.querySelector('[role="status"]').textContent = t('desktop.widget_update_failed');
                else showDesktopNotification({ title: t('desktop.notification'), message: t('desktop.widget_update_failed') });
            } finally {
                saving = false;
                submit.disabled = cancel.disabled = false;
                input.readOnly = false;
            }
        });
        input.addEventListener('keydown', event => {
            if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
                event.preventDefault();
                dialog.querySelector('form').requestSubmit();
            }
        });
        document.body.appendChild(dialog);
        dialog.showModal();
        input.focus();
    }
