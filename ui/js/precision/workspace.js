(function () {
    'use strict';

    const DENSITY_KEY = 'aurago.workspace.density.v1';
    const LEGACY_DENSITY_KEY = 'aurago.config.density.v1';
    const DEFAULT_DENSITY = 'comfortable';
    const DENSITIES = new Set(['comfortable', 'compact']);
    const ROUTE_ALIASES = {
        '/missions/v2': '/missions',
        '/gallery': '/media'
    };
    const DENSITY_KEYS = {
        toggle: 'common.workspace_density_toggle',
        comfortable: 'common.workspace_density_comfortable',
        compact: 'common.workspace_density_compact'
    };
    const FALLBACK_LABELS = {
        'common.workspace_density_toggle': 'Toggle workspace density',
        'common.workspace_density_comfortable': 'Comfortable',
        'common.workspace_density_compact': 'Compact'
    };
    const RADIAL_ICONS = {
        '/': '<path d="M5 6.5h14v9H9l-4 3v-12Z"/>',
        '/dashboard': '<rect x="4" y="4" width="6" height="6" rx="1"/><rect x="14" y="4" width="6" height="6" rx="1"/><rect x="4" y="14" width="6" height="6" rx="1"/><rect x="14" y="14" width="6" height="6" rx="1"/>',
        '/desktop': '<rect x="3.5" y="4.5" width="17" height="12" rx="2"/><path d="M8 20h8M12 16.5V20"/>',
        '/plans': '<path d="M4 6.5 9 4l6 2.5L20 4v13.5L15 20l-6-2.5L4 20V6.5Z"/><path d="M9 4v13.5M15 6.5V20"/>',
        '/missions': '<path d="m13.5 4 6.5 6.5-7.5 7.5-4-4L16 6.5"/><path d="M8.5 14 4 18.5M5 13l-1 4 4-1"/>',
        '/cheatsheets': '<path d="M7 3.5h8l3 3v14H7a2 2 0 0 1-2-2v-13a2 2 0 0 1 2-2Z"/><path d="M14.5 3.5v4h4M9 12h6M9 16h5"/>',
        '/media': '<path d="M4 6.5h6l2-2h8v15H4v-13Z"/><circle cx="15" cy="11" r="2"/><path d="m7 17 3-3 2 2 2-2 3 3"/>',
        '/knowledge': '<path d="M4.5 5.5A4.5 4.5 0 0 1 9 4h3v15H9a4.5 4.5 0 0 0-4.5 1.5v-15ZM19.5 5.5A4.5 4.5 0 0 0 15 4h-3v15h3a4.5 4.5 0 0 1 4.5 1.5v-15Z"/>',
        '/containers': '<path d="m12 3 8 4.5v9L12 21l-8-4.5v-9L12 3Z"/><path d="m4 7.5 8 4.5 8-4.5M12 12v9"/>',
        '/skills': '<path d="M9 3h6v4h4v6h-4v4H9v-4H5V7h4V3Z"/>',
        '/truenas': '<rect x="4" y="4" width="16" height="16" rx="2"/><path d="M7 8h10M7 12h10M7 16h6"/><circle cx="17" cy="16" r="1"/>',
        '/config': '<circle cx="12" cy="12" r="3"/><path d="M19 13.5v-3l-2.2-.7a7 7 0 0 0-.7-1.6l1.1-2-2.1-2.1-2 1.1a7 7 0 0 0-1.6-.7L10.5 2h-3l-.7 2.2a7 7 0 0 0-1.6.7l-2-1.1-2.1 2.1 1.1 2a7 7 0 0 0-.7 1.6L0 10.5v3l2.2.7a7 7 0 0 0 .7 1.6l-1.1 2 2.1 2.1 2-1.1a7 7 0 0 0 1.6.7l.7 2.2h3l.7-2.2a7 7 0 0 0 1.6-.7l2 1.1 2.1-2.1-1.1-2a7 7 0 0 0 .7-1.6l2.2-.7Z" transform="translate(2 -0.5) scale(.83)"/>',
        '/invasion': '<path d="M7 18c-2-2-2-6 0-9s3-5 5-5 3 2 5 5 2 7 0 9-3 2-5 2-3 0-5-2Z"/><path d="M9 12h.01M15 12h.01M10 16c1 .7 3 .7 4 0"/>',
        logout: '<path d="M10 5H5v14h5M14 8l4 4-4 4M8 12h10"/>'
    };
    const MODAL_SELECTOR = '.modal-overlay';
    const MODAL_FOCUSABLE_SELECTOR = [
        '[autofocus]',
        'a[href]',
        'button:not([disabled])',
        'input:not([disabled]):not([type="hidden"])',
        'select:not([disabled])',
        'textarea:not([disabled])',
        '[tabindex]:not([tabindex="-1"])'
    ].join(',');

    let currentDensity = DEFAULT_DENSITY;
    let initialized = false;
    let radialObserver = null;
    let modalObserver = null;
    let generatedModalTitle = 0;
    const activeModalOverlays = new Set();
    const modalPreviousFocus = new WeakMap();

    function normalizeDensity(value) {
        return DENSITIES.has(value) ? value : DEFAULT_DENSITY;
    }

    function readStorage(key) {
        try {
            return window.localStorage.getItem(key);
        } catch (_) {
            return null;
        }
    }

    function writeStorage(key, value) {
        try {
            window.localStorage.setItem(key, value);
            return true;
        } catch (_) {
            return false;
        }
    }

    function readInitialDensity() {
        const canonical = readStorage(DENSITY_KEY);
        if (DENSITIES.has(canonical)) return canonical;

        const legacy = readStorage(LEGACY_DENSITY_KEY);
        const density = DENSITIES.has(legacy) ? legacy : DEFAULT_DENSITY;
        writeStorage(DENSITY_KEY, density);
        return density;
    }

    function translate(key) {
        try {
            if (typeof window.t === 'function') {
                const translated = window.t(key);
                if (translated && translated !== key) return translated;
            }
        } catch (_) {
            // A translation failure must not block workspace controls.
        }
        return FALLBACK_LABELS[key] || key;
    }

    function syncDensityControls() {
        if (!document.body) return;

        document.body.dataset.density = currentDensity;
        const isCompact = currentDensity === 'compact';
        const valueKey = isCompact ? DENSITY_KEYS.compact : DENSITY_KEYS.comfortable;
        const toggleLabel = translate(DENSITY_KEYS.toggle);
        const valueLabel = translate(valueKey);

        document.querySelectorAll('[data-pw-density-toggle]').forEach((button) => {
            button.setAttribute('aria-pressed', isCompact ? 'true' : 'false');
            button.setAttribute('aria-label', toggleLabel);
            button.setAttribute('title', toggleLabel);
            button.setAttribute('data-i18n-title', DENSITY_KEYS.toggle);
            button.setAttribute('data-i18n-aria-label', DENSITY_KEYS.toggle);

            const label = button.querySelector('[data-pw-density-label], span');
            if (label) {
                label.setAttribute('data-i18n', valueKey);
                label.textContent = valueLabel;
            }
        });
    }

    function dispatchDensityChange(previous) {
        window.dispatchEvent(new CustomEvent('aurago:workspace-density-change', {
            detail: { density: currentDensity, previousDensity: previous }
        }));
    }

    function setDensity(value) {
        if (!initialized) init();
        const density = normalizeDensity(value);
        const previous = currentDensity;
        currentDensity = density;
        syncDensityControls();
        writeStorage(DENSITY_KEY, density);
        if (density !== previous) dispatchDensityChange(previous);
        return density;
    }

    function getDensity() {
        return currentDensity;
    }

    function bindDensityControls() {
        document.querySelectorAll('[data-pw-density-toggle]').forEach((button) => {
            if (button.dataset.pwDensityBound === 'true') return;
            button.dataset.pwDensityBound = 'true';
            button.addEventListener('click', () => {
                setDensity(currentDensity === 'compact' ? 'comfortable' : 'compact');
            });
        });
    }

    function canonicalRoute(pathname) {
        const cleanPath = (pathname || '/').replace(/\/+$/, '') || '/';
        return ROUTE_ALIASES[cleanPath] || cleanPath;
    }

    function routeMatches(currentRoute, targetRoute) {
        if (currentRoute === targetRoute) return true;
        return targetRoute !== '/' && currentRoute.startsWith(targetRoute + '/');
    }

    function outlineIcon(iconKey) {
        const paths = RADIAL_ICONS[iconKey];
        if (!paths) return '';
        return '<svg class="pw-radial-outline-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">' + paths + '</svg>';
    }

    function radialIconKey(item) {
        if (item.matches('[data-logout-action]')) return 'logout';
        const href = item.getAttribute('href');
        if (!href) return '';
        try {
            return canonicalRoute(new URL(href, window.location.origin).pathname);
        } catch (_) {
            return canonicalRoute(href);
        }
    }

    function enhanceRadialMenu() {
        if (!document.body || !document.body.classList.contains('pw-page')) return;
        const anchor = document.getElementById('radialMenuAnchor');
        if (!anchor) return;

        const currentRoute = canonicalRoute(window.location.pathname);
        anchor.querySelectorAll('.radial-item').forEach((item) => {
            const iconKey = radialIconKey(item);
            if (item.matches('a[href]')) {
                if (routeMatches(currentRoute, iconKey)) {
                    item.setAttribute('aria-current', 'page');
                } else {
                    item.removeAttribute('aria-current');
                }
            }

            const icon = item.querySelector('.radial-item-icon');
            if (!icon || !RADIAL_ICONS[iconKey] || icon.dataset.pwIcon === iconKey) return;
            icon.innerHTML = outlineIcon(iconKey);
            icon.dataset.pwIcon = iconKey;
        });
    }

    function observeRadialMenu() {
        const anchor = document.getElementById('radialMenuAnchor');
        if (!anchor || radialObserver) return;

        radialObserver = new MutationObserver(enhanceRadialMenu);
        radialObserver.observe(anchor, { childList: true, subtree: true });
        enhanceRadialMenu();
    }

    function modalFocusableElements(overlay) {
        return Array.from(overlay.querySelectorAll(MODAL_FOCUSABLE_SELECTOR)).filter((element) => {
            if (element.getAttribute('aria-hidden') === 'true') return false;
            return element.getClientRects().length > 0;
        });
    }

    function isModalOpen(overlay) {
        if (!overlay || !overlay.isConnected) return false;
        if (overlay.classList.contains('active') || overlay.classList.contains('open')) return true;
        const inlineDisplay = overlay.style.display;
        return Boolean(inlineDisplay && inlineDisplay !== 'none');
    }

    function focusModal(overlay) {
        const focusable = modalFocusableElements(overlay);
        const target = focusable[0] || overlay;
        if (target === overlay && !overlay.hasAttribute('tabindex')) {
            overlay.setAttribute('tabindex', '-1');
        }
        target.focus({ preventScroll: true });
    }

    function activateModal(overlay) {
        if (activeModalOverlays.has(overlay)) return;
        activeModalOverlays.add(overlay);
        const previousFocus = document.activeElement;
        if (previousFocus && previousFocus !== document.body) {
            modalPreviousFocus.set(overlay, previousFocus);
        }
        overlay.dataset.pwModalActive = 'true';
        window.requestAnimationFrame(() => {
            if (!isModalOpen(overlay)) return;
            if (!overlay.contains(document.activeElement)) focusModal(overlay);
        });
    }

    function deactivateModal(overlay) {
        if (!activeModalOverlays.delete(overlay)) return;
        delete overlay.dataset.pwModalActive;
        const previousFocus = modalPreviousFocus.get(overlay);
        modalPreviousFocus.delete(overlay);
        if (!previousFocus || !previousFocus.isConnected || typeof previousFocus.focus !== 'function') return;
        window.requestAnimationFrame(() => previousFocus.focus({ preventScroll: true }));
    }

    function handleModalKeydown(overlay, event) {
        if (event.key !== 'Tab' || !isModalOpen(overlay)) return;
        const focusable = modalFocusableElements(overlay);
        if (focusable.length === 0) {
            event.preventDefault();
            focusModal(overlay);
            return;
        }

        const first = focusable[0];
        const last = focusable[focusable.length - 1];
        if (event.shiftKey && (document.activeElement === first || !overlay.contains(document.activeElement))) {
            event.preventDefault();
            last.focus();
        } else if (!event.shiftKey && document.activeElement === last) {
            event.preventDefault();
            first.focus();
        }
    }

    function syncModalOverlay(overlay) {
        if (isModalOpen(overlay)) {
            activateModal(overlay);
        } else {
            deactivateModal(overlay);
        }
    }

    function dialogTargetForOverlay(overlay) {
        return overlay.querySelector('[role="dialog"]') || overlay;
    }

    function syncModalSemantics(overlay) {
        const dialogTarget = dialogTargetForOverlay(overlay);
        if (dialogTarget !== overlay) {
            overlay.removeAttribute('role');
            overlay.removeAttribute('aria-modal');
            overlay.removeAttribute('aria-labelledby');
        } else if (!dialogTarget.hasAttribute('role')) {
            dialogTarget.setAttribute('role', 'dialog');
        }
        dialogTarget.setAttribute('aria-modal', 'true');

        const labelledIds = (dialogTarget.getAttribute('aria-labelledby') || '').trim().split(/\s+/).filter(Boolean);
        if (labelledIds.length && labelledIds.every((labelledBy) => document.getElementById(labelledBy))) return;
        if (dialogTarget.hasAttribute('aria-label') && dialogTarget.getAttribute('aria-label').trim()) return;

        let heading = dialogTarget.querySelector('.modal-title[id]');
        if (!heading) heading = dialogTarget.querySelector('.modal-title');
        if (!heading) heading = dialogTarget.querySelector('.modal-header h1, .modal-header h2, .modal-header h3');
        if (!heading) heading = dialogTarget.querySelector('h1, h2, h3');
        if (!heading) return;

        if (!heading.id) {
            generatedModalTitle += 1;
            heading.id = 'pw-modal-title-' + generatedModalTitle;
        }
        dialogTarget.setAttribute('aria-labelledby', heading.id);
    }

    function enhanceModalOverlay(overlay) {
        if (!overlay || !overlay.matches(MODAL_SELECTOR)) return;
        syncModalSemantics(overlay);
        if (overlay.dataset.pwModalBound === 'true') {
            syncModalOverlay(overlay);
            return;
        }

        overlay.dataset.pwModalBound = 'true';
        overlay.addEventListener('keydown', (event) => handleModalKeydown(overlay, event));
        syncModalOverlay(overlay);
    }

    function modalOverlaysWithin(node) {
        if (!(node instanceof Element)) return [];
        const overlays = Array.from(node.querySelectorAll(MODAL_SELECTOR));
        if (node.matches(MODAL_SELECTOR)) overlays.unshift(node);
        return overlays;
    }

    function observeModalOverlays() {
        if (!document.body || modalObserver) return;
        document.querySelectorAll(MODAL_SELECTOR).forEach(enhanceModalOverlay);

        modalObserver = new MutationObserver((records) => {
            records.forEach((record) => {
                if (record.type === 'attributes') {
                    if (record.target.matches(MODAL_SELECTOR)) enhanceModalOverlay(record.target);
                    return;
                }
                record.addedNodes.forEach((node) => modalOverlaysWithin(node).forEach(enhanceModalOverlay));
                record.removedNodes.forEach((node) => modalOverlaysWithin(node).forEach(deactivateModal));
                const containingOverlay = record.target.closest(MODAL_SELECTOR);
                if (containingOverlay) enhanceModalOverlay(containingOverlay);
            });
        });
        modalObserver.observe(document.body, {
            childList: true,
            subtree: true,
            attributes: true,
            attributeFilter: ['class', 'style']
        });
    }

    function init() {
        if (!document.body || !document.body.classList.contains('pw-page')) return currentDensity;

        if (!initialized) {
            currentDensity = readInitialDensity();
            initialized = true;
        }
        bindDensityControls();
        syncDensityControls();
        observeRadialMenu();
        observeModalOverlays();
        return currentDensity;
    }

    window.AuraPrecisionWorkspace = {
        init: init,
        getDensity: getDensity,
        setDensity: setDensity
    };

    if (document.body) {
        init();
    } else {
        document.addEventListener('DOMContentLoaded', init, { once: true });
    }
}());
