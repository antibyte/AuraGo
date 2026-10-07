(function () {
    'use strict';

    const FPS = 30;
    const clamp = (value, min, max) => Math.max(min, Math.min(max, value));

    function mediaURL(asset) {
        const raw = String(asset && asset.media_url || '');
        if (!raw || raw.startsWith('//')) return '';
        try { const url = new URL(raw, location.origin); return url.origin === location.origin ? url.href : ''; }
        catch (_) { return ''; }
    }

    function mount(canvas, getProject, getFrame, setFrame, onPlayState) {
        const context = canvas.getContext('2d', { alpha: false });
        const media = new Map();
        const sourceNodes = new Map(), gainNodes = new Map();
        let audioContext = null;
        let frame = 0, playing = false, raf = 0, playStartAt = 0, playStartFrame = 0, disposed = false;

        function release(clipId, el) {
            if (!el) return;
            try {
                if (el instanceof HTMLMediaElement) { el.pause(); el.removeAttribute('src'); el.load(); }
                else el.removeAttribute('src');
            } catch (_) {}
            const source = sourceNodes.get(el), gain = gainNodes.get(el);
            if (source) { try { source.disconnect(); } catch (_) {} sourceNodes.delete(el); }
            if (gain) { try { gain.disconnect(); } catch (_) {} gainNodes.delete(el); }
            if (media.get(clipId) === el) media.delete(clipId);
        }

        function entry(asset, clip) {
            if (!asset || !clip || !asset.id || !clip.id) return null;
            const key = clip.id, src = mediaURL(asset);
            if (!src) return null;
            if (media.has(key) && media.get(key).__vsAssetId === asset.id) return media.get(key);
            if (media.has(key)) release(key, media.get(key));
            let el;
            if (asset.kind === 'image') {
                el = new Image(); el.decoding = 'async';
                el.addEventListener('load', () => { if (media.get(key) === el) paint(frame); });
                el.src = src;
            } else {
                el = document.createElement(asset.kind === 'audio' ? 'audio' : 'video');
                el.preload = 'metadata'; el.playsInline = true; el.src = src;
                el.addEventListener('loadeddata', () => { if (media.get(key) === el) paint(frame); });
                el.addEventListener('seeked', () => { if (media.get(key) === el && !playing) paint(frame); });
                el.load();
            }
            el.__vsAssetId = asset.id;
            media.set(key, el);
            return el;
        }

        function ensureAudioNode(el) {
            if (!audioContext || !(el instanceof HTMLMediaElement)) return null;
            if (gainNodes.has(el)) return gainNodes.get(el);
            try {
                const source = audioContext.createMediaElementSource(el), gain = audioContext.createGain();
                source.connect(gain); gain.connect(audioContext.destination);
                sourceNodes.set(el, source); gainNodes.set(el, gain);
                return gain;
            } catch (_) { return null; }
        }

        function assetFor(project, clip) { return (project.assets || []).find(asset => asset.id === clip.asset_id); }
        function activeAt(clip, at) { return at >= Number(clip.start || 0) && at < Number(clip.start || 0) + Number(clip.duration || 0); }
        function sourcePosition(clip, at) { return Math.max(0, (Number(clip.offset || 0) + at - Number(clip.start || 0)) / FPS); }

        function transitionIn(track, clip, at) {
            const sorted = (track.clips || []).slice().sort((a, b) => a.start - b.start);
            const index = sorted.findIndex(item => item.id === clip.id);
            if (index <= 0) return null;
            const prior = sorted[index - 1], duration = Number(prior.transition && prior.transition.duration || 0);
            if (!duration || !prior.transition || prior.start + prior.duration - clip.start !== duration) return null;
            return { type: prior.transition.type, progress: clamp((at - clip.start) / duration, 0, 1) };
        }

        function transitionOut(track, clip, at) {
            const transition = clip.transition, duration = Number(transition && transition.duration || 0);
            if (!duration) return null;
            const start = clip.start + clip.duration - duration;
            return at >= start ? { type: transition.type, progress: clamp((at - start) / duration, 0, 1) } : null;
        }

        function syncMedia(project, at, isPlaying) {
            const active = new Set(), keep = new Set(), preloadUntil = at + FPS * 2;
            (project.tracks || []).forEach(track => {
                if (track.hidden) return;
                (track.clips || []).forEach(clip => {
                    const current = activeAt(clip, at), near = !current && clip.start > at && clip.start <= preloadUntil;
                    if (!current && !near) return;
                    const asset = assetFor(project, clip), el = entry(asset, clip);
                    if (!asset || !el) return;
                    keep.add(clip.id);
                    if (near) {
                        if (el instanceof HTMLMediaElement) { el.preload = 'metadata'; if (!el.paused) el.pause(); }
                        return;
                    }
                    active.add(clip.id);
                    if (el instanceof HTMLMediaElement) el.preload = 'auto';
                    if (asset.kind === 'image') return;
                    const local = Math.max(0, at - clip.start);
                    const fadeIn = clip.fade_in > 0 ? clamp(local / clip.fade_in, 0, 1) : 1;
                    const fadeOut = clip.fade_out > 0 ? clamp((clip.duration - local) / clip.fade_out, 0, 1) : 1;
                    const audioAllowed = !track.muted && !track.hidden && (track.kind === 'audio' || asset.kind === 'video' && asset.has_audio);
                    const targetGain = audioAllowed ? clamp(Number(clip.volume == null ? 1 : clip.volume) * fadeIn * fadeOut, 0, 4) : 0;
                    const gain = ensureAudioNode(el);
                    if (gain) { gain.gain.value = targetGain; el.volume = 1; el.muted = false; }
                    else { el.volume = clamp(targetGain, 0, 1); if (asset.kind === 'video') el.muted = !audioAllowed; }
                    const desired = sourcePosition(clip, at);
                    if (Math.abs((el.currentTime || 0) - desired) > (isPlaying ? 0.38 : 0.025)) {
                        try { el.currentTime = desired; } catch (_) { /* source metadata is loading */ }
                    }
                    if (isPlaying && el.paused) el.play().catch(() => {});
                    else if (!isPlaying && !el.paused) el.pause();
                });
            });
            Array.from(media.entries()).forEach(([clipId, el]) => {
                if (!keep.has(clipId)) release(clipId, el);
                else if (!active.has(clipId) && el instanceof HTMLMediaElement && !el.paused) el.pause();
            });
        }

        function drawAsset(project, track, clip, at, incoming) {
            const asset = assetFor(project, clip), el = entry(asset, clip);
            if (!asset || !el) return;
            if (asset.kind !== 'image') {
                const desired = sourcePosition(clip, at);
                if (Math.abs((el.currentTime || 0) - desired) > (playing ? 0.38 : 0.025)) {
                    try { el.currentTime = desired; } catch (_) { /* source metadata is loading */ }
                }
            }
            const iw = Number(el.videoWidth || el.naturalWidth || asset.width || 0);
            const ih = Number(el.videoHeight || el.naturalHeight || asset.height || 0);
            if (asset.kind === 'image' && (!el.naturalWidth || !el.naturalHeight)) return;
            if (!iw || !ih || (el instanceof HTMLVideoElement && el.readyState < 2)) return;
            const cw = canvas.width, ch = canvas.height;
            const x = Number(clip.x || 0) * cw, y = Number(clip.y || 0) * ch;
            const w = Number(clip.width || 1) * cw, h = Number(clip.height || 1) * ch;
            const scale = clip.fit === 'cover' ? Math.max(w / iw, h / ih) : Math.min(w / iw, h / ih);
            const dw = iw * scale, dh = ih * scale;
            const radians = Number(clip.rotation || 0) * Math.PI / 180;
            const outgoing = transitionOut(track, clip, at);
            const entering = incoming || transitionIn(track, clip, at);
            let alpha = clamp(Number(clip.opacity == null ? 1 : clip.opacity), 0, 1);
            if (outgoing && outgoing.type === 'black') alpha *= outgoing.progress < 0.5 ? 1 - outgoing.progress * 2 : 0;
            if (entering && entering.type === 'dissolve') alpha *= entering.progress;
            if (entering && entering.type === 'black') alpha *= entering.progress < 0.5 ? 0 : (entering.progress - 0.5) * 2;
            if (outgoing && outgoing.type === 'dissolve') alpha *= 1; // draw A underneath a partially revealed B for a linear crossfade.
            context.save();
            context.globalAlpha = alpha;
            context.translate(x + w / 2, y + h / 2);
            context.rotate(radians);
            if (clip.fit === 'cover') {
                context.beginPath(); context.rect(-w / 2, -h / 2, w, h); context.clip();
            }
            if (entering && (entering.type === 'wipeleft' || entering.type === 'wiperight')) {
                context.beginPath();
                const reveal = w * entering.progress;
                context.rect(entering.type === 'wipeleft' ? -w / 2 : w / 2 - reveal, -h / 2, reveal, h);
                context.clip();
            }
            context.drawImage(el, -dw / 2, -dh / 2, dw, dh);
            context.restore();
        }

        function paint(nextFrame) {
            if (disposed) return;
            const project = getProject();
            if (!project) return;
            frame = Math.max(0, Number(nextFrame) || 0);
            if (canvas.width !== Number(project.width || 1280)) canvas.width = Number(project.width || 1280);
            if (canvas.height !== Number(project.height || 720)) canvas.height = Number(project.height || 720);
            context.fillStyle = '#101319'; context.fillRect(0, 0, canvas.width, canvas.height);
            const at = frame, tracks = project.tracks || [];
            tracks.forEach(track => {
                if (track.hidden || track.kind === 'audio') return;
                const clips = (track.clips || []).slice().sort((a, b) => a.start - b.start);
                clips.forEach(clip => {
                    if (!activeAt(clip, at)) return;
                    const entering = transitionIn(track, clip, at);
                    drawAsset(project, track, clip, at, entering);
                    const outgoing = transitionOut(track, clip, at);
                    if (outgoing && outgoing.type === 'black') {
                        const fade = outgoing.progress < 0.5 ? outgoing.progress * 2 : (1 - outgoing.progress) * 2;
                        context.save(); context.globalAlpha = clamp(fade, 0, 1); context.fillStyle = '#000'; context.fillRect(0, 0, canvas.width, canvas.height); context.restore();
                    }
                });
            });
            syncMedia(project, at, playing);
            if (typeof setFrame === 'function') setFrame(Math.round(frame));
        }

        function tick(now) {
            if (!playing || disposed) return;
            const project = getProject();
            const end = Math.max(1, ...(project && project.tracks || []).flatMap(track => track.clips.map(clip => clip.start + clip.duration)));
            frame = playStartFrame + Math.max(0, now - playStartAt) * FPS / 1000;
            if (frame >= end) { playStartFrame = 0; playStartAt = now; frame = 0; }
            paint(frame);
            raf = requestAnimationFrame(tick);
        }

        function play() {
            if (disposed || playing) return;
            playing = true; playStartFrame = frame; playStartAt = performance.now();
            const AudioContextClass = window.AudioContext || window.webkitAudioContext;
            if (!audioContext && AudioContextClass) {
                try { audioContext = new AudioContextClass(); media.forEach(el => ensureAudioNode(el)); } catch (_) { audioContext = null; }
            }
            if (audioContext && audioContext.state === 'suspended') audioContext.resume().catch(() => {});
            syncMedia(getProject() || { tracks: [] }, frame, true);
            if (onPlayState) onPlayState(true);
            raf = requestAnimationFrame(tick);
        }
        function pause() {
            if (!playing) { syncMedia(getProject() || { tracks: [] }, frame, false); paint(frame); return; }
            frame = playStartFrame + Math.max(0, performance.now() - playStartAt) * FPS / 1000;
            playing = false; cancelAnimationFrame(raf); syncMedia(getProject() || { tracks: [] }, frame, false);
            if (onPlayState) onPlayState(false);
            paint(frame);
        }
        function seek(value) {
            frame = Math.max(0, Math.round(value));
            if (playing) { playStartFrame = frame; playStartAt = performance.now(); }
            paint(frame);
        }
        function dispose() {
            if (disposed) return;
            disposed = true; playing = false; cancelAnimationFrame(raf);
            Array.from(media.entries()).forEach(([clipId, el]) => release(clipId, el));
            sourceNodes.clear(); gainNodes.clear();
            if (audioContext) audioContext.close().catch(() => {});
            context.clearRect(0, 0, canvas.width, canvas.height);
        }
        paint(Number(getFrame() || 0));
        return { play, pause, seek, getFrame: () => Math.round(frame), isPlaying: () => playing, dispose };
    }

    window.VideoStudioPreview = { mount };
})();
