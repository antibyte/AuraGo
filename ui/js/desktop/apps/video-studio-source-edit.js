(function () {
    'use strict';

    // DOM-free source-monitor rules (spec §4.7). `plan` applies the 3-point rules to the timeline in/out, the
    // playhead and the source range. `placements` turns the planned edit into clips on the viewer's target tracks.
    // The edit itself runs through VideoStudioEdit (insertClips / overwrite) in the app part.
    const Time = () => window.VideoStudioTime;
    const T = () => window.VideoStudioTimeline;
    const STILL_SECONDS = 5;
    const MARKS_LIMIT = 200;
    const REASON = Object.freeze({ start: 'start', media: 'media', target: 'target', nothing: 'nothing' });
    const clamp = (value, lo, hi) => Math.max(lo, Math.min(hi, value));

    // Frames a source offers: a still has STILL_SECONDS, media its probed duration.
    function sourceFrames(project, asset) {
        if (!asset) return 0;
        if (asset.kind === 'image') return Math.max(1, Time().secondsToFrames(STILL_SECONDS, project));
        return Math.max(0, Math.round(Number(asset.duration_frames)) || 0);
    }
    // The source range [in, out) in frames: marks default to the whole source and are clamped to it.
    function sourceRange(project, asset, marks) {
        const frames = sourceFrames(project, asset), m = marks || {};
        const inFrame = m.in == null ? 0 : clamp(Math.round(Number(m.in)) || 0, 0, Math.max(0, frames - 1));
        const outFrame = m.out == null ? frames : clamp(Math.round(Number(m.out)) || 0, inFrame + 1, Math.max(inFrame + 1, frames));
        return { in: inFrame, out: outFrame, duration: outFrame - inFrame, frames };
    }
    // The 3-point rules (spec §4.7). Timeline in and out place the clip at in with out - in frames; the source in
    // decides where the source starts and the source out is ignored (nothing is stretched, so a source that cannot
    // cover the length is refused). Only in: at in with the range's length. Only out: backtimed to end at out.
    // Neither: at the playhead. Stills are unbounded. Returns { start, duration, offset } or { reason }.
    function plan(input) {
        const range = input.range, still = !!input.still;
        if (!still && range.frames < 1) return { reason: REASON.media };
        const offset = still ? 0 : range.in;
        const hasIn = Number.isFinite(input.timelineIn), hasOut = Number.isFinite(input.timelineOut);
        if (hasIn && hasOut) {
            const duration = input.timelineOut - input.timelineIn;
            if (duration < 1) return { reason: REASON.nothing };
            if (!still && range.in + duration > range.frames) return { reason: REASON.media };
            return { start: input.timelineIn, duration, offset };
        }
        if (hasIn) return { start: input.timelineIn, duration: range.duration, offset };
        if (hasOut) {
            const start = input.timelineOut - range.duration;
            return start < 0 ? { reason: REASON.start } : { start, duration: range.duration, offset };
        }
        return { start: Math.max(0, Math.round(Number(input.playhead)) || 0), duration: range.duration, offset };
    }
    // The clips a source edit places. part: 'both' (picture and sound of the item), 'picture' or 'sound'. Targets map
    // a kind to a track id ({ video, audio, overlay }); each track must accept the media. A video with sound and an
    // audio target becomes a linked pair (video detached from its embedded sound, audio clip, one new link id); a
    // video with sound and no audio target is one clip with its embedded sound; a picture-only video clip is
    // detached so its embedded sound stays silent.
    function placements(project, asset, options) {
        const opts = options || {}, targets = opts.targets || {}, part = opts.part || 'both';
        const video = asset.kind === 'video', still = asset.kind === 'image';
        const picture = (part === 'both' || part === 'picture') && (video || still);
        const sound = (part === 'both' || part === 'sound') && (asset.kind === 'audio' || (video && !!asset.has_audio));
        if (!picture && !sound) return { reason: REASON.media };
        const trackOf = id => (id ? (project.tracks || []).find(track => track.id === id) || null : null);
        const accepted = track => (track && T().accepts(track, asset) ? track : null);
        const audioTrack = sound ? accepted(trackOf(targets.audio)) : null;
        const pictureTrack = picture ? (accepted(trackOf(targets.video)) || accepted(trackOf(targets.overlay))) : null;
        if (picture && !pictureTrack) return { reason: REASON.target };
        if (!picture && !audioTrack) return { reason: REASON.target };
        const result = [], created = [];
        const clipOn = () => Object.assign(T().makeClip(asset, opts.start, undefined, project), { duration: opts.duration, offset: still ? 0 : (opts.offset || 0) });
        const put = (track, clip) => {
            let entry = result.find(item => item.trackId === track.id);
            if (!entry) result.push(entry = { trackId: track.id, clips: [] });
            entry.clips.push(clip);
            created.push(clip.id);
        };
        if (!picture) {
            put(audioTrack, clipOn());
            return { placements: result, created };
        }
        const clip = clipOn(), paired = sound && !!audioTrack;
        if (video && asset.has_audio && (paired || part === 'picture')) clip.audio_detached = true;
        if (paired) {
            clip.link = T().newId('link');
            const partner = clipOn();
            partner.link = clip.link;
            put(pictureTrack, clip);
            put(audioTrack, partner);
        } else {
            put(pictureTrack, clip);
        }
        return { placements: result, created };
    }
    // Keeps at most `limit` entries; the entries changed longest ago (smallest `at`) go first.
    function pruneMarks(data, limit) {
        if (!data || typeof data !== 'object') return {};
        const max = limit || MARKS_LIMIT, keys = Object.keys(data);
        if (keys.length <= max) return data;
        keys.sort((a, b) => ((data[a] && data[a].at) || 0) - ((data[b] && data[b].at) || 0)).slice(0, keys.length - max).forEach(key => { delete data[key]; });
        return data;
    }
    // A stored entry: marks in seconds (rate-robust), `at` the time of the last change.
    function entryFor(project, marks, now) {
        const seconds = frame => (frame == null ? null : Math.round(Time().framesToSeconds(frame, project) * 1e6) / 1e6);
        return { in: seconds(marks.in), out: seconds(marks.out), at: now };
    }
    // Frames of a stored entry at the project rate. Values outside the source read as unset.
    function marksFrom(entry, project, frames) {
        const toFrame = value => (value == null || !Number.isFinite(Number(value)) ? null : Time().secondsToFrames(Number(value), project));
        const inFrame = entry ? toFrame(entry.in) : null, outFrame = entry ? toFrame(entry.out) : null;
        const inOk = inFrame != null && inFrame >= 0 && inFrame < frames ? inFrame : null;
        const outOk = outFrame != null && outFrame > (inOk || 0) && outFrame <= frames ? outFrame : null;
        return { in: inOk, out: outOk };
    }

    window.VideoStudioSourceEdit = { REASON, STILL_SECONDS, MARKS_LIMIT, sourceFrames, sourceRange, plan, placements, pruneMarks, entryFor, marksFrom };
})();
