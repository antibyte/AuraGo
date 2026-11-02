// Storymaker player: the vector part format. A Shape {d, fill, stroke, stroke_width, opacity, gradient} becomes a Path2D
// with fills, strokes and gradients; colours may be hex or a $token that a resolver chain answers. An unresolved token
// draws #ff00ff and is recorded so tests (and the catalog build) can report it. Classic script; publishes StorymakerCore.parts.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var MISSING = '#ff00ff';
    var unresolved = {};
    var pathCache = {};
    var pathCount = 0;

    // makeResolver(maps): maps are plain objects, the first that has the token wins. Returns resolve(value).
    function makeResolver(maps) {
        var list = (maps || []).filter(function (m) { return m && typeof m === 'object'; });
        return function resolve(value) {
            if (typeof value !== 'string' || value.charAt(0) !== '$') return value;
            var name = value.slice(1);
            for (var i = 0; i < list.length; i++) {
                var v = list[i][name];
                if (typeof v === 'string' && v) return v.charAt(0) === '$' ? MISSING : v;
            }
            unresolved[name] = (unresolved[name] || 0) + 1;
            return MISSING;
        };
    }

    function unresolvedTokens() { return Object.keys(unresolved).sort(); }
    function resetUnresolved() { unresolved = {}; }

    function pathFor(d) {
        var p = pathCache[d];
        if (!p) {
            if (pathCount > 4000) { pathCache = {}; pathCount = 0; }
            p = new Path2D(d);
            pathCache[d] = p;
            pathCount++;
        }
        return p;
    }

    function makeGradient(ctx, g, resolve) {
        var grad;
        if (g.type === 'radial') {
            var cx = g.cx || 0, cy = g.cy || 0;
            grad = ctx.createRadialGradient(cx, cy, 0, cx, cy, g.r > 0 ? g.r : 1);
        } else {
            grad = ctx.createLinearGradient(g.x1 || 0, g.y1 || 0, g.x2 === undefined ? 1 : g.x2, g.y2 || 0);
        }
        var stops = g.stops || [];
        for (var i = 0; i < stops.length; i++) {
            var off = stops[i][0];
            grad.addColorStop(off < 0 ? 0 : (off > 1 ? 1 : off), resolve(stops[i][1]));
        }
        return grad;
    }

    // drawShape(ctx, shape, resolve, alpha): draws in the current transform. alpha multiplies the shape opacity.
    function drawShape(ctx, shape, resolve, alpha) {
        if (!shape || typeof shape.d !== 'string' || !shape.d) return;
        var a = (alpha === undefined ? 1 : alpha) * (typeof shape.opacity === 'number' ? shape.opacity : 1);
        if (!(a > 0)) return;
        var path = pathFor(shape.d);
        ctx.globalAlpha = a > 1 ? 1 : a;
        var hasStroke = shape.stroke && shape.stroke !== 'none';
        if (shape.gradient) {
            ctx.fillStyle = makeGradient(ctx, shape.gradient, resolve);
            ctx.fill(path);
        } else if (shape.fill !== 'none' && (shape.fill || !hasStroke)) {
            ctx.fillStyle = resolve(shape.fill || '#000000');
            ctx.fill(path);
        }
        if (hasStroke) {
            ctx.strokeStyle = resolve(shape.stroke);
            ctx.lineWidth = typeof shape.stroke_width === 'number' ? shape.stroke_width : 1;
            ctx.lineJoin = 'round';
            ctx.lineCap = 'round';
            ctx.stroke(path);
        }
    }

    function drawShapes(ctx, shapes, resolve, alpha) {
        if (!shapes) return;
        for (var i = 0; i < shapes.length; i++) drawShape(ctx, shapes[i], resolve, alpha);
    }

    // drawPart(ctx, part, resolve, alpha, fit): a prop file {viewbox:[x,y,w,h], shapes}. The caller has set the transform;
    // the part is drawn in its own viewbox coordinates.
    function drawPart(ctx, part, resolve, alpha) {
        if (part) drawShapes(ctx, part.shapes, resolve, alpha);
    }

    SM.parts = {
        makeResolver: makeResolver, unresolvedTokens: unresolvedTokens, resetUnresolved: resetUnresolved,
        drawShape: drawShape, drawShapes: drawShapes, drawPart: drawPart, MISSING: MISSING
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
