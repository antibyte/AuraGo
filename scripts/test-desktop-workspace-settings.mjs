import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

const source = await readFile(new URL('../ui/js/desktop/core/settings-runtime.js', import.meta.url), 'utf8');
const clone = value => JSON.parse(JSON.stringify(value));
const defaults = { 'session.windows': '', 'appearance.wallpaper': 'groupshoot', 'appearance.wallpaper_by_space': '{}' };
const layout = () => ({ version: 1, revision: 1, next_id: 4, workspaces: ['1', '2', '3'].map(id => ({ id, name: '' })), removed_to: {} });
const tick = () => new Promise(resolve => setImmediate(resolve));

function fixture() {
    let current = { layout: layout(), settings: { ...defaults } };
    let config, held, rejectNext;
    const calls = [];
    const draft = { text: 'unsaved', dirty: true };
    const win = { spaceId: '2', draft, element: { dataset: { spaceId: '2' }, style: { display: 'none' } } };
    const state = { bootstrap: { settings: { ...defaults } }, windows: new Map([['editor', win]]), activeSpaceId: '2' };
    const manager = {
        configure(options) { config = options; },
        snapshot: () => clone(current),
        resolveId(id) { return current.layout.workspaces.some(space => space.id === id) ? id : current.layout.removed_to[id] || current.layout.workspaces[0].id; },
        adopt(next) {
            if (next.layout.revision < current.layout.revision) return false;
            current = clone(next);
            config.onChange(clone(current));
            return true;
        }
    };
    const context = vm.createContext({
        state, desktopSessionEnded: false, desktopSettingDefaults: defaults,
        window: { AuraDesktopWorkspaceManager: manager, dispatchEvent() {} },
        CustomEvent: class {}, desktopReadonly: () => false,
        mountDesktopDialog: () => () => {},
        settingValue: key => state.bootstrap.settings[key] ?? defaults[key],
        t: key => key, desktopSettings: () => state.bootstrap.settings,
        settingBool: () => true, currentDesktopTheme: () => ({}), desktopThemes: () => [],
        assignWindowSpace(target, id) { target.spaceId = id; target.element.dataset.spaceId = id; },
        showDesktopNotification() {}, applyDesktopSettings() {}, applySoundSettingsChange() {},
        renderStartButtonIcon() {}, renderIcons() {}, renderWidgets() {}, renderStartApps() {},
        async api(url, options) {
            assert.equal(url, '/api/desktop/settings');
            const body = JSON.parse(options.body);
            calls.push(body);
            if (held) { const wait = held; held = null; await wait; }
            if (rejectNext) { const error = rejectNext; rejectNext = null; throw error; }
            const server = clone(current);
            if (body.revision != null) {
                assert.equal(body.revision, server.layout.revision, 'request must use the revision its inputs belong to');
                server.layout.revision++;
            }
            Object.assign(server.settings, body.settings);
            return { settings: server.settings, workspace: server };
        }
    });
    vm.runInContext(source, context);
    vm.runInContext('initDesktopWorkspaceSettings()', context);
    return { context, state, win, draft, calls, manager,
        run: text => vm.runInContext(text, context),
        hold() { let release; held = new Promise(resolve => { release = resolve; }); return release; },
        conflict(next) { rejectNext = Object.assign(new Error('conflict'), { body: { code: 'workspace_revision_conflict', state: next } }); }
    };
}

// A queued unrelated session save must not cause a false wallpaper conflict.
{
    const f = fixture();
    const release = f.hold();
    const first = f.run("writeDesktopSettings({'session.windows':'new snapshot'})");
    await tick();
    const wallpaper = f.run("writeDesktopSettings({'appearance.wallpaper':'forest'})");
    release();
    await Promise.all([first, wallpaper]);
    assert.deepEqual(f.calls.map(call => call.revision), [1, 2]);
    assert.equal(f.state.bootstrap.settings['appearance.wallpaper'], 'forest');
}

// A changed workspace or wallpaper is never authorized merely by attaching a newer revision.
for (const change of ['layout', 'wallpaper']) {
    const f = fixture();
    const release = f.hold();
    const first = f.run("writeDesktopSettings({'sound.volume':'0.2'})");
    await tick();
    const pending = f.run("writeDesktopSettings({'appearance.wallpaper_by_space':'{\"2\":\"forest\"}'})");
    const rejected = assert.rejects(pending, /desktop.workspaces_conflict/);
    const next = f.manager.snapshot();
    next.layout.revision++;
    if (change === 'layout') {
        next.layout.workspaces = next.layout.workspaces.filter(space => space.id !== '2');
        next.layout.removed_to['2'] = '3';
    } else next.settings['appearance.wallpaper_by_space'] = '{"2":"aurora"}';
    f.manager.adopt(next);
    release();
    await first;
    await rejected;
    assert.equal(f.calls.length, 1);
    assert.equal(f.win.draft, f.draft);
    assert.equal(f.win.element.style.display, 'none');
    if (change === 'layout') assert.equal(f.win.spaceId, '3');
}

// A rejected in-flight write adopts current state without replay or rebuilding live windows.
{
    const f = fixture();
    const next = f.manager.snapshot();
    next.layout.revision = 4;
    next.layout.workspaces = next.layout.workspaces.filter(space => space.id !== '2');
    next.layout.removed_to['2'] = '3';
    f.conflict(next);
    await assert.rejects(f.run("writeDesktopSettings({'session.windows':'old'})"), /desktop.workspaces_conflict/);
    assert.equal(f.calls.length, 1);
    assert.equal(f.win.spaceId, '3');
    assert.equal(f.state.activeSpaceId, '3');
    assert.equal(f.win.draft, f.draft);
    const stale = { settings: { ...defaults, 'desktop.workspaces.layout': JSON.stringify(layout()) } };
    f.context.stale = stale;
    f.run('acceptDesktopBootstrap(stale)');
    assert.equal(f.manager.snapshot().layout.revision, 4);
    assert.equal(f.win.spaceId, '3');
    f.context.lateSettings = { settings: { ...defaults, 'desktop.workspaces.layout': JSON.stringify(layout()), 'pet.scale': '0.8' } };
    f.run('reconcileDesktopSettingsResponse(lateSettings)');
    assert.equal(JSON.parse(f.context.lateSettings.settings['desktop.workspaces.layout']).revision, 4);
    assert.equal(f.context.lateSettings.settings['pet.scale'], '0.8');
    assert.equal(f.win.draft, f.draft);
}

// Session producers read the live draft/window state when the queue drains, not when queued.
{
    const f = fixture();
    const release = f.hold();
    const first = f.run("writeDesktopSettings({'sound.volume':'0.5'})");
    await tick();
    const pending = f.run("queueDesktopSettingsWrite(() => ({revision:desktopWorkspaceRevision(),settings:{'session.windows':state.windows.get('editor').draft.text}}))");
    f.draft.text = 'edited while saving';
    release();
    await Promise.all([first, pending]);
    assert.equal(f.calls[1].settings['session.windows'], 'edited while saving');
    f.run('desktopSessionEnded = true');
    await assert.rejects(f.run("writeDesktopSettings({'sound.volume':'0'})"), /desktop.action_unavailable/);
    assert.equal(f.calls.length, 2);
}

console.log('Desktop workspace settings queue, conflict and draft-preservation checks passed.');
