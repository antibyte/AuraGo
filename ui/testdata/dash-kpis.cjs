// Takes the P5 KPI helpers out of the dashboard scripts without running their top-level code (function declarations
// are hoisted, so a wrapper returns them before the script body runs) and checks them on fixtures.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const dashboard = path.join(__dirname, '..', 'js', 'dashboard');
const json = value => JSON.stringify(value);
// Like shared-core.js: esc escapes &, < and > (no quotes); escAttr adds both quote kinds.
const esc = s => String(s).replace(/[&<>]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' }[c]));
const escAttr = s => esc(s).replace(/'/g, '&#39;').replace(/"/g, '&quot;');

function take(file, names) {
    const ctx = { esc, escAttr, document: { documentElement: { lang: 'en' } } };
    ctx.window = ctx;
    vm.createContext(ctx);
    const source = fs.readFileSync(path.join(dashboard, file), 'utf8');
    return vm.runInContext('(function () {\nreturn { ' + names.join(', ') + ' };\n' + source + '\n})', ctx, { filename: file })();
}

// renderDashKpiRow (main.js): numbers in the page locale, "—" for missing values, escaped text, escAttr'd title and
// action, an action cell as one button.
{
    const { renderDashKpiRow } = take('main.js', ['renderDashKpiRow']);
    const row = { innerHTML: 'old' };
    renderDashKpiRow(row, [
        { label: 'Builds', value: 1234 },
        { label: 'Avg <tokens>', value: null },
        { label: 'Zero', value: 0 },
        { label: 'Text', value: '< 0.1 %' },
        { label: 'Facts', value: 3, action: 'memory-core-facts', title: 'Show "core" facts' },
        { label: 'Issues', value: undefined, title: 'only with semantic search' },
        { label: 'Broken', value: NaN },
    ]);
    assert.equal(row.innerHTML,
        '<div class="dash-kpi" role="listitem"><span class="dash-kpi-value">1,234</span><span class="dash-kpi-label">Builds</span></div>' +
        '<div class="dash-kpi" role="listitem"><span class="dash-kpi-value">—</span><span class="dash-kpi-label">Avg &lt;tokens&gt;</span></div>' +
        '<div class="dash-kpi" role="listitem"><span class="dash-kpi-value">0</span><span class="dash-kpi-label">Zero</span></div>' +
        '<div class="dash-kpi" role="listitem"><span class="dash-kpi-value">&lt; 0.1 %</span><span class="dash-kpi-label">Text</span></div>' +
        '<div class="dash-kpi" role="listitem"><button type="button" class="dash-kpi-button" data-action="memory-core-facts" title="Show &quot;core&quot; facts"><span class="dash-kpi-value">3</span><span class="dash-kpi-label">Facts</span></button></div>' +
        '<div class="dash-kpi" role="listitem" title="only with semantic search"><span class="dash-kpi-value">—</span><span class="dash-kpi-label">Issues</span></div>' +
        '<div class="dash-kpi" role="listitem"><span class="dash-kpi-value">—</span><span class="dash-kpi-label">Broken</span></div>');
    renderDashKpiRow(null, [{ label: 'x', value: 1 }]);
    renderDashKpiRow(row, null);
    assert.equal(row.innerHTML, '', 'no items empty the row');
}

// toolingTelemetryKpis (sections/tools.js): format errors are the classifier_format_error_* and classifier_schema_error_*
// recovery events; recoveries and policy signals sum their maps; counts below 1 and non-numbers are skipped (as the
// lists do); failing tools counts the tools with at least one failed call.
{
    const { toolingTelemetryKpis } = take('sections/tools.js', ['toolingTelemetryKpis']);
    const kpis = toolingTelemetryKpis({
        agent_telemetry: {
            recovery_events: { classifier_format_error_raw_code: 2, classifier_schema_error_incomplete_tool_call: 3, classifier_empty_response_no_output: 4, provider_422_recovered: 1, duplicate_tool_call_blocked: '5', broken: 'x', negative: -2 },
            policy_events: { conservative_profile_applied: 6, family_guarded_shell: 1, unused: 0 },
        },
        by_tool: { shell: { failure_count: 2, total_calls: 9 }, files: { failure_count: 0, total_calls: 4 }, web: { total_calls: 3 }, mcp_call: { failure_count: '1' } },
    });
    assert.equal(json(kpis), json({ formatErrors: 5, recoveries: 15, policySignals: 7, failingTools: 2 }));
    assert.equal(json(toolingTelemetryKpis(null)), json({ formatErrors: 0, recoveries: 0, policySignals: 0, failingTools: 0 }));
}

console.log('dashboard KPI helpers pass.');
