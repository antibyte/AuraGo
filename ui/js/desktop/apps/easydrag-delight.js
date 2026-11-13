// EasyDrag motion gate ("Lebendigkeit"). One place decides whether anything may move. The level is
// "calm" when the user chose Ruhig, when the system asks for reduced motion, or when the desktop turned
// animations off; otherwise "normal". Each .ed-app root carries data-motion="calm|normal", and the
// stylesheet runs animations only under data-motion="normal". The choice is per device (localStorage) and
// never part of a flow document, so it never bumps model.version.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const KEY = 'aurago.easydrag.liveliness';
    const CHOICES = ['normal', 'calm'];
    const roots = new Set();
    let watching = false;

    // choice is what the user picked; normal is the default.
    function choice() {
        try { return localStorage.getItem(KEY) === 'calm' ? 'calm' : 'normal'; } catch (err) { return 'normal'; }
    }

    function systemCalm() {
        try { return !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches); } catch (err) { return false; }
    }

    // desktopOff reads the attribute that desktop-foundation.js sets from the setting windows.animations.
    function desktopOff() {
        const body = document.body;
        return !!(body && body.dataset && body.dataset.animations === 'false');
    }

    // level is 'calm' or 'normal'.
    function level() {
        return choice() === 'calm' || systemCalm() || desktopOff() ? 'calm' : 'normal';
    }

    // allow says whether a motion of this kind may run. kind names the motion ('run', 'drag', 'insert',
    // 'loading'); every kind follows the level today.
    function allow(kind) {
        return level() === 'normal';
    }

    // refresh writes the current level to every bound root and forgets roots that left the document.
    function refresh() {
        const value = level();
        roots.forEach(root => {
            if (!root.isConnected) { roots.delete(root); return; }
            if (root.getAttribute('data-motion') !== value) root.setAttribute('data-motion', value);
        });
        return value;
    }

    // watch starts the three listeners once: the media query, the desktop attribute and other tabs.
    function watch() {
        if (watching) return;
        watching = true;
        try {
            const query = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)');
            if (query && typeof query.addEventListener === 'function') query.addEventListener('change', refresh);
            else if (query && typeof query.addListener === 'function') query.addListener(refresh);
            if (typeof MutationObserver === 'function' && document.body) new MutationObserver(refresh).observe(document.body, { attributes: true, attributeFilter: ['data-animations'] });
            if (typeof window.addEventListener === 'function') window.addEventListener('storage', event => { if (!event.key || event.key === KEY) refresh(); });
        } catch (err) { /* a missing browser feature leaves the level as of the last refresh */ }
    }

    // bind sets data-motion on a .ed-app root and keeps it current; the returned function unbinds it.
    function bind(root) {
        roots.add(root);
        watch();
        refresh();
        return () => { roots.delete(root); };
    }

    // setLevel stores the user's choice ('normal' or 'calm') and returns the level now in effect.
    function setLevel(value) {
        if (!CHOICES.includes(value)) return level();
        try { localStorage.setItem(KEY, value); } catch (err) { /* storage may be blocked: the choice then lasts until reload */ }
        return refresh();
    }

    // menuItems returns the "Lebendigkeit" entry of the editor's more menu: one item with two choices,
    // and a check mark on the current one.
    function menuItems(t) {
        const current = choice();
        const mark = String.fromCodePoint(0x2713);
        const pick = value => () => { setLevel(value); };
        return [{
            icon: 'sparkles', label: t('easydrag.ui.liveliness'),
            items: [
                { id: 'liveliness-normal', icon: 'play', label: t('easydrag.ui.liveliness_normal'), shortcutHint: current === 'normal' ? mark : '', action: pick('normal') },
                { id: 'liveliness-calm', icon: 'pause', label: t('easydrag.ui.liveliness_calm'), shortcutHint: current === 'calm' ? mark : '', action: pick('calm') }
            ]
        }];
    }

    ED.delight = { allow, level, choice, setLevel, bind, refresh, menuItems };
})();
