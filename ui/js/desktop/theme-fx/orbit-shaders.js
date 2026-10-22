(function () {
    'use strict';

    // Orbit shaders (global AuraOrbitShaders). The GLSL sources of the space scene as plain strings: the nebula skydome,
    // the star layers, the gas giant with its atmosphere rim, the ring system, the sun with its flare and the final grade
    // pass (vignette, aberration, grain). The scene module compiles them; this file holds no WebGL calls. This file is the
    // scaffold; the sources arrive with the scene work.
    window.AuraOrbitShaders = Object.freeze({});
}());
