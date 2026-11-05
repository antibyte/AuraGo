// Storymaker player: the code-scene API. A code scene is `export default function scene(api)` bundled into a classic
// script; it runs once, in its own worker, to build a timeline of tracks (like a GSAP master timeline), and every frame
// evaluates that timeline. API: duration, add, text, shape, tween, camera, random(seed), cue(n), onFrame(fn); chart and
// diagram exist and report not_supported_yet in phase 1. Handle methods take {start, dur, ease}; `start` is seconds
// (or api.cue(n)) from the scene start (the start of the entry transition, like cues and beats). Positions are spot
// names or {x, y} in percent of the safe area. Errors are thrown with a message that names the fix.
// Classic script; publishes StorymakerCore.codescene.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var MAX_DURATION = 600;
    var TWEEN_PROPS = { opacity: 'alpha', x: 'x', y: 'y', scale: 'scale', rotation: 'rot', width: 'w', height: 'h' };

    // random(seed): mulberry32, the only randomness a scene has. SM.seed owns the generator.
    function random(seed) { return SM.seed.random(seed); }

    function finite(v, what) {
        if (typeof v !== 'number' || !isFinite(v)) throw new TypeError(what + ' must be a number, got ' + String(v));
        return v;
    }

    function timing(o, defaultDur) {
        o = o || {};
        var start = o.start === undefined ? 0 : finite(o.start, 'start');
        var dur = o.dur === undefined ? defaultDur : finite(o.dur, 'dur');
        if (start < 0) throw new RangeError('start must be 0 or more, got ' + start);
        if (dur < 0) throw new RangeError('dur must be 0 or more, got ' + dur);
        var ease = o.ease === undefined ? 'ease-in-out' : o.ease;
        if (!SM.isEase(ease)) throw new RangeError('Unknown ease "' + ease + '". Use ' + SM.easeNames.join(', ') + '.');
        return { start: start, end: start + dur, dur: dur, ease: ease };
    }

    // build(sceneFn, opts): opts = {sceneId, catalog, aspect, width, height, fps, cues, palette, families, styleFont, safe,
    // language, fontChain}.
    function build(sceneFn, opts) {
        var cat = SM.scene.indexCatalog(opts.catalog), S = SM.scene, DW = SM.stage.DESIGN_W, DH = SM.stage.DESIGN_H;
        var stage = SM.stage.create({ aspect: opts.aspect, width: opts.width, height: opts.height, env: null, safe: opts.safe });
        var cam = SM.camera.create(1, stage.vis.x + stage.vis.w / 2, stage.vis.y + stage.vis.h / 2);
        var state = { duration: null, entities: [], onFrame: null, needs: {}, issues: [], counter: 0, unsupportedSeen: {} };
        var cues = opts.cues || {};
        var rigFile = opts.catalog && opts.catalog.rig && opts.catalog.rig.file || '';
        var glyphIdx = opts.catalog && opts.catalog.glyph_data ? SM.glyphs.index(opts.catalog.glyph_data) : null;
        if (glyphIdx) SM.glyphs.facesForLanguage(glyphIdx, opts.language).forEach(function (face) { state.needs[SM.glyphs.key(face)] = true; });

        function note(code, message) {
            var key = code + message;
            if (state.unsupportedSeen[key]) return;
            state.unsupportedSeen[key] = true;
            state.issues.push(SM.plan.issue('minor', opts.sceneId, '/scenes/' + (opts.index === undefined ? 0 : opts.index), code, message));
        }

        function place(at, what, fallback) {
            var pos = at === undefined || at === null ? stage.resolve(fallback || 'center') : stage.resolve(at);
            if (!pos) {
                throw new RangeError('Unknown position ' + JSON.stringify(at) + ' for ' + what + '. Use a spot (' + stage.spotNames().join(', ') + ') or {x, y} in percent.');
            }
            return pos;
        }

        function nameOf(map, name, what, def) {
            var n = name === undefined || name === null ? def : name;
            if (!map[n]) throw new RangeError('Unknown ' + what + ' "' + n + '". Available: ' + Object.keys(map).sort().join(', ') + '.');
            return n;
        }

        function addEntity(e) { state.entities.push(e); return e; }
        function nextId(prefix) { state.counter++; return prefix + '-' + state.counter; }

        // ---- handles
        function common(e) {
            var h = {};
            h.moveTo = function (position, o) {
                var t = timing(o, 0.6), pos = place(position, 'moveTo');
                e.x.add(t.start, t.end, pos.x, t.ease);
                e.y.add(t.start, t.end, pos.y, t.ease);
                return h;
            };
            h.fadeIn = function (o) {
                var t = timing(o, 0.5);
                if (!e.alpha.segs.length && e.alpha.init === 1) e.alpha.init = 0;
                e.alpha.add(t.start, t.end, 1, t.ease);
                return h;
            };
            h.fadeOut = function (o) {
                var t = timing(o, 0.5);
                e.alpha.add(t.start, t.end, 0, t.ease);
                return h;
            };
            h.scaleTo = function (s, o) {
                var t = timing(o, 0.5);
                e.scale.add(t.start, t.end, finite(s, 'scale') / (e.baseScale || 1), t.ease);
                return h;
            };
            h.rotateTo = function (deg, o) {
                var t = timing(o, 0.5);
                e.rot.add(t.start, t.end, finite(deg, 'degrees'), t.ease);
                return h;
            };
            Object.defineProperty(h, '__entity', { value: e, enumerable: false });
            return h;
        }

        function actorHandle(e) {
            var h = common(e);
            h.walkTo = function (position, o) {
                var pos = place(position, 'walkTo'), start = o && o.start !== undefined ? finite(o.start, 'start') : 0;
                var from = { x: e.x.at(start), y: e.y.at(start) }, cyc = cat.cycles.walk, speed = cyc && cyc.speed > 0 ? cyc.speed : 12;
                var dist = Math.sqrt(Math.pow(pos.x - from.x, 2) + Math.pow(pos.y - from.y, 2));
                var dur = o && o.dur !== undefined ? finite(o.dur, 'dur') : Math.max(0.3, dist / (speed / 100 * DW * stage.depthScale(pos.y)));
                var t = timing({ start: start, dur: dur, ease: o && o.ease || 'linear' }, dur);
                if (Math.abs(pos.x - from.x) > 1) e.facing.set(t.start, pos.x > from.x ? 1 : -1);
                e.x.add(t.start, t.end, pos.x, t.ease);
                e.y.add(t.start, t.end, pos.y, t.ease);
                e.walks.add(t.start, t.end, {});
                return h;
            };
            h.pose = function (name, o) {
                var t = timing(o, 0.3);
                e.pose.set(t.start, nameOf(cat.poses, name, 'pose'), t.dur);
                return h;
            };
            h.emote = function (name, o) {
                var t = timing(o, 0.25);
                e.expr.set(t.start, nameOf(cat.expressions, name, 'expression'));
                return h;
            };
            h.gesture = function (name, o) {
                var entry = cat.gestures[nameOf(cat.gestures, name, 'gesture')];
                var count = Math.max(1, Math.min(5, Math.round(o && o.count || 1)));
                var t = timing({ start: o && o.start, dur: o && o.dur !== undefined ? o.dur : (entry.duration || 0.5) * count, ease: o && o.ease }, 0.5);
                e.gestures.add(t.start, t.end, { name: name, count: count });
                return h;
            };
            h.look = function (target, o) {
                var t = timing(o, 0.3);
                if (target === 'camera') e.look.set(t.start, 'camera');
                else { e.look.set(t.start, null); var p = place(target, 'look'); if (Math.abs(p.x - e.x.at(t.start)) > 1) e.facing.set(t.start, p.x > e.x.at(t.start) ? 1 : -1); }
                return h;
            };
            return h;
        }

        // ---- the API
        var api = {};
        api.duration = function (sec) {
            finite(sec, 'duration');
            if (sec <= 0 || sec > MAX_DURATION) throw new RangeError('duration must be between 0 and ' + MAX_DURATION + ' seconds, got ' + sec);
            state.duration = sec;
        };
        api.add = function (id, o) {
            o = o || {};
            var entry = cat.byId[id];
            if (!entry) throw new RangeError('Unknown catalog id "' + id + '". Search the catalog for an exact id (char.*, prop.*).');
            var pos = place(o.at, 'add');
            if (entry.kind === 'character') {
                var pose = nameOf(cat.poses, o.pose, 'pose', 'stand');
                if (rigFile) state.needs['catalog:' + rigFile] = true;
                var facing = o.facing === 'left' ? -1 : (o.facing === 'right' ? 1 : (pos.face === 'left' ? -1 : (pos.face === 'right' ? 1 : (pos.x <= stage.focusX ? 1 : -1))));
                var a = S.createActor({ id: nextId('actor'), cast: id, character: id, params: entry.params || {}, x: pos.x, y: pos.y, scale: o.scale, facing: facing, pose: pose, expression: 'neutral', phase: state.counter * 0.37 });
                return actorHandle(addEntity(a));
            }
            if (entry.kind === 'prop') {
                if (entry.file) state.needs['catalog:' + entry.file] = true;
                return common(addEntity(S.createProp({ id: nextId('prop'), item: id, x: pos.x, y: pos.y, scale: o.scale, layer: o.layer })));
            }
            throw new RangeError('api.add takes a character or a prop, "' + id + '" is a ' + entry.kind + '.');
        };
        api.text = function (str, style, o) {
            var name = style === undefined ? 'title' : style;
            nameOf(cat.textStyles, name, 'text style');
            var at = o && o.at !== undefined ? o.at : 'center', pos;
            if (typeof at === 'string' && stage.isBaseSpot(at)) pos = { x: stage.spot(at).x, y: 0.28 * DH };
            else pos = place(at, 'text');
            var se = cat.textStyles[name], font = !se.font || se.font === 'default' ? opts.styleFont || 'default' : se.font;
            (cat.fonts || []).forEach(function (f) { if (f.name === font) state.needs['font:' + f.name + '/' + f.weight] = true; });
            var e = addEntity(S.createText({ id: nextId('text'), text: String(str), style: name, x: pos.x, y: pos.y }));
            var h = common(e);
            h.setText = function (s) { e.text = String(s); return h; };
            return h;
        };
        api.shape = function (kind, o) {
            o = o || {};
            if (kind !== 'rect' && kind !== 'circle' && kind !== 'line') throw new RangeError('Unknown shape "' + kind + '". Use rect, circle or line.');
            var pos = place(o.at, 'shape');
            return common(addEntity(S.createShape({ id: nextId('shape'), kind: kind, x: pos.x, y: pos.y, w: o.w, h: o.h, color: o.fill || o.color, stroke: o.stroke })));
        };
        api.chart = function () { note('not_supported_yet', 'api.chart is not supported yet; the chart is not drawn.'); return common(S.createShape({ id: nextId('chart'), kind: 'rect', x: 0, y: 0, hidden: true })); };
        api.diagram = function () { note('not_supported_yet', 'api.diagram is not supported yet; the diagram is not drawn.'); return common(S.createShape({ id: nextId('diagram'), kind: 'rect', x: 0, y: 0, hidden: true })); };
        api.tween = function (target, props, o) {
            var e = target && target.__entity;
            if (!e) throw new TypeError('api.tween needs a handle returned by api.add, api.text or api.shape.');
            if (!props || typeof props !== 'object') throw new TypeError('api.tween needs an object of properties, for example {opacity: 1}.');
            var t = timing(o, 0.5), from = (o && o.from) || {};
            Object.keys(props).forEach(function (key) {
                var field = TWEEN_PROPS[key];
                if (!field) {
                    if (key === 'grow') { note('not_supported_yet', 'The tween property "grow" belongs to charts, which are not supported yet.'); return; }
                    throw new RangeError('Unknown tween property "' + key + '". Use ' + Object.keys(TWEEN_PROPS).join(', ') + '.');
                }
                var to = finite(props[key], key), track = e[field];
                if (!track) throw new RangeError('The property "' + key + '" does not apply to this handle.');
                if (key === 'x') to = to / 100 * DW;
                else if (key === 'y') to = to / 100 * DH;
                else if (key === 'scale') to = to / (e.baseScale || 1);
                var f = from[key] === undefined ? undefined : finite(from[key], 'from.' + key) * (key === 'x' ? DW / 100 : (key === 'y' ? DH / 100 : (key === 'scale' ? 1 / (e.baseScale || 1) : 1)));
                if (key === 'opacity' && f !== undefined && !track.segs.length) track.init = f;
                track.add(t.start, t.end, to, t.ease, f);
            });
            return target;
        };
        api.camera = {
            zoom: function (amount, o) { var t = timing(o, 0.8); SM.camera.addZoom(cam, t.start, t.end, finite(amount, 'zoom'), t.ease); },
            pan: function (position, o) { var t = timing(o, 0.8), p = place(position, 'camera.pan'); SM.camera.addPan(cam, t.start, t.end, p.x, p.y - 150, t.ease); },
            shake: function (amount, o) { var t = timing(o, 0.5); SM.camera.addShake(cam, t.start, t.end, amount > 0 ? amount : 14); }
        };
        api.random = random;
        api.cue = function (n) {
            if (cues[n] === undefined) throw new RangeError('The narration has no cue marker {' + n + '}.');
            return cues[n];
        };
        api.onFrame = function (fn) {
            if (typeof fn !== 'function') throw new TypeError('api.onFrame needs a function (t, g).');
            state.onFrame = fn;
        };

        sceneFn(api);

        var motionEnd = SM.camera.end(cam);
        state.entities.forEach(function (e) { var end = S.entityEnd(e); if (end > motionEnd) motionEnd = end; });
        var duration = state.duration !== null ? state.duration : Math.min(MAX_DURATION, Math.max(1, motionEnd + 0.5));
        return { duration: duration, entities: state.entities, camera: cam, onFrame: state.onFrame, needs: Object.keys(state.needs).sort(), issues: state.issues, stage: stage };
    }

    // ---- the restricted 2D facade passed to api.onFrame(fn)
    function facade(ctx, rc) {
        var resolve = SM.parts.makeResolver([rc.palette]);
        function num(v, what) { if (typeof v !== 'number' || !isFinite(v)) throw new TypeError(what + ' must be a finite number'); return v; }
        var g = {
            save: function () { ctx.save(); },
            restore: function () { ctx.restore(); },
            translate: function (x, y) { ctx.translate(num(x, 'x'), num(y, 'y')); },
            rotate: function (deg) { ctx.rotate(num(deg, 'degrees') * Math.PI / 180); },
            scale: function (x, y) { ctx.scale(num(x, 'x'), num(y === undefined ? x : y, 'y')); },
            alpha: function (a) { ctx.globalAlpha = Math.max(0, Math.min(1, num(a, 'alpha'))); },
            fill: function (color) { ctx.fillStyle = resolve(String(color)); },
            stroke: function (color, width) { ctx.strokeStyle = resolve(String(color)); if (width !== undefined) ctx.lineWidth = Math.max(0, num(width, 'width')); },
            rect: function (x, y, w, h) { ctx.fillRect(num(x, 'x'), num(y, 'y'), num(w, 'w'), num(h, 'h')); },
            strokeRect: function (x, y, w, h) { ctx.strokeRect(num(x, 'x'), num(y, 'y'), num(w, 'w'), num(h, 'h')); },
            circle: function (x, y, r) { ctx.beginPath(); ctx.arc(num(x, 'x'), num(y, 'y'), Math.max(0, num(r, 'r')), 0, Math.PI * 2); ctx.fill(); },
            line: function (x1, y1, x2, y2) { ctx.beginPath(); ctx.moveTo(num(x1, 'x1'), num(y1, 'y1')); ctx.lineTo(num(x2, 'x2'), num(y2, 'y2')); ctx.stroke(); },
            path: function (d, mode) {
                var p = new Path2D(String(d));
                if (mode !== 'stroke') ctx.fill(p);
                if (mode === 'stroke' || mode === 'both') ctx.stroke(p);
            },
            text: function (str, x, y, o) {
                o = o || {};
                var px = o.size > 0 ? o.size : 48, family = SM.render.familyOf(rc, { font: 'default' });
                ctx.font = SM.text.fontString(o.weight || 700, px, family);
                ctx.textAlign = o.align === 'left' || o.align === 'right' ? o.align : 'center';
                ctx.textBaseline = 'middle';
                if (o.color) ctx.fillStyle = resolve(String(o.color));
                // Set per call, so the direction never depends on what an earlier draw left on the canvas.
                ctx.direction = SM.ranges.dirOf(String(str));
                ctx.fillText(String(str), num(x, 'x'), num(y, 'y'));
            }
        };
        return Object.freeze(g);
    }

    // drawFrame(built, t, ctx, rc): draws the scene timeline at time t, then the onFrame hook in design coordinates.
    function drawFrame(built, t, ctx, rc) {
        var sceneLike = { id: rc.sceneId || 'code', env: null, variant: 'day', camera: built.camera, entities: built.entities, stages: built.stages || (built.stages = {}) };
        SM.render.drawScene(ctx, sceneLike, t, rc);
        if (!built.onFrame) return;
        var stage = SM.render.stageFor(sceneLike, rc), win = stage.cameraWindow(SM.camera.at(built.camera, t)), m = stage.matrix(win);
        ctx.save();
        ctx.setTransform(m[0], m[1], m[2], m[3], m[4], m[5]);
        ctx.globalAlpha = 1;
        try { built.onFrame(t, facade(ctx, rc)); } finally { ctx.restore(); }
    }

    SM.codescene = { build: build, drawFrame: drawFrame, random: random, MAX_DURATION: MAX_DURATION };
})(typeof globalThis !== 'undefined' ? globalThis : self);
