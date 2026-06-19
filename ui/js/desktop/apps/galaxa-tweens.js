(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    // Easing functions: t is normalized 0..1 progress
    const Easing = {
        linear: function (t) { return t; },
        easeInQuad: function (t) { return t * t; },
        easeOutQuad: function (t) { return t * (2 - t); },
        easeInOutQuad: function (t) { return t < 0.5 ? 2 * t * t : -1 + (4 - 2 * t) * t; },
        easeInCubic: function (t) { return t * t * t; },
        easeOutCubic: function (t) { const f = t - 1; return f * f * f + 1; },
        easeInOutCubic: function (t) { return t < 0.5 ? 4 * t * t * t : (t - 1) * (2 * t - 2) * (2 * t - 2) + 1; },
        easeInQuart: function (t) { return t * t * t * t; },
        easeOutQuart: function (t) { const f = t - 1; return f * f * f * (1 - f) + 1; },
        easeOutBack: function (t) { const c1 = 1.70158, c3 = c1 + 1; return 1 + c3 * Math.pow(t - 1, 3) + c1 * Math.pow(t - 1, 2); },
        easeInBack: function (t) { const c1 = 1.70158, c3 = c1 + 1; return c3 * t * t * t - c1 * t * t; },
        easeOutElastic: function (t) {
            if (t === 0 || t === 1) return t;
            const c4 = (2 * Math.PI) / 3;
            return Math.pow(2, -10 * t) * Math.sin((t * 10 - 0.75) * c4) + 1;
        },
        easeOutBounce: function (t) {
            const n1 = 7.5625, d1 = 2.75;
            if (t < 1 / d1) return n1 * t * t;
            if (t < 2 / d1) { t -= 1.5 / d1; return n1 * t * t + 0.75; }
            if (t < 2.5 / d1) { t -= 2.25 / d1; return n1 * t * t + 0.9375; }
            t -= 2.625 / d1; return n1 * t * t + 0.984375;
        },
        easeInOutSine: function (t) { return -(Math.cos(Math.PI * t) - 1) / 2; }
    };

    // Lightweight tween manager: drives a pool of active tweens, updated per frame.
    // Each tween animates a single numeric property on a target object.
    GC.createTweens = function (ctx) {
        const active = [];

        function tween(target, prop, to, durMs, ease) {
            const from = target[prop];
            const easeFn = ease || Easing.easeOutCubic;
            const tw = { target, prop, from, to, dur: Math.max(1, durMs), t: 0, easeFn, done: false, onUpdate: null, onComplete: null };
            active.push(tw);
            return tw;
        }

        function tweenVec(target, prop, toVal, durMs, ease) {
            // Animate x/y sub-properties of an object (e.g. {x,y})
            const easeFn = ease || Easing.easeOutCubic;
            const fromX = target[prop].x, fromY = target[prop].y;
            const tw = { target, prop, fromX, fromY, toX: toVal.x, toY: toVal.y, dur: Math.max(1, durMs), t: 0, easeFn, vec: true, done: false, onUpdate: null, onComplete: null };
            active.push(tw);
            return tw;
        }

        function update(dtMs) {
            let w = 0;
            for (let i = 0; i < active.length; i++) {
                const tw = active[i];
                tw.t += dtMs;
                const p = Math.min(1, tw.t / tw.dur);
                const e = tw.easeFn(p);
                if (tw.vec) {
                    tw.target[tw.prop].x = tw.fromX + (tw.toX - tw.fromX) * e;
                    tw.target[tw.prop].y = tw.fromY + (tw.toY - tw.fromY) * e;
                } else {
                    tw.target[tw.prop] = tw.from + (tw.to - tw.from) * e;
                }
                if (tw.onUpdate) tw.onUpdate(e);
                if (p >= 1) { tw.done = true; if (tw.onComplete) tw.onComplete(); }
                else active[w++] = tw;
            }
            active.length = w;
        }

        function clear() { active.length = 0; }
        function count() { return active.length; }

        // Convenience: get eased progress for a raw time value (stateless helper)
        function apply(easeName, t) { const fn = Easing[easeName] || Easing.easeOutCubic; return fn(t); }

        ctx.Easing = Easing;
        ctx.tween = tween;
        ctx.tweenVec = tweenVec;
        ctx.updateTweens = update;
        ctx.clearTweens = clear;
        ctx.tweenCount = count;
        ctx.tweenApply = apply;
    };
})();