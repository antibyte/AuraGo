import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

const source = await readFile(new URL('../ui/js/desktop/core/session-actions-runtime.js', import.meta.url), 'utf8');

function runtime(config) {
    const context = { window: {} };
    vm.runInNewContext(source, context, { filename: 'session-actions-runtime.js' });
    const api = context.window.AuraDesktopSessionActions;
    api.configure(config);
    return api;
}

function participant(state, options = {}) {
    const calls = options.calls || [];
    return {
        calls,
        inspect() {
            calls.push('inspect');
            return { dirty: state.dirty, revision: state.revision, label: options.label || 'Document' };
        },
        async decide(snapshot) {
            calls.push('decide:' + snapshot.revision);
            return options.decide ? options.decide(snapshot) : 'save';
        },
        async save(expectedRevision) {
            calls.push('save:' + expectedRevision);
            if (options.save) return options.save(expectedRevision, state);
            state.dirty = false;
            return { revision: expectedRevision };
        }
    };
}

function windowInfo(id, beforeClose = false) {
    return { id, appId: 'editor', title: 'Editor ' + id, beforeClose: beforeClose ? () => false : null };
}

{
    const windows = [windowInfo('A'), windowInfo('B')];
    const aState = { dirty: true, revision: 1, draft: 'saved recovery draft' };
    const bState = { dirty: true, revision: 1, draft: 'other draft' };
    const calls = [];
    let logoutCalls = 0;
    const api = runtime({
        getWindows: () => windows,
        canLogout: () => true,
        logout: () => { logoutCalls++; return true; },
        onProblem: problem => calls.push('problem:' + problem.code)
    });
    const a = participant(aState, { calls, decide: () => 'save' });
    const b = participant(bState, { calls, decide: () => 'cancel' });
    api.register('A', a);
    api.register('B', b);
    assert.equal(await api.requestLogout(), false, 'later cancellation must cancel the whole logout');
    assert.equal(calls.some(call => call.startsWith('save:')), false, 'no save/discard side effect may run before every choice is gathered');
    assert.equal(aState.draft, 'saved recovery draft', 'previously persisted drafts remain untouched');
    assert.equal(bState.draft, 'other draft');
    assert.equal(logoutCalls, 0);
}

{
    const windows = [windowInfo('A'), windowInfo('B')];
    const aState = { dirty: true, revision: 1 };
    const bState = { dirty: true, revision: 1 };
    let logoutCalls = 0;
    const api = runtime({ getWindows: () => windows, canLogout: () => true, logout: () => { logoutCalls++; return true; } });
    api.register('A', participant(aState, { decide: () => 'save' }));
    api.register('B', participant(bState, { decide: () => 'save', save: () => { throw new Error('save failed'); } }));
    assert.equal(await api.requestLogout(), false);
    assert.equal(aState.dirty, false, 'a prior successful save may remain saved');
    assert.equal(bState.dirty, true);
    assert.equal(logoutCalls, 0, 'a later save failure blocks authenticated logout');
}

{
    const windows = [windowInfo('A'), windowInfo('B')];
    const aState = { dirty: true, revision: 4 };
    let explicitSaves = 0;
    let logoutCalls = 0;
    const api = runtime({ getWindows: () => windows, canLogout: () => true, logout: () => { logoutCalls++; return true; } });
    api.register('A', participant(aState, {
        decide: () => 'save',
        save: () => { explicitSaves++; aState.dirty = false; return { revision: 4 }; }
    }));
    api.register('B', participant({ dirty: true, revision: 2 }, {
        decide: () => { aState.dirty = false; return 'keep'; }
    }));
    assert.equal(await api.requestLogout(), true, 'an autosave of the same revision during decisions is safe');
    assert.equal(explicitSaves, 0, 'the coordinator does not repeat an already completed autosave');
    assert.equal(logoutCalls, 1);
}

{
    const windows = [windowInfo('A')];
    const state = { dirty: true, revision: 7 };
    let saves = 0;
    let logoutCalls = 0;
    const api = runtime({ getWindows: () => windows, canLogout: () => true, logout: () => { logoutCalls++; return true; } });
    api.register('A', participant(state, {
        decide: () => { state.revision++; return 'save'; },
        save: () => { saves++; return { revision: state.revision }; }
    }));
    assert.equal(await api.requestLogout(), false, 'an edit during the decision phase must block logout');
    assert.equal(saves, 0);
    assert.equal(logoutCalls, 0);
}

