import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

const source = await readFile(new URL('../ui/js/desktop/core/workspace-manager-runtime.js', import.meta.url), 'utf8');

function makeRuntime(options = {}) {
    const context = { window: {} };
    vm.runInNewContext(source, context, { filename: 'workspace-manager-runtime.js' });
    const manager = context.window.AuraDesktopWorkspaceManager;
    manager.configure({ t: key => ({ 'desktop.space_n': 'Arbeitsfläche {{n}}' }[key] || key), ...options });
    return manager;
}

function state(layout, settings = {}) {
    return {
        layout,
        settings: {
            'session.windows': '',
            'appearance.wallpaper_by_space': '{}',
            'appearance.wallpaper': 'groupshoot',
            ...settings
        }
    };
}

function layout(overrides = {}) {
    return {
        version: 1,
        revision: 1,
        next_id: 4,
        workspaces: [{ id: '1', name: '' }, { id: '2', name: '' }, { id: '3', name: '' }],
        removed_to: {},
        ...overrides
    };
}

{
    const manager = makeRuntime();
    assert.deepEqual(JSON.parse(JSON.stringify(manager.snapshot().layout)), layout(), 'legacy/default state keeps the three stable identities');
    assert.equal(manager.name('2'), 'Arbeitsfläche 2', 'unnamed records use localized display names');

    const initial = manager.snapshot();
    initial.layout.workspaces[0].name = 'mutated outside';
    assert.equal(manager.name('1'), 'Arbeitsfläche 1', 'snapshots do not expose mutable internal state');

    const adopted = state(layout({
        revision: 8,
        next_id: 10,
        workspaces: [{ id: '4', name: 'Research' }, { id: '9', name: 'Archive' }],
        removed_to: { '1': '4', '2': '9', '3': '4', '8': '9' }
    }), { 'appearance.wallpaper': 'file:Desktop/wallpaper.webp' });
    assert.equal(manager.adopt(adopted), true);
    assert.equal(manager.resolveId('1'), '4');
    assert.equal(manager.resolveId('8'), '9');
    assert.equal(manager.resolveId('missing'), '4');
    assert.deepEqual(JSON.parse(JSON.stringify(manager.snapshot().settings)), adopted.settings);
    assert.equal(manager.adopt(state(layout({ revision: 7 }))), false, 'older bootstrap snapshots cannot roll back a newer layout');
}

{
    const calls = [];
    let serverState = state(layout());
    const manager = makeRuntime({ api: async (url, options) => {
        calls.push({ url, options });
        if (options.method === 'GET') return serverState;
        const mutation = JSON.parse(options.body);
        assert.equal(mutation.revision, serverState.layout.revision, 'mutations carry the latest revision');
        const next = structuredClone(serverState.layout);
        if (mutation.operation === 'add') {
            next.workspaces.push({ id: String(next.next_id++), name: mutation.name });
        } else if (mutation.operation === 'rename') {
            next.workspaces.find(item => item.id === mutation.id).name = mutation.name.trim();
        } else if (mutation.operation === 'reorder') {
            next.workspaces = mutation.order.map(id => next.workspaces.find(item => item.id === id));
        } else if (mutation.operation === 'delete') {
            next.removed_to[mutation.id] = mutation.move_to;
            next.workspaces = next.workspaces.filter(item => item.id !== mutation.id);
        }
        next.revision++;
        serverState = state(next, serverState.settings);
        return serverState;
    } });

    await manager.load();
    const seen = [];
    const unsubscribe = manager.subscribe((value, reason) => seen.push([value.layout.revision, reason]));
    await manager.add('Research');
    await manager.rename('4', 'Archive');
    await manager.reorder(['4', '1', '2', '3']);
    await manager.remove('4', '1');
    unsubscribe();
    assert.deepEqual(calls.map(call => call.options.method), ['GET', 'POST', 'POST', 'POST', 'POST']);
    assert.deepEqual(seen.map(item => item[0]), [2, 3, 4, 5]);
    assert.equal(manager.resolveId('4'), '1');
    assert.deepEqual(manager.snapshot().layout.workspaces.map(item => item.id), ['1', '2', '3']);
    assert.equal(JSON.parse(calls[1].options.body).operation, 'add');
    assert.equal(JSON.parse(calls[2].options.body).revision, 2, 'serial operations use adopted revisions');
}

{
    let resolveRequest;
    const calls = [];
    const manager = makeRuntime({ api: (url, options) => {
        calls.push({ url, options });
        return new Promise(resolve => { resolveRequest = resolve; });
    } });
    const running = manager.add('Pending');
    await assert.rejects(manager.rename('1', 'Second'), /workspace_mutation_busy/);
    resolveRequest(state(layout({ revision: 2, next_id: 5,
        workspaces: [...layout().workspaces, { id: '4', name: 'Pending' }] })));
    await running;
    assert.equal(calls.length, 1, 'parallel calls are rejected instead of reordering revisions');
}

{
    let calls = 0;
    let conflicts = 0;
    const serverState = state(layout({ revision: 3, workspaces: [{ id: '1', name: 'Authoritative' }] }));
    const manager = makeRuntime({
        api: async () => {
            calls++;
            const error = new Error('conflict');
            error.status = 409;
            error.body = { code: 'workspace_revision_conflict', state: serverState };
            throw error;
        },
        onConflict: snapshot => {
            conflicts++;
            assert.equal(snapshot.layout.revision, 3);
        }
    });
    await assert.rejects(manager.add('Never replayed'));
    assert.equal(calls, 1, 'revision conflicts are never automatically retried');
    assert.equal(conflicts, 1);
    assert.equal(manager.name('1'), 'Authoritative', 'conflict response adopts authoritative state');
}

{
    const manager = makeRuntime({ readonly: () => true, api: async () => { throw new Error('readonly mutation reached API'); } });
    await assert.rejects(manager.add('Blocked'), /desktop_readonly/);
    assert.equal(manager.isReadonly(), true);
}

for (const invalidLayout of [
    layout({ revision: Number.MAX_SAFE_INTEGER + 1 }),
    layout({ next_id: 4, workspaces: [...layout().workspaces, { id: '4', name: '' }] }),
    layout({ workspaces: [] }),
    layout({ removed_to: { '4': '99' }, next_id: 5 }),
    layout({ workspaces: [{ id: '01', name: '' }] })
]) {
    const manager = makeRuntime();
    assert.throws(() => manager.adopt(state(invalidLayout)), /Workspace/);
}

for (const marker of [
    'role="dialog" aria-modal="true"',
    'data-workspace-add-name',
    'data-workspace-confirm-delete',
    'data-workspace-up=',
    'data-workspace-down=',
    'function captureDialogState()',
    'function restoreDialogFocus(descriptor)',
    'desktop.workspaces_delete_select_target',
    'desktop.workspaces_minimum',
    'desktop.workspaces_maximum'
]) {
    assert.ok(source.includes(marker), `manager dialog includes ${marker}`);
}

console.log('Desktop workspace manager tests passed');
