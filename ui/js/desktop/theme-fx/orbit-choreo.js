(function () {
    'use strict';

    // Orbit choreography (global AuraOrbitChoreo). DOM-free logic for the live space scene: a seeded random generator,
    // composition anchors for the planet, station and sun per screen aspect, shuttle and comet schedules, the slow camera
    // drift and a damped spring shared by the scene and the window bridge. It touches neither the DOM nor WebGL, so the
    // Node regression suite can drive it. This file is the scaffold; the logic arrives with the scene work.
    window.AuraOrbitChoreo = Object.freeze({});
}());
