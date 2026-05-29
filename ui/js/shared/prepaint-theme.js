(function () {
    'use strict';
    try {
        var theme = localStorage.getItem('aurago-theme');
        if (theme) document.documentElement.setAttribute('data-theme', theme);
    } catch (_) { }
})();
