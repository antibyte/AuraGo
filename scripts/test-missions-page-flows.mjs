#!/usr/bin/env node
// Runs the standalone missions page (ui/js/missions/main.js) in Node on a small stub DOM and checks how it treats
// EasyDrag flow missions (execution_type "flow"): Edit, Duplicate and the compact row only explain that flows are
// edited in EasyDrag, cards show no preparation state or buttons and a disabled Duplicate, Run waits for a published
// and switched-on flow and says why (visible state, title, toast), icon buttons keep their action names, refused
// runs and deletes show the server's message (or the page's own text), the delete confirmation says that the flow
// goes too, the Flow filter, the status chip, the mission selector, and that mission fields cannot break out of
// the attributes they are written into.
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const uiDir = path.join(here, '..', 'ui');

// ── stub DOM: getElementById hands out one element per id; innerHTML is kept as a string ──
class ClassList {
    constructor() { this.set = new Set(); }
    add(...names) { names.forEach(n => this.set.add(n)); }
    remove(...names) { names.forEach(n => this.set.delete(n)); }
    toggle(name, on) { const next = on === undefined ? !this.set.has(name) : !!on; if (next) this.set.add(name); else this.set.delete(name); return next; }
    contains(name) { return this.set.has(name); }
}
const escapeText = (s) => String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
class El {
    constructor(id) {
        this.id = id || ''; this.classList = new ClassList(); this.dataset = {}; this.listeners = {}; this.attrs = {};
        this.innerHTML = ''; this.value = ''; this.checked = false; this.disabled = false; this.options = []; this.title = '';
        this._text = '';
    }
    // escapeHtml() in main.js relies on textContent -> innerHTML escaping.
    set textContent(value) { this._text = String(value); this.innerHTML = escapeText(value); }
    get textContent() { return this._text; }
    addEventListener(type, fn) { (this.listeners[type] = this.listeners[type] || []).push(fn); }
    setAttribute(name, value) { this.attrs[name] = String(value); }
    querySelector() { return null; }
    querySelectorAll() { return []; }
    closest() { return null; }
    reset() {}
}
const elements = new Map();
const queried = new Map();
const docListeners = {};
const document = {
    documentElement: { lang: 'en' },
    getElementById(id) { if (!elements.has(id)) elements.set(id, new El(id)); return elements.get(id); },
    querySelector(sel) { if (!queried.has(sel)) queried.set(sel, new El()); return queried.get(sel); },
    querySelectorAll() { return []; },
    createElement() { return new El(); },
    addEventListener(type, fn) { (docListeners[type] = docListeners[type] || []).push(fn); }
};

// ── recorded side effects ──
const toasts = [];
const modals = [];
const confirms = [];
const requests = [];
let confirmAnswer = false;
const confirmQueue = []; // answers for the next confirmations, before confirmAnswer
const responses = { '/api/missions/v2': { missions: [], queue: { items: [], running: '' } } };
// failing maps "METHOD url" to a refused answer { status, body }; body is sent as JSON text unless it is a string.
const failing = new Map();
function fetchStub(url, opts = {}) {
    const method = opts.method || 'GET';
    requests.push({ url, method });
    const refused = failing.get(`${method} ${url}`);
    if (refused) {
        const text = typeof refused.body === 'string' ? refused.body : JSON.stringify(refused.body);
        return Promise.resolve({ ok: false, status: refused.status, json: () => Promise.resolve(refused.body), text: () => Promise.resolve(text) });
    }
    const body = Object.prototype.hasOwnProperty.call(responses, url) ? responses[url] : (/\/run$/.test(url) ? { status: 'queued' } : []);
    return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve(body), text: () => Promise.resolve('') });
}

