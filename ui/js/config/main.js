// AuraGo – config page logic
// Extracted from config.html

const cfgMaskedSecretFallback = '••••••••';
const CONFIG_ASSET_VERSION = (typeof window !== 'undefined' && window.AURAGO_BUILD_VERSION) ? window.AURAGO_BUILD_VERSION : 'config-dev';
const CONFIG_RECENT_KEY = 'aurago.config.recent.v1';
const CONFIG_ADVANCED_KEY = 'aurago.config.advanced.v1';
const CONFIG_RECENT_LIMIT = 6;

if (typeof window.cfgIsMaskedSecret !== 'function') {
    window.cfgIsMaskedSecret = function (value) {
        return value === cfgMaskedSecretFallback;
    };
}
if (typeof window.cfgSecretValue !== 'function') {
    window.cfgSecretValue = function (value) {
        return window.cfgIsMaskedSecret(value) ? '' : (value || '');
    };
}
if (typeof window.cfgSecretPlaceholder !== 'function') {
    window.cfgSecretPlaceholder = function (value, defaultPlaceholder = cfgMaskedSecretFallback) {
        return window.cfgIsMaskedSecret(value) ? t('config.providers.key_placeholder_existing') : defaultPlaceholder;
    };
}
if (typeof window.cfgMarkSecretStored !== 'function') {
    window.cfgMarkSecretStored = function (input, configPath) {
        if (input) {
            input.value = '';
            input.placeholder = t('config.providers.key_placeholder_existing');
        }
        if (!configPath || typeof setNestedValue !== 'function') return;
        const paths = Array.isArray(configPath) ? configPath : [configPath];
        paths.forEach(path => {
            if (!path) return;
            setNestedValue(configData, path, cfgMaskedSecretFallback);
            if (window.AuraConfigState && typeof window.AuraConfigState.markSaved === 'function') {
                window.AuraConfigState.markSaved(path, cfgMaskedSecretFallback);
            }
        });
    };
}

// Section metadata grouped by logical categories
const SECTIONS = [
    {
        group: t('config.group.agent_ai'),
        items: [
            { key: 'agent', icon: '⚙️', label: t('config.section.agent.label'), desc: t('config.section.agent.desc') },
            { key: 'heartbeat', icon: '💓', label: t('config.section.heartbeat.label'), desc: t('config.section.heartbeat.desc'), customRender: 'renderHeartbeatSection' },
            { key: 'optimizations', icon: '🚀', label: t('config.section.optimizations.label'), desc: t('config.section.optimizations.desc') },
            { key: 'providers', icon: '🔌', label: t('config.section.providers.label'), desc: t('config.section.providers.desc') },
            { key: 'realtime_speech', icon: '〽', label: t('config.section.realtime_speech.label'), desc: t('config.section.realtime_speech.desc') },
            { key: 'manifest', icon: '▦', label: t('config.section.manifest.label'), desc: t('config.section.manifest.desc') },
            { key: 'omniroute', icon: '◎', label: t('config.section.omniroute.label'), desc: t('config.section.omniroute.desc') },
            { key: 'dograh', icon: '▧', label: t('config.section.dograh.label'), desc: t('config.section.dograh.desc') },
            { key: 'llm', icon: '🧠', label: t('config.section.llm.label'), desc: t('config.section.llm.desc') },
            { key: 'fallback_llm', icon: '🔄', label: t('config.section.fallback_llm.label'), desc: t('config.section.fallback_llm.desc') },
            { key: 'embeddings', icon: '🔗', label: t('config.section.embeddings.label'), desc: t('config.section.embeddings.desc') },
            { key: 'budget', icon: '💰', label: t('config.section.budget.label'), desc: t('config.section.budget.desc') },
            { key: 'memory_analysis', icon: '🧬', label: t('config.section.memory_analysis.label'), desc: t('config.section.memory_analysis.desc') },
            { key: 'co_agents', icon: '🤖', label: t('config.section.co_agents.label'), desc: t('config.section.co_agents.desc') },
            { key: 'prompts_editor', icon: '🎭', label: t('config.section.prompts_editor.label'), desc: t('config.section.prompts_editor.desc') },
            { key: 'rules', icon: '📏', label: t('config.section.rules.label'), desc: t('config.section.rules.desc') },
            { key: 'personality', icon: '🧬', label: t('config.section.personality.label'), desc: t('config.section.personality.desc') },
            { key: 'vision', icon: '👁️', label: t('config.section.vision.label'), desc: t('config.section.vision.desc') },
            { key: 'output_compression', icon: '🗜️', label: t('config.section.output_compression.label'), desc: t('config.section.output_compression.desc') }
        ]
    },
    {
        group: t('config.group.server_system'),
        items: [
            { key: 'server', icon: '🌐', label: t('config.section.server.label'), desc: t('config.section.server.desc') },
            { key: 'directories', icon: '📁', label: t('config.section.directories.label'), desc: t('config.section.directories.desc') },
            { key: 'sqlite', icon: '🗄️', label: t('config.section.sqlite.label'), desc: t('config.section.sqlite.desc') },
            { key: 'sql_connections', icon: '🔗', label: t('config.section.sql_connections.label'), desc: t('config.section.sql_connections.desc') },
            { key: 'web_config', icon: '🛡️', label: t('config.section.web_config.label'), desc: t('config.section.web_config.desc') },
            { key: 'logging', icon: '📋', label: t('config.section.logging.label'), desc: t('config.section.logging.desc') },
            { key: 'maintenance', icon: '🔧', label: t('config.section.maintenance.label'), desc: t('config.section.maintenance.desc') },
            { key: 'backup_restore', icon: '💾', label: t('config.section.backup_restore.label'), desc: t('config.section.backup_restore.desc') },
            { key: 'updates', icon: '🔄', label: t('config.section.updates.label'), desc: t('config.section.updates.desc') },
            { key: 'indexing', icon: '📇', label: t('config.section.indexing.label'), desc: t('config.section.indexing.desc') },
            { key: 'firewall', icon: '🧱', label: t('config.section.firewall.label'), desc: t('config.section.firewall.desc') }
        ]
    },
    {
        group: t('config.group.agent_tools'),
        items: [
            { key: 'tools', icon: '🛠️', label: t('config.section.tools.label'), desc: t('config.section.tools.desc') },
            { key: 'web_scraper', icon: '🕷️', label: t('config.section.web_scraper.label'), desc: t('config.section.web_scraper.desc'), customRender: 'renderWebScraperSection' },
            { key: 'browser_automation', icon: '🌐', label: t('config.section.browser_automation.label'), desc: t('config.section.browser_automation.desc') },
            { key: 'space_agent', icon: '🛰️', label: t('config.section.space_agent.label'), desc: t('config.section.space_agent.desc') },
            { key: 'virtual_desktop', icon: '▣', label: t('config.section.virtual_desktop.label'), desc: t('config.section.virtual_desktop.desc') },
            { key: 'virtual_computers', icon: 'VC', label: t('config.section.virtual_computers.label'), desc: t('config.section.virtual_computers.desc') },
            { key: 'sandbox', icon: '📦', label: t('config.section.sandbox.label'), desc: t('config.section.sandbox.desc') },
            { key: 'info_tools', icon: '🔍', label: t('config.section.info_tools.label'), desc: t('config.section.info_tools.desc') },
            { key: 'network_tools', icon: '📡', label: t('config.section.network_tools.label'), desc: t('config.section.network_tools.desc') },
            { key: 'brave_search', icon: '🦁', label: t('config.section.brave_search.label'), desc: t('config.section.brave_search.desc') },
            { key: 'skill_manager', icon: '🧩', label: t('config.section.skill_manager.label'), desc: t('config.section.skill_manager.desc') },
            { key: 'daemon_skills', icon: '👹', label: t('config.section.daemon_skills.label'), desc: t('config.section.daemon_skills.desc') },
            { key: 'mission_preparation', icon: '🎯', label: t('config.section.mission_preparation.label'), desc: t('config.section.mission_preparation.desc') }
        ]
    },
    {
        group: t('config.group.media_content'),
        items: [
            { key: 'whisper', icon: '🎤', label: t('config.section.whisper.label'), desc: t('config.section.whisper.desc') },
            { key: 'tts', icon: '🔊', label: t('config.section.tts.label'), desc: t('config.section.tts.desc') },
            { key: 'image_generation', icon: '🎨', label: t('config.section.image_generation.label'), desc: t('config.section.image_generation.desc') },
            { key: 'music_generation', icon: '🎵', label: t('config.section.music_generation.label'), desc: t('config.section.music_generation.desc') },
            { key: 'video_generation', icon: '🎬', label: t('config.section.video_generation.label'), desc: t('config.section.video_generation.desc') },
            { key: 'media_conversion', icon: '🎞️', label: t('config.section.media_conversion.label'), desc: t('config.section.media_conversion.desc') },
            { key: 'video_download', icon: '▶️', label: t('config.section.video_download.label'), desc: t('config.section.video_download.desc') },
            { key: 'document_creator', icon: '📄', label: t('config.section.document_creator.label'), desc: t('config.section.document_creator.desc') }
        ]
    },
    {
        group: t('config.group.container'),
        items: [
            { key: 'docker', icon: '🐳', label: t('config.section.docker.label'), desc: t('config.section.docker.desc') }
        ]
    },
    {
        group: t('config.group.cloud_storage'),
        items: [
            { key: 's3', icon: '🪣', label: t('config.section.s3.label'), desc: t('config.section.s3.desc') },
            { key: 'webdav', icon: '☁️', label: t('config.section.webdav.label'), desc: t('config.section.webdav.desc') },
            { key: 'koofr', icon: '📦', label: t('config.section.koofr.label'), desc: t('config.section.koofr.desc') }
        ]
    },
    {
        group: t('config.group.web_publishing'),
        items: [
            { key: 'netlify', icon: '🔺', label: t('config.section.netlify.label'), desc: t('config.section.netlify.desc') },
            { key: 'vercel', icon: '▲', label: t('config.section.vercel.label'), desc: t('config.section.vercel.desc') },
            { key: 'cloudflare_tunnel', icon: '🌩️', label: t('config.section.cloudflare_tunnel.label'), desc: t('config.section.cloudflare_tunnel.desc') },
            { key: 'homepage', icon: '🌐', label: t('config.section.homepage.label'), desc: t('config.section.homepage.desc') }
        ]
    },
    {
        group: t('config.group.messenger'),
        items: [
            { key: 'telegram', icon: '📱', label: t('config.section.telegram.label'), desc: t('config.section.telegram.desc') },
            { key: 'discord', icon: '💬', label: t('config.section.discord.label'), desc: t('config.section.discord.desc') },
            { key: 'rocketchat', icon: '🚀', label: t('config.section.rocketchat.label'), desc: t('config.section.rocketchat.desc') },
            { key: 'telnyx', icon: '📞', label: t('config.section.telnyx.label'), desc: t('config.section.telnyx.desc') }
        ]
    },
    {
        group: t('config.group.notifications_group'),
        items: [
            { key: 'email', icon: '✉️', label: t('config.section.email.label'), desc: t('config.section.email.desc') },
            { key: 'agentmail', icon: '@', label: t('config.section.agentmail.label'), desc: t('config.section.agentmail.desc') },
            { key: 'webhooks', icon: '🔗', label: t('config.section.webhooks.label'), desc: t('config.section.webhooks.desc') },
            { key: 'notifications', icon: '🔔', label: t('config.section.notifications.label'), desc: t('config.section.notifications.desc') }
        ]
    },
    {
        group: t('config.group.productivity'),
        items: [
            { key: 'github', icon: '🐙', label: t('config.section.github.label'), desc: t('config.section.github.desc') },
            { key: 'google_workspace', icon: '📊', label: t('config.section.google_workspace.label'), desc: t('config.section.google_workspace.desc') },
            { key: 'paperless_ngx', icon: '📄', label: t('config.section.paperless_ngx.label'), desc: t('config.section.paperless_ngx.desc') },
            { key: 'obsidian', icon: '📝', label: t('config.section.obsidian.label'), desc: t('config.section.obsidian.desc') },
            { key: 'yepapi', icon: '🌐', label: t('config.section.yepapi.label'), desc: t('config.section.yepapi.desc') }
        ]
    },
    {
        group: t('config.group.smart_home'),
        items: [
            { key: 'home_assistant', icon: '🏠', label: t('config.section.home_assistant.label'), desc: t('config.section.home_assistant.desc') },
            { key: 'mqtt', icon: '📡', label: t('config.section.mqtt.label'), desc: t('config.section.mqtt.desc') },
            { key: 'chromecast', icon: '📺', label: t('config.section.chromecast.label'), desc: t('config.section.chromecast.desc') },
            { key: 'bluetooth', icon: 'ᛒ', label: t('config.section.bluetooth.label'), desc: t('config.section.bluetooth.desc') },
            { key: 'adguard', icon: '🛡️', label: t('config.section.adguard.label'), desc: t('config.section.adguard.desc') },
            { key: 'fritzbox', icon: '📡', label: t('config.section.fritzbox.label'), desc: t('config.section.fritzbox.desc') },
            { key: 'ldap', icon: '📇', label: t('config.section.ldap.label'), desc: t('config.section.ldap.desc') }
        ]
    },
    {
        group: t('config.group.network_remote'),
        items: [
            { key: 'truenas', icon: '💾', label: t('config.section.truenas.label'), desc: t('config.section.truenas.desc') },
            { key: 'uptime_kuma', icon: '📈', label: t('config.section.uptime_kuma.label'), desc: t('config.section.uptime_kuma.desc') },
            { key: 'jellyfin', icon: '🎬', label: t('config.section.jellyfin.label'), desc: t('config.section.jellyfin.desc') },
            { key: 'tailscale', icon: '🔒', label: t('config.section.tailscale.label'), desc: t('config.section.tailscale.desc') },
            { key: 'proxmox', icon: '🖥️', label: t('config.section.proxmox.label'), desc: t('config.section.proxmox.desc') },
            { key: 'frigate', icon: '📹', label: t('config.section.frigate.label'), desc: t('config.section.frigate.desc') },
            { key: 'three_d_printers', icon: '🖨️', label: t('config.section.three_d_printers.label'), desc: t('config.section.three_d_printers.desc') },
            { key: 'remote_control', icon: '📡', label: t('config.section.remote_control.label'), desc: t('config.section.remote_control.desc') },
            { key: 'grafana', icon: '📊', label: t('config.section.grafana.label'), desc: t('config.section.grafana.desc') },
            { key: 'meshcentral', icon: '🖥️', label: t('config.section.meshcentral.label'), desc: t('config.section.meshcentral.desc') },
            { key: 'ansible', icon: '⚙️', label: t('config.section.ansible.label'), desc: t('config.section.ansible.desc') }
        ]
    },
    {
        group: t('config.group.security'),
        items: [
            { key: 'security_proxy', icon: '🔒', label: t('config.section.security_proxy.label'), desc: t('config.section.security_proxy.desc') },
            { key: 'guardian', icon: '🛡️', label: t('config.section.guardian.label'), desc: t('config.section.guardian.desc') },
            { key: 'llm_guardian', icon: '🤖', label: t('config.section.llm_guardian.label'), desc: t('config.section.llm_guardian.desc') },
            { key: 'virustotal', icon: '🦠', label: t('config.section.virustotal.label'), desc: t('config.section.virustotal.desc') }
        ]
    },
    {
        group: t('config.group.external_ai'),
        items: [
            { key: 'ai_gateway', icon: '🌩️', label: t('config.section.ai_gateway.label'), desc: t('config.section.ai_gateway.desc') },
            { key: 'composio', icon: '◇', label: t('config.section.composio.label'), desc: t('config.section.composio.desc') },
            { key: 'manus', icon: 'M', label: t('config.section.manus.label'), desc: t('config.section.manus.desc') },
            { key: 'huggingface', icon: 'HF', label: t('config.section.huggingface.label'), desc: t('config.section.huggingface.desc') },
            { key: 'evomap', icon: '◇', label: t('config.section.evomap.label'), desc: t('config.section.evomap.desc') },
            { key: 'mcp', icon: '🔌', label: t('config.section.mcp.label'), desc: t('config.section.mcp.desc') },
            { key: 'mcp_server', icon: '🔗', label: t('config.section.mcp_server.label'), desc: t('config.section.mcp_server.desc') },
            { key: 'a2a', icon: '🔀', label: t('config.section.a2a.label'), desc: t('config.section.a2a.desc') },
            { key: 'ollama', icon: '🦙', label: t('config.section.ollama.label'), desc: t('config.section.ollama.desc') }
        ]
    },
    {
        group: t('config.group.danger_zone'),
        dangerGroup: true,
        items: [
            { key: 'danger_zone', icon: '☠️', label: t('config.section.danger_zone.label'), desc: t('config.section.danger_zone.desc') }
        ]
    }
];

const lang = (typeof SYSTEM_LANG === 'string' && SYSTEM_LANG.trim())
    ? SYSTEM_LANG.trim().toLowerCase().split('-')[0]
    : 'en';
