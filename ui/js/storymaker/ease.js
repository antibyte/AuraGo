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
        'bounce-out': bounceOut
    };

    // ease(name, t): t is clamped to 0..1; an unknown name is linear.
    function ease(name, t) {
        var fn = FNS[name] || FNS.linear;
        return fn(clamp01(t));
    }

    function isEase(name) { return Object.prototype.hasOwnProperty.call(FNS, name); }

    SM.ease = ease;
    SM.easeNames = NAMES.slice();
    SM.isEase = isEase;
    SM.clamp01 = clamp01;
})(typeof globalThis !== 'undefined' ? globalThis : self);