// The translator returns the key, plus the params as JSON.
const t = (key, params) => key + (params ? ' ' + JSON.stringify(params) : '');
const sandbox = {
    window: { AuraSSE: { on() {} } }, document, console, setTimeout, clearTimeout, Promise, JSON,
    localStorage: { getItem: () => null, setItem() {} },
    fetch: fetchStub, t,
    showToast: (message, type) => toasts.push({ message, type }),
    showConfirm: (title, message) => { confirms.push({ title, message }); return Promise.resolve(confirmQueue.length ? confirmQueue.shift() : confirmAnswer); },
    openModal: (id) => modals.push(id),
    closeModal() {}
};
sandbox.globalThis = sandbox;
vm.createContext(sandbox);
const mainFile = path.join(uiDir, 'js', 'missions', 'main.js');
vm.runInContext(fs.readFileSync(mainFile, 'utf8'), sandbox, { filename: mainFile });
const P = sandbox; // page functions are globals of the context
// main.js keeps its state in top-level `let`s; a later script in the same context sees them. The page reloads
// its list after every action, so the list endpoint answers with the same missions.
function setMissions(list) {
    vm.runInContext(`missions = ${JSON.stringify(list)}; queue = { items: [], running: '' };`, sandbox);
    responses['/api/missions/v2'] = { missions: JSON.parse(JSON.stringify(list)), queue: { items: [], running: '' } };
}

let failures = 0;
function check(name, cond, detail) {
    if (cond) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + (detail ? ' — ' + detail : ''));
}
const tick = () => new Promise(resolve => setTimeout(resolve, 0));
function reset() { toasts.length = 0; modals.length = 0; confirms.length = 0; requests.length = 0; confirmQueue.length = 0; failing.clear(); }
const refreshed = () => requests.some(r => r.url === '/api/missions/v2' && r.method === 'GET');
// buttonFor returns the opening tag of the button with data-mission-action="action" in html, or ''.
function buttonFor(html, action) {
    const m = html.match(new RegExp(`<button[^>]*data-mission-action="${action}"[^>]*>`));
    return m ? m[0] : '';
}
// startTags parses every start tag of html into { tag, attrs: [{ name, value }] }. Attribute values must be quoted
// or plain; a value that breaks out of its quotes shows up as an extra attribute.
function startTags(html) {
    const tags = [];
    for (const m of html.matchAll(/<([a-zA-Z][a-zA-Z0-9-]*)((?:\s+[^\s"'>\/=]+(?:\s*=\s*(?:"[^"]*"|'[^']*'|[^\s"'=<>`]+))?)*)\s*\/?>/g)) {
        const attrs = [];
        for (const a of m[2].matchAll(/\s+([^\s"'>\/=]+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'=<>`]+)))?/g)) {
            attrs.push({ name: a[1].toLowerCase(), value: a[2] ?? a[3] ?? a[4] ?? '' });
        }
        tags.push({ tag: m[1].toLowerCase(), attrs });
    }
    return tags;
}
function eventHandlerAttrs(html) {
    return startTags(html).flatMap(tag => tag.attrs.filter(a => a.name.startsWith('on')).map(a => `${tag.tag}[${a.name}]`));
}

