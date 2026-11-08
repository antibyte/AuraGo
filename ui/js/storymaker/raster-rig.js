// Sprite attachment pivots and deterministic two-bone foot placement; the compiler still owns the walk interval.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});
    var RAD = Math.PI / 180;
    function slotMatrix(world, slot) {
        var R = SM.rig, m = slot.pivot ? R.mul(world, R.translate(slot.pivot[0], slot.pivot[1])) : world;
        if (slot.upright) m = R.mul(m, R.rotate(-Math.atan2(m[1], m[0]) / RAD));
        return slot.shear ? R.mul(m, [1, 0, slot.shear, 1, 0, 0]) : m;
    }
    function solve(rig, body, view, offsets, upper, lower, targetX, targetY, second) {
        var R = SM.rig, worlds = R.boneMatrices(rig, view, body, offsets), origin = worlds[upper], joint = worlds[lower];
        var first = Math.hypot(joint[4] - origin[4], joint[5] - origin[5]);
        var dx = targetX - origin[4], dy = targetY - origin[5], distance = Math.hypot(dx, dy);
        distance = Math.max(Math.abs(first - second) + 0.001, Math.min(first + second - 0.001, distance));
        var heading = Math.atan2(-dx, dy), bend = Math.acos(Math.max(-1, Math.min(1, (distance * distance + first * first - second * second) / (2 * distance * first))));
        var a = heading - bend, kneeX = origin[4] - Math.sin(a) * first, kneeY = origin[5] + Math.cos(a) * first;
        var b = Math.atan2(-(targetX - kneeX), targetY - kneeY), angles = R.boneAngles(rig, view, body, offsets);
        var parent = worlds[R.index(rig).byName[upper].parent], parentAngle = parent ? Math.atan2(parent[1], parent[0]) / RAD : 0;
        offsets[upper] = a / RAD - parentAngle - (angles[upper] - (offsets[upper] || 0));
        offsets[lower] = (b - a) / RAD - (angles[lower] - (offsets[lower] || 0));
    }
    function adjust(pose, state, entity, time, rc, stage) {
        var R = SM.rig, rig = rc.assets.rig, body = R.bodyOf(rig, state.params.body);
        var worlds = R.boneMatrices(rig, pose.view, body, pose.offsets);
        var upper = Math.hypot(worlds.lower_leg_r[4] - worlds.upper_leg_r[4], worlds.lower_leg_r[5] - worlds.upper_leg_r[5]);
        var seated = function (name) { return name === 'sit' || name === 'typing'; };
        var seatWeight = (seated(state.pose.a) ? 1 - state.pose.u : 0) + (seated(state.pose.b) ? state.pose.u : 0);
        pose.offsetY -= seatWeight * ((rig.seat_drop || 215) - upper);
        var idle = rc.cat.cycles.idle;
        var idleY = idle ? R.sampleKeys(idle.keys, R.cyclePhase(idle, time + state.phase)).offset_y || 0 : 0;
        if (state.walkTime === null || state.walkTime === undefined || !entity) { pose.offsetY -= idleY; return; }
        var interval = entity.walks.active(time), cycle = rc.cat.cycles.walk;
        if (!interval || !cycle) return;
        var x0 = entity.x.at(interval.t0), y0 = entity.y.at(interval.t0), x1 = entity.x.at(interval.t1), y1 = entity.y.at(interval.t1);
        var total = Math.hypot(x1 - x0, y1 - y0);
        if (total < 0.001) return;
        var scale = R.scaleFor(rig, SM.stage.DESIGN_H, stage.depthScale(state.y, state.depth), state.scale, body);
        var startScale = R.scaleFor(rig, SM.stage.DESIGN_H, stage.depthScale(y0, state.depth), entity.scale.at(interval.t0) * (entity.baseScale || 1), body);
        if (!(scale > 0.000001) || !(startScale > 0.000001)) return;
        var lower = R.index(rig).byName.lower_leg_r.length || 205, stride = (upper + lower) * 0.7 * startScale;
        var travelled = Math.hypot(state.x - x0, state.y - y0), turns = travelled / stride, phase = turns - Math.floor(turns);
        var oldCycle = R.sampleKeys(cycle.keys, R.cyclePhase(cycle, state.walkTime)), gait = R.sampleKeys(cycle.keys, phase), subtract = {};
        Object.keys(oldCycle.bones || {}).forEach(function (bone) { subtract[bone] = -oldCycle.bones[bone]; });
        var standing = SM.render.rigState(Object.assign({}, state, { walkTime: null }), time, rc, rig);
        standing.offsetY -= idleY + seatWeight * ((rig.seat_drop || 215) - upper);
        var weight = Math.max(0, Math.min(1, (time - interval.t0) / 0.12, (interval.t1 - time) / 0.12));
        weight = weight * weight * (3 - 2 * weight);
        pose.offsets = R.lerpBones(standing.offsets, R.addBones(pose.offsets, subtract, gait.bones), weight);
        pose.offsetY = standing.offsetY * (1 - weight) + (pose.offsetY - (oldCycle.offset_y || 0) + 24 + 6 * Math.cos(phase * Math.PI * 4)) * weight;
        var facing = state.facing < 0 ? -1 : 1;
        ['r', 'l'].forEach(function (side, index) {
            var offset = index * 0.5, turn = turns + offset, localPhase = turn - Math.floor(turn), base = Math.floor(turn) - offset;
            var anchor = (base + 0.275) * stride, lift = 0;
            if (localPhase > 0.55) {
                var swing = (localPhase - 0.55) / 0.45, smooth = swing * swing * (3 - 2 * swing);
                anchor = travelled + (-0.275 + smooth * 0.55) * stride;
                lift = Math.sin(swing * Math.PI) * 65;
            }
            var along = anchor - travelled;
            var x = along * (x1 - x0) / total / scale * facing;
            var y = along * (y1 - y0) / total / scale - 20 - pose.offsetY - lift;
            var a = 'upper_leg_' + side, b = 'lower_leg_' + side, priorA = standing.offsets[a] || 0, priorB = standing.offsets[b] || 0;
            solve(rig, body, pose.view, pose.offsets, a, b, x, y, lower);
            pose.offsets[a] = priorA * (1 - weight) + pose.offsets[a] * weight;
            pose.offsets[b] = priorB * (1 - weight) + pose.offsets[b] * weight;
        });
        pose.planted = true;
    }
    SM.rasterRig = { slotMatrix: slotMatrix, adjust: adjust };
})(typeof globalThis !== 'undefined' ? globalThis : self);
