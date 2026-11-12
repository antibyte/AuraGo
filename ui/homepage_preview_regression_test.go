package ui

import "testing"

func TestHomepageStudioRejectsUnavailableLocalPreview(t *testing.T) {
	t.Parallel()
	runHomepageStudioNode(t, homepageStudioNodeScript(t, `
const Sites = loadModule(SRC.sites, 'HomepageStudioSites');
const scope = new Function('window', 'state', 'homepageTargets', [
 extractFunction(SRC.main, 'firstString'),
 extractFunction(SRC.main, 'safeExternalURL'),
 extractFunction(SRC.main, 'firstPreviewURL'),
 extractFunction(SRC.main, 'homepageStatusPreviewURL')
].join('\n') + '\nreturn homepageStatusPreviewURL;');
const state = { selectedSiteId: 0, deploymentTargets: [], modules: {} };
const select = scope({ location: { origin: 'https://aurago.test' } }, state, Sites.targets);
const data = { web_container: { running: true }, preview_url: 'https://local.test/', local_preview: { available: false, http_status: 404 } };
assert.equal(select(data, 'local'), '', 'a running container returning an empty 404 is not a preview');
data.local_preview.available = true;
assert.equal(select(data, 'local'), 'https://local.test/');
`))
}

func TestHomepageStudioTargetsStayWithSelectedSiteAndProvider(t *testing.T) {
	t.Parallel()
	runHomepageStudioNode(t, homepageStudioNodeScript(t, `
const Sites = loadModule(SRC.sites, 'HomepageStudioSites');
const sites = [
 { id: 1, deploy_targets: [{ provider: 'vercel', url: 'https://first.test/', status: 'active' }] },
 { id: 2, deploy_targets: [{ provider: 'vercel', url: 'https://second.test/', status: 'active' }] }
];
const state = { selectedSiteId: 2, deploymentTargets: [], targetsLoading: null, modules: { sites: {
 getSites: () => sites, getDetail: id => sites.find(site => site.id === id)
} } };
const scope = new Function('window', 'state', 'api', 'homepageTargets', [
 extractFunction(SRC.main, 'firstString'),
 extractFunction(SRC.main, 'safeExternalURL'),
 extractFunction(SRC.main, 'firstPreviewURL'),
 extractFunction(SRC.main, 'safeHomepageApi'),
 extractFunction(SRC.main, 'loadHomepageTargets')
].join('\n') + '\nreturn loadHomepageTargets;');
const load = scope({ location: { origin: 'https://aurago.test' } }, state, async () => ({}), Sites.targets);
await load();
assert.equal(Sites.targets.external('vercel', state.deploymentTargets), 'https://second.test/', 'another site must never win target selection');
assert.equal(Sites.targets.external('netlify', state.deploymentTargets), '', 'a Vercel URL must never represent Netlify');
const targets = [];
Sites.targets.collect({ deploy_targets: [{ provider: 'vercel', url: 'https://stopped.test/', status: 'inactive' }], deployments: [{ provider: 'vercel', url: 'https://stopped.test/', status: 'ok' }] }, targets, value => value || '');
assert.equal(Sites.targets.external('vercel', targets), '', 'history must not reactivate a stopped target');
let releaseOld;
let calls = 0;
const racingLoad = scope({ location: { origin: 'https://aurago.test' } }, state, async () => {
 if (calls++ === 0) return await new Promise(resolve => { releaseOld = resolve; });
 return {};
}, Sites.targets);
state.selectedSiteId = 1;
const old = racingLoad();
state.selectedSiteId = 2;
await racingLoad();
releaseOld({});
await old;
assert.equal(Sites.targets.external('vercel', state.deploymentTargets), 'https://second.test/', 'a late response must not restore the previous selection');
`))
}

func TestHomepageStudioStatusUsesObservedPreview(t *testing.T) {
	t.Parallel()
	runHomepageStudioNode(t, homepageStudioNodeScript(t, `
const create = new Function('state', 'api', [
 "const t = key => key, statusDot = {}, statusText = {}, statusPill = {};",
 "const setStatus = (kind, text) => { state.view = { kind, text }; };",
 "const updatePreviewUrl = () => {};",
 "const loadHomepageTargets = async () => [];",
 "const homepageStatusPreviewURL = data => data.local_preview && !data.local_preview.available ? '' : (data.vercel_url || '');",
 extractFunction(SRC.main, 'loadStatus'),
 "return loadStatus;"
].join('\n'));
const state = { target: 'local', statusRequest: 0, modules: {} };
const failed = { status: 'ok', web_container: { running: true }, local_preview: { available: false, http_status: 404 } };
let data = failed;
const load = create(state, async () => data);
await load();
assert.equal(state.view.kind, 'offline', 'a successful query and running container must not hide an HTTP error');
assert.ok(state.view.text.includes('404'));
state.target = 'vercel';
data = { status: 'ok', vercel_url: 'https://stored.test/' };
await load();
assert.equal(state.view.kind, 'unknown', 'a stored URL is not proof of reachability');
let releaseOld, calls = 0;
const racingLoad = create(state, async () => calls++ === 0 ? await new Promise(resolve => { releaseOld = resolve; }) : failed);
const old = racingLoad();
state.target = 'local';
await racingLoad();
releaseOld(data);
assert.equal(await old, false);
assert.equal(state.lastStatus, failed, 'late status replies must not replace the current observation');
assert.equal(state.view.kind, 'offline');
`))
}
