window.fixtureErrors = [];
window.addEventListener('error', event => fixtureErrors.push(event.message));
window.addEventListener('unhandledrejection', event => fixtureErrors.push(String(event.reason)));
const jsonReply = body => new Response(body, {headers: {'Content-Type': 'application/json'}});
const nativeFetch = window.fetch.bind(window);
const gevApp = { id: 'store-gods-eye-view', name: "God's Eye View", runtime: 'container-web-app', icon: 'gods-eye-view', metadata: { store_app_id: 'gods-eye-view', open_maximized: 'true', logo_path: '/img/desktop/store/gods-eye-view.svg' } };
window.configPayload = null;
window.fetch = async (url, options = {}) => {
    const path = String(url);
    if (path === '/api/desktop/bootstrap') return jsonReply(JSON.stringify(gevTest.state.bootstrap));
    if (path.endsWith('/open-url')) return jsonReply(JSON.stringify({ url: 'http://127.0.0.1:14173/' }));
    if (path === '/api/desktop/store/catalog') return jsonReply(JSON.stringify({ catalog: [{ id: 'gods-eye-view', name: "God's Eye View", image: 'fixture', description: 'fixture', logo_url: '/img/desktop/store/gods-eye-view.svg', metadata: { description_key: 'desktop.store.gev_description' } }], installed: [{ app_id: 'gods-eye-view', status: 'running' }], docker_available: true, mutations_allowed: true }));
    if (path === '/api/desktop/store/apps') return jsonReply(JSON.stringify({ apps: [{ app_id: 'gods-eye-view', status: 'running' }] }));
    if (path.endsWith('/config')) {
        if (options.method === 'PUT') { window.configPayload = JSON.parse(options.body); return jsonReply(JSON.stringify({ operation: { id: 'fixture-operation', app_id: 'gods-eye-view', type: 'configure', status: 'pending' } })); }
        return jsonReply(JSON.stringify({ configured: { OPENAI_API_KEY: true }, allowed_origins: [location.origin], pending: false }));
    }
    if (path.includes('/operations/')) return jsonReply(JSON.stringify({ operation: { id: 'fixture-operation', type: 'configure', status: 'succeeded' } }));
    if (path.startsWith('/api/')) return jsonReply('{}');
    return nativeFetch(url, options);
};
window.fixtureReady = (async () => {
    const words = await (await nativeFetch('/lang/desktop/de.json')).json();
    document.documentElement.lang = 'de';
    window.i18n = { t: key => words[key] || key, getLanguage: () => 'de' };
    window.t = key => words[key] || key;
    gevTest.state.bootstrap = { enabled: true, builtin_apps: [{ id: 'software-store', name: 'Software Store', icon: 'software-store' }], installed_apps: [gevApp], widgets: [], shortcuts: [], desktop_files: [], settings: { 'appearance.theme': 'standard', 'windows.restore_session': false } };
    document.body.dataset.theme = 'standard';
    document.body.dataset.animations = 'false';
    document.getElementById('vd-disabled').hidden = true;
    await gevTest.loadIconManifest();
    gevTest.openApp('store-gods-eye-view');
})();
