(function () {
    'use strict';

    // DOM-free edit operations (spec §4.2). Each operation changes the given project in place and returns
    // true, or a REASON code; after a refusal the caller discards the project (the app runs operations
    // inside mutate, which restores its snapshot). Locked tracks never change.
    const T = () => window.VideoStudioTimeline;
    const Time = () => window.VideoStudioTime;
    const REASON = Object.freeze({ locked: 'locked', media: 'media', bounds: 'bounds', nothing: 'nothing', limit: 'limit', kind: 'kind', transition: 'transition' });
    const ATTRIBUTE_GROUPS = { picture: ['x', 'y', 'width', 'height', 'rotation', 'opacity', 'fit'], sound: ['volume', 'fade_in', 'fade_out'] };
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
    // Source extent rule: offset + (duration - 1) * speed <= duration_frames - 1. Stills are unbounded.
    function maxDurationFrom(project, clip, offset) {
        if (isStill(project, clip)) return Infinity;
        const frames = Number(assetOf(project, clip).duration_frames) || 0;
        return Math.floor((frames - 1 - offset) / speedOf(clip) + 1e-9) + 1;
    }
    // Source offset after the clip's first frame moves by `frames` timeline frames. Rounding down keeps the
    // last source frame inside the media.
    function shifted(clip, frames) { return Math.max(0, Math.floor(clip.offset + frames * speedOf(clip) + 1e-9)); }
    // Timeline frames the head can grow before the source runs out.
    function headroom(project, clip) { return isStill(project, clip) ? Infinity : Math.floor(clip.offset / speedOf(clip) + 1e-9); }
    // A clip never gets shorter than one frame or than a transition it shares with a neighbor.
    function minDuration(track, clip) {
        const order = sorted(track), prev = order[order.indexOf(clip) - 1];
        const incoming = prev && prev.transition ? endOf(prev) - clip.start : 0;
        const outgoing = clip.transition ? clip.transition.duration : 0;
        return Math.max(1, incoming + 1, outgoing + 1);
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
    function finish(project) {
        T().repairTransitions(project);
        return clipCount(project) > T().MAX_CLIPS ? REASON.limit : true;
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
        if (!isStill(project, clip)) copy.offset = shifted(clip, from - clip.start);
        copy.fade_in = from === clip.start ? Math.min(Number(clip.fade_in) || 0, copy.duration) : 0;
        copy.fade_out = to === endOf(clip) ? Math.min(Number(clip.fade_out) || 0, copy.duration) : 0;
        if (to !== endOf(clip)) copy.transition = null;
        return copy;
    }
    function splitAt(project, track, frame) {
        track.clips.slice().forEach(clip => {
            if (clip.start >= frame || endOf(clip) <= frame) return;
            const right = piece(project, clip, frame, endOf(clip), T().newId('clip'));
            Object.assign(clip, piece(project, clip, clip.start, frame, clip.id));
            track.clips.push(right);
        });
    }
    // After a clear a clip may still reach into the next one: a transition that fits keeps the overlap,
    // otherwise the earlier clip ends where the next one starts.
    function settleTrack(track) {
        const order = sorted(track);
        for (let i = 0; i < order.length - 1; i++) {
            const a = order[i], b = order[i + 1], overlap = endOf(a) - b.start;
            if (overlap <= 0 || (a.transition && overlap < Math.min(a.duration, b.duration))) continue;
            a.transition = null;
            a.duration = b.start - a.start;
            clampFades(a);
        }
        track.clips = order.filter(clip => clip.duration > 0);
    }
    // Removes [from, to) from every clip on the track: clips are trimmed, split or deleted.
    function clearRange(project, track, from, to) {
        if (to <= from) return;
        const kept = [];
        track.clips.forEach(clip => {
            const end = endOf(clip);
            if (end <= from || clip.start >= to) { kept.push(clip); return; }
            if (clip.start < from) kept.push(piece(project, clip, clip.start, from, clip.id));
            if (end > to) kept.push(piece(project, clip, to, end, clip.start < from ? T().newId('clip') : clip.id));
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
        placements.forEach(({ trackId, clips }) => { const track = trackById(project, trackId); clips.forEach(clip => clearRange(project, track, clip.start, endOf(clip))); });
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
        project.tracks.forEach(track => {
            if (track.locked) return;
            if (targets.has(track.id)) splitAt(project, track, at);
            track.clips.forEach(clip => { if (clip.start >= at) clip.start += duration; });
        });
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
                if (!isStill(project, clip)) clip.offset = shifted(clip, d);
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
            if (!isStill(project, right)) right.offset = shifted(right, d);
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
        let lo = -Infinity, hi = Infinity;
        moving.forEach(({ clip }) => {
            const frames = Number(assetOf(project, clip).duration_frames) || 0, speed = speedOf(clip);
            lo = Math.max(lo, -headroom(project, clip));
            hi = Math.min(hi, Math.floor((frames - 1 - (clip.duration - 1) * speed - clip.offset) / speed + 1e-9));
        });
        const d = clampDelta(lo, hi, delta);
        if (d === null) return REASON.bounds;
        if (!d) return delta ? REASON.bounds : REASON.nothing;
        moving.forEach(({ clip }) => { clip.offset = shifted(clip, d); });
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
            if (!isStill(project, next)) next.offset = shifted(next, d);
            next.start += d; next.duration -= d;
            clampFades(next);
        }
        return finish(project);
    }

    window.VideoStudioEdit = { REASON, ATTRIBUTE_GROUPS, speedOf, maxDurationFrom, editable, gapAt, overwrite, insertGap, insertClips, removeClips, rippleDelete, rippleDeleteGap, rippleTrim, roll, slip, slide };
})();