let configData = {};
// helpTexts built from I18N — no separate fetch needed
const helpTexts = new Proxy({}, {
    get(_, key) {
        const txt = I18N['help.' + key];
        const meta = I18N_META['help.' + key];
        if (!txt && !meta) return undefined;
        const obj = {};
        if (txt) obj[lang] = txt;
        if (meta) Object.assign(obj, meta);
        if (txt && !obj.en) obj.en = txt;
        return obj;
    }
});
let schema = [];
let activeSection = (window.location.hash || '').replace(/^#/, '') || localStorage.getItem('aurago-cfg-section') || 'overview';
let isDirty = false;
let configSaveInFlight = false;
let restartInFlight = false;
let initialSnapshot = '';
let suppressDirtyTracking = false;
let userEditedSinceSnapshot = false;
let configEditIntentUntil = 0;
let dirtyBaselineRefreshTimers = [];
let configEditIntentTrackingInstalled = false;
let vaultExists = false;
const CONFIG_EDIT_INTENT_WINDOW_MS = 2000;
const DIRTY_BASELINE_REFRESH_DELAY_MS = 160;
const DIRTY_BASELINE_SETTLE_DELAYS_MS = [120, 600, 1600, 3600, 8000];
const SENSITIVE_KEYS = ['api_key', 'bot_token', 'password', 'app_password', 'access_token', 'token', 'user_key', 'app_token', 'secret', 'master_key'];


function applyConfigDensity(value, persist = false) {
    const workspace = window.AuraPrecisionWorkspace;
    if (!workspace) return document.body.dataset.density || 'comfortable';
    workspace.init();
    if (persist) return workspace.setDensity(value);
    return workspace.getDensity();
}

applyConfigDensity();

function hasVisibleSection(key) {
    if (key === 'overview') return true;
    return SECTIONS.some(group => group.items.some(item => item.key === key));
}

function handleConfigRedirectResponse(resp) {
    if (!resp) return false;
    const redirectedTo = resp.redirected ? (resp.url || '') : '';
    if (redirectedTo.includes('/setup')) {
        window.location.replace('/setup');
        return true;
    }
    if (redirectedTo.includes('/auth/login')) {
        const next = encodeURIComponent(window.location.pathname + window.location.search);
        window.location.replace('/auth/login?redirect=' + next);
        return true;
    }
    return false;
}

// Provider management state (loaded from /api/providers)
let providersCache = [];
let providersLoaded = false;
let providersLoadError = '';
let providersLoadRedirected = false;

async function loadProviders() {
    providersLoadRedirected = false;
    try {
        const provResp = await fetch('/api/providers');
        if (handleConfigRedirectResponse(provResp)) {
            providersLoadRedirected = true;
            return false;
        }
        if (!provResp.ok) {
            const message = await provResp.text();
            throw new Error(message || t('config.common.error'));
        }
        providersCache = await provResp.json();
        providersLoaded = true;
        providersLoadError = '';
        return true;
    } catch (e) {
        providersLoadError = e && e.message ? e.message : String(e);
        if (!providersLoaded) providersCache = [];
        return false;
    }
}

// Personality profiles cache (loaded from /api/personalities)
let personalitiesCache = [];

// Runtime environment detection (loaded from /api/runtime)
let runtimeData = { runtime: {}, features: {} };
const SECTION_FEATURE_MAP = { docker: 'docker', invasion_control: 'invasion_local', updates: 'updates' };
const NON_BLOCKING_UNAVAILABLE_SECTIONS = new Set(['docker']);

// Init
async function init() {
    try {
        const [cfgResp, schemaResp, vaultResp] = await Promise.all([
            fetch('/api/config'),
            fetch('/api/config/schema'),
            fetch('/api/vault/status')
        ]);
        if (handleConfigRedirectResponse(cfgResp) || handleConfigRedirectResponse(schemaResp) || handleConfigRedirectResponse(vaultResp)) {
            return;
        }
        configData = await cfgResp.json();
        if (window.AuraConfigState) {
            window.AuraConfigState.init(configData);
            window.AuraConfigState.bind(document);
            window.AuraConfigState.subscribe(state => setDirty(state.dirty));
        }
        schema = await schemaResp.json();
        if (window.AuraConfigState) window.AuraConfigState.setRules(configValidationRules());
        try { vaultExists = (await vaultResp.json()).exists === true; } catch (_) { }
        // Load providers (best-effort – endpoint only exists when web_config is enabled)
        await loadProviders();
        if (providersLoadRedirected) return;
        // Load personality profiles
        try {
            const persResp = await fetch('/api/personalities');
            if (handleConfigRedirectResponse(persResp)) return;
            if (persResp.ok) { const d = await persResp.json(); personalitiesCache = d.personalities || []; }
        } catch (_) { }
        // Load runtime environment capabilities (Docker mode, socket, broadcast, etc.)
        try {
            const rtResp = await fetch('/api/runtime');
            if (handleConfigRedirectResponse(rtResp)) return;
            if (rtResp.ok) runtimeData = await rtResp.json();
        } catch (_) { }
    } catch (e) {
        document.getElementById('content').innerHTML = '<div class="cfg-error-state cfg-error-state-lg">❌ ' + escapeHtml(t('config.loading_error')) + '<br><small>' + escapeHtml(e && e.message ? e.message : String(e)) + '</small></div>';
        return;
    }
    if (!hasVisibleSection(activeSection)) {
        activeSection = 'overview';
        localStorage.setItem('aurago-cfg-section', activeSection);
    }
    installConfigEditIntentTracking();
    buildSidebar();
    await selectSection(activeSection, { scrollBehavior: 'auto' });
    resetDirtySnapshot();
}

/* ── Sidebar drawer (mobile) ── */
const cfgHamburger = document.getElementById('cfg-hamburger');
const sidebarEl = document.getElementById('sidebar');
const sidebarBackdrop = document.getElementById('sidebar-backdrop');

function openSidebar() {
    sidebarEl.classList.add('open');
    sidebarBackdrop.classList.add('open');
    cfgHamburger.textContent = '✕';
}

function closeSidebar() {
    sidebarEl.classList.remove('open');
    sidebarBackdrop.classList.remove('open');
    cfgHamburger.textContent = '☰';
}

cfgHamburger.addEventListener('click', () => {
    if (sidebarEl.classList.contains('open')) {
        closeSidebar();
    } else {
        openSidebar();
    }
});

sidebarBackdrop.addEventListener('click', closeSidebar);

// ── Group collapse state (persisted in localStorage, keyed by group name) ──
const COLLAPSED_GROUPS_KEY = 'aurago-cfg-groups-collapsed';

function loadCollapsedGroups() {
    const allGroups = SECTIONS.map(g => g.group);
    try {
        const raw = localStorage.getItem(COLLAPSED_GROUPS_KEY);
        if (raw !== null) {
            return new Set(JSON.parse(raw));
        }
        // First visit: collapse ALL groups by default
        return new Set(allGroups);
    } catch (_) {
        return new Set(allGroups);
    }
}

function saveCollapsedGroups() {
    localStorage.setItem(COLLAPSED_GROUPS_KEY, JSON.stringify([...collapsedGroups]));
}

const collapsedGroups = loadCollapsedGroups();
let sidebarSearchQuery = '';
let sidebarSearchSnapshot = null;
let sidebarSearchFocusedIndex = -1;
let sidebarSearchDebounceTimer = null;

function scrollActiveSidebarItemIntoView(behavior = 'smooth', delay = 0) {
    const scrollFn = () => {
        const activeItem = document.querySelector('.sidebar-item.active');
        if (!activeItem) return;
        activeItem.scrollIntoView({
            block: 'nearest',
            inline: 'nearest',
            behavior
        });
    };
    if (delay > 0) {
        setTimeout(scrollFn, delay);
        return;
    }
    requestAnimationFrame(scrollFn);
}

const CONFIG_SIDEBAR_ICON_GRID = Object.freeze({ columns: 11, rows: 10, cell: 128 });
const CONFIG_SIDEBAR_ICON_SLOTS = Object.freeze({
    overview: 0,
    agent: 1,
    heartbeat: 2,
    optimizations: 3,
    providers: 4,
    manifest: 5,
    omniroute: 6,
    dograh: 7,
    llm: 8,
    fallback_llm: 9,
    embeddings: 10,
    budget: 11,
    memory_analysis: 12,
    co_agents: 13,
    prompts_editor: 14,
    rules: 15,
    personality: 16,
    vision: 17,
    output_compression: 18,
    server: 19,
    directories: 20,
    sqlite: 21,
    sql_connections: 22,
    web_config: 23,
    logging: 24,
    maintenance: 25,
    backup_restore: 26,
    updates: 27,
    indexing: 28,
    firewall: 29,
    tools: 30,
    web_scraper: 31,
    browser_automation: 32,
    space_agent: 33,
    virtual_desktop: 34,
    virtual_computers: 35,
    sandbox: 36,
    info_tools: 37,
    network_tools: 38,
    brave_search: 39,
    skill_manager: 40,
    daemon_skills: 41,
    mission_preparation: 42,
    whisper: 43,
    tts: 44,
    image_generation: 45,
    music_generation: 46,
    video_generation: 47,
    media_conversion: 48,
    video_download: 49,
    document_creator: 50,
    docker: 51,
    s3: 52,
    webdav: 53,
    koofr: 54,
    netlify: 55,
    vercel: 56,
    cloudflare_tunnel: 57,
    homepage: 58,
    telegram: 59,
    discord: 60,
    rocketchat: 61,
    telnyx: 62,
    email: 63,
    agentmail: 64,
    webhooks: 65,
    notifications: 66,
    github: 67,
    google_workspace: 68,
    paperless_ngx: 69,
    obsidian: 70,
    yepapi: 71,
    home_assistant: 72,
    mqtt: 73,
    chromecast: 74,
    bluetooth: 104,
    adguard: 75,
    fritzbox: 76,
    ldap: 77,
    truenas: 78,
    uptime_kuma: 79,
    jellyfin: 80,
    tailscale: 81,
    proxmox: 82,
    frigate: 83,
    three_d_printers: 84,
    remote_control: 85,
    grafana: 86,
    meshcentral: 87,
    ansible: 88,
    security_proxy: 89,
    guardian: 90,
    llm_guardian: 91,
    virustotal: 92,
    ai_gateway: 93,
    composio: 94,
    huggingface: 95,
    evomap: 96,
    mcp: 97,
    mcp_server: 98,
    a2a: 99,
    ollama: 100,
    danger_zone: 101,
    manus: 102,
    realtime_speech: 103,
});

const CONFIG_SIDEBAR_ICON_SYMBOL_PREFIX = 'config-sidebar-icon-';
const CONFIG_SIDEBAR_ICON_SYMBOLS = Object.freeze({
    overview: "<g fill=\"#7da3c8\"><rect x=\"28\" y=\"28\" width=\"28\" height=\"28\" rx=\"7\"/><rect x=\"72\" y=\"28\" width=\"28\" height=\"28\" rx=\"7\" opacity=\".72\"/><rect x=\"28\" y=\"72\" width=\"28\" height=\"28\" rx=\"7\" opacity=\".72\"/><rect x=\"72\" y=\"72\" width=\"28\" height=\"28\" rx=\"7\"/></g><path d=\"M43 43h42M43 85h42\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#35c7d3\"/>",
    agent: "<circle cx=\"64\" cy=\"64\" r=\"23\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#7da3c8\"/><path d=\"M64 23v13M64 92v13M23 64h13M92 64h13M35 35l9 9M84 84l9 9M93 35l-9 9M44 84l-9 9\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#35c7d3\"/><circle cx=\"64\" cy=\"64\" r=\"8\" fill=\"#35c7d3\"/><text x=\"64\" y=\"64\" text-anchor=\"middle\" dominant-baseline=\"middle\" font-family=\"Geist, Inter, Segoe UI, Arial, sans-serif\" font-size=\"18\" font-weight=\"800\" fill=\"#7da3c8\">A</text>",
    heartbeat: "<path d=\"M30 67c-11-15 8-34 24-19l10 10 10-10c16-15 35 4 24 19-7 11-21 22-34 34-13-12-27-23-34-34z\" fill=\"#ef6f78\" opacity=\".2\"/><path d=\"M26 68h19l8-18 15 36 9-18h25\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#35c7d3\"/>",
    optimizations: "<path d=\"M51 86 42 98l18-5 26-26c9-9 13-24 9-37-13-4-28 0-37 9L32 65l-5 18 12-9z\" fill=\"#4f8ee8\" opacity=\".2\"/><path d=\"M39 74 58 39c9-9 24-13 37-9 4 13 0 28-9 37L51 86M45 88l-10 10M76 43l9 9\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#4f8ee8\"/><circle cx=\"76\" cy=\"52\" r=\"8\" fill=\"#35c7d3\"/>",
    providers: "<path d=\"M48 29v24M80 29v24M42 53h44v15c0 12-10 22-22 22S42 80 42 68zM64 90v14\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#27b6a9\"/><path d=\"M48 68h32\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#7da3c8\"/>",
    realtime_speech: "<path d=\"M24 64h12m8-18v36m10-52v68m10-48v28m10-44v60m10-72v84m10-50v36m8-18h12\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#35c7d3\"/><circle cx=\"64\" cy=\"64\" r=\"38\" fill=\"none\" stroke=\"#7da3c8\" stroke-width=\"4\" opacity=\".3\"/>",
    manifest: "<g fill=\"#7b8ff0\"><rect x=\"28\" y=\"28\" width=\"26\" height=\"26\" rx=\"6\"/><rect x=\"59\" y=\"28\" width=\"41\" height=\"26\" rx=\"6\" opacity=\".75\"/><rect x=\"28\" y=\"59\" width=\"41\" height=\"41\" rx=\"6\" opacity=\".75\"/><rect x=\"74\" y=\"59\" width=\"26\" height=\"41\" rx=\"6\"/></g><path d=\"M41 41h.1M87 87h.1\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#35c7d3\"/>",
    omniroute: "<circle cx=\"36\" cy=\"39\" r=\"10\" fill=\"#35c7d3\"/><circle cx=\"92\" cy=\"89\" r=\"10\" fill=\"#7b8ff0\"/><path d=\"M46 39h15c15 0 17 20 2 20H51c-17 0-20 30 8 30h23\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#35c7d3\"/>",
    dograh: "<path d=\"M31 40h24l13 24-13 24H31L18 64zM73 30h24l13 24-13 24H73L60 54z\" fill=\"#9b7cf0\" opacity=\".2\"/><path d=\"M43 64h36M70 50l14 14-14 14\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#35c7d3\"/><text x=\"64\" y=\"65\" text-anchor=\"middle\" dominant-baseline=\"middle\" font-family=\"Geist, Inter, Segoe UI, Arial, sans-serif\" font-size=\"19\" font-weight=\"800\" fill=\"#9b7cf0\">D</text>",
    llm: "<path d=\"M48 35c-13 2-19 15-13 27-9 7-7 24 7 29 6 12 22 10 26 0 8 9 25 3 25-11 12-7 10-25-1-30 2-14-14-25-26-17-5-6-13-7-18 2z\" fill=\"#9b7cf0\" opacity=\".18\"/><path d=\"M52 42c-9 5-10 17-2 24M74 39c8 6 9 18 0 25M48 84c8 5 18 2 22-6M73 78c7 6 17 6 23-1\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#9b7cf0\"/><circle cx=\"64\" cy=\"64\" r=\"7\" fill=\"#35c7d3\"/>",
    fallback_llm: "<path d=\"M91 51A30 30 0 0 0 37 43L27 53M37 43v21M37 77a30 30 0 0 0 54 8l10-10M91 85V64\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#4f8ee8\"/><circle cx=\"64\" cy=\"64\" r=\"8\" fill=\"#35c7d3\"/>",
    embeddings: "<circle cx=\"37\" cy=\"43\" r=\"10\" fill=\"#35c7d3\"/><circle cx=\"91\" cy=\"43\" r=\"10\" fill=\"#7da3c8\"/><circle cx=\"64\" cy=\"90\" r=\"10\" fill=\"#35c7d3\"/><path d=\"M47 43h34M42 52l17 29M86 52 69 81\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#9eb3c9\"/>",
    budget: "<rect x=\"27\" y=\"42\" width=\"74\" height=\"48\" rx=\"12\" fill=\"#6fca8f\" opacity=\".24\"/><path d=\"M29 45h67c6 0 10 4 10 10v29H29c-5 0-9-4-9-9V54c0-5 4-9 9-9z\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#6fca8f\"/><circle cx=\"88\" cy=\"66\" r=\"6\" fill=\"#f6cf58\"/>",
    memory_analysis: "<path d=\"M43 28c42 22 42 50 0 72M85 28c-42 22-42 50 0 72M48 44h32M44 64h40M48 84h32\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#9b7cf0\"/><circle cx=\"64\" cy=\"64\" r=\"6\" fill=\"#35c7d3\"/>",
    co_agents: "<rect x=\"35\" y=\"42\" width=\"58\" height=\"46\" rx=\"13\" fill=\"#7da3c8\" opacity=\".22\"/><path d=\"M64 42V27M47 57h.1M81 57h.1M50 75h28\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#7da3c8\"/><rect x=\"35\" y=\"42\" width=\"58\" height=\"46\" rx=\"13\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#7da3c8\"/><circle cx=\"64\" cy=\"27\" r=\"5\" fill=\"#35c7d3\"/>",
    prompts_editor: "<path d=\"M35 44c18-10 40-10 58 0v22c0 19-12 34-29 39-17-5-29-20-29-39z\" fill=\"#9b7cf0\" opacity=\".2\"/><path d=\"M46 63c8-6 17-6 26 0M76 63c8-6 17-6 26 0M52 82c10 8 24 8 34 0\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#9b7cf0\"/>",
    rules: "<path d=\"M31 91 91 31l14 14-60 60z\" fill=\"#7da3c8\" opacity=\".2\"/><path d=\"M31 91 91 31l14 14-60 60zM57 79l-8-8M69 67l-8-8M81 55l-8-8\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#7da3c8\"/>",
    personality: "<path d=\"M43 28c42 22 42 50 0 72M85 28c-42 22-42 50 0 72M48 44h32M44 64h40M48 84h32\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#9b7cf0\"/><circle cx=\"64\" cy=\"64\" r=\"6\" fill=\"#ef6f78\"/><text x=\"64\" y=\"64\" text-anchor=\"middle\" dominant-baseline=\"middle\" font-family=\"Geist, Inter, Segoe UI, Arial, sans-serif\" font-size=\"17\" font-weight=\"800\" fill=\"#ef6f78\">P</text>",
    vision: "<path d=\"M24 64s15-27 40-27 40 27 40 27-15 27-40 27-40-27-40-27z\" fill=\"#35c7d3\" opacity=\".18\"/><path d=\"M24 64s15-27 40-27 40 27 40 27-15 27-40 27-40-27-40-27z\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#35c7d3\"/><circle cx=\"64\" cy=\"64\" r=\"13\" fill=\"#4f8ee8\"/>",
    output_compression: "<path d=\"M31 42h26V28M97 86H71v14M57 28 31 54M71 100l26-26M86 42h-18M60 86H42\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#7da3c8\"/>",
    server: "<rect x=\"31\" y=\"28\" width=\"66\" height=\"72\" rx=\"10\" fill=\"#7da3c8\" opacity=\".16\"/><path d=\"M31 38h66M31 64h66M31 90h66M45 51h.1M45 77h.1\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#7da3c8\"/><path d=\"M78 51h10M78 77h10\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#35c7d3\"/>",
    directories: "<path d=\"M23 44h33l10 11h39v40c0 6-4 10-10 10H33c-6 0-10-4-10-10z\" fill=\"#f6cf58\" opacity=\".24\"/><path d=\"M23 44h33l10 11h39v40c0 6-4 10-10 10H33c-6 0-10-4-10-10z\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#f6cf58\"/>",
    sqlite: "<ellipse cx=\"64\" cy=\"35\" rx=\"34\" ry=\"13\" fill=\"#4f8ee8\" opacity=\".22\"/><path d=\"M30 35v48c0 7 15 13 34 13s34-6 34-13V35M30 59c0 7 15 13 34 13s34-6 34-13M30 83c0 7 15 13 34 13s34-6 34-13\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#4f8ee8\"/>",
    sql_connections: "<ellipse cx=\"42\" cy=\"43\" rx=\"21\" ry=\"9\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#4f8ee8\"/><path d=\"M21 43v35c0 5 9 9 21 9s21-4 21-9V43M65 64h20M79 52l14 12-14 12\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#4f8ee8\"/><text x=\"64\" y=\"98\" text-anchor=\"middle\" dominant-baseline=\"middle\" font-family=\"Geist, Inter, Segoe UI, Arial, sans-serif\" font-size=\"20\" font-weight=\"800\" fill=\"#6fca8f\">SQL</text>",
    web_config: "<path d=\"M64 25 31 39v25c0 22 13 36 33 45 20-9 33-23 33-45V39z\" fill=\"#35c7d3\" opacity=\".2\"/><path d=\"M64 25 31 39v25c0 22 13 36 33 45 20-9 33-23 33-45V39z\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#35c7d3\"/>",
    logging: "<path d=\"M38 25h35l19 19v59H38z\" fill=\"#7da3c8\" opacity=\".16\"/><path d=\"M38 25h35l19 19v59H38zM73 25v20h19M50 58h28M50 74h28M50 90h18\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#7da3c8\"/>",
    maintenance: "<path d=\"M82 28a22 22 0 0 0-25 29L27 87l14 14 30-30a22 22 0 0 0 29-25L86 60 68 42z\" fill=\"#7da3c8\" opacity=\".2\"/><path d=\"M82 28a22 22 0 0 0-25 29L27 87l14 14 30-30a22 22 0 0 0 29-25L86 60 68 42z\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#7da3c8\"/>",
    backup_restore: "<path d=\"M33 26h49l13 13v63H33z\" fill=\"#4f8ee8\" opacity=\".18\"/><path d=\"M33 26h49l13 13v63H33zM48 26v29h29V26M49 82h30\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#4f8ee8\"/>",
    updates: "<path d=\"M91 48A30 30 0 0 0 38 41M38 41h22M38 41v22M37 80a30 30 0 0 0 53 7M90 87H68M90 87V65\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#35c7d3\"/>",
    indexing: "<rect x=\"29\" y=\"34\" width=\"48\" height=\"58\" rx=\"8\" fill=\"#7da3c8\" opacity=\".18\"/><rect x=\"50\" y=\"27\" width=\"48\" height=\"58\" rx=\"8\" fill=\"#35c7d3\" opacity=\".22\"/><path d=\"M43 53h20M43 69h24M64 46h20M64 62h18\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#7da3c8\"/>",
    firewall: "<path d=\"M25 38h78v52H25zM25 55h78M25 72h78M44 38v17M68 38v17M56 55v17M80 55v17M44 72v18M68 72v18\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#ef6f78\"/>",
    tools: "<path d=\"M37 38h40M57 38v52M40 90h34M81 51l18-18M91 31l10 10M86 56l12 12\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#7da3c8\"/><circle cx=\"57\" cy=\"90\" r=\"10\" fill=\"#35c7d3\"/>",
    web_scraper: "<rect x=\"25\" y=\"33\" width=\"78\" height=\"62\" rx=\"10\" fill=\"#6fca8f\" opacity=\".16\"/><path d=\"M25 50h78M42 42h.1M57 42h.1M35 76h26M72 66l20 10-20 10z\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#6fca8f\"/><text x=\"64\" y=\"76\" text-anchor=\"middle\" dominant-baseline=\"middle\" font-family=\"Geist, Inter, Segoe UI, Arial, sans-serif\" font-size=\"20\" font-weight=\"800\" fill=\"#35c7d3\">W</text>",
    browser_automation: "<rect x=\"25\" y=\"33\" width=\"78\" height=\"62\" rx=\"10\" fill=\"#4f8ee8\" opacity=\".16\"/><path d=\"M25 50h78M42 42h.1M57 42h.1M35 76h26M72 66l20 10-20 10z\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#4f8ee8\"/>",
    space_agent: "<path d=\"M55 55 73 73M46 46l-17-8 8 17zM82 82l17 8-8-17zM57 31l40 40M31 57l40 40\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#4f8ee8\"/><circle cx=\"64\" cy=\"64\" r=\"9\" fill=\"#f6cf58\"/>",
    virtual_desktop: "<rect x=\"25\" y=\"31\" width=\"78\" height=\"52\" rx=\"8\" fill=\"#7da3c8\" opacity=\".18\"/><path d=\"M25 31h78v52H25zM53 101h22M64 83v18\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#7da3c8\"/>",
    virtual_computers: "<rect x=\"24\" y=\"28\" width=\"80\" height=\"52\" rx=\"9\" fill=\"#7da3c8\" opacity=\".18\"/><path d=\"M24 28h80v52H24zM44 96h40M64 80v16M43 52l10 10-10 10M61 72h24\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#7da3c8\"/><circle cx=\"92\" cy=\"36\" r=\"10\" fill=\"#35c7d3\"/>",
    sandbox: "<path d=\"m64 25 36 20v38l-36 20-36-20V45z\" fill=\"#27b6a9\" opacity=\".2\"/><path d=\"m64 25 36 20v38l-36 20-36-20V45zM28 45l36 20 36-20M64 65v38\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#27b6a9\"/>",
    info_tools: "<circle cx=\"56\" cy=\"56\" r=\"25\" fill=\"#35c7d3\" opacity=\".18\"/><path d=\"M74 74 98 98M56 31a25 25 0 1 0 0 50 25 25 0 0 0 0-50z\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#35c7d3\"/>",
    network_tools: "<path d=\"M64 87V59M42 37a32 32 0 0 1 44 0M31 25a48 48 0 0 1 66 0M53 49a16 16 0 0 1 22 0\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#4f8ee8\"/><circle cx=\"64\" cy=\"91\" r=\"10\" fill=\"#35c7d3\"/>",
    brave_search: "<circle cx=\"56\" cy=\"56\" r=\"25\" fill=\"#f28b54\" opacity=\".18\"/><path d=\"M74 74 98 98M56 31a25 25 0 1 0 0 50 25 25 0 0 0 0-50z\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#f28b54\"/><text x=\"64\" y=\"56\" text-anchor=\"middle\" dominant-baseline=\"middle\" font-family=\"Geist, Inter, Segoe UI, Arial, sans-serif\" font-size=\"22\" font-weight=\"800\" fill=\"#9b7cf0\">B</text>",
    skill_manager: "<path d=\"M48 31h32v21c8-8 22-1 18 10-2 7-11 9-18 3v32H49V75c-8 8-22 1-18-10 2-7 11-9 18-3z\" fill=\"#9b7cf0\" opacity=\".22\"/><path d=\"M48 31h32v21c8-8 22-1 18 10-2 7-11 9-18 3v32H49V75c-8 8-22 1-18-10 2-7 11-9 18-3z\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#9b7cf0\"/>",
    daemon_skills: "<rect x=\"27\" y=\"35\" width=\"74\" height=\"58\" rx=\"10\" fill=\"#7da3c8\" opacity=\".18\"/><path d=\"m43 55 13 10-13 10M62 78h26\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#7da3c8\"/><text x=\"64\" y=\"48\" text-anchor=\"middle\" dominant-baseline=\"middle\" font-family=\"Geist, Inter, Segoe UI, Arial, sans-serif\" font-size=\"18\" font-weight=\"800\" fill=\"#35c7d3\">DS</text>",
    mission_preparation: "<circle cx=\"64\" cy=\"64\" r=\"38\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#ef6f78\"/><circle cx=\"64\" cy=\"64\" r=\"22\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#35c7d3\"/><circle cx=\"64\" cy=\"64\" r=\"8\" fill=\"#ef6f78\"/>",
    whisper: "<rect x=\"50\" y=\"25\" width=\"28\" height=\"48\" rx=\"14\" fill=\"#35c7d3\" opacity=\".2\"/><path d=\"M50 49v10a14 14 0 0 0 28 0V49M36 57a28 28 0 0 0 56 0M64 85v19M50 104h28\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#35c7d3\"/>",
    tts: "<path d=\"M30 55h17l23-20v58L47 73H30z\" fill=\"#4f8ee8\" opacity=\".22\"/><path d=\"M30 55h17l23-20v58L47 73H30zM82 52a20 20 0 0 1 0 24M93 42a35 35 0 0 1 0 44\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#4f8ee8\"/>",
    image_generation: "<path d=\"M64 27c-23 0-40 15-40 36 0 20 15 37 36 37h7c7 0 9-9 4-13-5-5-1-14 7-14h8c11 0 18-7 18-18 0-17-17-28-40-28z\" fill=\"#9b7cf0\" opacity=\".22\"/><circle cx=\"48\" cy=\"53\" r=\"5\" fill=\"#f6cf58\"/><circle cx=\"64\" cy=\"45\" r=\"5\" fill=\"#f6cf58\"/><circle cx=\"80\" cy=\"54\" r=\"5\" fill=\"#ef6f78\"/><circle cx=\"55\" cy=\"72\" r=\"5\" fill=\"#27b6a9\"/>",
    music_generation: "<path d=\"M78 30v48a12 12 0 1 1-7-11V42l27-7v36a12 12 0 1 1-7-11V30z\" fill=\"#9b7cf0\" opacity=\".24\"/><path d=\"M78 30v48M78 42l27-7v36\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#9b7cf0\"/><circle cx=\"66\" cy=\"78\" r=\"11\" fill=\"#35c7d3\"/><circle cx=\"93\" cy=\"71\" r=\"11\" fill=\"#35c7d3\"/>",
    video_generation: "<rect x=\"28\" y=\"38\" width=\"54\" height=\"48\" rx=\"8\" fill=\"#ef6f78\" opacity=\".2\"/><path d=\"M82 54 104 42v40L82 70zM28 38h54v48H28z\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#ef6f78\"/>",
    media_conversion: "<rect x=\"28\" y=\"32\" width=\"72\" height=\"64\" rx=\"8\" fill=\"#7da3c8\" opacity=\".18\"/><path d=\"M44 32v64M84 32v64M28 48h16M28 80h16M84 48h16M84 80h16\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#7da3c8\"/>",
    video_download: "<rect x=\"31\" y=\"83\" width=\"66\" height=\"18\" rx=\"8\" fill=\"#4f8ee8\" opacity=\".18\"/><path d=\"M64 28v50M45 59l19 19 19-19M37 101h54\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#4f8ee8\"/>",
    document_creator: "<path d=\"M38 25h35l19 19v59H38z\" fill=\"#7da3c8\" opacity=\".16\"/><path d=\"M38 25h35l19 19v59H38zM73 25v20h19M50 58h28M50 74h28M50 90h18\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#7da3c8\"/>",
    docker: "<g fill=\"#2496ed\"><rect x=\"30\" y=\"53\" width=\"14\" height=\"12\" rx=\"2\"/><rect x=\"47\" y=\"53\" width=\"14\" height=\"12\" rx=\"2\"/><rect x=\"64\" y=\"53\" width=\"14\" height=\"12\" rx=\"2\"/><rect x=\"47\" y=\"38\" width=\"14\" height=\"12\" rx=\"2\"/><rect x=\"64\" y=\"38\" width=\"14\" height=\"12\" rx=\"2\"/><rect x=\"81\" y=\"53\" width=\"14\" height=\"12\" rx=\"2\"/><path d=\"M24 69h75c-4 18-19 28-43 28-17 0-28-6-32-28z\"/></g><path d=\"M98 59c7 0 11 4 13 10-6 1-11 0-16-5\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#ffffff\"/>",
    s3: "<path d=\"M33 43c0-9 14-16 31-16s31 7 31 16L88 98c-4 5-15 8-24 8s-20-3-24-8z\" fill=\"#f28b54\" opacity=\".22\"/><path d=\"M33 43c0 9 14 16 31 16s31-7 31-16M33 43l7 55c4 5 15 8 24 8s20-3 24-8l7-55\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#f28b54\"/><text x=\"64\" y=\"76\" text-anchor=\"middle\" dominant-baseline=\"middle\" font-family=\"Geist, Inter, Segoe UI, Arial, sans-serif\" font-size=\"20\" font-weight=\"800\" fill=\"#f6cf58\">S3</text>",
    webdav: "<path d=\"M42 86h43a19 19 0 0 0 2-38 28 28 0 0 0-53 9 15 15 0 0 0 8 29z\" fill=\"#4f8ee8\" opacity=\".22\"/><path d=\"M42 86h43a19 19 0 0 0 2-38 28 28 0 0 0-53 9 15 15 0 0 0 8 29z\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#4f8ee8\"/><text x=\"64\" y=\"68\" text-anchor=\"middle\" dominant-baseline=\"middle\" font-family=\"Geist, Inter, Segoe UI, Arial, sans-serif\" font-size=\"20\" font-weight=\"800\" fill=\"#35c7d3\">W</text>",
    koofr: "<path d=\"m64 25 36 20v38l-36 20-36-20V45z\" fill=\"#2d8be8\" opacity=\".2\"/><path d=\"m64 25 36 20v38l-36 20-36-20V45zM28 45l36 20 36-20M64 65v38\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#2d8be8\"/><text x=\"64\" y=\"65\" text-anchor=\"middle\" dominant-baseline=\"middle\" font-family=\"Geist, Inter, Segoe UI, Arial, sans-serif\" font-size=\"20\" font-weight=\"800\" fill=\"#35c7d3\">K</text>",
    netlify: "<path d=\"m64 22 18 31 34 11-34 11-18 31-18-31-34-11 34-11z\" fill=\"#00c7b7\" opacity=\".28\"/><path d=\"M64 22v84M22 64h84M46 53l36 22M82 53 46 75\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#00c7b7\"/>",
    vercel: "<path d=\"m64 28 42 72H22z\" fill=\"#f8fafc\"/><path d=\"m64 50 22 38H42z\" fill=\"#7da3c8\" opacity=\".5\"/>",
    cloudflare_tunnel: "<path d=\"M35 80h52c9 0 15-5 15-13 0-7-5-12-12-13-3-14-15-24-30-24-16 0-29 11-31 27-8 1-14 7-14 14 0 6 7 9 20 9z\" fill=\"#f38020\"/><path d=\"M52 91h38c9 0 17-6 19-14H70c-9 0-15 4-18 14z\" fill=\"#faae40\"/>",
    homepage: "<path d=\"M27 60 64 29l37 31v42H75V76H53v26H27z\" fill=\"#4f8ee8\" opacity=\".22\"/><path d=\"M27 60 64 29l37 31M37 57v45h16V76h22v26h16V57\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#4f8ee8\"/>",
    telegram: "<circle cx=\"64\" cy=\"64\" r=\"39\" fill=\"#2aabee\"/><path d=\"M43 63 89 42 80 89 66 75 58 84 57 70z\" fill=\"#ffffff\"/><path d=\"m57 70 29-25-20 30\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#2aabee\" stroke-width=\"4\"/>",
    discord: "<path d=\"M42 42c15-8 29-8 44 0l8 34c-8 8-16 12-25 13l-5-8-5 8c-9-1-17-5-25-13z\" fill=\"#5865f2\"/><circle cx=\"53\" cy=\"63\" r=\"5\" fill=\"#ffffff\"/><circle cx=\"75\" cy=\"63\" r=\"5\" fill=\"#ffffff\"/><path d=\"M51 77c9 5 17 5 26 0\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#ffffff\" stroke-width=\"4\"/>",
    rocketchat: "<path d=\"M32 43h61c7 0 12 5 12 12v20c0 7-5 12-12 12H58l-21 14V87h-5c-7 0-12-5-12-12V55c0-7 5-12 12-12z\" fill=\"#f5455c\" opacity=\".9\"/><path d=\"M42 63h43M42 76h28\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#f6cf58\" stroke-width=\"5\"/>",
    telnyx: "<path d=\"M46 28h36c6 0 10 4 10 10v52c0 6-4 10-10 10H46c-6 0-10-4-10-10V38c0-6 4-10 10-10z\" fill=\"#7b61ff\" opacity=\".2\"/><path d=\"M51 41h26M55 88h18M36 46l56 36\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#7b61ff\"/>",
    email: "<rect x=\"26\" y=\"40\" width=\"76\" height=\"54\" rx=\"9\" fill=\"#7da3c8\" opacity=\".18\"/><path d=\"M26 45 64 71l38-26M26 40h76v54H26z\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#7da3c8\"/>",
    agentmail: "<circle cx=\"64\" cy=\"64\" r=\"37\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#35c7d3\"/><text x=\"64\" y=\"65\" text-anchor=\"middle\" dominant-baseline=\"middle\" font-family=\"Geist, Inter, Segoe UI, Arial, sans-serif\" font-size=\"38\" font-weight=\"800\" fill=\"#4f8ee8\">@</text>",
    webhooks: "<circle cx=\"38\" cy=\"46\" r=\"12\" fill=\"#6fca8f\"/><circle cx=\"90\" cy=\"46\" r=\"12\" fill=\"#35c7d3\"/><circle cx=\"64\" cy=\"88\" r=\"12\" fill=\"#6fca8f\"/><path d=\"M50 46h28M44 57l14 21M84 57 70 78\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#9eb3c9\"/>",
    notifications: "<path d=\"M43 77V57c0-13 9-24 21-24s21 11 21 24v20l10 13H33z\" fill=\"#f6cf58\" opacity=\".25\"/><path d=\"M43 77V57c0-13 9-24 21-24s21 11 21 24v20l10 13H33zM54 98c5 7 15 7 20 0\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#f6cf58\"/>",
    github: "<circle cx=\"64\" cy=\"64\" r=\"39\" fill=\"#f8fafc\"/><path d=\"M48 91c0-8 0-13 6-16-12-2-20-10-20-23 0-6 2-11 6-15-1-4-1-9 1-14 8 0 13 4 16 6 5-1 9-1 14 0 3-2 8-6 16-6 2 5 2 10 1 14 4 4 6 9 6 15 0 13-8 21-20 23 6 3 6 8 6 16\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#111827\"/>",
    google_workspace: "<path d=\"M97 65c0 20-14 35-33 35-20 0-36-16-36-36s16-36 36-36c9 0 17 3 24 9L77 49c-4-3-8-5-13-5-11 0-20 9-20 20s9 20 20 20c9 0 15-5 17-13H64V58h32c1 2 1 5 1 7z\" fill=\"#4285f4\"/><path d=\"M31 52c4-14 17-24 33-24 9 0 17 3 24 9\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#ea4335\" /><path d=\"M31 76c5 14 17 24 33 24 15 0 27-9 32-23\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#34a853\" />",
    paperless_ngx: "<path d=\"M38 25h35l19 19v59H38z\" fill=\"#6fca8f\" opacity=\".16\"/><path d=\"M38 25h35l19 19v59H38zM73 25v20h19M50 58h28M50 74h28M50 90h18\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#6fca8f\"/><text x=\"64\" y=\"76\" text-anchor=\"middle\" dominant-baseline=\"middle\" font-family=\"Geist, Inter, Segoe UI, Arial, sans-serif\" font-size=\"22\" font-weight=\"800\" fill=\"#7da3c8\">P</text>",
    obsidian: "<path d=\"m64 22 38 42-38 42-38-42z\" fill=\"#7c3aed\" opacity=\".24\"/><path d=\"m64 22 38 42-38 42-38-42zM26 64h76M64 22l-16 42 16 42 16-42z\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#7c3aed\"/>",
    yepapi: "<path d=\"m64 25 35 20v38l-35 20-35-20V45z\" fill=\"#35c7d3\" opacity=\".22\"/><path d=\"m64 25 35 20v38l-35 20-35-20V45z\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#35c7d3\"/><text x=\"64\" y=\"65\" text-anchor=\"middle\" dominant-baseline=\"middle\" font-family=\"Geist, Inter, Segoe UI, Arial, sans-serif\" font-size=\"25\" font-weight=\"800\" fill=\"#4f8ee8\">Y</text>",
    home_assistant: "<circle cx=\"64\" cy=\"64\" r=\"39\" fill=\"#18bcf2\"/><path d=\"M43 70 64 46l21 24M52 65v22h24V65M64 46v41\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#ffffff\"/>",
    mqtt: "<path d=\"M64 87V59M42 37a32 32 0 0 1 44 0M31 25a48 48 0 0 1 66 0M53 49a16 16 0 0 1 22 0\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#6fca8f\"/><circle cx=\"64\" cy=\"91\" r=\"10\" fill=\"#35c7d3\"/><text x=\"64\" y=\"94\" text-anchor=\"middle\" dominant-baseline=\"middle\" font-family=\"Geist, Inter, Segoe UI, Arial, sans-serif\" font-size=\"16\" font-weight=\"800\" fill=\"#6fca8f\">M</text>",
    chromecast: "<rect x=\"29\" y=\"36\" width=\"70\" height=\"48\" rx=\"7\" fill=\"#4285f4\" opacity=\".2\"/><path d=\"M29 36h70v48H29zM31 95a28 28 0 0 1 28 28M31 79a44 44 0 0 1 44 44M31 106h.1\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#4285f4\"/>",
    bluetooth: "<path d=\"M56 24v80l30-25-44-31 44-24-30-24v104M34 43l52 37M34 85l52-42\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#4285f4\"/>",
    adguard: "<path d=\"M64 23 32 37v27c0 22 14 36 32 43 18-7 32-21 32-43V37z\" fill=\"#67b279\"/><path d=\"m47 65 12 12 26-31\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#ffffff\" stroke-width=\"7\"/>",
    fritzbox: "<rect x=\"27\" y=\"61\" width=\"74\" height=\"32\" rx=\"10\" fill=\"#ef6f78\" opacity=\".2\"/><path d=\"M27 61h74v32H27zM43 77h.1M59 77h.1M76 77h12M45 52a28 28 0 0 1 38 0M55 43a14 14 0 0 1 18 0\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#ef6f78\"/><text x=\"64\" y=\"77\" text-anchor=\"middle\" dominant-baseline=\"middle\" font-family=\"Geist, Inter, Segoe UI, Arial, sans-serif\" font-size=\"16\" font-weight=\"800\" fill=\"#7da3c8\">F</text>",
    ldap: "<rect x=\"33\" y=\"26\" width=\"62\" height=\"76\" rx=\"9\" fill=\"#7da3c8\" opacity=\".18\"/><path d=\"M47 26v76M60 49h22M60 66h22M60 83h14\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#7da3c8\"/>",
    truenas: "<path d=\"m64 22 21 12v24L64 70 43 58V34z\" fill=\"#0095d5\"/><path d=\"m31 66 21 12v24L31 114 10 102V78zM97 66l21 12v24l-21 12-21-12V78z\" fill=\"#00aeef\" opacity=\".82\"/><path d=\"M43 58 31 66M85 58l12 8M52 78l12-8 12 8\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#9eb3c9\" stroke-width=\"4\"/>",
    uptime_kuma: "<path d=\"M27 92h74M36 82l16-18 14 10 26-34\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#6fca8f\"/><circle cx=\"52\" cy=\"64\" r=\"6\" fill=\"#35c7d3\"/><circle cx=\"66\" cy=\"74\" r=\"6\" fill=\"#35c7d3\"/><circle cx=\"92\" cy=\"40\" r=\"6\" fill=\"#6fca8f\"/>",
    jellyfin: "<path d=\"m64 22 43 75H21z\" fill=\"#aa5cc3\" opacity=\".9\"/><path d=\"m64 46 22 39H42z\" fill=\"#00a4dc\"/><path d=\"m64 62 11 19H53z\" fill=\"#111827\" opacity=\".55\"/>",
    tailscale: "<g fill=\"#f8fafc\"><circle cx=\"44\" cy=\"44\" r=\"8\"/><circle cx=\"64\" cy=\"44\" r=\"8\"/><circle cx=\"84\" cy=\"44\" r=\"8\"/><circle cx=\"44\" cy=\"64\" r=\"8\"/><circle cx=\"64\" cy=\"64\" r=\"8\"/><circle cx=\"84\" cy=\"64\" r=\"8\"/><circle cx=\"44\" cy=\"84\" r=\"8\"/><circle cx=\"64\" cy=\"84\" r=\"8\"/><circle cx=\"84\" cy=\"84\" r=\"8\"/></g>",
    proxmox: "<path d=\"M28 38h30l16 26-16 26H28l16-26z\" fill=\"#e57000\"/><path d=\"M70 38h30L84 64l16 26H70L54 64z\" fill=\"#28394a\"/><path d=\"M49 45 79 83M79 45 49 83\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#f8fafc\" stroke-width=\"5\"/>",
    frigate: "<path d=\"M31 44h18l7-10h24l7 10h10v48H31z\" fill=\"#7da3c8\" opacity=\".2\"/><path d=\"M31 44h18l7-10h24l7 10h10v48H31z\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#7da3c8\"/><circle cx=\"64\" cy=\"68\" r=\"16\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#35c7d3\"/>",
    three_d_printers: "<path d=\"M34 35h60M64 35v25M45 60h38l10 35H35zM48 83h32\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#f28b54\"/><text x=\"64\" y=\"69\" text-anchor=\"middle\" dominant-baseline=\"middle\" font-family=\"Geist, Inter, Segoe UI, Arial, sans-serif\" font-size=\"21\" font-weight=\"800\" fill=\"#7da3c8\">3D</text>",
    remote_control: "<path d=\"M40 27 92 77 69 81 57 103z\" fill=\"#35c7d3\" opacity=\".26\"/><path d=\"M40 27 92 77 69 81 57 103z\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#35c7d3\"/>",
    grafana: "<circle cx=\"64\" cy=\"64\" r=\"29\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#f46800\"/><path d=\"M42 64h44M64 42v44\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#f6cf58\"/><text x=\"64\" y=\"64\" text-anchor=\"middle\" dominant-baseline=\"middle\" font-family=\"Geist, Inter, Segoe UI, Arial, sans-serif\" font-size=\"16\" font-weight=\"800\" fill=\"#f6cf58\">85</text>",
    meshcentral: "<circle cx=\"37\" cy=\"64\" r=\"12\" fill=\"#4f8ee8\"/><circle cx=\"91\" cy=\"39\" r=\"12\" fill=\"#35c7d3\"/><circle cx=\"91\" cy=\"89\" r=\"12\" fill=\"#4f8ee8\"/><path d=\"M48 59 80 44M48 69l32 15\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#9eb3c9\"/>",
    ansible: "<circle cx=\"64\" cy=\"64\" r=\"39\" fill=\"#f8fafc\"/><path d=\"m64 32 25 62-25-15-25 15zM64 32v47\" fill=\"#111827\"/>",
    security_proxy: "<path d=\"M64 25 31 39v25c0 22 13 36 33 45 20-9 33-23 33-45V39z\" fill=\"#7da3c8\" opacity=\".2\"/><path d=\"M64 25 31 39v25c0 22 13 36 33 45 20-9 33-23 33-45V39z\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#7da3c8\"/><text x=\"64\" y=\"66\" text-anchor=\"middle\" dominant-baseline=\"middle\" font-family=\"Geist, Inter, Segoe UI, Arial, sans-serif\" font-size=\"23\" font-weight=\"800\" fill=\"#35c7d3\">P</text>",
    guardian: "<path d=\"M64 25 31 39v25c0 22 13 36 33 45 20-9 33-23 33-45V39z\" fill=\"#35c7d3\" opacity=\".2\"/><path d=\"M64 25 31 39v25c0 22 13 36 33 45 20-9 33-23 33-45V39z\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#35c7d3\"/><path d=\"m49 66 11 11 22-27\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#6fca8f\"/>",
    llm_guardian: "<path d=\"M64 25 31 39v25c0 22 13 36 33 45 20-9 33-23 33-45V39z\" fill=\"#9b7cf0\" opacity=\".2\"/><path d=\"M64 25 31 39v25c0 22 13 36 33 45 20-9 33-23 33-45V39z\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#9b7cf0\"/><rect x=\"48\" y=\"53\" width=\"32\" height=\"24\" rx=\"8\" fill=\"#35c7d3\"/><circle cx=\"58\" cy=\"65\" r=\"3\" fill=\"#0f172a\"/><circle cx=\"70\" cy=\"65\" r=\"3\" fill=\"#0f172a\"/>",
    virustotal: "<circle cx=\"64\" cy=\"64\" r=\"34\" fill=\"#394eff\" opacity=\".9\"/><path d=\"M64 29v70M29 64h70M39 39l50 50M89 39 39 89\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#35c7d3\" stroke-width=\"5\"/>",
    ai_gateway: "<path d=\"M28 64h25M75 64h25M64 53v-25M64 75v25\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#4f8ee8\"/><rect x=\"53\" y=\"53\" width=\"22\" height=\"22\" rx=\"6\" fill=\"#35c7d3\"/><circle cx=\"28\" cy=\"64\" r=\"8\" fill=\"#4f8ee8\"/><circle cx=\"100\" cy=\"64\" r=\"8\" fill=\"#4f8ee8\"/><text x=\"64\" y=\"105\" text-anchor=\"middle\" dominant-baseline=\"middle\" font-family=\"Geist, Inter, Segoe UI, Arial, sans-serif\" font-size=\"17\" font-weight=\"800\" fill=\"#35c7d3\">AI</text>",
    composio: "<path d=\"m64 22 38 42-38 42-38-42z\" fill=\"#6d5dfc\" opacity=\".24\"/><path d=\"m64 22 38 42-38 42-38-42zM26 64h76M64 22l-16 42 16 42 16-42z\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#6d5dfc\"/><text x=\"64\" y=\"65\" text-anchor=\"middle\" dominant-baseline=\"middle\" font-family=\"Geist, Inter, Segoe UI, Arial, sans-serif\" font-size=\"19\" font-weight=\"800\" fill=\"#35c7d3\">C</text>",
    manus: "<circle cx=\"64\" cy=\"64\" r=\"38\" fill=\"#1f2937\"/><path d=\"M39 88V40l25 25 25-25v48\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"7\" stroke=\"#35c7d3\"/><circle cx=\"64\" cy=\"64\" r=\"7\" fill=\"#9b7cf0\"/>",
    huggingface: "<circle cx=\"64\" cy=\"64\" r=\"38\" fill=\"#ffd21e\"/><circle cx=\"51\" cy=\"57\" r=\"5\" fill=\"#111827\"/><circle cx=\"77\" cy=\"57\" r=\"5\" fill=\"#111827\"/><path d=\"M48 76c9 9 23 9 32 0\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#111827\" stroke-width=\"5\"/><text x=\"64\" y=\"95\" text-anchor=\"middle\" dominant-baseline=\"middle\" font-family=\"Geist, Inter, Segoe UI, Arial, sans-serif\" font-size=\"17\" font-weight=\"800\" fill=\"#111827\">HF</text>",
    evomap: "<path d=\"M26 36 52 26l24 10 26-10v66l-26 10-24-10-26 10z\" fill=\"#6fca8f\" opacity=\".18\"/><path d=\"M52 26v66M76 36v66M38 78c16-28 37-31 52-11\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#6fca8f\"/>",
    mcp: "<path d=\"M48 29v24M80 29v24M42 53h44v15c0 12-10 22-22 22S42 80 42 68zM64 90v14\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#9b7cf0\"/><path d=\"M48 68h32\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#35c7d3\"/><text x=\"64\" y=\"64\" text-anchor=\"middle\" dominant-baseline=\"middle\" font-family=\"Geist, Inter, Segoe UI, Arial, sans-serif\" font-size=\"18\" font-weight=\"800\" fill=\"#35c7d3\">M</text>",
    mcp_server: "<rect x=\"31\" y=\"28\" width=\"66\" height=\"72\" rx=\"10\" fill=\"#9b7cf0\" opacity=\".16\"/><path d=\"M31 38h66M31 64h66M31 90h66M45 51h.1M45 77h.1\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#9b7cf0\"/><path d=\"M78 51h10M78 77h10\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#35c7d3\"/><text x=\"64\" y=\"64\" text-anchor=\"middle\" dominant-baseline=\"middle\" font-family=\"Geist, Inter, Segoe UI, Arial, sans-serif\" font-size=\"18\" font-weight=\"800\" fill=\"#35c7d3\">M</text>",
    a2a: "<path d=\"M31 48h42M61 36l12 12-12 12M97 80H55M67 68 55 80l12 12\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#35c7d3\"/><text x=\"64\" y=\"64\" text-anchor=\"middle\" dominant-baseline=\"middle\" font-family=\"Geist, Inter, Segoe UI, Arial, sans-serif\" font-size=\"17\" font-weight=\"800\" fill=\"#4f8ee8\">A2A</text>",
    ollama: "<circle cx=\"64\" cy=\"64\" r=\"38\" fill=\"#f8fafc\"/><circle cx=\"64\" cy=\"64\" r=\"20\" fill=\"#7da3c8\" opacity=\".65\"/><circle cx=\"64\" cy=\"64\" r=\"9\" fill=\"#f8fafc\"/><text x=\"64\" y=\"105\" text-anchor=\"middle\" dominant-baseline=\"middle\" font-family=\"Geist, Inter, Segoe UI, Arial, sans-serif\" font-size=\"18\" font-weight=\"800\" fill=\"#7da3c8\">O</text>",
    danger_zone: "<path d=\"m64 24 42 76H22z\" fill=\"#ef6f78\" opacity=\".26\"/><path d=\"m64 24 42 76H22zM64 50v25M64 90h.1\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"6\" stroke=\"#ef6f78\"/>",
});

function ensureConfigSidebarIconSymbols() {
    if (typeof document === 'undefined') return;
    if (document.getElementById('config-sidebar-icon-symbols')) return;
    const sprite = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    sprite.id = 'config-sidebar-icon-symbols';
    sprite.setAttribute('class', 'config-sidebar-icon-symbols');
    sprite.setAttribute('aria-hidden', 'true');
    sprite.setAttribute('focusable', 'false');
    sprite.innerHTML = Object.keys(CONFIG_SIDEBAR_ICON_SYMBOLS).map(key => {
        return '<symbol id="' + CONFIG_SIDEBAR_ICON_SYMBOL_PREFIX + key + '" viewBox="0 0 128 128">' + CONFIG_SIDEBAR_ICON_SYMBOLS[key] + '</symbol>';
    }).join('');
    (document.body || document.documentElement).appendChild(sprite);
}

function createConfigSidebarIcon(key) {
    const iconKey = Object.prototype.hasOwnProperty.call(CONFIG_SIDEBAR_ICON_SLOTS, key) ? key : 'overview';
    const slot = CONFIG_SIDEBAR_ICON_SLOTS[iconKey];
    const maxSlot = CONFIG_SIDEBAR_ICON_GRID.columns * CONFIG_SIDEBAR_ICON_GRID.rows;
    const safeSlot = Number.isInteger(slot) && slot >= 0 && slot < maxSlot ? slot : CONFIG_SIDEBAR_ICON_SLOTS.overview;
    const symbolHref = '#' + CONFIG_SIDEBAR_ICON_SYMBOL_PREFIX + iconKey;
    ensureConfigSidebarIconSymbols();
    return '<span class="config-sidebar-icon-sprite config-icon-slot-' + safeSlot + '" aria-hidden="true"><svg class="config-sidebar-icon-svg" viewBox="0 0 128 128" focusable="false" aria-hidden="true"><use href="' + symbolHref + '"></use></svg></span>';
}

function buildSidebar() {
    const sb = document.getElementById('sidebar');
    sb.innerHTML = '';
    let lastWasIntSub = false;

    const search = document.createElement('div');
    search.className = 'cfg-sidebar-search';
    search.id = 'sidebarSearch';
    search.innerHTML = `
        <label for="sidebarSearchInput" class="cfg-visually-hidden">${escapeHtml(t('config.sidebar.search_placeholder'))}</label>
        <span class="cfg-sidebar-search-icon">🔍</span>
        <input type="text" id="sidebarSearchInput" class="cfg-sidebar-search-input"
            placeholder="${escapeHtml(t('config.sidebar.search_placeholder'))}"
            data-i18n-placeholder="config.sidebar.search_placeholder"
            autocomplete="off" spellcheck="false" value="${escapeHtml(sidebarSearchQuery)}">
        <button type="button" class="cfg-sidebar-search-clear${sidebarSearchQuery ? '' : ' hidden'}" id="sidebarSearchClear"
            title="${escapeHtml(t('config.common.clear'))}" aria-label="${escapeHtml(t('config.common.clear'))}">✕</button>
    `;
    sb.appendChild(search);

    const overviewItem = document.createElement('button');
    overviewItem.type = 'button';
    overviewItem.className = 'sidebar-item pw-overview-nav' + (activeSection === 'overview' ? ' active' : '');
    overviewItem.dataset.section = 'overview';
    overviewItem.dataset.searchLabel = t('config.precision.overview_title');
    overviewItem.dataset.searchDesc = t('config.precision.overview_desc');
    overviewItem.dataset.searchGroup = t('config.precision.workspace_label');
    overviewItem.innerHTML = `<span class="icon" aria-hidden="true">${createConfigSidebarIcon('overview')}</span><span class="sidebar-item-label">${escapeHtml(t('config.precision.overview_title'))}</span>`;
    overviewItem.onclick = () => navigateToConfigSection('overview');
    sb.appendChild(overviewItem);

    SECTIONS.forEach((group, groupIndex) => {
        const groupName = group.group;
        const isCollapsed = collapsedGroups.has(groupName);

        // Insert section divider before first integration sub-group
        if (group.sectionDivider && !lastWasIntSub) {
            const divider = document.createElement('div');
            divider.className = 'sidebar-section-divider';
            divider.textContent = group.sectionDivider;
            sb.appendChild(divider);
        }
        lastWasIntSub = !!group.integrationSubGroup;

        // Create collapsible group container
        const groupDiv = document.createElement('div');
        groupDiv.className = 'sidebar-group'
            + (isCollapsed ? ' collapsed' : '')
            + (group.integrationSubGroup ? ' integration-subgroup' : '');

        // Group header (collapsible)
        const header = document.createElement('button');
        header.type = 'button';
        header.className = 'sidebar-group-header'
            + (group.dangerGroup ? ' danger-group' : '')
            + (group.integrationSubGroup ? ' integration-subgroup-header' : '');
        header.innerHTML = `
                    <span class="sidebar-group-title">${groupName}</span>
                    <span class="sidebar-group-arrow">▼</span>
                `;
        header.onclick = () => toggleGroup(groupName, groupDiv);

        // Group content (items)
        const content = document.createElement('div');
        content.className = 'sidebar-group-content';
        const contentId = 'sidebar-group-content-' + groupIndex;
        content.id = contentId;
        content.style.maxHeight = isCollapsed ? '0' : 'none';
        header.setAttribute('aria-controls', contentId);
        header.setAttribute('aria-expanded', isCollapsed ? 'false' : 'true');

        group.items.forEach(s => {
            const blockedReason = sectionBlockedReason(s.key);
            const isBlocked = shouldBlockUnavailableSection(s.key) && blockedReason !== '';
            const item = document.createElement('button');
            item.type = 'button';
            item.className = 'sidebar-item'
                + (s.key === activeSection ? ' active' : '')
                + (group.dangerGroup ? ' danger-item' : '')
                + (group.integrationSubGroup ? ' integration-sub-item' : '')
                + (isBlocked ? ' sidebar-item-disabled' : '');
            item.dataset.section = s.key;
            item.dataset.searchLabel = s.label;
            item.dataset.searchDesc = s.desc || '';
            item.dataset.searchGroup = groupName;
            item.dataset.searchFields = JSON.stringify(configSearchEntriesForSection(s.key));
            if (isBlocked) {
                item.title = blockedReason;
                item.disabled = true;
            }
            item.innerHTML = '<span class="icon" aria-hidden="true">' + createConfigSidebarIcon(s.key) + '</span><span class="sidebar-item-label">' + escapeHtml(s.label) + '</span>';
            item.onclick = () => {
                if (shouldBlockUnavailableSection(s.key) && sectionBlockedReason(s.key)) return;
                if (item.dataset.searchTarget) navigateToConfigSection(s.key, { focusPath: item.dataset.searchTarget });
                else navigateToConfigSection(s.key);
            };
            content.appendChild(item);
        });

        groupDiv.appendChild(header);
        groupDiv.appendChild(content);
        sb.appendChild(groupDiv);
    });

    const noResults = document.createElement('div');
    noResults.className = 'cfg-sidebar-no-results hidden';
    noResults.id = 'sidebarSearchNoResults';
    noResults.textContent = t('config.sidebar.no_results');
    sb.appendChild(noResults);

    initSidebarSearch();
    if (sidebarSearchQuery) applySidebarSearch(sidebarSearchQuery);
    syncSidebarActiveState(activeSection);
}

function initSidebarSearch() {
    const input = document.getElementById('sidebarSearchInput');
    const clear = document.getElementById('sidebarSearchClear');
    if (!input || !clear) return;

    input.addEventListener('input', () => {
        sidebarSearchQuery = input.value;
        clear.classList.toggle('hidden', !sidebarSearchQuery);
        clearTimeout(sidebarSearchDebounceTimer);
        sidebarSearchDebounceTimer = setTimeout(() => applySidebarSearch(sidebarSearchQuery), 150);
    });
    input.addEventListener('keydown', handleSidebarSearchKeys);
    clear.addEventListener('click', () => clearSidebarSearch(true));
}

function getSidebarSearchTerms(query) {
    return (query || '').trim().toLowerCase().split(/\s+/).filter(Boolean);
}

function flattenConfigSchemaFields(fields, entries = []) {
    (fields || []).forEach(field => {
        if (field.type === 'object' && Array.isArray(field.children)) {
            flattenConfigSchemaFields(field.children, entries);
            return;
        }
        const path = field.key || field.yaml_key || '';
        const help = helpTexts[path];
        const helpText = help ? (help[lang] || help.en || '') : '';
        entries.push({
            path,
            label: fieldLabelText(path, field.yaml_key || path.split('.').pop()),
            help: helpText
        });
    });
    return entries;
}

function configSearchEntriesForSection(sectionKey) {
    const sectionSchema = schema.find(entry => entry.yaml_key === sectionKey);
    return sectionSchema ? flattenConfigSchemaFields(sectionSchema.children || []) : [];
}

function configValidationRules() {
    const result = {};
    const collect = fields => (fields || []).forEach(field => {
        if (field.type === 'object' && Array.isArray(field.children)) {
            collect(field.children);
            return;
        }
        if (field.type === 'int' || field.type === 'float') {
            result[field.key] = { type: 'number' };
        }
    });
    collect(schema);
    const catalog = window.AuraConfigCatalog || {};
    return Object.assign(result, catalog.validationRules || {});
}

function sidebarItemMatches(item, terms) {
    let searchFields = [];
    try { searchFields = JSON.parse(item.dataset.searchFields || '[]'); } catch (_) { }
    const haystack = [
        item.dataset.searchLabel || '',
        item.dataset.searchDesc || '',
        item.dataset.searchGroup || '',
        ...searchFields.flatMap(entry => [entry.label || '', entry.help || '', entry.path || ''])
    ].join(' ').toLowerCase();
    return terms.every(term => haystack.includes(term));
}

function matchingConfigFieldPath(item, terms) {
    let entries = [];
    try { entries = JSON.parse(item.dataset.searchFields || '[]'); } catch (_) { }
    const match = entries.find(entry => {
        const haystack = [entry.label || '', entry.help || '', entry.path || ''].join(' ').toLowerCase();
        return terms.every(term => haystack.includes(term));
    });
    return match ? match.path : '';
}

function highlightSidebarLabel(label, terms) {
    if (!terms.length) return escapeHtml(label);
    const uniqueTerms = [...new Set(terms)].sort((a, b) => b.length - a.length);
    const pattern = uniqueTerms.map(term => term.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|');
    if (!pattern) return escapeHtml(label);
    return escapeHtml(label).replace(new RegExp(pattern, 'gi'), match => `<mark class="cfg-search-match">${match}</mark>`);
}

function applySidebarSearch(query) {
    const terms = getSidebarSearchTerms(query);
    const clear = document.getElementById('sidebarSearchClear');
    const noResults = document.getElementById('sidebarSearchNoResults');
    let visibleItems = 0;

    if (clear) clear.classList.toggle('hidden', !terms.length);
    if (!terms.length) {
        clearSidebarSearch(false);
        return;
    }

    if (!sidebarSearchSnapshot) sidebarSearchSnapshot = new Set(collapsedGroups);

    document.querySelectorAll('.sidebar-group').forEach(groupEl => {
        const titleEl = groupEl.querySelector('.sidebar-group-title');
        const content = groupEl.querySelector('.sidebar-group-content');
        const groupName = titleEl ? titleEl.textContent.trim() : '';
        const groupMatches = terms.every(term => groupName.toLowerCase().includes(term));
        let groupVisibleItems = 0;

        groupEl.querySelectorAll('.sidebar-item').forEach(itemEl => {
            const isMatch = groupMatches || sidebarItemMatches(itemEl, terms);
            itemEl.dataset.searchTarget = isMatch ? matchingConfigFieldPath(itemEl, terms) : '';
            itemEl.classList.toggle('hidden', !isMatch);
            itemEl.classList.remove('search-focused');
            const labelEl = itemEl.querySelector('.sidebar-item-label');
            if (labelEl) {
                labelEl.innerHTML = highlightSidebarLabel(itemEl.dataset.searchLabel || '', terms);
            }
            if (isMatch) {
                groupVisibleItems++;
                visibleItems++;
            }
        });

        groupEl.classList.toggle('hidden', groupVisibleItems === 0);
        groupEl.classList.toggle('collapsed', groupVisibleItems === 0);
        if (content) content.style.maxHeight = groupVisibleItems > 0 ? 'none' : '0';
    });

    sidebarSearchFocusedIndex = -1;
    if (noResults) noResults.classList.toggle('hidden', visibleItems > 0);
}

function clearSidebarSearch(focusInput) {
    sidebarSearchQuery = '';
    sidebarSearchFocusedIndex = -1;
    const input = document.getElementById('sidebarSearchInput');
    const clear = document.getElementById('sidebarSearchClear');
    const noResults = document.getElementById('sidebarSearchNoResults');
    if (input) input.value = '';
    if (clear) clear.classList.add('hidden');
    if (noResults) noResults.classList.add('hidden');

    document.querySelectorAll('.sidebar-group').forEach(groupEl => {
        const titleEl = groupEl.querySelector('.sidebar-group-title');
        const content = groupEl.querySelector('.sidebar-group-content');
        const groupName = titleEl ? titleEl.textContent.trim() : '';
        const isCollapsed = sidebarSearchSnapshot ? sidebarSearchSnapshot.has(groupName) : collapsedGroups.has(groupName);

        groupEl.classList.remove('hidden');
        groupEl.classList.toggle('collapsed', isCollapsed);
        if (content) content.style.maxHeight = isCollapsed ? '0' : 'none';
    });

    document.querySelectorAll('.sidebar-item').forEach(itemEl => {
        itemEl.classList.remove('hidden', 'search-focused');
        itemEl.dataset.searchTarget = '';
        const labelEl = itemEl.querySelector('.sidebar-item-label');
        if (labelEl) labelEl.textContent = itemEl.dataset.searchLabel || '';
    });

    if (sidebarSearchSnapshot) {
        collapsedGroups.clear();
        sidebarSearchSnapshot.forEach(groupName => collapsedGroups.add(groupName));
        sidebarSearchSnapshot = null;
    }

    if (focusInput && input) input.focus();
}

function getVisibleSidebarSearchItems() {
    return [...document.querySelectorAll('.sidebar-item:not(.hidden):not(.sidebar-item-disabled)')]
        .filter(item => !item.closest('.sidebar-group.hidden'));
}

function focusSidebarSearchItem(index) {
    const items = getVisibleSidebarSearchItems();
    document.querySelectorAll('.sidebar-item.search-focused').forEach(item => item.classList.remove('search-focused'));
    if (!items.length) {
        sidebarSearchFocusedIndex = -1;
        return;
    }
    sidebarSearchFocusedIndex = (index + items.length) % items.length;
    const item = items[sidebarSearchFocusedIndex];
    item.classList.add('search-focused');
    item.scrollIntoView({ block: 'nearest', inline: 'nearest' });
}

function handleSidebarSearchKeys(event) {
    if (event.key === 'Escape') {
        event.preventDefault();
        clearSidebarSearch(true);
        return;
    }
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp' && event.key !== 'Enter') return;

    const items = getVisibleSidebarSearchItems();
    if (!items.length) return;
    event.preventDefault();

    if (event.key === 'ArrowDown') {
        focusSidebarSearchItem(sidebarSearchFocusedIndex + 1);
    } else if (event.key === 'ArrowUp') {
        focusSidebarSearchItem(sidebarSearchFocusedIndex - 1);
    } else if (event.key === 'Enter') {
        const item = items[sidebarSearchFocusedIndex >= 0 ? sidebarSearchFocusedIndex : 0];
        if (item && item.dataset.section) {
            if (item.dataset.searchTarget) navigateToConfigSection(item.dataset.section, { focusPath: item.dataset.searchTarget });
            else navigateToConfigSection(item.dataset.section);
        }
    }
}

function toggleGroup(groupName, groupDiv) {
    if (getSidebarSearchTerms(sidebarSearchQuery).length) return;
    const isCollapsed = collapsedGroups.has(groupName);
    const content = groupDiv.querySelector('.sidebar-group-content');

    if (isCollapsed) {
        collapsedGroups.delete(groupName);
        groupDiv.classList.remove('collapsed');
        content.style.maxHeight = content.scrollHeight + 'px';
        setTimeout(() => content.style.maxHeight = 'none', 300);
    } else {
        collapsedGroups.add(groupName);
        groupDiv.classList.add('collapsed');
        content.style.maxHeight = content.scrollHeight + 'px';
        setTimeout(() => content.style.maxHeight = '0', 0);
    }
    const headerBtn = groupDiv.querySelector('.sidebar-group-header');
    if (headerBtn) headerBtn.setAttribute('aria-expanded', isCollapsed ? 'true' : 'false');
    saveCollapsedGroups();
}


function hasUnsavedConfigChanges() {
    if (window.AuraConfigState) return window.AuraConfigState.isDirty();
    return isDirty && collectSnapshot() !== initialSnapshot;
}

function normalizeSectionKey(key) {
    if (!hasVisibleSection(key)) return 'server';
    if (shouldBlockUnavailableSection(key) && sectionBlockedReason(key)) return 'server';
    return key;
}

async function confirmDiscardUnsavedChanges() {
    if (!hasUnsavedConfigChanges()) return true;
    return showModal(
        t('config.unsaved_changes.title'),
        t('config.unsaved_changes.message'),
        true,
        {
            confirmText: t('config.unsaved_changes.discard'),
            cancelText: t('config.unsaved_changes.stay')
        }
    );
}

function showUnsavedChangesDecision() {
    return new Promise(resolve => {
        const previous = document.getElementById('cfg-unsaved-decision');
        if (previous) previous.remove();

        const overlay = document.createElement('div');
        overlay.id = 'cfg-unsaved-decision';
        overlay.className = 'modal-overlay open active pw-unsaved-overlay';
        overlay.innerHTML = `<div class="modal-card pw-unsaved-card" role="dialog" aria-modal="true" aria-labelledby="cfg-unsaved-title">
            <div class="pw-unsaved-eyebrow">${escapeHtml(t('config.save_bar.unsaved_changes'))}</div>
            <h2 id="cfg-unsaved-title" class="modal-title">${escapeHtml(t('config.unsaved_changes.title'))}</h2>
            <p class="modal-desc">${escapeHtml(t('config.unsaved_changes.message'))}</p>
            <div class="modal-actions pw-unsaved-actions">
                <button type="button" class="btn btn-secondary" data-decision="stay">${escapeHtml(t('config.unsaved_changes.stay'))}</button>
                <button type="button" class="btn btn-secondary" data-decision="discard">${escapeHtml(t('config.unsaved_changes.discard'))}</button>
                <button type="button" class="btn-save" data-decision="save">${escapeHtml(t('config.unsaved_changes.save_and_continue'))}</button>
            </div>
        </div>`;

        function close(decision) {
            document.removeEventListener('keydown', onKeyDown);
            overlay.remove();
            resolve(decision);
        }

        function onKeyDown(event) {
            if (event.key === 'Escape') close('stay');
        }

        overlay.addEventListener('click', event => {
            if (event.target === overlay) close('stay');
            const button = event.target.closest('[data-decision]');
            if (button) close(button.dataset.decision);
        });
        document.addEventListener('keydown', onKeyDown);
        document.body.appendChild(overlay);
        overlay.querySelector('[data-decision="save"]').focus();
    });
}

async function navigateToConfigSection(key, options = {}) {
    const target = normalizeSectionKey(key);
    if (target === activeSection && !hasUnsavedConfigChanges()) {
        await selectSection(target, options);
        resetDirtySnapshot();
        return true;
    }
    if (hasUnsavedConfigChanges()) {
        const decision = await showUnsavedChangesDecision();
        if (decision === 'save') {
            if (!await saveConfig()) return false;
        } else if (decision === 'discard') {
            if (window.AuraConfigState) configData = window.AuraConfigState.discard();
        } else {
            if (window.location.hash !== '#' + activeSection) {
                history.replaceState(null, '', '#' + activeSection);
            }
            return false;
        }
    }
    await selectSection(target, options);
    resetDirtySnapshot();
    return true;
}

function handleConfigBeforeUnload(event) {
    if (!hasUnsavedConfigChanges()) return;
    event.preventDefault();
    event.returnValue = '';
    return '';
}

function handleConfigHashChange() {
    const key = (window.location.hash || '').replace(/^#/, '') || 'overview';
    if (key === activeSection) return;
    navigateToConfigSection(key, { scrollBehavior: 'auto' });
}

function syncSidebarActiveState(key) {
    document.querySelectorAll('.sidebar-item').forEach(el => {
        const active = el.dataset.section === key;
        el.classList.toggle('active', active);
        if (active) el.setAttribute('aria-current', 'page');
        else el.removeAttribute('aria-current');
    });
}

function syncToggleA11y(toggle) {
    if (!toggle) return;
    const disabled = toggle.disabled === true || toggle.classList.contains('cfg-toggle-disabled') || toggle.getAttribute('aria-disabled') === 'true';
    const on = toggle.classList.contains('on');
    toggle.setAttribute('role', 'switch');
    toggle.setAttribute('aria-checked', on ? 'true' : 'false');
    toggle.setAttribute('tabindex', disabled ? '-1' : '0');
    if (disabled) toggle.setAttribute('aria-disabled', 'true');
    else toggle.removeAttribute('aria-disabled');
}

function enhanceConfigControls(root = document) {
    root.querySelectorAll('.toggle').forEach(toggle => {
        syncToggleA11y(toggle);
        if (toggle.dataset.a11yBound === 'true') return;
        toggle.dataset.a11yBound = 'true';
        toggle.addEventListener('keydown', event => {
            if (event.key !== ' ' && event.key !== 'Enter') return;
            event.preventDefault();
            if (toggle.disabled === true || toggle.classList.contains('cfg-toggle-disabled') || toggle.getAttribute('aria-disabled') === 'true') return;
            toggle.click();
        });
    });
}

function loadRecentSections() {
    try {
        const value = JSON.parse(localStorage.getItem(CONFIG_RECENT_KEY) || '[]');
        return Array.isArray(value) ? value.filter(hasVisibleSection).filter(key => key !== 'overview').slice(0, CONFIG_RECENT_LIMIT) : [];
    } catch (_) {
        return [];
    }
}

function recordRecentSection(key) {
    if (!key || key === 'overview') return;
    const recent = loadRecentSections().filter(value => value !== key);
    recent.unshift(key);
    localStorage.setItem(CONFIG_RECENT_KEY, JSON.stringify(recent.slice(0, CONFIG_RECENT_LIMIT)));
}

function sectionMetadata(key) {
    for (const group of SECTIONS) {
        const section = group.items.find(item => item.key === key);
        if (section) return { ...section, group: group.group };
    }
    return null;
}

function renderConfigOverview() {
    const content = document.getElementById('content');
    const recent = loadRecentSections().map(sectionMetadata).filter(Boolean);
    const sectionCount = SECTIONS.reduce((total, group) => total + group.items.length, 0);
    const recentMarkup = recent.length
        ? recent.map(section => `<button type="button" class="pw-recent-card" data-overview-section="${escapeAttr(section.key)}">
            <span class="pw-overview-card-kicker">${escapeHtml(section.group)}</span>
            <strong>${escapeHtml(section.label)}</strong>
            <span>${escapeHtml(section.desc || '')}</span>
        </button>`).join('')
        : `<div class="pw-overview-empty">${escapeHtml(t('config.precision.recent_empty'))}</div>`;
    const groupMarkup = SECTIONS.map(group => `<section class="pw-overview-card${group.dangerGroup ? ' pw-overview-card-danger' : ''}">
        <div class="pw-overview-card-heading">
            <h2>${escapeHtml(group.group)}</h2>
            <span>${group.items.length}</span>
        </div>
        <div class="pw-overview-links">
            ${group.items.map(section => `<button type="button" data-overview-section="${escapeAttr(section.key)}">${escapeHtml(section.label)}<span aria-hidden="true">→</span></button>`).join('')}
        </div>
    </section>`).join('');

    content.innerHTML = `<div class="cfg-section active pw-overview">
        <div class="pw-overview-hero">
            <span class="pw-overview-eyebrow">${escapeHtml(t('config.precision.workspace_label'))}</span>
            <h1>${escapeHtml(t('config.precision.overview_title'))}</h1>
            <p>${escapeHtml(t('config.precision.overview_desc'))}</p>
            <div class="pw-overview-stat"><strong>${sectionCount}</strong><span>${escapeHtml(t('config.precision.overview_sections'))}</span></div>
        </div>
        <section class="pw-overview-recent" aria-labelledby="pw-recent-title">
            <h2 id="pw-recent-title">${escapeHtml(t('config.precision.recent_title'))}</h2>
            <div class="pw-recent-grid">${recentMarkup}</div>
        </section>
        <section class="pw-overview-groups" aria-labelledby="pw-groups-title">
            <h2 id="pw-groups-title">${escapeHtml(t('config.precision.groups_title'))}</h2>
            <div class="pw-overview-grid">${groupMarkup}</div>
        </section>
    </div>`;
    content.querySelectorAll('[data-overview-section]').forEach(button => {
        button.addEventListener('click', () => navigateToConfigSection(button.dataset.overviewSection));
    });
}

function focusConfigField(path) {
    if (!path) return;
    requestAnimationFrame(() => {
        const field = document.querySelector('[data-path="' + CSS.escape(path) + '"]');
        if (!field) return;
        let disclosure = field.closest('details');
        while (disclosure) {
            disclosure.open = true;
            disclosure = disclosure.parentElement ? disclosure.parentElement.closest('details') : null;
        }
        const target = field.closest('.field-group, .pw-field') || field;
        target.classList.add('pw-field-focus');
        target.scrollIntoView({ block: 'center', behavior: 'smooth' });
        if (typeof field.focus === 'function') field.focus({ preventScroll: true });
        window.setTimeout(() => target.classList.remove('pw-field-focus'), 1800);
    });
}

function loadAdvancedSectionState() {
    try {
        const value = JSON.parse(localStorage.getItem(CONFIG_ADVANCED_KEY) || '{}');
        return value && typeof value === 'object' ? value : {};
    } catch (_) {
        return {};
    }
}

function isAdvancedConfigPath(path) {
    const catalog = window.AuraConfigCatalog || {};
    const explicit = catalog.sectionTiers && catalog.sectionTiers[path];
    if (explicit) return explicit === 'advanced';
    const normalized = String(path || '').toLowerCase();
    const fieldName = normalized.split('.').pop() || '';
    return (catalog.advancedPathPatterns || []).some(pattern => fieldName.includes(pattern));
}

function enhanceConfigSectionLayout(key) {
    if (!key || key === 'overview') return;
    const section = document.querySelector('#content > .cfg-section.active');
    if (!section || section.querySelector(':scope > .pw-advanced')) return;

    section.querySelectorAll('.field-group').forEach(group => group.classList.add('pw-field'));
    const advancedFields = [...section.querySelectorAll('.field-group')].filter(group => {
        if (group.closest('.modal-overlay, .pw-modal-overlay, .pw-advanced')) return false;
        if (group.parentElement !== section && !group.parentElement?.classList.contains('pw-panel-body')) return false;
        if (group.dataset.tier === 'advanced') return true;
        const control = group.querySelector('[data-path]');
        return control ? isAdvancedConfigPath(control.dataset.path) : false;
    });
    if (!advancedFields.length) return;

    const advancedState = loadAdvancedSectionState();
    const details = document.createElement('details');
    details.className = 'pw-advanced';
    details.open = advancedState[key] === true;
    const summary = document.createElement('summary');
    summary.innerHTML = `<span><strong>${escapeHtml(t('config.precision.advanced_title'))}</strong><small>${escapeHtml(t('config.precision.advanced_desc'))}</small></span><span class="pw-disclosure-mark" aria-hidden="true">+</span>`;
    const body = document.createElement('div');
    body.className = 'pw-advanced-body';
    advancedFields.forEach(field => body.appendChild(field));
    details.append(summary, body);
    details.addEventListener('toggle', () => {
        const nextState = loadAdvancedSectionState();
        nextState[key] = details.open;
        localStorage.setItem(CONFIG_ADVANCED_KEY, JSON.stringify(nextState));
    });
    section.appendChild(details);
}

let configSectionObserverFrame = 0;
const configSectionObserver = new MutationObserver(() => {
    if (configSectionObserverFrame) return;
    configSectionObserverFrame = requestAnimationFrame(() => {
        configSectionObserverFrame = 0;
        enhanceConfigSectionLayout(activeSection);
    });
});
const configSectionObserverRoot = document.getElementById('content');
// Only watch direct children of #content. Watching subtree causes a feedback
// loop because enhanceConfigSectionLayout mutates the active section itself.
if (configSectionObserverRoot) {
    configSectionObserver.observe(configSectionObserverRoot, { childList: true });
}

async function selectSection(key, options = {}) {
    const { scrollBehavior = 'smooth', focusPath = '' } = options;
    key = normalizeSectionKey(key);
    activeSection = key;
    recordRecentSection(key);
    updateSaveDockSection(key);
    if (window.AuraConfigState) window.AuraConfigState.beginSection(key);
    localStorage.setItem('aurago-cfg-section', key);
    if (window.location.hash !== '#' + key) {
        history.replaceState(null, '', '#' + key);
    }
    syncSidebarActiveState(key);
    // Auto-expand the group containing this section if it is collapsed
    let expandedTargetGroup = false;
    for (const group of SECTIONS) {
        if (!getSidebarSearchTerms(sidebarSearchQuery).length && group.items.some(s => s.key === key) && collapsedGroups.has(group.group)) {
            const groupDiv = [...document.querySelectorAll('.sidebar-group')].find(
                el => el.querySelector('.sidebar-group-title') &&
                    el.querySelector('.sidebar-group-title').textContent.trim() === group.group
            );
            if (groupDiv) {
                toggleGroup(group.group, groupDiv);
                expandedTargetGroup = true;
            }
            break;
        }
    }
    scrollActiveSidebarItemIntoView(scrollBehavior, expandedTargetGroup ? 320 : 0);
    window.dispatchEvent(new CustomEvent('cfg:section-leave'));
    await renderSection(key);
    const content = document.getElementById('content');
    if (content) content.scrollTop = 0;
    enhanceConfigSectionLayout(key);
    attachChangeListeners();
    document.dispatchEvent(new CustomEvent('cfg:section-rendered', { detail: { key, root: document.getElementById('content') } }));
    focusConfigField(focusPath);
    closeSidebar();
}

function renderOptimizations() {
    const agentData = configData['agent'] || {};
    const cbData = configData['circuit_breaker'] || {};
    const agentSchema = schema.find(s => s.yaml_key === 'agent');
    const cbSchema = schema.find(s => s.yaml_key === 'circuit_breaker');

    function agentF(...keys) {
        if (!agentSchema || !agentSchema.children) return [];
        return agentSchema.children.filter(f => keys.includes(f.yaml_key));
    }
    function cbF(...keys) {
        if (!cbSchema || !cbSchema.children) return [];
        return cbSchema.children.filter(f => keys.includes(f.yaml_key));
    }

    let html = '';

    // Group 1: Token & Context
    html += `<div class="cfg-group-title cfg-group-title-top">${t('config.section.optimizations.group.token_context')}</div>`;
    html += renderFields(
        agentF('optimizer_enabled', 'system_prompt_token_budget', 'adaptive_system_prompt_token_budget', 'context_window', 'memory_compression_char_limit'),
        agentData, 'agent'
    );

    // Group 2: Tool Context
    html += `<div class="cfg-group-title cfg-group-title-top">${t('config.section.optimizations.group.tool_context')}</div>`;
    html += renderFields(
        agentF('tool_output_limit', 'discover_tools_snapshot_ttl_minutes', 'max_tool_guides', 'core_memory_max_entries', 'core_memory_cap_mode'),
        agentData, 'agent'
    );

    // Group 3: Tool Visibility
    html += `<div class="cfg-group-title cfg-group-title-top">${t('config.section.optimizations.group.tool_visibility')}</div>`;
    html += renderFields(
        agentF('adaptive_tools'),
        agentData, 'agent'
    );

    // Group 4: Safety Limits & Recovery
    html += `<div class="cfg-group-title cfg-group-title-top">${t('config.section.optimizations.group.safety_limits')}</div>`;
    html += renderFields(
        cbF('max_tool_calls', 'llm_timeout_seconds', 'maintenance_timeout_minutes', 'retry_intervals'),
        cbData, 'circuit_breaker'
    );
    html += renderFields(agentF('recovery'), agentData, 'agent');

    // Group 5: Background Processing
    html += `<div class="cfg-group-title cfg-group-title-top">${t('config.section.optimizations.group.background_tasks')}</div>`;
    html += renderFields(agentF('background_tasks'), agentData, 'agent');

    return html;
}

async function renderSection(key) {
    if (key === 'overview') {
        renderConfigOverview();
        return;
    }
    let section = null;
    for (const group of SECTIONS) {
        const found = group.items.find(s => s.key === key);
        if (found) {
            section = found;
            break;
        }
    }
    if (!section) return;

    // Lazy-load section module if available
    const modInfo = SECTION_MODULES[key];
    if (modInfo) {
        try { await loadModule(modInfo.m); } catch (e) {
            document.getElementById('content').innerHTML = '<div class="cfg-error-state cfg-error-state-md">\u274c Module load error: ' + escapeHtml(e && e.message ? e.message : String(e)) + '</div>';
            return;
        }
        const fn = window[modInfo.fn];
        if (fn) {
            try {
                await fn(section);
            } catch (e) {
                console.error('Config section render failed', key, e);
                document.getElementById('content').innerHTML = '<div class="cfg-error-state cfg-error-state-md">\u274c Section render error: ' + escapeHtml(e && e.message ? e.message : String(e)) + '</div>';
            }
            return;
        }
    }

    const data = configData[key] || {};
    const sectionSchema = schema.find(s => s.yaml_key === key);

    // Keys managed by other sections — skip in generic renderer
    const AGENT_SKIP_KEYS = new Set([
        'allow_shell', 'allow_python', 'allow_filesystem_write',
        'allow_network_requests', 'allow_remote_shell', 'allow_self_update', 'allow_package_manager',
        'allow_mcp',               // → Danger Zone
        'allow_web_scraper',        // → deprecated, migrated to tools.web_scraper.enabled
        'sudo_enabled',            // → Danger Zone
        'core_personality',         // → Prompts & Personas
        'additional_prompt',        // → Prompts & Personas
        'personality_v2_model',     // → managed by provider
        'personality_v2_url',       // → managed by provider
        'personality_v2_api_key',   // → managed by provider
        // → Optimierungen section
        'optimizer_enabled', 'system_prompt_token_budget', 'adaptive_system_prompt_token_budget',
        'context_window', 'memory_compression_char_limit',
        'tool_output_limit', 'discover_tools_snapshot_ttl_minutes', 'max_tool_guides',
        'core_memory_max_entries', 'core_memory_cap_mode',
        'adaptive_tools', 'recovery', 'background_tasks',
        'output_compression'       // → Output Compression section
    ]);
    // Legacy fields superseded by provider management — hide from UI
    const EMBEDDINGS_SKIP_KEYS = new Set(['api_key', 'external_model', 'external_url', 'internal_model']);
    // Legacy fields superseded by provider management — hide from UI
    const LLM_SKIP_KEYS = new Set(['api_key', 'base_url', 'model', 'use_native_functions', 'structured_outputs', 'multimodal', 'multimodal_provider_types_extra']);
    // Sections whose api_key/base_url/model are managed by provider entries
    const PROVIDER_MANAGED_SECTIONS = new Set(['vision', 'whisper']);
    // Tool sub-keys with dedicated sections — hide from generic 'tools' section
    const TOOLS_SKIP_KEYS = new Set([
        'web_scraper',    // → Web Scraper section
        'wikipedia',      // → Information Tools section
        'ddg_search',     // → Information Tools section
        'pdf_extractor',  // → Information Tools section
        'wol',            // → Network Tools section
        'stop_process',   // → Network Tools section
        'network_ping',   // → Network Tools section
        'network_scan',   // → Network Tools section
        'web_capture',    // → Network Tools section
        'form_automation',// → Network Tools section
        'upnp_scan',      // → Network Tools section
        'browser_automation', // → Browser Automation section
        'virtual_desktop',// → Virtual Desktop section
        'media_conversion',// → Media Conversion section
        'video_download', // → Video Download section
        'send_youtube_video', // → Video Download section
        'document_creator',// → Document Creator section
        'skill_manager',  // → Skill Manager section
        'daemon_skills'   // → Daemon Skills section
    ]);

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // Generic feature-unavailability banner for sections with runtime checks
    const sectionFeatureKey = SECTION_FEATURE_MAP[key];
    if (sectionFeatureKey) {
        const fb = featureUnavailableBanner(sectionFeatureKey);
        if (fb) html += fb;
    }
    const sectionBlocked = sectionFeatureKey && shouldBlockUnavailableSection(key) && runtimeData.features && runtimeData.features[sectionFeatureKey] && !runtimeData.features[sectionFeatureKey].available;
    if (sectionBlocked) html += '<div class="feature-unavailable-fields">';

    // LLM settings only need a warning when a core helper path is disabled.
    if (key === 'llm') {
        if (!configData.llm || !configData.llm.helper_enabled) {
            html += `<div class="cfg-note-banner cfg-note-banner-warning">
                    \u{26A0} ${t('config.llm.helper_disabled_banner')}
                </div>`;
        }
    }

    // Embeddings explanation
    if (key === 'embeddings') {
        html += `<div class="cfg-note-banner cfg-note-banner-info">
                    \u{1F9E0} ${t('config.embeddings.info_banner')}
                </div>`;
        html += `<div class="cfg-note-banner cfg-note-banner-warning">
                    ⚠️ ${t('config.embeddings.change_warning_banner')}
                </div>`;
        html += renderEmbeddingsRuntimeBlock();
    }

    // Tools permissions warning
    if (key === 'tools') {
        html += `<div class="cfg-note-banner cfg-note-banner-warning">
                    ⚠️ ${t('config.tools.warning_banner')}
                </div>`;
    }

    // Information Tools — Wikipedia, DuckDuckGo, PDF Extractor (subset of tools config)
    if (key === 'info_tools' || key === 'network_tools') {
        if (key === 'info_tools') {
            html += `<div class="cfg-note-banner cfg-note-banner-info">
                        ℹ️ ${t('config.info_tools.always_active_banner')}
                    </div>`;
        }
        const toolsData = configData['tools'] || {};
        const toolsSchema = schema.find(s => s.yaml_key === 'tools');
        const SECTION_KEYS = key === 'info_tools'
            ? new Set(['wikipedia', 'ddg_search', 'pdf_extractor'])
            : new Set(['wol', 'stop_process', 'network_ping', 'network_scan', 'web_capture', 'form_automation', 'upnp_scan']);
        if (toolsSchema && toolsSchema.children) {
            const children = toolsSchema.children.filter(f => SECTION_KEYS.has(f.yaml_key));
            html += renderFields(children, toolsData, 'tools');
        }
        html += '</div>';
        document.getElementById('content').innerHTML = html;
        return;
    }

    // Optimierungen — combines agent.* and circuit_breaker.* fields
    if (key === 'optimizations') {
        html += renderOptimizations();
        html += '</div>';
        document.getElementById('content').innerHTML = html;
        return;
    }

    if (sectionSchema && sectionSchema.children) {
        let schemaChildren = sectionSchema.children;
        if (key === 'agent') {
            schemaChildren = schemaChildren.filter(f => !AGENT_SKIP_KEYS.has(f.yaml_key));
        }
        if (key === 'embeddings') {
            schemaChildren = schemaChildren.filter(f => !EMBEDDINGS_SKIP_KEYS.has(f.yaml_key));
        }
        if (key === 'llm' || key === 'fallback_llm') {
            schemaChildren = schemaChildren.filter(f => !LLM_SKIP_KEYS.has(f.yaml_key));
        }
        if (PROVIDER_MANAGED_SECTIONS.has(key)) {
            schemaChildren = schemaChildren.filter(f => !LLM_SKIP_KEYS.has(f.yaml_key));
        }
        if (key === 'co_agents') {
            schemaChildren = schemaChildren.map(f => {
                if (f.yaml_key === 'llm' && f.children) {
                    return { ...f, children: f.children.filter(c => !LLM_SKIP_KEYS.has(c.yaml_key)) };
                }
                return f;
            });
        }
        if (key === 'tools') {
            schemaChildren = schemaChildren.filter(f => !TOOLS_SKIP_KEYS.has(f.yaml_key));
        }
        html += renderFields(schemaChildren, data, key);
    } else {
        for (const [k, v] of Object.entries(data)) {
            // Skip keys that belong to other dedicated sections
            if (key === 'agent' && AGENT_SKIP_KEYS.has(k)) continue;
            if (key === 'embeddings' && EMBEDDINGS_SKIP_KEYS.has(k)) continue;
            if ((key === 'llm' || key === 'fallback_llm') && LLM_SKIP_KEYS.has(k)) continue;
            if (PROVIDER_MANAGED_SECTIONS.has(key) && LLM_SKIP_KEYS.has(k)) continue;
            if (typeof v === 'object' && v !== null && !Array.isArray(v)) {
                html += '<div class="cfg-group-title cfg-group-title-top">' + formatKey(k) + '</div>';
                for (const [sk, sv] of Object.entries(v)) {
                    if (key === 'co_agents' && k === 'llm' && LLM_SKIP_KEYS.has(sk)) continue;
                    html += renderField(key + '.' + k + '.' + sk, sk, sv, key + '.' + k);
                }
            } else {
                html += renderField(key + '.' + k, k, v, key);
            }
        }
    }

    // MeshCentral connection tester
    if (key === 'meshcentral') {
        await loadModule('providers');
        html += renderMeshCentralTestBlock();
    }

    // Ansible — inject "Generate Token" button block below the token field
    if (key === 'ansible') {
        html += `
        <div class="cfg-action-block" id="ansible-token-block">
            <div class="cfg-action-block-title">⚙️ ${t('config.ansible.generate_token_btn')}</div>
            <div class="pw-u-inline-actions">
                <button id="ansible-gen-token-btn" class="cfg-btn cfg-btn-primary" onclick="ansibleGenerateToken()">
                    🔑 ${t('config.ansible.generate_token_btn')}
                </button>
                <span id="ansible-gen-token-result" class="pw-u-muted"></span>
            </div>
            <div id="ansible-token-preview" class="pw-u-hidden pw-u-mt-8">
                <code id="ansible-token-value" class="pw-u-token-code"></code>
                <button class="cfg-btn cfg-btn-sm pw-u-mt-6" onclick="ansibleCopyToken()">${t('config.ansible.generate_token_copy')}</button>
                <span id="ansible-copy-feedback" class="pw-u-copy-success"></span>
            </div>
        </div>`;
    }

    if (sectionBlocked) html += '</div>'; // End feature-unavailable-fields
    html += '</div>';

    document.getElementById('content').innerHTML = html;

    _initArrayChipFields();

    // ── Embeddings: multimodal_format visibility depends on multimodal toggle ──
    if (key === 'embeddings') {
        _embeddingsBindMultimodal();
        applyManagedDockerGuards('embeddings');
        bindEmbeddingsRuntimeActions();
        refreshEmbeddingsRuntimeStatus();
    }
}

/** Wire the multimodal toggle to show/hide the format selector. */
function _embeddingsBindMultimodal() {
    const toggle = document.querySelector('[data-path="embeddings.multimodal"]');
    const formatEl = document.querySelector('[data-path="embeddings.multimodal_format"]');
    const providerEl = document.querySelector('[data-path="embeddings.provider"]');
    if (!toggle || !formatEl) return;
    const formatField = formatEl.closest('.field-group');
    if (!formatField) return;

    function sync() {
        const localGranite = providerEl && providerEl.value === 'local-granite';
        if (localGranite) {
            // Avoid writing the observed class attribute when the toggle is
            // already off. Repeated no-op writes can retrigger the observer
            // indefinitely in Chromium.
            if (toggle.classList.contains('on')) {
                toggle.classList.remove('on');
            }
            if (toggle.nextElementSibling) {
                toggle.nextElementSibling.textContent = t('config.toggle.inactive');
            }
        }
        toggle.dataset.disabled = localGranite ? 'true' : 'false';
        toggle.classList.toggle('cfg-toggle-disabled', localGranite);
        toggle.setAttribute('aria-disabled', localGranite ? 'true' : 'false');
        toggle.title = localGranite ? t('config.embeddings.text_only') : '';
        formatField.style.display = !localGranite && toggle.classList.contains('on') ? '' : 'none';
        syncEmbeddingsRuntimeVisibility(localGranite);
    }
    sync();

    // Observe class changes on the toggle to react to toggleBool()
    new MutationObserver(sync).observe(toggle, { attributes: true, attributeFilter: ['class'] });
    if (providerEl) providerEl.addEventListener('change', sync);
}

function syncEmbeddingsRuntimeVisibility(localGranite) {
    const runtimeCard = document.getElementById('emb-local-runtime-card');
    if (!runtimeCard) return;
    runtimeCard.classList.toggle('is-hidden', !localGranite);
    runtimeCard.setAttribute('aria-hidden', localGranite ? 'false' : 'true');
}

let embeddingsRuntimeRefreshTimer = 0;

function renderEmbeddingsRuntimeBlock() {
    const localGranite = (configData.embeddings || {}).provider === 'local-granite';
    return `<section id="emb-local-runtime-card" class="emb-runtime-card${localGranite ? '' : ' is-hidden'}"
            aria-labelledby="emb-runtime-title" aria-hidden="${localGranite ? 'false' : 'true'}">
        <div class="emb-runtime-header">
            <div>
                <div class="emb-runtime-eyebrow">${t('config.embeddings.runtime_eyebrow')}</div>
                <h3 id="emb-runtime-title">${t('config.embeddings.runtime_title')}</h3>
            </div>
            <span id="emb-runtime-state" class="emb-runtime-state">${t('config.embeddings.status_loading')}</span>
        </div>
        <div id="emb-runtime-progress" class="emb-runtime-progress is-hidden" role="status" aria-live="polite">
            <div class="emb-runtime-progress-track"><span id="emb-runtime-progress-bar"></span></div>
            <span id="emb-runtime-progress-label"></span>
        </div>
        <dl class="emb-runtime-grid">
            <div><dt>${t('config.embeddings.status_model')}</dt><dd id="emb-runtime-model">—</dd></div>
            <div><dt>${t('config.embeddings.status_runtime')}</dt><dd id="emb-runtime-engine">—</dd></div>
            <div><dt>${t('config.embeddings.status_backend')}</dt><dd id="emb-runtime-backend">—</dd></div>
            <div><dt>${t('config.embeddings.status_gpu')}</dt><dd id="emb-runtime-gpu">—</dd></div>
        </dl>
        <div id="emb-runtime-detail" class="emb-runtime-detail" aria-live="polite"></div>
        <div id="emb-runtime-benchmarks" class="emb-runtime-benchmarks"></div>
        <div class="pw-u-inline-actions emb-runtime-actions">
            <button id="embedding-runtime-benchmark-btn" type="button" class="cfg-btn">${t('config.embeddings.benchmark_action')}</button>
        </div>
    </section>
    <div class="pw-u-inline-actions emb-runtime-actions">
        <button id="embedding-runtime-test-btn" type="button" class="cfg-btn cfg-btn-primary">${t('config.embeddings.test_action')}</button>
        <span id="embedding-runtime-action-result" class="pw-u-muted" aria-live="polite"></span>
    </div>`;
}

function bindEmbeddingsRuntimeActions() {
    const actions = window.AuraConfigActions;
    if (!actions) return;
    actions.register('embedding-runtime-test', {
        elementId: 'embedding-runtime-test-btn',
        requiresSaved: true,
        run: testActiveEmbeddingRuntime
    });
    actions.register('embedding-runtime-benchmark', {
        elementId: 'embedding-runtime-benchmark-btn',
        requiresSaved: true,
        run: benchmarkActiveEmbeddingRuntime
    });
}

async function testActiveEmbeddingRuntime() {
    const result = document.getElementById('embedding-runtime-action-result');
    if (result) result.textContent = t('config.embeddings.test_running');
    try {
        const response = await fetch('/api/embeddings/test', { method: 'POST' });
        const data = await response.json();
        if (result) result.textContent = data.message || (response.ok ? t('config.embeddings.test_ok') : t('config.embeddings.test_failed'));
        await refreshEmbeddingsRuntimeStatus();
    } catch (error) {
        if (result) result.textContent = t('config.common.network_error') + ': ' + error.message;
    }
}

async function benchmarkActiveEmbeddingRuntime() {
    const result = document.getElementById('embedding-runtime-action-result');
    if (result) result.textContent = t('config.embeddings.benchmark_running');
    try {
        const response = await fetch('/api/embeddings/benchmark', { method: 'POST' });
        const data = await response.json();
        if (result) result.textContent = response.ok ? t('config.embeddings.benchmark_ok') : (data.message || t('config.embeddings.benchmark_failed'));
        await refreshEmbeddingsRuntimeStatus();
    } catch (error) {
        if (result) result.textContent = t('config.common.network_error') + ': ' + error.message;
    }
}

async function refreshEmbeddingsRuntimeStatus() {
    window.clearTimeout(embeddingsRuntimeRefreshTimer);
    const stateElement = document.getElementById('emb-runtime-state');
    if (!stateElement) return;
    try {
        const response = await fetch('/api/embeddings/status');
        if (!response.ok) throw new Error(await response.text());
        const status = await response.json();
        renderEmbeddingsRuntimeStatus(status);
        if (status.state === 'setting_up' || status.state === 'benchmarking') {
            embeddingsRuntimeRefreshTimer = window.setTimeout(refreshEmbeddingsRuntimeStatus, 1000);
        }
    } catch (error) {
        stateElement.textContent = t('config.embeddings.status_unavailable');
        stateElement.dataset.state = 'error';
        const detail = document.getElementById('emb-runtime-detail');
        if (detail) detail.textContent = error.message;
    }
}

function renderEmbeddingsRuntimeStatus(status) {
    const localRuntime = status.provider === 'local-granite';
    const providerElement = document.querySelector('[data-path="embeddings.provider"]');
    const localSelected = providerElement
        ? providerElement.value === 'local-granite'
        : (configData.embeddings || {}).provider === 'local-granite';
    syncEmbeddingsRuntimeVisibility(localRuntime && localSelected);
    if (!localRuntime || !localSelected) return;

    const setText = (id, value) => {
        const element = document.getElementById(id);
        if (element) element.textContent = value || '—';
    };
    const stateElement = document.getElementById('emb-runtime-state');
    if (stateElement) {
        stateElement.textContent = t('config.embeddings.state_' + String(status.state || 'unavailable'));
        stateElement.dataset.state = status.state || 'unavailable';
    }
    setText('emb-runtime-model', status.model_id);
    setText('emb-runtime-engine', [status.runtime, status.runtime_build].filter(Boolean).join(' '));
    setText('emb-runtime-backend', status.backend);
    setText('emb-runtime-gpu', status.gpu
        ? (status.gpu_verified ? t('config.embeddings.gpu_verified') : t('config.embeddings.gpu_unverified'))
        : t('config.embeddings.cpu_active'));

    const progress = document.getElementById('emb-runtime-progress');
    const progressBar = document.getElementById('emb-runtime-progress-bar');
    const progressLabel = document.getElementById('emb-runtime-progress-label');
    const download = status.download || {};
    const showProgress = Number(download.total) > 0 && Number(download.downloaded) < Number(download.total);
    if (progress) progress.classList.toggle('is-hidden', !showProgress);
    if (progressBar) progressBar.style.width = Math.max(0, Math.min(100, Number(download.percent) || 0)) + '%';
    if (progressLabel) progressLabel.textContent = showProgress
        ? `${download.asset || ''} · ${(Number(download.percent) || 0).toFixed(1)}%`
        : '';

    const detail = document.getElementById('emb-runtime-detail');
    if (detail) {
        detail.textContent = status.error || status.fallback_reason || '';
        detail.classList.toggle('is-error', !!status.error);
    }
    const benchmarks = document.getElementById('emb-runtime-benchmarks');
    if (benchmarks) {
        const rows = Array.isArray(status.benchmark) ? status.benchmark : [];
        benchmarks.innerHTML = rows.length ? `<div class="emb-runtime-benchmark-title">${t('config.embeddings.benchmark_results')}</div>
            <div class="emb-runtime-benchmark-list">${rows.map(row => {
                const outcome = row.skipped ? t('config.embeddings.result_skipped') : (row.valid ? t('config.embeddings.result_valid') : t('config.embeddings.result_failed'));
                const latency = row.latency_ms ? ` · ${Number(row.latency_ms).toFixed(1)} ms` : '';
                return `<div><span>${escapeHtml(row.candidate || '')}</span><span>${escapeHtml(outcome + latency)}</span></div>`;
            }).join('')}</div>` : '';
    }
    const benchmarkButton = document.getElementById('embedding-runtime-benchmark-btn');
    if (benchmarkButton) {
        benchmarkButton.classList.remove('is-hidden');
    }
}

window.addEventListener('cfg:section-leave', () => {
    window.clearTimeout(embeddingsRuntimeRefreshTimer);
});

function _initArrayChipFields() {
    document.querySelectorAll('.cfg-array-chips').forEach((wrap) => {
        const hidden = wrap.querySelector('input[data-path]');
        const row = wrap.querySelector('.cfg-chip-row');
        const input = wrap.querySelector('.cfg-chip-input');
        const addBtn = wrap.querySelector('.cfg-chip-add-btn');
        if (!hidden || !row || !input || !addBtn) return;

        function parse() {
            return (hidden.value || '').split(',').map(s => s.trim()).filter(Boolean);
        }

        function set(values) {
            hidden.value = values.join(', ');
            markDirty();
        }

        function render() {
            const values = parse();
            row.innerHTML = values.map(v => {
                const safe = escapeHtml(v);
                return `<span class="cfg-chip" data-chip="${escapeAttr(v)}">${safe}<button type="button" class="cfg-chip-x" data-chip-x="${escapeAttr(v)}" title="${t('config.field.remove')}">✕</button></span>`;
            }).join('');
        }

        function addCurrent() {
            const v = (input.value || '').trim();
            if (!v) return;
            const values = parse();
            const norm = v.toLowerCase();
            if (!values.some(x => x.toLowerCase() === norm)) {
                values.push(v);
                set(values);
            }
            input.value = '';
            render();
        }

        addBtn.addEventListener('click', addCurrent);
        input.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') {
                e.preventDefault();
                addCurrent();
            }
        });

        wrap.addEventListener('click', (e) => {
            const btn = e.target.closest('.cfg-chip-x');
            if (!btn) return;
            const v = btn.dataset.chipX;
            if (!v) return;
            const values = parse().filter(x => x !== v);
            set(values);
            render();
        });

        render();
    });
}


