(function () {
    'use strict';

    // Noisemaker — Suno-style AI music studio for the virtual desktop.
    // Talks to /api/desktop/noisemaker/* (state, enhance, generate, tracks, delete).
    // Exposes window.NoisemakerApp = { render, dispose }; every window instance
    // owns its controllers, timers, and the NoisemakerLibrary player.

    const instances = new Map();
    const preferenceKey = 'aurago.desktop.noisemaker.prefs';
    const NS = 'desktop.noisemaker';
    const TRACKS_PAGE_SIZE = 60;

    const STYLE_SUGGESTIONS = ['Pop', 'Lo-Fi', 'Synthwave', 'Techno', 'Hip-Hop', 'Rock', 'Jazz', 'Ambient', 'Epic Orchestra', 'Acoustic', 'EDM', 'Metal'];
    const IDEA_MAX = 2000;
    const STYLE_MAX = 500;
    const LYRICS_MAX = 8000;

    function text(ctx, key, params, fallback) {
        const fullKey = NS + '_' + key;
        const value = ctx.t(fullKey, params || {});
        return value && value !== fullKey ? value : (fallback || fullKey);
    }

    function uiLang() {
        return (window.SYSTEM_LANG || document.documentElement.lang || 'en').toString();
    }

    function readPrefs() {
        try {
            const value = JSON.parse(localStorage.getItem(preferenceKey) || '{}');
            return {
                style: String(value.style || ''),
                instrumental: value.instrumental === true,
                cover: value.cover !== false
            };
        } catch (_) {
            return { style: '', instrumental: false, cover: true };
        }
    }

    function savePrefs(state) {
        try {
            localStorage.setItem(preferenceKey, JSON.stringify({
                style: state.form.style,
                instrumental: state.form.instrumental,
                cover: state.form.cover
            }));
        } catch (_) {}
    }

    function createState(host, windowId, ctx) {
        const prefs = readPrefs();
        return {
            host, windowId, ctx,
            disposed: false,
            controllers: new Set(),
            caps: null,
            view: 'create',
            form: { idea: '', style: prefs.style, lyrics: '', title: '', instrumental: prefs.instrumental, cover: prefs.cover },
            generation: { active: false, startedAt: 0, timerId: null, result: null, error: '', lastParams: null, coverFailed: false },
            tracks: [],
            tracksTotal: 0,
            tracksQuery: '',
            tracksLoading: false,
            tracksLoadedOnce: false,
            library: null,
            root: null
        };
    }

    async function request(state, path, options) {
        const controller = new AbortController();
        state.controllers.add(controller);
        const requestOptions = Object.assign({}, options || {}, { signal: controller.signal });
        if (requestOptions.body && typeof requestOptions.body !== 'string') {
            requestOptions.headers = Object.assign({ 'Content-Type': 'application/json' }, requestOptions.headers || {});
            requestOptions.body = JSON.stringify(requestOptions.body);
        }
        try {
            return await state.ctx.api(path, requestOptions);
        } finally {
            state.controllers.delete(controller);
        }
    }

    // ---------- shell markup ----------

    function shellMarkup(state) {
        const ctx = state.ctx;
        const esc = ctx.esc;
        return '<div class="noisemaker-app">' +
            '<div class="nm-header">' +
                '<div class="nm-brand">' +
                    '<span class="nm-brand-icon" aria-hidden="true">♪</span>' +
                    '<div><strong>' + esc(text(ctx, 'title', {}, 'Noisemaker')) + '</strong>' +
                    '<span>' + esc(text(ctx, 'subtitle', {}, 'AI music studio')) + '</span></div>' +
                '</div>' +
                '<div class="nm-tabs" role="tablist">' +
                    '<button type="button" class="nm-tab" role="tab" data-view="create" aria-selected="true">' + esc(text(ctx, 'tab_create', {}, 'Create')) + '</button>' +
                    '<button type="button" class="nm-tab" role="tab" data-view="library" aria-selected="false">' + esc(text(ctx, 'tab_library', {}, 'Library')) +
                        ' <span class="nm-tab-count" data-nm-track-count hidden>0</span></button>' +
                '</div>' +
                '<div class="nm-header-chips">' +
                    '<span class="nm-chip nm-chip--busy" data-nm-busy hidden>' + esc(text(ctx, 'generating_badge', {}, 'Generating…')) + '</span>' +
                    '<span class="nm-chip" data-nm-provider hidden></span>' +
                    '<span class="nm-chip nm-chip--muted" data-nm-quota hidden></span>' +
                '</div>' +
            '</div>' +
            '<div class="nm-body"></div>' +
        '</div>';
    }

    function bodyViewsMarkup() {
        return '<div class="nm-view nm-view-create is-active" data-view="create"><div class="nm-scroll"><div class="nm-create" data-nm-create></div></div></div>' +
            '<div class="nm-view nm-view-library" data-view="library"></div>';
    }

    function createFormMarkup(state) {
        const ctx = state.ctx;
        const esc = ctx.esc;
        const caps = state.caps || {};
        const llm = caps.llm_available !== false;
        const aiBtn = (action, labelKey, labelFallback) => !llm ? '' :
            '<button type="button" class="nm-ai" data-nm-enhance="' + action + '">' +
            '<span class="nm-ai-glyph" aria-hidden="true">✨</span>' + esc(text(ctx, labelKey, {}, labelFallback)) + '</button>';

        return '' +
        '<div class="nm-field">' +
            '<div class="nm-field-head"><label for="nm-idea-' + state.windowId + '">' + esc(text(ctx, 'idea_label', {}, 'Song idea')) + '</label>' +
                aiBtn('idea', 'idea_enhance', 'Enhance with AI') +
                (llm ? '<button type="button" class="nm-ai" data-nm-enhance="random"><span class="nm-ai-glyph" aria-hidden="true">🎲</span>' + esc(text(ctx, 'idea_random', {}, 'Surprise me')) + '</button>' : '') +
            '</div>' +
            '<textarea id="nm-idea-' + state.windowId + '" class="nm-textarea nm-textarea--idea" data-nm-field="idea" maxlength="' + IDEA_MAX + '" placeholder="' + esc(text(ctx, 'idea_placeholder', {}, 'e.g. An epic synthwave track about a night drive through neon lights…')) + '"></textarea>' +
            '<div class="nm-field-foot"><span class="nm-counter" data-nm-counter="idea"></span></div>' +
        '</div>' +
        '<div class="nm-field">' +
            '<div class="nm-field-head"><label for="nm-style-' + state.windowId + '">' + esc(text(ctx, 'style_label', {}, 'Style / Genre')) + '</label>' +
                aiBtn('style', 'style_enhance', 'Improve style') +
            '</div>' +
            '<input id="nm-style-' + state.windowId + '" class="nm-input" data-nm-field="style" maxlength="' + STYLE_MAX + '" placeholder="' + esc(text(ctx, 'style_placeholder', {}, 'e.g. synthwave, 80s, driving, female vocals')) + '">' +
            '<div class="nm-chips" data-nm-chips>' + STYLE_SUGGESTIONS.map(tag =>
                '<button type="button" class="nm-suggestion" data-nm-chip="' + esc(tag) + '">' + esc(tag) + '</button>').join('') + '</div>' +
        '</div>' +
        '<div class="nm-row nm-row--wrap">' +
            '<label class="nm-switch"><input type="checkbox" data-nm-field="instrumental">' +
                '<span class="nm-switch-track" aria-hidden="true"></span>' + esc(text(ctx, 'instrumental', {}, 'Instrumental (no vocals)')) + '</label>' +
            (caps.covers_enabled ?
                '<label class="nm-check"><input type="checkbox" data-nm-field="cover">' + esc(text(ctx, 'cover_label', {}, 'Generate AI cover')) + '</label>' +
                '<span class="nm-hint">' + esc(text(ctx, 'cover_hint', { provider: caps.cover_provider || '' }, 'uses the configured image AI')) + '</span>' : '') +
        '</div>' +
        '<details class="nm-collapsible" data-nm-lyrics-wrap>' +
            '<summary>' + esc(text(ctx, 'lyrics_label', {}, 'Lyrics')) + ' <span class="nm-hint">(' + esc(text(ctx, 'optional', {}, 'optional')) + ')</span></summary>' +
            '<div class="nm-collapsible-body">' +
                (caps.supports_lyrics === false ?
                    '<p class="nm-hint">' + esc(text(ctx, 'lyrics_unsupported', {}, 'The current provider does not support custom lyrics.')) + '</p>' :
                    '<div class="nm-field-head">' + (llm ? '<button type="button" class="nm-ai" data-nm-enhance="lyrics"><span class="nm-ai-glyph" aria-hidden="true">✨</span>' + esc(text(ctx, 'lyrics_generate', {}, 'Write lyrics with AI')) + '</button>' : '') + '</div>' +
                    '<textarea class="nm-textarea nm-textarea--lyrics" data-nm-field="lyrics" maxlength="' + LYRICS_MAX + '" placeholder="' + esc(text(ctx, 'lyrics_placeholder', {}, 'Your own lyrics (optional)…')) + '"></textarea>' +
                    '<div class="nm-field-foot"><span class="nm-counter" data-nm-counter="lyrics"></span></div>') +
            '</div>' +
        '</details>' +
        '<div class="nm-field">' +
            '<div class="nm-field-head"><label for="nm-title-' + state.windowId + '">' + esc(text(ctx, 'title_label', {}, 'Title')) + '</label>' +
                aiBtn('title', 'title_suggest', 'Suggest title') +
            '</div>' +
            '<input id="nm-title-' + state.windowId + '" class="nm-input" data-nm-field="title" maxlength="200" placeholder="' + esc(text(ctx, 'title_placeholder', {}, 'Auto-generated if empty')) + '">' +
        '</div>' +
        '<div class="nm-create-action">' +
            '<div data-nm-progress-slot></div>' +
            '<button type="button" class="nm-create-btn" data-nm-create-btn><span aria-hidden="true">♪</span>' + esc(text(ctx, 'create_button', {}, 'Create')) + '</button>' +
            '<div class="nm-create-reason" data-nm-reason>' + esc(text(ctx, 'create_hint', {}, 'Generation takes about 1–2 minutes.')) + '</div>' +
            '<div data-nm-result-slot></div>' +
        '</div>';
    }

    function progressMarkup(state) {
        const ctx = state.ctx;
        const esc = ctx.esc;
        return '<div class="nm-progress">' +
            '<div class="nm-eq" aria-hidden="true"><span></span><span></span><span></span><span></span><span></span></div>' +
            '<div><strong>' + esc(text(ctx, 'progress_title', {}, 'Your song is being created…')) + '</strong>' +
            '<p>' + esc(text(ctx, 'progress_hint', {}, 'This usually takes 1–2 minutes. You can keep browsing your library.')) + '</p></div>' +
            '<span class="nm-progress-time" data-nm-elapsed>0 s</span>' +
        '</div>';
    }

    function resultMarkup(state, result) {
        const ctx = state.ctx;
        const esc = ctx.esc;
        const title = result.title || state.generation.lastParams?.title || text(ctx, 'result_untitled', {}, 'Untitled');
        const meta = [];
        if (result.duration_ms) meta.push(window.NoisemakerLibrary.formatDuration(result.duration_ms));
        if (result.provider) meta.push(result.provider);
        const lyricsBlock = result.lyrics
            ? '<details class="nm-collapsible nm-result-lyrics"><summary>' + esc(text(ctx, 'lyrics_label', {}, 'Lyrics')) + (result.auto_lyrics ? ' ✨' : '') + '</summary>' +
              '<div class="nm-collapsible-body"><pre class="nm-lyrics-text">' + esc(result.lyrics) + '</pre></div></details>'
            : '';
        return '<div class="nm-result">' +
            (result.cover_url
                ? '<div class="nm-cover"><img src="' + esc(result.cover_url) + '" alt="" draggable="false"></div>'
                : '<div class="nm-cover nm-cover--empty" aria-hidden="true">♪</div>') +
            '<div class="nm-result-main">' +
                '<div class="nm-result-title">' + esc(title) + '</div>' +
                '<div class="nm-result-meta">' + esc(meta.join(' · ')) + (state.generation.coverFailed ? ' · ' + esc(text(ctx, 'cover_failed', {}, 'cover failed')) : '') + '</div>' +
                '<audio controls preload="metadata" src="' + esc(result.web_path) + '"></audio>' +
                '<div class="nm-result-actions">' +
                    '<button type="button" class="nm-btn nm-btn--primary" data-nm-result-library>' + esc(text(ctx, 'result_show_library', {}, 'Show in library')) + '</button>' +
                    '<a class="nm-btn" href="' + esc(result.web_path) + '" download="' + esc(result.filename || '') + '">' + esc(text(ctx, 'track_download', {}, 'Download')) + '</a>' +
                    '<button type="button" class="nm-btn" data-nm-result-new>' + esc(text(ctx, 'result_new', {}, 'New song')) + '</button>' +
                '</div>' +
                lyricsBlock +
            '</div>' +
        '</div>';
    }

    function errorMarkup(state, message) {
        const ctx = state.ctx;
        const esc = ctx.esc;
        return '<div class="nm-error">' +
            '<strong>' + esc(text(ctx, 'error_title', {}, 'Generation failed')) + '</strong>' +
            '<p>' + esc(message || text(ctx, 'error_unknown', {}, 'Unknown error.')) + '</p>' +
            '<button type="button" class="nm-btn" data-nm-retry>' + esc(text(ctx, 'error_retry', {}, 'Retry')) + '</button>' +
        '</div>';
    }

    function onboardingMarkup(state) {
        const ctx = state.ctx;
        const esc = ctx.esc;
        return '<div class="nm-onboarding"><div class="nm-onboarding-card">' +
            '<span class="nm-onboarding-icon" aria-hidden="true">♪</span>' +
            '<h2>' + esc(text(ctx, 'onboarding_title', {}, 'Music generation is not set up')) + '</h2>' +
            '<p>' + esc(text(ctx, 'onboarding_hint', {}, 'Enable a music provider (MiniMax or Google Lyria) in the settings to create songs.')) + '</p>' +
            '<div class="nm-onboarding-actions">' +
                '<button type="button" class="nm-btn nm-btn--primary" data-nm-open-settings>' + esc(text(ctx, 'onboarding_open_settings', {}, 'Open settings')) + '</button>' +
                '<button type="button" class="nm-btn" data-nm-recheck>' + esc(text(ctx, 'onboarding_recheck', {}, 'Check again')) + '</button>' +
            '</div>' +
        '</div></div>';
    }

    // ---------- view helpers ----------

    function qs(state, sel) { return state.root ? state.root.querySelector(sel) : null; }

    function syncHeader(state) {
        const caps = state.caps || {};
        const providerChip = qs(state, '[data-nm-provider]');
        if (providerChip) {
            if (caps.provider_type) {
                providerChip.hidden = false;
                providerChip.textContent = caps.model ? caps.provider_type + ' · ' + caps.model : caps.provider_type;
            } else {
                providerChip.hidden = true;
            }
        }
        const quotaChip = qs(state, '[data-nm-quota]');
        if (quotaChip) {
            const used = Number(caps.daily_used) || 0;
            const max = Number(caps.daily_max) || 0;
            quotaChip.hidden = false;
            quotaChip.textContent = max > 0
                ? text(state.ctx, 'quota', { used, max }, used + '/' + max + ' today')
                : text(state.ctx, 'quota_unlimited', { used }, used + ' today');
        }
        const busy = qs(state, '[data-nm-busy]');
        if (busy) busy.hidden = !state.generation.active;
        const count = qs(state, '[data-nm-track-count]');
        if (count) {
            const total = state.tracksTotal || state.tracks.length;
            count.hidden = total === 0;
            count.textContent = String(total);
        }
    }

    function switchView(state, view) {
        state.view = view;
        state.root.querySelectorAll('.nm-tab').forEach(tab => {
            tab.setAttribute('aria-selected', tab.dataset.view === view ? 'true' : 'false');
        });
        state.root.querySelectorAll('.nm-view').forEach(el => {
            el.classList.toggle('is-active', el.dataset.view === view);
        });
        if (view === 'library' && state.tracksLoadedOnce) refreshTracks(state);
    }

    function syncCounters(state) {
        state.root.querySelectorAll('[data-nm-counter]').forEach(el => {
            const field = el.dataset.nmCounter;
            const len = (state.form[field] || '').length;
            const max = field === 'lyrics' ? LYRICS_MAX : IDEA_MAX;
            el.textContent = len > 0 ? len + ' / ' + max : '';
        });
    }

    function syncCreateButton(state) {
        const btn = qs(state, '[data-nm-create-btn]');
        const reason = qs(state, '[data-nm-reason]');
        if (!btn) return;
        const caps = state.caps || {};
        const hasInput = !!(state.form.idea.trim() || state.form.style.trim());
        const max = Number(caps.daily_max) || 0;
        const used = Number(caps.daily_used) || 0;
        const quotaHit = max > 0 && used >= max;
        btn.disabled = state.generation.active || !hasInput || quotaHit;
        if (reason) {
            if (quotaHit) {
                reason.textContent = text(state.ctx, 'create_disabled_quota', { used, max }, 'Daily limit reached.');
            } else if (!hasInput) {
                reason.textContent = text(state.ctx, 'create_disabled_idea', {}, 'Enter a song idea or a style first.');
            } else {
                reason.textContent = text(state.ctx, 'create_hint', {}, 'Generation takes about 1–2 minutes.');
            }
        }
    }

    function renderSlots(state) {
        const progressSlot = qs(state, '[data-nm-progress-slot]');
        const resultSlot = qs(state, '[data-nm-result-slot]');
        if (progressSlot) progressSlot.innerHTML = state.generation.active ? progressMarkup(state) : '';
        if (resultSlot) {
            if (state.generation.error) {
                resultSlot.innerHTML = errorMarkup(state, state.generation.error);
            } else if (state.generation.result) {
                resultSlot.innerHTML = resultMarkup(state, state.generation.result);
            } else {
                resultSlot.innerHTML = '';
            }
        }
        syncCreateButton(state);
        syncHeader(state);
    }

    // ---------- data ----------

    async function loadState(state) {
        try {
            const data = await request(state, '/api/desktop/noisemaker/state');
            if (state.disposed) return;
            state.caps = data;
        } catch (err) {
            if (state.disposed) return;
            state.caps = { enabled: false, error: err.message || '' };
        }
        renderApp(state);
    }

    async function fetchTrackPage(state, offset) {
        const params = new URLSearchParams({ limit: String(TRACKS_PAGE_SIZE), offset: String(offset) });
        if (state.tracksQuery) params.set('q', state.tracksQuery);
        return await request(state, '/api/desktop/noisemaker/tracks?' + params.toString());
    }

    async function refreshTracks(state) {
        if (!state.library) return;
        state.tracksLoading = true;
        state.library.setLoading(true);
        try {
            const data = await fetchTrackPage(state, 0);
            if (state.disposed) return;
            state.tracks = Array.isArray(data.items) ? data.items : [];
            state.tracksTotal = Number(data.total) || 0;
            if (state.caps && typeof data.daily_used === 'number') state.caps.daily_used = data.daily_used;
            state.library.setTracks(state.tracks);
            state.library.setPagination({ total: state.tracksTotal, hasMore: state.tracks.length < state.tracksTotal, loading: false });
        } catch (_) {
            if (state.disposed) return;
        }
        state.tracksLoading = false;
        state.tracksLoadedOnce = true;
        state.library.setLoading(false);
        syncHeader(state);
    }

    async function loadMoreTracks(state) {
        if (!state.library || state.tracksLoading) return;
        if (state.tracksTotal > 0 && state.tracks.length >= state.tracksTotal) return;
        state.tracksLoading = true;
        state.library.setPagination({ total: state.tracksTotal, hasMore: true, loading: true });
        try {
            const data = await fetchTrackPage(state, state.tracks.length);
            if (state.disposed) return;
            const additions = Array.isArray(data.items) ? data.items : [];
            state.tracks = state.tracks.concat(additions);
            state.tracksTotal = Number(data.total) || state.tracksTotal;
            if (state.caps && typeof data.daily_used === 'number') state.caps.daily_used = data.daily_used;
            state.library.appendTracks(additions);
            state.library.setPagination({ total: state.tracksTotal, hasMore: state.tracks.length < state.tracksTotal, loading: false });
        } catch (_) {
            if (state.disposed) return;
            state.library.setPagination({ total: state.tracksTotal, hasMore: state.tracks.length < state.tracksTotal, loading: false });
        }
        state.tracksLoading = false;
        syncHeader(state);
    }

    // ---------- enhance ----------

    function enhanceContextFor(state, kind) {
        if (kind === 'idea') return state.form.style.trim();
        if (kind === 'style') return state.form.idea.trim();
        return [state.form.idea.trim(), state.form.style.trim()].filter(Boolean).join('\n');
    }

    async function enhance(state, kind, button) {
        const apiKind = kind === 'random' ? 'idea' : kind;
        const fieldFor = { idea: 'idea', random: 'idea', style: 'style', lyrics: 'lyrics', title: 'title' };
        const field = fieldFor[kind];
        if (!field) return;
        const currentValue = kind === 'random' ? '' : (state.form[field] || '');
        if (kind === 'style' && !currentValue.trim()) return;
        button.classList.add('is-busy');
        button.disabled = true;
        try {
            const data = await request(state, '/api/desktop/noisemaker/enhance', {
                method: 'POST',
                body: {
                    kind: apiKind,
                    text: currentValue,
                    context: enhanceContextFor(state, apiKind),
                    lang: uiLang()
                }
            });
            if (state.disposed) return;
            if (data && data.text) {
                state.form[field] = data.text;
                const input = qs(state, '[data-nm-field="' + field + '"]');
                if (input) {
                    input.value = data.text;
                    if (field === 'lyrics') {
                        const wrap = qs(state, '[data-nm-lyrics-wrap]');
                        if (wrap) wrap.open = true;
                    }
                }
                syncCounters(state);
                syncCreateButton(state);
            }
        } catch (err) {
            if (state.disposed) return;
            state.ctx.notify((err && err.message) || text(state.ctx, 'enhance_failed', {}, 'AI enhancement failed.'));
        } finally {
            button.classList.remove('is-busy');
            button.disabled = false;
        }
    }

    // ---------- generate ----------

    function startElapsedTimer(state) {
        stopElapsedTimer(state);
        state.generation.timerId = setInterval(() => {
            if (state.disposed) { stopElapsedTimer(state); return; }
            const el = qs(state, '[data-nm-elapsed]');
            if (el) {
                const secs = Math.max(0, Math.round((Date.now() - state.generation.startedAt) / 1000));
                el.textContent = text(state.ctx, 'progress_elapsed', { seconds: secs }, secs + ' s');
            }
        }, 1000);
    }

    function stopElapsedTimer(state) {
        if (state.generation.timerId) {
            clearInterval(state.generation.timerId);
            state.generation.timerId = null;
        }
    }

    async function generate(state) {
        if (state.generation.active) return;
        const params = {
            prompt: state.form.idea.trim(),
            style: state.form.style.trim(),
            lyrics: state.form.lyrics.trim(),
            title: state.form.title.trim(),
            instrumental: state.form.instrumental,
            cover: state.form.cover && state.caps && state.caps.covers_enabled === true,
            lang: uiLang()
        };
        if (!params.prompt && !params.style) return;
        state.generation.active = true;
        state.generation.startedAt = Date.now();
        state.generation.result = null;
        state.generation.error = '';
        state.generation.coverFailed = false;
        state.generation.lastParams = params;
        renderSlots(state);
        startElapsedTimer(state);
        try {
            const data = await request(state, '/api/desktop/noisemaker/generate', { method: 'POST', body: params });
            if (state.disposed) return;
            state.generation.result = data;
            state.generation.coverFailed = !!data.cover_error;
            if (state.caps && typeof data.daily_used === 'number') state.caps.daily_used = data.daily_used;
            state.ctx.notify(text(state.ctx, 'track_created_toast', { title: data.title || '' }, 'Song created.'));
            refreshTracks(state);
        } catch (err) {
            if (state.disposed) return;
            let message = (err && err.message) || text(state.ctx, 'error_unknown', {}, 'Unknown error.');
            if (err && err.body && err.body.code === 'lyrics_required') {
                message = text(state.ctx, 'lyrics_required', {}, message);
            }
            state.generation.error = message;
        } finally {
            state.generation.active = false;
            stopElapsedTimer(state);
            if (!state.disposed) renderSlots(state);
        }
    }

    // ---------- library actions ----------

    async function deleteTrack(state, track) {
        const confirmed = await state.ctx.confirmDialog(
            text(state.ctx, 'track_delete_title', {}, 'Delete song?'),
            text(state.ctx, 'track_delete_confirm', { title: track.title || '' }, 'This song will be permanently deleted.')
        );
        if (!confirmed || state.disposed) return;
        try {
            await request(state, '/api/desktop/noisemaker/tracks/' + encodeURIComponent(track.id), { method: 'DELETE' });
            if (state.disposed) return;
            state.ctx.notify(text(state.ctx, 'track_deleted', {}, 'Song deleted.'));
            state.tracks = state.tracks.filter(item => item.id !== track.id);
            state.tracksTotal = Math.max(0, state.tracksTotal - 1);
            state.library.setTracks(state.tracks);
            state.library.setPagination({ total: state.tracksTotal, hasMore: state.tracks.length < state.tracksTotal, loading: false });
            syncHeader(state);
        } catch (err) {
            if (state.disposed) return;
            state.ctx.notify((err && err.message) || text(state.ctx, 'error_unknown', {}, 'Unknown error.'));
        }
    }

    function useTemplate(state, track) {
        let style = '';
        let idea = String(track.prompt || '');
        const sep = idea.indexOf(' — ');
        if (sep > 0) {
            style = idea.slice(0, sep);
            idea = idea.slice(sep + 3);
        }
        state.form.idea = idea;
        if (style) state.form.style = style;
        state.form.instrumental = !!track.instrumental;
        const ideaInput = qs(state, '[data-nm-field="idea"]');
        const styleInput = qs(state, '[data-nm-field="style"]');
        const instrumentalInput = qs(state, '[data-nm-field="instrumental"]');
        if (ideaInput) ideaInput.value = state.form.idea;
        if (styleInput) styleInput.value = state.form.style;
        if (instrumentalInput) instrumentalInput.checked = state.form.instrumental;
        savePrefs(state);
        syncCounters(state);
        syncCreateButton(state);
        switchView(state, 'create');
    }

    // ---------- render ----------

    function renderApp(state) {
        if (state.disposed || !state.root) return;
        const caps = state.caps || {};
        const body = qs(state, '.nm-body');
        const tabs = qs(state, '.nm-tabs');
        if (!caps.enabled) {
            teardownViews(state);
            body.innerHTML = onboardingMarkup(state);
            const openBtn = qs(state, '[data-nm-open-settings]');
            if (openBtn) openBtn.addEventListener('click', () => window.open('/config', '_blank', 'noopener'));
            const recheck = qs(state, '[data-nm-recheck]');
            if (recheck) recheck.addEventListener('click', () => loadState(state));
            if (tabs) tabs.style.display = 'none';
            syncHeader(state);
            return;
        }
        if (tabs) tabs.style.display = '';
        teardownViews(state);
        body.innerHTML = bodyViewsMarkup();
        mountMainViews(state);
        switchView(state, state.view || 'create');
        syncHeader(state);
    }

    function teardownViews(state) {
        state.tracksLoadedOnce = false;
        if (state.library) {
            try { state.library.dispose(); } catch (_) {}
            state.library = null;
        }
    }

    function mountMainViews(state) {
        const ctx = state.ctx;
        const createSlot = qs(state, '[data-nm-create]');
        createSlot.innerHTML = createFormMarkup(state);

        // Restore persisted form values
        const ideaInput = qs(state, '[data-nm-field="idea"]');
        const styleInput = qs(state, '[data-nm-field="style"]');
        const lyricsInput = qs(state, '[data-nm-field="lyrics"]');
        const titleInput = qs(state, '[data-nm-field="title"]');
        const instrumentalInput = qs(state, '[data-nm-field="instrumental"]');
        const coverInput = qs(state, '[data-nm-field="cover"]');
        if (ideaInput) ideaInput.value = state.form.idea;
        if (styleInput) styleInput.value = state.form.style;
        if (lyricsInput) lyricsInput.value = state.form.lyrics;
        if (titleInput) titleInput.value = state.form.title;
        if (instrumentalInput) instrumentalInput.checked = state.form.instrumental;
        if (coverInput) coverInput.checked = state.form.cover;
        const lyricsWrapInit = qs(state, '[data-nm-lyrics-wrap]');
        if (lyricsWrapInit) lyricsWrapInit.classList.toggle('is-disabled', state.form.instrumental);

        createSlot.addEventListener('input', event => {
            const field = event.target.dataset ? event.target.dataset.nmField : '';
            if (!field) return;
            if (event.target.type === 'checkbox') {
                state.form[field] = event.target.checked;
            } else {
                state.form[field] = event.target.value;
            }
            if (field === 'instrumental') {
                const wrap = qs(state, '[data-nm-lyrics-wrap]');
                if (wrap) {
                    wrap.classList.toggle('is-disabled', state.form.instrumental);
                    if (state.form.instrumental) wrap.open = false;
                }
            }
            savePrefs(state);
            syncCounters(state);
            syncCreateButton(state);
        });
        createSlot.addEventListener('click', event => {
            const enhanceBtn = event.target.closest('[data-nm-enhance]');
            if (enhanceBtn) { enhance(state, enhanceBtn.dataset.nmEnhance, enhanceBtn); return; }
            const chip = event.target.closest('[data-nm-chip]');
            if (chip) {
                const tag = chip.dataset.nmChip;
                const current = state.form.style.trim();
                const has = current.toLowerCase().split(/[,;]/).map(s => s.trim()).includes(tag.toLowerCase());
                state.form.style = has ? current : (current ? current + ', ' + tag : tag);
                if (styleInput) styleInput.value = state.form.style;
                savePrefs(state);
                syncCreateButton(state);
                return;
            }
            if (event.target.closest('[data-nm-create-btn]')) { generate(state); return; }
            if (event.target.closest('[data-nm-retry]')) { generate(state); return; }
            if (event.target.closest('[data-nm-result-new]')) {
                state.generation.result = null;
                state.generation.error = '';
                renderSlots(state);
                if (ideaInput) { ideaInput.focus(); ideaInput.select(); }
                return;
            }
            if (event.target.closest('[data-nm-result-library]')) { switchView(state, 'library'); }
        });

        const libraryView = qs(state, '.nm-view-library');
        state.library = window.NoisemakerLibrary.create({
            esc: ctx.esc,
            lang: uiLang(),
            t: (key, params, fallback) => {
                const value = ctx.t(key, params || {});
                return value && value !== key ? value : (fallback || key);
            }
        });
        libraryView.appendChild(state.library.element);
        state.library.on('delete', track => deleteTrack(state, track));
        state.library.on('template', track => useTemplate(state, track));
        state.library.on('create', () => switchView(state, 'create'));
        state.library.on('loadmore', () => loadMoreTracks(state));
        state.library.on('needmore-for-play', () => loadMoreTracks(state));
        state.library.on('search', value => {
            state.tracksQuery = String(value || '').trim();
            refreshTracks(state);
        });

        syncCounters(state);
        renderSlots(state);
        refreshTracks(state);
    }

    function render(host, windowId, context) {
        if (!host) return;
        const ctx = context || {};
        ctx.esc = ctx.esc || (v => String(v == null ? '' : v));
        ctx.t = ctx.t || ((k) => k);
        ctx.api = ctx.api || (() => Promise.reject(new Error('api unavailable')));
        ctx.notify = ctx.notify || (() => {});
        ctx.confirmDialog = ctx.confirmDialog || (() => Promise.resolve(false));
        ctx.iconMarkup = ctx.iconMarkup || ((k, f) => '<span>' + ctx.esc(f || k || '') + '</span>');

        const state = createState(host, windowId, ctx);
        instances.set(windowId, state);

        host.innerHTML = shellMarkup(state);
        state.root = host.querySelector('.noisemaker-app');

        state.root.querySelectorAll('.nm-tab').forEach(tab => {
            tab.addEventListener('click', () => switchView(state, tab.dataset.view));
        });

        // Paint a loading skeleton first, then fetch capabilities.
        const body = state.root.querySelector('.nm-body');
        body.innerHTML = '<div class="nm-onboarding"><div class="nm-loading">' + ctx.esc(text(ctx, 'library_loading', {}, 'Loading…')) + '</div></div>';
        loadState(state);
    }

    function dispose(windowId) {
        const state = instances.get(windowId);
        if (!state) return;
        instances.delete(windowId);
        state.disposed = true;
        stopElapsedTimer(state);
        state.controllers.forEach(controller => { try { controller.abort(); } catch (_) {} });
        state.controllers.clear();
        if (state.library) {
            try { state.library.dispose(); } catch (_) {}
            state.library = null;
        }
        state.root = null;
    }

    window.NoisemakerApp = { render, dispose };
})();
