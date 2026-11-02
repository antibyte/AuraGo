// Storymaker player: layout checks. Phase 1 has none: run() returns an empty list. The code scenes are listed as
// unverified (layout inside a code scene is never verified, only code_error, deadlines and the duration bounds are).
// The real checks (off_stage, overlap_face, text_overflow, ...) arrive in phase 3. Classic script; publishes
// StorymakerCore.checks.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    // run(scenes) = check results of the compiled scenes (phase 1: none).
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

    SM.checks = { run: run, unverified: unverified };
})(typeof globalThis !== 'undefined' ? globalThis : self);
