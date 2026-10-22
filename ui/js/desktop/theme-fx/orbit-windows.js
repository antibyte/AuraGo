(function () {
    'use strict';

    // Orbit window bridge (global AuraOrbitWindows). Listens to the aurago:desktop-window events and turns them into CSS
    // variables and particles: drag velocity drives tilt springs (--vd-orbit-tilt-x, --vd-orbit-tilt-y, --vd-orbit-lift),
    // open, close and minimize emit particles, the minimize target follows the taskbar button, and the pointer sheen
    // variables follow the mouse. Windows are flat at rest; the bridge removes its attributes once a spring settles. It
    // never changes application state and never intercepts input. This file is the scaffold; the bridge arrives with the
    // orchestrator work.
    window.AuraOrbitWindows = Object.freeze({});
}());
