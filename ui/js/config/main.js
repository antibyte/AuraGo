// AuraGo – config page logic
// Extracted from config.html

// SVG icons for password toggle (avoids emoji rendering issues)
const EYE_OPEN_SVG = '<svg viewBox="0 0 24 24"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>';
const EYE_CLOSED_SVG = '<svg viewBox="0 0 24 24"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>';
const cfgMaskedSecretFallback = '••••••••';
const CONFIG_ASSET_VERSION = (typeof window !== 'undefined' && window.AURAGO_BUILD_VERSION) ? window.AURAGO_BUILD_VERSION : 'config-dev';
const CONFIG_DENSITY_KEY = 'aurago.config.density.v1';
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
const CFG_TEXT_AUTOFILL_ATTRS = ' autocomplete="off" autocapitalize="off" autocorrect="off" spellcheck="false" data-lpignore="true" data-1p-ignore="true" data-bwignore="true" data-form-type="other"';
const CFG_SENSITIVE_AUTOFILL_ATTRS = ' autocomplete="new-password" autocapitalize="off" autocorrect="off" spellcheck="false" data-lpignore="true" data-1p-ignore="true" data-bwignore="true" data-form-type="other"';

function applyConfigDensity(value, persist = false) {
    const density = value === 'compact' ? 'compact' : 'comfortable';
    document.body.dataset.density = density;
    const button = document.getElementById('cfg-density-toggle');
    if (button) {
        const compact = density === 'compact';
        button.setAttribute('aria-pressed', compact ? 'true' : 'false');
        const label = button.querySelector('span');
        if (label) label.textContent = t(compact ? 'config.precision.density_compact' : 'config.precision.density_comfortable');
    }
    if (persist) localStorage.setItem(CONFIG_DENSITY_KEY, density);
    return density;
}

const configDensityButton = document.getElementById('cfg-density-toggle');
if (configDensityButton) {
    configDensityButton.addEventListener('click', () => {
        applyConfigDensity(document.body.dataset.density === 'compact' ? 'comfortable' : 'compact', true);
    });
}
applyConfigDensity(localStorage.getItem(CONFIG_DENSITY_KEY) || 'comfortable');

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

