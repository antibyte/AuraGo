package ui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// Hostile values for agent- and LLM-written dashboard data. Each starts with a unique marker, so an attribute value
// that holds only part of one (the attribute ended early) is found.
var dashboardHostileValues = map[string]string{
	"q":   `XQ1x" autofocus onfocus="alert(1)`,
	"mix": `XM2a'b"c<d>`,
	"sq":  `XS3y' onmouseover='alert(2)`,
}

// TestDashboardAttributeEscapingBrowser renders the dashboard renderers that put agent-written values into
// attributes with hostile values in a real browser, using shared-core.js's own esc and escAttr. No value may end its
// attribute (no extra attributes, every attribute holding a marker holds the whole value), text positions keep the
// value as text, and the knowledge graph detail buttons pass the values unchanged through main.js's delegated
// click listener.
func TestDashboardAttributeEscapingBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("browser required for the dashboard attribute escaping check")
	}

	core := normalizeAssetText(mustReadUIFile(t, "js/shared/shared-core.js"))
	start := strings.Index(core, "function esc(text)")
	end := strings.Index(core, "function escJs(text)")
	if start < 0 || end < start {
		t.Fatal("shared-core.js must define esc and escAttr before escJs")
	}
	helpers := core[start:end]

	template := regexp.MustCompile(`{{[^}]*}}`)
	var sections strings.Builder
	for _, id := range []string{"automations", "user", "problems", "audit", "tools", "system", "knowledge"} {
		sections.WriteString(template.ReplaceAllString(readDashboardSection(t, id), ""))
	}

	hostile, err := json.Marshal(dashboardHostileValues)
	if err != nil {
		t.Fatal(err)
	}
	h := dashboardHostileValues
	detail, err := json.Marshal(map[string]any{
		"node":      map[string]any{"id": h["q"], "label": h["mix"], "properties": map[string]any{"type": h["sq"], "note": h["q"]}},
		"neighbors": []any{map[string]any{"id": h["mix"], "label": h["q"]}},
		"edges":     []any{map[string]any{"source": h["q"], "target": h["mix"], "relation": h["sq"], "properties": map[string]any{"why": h["mix"]}}},
	})
	if err != nil {
		t.Fatal(err)
	}

	page := `<!doctype html><html lang="en"><head><meta charset="utf-8"></head><body>` + sections.String() + `
<div id="kgDetailOverlay"><div id="kgDetailBody"></div></div>
<script>
window.__errors = [];
addEventListener('error', e => __errors.push(String((e && e.message) || e)));
addEventListener('unhandledrejection', e => __errors.push(String(e.reason)));
window.HOSTILE = ` + string(hostile) + `;
window.LANG = 'en';
window.t = (key) => key;
window.tOr = (key, fallback) => fallback;
window.dashIcon = () => '';
window.showToast = () => {};
window.showConfirm = async () => true;
window.DashSections = { register() {} };
window.DashSummary = { init() {} };
window.DashOverview = { init() {} };
window.DashRouter = { init() {}, current: () => 'knowledge' };
</script>
<script>` + helpers + `</script>
<script src="/chart.min.js"></script>
<script src="/js/dashboard/dash-disclosures.js"></script>
<script src="/js/dashboard/main.js"></script>
<script src="/js/dashboard/widgets-knowledge.js"></script>
<script src="/js/dashboard/sections/automations.js"></script>
<script src="/js/dashboard/sections/user.js"></script>
<script src="/js/dashboard/sections/problems.js"></script>
<script src="/js/dashboard/sections/audit.js"></script>
<script src="/js/dashboard/sections/tools.js"></script>
<script src="/js/dashboard/sections/system.js"></script>
</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(page))
		case "/api/knowledge-graph/node":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(detail)
		default:
			http.FileServer(http.FS(Content)).ServeHTTP(w, r)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	l := launcher.New().Context(ctx).Bin(bin).Headless(true).NoSandbox(true)
	u, err := l.Launch()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { l.Kill(); l.Cleanup() }()
	b := rod.New().Context(ctx).ControlURL(u).MustConnect()
	defer b.Close()
	p := b.MustPage(server.URL).MustWaitLoad()
	defer p.Close()

	result := p.MustEval(`async () => {
 if (__errors.length) return { problems: ['the scripts failed to load'], errors: __errors };
 const H = HOSTILE;
 const problems = [];
 const expect = (cond, what) => { if (!cond) problems.push(what); };

 // Renderers that place agent-written values in attributes.
 renderCronjobs({ jobs: [{ id: H.mix, cron_expr: H.sq, task_prompt: H.q, source: H.q, status: 'error', last_error: H.mix }], total: 1, errors: 1 });
 renderMissionHistory({ entries: [{ mission_id: H.q, mission_name: H.mix, status: 'success', trigger_type: 'cron' }] }, false);
 renderDaemonsCard([{ skill_id: H.q, skill_name: H.mix, status: 'error', last_error: H.q }]);
 renderProfile({ categories: { [H.q]: [{ key: H.mix, value: H.q, confidence: 2 }] } });
 renderJournalSummary([{ date: '2026-10-18', summary: 'ok', sentiment: H.q }]);
 renderErrorPatterns({ frequent: [{ tool_name: H.mix, error_message: H.q, count: 3 }], recent: [] });
 renderAuditEvents({ entries: [{ id: 7, source: H.q, event_type: 'x', target_id: H.q, target_name: H.mix, status: H.q, summary: H.mix, detail: H.sq }], total: 1 });
 renderActivity({ cron_jobs: [{ id: H.mix, cron_expr: H.sq, task_prompt: H.q, source: H.sq }], coagents: [{ id: 'c1', task: H.mix, state: 'running', specialist: 'coder' }] });
 renderAdaptiveToolStats({ adaptive_enabled: true, adaptive_scores: [{ tool: H.q, score: 3 }], last_tool_filter_report: { largest_schemas: [{ name: H.mix, rough_tokens: 10 }] } });
 renderGitHubRepos({ enabled: true, owner: H.mix, repos: [{ name: H.mix, html_url: 'https://github.invalid/' + H.q, description: H.sq }] });
 const nodes = [{ id: H.q, label: H.mix, properties: { type: H.sq } }, { id: H.mix, label: H.q, properties: {} }];
 const edges = [{ source: H.q, target: H.mix, relation: H.sq }];
 KnowledgeGraphState.nodes = nodes;
 KnowledgeGraphState.edges = edges;
 renderKnowledgeGraphSummary(nodes, edges, null);
 renderKnowledgeGraphLists(nodes, edges);
 renderKnowledgeGraphSearchState(H.q, { nodes, edges });
 renderKnowledgeGraphQuality({ isolated_sample: nodes, untyped_sample: [], generic_sample: [], duplicate_candidates: [],
   id_duplicate_candidates: [{ ids: [H.q, H.mix], label: H.mix, recommended_target_id: H.q, count: 2 }] });
 renderKnowledgeGraphLegend();

 // The knowledge graph detail panel with both editors open.
 window.renderKnowledgeGraphVisual = () => {};
 const edgeKey = knowledgeGraphEdgeIdentity(edges[0]);
 KnowledgeGraphState.editingNodeId = H.q;
 KnowledgeGraphState.editingEdgeKey = edgeKey;
 await loadKnowledgeGraphNodeDetail(H.q);
 expect(document.querySelectorAll('#kgDetailBody [data-kg-action]').length === 9, 'detail panel renders 9 action buttons, got ' + document.querySelectorAll('#kgDetailBody [data-kg-action]').length);
 expect(document.querySelector('#kgDetailBody .knowledge-detail-title')?.textContent === H.mix, 'detail title keeps the label as text');
 expect(document.getElementById('knowledge-edit-label')?.value === H.mix, 'node editor input value round-trips');
 expect(document.getElementById('knowledge-edit-edge-relation')?.value === H.sq, 'edge editor input value round-trips');

 // No value ended its attribute: no injected attributes, every attribute that holds a marker holds the whole value.
 const markers = Object.values(H).map(v => [v.slice(0, 3), v]);
 for (const el of document.body.querySelectorAll('*')) {
   for (const attr of el.attributes) {
     if (!/^[a-zA-Z][a-zA-Z0-9-]*$/.test(attr.name) || ['autofocus', 'onfocus', 'onmouseover'].includes(attr.name)) {
       problems.push('injected attribute ' + JSON.stringify(attr.name) + ' on <' + el.tagName.toLowerCase() + ' class="' + el.getAttribute('class') + '">');
     }
     for (const [marker, full] of markers) {
       if (attr.value.includes(marker) && !attr.value.includes(full)) {
         problems.push('attribute ' + attr.name + ' on <' + el.tagName.toLowerCase() + ' class="' + el.getAttribute('class') + '"> holds a cut value ' + JSON.stringify(attr.value));
       }
     }
   }
 }
 expect(document.getElementsByTagName('d').length === 0, 'no <d> element from a text position');

 // Attribute values round-trip exactly where the code reads them back.
 const all = sel => [...document.querySelectorAll(sel)];
 expect(all('[data-cron-prompt]').length === 2 && all('[data-cron-prompt]').every(el => el.dataset.cronPrompt === H.q), 'data-cron-prompt round-trips');
 expect(all('[data-cron-id]').length === 4 && all('[data-cron-id]').every(el => el.dataset.cronId === H.mix), 'data-cron-id round-trips');
 expect(all('[data-cron-expr]').every(el => el.dataset.cronExpr === H.sq), 'data-cron-expr round-trips');
 expect(all('.profile-btn-edit').length === 1 && all('.profile-btn-edit, .profile-btn-delete').every(el => el.dataset.key === H.mix && el.dataset.cat === H.q), 'profile data-key and data-cat round-trip');
 expect(document.querySelector('.profile-entry')?.dataset.search === H.mix + ' ' + H.q, 'profile data-search round-trips');
 expect(document.querySelector('.profile-edit-input')?.value === H.q, 'profile edit input value round-trips');
 expect(document.querySelector('.profile-val')?.textContent === H.q, 'profile value stays text');
 expect(document.querySelector('.sentiment-day-badge')?.getAttribute('class') === 'sentiment-day-badge ' + H.q, 'sentiment class keeps the whole value');
 expect(document.querySelector('.error-pattern-msg')?.title === H.q, 'error pattern title round-trips');
 expect(document.querySelector('.gh-repo-name')?.getAttribute('href') === 'https://github.invalid/' + H.q, 'repository link round-trips');
 const openNodes = all('[data-kg-open-node]').map(el => el.dataset.kgOpenNode);
 expect(openNodes.length >= 8 && openNodes.every(id => id === H.q || id === H.mix), 'data-kg-open-node round-trips: ' + JSON.stringify(openNodes));
 const merge = document.querySelector('[data-kg-merge-source]');
 expect(merge && merge.dataset.kgMergeSource === H.mix && merge.dataset.kgMergeTarget === H.q && merge.dataset.kgMergeLabel === H.mix, 'merge button data round-trips');

 // The detail buttons reach the same functions with the same arguments through main.js's delegated listener.
 const calls = [];
 const record = name => (...args) => { calls.push([name, ...args]); };
 window.setupAuditControls = () => {};
 window.setupCronjobsControls = () => {};
 initDashboard();
 window.toggleKnowledgeGraphEdit = record('toggleKnowledgeGraphEdit');
 window.toggleKnowledgeGraphProtection = record('toggleKnowledgeGraphProtection');
 window.deleteKnowledgeGraphNode = record('deleteKnowledgeGraphNode');
 window.saveKnowledgeGraphNodeEdit = record('saveKnowledgeGraphNodeEdit');
 window.toggleKnowledgeGraphEdgeEdit = record('toggleKnowledgeGraphEdgeEdit');
 window.deleteKnowledgeGraphEdge = record('deleteKnowledgeGraphEdge');
 window.saveKnowledgeGraphEdgeEdit = record('saveKnowledgeGraphEdgeEdit');
 all('#kgDetailBody [data-kg-action]').forEach(btn => btn.click());
 const got = calls.map(c => JSON.stringify(c)).sort();
 const want = [
   ['toggleKnowledgeGraphEdit', H.q], ['toggleKnowledgeGraphEdit', H.q],
   ['toggleKnowledgeGraphProtection', H.q, true],
   ['deleteKnowledgeGraphNode', H.q, H.mix],
   ['saveKnowledgeGraphNodeEdit', H.q],
   ['toggleKnowledgeGraphEdgeEdit', edgeKey], ['toggleKnowledgeGraphEdgeEdit', edgeKey],
   ['deleteKnowledgeGraphEdge', H.q, H.mix, H.sq],
   ['saveKnowledgeGraphEdgeEdit', H.q, H.mix, H.sq],
 ].map(c => JSON.stringify(c)).sort();
 expect(JSON.stringify(got) === JSON.stringify(want), 'detail button calls: ' + JSON.stringify(got));
 expect(!document.activeElement || document.activeElement === document.body, 'nothing took focus');

 return { problems, errors: __errors };
}`)
	if problems := result.Get("problems").Arr(); len(problems) != 0 {
		t.Errorf("attribute escaping problems:\n%s", result.Get("problems").JSON("", "  "))
	}
	if errs := result.Get("errors").Arr(); len(errs) != 0 {
		t.Errorf("page errors: %s", result.Get("errors").JSON("", ""))
	}
}
