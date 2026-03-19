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
            { key: 'vision', icon: '👁️', label: t('config.section.vision.label'), desc: t('config.section.vision.desc') }
        ]
    },
    {
        group: t('config.group.server_system'),
        items: [
            { key: 'server', icon: '🌐', label: t('config.section.server.label'), desc: t('config.section.server.desc') },
            { key: 'directories', icon: '📁', label: t('config.section.directories.label'), desc: t('config.section.directories.desc') },
            { key: 'sqlite', icon: '🗄️', label: t('config.section.sqlite.label'), desc: t('config.section.sqlite.desc') },
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
            { key: 'brave_search', icon: '🦁', label: t('config.section.brave_search.label'), desc: t('config.section.brave_search.desc') }
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
            { key: 'onedrive', icon: '☁️', label: t('config.section.onedrive.label'), desc: t('config.section.onedrive.desc'), customRender: 'renderOneDriveSection' },
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
            { key: 'rocketchat', icon: '🚀', label: t('config.section.rocketchat.label'), desc: t('config.section.rocketchat.desc') }
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
            { key: 'paperless_ngx', icon: '📄', label: t('config.section.paperless_ngx.label'), desc: t('config.section.paperless_ngx.desc') }
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
            { key: 'tailscale', icon: '🔒', label: t('config.section.tailscale.label'), desc: t('config.section.tailscale.desc') },
            { key: 'devices', icon: '📱', label: t('config.section.devices.label'), desc: t('config.section.devices.desc') },
            { key: 'proxmox', icon: '🖥️', label: t('config.section.proxmox.label'), desc: t('config.section.proxmox.desc') },
            { key: 'remote_control', icon: '📡', label: t('config.section.remote_control.label'), desc: t('config.section.remote_control.desc') },
            { key: 'meshcentral', icon: '🖥️', label: t('config.section.meshcentral.label'), desc: t('config.section.meshcentral.desc') },
            { key: 'ansible', icon: '⚙️', label: t('config.section.ansible.label'), desc: t('config.section.ansible.desc') }
        ]
    },
    {
        group: t('config.group.security'),
        items: [
            { key: 'secrets', icon: '🔐', label: t('config.section.secrets.label'), desc: t('config.section.secrets.desc') },
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
const SENSITIVE_KEYS = ['api_key', 'bot_token', 'password', 'app_password', 'access_token', 'token', 'user_key', 'app_token', 'master_key'];

// Provider management state (loaded from /api/providers)
let providersCache = [];

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
        // Load runtime environment capabilities (Docker mode, socket, broadcast, etc.)
        try {
            const rtResp = await fetch('/api/runtime');
            if (rtResp.ok) runtimeData = await rtResp.json();
        } catch (_) { }
    } catch (e) {
        document.getElementById('content').innerHTML = '<div style="text-align:center;padding:4rem;color:var(--danger);">❌ ' + t('config.loading_error') + '<br><small>' + e.message + '</small></div>';
        return;
    }
    buildSidebar();
    selectSection(activeSection);
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

async function selectSection(key) {
    activeSection = key;
    localStorage.setItem('aurago-cfg-section', key);
    document.querySelectorAll('.sidebar-item').forEach(el => el.classList.toggle('active', el.dataset.section === key));
    // Auto-expand the group containing this section if it is collapsed
    for (const group of SECTIONS) {
        if (group.items.some(s => s.key === key) && collapsedGroups.has(group.group)) {
            const groupDiv = [...document.querySelectorAll('.sidebar-group')].find(
                el => el.querySelector('.sidebar-group-title') &&
                    el.querySelector('.sidebar-group-title').textContent.trim() === group.group
            );
            if (groupDiv) toggleGroup(group.group, groupDiv);
            break;
        }
    }
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
            document.getElementById('content').innerHTML = '<div style="text-align:center;padding:3rem;color:var(--danger);">\u274c Module load error: ' + e.message + '</div>';
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
        html += `<div style="margin-bottom:1.2rem;padding:0.65rem 0.9rem;border-radius:9px;background:rgba(99,179,237,0.08);border:1px solid rgba(99,179,237,0.22);font-size:0.78rem;color:var(--text-secondary);line-height:1.55;">
                    \u{1F9E0} ${t('config.llm.info_banner')}
                </div>`;
    }

    // Embeddings explanation
    if (key === 'embeddings') {
        html += `<div style="margin-bottom:1.2rem;padding:0.65rem 0.9rem;border-radius:9px;background:rgba(99,179,237,0.08);border:1px solid rgba(99,179,237,0.22);font-size:0.78rem;color:var(--text-secondary);line-height:1.55;">
                    \u{1F9E0} ${t('config.embeddings.info_banner')}
                </div>`;
    }

    // Budget disclaimer
    if (key === 'budget') {
        html += `<div style="margin-bottom:1.2rem;padding:0.65rem 0.9rem;border-radius:9px;background:rgba(99,179,237,0.08);border:1px solid rgba(99,179,237,0.22);font-size:0.78rem;color:var(--text-secondary);line-height:1.55;">
                    ℹ️ ${t('config.budget.info_banner')}
                </div>`;
    }

    // Tools permissions warning
    if (key === 'tools') {
        html += `<div style="margin-bottom:1.2rem;padding:0.65rem 0.9rem;border-radius:9px;background:rgba(251,191,36,0.08);border:1px solid rgba(251,191,36,0.28);font-size:0.78rem;color:var(--text-secondary);line-height:1.55;">
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
                html += '<div style="margin-top:1rem;margin-bottom:0.5rem;font-weight:600;font-size:0.85rem;color:var(--accent);">' + formatKey(k) + '</div>';
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

    if (sectionBlocked) html += '</div>'; // End feature-unavailable-fields
    html += '</div>';

    document.getElementById('content').innerHTML = html;
}


function renderFields(fields, data, parentPath) {
    let html = '';
    for (const field of fields) {
        const val = data[field.yaml_key];
        const fullPath = parentPath + '.' + field.yaml_key;

        if (field.type === 'object' && field.children) {
            html += '<div style="margin-top:1.2rem;margin-bottom:0.5rem;font-weight:600;font-size:0.85rem;color:var(--accent);border-bottom:1px solid var(--border-subtle);padding-bottom:0.3rem;">' + formatKey(field.yaml_key) + '</div>';
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
        h += '<div class="field-label">Master Key <span style="font-size:0.65rem;color:var(--warning);">🔒</span></div>';
        if (helpText) h += '<div class="field-help">' + helpText + '</div>';
        if (vaultExists) {
            h += '<div class="password-wrap">';
            h += '<input class="field-input" type="password" value="\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022" disabled style="opacity:0.55;cursor:not-allowed;flex:1;">';
            h += '<button type="button" class="password-toggle" title="' + t('config.master_key.vault_delete_tooltip') + '" onclick="vaultDeletePrompt()" style="color:#f87171;font-size:1rem;">🗑️</button>';
            h += '</div>';
            h += '<div style="font-size:0.75rem;color:var(--text-secondary);margin-top:0.3rem;">🔐 ' + t('config.master_key.vault_exists') + '</div>';
        } else {
            h += '<div class="password-wrap">';
            h += '<input class="field-input" type="password" data-path="server.master_key" value="" placeholder="' + t('config.master_key.placeholder') + '" autocomplete="off">';
            h += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
            h += '</div>';
            h += '<div style="font-size:0.75rem;color:var(--warning);margin-top:0.3rem;">⚠️ ' + t('config.master_key.no_vault') + '</div>';
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
    if (isSensitive) html += ' <span style="font-size:0.65rem;color:var(--warning);">🔒</span>';
    html += '</div>';
    if (helpText) html += '<div class="field-help">' + helpText + '</div>';

    if (fieldType === 'bool') {
        const isOn = value === true;
        html += '<div class="toggle-wrap">';
        html += '<div class="toggle' + (isOn ? ' on' : '') + '" data-path="' + fullPath + '" onclick="toggleBool(this)"></div>';
        html += '<span class="toggle-label">' + (isOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
        html += '</div>';
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

        html += '<select class="field-select" data-path="' + fullPath + '" onchange="if(this.value===\'Other / Custom\'){this.nextElementSibling.style.display=\'block\';this.nextElementSibling.focus();}else if(this.nextElementSibling){this.nextElementSibling.style.display=\'none\';}">';
        helpOptions.forEach(opt => {
            const selected = (String(value) === String(opt) || (opt === 'Other / Custom' && isCustomVal)) ? ' selected' : '';
            html += '<option value="' + escapeAttr(opt) + '"' + selected + '>' + escapeAttr(opt) + '</option>';
        });
        html += '</select>';

        if (hasCustom) {
            const display = isCustomVal ? 'block' : 'none';
            const customVal = isCustomVal ? value : '';
            html += '<input class="field-input" type="text" data-custom-for="' + fullPath + '" value="' + escapeAttr(customVal) + '" placeholder="Type custom value..." style="display:' + display + '; margin-top:0.4rem;" oninput="markDirty()">';
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
            html += '<div style="padding:0.6rem 0.8rem;border-radius:8px;background:rgba(72,199,142,0.06);border:1px solid rgba(72,199,142,0.18);font-size:0.78rem;color:var(--text-secondary);">'
                + '💰 ' + t('config.budget.models_moved_hint')
                + '</div>';
        } else {
        const isObjArray = (Array.isArray(value) && value.length > 0 && typeof value[0] === 'object' && value[0] !== null);
        if (isObjArray) {
            const jsonVal = Array.isArray(value) ? JSON.stringify(value, null, 2) : '[]';
            html += '<textarea class="field-input" data-path="' + fullPath + '" data-type="json" rows="6" style="font-family:monospace;font-size:0.78rem;resize:vertical;white-space:pre;">' + escapeHtml(jsonVal) + '</textarea>';
            html += '<div style="font-size:0.72rem;color:var(--text-secondary);margin-top:0.3rem;">' + t('config.field.json_array_hint') + '</div>';
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
function escapeHtml(s) { return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;'); }

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

    // Collect all field values
    const patch = {};
    document.querySelectorAll('[data-path]').forEach(el => {
        // Skip unchecked radio buttons — only the selected radio should write its value
        if (el.type === 'radio' && !el.checked) return;

        const path = el.dataset.path;
        const parts = path.split('.');
        let val;

        if (el.classList.contains('toggle')) {
            val = el.classList.contains('on');
        } else if (el.type === 'number' || el.type === 'range') {
            val = el.value === '' ? 0 : (el.step && parseFloat(el.step) < 1 ? parseFloat(el.value) : parseInt(el.value));
        } else if (el.dataset.type === 'array') {
            // Check if this is an object array (budget.models) or string array
            if (path === 'budget.models' || el.value.trim().startsWith('[')) {
                // Object array - parse as JSON
                try { val = JSON.parse(el.value); } catch (e) {
                    val = el.value.split(',').map(s => s.trim()).filter(Boolean);
                }
            } else {
                // Simple string array
                val = el.value.split(',').map(s => s.trim()).filter(Boolean);
            }
        } else if (el.dataset.type === 'array-lines') {
            // Newline-separated string array
            val = el.value.split('\n').map(s => s.trim()).filter(Boolean);
        } else if (el.dataset.type === 'json') {
            try { val = JSON.parse(el.value); } catch (e) { val = el.value; }
        } else if (el.tagName === 'SELECT' && el.value === 'Other / Custom') {
            const customInput = document.querySelector('[data-custom-for="' + path + '"]');
            val = customInput ? customInput.value.trim() : el.value;
        } else {
            val = el.value;
        }

        // Build nested object from dotted path
        let obj = patch;
        for (let i = 0; i < parts.length - 1; i++) {
            if (!obj[parts[i]]) obj[parts[i]] = {};
            obj = obj[parts[i]];
        }
        obj[parts[parts.length - 1]] = val;
    });

    try {
        const resp = await fetch('/api/config', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(patch)
        });
        const result = await resp.json();
        if (resp.ok) {
            if (result.needs_restart) {
                status.className = 'save-status';
                status.style.color = 'var(--warning)';
                status.textContent = '⚠ ' + (result.message || t('config.save_bar.restart_needed'));
            } else {
                status.className = 'save-status success';
                status.textContent = '✓ ' + (result.message || t('config.save_bar.saved'));
            }
            // Refresh config data and reset dirty state
            const cfgResp = await fetch('/api/config');
            configData = await cfgResp.json();
            initialSnapshot = collectSnapshot();
            setDirty(false);
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

async function restartAuraGo() {
    if (!confirm(t('config.restart.confirm'))) return;

    try {
        const resp = await fetch('/api/restart', { method: 'POST' });
        if (resp.ok) {
            const res = await resp.json();
            document.body.innerHTML = `
                        <div style="display:flex;flex-direction:column;align-items:center;justify-content:center;height:100vh;background:var(--bg-primary);color:var(--text-primary);">
                            <div style="font-size:4rem;margin-bottom:1.5rem;animation:spin 2s linear infinite;">↻</div>
                            <h2 style="font-size:1.5rem;font-weight:600;">${res.message}</h2>
                            <p style="color:var(--text-secondary);margin-top:0.75rem;">${t('config.restart.reloading')}</p>
                            <style>@keyframes spin { 100% { transform: rotate(360deg); } }</style>
                        </div>
                    `;
            // Attempt to reload after 4 seconds to give the service time to restart
            setTimeout(() => window.location.reload(), 4000);
        } else {
            alert(t('config.restart.error'));
        }
    } catch (e) {
        // If the fetch fails immediately, it might be that the server died instantly.
        // We'll still show the reloading screen.
        document.body.innerHTML = '<div style="display:flex;align-items:center;justify-content:center;height:100vh;color:var(--text-secondary);">' + t('config.restart.disconnected') + '</div>';
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
    secrets: { m: 'secrets', fn: 'renderSecretsSection' },
    webhooks: { m: 'webhooks', fn: 'renderWebhooksSection' },
    prompts_editor: { m: 'prompts', fn: 'renderPromptsSection' },
    indexing: { m: 'indexing', fn: 'renderIndexingSection' },
    backup_restore: { m: 'backup', fn: 'renderBackupSection' },
    updates: { m: 'updates', fn: 'renderUpdatesSection' },
    devices: { m: 'devices', fn: 'renderDevicesSection' },
    chromecast: { m: 'chromecast', fn: 'renderChromecastSection' },
    adguard: { m: 'adguard', fn: 'renderAdGuardSection' },
    fritzbox: { m: 'fritzbox', fn: 'renderFritzBoxSection' },
    paperless_ngx: { m: 'paperless', fn: 'renderPaperlessSection' },
    homepage: { m: 'homepage', fn: 'renderHomepageSection' },
    netlify: { m: 'netlify', fn: 'renderNetlifySection' },
    danger_zone: { m: 'danger', fn: 'renderDangerZoneSection' },
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
    onedrive: { m: 'onedrive', fn: 'renderOneDriveSection' },
    tailscale: { m: 'tailscale', fn: 'renderTailscaleSection' },
    server: { m: 'server', fn: 'renderServerSection' },
    a2a: { m: 'a2a', fn: 'renderA2ASection' }
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
    document.getElementById('vault-delete-overlay').style.display = 'flex';
    setTimeout(() => document.getElementById('vault-confirm-input').focus(), 100);
}

function vaultCheckWord() {
    const word = t('config.vault.confirm_word_de');
    document.getElementById('vault-confirm-btn').disabled =
        document.getElementById('vault-confirm-input').value !== word;
}

function vaultDeleteCancel() {
    document.getElementById('vault-delete-overlay').style.display = 'none';
}

async function vaultDeleteConfirm() {
    document.getElementById('vault-confirm-btn').disabled = true;
    try {
        const resp = await fetch('/api/vault', { method: 'DELETE' });
        const data = await resp.json();
        if (resp.ok) {
            vaultExists = false;
            document.getElementById('vault-delete-overlay').style.display = 'none';
            const cfgResp = await fetch('/api/config');
            configData = await cfgResp.json();
            selectSection('server');
            const toast = document.createElement('div');
            toast.style.cssText = 'position:fixed;top:1rem;right:1rem;z-index:9999;background:var(--surface-elevated);border:1px solid rgba(239,68,68,0.35);border-radius:10px;padding:0.75rem 1.25rem;font-size:0.85rem;color:#fca5a5;box-shadow:0 8px 30px rgba(0,0,0,0.3);max-width:420px;';
            toast.textContent = t('config.vault.deleted_toast');
            document.body.appendChild(toast);
            setTimeout(() => toast.remove(), 6000);
        } else {
            alert(data.message || t('config.save_bar.error'));
            document.getElementById('vault-confirm-btn').disabled = false;
        }
    } catch (e) {
        alert(t('config.common.network_error') + ' ' + e.message);
        document.getElementById('vault-confirm-btn').disabled = false;
    }
}

