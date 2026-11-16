(function () {
    'use strict';

    const participants = new Map();
    const defaults = {
        getWindows: () => [],
        canLogout: () => false,
        logout: async () => false,
        onProblem: () => {}
    };
    let config = { ...defaults };
    let running = null;

    function configure(options = {}) {
        config = {
            getWindows: typeof options.getWindows === 'function' ? options.getWindows : config.getWindows,
            canLogout: typeof options.canLogout === 'function' ? options.canLogout : config.canLogout,
            logout: typeof options.logout === 'function' ? options.logout : config.logout,
            onProblem: typeof options.onProblem === 'function' ? options.onProblem : config.onProblem
        };
        return api;
    }

    function register(windowId, participant) {
        const id = String(windowId || '').trim();
        if (!id) throw new TypeError('A session action requires a window id');
        if (participant == null) {
            participants.delete(id);
            return () => {};
        }
        if (typeof participant.inspect !== 'function' ||
            typeof participant.decide !== 'function' ||
            typeof participant.save !== 'function') {
            throw new TypeError('A session action requires inspect, decide, and save functions');
        }
        participants.set(id, participant);
        return () => {
            if (participants.get(id) === participant) participants.delete(id);
        };
    }

    function requestLogout() {
        if (running) return running;
        const wrapped = logoutOnce().finally(() => {
            if (running === wrapped) running = null;
        });
        running = wrapped;
        return wrapped;
    }

    async function logoutOnce() {
        if (!await allowed()) return false;
        let windows;
        try {
            windows = await currentWindows();
        } catch (_) {
            return problem('state_unavailable');
        }

        const records = [];
        for (const win of windows) {
            const participant = participants.get(win.id);
            if (win.hasBeforeClose && !participant) {
                return problem('unknown_guard', win);
            }
            if (!participant) continue;
            let state;
            try {
                state = await inspect(participant);
            } catch (_) {
                return problem('state_unavailable', win);
            }
            records.push({ win, participant, state, decision: state.dirty ? null : 'keep' });
        }

        // Collect every choice before any save, draft mutation, or close-side effect.
        for (const record of records) {
            if (!record.state.dirty) continue;
            let choice;
            try {
                choice = await record.participant.decide(publicSnapshot(record.win, record.state));
            } catch (_) {
                return problem('state_unavailable', record.win);
            }
            if (choice === 'cancel') return false;
            if (choice !== 'save' && choice !== 'keep') return problem('state_unavailable', record.win);
            record.decision = choice;
        }

        if (!await validate(records, windows)) return false;
        if (!await allowed()) return false;

        for (const record of records) {
            if (record.decision !== 'save' || !record.state.dirty) continue;
            if (!await validate(records, windows)) return false;
            let result;
            try {
                result = await record.participant.save(record.state.revision);
            } catch (_) {
                return problem('save_failed', record.win);
            }
            if (!result || result.revision !== record.state.revision) {
                return problem('save_failed', record.win);
            }
            let state;
            try {
                state = await inspect(record.participant);
            } catch (_) {
                return problem('state_unavailable', record.win);
            }
            if (state.revision !== result.revision) return problem('content_changed', record.win);
            if (state.dirty) return problem('save_failed', record.win);
            record.state = state;
        }

        if (!await validate(records, windows)) return false;
        if (!await allowed()) return false;
        try {
            return (await config.logout()) === true;
        } catch (_) {
            return problem('logout_failed');
        }
    }

    async function validate(records, expectedWindows) {
        let windows;
        try {
            windows = await currentWindows();
        } catch (_) {
            return problem('state_unavailable');
        }
        if (!sameWindows(expectedWindows, windows)) return problem('window_set_changed');
        const expectedParticipantIDs = records.map(record => record.win.id).sort();
        const actualParticipantIDs = windows.filter(win => participants.has(win.id)).map(win => win.id).sort();
        const staleParticipantIDs = [...participants.keys()].filter(id => !windows.some(win => win.id === id));
        if (staleParticipantIDs.length || expectedParticipantIDs.length !== actualParticipantIDs.length ||
            expectedParticipantIDs.some((id, index) => id !== actualParticipantIDs[index])) {
            return problem('window_set_changed');
        }

        for (const record of records) {
            const win = windows.find(candidate => candidate.id === record.win.id);
            if (!win || participants.get(record.win.id) !== record.participant) {
                return problem('window_set_changed', record.win);
            }
            let state;
            try {
                state = await inspect(record.participant);
            } catch (_) {
                return problem('state_unavailable', record.win);
            }
            if (state.revision !== record.state.revision || (!record.state.dirty && state.dirty)) {
                return problem('content_changed', record.win);
            }
            // An autosave may finish while another app's decision is open. The
            // unchanged revision proves it saved the same editable content, so
            // accept that transition and avoid an unnecessary second save.
            record.state = state;
        }
        return true;
    }

    async function inspect(participant) {
        const value = await participant.inspect();
        if (!value || typeof value.dirty !== 'boolean' || !validRevision(value.revision)) {
            throw new TypeError('Session action inspect returned an invalid snapshot');
        }
        return {
            dirty: value.dirty,
            revision: value.revision,
            label: String(value.label || ''),
            detail: String(value.detail || '')
        };
    }

    function validRevision(value) {
        return value === null || typeof value === 'string' || typeof value === 'boolean' ||
            (typeof value === 'number' && Number.isFinite(value));
    }

    function publicSnapshot(win, state) {
        return Object.freeze({
            id: win.id,
            appId: win.appId,
            title: win.title,
            dirty: state.dirty,
            revision: state.revision,
            label: state.label,
            detail: state.detail
        });
    }

    async function currentWindows() {
        const source = await config.getWindows();
        if (!Array.isArray(source)) throw new TypeError('getWindows must return an array');
        const seen = new Set();
        return source.map(value => {
            if (!value || typeof value !== 'object') throw new TypeError('getWindows returned an invalid window');
            const id = String(value && value.id || '').trim();
            if (!id || seen.has(id)) throw new TypeError('getWindows returned an invalid window id');
            seen.add(id);
            return {
                id,
                appId: String(value.appId || ''),
                title: String(value.title || ''),
                hasBeforeClose: typeof value.beforeClose === 'function' || value.beforeClose === true || value.hasBeforeClose === true
            };
        }).sort((left, right) => left.id < right.id ? -1 : left.id > right.id ? 1 : 0);
    }

    function sameWindows(left, right) {
        return left.length === right.length && left.every((win, index) =>
            win.id === right[index].id && win.appId === right[index].appId &&
            win.hasBeforeClose === right[index].hasBeforeClose);
    }

    async function allowed() {
        try { return await config.canLogout() === true; }
        catch (_) { return false; }
    }

    async function problem(code, win) {
        try {
            await config.onProblem({ code, windowId: win?.id || '', title: win?.title || '' });
        } catch (_) { /* Problem reporting must not turn a blocked logout into an unhandled rejection. */ }
        return false;
    }

    const api = { configure, register, requestLogout };
    window.AuraDesktopSessionActions = api;
})();