function renderFields(fields, data, parentPath) {
    let html = '';
    for (const field of fields) {
        const val = data[field.yaml_key];
        const fullPath = parentPath + '.' + field.yaml_key;

        if (field.type === 'object' && field.children) {
            // Use translated group title if available, otherwise fall back to formatKey
            const titleKey = 'config.group_title.' + fullPath;
            const groupTitle = (typeof I18N !== 'undefined' && I18N[titleKey]) ? I18N[titleKey] : formatKey(field.yaml_key);
            html += '<div class="cfg-group-title cfg-group-title-underlined">' + groupTitle + '</div>';
            html += renderFields(field.children, val || {}, fullPath);
        } else {
            html += renderField(fullPath, field.yaml_key, val, parentPath, field);
        }
    }
    return html;
}

/**
 * Returns a feature-unavailable banner HTML if the given feature key is unavailable.
 * featureKey: key from runtimeData.features (e.g. 'docker', 'sandbox', 'firewall')
 * options.blocked: if true, uses a stronger (red) styling
 * Returns empty string if the feature is available or unknown.
 */
function featureUnavailableBanner(featureKey, options) {
    const fa = (runtimeData.features || {})[featureKey];
    if (!fa || fa.available) return '';
    const blocked = options && options.blocked;
    return unavailableReasonBanner(fa.reason || t('config.feature_unavailable'), { blocked });
}

