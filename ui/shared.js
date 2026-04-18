/**
 * AuraGo - Shared JavaScript
 * Common functionality for all pages
 */

// ═══════════════════════════════════════════════════════════════
// I18N (INTERNATIONALIZATION)
// ═══════════════════════════════════════════════════════════════

/**
 * Translate a key using the page's I18N dictionary.
 * Each page must define `const I18N = ...` before loading shared.js.
 * Supports {{placeholder}} interpolation.
 * @param {string} k - The translation key
 * @param {Object} [p] - Optional placeholder map
 * @returns {string}
 */
function t(k, p) {
    const dict = typeof I18N !== 'undefined' ? I18N : null;
    let s = (dict && dict[k]) || k;
    if (p) Object.entries(p).forEach(([a, b]) => s = s.replaceAll('{{' + a + '}}', b));
    return s;
}

// ═══════════════════════════════════════════════════════════════
// MODAL DIALOGS (replaces alert/confirm)
// ═══════════════════════════════════════════════════════════════

let _sharedModalOverlay = null;

function _ensureSharedModal() {
    if (_sharedModalOverlay) return _sharedModalOverlay;
    
    // Check if page already has a modal-overlay (like index.html)
    _sharedModalOverlay = document.getElementById('modal-overlay');
    if (_sharedModalOverlay) return _sharedModalOverlay;
    
    // Create generic modal dynamically
    const modalHTML = `
        <div id="shared-modal-overlay" class="modal-overlay" style="display:none;position:fixed;top:0;left:0;right:0;bottom:0;background:rgba(0,0,0,0.7);z-index:9999;align-items:center;justify-content:center;">
            <div class="modal-card" style="background:var(--bg-secondary,#1a1a1a);border:1px solid var(--border,#2a2a2a);border-radius:12px;padding:24px;max-width:400px;width:90%;box-shadow:0 25px 50px -12px rgba(0,0,0,0.5);">
                <div id="shared-modal-title" class="modal-title" style="font-size:1.125rem;font-weight:600;margin-bottom:12px;color:var(--text-primary,#e5e5e5);"></div>
                <div id="shared-modal-message" class="modal-body" style="margin-bottom:20px;color:var(--text-secondary,#a1a1aa);line-height:1.5;"></div>
                <div class="modal-actions" style="display:flex;gap:12px;justify-content:flex-end;">
                    <button id="shared-modal-cancel" class="modal-btn cancel" style="padding:8px 16px;border-radius:6px;border:1px solid var(--border,#2a2a2a);background:transparent;color:var(--text-secondary,#a1a1aa);cursor:pointer;"></button>
                    <button id="shared-modal-confirm" class="modal-btn confirm" style="padding:8px 16px;border-radius:6px;border:none;background:var(--accent,#2dd4bf);color:#000;cursor:pointer;font-weight:500;"></button>
                </div>
            </div>
        </div>
    `;
    document.body.insertAdjacentHTML('beforeend', modalHTML);
    _sharedModalOverlay = document.getElementById('shared-modal-overlay');
    return _sharedModalOverlay;
}

/**
 * Show a modal dialog (Promise-based, replaces alert/confirm)
 * @param {string} title - Modal title
 * @param {string} message - Modal message
 * @param {boolean} isConfirm - If true, shows cancel button
 * @param {Object} options - Optional { confirmText, cancelText }
 * @returns {Promise<boolean>} - Resolves with true (confirmed) or false (cancelled)
 */
function showModal(title, message, isConfirm = false, options = {}) {
    return new Promise((resolve) => {
        const overlay = _ensureSharedModal();
        const titleEl = document.getElementById('shared-modal-title') || document.getElementById('modal-title');
        const msgEl = document.getElementById('shared-modal-message') || document.getElementById('modal-message');
        const confirmBtn = document.getElementById('shared-modal-confirm') || document.getElementById('modal-confirm');
        const cancelBtn = document.getElementById('shared-modal-cancel') || document.getElementById('modal-cancel');
        
        if (titleEl) titleEl.textContent = title;
        if (msgEl) msgEl.textContent = message;
        if (confirmBtn) confirmBtn.textContent = options.confirmText || t('common.btn_ok') || 'OK';
        if (cancelBtn) {
            cancelBtn.textContent = options.cancelText || t('common.btn_cancel') || 'Cancel';
            cancelBtn.style.display = isConfirm ? 'inline-block' : 'none';
        }
        
        overlay.style.display = 'flex';
        if (overlay.classList) overlay.classList.add('active');
        
        function cleanup(result) {
            overlay.style.display = 'none';
            if (overlay.classList) overlay.classList.remove('active');
            if (confirmBtn) confirmBtn.removeEventListener('click', onConfirm);
            if (cancelBtn) cancelBtn.removeEventListener('click', onCancel);
            overlay.removeEventListener('click', onOverlay);
            document.removeEventListener('keydown', onKey);
            resolve(result);
        }
        
        function onConfirm() { cleanup(true); }
        function onCancel() { cleanup(false); }
        function onOverlay(e) { if (e.target === overlay) cleanup(false); }
        function onKey(e) {
            if (e.key === 'Escape') cleanup(false);
            if (e.key === 'Enter') cleanup(true);
        }
        
        if (confirmBtn) confirmBtn.addEventListener('click', onConfirm);
        if (cancelBtn) cancelBtn.addEventListener('click', onCancel);
        overlay.addEventListener('click', onOverlay);
        document.addEventListener('keydown', onKey);
    });
}

/**
 * Show confirmation dialog (replaces confirm())
 * @param {string} message - Confirmation message
 * @param {string} [title] - Optional title
 * @returns {Promise<boolean>}
 */
function showConfirm(title, message) {
    if (arguments.length === 1) {
        message = title;
        title = t('common.confirm_title') || 'Confirm';
    }
    return showModal(title, message, true, { 
        confirmText: t('common.btn_yes') || 'Yes', 
        cancelText: t('common.btn_no') || 'No' 
    });
}

/**
 * Show alert dialog (replaces alert())
 * @param {string} message - Alert message
 * @param {string} [title] - Optional title
 * @returns {Promise<void>}
 */