function configSectionIcon(key) {
    const name = String(key || '').toLowerCase();
    let paths = '<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .34 1.88l.06.06-2.83 2.83-.06-.06A1.7 1.7 0 0 0 15 19.4a1.7 1.7 0 0 0-1 .6l-.04.08h-4l-.04-.08a1.7 1.7 0 0 0-1-.6 1.7 1.7 0 0 0-1.88.34l-.06.06-2.83-2.83.06-.06A1.7 1.7 0 0 0 4.6 15a1.7 1.7 0 0 0-.6-1l-.08-.04v-4L4 9.92a1.7 1.7 0 0 0 .6-1 1.7 1.7 0 0 0-.34-1.88L4.2 6.98l2.83-2.83.06.06A1.7 1.7 0 0 0 9 4.6a1.7 1.7 0 0 0 1-.6l.04-.08h4l.04.08a1.7 1.7 0 0 0 1 .6 1.7 1.7 0 0 0 1.88-.34l.06-.06 2.83 2.83-.06.06A1.7 1.7 0 0 0 19.4 9c.08.38.3.72.6 1l.08.04v4L20 14a1.7 1.7 0 0 0-.6 1z"/>';
    if (/(guardian|security|firewall|danger|virus|auth|proxy)/.test(name)) {
        paths = '<path d="M12 3 5 6v5c0 4.6 2.8 8 7 10 4.2-2 7-5.4 7-10V6l-7-3z"/><path d="m9.5 12 1.7 1.7 3.6-4"/>';
    } else if (/(sqlite|sql|memory|index|grafana)/.test(name)) {
        paths = '<ellipse cx="12" cy="5" rx="7" ry="3"/><path d="M5 5v6c0 1.7 3.1 3 7 3s7-1.3 7-3V5M5 11v6c0 1.7 3.1 3 7 3s7-1.3 7-3v-6"/>';
    } else if (/(image|music|video|media|tts|whisper|document|vision)/.test(name)) {
        paths = '<rect x="3" y="5" width="18" height="14" rx="2"/><path d="m8 15 2.5-3 2 2 2.5-3 3 4M9 9h.01"/>';
    } else if (/(email|telegram|discord|chat|telnyx|notification|webhook)/.test(name)) {
        paths = '<path d="M5 5h14a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2h-8l-4 3v-3H5a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2z"/><path d="M7 9h10M7 13h6"/>';
    } else if (/(s3|webdav|koofr|backup|cloud|netlify|vercel)/.test(name)) {
        paths = '<path d="M7 18h10a4 4 0 0 0 .5-8A6 6 0 0 0 6 8.5 4.5 4.5 0 0 0 7 18z"/><path d="m9 14 3-3 3 3M12 11v7"/>';
    } else if (/(docker|sandbox|server|proxmox|ollama|container)/.test(name)) {
        paths = '<rect x="4" y="3" width="16" height="18" rx="2"/><path d="M8 7h8M8 12h8M8 17h5"/>';
    } else if (/(network|remote|tailscale|fritz|mqtt|home|chromecast|mesh|uptime)/.test(name)) {
        paths = '<circle cx="12" cy="12" r="2"/><path d="M5.6 18.4a9 9 0 0 1 0-12.8M18.4 5.6a9 9 0 0 1 0 12.8M8.5 15.5a5 5 0 0 1 0-7M15.5 8.5a5 5 0 0 1 0 7"/>';
    } else if (/(agent|llm|provider|embedding|ai_|generation|personality|prompt|co_)/.test(name)) {
        paths = '<path d="m12 3 1.4 4.1L17.5 8.5l-4.1 1.4L12 14l-1.4-4.1-4.1-1.4 4.1-1.4L12 3zM18 14l.8 2.2L21 17l-2.2.8L18 20l-.8-2.2L15 17l2.2-.8L18 14z"/>';
    } else if (/(tool|browser|skill|automation|mission|ansible)/.test(name)) {
        paths = '<path d="M14.7 6.3a4 4 0 0 0-5 5L4 17l3 3 5.7-5.7a4 4 0 0 0 5-5l-2.4 2.4-3-3 2.4-2.4z"/>';
    }
    return `<svg viewBox="0 0 24 24" aria-hidden="true">${paths}</svg>`;
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
    overviewItem.innerHTML = `<span class="icon" aria-hidden="true"><svg viewBox="0 0 24 24"><path d="M4 4h6v6H4zM14 4h6v6h-6zM4 14h6v6H4zM14 14h6v6h-6z"/></svg></span><span class="sidebar-item-label">${escapeHtml(t('config.precision.overview_title'))}</span>`;
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
            item.innerHTML = '<span class="icon">' + configSectionIcon(s.key) + '</span><span class="sidebar-item-label">' + escapeHtml(s.label) + '</span>';
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
if (configSectionObserverRoot) {
    configSectionObserver.observe(configSectionObserverRoot, { childList: true, subtree: true });
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
    }
}

/** Wire the multimodal toggle to show/hide the format selector. */
function _embeddingsBindMultimodal() {
    const toggle = document.querySelector('[data-path="embeddings.multimodal"]');
    const formatEl = document.querySelector('[data-path="embeddings.multimodal_format"]');
    if (!toggle || !formatEl) return;
    const formatField = formatEl.closest('.field-group');
    if (!formatField) return;

    function sync() {
        formatField.style.display = toggle.classList.contains('on') ? '' : 'none';
    }
    sync();

    // Observe class changes on the toggle to react to toggleBool()
    new MutationObserver(sync).observe(toggle, { attributes: true, attributeFilter: ['class'] });
}

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
        const emptyLabel = t('config.field.no_provider');
        const emptySelected = (!value || value === '') ? ' selected' : '';
        html += '<option value=""' + emptySelected + '>' + emptyLabel + '</option>';
        if (help.allow_disabled) {
            const disSelected = (value === 'disabled') ? ' selected' : '';
            html += '<option value="disabled"' + disSelected + '>' + escapeHtml(cfgFieldOptionLabel('disabled')) + '</option>';
        }
        providersCache.forEach(p => {
            const selected = (String(value) === String(p.id)) ? ' selected' : '';
            const displayName = p.name || p.id;
            const badge = p.type ? (' [' + p.type + ']') : '';
            const modelHint = p.model ? (' — ' + p.model) : '';
            html += '<option value="' + escapeAttr(p.id) + '"' + selected + '>' + escapeAttr(displayName + badge + modelHint) + '</option>';
        });
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

function guessType(val) {
    if (typeof val === 'boolean') return 'bool';
    if (typeof val === 'number') return Number.isInteger(val) ? 'int' : 'float';
    if (Array.isArray(val)) return 'array';
    return 'string';
}

