// Storymaker player: tween tracks. A Track is a numeric value over time built from segments (t0, t1, from, to, ease); a
// StepTrack is a discrete value that changes at given times. Tracks are filled while a story compiles and are evaluated
// by time only, so evaluation at time t is a pure function of the track. Classic script; publishes StorymakerCore.timeline.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    function lerp(a, b, u) { return a + (b - a) * u; }
    function clamp(v, lo, hi) { return v < lo ? lo : (v > hi ? hi : v); }

    function evalSeg(seg, t) {
        if (t >= seg.t1) return seg.to;
        if (!(seg.t1 > seg.t0)) return seg.to;
        var vendor = root.StorymakerVendor;
        if (vendor && vendor.gsap) {
            if (!seg.animation) {
                var a = seg.log ? Math.log(seg.from) : seg.from;
                var b = seg.log ? Math.log(seg.to) : seg.to;
                seg.value = { v: a };
                seg.animation = vendor.gsap.timeline({ paused: true }).fromTo(seg.value, { v: a }, {
                    v: b, duration: 1, ease: function (u) { return SM.ease(seg.ease, u); },
                    immediateRender: false, lazy: false
                }, 0);
                vendor.gsap.ticker.sleep();
            }
            seg.animation.totalTime(Math.max(0, (t - seg.t0) / (seg.t1 - seg.t0)), true);
            return seg.log ? Math.exp(seg.value.v) : seg.value.v;
        }
        var s = SM.ease(seg.ease, (t - seg.t0) / (seg.t1 - seg.t0));
        return seg.log ? seg.from * Math.pow(seg.to / seg.from, s) : lerp(seg.from, seg.to, s);
    }

    function Track(init) {
        this.init = init;
        this.segs = [];
        this.dirty = false;
    }

    // add(t0, t1, to, ease, from): from is optional; without it the segment starts at the value the track has at t0 (the
    // end of the previous segment, or its mid-way value when segments overlap).
    // log: interpolate in log space (camera zoom), so a zoom reads at an even speed.
    Track.prototype.add = function (t0, t1, to, ease, from, log) {
        this.segs.push({ t0: t0, t1: t1 < t0 ? t0 : t1, to: to, ease: ease || 'linear', from: from, log: !!log, order: this.segs.length });
        this.dirty = true;
        return this;
    };

    Track.prototype.finalize = function () {
        if (!this.dirty) return this;
        this.segs.sort(function (a, b) { return a.t0 - b.t0 || a.order - b.order; });
        for (var k = 0; k < this.segs.length; k++) {
            var seg = this.segs[k];
            if (seg.animation) { seg.animation.kill(); seg.animation = null; }
            if (seg.from === undefined || seg.from === null) {
                seg.from = k === 0 ? this.init : evalSeg(this.segs[k - 1], seg.t0);
            }
        }
        this.dirty = false;
        return this;
    };

    Track.prototype.at = function (t) {
        if (this.dirty) this.finalize();
        var segs = this.segs, lo = 0, hi = segs.length - 1, found = -1;
        while (lo <= hi) {
            var mid = (lo + hi) >> 1;
            if (segs[mid].t0 <= t) { found = mid; lo = mid + 1; } else { hi = mid - 1; }
        }
        return found < 0 ? this.init : evalSeg(segs[found], t);
    };

    Track.prototype.end = function () {
        var end = 0;
        for (var i = 0; i < this.segs.length; i++) if (this.segs[i].t1 > end) end = this.segs[i].t1;
        return end;
    };

    Track.prototype.dispose = function () {
        this.segs.forEach(function (seg) { if (seg.animation) { seg.animation.kill(); seg.animation = null; } });
    };

    function dispose(sceneOrTimeline) {
        if (!sceneOrTimeline) return;
        (sceneOrTimeline.scenes || [sceneOrTimeline]).forEach(function (scene) {
            (scene.entities || []).concat([scene.camera || {}]).forEach(function (entity) {
                Object.keys(entity).forEach(function (key) { if (entity[key] instanceof Track) entity[key].dispose(); });
            });
        });
    }

    function StepTrack(init) {
        this.init = init;
        this.steps = [];
        this.dirty = false;
    }

    // set(t, value, extra): extra is carried along (the pose track keeps its blend duration there).
    StepTrack.prototype.set = function (t, value, extra) {
        this.steps.push({ t: t, v: value, x: extra, order: this.steps.length });
        this.dirty = true;
        return this;
    };

    StepTrack.prototype.index = function (t) {
        if (this.dirty) {
            this.steps.sort(function (a, b) { return a.t - b.t || a.order - b.order; });
            this.dirty = false;
        }
        var steps = this.steps, lo = 0, hi = steps.length - 1, found = -1;
        while (lo <= hi) {
            var mid = (lo + hi) >> 1;
            if (steps[mid].t <= t) { found = mid; lo = mid + 1; } else { hi = mid - 1; }
        }
        return found;
    };

    StepTrack.prototype.at = function (t) {
        var k = this.index(t);
        return k < 0 ? this.init : this.steps[k].v;
    };

    // blendAt(t): for a track whose steps carry a blend duration in x: {a: previous value, b: value, u: 0..1}, u eased by ease-in-out.
    StepTrack.prototype.blendAt = function (t) {
        var k = this.index(t);
        if (k < 0) return { a: this.init, b: this.init, u: 1 };
        var step = this.steps[k], prev = k === 0 ? this.init : this.steps[k - 1].v;
        var dur = step.x > 0 ? step.x : 0;
        var u = dur > 0 ? clamp((t - step.t) / dur, 0, 1) : 1;
        return u >= 1 ? { a: step.v, b: step.v, u: 1 } : { a: prev, b: step.v, u: SM.ease('ease-in-out', u) };
    };

    // Interval list (walking, gesture windows): active(t) returns the interval containing t, the latest start first.
    function Intervals() { this.list = []; }
    Intervals.prototype.add = function (t0, t1, data) { this.list.push({ t0: t0, t1: t1, data: data }); return this; };
    Intervals.prototype.active = function (t) {
        var best = null;
        for (var i = 0; i < this.list.length; i++) {
            var it = this.list[i];
            if (t >= it.t0 && t < it.t1 && (!best || it.t0 >= best.t0)) best = it;
        }
        return best;
    };

    SM.timeline = { Track: Track, StepTrack: StepTrack, Intervals: Intervals, lerp: lerp, clamp: clamp, dispose: dispose };
})(typeof globalThis !== 'undefined' ? globalThis : self);
