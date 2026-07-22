(function () {
    'use strict';

    // NoisemakerLibrary — track grid + bottom player bar for the Noisemaker app.
    // Factory pattern (like CheaterToolbar): create(deps) returns a controller
    // owning one <audio> element and all its listeners; dispose() cleans up.

    const SVG = {
        play: '<svg viewBox="0 0 16 16" width="15" height="15" aria-hidden="true"><path fill="currentColor" d="M4 2.5v11l9-5.5z"/></svg>',
        pause: '<svg viewBox="0 0 16 16" width="15" height="15" aria-hidden="true"><path fill="currentColor" d="M3 2.5h3.4v11H3zM9.6 2.5H13v11H9.6z"/></svg>',
        prev: '<svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true"><path fill="currentColor" d="M3 2.5h2.2v11H3zM13 2.5v11L5.5 8z"/></svg>',
        next: '<svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true"><path fill="currentColor" d="M10.8 2.5H13v11h-2.2zM3 2.5v11l7.5-5.5z"/></svg>',
        volume: '<svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true"><path fill="currentColor" d="M2 5.5v5h2.8L9 13.6V2.4L4.8 5.5H2zm9.4-.3v5.6a2.9 2.9 0 0 0 0-5.6z"/></svg>',
        download: '<svg viewBox="0 0 16 16" width="13" height="13" aria-hidden="true"><path fill="currentColor" d="M8 1.5v8m0 0 3-3m-3 3L5 6.5M2.5 12.5v1.5h11v-1.5" stroke="currentColor" stroke-width="1.6" fill="none" stroke-linecap="round" stroke-linejoin="round"/></svg>',
        trash: '<svg viewBox="0 0 16 16" width="13" height="13" aria-hidden="true"><path fill="currentColor" d="M6 2.5h4l.6 1.2h2.9v1.3h-11V3.7h2.9L6 2.5zm-2.4 3.4h8.8l-.6 7.4a1 1 0 0 1-1 .9H5.2a1 1 0 0 1-1-.9l-.6-7.4z"/></svg>',
        template: '<svg viewBox="0 0 16 16" width="13" height="13" aria-hidden="true"><path fill="currentColor" d="M3 2.5h7v2H5v9H3v-11zm4 4h6v7H7v-7zm2 2v3h2v-3H9z"/></svg>'
    };

    function formatDuration(ms) {
        const total = Math.max(0, Math.round((Number(ms) || 0) / 1000));
        const m = Math.floor(total / 60);
        const s = total % 60;
        return m + ':' + String(s).padStart(2, '0');
    }

    function formatDate(value, lang) {
        const ts = Date.parse(String(value || '').replace(' ', 'T'));
        if (!Number.isFinite(ts)) return '';
        try {
            return new Intl.DateTimeFormat(lang || undefined, { day: '2-digit', month: '2-digit', year: 'numeric' }).format(new Date(ts));
        } catch (_) {
            return new Date(ts).toLocaleDateString();
        }
    }

    function coverMarkup(esc, track, cls) {
        // SVG spacer: only a replaced element with an intrinsic ratio contributes
        // its width-derived height to grid track sizing (aspect-ratio does not).
        const spacer = String(cls || '').indexOf('nm-card-cover') >= 0
            ? '<svg class="nm-card-ratio" viewBox="0 0 1 1" aria-hidden="true"></svg>'
            : '';
        if (track && track.cover_url) {
            return '<div class="nm-cover ' + cls + '">' + spacer + '<img src="' + esc(track.cover_url) + '" alt="" loading="lazy" draggable="false"></div>';
        }
        return '<div class="nm-cover nm-cover--empty ' + cls + '" aria-hidden="true">' + spacer + '<span>♪</span></div>';
    }

    function create(deps) {
        const esc = deps.esc || (v => String(v == null ? '' : v));
        const t = deps.t || ((k, p, f) => f || k);
        const lang = deps.lang || '';
        const handlers = { delete: [], template: [], create: [], playstate: [] };

        let tracks = [];
        let query = '';
        let loading = false;
        let currentId = null;
        let isPlaying = false;
        let disposed = false;

        const root = document.createElement('div');
        root.className = 'nm-library';
        root.innerHTML =
            '<div class="nm-library-toolbar">' +
                '<div class="nm-search"><span class="nm-search-glyph">⌕</span>' +
                    '<input class="nm-input nm-search-input" type="search" aria-label="' + esc(t('desktop.noisemaker_library_search', {}, 'Search songs…')) + '" placeholder="' + esc(t('desktop.noisemaker_library_search', {}, 'Search songs…')) + '">' +
                '</div>' +
            '</div>' +
            '<div class="nm-grid" role="list"></div>' +
            '<div class="nm-player">' +
                '<div class="nm-player-cover-slot"></div>' +
                '<div class="nm-player-info"><div class="nm-player-title"></div><div class="nm-player-meta"></div></div>' +
                '<div class="nm-player-controls">' +
                    '<button type="button" class="nm-player-btn nm-player-prev" aria-label="' + esc(t('desktop.noisemaker_player_previous', {}, 'Previous')) + '">' + SVG.prev + '</button>' +
                    '<button type="button" class="nm-player-btn nm-player-btn--play nm-player-toggle" aria-label="' + esc(t('desktop.noisemaker_player_play', {}, 'Play')) + '">' + SVG.play + '</button>' +
                    '<button type="button" class="nm-player-btn nm-player-next" aria-label="' + esc(t('desktop.noisemaker_player_next', {}, 'Next')) + '">' + SVG.next + '</button>' +
                '</div>' +
                '<div class="nm-player-track">' +
                    '<span class="nm-time nm-time-current">0:00</span>' +
                    '<input type="range" class="nm-seek nm-seek-pos" min="0" max="1000" value="0" step="1" aria-label="' + esc(t('desktop.noisemaker_player_seek', {}, 'Seek')) + '">' +
                    '<span class="nm-time nm-time-total">0:00</span>' +
                '</div>' +
                '<div class="nm-volume"><span aria-hidden="true">' + SVG.volume + '</span>' +
                    '<input type="range" class="nm-seek nm-seek-vol" min="0" max="100" value="90" step="1" aria-label="' + esc(t('desktop.noisemaker_player_volume', {}, 'Volume')) + '">' +
                '</div>' +
            '</div>';

        const gridEl = root.querySelector('.nm-grid');
        const searchEl = root.querySelector('.nm-search-input');
        const playerEl = root.querySelector('.nm-player');
        const coverSlot = root.querySelector('.nm-player-cover-slot');
        const titleEl = root.querySelector('.nm-player-title');
        const metaEl = root.querySelector('.nm-player-meta');
        const toggleBtn = root.querySelector('.nm-player-toggle');
        const curEl = root.querySelector('.nm-time-current');
        const totEl = root.querySelector('.nm-time-total');
        const seekEl = root.querySelector('.nm-seek-pos');
        const volEl = root.querySelector('.nm-seek-vol');

        const audio = new Audio();
        audio.preload = 'metadata';
        audio.volume = 0.9;

        function emit(name, payload) {
            handlers[name].forEach(cb => { try { cb(payload); } catch (_) {} });
        }

        function filteredTracks() {
            if (!query) return tracks.slice();
            const q = query.toLowerCase();
            return tracks.filter(track =>
                String(track.title || '').toLowerCase().includes(q) ||
                String(track.prompt || '').toLowerCase().includes(q) ||
                (Array.isArray(track.tags) && track.tags.join(' ').toLowerCase().includes(q)));
        }

        function trackMeta(track) {
            const parts = [];
            if (track.duration_ms) parts.push(formatDuration(track.duration_ms));
            const date = formatDate(track.created_at, lang);
            if (date) parts.push(date);
            if (track.provider) parts.push(String(track.provider));
            return parts.join(' · ');
        }

        function renderGrid() {
            if (disposed) return;
            if (loading) {
                gridEl.innerHTML = '';
                gridEl.classList.add('is-loading');
                gridEl.innerHTML = '<div class="nm-loading">' + esc(t('desktop.noisemaker_library_loading', {}, 'Loading songs…')) + '</div>';
                return;
            }
            gridEl.classList.remove('is-loading');
            const list = filteredTracks();
            if (list.length === 0) {
                if (query) {
                    gridEl.innerHTML = '<div class="nm-empty"><div class="nm-empty-icon">⌕</div><h3>' +
                        esc(t('desktop.noisemaker_no_results', {}, 'No songs found.')) + '</h3></div>';
                } else {
                    gridEl.innerHTML = '<div class="nm-empty"><div class="nm-empty-icon">♪</div>' +
                        '<h3>' + esc(t('desktop.noisemaker_library_empty_title', {}, 'No songs yet')) + '</h3>' +
                        '<p>' + esc(t('desktop.noisemaker_library_empty_hint', {}, 'Create your first AI song — it will appear here.')) + '</p>' +
                        '<button type="button" class="nm-btn nm-btn--primary nm-empty-cta">' + esc(t('desktop.noisemaker_library_empty_cta', {}, 'Create now')) + '</button></div>';
                    const cta = gridEl.querySelector('.nm-empty-cta');
                    if (cta) cta.addEventListener('click', () => emit('create'));
                }
                return;
            }
            gridEl.innerHTML = list.map(track => {
                const playing = track.id === currentId;
                const title = track.title || track.prompt || t('desktop.noisemaker_result_untitled', {}, 'Untitled');
                const tags = [];
                if (track.instrumental) tags.push('<span class="nm-card-tag">' + esc(t('desktop.noisemaker_instrumental_tag', {}, 'Instrumental')) + '</span>');
                return '<div class="nm-card' + (playing ? ' is-playing' : '') + '" role="listitem" data-track-id="' + esc(track.id) + '" tabindex="0">' +
                    coverMarkup(esc, track, 'nm-card-cover') +
                    '<button type="button" class="nm-card-play" aria-label="' + esc(t('desktop.noisemaker_player_play', {}, 'Play')) + '">' + (playing && isPlaying ? SVG.pause : SVG.play) + '</button>' +
                    '<div class="nm-card-body">' +
                        '<div class="nm-card-title" title="' + esc(title) + '">' + esc(title) + '</div>' +
                        '<div class="nm-card-meta">' + esc(trackMeta(track)) + '</div>' +
                        (tags.length ? '<div>' + tags.join('') + '</div>' : '') +
                        '<div class="nm-card-actions">' +
                            '<button type="button" class="nm-icon-btn nm-act-template" title="' + esc(t('desktop.noisemaker_track_use_template', {}, 'Use as template')) + '" aria-label="' + esc(t('desktop.noisemaker_track_use_template', {}, 'Use as template')) + '">' + SVG.template + '</button>' +
                            '<button type="button" class="nm-icon-btn nm-act-download" title="' + esc(t('desktop.noisemaker_track_download', {}, 'Download')) + '" aria-label="' + esc(t('desktop.noisemaker_track_download', {}, 'Download')) + '">' + SVG.download + '</button>' +
                            '<button type="button" class="nm-icon-btn nm-icon-btn--danger nm-act-delete" title="' + esc(t('desktop.noisemaker_track_delete', {}, 'Delete')) + '" aria-label="' + esc(t('desktop.noisemaker_track_delete', {}, 'Delete')) + '">' + SVG.trash + '</button>' +
                        '</div>' +
                    '</div>' +
                '</div>';
            }).join('');
        }

        function trackById(id) {
            return tracks.find(track => String(track.id) === String(id)) || null;
        }

        function updatePlayerBar() {
            const track = currentId != null ? trackById(currentId) : null;
            if (!track) {
                playerEl.classList.remove('is-visible');
                return;
            }
            playerEl.classList.add('is-visible');
            coverSlot.innerHTML = coverMarkup(esc, track, 'nm-player-cover');
            titleEl.textContent = track.title || '';
            titleEl.title = track.title || '';
            metaEl.textContent = trackMeta(track);
            toggleBtn.innerHTML = isPlaying ? SVG.pause : SVG.play;
            toggleBtn.setAttribute('aria-label', isPlaying ? t('desktop.noisemaker_player_pause', {}, 'Pause') : t('desktop.noisemaker_player_play', {}, 'Play'));
        }

        function markPlayingCard() {
            gridEl.querySelectorAll('.nm-card').forEach(card => {
                const active = String(card.dataset.trackId) === String(currentId);
                card.classList.toggle('is-playing', active);
                const btn = card.querySelector('.nm-card-play');
                if (btn) btn.innerHTML = active && isPlaying ? SVG.pause : SVG.play;
            });
        }

        function play(track) {
            if (!track || !track.web_path) return;
            if (currentId === track.id) {
                toggle();
                return;
            }
            currentId = track.id;
            audio.src = track.web_path;
            audio.play().catch(() => {});
            isPlaying = true;
            updatePlayerBar();
            markPlayingCard();
            emit('playstate', { track, playing: true });
        }

        function toggle() {
            if (currentId == null) {
                const list = filteredTracks();
                if (list.length) play(list[0]);
                return;
            }
            if (audio.paused) {
                audio.play().catch(() => {});
            } else {
                audio.pause();
            }
        }

        function step(delta) {
            const list = filteredTracks();
            if (!list.length) return;
            const idx = list.findIndex(track => track.id === currentId);
            const nextIdx = idx < 0 ? 0 : (idx + delta + list.length) % list.length;
            play(list[nextIdx]);
        }

        function stop() {
            audio.pause();
            audio.removeAttribute('src');
            audio.load();
            currentId = null;
            isPlaying = false;
            updatePlayerBar();
            markPlayingCard();
        }

        audio.addEventListener('play', () => { isPlaying = true; updatePlayerBar(); markPlayingCard(); });
        audio.addEventListener('pause', () => { isPlaying = false; updatePlayerBar(); markPlayingCard(); });
        audio.addEventListener('ended', () => { step(1); });
        audio.addEventListener('timeupdate', () => {
            if (!audio.duration || !Number.isFinite(audio.duration)) return;
            curEl.textContent = formatDuration(audio.currentTime * 1000);
            totEl.textContent = formatDuration(audio.duration * 1000);
            if (!seekEl.matches(':active')) {
                seekEl.value = String(Math.round((audio.currentTime / audio.duration) * 1000));
            }
        });
        audio.addEventListener('loadedmetadata', () => {
            if (audio.duration && Number.isFinite(audio.duration)) {
                totEl.textContent = formatDuration(audio.duration * 1000);
            }
        });
        audio.addEventListener('error', () => {
            if (!audio.src) return;
            isPlaying = false;
            updatePlayerBar();
            markPlayingCard();
        });

        seekEl.addEventListener('input', () => {
            if (!audio.duration || !Number.isFinite(audio.duration)) return;
            const ratio = Number(seekEl.value) / 1000;
            curEl.textContent = formatDuration(ratio * audio.duration * 1000);
        });
        seekEl.addEventListener('change', () => {
            if (!audio.duration || !Number.isFinite(audio.duration)) return;
            audio.currentTime = (Number(seekEl.value) / 1000) * audio.duration;
        });
        volEl.addEventListener('input', () => {
            audio.volume = Math.max(0, Math.min(1, Number(volEl.value) / 100));
        });

        toggleBtn.addEventListener('click', toggle);
        root.querySelector('.nm-player-prev').addEventListener('click', () => step(-1));
        root.querySelector('.nm-player-next').addEventListener('click', () => step(1));

        searchEl.addEventListener('input', () => {
            query = searchEl.value.trim();
            renderGrid();
        });

        gridEl.addEventListener('click', event => {
            const card = event.target.closest('.nm-card');
            if (!card) return;
            const track = trackById(card.dataset.trackId);
            if (!track) return;
            if (event.target.closest('.nm-act-delete')) { emit('delete', track); return; }
            if (event.target.closest('.nm-act-download')) {
                const a = document.createElement('a');
                a.href = track.web_path;
                a.download = track.filename || (String(track.title || 'track') + '.' + (track.format || 'mp3'));
                document.body.appendChild(a);
                a.click();
                a.remove();
                return;
            }
            if (event.target.closest('.nm-act-template')) { emit('template', track); return; }
            play(track);
        });
        gridEl.addEventListener('keydown', event => {
            if (event.key !== 'Enter' && event.key !== ' ') return;
            const card = event.target.closest('.nm-card');
            if (!card) return;
            event.preventDefault();
            const track = trackById(card.dataset.trackId);
            if (track) play(track);
        });

        renderGrid();

        return {
            element: root,
            setTracks(list) {
                tracks = Array.isArray(list) ? list : [];
                if (currentId != null && !trackById(currentId)) {
                    stop();
                }
                renderGrid();
            },
            setLoading(value) {
                loading = !!value;
                renderGrid();
            },
            setQuery(value) {
                query = String(value || '').trim();
                searchEl.value = query;
                renderGrid();
            },
            play,
            stop,
            currentTrack() { return currentId != null ? trackById(currentId) : null; },
            on(name, cb) {
                if (handlers[name] && typeof cb === 'function') handlers[name].push(cb);
            },
            dispose() {
                if (disposed) return;
                disposed = true;
                stop();
                root.remove();
            }
        };
    }

    window.NoisemakerLibrary = { create, formatDuration };
})();
