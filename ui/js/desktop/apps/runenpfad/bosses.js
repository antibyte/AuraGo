/* Runenpfad boss attack patterns (stub; final API, no-op until its batch lands). */
(function (root) {
    'use strict';

    const api = {
        update: function () { return false; },
        onDefeat: function () {}
    };

    root.RunenpfadBosses = api;
})(typeof window !== 'undefined' ? window : globalThis);
