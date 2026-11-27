// Chibi Sensei chat: the pure reducer of the conversation (turns, one or two answers per turn, send limits, request body, answer
// settings, runtime view) and the chat area of the Test step: the runtime strip (state, backend, GPU notice, Stop models), the
// install card, the composer with system prompt and advanced answer settings, one or two answer columns that stream through
// api.chatStream, announcements for screen readers and keyboard handling. Model output and every server text reach the page as
// textContent only. The pure part touches no DOM at load time, so a Node test can load it in a vm. Classic script, publishes P.chat.
(function () {
    'use strict';

    const P = window.ChibiSenseiParts = window.ChibiSenseiParts || {};
    const TARGETS = ['tuned', 'base'];
    const POLL_INSTALL_MS = 1000;
    const POLL_IDLE_MS = 30000;
    const PIN_SLACK_PX = 24;
    const LIVE_DELAY_MS = 50;

    const STREAM_ERROR_KEYS = {
        start_failed: 'chibi_sensei.chat.error_start_failed', generation_failed: 'chibi_sensei.chat.error_generation_failed',
        context_exceeded: 'chibi_sensei.chat.error_context_exceeded', runtime_busy: 'chibi_sensei.chat.error_runtime_busy',
        cancelled: 'chibi_sensei.chat.error_cancelled', connection_lost: 'chibi_sensei.chat.error_connection_lost',
        unknown: 'chibi_sensei.chat.error_unknown'
    };
    const RUNTIME_STATE_KEYS = {
        loading: 'chibi_sensei.runtime.state_loading', error: 'chibi_sensei.runtime.state_error', disabled: 'chibi_sensei.runtime.state_disabled',
        unsupported: 'chibi_sensei.runtime.state_unsupported', not_installed: 'chibi_sensei.runtime.state_not_installed',
        installing: 'chibi_sensei.runtime.state_installing', ready: 'chibi_sensei.runtime.state_ready', failed: 'chibi_sensei.runtime.state_failed'
    };
    const BACKEND_KEYS = {
        cpu: 'chibi_sensei.runtime.backend_cpu', vulkan: 'chibi_sensei.runtime.backend_vulkan', cuda: 'chibi_sensei.runtime.backend_cuda',
        metal: 'chibi_sensei.runtime.backend_metal', auto: 'chibi_sensei.runtime.backend_auto'
    };
    const INSTALL_FAIL_KEYS = {
        download_failed: 'chibi_sensei.runtime.failed_download_failed', checksum_mismatch: 'chibi_sensei.runtime.failed_checksum_mismatch',
        disk_full: 'chibi_sensei.runtime.failed_disk_full', unknown: 'chibi_sensei.runtime.failed_unknown'
    };

    // A literal key map answers only for its own names, never for inherited ones such as constructor.
    const own = (map, key) => (Object.prototype.hasOwnProperty.call(map, key) ? map[key] : '');

    // ---- the pure part ---------------------------------------------------------------------------------------------------------

    function initialState() {
        return { turns: [], draft: '', system: '', compare: false, compareFrom: -1, streaming: false, seq: 0, notice: null };
    }

    const newAnswer = () => ({ state: 'waiting', text: '', stats: null, code: '' });
    const isFinal = a => a.state === 'done' || a.state === 'error' || a.state === 'stopped';

    // The last turn of a state with one answer replaced.
    function withAnswer(state, target, answer) {
        const last = state.turns[state.turns.length - 1];
        const turn = Object.assign({}, last, { started: true, answers: Object.assign({}, last.answers, { [target]: answer }) });
        return Object.assign({}, state, { turns: state.turns.slice(0, -1).concat([turn]) });
    }

    function applyEvent(state, ev) {
        if (!state.streaming || !ev || !state.turns.length) return state;
        const last = state.turns[state.turns.length - 1];
        const target = ev.target;
        const a = last.answers[target];
        if (TARGETS.indexOf(target) < 0 || !a || isFinal(a)) return state;
        switch (ev.type) {
        case 'status': return withAnswer(state, target, Object.assign({}, a, { state: ev.state === 'loading' ? 'loading' : 'streaming' }));
        case 'delta': return typeof ev.text === 'string' ? withAnswer(state, target, Object.assign({}, a, { state: 'streaming', text: a.text + ev.text })) : state;
        case 'done': return withAnswer(state, target, Object.assign({}, a, {
            state: 'done',
            stats: { finish_reason: ev.finish_reason, prompt_tokens: ev.prompt_tokens, completion_tokens: ev.completion_tokens, ms: ev.ms, tok_s: ev.tok_s, backend: ev.backend }
        }));
        case 'error':
            if (ev.code === 'cancelled') return withAnswer(state, target, Object.assign({}, a, { state: 'stopped', code: 'cancelled' }));
            return withAnswer(state, target, Object.assign({}, a, { state: 'error', code: Object.prototype.hasOwnProperty.call(STREAM_ERROR_KEYS, ev.code) ? ev.code : 'unknown' }));
        default: return state;
        }
    }

    function applyClose(state, action) {
        if (!state.streaming) return state;
        const last = state.turns[state.turns.length - 1];
        if (last && !last.started && action.error) {
            return Object.assign({}, state, { streaming: false, turns: state.turns.slice(0, -1), draft: last.user + (state.draft ? '\n' + state.draft : ''), notice: action.error });
        }
        if (!last) return Object.assign({}, state, { streaming: false });
        const answers = {};
        TARGETS.forEach(kind => {
            const a = last.answers[kind];
            if (!a) { answers[kind] = a; return; }
            if (isFinal(a)) { answers[kind] = a; return; }
            answers[kind] = action.aborted ? Object.assign({}, a, { state: 'stopped', code: 'cancelled' }) : Object.assign({}, a, { state: 'error', code: 'connection_lost' });
        });
        return Object.assign({}, state, { streaming: false, turns: state.turns.slice(0, -1).concat([Object.assign({}, last, { answers })]) });
    }

    // reduce(state, action) returns a new object, or the same object when nothing changes.
    function reduce(state, action) {
        const a = action || {};
        switch (a.type) {
        case 'draft': { const text = String(a.text == null ? '' : a.text); return text === state.draft ? state : Object.assign({}, state, { draft: text }); }
        case 'system': { const text = String(a.text == null ? '' : a.text); return text === state.system ? state : Object.assign({}, state, { system: text }); }
        case 'compare': {
            const on = !!a.on;
            if (on === state.compare) return state;
            return Object.assign({}, state, { compare: on, compareFrom: on && state.turns.length ? state.turns.length : state.compareFrom });
        }
        case 'send': {
            const text = String(a.text == null ? '' : a.text).trim();
            if (state.streaming || !text) return state;
            const turn = { id: state.seq + 1, user: text, started: false, answers: { tuned: newAnswer(), base: state.compare ? newAnswer() : null } };
            return Object.assign({}, state, { turns: state.turns.concat([turn]), seq: state.seq + 1, streaming: true, draft: '', notice: null });
        }
        case 'event': return applyEvent(state, a.ev);
        case 'close': return applyClose(state, a);
        case 'new_chat':
            if (state.streaming || (!state.turns.length && !state.notice && state.compareFrom < 0)) return state;
            return Object.assign({}, state, { turns: [], compareFrom: -1, notice: null });
        case 'notice': return a.value === state.notice ? state : Object.assign({}, state, { notice: a.value == null ? null : a.value });
        default: return state;
        }
    }

    function usable(a) { return !!a && (a.state === 'done' || a.state === 'stopped') && a.text.trim() !== ''; }

    // The target's own conversation of finished turns: a turn the target has no usable answer for is left out completely, so roles
    // keep alternating and every request ends with the new user message.
    function conversation(state, target) {
        const out = [];
        const system = String(state.system || '').trim();
        if (system) out.push({ role: 'system', content: system });
        state.turns.forEach(turn => {
            const a = turn.answers[target];
            if (usable(a)) out.push({ role: 'user', content: turn.user }, { role: 'assistant', content: a.text });
        });
        return out;
    }

    function targetsOf(state) { return state.compare ? ['tuned', 'base'] : ['tuned']; }

    // Characters of the longest conversation the next message would send (Unicode code points, as the server counts them).
    function usedChars(state, text) {
        const msg = String(text || '').trim();
        let most = 0;
        targetsOf(state).forEach(target => {
            const total = conversation(state, target).concat(msg ? [{ role: 'user', content: msg }] : []).reduce((n, m) => n + P.charCount(m.content), 0);
            if (total > most) most = total;
        });
        return most;
    }

    // canSend -> {ok, reasonKey, vars}
    function canSend(state, limits, text) {
        if (state.streaming) return { ok: false, reasonKey: 'chibi_sensei.chat.busy' };
        const msg = String(text || '').trim();
        if (!msg) return { ok: false, reasonKey: '' };
        for (const target of targetsOf(state)) {
            const msgs = conversation(state, target).concat([{ role: 'user', content: msg }]);
            if (msgs.filter(m => m.role === 'user').length > limits.max_turns) return { ok: false, reasonKey: 'chibi_sensei.chat.limit_turns', vars: { max: limits.max_turns } };
            if (msgs.reduce((n, m) => n + P.charCount(m.content), 0) > limits.max_chars) return { ok: false, reasonKey: 'chibi_sensei.chat.limit_chars', vars: { max: limits.max_chars } };
        }
        return { ok: true, reasonKey: '' };
    }

    function requestBody(state, runId, text, sampling) {
        const msg = String(text || '').trim();
        const body = { run_id: runId, targets: targetsOf(state).map(kind => ({ kind, messages: conversation(state, kind).concat([{ role: 'user', content: msg }]) })) };
        if (sampling && Object.keys(sampling).length) body.sampling = sampling;
        return body;
    }

    // validateSampling mirrors the server ranges of POST /runtime/chat. draft holds the form's strings ('' = the model's default);
    // a decimal comma is accepted; the seed is never sent. It returns only the values that are set.
    function validateSampling(draft, cap) {
        const d = draft || {}, sampling = {}, issues = [];
        const raw = name => String(d[name] == null ? '' : d[name]).trim().replace(',', '.');
        const decimal = (name, min, max, lowerOpen, shownMin) => {
            const s = raw(name);
            if (!s) return;
            const n = Number(s);
            if (!/^(\d+(\.\d*)?|\.\d+)$/.test(s) || !Number.isFinite(n) || n > max || n < min || (lowerOpen && n <= min)) issues.push({ field: name, min: shownMin, max }); else sampling[name] = n;
        };
        const whole = (name, min, max) => {
            const s = raw(name);
            if (!s) return;
            const n = Number(s);
            if (!/^\d+$/.test(s) || n < min || n > max) issues.push({ field: name, min, max }); else sampling[name] = n;
        };
        decimal('temperature', 0, 2, false, 0);
        decimal('top_p', 0, 1, true, 0.01);
        whole('top_k', 0, 200);
        whole('max_tokens', 1, Number(cap) > 0 ? Number(cap) : 1024);
        return { sampling, issues };
    }

    // runtimeView(rt, admin, readonly): rt is undefined while loading, null when the status could not be read.
    function runtimeView(rt, admin, readonly) {
        let state = rt === undefined ? 'loading' : (rt === null ? 'error' : String(rt.state || ''));
        if (!Object.prototype.hasOwnProperty.call(RUNTIME_STATE_KEYS, state)) state = 'error';
        let card = null;
        if (state === 'not_installed') card = admin ? 'install' : 'ask';
        else if (state === 'installing' || state === 'failed' || state === 'disabled' || state === 'unsupported' || state === 'error') card = state;
        return {
            state, card, composer: state === 'ready',
            canStop: !!(rt && Array.isArray(rt.instances) && rt.instances.length > 0),
            installEnabled: !!admin && !readonly,
            installReasonKey: readonly ? 'chibi_sensei.readonly_banner' : ''
        };
    }

    // answerView(a) -> {state, phase} for data-state and data-phase; the phase is '' once the answer is final.
    function answerView(a) {
        const s = a && a.state;
        if (s === 'waiting') return { state: 'streaming', phase: 'waiting' };
        if (s === 'loading') return { state: 'streaming', phase: 'loading' };
        if (s === 'streaming') return { state: 'streaming', phase: 'generating' };
        if (s === 'done') return { state: 'done', phase: '' };
        if (s === 'stopped') return { state: 'stopped', phase: '' };
        return { state: 'error', phase: '' };
    }

    // ---- the chat area ---------------------------------------------------------------------------------------------------------

    let areaCounter = 0;

    function createArea(inst, ctx) {
        const { el } = P;
        const t = inst.t;
        const c = ctx || {};
        const test = c.test || {};
        const models = test.models || {};
        const tunedOk = !!(models.tuned && models.tuned.available);
        const baseOk = !!(models.base && models.base.available);
        const defaults = test.chat || {};
        const limits = P.chatLimits(inst.meta);
        const idPrefix = 'csn-chat-' + (++areaCounter);
        let state = initialState();
        let runtime;
        let disposed = false, visible = false, shown = false, pinned = true, stream = null, rtSeq = 0, timer = 0, raf = 0, liveTimer = 0;
        let installBusy = false, stopping = false, turnsSig = null, lastNotice = null;
        const nodes = new Map();
        const offs = [];

        // ---- nodes ----
        const stateText = el('span', { class: 'csn-runtime-state', role: 'status' });
        const modelsText = el('span', { class: 'csn-muted' });
        const stopModels = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'runtime-stop' }, text: t('chibi_sensei.runtime.stop'), onClick: () => stopRuntimeModels() });
        const strip = el('div', { class: 'csn-runtime', dataset: { csn: 'runtime-strip', state: 'loading' } }, stateText, modelsText, stopModels);
        const gpuNote = el('p', { class: 'csn-note', dataset: { csn: 'runtime-notice', tone: 'warn' }, text: t('chibi_sensei.runtime.gpu_fallback') });
        const cardHead = el('strong');
        const cardText = el('p');
        const progressBar = el('progress', { class: 'csn-progressbar', dataset: { csn: 'runtime-progress' } });
        const progressText = el('p', { class: 'csn-hint' });
        const installBtn = el('button', { type: 'button', class: 'csn-btn csn-btn-primary', dataset: { csn: 'runtime-install' }, onClick: () => installRuntime() });
        const settingsBtn = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'runtime-settings' }, text: t('chibi_sensei.runtime.open_settings'), onClick: () => P.openLink('/config#chibi_sensei') });
        const retryBtn = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'runtime-retry' }, text: t('chibi_sensei.retry'), onClick: () => refreshRuntime() });
        const card = el('div', { class: 'csn-note', dataset: { csn: 'runtime-card', card: '' } }, el('p', null, cardHead), cardText, progressBar, progressText, el('div', { class: 'csn-actions' }, installBtn, settingsBtn, retryBtn));
        const errorLine = el('p', { class: 'csn-error', role: 'alert', dataset: { csn: 'chat-error' } });
        const readonlyNote = el('p', { class: 'csn-note', dataset: { csn: 'chat-readonly' }, text: t('chibi_sensei.chat.readonly') });
        const tunedMissing = el('p', { class: 'csn-note', dataset: { csn: 'chat-tuned-missing', tone: 'warn' } });

        const emptyNote = el('p', { class: 'csn-muted', dataset: { csn: 'chat-empty' }, text: t('chibi_sensei.chat.empty') });
        const tunedTitle = el('h4', { id: idPrefix + '-tuned', text: t('chibi_sensei.chat.column_tuned') });
        const baseTitle = el('h4', { id: idPrefix + '-base', text: t('chibi_sensei.run.confusion_base') });
        const tunedList = el('ol', { class: 'csn-chat-turns' });
        const baseList = el('ol', { class: 'csn-chat-turns' });
        const tunedColumn = el('section', { class: 'csn-chat-column', ariaLabelledby: idPrefix + '-tuned', dataset: { csn: 'chat-column', target: 'tuned' } }, tunedTitle, tunedList);
        const baseColumn = el('section', { class: 'csn-chat-column', ariaLabelledby: idPrefix + '-base', dataset: { csn: 'chat-column', target: 'base' } }, baseTitle, baseList);
        const columns = el('div', { class: 'csn-chat-columns', dataset: { compare: '0' } }, tunedColumn, baseColumn);
        const scroller = el('div', { class: 'csn-chat-scroll', role: 'region', tabindex: '0', ariaLabel: t('chibi_sensei.chat.history') }, emptyNote, columns);

        const systemInput = el('textarea', { class: 'csn-textarea', rows: '2', dataset: { csn: 'chat-system' } });
        const input = el('textarea', { class: 'csn-textarea', rows: '3', placeholder: t('chibi_sensei.chat.input_placeholder'), dataset: { csn: 'chat-input' } });
        const compareBox = el('input', { type: 'checkbox', role: 'switch', dataset: { csn: 'chat-compare' } });
        const compareReason = el('span', { class: 'csn-hint', dataset: { csn: 'chat-compare-reason' } });
        const charsHint = el('span', { class: 'csn-hint', dataset: { csn: 'chat-chars' } });
        const newBtn = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'chat-new' }, text: t('chibi_sensei.chat.new'), onClick: () => newChat() });
        const stopBtn = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'chat-stop' }, text: t('chibi_sensei.chat.stop'), onClick: () => stopAnswer() });
        const sendBtn = el('button', { type: 'button', class: 'csn-btn csn-btn-primary', dataset: { csn: 'chat-send' }, text: t('chibi_sensei.chat.send'), onClick: () => send() });
        const reasonLine = el('p', { class: 'csn-hint', dataset: { csn: 'chat-reason' } });
        const samplingError = el('p', { class: 'csn-error', role: 'alert', dataset: { csn: 'chat-sampling-error' } });

        const sampling = {
            temperature: samplingField('chat-temperature', t('chibi_sensei.chat.temperature'), defaults.sampling && defaults.sampling.temperature),
            top_p: samplingField('chat-top-p', t('chibi_sensei.chat.top_p'), defaults.sampling && defaults.sampling.top_p),
            top_k: samplingField('chat-top-k', t('chibi_sensei.chat.top_k'), defaults.sampling && defaults.sampling.top_k),
            max_tokens: samplingField('chat-max-tokens', t('chibi_sensei.chat.max_tokens', { max: P.format.number(limits.max_tokens_cap) }), defaults.max_tokens)
        };
        const resetBtn = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'chat-reset' }, text: t('chibi_sensei.chat.reset'), onClick: () => resetSampling() });
        const advanced = el('details', { class: 'csn-advanced', dataset: { csn: 'chat-advanced' } },
            el('summary', { text: t('chibi_sensei.chat.advanced') }),
            el('p', { class: 'csn-hint', text: t('chibi_sensei.chat.advanced_help') }),
            el('div', { class: 'csn-grid2' }, sampling.temperature.label, sampling.top_p.label, sampling.top_k.label, sampling.max_tokens.label),
            samplingError, resetBtn);

        const composer = el('div', { class: 'csn-composer' },
            el('label', { class: 'csn-field' }, el('span', { class: 'csn-label', text: t('chibi_sensei.chat.system_label') }), systemInput, el('span', { class: 'csn-hint', text: t('chibi_sensei.chat.system_help') })),
            el('label', { class: 'csn-field' }, el('span', { class: 'csn-label', text: t('chibi_sensei.chat.input_label') }), input),
            el('div', { class: 'csn-composer-bar' },
                el('label', { class: 'csn-check-option' }, compareBox, el('span', { text: t('chibi_sensei.chat.compare') })),
                compareReason, el('span', { class: 'csn-spacer' }), charsHint, newBtn, stopBtn, sendBtn),
            reasonLine, advanced,
            el('p', { class: 'csn-muted', text: t('chibi_sensei.chat.not_saved') }));
        const body = el('div', { class: 'csn-chat-body', dataset: { csn: 'chat-body' } }, scroller, composer);
        const live = el('div', { class: 'csn-sr-only', role: 'status', ariaLive: 'polite', ariaAtomic: 'true', dataset: { csn: 'chat-live' } });
        const node = el('div', { class: 'csn-chat', dataset: { csn: 'chat' } }, strip, gpuNote, card, errorLine, readonlyNote, tunedMissing, body, live);

        function samplingField(name, labelText, fallback) {
            const hasDefault = fallback !== undefined && fallback !== null && fallback !== '';
            const shown2 = hasDefault ? P.format.decimal(fallback, 3) : '';
            const field = el('input', { class: 'csn-input', type: 'text', inputmode: 'decimal', placeholder: shown2, dataset: { csn: name } });
            const hint = el('span', { class: 'csn-hint', text: hasDefault ? t('chibi_sensei.chat.default_value', { value: shown2 }) : '' });
            hint.hidden = !hasDefault;
            field.addEventListener('input', () => { field.removeAttribute('aria-invalid'); samplingError.textContent = ''; samplingError.hidden = true; });
            return { input: field, label: el('label', { class: 'csn-field' }, el('span', { class: 'csn-label', text: labelText }), field, hint) };
        }

        [errorLine, samplingError, gpuNote, readonlyNote, tunedMissing].forEach(n => { n.hidden = true; });

        const setText = (n, text) => { if (n.textContent !== text) n.textContent = text; };
        const showError = text => { setText(errorLine, text || ''); errorLine.hidden = !text; };
        const readonly = () => !!inst.readonly;
        const stoppable = () => !!stream;
        const backendName = id => (own(BACKEND_KEYS, id) ? t(own(BACKEND_KEYS, id)) : String(id || ''));

        function announce(text) {
            clearTimeout(liveTimer);
            live.textContent = '';
            liveTimer = setTimeout(() => { if (!disposed) live.textContent = text; }, LIVE_DELAY_MS);
        }

        // ---- painting ----
        function paintRuntime() {
            const view = runtimeView(runtime, !!(runtime && runtime.admin), readonly());
            const rt = runtime || null;
            strip.dataset.state = view.state;
            let line = t(RUNTIME_STATE_KEYS[view.state]);
            if (view.state === 'ready' && rt) {
                const b = rt.backend && (rt.backend.selected || rt.backend.requested);
                if (own(BACKEND_KEYS, b)) line += ' · ' + t('chibi_sensei.runtime.backend', { backend: t(own(BACKEND_KEYS, b)) });
            }
            setText(stateText, line);
            const count = rt && Array.isArray(rt.instances) ? rt.instances.length : 0;
            const lim = rt && rt.limits ? rt.limits : {};
            setText(modelsText, count ? t('chibi_sensei.runtime.models_loaded', { count: P.format.number(count), max: P.format.number(lim.max_instances || count) }) + ' · ' + t('chibi_sensei.runtime.idle_note', { minutes: P.format.number(lim.idle_minutes || 0) }) : '');
            modelsText.hidden = !count;
            stopModels.hidden = !view.canStop;
            stopModels.disabled = stopping;
            gpuNote.hidden = !(rt && rt.notice_code === 'gpu_fallback');
            paintCard(view, rt);
            paintComposer();
        }

        function paintCard(view, rt) {
            card.hidden = !view.card;
            card.dataset.card = view.card || '';
            if (!view.card) return;
            const dl = rt && rt.download ? rt.download : {};
            const admin = !!(rt && rt.admin);
            const key = view.card === 'ask' || view.card === 'install' ? 'not_installed' : view.card;
            setText(cardHead, t(RUNTIME_STATE_KEYS[key]));
            let text = '';
            if (view.card === 'install') {
                const b = rt.backend && (rt.backend.selected || rt.backend.requested);
                text = t('chibi_sensei.runtime.install_text', { build: String(rt.build || ''), backend: backendName(b), size: P.format.bytes(dl.bytes), host: String(dl.host || '') });
            } else if (view.card === 'ask') text = t('chibi_sensei.runtime.install_ask');
            else if (view.card === 'failed') text = t(own(INSTALL_FAIL_KEYS, dl.error_code) || INSTALL_FAIL_KEYS.unknown);
            else if (view.card === 'disabled') text = t(admin ? 'chibi_sensei.runtime.disabled_admin' : 'chibi_sensei.runtime.disabled_user');
            setText(cardText, text);
            cardText.hidden = !text;
            const installing = view.card === 'installing';
            progressBar.hidden = progressText.hidden = !installing;
            if (installing) {
                progressBar.max = Math.max(1, Number(dl.bytes) || 1);
                progressBar.value = Math.max(0, Number(dl.done_bytes) || 0);
                setText(progressText, t('chibi_sensei.runtime.progress', { done: P.format.bytes(dl.done_bytes), total: P.format.bytes(dl.bytes) }));
            }
            const canInstall = view.card === 'install' || (view.card === 'failed' && admin);
            installBtn.hidden = !canInstall;
            setText(installBtn, view.card === 'failed' ? t('chibi_sensei.retry') : t('chibi_sensei.runtime.install'));
            installBtn.disabled = !view.installEnabled || installBusy;
            installBtn.title = view.installReasonKey ? t(view.installReasonKey) : '';
            settingsBtn.hidden = !(view.card === 'disabled' && admin);
            retryBtn.hidden = view.card !== 'error';
        }

        function composerShown() {
            return runtimeView(runtime, !!(runtime && runtime.admin), readonly()).composer && !readonly() && tunedOk;
        }

        function paintComposer() {
            const ro = readonly();
            readonlyNote.hidden = !ro;
            const missing = !tunedOk && !!P.test && typeof P.test.modelLine === 'function';
            tunedMissing.hidden = !missing;
            if (missing) { const line = P.test.modelLine('tuned', models.tuned); setText(tunedMissing, t(line.key, line.vars)); }
            const show = composerShown();
            body.hidden = !show;
            if (!show) return;
            const can = canSend(state, limits, input.value);
            sendBtn.disabled = state.streaming || !can.ok;
            stopBtn.disabled = !state.streaming;
            newBtn.disabled = state.streaming || !state.turns.length;
            if (stopBtn.disabled && document.activeElement === stopBtn) input.focus();
            setText(reasonLine, can.reasonKey ? t(can.reasonKey, can.vars) : '');
            reasonLine.hidden = !can.reasonKey;
            setText(charsHint, t('chibi_sensei.chat.chars', { count: P.format.number(usedChars(state, input.value)), max: P.format.number(limits.max_chars) }));
            compareBox.checked = state.compare && baseOk;
            const locked = !baseOk || state.streaming;
            compareBox.disabled = locked;
            compareBox.setAttribute('aria-disabled', locked ? 'true' : 'false');
            let why = '';
            if (!baseOk) { const reasonKey = P.test && typeof P.test.baseReasonKey === 'function' ? P.test.baseReasonKey(models.base && models.base.reason) : ''; why = reasonKey ? t(reasonKey) : ''; }
            else if (state.compare && state.compareFrom >= 0 && state.turns.length === state.compareFrom) why = t('chibi_sensei.chat.compare_fresh');
            setText(compareReason, why);
            compareReason.hidden = !why;
        }

        function buildTurnNodes(turn) {
            const entry = {};
            const turnItem = target => {
                const answer = turn.answers[target];
                const user = el('div', { class: 'csn-chat-user' }, el('span', { class: 'csn-sr-only', text: t('chibi_sensei.chat.you') }), turn.user);
                if (!answer) return el('li', { class: 'csn-chat-turn' }, user, el('p', { class: 'csn-muted', text: t('chibi_sensei.chat.not_compared') }));
                const textNode = el('p', { class: 'csn-chat-text' });
                const stats = el('p', { class: 'csn-chat-stats' });
                const err = el('p', { class: 'csn-error' });
                const box = el('div', { class: 'csn-chat-answer', dataset: { csn: 'chat-answer', target, state: 'streaming', phase: 'waiting' } }, textNode, stats, err);
                entry[target] = { box, text: textNode, stats, err };
                return el('li', { class: 'csn-chat-turn' }, user, box);
            };
            nodes.set(turn.id, entry);
            return { tuned: turnItem('tuned'), base: turnItem('base') };
        }

        function answerStats(a) {
            if (a.state === 'stopped') return t('chibi_sensei.chat.error_cancelled');
            if (a.state !== 'done' || !a.stats) return '';
            const s = a.stats;
            let line = t('chibi_sensei.chat.stats', { tokens: P.format.number(s.completion_tokens), speed: P.format.decimal(s.tok_s, 1), backend: backendName(s.backend) });
            if (s.finish_reason === 'length') line += ' ' + t('chibi_sensei.chat.cut_length');
            return line;
        }

        function paintAnswer(turn, target) {
            const entry = nodes.get(turn.id);
            const parts = entry && entry[target];
            const a = turn.answers[target];
            if (!parts || !a) return;
            const view = answerView(a);
            parts.box.dataset.state = view.state;
            parts.box.dataset.phase = view.phase;
            parts.box.setAttribute('aria-busy', view.state === 'streaming' ? 'true' : 'false');
            let text = a.text;
            if (a.state === 'waiting') text = t('chibi_sensei.chat.waiting');
            else if (a.state === 'loading') text = t('chibi_sensei.chat.loading');
            setText(parts.text, text);
            setText(parts.stats, answerStats(a));
            setText(parts.err, a.state === 'error' ? t(own(STREAM_ERROR_KEYS, a.code) || STREAM_ERROR_KEYS.unknown) : '');
            parts.err.hidden = a.state !== 'error';
            parts.stats.hidden = !parts.stats.textContent;
        }

        // The turn lists are rebuilt only when the turns or the compare switch change; otherwise the answers repaint in place.
        function paintTurns() {
            const sig = state.turns.map(turn => turn.id).join(',') + '|' + (state.compare ? '1' : '0');
            if (sig !== turnsSig) {
                turnsSig = sig;
                nodes.clear();
                P.clear(tunedList); P.clear(baseList);
                state.turns.forEach(turn => { const items = buildTurnNodes(turn); tunedList.appendChild(items.tuned); baseList.appendChild(items.base); });
            }
            columns.dataset.compare = state.compare ? '1' : '0';
            baseColumn.hidden = !state.compare;
            emptyNote.hidden = state.turns.length > 0;
            state.turns.forEach(turn => TARGETS.forEach(target => paintAnswer(turn, target)));
            if (pinned) scroller.scrollTop = scroller.scrollHeight;
        }

        function paintNotice() {
            if (state.notice === lastNotice) return;
            lastNotice = state.notice;
            if (!state.notice) { showError(''); return; }
            const e = P.errors.normalize(state.notice);
            showError(e.status === 422 ? t('chibi_sensei.chat.invalid') : inst.handleError(state.notice));
            if (e.code === 'runtime_not_installed' || e.code === 'runtime_busy' || e.code === 'runtime_unavailable') refreshRuntime();
        }

        function paint() {
            if (disposed) return;
            if (raf) { cancelAnimationFrame(raf); raf = 0; }
            paintTurns();
            paintNotice();
            paintComposer();
        }

        function dispatch(action) {
            const next = reduce(state, action);
            if (next === state) return false;
            state = next;
            if (action.type === 'event' && action.ev && action.ev.type === 'delta') {
                if (!raf && !disposed) raf = requestAnimationFrame(() => { raf = 0; if (!disposed) paint(); });
            } else paint();
            return true;
        }

        // ---- runtime ----
        function syncTimer() {
            clearTimeout(timer);
            timer = 0;
            if (disposed || !visible || !runtime) return;
            const ms = runtime.state === 'installing' ? POLL_INSTALL_MS : (Array.isArray(runtime.instances) && runtime.instances.length ? POLL_IDLE_MS : 0);
            if (ms) timer = setTimeout(() => { timer = 0; refreshRuntime(); }, ms);
        }

        function setRuntime(next) {
            const was = runtime && runtime.state;
            const lost = card.contains(document.activeElement);
            runtime = next;
            paintRuntime();
            if (was === 'installing' && next && next.state === 'ready') {
                inst.notify(t('chibi_sensei.runtime.installed'), 'info');
                if (lost && composerShown()) input.focus();
            }
            syncTimer();
        }

        async function refreshRuntime() {
            const mine = ++rtSeq;
            try {
                const next = await inst.api.runtime();
                if (disposed || mine !== rtSeq) return;
                setRuntime(next);
            } catch (err) {
                if (disposed || mine !== rtSeq || (err && err.aborted)) return;
                if (runtime === undefined) setRuntime(null);
                else syncTimer();
            }
        }

        async function installRuntime() {
            const rt = runtime;
            if (installBusy || !rt || !rt.admin || readonly()) return;
            installBusy = true;
            rtSeq++;
            paintRuntime();
            const dl = rt.download || {};
            const b = rt.backend && (rt.backend.selected || rt.backend.requested);
            const ok = await inst.askConfirm(t('chibi_sensei.runtime.install_confirm_title'), t('chibi_sensei.runtime.install_confirm_text', { build: String(rt.build || ''), backend: backendName(b), size: P.format.bytes(dl.bytes), host: String(dl.host || '') }));
            if (disposed) return;
            if (!ok) { installBusy = false; paintRuntime(); return; }
            try {
                const next = await inst.api.installRuntime();
                if (disposed) return;
                dispatch({ type: 'notice', value: null });
                if (next) setRuntime(next); else refreshRuntime();
            } catch (err) {
                if (disposed || (err && err.aborted)) return;
                const e = P.errors.normalize(err);
                dispatch({ type: 'notice', value: err });
                if (e.code === 'runtime_unavailable') refreshRuntime();
            } finally { installBusy = false; if (!disposed) paintRuntime(); }
        }

        async function stopRuntimeModels() {
            if (stopping) return;
            stopping = true;
            rtSeq++;
            if (stream) stream.abort();
            paintRuntime();
            try {
                const next = await inst.api.stopRuntime();
                if (disposed) return;
                dispatch({ type: 'notice', value: null });
                if (next) setRuntime(next); else refreshRuntime();
                inst.notify(t('chibi_sensei.runtime.stopped'), 'info');
            } catch (err) {
                if (disposed || (err && err.aborted)) return;
                dispatch({ type: 'notice', value: err });
            } finally { stopping = false; if (!disposed) paintRuntime(); }
        }

        // ---- chat ----
        function markSamplingIssues(issues) {
            Object.keys(sampling).forEach(name => sampling[name].input.removeAttribute('aria-invalid'));
            issues.forEach(i => { if (sampling[i.field]) sampling[i.field].input.setAttribute('aria-invalid', 'true'); });
            const first = issues[0];
            setText(samplingError, first ? t('chibi_sensei.chat.sampling_invalid', { min: P.format.decimal(first.min, 2), max: P.format.number(first.max) }) : '');
            samplingError.hidden = !first;
            if (first) { advanced.open = true; sampling[first.field].input.focus(); }
        }

        function resetSampling() {
            Object.keys(sampling).forEach(name => { sampling[name].input.value = ''; sampling[name].input.removeAttribute('aria-invalid'); });
            samplingError.textContent = '';
            samplingError.hidden = true;
        }

        function send() {
            if (disposed || state.streaming || !composerShown()) return;
            const checked = validateSampling({ temperature: sampling.temperature.input.value, top_p: sampling.top_p.input.value, top_k: sampling.top_k.input.value, max_tokens: sampling.max_tokens.input.value }, limits.max_tokens_cap);
            if (checked.issues.length) { markSamplingIssues(checked.issues); return; }
            markSamplingIssues([]);
            const text = input.value;
            if (!canSend(state, limits, text).ok) { paintComposer(); return; }
            const request = requestBody(state, c.runId, text, checked.sampling);
            pinned = true;
            dispatch({ type: 'send', text });
            input.value = '';
            paintComposer();
            let mine = null;
            mine = stream = inst.api.chatStream(request, {
                onEvent: ev => {
                    if (disposed || stream !== mine) return;
                    const last = state.turns[state.turns.length - 1];
                    const before = !!(last && last.started);
                    dispatch({ type: 'event', ev });
                    const now = state.turns[state.turns.length - 1];
                    if (!before && now && now.started) announce(t(now.answers.base ? 'chibi_sensei.chat.announce_start_both' : 'chibi_sensei.chat.announce_start'));
                },
                onClose: result => {
                    if (disposed || stream !== mine) return;
                    stream = null;
                    const r = result || {};
                    const last = state.turns[state.turns.length - 1];
                    dispatch({ type: 'close', ended: !!r.ended, aborted: !!r.aborted, error: r.error || null });
                    const now = state.turns[state.turns.length - 1];
                    if (now && last && now.id === last.id) {
                        const list = TARGETS.map(kind => now.answers[kind]).filter(Boolean);
                        if (r.aborted) announce(t('chibi_sensei.chat.announce_stopped'));
                        else if (list.some(a => a.state === 'error')) announce(t('chibi_sensei.chat.announce_error'));
                        else announce(t(list.length > 1 ? 'chibi_sensei.chat.announce_done_both' : 'chibi_sensei.chat.announce_done'));
                    }
                    if (!now || !last || now.id !== last.id) input.value = state.draft;
                    else if (state.draft && !input.value) input.value = state.draft;
                    paintComposer();
                    refreshRuntime();
                    if (!disposed && composerShown() && (document.activeElement === document.body || !document.activeElement)) input.focus();
                }
            });
            input.focus();
        }

        function stopAnswer() {
            if (!stoppable()) return;
            stream.abort();
            input.focus();
        }

        function newChat() {
            if (dispatch({ type: 'new_chat' })) { pinned = true; clearTimeout(liveTimer); live.textContent = ''; input.focus(); }
        }

        // ---- events ----
        input.addEventListener('input', () => { dispatch({ type: 'draft', text: input.value }); paintComposer(); });
        systemInput.addEventListener('input', () => { dispatch({ type: 'system', text: systemInput.value }); paintComposer(); });
        input.addEventListener('keydown', event => {
            if (event.key === 'Enter' && !event.shiftKey && !event.isComposing && event.keyCode !== 229) { event.preventDefault(); send(); }
        });
        node.addEventListener('keydown', event => {
            if (event.key === 'Escape' && state.streaming) { event.preventDefault(); event.stopPropagation(); stopAnswer(); }
        });
        compareBox.addEventListener('change', () => { dispatch({ type: 'compare', on: compareBox.checked && baseOk }); paintComposer(); });
        scroller.addEventListener('scroll', () => { pinned = scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight <= PIN_SLACK_PX; }, { passive: true });
        offs.push(inst.bus.on('readonly', () => {
            if (disposed) return;
            if (readonly() && stream) stream.abort();
            paintRuntime();
        }));
        paintRuntime();
        paint();

        return {
            el: node,
            setVisible(on) {
                if (disposed) return;
                visible = !!on;
                if (!visible) { clearTimeout(timer); timer = 0; return; }
                if (!shown) shown = true;
                refreshRuntime();
            },
            // True when the message box took the focus; false when this window cannot chat (the caller focuses the panel).
            focus() {
                if (!composerShown()) return false;
                input.focus();
                return true;
            },
            // Stores an example as system text and draft; the message box shows it as soon as it appears. False only after dispose.
            setPrompt(prompt) {
                if (disposed) return false;
                const p = prompt || {};
                const system = String(p.system || '');
                if (system) { systemInput.value = system; dispatch({ type: 'system', text: system }); }
                input.value = String(p.user || '');
                dispatch({ type: 'draft', text: input.value });
                paintComposer();
                return true;
            },
            dispose() {
                disposed = true;
                clearTimeout(timer); clearTimeout(liveTimer);
                if (raf) cancelAnimationFrame(raf);
                raf = 0;
                offs.forEach(off => { if (typeof off === 'function') off(); });
                if (stream) { const s = stream; stream = null; s.abort(); }
            }
        };
    }

    P.chat = { createArea, initialState, reduce, conversation, canSend, requestBody, validateSampling, runtimeView, answerView, usable, STREAM_ERROR_KEYS };
})();