const agent = { id: 'agent-1', name: 'Agent', execution_type: 'manual', prompt: 'Do it', priority: 'medium', run_count: 2, preparation_status: 'none', enabled: true };
const preparedAgent = { ...agent, id: 'agent-2', name: 'Prepared agent', preparation_status: 'prepared' };
const scheduled = { id: 'sched-1', name: 'Daily', execution_type: 'scheduled', schedule: '0 9 * * *', prompt: 'Daily', priority: 'low', run_count: 0, enabled: true };
const triggered = { id: 'trig-1', name: 'Hook', execution_type: 'triggered', trigger_type: 'webhook', trigger_config: {}, prompt: 'Hook', priority: 'high', run_count: 0, enabled: true };
const flowLive = { id: 'flow-1', name: 'Live <b>flow</b>', execution_type: 'flow', flow_id: 'f1', flow_published: true, enabled: true, prompt: '', priority: 'medium', run_count: 4 };
// A flow that was never published (always switched off); a stale preparation status must not show anything.
const flowDraft = { id: 'flow-2', name: 'Draft flow', execution_type: 'flow', flow_id: 'f2', enabled: false, prompt: '', priority: 'medium', run_count: 0, preparation_status: 'prepared' };
// Published, but switched off: the server refuses its runs ("mission is disabled").
const flowPaused = { id: 'flow-3', name: 'Paused flow', execution_type: 'flow', flow_id: 'f3', flow_published: true, enabled: false, prompt: '', priority: 'medium', run_count: 1 };
const flowLocked = { id: 'flow-4', name: 'Locked flow', execution_type: 'flow', flow_id: 'f4', flow_published: true, enabled: true, locked: true, prompt: '', priority: 'medium', run_count: 1 };
const flowRunning = { id: 'flow-5', name: 'Busy flow', execution_type: 'flow', flow_id: 'f5', flow_published: true, enabled: true, status: 'running', prompt: '', priority: 'high', run_count: 9 };
// Hostile text in every field that reaches an attribute or the markup.
const EVIL = `Evil" onmouseover="alert(1)" x='y' <img src=x onerror=alert(2)> & co`;
const flowEvil = {
    id: `evil" onmouseover="alert(3)`, name: EVIL, execution_type: 'flow', flow_id: 'f6', flow_published: true, enabled: true,
    prompt: EVIL, priority: `high" onmouseover="alert(4)`, run_count: 1, preparation_status: `x" onmouseover="alert(5)`
};
const agentEvil = {
    id: `agent" onfocus="alert(6)`, name: EVIL, execution_type: 'scheduled', schedule: `<img src=x onerror=alert(7)>`, prompt: EVIL,
    priority: `low" onclick="alert(8)`, run_count: 0, enabled: true, preparation_status: `none" onclick="alert(9)`
};
const triggeredEvil = {
    id: 'trig-evil', name: 'Evil trigger', execution_type: 'triggered', trigger_type: 'email_received', priority: 'low', run_count: 0, enabled: true, prompt: '',
    trigger_config: { email_folder: '<img src=x onerror=alert(10)>', email_subject_contains: `" onmouseover="alert(11)`, min_interval_seconds: '<b>9</b>' }
};
const ALL = [agent, preparedAgent, scheduled, triggered, flowLive, flowDraft, flowPaused, flowLocked, flowRunning];
setMissions(ALL);

// ── guard: Edit, Duplicate and the compact row's open-edit ──
reset();
P.editMission('flow-1');
check('edit: a flow only explains where it is edited', toasts.length === 1 && toasts[0].message === 'missions.flow_managed' && toasts[0].type === 'info', JSON.stringify(toasts));
check('edit: a flow opens no mission editor', modals.length === 0, JSON.stringify(modals));
reset();
P.editMission('agent-1');
check('edit: an agent mission still opens the editor', modals.includes('modal') && toasts.length === 0, JSON.stringify({ modals, toasts }));
reset();
P.duplicateMission('flow-2');
check('duplicate: a flow is not copied into the agent form', modals.length === 0 && toasts.length === 1 && toasts[0].message === 'missions.flow_managed', JSON.stringify({ modals, toasts }));
reset();
P.duplicateMission('agent-1');
check('duplicate: an agent mission still opens the form', modals.includes('modal'), JSON.stringify(modals));

// The page's click delegation: a click on a compact row (data-mission-action="open-edit") reaches the guard.
P.bindMissionUI();
function clickAction(action, id, { compact = false } = {}) {
    const actionEl = new El();
    actionEl.dataset = { missionAction: action, missionId: id };
    if (compact) actionEl.classList.add('card-compact');
    const target = { closest: (sel) => (sel === '[data-mission-action]' ? actionEl : null) };
    (docListeners.click || []).forEach(fn => fn({ target }));
}
reset();
clickAction('open-edit', 'flow-1', { compact: true });
check('compact row: open-edit of a flow reaches the guard', toasts.length === 1 && toasts[0].message === 'missions.flow_managed' && modals.length === 0, JSON.stringify({ toasts, modals }));
reset();
clickAction('open-edit', 'agent-1', { compact: true });
check('compact row: open-edit of an agent mission opens the editor', modals.includes('modal'), JSON.stringify(modals));
reset();
clickAction('duplicate', 'flow-1');
check('click: duplicate of a flow reaches the guard', toasts.length === 1 && modals.length === 0, JSON.stringify({ toasts, modals }));

