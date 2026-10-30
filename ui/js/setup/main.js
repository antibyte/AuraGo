// AuraGo – setup page logic
// Extracted from setup.html

// ── State ────────────────────────────────────
// Step IDs for each flow (plan-select is always index 0)
const QUICK_FLOW_STEPS  = ['plan-select', 'plan-quick', 'step-3'];
const CUSTOM_FLOW_STEPS = ['plan-select', 'step-0', 'step-1', 'step-2', 'step-3'];
// Label i18n keys for each logical step in each flow
const QUICK_FLOW_LABELS  = ['setup.step_label_plan', 'setup.step_label_quick', 'setup.step_label_3'];
const CUSTOM_FLOW_LABELS = ['setup.step_label_plan', 'setup.step_label_0', 'setup.step_label_1', 'setup.step_label_2', 'setup.step_label_3'];

let currentStepIndex = 0;   // index into the active flow array
let highestStepIndex = 0;
let isQuickFlow = false;
let selectedProfile = null;
let prevSelectedProfileId = null;
let profiles = [];
let saving = false;
let setupStepInFlight = false;
let setupSkipInFlight = false;
// Tri-state: null = unknown (don't show star yet), true = required, false = already set
let setupPasswordRequired = null;
let csrfToken = '';
const SETUP_BOOTSTRAP_STORAGE_KEY = 'aurago.setupBootstrapToken';
let setupBootstrapToken = readSetupBootstrapToken();
let setupOllamaBaseURL = 'http://localhost:11434/v1';
let setupIsDocker = false;
// Local Ollama detection: null = still checking, otherwise { reachable, models: [{ name, size, parameter_size, family }] }.
let setupOllamaDetection = null;
let setupOllamaChecking = false;
// Set only by the user's own clicks/keys on a plan card, never by the default preselection.
let setupProfileUserChosen = false;
let setupHTTPS = null;
let setupCompleted = false;   // success screen shown: keep wizard chrome (HTTPS notice) hidden

// Sequence counter for /api/i18n fetches. When the user rapidly switches
// languages, only the most recent fetch's response should be applied.
let langFetchSeq = 0;

// Centralized language metadata (single source of truth for setup wizard).
// Key = ISO 639-1 code; Value = Display name in that language.
const LANG_MAP = {
    de: 'Deutsch', en: 'English', es: 'Español', fr: 'Français',
    pl: 'Polski', zh: '中文', hi: 'हिन्दी', nl: 'Nederlands',
    it: 'Italiano', pt: 'Português', da: 'Dansk', ja: '日本語',
    sv: 'Svenska', no: 'Norsk', el: 'Ελληνικά', cs: 'Čeština',
};
const SUPPORTED_LANGS = Object.keys(LANG_MAP);

// Pure: the agent's working language. A free-text entry from the advanced section wins over
// the language chosen on the first page.
function setupAgentLanguage(code, customText) {
    const custom = String(customText || '').trim();
    return custom || LANG_MAP[code] || code;
}

// Derived helpers
function activeFlow()  { return isQuickFlow ? QUICK_FLOW_STEPS  : CUSTOM_FLOW_STEPS; }
function activeLabels(){ return isQuickFlow ? QUICK_FLOW_LABELS : CUSTOM_FLOW_LABELS; }
function currentStepId(){ return activeFlow()[currentStepIndex]; }
function totalSteps()  { return activeFlow().length; }

// ── Security: bootstrap token for setup from other devices ──
// The server log prints a one-time token and a /setup#bootstrap=<token> link.
// A URL fragment never reaches the server or its access log; the token moves
// to sessionStorage and leaves the address bar.
function readSetupBootstrapToken() {
    let token = '';
    const match = /(?:^|[#&])bootstrap=([^&]+)/.exec(window.location.hash || '');
    if (match) {
        try { token = decodeURIComponent(match[1]).trim(); } catch (_) { token = ''; }
        try { history.replaceState(null, '', window.location.pathname + window.location.search); } catch (_) { /* keep going */ }
    }
    try {
        if (token) sessionStorage.setItem(SETUP_BOOTSTRAP_STORAGE_KEY, token);
        else token = sessionStorage.getItem(SETUP_BOOTSTRAP_STORAGE_KEY) || '';
    } catch (_) { /* storage unavailable: keep the in-memory token */ }
    return token;
}

function storeSetupBootstrapToken(token) {
    setupBootstrapToken = token;
    try { sessionStorage.setItem(SETUP_BOOTSTRAP_STORAGE_KEY, token); } catch (_) { /* in-memory only */ }
}

// Headers for every setup write: CSRF plus, when known, the bootstrap token.
function setupRequestHeaders(extra) {
    const headers = Object.assign({ 'X-CSRF-Token': csrfToken }, extra || {});
    if (setupBootstrapToken) headers['X-Setup-Token'] = setupBootstrapToken;
    return headers;
}

function openSetupOwnerGate(vaultLocked, tokenRejected) {
    setupSetHidden(document.getElementById('setup-bootstrap-token-view'), vaultLocked);
    setupSetHidden(document.getElementById('setup-vault-locked'), !vaultLocked);
    setupSetHidden(document.getElementById('setup-bootstrap-error'), !tokenRejected);
    document.getElementById('setup-bootstrap-gate')?.classList.add('open');
    if (!vaultLocked) document.getElementById('setup-bootstrap-input')?.focus();
}

function closeSetupOwnerGate() {
    document.getElementById('setup-bootstrap-gate')?.classList.remove('open');
}

async function submitSetupBootstrapToken(event) {
    event.preventDefault();
    const input = document.getElementById('setup-bootstrap-input');
    const token = (input?.value || '').trim();
    if (!token) return;
    storeSetupBootstrapToken(token);
    await checkSetupStatus();
}

document.getElementById('setup-bootstrap-form')?.addEventListener('submit', submitSetupBootstrapToken);

// ── Security: check setup status & redirect if already configured ──
async function checkSetupStatus() {
    try {
        const headers = setupBootstrapToken ? { 'X-Setup-Token': setupBootstrapToken } : {};
        const resp = await fetch('/api/setup/status', { headers });
        if (!resp.ok) {
            showSetupConnectionWarning('status ' + resp.status);
            return;
        }
        const data = await resp.json();
        setupHTTPS = data.https || null;
        renderSetupHTTPSStatus();
        if (!data.needs_setup) {
            window.location.href = '/';
            return;
        }
        if (data.vault_locked) {
            openSetupOwnerGate(true, false);
            return;
        }
        if (data.csrf_token) csrfToken = data.csrf_token;
        setupIsDocker = data.is_docker === true;
        if (data.bootstrap_token_required && data.bootstrap_token_valid !== true) {
            openSetupOwnerGate(false, Boolean(setupBootstrapToken));
        } else {
            closeSetupOwnerGate();
            if (csrfToken) detectSetupOllama();
        }
        if (data.ollama_base_url) {
            setupOllamaBaseURL = data.ollama_base_url;
            if (typeof providerConfig !== 'undefined' && providerConfig.ollama) {
                providerConfig.ollama.baseUrl = setupOllamaBaseURL;
                const provider = document.getElementById('llm-provider');
                const baseURL = document.getElementById('llm-base-url');
                if (provider && baseURL && provider.value === 'ollama') {
                    baseURL.value = setupOllamaBaseURL;
                }
            }
        }
    } catch (e) {
        showSetupConnectionWarning(e.message || 'network error');
    }
}
checkSetupStatus();

// Display a non-blocking warning banner when the setup status endpoint is
// unreachable so the user gets actionable feedback before the final save
// returns 403. The banner is inserted directly after the header.
function showSetupConnectionWarning(detail) {
    if (document.getElementById('setup-conn-warning')) return; // already shown
    const header = document.querySelector('.setup-header');
    if (!header) return;
    const banner = document.createElement('div');
    banner.id = 'setup-conn-warning';
    banner.className = 'setup-conn-warning';
    banner.setAttribute('role', 'alert');
    // The language file may not be loaded yet; fall back to English instead of showing the raw key.
    const message = t('setup.status_unreachable', { detail: detail });
    banner.textContent = message === 'setup.status_unreachable'
        ? 'Could not reach setup status endpoint (' + detail + '). Reload the page once the server is ready.'
        : message;
    header.parentNode.insertBefore(banner, header.nextSibling);
}

// Show the browser's actual secure-context result, independently of the
// listener setting (localhost and HTTPS proxies may also qualify).
function renderSetupHTTPSStatus() {
    const transport = document.getElementById('setup-https-transport');
    if (!transport) return;
    transport.textContent = setupHTTPS ? (setupHTTPS.enabled
        ? t('setup.https_active').replace('{port}', String(setupHTTPS.port))
        : t('setup.https_inactive')) : '';
    document.getElementById('setup-https-certificate')?.classList.toggle('is-hidden', !(setupHTTPS?.enabled && setupHTTPS.cert_mode === 'selfsigned'));
    const secure = window.isSecureContext === true;
    document.getElementById('setup-secure-context').textContent = t(secure ? 'setup.secure_ready' : 'setup.secure_unavailable');
    const status = document.getElementById('setup-https-status');
    status.classList.toggle('setup-https-insecure', !secure);
    setupSetHidden(status, secure || setupCompleted);
}

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
        sel.addEventListener('change', updateCorePersonalityDesc);
        updateCorePersonalityDesc();
    } catch (e) { /* ignore — fallback option remains */ }
})();

// One-line description under the personality select; unknown (user-made) personalities get the generic line.
function updateCorePersonalityDesc() {
    const sel = document.getElementById('core-personality');
    const target = document.getElementById('core-personality-desc');
    if (!sel || !target) return;
    const key = 'setup.personality_desc_' + sel.value;
    const text = t(key);
    target.textContent = text !== key ? text : t('setup.personality_desc_custom');
}

(async function loadSecurityStatus() {
    try {
        const resp = await fetch('/api/security/status');
        if (!resp.ok) return;
        const data = await resp.json();
        setupPasswordRequired = !(data && data.auth && data.auth.password_set);
        applyI18N();
    } catch (e) { /* ignore — first-run stays password-required */ }
})();

// ── Provider Config Map ──────────────────────
const providerConfig = {
    openrouter: {
        baseUrl: 'https://openrouter.ai/api/v1',
        placeholder: 'sk-or-v1-...',
        link: 'openrouter.ai/keys',
        defaultModel: 'google/gemini-3.1-flash-lite',
        needsKey: true,
        nativeFunctions: true,
    },
    agnes: {
        baseUrl: 'https://apihub.agnes-ai.com/v1',
        placeholder: 'API Key...',
        link: 'platform.agnes-ai.com',
        defaultModel: 'agnes-2.0-flash',
        hintKey: 'setup.step0_provider_hint_agnes',
        needsKey: true,
        nativeFunctions: true,
    },
    openai: {
        baseUrl: 'https://api.openai.com/v1',
        placeholder: 'sk-...',
        link: 'platform.openai.com/api-keys',
        defaultModel: 'gpt-5.5',
        needsKey: true,
        nativeFunctions: true,
    },
    anthropic: {
        baseUrl: 'https://api.anthropic.com/v1',
        placeholder: 'sk-ant-...',
        link: 'console.anthropic.com/settings/keys',
        defaultModel: 'claude-sonnet-5-5',
        needsKey: true,
        nativeFunctions: true,
    },
    google: {
        baseUrl: 'https://generativelanguage.googleapis.com/v1beta/openai/',
        placeholder: 'AIza...',
        link: 'aistudio.google.com/apikey',
        defaultModel: 'gemini-3.5-flash',
        needsKey: true,
        nativeFunctions: true,
    },
    ollama: {
        baseUrl: setupOllamaBaseURL,
        placeholder: t('setup.provider_ollama_key_placeholder'),
        link: 'ollama.com',
        defaultModel: 'llama3.1',
        needsKey: false,
        nativeFunctions: false,
    },
    custom: {
        baseUrl: '',
        placeholder: 'API Key...',
        link: '',
        defaultModel: '',
        needsKey: true,
        nativeFunctions: true,
    },
};

