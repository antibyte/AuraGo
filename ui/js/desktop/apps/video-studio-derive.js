(function () {
    'use strict';

    // Derived media (spec 4.4): reverse, freeze frame and export frame run as server jobs. A job changes the project only
    // when it succeeds, and then only the clips it was started for (they may have moved meanwhile). The new asset stays
    // in the library even when the edit does not apply.
    const P = window.VideoStudioParts;
    const E = () => window.VideoStudioEdit;
    const T = (...args) => P.T(...args);
    const showNotice = (...args) => P.showNotice(...args);

    const FREEZE_SECONDS = 2;
    const MAX_REVERSE_SECONDS = 300;

    function foundClip(s, clipId) { return clipId && s.project ? T().clipFor(s.project, clipId) : null; }
    function assetOf(s, clip) { return (s.project.assets || []).find(item => item.id === clip.asset_id) || null; }
    function insidePlayhead(s, clip) { return s.frame > clip.start && s.frame < clip.start + clip.duration; }
    // Go's MaxReverseFrames: the longest source range one reverse job accepts, in project frames.
    function maxReverseFrames(s) { return Math.floor(P.rateOf(s) * MAX_REVERSE_SECONDS + 1e-6); }
    // Whether one reverse job covers the source range of the clip and its same-asset link partners (E.reverseRange).
    function reverseFits(s, clipId) {
        const range = s.project ? E().reverseRange(s.project, clipId) : null;
        return !!range && range.frames <= maxReverseFrames(s);
    }
    function notifyRejected(s, error) {
        if (s.disposed) return;
        const code = error && error.body && error.body.code;
        if (code === 'reverse_range_too_long') showNotice(s, 'reverseTooLong', 'Reverse works on up to 5 minutes at a time.', true);
        else showNotice(s, 'jobRejected', 'This task cannot be started for this clip.', true);
    }
    // The edit a finished job will make is tried on a copy first (with a stand-in for the new asset), so a refusal that
    // does not depend on the wait (a partner on a locked track, a playhead inside a transition) comes before the job,
    // not after it with media nobody uses.
    function trial(s, standIn, edit) {
        const project = P.clone(s.project);
        project.assets.push(Object.assign({ name: '', duration_frames: 0 }, standIn));
        return edit(project);
    }
    function refuse(s, reason) {
        if (reason === E().REASON.nothing) showNotice(s, 'jobRejected', 'This task cannot be started for this clip.', true);
        else P.showReason(s, reason);
    }
    // Starts a job; a refused start becomes one notice. The success callback runs after the project is refreshed; it returns
    // false when its edit was refused (and says so), which keeps the generic job toast from replacing that message.
    async function start(s, payload, onSucceeded) {
        try {
            return await P.createJob(s, payload, s.projectId, s.projectEpoch, onSucceeded);
        } catch (error) {
            notifyRejected(s, error);
            return null;
        }
    }

    // Reverse (spec 4.4): the clip and its same-asset partners play the reversed asset once the job succeeds. The job
    // covers the union of the source ranges they read, so a J- or L-cut partner trimmed past the clip reverses too; the
    // partners are mirrored around the clip (E.reverseClips), which keeps picture and sound in sync.
    function reverseClip(s, clipId) {
        const found = foundClip(s, clipId);
        if (!found || s.readonly || found.track.locked || found.track.kind !== 'video') return false;
        const asset = assetOf(s, found.clip);
        if (!asset || asset.kind !== 'video') return false;
        const range = E().reverseRange(s.project, clipId);
        if (!range || range.frames > maxReverseFrames(s)) { showNotice(s, 'reverseTooLong', 'Reverse works on up to 5 minutes at a time.', true); return false; }
        const request = { rangeStart: range.rangeStart, frames: range.frames, sourceAssetId: asset.id };
        const tried = trial(s, { id: 'reverse-trial', kind: 'video', has_audio: !!asset.has_audio, duration_frames: range.frames },
            project => E().reverseClips(project, clipId, Object.assign({ assetId: 'reverse-trial' }, request)));
        if (tried !== true) { refuse(s, tried); return false; }
        start(s, { kind: 'reverse', asset_id: asset.id, offset: range.rangeStart, duration: range.frames, name: E().reversedName(asset.name, P.tr(s, 'reversedSuffix', 'reversed')) }, job => applyReverse(s, clipId, request, job));
        return true;
    }
    function applyReverse(s, clipId, request, job) {
        const asset = job && job.result && job.result.asset;
        if (!asset || !asset.id || !s.project) return;
        let result = true;
        const ok = P.applyEdit(s, 'reverse', project => (result = E().reverseClips(project, clipId, Object.assign({ assetId: asset.id }, request))));
        // No room for a moved partner is explained by applyEdit (reverseNoRoom); everything else changed while the job ran.
        if (!ok && result !== E().REASON.room) showNotice(s, 'clipChanged', 'The clip changed while the job ran. The new media is in your library.', true);
        return ok;
    }

    // A still job names the clip position (offset, clip frame and speed), so the server samples the source the way the
    // renderer does at that speed; frame is the source frame E.sourceFrameAt reports there, which the server checks. A
    // ramped clip sends its mean speed up to the frame (VideoStudioEffects.meanSpeed). The server then picks
    // floor(offset + S(t)), the frame the export shows, without knowing about ramps (index §4.18).
    function stillPayload(s, clip, point, purpose) {
        return { kind: 'still', asset_id: clip.asset_id, frame: E().sourceFrameAt(s.project, clip, point), offset: Number(clip.offset || 0),
            clip_frame: point - clip.start, speed: window.VideoStudioEffects.meanSpeed(clip, point - clip.start), purpose };
    }

    // Freeze frame (spec 4.4): the captured frame becomes a still inserted at the playhead on the clip's own track. The
    // request remembers the asset and source frame it captures; the edit applies only while the clip still shows that
    // frame at the cut.
    function freezeFrame(s, clipId) {
        const found = foundClip(s, clipId);
        if (!found || s.readonly || found.track.locked || found.track.kind !== 'video') return false;
        const asset = assetOf(s, found.clip);
        if (!asset || asset.kind !== 'video') return false;
        if (!insidePlayhead(s, found.clip)) { showNotice(s, 'freezeOutside', 'Move the playhead inside the clip first.', true); return false; }
        const point = s.frame, frame = E().sourceFrameAt(s.project, found.clip, point);
        const captured = { clip_id: T().newId('clip'), duration: Math.max(1, Math.round(P.rateOf(s) * FREEZE_SECONDS)), source_asset_id: asset.id, source_frame: frame };
        const tried = trial(s, { id: 'freeze-trial', kind: 'image', width: 1, height: 1 },
            project => E().freezeFrame(project, clipId, point, Object.assign({}, captured, { asset_id: 'freeze-trial' })));
        if (tried !== true) { refuse(s, tried); return false; }
        start(s, stillPayload(s, found.clip, point, 'freeze'), job => applyFreeze(s, clipId, point, captured, job));
        return true;
    }
    function applyFreeze(s, clipId, point, captured, job) {
        const asset = job && job.result && job.result.asset;
        if (!asset || !asset.id || !s.project) return;
        const ok = P.applyEdit(s, 'freeze frame', project => E().freezeFrame(project, clipId, point, Object.assign({}, captured, { asset_id: asset.id })));
        if (!ok) showNotice(s, 'clipChanged', 'The clip changed while the job ran. The new media is in your library.', true);
        return ok;
    }

    // Export frame as PNG (spec 4.4): the same job with purpose export; the download appears in the tasks list.
    function exportFrame(s, clipId) {
        const found = foundClip(s, clipId);
        if (!found || s.readonly) return false;
        const asset = assetOf(s, found.clip);
        if (!asset || asset.kind !== 'video') return false;
        if (!insidePlayhead(s, found.clip)) { showNotice(s, 'freezeOutside', 'Move the playhead inside the clip first.', true); return false; }
        start(s, stillPayload(s, found.clip, s.frame, 'export'), null);
        return true;
    }

    Object.assign(P, { reverseClip, freezeFrame, exportFrame, reverseFits });
})();
