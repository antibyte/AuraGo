(function () {
    'use strict';

    // Orbit particles (global AuraOrbitParticles). A DOM-free particle pool with the emitters of the theme (materialize,
    // dissolve, stream, thruster, ping and stardust) plus a Canvas 2D additive renderer that draws into the effect layer
    // only while particles are alive. The pool is bounded, so a burst of window events never grows memory. This file is
    // the scaffold; the pool, the emitters and the renderer arrive with the choreography work.
    window.AuraOrbitParticles = Object.freeze({});
}());