// ── cards: no preparation state or buttons, Duplicate disabled, Edit explains, icon buttons keep their names ──
const views = [['grid', (m) => P.renderMissionGrid(m, false)], ['list', (m) => P.renderMissionCompact(m)]];
for (const [view, render] of views) {
    for (const flow of [flowLive, flowDraft, flowPaused]) {
        const html = render(flow);
        for (const action of ['prepare', 'view-prepared', 'invalidate-prepared']) {
            check(`${view}: ${flow.id} has no ${action} button`, !html.includes(`data-mission-action="${action}"`));
        }
        check(`${view}: ${flow.id} shows no preparation badge`, !html.includes('badge-prep-'));
        const dup = buttonFor(html, 'duplicate');
        check(`${view}: ${flow.id} Duplicate is disabled with the flow hint`, / disabled/.test(dup) && dup.includes('title="missions.flow_managed"'), dup);
        check(`${view}: ${flow.id} Duplicate keeps its action name`, dup.includes('aria-label="missions.card_btn_duplicate_title"'), dup);
        const edit = buttonFor(html, 'open-edit');
        check(`${view}: ${flow.id} Edit stays clickable and explains`, !/ disabled/.test(edit) && edit.includes('title="missions.flow_managed"'), edit);
        check(`${view}: ${flow.id} Edit keeps its action name`, edit.includes('aria-label="missions.card_btn_edit_title"'), edit);
    }
    check(`${view}: names are escaped`, !render(flowLive).includes('<b>flow</b>'));
    const agentHtml = render(agent);
    check(`${view}: agent missions keep Prepare`, agentHtml.includes('data-mission-action="prepare"'));
    check(`${view}: agent missions keep an enabled Duplicate`, !/ disabled/.test(buttonFor(agentHtml, 'duplicate')) && buttonFor(agentHtml, 'duplicate').includes('title="missions.card_btn_duplicate_title"'));
    check(`${view}: prepared agent missions keep view/invalidate and their badge`, ['view-prepared', 'invalidate-prepared'].every(a => render(preparedAgent).includes(`data-mission-action="${a}"`)) && render(preparedAgent).includes('badge-prep-prepared'));
    const lockedDelete = buttonFor(render(flowLocked), 'delete');
    check(`${view}: a locked flow cannot be deleted`, / disabled/.test(lockedDelete), lockedDelete);
    check(`${view}: an unlocked flow can be deleted`, !/ disabled/.test(buttonFor(render(flowLive), 'delete')));
}

// ── Run: disabled with the reason while unpublished, switched off or running ──
const runCases = [
    ['a published flow can run', flowLive, false, 'missions.card_btn_run_title'],
    ['an unpublished flow cannot run and says why', flowDraft, true, 'missions.flow_publish_first'],
    ['a switched-off flow cannot run and says why', flowPaused, true, 'missions.flow_switch_on_first'],
    ['a running flow cannot run again', flowRunning, true, 'missions.card_btn_run_title'],
];
for (const [name, mission, disabled, title] of runCases) {
    const grid = buttonFor(P.renderMissionGrid(mission, false), 'run');
    check(`grid: ${name}`, / disabled/.test(grid) === disabled && grid.includes(`title="${title}"`), grid);
    check(`grid: Run of ${mission.id} keeps its visible label as name`, !grid.includes('aria-label='), grid);
    const list = buttonFor(P.renderMissionCompact(mission), 'run');
    check(`list: ${name}`, / disabled/.test(list) === disabled && list.includes(`title="${title}"`), list);
    check(`list: Run of ${mission.id} keeps its action name`, list.includes('aria-label="missions.card_btn_run_title"'), list);
}

