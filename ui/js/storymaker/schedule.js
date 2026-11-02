// Storymaker player: the frame schedule. It answers "which scenes are on screen at frame n" from the scene ranges alone
// (start_frame, end_frame, transition frames), so the document-realm host and the player worker agree without sharing the
// timeline. Scene boundaries are whole frames. Classic script; it is part of both bundles; it publishes StorymakerCore.schedule.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    // sched = {fps, frame_count, scenes:[{id, kind, start_frame, end_frame, tf, transition}]}
    // framePlan(sched, n) = {cur, layers:[{i, f}], transition: {kind, p} | null}. layers is drawn in order. f is the
    // scene-local frame (a scene held past its end during a transition shows its last frame). During the entry transition
    // of scene i the previous scene is the first layer and p runs over (0, 1).
    function framePlan(sched, n) {
        var scenes = sched.scenes || [];
        if (!scenes.length) return { cur: -1, layers: [], transition: null };
        var cur = 0;
        for (var i = 0; i < scenes.length; i++) {
            if (scenes[i].start_frame <= n) cur = i; else break;
        }
        var sc = scenes[cur], f = n - sc.start_frame;
        var layers = [{ i: cur, f: f < 0 ? 0 : f }], transition = null;
        if (cur > 0 && sc.tf > 0 && f < sc.tf && sc.transition && sc.transition !== 'cut') {
            var prev = scenes[cur - 1];
            var pf = n - prev.start_frame, last = prev.end_frame - prev.start_frame - 1;
            layers.unshift({ i: cur - 1, f: pf > last ? last : pf });
            transition = { kind: sc.transition, p: (f + 1) / (sc.tf + 1) };
        }
        return { cur: cur, layers: layers, transition: transition };
    }

    function frameToSeconds(fps, n) { return n / fps; }
    function secondsToFrames(fps, s) { return Math.round(s * fps + 1e-9); }

    SM.schedule = { framePlan: framePlan, frameToSeconds: frameToSeconds, secondsToFrames: secondsToFrames };
})(typeof globalThis !== 'undefined' ? globalThis : self);
