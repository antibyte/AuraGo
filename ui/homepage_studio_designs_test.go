package ui

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHomepageStudioDesignsModuleIsWired(t *testing.T) {
	t.Parallel()

	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	if !strings.Contains(loader, "/js/desktop/apps/homepage-studio-designs.js") {
		t.Fatal("module loader does not load the homepage studio designs module")
	}

	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")
	for _, want := range []string{
		"window.HomepageStudioDesigns.create(",
		"window.HomepageStudioDesigns.tabMarkup(",
		"window.HomepageStudioDesigns.panelMarkup(",
		"state.modules.designs.setSite(detail)",
		"state.modules.designs.chatContext()",
		"state.modules.designs.getSelectedId()",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio missing design marker %q", want)
		}
	}

	/* The picker owns its markup and its tab registration. */
	module := readDesktopAssetText(t, "js/desktop/apps/homepage-studio-designs.js")
	for _, want := range []string{
		"hp-tab-designs-${windowId}",
		"hp-panel-designs-${windowId}",
		"hp-designs-list-${windowId}",
	} {
		if !strings.Contains(module, want) {
			t.Fatalf("designs module missing markup marker %q", want)
		}
	}
}

func TestHomepageStudioDesignsModuleUsesTheDesignEndpoints(t *testing.T) {
	t.Parallel()

	module := readDesktopAssetText(t, "js/desktop/apps/homepage-studio-designs.js")
	for _, want := range []string{
		"/api/homepage/designs",
		"/api/homepage/designs/select",
		"/preview?variant=",
		"data-hp-design",
		"vd-hp-design-preview",
		"vd-hp-design-palette",
	} {
		if !strings.Contains(module, want) {
			t.Fatalf("designs module missing marker %q", want)
		}
	}
	/* The catalog ships with AuraGo: the picker must not call out to the network. */
	for _, forbidden := range []string{"designmd.ai", "https://"} {
		if strings.Contains(module, forbidden) {
			t.Fatalf("designs module must not load remote assets, found %q", forbidden)
		}
	}
}

func TestHomepageStudioDesignsStylesExist(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop-app-homepage-studio.css")
	for _, want := range []string{
		".vd-hp-designs-list",
		".vd-hp-designs-grid",
		".vd-hp-design-card",
		".vd-hp-design-card.is-selected",
		".vd-hp-design-preview img",
		".vd-hp-design-swatch",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("homepage studio css missing design marker %q", want)
		}
	}
}

func TestHomepageStudioDesignsTranslationsExist(t *testing.T) {
	t.Parallel()

	keys := []string{
		"homepage_studio.designs_tab",
		"homepage_studio.designs_headline",
		"homepage_studio.designs_selected",
		"homepage_studio.designs_scope_site",
		"homepage_studio.designs_scope_new",
		"homepage_studio.designs_loading",
		"homepage_studio.designs_empty",
		"homepage_studio.designs_load_failed",
		"homepage_studio.designs_applied",
		"homepage_studio.designs_apply_failed",
		"homepage_studio.designs_readonly",
	}
	for _, locale := range []string{"en", "de"} {
		var values map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, filepath.Join("lang", "desktop", locale+".json")), &values); err != nil {
			t.Fatalf("decode %s: %v", locale, err)
		}
		for _, key := range keys {
			if strings.TrimSpace(values[key]) == "" {
				t.Errorf("%s is missing translation %s", locale, key)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Behaviour check
//
// The check below executes the shipped design picker module so it asserts on
// runtime behaviour instead of on source text.
// ---------------------------------------------------------------------------

// homepageStudioDesignsNodeScript wraps a Node body into a failing runner.
func homepageStudioDesignsNodeScript(t *testing.T, body string) string {
	t.Helper()
	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio-designs.js")
	encoded, err := json.Marshal(source)
	if err != nil {
		t.Fatalf("encode designs module: %v", err)
	}
	return "(async () => {\n" +
		"const assert = require('node:assert/strict');\n" +
		"const sandbox = { location: { origin: 'https://aurago.test' } };\n" +
		"(new Function('window', " + string(encoded) + "))(sandbox);\n" +
		"const Designs = sandbox.HomepageStudioDesigns;\n" +
		"if (!Designs) throw new Error('designs module did not load');\n" +
		body +
		"\n})().catch(error => { console.error(error && error.stack ? error.stack : error); process.exit(1); });\n"
}

// runHomepageStudioDesignsNode executes a Node script against the module.
func runHomepageStudioDesignsNode(t *testing.T, script string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node runtime required for the Homepage Studio design behaviour check")
	}
	path := filepath.Join(t.TempDir(), "homepage-studio-designs-check.cjs")
	if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
		t.Fatalf("write design check script: %v", err)
	}
	if output, err := exec.Command(node, path).CombinedOutput(); err != nil {
		t.Fatalf("design picker behaviour check failed: %v\n%s", err, output)
	}
}

