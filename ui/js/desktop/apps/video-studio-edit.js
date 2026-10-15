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

    window.VideoStudioEdit = { REASON, ATTRIBUTE_GROUPS, speedOf, maxDurationFrom, editable, overwrite, insertGap, insertClips, removeClips };
})();
