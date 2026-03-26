// AuraGo – setup page logic
// Extracted from setup.html

// ── State ────────────────────────────────────
let currentStep = 0;
const totalSteps = 4;
let saving = false;

// ── Load Personality Profiles on startup ─────
(async function loadPersonalities() {
    try {
        const resp = await fetch('/api/personalities');
        if (!resp.ok) return;
        const data = await resp.json();
        const profiles = data.personalities || [];
        const active = data.active || 'friend';
        const sel = document.getElementById('core-personality');
        if (!sel || profiles.length === 0) return;
        sel.innerHTML = '';
        for (const p of profiles) {
            const name = p.name || p; // API returns objects {name, core}
            const opt = document.createElement('option');
            opt.value = name;
            opt.textContent = name.charAt(0).toUpperCase() + name.slice(1);
            if (name === active || name === 'friend') opt.selected = true;
            sel.appendChild(opt);
        }
        // Ensure 'friend' is selected by default for setup
        const names = profiles.map(p => p.name || p);
        if (names.includes('friend')) sel.value = 'friend';
        else if (active && names.includes(active)) sel.value = active;
    } catch (e) { /* ignore — fallback option remains */ }
})();

// ── Provider Config Map ──────────────────────
const providerConfig = {
    openrouter: {
        baseUrl: 'https://openrouter.ai/api/v1',
        placeholder: 'sk-or-v1-...',
        link: 'openrouter.ai/keys',
        defaultModel: 'arcee-ai/trinity-large-preview:free',
        needsKey: true,
    },
    openai: {
        baseUrl: 'https://api.openai.com/v1',
        placeholder: 'sk-...',
        link: 'platform.openai.com/api-keys',
        defaultModel: 'gpt-4o',
        needsKey: true,
    },
    anthropic: {
        baseUrl: 'https://api.anthropic.com/v1',
        placeholder: 'sk-ant-...',
        link: 'console.anthropic.com/settings/keys',
        defaultModel: 'claude-sonnet-4-20250514',
        needsKey: true,
    },
    google: {
        baseUrl: 'https://generativelanguage.googleapis.com/v1beta/openai/',
        placeholder: 'AIza...',
        link: 'aistudio.google.com/apikey',
        defaultModel: 'gemini-2.5-flash',
        needsKey: true,
    },
    ollama: {
        baseUrl: 'http://localhost:11434/v1',
        placeholder: t('setup.provider_ollama_key_placeholder'),
        link: 'ollama.com',
        defaultModel: 'llama3.1',
        needsKey: false,
    },
    custom: {
        baseUrl: '',
        placeholder: 'API Key...',
        link: '',
        defaultModel: '',
        needsKey: true,
    },
};

