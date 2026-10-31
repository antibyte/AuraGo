(function () {
    'use strict';
    try {
        var theme = localStorage.getItem('aurago-theme');
        var script = document.currentScript;
        if (script && script.dataset && script.dataset.themeScope === 'workspace') {
            // Workspace pages only style light/dark; map chat themes to their family.
            // Keep in sync with workspaceThemeFamily() in shared-chat.js. localStorage is untouched.
            document.documentElement.dataset.themeScope = 'workspace';
            if (theme) theme = (theme === 'light' || theme === 'papyrus' || theme === 'lollipop') ? 'light' : 'dark';
        }
        if (theme) document.documentElement.setAttribute('data-theme', theme);
        if (theme === 'galaxy') {
            var size = window.innerWidth < 768 ? 'mobile' : window.innerWidth >= 1600 ? '4k' : '2k';
            var version = new URL(document.currentScript.src).search;
            document.documentElement.style.setProperty('--galaxy-poster', 'url("/img/galaxy/poster-' + size + '.webp' + version + '")');
        }
    } catch (_) { }
})();
