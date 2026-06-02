async function renderWidgetDrawerContent(drawer) {
    drawer.querySelectorAll('.vd-widget-drawer-content').forEach(el => el.remove());

    const content = document.createElement('div');
    content.className = 'vd-widget-drawer-content';
    content.style.padding = '12px 16px 20px';
    content.style.overflow = 'auto';
    content.style.maxHeight = 'calc(100% - 48px)';

    const allWidgets = (state.bootstrap && state.bootstrap.all_widgets) || [];

    if (allWidgets.length === 0) {
        content.innerHTML = `
            <div style="padding: 24px 12px; color: var(--ds-color-fg-muted); font-size: 13px; text-align: center;">
                ${t('desktop.widget_drawer_empty', 'No widgets available.')}
            </div>
        `;
        drawer.appendChild(content);
        return;
    }

    const list = document.createElement('div');
    list.style.display = 'flex';
    list.style.flexDirection = 'column';
    list.style.gap = '8px';

    allWidgets.forEach(widget => {
        const isVisible = widget.visible !== false;
        const isBuiltin = widget.builtin === true;
        const card = document.createElement('div');
        const iconKey = widget.icon || 'widgets';

        card.style.cssText = 'display:flex; align-items:center; gap:10px; padding:8px 10px; background:var(--ds-color-surface-1); border:1px solid var(--ds-color-border-subtle); border-radius:8px;';
        card.innerHTML = `
            <div style="flex-shrink:0;">${iconMarkup(iconKey, widget.title || widget.id, 'vd-sprite-file', 22)}</div>
            <div style="flex:1; min-width:0;">
                <div style="font-weight:600; font-size:13px; color:var(--ds-color-fg-primary); white-space:nowrap; overflow:hidden; text-overflow:ellipsis;">
                    ${esc(widget.title || widget.id)}
                </div>
                <div style="font-size:11px; color:var(--ds-color-fg-muted);">
                    ${isBuiltin ? t('desktop.widget_builtin', 'Builtin') : t('desktop.widget_custom', 'Custom')}
                    ${isVisible ? ' • ' + t('desktop.widget_on_desktop', 'On desktop') : ''}
                </div>
            </div>
            <div>
                <button type="button" class="vd-wm-btn" data-widget-action="${isVisible ? 'hide' : 'show'}" data-widget-id="${esc(widget.id)}"
                    style="font-size:11px; padding:4px 10px; min-width:72px;">
                    ${isVisible ? t('desktop.widget_remove_from_desktop', 'Remove') : t('desktop.widget_add_to_desktop', 'Add')}
                </button>
            </div>
        `;

        const btn = card.querySelector('button');
        btn.addEventListener('click', async e => {
            e.stopPropagation();
            const action = btn.dataset.widgetAction;
            const id = btn.dataset.widgetId;
            if (!id) return;

            btn.disabled = true;
            btn.textContent = '...';

            try {
                await api('/api/desktop/widgets?id=' + encodeURIComponent(id), {
                    method: 'PATCH',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ visible: action === 'show' })
                });
                await loadBootstrap();
                renderWidgetDrawerContent(drawer);
            } catch (err) {
                showDesktopNotification({
                    title: t('desktop.notification'),
                    message: err.message || 'Failed to update widget'
                });
                btn.disabled = false;
                btn.textContent = action === 'show'
                    ? t('desktop.widget_add_to_desktop', 'Add')
                    : t('desktop.widget_remove_from_desktop', 'Remove');
            }
        });

        list.appendChild(card);
    });

    content.appendChild(list);
    drawer.appendChild(content);
}

function updateTaskbarSystemButtonsForMobile() {
    const isMobile = window.useMobileDesktopMode && window.useMobileDesktopMode();
    const shortcutsBtn = document.getElementById('vd-shortcuts-trigger');
    const widgetsBtn = document.getElementById('vd-widget-drawer-btn');

    if (shortcutsBtn) shortcutsBtn.style.display = isMobile ? 'none' : '';
    if (widgetsBtn) widgetsBtn.style.display = isMobile ? 'none' : '';
}
