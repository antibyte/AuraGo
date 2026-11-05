// Storymaker player: the six easing functions of the story schema (linear, ease-in, ease-out, ease-in-out, back-out,
// bounce-out). Pure and deterministic. Classic script, its own IIFE scope; it publishes StorymakerCore.ease.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var NAMES = ['linear', 'ease-in', 'ease-out', 'ease-in-out', 'back-out', 'bounce-out'];

    function clamp01(t) {
        if (!(t > 0)) return 0;
        return t > 1 ? 1 : t;
    }

    function bounceOut(t) {
        var n1 = 7.5625, d1 = 2.75;
        if (t < 1 / d1) return n1 * t * t;
        if (t < 2 / d1) { t -= 1.5 / d1; return n1 * t * t + 0.75; }
        if (t < 2.5 / d1) { t -= 2.25 / d1; return n1 * t * t + 0.9375; }
        t -= 2.625 / d1;
        return n1 * t * t + 0.984375;
    }

    // spring(u): damped oscillator, damping 0.75, w0 7.6. It overshoots 1 once and ends on exactly 1.
    var SPRING_ZETA = 0.75, SPRING_W0 = 7.6, SPRING_WD = SPRING_W0 * Math.sqrt(1 - SPRING_ZETA * SPRING_ZETA);
    function spring(u) {
        if (u >= 1) return 1;
        var a = SPRING_ZETA * SPRING_W0;
        return 1 - Math.exp(-a * u) * (Math.cos(SPRING_WD * u) + (a / SPRING_WD) * Math.sin(SPRING_WD * u));
    }

    var FNS = {
        'linear': function (t) { return t; },
        'ease-in': function (t) { return t * t * t; },
        'ease-out': function (t) { var u = 1 - t; return 1 - u * u * u; },
        'ease-in-out': function (t) {
            if (t < 0.5) return 4 * t * t * t;
            var u = -2 * t + 2;
            return 1 - (u * u * u) / 2;
        },
        'back-out': function (t) {
            var c1 = 1.70158, c3 = c1 + 1, u = t - 1;
            return 1 + c3 * u * u * u + c1 * u * u;
        },
        'bounce-out': bounceOut,
        // Internal curves: compile picks them by role, and a story or a code scene cannot name them (isEase below).
        'quart-out': function (t) { var u = 1 - t; return 1 - u * u * u * u; },
        'expo-out': function (t) { return t >= 1 ? 1 : 1 - Math.pow(2, -10 * t); },
        'spring': spring
    };

    // ease(name, t): t is clamped to 0..1; an unknown name is linear.
    function ease(name, t) {
        var fn = FNS[name] || FNS.linear;
        return fn(clamp01(t));
    }

    // isEase(name): only the six public names.
    function isEase(name) { return NAMES.indexOf(name) >= 0; }

    SM.ease = ease;
    SM.easeNames = NAMES.slice();
    SM.isEase = isEase;
    SM.clamp01 = clamp01;
})(typeof globalThis !== 'undefined' ? globalThis : self);
