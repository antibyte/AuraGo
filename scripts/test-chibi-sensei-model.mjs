// Chibi Sensei desktop app: Node test of the pure logic (readiness, primary action, tips, the table reducer, row previews and the
// mascot mood). The four modules run in one vm realm without a DOM. Run: node scripts/test-chibi-sensei-model.mjs
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';

const root = path.resolve(import.meta.dirname, '..');
const files = ['chibi-sensei-core.js', 'chibi-sensei-api.js', 'chibi-sensei-mascot.js', 'chibi-sensei-steps.js', 'chibi-sensei-table.js', 'chibi-sensei-projects.js', 'chibi-sensei-training.js', 'chibi-sensei-chart.js', 'chibi-sensei-run.js', 'chibi-sensei-chat.js', 'chibi-sensei-test.js'];
const read = file => fs.readFileSync(path.join(root, 'ui/js/desktop/apps/chibi-sensei', file), 'utf8');
const plain = value => JSON.parse(JSON.stringify(value));
let checks = 0;
// The modules run in their own vm realm, so arrays and objects they return are compared as plain JSON.
const eq = (actual, expected, message) => { checks++; assert.deepStrictEqual(plain(actual), plain(expected), message); };
const ok = (value, message) => { checks++; assert.ok(value, message); };

const window = {};
vm.runInNewContext(files.map(read).join('\n'), { window, console, Intl, Map, Set, JSON, Math, Date, URL, TextDecoder, TextEncoder, Uint8Array, AbortController, Promise });
const P = window.ChibiSenseiParts;
ok(P && P.steps && P.table && P.mascot, 'the modules publish their parts');

// ---- derive ---------------------------------------------------------------------------------------------------------------
const goalDone = { recipe: 'assistant', profile_id: 'qwen35-0p8b', goal_text: 'x' };
const derive = input => plain(P.steps.derive(input));
eq(derive({ project: { recipe: 'assistant', rows: 0 } }), { goal: 'in_progress', data: 'todo', training: 'todo', test: 'todo', use: 'todo' }, 'a: recipe only');
eq(derive({ project: Object.assign({ rows: 0 }, goalDone) }).goal, 'done', 'b: goal done');
eq(derive({ project: Object.assign({ rows: 0 }, goalDone) }).data, 'todo', 'b: no rows, data todo');
eq(derive({ project: Object.assign({ rows: 12 }, goalDone), checks: null }).data, 'in_progress', 'c: rows without checks');
const blockCheck = { data_rev: 5, ready: false, checks: [{ id: 'count_min', level: 'block' }] };
eq(derive({ project: Object.assign({ rows: 12, data_rev: 5 }, goalDone), checks: blockCheck }).data, 'blocked', 'd: block finding');
eq(derive({ project: Object.assign({ rows: 12, data_rev: 5 }, goalDone), checks: blockCheck, checksStale: true }).data, 'in_progress', 'e: stale checks');
eq(derive({ project: Object.assign({ rows: 12, data_rev: 6 }, goalDone), checks: blockCheck }).data, 'in_progress', 'f: data moved on');
const readyCheck = { data_rev: 5, ready: true, checks: [{ level: 'warn' }] };
eq(derive({ project: Object.assign({ rows: 12, data_rev: 5 }, goalDone), checks: readyCheck }).data, 'ready', 'g: warnings do not block');
eq(derive({ project: Object.assign({ rows: 0 }, goalDone), goalDirty: true }).goal, 'in_progress', 'h: unsaved goal');
const classify = labels => derive({ project: { recipe: 'specialist', profile_id: 'p', goal_text: 'x', task_kind: 'classify', labels } }).goal;
eq(classify(['a']), 'in_progress', 'i: one label');
eq(classify(['a', 'b']), 'done', 'i: two labels');
eq(derive({ project: { recipe: 'specialist', profile_id: 'p', goal_text: 'x' } }).goal, 'in_progress', 'i: no task kind');
const readyProject = Object.assign({ rows: 12, data_rev: 5 }, goalDone);
eq(derive({ project: readyProject, checks: readyCheck, jobs: [{ kind: 'generate', state: 'running' }] }).data, 'in_progress', 'j: generation runs');
eq(derive({ project: readyProject, checks: readyCheck, jobs: [{ kind: 'export', state: 'running' }] }).data, 'ready', 'j: export keeps ready');
eq(derive({ project: Object.assign({ steps: { training: 'weird' } }, goalDone) }).training, 'todo', 'k: unknown server state');

// ---- firstOpen ------------------------------------------------------------------------------------------------------------
eq(P.steps.firstOpen({ goal: 'in_progress' }), 'goal', 'firstOpen: goal open');
eq(P.steps.firstOpen({ goal: 'done' }), 'data', 'firstOpen: goal done');

// ---- primaryFor -----------------------------------------------------------------------------------------------------------
const primary = (step, steps, opts) => plain(P.steps.primaryFor(step, steps, opts));
let p = primary('goal', {}, { goalValid: false });
ok(p.disabled && p.reasonKey === 'chibi_sensei.primary.goal_incomplete', 'goal invalid is disabled');
ok(!primary('goal', {}, { goalValid: false, readonly: true }).disabled, 'goal invalid but read-only stays enabled');
eq(primary('data', { data: 'ready' }, {}).action, 'goto_training', 'data ready goes to training');
p = primary('data', { data: 'blocked' }, { rows: 3 });
ok(p.action === 'run_checks' && !p.disabled, 'blocked with rows runs the checks');
p = primary('data', { data: 'todo' }, { rows: 0 });
ok(p.disabled && p.reasonKey === 'chibi_sensei.primary.no_rows', 'no rows disables the check');
p = primary('data', { data: 'blocked' }, { rows: 3, readonly: true });
ok(p.disabled && p.reasonKey === 'chibi_sensei.readonly_banner', 'read-only disables the check');
p = primary('data', { data: 'blocked' }, { rows: 3, checksRunning: true });
ok(p.disabled && p.labelKey === 'chibi_sensei.checks.running', 'a running check disables the button');
p = primary('test', { test: 'todo' }, {});
ok(p.disabled && p.action === 'open_chat' && p.reasonKey === 'chibi_sensei.primary.no_succeeded_run', 'test without a succeeded run is disabled');
p = primary('test', { test: 'ready' }, {});
ok(!p.disabled && p.action === 'open_chat' && p.labelKey === 'chibi_sensei.primary.chat_model', 'test ready opens the chat');
ok(!primary('test', { test: 'todo' }, { hasSucceeded: true }).disabled, 'a succeeded run in the list enables the chat');
p = primary('test', { test: 'done' }, { readonly: true });
ok(p.disabled && p.reasonKey === 'chibi_sensei.readonly_banner', 'test is disabled on a read-only desktop');
p = primary('use', {}, {});
ok(p.disabled && p.reasonKey === 'chibi_sensei.primary.later', 'use comes later');
p = primary('training', { training: 'ready' }, { preflight: null });
ok(p.action === 'start_training' && p.disabled && p.reasonKey === 'chibi_sensei.primary.preflight_loading', 'training waits for the preflight');
p = primary('training', { training: 'ready' }, { preflight: { ready: false } });
ok(p.disabled && p.reasonKey === 'chibi_sensei.primary.preflight_blocked', 'a blocking preflight disables the start');
p = primary('training', { training: 'ready' }, { preflight: { ready: true } });
ok(!p.disabled && p.labelKey === 'chibi_sensei.primary.start_training', 'a ready preflight enables the start');
p = primary('training', { training: 'ready' }, { preflight: { ready: true }, readonly: true });
ok(p.disabled && p.reasonKey === 'chibi_sensei.readonly_banner', 'read-only cannot start');
eq(primary('training', { training: 'done' }, { preflight: { ready: true } }).labelKey, 'chibi_sensei.primary.train_again', 'after a success: train again');
p = primary('training', { training: 'in_progress' }, { runActive: true, readonly: true });
ok(p.action === 'show_run' && !p.disabled, 'an active run is shown, even read-only');
p = primary('training', {}, { preflight: { ready: true }, starting: true });
ok(p.disabled && p.labelKey === 'chibi_sensei.primary.starting', 'a start in flight disables the button');

