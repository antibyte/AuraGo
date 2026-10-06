#!/usr/bin/env node
// Runs the dashboard widgets in Node on a stub DOM and checks that cron jobs owned by EasyDrag (a flow's schedule)
// render read-only in both renderers: the Cronjobs tab (renderCronjobs, managed_by "easydrag") and the activity
// widget (renderActivity, source "flow"). The server refuses edit, toggle and delete of those jobs with 409.
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const file = path.join(here, '..', 'ui', 'js', 'dashboard', 'dashboard-widgets.js');

class El {
    constructor(tag) { this.tagName = String(tag || 'div').toUpperCase(); this.children = []; this.innerHTML = ''; this.textContent = ''; this.style = {}; this.className = ''; this.dataset = {}; }
    appendChild(node) { this.children.push(node); return node; }
    setAttribute() {}
    addEventListener() {}
}
const byId = new Map();
const document = {
    getElementById: (id) => { if (!byId.has(id)) byId.set(id, new El('div')); return byId.get(id); },
    createElement: (tag) => new El(tag),
    documentElement: { lang: 'en' },
    addEventListener() {}
};
const t = (key, params) => key + (params ? ' ' + JSON.stringify(params) : '');
const esc = (s) => String(s == null ? '' : s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
const truncate = (s, max) => (String(s || '').length > max ? String(s).slice(0, max) + '…' : String(s || ''));

const sandbox = { window: {}, document, console, t, esc, truncate, LANG: 'en', setTimeout, clearTimeout, URLSearchParams, AbortController };
sandbox.globalThis = sandbox;
vm.createContext(sandbox);
vm.runInContext(fs.readFileSync(file, 'utf8'), sandbox, { filename: file });

let failures = 0;
function check(name, cond, detail) {
    if (cond) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + (detail ? ' — ' + detail : ''));
}
const hasEdit = (html) => html.includes('openCronEditModal');
const hasDelete = (html) => html.includes('deleteCronJob');
// The label carries its hint as a tooltip and as visually hidden text (screen readers, keyboard users).
const hasManaged = (html) => html.includes('class="cronjobs-managed"') && html.includes('dashboard.cronjobs_managed_easydrag') && html.includes('title="dashboard.cronjobs_managed_easydrag_hint"') &&
    html.includes('<span class="visually-hidden"> dashboard.cronjobs_managed_easydrag_hint</span>');
// Hostile ids and expressions: nothing of them may become markup.
const HOSTILE_ID = 'x<img src=y onerror=z>"&';
const HOSTILE_EXPR = '*/5 "<b>"&';
const escapedOnly = (html) => !html.includes('<img') && !html.includes('<b>') && html.includes('x&lt;img src=y onerror=z&gt;&quot;&amp;') && html.includes('*/5 &quot;&lt;b&gt;&quot;&amp;');

// ── Cronjobs tab ──
sandbox.renderCronjobs({
    jobs: [
        { id: 'mission_m1__n1', cron_expr: '0 9 * * *', task_prompt: '', source: 'flow', status: 'enabled', registered: true, managed_by: 'easydrag' },
        { id: 'agent-job', cron_expr: '*/5 * * * *', task_prompt: 'check mail', source: 'agent', status: 'enabled', registered: true }
    ],
    total: 2, enabled: 2, disabled: 0, errors: 0
});
const rows = document.getElementById('cronjobs-tbody').children;
check('tab: renders both jobs', rows.length === 2, String(rows.length));
const flowRow = (rows[0] || {}).innerHTML || '';
const agentRow = (rows[1] || {}).innerHTML || '';
check('tab: managed job has no Edit and no Delete', !hasEdit(flowRow) && !hasDelete(flowRow));
check('tab: managed job shows the EasyDrag label with its hint', hasManaged(flowRow));
check('tab: managed job names EasyDrag as its source', flowRow.includes('cronjobs-source-flow') && flowRow.includes('dashboard.cronjobs_source_flow'));
check('tab: other jobs keep Edit and Delete', hasEdit(agentRow) && hasDelete(agentRow) && !agentRow.includes('cronjobs-managed'));

// ── activity widget ──
sandbox.renderActivity({
    cron_jobs: [
        { id: 'mission_m1__n1', cron_expr: '0 9 * * *', task_prompt: '', source: 'flow' },
        { id: 'agent-job', cron_expr: '*/5 * * * *', task_prompt: 'check mail', source: 'agent' }
    ]
});
const details = document.getElementById('activity-details').innerHTML;
const items = details.split('<div class="activity-item">').slice(1);
check('activity: renders both jobs', items.length === 2, String(items.length));
check('activity: flow job has no Edit and no Delete', items[0] && !hasEdit(items[0]) && !hasDelete(items[0]));
check('activity: flow job shows the EasyDrag label with its hint', items[0] && hasManaged(items[0]));
check('activity: other jobs keep Edit and Delete', items[1] && hasEdit(items[1]) && hasDelete(items[1]) && !items[1].includes('cronjobs-managed'));

// ── hostile ids and expressions, managed and not, in both renderers ──
document.getElementById('cronjobs-tbody').children = [];
sandbox.renderCronjobs({
    jobs: [
        { id: HOSTILE_ID, cron_expr: HOSTILE_EXPR, task_prompt: '', source: 'flow', status: 'enabled', registered: true, managed_by: 'easydrag' },
        { id: HOSTILE_ID, cron_expr: HOSTILE_EXPR, task_prompt: 'p', source: 'agent', status: 'enabled', registered: true }
    ],
    total: 2, enabled: 2, disabled: 0, errors: 0
});
const hostileRows = document.getElementById('cronjobs-tbody').children.map(row => row.innerHTML);
check('tab: a hostile id and expression render escaped (managed job)', hostileRows[0] && escapedOnly(hostileRows[0]));
check('tab: a hostile id and expression render escaped (other job)', hostileRows[1] && escapedOnly(hostileRows[1]));
sandbox.renderActivity({
    cron_jobs: [
        { id: HOSTILE_ID, cron_expr: HOSTILE_EXPR, task_prompt: '', source: 'flow' },
        { id: HOSTILE_ID, cron_expr: HOSTILE_EXPR, task_prompt: 'p', source: 'agent' }
    ]
});
const hostileItems = document.getElementById('activity-details').innerHTML.split('<div class="activity-item">').slice(1);
check('activity: a hostile id and expression render escaped (flow job)', hostileItems.length === 2 && escapedOnly(hostileItems[0]));
check('activity: a hostile id and expression render escaped (other job)', hostileItems.length === 2 && escapedOnly(hostileItems[1]));

if (failures) { console.error(`${failures} check(s) failed`); process.exit(1); }
console.log('all dashboard cron job checks passed');
