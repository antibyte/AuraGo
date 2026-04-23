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
let profiles = [];
let saving = false;
let setupPasswordRequired = true;
let csrfToken = '';

// Derived helpers
function activeFlow()  { return isQuickFlow ? QUICK_FLOW_STEPS  : CUSTOM_FLOW_STEPS; }
function activeLabels(){ return isQuickFlow ? QUICK_FLOW_LABELS : CUSTOM_FLOW_LABELS; }
function currentStepId(){ return activeFlow()[currentStepIndex]; }
function totalSteps()  { return activeFlow().length; }

// ── Security: check setup status & redirect if already configured ──
(async function checkSetupStatus() {
    try {
        const resp = await fetch('/api/setup/status');
        if (!resp.ok) return;
        const data = await resp.json();
        if (!data.needs_setup) {
            window.location.href = '/';
            return;
        }
        if (data.csrf_token) csrfToken = data.csrf_token;
    } catch (e) { /* ignore — proceed with setup */ }
})();

// ── Security: HTTPS warning ──────────────────
(function httpsWarning() {
    const isSecure = location.protocol === 'https:';
    const isLocal = ['localhost', '127.0.0.1', '[::1]'].includes(location.hostname);
    if (!isSecure && !isLocal) {
        const banner = document.getElementById('https-warning');
        if (banner) {
            banner.classList.remove('is-hidden');
            document.body.classList.add('has-https-warning');
        }
    }
})();

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

// ── Provider Profile Loading ─────────────────
async function loadProfiles() {
    try {
        const resp = await fetch('/api/setup/profiles');
        if (!resp.ok) throw new Error(resp.statusText);
        const data = await resp.json();
        profiles = data.profiles || [];
        renderProfileCards(profiles);
    } catch (e) {
        const loading = document.getElementById('profile-loading');
        if (loading) loading.innerHTML = `<p style="color:var(--text-secondary);font-size:0.85rem;">${escapeHtml(t('setup.plan_load_error') || 'Could not load profiles.')}</p>`;
    }
}