// ---- tipFor ---------------------------------------------------------------------------------------------------------------
eq(P.steps.tipFor('data', 'blocked'), 'chibi_sensei.tip.data_blocked', 'tip: data blocked');
eq(P.steps.tipFor('training', 'ready'), 'chibi_sensei.tip.training_ready', 'tip: training ready');
eq(P.steps.tipFor('training', 'weird'), 'chibi_sensei.tip.training_todo', 'tip: unknown training state');
eq(P.steps.tipFor('test', 'ready'), 'chibi_sensei.tip.test_ready', 'tip: test ready');
eq(P.steps.tipFor('test', 'done'), 'chibi_sensei.tip.test_done', 'tip: test done');
eq(P.steps.tipFor('test', 'weird'), 'chibi_sensei.tip.test_todo', 'tip: unknown test state');

// ---- table ----------------------------------------------------------------------------------------------------------------
const T = P.table;
const reduce = (state, action) => T.reduce(state, action);
let s = T.initialState();
eq([s.offset, s.limit, s.selection], [0, 50, { mode: 'ids', ids: [] }], 'table: initial state');
s = reduce(s, { type: 'load_ok', total: 120 });
eq(reduce(s, { type: 'page', offset: 100 }).offset, 100, 'page 100');
eq(reduce(s, { type: 'page', offset: 150 }).offset, 100, 'page 150 is clamped');
eq(reduce(s, { type: 'page', offset: -50 }).offset, 0, 'page -50 is clamped');
eq(reduce(s, { type: 'page', offset: 73 }).offset, 50, 'page 73 snaps to a page start');
let sel = s;
for (const id of [5, 3, 5]) sel = reduce(sel, { type: 'select_toggle', id });
eq(sel.selection.ids, [3], 'toggle 5, 3, 5');
const withRows = reduce(s, { type: 'load_ok', total: 120, rows: [{ id: 1 }, { id: 2 }, { id: 3 }] });
eq(reduce(withRows, { type: 'select_page', on: true }).selection.ids, [1, 2, 3], 'select page on');
eq(reduce(reduce(withRows, { type: 'select_page', on: true }), { type: 'select_page', on: false }).selection.ids, [], 'select page off');
const inFilter = reduce(s, { type: 'select_filter' });
eq(T.selectionCount(inFilter), 120, 'filter selection counts the total');
eq(T.selectionSpec(inFilter), { filter: { q: '', split: '', source: '', flag: '', label: '', batch: '' } }, 'filter selection spec');
eq(reduce(inFilter, { type: 'select_toggle', id: 4 }), inFilter, 'toggle in filter mode changes nothing');
const filtered = reduce(sel, { type: 'filter', patch: { q: 'x' } });
ok(filtered.filter.q === 'x' && filtered.offset === 0 && filtered.needsReload, 'filter resets the offset and reloads');
eq(filtered.selection, { mode: 'ids', ids: [] }, 'filter clears the selection');
const far = Object.assign({}, s, { offset: 100 });
const shrunk = reduce(far, { type: 'load_ok', total: 40, rows: [] });
ok(shrunk.offset === 0 && shrunk.needsReload, 'a shorter list moves back to the last page');
const idsState = n => Object.assign({}, s, { selection: { mode: 'ids', ids: Array.from({ length: n }, (_, i) => i + 1) } });
ok(T.needsDeletePreview(idsState(51)), '51 ids need a preview');
ok(!T.needsDeletePreview(idsState(50)), '50 ids need none');
ok(T.needsDeletePreview(inFilter), 'a filter selection needs a preview');

// editor
const rowNine = { id: 9, rev: 3, kind: 'chat', messages: [{ role: 'user', content: 'a' }, { role: 'assistant', content: 'b' }] };
let e = reduce(reduce(s, { type: 'load_ok', total: 120, rows: [rowNine] }), { type: 'edit_open', row: rowNine });
ok(e.editor.state === 'editing' && e.editor.baseRev === 3, 'edit_open');
e = reduce(e, { type: 'edit_change', draft: { messages: [] } });
ok(e.editor.dirty, 'edit_change marks dirty');
e = reduce(e, { type: 'edit_save_start' });
eq(e.editor.state, 'saving', 'edit_save_start');
eq(reduce(e, { type: 'edit_change', draft: { messages: [{}] } }), e, 'no change while saving');
e = reduce(e, { type: 'edit_conflict', row: Object.assign({}, rowNine, { rev: 5 }) });
eq(e.editor.state, 'conflict', 'edit_conflict');
e = reduce(e, { type: 'edit_keep_mine' });
ok(e.editor.state === 'editing' && e.editor.baseRev === 5, 'edit_keep_mine');
const saved = reduce(e, { type: 'edit_save_ok', row: Object.assign({}, rowNine, { rev: 6 }) });
ok(saved.editor.state === 'closed' && saved.rows[0].rev === 6, 'edit_save_ok replaces the row and closes');
const created = reduce(reduce(s, { type: 'edit_new', kind: 'chat' }), { type: 'edit_save_ok', row: { id: 77, rev: 1 } });
ok(created.needsReload, 'a new row reloads the page');

// I6: a late save reply for another row leaves the open editor alone
const lateOther = reduce(reduce(s, { type: 'edit_open', row: rowNine }), { type: 'edit_save_ok', rowId: 4, row: { id: 4, rev: 2 } });
ok(lateOther.editor.state === 'editing' && lateOther.editor.rowId === 9, 'a late save of another row keeps the editor open');
eq(reduce(s, { type: 'edit_open', row: rowNine }).editor.rowId, 9, 'editor opened on row 9');

