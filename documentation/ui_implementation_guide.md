# AuraGo UI - Implementierungs-Guide

## Schnelle Verbesserungen (Low-Hanging Fruits)

### 1. Navigation-Fix (30 Minuten)

```html
<!-- shared.js - Neue Navigation Funktion -->
function injectTopNavigation() {
    const nav = document.createElement('nav');
    nav.className = 'top-nav';
    nav.innerHTML = `
        <div class="nav-brand">
            <a href="/" class="nav-logo">
                <span class="logo-icon">🤖</span>
                <span class="logo-text">AURA<span>GO</span></span>
            </a>
        </div>
        <div class="nav-links">
            <a href="/" class="nav-link ${isActive('/')} ">
                <span class="nav-icon">💬</span>
                <span class="nav-label">Chat</span>
            </a>
            <a href="/dashboard" class="nav-link ${isActive('/dashboard')}">
                <span class="nav-icon">📊</span>
                <span class="nav-label">Dashboard</span>
            </a>
            <a href="/missions" class="nav-link ${isActive('/missions')}">
                <span class="nav-icon">🚀</span>
                <span class="nav-label">Missions</span>
            </a>
            <a href="/config" class="nav-link ${isActive('/config')}">
                <span class="nav-icon">⚙️</span>
                <span class="nav-label">Config</span>
            </a>
        </div>
        <div class="nav-actions">
            <button id="theme-toggle" class="btn-icon">🌙</button>
            <div class="user-menu">
                <button class="btn-icon">👤</button>
            </div>
        </div>
    `;
    document.body.insertBefore(nav, document.body.firstChild);
}
```

```css
/* shared.css - Top Navigation Styles */
.top-nav {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.75rem 1.5rem;
    background: var(--header-bg);
    backdrop-filter: blur(20px);
    border-bottom: 1px solid var(--border-subtle);
    position: sticky;
    top: 0;
    z-index: 100;
}

.nav-links {
    display: flex;
    gap: 0.5rem;
}

.nav-link {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem 1rem;
    border-radius: 8px;
    color: var(--text-secondary);
    text-decoration: none;
    font-size: 0.9rem;
    font-weight: 500;
    transition: all 0.2s;
}

.nav-link:hover {
    background: var(--accent-dim);
    color: var(--text-primary);
}

.nav-link.active {
    background: var(--accent-dim);
    color: var(--accent);
}

/* Mobile: Collapse to bottom bar */
@media (max-width: 768px) {
    .top-nav {
        position: fixed;
        bottom: 0;
        top: auto;
        border-top: 1px solid var(--border-subtle);
        border-bottom: none;
        padding: 0.5rem;
    }
    
    .nav-brand, .nav-label {
        display: none;
    }
    
    .nav-links {
        width: 100%;
        justify-content: space-around;
    }
    
    .nav-link {
        flex-direction: column;
        padding: 0.5rem;
        font-size: 0.7rem;
    }
    
    .nav-icon {
        font-size: 1.25rem;
    }
}
```

### 2. Config Sidebar Fix (1 Stunde)

```javascript
// config/main.js - Sidebar-Verbesserungen
function buildSidebar() {
    const sb = document.getElementById('sidebar');
    sb.innerHTML = `
        <!-- Search -->
        <div class="sidebar-search">
            <input type="text" 
                   id="sidebar-search" 
                   placeholder="🔍 Einstellungen suchen..."
                   oninput="filterSidebar(this.value)">
        </div>
        
        <!-- Favorites (if any) -->
        <div class="sidebar-section" id="favorites-section" style="display:none">
            <div class="sidebar-section-title">⭐ Favoriten</div>
            <div id="favorites-list"></div>
        </div>
        
        <!-- All Sections -->
        <div id="sidebar-sections"></div>
    `;
    
    // Auto-expand all groups
    renderSidebarSections(true); // true = expanded
}

function renderSidebarSections(expanded = true) {
    const container = document.getElementById('sidebar-sections');
    
    SECTIONS.forEach(group => {
        const groupEl = document.createElement('div');
        groupEl.className = 'sidebar-group';
        groupEl.innerHTML = `
            <div class="sidebar-group-header" onclick="toggleGroup(this)">
                <span class="group-title">${group.group}</span>
                <span class="group-arrow">${expanded ? '▼' : '▶'}</span>
            </div>
            <div class="sidebar-group-content" style="max-height: ${expanded ? 'none' : '0'}">
                ${group.items.map(item => `
                    <div class="sidebar-item ${item.key === activeSection ? 'active' : ''}"
                         data-section="${item.key}"
                         data-keywords="${item.label} ${item.desc}"
                         onclick="selectSection('${item.key}')">
                        <span class="item-icon">${item.icon}</span>
                        <span class="item-label">${item.label}</span>
                        ${isFavorite(item.key) ? '⭐' : ''}
                    </div>
                `).join('')}
            </div>
        `;
        container.appendChild(groupEl);
    });
}

// Filter function
function filterSidebar(query) {
    const items = document.querySelectorAll('.sidebar-item');
    const groups = document.querySelectorAll('.sidebar-group');
    
    const lowerQuery = query.toLowerCase();
    
    items.forEach(item => {
        const keywords = item.dataset.keywords.toLowerCase();
        const match = keywords.includes(lowerQuery);
        item.style.display = match ? '' : 'none';
    });
    
    // Hide empty groups
    groups.forEach(group => {
        const visibleItems = group.querySelectorAll('.sidebar-item:not([style*="none"])');
        group.style.display = visibleItems.length > 0 ? '' : 'none';
    });
}
```

