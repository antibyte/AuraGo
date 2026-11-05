// Observed durable capability and run metadata; execution remains owned by the existing Runner.
(function () {
    'use strict';
    const ED = window.EasyDrag = window.EasyDrag || {};
    const TYPE = 'logic.durable_wait';
    const REASONS = new Set(['disabled', 'authority', 'key_unavailable', 'checkpoint_unreadable', 'storage_unavailable', 'incompatible', 'service_unavailable']);
    const object = value => !!value && typeof value === 'object' && !Array.isArray(value);
    const count = value => value === null || Number.isSafeInteger(value) && value >= 0 && value <= 1000000000;
    function capability(value) {
        if (!object(value) || !['configured', 'effective', 'recovery_available'].every(key => typeof value[key] === 'boolean') || !['waiting_count', 'claimed_count', 'blocked_count'].every(key => count(value[key])) || value.reason !== undefined && !REASONS.has(value.reason)) return null;
        const result = {};
        ['configured', 'effective', 'recovery_available', 'waiting_count', 'claimed_count', 'blocked_count'].forEach(key => { result[key] = value[key]; });
        if (value.reason !== undefined) result.reason = value.reason;
        return result;
    }
    function status(run, value) {
        if (!run || !object(value) || value.run_id !== run.id || !['waiting', 'running', 'blocked'].includes(value.state) || !Number.isSafeInteger(value.generation) || value.generation < 0 || typeof value.cancel_available !== 'boolean' || value.blocked_code !== undefined && (!REASONS.has(value.blocked_code) || value.blocked_code === 'service_unavailable')) return null;
        for (const key of ['wake_at', 'deadline']) {
            if (value[key] !== undefined && (typeof value[key] !== 'string' || value[key].length > 64 || !/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,9})?Z$/.test(value[key]) || !Number.isFinite(Date.parse(value[key])))) return null;
        }
        const result = {};
        ['run_id', 'state', 'generation', 'cancel_available', 'blocked_code', 'wake_at', 'deadline'].forEach(key => { if (value[key] !== undefined) result[key] = value[key]; });
        return result;
    }
    function record(run) { return run && (run.record || run); }
    function info(run) { const r = record(run); return status(r, run && run.durableStatus); }
    function has(run) { return !!(record(run)?.durable === true || run?.durableKnown || info(run)); }
    function label(t, run) {
        const r = record(run), s = info(run);
        if (!r) return '';
        if (run.durableStopping && !['success', 'error', 'cancelled'].includes(r.status)) return t('easydrag.ui.run_stopping');
        // A legacy checkpoint can outlive a terminal run. Keep that run's actual result.
        if (!['success', 'error', 'cancelled'].includes(r.status) && s && s.state !== 'running') return t(s.state === 'blocked' ? 'easydrag.ui.durable_blocked' : 'easydrag.ui.durable_waiting');
        return ED.core.tr(t, 'easydrag.ui.status_' + r.status, r.status);
    }
    function reason(t, code) { return REASONS.has(code) ? t('easydrag.ui.durable_reason_' + code) : t('easydrag.ui.durable_unknown'); }
    function markup(env, run) {
        if (!has(run)) return '';
        const { t, esc } = env, s = info(run);
        return '<section class="ed-durable-status" data-ed-durable-run aria-label="' + esc(t('easydrag.ui.durable_title')) + '">' +
            ((!s && !['success', 'error', 'cancelled'].includes(record(run).status)) || run.durableUnknown ? '<p class="ed-hint" role="status">' + esc(t('easydrag.ui.durable_unknown')) + '</p>' : '') +
            (s && s.state === 'blocked' ? '<p class="ed-callout ed-callout--warn">' + esc(reason(t, s.blocked_code)) + '</p>' : '') +
            (s && (s.wake_at || s.deadline) ? '<dl class="ed-durable-times">' + (s.wake_at ? '<dt>' + esc(t('easydrag.ui.durable_wake')) + '</dt><dd>' + esc(ED.core.fmt.dateTime(s.wake_at)) + '</dd>' : '') + (s.deadline ? '<dt>' + esc(t('easydrag.ui.durable_deadline')) + '</dt><dd>' + esc(ED.core.fmt.dateTime(s.deadline)) + '</dd>' : '') + '</dl>' : '') +
            '<p class="ed-hint">' + esc(t('easydrag.ui.durable_no_followups')) + '</p>' + (!['success', 'error', 'cancelled'].includes(record(run).status) || s?.cancel_available ? '<button type="button" class="ed-btn ed-btn--small" data-ed-durable-check>' + esc(t('easydrag.ui.reload')) + '</button>' : '') + '</section>';
    }
    function supported(env) { return !!env.catalog?.types?.has(TYPE) || !!env.model?.doc?.nodes?.some(node => node.type === TYPE); }
    function requires(doc) { return !!doc?.nodes?.some(node => node.type === TYPE); }
    // Transport and timers only. The run controller owns snapshot adoption and execution state.
    function observeRun(env) {
        let reading = null, timer = 0, disposed = false;
        function reset() {
            if (reading) { clearTimeout(reading.timeout); reading.controller.abort(); reading = null; }
        }
        function read(id) {
            if (reading && reading.id === id) return reading.promise;
            reset();
            const owned = reading = { id, controller: new AbortController(), promise: null, timeout: 0 };
            const deadline = new Promise((_, reject) => {
                owned.timeout = setTimeout(() => {
                    owned.controller.abort();
                    reject(Object.assign(new Error('status read timed out'), { body: { code: 'FLOW_DURABLE_UNAVAILABLE' } }));
                }, 15000);
            });
            owned.promise = Promise.race([env.api.run(id, false, owned.controller.signal), deadline]).finally(() => {
                clearTimeout(owned.timeout);
                if (reading === owned) reading = null;
            });
            return owned.promise;
        }
        function watch() {
            const current = env.current();
            const needs = !disposed && has(current) && !current.durableAccessLost && (!env.isFinal(current.status) || info(current)?.cancel_available);
            if (!needs) { clearTimeout(timer); timer = 0; return; }
            if (timer) return;
            timer = setTimeout(async () => {
                timer = 0;
                if (disposed || env.current() !== current) { watch(); return; }
                if (document.hidden || !env.root.isConnected || typeof env.ctx.isActive === 'function' && !env.ctx.isActive()) { watch(); return; }
                try {
                    const detail = await read(current.id);
                    if (!disposed && env.current() === current) env.onDetail(detail, current);
                } catch (err) {
                    if (!disposed && env.current() === current && err.name !== 'AbortError') env.onError(current, err);
                } finally { if (!disposed && env.current() === current) watch(); }
            }, 5000);
        }
        return { read, watch, reset, dispose() { disposed = true; clearTimeout(timer); reset(); } };
    }
    function observe(env) {
        let value = null, last = null, request = null, disposed = false, observed = false, sequence = 0, readAt = 0, timer = 0;
        const core = ED.core, bag = core.bag();
        const el = core.el('<section class="ed-callout ed-durable-capability" data-ed-durable-capability hidden><div></div><button type="button" class="ed-btn ed-btn--small" data-ed-durable-refresh></button><a class="ed-btn ed-btn--small" href="/config#flows" target="_blank" rel="noopener"></a></section>');
        function render() {
            const s = value || last;
            el.hidden = !observed || !!(value && !value.configured && !value.effective && value.waiting_count === 0 && value.claimed_count === 0 && value.blocked_count === 0);
            el.querySelector('[data-ed-durable-refresh]').disabled = !!request;
            el.querySelector('[data-ed-durable-refresh]').textContent = env.t('easydrag.ui.reload');
            el.querySelector('a').textContent = env.t('easydrag.ui.open_settings');
            const unknown = '—';
            el.querySelector('div').innerHTML = '<strong>' + env.esc(env.t('easydrag.ui.durable_title')) + '</strong><p>' + env.esc(value && s.effective && s.recovery_available ? env.t('easydrag.ui.durable_no_followups') : value ? reason(env.t, s.reason) : env.t('easydrag.ui.durable_unknown')) + '</p>' +
                '<p class="ed-hint">' + env.esc(env.t('easydrag.ui.durable_counts', { waiting: s && s.waiting_count !== null ? s.waiting_count : unknown, running: s && s.claimed_count !== null ? s.claimed_count : unknown, blocked: s && s.blocked_count !== null ? s.blocked_count : unknown })) + '</p>';
        }
        async function refresh(force) {
            if (disposed) return null;
            if (request) return request.promise;
            if (!force && value && Date.now() - readAt < 5000) return value;
            const controller = new AbortController(), mine = ++sequence;
            const owned = request = { controller, promise: null };
            render();
            owned.promise = (async () => {
                let next = null;
                try { next = capability(await env.api.runtimeStatus(controller.signal)); }
                catch (err) {
                    // Access loss also clears the last protected count snapshot.
                    if ([401, 403].includes(Number(err && err.status))) last = null;
                }
                if (disposed || mine !== sequence || controller.signal.aborted) return null;
                value = next;
                observed = true;
                readAt = Date.now();
                if (next) last = next;
                request = null; render();
                if (env.bus) env.bus.emit('runtime', value);
                clearTimeout(timer);
                timer = setTimeout(() => {
                    if (disposed) return;
                    if (!document.hidden && env.root.isConnected) refresh(true);
                    else { readAt = 0; refreshWhenVisible(); }
                }, 5000);
                return value;
            })();
            return owned.promise;
        }
        function refreshWhenVisible() {
            clearTimeout(timer);
            timer = setTimeout(() => { if (disposed) return; if (!document.hidden && env.root.isConnected) refresh(true); else refreshWhenVisible(); }, 5000);
        }
        bag.listen(el.querySelector('[data-ed-durable-refresh]'), 'click', refresh);
        render();
        return { el, refresh, get value() { return value; }, dispose() { disposed = true; sequence++; clearTimeout(timer); if (request) request.controller.abort(); request = null; bag.dispose(); el.remove(); } };
    }
    ED.durable = { TYPE, REASONS, capability, status, record, info, has, label, reason, markup, supported, requires, observeRun, observe };
})();