function showAlert(title, message) {
    if (arguments.length === 1) {
        message = title;
        title = t('common.alert_title') || 'Notice';
    }
    return showModal(title, message, false, { confirmText: t('common.btn_ok') || 'OK' });
}

function ensureHeadAsset(tagName, attrs) {
    const selectorParts = [tagName];
    if (attrs.rel) selectorParts.push(`[rel="${attrs.rel}"]`);
    if (attrs.href) selectorParts.push(`[href="${attrs.href}"]`);
    if (attrs.name) selectorParts.push(`[name="${attrs.name}"]`);
    if (attrs.sizes) selectorParts.push(`[sizes="${attrs.sizes}"]`);
    const selector = selectorParts.join('');
    if (document.head.querySelector(selector)) return;

    const el = document.createElement(tagName);
    Object.entries(attrs).forEach(([key, value]) => {
        if (value !== undefined && value !== null) {
            el.setAttribute(key, value);
        }
    });
    document.head.appendChild(el);
}

function ensureBrandIcons() {
    ensureHeadAsset('link', {
        rel: 'apple-touch-icon',
        sizes: '180x180',
        href: '/apple-touch-icon.png'
    });
    ensureHeadAsset('link', {
        rel: 'icon',
        type: 'image/png',
        sizes: '32x32',
        href: '/favicon-32x32.png'
    });
    ensureHeadAsset('link', {
        rel: 'icon',
        type: 'image/png',
        sizes: '16x16',
        href: '/favicon-16x16.png'
    });
    ensureHeadAsset('link', {
        rel: 'shortcut icon',
        href: '/favicon.ico'
    });
    ensureHeadAsset('link', {
        rel: 'manifest',
        href: '/manifest.json'
    });
    ensureHeadAsset('meta', {
        name: 'theme-color',
        content: '#111827'
    });
}

function _applySharedPageTitle(dict) {
    const root = document.documentElement;
    if (!root || !dict) return;

    const exactKey = root.getAttribute('data-i18n-page-title');
    const sectionKey = root.getAttribute('data-i18n-page-section');
    if (exactKey) {
        const translated = dict[exactKey];
        if (translated) document.title = translated;
        return;
    }
    if (sectionKey) {
        const translated = dict[sectionKey];
        if (translated) document.title = `AuraGo – ${translated}`;
    }
}

/**
 * Apply translations to all elements with data-i18n attributes.
 * Supports data-i18n-attr to set a specific attribute instead of textContent.
 * Only updates if a translation exists (does not overwrite with the raw key).
 */
function _applySharedI18nBase() {
    const dict = typeof I18N !== 'undefined' ? I18N : null;
    if (!dict) return; // I18N not loaded yet — skip
    _applySharedPageTitle(dict);
    document.querySelectorAll('[data-i18n]').forEach(el => {
        const key = el.getAttribute('data-i18n');
        const translated = dict[key];
        if (!translated) return; // no translation → keep existing text
        const attr = el.getAttribute('data-i18n-attr');
        if (attr) {
            el.setAttribute(attr, translated);
        } else {
            el.textContent = translated;
        }
    });
    document.querySelectorAll('[data-i18n-html]').forEach(el => {
        const translated = dict[el.getAttribute('data-i18n-html')];
        if (translated) el.innerHTML = translated.replace(/\n/g, '<br>');
    });
    // Also handle data-i18n-placeholder, data-i18n-title, data-i18n-aria-label.
    // Keep data-i18n-ph as a temporary compatibility alias during refactors.
    document.querySelectorAll('[data-i18n-placeholder]').forEach(el => {
        const v = dict[el.getAttribute('data-i18n-placeholder')];
        if (v) el.placeholder = v;
    });
    document.querySelectorAll('[data-i18n-ph]').forEach(el => {
        const v = dict[el.getAttribute('data-i18n-ph')];
        if (v) el.placeholder = v;
    });
    document.querySelectorAll('[data-i18n-title]').forEach(el => {
        const v = dict[el.getAttribute('data-i18n-title')];
        if (v) el.title = v;
    });
    document.querySelectorAll('[data-i18n-aria-label]').forEach(el => {
        const v = dict[el.getAttribute('data-i18n-aria-label')];
        if (v) el.setAttribute('aria-label', v);
    });
}

function applyI18n() {
    _applySharedI18nBase();
}

window._auragoApplySharedI18n = _applySharedI18nBase;

// ═══════════════════════════════════════════════════════════════
// RADIAL NAVIGATION MENU INJECTION
// ═══════════════════════════════════════════════════════════════

/**
 * Inject the radial navigation menu HTML into the page.
 * Place a <div id="radialMenuAnchor"></div> where the menu should appear.
 * Automatically detects the current page and marks it as active.
 */