```css
/* config.css - Verbesserte Sidebar */
.sidebar-search {
    padding: 1rem;
    border-bottom: 1px solid var(--border-subtle);
}

.sidebar-search input {
    width: 100%;
    padding: 0.5rem 0.75rem;
    border: 1px solid var(--border-subtle);
    border-radius: 8px;
    background: var(--input-bg);
    color: var(--text-primary);
    font-size: 0.85rem;
}

.sidebar-search input:focus {
    outline: none;
    border-color: var(--accent);
    box-shadow: 0 0 0 3px var(--accent-dim);
}

.sidebar-section-title {
    padding: 0.75rem 1rem 0.5rem;
    font-size: 0.7rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    color: var(--text-secondary);
    opacity: 0.8;
}

.sidebar-group {
    margin-bottom: 0.5rem;
}

.sidebar-group-header {
    padding: 0.6rem 1rem;
    font-weight: 600;
    font-size: 0.85rem;
    color: var(--text-primary);
    cursor: pointer;
    display: flex;
    justify-content: space-between;
    align-items: center;
    transition: background 0.2s;
}

.sidebar-group-header:hover {
    background: var(--accent-dim);
}

.sidebar-group-content {
    overflow: hidden;
    transition: max-height 0.3s ease;
}

.sidebar-item {
    padding: 0.5rem 1rem 0.5rem 2rem;
    font-size: 0.82rem;
    color: var(--text-secondary);
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 0.5rem;
    transition: all 0.2s;
    border-left: 3px solid transparent;
}

.sidebar-item:hover {
    background: var(--accent-dim);
    color: var(--text-primary);
    border-left-color: var(--accent);
}

.sidebar-item.active {
    background: var(--accent-dim);
    color: var(--accent);
    border-left-color: var(--accent);
    font-weight: 500;
}

.item-icon {
    font-size: 1rem;
}
```

### 3. Dashboard Cards Collapsible (30 Minuten)

```javascript
// dashboard/main.js - Collapsible Cards
function initCollapsibleCards() {
    document.querySelectorAll('.dash-card-header').forEach(header => {
        const toggle = document.createElement('span');
        toggle.className = 'collapse-btn';
        toggle.innerHTML = '▼';
        toggle.onclick = (e) => {
            e.stopPropagation();
            const card = header.closest('.dash-card');
            const body = card.querySelector('.dash-card-body');
            const isCollapsed = body.style.display === 'none';
            
            body.style.display = isCollapsed ? '' : 'none';
            toggle.innerHTML = isCollapsed ? '▼' : '▶';
            
            // Save preference
            localStorage.setItem(`card-${card.id}-collapsed`, !isCollapsed);
        };
        header.appendChild(toggle);
    });
}

// Restore collapsed state
function restoreCardStates() {
    document.querySelectorAll('.dash-card').forEach(card => {
        const isCollapsed = localStorage.getItem(`card-${card.id}-collapsed`) === 'true';
        if (isCollapsed) {
            const body = card.querySelector('.dash-card-body');
            const toggle = card.querySelector('.collapse-btn');
            if (body && toggle) {
                body.style.display = 'none';
                toggle.innerHTML = '▶';
            }
        }
    });
}
```

### 4. Loading States (1 Stunde)

