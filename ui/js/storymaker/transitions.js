// Storymaker player: scene transitions. The previous and the current scene are drawn into two canvases and composited
// here: cut, fade, slide-left and wipe in phase 1 (the compile step already maps the other names to fade). p runs over
// (0, 1) and is the share of the current scene. Classic script; publishes StorymakerCore.transitions.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var IMPLEMENTED = ['cut', 'fade', 'slide-left', 'wipe'];

    // effective(kind) = the implemented transition that draws a kind (fade for the unknown ones).
    function effective(kind) { return IMPLEMENTED.indexOf(kind) >= 0 ? kind : 'fade'; }

    // composite(ctx, kind, p, w, h, prev, cur): prev and cur are canvases of the output size.
    function composite(ctx, kind, p, w, h, prev, cur) {
        ctx.setTransform(1, 0, 0, 1, 0, 0);
        ctx.globalAlpha = 1;
        switch (effective(kind)) {
            case 'cut':
                ctx.drawImage(cur, 0, 0);
                break;
            case 'slide-left': {
                var shift = Math.round(p * w);
                ctx.drawImage(prev, -shift, 0);
                ctx.drawImage(cur, w - shift, 0);
                break;
            }
            case 'wipe': {
                ctx.drawImage(prev, 0, 0);
                ctx.save();
                ctx.beginPath();
                ctx.rect(0, 0, Math.round(p * w), h);
                ctx.clip();
                ctx.drawImage(cur, 0, 0);
                ctx.restore();
                break;
            }
            default:
                ctx.drawImage(prev, 0, 0);
                ctx.globalAlpha = p < 0 ? 0 : (p > 1 ? 1 : p);
                ctx.drawImage(cur, 0, 0);
                ctx.globalAlpha = 1;
        }
    }

    SM.transitions = { IMPLEMENTED: IMPLEMENTED, effective: effective, composite: composite };
})(typeof globalThis !== 'undefined' ? globalThis : self);
