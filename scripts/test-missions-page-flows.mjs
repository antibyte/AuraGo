#!/usr/bin/env node
// Runs the standalone missions page (ui/js/missions/main.js) in Node on a small stub DOM and checks how it treats
// EasyDrag flow missions (execution_type "flow"): Edit, Duplicate and the compact row only explain that flows are
// edited in EasyDrag, cards show no preparation buttons and a disabled Duplicate, Run waits for a published flow,
// the delete confirmation says that the flow goes too, the Flow filter, the status chip and the mission selector.
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
const responses = { '/api/missions/v2': { missions: [], queue: { items: [], running: '' } } };
function fetchStub(url, opts = {}) {
    requests.push({ url, method: opts.method || 'GET' });
    const body = Object.prototype.hasOwnProperty.call(responses, url) ? responses[url] : (/\/run$/.test(url) ? { status: 'queued' } : []);
    return Promise.resolve({ ok: true, json: () => Promise.resolve(body), text: () => Promise.resolve('') });
}

// The translator returns the key, plus the params as JSON.
const t = (key, params) => key + (params ? ' ' + JSON.stringify(params) : '');
const sandbox = {
    window: { AuraSSE: { on() {} } }, document, console, setTimeout, clearTimeout, Promise, JSON,
    localStorage: { getItem: () => null, setItem() {} },
    fetch: fetchStub, t,
    showToast: (message, type) => toasts.push({ message, type }),
    showConfirm: (title, message) => { confirms.push({ title, message }); return Promise.resolve(confirmAnswer); },
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
function reset() { toasts.length = 0; modals.length = 0; confirms.length = 0; requests.length = 0; }
// buttonFor returns the opening tag of the button with data-mission-action="action" in html, or ''.
function buttonFor(html, action) {
    const m = html.match(new RegExp(`<button[^>]*data-mission-action="${action}"[^>]*>`));
    return m ? m[0] : '';
}

const agent = { id: 'agent-1', name: 'Agent', execution_type: 'manual', prompt: 'Do it', priority: 'medium', run_count: 2, preparation_status: 'none' };
const preparedAgent = { ...agent, id: 'agent-2', name: 'Prepared agent', preparation_status: 'prepared' };
const scheduled = { id: 'sched-1', name: 'Daily', execution_type: 'scheduled', schedule: '0 9 * * *', prompt: 'Daily', priority: 'low', run_count: 0 };
const triggered = { id: 'trig-1', name: 'Hook', execution_type: 'triggered', trigger_type: 'webhook', trigger_config: {}, prompt: 'Hook', priority: 'high', run_count: 0 };
const flowLive = { id: 'flow-1', name: 'Live <b>flow</b>', execution_type: 'flow', flow_id: 'f1', flow_published: true, enabled: true, prompt: '', priority: 'medium', run_count: 4 };
// A flow that was never published; a stale preparation status must not bring back the preparation buttons.
const flowDraft = { id: 'flow-2', name: 'Draft flow', execution_type: 'flow', flow_id: 'f2', enabled: false, prompt: '', priority: 'medium', run_count: 0, preparation_status: 'prepared' };
const ALL = [agent, preparedAgent, scheduled, triggered, flowLive, flowDraft];
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

// ── cards: no preparation buttons, Duplicate disabled, Edit explains, Run waits for publishing ──
for (const [view, render] of [['grid', (m) => P.renderMissionGrid(m, false)], ['list', (m) => P.renderMissionCompact(m)]]) {
    for (const flow of [flowLive, flowDraft]) {
        const html = render(flow);
        for (const action of ['prepare', 'view-prepared', 'invalidate-prepared']) {
            check(`${view}: ${flow.id} has no ${action} button`, !html.includes(`data-mission-action="${action}"`));
        }
        const dup = buttonFor(html, 'duplicate');
        check(`${view}: ${flow.id} Duplicate is disabled with the flow hint`, / disabled/.test(dup) && dup.includes('title="missions.flow_managed"'), dup);
        const edit = buttonFor(html, 'open-edit');
        check(`${view}: ${flow.id} Edit stays clickable and explains`, !/ disabled/.test(edit) && edit.includes('title="missions.flow_managed"'), edit);
        check(`${view}: ${flow.id} name is escaped`, !html.includes('<b>flow</b>'));
    }
    const live = buttonFor(render(flowLive), 'run');
    check(`${view}: a published flow can run`, !/ disabled/.test(live) && live.includes('title="missions.card_btn_run_title"'), live);
    const draft = buttonFor(render(flowDraft), 'run');
    check(`${view}: an unpublished flow cannot run and says why`, / disabled/.test(draft) && draft.includes('title="missions.flow_publish_first"'), draft);
    const agentHtml = render(agent);
    check(`${view}: agent missions keep Prepare`, agentHtml.includes('data-mission-action="prepare"'));
    check(`${view}: agent missions keep an enabled Duplicate`, !/ disabled/.test(buttonFor(agentHtml, 'duplicate')) && buttonFor(agentHtml, 'duplicate').includes('missions.card_btn_duplicate_title'));
    check(`${view}: prepared agent missions keep view/invalidate`, render(preparedAgent).includes('data-mission-action="view-prepared"') && render(preparedAgent).includes('data-mission-action="invalidate-prepared"'));
}
const flowGrid = P.renderMissionGrid(flowLive, false);
check('grid: the execution pill names the flow type', flowGrid.includes('🧩<span>missions.filter_flow</span>'), flowGrid.match(/mc-trigger-pill[^]*?<\/div>/)?.[0]);
check('grid: data-status is flow', flowGrid.includes('data-status="flow"'));
check('list: the type icon is the flow icon', P.renderMissionCompact(flowLive).includes('🧩'));

// ── status chip ──
const chip = P.renderStatusChip(flowLive, false, false, false);
check('status chip: a flow is labelled Flow, not Manual', chip.includes('mc-status-chip--flow') && chip.includes('missions.filter_flow') && !chip.includes('missions.filter_manual'), chip);
check('status chip: a running flow shows the running state', P.renderStatusChip(flowLive, true, false, false).includes('missions.card_badge_running'));
check('status chip: manual missions keep their label', P.renderStatusChip(agent, false, false, false).includes('missions.filter_manual'));

// ── Run ──
reset();
await P.runMission('flow-2');
check('run: an unpublished flow sends no request and says why', requests.length === 0 && toasts.length === 1 && toasts[0].message === 'missions.flow_publish_first', JSON.stringify({ requests, toasts }));
reset();
await P.runMission('flow-1');
check('run: a published flow is started through the mission API', requests.some(r => r.url === '/api/missions/v2/flow-1/run' && r.method === 'POST'), JSON.stringify(requests));
check('run: a flow run says it runs in EasyDrag, not that it was queued', toasts.length === 1 && toasts[0].message === 'missions.toast_flow_run_requested', JSON.stringify(toasts));
reset();
await P.runMission('agent-1');
check('run: agent missions keep the dispatch toast', toasts.length === 1 && toasts[0].message === 'missions.toast_queued', JSON.stringify(toasts));

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
confirmAnswer = false;
setMissions(ALL);

// ── Flow filter ──
P.filterMissions('flow');
const grid = document.getElementById('missions-grid').innerHTML;
check('filter: Flow shows flow missions', grid.includes('data-mission-id="flow-1"') && grid.includes('data-mission-id="flow-2"'));
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

// ── strings: every key the flow paths use exists in all 16 locales ──
const langs = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const keys = ['missions.filter_flow', 'missions.flow_managed', 'missions.flow_publish_first', 'missions.confirm_delete_flow', 'missions.toast_flow_run_requested'];
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