// ── visible state: the chip (grid) and a badge (list) say why a flow does not run ──
const stateCases = [
    [flowDraft, 'missions.flow_unpublished'],
    [flowPaused, 'missions.flow_paused'],
];
for (const [mission, label] of stateCases) {
    const chip = P.renderStatusChip(mission, false, false, false);
    check(`status chip: ${mission.id} reads ${label}`, chip.includes(`mc-status-chip__label">${label}</span>`) && chip.includes('mc-status-chip--flow-off'), chip);
    check(`status chip: ${mission.id} keeps the whole label in its title`, chip.includes(`mc-status-chip--flow-off" title="${label}"`), chip);
    const grid = P.renderMissionGrid(mission, false);
    check(`grid: ${mission.id} shows ${label} and keeps the Flow pill`, grid.includes(`>${label}</span>`) && grid.includes('🧩<span>missions.filter_flow</span>'));
    const list = P.renderMissionCompact(mission);
    check(`list: ${mission.id} shows ${label} as a badge`, list.includes(`<span class="badge badge-idle">${label}</span>`), list.match(/card-badges">[^]*?<\/div>/)?.[0]);
}
const liveChip = P.renderStatusChip(flowLive, false, false, false);
check('status chip: a runnable flow is labelled Flow, not Manual', liveChip.includes('mc-status-chip--flow"') && liveChip.includes('missions.filter_flow') && !liveChip.includes('missions.filter_manual'), liveChip);
check('status chip: one-word chips carry no title', !liveChip.includes(' title=') && !P.renderStatusChip(agent, false, false, false).includes(' title='));
check('list: a runnable flow has no state badge', !P.renderMissionCompact(flowLive).includes('badge-idle'));
const runningChip = P.renderStatusChip(flowRunning, true, false, false);
check('status chip: a running flow shows the running state', runningChip.includes('missions.card_badge_running') && runningChip.includes('mc-status-chip--running'));
check('list: a running flow shows the running badge', P.renderMissionCompact(flowRunning).includes('badge-running'));
check('status chip: manual missions keep their label', P.renderStatusChip(agent, false, false, false).includes('missions.filter_manual'));
const flowGrid = P.renderMissionGrid(flowLive, false);
check('grid: the execution pill names the flow type', flowGrid.includes('🧩<span>missions.filter_flow</span>'), flowGrid.match(/mc-trigger-pill[^]*?<\/div>/)?.[0]);
check('grid: data-status is flow', flowGrid.includes('data-status="flow"'));
check('list: the type icon is the flow icon', P.renderMissionCompact(flowLive).includes('🧩'));

// ── Run: requests, toasts and refused runs ──
reset();
await P.runMission('flow-2');
check('run: an unpublished flow sends no request and says why', requests.length === 0 && toasts.length === 1 && toasts[0].message === 'missions.flow_publish_first' && toasts[0].type === 'info', JSON.stringify({ requests, toasts }));
reset();
await P.runMission('flow-3');
check('run: a switched-off flow sends no request and says why', requests.length === 0 && toasts.length === 1 && toasts[0].message === 'missions.flow_switch_on_first' && toasts[0].type === 'info', JSON.stringify({ requests, toasts }));
reset();
await P.runMission('flow-1');
check('run: a published flow is started through the mission API', requests.some(r => r.url === '/api/missions/v2/flow-1/run' && r.method === 'POST'), JSON.stringify(requests));
check('run: a flow run says it runs in EasyDrag, not that it was queued', toasts.length === 1 && toasts[0].message === 'missions.toast_flow_run_requested', JSON.stringify(toasts));
check('run: the list is refreshed after a run', refreshed(), JSON.stringify(requests));
reset();
await P.runMission('agent-1');
check('run: agent missions keep the dispatch toast', toasts.length === 1 && toasts[0].message === 'missions.toast_queued', JSON.stringify(toasts));
const refusedRuns = [
    ['400 "mission is disabled"', 400, { error: 'mission is disabled' }, 'missions.flow_switch_on_first', 'info'],
    ['409 not published', 409, { error: 'the flow has not been published yet' }, 'missions.flow_publish_first', 'info'],
    ['503 flows unavailable', 503, { error: 'flows are not available' }, 'missions.flows_unavailable', 'info'],
    ['429 queue full', 429, { error: "the flow's run queue is full" }, "missions.toast_error_prefixthe flow's run queue is full", 'error'],
    ['500 plain text', 500, 'boom', 'missions.toast_error_prefixboom', 'error'],
];
for (const [name, status, body, message, type] of refusedRuns) {
    reset();
    failing.set('POST /api/missions/v2/flow-1/run', { status, body });
    await P.runMission('flow-1');
    check(`run refused (${name}): ${message}`, toasts.length === 1 && toasts[0].message === message && toasts[0].type === type, JSON.stringify(toasts));
    check(`run refused (${name}): no refresh`, !refreshed(), JSON.stringify(requests));
}
reset();
failing.set('POST /api/missions/v2/agent-1/run', { status: 400, body: { error: 'mission is disabled' } });
await P.runMission('agent-1');
check('run refused (agent): the server message, not raw JSON', toasts.length === 1 && toasts[0].message === 'missions.toast_error_prefixmission is disabled' && toasts[0].type === 'error', JSON.stringify(toasts));

// ── delete ──
reset();
confirmAnswer = false;
await P.deleteMission('flow-1');
check('delete: a flow names the flow that goes too', confirms.length === 1 && confirms[0].message === 'missions.confirm_delete_flow {"name":"Live <b>flow</b>"}', JSON.stringify(confirms));
check('delete: a declined confirmation sends nothing', requests.length === 0, JSON.stringify(requests));
reset();
await P.deleteMission('agent-1');
check('delete: agent missions keep their text', confirms.length === 1 && confirms[0].message === 'missions.confirm_delete {"name":"Agent"}', JSON.stringify(confirms));
reset();
confirmAnswer = true;
await P.deleteMission('flow-2');
await tick();
check('delete: a confirmed flow delete goes to the mission API', requests.some(r => r.url === '/api/missions/v2/flow-2' && r.method === 'DELETE'), JSON.stringify(requests));
check('delete: a deleted flow is announced', toasts.length === 1 && toasts[0].message === 'missions.toast_mission_deleted' && toasts[0].type === 'success', JSON.stringify(toasts));
check('delete: the list is refreshed after a delete', refreshed(), JSON.stringify(requests));
setMissions(ALL);
reset();
confirmAnswer = true;
failing.set('DELETE /api/missions/v2/flow-4', { status: 500, body: { error: 'mission is locked' } });
await P.deleteMission('flow-4');
await tick();
check('delete refused: the server message, not raw JSON', toasts.length === 1 && toasts[0].message === 'missions.toast_error_prefixmission is locked' && toasts[0].type === 'error', JSON.stringify(toasts));
check('delete refused: no refresh', !refreshed(), JSON.stringify(requests));
// A remote mission whose egg is offline: the page offers to remove it locally only (?force=true).
const remoteAgent = { id: 'remote-1', name: 'Remote agent', execution_type: 'manual', runner_type: 'remote', remote_nest_id: 'n1', prompt: 'Far away', priority: 'low', run_count: 0, enabled: true };
setMissions([...ALL, remoteAgent]);
reset();
confirmAnswer = true;
failing.set('DELETE /api/missions/v2/remote-1', { status: 409, body: { error: 'remote nest n1 is not connected' } });
await P.deleteMission('remote-1');
await tick();
check('delete remote: a second confirmation offers the local removal', confirms.length === 2 && confirms[1].message === 'missions.confirm_force_delete_remote {"name":"Remote agent"}', JSON.stringify(confirms));
check('delete remote: the forced delete is sent', requests.some(r => r.url === '/api/missions/v2/remote-1?force=true' && r.method === 'DELETE'), JSON.stringify(requests));
check('delete remote: the forced delete is announced and refreshes', toasts.length === 1 && toasts[0].message === 'missions.toast_mission_deleted' && refreshed(), JSON.stringify({ toasts, requests }));
reset();
confirmQueue.push(true, false);
failing.set('DELETE /api/missions/v2/remote-1', { status: 409, body: { error: 'remote nest n1 is not connected' } });
await P.deleteMission('remote-1');
check('delete remote: declining the local removal sends no forced delete', confirms.length === 2 && !requests.some(r => r.url.includes('force=true')) && !refreshed(), JSON.stringify({ confirms, requests }));
confirmAnswer = false;
setMissions(ALL);

// ── Flow filter ──
P.filterMissions('flow');
const grid = document.getElementById('missions-grid').innerHTML;
check('filter: Flow shows flow missions', ['flow-1', 'flow-2', 'flow-3'].every(id => grid.includes(`data-mission-id="${id}"`)));
check('filter: Flow hides other missions', !grid.includes('data-mission-id="agent-1"') && !grid.includes('data-mission-id="sched-1"') && !grid.includes('data-mission-id="trig-1"'));
P.filterMissions('all');
const html = fs.readFileSync(path.join(uiDir, 'missions_v2.html'), 'utf8');
const flowBtn = html.match(/<button[^>]*data-filter="flow"[^>]*>\s*<span data-i18n="([^"]+)"><\/span>/);
check('filter: missions_v2.html has a Flow filter button', !!flowBtn && flowBtn[1] === 'missions.filter_flow' && flowBtn[0].includes('class="missions-filter-btn pw-badge"') && flowBtn[0].includes('aria-pressed="false"'));
check('filter: the Flow button follows Triggered', html.indexOf('data-filter="triggered"') < html.indexOf('data-filter="flow"'));

// ── mission_completed source selector ──
P.loadMissionSelector();
const selector = document.getElementById('mission-selector').innerHTML;
check('selector: flow missions are sources', selector.includes('value="flow-1"') && selector.includes('value="flow-2"'));
check('selector: a flow option reads Flow', /value="flow-1"[^]*?mission-option-meta">missions\.filter_flow •/.test(selector), selector);
check('selector: manual and scheduled options read their type', /value="agent-1"[^]*?mission-option-meta">missions\.filter_manual •/.test(selector) && /value="sched-1"[^]*?mission-option-meta">missions\.filter_scheduled •/.test(selector));
check('selector: triggered missions stay out', !selector.includes('value="trig-1"'));

// ── hostile names, ids and fields stay inside their attributes ──
setMissions([flowEvil, agentEvil, triggeredEvil]);
vm.runInContext(`queue = { items: [{ mission_id: ${JSON.stringify(agentEvil.id)}, priority: 1, trigger_type: '<img src=x onerror=alert(12)>', enqueued_at: '' }], running: ${JSON.stringify(flowEvil.id)} };`, sandbox);
P.loadMissionSelector();
P.renderQueue();
const rendered = {
    selector: document.getElementById('mission-selector').innerHTML,
    queue: document.getElementById('queue-items').innerHTML,
};
for (const m of [flowEvil, agentEvil, triggeredEvil]) {
    rendered[`grid ${m.id}`] = P.renderMissionGrid(m, false);
    rendered[`list ${m.id}`] = P.renderMissionCompact(m);
    rendered[`chip ${m.id}`] = P.renderStatusChip(m, false, false, false);
}
// Every trigger type renderTriggerText describes, with hostile text in each field it shows.
const H = `x" onmouseover="alert(20)" <img src=x onerror=alert(21)> &`;
const H_ESCAPED = '&lt;img src=x onerror=alert(21)&gt; &amp;';
const hostileTriggers = {
    mission_completed: { source_mission_name: H, require_success: true },
    email_received: { email_folder: H, email_subject_contains: H, email_from_contains: H },
    webhook: { webhook_slug: H },
    egg_hatched: { egg_name: H, nest_name: H },
    nest_cleared: { nest_name: H },
    mqtt_message: { mqtt_topic: H, mqtt_payload_contains: H, mqtt_min_interval_seconds: H },
    system_startup: { min_interval_seconds: H },
    home_assistant_state: { ha_entity_id: H, ha_state_equals: H },
    device_connected: { device_name: H },
    device_disconnected: { device_id: H },
    fritzbox_call: { call_type: H },
    budget_warning: { min_interval_seconds: H },
    budget_exceeded: { min_interval_seconds: H },
    planner_appointment_due: { planner_appointment_id: H, planner_title_contains: H },
    planner_todo_overdue: { planner_todo_id: H, planner_title_contains: H },
    planner_operational_issue: { planner_issue_source: H, planner_issue_severity: H, planner_title_contains: H },
};
// The id-only fallbacks of webhook and egg/nest triggers.
const hostileFallbacks = { webhook: { webhook_id: H }, egg_hatched: { egg_id: H, nest_id: H }, nest_cleared: { nest_id: H } };
for (const [type, cfg] of [...Object.entries(hostileTriggers), ...Object.entries(hostileFallbacks).map(([k, v]) => [k, v])]) {
    const m = { id: `trig-${type}`, name: type, execution_type: 'triggered', trigger_type: type, trigger_config: cfg, priority: 'low', run_count: 0, enabled: true, prompt: '' };
    const label = `${type} ${Object.keys(cfg).join('+')}`;
    rendered[`grid ${label}`] = P.renderMissionGrid(m, false);
    rendered[`info ${label}`] = P.renderTriggerInfo(m);
    check(`escaping: ${label} shows its hostile value escaped`, P.renderTriggerText(m).includes(H_ESCAPED), P.renderTriggerText(m));
}
// Options filled from other APIs: invasion eggs/nests, cheat sheets, webhooks and remote targets.
responses['/api/invasion/eggs'] = { eggs: [{ id: H, name: H }] };
responses['/api/invasion/nests'] = { nests: [{ id: H, name: H }] };
responses['/api/cheatsheets?active=true&created_by=user'] = [{ id: H, name: H, abstract: H }];
responses['/api/webhooks'] = [{ id: H, slug: H, name: H }];
responses['/api/missions/v2/remote-targets'] = { targets: [{ nest_id: H, egg_id: H, nest_name: H, egg_name: H }] };
await P.loadInvasionData();
await P.loadCheatsheetPicker([H]);
await P.loadWebhooks();
await P.loadRemoteTargets(null);
for (const id of ['egg-hatched-egg-select', 'egg-hatched-nest-select', 'nest-cleared-nest-select', 'cheatsheet-picker', 'webhook-select', 'remote-target-select']) {
    const markup = document.getElementById(id).innerHTML;
    rendered[`options ${id}`] = markup;
    check(`escaping: ${id} received the hostile option`, markup.includes(H_ESCAPED), markup);
}
for (const [where, markup] of Object.entries(rendered)) {
    // A value that breaks out of its quotes leaves a tag the parser cannot read; count those too.
    const unparsed = (markup.match(/<[a-zA-Z]/g) || []).length - startTags(markup).length;
    check(`escaping: ${where} has only well-formed tags`, unparsed === 0, `${unparsed} tag(s) did not parse`);
    const handlers = eventHandlerAttrs(markup);
    check(`escaping: ${where} has no injected event handler`, handlers.length === 0, handlers.join(', '));
    check(`escaping: ${where} has no injected tag`, !/<img/i.test(markup), (markup.match(/<img[^>]*>/i) || [])[0]);
}
check('escaping: the selector keeps the id and name as escaped attribute values',
    rendered.selector.includes('value="evil&quot; onmouseover=&quot;alert(3)"') && rendered.selector.includes('data-name="Evil&quot; onmouseover=&quot;alert(1)&quot; x=&#39;y&#39; &lt;img src=x onerror=alert(2)&gt; &amp; co"'),
    rendered.selector);
const nameTitle = startTags(rendered[`grid ${flowEvil.id}`]).find(tag => tag.tag === 'h3')?.attrs.find(a => a.name === 'title')?.value;
check('escaping: the card title attribute holds the whole name', nameTitle === EVIL.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/'/g, '&#39;').replace(/</g, '&lt;').replace(/>/g, '&gt;'), nameTitle);
const pillTitle = startTags(rendered[`grid ${triggeredEvil.id}`]).find(tag => tag.attrs.some(a => a.name === 'class' && a.value === 'mc-trigger-pill'))?.attrs.find(a => a.name === 'title')?.value || '';
check('escaping: the trigger pill title is escaped once', pillTitle.includes('&lt;img src=x onerror=alert(10)&gt;') && !pillTitle.includes('&amp;lt;'), pillTitle);
setMissions(ALL);

// ── strings: every key the flow paths use exists in all 16 locales ──
const langs = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const keys = ['missions.filter_flow', 'missions.flow_managed', 'missions.flow_publish_first', 'missions.confirm_delete_flow', 'missions.toast_flow_run_requested',
    'missions.flow_unpublished', 'missions.flow_paused', 'missions.flow_switch_on_first', 'missions.flows_unavailable'];
const source = fs.readFileSync(mainFile, 'utf8');
for (const key of keys) check(`strings: main.js uses ${key} literally`, source.includes(`'${key}'`));
for (const lang of langs) {
    const bundle = JSON.parse(fs.readFileSync(path.join(uiDir, 'lang', 'missions', `${lang}.json`), 'utf8'));
    const missing = keys.filter(k => typeof bundle[k] !== 'string' || !bundle[k].trim());
    check(`strings: ${lang} has the flow keys`, missing.length === 0, missing.join(', '));
    check(`strings: ${lang} delete text names the mission`, String(bundle['missions.confirm_delete_flow']).includes('{{name}}'));
}

if (failures) {
    console.log(`\n${failures} check(s) failed`);
    process.exit(1);
}
console.log('\nall missions page flow checks passed');
