// Pure checks of the credential purge hint: what needs attention and what the card says.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

function load() {
    const here = path.dirname(fileURLToPath(import.meta.url));
    const source = path.join(here, '..', 'ui', 'js', 'desktop', 'apps', 'easydrag-credentials-purge.js');
    const esc = value => String(value).replace(/[&<>"']/g, c => '&#' + c.charCodeAt(0) + ';');
    const tr = (t, key, fallback) => { const value = t(key); return value && value !== key ? value : fallback; };
    const sandbox = vm.createContext({ window: { EasyDrag: { core: { esc, tr } } }, Intl, Date, Map });
    const format = path.join(here, '..', 'ui', 'js', 'desktop', 'apps', 'easydrag-format.js');
    vm.runInContext(fs.readFileSync(format, 'utf8'), sandbox, { filename: format });
    vm.runInContext(fs.readFileSync(source, 'utf8'), sandbox, { filename: source });
    return sandbox.window.EasyDrag.credentialsPurge;
}

export async function run() {
    const purge = load();
    const empty = { drafts: [], live: [], versions_purgeable: 0, versions_pinned: 0, runs: 0, steps: 0, test_rows: 0, snapshots: [], leftovers: [], backups: [{ name: 'x.enc' }] };
    assert.equal(purge.needsAttention(empty), false, 'encrypted backups alone need no attention');
    assert.equal(purge.needsAttention(null), false);
    assert.equal(purge.needsAttention('x'), false);
    for (const change of [{ drafts: [{ flow_id: 'f' }] }, { live: [{ flow_id: 'f' }] }, { versions_purgeable: 1 }, { versions_pinned: 2 },
        { runs: 1 }, { steps: 3 }, { test_rows: 1 }, { snapshots: [{ name: 's' }] }, { leftovers: [{ name: 'l' }] }, { live_blocks: [{ block_id: 'flow_blk', revision: 1, parent_ids: ['flow_p'] }] }]) {
        assert.equal(purge.needsAttention(Object.assign({}, empty, change)), true, JSON.stringify(change));
    }
    const t = key => ({ 'easydrag.ui.cred_hint_title': 'Title <b>', 'easydrag.ui.cred_hint_text': 'Text', 'easydrag.ui.cred_hint_action': 'Clean' }[key] || key);
    assert.equal(purge.hintMarkup(t, empty), '', 'a clean report shows no card');
    const markup = purge.hintMarkup(t, Object.assign({}, empty, { runs: 2 }));
    assert.ok(markup.includes('Title &#60;b&#62;') && !markup.includes('<b>'), 'the title is escaped');
    assert.ok(markup.includes('data-ed-credentials-open') && markup.includes('Clean') && markup.includes('Text'));
    const tt = (key, params) => key + (params ? '[' + Object.values(params).join(',') + ']' : ''); // esc() would mangle JSON quotes
    const full = Object.assign({}, empty, { versions_purgeable: 2, versions_pinned: 1, runs: 3, steps: 4, test_rows: 5,
        drafts: [{ flow_id: 'flow_a', name: 'A <i>' }], live: [{ flow_id: 'flow_a', name: 'A <i>' }, { flow_id: 'flow_b', name: 'B' }],
        snapshots: [{ name: '.flows-v7-backup-abc', bytes: 2048, mod_time: '2026-11-01T10:00:00Z' }] });
    const dialog = purge.dialogMarkup(tt, full);
    assert.ok(dialog.includes('cred_purge_versions[2]') && dialog.includes('cred_purge_rows[3,9]') && dialog.includes('cred_purge_pinned_one[1]'));
    assert.equal((dialog.match(/data-ed-credentials-flow=/g) || []).length, 2, 'a flow with a draft finding is listed once, as move first');
    assert.ok(dialog.includes('cred_purge_move_first') && dialog.includes('cred_purge_republish') && dialog.includes('A &#60;i&#62;'), 'names are escaped');
    assert.ok(/data-ed-credentials-snapshot="\.flows-v7-backup-abc"/.test(dialog) && /checked/.test(dialog), 'snapshots are checked by default');
    assert.ok(dialog.includes('cred_purge_outside'));
    assert.ok(!dialog.includes('cred_purge_live_block_advice'), 'no live block, no live block advice');
    const withBlocks = purge.dialogMarkup(tt, Object.assign({}, empty, { live_blocks: [{ block_id: 'flow_b<k', revision: 3, parent_ids: ['flow_p1', 'flow_p2'] }] }));
    assert.ok(withBlocks.includes('cred_purge_live_block_advice') && withBlocks.includes('cred_purge_live_block_line[flow_b&#60;k,3]'), 'the live block is listed with its id and revision, escaped');
    assert.equal((withBlocks.match(/data-ed-credentials-flow="flow_p[12]"/g) || []).length, 2, 'each parent flow is a button');
    assert.ok(purge.stateMarkup(tt, { status: 'running', step: 'redact' }).includes('cred_purge_step'));
    assert.ok(purge.stateMarkup(tt, { status: 'done', clean: true }).includes('cred_purge_clean'));
    assert.ok(purge.stateMarkup(tt, { status: 'done', clean: false, remaining: [{ where: 'flows.db-wal', count: 1 }] }).includes('flows.db-wal'));
    assert.ok(purge.stateMarkup(tt, { status: 'failed', failed_step: 'compact', backup: 'x.enc' }).includes('cred_purge_backup'));
    await hintIsolation();
}

// hintIsolation: a failing scan (500, 401, 403, 404, network error) hides the hint silently. The card never
// carries the start page's stale class, nothing is notified and nothing rejects.
async function hintIsolation() {
    const here = path.dirname(fileURLToPath(import.meta.url));
    const source = fs.readFileSync(path.join(here, '..', 'ui', 'js', 'desktop', 'apps', 'easydrag-credentials-purge.js'), 'utf8');
    for (const failure of [{ status: 500 }, { status: 401 }, { status: 403 }, { status: 404 }, new TypeError('Failed to fetch')]) {
        const notices = [], unhandled = [];
        const onUnhandled = reason => unhandled.push(reason);
        process.on('unhandledRejection', onUnhandled);
        let card = null;
        const container = { querySelector: () => null, appendChild: node => { card = node; }, insertBefore: node => { card = node; } };
        const el = html => {
            const node = { hidden: html.includes(' hidden'), innerHTML: '', className: /class="([^"]*)"/.exec(html)[1], listeners: [], removed: false,
                addEventListener(type, fn) { this.listeners.push(fn); }, remove() { this.removed = true; } };
            return node;
        };
        const esc = value => String(value);
        const sandbox = vm.createContext({ window: { EasyDrag: { core: { esc, el } } }, setTimeout, console });
        vm.runInContext(source, sandbox, { filename: 'easydrag-credentials-purge.js' });
        let calls = 0;
        const app = { t: key => key, api: { credentialScan: async () => { calls++; throw failure; } }, ctx: { notify: n => notices.push(n) } };
        const handle = sandbox.window.EasyDrag.credentialsPurge.mountHint(container, app);
        await new Promise(resolve => setTimeout(resolve, 10));
        const label = JSON.stringify(failure.status || failure.message);
        assert.equal(calls, 1, 'the hint asked the scan once ' + label);
        assert.ok(card && card.hidden && card.innerHTML === '', 'the hint stays hidden and empty on ' + label);
        assert.ok(!card.className.split(' ').includes('ed-home-stale'), 'the hint is not the start page stale banner ' + label);
        assert.deepEqual(notices, [], 'no toast on ' + label);
        await handle.refresh();
        assert.ok(card.hidden, 'a second failing refresh keeps it hidden ' + label);
        process.off('unhandledRejection', onUnhandled);
        assert.deepEqual(unhandled, [], 'no unhandled rejection on ' + label);
    }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) { await run(); console.log('credentials purge hint checks passed'); }
