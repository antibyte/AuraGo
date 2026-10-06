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
const hasManaged = (html) => html.includes('class="cronjobs-managed"') && html.includes('dashboard.cronjobs_managed_easydrag') && html.includes('title="dashboard.cronjobs_managed_easydrag_hint"');

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

if (failures) { console.error(`${failures} check(s) failed`); process.exit(1); }
console.log('all dashboard cron job checks passed');
