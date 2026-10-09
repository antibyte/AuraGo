// Retro-Net terminal browser fixture (TestDesktopTerminalRetroNetBrowser): the real desktop shell bundle,
// xterm, CRT and Retro-Net modules with a fake WebSocket and a fake API. Served after
// terminalFixturePrelude (ui/desktop_terminal_harness_test.go), which provides fixtureErrors, fixtureTerms
// and the xterm subclass. ?motion=full|reduce selects the motion preference.

const motion = new URLSearchParams(location.search).get('motion') || 'full';

// Page-level override: the Windows animation setting flips headless Chrome's media query otherwise.
const nativeMatchMedia = window.matchMedia.bind(window);
window.matchMedia = query => {
    if (!/prefers-reduced-motion/.test(query)) return nativeMatchMedia(query);
    const matches = motion === 'reduce' && /:\s*reduce\)/.test(query);
    return {
        matches, media: query, onchange: null,
        addEventListener() {}, removeEventListener() {}, addListener() {}, removeListener() {},
        dispatchEvent() { return false; }
    };
};

window.fixtureSockets = [];
window.fixtureCalls = [];
window.fixtureOscillators = 0;
window.fixtureCanEdit = true;
window.fixtureSettingsError = '';
window.fixtureDirectoryStatus = 0;

// Synthesized tones: every oscillator started in the page (key clicks only sound on real keydown events,
// which the modem checks never send).
if (window.AudioContext) {
    const createOscillator = AudioContext.prototype.createOscillator;
    AudioContext.prototype.createOscillator = function (...args) {
        fixtureOscillators++;
        return createOscillator.apply(this, args);
    };
}

// Opens on the next task; a second socket while another is live is recorded as an error.
window.WebSocket = class {
    static CONNECTING = 0; static OPEN = 1; static CLOSING = 2; static CLOSED = 3;
    constructor(url) {
        if (fixtureSockets.some(s => s.readyState !== 3)) fixtureErrors.push('second live socket: ' + url);
        this.url = String(url);
        this.readyState = 0;
        this.sent = [];
        this.binaryType = 'blob';
        this.closedBy = '';
        fixtureSockets.push(this);
        setTimeout(() => {
            if (this.readyState !== 0) return;
            this.readyState = 1;
            this.onopen && this.onopen({});
        }, 0);
    }
    send(value) {
        if (this.readyState !== 1) {
            fixtureErrors.push('send on a socket that is not open: ' + this.url);
            return;
        }
        this.sent.push(value instanceof Uint8Array ? new Uint8Array(value) : value);
    }
    close(code) {
        if (this.readyState === 3) return;
        this.readyState = 3;
        this.closedBy = 'client';
        setTimeout(() => this.onclose && this.onclose({ code: code || 1005, wasClean: true }), 0);
    }
    serverClose(code) {
        if (this.readyState === 3) return;
        this.readyState = 3;
        this.closedBy = 'server';
        this.onclose && this.onclose({ code: code || 1000, wasClean: true });
    }
};

const nativeFetch = window.fetch.bind(window);
const json = (body, status) => Promise.resolve(new Response(JSON.stringify(body), {
    status: status || 200,
    headers: { 'Content-Type': 'application/json' }
}));
const catalogEntry = (id, name, category, protocol, host, port, kind, charset, descriptionId) => Object.assign({
    id, name,
    description_key: 'desktop.terminal_retronet_entry_' + (descriptionId || id).replace(/-/g, '_'),
    category, protocol, host, port, own: false
}, protocol === 'telnet' ? { kind, charset } : { user: 'guest', host_key: 'SHA256:' + 'B'.repeat(43) });