function injectRadialMenu() {
    const anchor = document.getElementById('radialMenuAnchor');
    if (!anchor) return;

    // Detect current page from URL
    const path = window.location.pathname;
    const allPages = [
        { href: '/', icon: '💬', key: 'common.nav_chat' },
        { href: '/dashboard', icon: '📊', key: 'common.nav_dashboard' },
        { href: '/plans', icon: '🗺️', key: 'common.nav_plans' },
        { href: '/missions', icon: '🚀', key: 'common.nav_missions' },
        { href: '/cheatsheets', icon: '📋', key: 'common.nav_cheatsheets' },
        { href: '/media', icon: '📁', key: 'common.nav_media' },
        { href: '/knowledge', icon: '📚', key: 'common.nav_knowledge' },
        { href: '/containers', icon: '🐳', key: 'common.nav_containers' },
        { href: '/skills', icon: '🧩', key: 'common.nav_skills' },
        { href: '/truenas', icon: '🖥️', key: 'common.nav_truenas' },
        { href: '/config', icon: '⚙️', key: 'common.nav_config' },
        { href: '/invasion', icon: '🥚', key: 'common.nav_invasion' },
    ];
    const hiddenRadialPages = new Set(['/plans', '/truenas']);
    const pages = allPages.filter((page) => !hiddenRadialPages.has(page.href));

    const totalItems = pages.length + 1;
    const renderItem = (p, index) => {
        const active = (p.href === '/' && path === '/') ||
            (p.href !== '/' && path.startsWith(p.href)) ? ' active' : '';
        const itemIndex = index + 1;
        const openDelay = (itemIndex * 0.03).toFixed(2);
        const closeDelay = ((totalItems - itemIndex + 1) * 0.03).toFixed(2);
        return `<a href="${p.href}" class="radial-item${active}" style="--radial-index:${itemIndex};--radial-open-delay:${openDelay}s;--radial-close-delay:${closeDelay}s"><span class="radial-item-label" data-i18n="${p.key}">${t(p.key)}</span><span class="radial-item-icon">${p.icon}</span></a>`;
    };
    const items = pages.map((p, index) => renderItem(p, index)).join('\n            ');
    const logoutIndex = totalItems;
    const logoutOpenDelay = (logoutIndex * 0.03).toFixed(2);
    const logoutCloseDelay = (0.03).toFixed(2);

    anchor.innerHTML = `
    <nav class="radial-menu" id="radialMenu">
        <button class="radial-trigger" id="radialTrigger" aria-label="${t('common.nav_aria_label') || 'Navigation'}">
            <div class="radial-icon"><span></span><span></span><span></span></div>
        </button>
        <div class="radial-items">
            ${items}
            <button id="radialLogout" type="button" class="radial-item radial-item-button is-hidden" data-logout-action="true" onclick="performLogout(); return false;" ontouchend="event.preventDefault(); performLogout(); return false;" style="--radial-index:${logoutIndex};--radial-open-delay:${logoutOpenDelay}s;--radial-close-delay:${logoutCloseDelay}s"><span class="radial-item-label" data-i18n="common.nav_logout">${t('common.nav_logout')}</span><span class="radial-item-icon">🔓</span></button>
        </div>
    </nav>
    <div class="radial-backdrop" id="radialBackdrop"></div>`;

    // Re-initialize radial menu events for the new DOM elements
    const trigger = document.getElementById('radialTrigger');
    if (trigger) trigger.dataset.initialized = '';
    initRadialMenu();
}

// ═══════════════════════════════════════════════════════════════
// THEME MANAGEMENT
// ═══════════════════════════════════════════════════════════════

// Supported chat themes: 'dark' (standard), 'light', 'retro-crt', 'cyberwar'
const CHAT_THEMES = ['dark', 'light', 'retro-crt', 'cyberwar'];
const DEFAULT_CHAT_THEME = 'dark';

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
 * dark → github-dark, light → github, retro-crt/cyberwar → github-dark with CSS enhancement layers
 */
function _updateHljsTheme(theme) {
    var link = document.getElementById('hljs-theme');
    if (!link) return;
    var base = 'https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.9.0/styles/';
    // retro-crt and cyberwar use the dark base; theme-specific coloring is handled in CSS.
    if (theme === 'light') {
        link.href = base + 'github.min.css';
    } else {
        link.href = base + 'github-dark.min.css';
    }
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
}

// ═══════════════════════════════════════════════════════════════
// RADIAL MENU
// ═══════════════════════════════════════════════════════════════

/**
 * Initialize the radial navigation menu
 */
function initRadialMenu() {
    const menu = document.getElementById('radialMenu');
    const trigger = document.getElementById('radialTrigger');
    const backdrop = document.getElementById('radialBackdrop');

    if (!menu || !trigger) {
        console.warn('[AuraGo] Radial menu elements not found');
        return;
    }

    // Prevent double initialization
    if (trigger.dataset.initialized === 'true') {
        console.log('[AuraGo] Radial menu already initialized');
        return;
    }
    trigger.dataset.initialized = 'true';

    function openMenu() {
        menu.classList.add('open');
        if (backdrop) backdrop.classList.add('open');
    }

    function closeMenu() {
        menu.classList.remove('open');
        if (backdrop) backdrop.classList.remove('open');
    }

    trigger.addEventListener('click', (e) => {
        e.stopPropagation();
        if (menu.classList.contains('open')) {
            closeMenu();
        } else {
            openMenu();
        }
    });

    if (backdrop) {
        backdrop.addEventListener('click', closeMenu);
    }

    // Close on Escape key
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape' && menu.classList.contains('open')) {
            closeMenu();
        }
    });

    // Close when clicking outside
    document.addEventListener('click', (e) => {
        if (menu.classList.contains('open') && !menu.contains(e.target)) {
            closeMenu();
        }
    });
}

// ═══════════════════════════════════════════════════════════════
// AUTHENTICATION
// ═══════════════════════════════════════════════════════════════

/**
 * Check auth status and show logout button if enabled
 */
async function checkAuth() {
    try {
        const resp = await fetch('/api/auth/status');
        if (resp.ok) {
            const data = await resp.json();
            if (data.enabled) {
                // Show radial logout
                const radialLogout = document.getElementById('radialLogout');
                if (radialLogout) {
                    radialLogout.classList.remove('is-hidden');
                }
                // Show header logout if exists
                const headerLogout = document.getElementById('logout-btn');
                if (headerLogout) {
                    headerLogout.classList.remove('is-hidden');
                }
            }
        }
    } catch (e) {
        // Auth check failed, ignore
    }
}

function initLogoutLinks() {
    document.querySelectorAll('[data-logout-action="true"]').forEach(link => {
        if (link.dataset.logoutBound === 'true') return;
        link.dataset.logoutBound = 'true';
        const handler = function (e) {
            e.preventDefault();
            e.stopPropagation();
            performLogout();
        };
        link.addEventListener('click', handler);
        link.addEventListener('pointerup', handler);
        link.addEventListener('touchend', handler, { passive: false });
    });
}

