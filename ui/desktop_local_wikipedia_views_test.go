package ui

import (
	"os/exec"
	"testing"
)

func TestLocalWikipediaViewHelpers(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Local Wikipedia view checks")
	}
	script := `
const assert = require('node:assert/strict');
global.window = {};
require('./js/desktop/apps/local-wikipedia-views.js');
const V = window.LocalWikipediaViews;
const labels = require('./lang/desktop/de.json');
const t = (key, params) => { let text = labels[key] || key; for (const [name, value] of Object.entries(params || {})) text = text.split('{' + name + '}').join(String(value)); return text; };

assert.equal(V.contentURL('Berlin'), '/api/desktop/local-wikipedia/content/Berlin');
assert.equal(V.contentURL('AC/DC'), '/api/desktop/local-wikipedia/content/AC/DC');
assert.equal(V.contentURL('Café (Begriffsklärung)'), '/api/desktop/local-wikipedia/content/Caf%C3%A9%20(Begriffskl%C3%A4rung)');
assert.equal(V.contentURL('Frage?#1'), '/api/desktop/local-wikipedia/content/Frage%3F%231');
for (const bad of ['', '..', '../status', 'a/./b', 'a/..']) assert.equal(V.contentURL(bad), '', 'unsafe path ' + bad);
for (const path of ['Berlin', 'AC/DC', 'Café (Begriffsklärung)', 'Frage?#1', '100%_Wolle']) {
  assert.equal(V.pathFromLocation(new URL(V.contentURL(path), 'http://x').pathname), path, 'round trip ' + path);
}
assert.equal(V.pathFromLocation('/api/desktop/local-wikipedia/status'), '');
assert.equal(V.pathFromLocation('/api/desktop/local-wikipedia/content/%E0%A4%A'), '');
assert.equal(V.isContentPath('/api/desktop/local-wikipedia/content/Berlin'), true);
assert.equal(V.isContentPath('/api/vault'), false);
assert.equal(V.percent({progress: 0.425}), 42);
assert.equal(V.percent({progress: 42.5}), 42);
assert.equal(V.percent({progress: 1}), 100);
assert.equal(V.percent({progress: 250}), 100);
assert.equal(V.percent({}), 0);
assert.equal(V.percent({progress: 'x'}), 0);
assert.equal(V.readable({readable: true, state: 'ready', edition: {}}), true);
assert.equal(V.readable({readable: true, state: 'downloading', edition: {}}), true, 'an update download keeps the old edition readable');
assert.equal(V.readable({readable: true, state: 'interrupted', edition: {}}), true, 'an interrupted update keeps the old edition readable');
assert.equal(V.readable({readable: false, state: 'ready', edition: {}}), false, 'readable comes from the server flag only');
assert.equal(V.readable({state: 'ready', edition: {}}), false, 'no state or edition heuristics');
assert.equal(V.readable({readable: 'true'}), false);
assert.equal(V.readable(null), false);
assert.equal(V.stateKind({state: 'interrupted'}), 'interrupted');
assert.equal(V.stateKind({state: 'ready'}), 'not_installed');
assert.equal(V.stateKind({state: 'something-new'}), 'failed');
assert.equal(V.stateKind(null), 'failed');
assert.equal(V.stateKind({loading: true, state: 'not_installed', error_code: 'busy'}), 'loading');
assert.equal(V.stateKind({state: 'downloading'}), 'downloading');
assert.equal(V.stateKind({state: 'error', error_code: 'zim_unreadable', edition: {}}), 'error');
assert.equal(V.stateKind({state: 'error', error_code: 'state_unreadable'}), 'state_unreadable');
assert.equal(V.stateKind({state: 'interrupted', error_code: 'state_unreadable'}), 'state_unreadable');
assert.equal(V.stateKind({state: 'error', error_code: 'download_failed'}), 'install_failed');
assert.equal(V.stateKind({state: 'error', error_code: 'zim_unreadable'}), 'install_failed', 'a downloaded file that cannot be read is a failed install');
assert.equal(V.monthLabel('2026-10', 'de'), 'Oktober 2026');
assert.equal(V.monthLabel('2026-10', 'en'), 'October 2026');
assert.equal(V.monthLabel('soon', 'de'), 'soon');
assert.equal(V.esc('<b title="x">\'&</b>'), '&lt;b title=&quot;x&quot;&gt;&#39;&amp;&lt;/b&gt;');

const results = V.results([{title: 'Berlin <i>', path: 'Berlin', snippet: '<script>alert(1)</script>'}], '<q>', t, 'done');
assert(results.includes('&lt;script&gt;alert(1)&lt;/script&gt;') && !results.includes('<script>'), 'snippets are escaped');
assert(results.includes('data-path="Berlin"') && results.includes('„&lt;q&gt;“'), 'heading and path');
assert(V.results([], 'x', t, 'done').includes(labels['desktop.local_wikipedia_no_results']));
assert(V.results([], 'x', t, 'error').includes('role="alert"'));
assert(V.results([], 'x', t, 'loading').includes(labels['desktop.local_wikipedia_searching']));
const footer = V.footer({language: 'de', variant: 'nopic', date: '2026-10'}, t, 'de');
assert(footer.includes('Deutsch · ohne Medien · Stand Oktober 2026'), footer);
assert.equal(V.footer(null, t, 'de'), '');
const suggestions = V.suggestions([{title: 'Bern', path: 'Bern'}, {title: '<x>', path: 'x'}], 1, 'lw-suggest-w1');
assert(suggestions.includes('id="lw-suggest-w1-1"') && suggestions.includes('aria-selected="true"') && suggestions.includes('&lt;x&gt;'));
const state = V.stateView('downloading', {progress: 0.42}, t, true);
assert(state.includes('value="42"') && state.includes('42 %') && !state.includes('data-action="settings"'));
assert(V.stateView('not_installed', {}, t, true).includes('data-action="settings"'));
assert(!V.stateView('not_installed', {}, t, false).includes('data-action="settings"'));
assert(V.stateView('failed', null, t, true).includes('data-action="retry"'));
const banners = V.banners({readable: true, state: 'ready', edition: {}, fulltext: false, update_available: {date: '2026-11'}}, t, 'de', true);
assert(banners.includes('November 2026') && banners.includes(labels['desktop.local_wikipedia_title_only']) && banners.includes('data-action="settings"'));
assert(V.banners({readable: true, state: 'downloading', edition: {}, fulltext: true, progress: 0.5}, t, 'de', true).includes('50 %'));
assert.equal(V.banners({state: 'not_installed'}, t, 'de', true), '');
assert.equal(V.banners({readable: false, state: 'ready', edition: {}, update_available: {date: '2026-11'}}, t, 'de', true), '', 'no banners without a readable edition');
const failed = V.banners({readable: true, state: 'ready', edition: {}, fulltext: true, error_code: 'download_failed'}, t, 'de', true);
assert(failed.includes(labels['desktop.local_wikipedia_update_failed']) && failed.includes('data-action="settings"'), 'a failed update is reported to administrators');
assert.equal(V.banners({readable: true, state: 'ready', edition: {}, fulltext: true, error_code: 'download_failed'}, t, 'de', false), '', 'readers are not bothered with a failed update');
assert.equal(V.banners({readable: true, state: 'ready', edition: {}, fulltext: true, error_code: 'busy'}, t, 'de', true), '', 'other codes are not update failures');
const paused = V.banners({readable: true, state: 'interrupted', edition: {}, fulltext: true, error_code: 'insufficient_disk_space'}, t, 'de', false);
assert(paused.includes(labels['desktop.local_wikipedia_update_interrupted']) && !paused.includes('data-action="settings"'));
assert(V.banners({readable: true, state: 'ready', edition: {}, fulltext: true, error_code: 'x', recommendation: 'English hint'}, t, 'de', true).indexOf('English hint') < 0);
const loading = V.stateView('loading', {loading: true}, t, true);
assert(loading.includes(labels['desktop.local_wikipedia_edition_loading_title']) && loading.includes('lw-spinner') && !loading.includes('data-action='));
const unreadable = V.stateView('state_unreadable', {error_code: 'state_unreadable', recommendation: 'English hint'}, t, true);
assert(unreadable.includes(labels['desktop.local_wikipedia_state_error_admin']) && unreadable.includes('data-action="settings"') && !unreadable.includes('English hint'));
assert(V.stateView('state_unreadable', {}, t, false).includes(labels['desktop.local_wikipedia_state_error_user']));
const installFailed = V.stateView('install_failed', {}, t, true);
assert(installFailed.includes(labels['desktop.local_wikipedia_install_failed_title']) && installFailed.includes('data-action="settings"'));
assert(!V.stateView('install_failed', {}, t, false).includes('data-action="settings"'));
const shell = V.shell(t, {list: 'lw-suggest-w1'});
assert(shell.includes('sandbox="allow-same-origin allow-popups allow-popups-to-escape-sandbox"') && !shell.includes('allow-scripts'));
assert(shell.includes('aria-controls="lw-suggest-w1"') && shell.includes('role="combobox"') && shell.includes('role="toolbar"'));
`
	cmd := exec.Command(node, "-e", script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("view checks: %v\n%s", err, output)
	}
}
