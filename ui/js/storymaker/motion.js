// Storymaker player: motion rules. Rhythm factors, default beat durations, lead fractions and text holds. Pure placement
// helpers live here so that beats.js stays under its line budget. Classic script; publishes StorymakerCore.motion.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var RHYTHM = { calm: 1.25, standard: 1, lively: 0.8 };
    // Seconds. wait is never scaled by the rhythm, every other default is.
    var DEFAULTS = { enter: 0.6, exit: 0.6, move: 0.8, pose: 0.3, emote: 0.25, look: 0.3, show: 0.5, hide: 0.35, camera: 0.8, shake: 0.5, wait: 1, highlight: 0.8 };
    // Fraction of the duration until the value first reaches 0.9, or for spring until it first crosses 1.
    var LEAD = {
        'linear': 0.9, 'ease-in': 0.9655, 'ease-out': 0.5358, 'ease-in-out': 0.7076, 'bounce-out': 0.3450,
        'quart-out': 0.4377, 'expo-out': 0.3322, 'spring': 0.4812, 'back-out': 0.2955
    };
    var OVERLAP = 0.4;
    var SECONDARY_HOLD = 1.2;

    // factor(style): the rhythm factor of a story style. No rhythm, or an unknown one, means 1.
    function factor(style) {
        var f = style ? RHYTHM[style.rhythm] : 0;
        return f > 0 ? f : 1;
    }

    // leadFraction(easeName): see LEAD. An unknown name counts as linear, as in SM.ease.
    function leadFraction(easeName) {
        var v = LEAD[easeName];
        return v > 0 ? v : LEAD.linear;
    }

    // holdSeconds(text, opts) = {base, scaled}. opts.secondary marks a caption or label in a scene that shows a title;
    // opts.factor is the rhythm factor. The scaled hold is never shorter than the base.
    function holdSeconds(text, opts) {
        opts = opts || {};
        var base = 0.5 + 0.3 * SM.ranges.wordCount(String(text));
        if (opts.secondary && base < SECONDARY_HOLD) base = SECONDARY_HOLD;
        return { base: base, scaled: base * Math.max(1, opts.factor > 0 ? opts.factor : 1) };
    }

    function isTextShow(b) { return b.text !== undefined && b.text !== null && b.item === undefined; }

    // defaultEase(b): the ease by role of a beat that sets no explicit ease. Walking (enter, exit, walk) is linear.
    function defaultEase(b) {
        if (b.do === 'move') return 'quart-out';
        if (b.do === 'hide') return 'ease-in';
        if (b.do === 'camera') return b.op === 'focus' ? 'expo-out' : 'ease-in-out';
        if (b.do === 'show') return b.entry === 'pop' && !isTextShow(b) ? 'spring' : 'quart-out';
        return 'linear';
    }

    // fadeEase(b): the ease of a show's opacity. Opacity never overshoots, and canvas ignores a globalAlpha above 1. So a
    // pop fades on quart-out whatever eases its scale, and an explicit back-out falls back to quart-out.
    function fadeEase(b) {
        if (b.entry === 'pop' && !isTextShow(b)) return 'quart-out';
        var ease = SM.isEase(b.ease) ? b.ease : 'quart-out';
        return ease === 'back-out' ? 'quart-out' : ease;
    }

    SM.motion = {
        RHYTHM: RHYTHM, DEFAULTS: DEFAULTS, OVERLAP: OVERLAP, factor: factor, leadFraction: leadFraction, holdSeconds: holdSeconds,
        defaultEase: defaultEase, fadeEase: fadeEase
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