async function performLogout() {
    if (window._logoutInProgress) return;
    window._logoutInProgress = true;

    const menu = document.getElementById('radialMenu');
    const backdrop = document.getElementById('radialBackdrop');
    if (menu) menu.classList.remove('open');
    if (backdrop) backdrop.classList.remove('open');

    const fallbackURL = '/auth/logout?ts=' + Date.now();
    const apiLogoutURL = '/api/auth/logout?ts=' + Date.now();
    const controller = typeof AbortController !== 'undefined' ? new AbortController() : null;
    const fallbackTimer = setTimeout(() => {
        if (controller) controller.abort();
        window.location.replace(fallbackURL);
    }, 1800);

    try {
        const resp = await fetch(apiLogoutURL, {
            method: 'POST',
            credentials: 'same-origin',
            cache: 'no-store',
            signal: controller ? controller.signal : undefined,
            headers: {
                'X-Requested-With': 'XMLHttpRequest',
                'Accept': 'application/json'
            }
        });
        if (resp.ok) {
            clearTimeout(fallbackTimer);
            const data = await resp.json().catch(() => ({}));
            const redirect = data.redirect || '/auth/login';
            // Validate redirect: only allow relative paths starting with /
            const allowed = redirect === '/auth/login' || (/^\/[a-zA-Z0-9._~!$&'()*+,;=:@-]*$/).test(redirect);
            window.location.replace(allowed ? redirect : '/auth/login');
            return;
        }
    } catch (_) {
        // Fallback below
    }

    clearTimeout(fallbackTimer);
    window.location.replace(fallbackURL);
}

// ═══════════════════════════════════════════════════════════════
// TOAST NOTIFICATIONS
// ═══════════════════════════════════════════════════════════════

/**
 * Show a toast notification
 * @param {string} message - The message to display
 * @param {string} type - 'success', 'error', or 'warning'
 * @param {number} duration - Duration in milliseconds (default: 3000)
 */
function showToast(message, type = 'success', duration = 3000) {
    // Remove existing toast
    const existing = document.getElementById('toast');
    if (existing) {
        existing.remove();
    }

    const toast = document.createElement('div');
    toast.id = 'toast';
    toast.className = `toast ${type}`;
    toast.textContent = message;
    document.body.appendChild(toast);

    // Trigger animation
    requestAnimationFrame(() => {
        toast.classList.add('show');
    });

    setTimeout(() => {
        toast.classList.remove('show');
        setTimeout(() => toast.remove(), 400);
    }, duration);
}

// ═══════════════════════════════════════════════════════════════
// MODAL UTILITIES
// ═══════════════════════════════════════════════════════════════

/**
 * Open a modal by ID
 * Supports both legacy 'open' class and new 'active' class
 * @param {string} id - The modal overlay ID
 */
function openModal(id) {
    const modal = document.getElementById(id);
    if (modal) {
        modal.classList.add('active');
        modal.classList.add('open');
        document.body.style.overflow = 'hidden';
    }
}

/**
 * Close a modal by ID
 * Supports both legacy 'open' class and new 'active' class
 * @param {string} id - The modal overlay ID
 */
function closeModal(id) {
    const modal = document.getElementById(id);
    if (modal) {
        modal.classList.remove('active');
        modal.classList.remove('open');
        document.body.style.overflow = '';
    }
}

// ═══════════════════════════════════════════════════════════════
// TOGGLE SWITCH UTILITIES
// ═══════════════════════════════════════════════════════════════

/**
 * Initialize toggle switches with checkbox input
 * For use with: <label class="toggle"><input type="checkbox"><span class="slider"></span></label>
 */
function initToggles() {
    document.querySelectorAll('.toggle input[type="checkbox"]').forEach(checkbox => {
        // Ensure the toggle reflects the checkbox state visually
        checkbox.addEventListener('change', function() {
            // The CSS handles the visual state via :checked + .slider
            // This listener is for any additional JS logic
            const event = new CustomEvent('toggleChange', { 
                detail: { checked: this.checked, toggle: this.closest('.toggle') }
            });
            this.closest('.toggle').dispatchEvent(event);
        });
    });
}

/**
 * Get the checked state of a toggle
 * @param {string|Element} toggle - The toggle element or its ID
 * @returns {boolean}
 */
function getToggleState(toggle) {
    const el = typeof toggle === 'string' ? document.getElementById(toggle) : toggle;
    if (!el) return false;
    const checkbox = el.querySelector('input[type="checkbox"]');
    return checkbox ? checkbox.checked : el.classList.contains('on');
}

/**
 * Set the checked state of a toggle
 * @param {string|Element} toggle - The toggle element or its ID
 * @param {boolean} checked
 */
function setToggleState(toggle, checked) {
    const el = typeof toggle === 'string' ? document.getElementById(toggle) : toggle;
    if (!el) return;
    const checkbox = el.querySelector('input[type="checkbox"]');
    if (checkbox) {
        checkbox.checked = checked;
        checkbox.dispatchEvent(new Event('change'));
    } else {
        el.classList.toggle('on', checked);
    }
}

/**
 * Initialize modal close on backdrop click and Escape key
 */
function initModals() {
    if (window._modalsInitialized) return;
    window._modalsInitialized = true;

    document.querySelectorAll('.modal-overlay').forEach(overlay => {
        overlay.addEventListener('click', (e) => {
            if (e.target === overlay) {
                overlay.classList.remove('active', 'open');
                document.body.style.overflow = '';
            }
        });
    });

    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') {
            document.querySelectorAll('.modal-overlay.active, .modal-overlay.open').forEach(modal => {
                modal.classList.remove('active', 'open');
            });
            document.body.style.overflow = '';
        }
    });
}

// ═══════════════════════════════════════════════════════════════
// UTILITY FUNCTIONS
// ═══════════════════════════════════════════════════════════════

/**
 * Escape HTML special characters
 * @param {string} text
 * @returns {string}
 */
