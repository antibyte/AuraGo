// Chibi Sensei desktop app: Node test of the pure logic (readiness, primary action, tips, the table reducer, row previews and the
// mascot mood). The four modules run in one vm realm without a DOM. Run: node scripts/test-chibi-sensei-model.mjs
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';

const root = path.resolve(import.meta.dirname, '..');
const files = ['chibi-sensei-core.js', 'chibi-sensei-mascot.js', 'chibi-sensei-steps.js', 'chibi-sensei-table.js', 'chibi-sensei-projects.js', 'chibi-sensei-training.js', 'chibi-sensei-chart.js', 'chibi-sensei-run.js'];
const read = file => fs.readFileSync(path.join(root, 'ui/js/desktop/apps/chibi-sensei', file), 'utf8');
const plain = value => JSON.parse(JSON.stringify(value));
let checks = 0;
// The modules run in their own vm realm, so arrays and objects they return are compared as plain JSON.
const eq = (actual, expected, message) => { checks++; assert.deepStrictEqual(plain(actual), plain(expected), message); };
const ok = (value, message) => { checks++; assert.ok(value, message); };

const window = {};
vm.runInNewContext(files.map(read).join('\n'), { window, console, Intl, Map, Set, JSON, Math, Date, URL });
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
for (const step of ['test', 'use']) {
    p = primary(step, {}, {});
    ok(p.disabled && p.reasonKey === 'chibi_sensei.primary.later', step + ' comes later');
}
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

console.log('chibi-sensei model: ok (' + checks + ' checks)');