// ── Helpers ───────────────────────────────────
function escapeAttr(s) { return String(s).replace(/"/g, '&quot;').replace(/</g, '&lt;'); }
function escapeHtml(s) { return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;'); }
function setupSetHidden(el, hidden) {
    if (!el) return;
    el.classList.toggle('is-hidden', hidden);
}

/* ── OpenRouter Model Browser (reusable modal) ── */
let _orModelsCache = null;
let _orModelsCacheTime = 0;
const OR_CACHE_TTL = 5 * 60 * 1000;

async function openOpenRouterBrowser(onSelect) {

    const existing = document.getElementById('or-browser-overlay');
    if (existing) existing.remove();

    const overlay = document.createElement('div');
    overlay.id = 'or-browser-overlay';
    overlay.className = 'or-browser-overlay';
    overlay.onclick = (e) => { if (e.target === overlay) overlay.remove(); };

    overlay.innerHTML = `
            <div class="or-browser-modal" onclick="event.stopPropagation()">
                <div class="or-browser-header">
                    <div class="or-browser-title-row">
                        <div class="or-browser-title">${t('setup.or_browser_title')}</div>
                        <button onclick="document.getElementById('or-browser-overlay').remove()" class="or-browser-close-btn">✕</button>
                    </div>
                    <div class="or-browser-controls">
                        <input id="or-search" class="field-input or-search-input" type="text" placeholder="${t('setup.or_browser_search_placeholder')}">
                        <button id="or-free-btn" class="or-free-btn" title="${t('setup.or_browser_free_button_title')}">
                            ${t('setup.or_browser_free_button')}
                        </button>
                        <div id="or-count" class="or-count"></div>
                    </div>
                </div>
                <div class="or-browser-body">
                    <div id="or-list-wrap" class="or-list-wrap">
                        <div id="or-list" class="or-list"></div>
                    </div>
                    <div id="or-detail" class="or-detail"></div>
                </div>
                <div id="or-loading" class="or-loading">
                    ${t('setup.or_browser_loading')}
                </div>
            </div>`;
    document.body.appendChild(overlay);

    const searchInput = document.getElementById('or-search');
    const freeBtn = document.getElementById('or-free-btn');
    const listDiv = document.getElementById('or-list');
    const detailDiv = document.getElementById('or-detail');
    const countDiv = document.getElementById('or-count');
    const loadingDiv = document.getElementById('or-loading');
    const listWrap = document.getElementById('or-list-wrap');

    let allModels = [];
    let freeOnly = false;
    let selectedModel = null;

    try {
        const now = Date.now();
        if (_orModelsCache && (now - _orModelsCacheTime) < OR_CACHE_TTL) {
            allModels = _orModelsCache;
        } else {
            const resp = await fetch('/api/openrouter/models');
            const json = await resp.json();
            if (json.available === false) {
                loadingDiv.innerHTML = '<span class="or-loading-error">❌ ' + (json.reason || t('common.error')) + '</span>';
                return;
            }
            allModels = (json.data || []).map(m => ({
                id: m.id || '',
                name: m.name || m.id || '',
                description: m.description || '',
                context_length: m.context_length || 0,
                pricing: m.pricing || {},
            }));
            allModels.sort((a, b) => a.name.localeCompare(b.name));
            _orModelsCache = allModels;
            _orModelsCacheTime = now;
        }
    } catch (e) {
        loadingDiv.innerHTML = '<span class="or-loading-error">❌ ' + t('setup.or_browser_connection_error') + e.message + '</span>';
        return;
    }
    setupSetHidden(loadingDiv, true);
    setupSetHidden(listWrap, false);

    function isFree(m) {
        return m.pricing && parseFloat(m.pricing.prompt || '1') === 0 && parseFloat(m.pricing.completion || '1') === 0;
    }
    function formatCost(perToken) {
        const val = parseFloat(perToken || '0');
        if (val === 0) return t('setup.or_browser_free_cost');
        const perMillion = val * 1000000;
        if (perMillion < 0.01) return '$' + perMillion.toFixed(4) + '/M';
        if (perMillion < 1) return '$' + perMillion.toFixed(3) + '/M';
        return '$' + perMillion.toFixed(2) + '/M';
    }
    function formatContext(ctx) {
        if (!ctx) return '—';
        if (ctx >= 1000000) return (ctx / 1000000).toFixed(1) + 'M';
        if (ctx >= 1000) return Math.round(ctx / 1000) + 'K';
        return ctx.toString();
    }

    function renderList() {
        const query = searchInput.value.toLowerCase().trim();
        const filtered = allModels.filter(m => {
            if (freeOnly && !isFree(m)) return false;
            if (query) return m.id.toLowerCase().includes(query) || m.name.toLowerCase().includes(query);
            return true;
        });
        countDiv.textContent = filtered.length + ' ' + t('setup.or_browser_model_count') + (freeOnly ? ' ' + t('setup.or_browser_model_count_free_only') : '');
        if (filtered.length === 0) {
            listDiv.innerHTML = '<div class="or-empty-list">' + t('setup.or_browser_no_models_found') + '</div>';
            return;
        }
        listDiv.innerHTML = filtered.map(m => {
            const free = isFree(m);
            const promptCost = formatCost(m.pricing.prompt);
            const completionCost = formatCost(m.pricing.completion);
            const isSelected = selectedModel && selectedModel.id === m.id;
            return `<div class="or-model-row ${isSelected ? 'is-selected' : ''}" data-id="${escapeAttr(m.id)}">
                        <div class="or-model-main">
                            <div class="or-model-name" title="${escapeAttr(m.id)}">${escapeHtml(m.name)}</div>
                            <div class="or-model-id">${escapeHtml(m.id)}</div>
                        </div>
                        <div class="or-model-meta">
                            ${free ? '<span class="or-free-badge">' + t('setup.or_browser_free_badge') + '</span>' : '<span class="or-meta-text" title="Input / Output">' + promptCost + ' · ' + completionCost + '</span>'}
                            <span class="or-meta-text" title="Context">${formatContext(m.context_length)}</span>
                        </div>
                    </div>`;
        }).join('');
        listDiv.querySelectorAll('.or-model-row').forEach(row => {
            row.onclick = () => {
                const m = allModels.find(x => x.id === row.dataset.id);
                if (!m) return;
                selectedModel = m;
                showDetail(m);
                listDiv.querySelectorAll('.or-model-row').forEach(r => r.classList.remove('is-selected'));
                row.classList.add('is-selected');
            };
        });
    }

    function showDetail(m) {
        detailDiv.classList.add('is-open');
        const free = isFree(m);
        const promptPerM = (parseFloat(m.pricing.prompt || '0') * 1000000);
        const completionPerM = (parseFloat(m.pricing.completion || '0') * 1000000);
        detailDiv.innerHTML = `
                    <div class="or-detail-name">${escapeHtml(m.name)}</div>
                    <div class="or-detail-id">${escapeHtml(m.id)}</div>
                    ${m.description ? '<div class="or-detail-desc">' + escapeHtml(m.description) + '</div>' : ''}
                    <div class="or-detail-grid">
                        <div><span class="or-detail-key">${t('setup.or_browser_detail_context')}</span></div>
                        <div class="or-detail-val">${formatContext(m.context_length)} ${t('setup.or_browser_detail_tokens')}</div>
                        <div><span class="or-detail-key">${t('setup.or_browser_detail_input')}</span></div>
                        <div class="or-detail-val ${free ? 'is-free' : ''}">${free ? t('setup.or_browser_free_cost') : '$' + promptPerM.toFixed(4) + '/M'}</div>
                        <div><span class="or-detail-key">${t('setup.or_browser_detail_output')}</span></div>
                        <div class="or-detail-val ${free ? 'is-free' : ''}">${free ? t('setup.or_browser_free_cost') : '$' + completionPerM.toFixed(4) + '/M'}</div>
                    </div>
                    ${free ? '<div class="or-free-detail-note">' + t('setup.or_browser_free_model_badge') + '</div>' : ''}
                    <button id="or-apply-btn" class="or-apply-btn">
                        ${t('setup.or_browser_apply_model')}
                    </button>
                `;
        document.getElementById('or-apply-btn').onclick = () => {
            if (onSelect) onSelect({
                id: m.id,
                name: m.name,
                pricing: m.pricing,
                context_length: m.context_length,
                inputPerMillion: parseFloat(m.pricing.prompt || '0') * 1000000,
                outputPerMillion: parseFloat(m.pricing.completion || '0') * 1000000,
            });
            overlay.remove();
        };
    }

    let searchTimer;
    searchInput.oninput = () => {
        clearTimeout(searchTimer);
        searchTimer = setTimeout(renderList, 150);
    };
    freeBtn.onclick = () => {
        freeOnly = !freeOnly;
        freeBtn.classList.toggle('is-active', freeOnly);
        renderList();
    };
    renderList();
    searchInput.focus();
}

// ── Provider Change Handler ──────────────────
function onProviderChange() {
    const provider = document.getElementById('llm-provider').value;
    const cfg = providerConfig[provider] || providerConfig.custom;

    document.getElementById('llm-base-url').value = cfg.baseUrl;
    document.getElementById('llm-base-url').placeholder = cfg.baseUrl || 'https://...';
    document.getElementById('llm-api-key').placeholder = cfg.placeholder;
    document.getElementById('llm-model').placeholder = cfg.defaultModel;
    document.getElementById('provider-link').textContent = cfg.link;

    // Pre-fill model if empty
    const modelInput = document.getElementById('llm-model');
    if (!modelInput.value && cfg.defaultModel) {
        modelInput.value = cfg.defaultModel;
    }

    // Show/hide API key field for Ollama
    const apiKeyGroup = document.getElementById('group-api-key');
    if (!cfg.needsKey) {
        apiKeyGroup.classList.add('setup-dimmed');
        document.getElementById('llm-api-key').removeAttribute('required');
    } else {
        apiKeyGroup.classList.remove('setup-dimmed');
    }

    // Show/hide OpenRouter browse button
    const orBrowseBtn = document.getElementById('or-browse-btn');
    if (orBrowseBtn) {
        setupSetHidden(orBrowseBtn, provider !== 'openrouter');
    }
}

// ── Whisper Provider Change Handler ─────────
// Auto-selects the appropriate transcription mode when the user changes the
// Whisper provider so they don't have to change it manually.
function onWhisperProviderChange() {
    const provider = document.getElementById('whisper-provider').value;
    const modeEl = document.getElementById('whisper-mode');
    if (!modeEl) return;
    if (provider === 'openai') {
        modeEl.value = 'whisper';
    } else if (provider === 'ollama') {
        modeEl.value = 'local';
    } else if (provider === 'openrouter') {
        modeEl.value = 'multimodal';
    }
}

// ── Agent Language Change Handler ────────────
function onLanguageChange() {
    const sel = document.getElementById('system-language');
    const customInput = document.getElementById('system-language-custom');
    if (sel.value === 'Other / Custom') {
        setupSetHidden(customInput, false);
        customInput.focus();
    } else {
        setupSetHidden(customInput, true);
        // Fetch and apply translations for the selected language immediately
        fetchAndApplyLang(sel.value);
    }
}

function fetchAndApplyLang(langValue) {
    fetch('/api/i18n?lang=' + encodeURIComponent(langValue))
        .then(r => r.ok ? r.json() : null)
        .then(json => {
            if (json && json.data && typeof json.data === 'object') {
                I18N = json.data;
                applyI18N();
            }
        })
        .catch(() => { /* silently ignore — UI stays in current language */ });
}

// ── Personality V2 Toggle ────────────────────
function onPersonalityToggle() {
    const fields = document.getElementById('personality-v2-fields');
    const checked = document.getElementById('personality-v2').checked;
    fields.classList.toggle('visible', checked);
}

// ── Step Navigation ──────────────────────────
function goToStep(step) {
    if (step < 0 || step >= totalSteps) return;
    // Only allow going back or to completed steps
    if (step > currentStep) return;

    currentStep = step;
    updateUI();
}

function nextStep(skip = false) {
    if (currentStep === 0 && !skip) {
        if (!validateStep0()) return;
    }

    if (currentStep < totalSteps - 1) {
        currentStep++;
        updateUI();
    } else {
        // Final step — save
        saveConfig();
    }
}

function prevStep() {
    if (currentStep > 0) {
        currentStep--;
        updateUI();
    }
}

function updateUI() {
    // Update sections
    document.querySelectorAll('.setup-section').forEach((s, i) => {
        s.classList.toggle('active', i === currentStep);
    });

    // Update step dots
    document.querySelectorAll('.step-dot').forEach((dot, i) => {
        dot.classList.toggle('active', i === currentStep);
        dot.classList.toggle('completed', i < currentStep);
    });

    // Update lines
    for (let i = 0; i < totalSteps - 1; i++) {
        const line = document.getElementById(`line-${i}-${i + 1}`);
        if (line) line.classList.toggle('completed', i < currentStep);
    }

    // Update buttons
    setupSetHidden(document.getElementById('btn-back'), currentStep <= 0);
    setupSetHidden(document.getElementById('btn-skip-step'), currentStep <= 0);

    const btnNext = document.getElementById('btn-next');
    if (currentStep === totalSteps - 1) {
        btnNext.innerHTML = t('setup.nav_save_and_start');
    } else {
        btnNext.innerHTML = t('setup.nav_next');
    }
}

// ── Validation ───────────────────────────────
function validateStep0() {
    let valid = true;
    const provider = document.getElementById('llm-provider').value;
    const cfg = providerConfig[provider];

    // API Key required (except Ollama)
    if (cfg && cfg.needsKey) {
        const apiKey = document.getElementById('llm-api-key').value.trim();
        if (!apiKey) {
            showFieldError('llm-api-key', 'err-api-key');
            valid = false;
        } else {
            clearFieldError('llm-api-key', 'err-api-key');
        }
    }

    // Model required
    const model = document.getElementById('llm-model').value.trim();
    if (!model) {
        showFieldError('llm-model', 'err-model');
        valid = false;
    } else {
        clearFieldError('llm-model', 'err-model');
    }

    return valid;
}

function showFieldError(inputId, errorId) {
    document.getElementById(inputId).classList.add('error');
    document.getElementById(errorId).classList.add('visible');
}

function clearFieldError(inputId, errorId) {
    document.getElementById(inputId).classList.remove('error');
    document.getElementById(errorId).classList.remove('visible');
}

// Clear errors on input
document.addEventListener('input', (e) => {
    if (e.target.classList.contains('error')) {
        e.target.classList.remove('error');
        const errEl = e.target.parentElement.querySelector('.field-error');
        if (errEl) errEl.classList.remove('visible');
    }
});

// ── Build Provider Entries ───────────────────
// Creates provider entries from setup wizard fields.
// Each subsystem (embeddings, vision, whisper, personality_v2) gets its own
// provider entry so it can have a different model while sharing connection details.
function buildProviderEntries() {
    const mainType = document.getElementById('llm-provider').value;
    const mainCfg = providerConfig[mainType] || providerConfig.custom;
    const mainUrl = document.getElementById('llm-base-url').value.trim() || mainCfg.baseUrl;
    const mainKey = document.getElementById('llm-api-key').value.trim();
    const mainModel = document.getElementById('llm-model').value.trim();

    const providers = [];

    // Main LLM provider (always created)
    providers.push({
        id: 'main', name: 'Main LLM', type: mainType,
        base_url: mainUrl, api_key: mainKey, model: mainModel,
    });

    // Embeddings provider
    const embProvValue = document.getElementById('emb-provider').value;
    if (embProvValue && embProvValue !== '') {
        const embModel = document.getElementById('emb-model').value.trim();
        if (embProvValue === 'ollama') {
            providers.push({
                id: 'embeddings', name: 'Embeddings', type: 'ollama',
                base_url: providerConfig.ollama.baseUrl, api_key: '', model: embModel,
            });
        } else {
            // "internal" = same URL/key as main, different model
            providers.push({
                id: 'embeddings', name: 'Embeddings', type: mainType,
                base_url: mainUrl, api_key: mainKey, model: embModel,
            });
        }
    }

    // Vision provider
    const visionType = document.getElementById('vision-provider').value;
    if (visionType) {
        const visionModel = document.getElementById('vision-model').value.trim();
        const visionCfg = providerConfig[visionType] || providerConfig.custom;
        providers.push({
            id: 'vision', name: 'Vision', type: visionType,
            base_url: visionCfg.baseUrl || mainUrl,
            api_key: visionType === 'ollama' ? '' : mainKey,
            model: visionModel,
        });
    }

    // Whisper / STT provider
    const whisperType = document.getElementById('whisper-provider').value;
    if (whisperType) {
        const whisperModel = document.getElementById('whisper-model').value.trim();
        const whisperCfg = providerConfig[whisperType] || providerConfig.custom;
        providers.push({
            id: 'whisper', name: 'Whisper / STT', type: whisperType,
            base_url: whisperCfg.baseUrl || mainUrl,
            api_key: whisperType === 'ollama' ? '' : mainKey,
            model: whisperModel,
        });
    }

    // Personality V2 provider
    if (document.getElementById('personality-v2').checked) {
        const v2Model = document.getElementById('v2-model').value.trim();
        if (v2Model) {
            providers.push({
                id: 'personality-v2', name: 'Personality V2', type: mainType,
                base_url: mainUrl, api_key: mainKey, model: v2Model,
            });
        }
    }

    return providers;
}

// ── Build Trust Level Patch ──────────────────
// Returns a config patch based on the selected trust level (1-4).
// Base tools (Memory, Knowledge Graph, Secrets Vault, Scheduler, Notes,
// Missions, Inventory, Memory Maintenance, Journal, Contacts) are always
// enabled at every level — they are harmless and essential.
function buildTrustLevelPatch(level) {
    const n = parseInt(level, 10) || 1;

    // Base tools — always enabled at all levels
    const baseTools = {
        memory:             { enabled: true, readonly: n <= 1 },
        knowledge_graph:    { enabled: true, readonly: n <= 1 },
        secrets_vault:      { enabled: true, readonly: n <= 1 },
        scheduler:          { enabled: true, readonly: n <= 1 },
        notes:              { enabled: true, readonly: n <= 1 },
        missions:           { enabled: true, readonly: n <= 1 },
        inventory:          { enabled: true },
        memory_maintenance: { enabled: true },
        journal:            { enabled: true, readonly: n <= 1 },
        contacts:           { enabled: true },
    };

    // Extended tools — vary by level
    const extTools = {
        web_scraper:              { enabled: n >= 2 },
        web_capture:              { enabled: n >= 2 },
        network_ping:             { enabled: true },
        network_scan:             { enabled: n >= 2 },
        stop_process:             { enabled: n >= 3 },
        form_automation:          { enabled: n >= 3 },
        wol:                      { enabled: n >= 3 },
        upnp_scan:                { enabled: n >= 3 },
        skill_manager:            { enabled: n >= 2, readonly: n <= 2 },
        python_secret_injection:  { enabled: n >= 3 },
        document_creator:         { enabled: true },
    };

    const patch = {
        agent: {
            allow_shell:            n >= 3,
            allow_python:           n >= 3,
            allow_filesystem_write: n >= 3,
            allow_network_requests: n >= 3,
            allow_remote_shell:     n >= 4,
            allow_self_update:      n >= 4,
            allow_mcp:              n >= 4,
            sudo_enabled:           n >= 4,
        },
        tools: Object.assign({}, baseTools, extTools),
        sandbox: {
            enabled:         n >= 2,
            network_enabled: n >= 3,
        },
        shell_sandbox: {
            enabled: n >= 3,
        },
        homepage: {
            enabled:                    n >= 3,
            allow_deploy:               n >= 4,
            allow_container_management: n >= 3,
            allow_local_server:         n >= 4,
        },
        co_agents: {
            enabled: n >= 3,
        },
        // Integration readonly flags — true for L1/L2, false for L3/L4
        discord:          { readonly: n <= 2 },
        email:            { readonly: n <= 2 },
        home_assistant:   { readonly: n <= 2 },
        fritzbox:         { readonly: n <= 2 },
        telnyx:           { readonly: n <= 2 },
        meshcentral:      { readonly: n <= 2 },
        docker:           { readonly: n <= 2 },
        proxmox:          { readonly: n <= 2 },
        ollama:           { readonly: n <= 2 },
        ansible:          { readonly: n <= 2 },
        webdav:           { readonly: n <= 2 },
        koofr:            { readonly: n <= 2 },
        s3:               { readonly: n <= 2 },
        paperless_ngx:    { readonly: n <= 2 },
        onedrive:         { readonly: n <= 2 },
        truenas:          { readonly: n <= 2, allow_destructive: n >= 4 },
        mqtt:             { readonly: n <= 2 },
        adguard:          { readonly: n <= 2 },
        tailscale:        { readonly: n <= 2 },
        cloudflare_tunnel:{ readonly: n <= 2 },
        github:           { readonly: n <= 2 },
        webhooks:         { readonly: n <= 2 },
        n8n:              { readonly: n <= 2 },
        google_workspace: { readonly: n <= 2 },
        netlify:          { readonly: n <= 2 },
        invasion_control: { readonly: n <= 2 },
        remote_control:   { readonly: n <= 2 },
    };

    // Level 4: enable lifeboat self-update
    if (n >= 4) {
        patch.maintenance = { lifeboat_enabled: true };
    }

    return patch;
}

// ── Deep merge helper for trust level patch ──
function deepMergePatch(target, source) {
    for (const key of Object.keys(source)) {
        if (source[key] && typeof source[key] === 'object' && !Array.isArray(source[key])) {
            if (!target[key] || typeof target[key] !== 'object') target[key] = {};
            deepMergePatch(target[key], source[key]);
        } else {
            target[key] = source[key];
        }
    }
    return target;
}

// ── Build Config Patch ───────────────────────
// Returns the config patch with provider references (no inline API keys/URLs).
// Provider entries carry all connection details separately.
function buildConfigPatch() {
    const patch = {
        providers: buildProviderEntries(),
        server: {
            ui_language: document.documentElement.lang || 'en',
        },
        llm: {
            provider: 'main',
            use_native_functions: document.getElementById('native-functions').checked,
        },
        agent: {
            system_language: document.getElementById('system-language').value === 'Other / Custom' ? document.getElementById('system-language-custom').value.trim() : document.getElementById('system-language').value,
            personality_engine_v2: document.getElementById('personality-v2').checked,
            personality_engine: document.getElementById('personality-v2').checked,
            core_personality: document.getElementById('core-personality').value,
        },
        maintenance: {
            enabled: document.getElementById('maintenance-enabled').checked,
        },
        web_config: {
            enabled: document.getElementById('web-config-enabled').checked,
        },
    };

    // Embeddings: reference provider entry or disable
    const embProvider = document.getElementById('emb-provider').value;
    patch.embeddings = { provider: (embProvider && embProvider !== '') ? 'embeddings' : 'disabled' };

    // Vision: reference provider entry
    const visionProvider = document.getElementById('vision-provider').value;
    if (visionProvider) {
        patch.vision = { provider: 'vision' };
    }

    // Whisper: reference provider entry + transcription mode
    const whisperProvider = document.getElementById('whisper-provider').value;
    if (whisperProvider) {
        const whisperMode = document.getElementById('whisper-mode').value;
        patch.whisper = { provider: 'whisper', mode: whisperMode || 'multimodal' };
    }

    // Personality V2: reference provider entry
    if (patch.agent.personality_engine_v2) {
        const v2Model = document.getElementById('v2-model').value.trim();
        if (v2Model) {
            patch.agent.personality_v2_provider = 'personality-v2';
        }
    }

    // Trust level (Mutprobe): merge permission config
    const trustRadio = document.querySelector('input[name="trust-level"]:checked');
    if (trustRadio) {
        const trustPatch = buildTrustLevelPatch(trustRadio.value);
        deepMergePatch(patch, trustPatch);
    }

    return patch;
}

// ── Save Config ──────────────────────────────
async function saveConfig() {
    if (saving) return;
    saving = true;

    const btnNext = document.getElementById('btn-next');
    btnNext.disabled = true;
    btnNext.innerHTML = '<div class="spinner"></div> ' + t('setup.nav_saving');

    try {
        const patch = buildConfigPatch();
        const resp = await fetch('/api/setup', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(patch),
        });

        if (!resp.ok) {
            const text = await resp.text();
            throw new Error(text || `HTTP ${resp.status}`);
        }

        const result = await resp.json();

        // Show success screen
        document.querySelectorAll('.setup-section').forEach(s => s.classList.remove('active'));
        setupSetHidden(document.getElementById('setup-footer'), true);
        setupSetHidden(document.querySelector('.step-indicator'), true);
        document.getElementById('success-screen').classList.add('active');

        if (result.needs_restart) {
            setupSetHidden(document.getElementById('restart-notice'), false);
        }

        showToast(t('setup.toast_config_saved'), 'success');
    } catch (err) {
        showToast(t('setup.toast_error_prefix') + err.message, 'error');
        btnNext.disabled = false;
        btnNext.innerHTML = t('setup.nav_save_and_start');
    } finally {
        saving = false;
    }
}

// ── Skip Setup ───────────────────────────────
function skipSetup() {
    if (confirm(t('setup.confirm_skip_setup'))) {
        window.location.href = '/?skip_setup=1';
    }
}

// ── Toast Notifications ──────────────────────
function showToast(message, type = 'info') {
    const container = document.getElementById('toastContainer');
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    const icons = { success: '✓', error: '✕', warning: '⚠', info: 'ℹ' };
    toast.innerHTML = `<span class="setup-toast-icon">${icons[type] || ''}</span> ${message}`;
    container.appendChild(toast);
    setTimeout(() => {
        toast.classList.add('toast-exit');
        setTimeout(() => toast.remove(), 300);
    }, 4000);
}

// ── Browser Language Auto-detection ──────────
(function detectAndSetLanguage() {
    const lang = (navigator.languages && navigator.languages[0]) || navigator.language || 'en';
    const base = lang.toLowerCase().split('-')[0];
    const map = {
        'de': 'Deutsch',
        'en': 'English',
        'es': 'Español',
        'fr': 'Français',
        'pl': 'Polski',
        'zh': '中文',
        'hi': 'हिन्दी',
        'nl': 'Nederlands',
        'it': 'Italiano',
        'pt': 'Português',
        'da': 'Dansk',
        'ja': '日本語',
        'sv': 'Svenska',
        'no': 'Norsk',
        'cs': 'Čeština',
    };
    const sel = document.getElementById('system-language');
    sel.value = map[base] || 'English';
})();

// ── i18n: populate text ──
function applyI18N() {
    const el = id => document.getElementById(id);
    // Page title
    document.title = t('setup.page_title');
    // Header
    el('header-subtitle').textContent = t('setup.header_subtitle');
    el('btn-skip-setup').textContent = t('setup.skip_button');
    el('btn-skip-setup').title = t('setup.skip_button_title');
    // Step 0
    el('badge-step0').textContent = t('setup.step0_badge');
    el('title-step0').textContent = t('setup.step0_title');
    el('desc-step0').textContent = t('setup.step0_description');
    el('lbl-provider').innerHTML = t('setup.step0_provider_label') + ' <span class="required-star">*</span>';
    el('hint-provider').textContent = t('setup.step0_provider_hint');
    // Provider select options
    const provSel = el('llm-provider');
    provSel.querySelector('[value="openrouter"]').textContent = t('setup.step0_provider_openrouter');
    provSel.querySelector('[value="openai"]').textContent = t('setup.step0_provider_openai');
    provSel.querySelector('[value="anthropic"]').textContent = t('setup.step0_provider_anthropic');
    provSel.querySelector('[value="google"]').textContent = t('setup.step0_provider_google');
    provSel.querySelector('[value="ollama"]').textContent = t('setup.step0_provider_ollama');
    provSel.querySelector('[value="custom"]').textContent = t('setup.step0_provider_custom');
    // API Key
    el('lbl-api-key').innerHTML = t('setup.step0_api_key_label') + ' <span class="required-star">*</span>';
    el('err-api-key').textContent = t('setup.step0_api_key_error');
    el('hint-api-key-text').textContent = t('setup.step0_api_key_hint');
    // Base URL
    el('lbl-base-url').textContent = t('setup.step0_base_url_label');
    el('hint-base-url').textContent = t('setup.step0_base_url_hint');
    // Model
    el('lbl-model').innerHTML = t('setup.step0_model_label') + ' <span class="required-star">*</span>';
    el('llm-model').placeholder = t('setup.step0_model_placeholder');
    el('or-browse-btn').textContent = t('setup.step0_model_browse');
    el('err-model').textContent = t('setup.step0_model_error');
    // Language
    el('lang-label').textContent = t('setup.step0_language_label');
    // Native Functions
    el('lbl-native-functions').textContent = t('setup.step0_native_functions_label');
    el('desc-native-functions').innerHTML = t('setup.step0_native_functions_desc');
    // Step 1
    el('badge-step1').textContent = t('setup.step1_badge');
    el('title-step1').textContent = t('setup.step1_title');
    el('desc-step1').textContent = t('setup.step1_description');
    // Embeddings
    el('heading-embeddings').textContent = t('setup.step1_embeddings_heading');
    el('lbl-emb-provider').textContent = t('setup.step1_embeddings_provider_label');
    const embSel = el('emb-provider');
    embSel.querySelector('[value="internal"]').textContent = t('setup.step1_embeddings_provider_internal');
    embSel.querySelector('[value="ollama"]').textContent = t('setup.step1_embeddings_provider_ollama');
    embSel.querySelector('[value=""]').textContent = t('setup.step1_embeddings_provider_disabled');
    el('lbl-emb-model').textContent = t('setup.step1_embeddings_model_label');
    el('lbl-emb-apikey').textContent = t('setup.step1_embeddings_apikey_label');
    el('emb-api-key').placeholder = t('setup.step1_embeddings_apikey_placeholder');
    el('hint-emb-apikey').textContent = t('setup.step1_embeddings_apikey_hint');
    // Vision
    el('heading-vision').textContent = t('setup.step1_vision_heading');
    el('lbl-vision-provider').textContent = t('setup.step1_vision_provider_label');
    const visSel = el('vision-provider');
    visSel.querySelector('[value="openrouter"]').textContent = t('setup.step1_vision_provider_openrouter');
    visSel.querySelector('[value="openai"]').textContent = t('setup.step1_vision_provider_openai');
    visSel.querySelector('[value="ollama"]').textContent = t('setup.step1_vision_provider_ollama');
    visSel.querySelector('[value=""]').textContent = t('setup.step1_vision_provider_disabled');
    el('lbl-vision-model').textContent = t('setup.step1_vision_model_label');
    // Whisper
    el('heading-whisper').textContent = t('setup.step1_whisper_heading');
    el('lbl-whisper-provider').textContent = t('setup.step1_whisper_provider_label');
    const whSel = el('whisper-provider');
    whSel.querySelector('[value="openrouter"]').textContent = t('setup.step1_whisper_provider_openrouter');
    whSel.querySelector('[value="openai"]').textContent = t('setup.step1_whisper_provider_openai');
    whSel.querySelector('[value="ollama"]').textContent = t('setup.step1_whisper_provider_ollama');
    whSel.querySelector('[value=""]').textContent = t('setup.step1_whisper_provider_disabled');
    el('lbl-whisper-model').textContent = t('setup.step1_whisper_model_label');
    el('lbl-whisper-mode').textContent = t('setup.step1_whisper_mode_label');
    const whModeSel = el('whisper-mode');
    whModeSel.querySelector('[value="whisper"]').textContent = t('setup.step1_whisper_mode_whisper');
    whModeSel.querySelector('[value="multimodal"]').textContent = t('setup.step1_whisper_mode_multimodal');
    whModeSel.querySelector('[value="local"]').textContent = t('setup.step1_whisper_mode_local');
    // Step 2
    el('badge-step2').textContent = t('setup.step2_badge');
    el('title-step2').textContent = t('setup.step2_title');
    el('desc-step2').textContent = t('setup.step2_description');
    // Personality V2
    el('lbl-personality-v2').textContent = t('setup.step2_personality_v2_label');
    el('desc-personality-v2').textContent = t('setup.step2_personality_v2_desc');
    // V2 fields
    el('lbl-v2-model').textContent = t('setup.step2_v2_model_label');
    el('hint-v2-model').textContent = t('setup.step2_v2_model_hint');
    el('lbl-v2-url').textContent = t('setup.step2_v2_url_label');
    el('v2-url').placeholder = t('setup.step2_v2_url_placeholder');
    el('lbl-v2-apikey').textContent = t('setup.step2_v2_apikey_label');
    el('v2-api-key').placeholder = t('setup.step2_v2_apikey_placeholder');
    el('hint-v2-apikey').textContent = t('setup.step2_v2_apikey_hint');
    // Core personality
    el('lbl-core-personality').textContent = t('setup.step2_core_personality_label');
    el('hint-core-personality').textContent = t('setup.step2_core_personality_hint');
    // Default option fallback
    const coreDefault = el('core-personality').querySelector('[value="friend"]');
    if (coreDefault) coreDefault.textContent = t('setup.step2_core_personality_default');
    // Maintenance
    el('lbl-maintenance').textContent = t('setup.step2_maintenance_label');
    el('desc-maintenance').textContent = t('setup.step2_maintenance_desc');
    // Web Config
    el('lbl-web-config').textContent = t('setup.step2_web_config_label');
    el('desc-web-config').textContent = t('setup.step2_web_config_desc');
    // Step 3 — Mutprobe (Trust Level)
    el('badge-step3').textContent = t('setup.step3_badge');
    el('title-step3').textContent = t('setup.step3_title');
    el('desc-step3').textContent = t('setup.step3_description');
    el('trust-title-1').textContent = t('setup.step3_level1_title');
    el('trust-desc-1').textContent = t('setup.step3_level1_desc');
    el('trust-title-2').textContent = t('setup.step3_level2_title');
    el('trust-desc-2').textContent = t('setup.step3_level2_desc');
    el('trust-title-3').textContent = t('setup.step3_level3_title');
    el('trust-desc-3').textContent = t('setup.step3_level3_desc');
    el('trust-title-4').textContent = t('setup.step3_level4_title');
    el('trust-desc-4').textContent = t('setup.step3_level4_desc');
    // Success
    el('success-title').textContent = t('setup.success_title');
    el('success-desc').innerHTML = t('setup.success_description').replace(/\n/g, '<br>');
    el('restart-notice').innerHTML = t('setup.success_restart_notice');
    el('btn-go-to-chat').textContent = t('setup.success_go_to_chat');
    // Footer
    el('btn-back').textContent = t('setup.nav_back');
    el('btn-skip-step').textContent = t('setup.nav_skip_step');
    el('btn-next').textContent = t('setup.nav_next');
}

// ── Init ─────────────────────────────────────
applyI18N();
onProviderChange();
updateUI();