function sectionBlockedReason(sectionKey) {
    const featureKey = SECTION_FEATURE_MAP[sectionKey];
    if (!featureKey) return '';
    const fa = (runtimeData.features || {})[featureKey];
    if (!fa || fa.available) return '';
    return fa.reason || t('config.feature_unavailable');
}

function shouldBlockUnavailableSection(sectionKey) {
    return !NON_BLOCKING_UNAVAILABLE_SECTIONS.has(sectionKey);
}

function unavailableReasonBanner(reason, options) {
    if (!reason) return '';
    const blocked = options && options.blocked;
    const cls = blocked ? 'feature-unavailable-banner fub-blocked' : 'feature-unavailable-banner';
    const icon = blocked ? '🚫' : '⚠️';
    return '<div class="' + cls + '"><span class="fub-icon">' + icon + '</span><span>' + escapeHtml(reason) + '</span></div>';
}

function managedDockerUnavailableReason() {
    const dockerCfg = configData.docker || {};
    if (dockerCfg.enabled !== true) {
        return t('config.docker.managed_disabled_reason');
    }
    const dockerFeature = (runtimeData.features || {}).docker;
    if (dockerFeature && dockerFeature.available === false) {
        return dockerFeature.reason || t('config.docker.socket_missing_reason');
    }
    return '';
}

