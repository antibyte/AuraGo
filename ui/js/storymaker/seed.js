// Storymaker player: seeded numbers. FNV-1a over UTF-16 units, mulberry32 and the rule that picks a sound take. Every seed is
// a string built from ids, so no compile input needs a project seed. Classic script; publishes StorymakerCore.seed.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    // hash(str): FNV-1a over UTF-16 units. Any value is turned into a string first; undefined hashes like 0.
    function hash(str) {
        var s = String(str === undefined ? 0 : str), h = 2166136261;
        for (var i = 0; i < s.length; i++) { h ^= s.charCodeAt(i); h = Math.imul(h, 16777619); }
        return h >>> 0;
    }

    // random(seedStr): mulberry32; the returned function gives values in [0, 1).
    function random(seedStr) {
        var a = hash(seedStr);
        return function () {
            a = (a + 0x6D2B79F5) | 0;
            var t = Math.imul(a ^ (a >>> 15), 1 | a);
            t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
            return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
        };
    }

    // takeIndex(sceneId, beatIndex, soundId, key, count, last): the take of one sound event; never last while count > 1.
    function takeIndex(sceneId, beatIndex, soundId, key, count, last) {
        var i = hash(sceneId + ':' + beatIndex + ':' + soundId + ':' + key) % count;
        if (count > 1 && i === last) i = (i + 1) % count;
        return i;
    }

    SM.seed = { hash: hash, random: random, takeIndex: takeIndex };
})(typeof globalThis !== 'undefined' ? globalThis : self);