// Pure: whether native function calling starts enabled for a main provider. Small local
// models (Ollama) start with it off; the custom provider matches the config template.
function setupNativeFunctionsDefault(provider) {
    const cfg = providerConfig[provider] || providerConfig.custom;
    return cfg.nativeFunctions === true;
}

// ── Provider Profile Loading ─────────────────
async function loadProfiles() {
    try {
        const resp = await fetch('/api/setup/profiles');
        if (!resp.ok) throw new Error(resp.statusText);
        const data = await resp.json();
        profiles = data.profiles || [];
        renderProfileCards(profiles);
        if (!selectedProfile) {
            const id = setupDefaultProfileId(profiles);
            if (id) selectProfile(id);
        }
        maybePreselectOllama();
    } catch (e) {
        const loading = document.getElementById('profile-loading');
        if (loading) loading.innerHTML = `<p class="profile-state-message profile-load-error">${escapeHtml(t('setup.plan_load_error'))}</p>`;
    }
}

// Profile preselected on first load: the recommended provider (never "custom").
function setupDefaultProfileId(list) {
    const p = (list || []).find(x => x && x.recommended && x.id !== 'custom');
    return p ? p.id : '';
}

// i18n keys of the optional features a quick profile really enables (mirrors buildQuickConfigPatch).
function setupQuickProfileFeatureKeys(p) {
    if (!p || !p.features) return [];
    const m = p.models || {};
    const map = [
        ['vision', 'setup.feature_vision'],
        ['whisper', 'setup.feature_speech_input'],
        ['embeddings', 'setup.feature_memory'],
        ['image_generation', 'setup.feature_images'],
        ['video_generation', 'setup.feature_video'],
        ['music_generation', 'setup.feature_music'],
    ];
    return map.filter(([key]) => p.features[key] && m[key]).map(([, i18nKey]) => i18nKey);
}

// Pure: plans that run on the user's own machine (Ollama) need no API key.
function setupProfileNeedsKey(p) {
    return !(p && p.no_api_key);
}

// Pure: the chosen local model. The select (filled from detection) wins over the manual field.
function setupOllamaModelFrom(selectValue, manualValue) {
    return String(selectValue || '').trim() || String(manualValue || '').trim();
}

// Pure: the usable model entries of a detection payload; tolerates any shape.
function setupOllamaModels(detection) {
    const list = detection && Array.isArray(detection.models) ? detection.models : [];
    return list.filter(m => m && typeof m.name === 'string' && m.name !== '');
}

// Pure: Ollama is preselected only when it runs with a model, the user has not picked a plan yet
// and the wizard is still on the plan page.
function setupShouldPreselectOllama(detection, userChosen, stepIndex) {
    return !!detection && detection.reachable === true && setupOllamaModels(detection).length > 0 &&
        !userChosen && stepIndex === 0;
}

// Pure: status line of the Ollama plan card.
function setupOllamaCardStatus(detection) {
    if (detection === null || detection === undefined) return { cls: '', key: 'setup.ollama_status_checking' };
    const count = setupOllamaModels(detection).length;
    if (detection.reachable === true && count > 0) return { cls: 'is-ok', key: 'setup.ollama_status_detected', count };
    return { cls: 'is-missing', key: 'setup.ollama_status_missing' };
}

// Pure: quick plans without an embeddings model of their own use the local embedder, like the custom flow.
function setupQuickEmbeddingsPatch(p, models) {
    if (p && p.features && p.features.embeddings && models && models.embeddings) return { provider: 'embeddings' };
    return { provider: 'local-granite', local: { backend: 'auto', context_size: 2048, batch_size: 2048 }, multimodal: false };
}

// Badges list only what a plan really enables; the custom plan gets one neutral badge instead.
function getFeatureBadges(profile) {
    if (!profile) return '';
    if (profile.id === 'custom') {
        return `<span class="feature-badge feature-badge-neutral">${escapeHtml(t('setup.feature_custom_all'))}</span>`;
    }
    const icons = {
        'setup.feature_vision': '👁',
        'setup.feature_speech_input': '🎙',
        'setup.feature_memory': '🧠',
        'setup.feature_images': '🎨',
        'setup.feature_video': '🎬',
        'setup.feature_music': '🎵',
    };
    return setupQuickProfileFeatureKeys(profile)
        .map(key => `<span class="feature-badge">${icons[key] || ''} ${escapeHtml(t(key))}</span>`).join('');
}

function tProfile(profileId, field, fallback) {
    const key = 'setup.profile_' + profileId + '_' + field;
    const val = t(key);
    return (val !== key) ? val : (fallback || '');
}

function renderProfileCards(list) {
    const grid = document.getElementById('profile-grid');
    if (!grid) return;
    if (!list || list.length === 0) {
        grid.innerHTML = `<p class="profile-state-message profile-empty-state">${escapeHtml(t('setup.plan_no_profiles'))}</p>`;
        return;
    }
    grid.innerHTML = list.map(p => {
        const isCustom = p.id === 'custom';
        const name = tProfile(p.id, 'name', p.name);
        const desc = tProfile(p.id, 'description', p.description || '');
        const pricing = tProfile(p.id, 'pricing', p.pricing_label || '');
        const badges = getFeatureBadges(p);
        const status = p.no_api_key ? setupOllamaCardStatus(setupOllamaDetection) : null;
        return `
        <div class="profile-card${isCustom ? ' is-custom' : ''}"
             id="profile-card-${escapeAttr(p.id)}"
             role="button"
             tabindex="0"
             aria-pressed="false"
             onclick="selectProfile('${escapeAttr(p.id)}', true)"
             onkeydown="if(event.key==='Enter'||event.key===' '){event.preventDefault();selectProfile('${escapeAttr(p.id)}', true)}">
            <div class="profile-check">✓</div>
            ${p.recommended ? '<div class="profile-recommended-bubble">' + escapeHtml(t('setup.profile_recommended_badge')) + '</div>' : ''}
            <div class="profile-card-icon">${escapeHtml(p.icon || (isCustom ? '⚙️' : '🤖'))}</div>
            <div class="profile-card-name">${escapeHtml(name)}</div>
            <div class="profile-card-description">${escapeHtml(desc)}</div>
            ${badges ? `<div class="profile-features">${badges}</div>` : ''}
            ${pricing ? `<div class="profile-card-pricing">${escapeHtml(pricing)}</div>` : ''}
            ${status ? `<div class="profile-card-status ${status.cls}" id="profile-status-${escapeAttr(p.id)}">${escapeHtml(t(status.key, { count: status.count }))}</div>` : ''}
        </div>`;
    }).join('');
}

function isMiniMaxQuickProfile(profile) {
    return !!profile && profile.id === 'minimax_coding';
}

function getQuickProfileRuntimeConfig(profile) {
    const runtime = {
        providerType: (profile && profile.provider_type) || 'openai',
        baseUrl: (profile && profile.base_url) || '',
        mainModel: (profile && profile.main_model) || '',
        keyPlaceholder: (profile && profile.key_placeholder) || 'sk-...',
        keyUrl: (profile && profile.key_url) || '',
    };
    if (!setupProfileNeedsKey(profile)) {
        runtime.baseUrl = setupOllamaBaseURL;
        runtime.mainModel = setupQuickOllamaModel();
        return runtime;
    }
    if (!isMiniMaxQuickProfile(profile)) return runtime;

    const region = (document.getElementById('quick-minimax-region') || {}).value || 'international';
    const useHighspeed = !!((document.getElementById('quick-minimax-highspeed') || {}).checked);
    runtime.baseUrl = region === 'china'
        ? (profile.alt_base_url || 'https://api.minimaxi.com/v1')
        : (profile.base_url || 'https://api.minimax.io/v1');
    runtime.keyUrl = region === 'china'
        ? (profile.alt_key_url || profile.key_url || 'https://platform.minimaxi.com')
        : (profile.key_url || 'https://platform.minimax.io');
    runtime.mainModel = useHighspeed
        ? (profile.highspeed_model || profile.main_model || 'MiniMax-M2.7-highspeed')
        : (profile.main_model || 'MiniMax-M2.7');
    runtime.keyPlaceholder = profile.key_placeholder || 'sk-...';
    return runtime;
}

function resolveQuickProfileModel(profile, model) {
    if (!isMiniMaxQuickProfile(profile)) return model || '';
    const useHighspeed = !!((document.getElementById('quick-minimax-highspeed') || {}).checked);
    if (useHighspeed && model === (profile.main_model || '')) {
        return profile.highspeed_model || model || '';
    }
    return model || '';
}

function getQuickProfileSubsystemRuntime(profile, subsystem) {
    const runtime = getQuickProfileRuntimeConfig(profile);
    const config = {
        providerType: runtime.providerType,
        baseUrl: runtime.baseUrl,
        model: resolveQuickProfileModel(profile, (subsystem && subsystem.model) || ''),
    };
    if (!subsystem) return config;
    if (subsystem.provider_type) {
        config.providerType = subsystem.provider_type;
    }
    const region = (document.getElementById('quick-minimax-region') || {}).value || 'international';
    if (region === 'china' && subsystem.alt_base_url) {
        config.baseUrl = subsystem.alt_base_url;
    } else if (subsystem.base_url) {
        config.baseUrl = subsystem.base_url;
    }
    return config;
}

// The model chosen for the local (Ollama) plan, read from the quick step.
function setupQuickOllamaModel() {
    return setupOllamaModelFrom(
        (document.getElementById('quick-ollama-model') || {}).value,
        (document.getElementById('quick-ollama-model-manual') || {}).value);
}

// The id of the visible model control of the local plan (for error marking and focus).
function setupQuickOllamaInputId() {
    const select = document.getElementById('quick-ollama-model');
    return select && !select.classList.contains('is-hidden') ? 'quick-ollama-model' : 'quick-ollama-model-manual';
}

function setupOllamaDisplayURL() {
    return setupOllamaBaseURL.replace(/\/v1\/?$/, '');
}

