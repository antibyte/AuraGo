(function () {
    'use strict';

    // DOM-free keyframe operations (spec §5.1, §5.2, §8.2; plan index §4.18). Like VideoStudioEdit, an operation that
    // takes a project changes it in place and returns true or a VideoStudioEdit.REASON; after a refusal the caller
    // discards the project (the app runs them inside P.applyEdit, which restores its snapshot). Keys live on visual-track
    // clips at source frames: a key set at timeline frame T is stored at f = round(sourceAt(clip, T)), so it stays on its
    // picture content through every timing edit. A key is named {path, f}. Task 14 adds the ramp operations.
    const Fx = () => window.VideoStudioEffects;
    const T = () => window.VideoStudioTimeline;
    const REASON = () => window.VideoStudioEdit.REASON;
    const AXES = [['x', 'width'], ['y', 'height']];
    const MIN_SIZE = 0.01;

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

    window.VideoStudioKeyframeEdit = { keyFrame, atPlayhead, setAnimated, toggleKeyframe, setValues, neighbor, moveKeys, removeKeys, setEasing, copyKeys, pasteKeys };
})();
