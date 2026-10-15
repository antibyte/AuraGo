(function () {
    'use strict';

    // Every frame↔time conversion of the editor. Projects carry integer frames at fps (NTSC: fps * 1000 / 1001);
    // the formulas mirror internal/videostudio/time.go so both sides agree on every length.
    const FRAME_RATES = [
        { fps: 24, ntsc: true }, { fps: 24, ntsc: false }, { fps: 25, ntsc: false }, { fps: 30, ntsc: true },
        { fps: 30, ntsc: false }, { fps: 50, ntsc: false }, { fps: 60, ntsc: true }, { fps: 60, ntsc: false }
    ];
    const NTSC_RATES = [24, 30, 60];
    const MAX_SECONDS = 3600;

    function fraction(project) {
        const fps = Number(project && project.fps) || 30;
        return project && project.ntsc ? { num: fps * 1000, den: 1001 } : { num: fps, den: 1 };
    }
    function rate(project) { const f = fraction(project); return f.num / f.den; }
    function framesToSeconds(frames, project) { const f = fraction(project); return (Number(frames) || 0) * f.den / f.num; }
    function secondsToFrames(seconds, project) { return Math.round((Number(seconds) || 0) * rate(project)); }
    // Same formula as Go's Rate.FramesAt: a frame counts once its start lies inside the duration.
    function framesAt(durationUs, project) {
        const us = Number(durationUs) || 0;
        if (us <= 0) return 0;
        const f = fraction(project);
        return Math.max(1, Math.ceil(us * f.num / (f.den * 1e6) - 1e-3));
    }
    function maxFrames(project) { const f = fraction(project); return Math.floor(MAX_SECONDS * f.num / f.den); }
    function pad(value) { return String(value).padStart(2, '0'); }
    // m:ss.cc (h:mm:ss.cc from one hour on). Centiseconds are rounded first and then split, so a carry reaches the
    // seconds (59.94 fps frame 959 is 0:16.00) and parseTime reads every shown frame back to the same frame.
    function formatTime(frames, project) {
        const totalCs = Math.round(Math.max(0, framesToSeconds(Math.round(Number(frames) || 0), project)) * 100);
        const cs = totalCs % 100, whole = Math.floor(totalCs / 100);
        const h = Math.floor(whole / 3600), m = Math.floor(whole % 3600 / 60), s = whole % 60;
        const tail = pad(s) + '.' + pad(cs);
        return h ? `${h}:${pad(m)}:${tail}` : `${m}:${tail}`;
    }
    // Accepts seconds ("4,5"), m:ss(.cc) and h:mm:ss(.cc); returns frames or null.
    function parseTime(text, project) {
        const raw = String(text == null ? '' : text).trim().replace(',', '.');
        if (!raw || !/^\d+(:\d+){0,2}(\.\d+)?$/.test(raw)) return null;
        const parts = raw.split(':').map(Number);
        const seconds = parts.pop();
        if (parts.length && seconds >= 60) return null;
        if (parts.length === 2 && parts[1] >= 60) return null;
        let total = seconds, unit = 60;
        for (let i = parts.length - 1; i >= 0; i--) { total += parts[i] * unit; unit *= 60; }
        return Number.isFinite(total) ? secondsToFrames(total, project) : null;
    }
    function shortClock(frames, project) {
        const seconds = Math.floor(framesToSeconds(Math.max(0, Number(frames) || 0), project) + 1e-9);
        const h = Math.floor(seconds / 3600), m = Math.floor(seconds % 3600 / 60), s = seconds % 60;
        return h ? `${h}:${pad(m)}:${pad(s)}` : `${m}:${pad(s)}`;
    }
    function formatSeconds(frames, project, locale) {
        const value = Math.round(framesToSeconds(frames, project) * 10) / 10;
        return locale ? value.toLocaleString(locale) : String(value);
    }
    function rateLabel(project, locale) {
        const value = Math.round(rate(project) * 1000) / 1000;
        return (locale ? value.toLocaleString(locale) : String(value)) + ' fps';
    }
    // Converts every frame value to a new rate in place and returns the project. Clip edges are scaled (not
    // durations) so neighbouring clips stay gapless; asset lengths come from their microseconds like on the server;
    // transitions follow the rounded overlap (repairTransitions).
    function rescaleProject(project, fps, ntsc) {
        const target = { fps: Number(fps), ntsc: !!ntsc && NTSC_RATES.includes(Number(fps)) };
        const ratio = rate(target) / rate(project);
        const edge = value => Math.round((Number(value) || 0) * ratio);
        project.fps = target.fps;
        if (target.ntsc) project.ntsc = true; else delete project.ntsc;
        const assets = new Map();
        (project.assets || []).forEach(asset => {
            if (asset.kind !== 'image') asset.duration_frames = asset.duration_us > 0 ? framesAt(asset.duration_us, project) : Math.max(1, edge(asset.duration_frames));
            assets.set(asset.id, asset);
        });
        (project.tracks || []).forEach(track => (track.clips || []).forEach(clip => {
            const start = edge(clip.start), end = edge(clip.start + clip.duration);
            clip.start = start;
            clip.duration = Math.max(1, end - start);
            clip.offset = edge(clip.offset);
            const asset = assets.get(clip.asset_id);
            if (asset && asset.kind !== 'image') {
                // Source extent rule: offset + (duration - 1) * speed <= duration_frames - 1.
                const speed = clip.speed > 0 ? Number(clip.speed) : 1;
                clip.offset = Math.min(clip.offset, Math.max(0, asset.duration_frames - 1));
                clip.duration = Math.max(1, Math.min(clip.duration, Math.floor((asset.duration_frames - 1 - clip.offset) / speed + 1e-9) + 1));
            }
            clip.fade_in = Math.min(edge(clip.fade_in), clip.duration);
            clip.fade_out = Math.min(edge(clip.fade_out), clip.duration);
            if (clip.transition) clip.transition = Object.assign({}, clip.transition, { duration: Math.max(1, edge(clip.transition.duration)) });
        }));
        if (window.VideoStudioTimeline) window.VideoStudioTimeline.repairTransitions(project);
        return project;
    }

    window.VideoStudioTime = { FRAME_RATES, MAX_SECONDS, rate, framesToSeconds, secondsToFrames, framesAt, maxFrames, formatTime, parseTime, shortClock, formatSeconds, rateLabel, rescaleProject };
})();