eq(reduce(Object.assign({}, s, { dataRev: 7, needsReload: false }), { type: 'remote_rev', data_rev: 8 }).needsReload, true, 'remote_rev ahead reloads');
const calm = Object.assign({}, s, { dataRev: 7, needsReload: false });
eq(reduce(calm, { type: 'remote_rev', data_rev: 9 }).needsReload, true, 'remote_rev newer reloads');
eq(reduce(Object.assign({}, calm, { dataRev: 9 }), { type: 'remote_rev', data_rev: 9 }), Object.assign({}, calm, { dataRev: 9 }), 'remote_rev at the same revision changes nothing');
eq([T.lastOffset(0, 50), T.lastOffset(50, 50), T.lastOffset(51, 50)], [0, 0, 50], 'lastOffset');
eq(T.query(Object.assign({}, s, { filter: Object.assign({}, s.filter, { q: 'x' }) })), { offset: 0, limit: 50, sort: 'seq', dir: 'asc', q: 'x' }, 'query drops empty filters');

// ---- goal patch (C1) and a missing job (I3) -------------------------------------------------------------------------------
const baseGoal = P.projects.draftOf({ recipe: 'assistant', goal_text: 'old', profile_id: 'p', labels: [] });
const myDraft = Object.assign({}, baseGoal, { profile_id: 'q' });
const refreshed = P.projects.draftOf({ recipe: 'assistant', goal_text: 'changed elsewhere', profile_id: 'p', labels: [] });
eq(P.projects.goalPatch(baseGoal, myDraft), { profile_id: 'q' }, 'the patch is computed against the base values');
eq(P.projects.goalPatch(refreshed, myDraft), { goal_text: 'old', profile_id: 'q' }, 'against the refreshed project the same draft would revert the other change');
const jobs = new Map([['a', { id: 'a', state: 'running' }], ['b', { id: 'b', state: 'running' }]]);
ok(!P.pruneMissingJob(jobs, 'a', { status: 500 }) && jobs.size === 2, 'a server error keeps the job');
ok(P.pruneMissingJob(jobs, 'a', { status: 404 }) && !jobs.has('a'), 'a 404 drops the job');
ok(P.pruneMissingJob(jobs, 'b', { status: 400, body: { code: 'job_not_found' } }) && jobs.size === 0, 'job_not_found drops the job');

// ---- rowPreview -----------------------------------------------------------------------------------------------------------
eq(T.rowPreview({ kind: 'chat', messages: [
    { role: 'system', content: 's' }, { role: 'user', content: 'a' }, { role: 'assistant', content: 'b' },
    { role: 'user', content: 'c' }, { role: 'assistant', content: 'd' }] }), { prompt: 'c', answer: 'd' }, 'preview of a chat row');
eq(T.rowPreview({ kind: 'text', text: 'hello' }), { prompt: 'hello', answer: '' }, 'preview of a text row');
const longAnswer = T.rowPreview({ kind: 'chat', messages: [{ role: 'user', content: 'q' }, { role: 'assistant', content: 'x'.repeat(300) }] }).answer;
ok(longAnswer.length === 240 && longAnswer.endsWith('…'), 'a long answer is cut to 240 characters');

// ---- moodFor --------------------------------------------------------------------------------------------------------------
const mood = P.mascot.moodFor;
eq(mood({ disabled: true, jobsActive: true }), 'sleeping', 'mood: disabled');
eq(mood({ jobsActive: true, blocked: true }), 'thinking', 'mood: working beats blocked');
eq(mood({ blocked: true }), 'worried', 'mood: blocked');
eq(mood({ jobFailed: true }), 'worried', 'mood: failed job');
eq(mood({ stepState: 'ready' }), 'happy', 'mood: ready');
eq(mood({}), 'idle', 'mood: idle');
eq(mood({ runActive: true, runFailed: true }), 'thinking', 'mood: an active run is work');
eq(mood({ runFailed: true }), 'worried', 'mood: failed run');
eq(mood({ runSucceeded: true, stepState: 'done' }), 'celebrating', 'mood: run succeeded');

// ---- runs, links ----------------------------------------------------------------------------------------------------------
ok(P.runTerminal('failed') && !P.runTerminal('running') && !P.runTerminal('weird'), 'runTerminal');
const runs = [{ id: 'b', state: 'running', created: '2026-11-23T10:00:00Z' }, { id: 'a', state: 'succeeded', created: '2026-11-22T10:00:00Z' }];
eq(P.activeRun(runs).id, 'b', 'activeRun');
eq(P.activeRun([{ id: 'x', state: 'weird' }]), null, 'an unknown state is not active');
const up = P.upsertRun(runs, { id: 'b', state: 'downloading' });
ok(up[0].id === 'b' && up[0].state === 'downloading' && up[0].created === '2026-11-23T10:00:00Z', 'upsert merges and keeps fields');
eq(P.upsertRun(runs, { id: 'c', state: 'queued', created: '2026-11-24T00:00:00Z' }).map(r => r.id), ['c', 'b', 'a'], 'newest first');
eq(P.upsertRun(runs, { id: 'c', created: '2026-11-24T00:00:00Z' }, 2).length, 2, 'limit');
eq([P.safeLink('/config#kaggle'), P.safeLink('//evil.test/x'), P.safeLink('javascript:void(0)'), P.safeLink('http://example.test/'), P.safeLink('https://example.test/a b')],
    ['/config#kaggle', '', '', '', 'https://example.test/a%20b'], 'safeLink');

// ---- preflight, enable flags, overrides -----------------------------------------------------------------------------------
const pf = { ready: false, items: [
    { id: 'kaggle_enabled', group: 'kaggle', ok: false, level: 'block', flag: 'kaggle.enabled' },
    { id: 'dataset_ready', group: 'data', ok: true, level: 'block' },
    { id: 'mystery', group: 'elsewhere', ok: false, level: 'warn' },
    { id: 'kaggle_allow_delete', group: 'kaggle', ok: false, level: 'warn', flag: 'kaggle.allow_delete' },
    { id: 'kaggle_quota', group: 'quota', ok: true, level: 'block' }], fix: { flags: ['kaggle.allow_delete', 'kaggle.enabled', 'kaggle.enabled'] } };