function esc(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

/**
 * Escape text for safe attribute usage
 * @param {string} text
 * @returns {string}
 */
function escAttr(text) {
    return esc(text).replace(/'/g, '&#39;').replace(/"/g, '&quot;');
}

/**
 * Escape text for safe usage in JavaScript string literals (e.g., onclick handlers)
 * @param {string} text
 * @returns {string}
 */
function escJs(text) {
    if (!text) return '';
    return String(text)
        .replace(/\\/g, '\\\\')
        .replace(/'/g, "\\'")
        .replace(/"/g, '\\"')
        .replace(/</g, '\\x3c')
        .replace(/>/g, '\\x3e');
}

/**
 * Format a date relative to now
 * @param {string|Date} date
 * @returns {string}
 */
function timeAgo(date) {
    const now = new Date();
    const then = new Date(date);
    const seconds = Math.floor((now - then) / 1000);

    if (seconds < 60) return t('common.time_ago_just_now') || 'just now';
    if (seconds < 3600) return t('common.time_ago_minutes', {n: Math.floor(seconds / 60)}) || (Math.floor(seconds / 60) + 'm ago');
    if (seconds < 86400) return t('common.time_ago_hours', {n: Math.floor(seconds / 3600)}) || (Math.floor(seconds / 3600) + 'h ago');
    return t('common.time_ago_days', {n: Math.floor(seconds / 86400)}) || (Math.floor(seconds / 86400) + 'd ago');
}

/**
 * Debounce a function
 * @param {Function} fn
 * @param {number} delay
 * @returns {Function}
 */
function debounce(fn, delay) {
    let timeout;
    return (...args) => {
        clearTimeout(timeout);
        timeout = setTimeout(() => fn(...args), delay);
    };
}

/**
 * Copy text to clipboard
 * @param {string} text
 * @returns {Promise<boolean>}
 */
async function copyToClipboard(text) {
    try {
        await navigator.clipboard.writeText(text);
        showToast(t('common.clipboard_copied') || 'Copied to clipboard', 'success');
        return true;
    } catch (err) {
        showToast(t('common.clipboard_failed') || 'Failed to copy', 'error');
        return false;
    }
}

// ═══════════════════════════════════════════════════════════════
// API HELPERS
// ═══════════════════════════════════════════════════════════════

/**
 * Make an API request with error handling
 * @param {string} url
 * @param {Object} options
 * @returns {Promise<any>}
 */
async function api(url, options = {}) {
    const defaults = {
        headers: {
            'Content-Type': 'application/json',
        },
    };

    const config = { ...defaults, ...options };
    if (options.body && typeof options.body === 'object') {
        config.body = JSON.stringify(options.body);
    }

    const resp = await fetch(url, config);
    if (!resp.ok) {
        const err = await resp.json().catch(() => ({ error: resp.statusText }));
        throw new Error(err.error || resp.statusText);
    }
    return resp.json();
}

// ═══════════════════════════════════════════════════════════════
// INITIALIZATION
// ═══════════════════════════════════════════════════════════════

// ═══════════════════════════════════════════════════════════════
// UI LANGUAGE SWITCHER
// ═══════════════════════════════════════════════════════════════

/**
 * Inject the UI language switcher widget
 */
function shouldShowLanguageSwitcher() {
    const path = window.location.pathname || '';
    return path.includes('/config') || path.includes('/login');
}

function injectLanguageSwitcher() {
    if (!shouldShowLanguageSwitcher()) return;
    if (document.getElementById('ui-lang-switcher')) return;

    const langs = [
        { code: 'en', label: 'English', flag: '🇬🇧' },
        { code: 'de', label: 'Deutsch', flag: '🇩🇪' },
        { code: 'fr', label: 'Français', flag: '🇫🇷' },
        { code: 'es', label: 'Español', flag: '🇪🇸' },
        { code: 'zh', label: '中文', flag: '🇨🇳' },
        { code: 'ja', label: '日本語', flag: '🇯🇵' },
        { code: 'nl', label: 'Nederlands', flag: '🇳🇱' },
        { code: 'pt', label: 'Português', flag: '🇵🇹' },
        { code: 'pl', label: 'Polski', flag: '🇵🇱' },
        { code: 'cs', label: 'Čeština', flag: '🇨🇿' },
        { code: 'it', label: 'Italiano', flag: '🇮🇹' },
        { code: 'sv', label: 'Svenska', flag: '🇸🇪' },
        { code: 'no', label: 'Norsk', flag: '🇳🇴' },
        { code: 'da', label: 'Dansk', flag: '🇩🇰' },
        { code: 'el', label: 'Ελληνικά', flag: '🇬🇷' },
        { code: 'hi', label: 'हिन्दी', flag: '🇮🇳' },
    ];

    const currentLangCode = document.documentElement.lang || 'en';
    const currentLang = langs.find(l => l.code === currentLangCode) || langs[0];

    const container = document.createElement('div');
    container.id = 'ui-lang-switcher';
    container.className = 'ui-lang-switcher';

    container.innerHTML = `
        <button class="ui-lang-btn" id="ui-lang-btn" title="Change UI Language">
            <span class="ui-lang-flag">${currentLang.flag}</span>
        </button>
        <div class="ui-lang-menu" id="ui-lang-menu">
            ${langs.map(l => `
                <button class="ui-lang-option ${l.code === currentLangCode ? 'active' : ''}" data-lang="${l.code}">
                    <span class="ui-lang-flag">${l.flag}</span>
                    <span class="ui-lang-label">${l.label}</span>
                </button>
            `).join('')}
        </div>
    `;

    document.body.appendChild(container);

    const btn = document.getElementById('ui-lang-btn');
    const menu = document.getElementById('ui-lang-menu');

    btn.addEventListener('click', (e) => {
        e.stopPropagation();
        menu.classList.toggle('open');
    });

    document.addEventListener('click', () => {
        menu.classList.remove('open');
    });

    menu.addEventListener('click', async (e) => {
        const option = e.target.closest('.ui-lang-option');
        if (!option) return;

        const lang = option.dataset.lang;
        if (lang === currentLangCode) return;

        try {
            const resp = await fetch('/api/ui-language', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ language: lang })
            });
            if (resp.ok) {
                window.location.reload();
            } else {
                const err = await resp.text();
                showToast((t('common.language_update_failed') || 'Failed to update UI language') + ': ' + err, 'error');
            }
        } catch (err) {
            console.error('Failed to update UI language:', err);
            showToast(t('common.server_connection_failed') || 'Failed to connect to server', 'error');
        }
    });
}

/**
 * Initialize Progressive Web App (PWA) and Push Notifications.
 * Registers the service worker and wires up push subscription helpers,
 * but does NOT prompt the user — that must be triggered by a user gesture
 * (e.g. the bell button in the chat UI).
 */
async function initPWA() {
    // 1. Ensure the manifest and favicon set is present for all pages.
    ensureBrandIcons();

    // Always expose getPushStatus so UI can query it immediately
    window.getPushStatus = function () {
        if (!('serviceWorker' in navigator) || !('PushManager' in window)) {
            return { available: false, permission: 'default', reason: 'not-supported' };
        }
        if (!window._swRegistration) {
            return { available: false, permission: 'default', reason: 'sw-failed' };
        }
        const permission = Notification.permission; // 'granted' | 'denied' | 'default'
        return { available: true, permission };
    };

    if (!('serviceWorker' in navigator) || !('PushManager' in window)) {
        window._pushStatus = { available: false, reason: 'not-supported' };
        return;
    }

    // 2. Register Service Worker
    let registration;
    try {
        registration = await navigator.serviceWorker.register('/sw.js');
        console.log('[PWA] Service Worker registered, scope:', registration.scope);
    } catch (err) {
        console.error('[PWA] Service Worker registration failed:', err);
        window._pushStatus = { available: false, reason: 'sw-failed' };
        return;
    }

    // 3. Expose push status and opt-in helpers on window for use by the chat UI
    window._swRegistration = registration;

    window.requestPushPermission = async function () {
        if (Notification.permission === 'denied') {
            return { success: false, reason: 'denied' };
        }

        let permission = Notification.permission;
        if (permission === 'default') {
            permission = await Notification.requestPermission();
        }

        if (permission !== 'granted') {
            return { success: false, reason: 'denied' };
        }

        return _subscribePush(registration);
    };

    window.revokePushPermission = async function () {
        try {
            const sub = await registration.pushManager.getSubscription();
            if (sub) {
                await fetch('/api/push/unsubscribe', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ endpoint: sub.endpoint })
                });
                await sub.unsubscribe();
                console.log('[PWA] Push subscription removed.');
            }
            return { success: true };
        } catch (err) {
            console.warn('[PWA] Failed to unsubscribe:', err);
            return { success: false };
        }
    };

    // 4. Auto-subscribe silently if permission was already granted
    if (Notification.permission === 'granted') {
        _subscribePush(registration).catch(err =>
            console.warn('[PWA] Silent re-subscribe failed:', err)
        );
    }

    // 5. Signal that PWA is ready (button UI can now update)
    window.dispatchEvent(new CustomEvent('pwa-ready'));
}

