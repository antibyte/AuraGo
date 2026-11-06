(function () {
    'use strict';

    // Native AuraGo music player. Opening an audio file from the file manager
    // or the desktop launches this app in a normal window (never fullscreen)
    // with a live spectrum visualizer, playlist and OS media key support.

    const AUDIO_EXTENSIONS = new Set(['mp3', 'm4a', 'mp4', 'aac', 'flac', 'wav', 'ogg', 'oga', 'opus', 'aiff', 'aif', 'weba', 'wma', 'amr', 'mid', 'midi', 'm4b']);
    const SPEEDS = [0.5, 0.75, 1, 1.25, 1.5, 2];
    const REPEAT_MODES = ['off', 'all', 'one'];
    const QUEUE_LIMIT = 500;
    const LIBRARY_FOLDER = 'Music';
    const PREF_KEY = 'aurago.audio-player.prefs.v1';

    const instances = new Map();

    const ICONS = {
        play: '<path d="M8 5.4v13.2L19 12z" fill="currentColor" stroke="none"/>',
        pause: '<path d="M8 5h3.2v14H8zM13 5h3.2v14H13z" fill="currentColor" stroke="none"/>',
        prev: '<path d="M7 5.5h2.2v13H7zM19 5.5v13l-8.4-6.5z" fill="currentColor" stroke="none"/>',
        next: '<path d="M14.8 5.5H17v13h-2.2zM5 5.5v13l8.4-6.5z" fill="currentColor" stroke="none"/>',
        shuffle: '<path d="M3.5 7h3.2l9.6 10H21M3.5 17h3.2l2.8-3M14.8 9.6l2.7-2.6H21M18 4l3 3-3 3M18 14l3 3-3 3" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>',
        repeat: '<path d="M4 11a5 5 0 0 1 5-5h9m0 0-3-2.6M18 6l-3 2.6M20 13a5 5 0 0 1-5 5H6m0 0 3 2.6M6 18l3-2.6" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>',
        volume: '<path d="M4 9.5h3.4L12 5.6v12.8L7.4 14.5H4z" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linejoin="round"/><path d="M15 9.2a4 4 0 0 1 0 5.6M17.6 6.6a7.6 7.6 0 0 1 0 10.8" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/>',
        mute: '<path d="M4 9.5h3.4L12 5.6v12.8L7.4 14.5H4z" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linejoin="round"/><path d="M16 9.5l5 5M21 9.5l-5 5" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/>',
        music: '<path d="M9 18V6.6l10-2v11" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/><circle cx="7" cy="18" r="2.4" fill="none" stroke="currentColor" stroke-width="1.7"/><circle cx="17" cy="15.6" r="2.4" fill="none" stroke="currentColor" stroke-width="1.7"/>',
        refresh: '<path d="M20 12a8 8 0 1 1-2.6-5.9M20 4v4.4h-4.4" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>',
        folder: '<path d="M3.5 6.5h5.2l1.8 2.2H20v9.3a1.5 1.5 0 0 1-1.5 1.5H5a1.5 1.5 0 0 1-1.5-1.5z" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/>'
    };

    function icon(name, className) {
        return `<svg class="${className || 'vd-player-icon'}" viewBox="0 0 24 24" aria-hidden="true" focusable="false">${ICONS[name] || ''}</svg>`;
    }

    function extensionOf(name) {
        const match = /\.([a-z0-9]+)$/i.exec(String(name || ''));
        return match ? match[1].toLowerCase() : '';
    }

    function isAudioName(name) {
        return AUDIO_EXTENSIONS.has(extensionOf(name));
    }

    function baseName(path) {
        const parts = String(path || '').split('/');
        return parts[parts.length - 1] || '';
    }

    function folderOf(path) {
        const parts = String(path || '').split('/');
        parts.pop();
        return parts.join('/');
    }

    function stripExtension(name) {
        return String(name || '').replace(/\.[^.]+$/, '');
    }

    function formatTime(seconds) {
        const total = Math.max(0, Math.floor(Number(seconds) || 0));
        const hours = Math.floor(total / 3600);
        const minutes = Math.floor((total % 3600) / 60);
        const rest = total % 60;
        const pad = value => String(value).padStart(2, '0');
        return hours > 0 ? `${hours}:${pad(minutes)}:${pad(rest)}` : `${minutes}:${pad(rest)}`;
    }

    function loadPrefs() {
        try {
            const raw = localStorage.getItem(PREF_KEY);
            return raw ? JSON.parse(raw) || {} : {};
        } catch (_) {
            return {};
        }
    }

    function savePrefs(prefs) {
        try {
            localStorage.setItem(PREF_KEY, JSON.stringify(prefs));
        } catch (_) { /* storage disabled */ }
    }

    function safeUrl(url) {
        try {
            return new URL(String(url || ''), window.location.origin).href;
        } catch (_) {
            return '';
        }
    }

    function isSameOrigin(url) {
        const resolved = safeUrl(url);
        if (!resolved) return false;
        if (resolved.startsWith('blob:') || resolved.startsWith('data:')) return true;
        try {
            return new URL(resolved).origin === window.location.origin;
        } catch (_) {
            return false;
        }
    }

    function render(host, windowId, ctx) {
        if (!host) return;
        const context = ctx || {};
        const t = context.t || ((key, vars) => {
            let out = String(key);
            Object.entries(vars || {}).forEach(([name, value]) => {
                out = out.split('{{' + name + '}}').join(String(value)).split('{' + name + '}').join(String(value));
            });
            return out;
        });
        const esc = context.esc || (value => String(value == null ? '' : value)
            .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;'));
        const api = context.api || (() => Promise.reject(new Error('api_unavailable')));
        const mediaPreviewURL = typeof context.mediaPreviewURL === 'function'
            ? context.mediaPreviewURL
            : (file => (file && file.web_path) || (file && file.path ? '/api/desktop/download?path=' + encodeURIComponent(file.path) + '&inline=1' : ''));

        const prefs = loadPrefs();
        const state = {
            tracks: [],
            index: -1,
            playing: false,
            shuffle: !!prefs.shuffle,
            repeat: REPEAT_MODES.indexOf(prefs.repeat) >= 0 ? prefs.repeat : 'off',
            volume: typeof prefs.volume === 'number' ? Math.min(1, Math.max(0, prefs.volume)) : 0.85,
            muted: false,
            speed: SPEEDS.indexOf(prefs.speed) >= 0 ? prefs.speed : 1,
            mode: prefs.mode || 'bars',
            folder: '',
            graph: null,
            visualizer: null,
            scrubbing: false,
            disposed: false,
            tagAbort: null,
            canVisualize: true
        };

        host.innerHTML = `
<div class="vd-player-shell">
<div class="vd-player">
  <section class="vd-player-stage">
    <div class="vd-player-glow" aria-hidden="true"></div>
    <canvas class="vd-player-canvas" data-canvas aria-hidden="true"></canvas>
    <div class="vd-player-now">
      <div class="vd-player-art">
        <div class="vd-player-cover" data-cover hidden>
          <img alt="" data-cover-img>
        </div>
        <div class="vd-player-badge" data-badge>${icon('music', 'vd-player-icon vd-player-icon--badge')}</div>
      </div>
      <div class="vd-player-now-copy">
        <div class="vd-player-title" data-title>${esc(t('desktop.audio_player_no_track'))}</div>
        <div class="vd-player-sub">
          <span data-artist></span>
          <span class="vd-player-sep" data-sep hidden>·</span>
          <span data-album></span>
        </div>
        <div class="vd-player-folder" data-folder></div>
      </div>
    </div>
  </section>
  <section class="vd-player-bar">
    <div class="vd-player-timeline">
      <span class="vd-player-time" data-current>0:00</span>
      <div class="vd-player-seek" data-seek role="slider" tabindex="0" aria-label="${esc(t('desktop.audio_player_seek'))}" aria-valuemin="0" aria-valuemax="100" aria-valuenow="0">
        <div class="vd-player-seek-track">
          <div class="vd-player-seek-buffer" data-buffer></div>
          <div class="vd-player-seek-fill" data-fill></div>
        </div>
        <div class="vd-player-seek-knob" data-knob></div>
        <div class="vd-player-seek-tip" data-seek-tip hidden>0:00</div>
      </div>
      <span class="vd-player-time" data-duration>0:00</span>
    </div>
    <div class="vd-player-controls">
      <div class="vd-player-group">
        <button type="button" class="vd-player-btn" data-shuffle aria-label="${esc(t('desktop.audio_player_shuffle'))}" aria-pressed="false">${icon('shuffle')}</button>
        <button type="button" class="vd-player-btn" data-repeat>${icon('repeat')}</button>
      </div>
      <div class="vd-player-group vd-player-group--main">
        <button type="button" class="vd-player-btn" data-prev aria-label="${esc(t('desktop.audio_player_previous'))}">${icon('prev')}</button>
        <button type="button" class="vd-player-btn vd-player-btn--play" data-play aria-label="${esc(t('desktop.audio_player_play'))}">${icon('play')}</button>
        <button type="button" class="vd-player-btn" data-next aria-label="${esc(t('desktop.audio_player_next'))}">${icon('next')}</button>
      </div>
      <div class="vd-player-group">
        <div class="vd-player-volume">
          <button type="button" class="vd-player-btn" data-mute aria-pressed="false">${icon('volume')}</button>
          <input class="vd-player-range" type="range" min="0" max="100" step="1" data-volume aria-label="${esc(t('desktop.audio_player_volume'))}">
        </div>
        <button type="button" class="vd-player-btn vd-player-btn--text" data-speed>1&times;</button>
        <button type="button" class="vd-player-btn vd-player-btn--text" data-visualizer-mode>Bars</button>
      </div>
    </div>
  </section>
  <aside class="vd-player-queue">
    <header class="vd-player-queue-head">
      <span class="vd-player-queue-title" data-queue-title>${esc(t('desktop.audio_player_queue'))}</span>
      <button type="button" class="vd-player-btn" data-reload title="${esc(t('desktop.audio_player_reload'))}" aria-label="${esc(t('desktop.audio_player_reload'))}">${icon('refresh')}</button>
    </header>
    <ol class="vd-player-queue-list" data-queue></ol>
    <footer class="vd-player-queue-foot" data-queue-foot></footer>
  </aside>
  <audio data-audio preload="metadata"></audio>
</div>
</div>`;

        const root = host.querySelector('.vd-player');
        const audio = host.querySelector('[data-audio]');
        const canvas = host.querySelector('[data-canvas]');
        const els = {
            badge: host.querySelector('[data-badge]'),
            cover: host.querySelector('[data-cover]'),
            coverImg: host.querySelector('[data-cover-img]'),
            title: host.querySelector('[data-title]'),
            artist: host.querySelector('[data-artist]'),
            album: host.querySelector('[data-album]'),
            sep: host.querySelector('[data-sep]'),
            folder: host.querySelector('[data-folder]'),
            current: host.querySelector('[data-current]'),
            duration: host.querySelector('[data-duration]'),
            seek: host.querySelector('[data-seek]'),
            fill: host.querySelector('[data-fill]'),
            buffer: host.querySelector('[data-buffer]'),
            knob: host.querySelector('[data-knob]'),
            tip: host.querySelector('[data-seek-tip]'),
            play: host.querySelector('[data-play]'),
            prev: host.querySelector('[data-prev]'),
            next: host.querySelector('[data-next]'),
            shuffle: host.querySelector('[data-shuffle]'),
            repeat: host.querySelector('[data-repeat]'),
            mute: host.querySelector('[data-mute]'),
            volume: host.querySelector('[data-volume]'),
            speed: host.querySelector('[data-speed]'),
            mode: host.querySelector('[data-visualizer-mode]'),
            queue: host.querySelector('[data-queue]'),
            queueTitle: host.querySelector('[data-queue-title]'),
            queueFoot: host.querySelector('[data-queue-foot]'),
            reload: host.querySelector('[data-reload]')
        };

        const MODE_LABELS = {
            bars: t('desktop.audio_player_mode_bars'),
            radial: t('desktop.audio_player_mode_radial'),
            wave: t('desktop.audio_player_mode_wave')
        };

        function persist() {
            savePrefs({
                shuffle: state.shuffle,
                repeat: state.repeat,
                volume: state.volume,
                speed: state.speed,
                mode: state.mode
            });
        }

        function currentTrack() {
            return state.index >= 0 ? state.tracks[state.index] : null;
        }

        // Web Audio graph: element -> analyser -> gain -> destination. The
        // analyser sits before the gain so the visualizer reacts to the music
        // itself and not to the current volume.
        function ensureGraph() {
            if (state.graph) return state.graph;
            // A MediaElementSource silences cross-origin media without CORS
            // headers, so the whole playlist must be same-origin before the
            // graph may be built.
            if (!state.canVisualize) return null;
            const track = currentTrack();
            if (track && !isSameOrigin(track.url)) return null;
            const Ctor = window.AudioContext || window.webkitAudioContext;
            if (!Ctor) return null;
            try {
                const audioContext = new Ctor();
                const source = audioContext.createMediaElementSource(audio);
                const analyser = audioContext.createAnalyser();
                analyser.fftSize = 2048;
                analyser.smoothingTimeConstant = 0.58;
                const gain = audioContext.createGain();
                source.connect(analyser);
                analyser.connect(gain);
                gain.connect(audioContext.destination);
                state.graph = { context: audioContext, source, analyser, gain };
                applyVolume();
                return state.graph;
            } catch (_) {
                state.graph = null;
                return null;
            }
        }

        function applyVolume() {
            const level = state.muted ? 0 : state.volume;
            if (state.graph) {
                audio.volume = 1;
                const now = state.graph.context.currentTime;
                state.graph.gain.gain.cancelScheduledValues(now);
                state.graph.gain.gain.setTargetAtTime(level * level, now, 0.03);
            } else {
                audio.volume = level;
            }
            if (els.volume) els.volume.value = String(Math.round(state.volume * 100));
            if (els.mute) {
                els.mute.innerHTML = icon(state.muted || state.volume === 0 ? 'mute' : 'volume');
                els.mute.setAttribute('aria-pressed', String(state.muted));
                els.mute.setAttribute('aria-label', t(state.muted ? 'desktop.audio_player_unmute' : 'desktop.audio_player_mute'));
            }
        }

        function updateMediaSession() {
            if (!window.AuraDesktopMediaSession) return;
            const track = currentTrack();
            const artwork = track && track.picture ? [{ src: track.picture, sizes: '512x512' }] : [];
            try {
                window.AuraDesktopMediaSession.claim(state, {
                    priority: 60,
                    activate: true,
                    playbackState: state.playing ? 'playing' : 'paused',
                    metadata: {
                        title: track ? (track.title || track.name) : t('desktop.audio_player_no_track'),
                        artist: track ? (track.artist || t('desktop.audio_player_unknown_artist')) : 'AuraGo',
                        album: track ? (track.album || state.folder || '') : '',
                        artwork
                    },
                    handlers: {
                        play: () => { play(); },
                        pause: () => { pause(); },
                        stop: () => { pause(); },
                        previoustrack: () => { previous(); },
                        nexttrack: () => { next(); },
                        seekbackward: details => { seekBy(-(details && details.seekOffset ? details.seekOffset : 10)); },
                        seekforward: details => { seekBy(details && details.seekOffset ? details.seekOffset : 10); },
                        seekto: details => { seekToRatio(details && details.seekTime && track && track.duration ? details.seekTime / track.duration : 0); }
                    }
                });
            } catch (_) { /* media session unavailable */ }
        }

        function setPlaying(playing) {
            state.playing = !!playing;
            if (els.play) {
                els.play.innerHTML = icon(state.playing ? 'pause' : 'play');
                els.play.setAttribute('aria-label', t(state.playing ? 'desktop.audio_player_pause' : 'desktop.audio_player_play'));
            }
            root.classList.toggle('is-playing', state.playing);
            updateMediaSession();
        }

        async function play() {
            if (!audio.getAttribute('src')) return;
            ensureGraph();
            if (state.graph && state.graph.context.state === 'suspended') {
                try { await state.graph.context.resume(); } catch (_) { /* needs a gesture */ }
            }
            try {
                await audio.play();
            } catch (_) {
                setPlaying(false);
                return;
            }
            setPlaying(true);
        }

        function pause() {
            audio.pause();
            setPlaying(false);
        }

        function toggle() {
            if (state.playing) pause();
            else play();
        }

        function nextIndex() {
            if (!state.tracks.length) return -1;
            if (state.shuffle && state.tracks.length > 1) {
                let candidate = state.index;
                while (candidate === state.index) candidate = Math.floor(Math.random() * state.tracks.length);
                return candidate;
            }
            const next = state.index + 1;
            if (next >= state.tracks.length) return state.repeat === 'all' ? 0 : -1;
            return next;
        }

        function loadTrack(index, autoplay) {
            if (index < 0 || index >= state.tracks.length) return;
            state.index = index;
            const track = state.tracks[index];
            audio.src = track.url;
            audio.playbackRate = state.speed;
            audio.load();
            renderMeta();
            renderQueue();
            loadTags(track);
            if (autoplay) play();
        }

        function next() {
            const target = nextIndex();
            if (target < 0) {
                pause();
                return;
            }
            loadTrack(target, true);
        }

        function previous() {
            if (!state.tracks.length) return;
            if (audio.currentTime > 3) {
                audio.currentTime = 0;
                return;
            }
            const target = state.index > 0 ? state.index - 1 : state.tracks.length - 1;
            loadTrack(target, true);
        }

        function seekToRatio(ratio) {
            const duration = audio.duration;
            if (!isFinite(duration) || duration <= 0) return;
            audio.currentTime = Math.min(duration, Math.max(0, ratio * duration));
        }

        function seekBy(seconds) {
            const duration = audio.duration;
            if (!isFinite(duration) || duration <= 0) return;
            audio.currentTime = Math.min(duration, Math.max(0, audio.currentTime + seconds));
        }

        function ratioFromEvent(event) {
            const rect = els.seek.getBoundingClientRect();
            if (!rect.width) return 0;
            return Math.min(1, Math.max(0, (event.clientX - rect.left) / rect.width));
        }

        function clearCoverTint() {
            state.coverTint = false;
            state.coverToken = null;
            if (els.coverImg) {
                els.coverImg.onload = null;
                els.coverImg.onerror = null;
                els.coverImg.removeAttribute('src');
            }
            root.style.removeProperty('--pl-cover');
            if (state.visualizer && state.visualizer.clearCoverColor) state.visualizer.clearCoverColor();
        }

        // The sleeve colour tints the glow, the transport and the spectrum.
        // Near-grey covers keep the theme accent.
        function paintCoverTint(img, token) {
            if (state.disposed || state.coverToken !== token) return;
            try {
                const sample = document.createElement('canvas');
                sample.width = 16;
                sample.height = 16;
                const ctx = sample.getContext('2d', { willReadFrequently: true });
                if (!ctx) return;
                ctx.drawImage(img, 0, 0, 16, 16);
                const data = ctx.getImageData(0, 0, 16, 16).data;
                let best = 0;
                let red = 0;
                let green = 0;
                let blue = 0;
                let averageR = 0;
                let averageG = 0;
                let averageB = 0;
                let count = 0;
                for (let i = 0; i < data.length; i += 4) {
                    if (data[i + 3] < 40) continue;
                    const pr = data[i];
                    const pg = data[i + 1];
                    const pb = data[i + 2];
                    const max = Math.max(pr, pg, pb);
                    const sat = max - Math.min(pr, pg, pb);
                    if (max < 28) continue;
                    averageR += pr;
                    averageG += pg;
                    averageB += pb;
                    count += 1;
                    if (sat > best && max < 250) {
                        best = sat;
                        red = pr;
                        green = pg;
                        blue = pb;
                    }
                }
                if (!count) return;
                if (best < 18) {
                    red = averageR / count;
                    green = averageG / count;
                    blue = averageB / count;
                }
                const tint = `rgb(${Math.round(red)}, ${Math.round(green)}, ${Math.round(blue)})`;
                state.coverTint = true;
                root.style.setProperty('--pl-cover', tint);
                if (state.visualizer && state.visualizer.setCoverColor) {
                    state.visualizer.setCoverColor(red, green, blue);
                }
            } catch (_) { /* an unreadable cover keeps the theme accent */ }
        }

        function showCover(picture) {
            const token = {};
            state.coverToken = token;
            els.coverImg.onload = () => paintCoverTint(els.coverImg, token);
            els.coverImg.onerror = () => {
                if (state.coverToken !== token) return;
                els.cover.hidden = true;
                els.badge.hidden = false;
                clearCoverTint();
            };
            if (els.coverImg.getAttribute('src') !== picture) els.coverImg.src = picture;
            else if (els.coverImg.complete) paintCoverTint(els.coverImg, token);
            els.cover.hidden = false;
            els.badge.hidden = true;
        }

        function renderMeta() {
            const track = currentTrack();
            if (!track) {
                els.title.textContent = t('desktop.audio_player_no_track');
                els.artist.textContent = '';
                els.album.textContent = '';
                els.sep.hidden = true;
                els.folder.textContent = state.folder || '';
                els.cover.hidden = true;
                els.badge.hidden = false;
                clearCoverTint();
                return;
            }
            els.title.textContent = track.title || stripExtension(track.name);
            els.artist.textContent = track.artist || '';
            els.album.textContent = track.album || '';
            els.sep.hidden = !(track.artist && track.album);
            els.folder.textContent = state.folder || '';
            if (track.picture) showCover(track.picture);
            else {
                els.cover.hidden = true;
                els.badge.hidden = false;
                clearCoverTint();
            }
        }

        function renderQueue() {
            els.queue.innerHTML = '';
            state.tracks.forEach((track, index) => {
                const item = document.createElement('li');
                item.className = 'vd-player-queue-item';
                item.dataset.index = String(index);
                if (index === state.index) item.classList.add('is-current');
                const title = track.title || stripExtension(track.name);
                const meta = [track.artist, track.album].filter(Boolean).join(' · ');
                item.innerHTML = `<button type="button" class="vd-player-queue-btn" data-select="${index}">
          <span class="vd-player-queue-mark">
            <span class="vd-player-queue-index">${index + 1}</span>
            <span class="vd-player-eq" aria-hidden="true"><span></span><span></span><span></span></span>
          </span>
          <span class="vd-player-queue-text">
            <span class="vd-player-track-name">${esc(title)}</span>
            ${meta ? `<span class="vd-player-queue-meta">${esc(meta)}</span>` : ''}
          </span>
          <span class="vd-player-queue-duration">${track.duration ? esc(formatTime(track.duration)) : ''}</span>
        </button>`;
                els.queue.appendChild(item);
            });
            const active = els.queue.querySelector('.is-current');
            if (active && typeof active.scrollIntoView === 'function') active.scrollIntoView({ block: 'nearest' });
            els.queueFoot.textContent = state.tracks.length
                ? t('desktop.audio_player_queue_count', { count: state.tracks.length })
                : t('desktop.audio_player_queue_empty');
        }

        async function loadTags(track) {
            if (!track || !window.AudioPlayerTags) return;
            if (state.tagAbort) state.tagAbort.abort();
            const controller = typeof AbortController === 'function' ? new AbortController() : null;
            state.tagAbort = controller;
            const result = await window.AudioPlayerTags.read(track.url, { signal: controller ? controller.signal : undefined }).catch(() => null);
            if (state.disposed || state.tagAbort !== controller) return;
            const tags = (result && result.tags) || {};
            if (tags.title) track.title = tags.title;
            if (tags.artist) track.artist = tags.artist;
            if (tags.album) track.album = tags.album;
            if (tags.year && !track.year) track.year = tags.year;
            if (result && result.picture && !track.picture) track.picture = result.picture;
            renderMeta();
            renderQueue();
            updateMediaSession();
        }

        function updateTimeline() {
            const duration = isFinite(audio.duration) ? audio.duration : (currentTrack() && currentTrack().duration) || 0;
            const position = audio.currentTime || 0;
            const ratio = duration > 0 ? Math.min(1, position / duration) : 0;
            els.fill.style.width = (ratio * 100).toFixed(2) + '%';
            els.knob.style.left = (ratio * 100).toFixed(2) + '%';
            els.current.textContent = formatTime(position);
            els.duration.textContent = formatTime(duration);
            els.seek.setAttribute('aria-valuenow', String(Math.round(ratio * 100)));
            els.seek.setAttribute('aria-valuetext', formatTime(position) + ' / ' + formatTime(duration));
            if (duration > 0) {
                let buffered = 0;
                try {
                    if (audio.buffered && audio.buffered.length) buffered = audio.buffered.end(audio.buffered.length - 1);
                } catch (_) { buffered = 0; }
                els.buffer.style.width = Math.min(100, (buffered / duration) * 100).toFixed(2) + '%';
            } else {
                els.buffer.style.width = '0%';
            }
            const track = currentTrack();
            if (track && duration > 0 && !track.duration) {
                track.duration = duration;
                renderQueue();
            }
        }

        function toggleShuffle() {
            state.shuffle = !state.shuffle;
            els.shuffle.classList.toggle('is-active', state.shuffle);
            els.shuffle.setAttribute('aria-pressed', String(state.shuffle));
            persist();
        }

        function cycleRepeat() {
            const nextMode = REPEAT_MODES[(REPEAT_MODES.indexOf(state.repeat) + 1) % REPEAT_MODES.length];
            state.repeat = nextMode;
            els.repeat.classList.toggle('is-active', nextMode !== 'off');
            els.repeat.dataset.repeatState = nextMode;
            els.repeat.setAttribute('aria-label', t('desktop.audio_player_repeat_' + nextMode));
            els.repeat.title = t('desktop.audio_player_repeat_' + nextMode);
            persist();
        }

        function cycleSpeed() {
            const nextSpeed = SPEEDS[(SPEEDS.indexOf(state.speed) + 1) % SPEEDS.length];
            state.speed = nextSpeed;
            audio.playbackRate = nextSpeed;
            els.speed.textContent = (nextSpeed === 1 ? '1' : String(nextSpeed).replace('0.', '.')) + '×';
            persist();
        }

        function cycleMode() {
            if (!state.visualizer) return;
            state.mode = state.visualizer.cycleMode();
            els.mode.textContent = MODE_LABELS[state.mode] || state.mode;
            persist();
        }

        function selectTrack(index) {
            if (index === state.index) {
                toggle();
                return;
            }
            loadTrack(index, true);
        }

        async function loadFolder(folder, focusPath, recursive) {
            state.folder = folder || '';
            const params = new URLSearchParams({ path: folder || '', limit: String(QUEUE_LIMIT) });
            if (recursive) params.set('recursive', 'true');
            let files = [];
            try {
                const body = await api('/api/desktop/files?' + params.toString());
                files = (body && body.files) || [];
            } catch (_) {
                files = [];
            }
            const tracks = files
                .filter(file => file && file.type === 'file' && isAudioName(file.name))
                .map(file => ({
                    name: file.name,
                    path: file.path,
                    url: mediaPreviewURL(file) || '',
                    title: '',
                    artist: '',
                    album: '',
                    duration: 0,
                    picture: ''
                }))
                .filter(track => !!track.url);
            state.tracks = tracks;
            // The spectrum needs a same-origin stream for every track of the
            // playlist, otherwise one remote file would mute the whole app.
            state.canVisualize = tracks.every(track => isSameOrigin(track.url));
            els.queueTitle.textContent = folder ? baseName(folder) || folder : t('desktop.audio_player_queue');
            const wanted = focusPath || (currentTrack() && currentTrack().path);
            let index = wanted ? tracks.findIndex(track => track.path === wanted) : -1;
            if (index < 0) index = tracks.length ? 0 : -1;
            state.index = -1;
            renderQueue();
            if (index >= 0) loadTrack(index, !!focusPath);
            else {
                renderMeta();
                updateMediaSession();
            }
        }

        function openPath(path) {
            if (!path) return;
            const folder = folderOf(path);
            loadFolder(folder, path);
        }

        function wireEvents() {
            els.play.addEventListener('click', toggle);
            els.prev.addEventListener('click', previous);
            els.next.addEventListener('click', next);
            els.shuffle.addEventListener('click', toggleShuffle);
            els.repeat.addEventListener('click', cycleRepeat);
            els.speed.addEventListener('click', cycleSpeed);
            els.mode.addEventListener('click', cycleMode);
            els.mute.addEventListener('click', () => {
                state.muted = !state.muted;
                applyVolume();
            });
            els.volume.addEventListener('input', () => {
                state.volume = Math.min(1, Math.max(0, Number(els.volume.value) / 100));
                state.muted = state.volume === 0 ? state.muted : false;
                applyVolume();
                persist();
            });
            els.reload.addEventListener('click', () => {
                const track = currentTrack();
                loadFolder(state.folder, track ? track.path : '');
            });
            els.queue.addEventListener('click', event => {
                const button = event.target.closest('[data-select]');
                if (!button) return;
                selectTrack(Number(button.dataset.select));
            });

            let dragging = false;
            const placeTip = event => {
                if (!els.tip) return;
                const duration = audio.duration;
                if (!isFinite(duration) || duration <= 0) {
                    els.tip.hidden = true;
                    return;
                }
                const ratio = ratioFromEvent(event);
                els.tip.hidden = false;
                els.tip.textContent = formatTime(ratio * duration);
                els.tip.style.left = (ratio * 100).toFixed(2) + '%';
            };
            const hideTip = () => { if (els.tip && !dragging) els.tip.hidden = true; };
            els.seek.addEventListener('pointerdown', event => {
                if (!isFinite(audio.duration)) return;
                dragging = true;
                state.scrubbing = true;
                root.classList.add('is-scrubbing');
                els.seek.setPointerCapture && els.seek.setPointerCapture(event.pointerId);
                seekToRatio(ratioFromEvent(event));
                placeTip(event);
            });
            els.seek.addEventListener('pointermove', event => {
                placeTip(event);
                if (!dragging) return;
                seekToRatio(ratioFromEvent(event));
            });
            els.seek.addEventListener('pointerleave', hideTip);
            const endDrag = () => {
                dragging = false;
                state.scrubbing = false;
                root.classList.remove('is-scrubbing');
                if (els.tip) els.tip.hidden = true;
            };
            els.seek.addEventListener('pointerup', endDrag);
            els.seek.addEventListener('pointercancel', endDrag);
            els.seek.addEventListener('keydown', event => {
                if (event.key === 'ArrowRight') { seekBy(5); event.preventDefault(); }
                else if (event.key === 'ArrowLeft') { seekBy(-5); event.preventDefault(); }
            });

            audio.addEventListener('play', () => setPlaying(true));
            audio.addEventListener('pause', () => setPlaying(false));
            audio.addEventListener('timeupdate', updateTimeline);
            audio.addEventListener('progress', updateTimeline);
            audio.addEventListener('loadedmetadata', updateTimeline);
            audio.addEventListener('durationchange', updateTimeline);
            audio.addEventListener('ended', () => {
                if (state.repeat === 'one') {
                    audio.currentTime = 0;
                    play();
                    return;
                }
                const target = nextIndex();
                if (target < 0) {
                    setPlaying(false);
                    return;
                }
                loadTrack(target, true);
            });
            audio.addEventListener('error', () => {
                if (state.disposed) return;
                setPlaying(false);
            });

            root.addEventListener('keydown', event => {
                if (event.target && /^(INPUT|TEXTAREA|SELECT)$/.test(event.target.tagName)) return;
                if (event.target && typeof event.target.closest === 'function' && event.target.closest('[data-seek]')) return;
                const key = event.key;
                if (key === ' ' || key === 'k') { toggle(); event.preventDefault(); }
                else if (key === 'ArrowRight') { seekBy(5); event.preventDefault(); }
                else if (key === 'ArrowLeft') { seekBy(-5); event.preventDefault(); }
                else if (key === 'ArrowUp') { state.volume = Math.min(1, state.volume + 0.05); applyVolume(); persist(); event.preventDefault(); }
                else if (key === 'ArrowDown') { state.volume = Math.max(0, state.volume - 0.05); applyVolume(); persist(); event.preventDefault(); }
                else if (key === 'm') { state.muted = !state.muted; applyVolume(); }
                else if (key === 's') { toggleShuffle(); }
                else if (key === 'r') { cycleRepeat(); }
                else if (key === 'v') { cycleMode(); }
                else if (key === 'n') { next(); }
                else if (key === 'p') { previous(); }
            });
        }

        function setWindowMenus() {
            if (typeof context.setWindowMenus !== 'function') return;
            context.setWindowMenus(windowId, [
                {
                    id: 'playback',
                    labelKey: 'desktop.menu_playback',
                    items: [
                        { id: 'play-pause', labelKey: state.playing ? 'desktop.audio_player_pause' : 'desktop.audio_player_play', icon: state.playing ? 'pause' : 'play', action: toggle },
                        { id: 'next', labelKey: 'desktop.audio_player_next', icon: 'next', action: next },
                        { id: 'previous', labelKey: 'desktop.audio_player_previous', icon: 'previous', action: previous },
                        { id: 'shuffle', labelKey: 'desktop.audio_player_shuffle', icon: 'shuffle', checked: state.shuffle, action: toggleShuffle },
                        { id: 'repeat', labelKey: 'desktop.audio_player_repeat_' + state.repeat, icon: 'repeat', checked: state.repeat !== 'off', action: cycleRepeat }
                    ]
                },
                {
                    id: 'view',
                    labelKey: 'desktop.menu_view',
                    items: [
                        { id: 'mode-bars', labelKey: 'desktop.audio_player_mode_bars', checked: state.mode === 'bars', action: () => setMode('bars') },
                        { id: 'mode-radial', labelKey: 'desktop.audio_player_mode_radial', checked: state.mode === 'radial', action: () => setMode('radial') },
                        { id: 'mode-wave', labelKey: 'desktop.audio_player_mode_wave', checked: state.mode === 'wave', action: () => setMode('wave') }
                    ]
                }
            ]);
        }

        function setMode(mode) {
            if (!state.visualizer) return;
            state.visualizer.setMode(mode);
            state.mode = mode;
            els.mode.textContent = MODE_LABELS[mode] || mode;
            persist();
            setWindowMenus();
        }

        function dispose() {
            if (state.disposed) return;
            state.disposed = true;
            try { audio.pause(); } catch (_) { /* ignore */ }
            audio.removeAttribute('src');
            try { audio.load(); } catch (_) { /* ignore */ }
            if (state.tagAbort) state.tagAbort.abort();
            if (state.themeObserver) state.themeObserver.disconnect();
            if (state.visualizer) state.visualizer.destroy();
            if (state.graph) {
                try { state.graph.context.close(); } catch (_) { /* ignore */ }
                state.graph = null;
            }
            if (window.AuraDesktopMediaSession) window.AuraDesktopMediaSession.release(state);
            instances.delete(windowId);
            if (typeof context.clearWindowMenus === 'function') context.clearWindowMenus(windowId);
        }

        // Initial state
        els.mode.textContent = MODE_LABELS[state.mode] || state.mode;
        els.speed.textContent = (state.speed === 1 ? '1' : String(state.speed).replace('0.', '.')) + '×';
        els.shuffle.classList.toggle('is-active', state.shuffle);
        els.shuffle.setAttribute('aria-pressed', String(state.shuffle));
        els.repeat.classList.toggle('is-active', state.repeat !== 'off');
        els.repeat.dataset.repeatState = state.repeat;
        els.repeat.setAttribute('aria-label', t('desktop.audio_player_repeat_' + state.repeat));
        els.repeat.title = t('desktop.audio_player_repeat_' + state.repeat);
        applyVolume();
        wireEvents();
        setWindowMenus();

        if (window.AudioPlayerVisualizer) {
            state.visualizer = window.AudioPlayerVisualizer.create(canvas, {
                mode: state.mode,
                root,
                getAnalyser: () => (state.graph ? state.graph.analyser : null),
                onEnergy: bass => {
                    root.style.setProperty('--pl-pulse', (Number(bass) || 0).toFixed(3));
                }
            });
        }

        if (typeof MutationObserver === 'function' && document.body) {
            state.themeObserver = new MutationObserver(() => {
                if (state.disposed || state.coverTint || !state.visualizer) return;
                state.visualizer.refreshPalette();
            });
            state.themeObserver.observe(document.body, {
                attributes: true,
                attributeFilter: ['data-accent', 'data-theme', 'data-scheme']
            });
        }

        instances.set(windowId, { state, open: path => openPath(path), dispose });
        if (typeof context.registerWindowCleanup === 'function') context.registerWindowCleanup(windowId, dispose);

        const launchPath = context.path || '';
        if (launchPath) openPath(launchPath);
        else {
            // Started from the dock: show the whole music library instead of an
            // empty window.
            loadFolder(LIBRARY_FOLDER, '', true);
        }
        root.tabIndex = 0;
        setTimeout(() => root.focus({ preventScroll: true }), 0);
    }

    function open(windowId, context) {
        const instance = instances.get(windowId);
        if (!instance) return;
        const path = context && context.path;
        if (path) instance.open(path);
    }

    window.AudioPlayerApp = Object.freeze({ render, open, instances });
})();