function applyManagedDockerToggleGuard(fullPath, reason) {
    if (!reason) return;
    const toggle = document.querySelector('[data-path="' + fullPath + '"]');
    if (!toggle) return;

    const group = toggle.closest('.field-group');
    if (group && !group.previousElementSibling?.classList?.contains('managed-docker-unavailable-banner')) {
        const wrapper = document.createElement('div');
        wrapper.className = 'managed-docker-unavailable-banner';
        wrapper.innerHTML = unavailableReasonBanner(reason, { blocked: true });
        group.parentNode.insertBefore(wrapper, group);
    }

    if (toggle.classList.contains('on')) {
        return;
    }
    toggle.classList.add('cfg-toggle-disabled');
    toggle.removeAttribute('onclick');
    toggle.setAttribute('title', reason);
    syncToggleA11y(toggle);
}

function applyManagedDockerGuards(sectionKey) {
    const reason = managedDockerUnavailableReason();
    if (!reason) return;
    if (sectionKey === 'embeddings') {
        applyManagedDockerToggleGuard('embeddings.local_ollama.enabled', reason);
    }
    if (sectionKey === 'ollama') {
        applyManagedDockerToggleGuard('ollama.managed_instance.enabled', reason);
    }
}

/** Returns true if AuraGo is running inside a Docker container. */
function isDockerRuntime() {
    return !!(runtimeData.runtime && runtimeData.runtime.is_docker);
}

