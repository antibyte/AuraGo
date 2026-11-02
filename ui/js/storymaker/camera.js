// Storymaker player: the scene camera. A camera is a zoom track plus a focus point (design pixels) and shake bursts.
// Framings wide/medium/close are zoom levels; the zoom, pan, focus and shake beats add tween segments. The stage turns the
// camera state into a window of the design frame. Classic script; publishes StorymakerCore.camera.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var FRAMING = { wide: 1, medium: 1.35, close: 1.9 };
    var MAX_ZOOM = 4;

    // create(zoom, cx, cy): cx/cy are the focus point in design pixels.
    function create(zoom, cx, cy) {
        var T = SM.timeline;
        return { zoom: new T.Track(zoom), cx: new T.Track(cx), cy: new T.Track(cy), shakes: [] };
    }

    function clampZoom(z) { return z < 1 ? 1 : (z > MAX_ZOOM ? MAX_ZOOM : z); }

    function addZoom(cam, t0, t1, zoom, ease) { cam.zoom.add(t0, t1, clampZoom(zoom), ease); }
    function addPan(cam, t0, t1, cx, cy, ease) { cam.cx.add(t0, t1, cx, ease); cam.cy.add(t0, t1, cy, ease); }
    function addShake(cam, t0, t1, amp) { cam.shakes.push({ t0: t0, t1: t1 > t0 ? t1 : t0 + 0.01, amp: amp }); }

    function shakeAt(cam, t) {
        var x = 0, y = 0;
        for (var i = 0; i < cam.shakes.length; i++) {
            var s = cam.shakes[i];
            if (t < s.t0 || t >= s.t1) continue;
            var decay = 1 - (t - s.t0) / (s.t1 - s.t0);
            x += s.amp * decay * (Math.sin(t * 61.7) * 0.6 + Math.sin(t * 37.3 + 1.3) * 0.4);
            y += s.amp * decay * (Math.sin(t * 53.1 + 0.7) * 0.6 + Math.sin(t * 29.9 + 2.1) * 0.4);
        }
        return { x: x, y: y };
    }

    // at(cam, t) = {zoom, cx, cy, shakeX, shakeY}
    function at(cam, t) {
        var sh = shakeAt(cam, t);
        return { zoom: cam.zoom.at(t), cx: cam.cx.at(t), cy: cam.cy.at(t), shakeX: sh.x, shakeY: sh.y };
    }

    function end(cam) {
        var e = Math.max(cam.zoom.end(), cam.cx.end(), cam.cy.end());
        for (var i = 0; i < cam.shakes.length; i++) if (cam.shakes[i].t1 > e) e = cam.shakes[i].t1;
        return e;
    }

    SM.camera = {
        FRAMING: FRAMING, MAX_ZOOM: MAX_ZOOM, create: create, clampZoom: clampZoom, addZoom: addZoom, addPan: addPan,
        addShake: addShake, at: at, end: end
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