// TestHomepageStudioDesignsPickerRendersAndSelects proves that the shipped
// picker renders every catalog entry with a preview, escapes server values and
// only sends a validated design id to the server.
func TestHomepageStudioDesignsPickerRendersAndSelects(t *testing.T) {
	t.Parallel()

	script := homepageStudioDesignsNodeScript(t, `
const calls = [];
const notices = [];
const changes = [];

function makeContainer() {
	return { innerHTML: '', listeners: {}, addEventListener(name, fn) { (this.listeners[name] = this.listeners[name] || []).push(fn); }, contains() { return true; } };
}

const esc = value => String(value == null ? '' : value)
	.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
	.replace(/"/g, '&quot;').replace(/'/g, '&#39;');

const templates = [];
for (let index = 1; index <= 25; index++) {
	templates.push({
		id: 'design-' + index,
		name: index === 25 ? '"><script>alert(1)</script>' : 'Design ' + index,
		author: 'chef',
		likes: 40 - index,
		palette: ['#112233', '#aabbcc', 'not-a-color']
	});
}

/* The binding lives with the site: one site has a design, the other has none. */
const bindings = { '/api/homepage/designs?project_dir=my-site': 'design-3' };

const api = async (path, options) => {
	calls.push({ path: path, options: options });
	if (path.indexOf('/api/homepage/designs?') === 0) {
		return { designs: templates, selected_design: bindings[path] || '' };
	}
	return { status: 'ok', applied: true, registered: true };
};

function makePicker(container) {
	return Designs.create({
		api: api,
		esc: esc,
		t: key => key,
		notify: (message, kind) => notices.push({ message: message, kind: kind }),
		readonly: false,
		container: container,
		isDisposed: () => false,
		onDesignChange: id => changes.push(id)
	});
}

const container = makeContainer();
const picker = makePicker(container);
picker.setSite({ project_dir: 'my-site', design_id: 'design-3' });
await picker.load();

assert.equal(calls[0].path, '/api/homepage/designs?project_dir=my-site', 'the panel asks for the project binding');

const html = container.innerHTML;
assert.equal((html.match(/data-hp-design="/g) || []).length, 25, 'every template renders a card');
assert.ok(html.indexOf('data-hp-design="design-1"') >= 0, 'a card carries its template id');
assert.ok(html.indexOf('/api/homepage/designs/design-1/preview?variant=light') >= 0, 'a card carries its preview');
assert.equal((html.match(/vd-hp-design-swatch/g) || []).length, 50, 'only valid palette colors become swatches');
assert.ok(html.indexOf('not-a-color') < 0, 'an invalid palette color is dropped');
assert.ok(html.indexOf('aria-pressed="true"') >= 0, 'the bound template is marked as selected');
assert.ok(html.indexOf('<script>alert(1)</script>') < 0, 'a hostile template name must be escaped');
assert.ok(html.indexOf('&lt;script&gt;') >= 0, 'a hostile template name stays visible as text');
assert.equal(picker.getSelectedId(), 'design-3', 'the server selection wins');

await picker.select('design-7');
const post = calls[calls.length - 1];
assert.equal(post.path, '/api/homepage/designs/select', 'a selection is posted');
assert.equal(post.options.method, 'POST');
assert.deepEqual(JSON.parse(post.options.body), { project_dir: 'my-site', design_id: 'design-7' });
assert.equal(picker.getSelectedId(), 'design-7');
assert.equal(changes.length, 1, 'the studio learns about the selection');
assert.equal(changes[0], 'design-7');
assert.ok(container.innerHTML.indexOf('data-hp-design="design-7"') >= 0 && container.innerHTML.indexOf('aria-pressed="true"') >= 0, 'the selected card is active');

/* The chat turn carries the design and the site it belongs to. */
assert.deepEqual(picker.chatContext(), { design_id: 'design-7', project_dir: 'my-site' });

const before = calls.length;
await picker.select('../../etc/passwd');
assert.equal(calls.length, before, 'an invalid design id never reaches the server');

const pendingContainer = makeContainer();
const pending = makePicker(pendingContainer);
await pending.load('');
assert.equal(calls[calls.length - 1].path, '/api/homepage/designs', 'without a site the panel asks for the plain catalog');
const beforePending = calls.length;
await pending.select('design-2');
assert.equal(calls.length, beforePending, 'without a site the selection is not posted');
assert.equal(changes[changes.length - 1], 'design-2', 'without a site the selection rides along with the chat turn');
assert.deepEqual(pending.chatContext(), { design_id: 'design-2' }, 'without a site only the design travels');

/* A site without a design must not inherit the design of the previous site. */
picker.setSite({ project_dir: 'other-site', design_id: '' });
await picker.load();
assert.equal(picker.getSelectedId(), '', 'a site without a design has no selection');
assert.deepEqual(picker.chatContext(), { project_dir: 'other-site' }, 'a site without a design carries no design id');

/* A site with a design restores its own binding. */
bindings['/api/homepage/designs?project_dir=my-site'] = 'design-9';
picker.setSite({ project_dir: 'my-site', design_id: 'design-9' });
await picker.load();
assert.equal(picker.getSelectedId(), 'design-9', 'the binding of the site wins');

/* Clearing the selection is allowed. */
picker.setSelectedId('');
assert.equal(picker.getSelectedId(), '', 'an empty id clears the selection');
`)

	runHomepageStudioDesignsNode(t, script)
}