function renderField(fullPath, key, value, parentPath, fieldSchema) {
    // Special: server.master_key — locked when vault exists, editable when no vault
    if (fullPath === 'server.master_key') {
        const help = helpTexts[fullPath];
        const helpText = help ? (help[lang] || help['en'] || '') : '';
        let h = '<div class="field-group">';
        h += '<div class="field-label">' + t('config.server.master_key_label') + ' <span class="cfg-sensitive-icon">🔒</span></div>';
        if (helpText) h += '<div class="field-help">' + helpText + '</div>';
        if (vaultExists) {
            h += '<div class="password-wrap">';
            h += '<input class="field-input cfg-master-key-locked-input" type="password" value="\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022" disabled>';
            h += '<button type="button" class="password-toggle cfg-master-key-delete-btn" title="' + t('config.master_key.vault_delete_tooltip') + '" onclick="vaultDeletePrompt()">🗑️</button>';
            h += '</div>';
            h += '<div class="cfg-master-key-note">🔐 ' + t('config.master_key.vault_exists') + '</div>';
        } else {
            h += '<div class="password-wrap">';
            h += '<input class="field-input" type="password" data-path="server.master_key" value="" placeholder="' + t('config.master_key.placeholder') + '"' + cfgNoAutofillAttrs(true) + '>';
            h += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
            h += '</div>';
            h += '<div class="cfg-master-key-note cfg-master-key-note-warning">⚠️ ' + t('config.master_key.no_vault') + '</div>';
        }
        h += '</div>';
        return h;
    }

    const helpKey = fullPath;
    const help = helpTexts[helpKey];
    const helpText = help ? (help[lang] || help['en'] || '') : '';
    const helpOptions = help ? help.options : null;
    const isSensitive = fieldSchema?.sensitive || SENSITIVE_KEYS.includes(key);
    const fieldType = fieldSchema?.type || guessType(value);

    let html = '<div class="field-group">';
    html += '<div class="field-label">' + fieldLabelText(fullPath, key);
    if (isSensitive) html += ' <span class="cfg-sensitive-icon">🔒</span>';
    html += '</div>';
    if (helpText) html += '<div class="field-help">' + helpText + '</div>';

    if (fieldType === 'bool') {
        const isOn = value === true;
        html += '<div class="toggle-wrap">';
        html += '<div class="toggle' + (isOn ? ' on' : '') + '" data-path="' + fullPath + '" onclick="toggleBool(this)"></div>';
        html += '<span class="toggle-label">' + (isOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
        html += '</div>';
    } else if (help && help.personalities_ref) {
        // Dynamic personality profile dropdown — populated from /api/personalities
        html += '<select class="field-select" data-path="' + fullPath + '">';
        const emptyLabel = t('config.field.no_personality');
        const emptySelected = (!value || value === '') ? ' selected' : '';
        html += '<option value=""' + emptySelected + '>' + emptyLabel + '</option>';
        personalitiesCache.forEach(p => {
            const selected = (String(value) === String(p.name)) ? ' selected' : '';
            html += '<option value="' + escapeAttr(p.name) + '"' + selected + '>' + escapeAttr(p.name) + '</option>';
        });
        html += '</select>';
    } else if (help && help.provider_ref) {
        // Dynamic provider dropdown — populated from /api/providers
        html += '<select class="field-select" data-path="' + fullPath + '">';
        if (!help.allow_disabled) {
            const emptyLabel = t('config.field.no_provider');
            const emptySelected = (!value || value === '') ? ' selected' : '';
            html += '<option value=""' + emptySelected + '>' + emptyLabel + '</option>';
        }
        if (help.builtin_options && Array.isArray(help.builtin_options)) {
            help.builtin_options.forEach(option => {
                const selected = (String(value) === String(option)) ? ' selected' : '';
                const label = option === 'local-granite'
                    ? t('config.embeddings.provider_local')
                    : option;
                html += '<option value="' + escapeAttr(option) + '"' + selected + '>' + escapeAttr(label) + '</option>';
            });
        }
        providersCache.forEach(p => {
            const selected = (String(value) === String(p.id)) ? ' selected' : '';
            const displayName = p.name || p.id;
            const badge = p.type ? (' [' + p.type + ']') : '';
            const modelHint = p.model ? (' — ' + p.model) : '';
            html += '<option value="' + escapeAttr(p.id) + '"' + selected + '>' + escapeAttr(displayName + badge + modelHint) + '</option>';
        });
        if (help.allow_disabled) {
            const disSelected = (value === 'disabled') ? ' selected' : '';
            html += '<option value="disabled"' + disSelected + '>' + escapeHtml(cfgFieldOptionLabel('disabled')) + '</option>';
        }
        html += '</select>';
    } else if (helpOptions && Array.isArray(helpOptions)) {
        // Dropdown for fields with predefined options
        const hasCustom = helpOptions.includes(CFG_OPTION_OTHER_CUSTOM);
        const isCustomVal = hasCustom && value && !helpOptions.includes(value) && value !== CFG_OPTION_OTHER_CUSTOM;

        html += '<select class="field-select" data-path="' + fullPath + '" onchange="cfgToggleCustomInput(this)">';
        helpOptions.forEach(opt => {
            const selected = (String(value) === String(opt) || (opt === CFG_OPTION_OTHER_CUSTOM && isCustomVal)) ? ' selected' : '';
            html += '<option value="' + escapeAttr(opt) + '"' + selected + '>' + escapeAttr(cfgFieldOptionLabel(opt)) + '</option>';
        });
        html += '</select>';

        if (hasCustom) {
            const hiddenCls = isCustomVal ? '' : ' is-hidden';
            const customVal = isCustomVal ? value : '';
            html += `<input class="field-input cfg-custom-input${hiddenCls}" type="text" data-custom-for="${fullPath}" value="${escapeAttr(customVal)}" placeholder="${t('config.field.custom_value_placeholder')}"${cfgNoAutofillAttrs(false)} oninput="markDirty(event)">`;
        }
    } else if (isSensitive) {
        const displayVal = cfgSecretValue(value);
        html += '<div class="password-wrap">';
        html += '<input class="field-input" type="password" data-path="' + fullPath + '" value="' + escapeAttr(displayVal) + '" placeholder="' + escapeAttr(cfgSecretPlaceholder(value)) + '"' + cfgNoAutofillAttrs(true) + '>';
        html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
        html += '</div>';
    } else if (fieldType === 'int' || fieldType === 'float') {
        const step = fieldType === 'float' ? '0.01' : '1';
        // Special handling for agent.core_memory_max_entries: show default 80 if missing/0
        let showValue = value;
        if (fullPath.endsWith('agent.core_memory_max_entries')) {
            showValue = (value === undefined || value === null || value === 0 || value === '') ? 80 : value;
        }
        html += '<input class="field-input" type="number" step="' + step + '" data-path="' + fullPath + '" value="' + (showValue ?? '') + '"' + cfgNoAutofillAttrs(false) + '>';
    } else if (fieldType === 'array') {
        // budget.models is now managed in Provider settings
        if (fullPath === 'budget.models') {
            html += '<div class="cfg-budget-models-hint">'
                + '💰 ' + t('config.budget.models_moved_hint')
                + '</div>';
        } else {
        if (fullPath === 'llm.multimodal_provider_types_extra') {
            const arr = Array.isArray(value) ? value : (value ? String(value).split(',').map(s => s.trim()).filter(Boolean) : []);
            const joined = arr.join(', ');
            html += `<div class="cfg-array-chips" data-array-path="${escapeAttr(fullPath)}">
                <div class="cfg-chip-row" data-chip-row="1"></div>
                <div class="cfg-chip-input-row">
                    <input class="field-input cfg-chip-input" type="text" placeholder="${t('config.field.placeholder_example')}"${cfgNoAutofillAttrs(false)}>
                    <button type="button" class="cfg-btn cfg-btn-sm cfg-chip-add-btn" title="${t('config.field.add')}">+</button>
                </div>
                <input class="field-input is-hidden" type="text" data-path="${escapeAttr(fullPath)}" data-type="array" value="${escapeAttr(joined)}"${cfgNoAutofillAttrs(false)}>
            </div>`;
        } else {
            const isObjArray = (Array.isArray(value) && value.length > 0 && typeof value[0] === 'object' && value[0] !== null);
            if (isObjArray) {
                const jsonVal = Array.isArray(value) ? JSON.stringify(value, null, 2) : '[]';
                html += '<textarea class="field-input cfg-json-array-input" data-path="' + fullPath + '" data-type="json" rows="6">' + escapeHtml(jsonVal) + '</textarea>';
                html += '<div class="cfg-json-array-hint">' + t('config.field.json_array_hint') + '</div>';
            } else {
                const arrVal = Array.isArray(value) ? value.join(', ') : (value || '');
                html += '<input class="field-input" type="text" data-path="' + fullPath + '" data-type="array" value="' + escapeAttr(arrVal) + '" placeholder="' + t('config.field.comma_separated') + '"' + cfgNoAutofillAttrs(false) + '>';
            }
        }
        }
    } else {
        html += '<input class="field-input" type="text" data-path="' + fullPath + '" value="' + escapeAttr(value ?? '') + '"' + cfgNoAutofillAttrs(false) + '>';
    }
    html += '</div>';
    return html;
}

function fieldLabelText(fullPath, key) {
    const labelKey = 'config.' + fullPath + '_label';
    const translated = t(labelKey);
    if (typeof translated === 'string' && translated.trim() !== '' && translated !== labelKey && translated !== '-') {
        return translated;
    }
    return formatKey(key);
}

/** Set a deep value on an object using a dot-separated path, e.g. "tools.web_scraper.enabled". */
function setNestedValue(obj, path, value) {
    const parts = path.split('.');
    let cur = obj;
    for (let i = 0; i < parts.length - 1; i++) {
        if (cur[parts[i]] === undefined || cur[parts[i]] === null || typeof cur[parts[i]] !== 'object') {
            cur[parts[i]] = {};
        }
        cur = cur[parts[i]];
    }
    cur[parts[parts.length - 1]] = value;
}

function toggleBool(el) {
    if (el.dataset.disabled === 'true' || el.getAttribute('aria-disabled') === 'true') return;
    const on = el.classList.toggle('on');
    if (el.nextElementSibling) {
        el.nextElementSibling.textContent = on ? t('config.toggle.active') : t('config.toggle.inactive');
    }
    syncToggleA11y(el);
    markDirty();
}

function togglePassword(btn) {
    const input = btn.closest('.password-wrap').querySelector('.field-input');
    const isVisible = btn.dataset.visible === 'true';
    if (isVisible) {
        input.type = 'password';
        btn.innerHTML = EYE_OPEN_SVG;
        btn.dataset.visible = 'false';
    } else {
        input.type = 'text';
        btn.innerHTML = EYE_CLOSED_SVG;
        btn.dataset.visible = 'true';
    }
}

// ── Dirty tracking ──────────────────────────────────
function collectSnapshot() {
    const parts = [];
    document.querySelectorAll('[data-path]').forEach(el => {
        const path = el.dataset.path;
        let val;
        if (el.classList.contains('toggle')) {
            val = el.classList.contains('on') ? 'true' : 'false';
        } else {
            val = el.value;
        }
        parts.push(path + '=' + val);
    });
    return parts.join('|');
}

function clearDirtyBaselineRefreshTimers() {
    dirtyBaselineRefreshTimers.forEach(timer => clearTimeout(timer));
    dirtyBaselineRefreshTimers = [];
}

function refreshDirtyBaselineIfIdle() {
    if (userEditedSinceSnapshot) return;
    // AuraConfigState tracks draft paths independently; never re-baseline over real edits.
    if (window.AuraConfigState && window.AuraConfigState.isDirty()) return;
    initialSnapshot = collectSnapshot();
    setDirty(false);
}

function scheduleDirtyBaselineRefresh(delayMs = DIRTY_BASELINE_REFRESH_DELAY_MS) {
    const timer = setTimeout(() => {
        dirtyBaselineRefreshTimers = dirtyBaselineRefreshTimers.filter(t => t !== timer);
        refreshDirtyBaselineIfIdle();
    }, delayMs);
    dirtyBaselineRefreshTimers.push(timer);
}

function scheduleDirtyBaselineSettling() {
    clearDirtyBaselineRefreshTimers();
    DIRTY_BASELINE_SETTLE_DELAYS_MS.forEach(delay => scheduleDirtyBaselineRefresh(delay));
    const finalDelay = DIRTY_BASELINE_SETTLE_DELAYS_MS[DIRTY_BASELINE_SETTLE_DELAYS_MS.length - 1] || 0;
    const timer = setTimeout(() => {
        dirtyBaselineRefreshTimers = dirtyBaselineRefreshTimers.filter(t => t !== timer);
        if (!userEditedSinceSnapshot) suppressDirtyTracking = false;
    }, finalDelay + 50);
    dirtyBaselineRefreshTimers.push(timer);
}

function resetDirtySnapshot() {
    suppressDirtyTracking = true;
    userEditedSinceSnapshot = false;
    initialSnapshot = collectSnapshot();
    setDirty(false);
    scheduleDirtyBaselineSettling();
}

function closestConfigEditable(target) {
    if (!target || typeof target.closest !== 'function') return null;
    return target.closest('.field-input, .field-select, .field-textarea, .cfg-input, .toggle, [data-path]');
}

function noteConfigEditIntent(event) {
    if (!event || event.isTrusted !== true || !closestConfigEditable(event.target)) return;
    if (event.type === 'keydown' && ['Tab', 'Shift', 'Control', 'Alt', 'Meta', 'Escape'].includes(event.key)) return;
    configEditIntentUntil = Date.now() + CONFIG_EDIT_INTENT_WINDOW_MS;
}

function installConfigEditIntentTracking() {
    if (configEditIntentTrackingInstalled) return;
    configEditIntentTrackingInstalled = true;
    ['beforeinput', 'keydown', 'pointerdown', 'paste', 'drop'].forEach(type => {
        document.addEventListener(type, noteConfigEditIntent, true);
    });
}

/** True for trusted commits that never set inputType (selects, checkboxes, radios, ranges). */
function isTrustedDiscreteControlChange(event) {
    if (!event || (event.type !== 'change' && event.type !== 'input')) return false;
    const el = event.target;
    if (!el || !closestConfigEditable(el)) return false;
    const tag = String(el.tagName || '').toUpperCase();
    if (tag === 'SELECT') return true;
    if (tag === 'INPUT') {
        const type = String(el.type || 'text').toLowerCase();
        return type === 'checkbox' || type === 'radio' || type === 'range' || type === 'file' || type === 'color';
    }
    return false;
}

function isUserInitiatedConfigChange(event) {
    if (!event) return true;
    if (event.isTrusted !== true) return false;
    // Recent pointer/keyboard intent always wins (covers typing during settle windows).
    if (Date.now() <= configEditIntentUntil) return true;
    // Native <select> change events have no inputType. Users often keep the option
    // list open longer than CONFIG_EDIT_INTENT_WINDOW_MS before committing.
    if (isTrustedDiscreteControlChange(event)) return true;
    if (suppressDirtyTracking) return false;
    return !!(event.inputType && event.inputType !== 'insertReplacementText');
}

function getNestedValue(obj, path) {
    if (!obj || !path) return undefined;
    return path.split('.').reduce((cur, part) => (cur && cur[part] !== undefined) ? cur[part] : undefined, obj);
}

function isNumericPathPart(part) {
    return typeof part === 'string' && /^(0|[1-9]\d*)$/.test(part);
}

function buildConfigPatchFromForm() {
    const patch = {};
    const forbidden = new Set(['__proto__', 'constructor', 'prototype']);
    document.querySelectorAll('[data-path]').forEach(el => {
        if (el.type === 'radio' && !el.checked) return;

        const path = el.dataset.path;
        const parts = path.split('.');
        let val;

        if (el.classList.contains('toggle')) {
            val = el.classList.contains('on');
        } else if (el.type === 'number' || el.type === 'range') {
            val = el.value === '' ? 0 : (el.step && parseFloat(el.step) < 1 ? parseFloat(el.value) : parseInt(el.value));
        } else if (el.dataset.type === 'array') {
            if (path === 'budget.models' || el.value.trim().startsWith('[')) {
                try { val = JSON.parse(el.value); } catch (e) {
                    val = el.value.split(',').map(s => s.trim()).filter(Boolean);
                }
            } else {
                val = el.value.split(',').map(s => s.trim()).filter(Boolean);
            }
        } else if (el.dataset.type === 'array-lines') {
            val = el.value.split('\n').map(s => s.trim()).filter(Boolean);
        } else if (el.dataset.type === 'json') {
            try { val = JSON.parse(el.value); } catch (e) { val = el.value; }
        } else if (el.tagName === 'SELECT' && el.value === CFG_OPTION_OTHER_CUSTOM) {
            const customInput = document.querySelector('[data-custom-for="' + path + '"]');
            val = customInput ? customInput.value.trim() : el.value;
        } else {
            val = el.value;
        }

        let obj = patch;
        for (let i = 0; i < parts.length - 1; i++) {
            const part = parts[i];
            const nextPart = parts[i + 1];
            if (forbidden.has(part)) return; // Prototype pollution guard
            if (Array.isArray(obj)) {
                const idx = parseInt(part, 10);
                if (!Number.isInteger(idx) || idx < 0) return;
                if (!obj[idx]) obj[idx] = isNumericPathPart(nextPart) ? [] : {};
                obj = obj[idx];
                continue;
            }
            if (!obj[part]) obj[part] = isNumericPathPart(nextPart) ? [] : {};
            obj = obj[part];
        }
        const lastKey = parts[parts.length - 1];
        if (forbidden.has(lastKey)) return;
        if (Array.isArray(obj) && isNumericPathPart(lastKey)) {
            obj[parseInt(lastKey, 10)] = val;
        } else {
            obj[lastKey] = val;
        }
    });
    return patch;
}

function embeddingsConfigWillLikelyChange(patch) {
    const nextEmbeddings = patch && patch.embeddings;
    if (!nextEmbeddings) return false;

    const current = configData.embeddings || {};
    const currentGranite = current.local || {};
    const nextGranite = nextEmbeddings.local || {};
    const currentLocal = current.local_ollama || {};
    const nextLocal = nextEmbeddings.local_ollama || {};

    const comparePaths = [
        ['provider', current.provider, nextEmbeddings.provider],
        ['internal_model', current.internal_model, nextEmbeddings.internal_model],
        ['external_url', current.external_url, nextEmbeddings.external_url],
        ['external_model', current.external_model, nextEmbeddings.external_model],
        ['multimodal', current.multimodal, nextEmbeddings.multimodal],
        ['multimodal_format', current.multimodal_format, nextEmbeddings.multimodal_format],
        ['local.backend', currentGranite.backend, nextGranite.backend],
        ['local.context_size', currentGranite.context_size, nextGranite.context_size],
        ['local.batch_size', currentGranite.batch_size, nextGranite.batch_size],
        ['local_ollama.enabled', currentLocal.enabled, nextLocal.enabled],
        ['local_ollama.model', currentLocal.model, nextLocal.model],
        ['local_ollama.container_port', currentLocal.container_port, nextLocal.container_port],
        ['local_ollama.use_host_gpu', currentLocal.use_host_gpu, nextLocal.use_host_gpu],
        ['local_ollama.gpu_backend', currentLocal.gpu_backend, nextLocal.gpu_backend]
    ];

    return comparePaths.some(([, before, after]) => JSON.stringify(before) !== JSON.stringify(after));
}

function showEmbeddingsResetModal(options = {}) {
    const { postSave = false } = options;
    return new Promise(resolve => {
        const existing = document.getElementById('embeddings-reset-modal');
        if (existing) existing.remove();

        const modal = document.createElement('div');
        modal.id = 'embeddings-reset-modal';
        modal.className = 'sec-modal-overlay';
        modal.innerHTML = `<div class="sec-modal-panel emb-modal-panel">
            <div class="sec-modal-title emb-modal-title">🧠 ${t('config.embeddings.reset_title')}</div>
            <div class="sec-modal-desc">${t(postSave ? 'config.embeddings.reset_desc_postsave' : 'config.embeddings.reset_desc')}</div>
            <ul class="sec-modal-list">
                <li class="sec-modal-item">${t('config.embeddings.reset_point_vectors')}</li>
                <li class="sec-modal-item">${t('config.embeddings.reset_point_rebuild')}</li>
                <li class="sec-modal-item">${t('config.embeddings.reset_point_memories')}</li>
            </ul>
            <div class="sec-modal-actions">
                <button id="embeddings-reset-cancel" class="sec-modal-btn sec-modal-btn-skip">${t('config.embeddings.reset_cancel')}</button>
                <button id="embeddings-reset-continue" class="sec-modal-btn emb-modal-btn-danger">${t('config.embeddings.reset_continue')}</button>
            </div>
        </div>`;

        function close(result) {
            modal.remove();
            resolve(result);
        }

        modal.addEventListener('click', e => {
            if (e.target === modal) close(false);
        });
        modal.querySelector('#embeddings-reset-cancel').addEventListener('click', () => close(false));
        modal.querySelector('#embeddings-reset-continue').addEventListener('click', () => close(true));
        document.body.appendChild(modal);
    });
}

async function scheduleEmbeddingsReset() {
    const resp = await fetch('/api/embeddings/reset', { method: 'POST' });
    let data = {};
    try { data = await resp.json(); } catch (_) { }
    return { ok: resp.ok, data };
}

function markDirty(event) {
    if (!isUserInitiatedConfigChange(event)) {
        if (!userEditedSinceSnapshot) scheduleDirtyBaselineRefresh();
        return;
    }
    userEditedSinceSnapshot = true;
    suppressDirtyTracking = false;
    clearDirtyBaselineRefreshTimers();
    const dirty = collectSnapshot() !== initialSnapshot;
    setDirty(dirty);
}

function setDirty(dirty) {
    if (dirty) {
        // Direct callers (inline onchange, section modules) must pin the draft so
        // delayed baseline refresh timers cannot clear a real user edit.
        userEditedSinceSnapshot = true;
        suppressDirtyTracking = false;
        clearDirtyBaselineRefreshTimers();
        if (window.AuraConfigState) {
            window.AuraConfigState.syncFromDOM();
            dirty = window.AuraConfigState.isDirty();
        }
    } else if (window.AuraConfigState && window.AuraConfigState.isDirty()) {
        // Never hide the save bar while the draft still has changes.
        dirty = true;
        userEditedSinceSnapshot = true;
        clearDirtyBaselineRefreshTimers();
    }
    isDirty = dirty;
    const btn = document.getElementById('btnSave');
    const pill = document.getElementById('changesPill');
    const restartBtn = document.getElementById('cfg-restart-btn');
    const changeCount = document.getElementById('saveChangeCount');
    const validation = document.getElementById('saveValidation');
    const count = window.AuraConfigState ? window.AuraConfigState.dirtyPaths().length : (dirty ? 1 : 0);
    if (btn) btn.disabled = !dirty || configSaveInFlight;
    if (pill) pill.classList.toggle('visible', dirty);
    if (changeCount) {
        changeCount.textContent = t('config.precision.changed_fields').replace('{count}', String(count));
        changeCount.classList.toggle('visible', dirty);
    }
    if (validation) {
        validation.textContent = dirty ? t('config.precision.validation_ready') : '';
        validation.className = 'pw-save-validation' + (dirty ? ' visible' : '');
    }
    if (restartBtn) {
        restartBtn.setAttribute('aria-disabled', dirty ? 'true' : 'false');
        restartBtn.title = dirty ? t('config.precision.restart_save_first') : t('config.header.restart_tooltip');
    }
}

function updateSaveDockSection(key) {
    const element = document.getElementById('saveSection');
    if (!element) return;
    const section = sectionMetadata(key);
    element.textContent = section ? section.label : t('config.precision.overview_title');
}

function updateSaveDockValidation(valid) {
    const element = document.getElementById('saveValidation');
    if (!element) return;
    element.textContent = t(valid ? 'config.precision.validation_valid' : 'config.precision.validation_invalid');
    element.className = 'pw-save-validation visible ' + (valid ? 'is-valid' : 'is-invalid');
}

function clearConfigValidation() {
    document.querySelectorAll('.pw-field-error[data-config-validation]').forEach(element => element.remove());
    document.querySelectorAll('[aria-invalid="true"][data-path]').forEach(element => {
        element.removeAttribute('aria-invalid');
        const describedBy = (element.getAttribute('aria-describedby') || '').split(/\s+/).filter(id => id && !id.endsWith('-validation'));
        if (describedBy.length) element.setAttribute('aria-describedby', describedBy.join(' '));
        else element.removeAttribute('aria-describedby');
    });
}

function showConfigValidationErrors(errors) {
    clearConfigValidation();
    let first = null;
    (errors || []).forEach(error => {
        const selector = '[data-path="' + CSS.escape(error.path || '') + '"]';
        const control = document.querySelector(selector);
        if (!control) return;
        const id = (control.id || 'cfg-' + (error.path || '').replace(/[^a-z0-9_-]/gi, '-')) + '-validation';
        const message = document.createElement('div');
        message.id = id;
        message.className = 'pw-field-error';
        message.dataset.configValidation = 'true';
        const translated = t('config.precision.validation_' + error.code);
        message.textContent = translated && translated !== 'config.precision.validation_' + error.code ? translated : error.message;
        control.setAttribute('aria-invalid', 'true');
        const describedBy = new Set((control.getAttribute('aria-describedby') || '').split(/\s+/).filter(Boolean));
        describedBy.add(id);
        control.setAttribute('aria-describedby', Array.from(describedBy).join(' '));
        control.insertAdjacentElement('afterend', message);
        if (!first) first = control;
    });
    if (first) first.focus();
}

function setConfigSaveBusy(busy) {
    const btn = document.getElementById('btnSave');
    if (btn) btn.disabled = busy || !isDirty;
}

function attachChangeListeners() {
    // Include data-path controls even when modules omit field-* classes (e.g. indexing selects).
    document.querySelectorAll('.field-input, .field-select, .field-textarea, .cfg-input, [data-path]').forEach(el => {
        if (el.dataset.cfgDirtyBound === 'true') return;
        el.dataset.cfgDirtyBound = 'true';
        el.addEventListener('input', markDirty);
        el.addEventListener('change', markDirty);
    });
    enhanceConfigControls();
}

// ── Save ────────────────────────────────────────────
async function saveConfig() {
    if (configSaveInFlight) return;
    if (window.AuraConfigState) {
        window.AuraConfigState.syncFromDOM();
        const validation = window.AuraConfigState.validate();
        if (!validation.valid) {
            showConfigValidationErrors(validation.errors);
            updateSaveDockValidation(false);
            return false;
        }
        updateSaveDockValidation(true);
    }
    configSaveInFlight = true;
    let saveSucceeded = false;
    const status = document.getElementById('saveStatus');
    setConfigSaveBusy(true);
    status.className = 'save-status';
    status.textContent = t('config.save_bar.saving');

    try {
        clearConfigValidation();
        const patch = window.AuraConfigState ? window.AuraConfigState.buildPatch() : buildConfigPatchFromForm();
        const likelyEmbeddingsChange = embeddingsConfigWillLikelyChange(patch);
        let scheduleResetAfterSave = false;

        if (likelyEmbeddingsChange) {
            const wantsReset = await showEmbeddingsResetModal();
            if (!wantsReset) {
                status.className = 'save-status warning';
                status.textContent = '⚠ ' + t('config.embeddings.reset_cancelled');
                setTimeout(() => { status.textContent = ''; }, 5000);
                return false;
            }
            if (!await showConfirm(t('config.embeddings.reset_confirm_final'))) {
                status.className = 'save-status warning';
                status.textContent = '⚠ ' + t('config.embeddings.reset_cancelled');
                setTimeout(() => { status.textContent = ''; }, 5000);
                return false;
            }
            scheduleResetAfterSave = true;
        }

        const resp = await fetch('/api/config', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(patch)
        });
        const result = await resp.json();
        if (resp.ok) {
            const embeddingsChanged = result.embeddings_changed === true;
            let shouldScheduleResetNow = scheduleResetAfterSave;

            if (embeddingsChanged && !shouldScheduleResetNow) {
                const wantsReset = await showEmbeddingsResetModal({ postSave: true });
                if (wantsReset && await showConfirm(t('config.embeddings.reset_confirm_final'))) {
                    shouldScheduleResetNow = true;
                }
            }

            if (shouldScheduleResetNow) {
                const resetResult = await scheduleEmbeddingsReset();
                if (!resetResult.ok) {
                    status.className = 'save-status error';
                    status.textContent = '✗ ' + (resetResult.data.message || t('config.embeddings.reset_error'));
                    setTimeout(() => { status.textContent = ''; }, 7000);
                    return false;
                }
            }

            if (result.needs_restart) {
                status.className = 'save-status warning';
                status.textContent = '⚠ ' + (result.message || t('config.save_bar.restart_needed'));
            } else {
                status.className = 'save-status success';
                status.textContent = '✓ ' + (result.message || t('config.save_bar.saved'));
            }
            // Refresh config data and reset dirty state
            // Use a retry loop so a briefly-restarting tunnel (e.g. Cloudflare hot-reload)
            // doesn't cause an HTML response to be parsed as JSON.
            for (let _i = 0; _i < 4; _i++) {
                try {
                    const cfgResp = await fetch('/api/config');
                    if (cfgResp.ok) {
                        const ct = cfgResp.headers.get('content-type') || '';
                        if (ct.includes('json')) {
                            configData = await cfgResp.json();
                            break;
                        }
                    }
                } catch (_) { /* tunnel restarting, retry */ }
                await new Promise(r => setTimeout(r, 800));
            }
            if (window.AuraConfigState) window.AuraConfigState.commit(configData);
            resetDirtySnapshot();
            saveSucceeded = true;
            // Check for security issues introduced by this save
            checkSecurityAfterSave();
            if (shouldScheduleResetNow) {
                status.className = 'save-status warning';
                status.textContent = '⚠ ' + t('config.embeddings.reset_restarting');
                await restartAuraGo(true);
                return true;
            }
            if (embeddingsChanged) {
                status.className = 'save-status warning';
                status.textContent = '⚠ ' + t('config.embeddings.reset_pending_warning');
            }
        } else {
            if (Array.isArray(result.field_errors) && result.field_errors.length) {
                showConfigValidationErrors(result.field_errors);
                updateSaveDockValidation(false);
            }
            status.className = 'save-status error';
            status.textContent = '✗ ' + (result.message || t('config.save_bar.error'));
        }
        setTimeout(() => { status.textContent = ''; }, 5000);
    } catch (e) {
        status.className = 'save-status error';
        status.textContent = '✗ ' + e.message;
        setTimeout(() => { status.textContent = ''; }, 5000);
    } finally {
        configSaveInFlight = false;
        setConfigSaveBusy(false);
    }
    return saveSucceeded;
}

