// AuraGo dashboard – summary feed (window.DashSummary). Classic script, loaded with defer after dash-api.js and
// before dash-overview.js and main.js. One GET /api/dashboard/summary answer (through DashAPI, 15 s cache) feeds
// the overview tiles, the sidebar status block and the overview/problems/automations badges on every section.
// Refreshes: once at init, on refresh() (overview loader, section refresh button), 2 s after the first of a
// burst of SSE triggers (scheduleRefresh, cache bypassed) and 60 s after the last fetch while the page is
// visible. Nothing runs while the page is hidden; on becoming visible the feed refreshes once when an SSE
// trigger was missed or the last fetch is 60 s old. The helpers in the first block are pure and nothing touches
// the DOM at load time, so the file also runs in a Node vm context (ui/testdata/dash-summary.cjs).
(function () {
    'use strict';

    const SUMMARY_URL = '/api/dashboard/summary';
    const CACHE_TTL_MS = 15000;
    const DEBOUNCE_MS = 2000;
    const POLL_MS = 60000;
    const BADGE_SECTIONS = Object.freeze(['overview', 'problems', 'automations']);

    // ── Pure helpers ─────────────────────────────────────────────────────────────

    function attentionItems(summary) {
        const list = summary && Array.isArray(summary.attention) ? summary.attention : [];
        return list.filter(item => item && typeof item === 'object');
    }

    // Severity counts of the returned attention rows. The server sorts by severity and keeps 8 rows, so when
    // rows were cut off (attention_more > 0) the count of the last row's severity is only a lower bound.
    function attentionCounts(summary) {
        const items = attentionItems(summary);
        const counts = { critical: 0, warning: 0, info: 0, criticalOpen: false, warningOpen: false };
        items.forEach(item => {
            if (item.severity === 'critical' || item.severity === 'warning') counts[item.severity] += 1;
            else counts.info += 1;
        });
        const cut = !!summary && Number(summary.attention_more) > 0;
        const last = items.length ? items[items.length - 1].severity : '';
        counts.criticalOpen = cut && last === 'critical';
        counts.warningOpen = cut && last === 'warning';
        return counts;
    }

    // Overview badge: critical + warning rows; "8+" when rows were cut off after a critical or warning row.
    function overviewBadgeText(summary) {
        const counts = attentionCounts(summary);
        const total = counts.critical + counts.warning;
        if (total === 0) return '';
        return String(total) + (counts.criticalOpen || counts.warningOpen ? '+' : '');
    }

    function countText(value) {
        const n = Math.floor(Number(value));
        return Number.isFinite(n) && n > 0 ? String(n) : '';
    }

    // Badge text per sidebar section; '' hides the badge.
    function badgeTexts(summary) {
        const badges = summary && summary.badges && typeof summary.badges === 'object' ? summary.badges : {};
        return {
            overview: overviewBadgeText(summary),
            problems: countText(badges.problems),
            automations: countText(badges.automations),
        };
    }

    function overallState(summary) {
        const overall = summary ? summary.overall : '';
        return overall === 'ok' || overall === 'warning' || overall === 'critical' ? overall : '';
    }

    // Pill text: "All good" or "1 critical, 2 warning(s)" from the attention rows. translate is t(key, params).
    function overallText(summary, translate) {
        const state = overallState(summary);
        if (!state) return translate('dashboard.overview.pill_unavailable');
        if (state === 'ok') return translate('dashboard.overview.pill_ok');
        const counts = attentionCounts(summary);
        const parts = [];
        if (counts.critical > 0) parts.push(translate('dashboard.overview.pill_critical', { n: counts.critical + (counts.criticalOpen ? '+' : '') }));
        if (counts.warning > 0) parts.push(translate('dashboard.overview.pill_warning', { n: counts.warning + (counts.warningOpen ? '+' : '') }));
        return parts.length ? parts.join(', ') : translate('dashboard.overview.severity_' + state);
    }

    // ── Feed state and schedule ─────────────────────────────────────────────────

    let latestSummary = null;
    let lastStatus = 0;           // 200 after a good answer, else the HTTP status (0 = network error)
    let lastFetchAt = 0;
    let requestSeq = 0;
    let appliedSeq = 0;
    let inFlight = 0;
    let debounceTimer = null;
    let pollTimer = null;
    let missedWhileHidden = false;
    let liveState = 'connecting';  // 'connecting' | 'on' | 'off' (AuraSSE open/error, set by dashboard-events.js)
    let initialized = false;
    const listeners = [];

    function pageVisible() {
        return typeof document === 'undefined' || document.visibilityState !== 'hidden';
    }

    function clearPoll() {
        if (pollTimer) clearTimeout(pollTimer);
        pollTimer = null;
    }

    // The next poll runs `delay` ms from now (default 60 s); never before init and never while hidden.
    function schedulePoll(delay) {
        clearPoll();
        if (!initialized || !pageVisible()) return;
        pollTimer = setTimeout(() => {
            pollTimer = null;
            refresh({ force: true });
        }, delay === undefined ? POLL_MS : Math.max(0, delay));
    }

    function notify() {
        listeners.slice().forEach(fn => {
            try {
                fn(latestSummary);
            } catch (err) {
                console.warn('DashSummary listener failed', err);
            }
        });
    }

    // Fetches the summary (15 s cache unless force). Answers are applied in request order, so a slow older answer
    // never replaces a newer one. Resolves to { ok, status }; latest() keeps the last good summary.
    function refresh(options) {
        const force = !!(options && options.force);
        const seq = ++requestSeq;
        inFlight += 1;
        return window.DashAPI.getWithStatus(SUMMARY_URL, { ttl: CACHE_TTL_MS, force })
            .catch(() => ({ ok: false, status: 0 }))
            .then(result => {
                inFlight -= 1;
                const ok = !!(result && result.ok && result.data && typeof result.data === 'object');
                const status = ok ? 200 : (result && result.status) || 0;
                if (seq > appliedSeq) {
                    appliedSeq = seq;
                    lastFetchAt = Date.now();
                    lastStatus = status;
                    if (ok) latestSummary = result.data;
                    renderSidebar();
                    if (ok) notify();
                }
                schedulePoll();
                return { ok, status };
            });
    }

    // SSE triggers: the first trigger starts a 2 s timer and later triggers ride along, so a burst costs one
    // request and a steady stream of events cannot postpone the refresh forever. While hidden the trigger is
    // remembered and served by the catch-up refresh on becoming visible.
    function scheduleRefresh() {
        if (!pageVisible()) {
            missedWhileHidden = true;
            return;
        }
        if (debounceTimer) return;
        debounceTimer = setTimeout(() => {
            debounceTimer = null;
            refresh({ force: true });
        }, DEBOUNCE_MS);
    }

    function onVisibilityChange() {
        if (!pageVisible()) {
            clearPoll();
            if (debounceTimer) {
                clearTimeout(debounceTimer);
                debounceTimer = null;
                missedWhileHidden = true;
            }
            return;
        }
        if (inFlight > 0) return; // its answer schedules the next poll
        const age = Date.now() - lastFetchAt;
        if (missedWhileHidden || age >= POLL_MS) {
            missedWhileHidden = false;
            refresh({ force: true });
        } else {
            schedulePoll(POLL_MS - age);
        }
    }

    function onUpdate(fn) {
        if (typeof fn !== 'function') return () => {};
        listeners.push(fn);
        return () => {
            const at = listeners.indexOf(fn);
            if (at !== -1) listeners.splice(at, 1);
        };
    }

    function setLive(state) {
        liveState = state === 'on' || state === 'off' ? state : 'connecting';
        renderSidebar();
    }

    // ── Sidebar ─────────────────────────────────────────────────────────────────

    function setDot(dot, color) {
        if (dot) dot.className = 'status-dot ' + color;
    }

    function setText(el, text) {
        if (el) el.textContent = text;
    }

    // Status block (overall pill, agent busy/idle, SSE live) and the three badges, on every section.
    function renderSidebar() {
        if (typeof document === 'undefined' || typeof document.querySelector !== 'function') return;
        const block = document.querySelector('[data-sidebar-status]');
        if (block) {
            const overall = latestSummary
                ? overallText(latestSummary, t)
                : t(lastStatus ? 'dashboard.overview.pill_unavailable' : 'dashboard.overview.pill_loading');
            const agent = latestSummary && latestSummary.agent && typeof latestSummary.agent === 'object' ? latestSummary.agent : null;
            const agentText = agent ? t(agent.busy ? 'dashboard.agent_banner_status_busy' : 'dashboard.agent_banner_status_ready') : '—';
            const liveText = t('dashboard.overview.live_' + liveState);
            block.setAttribute('data-overall', overallState(latestSummary));
            setText(block.querySelector('[data-sidebar-overall]'), overall);
            setDot(block.querySelector('[data-sidebar-busy]'), !agent ? 'gray' : agent.busy ? 'yellow' : 'green');
            setText(block.querySelector('[data-sidebar-busy-label]'), agentText);
            setDot(block.querySelector('[data-sidebar-live]'), liveState === 'on' ? 'green' : liveState === 'off' ? 'red' : 'gray');
            setText(block.querySelector('[data-sidebar-live-label]'), liveText);
            block.setAttribute('aria-label', t('dashboard.overview.sidebar_status', { overall, agent: agentText, live: liveText }));
        }
        const texts = badgeTexts(latestSummary);
        const critical = attentionCounts(latestSummary).critical;
        BADGE_SECTIONS.forEach(id => {
            const badge = document.querySelector('[data-nav-badge="' + id + '"]');
            if (!badge) return;
            const text = texts[id];
            badge.textContent = text;
            badge.hidden = !text;
            // problems: danger; automations: warning; overview: danger with a critical row, else warning.
            badge.classList.toggle('is-warning', id === 'automations' || (id === 'overview' && critical === 0));
            if (text) badge.setAttribute('aria-label', t('dashboard.overview.badge_' + id, { n: text }));
            else badge.removeAttribute('aria-label');
        });
    }

    // On the overview the status link changes no hash, so DashRouter would leave an open drawer open.
    function onStatusClick() {
        if (window.DashRouter && window.DashRouter.current() === 'overview') window.DashRouter.closeDrawer();
    }

    function init() {
        if (initialized) return;
        initialized = true;
        if (window.AuraSSE && typeof window.AuraSSE.isConnected === 'function' && window.AuraSSE.isConnected()) liveState = 'on';
        const block = document.querySelector('[data-sidebar-status]');
        if (block) block.addEventListener('click', onStatusClick);
        document.addEventListener('visibilitychange', onVisibilityChange);
        renderSidebar();
        refresh();
    }

    window.DashSummary = {
        init,
        refresh,
        latest: () => latestSummary,
        onUpdate,
        scheduleRefresh,
        setLive,
        attentionCounts,
        overviewBadgeText,
        badgeTexts,
        overallState,
        overallText,
    };
})();
