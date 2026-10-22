(function () {
    'use strict';

    // Orbit scene (global AuraOrbitScene). Builds the live WebGL scene behind the shell on the canvas .vd-fx-orbit-scene:
    // renderer, camera, post-processing composer, loading of the station, shuttle, satellite and asteroid models, the
    // gas giant, its rings and the sun, the quality ladder that steps down when frames get slow, and the disposal of every
    // GPU resource. Three.js comes from the lazily loaded vendor bundle. This file is the scaffold; the scene arrives with
    // the WebGL work.
    window.AuraOrbitScene = Object.freeze({});
}());
