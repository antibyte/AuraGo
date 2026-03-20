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

/**
 * Apply translations to all elements with data-i18n attributes.
 * Supports data-i18n-attr to set a specific attribute instead of textContent.
 * Only updates if a translation exists (does not overwrite with the raw key).
 */
function applyI18n() {
    const dict = typeof I18N !== 'undefined' ? I18N : null;
    if (!dict) return; // I18N not loaded yet — skip
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
    // Also handle data-i18n-placeholder, data-i18n-title, data-i18n-aria-label
    document.querySelectorAll('[data-i18n-placeholder]').forEach(el => {
        const v = dict[el.getAttribute('data-i18n-placeholder')];
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
    const pages = [
        { href: '/', icon: '💬', key: 'common.nav_chat' },
        { href: '/dashboard', icon: '📊', key: 'common.nav_dashboard' },
        { href: '/missions', icon: '🚀', key: 'common.nav_missions' },
        { href: '/cheatsheets', icon: '📋', key: 'common.nav_cheatsheets' },
        { href: '/media', icon: '📁', key: 'common.nav_media' },
        { href: '/config', icon: '⚙️', key: 'common.nav_config' },
        { href: '/invasion', icon: '🥚', key: 'common.nav_invasion' },
    ];

    const items = pages.map(p => {
        const active = (p.href === '/' && path === '/') ||
            (p.href !== '/' && path.startsWith(p.href)) ? ' active' : '';
        return `<a href="${p.href}" class="radial-item${active}"><span class="radial-item-label" data-i18n="${p.key}">${t(p.key)}</span><span class="radial-item-icon">${p.icon}</span></a>`;
    }).join('\n            ');

    anchor.innerHTML = `
    <nav class="radial-menu" id="radialMenu">
        <button class="radial-trigger" id="radialTrigger" aria-label="${t('common.nav_aria_label') || 'Navigation'}">
            <div class="radial-icon"><span></span><span></span><span></span></div>
        </button>
        <div class="radial-items">
            ${items}
            <a id="radialLogout" href="/auth/logout" class="radial-item" style="display:none;"><span class="radial-item-label" data-i18n="common.nav_logout">${t('common.nav_logout')}</span><span class="radial-item-icon">🔓</span></a>
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

/**
 * Toggle between dark and light theme
 */
function toggleTheme() {
    const html = document.documentElement;
    const current = html.getAttribute('data-theme');
    const next = current === 'dark' ? 'light' : 'dark';
    html.setAttribute('data-theme', next);
    localStorage.setItem('aurago-theme', next);
}

/**
 * Initialize theme from localStorage on page load
 */
function initTheme() {
    if (window._themeInitialized) return;
    window._themeInitialized = true;
    const saved = localStorage.getItem('aurago-theme');
    if (saved) {
        document.documentElement.setAttribute('data-theme', saved);
    }
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
                    radialLogout.style.display = '';
                }
                // Show header logout if exists
                const headerLogout = document.getElementById('logout-btn');
                if (headerLogout) {
                    headerLogout.style.display = '';
                }
            }
        }
    } catch (e) {
        // Auth check failed, ignore
    }
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
 * Format a date relative to now
 * @param {string|Date} date
 * @returns {string}
 */
function timeAgo(date) {
    const now = new Date();
    const then = new Date(date);
    const seconds = Math.floor((now - then) / 1000);

    if (seconds < 60) return 'just now';
    if (seconds < 3600) return Math.floor(seconds / 60) + 'm ago';
    if (seconds < 86400) return Math.floor(seconds / 3600) + 'h ago';
    return Math.floor(seconds / 86400) + 'd ago';
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
        showToast('Copied to clipboard', 'success');
        return true;
    } catch (err) {
        showToast('Failed to copy', 'error');
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
function injectLanguageSwitcher() {
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
                showToast('Failed to update UI language: ' + err, 'error');
            }
        } catch (err) {
            console.error('Failed to update UI language:', err);
            showToast('Failed to connect to server', 'error');
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
    // 1. Inject Manifest
    if (!document.querySelector('link[rel="manifest"]')) {
        const link = document.createElement('link');
        link.rel = 'manifest';
        link.href = '/manifest.json';
        document.head.appendChild(link);
    }

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

    window.getPushStatus = function () {
        const permission = Notification.permission; // 'granted' | 'denied' | 'default'
        return { available: true, permission };
    };

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
        banner.style.cssText = [
            'position:fixed', 'top:0', 'left:0', 'right:0', 'z-index:99999',
            'background:#7c3300', 'border-bottom:2px solid #f9a825',
            'color:#ffe082', 'font-size:0.85rem', 'font-weight:600',
            'padding:0.55rem 1rem', 'display:flex', 'align-items:center',
            'gap:0.75rem', 'box-shadow:0 2px 12px rgba(0,0,0,0.5)'
        ].join(';');
        const label = t('config.tailscale.tsnet_needs_login') !== 'config.tailscale.tsnet_needs_login'
            ? '🔐 ' + t('config.tailscale.tsnet_needs_login')
            : '🔐 Tailscale: Authentication required — open the link to connect to your Tailscale network';
        const linkText = t('shared.tsnet.login_banner_link') !== 'shared.tsnet.login_banner_link'
            ? t('shared.tsnet.login_banner_link')
            : 'Open login link';
        banner.innerHTML =
            '<span>' + label + '</span>' +
            '<a href="' + loginUrl + '" target="_blank" rel="noopener noreferrer" ' +
            'style="color:#fff;text-decoration:underline;">' + linkText + '</a>' +
            '<span style="margin-left:auto;cursor:pointer;font-size:1rem;opacity:0.7;" ' +
            'title="Dismiss" onclick="document.getElementById(\'' + _tsnetBannerId() + '\').remove();window._tsnetBannerDismissed=true;">✕</span>';
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
            const resp = await fetch('/api/tsnet/status', { signal: AbortSignal.timeout(5000) });
            if (!resp.ok) return; // server not ready / not authenticated yet
            const data = await resp.json();
            if (data.login_url) {
                if (!window._tsnetBannerDismissed || data.login_url !== _tsnetBannerUrl) {
                    window._tsnetBannerDismissed = false;
                    _tsnetBannerUrl = data.login_url;
                    _tsnetShowBanner(data.login_url);
                }
            } else {
                _tsnetBannerUrl = null;
                window._tsnetBannerDismissed = false;
                _tsnetHideBanner();
            }
        } catch (_) {
            // Silently ignore network errors (server may be offline)
        }
    }

    window.initTsnetLoginWatcher = function () {
        // Initial check after a short delay (let auth complete first)
        setTimeout(_tsnetPoll, 2000);
        // Re-check every 60 seconds
        if (_tsnetPollTimer) clearInterval(_tsnetPollTimer);
        _tsnetPollTimer = setInterval(_tsnetPoll, 60000);
    };
}());

/**
 * Initialize all shared functionality
 */
function initShared() {
    console.log('[AuraGo] Initializing shared components...');

    try { initTheme(); } catch (e) { console.error('[AuraGo] initTheme failed:', e); }
    try { injectRadialMenu(); } catch (e) { console.error('[AuraGo] injectRadialMenu failed:', e); }
    try { initRadialMenu(); } catch (e) { console.error('[AuraGo] initRadialMenu failed:', e); }
    try { initModals(); } catch (e) { console.error('[AuraGo] initModals failed:', e); }
    try { initToggles(); } catch (e) { console.error('[AuraGo] initToggles failed:', e); }
    try { initThemeToggle(); } catch (e) { console.error('[AuraGo] initThemeToggle failed:', e); }
    try { applyI18n(); } catch (e) { console.error('[AuraGo] applyI18n failed:', e); }
    try { injectLanguageSwitcher(); } catch (e) { console.error('[AuraGo] injectLanguageSwitcher failed:', e); }
    try { checkAuth(); } catch (e) { console.error('[AuraGo] checkAuth failed:', e); }
    try { initPWA(); } catch (e) { console.error('[AuraGo] initPWA failed:', e); }
    try { initTsnetLoginWatcher(); } catch (e) { console.error('[AuraGo] initTsnetLoginWatcher failed:', e); }

    console.log('[AuraGo] Shared components initialized');
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
