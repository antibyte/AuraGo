(function () {
    'use strict';

    // NoisemakerPlayer - audio engine, play queue, visualizer, bottom player bar and
    // Now Playing view for the Noisemaker desktop app.
    // Factory: create(deps) -> controller.
    // deps = { esc, t, lang, readonly, formatDuration, formatDate, hasMore(), isVisible(), prefs }.
    // Events: state({ track, playing }), change(prefs), favorite(track, value), delete(track),
    // template(track), download(track), expand(open), needmore(), error(track), visualizer-unavailable().

    const SVG = {
        play: '<svg class="nm-svg-play" viewBox="0 0 16 16" width="18" height="18" aria-hidden="true"><path fill="currentColor" d="M4 2.5v11l9-5.5z"/></svg>',
        pause: '<svg class="nm-svg-pause" viewBox="0 0 16 16" width="18" height="18" aria-hidden="true"><path fill="currentColor" d="M3 2.5h3.4v11H3zM9.6 2.5H13v11H9.6z"/></svg>',
        prev: '<svg viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill="currentColor" d="M3 3h2v10H3zM13 3v10L6 8z"/></svg>',
        next: '<svg viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path fill="currentColor" d="M11 3h2v10h-2zM3 3v10l7-5z"/></svg>',
        shuffle: '<svg viewBox="0 0 16 16" width="15" height="15" aria-hidden="true"><path d="M2 4h2.5l5 8H13M2 12h2.5l1.6-2.6M9.5 4H13M11.5 2l2 2-2 2M11.5 10l2 2-2 2" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
        repeat: '<svg viewBox="0 0 16 16" width="15" height="15" aria-hidden="true"><path d="M3 7V6a2 2 0 0 1 2-2h8m0 0-2-2m2 2-2 2M13 9v1a2 2 0 0 1-2 2H3m0 0 2 2m-2-2 2-2" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
        volume: '<svg class="nm-svg-volume" viewBox="0 0 16 16" width="15" height="15" aria-hidden="true"><path fill="currentColor" d="M2 6h3l4-3v10L5 10H2z"/><path d="M11 5.5a3.5 3.5 0 0 1 0 5M12.8 3.5a6 6 0 0 1 0 9" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>',
        muted: '<svg class="nm-svg-muted" viewBox="0 0 16 16" width="15" height="15" aria-hidden="true"><path fill="currentColor" d="M2 6h3l4-3v10L5 10H2z"/><path d="m10.5 6 4 4m0-4-4 4" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>',
        heart: '<svg viewBox="0 0 16 16" width="15" height="15" aria-hidden="true"><path class="nm-heart-path" d="M8 13.6 2.9 8.7A3.1 3.1 0 0 1 7.3 4.3L8 5l.7-.7a3.1 3.1 0 0 1 4.4 4.4z" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/></svg>',
        expand: '<svg class="nm-svg-expand" viewBox="0 0 16 16" width="15" height="15" aria-hidden="true"><path d="M3 10.5 8 5.5l5 5" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/></svg>',
        collapse: '<svg class="nm-svg-collapse" viewBox="0 0 16 16" width="15" height="15" aria-hidden="true"><path d="M3 5.5 8 10.5l5-5" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/></svg>',
        close: '<svg viewBox="0 0 16 16" width="13" height="13" aria-hidden="true"><path d="M4 4l8 8M12 4l-8 8" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>',
        download: '<svg viewBox="0 0 16 16" width="15" height="15" aria-hidden="true"><path d="M8 2v8m0 0 3-3M8 10 5 7M3 13h10" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>',
        trash: '<svg viewBox="0 0 16 16" width="15" height="15" aria-hidden="true"><path d="M3 4h10M6 4V2.5h4V4M4.5 4l.6 9h5.8l.6-9" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
        template: '<svg viewBox="0 0 16 16" width="15" height="15" aria-hidden="true"><path d="M2.5 13.5 5 13l7.5-7.5-2.5-2.5L2.5 10.5zM9 4l2.5 2.5" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
        note: '<svg viewBox="0 0 24 24" width="40" height="40" aria-hidden="true"><path fill="currentColor" d="M9 3v10.55A4 4 0 1 0 11 17V7h5V3z"/></svg>',
        back: '<svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true"><path d="M10 3 5 8l5 5" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/></svg>'
    };

    function clamp01(value) {
        const n = Number(value);
        if (!Number.isFinite(n)) return 0;
        return Math.min(1, Math.max(0, n));
    }

    function create(deps) {
        const esc = deps.esc || (v => String(v == null ? '' : v));
        const t = deps.t || ((key, params, fallback) => fallback || key);
        const readonly = !!deps.readonly;
        const formatDuration = typeof deps.formatDuration === 'function' ? deps.formatDuration : (ms => Math.round((ms || 0) / 1000) + ' s');
        const formatDate = typeof deps.formatDate === 'function' ? deps.formatDate : (value => String(value || ''));
        const handlers = {};
        const on = (name, cb) => { (handlers[name] = handlers[name] || []).push(cb); };
        const emit = (name, ...args) => {
            (handlers[name] || []).forEach(cb => {
                try { cb(...args); } catch (err) { console.warn('Noisemaker player handler failed', err); }
            });
        };
        const reducedMotion = !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
        const initial = Object.assign({ shuffle: false, repeat: 'off', volume: 0.9, muted: false, visualizer: true }, deps.prefs || {});

        const audio = new Audio();
        audio.preload = 'metadata';
        audio.volume = clamp01(initial.volume);
        audio.muted = !!initial.muted;
        let queue = [];      // track objects
        let order = [];      // play order: indices into queue
        let pos = -1;        // position inside order
        let shuffle = !!initial.shuffle;
        let repeat = ['off', 'all', 'one'].includes(initial.repeat) ? initial.repeat : 'off';
        let visualizerOn = initial.visualizer !== false;
        let visualizerAvailable = !!(window.AudioContext || window.webkitAudioContext);
        let audioCtx = null;
        let analyser = null;
        let freqData = null;
        let raf = 0;
        let visTimer = null;
        let errorStreak = 0;
        let nowPlayingOpen = false;
        let pendingAutoplay = false;
        let seeking = false;
        let disposed = false;

        const barShell = document.createElement('div');
        barShell.innerHTML = '<div class="nm-player" role="region"></div>';
        const bar = barShell.firstElementChild;
        bar.setAttribute('aria-label', t('desktop.noisemaker_now_playing'));
        bar.innerHTML = barMarkup();
        const np = document.createElement('section');
        np.className = 'nm-now-playing';
        np.setAttribute('aria-label', t('desktop.noisemaker_now_playing'));
        np.hidden = true;
        np.innerHTML = nowPlayingMarkup();

        const q = selector => bar.querySelector(selector);
        const nq = selector => np.querySelector(selector);
        const barCanvas = q('[data-nm-viz]');
        const npCanvas = nq('[data-np-viz]');
        const seek = q('[data-nm-seek]');
        const volumeInput = q('[data-nm-volume]');
        volumeInput.value = String(audio.volume);

        function barMarkup() {
            return `<button type="button" class="nm-player-cover" data-nm-act="expand" aria-label="${esc(t('desktop.noisemaker_player_expand'))}" title="${esc(t('desktop.noisemaker_player_expand'))}"><span class="nm-cover nm-player-cover-img" data-nm-cover></span></button>
            <div class="nm-player-info">
                <div class="nm-player-title" data-nm-title></div>
                <div class="nm-player-meta nm-muted" data-nm-meta></div>
            </div>
            <div class="nm-player-center">
                <div class="nm-player-controls">
                    <button type="button" class="nm-icon-btn nm-player-mode" data-nm-act="shuffle" aria-pressed="false" aria-label="${esc(t('desktop.noisemaker_player_shuffle'))}" title="${esc(t('desktop.noisemaker_player_shuffle'))}">${SVG.shuffle}</button>
                    <button type="button" class="nm-icon-btn" data-nm-act="prev" aria-label="${esc(t('desktop.noisemaker_player_previous'))}" title="${esc(t('desktop.noisemaker_player_previous'))}">${SVG.prev}</button>
                    <button type="button" class="nm-player-play" data-nm-act="toggle" aria-label="${esc(t('desktop.noisemaker_player_play'))}" title="${esc(t('desktop.noisemaker_player_play'))}">${SVG.play}${SVG.pause}</button>
                    <button type="button" class="nm-icon-btn" data-nm-act="next" aria-label="${esc(t('desktop.noisemaker_player_next'))}" title="${esc(t('desktop.noisemaker_player_next'))}">${SVG.next}</button>
                    <button type="button" class="nm-icon-btn nm-player-mode" data-nm-act="repeat" data-repeat="off" aria-label="${esc(t('desktop.noisemaker_player_repeat_off'))}" title="${esc(t('desktop.noisemaker_player_repeat_off'))}">${SVG.repeat}<span class="nm-repeat-one" aria-hidden="true">1</span></button>
                </div>
                <div class="nm-player-track">
                    <span class="nm-player-time" data-nm-current>0:00</span>
                    <input type="range" class="nm-range nm-seek" data-nm-seek min="0" max="1000" value="0" step="1" aria-label="${esc(t('desktop.noisemaker_player_seek'))}">
                    <span class="nm-player-time" data-nm-total>0:00</span>
                </div>
            </div>
            <div class="nm-player-extras">
                <canvas class="nm-player-viz" data-nm-viz width="96" height="48" aria-hidden="true"></canvas>
                <button type="button" class="nm-icon-btn nm-player-fav" data-nm-act="favorite" aria-pressed="false" aria-label="${esc(t('desktop.noisemaker_favorite_add'))}" title="${esc(t('desktop.noisemaker_favorite_add'))}" ${readonly ? 'disabled' : ''}>${SVG.heart}</button>
                <div class="nm-player-volume">
                    <button type="button" class="nm-icon-btn nm-player-mute" data-nm-act="mute" aria-pressed="false" aria-label="${esc(t('desktop.noisemaker_player_mute'))}" title="${esc(t('desktop.noisemaker_player_mute'))}">${SVG.volume}${SVG.muted}</button>
                    <input type="range" class="nm-range nm-volume" data-nm-volume min="0" max="1" step="0.01" value="0.9" aria-label="${esc(t('desktop.noisemaker_player_volume'))}">
                </div>
                <button type="button" class="nm-icon-btn nm-player-expand" data-nm-act="expand" aria-expanded="false" aria-label="${esc(t('desktop.noisemaker_player_expand'))}" title="${esc(t('desktop.noisemaker_player_expand'))}">${SVG.expand}${SVG.collapse}</button>
            </div>`;
        }

        function nowPlayingMarkup() {
            return `<div class="nm-np-backdrop" data-np-backdrop aria-hidden="true"></div>
            <div class="nm-np-inner">
                <div class="nm-np-top">
                    <button type="button" class="nm-btn nm-np-back" data-np-back>${SVG.back}<span>${esc(t('desktop.noisemaker_back_to_library'))}</span></button>
                    <span class="nm-np-label">${esc(t('desktop.noisemaker_now_playing'))}</span>
                </div>
                <div class="nm-np-body">
                    <div class="nm-np-hero">
                        <div class="nm-cover nm-np-cover" data-np-cover></div>
                        <h2 class="nm-np-title" data-np-title></h2>
                        <div class="nm-np-chips" data-np-chips></div>
                        <div class="nm-np-meta nm-muted" data-np-meta></div>
                        <canvas class="nm-np-viz" data-np-viz height="72" aria-hidden="true"></canvas>
                        <div class="nm-np-actions">
                            <button type="button" class="nm-btn nm-np-fav" data-np-action="favorite" aria-pressed="false" ${readonly ? 'disabled' : ''}>${SVG.heart}<span data-np-fav-label>${esc(t('desktop.noisemaker_favorite_add'))}</span></button>
                            <button type="button" class="nm-btn" data-np-action="download">${SVG.download}<span>${esc(t('desktop.noisemaker_track_download'))}</span></button>
                            <button type="button" class="nm-btn" data-np-action="template">${SVG.template}<span>${esc(t('desktop.noisemaker_track_use_template'))}</span></button>
                            ${readonly ? '' : `<button type="button" class="nm-btn nm-btn--danger" data-np-action="delete">${SVG.trash}<span>${esc(t('desktop.noisemaker_track_delete'))}</span></button>`}
                        </div>
                        <section class="nm-np-lyrics">
                            <h3>${esc(t('desktop.noisemaker_lyrics_label'))}</h3>
                            <pre class="nm-lyrics-text" data-np-lyrics></pre>
                        </section>
                    </div>
                    <aside class="nm-np-queue">
                        <div class="nm-np-queue-head">
                            <h3>${esc(t('desktop.noisemaker_queue_title'))} <span class="nm-muted" data-np-queue-count></span></h3>
                            <button type="button" class="nm-btn nm-btn--small" data-np-queue-clear>${esc(t('desktop.noisemaker_queue_clear'))}</button>
                        </div>
                        <ol class="nm-queue-list" data-np-queue></ol>
                    </aside>
                </div>
            </div>`;
        }

        // ---- queue model -------------------------------------------------

        function current() { return pos >= 0 && pos < order.length ? queue[order[pos]] : null; }
        function isPlaying() { return !!current() && !audio.paused && !audio.ended; }
        function trackTitle(track) { return (track && track.title) || t('desktop.noisemaker_result_untitled'); }
        function sameId(a, b) { return String(a) === String(b); }
        function playable(list) { return (list || []).filter(x => x && x.web_path); }

        function buildOrder(startIndex) {
            const ids = queue.map((_, i) => i);
            if (!shuffle) {
                order = ids;
                pos = startIndex;
                return;
            }
            const rest = ids.filter(i => i !== startIndex);
            for (let i = rest.length - 1; i > 0; i--) {
                const j = Math.floor(Math.random() * (i + 1));
                const tmp = rest[i]; rest[i] = rest[j]; rest[j] = tmp;
            }
            order = startIndex >= 0 ? [startIndex].concat(rest) : rest;
            pos = startIndex >= 0 ? 0 : -1;
        }

        function play(track, context) {
            if (!track || !track.web_path) return;
            const cur = current();
            if (cur && sameId(cur.id, track.id)) { toggle(); return; }
            if (Array.isArray(context) && context.length) {
                const list = playable(context);
                let idx = list.findIndex(x => sameId(x.id, track.id));
                if (idx < 0) { list.unshift(track); idx = 0; }
                queue = list;
                buildOrder(idx);
            } else {
                let idx = queue.findIndex(x => sameId(x.id, track.id));
                if (idx < 0) {
                    queue.push(track);
                    idx = queue.length - 1;
                    if (shuffle) order.push(idx); else order = queue.map((_, i) => i);
                }
                pos = order.indexOf(idx);
            }
            startCurrent();
            renderQueue();
        }

        function setQueue(list, index) {
            queue = playable(list);
            const idx = Math.min(Math.max(0, Number(index) || 0), Math.max(0, queue.length - 1));
            buildOrder(queue.length ? idx : -1);
            if (queue.length) startCurrent(); else stopAudio();
            renderQueue();
        }

        function enqueue(list) {
            const known = new Set(queue.map(x => String(x.id)));
            const fresh = playable(list).filter(x => !known.has(String(x.id)));
            if (!fresh.length) return 0;
            const start = queue.length;
            queue = queue.concat(fresh);
            const added = fresh.map((_, i) => start + i);
            if (shuffle) {
                for (let i = added.length - 1; i > 0; i--) {
                    const j = Math.floor(Math.random() * (i + 1));
                    const tmp = added[i]; added[i] = added[j]; added[j] = tmp;
                }
                order = order.concat(added);
            } else {
                order = queue.map((_, i) => i);
            }
            if (pendingAutoplay) {
                pendingAutoplay = false;
                pos = pos + 1 < order.length ? pos + 1 : pos;
                startCurrent();
            }
            renderQueue();
            renderBar();
            return fresh.length;
        }

        function clearQueue() {
            const cur = current();
            queue = cur ? [cur] : [];
            buildOrder(cur ? 0 : -1);
            renderQueue();
            renderBar();
        }

        function removeQueuePosition(i) {
            if (i < 0 || i >= order.length) return;
            const qi = order[i];
            const wasCurrent = i === pos;
            queue.splice(qi, 1);
            order = order.filter(x => x !== qi).map(x => (x > qi ? x - 1 : x));
            if (wasCurrent) {
                if (!order.length) { pos = -1; stopAudio(); }
                else { pos = Math.min(i, order.length - 1); startCurrent(); }
            } else if (i < pos) {
                pos -= 1;
            }
            renderQueue();
            renderBar();
        }

        function removeTracks(ids) {
            const remove = new Set((ids || []).map(String));
            if (!remove.size || !queue.length) return;
            const cur = current();
            const curRemoved = !!(cur && remove.has(String(cur.id)));
            let nextQueueIndex = -1;
            if (curRemoved) {
                for (let k = pos + 1; k < order.length; k++) {
                    if (!remove.has(String(queue[order[k]].id))) { nextQueueIndex = order[k]; break; }
                }
            }
            const oldQueue = queue;
            const remap = new Map();
            queue = [];
            oldQueue.forEach((track, i) => {
                if (!remove.has(String(track.id))) { remap.set(i, queue.length); queue.push(track); }
            });
            order = order.filter(i => remap.has(i)).map(i => remap.get(i));
            if (curRemoved) {
                if (nowPlayingOpen) setNowPlayingOpen(false);
                if (nextQueueIndex >= 0) { pos = order.indexOf(remap.get(nextQueueIndex)); startCurrent(); }
                else { pos = -1; stopAudio(); }
            } else if (cur) {
                pos = order.indexOf(remap.get(oldQueue.indexOf(cur)));
            }
            renderQueue();
        }

        function updateTrack(track) {
            if (!track) return;
            let touchedCurrent = false;
            queue = queue.map(x => {
                if (!sameId(x.id, track.id)) return x;
                if (x === current()) touchedCurrent = true;
                return Object.assign({}, x, track);
            });
            if (touchedCurrent) { renderBar(); renderNowPlaying(); } else { renderQueue(); }
        }

        // ---- transport ---------------------------------------------------

        function startCurrent() {
            const track = current();
            if (!track) { stopAudio(); return; }
            ensureAudioGraph();
            resumeContext();
            audio.src = track.web_path;
            seek.value = '0';
            audio.play().catch(() => {});
            renderBar();
            renderTime();
            renderNowPlaying();
            emitState();
        }

        function stopAudio() {
            audio.pause();
            audio.removeAttribute('src');
            try { audio.load(); } catch (_) { /* ignore */ }
            if (nowPlayingOpen) setNowPlayingOpen(false);
            renderBar();
            renderNowPlaying();
            emitState();
        }

        function toggle() {
            if (!current()) {
                if (!order.length) return;
                pos = 0;
                startCurrent();
                return;
            }
            if (audio.paused) { resumeContext(); audio.play().catch(() => {}); }
            else audio.pause();
        }

        function next() {
            if (!order.length) return;
            if (pos + 1 < order.length) { pos += 1; startCurrent(); return; }
            if (repeat === 'all') { pos = 0; startCurrent(); return; }
            if (typeof deps.hasMore === 'function' && deps.hasMore()) { pendingAutoplay = true; emit('needmore'); }
        }

        function prev() {
            if (!order.length) return;
            if (audio.currentTime > 3 || pos <= 0) { audio.currentTime = 0; return; }
            pos -= 1;
            startCurrent();
        }

        function onEnded() {
            if (repeat === 'one') { audio.currentTime = 0; audio.play().catch(() => {}); return; }
            if (pos + 1 < order.length) { pos += 1; startCurrent(); return; }
            if (repeat === 'all' && order.length) { pos = 0; startCurrent(); return; }
            if (typeof deps.hasMore === 'function' && deps.hasMore()) { pendingAutoplay = true; emit('needmore'); return; }
            renderBar();
            emitState();
        }

        function onError() {
            if (!audio.getAttribute('src')) return; // clearing src fires a benign error in some browsers
            const track = current();
            errorStreak += 1;
            emit('error', track);
            if (errorStreak >= 2 || pos + 1 >= order.length) { audio.pause(); renderBar(); emitState(); return; }
            pos += 1;
            startCurrent();
        }

        function emitState() { emit('state', { track: current(), playing: isPlaying() }); }
        function prefsSnapshot() { return { shuffle, repeat, volume: audio.volume, muted: audio.muted, visualizer: visualizerOn }; }
        function emitChange() { emit('change', prefsSnapshot()); }

        function setShuffle(value) {
            shuffle = !!value;
            const cur = current();
            buildOrder(cur ? queue.indexOf(cur) : -1);
            renderBar();
            renderQueue();
            emitChange();
        }
        function setRepeat(mode) {
            repeat = ['off', 'all', 'one'].includes(mode) ? mode : 'off';
            renderBar();
            emitChange();
        }
        function setVolume(value) {
            audio.volume = clamp01(value);
            renderBar();
            emitChange();
        }
        function setMuted(value) {
            audio.muted = !!value;
            renderBar();
            emitChange();
        }
        function setVisualizer(value) {
            visualizerOn = !!value;
            if (visualizerOn) startLoop();
            else { stopLoop(); clearCanvas(barCanvas); clearCanvas(npCanvas); }
            renderBar();
            emitChange();
        }
        function setNowPlayingOpen(value) {
            const open = !!value && !!current();
            if (open === nowPlayingOpen) { if (!open) np.hidden = true; return; }
            nowPlayingOpen = open;
            renderNowPlaying();
            renderBar();
            if (open) startLoop();
            emit('expand', open);
        }

        // ---- visualizer --------------------------------------------------

        function closeAudioContext() {
            if (!audioCtx) return;
            try {
                if (typeof audioCtx.close === 'function') {
                    const p = audioCtx.close();
                    if (p && p.catch) p.catch(() => {});
                }
            } catch (_) { /* ignore */ }
            audioCtx = null;
        }

        function ensureAudioGraph() {
            if (audioCtx || !visualizerAvailable) return;
            const Ctx = window.AudioContext || window.webkitAudioContext;
            try {
                audioCtx = new Ctx();
                const source = audioCtx.createMediaElementSource(audio);
                try {
                    analyser = audioCtx.createAnalyser();
                    analyser.fftSize = 256;
                    analyser.smoothingTimeConstant = 0.82;
                    source.connect(analyser);
                    analyser.connect(audioCtx.destination);
                    freqData = new Uint8Array(analyser.frequencyBinCount);
                } catch (_) {
                    source.connect(audioCtx.destination); // keep audio audible without analyser
                    markVisualizerUnavailable();
                }
            } catch (_) {
                closeAudioContext();
                markVisualizerUnavailable();
            }
        }

        function markVisualizerUnavailable() {
            analyser = null;
            freqData = null;
            visualizerAvailable = false;
            stopLoop();
            bar.classList.add('is-viz-off');
            np.classList.add('is-viz-off');
            emit('visualizer-unavailable');
        }

        function resumeContext() {
            if (audioCtx && audioCtx.state === 'suspended') audioCtx.resume().catch(() => {});
        }

        function shouldAnimate() {
            return !disposed && !!analyser && visualizerOn && !audio.paused && !document.hidden && (typeof deps.isVisible !== 'function' || deps.isVisible());
        }

        function startLoop() {
            if (raf || visTimer || !analyser || !visualizerOn) return;
            if (reducedMotion) { paintIdle(); return; }
            if (shouldAnimate()) raf = requestAnimationFrame(drawFrame);
            else if (!audio.paused && !disposed) visTimer = setTimeout(() => { visTimer = null; startLoop(); }, 1000);
        }

        function stopLoop() {
            if (raf) cancelAnimationFrame(raf);
            raf = 0;
            if (visTimer) clearTimeout(visTimer);
            visTimer = null;
        }

        function drawFrame() {
            raf = 0;
            if (!shouldAnimate()) {
                if (!audio.paused && !disposed) visTimer = setTimeout(() => { visTimer = null; startLoop(); }, 1000);
                return;
            }
            analyser.getByteFrequencyData(freqData);
            paintBars(barCanvas, 12, i => (freqData[i] || 0) / 255);
            if (nowPlayingOpen) paintBars(npCanvas, 48, i => (freqData[i] || 0) / 255);
            raf = requestAnimationFrame(drawFrame);
        }

        function fitCanvas(canvas) {
            const dpr = window.devicePixelRatio || 1;
            const w = Math.round((canvas.clientWidth || canvas.width / dpr) * dpr);
            const h = Math.round((canvas.clientHeight || canvas.height / dpr) * dpr);
            if (w && canvas.width !== w) canvas.width = w;
            if (h && canvas.height !== h) canvas.height = h;
        }

        function paintBars(canvas, bars, level) {
            if (!canvas) return;
            const ctx2d = canvas.getContext('2d');
            if (!ctx2d) return;
            fitCanvas(canvas);
            const w = canvas.width;
            const h = canvas.height;
            ctx2d.clearRect(0, 0, w, h);
            ctx2d.fillStyle = getComputedStyle(canvas).color || '#8b5cf6';
            const gap = Math.max(1, Math.round((w / bars) * 0.3));
            const bw = (w - gap * (bars - 1)) / bars;
            const bins = freqData ? freqData.length : 128;
            const step = Math.max(1, Math.floor((bins * 0.75) / bars));
            for (let i = 0; i < bars; i++) {
                let sum = 0;
                for (let j = 0; j < step; j++) sum += level(i * step + j) || 0;
                const v = Math.min(1, sum / step);
                const bh = Math.max(2, v * h);
                ctx2d.fillRect(i * (bw + gap), h - bh, bw, bh);
            }
        }

        function paintIdle() {
            if (!visualizerOn) return;
            paintBars(barCanvas, 12, () => 0.4);
            if (nowPlayingOpen) paintBars(npCanvas, 48, () => 0.4);
        }

        function clearCanvas(canvas) {
            if (!canvas) return;
            const ctx2d = canvas.getContext('2d');
            if (ctx2d) ctx2d.clearRect(0, 0, canvas.width, canvas.height);
        }

        // ---- rendering ---------------------------------------------------

        function coverHTML(track) {
            return track && track.cover_url ? `<img src="${esc(track.cover_url)}" alt="" decoding="async">` : `<span class="nm-cover-fallback">${SVG.note}</span>`;
        }

        function setLabel(el, label) {
            if (!el) return;
            el.setAttribute('aria-label', label);
            el.title = label;
        }

        function renderBar() {
            const track = current();
            const playing = isPlaying();
            bar.classList.toggle('is-visible', !!track);
            bar.classList.toggle('is-playing', playing);
            bar.classList.toggle('is-shuffle', shuffle);
            bar.classList.toggle('is-muted', audio.muted);
            bar.classList.toggle('is-open', nowPlayingOpen);
            bar.classList.toggle('is-viz-off', !visualizerOn || !visualizerAvailable);
            q('[data-nm-act="shuffle"]').setAttribute('aria-pressed', shuffle ? 'true' : 'false');
            q('[data-nm-act="mute"]').setAttribute('aria-pressed', audio.muted ? 'true' : 'false');
            setLabel(q('[data-nm-act="mute"]'), audio.muted ? t('desktop.noisemaker_player_unmute') : t('desktop.noisemaker_player_mute'));
            const repeatBtn = q('[data-nm-act="repeat"]');
            repeatBtn.dataset.repeat = repeat;
            repeatBtn.classList.toggle('is-active', repeat !== 'off');
            repeatBtn.setAttribute('aria-pressed', repeat !== 'off' ? 'true' : 'false');
            setLabel(repeatBtn, t('desktop.noisemaker_player_repeat_' + repeat));
            setLabel(q('[data-nm-act="toggle"]'), playing ? t('desktop.noisemaker_player_pause') : t('desktop.noisemaker_player_play'));
            const expandBtn = q('.nm-player-expand');
            expandBtn.setAttribute('aria-expanded', nowPlayingOpen ? 'true' : 'false');
            setLabel(expandBtn, nowPlayingOpen ? t('desktop.noisemaker_player_collapse') : t('desktop.noisemaker_player_expand'));
            setLabel(q('.nm-player-cover'), nowPlayingOpen ? t('desktop.noisemaker_player_collapse') : t('desktop.noisemaker_player_expand'));
            if (volumeInput.value !== String(audio.volume)) volumeInput.value = String(audio.volume);
            if (!track) return;
            q('[data-nm-cover]').innerHTML = coverHTML(track);
            q('[data-nm-title]').textContent = trackTitle(track);
            q('[data-nm-meta]').textContent = [track.style || '', track.provider || ''].filter(Boolean).join(' · ');
            const fav = q('[data-nm-act="favorite"]');
            fav.setAttribute('aria-pressed', track.favorite ? 'true' : 'false');
            fav.classList.toggle('is-active', !!track.favorite);
            setLabel(fav, track.favorite ? t('desktop.noisemaker_favorite_remove') : t('desktop.noisemaker_favorite_add'));
        }

        function renderTime() {
            const track = current();
            const dur = Number.isFinite(audio.duration) && audio.duration > 0 ? audio.duration : (track && track.duration_ms ? track.duration_ms / 1000 : 0);
            q('[data-nm-current]').textContent = formatDuration(audio.currentTime * 1000);
            q('[data-nm-total]').textContent = formatDuration(dur * 1000);
            if (dur && !seeking) seek.value = String(Math.round((audio.currentTime / dur) * 1000));
            seek.style.setProperty('--nm-progress', (dur ? (audio.currentTime / dur) * 100 : 0).toFixed(2) + '%');
        }

        function renderNowPlaying() {
            const track = current();
            const open = nowPlayingOpen && !!track;
            np.hidden = !open;
            np.classList.toggle('is-open', open);
            if (!track) return;
            nq('[data-np-backdrop]').style.backgroundImage = track.cover_url ? 'url("' + String(track.cover_url).replace(/["\\\n\r]/g, '') + '")' : '';
            nq('[data-np-cover]').innerHTML = coverHTML(track);
            nq('[data-np-title]').textContent = trackTitle(track);
            const chips = String(track.style || '').split(',').map(s => s.trim()).filter(Boolean).slice(0, 8);
            nq('[data-np-chips]').innerHTML = chips.map(c => `<span class="nm-chip">${esc(c)}</span>`).join('')
                + (track.instrumental ? `<span class="nm-chip nm-chip--muted">${esc(t('desktop.noisemaker_instrumental_tag'))}</span>` : '');
            const meta = [];
            if (track.duration_ms) meta.push(formatDuration(track.duration_ms));
            if (track.provider) meta.push(track.provider + (track.model ? ' · ' + track.model : ''));
            const date = formatDate(track.created_at, deps.lang);
            if (date) meta.push(date);
            nq('[data-np-meta]').textContent = meta.join('  ·  ');
            const lyrics = String(track.lyrics || '').trim();
            const lyricsEl = nq('[data-np-lyrics]');
            lyricsEl.textContent = lyrics || t('desktop.noisemaker_lyrics_empty');
            lyricsEl.classList.toggle('is-empty', !lyrics);
            const fav = nq('[data-np-action="favorite"]');
            fav.setAttribute('aria-pressed', track.favorite ? 'true' : 'false');
            fav.classList.toggle('is-active', !!track.favorite);
            nq('[data-np-fav-label]').textContent = track.favorite ? t('desktop.noisemaker_favorite_remove') : t('desktop.noisemaker_favorite_add');
            renderQueue();
            if (open && (audio.paused || reducedMotion)) paintIdle();
        }

        function renderQueue() {
            const list = nq('[data-np-queue]');
            nq('[data-np-queue-count]').textContent = queue.length ? String(queue.length) : '';
            nq('[data-np-queue-clear]').disabled = queue.length <= 1;
            if (!order.length) {
                list.innerHTML = `<li class="nm-queue-empty nm-muted">${esc(t('desktop.noisemaker_queue_empty'))}</li>`;
                return;
            }
            list.innerHTML = order.map((qi, i) => {
                const track = queue[qi];
                const cur = i === pos;
                return `<li class="nm-queue-item${cur ? ' is-current' : ''}">
                    <button type="button" class="nm-queue-main" data-queue-play="${i}" aria-current="${cur ? 'true' : 'false'}">
                        <span class="nm-queue-index">${cur ? '<span class="nm-eq nm-eq--inline" aria-hidden="true"><i></i><i></i><i></i></span>' : (i + 1)}</span>
                        <span class="nm-cover nm-queue-cover">${coverHTML(track)}</span>
                        <span class="nm-queue-text"><span class="nm-queue-title">${esc(trackTitle(track))}</span><span class="nm-queue-sub nm-muted">${esc(track.style || track.provider || '')}</span></span>
                        <span class="nm-queue-duration nm-muted">${track.duration_ms ? esc(formatDuration(track.duration_ms)) : ''}</span>
                    </button>
                    <button type="button" class="nm-icon-btn nm-queue-remove" data-queue-remove="${i}" aria-label="${esc(t('desktop.noisemaker_queue_remove'))}" title="${esc(t('desktop.noisemaker_queue_remove'))}">${SVG.close}</button>
                </li>`;
            }).join('');
        }

        // ---- events ------------------------------------------------------

        audio.addEventListener('playing', () => { errorStreak = 0; renderBar(); startLoop(); emitState(); });
        audio.addEventListener('pause', () => { renderBar(); stopLoop(); paintIdle(); emitState(); });
        audio.addEventListener('ended', onEnded);
        audio.addEventListener('timeupdate', renderTime);
        audio.addEventListener('loadedmetadata', () => { renderTime(); renderBar(); });
        audio.addEventListener('error', onError);
        function onVisibility() { if (document.hidden) stopLoop(); else startLoop(); }
        document.addEventListener('visibilitychange', onVisibility);

        bar.addEventListener('click', event => {
            const btn = event.target.closest('[data-nm-act]');
            if (!btn) return;
            switch (btn.dataset.nmAct) {
                case 'toggle': toggle(); break;
                case 'next': next(); break;
                case 'prev': prev(); break;
                case 'shuffle': setShuffle(!shuffle); break;
                case 'repeat': setRepeat(repeat === 'off' ? 'all' : (repeat === 'all' ? 'one' : 'off')); break;
                case 'mute': setMuted(!audio.muted); break;
                case 'favorite': { const track = current(); if (track && !readonly) emit('favorite', track, !track.favorite); break; }
                case 'expand': setNowPlayingOpen(!nowPlayingOpen); break;
                default: break;
            }
        });

        function applySeek() {
            const dur = Number.isFinite(audio.duration) ? audio.duration : 0;
            if (dur) audio.currentTime = (Number(seek.value) / 1000) * dur;
            seeking = false;
        }
        seek.addEventListener('pointerdown', () => { seeking = true; });
        seek.addEventListener('input', () => {
            const dur = Number.isFinite(audio.duration) ? audio.duration : 0;
            if (!dur) return;
            seeking = true;
            q('[data-nm-current]').textContent = formatDuration((Number(seek.value) / 1000) * dur * 1000);
        });
        seek.addEventListener('change', applySeek);
        seek.addEventListener('pointerup', applySeek);
        seek.addEventListener('pointercancel', applySeek);
        volumeInput.addEventListener('input', () => {
            const value = clamp01(volumeInput.value);
            if (audio.muted && value > 0) audio.muted = false;
            setVolume(value);
        });

        np.addEventListener('click', event => {
            if (event.target.closest('[data-np-back]')) { setNowPlayingOpen(false); return; }
            if (event.target.closest('[data-np-queue-clear]')) { clearQueue(); return; }
            const playBtn = event.target.closest('[data-queue-play]');
            if (playBtn) {
                const i = Number(playBtn.dataset.queuePlay);
                if (i === pos) toggle(); else { pos = i; startCurrent(); renderQueue(); }
                return;
            }
            const removeBtn = event.target.closest('[data-queue-remove]');
            if (removeBtn) { removeQueuePosition(Number(removeBtn.dataset.queueRemove)); return; }
            const action = event.target.closest('[data-np-action]');
            const track = current();
            if (!action || !track) return;
            switch (action.dataset.npAction) {
                case 'favorite': if (!readonly) emit('favorite', track, !track.favorite); break;
                case 'download': emit('download', track); break;
                case 'template': emit('template', track); break;
                case 'delete': emit('delete', track); break;
                default: break;
            }
        });

        function dispose() {
            disposed = true;
            stopLoop();
            document.removeEventListener('visibilitychange', onVisibility);
            try { audio.pause(); audio.removeAttribute('src'); audio.load(); } catch (_) { /* ignore */ }
            closeAudioContext();
            analyser = null;
            Object.keys(handlers).forEach(key => { handlers[key] = []; });
            bar.remove();
            np.remove();
        }

        renderBar();

        return {
            barElement: bar, nowPlayingElement: np,
            play, toggle, next, prev, enqueue, setQueue, queue: () => queue.slice(), clearQueue,
            current, isPlaying,
            setShuffle, setRepeat, setVolume, setMuted, setVisualizer, setNowPlayingOpen,
            isNowPlayingOpen: () => nowPlayingOpen, visualizerAvailable: () => visualizerAvailable, prefs: prefsSnapshot,
            updateTrack, removeTracks, on, dispose
        };
    }

    window.NoisemakerPlayer = { create };
})();
