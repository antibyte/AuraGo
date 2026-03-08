/**
 * AuraGo - Shared JavaScript
 * Common functionality for all pages
 */

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
 * @param {string} id - The modal overlay ID
 */
function openModal(id) {
    const modal = document.getElementById(id);
    if (modal) {
        modal.classList.add('active');
        document.body.style.overflow = 'hidden';
    }
}

/**
 * Close a modal by ID
 * @param {string} id - The modal overlay ID
 */
function closeModal(id) {
    const modal = document.getElementById(id);
    if (modal) {
        modal.classList.remove('active');
        document.body.style.overflow = '';
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
                overlay.classList.remove('active');
                document.body.style.overflow = '';
            }
        });
    });

    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') {
            document.querySelectorAll('.modal-overlay.active').forEach(modal => {
                modal.classList.remove('active');
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

/**
 * Initialize all shared functionality
 */
function initShared() {
    console.log('[AuraGo] Initializing shared components...');
    
    try { initTheme(); } catch (e) { console.error('[AuraGo] initTheme failed:', e); }
    try { initRadialMenu(); } catch (e) { console.error('[AuraGo] initRadialMenu failed:', e); }
    try { initModals(); } catch (e) { console.error('[AuraGo] initModals failed:', e); }
    try { initThemeToggle(); } catch (e) { console.error('[AuraGo] initThemeToggle failed:', e); }
    try { checkAuth(); } catch (e) { console.error('[AuraGo] checkAuth failed:', e); }
    
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