function getFeatureBadges(features) {
    if (!features) return '';
    const map = [
        ['vision',           'setup.feature_vision',  '👁'],
        ['tts',              'setup.feature_tts',     '🔊'],
        ['image_generation', 'setup.feature_images',  '🎨'],
        ['music_generation', 'setup.feature_music',   '🎵'],
    ];
    return map.map(([key, i18nKey, icon]) => {
        const active = !!features[key];
        const label = (t(i18nKey) || key);
        const cls = active ? 'feature-badge' : 'feature-badge inactive';
        return `<span class="${cls}" title="${active ? '' : t('setup.feature_unavailable') || 'Not available'}">${icon} ${escapeHtml(label)}</span>`;
    }).join('');
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
        grid.innerHTML = `<p style="color:var(--text-secondary);font-size:0.85rem;grid-column:1/-1;text-align:center;padding:2rem;">${escapeHtml(t('setup.plan_no_profiles') || 'No profiles available.')}</p>`;
        return;
    }
    grid.innerHTML = list.map(p => {
        const isCustom = p.id === 'custom';
        const name = tProfile(p.id, 'name', p.name);
        const desc = tProfile(p.id, 'description', p.description || '');
        const pricing = tProfile(p.id, 'pricing', p.pricing_label || '');
        return `
        <div class="profile-card${isCustom ? ' is-custom' : ''}"
             id="profile-card-${escapeAttr(p.id)}"
             onclick="selectProfile('${escapeAttr(p.id)}')">
            <div class="profile-check">✓</div>
            ${p.recommended ? '<div class="profile-recommended-bubble">Recommended</div>' : ''}
            <div class="profile-card-icon">${escapeHtml(p.icon || (isCustom ? '⚙️' : '🤖'))}</div>
            <div class="profile-card-name">${escapeHtml(name)}</div>
            <div class="profile-card-description">${escapeHtml(desc)}</div>
            ${p.features ? `<div class="profile-features">${getFeatureBadges(p.features)}</div>` : ''}
            ${pricing ? `<div class="profile-card-pricing">${escapeHtml(pricing)}</div>` : ''}
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

function updateQuickProfileUI() {
    const minimaxOptions = document.getElementById('quick-minimax-options');
    if (minimaxOptions) {
        setupSetHidden(minimaxOptions, !isMiniMaxQuickProfile(selectedProfile));
    }
    if (!selectedProfile || selectedProfile.id === 'custom') return;

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

function selectProfile(profileId) {
    selectedProfile = profiles.find(p => p.id === profileId) || null;
    // Update card selection state
    document.querySelectorAll('.profile-card').forEach(card => {
        card.classList.toggle('selected', card.id === `profile-card-${profileId}`);
    });
    if (!selectedProfile) return;
    // Update Next button state
    updateNextButtonState();
    updateQuickProfileUI();
}

// ── Quick Connection Test ────────────────────
async function testQuickConnection() {
    if (!selectedProfile) return;
    const btn = document.getElementById('btn-quick-test');
    const result = document.getElementById('quick-test-result');
    if (!btn || btn.disabled) return;

    const apiKey = document.getElementById('quick-api-key').value.trim();
    if (!apiKey) {
        result.textContent = t('setup.step0_api_key_error') || 'API Key is required.';
        result.className = 'field-hint error';
        setupSetHidden(result, false);
        return;
    }

    btn.disabled = true;
    result.textContent = t('setup.step0_test_testing') || 'Testing…';
    result.className = 'field-hint';
    setupSetHidden(result, false);

    try {
        const runtime = getQuickProfileRuntimeConfig(selectedProfile);
        const resp = await fetch('/api/setup/test', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                provider_type: runtime.providerType,
                base_url: runtime.baseUrl,
                api_key: apiKey,
                model: runtime.mainModel,
            }),
        });
        const data = await resp.json();
        if (data.ok) {
            result.textContent = '✓ ' + (t('setup.step0_test_success') || 'Connection successful!');
            result.className = 'field-hint success';
        } else {
            result.textContent = '✕ ' + (data.error || t('setup.step0_test_failed') || 'Connection failed.');
            result.className = 'field-hint error';
        }
    } catch (err) {
        result.textContent = '✕ ' + err.message;
        result.className = 'field-hint error';
    } finally {
        btn.disabled = false;
    }
}

// ── Quick Flow Language Change ───────────────
function onQuickLanguageChange() {
    const sel = document.getElementById('quick-language');
    if (!sel) return;
    fetchAndApplyLang(sel.value);
    // Mirror to other language selectors so all flows stay in sync
    const mainSel = document.getElementById('system-language');
    if (mainSel) mainSel.value = sel.value;
    const planSel = document.getElementById('plan-language');
    if (planSel) planSel.value = sel.value;
}

function onPlanLanguageChange() {
    const sel = document.getElementById('plan-language');
    if (!sel) return;
    fetchAndApplyLang(sel.value);
    // Mirror to other language selectors so all flows stay in sync
    const mainSel = document.getElementById('system-language');
    if (mainSel) mainSel.value = sel.value;
    const quickSel = document.getElementById('quick-language');
    if (quickSel) quickSel.value = sel.value;
}

// ── Quick Flow Validation ────────────────────
function validateQuickStep(skip = false) {
    let valid = true;

    if (setupPasswordRequired) {
        const pw = document.getElementById('quick-admin-password').value.trim();
        if (pw.length < 8) {
            showFieldError('quick-admin-password', 'err-quick-admin-password');
            valid = false;
        } else {
            clearFieldError('quick-admin-password', 'err-quick-admin-password');
        }
        const pw2 = document.getElementById('quick-admin-password-confirm').value.trim();
        if (pw.length >= 8 && pw2 !== pw) {
            showFieldError('quick-admin-password-confirm', 'err-quick-admin-password-confirm');
            valid = false;
        } else {
            clearFieldError('quick-admin-password-confirm', 'err-quick-admin-password-confirm');
        }
    }

    if (!skip && selectedProfile && selectedProfile.id !== 'custom') {
        const apiKey = document.getElementById('quick-api-key').value.trim();
        if (!apiKey) {
            showFieldError('quick-api-key', 'err-quick-api-key');
            valid = false;
        } else {
            clearFieldError('quick-api-key', 'err-quick-api-key');
        }
    }

    return valid;
}

// ── Quick Flow Config Patch Builder ──────────
function buildQuickConfigPatch() {
    if (!selectedProfile) return buildConfigPatch();

    const p = selectedProfile;
    const runtime = getQuickProfileRuntimeConfig(p);
    const apiKey = (document.getElementById('quick-api-key').value || '').trim();
    const adminPassword = (document.getElementById('quick-admin-password').value || '').trim();
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

    // Read trust level from radio button (may have been pre-selected by nextStep)
    const trustRadio = document.querySelector('input[name="trust-level"]:checked');
    const trustLevel = trustRadio ? parseInt(trustRadio.value, 10) : (p.default_trust_level || 1);

    const langMap = {de:'Deutsch',en:'English',es:'Español',fr:'Français',pl:'Polski',zh:'中文',hi:'हिन्दी',nl:'Nederlands',it:'Italiano',pt:'Português',da:'Dansk',ja:'日本語',sv:'Svenska',no:'Norsk',el:'Ελληνικά',cs:'Čeština'};
    const patch = {
        server: {
            ui_language: lang,
        },
        auth: {
            enabled: true,
            ...(adminPassword ? { admin_password: adminPassword } : {}),
        },
        providers,
        agent: {
            system_language: langMap[lang] || lang,
        },
        llm: {
            provider: 'main',
            use_native_functions: p.native_function_calling !== false,
            helper_enabled: !!(p.features && p.features.helper && m.helper),
            helper_provider: (p.features && p.features.helper && m.helper) ? 'helper' : '',
        },
        embeddings: {
            provider: (p.features && p.features.embeddings && m.embeddings) ? 'embeddings' : 'disabled',
        },
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
    };

    // Merge trust level permissions
    deepMergePatch(patch, buildTrustLevelPatch(trustLevel));

    // TTS — uses its own config structure (NOT the provider system)
    if (p.tts && p.tts.provider) {
        const ttsProvider = p.tts.provider;
        patch.tts = { provider: ttsProvider };
        patch.tts[ttsProvider] = { api_key: apiKey };
        if (p.tts.model_id) patch.tts[ttsProvider].model_id = p.tts.model_id;
        if (p.tts.voice_id) patch.tts[ttsProvider].voice_id = p.tts.voice_id;
        if (p.tts.speed)    patch.tts[ttsProvider].speed    = p.tts.speed;
    }

    // Merge any extra config_patch defined in the profile YAML
    if (p.config_patch) deepMergePatch(patch, p.config_patch);

    return patch;
}
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

    syncHelperModelSuggestion();
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

// ── Embeddings Provider Change Handler ───────
function onEmbProviderChange() {
    const prov = document.getElementById('emb-provider').value;
    const group = document.getElementById('emb-apikey-group');
    if (group) setupSetHidden(group, prov !== 'internal');
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
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ provider_type: providerType, base_url: baseUrl, api_key: apiKey, model: model }),
        });
        const data = await resp.json();
        if (data.ok) {
            result.textContent = '✓ ' + t('setup.step0_test_success');
            result.className = 'field-hint test-result-ok';
        } else {
            result.textContent = '✕ ' + (data.error || t('setup.step0_test_failed'));
            result.className = 'field-hint test-result-fail';
        }
    } catch (err) {
        result.textContent = '✕ ' + err.message;
        result.className = 'field-hint test-result-fail';
    } finally {
        btn.disabled = false;
    }
}

// ── Agent Language Change Handler ────────────
function onLanguageChange() {
    const sel = document.getElementById('system-language');
    const customInput = document.getElementById('system-language-custom');
    if (sel.value === 'custom') {
        setupSetHidden(customInput, false);
        customInput.focus();
    } else {
        setupSetHidden(customInput, true);
        fetchAndApplyLang(sel.value);
    }
}

function fetchAndApplyLang(langValue) {
    document.documentElement.lang = langValue || 'en';
    fetch('/api/i18n?lang=' + encodeURIComponent(langValue))
        .then(r => r.ok ? r.json() : null)
        .then(json => {
            if (json && json.data && typeof json.data === 'object') {
                I18N = json.data;
                applyI18N();
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

// ── Step Navigation ──────────────────────────
function goToStep(index) {
    const flow = activeFlow();
    if (index < 0 || index >= flow.length) return;
    if (index > highestStepIndex) return;
    currentStepIndex = index;
    updateUI();
}

function nextStep(skip = false) {
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
    } else if (stepId === 'plan-quick') {
        if (!validateQuickStep(skip)) return;
    } else if (stepId === 'step-0') {
        if (!validateStep0(skip)) return;
    }

    const nextIndex = currentStepIndex + 1;
    const nextId = activeFlow()[nextIndex];

    // Pre-select trust level from profile when entering step-3 in quick flow
    if (nextId === 'step-3' && isQuickFlow && selectedProfile && selectedProfile.default_trust_level) {
        const level = selectedProfile.default_trust_level;
        const radio = document.querySelector(`input[name="trust-level"][value="${level}"]`);
        if (radio) radio.checked = true;
    }

    if (currentStepIndex < activeFlow().length - 1) {
        currentStepIndex++;
        if (currentStepIndex > highestStepIndex) highestStepIndex = currentStepIndex;
        updateUI();
    } else {
        saveConfig();
    }
}

function prevStep() {
    if (currentStepIndex > 0) {
        // When going back to plan-select, reset flow choice to allow changing
        if (currentStepIndex === 1) {
            // stay on plan-select; reset flow
            isQuickFlow = false;
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
    if (!container) return;
    const flow = activeFlow();
    const labels = activeLabels();
    let html = '';
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
        html += `<div class="step-group">`;
        html += `<div class="${classes}"${clickable ? ` onclick="goToStep(${i})" style="cursor:pointer"` : ''}>${isCompleted ? '✓' : i + 1}</div>`;
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
    const shouldDisable = (stepId === 'plan-select' && !selectedProfile);
    btnNext.disabled = shouldDisable;
    btnNext.classList.toggle('disabled', shouldDisable);
}

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

    const btnNext = document.getElementById('btn-next');
    if (btnNext) {
        if (currentStepIndex === flow.length - 1) {
            btnNext.innerHTML = t('setup.nav_save_and_start');
        } else {
            btnNext.innerHTML = t('setup.nav_next');
        }
    }
    updateNextButtonState();
}

// ── Validation ───────────────────────────────
function validateStep0(skip = false) {
    let valid = true;

    // Password validation — always enforced, never skippable
    if (setupPasswordRequired) {
        const adminPassword = document.getElementById('admin-password').value.trim();
        if (adminPassword.length < 8) {
            showFieldError('admin-password', 'err-admin-password');
            valid = false;
        } else {
            clearFieldError('admin-password', 'err-admin-password');
        }

        const confirmPassword = document.getElementById('admin-password-confirm').value.trim();
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

// ── Keyboard Navigation ───────────────────────
document.addEventListener('keydown', (e) => {
    // Ignore when typing in inputs/textareas/selects
    const tag = e.target.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') {
        if (e.key === 'Enter' && tag !== 'TEXTAREA') {
            e.preventDefault();
            nextStep();
        }
        return;
    }
    if (e.key === 'Enter') { e.preventDefault(); nextStep(); }
    if (e.key === 'Escape') { e.preventDefault(); prevStep(); }
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
    if (embProvValue && embProvValue !== '') {
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
        memory:             { enabled: true },
        knowledge_graph:    { enabled: true },
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
    const helperRequested = document.getElementById('helper-llm').checked;
    const helperModel = document.getElementById('helper-model').value.trim();
    const helperConfigured = helperRequested && helperModel !== '';
    const selectedLanguage = document.getElementById('system-language').value;
    const uiLanguage = selectedLanguage === 'custom'
        ? (document.documentElement.lang || 'en')
        : (selectedLanguage || document.documentElement.lang || 'en');
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
            system_language: (function() {
                const langMap = {de:'Deutsch',en:'English',es:'Español',fr:'Français',pl:'Polski',zh:'中文',hi:'हिन्दी',nl:'Nederlands',it:'Italiano',pt:'Português',da:'Dansk',ja:'日本語',sv:'Svenska',no:'Norsk',el:'Ελληνικά',cs:'Čeština'};
                const v = document.getElementById('system-language').value;
                if (v === 'custom') return document.getElementById('system-language-custom').value.trim();
                return langMap[v] || v;
            })(),
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
        const patch = isQuickFlow ? buildQuickConfigPatch() : buildConfigPatch();
        const resp = await fetch('/api/setup', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'X-CSRF-Token': csrfToken,
            },
            body: JSON.stringify(patch),
        });

        if (!resp.ok) {
            const text = await resp.text();
            let errMsg = text || `HTTP ${resp.status}`;
            try {
                const parsed = JSON.parse(text);
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
        const indicator = document.getElementById('step-indicator');
        if (indicator) indicator.innerHTML = '';
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
async function skipSetup() {
    if (await showConfirm(t('setup.confirm_skip_setup_title') || 'Skip Setup', t('setup.confirm_skip_setup'))) {
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
    const supported = ['de','en','es','fr','pl','zh','hi','nl','it','pt','da','ja','sv','no','cs','el'];
    const detected = supported.includes(base) ? base : 'en';
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
    // Page title
    document.title = t('setup.page_title');

    // Generic: data-i18n → textContent
    document.querySelectorAll('[data-i18n]').forEach(el => {
        const key = el.getAttribute('data-i18n');
        const val = t(key);
        if (val !== key) el.textContent = val;
    });

    // Generic: data-i18n-html → innerHTML (with \n → <br>)
    document.querySelectorAll('[data-i18n-html]').forEach(el => {
        const key = el.getAttribute('data-i18n-html');
        const val = t(key);
        if (val !== key) el.innerHTML = val.replace(/\n/g, '<br>');
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

    // Special: admin password — show/hide required star based on auth status
    const lblPw = document.getElementById('lbl-admin-password');
    if (lblPw) {
        const star = lblPw.querySelector('.required-star');
        if (star) star.style.display = setupPasswordRequired ? '' : 'none';
    }
}

// ── Init ─────────────────────────────────────
applyI18N();
onProviderChange();
onEmbProviderChange();
onHelperToggle();
updateUI();
loadProfiles();
