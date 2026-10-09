(function () {
    'use strict';

    const CONTENT_PREFIX = '/api/desktop/local-wikipedia/content/';
    const ICONS = {
        back: '<path d="M15 18l-6-6 6-6"/>',
        forward: '<path d="M9 18l6-6-6-6"/>',
        home: '<path d="M3 11.5L12 4l9 7.5"/><path d="M5.5 10v10h13V10"/><path d="M10 20v-5h4v5"/>',
        random: '<path d="M16 4h4v4"/><path d="M4 20L20 4"/><path d="M20 16v4h-4"/><path d="M15 15l5 5"/><path d="M4 4l5 5"/>',
        search: '<circle cx="11" cy="11" r="6.5"/><path d="M20 20l-4.2-4.2"/>',
        external: '<path d="M14 4h6v6"/><path d="M20 4l-8.5 8.5"/><path d="M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5"/>',
        book: '<path d="M2 5h6a4 4 0 0 1 4 4v11a3 3 0 0 0-3-3H2z"/><path d="M22 5h-6a4 4 0 0 0-4 4v11a3 3 0 0 1 3-3h7z"/>'
    };

    function esc(value) {
        return String(value ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
    }

    function icon(name) {
        return '<svg class="lw-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">' + (ICONS[name] || ICONS.search) + '</svg>';
    }

    // contentURL returns the frame address of a content path, or '' when the path
    // cannot be addressed safely (browsers would normalise dot segments away).
    function contentURL(path) {
        const value = String(path || '');
        if (!value) return '';
        const parts = value.split('/');
        if (parts.some(part => part === '.' || part === '..')) return '';
        return CONTENT_PREFIX + parts.map(part => encodeURIComponent(part)).join('/');
    }

    function isContentPath(pathname) {
        return String(pathname || '').startsWith(CONTENT_PREFIX);
    }

    function pathFromLocation(pathname) {
        const value = String(pathname || '');
        if (!isContentPath(value)) return '';
        try {
            return value.slice(CONTENT_PREFIX.length).split('/').map(part => decodeURIComponent(part)).join('/');
        } catch (_) {
            return '';
        }
    }

    // percent accepts a 0..1 fraction (Local LLM convention) or a 0..100 value.
    function percent(status) {
        const value = Number(status && status.progress);
        if (!Number.isFinite(value) || value <= 0) return 0;
        return Math.min(100, Math.floor(value <= 1 ? value * 100 : value));
    }

    // Codes of a finished or paused operation that left the installed edition in
    // place; only an administrator can do something about them.
    const UPDATE_FAILURE_CODES = ['download_failed', 'checksum_mismatch', 'insufficient_disk_space', 'zim_unreadable'];

    // readable: the server serves an installed edition. It stays true while an
    // update downloads or after one failed, so the app never guesses it from the
    // state or from the presence of an edition.
    function readable(status) {
        return !!status && status.readable === true;
    }

    // stateKind picks the full-page state of an edition that is not readable.
    // Texts are always derived from the state and the error code, never from the
    // server's English recommendation.
    function stateKind(status) {
        if (!status) return 'failed';
        if (status.loading === true) return 'loading';
        const state = status.state;
        if (state === 'downloading' || state === 'verifying') return state;
        if (status.error_code === 'state_unreadable') return 'state_unreadable';
        if (state === 'interrupted') return 'interrupted';
        if (state === 'error') return status.error_code === 'zim_unreadable' && status.edition ? 'error' : 'install_failed';
        if (state === 'not_installed' || state === 'ready') return 'not_installed';
        return 'failed';
    }

    function monthLabel(date, lang) {
        const match = /^(\d{4})-(\d{2})/.exec(String(date || ''));
        if (!match) return String(date || '');
        try {
            return new Intl.DateTimeFormat(lang || 'en', { year: 'numeric', month: 'long', timeZone: 'UTC' }).format(new Date(Date.UTC(Number(match[1]), Number(match[2]) - 1, 1)));
        } catch (_) {
            return match[1] + '-' + match[2];
        }
    }

    function languageLabel(code, lang) {
        try {
            return new Intl.DisplayNames([lang || 'en'], { type: 'language' }).of(code) || code;
        } catch (_) {
            return code;
        }
    }

    function variantLabel(variant, t) {
        if (variant === 'maxi') return t('desktop.local_wikipedia_variant_maxi');
        if (variant === 'nopic') return t('desktop.local_wikipedia_variant_nopic');
        return '';
    }

    function toolButton(iconName, action, label) {
        return '<button class="lw-tool" type="button" data-action="' + esc(action) + '" aria-label="' + esc(label) + '" title="' + esc(label) + '">' + icon(iconName) + '</button>';
    }

    function settingsButton(t) {
        return '<button class="lw-button lw-primary" type="button" data-action="settings">' + icon('external') + '<span>' + esc(t('desktop.local_wikipedia_open_settings')) + '</span></button>';
    }

    function shell(t, ids) {
        const searchLabel = esc(t('desktop.local_wikipedia_search_button'));
        return '<div class="lw-app">'
            + '<header class="lw-toolbar">'
            + '<div class="lw-nav" role="toolbar" aria-label="' + esc(t('desktop.local_wikipedia_toolbar')) + '">'
            + toolButton('back', 'back', t('desktop.back'))
            + toolButton('forward', 'forward', t('desktop.forward'))
            + toolButton('home', 'main', t('desktop.local_wikipedia_main_page'))
            + toolButton('random', 'random', t('desktop.local_wikipedia_random'))
            + '</div>'
            + '<p class="lw-title" aria-live="polite"></p>'
            + '<form class="lw-search" role="search" autocomplete="off">'
            + '<div class="lw-combo">'
            + '<input class="lw-input" type="search" name="q" maxlength="200" spellcheck="false" autocomplete="off" role="combobox" aria-autocomplete="list" aria-expanded="false" aria-controls="' + esc(ids.list) + '" aria-label="' + esc(t('desktop.local_wikipedia_search_label')) + '" placeholder="' + esc(t('desktop.local_wikipedia_search_placeholder')) + '">'
            + '<ul class="lw-suggest" id="' + esc(ids.list) + '" role="listbox" aria-label="' + esc(t('desktop.local_wikipedia_suggestions')) + '" hidden></ul>'
            + '</div>'
            + '<button class="lw-submit" type="submit" aria-label="' + searchLabel + '">' + icon('search') + '<span>' + searchLabel + '</span></button>'
            + '</form>'
            + '</header>'
            + '<div class="lw-banners"></div>'
            + '<main class="lw-body">'
            + '<section class="lw-view lw-loading" data-view="loading"><span class="lw-spinner" aria-hidden="true"></span><span>' + esc(t('desktop.local_wikipedia_loading')) + '</span></section>'
            + '<section class="lw-view lw-state" data-view="state" hidden></section>'
            + '<section class="lw-view lw-results" data-view="results" hidden></section>'
            + '<section class="lw-view lw-article" data-view="article" hidden>'
            + '<iframe class="lw-frame" sandbox="allow-same-origin allow-popups allow-popups-to-escape-sandbox" referrerpolicy="no-referrer" title="' + esc(t('desktop.local_wikipedia_loading')) + '"></iframe>'
            + '<div class="lw-frame-error" role="alert" hidden></div>'
            + '</section>'
            + '</main>'
            + '<footer class="lw-footer"></footer>'
            + '</div>';
    }

    function stateView(kind, status, t, canManage) {
        const pct = percent(status);
        const texts = {
            loading: [t('desktop.local_wikipedia_edition_loading_title'), t('desktop.local_wikipedia_edition_loading_text')],
            not_installed: [t('desktop.local_wikipedia_not_installed_title'), canManage ? t('desktop.local_wikipedia_not_installed_admin') : t('desktop.local_wikipedia_not_installed_user')],
            downloading: [t('desktop.local_wikipedia_downloading_title'), t('desktop.local_wikipedia_downloading_text', { percent: pct })],
            verifying: [t('desktop.local_wikipedia_verifying_title'), t('desktop.local_wikipedia_verifying_text')],
            interrupted: [t('desktop.local_wikipedia_interrupted_title'), canManage ? t('desktop.local_wikipedia_interrupted_admin') : t('desktop.local_wikipedia_interrupted_user')],
            error: [t('desktop.local_wikipedia_error_title'), canManage ? t('desktop.local_wikipedia_error_admin') : t('desktop.local_wikipedia_error_user')],
            state_unreadable: [t('desktop.local_wikipedia_error_title'), canManage ? t('desktop.local_wikipedia_state_error_admin') : t('desktop.local_wikipedia_state_error_user')],
            install_failed: [t('desktop.local_wikipedia_install_failed_title'), canManage ? t('desktop.local_wikipedia_install_failed_admin') : t('desktop.local_wikipedia_install_failed_user')],
            disabled: [t('desktop.local_wikipedia_disabled_title'), canManage ? t('desktop.local_wikipedia_disabled_admin') : t('desktop.local_wikipedia_disabled_user')],
            failed: [t('desktop.local_wikipedia_failed_title'), t('desktop.local_wikipedia_failed_text')]
        };
        const known = Object.prototype.hasOwnProperty.call(texts, kind) ? kind : 'failed';
        const [title, text] = texts[known];
        const progressLabel = esc(t('desktop.local_wikipedia_progress'));
        let extra = '';
        if (known === 'downloading') extra = '<progress class="lw-progress" max="100" value="' + pct + '" aria-label="' + progressLabel + '"></progress>';
        if (known === 'verifying') extra = '<progress class="lw-progress" aria-label="' + progressLabel + '"></progress>';
        if (known === 'loading') extra = '<span class="lw-spinner" aria-hidden="true"></span>';
        const actions = [];
        if (canManage && ['not_installed', 'interrupted', 'error', 'state_unreadable', 'install_failed', 'disabled'].includes(known)) actions.push(settingsButton(t));
        if (known === 'failed') actions.push('<button class="lw-button" type="button" data-action="retry">' + esc(t('desktop.local_wikipedia_retry')) + '</button>');
        return '<div class="lw-state-card" data-state="' + esc(known) + '" role="status">'
            + '<span class="lw-state-mark" aria-hidden="true">' + icon('book') + '</span>'
            + '<h2>' + esc(title) + '</h2>'
            + '<p>' + esc(text) + '</p>'
            + extra
            + (actions.length ? '<div class="lw-actions">' + actions.join('') + '</div>' : '')
            + '</div>';
    }

    function banner(kind, text, action) {
        return '<div class="lw-banner" data-kind="' + esc(kind) + '" role="status"><span>' + esc(text) + '</span>' + (action || '') + '</div>';
    }

    // staleBanner tells the reader that the status could not be refreshed while an
    // article stays on screen.
    function staleBanner(t) {
        return banner('warn', t('desktop.local_wikipedia_status_stale'), '');
    }

    function banners(status, t, lang, canManage) {
        if (!readable(status)) return '';
        const out = [];
        if (status.state === 'downloading' || status.state === 'verifying') {
            out.push(banner('info', t('desktop.local_wikipedia_update_downloading', { percent: percent(status) }), ''));
        } else if (status.state === 'interrupted') {
            out.push(banner('info', t('desktop.local_wikipedia_update_interrupted'), canManage ? settingsButton(t) : ''));
        } else {
            // A failed update keeps state "ready" and reports its code while the old edition is served.
            if (canManage && UPDATE_FAILURE_CODES.includes(status.error_code)) out.push(banner('warn', t('desktop.local_wikipedia_update_failed'), settingsButton(t)));
            if (status.update_available && status.update_available.date) {
                out.push(banner('info', t('desktop.local_wikipedia_update_available', { date: monthLabel(status.update_available.date, lang) }), canManage ? settingsButton(t) : ''));
            }
        }
        if (status.fulltext === false) out.push(banner('hint', t('desktop.local_wikipedia_title_only'), ''));
        return out.join('');
    }

    function results(list, query, t, state) {
        const heading = '<h2 class="lw-results-title">' + esc(t('desktop.local_wikipedia_results_title', { query })) + '</h2>';
        if (state === 'loading') return heading + '<p class="lw-muted" role="status">' + esc(t('desktop.local_wikipedia_searching')) + '</p>';
        if (state === 'error') return heading + '<p class="lw-error" role="alert">' + esc(t('desktop.local_wikipedia_search_failed')) + '</p>';
        const items = Array.isArray(list) ? list : [];
        if (!items.length) return heading + '<p class="lw-muted" role="status">' + esc(t('desktop.local_wikipedia_no_results')) + '</p>';
        return heading + '<ol class="lw-result-list">' + items.map(hit => '<li><button class="lw-result" type="button" data-path="' + esc(hit.path) + '"><strong>' + esc(hit.title || hit.path) + '</strong>' + (hit.snippet ? '<span>' + esc(hit.snippet) + '</span>' : '') + '</button></li>').join('') + '</ol>';
    }

    function suggestions(list, active, listId) {
        return (Array.isArray(list) ? list : []).map((ref, index) => '<li class="lw-option" role="option" id="' + esc(listId + '-' + index) + '" data-index="' + index + '" aria-selected="' + (index === active ? 'true' : 'false') + '">' + esc(ref.title || ref.path) + '</li>').join('');
    }

    function footer(edition, t, lang) {
        if (!edition) return '';
        const parts = [languageLabel(edition.language, lang), variantLabel(edition.variant, t)];
        if (edition.date) parts.push(t('desktop.local_wikipedia_edition_date', { date: monthLabel(edition.date, lang) }));
        return '<span class="lw-footer-label">' + esc(t('desktop.local_wikipedia_edition')) + '</span><span>' + esc(parts.filter(Boolean).join(' · ')) + '</span>';
    }

    // toolbarTarget maps an arrow, Home or End key to the index of the toolbar
    // button that takes focus next (-1 when the key does not move focus). `current`
    // is the index of the focused button among `count` enabled buttons, or -1.
    function toolbarTarget(key, current, count, rtl) {
        if (!(count > 0)) return -1;
        if (key === 'Home') return 0;
        if (key === 'End') return count - 1;
        const next = rtl ? 'ArrowLeft' : 'ArrowRight';
        const previous = rtl ? 'ArrowRight' : 'ArrowLeft';
        if (key === next) return (current + 1) % count;
        if (key === previous) return (current <= 0 ? count : current) - 1;
        return -1;
    }

    // adoptLinks applies the reader's link rules to a loaded article. The frame
    // sandbox has no scripts, so links are the only active content left:
    //  - ping and attributionsrc make the browser send a request on click, to any
    //    address and with the session cookie, so they are removed from every link;
    //  - links into the article route stay, other links of this origin lose their href;
    //  - http(s) links open in a new tab without opener or referrer, mailto stays,
    //    every other scheme (javascript:, data:, ...) and unparsable links lose their href.
    function adoptLinks(doc, origin) {
        doc.querySelectorAll('a, area').forEach(link => {
            link.removeAttribute('ping');
            link.removeAttribute('attributionsrc');
            const href = link.getAttribute('href');
            if (href === null || href === undefined) return;
            let target;
            try {
                target = new URL(href, doc.baseURI);
            } catch (_) {
                link.removeAttribute('href');
                return;
            }
            if (target.origin === origin) {
                if (!isContentPath(target.pathname)) link.removeAttribute('href');
                return;
            }
            if (target.protocol === 'http:' || target.protocol === 'https:') {
                link.setAttribute('target', '_blank');
                link.setAttribute('rel', 'noopener noreferrer');
            } else if (target.protocol !== 'mailto:') {
                link.removeAttribute('href');
            }
        });
    }

    window.LocalWikipediaViews = { CONTENT_PREFIX, esc, icon, contentURL, isContentPath, pathFromLocation, percent, readable, stateKind, monthLabel, shell, stateView, banners, staleBanner, results, suggestions, footer, toolbarTarget, adoptLinks };
})();
