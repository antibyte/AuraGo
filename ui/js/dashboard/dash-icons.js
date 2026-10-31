// ══════════════════════════════════════════════════════════════════════════════
// Dashboard SVG Icon System (shim)
// The path table lives in js/precision/icons.js (AuraIcons); this file keeps the
// dashboard contract: dashIcon(), dashIconEl(), the #ic-* sprite and `dash-ic`.
// Usage: dashIcon('home') -> '<svg class="dash-ic dash-ic-home" ...><use href="#ic-home"/></svg>'
// Or:   dashIconEl('home') -> DOM element
// Load order: icons.js first, then this file (both deferred).
// ══════════════════════════════════════════════════════════════════════════════

(function () {
    'use strict';

    const ICON_PATHS = window.AuraIcons ? window.AuraIcons.paths() : {};

    function markup(name, cls) {
        const extra = cls ? ' ' + cls : '';
        return '<svg class="dash-ic dash-ic-' + name + extra + '" aria-hidden="true" focusable="false"><use href="#ic-' + name + '"/></svg>';
    }

    /**
     * Generate SVG icon markup string for use in innerHTML templates.
     * @param {string} name - Icon name
     * @param {string} [cls] - Additional CSS classes
     * @returns {string} SVG HTML string
     */
    function dashIcon(name, cls) {
        if (!Object.prototype.hasOwnProperty.call(ICON_PATHS, name)) return '';
        return markup(name, cls);
    }

    /**
     * Generate SVG icon DOM element.
     * @param {string} name - Icon name
     * @param {string} [cls] - Additional CSS classes
     * @returns {SVGElement} SVG element, or empty span if name is invalid
     */
    function dashIconEl(name, cls) {
        if (!Object.prototype.hasOwnProperty.call(ICON_PATHS, name)) {
            const fallback = document.createElement('span');
            fallback.setAttribute('aria-hidden', 'true');
            return fallback;
        }
        const temp = document.createElement('div');
        temp.innerHTML = markup(name, cls);
        return temp.firstElementChild;
    }

    // This script is loaded with `defer`: the DOM is parsed by the time it runs.
    if (window.AuraIcons) window.AuraIcons.injectSprite('ic-');

    window.dashIcon = dashIcon;
    window.dashIconEl = dashIconEl;
    window.DASH_ICONS = ICON_PATHS;
})();
