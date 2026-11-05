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

    // isArrival(b): true for a beat that lands. Its lead counts against a cue, and sfx with prev reads its landing.
    function isArrival(b) {
        if (b.do === 'enter' || b.do === 'walk' || b.do === 'move') return true;
        if (b.do === 'show') return (b.entry || 'fade') !== 'none';
        return b.do === 'camera' && (b.op === 'focus' || (b.op === 'zoom' && !!b.target));
    }

    // landingEase(b): the ease the landing is read from. A fade reads its alpha, the others their moving track.
    function landingEase(b) {
        if (b.do === 'show' && b.entry !== 'slide' && !(b.entry === 'pop' && !isTextShow(b))) return fadeEase(b);
        return SM.isEase(b.ease) ? b.ease : defaultEase(b);
    }

    // leadOf(b, dur): the seconds from the start of an arrival to its landing, 0 for every other beat.
    function leadOf(b, dur) { return isArrival(b) ? leadFraction(landingEase(b)) * dur : 0; }

    SM.motion = {
        RHYTHM: RHYTHM, DEFAULTS: DEFAULTS, OVERLAP: OVERLAP, factor: factor, leadFraction: leadFraction, holdSeconds: holdSeconds,
        defaultEase: defaultEase, fadeEase: fadeEase, isArrival: isArrival, leadOf: leadOf
    };

    // pickTake(sceneId, j, entry, key, lastTake) = {take, impact in seconds}. It rotates through seed.takeIndex and
    // remembers the take in lastTake (soundId -> take), which spans the film. A plain sfx uses key 0.
    function pickTake(sceneId, j, entry, key, lastTake) {
        var take = SM.seed.takeIndex(sceneId, j, entry.id, key, entry.takes.length, lastTake.get(entry.id));
        lastTake.set(entry.id, take);
        return { take: take, impact: (entry.takes[take].impact_ms || 0) / 1000 };
    }

    var MAX_TYPING_KEYS = 32768; // the report's event ceiling; a story that never saw Go must not stall compile

    // typingKeys(o) = the key events of a pattern sfx (frozen 4.6 Typing). o = {sceneId, j, entry, first, start, dur, lastTake}.
    // first is the take of step 5. x starts at the beat's start plus that take's impact; each key starts at x minus the
    // impact of its own take; x advances by a seeded spacing while x < start+dur.
    function typingKeys(o) {
        var spacing = o.entry.pattern.spacing_ms, lo = spacing[0], span = spacing[1] - spacing[0];
        var prng = SM.seed.random(o.sceneId + ':' + o.j + ':' + o.entry.id);
        var pick = o.first, x = o.start + pick.impact, end = o.start + o.dur, key = 0, out = [];
        while (x < end && out.length < MAX_TYPING_KEYS) {
            out.push({ ref: o.entry.id, t: x - pick.impact, take: pick.take });
            x += (lo + span * prng()) / 1000;
            if (x >= end) break;
            key++;
            pick = pickTake(o.sceneId, o.j, o.entry, key, o.lastTake);
        }
        return out;
    }

    SM.motion.pickTake = pickTake;
    SM.motion.typingKeys = typingKeys;

    function hasCue(b) { return b.cue !== undefined && b.cue !== null; }

    // autoSfxEvents(o) = one sfx.pop event on the landing of each show with entry pop (frozen 4.6 Auto-SFX), unless an sfx
    // beat has the same cue or runs with:"prev" on it. o = {sceneId, log, beats, entry, lastTake}. Log rows are in beat order.
    function autoSfxEvents(o) {
        var cued = {}, out = [];
        o.log.forEach(function (row) {
            var b = o.beats[row.j];
            if (b.do === 'sfx' && hasCue(b)) cued[b.cue] = true;
        });
        o.log.forEach(function (row, k) {
            var b = o.beats[row.j], next = o.log[k + 1], after = next ? o.beats[next.j] : null;
            if (b.do !== 'show' || b.entry !== 'pop') return;
            if (hasCue(b) && cued[b.cue]) return;
            if (after && after.do === 'sfx' && after.with === 'prev' && !hasCue(after)) return;
            var pick = pickTake(o.sceneId, row.j, o.entry, 0, o.lastTake);
            out.push({ ref: o.entry.id, t: Math.max(0, row.landing - pick.impact), take: pick.take });
        });
        return out;
    }

    SM.motion.autoSfxEvents = autoSfxEvents;
})(typeof globalThis !== 'undefined' ? globalThis : self);
