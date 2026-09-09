    function renderLeafyWidget(container) {
        let disposed = false, runtime = null;
        registerWidgetCleanup(() => { disposed = true; runtime?.dispose(); });
        container.closest('.vd-widget')?.setAttribute('data-leafy-owner', '');
        async function start() {
            try {
                await window.AuraLazyAssets.loadAll({
                    styles: ['/css/desktop-leafy.css'],
                    scripts: ['/js/desktop/leafy/geometry.js', '/js/desktop/leafy/fallback.js', '/js/desktop/leafy/runtime.js']
                });
                if (!disposed) runtime = window.AuraLeafy.mount({ api, t, esc, confirm: confirmDialog, readonly: desktopReadonly });
            } catch (err) {
                if (!disposed) {
                    container.closest('.vd-widget')?.removeAttribute('data-leafy-owner');
                    container.textContent = t('desktop.leafy_error');
                    const retry = document.createElement('button');
                    retry.textContent = t('desktop.leafy_retry');
                    retry.onclick = () => { container.textContent = ''; container.closest('.vd-widget')?.setAttribute('data-leafy-owner', ''); start(); };
                    container.appendChild(retry);
                }
            }
        }
        start();
    }
