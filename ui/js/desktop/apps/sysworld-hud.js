(function () {
    'use strict';

    // system-world HUD: HTML overlay (stats, action buttons, legend, event feed,
    // tooltip, info slide-in panel). Augments the shared window.SysWorld namespace
    // with the createHud factory. Loaded after the scene modules, before sysworld.js.

    const NS = (window.SysWorld = window.SysWorld || {});

    const QUALITY_ORDER = ['low', 'medium', 'high'];
    const QUALITY_SCALES = { low: 0.4, medium: 0.7, high: 1.0 };
    const QUALITY_STORAGE_KEY = 'aurago.desktop.sysworld.quality';

    const STAT_KEYS = ['cpu', 'ram', 'uptime', 'budget', 'missions', 'memories', 'agent'];
    const CATEGORY_KEYS = ['communication', 'smarthome', 'infrastructure', 'ai', 'storage', 'monitoring', 'other'];
    // Zone legend entries: [i18n zone suffix, representative PALETTE color name].
    const ZONE_DEFS = [
        ['core', 'core'],
        ['integrations', 'communication'],
        ['graph', 'other'],
        ['memory', 'memory'],
        ['missions', 'mission'],
        ['agents', 'agent'],
        ['tools', 'tool'],
        ['infra', 'infrastructure']
    ];

    const MAX_EVENTS = 6;
    const EVENT_TTL_MS = 7000;
    const EVENT_FADE_MS = 400;
    const TOOLTIP_OFFSET_PX = 14;
    const EMPTY_VALUE = '–';

    NS.createHud = function (inst) {
        const handlers = inst.hudHandlers || {};
        const esc = (inst.ctx && typeof inst.ctx.esc === 'function')
            ? inst.ctx.esc
            : value => String(value == null ? '' : value);
        const L = key => {
            try {
                const v = inst.L(key);
                return typeof v === 'string' ? v : key;
            } catch (_) {
                return key;
            }
        };
        // PALETTE hex numbers (0x4fc3f7) → CSS color strings.
        const paletteCss = name => {
            const hex = NS.PALETTE && typeof NS.PALETTE[name] === 'number' ? NS.PALETTE[name] : 0xffffff;
            return '#' + hex.toString(16).padStart(6, '0');
        };
        const notify = (name, arg) => {
            if (typeof handlers[name] === 'function') {
                try { handlers[name](arg); } catch (_) {}
            }
        };

        let disposed = false;
        const stats = {};
        const statValueEls = {};
        const eventTimers = [];

        // ── DOM ──────────────────────────────────────────────────────────────

        const root = document.createElement('div');
        root.className = 'sysworld-hud';

        const panelTop = document.createElement('div');
        panelTop.className = 'sw-panel-top';
        const statsEl = document.createElement('div');
        statsEl.className = 'sw-stats sw-interactive';
        panelTop.appendChild(statsEl);
        root.appendChild(panelTop);

        STAT_KEYS.forEach(key => {
            const row = document.createElement('div');
            row.className = 'sw-stat';
            const label = document.createElement('span');
            label.className = 'sw-stat-label';
            label.textContent = L('sysworld.stats.' + key);
            const value = document.createElement('span');
            value.className = 'sw-stat-value';
            value.setAttribute('data-sw-stat', key);
            value.textContent = EMPTY_VALUE;
            row.appendChild(label);
            row.appendChild(value);
            statsEl.appendChild(row);
            statValueEls[key] = value;
        });

        const actionsEl = document.createElement('div');
        actionsEl.className = 'sw-actions sw-interactive';
        root.appendChild(actionsEl);

        function makeButton(action, labelKey) {
            const btn = document.createElement('button');
            btn.type = 'button';
            btn.className = 'sw-btn';
            btn.setAttribute('data-sw-action', action);
            const label = L(labelKey);
            btn.title = label;
            btn.setAttribute('aria-label', label);
            btn.textContent = label;
            actionsEl.appendChild(btn);
            return btn;
        }

        const btnOverview = makeButton('overview', 'sysworld.btn.overview');
        const btnGraph = makeButton('graph', 'sysworld.btn.graph');
        const btnEffects = makeButton('effects', 'sysworld.btn.effects');
        const btnQuality = makeButton('quality', 'sysworld.btn.quality');

        const legendEl = document.createElement('div');
        legendEl.className = 'sw-legend sw-interactive';
        root.appendChild(legendEl);

        const eventsEl = document.createElement('div');
        eventsEl.className = 'sw-events';
        const eventsTitle = document.createElement('div');
        eventsTitle.className = 'sw-events-title';
        eventsTitle.textContent = L('sysworld.events');
        eventsEl.appendChild(eventsTitle);
        root.appendChild(eventsEl);

        const tooltipEl = document.createElement('div');
        tooltipEl.className = 'sw-tooltip';
        root.appendChild(tooltipEl);

        // Persistent selection label: floats above the focused object and is
        // positioned every frame by the entry module (positionSelLabel).
        const selLabel = document.createElement('div');
        selLabel.className = 'sw-sel-label';
        const selDot = document.createElement('span');
        selDot.className = 'sw-sel-dot';
        const selName = document.createElement('span');
        selName.className = 'sw-sel-name';
        const selKind = document.createElement('span');
        selKind.className = 'sw-sel-kind';
        selLabel.appendChild(selDot);
        selLabel.appendChild(selName);
        selLabel.appendChild(selKind);
        root.appendChild(selLabel);

        const infoEl = document.createElement('div');
        infoEl.className = 'sw-info sw-interactive';
        const infoGlow = document.createElement('div');
        infoGlow.className = 'sw-info-glow';
        const infoHead = document.createElement('div');
        infoHead.className = 'sw-info-head';
        const infoBadge = document.createElement('div');
        infoBadge.className = 'sw-info-badge';
        const infoBadgeDot = document.createElement('span');
        infoBadgeDot.className = 'sw-info-badge-dot';
        const infoBadgeText = document.createElement('span');
        infoBadgeText.className = 'sw-info-badge-text';
        infoBadge.appendChild(infoBadgeDot);
        infoBadge.appendChild(infoBadgeText);
        const infoTitle = document.createElement('div');
        infoTitle.className = 'sw-info-title';
        infoHead.appendChild(infoBadge);
        infoHead.appendChild(infoTitle);
        const infoClose = document.createElement('button');
        infoClose.type = 'button';
        infoClose.className = 'sw-info-close';
        const closeLabel = L('sysworld.panel.close');
        infoClose.title = closeLabel;
        infoClose.setAttribute('aria-label', closeLabel);
        infoClose.textContent = '×';
        const infoRows = document.createElement('div');
        infoRows.className = 'sw-info-rows';
        const infoFoot = document.createElement('div');
        infoFoot.className = 'sw-info-foot';
        const infoFootHint = document.createElement('div');
        infoFootHint.textContent = L('sysworld.panel.hint');
        const infoFootKeys = document.createElement('div');
        infoFootKeys.className = 'sw-info-foot-keys';
        infoFootKeys.textContent = L('sysworld.panel.hint_keys');
        infoFoot.appendChild(infoFootHint);
        infoFoot.appendChild(infoFootKeys);
        infoEl.appendChild(infoGlow);
        infoEl.appendChild(infoClose);
        infoEl.appendChild(infoHead);
        infoEl.appendChild(infoRows);
        infoEl.appendChild(infoFoot);

        inst.root.appendChild(root);
        inst.root.appendChild(infoEl);

        // ── Stats ────────────────────────────────────────────────────────────

        function fmtPct(v) {
            return typeof v === 'number' && isFinite(v) ? Math.round(v) + '%' : EMPTY_VALUE;
        }

        function fmtUptime(seconds) {
            if (typeof seconds !== 'number' || !isFinite(seconds) || seconds < 0) return EMPTY_VALUE;
            const totalMin = Math.floor(seconds / 60);
            const hours = Math.floor(totalMin / 60);
            const days = Math.floor(hours / 24);
            if (days > 0) return days + 'd ' + (hours % 24) + 'h';
            if (hours > 0) return hours + 'h ' + (totalMin % 60) + 'm';
            return totalMin + 'm';
        }

        function fmtMoney(v) {
            return '$' + (typeof v === 'number' && isFinite(v) ? v.toFixed(2) : '0.00');
        }

        // Budget accepts {spent, limit}, a bare number, or a preformatted string.
        function fmtBudget(b) {
            if (b == null) return EMPTY_VALUE;
            if (typeof b === 'string') return b || EMPTY_VALUE;
            if (typeof b === 'number') return fmtMoney(b);
            if (typeof b === 'object') {
                const spent = typeof b.spent === 'number' ? b.spent : null;
                const limit = typeof b.limit === 'number' ? b.limit : null;
                if (spent != null && limit != null) return fmtMoney(spent) + ' / ' + fmtMoney(limit);
                if (spent != null) return fmtMoney(spent);
                if (limit != null) return fmtMoney(limit);
            }
            return EMPTY_VALUE;
        }

        function fmtCount(v) {
            if (typeof v === 'number' && isFinite(v)) return String(v);
            if (typeof v === 'string' && v) return v;
            return EMPTY_VALUE;
        }

        // Missions accepts a count or an object with active/running + total fields.
        function fmtMissions(v) {
            if (typeof v === 'number' && isFinite(v)) return String(v);
            if (v && typeof v === 'object') {
                const active = firstNum(v.active, v.running, v.in_progress);
                const total = firstNum(v.total, v.count, v.size);
                if (active != null && total != null) return active + ' / ' + total;
                if (active != null) return String(active);
                if (total != null) return String(total);
            }
            return EMPTY_VALUE;
        }

        function firstNum() {
            for (let i = 0; i < arguments.length; i++) {
                const v = arguments[i];
                if (typeof v === 'number' && isFinite(v)) return v;
            }
            return null;
        }

        function renderStats() {
            statValueEls.cpu.textContent = fmtPct(stats.cpu);
            statValueEls.ram.textContent = fmtPct(stats.ram);
            statValueEls.uptime.textContent = fmtUptime(stats.uptime);
            statValueEls.budget.textContent = fmtBudget(stats.budget);
            statValueEls.missions.textContent = fmtMissions(stats.missions);
            statValueEls.memories.textContent = fmtCount(stats.memories);
            statValueEls.agent.textContent = stats.agentBusy === true
                ? L('sysworld.agent.busy')
                : (stats.agentBusy === false ? L('sysworld.agent.idle') : EMPTY_VALUE);
            // Pulsing dot next to the agent state while the agent is working.
            statValueEls.agent.classList.toggle('busy', stats.agentBusy === true);
        }

        // Partial update: only keys present in the patch are touched.
        function setStats(patch) {
            if (!patch || typeof patch !== 'object') return;
            Object.keys(patch).forEach(key => {
                stats[key] = patch[key];
            });
            renderStats();
        }

        // ── Event feed ───────────────────────────────────────────────────────

        function eventItems() {
            return Array.prototype.filter.call(
                eventsEl.querySelectorAll('.sw-event'),
                el => !el.classList.contains('sw-event-empty')
            );
        }

        function trackTimer(id) {
            eventTimers.push(id);
            return id;
        }

        function removeItem(item) {
            if (item._swTimers) item._swTimers.forEach(id => clearTimeout(id));
            try { item.remove(); } catch (_) {}
        }

        function showEmpty() {
            if (eventsEl.querySelector('.sw-event-empty')) return;
            const empty = document.createElement('div');
            empty.className = 'sw-event info sw-event-empty';
            empty.textContent = L('sysworld.events.empty');
            eventsEl.appendChild(empty);
        }

        function event(text, kind) {
            if (disposed) return;
            const cls = kind === 'ok' || kind === 'err' ? kind : 'info';
            const empty = eventsEl.querySelector('.sw-event-empty');
            if (empty) removeItem(empty);
            const item = document.createElement('div');
            item.className = 'sw-event ' + cls;
            item.textContent = String(text == null ? '' : text);
            // Newest first, directly below the title.
            eventsEl.insertBefore(item, eventsTitle.nextSibling);
            while (eventItems().length > MAX_EVENTS) {
                const items = eventItems();
                removeItem(items[items.length - 1]);
            }
            // Auto-fade after ~7s, then remove; empty state returns when feed drains.
            const fadeTimer = trackTimer(setTimeout(() => {
                item.style.transition = 'opacity ' + EVENT_FADE_MS + 'ms ease';
                item.style.opacity = '0';
            }, EVENT_TTL_MS));
            const removeTimer = trackTimer(setTimeout(() => {
                removeItem(item);
                if (!eventItems().length) showEmpty();
            }, EVENT_TTL_MS + EVENT_FADE_MS));
            item._swTimers = [fadeTimer, removeTimer];
        }

        // ── Legend ───────────────────────────────────────────────────────────

        function legendItem(colorCss, label, extraClass) {
            const item = document.createElement('div');
            item.className = 'sw-legend-item' + (extraClass ? ' ' + extraClass : '');
            const dot = document.createElement('span');
            dot.className = 'sw-dot';
            dot.style.background = colorCss;
            const lab = document.createElement('span');
            lab.textContent = label;
            item.appendChild(dot);
            item.appendChild(lab);
            return item;
        }

        function setLegend() {
            legendEl.innerHTML = '';
            const title = document.createElement('div');
            title.className = 'sw-legend-title';
            title.textContent = L('sysworld.legend');
            legendEl.appendChild(title);
            CATEGORY_KEYS.forEach(cat => {
                legendEl.appendChild(legendItem(paletteCss(cat), L('sysworld.cat.' + cat)));
            });
            ZONE_DEFS.forEach(def => {
                const item = legendItem(paletteCss(def[1]), L('sysworld.zone.' + def[0]), 'sw-legend-zone');
                item.setAttribute('data-sw-zone', def[0]);
                legendEl.appendChild(item);
            });
        }

        // Legend zone interactivity: hover notifies for a zone pulse, click
        // requests a camera flight to the zone (behaviour lives in the entry).
        let hoverZone = null;

        function zoneOf(ev) {
            const target = ev.target;
            const item = target && typeof target.closest === 'function'
                ? target.closest('[data-sw-zone]')
                : null;
            return item && legendEl.contains(item) ? item.getAttribute('data-sw-zone') : null;
        }

        const onLegendOver = ev => {
            const zone = zoneOf(ev);
            if (zone === hoverZone) return;
            hoverZone = zone;
            notify('onZoneHover', zone);
        };
        const onLegendOut = ev => {
            const item = ev.target && typeof ev.target.closest === 'function'
                ? ev.target.closest('[data-sw-zone]')
                : null;
            if (item && ev.relatedTarget && item.contains(ev.relatedTarget)) return;
            if (hoverZone !== null) {
                hoverZone = null;
                notify('onZoneHover', null);
            }
        };
        const onLegendClick = ev => {
            const zone = zoneOf(ev);
            if (zone) notify('onZoneFocus', zone);
        };
        legendEl.addEventListener('mouseover', onLegendOver);
        legendEl.addEventListener('mouseout', onLegendOut);
        legendEl.addEventListener('click', onLegendClick);

        // ── Tooltip ──────────────────────────────────────────────────────────

        // html is pre-escaped markup provided by the entry module.
        function showTooltip(html, clientX, clientY) {
            tooltipEl.innerHTML = String(html || '');
            tooltipEl.classList.add('visible');
            const rect = inst.root.getBoundingClientRect();
            let x = clientX - rect.left + TOOLTIP_OFFSET_PX;
            let y = clientY - rect.top + TOOLTIP_OFFSET_PX;
            const w = tooltipEl.offsetWidth;
            const h = tooltipEl.offsetHeight;
            // Flip to the other cursor side when the tooltip would overflow.
            if (x + w > rect.width) x = clientX - rect.left - w - TOOLTIP_OFFSET_PX;
            if (y + h > rect.height) y = clientY - rect.top - h - TOOLTIP_OFFSET_PX;
            tooltipEl.style.left = Math.max(0, x) + 'px';
            tooltipEl.style.top = Math.max(0, y) + 'px';
        }

        function hideTooltip() {
            tooltipEl.classList.remove('visible');
        }

        // ── Selection label ─────────────────────────────────────────────────

        // Translated kind caption with graceful fallback to 'object'.
        function kindText(kind) {
            let label = L('sysworld.kind.' + (kind || 'object'));
            if (label.indexOf('sysworld.') === 0) label = L('sysworld.kind.object');
            return label;
        }

        // info: {name, kind, accent} — content updates only; positioning is
        // done per frame via positionSelLabel from the entry's RAF loop.
        function showSelLabel(info) {
            const m = info || {};
            const accent = typeof m.accent === 'string' && m.accent ? m.accent : '#59d4ff';
            selLabel.style.setProperty('--sw-accent', accent);
            selDot.style.background = accent;
            selDot.style.boxShadow = '0 0 8px ' + accent;
            selName.textContent = String(m.name == null ? '' : m.name);
            selKind.textContent = kindText(m.kind);
            selLabel.classList.add('visible');
        }

        function positionSelLabel(x, y) {
            if (!selLabel.classList.contains('visible')) return;
            selLabel.style.left = x + 'px';
            selLabel.style.top = y + 'px';
        }

        function hideSelLabel() {
            selLabel.classList.remove('visible');
        }

        // ── Info panel ───────────────────────────────────────────────────────

        // rows: [{k: translated label, v: pre-escaped value html, tone?}]
        // meta: {kind: 'sysworld.kind.*' suffix, accent: css color} drives the
        // badge, the accent glow and the top strip so panel and 3D selection
        // share one color story.
        function showPanel(title, rows, meta) {
            const m = meta || {};
            const kind = typeof m.kind === 'string' && m.kind ? m.kind : 'object';
            const accent = typeof m.accent === 'string' && m.accent ? m.accent : '#59d4ff';
            infoEl.style.setProperty('--sw-accent', accent);
            infoBadgeText.textContent = kindText(kind);
            infoBadgeDot.style.background = accent;
            infoBadgeDot.style.boxShadow = '0 0 8px ' + accent;
            infoTitle.textContent = String(title == null ? '' : title);
            infoRows.innerHTML = '';
            let rowIndex = 0;
            (Array.isArray(rows) ? rows : []).forEach(r => {
                if (!r) return;
                // Section header row: {section: 'Label'}
                if (r.section) {
                    const sec = document.createElement('div');
                    sec.className = 'sw-section';
                    sec.textContent = String(r.section);
                    sec.style.animationDelay = Math.min(rowIndex, 10) * 38 + 'ms';
                    rowIndex++;
                    infoRows.appendChild(sec);
                    return;
                }
                // Relation list row group: {rel: [{label, relation}]}
                if (Array.isArray(r.rel)) {
                    r.rel.slice(0, 5).forEach(item => {
                        if (!item) return;
                        const row = document.createElement('div');
                        row.className = 'sw-row sw-rel';
                        row.style.animationDelay = Math.min(rowIndex, 10) * 38 + 'ms';
                        rowIndex++;
                        const k = document.createElement('span');
                        k.className = 'sw-key';
                        k.textContent = String(item.relation == null ? '' : item.relation);
                        const v = document.createElement('span');
                        v.className = 'sw-val';
                        v.textContent = String(item.label == null ? '' : item.label);
                        row.appendChild(k);
                        row.appendChild(v);
                        infoRows.appendChild(row);
                    });
                    return;
                }
                const row = document.createElement('div');
                row.className = 'sw-row';
                // Staggered cascade-in; delay capped so long panels stay snappy.
                row.style.animationDelay = Math.min(rowIndex, 10) * 38 + 'ms';
                rowIndex++;
                const k = document.createElement('span');
                k.className = 'sw-key';
                k.textContent = String(r.k == null ? '' : r.k);
                const v = document.createElement('span');
                v.className = 'sw-val' + (r.tone ? ' sw-pill sw-tone-' + r.tone : '');
                v.innerHTML = String(r.v == null ? '' : r.v);
                // Optional mini bar under the value: {bar: 0..1}
                if (typeof r.bar === 'number' && isFinite(r.bar)) {
                    const wrap = document.createElement('span');
                    wrap.className = 'sw-valwrap';
                    wrap.appendChild(v);
                    const bar = document.createElement('span');
                    bar.className = 'sw-bar';
                    const fill = document.createElement('span');
                    fill.className = 'sw-bar-fill';
                    const pct = Math.max(0, Math.min(1, r.bar));
                    fill.style.width = Math.round(pct * 100) + '%';
                    bar.appendChild(fill);
                    wrap.appendChild(bar);
                    row.appendChild(k);
                    row.appendChild(wrap);
                } else {
                    row.appendChild(k);
                    row.appendChild(v);
                }
                infoRows.appendChild(row);
            });
            // Restart the pop animation on every (re)open, even same target.
            infoEl.classList.remove('sw-pop');
            void infoEl.offsetWidth;
            infoEl.classList.add('sw-pop');
            infoEl.classList.add('open');
        }

        function hidePanel() {
            infoEl.classList.remove('open');
        }

        function isPanelOpen() {
            return infoEl.classList.contains('open');
        }

        // ── Action buttons ───────────────────────────────────────────────────

        function setQualityLabel(q) {
            const label = L('sysworld.btn.quality') + ': ' + L('sysworld.quality.' + q);
            btnQuality.textContent = label;
            btnQuality.title = label;
            btnQuality.setAttribute('aria-label', label);
        }

        function setGraphVisible(visible) {
            btnGraph.classList.toggle('active', !!visible);
        }

        function setEffectsEnabled(enabled) {
            btnEffects.classList.toggle('active', !!enabled);
        }

        function toggleGraph() {
            const visible = inst.graphVisible === false;
            inst.graphVisible = visible;
            try {
                if (inst.graph && typeof inst.graph.setVisible === 'function') inst.graph.setVisible(visible);
            } catch (_) {}
            setGraphVisible(visible);
            notify('onGraphToggle', visible);
        }

        function toggleEffects() {
            const enabled = inst.effectsEnabled === false;
            inst.effectsEnabled = enabled;
            setEffectsEnabled(enabled);
            notify('onEffectsToggle', enabled);
        }

        // Cycles low → medium → high → low, persists and applies immediately.
        function cycleQuality() {
            const cur = QUALITY_ORDER.indexOf(inst.quality) >= 0 ? inst.quality : 'high';
            const next = QUALITY_ORDER[(QUALITY_ORDER.indexOf(cur) + 1) % QUALITY_ORDER.length];
            inst.quality = next;
            inst.qualityScale = QUALITY_SCALES[next] || 1;
            try { localStorage.setItem(QUALITY_STORAGE_KEY, next); } catch (_) {}
            try {
                if (inst.fx && typeof inst.fx.setQuality === 'function') inst.fx.setQuality(next);
            } catch (_) {}
            setQualityLabel(next);
            notify('onQualityChange', next);
        }

        const onActionsClick = ev => {
            const target = ev.target;
            const btn = target && typeof target.closest === 'function'
                ? target.closest('[data-sw-action]')
                : null;
            if (!btn || !actionsEl.contains(btn)) return;
            const action = btn.getAttribute('data-sw-action');
            if (action === 'overview') {
                try {
                    if (inst.stage && typeof inst.stage.resetView === 'function') inst.stage.resetView();
                } catch (_) {}
                notify('onOverview');
            } else if (action === 'graph') {
                toggleGraph();
            } else if (action === 'effects') {
                toggleEffects();
            } else if (action === 'quality') {
                cycleQuality();
            }
        };
        actionsEl.addEventListener('click', onActionsClick);

        const onCloseClick = () => {
            hidePanel();
            notify('onPanelClose');
        };
        infoClose.addEventListener('click', onCloseClick);

        // Initial state mirrors the instance flags (defaults: graph on, effects on).
        setQualityLabel(QUALITY_ORDER.indexOf(inst.quality) >= 0 ? inst.quality : 'high');
        setGraphVisible(inst.graphVisible !== false);
        setEffectsEnabled(inst.effectsEnabled !== false);
        setLegend();
        showEmpty();

        // ── Dispose ──────────────────────────────────────────────────────────

        function dispose() {
            if (disposed) return;
            disposed = true;
            eventTimers.forEach(id => clearTimeout(id));
            eventTimers.length = 0;
            try { legendEl.removeEventListener('mouseover', onLegendOver); } catch (_) {}
            try { legendEl.removeEventListener('mouseout', onLegendOut); } catch (_) {}
            try { legendEl.removeEventListener('click', onLegendClick); } catch (_) {}
            try { actionsEl.removeEventListener('click', onActionsClick); } catch (_) {}
            try { infoClose.removeEventListener('click', onCloseClick); } catch (_) {}
            try { root.remove(); } catch (_) {}
            try { infoEl.remove(); } catch (_) {}
        }

        return {
            el: root,
            infoEl,
            btnOverview,
            setStats,
            event,
            setLegend,
            showTooltip,
            hideTooltip,
            showSelLabel,
            positionSelLabel,
            hideSelLabel,
            showPanel,
            hidePanel,
            isPanelOpen,
            setQualityLabel,
            setGraphVisible,
            setEffectsEnabled,
            dispose
        };
    };
})();
