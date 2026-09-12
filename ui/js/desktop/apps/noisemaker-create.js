(function () {
    'use strict';

    // NoisemakerCreate — the create panel of the Noisemaker studio.
    // Simple mode (idea + genre presets) and Custom mode (style, lyrics, title,
    // local ACE-Step controls). Emits `generate(params)`; the shell runs the
    // request and reports progress/result back through setGeneration().
    // Factory: create(deps) with deps = { esc, t, lang, readonly, request, notify,
    // formatDuration, form, mode, windowId }.

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

        const handlers = {};
        const on = (name, cb) => { (handlers[name] = handlers[name] || []).push(cb); };
        const emit = (name, ...args) => { (handlers[name] || []).forEach(cb => { try { cb(...args); } catch (_) {} }); };

        const form = Object.assign(defaultForm(), deps.form || {});
        let mode = deps.mode === 'custom' ? 'custom' : 'simple';
        let caps = null;
        let generation = emptyGeneration();
        let timerId = null;
        let disposed = false;

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
            return '<button type="button" class="nm-ai" data-nm-enhance="' + action + '">' +
                '<span class="nm-ai-glyph" aria-hidden="true">' + (glyph || '✨') + '</span>' + esc(t('desktop.noisemaker_' + key)) + '</button>';
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
            const cover = c.covers_enabled && mode === 'custom'
                ? '<label class="nm-check"><input type="checkbox" data-nm-field="cover">' + esc(t('desktop.noisemaker_cover_label')) + '</label>' +
                  '<span class="nm-hint">' + esc(t('desktop.noisemaker_cover_hint', { provider: c.cover_provider || '' })) + '</span>'
                : '';
            return '<div class="nm-form-row">' +
                '<label class="nm-switch"><input type="checkbox" data-nm-field="instrumental"><span class="nm-switch-track" aria-hidden="true"></span>' + esc(t('desktop.noisemaker_instrumental')) + '</label>' +
                cover +
            '</div>';
        }

        function lyricsMarkup() {
            const c = caps || {};
            const body = c.supports_lyrics === false
                ? '<p class="nm-hint">' + esc(t('desktop.noisemaker_lyrics_unsupported')) + '</p>'
                : '<div class="nm-field-head">' + aiButton('lyrics', 'lyrics_generate') + '</div>' +
                  '<textarea class="nm-textarea nm-textarea--lyrics" data-nm-field="lyrics" maxlength="' + LYRICS_MAX + '" placeholder="' + esc(t('desktop.noisemaker_lyrics_placeholder')) + '"></textarea>' +
                  '<div class="nm-field-foot"><span class="nm-counter" data-nm-counter="lyrics"></span></div>';
            return '<details class="nm-collapsible" data-nm-lyrics-wrap' + (form.lyrics ? ' open' : '') + '>' +
                '<summary>' + esc(t('desktop.noisemaker_lyrics_label')) + ' <span class="nm-hint">(' + esc(t('desktop.noisemaker_optional')) + ')</span></summary>' +
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
            return '<div class="nm-error" role="alert">' +
                '<strong>' + esc(t('desktop.noisemaker_error_title')) + '</strong>' +
                '<p>' + esc(message || t('desktop.noisemaker_error_unknown')) + '</p>' +
                '<button type="button" class="nm-btn" data-nm-retry>' + esc(t('desktop.noisemaker_error_retry')) + '</button>' +
            '</div>';
        }

        // ---------- rendering / sync ----------

        function renderForm() {
            const c = caps || {};
            formEl.innerHTML = mode === 'simple'
                ? ideaFieldMarkup() + presetsMarkup() + switchesMarkup()
                : ideaFieldMarkup() + styleFieldMarkup() + switchesMarkup() + lyricsMarkup() + titleMarkup() + (c.supports_controls ? localControlsMarkup() : '');
            root.dataset.nmMode = mode;
            root.querySelectorAll('[data-nm-mode]').forEach(btn => {
                const active = btn.dataset.nmMode === mode;
                btn.classList.toggle('is-active', active);
                btn.setAttribute('aria-selected', active ? 'true' : 'false');
            });
            applyFormToInputs();
            syncPresets();
            syncCounters();
            syncCreateButton();
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

        function syncCounters() {
            root.querySelectorAll('[data-nm-counter]').forEach(el => {
                const field = el.dataset.nmCounter;
                const len = String(form[field] || '').length;
                const max = field === 'lyrics' ? LYRICS_MAX : IDEA_MAX;
                el.textContent = len > 0 ? len + ' / ' + max : '';
            });
        }

        function localState() {
            return (caps && caps.local) || null;
        }

        function needsLyrics() {
            const c = caps || {};
            const local = localState();
            const lyrics = mode === 'custom' ? String(form.lyrics || '').trim() : '';
            return !!(c.supports_controls && local && local.profile && !local.profile.lm_model && !form.instrumental && !lyrics);
        }

        function invalidLocalInput() {
            return Array.from(root.querySelectorAll('.nm-local-controls input')).find(input => !input.checkValidity()) || null;
        }

        function syncCreateButton() {
            const btn = qs('[data-nm-create-btn]');
            const reason = qs('[data-nm-reason]');
            const c = caps || {};
            const local = localState();
            const hasInput = !!(String(form.idea || '').trim() || String(form.style || '').trim());
            const max = Number(c.daily_max) || 0;
            const used = Number(c.daily_used) || 0;
            const quotaHit = max > 0 && used >= max;
            const unavailable = !!(c.supports_controls && (!local || !local.ready || local.state === 'busy'));
            const lyricsMissing = needsLyrics();
            const invalid = invalidLocalInput();
            btn.disabled = generation.active || !hasInput || quotaHit || unavailable || lyricsMissing || !!invalid;

            const localStatus = qs('[data-nm-local-status]');
            if (localStatus) {
                localStatus.textContent = t('desktop.noisemaker_local_' + ((local && local.state) || 'starting')) +
                    (local && local.error_code ? ' · ' + local.error_code : '') +
                    (local && local.profile ? ' · ' + local.profile.model : '');
            }

            reason.textContent = '';
            if (invalid) {
                reason.textContent = invalid.validationMessage;
            } else if (lyricsMissing) {
                reason.textContent = t('desktop.noisemaker_lyrics_required');
                if (mode === 'simple') {
                    reason.insertAdjacentHTML('beforeend', ' <button type="button" class="nm-link-btn" data-nm-switch-custom>' + esc(t('desktop.noisemaker_mode_switch_custom')) + '</button>');
                }
            } else if (unavailable) {
                reason.textContent = t('desktop.noisemaker_local_' + ((local && local.state) || 'starting'));
            } else if (quotaHit) {
                reason.textContent = t('desktop.noisemaker_create_disabled_quota', { used, max });
            } else if (!hasInput) {
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

        function setForm(patch) {
            Object.assign(form, patch || {});
            applyFormToInputs();
            syncPresets();
            syncCounters();
            syncCreateButton();
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

        async function enhance(kind, button) {
            const apiKind = kind === 'random' ? 'idea' : kind;
            const fieldFor = { idea: 'idea', random: 'idea', style: 'style', lyrics: 'lyrics', title: 'title' };
            const field = fieldFor[kind];
            if (!field) return;
            const currentValue = kind === 'random' ? '' : String(form[field] || '');
            if (kind === 'style' && !currentValue.trim()) return;
            button.classList.add('is-busy');
            button.disabled = true;
            try {
                const data = await request('/api/desktop/noisemaker/enhance', {
                    method: 'POST',
                    body: { kind: apiKind, text: currentValue, context: enhanceContextFor(apiKind), lang }
                });
                if (disposed) return;
                if (data && data.text) {
                    setForm({ [field]: data.text });
                    if (field === 'lyrics') {
                        const wrap = qs('[data-nm-lyrics-wrap]');
                        if (wrap) wrap.open = true;
                    }
                    emit('change', getForm());
                }
            } catch (err) {
                if (disposed) return;
                notify((err && err.message) || t('desktop.noisemaker_enhance_failed'));
            } finally {
                button.classList.remove('is-busy');
                button.disabled = false;
            }
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
                params.duration_seconds = Number(form.duration_seconds) || 120;
                if (String(form.bpm) !== '') params.bpm = Number(form.bpm);
                params.vocal_language = String(form.vocal_language || '').trim();
                if (String(form.seed) !== '') params.seed = Number(form.seed);
            }
            return params;
        }

        function submit() {
            if (generation.active) return;
            for (const input of root.querySelectorAll('.nm-local-controls input')) {
                if (!input.reportValidity()) return;
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
            form[input.dataset.nmField] = input.type === 'checkbox' ? input.checked : input.value;
            syncCounters();
            syncPresets();
            syncCreateButton();
            emit('change', getForm());
        });

        root.addEventListener('change', event => {
            const input = event.target.closest('[data-nm-field]');
            if (!input || input.type !== 'checkbox') return;
            form[input.dataset.nmField] = input.checked;
            syncCreateButton();
            emit('change', getForm());
        });

        root.addEventListener('click', event => {
            const modeBtn = event.target.closest('[data-nm-mode]');
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
            const ai = event.target.closest('[data-nm-enhance]');
            if (ai) { enhance(ai.dataset.nmEnhance, ai); return; }
            if (event.target.closest('[data-nm-create-btn]') || event.target.closest('[data-nm-retry]')) { submit(); return; }
            if (event.target.closest('[data-nm-result-new]')) { emit('new-song'); return; }
            if (event.target.closest('[data-nm-result-library]')) { if (generation.result) emit('show-in-library', generation.result); return; }
            if (event.target.closest('[data-nm-result-play]')) { if (generation.result) emit('play-result', generation.result); }
        });

        root.addEventListener('keydown', event => {
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
            setForm, getForm, setGeneration, focusIdea,
            on, dispose
        };
    }

    window.NoisemakerCreate = { create, PRESETS };
})();
