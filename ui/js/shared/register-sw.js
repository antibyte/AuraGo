// Register the cache-static-assets service worker when served over HTTP(S).
(function () {
    'use strict';
    if (!('serviceWorker' in navigator) || location.protocol === 'file:') return;

    var script = document.currentScript;
    var swUrl = (script && script.dataset.swUrl) || '/sw.js';
    navigator.serviceWorker.register(swUrl)
        .catch(function (err) { console.warn('Service Worker registration failed:', err); });
}());
