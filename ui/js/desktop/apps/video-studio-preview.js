(function () {
    'use strict';

    const clamp = (value, min, max) => Math.max(min, Math.min(max, value));
    // Shuttle rates stop at 8x in both directions. Backward playback repaints at most every 100 ms (10 frames/s).
    const MAX_RATE = 8, REVERSE_STEP_MS = 100;

    function mediaURL(asset) {
        const raw = String(asset && asset.media_url || '');
        if (!raw || raw.startsWith('//')) return '';
        try { const url = new URL(raw, location.origin); return url.origin === location.origin ? url.href : ''; }
        catch (_) { return ''; }
    }

    function mount(canvas, getProject, getFrame, setFrame, onPlayState, getScale) {
        const fps = () => window.VideoStudioTime.rate(getProject());
        const scale = () => { const value = typeof getScale === 'function' ? Number(getScale()) : 1; return Math.max(0.1, Math.min(1, value || 1)); };
        const context = canvas.getContext('2d', { alpha: false });
        const media = new Map();
        const sourceNodes = new Map(), gainNodes = new Map();
        let audioContext = null;
        let frame = 0, playing = false, raf = 0, playStartAt = 0, playStartFrame = 0, disposed = false;
        // Shuttle and view state (never saved). rate is signed: 1 forward, negative backward; the JKL shuttle sets it.
        // loopRange is {start, end} or null. soloIds are the audio tracks that alone sound in the preview.
        let rate = 1, loopRange = null, soloIds = new Set(), lastPaint = 0;
        let layer = null, layerContext = null;
        // Forward playback runs the media clocks; backward and paused playback only seek the media.
        const forward = () => playing && rate > 0;
        // Fast forward above 2x and every backward rate are silent.
        const shuttleMuted = () => playing && (rate < 0 || rate > 2);
        const nowFrame = now => playStartFrame + Math.max(0, now - playStartAt) * fps() * rate / 1000;

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
                el.addEventListener('seeked', () => { if (media.get(key) === el && !forward()) paint(frame); });
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
        function speedOf(clip) { const speed = Number(clip.speed); return speed > 0 ? speed : 1; }
        // Disabled clips are neither drawn nor played.
        function activeAt(clip, at) { return !clip.disabled && at >= Number(clip.start || 0) && at < Number(clip.start || 0) + Number(clip.duration || 0); }
        function sourcePosition(clip, at) { return Math.max(0, (Number(clip.offset || 0) + (at - Number(clip.start || 0)) * speedOf(clip)) / fps()); }

        function transitionIn(track, clip, at) {
            const sorted = (track.clips || []).slice().sort((a, b) => a.start - b.start);
            const index = sorted.findIndex(item => item.id === clip.id);
            if (index <= 0 || sorted[index - 1].disabled) return null;
            const prior = sorted[index - 1], duration = Number(prior.transition && prior.transition.duration || 0);
            if (!duration || !prior.transition || prior.start + prior.duration - clip.start !== duration) return null;
            return { type: prior.transition.type, progress: clamp((at - clip.start) / duration, 0, 1) };
        }

        function transitionOut(track, clip, at) {
            const transition = clip.transition, duration = Number(transition && transition.duration || 0);
            if (!duration) return null;
            // The transition is honoured only when the clip after it on the track is enabled (export agrees).
            const after = (track.clips || []).slice().sort((a, b) => a.start - b.start).find(item => item.start > clip.start);
            if (!after || after.disabled) return null;
            const start = clip.start + clip.duration - duration;
            return at >= start ? { type: transition.type, progress: clamp((at - start) / duration, 0, 1) } : null;
        }

        // Keeps the media in step with the frame. forwardPlayback is true only while playing forward: then the media
        // clocks run (at the shuttle rate); otherwise they stay paused and are only seeked.
        function syncMedia(project, at, forwardPlayback) {
            const active = new Set(), keep = new Set(), preloadUntil = at + fps() * 2;
            (project.tracks || []).forEach(track => {
                if (track.hidden) return;
                (track.clips || []).forEach(clip => {
                    const current = activeAt(clip, at), near = !current && !clip.disabled && clip.start > at && clip.start <= preloadUntil;
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
                    // With a solo active only the soloed audio tracks sound; embedded video audio is silent too.
                    const soloed = soloIds.size === 0 || (track.kind === 'audio' && soloIds.has(track.id));
                    // A detached video clip is silent here (its audio clip plays), and so is any clip above 4x.
                    const audioAllowed = !track.muted && !track.hidden && !clip.audio_detached && speedOf(clip) <= 4 && soloed && !shuttleMuted() && (track.kind === 'audio' || asset.kind === 'video' && asset.has_audio);
                    const targetGain = audioAllowed ? clamp(Number(clip.volume == null ? 1 : clip.volume) * fadeIn * fadeOut, 0, 4) : 0;
                    const gain = ensureAudioNode(el);
                    if (gain) { gain.gain.value = targetGain; el.volume = 1; el.muted = false; }
                    else { el.volume = clamp(targetGain, 0, 1); if (asset.kind === 'video') el.muted = !audioAllowed; }
                    // Browsers accept 1/16x to 16x. The clip speed scales the shuttle rate while the media clocks run.
                    const playRate = clamp(speedOf(clip) * (forwardPlayback ? rate : 1), 0.0625, 16);
                    if (el instanceof HTMLMediaElement && Math.abs(el.playbackRate - playRate) > 1e-6) el.playbackRate = playRate;
                    const desired = sourcePosition(clip, at);
                    if (Math.abs((el.currentTime || 0) - desired) > (forwardPlayback ? 0.38 : 0.025)) {
                        try { el.currentTime = desired; } catch (_) { /* source metadata is loading */ }
                    }
                    if (forwardPlayback && el.paused) el.play().catch(() => {});
                    else if (!forwardPlayback && !el.paused) el.pause();
                });
            });
            Array.from(media.entries()).forEach(([clipId, el]) => {
                if (!keep.has(clipId)) release(clipId, el);
                else if (!active.has(clipId) && el instanceof HTMLMediaElement && !el.paused) el.pause();
            });
        }

        // Draws one clip into ctx. alphaScale and reveal carry the parts of a transition the clip owns itself.
        function drawAsset(ctx, project, track, clip, at, alphaScale) {
            const asset = assetFor(project, clip), el = entry(asset, clip);
            if (!asset || !el) return;
            if (asset.kind !== 'image') {
                const desired = sourcePosition(clip, at);
                if (Math.abs((el.currentTime || 0) - desired) > (forward() ? 0.38 : 0.025)) {
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
            const outgoing = transitionOut(track, clip, at), entering = transitionIn(track, clip, at);
            let alpha = clamp(Number(clip.opacity == null ? 1 : clip.opacity), 0, 1) * (alphaScale == null ? 1 : alphaScale);
            if (outgoing && outgoing.type === 'black') alpha *= outgoing.progress < 0.5 ? 1 - outgoing.progress * 2 : 0;
            if (entering && entering.type === 'black') alpha *= entering.progress < 0.5 ? 0 : (entering.progress - 0.5) * 2;
            ctx.save();
            ctx.globalAlpha = alpha;
            ctx.translate(x + w / 2, y + h / 2);
            ctx.rotate(radians);
            if (clip.fit === 'cover') {
                ctx.beginPath(); ctx.rect(-w / 2, -h / 2, w, h); ctx.clip();
            }
            ctx.drawImage(el, -dw / 2, -dh / 2, dw, dh);
            ctx.restore();
        }

        // The export blends whole track layers (xfade): a dissolve fades the outgoing layer out while the
        // incoming one fades in, and a wipe reveals the incoming layer across the full frame. The preview
        // composes the pair in a track layer the same way. Fade through black already matches.
        function transitionPair(track, at) {
            const clips = (track.clips || []).slice().sort((a, b) => a.start - b.start);
            for (let i = 0; i < clips.length - 1; i++) {
                const clip = clips[i], next = clips[i + 1], transition = clip.transition;
                if (!transition || transition.type === 'black' || !activeAt(clip, at) || !activeAt(next, at)) continue;
                const duration = Number(transition.duration || 0);
                if (duration > 0 && clip.start + clip.duration - next.start === duration) return { out: clip, inc: next, type: transition.type, progress: clamp((at - next.start) / duration, 0, 1) };
            }
            return null;
        }
        function trackLayer() {
            if (!layer) {
                layer = typeof OffscreenCanvas === 'function' ? new OffscreenCanvas(canvas.width, canvas.height) : document.createElement('canvas');
                layerContext = layer.getContext('2d');
            }
            if (layer.width !== canvas.width) layer.width = canvas.width;
            if (layer.height !== canvas.height) layer.height = canvas.height;
            layerContext.clearRect(0, 0, layer.width, layer.height);
            return layerContext;
        }
        function drawTransition(project, track, pair, at) {
            const ctx = trackLayer(), p = pair.progress, W = canvas.width, H = canvas.height;
            if (pair.type === 'dissolve') {
                drawAsset(ctx, project, track, pair.out, at, 1 - p);
                ctx.globalCompositeOperation = 'lighter';
                drawAsset(ctx, project, track, pair.inc, at, p);
                ctx.globalCompositeOperation = 'source-over';
            } else {
                // User "wipeleft" reveals from the left edge (xfade wiperight), "wiperight" from the right.
                const reveal = W * p, left = pair.type === 'wipeleft' ? 0 : W - reveal;
                drawAsset(ctx, project, track, pair.out, at, 1);
                ctx.clearRect(left, 0, reveal, H);
                ctx.save(); ctx.beginPath(); ctx.rect(left, 0, reveal, H); ctx.clip();
                drawAsset(ctx, project, track, pair.inc, at, 1);
                ctx.restore();
            }
            context.drawImage(layer, 0, 0);
        }

        function paint(nextFrame) {
            if (disposed) return;
            const project = getProject();
            if (!project) return;
            frame = Math.max(0, Number(nextFrame) || 0);
            // The backing store follows the preview quality; drawing uses canvas.width/height, so geometry scales with it.
            const width = Math.max(2, Math.round(Number(project.width || 1280) * scale())), height = Math.max(2, Math.round(Number(project.height || 720) * scale()));
            if (canvas.width !== width) canvas.width = width;
            if (canvas.height !== height) canvas.height = height;
            // The export composes onto black, so empty and letterboxed areas are black here too.
            context.fillStyle = '#000'; context.fillRect(0, 0, canvas.width, canvas.height);
            const at = frame, tracks = project.tracks || [];
            tracks.forEach(track => {
                if (track.hidden || track.kind === 'audio') return;
                const pair = transitionPair(track, at);
                const clips = (track.clips || []).slice().sort((a, b) => a.start - b.start);
                clips.forEach(clip => {
                    if (!activeAt(clip, at)) return;
                    if (pair && (clip === pair.out || clip === pair.inc)) { if (clip === pair.out) drawTransition(project, track, pair, at); return; }
                    drawAsset(context, project, track, clip, at);
                    const outgoing = transitionOut(track, clip, at);
                    if (outgoing && outgoing.type === 'black') {
                        const fade = outgoing.progress < 0.5 ? outgoing.progress * 2 : (1 - outgoing.progress) * 2;
                        context.save(); context.globalAlpha = clamp(fade, 0, 1); context.fillStyle = '#000'; context.fillRect(0, 0, canvas.width, canvas.height); context.restore();
                    }
                });
            });
            syncMedia(project, at, forward());
            if (typeof setFrame === 'function') setFrame(Math.round(frame));
        }

        // The range playback runs in: the loop range when looping, else the whole timeline (enabled clips only).
        function playBounds(project) {
            if (loopRange) return loopRange;
            const end = Math.max(1, ...(project && project.tracks || []).flatMap(track => track.clips.filter(clip => !clip.disabled).map(clip => clip.start + clip.duration)));
            return { start: 0, end };
        }

        function tick(now) {
            if (!playing || disposed) return;
            const bounds = playBounds(getProject());
            let next = nowFrame(now);
            if (rate > 0 && next >= bounds.end) {
                // Looping wraps to the range start; normal speed wraps to the start as before; a shuttle stops at the end.
                if (!loopRange && rate !== 1) { halt(bounds.end - 1); return; }
                next = loopRange ? loopRange.start : 0; playStartFrame = next; playStartAt = now;
            } else if (rate < 0 && next <= bounds.start) {
                // Backward playback stops at the start, or wraps to the range end when looping.
                if (!loopRange) { halt(0); return; }
                next = loopRange.end - 1; playStartFrame = next; playStartAt = now; lastPaint = 0;
            }
            // Forward playback paints every frame and never shows the end (it is exclusive); backward playback repaints
            // at most every 100 ms. The frame moves only on paint, so a seek that completes in between repaints the
            // frame on screen instead of a newer one.
            if (rate > 0 || now - lastPaint >= REVERSE_STEP_MS) { lastPaint = now; paint(rate > 0 ? Math.min(next, bounds.end - 1) : next); }
            raf = requestAnimationFrame(tick);
        }

        function play() {
            if (disposed || playing) return;
            playing = true; playStartFrame = frame; playStartAt = performance.now(); lastPaint = 0;
            const AudioContextClass = window.AudioContext || window.webkitAudioContext;
            if (!audioContext && AudioContextClass) {
                try { audioContext = new AudioContextClass(); media.forEach(el => ensureAudioNode(el)); } catch (_) { audioContext = null; }
            }
            if (audioContext && audioContext.state === 'suspended') audioContext.resume().catch(() => {});
            syncMedia(getProject() || { tracks: [] }, frame, forward());
            if (onPlayState) onPlayState(true);
            raf = requestAnimationFrame(tick);
        }
        // Stops playback on frame at (pause, or the end of a shuttle run) and returns the rate to 1x.
        function halt(at) {
            playing = false; rate = 1; cancelAnimationFrame(raf);
            frame = Math.max(0, at);
            syncMedia(getProject() || { tracks: [] }, frame, false);
            if (onPlayState) onPlayState(false);
            paint(frame);
        }
        // Pausing always returns the rate to 1x, so Space and the play button play forward again.
        function pause() {
            if (!playing) { rate = 1; syncMedia(getProject() || { tracks: [] }, frame, false); paint(frame); return; }
            halt(nowFrame(performance.now()));
        }
        function seek(value) {
            frame = Math.max(0, Math.round(value));
            if (playing) { playStartFrame = frame; playStartAt = performance.now(); }
            paint(frame);
        }
        // Signed playback rate, clamped to ±8x (0 or invalid means 1x). While playing, the playhead continues from where
        // it is at the new rate.
        function setRate(value) {
            const next = clamp(Number(value) || 1, -MAX_RATE, MAX_RATE);
            if (next === rate) return;
            if (playing) { const now = performance.now(); frame = Math.max(0, nowFrame(now)); playStartFrame = frame; playStartAt = now; lastPaint = 0; }
            rate = next;
            if (playing) syncMedia(getProject() || { tracks: [] }, frame, forward());
        }
        // Loop range {start, end} in frames, or null to play normally. An empty range means no loop.
        function setLoop(range) {
            const start = range ? Math.max(0, Number(range.start) || 0) : 0, end = range ? Number(range.end) || 0 : 0;
            loopRange = end > start ? { start, end } : null;
        }
        // Audio tracks that alone sound (preview only, never saved). Empty means every track sounds as usual.
        function setSolo(ids) {
            const next = new Set((ids || []).filter(Boolean));
            if (next.size === soloIds.size && Array.from(next).every(id => soloIds.has(id))) return;
            soloIds = next;
            const project = getProject();
            if (!disposed && project) syncMedia(project, frame, forward());
        }
        function dispose() {
            if (disposed) return;
            disposed = true; playing = false; cancelAnimationFrame(raf);
            Array.from(media.entries()).forEach(([clipId, el]) => release(clipId, el));
            sourceNodes.clear(); gainNodes.clear();
            layer = null; layerContext = null;
            if (audioContext) audioContext.close().catch(() => {});
            context.clearRect(0, 0, canvas.width, canvas.height);
        }
        paint(Number(getFrame() || 0));
        return { play, pause, seek, setRate, setLoop, setSolo, rate: () => rate, getFrame: () => Math.round(frame), isPlaying: () => playing, dispose };
    }

    window.VideoStudioPreview = { mount };
})();
