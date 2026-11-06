// Storymaker app handoff to Video Studio. buildTracks turns the imported film and its stems into the tracks of a Video Studio
// project: the picture on one video track with its audio detached, then one audio track for each stem that exists. It is pure, and
// a Node test checks its output against the Go validator. Classic script, publishes S.handoff.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};
    const STEMS = ['narration', 'music', 'sfx'];

    // The fields follow Clip in internal/videostudio/model.go and makeClip in video-studio-timeline.js. Validate refuses one frame
    // past the asset, so a duration never exceeds the asset or the film.
    function clipOf(asset, frameCount) {
        return {
            id: 'clip-' + S.uuid(), asset_id: asset.id, start: 0, offset: 0,
            duration: Math.min(asset.duration_frames, frameCount),
            x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, volume: 1,
            fade_in: 0, fade_out: 0, fit: 'contain'
        };
    }

    function trackOf(kind, name, clip) {
        return { id: 'track-' + S.uuid(), kind, name, muted: false, hidden: false, locked: false, clips: [clip] };
    }

    // buildTracks(project, assets, frameCount, names): assets maps video, narration, music and sfx to asset ids of project.assets;
    // names holds the track names under the same keys. A stem without an asset is skipped, because a silent stem has no file.
    function buildTracks(project, assets, frameCount, names) {
        const byId = new Map((project.assets || []).map(asset => [asset.id, asset]));
        const video = byId.get(assets.video);
        if (!video) throw new Error('The imported video is missing from the project.');
        const tracks = [trackOf('video', names.video, Object.assign(clipOf(video, frameCount), { audio_detached: true }))];
        STEMS.forEach(name => {
            const asset = byId.get(assets[name]);
            if (asset) tracks.push(trackOf('audio', names[name], clipOf(asset, frameCount)));
        });
        return tracks;
    }

    // ---- the run --------------------------------------------------------------------------------------------------------------

    const API = '/api/desktop/video-studio';
    const POLL_MS = 500;
    const POLL_LIMIT_MS = 120000;
    const MAX_NAME_CHARS = 120;
    const PROBE_FAILED = 'handoff_probe_failed';

    // One Video Studio request: {body, etag}. A failure is thrown in the app's error shape (S.errors).
    async function call(method, path, headers, body) {
        const init = { method, headers: Object.assign({}, headers || {}), credentials: 'same-origin', cache: 'no-store' };
        if (body !== undefined) {
            init.headers['Content-Type'] = 'application/json';
            init.body = JSON.stringify(body);
        }
        let response;
        try {
            response = await window.fetch(API + path, init);
        } catch (err) {
            throw S.errors.normalize({ message: String(err && err.message || err) });
        }
        let answer = null;
        try { answer = await response.json(); } catch (_) { answer = null; }
        if (!response.ok) throw S.errors.fromResponse(response.status, answer, response.headers.get('retry-after'));
        const data = answer && typeof answer === 'object' ? answer : {};
        return { body: data, etag: response.headers.get('ETag') || String(data.etag || '') };
    }

    function refused(message, code) {
        return { stm: true, status: 0, code: code || 'handoff_failed', message, issues: [], network: false };
    }

    function wait(ms) {
        return new Promise(resolve => setTimeout(resolve, ms));
    }

    // Waits for the probe jobs in turn. They share one limit, so the whole wait stays within 120 s.
    async function waitForProbes(inst, jobIds) {
        let waited = 0;
        for (const id of jobIds) {
            for (;;) {
                if (inst.disposed) throw refused('');
                if (waited >= POLL_LIMIT_MS) throw refused(inst.t('storymaker.handoff.timeout'));
                await wait(POLL_MS);
                waited += POLL_MS;
                const { body } = await call('GET', '/jobs/' + encodeURIComponent(id));
                if (body.status === 'succeeded') break;
                if (body.status !== 'queued' && body.status !== 'running') throw refused(inst.t('storymaker.handoff.import_failed'), PROBE_FAILED);
            }
        }
    }

    // Canvas and rate follow the story. Video Studio accepts every Storymaker format, so nothing is rescaled.
    function projectSpec(inst) {
        const story = inst.story || {};
        const format = story.format || {};
        const size = S.model.exportSize(format.aspect);
        const title = String(story.title || (inst.project && inst.project.title) || '').trim() || 'Storymaker';
        return { name: Array.from(title).slice(0, MAX_NAME_CHARS).join('').trim(), width: size[0], height: size[1], fps: Number(format.fps) || 30, ntsc: false };
    }

    // The client owns only these asset fields. The server fills in everything else from its media records.
    function keptAsset(asset) {
        const kept = { id: asset.id, name: asset.name };
        if (asset.folder) kept.folder = asset.folder;
        return kept;
    }

    // Imports the MP4 and each stem into the project, waits for the probes, reads the project back for its ETag and assets and writes
    // the tracks with If-Match.
    async function fillProject(inst, job, projectId) {
        const output = job.output || {};
        const stems = output.stems || {};
        const sources = [['video', output.path]].concat(STEMS.filter(name => stems[name]).map(name => [name, stems[name]]));
        const base = '/projects/' + encodeURIComponent(projectId);
        const assets = {};
        const probes = [];
        for (const [name, source] of sources) {
            const imported = await call('POST', base + '/media', { 'Idempotency-Key': 'storymaker-' + job.id + '-' + name }, { source_path: source, folder: '' });
            assets[name] = imported.body.asset.id;
            probes.push(imported.body.job.id);
        }
        await waitForProbes(inst, probes);
        const fresh = await call('GET', base);
        const project = fresh.body.project;
        const tracks = buildTracks(project, assets, Number(job.frames_total) || 0, {
            video: inst.t('storymaker.handoff.track_video'),
            narration: inst.t('storymaker.handoff.track_narration'),
            music: inst.t('storymaker.handoff.track_music'),
            sfx: inst.t('storymaker.handoff.track_sfx')
        });
        await call('PUT', base, { 'If-Match': fresh.etag }, Object.assign({}, project, { version: 3, assets: project.assets.map(keptAsset), tracks }));
    }

    // run(inst, job, projects) -> {projectId}: creates the project, fills it and opens Video Studio on it. projects is the panel's Map from
    // job id to Video Studio project, and it is optional. It keeps the project of a failed try, so a retry fills that one and adds no
    // other: the import keys are per project, so the repeated imports replay the first ones and double no asset. A finished handoff
    // drops the entry, so a later click starts a new project and cannot overwrite edits made in Video Studio.
    async function run(inst, job, projects) {
        const kept = projects || new Map();
        const reused = kept.get(job.id);
        const projectId = reused || (await call('POST', '/projects', null, projectSpec(inst))).body.id;
        kept.set(job.id, projectId);
        try {
            await fillProject(inst, job, projectId);
        } catch (err) {
            // A 404 on a kept project means it was deleted in Video Studio. A failed probe replays as failed for the same key, so only a
            // new project can try the file again.
            const gone = reused && err && err.status === 404;
            if (gone || (err && err.code === PROBE_FAILED)) kept.delete(job.id);
            if (gone) return run(inst, job, kept);
            throw err;
        }
        inst.ctx.openApp('video-studio', { projectId });
        kept.delete(job.id);
        return { projectId };
    }

    // The status half of canOffer: Video Studio can take a film now. The export dialog asks this before a render starts.
    function studioReady(vsStatus) {
        const status = vsStatus || {};
        return !!(status.enabled && status.desktop_enabled && status.ffmpeg_ready && !status.read_only);
    }

    // A stem that failed to save has no track, and the film's own mix is detached from the picture, so that sound would be missing.
    function stemFailed(job) {
        return !!job && Array.isArray(job.warnings) && job.warnings.some(warning => warning && warning.code === 'stem_failed');
    }

    function canOffer(job, vsStatus) {
        return !!(job && job.status === 'succeeded' && job.output && job.output.stems && !stemFailed(job) && studioReady(vsStatus));
    }

    // The Video Studio status object that canOffer reads.
    async function status() {
        return (await call('GET', '/status')).body;
    }

    S.handoff = { canOffer, studioReady, stemFailed, run, status, buildTracks };
})();
