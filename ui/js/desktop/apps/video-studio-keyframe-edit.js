(function () {
    'use strict';

    // DOM-free keyframe operations (spec §5.1, §5.2, §8.2; plan index §4.18). Like VideoStudioEdit, an operation that
    // takes a project changes it in place and returns true or a VideoStudioEdit.REASON; after a refusal the caller
    // discards the project (the app runs them inside P.applyEdit, which restores its snapshot). Keys live on visual-track
    // clips at source frames: a key set at timeline frame T is stored at f = round(sourceAt(clip, T)), so it stays on its
    // picture content through every timing edit. A key is named {path, f}. Paths are the clip's animatable values
    // (VideoStudioEffects.animatablePaths, 2c): base values, crop.l|t|r|b and effects.<index>.<param>; the path store
    // below knows where their keys and fields live. The speed ramp operations follow the key operations, and the effect
    // stack operations (2c) follow the ramps.
    const Fx = () => window.VideoStudioEffects;
    const T = () => window.VideoStudioTimeline;
    const REASON = () => window.VideoStudioEdit.REASON;
    const AXES = [['x', 'width'], ['y', 'height']];
    const MIN_SIZE = window.VideoStudioEffects.MIN_BOX_SIZE;
    // The crop (spec §5.3): each side 0..0.95 of the source picture, l + r and t + b at most 0.95.
    const CROP_SIDES = ['l', 't', 'r', 'b'], CROP_AXES = [['l', 'r'], ['t', 'b']], CROP_MAX = 0.95;
    const EFFECT_PATH = /^effects\.(\d+)\.([a-z][a-z0-9_]*)$/;

    const keyId = (path, f) => path + ':' + f;
    const keySet = keys => new Set((keys || []).map(key => keyId(key.path, Number(key.f))));
    // A path that can carry keys on this clip: a keyframable base value or crop side, or a keyframable number param of one
    // of the clip's effect entries.
    const animatable = (clip, path) => !!clip && Fx().animatablePaths(clip).includes(path);

    // ---- Path store (2c). Keys of base values and crop sides live in clip.keyframes[path], keys of an effect param in its
    // entry (clip.effects[i].keyframes[param]). Static values live in clip[path], clip.crop[side] (the crop object exists
    // only while a side is non-zero, as Validate requires) and clip.effects[i].params[param].
    function effectPath(path) { const match = EFFECT_PATH.exec(String(path)); return match ? { index: Number(match[1]), param: match[2] } : null; }
    // {holder, key}: the object whose keyframes map holds the path's list, and the list's name in it; null for an effect
    // path without that entry.
    function holderOf(clip, path) {
        if (!clip) return null;
        const fx = effectPath(path);
        if (!fx) return { holder: clip, key: path };
        const entry = Array.isArray(clip.effects) ? clip.effects[fx.index] : null;
        return entry ? { holder: entry, key: fx.param } : null;
    }
    // The stored list of a path (read it, do not keep it); [] without keys.
    function listOf(clip, path) {
        const at = holderOf(clip, path), list = at && at.holder.keyframes ? at.holder.keyframes[at.key] : null;
        return Array.isArray(list) ? list : [];
    }
    // Stores a path's list. An empty list is removed, and so is a keyframes map it leaves empty (canonical form, §4.1).
    function setList(clip, path, list) {
        const at = holderOf(clip, path);
        if (!at) return;
        if (list.length) { at.holder.keyframes = Object.assign(at.holder.keyframes || {}, { [at.key]: list }); return; }
        if (!at.holder.keyframes) return;
        delete at.holder.keyframes[at.key];
        if (!Object.keys(at.holder.keyframes).length) delete at.holder.keyframes;
    }
    function isAnimated(clip, path) { return listOf(clip, path).length > 0; }
    // Every path of the clip that holds keys: the clip's own paths sorted (Go's map order), then each effect entry's
    // params in stack order.
    function keyedPaths(clip) {
        const out = Object.keys((clip && clip.keyframes) || {}).filter(path => isAnimated(clip, path)).sort();
        ((clip && clip.effects) || []).forEach((entry, index) => Object.keys((entry && entry.keyframes) || {}).sort().forEach(param => {
            if (isAnimated(clip, 'effects.' + index + '.' + param)) out.push('effects.' + index + '.' + param);
        }));
        return out;
    }
    // The clip's keys, base and effect keys together (Go countKeyframes; at most LIMITS.keyframesPerClip).
    function keyCount(clip) { return keyedPaths(clip).reduce((n, path) => n + listOf(clip, path).length, 0); }
    function inside(clip, frame) { return frame >= clip.start && frame < clip.start + clip.duration; }
    // A number inside the path's registry range (VideoStudioEffects.paramSpec); widths and heights stay at least 1 % like
    // every box the editor makes. null for a path or a value that is not a number.
    function clampValue(clip, path, value) {
        const spec = Fx().paramSpec(clip, path), number = Number(value);
        if (!spec || spec.type !== 'number' || !Number.isFinite(number)) return null;
        return Math.max(path === 'width' || path === 'height' ? MIN_SIZE : spec.min, Math.min(spec.max, number));
    }
    // Writes a path's static value through the path store.
    function writeStatic(clip, path, value) {
        const fx = effectPath(path);
        if (fx) { const entry = clip.effects[fx.index]; entry.params = Object.assign(entry.params || {}, { [fx.param]: value }); return; }
        if (path.startsWith('crop.')) { clip.crop = Object.assign({ l: 0, t: 0, r: 0, b: 0 }, clip.crop || {}, { [path.slice(5)]: value }); return; }
        clip[path] = value;
    }
    // The clip an operation edits: a visual-track clip on an unlocked track.
    function editTarget(project, clipId) {
        const found = T().clipFor(project, clipId);
        if (!found) return REASON().nothing;
        if (found.track.locked) return REASON().locked;
        return found.track.kind === 'audio' ? REASON().kind : found;
    }
    // The fields must stay a valid box (x + width <= 1, y + height <= 1) even while keys drive the picture, because the
    // server validates them; keyframe values are checked per parameter only (index §4.18). On an axis that breaks the
    // rule, the field the clip does not show gives way: an animated size's, else an animated position's; with both
    // fields static, the value just written is clamped against the other (written: paths written this call). The crop
    // follows the same rule (settleCrop).
    function settleStatic(clip, written) {
        AXES.forEach(([pos, size]) => {
            if (!(Number(clip[pos]) + Number(clip[size]) > 1)) return;
            const shrinkSize = isAnimated(clip, size) || (!isAnimated(clip, pos) && written.has(size) && !written.has(pos));
            if (shrinkSize) clip[size] = Math.max(MIN_SIZE, 1 - clip[pos]);
            if (clip[pos] + clip[size] > 1) clip[pos] = Math.max(0, 1 - clip[size]);
            while (clip[pos] > 0 && clip[pos] + clip[size] > 1) clip[pos] = Math.max(0, clip[pos] - 1e-9);
        });
        settleCrop(clip, written);
    }
    // The crop fields must stay a valid crop (l + r <= 0.95, t + b <= 0.95; keyframe values are checked per side only).
    // On a pair that breaks the rule, an animated side's field gives way; with both sides static, the side written alone
    // this call is clamped against the other, else the second side (r, b). A crop left with four zeros is removed.
    function settleCrop(clip, written) {
        const crop = clip.crop;
        if (!crop) return;
        CROP_AXES.forEach(([a, b]) => {
            if (!(crop[a] + crop[b] > CROP_MAX)) return;
            const giveA = !isAnimated(clip, 'crop.' + b) && (isAnimated(clip, 'crop.' + a) || (written.has('crop.' + a) && !written.has('crop.' + b)));
            const give = giveA ? a : b, keep = giveA ? b : a;
            crop[give] = Math.max(0, Math.round((CROP_MAX - crop[keep]) * 1e6) / 1e6);
            while (crop[give] > 0 && crop[keep] + crop[give] > CROP_MAX) crop[give] = Math.max(0, crop[give] - 1e-9);
        });
        if (CROP_SIDES.every(side => !crop[side])) delete clip.crop;
    }
    // Puts a key at source frame f. An existing key takes the value (and the easing e when given); a new key takes e, else
    // the easing of the key before it, else linear (index §4.18). False when the limits would be passed.
    function putKey(clip, path, f, v, e) {
        const list = listOf(clip, path), at = list.findIndex(key => key.f === f);
        if (at >= 0) { list[at] = { f, v, e: e || list[at].e }; return true; }
        const limits = Fx().LIMITS;
        if (list.length >= limits.keyframesPerParam || keyCount(clip) >= limits.keyframesPerClip) return false;
        const before = list.filter(key => key.f < f).pop();
        setList(clip, path, list.concat({ f, v, e: e || (before ? before.e : 'linear') }).sort((a, b) => a.f - b.f));
        return true;
    }
    // Ends the animation of path: the keys go and the field takes value, clamped to a valid box and crop.
    function endAnimation(clip, path, value) {
        setList(clip, path, []);
        const number = clampValue(clip, path, value);
        if (number !== null) writeStatic(clip, path, number);
        settleStatic(clip, new Set([path]));
    }
    // Auto-key of {path: value} on a clip (setValues, setEffectParams, setCrop); every path must be animatable there.
    // Animated values get a key at the playhead (inside the clip; a new key takes the easing of the key before it), the
    // others change their field; the box and crop fields stay valid.
    function applyValues(clip, values, frame) {
        const entries = Object.entries(values || {}), written = new Set();
        if (!entries.length || entries.some(([path]) => !animatable(clip, path))) return REASON().nothing;
        for (const [path, raw] of entries) {
            const value = clampValue(clip, path, raw);
            if (value === null) return REASON().nothing;
            if (!isAnimated(clip, path)) { writeStatic(clip, path, value); written.add(path); continue; }
            if (!inside(clip, frame)) return REASON().nothing;
            if (!putKey(clip, path, keyFrame(clip, frame), value)) return REASON().limit;
        }
        settleStatic(clip, written);
        return true;
    }

    // The source frame a key set at timeline frame `frame` gets (index §4.18 content anchor).
    function keyFrame(clip, frame) { return Math.round(Fx().sourceAt(clip, frame)); }
    // The key of path the playhead stands on (stored at keyFrame(clip, frame)), or null.
    function atPlayhead(clip, path, frame) { const f = keyFrame(clip, frame); return listOf(clip, path).find(key => key.f === f) || null; }
    // The stopwatch. on: the value becomes animated with one key at the playhead (inside the clip) holding the value shown
    // there. off: the keys go and the field takes the value shown at the playhead, clamped to a valid box and crop.
    function setAnimated(project, clipId, path, on, frame) {
        const found = editTarget(project, clipId);
        if (typeof found === 'string') return found;
        const clip = found.clip;
        if (!animatable(clip, path) || isAnimated(clip, path) === !!on) return REASON().nothing;
        if (!on) { endAnimation(clip, path, Fx().valueAt(clip, path, frame, project)); return true; }
        if (!inside(clip, frame)) return REASON().nothing;
        return putKey(clip, path, keyFrame(clip, frame), clampValue(clip, path, Fx().valueAt(clip, path, frame, project))) ? true : REASON().limit;
    }
    // The diamond: removes the key at the playhead (the last one ends the animation, keeping its value) or adds one holding
    // the value shown; on a static value it starts the animation like the stopwatch. Inside the clip only.
    function toggleKeyframe(project, clipId, path, frame) {
        const found = editTarget(project, clipId);
        if (typeof found === 'string') return found;
        const clip = found.clip;
        if (!animatable(clip, path) || !inside(clip, frame)) return REASON().nothing;
        if (!isAnimated(clip, path)) return setAnimated(project, clipId, path, true, frame);
        const f = keyFrame(clip, frame), list = listOf(clip, path);
        if (list.some(key => key.f === f)) {
            if (list.length === 1) endAnimation(clip, path, list[0].v);
            else setList(clip, path, list.filter(key => key.f !== f));
            return true;
        }
        return putKey(clip, path, f, clampValue(clip, path, Fx().valueAt(clip, path, frame, project))) ? true : REASON().limit;
    }
    // Auto-key (spec §8.2): values = {path: value} of animatable values (base, crop sides, effect params). Animated values
    // get a key at the playhead, the others change their field; the box and crop fields stay valid. The caller passes a
    // box that is valid at the playhead (the UI clamps it).
    function setValues(project, clipId, values, frame) {
        const found = editTarget(project, clipId);
        return typeof found === 'string' ? found : applyValues(found.clip, values, frame);
    }
    // ◀▶: the whole timeline frame of the previous (direction < 0) or next key of any listed value, among the keys whose
    // timeline position lies inside the clip; null when there is none.
    function neighbor(clip, paths, frame, direction) {
        const first = clip.start, last = clip.start + clip.duration - 1;
        let best = null;
        (paths || []).forEach(path => listOf(clip, path).forEach(key => {
            const at = Math.round(Fx().timelineAt(clip, key.f));
            if (at < first || at > last) return;
            if (direction < 0 ? at < frame && (best === null || at > best) : at > frame && (best === null || at < best)) best = at;
        }));
        return best;
    }
    // Moves keys by delta whole timeline frames, each from its own timeline position through the clip's time map. A key
    // may pass another one; landing on another key of its value refuses (bounds).
    function moveKeys(project, clipId, keys, delta) {
        const found = editTarget(project, clipId);
        if (typeof found === 'string') return found;
        const clip = found.clip, wanted = keySet(keys), d = Math.round(Number(delta) || 0);
        if (!d || !wanted.size) return REASON().nothing;
        let changed = false;
        for (const path of keyedPaths(clip)) {
            const next = listOf(clip, path).map(key => {
                if (!wanted.has(keyId(path, key.f))) return key;
                const f = Math.round(Fx().sourceAt(clip, Fx().timelineAt(clip, key.f) + d));
                if (f !== key.f) changed = true;
                return { f, v: key.v, e: key.e };
            }).sort((a, b) => a.f - b.f);
            if (next.some((key, i) => i > 0 && key.f === next[i - 1].f)) return REASON().bounds;
            setList(clip, path, next);
        }
        return changed ? true : REASON().nothing;
    }
    // Removes keys. A value left without keys ends its animation; its field takes the value shown at the clip's first
    // frame (clamped to a valid box and crop).
    function removeKeys(project, clipId, keys) {
        const found = editTarget(project, clipId);
        if (typeof found === 'string') return found;
        const clip = found.clip, wanted = keySet(keys);
        let removed = 0;
        keyedPaths(clip).forEach(path => {
            const list = listOf(clip, path), rest = list.filter(key => !wanted.has(keyId(path, key.f)));
            removed += list.length - rest.length;
            if (rest.length === list.length) return;
            if (rest.length) setList(clip, path, rest);
            else endAnimation(clip, path, Fx().sample(list, Fx().sourceAt(clip, clip.start)));
        });
        return removed ? true : REASON().nothing;
    }
    // Sets the easing of keys (one of VideoStudioEffects.EASINGS); nothing when no key changes.
    function setEasing(project, clipId, keys, easing) {
        const found = editTarget(project, clipId);
        if (typeof found === 'string') return found;
        if (!Fx().EASINGS.includes(easing)) return REASON().nothing;
        const clip = found.clip, wanted = keySet(keys);
        let changed = false;
        keyedPaths(clip).forEach(path => listOf(clip, path).forEach((key, i, list) => {
            if (wanted.has(keyId(path, key.f)) && key.e !== easing) { list[i] = { f: key.f, v: key.v, e: easing }; changed = true; }
        }));
        return changed ? true : REASON().nothing;
    }
    // Clipboard data for keys: {keys: [{path, dt, v, e}]} sorted by dt, with dt the key's timeline position minus `frame`
    // (fractional where the time map is). pasteKeys at frame F puts each key at timeline F + dt. null without keys.
    function copyKeys(clip, keys, frame) {
        const wanted = keySet(keys), out = [];
        keyedPaths(clip).forEach(path => listOf(clip, path).forEach(key => {
            if (wanted.has(keyId(path, key.f))) out.push({ path, dt: Fx().timelineAt(clip, key.f) - frame, v: key.v, e: key.e });
        }));
        out.sort((a, b) => a.dt - b.dt || (a.path < b.path ? -1 : a.path > b.path ? 1 : 0));
        return out.length ? { keys: out } : null;
    }
    // Pastes clipboard keys onto a clip at timeline frame `frame`: each lands at round(sourceAt(clip, frame + dt)) with its
    // value and easing, replacing a key on that frame; two board keys of one value on one frame keep the first. A value
    // without keys becomes animated. Paths the clip cannot animate (an effect index it does not have) are skipped.
    function pasteKeys(project, clipId, board, frame) {
        const found = editTarget(project, clipId);
        if (typeof found === 'string') return found;
        const clip = found.clip, done = new Set();
        for (const item of (board && Array.isArray(board.keys) ? board.keys : [])) {
            const value = animatable(clip, item.path) ? clampValue(clip, item.path, item.v) : null;
            if (value === null) continue;
            const f = Math.round(Fx().sourceAt(clip, frame + Number(item.dt)));
            if (done.has(keyId(item.path, f))) continue;
            done.add(keyId(item.path, f));
            if (!putKey(clip, item.path, f, value, Fx().EASINGS.includes(item.e) ? item.e : 'linear')) return REASON().limit;
        }
        return done.size ? true : REASON().nothing;
    }

    // ---- Speed ramps (2b, index §4.18). Ramp points are clip time: timeline frames from the clip start. Like the
    // keyframe operations above, every operation returns true or a VideoStudioEdit.REASON. After a refusal the caller
    // discards the project (applyEdit restores its snapshot).
    const RAMP_PRESETS = Object.freeze(['ramp_up', 'ramp_down', 'slow_middle', 'custom']);
    function rampClamp(speed) { return Math.min(16, Math.max(0.25, Math.round(Number(speed) * 1000) / 1000)); }
    // The points of a preset for a clip of `duration` frames at the constant speed `speed`; null for 'none' or an
    // unknown name.
    // - Ramp up eases from the speed to twice the speed; ramp down is the reverse.
    // - Slow middle drops to a quarter of the speed between 40 % and 60 % of the clip.
    // - Custom is flat, for the speed lane.
    // Points stay strictly increasing even for one-frame clips.
    function rampPreset(name, duration, speed) {
        const s0 = rampClamp(Number(speed) > 0 ? speed : 1), last = Math.max(1, Math.round(Number(duration) || 1) - 1);
        const point = (f, value, e) => ({ f, speed: rampClamp(value), e });
        if (name === 'ramp_up') return [point(0, s0, 'ease_in_out'), point(last, 2 * s0, 'linear')];
        if (name === 'ramp_down') return [point(0, 2 * s0, 'ease_in_out'), point(last, s0, 'linear')];
        if (name === 'custom') return [point(0, s0, 'linear'), point(last, s0, 'linear')];
        if (name !== 'slow_middle') return null;
        const a = Math.max(1, Math.round(last * 0.4)), b = Math.max(a + 1, Math.round(last * 0.6)), end = Math.max(b + 1, last);
        return [point(0, s0, 'ease_in_out'), point(a, s0 / 4, 'linear'), point(b, s0 / 4, 'ease_in_out'), point(end, s0, 'linear')];
    }
    // A clip that may hold a ramp: on an unlocked video track, with a video asset. Returns {track, clip, asset} or a
    // REASON.
    function rampTarget(project, clipId) {
        const E = window.VideoStudioEdit, found = T().clipFor(project, clipId);
        if (!found) return E.REASON.nothing;
        if (found.track.locked) return E.REASON.locked;
        const asset = (project.assets || []).find(item => item.id === found.clip.asset_id);
        if (found.track.kind !== 'video' || !asset || asset.kind !== 'video' || found.clip.text) return E.REASON.media;
        return { track: found.track, clip: found.clip, asset };
    }
    // The ramp-aware source extent rule: offset + S(duration - 1) <= duration_frames - 1.
    function rampFits(clip, asset) { return clip.offset + Fx().sourceExtent(clip) <= Number(asset.duration_frames) - 1 + 1e-9; }
    // Replaces the clip's ramp and keeps its duration. The points are sorted, f is rounded, speeds are clamped and
    // unknown easings become linear. Fewer than two points remove the ramp. The media must hold the new source range.
    function setRamp(project, clipId, points) {
        const E = window.VideoStudioEdit, target = rampTarget(project, clipId);
        if (typeof target === 'string') return target;
        const list = (points || []).map(point => ({ f: Math.round(Number(point.f)), speed: rampClamp(point.speed), e: Fx().EASINGS.includes(point.e) ? point.e : 'linear' })).sort((a, b) => a.f - b.f);
        if (list.length > Fx().LIMITS.rampPoints) return E.REASON.limit;
        if (list.some((point, index) => !Number.isSafeInteger(point.f) || !Number.isFinite(point.speed) || (index > 0 && point.f <= list[index - 1].f))) return E.REASON.bounds;
        const { track, clip, asset } = target;
        if (list.length < 2) delete clip.ramp;
        else { clip.ramp = list; delete clip.speed; }
        if (!rampFits(clip, asset)) return E.REASON.media;
        return Fx().validAnimation(clip, track, asset, window.VideoStudioTime.maxFrames(project)) ? true : E.REASON.bounds;
    }
    // Adds a point at clip frame f, at the speed there when `speed` is omitted.
    // - The first point turns a constant speed into a flat ramp first.
    // - A point already at f takes the new speed.
    // - A new point takes the easing of the point before it.
    function addRampPoint(project, clipId, f, speed) {
        const target = rampTarget(project, clipId);
        if (typeof target === 'string') return target;
        const clip = target.clip, at = Math.round(Number(f)), s0 = window.VideoStudioEdit.speedOf(clip);
        const points = Fx().hasRamp(clip) ? clip.ramp.map(point => Object.assign({}, point)) : [{ f: 0, speed: s0, e: 'linear' }, { f: Math.max(1, clip.duration - 1), speed: s0, e: 'linear' }];
        const value = speed == null ? Fx().speedAt(clip, at) : speed;
        const same = points.find(point => point.f === at);
        if (same) same.speed = value;
        else { const before = points.filter(point => point.f < at).pop(); points.push({ f: at, speed: value, e: before ? before.e : 'linear' }); }
        return setRamp(project, clipId, points);
    }
    // Moves point `index` to clip frame f with `speed` (kept when omitted). A point never passes its neighbours.
    function moveRampPoint(project, clipId, index, f, speed) {
        const E = window.VideoStudioEdit, target = rampTarget(project, clipId);
        if (typeof target === 'string') return target;
        const ramp = Fx().hasRamp(target.clip) ? target.clip.ramp : null;
        if (!ramp || !ramp[index]) return E.REASON.nothing;
        const at = Math.round(Number(f));
        if ((index > 0 && at <= ramp[index - 1].f) || (index < ramp.length - 1 && at >= ramp[index + 1].f)) return E.REASON.bounds;
        return setRamp(project, clipId, ramp.map((point, i) => (i === index ? { f: at, speed: speed == null ? point.speed : speed, e: point.e } : Object.assign({}, point))));
    }
    // Removes one point. When only one point would be left, the clip plays at that point's constant speed instead.
    function removeRampPoint(project, clipId, index) {
        const E = window.VideoStudioEdit, target = rampTarget(project, clipId);
        if (typeof target === 'string') return target;
        const { clip, asset } = target, ramp = Fx().hasRamp(clip) ? clip.ramp : null;
        if (!ramp || !ramp[index]) return E.REASON.nothing;
        const rest = ramp.filter((_, i) => i !== index);
        if (rest.length >= 2) return setRamp(project, clipId, rest);
        delete clip.ramp;
        if (rest[0].speed === 1) delete clip.speed; else clip.speed = rest[0].speed;
        return rampFits(clip, asset) ? true : E.REASON.media;
    }
    // Animation section presets for the listed clips that take a ramp; the others are skipped.
    // - Default: keep each clip's source range, like setSpeed. The new duration is the one whose ramped extent is
    //   closest to the old extent; extent grows with duration, so a bisection finds it, and the media must hold it.
    //   Later clips on the clip's track move by the difference.
    // - keepDuration keeps the length; the media must hold the new range.
    // - Presets start from the clip's speed, which is 1 once it has a ramp, so applying a preset twice gives the same
    //   ramp.
    // - 'none' is setSpeed(1).
    function applyRampPreset(project, ids, preset, keepDuration) {
        const E = window.VideoStudioEdit, Time = window.VideoStudioTime;
        if (preset === 'none') return E.setSpeed(project, ids, 1, !!keepDuration);
        if (!RAMP_PRESETS.includes(preset)) return E.REASON.nothing;
        const items = E.editable(project, ids);
        if (!items.length) return (project.tracks || []).some(track => track.locked && track.clips.some(clip => (ids || []).includes(clip.id))) ? E.REASON.locked : E.REASON.nothing;
        const limit = Time.maxFrames(project), plans = [];
        for (const { track, clip } of items) {
            const target = rampTarget(project, clip.id);
            if (typeof target === 'string') continue;
            const speed = E.speedOf(clip), asset = target.asset;
            const shaped = frames => Object.assign({}, clip, { duration: frames, ramp: rampPreset(preset, frames, speed) });
            let duration = clip.duration;
            if (!keepDuration) {
                const goal = Fx().sourceExtent(clip), extent = frames => Fx().sourceExtent(shaped(frames));
                let lo = 1, hi = Math.max(1, limit - clip.start);
                while (lo < hi) { const mid = Math.ceil((lo + hi) / 2); if (extent(mid) <= goal + 1e-9) lo = mid; else hi = mid - 1; }
                duration = lo;
                if (lo + 1 <= limit - clip.start && Math.abs(extent(lo + 1) - goal) < Math.abs(extent(lo) - goal) && rampFits(shaped(lo + 1), asset)) duration = lo + 1;
            }
            const result = shaped(duration);
            if (!rampFits(result, asset)) return E.REASON.media;
            plans.push({ track, clip, duration, ramp: result.ramp });
        }
        if (!plans.length) return E.REASON.media;
        plans.forEach(({ track, clip, duration, ramp }) => {
            const delta = duration - clip.duration;
            if (delta) track.clips.filter(item => item !== clip && item.start > clip.start).forEach(item => { item.start += delta; });
            clip.duration = duration;
            clip.ramp = ramp;
            delete clip.speed;
            clip.fade_in = Math.min(Number(clip.fade_in) || 0, duration);
            clip.fade_out = Math.min(Number(clip.fade_out) || 0, duration);
        });
        if (T().projectEnd(project) > limit) return E.REASON.limit;
        T().repairTransitions(project);
        return T().validTimeline(project) ? true : E.REASON.bounds;
    }

    // ---- Effect stack (2c; spec §5.4, §8.1). Entries are {id, on, params, keyframes?} in processing order, at most
    // LIMITS.effectsPerClip per visual-track clip; params are always complete (the editor never relies on registry
    // defaults, index §4.1), and entry keys stay inside the entry, so moving or removing an entry needs no key remapping.
    // Each operation returns true or a REASON: nothing (unknown id, index or param, or no change), kind (audio clip),
    // locked, limit (a full stack or the key limits), media (a LUT the project does not hold).
    function entryAt(found, index) {
        const list = Array.isArray(found.clip.effects) ? found.clip.effects : [];
        return Number.isInteger(index) && index >= 0 && index < list.length ? list[index] : null;
    }
    // A lut param value: '' (pass-through), builtin:<registry look id>, or the id of a project asset of kind lut.
    function lutRefKnown(project, ref) {
        if (ref === '') return true;
        if (typeof ref !== 'string') return false;
        if (ref.startsWith('builtin:')) return Fx().looks().some(look => 'builtin:' + look.id === ref);
        return (project.assets || []).some(asset => asset.id === ref && asset.kind === 'lut');
    }
    // Writes {param: value} into an entry's params as static values: numbers clamped to the registry range, LUT refs the
    // project knows (media otherwise); an unknown param or a value of the wrong kind refuses (nothing).
    function writeParams(project, entry, spec, values) {
        for (const [id, raw] of Object.entries(values || {})) {
            const param = (spec.params || []).find(item => item.id === id);
            if (!param) return REASON().nothing;
            if (param.type === 'lut') {
                if (!lutRefKnown(project, raw)) return REASON().media;
                entry.params[id] = raw;
                continue;
            }
            const number = Number(raw);
            if (param.type !== 'number' || raw === null || raw === '' || !Number.isFinite(number)) return REASON().nothing;
            entry.params[id] = Math.max(param.min, Math.min(param.max, number));
        }
        return true;
    }
    // Adds an entry of registry effect effectId at the end of the stack: on, every param at its registry default, and
    // params on top (the library's look items set lut directly).
    function addEffect(project, clipId, effectId, params) {
        const found = editTarget(project, clipId);
        if (typeof found === 'string') return found;
        const spec = Fx().effect(effectId), clip = found.clip, list = Array.isArray(clip.effects) ? clip.effects : [];
        if (!spec) return REASON().nothing;
        if (list.length >= Fx().LIMITS.effectsPerClip) return REASON().limit;
        const entry = { id: spec.id, on: true, params: Fx().defaults(spec.id) };
        const result = writeParams(project, entry, spec, params);
        if (result !== true) return result;
        clip.effects = list.concat(entry);
        return true;
    }
    // Removes entry `index` with its keys; the last entry leaves no empty list.
    function removeEffect(project, clipId, index) {
        const found = editTarget(project, clipId);
        if (typeof found === 'string') return found;
        if (!entryAt(found, index)) return REASON().nothing;
        found.clip.effects.splice(index, 1);
        if (!found.clip.effects.length) delete found.clip.effects;
        return true;
    }
    // Moves entry `from` to stack position `to`; the entry keeps its params and keys.
    function moveEffect(project, clipId, from, to) {
        const found = editTarget(project, clipId);
        if (typeof found === 'string') return found;
        const list = found.clip.effects;
        if (!entryAt(found, from) || !Number.isInteger(to) || to < 0 || to >= list.length || to === from) return REASON().nothing;
        list.splice(to, 0, list.splice(from, 1)[0]);
        return true;
    }
    // The entry's on/off switch.
    function setEffectOn(project, clipId, index, on) {
        const found = editTarget(project, clipId);
        if (typeof found === 'string') return found;
        const entry = entryAt(found, index);
        if (!entry || !!entry.on === !!on) return REASON().nothing;
        entry.on = !!on;
        return true;
    }
    // Reset (spec §8.1): every number param back to its registry default and the entry's keys removed. The on state and
    // the LUT choice stay: the LUT is what a lut entry is, so only its intensity resets.
    function resetEffect(project, clipId, index) {
        const found = editTarget(project, clipId);
        if (typeof found === 'string') return found;
        const entry = entryAt(found, index), spec = entry ? Fx().effect(entry.id) : null;
        if (!spec) return REASON().nothing;
        const params = Fx().defaults(spec.id), current = entry.params || {};
        (spec.params || []).forEach(param => { if (param.type === 'lut' && typeof current[param.id] === 'string') params[param.id] = current[param.id]; });
        const same = Object.keys(Object.assign({}, params, current)).every(key => current[key] === params[key]);
        if (same && !entry.keyframes) return REASON().nothing;
        entry.params = params;
        delete entry.keyframes;
        return true;
    }
    // The inspector's param controls (spec §8.1, §8.2): values = {param: value} for entry `index`. A keyframable number
    // param is auto-keyed like setValues (a key at the playhead while it is animated, its field otherwise); every other
    // param (the blur radius, the LUT) changes its static value.
    function setEffectParams(project, clipId, index, values, frame) {
        const found = editTarget(project, clipId);
        if (typeof found === 'string') return found;
        const clip = found.clip, entry = entryAt(found, index), spec = entry ? Fx().effect(entry.id) : null, pairs = Object.entries(values || {});
        if (!spec || !pairs.length) return REASON().nothing;
        const keyed = {}, statics = {};
        pairs.forEach(([id, raw]) => { const path = 'effects.' + index + '.' + id; if (animatable(clip, path)) keyed[path] = raw; else statics[id] = raw; });
        entry.params = entry.params || {};
        const result = writeParams(project, entry, spec, statics);
        if (result !== true) return result;
        return Object.keys(keyed).length ? applyValues(clip, keyed, frame) : true;
    }
    // Crop fields and on-canvas handles (spec §5.3, §8.1): crop = {l, t, r, b} or any subset, fractions of the unflipped
    // source picture. Animated sides get a key at the playhead, the others their field; the fields stay a valid crop.
    function setCrop(project, clipId, crop, frame) {
        const sides = Object.keys(crop || {});
        if (!sides.length || sides.some(side => !CROP_SIDES.includes(side))) return REASON().nothing;
        const values = {};
        sides.forEach(side => { values['crop.' + side] = crop[side]; });
        return setValues(project, clipId, values, frame);
    }
    // Flip toggles (spec §5.3): axis 'h' or 'v'. flip_h and flip_v are stored only while true.
    function setFlip(project, clipId, axis, on) {
        const found = editTarget(project, clipId);
        if (typeof found === 'string') return found;
        const key = axis === 'h' ? 'flip_h' : axis === 'v' ? 'flip_v' : '';
        if (!key || !!found.clip[key] === !!on) return REASON().nothing;
        if (on) found.clip[key] = true; else delete found.clip[key];
        return true;
    }

    // keysOf(clip, path) returns the stored list (read only); keyedPaths(clip) every path holding keys (path store, 2c).
    window.VideoStudioKeyframeEdit = { keyFrame, atPlayhead, setAnimated, toggleKeyframe, setValues, neighbor, moveKeys, removeKeys, setEasing, copyKeys, pasteKeys, keysOf: listOf, keyedPaths, rampPreset, setRamp, applyRampPreset, addRampPoint, moveRampPoint, removeRampPoint, addEffect, removeEffect, moveEffect, setEffectOn, resetEffect, setEffectParams, setCrop, setFlip };
})();
