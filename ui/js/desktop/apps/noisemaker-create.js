(function () {
    'use strict';

    // NoisemakerCreate — the create panel of the Noisemaker studio.
    // Simple mode (idea + genre presets) and Custom mode (style, lyrics, title,
    // local ACE-Step controls). Emits `generate(params)`; the shell runs the
    // request and reports progress/result back through setGeneration().
    // Factory: create(deps) with deps = { esc, t, lang, readonly, request, notify,
    // formatDuration, menu, form, mode, windowId }.

    const IDEA_MAX = 2000;
    const STYLE_MAX = 500;
    const LYRICS_MAX = 8000;
    const TITLE_MAX = 200;
    const STYLE_SUGGESTIONS = ['Pop', 'Lo-Fi', 'Synthwave', 'Techno', 'Hip-Hop', 'Rock', 'Jazz', 'Ambient', 'Epic Orchestra', 'Acoustic', 'EDM', 'Metal'];

    const PRESETS = [
        { id: 'lofi', glyph: '☕', style: 'lo-fi hip hop, chill, mellow, vinyl crackle, jazzy chords, 80 bpm', instrumental: true },
        { id: 'synthwave', glyph: '🌆', style: 'synthwave, 80s, retro, neon, driving synth bass, gated reverb drums, 110 bpm', instrumental: false },
        { id: 'orchestral', glyph: '🎻', style: 'epic orchestral, cinematic, choir, brass, dramatic, trailer music', instrumental: true },
        { id: 'folk', glyph: '🎸', style: 'acoustic folk, warm, fingerpicked guitar, intimate vocals, organic', instrumental: false },
        { id: 'techno', glyph: '◼', style: 'deep techno, hypnotic, dark, rolling bassline, club, 128 bpm', instrumental: true },
        { id: 'boombap', glyph: '🎧', style: 'boom bap hip hop, 90s, dusty drums, soulful samples, vinyl, 92 bpm', instrumental: false },
        { id: 'jazz', glyph: '🎷', style: 'jazz lounge, smooth, late night, upright bass, brushed drums, piano trio', instrumental: true },
        { id: 'ambient', glyph: '🌌', style: 'cinematic ambient, slow, evolving pads, atmospheric, no drums, reverb', instrumental: true }
    ];

    const SONG_FIELDS = ['idea', 'style', 'title', 'lyrics'];
    const FIELD_LABELS = {
        idea: { labelKey: 'desktop.noisemaker_idea_label' },
        style: { labelKey: 'desktop.noisemaker_style_label' },
        title: { labelKey: 'desktop.noisemaker_title_label' },
        lyrics: { labelKey: 'desktop.noisemaker_lyrics_label' },
        bpm: { labelKey: 'desktop.noisemaker_bpm' },
        vocal_language: { labelKey: 'desktop.noisemaker_vocal_language' }
    };
    const ISSUE_TEXT = {
        lyrics_too_long: { messageKey: 'desktop.noisemaker_issue_lyrics_too_long' },
        unknown_tag: { messageKey: 'desktop.noisemaker_issue_unknown_tag' },
        unknown_tag_suggest: { messageKey: 'desktop.noisemaker_issue_unknown_tag_suggest' },
        tag_numbered: { messageKey: 'desktop.noisemaker_issue_tag_numbered' },
        tag_modifier_unsupported: { messageKey: 'desktop.noisemaker_issue_tag_modifier_unsupported' },
        tag_too_many_modifiers: { messageKey: 'desktop.noisemaker_issue_tag_too_many_modifiers' },
        heading_not_tag: { messageKey: 'desktop.noisemaker_issue_heading_not_tag' },
        repeat_marker: { messageKey: 'desktop.noisemaker_issue_repeat_marker' },
        no_section_tags: { messageKey: 'desktop.noisemaker_issue_no_section_tags' },
        bpm_in_style: { messageKey: 'desktop.noisemaker_issue_bpm_in_style' }
    };
    const PROVIDER_NAMES = { acestep: 'ACE-Step', minimax: 'MiniMax', google: 'Google Lyria' };

    function defaultForm() {
        return { idea: '', style: '', lyrics: '', title: '', instrumental: false, cover: true, duration_seconds: '120', bpm: '', vocal_language: '', seed: '' };
    }

    function emptyGeneration() {
        return { active: false, startedAt: 0, result: null, error: '', coverFailed: false, lastParams: null };
    }

    function create(deps) {
        const esc = deps.esc || (v => String(v == null ? '' : v));
        const t = deps.t || ((key, params, fallback) => fallback || key);
        const request = deps.request || (() => Promise.reject(new Error('request unavailable')));
        const notify = deps.notify || (() => {});
        const formatDuration = deps.formatDuration || (ms => Math.round((Number(ms) || 0) / 1000) + ' s');
        const lang = String(deps.lang || 'en');
        const windowId = String(deps.windowId || 'nm');
        const readonly = deps.readonly === true;
        const menu = deps.menu || (() => {});

        const handlers = {};
        const on = (name, cb) => { (handlers[name] = handlers[name] || []).push(cb); };
        const emit = (name, ...args) => { (handlers[name] || []).forEach(cb => { try { cb(...args); } catch (_) {} }); };

        const form = Object.assign(defaultForm(), deps.form || {});
        const fieldRevisions = Object.create(null);
        const enhancementRequests = Object.create(null);
        const enhancementBusyEls = Object.create(null);
        let mode = deps.mode === 'custom' ? 'custom' : 'simple';
        let caps = null;
        let generation = emptyGeneration();
        let timerId = null;
        let disposed = false;
        let undoState = null;      // { before: { field: previousValue } }
        let checkState = null;     // { loading, issues, provider, error, token }
        let songRequestId = 0;

        const root = document.createElement('div');
        root.className = 'nm-create';
        root.innerHTML =
            '<div class="nm-create-scroll">' +
                '<div class="nm-create-head">' +
                    '<div class="nm-segment nm-mode-switch" role="tablist" aria-label="' + esc(t('desktop.noisemaker_mode_simple')) + ' / ' + esc(t('desktop.noisemaker_mode_custom')) + '">' +
                        '<button type="button" class="nm-segment-btn" role="tab" data-nm-mode="simple" aria-selected="false">' + esc(t('desktop.noisemaker_mode_simple')) + '</button>' +
                        '<button type="button" class="nm-segment-btn" role="tab" data-nm-mode="custom" aria-selected="false">' + esc(t('desktop.noisemaker_mode_custom')) + '</button>' +
                    '</div>' +
                '</div>' +
                '<div class="nm-create-form" data-nm-form></div>' +
                '<div class="nm-create-action">' +
                    '<div data-nm-progress-slot></div>' +
                    '<button type="button" class="nm-create-btn" data-nm-create-btn><span aria-hidden="true">♪</span><span>' + esc(t('desktop.noisemaker_create_button')) + '</span></button>' +
                    '<div class="nm-create-reason" data-nm-reason></div>' +
                    '<div data-nm-result-slot></div>' +
                '</div>' +
            '</div>';

        const formEl = root.querySelector('[data-nm-form]');
        const qs = sel => root.querySelector(sel);

        // ---------- markup ----------

        function aiButton(action, key, glyph) {
            if (!caps || caps.llm_available === false) return '';
            return '<button type="button" class="nm-ai" data-nm-enhance="' + action + '"' + (readonly ? ' disabled title="' + esc(t('desktop.noisemaker_readonly_hint')) + '"' : '') + '>' +
                '<span class="nm-ai-glyph" aria-hidden="true">' + (glyph || '✨') + '</span>' + esc(t('desktop.noisemaker_' + key)) + '</button>';
        }

        function aiAvailable() {
            return !!(caps && caps.llm_available !== false);
        }

        function songRowMarkup() {
            if (!aiAvailable()) return '';
            return '<div class="nm-ai-row">' +
                '<button type="button" class="nm-ai nm-ai--song" data-nm-song title="' + esc(t('desktop.noisemaker_song_suggest_hint')) + '"' + (readonly ? ' disabled' : '') + '>' +
                    '<span class="nm-ai-glyph" aria-hidden="true">✨</span>' + esc(t('desktop.noisemaker_song_suggest')) + '</button>' +
            '</div>';
        }

        function undoMarkup() {
            return '<div class="nm-undo" data-nm-undo role="status" aria-live="polite" hidden></div>';
        }

        function lyricsToolsMarkup() {
            if (aiAvailable()) {
                return '<button type="button" class="nm-ai" data-nm-lyrics-menu aria-haspopup="menu">' +
                    '<span class="nm-ai-glyph" aria-hidden="true">✨</span>' + esc(t('desktop.noisemaker_lyrics_ai_menu')) + ' <span aria-hidden="true">▾</span></button>';
            }
            return '<button type="button" class="nm-ai" data-nm-lyrics-check>' +
                '<span class="nm-ai-glyph" aria-hidden="true">✓</span>' + esc(t('desktop.noisemaker_lyrics_check')) + '</button>';
        }

        function instructionMarkup() {
            return '<div class="nm-instruction" data-nm-instruction hidden>' +
                '<input class="nm-input" data-nm-instruction-input maxlength="500" placeholder="' + esc(t('desktop.noisemaker_revise_custom_placeholder')) + '" aria-label="' + esc(t('desktop.noisemaker_revise_custom')) + '">' +
                '<button type="button" class="nm-btn nm-btn--primary" data-nm-instruction-apply>' + esc(t('desktop.noisemaker_revise_apply')) + '</button>' +
                '<button type="button" class="nm-icon-btn nm-icon-btn--small" data-nm-instruction-close aria-label="' + esc(t('desktop.noisemaker_panel_close')) + '" title="' + esc(t('desktop.noisemaker_panel_close')) + '">×</button>' +
            '</div>';
        }

        function ideaFieldMarkup() {
            return '<div class="nm-field">' +
                '<div class="nm-field-head"><label for="nm-idea-' + esc(windowId) + '">' + esc(t('desktop.noisemaker_idea_label')) + '</label>' +
                    aiButton('idea', 'idea_enhance') + aiButton('random', 'idea_random', '🎲') + '</div>' +
                '<textarea id="nm-idea-' + esc(windowId) + '" class="nm-textarea nm-textarea--idea" data-nm-field="idea" maxlength="' + IDEA_MAX + '" placeholder="' + esc(t('desktop.noisemaker_idea_placeholder')) + '"></textarea>' +
                '<div class="nm-field-foot"><span class="nm-counter" data-nm-counter="idea"></span></div>' +
            '</div>';
        }

        function presetsMarkup() {
            return '<div class="nm-field">' +
                '<div class="nm-field-head"><span class="nm-field-label">' + esc(t('desktop.noisemaker_presets_label')) + '</span></div>' +
                '<div class="nm-presets" role="listbox" aria-label="' + esc(t('desktop.noisemaker_presets_label')) + '">' +
                    PRESETS.map(p => '<button type="button" class="nm-preset nm-preset--' + p.id + '" role="option" data-nm-preset="' + p.id + '" aria-selected="false" title="' + esc(t('desktop.noisemaker_preset_' + p.id + '_hint')) + '">' +
                        '<span class="nm-preset-glyph" aria-hidden="true">' + p.glyph + '</span>' +
                        '<span class="nm-preset-name">' + esc(t('desktop.noisemaker_preset_' + p.id)) + '</span>' +
                        '<span class="nm-preset-hint">' + esc(t('desktop.noisemaker_preset_' + p.id + '_hint')) + '</span>' +
                    '</button>').join('') +
                '</div>' +
                '<div class="nm-simple-style" data-nm-simple-style hidden><span class="nm-simple-style-text"></span>' +
                    '<button type="button" class="nm-icon-btn nm-icon-btn--small" data-nm-clear-style aria-label="' + esc(t('desktop.noisemaker_select_none')) + '" title="' + esc(t('desktop.noisemaker_select_none')) + '">×</button></div>' +
            '</div>';
        }

        function styleFieldMarkup() {
            return '<div class="nm-field">' +
                '<div class="nm-field-head"><label for="nm-style-' + esc(windowId) + '">' + esc(t('desktop.noisemaker_style_label')) + '</label>' + aiButton('style', 'style_enhance') + '</div>' +
                '<input id="nm-style-' + esc(windowId) + '" class="nm-input" data-nm-field="style" maxlength="' + STYLE_MAX + '" placeholder="' + esc(t('desktop.noisemaker_style_placeholder')) + '">' +
                '<div class="nm-chips">' + STYLE_SUGGESTIONS.map(tag => '<button type="button" class="nm-suggestion" data-nm-chip="' + esc(tag) + '">' + esc(tag) + '</button>').join('') + '</div>' +
            '</div>';
        }

        function switchesMarkup() {
            const c = caps || {};
            const provider = String(c.cover_provider || '').trim();
            const coverHint = t('desktop.noisemaker_cover_hint') + (provider ? ' · ' + provider : '');
            const cover = c.covers_enabled
                ? '<label class="nm-switch"><input type="checkbox" data-nm-field="cover"><span class="nm-switch-track" aria-hidden="true"></span>' + esc(t('desktop.noisemaker_cover_label')) + '</label>' +
                  '<span class="nm-hint">' + esc(coverHint) + '</span>'
                : '';
            return '<div class="nm-form-row">' +
                '<label class="nm-switch"><input type="checkbox" data-nm-field="instrumental"><span class="nm-switch-track" aria-hidden="true"></span>' + esc(t('desktop.noisemaker_instrumental')) + '</label>' +
                cover +
            '</div>';
        }

        function lyricsMarkup() {
            const c = caps || {};
            const required = lyricsRequired();
            const requirement = required ? 'desktop.noisemaker_lyrics_required_label' : 'desktop.noisemaker_optional';
            const body = c.supports_lyrics === false
                ? '<p class="nm-hint">' + esc(t('desktop.noisemaker_lyrics_unsupported')) + '</p>'
                : '<div class="nm-field-head">' + lyricsToolsMarkup() + '</div>' +
                  '<textarea class="nm-textarea nm-textarea--lyrics" data-nm-field="lyrics" maxlength="' + LYRICS_MAX + '" placeholder="' + esc(t('desktop.noisemaker_lyrics_placeholder')) + '"></textarea>' +
                  '<div class="nm-field-foot"><span class="nm-counter" data-nm-counter="lyrics"></span></div>' +
                  instructionMarkup() +
                  '<div class="nm-check" data-nm-check aria-live="polite" hidden></div>' +
                  '<p class="nm-hint" data-nm-lyrics-required-hint' + (required ? '' : ' hidden') + '>' + esc(t('desktop.noisemaker_lyrics_required_hint')) + '</p>';
            return '<details class="nm-collapsible" data-nm-lyrics-wrap' + (form.lyrics || required ? ' open' : '') + '>' +
                '<summary>' + esc(t('desktop.noisemaker_lyrics_label')) + ' <span class="nm-hint" data-nm-lyrics-requirement>(' + esc(t(requirement)) + ')</span></summary>' +
                '<div class="nm-collapsible-body">' + body + '</div>' +
            '</details>';
        }

        function titleMarkup() {
            return '<div class="nm-field">' +
                '<div class="nm-field-head"><label for="nm-title-' + esc(windowId) + '">' + esc(t('desktop.noisemaker_title_label')) + '</label>' + aiButton('title', 'title_suggest') + '</div>' +
                '<input id="nm-title-' + esc(windowId) + '" class="nm-input" data-nm-field="title" maxlength="' + TITLE_MAX + '" placeholder="' + esc(t('desktop.noisemaker_title_placeholder')) + '">' +
            '</div>';
        }

        function maxDuration() {
            return (caps && caps.local && caps.local.profile && Number(caps.local.profile.max_duration)) || 600;
        }

        function localControlsMarkup() {
            const inputs = [
                ['duration_seconds', 'number', 'min="10" max="' + maxDuration() + '" step="1" required'],
                ['bpm', 'number', 'min="30" max="300" step="1"'],
                ['vocal_language', 'text', 'maxlength="8" pattern="[a-z]{2,3}(-[A-Za-z]{2,4})?" placeholder="de, en, ja…"']
            ];
            return '<div class="nm-local-controls">' +
                inputs.map(([field, type, attrs]) => '<label class="nm-field">' + esc(t('desktop.noisemaker_' + field)) +
                    '<input class="nm-input" type="' + type + '" data-nm-field="' + field + '" ' + attrs + '></label>').join('') +
                '<label class="nm-field">' + esc(t('desktop.noisemaker_seed')) +
                    '<input class="nm-input" type="number" data-nm-field="seed" min="0" max="2147483647" step="1"></label>' +
            '</div>' +
            '<p class="nm-hint">' + esc(t('desktop.noisemaker_local_help')) + '</p>' +
            '<p class="nm-hint" role="status" aria-live="polite" data-nm-local-status></p>';
        }

        function progressMarkup() {
            return '<div class="nm-progress">' +
                '<div class="nm-eq" aria-hidden="true"><span></span><span></span><span></span><span></span><span></span></div>' +
                '<div><strong>' + esc(t('desktop.noisemaker_progress_title')) + '</strong><p>' + esc(t('desktop.noisemaker_progress_hint')) + '</p></div>' +
                '<span class="nm-progress-time" data-nm-elapsed>' + esc(t('desktop.noisemaker_progress_elapsed', { seconds: 0 })) + '</span>' +
            '</div>';
        }

        function resultMarkup(result) {
            const title = result.title || (generation.lastParams && generation.lastParams.title) || t('desktop.noisemaker_result_untitled');
            const meta = [];
            if (result.duration_ms) meta.push(formatDuration(result.duration_ms));
            if (result.provider) meta.push(result.provider);
            if (generation.coverFailed) meta.push(t('desktop.noisemaker_cover_failed'));
            const cover = result.cover_url
                ? '<div class="nm-cover nm-result-cover"><img src="' + esc(result.cover_url) + '" alt="" draggable="false"></div>'
                : '<div class="nm-cover nm-result-cover nm-cover--empty" aria-hidden="true">♪</div>';
            const lyrics = result.lyrics
                ? '<details class="nm-collapsible nm-result-lyrics"><summary>' + esc(t('desktop.noisemaker_lyrics_label')) + (result.auto_lyrics ? ' ✨' : '') + '</summary>' +
                  '<div class="nm-collapsible-body"><pre class="nm-lyrics-text">' + esc(result.lyrics) + '</pre></div></details>'
                : '';
            return '<div class="nm-result">' + cover +
                '<div class="nm-result-main">' +
                    '<div class="nm-result-title">' + esc(title) + '</div>' +
                    '<div class="nm-result-meta nm-muted">' + esc(meta.join(' · ')) + '</div>' +
                    '<div class="nm-result-actions">' +
                        '<button type="button" class="nm-btn nm-btn--primary" data-nm-result-play>▶ ' + esc(t('desktop.noisemaker_result_play')) + '</button>' +
                        '<button type="button" class="nm-btn" data-nm-result-library>' + esc(t('desktop.noisemaker_result_show_library')) + '</button>' +
                        '<a class="nm-btn" href="' + esc(result.web_path || '#') + '" download="' + esc(result.filename || '') + '">' + esc(t('desktop.noisemaker_track_download')) + '</a>' +
                        '<button type="button" class="nm-btn" data-nm-result-new>' + esc(t('desktop.noisemaker_result_new')) + '</button>' +
                    '</div>' +
                    lyrics +
                '</div>' +
            '</div>';
        }

        function errorMarkup(message) {
            const retryAttrs = readonly ? ' disabled title="' + esc(t('desktop.noisemaker_readonly_hint')) + '"' : '';
            return '<div class="nm-error" role="alert">' +
                '<strong>' + esc(t('desktop.noisemaker_error_title')) + '</strong>' +
                '<p>' + esc(message || t('desktop.noisemaker_error_unknown')) + '</p>' +
                '<button type="button" class="nm-btn" data-nm-retry' + retryAttrs + '>' + esc(t('desktop.noisemaker_error_retry')) + '</button>' +
            '</div>';
        }

        // ---------- rendering / sync ----------

        function renderForm() {
            const c = caps || {};
            formEl.innerHTML = (readonly ? '<p class="nm-hint" role="status">' + esc(t('desktop.noisemaker_readonly_hint')) + '</p>' : '') +
                songRowMarkup() + undoMarkup() + (mode === 'simple'
                ? ideaFieldMarkup() + presetsMarkup() + switchesMarkup()
                : ideaFieldMarkup() + styleFieldMarkup() + switchesMarkup() + lyricsMarkup() + titleMarkup() + (c.supports_controls ? localControlsMarkup() : ''));
            root.dataset.nmCreateMode = mode;
            root.querySelectorAll('.nm-segment-btn[data-nm-mode]').forEach(btn => {
                const active = btn.dataset.nmMode === mode;
                btn.classList.toggle('is-active', active);
                btn.setAttribute('aria-selected', active ? 'true' : 'false');
            });
            applyFormToInputs();
            syncPresets();
            syncCounters();
            syncCreateButton();
            renderUndo();
            renderCheck();
        }

        function applyFormToInputs() {
            root.querySelectorAll('[data-nm-field]').forEach(input => {
                const field = input.dataset.nmField;
                if (input.type === 'checkbox') input.checked = !!form[field];
                else input.value = form[field] == null ? '' : String(form[field]);
            });
        }

        function selectedPreset() {
            const style = String(form.style || '').trim();
            return PRESETS.find(p => p.style === style) || null;
        }

        function syncPresets() {
            const active = selectedPreset();
            root.querySelectorAll('[data-nm-preset]').forEach(btn => {
                const on = !!active && btn.dataset.nmPreset === active.id;
                btn.classList.toggle('is-active', on);
                btn.setAttribute('aria-selected', on ? 'true' : 'false');
            });
            const line = qs('[data-nm-simple-style]');
            if (line) {
                const style = String(form.style || '').trim();
                line.hidden = !style;
                line.querySelector('.nm-simple-style-text').textContent = style;
            }
        }

        function lyricsMax() {
            const guide = caps && caps.lyrics_guide;
            const max = guide ? Number(guide.max_chars) : 0;
            return max > 0 ? Math.min(max, LYRICS_MAX) : LYRICS_MAX;
        }

        function syncCounters() {
            root.querySelectorAll('[data-nm-counter]').forEach(el => {
                const field = el.dataset.nmCounter;
                const len = String(form[field] || '').length;
                const max = field === 'lyrics' ? lyricsMax() : IDEA_MAX;
                el.textContent = len > 0 ? len + ' / ' + max : '';
                el.classList.toggle('is-over', len > max);
            });
        }

        function localState() {
            return (caps && caps.local) || null;
        }

        function lyricsRequired() {
            const c = caps || {};
            const local = localState();
            return !!(c.supports_controls && c.supports_lyrics !== false && local && local.profile && !local.profile.lm_model && !form.instrumental);
        }

        function needsLyrics() {
            const lyrics = mode === 'custom' ? String(form.lyrics || '').trim() : '';
            return lyricsRequired() && !lyrics;
        }

        function invalidLocalInput() {
            if (!(caps && caps.supports_controls)) return null;
            const duration = String(form.duration_seconds == null ? '' : form.duration_seconds);
            const bpm = String(form.bpm == null ? '' : form.bpm);
            const language = String(form.vocal_language == null ? '' : form.vocal_language);
            const seed = String(form.seed == null ? '' : form.seed);
            let field = '';
            if (!integerInputInRange(duration, 10, maxDuration(), true)) field = 'duration_seconds';
            else if (!integerInputInRange(bpm, 30, 300, false)) field = 'bpm';
            else if (language !== '' && !/^[a-z]{2,3}(-[A-Za-z]{2,4})?$/.test(language)) field = 'vocal_language';
            else if (!integerInputInRange(seed, 0, 2147483647, false)) field = 'seed';
            if (!field) return null;
            return { field, input: root.querySelector('[data-nm-field="' + field + '"]') };
        }

        function integerInputInRange(raw, min, max, required) {
            const value = String(raw == null ? '' : raw);
            if (!value.trim()) return !required;
            if (!/^[+-]?(?:\d+\.?\d*|\.\d+)(?:e[+-]?\d+)?$/i.test(value)) return false;
            const number = Number(value);
            return Number.isFinite(number) && Number.isInteger(number) && number >= min && number <= max;
        }

        function syncLyricsRequirement() {
            const required = lyricsRequired();
            const label = qs('[data-nm-lyrics-requirement]');
            const hint = qs('[data-nm-lyrics-required-hint]');
            const input = qs('[data-nm-field="lyrics"]');
            const wrap = qs('[data-nm-lyrics-wrap]');
            if (label) label.textContent = '(' + t(required ? 'desktop.noisemaker_lyrics_required_label' : 'desktop.noisemaker_optional') + ')';
            if (hint) hint.hidden = !required;
            if (input) {
                input.required = required;
                input.setAttribute('aria-required', required ? 'true' : 'false');
                input.placeholder = t(required ? 'desktop.noisemaker_lyrics_required_hint' : 'desktop.noisemaker_lyrics_placeholder');
            }
            if (required && wrap) wrap.open = true;
        }

        function submissionState() {
            const c = caps || {};
            const local = localState();
            const hasInput = !!(String(form.idea || '').trim() || String(form.style || '').trim());
            const max = Number(c.daily_max) || 0;
            const used = Number(c.daily_used) || 0;
            const quotaHit = max > 0 && used >= max;
            const unavailable = !!(c.supports_controls && (!local || !local.ready || local.state === 'busy'));
            const lyricsMissing = needsLyrics();
            const invalid = invalidLocalInput();
            return { hasInput, max, used, quotaHit, unavailable, lyricsMissing, invalid,
                blocked: readonly || generation.active || !hasInput || quotaHit || unavailable || lyricsMissing || !!invalid };
        }

        function switchToCustomLink(reason) {
            reason.insertAdjacentHTML('beforeend', ' <button type="button" class="nm-link-btn" data-nm-switch-custom>' + esc(t('desktop.noisemaker_mode_switch_custom')) + '</button>');
        }

        function syncCreateButton() {
            const btn = qs('[data-nm-create-btn]');
            const reason = qs('[data-nm-reason]');
            syncLyricsRequirement();
            const state = submissionState();
            btn.disabled = state.blocked;

            const localStatus = qs('[data-nm-local-status]');
            const local = localState();
            if (localStatus) {
                localStatus.textContent = t('desktop.noisemaker_local_' + ((local && local.state) || 'starting')) +
                    (local && local.error_code ? ' · ' + local.error_code : '') +
                    (local && local.profile ? ' · ' + local.profile.model : '');
            }

            reason.textContent = '';
            if (readonly) {
                reason.textContent = t('desktop.noisemaker_readonly_hint');
            } else if (state.invalid) {
                reason.textContent = mode === 'custom' && state.invalid.input
                    ? state.invalid.input.validationMessage || t('desktop.noisemaker_local_controls_invalid')
                    : t('desktop.noisemaker_local_controls_invalid');
                if (mode === 'simple') switchToCustomLink(reason);
            } else if (state.lyricsMissing) {
                reason.textContent = t('desktop.noisemaker_lyrics_required');
                if (mode === 'simple') switchToCustomLink(reason);
            } else if (state.unavailable) {
                reason.textContent = t('desktop.noisemaker_local_' + ((local && local.state) || 'starting'));
            } else if (state.quotaHit) {
                reason.textContent = t('desktop.noisemaker_create_disabled_quota', { used: state.used, max: state.max });
            } else if (!state.hasInput) {
                reason.textContent = t('desktop.noisemaker_create_disabled_idea');
            } else {
                reason.textContent = t('desktop.noisemaker_create_hint');
            }
        }

        function renderSlots() {
            qs('[data-nm-progress-slot]').innerHTML = generation.active ? progressMarkup() : '';
            const slot = qs('[data-nm-result-slot]');
            slot.innerHTML = generation.error ? errorMarkup(generation.error) : (generation.result ? resultMarkup(generation.result) : '');
            root.classList.toggle('is-generating', generation.active);
            root.classList.toggle('has-result', !!generation.result && !generation.error);
            syncCreateButton();
        }

        function startTimer() {
            stopTimer();
            timerId = setInterval(() => {
                if (disposed) { stopTimer(); return; }
                const el = qs('[data-nm-elapsed]');
                if (!el) return;
                const secs = Math.max(0, Math.round((Date.now() - generation.startedAt) / 1000));
                el.textContent = t('desktop.noisemaker_progress_elapsed', { seconds: secs });
            }, 1000);
        }

        function stopTimer() {
            if (timerId) { clearInterval(timerId); timerId = null; }
        }

        // ---------- form mutations ----------

        function assignForm(patch) {
            Object.entries(patch || {}).forEach(([field, value]) => {
                if (!Object.is(form[field], value)) fieldRevisions[field] = (fieldRevisions[field] || 0) + 1;
                form[field] = value;
            });
        }

        function writeForm(patch) {
            assignForm(patch);
            applyFormToInputs();
            syncPresets();
            syncCounters();
            syncCreateButton();
            if (patch && Object.prototype.hasOwnProperty.call(patch, 'lyrics')) closeCheck();
        }

        // setForm is used for user-driven and external changes; it drops the AI undo.
        function setForm(patch) {
            clearUndo();
            writeForm(patch);
        }

        function applyAiChange(patch) {
            const before = {};
            Object.keys(patch).forEach(field => { before[field] = form[field]; });
            writeForm(patch);
            undoState = { before };
            renderUndo();
            emit('change', getForm());
        }

        function clearUndo() {
            if (!undoState) return;
            undoState = null;
            renderUndo();
        }

        function undoAi() {
            if (!undoState) return;
            const before = undoState.before;
            undoState = null;
            writeForm(before);
            renderUndo();
            emit('change', getForm());
        }

        function renderUndo() {
            const bar = qs('[data-nm-undo]');
            if (!bar) return;
            if (!undoState) { bar.hidden = true; bar.innerHTML = ''; return; }
            const names = Object.keys(undoState.before).filter(field => FIELD_LABELS[field]).map(field => t(FIELD_LABELS[field].labelKey));
            bar.hidden = false;
            bar.innerHTML = '<span class="nm-undo-text">' + esc(t('desktop.noisemaker_ai_applied', { fields: names.join(', ') })) + '</span>' +
                '<button type="button" class="nm-link-btn" data-nm-undo-btn>' + esc(t('desktop.noisemaker_ai_undo')) + '</button>';
        }

        function openLyrics() {
            const wrap = qs('[data-nm-lyrics-wrap]');
            if (wrap) wrap.open = true;
        }

        function getForm() {
            return Object.assign({}, form);
        }

        function applyPreset(id) {
            const preset = PRESETS.find(p => p.id === id);
            if (!preset) return;
            const current = selectedPreset();
            if (current && current.id === preset.id) {
                setForm({ style: '' });
            } else {
                setForm({ style: preset.style, instrumental: preset.instrumental });
            }
            emit('change', getForm());
            if (!String(form.idea || '').trim()) focusIdea();
        }

        function appendStyle(tag) {
            const parts = String(form.style || '').split(',').map(s => s.trim()).filter(Boolean);
            if (!parts.some(p => p.toLowerCase() === tag.toLowerCase())) parts.push(tag);
            setForm({ style: parts.join(', ') });
            emit('change', getForm());
        }

        function setMode(next) {
            const value = next === 'custom' ? 'custom' : 'simple';
            if (value === mode && formEl.childElementCount) return;
            mode = value;
            clearUndo(); checkState = null;
            renderForm();
            emit('mode', mode);
        }

        function focusIdea() {
            const el = qs('[data-nm-field="idea"]');
            if (el) el.focus();
        }

        // ---------- enhance ----------

        function enhanceContextFor(kind) {
            if (kind === 'idea') return String(form.style || '').trim();
            if (kind === 'style') return String(form.idea || '').trim();
            return [String(form.idea || '').trim(), String(form.style || '').trim()].filter(Boolean).join('\n');
        }

        function setBusy(el, on) {
            if (!el) return;
            el.classList.toggle('is-busy', on);
            el.disabled = on;
        }

        function aiErrorMessage(err) {
            if (err && err.body && err.body.code === 'ai_unreadable') return t('desktop.noisemaker_ai_unreadable');
            return (err && err.message) || t('desktop.noisemaker_enhance_failed');
        }

        async function runFieldRequest(field, busyEl, body) {
            if (readonly || disposed) return;
            const revision = fieldRevisions[field] || 0;
            const requestId = (enhancementRequests[field] || 0) + 1;
            enhancementRequests[field] = requestId;
            enhancementBusyEls[field] = busyEl;
            const isCurrent = () => !disposed && enhancementRequests[field] === requestId && (fieldRevisions[field] || 0) === revision;
            setBusy(busyEl, true);
            try {
                const data = await request('/api/desktop/noisemaker/enhance', { method: 'POST', body });
                if (!isCurrent()) return;
                if (data && !data.unchanged && typeof data.text === 'string' && data.text && data.text !== form[field]) {
                    applyAiChange({ [field]: data.text });
                    if (field === 'lyrics') openLyrics();
                }
            } catch (err) {
                if (!isCurrent()) return;
                notify(aiErrorMessage(err));
            } finally {
                // a superseded request keeps a shared button busy only while the newer one uses the same button
                if (enhancementRequests[field] === requestId || enhancementBusyEls[field] !== busyEl) setBusy(busyEl, false);
            }
        }

        function enhance(kind, button) {
            const apiKind = kind === 'random' ? 'idea' : kind;
            const fieldFor = { idea: 'idea', random: 'idea', style: 'style', lyrics: 'lyrics', title: 'title' };
            const field = fieldFor[kind];
            if (!field) return;
            const currentValue = kind === 'random' ? '' : String(form[field] || '');
            if (kind === 'style' && !currentValue.trim()) return;
            runFieldRequest(field, button, { kind: apiKind, text: currentValue, context: enhanceContextFor(apiKind), lang });
        }

        function revise(action, instruction) {
            const text = String(form.lyrics || '');
            if (!text.trim()) return;
            runFieldRequest('lyrics', qs('[data-nm-lyrics-menu]'), {
                kind: 'revise', action, instruction: instruction || '', text, context: enhanceContextFor('lyrics'), lang
            });
        }

        async function suggestSong(body, overrides) {
            if (readonly || disposed || !aiAvailable()) return;
            const extra = overrides || {};
            const instrumental = Object.prototype.hasOwnProperty.call(extra, 'instrumental') ? !!extra.instrumental : !!form.instrumental;
            const controls = !!(caps && caps.supports_controls);
            const fields = SONG_FIELDS.concat(['instrumental'], controls ? ['bpm', 'vocal_language'] : []);
            fields.forEach(field => { enhancementRequests[field] = (enhancementRequests[field] || 0) + 1; });
            const revisions = fields.map(field => fieldRevisions[field] || 0);
            const requestId = ++songRequestId;
            const isCurrent = () => !disposed && songRequestId === requestId && fields.every((field, i) => (fieldRevisions[field] || 0) === revisions[i]);
            setBusy(qs('[data-nm-song]'), true);
            root.classList.add('is-suggesting');
            try {
                const data = await request('/api/desktop/noisemaker/enhance', { method: 'POST', body: Object.assign({ lang, instrumental }, body) });
                if (!isCurrent() || !data || !data.song) return;
                const song = data.song;
                const patch = { idea: String(song.idea || ''), style: String(song.style || ''), title: String(song.title || '') };
                if (instrumental !== !!form.instrumental) patch.instrumental = instrumental;
                if (!instrumental && !(caps && caps.supports_lyrics === false)) patch.lyrics = String(song.lyrics || '');
                if (controls && song.bpm) patch.bpm = String(song.bpm);
                if (controls && song.vocal_language) patch.vocal_language = String(song.vocal_language);
                if (mode !== 'custom') setMode('custom');
                applyAiChange(patch);
                openLyrics();
            } catch (err) {
                if (isCurrent()) notify(aiErrorMessage(err));
            } finally {
                if (songRequestId === requestId) {
                    setBusy(qs('[data-nm-song]'), false);
                    root.classList.remove('is-suggesting');
                }
            }
        }

        function suggestVariation(base, variationMode) {
            if (!base) return;
            suggestSong({
                kind: 'variation',
                mode: variationMode,
                base: { title: String(base.title || ''), idea: String(base.idea || ''), style: String(base.style || ''), lyrics: String(base.lyrics || '') }
            }, { instrumental: !!base.instrumental });
        }

        function openLyricsMenu(button) {
            const hasText = !!String(form.lyrics || '').trim();
            const aiOff = readonly || !aiAvailable();
            const items = [
                { id: 'lyrics-write', label: hasText ? t('desktop.noisemaker_lyrics_continue') : t('desktop.noisemaker_lyrics_generate'), icon: 'edit', fallback: '+', disabled: aiOff, action: () => enhance('lyrics', button) },
                { separator: true },
                { id: 'lyrics-chorus', label: t('desktop.noisemaker_revise_catchier_chorus'), icon: 'audio', fallback: '♪', disabled: aiOff || !hasText, action: () => revise('catchier_chorus') },
                { id: 'lyrics-shorten', label: t('desktop.noisemaker_revise_shorten'), icon: 'x', fallback: '-', disabled: aiOff || !hasText, action: () => revise('shorten') },
                { id: 'lyrics-rhymes', label: t('desktop.noisemaker_revise_rhymes'), icon: 'refresh', fallback: '~', disabled: aiOff || !hasText, action: () => revise('rhymes') },
                { id: 'lyrics-bridge', label: t('desktop.noisemaker_revise_add_bridge'), icon: 'plus', fallback: '+', disabled: aiOff || !hasText, action: () => revise('add_bridge') },
                { id: 'lyrics-custom', label: t('desktop.noisemaker_revise_custom'), icon: 'edit', fallback: '…', disabled: aiOff || !hasText, action: () => openInstruction() },
                { separator: true },
                { id: 'lyrics-check', label: t('desktop.noisemaker_lyrics_check'), icon: 'check-square', fallback: '✓', disabled: !hasText, action: () => checkLyrics() }
            ];
            const rect = button.getBoundingClientRect();
            menu(rect.left, rect.bottom + 4, items);
        }

        function openInstruction() {
            const row = qs('[data-nm-instruction]');
            if (!row) return;
            openLyrics();
            row.hidden = false;
            const input = row.querySelector('[data-nm-instruction-input]');
            if (input) input.focus();
        }

        function closeInstruction() {
            const row = qs('[data-nm-instruction]');
            if (row) row.hidden = true;
        }

        function applyInstruction() {
            const input = qs('[data-nm-instruction-input]');
            const value = input ? input.value.trim() : '';
            if (!value) { if (input) input.focus(); return; }
            input.value = '';
            closeInstruction();
            revise('custom', value);
        }

        function closeCheck() {
            if (!checkState) return;
            checkState = null;
            renderCheck();
        }

        async function checkLyrics() {
            if (disposed) return;
            const lyrics = String(form.lyrics || '');
            if (!lyrics.trim()) return;
            const revision = fieldRevisions.lyrics || 0;
            const token = {};
            checkState = { loading: true, issues: [], token };
            openLyrics();
            renderCheck();
            try {
                const data = await request('/api/desktop/noisemaker/lyrics/check', { method: 'POST', body: { lyrics, style: String(form.style || '') } });
                if (disposed || !checkState || checkState.token !== token || (fieldRevisions.lyrics || 0) !== revision) return;
                checkState = { loading: false, issues: Array.isArray(data && data.issues) ? data.issues : [], provider: String((data && data.provider) || ''), token };
            } catch (err) {
                if (disposed || !checkState || checkState.token !== token) return;
                checkState = { loading: false, issues: [], error: (err && err.message) || t('desktop.noisemaker_check_failed'), token };
            }
            renderCheck();
        }

        function issueText(issue) {
            const params = Object.assign({}, issue.params || {});
            let entry = ISSUE_TEXT[issue.code];
            if (issue.code === 'unknown_tag' && params.suggestion) entry = ISSUE_TEXT.unknown_tag_suggest;
            return entry ? t(entry.messageKey, params) : String(issue.code || '');
        }

        function renderCheck() {
            const el = qs('[data-nm-check]');
            if (!el) return;
            if (!checkState) { el.hidden = true; el.innerHTML = ''; return; }
            el.hidden = false;
            const closeLabel = esc(t('desktop.noisemaker_panel_close'));
            const head = '<div class="nm-check-head"><strong>' + esc(t('desktop.noisemaker_check_title')) + '</strong>' +
                '<button type="button" class="nm-icon-btn nm-icon-btn--small" data-nm-check-close aria-label="' + closeLabel + '" title="' + closeLabel + '">×</button></div>';
            if (checkState.loading) { el.innerHTML = head + '<p class="nm-hint">' + esc(t('desktop.noisemaker_check_running')) + '</p>'; return; }
            if (checkState.error) { el.innerHTML = head + '<p class="nm-hint nm-check-error">' + esc(checkState.error) + '</p>'; return; }
            const issues = checkState.issues;
            if (!issues.length) {
                const provider = PROVIDER_NAMES[checkState.provider] || checkState.provider || '';
                el.innerHTML = head + '<p class="nm-check-ok">✓ ' + esc(t('desktop.noisemaker_check_ok', { provider })) + '</p>';
                return;
            }
            const fixable = issues.some(issue => issue.severity === 'error' || issue.severity === 'warning');
            const canFix = fixable && aiAvailable() && !readonly;
            const glyph = { error: '⛔', warning: '⚠', info: 'ℹ' };
            el.innerHTML = head + '<ul class="nm-check-list">' + issues.map(issue =>
                '<li class="nm-check-item nm-check-item--' + esc(issue.severity) + '">' +
                    '<span class="nm-check-sev" aria-hidden="true">' + (glyph[issue.severity] || 'ℹ') + '</span>' +
                    (issue.line ? '<button type="button" class="nm-link-btn nm-check-line" data-nm-check-line="' + Number(issue.line) + '">' + esc(t('desktop.noisemaker_check_line', { line: issue.line })) + '</button>' : '') +
                    '<span class="nm-check-text">' + esc(issueText(issue)) + '</span></li>').join('') + '</ul>' +
                (canFix ? '<button type="button" class="nm-btn nm-btn--primary nm-check-fix" data-nm-check-fix>✨ ' + esc(t('desktop.noisemaker_check_fix')) + '</button>' : '');
        }

        function selectLyricsLine(lineNo) {
            const area = qs('[data-nm-field="lyrics"]');
            if (!area || !(lineNo > 0)) return;
            const lines = area.value.split('\n');
            let start = 0;
            for (let i = 0; i < lineNo - 1 && i < lines.length; i++) start += lines[i].length + 1;
            const end = start + (lines[lineNo - 1] || '').length;
            area.focus();
            area.setSelectionRange(start, end);
        }

        // ---------- generate ----------

        function buildParams() {
            const c = caps || {};
            const params = {
                prompt: String(form.idea || '').trim(),
                style: String(form.style || '').trim(),
                lyrics: mode === 'custom' ? String(form.lyrics || '').trim() : '',
                title: mode === 'custom' ? String(form.title || '').trim() : '',
                instrumental: !!form.instrumental,
                cover: form.cover !== false && c.covers_enabled === true,
                lang
            };
            if (c.supports_controls) {
                const duration = String(form.duration_seconds == null ? '' : form.duration_seconds).trim();
                const bpm = String(form.bpm == null ? '' : form.bpm).trim();
                const seed = String(form.seed == null ? '' : form.seed).trim();
                params.duration_seconds = duration === '' ? 120 : Number(duration);
                if (bpm !== '') params.bpm = Number(bpm);
                params.vocal_language = String(form.vocal_language || '').trim();
                if (seed !== '') params.seed = Number(seed);
            }
            return params;
        }

        function submit() {
            const state = submissionState();
            if (state.blocked) {
                if (!readonly && !generation.active && state.invalid && mode === 'custom' && state.invalid.input) state.invalid.input.reportValidity();
                return;
            }
            const params = buildParams();
            if (!params.prompt && !params.style) return;
            emit('generate', params);
        }

        // ---------- public setters ----------

        function setCaps(next, options) {
            caps = next || {};
            if (options && options.light && formEl.childElementCount) {
                const duration = qs('[data-nm-field="duration_seconds"]');
                if (duration) duration.max = String(maxDuration());
                syncCreateButton();
                return;
            }
            renderForm();
            renderSlots();
        }

        function setGeneration(next) {
            generation = Object.assign(emptyGeneration(), next || {});
            renderSlots();
            if (generation.active) startTimer(); else stopTimer();
        }

        // ---------- events ----------

        root.addEventListener('input', event => {
            const input = event.target.closest('[data-nm-field]');
            if (!input) return;
            const field = input.dataset.nmField;
            assignForm({ [field]: input.type === 'checkbox' ? input.checked : input.value });
            if (undoState && Object.prototype.hasOwnProperty.call(undoState.before, field)) clearUndo();
            if (field === 'lyrics') closeCheck();
            syncCounters();
            syncPresets();
            syncCreateButton();
            emit('change', getForm());
        });

        root.addEventListener('change', event => {
            const input = event.target.closest('[data-nm-field]');
            if (!input || input.type !== 'checkbox') return;
            const field = input.dataset.nmField;
            assignForm({ [field]: input.checked });
            if (undoState && Object.prototype.hasOwnProperty.call(undoState.before, field)) clearUndo();
            syncCreateButton();
            emit('change', getForm());
        });

        root.addEventListener('click', event => {
            const modeBtn = event.target.closest('.nm-segment-btn[data-nm-mode]');
            if (modeBtn) { setMode(modeBtn.dataset.nmMode); return; }
            if (event.target.closest('[data-nm-switch-custom]')) {
                setMode('custom');
                const wrap = qs('[data-nm-lyrics-wrap]');
                if (wrap) wrap.open = true;
                const lyrics = qs('[data-nm-field="lyrics"]');
                if (lyrics) lyrics.focus();
                return;
            }
            const preset = event.target.closest('[data-nm-preset]');
            if (preset) { applyPreset(preset.dataset.nmPreset); return; }
            if (event.target.closest('[data-nm-clear-style]')) { setForm({ style: '' }); emit('change', getForm()); return; }
            const chip = event.target.closest('[data-nm-chip]');
            if (chip) { appendStyle(chip.dataset.nmChip); return; }
            if (event.target.closest('[data-nm-song]')) { suggestSong({ kind: 'song', text: String(form.idea || '').trim(), context: String(form.style || '').trim() }); return; }
            const lyricsMenuBtn = event.target.closest('[data-nm-lyrics-menu]');
            if (lyricsMenuBtn) { openLyricsMenu(lyricsMenuBtn); return; }
            if (event.target.closest('[data-nm-lyrics-check]')) { checkLyrics(); return; }
            if (event.target.closest('[data-nm-undo-btn]')) { undoAi(); return; }
            if (event.target.closest('[data-nm-instruction-apply]')) { applyInstruction(); return; }
            if (event.target.closest('[data-nm-instruction-close]')) { closeInstruction(); return; }
            if (event.target.closest('[data-nm-check-close]')) { closeCheck(); return; }
            if (event.target.closest('[data-nm-check-fix]')) { closeCheck(); revise('fix_tags'); return; }
            const lineBtn = event.target.closest('[data-nm-check-line]');
            if (lineBtn) { selectLyricsLine(Number(lineBtn.dataset.nmCheckLine)); return; }
            const ai = event.target.closest('[data-nm-enhance]');
            if (ai) { enhance(ai.dataset.nmEnhance, ai); return; }
            if (event.target.closest('[data-nm-create-btn]') || event.target.closest('[data-nm-retry]')) { submit(); return; }
            if (event.target.closest('[data-nm-result-new]')) { emit('new-song'); return; }
            if (event.target.closest('[data-nm-result-library]')) { if (generation.result) emit('show-in-library', generation.result); return; }
            if (event.target.closest('[data-nm-result-play]')) { if (generation.result) emit('play-result', generation.result); }
        });

        root.addEventListener('keydown', event => {
            if (event.target.closest('[data-nm-instruction-input]')) {
                if (event.key === 'Enter' && !event.isComposing) { event.preventDefault(); applyInstruction(); }
                else if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); closeInstruction(); }
                return;
            }
            if ((event.ctrlKey || event.metaKey) && event.key === 'Enter' && event.target.closest('[data-nm-field]')) {
                event.preventDefault();
                submit();
            }
        });

        function dispose() {
            disposed = true;
            stopTimer();
            Object.keys(handlers).forEach(key => { handlers[key] = []; });
            root.remove();
        }

        renderForm();
        renderSlots();

        return {
            element: root,
            setCaps, setMode, getMode: () => mode,
            setForm, getForm, setGeneration, focusIdea, suggestVariation,
            on, dispose
        };
    }

    window.NoisemakerCreate = { create, PRESETS };
})();