// Status, model list and hints of the local plan card on the quick step.
function renderQuickOllamaCard() {
    const detection = setupOllamaDetection;
    const models = setupOllamaModels(detection);
    const reachable = !!detection && detection.reachable === true;
    const status = document.getElementById('quick-ollama-status');
    const select = document.getElementById('quick-ollama-model');
    const manual = document.getElementById('quick-ollama-model-manual');
    const hint = document.getElementById('quick-ollama-hint');
    const recheck = document.getElementById('btn-quick-ollama-recheck');
    const url = setupOllamaDisplayURL();

    if (status) {
        status.className = 'setup-ollama-status' + (detection === null ? '' : reachable ? ' is-ok' : ' is-missing');
        status.textContent = detection === null ? t('setup.ollama_status_checking')
            : reachable ? t('setup.ollama_found_at', { url }) : t('setup.ollama_status_missing');
    }
    if (select && manual) {
        if (models.length > 0) {
            const previous = select.value;
            select.replaceChildren(...models.map(model =>
                new Option(model.name + (model.parameter_size ? ' \u00b7 ' + model.parameter_size : ''), model.name)));
            const wanted = (selectedProfile && selectedProfile.main_model) || '';
            const preferred = models.find(model => model.name === previous) ||
                models.find(model => wanted && model.name.indexOf(wanted) === 0) || models[0];
            select.value = preferred.name;
        }
        setupSetHidden(select, models.length === 0);
        setupSetHidden(manual, models.length > 0);
    }
    if (hint) {
        const parts = [];
        if (detection !== null && models.length === 0) {
            parts.push(reachable ? t('setup.ollama_no_models_hint') : t('setup.ollama_missing_hint', { url }));
            if (!reachable && setupIsDocker) parts.push(t('setup.ollama_docker_hint'));
        }
        hint.textContent = parts.join(' ');
    }
    if (recheck) recheck.disabled = setupOllamaChecking;
}

// Detection finished: update the plan card in place (no re-render, keeps keyboard focus) and the quick step.
function refreshSetupOllamaUI() {
    const card = document.getElementById('profile-status-ollama_local');
    if (card) {
        const status = setupOllamaCardStatus(setupOllamaDetection);
        card.className = 'profile-card-status ' + status.cls;
        card.textContent = t(status.key, { count: status.count });
    }
    if (selectedProfile && !setupProfileNeedsKey(selectedProfile)) renderQuickOllamaCard();
}

function maybePreselectOllama() {
    if (!profiles.some(p => p.id === 'ollama_local')) return;
    if (setupShouldPreselectOllama(setupOllamaDetection, setupProfileUserChosen, currentStepIndex)) selectProfile('ollama_local');
}

async function detectSetupOllama() {
    if (!csrfToken || setupOllamaChecking) return;
    setupOllamaChecking = true;
    setupOllamaDetection = null;
    refreshSetupOllamaUI();
    try {
        const resp = await fetch('/api/setup/ollama', { headers: setupRequestHeaders() });
        const data = resp.ok ? await resp.json().catch(() => null) : null;
        setupOllamaDetection = data && typeof data === 'object'
            ? { reachable: data.reachable === true, models: setupOllamaModels(data) }
            : { reachable: false, models: [] };
    } catch (_) {
        setupOllamaDetection = { reachable: false, models: [] };
    }
    setupOllamaChecking = false;
    refreshSetupOllamaUI();
    maybePreselectOllama();
}

function renderQuickSummary() {
    const summary = document.getElementById('plan-quick-summary');
    if (!summary || !selectedProfile) return;
    const runtime = getQuickProfileRuntimeConfig(selectedProfile);
    const chips = setupQuickProfileFeatureKeys(selectedProfile)
        .map(k => `<span class="plan-quick-chip">${escapeHtml(t(k))}</span>`).join('');
    summary.innerHTML = `
        ${runtime.mainModel ? `<p class="plan-quick-summary-model">${escapeHtml(t('setup.plan_quick_summary_model', { model: runtime.mainModel }))}</p>` : ''}
        ${chips ? `<div class="plan-quick-chips">${chips}</div>` : ''}
        <p class="plan-quick-summary-later">${escapeHtml(t('setup.plan_quick_summary_later'))}</p>`;
}

document.addEventListener('change', (e) => {
    if (e.target && e.target.id === 'quick-ollama-model') renderQuickSummary();
});
document.addEventListener('input', (e) => {
    if (e.target && e.target.id === 'quick-ollama-model-manual') renderQuickSummary();
});

