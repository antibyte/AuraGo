    const THUMB_MAX_W = 220;
    const THUMB_MAX_H = 140;
    const THUMB_SHOW_DELAY_MS = 360;
    const THUMB_HIDE_DELAY_MS = 140;
    const SENSITIVE_PREVIEW_APPS = new Set([
        'agent-chat', 'calendar', 'code-studio', 'contacts', 'editor', 'files', 'mailer',
        'notes', 'people', 'quick-connect', 'quickconnect', 'settings', 'sheets', 'terminal', 'tresor', 'virtual-computers', 'writer'
    ]);
    const PREVIEW_MAX_NODES = 512;
    const PREVIEW_MAX_TEXT_CHARS = 32 * 1024;
    const PREVIEW_UNSAFE_TAGS = new Set(['audio', 'canvas', 'embed', 'iframe', 'input', 'object', 'select', 'textarea', 'video']);
    const PREVIEW_SKIP_TAGS = new Set([
        'a', 'base', 'button', 'form', 'img', 'link', 'math', 'meta', 'option', 'picture', 'script',
        'source', 'style', 'svg', 'template'
    ]);
    const PREVIEW_ALLOWED_TAGS = new Set([
        'address', 'article', 'aside', 'b', 'blockquote', 'br', 'caption', 'code', 'dd', 'div', 'dl', 'dt',
        'em', 'figcaption', 'figure', 'footer', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'header', 'hr', 'i',
        'label', 'li', 'main', 'nav', 'ol', 'p', 'pre', 'section', 'small', 'span', 'strong', 'sub', 'sup',
        'table', 'tbody', 'td', 'tfoot', 'th', 'thead', 'time', 'tr', 'u', 'ul', 'wbr'
    ]);
    const PREVIEW_SAFE_ATTRIBUTES = new Set(['class', 'dir', 'lang']);
    let thumbShowTimer = 0;
    let thumbHideTimer = 0;
    let activeThumbWindowId = '';

    function taskbarThumbnailsEnabled() {
        if (window.AuraDesktopWindowOverview?.isOpen?.()) return false;
        if (isCompactViewport()) return false;
        if (window.matchMedia && window.matchMedia('(hover: none) and (pointer: coarse)').matches) return false;
        return true;
    }

    function dockThumbnailWindow(appId) {
        const id = String(appId || '');
        if (!id) return null;
        const running = taskbarWindows().filter(win => (
            win && !win.isGadget && win.appId === id && win.element
            && win.element.style.display !== 'none'
            && !win.element.classList.contains('vd-space-hidden')
        ));
        if (!running.length) return null;
        return running.find(win => win.id === state.activeWindowId) || running[0];
    }

    function wireDockThumbnailHover(btn) {
        if (!btn || btn.getAttribute('data-thumb-wired') === 'true') return;
        btn.setAttribute('data-thumb-wired', 'true');
        const preview = () => {
            const win = dockThumbnailWindow(btn.dataset.appId);
            if (win) scheduleShowTaskbarThumbnail(win.id, btn);
        };
        btn.addEventListener('mouseenter', preview);
        btn.addEventListener('mouseleave', scheduleHideTaskbarThumbnail);
        btn.addEventListener('focus', preview);
        btn.addEventListener('blur', scheduleHideTaskbarThumbnail);
    }

    function cancelTaskbarThumbnailTimers() {
        if (thumbShowTimer) {
            window.clearTimeout(thumbShowTimer);
            thumbShowTimer = 0;
        }
        if (thumbHideTimer) {
            window.clearTimeout(thumbHideTimer);
            thumbHideTimer = 0;
        }
    }

    function ensureTaskbarThumbnailPanel() {
        let panel = document.getElementById('vd-taskbar-thumbnail');
        if (!panel) {
            panel = document.createElement('div');
            panel.id = 'vd-taskbar-thumbnail';
            panel.className = 'vd-taskbar-thumbnail';
            panel.hidden = true;
            panel.setAttribute('role', 'tooltip');
            document.body.appendChild(panel);
            panel.addEventListener('mouseenter', () => {
                if (thumbHideTimer) {
                    window.clearTimeout(thumbHideTimer);
                    thumbHideTimer = 0;
                }
            });
            panel.addEventListener('mouseleave', scheduleHideTaskbarThumbnail);
            panel.addEventListener('click', () => {
                if (activeThumbWindowId) focusWindow(activeThumbWindowId);
                hideTaskbarThumbnail();
            });
        }
        return panel;
    }

    function hideTaskbarThumbnail() {
        cancelTaskbarThumbnailTimers();
        activeThumbWindowId = '';
        const panel = document.getElementById('vd-taskbar-thumbnail');
        if (panel) {
            panel.hidden = true;
            panel.replaceChildren();
        }
    }

    function scheduleHideTaskbarThumbnail() {
        if (thumbHideTimer) window.clearTimeout(thumbHideTimer);
        thumbHideTimer = window.setTimeout(hideTaskbarThumbnail, THUMB_HIDE_DELAY_MS);
    }

    function positionTaskbarThumbnail(panel, anchor) {
        if (!panel || !anchor || !anchor.getBoundingClientRect) return;
        const rect = anchor.getBoundingClientRect();
        const panelRect = panel.getBoundingClientRect();
        const margin = 8;
        let left = rect.left + (rect.width / 2) - (panelRect.width / 2);
        left = Math.max(margin, Math.min(left, window.innerWidth - panelRect.width - margin));
        const taskbar = document.querySelector('.vd-taskbar');
        const taskbarTop = taskbar ? taskbar.getBoundingClientRect().top : window.innerHeight;
        let top = taskbarTop - panelRect.height - margin;
        if (top < margin) top = margin;
        panel.style.left = Math.round(left) + 'px';
        panel.style.top = Math.round(top) + 'px';
    }

    function sanitizeWindowPreviewClone(content, appId) {
        if (!content || content.nodeType !== 1 || SENSITIVE_PREVIEW_APPS.has(String(appId || ''))) return null;
        const budget = { nodes: 0, textChars: 0, unsafe: false, exceeded: false };
        const inspectNode = source => {
            if (!source) return;
            budget.nodes++;
            if (budget.nodes > PREVIEW_MAX_NODES) { budget.exceeded = true; return; }
            if (source.nodeType === 3) {
                budget.textChars += String(source.nodeValue || '').length;
                if (budget.textChars > PREVIEW_MAX_TEXT_CHARS) budget.exceeded = true;
                return;
            }
            if (source.nodeType !== 1) {
                // Count comments and other node types toward the traversal cap.
                for (const child of source.childNodes || []) {
                    inspectNode(child);
                    if (budget.exceeded) return;
                }
                return;
            }
            const tag = String(source.localName || source.tagName || '').toLowerCase();
            if (!tag || tag.includes('-')) { budget.unsafe = true; return null; }
            if (PREVIEW_UNSAFE_TAGS.has(tag) || (source.hasAttribute?.('contenteditable') && source.getAttribute('contenteditable') !== 'false')) {
                budget.unsafe = true;
            }
            for (const attribute of PREVIEW_SAFE_ATTRIBUTES) {
                if (!source.hasAttribute?.(attribute)) continue;
                budget.textChars += String(source.getAttribute(attribute) || '').length;
                if (budget.textChars > PREVIEW_MAX_TEXT_CHARS) { budget.exceeded = true; return; }
            }
            // Inspect children even under skipped wrappers. This catches an unsafe
            // descendant hidden inside a form or navigation node before text is copied.
            for (const child of source.childNodes || []) {
                inspectNode(child);
                if (budget.exceeded) return;
            }
        };
        inspectNode(content);
        if (budget.unsafe || budget.exceeded) return null;

        const projectNode = source => {
            if (!source || (source.nodeType !== 1 && source.nodeType !== 3)) return null;
            if (source.nodeType === 3) return document.createTextNode(String(source.nodeValue || ''));
            const tag = String(source.localName || source.tagName || '').toLowerCase();
            if (PREVIEW_SKIP_TAGS.has(tag) || source.hasAttribute?.('hidden') || source.getAttribute?.('aria-hidden') === 'true') return null;
            if (!PREVIEW_ALLOWED_TAGS.has(tag)) return null;
            const projected = document.createElement(tag);
            for (const attribute of PREVIEW_SAFE_ATTRIBUTES) {
                if (source.hasAttribute?.(attribute)) projected.setAttribute(attribute, String(source.getAttribute(attribute) || ''));
            }
            for (const child of source.childNodes || []) {
                const projectedChild = projectNode(child);
                if (projectedChild) projected.appendChild(projectedChild);
            }
            return projected;
        };

        const projected = projectNode(content);
        if (!projected || budget.unsafe || budget.exceeded) return null;
        projected.setAttribute('aria-hidden', 'true');
        projected.setAttribute('inert', '');
        return projected;
    }

    function windowPreviewBodyMarkup(win, options) {
        options = options || {};
        const maxW = options.maxW || THUMB_MAX_W;
        const maxH = options.maxH || THUMB_MAX_H;
        const app = appById(win.appId);
        const content = win.element.querySelector('.vd-window-content');
        const sourceW = Math.max(win.element.offsetWidth || 640, 320);
        const sourceH = Math.max(content ? content.offsetHeight : 420, 240);
        const scale = Math.min(maxW / sourceW, maxH / sourceH, 1);
        const viewW = Math.max(128, Math.round(sourceW * scale));
        const viewH = Math.max(80, Math.round(sourceH * scale));
        const viewportClass = options.viewportClass || 'vd-taskbar-thumb-viewport';
        const scaleClass = options.scaleClass || 'vd-taskbar-thumb-scale';
        const fallbackClass = options.fallbackClass || 'vd-taskbar-thumb-fallback';
        const fallbackIconClass = options.fallbackIconClass || 'vd-taskbar-thumb-fallback-icon';
        const preview = sanitizeWindowPreviewClone(content, win.appId);
        if (!preview) {
            return `<div class="${fallbackClass}">${iconMarkup(iconForApp(app), iconGlyph(app), fallbackIconClass, options.fallbackIconSize || 42)}<span>${esc(t('desktop.taskbar_thumb_live'))}</span></div>`;
        }
        return `<div class="${viewportClass}" inert aria-hidden="true" style="width:${viewW}px;height:${viewH}px;pointer-events:none"><div class="${scaleClass}" inert aria-hidden="true" style="width:${sourceW}px;height:${sourceH}px;transform:scale(${scale});pointer-events:none">${preview.innerHTML}</div></div>`;
    }

    function windowPreviewMarkup(win, options) {
        options = options || {};
        const app = appById(win.appId);
        const iconSize = options.iconSize || 18;
        const icon = iconMarkup(win.icon || iconForApp(app), win.iconGlyph || iconGlyph(app), options.iconClass || 'vd-taskbar-thumb-icon', iconSize);
        const titleClass = options.titleClass || 'vd-taskbar-thumb-title';
        const labelClass = options.labelClass || 'vd-taskbar-thumb-label';
        const title = `<div class="${titleClass}">${icon}<span class="${labelClass}">${esc(win.title)}</span></div>`;
        if (options.bodyOnly) return windowPreviewBodyMarkup(win, options);
        return title + windowPreviewBodyMarkup(win, options);
    }

    function buildTaskbarThumbnailMarkup(win) {
        return windowPreviewMarkup(win, {
            maxW: THUMB_MAX_W,
            maxH: THUMB_MAX_H,
            titleClass: 'vd-taskbar-thumb-title',
            labelClass: 'vd-taskbar-thumb-label',
            iconClass: 'vd-taskbar-thumb-icon',
            iconSize: 18
        });
    }

    function showTaskbarThumbnail(windowId, anchor) {
        if (!taskbarThumbnailsEnabled()) return;
        const win = state.windows.get(windowId);
        if (!win || !win.element || win.isGadget) return;
        if (win.element.style.display === 'none' || win.element.classList.contains('vd-space-hidden')) return;
        if (!isWindowOnActiveSpace(win)) return;
        const panel = ensureTaskbarThumbnailPanel();
        activeThumbWindowId = windowId;
        panel.innerHTML = buildTaskbarThumbnailMarkup(win);
        panel.hidden = false;
        panel.setAttribute('aria-label', t('desktop.taskbar_thumb'));
        positionTaskbarThumbnail(panel, anchor);
    }

    function scheduleShowTaskbarThumbnail(windowId, anchor) {
        if (!taskbarThumbnailsEnabled()) return;
        if (thumbHideTimer) {
            window.clearTimeout(thumbHideTimer);
            thumbHideTimer = 0;
        }
        if (thumbShowTimer) window.clearTimeout(thumbShowTimer);
        thumbShowTimer = window.setTimeout(() => {
            thumbShowTimer = 0;
            showTaskbarThumbnail(windowId, anchor);
        }, THUMB_SHOW_DELAY_MS);
    }

    function wireTaskbarThumbnailHover(btn) {
        if (!taskbarThumbnailsEnabled()) return;
        if (btn.getAttribute('data-thumb-wired') === 'true') return;
        btn.setAttribute('data-thumb-wired', 'true');
        btn.addEventListener('mouseenter', () => scheduleShowTaskbarThumbnail(btn.dataset.windowId, btn));
        btn.addEventListener('mouseleave', scheduleHideTaskbarThumbnail);
        btn.addEventListener('focus', () => scheduleShowTaskbarThumbnail(btn.dataset.windowId, btn));
        btn.addEventListener('blur', scheduleHideTaskbarThumbnail);
    }

    function initTaskbarThumbnailsRuntime() {
        hideTaskbarThumbnail();
        window.addEventListener('resize', hideTaskbarThumbnail);
        if (window.visualViewport) window.visualViewport.addEventListener('resize', hideTaskbarThumbnail);
    }