const g = plain(P.training.groupPreflight(pf));
eq(g.groups.map(x => x.id), ['data', 'kaggle', 'quota', 'other'], 'groups in fixed order, unknown last');
eq([g.blocks, g.warns, g.ready], [1, 2, false], 'preflight counts');
eq(plain(P.training.groupPreflight({ ready: true, items: [] })).ready, true, 'an empty ready list');
eq(P.training.enableFlags(pf), [{ flag: 'kaggle.enabled', id: 'kaggle_enabled', optional: false }, { flag: 'kaggle.allow_delete', id: 'kaggle_allow_delete', optional: true }], 'required first, deduped');
eq(P.training.enableFlags({ items: [], fix: null }), [], 'no fix');
const gemma = { method_default: 'full', methods: ['full', 'lora'], max_seq_cap: 2048 };
const qwen = { method_default: 'lora', methods: ['lora'], max_seq_cap: 2048 };
const v = (d, prof) => plain(P.training.validateOverrides(d, prof));
eq(v({}, qwen), { overrides: {}, issues: [] }, 'empty is automatic');
eq(v({ epochs: '3', learning_rate: '0,0002', max_seq_len: '1024', lora_r: '16', lora_alpha: '32' }, qwen).overrides,
    { epochs: 3, learning_rate: 0.0002, max_seq_len: 1024, lora_r: 16, lora_alpha: 32 }, 'valid values');
eq(v({ method: 'full' }, qwen).issues.map(i => i.field), ['method'], 'qwen has no full method');
eq(v({ epochs: '11', learning_rate: '0.01', max_seq_len: '4096', lora_r: '0', lora_alpha: '1.5' }, qwen).issues.map(i => i.field),
    ['epochs', 'learning_rate', 'max_seq_len', 'lora_r', 'lora_alpha'], 'out of range');
eq(v({ lora_r: '999' }, gemma).issues, [], 'gemma defaults to full: LoRA fields are ignored');
eq(v({ method: 'lora', lora_r: '999' }, gemma).issues.map(i => i.field), ['lora_r'], 'gemma with LoRA checks the rank');
eq(v({ max_seq_len: '300' }, { method_default: 'lora', context_max: 32768 }).issues.map(i => i.field), ['max_seq_len'], 'not a bucket');
eq(P.training.seqOptions({ method_default: 'lora', context_max: 32768 }), [256, 512, 1024, 2048, 4096], 'no cap: buckets up to 4096');
eq(P.mascot.MOODS.length, 6, 'six moods');

// ---- run timeline, metrics, events ------------------------------------------------------------------------------------------
const R = P.run, C = P.chart;
const phases = run => plain(R.timeline(run).phases.map(x => x.status));
eq(R.STAGES, ['boot', 'env_check', 'data', 'install', 'load_base', 'converter', 'base_eval', 'train', 'tuned_eval', 'save', 'gguf', 'manifest'], 'stages in run order');
eq(phases({ state: 'uploading' }), ['done', 'active', 'todo', 'todo', 'todo', 'todo'], 'timeline: uploading');
const tl = plain(R.timeline({ state: 'running', stage: { name: 'train', state: 'start' } }));
eq(tl.phases.map(x => x.status), ['done', 'done', 'done', 'active', 'todo', 'todo'], 'timeline: running');
eq(tl.stages.slice(6, 9).map(x => x.status), ['done', 'active', 'todo'], 'timeline: stages around train');
eq(plain(R.timeline({ state: 'running', stage: { name: 'converter', state: 'done' } })).stages.slice(4, 8).map(x => x.status), ['done', 'done', 'todo', 'todo'], 'timeline: converter stage');
eq(phases({ state: 'succeeded' }), ['done', 'done', 'done', 'done', 'done', 'done'], 'timeline: succeeded');
eq(phases({ state: 'failed', last_state: 'waiting_dataset' }), ['done', 'failed', 'skipped', 'skipped', 'skipped', 'skipped'], 'timeline: failed while waiting');
eq(phases({ state: 'cancelled', last_state: 'queued' }), ['stopped', 'skipped', 'skipped', 'skipped', 'skipped', 'skipped'], 'timeline: cancelled early');
eq(phases({ state: 'failed', error_code: 'download_failed' }), ['done', 'done', 'done', 'done', 'failed', 'skipped'], 'timeline: fallback by code');
const gg = plain(R.timeline({ state: 'failed', stage: { name: 'gguf', state: 'failed' } }));
ok(gg.phases[3].status === 'failed' && gg.stages[10].status === 'failed' && gg.stages[11].status === 'skipped', 'timeline: failed in gguf');
const unk = plain(R.timeline({ state: 'running', stage: { name: 'mystery', state: 'start' } }));
ok(unk.stages.every(x => x.status === 'todo'), 'timeline: an unknown stage marks nothing');
eq(plain(R.mergeMetrics([{ step: 20, kind: 'train', loss: 1 }], [{ step: 10, kind: 'train', loss: 2 }, { step: 20, kind: 'train', loss: 0.9 }, { step: 20, kind: 'eval', loss: 1.1 }]))
    .map(m => m.kind + m.step + ':' + m.loss), ['train10:2', 'train20:0.9', 'eval20:1.1'], 'mergeMetrics');
eq(R.elapsedSeconds({ state: 'running', created: '2026-11-23T10:00:00Z' }, Date.parse('2026-11-23T10:05:00Z')), 300, 'elapsed since creation');
eq(R.elapsedSeconds({ state: 'succeeded', remote_started: '2026-11-23T10:00:00Z', finished: '2026-11-23T10:01:00Z' }, 0), 60, 'elapsed of a finished run');
eq(R.elapsedSeconds({ state: 'succeeded', created: '2026-11-23T10:00:00Z', remote_started: '', finished: '', updated: '2026-11-23T10:02:00Z' }, 0), 120, 'empty strings fall back');
eq([R.liveNote({ state: 'running', live: 'no' }, 0), R.liveNote({ state: 'running', live: 'unknown' }, 0), R.liveNote({ state: 'running', live: 'yes' }, 5), R.liveNote({ state: 'succeeded', live: 'no' }, 0)],
    ['chibi_sensei.run.live_off', 'chibi_sensei.run.live_waiting', '', ''], 'liveNote');
const tt = (k, vars) => k + (vars ? JSON.stringify(vars) : '');
eq(R.eventText(tt, { kind: 'stage', data: { stage: 'train', state: 'start' } }), 'chibi_sensei.event.stage_start{"stage":"chibi_sensei.stage.train"}', 'event: stage');
eq(R.eventText(tt, { kind: 'stage', data: { stage: 'converter', state: 'done' } }), 'chibi_sensei.event.stage_done{"stage":"chibi_sensei.stage.converter"}', 'event: converter stage');
eq(R.eventText(tt, { kind: 'stage', data: { stage: 'brand_new', state: 'start' } }), 'chibi_sensei.event.stage_start{"stage":"chibi_sensei.stage.unknown"}', 'event: unknown stage');
eq(R.eventText(tt, { kind: 'error', data: { code: 'oom' } }), 'chibi_sensei.event.error{"text":"chibi_sensei.fail.oom"}', 'event: error');
eq(R.eventText(tt, { kind: 'error', data: { code: 'train_failed' } }), 'chibi_sensei.event.error{"text":"chibi_sensei.fail.train_failed"}', 'event: new failure code');
eq(R.eventText(tt, { kind: 'warn', data: { code: 'time_budget' } }), 'chibi_sensei.event.warn{"text":"chibi_sensei.warning.time_budget"}', 'event: warning');
eq(R.eventText(tt, { kind: 'state', data: { to: 'running' } }), 'chibi_sensei.event.state{"state":"chibi_sensei.run_state.running"}', 'event: state');
eq([['running', 'verifying'], ['running', 'cleaning'], ['verifying', 'cleaning'], ['downloading', 'succeeded'], ['verifying', 'failed'], ['running', 'downloading'], ['starting', 'running'], ['cleaning', 'succeeded'], ['succeeded', 'running']].map(x => R.metricsReloadDue(x[0], x[1])),
    [false, true, true, true, true, false, false, false, false], 'metricsReloadDue');