function fieldLabelText(fullPath, key) {
    const labelKey = 'config.' + fullPath + '_label';
    const translated = t(labelKey);
    if (typeof translated === 'string' && translated.trim() !== '' && translated !== labelKey && translated !== '-') {
        return translated;
    }
    return formatKey(key);
}

function formatKey(key) {
    return key.split('_').map(w => w.charAt(0).toUpperCase() + w.slice(1)).join(' ');
}

function escapeAttr(s) { return String(s).replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/'/g, '&#39;').replace(/</g, '&lt;').replace(/>/g, '&gt;'); }
function escapeHtml(s) { return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#39;'); }
function cfgNoAutofillAttrs(sensitive = false) {
    return sensitive ? CFG_SENSITIVE_AUTOFILL_ATTRS : CFG_TEXT_AUTOFILL_ATTRS;
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
    const on = el.classList.toggle('on');
    if (el.nextElementSibling) {
        el.nextElementSibling.textContent = on ? t('config.toggle.active') : t('config.toggle.inactive');
    }
    syncToggleA11y(el);
    markDirty();
}

const CFG_OPTION_OTHER_CUSTOM = 'Other / Custom';

function cfgFieldOptionLabel(option) {
    if (option === 'disabled') return '\u{1F6AB} ' + (t('config.field.disabled_option'));
    if (option === CFG_OPTION_OTHER_CUSTOM) return t('config.field.other_custom_option') || CFG_OPTION_OTHER_CUSTOM;
    return option;
}

function cfgToggleCustomInput(selectEl) {
    const customInput = selectEl.nextElementSibling;
    if (!customInput || !customInput.classList.contains('cfg-custom-input')) return;
    const showCustom = selectEl.value === CFG_OPTION_OTHER_CUSTOM;
    customInput.classList.toggle('is-hidden', !showCustom);
    if (showCustom) customInput.focus();
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
    return target.closest('.field-input, .field-select, .cfg-input, .toggle');
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

function isUserInitiatedConfigChange(event) {
    if (!event) return true;
    if (event.isTrusted !== true) return false;
    if (suppressDirtyTracking && Date.now() > configEditIntentUntil) return false;
    if (Date.now() <= configEditIntentUntil) return true;
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
    const currentLocal = current.local_ollama || {};
    const nextLocal = nextEmbeddings.local_ollama || {};

    const comparePaths = [
        ['provider', current.provider, nextEmbeddings.provider],
        ['internal_model', current.internal_model, nextEmbeddings.internal_model],
        ['external_url', current.external_url, nextEmbeddings.external_url],
        ['external_model', current.external_model, nextEmbeddings.external_model],
        ['multimodal', current.multimodal, nextEmbeddings.multimodal],
        ['multimodal_format', current.multimodal_format, nextEmbeddings.multimodal_format],
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
    if (dirty && window.AuraConfigState) {
        window.AuraConfigState.syncFromDOM();
        dirty = window.AuraConfigState.isDirty();
    }
    isDirty = dirty;
    const btn = document.getElementById('btnSave');
    const pill = document.getElementById('changesPill');
    const restartBtn = document.getElementById('cfg-restart-btn');
    const changeCount = document.getElementById('saveChangeCount');
    const validation = document.getElementById('saveValidation');
    const count = window.AuraConfigState ? window.AuraConfigState.dirtyPaths().length : (dirty ? 1 : 0);
    btn.disabled = !dirty || configSaveInFlight;
    pill.classList.toggle('visible', dirty);
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
    document.querySelectorAll('.field-input, .field-select, .cfg-input').forEach(el => {
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
    media_conversion: { m: 'media_conversion', fn: 'renderMediaConversionSection' },
    video_download: { m: 'video_download', fn: 'renderVideoDownloadSection' },
    webhooks: { m: 'webhooks', fn: 'renderWebhooksSection' },
    prompts_editor: { m: 'prompts', fn: 'renderPromptsSection' },
    rules: { m: 'rules', fn: 'renderRulesSection' },
    indexing: { m: 'indexing', fn: 'renderIndexingSection' },
    backup_restore: { m: 'backup', fn: 'renderBackupSection' },
    updates: { m: 'updates', fn: 'renderUpdatesSection' },
    chromecast: { m: 'chromecast', fn: 'renderChromecastSection' },
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
