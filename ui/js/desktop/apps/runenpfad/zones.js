/* Runenpfad zone archetypes (stub; final API, no-op until its batch lands). */
(function (root) {
    'use strict';

    const api = {
        ZONE_IDS: [],
        has: function () { return false; },
        build: function () { return 0; },
        step: function () {},
        applyForces: function () {},
        interact: function () {}
    };

    root.RunenpfadZones = api;
})(typeof window !== 'undefined' ? window : globalThis);
