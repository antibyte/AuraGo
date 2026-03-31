// AuraGo – config page logic
// Extracted from config.html

// SVG icons for password toggle (avoids emoji rendering issues)
const EYE_OPEN_SVG = '<svg viewBox="0 0 24 24"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>';
const EYE_CLOSED_SVG = '<svg viewBox="0 0 24 24"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>';

// Section metadata grouped by logical categories
const SECTIONS = [
    {
        group: t('config.group.agent_ai'),
        items: [
            { key: 'agent', icon: '⚙️', label: t('config.section.agent.label'), desc: t('config.section.agent.desc') },
            { key: 'providers', icon: '🔌', label: t('config.section.providers.label'), desc: t('config.section.providers.desc') },
            { key: 'llm', icon: '🧠', label: t('config.section.llm.label'), desc: t('config.section.llm.desc') },
            { key: 'fallback_llm', icon: '🔄', label: t('config.section.fallback_llm.label'), desc: t('config.section.fallback_llm.desc') },
            { key: 'embeddings', icon: '🔗', label: t('config.section.embeddings.label'), desc: t('config.section.embeddings.desc') },
            { key: 'circuit_breaker', icon: '⚡', label: t('config.section.circuit_breaker.label'), desc: t('config.section.circuit_breaker.desc') },
            { key: 'budget', icon: '💰', label: t('config.section.budget.label'), desc: t('config.section.budget.desc') },
            { key: 'memory_analysis', icon: '🧬', label: t('config.section.memory_analysis.label'), desc: t('config.section.memory_analysis.desc') },
            { key: 'co_agents', icon: '🤖', label: t('config.section.co_agents.label'), desc: t('config.section.co_agents.desc') },
            { key: 'prompts_editor', icon: '🎭', label: t('config.section.prompts_editor.label'), desc: t('config.section.prompts_editor.desc') },
            { key: 'personality', icon: '🧬', label: t('config.section.personality.label'), desc: t('config.section.personality.desc') },
            { key: 'vision', icon: '👁️', label: t('config.section.vision.label'), desc: t('config.section.vision.desc') }
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
            { key: 'sandbox', icon: '📦', label: t('config.section.sandbox.label'), desc: t('config.section.sandbox.desc') },
            { key: 'info_tools', icon: '🔍', label: t('config.section.info_tools.label'), desc: t('config.section.info_tools.desc') },
            { key: 'network_tools', icon: '📡', label: t('config.section.network_tools.label'), desc: t('config.section.network_tools.desc') },
            { key: 'brave_search', icon: '🦁', label: t('config.section.brave_search.label'), desc: t('config.section.brave_search.desc') },
            { key: 'skill_manager', icon: '🧩', label: t('config.section.skill_manager.label'), desc: t('config.section.skill_manager.desc') },
            { key: 'mission_preparation', icon: '🎯', label: t('config.section.mission_preparation.label'), desc: t('config.section.mission_preparation.desc') }
        ]
    },
    {
        group: t('config.group.media_content'),
        items: [
            { key: 'whisper', icon: '🎤', label: t('config.section.whisper.label'), desc: t('config.section.whisper.desc') },
            { key: 'tts', icon: '🔊', label: t('config.section.tts.label'), desc: t('config.section.tts.desc') },
            { key: 'image_generation', icon: '🎨', label: t('config.section.image_generation.label'), desc: t('config.section.image_generation.desc') },
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
            { key: 'n8n', icon: '🔀', label: t('config.section.n8n.label'), desc: t('config.section.n8n.desc') }
        ]
    },
    {
        group: t('config.group.smart_home'),
        items: [
            { key: 'home_assistant', icon: '🏠', label: t('config.section.home_assistant.label'), desc: t('config.section.home_assistant.desc') },
            { key: 'mqtt', icon: '📡', label: t('config.section.mqtt.label'), desc: t('config.section.mqtt.desc') },
            { key: 'chromecast', icon: '📺', label: t('config.section.chromecast.label'), desc: t('config.section.chromecast.desc') },
            { key: 'adguard', icon: '🛡️', label: t('config.section.adguard.label'), desc: t('config.section.adguard.desc') },
            { key: 'fritzbox', icon: '📡', label: t('config.section.fritzbox.label'), desc: t('config.section.fritzbox.desc') }
        ]
    },
    {
        group: t('config.group.network_remote'),
        items: [
            { key: 'truenas', icon: '💾', label: t('config.section.truenas.label'), desc: t('config.section.truenas.desc') },
            { key: 'jellyfin', icon: '🎬', label: t('config.section.jellyfin.label'), desc: t('config.section.jellyfin.desc') },
            { key: 'tailscale', icon: '🔒', label: t('config.section.tailscale.label'), desc: t('config.section.tailscale.desc') },
            { key: 'proxmox', icon: '🖥️', label: t('config.section.proxmox.label'), desc: t('config.section.proxmox.desc') },
            { key: 'remote_control', icon: '📡', label: t('config.section.remote_control.label'), desc: t('config.section.remote_control.desc') },
            { key: 'meshcentral', icon: '🖥️', label: t('config.section.meshcentral.label'), desc: t('config.section.meshcentral.desc') },
            { key: 'ansible', icon: '⚙️', label: t('config.section.ansible.label'), desc: t('config.section.ansible.desc') }
        ]
    },
    {
        group: t('config.group.security'),
        items: [
            { key: 'security_proxy', icon: '🔒', label: t('config.section.security_proxy.label'), desc: t('config.section.security_proxy.desc') },
            { key: 'llm_guardian', icon: '🛡️', label: t('config.section.llm_guardian.label'), desc: t('config.section.llm_guardian.desc') },
            { key: 'virustotal', icon: '🦠', label: t('config.section.virustotal.label'), desc: t('config.section.virustotal.desc') }
        ]
    },
    {
        group: t('config.group.external_ai'),
        items: [
            { key: 'ai_gateway', icon: '🌩️', label: t('config.section.ai_gateway.label'), desc: t('config.section.ai_gateway.desc') },
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

const lang = SYSTEM_LANG === 'de' ? 'de' : 'en';
let configData = {};
// helpTexts built from I18N — no separate fetch needed
const helpTexts = new Proxy({}, {
    get(_, key) {
        const txt = I18N['help.' + key];
        const meta = I18N_META['help.' + key];
        if (!txt && !meta) return undefined;
        const obj = {};
        if (txt) { obj[lang] = txt; obj.en = txt; }
        if (meta) Object.assign(obj, meta);
        return obj;
    }
});
let schema = [];
let activeSection = localStorage.getItem('aurago-cfg-section') || 'server';
let isDirty = false;
let initialSnapshot = '';
let vaultExists = false;
const SENSITIVE_KEYS = ['api_key', 'bot_token', 'password', 'app_password', 'access_token', 'token', 'user_key', 'app_token', 'secret', 'master_key'];

function hasVisibleSection(key) {
    return SECTIONS.some(group => group.items.some(item => item.key === key));
}

// Provider management state (loaded from /api/providers)
let providersCache = [];

// Personality profiles cache (loaded from /api/personalities)
let personalitiesCache = [];

// Runtime environment detection (loaded from /api/runtime)
let runtimeData = { runtime: {}, features: {} };

// Init
async function init() {
    try {
        const [cfgResp, schemaResp, vaultResp] = await Promise.all([
            fetch('/api/config'),
            fetch('/api/config/schema'),
            fetch('/api/vault/status')
        ]);
        configData = await cfgResp.json();
        schema = await schemaResp.json();
        try { vaultExists = (await vaultResp.json()).exists === true; } catch (_) { }
        // Load providers (best-effort – endpoint only exists when web_config is enabled)
        try {
            const provResp = await fetch('/api/providers');
            if (provResp.ok) providersCache = await provResp.json();
        } catch (_) { }
        // Load personality profiles
        try {
            const persResp = await fetch('/api/personalities');
            if (persResp.ok) { const d = await persResp.json(); personalitiesCache = d.personalities || []; }
        } catch (_) { }
        // Load runtime environment capabilities (Docker mode, socket, broadcast, etc.)
        try {
            const rtResp = await fetch('/api/runtime');
            if (rtResp.ok) runtimeData = await rtResp.json();
        } catch (_) { }
    } catch (e) {
        document.getElementById('content').innerHTML = '<div class="cfg-error-state cfg-error-state-lg">❌ ' + t('config.loading_error') + '<br><small>' + e.message + '</small></div>';
        return;
    }
    if (!hasVisibleSection(activeSection)) {
        activeSection = 'server';
        localStorage.setItem('aurago-cfg-section', activeSection);
    }
    buildSidebar();
    selectSection(activeSection, { scrollBehavior: 'auto' });
    // Take initial snapshot for dirty tracking after first render
    setTimeout(() => { initialSnapshot = collectSnapshot(); setDirty(false); }, 100);
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

function buildSidebar() {
    const sb = document.getElementById('sidebar');
    sb.innerHTML = '';
    let lastWasIntSub = false;

    SECTIONS.forEach((group) => {
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
        const header = document.createElement('div');
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
        content.style.maxHeight = isCollapsed ? '0' : 'none';

        group.items.forEach(s => {
            const item = document.createElement('div');
            item.className = 'sidebar-item'
                + (s.key === activeSection ? ' active' : '')
                + (group.dangerGroup ? ' danger-item' : '')
                + (group.integrationSubGroup ? ' integration-sub-item' : '');
            item.dataset.section = s.key;
            item.innerHTML = '<span class="icon">' + s.icon + '</span><span>' + s.label + '</span>';
            item.onclick = () => selectSection(s.key);
            content.appendChild(item);
        });

        groupDiv.appendChild(header);
        groupDiv.appendChild(content);
        sb.appendChild(groupDiv);
    });
}

function toggleGroup(groupName, groupDiv) {
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
    saveCollapsedGroups();
}

async function selectSection(key, options = {}) {
    const { scrollBehavior = 'smooth' } = options;
    if (!hasVisibleSection(key)) key = 'server';
    activeSection = key;
    localStorage.setItem('aurago-cfg-section', key);
    document.querySelectorAll('.sidebar-item').forEach(el => el.classList.toggle('active', el.dataset.section === key));
    // Auto-expand the group containing this section if it is collapsed
    let expandedTargetGroup = false;
    for (const group of SECTIONS) {
        if (group.items.some(s => s.key === key) && collapsedGroups.has(group.group)) {
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
    await renderSection(key);
    // Re-attach change listeners after rendering
    attachChangeListeners();
    // Auto-close sidebar on mobile
    closeSidebar();
}

async function renderSection(key) {
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
            document.getElementById('content').innerHTML = '<div class="cfg-error-state cfg-error-state-md">\u274c Module load error: ' + e.message + '</div>';
            return;
        }
        const fn = window[modInfo.fn];
        if (fn) { await fn(section); return; }
    }

    const data = configData[key] || {};
    const sectionSchema = schema.find(s => s.yaml_key === key);

    // Keys managed by other sections — skip in generic renderer
    const AGENT_SKIP_KEYS = new Set([
        'allow_shell', 'allow_python', 'allow_filesystem_write',
        'allow_network_requests', 'allow_remote_shell', 'allow_self_update',
        'allow_mcp',               // → Danger Zone
        'allow_web_scraper',        // → deprecated, migrated to tools.web_scraper.enabled
        'sudo_enabled',            // → Danger Zone
        'core_personality',         // → Prompts & Personas
        'additional_prompt',        // → Prompts & Personas
        'personality_v2_model',     // → managed by provider
        'personality_v2_url',       // → managed by provider
        'personality_v2_api_key'    // → managed by provider
    ]);
    // Legacy fields superseded by provider management — hide from UI
    const EMBEDDINGS_SKIP_KEYS = new Set(['api_key', 'external_model', 'external_url', 'internal_model']);
    // Legacy fields superseded by provider management — hide from UI
    const LLM_SKIP_KEYS = new Set(['api_key', 'base_url', 'model']);
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
        'document_creator'// → Document Creator section
    ]);

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // Generic feature-unavailability banner for sections with runtime checks
    const SECTION_FEATURE_MAP = { docker: 'docker', invasion_control: 'invasion_local' };
    const sectionFeatureKey = SECTION_FEATURE_MAP[key];
    if (sectionFeatureKey) {
        const fb = featureUnavailableBanner(sectionFeatureKey);
        if (fb) html += fb;
    }
    const sectionBlocked = sectionFeatureKey && runtimeData.features && runtimeData.features[sectionFeatureKey] && !runtimeData.features[sectionFeatureKey].available;
    if (sectionBlocked) html += '<div class="feature-unavailable-fields">';

    // LLM settings explanation
    if (key === 'llm') {
        html += `<div class="cfg-note-banner cfg-note-banner-info">
                    \u{1F9E0} ${t('config.llm.info_banner')}
                </div>`;
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
            <div style="display:flex;gap:8px;align-items:center;flex-wrap:wrap;">
                <button id="ansible-gen-token-btn" class="cfg-btn cfg-btn-primary" onclick="ansibleGenerateToken()">
                    🔑 ${t('config.ansible.generate_token_btn')}
                </button>
                <span id="ansible-gen-token-result" style="font-size:0.85em;color:var(--color-text-muted)"></span>
            </div>
            <div id="ansible-token-preview" style="display:none;margin-top:8px">
                <code id="ansible-token-value" style="font-size:0.78em;word-break:break-all;background:var(--input-bg);padding:6px 10px;border-radius:6px;display:block"></code>
                <button class="cfg-btn cfg-btn-sm" style="margin-top:6px" onclick="ansibleCopyToken()">${t('config.ansible.generate_token_copy')}</button>
                <span id="ansible-copy-feedback" style="font-size:0.8em;margin-left:8px;color:var(--color-success)"></span>
            </div>
        </div>`;
    }

    if (sectionBlocked) html += '</div>'; // End feature-unavailable-fields
    html += '</div>';

    document.getElementById('content').innerHTML = html;

    // ── Embeddings: multimodal_format visibility depends on multimodal toggle ──
    if (key === 'embeddings') {
        _embeddingsBindMultimodal();
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


function renderFields(fields, data, parentPath) {
    let html = '';
    for (const field of fields) {
        const val = data[field.yaml_key];
        const fullPath = parentPath + '.' + field.yaml_key;

        if (field.type === 'object' && field.children) {
            html += '<div class="cfg-group-title cfg-group-title-underlined">' + formatKey(field.yaml_key) + '</div>';
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
    const cls = blocked ? 'feature-unavailable-banner fub-blocked' : 'feature-unavailable-banner';
    const icon = blocked ? '🚫' : '⚠️';
    return '<div class="' + cls + '"><span class="fub-icon">' + icon + '</span><span>' + escapeHtml(fa.reason || t('config.feature_unavailable')) + '</span></div>';
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
        h += '<div class="field-label">Master Key <span class="cfg-sensitive-icon">🔒</span></div>';
        if (helpText) h += '<div class="field-help">' + helpText + '</div>';
        if (vaultExists) {
            h += '<div class="password-wrap">';
            h += '<input class="field-input cfg-master-key-locked-input" type="password" value="\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022" disabled>';
            h += '<button type="button" class="password-toggle cfg-master-key-delete-btn" title="' + t('config.master_key.vault_delete_tooltip') + '" onclick="vaultDeletePrompt()">🗑️</button>';
            h += '</div>';
            h += '<div class="cfg-master-key-note">🔐 ' + t('config.master_key.vault_exists') + '</div>';
        } else {
            h += '<div class="password-wrap">';
            h += '<input class="field-input" type="password" data-path="server.master_key" value="" placeholder="' + t('config.master_key.placeholder') + '" autocomplete="off">';
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
    html += '<div class="field-label">' + formatKey(key);
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
        const emptyLabel = t('config.field.no_personality') || '— default —';
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
            html += '<option value="disabled"' + disSelected + '>🚫 disabled</option>';
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
        const hasCustom = helpOptions.includes('Other / Custom');
        const isCustomVal = hasCustom && value && !helpOptions.includes(value) && value !== 'Other / Custom';

        html += '<select class="field-select" data-path="' + fullPath + '" onchange="cfgToggleCustomInput(this)">';
        helpOptions.forEach(opt => {
            const selected = (String(value) === String(opt) || (opt === 'Other / Custom' && isCustomVal)) ? ' selected' : '';
            html += '<option value="' + escapeAttr(opt) + '"' + selected + '>' + escapeAttr(opt) + '</option>';
        });
        html += '</select>';

        if (hasCustom) {
            const hiddenCls = isCustomVal ? '' : ' is-hidden';
            const customVal = isCustomVal ? value : '';
            html += '<input class="field-input cfg-custom-input' + hiddenCls + '" type="text" data-custom-for="' + fullPath + '" value="' + escapeAttr(customVal) + '" placeholder="Type custom value..." oninput="markDirty()">';
        }
    } else if (isSensitive) {
        const displayVal = (value === '••••••••' || !value) ? '' : value;
        html += '<div class="password-wrap">';
        html += '<input class="field-input" type="password" data-path="' + fullPath + '" value="' + escapeAttr(displayVal) + '" placeholder="••••••••" autocomplete="off">';
        html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
        html += '</div>';
    } else if (fieldType === 'int' || fieldType === 'float') {
        const step = fieldType === 'float' ? '0.01' : '1';
        // Special handling for agent.core_memory_max_entries: show default 200 if missing/0
        let showValue = value;
        if (fullPath.endsWith('agent.core_memory_max_entries')) {
            showValue = (value === undefined || value === null || value === 0 || value === '') ? 200 : value;
        }
        html += '<input class="field-input" type="number" step="' + step + '" data-path="' + fullPath + '" value="' + (showValue ?? '') + '">';
    } else if (fieldType === 'array') {
        // budget.models is now managed in Provider settings
        if (fullPath === 'budget.models') {
            html += '<div class="cfg-budget-models-hint">'
                + '💰 ' + t('config.budget.models_moved_hint')
                + '</div>';
        } else {
        const isObjArray = (Array.isArray(value) && value.length > 0 && typeof value[0] === 'object' && value[0] !== null);
        if (isObjArray) {
            const jsonVal = Array.isArray(value) ? JSON.stringify(value, null, 2) : '[]';
            html += '<textarea class="field-input cfg-json-array-input" data-path="' + fullPath + '" data-type="json" rows="6">' + escapeHtml(jsonVal) + '</textarea>';
            html += '<div class="cfg-json-array-hint">' + t('config.field.json_array_hint') + '</div>';
        } else {
            const arrVal = Array.isArray(value) ? value.join(', ') : (value || '');
            html += '<input class="field-input" type="text" data-path="' + fullPath + '" data-type="array" value="' + escapeAttr(arrVal) + '" placeholder="' + t('config.field.comma_separated') + '">';
        }
        }
    } else {
        html += '<input class="field-input" type="text" data-path="' + fullPath + '" value="' + escapeAttr(value ?? '') + '">';
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

function formatKey(key) {
    return key.split('_').map(w => w.charAt(0).toUpperCase() + w.slice(1)).join(' ');
}

function escapeAttr(s) { return String(s).replace(/"/g, '&quot;').replace(/</g, '&lt;'); }
function escapeHtml(s) { return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#39;'); }

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
    markDirty();
}

function cfgToggleCustomInput(selectEl) {
    const customInput = selectEl.nextElementSibling;
    if (!customInput || !customInput.classList.contains('cfg-custom-input')) return;
    const showCustom = selectEl.value === 'Other / Custom';
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

function getNestedValue(obj, path) {
    if (!obj || !path) return undefined;
    return path.split('.').reduce((cur, part) => (cur && cur[part] !== undefined) ? cur[part] : undefined, obj);
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
        } else if (el.tagName === 'SELECT' && el.value === 'Other / Custom') {
            const customInput = document.querySelector('[data-custom-for="' + path + '"]');
            val = customInput ? customInput.value.trim() : el.value;
        } else {
            val = el.value;
        }

        let obj = patch;
        for (let i = 0; i < parts.length - 1; i++) {
            if (forbidden.has(parts[i])) return; // Prototype pollution guard
            if (!obj[parts[i]]) obj[parts[i]] = {};
            obj = obj[parts[i]];
        }
        const lastKey = parts[parts.length - 1];
        if (forbidden.has(lastKey)) return;
        obj[lastKey] = val;
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

function markDirty() {
    if (!isDirty) {
        isDirty = true;
        setDirty(true);
    }
}

function setDirty(dirty) {
    isDirty = dirty;
    const btn = document.getElementById('btnSave');
    const pill = document.getElementById('changesPill');
    btn.disabled = !dirty;
    pill.classList.toggle('visible', dirty);
}

function attachChangeListeners() {
    document.querySelectorAll('.field-input, .field-select, .cfg-input').forEach(el => {
        el.addEventListener('input', markDirty);
        el.addEventListener('change', markDirty);
    });
}

// ── Save ────────────────────────────────────────────
async function saveConfig() {
    const btn = document.getElementById('btnSave');
    const status = document.getElementById('saveStatus');
    btn.disabled = true;
    status.className = 'save-status';
    status.textContent = t('config.save_bar.saving');

    const patch = buildConfigPatchFromForm();
    const likelyEmbeddingsChange = embeddingsConfigWillLikelyChange(patch);
    let scheduleResetAfterSave = false;

    if (likelyEmbeddingsChange) {
        const wantsReset = await showEmbeddingsResetModal();
        if (!wantsReset) {
            status.className = 'save-status warning';
            status.textContent = '⚠ ' + t('config.embeddings.reset_cancelled');
            btn.disabled = false;
            setTimeout(() => { status.textContent = ''; }, 5000);
            return;
        }
        if (!await showConfirm(t('config.embeddings.reset_confirm_final'))) {
            status.className = 'save-status warning';
            status.textContent = '⚠ ' + t('config.embeddings.reset_cancelled');
            btn.disabled = false;
            setTimeout(() => { status.textContent = ''; }, 5000);
            return;
        }
        scheduleResetAfterSave = true;
    }

    try {
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
                    btn.disabled = false;
                    setTimeout(() => { status.textContent = ''; }, 7000);
                    return;
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
            initialSnapshot = collectSnapshot();
            setDirty(false);
            // Check for security issues introduced by this save
            checkSecurityAfterSave();
            if (shouldScheduleResetNow) {
                status.className = 'save-status warning';
                status.textContent = '⚠ ' + t('config.embeddings.reset_restarting');
                await restartAuraGo(true);
                return;
            }
            if (embeddingsChanged) {
                status.className = 'save-status warning';
                status.textContent = '⚠ ' + t('config.embeddings.reset_pending_warning');
            }
        } else {
            status.className = 'save-status error';
            status.textContent = '✗ ' + (result.message || t('config.save_bar.error'));
            btn.disabled = false;
        }
    } catch (e) {
        status.className = 'save-status error';
        status.textContent = '✗ ' + e.message;
        btn.disabled = false;
    }
    setTimeout(() => { status.textContent = ''; }, 5000);
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
        applyBtn.textContent = t('config.security.applying') || 'Applying…';
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

async function restartAuraGo(skipConfirm = false) {
    if (!skipConfirm && !await showConfirm(t('config.restart.confirm'))) return;

    try {
        const resp = await fetch('/api/restart', { method: 'POST' });
        if (resp.ok) {
            const res = await resp.json();
            document.body.innerHTML = `
                        <div class="cfg-restart-screen">
                            <div class="cfg-restart-spinner">↻</div>
                            <h2 class="cfg-restart-title">${res.message}</h2>
                            <p class="cfg-restart-desc">${t('config.restart.reloading')}</p>
                        </div>
                    `;
            // Attempt to reload after 4 seconds to give the service time to restart
            setTimeout(() => window.location.reload(), 4000);
        } else {
            await showAlert(t('config.restart.error'));
        }
    } catch (e) {
        // If the fetch fails immediately, it might be that the server died instantly.
        // We'll still show the reloading screen.
        document.body.innerHTML = '<div class="cfg-restart-disconnected">' + t('config.restart.disconnected') + '</div>';
        setTimeout(() => window.location.reload(), 4000);
    }
}

// Boot

/* ── esc() is now provided by shared.js ── */

/* ── Lazy module loader ── */
const _moduleCache = {};
const SECTION_MODULES = {
    providers: { m: 'providers', fn: 'renderProvidersSection' },
    email: { m: 'email', fn: 'renderEmailSection' },
    mcp: { m: 'mcp', fn: 'renderMCPSection' },
    sandbox: { m: 'sandbox', fn: 'renderSandboxSection' },
    web_scraper: { m: 'scraper', fn: 'renderWebScraperSection' },
    webhooks: { m: 'webhooks', fn: 'renderWebhooksSection' },
    prompts_editor: { m: 'prompts', fn: 'renderPromptsSection' },
    indexing: { m: 'indexing', fn: 'renderIndexingSection' },
    backup_restore: { m: 'backup', fn: 'renderBackupSection' },
    updates: { m: 'updates', fn: 'renderUpdatesSection' },
    chromecast: { m: 'chromecast', fn: 'renderChromecastSection' },
    adguard: { m: 'adguard', fn: 'renderAdGuardSection' },
    fritzbox: { m: 'fritzbox', fn: 'renderFritzBoxSection' },
    webdav: { m: 'webdav', fn: 'renderWebDAVSection' },
    koofr: { m: 'koofr', fn: 'renderKoofrSection' },
    telnyx: { m: 'telnyx', fn: 'renderTelnyxSection' },
    paperless_ngx: { m: 'paperless', fn: 'renderPaperlessSection' },
    homepage: { m: 'homepage', fn: 'renderHomepageSection' },
    netlify: { m: 'netlify', fn: 'renderNetlifySection' },
    danger_zone: { m: 'danger', fn: 'renderDangerZoneSection' },
    truenas: { m: 'truenas', fn: 'renderTrueNASSection' },
    jellyfin: { m: 'jellyfin', fn: 'renderJellyfinSection' },
    web_config: { m: 'auth', fn: 'renderWebConfigSection' },
    firewall: { m: 'firewall', fn: 'renderFirewallSection' },
    github: { m: 'github', fn: 'renderGitHubSection' },
    google_workspace: { m: 'google_workspace', fn: 'renderGoogleWorkspaceSection' },
    ai_gateway: { m: 'ai_gateway', fn: 'renderAIGatewaySection' },
    cloudflare_tunnel: { m: 'cloudflare_tunnel', fn: 'renderCloudflareTunnelSection' },
    mcp_server: { m: 'mcp_server', fn: 'renderMCPServerSection' },
    image_generation: { m: 'image_generation', fn: 'renderImageGenerationSection' },
    remote_control: { m: 'remote_control', fn: 'renderRemoteControlSection' },
    security_proxy: { m: 'security_proxy', fn: 'renderSecurityProxySection' },
    memory_analysis: { m: 'memory_analysis', fn: 'renderMemoryAnalysisSection' },
    llm_guardian: { m: 'llm_guardian', fn: 'renderLLMGuardianSection' },
    document_creator: { m: 'document_creator', fn: 'renderDocumentCreatorSection' },
    tailscale: { m: 'tailscale', fn: 'renderTailscaleSection' },
    server: { m: 'server', fn: 'renderServerSection' },
    a2a: { m: 'a2a', fn: 'renderA2ASection' },
    n8n: { m: 'n8n', fn: 'renderN8nSection' },
    tts: { m: 'tts', fn: 'renderTTSSection' },
    co_agents: { m: 'co_agents', fn: 'renderCoAgentsSection' },
    sql_connections: { m: 'sql_connections', fn: 'renderSQLConnectionsSection' },
    skill_manager: { m: 'skill_manager', fn: 'renderSkillManagerSection' },
    ollama: { m: 'ollama', fn: 'renderOllamaSection' },
    mission_preparation: { m: 'mission_preparation', fn: 'renderMissionPreparationSection' }
};

function loadModule(name) {
    if (_moduleCache[name]) return _moduleCache[name];
    _moduleCache[name] = new Promise((resolve, reject) => {
        const s = document.createElement('script');
        s.src = '/cfg/' + name + '.js';
        s.onload = resolve;
        s.onerror = () => reject(new Error('Failed to load module: ' + name));
        document.head.appendChild(s);
    });
    return _moduleCache[name];
}

init();

/* ── Vault delete functions (core — used by vault modal HTML + renderField) ── */
function vaultDeletePrompt() {
    const word = t('config.vault.confirm_word_de');
    document.getElementById('vault-confirm-word').textContent = word;
    document.getElementById('vault-confirm-input').placeholder = word;
    document.getElementById('vault-confirm-input').value = '';
    document.getElementById('vault-confirm-btn').disabled = true;
    document.getElementById('vault-modal-title').textContent = t('config.vault.delete_title');
    document.getElementById('vault-modal-desc').innerHTML = t('config.vault.delete_desc');
    document.getElementById('vault-confirm-btn').textContent = t('config.vault.destroy_button');
    document.getElementById('vault-cancel-btn').textContent = t('config.vault.cancel');
    document.getElementById('vault-delete-overlay').classList.remove('is-hidden');
    setTimeout(() => document.getElementById('vault-confirm-input').focus(), 100);
}

function vaultCheckWord() {
    const word = t('config.vault.confirm_word_de');
    document.getElementById('vault-confirm-btn').disabled =
        document.getElementById('vault-confirm-input').value !== word;
}

function vaultDeleteCancel() {
    document.getElementById('vault-delete-overlay').classList.add('is-hidden');
}

async function vaultDeleteConfirm() {
    document.getElementById('vault-confirm-btn').disabled = true;
    try {
        const resp = await fetch('/api/vault', { method: 'DELETE' });
        const data = await resp.json();
        if (resp.ok) {
            vaultExists = false;
            document.getElementById('vault-delete-overlay').classList.add('is-hidden');
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
    preview.style.display = 'none';

    try {
        const resp = await fetch('/api/ansible/generate-token', { method: 'POST' });
        const data = await resp.json();
        if (resp.ok && data.status === 'ok') {
            _ansibleGeneratedToken = data.token;
            tokenVal.textContent = data.token;
            preview.style.display = '';
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
