(function () {
    'use strict';

    // DOM-free edit operations (spec §4.2). Each operation changes the given project in place and returns
    // true, or a REASON code; after a refusal the caller discards the project (the app runs operations
    // inside mutate, which restores its snapshot). Locked tracks never change.
    const T = () => window.VideoStudioTimeline;
    const Time = () => window.VideoStudioTime;
    const FX = () => window.VideoStudioEffects;
    const REASON = Object.freeze({ locked: 'locked', media: 'media', bounds: 'bounds', nothing: 'nothing', limit: 'limit', kind: 'kind', transition: 'transition', tracks: 'tracks', link: 'link', room: 'room', ramp: 'ramp' });
    const maxLinkClips = () => T().MAX_LINK_CLIPS;
    const ATTRIBUTE_GROUPS = { picture: ['x', 'y', 'width', 'height', 'rotation', 'opacity', 'fit'], sound: ['volume', 'fade_in', 'fade_out'], speed: ['speed'], keyframes: ['keyframes'] };
    const clone = value => JSON.parse(JSON.stringify(value));
    const endOf = clip => clip.start + clip.duration;
    const sorted = track => track.clips.slice().sort((a, b) => a.start - b.start);

    function speedOf(clip) { const speed = Number(clip && clip.speed); return speed > 0 ? speed : 1; }
    function assetOf(project, clip) { return (project.assets || []).find(asset => asset.id === clip.asset_id) || null; }
    function isStill(project, clip) { const asset = assetOf(project, clip); return !asset || asset.kind === 'image'; }
    function trackById(project, id) { return (project.tracks || []).find(track => track.id === id) || null; }
    function clipCount(project) { return project.tracks.reduce((n, track) => n + track.clips.length, 0); }
    function clampFades(clip) {
        clip.fade_in = Math.min(Number(clip.fade_in) || 0, clip.duration);
        clip.fade_out = Math.min(Number(clip.fade_out) || 0, clip.duration);
    }
    // Keyframe limits (index §4.18): 64 per value, 512 per clip.
    function overLimit(keyframes) {
        const limits = FX().LIMITS, lists = Object.values(keyframes || {});
        return lists.some(list => list.length > limits.keyframesPerParam) || lists.reduce((n, list) => n + list.length, 0) > limits.keyframesPerClip;
    }
    // A box evaluated from keys, made valid as fields: sizes within 0.01..1, positions pulled in so x + width <= 1 and
    // y + height <= 1 (keyframe values are checked per parameter only).
    function fieldBox(state) {
        const axis = (position, size) => { let p = Math.max(0, Math.min(Number(position) || 0, 1 - size)); while (p > 0 && p + size > 1) p = Math.max(0, p - 1e-9); return p; };
        const width = Math.max(0.01, Math.min(1, Number(state.width) || 1)), height = Math.max(0.01, Math.min(1, Number(state.height) || 1));
        return { x: axis(state.x, width), y: axis(state.y, height), width, height };
    }
    // Source extent rule: offset + clipSource(duration - 1) <= duration_frames - 1. Without a ramp that is phase 1's
    // (duration - 1) * speed; with one it is S (VideoStudioEffects.maxDurationFor). Stills are unbounded.
    function maxDurationFrom(project, clip, offset) {
        if (isStill(project, clip)) return Infinity;
        const frames = Number(assetOf(project, clip).duration_frames) || 0;
        if (FX().hasRamp(clip)) return FX().maxDurationFor(clip, frames, offset);
        return Math.floor((frames - 1 - offset) / speedOf(clip) + 1e-9) + 1;
    }
    // Moves the clip's first frame by `frames` timeline frames in place (VideoStudioEffects.headShift). The source the
    // head passes goes into the offset, rounded down so that the last source frame stays inside the media. Ramp points
    // (clip time) shift with the head, so the curve stays on the same timeline frames.
    function moveHead(clip, frames) { const head = FX().headShift(clip, frames); clip.offset = head.offset; if (head.ramp) clip.ramp = head.ramp; }
    // Timeline frames the head can grow before the source runs out (a ramp continues its curve before the clip).
    function headroom(project, clip) { return isStill(project, clip) ? Infinity : FX().hasRamp(clip) ? FX().headroom(clip) : Math.floor(clip.offset / speedOf(clip) + 1e-9); }
    // A clip never gets shorter than one frame, than a transition it shares with a neighbor, or than its
    // incoming and outgoing transitions together (they may touch but must not overlap).
    function minDuration(track, clip) {
        const order = sorted(track), prev = order[order.indexOf(clip) - 1];
        const incoming = prev && prev.transition ? endOf(prev) - clip.start : 0;
        const outgoing = clip.transition ? clip.transition.duration : 0;
        return Math.max(1, incoming + 1, outgoing + 1, incoming + outgoing);
    }
    // The listed clips that sit on unlocked tracks.
    function editable(project, ids) {
        const wanted = new Set(ids || []), found = [];
        (project.tracks || []).forEach(track => {
            if (!track.locked) track.clips.forEach(clip => { if (wanted.has(clip.id)) found.push({ track, clip }); });
        });
        return found;
    }
    function refusal(project, ids) {
        const wanted = new Set(ids || []);
        return (project.tracks || []).some(track => track.locked && track.clips.some(clip => wanted.has(clip.id))) ? REASON.locked : REASON.nothing;
    }
    // Go's MaxLinkClips: an edit never leaves a link group with more clips than the server accepts.
    function linkLimitExceeded(project) {
        const counts = new Map();
        for (const track of project.tracks) {
            for (const clip of track.clips) {
                if (!clip.link) continue;
                counts.set(clip.link, (counts.get(clip.link) || 0) + 1);
                if (counts.get(clip.link) > maxLinkClips()) return true;
            }
        }
        return false;
    }
    function finish(project) {
        T().repairTransitions(project);
        if (clipCount(project) > T().MAX_CLIPS) return REASON.limit;
        return linkLimitExceeded(project) ? REASON.link : true;
    }
    // A cut at `frame` would separate a transition from its clips.
    function isCutInTransition(track, frame) {
        const order = sorted(track);
        return order.some((clip, index) => clip.transition && order[index + 1] && order[index + 1].start <= frame && frame <= endOf(clip));
    }
    function merge(ranges) {
        const out = [];
        ranges.filter(([a, b]) => b > a).sort((x, y) => x[0] - y[0]).forEach(([a, b]) => {
            const last = out[out.length - 1];
            if (last && a <= last[1]) last[1] = Math.max(last[1], b); else out.push([a, b]);
        });
        return out;
    }
    function subtract(ranges, a, b) {
        return ranges.flatMap(([x, y]) => (b <= x || a >= y ? [[x, y]] : [x < a ? [x, a] : null, b < y ? [b, y] : null].filter(Boolean)));
    }
    // Closes the parts of `ranges` that are empty on every one of `tracks`: later clips on those tracks move
    // left by the closed length before them, so the tracks stay in sync with each other.
    function closeRanges(tracks, ranges) {
        let cuts = merge(ranges);
        tracks.forEach(track => track.clips.forEach(clip => { cuts = subtract(cuts, clip.start, endOf(clip)); }));
        tracks.forEach(track => track.clips.forEach(clip => {
            clip.start -= cuts.reduce((n, [a, b]) => n + (b <= clip.start ? b - a : 0), 0);
        }));
    }
    // Copy of `clip` limited to [from, to): the head keeps the fade in, the tail keeps the fade out and the
    // outgoing transition.
    function piece(project, clip, from, to, id) {
        const copy = clone(clip);
        copy.id = id; copy.start = from; copy.duration = to - from;
        if (!isStill(project, clip)) moveHead(copy, from - clip.start);
        copy.fade_in = from === clip.start ? Math.min(Number(clip.fade_in) || 0, copy.duration) : 0;
        copy.fade_out = to === endOf(clip) ? Math.min(Number(clip.fade_out) || 0, copy.duration) : 0;
        if (to !== endOf(clip)) copy.transition = null;
        return copy;
    }
    // Blade rule for every split (blade, overwrite, insert): the left part keeps id and link; the right parts that one
    // operation splits off a group at the same frame share one new link id, and a lone right part is unlinked. So a
    // split never grows a group. `rights` collects right-part ids per (link, frame) until relinkRights runs once at the
    // end of the operation (clearRange replaces clip objects, so ids are looked up again).
    function noteRight(rights, clip, right) {
        if (!rights || !clip.link) return;
        const key = JSON.stringify([clip.link, right.start]);
        if (!rights.has(key)) rights.set(key, []);
        rights.get(key).push(right.id);
    }
    function relinkRights(project, rights) {
        rights.forEach(ids => {
            const parts = ids.map(id => T().clipFor(project, id)).filter(Boolean).map(({ clip }) => clip);
            const link = parts.length > 1 ? T().newId('link') : '';
            parts.forEach(clip => { if (link) clip.link = link; else delete clip.link; });
        });
    }
    function splitAt(project, track, frame, rights) {
        track.clips.slice().forEach(clip => {
            if (clip.start >= frame || endOf(clip) <= frame) return;
            const right = piece(project, clip, frame, endOf(clip), T().newId('clip'));
            noteRight(rights, clip, right);
            Object.assign(clip, piece(project, clip, clip.start, frame, clip.id));
            track.clips.push(right);
        });
    }
    // After a clear a clip may still reach into the next one. Every clip ends up following validTimeline's rules: a
    // transition keeps the overlap only while it is shorter than both clips and the clip ends before the clip after next
    // starts; otherwise the earlier clip ends where the next one starts. Shortening a piece can break the rule for the
    // transition into it (a piece no longer than its incoming transition), so the pass repeats until nothing changes, and
    // empty pieces disappear. Pieces with the same start are ordered by end: a transition remnant comes first and empties.
    function settleTrack(track) {
        for (let changed = true; changed;) {
            changed = false;
            const order = track.clips.filter(clip => clip.duration > 0).sort((a, b) => a.start - b.start || endOf(a) - endOf(b));
            for (let i = 0; i < order.length - 1; i++) {
                const a = order[i], b = order[i + 1], after = order[i + 2], overlap = endOf(a) - b.start;
                if (overlap <= 0) continue;
                if (a.transition && overlap < Math.min(a.duration, b.duration) && (!after || endOf(a) <= after.start)) continue;
                a.transition = null;
                a.duration = b.start - a.start;
                clampFades(a);
                changed = true;
            }
            track.clips = order.filter(clip => clip.duration > 0);
        }
    }
    // Removes [from, to) from every clip on the track: clips are trimmed, split (blade rule, see noteRight) or deleted.
    function clearRange(project, track, from, to, rights) {
        if (to <= from) return;
        const kept = [];
        track.clips.forEach(clip => {
            const end = endOf(clip);
            if (end <= from || clip.start >= to) { kept.push(clip); return; }
            if (clip.start < from) kept.push(piece(project, clip, clip.start, from, clip.id));
            if (end > to) {
                const split = clip.start < from, right = piece(project, clip, to, end, split ? T().newId('clip') : clip.id);
                if (split) noteRight(rights, clip, right);
                kept.push(right);
            }
        });
        track.clips = kept;
        settleTrack(track);
    }
    function checkPlacements(project, placements) {
        const max = Time().maxFrames(project);
        if (!placements.length) return REASON.nothing;
        for (const { trackId, clips } of placements) {
            const track = trackById(project, trackId);
            if (!track) return REASON.nothing;
            if (track.locked) return REASON.locked;
            for (const clip of clips) {
                if (!T().accepts(track, assetOf(project, clip))) return REASON.kind;
                if (clip.start < 0 || clip.duration < 1 || endOf(clip) > max) return REASON.limit;
                if (clip.duration > maxDurationFrom(project, clip, clip.offset)) return REASON.media;
            }
        }
        return true;
    }
    // Places clips over whatever lies on their tracks (overwrite is the default drop and move).
    function overwrite(project, placements) {
        const check = checkPlacements(project, placements);
        if (check !== true) return check;
        const rights = new Map();
        placements.forEach(({ trackId, clips }) => { const track = trackById(project, trackId); clips.forEach(clip => clearRange(project, track, clip.start, endOf(clip), rights)); });
        relinkRights(project, rights);
        placements.forEach(({ trackId, clips }) => { trackById(project, trackId).clips.push(...clips); });
        return finish(project);
    }
    // Opens `duration` frames at `at`: target tracks split a clip spanning the point, and every unlocked track
    // moves the clips that start at or after it. Clips spanning the point on other tracks stay whole.
    function insertGap(project, at, duration, targetIds) {
        if (!(duration > 0)) return REASON.nothing;
        const targets = new Set(targetIds);
        for (const track of project.tracks) {
            if (!targets.has(track.id)) continue;
            if (track.locked) return REASON.locked;
            if (isCutInTransition(track, at)) return REASON.transition;
        }
        const rights = new Map();
        project.tracks.forEach(track => {
            if (track.locked) return;
            if (targets.has(track.id)) splitAt(project, track, at, rights);
            track.clips.forEach(clip => { if (clip.start >= at) clip.start += duration; });
        });
        relinkRights(project, rights);
        return T().projectEnd(project) > Time().maxFrames(project) ? REASON.limit : true;
    }
    // Insert edit: the placed clips start at or after `at`; room for all of them opens first.
    function insertClips(project, at, placements) {
        const check = checkPlacements(project, placements);
        if (check !== true) return check;
        if (placements.some(({ clips }) => clips.some(clip => clip.start < at))) return REASON.nothing;
        const end = Math.max(...placements.flatMap(({ clips }) => clips.map(endOf)));
        const gap = insertGap(project, at, end - at, placements.map(({ trackId }) => trackId));
        if (gap !== true) return gap;
        placements.forEach(({ trackId, clips }) => { trackById(project, trackId).clips.push(...clips); });
        return finish(project);
    }
    // Lift delete: the clips disappear and leave gaps.
    function removeClips(project, ids) {
        const doomed = editable(project, ids);
        if (!doomed.length) return refusal(project, ids);
        doomed.forEach(({ track, clip }) => { track.clips = track.clips.filter(item => item !== clip); });
        return finish(project);
    }

    // Deletes clips and closes the time that becomes empty on all of their tracks; other tracks stay.
    function rippleDelete(project, ids) {
        const doomed = editable(project, ids);
        if (!doomed.length) return refusal(project, ids);
        const tracks = Array.from(new Set(doomed.map(({ track }) => track)));
        const ranges = doomed.map(({ clip }) => [clip.start, endOf(clip)]);
        doomed.forEach(({ track, clip }) => { track.clips = track.clips.filter(item => item !== clip); });
        closeRanges(tracks, ranges);
        return finish(project);
    }
    // The empty space around `frame` on a track, bounded by a later clip (space after the last clip is no gap).
    function gapAt(project, trackId, frame) {
        const track = trackById(project, trackId);
        if (!track || track.clips.some(clip => clip.start <= frame && frame < endOf(clip))) return null;
        const next = sorted(track).find(clip => clip.start > frame);
        if (!next) return null;
        return { trackId, start: Math.max(0, ...track.clips.filter(clip => endOf(clip) <= frame).map(endOf)), end: next.start };
    }
    function rippleDeleteGap(project, trackId, frame) {
        const gap = gapAt(project, trackId, frame);
        if (!gap) return REASON.nothing;
        const track = trackById(project, trackId);
        if (track.locked) return REASON.locked;
        closeRanges([track], [[gap.start, gap.end]]);
        return finish(project);
    }
    function clampDelta(lo, hi, delta) {
        if (lo > hi) return null;
        return Math.round(Math.max(lo, Math.min(hi, delta)));
    }
    // Trims the same edge of every listed clip by `delta` (clamped to what all of them allow) and moves the
    // later clips on their tracks. End edge: the clip ends later or earlier. Start edge: the clip keeps its
    // start and shows `delta` frames less at its head.
    function rippleTrim(project, ids, edge, delta) {
        const items = editable(project, ids);
        if (!items.length) return refusal(project, ids);
        const max = Time().maxFrames(project), perTrack = new Map();
        items.forEach(({ track }) => perTrack.set(track, (perTrack.get(track) || 0) + 1));
        let lo = -Infinity, hi = Infinity;
        items.forEach(({ track, clip }) => {
            const min = minDuration(track, clip), room = Math.floor((max - Math.max(...track.clips.map(endOf))) / perTrack.get(track));
            if (edge === 'end') {
                lo = Math.max(lo, min - clip.duration);
                hi = Math.min(hi, maxDurationFrom(project, clip, clip.offset) - clip.duration, room);
            } else {
                hi = Math.min(hi, clip.duration - min);
                lo = Math.max(lo, -headroom(project, clip), -room);
            }
        });
        const d = clampDelta(lo, hi, delta);
        if (d === null) return REASON.bounds;
        if (!d) return delta ? REASON.bounds : REASON.nothing;
        items.forEach(({ track, clip }) => {
            const later = track.clips.filter(item => item !== clip && item.start > clip.start);
            if (edge === 'end') {
                clip.duration += d;
                later.forEach(item => { item.start += d; });
            } else {
                if (!isStill(project, clip)) moveHead(clip, d);
                clip.duration -= d;
                later.forEach(item => { item.start -= d; });
            }
            clampFades(clip);
        });
        return finish(project);
    }
    // Moves cuts between neighbors: the left clip ends `delta` later, the right clip starts later and shows
    // later source frames. A transition at the cut keeps its overlap.
    function roll(project, pairs, delta) {
        const items = [];
        for (const [leftId, rightId] of pairs || []) {
            const a = T().clipFor(project, leftId), b = T().clipFor(project, rightId);
            if (!a || !b || a.track !== b.track) return REASON.nothing;
            if (a.track.locked) return REASON.locked;
            const order = sorted(a.track);
            if (order[order.indexOf(a.clip) + 1] !== b.clip || endOf(a.clip) < b.clip.start) return REASON.nothing;
            items.push({ track: a.track, left: a.clip, right: b.clip });
        }
        if (!items.length) return REASON.nothing;
        let lo = -Infinity, hi = Infinity;
        items.forEach(({ track, left, right }) => {
            lo = Math.max(lo, minDuration(track, left) - left.duration, -headroom(project, right));
            hi = Math.min(hi, maxDurationFrom(project, left, left.offset) - left.duration, right.duration - minDuration(track, right));
        });
        const d = clampDelta(lo, hi, delta);
        if (d === null) return REASON.bounds;
        if (!d) return delta ? REASON.bounds : REASON.nothing;
        items.forEach(({ left, right }) => {
            left.duration += d;
            if (!isStill(project, right)) moveHead(right, d);
            right.start += d; right.duration -= d;
            clampFades(left); clampFades(right);
        });
        return finish(project);
    }
    // Shows other source frames in the same place: the offset moves by `delta` timeline frames.
    function slip(project, ids, delta) {
        const items = editable(project, ids);
        if (!items.length) return refusal(project, ids);
        const moving = items.filter(({ clip }) => !isStill(project, clip));
        if (!moving.length) return REASON.media;
        // A ramped clip keeps its curve on the timeline; slipping would move the curve against the content (index §4.18).
        if (moving.some(({ clip }) => FX().hasRamp(clip))) return REASON.ramp;
        let lo = -Infinity, hi = Infinity;
        moving.forEach(({ clip }) => {
            const frames = Number(assetOf(project, clip).duration_frames) || 0, speed = speedOf(clip);
            lo = Math.max(lo, -headroom(project, clip));
            hi = Math.min(hi, Math.floor((frames - 1 - (clip.duration - 1) * speed - clip.offset) / speed + 1e-9));
        });
        const d = clampDelta(lo, hi, delta);
        if (d === null) return REASON.bounds;
        if (!d) return delta ? REASON.bounds : REASON.nothing;
        moving.forEach(({ clip }) => moveHead(clip, d));
        return finish(project);
    }
    // Moves a clip between its neighbors: a joined left neighbor ends with it, a joined right neighbor starts
    // with it (showing other frames). Without a joined neighbor the clip moves into the gap on that side.
    function slide(project, clipId, delta) {
        const found = T().clipFor(project, clipId);
        if (!found) return REASON.nothing;
        if (found.track.locked) return REASON.locked;
        const { track, clip } = found, order = sorted(track), index = order.indexOf(clip);
        const prev = order[index - 1], next = order[index + 1];
        const leftJoined = !!prev && endOf(prev) >= clip.start, rightJoined = !!next && next.start <= endOf(clip);
        let lo = leftJoined ? minDuration(track, prev) - prev.duration : (prev ? endOf(prev) : 0) - clip.start;
        let hi = leftJoined ? maxDurationFrom(project, prev, prev.offset) - prev.duration : Infinity;
        if (rightJoined) {
            hi = Math.min(hi, next.duration - minDuration(track, next));
            lo = Math.max(lo, -headroom(project, next));
        } else hi = Math.min(hi, (next ? next.start : Time().maxFrames(project)) - endOf(clip));
        const d = clampDelta(lo, hi, delta);
        if (d === null) return REASON.bounds;
        if (!d) return delta ? REASON.bounds : REASON.nothing;
        clip.start += d;
        if (leftJoined) { prev.duration += d; clampFades(prev); }
        if (rightJoined) {
            if (!isStill(project, next)) moveHead(next, d);
            next.start += d; next.duration -= d;
            clampFades(next);
        }
        return finish(project);
    }

    // Moves clips by `delta` frames. Clips of the anchor clip's track kind also move `trackShift` tracks
    // within that kind. Overwrite replaces what lies under the moved clips; insert opens room on every
    // unlocked track. Magnetic first closes the places the clips leave on their own tracks.
    function moveClips(project, ids, anchorId, delta, trackShift, mode, options) {
        const items = editable(project, ids);
        if (!items.length) return refusal(project, ids);
        const anchor = T().clipFor(project, anchorId), kind = anchor ? anchor.track.kind : '';
        const plan = [];
        for (const { track, clip } of items) {
            let dest = track;
            if (trackShift && track.kind === kind) {
                const list = project.tracks.filter(item => item.kind === kind);
                dest = list[list.indexOf(track) + trackShift];
                if (!dest) return REASON.bounds;
                if (dest.locked) return REASON.locked;
                if (!T().accepts(dest, assetOf(project, clip))) return REASON.kind;
            }
            plan.push({ source: track, dest, clip });
        }
        const first = Math.min(...plan.map(({ clip }) => clip.start)), last = Math.max(...plan.map(({ clip }) => endOf(clip)));
        const shift = Math.max(-first, Math.min(Math.round(delta) || 0, Time().maxFrames(project) - last));
        if (!shift && !trackShift) return REASON.nothing;
        const lifted = new Map();
        plan.forEach(({ source, clip }) => {
            source.clips = source.clips.filter(item => item !== clip);
            if (!lifted.has(source)) lifted.set(source, []);
            lifted.get(source).push([clip.start, endOf(clip)]);
        });
        if (options && options.magnetic) lifted.forEach((ranges, track) => closeRanges([track], ranges));
        const placements = [];
        plan.forEach(({ dest, clip }) => {
            clip.start += shift;
            let entry = placements.find(item => item.trackId === dest.id);
            if (!entry) placements.push(entry = { trackId: dest.id, clips: [] });
            entry.clips.push(clip);
        });
        return mode === 'insert' ? insertClips(project, first + shift, placements) : overwrite(project, placements);
    }
    // Clipboard data: clip copies with times relative to the earliest one and the track they came from.
    function copyClips(project, ids) {
        const wanted = new Set(ids || []), entries = [];
        project.tracks.forEach(track => {
            const kindIndex = project.tracks.filter(item => item.kind === track.kind).indexOf(track);
            track.clips.forEach(clip => { if (wanted.has(clip.id)) entries.push({ trackId: track.id, kind: track.kind, kindIndex, clip: clone(clip) }); });
        });
        if (!entries.length) return null;
        const origin = Math.min(...entries.map(({ clip }) => clip.start));
        entries.forEach(({ clip }) => { clip.start -= origin; });
        return { span: Math.max(...entries.map(({ clip }) => endOf(clip))), entries };
    }
    function pasteTrack(project, board, entry, targets) {
        const sameKind = project.tracks.filter(track => track.kind === entry.kind);
        if (targets && targets[entry.kind]) {
            const base = sameKind.findIndex(track => track.id === targets[entry.kind]);
            const lowest = Math.min(...board.entries.filter(item => item.kind === entry.kind).map(item => item.kindIndex));
            return base < 0 ? null : sameKind[base + entry.kindIndex - lowest] || null;
        }
        return trackById(project, entry.trackId) || sameKind[entry.kindIndex] || null;
    }
    // The next start after a clip that collides with the copies placed at `at`, or 0 when all of them fit.
    function collision(project, board, at) {
        let next = 0;
        board.entries.forEach(({ trackId, clip }) => {
            const from = at + clip.start, to = from + clip.duration;
            trackById(project, trackId).clips.forEach(item => { if (item.start < to && from < endOf(item)) next = Math.max(next, endOf(item) - clip.start); });
        });
        return next;
    }
    // Copies the clips to the first place after them where every copy fits on its own track.
    function duplicateClips(project, ids, out) {
        const items = editable(project, ids);
        if (!items.length) return refusal(project, ids);
        const board = copyClips(project, items.map(({ clip }) => clip.id)), max = Time().maxFrames(project);
        let at = Math.min(...items.map(({ clip }) => clip.start)) + board.span;
        while (at + board.span <= max) {
            const next = collision(project, board, at);
            if (!next) break;
            at = next;
        }
        if (at + board.span > max) return REASON.limit;
        return paste(project, board, at, 'overwrite', { out });
    }
    function clipsFrom(project, trackId, frame) {
        const track = trackById(project, trackId);
        return track ? sorted(track).filter(clip => clip.start >= frame).map(clip => clip.id) : [];
    }
    // The two clips that share the clip's edge (touching or joined by a transition), or null.
    function neighborAt(project, clipId, edge) {
        const found = T().clipFor(project, clipId);
        if (!found) return null;
        const order = sorted(found.track), index = order.indexOf(found.clip);
        if (edge === 'end') { const next = order[index + 1]; return next && next.start <= endOf(found.clip) ? [found.clip.id, next.id] : null; }
        const prev = order[index - 1];
        return prev && endOf(prev) >= found.clip.start ? [prev.id, found.clip.id] : null;
    }
    function snapPoints(project, excludeIds, extra) {
        const skip = new Set(excludeIds || []), points = [0];
        project.tracks.forEach(track => track.clips.forEach(clip => { if (!skip.has(clip.id)) points.push(clip.start, endOf(clip)); }));
        return points.concat((extra || []).filter(Number.isFinite));
    }
    // Adjusts `delta` by at most `range` frames so one of the moving edges lands on a snap point.
    function snapDelta(edges, delta, points, range) {
        if (!(range > 0)) return delta;
        let best = null;
        edges.forEach(edge => points.forEach(point => {
            const adjust = point - (edge + delta);
            if (Math.abs(adjust) <= range && (best === null || Math.abs(adjust) < Math.abs(best))) best = adjust;
        }));
        return best === null ? delta : delta + best;
    }

    // ---- Link groups (spec 4.2): clips with the same link id. ----
    function linkMembers(project, link) {
        const found = [];
        if (!link) return found;
        (project.tracks || []).forEach(track => track.clips.forEach(clip => { if (clip.link === link) found.push({ track, clip }); }));
        return found;
    }
    function linkGroup(project, clipId) {
        const found = T().clipFor(project, clipId);
        if (!found) return [];
        if (!found.clip.link) return [clipId];
        return linkMembers(project, found.clip.link).map(({ clip }) => clip.id);
    }
    function withLinks(project, ids) {
        return Array.from(new Set((ids || []).flatMap(id => linkGroup(project, id))));
    }
    // The group members whose same edge sits on the clip's frame (the clip itself included).
    function linkedEdges(project, clipId, edge) {
        const found = T().clipFor(project, clipId);
        if (!found) return [];
        if (!found.clip.link) return [clipId];
        const at = edge === 'start' ? found.clip.start : endOf(found.clip);
        return linkMembers(project, found.clip.link).filter(({ clip }) => (edge === 'start' ? clip.start : endOf(clip)) === at).map(({ clip }) => clip.id);
    }
    // The audio clip a detached video clip owns: same group, audio track, same asset and timing.
    function audioPartner(project, clipId) {
        const found = T().clipFor(project, clipId);
        if (!found || !found.clip.link) return null;
        const clip = found.clip;
        const match = linkMembers(project, clip.link).find(({ track, clip: other }) => track.kind === 'audio' && other.asset_id === clip.asset_id &&
            other.start === clip.start && other.duration === clip.duration && other.offset === clip.offset);
        return match ? match.clip.id : null;
    }
    // Ctrl+L: the editable clips of ids share one new group (link '' unlinks them). A group left with one clip ends
    // (unless that clip sits on a locked track, which never changes).
    function setLink(project, ids, link) {
        const items = editable(project, ids);
        if (!items.length) return refusal(project, ids);
        if (link && items.length < 2) return REASON.nothing;
        if (link && new Set(items.concat(linkMembers(project, link)).map(({ clip }) => clip.id)).size > maxLinkClips()) return REASON.link;
        const previous = new Set(items.map(({ clip }) => clip.link).filter(Boolean));
        items.forEach(({ clip }) => { if (link) clip.link = link; else delete clip.link; });
        previous.forEach(id => { const rest = linkMembers(project, id); if (rest.length === 1 && !rest[0].track.locked) delete rest[0].clip.link; });
        return finish(project);
    }

    // ---- Blade with links (spec 4.2): scope.trackIds or scope.clipIds pick the clips under the frame; with
    // scope.linked their link partners under the frame are cut too. The left part keeps id, link and fade in; the
    // right part gets fade out and the outgoing transition. Right parts of one cut group share a new link id (a lone
    // right part is unlinked), so the cut leaves two groups. A linked cut takes a group whole or not at all: when a
    // transition protects one of its members under the frame while another would be cut, nothing is cut. ----
    function blade(project, frame, scope, out) {
        const wanted = scope.clipIds ? new Set(scope.clipIds) : null, trackIds = scope.trackIds ? new Set(scope.trackIds) : null;
        const under = clip => clip.start < frame && frame < endOf(clip);
        const picks = [];
        project.tracks.forEach(track => track.clips.forEach(clip => {
            if (under(clip) && (!trackIds || trackIds.has(track.id)) && (!wanted || wanted.has(clip.id))) picks.push({ track, clip });
        }));
        if (out) out.created = [];
        if (scope.linked) {
            const links = new Set(picks.map(({ clip }) => clip.link).filter(Boolean)), seen = new Set(picks.map(({ clip }) => clip.id));
            project.tracks.forEach(track => track.clips.forEach(clip => {
                if (under(clip) && clip.link && links.has(clip.link) && !seen.has(clip.id)) { picks.push({ track, clip }); seen.add(clip.id); }
            }));
            const groups = new Map();
            picks.forEach(({ track, clip }) => {
                if (!clip.link || track.locked) return;
                const state = groups.get(clip.link) || { cut: false, kept: false };
                if (isCutInTransition(track, frame)) state.kept = true; else state.cut = true;
                groups.set(clip.link, state);
            });
            if (Array.from(groups.values()).some(state => state.cut && state.kept)) return REASON.transition;
        }
        const created = [], rights = new Map();
        let locked = false, transition = false;
        picks.forEach(({ track, clip }) => {
            if (track.locked) { locked = true; return; }
            if (isCutInTransition(track, frame)) { transition = true; return; }
            const right = piece(project, clip, frame, endOf(clip), T().newId('clip'));
            noteRight(rights, clip, right);
            Object.assign(clip, piece(project, clip, clip.start, frame, clip.id));
            track.clips.push(right);
            created.push(right.id);
        });
        relinkRights(project, rights);
        if (out) out.created = created;
        if (!created.length) return transition ? REASON.transition : locked ? REASON.locked : REASON.nothing;
        return finish(project);
    }

    // Pastes clipboard data at `at` (overwrite or insert). options.targets ({ video, audio, overlay } → track id)
    // puts each kind's clips on that track and the following ones of the kind; without it clips return to the
    // tracks they came from. A group pasted with two or more clips gets one new id; a lone copy is unlinked. A detached
    // video clip pasted without its audio clip (same group, audio track, same asset) plays its own sound again.
    // options.out.created lists the new clip ids.
    function paste(project, board, at, mode, options) {
        const opts = options || {};
        if (!board || !board.entries.length) return REASON.nothing;
        const counts = new Map(), remap = new Map(), placements = [], created = [];
        board.entries.forEach(({ clip }) => { if (clip.link) counts.set(clip.link, (counts.get(clip.link) || 0) + 1); });
        const soundCopied = clip => !!clip.link && board.entries.some(other => other.kind === 'audio' && other.clip.link === clip.link && other.clip.asset_id === clip.asset_id);
        for (const entry of board.entries) {
            if (!assetOf(project, entry.clip)) return REASON.media;
            const track = pasteTrack(project, board, entry, opts.targets);
            if (!track) return REASON.kind;
            const clip = Object.assign(clone(entry.clip), { id: T().newId('clip'), start: at + entry.clip.start });
            if (entry.clip.link) {
                if (counts.get(entry.clip.link) > 1) {
                    if (!remap.has(entry.clip.link)) remap.set(entry.clip.link, T().newId('link'));
                    clip.link = remap.get(entry.clip.link);
                } else delete clip.link;
            }
            if (clip.audio_detached && !soundCopied(entry.clip)) delete clip.audio_detached;
            let placement = placements.find(item => item.trackId === track.id);
            if (!placement) placements.push(placement = { trackId: track.id, clips: [] });
            placement.clips.push(clip);
            created.push(clip.id);
        }
        const result = mode === 'insert' ? insertClips(project, at, placements) : overwrite(project, placements);
        if (result === true && opts.out) opts.out.created = created;
        return result;
    }

    // Copies attribute groups from `source` (a clip object) to the listed clips. picture and keyframes skip audio clips.
    // The speed group skips stills and goes through setSpeed for the clips and their link partners, so it follows the
    // timing rules (ripple by default); a ramped source gives its ramp instead (pasteRamp). keyframes replaces the
    // clip's keys, re-anchored after the speed group so each key keeps its timing from the clip start (index §4.18).
    // A value the target animates ignores its field, so the picture group writes it as a key at `frame` (the playhead; the
    // auto-key of VideoStudioKeyframeEdit.setValues) and changes the field only for static values. While `frame` is
    // outside the clip (or not given) an animated value is left as it is. With the keyframes group in the same paste the
    // target's keys are replaced anyway, so every field takes the source's value and nothing is keyed.
    function pasteAttributes(project, source, ids, groups, frame) {
        const items = editable(project, ids), wanted = groups || [], replaceKeys = wanted.includes('keyframes');
        if (!items.length) return refusal(project, ids);
        const KE = window.VideoStudioKeyframeEdit;
        let changed = false;
        for (const { track, clip } of items) {
            for (const group of wanted) {
                if (group === 'picture' && track.kind === 'audio') continue;
                const keyed = {};
                (ATTRIBUTE_GROUPS[group] || []).forEach(key => {
                    if (key === 'speed' || key === 'keyframes' || source[key] === undefined) return;
                    if (!replaceKeys && Array.isArray(clip.keyframes && clip.keyframes[key]) && clip.keyframes[key].length) keyed[key] = source[key];
                    else { clip[key] = clone(source[key]); changed = true; }
                });
                if (Object.keys(keyed).length && KE && frame >= clip.start && frame < clip.start + clip.duration) {
                    const result = KE.setValues(project, clip.id, keyed, frame);
                    if (result !== true) return result;
                    changed = true;
                }
            }
            clampFades(clip);
        }
        if (wanted.includes('speed')) {
            const timed = withLinks(project, items.map(({ clip }) => clip.id)).filter(id => !isStill(project, T().clipFor(project, id).clip));
            if (timed.length) {
                const result = FX().hasRamp(source) ? pasteRamp(project, source, timed) : setSpeed(project, timed, speedOf(source), false);
                if (result !== true) return result;
                changed = true;
            }
        }
        if (wanted.includes('keyframes')) {
            items.forEach(({ track, clip }) => {
                if (track.kind === 'audio') return;
                const keys = FX().reanchorKeyframes(source, clip);
                if (keys) clip.keyframes = keys; else delete clip.keyframes;
                changed = true;
            });
        }
        return changed ? finish(project) : REASON.nothing;
    }
    // The speed group with a ramped source (index §4.18): unlocked video-track clips of video media take the ramp and
    // keep their duration; the other clips keep their timing. media when the media ends before a ramped clip does.
    function pasteRamp(project, source, ids) {
        for (const id of ids) {
            const { track, clip } = T().clipFor(project, id), asset = assetOf(project, clip);
            if (track.locked || track.kind !== 'video' || !asset || asset.kind !== 'video') continue;
            clip.ramp = clone(source.ramp);
            delete clip.speed;
            if (clip.duration > FX().maxDurationFor(clip, Number(asset.duration_frames) || 0, clip.offset)) return REASON.media;
        }
        return true;
    }

    // ---- Speed (spec 4.4). The default keeps the source range: round(duration × old / new) frames at a constant
    // speed (phase 1), round(sourceExtent / new) + 1 frames on a ramp (index §4.18), at most what the media holds at
    // the new speed, and later clips on each track ripple. keepDuration keeps the duration and changes the source
    // range. The new constant speed replaces a ramp. Stills have no speed. Every clip keeps the source extent rule,
    // its transitions and the project length. ----
    function setSpeed(project, ids, speed, keepDuration) {
        if (!(speed >= 0.25 && speed <= 16)) return REASON.nothing;
        const items = editable(project, ids);
        if (!items.length) return refusal(project, ids);
        const plans = [];
        for (const { track, clip } of items) {
            if (isStill(project, clip)) {
                if (speed !== 1) return REASON.media;
                plans.push({ track, clip, duration: clip.duration });
                continue;
            }
            const fits = Math.floor((Number(assetOf(project, clip).duration_frames) - 1 - clip.offset) / speed + 1e-9) + 1;
            // A ramp keeps its source range sourceExtent (index §4.18); a constant speed keeps phase 1's formula.
            const kept = FX().hasRamp(clip) ? Math.round(FX().sourceExtent(clip) / speed) + 1 : Math.round(clip.duration * speedOf(clip) / speed);
            const duration = keepDuration ? clip.duration : Math.max(1, Math.min(fits, kept));
            if (!(duration <= fits)) return REASON.media;
            if (duration < minDuration(track, clip)) return REASON.bounds;
            plans.push({ track, clip, duration });
        }
        plans.forEach(({ track, clip, duration }) => {
            const delta = duration - clip.duration;
            if (delta) track.clips.filter(item => item !== clip && item.start > clip.start).forEach(item => { item.start += delta; });
            clip.duration = duration;
            if (speed === 1) delete clip.speed; else clip.speed = speed; delete clip.ramp;
            clampFades(clip);
        });
        if (T().projectEnd(project) > Time().maxFrames(project)) return REASON.limit;
        return finish(project);
    }

    // ---- Detached audio (spec 4.4). The sound of a video clip moves to an audio clip with the same timing on the first
    // audio track free for that range, or on a new track while fewer than MAX_TRACKS_PER_KIND exist. Both clips join one
    // link group. ids = { clip, link, track, trackName } provides the new ids and the name of a new track. ----
    function detachAudio(project, clipId, ids) {
        const found = T().clipFor(project, clipId);
        if (!found || found.track.kind !== 'video') return REASON.nothing;
        if (found.track.locked) return REASON.locked;
        const clip = found.clip, asset = assetOf(project, clip);
        if (!asset || asset.kind !== 'video' || !asset.has_audio) return REASON.media;
        if (clip.audio_detached) return REASON.nothing;
        // The sound of a ramped clip plays at 1x (ramps and keyframes animate the picture only), so its range must fit at 1x.
        if (FX().hasRamp(clip) && clip.offset + clip.duration > (Number(asset.duration_frames) || 0)) return REASON.media;
        if ((clip.link ? linkMembers(project, clip.link).length : 1) + 1 > maxLinkClips()) return REASON.link;
        const start = clip.start, end = endOf(clip);
        const audioTracks = project.tracks.filter(track => track.kind === 'audio');
        let track = audioTracks.find(item => !item.locked && !item.clips.some(other => other.start < end && start < endOf(other)));
        if (!track) {
            if (audioTracks.length >= T().MAX_TRACKS_PER_KIND) return REASON.tracks;
            track = { id: ids.track, kind: 'audio', name: ids.trackName, muted: false, hidden: false, locked: false, clips: [] };
            project.tracks.push(track);
        }
        const link = clip.link || ids.link;
        const audio = Object.assign(clone(clip), {
            id: ids.clip, link, x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, fit: 'contain', text: '', text_style: null, transition: null
        });
        delete audio.audio_detached; delete audio.keyframes; delete audio.ramp;
        clip.link = link;
        clip.audio_detached = true;
        track.clips.push(audio);
        return finish(project);
    }
    // An audio clip that still plays a detached clip's sound: an audio-track clip of the same asset in its group or
    // overlapping it in time (a J-cut partner, or the uncut audio under a part that an Alt blade unlinked).
    function soundHeld(project, clip) {
        return project.tracks.some(track => track.kind === 'audio' && track.clips.some(other => other.asset_id === clip.asset_id &&
            ((!!clip.link && other.link === clip.link) || (other.start < endOf(clip) && clip.start < endOf(other)))));
    }
    // Reattach applies to a clip that owns its audio clip (audioPartner) and to an orphaned detached clip, whose audio
    // clip was deleted on its own: no audio clip holds its sound any more.
    function canReattach(project, clipId) {
        const found = T().clipFor(project, clipId);
        return !!found && !!found.clip.audio_detached && (!!audioPartner(project, clipId) || !soundHeld(project, found.clip));
    }
    // Reattach: removes the audio clip a detached clip owns and clears the flag; an orphaned clip only gets its sound
    // back (the flag is cleared). A group left with the video clip ends.
    function reattachAudio(project, clipId) {
        const found = T().clipFor(project, clipId);
        if (!found || !found.clip.audio_detached) return REASON.nothing;
        if (found.track.locked) return REASON.locked;
        const partnerId = audioPartner(project, clipId);
        if (partnerId) {
            const partner = T().clipFor(project, partnerId);
            if (partner.track.locked) return REASON.locked;
            partner.track.clips = partner.track.clips.filter(item => item !== partner.clip);
        } else if (soundHeld(project, found.clip)) return REASON.nothing;
        delete found.clip.audio_detached;
        if (linkMembers(project, found.clip.link).length <= 1) delete found.clip.link;
        return finish(project);
    }

    // Enable and disable (Shift+E) and colour labels (empty clears the label). Neither changes timing.
    function setDisabled(project, ids, disabled) {
        const items = editable(project, ids);
        if (!items.length) return refusal(project, ids);
        items.forEach(({ clip }) => { if (disabled) clip.disabled = true; else delete clip.disabled; });
        return finish(project);
    }
    function setLabel(project, ids, label) {
        if (label && !T().LABEL_COLORS.includes(label)) return REASON.nothing;
        const items = editable(project, ids);
        if (!items.length) return refusal(project, ids);
        items.forEach(({ clip }) => { if (label) clip.label = label; else delete clip.label; });
        return finish(project);
    }

    // ---- Derived media (spec 4.4). ----
    // The source frame the renderer shows at timeline frame `frame`: offset + (frame - start) × speed, rounded down and
    // kept inside the media. A ramped clip uses its mean speed up to the frame (VideoStudioEffects.meanSpeed). That is
    // floor(offset + S(t)), computed exactly as the server checks a still request. A freeze frame captures this frame.
    function sourceFrameAt(project, clip, frame) {
        const asset = assetOf(project, clip), t = frame - clip.start;
        const value = Math.floor(Number(clip.offset || 0) + t * FX().meanSpeed(clip, t) + 1e-9);
        return Math.max(0, Math.min(Math.max(0, Number(asset && asset.duration_frames) - 1), value));
    }
    // The name of a reversed copy, without extension (the server adds .mp4): the name with the localized
    // " (<label>)" suffix toggled, so reversing a reversed copy gives back the plain name instead of another suffix.
    function reversedName(name, label) {
        const suffix = ' (' + label + ')';
        const base = String(name || '').trim().replace(/\.[A-Za-z0-9]{1,5}$/, '');
        const plain = base.endsWith(suffix) ? base.slice(0, -suffix.length).trim() : '';
        return plain || base + suffix;
    }
    // Members of the clip's group that read the clip's asset (the clip alone when it is unlinked); reverse switches all.
    function sameAssetMembers(project, found) {
        return found.clip.link ? linkMembers(project, found.clip.link).filter(({ clip }) => clip.asset_id === found.clip.asset_id) : [found];
    }
    // Whether the clip or a same-asset link partner has a speed ramp: reverseClips refuses those (reason ramp), so the UI
    // offers Reverse only when this is false (P.reverseFits).
    function reverseRamped(project, clipId) {
        const found = T().clipFor(project, clipId);
        return !!found && sameAssetMembers(project, found).some(({ clip }) => FX().hasRamp(clip));
    }
    // The source range one reverse job covers: the union of the ranges the same-asset group members read, each
    // [offset, offset + ceil((duration - 1) × speed) + 1). A J- or L-cut partner trimmed past the clip widens it. At a
    // fractional speed the range holds the next source frame too (it exists: the source extent rule bounds
    // offset + (duration - 1) × speed by a whole frame), so the mirrored offset never rounds below 0 and the clip fits.
    function reverseRange(project, clipId) {
        const found = T().clipFor(project, clipId);
        if (!found) return null;
        let first = Infinity, end = -Infinity;
        sameAssetMembers(project, found).forEach(({ clip }) => {
            const extent = FX().hasRamp(clip) ? FX().sourceExtent(clip) : (clip.duration - 1) * speedOf(clip);
            first = Math.min(first, clip.offset);
            end = Math.max(end, clip.offset + Math.ceil(extent - 1e-9) + 1);
        });
        return { rangeStart: first, frames: end - first };
    }
    // reversed = { assetId, rangeStart, frames, sourceAssetId }: the reversed asset holds the source range
    // [rangeStart, rangeStart + frames) of sourceAssetId backwards. The group plays backwards around the anchor (the clip
    // the user reversed), which keeps its place: timeline frame t maps to anchor.start + anchor.end - 1 - t, so another
    // same-asset member moves to start' = anchor.start + (anchor.end - member.end) with its duration (a J-cut becomes an
    // L-cut), and every member reads the mirrored offset frames - 1 - (offset - rangeStart) - (duration - 1) × speed. At
    // the mapped frame each member shows the source frame it showed before, so picture and sound stay in sync. Members of
    // other assets stay. A member outside the range, a clip that no longer reads the reversed source, or a moved member
    // that would overlap another clip, touch a transition or leave the project refuses (reason room) without changes.
    // Keyframes mirror around the reversed asset's last frame (index §4.18); extra hold keys past the limits refuse
    // (limit). A ramped member refuses (reason ramp): a mirrored ramp with hold spans is not representable, so reverse
    // first and ramp afterwards (index §4.18).
    function reverseClips(project, clipId, reversed) {
        const found = T().clipFor(project, clipId);
        if (!found) return REASON.nothing;
        if (found.track.locked) return REASON.locked;
        if (reverseRamped(project, clipId)) return REASON.ramp;
        const target = (project.assets || []).find(asset => asset.id === reversed.assetId);
        if (!target) return REASON.media;
        if (reversed.sourceAssetId && found.clip.asset_id !== reversed.sourceAssetId) return REASON.media;
        const anchor = found.clip, pivot = anchor.start + endOf(anchor), max = Time().maxFrames(project);
        const plans = [];
        for (const { track, clip } of sameAssetMembers(project, found)) {
            if (track.locked) return REASON.locked;
            if (!T().accepts(track, target) || (clip.audio_detached && !target.has_audio)) return REASON.media;
            const speed = speedOf(clip), last = clip.offset + (clip.duration - 1) * speed;
            if (clip.offset < reversed.rangeStart || last > reversed.rangeStart + reversed.frames - 1 + 1e-9) return REASON.media;
            const offset = Math.max(0, Math.floor(reversed.frames - 1 - (clip.offset - reversed.rangeStart) - (clip.duration - 1) * speed + 1e-9));
            if (clip.duration > Math.floor((Number(target.duration_frames) - 1 - offset) / speed + 1e-9) + 1) return REASON.media;
            const keyframes = clip.keyframes ? FX().mirrorKeyframes(clip.keyframes, reversed.rangeStart + reversed.frames - 1) : undefined;
            if (keyframes && overLimit(keyframes)) return REASON.limit;
            plans.push({ track, clip, offset, keyframes, start: clip === anchor ? clip.start : pivot - endOf(clip) });
        }
        const moving = plans.filter(plan => plan.start !== plan.clip.start), members = new Set(plans.map(({ clip }) => clip));
        for (const { track, clip, start } of moving) {
            const order = sorted(track), prev = order[order.indexOf(clip) - 1];
            if (start < 0 || start + clip.duration > max || clip.transition || (prev && endOf(prev) > clip.start)) return REASON.room;
            const end = start + clip.duration;
            const placed = other => (members.has(other) ? plans.find(plan => plan.clip === other).start : other.start);
            if (track.clips.some(other => other !== clip && placed(other) < end && start < placed(other) + other.duration)) return REASON.room;
        }
        plans.forEach(({ clip, offset, start, keyframes }) => { clip.asset_id = reversed.assetId; clip.offset = offset; clip.start = start; if (keyframes) clip.keyframes = keyframes; else delete clip.keyframes; });
        return finish(project);
    }
    // Freeze frame (spec 4.4): the clip and its link partners are cut at the playhead, then a still of the captured
    // frame (the geometry and opacity the clip shows there, keyframes evaluated, no keys) is inserted on the clip's own
    // track. Insert rules move the later clips.
    // captured = { asset_id, clip_id, duration, source_asset_id, source_frame } (duration in frames). When given, the
    // source fields name the frame the still shows; a clip that shows another frame at the cut (slipped, moved, re-timed
    // or reversed while the capture waited) refuses, so the still never jumps against the clip.
    function freezeFrame(project, clipId, frame, captured) {
        const found = T().clipFor(project, clipId);
        if (!found) return REASON.nothing;
        const { track, clip } = found;
        if (track.locked) return REASON.locked;
        if (track.kind !== 'video') return REASON.kind;
        if (!(frame > clip.start && frame < endOf(clip))) return REASON.nothing;
        if (!(project.assets || []).some(asset => asset.id === captured.asset_id && asset.kind === 'image')) return REASON.media;
        if (captured.source_asset_id != null && clip.asset_id !== captured.source_asset_id) return REASON.media;
        if (captured.source_frame != null && sourceFrameAt(project, clip, frame) !== captured.source_frame) return REASON.media;
        // Evaluated before the cut: the still shows what the clip shows at the frozen frame (index §4.18).
        const state = FX().clipState(clip, frame, project), box = fieldBox(state);
        const cut = blade(project, frame, { clipIds: [clipId], linked: true }, {});
        if (cut !== true) return cut;
        const still = {
            id: captured.clip_id, asset_id: captured.asset_id, start: frame, offset: 0, duration: Math.max(1, Math.round(captured.duration)),
            x: box.x, y: box.y, width: box.width, height: box.height, rotation: state.rotation, opacity: state.opacity,
            volume: 1, fade_in: 0, fade_out: 0, fit: clip.fit, text: '', text_style: null, transition: null
        };
        return insertClips(project, frame, [{ trackId: track.id, clips: [still] }]);
    }

    window.VideoStudioEdit = { REASON, ATTRIBUTE_GROUPS, speedOf, maxDurationFrom, editable, gapAt, overwrite, insertGap, insertClips, removeClips, rippleDelete, rippleDeleteGap, rippleTrim, roll, slip, slide, blade, moveClips, copyClips, paste, duplicateClips, pasteAttributes, clipsFrom, neighborAt, snapPoints, snapDelta, linkGroup, withLinks, linkedEdges, audioPartner, setLink, setSpeed, detachAudio, canReattach, reattachAudio, setDisabled, setLabel, sourceFrameAt, reversedName, reverseRange, reverseRamped, reverseClips, freezeFrame };
})();
