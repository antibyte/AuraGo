/* Galaxy owns placement, while the original controls retain their handlers. */
(() => {
    'use strict';
    const root = document.documentElement;
    const byId = id => document.getElementById(id);
    const translate = key => typeof t === 'function' ? t(key) : key;
    const asset = name => '/img/galaxy/' + name + '?v=' + encodeURIComponent(window.BUILD_VERSION || 'galaxy-2');
    let active = false, moves = [], owned = [], observer = null, headerObserver = null, clockTimer = null;
    let originalPlaceholder = '';
    function element(tag, className, text) {
        const node = document.createElement(tag);
        node.className = className;
        if (text) node.textContent = text;
        return node;
    }
    function own(node, parent) {
        parent.append(node);
        owned.push(node);
        return node;
    }
    function move(node, parent, before = null) {
        if (!node) return;
        const anchor = document.createComment('galaxy-position');
        node.before(anchor);
        moves.push([node, anchor]);
        parent.insertBefore(node, before);
    }
    const glyphs = {
        voice: 'mic', more: 'layout-grid', attach: 'paperclip', network: 'users', send: 'send',
        warning: 'shield', target: 'target', home: 'house', conversation: 'message-circle',
        'mood-analytical': 'chart-no-axes-column-increasing', 'mood-curious': 'star', settings: 'settings-2',
        generic: 'sparkles', 'mood-creative': 'lightbulb', document: 'file-text',
        'chevron-down': 'chevron-down', speaker: 'volume-2', 'speaker-muted': 'volume-x', web: 'orbit'
    };
    function decorateIcon(node) {
        const key = glyphs[node.dataset.chatIcon];
        let svg = node.querySelector('.galaxy-glyph');
        if (!key) { svg?.remove(); return; }
        if (!svg) {
            svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
            svg.classList.add('galaxy-glyph');
            svg.setAttribute('viewBox', '0 0 24 24');
            svg.setAttribute('aria-hidden', 'true');
            svg.append(document.createElementNS('http://www.w3.org/2000/svg', 'use'));
            own(svg, node);
        }
        const href = asset('control-symbols.svg') + '#' + key;
        if (svg.firstChild.getAttribute('href') !== href) svg.firstChild.setAttribute('href', href);
    }
    function icon(key) {
        const node = element('span', 'chat-ui-icon');
        node.setAttribute('aria-hidden', 'true');
        node.dataset.chatIcon = key;
        decorateIcon(node);
        return node;
    }
    function updateHeader() {
        if (!active) return;
        document.querySelectorAll('.app-header [data-chat-icon]').forEach(decorateIcon);
        const source = byId('connectionPill');
        const status = document.querySelector('.galaxy-welcome-status');
        if (source && status) {
            status.textContent = source.textContent;
            status.dataset.state = source.classList.contains('pill-active') ? 'connected' : source.classList.contains('pill-reconnecting') ? 'reconnecting' : 'disconnected';
        }
    }
    function link(href, key, label, className = '') {
        const node = element('a', className);
        node.href = href;
        node.setAttribute('aria-label', translate(label));
        node.title = translate(label);
        node.append(icon(key));
        return node;
    }
    function label(node, key) {
        if (node) own(element('span', 'galaxy-control-label', translate('chat.galaxy_' + key)), node);
    }
    function clock() {
        clearTimeout(clockTimer);
        const node = byId('galaxy-clock');
        if (!active || document.hidden || !node) return;
        const now = new Date();
        const locale = root.lang || navigator.language;
        node.querySelector('time').textContent = now.toLocaleTimeString(locale, {hour: '2-digit', minute: '2-digit'});
        node.querySelector('time').dateTime = now.toISOString();
        node.querySelector('small').textContent = now.toLocaleDateString(locale, {weekday: 'short', day: 'numeric', month: 'short', year: 'numeric'});
        clockTimer = setTimeout(clock, 60000 - now.getSeconds() * 1000);
    }
    function greeting() {
        if (!active) return;
        const content = byId('chat-content');
        const row = content?.querySelector('[data-greeting], .greeting-row');
        content?.classList.toggle('galaxy-welcome-only', !!row && !content.querySelector('.msg-row'));
        if (!row || row.querySelector('.galaxy-welcome')) return;
        const card = element('div', 'galaxy-welcome');
        const orb = element('img', 'galaxy-orb');
        orb.src = asset('orb-companion.png');
        orb.alt = '';
        orb.width = 180;
        orb.height = 180;
        card.append(orb, element('span', 'galaxy-welcome-status'));
        const copy = element('div', 'galaxy-welcome-copy');
        copy.append(element('small', 'galaxy-eyebrow', translate('chat.galaxy_eyebrow')));
        copy.append(element('h1', '', translate('chat.galaxy_title')));
        copy.append(element('p', '', translate('chat.galaxy_subtitle')));
        card.append(copy);
        const choices = element('div', 'galaxy-suggestions');
        for (const [key, symbol] of [['ideas', 'mood-creative'], ['write', 'document'], ['analyze', 'mood-analytical'], ['other', 'generic']]) {
            const button = element('button', 'galaxy-suggestion');
            button.type = 'button';
            button.append(icon(symbol), element('span', '', translate('chat.galaxy_' + key)));
            button.addEventListener('click', () => {
                const input = byId('user-input');
                if (!input || input.disabled) return;
                input.value = key === 'other' ? '' : translate('chat.galaxy_' + key);
                input.dispatchEvent(new Event('input', {bubbles: true}));
                input.focus();
            });
            choices.append(button);
        }
        card.append(choices);
        own(card, row);
        updateHeader();
    }
    function mount() {
        if (active || !byId('chat-form')) return;
        active = true;
        const form = byId('chat-form'), input = form.querySelector('.input-wrap');
        const panel = byId('composer-panel');
        move(byId('upload-btn'), form, input);
        move(byId('composer-more-btn'), form, input);
        move(panel, form, input);
        for (const [id, key] of [['voice-btn', 'voice'], ['composer-more-btn', 'tools'], ['upload-btn', 'file'], ['send-btn', 'send'], ['realtime-speech-btn', 'live']]) label(byId(id), key);

        const logo = document.querySelector('.app-header .logo');
        logo.style.setProperty('--galaxy-mark', 'url("' + asset('orbit-mark.png') + '")');
        own(element('small', 'galaxy-brand-tag', 'YOUR AI AGENT\nFOR A BRIGHTER TOMORROW'), logo);
        const nav = own(element('nav', 'galaxy-nav'), document.body);
        nav.setAttribute('aria-label', translate('common.nav_aria_label'));
        nav.append(link('/desktop', 'home', 'common.nav_desktop'));
        move(byId('integrations-toggle-btn'), nav);
        move(byId('session-toggle-btn'), nav);
        nav.append(link('/dashboard', 'mood-analytical', 'common.nav_dashboard'));
        nav.append(link('/missions', 'mood-curious', 'common.nav_missions'));
        nav.append(link('/config', 'settings', 'common.nav_config', 'galaxy-nav-settings'));
        const plate = own(element('div', 'galaxy-clock'), document.body);
        plate.id = 'galaxy-clock';
        plate.append(element('time', ''), element('small', ''));
        clock();
        const left = own(element('div', 'galaxy-motto galaxy-motto-left', 'EXPLORE.\nTHINK.\nCREATE\nTOGETHER'), document.body);
        const right = own(element('div', 'galaxy-motto galaxy-motto-right', 'A MORE\nINTELLIGENT\nTOMORROW\n— TOGETHER'), document.body);
        [left, right].forEach(node => node.setAttribute('aria-hidden', 'true'));
        label(byId('personality-select'), 'persona');
        label(byId('moodToggle'), 'mood');
        originalPlaceholder = byId('user-input').placeholder;
        byId('user-input').placeholder = translate('chat.galaxy_placeholder');
        // The shared composer owns toggling, outside click, Escape and focus.
        if (!byId('composer-more-btn').classList.contains('is-open')) panel.classList.add('is-hidden');
        greeting();
        observer = new MutationObserver(greeting);
        headerObserver = new MutationObserver(updateHeader);
        headerObserver.observe(document.querySelector('.header-actions'), {attributes: true, attributeFilter: ['data-chat-icon', 'class'], childList: true, subtree: true});
        observer.observe(byId('chat-content'), {childList: true});
        document.querySelectorAll('#chat-form [data-chat-icon], .galaxy-nav [data-chat-icon]').forEach(decorateIcon);
        updateHeader();
    }
    function unmount() {
        if (!active) return;
        active = false;
        observer?.disconnect();
        observer = null;
        headerObserver?.disconnect();
        headerObserver = null;
        clearTimeout(clockTimer);
        moves.reverse().forEach(([node, anchor]) => { anchor.replaceWith(node); });
        moves = [];
        owned.forEach(node => node.remove());
        owned = [];
        byId('chat-content')?.classList.remove('galaxy-welcome-only');
        byId('user-input').placeholder = originalPlaceholder;
        document.querySelector('.app-header .logo')?.style.removeProperty('--galaxy-mark');
    }
    function sync() { root.dataset.theme === 'galaxy' ? mount() : unmount(); }
    window.addEventListener('aurago:themechange', sync);
    document.addEventListener('visibilitychange', clock);
    sync();
})();
