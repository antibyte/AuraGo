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
    // The source frame the renderer shows at the playhead: offset + (playhead - start) × speed, rounded down.
    function sourceFrameAt(s, clip, asset) {
        const frame = Math.floor(Number(clip.offset || 0) + (s.frame - clip.start) * E().speedOf(clip) + 1e-9);
        return Math.max(0, Math.min(Number(asset.duration_frames) - 1, frame));
    }
    // Reverse covers the source frames the clip reads: [offset, offset + floor((duration - 1) × speed)].
    function reverseRange(clip) {
        return { offset: clip.offset, frames: Math.floor((clip.duration - 1) * E().speedOf(clip) + 1e-9) + 1 };
    }
    function maxReverseFrames(s) { return Math.floor(P.rateOf(s) * MAX_REVERSE_SECONDS + 1e-6); }
    function notifyRejected(s, error) {
        if (s.disposed) return;
        const code = error && error.body && error.body.code;
        if (code === 'reverse_range_too_long') showNotice(s, 'reverseTooLong', 'Reverse works on up to 5 minutes at a time.', true);
        else showNotice(s, 'jobRejected', 'This task cannot be started for this clip.', true);
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

    // Reverse (spec 4.4): the clip and its same-asset partners play the reversed asset once the job succeeds.
    function reverseClip(s, clipId) {
        const found = foundClip(s, clipId);
        if (!found || s.readonly || found.track.locked || found.track.kind !== 'video') return false;
        const asset = assetOf(s, found.clip);
        if (!asset || asset.kind !== 'video') return false;
        const range = reverseRange(found.clip);
        if (range.frames > maxReverseFrames(s)) { showNotice(s, 'reverseTooLong', 'Reverse works on up to 5 minutes at a time.', true); return false; }
        start(s, { kind: 'reverse', asset_id: asset.id, offset: range.offset, duration: range.frames }, job => applyReverse(s, clipId, range, job));
        return true;
    }
    function applyReverse(s, clipId, range, job) {
        const asset = job && job.result && job.result.asset;
        if (!asset || !asset.id || !s.project) return;
        const ok = P.applyEdit(s, 'reverse', project => E().reverseClips(project, clipId, { assetId: asset.id, rangeStart: range.offset, frames: range.frames }));
        if (!ok) showNotice(s, 'clipChanged', 'The clip changed while the job ran. The new media is in your library.', true);
        return ok;
    }

    // Freeze frame (spec 4.4): the captured frame becomes a still inserted at the playhead on the clip's own track.
    function freezeFrame(s, clipId) {
        const found = foundClip(s, clipId);
        if (!found || s.readonly || found.track.locked || found.track.kind !== 'video') return false;
        const asset = assetOf(s, found.clip);
        if (!asset || asset.kind !== 'video') return false;
        if (!insidePlayhead(s, found.clip)) { showNotice(s, 'freezeOutside', 'Move the playhead inside the clip first.', true); return false; }
        const point = s.frame, frame = sourceFrameAt(s, found.clip, asset);
        const captured = { clip_id: T().newId('clip'), duration: Math.max(1, Math.round(P.rateOf(s) * FREEZE_SECONDS)) };
        start(s, { kind: 'still', asset_id: asset.id, frame, purpose: 'freeze' }, job => applyFreeze(s, clipId, point, captured, job));
        return true;
    }
    function applyFreeze(s, clipId, point, captured, job) {
        const asset = job && job.result && job.result.asset;
        if (!asset || !asset.id || !s.project) return;
        const ok = P.applyEdit(s, 'freeze frame', project => E().freezeFrame(project, clipId, point, { asset_id: asset.id, clip_id: captured.clip_id, duration: captured.duration }));
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
        start(s, { kind: 'still', asset_id: asset.id, frame: sourceFrameAt(s, found.clip, asset), purpose: 'export' }, null);
        return true;
    }

    Object.assign(P, { reverseClip, freezeFrame, exportFrame });
})();
