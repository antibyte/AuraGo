#!/usr/bin/env node
// Runs the Mission Control modules (schedule, triggers, menus, list, detail and the app shell) in Node on a
// small stub DOM and checks how they treat EasyDrag flow missions (execution_type "flow"): the trigger summary,
// the Flows filter, Open in EasyDrag instead of the editor, the flow delete text, Run and Resume of an
// unpublished flow, the 409 cancel hint, the run toast and the PUT body that the server accepts.
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const apps = path.join(here, '..', 'ui', 'js', 'desktop', 'apps');

// ── stub DOM: enough for innerHTML-built modules; querySelector hands out one stub per selector ──
class ClassList {
    constructor() { this.set = new Set(); }
    add(...names) { names.forEach(n => this.set.add(n)); }
    remove(...names) { names.forEach(n => this.set.delete(n)); }
    toggle(name, on) { const next = on === undefined ? !this.set.has(name) : !!on; if (next) this.set.add(name); else this.set.delete(name); return next; }
    contains(name) { return this.set.has(name); }
}
class El {
    constructor(tag) {
        this.tagName = String(tag || 'div').toUpperCase();
        this.children = []; this.parent = null; this.attrs = {}; this.dataset = {}; this.listeners = {};
        this.style = { setProperty() {} }; this.classList = new ClassList(); this.queries = new Map();
        this.innerHTML = ''; this.textContent = ''; this.hidden = false; this.value = ''; this.scrollTop = 0; this.disabled = false;
    }
    set className(value) { this.classList = new ClassList(); String(value).split(/\s+/).filter(Boolean).forEach(c => this.classList.add(c)); }
    get className() { return [...this.classList.set].join(' '); }
    setAttribute(name, value) { this.attrs[name] = String(value); }
    getAttribute(name) { return Object.prototype.hasOwnProperty.call(this.attrs, name) ? this.attrs[name] : null; }
    removeAttribute(name) { delete this.attrs[name]; }
    addEventListener(type, fn) { (this.listeners[type] = this.listeners[type] || []).push(fn); }
    removeEventListener() {}
    appendChild(node) {
        if (node.tagName === '#FRAG') { node.children.slice().forEach(child => this.appendChild(child)); node.children = []; return node; }
        node.remove();
        node.parent = this;
        this.children.push(node);
        return node;
    }
    append(...nodes) { nodes.forEach(n => this.appendChild(n)); }
    remove() { if (!this.parent) return; const list = this.parent.children; list.splice(list.indexOf(this), 1); this.parent = null; }
    replaceChildren(...nodes) { this.children.slice().forEach(c => c.remove()); nodes.forEach(n => this.appendChild(n)); }
    get firstElementChild() { return this.children[0] || new El('div'); }
    querySelector(sel) { if (!this.queries.has(sel)) this.queries.set(sel, new El('div')); return this.queries.get(sel); }
    querySelectorAll() { return []; }
    closest() { return null; }
    contains() { return false; }
    focus() {}
    scrollIntoView() {}
    getBoundingClientRect() { return { left: 0, top: 0, right: 0, bottom: 0, width: 0, height: 0 }; }
    fire(type, event) { (this.listeners[type] || []).forEach(fn => fn(event)); }
}
const document = {
    createElement: (tag) => new El(tag),
    createDocumentFragment: () => new El('#frag'),
    activeElement: null,
    documentElement: { lang: 'en' },
    addEventListener() {},
    removeEventListener() {}
};

const sandbox = { window: { SYSTEM_LANG: 'en' }, document, console, setTimeout, clearTimeout, setInterval, clearInterval, URLSearchParams, Intl };
sandbox.globalThis = sandbox;
vm.createContext(sandbox);
for (const file of ['mission-control-schedule.js', 'mission-control-triggers.js', 'mission-control-menus.js', 'mission-control-list.js', 'mission-control-detail.js', 'mission-control.js']) {
    const full = path.join(apps, file);
    vm.runInContext(fs.readFileSync(full, 'utf8'), sandbox, { filename: full });
}
const W = sandbox.window;
const S = W.MissionControlSchedule, TR = W.MissionControlTriggers, MN = W.MissionControlMenus;

