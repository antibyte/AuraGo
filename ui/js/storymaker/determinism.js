// Storymaker player: determinism. The player worker and the code-scene workers shadow Date (now and the zero-argument
// constructor), Math.random, performance.now, crypto.getRandomValues, setTimeout, setInterval and requestAnimationFrame:
// a scene that calls one gets an error that names api.random(seed) or the frame time instead of a different picture on
// every run. This is for determinism, not a security barrier (the CSP and the worker realm are the barriers).
// Classic script; publishes StorymakerCore.determinism.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    function blocked(name, hint) {
        return function () {
            throw new Error(name + ' is not available in a Storymaker scene (it would make the video differ between runs). ' + hint);
        };
    }

    function define(obj, key, value) {
        try {
            Object.defineProperty(obj, key, { value: value, writable: true, configurable: true, enumerable: false });
            return true;
        } catch (e) {
            try { obj[key] = value; return obj[key] === value; } catch (e2) { return false; }
        }
    }

    // install(g): g is the worker global. Returns the names that were shadowed.
    function install(g) {
        var done = [];
        function mark(name, ok) { if (ok) done.push(name); }
        var RealDate = g.Date;
        if (RealDate) {
            var ShadowDate = function Date() {
                if (arguments.length === 0) blocked('new Date()', 'Use the frame time t that api.onFrame passes, or api.tween.')();
                if (!new.target) return RealDate.apply(null, arguments);
                return Reflect.construct(RealDate, arguments, new.target === ShadowDate ? RealDate : new.target);
            };
            ShadowDate.prototype = RealDate.prototype;
            ShadowDate.UTC = RealDate.UTC;
            ShadowDate.parse = RealDate.parse;
            ShadowDate.now = blocked('Date.now()', 'Use the frame time t that api.onFrame passes, or api.tween.');
            mark('Date', define(g, 'Date', ShadowDate));
        }
        if (g.Math) mark('Math.random', define(g.Math, 'random', blocked('Math.random()', 'Use api.random(seed), which returns a seeded generator.')));
        if (g.performance) mark('performance.now', define(g.performance, 'now', blocked('performance.now()', 'Time comes from the frame number only.')));
        if (g.crypto) {
            mark('crypto.getRandomValues', define(g.crypto, 'getRandomValues', blocked('crypto.getRandomValues()', 'Use api.random(seed).')));
            if (typeof g.crypto.randomUUID === 'function') mark('crypto.randomUUID', define(g.crypto, 'randomUUID', blocked('crypto.randomUUID()', 'Use api.random(seed).')));
        }
        ['setTimeout', 'setInterval', 'requestAnimationFrame'].forEach(function (name) {
            if (typeof g[name] === 'function' || name in g) {
                mark(name, define(g, name, blocked(name + '()', 'Animation is built with api.tween and the beat times; there are no timers.')));
            }
        });
        return done;
    }

    SM.determinism = { install: install };
})(typeof globalThis !== 'undefined' ? globalThis : self);
