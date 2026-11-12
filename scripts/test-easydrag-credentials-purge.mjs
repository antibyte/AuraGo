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
    const sandbox = vm.createContext({ window: { EasyDrag: { core: { esc, tr } } } });
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
        { runs: 1 }, { steps: 3 }, { test_rows: 1 }, { snapshots: [{ name: 's' }] }, { leftovers: [{ name: 'l' }] }]) {
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
    assert.ok(dialog.includes('cred_purge_versions[2]') && dialog.includes('cred_purge_rows[3,9]') && dialog.includes('cred_purge_pinned[1]'));
    assert.equal((dialog.match(/data-ed-credentials-flow=/g) || []).length, 2, 'a flow with a draft finding is listed once, as move first');
    assert.ok(dialog.includes('cred_purge_move_first') && dialog.includes('cred_purge_republish') && dialog.includes('A &#60;i&#62;'), 'names are escaped');
    assert.ok(/data-ed-credentials-snapshot="\.flows-v7-backup-abc"/.test(dialog) && /checked/.test(dialog), 'snapshots are checked by default');
    assert.ok(dialog.includes('cred_purge_outside'));
    assert.ok(purge.stateMarkup(tt, { status: 'running', step: 'redact' }).includes('cred_purge_step'));
    assert.ok(purge.stateMarkup(tt, { status: 'done', clean: true }).includes('cred_purge_clean'));
    assert.ok(purge.stateMarkup(tt, { status: 'done', clean: false, remaining: [{ where: 'flows.db-wal', count: 1 }] }).includes('flows.db-wal'));
    assert.ok(purge.stateMarkup(tt, { status: 'failed', failed_step: 'compact', backup: 'x.enc' }).includes('cred_purge_backup'));
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) { await run(); console.log('credentials purge hint checks passed'); }
