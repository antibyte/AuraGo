// Storymaker player: the stage. Everything is laid out in the 1920x1080 design frame (positions are design pixels);
// the stage maps the design frame onto the output canvas for 16:9, 9:16 and 1:1 with a cover fit (the narrow aspects show
// a vertical slice centred on the environment's focus_x). It resolves spots (with the per-aspect overrides), {x, y}
// positions (percent of the visible safe area) and the depth scale from the environment's floor line. Classic script;
// publishes StorymakerCore.stage.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var DESIGN_W = 1920, DESIGN_H = 1080;
    var SIZES = { '16:9': [1920, 1080], '9:16': [1080, 1920], '1:1': [1080, 1080] };
    var DEFAULT_SAFE = { x: 5, y: 5, w: 90, h: 90 };
    var DEFAULT_FLOOR = { y_near: 96, y_far: 70, scale_near: 1.05, scale_far: 0.8 };
    var DEFAULT_SPOTS = {
        'far-left': { x: 12, y: 88 }, 'left': { x: 30, y: 88 }, 'center': { x: 50, y: 88 }, 'right': { x: 70, y: 88 },
        'far-right': { x: 88, y: 88 }, 'board': { x: 72, y: 60 }
    };
    var BASE_SPOTS = ['far-left', 'left', 'center', 'right', 'far-right'];
    var SLICE_MARGIN = 0.03;   // a spot closer than this fraction of the slice width to its edge counts as outside

    function clamp(v, lo, hi) { return v < lo ? lo : (v > hi ? hi : v); }
    function num(v, d) { return typeof v === 'number' && isFinite(v) ? v : d; }

    // sizeFor(aspect, width): the canvas size of an aspect; a width override keeps the aspect.
    function sizeFor(aspect, width) {
        var base = SIZES[aspect] || SIZES['16:9'];
        if (!(width > 0)) return { width: base[0], height: base[1] };
        return { width: Math.round(width), height: Math.round(width * base[1] / base[0]) };
    }

    // create({aspect, width, height, env, safe}): env is a catalog environment entry (spots, floor, aspects) or null.
    function create(opts) {
        opts = opts || {};
        var aspect = SIZES[opts.aspect] ? opts.aspect : '16:9';
        var size = sizeFor(aspect);
        var width = opts.width > 0 ? opts.width : size.width;
        var height = opts.height > 0 ? opts.height : (opts.width > 0 ? Math.round(opts.width * size.height / size.width) : size.height);
        var env = opts.env || null;
        var safe = opts.safe || DEFAULT_SAFE;
        var over = env && env.aspects && env.aspects[aspect] ? env.aspects[aspect] : null;

        // cover fit
        var scale = Math.max(width / DESIGN_W, height / DESIGN_H);
        var visW = Math.min(DESIGN_W, width / scale), visH = Math.min(DESIGN_H, height / scale);
        var focus = over && typeof over.focus_x === 'number' ? over.focus_x : 50;
        var visX = clamp(focus / 100 * DESIGN_W - visW / 2, 0, DESIGN_W - visW);
        var visY = (DESIGN_H - visH) / 2;
        var vis = { x: visX, y: visY, w: visW, h: visH };
        var cropped = visW < DESIGN_W - 0.5;
        var floor = env && env.floor ? env.floor : DEFAULT_FLOOR;

        var stage = {
            aspect: aspect, width: width, height: height, scale: width / visW, vis: vis, safe: safe, floor: floor,
            cropped: cropped, focusX: visX + visW / 2, env: env
        };

        // Percent of the design frame to design pixels.
        stage.pct = function (xPct, yPct) { return { x: xPct / 100 * DESIGN_W, y: yPct / 100 * DESIGN_H }; };

        // {x, y} in percent of the visible safe area to design pixels.
        stage.safePoint = function (xPct, yPct) {
            return {
                x: vis.x + (safe.x + clamp(xPct, 0, 100) / 100 * safe.w) / 100 * vis.w,
                y: vis.y + (safe.y + clamp(yPct, 0, 100) / 100 * safe.h) / 100 * vis.h
            };
        };

        stage.spotNames = function () {
            var names = {}, k;
            for (k in DEFAULT_SPOTS) names[k] = true;
            if (env && env.spots) for (k in env.spots) names[k] = true;
            return Object.keys(names);
        };

        stage.hasSpot = function (name) {
            return !!((env && env.spots && env.spots[name]) || DEFAULT_SPOTS[name]);
        };

        // spot(name) = {x, y, face, depth, name} in design pixels, or null for an unknown name. The per-aspect override wins;
        // without one the spot keeps its design position (furniture such as the desk is drawn there), unless a cropped aspect
        // would put it outside the visible slice (3 percent margin): then it is squeezed into the slice, so a catalog that
        // lists no overrides still keeps left, centre and right on screen.
        stage.spot = function (name) {
            var src = (env && env.spots && env.spots[name]) || DEFAULT_SPOTS[name];
            if (!src) return null;
            var o = over && over.spots && over.spots[name];
            var xPct = num(o && o.x, src.x), yPct = num(o && o.y, src.y), x = xPct / 100 * DESIGN_W;
            if (cropped && !(o && typeof o.x === 'number') && (x < vis.x + SLICE_MARGIN * vis.w || x > vis.x + (1 - SLICE_MARGIN) * vis.w)) {
                x = vis.x + xPct / 100 * vis.w;
            }
            var face = (o && o.face) || src.face || null;
            return { name: name, x: x, y: yPct / 100 * DESIGN_H, face: face, depth: typeof src.depth === 'number' ? src.depth : null };
        };

        // resolve(at, fallbackName): a spot name or {x, y} to {x, y, face, depth, spot}. Unknown values give null.
        stage.resolve = function (at) {
            if (typeof at === 'string') {
                var s = stage.spot(at);
                return s ? { x: s.x, y: s.y, face: s.face, depth: s.depth, spot: s.name } : null;
            }
            if (at && typeof at === 'object' && typeof at.x === 'number' && typeof at.y === 'number') {
                var p = stage.safePoint(at.x, at.y);
                return { x: p.x, y: p.y, face: null, depth: null, spot: null };
            }
            return null;
        };

        // Depth scale from the floor line: y_far -> scale_far, y_near -> scale_near, linear and clamped. depth (0 far .. 1 near)
        // overrides the y lookup for spots that carry one.
        stage.depthScale = function (yDesign, depth) {
            var f = floor, t;
            if (typeof depth === 'number') t = depth;
            else {
                var yPct = yDesign / DESIGN_H * 100, span = f.y_near - f.y_far;
                t = span === 0 ? 1 : (yPct - f.y_far) / span;
            }
            t = clamp(t, 0, 1);
            return f.scale_far + (f.scale_near - f.scale_far) * t;
        };

        // Off-stage x positions for enter and exit: clearly outside the visible slice.
        stage.offLeft = function () { return vis.x - 220; };
        stage.offRight = function () { return vis.x + vis.w + 220; };

        // cameraWindow(cam = {zoom, cx, cy, shakeX, shakeY}) = the rectangle of the design frame shown on the canvas.
        // The window keeps the canvas aspect and stays inside the design frame.
        stage.cameraWindow = function (cam) {
            var zoom = cam && cam.zoom > 0 ? cam.zoom : 1;
            var w = vis.w / zoom, h = vis.h / zoom;
            var cx = cam && typeof cam.cx === 'number' ? cam.cx : vis.x + vis.w / 2;
            var cy = cam && typeof cam.cy === 'number' ? cam.cy : vis.y + vis.h / 2;
            var x = clamp(cx - w / 2, 0, DESIGN_W - w) + (cam && cam.shakeX || 0);
            var y = clamp(cy - h / 2, 0, DESIGN_H - h) + (cam && cam.shakeY || 0);
            return { x: x, y: y, w: w, h: h, scale: width / w };
        };

        // The base window (no camera movement) and the matrix of a window: [a, b, c, d, e, f] for ctx.setTransform.
        stage.baseWindow = function () { return { x: vis.x, y: vis.y, w: vis.w, h: vis.h, scale: width / vis.w }; };
        stage.matrix = function (win) { return [win.scale, 0, 0, win.scale, -win.x * win.scale, -win.y * win.scale]; };

        // Canvas pixel of a design point under a window (used by screen-space overlays and tests).
        stage.toCanvas = function (px, py, win) {
            var w = win || stage.baseWindow();
            return { x: (px - w.x) * w.scale, y: (py - w.y) * w.scale };
        };

        stage.isBaseSpot = function (name) { return BASE_SPOTS.indexOf(name) >= 0; };
        return stage;
    }

    SM.stage = {
        create: create, sizeFor: sizeFor, DESIGN_W: DESIGN_W, DESIGN_H: DESIGN_H, SIZES: SIZES,
        DEFAULT_SAFE: DEFAULT_SAFE, DEFAULT_FLOOR: DEFAULT_FLOOR, DEFAULT_SPOTS: DEFAULT_SPOTS
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
