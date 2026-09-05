// Config-only presentation. Never changes field values, draft state, or integration requests.
(function () {
    'use strict';

    let nextID = 0;
    let frame = 0;
    const root = document.body;
    if (root?.dataset.workspacePage !== 'config') return;
    const icons = {
        '💾': 'M5 3h12l4 4v14H3V3h2Zm2 0v6h10V3M7 21v-8h10v8',
        '✏': 'm4 16 12-12 4 4L8 20H4v-4Zm10-10 4 4',
        '🗑': 'M3 6h18M9 6V3h6v3M5 6l1 15h12l1-15M10 10v7M14 10v7',
        '🔌': 'M8 3v5M16 3v5M6 8h12v3a6 6 0 0 1-12 0V8Zm6 9v4',
        '🔄': 'M20 4v6h-6M4 20v-6h6M5 8a8 8 0 0 1 14-3M19 16a8 8 0 0 1-14 3',
        '↻': 'M20 4v6h-6M5 8a8 8 0 1 0 14-3',
        '＋': 'M12 4v16M4 12h16',
        '➕': 'M12 4v16M4 12h16',
        '✕': 'm6 6 12 12M6 18 18 6',
        '☀': 'M12 3v2M12 19v2M3 12h2M19 12h2M5.6 5.6 7 7M17 17l1.4 1.4M5.6 18.4 7 17M17 7l1.4-1.4M16 12a4 4 0 1 1-8 0 4 4 0 0 1 8 0',
        '🌙': 'M20 15A9 9 0 0 1 9 4a9 9 0 1 0 11 11'
    };
    icons['🌞'] = icons['☀'];

    function identify(element) {
        if (!element.id) element.id = 'cfg-control-label-' + (++nextID);
        return element.id;
    }

    function describe(control, attribute, element) {
        if (!element) return;
        const ids = new Set((control.getAttribute(attribute) || '').split(/\s+/).filter(Boolean));
        const id = identify(element);
        if (ids.has(id)) return;
        ids.add(id);
        control.setAttribute(attribute, [...ids].join(' '));
    }

    function enhance(scope) {
        const language = document.getElementById('ui-lang-switcher');
        const header = document.querySelector('.cfg-header .header-actions');
        if (language && header && language.parentElement !== header) header.append(language);
        scope.querySelectorAll('button').forEach(button => {
            if (button.querySelector('svg, img')) return;
            const node = [...button.childNodes].find(child => child.nodeType === Node.TEXT_NODE && child.textContent.trim());
            if (!node) return;
            const text = node.textContent.trimStart();
            const symbol = Object.keys(icons).find(key => text.startsWith(key));
            if (!symbol) return;
            const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
            svg.setAttribute('class', 'cfg-ui-icon');
            svg.setAttribute('viewBox', '0 0 24 24');
            svg.setAttribute('aria-hidden', 'true');
            const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
            path.setAttribute('d', icons[symbol]);
            svg.append(path);
            node.textContent = text.slice(symbol.length).replace(/^\uFE0F?\s*/, '');
            node.before(svg);
            if (!button.textContent.trim() && !button.hasAttribute('aria-label')) {
                const label = { '✕': 'cancel', '🗑': 'delete', '✏': 'edit', '💾': 'save' }[symbol];
                if (button.title || label) button.setAttribute('aria-label', button.title || t('config.common.' + label));
            }
        });
        // Let the existing workspace controller own trapping and restoring dialog focus.
        scope.querySelectorAll('[class*="modal-overlay"], .prov-or-overlay, .prov-pricing-overlay').forEach(overlay => {
            if (overlay.hasAttribute('role') || overlay.querySelector('[role="dialog"]')) return;
            const heading = overlay.querySelector('[class*="modal-title"], h1, h2, h3');
            if (!heading) return;
            overlay.setAttribute('role', 'dialog');
            overlay.setAttribute('aria-modal', 'true');
            overlay.setAttribute('aria-labelledby', identify(heading));
            overlay.classList.add('cfg-dialog');
        });
        scope.querySelectorAll('.section-header').forEach(heading => {
            if (heading.closest('.modal-overlay')) return;
            heading.setAttribute('role', 'heading');
            heading.setAttribute('aria-level', '1');
        });
        scope.querySelectorAll('.field-group').forEach(group => {
            group.classList.add('pw-field');
            const label = group.querySelector(':scope > .field-label');
            const help = group.querySelector(':scope > .field-help');
            const toggle = group.querySelector(':scope > .toggle, :scope > .toggle-wrap');
            group.classList.toggle('cfg-switch-field', !!toggle && !!label);
            // The help follows the editor in both visual and screen-reader order.
            if (help && !toggle && help.nextElementSibling && !help.nextElementSibling.matches('.pw-field-error, .cfg-save-scope')) {
                const editor = group.querySelector(':scope > input, :scope > select, :scope > textarea, :scope > .password-wrap, :scope > .adg-password-row, :scope > .field-select-wrap');
                if (editor && editor.previousElementSibling === help) editor.after(help);
            }
            group.querySelectorAll('input, select, textarea, .toggle[data-path]').forEach(control => {
                if (control.closest('.field-group') !== group) return;
                if (!control.labels?.length && !control.hasAttribute('aria-label')) describe(control, 'aria-labelledby', label);
                describe(control, 'aria-describedby', help);
            });
        });
        scope.querySelectorAll('.cfg-toggle-row, .cfg-toggle-row-highlight, .cfg-toggle-row-compact').forEach(row => {
            const label = row.querySelector('.cfg-toggle-label');
            const toggle = row.querySelector('.toggle');
            if (label && toggle) describe(toggle, 'aria-labelledby', label);
        });
        scope.querySelectorAll('.toggle[data-path]').forEach(toggle => {
            // Hidden binding mirrors must not acquire a visible state label.
            if (toggle.matches('[hidden], [aria-hidden="true"], .pw-u-hidden')) return;
            if (!toggle.parentElement.classList.contains('toggle-wrap')) {
                const wrap = document.createElement('div');
                wrap.className = 'toggle-wrap';
                const state = document.createElement('span');
                state.className = 'toggle-label';
                state.textContent = t(toggle.classList.contains('on') ? 'config.toggle.active' : 'config.toggle.inactive');
                toggle.before(wrap);
                wrap.append(toggle, state);
            }
        });
        scope.querySelectorAll('.password-toggle').forEach(button => {
            if (button.hasAttribute('aria-label') || button.title) return;
            button.setAttribute('aria-label', t('config.refresh.toggle_password'));
        });
        scope.querySelectorAll('.cfg-status-banner, .adg-status-banner, .pw-status, .cfg-action-status').forEach(status => {
            if (!status.hasAttribute('role')) status.setAttribute('role', 'status');
            if (!status.hasAttribute('aria-live')) status.setAttribute('aria-live', 'polite');
        });
        scope.querySelectorAll('.password-wrap').forEach(wrap => {
            const row = wrap.closest('.field-group');
            if (!row || row.querySelector('.cfg-save-scope')) return;
            // A credential action is independent of the page-level config save.
            if (row.querySelector('button[onclick*="Save"], button[onclick*="save"]:not(.password-toggle)')) {
                const note = document.createElement('small');
                note.className = 'cfg-save-scope';
                note.textContent = t('config.refresh.credential_save');
                row.append(note);
            }
        });
    }

    const observer = new MutationObserver(() => {
        if (frame) return;
        frame = requestAnimationFrame(() => {
            frame = 0;
            observer.disconnect();
            enhance(root);
            if (typeof enhanceConfigSectionLayout === 'function' && typeof activeSection === 'string') {
                enhanceConfigSectionLayout(activeSection);
            }
            observer.observe(root, { childList: true, subtree: true });
        });
    });
    window.AuraConfigPresentation = { enhance };
    enhance(root);
    observer.observe(root, { childList: true, subtree: true });

    document.querySelector('[data-config-action="save"]')?.addEventListener('click', () => saveConfig());
    document.querySelector('[data-config-action="restart"]')?.addEventListener('click', () => restartAuraGo());
})();
