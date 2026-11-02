// Storymaker player: the 2D character rig (rig.human/1). 13 bones with a rest transform per view (front, three_quarter,
// profile), slots (vector parts attached to a bone, drawn in z order per view), poses (bone angle offsets), gestures and
// cycles (timed additive bone deltas that return to the base pose), expressions (eye/brow/mouth variants), speech mouth
// shapes and hands. The semantics are contract section 3a: world(b) = world(parent) . T(rest.x, rest.y) .
// R(rest.angle + pose[b] + sum of deltas[b]), angles in degrees, clockwise positive. Mirroring for left-facing actors
// is done by the caller (a scale(-1, 1) about the root). Classic script; publishes StorymakerCore.rig.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var VIEWS = ['front', 'three_quarter', 'profile'];
    var IDENTITY = [1, 0, 0, 1, 0, 0];

    function mul(m, n) {
        return [
            m[0] * n[0] + m[2] * n[1], m[1] * n[0] + m[3] * n[1],
            m[0] * n[2] + m[2] * n[3], m[1] * n[2] + m[3] * n[3],
            m[0] * n[4] + m[2] * n[5] + m[4], m[1] * n[4] + m[3] * n[5] + m[5]
        ];
    }
    function translate(x, y) { return [1, 0, 0, 1, x, y]; }
    function rotate(deg) {
        var r = deg * Math.PI / 180, c = Math.cos(r), s = Math.sin(r);
        return [c, s, -s, c, 0, 0];
    }

    // index(rig): bone order (parents before children) and name lookup; cached on the rig object.
    function index(rig) {
        if (rig.__idx) return rig.__idx;
        var byName = {}, order = [], bones = rig.bones || [], i;
        for (i = 0; i < bones.length; i++) byName[bones[i].name] = bones[i];
        var seen = {};
        function visit(b) {
            if (seen[b.name]) return;
            seen[b.name] = true;
            if (b.parent && byName[b.parent]) visit(byName[b.parent]);
            order.push(b);
        }
        for (i = 0; i < bones.length; i++) visit(bones[i]);
        var slots = (rig.slots || []).map(function (s, k) { return { s: s, k: k }; });
        var zOrder = {};
        VIEWS.forEach(function (view) {
            zOrder[view] = slots.slice().sort(function (a, b) {
                var za = a.s.z && typeof a.s.z[view] === 'number' ? a.s.z[view] : 0;
                var zb = b.s.z && typeof b.s.z[view] === 'number' ? b.s.z[view] : 0;
                return za - zb || a.k - b.k;
            }).map(function (entry) { return entry.s; });
        });
        Object.defineProperty(rig, '__idx', { value: { byName: byName, order: order, zOrder: zOrder }, enumerable: false });
        return rig.__idx;
    }

    function viewOf(name) { return VIEWS.indexOf(name) >= 0 ? name : 'three_quarter'; }

    // bodyOf(rig, name) = {scale, bones}; an unknown body is the plain rig.
    function bodyOf(rig, name) {
        var b = rig.bodies && rig.bodies[name];
        return b || { scale: 1 };
    }

    function restOf(bone, view, body) {
        var r = (bone.rest && (bone.rest[view] || bone.rest.three_quarter || bone.rest.front)) || { x: 0, y: 0, angle: 0 };
        var o = body && body.bones && body.bones[bone.name] && body.bones[bone.name].rest && body.bones[bone.name].rest[view];
        if (!o) return r;
        return { x: typeof o.x === 'number' ? o.x : r.x, y: typeof o.y === 'number' ? o.y : r.y, angle: typeof o.angle === 'number' ? o.angle : r.angle };
    }

    // angles(a, b, c ...): sums bone-angle maps (pose, gesture delta, cycle delta).
    function addBones() {
        var out = {};
        for (var i = 0; i < arguments.length; i++) {
            var m = arguments[i];
            if (!m) continue;
            for (var k in m) if (typeof m[k] === 'number') out[k] = (out[k] || 0) + m[k];
        }
        return out;
    }

    function lerpBones(a, b, u) {
        if (u >= 1) return b || {};
        if (u <= 0 || !a) return a || {};
        var out = {}, k;
        for (k in a) out[k] = (a[k] || 0) * (1 - u);
        for (k in (b || {})) out[k] = (out[k] || 0) + b[k] * u;
        return out;
    }

    // boneAngles(rig, view, body, extra): the final angle of every bone (rest angle + sum of the offset maps).
    function boneAngles(rig, view, body, offsets) {
        var idx = index(rig), out = {};
        for (var i = 0; i < idx.order.length; i++) {
            var bone = idx.order[i];
            out[bone.name] = restOf(bone, viewOf(view), body).angle + ((offsets && offsets[bone.name]) || 0);
        }
        return out;
    }

    // boneMatrices(rig, view, body, offsets) = {bone: matrix} in rig space (root at the origin).
    function boneMatrices(rig, view, body, offsets) {
        var idx = index(rig), world = {};
        view = viewOf(view);
        for (var i = 0; i < idx.order.length; i++) {
            var bone = idx.order[i], r = restOf(bone, view, body);
            var local = mul(translate(r.x || 0, r.y || 0), rotate((r.angle || 0) + ((offsets && offsets[bone.name]) || 0)));
            world[bone.name] = bone.parent && world[bone.parent] ? mul(world[bone.parent], local) : local;
        }
        return world;
    }

    function smooth(f) { return f * f * (3 - 2 * f); }

    // sampleKeys(keys, u): additive bone deltas and offset_y of gesture/cycle keys at u (0..1), linear between keys with an
    // ease-in-out on each segment. A bone missing from a key counts as 0.
    function sampleKeys(keys, u) {
        if (!keys || !keys.length) return { bones: {}, offset_y: 0 };
        if (u <= keys[0].t) return { bones: keys[0].bones || {}, offset_y: keys[0].offset_y || 0 };
        var last = keys[keys.length - 1];
        if (u >= last.t) return { bones: last.bones || {}, offset_y: last.offset_y || 0 };
        var k = 0;
        while (k < keys.length - 2 && keys[k + 1].t <= u) k++;
        var a = keys[k], b = keys[k + 1], span = b.t - a.t, f = span > 0 ? smooth((u - a.t) / span) : 1;
        var bones = {}, name;
        for (name in (a.bones || {})) bones[name] = a.bones[name] * (1 - f);
        for (name in (b.bones || {})) bones[name] = (bones[name] || 0) + b.bones[name] * f;
        return { bones: bones, offset_y: (a.offset_y || 0) * (1 - f) + (b.offset_y || 0) * f };
    }

    // handsAt(keys, u): the hand override of the last key at or before u (a key without hands clears the override).
    function handsAt(keys, u) {
        var hands = null;
        for (var i = 0; i < (keys || []).length; i++) {
            if (keys[i].t <= u) hands = keys[i].hands || null; else break;
        }
        return hands;
    }

    // cyclePhase(entry, seconds): 0..1 phase of a periodic cycle.
    function cyclePhase(entry, seconds) {
        var period = entry && entry.period > 0 ? entry.period : 1;
        var p = (seconds / period) % 1;
        return p < 0 ? p + 1 : p;
    }

    // Colour tokens of a character: skin/hair/outfit colours from the rig parameter tables, the fixed set last.
    function tokensFor(rig, params) {
        var p = rig.params || {}, outfit = p.outfit && p.outfit[params.outfit] || {}, colors = outfit.colors || {};
        var t = {}, fixed = p.fixed || {}, k;
        for (k in fixed) t[k] = fixed[k];
        if (p.skin && p.skin[params.skin]) t.skin = p.skin[params.skin];
        if (p.hair_color && p.hair_color[params.hair_color]) t.hair = p.hair_color[params.hair_color];
        if (colors.top) t.top = colors.top;
        if (colors.bottom) t.bottom = colors.bottom;
        if (colors.shoes) t.shoes = colors.shoes;
        return t;
    }

    // variantKeys(rig, params, slot, state): candidate variant keys of a slot, best first.
    function variantKeys(rig, params, slot, state) {
        var expr = state.expr || {}, outfit = rig.params && rig.params.outfit && rig.params.outfit[params.outfit] || {};
        switch (slot.param) {
            case 'body': return [params.body];
            case 'skin': return [params.skin];
            case 'hair': case 'hair_back': return [params.hair];
            case 'eyes':
                return (state.blink ? [params.eyes + '/blink'] : []).concat([params.eyes + '/' + (expr.eyes || 'neutral'), params.eyes + '/neutral']);
            case 'brows': return [params.brows + '/' + (expr.brows || 'neutral'), params.brows + '/neutral'];
            case 'mouth': return [state.mouth || expr.mouth || 'rest', 'rest'];
            case 'hand': {
                var side = /_r$/.test(slot.name) ? 'r' : 'l';
                return [(state.hands && state.hands[side]) || 'open', 'open'];
            }
            case 'top': case 'bottom': case 'shoes': return [outfit[slot.param]];
            default: return [params[slot.param]];
        }
    }

    function pickVariant(rig, params, slot, state, view) {
        var table = rig.variants && rig.variants[slot.name];
        if (!table) return null;
        var keys = variantKeys(rig, params, slot, state);
        for (var i = 0; i < keys.length; i++) {
            var key = keys[i];
            if (key === undefined || key === null || key === 'none') {
                if (key === 'none') return null;
                continue;
            }
            var v = table[key];
            if (v) return v[view] || v.three_quarter || v.front || null;
        }
        return null;
    }

    // Screen scale of an actor (contract 3a): rig.height x stage height x depth x actor scale x body scale / units_height.
    function scaleFor(rig, designHeight, depth, actorScale, body) {
        var units = rig.units_height > 0 ? rig.units_height : 1;
        return (rig.height || units) * designHeight * depth * actorScale * ((body && body.scale) || 1) / units;
    }

    // seatedOffset(rig): how far the root drops (rig units) for seated poses.
    function seatedOffset(rig) {
        if (typeof rig.seated_offset === 'number') return rig.seated_offset;
        var idx = index(rig), lower = idx.byName.lower_leg_l;
        return lower && lower.length > 0 ? lower.length : 0.24 * (rig.units_height || 400);
    }

    // drawActor(ctx, rig, params, state, base, resolve, alpha): draws every visible slot. base is the matrix from rig
    // space to the canvas (placement, scale, mirroring); state = {view, offsets, hands, expr, mouth, blink, offsetY}.
    function drawActor(ctx, rig, params, state, base, resolve, alpha) {
        var idx = index(rig), view = viewOf(state.view);
        var body = bodyOf(rig, params.body);
        var world = boneMatrices(rig, view, body, state.offsets);
        var lift = translate(0, state.offsetY || 0);
        var slots = idx.zOrder[view];
        for (var i = 0; i < slots.length; i++) {
            var slot = slots[i], shapes = pickVariant(rig, params, slot, state, view), m = world[slot.bone];
            if (!shapes || !m) continue;
            var full = mul(base, mul(lift, m));
            ctx.setTransform(full[0], full[1], full[2], full[3], full[4], full[5]);
            SM.parts.drawShapes(ctx, shapes, resolve, alpha);
        }
    }

    SM.rig = {
        VIEWS: VIEWS, IDENTITY: IDENTITY, mul: mul, translate: translate, rotate: rotate, index: index, bodyOf: bodyOf,
        addBones: addBones, lerpBones: lerpBones, boneAngles: boneAngles, boneMatrices: boneMatrices,
        sampleKeys: sampleKeys, handsAt: handsAt, cyclePhase: cyclePhase, tokensFor: tokensFor, scaleFor: scaleFor,
        seatedOffset: seatedOffset, pickVariant: pickVariant, drawActor: drawActor
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