/** Internal: subscribe to push and POST subscription to server. */
async function _subscribePush(registration) {
    try {
        const resp = await fetch('/api/push/vapid-pubkey');
        if (!resp.ok) throw new Error('vapid-pubkey fetch failed: ' + resp.status);
        const { public_key } = await resp.json();
        if (!public_key) throw new Error('no public key returned');

        const subscription = await registration.pushManager.subscribe({
            userVisibleOnly: true,
            applicationServerKey: urlBase64ToUint8Array(public_key)
        });

        const postResp = await fetch('/api/push/subscribe', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(subscription)
        });
        if (!postResp.ok) throw new Error('subscribe POST failed: ' + postResp.status);

        console.log('[PWA] Push subscription saved.');
        return { success: true };
    } catch (err) {
        console.warn('[PWA] Push subscription failed:', err);
        return { success: false, reason: err.message };
    }
}

// Utility function to convert VAPID Key
function urlBase64ToUint8Array(base64String) {
    const padding = '='.repeat((4 - base64String.length % 4) % 4);
    const base64 = (base64String + padding).replace(/\-/g, '+').replace(/_/g, '/');
    const rawData = window.atob(base64);
    const outputArray = new Uint8Array(rawData.length);
    for (let i = 0; i < rawData.length; ++i) {
        outputArray[i] = rawData.charCodeAt(i);
    }
    return outputArray;
}

/**
 * Initialize theme toggle button
 */
function initThemeToggle() {
    const themeToggleBtn = document.getElementById('theme-toggle');
    if (!themeToggleBtn) {
        console.warn('[AuraGo] Theme toggle button not found');
        return;
    }
    if (themeToggleBtn.dataset.initialized === 'true') return;
    themeToggleBtn.dataset.initialized = 'true';
    themeToggleBtn.addEventListener('click', toggleTheme);
    console.log('[AuraGo] Theme toggle initialized');
}

