(function () {
    'use strict';
    try {
        var theme = localStorage.getItem('aurago-theme');
        if (theme) document.documentElement.setAttribute('data-theme', theme);
        if (theme === 'galaxy') {
            var size = window.innerWidth < 768 ? 'mobile' : window.innerWidth >= 1600 ? '4k' : '2k';
            var version = new URL(document.currentScript.src).search;
            document.documentElement.style.setProperty('--galaxy-poster', 'url("/img/galaxy/poster-' + size + '.webp' + version + '")');
        }
    } catch (_) { }
})();
