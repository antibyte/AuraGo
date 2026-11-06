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

    S.handoff = { buildTracks };
})();