let failures = 0;
function check(name, cond, detail) {
    if (cond) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + (detail ? ' — ' + detail : ''));
}
function eq(name, got, want) { check(name, JSON.stringify(got) === JSON.stringify(want), `got ${JSON.stringify(got)} want ${JSON.stringify(want)}`); }
const tick = () => new Promise(resolve => setTimeout(resolve, 0));

// The translator returns the key, plus the params as JSON.
const t = (key, params) => key + (params ? ' ' + JSON.stringify(params) : '');
const esc = (s) => String(s == null ? '' : s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
const svg = MN.ICONS;
const fmt = { relative: () => 'rel', dateTime: () => 'date', duration: () => 'dur', clock: () => '00:00' };

const agent = { id: 'agent-1', name: 'B agent', execution_type: 'manual', prompt: 'Do it', enabled: true, priority: 'medium' };
const scheduled = { id: 'sched-1', name: 'C scheduled', execution_type: 'scheduled', schedule: '0 9 * * *', prompt: 'Daily', enabled: true, next_run: '2026-10-08T09:00:00Z' };
const flowLive = {
    id: 'flow-1', name: 'A flow', execution_type: 'flow', flow_id: 'f_live', flow_published: true, enabled: true, priority: 'medium', prompt: '',
    next_run: '2026-10-07T09:00:00Z',
    flow_triggers: [{ node_id: 'n1', trigger_type: 'schedule', schedule: '0 9 * * *' }, { node_id: 'n2', trigger_type: 'manual' }]
};
const flowDraft = { id: 'flow-2', name: 'D draft flow', execution_type: 'flow', flow_id: 'f_draft', enabled: false, priority: 'medium', prompt: '' };
// Several flows hold this mission: its cancel answers FLOW_MISSION_AMBIGUOUS.
const flowTwin = { id: 'flow-3', name: 'E twin flow', execution_type: 'flow', flow_id: 'f_twin', flow_published: true, enabled: true, priority: 'medium', prompt: '', flow_triggers: [{ node_id: 'm', trigger_type: 'manual' }] };
// Hostile text from a trigger type and a name must render escaped.
const flowOdd = { id: 'flow-4', name: 'F <i>odd</i> flow', execution_type: 'flow', flow_id: 'f_odd', flow_published: true, enabled: true, priority: 'medium', prompt: '', flow_triggers: [{ node_id: 'x', trigger_type: '<b>x</b>' }] };
const agentPaused = { id: 'agent-2', name: 'G paused agent', execution_type: 'manual', prompt: 'Later', enabled: false, priority: 'medium' };
const ALL = [agent, scheduled, flowLive, flowDraft, flowTwin, flowOdd, agentPaused];

// ── flowSummary through MissionControlTriggers.summary ──
const ctx = { schedule: S, lang: 'en' };
eq('summary: unpublished flow', TR.summary(flowDraft, t, ctx), 'desktop.mc_flow_unpublished');
eq('summary: published flow without triggers', TR.summary({ execution_type: 'flow', flow_published: true, flow_triggers: [] }, t, ctx), 'desktop.mc_flow_no_trigger');
eq('summary: missing flow_triggers counts as none', TR.summary({ execution_type: 'flow', flow_published: true }, t, ctx), 'desktop.mc_flow_no_trigger');
const mixed = {
    execution_type: 'flow', flow_published: true, flow_triggers: [
        { node_id: 'a', trigger_type: 'schedule', schedule: '*/15 * * * *' },
        { node_id: 'b', trigger_type: 'datetime' },
        { node_id: 'c', trigger_type: 'manual' },
        { node_id: 'd', trigger_type: 'webhook', trigger_config: { webhook_id: 'w1' } },
        { node_id: 'e', trigger_type: 'manual' },
        { node_id: 'f', trigger_type: 'datetime' },
        { node_id: 'g', trigger_type: 'schedule' },
        { node_id: 'h', trigger_type: 'custom_thing' }
    ]
};
eq('summary: schedule, datetime, manual, other; duplicates removed', TR.summary(mixed, t, ctx),
    [S.describe('*/15 * * * *', t, 'en'), 'desktop.mc_flow_trigger_datetime', 'desktop.mc_filter_manual', TR.label(TR.byKey('webhook'), t), 'desktop.mc_filter_scheduled', 'custom_thing'].join(' · '));
eq('summary: agent missions unchanged', TR.summary(scheduled, t, ctx), S.describe('0 9 * * *', t, 'en'));
eq('summary: an unknown trigger type is passed on as text (renderers escape it)', TR.summary(flowOdd, t, ctx), '<b>x</b>');

// ── shared predicates ──
check('isFlow / isUnpublishedFlow', TR.isFlow(flowLive) && !TR.isFlow(agent) && !TR.isFlow(null) && TR.isUnpublishedFlow(flowDraft) && !TR.isUnpublishedFlow(flowLive) && !TR.isUnpublishedFlow(agentPaused));
eq('upcomingRun: enabled scheduled and flow missions only', [TR.upcomingRun(flowLive), TR.upcomingRun(scheduled), TR.upcomingRun(Object.assign({}, flowLive, { enabled: false })), TR.upcomingRun(Object.assign({}, agent, { next_run: '2026-10-07T09:00:00Z' }))],
    ['2026-10-07T09:00:00Z', '2026-10-08T09:00:00Z', '', '']);

// ── list: Flows filter, badge, quick run, states, next run ──
const list = W.MissionControlList.create({ esc, t, lang: 'en', svg, triggers: TR, schedule: S, readonly: false, fmt });
list.setData({ missions: ALL, queue: { items: [], running: '' } });
const rowsOf = (root) => root.children.flatMap(child => (child.dataset.mcId ? [child] : rowsOf(child)));
const rowHtml = (id) => (rowsOf(list.element).find(r => r.dataset.mcId === id) || {}).innerHTML || '';
check('list: flow row has the flow badge', rowHtml('flow-1').includes('vd-mc-row-badge--flow') && rowHtml('flow-1').includes('desktop.mc_badge_flow'));
check('list: agent row has no flow badge', !rowHtml('agent-1').includes('vd-mc-row-badge--flow'));
check('list: quick run disabled for an unpublished flow', /data-mc-quick="run"[^>]*disabled/.test(rowHtml('flow-2')));
check('list: quick run enabled for a published flow', !/data-mc-quick="run"[^>]*disabled/.test(rowHtml('flow-1')));
check('list: an unpublished flow reads "Not published yet", not Paused', rowHtml('flow-2').includes('data-state="unpublished"') && rowHtml('flow-2').includes('vd-mc-row-badge--text">desktop.mc_flow_unpublished<') && !rowHtml('flow-2').includes('desktop.mc_state_paused'));
check('list: a paused agent mission still reads Paused', rowHtml('agent-2').includes('data-state="paused"') && rowHtml('agent-2').includes('desktop.mc_state_paused'));
check('list: a flow row shows its next run', rowHtml('flow-1').includes('desktop.mc_next_run_in {&quot;when&quot;:&quot;rel&quot;}'));
check('list: a scheduled row still shows its next run', rowHtml('sched-1').includes('desktop.mc_next_run_in {&quot;when&quot;:&quot;rel&quot;}'));
check('list: hostile trigger type and name render escaped', rowHtml('flow-4').includes('&lt;b&gt;x&lt;/b&gt;') && rowHtml('flow-4').includes('F &lt;i&gt;odd&lt;/i&gt; flow') && !rowHtml('flow-4').includes('<b>') && !rowHtml('flow-4').includes('<i>'));
list.setFilter('flow');
eq('list: matchesFilter(flow) keeps only flow missions', list.visibleIds().slice().sort(), ['flow-1', 'flow-2', 'flow-3', 'flow-4']);
list.setFilter('manual');
eq('list: manual filter excludes flows', list.visibleIds().slice().sort(), ['agent-1', 'agent-2']);

// ── menus ──
function menuModel(selected, extra) {
    return Object.assign({ t, readonly: false, s: { selected, running: false, queued: false, queuedIds: new Set(), canCancel: false, filter: 'all', sort: 'name', tab: 'overview', listCollapsed: false }, actions: {} }, extra || {});
}
const item = (menus, id) => menus.flatMap(menu => menu.items).find(i => i.id === id);
const off = (entry) => (typeof entry.disabled === 'function' ? entry.disabled() : !!entry.disabled);
let menus = MN.windowMenus(menuModel(flowDraft));
check('menus: Run disabled for an unpublished flow', off(item(menus, 'run')));
check('menus: Resume disabled for an unpublished flow', off(item(menus, 'pause-resume')) && item(menus, 'pause-resume').label === 'desktop.mc_action_resume');
check('menus: Duplicate disabled for a flow', off(item(menus, 'duplicate')));
check('menus: Prepare disabled for a flow', off(item(menus, 'prepare')));
eq('menus: Edit reads Open in EasyDrag for a flow', item(menus, 'edit').labelKey, 'desktop.mc_action_open_easydrag');
check('menus: New flow and the Flows filter exist', !!item(menus, 'new-flow') && !!item(menus, 'filter-flow'));
menus = MN.windowMenus(menuModel(Object.assign({}, flowLive, { enabled: false })));
check('menus: Resume enabled for a paused published flow', !off(item(menus, 'pause-resume')));
check('menus: Run enabled for a published flow', !off(item(menus, 'run')));
menus = MN.windowMenus(menuModel(flowLive, { readonly: true }));
check('menus: Open in EasyDrag stays enabled read-only', !off(item(menus, 'edit')));
menus = MN.windowMenus(menuModel(agent, { readonly: true }));
check('menus: Edit of an agent mission stays disabled read-only', off(item(menus, 'edit')));
menus = MN.windowMenus(menuModel(agent));
check('menus: agent mission keeps Run, Resume and Duplicate', !off(item(menus, 'run')) && !off(item(menus, 'pause-resume')) && !off(item(menus, 'duplicate')));

const opened = [];
const ctxModel = menuModel(flowDraft, { actions: { openFlow: (id) => opened.push(id), editMission: () => opened.push('editor') } });
const ctxItems = MN.missionContextItems(ctxModel, flowDraft);
const byLabel = (items, label) => items.find(i => i.label === label);
check('context: Run disabled for an unpublished flow', byLabel(ctxItems, 'desktop.mc_action_run').disabled === true);
check('context: Resume disabled for an unpublished flow', byLabel(ctxItems, 'desktop.mc_action_resume').disabled === true);
check('context: Duplicate disabled for a flow', byLabel(ctxItems, 'desktop.mc_action_duplicate').disabled === true);
byLabel(ctxItems, 'desktop.mc_action_open_easydrag').action();
eq('context: Open in EasyDrag calls openFlow', opened, ['flow-2']);
const liveItems = MN.missionContextItems(menuModel(flowLive), Object.assign({}, flowLive, { enabled: false }));
check('context: Resume enabled for a paused published flow', !byLabel(liveItems, 'desktop.mc_action_resume').disabled);
check('context: list menu offers New flow', !!byLabel(MN.listContextItems(menuModel(null)), 'desktop.mc_new_flow'));

// ── detail ──
const detail = W.MissionControlDetail.create({ esc, t, lang: 'en', svg, triggers: TR, schedule: S, readonly: false, fmt });
detail.setMission(flowDraft, { running: false, queuePosition: 0, cancelling: false, busy: '' });
let hero = detail.element.querySelector('[data-mc-hero]').innerHTML;
let overview = detail.element.querySelector('[data-mc-panel="overview"]').innerHTML;
check('detail: hero opens EasyDrag instead of the editor', hero.includes('data-mc-action="openFlow"') && !hero.includes('data-mc-action="edit"'));
check('detail: Run disabled with the publish hint', /data-mc-action="run" disabled\s+title="desktop\.mc_flow_publish_first"/.test(hero), hero.match(/<button[^>]*data-mc-action="run"[^>]*>/));
check('detail: flow card with the publish hint, no prompt, no preparation', overview.includes('vd-mc-card--flow') && overview.includes('vd-mc-flow-hint') && !overview.includes('vd-mc-prompt') && !overview.includes('vd-mc-card--prep'));
check('detail: facts name the flow triggers, no priority', overview.includes('desktop.mc_flow_triggers') && !overview.includes('desktop.mc_exec_priority'));
check('detail: an unpublished flow\'s pill reads "Not published yet", not Paused', hero.includes('data-state="unpublished">desktop.mc_flow_unpublished<') && !hero.includes('desktop.mc_state_paused'));
check('detail: an unpublished flow\'s next run reads "Not published yet", not Paused', /desktop\.mc_overview_next_run<\/dt><dd>desktop\.mc_flow_unpublished</.test(overview) && !overview.includes('desktop.mc_overview_paused'));
detail.setMission(flowLive, { running: false, queuePosition: 0, cancelling: false, busy: '' });
hero = detail.element.querySelector('[data-mc-hero]').innerHTML;
overview = detail.element.querySelector('[data-mc-panel="overview"]').innerHTML;
check('detail: Run enabled for a published flow', /data-mc-action="run"\s+>/.test(hero), hero.match(/<button[^>]*data-mc-action="run"[^>]*>/));
check('detail: no publish hint for a published flow', !overview.includes('vd-mc-flow-hint'));
check('detail: a flow shows its next run', overview.includes('desktop.mc_overview_next_run'));
detail.setMission(Object.assign({}, flowLive, { enabled: false, next_run: '' }), { running: false, queuePosition: 0, cancelling: false, busy: '' });
check('detail: a paused published flow still reads Paused', detail.element.querySelector('[data-mc-hero]').innerHTML.includes('desktop.mc_state_paused') && detail.element.querySelector('[data-mc-panel="overview"]').innerHTML.includes('desktop.mc_overview_paused'));
detail.setMission(agentPaused, { running: false, queuePosition: 0, cancelling: false, busy: '' });
check('detail: a paused agent mission still reads Paused', detail.element.querySelector('[data-mc-hero]').innerHTML.includes('data-state="paused">desktop.mc_state_paused<'));
detail.setMission(flowOdd, { running: false, queuePosition: 0, cancelling: false, busy: '' });
hero = detail.element.querySelector('[data-mc-hero]').innerHTML;
overview = detail.element.querySelector('[data-mc-panel="overview"]').innerHTML;
check('detail: hostile trigger type and name render escaped', hero.includes('&lt;b&gt;x&lt;/b&gt;') && overview.includes('&lt;b&gt;x&lt;/b&gt;') && hero.includes('F &lt;i&gt;odd&lt;/i&gt; flow') && !(hero + overview).includes('<b>') && !(hero + overview).includes('<i>'));

// ── the app shell ──
const calls = { notify: [], confirm: [], openApp: [], editorOpen: [], api: [], menus: null, context: null };
let confirmAnswer = false;
const missionsPayload = () => ({ missions: ALL.map(m => Object.assign({}, m)), queue: { items: [], running: '' } });
// Like the shell's api(): the error carries the HTTP status and the parsed body ({error, code?}).
function httpError(status, message, code) { const err = new Error(message); err.status = status; err.body = code ? { error: message, code } : { error: message }; return err; }
// The cancel answers of handleMissionCancelV2 (internal/server/mission_v2_handlers.go).
const CANCEL_ANSWERS = {
    'flow-1': [409, 'mission run cannot be cancelled yet', 'FLOW_NO_ACTIVE_RUN'],
    'flow-3': [409, 'several flows hold this mission; cancel their runs in EasyDrag', 'FLOW_MISSION_AMBIGUOUS'],
    'flow-4': [409, 'mission run cannot be cancelled yet'],
    'agent-1': [409, 'mission run cannot be cancelled yet']
};
async function api(url, options) {
    const method = (options && options.method) || 'GET';
    calls.api.push({ method, url, body: options && options.body ? JSON.parse(options.body) : undefined });
    if (url === '/api/missions/v2' && method === 'GET') return missionsPayload();
    if (url.endsWith('/run')) return { status: 'queued' };
    if (url.endsWith('/cancel')) throw httpError(...CANCEL_ANSWERS[url.split('/')[4]]);
    if (method === 'PUT' || method === 'DELETE') return { status: 'ok' };
    return {};
}
W.MissionControlEditor = {
    create: () => ({ element: new El('div'), open: (arg) => calls.editorOpen.push(arg.mode + ':' + (arg.mission ? arg.mission.id : '')), close() {}, isDirty: () => false, on() {}, setSaving() {}, setServerError() {}, dispose() {} })
};
const container = new El('div');
W.MissionControlApp.render(container, 'w-mc', {
    esc, t, api, readonly: false,
    notify: (message, type) => calls.notify.push([message, type || 'info']),
    setWindowMenus: (_id, built) => { calls.menus = built; }, clearWindowMenus() {},
    confirmDialog: async (title, message) => { calls.confirm.push([title, message]); return confirmAnswer; },
    showContextMenu: (_x, _y, items) => { calls.context = items; }, setWindowBeforeClose() {}, isActive: () => true,
    openApp: (appId, launch) => calls.openApp.push([appId, launch])
});
await tick(); await tick();
const root = container.querySelector('[data-mc-root]');
const listEl = root.querySelector('[data-mc-listpane]').children[0];
async function selectRow(id) {
    listEl.fire('click', { target: { closest: (sel) => (sel === '[data-mc-id]' ? { dataset: { mcId: id } } : null) }, stopPropagation() {} });
    await tick();
}
const menuAction = async (id) => { item(calls.menus, id).action(); await tick(); await tick(); };
const lastNotify = () => calls.notify[calls.notify.length - 1] || [];
const apiCalls = (method, suffix) => calls.api.filter(c => c.method === method && (!suffix || c.url.endsWith(suffix)));
// The detail's "more" button opens the selected mission's context menu.
const detailEl = root.querySelector('[data-mc-main]').children[0];
function openMoreMenu() {
    const more = { dataset: { mcAction: 'more' }, disabled: false, getBoundingClientRect: () => ({ left: 0, bottom: 0 }) };
    detailEl.fire('click', { target: { closest: (sel) => (sel === '[data-mc-action]' ? more : null) } });
    return calls.context || [];
}

check('shell: the status bar names the earliest next run, a flow\'s included', String(root.querySelector('[data-mc-status-next]').textContent).startsWith('desktop.mc_status_next {"name":"A flow"'), root.querySelector('[data-mc-status-next]').textContent);
await selectRow('flow-1');
await menuAction('edit');
eq('shell: Edit of a flow opens EasyDrag with its flow id', calls.openApp, [['easydrag', { flowId: 'f_live' }]]);
eq('shell: the editor stays closed for a flow', calls.editorOpen, []);
await menuAction('duplicate');
byLabel(openMoreMenu(), 'desktop.mc_action_duplicate').action();
await tick();
eq('shell: Duplicate of a flow (menu, Ctrl+D, context menu) opens neither EasyDrag nor the editor', [calls.openApp.length, calls.editorOpen.length], [1, 0]);
await menuAction('new-flow');
eq('shell: New flow opens EasyDrag without a flow', calls.openApp[1], ['easydrag', {}]);
await selectRow('agent-1');
await menuAction('edit');
eq('shell: Edit of an agent mission opens the editor', calls.editorOpen, ['edit:agent-1']);

// Delete texts (answer "cancel": nothing is deleted).
await selectRow('flow-1');
await menuAction('delete');
check('shell: delete of a flow warns that the flow goes too', (calls.confirm[0] || [])[1] === 'desktop.mc_delete_flow_message {"name":"A flow"}', JSON.stringify(calls.confirm[0]));
await selectRow('agent-1');
await menuAction('delete');
check('shell: delete of an agent mission keeps its text', (calls.confirm[1] || [])[1] === 'desktop.mc_delete_message {"name":"B agent"}', JSON.stringify(calls.confirm[1]));
eq('shell: a cancelled confirm deletes nothing', apiCalls('DELETE').length, 0);

// Run and Resume of an unpublished flow.
await selectRow('flow-2');
check('shell: window menu disables Run and Resume of an unpublished flow', off(item(calls.menus, 'run')) && off(item(calls.menus, 'pause-resume')));
await menuAction('run');
await menuAction('pause-resume');
eq('shell: no run or update request for an unpublished flow', apiCalls('POST', '/run').length + apiCalls('PUT').length, 0);
eq('shell: Run/Resume of an unpublished flow show the publish hint', lastNotify(), ['desktop.mc_flow_publish_first', 'info']);

// Run toast.
await selectRow('flow-1');
await menuAction('run');
eq('shell: a flow run toast names EasyDrag', lastNotify(), ['desktop.mc_toast_flow_run_requested', 'info']);
await selectRow('agent-1');
await menuAction('run');
eq('shell: agent missions keep the queue toast', lastNotify(), ['desktop.mc_toast_run_queued', 'info']);

// Cancel answered 409: only FLOW_NO_ACTIVE_RUN ("nothing to cancel here") becomes the EasyDrag hint.
await selectRow('flow-1');
await menuAction('cancel-run');
eq('shell: FLOW_NO_ACTIVE_RUN on a flow cancel shows the EasyDrag hint', lastNotify(), ['desktop.mc_flow_cancel_in_easydrag', 'info']);
await selectRow('flow-3');
await menuAction('cancel-run');
eq('shell: an ambiguous flow cancel (409) stays an error', lastNotify(), ['desktop.mc_toast_action_failed {"error":"several flows hold this mission; cancel their runs in EasyDrag"}', 'error']);
await selectRow('flow-4');
await menuAction('cancel-run');
eq('shell: a flow cancel 409 without a code stays an error', lastNotify(), ['desktop.mc_toast_action_failed {"error":"mission run cannot be cancelled yet"}', 'error']);
await selectRow('agent-1');
await menuAction('cancel-run');
eq('shell: 409 on an agent cancel stays an error', lastNotify(), ['desktop.mc_toast_action_failed {"error":"mission run cannot be cancelled yet"}', 'error']);

// PUT bodies: the server's updateFlowMissionLocked (internal/tools/missions_v2_flow_runs.go) takes only enabled and
// locked, refuses another execution_type and refuses enabled: true while the flow is unpublished.
function serverAcceptsFlowUpdate(existing, body) {
    if (body.execution_type && body.execution_type !== 'flow') return false;
    return !(body.enabled && !existing.flow_published);
}
await selectRow('flow-1');
await menuAction('pause-resume');
await menuAction('lock-toggle');
await selectRow('flow-2');
await menuAction('lock-toggle');
const puts = apiCalls('PUT');
eq('shell: pause, lock (live) and lock (draft) send PUTs', puts.map(p => p.url), ['/api/missions/v2/flow-1', '/api/missions/v2/flow-1', '/api/missions/v2/flow-2']);
check('shell: pause sends the whole flow mission with enabled false', puts[0] && puts[0].body.execution_type === 'flow' && puts[0].body.enabled === false && puts[0].body.flow_id === 'f_live' && !('next_run' in puts[0].body));
check('shell: lock keeps enabled and sets locked', puts[1] && puts[1].body.locked === true && puts[1].body.enabled === true);
check('shell: lock of an unpublished flow keeps enabled false', puts[2] && puts[2].body.locked === true && puts[2].body.enabled === false);
check('shell: the server accepts every PUT body', puts.every(p => serverAcceptsFlowUpdate(p.url.endsWith('flow-1') ? flowLive : flowDraft, p.body)));
check('shell: the server would refuse Resume of an unpublished flow', !serverAcceptsFlowUpdate(flowDraft, Object.assign({}, flowDraft, { enabled: true })));

W.MissionControlApp.dispose('w-mc');

if (failures) { console.error(`${failures} check(s) failed`); process.exit(1); }
console.log('all mission control flow checks passed');
