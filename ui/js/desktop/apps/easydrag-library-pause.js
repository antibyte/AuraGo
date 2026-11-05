(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const PAUSE_LIMIT = 50;
    const PAUSE_REQUEST_MS = 8000;
    const PAUSE_REVIEW_MS = 60000;
    const PAUSE_OPERATION_MS = 120000;

    function abortError() { return Object.assign(new Error('request cancelled'), { name: 'AbortError' }); }
    function deadlineError() { return Object.assign(new Error('request deadline exceeded'), { name: 'TimeoutError' }); }
    function notSentError(cancelled, permissionLost) { return Object.assign(new Error('pause request was not sent'), { pauseNotSent: true, pauseCancelled: !!cancelled, pausePermissionLost: !!permissionLost }); }
    function errorCode(err) { return String(err && err.body && (err.body.code || err.body.error) || '').toLowerCase(); }
    function authorityLost(err) {
        return [401, 403].includes(Number(err && err.status)) || ['unauthorized', 'forbidden', 'permission_denied', 'desktop_scope_required', 'desktop_readonly', 'flow_permission_denied', 'flows_disabled', 'flow_request_cancelled'].includes(errorCode(err));
    }
    function stateFromError(err) {
        if (Number(err && err.status) === 404 || ['flow_not_found', 'flow_mission_missing'].includes(errorCode(err))) return 'missing';
        if (['flow_mission_mismatch', 'flow_mission_wrong_owner'].includes(errorCode(err))) return 'mismatch';
        return 'unavailable';
    }
    function missionState(reply) {
        const state = reply && reply.mission_state;
        return ['enabled', 'disabled', 'missing', 'mismatch', 'unavailable'].includes(state) ? state : 'unavailable';
    }
    async function pauseRequest(fn, signals, timeoutMs) {
        if (signals.some(signal => signal && signal.aborted)) throw abortError();
        const controller = new AbortController();
        let timer, rejectCancel;
        const cancel = new Promise((_, reject) => { rejectCancel = reject; });
        const listeners = signals.filter(Boolean).map(signal => {
            const listener = () => { controller.abort(); rejectCancel(abortError()); };
            signal.addEventListener('abort', listener, { once: true });
            return () => signal.removeEventListener('abort', listener);
        });
        const deadline = new Promise((_, reject) => {
            timer = setTimeout(() => { controller.abort(); reject(deadlineError()); }, timeoutMs);
        });
        const request = Promise.resolve().then(() => {
            if (controller.signal.aborted) throw abortError();
            return fn(controller.signal);
        });
        try { return await Promise.race([request, cancel, deadline]); }
        finally { clearTimeout(timer); listeners.forEach(remove => remove()); }
    }
    async function reviewPauseTargets(api, targets, options) {
        const opts = options || {}, endAt = Date.now() + (opts.totalMs || PAUSE_REVIEW_MS), rows = [];
        let stopped = false, permissionLost = false;
        for (let index = 0; index < targets.length; index++) {
            const target = targets[index];
            if (opts.signal && opts.signal.aborted) return { rows, stopped: true, cancelled: true, permissionLost };
            if (Date.now() >= endAt) {
                stopped = true;
                rows.push(...targets.slice(index).map(item => ({ ...item, state: 'unavailable' })));
                break;
            }
            if (opts.onProgress) opts.onProgress(index + 1, targets.length, target);
            try {
                const reply = await pauseRequest(signal => api.enabledStatus(target.id, signal), [opts.signal, opts.destroySignal], Math.min(opts.requestMs || PAUSE_REQUEST_MS, Math.max(1, endAt - Date.now())));
                rows.push({ ...target, state: missionState(reply) });
                if (missionState(reply) === 'unavailable') stopped = true;
            } catch (err) {
                if (opts.signal && opts.signal.aborted || opts.destroySignal && opts.destroySignal.aborted) return { rows, stopped: true, cancelled: true, permissionLost };
                const state = stateFromError(err);
                rows.push({ ...target, state });
                if (authorityLost(err)) { permissionLost = true; stopped = true; rows.push(...targets.slice(index + 1).map(item => ({ ...item, state: 'unavailable' }))); break; }
            }
        }
        return { rows, stopped, cancelled: false, permissionLost };
    }
    async function executePauseTargets(api, reviewed, options) {
        const opts = options || {}, rows = reviewed.map(row => ({ ...row, outcome: row.state === 'disabled' ? 'disabled' : row.state === 'enabled' ? 'not_attempted' : row.state }));
        const endAt = Date.now() + (opts.totalMs || PAUSE_OPERATION_MS), candidates = rows.filter(row => row.state === 'enabled');
        let stopped = false, permissionLost = false, cancelled = false;
        const finishUnsent = from => {
            candidates.slice(from).forEach(row => { row.outcome = 'not_attempted'; });
        };
        for (let index = 0; index < candidates.length; index++) {
            const target = candidates[index], row = rows.find(item => item.id === target.id);
            if (opts.cancelled && opts.cancelled() || opts.cancelSignal && opts.cancelSignal.aborted) { cancelled = true; stopped = true; finishUnsent(index); break; }
            if (opts.destroySignal && opts.destroySignal.aborted || Date.now() >= endAt || opts.canWrite && !opts.canWrite()) {
                stopped = true; finishUnsent(index); break;
            }
            if (opts.onProgress) opts.onProgress(index + 1, candidates.length, target, 'check');
            let state;
            try {
                const reply = await pauseRequest(signal => api.enabledStatus(target.id, signal), [opts.cancelSignal, opts.destroySignal], Math.min(opts.requestMs || PAUSE_REQUEST_MS, Math.max(1, endAt - Date.now())));
                state = missionState(reply);
            } catch (err) {
                if (opts.cancelled && opts.cancelled() || opts.cancelSignal && opts.cancelSignal.aborted) { cancelled = true; stopped = true; row.outcome = 'not_attempted'; finishUnsent(index + 1); break; }
                state = stateFromError(err);
                if (state === 'unavailable' || authorityLost(err)) {
                    row.outcome = 'unresolved'; stopped = true; permissionLost = authorityLost(err); finishUnsent(index + 1); break;
                }
            }
            row.state = state;
            if (state === 'unavailable') { row.outcome = 'unresolved'; stopped = true; finishUnsent(index + 1); break; }
            if (state !== 'enabled') { row.outcome = state; continue; }

            let acknowledged = false, mutationError = null;
            if (opts.cancelled && opts.cancelled() || opts.cancelSignal && opts.cancelSignal.aborted) {
                cancelled = true; stopped = true; row.outcome = 'not_attempted'; finishUnsent(index + 1); break;
            }
            if (opts.destroySignal && opts.destroySignal.aborted || Date.now() >= endAt || opts.canWrite && !opts.canWrite()) {
                stopped = true; finishUnsent(index); break;
            }
            try {
                if (opts.onProgress) opts.onProgress(index + 1, candidates.length, target, 'pause');
                await pauseRequest(signal => {
                    const closed = opts.cancelled && opts.cancelled() || opts.cancelSignal && opts.cancelSignal.aborted;
                    if (closed) throw notSentError(true, false);
                    const writable = !opts.canWrite || opts.canWrite();
                    if (opts.destroySignal && opts.destroySignal.aborted || Date.now() >= endAt || !writable) throw notSentError(false, !writable);
                    return api.setEnabled(target.id, false, signal);
                }, [opts.destroySignal], Math.min(opts.requestMs || PAUSE_REQUEST_MS, Math.max(1, endAt - Date.now())));
                acknowledged = true;
            } catch (err) {
                if (err && err.pauseNotSent) {
                    if (err.pauseCancelled) cancelled = true;
                    if (err.pausePermissionLost) permissionLost = true;
                    stopped = true; finishUnsent(index); break;
                }
                mutationError = err;
            }

            // A sent pause is never retried. Even a lost reply gets exactly one readback.
            try {
                const reply = await pauseRequest(signal => api.enabledStatus(target.id, signal), [opts.destroySignal], Math.min(opts.requestMs || PAUSE_REQUEST_MS, Math.max(1, endAt - Date.now())));
                const after = missionState(reply);
                row.state = after;
                row.outcome = after === 'disabled' ? (acknowledged ? 'confirmed' : 'save_uncertain') : after === 'enabled' ? 'active' : 'unresolved';
                if (row.outcome === 'unresolved') stopped = true;
            } catch (err) {
                row.state = stateFromError(err);
                row.outcome = 'unresolved';
                stopped = true;
                permissionLost = authorityLost(err);
            }
            if (mutationError) {
                stopped = true;
                permissionLost = permissionLost || authorityLost(mutationError);
            }
            if (stopped) { finishUnsent(index + 1); break; }
        }
        return { rows, stopped, cancelled, permissionLost };
    }

    ED.libraryPause = { LIMIT: PAUSE_LIMIT, review: reviewPauseTargets, execute: executePauseTargets };
})();
