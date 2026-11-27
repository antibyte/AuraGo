// Storymaker player: the layout run. measure(timeline, assets) samples every storyboard scene once, at its hold frame (after
// its last cue arrival and its last motion, plus half a second), and reports where the actors, the shown props, the texts and
// the burned caption are, as boxes in fractions of the output frame. Go turns the boxes into findings. It reads the compiled
// timeline and the loaded art and draws nothing; the one canvas it uses only measures text. Classic script; publishes
// StorymakerCore.layout. A code scene gets a block with code true and the boxes of its explainer blocks (sc.codeLayout, from the
// scene worker); raw drawing in a code scene is never measured.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var HOLD_AFTER = 0.5;       // seconds after the last arrival or motion
    var CODE_HOLD_SHARE = 0.7;  // a code scene is sampled at this share of its length (render/compileshots.go shotDefaultFraction)
    var VISIBLE = 0.05;         // an entity with less alpha is not on the stage
    var EPS = 0.5;              // design pixels: a smaller move is no move

    function round4(v) { return Math.round(v * 10000) / 10000; }

    // holdSeconds(timeline, index): the scene-local time of the sample, clamped to the last frame in which the scene is the
    // one on screen (the next scene's entry transition overlaps its end).
    // A code scene has no stage entities of the storyboard: it is sampled at the larger of 70 % of its length and half a second
    // after its last cue or the end of its last block, and never after its last frame.
    function holdSeconds(timeline, index) {
        var sc = timeline.scenes[index], next = timeline.scenes[index + 1], fps = timeline.fps, last = 0;
        var cues = sc.narration && sc.narration.cueList || [];
        for (var i = 0; i < cues.length; i++) if (cues[i].at_ms / 1000 > last) last = cues[i].at_ms / 1000;
        var lastFrame = Math.min(sc.end_frame, next ? next.start_frame : sc.end_frame) - 1;
        var limit = Math.max(0, (lastFrame - sc.start_frame) / fps);
        if (sc.kind === 'code') {
            var held = sc.codeLayout ? sc.codeLayout.entities : [];
            for (var h = 0; h < held.length; h++) { var he = SM.scene.entityEnd(held[h]); if (he > last) last = he; }
            return Math.min(Math.max(CODE_HOLD_SHARE * (sc.end_frame - sc.start_frame) / fps, last + HOLD_AFTER), limit);
        }
        for (var k = 0; k < sc.entities.length; k++) { var end = SM.scene.entityEnd(sc.entities[k]); if (end > last) last = end; }
        return Math.min(last + HOLD_AFTER, limit);
    }

    // frameBox(box, win, stage, W, H): a design box under a camera window as [x, y, w, h] in fractions of the canvas.
    function frameBox(box, win, stage, W, H) {
        var a = stage.toCanvas(box.x, box.y, win), b = stage.toCanvas(box.x + box.w, box.y + box.h, win);
        return [round4(a.x / W), round4(a.y / H), round4((b.x - a.x) / W), round4((b.y - a.y) / H)];
    }

    // pixelBox(box, W, H): a canvas-pixel box as fractions of the canvas.
    function pixelBox(box, W, H) { return [round4(box.x / W), round4(box.y / H), round4(box.w / W), round4(box.h / H)]; }

    // snapshot(sc, t): what a viewer could see change, without the idle cycle and the mouth.
    function snapshot(sc, t) {
        var out = [];
        for (var i = 0; i < sc.entities.length; i++) {
            var e = sc.entities[i];
            if (e.type !== 'actor' && e.type !== 'prop' && e.type !== 'text' && e.type !== 'shape' && e.type !== 'block') continue;
            var s = SM.scene.evalEntity(e, t);
            if (e.type === 'block') { out.push([e.id, SM.blocks.cacheKey(s), Math.round(SM.blocks.itemProgress(s, 0) * 50), Math.round(SM.blocks.itemProgress(s, Math.max(0, SM.blocks.itemCount(s.block) - 1)) * 50)]); continue; }
            out.push([
                e.id, Math.round(s.x / EPS), Math.round(s.y / EPS), Math.round(s.alpha * 50), Math.round(s.scale * 50), Math.round(s.rot * 50),
                typeof s.pose === 'string' ? s.pose : (s.pose && s.pose.b) || '', s.expression || '', s.gesture && s.gesture.name || ''
            ]);
        }
        return JSON.stringify(out);
    }

    // layoutOf(block): the diagram layout, the chart type or the icon style; "" for a headline and a list.
    function layoutOf(b) {
        if (b.kind === 'diagram') return b.diagram.layout;
        if (b.kind === 'chart') return b.chart.type;
        return b.kind === 'icon' ? b.style || 'line' : '';
    }

    // blockBoxes(entities, camera, t, rc, ctx, stage, W, H): the boxes of the visible blocks under a camera at scene time t.
    function blockBoxes(entities, camera, t, rc, ctx, stage, W, H) {
        var out = [], win = camera ? stage.cameraWindow(SM.camera.at(camera, t)) : stage.baseWindow();
        for (var i = 0; i < entities.length && ctx; i++) {
            var st = SM.scene.evalEntity(entities[i], t);
            if (entities[i].type !== 'block' || !(st.alpha > VISIBLE)) continue;
            var m = SM.blocks.measure(ctx, st, rc, stage, win);
            if (m) out.push({ id: st.id, kind: st.block.kind, layout: layoutOf(st.block), label: round4(m.labelPx / Math.min(W, H)), box: frameBox(m.box, win, stage, W, H) });
        }
        return out;
    }

    function measureScene(timeline, index, rc, ctx) {
        var sc = timeline.scenes[index], W = timeline.width, H = timeline.height, fps = timeline.fps;
        var t = holdSeconds(timeline, index), stage = SM.render.stageFor(sc, rc);
        var win = stage.cameraWindow(SM.camera.at(sc.camera, t)), base = stage.baseWindow();
        var code = sc.kind === 'code';
        var block = { id: sc.id, index: sc.index, hold_s: round4(t), code: code, actors: [], props: [], texts: [], blocks: [], caption: null, missing: 0, static: false };
        for (var i = 0; i < sc.entities.length && !code; i++) {
            var e = sc.entities[i], st = SM.scene.evalEntity(e, t);
            if (!(st.alpha > VISIBLE)) continue;
            if (e.type === 'actor' || e.type === 'prop') {
                var box = SM.render.entityBox(st, rc, stage);
                if (!box) { block.missing++; continue; }
                var framed = frameBox(box, win, stage, W, H);
                if (e.type === 'actor') block.actors.push({ id: e.id, box: framed });
                else block.props.push({ id: e.id, item: e.item, hero: !!e.hero, back: e.layer === 'back', scale: round4(e.baseScale || 1), box: framed });
            } else if (e.type === 'text' && st.text && ctx) {
                var tb = SM.render.textBox(ctx, st, rc, stage, base);
                if (tb) block.texts.push({ id: e.id, box: pixelBox(tb, W, H) });
            }
        }
        block.blocks = code ? (sc.codeLayout ? blockBoxes(sc.codeLayout.entities, sc.codeLayout.camera, t, rc, ctx, stage, W, H) : []) : blockBoxes(sc.entities, sc.camera, t, rc, ctx, stage, W, H);
        if (timeline.style.captions && timeline.captions && timeline.captions.length && ctx) {
            var cap = SM.captions.at(timeline.captions, sc.start_frame * 1000 / fps + t * 1000);
            if (cap) block.caption = pixelBox(SM.render.captionBox(ctx, cap, rc), W, H);
        }
        if (!code) block.static = snapshot(sc, 0) === snapshot(sc, t);
        return block;
    }

    // measure(timeline, assets): one block per scene, in story order. assets is what the renderer gets (rig, props,
    // envs, canvas(name, w, h)). A scene with missing art still reports; its missing count says how many boxes are absent.
    function measure(timeline, assets) {
        var W = timeline.width, H = timeline.height, rc = SM.render.makeContext(timeline, assets, W, H);
        var holder = assets && typeof assets.canvas === 'function' ? assets.canvas('layout', W, H) : null, ctx = holder && holder.ctx || null;
        var out = [];
        for (var i = 0; i < timeline.scenes.length; i++) {
            out.push(measureScene(timeline, i, rc, ctx));
        }
        return out;
    }

    SM.layout = { measure: measure, holdSeconds: holdSeconds, HOLD_AFTER: HOLD_AFTER };
})(typeof globalThis !== 'undefined' ? globalThis : self);