// ═══════════════════════════════════════════════════════════════
// AURA SSE — Shared single EventSource connection
// All pages share one /events connection. Register typed-event
// handlers with AuraSSE.on(type, fn) and legacy {event,detail}
// handlers with AuraSSE.onLegacy(fn). Auto-reconnects on error.
// Special internal types: '_open', '_error'
// ═══════════════════════════════════════════════════════════════
window.AuraSSE = (function () {
    'use strict';
    var _typed = {};
    var _legacy = [];
    var _es = null;
    var _retryTimer = null;
    var _connected = false;

    function _dispatch(e) {
        var data;
        try { data = JSON.parse(e.data); } catch (_) { return; }
        if (typeof data.type === 'string') {
            var handlers = _typed[data.type];
            if (handlers) {
                handlers.slice().forEach(function (fn) {
                    try { fn(data.payload, data); } catch (_) { }
                });
            }
        } else if (typeof data.event === 'string') {
            _legacy.slice().forEach(function (fn) {
                try { fn(e); } catch (_) { }
            });
        }
    }

    function _fireInternal(type, arg) {
        var handlers = _typed[type];
        if (handlers) handlers.slice().forEach(function (fn) { try { fn(arg); } catch (_) { } });
    }

    var _retryAttempt = 0;

    function _nextReconnectDelay() {
        var base = Math.min(30000, 1000 * Math.pow(2, Math.min(_retryAttempt, 5)));
        var jitter = Math.floor(Math.random() * 750);
        _retryAttempt += 1;
        return base + jitter;
    }

    function _connect() {
        var path = window.location.pathname;
        if (path.indexOf('/login') !== -1 || path.indexOf('/setup') !== -1) return;
        if (_es) { _es.close(); _es = null; }
        _es = new EventSource('/events', { withCredentials: true });
        _es.onopen = function () {
            _connected = true;
            _retryAttempt = 0;
            if (_retryTimer) { clearTimeout(_retryTimer); _retryTimer = null; }
            _fireInternal('_open');
        };
        _es.onerror = function () {
            _connected = false;
            _fireInternal('_error', _es ? _es.readyState : -1);
            if (!_retryTimer) {
                var delay = _nextReconnectDelay();
                _retryTimer = setTimeout(function () {
                    _retryTimer = null;
                    _connect();
                }, delay);
            }
        };
        _es.onmessage = _dispatch;
    }

    // Auto-redirect to login on SSE auth failure
    var _authRedirectInProgress = false;
    var _authErrorCount = 0;
    var _authErrorTimer = null;
    function _redirectToLogin() {
        if (_authRedirectInProgress) return;
        if (window.location.pathname.indexOf('/login') !== -1 || window.location.pathname.indexOf('/setup') !== -1) return;
        _authRedirectInProgress = true;
        window.location.href = '/auth/login?redirect=' + encodeURIComponent(window.location.pathname);
    }
    if (!_typed['_error']) _typed['_error'] = [];
    _typed['_error'].push(function () {
        _authErrorCount++;
        if (_authErrorTimer) clearTimeout(_authErrorTimer);
        _authErrorTimer = setTimeout(function () { _authErrorCount = 0; }, 5000);
        // Only redirect after multiple consecutive auth errors (avoids false positives on temporary disconnects)
        if (_authErrorCount < 3) return;
        fetch('/api/auth/status', { credentials: 'same-origin' }).then(function (r) {
            if (r.status === 401) _redirectToLogin();
        }).catch(function () {});
    });

    return {
        connect: _connect,
        isConnected: function () { return _connected; },
        on: function (type, fn) {
            if (!_typed[type]) _typed[type] = [];
            _typed[type].push(fn);
        },
        off: function (type, fn) {
            if (!_typed[type]) return;
            _typed[type] = _typed[type].filter(function (f) { return f !== fn; });
        },
        onLegacy: function (fn) { _legacy.push(fn); },
        offLegacy: function (fn) {
            var i = _legacy.indexOf(fn);
            if (i >= 0) _legacy.splice(i, 1);
        }
    };
}());

// ═══════════════════════════════════════════════════════════════
// TAILSCALE LOGIN WATCHER
// Polls /api/tsnet/status and shows a persistent top banner when
// the tsnet node needs interactive authentication.
// ═══════════════════════════════════════════════════════════════

