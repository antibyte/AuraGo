(function () {
    'use strict';

    // DOM-free keyframe operations (spec §5.1, §5.2, §8.2; plan index §4.18). Like VideoStudioEdit, an operation that
    // takes a project changes it in place and returns true or a VideoStudioEdit.REASON; after a refusal the caller
    // discards the project (the app runs them inside P.applyEdit, which restores its snapshot). Keys live on visual-track
    // clips at source frames: a key set at timeline frame T is stored at f = round(sourceAt(clip, T)), so it stays on its
    // picture content through every timing edit. A key is named {path, f}. The speed ramp operations follow the key operations.
    const Fx = () => window.VideoStudioEffects;
    const T = () => window.VideoStudioTimeline;
    const REASON = () => window.VideoStudioEdit.REASON;
    const AXES = [['x', 'width'], ['y', 'height']];
    const MIN_SIZE = window.VideoStudioEffects.MIN_BOX_SIZE;

    const keyId = (path, f) => path + ':' + f;
    const keySet = keys => new Set((keys || []).map(key => keyId(key.path, Number(key.f))));
    const keyframable = path => Fx().keyframable().includes(path);
    function listOf(clip, path) { const list = clip && clip.keyframes && clip.keyframes[path]; return Array.isArray(list) ? list : []; }
    function isAnimated(clip, path) { return listOf(clip, path).length > 0; }
    function keyCount(clip) { return Object.keys((clip && clip.keyframes) || {}).reduce((n, path) => n + listOf(clip, path).length, 0); }
    function inside(clip, frame) { return frame >= clip.start && frame < clip.start + clip.duration; }
    // Empty lists and an empty map are never stored (canonical form, index §4.1).
    function dropEmpty(clip) {
        if (!clip.keyframes) return;
        Object.keys(clip.keyframes).forEach(path => { if (!listOf(clip, path).length) delete clip.keyframes[path]; });
        if (!Object.keys(clip.keyframes).length) delete clip.keyframes;
    }
    // A value inside its registry range; widths and heights stay at least 1 % like every box the editor makes.
    function clampValue(path, value) {
        const spec = Fx().base(path), number = Number(value);
        if (!spec || !Number.isFinite(number)) return null;
        return Math.max(path === 'width' || path === 'height' ? MIN_SIZE : spec.min, Math.min(spec.max, number));
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
    // fields static, the value just written is clamped against the other (written: paths written this call).
    function settleStatic(clip, written) {
        AXES.forEach(([pos, size]) => {
            if (!(Number(clip[pos]) + Number(clip[size]) > 1)) return;
            const shrinkSize = isAnimated(clip, size) || (!isAnimated(clip, pos) && written.has(size) && !written.has(pos));
            if (shrinkSize) clip[size] = Math.max(MIN_SIZE, 1 - clip[pos]);
            if (clip[pos] + clip[size] > 1) clip[pos] = Math.max(0, 1 - clip[size]);
            while (clip[pos] > 0 && clip[pos] + clip[size] > 1) clip[pos] = Math.max(0, clip[pos] - 1e-9);
        });
    }
    // Puts a key at source frame f. An existing key takes the value (and the easing e when given); a new key takes e, else
    // the easing of the key before it, else linear (index §4.18). False when the limits would be passed.
    function putKey(clip, path, f, v, e) {
        const list = listOf(clip, path), at = list.findIndex(key => key.f === f);
        if (at >= 0) { list[at] = { f, v, e: e || list[at].e }; return true; }
        const limits = Fx().LIMITS;
        if (list.length >= limits.keyframesPerParam || keyCount(clip) >= limits.keyframesPerClip) return false;
        const before = list.filter(key => key.f < f).pop();
        clip.keyframes = Object.assign(clip.keyframes || {}, { [path]: list.concat({ f, v, e: e || (before ? before.e : 'linear') }).sort((a, b) => a.f - b.f) });
        return true;
    }
    // Ends the animation of path: the keys go and the field takes value, clamped to a valid box.
    function endAnimation(clip, path, value) {
        if (clip.keyframes) delete clip.keyframes[path];
        dropEmpty(clip);
        clip[path] = clampValue(path, value);
        settleStatic(clip, new Set([path]));
    }

    // The source frame a key set at timeline frame `frame` gets (index §4.18 content anchor).
    function keyFrame(clip, frame) { return Math.round(Fx().sourceAt(clip, frame)); }
    // The key of path the playhead stands on (stored at keyFrame(clip, frame)), or null.
    function atPlayhead(clip, path, frame) { const f = keyFrame(clip, frame); return listOf(clip, path).find(key => key.f === f) || null; }
    // The stopwatch. on: the value becomes animated with one key at the playhead (inside the clip) holding the value shown
    // there. off: the keys go and the field takes the value shown at the playhead, clamped to a valid box.
    function setAnimated(project, clipId, path, on, frame) {
        const found = editTarget(project, clipId);
        if (typeof found === 'string') return found;
        const clip = found.clip;
        if (!keyframable(path) || isAnimated(clip, path) === !!on) return REASON().nothing;
        if (!on) { endAnimation(clip, path, Fx().valueAt(clip, path, frame, project)); return true; }
        if (!inside(clip, frame)) return REASON().nothing;
        return putKey(clip, path, keyFrame(clip, frame), clampValue(path, Fx().valueAt(clip, path, frame, project))) ? true : REASON().limit;
    }
    // The diamond: removes the key at the playhead (the last one ends the animation, keeping its value) or adds one holding
    // the value shown; on a static value it starts the animation like the stopwatch. Inside the clip only.
    function toggleKeyframe(project, clipId, path, frame) {
        const found = editTarget(project, clipId);
        if (typeof found === 'string') return found;
        const clip = found.clip;
        if (!keyframable(path) || !inside(clip, frame)) return REASON().nothing;
        if (!isAnimated(clip, path)) return setAnimated(project, clipId, path, true, frame);
        const f = keyFrame(clip, frame), list = listOf(clip, path);
        if (list.some(key => key.f === f)) {
            if (list.length === 1) endAnimation(clip, path, list[0].v);
            else clip.keyframes[path] = list.filter(key => key.f !== f);
            return true;
        }
        return putKey(clip, path, f, clampValue(path, Fx().valueAt(clip, path, frame, project))) ? true : REASON().limit;
    }
    // Auto-key (spec §8.2): values = {path: value} of keyframable values. Animated values get a key at the playhead (inside
    // the clip; a new key takes the easing of the key before it), the others change their field; the fields stay a valid
    // box. The caller passes a box that is valid at the playhead (the UI clamps it).
    function setValues(project, clipId, values, frame) {
        const found = editTarget(project, clipId);
        if (typeof found === 'string') return found;
        const clip = found.clip, entries = Object.entries(values || {}), written = new Set();
        if (!entries.length || entries.some(([path]) => !keyframable(path))) return REASON().nothing;
        for (const [path, raw] of entries) {
            const value = clampValue(path, raw);
            if (value === null) return REASON().nothing;
            if (!isAnimated(clip, path)) { clip[path] = value; written.add(path); continue; }
            if (!inside(clip, frame)) return REASON().nothing;
            if (!putKey(clip, path, keyFrame(clip, frame), value)) return REASON().limit;
        }
        settleStatic(clip, written);
        return true;
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
        for (const path of Object.keys(clip.keyframes || {})) {
            const next = listOf(clip, path).map(key => {
                if (!wanted.has(keyId(path, key.f))) return key;
                const f = Math.round(Fx().sourceAt(clip, Fx().timelineAt(clip, key.f) + d));
                if (f !== key.f) changed = true;
                return { f, v: key.v, e: key.e };
            }).sort((a, b) => a.f - b.f);
            if (next.some((key, i) => i > 0 && key.f === next[i - 1].f)) return REASON().bounds;
            clip.keyframes[path] = next;
        }
        return changed ? true : REASON().nothing;
    }
    // Removes keys. A value left without keys ends its animation; its field takes the value shown at the clip's first
    // frame (clamped to a valid box).
    function removeKeys(project, clipId, keys) {
        const found = editTarget(project, clipId);
        if (typeof found === 'string') return found;
        const clip = found.clip, wanted = keySet(keys);
        let removed = 0;
        Object.keys(clip.keyframes || {}).forEach(path => {
            const list = listOf(clip, path), rest = list.filter(key => !wanted.has(keyId(path, key.f)));
            removed += list.length - rest.length;
            if (rest.length === list.length) return;
            if (rest.length) clip.keyframes[path] = rest;
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
        Object.keys(clip.keyframes || {}).forEach(path => listOf(clip, path).forEach((key, i, list) => {
            if (wanted.has(keyId(path, key.f)) && key.e !== easing) { list[i] = { f: key.f, v: key.v, e: easing }; changed = true; }
        }));
        return changed ? true : REASON().nothing;
    }
    // Clipboard data for keys: {keys: [{path, dt, v, e}]} sorted by dt, with dt the key's timeline position minus `frame`
    // (fractional where the time map is). pasteKeys at frame F puts each key at timeline F + dt. null without keys.
    function copyKeys(clip, keys, frame) {
        const wanted = keySet(keys), out = [];
        Object.keys((clip && clip.keyframes) || {}).sort().forEach(path => listOf(clip, path).forEach(key => {
            if (wanted.has(keyId(path, key.f))) out.push({ path, dt: Fx().timelineAt(clip, key.f) - frame, v: key.v, e: key.e });
        }));
        out.sort((a, b) => a.dt - b.dt || (a.path < b.path ? -1 : a.path > b.path ? 1 : 0));
        return out.length ? { keys: out } : null;
    }
    // Pastes clipboard keys onto a clip at timeline frame `frame`: each lands at round(sourceAt(clip, frame + dt)) with its
    // value and easing, replacing a key on that frame; two board keys of one value on one frame keep the first. A value
    // without keys becomes animated. Unknown values are skipped.
    function pasteKeys(project, clipId, board, frame) {
        const found = editTarget(project, clipId);
        if (typeof found === 'string') return found;
        const clip = found.clip, done = new Set();
        for (const item of (board && Array.isArray(board.keys) ? board.keys : [])) {
            const value = keyframable(item.path) ? clampValue(item.path, item.v) : null;
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

    window.VideoStudioKeyframeEdit = { keyFrame, atPlayhead, setAnimated, toggleKeyframe, setValues, neighbor, moveKeys, removeKeys, setEasing, copyKeys, pasteKeys, rampPreset, setRamp, applyRampPreset, addRampPoint, moveRampPoint, removeRampPoint };
})();