// ── Post-save security check ────────────────────────────────────────────────
// Runs silently after any successful config save. If auto-fixable critical
// security issues are detected, a modal prompts the user to apply them.
async function checkSecurityAfterSave() {
    try {
        const resp = await fetch('/api/security/hints');
        if (!resp.ok) return;
        const data = await resp.json();
        const hints = (data.hints || []);
        const critFixable = hints.filter(h => h.severity === 'critical' && h.auto_fixable);
        if (!critFixable.length) return;
        showSecurityModal(critFixable);
    } catch (_) { /* silent */ }
}

function showSecurityModal(critFixable) {
    // Remove any existing modal first
    const existing = document.getElementById('sec-harden-modal');
    if (existing) existing.remove();

    const ids = critFixable.map(h => h.id);
    const itemsHtml = critFixable.map(h =>
        `<li class="sec-modal-item">⚠ ${esc(h.title)}</li>`
    ).join('');

    const modal = document.createElement('div');
    modal.id = 'sec-harden-modal';
    modal.className = 'sec-modal-overlay';
    modal.innerHTML = `<div class="sec-modal-panel">
        <div class="sec-modal-title">🔒 ${t('config.security.modal.title')}</div>
        <div class="sec-modal-desc">${t('config.security.modal.desc')}</div>
        <ul class="sec-modal-list">${itemsHtml}</ul>
        <div class="sec-modal-actions">
            <button id="sec-modal-skip" class="sec-modal-btn sec-modal-btn-skip">
                ${t('config.security.modal.later')}
            </button>
            <button id="sec-modal-apply" class="sec-modal-btn sec-modal-btn-apply">
                🔧 ${t('config.security.modal.apply')}
            </button>
        </div>
    </div>`;

    document.body.appendChild(modal);

    modal.querySelector('#sec-modal-skip').addEventListener('click', () => modal.remove());
    modal.querySelector('#sec-modal-apply').addEventListener('click', async () => {
        const applyBtn = modal.querySelector('#sec-modal-apply');
        applyBtn.disabled = true;
        applyBtn.textContent = t('config.security.applying');
        try {
            const resp = await fetch('/api/security/harden', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ ids })
            });
            modal.remove();
            if (resp.ok) {
                // Refresh UI for the currently visible section
                if (typeof renderWebConfigSection === 'function') {
                    const active = document.querySelector('.sidebar-item.active');
                    if (active && active.dataset.key === 'web_config') {
                        selectSection('web_config');
                    }
                }
            }
        } catch (_) { modal.remove(); }
    });
}