(function () {
    let _tsnetBannerUrl = null;
    let _tsnetPollTimer = null;

    function _tsnetBannerId() { return 'tsnet-login-banner'; }

    function _tsnetShowBanner(loginUrl) {
        if (document.getElementById(_tsnetBannerId())) {
            // Update link if URL changed
            const a = document.querySelector('#' + _tsnetBannerId() + ' a');
            if (a) a.href = loginUrl;
            return;
        }
        const banner = document.createElement('div');
        banner.id = _tsnetBannerId();
        banner.className = 'tsnet-login-banner';
        const label = t('config.tailscale.tsnet_needs_login') !== 'config.tailscale.tsnet_needs_login'
            ? '🔐 ' + t('config.tailscale.tsnet_needs_login')
            : '🔐 Tailscale: Authentication required — open the link to connect to your Tailscale network';
        const linkText = t('shared.tsnet.login_banner_link') !== 'shared.tsnet.login_banner_link'
            ? t('shared.tsnet.login_banner_link')
            : 'Open login link';
        const labelEl = document.createElement('span');
        labelEl.textContent = label;
        const linkEl = document.createElement('a');
        // Validate loginUrl: only allow https:// URLs
        const allowed = loginUrl && loginUrl.startsWith('https://') && /^https:\/\/[a-zA-Z0-9._~:/?#\[\]@!$&'()*+,;=-]+$/.test(loginUrl);
        linkEl.href = allowed ? loginUrl : '#';
        linkEl.target = '_blank';
        linkEl.rel = 'noopener noreferrer';
        linkEl.className = 'tsnet-login-banner-link';
        linkEl.textContent = linkText;
        const closeEl = document.createElement('button');
        closeEl.type = 'button';
        closeEl.className = 'tsnet-login-banner-close';
        closeEl.title = 'Dismiss';
        closeEl.textContent = '✕';
        closeEl.addEventListener('click', () => {
            document.getElementById(_tsnetBannerId())?.remove();
            window._tsnetBannerDismissed = true;
        });
        banner.appendChild(labelEl);
        banner.appendChild(linkEl);
        banner.appendChild(closeEl);
        document.body.insertBefore(banner, document.body.firstChild);
        // Push body down so banner doesn't overlap content
        document.body.style.paddingTop = 'calc(' + (document.body.style.paddingTop || '0px') + ' + 38px)';
    }

    function _tsnetHideBanner() {
        const el = document.getElementById(_tsnetBannerId());
        if (el) {
            el.remove();
            document.body.style.paddingTop = '';
        }
    }

    async function _tsnetPoll() {
        // Skip on login and setup pages — no auth yet
        const path = window.location.pathname;
        if (path.includes('/login') || path.includes('/setup')) return;

        try {
            const resp = await fetch('/api/tsnet/status', { signal: typeof AbortSignal !== 'undefined' && typeof AbortSignal.timeout === 'function' ? AbortSignal.timeout(8000) : undefined });
            if (!resp.ok) return; // server not ready / not authenticated yet
            const data = await resp.json();
            if (data.login_url) {
                if (!window._tsnetBannerDismissed || data.login_url !== _tsnetBannerUrl) {
                    window._tsnetBannerDismissed = false;
                    _tsnetBannerUrl = data.login_url;
                    _tsnetShowBanner(data.login_url);
                }
                // While waiting for auth, poll more frequently so the banner
                // disappears promptly after the user completes authentication.
                if (_tsnetPollTimer) {
                    clearInterval(_tsnetPollTimer);
                    _tsnetPollTimer = setInterval(_tsnetPoll, 10000);
                }
            } else if (data.starting) {
                // Node is starting but no login_url yet — check again soon.
                if (_tsnetPollTimer) {
                    clearInterval(_tsnetPollTimer);
                    _tsnetPollTimer = setInterval(_tsnetPoll, 5000);
                }
            } else {
                _tsnetBannerUrl = null;
                window._tsnetBannerDismissed = false;
                _tsnetHideBanner();
                // Back to normal slow polling once authenticated / stopped.
                if (_tsnetPollTimer) {
                    clearInterval(_tsnetPollTimer);
                    _tsnetPollTimer = setInterval(_tsnetPoll, 60000);
                }
            }
        } catch (_) {
            // Silently ignore network errors (server may be offline)
        }
    }

    window.initTsnetLoginWatcher = function () {
        // Register SSE handler for real-time tsnet status pushes (no more polling).
        window.AuraSSE.on('tsnet_status', function (payload) {
            if (!payload) return;
            if (payload.login_url) {
                if (!window._tsnetBannerDismissed || payload.login_url !== _tsnetBannerUrl) {
                    window._tsnetBannerDismissed = false;
                    _tsnetBannerUrl = payload.login_url;
                    _tsnetShowBanner(payload.login_url);
                }
            } else {
                _tsnetBannerUrl = null;
                window._tsnetBannerDismissed = false;
                _tsnetHideBanner();
            }
        });
        // Do one immediate check so the banner appears right away on page load
        // without waiting for the first SSE push (~10s server interval).
        setTimeout(function () {
            var path = window.location.pathname;
            if (path.indexOf('/login') !== -1 || path.indexOf('/setup') !== -1) return;
            fetch('/api/tsnet/status', { signal: typeof AbortSignal !== 'undefined' && typeof AbortSignal.timeout === 'function' ? AbortSignal.timeout(5000) : undefined })
                .then(function (r) { return r.ok ? r.json() : null; })
                .then(function (data) {
                    if (!data || !data.login_url) return;
                    if (!window._tsnetBannerDismissed) {
                        _tsnetBannerUrl = data.login_url;
                        _tsnetShowBanner(data.login_url);
                    }
                })
                .catch(function () { });
        }, 1000);
    };}());

/**
 * Initialize all shared functionality
 */
function initShared() {
    console.log('[AuraGo] Initializing shared components...');

    try { ensureBrandIcons(); } catch (e) { console.error('[AuraGo] ensureBrandIcons failed:', e); }
    try { initTheme(); } catch (e) { console.error('[AuraGo] initTheme failed:', e); }
    try { injectRadialMenu(); } catch (e) { console.error('[AuraGo] injectRadialMenu failed:', e); }
    try { initRadialMenu(); } catch (e) { console.error('[AuraGo] initRadialMenu failed:', e); }
    try { initLogoutLinks(); } catch (e) { console.error('[AuraGo] initLogoutLinks failed:', e); }
    try { initModals(); } catch (e) { console.error('[AuraGo] initModals failed:', e); }
    try { initToggles(); } catch (e) { console.error('[AuraGo] initToggles failed:', e); }
    try { initThemeToggle(); } catch (e) { console.error('[AuraGo] initThemeToggle failed:', e); }
    try { window._auragoApplySharedI18n(); } catch (e) { console.error('[AuraGo] applyI18n failed:', e); }
    try { injectLanguageSwitcher(); } catch (e) { console.error('[AuraGo] injectLanguageSwitcher failed:', e); }
    try { checkAuth(); } catch (e) { console.error('[AuraGo] checkAuth failed:', e); }
    try { initPWA(); } catch (e) { console.error('[AuraGo] initPWA failed:', e); }
    try { window.AuraSSE.connect(); } catch (e) { console.error('[AuraGo] AuraSSE.connect failed:', e); }
    try { initTsnetLoginWatcher(); } catch (e) { console.error('[AuraGo] initTsnetLoginWatcher failed:', e); }

    console.log('[AuraGo] Shared components initialized');
}

// ── Config-page shared utilities ─────────────────────

function setHidden(el, hidden) {
    if (!el) return;
    el.classList.toggle('is-hidden', hidden);
}

const CFG_MASKED_SECRET = '••••••••';

function cfgIsMaskedSecret(value) {
    return value === CFG_MASKED_SECRET;
}

function cfgSecretValue(value) {
    return cfgIsMaskedSecret(value) ? '' : (value || '');
}

function cfgSecretPlaceholder(value, defaultPlaceholder = CFG_MASKED_SECRET) {
    return cfgIsMaskedSecret(value) ? t('config.providers.key_placeholder_existing') : defaultPlaceholder;
}

function cfgMarkSecretStored(input, configPath) {
    if (input) {
        input.value = '';
        input.placeholder = t('config.providers.key_placeholder_existing');
    }
    if (!configPath) return;
    const paths = Array.isArray(configPath) ? configPath : [configPath];
    paths.forEach(path => {
        if (path) setNestedValue(configData, path, CFG_MASKED_SECRET);
    });
}

async function vaultSave(key, value, statusEl) {
    const resp = await fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key, value })
    });
    const res = await resp.json();
    if (res.status === 'ok' || res.success) {
        showToast('✓ ' + t('config.common.saved'), 'success');
        if (statusEl) { statusEl.textContent = ''; }
        return true;
    } else {
        const msg = res.message || t('config.common.error');
        showToast(msg, 'error');
        if (statusEl) {
            statusEl.style.color = 'var(--danger)';
            statusEl.textContent = '✗ ' + msg;
        }
        return false;
    }
}

async function cfgFetch(url, options = {}) {
    const resp = await fetch(url, options);
    if (!resp.ok) {
        const text = await resp.text();
        let msg;
        try { msg = JSON.parse(text).message; } catch(_) { msg = text.slice(0, 200); }
        throw new Error(msg || 'HTTP ' + resp.status);
    }
    return resp.json();
}

// Auto-initialize on DOM ready - simple and reliable approach
function scheduleInit() {
    if (document.readyState === 'loading') {
        // DOM not ready yet, wait for it
        document.addEventListener('DOMContentLoaded', initShared);
    } else {
        // DOM is ready, run immediately
        initShared();
    }
}

// Ensure we only initialize once
if (!window._auragoSharedInitialized) {
    window._auragoSharedInitialized = true;
    scheduleInit();
}
