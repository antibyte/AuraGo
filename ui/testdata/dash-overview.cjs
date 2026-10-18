// Runs ui/js/dashboard/dash-overview.js in a bare Node vm context (no DOM) and checks its pure helpers:
// hidden-tile storage (round trip, garbage), every attention code against the real en.json strings, the
// tile/source mapping, safe section links, and the budget, context, activity, health and agent-state views.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const file = path.join(__dirname, '..', 'js', 'dashboard', 'dash-overview.js');
const ctx = {};
ctx.window = ctx;
vm.createContext(ctx);
vm.runInContext(fs.readFileSync(file, 'utf8'), ctx, { filename: file });
const O = ctx.DashOverview;
assert.ok(O, 'dash-overview.js must define window.DashOverview without touching the DOM at load time');
const json = value => JSON.stringify(value);

// Hidden tiles
assert.equal(json(Array.from(O.HIDEABLE_TILES)), json(['ov-system', 'ov-budget', 'ov-automations', 'ov-knowledge', 'ov-activity', 'ov-agent-state']));
assert.equal(O.STORAGE_KEY, 'aurago-dash-hidden-tiles');
assert.equal(O.serializeHiddenTiles(['ov-activity', 'ov-system', 'ov-system', 'ov-status', 'nope']), '["ov-system","ov-activity"]', 'hideable ids only, tile order, no duplicates');
assert.equal(json(O.parseHiddenTiles(O.serializeHiddenTiles(['ov-agent-state', 'ov-budget']))), '["ov-budget","ov-agent-state"]', 'round trip');
for (const garbage of [null, undefined, '', 'not json', '42', '{"ov-system":true}', '[1,2]', '["ov-status","ov-attention"]']) {
    assert.equal(json(O.parseHiddenTiles(garbage)), '[]', 'garbage ' + String(garbage));
}

// Attention texts from the real English bundle (same semantics as tOr in ui/js/shared/shared-core.js)
const en = JSON.parse(fs.readFileSync(path.join(__dirname, '..', 'lang', 'dashboard', 'en.json'), 'utf8'));
const tOr = (key, fallback, params) => {
    if (!Object.prototype.hasOwnProperty.call(en, key)) return fallback;
    let text = en[key];
    Object.entries(params || {}).forEach(([name, value]) => {
        text = text.replaceAll('{{' + name + '}}', value);
        text = text.replaceAll('{' + name + '}', value);
    });
    return text;
};
const samples = {
    operational_issue: [{ count: 1, params: { kind: 'runtime', occurrences: 3, source: 'cron', title: 'Backup job keeps failing' } }, 'Backup job keeps failing (3×)'],
    cron_errors: [{ count: 2, params: { first_id: 'nightly-backup' } }, 'Cron jobs with errors: 2 (first: nightly-backup)'],
    missions_failed: [{ count: 4, params: {} }, 'Failed mission runs in the last 24 h: 4'],
    daemon_failed: [{ count: 1, params: { first_name: 'Mail watcher' } }, 'Failed daemon skills: 1 (first: Mail watcher)'],
    budget_blocked: [{ count: 1, params: { percent: 104 } }, 'Daily budget used up (104%), LLM calls are blocked'],
    budget_warning: [{ count: 1, params: { percent: 85 } }, 'Daily budget at 85%'],
    disk_high: [{ count: 1, params: { percent: 93 } }, 'Disk 93% full'],
    memory_high: [{ count: 1, params: { percent: 96 } }, 'RAM usage at 96%'],
    indexer_error: [{ count: 3, params: {} }, 'File indexer errors: 3'],
    memory_conflicts: [{ count: 5, params: {} }, 'Open memory conflicts: 5'],
    localwiki_update: [{ count: 1, params: { date: '2026-10' } }, 'Local Wikipedia update available (2026-10)'],
};
assert.equal(json(Array.from(O.ATTENTION_CODES).sort()), json(Object.keys(samples).sort()), 'one sample per server attention code');
for (const [code, [item, want]] of Object.entries(samples)) {
    assert.equal(O.attentionText(Object.assign({ code }, item), tOr), want, code);
}
assert.equal(O.attentionText({ code: 'brand_new_code', count: 1, params: {} }, tOr), 'brand_new_code', 'unknown codes fall back to the code');
assert.equal(O.attentionText({ code: 'operational_issue', count: 1, params: { occurrences: 2, title: '{{count}} <b>' } }, tOr), '{{count}} <b> (2×)', 'a title is inserted as text, never as a template');

