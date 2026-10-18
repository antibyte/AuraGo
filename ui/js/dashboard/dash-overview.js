// AuraGo dashboard – overview tiles ("Lagebild", window.DashOverview). Classic script, loaded with defer after
// dash-summary.js and before main.js. render(summary) fills the eight ov-* tiles of the overview section from
// one /api/dashboard/summary answer; updateHealth(sys) updates ov-system in place from the system_metrics SSE
// event; init() wires "Customize" (hide/show the hideable tiles, stored per browser in localStorage
// 'aurago-dash-hidden-tiles') and re-renders on summary updates while the overview is shown. The helpers in the
// first block are pure and nothing touches the DOM at load time (ui/testdata/dash-overview.cjs).
(function () {
    'use strict';

    const STORAGE_KEY = 'aurago-dash-hidden-tiles';
    const HIDEABLE_TILES = Object.freeze(['ov-system', 'ov-budget', 'ov-automations', 'ov-knowledge', 'ov-activity', 'ov-agent-state']);
    // Summary sources (summary.unavailable) a tile depends on. Health is never listed: a null health only means
    // "not sampled yet", so ov-system waits for the first sample instead.
    const TILE_SOURCES = Object.freeze({
        'ov-attention': ['operational_issues'],
        'ov-automations': ['missions'],
        'ov-knowledge': ['knowledge', 'memory'],
        'ov-activity': ['audit'],
    });
    // Codes of internal/server/dashboard_attention.go; each one has a dashboard.attention.<code> string.
    const ATTENTION_CODES = Object.freeze([
        'operational_issue', 'cron_errors', 'missions_failed', 'daemon_failed', 'budget_blocked', 'budget_warning',
        'disk_high', 'memory_high', 'indexer_error', 'memory_conflicts', 'localwiki_update',
    ]);
    const SECTIONS = Object.freeze([
        'overview', 'personality', 'memory', 'user', 'knowledge', 'filesync', 'problems',
        'automations', 'system', 'audit', 'logs', 'llm', 'prompts', 'tools',
    ]);
    const SEVERITIES = Object.freeze(['critical', 'warning', 'info']);
    const TRENDS = Object.freeze(['steady', 'recovering', 'strained']);
    const MAX_ROWS = 8;
    const PERSONALITY_URL = '/api/personality/state';
    const PERSONALITY_TTL_MS = 15000; // same URL and ttl as the personality section: one shared cache entry

    // ── Pure helpers ─────────────────────────────────────────────────────────────

    // Stored hidden tiles: a JSON array of tile ids; anything else, and unknown ids, are ignored.
    function parseHiddenTiles(raw) {
        if (typeof raw !== 'string' || raw === '') return [];
        let list;
        try {
            list = JSON.parse(raw);
        } catch (_) {
            return [];
        }
        if (!Array.isArray(list)) return [];
        return HIDEABLE_TILES.filter(id => list.indexOf(id) !== -1);
    }

    function serializeHiddenTiles(ids) {
        const list = Array.isArray(ids) ? ids : [];
        return JSON.stringify(HIDEABLE_TILES.filter(id => list.indexOf(id) !== -1));
    }

    function tileUnavailable(tileId, summary) {
        const sources = Object.prototype.hasOwnProperty.call(TILE_SOURCES, tileId) ? TILE_SOURCES[tileId] : null;
        const unavailable = summary && Array.isArray(summary.unavailable) ? summary.unavailable : [];
        return !!sources && sources.some(source => unavailable.indexOf(source) !== -1);
    }

    function sectionHref(section) {
        return '#' + (SECTIONS.indexOf(section) !== -1 ? section : 'overview');
    }

    // Params for dashboard.attention.<code>. count comes first: t()/tOr() replace in insertion order, so a
    // title that contains "{{count}}" stays literal text.
    function attentionParams(item) {
        const params = { count: String(Math.max(0, Math.floor(Number(item && item.count) || 0))) };
        const raw = item && item.params && typeof item.params === 'object' ? item.params : {};
        Object.keys(raw).forEach(key => {
            params[key] = raw[key] == null ? '' : String(raw[key]);
        });
        return params;
    }

    // translateOr is tOr(key, fallback, params); an unknown code falls back to the code itself.
    function attentionText(item, translateOr) {
        const code = item && typeof item.code === 'string' ? item.code : '';
        return translateOr('dashboard.attention.' + code, code, attentionParams(item));
    }

    function percentValue(value) {
        const n = Number(value);
        return Number.isFinite(n) ? Math.min(100, Math.max(0, Math.round(n))) : 0;
    }

    function meterLevel(percent) {
        return percent >= 90 ? 'high' : percent >= 75 ? 'med' : 'ok';
    }

    function formatUSD(value) {
        const n = Number(value);
        return '$' + (Number.isFinite(n) ? n : 0).toFixed(2);
    }

    function budgetView(budget) {
        const b = budget && typeof budget === 'object' ? budget : {};
        if (!b.enabled) return { enabled: false };
        const state = b.state === 'blocked' || b.state === 'warning' ? b.state : 'ok';
        const raw = Math.round((Number(b.percentage) || 0) * 100);
        return {
            enabled: true,
            state,
            percent: percentValue(raw),
            percentText: raw + '%',
            level: state === 'blocked' ? 'high' : state === 'warning' ? 'med' : 'ok',
            spent: formatUSD(b.spent_usd),
            limit: formatUSD(b.daily_limit_usd),
        };
    }

    // Context fill as the old banner computed it: ~4 characters per token, 128k window by default.
    function contextPercent(agent, context) {
        const windowTokens = Number(agent && agent.context_window) || 128000;
        const chars = Number(context && context.total_chars) || 0;
        return Math.min(100, Math.max(0, Math.round((Math.round(chars / 4) / windowTokens) * 100)));
    }

    function lastActivityText(hours, translate) {
        const h = Number(hours);
        if (!Number.isFinite(h) || h < 0) return translate('dashboard.agent_banner_no_activity');
        if (h < 1) return translate('dashboard.agent_banner_just_active');
        if (h < 24) return translate('dashboard.agent_banner_hours_ago', { n: String(Math.round(h)) });
        return translate('dashboard.agent_banner_days_ago', { n: String(Math.round(h / 24)) });
    }

    // system_metrics SSE payload -> the summary's health shape.
    function healthFromMetrics(sys) {
        if (!sys || typeof sys !== 'object') return null;
        return {
            cpu_percent: Number(sys.cpu && sys.cpu.usage_percent) || 0,
            memory_percent: Number(sys.memory && sys.memory.used_percent) || 0,
            disk_percent: Number(sys.disk && sys.disk.used_percent) || 0,
            uptime_seconds: Number(sys.uptime_seconds) || 0,
        };
    }

    function uptimeText(seconds) {
        const s = Math.max(0, Math.floor(Number(seconds) || 0));
        const d = Math.floor(s / 86400);
        const h = Math.floor((s % 86400) / 3600);
        const m = Math.floor((s % 3600) / 60);
        if (d > 0) return d + 'd ' + h + 'h';
        if (h > 0) return h + 'h ' + m + 'm';
        return m + 'm';
    }

    // /api/personality/state -> ov-agent-state. affect_valence (-1..1) and affect_arousal (0..1) are only present
    // while affect is active; dynamics.trend is steady, recovering or strained.
    function agentStateView(state) {
        if (!state || typeof state !== 'object' || !state.enabled) return { enabled: false };
        const hasValence = typeof state.affect_valence === 'number' && Number.isFinite(state.affect_valence);
        const hasArousal = typeof state.affect_arousal === 'number' && Number.isFinite(state.affect_arousal);
        const valence = hasValence ? Math.max(-1, Math.min(1, state.affect_valence)) : 0;
        const arousal = hasArousal ? Math.max(0, Math.min(1, state.affect_arousal)) : 0;
        const dynamics = state.dynamics && typeof state.dynamics === 'object' ? state.dynamics : null;
        return {
            enabled: true,
            mood: typeof state.mood === 'string' ? state.mood : '',
            affect: hasValence && hasArousal,
            valencePercent: percentValue((valence + 1) * 50),
            valenceText: (valence > 0 ? '+' : '') + valence.toFixed(2),
            arousalPercent: percentValue(arousal * 100),
            arousalText: arousal.toFixed(2),
            trend: dynamics ? (TRENDS.indexOf(dynamics.trend) !== -1 ? dynamics.trend : 'steady') : '',
        };
    }

    // ── DOM (runs only after DOMContentLoaded; uses t, tOr, esc, escAttr, dashIcon from the page) ───────────

    let initialized = false;
    let customizing = false;
    let hiddenTiles = [];
    let lastRendered = null;
    let liveHealth = null;
    let agentStateSeq = 0;

    function byId(id) {
        return document.getElementById(id);
    }

    function isHidden(id) {
        return hiddenTiles.indexOf(id) !== -1;
    }

    function readHiddenTiles() {
        try {
            return parseHiddenTiles(window.localStorage.getItem(STORAGE_KEY));
        } catch (_) {
            return [];
        }
    }

    function storeHiddenTiles() {
        try {
            window.localStorage.setItem(STORAGE_KEY, serializeHiddenTiles(hiddenTiles));
        } catch (_) { /* storage unavailable: the choice lasts for this page view */ }
    }

    function cssToken(value) {
        return String(value == null ? '' : value).replace(/[^a-z0-9_-]/gi, '');
    }

    function formatTimestamp(value) {
        if (!value) return '';
        const date = new Date(value);
        if (Number.isNaN(date.getTime())) return String(value);
        return date.toLocaleString(document.documentElement.lang || undefined, { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' });
    }

    function note(text) {
        return '<p class="ov-tile-note">' + esc(text) + '</p>';
    }

    // Bars use the existing .w-pct-N width classes and .ctx-level-* colours: no inline style.
    function bar(percent, level) {
        return '<span class="ov-meter-bar"><span class="ov-meter-fill w-pct-' + percentValue(percent) + ' ctx-level-' + level + '"></span></span>';
    }

    function meter(label, percent, valueText, level) {
        return '<div class="ov-meter"><span class="ov-meter-label">' + esc(label) + '</span>' + bar(percent, level) + '<span class="ov-num">' + esc(valueText) + '</span></div>';
    }

    function kpiList(entries) {
        return '<dl class="ov-kpis">' + entries.map(entry => {
            const n = Math.max(0, Math.floor(Number(entry.value) || 0));
            return '<div class="ov-kpi' + (entry.alarm && n > 0 ? ' is-alarm' : '') + '"><dt class="ov-kpi-label">' + esc(entry.label) + '</dt><dd class="ov-kpi-value">' + esc(String(n)) + '</dd></div>';
        }).join('') + '</dl>';
    }

    function statusItem(label, valueHtml, title) {
        const titleAttr = title ? ' title="' + escAttr(title) + '"' : '';
        return '<li class="ov-status-item"' + titleAttr + '><span class="ov-status-label">' + esc(label) + '</span><span class="ov-status-value">' + valueHtml + '</span></li>';
    }

    function renderStatus(summary) {
        const pill = byId('ov-overall');
        if (pill) {
            pill.textContent = window.DashSummary.overallText(summary, t);
            pill.setAttribute('data-overall', window.DashSummary.overallState(summary));
        }
        const line = byId('ov-status-line');
        if (!line) return;
        const agent = summary.agent && typeof summary.agent === 'object' ? summary.agent : {};
        const stateKey = agent.busy ? 'dashboard.agent_banner_status_busy' : agent.debug ? 'dashboard.agent_banner_status_debug' : 'dashboard.agent_banner_status_ready';
        const dot = agent.busy ? 'yellow' : agent.debug ? 'red' : 'green';
        const ctx = contextPercent(agent, summary.context);
        const items = [
            statusItem(t('dashboard.overview.label_agent'), '<span class="status-dot ' + dot + '" aria-hidden="true"></span>' + esc(t(stateKey))),
            statusItem(t('dashboard.overview.label_model'), esc(String(agent.model || '—'))),
            statusItem(t('dashboard.overview.label_persona'), esc(String(agent.personality || t('dashboard.agent_banner_personality_default')))),
            statusItem(t('dashboard.overview.label_mood'), esc(agent.mood ? tOr('dashboard.personality_mood_' + agent.mood, String(agent.mood)) : '—')),
            statusItem(t('dashboard.overview.label_context'), bar(ctx, ctx > 80 ? 'high' : ctx > 60 ? 'med' : 'ok') + '<span class="ov-num">' + ctx + '%</span>'),
            statusItem(t('dashboard.overview.label_last_activity'), esc(lastActivityText(summary.last_activity_hours, t))),
        ];
        const maintenance = summary.maintenance && typeof summary.maintenance === 'object' ? summary.maintenance : {};
        if (maintenance.enabled) {
            const status = String(maintenance.last_status || 'never');
            const lastRun = maintenance.last_run ? formatTimestamp(maintenance.last_run) : t('dashboard.maintenance_status_never');
            const nextRun = maintenance.next_run ? formatTimestamp(maintenance.next_run) : '—';
            const title = t('dashboard.maintenance_last_run', { time: lastRun }) + ' · ' + t('dashboard.maintenance_next_run', { time: nextRun });
            items.push(statusItem(t('dashboard.maintenance_nightly_title'), esc(tOr('dashboard.maintenance_status_' + status, status)) + ' · ' + esc(lastRun), title));
        }
        line.innerHTML = items.join('');
    }

    function attentionRow(severity, href, text, when) {
        const icon = severity === 'info' ? dashIcon('circle-dot') : dashIcon('warning');
        return '<li><a class="ov-attention-row" data-severity="' + severity + '" href="' + href + '">' + icon
            + '<span class="visually-hidden">' + esc(t('dashboard.overview.severity_' + severity)) + ': </span>'
            + '<span class="ov-attention-text">' + esc(text) + '</span>'
            + (when ? '<span class="ov-attention-when">' + esc(when) + '</span>' : '') + '</a></li>';
    }

    function renderAttention(summary) {
        const list = byId('ov-attention-list');
        if (!list) return;
        const rows = [];
        if (tileUnavailable('ov-attention', summary)) {
            rows.push(attentionRow('warning', '#problems', t('dashboard.overview.attention_problems_unchecked'), ''));
        }
        const items = Array.isArray(summary.attention) ? summary.attention.slice(0, MAX_ROWS) : [];
        items.forEach(item => {
            if (!item || typeof item !== 'object') return;
            const severity = SEVERITIES.indexOf(item.severity) !== -1 ? item.severity : 'info';
            rows.push(attentionRow(severity, sectionHref(item.section), attentionText(item, tOr), formatTimestamp(item.at)));
        });
        if (!rows.length) {
            rows.push('<li class="ov-attention-row ov-attention-ok">' + dashIcon('check') + '<span class="ov-attention-text">' + esc(t('dashboard.overview.attention_empty')) + '</span></li>');
        }
        const more = Math.max(0, Math.floor(Number(summary.attention_more) || 0));
        if (more > 0) {
            rows.push('<li><a class="ov-attention-more" href="#problems">' + esc(t('dashboard.overview.attention_more', { n: String(more) })) + '</a></li>');
        }
        list.innerHTML = rows.join('');
    }

    function renderSystem(health) {
        const body = byId('ov-system-body');
        if (!body) return;
        if (!health) {
            body.innerHTML = note(t('dashboard.overview.waiting_for_sample'));
            return;
        }
        const meters = [
            ['dashboard.system_cpu', health.cpu_percent],
            ['dashboard.system_ram', health.memory_percent],
            ['dashboard.system_disk', health.disk_percent],
        ].map(([key, value]) => {
            const percent = percentValue(value);
            return meter(t(key), percent, percent + '%', meterLevel(percent));
        });
        body.innerHTML = '<div class="ov-meters">' + meters.join('') + '</div>'
            + '<p class="ov-tile-foot">' + esc(t('dashboard.system_uptime')) + ' ' + esc(uptimeText(health.uptime_seconds)) + '</p>';
    }

    function renderBudget(summary) {
        const body = byId('ov-budget-body');
        if (!body) return;
        const view = budgetView(summary.kpis && summary.kpis.budget);
        if (!view.enabled) {
            body.innerHTML = note(t('dashboard.budget_disabled'));
            return;
        }
        const badgeClass = view.state === 'blocked' ? 'danger' : view.state === 'warning' ? 'warning' : 'info';
        const badgeText = view.state === 'blocked' ? t('dashboard.budget_blocked') : view.state === 'warning' ? t('dashboard.budget_warning') : t('dashboard.overview.budget_ok');
        body.innerHTML = '<div class="ov-kpi-value">' + esc(view.spent) + '</div>'
            + '<p class="ov-tile-foot">' + esc(t('dashboard.overview.budget_spent', { spent: view.spent, limit: view.limit })) + '</p>'
            + '<div class="ov-meter ov-meter-plain">' + bar(view.percent, view.level) + '<span class="ov-num">' + esc(view.percentText) + '</span></div>'
            + '<span class="budget-badge ' + badgeClass + '">' + esc(badgeText) + '</span>';
    }

    function renderAutomations(summary) {
        const body = byId('ov-automations-body');
        if (!body) return;
        if (tileUnavailable('ov-automations', summary)) {
            body.innerHTML = note(t('dashboard.overview.not_available'));
            return;
        }
        const a = summary.kpis && summary.kpis.automations ? summary.kpis.automations : {};
        body.innerHTML = kpiList([
            { label: t('dashboard.overview.kpi_running'), value: a.missions_running },
            { label: t('dashboard.overview.kpi_queued'), value: a.missions_queued },
            { label: t('dashboard.overview.kpi_cron_errors'), value: a.cron_errors, alarm: true },
            { label: t('dashboard.overview.kpi_failed_missions'), value: a.missions_failed_24h, alarm: true },
        ]);
    }

    function renderKnowledge(summary) {
        const body = byId('ov-knowledge-body');
        if (!body) return;
        if (tileUnavailable('ov-knowledge', summary)) {
            body.innerHTML = note(t('dashboard.overview.not_available'));
            return;
        }
        const k = summary.kpis && summary.kpis.knowledge ? summary.kpis.knowledge : {};
        body.innerHTML = kpiList([
            { label: t('dashboard.overview.kpi_kg_nodes'), value: k.kg_nodes },
            { label: t('dashboard.memory_core_facts'), value: k.core_facts },
        ]);
    }

    function renderActivity(summary) {
        const list = byId('ov-activity-list');
        if (!list) return;
        if (tileUnavailable('ov-activity', summary)) {
            list.innerHTML = '<li>' + note(t('dashboard.overview.not_available')) + '</li>';
            return;
        }
        const events = (Array.isArray(summary.recent_activity) ? summary.recent_activity : [])
            .filter(event => event && typeof event === 'object')
            .slice(0, MAX_ROWS);
        if (!events.length) {
            list.innerHTML = '<li>' + note(t('dashboard.overview.activity_empty')) + '</li>';
            return;
        }
        list.innerHTML = events.map(event => {
            const type = String(event.event_type || '');
            const source = String(event.source || '');
            const status = cssToken(event.status);
            const typeLabel = type ? tOr('dashboard.audit_type_' + type.replace(/-/g, '_'), type) : '—';
            const sourceLabel = source ? tOr('dashboard.audit_source_' + source, source) : '';
            const statusLabel = status ? tOr('dashboard.audit_status_' + status, status) : '';
            return '<li><a class="ov-activity-row" href="' + sectionHref(event.section === 'automations' ? 'automations' : 'audit') + '">'
                + '<span class="ov-activity-time">' + esc(formatTimestamp(event.timestamp)) + '</span>'
                + '<span class="ov-activity-what"><strong>' + esc(typeLabel) + '</strong> ' + esc(String(event.target_name || '')) + '</span>'
                + '<span class="ov-activity-source">' + esc(sourceLabel) + '</span>'
                + (statusLabel ? '<span class="audit-status audit-status-' + status + '">' + esc(statusLabel) + '</span>' : '')
                + '</a></li>';
        }).join('');
    }

    function renderAgentState(state) {
        const body = byId('ov-agent-state-body');
        if (!body) return;
        if (!state) {
            body.innerHTML = note(t('dashboard.overview.not_available'));
            return;
        }
        const view = agentStateView(state);
        if (!view.enabled) {
            body.innerHTML = note(t('dashboard.personality_disabled'));
            return;
        }
        const mood = view.mood ? tOr('dashboard.personality_mood_' + view.mood, view.mood) : '—';
        const affect = view.affect
            ? '<div class="ov-meters">' + meter(t('dashboard.personality_affect_valence'), view.valencePercent, view.valenceText, 'ok')
                + meter(t('dashboard.personality_affect_arousal'), view.arousalPercent, view.arousalText, 'ok') + '</div>'
            : note(t('dashboard.overview.affect_at_rest'));
        const trend = view.trend
            ? '<p class="ov-tile-foot">' + esc(t('dashboard.overview.trend_label')) + ' <span class="budget-badge ' + (view.trend === 'strained' ? 'warning' : 'info') + '">' + esc(t('dashboard.overview.trend_' + view.trend)) + '</span></p>'
            : '';
        body.innerHTML = '<div class="ov-kpi-value">' + esc(mood) + '</div>' + affect + trend;
    }

    // Only fetched while the tile is shown (hidden tiles cost no request).
    function loadAgentState() {
        if (isHidden('ov-agent-state') || !byId('ov-agent-state-body') || !window.DashAPI) return Promise.resolve();
        const seq = ++agentStateSeq;
        return window.DashAPI.getWithStatus(PERSONALITY_URL, { ttl: PERSONALITY_TTL_MS }).then(result => {
            if (seq === agentStateSeq) renderAgentState(result && result.ok ? result.data : null);
        });
    }

    // Customize: outside the mode hidden tiles get .is-hidden (display: none, no grid gap); inside it they
    // stay visible but dimmed (.ov-tile-off) so each one can be switched back on.
    function applyVisibility() {
        const section = byId('sec-overview');
        if (section) section.classList.toggle('is-customizing', customizing);
        HIDEABLE_TILES.forEach(id => {
            const tile = byId(id);
            if (!tile) return;
            const off = isHidden(id);
            tile.classList.toggle('is-hidden', off && !customizing);
            tile.classList.toggle('ov-tile-off', off && customizing);
            const toggle = tile.querySelector('[data-tile-toggle]');
            if (toggle) {
                toggle.hidden = !customizing;
                toggle.setAttribute('aria-pressed', off ? 'false' : 'true');
            }
        });
        const button = document.querySelector('[data-action="overview-customize"]');
        if (button) button.setAttribute('aria-pressed', customizing ? 'true' : 'false');
        const showAll = document.querySelector('[data-action="overview-show-all"]');
        if (showAll) showAll.hidden = !customizing;
    }

    function setCustomizing(on) {
        const active = document.activeElement;
        customizing = !!on;
        applyVisibility();
        // A switch that just got hidden must not keep the focus.
        if (!customizing && active && typeof active.closest === 'function' && active.closest('[data-tile-toggle]')) {
            const button = document.querySelector('[data-action="overview-customize"]');
            if (button) button.focus();
        }
    }

    function toggleTile(id) {
        if (HIDEABLE_TILES.indexOf(id) === -1) return;
        hiddenTiles = isHidden(id) ? hiddenTiles.filter(other => other !== id) : hiddenTiles.concat(id);
        storeHiddenTiles();
        applyVisibility();
        if (id === 'ov-agent-state' && !isHidden(id)) loadAgentState();
    }

    function showAllTiles() {
        hiddenTiles = [];
        storeHiddenTiles();
        applyVisibility();
        loadAgentState();
    }

    function onClick(event) {
        const target = event.target instanceof Element ? event.target : null;
        if (!target) return;
        const toggle = target.closest('[data-tile-toggle]');
        if (toggle) {
            event.preventDefault();
            toggleTile(toggle.getAttribute('data-tile-toggle'));
            return;
        }
        const action = target.closest('[data-action="overview-customize"], [data-action="overview-show-all"]');
        if (!action) return;
        event.preventDefault();
        if (action.getAttribute('data-action') === 'overview-customize') setCustomizing(!customizing);
        else showAllTiles();
    }

    // Renders every tile from one summary. A summary object that was already rendered is skipped (the loader
    // and the update listener may both hand over the same object).
    function render(summary) {
        if (!summary || typeof summary !== 'object') return;
        if (window.CardState) {
            window.CardState.setLoaded('ov-status');
            window.CardState.setLoaded('ov-attention');
        }
        if (summary === lastRendered) return;
        lastRendered = summary;
        renderStatus(summary);
        renderAttention(summary);
        renderSystem(summary.health || liveHealth);
        renderBudget(summary);
        renderAutomations(summary);
        renderKnowledge(summary);
        renderActivity(summary);
        loadAgentState();
    }

    // system_metrics (every 10 s): update ov-system in place, no summary refetch.
    function updateHealth(sys) {
        const health = healthFromMetrics(sys);
        if (!health) return;
        liveHealth = health;
        renderSystem(health);
    }

    function init() {
        if (initialized) return;
        initialized = true;
        hiddenTiles = readHiddenTiles();
        document.querySelectorAll('[data-tile-toggle]').forEach(toggle => {
            const tile = toggle.closest('[data-tile]');
            const title = tile ? tile.querySelector('.dash-card-title') : null;
            const name = title ? t(title.getAttribute('data-i18n')) : toggle.getAttribute('data-tile-toggle');
            const label = t('dashboard.overview.tile_toggle', { tile: name });
            toggle.setAttribute('aria-label', label);
            toggle.title = label;
        });
        applyVisibility();
        document.addEventListener('click', onClick);
        if (window.DashSummary) {
            window.DashSummary.onUpdate(summary => {
                if (!window.DashRouter || window.DashRouter.current() !== 'overview') return;
                render(summary);
                const updated = document.querySelector('[data-section-updated="overview"]');
                if (updated) updated.textContent = t('dashboard.section.updated', { time: new Date().toLocaleTimeString() });
            });
        }
    }

    window.DashOverview = {
        init,
        render,
        updateHealth,
        STORAGE_KEY,
        HIDEABLE_TILES,
        ATTENTION_CODES,
        parseHiddenTiles,
        serializeHiddenTiles,
        tileUnavailable,
        sectionHref,
        attentionParams,
        attentionText,
        percentValue,
        budgetView,
        contextPercent,
        lastActivityText,
        healthFromMetrics,
        uptimeText,
        agentStateView,
    };
})();
