// Chibi Sensei test step: the run picker (succeeded runs, newest first), three views in a WAI-ARIA tab pattern (report card, evaluation
// examples with filter and pages, chat from P.chat), the model and quick-check notes of the report card, "Try in chat" and the
// once-per-run report-seen call (never from a read-only desktop). All text from the model or the server reaches the page as
// textContent only. The pure helpers are exported for the Node test. Classic script, publishes P.test.
(function () {
    'use strict';

    const P = window.ChibiSenseiParts = window.ChibiSenseiParts || {};
    const VIEWS = ['report', 'samples', 'chat'];
    const PAGE_SIZE = 20;
    let testCounter = 0;

    const VIEW_KEYS = { report: 'chibi_sensei.test.view_report', samples: 'chibi_sensei.test.view_samples', chat: 'chibi_sensei.test.view_chat' };
    const FILTER_KEYS = { all: 'chibi_sensei.test.filter_all', different: 'chibi_sensei.test.filter_different', wrong: 'chibi_sensei.test.filter_wrong' };
    const PRESET_NAME_KEYS = { quick: 'chibi_sensei.training.preset_quick', balanced: 'chibi_sensei.training.preset_balanced', thorough: 'chibi_sensei.training.preset_thorough' };
    const ROLE_KEYS = { system: 'chibi_sensei.editor.role_system', user: 'chibi_sensei.editor.role_user', assistant: 'chibi_sensei.editor.role_assistant' };
    const SMOKE_SKIP_KEYS = {
        runtime_not_installed: 'chibi_sensei.test.smoke_skipped_installed', runtime_busy: 'chibi_sensei.test.smoke_skipped_busy',
        runtime_disabled: 'chibi_sensei.test.smoke_skipped_disabled'
    };
    const TUNED_KEYS = { ready: 'chibi_sensei.test.tuned_ready', missing: 'chibi_sensei.test.tuned_missing', invalid: 'chibi_sensei.test.tuned_invalid' };
    const BASE_KEYS = {
        ready: 'chibi_sensei.test.base_ready', missing: 'chibi_sensei.test.base_missing', skipped: 'chibi_sensei.test.base_skipped',
        invalid: 'chibi_sensei.test.base_invalid'
    };
    const BASE_REASON_KEYS = { missing: BASE_KEYS.missing, skipped: BASE_KEYS.skipped, invalid: BASE_KEYS.invalid };

    // A literal key map answers only for its own names, never for inherited ones such as constructor.
    const own = (map, key) => (Object.prototype.hasOwnProperty.call(map, key) ? map[key] : '');

    // ---- the pure part ---------------------------------------------------------------------------------------------------------

    // The succeeded runs, newest first by the end time (or the start when the end is missing).
    function succeededRuns(runs) {
        const when = r => { const n = Date.parse(r.finished || r.created || ''); return Number.isFinite(n) ? n : 0; };
        return (Array.isArray(runs) ? runs : []).filter(r => r && r.state === 'succeeded').sort((a, b) => when(b) - when(a));
    }

    function timeText(ms) {
        const n = Math.max(0, Number(ms) || 0);
        return n < 1000 ? P.format.number(Math.round(n)) + ' ms' : P.format.decimal(n / 1000, 1) + ' s';
    }

    // smokeLine(smoke) -> {key, vars, tone} or null: the quick check after training as one line.
    function smokeLine(smoke) {
        if (!smoke || typeof smoke !== 'object') return null;
        if (smoke.state === 'ok') return { key: 'chibi_sensei.test.smoke_ok', vars: { time: timeText(smoke.ms) }, tone: '' };
        if (smoke.state === 'failed') return { key: 'chibi_sensei.test.smoke_failed', vars: {}, tone: 'warn' };
        if (smoke.state === 'skipped') return { key: own(SMOKE_SKIP_KEYS, smoke.code) || 'chibi_sensei.test.smoke_skipped', vars: {}, tone: '' };
        return null;
    }

    // modelLine(kind, model) -> {key, vars: {size}, tone}: a ready model says its size; an unknown reason counts as missing.
    function modelLine(kind, model) {
        const keys = kind === 'base' ? BASE_KEYS : TUNED_KEYS;
        const m = model && typeof model === 'object' ? model : {};
        const size = P.format.bytes(m.bytes);
        if (m.available) return { key: keys.ready, vars: { size }, tone: '' };
        const reason = Object.prototype.hasOwnProperty.call(keys, m.reason) && m.reason !== 'ready' ? m.reason : 'missing';
        return { key: keys[reason], vars: { size }, tone: 'warn' };
    }

    // The text key for a base model that is not available; an unknown reason counts as missing.
    function baseReasonKey(reason) {
        return Object.prototype.hasOwnProperty.call(BASE_REASON_KEYS, reason) ? BASE_REASON_KEYS[reason] : BASE_REASON_KEYS.missing;
    }

    // promptOf(sample) -> {system, user}: the system message first, then the last user message; a text prompt is the user message.
    function promptOf(sample) {
        const s = sample || {};
        const list = Array.isArray(s.prompt_messages) ? s.prompt_messages : null;
        if (list && list.length) {
            const system = list[0] && list[0].role === 'system' ? String(list[0].content || '') : '';
            let user = '';
            for (let i = list.length - 1; i >= 0; i--) { if (list[i] && list[i].role === 'user') { user = String(list[i].content || ''); break; } }
            return { system, user };
        }
        return { system: '', user: typeof s.prompt_text === 'string' ? s.prompt_text : '' };
    }

    // ---- the step view ---------------------------------------------------------------------------------------------------------

    function create(inst) {
        const { el, icon } = P;
        const t = inst.t;
        const n = ++testCounter;
        const tabId = view => 'csn-test-' + n + '-tab-' + view;
        const panelId = view => 'csn-test-' + n + '-panel-' + view;
        let selectedId = '', data = null, dataSeq = 0, view = 'report', disposed = false, chatArea = null, pendingFocus = false, loadFailed = false;
        let optionSig = null;
        const seen = new Set();
        const samples = { filter: 'all', offset: 0, page: null, seq: 0, loaded: false, loading: false, missing: false };
        const offs = [];

        // ---- nodes ----
        const runSelect = el('select', { class: 'csn-select', dataset: { csn: 'test-run' } });
        const runField = el('label', { class: 'csn-field', dataset: { csn: 'test-run-field' } }, el('span', { class: 'csn-label', text: t('chibi_sensei.test.run_label') }), runSelect);
        const head = el('div', { class: 'csn-test-head' }, el('h2', { text: t('chibi_sensei.test.title') }), runField);
        const intro = el('p', { class: 'csn-muted', text: t('chibi_sensei.test.intro') });
        const gotoTraining = el('button', { type: 'button', class: 'csn-btn csn-btn-primary', dataset: { csn: 'test-goto-training' }, text: t('chibi_sensei.test.goto_training'), onClick: () => inst.showStep('training') });
        const empty = el('div', { class: 'csn-empty', dataset: { csn: 'test-empty' } },
            P.mascot ? P.mascot.create({ size: 88, mood: 'idle' }).el : null,
            el('h3', { text: t('chibi_sensei.test.empty_title') }), el('p', { text: t('chibi_sensei.test.empty_text') }), gotoTraining);
        const loading = el('p', { class: 'csn-muted', role: 'status', text: t('chibi_sensei.test.loading') });
        runSelect.addEventListener('change', () => { if (runSelect.value && runSelect.value !== selectedId) selectRun(runSelect.value); });
        const retry = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'test-retry' }, text: t('chibi_sensei.retry'), onClick: () => { if (selectedId) selectRun(selectedId); } });
        const errorLine = el('p', { class: 'csn-error', role: 'alert', dataset: { csn: 'test-error' } });

        const tabs = {}, panels = {};
        VIEWS.forEach(name => {
            tabs[name] = el('button', {
                type: 'button', class: 'csn-tab', role: 'tab', id: tabId(name), ariaControls: panelId(name), ariaSelected: 'false', tabindex: '-1',
                dataset: { csn: 'test-view', view: name }, text: t(VIEW_KEYS[name]),
                onClick: () => showView(name, {}),
                onKeydown: event => onTabKey(event, name)
            });
            panels[name] = el('div', { class: 'csn-tabpanel', role: 'tabpanel', id: panelId(name), ariaLabelledby: tabId(name), tabindex: '0', dataset: { csn: 'test-panel', view: name } });
        });
        const tabList = el('div', { class: 'csn-tabs', role: 'tablist', ariaLabel: t('chibi_sensei.test.views_label') }, VIEWS.map(name => tabs[name]));

        const reportBox = el('section', { class: 'csn-training-section', dataset: { csn: 'report-card' } });
        panels.report.appendChild(reportBox);
        const filterSelect = el('select', { class: 'csn-select', ariaLabel: t('chibi_sensei.test.filter_label'), dataset: { csn: 'samples-filter' } });
        filterSelect.addEventListener('change', () => { samples.filter = filterSelect.value; samples.offset = 0; loadSamples(); });
        const samplesHelp = el('p', { class: 'csn-muted', text: t('chibi_sensei.test.samples_help') });
        const samplesNote = el('p', { class: 'csn-muted', role: 'status' });
        const samplesList = el('ol', { class: 'csn-examples' });
        const pagePrev = el('button', { type: 'button', class: 'csn-btn csn-btn-icon', ariaLabel: t('chibi_sensei.table.prev'), title: t('chibi_sensei.table.prev'), onClick: () => { samples.offset = Math.max(0, samples.offset - PAGE_SIZE); loadSamples(); } }, icon('left'));
        const pageNext = el('button', { type: 'button', class: 'csn-btn csn-btn-icon', ariaLabel: t('chibi_sensei.table.next'), title: t('chibi_sensei.table.next'), onClick: () => { samples.offset += PAGE_SIZE; loadSamples(); } }, icon('right'));
        const pageText = el('span', { class: 'csn-muted' });
        const pager = el('div', { class: 'csn-pager', dataset: { csn: 'samples-pager' } }, pagePrev, pageText, pageNext);
        const samplesBox = el('section', { class: 'csn-training-section', dataset: { csn: 'samples' } },
            el('div', { class: 'csn-section-head' }, el('h3', { text: t('chibi_sensei.test.samples_title') }), el('label', { class: 'csn-check-option' }, el('span', { class: 'csn-label', text: t('chibi_sensei.test.filter_label') }), filterSelect)),
            samplesHelp, samplesNote, samplesList, pager);
        panels.samples.appendChild(samplesBox);

        const root = el('section', { class: 'csn-test', dataset: { csn: 'test' } }, head, intro, empty, loading, errorLine, retry, tabList, VIEWS.map(name => panels[name]));
        [errorLine, retry, loading, samplesNote].forEach(node => { node.hidden = true; });

        const setText = (node, text) => { if (node.textContent !== text) node.textContent = text; };
        const showError = text => { setText(errorLine, text || ''); errorLine.hidden = !text; };
        const stale = mine => disposed || mine !== dataSeq;
        const tunedOk = () => !!(data && data.models && data.models.tuned && data.models.tuned.available);

        // ---- the run picker ----
        function paintRuns() {
            const list = succeededRuns(inst.runs);
            const sig = JSON.stringify(list.map(r => [r.id, r.finished || r.created, r.preset]));
            if (sig !== optionSig) {
                optionSig = sig;
                P.clear(runSelect);
                list.forEach(r => {
                    const preset = own(PRESET_NAME_KEYS, r.preset) ? t(own(PRESET_NAME_KEYS, r.preset)) : String(r.preset || '-');
                    runSelect.appendChild(el('option', { value: r.id, text: t('chibi_sensei.test.run_option', { date: P.format.dateTime(r.finished || r.created), preset }) }));
                });
            }
            runField.hidden = list.length < 2;
            empty.hidden = list.length > 0;
            tabList.hidden = !list.length;
            VIEWS.forEach(name => { panels[name].hidden = !list.length || name !== view; });
            intro.hidden = !list.length;
            return list;
        }

        function syncRuns() {
            if (disposed) return;
            const list = paintRuns();
            const next = list.some(r => r.id === selectedId) ? selectedId : (list.length ? list[0].id : '');
            if (next === selectedId) { runSelect.value = selectedId; return; }
            selectRun(next);
        }

        // A new selection drops the chat (which aborts a running stream) and the examples, then loads the test data.
        function selectRun(id) {
            if (chatArea) { const old = chatArea; chatArea = null; old.dispose(); if (old.el.parentNode) old.el.parentNode.removeChild(old.el); }
            selectedId = id;
            runSelect.value = id;
            samples.page = null; samples.offset = 0; samples.filter = 'all'; samples.loaded = false; samples.loading = false; samples.missing = false; samples.seq++;
            data = null; loadFailed = false;
            showError('');
            paintAll();
            if (id) loadData(id);
        }

        async function loadData(id) {
            const mine = ++dataSeq;
            try {
                const body = await inst.api.runTest(id);
                if (stale(mine)) return;
                data = body;
                paintAll();
                maybeSeen();
            } catch (err) {
                if (stale(mine) || (err && err.aborted)) return;
                const e = P.errors.normalize(err);
                if (e.code === 'run_not_succeeded' || e.code === 'run_not_found' || e.status === 404) { inst.reloadRuns(); return; }
                loadFailed = true;
                showError(inst.handleError(err));
                paintAll();
            }
        }

        // ---- the report card ----
        function paintReport() {
            P.clear(reportBox);
            if (!data) return;
            const result = el('div', { class: 'csn-result', dataset: { outcome: 'succeeded' } });
            if (data.report && P.run && typeof P.run.fillReport === 'function') P.run.fillReport(t, result, data.run || {}, data.report);
            else result.appendChild(el('p', { text: t('chibi_sensei.test.report_none') }));
            const models = data.models || {};
            const note = (name, line) => {
                const box = el('p', { class: 'csn-note', dataset: { csn: name, tone: line.tone } });
                box.textContent = t(line.key, line.vars);
                return box;
            };
            reportBox.appendChild(el('h3', { text: t('chibi_sensei.test.view_report') }));
            reportBox.appendChild(result);
            reportBox.appendChild(note('model-tuned', modelLine('tuned', models.tuned)));
            reportBox.appendChild(note('model-base', modelLine('base', models.base)));
            const smoke = smokeLine(data.run && data.run.smoke);
            if (smoke) reportBox.appendChild(note('report-smoke', smoke));
        }

        // The report is marked as seen once per run and window, and never from a read-only desktop (a refused POST would switch the
        // whole window to read-only). A failure stays silent and the next visit tries again.
        function maybeSeen() {
            if (disposed || view !== 'report' || !data || data.report_seen !== false || inst.readonly || !selectedId || seen.has(selectedId)) return;
            const id = selectedId;
            seen.add(id);
            inst.api.reportSeen(id).then(() => {
                if (disposed) return;
                if (data && selectedId === id) data.report_seen = true;
                if (inst.runs.some(r => r.id === id)) inst.applyRun({ id, report_seen: true });
                if (inst.refresh) inst.refresh();
            }).catch(() => { seen.delete(id); });
        }

        // ---- the examples ----
        function filterOptions() {
            const rep = data && data.report;
            const ids = P.SAMPLE_FILTERS.filter(id => id !== 'wrong' || !!(rep && (rep.exact_match || rep.label_accuracy)));
            const sig = ids.join(',');
            if (filterSelect.dataset.sig !== sig) {
                filterSelect.dataset.sig = sig;
                P.clear(filterSelect);
                ids.forEach(id => filterSelect.appendChild(el('option', { value: id, text: t(FILTER_KEYS[id]) })));
            }
            filterSelect.value = ids.indexOf(samples.filter) >= 0 ? samples.filter : 'all';
        }

        async function loadSamples() {
            if (!selectedId || !data) return;
            const id = selectedId, mine = ++samples.seq, run = dataSeq;
            samples.loading = true; samples.loaded = true;
            paintSamples();
            try {
                const page = await inst.api.runSamples(id, { offset: samples.offset, limit: PAGE_SIZE, filter: samples.filter });
                if (disposed || mine !== samples.seq || run !== dataSeq) return;
                samples.page = page && typeof page === 'object' ? page : { samples: [], total: 0, offset: 0, limit: PAGE_SIZE };
                samples.missing = false; samples.loading = false;
                showError('');
            } catch (err) {
                if (disposed || mine !== samples.seq || run !== dataSeq || (err && err.aborted)) return;
                samples.loading = false; samples.loaded = false;
                const e = P.errors.normalize(err);
                if (e.code === 'samples_missing') { samples.missing = true; samples.page = null; } else showError(inst.handleError(err));
            }
            paintSamples();
        }

        function matchMark(labelKey, value) {
            if (typeof value !== 'boolean') return null;
            return el('span', { class: 'csn-match', dataset: { match: value ? '1' : '0' }, text: t(labelKey) + ': ' + t(value ? 'chibi_sensei.test.match_yes' : 'chibi_sensei.test.match_no') });
        }

        function answerCol(titleKey, output, ms) {
            const has = typeof output === 'string' && output !== '';
            return el('div', { class: 'csn-diff-col' },
                el('h5', { text: t(titleKey) }),
                has ? el('p', { class: 'csn-diff-text', text: output }) : el('p', { class: 'csn-muted', text: t('chibi_sensei.test.no_answer') }),
                Number.isFinite(Number(ms)) && ms !== null ? el('p', { class: 'csn-muted', text: t('chibi_sensei.test.answer_time', { time: timeText(ms) }) }) : null);
        }

        function sampleItem(sample, position) {
            const num = P.format.number(samples.offset + position + 1);
            const tryBtn = el('button', { type: 'button', class: 'csn-btn', ariaLabel: t('chibi_sensei.test.try_chat_label', { n: num }), dataset: { csn: 'sample-try' }, text: t('chibi_sensei.test.try_chat'), onClick: () => tryChat(sample) });
            const messages = Array.isArray(sample.prompt_messages) && sample.prompt_messages.length
                ? sample.prompt_messages.map(m => el('p', null, el('strong', { text: own(ROLE_KEYS, m && m.role) ? t(own(ROLE_KEYS, m.role)) : String(m && m.role || '') }), ' ', el('span', { class: 'csn-diff-text', text: String(m && m.content || '') })))
                : [el('p', { class: 'csn-diff-text', text: typeof sample.prompt_text === 'string' ? sample.prompt_text : '' })];
            return el('li', { class: 'csn-example', dataset: { csn: 'sample', index: String(samples.offset + position) } },
                el('div', { class: 'csn-example-head' }, el('h4', { text: t('chibi_sensei.test.sample_title', { n: num }) }),
                    matchMark('chibi_sensei.run.confusion_base', sample.base_match), matchMark('chibi_sensei.run.confusion_tuned', sample.tuned_match), tryBtn),
                el('h5', { text: t('chibi_sensei.test.prompt') }), messages,
                el('h5', { text: t('chibi_sensei.test.expected') }), el('p', { class: 'csn-diff-text', text: String(sample.expected || '') }),
                el('div', { class: 'csn-diff' }, answerCol('chibi_sensei.run.confusion_base', sample.base_output, sample.base_ms), answerCol('chibi_sensei.run.confusion_tuned', sample.tuned_output, sample.tuned_ms)),
                sample.truncated ? el('p', { class: 'csn-note', text: t('chibi_sensei.test.truncated') }) : null);
        }

        function paintTry() {
            const blocked = inst.readonly || !tunedOk();
            const why = inst.readonly ? t('chibi_sensei.readonly_banner') : (!tunedOk() && data ? t(modelLine('tuned', data.models && data.models.tuned).key) : '');
            samplesList.querySelectorAll('[data-csn="sample-try"]').forEach(btn => { btn.disabled = blocked; btn.title = blocked ? why : ''; });
        }

        function paintSamples() {
            if (!data) { samplesBox.hidden = true; return; }
            samplesBox.hidden = false;
            filterOptions();
            const page = samples.page;
            const list = page && Array.isArray(page.samples) ? page.samples : [];
            P.clear(samplesList);
            list.forEach((sample, i) => samplesList.appendChild(sampleItem(sample || {}, i)));
            let note = '';
            if (samples.loading) note = t('chibi_sensei.test.loading');
            else if (samples.missing) note = t('chibi_sensei.error.samples_missing');
            else if (samples.loaded && !list.length) note = t('chibi_sensei.test.samples_empty');
            setText(samplesNote, note);
            samplesNote.hidden = !note;
            const total = page ? Number(page.total) || 0 : 0;
            pager.hidden = !list.length && !total;
            setText(pageText, t('chibi_sensei.table.page', { from: P.format.number(list.length ? samples.offset + 1 : 0), to: P.format.number(samples.offset + list.length), total: P.format.number(total) }));
            pagePrev.disabled = samples.loading || samples.offset <= 0;
            pageNext.disabled = samples.loading || samples.offset + PAGE_SIZE >= total;
            samplesBox.setAttribute('aria-busy', samples.loading ? 'true' : 'false');
            paintTry();
        }

        // ---- the chat ----
        function ensureChat() {
            if (chatArea || !data || !P.chat) return;
            chatArea = P.chat.createArea(inst, { runId: selectedId, test: data });
            panels.chat.appendChild(chatArea.el);
        }

        function tryChat(sample) {
            showView('chat', {});
            if (!chatArea || !chatArea.setPrompt(promptOf(sample))) return;
            inst.notify(t('chibi_sensei.test.try_chat_done'), 'info');
            chatArea.focus();
        }

        // ---- tabs ----
        function onTabKey(event, name) {
            const at = VIEWS.indexOf(name);
            let next = -1;
            if (event.key === 'ArrowRight') next = (at + 1) % VIEWS.length;
            else if (event.key === 'ArrowLeft') next = (at + VIEWS.length - 1) % VIEWS.length;
            else if (event.key === 'Home') next = 0;
            else if (event.key === 'End') next = VIEWS.length - 1;
            if (next < 0) return;
            event.preventDefault();
            showView(VIEWS[next], {});
            tabs[VIEWS[next]].focus();
        }

        function paintViews() {
            VIEWS.forEach(name => {
                const on = name === view;
                tabs[name].setAttribute('aria-selected', on ? 'true' : 'false');
                tabs[name].tabIndex = on ? 0 : -1;
                panels[name].hidden = !on || !selectedId;
            });
            if (view === 'chat') ensureChat();
            if (chatArea) chatArea.setVisible(view === 'chat');
        }

        function showView(name, opts) {
            if (disposed || VIEWS.indexOf(name) < 0) return;
            view = name;
            paintViews();
            if (name === 'samples' && data && !samples.loaded) loadSamples();
            if (name === 'report') maybeSeen();
            if (opts && opts.focus) {
                if (!data) pendingFocus = true; else focusView();
            }
        }

        function focusView() {
            pendingFocus = false;
            if (view === 'chat' && chatArea && chatArea.focus()) return;
            panels[view].focus({ preventScroll: false });
        }

        function paintAll() {
            if (disposed) return;
            paintRuns();
            loading.hidden = !(selectedId && !data && !loadFailed);
            retry.hidden = !loadFailed;
            paintReport();
            paintSamples();
            paintViews();
            if (pendingFocus && data) focusView();
        }

        offs.push(inst.bus.on('runs', syncRuns), inst.bus.on('run', syncRuns), inst.bus.on('readonly', () => { if (disposed) return; paintTry(); maybeSeen(); }));
        optionSig = null;
        syncRuns();
        paintViews();

        return {
            el: root,
            focus() { if (!empty.hidden) gotoTraining.focus(); else tabs[view].focus(); },
            showView,
            dispose() {
                disposed = true;
                offs.forEach(off => { if (typeof off === 'function') off(); });
                if (chatArea) { const old = chatArea; chatArea = null; old.dispose(); }
            }
        };
    }

    P.test = { create, succeededRuns, smokeLine, modelLine, baseReasonKey, promptOf, VIEWS };
})();
