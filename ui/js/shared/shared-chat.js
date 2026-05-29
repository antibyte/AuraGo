/**
 * AuraGo - Shared chat JavaScript
 * Chat-only theme definitions and controls loaded on chat surfaces.
 */

// ═══════════════════════════════════════════════════════════════
// THEME MANAGEMENT
// ═══════════════════════════════════════════════════════════════

const CHAT_THEME_DEFINITIONS = [
    { theme: 'dark', icon: 'theme-dark', labelKey: 'chat.theme_standard', fallbackLabel: 'Standard' },
    { theme: 'light', icon: 'theme-light', labelKey: 'chat.theme_light', fallbackLabel: 'Light' },
    { theme: 'retro-crt', icon: 'theme-retro-crt', labelKey: 'chat.theme_retro_crt', fallbackLabel: 'Retro CRT' },
    { theme: '8bit', icon: 'theme-8bit', labelKey: 'chat.theme_8bit', fallbackLabel: '8Bit' },
    { theme: 'cyberwar', icon: 'theme-cyberwar', labelKey: 'chat.theme_cyberwar', fallbackLabel: 'Cyberwar' },
    { theme: 'lollipop', icon: 'theme-lollipop', labelKey: 'chat.theme_lollipop', fallbackLabel: 'Lollipop' },
    { theme: 'dark-sun', icon: 'theme-dark-sun', labelKey: 'chat.theme_dark_sun', fallbackLabel: 'Dark Sun' },
    { theme: 'ocean', icon: 'theme-ocean', labelKey: 'chat.theme_ocean', fallbackLabel: 'Ocean' },
    { theme: 'sandstorm', icon: 'theme-sandstorm', labelKey: 'chat.theme_sandstorm', fallbackLabel: 'Sandstorm' },
    { theme: 'papyrus', icon: 'theme-papyrus', labelKey: 'chat.theme_papyrus', fallbackLabel: 'Papyrus' },
    { theme: 'threedee', icon: 'theme-threedee', labelKey: 'chat.theme_threedee', fallbackLabel: 'ThreeDee' },
    { theme: 'black-matrix', icon: 'theme-black-matrix', labelKey: 'chat.theme_black_matrix', fallbackLabel: 'Black Matrix' },
];
const CHAT_THEMES = CHAT_THEME_DEFINITIONS.map(def => def.theme);
const DEFAULT_CHAT_THEME = 'dark';
window.AuraChatThemes = CHAT_THEME_DEFINITIONS;

// Debounce lock: prevents double-click from toggling back immediately
let _themeToggleLock = false;

/**
 * Set the active chat theme by name.
 * @param {string} theme - One of CHAT_THEMES
 */
function setChatTheme(theme) {
    if (!CHAT_THEMES.includes(theme)) {
        console.warn('[AuraGo] Unknown chat theme:', theme);
        return;
    }
    const html = document.documentElement;
    const current = html.getAttribute('data-theme') || DEFAULT_CHAT_THEME;
    if (current === theme) return;

    html.setAttribute('data-theme', theme);
    localStorage.setItem('aurago-theme', theme);
    _updateHljsTheme(theme);
    _updateThemeColor(theme);
    if (window.AuraChatThemeEffects && typeof window.AuraChatThemeEffects.ensure === 'function') {
        window.AuraChatThemeEffects.ensure(theme).catch(err => {
            console.warn('[AuraGo] Failed to load chat theme assets:', theme, err);
        });
    }

    // Notify other components (e.g. charts) that the theme changed
    try {
        window.dispatchEvent(new CustomEvent('aurago:themechange', { detail: { theme: theme } }));
    } catch (_) { }
}

/**
 * Cycle to the next theme in CHAT_THEMES.
 * Legacy wrapper for the old binary toggle behavior.
 */
function toggleTheme() {
    if (_themeToggleLock) return;
    _themeToggleLock = true;
    setTimeout(function () { _themeToggleLock = false; }, 400);

    const current = document.documentElement.getAttribute('data-theme') || DEFAULT_CHAT_THEME;
    const idx = CHAT_THEMES.indexOf(current);
    const next = CHAT_THEMES[(idx + 1) % CHAT_THEMES.length];
    setChatTheme(next);
}