```css
/* shared.css - Loading Components */

/* Button Loading */
.btn-loading {
    position: relative;
    color: transparent !important;
    pointer-events: none;
}

.btn-loading::after {
    content: '';
    position: absolute;
    width: 1rem;
    height: 1rem;
    border: 2px solid transparent;
    border-top-color: currentColor;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
    left: 50%;
    top: 50%;
    margin-left: -0.5rem;
    margin-top: -0.5rem;
}

@keyframes spin {
    to { transform: rotate(360deg); }
}

/* Skeleton Loader */
.skeleton {
    background: linear-gradient(
        90deg,
        var(--bg-secondary) 25%,
        var(--bg-glass) 50%,
        var(--bg-secondary) 75%
    );
    background-size: 200% 100%;
    animation: shimmer 1.5s infinite;
    border-radius: 4px;
}

@keyframes shimmer {
    0% { background-position: 200% 0; }
    100% { background-position: -200% 0; }
}

.skeleton-text {
    height: 1em;
    margin-bottom: 0.5em;
}

.skeleton-text:last-child {
    width: 80%;
}

.skeleton-card {
    height: 100px;
    margin-bottom: 1rem;
}

/* Page Loader */
.page-loader {
    position: fixed;
    inset: 0;
    background: var(--bg-primary);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 9999;
}

.page-loader-spinner {
    width: 48px;
    height: 48px;
    border: 3px solid var(--border-subtle);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 1s linear infinite;
}
```

```javascript
// Loading State Helpers
const Loading = {
    button: (btn, loading = true) => {
        if (loading) {
            btn.dataset.originalText = btn.textContent;
            btn.classList.add('btn-loading');
            btn.disabled = true;
        } else {
            btn.classList.remove('btn-loading');
            btn.disabled = false;
        }
    },
    
    show: () => {
        const loader = document.createElement('div');
        loader.className = 'page-loader';
        loader.id = 'page-loader';
        loader.innerHTML = '<div class="page-loader-spinner"></div>';
        document.body.appendChild(loader);
    },
    
    hide: () => {
        const loader = document.getElementById('page-loader');
        if (loader) loader.remove();
    },
    
    skeleton: (container, count = 3) => {
        container.innerHTML = Array(count).fill(`
            <div class="skeleton skeleton-card">
                <div class="skeleton skeleton-text"></div>
                <div class="skeleton skeleton-text"></div>
            </div>
        `).join('');
    }
};
```

### 5. Toast Notifications (30 Minuten)

```css
/* shared.css - Toast System */
.toast-container {
    position: fixed;
    top: 1rem;
    right: 1rem;
    z-index: 9999;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
}

.toast {
    padding: 1rem 1.25rem;
    border-radius: 10px;
    background: var(--surface);
    border: 1px solid var(--border-subtle);
    box-shadow: 0 10px 30px rgba(0,0,0,0.2);
    display: flex;
    align-items: center;
    gap: 0.75rem;
    min-width: 300px;
    max-width: 400px;
    animation: slideIn 0.3s ease;
    backdrop-filter: blur(10px);
}

.toast.success {
    border-left: 4px solid var(--color-success);
}

.toast.error {
    border-left: 4px solid var(--color-error);
}

.toast.warning {
    border-left: 4px solid var(--color-warning);
}

.toast.info {
    border-left: 4px solid var(--color-info);
}

.toast-icon {
    font-size: 1.25rem;
}

.toast-content {
    flex: 1;
}

.toast-title {
    font-weight: 600;
    font-size: 0.9rem;
    color: var(--text-primary);
}

.toast-message {
    font-size: 0.8rem;
    color: var(--text-secondary);
    margin-top: 0.15rem;
}

.toast-close {
    background: none;
    border: none;
    color: var(--text-secondary);
    cursor: pointer;
    font-size: 1.25rem;
    padding: 0;
    width: 24px;
    height: 24px;
    display: grid;
    place-items: center;
    border-radius: 4px;
    transition: all 0.2s;
}

.toast-close:hover {
    background: var(--bg-glass);
    color: var(--text-primary);
}

@keyframes slideIn {
    from {
        transform: translateX(100%);
        opacity: 0;
    }
    to {
        transform: translateX(0);
        opacity: 1;
    }
}

@keyframes slideOut {
    to {
        transform: translateX(100%);
        opacity: 0;
    }
}

.toast.hiding {
    animation: slideOut 0.3s ease forwards;
}
```

