// Storymaker app music panel: mode, mood and prompt, a file or a Noisemaker track, the volume and the state of the music job.
// Generated music costs money and its key changes at every pause in typing, so it starts only from the button that shows the mood
// and the prompt. A file or a track costs nothing, so one job per source starts without a click. Classic script, publishes
// S.createMusicPanel.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};
    const ACTIVE = ['queued', 'running'];
    const DEFAULT_VOLUME = 0.25;
    const MIN_MUSIC_MS = 1000;
    const MODE_KEYS = { none: 'storymaker.music.mode_none', generated: 'storymaker.music.mode_generated', file: 'storymaker.music.mode_file' };
    // MusicMoods of internal/storymaker/music.go, each with a literal key so that the guard test sees it.
    const MOODS = [
        ['upbeat-corporate', 'storymaker.music.mood_upbeat_corporate'],
        ['calm-ambient', 'storymaker.music.mood_calm_ambient'],
        ['inspiring', 'storymaker.music.mood_inspiring'],
        ['playful', 'storymaker.music.mood_playful'],
        ['tech-pulse', 'storymaker.music.mood_tech_pulse'],
        ['dramatic', 'storymaker.music.mood_dramatic'],
        ['lo-fi', 'storymaker.music.mood_lo_fi'],
        ['acoustic', 'storymaker.music.mood_acoustic']
    ];

    function option(value, label) {
        return S.el('option', { value, text: label });
    }

    // inst: {t, bus, api, story, status, report, audioInfo, readonly, dirty, projectId, domId(name), notify(text, kind), editStory(fn, options), flushEdits(), refreshAudio()}
    function createMusicPanel(inst) {
        const { el, icon } = S;
        const t = inst.t;
        let job = null;
        let requestedFor = '';     // the file or track source that already got its one job
        let starting = false;
        let cancelling = false;
        let startError = '';
        let startIssue = '';
        let startDetail = '';      // the server's own reason for a refused start, shown under startError
        let answerDue = false;     // a finished job asked for a fresh answer, so the old one says nothing yet
        let tracksAsked = false;
        let tracks = [];
        let modesKey = '';
        let tracksKey = '';
        let disposed = false;

        function field(labelKey, id, control) {
            return el('div', { class: 'stm-field' }, el('label', { class: 'stm-label', for: id, text: t(labelKey) }), control);
        }

        const modeSelect = el('select', { class: 'stm-select', id: inst.domId('music-mode'), dataset: { stm: 'music-mode' } });
        const moodSelect = el('select', { class: 'stm-select', id: inst.domId('music-mood'), dataset: { stm: 'music-mood' } },
            option('', t('storymaker.music.mood_none')), MOODS.map(([name, key]) => option(name, t(key))));
        const promptBox = el('textarea', { class: 'stm-textarea', id: inst.domId('music-prompt'), rows: '3', dataset: { stm: 'music-prompt' }, placeholder: t('storymaker.music.prompt_placeholder') });
        const moodField = field('storymaker.music.mood', inst.domId('music-mood'), moodSelect);
        const promptField = field('storymaker.music.prompt', inst.domId('music-prompt'), promptBox);
        const notConfigured = el('p', { class: 'stm-hint', dataset: { stm: 'music-not-configured' }, text: t('storymaker.music.not_configured') });
        const summary = el('p', { class: 'stm-hint', dataset: { stm: 'music-summary' } });
        const tooLong = el('p', { class: 'stm-hint stm-music-bad', hidden: true, dataset: { stm: 'music-too-long' } });
        const generateButton = el('button', { type: 'button', class: 'stm-btn stm-btn-primary', dataset: { stm: 'music-generate' }, onClick: () => start(true) }, icon('sparkle'), el('span', { text: t('storymaker.music.generate') }));
        const chooseLabel = el('span', { text: t('storymaker.music.choose_file') });
        const chooseButton = el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'music-choose' }, onClick: () => pickFile() }, icon('folder'), chooseLabel);
        const fileName = el('code', { class: 'stm-path', dataset: { stm: 'music-file' } });
        const fileField = el('div', { class: 'stm-field' }, fileName, chooseButton);
        const trackSelect = el('select', { class: 'stm-select', id: inst.domId('music-track'), dataset: { stm: 'music-track' } });
        const trackField = field('storymaker.music.track', inst.domId('music-track'), trackSelect);
        const volumeInput = el('input', { class: 'stm-input', type: 'range', min: '0', max: '1', step: '0.05', id: inst.domId('music-volume'), dataset: { stm: 'music-volume' } });
        const volumeField = field('storymaker.music.volume', inst.domId('music-volume'), volumeInput);
        const status = el('div', { class: 'stm-music-status', role: 'status', dataset: { stm: 'music-status' } });
        const cancelButton = el('button', { type: 'button', class: 'stm-btn', hidden: true, dataset: { stm: 'music-cancel' }, onClick: () => cancel() }, icon('stop'), el('span', { text: t('storymaker.music.cancel') }));
        const root = el('section', { class: 'stm-music', ariaLabel: t('storymaker.side_music'), dataset: { stm: 'music' } },
            field('storymaker.music.mode', inst.domId('music-mode'), modeSelect), notConfigured, moodField, promptField, summary, tooLong, generateButton, fileField, trackField, volumeField, status, cancelButton);

        function music() {
            const value = inst.story && inst.story.music;
            return value && typeof value === 'object' ? value : { mode: 'none' };
        }
        function isActive() { return !!job && ACTIVE.indexOf(job.status) >= 0; }
        function volumeOf(m) { return typeof m.volume === 'number' ? m.volume : DEFAULT_VOLUME; }
        function sourceOf(m) { return m.track ? 'track|' + m.track : m.file ? 'file|' + m.file : ''; }
        function durationMs() {
            const ms = inst.report ? Number(inst.report.duration_ms) : 0;
            return ms > 0 ? Math.max(MIN_MUSIC_MS, Math.round(ms)) : 0;
        }
        // The /music route refuses a generation for a film over max_duration_seconds. This answers that limit for such a film, else 0.
        function overLimit() {
            const max = Number(inst.status && inst.status.limits && inst.status.limits.max_duration_seconds) || 0;
            return max > 0 && inst.report && inst.report.duration_ms / 1000 > max ? max : 0;
        }
        function moodLabel(name) {
            const hit = MOODS.find(item => item[0] === name);
            return hit ? t(hit[1]) : name ? String(name) : t('storymaker.music.mood_none');
        }
        function refreshAnswer() {
            answerDue = true;
            if (typeof inst.refreshAudio === 'function') inst.refreshAudio();
        }

        // story.music belongs to the story, not to a scene, so it goes through editStory, which refuses read-only itself.
        function editMusic(fn, immediate) {
            inst.editStory(draft => {
                const next = draft.music && typeof draft.music === 'object' ? draft.music : { mode: 'none' };
                fn(next);
                draft.music = next;
            }, { immediate: !!immediate });
        }

        async function pickFile() {
            if (inst.readonly) return;
            const dialogs = window.AuraDesktopFileDialogs;
            if (!dialogs || typeof dialogs.open !== 'function') return;
            const m = music();
            const options = { title: t('storymaker.music.choose_file'), filters: [{ label: t('storymaker.music.file_filter'), extensions: ['.wav', '.mp3'] }] };
            if (m.file) options.initialFile = m.file;
            const result = await dialogs.open(options);
            if (disposed || !result || result.canceled || !result.path) return;
            startError = '';
            startIssue = '';
            editMusic(next => { next.file = String(result.path); delete next.track; }, true);
        }

        async function loadTracks() {
            if (tracksAsked) return;
            tracksAsked = true;
            try {
                const body = await inst.api.noisemakerTracks();
                tracks = body && Array.isArray(body.items) ? body.items : [];
            } catch (err) {
                if (err && err.aborted) return;
                tracks = [];
            }
            paint();
        }

        // generate is true only from the Generate button, the one call that may start a paid generation.
        async function start(generate) {
            if (starting || inst.readonly || disposed) return;
            const ms = durationMs();
            if (!ms) return;
            starting = true;
            startError = '';
            startIssue = '';
            paint();
            try {
                if (!(await inst.flushEdits())) return;
                const answer = await inst.api.music(inst.projectId, ms, generate);
                if (disposed) return;
                if (answer && answer.job) applyJob(answer.job, true);
                else refreshAnswer();
            } catch (err) {
                if (err && err.aborted) return;
                // The music budget shares its code with the narration's character limit, so this panel has its own sentence.
                const refusal = S.errors.normalize(err);
                startError = refusal.code === 'budget_exceeded' ? t('storymaker.music.budget_exceeded') : S.errors.describe(t, err);
                // The server's own reason, as a job failure shows it: the sentence for a refused file cannot say why this file was refused.
                startDetail = !refusal.network && refusal.message && startError.indexOf(refusal.message) < 0 ? refusal.message : '';
                startIssue = err && err.data && err.data.music_issue ? String(err.data.music_issue) : '';
            } finally {
                starting = false;
                paint();
            }
        }

        // The stop control of a queued or running job, on the existing cancel route.
        async function cancel() {
            if (!isActive() || inst.readonly || cancelling) return;
            cancelling = true;
            paint();
            try {
                applyJob(await inst.api.cancelJob(job.id), true);
            } catch (err) {
                if (!(err && err.aborted)) inst.notify(S.errors.describe(t, err), 'error');
            } finally {
                cancelling = false;
                paint();
            }
        }

        // A job event: an old finished job of an earlier session says nothing, a started or running one is taken up.
        function applyJob(next, mine) {
            if (!next || typeof next !== 'object' || next.kind !== 'music') return;
            if (next.project_id && inst.projectId && next.project_id !== inst.projectId) return;
            const same = !!job && job.id === next.id;
            if (!same && !mine && ACTIVE.indexOf(next.status) < 0) return;
            const was = same ? job.status : '';
            job = next;
            if (ACTIVE.indexOf(next.status) < 0 && was && was !== next.status) refreshAnswer();
            paint();
        }

        // One job per source, without a click: a file or a track costs nothing.
        function maybeAutoStart(m) {
            if (m.mode !== 'file' || inst.readonly || inst.dirty || starting || isActive()) return;
            const source = sourceOf(m);
            const answer = inst.audioInfo || null;
            if (!source || source === requestedFor || !answer || answer.music || answer.music_issue || volumeOf(m) <= 0 || !durationMs()) return;
            requestedFor = source;
            start();
        }

        function paintModes(configured) {
            const offered = configured ? ['none', 'generated', 'file'] : ['none', 'file'];
            const key = offered.join();
            if (key === modesKey) return;
            modesKey = key;
            S.clear(modeSelect);
            offered.forEach(name => modeSelect.appendChild(option(name, t(MODE_KEYS[name]))));
        }

        function paintTracks(m) {
            const wanted = String(m.track || '');
            const key = tracks.map(item => item.id).join() + '|' + wanted;
            if (key !== tracksKey) {
                tracksKey = key;
                S.clear(trackSelect);
                trackSelect.appendChild(option('', t('storymaker.music.track_none')));
                tracks.forEach(item => {
                    const id = String(item.id);
                    trackSelect.appendChild(option(id, String(item.title || id).slice(0, 80)));
                });
                if (wanted && !tracks.some(item => String(item.id) === wanted)) trackSelect.appendChild(option(wanted, wanted));
            }
            trackSelect.value = wanted;
        }

        function jobFailureText() {
            if (job.status === 'interrupted') return t('storymaker.music.interrupted');
            if (job.status === 'cancelled') return t('storymaker.music.cancelled');
            if (job.error && job.error.code === 'budget_exceeded') return t('storymaker.music.budget_exceeded');
            return S.errors.describeJob(t, job.error);
        }

        function retryButton() {
            return el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'music-retry' }, disabled: inst.readonly || null, onClick: () => { job = null; requestedFor = ''; startError = ''; paint(); } }, icon('refresh'), el('span', { text: t('storymaker.retry') }));
        }

        function paintStatus(m, answer, issue) {
            S.clear(status);
            if (isActive()) {
                const fraction = Number.isFinite(job.progress) ? S.clamp(job.progress, 0, 1) : 0;
                status.appendChild(el('p', { class: 'stm-hint', text: t('storymaker.music.running') }));
                status.appendChild(el('progress', { class: 'stm-progress', max: '1', value: String(fraction), ariaLabel: t('storymaker.music.running') }));
                return;
            }
            if (m.mode === 'none') return;
            // A failed first read must not look like an empty answer: the reason and a way to read again.
            if (!answer && inst.audioError) {
                status.appendChild(el('p', { class: 'stm-hint stm-music-bad', dataset: { stm: 'music-read-error' }, text: S.errors.describe(t, inst.audioError) }));
                status.appendChild(el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'music-read-retry' }, onClick: () => inst.refreshAudio() }, el('span', { text: t('storymaker.retry') })));
                return;
            }
            if (issue === 'source_missing' && m.mode === 'file') {
                status.appendChild(el('p', { class: 'stm-hint stm-music-bad', dataset: { stm: 'music-missing' }, text: t('storymaker.music.source_missing', { source: String(m.file || m.track || '') }) }));
                return;
            }
            if (job && job.status !== 'succeeded') {
                status.appendChild(el('p', { class: 'stm-hint stm-music-bad', text: jobFailureText() }));
                if (job.status === 'failed' && job.error && job.error.message) status.appendChild(el('code', { class: 'stm-cap-issue', text: String(job.error.message).slice(0, 600) }));
                if (m.mode === 'file') status.appendChild(retryButton());
                return;
            }
            if (startError) {
                status.appendChild(el('p', { class: 'stm-hint stm-music-bad', text: startError }));
                if (startDetail) status.appendChild(el('code', { class: 'stm-cap-issue', dataset: { stm: 'music-refusal-detail' }, text: startDetail.slice(0, 600) }));
                if (m.mode === 'file') status.appendChild(retryButton());
                return;
            }
            // A file replaced on disk has a new key: the answer names no music and no issue, and this source already had its one job, so only a click starts another.
            if (m.mode === 'file' && requestedFor && requestedFor === sourceOf(m) && !starting && !answerDue && answer && !answer.music && !issue && volumeOf(m) > 0) {
                status.appendChild(el('p', { class: 'stm-hint', dataset: { stm: 'music-unprepared' }, text: t('storymaker.export.missing_music') }));
                status.appendChild(retryButton());
                return;
            }
            if (answer && answer.music) status.appendChild(el('p', { class: 'stm-hint', text: t('storymaker.music.ready', { seconds: S.format.seconds(answer.music.duration_ms / 1000) }) }));
        }

        function paint() {
            if (disposed) return;
            const m = music();
            const configured = !!(inst.status && inst.status.music_configured);
            const answer = inst.audioInfo || null;
            const generated = m.mode === 'generated';
            const fromFile = m.mode === 'file';
            const issue = startIssue || (answer && answer.music_issue) || '';
            paintModes(configured);
            modeSelect.value = m.mode || 'none';
            modeSelect.disabled = inst.readonly;
            moodSelect.value = m.mood || '';
            moodSelect.disabled = inst.readonly;
            if (document.activeElement !== promptBox) promptBox.value = m.prompt || '';
            promptBox.disabled = inst.readonly;
            if (document.activeElement !== volumeInput) volumeInput.value = String(volumeOf(m));
            volumeInput.disabled = inst.readonly;
            notConfigured.hidden = !generated || configured;
            moodField.hidden = !generated || !configured;
            promptField.hidden = !generated || !configured;
            const offer = generated && configured && !!answer && !answer.music && volumeOf(m) > 0 && !isActive();
            summary.hidden = !offer;
            summary.textContent = t('storymaker.music.generate_summary', { mood: moodLabel(m.mood), prompt: m.prompt || '-' });
            const over = overLimit();
            tooLong.hidden = !offer || !over;
            tooLong.textContent = over ? t('storymaker.export.blocked_too_long', { max: S.format.seconds(over) }) : '';
            generateButton.hidden = !offer;
            generateButton.disabled = inst.readonly || starting || !durationMs() || over > 0;
            fileField.hidden = !fromFile;
            fileName.hidden = !m.file;
            fileName.textContent = String(m.file || '');
            chooseButton.disabled = inst.readonly;
            chooseLabel.textContent = issue === 'source_missing' ? t('storymaker.music.choose_other') : t('storymaker.music.choose_file');
            trackField.hidden = !fromFile;
            trackSelect.disabled = inst.readonly;
            volumeField.hidden = m.mode === 'none' || !m.mode;
            if (fromFile) {
                loadTracks();
                paintTracks(m);
            }
            cancelButton.hidden = !isActive() || inst.readonly;
            cancelButton.disabled = cancelling;
            paintStatus(m, answer, issue);
            maybeAutoStart(m);
        }

        modeSelect.addEventListener('change', () => editMusic(next => {
            next.mode = modeSelect.value;
            if (next.mode === 'generated' && !next.mood && !next.prompt) next.mood = MOODS[0][0];
        }, true));
        moodSelect.addEventListener('change', () => editMusic(next => {
            if (moodSelect.value) next.mood = moodSelect.value; else delete next.mood;
        }, true));
        promptBox.addEventListener('input', () => editMusic(next => {
            if (promptBox.value) next.prompt = promptBox.value; else delete next.prompt;
        }, false));
        trackSelect.addEventListener('change', () => editMusic(next => {
            if (trackSelect.value) { next.track = trackSelect.value; delete next.file; } else delete next.track;
        }, true));
        volumeInput.addEventListener('change', () => editMusic(next => { next.volume = S.clamp(volumeInput.value, 0, 1); }, true));

        const off = [
            inst.bus.on('story', () => paint()),
            inst.bus.on('report', () => paint()),
            inst.bus.on('status', () => paint()),
            inst.bus.on('readonly', () => paint()),
            inst.bus.on('audio', () => { startIssue = ''; answerDue = false; paint(); }),
            inst.bus.on('job', next => applyJob(next, false))
        ];
        paint();

        return {
            el: root,
            paint,
            dispose() {
                disposed = true;
                off.forEach(fn => fn());
            }
        };
    }

    S.createMusicPanel = createMusicPanel;
})();