/**
 * Get the current chat theme name.
 * @returns {string}
 */
function getCurrentChatTheme() {
    return document.documentElement.getAttribute('data-theme') || DEFAULT_CHAT_THEME;
}

/**
 * Swap highlight.js theme stylesheet based on active theme.
 * dark/retro-crt/cyberwar/dark-sun/ocean/sandstorm/threedee/black-matrix -> github-dark, light/lollipop/papyrus -> github with CSS enhancement layers
 */
function _updateHljsTheme(theme) {
    var link = document.getElementById('hljs-theme');
    if (!link) return;
    var base = '/css/hljs-';
    // Light-leaning themes use the light base; deeper customization is handled in CSS.
    if (theme === 'light' || theme === 'lollipop' || theme === 'papyrus') {
        link.href = base + 'github.min.css';
    } else {
        link.href = base + 'github-dark.min.css';
    }
}

/**
 * Update the <meta name="theme-color"> tag to match the active theme.
 * This ensures mobile browsers (address bar, task switcher) reflect the correct color.
 */
var THEME_COLORS = {
    'dark': '#111827',
    'light': '#d8e4df',
    'retro-crt': '#0b0f1a',
    'cyberwar': '#071128',
    'lollipop': '#fff8fd',
    'dark-sun': '#140b09',
    'ocean': '#051a24',
    'sandstorm': '#1d140d',
    'papyrus': '#bca784',
    'threedee': '#070b14',
    'black-matrix': '#030404',
    '8bit': '#1a252f'
};

function ensure8BitChatThemeOption() {
    const dropdown = document.getElementById('chat-theme-dropdown');
    if (!dropdown) return;

    let option = dropdown.querySelector('.chat-theme-option[data-theme="8bit"]');
    if (!option) {
        option = document.createElement('button');
        option.type = 'button';
        option.className = 'chat-theme-option';
        option.dataset.theme = '8bit';
        option.setAttribute('role', 'option');

        const icon = document.createElement('span');
        icon.className = 'chat-theme-option-icon';
        icon.dataset.chatIcon = 'theme-8bit';

        const label = document.createElement('span');
        label.className = 'chat-theme-option-label';
        label.dataset.i18n = 'chat.theme_8bit';
        label.textContent = t('chat.theme_8bit') === 'chat.theme_8bit' ? '8Bit' : t('chat.theme_8bit');

        option.append(icon, label);
    }

    const anchor = dropdown.querySelector('.chat-theme-option[data-theme="retro-crt"]');
    if (anchor && anchor.nextElementSibling !== option) {
        anchor.after(option);
    } else if (!option.parentElement) {
        dropdown.appendChild(option);
    }

    if (window.AuraChatIcons) window.AuraChatIcons.hydrate(option);
}

function _updateThemeColor(theme) {
    var meta = document.querySelector('meta[name="theme-color"]');
    if (!meta) return;
    var color = THEME_COLORS[theme] || THEME_COLORS[DEFAULT_CHAT_THEME];
    if (color) meta.setAttribute('content', color);
}

/**
 * Initialize theme from localStorage on page load.
 */
function initTheme() {
    if (window._themeInitialized) return;
    window._themeInitialized = true;
    const saved = localStorage.getItem('aurago-theme');
    const theme = (saved && CHAT_THEMES.includes(saved)) ? saved : DEFAULT_CHAT_THEME;
    document.documentElement.setAttribute('data-theme', theme);
    _updateHljsTheme(theme);
    _updateThemeColor(theme);
}


/**
 * Initialize theme toggle button
 */
function initThemeToggle() {
    const themeToggleBtn = document.getElementById('theme-toggle');
    if (!themeToggleBtn) {
        return;
    }
    if (themeToggleBtn.dataset.initialized === 'true') return;
    themeToggleBtn.dataset.initialized = 'true';
    themeToggleBtn.addEventListener('click', toggleTheme);
    console.log('[AuraGo] Theme toggle initialized');
}
