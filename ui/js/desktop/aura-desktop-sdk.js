(function () {
    'use strict';

    const REQUEST_TYPE = 'aurago.desktop.request';
    const RESPONSE_TYPE = 'aurago.desktop.response';
    const MENU_ACTION_TYPE = 'aurago.desktop.menu-action';
    const CONTEXT_MENU_ACTION_TYPE = 'aurago.desktop.context-menu-action';
    const RUNTIME = 'aura-desktop-sdk@1';
    const VERSION = '1.0.0';
    const THEMED_ICON_PREFIXES = ['papirus:', 'whitesur:'];
    let requestSeq = 0;
    let contextPromise = null;
    let iconContextPromise = null;
    let iconManifestPromise = null;
    const pending = new Map();
    const menuActionHandlers = new Map();
    const menuDirectActions = new Map();
    const contextMenuActionHandlers = new Map();
    const contextMenuDirectActions = new Map();
    let menuActionSeq = 0;
    let contextMenuActionSeq = 0;
    let contextMenuDisposer = null;
    let widgetAutoResizeStarted = false;
    let widgetAutoResizeFrame = 0;
    let widgetAutoResizeObserver = null;

    function parentRequest(action, payload) {
        const id = 'sdk-' + Date.now() + '-' + (++requestSeq);
        return new Promise((resolve, reject) => {
            const timer = window.setTimeout(() => {
                pending.delete(id);
                reject(new Error('Desktop bridge request timed out'));
            }, 15000);
            pending.set(id, { resolve, reject, timer });
            window.parent.postMessage({
                type: REQUEST_TYPE,
                id,
                action,
                payload: payload || {}
            }, '*');
        });
    }

    window.addEventListener('message', (event) => {
        if (event.origin !== window.location.origin || event.source !== window.parent) {
            return;
        }
        const msg = event.data;
        if (msg && msg.type === MENU_ACTION_TYPE) {
            dispatchMenuAction(msg.actionId);
            return;
        }
        if (msg && msg.type === CONTEXT_MENU_ACTION_TYPE) {
            dispatchContextMenuAction(msg.actionId);
            return;
        }
        if (!msg || msg.type !== RESPONSE_TYPE || !pending.has(msg.id)) return;
        const item = pending.get(msg.id);
        pending.delete(msg.id);
        window.clearTimeout(item.timer);
        if (msg.ok) {
            item.resolve(msg.payload);
        } else {
            item.reject(new Error(msg.error || 'Desktop bridge request failed'));
        }
    });

    function dispatchMenuAction(actionId) {
        const id = String(actionId || '');
        const direct = menuDirectActions.get(id);
        if (typeof direct === 'function') {
            direct(id);
            return;
        }
        menuActionHandlers.forEach(handler => {
            if (typeof handler === 'function') handler(id);
        });
    }

    function dispatchContextMenuAction(actionId) {
        const id = String(actionId || '');
        const direct = contextMenuDirectActions.get(id);
        if (typeof direct === 'function') {
            direct(id);
            return;
        }
        contextMenuActionHandlers.forEach(handler => {
            if (typeof handler === 'function') handler(id);
        });
    }

    function context() {
        if (!contextPromise) contextPromise = parentRequest('desktop:context');
        return contextPromise;
    }

    function widgetIdFromLocation() {
        try {
            return new URLSearchParams(window.location.search).get('widget_id') || '';
        } catch (_) {
            return '';
        }
    }

    function measureWidgetContentSize() {
        const doc = document.documentElement;
        const body = document.body;
        let contentWidth = 0;
        let contentHeight = 0;
        const include = node => {
            if (!node) return;
            const rect = typeof node.getBoundingClientRect === 'function' ? node.getBoundingClientRect() : null;
            const left = rect ? rect.left + (window.scrollX || 0) : 0;
            const top = rect ? rect.top + (window.scrollY || 0) : 0;
            contentWidth = Math.max(contentWidth, node.scrollWidth || 0, node.offsetWidth || 0, node.clientWidth || 0, rect ? rect.right + (window.scrollX || 0) : 0, left + (node.scrollWidth || 0));
            contentHeight = Math.max(contentHeight, node.scrollHeight || 0, node.offsetHeight || 0, node.clientHeight || 0, rect ? rect.bottom + (window.scrollY || 0) : 0, top + (node.scrollHeight || 0));
        };
        include(doc);
        include(body);
        if (body) body.querySelectorAll('*').forEach(include);
        return {
            width: Math.ceil(contentWidth),
            height: Math.ceil(contentHeight)
        };
    }

    function normalizeWidgetResizePayload(options) {
        const measured = measureWidgetContentSize();
        const data = options && typeof options === 'object' ? options : measured;
        return {
            width: Math.ceil(Number(data.width || data.w || measured.width || 0)),
            height: Math.ceil(Number(data.height || data.h || measured.height || 0))
        };
    }

    function queueWidgetAutoResize() {
        if (!widgetAutoResizeStarted) return;
        if (widgetAutoResizeFrame) window.cancelAnimationFrame(widgetAutoResizeFrame);
        widgetAutoResizeFrame = window.requestAnimationFrame(() => {
            widgetAutoResizeFrame = 0;
            widgets.resize(measureWidgetContentSize()).catch(() => {});
        });
    }

    function startWidgetAutoResize() {
        if (widgetAutoResizeStarted || !widgetIdFromLocation()) return;
        widgetAutoResizeStarted = true;
        queueWidgetAutoResize();
        if (window.ResizeObserver) {
            widgetAutoResizeObserver = new ResizeObserver(queueWidgetAutoResize);
            if (document.documentElement) widgetAutoResizeObserver.observe(document.documentElement);
            if (document.body) widgetAutoResizeObserver.observe(document.body);
        }
        window.addEventListener('load', queueWidgetAutoResize, { once: true });
        window.addEventListener('resize', queueWidgetAutoResize);
        if (document.fonts && document.fonts.ready) {
            document.fonts.ready.then(queueWidgetAutoResize).catch(() => {});
        }
    }

    function append(parent, child) {
        if (child == null || child === false) return;
        if (Array.isArray(child)) {
            child.forEach(item => append(parent, item));
            return;
        }
        if (child instanceof Node) {
            parent.appendChild(child);
            return;
        }
        parent.appendChild(document.createTextNode(String(child)));
    }

    function el(tag, attrs, children) {
        const node = document.createElement(tag);
        Object.entries(attrs || {}).forEach(([key, value]) => {
            if (value == null || value === false) return;
            if (key === 'className') node.className = value;
            else if (key === 'text') node.textContent = value;
            else if (key === 'html') node.innerHTML = value;
            else if (key === 'dataset') Object.entries(value).forEach(([name, data]) => { node.dataset[name] = data; });
            else if (key === 'style' && typeof value === 'object') Object.assign(node.style, value);
            else if (key.startsWith('on') && typeof value === 'function') node.addEventListener(key.slice(2).toLowerCase(), value);
            else if (value === true) node.setAttribute(key, '');
            else node.setAttribute(key, String(value));
        });
        append(node, children);
        return node;
    }

    function loadIcons() {
        if (!iconManifestPromise) {
            iconManifestPromise = loadIconContext()
                .then(ctx => ctx.sprite || null)
                .catch(() => null);
        }
        return iconManifestPromise;
    }

    function loadIconContext() {
        if (!iconContextPromise) {
            iconContextPromise = context()
                .then(ctx => ({
                    sprite: ctx && ctx.icon_manifest ? ctx.icon_manifest : null,
                    themes: ctx && ctx.icon_theme_manifests
                        ? ctx.icon_theme_manifests
                        : (ctx && ctx.papirus_icon_manifest ? { papirus: ctx.papirus_icon_manifest } : {}),
                    theme: ctx && ctx.bootstrap && ctx.bootstrap.settings ? ctx.bootstrap.settings['appearance.icon_theme'] || 'papirus' : 'papirus'
                }))
                .catch(() => ({ sprite: null, themes: {}, theme: 'papirus' }));
        }
        return iconContextPromise;
    }

    function normalizeIconName(name) {
        return String(name || '').trim().toLowerCase().replace(/[^a-z0-9:_-]+/g, '_');
    }

    function themeIconPath(iconContext, name) {
        iconContext = iconContext || {};
        const themes = iconContext.themes || {};
        let normalized = normalizeIconName(name);
        if (!normalized || normalized.startsWith('sprite:')) return '';
        let theme = iconContext.theme || 'papirus';
        const themeKeys = Array.from(new Set([
            ...Object.keys(themes),
            ...THEMED_ICON_PREFIXES.map(prefix => prefix.slice(0, -1))
        ]));
        themeKeys.forEach(themeKey => {
            const prefix = themeKey + ':';
            if (normalized.startsWith(prefix)) {
                theme = themeKey;
                normalized = normalized.slice(prefix.length);
            }
        });
        const manifest = themes[theme] || themes.papirus;
        if (!manifest || !manifest.icons) return '';
        const aliases = manifest.aliases || {};
        const candidates = [
            normalized,
            aliases[normalized],
            normalized.replaceAll('_', '-'),
            aliases[normalized.replaceAll('_', '-')]
        ].filter(Boolean);
        for (const candidate of candidates) {
            if (manifest.icons[candidate]) return '/' + String(manifest.icons[candidate]).replace(/^\/+/, '');
        }
        return '';
    }

    function spriteIconName(name) {
        const normalized = normalizeIconName(name);
        return normalized.startsWith('sprite:') ? normalized.slice('sprite:'.length) : normalized;
    }

    function resolveIconSource(name, iconContext) {
        iconContext = iconContext || {};
        const normalized = normalizeIconName(name);
        if (!normalized) return { type: 'fallback' };
        if (!normalized.startsWith('sprite:')) {
            const path = themeIconPath(iconContext, normalized);
            if (path) return { type: 'theme', path };
        }
        const spriteName = spriteIconName(normalized);
        const manifest = iconContext.sprite;
        const exists = manifest && Array.isArray(manifest.icons) && manifest.icons.some(item => item.name === spriteName);
        return exists ? { type: 'sprite', name: spriteName } : { type: 'fallback' };
    }

    function applySpriteIcon(span, manifest, name, size) {
        if (!manifest || !Array.isArray(manifest.icons)) {
            span.textContent = String(name || 'app').slice(0, 2).toUpperCase();
            span.classList.add('ad-icon-fallback');
            return;
        }
        const icon = manifest.icons.find(item => item.name === name);
        if (!icon) {
            span.textContent = String(name || 'app').slice(0, 2).toUpperCase();
            span.classList.add('ad-icon-fallback');
            return;
        }
        const iconSize = manifest.icon_size || 128;
        const scale = size / iconSize;
        span.style.backgroundImage = "url('/img/desktop-icons-sprite.png')";
        span.style.backgroundSize = `${Math.round((manifest.width || 1536) * scale)}px ${Math.round((manifest.height || 1536) * scale)}px`;
        span.style.backgroundPosition = `${Math.round(-icon.x * scale)}px ${Math.round(-icon.y * scale)}px`;
    }

    function applyResolvedIcon(span, iconContext, name, size) {
        const source = resolveIconSource(name, iconContext);
        if (source.type === 'theme') {
            span.style.backgroundImage = `url('${source.path}')`;
            span.classList.add('ad-theme-icon', 'ad-papirus-icon');
            return;
        }
        applySpriteIcon(span, iconContext && iconContext.sprite, source.name || name, size);
    }

    function desktopIcon(name, options) {
        const size = Number((options && options.size) || 22);
        const span = el('span', {
            className: 'ad-icon',
            'aria-hidden': 'true',
            style: { width: size + 'px', height: size + 'px' }
        });
        loadIconContext().then(iconContext => applyResolvedIcon(span, iconContext, name, size));
        return span;
    }

    function spriteIcon(name, options) {
        return desktopIcon('sprite:' + name, options);
    }

    const ui = {};

    ui.icon = function icon(name, options) {
        return desktopIcon(name, options);
    };

    ui.button = function button(options) {
        options = options || {};
        const className = ['ad-button', options.variant ? 'ad-button-' + options.variant : ''].filter(Boolean).join(' ');
        const buttonEl = el('button', {
            className,
            type: options.type || 'button',
            title: options.title || options.label || '',
            onclick: options.onClick
        }, [
            options.icon ? ui.icon(options.icon, { size: options.iconSize || 20 }) : null,
            options.label ? el('span', { className: 'ad-button-label', text: options.label }) : null
        ]);
        return buttonEl;
    };

    ui.toolbar = function toolbar(items) {
        return el('div', { className: 'ad-toolbar' }, items || []);
    };

    function serializeMenuItems(items) {
        return (Array.isArray(items) ? items : []).map((item, index) => {
            if (!item) return null;
            if (item.type === 'separator' || item.separator) return { type: 'separator' };
            const actionId = item.actionId || (typeof item.action === 'string' ? item.action : '') || item.id || ('item-' + index);
            const serialized = {
                id: item.id || actionId,
                label: item.label || '',
                labelKey: item.labelKey || '',
                icon: item.icon || '',
                fallback: item.fallback || '',
                shortcut: item.shortcut || '',
                disabled: !!item.disabled,
                checked: !!item.checked,
                hidden: !!item.hidden
            };
            const children = item.items || item.children;
            if (children) {
                serialized.items = serializeMenuItems(children);
            } else {
                serialized.actionId = actionId;
                if (typeof item.action === 'function') menuDirectActions.set(actionId, item.action);
            }
            return serialized;
        }).filter(Boolean);
    }

    function serializeMenus(menus) {
        menuDirectActions.clear();
        return (Array.isArray(menus) ? menus : []).map((menu, index) => ({
            id: menu && menu.id || ('menu-' + index),
            label: menu && menu.label || '',
            labelKey: menu && menu.labelKey || '',
            hidden: !!(menu && menu.hidden),
            items: serializeMenuItems(menu && menu.items)
        }));
    }

    function serializeContextMenuItems(items) {
        return (Array.isArray(items) ? items : []).map((item, index) => {
            if (!item) return null;
            if (item.type === 'separator' || item.separator) return { type: 'separator' };
            const actionId = item.actionId || (typeof item.action === 'string' ? item.action : '') || item.id || ('context-item-' + index);
            const serialized = {
                id: item.id || actionId,
                label: item.label || '',
                labelKey: item.labelKey || '',
                icon: item.icon || '',
                fallback: item.fallback || '',
                shortcut: item.shortcut || '',
                disabled: !!item.disabled,
                checked: !!item.checked,
                hidden: !!item.hidden
            };
            const children = item.items || item.children;
            if (children) {
                serialized.items = serializeContextMenuItems(children);
            } else {
                serialized.actionId = actionId;
                if (typeof item.action === 'function') contextMenuDirectActions.set(actionId, item.action);
            }
            return serialized;
        }).filter(Boolean);
    }

    function contextMenuPoint(eventOrPoint) {
        if (eventOrPoint && typeof eventOrPoint.preventDefault === 'function') {
            eventOrPoint.preventDefault();
            if (typeof eventOrPoint.stopPropagation === 'function') eventOrPoint.stopPropagation();
            return { x: eventOrPoint.clientX || 0, y: eventOrPoint.clientY || 0 };
        }
        if (eventOrPoint && typeof eventOrPoint === 'object') {
            return { x: Number(eventOrPoint.x || eventOrPoint.clientX || 0), y: Number(eventOrPoint.y || eventOrPoint.clientY || 0) };
        }
        return { x: Math.round(window.innerWidth / 2), y: Math.round(window.innerHeight / 2) };
    }

    ui.menu = {
        set(menus) {
            return parentRequest('desktop:menu:set', { menus: serializeMenus(menus) });
        },
        clear() {
            menuDirectActions.clear();
            return parentRequest('desktop:menu:clear');
        },
        onAction(handler) {
            const id = 'handler-' + (++menuActionSeq);
            menuActionHandlers.set(id, handler);
            return () => menuActionHandlers.delete(id);
        }
    };

    ui.contextMenu = {
        set(itemsOrFactory) {
            if (contextMenuDisposer) {
                contextMenuDisposer();
                contextMenuDisposer = null;
            }
            contextMenuDirectActions.clear();
            parentRequest('desktop:context-menu:clear').catch(() => {});
            const handler = event => {
                const items = typeof itemsOrFactory === 'function' ? itemsOrFactory(event) : itemsOrFactory;
                if (!items || (Array.isArray(items) && !items.length)) {
                    event.preventDefault();
                    event.stopPropagation();
                    return;
                }
                ui.contextMenu.show(items, event);
            };
            document.addEventListener('contextmenu', handler);
            contextMenuDisposer = () => document.removeEventListener('contextmenu', handler);
            return contextMenuDisposer;
        },
        show(items, eventOrPoint) {
            contextMenuDirectActions.clear();
            const point = contextMenuPoint(eventOrPoint);
            return parentRequest('desktop:context-menu:show', {
                x: point.x,
                y: point.y,
                items: serializeContextMenuItems(items)
            });
        },
        clear() {
            if (contextMenuDisposer) {
                contextMenuDisposer();
                contextMenuDisposer = null;
            }
            contextMenuDirectActions.clear();
            return parentRequest('desktop:context-menu:clear');
        },
        onAction(handler) {
            const id = 'context-handler-' + (++contextMenuActionSeq);
            contextMenuActionHandlers.set(id, handler);
            return () => contextMenuActionHandlers.delete(id);
        }
    };

    ui.clipboard = {
        readText() {
            return parentRequest('desktop:clipboard:read-text').then(body => body && body.text || '');
        },
        writeText(text) {
            return parentRequest('desktop:clipboard:write-text', { text: String(text == null ? '' : text) });
        }
    };

    ui.panel = function panel(children, options) {
        return el('section', { className: 'ad-panel' + ((options && options.compact) ? ' ad-panel-compact' : '') }, children || []);
    };

    ui.card = function card(options) {
        options = options || {};
        return el('article', { className: 'ad-card' }, [
            options.icon ? ui.icon(options.icon, { size: 28 }) : null,
            el('div', { className: 'ad-card-content' }, [
                options.title ? el('h3', { text: options.title }) : null,
                options.body ? el('p', { text: options.body }) : null,
                options.content || null
            ])
        ]);
    };

    ui.emptyState = function emptyState(options) {
        options = options || {};
        return el('div', { className: 'ad-empty' }, [
            options.icon ? ui.icon(options.icon, { size: 34 }) : null,
            options.title ? el('strong', { text: options.title }) : null,
            options.body ? el('span', { text: options.body }) : null
        ]);
    };

    ui.input = function input(options) {
        options = options || {};
        return el('input', {
            className: 'ad-input',
            type: options.type || 'text',
            placeholder: options.placeholder || '',
            value: options.value || '',
            oninput: options.onInput
        });
    };

    ui.textarea = function textarea(options) {
        options = options || {};
        const node = el('textarea', {
            className: 'ad-textarea',
            placeholder: options.placeholder || '',
            spellcheck: options.spellcheck === true ? 'true' : 'false',
            oninput: options.onInput
        });
        node.value = options.value || '';
        return node;
    };

    ui.select = function select(options) {
        options = options || {};
        return el('select', { className: 'ad-select', onchange: options.onChange },
            (options.options || []).map(item => el('option', {
                value: item.value,
                selected: item.value === options.value
            }, item.label || item.value))
        );
    };

    ui.field = function field(options) {
        options = options || {};
        return el('label', { className: 'ad-field' }, [
            options.label ? el('span', { className: 'ad-field-label', text: options.label }) : null,
            options.control || null,
            options.hint ? el('small', { text: options.hint }) : null
        ]);
    };

    ui.toggle = function toggle(options) {
        options = options || {};
        return el('label', { className: 'ad-toggle' }, [
            el('input', {
                type: 'checkbox',
                checked: !!options.checked,
                onchange: options.onChange
            }),
            el('span', { className: 'ad-toggle-track' }),
            options.label ? el('span', { className: 'ad-toggle-label', text: options.label }) : null
        ]);
    };

    ui.tabs = function tabs(options) {
        options = options || {};
        const host = el('div', { className: 'ad-tabs' });
        (options.tabs || []).forEach(tab => {
            host.appendChild(el('button', {
                className: 'ad-tab' + (tab.id === options.active ? ' active' : ''),
                type: 'button',
                onclick: () => options.onChange && options.onChange(tab.id)
            }, tab.label || tab.id));
        });
        return host;
    };

    ui.list = function list(items, renderItem) {
        return el('div', { className: 'ad-list' }, (items || []).map((item, index) => {
            const rendered = renderItem ? renderItem(item, index) : String(item);
            return el('div', { className: 'ad-list-row' }, rendered);
        }));
    };

    ui.toast = function toast(message) {
        const node = el('div', { className: 'ad-toast', text: message || '' });
        document.body.appendChild(node);
        window.setTimeout(() => node.remove(), 3600);
        return node;
    };

    const fs = {};
    fs.list = path => parentRequest('fs:list', { path: path || '' });
    fs.read = path => parentRequest('fs:read', { path: path || '' });
    fs.write = (path, content) => parentRequest('fs:write', { path: path || '', content: content || '' });

    const widgets = {};
    widgets.register = definition => parentRequest('widget:upsert', definition || {});
    widgets.resize = options => parentRequest('desktop:widget:resize', normalizeWidgetResizePayload(options));

    const notifications = {};
    notifications.show = options => parentRequest('notification:show', options || {});

    const desktop = {};
    desktop.openApp = appID => parentRequest('app:open', { app_id: appID });
    desktop.context = context;

    function app(options) {
        options = options || {};
        const root = typeof options.root === 'string'
            ? document.querySelector(options.root)
            : (options.root || document.getElementById('app') || document.body);
        root.classList.add('ad-app');
        return {
            root,
            context,
            mount(content) {
                root.replaceChildren();
                append(root, content);
                return root;
            },
            toolbar(items) {
                const bar = ui.toolbar(items);
                root.prepend(bar);
                return bar;
            },
            notify(message, title) {
                return notifications.show({ title: title || options.title || '', message });
            }
        };
    }

    window.AuraDesktop = {
        version: VERSION,
        runtime: RUNTIME,
        request: parentRequest,
        context,
        app,
        el,
        ui,
        menu: ui.menu,
        contextMenu: ui.contextMenu,
        clipboard: ui.clipboard,
        fs,
        widgets,
        notifications,
        desktop,
        icons: {
            resolve: name => loadIconContext().then(iconContext => resolveIconSource(name, iconContext)),
            sprite: spriteIcon,
            icon: desktopIcon,
            catalog: () => context().then(ctx => ctx && ctx.bootstrap ? ctx.bootstrap.icon_catalog || null : null),
            load: loadIcons
        }
    };
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', startWidgetAutoResize, { once: true });
    } else {
        startWidgetAutoResize();
    }
})();
