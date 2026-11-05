// Storymaker player: renderFrame(timeline, n, ctx, assets, opts). It draws one frame of the compiled timeline into a 2D
// context of the output size: environment layers with parallax, back props, actors by depth, front props, shapes, texts;
// code scenes come in as ImageBitmaps (assets.codeFrames) and during a transition the two scenes are drawn into two
// canvases (assets.canvas) and composited. The result depends only on the arguments and on n. Classic script;
// publishes StorymakerCore.render.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var FALLBACK_TEXT_STYLE = { font: 'default', size: 0.06, weight: 700, color: '$text', align: 'center', max_lines: 2 };
    var SHADOW_ALPHA = 0.16;
    var PULSE_AMOUNT = 0.08, PULSE_CYCLES = 2, OUTLINE_PX = 6, ARROW_PX = 40, ARROW_BOB_PX = 8, ARROW_SIZE_PX = 28;
    var ACTOR_BOX_WIDTH = 0.42;   // of the actor's height: the rig has no width of its own

    function R() { return SM.rig; }
    function scaleM(sx, sy) { return [sx, 0, 0, sy, 0, 0]; }
    function setM(ctx, m) { ctx.setTransform(m[0], m[1], m[2], m[3], m[4], m[5]); }

    function makeContext(timeline, assets, W, H) {
        return {
            W: W, H: H, aspect: timeline.aspect, fps: timeline.fps, palette: timeline.palette, cat: timeline.cat,
            families: timeline.fontFamilies, fontChain: timeline.fontChain, styleFont: timeline.style.font, safe: timeline.safe || SM.stage.DEFAULT_SAFE,
            assets: assets || {}, codeFrames: (assets && assets.codeFrames) || {}, codeErrors: (assets && assets.codeErrors) || {}
        };
    }

    function stageFor(sc, rc) {
        var key = rc.W + 'x' + rc.H;
        if (!sc.stages) sc.stages = {};
        if (!sc.stages[key]) sc.stages[key] = SM.stage.create({ aspect: rc.aspect, width: rc.W, height: rc.H, env: sc.env, safe: rc.safe });
        return sc.stages[key];
    }

    function familyOf(rc, style) {
        var name = !style.font || style.font === 'default' ? rc.styleFont : style.font;
        var family = rc.families[name] || rc.families.default || 'sans-serif';
        if (!rc.fontChain) return family;
        return rc.fontChain.map(function (f) { return f === '$style' ? family : f; });
    }

    function drawBackdrop(ctx, rc) {
        var g = ctx.createLinearGradient(0, 0, 0, rc.H);
        g.addColorStop(0, rc.palette.bg);
        g.addColorStop(1, rc.palette.bg2 || rc.palette.bg);
        ctx.setTransform(1, 0, 0, 1, 0, 0);
        ctx.globalAlpha = 1;
        ctx.fillStyle = g;
        ctx.fillRect(0, 0, rc.W, rc.H);
    }

    // envTokens(sc, rc): the colour tokens of the scene's environment variant (the day tokens when the variant has none), or null.
    function envTokens(sc, rc) {
        var env = sc.env && rc.assets.envs && rc.assets.envs[sc.env.file];
        return (env && env.tokens && (env.tokens[sc.variant] || env.tokens.day)) || null;
    }

    function drawEnvironment(ctx, sc, rc, win, bw, Mv) {
        var env = sc.env && rc.assets.envs && rc.assets.envs[sc.env.file];
        if (!env) return;
        var tokens = envTokens(sc, rc);
        var resolve = SM.parts.makeResolver([tokens, rc.palette]);
        // the layers are listed back to front; parallax only says how much a layer follows the camera (it is not a draw order)
        var layers = env.layers || [];
        for (var i = 0; i < layers.length; i++) {
            var p = typeof layers[i].parallax === 'number' ? layers[i].parallax : 1;
            var dx = (1 - p) * (win.x - bw.x), dy = (1 - p) * (win.y - bw.y);
            setM(ctx, R().mul(Mv, R().translate(dx, dy)));
            SM.parts.drawShapes(ctx, layers[i].shapes, resolve, 1);
        }
    }

    function drawProp(ctx, st, rc, stage, Mv) {
        var entry = rc.cat.byId[st.item], part = entry && rc.assets.props && rc.assets.props[entry.file];
        if (!part) return;
        var vb = part.viewbox || [0, 0, 100, 100], anchor = entry.anchor || { x: 0.5, y: 1 };
        var depth = stage.depthScale(st.y, st.depth);
        var k = (entry.size && entry.size.h > 0 ? entry.size.h : 0.15) * SM.stage.DESIGN_H * depth * st.scale / (vb[3] || 100);
        var m = R().mul(Mv, R().mul(R().translate(st.x, st.y), R().mul(R().rotate(st.rot), R().mul(scaleM(k, k), R().translate(-(vb[0] + anchor.x * vb[2]), -(vb[1] + anchor.y * vb[3]))))));
        setM(ctx, m);
        SM.parts.drawPart(ctx, part, SM.parts.makeResolver([rc.palette]), st.alpha);
    }

    var MIRROR_POSES = { 'point-left': 'point-right', 'point-right': 'point-left' };

    // poseFor(cat, name, facing): the catalog entry of a pose name. A left-facing actor is drawn mirrored, so point-left and
    // point-right swap entries and the name keeps meaning the direction on the screen. The swap needs both entries.
    function poseFor(cat, name, facing) {
        var other = facing < 0 && MIRROR_POSES[name];
        var pick = other && cat.poses[other] && cat.poses[name] ? other : name;
        return cat.poses[pick] || cat.poses.stand || { bones: {} };
    }

    // rigState(st, t, rc, rig): the draw state of an actor from its logical state (pose blend, gesture, walk or idle cycle).
    function rigState(st, t, rc, rig) {
        var cat = rc.cat, rg = R();
        var poseA = poseFor(cat, st.pose.a, st.facing);
        var poseB = poseFor(cat, st.pose.b, st.facing);
        var u = st.pose.u;
        var bones = rg.lerpBones(poseA.bones, poseB.bones, u);
        var drop = rg.poseDrop(rig, poseA) * (1 - u) + rg.poseDrop(rig, poseB) * u;
        var hands = u >= 0.5 ? poseB.hands : poseA.hands;
        var walking = st.walkTime !== null && st.walkTime !== undefined;
        var cyc = walking ? cat.cycles.walk : cat.cycles.idle;
        var cd = cyc ? rg.sampleKeys(cyc.keys, rg.cyclePhase(cyc, walking ? st.walkTime : t + st.phase)) : { bones: {}, offset_y: 0 };
        var gd = { bones: {}, offset_y: 0 }, gestureHands = null;
        if (st.gesture) {
            var ge = cat.gestures[st.gesture.name];
            if (ge) {
                gd = rg.sampleKeys(ge.keys, st.gesture.frac);
                gestureHands = rg.handsAt(ge.keys, st.gesture.frac);
            }
        }
        var handsOut = {};
        if (hands) { if (hands.l) handsOut.l = hands.l; if (hands.r) handsOut.r = hands.r; }
        if (gestureHands) { if (gestureHands.l) handsOut.l = gestureHands.l; if (gestureHands.r) handsOut.r = gestureHands.r; }
        var view = walking ? 'profile' : (st.look === 'camera' ? 'front' : (poseB.view || 'three_quarter'));
        var expr = cat.expressions[st.expression] || cat.expressions.neutral || {};
        return {
            view: view, offsets: rg.addBones(bones, cd.bones, gd.bones), hands: handsOut, expr: expr,
            mouth: null, blink: ((t + st.phase * 3) % 3.6) < 0.12,
            offsetY: drop + (cd.offset_y || 0) + (gd.offset_y || 0)
        };
    }

    function drawActor(ctx, st, t, rc, stage, Mv) {
        var rig = rc.assets.rig;
        if (!rig) return;
        var body = R().bodyOf(rig, st.params.body), depth = stage.depthScale(st.y, st.depth);
        var s = R().scaleFor(rig, SM.stage.DESIGN_H, depth, st.scale, body);
        var anchorM = R().mul(Mv, R().translate(st.x, st.y));
        // ground shadow
        var rx = s * (rig.units_height || 400) * 0.17;
        setM(ctx, anchorM);
        ctx.globalAlpha = SHADOW_ALPHA * st.alpha;
        ctx.fillStyle = '#000000';
        ctx.beginPath();
        ctx.ellipse(0, 0, rx, rx * 0.22, 0, 0, Math.PI * 2);
        ctx.fill();
        var m = R().mul(anchorM, scaleM(s * (st.facing < 0 ? -1 : 1), s));
        var resolve = SM.parts.makeResolver([R().tokensFor(rig, st.params), rc.palette]);
        R().drawActor(ctx, rig, st.params, rigState(st, t, rc, rig), m, resolve, st.alpha);
    }

    function drawShapeEntity(ctx, st, rc, Mv) {
        var resolve = SM.parts.makeResolver([rc.palette]);
        setM(ctx, R().mul(Mv, R().mul(R().translate(st.x, st.y), R().mul(R().rotate(st.rot), scaleM(st.scale, st.scale)))));
        ctx.globalAlpha = st.alpha > 1 ? 1 : st.alpha;
        if (st.kind === 'circle') {
            ctx.fillStyle = resolve(st.color);
            ctx.beginPath();
            ctx.arc(0, 0, st.w, 0, Math.PI * 2);
            ctx.fill();
        } else if (st.kind === 'line') {
            ctx.strokeStyle = resolve(st.stroke || st.color);
            ctx.lineWidth = st.h;
            ctx.lineCap = 'round';
            ctx.beginPath();
            ctx.moveTo(-st.w / 2, 0);
            ctx.lineTo(st.w / 2, 0);
            ctx.stroke();
        } else {
            ctx.fillStyle = resolve(st.color);
            ctx.fillRect(-st.w / 2, -st.h / 2, st.w, st.h);
            if (st.stroke) { ctx.strokeStyle = resolve(st.stroke); ctx.lineWidth = 3; ctx.strokeRect(-st.w / 2, -st.h / 2, st.w, st.h); }
        }
    }

    // pulseOf(st): the scale factor of a running pulse highlight, else 1. It peaks at u = 1/8.
    function pulseOf(st) {
        var h = st.highlight;
        return h && h.style === 'pulse' ? 1 + PULSE_AMOUNT * Math.sin(2 * Math.PI * PULSE_CYCLES * h.u) : 1;
    }

    function layText(ctx, st, style, rc, x, align) {
        return SM.text.layout(ctx, st.text, style, { canvasH: rc.H, canvasW: rc.W, short: Math.min(rc.W, rc.H), styleName: st.style || 'title', maxWidth: SM.text.maxWidthFor(rc.W, rc.safe, x, align), family: familyOf(rc, style) });
    }

    // entityBox(st, rc, stage): the box of an actor or a prop in design pixels, or null when its art is missing.
    function entityBox(st, rc, stage) {
        var depth = stage.depthScale(st.y, st.depth), h, w;
        if (st.type === 'prop') {
            var entry = rc.cat.byId[st.item], part = entry && rc.assets.props && rc.assets.props[entry.file];
            if (!part) return null;
            var vb = part.viewbox || [0, 0, 100, 100], anchor = entry.anchor || { x: 0.5, y: 1 };
            h = (entry.size && entry.size.h > 0 ? entry.size.h : 0.15) * SM.stage.DESIGN_H * depth * st.scale;
            w = h * (vb[2] || 100) / (vb[3] || 100);
            return { x: st.x - anchor.x * w, y: st.y - anchor.y * h, w: w, h: h };
        }
        var rig = rc.assets.rig;
        if (!rig) return null;
        h = R().scaleFor(rig, SM.stage.DESIGN_H, depth, st.scale, R().bodyOf(rig, st.params.body)) * (rig.units_height || 400);
        w = h * ACTOR_BOX_WIDTH;
        return { x: st.x - w / 2, y: st.y - h, w: w, h: h };
    }

    // textBox(ctx, st, rc, stage, bw): the box of a text in canvas pixels, padding included.
    function textBox(ctx, st, rc, stage, bw) {
        if (!st.text) return null;
        var style = rc.cat.textStyles[st.style] || rc.cat.textStyles.title || FALLBACK_TEXT_STYLE;
        var pos = stage.toCanvas(st.x, st.y, bw), align = style.align || 'center';
        var lay = layText(ctx, st, style, rc, pos.x, align);
        var left = align === 'left' ? pos.x : (align === 'right' ? pos.x - lay.width : pos.x - lay.width / 2);
        return { x: left - lay.pad, y: pos.y - lay.height / 2 - lay.pad, w: lay.width + lay.pad * 2, h: lay.height + lay.pad * 2 };
    }

    // drawHighlight: the outline or arrow of a running highlight, on top of every entity. Actors and props are drawn through
    // the camera matrix, so their sizes are design pixels; a text is in canvas pixels, so its sizes follow the stage scale.
    function drawHighlight(ctx, st, rc, stage, bw, Mv) {
        var h = st.highlight;
        if (!h || h.style === 'pulse' || !(st.alpha > 0)) return;
        var box, unit = 1;
        if (st.type === 'text') {
            ctx.setTransform(1, 0, 0, 1, 0, 0);
            box = textBox(ctx, st, rc, stage, bw);
            unit = stage.scale;
        } else {
            box = entityBox(st, rc, stage);
            setM(ctx, Mv);
        }
        if (!box) return;
        var accent = SM.parts.makeResolver([rc.palette])('$accent');
        ctx.globalAlpha = st.alpha > 1 ? 1 : st.alpha;
        if (h.style === 'outline') {
            ctx.strokeStyle = accent;
            ctx.lineWidth = OUTLINE_PX * unit;
            ctx.strokeRect(box.x, box.y, box.w, box.h);
            return;
        }
        var cx = box.x + box.w / 2, tip = box.y - (ARROW_PX + ARROW_BOB_PX * Math.sin(2 * Math.PI * PULSE_CYCLES * h.u)) * unit;
        ctx.fillStyle = accent;
        ctx.beginPath();
        ctx.moveTo(cx, tip);
        ctx.lineTo(cx - ARROW_SIZE_PX / 2 * unit, tip - ARROW_SIZE_PX * unit);
        ctx.lineTo(cx + ARROW_SIZE_PX / 2 * unit, tip - ARROW_SIZE_PX * unit);
        ctx.closePath();
        ctx.fill();
    }

    // drawText: tokens (optional) are the environment's colour tokens; they win over the palette, so a dark environment can carry
    // its own text colour and label box colour.
    function drawText(ctx, st, rc, stage, bw, tokens) {
        if (!(st.alpha > 0) || !st.text) return;
        var style = rc.cat.textStyles[st.style] || rc.cat.textStyles.title || FALLBACK_TEXT_STYLE;
        var pos = stage.toCanvas(st.x, st.y, bw), align = style.align || 'center';
        ctx.setTransform(1, 0, 0, 1, 0, 0);
        var lay = layText(ctx, st, style, rc, pos.x, align), k = pulseOf(st);
        // a text reads no scale otherwise: a pulse scales it around its anchor
        if (k !== 1) ctx.setTransform(k, 0, 0, k, pos.x * (1 - k), pos.y * (1 - k));
        SM.text.draw(ctx, lay, style, pos.x, pos.y, SM.parts.makeResolver([tokens, rc.palette]), st.alpha);
        if (k !== 1) ctx.setTransform(1, 0, 0, 1, 0, 0);
    }

    // drawScene(ctx, sc, t, rc): a storyboard or code-scene-like object {env, variant, camera, entities, stages}.
    function drawScene(ctx, sc, t, rc) {
        var stage = stageFor(sc, rc);
        var cam = SM.camera.at(sc.camera, t), win = stage.cameraWindow(cam), bw = stage.baseWindow(), Mv = stage.matrix(win);
        drawBackdrop(ctx, rc);
        drawEnvironment(ctx, sc, rc, win, bw, Mv);
        var states = sc.entities.map(function (e, k) { var s = SM.scene.evalEntity(e, t); s.k = k; s.scale *= pulseOf(s); return s; });
        var live = function (s) { return s.alpha > 0; };
        states.filter(function (s) { return s.type === 'prop' && s.layer === 'back' && live(s); }).forEach(function (s) { drawProp(ctx, s, rc, stage, Mv); });
        states.filter(function (s) { return s.type === 'actor' && live(s); })
            .sort(function (a, b) { return a.y - b.y || a.k - b.k; })
            .forEach(function (s) { drawActor(ctx, s, t, rc, stage, Mv); });
        states.filter(function (s) { return s.type === 'prop' && s.layer !== 'back' && live(s); }).forEach(function (s) { drawProp(ctx, s, rc, stage, Mv); });
        states.filter(function (s) { return s.type === 'shape' && live(s); }).forEach(function (s) { drawShapeEntity(ctx, s, rc, Mv); });
        var tokens = envTokens(sc, rc);
        states.filter(function (s) { return s.type === 'text'; }).forEach(function (s) { drawText(ctx, s, rc, stage, bw, tokens); });
        states.forEach(function (s) { drawHighlight(ctx, s, rc, stage, bw, Mv); });
        ctx.setTransform(1, 0, 0, 1, 0, 0);
        ctx.globalAlpha = 1;
    }

    function drawPlaceholder(ctx, sc, rc) {
        drawBackdrop(ctx, rc);
        var msg = (rc.codeErrors[sc.id]) || (sc.code && sc.code.error && sc.code.error.message) || 'Code scene';
        var stage = stageFor(sc, rc), bw = stage.baseWindow();
        drawText(ctx, { alpha: 1, text: 'Code scene "' + sc.id + '"', style: 'title', x: stage.focusX, y: 440 }, rc, stage, bw);
        drawText(ctx, { alpha: 1, text: String(msg), style: 'caption', x: stage.focusX, y: 600 }, rc, stage, bw);
    }

    function drawLayer(ctx, timeline, layer, rc) {
        var sc = timeline.scenes[layer.i];
        if (sc.kind === 'code') {
            var bmp = rc.codeFrames[sc.id];
            if (bmp) {
                ctx.setTransform(1, 0, 0, 1, 0, 0);
                ctx.globalAlpha = 1;
                ctx.drawImage(bmp, 0, 0, rc.W, rc.H);
            } else drawPlaceholder(ctx, sc, rc);
            return;
        }
        drawScene(ctx, sc, layer.f / timeline.fps, rc);
    }

    // renderFrame(timeline, n, ctx, assets, opts): assets = {rig, props: {file: json}, envs: {file: json}, codeFrames,
    // codeErrors, canvas(name, w, h) -> {canvas, ctx}}; opts.width renders at another width (thumbnails).
    function renderFrame(timeline, n, ctx, assets, opts) {
        var W = opts && opts.width > 0 ? Math.round(opts.width) : timeline.width;
        var H = opts && opts.width > 0 ? Math.round(opts.width * timeline.height / timeline.width) : timeline.height;
        var rc = makeContext(timeline, assets, W, H);
        var plan = SM.schedule.framePlan(timeline.schedule, n);
        if (!plan.layers.length) {
            drawBackdrop(ctx, rc);
            return;
        }
        if (!plan.transition) {
            drawLayer(ctx, timeline, plan.layers[0], rc);
            return;
        }
        var prev = assets.canvas('prev', W, H), cur = assets.canvas('cur', W, H);
        drawLayer(prev.ctx, timeline, plan.layers[0], rc);
        drawLayer(cur.ctx, timeline, plan.layers[1], rc);
        SM.transitions.composite(ctx, plan.transition.kind, plan.transition.p, W, H, prev.canvas, cur.canvas);
    }

    SM.render = {
        renderFrame: renderFrame, drawScene: drawScene, rigState: rigState, makeContext: makeContext, familyOf: familyOf,
        drawBackdrop: drawBackdrop, drawText: drawText, stageFor: stageFor, pulseOf: pulseOf
    };
    SM.renderFrame = renderFrame;
})(typeof globalThis !== 'undefined' ? globalThis : self);