// Tiles, sources and links
assert.equal(O.tileUnavailable('ov-automations', { unavailable: ['missions'] }), true);
assert.equal(O.tileUnavailable('ov-knowledge', { unavailable: ['memory'] }), true, 'memory feeds core facts');
assert.equal(O.tileUnavailable('ov-knowledge', { unavailable: ['knowledge'] }), true);
assert.equal(O.tileUnavailable('ov-attention', { unavailable: ['operational_issues'] }), true);
assert.equal(O.tileUnavailable('ov-activity', { unavailable: ['missions', 'memory'] }), false);
assert.equal(O.tileUnavailable('ov-system', { unavailable: ['health'] }), false, 'a null health waits for the first sample');
assert.equal(O.tileUnavailable('constructor', { unavailable: ['missions'] }), false);
assert.equal(O.tileUnavailable('ov-activity', { unavailable: null }), false);
assert.equal(O.sectionHref('automations'), '#automations');
assert.equal(O.sectionHref('javascript:alert(1)'), '#overview', 'sections outside the sidebar never become links');

// Views
assert.equal(json(O.budgetView({ enabled: false, state: 'ok' })), '{"enabled":false}');
const warning = O.budgetView({ enabled: true, spent_usd: 4.5, daily_limit_usd: 5, percentage: 0.9, state: 'warning' });
assert.equal(json([warning.percent, warning.percentText, warning.level, warning.spent, warning.limit]), '[90,"90%","med","$4.50","$5.00"]');
const blocked = O.budgetView({ enabled: true, spent_usd: 6, daily_limit_usd: 5, percentage: 1.2, state: 'blocked' });
assert.equal(json([blocked.percent, blocked.percentText, blocked.level]), '[100,"120%","high"]', 'the bar is capped, the text is not');
assert.equal(O.contextPercent({ context_window: 1000 }, { total_chars: 2000 }), 50);
assert.equal(O.contextPercent({}, { total_chars: 1024000 }), 100, '128k default window, capped at 100');
const tr = (key, params) => key.split('.').pop() + (params ? ':' + params.n : '');
assert.equal(O.lastActivityText(-1, tr), 'agent_banner_no_activity');
assert.equal(O.lastActivityText(0.4, tr), 'agent_banner_just_active');
assert.equal(O.lastActivityText(5.6, tr), 'agent_banner_hours_ago:6');
assert.equal(O.lastActivityText(50, tr), 'agent_banner_days_ago:2');
assert.equal(json(O.healthFromMetrics({ cpu: { usage_percent: 12.5 }, memory: { used_percent: 40 }, disk: { used_percent: 91 }, uptime_seconds: 3600 })), '{"cpu_percent":12.5,"memory_percent":40,"disk_percent":91,"uptime_seconds":3600}');
assert.equal(O.healthFromMetrics(null), null);
assert.equal(O.uptimeText(7260), '2h 1m');
assert.equal(O.uptimeText(90000), '1d 1h');
const strained = O.agentStateView({ enabled: true, mood: 'focused', affect_valence: -0.5, affect_arousal: 0.7, dynamics: { trend: 'strained' } });
assert.equal(json([strained.affect, strained.valencePercent, strained.valenceText, strained.arousalPercent, strained.arousalText, strained.trend]), '[true,25,"-0.50",70,"0.70","strained"]');
const rest = O.agentStateView({ enabled: true, mood: 'curious', dynamics: { trend: 'odd' } });
assert.equal(json([rest.affect, rest.trend]), '[false,"steady"]', 'no affect while at rest; unknown trends read as steady');
assert.equal(O.agentStateView({ enabled: false }).enabled, false);
assert.equal(O.agentStateView({}).enabled, false);
console.log('DashOverview: hidden tiles, ' + Object.keys(samples).length + ' attention codes, tile sources and views pass.');
