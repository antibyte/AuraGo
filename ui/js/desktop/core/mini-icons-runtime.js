    // Action roles are semantic: a 16px taskbar logo is still an app icon.
    const MINI_SYMBOLS = new Set(["chevron-left","chevron-right","chevron-up","chevron-down","arrow-up","arrow-down","plus","minus","x","check","square","check-square","maximize","restore","refresh","undo","redo","menu","list","sort","play","pause","stop","external","eye","eye-off","zoom-in","zoom-out","grid","columns","layout","keyboard","contrast","mesh","format-bold","format-italic","format-strike","format-numbered","format-quote"]);
    const MINI_ALIASES = {
        'arrow-left': 'chevron-left', back: 'chevron-left', 'arrow-right': 'chevron-right',
        'folder-open': 'folder', documents: 'file', 'file-text': 'file', paste: 'clipboard',
        cut: 'scissors', delete: 'trash', 'trash-empty': 'trash', 'trash-full': 'trash',
        'gallery-action-delete': 'trash', 'gallery-action-download': 'download',
        'gallery-action-edit': 'edit', 'gallery-action-preview': 'eye', 'check_square': 'check-square',
        'agent-chat': 'chat', agent: 'chat', 'message-square': 'chat', attach: 'attachment',
        audio: 'music', 'audio-player': 'music', 'music-player': 'music', volume: 'speaker', sound: 'speaker',
        'volume-2': 'speaker', lock: 'shield', 'unlock': 'key', tools: 'sliders',
        widgets: 'layout', launchpad: 'apps', desktop: 'monitor',
        browser: 'globe', 'theme-threedee': 'cube', 'code-studio': 'code', cheater: 'notes',
        'file-code': 'code', 'file-archive': 'archive',
        'git-branch': 'network', 'git-commit': 'code', 'git-merge': 'network',
        'settings-2': 'sliders', 'user': 'users', 'image-plus': 'image', 'pencil': 'edit',
        'hard-drive': 'server', 'refresh-cw': 'refresh', 'rotate-cw': 'refresh',
        'rotate-ccw': 'undo', 'maximize-2': 'maximize', 'minimize': 'minus',
        'more-horizontal': 'menu', 'more-vertical': 'menu', 'check-circle': 'check',
        'file-minus': 'file', 'external-link': 'external', 'zoom-reset': 'search'
    };
    function miniIconRole(className) {
        return /(?:^|\s)(?:vd-(?:tool|context-papirus|window-menu-papirus|settings-nav|settings-pane-papirus|settings-hamburger|modal-action|window-ai-button|quickchat-send|dock-scroll|todo-action|calendar-(?:action|mini)|gallery-action|chess-action|chat-(?:toolbar|sidebar|scroll|voice|send|context)|qc-(?:btn|filter|close|input)|launchpad-action|store-(?:btn|terminal-action|terminal-tab-close)|hp-(?:btn|send)|viewer-action)|fm-(?:btn|search|sort-indicator|drop|context)|cs-(?:button|icon-button|file-action|tab-close)|pixel-toolbar|oscad-btn|camera-btn)-icon(?:\s|$)/.test(String(className || ''));
    }
    function miniIconName(key) {
        const name = normalizeIconName(key).replace(/^(?:papirus|whitesur):/, '').replace(/_/g, '-').replace(/-symbolic$/, '');
        return MINI_ALIASES[name] || name;
    }
    function miniIconMarkup(key, className, size) {
        const name = miniIconName(key);
        const pixels = Math.max(8, Math.min(64, Number(size) || 16));
        const manifest = state.miniIconManifest, icon = manifest && manifest.icons && manifest.icons[name];
        // Prefer colored action artwork; structural glyphs retain sharp SVGs.
        if (!icon && MINI_SYMBOLS.has(name)) {
            return '<svg class="' + esc(className) + ' vd-mini-symbol" aria-hidden="true" focusable="false" width="' + pixels + '" height="' + pixels + '"><use href="' + esc(versionedIconAssetPath('/img/desktop-mini/symbols.svg')) + '#' + name + '"></use></svg>';
        }
        if (!icon) return '';
        const scale = pixels / manifest.icon_size;
        return '<span class="' + esc(className) + ' vd-mini-icon" data-vd-mini-key="' + esc(name) + '" aria-hidden="true" style="width:' + pixels + 'px;height:' + pixels + 'px;--vd-mini-position:' + (-icon.x*scale) + 'px ' + (-icon.y*scale) + 'px;--vd-mini-size:' + manifest.width*scale + 'px ' + manifest.height*scale + 'px"></span>';
    }
    function refreshMiniIconTheme() {
        const manifest = state.miniIconManifest;
        if (!manifest || !manifest.images) return;
        const path = manifest.images[isFruityTheme() ? 'fruity' : 'standard'];
        if (path) document.body.style.setProperty('--vd-mini-sheet', iconUrlStyle(path));
    }
