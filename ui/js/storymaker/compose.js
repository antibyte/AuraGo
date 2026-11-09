// Storymaker player: composition v2. A story with style.compose "v2" (the agent's projects) gets three rules on top of the
// stage: the base spots stand above the caption band (stage.js), the first cue-bound prop of a scene is enlarged to about a
// quarter of the frame height, and a scene without its own camera pushes in slowly. A story without compose keeps every
// position, size and camera as before. Classic script; publishes StorymakerCore.compose.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var HERO_HEIGHT = 0.24;                 // of the frame height: the height of a scene's hero prop
    var HERO_MIN = 0.5, HERO_MAX = 3;       // the scale that makes it so is clamped to this range
    var PUSH_IN = 1.08, PUSH_IN_CALM = 1.04;
    // Medium framing under captions: the speaker's feet stand at this share of the window height, above the caption box
    // (79.4 percent). The actor is about 0.72 of the window height at zoom 1.35, so the head ends up about 6 percent below the
    // top edge. Plain medium framing puts the feet at about 86 percent, inside the caption.
    var MEDIUM_FEET_ON_SCREEN = 0.78;

    // isV2(story): true for style.compose "v2".
    function isV2(story) { return !!(story && story.style && story.style.compose === 'v2'); }

    // heroScale(entry, depth): the scale that makes a prop of the catalog entry HERO_HEIGHT of the frame height at the given
    // depth scale: size.h * depth * scale = HERO_HEIGHT. An entry without size.h counts 0.15, as the renderers do.
    function heroScale(entry, depth) {
        var h = entry && entry.size && entry.size.h > 0 ? entry.size.h : 0.15;
        var d = depth > 0 ? depth : 1;
        var s = Math.round(HERO_HEIGHT / (h * d) * 1000) / 1000;
        return s < HERO_MIN ? HERO_MIN : (s > HERO_MAX ? HERO_MAX : s);
    }

    // hasOwnCamera(scene): the author chose a camera, so the automatic push-in stays off. ApplyDefaults writes framing wide
    // into every scene, so only a framing other than wide, a target or a camera beat counts.
    function hasOwnCamera(scene) {
        var cam = scene && scene.camera;
        if (cam && ((cam.framing && cam.framing !== 'wide') || cam.target)) return true;
        var beats = scene && scene.beats || [];
        for (var i = 0; i < beats.length; i++) if (beats[i] && beats[i].do === 'camera') return true;
        return false;
    }

    // mediumFocusY(feetY, zoom, visH): the camera centre (design y) that puts feetY at MEDIUM_FEET_ON_SCREEN of the window
    // height. The window is visH / zoom high and starts at centre - height / 2, so feetY = top + 0.78 * height.
    function mediumFocusY(feetY, zoom, visH) {
        var h = visH / zoom;
        return feetY - MEDIUM_FEET_ON_SCREEN * h + h / 2;
    }

    // pushInAmount(style): the zoom a scene reaches by its end; rhythm calm moves less.
    function pushInAmount(style) { return style && style.rhythm === 'calm' ? PUSH_IN_CALM : PUSH_IN; }

    SM.compose = {
        isV2: isV2, heroScale: heroScale, hasOwnCamera: hasOwnCamera, pushInAmount: pushInAmount, mediumFocusY: mediumFocusY,
        HERO_HEIGHT: HERO_HEIGHT, HERO_MIN: HERO_MIN, HERO_MAX: HERO_MAX, PUSH_IN: PUSH_IN, PUSH_IN_CALM: PUSH_IN_CALM,
        MEDIUM_FEET_ON_SCREEN: MEDIUM_FEET_ON_SCREEN
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
