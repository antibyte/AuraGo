// AuraGo dashboard – section router (window.DashRouter). Classic script, loaded with defer before main.js.
// The pure part (SECTIONS, LEGACY_SECTION_MAP, resolveSection) touches no DOM, so the file also loads in a
// bare Node vm context (ui/testdata/dash-router.cjs). init() wires the sidebar links, hash changes, the menu
// button, the backdrop, Escape and the drawer media query; show() switches the visible section.
(function () {
    'use strict';

    const SECTIONS = Object.freeze([
        'overview', 'personality', 'memory', 'user', 'knowledge', 'filesync', 'problems',
        'automations', 'system', 'audit', 'logs', 'llm', 'prompts', 'tools',
    ]);
    // Old tab ids (bookmarked hashes, localStorage 'aurago-dash-tab') -> section ids.
    const LEGACY_SECTION_MAP = Object.freeze({
        overview: 'overview', agent: 'personality', user: 'user', knowledge: 'knowledge',
        filesync: 'filesync', audit: 'audit', cronjobs: 'automations', system: 'system',
    });
    const STORAGE_KEY = 'aurago-dash-section';
    const LEGACY_STORAGE_KEY = 'aurago-dash-tab';
    const DRAWER_QUERY = '(max-width: 900px)';

    // '#x' or 'x' -> section id (legacy ids mapped); '' when the value names no section.
    function sectionId(value) {
        if (typeof value !== 'string') return '';
        const id = value.charAt(0) === '#' ? value.slice(1) : value;
        if (SECTIONS.indexOf(id) !== -1) return id;
        return Object.prototype.hasOwnProperty.call(LEGACY_SECTION_MAP, id) ? LEGACY_SECTION_MAP[id] : '';
    }

    // Pure: hash ('#x', 'x' or '') and stored value (string or null) -> section id; fallback 'overview'.
    function resolveSection(hash, stored) {
        return sectionId(hash) || sectionId(stored) || 'overview';
    }

    let active = null;
    let onShowCallback = null;
    let initialized = false;
    let sidebar = null;
    let backdrop = null;
    let menuButton = null;
    let drawerMedia = null;

    // Stored section; migrates the old 'aurago-dash-tab' value once. The page works without storage.
    function readStoredSection() {
        try {
            const storage = window.localStorage;
            let stored = storage.getItem(STORAGE_KEY);
            const legacy = storage.getItem(LEGACY_STORAGE_KEY);
            if (legacy !== null) {
                storage.removeItem(LEGACY_STORAGE_KEY);
                if (!stored && sectionId(legacy)) {
                    stored = sectionId(legacy);
                    storage.setItem(STORAGE_KEY, stored);
                }
            }
            return stored;
        } catch (_) {
            return null;
        }
    }

    function storeSection(id) {
        try {
            window.localStorage.setItem(STORAGE_KEY, id);
        } catch (_) { /* storage unavailable */ }
    }

    function translate(key) {
        return typeof window.t === 'function' ? window.t(key) : key;
    }

    function drawerMode() {
        return !!(drawerMedia && drawerMedia.matches);
    }

    function drawerIsOpen() {
        return !!(sidebar && sidebar.classList.contains('open'));
    }

    // The sidebar is inert while it is a closed drawer, so Tab never lands on off-canvas links.
    function setDrawer(open) {
        if (!sidebar) return;
        const visible = open && drawerMode();
        sidebar.classList.toggle('open', visible);
        sidebar.inert = drawerMode() && !visible;
        if (backdrop) backdrop.hidden = !visible;
        if (menuButton) menuButton.setAttribute('aria-expanded', visible ? 'true' : 'false');
    }

    function openDrawer() {
        if (!sidebar || !drawerMode()) return;
        setDrawer(true);
        const target = sidebar.querySelector('[data-section-link][aria-current="page"]') || sidebar.querySelector('[data-section-link]');
        if (target) target.focus();
    }

    // Closes the drawer (also used for media-query changes); focus inside the sidebar returns to the menu button.
    function closeDrawer() {
        if (!sidebar) return;
        const hadFocus = sidebar.contains(document.activeElement);
        setDrawer(false);
        if (hadFocus && drawerMode() && menuButton) menuButton.focus();
    }

    function show(id, options) {
        const target = sectionId(id) || 'overview';
        const moveFocus = !options || options.focus !== false;
        document.querySelectorAll('.dash-section').forEach(section => {
            section.hidden = section.getAttribute('data-section') !== target;
        });
        document.querySelectorAll('[data-section-link]').forEach(link => {
            if (link.getAttribute('data-section-link') === target) link.setAttribute('aria-current', 'page');
            else link.removeAttribute('aria-current');
        });
        if (window.location.hash !== '#' + target) {
            try {
                window.history.replaceState(null, '', '#' + target);
            } catch (_) { /* sandboxed frame */ }
        }
        storeSection(target);
        document.title = translate('dashboard.section.' + target + '.title') + ' · ' + translate('dashboard.page_title');
        closeDrawer();
        active = target;
        if (moveFocus) {
            window.scrollTo(0, 0);
            const heading = document.getElementById('sec-' + target + '-title');
            if (heading) heading.focus({ preventScroll: true });
        }
        if (onShowCallback) {
            try {
                onShowCallback(target);
            } catch (err) {
                console.warn('DashRouter onShow failed', target, err);
            }
        }
    }

    function onDocumentClick(event) {
        if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
        const link = event.target && typeof event.target.closest === 'function' ? event.target.closest('a[data-section-link]') : null;
        if (!link) return;
        const id = sectionId(link.getAttribute('data-section-link'));
        if (!id) return;
        event.preventDefault();
        if (id !== active) {
            try {
                window.history.pushState(null, '', '#' + id);
            } catch (_) { /* show() still sets the hash via replaceState */ }
        }
        show(id);
    }

    // Back/forward, typed hashes and plain href="#<id>" links. Hashes that name no section
    // (the skip link's #main-content) are ignored.
    function onLocationChange() {
        const id = sectionId(window.location.hash);
        if (!id) return;
        if (id === active) {
            if (window.location.hash !== '#' + id) {
                try {
                    window.history.replaceState(null, '', '#' + id);
                } catch (_) { /* sandboxed frame */ }
            }
            return;
        }
        show(id);
    }

    function onKeydown(event) {
        if (!drawerIsOpen()) return;
        if (event.key === 'Escape') {
            event.preventDefault();
            setDrawer(false);
            if (menuButton) menuButton.focus();
            return;
        }
        if (event.key !== 'Tab' || !sidebar.contains(document.activeElement)) return;
        const items = Array.from(sidebar.querySelectorAll('a[href], button:not([disabled])')).filter(el => el.getClientRects().length > 0);
        if (!items.length) return;
        const first = items[0];
        const last = items[items.length - 1];
        if (event.shiftKey && document.activeElement === first) {
            event.preventDefault();
            last.focus();
        } else if (!event.shiftKey && document.activeElement === last) {
            event.preventDefault();
            first.focus();
        }
    }

    function onMenuClick() {
        if (drawerIsOpen()) closeDrawer();
        else openDrawer();
    }

    function init(options) {
        onShowCallback = options && typeof options.onShow === 'function' ? options.onShow : null;
        if (!initialized) {
            initialized = true;
            sidebar = document.getElementById('dash-sidebar');
            backdrop = document.getElementById('dash-sidebar-backdrop');
            menuButton = document.getElementById('dash-menu-btn');
            drawerMedia = typeof window.matchMedia === 'function' ? window.matchMedia(DRAWER_QUERY) : null;
            document.addEventListener('click', onDocumentClick);
            document.addEventListener('keydown', onKeydown);
            window.addEventListener('hashchange', onLocationChange);
            window.addEventListener('popstate', onLocationChange);
            if (menuButton) menuButton.addEventListener('click', onMenuClick);
            if (backdrop) backdrop.addEventListener('click', () => closeDrawer());
            if (drawerMedia && typeof drawerMedia.addEventListener === 'function') {
                drawerMedia.addEventListener('change', () => closeDrawer());
            }
        }
        // The initial section does not steal focus or scroll.
        show(resolveSection(window.location.hash, readStoredSection()), { focus: false });
    }

    window.DashRouter = {
        SECTIONS,
        LEGACY_SECTION_MAP,
        resolveSection,
        current: () => active,
        init,
        show,
        openDrawer,
        closeDrawer,
    };
})();
