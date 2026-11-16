/* Runenpfad GPU post-processing (stub; final API, no-op until its batch lands). */
(function (root) {
    'use strict';

    const api = { create: function () { return Promise.resolve(null); } };

    root.RunenpfadPostfx = api;
})(typeof window !== 'undefined' ? window : globalThis);