```javascript
// Toast API
const Toast = {
    container: null,
    
    init() {
        if (!this.container) {
            this.container = document.createElement('div');
            this.container.className = 'toast-container';
            document.body.appendChild(this.container);
        }
    },
    
    show(message, type = 'info', duration = 3000) {
        this.init();
        
        const icons = {
            success: '✅',
            error: '❌',
            warning: '⚠️',
            info: 'ℹ️'
        };
        
        const titles = {
            success: 'Erfolg',
            error: 'Fehler',
            warning: 'Warnung',
            info: 'Info'
        };
        
        const toast = document.createElement('div');
        toast.className = `toast ${type}`;
        toast.innerHTML = `
            <span class="toast-icon">${icons[type]}</span>
            <div class="toast-content">
                <div class="toast-title">${titles[type]}</div>
                <div class="toast-message">${message}</div>
            </div>
            <button class="toast-close" onclick="this.parentElement.remove()">✕</button>
        `;
        
        this.container.appendChild(toast);
        
        if (duration > 0) {
            setTimeout(() => {
                toast.classList.add('hiding');
                setTimeout(() => toast.remove(), 300);
            }, duration);
        }
        
        return toast;
    },
    
    success: (msg, duration) => Toast.show(msg, 'success', duration),
    error: (msg, duration) => Toast.show(msg, 'error', duration || 5000),
    warning: (msg, duration) => Toast.show(msg, 'warning', duration),
    info: (msg, duration) => Toast.show(msg, 'info', duration)
};
```

### 6. Keyboard Shortcuts (1 Stunde)

```javascript
// shared.js - Keyboard Shortcuts
const KeyboardShortcuts = {
    shortcuts: {
        'cmd+k': () => openCommandPalette(),
        'cmd+slash': () => showShortcutsHelp(),
        'cmd+s': () => { if (isConfigPage) saveConfig(); },
        'cmd+enter': () => { if (isChatPage) sendMessage(); },
        'esc': () => closeAllModals(),
        'shift+?': () => showShortcutsHelp()
    },
    
    init() {
        document.addEventListener('keydown', (e) => {
            const key = [];
            if (e.metaKey || e.ctrlKey) key.push('cmd');
            if (e.shiftKey) key.push('shift');
            if (e.altKey) key.push('alt');
            key.push(e.key.toLowerCase());
            
            const shortcut = key.join('+');
            if (this.shortcuts[shortcut]) {
                e.preventDefault();
                this.shortcuts[shortcut]();
            }
        });
    }
};

// Command Palette
function openCommandPalette() {
    const modal = document.createElement('div');
    modal.className = 'command-palette-overlay';
    modal.innerHTML = `
        <div class="command-palette">
            <input type="text" placeholder="Befehl suchen..." autofocus>
            <div class="command-list">
                <div class="command-item" data-action="navigate" data-target="/">
                    <span class="cmd-icon">💬</span>
                    <span class="cmd-label">Zum Chat</span>
                    <span class="cmd-shortcut">G C</span>
                </div>
                <div class="command-item" data-action="navigate" data-target="/dashboard">
                    <span class="cmd-icon">📊</span>
                    <span class="cmd-label">Zum Dashboard</span>
                    <span class="cmd-shortcut">G D</span>
                </div>
                <div class="command-item" data-action="theme">
                    <span class="cmd-icon">🌙</span>
                    <span class="cmd-label">Theme wechseln</span>
                    <span class="cmd-shortcut">Cmd+T</span>
                </div>
            </div>
        </div>
    `;
    
    modal.onclick = (e) => {
        if (e.target === modal) modal.remove();
    };
    
    document.body.appendChild(modal);
    modal.querySelector('input').focus();
}
```

## Test-Checkliste

### Visuelle Konsistenz
- [ ] Alle Buttons sehen gleich aus (Primary, Secondary, Danger)
- [ ] Alle Cards haben gleiche Border-Radius und Padding
- [ ] Alle Inputs haben gleiche Fokus-Styles
- [ ] Alle Überschriften nutzen die gleiche Schriftgrößen-Hierarchie

### Interaktion
- [ ] Jeder Klick hat visuelles Feedback (mindestens :hover)
- [ ] Loading-States sind für alle async-Aktionen vorhanden
- [ ] Formulare haben Validierungs-Feedback
- [ ] Navigation zeigt aktiven Zustand klar an

### Mobile
- [ ] Touch-Targets sind mindestens 44x44px
- [ ] Text ist ohne Zoomen lesbar (min. 16px)
- [ ] Navigation ist mit Daumen erreichbar
- [ ] Kein horizontaler Scroll

### Accessibility
- [ ] Alle Bilder haben alt-Text
- [ ] Alle Inputs haben labels
- [ ] Farbkontrast ist WCAG AA konform
- [ ] Keyboard-Navigation funktioniert

---

## Performance-Tipps

1. **CSS-Variablen nutzen** statt inline-styles
2. **Intersection Observer** für Lazy-Loading
3. **Debounced Event Handler** für Scroll/Resize
4. **CSS-Animationen** statt JS-Animationen (GPU-beschleunigt)
5. **Prefetch** für wahrscheinliche nächste Seiten