{
    const windows = [windowInfo('A')];
    const state = { dirty: true, revision: 1 };
    let saves = 0;
    let logoutCalls = 0;
    const api = runtime({ getWindows: () => windows, canLogout: () => true, logout: () => { logoutCalls++; return true; } });
    api.register('A', participant(state, {
        decide: () => 'save',
        save: async () => { saves++; state.revision++; state.dirty = false; return { revision: 1 }; }
    }));
    assert.equal(await api.requestLogout(), false, 'an edit racing the save must not be marked clean');
    assert.equal(saves, 1);
    assert.equal(logoutCalls, 0);
}

{
    let releaseDecision;
    const windows = [windowInfo('A')];
    const state = { dirty: true, revision: 1 };
    let logoutCalls = 0;
    let decideCalls = 0;
    const api = runtime({ getWindows: () => windows, canLogout: () => true, logout: () => { logoutCalls++; return true; } });
    api.register('A', participant(state, {
        decide: () => { decideCalls++; return new Promise(resolve => { releaseDecision = resolve; }); }
    }));
    const first = api.requestLogout();
    const second = api.requestLogout();
    assert.equal(first, second, 'concurrent logout requests share one coordinator promise');
    await new Promise(resolve => setImmediate(resolve));
    assert.equal(decideCalls, 1);
    releaseDecision('keep');
    assert.equal(await first, true);
    assert.equal(logoutCalls, 1);
    assert.equal(state.dirty, true, 'Without saving must leave the in-memory document unchanged until final logout');
}

{
    let invokedLegacyGuard = false;
    let logoutCalls = 0;
    const api = runtime({
        getWindows: () => [{ ...windowInfo('legacy'), beforeClose: () => { invokedLegacyGuard = true; return true; } }],
        canLogout: () => true,
        logout: () => { logoutCalls++; return true; }
    });
    api.configure({ onProblem: value => assert.equal(value.code, 'unknown_guard') });
    assert.equal(await api.requestLogout(), false, 'unknown custom close guards fail safely');
    assert.equal(invokedLegacyGuard, false);
    assert.equal(logoutCalls, 0);
}

{
    const windows = [windowInfo('A')];
    let decisions = 0;
    let logoutCalls = 0;
    const api = runtime({ getWindows: () => windows, canLogout: () => false, logout: () => { logoutCalls++; return true; } });
    api.register('A', participant({ dirty: true, revision: 1 }, { decide: () => { decisions++; return 'save'; } }));
    assert.equal(await api.requestLogout(), false, 'auth-disabled state is not actionable');
    assert.equal(decisions, 0);
    assert.equal(logoutCalls, 0);
}

{
    const windows = [windowInfo('A')];
    const state = { dirty: true, revision: 1 };
    let saves = 0;
    let logoutCalls = 0;
    const api = runtime({
        getWindows: () => windows,
        canLogout: () => true,
        logout: () => { logoutCalls++; return true; }
    });
    api.register('A', participant(state, {
        decide: () => { windows.push(windowInfo('B')); return 'save'; },
        save: () => { saves++; state.dirty = false; return { revision: 1 }; }
    }));
    assert.equal(await api.requestLogout(), false, 'a newly opened window invalidates the decision snapshot');
    assert.equal(saves, 0);
    assert.equal(logoutCalls, 0);
}

{
    let releaseProblem;
    let problemCalls = 0;
    const api = runtime({
        getWindows: () => [windowInfo('legacy', true)],
        canLogout: () => true,
        logout: async () => true,
        onProblem: () => {
            problemCalls++;
            if (problemCalls > 1) return Promise.resolve();
            return new Promise(resolve => { releaseProblem = resolve; });
        }
    });
    const first = api.requestLogout();
    await new Promise(resolve => setImmediate(resolve));
    const second = api.requestLogout();
    assert.equal(first, second, 'the coordinator stays serialized while an async problem dialog is open');
    assert.equal(problemCalls, 1);
    releaseProblem();
    assert.equal(await first, false);
    assert.equal(await api.requestLogout(), false);
    assert.equal(problemCalls, 2);
}

{
    const api = runtime({
        getWindows: () => [windowInfo('legacy', true)],
        canLogout: () => true,
        logout: async () => true,
        onProblem: async () => { throw new Error('dialog failed'); }
    });
    assert.equal(await api.requestLogout(), false, 'a rejected async problem dialog remains contained');
}

console.log('Desktop session action coordinator tests passed');
