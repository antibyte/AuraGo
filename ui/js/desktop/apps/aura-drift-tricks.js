/* Aura Drift - air state, tricks, landing, drift turbo, perfect start, slingshot, reverse.
   T1.1 placeholder: working no-op exports with the final signatures (section 1.5).
   T1.2 replaces this file with the real implementation. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;

    AD.TRICKS = {};
    AD.createTrickState = function () {
        return { chain: 0, chainPoints: 0, counts: {}, best: 0 };
    };
    AD.launch = function (race, actor, vz, source) { return false; };
    AD.stepAir = function (actor, input, dt, race) {
        /* Nothing launches a car yet; never leave one stranded in the air. */
        actor.v.airborne = false;
    };
    AD.landingQuality = function (v) { return 'good'; };
    AD.updateStart = function (actor, input, dt) {};
    AD.applyStart = function (race, actor) {};
    AD.updateVehicleTricks = function (race, actor, input, dt) {};
    AD.aiAirInput = function (race, car, input) { return input; };
})();
