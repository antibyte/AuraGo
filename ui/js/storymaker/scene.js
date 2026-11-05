// Storymaker player: scene entities. Actors, props, texts and shapes are bundles of tracks (position, alpha, scale,
// rotation, pose, expression, facing, gestures, walking). The same entities serve storyboard scenes (filled by compile)
// and code scenes (filled by the code-scene API). evalEntity(entity, t) is a pure function of the tracks and t. This file
// also indexes the catalog manifest by kind and bare name. Classic script; publishes StorymakerCore.scene.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});
    var T = function () { return SM.timeline; };

    // indexCatalog(catalog): the manifest by id and by kind + bare name. The result is plain data.
    function indexCatalog(catalog) {
        var idx = {
            version: (catalog && catalog.version) || '', byId: {}, poses: {}, gestures: {}, expressions: {}, cycles: {},
            textStyles: {}, transitions: {}, mouths: {}, palettes: {}, fonts: (catalog && catalog.fonts) || [],
            stage: (catalog && catalog.stage) || null, rig: (catalog && catalog.rig) || null
        };
        var entries = (catalog && catalog.entries) || [];
        for (var i = 0; i < entries.length; i++) {
            var e = entries[i];
            if (!e || !e.id) continue;
            idx.byId[e.id] = e;
            var bare = e.name && e.name !== e.id ? e.name : String(e.id).replace(/^[a-z]+\./, '');
            if (e.kind === 'pose') idx.poses[bare] = e;
            else if (e.kind === 'gesture') idx.gestures[bare] = e;
            else if (e.kind === 'expression') idx.expressions[bare] = e;
            else if (e.kind === 'cycle') idx.cycles[bare] = e;
            else if (e.kind === 'text_style') idx.textStyles[bare] = e;
            else if (e.kind === 'transition') idx.transitions[bare] = e;
            else if (e.kind === 'mouth') idx.mouths[bare] = e;
        }
        var pals = (catalog && catalog.palettes) || [];
        for (var p = 0; p < pals.length; p++) idx.palettes[pals[p].name || String(pals[p].id).replace(/^palette\./, '')] = pals[p];
        return idx;
    }

    function base(type, id, x, y) {
        var t = T();
        return {
            type: type, id: id, x: new t.Track(x), y: new t.Track(y), alpha: new t.Track(1), scale: new t.Track(1),
            rot: new t.Track(0), depth: null
        };
    }

    // createActor({id, cast, character, params, x, y, scale, facing, pose, expression, hidden, phase})
    function createActor(def) {
        var t = T(), e = base('actor', def.id, def.x, def.y);
        e.cast = def.cast;
        e.character = def.character;
        e.params = def.params || {};
        e.baseScale = def.scale > 0 ? def.scale : 1;
        e.depth = typeof def.depth === 'number' ? def.depth : null;
        e.facing = new t.StepTrack(def.facing === -1 ? -1 : 1);
        e.look = new t.StepTrack(null);
        e.pose = new t.StepTrack(def.pose || 'stand');
        e.expr = new t.StepTrack(def.expression || 'neutral');
        e.gestures = new t.Intervals();
        e.walks = new t.Intervals();
        e.speech = new t.Intervals();
        e.highlights = new t.Intervals();
        e.phase = def.phase || 0;
        if (def.hidden) e.alpha.init = 0;
        return e;
    }

    // createProp({id, item, x, y, scale, layer, hidden})
    function createProp(def) {
        var e = base('prop', def.id, def.x, def.y);
        e.item = def.item;
        e.layer = def.layer === 'back' ? 'back' : 'front';
        e.highlights = new (T().Intervals)();
        e.baseScale = def.scale > 0 ? def.scale : 1;
        if (def.hidden) e.alpha.init = 0;
        return e;
    }

    // createText({id, text, style, x, y, hidden}); x and y are design pixels, text is drawn in screen space.
    function createText(def) {
        var e = base('text', def.id, def.x, def.y);
        e.text = String(def.text === undefined || def.text === null ? '' : def.text);
        e.style = def.style || 'title';
        e.highlights = new (T().Intervals)();
        e.baseScale = 1;
        if (def.hidden) e.alpha.init = 0;
        return e;
    }

    // createShape({id, kind, x, y, w, h, color, stroke, hidden}); w/h are design pixels (circle: w is the radius).
    function createShape(def) {
        var e = base('shape', def.id, def.x, def.y);
        e.kind = def.kind || 'rect';
        e.w = new (T().Track)(def.w > 0 ? def.w : 100);
        e.h = new (T().Track)(def.h > 0 ? def.h : 100);
        e.color = def.color || '$accent';
        e.stroke = def.stroke || null;
        e.baseScale = 1;
        if (def.hidden) e.alpha.init = 0;
        return e;
    }

    // createBubble({id, owner, kind, lines, t0, t1}): a bubble of the actor whose id is owner, shown from t0 to t1 with the fades
    // of SM.bubble. kind is speech or thought and also names the catalog text style. It is drawn in screen space above the texts.
    function createBubble(def) {
        var e = base('bubble', def.id, 0, 0), fade = Math.min(SM.bubble.FADE_SECONDS, Math.max(0, def.t1 - def.t0) / 2);
        e.owner = def.owner;
        e.kind = def.kind === 'thought' ? 'thought' : 'speech';
        e.style = e.kind;
        e.lines = def.lines || [];
        e.baseScale = 1;
        e.alpha.init = 0;
        e.alpha.add(def.t0, def.t0 + fade, 1, 'linear');
        e.alpha.add(def.t1 - fade, def.t1, 0, 'linear');
        return e;
    }

    // evalEntity(e, t) = the logical state of an entity at scene time t (no rig or catalog data needed).
    function evalEntity(e, t) {
        var s = { type: e.type, id: e.id, x: e.x.at(t), y: e.y.at(t), alpha: e.alpha.at(t), scale: e.scale.at(t) * (e.baseScale || 1), rot: e.rot.at(t), depth: e.depth };
        if (e.type === 'actor') {
            var walk = e.walks.active(t), g = e.gestures.active(t);
            s.cast = e.cast;
            s.character = e.character;
            s.params = e.params;
            s.pose = e.pose.blendAt(t);
            s.expression = e.expr.at(t);
            s.facing = e.facing.at(t);
            s.look = e.look.at(t);
            s.walkTime = walk ? t - walk.t0 : null;
            s.phase = e.phase;
            if (g) {
                var span = g.t1 - g.t0, count = g.data.count > 0 ? g.data.count : 1;
                var u = span > 0 ? (t - g.t0) / span * count : 1;
                s.gesture = { name: g.data.name, frac: u - Math.floor(u) };
            } else s.gesture = null;
        } else if (e.type === 'prop') {
            s.item = e.item;
            s.layer = e.layer;
        } else if (e.type === 'text') {
            s.text = e.text;
            s.style = e.style;
        } else if (e.type === 'bubble') {
            s.owner = e.owner;
            s.kind = e.kind;
            s.style = e.style;
            s.lines = e.lines;
        } else if (e.type === 'shape') {
            s.kind = e.kind;
            s.w = e.w.at(t);
            s.h = e.h.at(t);
            s.color = e.color;
            s.stroke = e.stroke;
        }
        var h = e.highlights ? e.highlights.active(t) : null;
        if (h) s.highlight = { style: h.data.style, u: h.t1 > h.t0 ? (t - h.t0) / (h.t1 - h.t0) : 1 };
        return s;
    }

    function entityEnd(e) {
        var end = Math.max(e.x.end(), e.y.end(), e.alpha.end(), e.scale.end(), e.rot.end());
        if (e.type === 'actor') {
            var lists = [e.gestures.list, e.walks.list];
            for (var k = 0; k < lists.length; k++) for (var i = 0; i < lists[k].length; i++) if (lists[k][i].t1 > end) end = lists[k][i].t1;
        }
        var marks = e.highlights ? e.highlights.list : [];
        for (var m = 0; m < marks.length; m++) if (marks[m].t1 > end) end = marks[m].t1;
        return end;
    }

    SM.scene = {
        indexCatalog: indexCatalog, createActor: createActor, createProp: createProp, createText: createText,
        createShape: createShape, createBubble: createBubble, evalEntity: evalEntity, entityEnd: entityEnd
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