eq(['remote_timeout', 'data_changed', 'internal_error', 'storage_full'].map(c => R.failText(tt, c).fix), ['chibi_sensei.fail.remote_timeout_fix', 'chibi_sensei.fail.data_changed_fix', 'chibi_sensei.fail.internal_fix', 'chibi_sensei.fail.storage_full_fix'], 'failText: orchestrator codes');
eq(R.eventText(tt, { kind: 'warn', data: { code: 'cleanup_failed' } }), 'chibi_sensei.event.warn{"text":"chibi_sensei.warning.cleanup_failed"}', 'event: log highlight warning');
eq(['batch_reduced', 'eval_batch_reduced', 'oom_step_skipped', 'rows_truncated'].map(c => R.eventText(tt, { kind: 'warn', data: { code: c, message: 'English' } })),
    ['batch_reduced', 'eval_batch_reduced', 'oom_step_skipped', 'rows_truncated'].map(c => 'chibi_sensei.event.warn{"text":"chibi_sensei.warning.' + c + '"}'), 'event: trainer warning codes have texts');
eq(R.eventText(tt, { kind: 'mystery', data: { x: 1 } }), '', 'event: unknown kinds are skipped');
eq(R.failText(tt, 'env_low_disk'), { title: 'chibi_sensei.fail.env_low_disk', fix: 'chibi_sensei.fail.env_low_disk_fix' }, 'failText: known code');
eq(R.failText(tt, 'something_new'), { title: 'chibi_sensei.fail.unknown', fix: 'chibi_sensei.fail.unknown_fix' }, 'failText: unknown code is generic');
const conf = plain(R.confusion({ labels: ['pos', 'neg'], confusion: { base: [[4, 5, 1], [6, 4, 0]], tuned: [[9, 1, 0], [1, 9, 0]] } }, 'base'));
eq(conf.rows.map(r => r.label + ':' + r.cells.join('/')), ['pos:4/5/1', 'neg:6/4/0'], 'confusion: one column per label plus the no-label column');
eq(plain(R.confusion({ labels: ['a'], confusion: { base: [[1]] } }, 'base')).rows[0].cells, [1, 0], 'confusion: a short row is padded');
eq(R.confusion({ labels: ['a'], confusion: {} }, 'base'), null, 'confusion: missing grid');

// ---- chart layout --------------------------------------------------------------------------------------------------------
ok(C.layout([]).empty, 'chart: empty');
const pts = Array.from({ length: 1000 }, (_, i) => ({ step: (i + 1) * 2, kind: 'train', loss: 3 - i / 1000 }));
const L = plain(C.layout(pts.concat([{ step: 1000, kind: 'eval', loss: 2.4 }]), { maxSteps: 2000 }));
eq(L.trainCount, 500, 'chart: downsampled in buckets of two');
ok(L.path.charAt(0) === 'M' && (L.path.match(/L/g) || []).length === 499, 'chart: one command per point');
ok(L.summary.first === 3 && L.summary.steps === 2000 && Math.abs(L.summary.last - 2.001) < 1e-9 && Math.abs(L.summary.min - 2.001) < 1e-9, 'chart: summary from raw points');
eq(L.xTicks.map(x => x.value), [0, 500, 1000, 1500, 2000], 'chart: x ticks');
eq(L.evalPoints.length, 1, 'chart: eval point');
ok(L.yTicks.every(tk => tk.y >= L.plot.top && tk.y <= L.plot.bottom), 'chart: y ticks inside the plot');
const flat = plain(C.layout([{ step: 1, kind: 'train', loss: 1 }, { step: 2, kind: 'train', loss: 1 }]));
ok(!flat.empty && flat.yTicks[0].value < 1 && flat.yTicks[3].value > 1, 'chart: a flat line gets a range');
ok(C.layout([{ step: 1, kind: 'train', loss: NaN }, { step: 'x', loss: 1 }]).empty, 'chart: non-numbers are dropped');
ok(C.layout([{ step: 1, kind: 'eval', loss: null }]).empty, 'chart: a null loss is dropped');
eq(plain(C.layout([{ step: 0, kind: 'train', loss: 1 }, { step: 2, kind: 'train', loss: 2 }], { maxSteps: 2 })).xTicks.map(x => x.value), [0, 1, 2], 'chart: small step counts dedupe ticks');

// ---- stream parser and chatStream (P3) --------------------------------------------------------------------------------------
const S = () => P.createSseParser(new TextDecoder());
const bytes = text => new TextEncoder().encode(text);
let sse = S();
eq(sse.push('event: delta\ndata: {"text":').concat(sse.push('"hi"}\n\n')), [{ event: 'delta', data: '{"text":"hi"}' }], 'sse: a delta split inside its JSON is one event');
const utf = bytes('event: delta\ndata: {"text":"ä€😀"}\n\n');
let splitsOk = true;
for (let i = 0; i <= utf.length; i++) {
    const parser = S();
    const got = parser.push(utf.slice(0, i)).concat(parser.push(utf.slice(i)));
    if (got.length !== 1 || got[0].data !== '{"text":"ä€😀"}') splitsOk = false;
}
ok(splitsOk, 'sse: every split point of the UTF-8 bytes keeps the text byte-identical');
eq(S().push(': ping\n\n'), [], 'sse: a comment gives no event');
eq(S().push(': ping\nevent: end\ndata: {}\n\n'), [{ event: 'end', data: '{}' }], 'sse: a comment before an event');
eq(S().push('event: done\r\ndata: {}\r\n\r\n'), [{ event: 'done', data: '{}' }], 'sse: CRLF line ends');
sse = S();
eq(sse.push('event: done\ndata: {}\r').concat(sse.push('\n\r\n')), [{ event: 'done', data: '{}' }], 'sse: a CR at the end of a push waits for the LF');
eq(S().push('data: a\ndata: b\n\n'), [{ event: 'message', data: 'a\nb' }], 'sse: two data lines join with LF');
eq(S().push('data:x\n\n'), [{ event: 'message', data: 'x' }], 'sse: no space after the colon');
eq(S().push('event: x\n\n'), [], 'sse: an event without data is not dispatched');
sse = S();
eq(sse.push('event: done\ndata: {}\n'), [], 'sse: an unterminated event waits');
eq(sse.end(), [], 'sse: end() drops an unterminated event');
let tooLong = null;
try { S().push('data: ' + 'x'.repeat((1 << 20) + 1)); } catch (err) { tooLong = err; }
ok(tooLong && tooLong.code === 'bad_response', 'sse: a line over 1 MiB throws bad_response');