function updateQuickProfileUI() {
    const minimaxOptions = document.getElementById('quick-minimax-options');
    if (minimaxOptions) {
        setupSetHidden(minimaxOptions, !isMiniMaxQuickProfile(selectedProfile));
    }
    updateEmbeddingSetupWarnings();
    if (!selectedProfile || selectedProfile.id === 'custom') return;

    const needsKey = setupProfileNeedsKey(selectedProfile);
    setupSetHidden(document.getElementById('quick-api-key-card'), !needsKey);
    setupSetHidden(document.getElementById('quick-ollama-card'), needsKey);
    setupSetHidden(document.getElementById('quick-description'), !needsKey);
    setupSetHidden(document.getElementById('quick-description-local'), needsKey);
    if (!needsKey) renderQuickOllamaCard();

    const runtime = getQuickProfileRuntimeConfig(selectedProfile);

    const header = document.getElementById('plan-quick-header');
    if (header) {
        const name = tProfile(selectedProfile.id, 'name', selectedProfile.name);
        const desc = tProfile(selectedProfile.id, 'description', selectedProfile.description || '');
        header.innerHTML = `
            <div class="plan-quick-icon">${escapeHtml(selectedProfile.icon || '🤖')}</div>
            <div>
                <div class="plan-quick-name">${escapeHtml(name)}</div>
                <div class="plan-quick-subtitle">${escapeHtml(desc)}</div>
            </div>`;
    }

    renderQuickSummary();

    const hint = document.getElementById('quick-api-key-hint');
    const link = document.getElementById('quick-key-link');
    if (hint && link && runtime.keyUrl) {
        link.href = runtime.keyUrl;
        const domain = runtime.keyUrl.replace(/^https?:\/\//, '').split('/')[0];
        link.textContent = domain;
        setupSetHidden(hint, false);
    } else if (hint) {
        setupSetHidden(hint, true);
    }

    const apiKeyInput = document.getElementById('quick-api-key');
    if (apiKeyInput) {
        apiKeyInput.placeholder = runtime.keyPlaceholder;
    }

    if (isMiniMaxQuickProfile(selectedProfile)) {
        const regionHint = document.getElementById('quick-minimax-region-hint');
        if (regionHint) {
            regionHint.innerHTML = t('setup.minimax_region_hint', {
                international_url: selectedProfile.base_url || 'https://api.minimax.io/v1',
                china_url: selectedProfile.alt_base_url || 'https://api.minimaxi.com/v1',
            });
        }
        const runtimeHint = document.getElementById('quick-minimax-runtime-hint');
        if (runtimeHint) {
            runtimeHint.innerHTML = t('setup.minimax_runtime_hint', {
                model: runtime.mainModel,
                base_url: runtime.baseUrl,
            });
        }
    }
}

function onQuickMiniMaxOptionsChange() {
    updateQuickProfileUI();
}

function selectProfile(profileId, byUser = false) {
    if (byUser) setupProfileUserChosen = true;
    selectedProfile = profiles.find(p => p.id === profileId) || null;
    // Update card selection state and ARIA pressed state
    document.querySelectorAll('.profile-card').forEach(card => {
        const isSel = card.id === `profile-card-${profileId}`;
        card.classList.toggle('selected', isSel);
        card.setAttribute('aria-pressed', isSel ? 'true' : 'false');
    });
    if (!selectedProfile) return;
    // On the plan page the step indicator previews the flow the choice leads to.
    if (currentStepIndex === 0) {
        isQuickFlow = selectedProfile.id !== 'custom';
        renderStepIndicator();
    }
    // Update Next button state
    updateNextButtonState();
    updateQuickProfileUI();
}

// ── Connection Test Result ───────────────────
// Shared by the quick and custom flow: shows why the API key check failed.
// keyless: no API key was sent (local Ollama), so success must not claim one was accepted.
function renderSetupTestResult(el, resp, data, model, keyless) {
    data = data || {};
    const fail = (text) => {
        el.textContent = '✕ ' + text;
        el.className = 'field-hint test-result-fail';
    };
    if (data.ok) {
        let text = '✓ ' + t(keyless ? 'setup.test_connection_ok' : 'setup.test_key_ok');
        if (data.model_checked) text += ' · ' + t('setup.test_model_ok');
        if (typeof data.credit_remaining === 'number') {
            text += ' · ' + t('setup.test_credit_remaining', { amount: '$' + data.credit_remaining.toFixed(2) });
        } else if (data.free_tier === true) {
            text += ' · ' + t('setup.test_free_tier');
        }
        el.textContent = text;
        el.className = 'field-hint test-result-ok';
        return;
    }
    const code = data.code || '';
    switch (code) {
        case 'setup_bootstrap_token_required':
            openSetupOwnerGate(false, true);
            fail(data.error || t('setup.step0_test_failed'));
            return;
        case 'metadata_probe_unsupported':
        case 'metadata_probe_unavailable':
            el.textContent = t('setup.step0_test_unsupported');
            el.className = 'field-hint test-result-fail';
            return;
        case 'key_invalid': fail(t('setup.test_key_invalid')); return;
        case 'model_not_found': fail(t('setup.test_model_not_found', { model: model || '' })); return;
        case 'rate_limited': fail(t('setup.test_rate_limited')); return;
        case 'provider_unreachable': fail(t('setup.test_provider_unreachable')); return;
        case 'provider_error': fail(t('setup.test_provider_error', { status: data.http_status || (resp && resp.status) || '' })); return;
        case 'url_not_allowed': fail(t('setup.test_url_not_allowed')); return;
        case 'model_required': fail(t('setup.test_model_required')); return;
        case 'provider_test_busy': fail(t('setup.test_busy')); return;
        case 'csrf_invalid': fail(t('setup.test_session_expired')); return;
        case 'provider_incomplete': fail(t('setup.test_provider_incomplete')); return;
        default:
            fail(t('setup.step0_test_failed') + (code ? ' (' + code + ')' : ''));
    }
}

// ── Quick Connection Test ────────────────────
async function testQuickConnection() {
    if (!selectedProfile) return;
    const btn = document.getElementById('btn-quick-test');
    const result = document.getElementById('quick-test-result');
    if (!btn || btn.disabled) return;

    const needsKey = setupProfileNeedsKey(selectedProfile);
    const apiKey = needsKey ? document.getElementById('quick-api-key').value.trim() : '';
    if (needsKey && !apiKey) {
        result.textContent = t('setup.step0_api_key_error');
        result.className = 'field-hint test-result-fail';
        setupSetHidden(result, false);
        return;
    }
    if (!needsKey && !setupQuickOllamaModel()) {
        result.textContent = t('setup.ollama_model_error');
        result.className = 'field-hint test-result-fail';
        setupSetHidden(result, false);
        return;
    }

    btn.disabled = true;
    result.textContent = t('setup.step0_test_testing');
    result.className = 'field-hint';
    setupSetHidden(result, false);

    try {
        const runtime = getQuickProfileRuntimeConfig(selectedProfile);
        const resp = await fetch('/api/setup/test', {
            method: 'POST',
            headers: setupRequestHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify({
                provider_type: runtime.providerType,
                base_url: runtime.baseUrl,
                api_key: apiKey,
                model: runtime.mainModel,
            }),
        });
        const data = await resp.json().catch(() => ({}));
        renderSetupTestResult(result, resp, data, runtime.mainModel, !setupProfileNeedsKey(selectedProfile));
    } catch (err) {
        result.textContent = '✕ ' + err.message;
        result.className = 'field-hint test-result-fail';
    } finally {
        btn.disabled = false;
    }
}

// ── Language Change ──────────────────────────
// Sync the three language selectors and apply the chosen language.
// Single source of truth for cross-flow language mirror.
function syncLanguage(value) {
    if (!value) return;
    ['plan-language', 'system-language', 'quick-language'].forEach(id => {
        const el = document.getElementById(id);
        if (el) {
            el.value = value;
        } else if (typeof console !== 'undefined' && console.warn) {
            console.warn('syncLanguage: missing selector #' + id + ' (i18n mirror incomplete)');
        }
    });
    fetchAndApplyLang(value);
}

function onQuickLanguageChange() {
    syncLanguage(document.getElementById('quick-language')?.value);
}

function onPlanLanguageChange() {
    syncLanguage(document.getElementById('plan-language')?.value);
}

// ── Quick Flow Validation ────────────────────
function validateQuickStep(skip = false) {
    let valid = true;

    if (setupPasswordRequired === true) {
        const pw = document.getElementById('quick-admin-password').value;
        if (pw.length < 8 || !pw.trim()) {
            showFieldError('quick-admin-password', 'err-quick-admin-password');
            valid = false;
        } else {
            clearFieldError('quick-admin-password', 'err-quick-admin-password');
        }
        const pw2 = document.getElementById('quick-admin-password-confirm').value;
        if (pw.length >= 8 && pw2 !== pw) {
            showFieldError('quick-admin-password-confirm', 'err-quick-admin-password-confirm');
            valid = false;
        } else {
            clearFieldError('quick-admin-password-confirm', 'err-quick-admin-password-confirm');
        }
    }

    if (!skip && selectedProfile && selectedProfile.id !== 'custom') {
        if (setupProfileNeedsKey(selectedProfile)) {
            const apiKey = document.getElementById('quick-api-key').value.trim();
            if (!apiKey) {
                showFieldError('quick-api-key', 'err-quick-api-key');
                valid = false;
            } else {
                clearFieldError('quick-api-key', 'err-quick-api-key');
            }
        } else if (!setupQuickOllamaModel()) {
            showFieldError(setupQuickOllamaInputId(), 'err-quick-ollama-model');
            valid = false;
        } else {
            clearFieldError(setupQuickOllamaInputId(), 'err-quick-ollama-model');
        }
    }

    return valid;
}

// ── Quick Flow Config Patch Builder ──────────
function buildQuickConfigPatch() {
    if (!selectedProfile) return buildConfigPatch();

    const p = selectedProfile;
    const runtime = getQuickProfileRuntimeConfig(p);
    const apiKey = setupProfileNeedsKey(p) ? (document.getElementById('quick-api-key').value || '').trim() : '';
    const adminPassword = (document.getElementById('quick-admin-password').value || '');
    const quickLang = document.getElementById('quick-language');
    const mainLang = document.getElementById('system-language');
    const lang = (quickLang || mainLang || {}).value || 'de';

    // Build provider list — main + one entry per enabled subsystem
    const makeProvider = (id, label, model, extra) => ({
        id,
        type: runtime.providerType,
        name: `${p.name} ${label}`.trim(),
        base_url: runtime.baseUrl,
        api_key: apiKey,
        model,
        native_function_calling: p.native_function_calling !== false,
        ...extra,
    });

    const providers = [makeProvider('main', '', runtime.mainModel || '', {})];
    const m = p.models || {};
    const makeSubsystemProvider = (id, label, subsystem, extra) => {
        const subsystemRuntime = getQuickProfileSubsystemRuntime(p, subsystem);
        return {
            id,
            type: subsystemRuntime.providerType,
            name: `${p.name} ${label}`.trim(),
            base_url: subsystemRuntime.baseUrl,
            api_key: apiKey,
            model: subsystemRuntime.model,
            native_function_calling: p.native_function_calling !== false,
            ...extra,
        };
    };

    if (p.features && p.features.vision    && m.vision)           providers.push(makeSubsystemProvider('vision',     'Vision',     m.vision,           {}));
    if (p.features && p.features.whisper   && m.whisper)          providers.push(makeSubsystemProvider('whisper',    'Whisper',    m.whisper,          {}));
    if (p.features && p.features.embeddings && m.embeddings)      providers.push(makeSubsystemProvider('embeddings', 'Embeddings', m.embeddings,       { native_function_calling: false }));
    if (p.features && p.features.helper    && m.helper)           providers.push(makeSubsystemProvider('helper',     'Helper',     m.helper,           {}));
    if (p.features && p.features.image_generation && m.image_generation) providers.push(makeSubsystemProvider('image_gen', 'Image Gen', m.image_generation, {}));
    if (p.features && p.features.music_generation && m.music_generation) providers.push(makeSubsystemProvider('music_gen', 'Music Gen',  m.music_generation, {}));
    if (p.features && p.features.video_generation && m.video_generation) providers.push(makeSubsystemProvider('video_gen', 'Video Gen', m.video_generation, {}));

    // Read trust level from the radio group (level 2 is checked by default in setup.html)
    const trustRadio = document.querySelector('input[name="trust-level"]:checked');
    const trustLevel = trustRadio ? parseInt(trustRadio.value, 10) : 2;

    const patch = {
        _setup_profile_id: p.id,
        server: {
            ui_language: lang,
        },
        auth: {
            enabled: true,
            ...(adminPassword ? { admin_password: adminPassword } : {}),
        },
        providers,
        agent: {
            system_language: setupAgentLanguage(lang, ''),
        },
        llm: {
            provider: 'main',
            use_native_functions: p.native_function_calling !== false,
            helper_enabled: !!(p.features && p.features.helper && m.helper),
            helper_provider: (p.features && p.features.helper && m.helper) ? 'helper' : '',
            helper_model: (p.features && p.features.helper && m.helper) ? getQuickProfileSubsystemRuntime(p, m.helper).model : '',
        },
        embeddings: setupQuickEmbeddingsPatch(p, m),
        vision: {
            provider: (p.features && p.features.vision && m.vision) ? 'vision' : '',
        },
        whisper: {
            provider: (p.features && p.features.whisper && m.whisper) ? 'whisper' : '',
            mode: (m.whisper && m.whisper.mode) ? m.whisper.mode : 'multimodal',
        },
        image_generation: {
            enabled: !!(p.features && p.features.image_generation),
            provider: (p.features && p.features.image_generation && m.image_generation) ? 'image_gen' : '',
        },
        music_generation: {
            enabled: !!(p.features && p.features.music_generation),
            provider: (p.features && p.features.music_generation && m.music_generation) ? 'music_gen' : '',
        },
        video_generation: {
            enabled: !!(p.features && p.features.video_generation),
            provider: (p.features && p.features.video_generation && m.video_generation) ? 'video_gen' : '',
            default_duration_seconds: 6,
            default_resolution: '768P',
        },
    };

    // Merge trust level permissions
    deepMergePatch(patch, buildTrustLevelPatch(trustLevel));

    // Keep local CPU speech as the installation default; retain profile credentials
    // so the user can select that provider later in Speech Output settings.
    patch.tts = { provider: 'phonon', language: 'auto' };
    if (p.tts && p.tts.provider) {
        const ttsProvider = p.tts.provider;
        patch.tts[ttsProvider] = { api_key: apiKey };
        if (p.tts.model_id) patch.tts[ttsProvider].model_id = p.tts.model_id;
        if (p.tts.voice_id) patch.tts[ttsProvider].voice_id = p.tts.voice_id;
        if (p.tts.speed)    patch.tts[ttsProvider].speed    = p.tts.speed;
    }

    // Merge any extra config_patch defined in the profile YAML
    if (p.config_patch) deepMergePatch(patch, p.config_patch);

    applySetupLocalLLMPatch(patch);
    return patch;
}
// Tooltip label from a "Label:" detail key, without the trailing colon.
function orTooltipLabel(key) { return String(t(key)).replace(/:\s*$/, ''); }
function escapeAttr(s) { return String(s).replace(/"/g, '&quot;').replace(/</g, '&lt;'); }
function escapeHtml(s) { return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;'); }
function setupSetHidden(el, hidden) {
    if (!el) return;
    el.classList.toggle('is-hidden', hidden);
}

// ── Password fields: strength, match hint, show/hide, twin sync ──
// Pure: '' (empty), 'short' (< 8), 'fair' (8-11), 'good' (12-15) or 'strong' (>= 16).
function setupPasswordStrength(pw) {
    const len = String(pw || '').length;
    if (len === 0) return '';
    if (len < 8) return 'short';
    if (len < 12) return 'fair';
    if (len < 16) return 'good';
    return 'strong';
}

// Pure: '' while the confirmation is empty, then 'match' or 'mismatch'.
function setupPasswordMatchState(pw, confirm) {
    if (!confirm) return '';
    return pw === confirm ? 'match' : 'mismatch';
}

// Pure: the quick-flow field mirroring a custom-flow password field and vice versa.
function setupPasswordTwinId(id) {
    const pairs = {
        'admin-password': 'quick-admin-password',
        'admin-password-confirm': 'quick-admin-password-confirm',
        'quick-admin-password': 'admin-password',
        'quick-admin-password-confirm': 'admin-password-confirm',
    };
    return pairs[id] || '';
}

function updateSetupPasswordIndicators(id) {
    const confirmId = /-confirm$/.test(id) ? id : id + '-confirm';
    const pwId = confirmId.replace(/-confirm$/, '');
    const pwInput = document.getElementById(pwId);
    const confirmInput = document.getElementById(confirmId);
    const strengthEl = document.getElementById(pwId + '-strength');
    const matchEl = document.getElementById(confirmId + '-match');
    if (strengthEl && pwInput) {
        const state = setupPasswordStrength(pwInput.value);
        strengthEl.textContent = state ? t('setup.password_strength_' + state) : '';
        strengthEl.classList.toggle('is-weak', state === 'short');
        strengthEl.classList.toggle('is-ok', state === 'fair');
        strengthEl.classList.toggle('is-strong', state === 'good' || state === 'strong');
    }
    if (matchEl && pwInput && confirmInput) {
        const state = setupPasswordMatchState(pwInput.value, confirmInput.value);
        matchEl.textContent = state ? t('setup.password_' + state) : '';
        matchEl.classList.toggle('is-match', state === 'match');
        matchEl.classList.toggle('is-mismatch', state === 'mismatch');
    }
}

// Autofill and password managers can fill one flow's fields without input events.
function syncSetupPasswordTwins() {
    ['admin-password', 'admin-password-confirm'].forEach(id => {
        const a = document.getElementById(id);
        const b = document.getElementById(setupPasswordTwinId(id));
        if (!a || !b) return;
        if (a.value && !b.value) b.value = a.value;
        else if (b.value && !a.value) a.value = b.value;
    });
    ['admin-password', 'admin-password-confirm', 'quick-admin-password', 'quick-admin-password-confirm']
        .forEach(updateSetupPasswordIndicators);
}

/* ── OpenRouter Model Browser (reusable modal) ── */
let _orModelsCache = null;
let _orModelsCacheTime = 0;
const OR_CACHE_TTL = 5 * 60 * 1000;

async function openOpenRouterBrowser(onSelect) {

    const previouslyFocused = document.activeElement;
    const existing = document.getElementById('or-browser-overlay');
    if (existing) existing.remove();

    const overlay = document.createElement('div');
    overlay.id = 'or-browser-overlay';
    overlay.className = 'or-browser-overlay';
    const closeBrowser = () => {
        overlay.remove();
        if (previouslyFocused && typeof previouslyFocused.focus === 'function') {
            previouslyFocused.focus();
        }
    };
    overlay.onclick = (e) => { if (e.target === overlay) closeBrowser(); };

    overlay.innerHTML = `
            <div class="or-browser-modal" role="dialog" aria-modal="true" aria-labelledby="or-browser-title" tabindex="-1">
                <div class="or-browser-header">
                    <div class="or-browser-title-row">
                        <div class="or-browser-title" id="or-browser-title">${t('setup.or_browser_title')}</div>
                        <button type="button" id="or-browser-close" class="or-browser-close-btn" aria-label="${escapeAttr(t('common.close'))}">✕</button>
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
    const modal = overlay.querySelector('.or-browser-modal');
    const closeButton = document.getElementById('or-browser-close');

    modal.onclick = (event) => event.stopPropagation();
    closeButton.onclick = closeBrowser;
    overlay.onkeydown = (event) => {
        if (event.key === 'Escape') {
            event.preventDefault();
            closeBrowser();
            return;
        }
        if (event.key !== 'Tab') return;

        const focusable = Array.from(overlay.querySelectorAll(
            'button:not([disabled]), input:not([disabled]), [href], [tabindex]:not([tabindex="-1"])'
        )).filter(element => element.getAttribute('aria-hidden') !== 'true');
        if (focusable.length === 0) {
            event.preventDefault();
            modal.focus();
            return;
        }

        if (event.shiftKey && document.activeElement === focusable[0]) {
            event.preventDefault();
            focusable[focusable.length - 1].focus();
        } else if (!event.shiftKey && document.activeElement === focusable[focusable.length - 1]) {
            event.preventDefault();
            focusable[0].focus();
        }
    };
    searchInput.focus();

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
        loadingDiv.innerHTML = '<span class="or-loading-error">❌ ' + escapeHtml(t('setup.or_browser_connection_error')) + escapeHtml(e && e.message ? e.message : String(e)) + '</span>';
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
                            ${free ? '<span class="or-free-badge">' + t('setup.or_browser_free_badge') + '</span>' : '<span class="or-meta-text" title="' + escapeAttr(orTooltipLabel('setup.or_browser_detail_input') + ' / ' + orTooltipLabel('setup.or_browser_detail_output')) + '">' + promptCost + ' · ' + completionCost + '</span>'}
                            <span class="or-meta-text" title="${escapeAttr(orTooltipLabel('setup.or_browser_detail_context'))}">${formatContext(m.context_length)}</span>
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
            closeBrowser();
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
}

// ── Vision / STT defaults per main provider ──
// Pure: which vision and speech-to-text setup fits the chosen main provider.
// An empty provider means the subsystem stays disabled.
const SETUP_OPENROUTER_SUBSYSTEM_MODEL = 'google/gemini-3.1-flash-lite';

function setupSubsystemDefaults(mainType, mainModel) {
    const model = mainModel || '';
    switch (mainType) {
        case 'openrouter':
            return {
                vision: { provider: 'openrouter', model: SETUP_OPENROUTER_SUBSYSTEM_MODEL },
                whisper: { provider: 'openrouter', mode: 'multimodal', model: SETUP_OPENROUTER_SUBSYSTEM_MODEL },
            };
        case 'openai':
            return {
                vision: { provider: 'openai', model },
                whisper: { provider: 'openai', mode: 'whisper', model: 'whisper-1' },
            };
        case 'google':
            return {
                vision: { provider: 'main', model },
                whisper: { provider: 'main', mode: 'multimodal', model },
            };
        case 'ollama':
            return {
                vision: { provider: '', model: '' },
                whisper: { provider: '', mode: 'multimodal', model: '' },
            };
        default: // anthropic, agnes, custom: vision through the main provider, no STT
            return {
                vision: { provider: 'main', model },
                whisper: { provider: '', mode: 'multimodal', model: '' },
            };
    }
}

// Pure: the optional steps the header "Skip" button may jump over.
function setupStepSkippable(stepId) {
    return stepId === 'step-1' || stepId === 'step-2';
}

// The user's own choice wins: once a vision/STT control was touched the
// defaults no longer overwrite it. Programmatic value changes fire no events.
let setupVisionTouched = false;
let setupWhisperTouched = false;
let setupNativeFunctionsTouched = false;

function applySetupSubsystemDefaults() {
    const mainType = (document.getElementById('llm-provider') || {}).value || '';
    const mainModel = ((document.getElementById('llm-model') || {}).value || '').trim();
    const defaults = setupSubsystemDefaults(mainType, mainModel);
    const set = (id, value) => {
        const el = document.getElementById(id);
        if (el) el.value = value;
    };
    if (!setupVisionTouched) {
        set('vision-provider', defaults.vision.provider);
        set('vision-model', defaults.vision.model);
    }
    if (!setupWhisperTouched) {
        set('whisper-provider', defaults.whisper.provider);
        set('whisper-mode', defaults.whisper.mode);
        set('whisper-model', defaults.whisper.model);
    }
}

['change', 'input'].forEach(type => document.addEventListener(type, (e) => {
    const id = e.target && e.target.id;
    if (id === 'vision-provider' || id === 'vision-model') setupVisionTouched = true;
    else if (id === 'whisper-provider' || id === 'whisper-model' || id === 'whisper-mode') setupWhisperTouched = true;
    else if (id === 'native-functions') setupNativeFunctionsTouched = true;
    else if (type === 'input' && id === 'llm-model') applySetupSubsystemDefaults();
}));

// ── Provider Change Handler ──────────────────
function setupProviderDefaultModels() {
    return Object.values(providerConfig).map(cfg => cfg.defaultModel).filter(Boolean);
}

function onProviderChange() {
    const provider = document.getElementById('llm-provider').value;
    const cfg = providerConfig[provider] || providerConfig.custom;

    document.getElementById('llm-base-url').value = cfg.baseUrl;
    document.getElementById('llm-base-url').placeholder = cfg.baseUrl || 'https://...';
    document.getElementById('llm-api-key').placeholder = cfg.placeholder;
    document.getElementById('llm-model').placeholder = cfg.defaultModel;
    const providerLink = document.getElementById('provider-link');
    if (providerLink) {
        providerLink.href = cfg.link ? 'https://' + cfg.link : '';
        providerLink.textContent = cfg.link ? cfg.link.split('/')[0] : '';
    }
    setupSetHidden(document.getElementById('hint-api-key'), !cfg.link || !cfg.needsKey);
    document.getElementById('hint-provider').textContent = t(cfg.hintKey || 'setup.step0_provider_hint');

    // Pre-fill the model unless the user typed one: a value that is another
    // provider's default was filled in by us and would not work here.
    // Agnes uses one documented setup model for both agents.
    const modelInput = document.getElementById('llm-model');
    const helperInput = document.getElementById('helper-model');
    const autoFilledModel = (value) => !value || setupProviderDefaultModels().includes(value);
    const helperWasAutoFilled = autoFilledModel(helperInput.value.trim()) && helperInput.value.trim() !== 'agnes-2.0-flash';
    if (provider === 'agnes' && cfg.defaultModel) {
        modelInput.value = cfg.defaultModel;
    } else if (autoFilledModel(modelInput.value.trim())) {
        modelInput.value = cfg.defaultModel || '';
    }
    if (helperWasAutoFilled) helperInput.value = '';

    // Show/hide API key field for Ollama
    const apiKeyGroup = document.getElementById('group-api-key');
    if (!cfg.needsKey) {
        apiKeyGroup.classList.add('setup-dimmed');
        document.getElementById('llm-api-key').removeAttribute('required');
    } else {
        apiKeyGroup.classList.remove('setup-dimmed');
    }

    // The advanced section holds the base URL a custom provider needs.
    if (provider === 'custom') {
        const advanced = document.getElementById('step0-advanced');
        if (advanced) advanced.open = true;
    }
    const nativeFunctions = document.getElementById('native-functions');
    if (nativeFunctions && !setupNativeFunctionsTouched) nativeFunctions.checked = setupNativeFunctionsDefault(provider);

    // Show/hide OpenRouter browse button
    const orBrowseBtn = document.getElementById('or-browse-btn');
    if (orBrowseBtn) {
        setupSetHidden(orBrowseBtn, provider !== 'openrouter');
    }

    updateEmbeddingSetupWarnings();
    applySetupSubsystemDefaults();
    if (provider === 'agnes') {
        document.getElementById('helper-model').value = cfg.defaultModel;
        document.getElementById('helper-llm').checked = true;
        onHelperToggle();
    } else {
        if (document.getElementById('helper-model').value === 'agnes-2.0-flash') {
            document.getElementById('helper-model').value = '';
            document.getElementById('helper-llm').checked = false;
            onHelperToggle();
        }
        syncHelperModelSuggestion();
    }
}

function updateEmbeddingSetupWarnings() {
    const providerWarning = document.getElementById('provider-embedding-warning');
    const provider = (document.getElementById('llm-provider') || {}).value || '';
    if (providerWarning) {
        setupSetHidden(providerWarning, !provider || provider === 'openrouter');
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
    } else if (provider === 'openrouter' || provider === 'main') {
        modeEl.value = 'multimodal';
    }
}

// ── Embeddings Provider Change Handler ───────
function onEmbProviderChange() {
    const prov = document.getElementById('emb-provider').value;
    const apiKeyGroup = document.getElementById('emb-apikey-group');
    const modelGroup = document.getElementById('emb-model-group');
    if (apiKeyGroup) setupSetHidden(apiKeyGroup, prov !== 'internal');
    if (modelGroup) setupSetHidden(modelGroup, prov === 'local-granite' || prov === '');
}

// ── Test Connection ──────────────────────────
async function testConnection() {
    const btn = document.getElementById('btn-test-connection');
    const result = document.getElementById('test-connection-result');
    if (!btn || btn.disabled) return;

    const providerType = document.getElementById('llm-provider').value;
    const baseUrl = document.getElementById('llm-base-url').value.trim();
    const apiKey = document.getElementById('llm-api-key').value.trim();
    const model = document.getElementById('llm-model').value.trim();

    if (!model) {
        result.textContent = t('setup.step0_test_no_model');
        result.className = 'field-hint test-result-fail';
        setupSetHidden(result, false);
        return;
    }

    btn.disabled = true;
    result.textContent = t('setup.step0_test_testing');
    result.className = 'field-hint';
    setupSetHidden(result, false);

    try {
        const resp = await fetch('/api/setup/test', {
            method: 'POST',
            headers: setupRequestHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify({ provider_type: providerType, base_url: baseUrl, api_key: apiKey, model: model }),
        });
        const data = await resp.json().catch(() => ({}));
        renderSetupTestResult(result, resp, data, model, !apiKey);
    } catch (err) {
        result.textContent = '✕ ' + err.message;
        result.className = 'field-hint test-result-fail';
    } finally {
        btn.disabled = false;
    }
}

function fetchAndApplyLang(langValue) {
    document.documentElement.lang = langValue || 'en';
    const seq = ++langFetchSeq; // capture current sequence; stale responses are discarded
    fetch('/api/i18n?lang=' + encodeURIComponent(langValue))
        .then(r => r.ok ? r.json() : null)
        .then(json => {
            if (seq !== langFetchSeq) return; // a newer fetch started — discard this response
            if (json && json.data && typeof json.data === 'object') {
                I18N = json.data;
                applyI18N();
                updateNavLabels(); // applyI18N resets #btn-next to "Next" via data-i18n
                renderStepIndicator();
                // Re-render dynamically built content that used t() at creation time
                if (profiles && profiles.length > 0) {
                    renderProfileCards(profiles);
                    if (selectedProfile) selectProfile(selectedProfile.id);
                }
            }
        })
        .catch((err) => { console.warn('fetchAndApplyLang failed:', err); });
}

// ── Helper LLM Toggle ────────────────────────
function syncHelperModelSuggestion() {
    const helperModel = document.getElementById('helper-model');
    const mainModel = document.getElementById('llm-model');
    if (!helperModel || !mainModel) return;
    if (!helperModel.value.trim()) {
        helperModel.value = mainModel.value.trim();
    }
}

function onHelperToggle() {
    const fields = document.getElementById('helper-llm-fields');
    const checked = document.getElementById('helper-llm').checked;
    fields.classList.toggle('visible', checked);
    if (checked) syncHelperModelSuggestion();
}

function onSetupLocalLLMToggle() {
    const enabled = document.getElementById('setup-local-llm-enabled').checked;
    setupSetHidden(document.getElementById('setup-local-llm-fields'), !enabled);
}

let setupLocalLLMProbeFingerprint = '';
let setupLocalLLMAcknowledgementRequired = false;

function onSetupLocalLLMFamilyChange() {
    const ling = document.getElementById('setup-local-llm-family').value === 'ling';
    const spark = document.getElementById('setup-local-llm-family').value === 'spark';
    const model = document.getElementById('setup-local-llm-model');
    model.replaceChildren(...(spark ? [['q4_k_m', 'Q4_K_M · 2.42 GiB']] : ling ? [['q4_k_l', 'Q4_K_L · 4.75 GiB']] : [['q4_k_m', 'Q4_K_M · 2.59 GiB'], ['q8_0', 'Q8_0 · 4.29 GiB']])
        .map(([value, label]) => new Option(label, value)));
    const mtp = document.getElementById('setup-local-llm-mtp');
    mtp.value = 'off';
    mtp.disabled = ling || spark;
    document.getElementById('setup-local-llm-ling-quality').classList.toggle('is-hidden', !ling);
    document.getElementById('setup-local-llm-spark-quality').classList.toggle('is-hidden', !spark);
    document.getElementById('setup-local-llm-qwen-quality').classList.toggle('is-hidden', ling || spark);
    onSetupLocalLLMBackendChange();
}

document.addEventListener('change', event => {
    if (event.target.id === 'setup-local-llm-family') onSetupLocalLLMFamilyChange();
});

function onSetupLocalLLMBackendChange() {
    setupLocalLLMProbeFingerprint = '';
    setupLocalLLMAcknowledgementRequired = false;
    const acknowledgement = document.getElementById('setup-local-llm-ack');
    const wrapper = document.getElementById('setup-local-llm-ack-wrap');
    if (acknowledgement) acknowledgement.checked = false;
    if (wrapper) wrapper.classList.add('is-hidden');
    const target = document.getElementById('setup-local-llm-probe-result');
    if (target) target.textContent = '';
}

async function probeSetupLocalLLM() {
    const target = document.getElementById('setup-local-llm-probe-result');
    target.textContent = t('setup.local_llm_probing');
    try {
        const response = await fetch('/api/setup/local-llm/probe', {
            method: 'POST',
            headers: setupRequestHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify({ backend: document.getElementById('setup-local-llm-backend').value,
                model_family: document.getElementById('setup-local-llm-family').value }),
        });
        const data = await response.json();
        if (!response.ok) throw new Error(data.message || data.error || ('HTTP ' + response.status));
        const compatibilityKey = 'config.local_llm.compat_' + String(data.compatibility || 'unknown')
            .toLowerCase().replace(/[^a-z0-9_]/g, '_');
        const localizedCompatibility = t(compatibilityKey);
        target.textContent = t('setup.local_llm_probe_result')
            .replace('{compatibility}', localizedCompatibility === compatibilityKey
                ? (data.compatibility || 'unknown')
                : localizedCompatibility)
            .replace('{backend}', data.backend || '—');
        setupLocalLLMProbeFingerprint = data.hardware_fingerprint || '';
        setupLocalLLMAcknowledgementRequired = data.acknowledgement_required === true;
        const wrapper = document.getElementById('setup-local-llm-ack-wrap');
        const acknowledgement = document.getElementById('setup-local-llm-ack');
        if (acknowledgement) acknowledgement.checked = false;
        if (wrapper) wrapper.classList.toggle('is-hidden', !setupLocalLLMAcknowledgementRequired);
    } catch (error) {
        target.textContent = t('setup.local_llm_probe_error') + ': ' + error.message;
    }
}

function applySetupLocalLLMPatch(patch) {
    const enabled = document.getElementById('setup-local-llm-enabled').checked;
    patch.local_llm = {
        enabled,
        backend: document.getElementById('setup-local-llm-backend').value,
        model_family: document.getElementById('setup-local-llm-family').value,
        model_variant: document.getElementById('setup-local-llm-model').value,
        mtp: document.getElementById('setup-local-llm-mtp').value,
        context_size: document.getElementById('setup-local-llm-family').value === 'spark' ? 65536 : 16384,
        idle_timeout_minutes: 15,
        listen_port: 18081,
    };
    if (enabled) {
        const acknowledgement = document.getElementById('setup-local-llm-ack');
        const backend = document.getElementById('setup-local-llm-backend').value;
        if (!setupLocalLLMProbeFingerprint) {
            throw new Error(t('setup.local_llm_probe_error'));
        }
        if ((setupLocalLLMAcknowledgementRequired || backend === 'cpu') &&
            (!acknowledgement || !acknowledgement.checked)) {
            throw new Error(t('config.local_llm.ack_warning'));
        }
        patch._local_llm_setup = {
            enabled: true,
            role: document.getElementById('setup-local-llm-role').value,
            regular_provider: 'main',
            acknowledgement_fingerprint: setupLocalLLMAcknowledgementRequired
                ? setupLocalLLMProbeFingerprint
                : '',
        };
    }
}

// ── Step Navigation ──────────────────────────
function goToStep(index) {
    const flow = activeFlow();
    if (index < 0 || index >= flow.length) return;
    if (index > highestStepIndex) return;
    currentStepIndex = index;
    updateUI();
}

function setSetupStepBusy(busy) {
    ['btn-next', 'btn-skip-step', 'btn-back'].forEach(id => {
        const btn = document.getElementById(id);
        if (btn) btn.disabled = busy;
    });
    if (!busy) updateNextButtonState();
}

function releaseSetupStepBusySoon() {
    window.setTimeout(() => {
        setupStepInFlight = false;
        setSetupStepBusy(false);
    }, 250);
}

async function nextStep(skip = false) {
    if (setupStepInFlight || saving) return;
    setupStepInFlight = true;
    setSetupStepBusy(true);

    try {
        const flow = activeFlow();
        const stepId = flow[currentStepIndex];

        // Validate current step
        if (stepId === 'plan-select') {
            // Must select a profile before advancing
            if (!selectedProfile) {
                const grid = document.getElementById('profile-grid');
                if (grid) {
                    grid.classList.add('shake');
                    setTimeout(() => grid.classList.remove('shake'), 400);
                }
                return;
            }
            // Determine flow based on selected profile
            isQuickFlow = (selectedProfile.id !== 'custom');
            syncSetupPasswordTwins();
            // If the user just changed their profile choice (re-selected after
            // going back), reset any flow-specific state to avoid stale references.
            if (selectedProfile.id !== prevSelectedProfileId) {
                prevSelectedProfileId = selectedProfile.id;
                highestStepIndex = 0;
                // selectedProfile is already the new one; nothing else to reset.
            }
        } else if (stepId === 'plan-quick') {
            if (!validateQuickStep(skip)) return;
        } else if (stepId === 'step-0') {
            if (!validateStep0(skip)) return;
        }

        if (currentStepIndex < activeFlow().length - 1) {
            currentStepIndex++;
            if (currentStepIndex > highestStepIndex) highestStepIndex = currentStepIndex;
            updateUI();
        } else {
            await saveConfig();
        }
    } finally {
        releaseSetupStepBusySoon();
    }
}

function prevStep() {
    if (setupStepInFlight || saving) return;
    if (currentStepIndex > 0) {
        // When going back to plan-select, reset flow choice to allow changing
        if (currentStepIndex === 1) {
            // stay on plan-select; preview the flow of the still selected plan
            isQuickFlow = !!selectedProfile && selectedProfile.id !== 'custom';
            currentStepIndex = 0;
            highestStepIndex = 0;
        } else {
            currentStepIndex--;
        }
        updateUI();
    }
}

function renderStepIndicator() {
    const container = document.getElementById('step-indicator');
    if (!container || setupCompleted) return;
    const flow = activeFlow();
    const labels = activeLabels();
    const currentRaw = t(labels[currentStepIndex]);
    const currentLabel = (currentRaw && currentRaw !== labels[currentStepIndex]) ? currentRaw : labels[currentStepIndex].split('.').pop();
    let html = `<div class="step-compact"><p class="step-compact-text">${escapeHtml(t('setup.step_counter', { current: currentStepIndex + 1, total: flow.length }) + ' · ' + currentLabel)}</p><progress class="step-compact-progress" max="${flow.length}" value="${currentStepIndex + 1}"></progress></div>`;
    for (let i = 0; i < flow.length; i++) {
        const isActive    = i === currentStepIndex;
        const isCompleted = i < currentStepIndex;
        const isReachable = i <= highestStepIndex && !isActive;
        const classes = ['step-dot',
            isActive    ? 'active'    : '',
            isCompleted ? 'completed' : '',
            isReachable ? 'reachable' : '',
        ].filter(Boolean).join(' ');
        const clickable = isReachable || isCompleted;
        const raw = t(labels[i]);
        const label = (raw && raw !== labels[i]) ? raw : labels[i].split('.').pop();
        const stepContent = isCompleted ? '✓' : i + 1;
        html += `<div class="step-group">`;
        if (clickable) {
            html += `<button type="button" class="${classes}" aria-label="${escapeAttr(label)}" onclick="goToStep(${i})">${stepContent}</button>`;
        } else {
            html += `<div class="${classes}" aria-label="${escapeAttr(label)}"${isActive ? ' aria-current="step"' : ''}>${stepContent}</div>`;
        }
        html += `<span class="step-label">${escapeHtml(label)}</span>`;
        html += `</div>`;
        if (i < flow.length - 1) {
            const lineActive = i < currentStepIndex;
            html += `<div class="step-line${lineActive ? ' active' : ''}"></div>`;
        }
    }
    container.innerHTML = html;
}

function updateNextButtonState() {
    const btnNext = document.getElementById('btn-next');
    if (!btnNext) return;
    const flow = activeFlow();
    const stepId = flow[currentStepIndex];
    const shouldDisable = setupStepInFlight || saving || (stepId === 'plan-select' && !selectedProfile);
    btnNext.disabled = shouldDisable;
    btnNext.classList.toggle('disabled', shouldDisable);
}

// Set the Next/Save label for the current step. Leaves the button alone while a save is running.
function updateNavLabels() {
    if (saving) return;
    const btnNext = document.getElementById('btn-next');
    if (!btnNext) return;
    btnNext.innerHTML = currentStepIndex === activeFlow().length - 1
        ? t('setup.nav_save_and_start')
        : t('setup.nav_next');
}

// The scroll container is #main-content on desktop; narrow layouts scroll the page itself.
function resetSetupScroll() {
    const main = document.getElementById('main-content');
    if (main) main.scrollTop = 0;
    window.scrollTo(0, 0);
}

// Move keyboard/screen-reader focus to the heading of the visible section.
function focusSetupHeading(root) {
    const heading = root && root.querySelector('.section-title, h2');
    if (!heading) return;
    heading.setAttribute('tabindex', '-1');
    heading.focus({ preventScroll: true });
}

let lastSetupUIStepId = null;

function updateUI() {
    const flow = activeFlow();
    const stepId = flow[currentStepIndex];

    // Show active section
    document.querySelectorAll('.setup-section').forEach(s => {
        s.classList.toggle('active', s.id === stepId);
    });

    // Rebuild step indicator
    renderStepIndicator();

    // Update navigation buttons
    setupSetHidden(document.getElementById('btn-back'), currentStepIndex <= 0);
    setupSetHidden(document.getElementById('btn-skip-step'), !setupStepSkippable(stepId));
    updateNavLabels();
    updateNextButtonState();

    // Start each new step at the top; the first call at page load must not steal focus.
    if (lastSetupUIStepId !== null && lastSetupUIStepId !== stepId) {
        resetSetupScroll();
        focusSetupHeading(document.getElementById(stepId));
    }
    lastSetupUIStepId = stepId;
}

// ── Validation ───────────────────────────────
function validateStep0(skip = false) {
    let valid = true;

    // Password validation — always enforced, never skippable
    if (setupPasswordRequired === true) {
        const adminPassword = document.getElementById('admin-password').value;
        if (adminPassword.length < 8 || !adminPassword.trim()) {
            showFieldError('admin-password', 'err-admin-password');
            valid = false;
        } else {
            clearFieldError('admin-password', 'err-admin-password');
        }

        const confirmPassword = document.getElementById('admin-password-confirm').value;
        if (adminPassword.length >= 8 && confirmPassword !== adminPassword) {
            showFieldError('admin-password-confirm', 'err-admin-password-confirm');
            valid = false;
        } else {
            clearFieldError('admin-password-confirm', 'err-admin-password-confirm');
        }
    }

    // Skip remaining validation when user pressed Skip
    if (skip) return valid;

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

    // A custom provider has no built-in endpoint
    if (provider === 'custom') {
        if (!document.getElementById('llm-base-url').value.trim()) {
            showFieldError('llm-base-url', 'err-base-url');
            const advanced = document.getElementById('step0-advanced');
            if (advanced) advanced.open = true;
            valid = false;
        } else {
            clearFieldError('llm-base-url', 'err-base-url');
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
        const group = e.target.closest('.field-group') || e.target.parentElement;
        const errEl = group && group.querySelector('.field-error');
        if (errEl) errEl.classList.remove('visible');
    }
});

// Keep both flows' password fields in step and refresh the strength / match hints.
document.addEventListener('input', (e) => {
    const id = e.target && e.target.id;
    const twinId = setupPasswordTwinId(id);
    if (!twinId) return;
    const twin = document.getElementById(twinId);
    if (twin && twin.value !== e.target.value) twin.value = e.target.value;
    updateSetupPasswordIndicators(id);
    updateSetupPasswordIndicators(twinId);
});

document.addEventListener('click', (e) => {
    const btn = e.target.closest && e.target.closest('[data-password-toggle]');
    if (!btn) return;
    const input = document.getElementById(btn.getAttribute('data-password-toggle'));
    if (!input) return;
    const show = input.type === 'password';
    input.type = show ? 'text' : 'password';
    btn.textContent = show ? '\u{1F648}' : '\u{1F441}';
    btn.setAttribute('aria-pressed', show ? 'true' : 'false');
});

// ── Keyboard Navigation ───────────────────────
const SETUP_ENTER_INPUT_TYPES = ['text', 'password', 'url', 'email', 'number', 'search'];

function setupIsPageTarget(target) {
    return !target || target.nodeType === 9 || target.tagName === 'BODY' || target.tagName === 'HTML';
}

// Enter advances only from text-like fields of a wizard step or from the bare page.
// Buttons, links, selects, checkboxes and dialogs keep their native behaviour.
function setupEnterAdvances(target) {
    if (setupIsPageTarget(target)) return true;
    if (target.tagName !== 'INPUT') return false;
    if (SETUP_ENTER_INPUT_TYPES.indexOf(String(target.type || 'text').toLowerCase()) < 0) return false;
    if (target.closest('[role="dialog"], [aria-modal="true"], .modal-overlay')) return false;
    return !!target.closest('.setup-section');
}

// Escape goes back only when focus is not inside a field, control or dialog.
function setupEscapeGoesBack(target) {
    if (setupIsPageTarget(target)) return true;
    if (['INPUT', 'TEXTAREA', 'SELECT'].indexOf(target.tagName) >= 0) return false;
    return !target.closest('[role="dialog"], [aria-modal="true"], .modal-overlay');
}

function setupKeyboardNavBlocked(e) {
    if (e.defaultPrevented || e.altKey || e.ctrlKey || e.metaKey || e.shiftKey) return true;
    if (document.querySelector('#setup-bootstrap-gate.open, #or-browser-overlay, .modal-overlay.open, .modal-overlay.active')) return true;
    return Array.from(document.querySelectorAll('[aria-modal="true"]')).some(el =>
        el.id !== 'setup-bootstrap-gate' && el.getClientRects().length > 0);
}

document.addEventListener('keydown', (e) => {
    if (e.key !== 'Enter' && e.key !== 'Escape') return;
    if (setupKeyboardNavBlocked(e)) return;
    if (e.key === 'Enter' && setupEnterAdvances(e.target)) { e.preventDefault(); nextStep(); }
    else if (e.key === 'Escape' && setupEscapeGoesBack(e.target)) { e.preventDefault(); prevStep(); }
});

// ── Build Provider Entries ───────────────────
// Creates provider entries from setup wizard fields.
// Each subsystem (embeddings, vision, whisper, helper) gets its own
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
    if (embProvValue && embProvValue !== '' && embProvValue !== 'local-granite') {
        const embModel = document.getElementById('emb-model').value.trim();
        if (embProvValue === 'ollama') {
            providers.push({
                id: 'embeddings', name: 'Embeddings', type: 'ollama',
                base_url: providerConfig.ollama.baseUrl, api_key: '', model: embModel,
            });
        } else {
            // "internal" = same URL/key as main, different model (unless overridden)
            const embKey = document.getElementById('emb-api-key').value.trim();
            providers.push({
                id: 'embeddings', name: 'Embeddings', type: mainType,
                base_url: mainUrl, api_key: embKey || mainKey, model: embModel,
            });
        }
    }

    // Vision provider
    const visionType = document.getElementById('vision-provider').value;
    if (visionType) {
        const visionModel = document.getElementById('vision-model').value.trim();
        const visionCfg = providerConfig[visionType] || providerConfig.custom;
        if (visionType === 'main') {
            providers.push({
                id: 'vision', name: 'Vision', type: mainType,
                base_url: mainUrl, api_key: mainKey, model: visionModel || mainModel,
            });
        } else {
            providers.push({
                id: 'vision', name: 'Vision', type: visionType,
                base_url: visionCfg.baseUrl || mainUrl,
                api_key: visionType === 'ollama' ? '' : mainKey,
                model: visionModel,
            });
        }
    }

    // Whisper / STT provider
    const whisperType = document.getElementById('whisper-provider').value;
    if (whisperType) {
        const whisperModel = document.getElementById('whisper-model').value.trim();
        const whisperCfg = providerConfig[whisperType] || providerConfig.custom;
        if (whisperType === 'main') {
            providers.push({
                id: 'whisper', name: 'Whisper / STT', type: mainType,
                base_url: mainUrl, api_key: mainKey, model: whisperModel || mainModel,
            });
        } else {
            providers.push({
                id: 'whisper', name: 'Whisper / STT', type: whisperType,
                base_url: whisperCfg.baseUrl || mainUrl,
                api_key: whisperType === 'ollama' ? '' : mainKey,
                model: whisperModel,
            });
        }
    }

    // Helper LLM provider
    if (document.getElementById('helper-llm').checked) {
        const helperModel = document.getElementById('helper-model').value.trim();
        if (helperModel) {
            providers.push({
                id: 'helper', name: 'Helper LLM', type: mainType,
                base_url: mainUrl, api_key: mainKey, model: helperModel,
            });
        }
    }

    // Agnes AI uses dedicated models on the same API base for image and video generation.
    if (mainType === 'agnes') {
        providers.push({
            id: 'image_gen', name: 'Agnes AI Image Gen', type: 'agnes',
            base_url: mainUrl, api_key: mainKey, model: 'agnes-image-2.1-flash',
        });
        providers.push({
            id: 'video_gen', name: 'Agnes AI Video Gen', type: 'agnes',
            base_url: mainUrl, api_key: mainKey, model: 'agnes-video-v2.0',
        });
    }

    return providers;
}

// ── Build Trust Level Patch ──────────────────
// Returns a config patch based on the selected trust level (1-4).
// Base tools (Memory, Knowledge Graph, Secrets Vault, Scheduler, Notes,
// Missions, Inventory, Memory Maintenance, Journal, Contacts) are always
// enabled at every level — they are harmless and essential. Write access is
// always allowed since these tools only store data locally and pose no risk.
function buildTrustLevelPatch(level) {
    const n = parseInt(level, 10) || 1;

    // Base tools — always enabled at all levels, always read-write
    const baseTools = {
        memory:             {
            enabled: true,
            ondemand_retrieval: {
                enabled: true,
                max_essential_memories: 1,
                max_available_memories: 6,
                max_available_kg_nodes: 6,
                max_available_chars: 1600,
                dedupe_scope: 'turn'
            }
        },
        knowledge_graph:    { enabled: true, auto_extraction: true, prompt_injection: true },
        secrets_vault:      { enabled: true },
        scheduler:          { enabled: true },
        notes:              { enabled: true },
        missions:           { enabled: true },
        inventory:          { enabled: true },
        memory_maintenance: { enabled: true },
        journal:            { enabled: true },
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
            sudo_unrestricted:      n >= 4,
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
        meshcentral:      { readonly: n <= 2, allow_power_actions: n >= 3 },
        // Docker is useful infrastructure for many integrations. Keep it off
        // only at the lowest trust level, read-only at level 2, and writable
        // once the user explicitly grants broader agent permissions.
        docker:           { enabled: n >= 2, readonly: n <= 2 },
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
        google_workspace: { readonly: n <= 2 },
        netlify:          { readonly: n <= 2 },
        invasion_control: { readonly: n <= 2 },
        remote_control:   { readonly: n <= 2 },
    };

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
    const helperRequested = document.getElementById('helper-llm').checked;
    const helperModel = document.getElementById('helper-model').value.trim();
    const helperConfigured = helperRequested && helperModel !== '';
    const selectedLanguage = document.getElementById('system-language').value;
    const uiLanguage = selectedLanguage || document.documentElement.lang || 'en';
    const patch = {
        providers: buildProviderEntries(),
        server: {
            ui_language: uiLanguage,
        },
        llm: {
            provider: 'main',
            helper_enabled: helperConfigured,
            helper_provider: helperConfigured ? 'helper' : '',
            helper_model: helperConfigured ? helperModel : '',
            use_native_functions: document.getElementById('native-functions').checked,
        },
        agent: {
            system_language: setupAgentLanguage(uiLanguage, (document.getElementById('system-language-custom') || {}).value),
            personality_engine_v2: helperConfigured,
            personality_engine: helperConfigured,
            core_personality: document.getElementById('core-personality').value,
        },
        auth: {
            enabled: true,
            admin_password: document.getElementById('admin-password').value,
        },
        maintenance: {
            enabled: document.getElementById('maintenance-enabled').checked,
        },
    };

    if (document.getElementById('llm-provider').value === 'agnes') {
        patch.image_generation = {
            enabled: true,
            provider: 'image_gen',
        };
        patch.video_generation = {
            enabled: true,
            provider: 'video_gen',
            default_duration_seconds: 6,
            default_resolution: '768P',
        };
    }

    // Embeddings: reference provider entry or disable
    const embProvider = document.getElementById('emb-provider').value;
    if (embProvider === 'local-granite') {
        patch.embeddings = {
            provider: 'local-granite',
            local: { backend: 'auto', context_size: 2048, batch_size: 2048 },
            multimodal: false
        };
    } else {
        patch.embeddings = { provider: (embProvider && embProvider !== '') ? 'embeddings' : 'disabled' };
    }

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

    // Trust level (Mutprobe): merge permission config
    const trustRadio = document.querySelector('input[name="trust-level"]:checked');
    if (trustRadio) {
        const trustPatch = buildTrustLevelPatch(trustRadio.value);
        deepMergePatch(patch, trustPatch);
    }

    applySetupLocalLLMPatch(patch);
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
        const patch = isQuickFlow ? buildQuickConfigPatch() : buildConfigPatch();
        const resp = await fetch('/api/setup', {
            method: 'POST',
            headers: setupRequestHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify(patch),
        });

        if (!resp.ok) {
            const text = await resp.text();
            let errMsg = text || `HTTP ${resp.status}`;
            try {
                const parsed = JSON.parse(text);
                // A server restart issues a new token; ask for it instead of failing silently.
                if (parsed.code === 'setup_bootstrap_token_required') openSetupOwnerGate(false, true);
                if (parsed.code === 'setup_vault_locked') openSetupOwnerGate(true, false);
                if (parsed.error && parsed.details) errMsg = `${parsed.error}: ${parsed.details}`;
                else if (parsed.error) errMsg = parsed.error;
                else if (parsed.message) errMsg = parsed.message;
            } catch (_) { /* not JSON, use raw text */ }
            throw new Error(errMsg);
        }

        const result = await resp.json();

        // Show success screen
        document.querySelectorAll('.setup-section').forEach(s => s.classList.remove('active'));
        setupSetHidden(document.getElementById('header-nav'), true);
        setupCompleted = true;
        const indicator = document.getElementById('step-indicator');
        if (indicator) {
            indicator.innerHTML = '';
            setupSetHidden(indicator, true);
        }
        setupSetHidden(document.getElementById('setup-https-status'), true);
        setupSetHidden(document.getElementById('btn-skip-setup'), true);
        const successScreen = document.getElementById('success-screen');
        successScreen.classList.add('active');
        resetSetupScroll();
        focusSetupHeading(successScreen);

        setupSetHidden(document.getElementById('success-signed-in'), result.signed_in !== true);
        setupSetHidden(document.getElementById('success-sign-in-hint'), result.signed_in === true);
        if (result.needs_restart) {
            setupSetHidden(document.getElementById('restart-notice'), false);
        }
        if (result.local_llm_job_id && result.local_llm_job_token) {
            pollSetupLocalLLMJob(result.local_llm_job_id, result.local_llm_job_token);
        }

        showToast(t('setup.toast_config_saved'), 'success');
    } catch (err) {
        showToast(t('setup.toast_error_prefix') + err.message, 'error');
        // A failed save may come from the optional local model: open its section so the cause is visible.
        const localLLMEnabled = document.getElementById('setup-local-llm-enabled');
        const localLLMSection = document.getElementById('step-local-llm');
        if (localLLMEnabled && localLLMEnabled.checked && localLLMSection) localLLMSection.open = true;
        btnNext.disabled = false;
        btnNext.innerHTML = t('setup.nav_save_and_start');
    } finally {
        saving = false;
    }
}

// First-steps chips on the success screen: hand the prompt to the chat, which only prefills it.
document.addEventListener('click', (ev) => {
    const chip = ev.target.closest && ev.target.closest('.success-prompt-chip');
    if (!chip) return;
    try { sessionStorage.setItem('aurago.chat.prefill', t(chip.getAttribute('data-prompt-key'))); } catch (_) { /* storage blocked: open the chat empty */ }
    window.location.href = '/';
});

async function pollSetupLocalLLMJob(id, token) {
    const target = document.getElementById('local-llm-job-notice');
    const modelName = { qwen: 'AuraGo-Qwen', ling: 'AuraGo-Ling', spark: 'AuraGo-Spark' }[document.getElementById('setup-local-llm-family')?.value] || 'AuraGo-Qwen';
    const modelText = key => t(key).replace('{model}', modelName);
    setupSetHidden(target, false);
    target.textContent = modelText('setup.local_llm_job_running');
    for (let attempt = 0; attempt < 720; attempt++) {
        try {
            const response = await fetch('/api/setup/local-llm/job?id=' + encodeURIComponent(id), {
                headers: { 'X-Setup-Job-Token': token },
            });
            const job = await response.json();
            if (!response.ok) throw new Error(job.message || job.error || ('HTTP ' + response.status));
            if (job.state === 'completed') {
                target.textContent = modelText('setup.local_llm_job_complete');
                return;
            }
            if (job.state === 'failed') {
                target.textContent = modelText('setup.local_llm_job_failed') + ' (' + (job.error_code || 'local_llm_error') + ')';
                return;
            }
            target.textContent = modelText('setup.local_llm_job_progress').replace('{progress}', Math.round((job.progress || 0) * 100));
        } catch (error) {
            target.textContent = modelText('setup.local_llm_job_failed') + ': ' + error.message;
            return;
        }
        await new Promise(resolve => setTimeout(resolve, 5000));
    }
}

// ── Skip Setup ───────────────────────────────
function setSetupSkipBusy(busy) {
    const skipBtn = document.getElementById('btn-skip-setup');
    if (skipBtn) skipBtn.disabled = busy;
}

async function skipSetup() {
    if (setupSkipInFlight) return;
    setupSkipInFlight = true;
    setSetupSkipBusy(true);

    let confirmed = false;
    try {
        confirmed = await showConfirm(t('setup.confirm_skip_setup_title'), t('setup.confirm_skip_setup'));
        if (confirmed) {
            window.location.href = '/?skip_setup=1';
            return;
        }
    } finally {
        if (!confirmed) {
            setupSkipInFlight = false;
            setSetupSkipBusy(false);
        }
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
    requestAnimationFrame(() => toast.classList.add('show'));
    setTimeout(() => {
        toast.classList.remove('show');
        toast.classList.add('toast-exit');
        setTimeout(() => toast.remove(), 400);
    }, 5000);
}

// ── Browser Language Auto-detection ──────────
(function detectAndSetLanguage() {
    const lang = (navigator.languages && navigator.languages[0]) || navigator.language || 'en';
    const base = lang.toLowerCase().split('-')[0];
    const detected = SUPPORTED_LANGS.includes(base) ? base : 'en';
    const sel = document.getElementById('system-language');
    if (sel) sel.value = detected;
    const quickSel = document.getElementById('quick-language');
    if (quickSel) quickSel.value = detected;
    const planSel = document.getElementById('plan-language');
    if (planSel) planSel.value = detected;
    fetchAndApplyLang(detected);
})();

// ── i18n: populate text ──
function applyI18N() {
    renderSetupHTTPSStatus();
    // Page title
    document.title = t('setup.page_title');

    // Generic: data-i18n → textContent
    document.querySelectorAll('[data-i18n]').forEach(el => {
        const key = el.getAttribute('data-i18n');
        const val = t(key);
        if (val !== key) el.textContent = val;
    });

    // Generic: data-i18n-html → safe text nodes (with \n → <br>)
    document.querySelectorAll('[data-i18n-html]').forEach(el => {
        const key = el.getAttribute('data-i18n-html');
        const val = t(key);
        if (val !== key) renderSetupI18nMultilineText(el, val);
    });

    // Generic: data-i18n-placeholder → placeholder
    document.querySelectorAll('[data-i18n-placeholder]').forEach(el => {
        const key = el.getAttribute('data-i18n-placeholder');
        const val = t(key);
        if (val !== key) el.placeholder = val;
    });

    // Generic: data-i18n-title → title attribute
    document.querySelectorAll('[data-i18n-title]').forEach(el => {
        const key = el.getAttribute('data-i18n-title');
        const val = t(key);
        if (val !== key) el.title = val;
    });

    // Generic: data-i18n-aria-label → aria-label attribute
    document.querySelectorAll('[data-i18n-aria-label]').forEach(el => {
        const key = el.getAttribute('data-i18n-aria-label');
        const val = t(key);
        if (val !== key) el.setAttribute('aria-label', val);
    });

    updateCorePersonalityDesc();
    ['admin-password', 'admin-password-confirm', 'quick-admin-password', 'quick-admin-password-confirm']
        .forEach(updateSetupPasswordIndicators);

    // Special: admin password — show/hide required star based on auth status.
    // Star is hidden until the security status response confirms password is required.
    const lblPw = document.getElementById('lbl-admin-password');
    if (lblPw) {
        const star = lblPw.querySelector('.required-star');
        if (star) {
            // Default hidden; shown only when explicitly confirmed required.
            star.style.display = (setupPasswordRequired === true) ? '' : 'none';
        }
    }
}

function renderSetupI18nMultilineText(el, text) {
    el.replaceChildren();
    String(text).split('\n').forEach((part, index) => {
        if (index > 0) el.appendChild(document.createElement('br'));
        el.appendChild(document.createTextNode(part));
    });
}

// ── Init ─────────────────────────────────────
applyI18N();
onProviderChange();
onEmbProviderChange();
onHelperToggle();
updateUI();
loadProfiles();
