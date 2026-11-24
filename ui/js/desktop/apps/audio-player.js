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
        folder: '<path d="M3.5 6.5h5.2l1.8 2.2H20v9.3a1.5 1.5 0 0 1-1.5 1.5H5a1.5 1.5 0 0 1-1.5-1.5z" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/>',
        eq: '<path d="M6 4v16M12 4v16M18 4v16" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/><circle cx="6" cy="9" r="2.1" fill="currentColor" stroke="none"/><circle cx="12" cy="15" r="2.1" fill="currentColor" stroke="none"/><circle cx="18" cy="10" r="2.1" fill="currentColor" stroke="none"/>',
        power: '<path d="M12 3.5v7.2M7.2 6.2a7 7 0 1 0 9.6 0" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>',
        reset: '<path d="M4.5 11.5A7.5 7.5 0 1 1 7.2 17M4.5 6.5v5h5" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>',
        save: '<path d="M5 4.5h11l3.5 3.5V19a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V5.5a1 1 0 0 1 1-1z" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/><path d="M8 4.5v4.5h6V4.5M8 19.5v-5.5h8v5.5" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/>',
        trash: '<path d="M5 7h14M9.5 7V5a1 1 0 0 1 1-1h3a1 1 0 0 1 1 1v2M8 7l.7 11.6a1.4 1.4 0 0 0 1.4 1.4h3.8a1.4 1.4 0 0 0 1.4-1.4L16 7" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>',
        close: '<path d="M6.5 6.5l11 11M17.5 6.5l-11 11" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>'
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
        const VIZ_MODES = (window.AudioPlayerVisualizer && Array.isArray(window.AudioPlayerVisualizer.MODES) && window.AudioPlayerVisualizer.MODES.length)
            ? window.AudioPlayerVisualizer.MODES
            : ['bars', 'radial', 'wave', 'nebula', 'horizon'];
        const initialMode = VIZ_MODES.indexOf(prefs.mode) >= 0 ? prefs.mode : 'bars';
        const eqApi = window.AudioPlayerEQ || null;
        const eqStore = eqApi ? eqApi.createStore() : null;
        const eqGradId = 'vd-eq-grad-' + String(windowId || '0').replace(/[^a-zA-Z0-9_-]/g, '');

        const state = {
            tracks: [],
            index: -1,
            playing: false,
            status: 'idle',
            shuffle: !!prefs.shuffle,
            repeat: REPEAT_MODES.indexOf(prefs.repeat) >= 0 ? prefs.repeat : 'off',
            volume: typeof prefs.volume === 'number' ? Math.min(1, Math.max(0, prefs.volume)) : 0.85,
            muted: false,
            speed: SPEEDS.indexOf(prefs.speed) >= 0 ? prefs.speed : 1,
            mode: initialMode,
            folder: '',
            graph: null,
            visualizer: null,
            eqStore,
            eqOpen: false,
            eqOptionSig: '',
            eqStatusTimer: 0,
            scrubbing: false,
            disposed: false,
            tagAbort: null,
            canVisualize: true
        };

        const MODE_LABELS = {
            bars: t('desktop.audio_player_mode_bars'),
            radial: t('desktop.audio_player_mode_radial'),
            wave: t('desktop.audio_player_mode_wave'),
            nebula: t('desktop.audio_player_mode_nebula'),
            horizon: t('desktop.audio_player_mode_horizon')
        };

        const eqBandMarkup = eqApi
            ? eqApi.BANDS.map((band, idx) => `<div class="vd-player-eqpanel-band" data-eq-col="${idx}">
              <button type="button" class="vd-player-eqpanel-db" data-eq-db="${idx}" title="${esc(t('desktop.audio_player_eq_reset'))}">0.0</button>
              <div class="vd-player-eqpanel-rail">
                <div class="vd-player-eqpanel-track">
                  <div class="vd-player-eqpanel-zero"></div>
                  <div class="vd-player-eqpanel-fill" data-eq-fill="${idx}"></div>
                </div>
                <input type="range" class="vd-player-eqpanel-slider" min="${eqApi.MIN_GAIN}" max="${eqApi.MAX_GAIN}" step="${eqApi.GAIN_STEP}" value="0" data-eq-band="${idx}" aria-label="${esc(t('desktop.audio_player_eq_band', { freq: band.freqLabel }))}">
              </div>
              <span class="vd-player-eqpanel-freq">${esc(band.label)}</span>
            </div>`).join('')
            : '';

        const vizPillsMarkup = VIZ_MODES.map(mode => `<button type="button" class="vd-player-viz-pill${mode === state.mode ? ' is-active' : ''}" data-viz-pill="${esc(mode)}" aria-pressed="${mode === state.mode ? 'true' : 'false'}">${esc(MODE_LABELS[mode] || mode)}</button>`).join('');

        host.innerHTML = `
<div class="vd-player-shell">
<div class="vd-player">
  <section class="vd-player-stage">
    <div class="vd-player-glow" aria-hidden="true"></div>
    <canvas class="vd-player-canvas" data-canvas aria-hidden="true"></canvas>
    <div class="vd-player-viz-dock" role="toolbar" aria-label="${esc(t('desktop.audio_player_visualizer'))}" data-viz-dock>${vizPillsMarkup}</div>
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
        <button type="button" class="vd-player-btn vd-player-btn--text vd-player-eq-btn" data-eq-toggle aria-pressed="false" aria-expanded="false" aria-label="${esc(t('desktop.audio_player_eq'))}" title="${esc(t('desktop.audio_player_eq'))}">${icon('eq')}<span>${esc(t('desktop.audio_player_eq_short'))}</span><span class="vd-player-eq-dot" aria-hidden="true"></span></button>
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
        <button type="button" class="vd-player-btn vd-player-btn--text" data-visualizer-mode aria-label="${esc(t('desktop.audio_player_visualizer'))}">Bars</button>
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
  <section class="vd-player-eqpanel" data-eq-panel hidden aria-label="${esc(t('desktop.audio_player_eq'))}">
    <header class="vd-player-eqpanel-head">
      <button type="button" class="vd-player-btn vd-player-btn--text vd-player-eqpanel-power" data-eq-enabled role="switch" aria-checked="true" title="${esc(t('desktop.audio_player_eq_enable'))}">
        ${icon('power')}<span>${esc(t('desktop.audio_player_eq'))}</span>
      </button>
      <select class="vd-player-eqpanel-select" data-eq-select aria-label="${esc(t('desktop.audio_player_eq_presets'))}"></select>
      <button type="button" class="vd-player-btn vd-player-btn--text vd-player-eqpanel-reset" data-eq-reset title="${esc(t('desktop.audio_player_eq_reset'))}">${icon('reset')}<span>${esc(t('desktop.audio_player_eq_reset'))}</span></button>
      <button type="button" class="vd-player-btn vd-player-eqpanel-close" data-eq-close aria-label="${esc(t('desktop.audio_player_eq_close'))}" title="${esc(t('desktop.audio_player_eq_close'))}">${icon('close')}</button>
    </header>
    <div class="vd-player-eqpanel-curve">
      <svg class="vd-player-eqpanel-svg" viewBox="0 0 480 68" preserveAspectRatio="none" aria-hidden="true">
        <defs>
          <linearGradient id="${esc(eqGradId)}" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stop-color="var(--pl-cover)" stop-opacity="0.44"/>
            <stop offset="55%" stop-color="var(--pl-cover)" stop-opacity="0.16"/>
            <stop offset="100%" stop-color="var(--pl-cover)" stop-opacity="0.03"/>
          </linearGradient>
        </defs>
        <line class="vd-player-eqpanel-gridline" x1="12" y1="14" x2="468" y2="14"/>
        <line class="vd-player-eqpanel-gridline vd-player-eqpanel-gridline--zero" x1="12" y1="34" x2="468" y2="34"/>
        <line class="vd-player-eqpanel-gridline" x1="12" y1="54" x2="468" y2="54"/>
        <path class="vd-player-eqpanel-fillpath" data-eq-curve-fill fill="url(#${esc(eqGradId)})" d=""/>
        <path class="vd-player-eqpanel-linepath" data-eq-curve-line fill="none" d=""/>
        <g data-eq-curve-dots></g>
      </svg>
      <div class="vd-player-eqpanel-scale" aria-hidden="true"><span>+12</span><span>0 dB</span><span>−12</span></div>
    </div>
    <div class="vd-player-eqpanel-bands" data-eq-bands>${eqBandMarkup}</div>
    <footer class="vd-player-eqpanel-foot">
      <input type="text" class="vd-player-eqpanel-input" maxlength="40" autocomplete="off" data-eq-profile-name placeholder="${esc(t('desktop.audio_player_eq_profile_placeholder'))}" aria-label="${esc(t('desktop.audio_player_eq_profile_placeholder'))}">
      <button type="button" class="vd-player-btn vd-player-btn--text vd-player-eqpanel-save" data-eq-save>${icon('save')}<span>${esc(t('desktop.audio_player_eq_save'))}</span></button>
      <button type="button" class="vd-player-btn vd-player-eqpanel-del" data-eq-delete aria-label="${esc(t('desktop.audio_player_eq_delete'))}" title="${esc(t('desktop.audio_player_eq_delete'))}">${icon('trash')}</button>
      <span class="vd-player-eqpanel-status" data-eq-status role="status" aria-live="polite"></span>
    </footer>
  </section>
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
            vizDock: host.querySelector('[data-viz-dock]'),
            vizPills: Array.from(host.querySelectorAll('[data-viz-pill]')),
            eqToggle: host.querySelector('[data-eq-toggle]'),
            eqPanel: host.querySelector('[data-eq-panel]'),
            eqEnabled: host.querySelector('[data-eq-enabled]'),
            eqSelect: host.querySelector('[data-eq-select]'),
            eqReset: host.querySelector('[data-eq-reset]'),
            eqClose: host.querySelector('[data-eq-close]'),
            eqCurveFill: host.querySelector('[data-eq-curve-fill]'),
            eqCurveLine: host.querySelector('[data-eq-curve-line]'),
            eqCurveDots: host.querySelector('[data-eq-curve-dots]'),
            eqSliders: Array.from(host.querySelectorAll('[data-eq-band]')),
            eqFills: Array.from(host.querySelectorAll('[data-eq-fill]')),
            eqDbLabels: Array.from(host.querySelectorAll('[data-eq-db]')),
            eqProfileName: host.querySelector('[data-eq-profile-name]'),
            eqSave: host.querySelector('[data-eq-save]'),
            eqDelete: host.querySelector('[data-eq-delete]'),
            eqStatus: host.querySelector('[data-eq-status]'),
            glow: host.querySelector('.vd-player-glow'),
            queue: host.querySelector('[data-queue]'),
            queueTitle: host.querySelector('[data-queue-title]'),
            queueFoot: host.querySelector('[data-queue-foot]'),
            reload: host.querySelector('[data-reload]')
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

        function formatDb(value) {
            const n = Number(value) || 0;
            if (n > 0.05) return '+' + n.toFixed(1);
            if (n < -0.05) return n.toFixed(1);
            return '0.0';
        }

        function buildCurvePath(points) {
            if (!points || !points.length) return '';
            let d = `M ${points[0][0]} ${points[0][1]}`;
            for (let i = 1; i < points.length - 1; i++) {
                const xc = ((points[i][0] + points[i + 1][0]) * 0.5).toFixed(2);
                const yc = ((points[i][1] + points[i + 1][1]) * 0.5).toFixed(2);
                d += ` Q ${points[i][0]} ${points[i][1]} ${xc} ${yc}`;
            }
            const last = points[points.length - 1];
            d += ` L ${last[0]} ${last[1]}`;
            return d;
        }

        function setEqStatus(message, isError) {
            if (!els.eqStatus) return;
            if (state.eqStatusTimer) {
                clearTimeout(state.eqStatusTimer);
                state.eqStatusTimer = 0;
            }
            els.eqStatus.textContent = message || '';
            els.eqStatus.classList.toggle('is-error', !!isError);
            els.eqStatus.classList.toggle('is-visible', !!message);
            if (message) {
                state.eqStatusTimer = setTimeout(() => {
                    if (state.disposed || !els.eqStatus) return;
                    els.eqStatus.textContent = '';
                    els.eqStatus.classList.remove('is-visible', 'is-error');
                }, 2600);
            }
        }

        function syncEQ(immediate) {
            if (!eqApi || !state.eqStore) return;
            const snap = state.eqStore.snapshot();
            const effective = state.eqStore.isEffective();
            if (state.graph && state.graph.eqChain) {
                eqApi.applyToChain(state.graph.eqChain, state.graph.context, snap, !!immediate);
            }
            if (els.eqToggle) {
                els.eqToggle.classList.toggle('is-active', effective);
                els.eqToggle.classList.toggle('is-open', state.eqOpen);
                els.eqToggle.setAttribute('aria-pressed', String(state.eqOpen));
                els.eqToggle.setAttribute('aria-expanded', String(state.eqOpen));
            }
            if (els.eqPanel) {
                els.eqPanel.hidden = !state.eqOpen;
                els.eqPanel.classList.toggle('is-bypassed', !snap.enabled);
            }
            if (els.eqEnabled) {
                els.eqEnabled.setAttribute('aria-checked', String(snap.enabled));
                els.eqEnabled.classList.toggle('is-active', snap.enabled);
            }
            for (let i = 0; i < els.eqSliders.length; i++) {
                const gain = snap.bands[i] || 0;
                const slider = els.eqSliders[i];
                if (slider && slider.value !== String(gain)) {
                    slider.value = String(gain);
                }
                if (els.eqDbLabels[i]) {
                    els.eqDbLabels[i].textContent = formatDb(gain);
                    els.eqDbLabels[i].classList.toggle('is-nonzero', Math.abs(gain) >= 0.05);
                }
                const fill = els.eqFills[i];
                if (fill) {
                    const shown = snap.enabled ? gain : 0;
                    const pct = (Math.abs(shown) / eqApi.MAX_GAIN) * 50;
                    if (shown >= 0) {
                        fill.style.bottom = '50%';
                        fill.style.height = pct.toFixed(2) + '%';
                    } else {
                        fill.style.bottom = (50 - pct).toFixed(2) + '%';
                        fill.style.height = pct.toFixed(2) + '%';
                    }
                    fill.classList.toggle('is-cut', shown < -0.05);
                }
            }
            if (els.eqCurveLine && els.eqCurveFill) {
                const curve = eqApi.computeCurve(snap.bands, snap.enabled, 480, 68, 88);
                const linePath = buildCurvePath(curve.points);
                els.eqCurveLine.setAttribute('d', linePath);
                const first = curve.points[0];
                const last = curve.points[curve.points.length - 1];
                els.eqCurveFill.setAttribute('d', `${linePath} L ${last[0]} ${curve.zeroY} L ${first[0]} ${curve.zeroY} Z`);
                if (els.eqCurveDots) {
                    els.eqCurveDots.innerHTML = curve.bandDots
                        .map(dot => `<circle class="vd-player-eqpanel-dot${Math.abs(dot.gain) >= 0.05 ? ' is-nonzero' : ''}" cx="${dot.x}" cy="${dot.y}" r="${Math.abs(dot.gain) >= 0.05 ? 3.2 : 2.2}"/>`)
                        .join('');
                }
            }
            if (els.eqSelect) {
                const sig = (snap.activeId === 'custom' ? '1:' : '0:') + snap.profiles.map(p => p.id + '=' + p.name).join('|');
                if (sig !== state.eqOptionSig) {
                    const parts = [];
                    if (snap.activeId === 'custom') {
                        parts.push(`<option value="custom">${esc(t('desktop.audio_player_eq_custom'))}</option>`);
                    }
                    parts.push(`<optgroup label="${esc(t('desktop.audio_player_eq_presets'))}">${eqApi.PRESETS.map(p => `<option value="${esc(p.id)}">${esc(t(p.labelKey))}</option>`).join('')}</optgroup>`);
                    if (snap.profiles.length) {
                        parts.push(`<optgroup label="${esc(t('desktop.audio_player_eq_profiles'))}">${snap.profiles.map(p => `<option value="${esc(p.id)}">${esc(p.name)}</option>`).join('')}</optgroup>`);
                    }
                    els.eqSelect.innerHTML = parts.join('');
                    state.eqOptionSig = sig;
                }
                els.eqSelect.value = snap.activeId;
            }
            const activeUserProfile = state.eqStore.activeProfile();
            if (els.eqDelete) {
                els.eqDelete.disabled = !activeUserProfile;
            }
            if (els.eqProfileName && document.activeElement !== els.eqProfileName) {
                if (activeUserProfile) els.eqProfileName.value = activeUserProfile.name;
                else if (snap.activeId && snap.activeId.startsWith('preset:')) els.eqProfileName.value = '';
            }
        }

        function setEqOpen(open) {
            state.eqOpen = !!open;
            syncEQ(false);
        }

        // Web Audio graph: element -> 8-band EQ chain -> analyser -> gain ->
        // destination. The analyser sits after EQ so the visualizer reflects
        // tone shaping, and before volume gain so volume never scales visuals.
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
                const eqChain = eqApi ? eqApi.buildChain(audioContext) : null;
                const analyser = audioContext.createAnalyser();
                analyser.fftSize = 2048;
                analyser.smoothingTimeConstant = 0.58;
                const gain = audioContext.createGain();
                if (eqChain) {
                    source.connect(eqChain.input);
                    eqChain.output.connect(analyser);
                } else {
                    source.connect(analyser);
                }
                analyser.connect(gain);
                gain.connect(audioContext.destination);
                state.graph = { context: audioContext, source, eqChain, analyser, gain };
                syncEQ(true);
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
            if (state.disposed) return;
            const track = currentTrack();
            const artwork = track && track.picture ? [{ src: track.picture, sizes: '512x512' }] : [];
            try {
                window.AuraDesktopMediaSession.claim(state, {
                    priority: 60,
                    activate: true,
                    playbackState: state.playing ? 'playing' : 'paused',
                    status: state.status,
                    metadata: {
                        title: track ? (track.title || track.name) : t('desktop.audio_player_no_track'),
                        artist: track ? (track.artist || t('desktop.audio_player_unknown_artist')) : 'AuraGo',
                        album: track ? (track.album || state.folder || '') : '',
                        artwork
                    },
                    handlers: {
                        play: () => play(),
                        pause: () => pause(),
                        stop: () => pause(),
                        previoustrack: () => previous(),
                        nexttrack: () => next(),
                        seekbackward: details => seekBy(-(details && details.seekOffset ? details.seekOffset : 10)),
                        seekforward: details => seekBy(details && details.seekOffset ? details.seekOffset : 10),
                        seekto: details => seekToRatio(details && details.seekTime && track && track.duration ? details.seekTime / track.duration : 0)
                    }
                });
            } catch (_) { /* media session unavailable */ }
        }

        function setPlaying(playing, status) {
            if (state.disposed) return;
            state.playing = !!playing;
            state.status = status || (state.playing ? 'playing' : 'paused');
            if (els.play) {
                els.play.innerHTML = icon(state.playing ? 'pause' : 'play');
                els.play.setAttribute('aria-label', t(state.playing ? 'desktop.audio_player_pause' : 'desktop.audio_player_play'));
            }
            root.classList.toggle('is-playing', state.playing);
            updateMediaSession();
        }

        async function play() {
            if (!audio.getAttribute('src')) return false;
            ensureGraph();
            if (state.graph && state.graph.context.state === 'suspended') {
                try { await state.graph.context.resume(); } catch (_) { /* needs a gesture */ }
            }
            try {
                await audio.play();
            } catch (error) {
                setPlaying(false, 'error');
                throw error;
            }
            setPlaying(true, audio.readyState < 3 ? 'buffering' : 'playing');
            return true;
        }

        function pause() {
            audio.pause();
            setPlaying(false, 'paused');
        }

        function toggle() {
            if (state.playing) { pause(); return Promise.resolve(true); }
            return play();
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
            setPlaying(false, 'buffering');
            const track = state.tracks[index];
            audio.src = track.url;
            audio.playbackRate = state.speed;
            audio.load();
            renderMeta();
            renderQueue();
            loadTags(track);
            if (autoplay) play().catch(() => {});
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
            // One parse for the whole playlist. Building the list entry by
            // entry costs one HTML parse per track, which stalls the desktop
            // for seconds when the library is large.
            const rows = [];
            state.tracks.forEach((track, index) => {
                const title = track.title || stripExtension(track.name);
                const meta = [track.artist, track.album].filter(Boolean).join(' · ');
                const current = index === state.index ? ' is-current' : '';
                rows.push(`<li class="vd-player-queue-item${current}" data-index="${index}"><button type="button" class="vd-player-queue-btn" data-select="${index}">
          <span class="vd-player-queue-mark">
            <span class="vd-player-queue-index">${index + 1}</span>
            <span class="vd-player-eq" aria-hidden="true"><span></span><span></span><span></span></span>
          </span>
          <span class="vd-player-queue-text">
            <span class="vd-player-track-name">${esc(title)}</span>
            ${meta ? `<span class="vd-player-queue-meta">${esc(meta)}</span>` : ''}
          </span>
          <span class="vd-player-queue-duration">${track.duration ? esc(formatTime(track.duration)) : ''}</span>
        </button></li>`);
            });
            els.queue.innerHTML = rows.join('');
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

        function syncVisualizerUI() {
            els.mode.textContent = MODE_LABELS[state.mode] || state.mode;
            if (els.vizPills) {
                els.vizPills.forEach(pill => {
                    const active = pill.dataset.vizPill === state.mode;
                    pill.classList.toggle('is-active', active);
                    pill.setAttribute('aria-pressed', String(active));
                });
            }
        }

        function cycleMode() {
            if (!state.visualizer) return;
            state.mode = state.visualizer.cycleMode();
            syncVisualizerUI();
            persist();
            setWindowMenus();
        }

        function selectTrack(index) {
            if (index === state.index) {
                toggle().catch(() => {});
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
                state.playing = false;
                state.status = 'idle';
                audio.pause();
                audio.removeAttribute('src');
                audio.load();
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
            els.play.addEventListener('click', () => { toggle().catch(() => {}); });
            els.prev.addEventListener('click', previous);
            els.next.addEventListener('click', next);
            els.shuffle.addEventListener('click', toggleShuffle);
            els.repeat.addEventListener('click', cycleRepeat);
            els.speed.addEventListener('click', cycleSpeed);
            els.mode.addEventListener('click', cycleMode);
            if (els.vizDock) {
                els.vizDock.addEventListener('click', event => {
                    const pill = event.target.closest('[data-viz-pill]');
                    if (!pill) return;
                    setMode(pill.dataset.vizPill);
                });
            }
            if (els.eqToggle) {
                els.eqToggle.addEventListener('click', () => setEqOpen(!state.eqOpen));
            }
            if (els.eqClose) {
                els.eqClose.addEventListener('click', () => {
                    setEqOpen(false);
                    if (els.eqToggle) els.eqToggle.focus({ preventScroll: true });
                });
            }
            if (els.eqEnabled && state.eqStore) {
                els.eqEnabled.addEventListener('click', () => {
                    const snap = state.eqStore.snapshot();
                    state.eqStore.setEnabled(!snap.enabled);
                    syncEQ(false);
                });
            }
            if (els.eqSelect && state.eqStore) {
                els.eqSelect.addEventListener('change', () => {
                    state.eqStore.selectProfile(els.eqSelect.value);
                    syncEQ(false);
                });
            }
            if (els.eqReset && state.eqStore) {
                els.eqReset.addEventListener('click', () => {
                    state.eqStore.reset();
                    syncEQ(false);
                });
            }
            els.eqSliders.forEach(slider => {
                slider.addEventListener('input', () => {
                    if (!state.eqStore) return;
                    state.eqStore.setBand(Number(slider.dataset.eqBand), Number(slider.value));
                    syncEQ(true);
                });
                slider.addEventListener('dblclick', () => {
                    if (!state.eqStore) return;
                    state.eqStore.setBand(Number(slider.dataset.eqBand), 0);
                    syncEQ(false);
                });
            });
            els.eqDbLabels.forEach(btn => {
                btn.addEventListener('click', () => {
                    if (!state.eqStore) return;
                    state.eqStore.setBand(Number(btn.dataset.eqDb), 0);
                    syncEQ(false);
                });
            });
            const saveCurrentProfile = () => {
                if (!state.eqStore || !els.eqProfileName) return;
                const result = state.eqStore.saveProfile(els.eqProfileName.value);
                if (!result.ok) {
                    setEqStatus(
                        t(result.reason === 'limit_reached' ? 'desktop.audio_player_eq_limit_reached' : 'desktop.audio_player_eq_name_required'),
                        true
                    );
                    els.eqProfileName.focus({ preventScroll: true });
                    return;
                }
                syncEQ(false);
                setEqStatus(t('desktop.audio_player_eq_saved'), false);
            };
            if (els.eqSave) els.eqSave.addEventListener('click', saveCurrentProfile);
            if (els.eqProfileName) {
                els.eqProfileName.addEventListener('keydown', event => {
                    if (event.key === 'Enter') {
                        event.preventDefault();
                        saveCurrentProfile();
                    } else if (event.key === 'Escape') {
                        event.stopPropagation();
                        setEqOpen(false);
                        if (els.eqToggle) els.eqToggle.focus({ preventScroll: true });
                    }
                });
            }
            if (els.eqDelete && state.eqStore) {
                els.eqDelete.addEventListener('click', () => {
                    const active = state.eqStore.activeProfile();
                    if (!active) return;
                    const res = state.eqStore.deleteProfile(active.id);
                    if (res.ok) {
                        if (els.eqProfileName) els.eqProfileName.value = '';
                        syncEQ(false);
                        setEqStatus(t('desktop.audio_player_eq_deleted'), false);
                    }
                });
            }
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

            audio.addEventListener('loadstart', () => {
                if (!state.playing) setPlaying(false, 'buffering');
            });
            audio.addEventListener('play', () => setPlaying(true, audio.readyState < 3 ? 'buffering' : 'playing'));
            audio.addEventListener('playing', () => setPlaying(true, 'playing'));
            audio.addEventListener('waiting', () => {
                if (!audio.paused) setPlaying(true, 'buffering');
            });
            audio.addEventListener('stalled', () => {
                if (!audio.paused) setPlaying(true, 'buffering');
            });
            audio.addEventListener('canplay', () => {
                if (audio.paused && state.status === 'buffering') setPlaying(false, 'paused');
            });
            audio.addEventListener('pause', () => {
                if (state.disposed) return;
                setPlaying(false, !currentTrack() ? 'idle' : state.status === 'error' || state.status === 'ended' ? state.status : 'paused');
            });
            audio.addEventListener('timeupdate', updateTimeline);
            audio.addEventListener('progress', updateTimeline);
            audio.addEventListener('loadedmetadata', updateTimeline);
            audio.addEventListener('durationchange', updateTimeline);
            audio.addEventListener('ended', () => {
                if (state.repeat === 'one') {
                    audio.currentTime = 0;
                    play().catch(() => {});
                    return;
                }
                const target = nextIndex();
                if (target < 0) {
                    setPlaying(false, 'ended');
                    return;
                }
                loadTrack(target, true);
            });
            audio.addEventListener('error', () => {
                if (state.disposed) return;
                setPlaying(false, 'error');
            });

            root.addEventListener('keydown', event => {
                if (event.target && /^(INPUT|TEXTAREA|SELECT)$/.test(event.target.tagName)) return;
                if (event.target && typeof event.target.closest === 'function' && event.target.closest('[data-seek]')) return;
                const key = event.key;
                if (key === ' ' || key === 'k') { toggle().catch(() => {}); event.preventDefault(); }
                else if (key === 'ArrowRight') { seekBy(5); event.preventDefault(); }
                else if (key === 'ArrowLeft') { seekBy(-5); event.preventDefault(); }
                else if (key === 'ArrowUp') { state.volume = Math.min(1, state.volume + 0.05); applyVolume(); persist(); event.preventDefault(); }
                else if (key === 'ArrowDown') { state.volume = Math.max(0, state.volume - 0.05); applyVolume(); persist(); event.preventDefault(); }
                else if (key === 'm') { state.muted = !state.muted; applyVolume(); }
                else if (key === 's') { toggleShuffle(); }
                else if (key === 'r') { cycleRepeat(); }
                else if (key === 'v') { cycleMode(); }
                else if (key === 'e') { setEqOpen(!state.eqOpen); event.preventDefault(); }
                else if (key === 'Escape' && state.eqOpen) { setEqOpen(false); if (els.eqToggle) els.eqToggle.focus({ preventScroll: true }); event.preventDefault(); }
                else if (key >= '1' && key <= String(VIZ_MODES.length)) { setMode(VIZ_MODES[Number(key) - 1]); }
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
                        { id: 'play-pause', labelKey: state.playing ? 'desktop.audio_player_pause' : 'desktop.audio_player_play', icon: state.playing ? 'pause' : 'play', action: () => toggle().catch(() => {}) },
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
                        ...VIZ_MODES.map(mode => ({
                            id: 'mode-' + mode,
                            labelKey: 'desktop.audio_player_mode_' + mode,
                            checked: state.mode === mode,
                            action: () => setMode(mode)
                        })),
                        { id: 'toggle-eq', labelKey: 'desktop.audio_player_eq', checked: state.eqOpen, action: () => setEqOpen(!state.eqOpen) }
                    ]
                }
            ]);
        }

        function setMode(mode) {
            if (!state.visualizer) return;
            state.visualizer.setMode(mode);
            state.mode = state.visualizer.mode();
            syncVisualizerUI();
            persist();
            setWindowMenus();
        }

        function dispose() {
            if (state.disposed) return;
            state.disposed = true;
            if (state.eqStatusTimer) {
                clearTimeout(state.eqStatusTimer);
                state.eqStatusTimer = 0;
            }
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
        syncVisualizerUI();
        syncEQ(true);
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
                onEnergy: (bass, energy) => {
                    const pulse = Math.max(0, Math.min(1, (Number(bass) || 0) * 0.78 + (Number(energy) || 0) * 0.30));
                    // The pulse belongs to the glow, not to the player root. An
                    // inherited custom property is written on every frame, and a
                    // write on the root invalidates the style of the whole
                    // player subtree - with a full playlist that is thousands of
                    // queue nodes - so the entire desktop stalls.
                    const pulseTarget = els.glow || root;
                    pulseTarget.style.setProperty('--pl-pulse', pulse.toFixed(3));
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