const sseBody = text => new TextEncoder().encode(text);
const streamResponse = (chunks, type) => {
    let i = 0;
    return {
        ok: true, status: 200, headers: { get: name => (name.toLowerCase() === 'content-type' ? type : null) },
        body: { getReader: () => ({ read: async () => (i < chunks.length ? { done: false, value: chunks[i++] } : { done: true, value: undefined }), cancel: async () => {} }) }
    };
};
const runChat = fetchFn => new Promise(resolve => {
    const events = [];
    const api = P.createApi({ fetch: fetchFn });
    api.chatStream({ run_id: 'r' }, { onEvent: ev => events.push(ev), onClose: result => resolve({ events, result: plain(result) }) });
});
let chat = await runChat(async () => streamResponse([
    sseBody('event: status\ndata: {"target":"tuned","state":"generating"}\n\nevent: del'),
    sseBody('ta\ndata: {"target":"tuned","text":"Hallo"}\n\n'),
    sseBody('event: done\ndata: {"target":"tuned","finish_reason":"stop"}\n\nevent: end\ndata: {}\n\n')
], 'text/event-stream'));
eq(chat.events.map(ev => ev.type), ['status', 'delta', 'done'], 'chat: (a) event types');
eq(chat.events[1].text, 'Hallo', 'chat: (a) the delta text');
eq(chat.result, { ended: true, aborted: false, error: null }, 'chat: (a) closes after end');
chat = await runChat(async () => ({ ok: false, status: 409, headers: { get: () => null }, json: async () => ({ code: 'runtime_busy', error: 'runtime_busy' }) }));
ok(chat.events.length === 0 && chat.result.error && chat.result.error.code === 'runtime_busy' && chat.result.ended === false, 'chat: (b) a JSON error answer before the stream');
chat = await runChat(async () => streamResponse([sseBody('{}')], 'application/json'));
ok(chat.result.error && chat.result.error.code === 'bad_response', 'chat: (c) a 200 that is not an event stream');
chat = await runChat(async () => streamResponse([sseBody('event: delta\ndata: {"target":"tuned","text":"x"}\n\n')], 'text/event-stream'));
ok(chat.result.error && chat.result.error.code === 'connection_lost' && chat.events.length === 1, 'chat: (d) a stream that ends without end');
chat = await runChat(async () => streamResponse([sseBody('event: delta\ndata: {"target":"provider","text":"x"}\n\nevent: end\ndata: {}\n\n')], 'text/event-stream'));
eq(chat.events, [], 'chat: (e) an unknown target is not forwarded');
chat = await runChat(async () => streamResponse([sseBody('event: done\ndata: {"target":"tuned"}\n\nevent: end\ndata: {}\n\nevent: delta\ndata: {"target":"tuned","text":"late"}\n\n')], 'text/event-stream'));
eq(chat.events.map(x => x.type), ['done'], 'chat: (f) events after end in the same chunk are dropped');
eq(chat.result, { ended: true, aborted: false, error: null }, 'chat: (f) closes once after end');
const throwing = await new Promise(resolve => {
    let closes = 0, cancelled = false;
    const res = streamResponse([sseBody('event: delta\ndata: {"target":"tuned","text":"x"}\n\n')], 'text/event-stream');
    const inner = res.body.getReader;
    res.body.getReader = () => Object.assign(inner(), { cancel: async () => { cancelled = true; } });
    P.createApi({ fetch: async () => res }).chatStream({ run_id: 'r' }, { onEvent: () => { throw new Error('boom'); }, onClose: result => { closes++; setTimeout(() => resolve({ closes, cancelled, result: plain(result) }), 20); } });
});
ok(throwing.closes === 1 && throwing.cancelled && throwing.result.error && throwing.result.error.code === 'bad_response', 'chat: (g) a throwing onEvent cancels the reader and closes once with bad_response');

