// Storymaker app voice panel: the side tab that gives a story its voice. It lists the providers of GET /voices, preselects the
// active one for a story with voice null, and offers a voice select or a free-text id, a language select limited to the
// provider's languages (a free-text field for a provider with "*"), the "not tested" mark and the facts of the provider's note.
// One button writes {provider, voice, language} into the story. The audition plays the first stored piece of the current
// /narration answer and never synthesises, so no paid call starts here. Server texts are set as text only. Classic script, publishes
// S.createVoicePanel. The narration half shows what still needs audio and the progress of the narrate job, starts it with Generate
// (the only paid path, and it runs in the job loop with its caps), stops it with Stop, switches the burned captions and owns the "no audio" marker.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};
    const VOICE_ID = /^[A-Za-z0-9._-]{0,128}$/;

    function languageBase(tag) {
        return String(tag || '').trim().toLowerCase().split(/[-_]/)[0];
    }

    function providerId(id) {
        return String(id || '').trim().toLowerCase();
    }

    function desktopLanguage() {
        const lang = window.SYSTEM_LANG || (document.documentElement && document.documentElement.lang) || 'en';
        return String(lang).trim() || 'en';
    }

    function languagesOf(provider) {
        return provider && Array.isArray(provider.languages) ? provider.languages : [];
    }

    // "*" means not restricted, and an empty list restricts nothing either: the field takes free text.
    function languageIsFree(provider) {
        const list = languagesOf(provider);
        return !list.length || list.indexOf('*') >= 0;
    }

    function isTested(provider, language) {
        const tested = provider && Array.isArray(provider.tested_languages) ? provider.tested_languages : [];
        return tested.map(code => languageBase(code)).indexOf(languageBase(language)) >= 0;
    }

    // A Phonon voice that is not installed fails its job, because nothing downloads it on the way (phonon.VoicePath). The panel lists it only while
    // the draft holds it, the rule of the Settings select (ui/cfg/tts.js:421). Settings offers every Piper voice, so that flag decides nothing here.
    function voiceMissing(provider, item) {
        return !!provider && !!item && provider.id === 'phonon' && item.installed === false;
    }

    function voicesFor(provider, language, current) {
        const list = (provider && Array.isArray(provider.voices) ? provider.voices : []).filter(item => !voiceMissing(provider, item) || item.id === current);
        const base = languageBase(language);
        const fitting = list.filter(item => !item.language || languageBase(item.language) === base);
        return fitting.length ? fitting : list;
    }

    // The first piece of the first block, in the story's scene order: the answer's map has no order of its own.
    function firstPiece(story, answer) {
        const narration = answer && answer.narration ? answer.narration : {};
        for (const scene of S.model.scenesOf(story)) {
            const entry = scene ? narration[scene.id] : null;
            for (const block of entry && Array.isArray(entry.blocks) ? entry.blocks : []) {
                if (block && Array.isArray(block.pieces) && block.pieces.length) return block.pieces[0];
            }
        }
        return null;
    }

    const ACTIVE = ['queued', 'running'];

    function jobIsActive(job) {
        return !!job && ACTIVE.indexOf(job.status) >= 0;
    }

    // The narration half of the panel. Job events of kind narrate land here; the export popover takes kind render only.
    // voiceDiffers() says the panel shows a voice the story does not hold. Generate runs the stored voice, so it waits until that voice is written.
    function createNarrationStatus(inst, voiceDiffers) {
        const { el, icon } = S;
        const t = inst.t;
        let job = null;
        let starting = false;
        let refusal = '';
        let cancelling = false;

        const summary = el('p', { class: 'stm-hint', dataset: { stm: 'narration-summary' } });
        const meter = el('progress', { class: 'stm-progress', max: '1', value: '0', hidden: true, ariaLabel: t('storymaker.voice.status_title'), dataset: { stm: 'narration-progress' } });
        const pieces = el('p', { class: 'stm-hint', hidden: true, dataset: { stm: 'narration-pieces' } });
        const jobLine = el('p', { role: 'status', hidden: true, dataset: { stm: 'narration-job' } });
        const jobDetail = el('code', { class: 'stm-cap-issue', hidden: true, dataset: { stm: 'narration-job-detail' } });
        const refusalLine = el('p', { class: 'stm-voice-error', role: 'alert', hidden: true, dataset: { stm: 'narration-refusal' } });
        const voiceHint = el('p', { class: 'stm-hint', hidden: true, dataset: { stm: 'narration-voice-hint' }, text: t('storymaker.voice.apply_first') });
        const generateLabel = el('span', { text: t('storymaker.voice.generate') });
        const generate = el('button', { type: 'button', class: 'stm-btn stm-btn-primary', dataset: { stm: 'narration-generate' }, onClick: () => start() }, icon('sparkle'), generateLabel);
        const cancelButton = el('button', { type: 'button', class: 'stm-btn', hidden: true, dataset: { stm: 'narration-cancel' }, onClick: () => cancel() }, icon('stop'), el('span', { text: t('storymaker.voice.cancel') }));
        const captions = el('input', { type: 'checkbox', id: inst.domId('captions'), dataset: { stm: 'captions-toggle' }, onChange: () => toggleCaptions() });
        const marker = el('div', { class: 'stm-no-audio', role: 'status', hidden: true, dataset: { stm: 'no-audio' } }, icon('alert'), el('span', { text: t('storymaker.voice.no_audio') }));
        const readRetry = el('button', { type: 'button', class: 'stm-btn', hidden: true, dataset: { stm: 'narration-read-retry' }, onClick: () => inst.refreshAudio() }, el('span', { text: t('storymaker.retry') }));
        const root = el('div', { class: 'stm-voice-status', dataset: { stm: 'narration-status' } },
            el('span', { class: 'stm-label', text: t('storymaker.voice.status_title') }),
            summary, meter, pieces, jobLine, jobDetail, refusalLine, voiceHint,
            el('div', { class: 'stm-actions' }, generate, cancelButton, readRetry),
            el('label', { class: 'stm-voice-check', for: inst.domId('captions') }, captions, el('span', { text: t('storymaker.voice.captions') })));

        function jobText() {
            if (job.status === 'succeeded') return t('storymaker.voice.job_done');
            if (job.status === 'cancelled') return t('storymaker.voice.job_cancelled');
            if (job.status === 'interrupted') return t('storymaker.voice.job_interrupted');
            if (job.status === 'failed') return S.errors.describeJob(t, job.error);
            if (job.status === 'queued') return t('storymaker.voice.job_queued');
            return t('storymaker.voice.job_running');
        }

        function paintJob() {
            const active = jobIsActive(job);
            const total = job && Number(job.pieces_total) > 0 ? Number(job.pieces_total) : 0;
            const done = job ? Number(job.pieces_done) || 0 : 0;
            meter.hidden = !active;
            if (active) meter.value = total ? S.clamp(done / total, 0, 1) : S.clamp(job.progress, 0, 1);
            pieces.hidden = !active || !total;
            if (active && total) pieces.textContent = t('storymaker.voice.pieces', { done, total });
            jobLine.hidden = !job;
            jobLine.textContent = job ? jobText() : '';
            const detail = job && job.status === 'failed' && job.error && job.error.message ? String(job.error.message).slice(0, 600) : '';
            jobDetail.hidden = !detail;
            jobDetail.textContent = detail;
        }

        function paint() {
            const answer = inst.audioInfo;
            const chars = answer && answer.chars ? answer.chars : { total: 0, missing: 0 };
            const missing = answer && Array.isArray(answer.missing) ? answer.missing : [];
            // A failed first read must not look like loading forever: the reason and a way to read again.
            const readFailed = !answer && !!inst.audioError;
            readRetry.hidden = !readFailed;
            if (readFailed) summary.textContent = S.errors.describe(t, inst.audioError);
            else if (!answer) summary.textContent = t('storymaker.loading');
            else if (missing.length) summary.textContent = t('storymaker.voice.missing', { blocks: missing.length, chars: chars.missing, total: chars.total });
            else if (chars.total > 0) summary.textContent = t('storymaker.voice.complete', { total: chars.total });
            else summary.textContent = t('storymaker.voice.no_text');
            paintJob();
            refusalLine.textContent = refusal;
            refusalLine.hidden = !refusal;
            generateLabel.textContent = t(starting ? 'storymaker.voice.generating' : 'storymaker.voice.generate');
            const differs = voiceDiffers();
            voiceHint.hidden = !differs;
            generate.disabled = inst.readonly || starting || jobIsActive(job) || !missing.length || differs;
            cancelButton.hidden = inst.readonly || !jobIsActive(job);
            cancelButton.disabled = cancelling;
            const style = inst.story && inst.story.style;
            captions.checked = !style || style.captions !== false;
            captions.disabled = inst.readonly;
            const scene = S.model.scenesOf(inst.story)[inst.selected];
            marker.hidden = !scene || !missing.some(item => item.scene_id === scene.id);
        }

        // take(next, adopt): without a tracked job only an active one is taken up, because the event stream replays old jobs on every open.
        function take(next, adopt) {
            if (!next || typeof next !== 'object' || next.kind !== 'narrate') return;
            if (next.project_id && inst.projectId && next.project_id !== inst.projectId) return;
            if (job && job.id !== next.id && (jobIsActive(job) || !jobIsActive(next))) return;
            if (!job && !adopt && !jobIsActive(next)) return;
            job = next;
            paint();
        }

        function refusalText(err) {
            const e = S.errors.normalize(err);
            const data = e.data || {};
            const parts = [S.errors.describe(t, e)];
            if (Array.isArray(data.alternatives) && data.alternatives.length) parts.push(t('storymaker.voice.alternatives', { names: data.alternatives.join(', ') }));
            if (e.code === 'budget_exceeded' && data.limit) parts.push(t('storymaker.voice.budget_detail', { limit: data.limit, missing: data.missing }));
            return parts.join(' ');
        }

        async function start() {
            if (starting || inst.readonly || jobIsActive(job) || voiceDiffers()) return;
            starting = true;
            refusal = '';
            paint();
            try {
                if (!(await inst.flushEdits())) return;
                const answer = await inst.api.narrate(inst.projectId);
                if (answer && answer.job) {
                    // An event of this job may already have arrived and be newer than this answer.
                    if (!job || job.id !== answer.job.id) { job = null; take(answer.job, true); }
                } else {
                    job = null;
                    inst.refreshAudio();
                }
            } catch (err) {
                if (err && err.aborted) return;
                refusal = refusalText(err);
            } finally {
                starting = false;
                paint();
            }
        }

        async function cancel() {
            if (inst.readonly || cancelling || !jobIsActive(job)) return;
            cancelling = true;
            paint();
            try {
                take(await inst.api.cancelJob(job.id), false);
            } catch (err) {
                if (err && err.aborted) return;
                inst.notify(S.errors.describe(t, err), 'error');
            } finally {
                cancelling = false;
                paint();
            }
        }

        function toggleCaptions() {
            const on = captions.checked;
            inst.editStory(story => { story.style = Object.assign({}, story.style, { captions: on }); }, { immediate: true });
        }

        const off = [
            inst.bus.on('job', next => take(next, false)),
            inst.bus.on('audio', paint),
            inst.bus.on('story', paint),
            inst.bus.on('selection', paint),
            inst.bus.on('readonly', paint)
        ];
        paint();
        return { el: root, marker, paint, dispose() { off.forEach(fn => fn()); } };
    }

    // inst: {t, bus, api, story, selected, projectId, readonly, audioInfo, domId(suffix), notify(text, kind), editStory(fn, options), flushEdits(), refreshAudio()}
    function createVoicePanel(inst) {
        const { el, icon } = S;
        const t = inst.t;
        let providers = null;
        let loadError = '';
        let draft = { provider: '', voice: '', language: '' };
        let savedText = '';
        let replacedLanguage = '';
        let disposed = false;

        function field(labelKey, control, stm) {
            return el('div', { class: 'stm-field', dataset: { stm } },
                el('label', { class: 'stm-label', for: control.id, text: t(labelKey) }), control);
        }

        const providerSelect = el('select', { class: 'stm-select', id: inst.domId('voice-provider'), dataset: { stm: 'voice-provider' }, onChange: () => changeProvider() });
        const voiceSelect = el('select', { class: 'stm-select', id: inst.domId('voice-select'), dataset: { stm: 'voice-select' }, onChange: () => { draft.voice = voiceSelect.value; paintNote(); sync(); } });
        const voiceInput = el('input', { class: 'stm-input', type: 'text', id: inst.domId('voice-free'), maxlength: '128', autocomplete: 'off', spellcheck: 'false', dataset: { stm: 'voice-free' }, onInput: () => { draft.voice = voiceInput.value.trim(); sync(); } });
        const languageSelect = el('select', { class: 'stm-select', id: inst.domId('voice-language'), dataset: { stm: 'voice-language' }, onChange: () => { draft.language = languageSelect.value; replacedLanguage = ''; fitDraft(); fillLanguage(); fillVoice(); sync(); } });
        const languageInput = el('input', { class: 'stm-input', type: 'text', id: inst.domId('voice-language-free'), maxlength: '32', autocomplete: 'off', spellcheck: 'false', dataset: { stm: 'voice-language-free' }, onInput: () => { draft.language = languageInput.value.trim(); fitDraft(); fillVoice(); sync(); } });
        const voiceSelectField = field('storymaker.voice.voice', voiceSelect, 'voice-select-field');
        const voiceInputField = field('storymaker.voice.voice_free', voiceInput, 'voice-free-field');
        const languageSelectField = field('storymaker.voice.language', languageSelect, 'voice-language-field');
        const languageInputField = field('storymaker.voice.language_free', languageInput, 'voice-language-free-field');
        const notTested = el('span', { class: 'stm-badge', hidden: true, dataset: { stm: 'voice-not-tested' }, text: t('storymaker.voice.not_tested') });
        const note = el('p', { class: 'stm-hint', hidden: true, dataset: { stm: 'voice-note' } });
        const apply = el('button', { type: 'button', class: 'stm-btn stm-btn-primary', dataset: { stm: 'voice-apply' }, onClick: () => writeVoice() }, icon('check'), el('span', { text: t('storymaker.voice.apply') }));
        const audition = el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'voice-audition' }, onClick: () => playSample() }, icon('play'), el('span', { text: t('storymaker.voice.audition') }));
        const player = el('audio', { preload: 'none', dataset: { stm: 'voice-audio' } });
        const form = el('div', { class: 'stm-voice-form', hidden: true, dataset: { stm: 'voice-form' } },
            field('storymaker.voice.provider', providerSelect, 'voice-provider-field'),
            languageSelectField, languageInputField, voiceSelectField, voiceInputField,
            el('div', { class: 'stm-actions' }, notTested),
            note,
            el('div', { class: 'stm-actions' }, apply, audition));
        const none = el('p', { class: 'stm-voice-none', role: 'status', hidden: true, dataset: { stm: 'voice-none' }, text: t('storymaker.voice.none_ready') });
        const errorLine = el('p', { class: 'stm-voice-error', role: 'alert', hidden: true, dataset: { stm: 'voice-error' } });
        const retry = el('button', { type: 'button', class: 'stm-btn', hidden: true, dataset: { stm: 'voice-retry' }, onClick: () => load() }, icon('refresh'), el('span', { text: t('storymaker.retry') }));
        const loading = el('p', { class: 'stm-muted', role: 'status', dataset: { stm: 'voice-loading' }, text: t('storymaker.loading') });
        const status = createNarrationStatus(inst, draftDiffers);
        const root = el('section', { class: 'stm-voice', ariaLabel: t('storymaker.voice.title'), dataset: { stm: 'voice' } }, loading, none, errorLine, retry, form, player, status.el);

        function currentProvider() {
            return (providers || []).find(item => item.id === draft.provider) || null;
        }

        // The draft always holds what the controls can show: a language the provider lists, a voice of its list.
        // A language it has to replace stays in replacedLanguage: the note names it, so a German voice never reads Japanese text unannounced.
        function fitDraft() {
            const provider = currentProvider();
            if (!provider) return;
            const listed = languagesOf(provider);
            if (!languageIsFree(provider) && !listed.some(code => languageBase(code) === languageBase(draft.language))) {
                replacedLanguage = draft.language;
                draft.language = listed[0];
            }
            if (provider.free_text) return;
            const voices = voicesFor(provider, draft.language, draft.voice);
            if (!voices.length) { draft.voice = provider.default_voice || ''; return; }
            if (voices.some(item => item.id === draft.voice)) return;
            draft.voice = voices.some(item => item.id === provider.default_voice) ? provider.default_voice : voices[0].id;
        }

        function startDraft() {
            const saved = inst.story && inst.story.voice;
            savedText = JSON.stringify(saved || null);
            replacedLanguage = '';
            if (saved && saved.provider) {
                draft = { provider: providerId(saved.provider), voice: saved.voice || '', language: saved.language || desktopLanguage() };
            } else {
                const list = providers || [];
                const first = list.find(item => item.active && item.ready) || list.find(item => item.ready) || list[0] || null;
                draft = { provider: first ? first.id : '', voice: first && first.default_voice ? first.default_voice : '', language: desktopLanguage() };
            }
            fitDraft();
        }

        function changeProvider() {
            draft.provider = providerSelect.value;
            const provider = currentProvider();
            draft.voice = provider && provider.default_voice ? provider.default_voice : '';
            // The language asked for gets another chance with this provider. fitDraft takes it out again if this one has no voice for it either.
            if (replacedLanguage) { draft.language = replacedLanguage; replacedLanguage = ''; }
            fitDraft();
            fillAll();
            sync();
        }

        function fillProviders() {
            S.clear(providerSelect);
            (providers || []).forEach(item => providerSelect.appendChild(el('option', {
                value: item.id, disabled: !item.ready || null,
                text: item.ready ? item.id : t('storymaker.voice.provider_not_ready', { name: item.id })
            })));
            if (draft.provider && !currentProvider()) providerSelect.appendChild(el('option', { value: draft.provider, text: t('storymaker.voice.provider_not_ready', { name: draft.provider }) }));
            providerSelect.value = draft.provider;
        }

        function fillLanguage() {
            const provider = currentProvider();
            const free = !provider || languageIsFree(provider);
            languageSelectField.hidden = free;
            languageInputField.hidden = !free;
            S.clear(languageSelect);
            if (free) {
                if (languageInput.value !== draft.language) languageInput.value = draft.language;
                return;
            }
            const base = languageBase(draft.language);
            languagesOf(provider).forEach(code => {
                const value = languageBase(code) === base ? draft.language : code;
                languageSelect.appendChild(el('option', { value, text: value }));
            });
            languageSelect.value = draft.language;
        }

        function paintNote() {
            const provider = currentProvider();
            const voice = provider && Array.isArray(provider.voices) ? provider.voices.find(item => item.id === draft.voice) : null;
            const replaced = replacedLanguage ? t('storymaker.voice.language_replaced', { language: replacedLanguage }) : '';
            const facts = [replaced, provider ? provider.note : '', voice ? voice.note : '', voiceMissing(provider, voice) ? t('storymaker.voice.not_installed') : ''].filter(Boolean).join(' ');
            note.textContent = facts;
            note.hidden = !facts;
        }

        function fillVoice() {
            const provider = currentProvider();
            const free = !!(provider && provider.free_text);
            const voices = provider && !free ? voicesFor(provider, draft.language, draft.voice) : [];
            voiceSelectField.hidden = !voices.length;
            voiceInputField.hidden = !free;
            S.clear(voiceSelect);
            voices.forEach(item => {
                const label = item.name ? item.name + ' (' + item.id + ')' : item.id;
                voiceSelect.appendChild(el('option', { value: item.id, text: voiceMissing(provider, item) ? label + ' - ' + t('storymaker.voice.not_installed') : label }));
            });
            if (voices.length) voiceSelect.value = draft.voice;
            if (free) {
                if (voiceInput.value !== draft.voice) voiceInput.value = draft.voice;
                voiceInput.placeholder = provider.default_voice || '';
            }
            paintNote();
        }

        function fillAll() {
            fillProviders();
            fillLanguage();
            fillVoice();
        }

        function sameAsStory() {
            const saved = inst.story && inst.story.voice;
            return !!saved && providerId(saved.provider) === draft.provider && (saved.voice || '') === draft.voice && (saved.language || '') === draft.language;
        }

        // The controls show a voice the story does not hold. Generate and the audition run the stored voice, so both wait until it is written. There is nothing to
        // write with no form on screen, in read-only mode or with a provider that is not ready, so the stored voice stays usable and the server's refusal says why.
        function draftDiffers() {
            const provider = currentProvider();
            return !inst.readonly && !form.hidden && !!provider && !!provider.ready && !sameAsStory();
        }

        function sync() {
            const provider = currentProvider();
            const loaded = providers !== null;
            const anyReady = (providers || []).some(item => item.ready);
            loading.hidden = loaded;
            errorLine.textContent = loadError;
            errorLine.hidden = !loadError;
            retry.hidden = !loadError;
            none.hidden = !loaded || !!loadError || anyReady;
            form.hidden = !anyReady;
            [providerSelect, voiceSelect, voiceInput, languageSelect, languageInput].forEach(control => { control.disabled = inst.readonly; });
            notTested.hidden = !provider || isTested(provider, draft.language);
            const listed = provider && Array.isArray(provider.voices) ? provider.voices.find(item => item.id === draft.voice) : null;
            const valid = !!provider && !!provider.ready && !!draft.language && VOICE_ID.test(draft.voice) && !voiceMissing(provider, listed);
            apply.disabled = inst.readonly || !valid || sameAsStory();
            const piece = firstPiece(inst.story, inst.audioInfo);
            const differs = draftDiffers();
            audition.disabled = !piece || differs;
            audition.title = !piece ? t('storymaker.voice.audition_none') : differs ? t('storymaker.voice.apply_first') : '';
            status.paint();
        }

        function writeVoice() {
            if (inst.readonly || apply.disabled) return;
            const voice = { provider: draft.provider, voice: draft.voice, language: draft.language };
            inst.editStory(story => { story.voice = voice; }, { immediate: true });
        }

        function playSample() {
            if (audition.disabled) return;
            const piece = firstPiece(inst.story, inst.audioInfo);
            if (!piece) return;
            const url = inst.api.narrationUrl(inst.projectId, piece.hash, piece.ext);
            if (!url) { inst.notify(t('storymaker.voice.audition_failed'), 'error'); return; }
            player.src = url;
            const playing = player.play();
            if (playing && typeof playing.catch === 'function') playing.catch(() => inst.notify(t('storymaker.voice.audition_failed'), 'error'));
        }

        // A story changed elsewhere (the Code tab, a reload) restarts the draft. Anything else keeps the controls, so typing and an open select survive an autosave.
        function paint() {
            if (disposed || providers === null) return;
            const text = JSON.stringify((inst.story && inst.story.voice) || null);
            if (text !== savedText) { startDraft(); fillAll(); }
            sync();
        }

        async function load() {
            loadError = '';
            providers = null;
            sync();
            try {
                const answer = await inst.api.voices();
                if (disposed) return;
                providers = answer && Array.isArray(answer.providers) ? answer.providers : [];
            } catch (err) {
                if (disposed || (err && err.aborted)) return;
                providers = [];
                loadError = S.errors.describe(t, err);
            }
            startDraft();
            fillAll();
            sync();
        }

        const off = [
            inst.bus.on('story', paint),
            inst.bus.on('audio', paint),
            inst.bus.on('readonly', paint)
        ];
        load();
        return {
            el: root,
            marker: status.marker,
            paint() { paint(); status.paint(); },
            dispose() {
                disposed = true;
                status.dispose();
                off.forEach(fn => fn());
                player.pause();
                player.removeAttribute('src');
                player.load();
            }
        };
    }

    S.createVoicePanel = createVoicePanel;
})();
