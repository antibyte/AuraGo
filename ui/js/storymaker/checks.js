// Storymaker player: the measured checks. The frame probe of the compile check page records two numbers per frame (see
// host-render.js), and the pure functions here turn that series into findings: flashes (WCAG 2.3.1 style), frozen or empty
// code scenes. Layout findings come from the layout run and Go, not from here. The code scenes are listed as unverified
// (layout inside a code scene is never verified, only code_error, deadlines and the duration bounds are). Classic script;
// publishes StorymakerCore.checks.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var FLASH_STEP = 0.10, FLASH_BRIGHT = 0.80, FLASH_MAX = 3, LEVEL_NOISE = 0.002, DEAD_DIFF = 0.01, DEAD_SECONDS = 3;

    // transitions(y) = the monotone runs of the luminance series whose total change is at least 0.10 and whose darker end is
    // below 0.80, as {from, to, dir} (dir 1 brightens). A run ends at its first extreme frame and the next one starts on the
    // last frame of the plateau; flat frames inside a run keep it.
    function transitions(y) {
        var runs = [], from = 0, to = 0, dir = 0, i;
        function close() {
            if (dir === 0) return;
            var change = Math.abs(y[to] - y[from]);
            if (change >= FLASH_STEP && Math.min(y[from], y[to]) < FLASH_BRIGHT) runs.push({ from: from, to: to, dir: dir });
        }
        for (i = 1; i < y.length; i++) {
            var dy = y[i] - y[i - 1], s = dy > LEVEL_NOISE ? 1 : dy < -LEVEL_NOISE ? -1 : 0;
            if (s === 0) continue;
            if (dir === 0) { dir = s; from = i - 1; to = i; continue; }
            if (s === dir) { to = i; continue; }
            close();
            from = i - 1; dir = s; to = i;
        }
        close();
        return runs;
    }

    // flashes(y) = the start frames of the flashes in a luminance series: two consecutive transitions in opposite
    // directions, paired from the front, so four transitions are two flashes.
    function flashes(y) {
        var runs = transitions(y || []), out = [];
        for (var i = 0; i + 1 < runs.length; i++) {
            if (runs[i].dir === runs[i + 1].dir) continue;
            out.push(runs[i].from);
            i++;
        }
        return out;
    }

    // flashWindows(starts, fps) = the start frame of every window of fps frames that holds more than 3 flash starts.
    function flashWindows(starts, fps) {
        var out = [];
        for (var i = 0; i + FLASH_MAX < starts.length; i++) {
            if (starts[i + FLASH_MAX] - starts[i] < fps) out.push(starts[i]);
        }
        return out;
    }

    // sceneAt(sched, frame) = the index of the scene that owns a film frame: the last scene that has started.
    function sceneAt(sched, frame) {
        var scenes = sched.scenes || [], at = 0;
        for (var i = 0; i < scenes.length; i++) if (scenes[i].start_frame <= frame) at = i; else break;
        return at;
    }

    // run(probe, timeline) = the findings of the measured series. Times in at_seconds are scene-local, like the lint's. The
    // probe is {fps, frame_count, y, d, uniform}: one y and one d per frame it reached, in order; without a probe there is
    // nothing to say.
    function run(probe, timeline) {
        var out = [];
        if (!probe || !timeline || !timeline.schedule) return out;
        var sched = timeline.schedule, scenes = sched.scenes || [], fps = probe.fps > 0 ? probe.fps : timeline.fps;
        var y = probe.y || [];
        function add(index, code, message, fix, frame) {
            out.push(SM.plan.issue('warning', scenes[index].id, '/scenes/' + index, code, message, { at_seconds: (frame - scenes[index].start_frame) / fps, fix: fix }));
        }

        var reported = {};
        flashWindows(flashes(y), fps).forEach(function (frame) {
            var index = sceneAt(sched, frame);
            if (reported[index]) return;
            reported[index] = true;
            add(index, 'flash', 'The picture flashes more than 3 times in one second here.',
                'Slow the change down: keep big brightness jumps at least a third of a second apart and avoid blinking full-screen colours. Rewrite this scene with patch_scene or put_scene, then check again.', frame);
        });

        var d = probe.d || [], reach = Math.min(y.length, d.length);
        scenes.forEach(function (sc, index) {
            if (sc.kind !== 'code') return;
            // dead_stretch: a run of frames equal to the one before them. The first frame of the scene differs from the last
            // frame of the scene before it, so the run is counted from the second.
            var run = 0, last = Math.min(sc.end_frame, reach);
            function stretch(end) {
                if (run / fps > DEAD_SECONDS) {
                    var secs = (run / fps).toFixed(1);
                    add(index, 'dead_stretch', 'The picture barely changes for ' + secs + ' s in this code scene.',
                        'Add motion in this stretch (a tween, a gesture, a new element) or make the scene shorter. Rewrite this scene with patch_scene or put_scene, then check again.', end - run);
                }
                run = 0;
            }
            for (var n = sc.start_frame + 1; n < last; n++) {
                if (d[n] < DEAD_DIFF) run++; else stretch(n);
            }
            stretch(last);
            var settled = SM.schedule.settledFrame(sched, index);
            if (probe.uniform && probe.uniform[index] === true && settled < reach) {
                add(index, 'first_frame_empty', 'The first frame after the transition is one flat colour.',
                    'Draw something at the very start of this code scene: a background, an actor or a text. Rewrite this scene with patch_scene or put_scene, then check again.', settled);
            }
        });

        if (y.length < sched.frame_count && scenes.length) {
            add(sceneAt(sched, y.length), 'flash_unverified', 'Flashing was not checked from here: the check ran out of time.',
                'Nothing to fix; a shorter film or fewer code scenes can be checked completely.', y.length);
        }
        return out;
    }

    // unverified(scenes) = one minor layout_unverified issue per code scene.
    function unverified(scenes) {
        var out = [];
        (scenes || []).forEach(function (s) {
            if (s.kind === 'code') {
                out.push(SM.plan.issue('minor', s.id, '/scenes/' + s.index, 'layout_unverified', 'Layout inside a code scene is not verified.'));
            }
        });
        return out;
    }

    SM.checks = { run: run, unverified: unverified, flashes: flashes, flashWindows: flashWindows };
})(typeof globalThis !== 'undefined' ? globalThis : self);
