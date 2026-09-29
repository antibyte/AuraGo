(function () {
    'use strict';
    const R = window.RTLSDRRuntime, P = window.RTLSDRPanel, Scope = window.RTLSDRScope, instances = new Map();
    const defaults = { frequency_hz: 100e6, mode: 'wfm', bandwidth_hz: 180000, gain_db: 0, agc: true, ppm: 0, squelch_db: -100, stereo: true };
    // Default filter, filter range and tuning raster of every demodulator.
    const MODES = {
        wfm: { bandwidth: 180000, min: 80000, max: 250000, snap: 10000, step: 100000 },
        nfm: { bandwidth: 12500, min: 4000, max: 25000, snap: 500, step: 12500 },
        am: { bandwidth: 9000, min: 4000, max: 25000, snap: 500, step: 9000 },
        usb: { bandwidth: 2800, min: 1000, max: 5000, snap: 100, step: 100 },
        lsb: { bandwidth: 2800, min: 1000, max: 5000, snap: 100, step: 100 },
        dab: { bandwidth: 1536000, min: 1536000, max: 1536000, snap: 1, step: 0 }
    };
    const STEPS = [100, 500, 1000, 5000, 6250, 9000, 10000, 12500, 25000, 50000, 100000, 200000, 1000000];
    const BLOCKS = ['5A', '5B', '5C', '5D', '6A', '6B', '6C', '6D', '7A', '7B', '7C', '7D', '8A', '8B', '8C', '8D', '9A', '9B', '9C', '9D', '10A', '10B', '10C', '10D', '11A', '11B', '11C', '11D', '12A', '12B', '12C', '12D', '13A', '13B', '13C', '13D', '13E', '13F'];
    const ERRORS = ['sdr_busy', 'sdr_quota_exceeded', 'sdr_read_only', 'sdr_device_missing', 'sdr_usb_permissions', 'sdr_docker_disabled', 'sdr_linux_required', 'sdr_image_unavailable', 'sdr_disabled', 'sdr_audio_unlock', 'sdr_service_user', 'sdr_local_docker_required', 'sdr_data_mount_required'];
    const SETUP_ERRORS = ERRORS.filter(key => !['sdr_busy', 'sdr_quota_exceeded', 'sdr_read_only', 'sdr_audio_unlock'].includes(key));
    const LOCAL_ACTIONS = ['mute', 'stop', 'result', 'close-result', 'dismiss'], ADMIN_ACTIONS = ['enable', 'prepare', 'save-config'];
    const STEP_KEY = 'rtl-sdr.step';

    const modeName = mode => mode === 'dab' ? 'DAB+' : String(mode || '').toUpperCase();
    const short = value => String(Number(value.toFixed(3)));
    const hertz = hz => hz >= 1e6 ? short(hz / 1e6) + ' MHz' : hz >= 1000 ? short(hz / 1000) + ' kHz' : hz + ' Hz';
    function mhz(hz) { const [whole, part = ''] = (hz / 1e6).toFixed(6).replace(/0+$/, '').split('.'); return whole + '.' + part.padEnd(3, '0'); }
    const where = tuning => tuning.mode === 'dab' ? 'DAB+ ' + (tuning.dab_block || '—') : mhz(tuning.frequency_hz) + ' MHz';
    const same = (a, b) => a.mode === b.mode && (a.mode === 'dab' ? !!a.service_id && a.service_id === b.service_id && a.dab_block === b.dab_block : a.frequency_hz === b.frequency_hz);
    function storedStep() { try { const step = Number(localStorage.getItem(STEP_KEY)); return STEPS.includes(step) ? step : 0; } catch (_) { return 0; } }

    function dispose(id) {
        const s = instances.get(id);
        if (!s) return;
        s.disposed = true; s.abort.abort(); s.view?.abort();
        clearInterval(s.receiverTimer); clearTimeout(s.tuneTimer);
        s.unsub?.(); s.preview?.pause(); s.scope?.destroy();
        instances.delete(id);
        R.present(id, false);
    }

    function render(host, id, ctx) {
        dispose(id);
        const s = { host, id, ctx, abort: new AbortController(), tab: 'receive', tuning: { ...defaults }, receiver: {}, gains: [], step: storedStep(), query: '', hold: { level: 0, at: 0 }, trace: Scope.memory(), pending: false, disposed: false, data: null };
        s.t = key => ctx.t('rtlSdr.' + key); s.esc = ctx.esc; instances.set(id, s);
        const t = key => ctx.esc(s.t(key)), signal = s.abort.signal;
        host.innerHTML = `<div class="sdr-app" data-sdr="app"><header class="sdr-header"><div class="sdr-brand"><span class="sdr-logo" aria-hidden="true">${P.icon('antenna')}</span><div><strong>RTL-SDR</strong><small>${t('subtitle')}</small></div></div>
          <nav class="sdr-tabs" aria-label="RTL-SDR">${['receive', 'recordings', 'schedules', 'setup'].map(tab => `<button type="button" class="sdr-key" data-tab="${tab}" aria-pressed="${tab === 'receive'}"${tab === 'setup' ? ` title="${t(tab)}"` : ''}>${tab === 'setup' ? P.icon('setup') : ''}<span>${t(tab)}</span></button>`).join('')}</nav>
          <div class="sdr-connection"><i data-sdr="lamp"></i><span data-sdr="connection">${t('stopped')}</span></div></header>
          <div class="sdr-notice" data-sdr="notice" role="status" hidden><span data-sdr="notice-text"></span><button type="button" class="sdr-key" data-tab="setup" data-sdr="notice-setup" hidden>${t('setup')}</button><button type="button" class="sdr-notice-close" data-action="dismiss" aria-label="${t('close')}" title="${t('close')}">${P.icon('close')}</button></div>
          <div class="sdr-priority" data-sdr="priority" role="status" hidden></div>
          <main data-sdr="content"></main><footer class="sdr-footer"><span data-sdr="footer">${t('background_hint')}</span><span class="sdr-storage"><i data-sdr="storage-bar" aria-hidden="true"></i><span data-sdr="storage"></span></span></footer></div>`;
        s.q = key => host.querySelector('[data-sdr="' + key + '"]');
        host.addEventListener('click', e => {
            const b = e.target.closest('button');
            if (!b || s.disposed) return;
            if (b.dataset.tab) { s.tab = b.dataset.tab; s.q('notice').hidden = true; content(s); }
            else if (b.dataset.action) action(s, b).catch(err => notice(s, err));
        }, { signal });
        host.addEventListener('change', e => changed(s, e.target, true), { signal });
        host.addEventListener('input', e => changed(s, e.target, false), { signal });
        host.addEventListener('keydown', e => {
            if (e.defaultPrevented || e.ctrlKey || e.metaKey || e.altKey || s.tab !== 'receive' || (e.key !== 'm' && e.key !== 'M')) return;
            if (e.target.closest('input:not([type="range"],[type="checkbox"],[type="radio"]), select, textarea')) return;
            e.preventDefault(); R.mute();
        }, { signal });
        s.unsub = R.subscribe((value, error) => {
            if (s.disposed) return;
            s.data = value;
            if (!s.initialized && value) {
                s.tuning = { ...defaults, ...value.state.tuning }; s.initialized = true;
                if (!value.config?.enabled) s.tab = 'setup';
                content(s);
            }
            update(s);
            if (error) notice(s, Error(error));
        });
        R.init(ctx).catch(err => notice(s, err)); content(s);
        R.present(id, true);
        s.receiverTimer = setInterval(() => receiver(s), 350);
    }

    function notice(s, error) {
        if (s.disposed) return;
        const key = String(error.message || error);
        s.noticeKey = key;
        s.q('notice-text').textContent = s.t(ERRORS.includes(key) ? key : 'error');
        s.q('notice-setup').hidden = s.tab === 'setup' || !SETUP_ERRORS.includes(key);
        s.q('notice').hidden = false;
    }
    function button(s, action, label, attrs = '', icon = '') { return `<button type="button" class="sdr-key" data-action="${action}" ${attrs}>${icon ? P.icon(icon) : ''}<span>${s.esc(s.t(label))}</span></button>`; }
    function iconKey(s, action, label, icon) { const text = s.esc(s.t(label)); return `<button type="button" class="sdr-key sdr-key--icon" data-action="${action}" aria-label="${text}" title="${text}">${P.icon(icon)}</button>`; }
    function field(s, key, input) { return `<label class="sdr-field"><span>${s.esc(s.t(key))}</span>${input}</label>`; }
    function value(s, name) { return s.q(name)?.value; }

    function content(s) {
        s.preview?.pause(); s.preview = null; s.signature = ''; s.stationSignature = '';
        s.scope?.destroy(); s.scope = null; s.knobs = {}; s.dial = null;
        s.view?.abort(); s.view = new AbortController();
        s.host.querySelectorAll('[data-tab]').forEach(b => b.setAttribute('aria-pressed', String(b.dataset.tab === s.tab)));
        s.q('app').dataset.view = s.tab;
        if (s.tab === 'receive') receive(s); else if (s.tab === 'setup') service(s); else library(s);
        update(s);
    }

    function receive(s) {
        const t = key => s.esc(s.t(key)), signal = s.view.signal, name = 'sdr-mode-' + String(s.id).replace(/[^\w-]/g, '');
        const knob = (key, label, attrs) => `<div class="sdr-knob-unit"><div class="sdr-knob" data-knob="${key}" title="${t(label)}"><input data-sdr="${key}" type="range" aria-label="${t(label)}" ${attrs}><span class="sdr-knob-ring" aria-hidden="true"></span><span class="sdr-knob-cap" aria-hidden="true"><i></i></span></div><span class="sdr-knob-label" aria-hidden="true">${t(label)}</span><output data-sdr="${key}-value"></output></div>`;
        const lamp = (key, body, label) => `<span data-ind="${key}"${label ? ` title="${t(label)}"` : ''}>${body}</span>`;
        s.q('content').innerHTML = `<section class="sdr-rig">
          <aside class="sdr-plate sdr-memory"><div class="sdr-plate-head"><h3>${t('stations')}</h3><button type="button" class="sdr-key sdr-key--icon" data-action="favorite" aria-pressed="false" aria-label="${t('favorite')}" title="${t('favorite')}">${P.icon('star')}${P.icon('starred')}</button></div>
            <label class="sdr-search" data-sdr="memory-search" hidden>${P.icon('scan')}<input data-sdr="memory-filter" type="search" autocomplete="off" placeholder="${t('filter_stations')}" aria-label="${t('filter_stations')}"></label>
            <div class="sdr-stations" data-sdr="stations"></div>
            <div class="sdr-scanbar"><div class="sdr-scan-progress" data-sdr="scan-progress" hidden><i></i></div><span data-sdr="scan-status" role="status"></span>${button(s, 'scan', 'scan', '', 'scan')}${button(s, 'cancel-scan', 'cancel', 'hidden')}</div></aside>
          <section class="sdr-display"><div class="sdr-ann" aria-hidden="true">${lamp('mode', '', 'mode')}${lamp('stereo', t('stereo'))}${lamp('agc', 'AGC', 'agc')}${lamp('sql', 'SQL', 'squelch')}${lamp('mute', P.icon('muted'), 'mute')}${lamp('rec', P.icon('record') + 'REC', 'recording')}${lamp('scan', P.icon('scan') + '<b data-sdr="scan-block"></b>', 'scanning')}${lamp('lock', P.icon('lock'), 'readonly')}<span class="sdr-ann-info"><span data-sdr="tuner"></span><span data-sdr="clock"></span></span></div>
            <div class="sdr-readout"><div class="sdr-tuned"><div class="sdr-frequency" data-sdr="digits" role="group" aria-label="${t('frequency')}"></div><span class="sdr-unit" aria-hidden="true">MHz</span></div><div class="sdr-station"><strong data-sdr="station">${t('ready')}</strong><span data-sdr="radiotext"></span></div></div>
            <div class="sdr-meter" data-sdr="meter" role="meter" aria-label="${t('signal')}" aria-valuemin="-100" aria-valuemax="0" aria-valuenow="-100"><span class="sdr-meter-label" aria-hidden="true">${t('signal')}</span><div class="sdr-meter-track" aria-hidden="true"><i></i><b></b></div><output data-sdr="level">— dB</output><div class="sdr-meter-scale" data-sdr="scale" aria-hidden="true"></div></div>
            <div class="sdr-scope"><div class="sdr-scope-view"><canvas data-sdr="spectrum" role="slider" tabindex="0" aria-label="${t('spectrum')}" aria-valuemin="24" aria-valuemax="1766" aria-valuenow="100"></canvas></div><div class="sdr-scope-labels"><span data-sdr="low"></span><span>${t('spectrum_hint')}</span><span data-sdr="high"></span></div></div></section>
          <aside class="sdr-plate sdr-dial"><label class="sdr-entry"><span>${t('frequency')}</span><span class="sdr-entry-row"><input data-sdr="frequency" type="text" inputmode="decimal" enterkeyhint="go" autocomplete="off" spellcheck="false"><span class="sdr-entry-unit" aria-hidden="true">MHz</span>${iconKey(s, 'apply', 'tune', 'tune')}</span></label>
            <div class="sdr-vfo"><span class="sdr-vfo-scale" aria-hidden="true"></span><button type="button" class="sdr-vfo-knob" data-sdr="knob" role="slider" aria-label="${t('tuning')}" title="${t('tuning')}" aria-valuemin="24" aria-valuemax="1766" aria-valuenow="100"><span class="sdr-vfo-grip"></span><span class="sdr-vfo-face"><i></i></span></button>${iconKey(s, 'down', 'decrease', 'left')}${iconKey(s, 'up', 'increase', 'right')}</div>
            <label class="sdr-step"><span>${t('step')}</span><select data-sdr="step"><option value="0"></option>${STEPS.map(step => `<option value="${step}">${hertz(step)}</option>`).join('')}</select></label>
            <div class="sdr-knobs">${knob('volume', 'volume', `min="0" max="1" step=".01" value="${R.volumeValue}"`)}${knob('squelch', 'squelch', 'min="-140" max="0" step="1" data-reset="-100"')}${knob('gain', 'gain', 'min="0" max="0" step="1"')}${knob('bandwidth', 'bandwidth', 'min="0" max="1" step="1"')}</div></aside>
          <div class="sdr-plate sdr-keys"><div class="sdr-keygroup">${button(s, 'play', 'listen', 'data-key="power"', 'play')}${button(s, 'stop', 'stop', '', 'stop')}</div>
            <div class="sdr-keygroup" role="radiogroup" aria-label="${t('mode')}">${Object.keys(MODES).map(mode => `<label class="sdr-key sdr-key--toggle"><input type="radio" name="${name}" data-sdr="mode" value="${mode}"><i class="sdr-led" aria-hidden="true"></i><span>${modeName(mode)}</span></label>`).join('')}</div>
            <div class="sdr-keygroup"><label class="sdr-key sdr-key--toggle" title="${t('agc')}"><input type="checkbox" data-sdr="agc" aria-label="${t('agc')}"><i class="sdr-led" aria-hidden="true"></i><span aria-hidden="true">AGC</span></label><label class="sdr-key sdr-key--toggle"><input type="checkbox" data-sdr="stereo"><i class="sdr-led" aria-hidden="true"></i><span>${t('stereo')}</span></label><button type="button" class="sdr-key sdr-key--toggle" data-action="mute" data-key="mute" aria-pressed="false" aria-label="${t('mute')}" title="${t('mute')} (M)"><i class="sdr-led" aria-hidden="true"></i>${P.icon('muted')}</button></div></div></section>`;
        const on = (target, type, fn, options) => s.q(target).addEventListener(type, fn, Object.assign({ signal }, options));
        // While the panel itself scrolls, the wheel only turns the control that has focus.
        const wheel = el => el.contains(document.activeElement) || s.q('content').scrollHeight <= s.q('content').clientHeight + 1;
        P.frequency(s.q('digits'), s.tuning.frequency_hz, power => t('digit') + ' ' + power + ' Hz');
        for (const key of ['volume', 'squelch', 'gain', 'bandwidth']) s.knobs[key] = P.knob(s.host.querySelector('[data-knob="' + key + '"]'), signal, wheel);
        s.dial = P.dial(s.q('knob'), signal, wheel, { step: n => step(s, n), commit: () => retune(s), edge: end => tuneTo(s, end ? s.receiver.maximum_hz || 1766e6 : s.receiver.minimum_hz || 24e6) });
        s.scope = Scope.create(s.q('spectrum'), { signal, memory: s.trace, label: hz => mhz(hz) + ' MHz' });
        s.gains = [];
        on('frequency', 'keydown', e => { if (e.key !== 'Enter') return; e.preventDefault(); action(s, { dataset: { action: 'apply' } }).catch(err => notice(s, err)); });
        on('spectrum', 'click', e => { const hz = s.scope.frequencyAt(e.clientX), inc = increment(s); if (hz !== null && inc) tuneTo(s, Math.round(hz / inc) * inc); });
        on('spectrum', 'keydown', e => { if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return; e.preventDefault(); step(s, e.key === 'ArrowRight' ? 1 : -1); });
        on('spectrum', 'wheel', e => { if (!wheel(e.currentTarget)) return; e.preventDefault(); step(s, e.deltaY < 0 ? 1 : -1); }, { passive: false });
        on('digits', 'keydown', e => {
            const b = e.target.closest('[data-power]');
            if (!b || b.disabled) return;
            const power = Number(b.dataset.power), digit = /^[0-9]$/.test(e.key), side = e.key === 'ArrowLeft' ? 'previousElementSibling' : e.key === 'ArrowRight' || digit ? 'nextElementSibling' : '';
            if (e.key !== 'ArrowUp' && e.key !== 'ArrowDown' && !side) return;
            e.preventDefault();
            if (digit) tuneTo(s, s.tuning.frequency_hz + (Number(e.key) - Math.floor(s.tuning.frequency_hz / power) % 10) * power);
            else if (!side) tuneTo(s, s.tuning.frequency_hz + (e.key === 'ArrowUp' ? power : -power));
            // Typing a digit moves on to the next one, like a keypad entry.
            for (let next = side && b[side]; next; next = next[side]) if (next.dataset.power) { next.focus({ preventScroll: true }); break; }
        });
        // Pulling a digit up or down tunes it, which also serves touch screens.
        let pull = null;
        on('digits', 'pointerdown', e => { const b = e.target.closest('[data-power]'); if (!b || b.disabled || e.button) return; pull = { y: e.clientY, power: Number(b.dataset.power), from: s.tuning.frequency_hz, done: 0 }; P.capture(b, e); });
        on('digits', 'pointermove', e => { const steps = pull ? Math.trunc((pull.y - e.clientY) / 12) : 0; if (!pull || steps === pull.done) return; pull.done = steps; tuneTo(s, pull.from + steps * pull.power); });
        for (const type of ['pointerup', 'pointercancel']) on('digits', type, () => { pull = null; });
        on('digits', 'wheel', e => { const b = e.target.closest('[data-power]'); if (!b || b.disabled || !wheel(e.currentTarget)) return; e.preventDefault(); tuneTo(s, s.tuning.frequency_hz + (e.deltaY < 0 ? 1 : -1) * Number(b.dataset.power)); }, { passive: false });
        controls(s); stations(s); tick(s);
    }

    function library(s) {
        const t = key => s.esc(s.t(key)), schedule = s.tab === 'schedules';
        s.q('content').innerHTML = `<section class="sdr-library"><div class="sdr-plate"><div class="sdr-plate-head"><h2>${t(s.tab)}</h2></div><p class="sdr-plate-hint">${t(schedule ? 'schedule_hint' : 'recording_hint')}</p><div class="sdr-source"><i class="sdr-led is-on" aria-hidden="true"></i><span data-sdr="job-tune"></span></div>
          <form class="sdr-job-form" data-sdr="job-form">${field(s, 'name', '<input data-sdr="name" maxlength="120" autocomplete="off">')}${field(s, 'duration', '<input data-sdr="duration" type="number" min="0.1" max="120" value="10" step="0.1">')}${schedule ? field(s, 'start', '<input data-sdr="start" type="datetime-local" required>') + field(s, 'timezone', '<input data-sdr="timezone" value="' + s.esc(Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC') + '" required>') + field(s, 'repeat', '<select data-sdr="repeat">' + ['once', 'daily', 'weekly'].map(v => '<option value="' + v + '">' + t(v) + '</option>').join('') + '</select>') : ''}<label class="sdr-field sdr-field--switch"><span>${t('transcribe')}</span><input data-sdr="transcribe" type="checkbox" checked></label><button type="submit" class="sdr-key sdr-primary">${P.icon(schedule ? 'tune' : 'record')}<span>${t(schedule ? 'save_schedule' : 'record')}</span></button></form></div>
          <div class="sdr-plate sdr-joblist"><section data-sdr="result" class="sdr-result" hidden></section><div data-sdr="jobs" class="sdr-jobs"></div></div></section>`;
        if (schedule) { const date = new Date(Date.now() + 3600000); date.setMinutes(0, 0, 0); s.q('start').value = new Date(date - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16); }
        s.q('job-form').addEventListener('submit', e => { e.preventDefault(); saveJob(s, schedule).catch(err => notice(s, err)); }, { signal: s.view.signal });
        jobs(s);
    }

    function service(s) {
        const t = key => s.esc(s.t(key));
        s.q('content').innerHTML = `<section class="sdr-service"><div class="sdr-plate"><div class="sdr-plate-head"><h2>${t('setup')}</h2><span class="sdr-chip" data-sdr="runtime"></span></div><p class="sdr-plate-hint">${t('setup_hint')}</p><div class="sdr-setup" data-sdr="setup"></div></div>
          <div class="sdr-plate"><div class="sdr-plate-head"><h2>${t('advanced')}</h2></div><p class="sdr-facts" data-sdr="facts"></p><div class="sdr-fields">${field(s, 'ppm', '<input data-sdr="ppm" type="number" min="-200" max="200" step="1">')}</div></div></section>`;
        s.q('ppm').value = s.tuning.ppm;
        setup(s);
    }

    function increment(s) { return s.tuning.mode === 'dab' ? 0 : s.step || MODES[s.tuning.mode]?.step || 0; }
    function clamp(s, f) { return Math.max(s.receiver.minimum_hz || 100000, Math.min(s.receiver.maximum_hz || 2200000000, f)); }
    function locked(s) { return s.tuning.mode === 'dab' || !!s.off; }
    function tuneTo(s, hz) { if (!locked(s)) commit(s, { frequency_hz: clamp(s, Math.round(hz)) }); }
    // Steps land on the tuning raster, so an odd frequency is pulled onto it first.
    function step(s, n) {
        const inc = increment(s), f = s.tuning.frequency_hz;
        if (inc && n) tuneTo(s, ((n > 0 ? Math.floor(f / inc) : Math.ceil(f / inc)) + n) * inc);
    }
    function commit(s, patch) {
        const moved = 'frequency_hz' in patch && patch.frequency_hz !== s.tuning.frequency_hz;
        s.tuning = { ...s.tuning, ...patch };
        if (moved && s.tuning.mode !== 'dab') delete s.tuning.label;
        controls(s); update(s); retune(s);
    }
    function retune(s) {
        clearTimeout(s.tuneTimer);
        const allowed = () => !s.disposed && R.playing && !s.data?.state?.active_job && !s.data?.state?.scanning && !s.ctx.readonly && !s.data?.state?.read_only && (s.tuning.mode !== 'dab' || !!s.tuning.service_id);
        if (!allowed()) return;
        s.tuneTimer = setTimeout(() => {
            if (!allowed()) return;
            // Changes made while the decoder switches mode must not be lost.
            if (s.pending) { retune(s); return; }
            run(s, () => R.tune({ ...s.tuning })).catch(err => notice(s, err));
        }, 400);
    }

    function changed(s, el, final) {
        const key = el.dataset?.sdr;
        if (!key || s.disposed) return;
        if (key === 'volume') { R.volume(el.value); readouts(s); }
        else if (key === 'memory-filter') { s.query = el.value.trim().toLowerCase(); stations(s); update(s); }
        else if (key === 'squelch') commit(s, { squelch_db: Number(el.value) });
        else if (key === 'bandwidth') commit(s, { bandwidth_hz: Number(el.value) });
        else if (key === 'gain') commit(s, { gain_db: s.gains[Number(el.value)] ?? 0 });
        else if (!final) return;
        else if (key === 'frequency') entry(s);
        else if (key === 'mode') { if (el.checked) mode(s, el.value); }
        else if (key === 'agc' || key === 'stereo') commit(s, { [key]: el.checked });
        else if (key === 'ppm') commit(s, { ppm: Math.max(-200, Math.min(200, Math.round(Number(el.value)) || 0)) });
        else if (key === 'step') { s.step = Number(el.value) || 0; try { localStorage.setItem(STEP_KEY, String(s.step)); } catch (_) {} controls(s); }
    }
    function entry(s) {
        const hz = Math.round(Number(String(value(s, 'frequency')).replace(',', '.')) * 1e6);
        if (Number.isFinite(hz) && hz > 0 && !locked(s)) commit(s, { frequency_hz: clamp(s, hz) }); else controls(s);
    }
    function mode(s, next) {
        const from = s.tuning.mode, st = s.data?.state || {};
        if (next === from || !MODES[next]) return;
        const tuning = { ...s.tuning, mode: next, bandwidth_hz: MODES[next].bandwidth };
        if (from !== 'dab') s.analog = s.tuning.frequency_hz;
        delete tuning.dab_block; delete tuning.service_id; delete tuning.label;
        if (from === 'dab' && s.analog) tuning.frequency_hz = s.analog;
        if (next === 'dab') {
            // DAB+ is tuned by service: return to the last one or offer the first known one.
            const station = s.dab || [...(st.favorites || []), ...(st.stations || [])].find(v => v.tuning.mode === 'dab');
            if (station) Object.assign(tuning, station.tuning, { label: station.tuning.label || station.name });
        }
        s.tuning = tuning;
        controls(s); update(s); retune(s);
    }

    function controls(s) {
        if (s.tab !== 'receive') return;
        const tune = s.tuning, setup = MODES[tune.mode] || MODES.wfm, inc = increment(s), text = mhz(tune.frequency_hz) + ' MHz';
        s.q('frequency').value = mhz(tune.frequency_hz);
        s.host.querySelectorAll('[data-sdr="mode"]').forEach(input => { input.checked = input.value === tune.mode; });
        s.q('agc').checked = tune.agc; s.q('stereo').checked = tune.stereo;
        Object.assign(s.q('bandwidth'), { min: setup.min, max: setup.max, step: setup.snap });
        s.q('bandwidth').value = tune.bandwidth_hz; s.q('squelch').value = tune.squelch_db;
        s.q('step').value = String(s.step); s.q('step').options[0].textContent = s.t('automatic') + (setup.step ? ' · ' + hertz(setup.step) : '');
        gains(s);
        P.frequency(s.q('digits'), tune.frequency_hz);
        s.q('digits').setAttribute('aria-label', s.t('frequency') + ' ' + text);
        for (const key of ['knob', 'spectrum']) { s.q(key).setAttribute('aria-valuenow', String(tune.frequency_hz / 1e6)); s.q(key).setAttribute('aria-valuetext', text); }
        s.dial.turn(inc ? tune.frequency_hz / inc : 0);
        s.scope.retune(tune);
        readouts(s); marks(s);
    }
    function gains(s) {
        const input = s.q('gain'), list = s.gains;
        input.max = Math.max(0, list.length - 1);
        input.value = list.reduce((best, gain, i) => Math.abs(gain - s.tuning.gain_db) < Math.abs(list[best] - s.tuning.gain_db) ? i : best, 0);
    }
    function readouts(s) {
        if (s.tab !== 'receive') return;
        const tune = s.tuning, st = s.data?.state || {}, r = s.receiver, shown = (st.recordings || []).find(v => v.id === st.active_job)?.tuning || tune;
        // Station data belongs to what the receiver is tuned to, not to a frequency that is only dialled in.
        const tuned = Math.abs((r.center_hz ?? shown.frequency_hz) - shown.frequency_hz) < 1000, live = R.playing || !!st.active_job, text = tuned && r.text?.trim() || '';
        const lit = { mode: true, stereo: tuned && !!r.stereo, agc: shown.agc, sql: shown.squelch_db > -100, mute: R.muted, rec: !!st.active_job, scan: !!st.scanning, lock: !!(s.ctx.readonly || st.read_only) };
        s.q('volume').value = R.volumeValue;
        s.q('volume-value').textContent = Math.round(R.volumeValue * 100) + ' %';
        s.q('squelch-value').textContent = tune.squelch_db + ' dB';
        s.q('gain-value').textContent = tune.agc ? 'AGC' : short(tune.gain_db) + ' dB';
        s.q('bandwidth-value').textContent = hertz(tune.bandwidth_hz);
        s.q('scan-block').textContent = st.scan_block || '';
        s.host.querySelectorAll('[data-ind]').forEach(el => el.classList.toggle('is-on', !!lit[el.dataset.ind]));
        s.host.querySelector('[data-ind="mode"]').textContent = modeName(shown.mode);
        s.q('station').textContent = live && tuned && r.label?.trim() || shown.label || tuned && r.label?.trim() || s.t(shown.mode === 'dab' && !shown.service_id ? 'dab_hint' : live ? 'live' : 'ready');
        s.q('radiotext').textContent = text; s.q('radiotext').title = text;
        for (const key of Object.keys(s.knobs)) { s.knobs[key].input.setAttribute('aria-valuetext', s.q(key + '-value').textContent); s.knobs[key].paint(); }
    }
    function tick(s) {
        const clock = s.tab === 'receive' && s.q('clock'), text = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
        if (clock && clock.textContent !== text) clock.textContent = text;
    }
    function readTuning(s) { if (s.tab === 'receive') entry(s); return s.tuning; }

    async function run(s, fn) { if (s.pending) return; s.pending = true; s.dismissed = ''; update(s); s.q('notice').hidden = true; try { return await fn(); } finally { s.pending = false; if (!s.disposed) { await R.refresh(); update(s); } } }
    function update(s) {
        if (!s.data) return;
        const st = s.data.state || {}, rt = s.data.runtime || {}, status = st.active_job ? 'recording' : st.scanning ? 'scanning' : R.playing ? 'live' : rt.status === 'ready' ? 'ready' : rt.status === 'preparing' ? 'preparing' : 'stopped';
        s.q('connection').textContent = s.t(status); s.q('lamp').dataset.state = status; s.q('lamp').classList.toggle('is-live', R.playing || !!st.active_job);
        s.q('app').dataset.live = String(R.playing);
        s.q('storage').textContent = (st.used_bytes / 1073741824).toFixed(2) + ' / ' + (st.quota_bytes / 1073741824).toFixed(0) + ' GB';
        s.q('storage-bar').style.setProperty('--sdr-used', String(Math.max(0, Math.min(1, st.used_bytes / (st.quota_bytes || 1)))));
        const next = (st.schedules || []).filter(x => x.enabled && Date.parse(x.next_start_at || x.start_at) > Date.now() && Date.parse(x.next_start_at || x.start_at) < Date.now() + 60000)[0];
        s.q('priority').hidden = !st.active_job && !next; s.q('priority').textContent = s.t(st.active_job ? 'recording_priority' : 'recording_soon');
        const ro = s.ctx.readonly || st.read_only || !s.data.config?.enabled, busy = !!st.active_job || !!st.scanning, dab = s.tuning.mode === 'dab';
        s.off = ro || busy;
        if (s.tab === 'receive') { stations(s); scanning(s, st); readouts(s); }
        else if (s.tab === 'setup') facts(s, rt);
        else { jobs(s); s.q('job-tune').textContent = [s.tuning.label, where(s.tuning), s.tuning.mode === 'dab' ? '' : modeName(s.tuning.mode)].filter(Boolean).join(' · '); }
        s.host.querySelectorAll('[data-action]').forEach(b => { const a = b.dataset.action; b.disabled = a !== 'dismiss' && (s.pending || (ADMIN_ACTIONS.includes(a) ? !!s.ctx.readonly : !LOCAL_ACTIONS.includes(a) && ro)); });
        s.host.querySelectorAll('.sdr-job-form button[type="submit"]').forEach(b => { b.disabled = s.pending || ro; });
        if (s.tab === 'receive') {
            const off = s.off, idle = name => s.host.querySelectorAll(name);
            idle('[data-action="mute"]').forEach(b => b.setAttribute('aria-pressed', String(R.muted)));
            idle('[data-action="scan"]').forEach(b => { b.disabled = off || s.pending; });
            idle('[data-action="play"],[data-action="station"]').forEach(b => { b.disabled = off || s.pending || b.dataset.action === 'play' && dab && !s.tuning.service_id; });
            idle('[data-action="apply"],[data-action="up"],[data-action="down"]').forEach(b => { b.disabled = off || dab || s.pending; });
            // Tuning controls stay usable while a retune is in flight; the change is queued.
            idle('[data-power],[data-sdr="frequency"],[data-sdr="knob"],[data-sdr="step"],[data-sdr="bandwidth"]').forEach(b => { b.disabled = off || dab; });
            idle('[data-sdr="mode"],[data-sdr="agc"],[data-sdr="stereo"],[data-sdr="squelch"]').forEach(b => { b.disabled = off; });
            s.q('gain').disabled = off || s.tuning.agc || s.gains.length < 2;
            for (const key of Object.keys(s.knobs)) s.knobs[key].paint();
        }
        if (s.tab === 'setup') s.host.querySelectorAll('[data-sdr="ppm"]').forEach(b => { b.disabled = ro; });
        if (rt.error && !s.pending && rt.error !== s.dismissed) notice(s, Error(rt.error));
    }
    function scanning(s, st) {
        const at = BLOCKS.indexOf(st.scan_block), bar = s.q('scan-progress'), failed = !st.scanning && st.scan_error;
        bar.hidden = !st.scanning; bar.style.setProperty('--sdr-progress', String(at < 0 ? 0 : (at + 1) / BLOCKS.length));
        s.q('scan-status').textContent = st.scanning ? s.t('scanning') + (st.scan_block ? ' · ' + st.scan_block : '') : failed ? s.t(ERRORS.includes(st.scan_error) ? st.scan_error : 'error') : '';
        s.q('scan-status').classList.toggle('sdr-warning', !!failed);
        s.host.querySelector('[data-action="cancel-scan"]').hidden = !st.scanning;
        s.host.querySelector('[data-action="scan"]').hidden = !!st.scanning;
    }
    function facts(s, rt) {
        const r = s.receiver, chip = s.q('runtime'), status = ['ready', 'preparing'].includes(rt.status) ? rt.status : 'stopped';
        chip.textContent = s.t(status); chip.dataset.state = status;
        s.q('facts').textContent = [r.device || rt.device, r.tuner, r.minimum_hz ? mhz(r.minimum_hz) + ' – ' + mhz(r.maximum_hz) + ' MHz' : ''].filter(Boolean).join(' · ');
    }

    async function setup(s) {
        if (!s.q('setup')) return;
        const t = key => s.esc(s.t(key)); let devices = [];
        try { devices = await R.request('devices', 'GET', undefined, s.abort.signal); } catch (_) {}
        if (s.disposed || !s.q('setup')) return;
        const cfg = s.data?.config || {};
        s.q('setup').innerHTML = `<div class="sdr-fields">${field(s, 'device', '<select data-sdr="device"><option value="">' + t('automatic') + '</option>' + devices.map(d => '<option value="' + s.esc(d.id) + '">' + s.esc(d.name + ' · ' + d.id) + '</option>').join('') + '</select>')}${field(s, 'quota', '<input data-sdr="quota-gb" type="number" min="1" max="1000" value="' + (cfg.quota_gb || 10) + '">')}<label class="sdr-field sdr-field--switch"><span>${t('allow_agent')}</span><input data-sdr="allow-agent" type="checkbox" ${cfg.allow_agent ? 'checked' : ''}></label><label class="sdr-field sdr-field--switch"><span>${t('readonly')}</span><input data-sdr="readonly" type="checkbox" ${cfg.read_only ? 'checked' : ''}></label></div><div class="sdr-setup-actions">${button(s, 'enable', cfg.enabled ? 'disable' : 'enable', cfg.enabled ? '' : 'data-key="primary"')}${button(s, 'prepare', 'prepare')}${button(s, 'save-config', 'save')}</div>`;
        s.q('device').value = cfg.device || '';
        update(s);
    }
    function stations(s) {
        const target = s.q('stations');
        if (!target) return;
        const st = s.data?.state || {}, all = [...(st.favorites || []).map(v => ({ ...v, favorite: true })), ...(st.stations || [])];
        const items = s.query ? all.filter(v => (v.name + ' ' + where(v.tuning) + ' ' + modeName(v.tuning.mode)).toLowerCase().includes(s.query)) : all, sig = JSON.stringify([items, s.query]);
        s.q('memory-search').hidden = all.length < 9 && !s.query;
        if (sig === s.stationSignature && target.children.length) { marks(s); return; }
        s.stationSignature = sig;
        target.innerHTML = items.length ? items.map((v, i) => '<div class="sdr-preset' + (v.favorite ? ' is-favorite' : '') + '"><button type="button" class="sdr-preset-main" data-action="station" data-id="' + s.esc(v.id) + '"><b aria-hidden="true">' + String(i + 1).padStart(2, '0') + '</b><span>' + s.esc(v.name) + '</span><small>' + s.esc(where(v.tuning) + (v.tuning.mode === 'dab' ? '' : ' · ' + modeName(v.tuning.mode))) + '</small></button>' + (v.favorite ? '<button type="button" class="sdr-preset-remove" data-action="delete-favorite" data-id="' + s.esc(v.id) + '" aria-label="' + s.esc(s.t('remove') + ': ' + v.name) + '" title="' + s.esc(s.t('remove')) + '">' + P.icon('close') + '</button>' : '') + '</div>').join('') : '<p class="sdr-empty">' + s.esc(s.t(all.length ? 'no_match' : s.tuning.mode === 'dab' ? 'dab_hint' : 'stations_empty')) + '</p>';
        marks(s);
    }
    // The preset that matches the current tuning glows like a selected memory channel.
    function marks(s) {
        const st = s.data?.state || {}, all = [...(st.favorites || []), ...(st.stations || [])];
        s.host.querySelectorAll('[data-action="station"]').forEach(b => { const item = all.find(v => v.id === b.dataset.id); b.parentElement.classList.toggle('is-active', !!item && same(item.tuning, s.tuning)); });
        s.host.querySelectorAll('[data-action="favorite"]').forEach(b => b.setAttribute('aria-pressed', String((st.favorites || []).some(v => same(v.tuning, s.tuning)))));
    }
    function jobs(s) {
        const st = s.data?.state || {}, rows = s.tab === 'schedules' ? st.schedules || [] : st.recordings || [], sig = JSON.stringify([rows, st.active_job]);
        if (s.signature === sig) return;
        s.signature = sig;
        s.q('jobs').innerHTML = rows.length ? rows.slice().reverse().map(r => `<article class="sdr-job" data-status="${s.esc(r.id === st.active_job ? 'recording' : r.status || (r.enabled ? 'scheduled' : 'off'))}"><i class="sdr-led" aria-hidden="true"></i><div class="sdr-job-body"><strong>${s.esc(r.name || r.tuning?.label || s.t('untitled'))}</strong><small>${s.esc(new Date(s.tab === 'schedules' && r.next_start_at && !r.next_start_at.startsWith('0001-') ? r.next_start_at : r.start_at).toLocaleString())} · ${s.esc(s.t(r.status || r.repeat))} · ${Math.round((r.duration_seconds || 0) / 60)} ${s.esc(s.t('minutes'))}</small><small class="sdr-job-tune">${s.esc(r.tuning ? where(r.tuning) + (r.tuning.mode === 'dab' ? '' : ' · ' + modeName(r.tuning.mode)) : '')}</small>${r.error || r.asr_error ? '<span class="sdr-warning">' + s.esc(s.t(r.asr_error ? 'asr_failed' : r.status === 'missed' ? 'missed' : r.error === 'sdr_device_lost' ? 'sdr_device_lost' : 'partial_hint')) + '</span>' : ''}</div><div class="sdr-job-actions">${s.tab === 'recordings' ? button(s, 'result', 'details', 'data-id="' + s.esc(r.id) + '"') + (r.id === st.active_job ? button(s, 'stop-recording', 'stop', 'data-id="' + s.esc(r.id) + '"', 'stop') : '') : ''}${button(s, s.tab === 'recordings' ? 'delete-recording' : 'delete-schedule', 'remove', 'data-id="' + s.esc(r.id) + '"', 'trash')}</div></article>`).join('') : '<p class="sdr-empty">' + s.esc(s.t(s.tab === 'schedules' ? 'schedules_empty' : 'recordings_empty')) + '</p>';
    }
    async function saveJob(s, schedule) {
        return run(s, async () => {
            const body = { name: value(s, 'name'), tuning: s.tuning, duration_seconds: Math.round(Number(value(s, 'duration')) * 60), transcribe: s.q('transcribe').checked };
            if (schedule) { const local = value(s, 'start'), zone = value(s, 'timezone'); body.timezone = zone; body.start_at = zonedISO(local, zone); body.repeat = value(s, 'repeat'); body.enabled = true; }
            await R.request(schedule ? 'schedules' : 'recordings', 'POST', body);
        });
    }
    function zonedISO(local, zone) { const desired = new Date(local + 'Z').getTime(); if (!Number.isFinite(desired)) throw Error('invalid'); const formatter = new Intl.DateTimeFormat('en-CA', { timeZone: zone, year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23' }); let instant = desired; for (let i = 0; i < 3; i++) { const p = Object.fromEntries(formatter.formatToParts(new Date(instant)).map(x => [x.type, x.value])); const rendered = Date.UTC(+p.year, +p.month - 1, +p.day, +p.hour, +p.minute, +p.second); if (rendered === desired) return new Date(instant).toISOString(); instant += desired - rendered; } throw Error('invalid'); }
    async function result(s, id) {
        const r = await R.request('recordings/' + id);
        if (s.disposed || !s.q('result')) return;
        const box = s.q('result'), audio = '/api/desktop/rtl-sdr/recordings/' + encodeURIComponent(id) + '/audio';
        box.hidden = false;
        box.innerHTML = '<div class="sdr-result-head"><h3>' + s.esc(r.name || s.t('untitled')) + '</h3>' + iconKey(s, 'close-result', 'close', 'close') + '</div>' + (r.bytes ? '<audio controls preload="metadata" src="' + audio + '"></audio><div class="sdr-result-actions"><a class="sdr-key" href="' + audio + '?download=1">' + P.icon('download') + '<span>' + s.esc(s.t('download')) + '</span></a>' + button(s, 'transcribe', 'retry_asr', 'data-id="' + s.esc(id) + '"') + '</div>' : '') + '<div class="sdr-transcript">' + (r.segments || []).map(seg => '<p><time>' + Math.floor(seg.start_seconds / 60) + ':' + String(Math.floor(seg.start_seconds % 60)).padStart(2, '0') + '</time><span>' + s.esc(seg.error ? s.t('asr_failed') : seg.text || s.t('no_speech')) + '</span></p>').join('') + '</div>';
        s.preview = box.querySelector('audio'); update(s); box.scrollIntoView({ block: 'nearest' });
    }
    async function action(s, b) {
        const a = b.dataset.action, id = b.dataset.id;
        if (a === 'dismiss') { s.dismissed = s.noticeKey; s.q('notice').hidden = true; return; }
        if (a === 'mute') { R.mute(); return; }
        if (a === 'up' || a === 'down') { step(s, a === 'up' ? 1 : -1); return; }
        if (a === 'close-result') { s.preview?.pause(); s.q('result').hidden = true; return; }
        if (a === 'result') return result(s, id);
        return run(s, async () => {
            if (a === 'play' || a === 'apply') { clearTimeout(s.tuneTimer); readTuning(s); clearTimeout(s.tuneTimer); await R.tune({ ...s.tuning }); }
            else if (a === 'stop') { clearTimeout(s.tuneTimer); await R.stop(); }
            else if (a === 'scan') await R.request('scan', 'POST', {});
            else if (a === 'cancel-scan') await R.request('scan', 'DELETE');
            else if (a === 'favorite') {
                // The star works like a bookmark: a second press removes the saved station again.
                readTuning(s); clearTimeout(s.tuneTimer);
                const saved = (s.data.state.favorites || []).find(v => same(v.tuning, s.tuning));
                if (saved) await R.request('favorites/' + saved.id, 'DELETE');
                else await R.request('favorites', 'POST', { name: s.receiver.label?.trim() || s.tuning.label || where(s.tuning), tuning: s.tuning });
            }
            else if (a === 'station') { const st = s.data.state, item = [...(st.favorites || []), ...(st.stations || [])].find(x => x.id === id); if (item) { clearTimeout(s.tuneTimer); if (item.tuning.mode === 'dab') s.dab = item; s.tuning = { ...item.tuning, label: item.tuning.label || item.name }; controls(s); await R.tune({ ...s.tuning }); } }
            else if (a === 'delete-favorite') await R.request('favorites/' + id, 'DELETE');
            else if (a === 'stop-recording') await R.request('recordings/' + id + '/stop', 'POST', {});
            else if (a === 'transcribe') await R.request('recordings/' + id + '/transcribe', 'POST', {});
            else if (a === 'delete-recording' || a === 'delete-schedule') { if (await s.ctx.confirmDialog(s.t('remove'), s.t('delete_confirm'))) await R.request((a === 'delete-recording' ? 'recordings/' : 'schedules/') + id, 'DELETE'); }
            else if (a === 'prepare') await R.request('setup', 'POST', {});
            else if (a === 'enable' || a === 'save-config') { const body = { device: value(s, 'device'), quota_gb: Number(value(s, 'quota-gb')), allow_agent: s.q('allow-agent').checked, read_only: s.q('readonly').checked }; if (a === 'enable') body.enabled = !s.data.config.enabled; await R.request('config', 'PUT', body); await R.refresh(); await setup(s); }
        });
    }
    async function receiver(s) {
        if (s.disposed) return;
        tick(s);
        // A minimized window or one on another space leaves control to the desktop mini control.
        const shown = s.q('app').checkVisibility?.({ visibilityProperty: true }) ?? true;
        R.present(s.id, shown);
        if (!shown || s.tab !== 'receive' || s.receiving || document.hidden || !s.data?.config?.enabled) return;
        s.receiving = true;
        try {
            const r = await R.request('receiver', 'GET', undefined, s.abort.signal);
            if (s.disposed || s.tab !== 'receive') return;
            s.receiver = r;
            const st = s.data?.state || {}, actual = (st.recordings || []).find(v => v.id === st.active_job)?.tuning || s.tuning, dab = actual.mode === 'dab';
            P.frequency(s.q('digits'), actual.frequency_hz);
            s.q('tuner').textContent = r.tuner || '';
            meter(s, dab ? Number(r.snr_db || 0) : Number(r.power_db ?? -100), dab);
            if (JSON.stringify(r.gains_db) !== JSON.stringify(s.gains)) { s.gains = r.gains_db || []; gains(s); update(s); }
            readouts(s);
            s.scope.push(r, actual);
            const edges = s.scope.edges();
            if (edges) { s.q('low').textContent = mhz(edges[0]); s.q('high').textContent = mhz(edges[1]) + ' MHz'; }
        } catch (_) {} finally { s.receiving = false; }
    }
    function meter(s, db, snr) {
        const box = s.q('meter'), low = snr ? 0 : -100, high = snr ? 40 : 0, level = Math.max(0, Math.min(1, (db - low) / (high - low))), now = Date.now();
        if (level >= s.hold.level) s.hold = { level, at: now }; else if (now - s.hold.at > 1200) s.hold.level = Math.max(level, s.hold.level - .03);
        if (box.dataset.scale !== String(snr)) { box.dataset.scale = String(snr); s.q('scale').innerHTML = [0, 1, 2, 3, 4, 5].map(i => '<span>' + (low + i * (high - low) / 5) + '</span>').join(''); box.setAttribute('aria-valuemin', low); box.setAttribute('aria-valuemax', high); }
        box.style.setProperty('--sdr-level', (level * 100).toFixed(1) + '%'); box.style.setProperty('--sdr-peak', (s.hold.level * 100).toFixed(1) + '%');
        box.setAttribute('aria-valuenow', db.toFixed(1)); box.setAttribute('aria-valuetext', db.toFixed(1) + (snr ? ' dB SNR' : ' dB'));
        s.q('level').textContent = db.toFixed(1) + (snr ? ' dB SNR' : ' dB');
    }
    window.RTLSDRApp = { render, dispose };
})();
