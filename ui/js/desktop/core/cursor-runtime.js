    // Native CSS cursors: no pointer-following DOM, animation loop or input interception.
    const DESKTOP_CURSOR_ROLES = Object.freeze({
        default: [3, 2], pointer: [13, 3], text: [16, 16], grab: [16, 16], grabbing: [16, 16],
        move: [16, 16], 'ew-resize': [16, 16], 'ns-resize': [16, 16],
        'nesw-resize': [16, 16], 'nwse-resize': [16, 16], 'not-allowed': [16, 16], wait: [16, 16]
    });

    function applyDesktopCursors(body, theme) {
        const id = settingValue('appearance.theme_cursors') !== 'false' ? theme.cursorTheme : '';
        if (body.dataset.cursorTheme === (id || 'system')) return;
        body.dataset.cursorTheme = id || 'system';
        Object.entries(DESKTOP_CURSOR_ROLES).forEach(([role, hotspot]) => {
            const property = '--vd-cursor-' + role;
            if (!id) { body.style.removeProperty(property); return; }
            const path = '/img/cursors/' + id + '/' + role + '.svg';
            const url = window.AuraLazyAssets ? window.AuraLazyAssets.versionedURL(path) : path;
            body.style.setProperty(property, 'url("' + url + '") ' + hotspot.join(' ') + ', ' + role);
        });
    }
