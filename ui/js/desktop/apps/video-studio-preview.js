(function () {
    'use strict';

    const clamp = (value, min, max) => Math.max(min, Math.min(max, value));
    // Shuttle rates stop at 8x in both directions. Backward playback repaints at most every 100 ms (10 frames/s).
    const MAX_RATE = 8, REVERSE_STEP_MS = 100;
    // settle() polls every 20 ms; a seek whose frame callback never arrives counts as shown 250 ms after 'seeked'.
    const SETTLE_POLL_MS = 20, SETTLE_GRACE_MS = 250;
    // Built-in looks are served as .cube by the look endpoint (index §4.20).
    const BUILTIN_LUT_URL = '/api/desktop/video-studio/luts/builtin/';

    // Only same-origin media plays. options.mediaURL may pick another URL (a proxy); it passes the same check.
    function sameOriginURL(value) {
        const raw = String(value || '');
        if (!raw || raw.startsWith('//')) return '';
        try { const url = new URL(raw, location.origin); return url.origin === location.origin ? url.href : ''; }
        catch (_) { return ''; }
    }

    // Pure rules, exported as VideoStudioPreview.rules for Node checks.
    // Disabled clips are neither drawn nor played.
    function activeAt(clip, at) { return !clip.disabled && at >= Number(clip.start || 0) && at < Number(clip.start || 0) + Number(clip.duration || 0); }

    // The export blends whole track layers (xfade): a dissolve fades the outgoing layer out while the incoming one fades
    // in, a wipe reveals the incoming layer across the full frame, and a fade through black mixes both layers with
    // opaque black (fadeblack). The painters compose the pair in one track layer the same way. A transition applies only
    // while both clips are enabled and active (the export agrees).
    function transitionPair(track, at) {
        const clips = (track.clips || []).slice().sort((a, b) => a.start - b.start);
        for (let i = 0; i < clips.length - 1; i++) {
            const clip = clips[i], next = clips[i + 1], transition = clip.transition;
            if (!transition || !activeAt(clip, at) || !activeAt(next, at)) continue;
            const duration = Number(transition.duration || 0);
            // index is the overlap frame i = at − inc.start (whole frames, 0..D−1) and duration is D. The painters derive every
            // transition parameter from them (plan 2e D1); progress = i/D stays for the hard-cut fallback.
            const index = clamp(Math.floor(at - next.start), 0, duration - 1);
            if (duration > 0 && clip.start + clip.duration - next.start === duration) return { out: clip, inc: next, type: transition.type, progress: clamp((at - next.start) / duration, 0, 1), index, duration };
        }
        return null;
    }

    // The range playback runs in: the loop range when looping, else the whole timeline (enabled clips only).
    function playBounds(project, loopRange) {
        if (loopRange) return loopRange;
        const end = Math.max(1, ...(project && project.tracks || []).flatMap(track => track.clips.filter(clip => !clip.disabled).map(clip => clip.start + clip.duration)));
        return { start: 0, end };
    }

    // Ramp-aware media timing (plan 2b, index §4.18). Positions use the content anchor that the keyframes and the export
    // share, VideoStudioEffects.sourceAt: offset + S(t) on a ramp, offset + t x speed otherwise.
    // The source time that a clip shows at timeline frame `at`, in seconds and never negative. fps is the project rate.
    function sourceSeconds(clip, at, fps) { return Math.max(0, window.VideoStudioEffects.sourceAt(clip, at) / fps); }
    // The media element's playbackRate at frame `at`: the clip's instantaneous speed (VideoStudioEffects.speedAt, clip
    // time) times the shuttle rate (1 unless playing forward), inside the 1/16x..16x that browsers accept. Forward
    // playback sets it on every paint, so a ramp's media clock follows the curve; the 0.38 s drift seek corrects the rest.
    function mediaRate(clip, at, rate) { return clamp(window.VideoStudioEffects.speedAt(clip, at - Number(clip.start || 0)) * rate, 0.0625, 16); }
    // Whether a clip may sound at all; syncMedia applies the solo and shuttle state on top. Muted or hidden tracks are
    // silent. A detached video clip is silent because its audio clip plays, and so is any clip above 4x. The linked
    // sound of a ramped video-track clip is silent too, because the export mutes it (Go audibleClip). Audio tracks
    // sound, and so does video with a sound stream.
    function audioAllowed(track, clip, asset) {
        const speed = Number(clip.speed) > 0 ? Number(clip.speed) : 1;
        if (!asset || track.muted || track.hidden || clip.audio_detached || speed > 4) return false;
        if (track.kind === 'video' && window.VideoStudioEffects.hasRamp(clip)) return false;
        return track.kind === 'audio' || (asset.kind === 'video' && !!asset.has_audio);
    }

    function mount(canvas, getProject, getFrame, setFrame, onPlayState, getScale, options) {
        const fps = () => window.VideoStudioTime.rate(getProject());
        const scale = () => { const value = typeof getScale === 'function' ? Number(getScale()) : 1; return Math.max(0.1, Math.min(1, value || 1)); };
        const media = new Map();
        // options.painter: 'auto' | 'gl' | '2d' (default '2d'). options.mediaURL(asset): the URL to play (a proxy), by
        // default asset.media_url; it still passes the same-origin check. The painter owns its canvas element, because a
        // canvas holds one context type only.
        const pickURL = options && typeof options.mediaURL === 'function' ? options.mediaURL : null;
        // Painter selection (index §4.5): GL only when asked for ('auto' or 'gl'), WebGL2 exists and the compositor
        // starts. A canvas that ever returned a WebGL context cannot give a 2D one, so after a failed GL attempt, and on
        // every fallback, the 2D painter draws on a fresh shallow clone of the element (same attributes, including
        // data-preview), which replaces it in the page. Nothing may cache the old element: callers re-query
        // [data-preview]. options.onPainterChange(kind, reason, canvas) reports each switch to 2D, and
        // options.onProgramFailed(kind, id) each program the GL painter could not compile.
        const requestedPainter = options && (options.painter === 'auto' || options.painter === 'gl') ? options.painter : '2d';
        let programFailures = [], initialFallback = '';
        function freshCanvas() {
            const fresh = canvas.cloneNode(false);
            if (canvas.parentNode) canvas.replaceWith(fresh);
            canvas = fresh;
        }
        function notifyPainter(reason) {
            if (options && typeof options.onPainterChange === 'function') options.onPainterChange(painter.kind, reason, canvas);
        }
        function glHooks() {
            return {
                onLost() {},
                onRestored() { if (!disposed) paint(frame); },
                onFallback(reason) { fallback(reason); },
                onProgramFailed(kind, id) {
                    if (programFailures.some(item => item.kind === kind && item.id === id)) return;
                    programFailures.push({ kind, id });
                    if (options && typeof options.onProgramFailed === 'function') options.onProgramFailed(kind, id);
                },
                // LUT tables for the GL painter's lut passes (index §4.5); a finished load repaints the frame.
                loadLUT(ref) { return fetchLUT(ref); },
                onResource() { if (!disposed) paint(frame); }
            };
        }
        function choosePainter() {
            if (requestedPainter === '2d') return window.VideoStudioPainter2D.create(canvas);
            const compositor = window.VideoStudioCompositor;
            initialFallback = 'unsupported';
            if (compositor && compositor.supported()) {
                const gl = compositor.create(canvas, glHooks());
                if (gl) { initialFallback = ''; return gl; }
                initialFallback = compositor.lastError || 'init_failed';
                freshCanvas();
            }
            return window.VideoStudioPainter2D.create(canvas);
        }
        // Switches to the 2D painter on a fresh element (a fallback trigger, or 'forced'); false when it already draws in 2D.
        function fallback(reason) {
            if (disposed || !painter || painter.kind === '2d') return false;
            const previous = painter;
            try { previous.dispose(); } catch (_) { /* a lost context may refuse cleanup */ }
            freshCanvas();
            painter = window.VideoStudioPainter2D.create(canvas);
            programFailures = [];
            notifyPainter(reason || 'forced');
            paint(frame);
            return true;
        }
        function failedPrograms() { return programFailures.map(item => ({ kind: item.kind, id: item.id })); }
        // The size a clip's media element reports (autorotated), or null before it knows one (index §4.5). The stage's crop
        // overlay passes it to VideoStudioPicture.decodedSize, like both painters (plan 2c R7).
        function mediaSize(clipId) {
            const el = media.get(clipId);
            const width = el ? Number(el.videoWidth || el.naturalWidth || 0) : 0, height = el ? Number(el.videoHeight || el.naturalHeight || 0) : 0;
            return width && height ? { width, height } : null;
        }
        let painter = choosePainter();
        if (initialFallback) notifyPainter(initialFallback);
        const sourceNodes = new Map(), gainNodes = new Map();
        let audioContext = null;
        let frame = 0, playing = false, raf = 0, playStartAt = 0, playStartFrame = 0, disposed = false;
        // Shuttle and view state (never saved). rate is signed: 1 forward, negative backward; the JKL shuttle sets it.
        // loopRange is {start, end} or null. soloIds are the audio tracks that alone sound in the preview.
        let rate = 1, loopRange = null, soloIds = new Set(), lastPaint = 0;
        // options.stopAtEnd (source monitor): playback stops at the end at every rate instead of wrapping at 1x.
        const stopAtEnd = !!(options && options.stopAtEnd);
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

        function sourceURL(asset) {
            let raw = asset.media_url;
            if (pickURL) { try { raw = pickURL(asset); } catch (_) { raw = asset.media_url; } }
            return sameOriginURL(raw);
        }

        // The URL of a LUT ref (index §4.5): builtin:<id> comes from the look endpoint. Any other ref must name a
        // project asset of kind lut, fetched through the same media URL as other media (options.mediaURL, else
        // media_url). Same origin only; '' when there is none.
        function lutURL(ref) {
            const text = String(ref || '');
            if (text.startsWith('builtin:')) return text.length > 8 ? sameOriginURL(BUILTIN_LUT_URL + encodeURIComponent(text.slice(8)) + '.cube') : '';
            const project = getProject();
            const asset = text && project ? (project.assets || []).find(item => item.id === text && item.kind === 'lut') : null;
            return asset ? sourceURL(asset) : '';
        }
        // Fetches and parses a LUT with VideoStudioPicture.parseCube (Go ParseCube's rules) into
        // {size, domainMin, domainMax, data}. It rejects for an unknown ref, an HTTP error or a bad table.
        function fetchLUT(ref) {
            const url = lutURL(ref);
            if (!url) return Promise.reject(new Error('lut_not_found'));
            return fetch(url, { credentials: 'same-origin' }).then(response => {
                if (!response.ok) throw new Error('lut_http_' + response.status);
                return response.text();
            }).then(text => window.VideoStudioPicture.parseCube(text));
        }

        function entry(asset, clip) {
            if (!asset || !clip || !asset.id || !clip.id) return null;
            const key = clip.id, src = sourceURL(asset);
            if (!src) return null;
            // The cache keys on the asset and the URL, so a switch between original and proxy reloads the element.
            if (media.has(key) && media.get(key).__vsAssetId === asset.id && media.get(key).__vsSrc === src) return media.get(key);
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
                el.addEventListener('seeked', () => { el.__vsSeekedAt = performance.now(); if (media.get(key) === el && !forward()) paint(frame); });
                el.load();
            }
            el.__vsAssetId = asset.id; el.__vsSrc = src;
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
        // The source time of a clip at a frame: the shared content anchor (rules.sourceSeconds), ramp-aware.
        function sourcePosition(clip, at) { return sourceSeconds(clip, at, fps()); }

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
                    // The clip rules (rules.audioAllowed: detached, above 4x, ramped video-track sound), then solo and shuttle.
                    const audible = audioAllowed(track, clip, asset) && soloed && !shuttleMuted();
                    const targetGain = audible ? clamp(Number(clip.volume == null ? 1 : clip.volume) * fadeIn * fadeOut, 0, 4) : 0;
                    const gain = ensureAudioNode(el);
                    if (gain) { gain.gain.value = targetGain; el.volume = 1; el.muted = false; }
                    else { el.volume = clamp(targetGain, 0, 1); if (asset.kind === 'video') el.muted = !audible; }
                    // The clip's speed at this frame scales the shuttle rate (rules.mediaRate). A ramp changes it on every paint.
                    const playRate = mediaRate(clip, at, forwardPlayback ? rate : 1);
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
                if (!keep.has(clipId)) { release(clipId, el); painter.release(clipId); }
                else if (!active.has(clipId) && el instanceof HTMLMediaElement && !el.paused) el.pause();
            });
        }

        // Remembers a seek so settle() can wait for its frame: a frame callback where the browser offers one.
        function markSeek(el) {
            const token = (el.__vsSeekToken || 0) + 1;
            el.__vsSeekToken = token; el.__vsSeekedAt = 0; el.__vsFrameSeen = typeof el.requestVideoFrameCallback !== 'function';
            if (el.__vsFrameSeen) return;
            try { el.requestVideoFrameCallback(() => { if (el.__vsSeekToken === token) el.__vsFrameSeen = true; }); }
            catch (_) { el.__vsFrameSeen = true; }
        }

        // An image counts only once it finished loading: naturalWidth is known while the file is still arriving, and a
        // WebGL texture taken from such an image stays blank (the upload fails with INVALID_VALUE) and is never refreshed.
        function imageLoaded(el) { return !!(el.naturalWidth && el.naturalHeight) && el.complete !== false; }

        // A decoded frame is available: the 2D painter's draw condition (the compositor keeps its own upload gating).
        function frameReady(asset, el) {
            if (!asset || !el) return false;
            if (asset.kind === 'image') return imageLoaded(el);
            const width = Number(el.videoWidth || el.naturalWidth || asset.width || 0), height = Number(el.videoHeight || el.naturalHeight || asset.height || 0);
            return !!(width && height) && !(el instanceof HTMLVideoElement && el.readyState < 2);
        }

        // One clip of the frame description. Video seeks here when it drifted: more than 0.38 s while playing forward
        // (the media clock runs), else more than 0.025 s.
        function describeItem(project, clip, at) {
            const asset = assetFor(project, clip) || null, el = asset ? entry(asset, clip) : null;
            if (el && asset.kind !== 'image') {
                const desired = sourcePosition(clip, at);
                if (Math.abs((el.currentTime || 0) - desired) > (forward() ? 0.38 : 0.025)) {
                    try { el.currentTime = desired; markSeek(el); } catch (_) { /* source metadata is loading */ }
                }
            }
            return { clip, asset, el: el || null, ready: frameReady(asset, el), state: window.VideoStudioEffects.clipState(clip, at, project) };
        }

        // The frame description handed to the painter (phase 2 index §4.5): visual tracks in paint order, hidden tracks
        // skipped; per track the active clips and the transition pair, which the painter composes as one track layer.
        function describeFrame(project, at, opts) {
            const scene = { project, frame: at, width: opts.width, height: opts.height, layers: [] };
            (project.tracks || []).forEach(track => {
                if (track.hidden || track.kind === 'audio') return;
                const pair = transitionPair(track, at), items = [];
                (track.clips || []).slice().sort((a, b) => a.start - b.start).forEach(clip => {
                    if (activeAt(clip, at) && !(pair && (clip === pair.out || clip === pair.inc))) items.push(describeItem(project, clip, at));
                });
                const transition = pair ? { type: pair.type, progress: pair.progress, index: pair.index, duration: pair.duration, out: describeItem(project, pair.out, at), inc: describeItem(project, pair.inc, at) } : null;
                scene.layers.push({ trackId: track.id, items, transition });
            });
            return scene;
        }

        function paint(nextFrame) {
            if (disposed) return;
            const project = getProject();
            if (!project) return;
            frame = Math.max(0, Number(nextFrame) || 0);
            // The backing store follows the preview quality; the painter sizes its canvas to it, so geometry scales with it.
            const width = Math.max(2, Math.round(Number(project.width || 1280) * scale())), height = Math.max(2, Math.round(Number(project.height || 720) * scale()));
            painter.frame(describeFrame(project, frame, { width, height }));
            syncMedia(project, frame, forward());
            if (typeof setFrame === 'function') setFrame(Math.round(frame));
        }

        function tick(now) {
            if (!playing || disposed) return;
            const bounds = playBounds(getProject(), loopRange);
            let next = nowFrame(now);
            if (rate > 0 && next >= bounds.end) {
                // Looping wraps to the range start; normal speed wraps to the start as before unless stopAtEnd is set;
                // a shuttle stops at the end.
                if (!loopRange && (stopAtEnd || rate !== 1)) { halt(bounds.end - 1); return; }
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
            if (audioContext) audioContext.close().catch(() => {});
            painter.dispose();
        }
        // Every active visual clip holds its frame for the current position: a video has decoded data, is not seeking,
        // sits within 0.025 s of its position and, where frame callbacks exist, showed a frame after the last seek (or
        // the seek completed SETTLE_GRACE_MS ago, for elements that never present frames); an image is decoded.
        function mediaSettled(project) {
            return (project.tracks || []).every(track => track.hidden || track.kind === 'audio' || (track.clips || []).every(clip => {
                if (!activeAt(clip, frame)) return true;
                const asset = assetFor(project, clip);
                if (!asset || !sourceURL(asset)) return true;
                const el = media.get(clip.id);
                if (!el) return false;
                if (asset.kind === 'image') return imageLoaded(el);
                if (el.readyState < 2 || el.seeking || Math.abs((el.currentTime || 0) - sourcePosition(clip, frame)) > 0.025) return false;
                return el.__vsFrameSeen !== false || (el.__vsSeekedAt > 0 && performance.now() - el.__vsSeekedAt >= SETTLE_GRACE_MS);
            }));
        }
        // The painter holds every resource the frame needs: the GL painter has no LUT load in flight (pending()), and the
        // 2D painter never loads anything.
        function painterIdle() { return !(painter && typeof painter.pending === 'function' && painter.pending() > 0); }
        // Resolves true once the media of the current frame settled and the painter is idle, after one more paint; false
        // after timeoutMs.
        function settle(timeoutMs) {
            const deadline = Date.now() + Math.max(0, Number(timeoutMs) || 0);
            paint(frame);
            return new Promise(resolve => {
                const check = () => {
                    const project = getProject();
                    if (disposed || !project) { resolve(false); return; }
                    if (mediaSettled(project) && painterIdle()) { paint(frame); resolve(true); return; }
                    if (Date.now() >= deadline) { resolve(false); return; }
                    paint(frame);
                    setTimeout(check, SETTLE_POLL_MS);
                };
                check();
            });
        }
        // Paints the current frame synchronously and reads it back through the painter (rect in backing pixels).
        function capture(rect) {
            if (disposed) return null;
            paint(frame);
            return painter.read(rect);
        }
        paint(Number(getFrame() || 0));
        return { play, pause, seek, setRate, setLoop, setSolo, rate: () => rate, getFrame: () => Math.round(frame), isPlaying: () => playing, fallback, failedPrograms, mediaSize, dispose,
            painterKind: () => painter.kind, capture, settle };
    }

    window.VideoStudioPreview = { mount, rules: { transitionPair, activeAt, playBounds, sourceSeconds, mediaRate, audioAllowed } };
})();
