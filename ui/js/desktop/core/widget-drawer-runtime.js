async function renderWidgetDrawerContent(drawer) {
    drawer.querySelectorAll('.vd-widget-drawer-content').forEach(el => el.remove());

    const content = document.createElement('div');
    content.className = 'vd-widget-drawer-content vd-scroll';

    const allWidgets = (state.bootstrap && state.bootstrap.all_widgets) || [];

    if (allWidgets.length === 0) {
        content.innerHTML = `
            <div class="vd-widget-drawer-empty">
                ${t('desktop.widget_drawer_empty')}
            </div>
        `;
        drawer.appendChild(content);
        return;
    }

    const list = document.createElement('div');
    list.className = 'vd-widget-drawer-list';

    allWidgets.forEach(widget => {
        const isVisible = widget.visible !== false;
        const isBuiltin = widget.builtin === true;
        const card = document.createElement('div');
        const iconKey = widget.icon || 'widgets';

        card.className = 'vd-widget-drawer-card';
        card.innerHTML = `
            <div class="vd-widget-drawer-card-icon">${iconMarkup(iconKey, widget.title || widget.id, 'vd-sprite-file', 22)}</div>
            <div class="vd-widget-drawer-card-text">
                <div class="vd-widget-drawer-card-title">
                    ${esc(widget.title || widget.id)}
                </div>
                <div class="vd-widget-drawer-card-meta">
                    ${isBuiltin ? t('desktop.widget_builtin') : t('desktop.widget_custom')}
                    ${isVisible ? ' • ' + t('desktop.widget_on_desktop') : ''}
                </div>
            </div>
            <div>
                <button type="button" class="vd-wm-btn vd-widget-drawer-card-btn" data-widget-action="${isVisible ? 'hide' : 'show'}" data-widget-id="${esc(widget.id)}">
                    ${isVisible ? t('desktop.widget_remove_from_desktop') : t('desktop.widget_add_to_desktop')}
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
                    ? t('desktop.widget_add_to_desktop')
                    : t('desktop.widget_remove_from_desktop');
            }
        });

        list.appendChild(card);
    });

    content.appendChild(list);
    drawer.appendChild(content);
}

function updateTaskbarSystemButtonsForMobile() {
    const isMobile = window.useMobileDesktopMode && window.useMobileDesktopMode();
    const widgetsBtn = document.getElementById('vd-widget-drawer-btn');

    if (widgetsBtn) widgetsBtn.style.display = isMobile ? 'none' : '';
}