// ---- chat reducer, conversation, limits (P3) ------------------------------------------------------------------------------
const K = P.chat;
const LIM = { max_turns: 20, max_chars: 16000 };
const act = (state, ...actions) => actions.reduce((acc, a) => K.reduce(acc, a), state);
const ev = (type, target, rest) => ({ type: 'event', ev: Object.assign({ type, target }, rest || {}) });
let cs = act(K.initialState(), { type: 'send', text: ' hi ' });
ok(cs.streaming && cs.turns.length === 1 && cs.turns[0].user === 'hi' && cs.turns[0].answers.base === null && cs.turns[0].answers.tuned.state === 'waiting', 'chat: send without compare has no base answer');
const withBase = act(K.initialState(), { type: 'compare', on: true }, { type: 'send', text: 'hi' });
ok(withBase.turns[0].answers.base.state === 'waiting' && withBase.turns[0].answers.tuned.state === 'waiting', 'chat: send with compare starts both answers');
ok(K.reduce(cs, { type: 'send', text: 'again' }) === cs, 'chat: send while streaming changes nothing');
ok(K.reduce(K.initialState(), { type: 'send', text: '  ' }).turns.length === 0, 'chat: an empty send is ignored');
cs = act(cs, ev('status', 'tuned', { state: 'loading' }));
eq(cs.turns[0].answers.tuned.state, 'loading', 'chat: status loading');
cs = act(cs, ev('status', 'tuned', { state: 'generating' }), ev('delta', 'tuned', { text: 'Ha' }), ev('delta', 'tuned', { text: 'llo' }));
ok(cs.turns[0].answers.tuned.state === 'streaming' && cs.turns[0].answers.tuned.text === 'Hallo' && cs.turns[0].started, 'chat: two deltas concatenate');
cs = act(cs, ev('delta', 'tuned', { text: 42 }));
eq(cs.turns[0].answers.tuned.text, 'Hallo', 'chat: a delta without a string is dropped');
ok(K.reduce(cs, ev('delta', 'base', { text: 'x' })) === cs, 'chat: an event for a target without an answer changes nothing');
cs = act(cs, ev('done', 'tuned', { finish_reason: 'stop', prompt_tokens: 3, completion_tokens: 5, ms: 40, tok_s: 12.5, backend: 'cpu', extra: 1 }));
eq(cs.turns[0].answers.tuned.stats, { finish_reason: 'stop', prompt_tokens: 3, completion_tokens: 5, ms: 40, tok_s: 12.5, backend: 'cpu' }, 'chat: done stores the stats');
ok(K.reduce(cs, ev('delta', 'tuned', { text: 'late' })) === cs, 'chat: a final answer takes no more events');
const cancelled = act(act(K.initialState(), { type: 'send', text: 'x' }), ev('error', 'tuned', { code: 'cancelled' }));
ok(cancelled.turns[0].answers.tuned.state === 'stopped', 'chat: the error cancelled is a stop, not a failure');
const strange = act(act(K.initialState(), { type: 'send', text: 'x' }), ev('error', 'tuned', { code: 'weird_code' }));
ok(strange.turns[0].answers.tuned.state === 'error' && strange.turns[0].answers.tuned.code === 'unknown', 'chat: a strange error code becomes unknown');
const known = act(act(K.initialState(), { type: 'send', text: 'x' }), ev('error', 'tuned', { code: 'context_exceeded' }));
eq(known.turns[0].answers.tuned.code, 'context_exceeded', 'chat: a known error code is kept');
const inherited = act(act(K.initialState(), { type: 'send', text: 'x' }), ev('error', 'tuned', { code: 'constructor' }));
eq(inherited.turns[0].answers.tuned.code, 'unknown', 'chat: an inherited name is not an error code');
const lost = act(act(K.initialState(), { type: 'send', text: 'x' }), ev('delta', 'tuned', { text: 'ab' }), { type: 'close', ended: false, aborted: false, error: { code: 'network' } });
ok(!lost.streaming && lost.turns[0].answers.tuned.state === 'error' && lost.turns[0].answers.tuned.code === 'connection_lost' && lost.turns[0].answers.tuned.text === 'ab', 'chat: close without end turns in-flight answers into connection_lost');
const aborted = act(act(K.initialState(), { type: 'send', text: 'x' }), ev('delta', 'tuned', { text: 'ab' }), { type: 'close', ended: false, aborted: true, error: null });
ok(aborted.turns[0].answers.tuned.state === 'stopped' && !aborted.streaming, 'chat: close aborted gives stopped');
const early = act(K.initialState(), { type: 'send', text: 'first' }, { type: 'close', ended: false, aborted: false, error: { code: 'runtime_busy', status: 409 } });
ok(early.turns.length === 0 && early.draft === 'first' && early.notice.code === 'runtime_busy' && !early.streaming, 'chat: an error before the stream removes the turn, restores the draft and sets the notice');
const typed = act(K.initialState(), { type: 'send', text: 'first' }, { type: 'draft', text: 'second' }, { type: 'close', ended: false, aborted: false, error: { code: 'x' } });
eq(typed.draft, 'first\nsecond', 'chat: a refused text and what the user typed meanwhile are both kept');
const preset = act(K.initialState(), { type: 'system', text: 'S' }, { type: 'draft', text: 'U' });
ok(preset.system === 'S' && preset.draft === 'U' && !preset.streaming, 'chat: a prompt set while no composer is shown stays in the draft state');
const busyChat = act(K.initialState(), { type: 'send', text: 'x' });
ok(K.reduce(busyChat, { type: 'new_chat' }) === busyChat, 'chat: new_chat is ignored while streaming');
const midChat = act(K.initialState(), { type: 'send', text: 'x' }, ev('delta', 'tuned', { text: 'a' }), ev('done', 'tuned', {}), { type: 'close', ended: true, aborted: false, error: null }, { type: 'compare', on: true });
ok(midChat.compare && midChat.compareFrom === 1, 'chat: compare on mid-chat sets compareFrom');
const cleared = act(midChat, { type: 'new_chat' });
ok(cleared.turns.length === 0 && cleared.compareFrom === -1 && cleared.compare, 'chat: new_chat keeps the compare switch and drops the turns');

const done1 = (state, user, tuned, base) => {
    let x = act(state, { type: 'send', text: user }, ev('delta', 'tuned', { text: tuned }), ev('done', 'tuned', {}));
    if (base !== undefined) x = act(x, ev('delta', 'base', { text: base }), ev('done', 'base', {}));
    return act(x, { type: 'close', ended: true, aborted: false, error: null });
};
let conv = act(K.initialState(), { type: 'system', text: ' Be brief ' });
conv = done1(conv, 'q1', 'a1');
eq(K.conversation(conv, 'tuned'), [{ role: 'system', content: 'Be brief' }, { role: 'user', content: 'q1' }, { role: 'assistant', content: 'a1' }], 'conversation: a leading system prompt');
let failedTurn = act(K.initialState(), { type: 'send', text: 'q1' }, ev('error', 'tuned', { code: 'generation_failed' }), { type: 'close', ended: true, aborted: false, error: null });
failedTurn = done1(failedTurn, 'q2', 'a2');
eq(K.conversation(failedTurn, 'tuned'), [{ role: 'user', content: 'q2' }, { role: 'assistant', content: 'a2' }], 'conversation: a turn with a failed tuned answer is left out');
let mixed = done1(K.initialState(), 'q1', 'a1');
mixed = act(mixed, { type: 'compare', on: true });
mixed = done1(mixed, 'q2', 'a2', 'b2');
eq(K.conversation(mixed, 'base'), [{ role: 'user', content: 'q2' }, { role: 'assistant', content: 'b2' }], 'conversation: base skips the turns without a base answer');
eq(K.conversation(mixed, 'tuned').length, 4, 'conversation: tuned keeps both turns');
const partial = act(K.initialState(), { type: 'send', text: 'q' }, ev('delta', 'tuned', { text: 'half' }), { type: 'close', ended: false, aborted: true, error: null });
eq(K.conversation(partial, 'tuned'), [{ role: 'user', content: 'q' }, { role: 'assistant', content: 'half' }], 'conversation: a stopped answer with text is kept');
ok(!K.usable({ state: 'stopped', text: '  ' }) && !K.usable({ state: 'error', text: 'x' }) && K.usable({ state: 'done', text: 'x' }), 'usable: only final answers with text');

eq(K.canSend(K.initialState(), LIM, '   '), { ok: false, reasonKey: '' }, 'canSend: empty text has no reason');
eq(K.canSend(busyChat, LIM, 'y').reasonKey, 'chibi_sensei.chat.busy', 'canSend: busy while streaming');
let long = K.initialState();
for (let i = 0; i < 20; i++) long = done1(long, 'q' + i, 'a' + i);
eq(K.canSend(long, LIM, 'one more'), { ok: false, reasonKey: 'chibi_sensei.chat.limit_turns', vars: { max: 20 } }, 'canSend: 20 usable turns are the limit');
ok(K.canSend(K.initialState(), LIM, '😀'.repeat(16000)).ok, 'canSend: 16000 code points pass');
eq(K.canSend(K.initialState(), LIM, '😀'.repeat(16001)).reasonKey, 'chibi_sensei.chat.limit_chars', 'canSend: 16001 code points are too many (code points, not UTF-16 units)');
const body = K.requestBody(mixed, 'run-1', ' next ', {});
eq(body.targets.map(x => x.kind), ['tuned', 'base'], 'requestBody: targets in order');
ok(body.targets.every(x => x.messages[x.messages.length - 1].role === 'user' && x.messages[x.messages.length - 1].content === 'next') && body.run_id === 'run-1', 'requestBody: every conversation ends with the new user message');
ok(!('sampling' in body) && K.requestBody(mixed, 'r', 'x', { top_k: 5 }).sampling.top_k === 5, 'requestBody: sampling only when set');