// 00 local shell, 01-02 classics, 03 bbs, 04-34 muds, 35 games, 36-37 own entries.
window.fixtureCatalog = [
    catalogEntry('telehack', 'Telehack', 'classics', 'telnet', 'telehack.com', 23, 'world', 'utf8'),
    catalogEntry('towel', 'Star Wars ASCII', 'classics', 'telnet', 'towel.blinkenlights.nl', 23, 'world', 'utf8'),
    catalogEntry('vertrauen', 'Vertrauen', 'bbs', 'telnet', 'vert.synchro.net', 23, 'bbs', 'cp437'),
    catalogEntry('discworld', 'Discworld MUD', 'muds', 'telnet', 'discworld.atuin.net', 4242, 'world', 'utf8')
];
for (let i = 0; i < 30; i++) {
    fixtureCatalog.push(catalogEntry('fill-' + i, 'Filler MUD ' + i, 'muds', 'telnet', 'aardwolf.org', 4000, 'world', 'utf8', 'aardwolf'));
}
fixtureCatalog.push(catalogEntry('sshtron', 'SSHTron', 'games', 'ssh', 'sshtron.zachlatta.com', 22));
window.fixtureOwn = [
    {
        id: 'own-homeboard001', name: 'Home Board', description: 'My own test board', category: 'own', protocol: 'ssh',
        host: 'bbs.example.org', port: 2222, user: 'guest', host_key: 'SHA256:' + 'A'.repeat(43), own: true
    },
    {
        id: 'own-fresh0000001', name: 'Fresh SSH', description: 'Never contacted', category: 'own', protocol: 'ssh',
        host: 'ssh.example.org', port: 22, user: 'guest', own: true
    }
];
window.fixtureStatus = refreshed => {
    const now = new Date().toISOString();
    return {
        telehack: { state: 'online', checked_at: now },
        towel: { state: 'offline', checked_at: now, last_online_at: new Date(Date.now() - 3 * 86400000).toISOString() },
        vertrauen: { state: 'online', checked_at: now },
        discworld: { state: 'unknown' },
        sshtron: { state: refreshed ? 'online' : 'unknown' }
    };
};

window.fetch = (url, opts) => {
    const options = opts || {};
    const path = String(url).split('?')[0];
    const method = String(options.method || 'GET').toUpperCase();
    if (!path.startsWith('/api/')) return nativeFetch(url, options);
    const call = { path, method, body: options.body ? String(options.body) : '', failed: false };
    fixtureCalls.push(call);
    if (path === '/api/desktop/retronet/directory') {
        if (fixtureDirectoryStatus) {
            call.failed = true;
            return json({ error: 'retro-net is disabled' }, fixtureDirectoryStatus);
        }
        // Entry saves re-read this list before every PUT: it always carries the current own entries.
        return json({
            entries: [...fixtureCatalog, ...fixtureOwn],
            status: fixtureStatus(false),
            stale: true,
            can_edit: fixtureCanEdit
        });
    }
    if (path === '/api/desktop/retronet/status' && method === 'POST') return json({ status: fixtureStatus(true) });
    if (path === '/api/desktop/settings' && method === 'PUT') {
        const request = JSON.parse(call.body);
        if (request.key !== 'retronet.entries') return json({ settings: {} });
        if (fixtureSettingsError) {
            call.failed = true;
            return json({ error: fixtureSettingsError }, 400);
        }
        fixtureOwn = JSON.parse(request.value).entries.map(e => ({ ...e, category: 'own', own: true }));
        return json({ settings: {} });
    }
    return json({});
};

