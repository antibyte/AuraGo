(function () {
    'use strict';

    const instances = new Map();
    const BASE = '/api/desktop/local-wikipedia';
    const SETTINGS_URL = '/config#local_wikipedia';
    const SUGGEST_DELAY_MS = 200;
    const POLL_LOADING_MS = 1000;
    const POLL_RETRY_MS = 5000;
    const POLL_ACTIVE_MS = 3000;
    const POLL_IDLE_MS = 15000;
    const POLL_READY_MS = 300000;
    const HISTORY_LIMIT = 200;
    const SEARCH_LIMIT = 20;

    function render(host, windowId, context) {
        dispose(windowId);
        const ctx = context || {};
        const v = window.LocalWikipediaViews;
        const t = (key, params) => (typeof ctx.t === 'function' ? ctx.t(key, params) : key);
        const lang = () => document.documentElement.lang || 'en';
        const controller = new AbortController();
        const listId = 'lw-suggest-' + String(windowId).replace(/[^A-Za-z0-9_-]/g, '_');
        const st = {
            host, controller, disposed: false, status: null, canManage: false, view: 'loading',
            query: '', results: [], resultsState: 'done', searchGen: 0,
            suggestions: [], active: -1, suggestTimer: 0, suggestAbort: null, suggestGen: 0,
            history: [], index: -1, pending: null, frameLoaded: false, frameFailed: false, focusFrame: false,
            busy: false, pollTimer: 0, bannersHTML: '', footerHTML: '', stateHTML: '',
            statusStale: false, failedIndex: -1, toolStop: 'back'
        };
        instances.set(windowId, st);
        host.innerHTML = v.shell(t, { list: listId });
        const $ = selector => host.querySelector(selector);
        const input = $('.lw-input');
        const list = $('.lw-suggest');
        const frame = $('.lw-frame');
        const titleNode = $('.lw-title');
        const nav = $('.lw-nav');

        function request(path, options) {
            if (typeof ctx.api !== 'function') return Promise.reject(new Error('Desktop API unavailable'));
            return ctx.api(BASE + path, Object.assign({ signal: controller.signal }, options || {}));
        }

        function isAbort(err) {
            return controller.signal.aborted || !!(err && err.name === 'AbortError');
        }

        function setTitle(text) {
            titleNode.textContent = text || '';
            frame.title = text ? t('desktop.local_wikipedia_article_frame', { title: text }) : t('desktop.local_wikipedia_loading');
        }

        function setDisabled(action, disabled) {
            const button = host.querySelector('.lw-tool[data-action="' + action + '"]');
            if (button) button.disabled = disabled;
        }

        function toolButtons() {
            return Array.from(nav.querySelectorAll('.lw-tool'));
        }

        // Roving tabindex: exactly one enabled tool is the tab stop and the arrow keys
        // move between the enabled tools. refocus moves focus to the tab stop when the
        // focused tool has just been disabled.
        function syncToolbar(refocus) {
            const tools = toolButtons();
            const enabled = tools.filter(button => !button.disabled);
            const stop = enabled.find(button => button.dataset.action === st.toolStop) || enabled[0] || null;
            tools.forEach(button => button.setAttribute('tabindex', button === stop ? '0' : '-1'));
            if (stop) st.toolStop = stop.dataset.action;
            if (refocus && stop) stop.focus();
        }

        // backTarget is the history index Back leads to, or -1. From the results or after a
        // new navigation failed it returns to the last article that loaded; when that entry
        // itself failed to load it goes one entry further back instead of retrying it.
        function backTarget() {
            if (st.index < 0) return -1;
            if (st.view === 'results') return st.index;
            if (st.view !== 'article') return -1;
            if (st.frameFailed && st.failedIndex !== st.index) return st.index;
            return st.index > 0 ? st.index - 1 : -1;
        }

        function updateNav() {
            const ready = v.readable(st.status);
            const focused = nav.contains(document.activeElement) ? document.activeElement : null;
            const forward = ready && st.view === 'article' && !st.frameFailed && st.index < st.history.length - 1;
            setDisabled('back', !(ready && backTarget() >= 0));
            setDisabled('forward', !forward);
            // Main and random stay enabled while a request runs (openRef ignores the extra click), so keyboard focus stays on them.
            setDisabled('main', !ready);
            setDisabled('random', !ready);
            input.disabled = !ready;
            $('.lw-submit').disabled = !ready;
            syncToolbar(!!(focused && focused.disabled));
        }

        function showView(name) {
            st.view = name;
            host.querySelectorAll('.lw-view').forEach(section => { section.hidden = section.dataset.view !== name; });
            if (name !== 'article') setTitle('');
            updateNav();
        }

        // renderChrome rewrites banners and footer only when they change, so polling
        // never steals focus from a banner button.
        function renderChrome() {
            const banners = (st.statusStale ? v.staleBanner(t) : '') + v.banners(st.status, t, lang(), st.canManage);
            const footer = v.readable(st.status) ? v.footer(st.status.edition, t, lang()) : '';
            if (banners !== st.bannersHTML) {
                st.bannersHTML = banners;
                $('.lw-banners').innerHTML = banners;
            }
            if (footer !== st.footerHTML) {
                st.footerHTML = footer;
                $('.lw-footer').innerHTML = footer;
            }
        }

        function showState(kind) {
            closeSuggestions();
            st.pending = null;
            const html = v.stateView(kind, st.status, t, st.canManage);
            if (html !== st.stateHTML) {
                st.stateHTML = html;
                $('.lw-state').innerHTML = html;
            }
            showView('state');
        }

        function schedulePoll(delay) {
            clearTimeout(st.pollTimer);
            if (!st.disposed) st.pollTimer = setTimeout(refreshStatus, delay);
        }

        // resumeReading leaves a state screen: after an outage the reader returns to the
        // article that was open; only a fresh window starts at the main page.
        function resumeReading() {
            const path = st.index >= 0 ? st.history[st.index] : '';
            if (path) navigate(path, 'history', false);
            else openRef('/main');
        }

        function applyStatus(status) {
            st.status = status;
            st.statusStale = false;
            st.canManage = !!status.can_manage;
            renderChrome();
            const active = status.state === 'downloading' || status.state === 'verifying';
            if (v.readable(status)) {
                if (st.view === 'state' || st.view === 'loading') resumeReading();
                updateNav();
                schedulePoll(active ? POLL_ACTIVE_MS : POLL_READY_MS);
                return;
            }
            // The first background load of the installed edition is still running: poll quickly until it is done.
            const kind = v.stateKind(status);
            showState(kind);
            schedulePoll(kind === 'loading' ? POLL_LOADING_MS : (active ? POLL_ACTIVE_MS : POLL_IDLE_MS));
        }

        async function refreshStatus() {
            clearTimeout(st.pollTimer);
            try {
                const status = await request('/status');
                if (st.disposed) return;
                applyStatus(status || {});
            } catch (err) {
                if (st.disposed || isAbort(err)) return;
                const body = (err && err.body) || {};
                if (body.code !== 'disabled' && v.readable(st.status) && (st.view === 'article' || st.view === 'results')) {
                    // One failed poll must not replace what the reader is looking at: keep it and say so.
                    st.statusStale = true;
                    renderChrome();
                    schedulePoll(POLL_RETRY_MS);
                    return;
                }
                st.status = null;
                st.statusStale = false;
                if (body.code === 'disabled') {
                    st.canManage = !!body.can_manage;
                    showState('disabled');
                } else {
                    showState('failed');
                }
                renderChrome();
                schedulePoll(POLL_IDLE_MS);
            }
        }

        function showFrameError(text) {
            st.frameFailed = true;
            const node = $('.lw-frame-error');
            node.textContent = text;
            node.hidden = false;
            frame.classList.add('is-failed');
            setTitle('');
            updateNav();
        }

        // failLoad reports a failed load. A failed back or forward step marks its history
        // entry, so Back can skip it instead of loading it again.
        function failLoad(text, mode) {
            if (mode === 'history') st.failedIndex = st.index;
            showFrameError(text);
        }

        function hideFrameError() {
            st.frameFailed = false;
            $('.lw-frame-error').hidden = true;
            frame.classList.remove('is-failed');
        }

        // navigate loads a content path; mode 'push' adds a history entry on load,
        // 'history' replaces the current entry (back/forward and redirects).
        function navigate(path, mode, focusFrame) {
            closeSuggestions();
            st.failedIndex = -1;
            showView('article');
            const url = v.contentURL(path);
            if (!url) {
                failLoad(t('desktop.local_wikipedia_article_missing'), mode);
                return;
            }
            hideFrameError();
            setTitle('');
            st.pending = { path, mode };
            st.focusFrame = !!focusFrame;
            try {
                if (st.frameLoaded && frame.contentWindow) frame.contentWindow.location.replace(url);
                else frame.src = url;
            } catch (_) {
                frame.src = url;
            }
            updateNav();
        }

        async function openRef(route) {
            if (st.busy) return;
            st.busy = true;
            updateNav();
            try {
                const ref = await request(route);
                if (!st.disposed && ref && ref.path) navigate(ref.path, 'push', false);
            } catch (err) {
                if (st.disposed || isAbort(err)) return;
                showView('article');
                showFrameError(t('desktop.local_wikipedia_article_failed'));
            } finally {
                if (!st.disposed) {
                    st.busy = false;
                    updateNav();
                }
            }
        }

        function goBack() {
            const target = backTarget();
            if (target < 0) return;
            st.index = target;
            navigate(st.history[st.index], 'history', false);
        }

        function goForward() {
            if (st.index < st.history.length - 1) {
                st.index += 1;
                navigate(st.history[st.index], 'history', false);
            }
        }

        function recordHistory(path, pending) {
            if (pending && pending.mode === 'history' && st.index >= 0) {
                st.history[st.index] = path;
                return;
            }
            if (st.index >= 0 && st.history[st.index] === path) return;
            st.history = st.history.slice(0, st.index + 1);
            st.history.push(path);
            if (st.history.length > HISTORY_LIMIT) st.history.shift();
            st.index = st.history.length - 1;
        }

        function readTitle(doc, path) {
            const heading = doc.querySelector('h1');
            const text = String(doc.title || '').trim() || String((heading && heading.textContent) || '').trim();
            return text || path.replace(/_/g, ' ');
        }

        function onFrameLoad() {
            if (st.disposed) return;
            let doc = null;
            try {
                doc = frame.contentDocument;
            } catch (_) {
                doc = null;
            }
            const href = doc && doc.location ? doc.location.href : '';
            if (href === 'about:blank') return;
            const pending = st.pending;
            const focusFrame = st.focusFrame;
            st.pending = null;
            st.focusFrame = false;
            const mode = pending && pending.mode;
            if (!href) {
                failLoad(t('desktop.local_wikipedia_article_failed'), mode);
                return;
            }
            st.frameLoaded = true;
            const marker = doc.querySelector('meta[name="aurago-local-wikipedia-error"]');
            if (marker) {
                failLoad(marker.getAttribute('content') === 'not_found' ? t('desktop.local_wikipedia_article_missing') : t('desktop.local_wikipedia_article_failed'), mode);
                return;
            }
            const path = v.pathFromLocation(doc.location.pathname);
            if (!path) {
                failLoad(t('desktop.local_wikipedia_article_failed'), mode);
                return;
            }
            st.failedIndex = -1;
            recordHistory(path, pending);
            hideFrameError();
            // ZIM scripts never run (the frame sandbox omits the scripts permission); links are the only active content.
            v.adoptLinks(doc, window.location.origin);
            // A load that finishes after the reader moved on (results, a state screen) only
            // records its history entry; it never takes the view back to the article.
            if (st.view !== 'article') {
                updateNav();
                return;
            }
            setTitle(readTitle(doc, path));
            updateNav();
            if (focusFrame) frame.focus();
        }

        function closeSuggestions() {
            clearTimeout(st.suggestTimer);
            st.suggestGen += 1;
            if (st.suggestAbort) {
                st.suggestAbort.abort();
                st.suggestAbort = null;
            }
            st.suggestions = [];
            st.active = -1;
            list.innerHTML = '';
            list.hidden = true;
            input.setAttribute('aria-expanded', 'false');
            input.removeAttribute('aria-activedescendant');
        }

        function drawSuggestions() {
            if (!st.suggestions.length) {
                closeSuggestions();
                return;
            }
            list.innerHTML = v.suggestions(st.suggestions, st.active, listId);
            list.hidden = false;
            input.setAttribute('aria-expanded', 'true');
            if (st.active >= 0) {
                input.setAttribute('aria-activedescendant', listId + '-' + st.active);
                const option = list.querySelector('[aria-selected="true"]');
                if (option) option.scrollIntoView({ block: 'nearest' });
            } else {
                input.removeAttribute('aria-activedescendant');
            }
        }

        function scheduleSuggestions() {
            clearTimeout(st.suggestTimer);
            const query = input.value.trim();
            if (!query) {
                closeSuggestions();
                return;
            }
            st.suggestTimer = setTimeout(() => loadSuggestions(query), SUGGEST_DELAY_MS);
        }

        async function loadSuggestions(query, activateFirst) {
            clearTimeout(st.suggestTimer);
            if (st.suggestAbort) st.suggestAbort.abort();
            const local = new AbortController();
            st.suggestAbort = local;
            const generation = ++st.suggestGen;
            try {
                const data = await request('/suggest?q=' + encodeURIComponent(query), { signal: local.signal });
                if (st.disposed || generation !== st.suggestGen || input.value.trim() !== query) return;
                st.suggestions = Array.isArray(data && data.results) ? data.results.slice(0, 10) : [];
                st.active = activateFirst && st.suggestions.length ? 0 : -1;
                drawSuggestions();
            } catch (err) {
                if (st.disposed || local.signal.aborted || isAbort(err) || generation !== st.suggestGen) return;
                closeSuggestions();
            }
        }

        function chooseSuggestion(index) {
            const ref = st.suggestions[index];
            if (!ref) return;
            input.value = ref.title || ref.path;
            navigate(ref.path, 'push', true);
        }

        function drawResults() {
            $('.lw-results').innerHTML = v.results(st.results, st.query, t, st.resultsState);
        }

        async function search(text) {
            const query = String(text || '').trim();
            if (!query || !v.readable(st.status)) return;
            closeSuggestions();
            st.query = query;
            const generation = ++st.searchGen;
            st.results = [];
            st.resultsState = 'loading';
            drawResults();
            showView('results');
            try {
                const data = await request('/search?q=' + encodeURIComponent(query) + '&limit=' + SEARCH_LIMIT);
                if (st.disposed || generation !== st.searchGen) return;
                st.results = Array.isArray(data && data.results) ? data.results : [];
                st.resultsState = 'done';
            } catch (err) {
                if (st.disposed || isAbort(err) || generation !== st.searchGen) return;
                st.results = [];
                st.resultsState = 'error';
            }
            drawResults();
        }

        function onInputKeydown(event) {
            const open = !list.hidden && st.suggestions.length > 0;
            if (!open) {
                // ArrowDown reopens suggestions that were closed with Escape or by leaving the field.
                const query = input.value.trim();
                if (event.key === 'ArrowDown' && query && !input.disabled) {
                    event.preventDefault();
                    loadSuggestions(query, true);
                }
                return;
            }
            if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
                event.preventDefault();
                const count = st.suggestions.length;
                st.active = event.key === 'ArrowDown' ? (st.active + 1) % count : (st.active <= 0 ? count - 1 : st.active - 1);
                drawSuggestions();
            } else if (event.key === 'Enter' && st.active >= 0) {
                event.preventDefault();
                chooseSuggestion(st.active);
            } else if (event.key === 'Escape') {
                event.preventDefault();
                event.stopPropagation();
                closeSuggestions();
            }
        }

        input.addEventListener('input', scheduleSuggestions, { signal: controller.signal });
        input.addEventListener('keydown', onInputKeydown, { signal: controller.signal });
        $('.lw-search').addEventListener('submit', event => {
            event.preventDefault();
            search(input.value);
        }, { signal: controller.signal });
        list.addEventListener('mousedown', event => event.preventDefault(), { signal: controller.signal });
        list.addEventListener('click', event => {
            const option = event.target.closest('.lw-option');
            if (option) chooseSuggestion(Number(option.dataset.index));
        }, { signal: controller.signal });
        $('.lw-combo').addEventListener('focusout', event => {
            if (!event.relatedTarget || !$('.lw-combo').contains(event.relatedTarget)) closeSuggestions();
        }, { signal: controller.signal });
        host.addEventListener('click', event => {
            const target = event.target.closest('button[data-action], button[data-path]');
            if (!target || target.disabled || !host.contains(target)) return;
            if (target.dataset.path) {
                navigate(target.dataset.path, 'push', true);
                return;
            }
            switch (target.dataset.action) {
            case 'back': goBack(); break;
            case 'forward': goForward(); break;
            case 'main': openRef('/main'); break;
            case 'random': openRef('/random'); break;
            case 'retry': showView('loading'); refreshStatus(); break;
            case 'settings': window.open(SETTINGS_URL, '_blank', 'noopener'); break;
            default: break;
            }
        }, { signal: controller.signal });
        nav.addEventListener('focusin', event => {
            const button = event.target.closest('.lw-tool');
            if (button && !button.disabled) {
                st.toolStop = button.dataset.action;
                syncToolbar(false);
            }
        }, { signal: controller.signal });
        nav.addEventListener('keydown', event => {
            if (event.ctrlKey || event.altKey || event.metaKey) return;
            const tools = toolButtons().filter(button => !button.disabled);
            const next = v.toolbarTarget(event.key, tools.indexOf(event.target.closest('.lw-tool')), tools.length, window.getComputedStyle(host).direction === 'rtl');
            if (next < 0) return;
            event.preventDefault();
            st.toolStop = tools[next].dataset.action;
            syncToolbar(false);
            tools[next].focus();
        }, { signal: controller.signal });
        frame.addEventListener('load', onFrameLoad, { signal: controller.signal });
        setTitle('');
        updateNav();
        refreshStatus();
    }

    function dispose(windowId) {
        const st = instances.get(windowId);
        if (!st) return;
        st.disposed = true;
        clearTimeout(st.pollTimer);
        clearTimeout(st.suggestTimer);
        if (st.suggestAbort) st.suggestAbort.abort();
        st.controller.abort();
        const frame = st.host && st.host.querySelector('.lw-frame');
        if (frame) frame.src = 'about:blank';
        instances.delete(windowId);
    }

    window.LocalWikipediaApp = { render, dispose };
})();