// ---- sampling, runtime view, answer view ----------------------------------------------------------------------------------
eq(K.validateSampling({ temperature: '0,7' }, 1024), { sampling: { temperature: 0.7 }, issues: [] }, 'sampling: a decimal comma');
eq(K.validateSampling({ temperature: '', top_p: ' ', top_k: '', max_tokens: '', seed: '5' }, 1024), { sampling: {}, issues: [] }, 'sampling: empty fields and the seed are left out');
eq(K.validateSampling({ top_p: '0' }, 1024).issues, [{ field: 'top_p', min: 0.01, max: 1 }], 'sampling: top_p of 0 is an issue');
eq(K.validateSampling({ top_k: '201' }, 1024).issues, [{ field: 'top_k', min: 0, max: 200 }], 'sampling: top_k above 200');
eq(K.validateSampling({ max_tokens: '1025' }, 1024).issues, [{ field: 'max_tokens', min: 1, max: 1024 }], 'sampling: max_tokens above the cap');
ok(K.validateSampling({ top_k: '2.5' }, 1024).issues.length === 1 && K.validateSampling({ temperature: '2.1' }, 1024).issues.length === 1 && K.validateSampling({ temperature: 'abc' }, 1024).issues.length === 1, 'sampling: fractions, range and text are issues');
eq(K.validateSampling({ temperature: '2', top_p: '1', top_k: '0', max_tokens: '1' }, 1024).sampling, { temperature: 2, top_p: 1, top_k: 0, max_tokens: 1 }, 'sampling: the limits themselves pass');
const rv = (rt, admin, readonly) => plain(K.runtimeView(rt, admin, readonly));
eq([rv({ state: 'not_installed', instances: [] }, true, false).card, rv({ state: 'not_installed', instances: [] }, false, false).card], ['install', 'ask'], 'runtimeView: install for an admin, ask for everyone else');
const readyView = rv({ state: 'ready', instances: [{}] }, false, false);
ok(readyView.composer && readyView.canStop && readyView.card === null, 'runtimeView: ready has the composer and no card');
ok(!rv({ state: 'ready', instances: [] }, false, false).canStop, 'runtimeView: nothing to stop without instances');
eq(['installing', 'disabled', 'unsupported', 'failed'].map(state => rv({ state, instances: [] }, true, false).card), ['installing', 'disabled', 'unsupported', 'failed'], 'runtimeView: one card per state');
const roView = rv({ state: 'not_installed', instances: [] }, true, true);
ok(!roView.installEnabled && roView.installReasonKey === 'chibi_sensei.readonly_banner', 'runtimeView: read-only blocks the install with its reason');
ok(rv(undefined, false, false).state === 'loading' && rv(null, false, false).state === 'error' && !rv(undefined, false, false).composer, 'runtimeView: undefined is loading, null is error');
eq(['waiting', 'loading', 'streaming'].map(x => plain(K.answerView({ state: x })).state), ['streaming', 'streaming', 'streaming'], 'answerView: the three live states');
eq(['waiting', 'loading', 'streaming'].map(x => plain(K.answerView({ state: x })).phase), ['waiting', 'loading', 'generating'], 'answerView: the phases');
eq(['done', 'stopped', 'error'].map(x => plain(K.answerView({ state: x })).state), ['done', 'stopped', 'error'], 'answerView: final states');

// ---- test step helpers ---------------------------------------------------------------------------------------------------------
const TS = P.test;
eq(TS.succeededRuns([{ id: 'a', state: 'failed' }, { id: 'b', state: 'succeeded', created: '2026-11-01T10:00:00Z' }, { id: 'c', state: 'succeeded', created: '2026-10-01T10:00:00Z', finished: '2026-11-05T10:00:00Z' }, null]).map(r => r.id), ['c', 'b'], 'succeededRuns: filters and sorts newest first');
eq(TS.smokeLine(null), null, 'smokeLine: null');
eq(TS.smokeLine({ state: 'ok', ms: 812 }), { key: 'chibi_sensei.test.smoke_ok', vars: { time: '812 ms' }, tone: '' }, 'smokeLine: ok');
eq(TS.smokeLine({ state: 'failed' }).tone, 'warn', 'smokeLine: failed is a warning');
eq(['runtime_not_installed', 'runtime_busy', 'runtime_disabled', 'new_code'].map(code => TS.smokeLine({ state: 'skipped', code }).key),
    ['chibi_sensei.test.smoke_skipped_installed', 'chibi_sensei.test.smoke_skipped_busy', 'chibi_sensei.test.smoke_skipped_disabled', 'chibi_sensei.test.smoke_skipped'], 'smokeLine: skip codes');
eq(TS.smokeLine({ state: 'skipped', code: 'constructor' }).key, 'chibi_sensei.test.smoke_skipped', 'smokeLine: an inherited name is not a code');
eq(['missing', 'skipped', 'invalid', 'something'].map(reason => TS.modelLine('base', { available: false, reason }).key),
    ['chibi_sensei.test.base_missing', 'chibi_sensei.test.base_skipped', 'chibi_sensei.test.base_invalid', 'chibi_sensei.test.base_missing'], 'modelLine: base reasons, unknown counts as missing');
eq(TS.modelLine('tuned', { available: true, bytes: 2048 }).key, 'chibi_sensei.test.tuned_ready', 'modelLine: tuned ready');
eq(TS.modelLine('tuned', { available: false, reason: 'invalid' }).key, 'chibi_sensei.test.tuned_invalid', 'modelLine: tuned invalid');
eq([TS.baseReasonKey('skipped'), TS.baseReasonKey('nope')], ['chibi_sensei.test.base_skipped', 'chibi_sensei.test.base_missing'], 'baseReasonKey');
eq(TS.promptOf({ prompt_messages: [{ role: 'system', content: 'S' }, { role: 'user', content: 'U' }] }), { system: 'S', user: 'U' }, 'promptOf: system and user');
eq(TS.promptOf({ prompt_messages: [{ role: 'user', content: 'u1' }, { role: 'assistant', content: 'a1' }, { role: 'user', content: 'u2' }] }), { system: '', user: 'u2' }, 'promptOf: the last user message of a multi-turn prompt');
eq(TS.promptOf({ prompt_messages: null, prompt_text: 'plain' }), { system: '', user: 'plain' }, 'promptOf: a text prompt');

console.log('chibi-sensei model: ok (' + checks + ' checks)');
