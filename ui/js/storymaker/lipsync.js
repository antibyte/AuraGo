// Storymaker player: lip-sync. track(block) turns the per-frame levels of a measured block into mouth shapes: rest below 85,
// E, A/I and O for low, mid and high levels, each with a seeded variation, and M/B/P at a short dip. The seed is the block
// id, so a block has one track and frame n stays a pure function of n. A block without levels has no track and the mouth
// rests. mouthAt reads a speech interval of an actor at a scene time. Classic script; publishes StorymakerCore.lipsync.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var LOW = 85, MID = 128, HIGH = 171;
    var VARIATION = 0.25;
    var DIP = 40;

    function shapeFor(level, r) {
        if (level < LOW) return 'rest';
        if (level < MID) return r < VARIATION ? 'fv' : 'e';
        if (level < HIGH) return r < VARIATION ? 'e' : 'ai';
        return r < VARIATION ? 'u' : 'o';
    }

    // track(block) = [mouth name per frame]. One draw per frame, so a dip never shifts the variation of later frames.
    function track(block) {
        var levels = block && block.levels, out = [];
        if (!levels || !levels.length) return out;
        var rnd = SM.seed.random(block.id);
        for (var i = 0; i < levels.length; i++) {
            var r = rnd();
            var dip = i > 0 && i < levels.length - 1 && levels[i - 1] - levels[i] >= DIP && levels[i + 1] - levels[i] >= DIP;
            out.push(dip ? 'mbp' : shapeFor(levels[i], r));
        }
        return out;
    }

    // mouthAt(interval, t): the mouth at scene time t inside a speech interval, whose data is {block}. The track is kept on the
    // block, because it never changes and a block can hold 27000 frames.
    function mouthAt(interval, t) {
        var block = interval.data && interval.data.block;
        if (!block) return 'rest';
        if (!block.mouths) block.mouths = track(block);
        var frames = block.mouths.length, span = block.duration_ms > 0 ? block.duration_ms / 1000 : interval.t1 - interval.t0;
        if (!frames || !(span > 0)) return 'rest';
        // a block holds ceil(duration * fps) frames, so this lands on frame k at t0 + k / fps with no fps at hand
        var k = Math.floor((t - interval.t0) / span * frames + 1e-9);
        return block.mouths[k < 0 ? 0 : (k >= frames ? frames - 1 : k)];
    }

    SM.lipsync = { track: track, mouthAt: mouthAt };
})(typeof globalThis !== 'undefined' ? globalThis : self);
