(function () {
    'use strict';

    // DOM-free edit operations (spec §4.2). Each operation changes the given project in place and returns
    // true, or a REASON code; after a refusal the caller discards the project (the app runs operations
    // inside mutate, which restores its snapshot). Locked tracks never change.
    const T = () => window.VideoStudioTimeline;
    const Time = () => window.VideoStudioTime;
    const REASON = Object.freeze({ locked: 'locked', media: 'media', bounds: 'bounds', nothing: 'nothing', limit: 'limit', kind: 'kind', transition: 'transition', tracks: 'tracks', link: 'link' });
    const MAX_LINK_CLIPS = 16;
    const ATTRIBUTE_GROUPS = { picture: ['x', 'y', 'width', 'height', 'rotation', 'opacity', 'fit'], sound: ['volume', 'fade_in', 'fade_out'], speed: ['speed'] };
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
        if (link && new Set(items.concat(linkMembers(project, link)).map(({ clip }) => clip.id)).size > MAX_LINK_CLIPS) return REASON.link;
        const previous = new Set(items.map(({ clip }) => clip.link).filter(Boolean));
        items.forEach(({ clip }) => { if (link) clip.link = link; else delete clip.link; });
        previous.forEach(id => { const rest = linkMembers(project, id); if (rest.length === 1 && !rest[0].track.locked) delete rest[0].clip.link; });
        return finish(project);
    }

    // ---- Blade with links (spec 4.2): scope.trackIds or scope.clipIds pick the clips under the frame; with
    // scope.linked their link partners under the frame are cut too. The left part keeps id, link and fade in; the
    // right part gets fade out and the outgoing transition. Right parts of one cut group share a new link id (a lone
    // right part is unlinked), so the cut leaves two groups. ----
    function blade(project, frame, scope, out) {
        const wanted = scope.clipIds ? new Set(scope.clipIds) : null, trackIds = scope.trackIds ? new Set(scope.trackIds) : null;
        const under = clip => clip.start < frame && frame < endOf(clip);
        const picks = [];
        project.tracks.forEach(track => track.clips.forEach(clip => {
            if (under(clip) && (!trackIds || trackIds.has(track.id)) && (!wanted || wanted.has(clip.id))) picks.push({ track, clip });
        }));
        if (scope.linked) {
            const links = new Set(picks.map(({ clip }) => clip.link).filter(Boolean)), seen = new Set(picks.map(({ clip }) => clip.id));
            project.tracks.forEach(track => track.clips.forEach(clip => {
                if (under(clip) && clip.link && links.has(clip.link) && !seen.has(clip.id)) { picks.push({ track, clip }); seen.add(clip.id); }
            }));
        }
        const created = [], rights = new Map();
        let locked = false, transition = false;
        picks.forEach(({ track, clip }) => {
            if (track.locked) { locked = true; return; }
            if (isCutInTransition(track, frame)) { transition = true; return; }
            const right = piece(project, clip, frame, endOf(clip), T().newId('clip'));
            Object.assign(clip, piece(project, clip, clip.start, frame, clip.id));
            track.clips.push(right);
            created.push(right.id);
            if (clip.link) { if (!rights.has(clip.link)) rights.set(clip.link, []); rights.get(clip.link).push(right); }
        });
        rights.forEach(parts => {
            const link = parts.length > 1 ? T().newId('link') : '';
            parts.forEach(right => { if (link) right.link = link; else delete right.link; });
        });
        if (out) out.created = created;
        if (!created.length) return transition ? REASON.transition : locked ? REASON.locked : REASON.nothing;
        return finish(project);
    }

    // Pastes clipboard data at `at` (overwrite or insert). options.targets ({ video, audio, overlay } → track id)
    // puts each kind's clips on that track and the following ones of the kind; without it clips return to the
    // tracks they came from. A group pasted with two or more clips gets one new id; a lone copy is unlinked.
    // options.out.created lists the new clip ids.
    function paste(project, board, at, mode, options) {
        const opts = options || {};
        if (!board || !board.entries.length) return REASON.nothing;
        const counts = new Map(), remap = new Map(), placements = [], created = [];
        board.entries.forEach(({ clip }) => { if (clip.link) counts.set(clip.link, (counts.get(clip.link) || 0) + 1); });
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
            let placement = placements.find(item => item.trackId === track.id);
            if (!placement) placements.push(placement = { trackId: track.id, clips: [] });
            placement.clips.push(clip);
            created.push(clip.id);
        }
        const result = mode === 'insert' ? insertClips(project, at, placements) : overwrite(project, placements);
        if (result === true && opts.out) opts.out.created = created;
        return result;
    }

    // Copies attribute groups from `source` (a clip object) to the listed clips. picture skips audio clips. The speed
    // group skips stills and goes through setSpeed for the clips and their link partners, so it follows the timing
    // rules (ripple by default).
    function pasteAttributes(project, source, ids, groups) {
        const items = editable(project, ids), wanted = groups || [];
        if (!items.length) return refusal(project, ids);
        let changed = false;
        items.forEach(({ track, clip }) => {
            wanted.forEach(group => {
                if (group === 'picture' && track.kind === 'audio') return;
                (ATTRIBUTE_GROUPS[group] || []).forEach(key => { if (key !== 'speed' && source[key] !== undefined) { clip[key] = clone(source[key]); changed = true; } });
            });
            clampFades(clip);
        });
        if (wanted.includes('speed')) {
            const timed = withLinks(project, items.map(({ clip }) => clip.id)).filter(id => !isStill(project, T().clipFor(project, id).clip));
            if (timed.length) {
                const result = setSpeed(project, timed, speedOf(source), false);
                if (result !== true) return result;
                changed = true;
            }
        }
        return changed ? finish(project) : REASON.nothing;
    }

    // ---- Speed (spec 4.4). The default keeps the source range and changes the duration to round(duration × old / new)
    // (at most what the media holds at the new speed), rippling later clips on each track. keepDuration keeps the
    // duration and changes the source range. Stills have no speed; every clip keeps the source extent rule, its
    // transitions and the project length. ----
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
            const duration = keepDuration ? clip.duration : Math.max(1, Math.min(fits, Math.round(clip.duration * speedOf(clip) / speed)));
            if (!(duration <= fits)) return REASON.media;
            if (duration < minDuration(track, clip)) return REASON.bounds;
            plans.push({ track, clip, duration });
        }
        plans.forEach(({ track, clip, duration }) => {
            const delta = duration - clip.duration;
            if (delta) track.clips.filter(item => item !== clip && item.start > clip.start).forEach(item => { item.start += delta; });
            clip.duration = duration;
            if (speed === 1) delete clip.speed; else clip.speed = speed;
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
        if ((clip.link ? linkMembers(project, clip.link).length : 1) + 1 > MAX_LINK_CLIPS) return REASON.link;
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
        delete audio.audio_detached;
        clip.link = link;
        clip.audio_detached = true;
        track.clips.push(audio);
        return finish(project);
    }
    // Reattach: removes the audio clip a detached clip owns and clears the flag. A group left with the video clip ends.
    function reattachAudio(project, clipId) {
        const found = T().clipFor(project, clipId);
        if (!found || !found.clip.audio_detached) return REASON.nothing;
        if (found.track.locked) return REASON.locked;
        const partnerId = audioPartner(project, clipId);
        if (!partnerId) return REASON.nothing;
        const partner = T().clipFor(project, partnerId);
        if (partner.track.locked) return REASON.locked;
        partner.track.clips = partner.track.clips.filter(item => item !== partner.clip);
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

    // ---- Derived media (spec 4.4). reversed = { assetId, rangeStart, frames }: the reversed asset holds the source
    // range [rangeStart, rangeStart + frames) backwards. A clip reads it from the mirrored offset
    // frames - 1 - (offset - rangeStart) - (duration - 1) * speed, so it plays the same source frames in reverse.
    // Members of the clip's group that use the same asset switch too; a member outside the range refuses. ----
    function reverseClips(project, clipId, reversed) {
        const found = T().clipFor(project, clipId);
        if (!found) return REASON.nothing;
        if (found.track.locked) return REASON.locked;
        const target = (project.assets || []).find(asset => asset.id === reversed.assetId);
        if (!target) return REASON.media;
        const source = found.clip.asset_id;
        const members = found.clip.link ? linkMembers(project, found.clip.link).filter(({ clip }) => clip.asset_id === source) : [found];
        const plans = [];
        for (const { track, clip } of members) {
            if (track.locked) return REASON.locked;
            if (!T().accepts(track, target) || (clip.audio_detached && !target.has_audio)) return REASON.media;
            const speed = speedOf(clip), last = clip.offset + (clip.duration - 1) * speed;
            if (clip.offset < reversed.rangeStart || last > reversed.rangeStart + reversed.frames - 1 + 1e-9) return REASON.media;
            plans.push({ clip, offset: Math.max(0, Math.floor(reversed.frames - 1 - (clip.offset - reversed.rangeStart) - (clip.duration - 1) * speed + 1e-9)) });
        }
        plans.forEach(({ clip, offset }) => { clip.asset_id = reversed.assetId; clip.offset = offset; });
        if (plans.some(({ clip }) => clip.duration > maxDurationFrom(project, clip, clip.offset))) return REASON.media;
        return finish(project);
    }
    // Freeze frame (spec 4.4): the clip and its link partners are cut at the playhead, then a still of the captured
    // frame (the clip's geometry) is inserted on the clip's own track. Insert rules move the later clips.
    // captured = { asset_id, clip_id, duration } (duration in frames).
    function freezeFrame(project, clipId, frame, captured) {
        const found = T().clipFor(project, clipId);
        if (!found) return REASON.nothing;
        const { track, clip } = found;
        if (track.locked) return REASON.locked;
        if (track.kind !== 'video') return REASON.kind;
        if (!(frame > clip.start && frame < endOf(clip))) return REASON.nothing;
        if (!(project.assets || []).some(asset => asset.id === captured.asset_id && asset.kind === 'image')) return REASON.media;
        const cut = blade(project, frame, { clipIds: [clipId], linked: true }, {});
        if (cut !== true) return cut;
        const still = {
            id: captured.clip_id, asset_id: captured.asset_id, start: frame, offset: 0, duration: Math.max(1, Math.round(captured.duration)),
            x: clip.x, y: clip.y, width: clip.width, height: clip.height, rotation: clip.rotation, opacity: clip.opacity,
            volume: 1, fade_in: 0, fade_out: 0, fit: clip.fit, text: '', text_style: null, transition: null
        };
        return insertClips(project, frame, [{ trackId: track.id, clips: [still] }]);
    }

    window.VideoStudioEdit = { REASON, ATTRIBUTE_GROUPS, speedOf, maxDurationFrom, editable, gapAt, overwrite, insertGap, insertClips, removeClips, rippleDelete, rippleDeleteGap, rippleTrim, roll, slip, slide, blade, moveClips, copyClips, paste, duplicateClips, pasteAttributes, clipsFrom, neighborAt, snapPoints, snapDelta, linkGroup, withLinks, linkedEdges, audioPartner, setLink, setSpeed, detachAudio, reattachAudio, setDisabled, setLabel, reverseClips, freezeFrame };
})();
