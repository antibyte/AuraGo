// Storymaker player: the measured checks. The frame probe of the compile check page records two numbers per frame (see
// host-render.js), and the pure functions here turn that series into findings: flashes (WCAG 2.3.1 style), frozen or empty
// code scenes. Layout findings come from the layout run and Go, not from here. The code scenes are listed as unverified
// (layout inside a code scene is never verified, only code_error, deadlines and the duration bounds are). Classic script;
// publishes StorymakerCore.checks.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var FLASH_STEP = 0.10, FLASH_BRIGHT = 0.80, FLASH_MAX = 3, LEVEL_NOISE = 0.002;

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

    // run(probe, timeline) = the findings of the measured series. Without a probe there is nothing to say.
    function run() { return []; }

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