window.fixtureTerm = () => fixtureTerms.at(-1);
window.fixtureRoot = () => [...document.querySelectorAll('.vd-terminal-app')].at(-1);
window.fixtureSocket = () => fixtureSockets.at(-1);
window.fixtureLive = () => fixtureSockets.filter(s => s.readyState !== 3);
// Resolves once xterm has parsed everything written so far.
window.fixtureFlush = () => new Promise(resolve => {
    const term = fixtureTerm();
    if (!term) return resolve();
    term.write('', resolve);
});
// The visible rows of the active buffer, right-trimmed.
window.fixtureLines = () => {
    const term = fixtureTerm();
    if (!term || !term.buffer) return [];
    const buffer = term.buffer.active;
    const out = [];
    for (let y = 0; y < term.rows; y++) {
        const line = buffer.getLine(buffer.viewportY + y);
        out.push(line ? line.translateToString(true) : '');
    }
    return out;
};
window.fixtureText = async () => { await fixtureFlush(); return fixtureLines().join('\n'); };
window.fixtureFlat = async () => { await fixtureFlush(); return fixtureLines().join(''); };
window.fixtureSelectedLine = async () => { await fixtureFlush(); return fixtureLines().find(l => l.startsWith('>')) || ''; };
window.fixtureInput = async data => { fixtureTerm().input(data, true); await fixtureFlush(); };
// Keys typed within ~400 ms of a result are ignored (a service hanging up must not dismiss it unread).
window.fixtureDismiss = async key => { await new Promise(r => setTimeout(r, 450)); await fixtureInput(key); };
window.fixtureControl = message => fixtureSocket().onmessage && fixtureSocket().onmessage({ data: JSON.stringify(message) });
window.fixtureData = text => fixtureSocket().onmessage && fixtureSocket().onmessage({ data: new TextEncoder().encode(text).buffer });
// Frames as 'txt:<json>' (control) or 'bin:<utf-8 text>' (keystrokes).
window.fixtureSent = socket => (socket || fixtureSocket()).sent.map(f => typeof f === 'string' ? 'txt:' + f : 'bin:' + new TextDecoder().decode(f));
// The own-entry documents stored by successful PUTs, oldest first.
window.fixturePuts = () => fixtureCalls
    .filter(c => c.path === '/api/desktop/settings' && c.method === 'PUT' && !c.failed)
    .map(c => JSON.parse(c.body))
    .filter(b => b.key === 'retronet.entries')
    .map(b => JSON.parse(b.value));
window.fixtureDialog = kind => document.querySelector('dialog[data-terminal-retronet-dialog="' + kind + '"]');
window.fixtureForm = (kind, values, submit) => {
    const dialog = fixtureDialog(kind);
    for (const [name, value] of Object.entries(values || {})) {
        const field = dialog.querySelector('[name="' + name + '"]');
        field.value = value;
        field.dispatchEvent(new Event('change', { bubbles: true }));
    }
    if (submit) dialog.querySelector('button[type="submit"]').click();
    return true;
};
window.fixtureDialogError = kind => {
    const dialog = fixtureDialog(kind);
    const error = dialog && dialog.querySelector('[data-retronet-error]');
    return error && !error.hidden ? error.textContent : '';
};
window.fixtureStyle = id => {
    const select = fixtureRoot().querySelector('select[data-terminal-style]');
    select.value = id;
    select.dispatchEvent(new Event('change'));
};
// Closes every window, then opens a fresh Terminal with the given toggle and window context.
window.fixtureBoot = async (retronet, context) => {
    for (const id of [...terminalTest.state.windows.keys()]) terminalTest.closeWindow(id);
    const until = Date.now() + 4000;
    while (terminalTest.state.windows.size && Date.now() < until) await new Promise(r => setTimeout(r, 25));
    terminalTest.state.bootstrap.retronet_enabled = retronet;
    terminalTest.openApp('terminal', context);
    await new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r)));
};

window.fixtureReady = (async () => {
    const words = await (await nativeFetch('/lang/desktop/de.json')).json();
    window.t = (key, params) => {
        let text = words[key] || key;
        if (params) for (const [name, value] of Object.entries(params)) text = text.split('{{' + name + '}}').join(String(value));
        return text;
    };
    window.i18n = { t: window.t };
    const animated = motion === 'full';
    // Subtests share the origin: start every run from the same storage.
    localStorage.clear();
    localStorage.setItem('aurago.desktop.terminal.style', 'amber');
    localStorage.setItem('aurago.desktop.terminal.audioMuted', '1');
    localStorage.setItem('aurago.desktop.terminal.baud', '0');
    localStorage.setItem('aurago.desktop.terminal.retronet.last', 'vertrauen');
    terminalTest.state.bootstrap = {
        enabled: true,
        retronet_enabled: true,
        builtin_apps: [{ id: 'terminal', name: 'Terminal', icon: 'terminal' }],
        apps: [], widgets: [], shortcuts: [], desktop_files: [],
        settings: { 'appearance.theme': 'standard', 'windows.restore_session': false, 'windows.animations': animated }
    };
    document.body.dataset.theme = 'standard';
    document.body.dataset.animations = animated ? 'true' : 'false';
    document.getElementById('vd-disabled').hidden = true;
    await terminalTest.loadIconManifest();
    terminalTest.openApp('terminal');
})();