function setRestartBusy(busy) {
    const restartBtn = document.getElementById('cfg-restart-btn');
    if (restartBtn) restartBtn.disabled = busy;
}

function showRestartScreen(message, description) {
    const screen = document.createElement('div');
    screen.className = 'cfg-restart-screen';
    const spinner = document.createElement('div');
    spinner.className = 'cfg-restart-spinner';
    spinner.textContent = '↻';
    const title = document.createElement('h2');
    title.className = 'cfg-restart-title';
    title.textContent = message || '';
    const desc = document.createElement('p');
    desc.className = 'cfg-restart-desc';
    desc.textContent = description || '';
    screen.append(spinner, title, desc);
    document.body.replaceChildren(screen);
}

function showRestartDisconnected(message) {
    const node = document.createElement('div');
    node.className = 'cfg-restart-disconnected';
    node.textContent = message || '';
    document.body.replaceChildren(node);
}

async function restartAuraGo(skipConfirm = false) {
    if (restartInFlight) return;
    if (hasUnsavedConfigChanges()) {
        await showAlert(t('config.precision.restart_save_first'));
        return;
    }
    restartInFlight = true;

    try {
        if (!skipConfirm && !await showConfirm(t('config.restart.confirm'))) {
            restartInFlight = false;
            return;
        }
        setRestartBusy(true);
        const resp = await fetch('/api/restart', { method: 'POST' });
        if (resp.ok) {
            const res = await resp.json();
            showRestartScreen(res.message, t('config.restart.reloading'));
            // Wait for server to restart, then reload with retry logic
            scheduleReloadWithRetry(8000);
        } else {
            await showAlert(t('config.restart.error'));
            restartInFlight = false;
            setRestartBusy(false);
        }
    } catch (e) {
        // If the fetch fails immediately, it might be that the server died instantly.
        // We'll still show the reloading screen.
        showRestartDisconnected(t('config.restart.disconnected'));
        scheduleReloadWithRetry(5000);
    }
}

let _reloadRetryTimer = null;
let _reloadRetryCount = 0;
const MAX_RELOAD_RETRIES = 10;

function scheduleReloadWithRetry(delayMs) {
    if (_reloadRetryTimer) return;
    _reloadRetryTimer = setTimeout(function attemptReload() {
        const img = new Image();
        img.onload = img.onerror = function () {
            // Small image load test - if it works, server is back
            _reloadRetryCount = 0;
            _reloadRetryTimer = null;
            window.location.reload();
        };
        img.src = '/favicon.ico?t=' + Date.now(); // Cache-bust
        // If server not back yet, retry after increasing delay
        _reloadRetryCount++;
        if (_reloadRetryCount < MAX_RELOAD_RETRIES) {
            // Exponential backoff: 8s, 12s, 18s, 27s, 40s, 60s, 90s, ...
            const nextDelay = Math.min(delayMs * 1.5, 120000);
            delayMs = nextDelay;
            _reloadRetryTimer = setTimeout(attemptReload, nextDelay);
        }
    }, delayMs);
}

// Boot

/* ── esc() is now provided by shared.js ── */

/* ── Lazy module loader ── */
const _moduleCache = {};
const SECTION_MODULES = {
    heartbeat: { m: 'heartbeat', fn: 'renderHeartbeatSection' },
    providers: { m: 'providers', fn: 'renderProvidersSection' },
    realtime_speech: { m: 'realtime_speech', fn: 'renderRealtimeSpeechSection' },
    manifest: { m: 'manifest', fn: 'renderManifestSection' },
    omniroute: { m: 'omniroute', fn: 'renderOmniRouteSection' },
    dograh: { m: 'dograh', fn: 'renderDograhSection' },
    email: { m: 'email', fn: 'renderEmailSection' },
    agentmail: { m: 'agentmail', fn: 'renderAgentMailSection' },
    mcp: { m: 'mcp', fn: 'renderMCPSection' },
    sandbox: { m: 'sandbox', fn: 'renderSandboxSection' },
    web_scraper: { m: 'scraper', fn: 'renderWebScraperSection' },
    browser_automation: { m: 'browser_automation', fn: 'renderBrowserAutomationSection' },
    space_agent: { m: 'space_agent', fn: 'renderSpaceAgentSection' },
    virtual_desktop: { m: 'virtual_desktop', fn: 'renderVirtualDesktopSection' },
    virtual_computers: { m: 'virtual_computers', fn: 'renderVirtualComputersSection' },
    media_conversion: { m: 'media_conversion', fn: 'renderMediaConversionSection' },
    video_download: { m: 'video_download', fn: 'renderVideoDownloadSection' },
    webhooks: { m: 'webhooks', fn: 'renderWebhooksSection' },
    prompts_editor: { m: 'prompts', fn: 'renderPromptsSection' },
    rules: { m: 'rules', fn: 'renderRulesSection' },
    indexing: { m: 'indexing', fn: 'renderIndexingSection' },
    backup_restore: { m: 'backup', fn: 'renderBackupSection' },
    updates: { m: 'updates', fn: 'renderUpdatesSection' },
    chromecast: { m: 'chromecast', fn: 'renderChromecastSection' },
    bluetooth: { m: 'bluetooth', fn: 'renderBluetoothSection' },
    adguard: { m: 'adguard', fn: 'renderAdGuardSection' },
    uptime_kuma: { m: 'uptime_kuma', fn: 'renderUptimeKumaSection' },
    grafana: { m: 'grafana', fn: 'renderGrafanaSection' },
    three_d_printers: { m: 'three_d_printers', fn: 'renderThreeDPrintersSection' },
    fritzbox: { m: 'fritzbox', fn: 'renderFritzBoxSection' },
    ldap: { m: 'ldap', fn: 'renderLDAPSection' },
    webdav: { m: 'webdav', fn: 'renderWebDAVSection' },
    koofr: { m: 'koofr', fn: 'renderKoofrSection' },
    telnyx: { m: 'telnyx', fn: 'renderTelnyxSection' },
    paperless_ngx: { m: 'paperless', fn: 'renderPaperlessSection' },
    homepage: { m: 'homepage', fn: 'renderHomepageSection' },
    netlify: { m: 'netlify', fn: 'renderNetlifySection' },
    vercel: { m: 'vercel', fn: 'renderVercelSection' },
    danger_zone: { m: 'danger', fn: 'renderDangerZoneSection' },
    truenas: { m: 'truenas', fn: 'renderTrueNASSection' },
    jellyfin: { m: 'jellyfin', fn: 'renderJellyfinSection' },
    obsidian: { m: 'obsidian', fn: 'renderObsidianSection' },
    web_config: { m: 'auth', fn: 'renderWebConfigSection' },
    firewall: { m: 'firewall', fn: 'renderFirewallSection' },
    github: { m: 'github', fn: 'renderGitHubSection' },
    google_workspace: { m: 'google_workspace', fn: 'renderGoogleWorkspaceSection' },
    ai_gateway: { m: 'ai_gateway', fn: 'renderAIGatewaySection' },
    composio: { m: 'composio', fn: 'renderComposioSection' },
    manus: { m: 'manus', fn: 'renderManusSection' },
    huggingface: { m: 'huggingface', fn: 'renderHuggingFaceSection' },
    evomap: { m: 'evomap', fn: 'renderEvomapSection' },
    cloudflare_tunnel: { m: 'cloudflare_tunnel', fn: 'renderCloudflareTunnelSection' },
    mcp_server: { m: 'mcp_server', fn: 'renderMCPServerSection' },
    image_generation: { m: 'image_generation', fn: 'renderImageGenerationSection' },
    music_generation: { m: 'music_generation', fn: 'renderMusicGenerationSection' },
    video_generation: { m: 'video_generation', fn: 'renderVideoGenerationSection' },
    remote_control: { m: 'remote_control', fn: 'renderRemoteControlSection' },
    security_proxy: { m: 'security_proxy', fn: 'renderSecurityProxySection' },
    memory_analysis: { m: 'memory_analysis', fn: 'renderMemoryAnalysisSection' },
    guardian: { m: 'guardian', fn: 'renderGuardianSection' },
    llm_guardian: { m: 'llm_guardian', fn: 'renderLLMGuardianSection' },
    output_compression: { m: 'output_compression', fn: 'renderOutputCompressionSection' },
    document_creator: { m: 'document_creator', fn: 'renderDocumentCreatorSection' },
    tailscale: { m: 'tailscale', fn: 'renderTailscaleSection' },
    server: { m: 'server', fn: 'renderServerSection' },
    a2a: { m: 'a2a', fn: 'renderA2ASection' },
    tts: { m: 'tts', fn: 'renderTTSSection' },
    co_agents: { m: 'co_agents', fn: 'renderCoAgentsSection' },
    sql_connections: { m: 'sql_connections', fn: 'renderSQLConnectionsSection' },
    skill_manager: { m: 'skill_manager', fn: 'renderSkillManagerSection' },
    daemon_skills: { m: 'daemon_skills', fn: 'renderDaemonSkillsSection' },
    ollama: { m: 'ollama', fn: 'renderOllamaSection' },
    mission_preparation: { m: 'mission_preparation', fn: 'renderMissionPreparationSection' },
    mqtt: { m: 'mqtt', fn: 'renderMQTTSection' },
    yepapi: { m: 'yepapi', fn: 'renderYepAPISection' }
};

function loadModule(name) {
    if (_moduleCache[name]) return _moduleCache[name];
    _moduleCache[name] = new Promise((resolve, reject) => {
        const s = document.createElement('script');
        s.src = '/cfg/' + name + '.js?v=' + encodeURIComponent(CONFIG_ASSET_VERSION);
        s.onload = resolve;
        s.onerror = () => reject(new Error('Failed to load module: ' + name));
        document.head.appendChild(s);
    });
    return _moduleCache[name];
}

window.addEventListener('beforeunload', handleConfigBeforeUnload);
window.addEventListener('hashchange', handleConfigHashChange);

init();

/* ── Vault delete functions (core — used by vault modal HTML + renderField) ── */
function vaultDeletePrompt() {
    const word = t('config.vault.confirm_word_de');
    document.getElementById('vault-confirm-word').textContent = word;
    document.getElementById('vault-confirm-input').placeholder = word;
    document.getElementById('vault-confirm-input').value = '';
    document.getElementById('vault-confirm-btn').disabled = true;
    document.getElementById('modal-title').textContent = t('config.vault.delete_title');
    document.getElementById('vault-modal-desc').innerHTML = t('config.vault.delete_desc');
    document.getElementById('vault-confirm-btn').textContent = t('config.vault.destroy_button');
    document.getElementById('vault-cancel-btn').textContent = t('config.vault.cancel');
    document.getElementById('vault-delete-overlay').classList.add('active');
    setTimeout(() => document.getElementById('vault-confirm-input').focus(), 100);
}

function vaultCheckWord() {
    const word = t('config.vault.confirm_word_de');
    document.getElementById('vault-confirm-btn').disabled =
        document.getElementById('vault-confirm-input').value !== word;
}

function vaultDeleteCancel() {
    document.getElementById('vault-delete-overlay').classList.remove('active');
}

async function vaultDeleteConfirm() {
    document.getElementById('vault-confirm-btn').disabled = true;
    try {
        const resp = await fetch('/api/vault', { method: 'DELETE' });
        const data = await resp.json();
        if (resp.ok) {
            vaultExists = false;
            document.getElementById('vault-delete-overlay').classList.remove('active');
            const cfgResp = await fetch('/api/config');
            configData = await cfgResp.json();
            selectSection('server');
            const toast = document.createElement('div');
            toast.className = 'cfg-vault-toast';
            toast.textContent = t('config.vault.deleted_toast');
            document.body.appendChild(toast);
            setTimeout(() => toast.remove(), 6000);
        } else {
            await showAlert(data.message || t('config.save_bar.error'));
            document.getElementById('vault-confirm-btn').disabled = false;
        }
    } catch (e) {
        await showAlert(t('config.common.network_error') + ' ' + e.message);
        document.getElementById('vault-confirm-btn').disabled = false;
    }
}

// ── Ansible: generate token ────────────────────────────────────────────────
let _ansibleGeneratedToken = '';

async function ansibleGenerateToken() {
    const btn = document.getElementById('ansible-gen-token-btn');
    const result = document.getElementById('ansible-gen-token-result');
    const preview = document.getElementById('ansible-token-preview');
    const tokenVal = document.getElementById('ansible-token-value');
    if (!btn) return;

    btn.disabled = true;
    btn.textContent = '⏳ ' + t('config.ansible.generate_token_generating');
    result.textContent = '';
    preview.classList.add('pw-u-hidden');

    try {
        const resp = await fetch('/api/ansible/generate-token', { method: 'POST' });
        const data = await resp.json();
        if (resp.ok && data.status === 'ok') {
            _ansibleGeneratedToken = data.token;
            tokenVal.textContent = data.token;
            preview.classList.remove('pw-u-hidden');
            result.textContent = t('config.ansible.generate_token_ok');
            result.style.color = 'var(--color-success)';
            // Show masked marker in the token config field if visible
            const tokenField = document.querySelector('[data-path="ansible.token"]');
            if (tokenField) tokenField.value = '••••••••';
        } else {
            result.textContent = data.error || t('config.ansible.generate_token_fail');
            result.style.color = 'var(--color-danger)';
        }
    } catch (e) {
        result.textContent = t('config.ansible.generate_token_fail') + ' ' + e.message;
        result.style.color = 'var(--color-danger)';
    } finally {
        btn.disabled = false;
        btn.innerHTML = '🔑 ' + t('config.ansible.generate_token_btn');
    }
}

function ansibleCopyToken() {
    if (!_ansibleGeneratedToken) return;
    navigator.clipboard.writeText(_ansibleGeneratedToken).then(() => {
        const fb = document.getElementById('ansible-copy-feedback');
        if (fb) {
            fb.textContent = t('config.ansible.generate_token_copied');
            setTimeout(() => { fb.textContent = ''; }, 3000);
        }
    });
}
