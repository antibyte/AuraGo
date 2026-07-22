(function () {
    'use strict';

    // system-world entry: app lifecycle, data polling, SSE wiring, pointer
    // interaction and the single RAF loop. The sibling sysworld-*.js modules
    // attach their factories to window.SysWorld and load before this file
    // (see ui/js/desktop/core/module-loader.js, app id 'system-world').

    const instances = new Map();

    const QUALITY_STORAGE_KEY = 'aurago.desktop.sysworld.quality';
    const QUALITY_SCALES = { low: 0.4, medium: 0.7, high: 1.0, ultra: 1.6 };
    const MAX_DT = 0.05;
    const HOVER_THROTTLE_MS = 40;
    const CLICK_SLOP_PX = 5;
    const API_TIMEOUT_MS = 10000;
    const LOADING_FAILSAFE_MS = 15000;
    const HOVER_SCALE = 1.18;
    const FOCUS_DURATION = 1.2;

    const CATEGORY_KEYS = { communication: 1, smarthome: 1, infrastructure: 1, ai: 1, storage: 1, monitoring: 1, other: 1 };
    const STATE_KEYS = { running: 1, queued: 1, idle: 1, error: 1, done: 1, waiting: 1, exited: 1, paused: 1 };
    const AMBIENT_COMET_MS = 6500;

    // Accent color per pickable kind — drives beacon, tooltip dot and the
    // info panel's accent theming so a selection reads as one color story.
    function accentHexFor(ud) {
        const P = NS().PALETTE || {};
        const fallback = P.core != null ? P.core : 0x59d4ff;
        if (!ud || !ud.kind) return fallback;
        if (ud.kind === 'integration') {
            return P[ud.category] != null ? P[ud.category] : (P.other != null ? P.other : fallback);
        }
        switch (ud.kind) {
            case 'kgnode': return P.other != null ? P.other : fallback;
            case 'mission': return P.mission != null ? P.mission : fallback;
            case 'coagent': return P.agent != null ? P.agent : fallback;
            case 'container':
            case 'daemon': return P.infrastructure != null ? P.infrastructure : fallback;
            case 'tool': return P.tool != null ? P.tool : fallback;
            case 'cron': return P.cron != null ? P.cron : fallback;
            default: return fallback;
        }
    }

    function hexCss(hex) {
        return '#' + ((Number(hex) >>> 0) & 0xffffff).toString(16).padStart(6, '0');
    }

    // Maps a raw state word onto a pill tone for the detail panel.
    function toneForState(v) {
        const s = String(v == null ? '' : v).toLowerCase();
        if (s === 'running' || s === 'done' || s === 'enabled' || s === 'active' || s === 'ok') return 'ok';
        if (s === 'error' || s === 'failed' || s === 'disabled' || s === 'exited') return 'err';
        if (s === 'queued' || s === 'waiting' || s === 'paused' || s === 'restarting') return 'warn';
        return 'dim';
    }

    // Relative time via sysworld.time.in / sysworld.time.ago ("in 3h" / "3h ago").
    // Returns null for missing/unparseable timestamps so callers can fall back.
    function relTime(inst, ts) {
        if (ts == null || ts === '') return null;
        const d = new Date(ts);
        if (isNaN(d.getTime())) return null;
        const diff = d.getTime() - Date.now();
        const abs = Math.abs(diff);
        let span;
        if (abs < 90000) span = Math.max(1, Math.round(abs / 1000)) + 's';
        else if (abs < 3600000) span = Math.round(abs / 60000) + 'm';
        else if (abs < 86400000) span = Math.round(abs / 3600000) + 'h';
        else span = Math.round(abs / 86400000) + 'd';
        const key = diff >= 0 ? 'sysworld.time.in' : 'sysworld.time.ago';
        const phrase = inst.L(key);
        return phrase.indexOf('{{time}}') >= 0 ? phrase.replace('{{time}}', span) : span;
    }

    const NS = () => window.SysWorld || {};

    // ── Small helpers ────────────────────────────────────────────────────────

    function fallbackEsc(value) {
        return String(value == null ? '' : value).replace(/[&<>"']/g, ch => (
            { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]
        ));
    }

    function normalizeContext(context) {
        const ctx = Object.assign({}, context || {});
        if (typeof ctx.esc !== 'function') ctx.esc = fallbackEsc;
        if (typeof ctx.t !== 'function') ctx.t = key => key;
        return ctx;
    }

    function pickNum() {
        for (let i = 0; i < arguments.length; i++) {
            const v = arguments[i];
            if (typeof v === 'number' && isFinite(v)) return v;
            if (typeof v === 'string' && v.trim() !== '' && isFinite(Number(v))) return Number(v);
        }
        return undefined;
    }

    // Accepts an array payload or a wrapper object holding the list.
    function asList(data, key) {
        if (Array.isArray(data)) return data;
        if (data && Array.isArray(data[key])) return data[key];
        if (data && Array.isArray(data.items)) return data.items;
        if (data && Array.isArray(data.list)) return data.list;
        return [];
    }

    function webglAvailable() {
        try {
            const canvas = document.createElement('canvas');
            return !!(window.WebGLRenderingContext && (
                canvas.getContext('webgl2') ||
                canvas.getContext('webgl') ||
                canvas.getContext('experimental-webgl')
            ));
        } catch (_) {
            return false;
        }
    }

    function readQuality() {
        try {
            const q = localStorage.getItem(QUALITY_STORAGE_KEY);
            if (QUALITY_SCALES[q]) return q;
        } catch (_) {}
        return 'high';
    }

    // ── Render ───────────────────────────────────────────────────────────────

    function render(container, windowId, context = {}) {
        dispose(windowId);
        const ctx = normalizeContext(context);
        const t = key => {
            try {
                const v = ctx.t(key);
                return typeof v === 'string' ? v : key;
            } catch (_) {
                return key;
            }
        };

        // Hard requirements: vendored THREE plus a working WebGL context.
        if (!window.THREE || !webglAvailable()) {
            const fallbackRoot = document.createElement('div');
            fallbackRoot.className = 'sysworld';
            const fallback = document.createElement('div');
            fallback.className = 'sw-fallback';
            fallback.textContent = t('sysworld.no_webgl');
            fallbackRoot.appendChild(fallback);
            container.innerHTML = '';
            container.appendChild(fallbackRoot);
            instances.set(windowId, { fallback: true, root: fallbackRoot, disposed: false });
            return;
        }

        const quality = readQuality();
        const root = document.createElement('div');
        root.className = 'sysworld';
        const canvasHost = document.createElement('div');
        canvasHost.className = 'sysworld-canvas';
        const vignette = document.createElement('div');
        vignette.className = 'sysworld-vignette';
        const loading = document.createElement('div');
        loading.className = 'sw-loading';
        const spinner = document.createElement('div');
        spinner.className = 'sw-loading-spinner';
        loading.appendChild(spinner);
        root.appendChild(canvasHost);
        root.appendChild(vignette);
        root.appendChild(loading);
        container.innerHTML = '';
        container.appendChild(root);

        const inst = {
            container,
            windowId,
            ctx,
            root,
            canvasHost,
            THREE: window.THREE,
            quality,
            qualityScale: QUALITY_SCALES[quality] || 1,
            effectsEnabled: true,
            graphVisible: true,
            data: {},
            timers: [],
            disposers: [],
            paused: false,
            disposed: false,
            elapsed: 0,
            rafId: 0,
            panelKind: null,
            panelToken: 0,
            hovered: null,
            loadingEl: loading,
            sseHandlers: {},
            debouncers: {},
            aborts: new Set(),
            intersectionObserver: null,
            docHidden: !!document.hidden,
            inView: true,
            L: t,
            apiGet: null,
            focusObject: null,
            focused: null,
            follow: null,
            lastActivity: Date.now()
        };
        instances.set(windowId, inst);
        inst.apiGet = path => apiGet(inst, path);
        inst.focusObject = mesh => focusObject(inst, mesh);
        // HUD callback hooks (behaviour itself lives in the HUD module).
        inst.hudHandlers = {
            onPanelClose: () => {
                inst.panelKind = null;
                inst.panelToken++;
            },
            onZoneHover: zone => {
                if (!zone || !inst.fx || typeof inst.fx.pulseRing !== 'function') return;
                const anchor = zoneAnchor(inst, zone);
                if (!anchor) return;
                const THREE = inst.THREE;
                const P = NS().PALETTE || {};
                const hex = P[anchor.palette] != null ? P[anchor.palette] : (P.core || 0x59d4ff);
                try {
                    inst.fx.pulseRing(new THREE.Vector3(anchor.tgt[0], anchor.tgt[1], anchor.tgt[2]), hex, 16);
                } catch (_) {}
            },
            onZoneFocus: zone => {
                const anchor = zoneAnchor(inst, zone);
                if (!anchor || !inst.stage || typeof inst.stage.flyTo !== 'function') return;
                clearFollow(inst);
                const THREE = inst.THREE;
                try {
                    inst.stage.flyTo(
                        new THREE.Vector3(anchor.cam[0], anchor.cam[1], anchor.cam[2]),
                        new THREE.Vector3(anchor.tgt[0], anchor.tgt[1], anchor.tgt[2]),
                        1.8
                    );
                } catch (_) {}
            },
            onQualityChange: () => {
                applyQuality(inst);
            }
        };

        const ns = NS();
        try {
            inst.fx = ns.createFx(inst);
            inst.stage = ns.createStage(inst);
            inst.core = ns.createCore(inst);
            inst.orbit = ns.createOrbit(inst);
            inst.graph = ns.createGraph(inst);
            inst.fleet = ns.createFleet(inst);
            inst.hud = ns.createHud(inst);
        } catch (err) {
            console.error('[SysWorld] module init failed:', err);
            renderFallback(root, t, 'sysworld.init_error');
            instances.set(windowId, { fallback: true, root, disposed: false });
            return;
        }
        // Keep the loading overlay on top of the HUD until first data arrives.
        root.appendChild(loading);
        try { inst.stage.introFlight(); } catch (_) {}

        applyQuality(inst);
        wireInteraction(inst);
        wireSse(inst);
        startPolling(inst);
        startLoop(inst);
        watchVisibility(inst);
    }

    function renderFallback(root, t, messageKey) {
        root.innerHTML = '';
        const fallback = document.createElement('div');
        fallback.className = 'sw-fallback';
        fallback.textContent = t(messageKey || 'sysworld.no_webgl');
        root.appendChild(fallback);
    }

    // ── API (never throws, 10s timeout, aborted on dispose) ──────────────────

    function apiGet(inst, path) {
        if (inst.disposed) return Promise.resolve(null);
        let ctl = null;
        try {
            ctl = new AbortController();
            inst.aborts.add(ctl);
        } catch (_) {
            ctl = null;
        }
        const timer = setTimeout(() => {
            try { if (ctl) ctl.abort(); } catch (_) {}
        }, API_TIMEOUT_MS);
        return fetch(path, { credentials: 'same-origin', signal: ctl ? ctl.signal : undefined })
            .then(resp => {
                if (!resp || !resp.ok) return null;
                return resp.json().catch(() => null);
            })
            .catch(() => null)
            .then(data => {
                clearTimeout(timer);
                if (ctl) inst.aborts.delete(ctl);
                return inst.disposed ? null : data;
            });
    }

    function hideLoading(inst) {
        if (!inst.loadingEl) return;
        try { inst.loadingEl.remove(); } catch (_) {}
        inst.loadingEl = null;
    }

    // ── Polling ──────────────────────────────────────────────────────────────

    function poll(inst, intervalMs, fn) {
        fn();
        const id = setInterval(() => {
            if (!inst.disposed) fn();
        }, intervalMs);
        inst.timers.push(id);
    }

    function agentBusyOf(agent) {
        if (!agent || typeof agent !== 'object') return undefined;
        const raw = agent.busy != null ? agent.busy
            : (agent.is_busy != null ? agent.is_busy : agent.processing);
        if (raw != null) return !!raw;
        if (typeof agent.status === 'string') {
            const s = agent.status.toLowerCase();
            return s === 'busy' || s === 'running' || s === 'active' || s === 'processing';
        }
        return undefined;
    }

    function normalizeBudget(d) {
        if (!d || typeof d !== 'object') return undefined;
        const spent = pickNum(d.spent, d.spent_usd, d.used, d.used_usd, d.total_spent,
            d.cost, d.cost_usd, d.month_spent, d.month_spent_usd);
        const limit = pickNum(d.limit, d.limit_usd, d.budget, d.budget_usd,
            d.monthly_limit, d.monthly_limit_usd, d.max_usd);
        if (spent == null && limit == null) return undefined;
        return { spent, limit };
    }

    // Dashboard/SSE system_metrics payloads use nested cpu/memory objects
    // (usage_percent / used_percent). Accept flat numbers too for robustness.
    function normalizeSystemMetrics(payload) {
        if (!payload || typeof payload !== 'object') return null;
        const cpuObj = payload.cpu && typeof payload.cpu === 'object' ? payload.cpu : null;
        const memObj = payload.memory && typeof payload.memory === 'object' ? payload.memory : null;
        const cpu = pickNum(
            typeof payload.cpu === 'number' ? payload.cpu : undefined,
            payload.cpu_percent,
            cpuObj && cpuObj.usage_percent,
            cpuObj && cpuObj.percent,
            cpuObj && cpuObj.used_percent
        );
        const ram = pickNum(
            typeof payload.memory === 'number' ? payload.memory : undefined,
            payload.memory_percent,
            payload.ram,
            memObj && memObj.used_percent,
            memObj && memObj.percent,
            memObj && memObj.usage_percent
        );
        const uptime = pickNum(payload.uptime_seconds, payload.uptime);
        if (cpu == null && ram == null && uptime == null) return null;
        return { cpu, ram, uptime };
    }

    function applySystemMetrics(inst, payload) {
        const m = normalizeSystemMetrics(payload);
        if (!m) return;
        try {
            if (inst.core && typeof inst.core.setMetrics === 'function') {
                inst.core.setMetrics({
                    cpu: m.cpu != null ? m.cpu : 0,
                    memory: m.ram != null ? m.ram : 0
                });
            }
        } catch (_) {}
        const patch = {};
        if (m.cpu != null) patch.cpu = m.cpu;
        if (m.ram != null) patch.ram = m.ram;
        if (m.uptime != null) patch.uptime = m.uptime;
        if (Object.keys(patch).length) {
            try { inst.hud.setStats(patch); } catch (_) {}
        }
    }

    function fetchOverview(inst) {
        inst.apiGet('/api/dashboard/overview').then(d => {
            if (inst.disposed) return;
            if (!d) { hideLoading(inst); return; }
            inst.data.overview = d;
            try { if (inst.core.setAgent) inst.core.setAgent(d.agent || null); } catch (_) {}
            try { if (inst.orbit.setIntegrations) inst.orbit.setIntegrations(d.integrations || {}); } catch (_) {}
            try {
                inst.hud.setStats({ missions: d.missions, agentBusy: agentBusyOf(d.agent) });
            } catch (_) {}
            hideLoading(inst);
        });
    }

    function fetchMemory(inst) {
        inst.apiGet('/api/dashboard/memory').then(d => {
            if (!d || inst.disposed) return;
            inst.data.memory = d;
            const mem = {
                vectordb_entries: pickNum(d.vectordb_entries, d.vector_entries, d.entries),
                core_memory_facts: pickNum(d.core_memory_facts, d.core_facts, d.facts),
                journal_entries: pickNum(d.journal_entries, d.journal)
            };
            try { if (inst.core.setMemory) inst.core.setMemory(mem); } catch (_) {}
            const parts = [mem.vectordb_entries, mem.core_memory_facts, mem.journal_entries];
            const any = parts.some(v => typeof v === 'number');
            const total = parts.reduce((sum, v) => sum + (typeof v === 'number' ? v : 0), 0);
            try { inst.hud.setStats({ memories: any ? total : undefined }); } catch (_) {}
        });
    }

    function fetchActivity(inst) {
        inst.apiGet('/api/dashboard/activity').then(d => {
            if (!d || inst.disposed) return;
            inst.data.activity = d;
            try { if (inst.fleet.setCoAgents) inst.fleet.setCoAgents(asList(d, 'coagents')); } catch (_) {}
            try { if (inst.fleet.setCron) inst.fleet.setCron(asList(d, 'cron_jobs')); } catch (_) {}
        });
    }

    function fetchMissions(inst) {
        inst.apiGet('/api/missions/v2').then(d => {
            if (!d || inst.disposed) return;
            const missions = asList(d, 'missions');
            inst.data.missions = missions;
            try { if (inst.fleet.setMissions) inst.fleet.setMissions(missions); } catch (_) {}
        });
    }

    function fetchTools(inst) {
        inst.apiGet('/api/dashboard/tool-stats').then(d => {
            if (!d || inst.disposed) return;
            const tools = asList(d, 'top_tools');
            inst.data.tools = tools;
            try { if (inst.fleet.setTools) inst.fleet.setTools(tools); } catch (_) {}
        });
    }

    function fetchInfra(inst) {
        Promise.all([inst.apiGet('/api/containers'), inst.apiGet('/api/daemons')]).then(results => {
            if (inst.disposed || !results) return;
            inst.data.infra = {
                containers: asList(results[0], 'containers'),
                daemons: asList(results[1], 'daemons')
            };
            try { if (inst.fleet.setInfra) inst.fleet.setInfra(inst.data.infra); } catch (_) {}
        });
    }

    function fetchGraph(inst) {
        // Never rebuild while a KG node panel is open: it would yank the camera target.
        if (inst.panelKind === 'kgnode' && inst.hud && inst.hud.isPanelOpen && inst.hud.isPanelOpen()) return;
        Promise.all([
            inst.apiGet('/api/knowledge-graph/nodes?limit=300'),
            inst.apiGet('/api/knowledge-graph/edges?limit=500')
        ]).then(results => {
            if (inst.disposed || !results || !results[0]) return;
            if (inst.panelKind === 'kgnode' && inst.hud && inst.hud.isPanelOpen && inst.hud.isPanelOpen()) return;
            const nodes = asList(results[0], 'nodes');
            const edges = asList(results[1], 'edges');
            inst.data.graph = { nodes, edges };
            try { if (inst.graph.build) inst.graph.build(nodes, edges); } catch (_) {}
        });
    }

    function fetchPersonality(inst) {
        inst.apiGet('/api/personality/state').then(d => {
            if (!d || inst.disposed) return;
            inst.data.personality = d;
            try { if (inst.core.setMood) inst.core.setMood(d.mood); } catch (_) {}
        });
    }

    function fetchBudget(inst) {
        inst.apiGet('/api/budget').then(d => {
            if (!d || inst.disposed) return;
            inst.data.budget = d;
            try { inst.hud.setStats({ budget: normalizeBudget(d) }); } catch (_) {}
        });
    }

    function fetchSystem(inst) {
        inst.apiGet('/api/dashboard/system').then(d => {
            if (!d || inst.disposed) return;
            inst.data.system = d;
            applySystemMetrics(inst, d);
        });
    }

    function startPolling(inst) {
        poll(inst, 5000, () => fetchOverview(inst));
        // CPU/RAM/uptime: REST bootstrap + backup poll; SSE keeps them live.
        poll(inst, 8000, () => fetchSystem(inst));
        poll(inst, 12000, () => fetchMemory(inst));
        poll(inst, 4000, () => fetchActivity(inst));
        poll(inst, 10000, () => fetchMissions(inst));
        poll(inst, 30000, () => fetchTools(inst));
        poll(inst, 20000, () => fetchInfra(inst));
        poll(inst, 90000, () => fetchGraph(inst));
        poll(inst, 20000, () => fetchPersonality(inst));
        poll(inst, 30000, () => fetchBudget(inst));
        // Never leave the loading veil up when the backend is unreachable.
        inst.timers.push(setTimeout(() => hideLoading(inst), LOADING_FAILSAFE_MS));
        // Ambient shooting stars (dice-rolled so the sky stays irregular).
        inst.timers.push(setInterval(() => {
            if (!inst.disposed && !inst.paused && Math.random() < 0.6) fireAmbientComet(inst);
        }, AMBIENT_COMET_MS));
    }

    // ── SSE wiring (shared AuraSSE client, all handlers off'ed on dispose) ────

    function debounce(inst, key, ms, fn) {
        if (inst.debouncers[key]) clearTimeout(inst.debouncers[key]);
        inst.debouncers[key] = setTimeout(() => {
            delete inst.debouncers[key];
            if (!inst.disposed) fn();
        }, ms);
    }

    function toolNameOf(payload) {
        if (!payload || typeof payload !== 'object') return null;
        if (typeof payload === 'string') return payload;
        const name = payload.tool || payload.tool_name || payload.name ||
            (payload.call && (payload.call.tool || payload.call.name)) ||
            (typeof payload.action === 'string' ? payload.action : null);
        return name ? String(name) : null;
    }

    function corePosition(inst, out) {
        try {
            if (inst.core && inst.core.group) inst.core.group.getWorldPosition(out);
        } catch (_) {}
        return out;
    }

    // Camera anchor + pulse palette per legend zone (hover pulses, click flies).
    function zoneAnchor(inst, zone) {
        const L = NS().LAYOUT || {};
        switch (zone) {
            case 'core': return { cam: [0, 22, 52], tgt: [0, 2, 0], palette: 'core' };
            case 'integrations': return { cam: [0, 34, 96], tgt: [0, 2, 0], palette: 'communication' };
            case 'graph': {
                const c = L.graphCenter || { x: 0, y: 34, z: -120 };
                return { cam: [c.x, c.y + 30, c.z + 85], tgt: [c.x, c.y, c.z], palette: 'other' };
            }
            case 'memory': return { cam: [0, 14, 34], tgt: [0, 3, 0], palette: 'memory' };
            case 'missions': {
                const r = L.missionRingRadius || 84;
                return { cam: [0, 26, r + 70], tgt: [0, 0, r * 0.8], palette: 'mission' };
            }
            case 'agents': return { cam: [26, 20, 48], tgt: [0, 5, 0], palette: 'agent' };
            case 'tools': {
                const r = L.beltRadius || 130;
                return { cam: [0, 55, r + 90], tgt: [0, 0, r * 0.75], palette: 'tool' };
            }
            case 'infra': {
                const y = L.infraY != null ? L.infraY : -34;
                return { cam: [0, y + 26, 95], tgt: [0, y, 0], palette: 'infrastructure' };
            }
            default: return null;
        }
    }

    // Ambient shooting stars: slow distant comets keep the far field alive.
    function fireAmbientComet(inst) {
        if (!inst.effectsEnabled || !inst.fx || typeof inst.fx.comet !== 'function') return;
        const THREE = inst.THREE;
        const mk = () => {
            const r = 300 + Math.random() * 260;
            const th = Math.random() * Math.PI * 2;
            const ph = Math.acos(Math.random() * 2 - 1);
            return new THREE.Vector3(
                r * Math.sin(ph) * Math.cos(th),
                Math.abs(r * Math.cos(ph)) * 0.7 + 30,
                r * Math.sin(ph) * Math.sin(th)
            );
        };
        const hex = (NS().PALETTE && NS().PALETTE.communication) || 0x4fc3f7;
        try { inst.fx.comet(mk(), mk(), hex, { size: 1.5, arc: 0.04, duration: 2.4 }); } catch (_) {}
    }

    // Comet from the agent core towards the tool mesh (falling back to a random
    // enabled satellite, then to a point on the tool belt).
    function fireToolComet(inst, toolName) {
        if (!inst.effectsEnabled) return;
        const fx = inst.fx;
        if (!fx || typeof fx.comet !== 'function') return;
        const THREE = inst.THREE;
        const from = corePosition(inst, new THREE.Vector3());
        from.x += 2;
        from.y += 2;
        let to = null;
        if (toolName && inst.fleet && typeof inst.fleet.flashTool === 'function') {
            try { to = inst.fleet.flashTool(toolName); } catch (_) { to = null; }
        }
        if (!to) {
            const integrations = (inst.data.overview && inst.data.overview.integrations) || {};
            const ids = Object.keys(integrations).filter(id => integrations[id] && integrations[id].enabled !== false);
            if (ids.length && inst.orbit && typeof inst.orbit.satellitePosition === 'function') {
                const id = ids[Math.floor(Math.random() * ids.length)];
                try { to = inst.orbit.satellitePosition(id); } catch (_) { to = null; }
            }
        }
        if (!to) {
            const radius = (NS().LAYOUT && NS().LAYOUT.beltRadius) || 130;
            const angle = Math.random() * Math.PI * 2;
            to = new THREE.Vector3(Math.cos(angle) * radius, 0, Math.sin(angle) * radius);
        }
        const hex = (NS().PALETTE && NS().PALETTE.tool) || 0xfff176;
        try { fx.comet(from, to, hex); } catch (_) {}
    }

    function wireSse(inst) {
        const sse = window.AuraSSE;
        if (!sse || typeof sse.on !== 'function') return;
        const reg = (type, fn) => {
            inst.sseHandlers[type] = fn;
            try { sse.on(type, fn); } catch (_) {}
        };

        reg('system_metrics', payload => {
            if (inst.disposed || !payload) return;
            applySystemMetrics(inst, payload);
        });

        reg('agent_status', payload => {
            if (inst.disposed) return;
            try { if (inst.core.punch) inst.core.punch(1); } catch (_) {}
            const busy = agentBusyOf(payload);
            const text = payload && typeof payload.message === 'string' && payload.message
                ? payload.message
                : inst.L('sysworld.stats.agent') + ': ' +
                    inst.L(busy === false ? 'sysworld.agent.idle' : 'sysworld.agent.busy');
            try { inst.hud.event(text, busy === false ? 'ok' : 'info'); } catch (_) {}
            if (busy !== undefined) {
                try { inst.hud.setStats({ agentBusy: busy }); } catch (_) {}
            }
        });

        reg('mission_update', () => {
            if (inst.disposed) return;
            debounce(inst, 'missions', 2000, () => fetchMissions(inst));
        });

        reg('coagent_progress', () => {
            if (inst.disposed) return;
            debounce(inst, 'activity', 2000, () => fetchActivity(inst));
        });

        reg('memory_update', () => {
            if (inst.disposed) return;
            try { if (inst.core.memoryFlash) inst.core.memoryFlash(); } catch (_) {}
            debounce(inst, 'graph', 5000, () => fetchGraph(inst));
        });

        reg('budget_update', payload => {
            if (inst.disposed || !payload) return;
            inst.data.budget = payload;
            try { inst.hud.setStats({ budget: normalizeBudget(payload) }); } catch (_) {}
        });

        const onToolEvent = payload => {
            if (inst.disposed) return;
            const name = toolNameOf(payload);
            fireToolComet(inst, name);
            if (name) {
                try { inst.hud.event(inst.L('sysworld.zone.tools') + ': ' + name, 'info'); } catch (_) {}
            }
        };
        reg('tool_call_preview', onToolEvent);
        reg('agent_action', onToolEvent);

        reg('system_warning', payload => {
            if (inst.disposed) return;
            const text = (payload && (payload.message || payload.text || payload.warning || payload.title)) ||
                'system_warning';
            try { inst.hud.event(String(text), 'err'); } catch (_) {}
            if (inst.fx && typeof inst.fx.pulseRing === 'function') {
                const origin = corePosition(inst, new inst.THREE.Vector3());
                const hex = (NS().PALETTE && NS().PALETTE.error) || 0xef5350;
                try { inst.fx.pulseRing(origin, hex, 40); } catch (_) {}
            }
        });
    }

    // ── Interaction ──────────────────────────────────────────────────────────

    function collectPickables(inst) {
        const out = [];
        ['orbit', 'graph', 'fleet'].forEach(key => {
            const mod = inst[key];
            const list = mod && mod.pickables;
            if (Array.isArray(list)) {
                for (let i = 0; i < list.length; i++) {
                    if (list[i]) out.push(list[i]);
                }
            }
        });
        return out;
    }

    function pick(inst, clientX, clientY) {
        const stage = inst.stage;
        if (!stage || typeof stage.raycast !== 'function' || typeof stage.screenToNDC !== 'function') return null;
        let ndc = null;
        let hit = null;
        try {
            ndc = stage.screenToNDC(clientX, clientY);
            if (ndc) hit = stage.raycast(collectPickables(inst), ndc);
        } catch (_) {
            hit = null;
        }
        if (!hit) return null;
        return hit.object || hit;
    }

    // Hover highlight: simple scale pop, restored exactly on hover-out.
    function setHovered(inst, mesh) {
        const prev = inst.hovered;
        if (prev && prev.mesh === mesh) return;
        if (prev) {
            try { prev.mesh.scale.copy(prev.scale); } catch (_) {}
            inst.hovered = null;
        }
        if (mesh && mesh.scale) {
            const radius = objectRadius(inst, mesh);
            inst.hovered = { mesh, scale: mesh.scale.clone() };
            try { mesh.scale.multiplyScalar(HOVER_SCALE); } catch (_) {}
            try {
                if (inst.fx && inst.fx.hoverRing) inst.fx.hoverRing(mesh, radius, accentHexFor(mesh.userData || {}));
            } catch (_) {}
        } else {
            try { if (inst.fx && inst.fx.hoverRing) inst.fx.hoverRing(null); } catch (_) {}
        }
    }

    function tooltipHtml(inst, ud) {
        const esc = inst.ctx.esc;
        const label = esc(String(ud.label != null ? ud.label : (ud.id != null ? ud.id : '')));
        const accent = hexCss(accentHexFor(ud));
        const dot = '<span class="sw-tt-dot" style="background:' + accent +
            ';box-shadow:0 0 8px ' + accent + '"></span>';
        let sub = '';
        if (ud.kind === 'integration') {
            const cat = CATEGORY_KEYS[ud.category] ? inst.L('sysworld.cat.' + ud.category) : esc(String(ud.category || ''));
            sub = cat + ' · ' + inst.L(ud.enabled !== false ? 'sysworld.panel.enabled' : 'sysworld.panel.disabled');
        } else if (ud.kind === 'kgnode') {
            sub = esc(String(ud.type || ''));
        } else if (ud.payload && (ud.payload.state || ud.payload.status)) {
            sub = esc(String(ud.payload.state || ud.payload.status));
        }
        return sub
            ? dot + '<strong>' + label + '</strong><span class="sw-tt-sub">' + sub + '</span>'
            : dot + '<strong>' + label + '</strong>';
    }

    function handleHover(inst, clientX, clientY) {
        if (inst.disposed || inst.paused) return;
        const mesh = pick(inst, clientX, clientY);
        setHovered(inst, mesh);
        // KG neighborhood highlight (deduped — only fires on target change).
        const hlId = mesh && mesh.userData && mesh.userData.kind === 'kgnode'
            ? String(mesh.userData.id)
            : null;
        if (hlId !== inst._kgHover) {
            inst._kgHover = hlId;
            try {
                if (inst.graph && typeof inst.graph.highlightNeighbors === 'function') {
                    inst.graph.highlightNeighbors(hlId);
                }
            } catch (_) {}
        }
        if (!inst.hud) return;
        if (mesh && mesh.userData && mesh.userData.kind) {
            try { inst.hud.showTooltip(tooltipHtml(inst, mesh.userData), clientX, clientY); } catch (_) {}
        } else {
            try { inst.hud.hideTooltip(); } catch (_) {}
        }
        inst.canvasHost.style.cursor = mesh ? 'pointer' : '';
    }

    // Clicks a HUD action button by action name (used by keyboard shortcuts).
    function hudAction(inst, action) {
        try {
            const btn = inst.hud && inst.hud.el
                ? inst.hud.el.querySelector('[data-sw-action="' + action + '"]')
                : null;
            if (btn) btn.click();
        } catch (_) {}
    }

    // Applies every live quality lever of the current tier to the modules.
    // Buffers are always allocated at ultra capacity, so tier switches are
    // instant: draw ranges, structure visibility, fx caps, pixel ratio and
    // the ambient effects rate all change without a rebuild.
    function applyQuality(inst) {
        const tier = inst.quality || 'high';
        try { if (inst.fx && typeof inst.fx.setQuality === 'function') inst.fx.setQuality(tier); } catch (_) {}
        try { if (inst.stage && typeof inst.stage.setQuality === 'function') inst.stage.setQuality(tier); } catch (_) {}
        try { if (inst.core && typeof inst.core.setQuality === 'function') inst.core.setQuality(tier); } catch (_) {}
        try { if (inst.fleet && typeof inst.fleet.setQuality === 'function') inst.fleet.setQuality(tier); } catch (_) {}
        inst._ambientRateMul = tier === 'low' ? 1.6 : tier === 'medium' ? 1.2 : tier === 'ultra' ? 0.3 : 1;
    }

    // Arrow-key cycling through every pickable in a stable (kind, id) order.
    function cycleFocus(inst, dir) {
        const list = collectPickables(inst).filter(m => m && m.parent);
        if (!list.length) return;
        list.sort((a, b) => {
            const ka = ((a.userData && a.userData.kind) || '') + ':' + ((a.userData && a.userData.id) || '');
            const kb = ((b.userData && b.userData.kind) || '') + ':' + ((b.userData && b.userData.id) || '');
            return ka < kb ? -1 : (ka > kb ? 1 : 0);
        });
        let ix = inst.focused && inst.focused.mesh ? list.indexOf(inst.focused.mesh) : -1;
        ix = (ix + dir + list.length) % list.length;
        focusObject(inst, list[ix]);
    }

    function clearFollow(inst) {
        if (!inst.follow) return;
        inst.follow = null;
        // Pan was disabled while chasing a moving target; restore it.
        try { if (inst.stage && inst.stage.controls) inst.stage.controls.enablePan = true; } catch (_) {}
    }

    function clearFocus(inst) {
        inst.panelKind = null;
        inst.panelToken++;
        inst.focused = null;
        clearFollow(inst);
        try { if (inst.fx && inst.fx.clearBeacon) inst.fx.clearBeacon(); } catch (_) {}
        try { if (inst.hud && inst.hud.hideSelLabel) inst.hud.hideSelLabel(); } catch (_) {}
        try { if (inst.hud) inst.hud.hidePanel(); } catch (_) {}
    }

    function wireInteraction(inst) {
        const host = inst.canvasHost;
        let lastHover = 0;
        let down = false;
        let downX = 0;
        let downY = 0;

        const onMove = ev => {
            inst.lastActivity = Date.now();
            const now = Date.now();
            if (now - lastHover < HOVER_THROTTLE_MS) return;
            lastHover = now;
            handleHover(inst, ev.clientX, ev.clientY);
        };
        const onDown = ev => {
            inst.lastActivity = Date.now();
            down = true;
            downX = ev.clientX;
            downY = ev.clientY;
        };
        // Click = pointerup near pointerdown (<5px travel); anything else was a drag.
        const onUp = ev => {
            inst.lastActivity = Date.now();
            if (!down) return;
            down = false;
            if (Math.abs(ev.clientX - downX) > CLICK_SLOP_PX || Math.abs(ev.clientY - downY) > CLICK_SLOP_PX) return;
            const mesh = pick(inst, ev.clientX, ev.clientY);
            if (mesh) {
                inst.focusObject(mesh);
            } else {
                clearFocus(inst);
            }
        };
        // Double-click on empty space resets the camera to the overview.
        const onDbl = ev => {
            inst.lastActivity = Date.now();
            const mesh = pick(inst, ev.clientX, ev.clientY);
            if (!mesh) {
                clearFocus(inst);
                try { if (inst.stage && inst.stage.resetView) inst.stage.resetView(); } catch (_) {}
            }
        };
        const onLeave = () => {
            down = false;
            setHovered(inst, null);
            try { if (inst.hud) inst.hud.hideTooltip(); } catch (_) {}
            host.style.cursor = '';
        };
        const onKey = ev => {
            const tag = ev.target && ev.target.tagName;
            if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
            if (ev.key === 'Escape') {
                if (inst.hud && inst.hud.isPanelOpen && inst.hud.isPanelOpen()) {
                    clearFocus(inst);
                    ev.stopPropagation();
                }
                return;
            }
            if (inst.disposed || inst.paused) return;
            // Only when this window plausibly owns the keyboard focus.
            const ae = document.activeElement;
            if (ae && ae !== document.body && !inst.root.contains(ae)) return;
            inst.lastActivity = Date.now();
            if (ev.key === 'ArrowRight') { cycleFocus(inst, 1); ev.preventDefault(); } else if (ev.key === 'ArrowLeft') { cycleFocus(inst, -1); ev.preventDefault(); } else if (ev.key === 'o' || ev.key === 'O') {
                clearFollow(inst);
                try { if (inst.stage && inst.stage.resetView) inst.stage.resetView(); } catch (_) {}
            } else if (ev.key === 'g' || ev.key === 'G') {
                hudAction(inst, 'graph');
            } else if (ev.key === 'e' || ev.key === 'E') {
                hudAction(inst, 'effects');
            }
        };

        host.addEventListener('pointermove', onMove);
        host.addEventListener('pointerdown', onDown);
        host.addEventListener('pointerup', onUp);
        host.addEventListener('pointerleave', onLeave);
        host.addEventListener('dblclick', onDbl);
        document.addEventListener('keydown', onKey, true);
        inst.disposers.push(() => {
            host.removeEventListener('pointermove', onMove);
            host.removeEventListener('pointerdown', onDown);
            host.removeEventListener('pointerup', onUp);
            host.removeEventListener('pointerleave', onLeave);
            host.removeEventListener('dblclick', onDbl);
            document.removeEventListener('keydown', onKey, true);
        });
    }

    // ── Focus flight + info panel ────────────────────────────────────────────

    // Shared world-radius estimate (bounding sphere × world scale), used by
    // the focus flight, the hover ring and the selection beacon. The scratch
    // vector is allocated once per window, never per call.
    const radiusScratch = { v: null };

    function objectRadius(inst, mesh) {
        let radius = 1;
        try {
            const geometry = mesh.geometry;
            if (geometry) {
                if (!geometry.boundingSphere && geometry.computeBoundingSphere) {
                    try { geometry.computeBoundingSphere(); } catch (_) {}
                }
                if (geometry.boundingSphere && isFinite(geometry.boundingSphere.radius)) {
                    if (!radiusScratch.v) radiusScratch.v = new inst.THREE.Vector3();
                    mesh.getWorldScale(radiusScratch.v);
                    radius = Math.max(0.5, geometry.boundingSphere.radius *
                        Math.max(radiusScratch.v.x, radiusScratch.v.y, radiusScratch.v.z));
                }
            }
        } catch (_) {}
        return radius;
    }

    function focusObject(inst, mesh) {
        if (!mesh || inst.disposed) return;
        const THREE = inst.THREE;
        try {
            const target = new THREE.Vector3();
            mesh.getWorldPosition(target);
            const radius = objectRadius(inst, mesh);
            // Approach from the current camera side: 2.5× object radius + 18 units out.
            const dist = radius * 2.5 + 18;
            const dir = new THREE.Vector3();
            if (inst.stage && inst.stage.camera) dir.copy(inst.stage.camera.position).sub(target);
            if (dir.lengthSq() < 1e-6) dir.set(0, 0.4, 1);
            dir.normalize();
            const camPos = target.clone().add(dir.multiplyScalar(dist));
            if (inst.stage && typeof inst.stage.flyTo === 'function') inst.stage.flyTo(camPos, target, FOCUS_DURATION, { trackMesh: mesh });
            // Halo locks onto the selection and follows it while it drifts.
            if (inst.fx && typeof inst.fx.selectBeacon === 'function') {
                inst.fx.selectBeacon(mesh, accentHexFor(mesh.userData || {}), radius);
            }
            // Persistent floating label (positioned per frame in startLoop).
            inst.focused = { mesh: mesh, ud: mesh.userData || {}, radius: radius };
            // Camera follow: while the selection drifts (satellites, drones,
            // comet riders), the camera tracks it once the focus flight ends.
            clearFollow(inst);
            inst.follow = { mesh: mesh, pending: true };
            // Attention link: the core acknowledges the selection twice.
            if (inst.fx && typeof inst.fx.beam === 'function') {
                const P = NS().PALETTE || {};
                const coreHex = P.core != null ? P.core : 0x59d4ff;
                try {
                    inst.fx.beam(corePosition(inst, new THREE.Vector3()), target, coreHex, { burst: false, duration: 0.9 });
                } catch (_) {}
                const second = setTimeout(() => {
                    if (inst.disposed) return;
                    try {
                        inst.fx.beam(corePosition(inst, new THREE.Vector3()), target, coreHex, { burst: false, duration: 0.7 });
                    } catch (_) {}
                }, 220);
                inst.timers.push(second);
            }
        } catch (_) {}
        showInfoFor(inst, mesh, mesh.userData || {});
    }

    // Escaped plain value, or the dash placeholder.
    function fmtVal(inst, v) {
        if (v == null || v === '') return '–';
        const s = String(v);
        // Go's zero time renders as 0001-01-01T00:00:00Z — treat as no value.
        if (/^0001-01-01T00:00:00(\.\d+)?Z?$/.test(s)) return '–';
        return inst.ctx.esc(s);
    }

    // Translate known state words via sysworld.state.*, otherwise escaped raw text.
    function stateLabel(inst, v) {
        const key = String(v == null ? '' : v).toLowerCase();
        if (STATE_KEYS[key]) return inst.L('sysworld.state.' + key);
        return fmtVal(inst, v);
    }

    function showInfoFor(inst, mesh, ud) {
        if (!inst.hud) return;
        const L = inst.L;
        const kind = ud.kind || null;
        const payload = ud.payload || {};
        const title = String(ud.label != null ? ud.label : (ud.id != null ? ud.id : ''));
        const rows = [];
        inst.panelKind = kind;
        inst.panelToken++;
        const token = inst.panelToken;
        const meta = { kind: kind || 'object', accent: hexCss(accentHexFor(ud)) };

        if (kind === 'integration') {
            const enabled = ud.enabled !== false;
            rows.push({ k: L('sysworld.panel.status'), v: L(enabled ? 'sysworld.panel.enabled' : 'sysworld.panel.disabled'), tone: enabled ? 'ok' : 'err' });
            rows.push({
                k: L('sysworld.panel.category'),
                v: CATEGORY_KEYS[ud.category] ? L('sysworld.cat.' + ud.category) : fmtVal(inst, ud.category)
            });
            rows.push({ k: 'ID', v: fmtVal(inst, ud.id) });
            rows.push({ k: L('sysworld.panel.zone'), v: L('sysworld.zone.integrations') });
        } else if (kind === 'kgnode') {
            rows.push({ section: L('sysworld.sec.status') });
            rows.push({ k: L('sysworld.panel.type'), v: fmtVal(inst, ud.type) });
            rows.push({ k: L('sysworld.panel.access_count'), v: fmtVal(inst, ud.accessCount) });
            // Protection state (enabled/disabled wording is the closest panel vocabulary).
            rows.push({ k: L('sysworld.panel.status'), v: L(ud.protected ? 'sysworld.panel.enabled' : 'sysworld.panel.disabled'), tone: ud.protected ? 'ok' : 'dim' });
            // Detail fetch: expand the node in the graph, then list top relations.
            inst.apiGet('/api/knowledge-graph/node?id=' + encodeURIComponent(ud.id)).then(detail => {
                if (inst.disposed || token !== inst.panelToken) return;
                if (detail) {
                    try { if (inst.graph.expand) inst.graph.expand(ud.id, detail); } catch (_) {}
                }
                const rel = detail ? pickNum(
                    detail.relation_count, detail.relations_count, detail.edge_count,
                    Array.isArray(detail.relations) ? detail.relations.length : undefined,
                    Array.isArray(detail.edges) ? detail.edges.length : undefined
                ) : undefined;
                rows.push({ k: L('sysworld.panel.relations'), v: rel != null ? String(rel) : '–' });
                // Top-5 neighbor list with relation type, resolved to labels.
                const relRows = [];
                if (detail && Array.isArray(detail.edges)) {
                    const labels = {};
                    if (Array.isArray(detail.neighbors)) {
                        detail.neighbors.forEach(n => {
                            if (n && n.id != null) labels[String(n.id)] = n.label || String(n.id);
                        });
                    }
                    const seen = {};
                    detail.edges.forEach(edge => {
                        if (!edge || relRows.length >= 5) return;
                        const src = String(edge.source != null ? edge.source : '');
                        const tgt = String(edge.target != null ? edge.target : '');
                        const other = src === String(ud.id) ? tgt : (tgt === String(ud.id) ? src : null);
                        if (other == null || seen[other]) return;
                        seen[other] = 1;
                        relRows.push({
                            relation: String(edge.relation || '–'),
                            label: labels[other] || other
                        });
                    });
                }
                if (relRows.length) {
                    rows.push({ section: L('sysworld.sec.relations') });
                    rows.push({ rel: relRows });
                }
                if (inst.panelKind === 'kgnode' && inst.hud.isPanelOpen && inst.hud.isPanelOpen()) {
                    try { inst.hud.showPanel(title, rows, meta); } catch (_) {}
                }
            });
        } else if (kind === 'mission') {
            rows.push({ section: L('sysworld.sec.status') });
            rows.push({ k: L('sysworld.panel.status'), v: stateLabel(inst, payload.status || payload.state), tone: toneForState(payload.status || payload.state) });
            const success = pickNum(payload.success_rate, payload.successRate, payload.rate);
            if (success != null) {
                const ratio = success > 1 ? success / 100 : success;
                rows.push({ k: 'OK', v: Math.round(ratio * 100) + '%', bar: Math.max(0, Math.min(1, ratio)) });
            }
            rows.push({ section: L('sysworld.sec.details') });
            rows.push({ k: L('sysworld.panel.schedule'), v: fmtVal(inst, payload.schedule || payload.cron) });
            const nextRel = relTime(inst, payload.next_run || payload.nextRun);
            rows.push({ k: L('sysworld.panel.next_run'), v: nextRel || fmtVal(inst, payload.next_run || payload.nextRun) });
            const lastRel = relTime(inst, payload.last_run || payload.lastRun);
            rows.push({ k: L('sysworld.panel.last_run'), v: lastRel || fmtVal(inst, payload.last_run || payload.lastRun) });
            rows.push({ k: L('sysworld.panel.access_count'), v: fmtVal(inst, pickNum(payload.run_count, payload.runs, payload.runCount)) });
        } else if (kind === 'coagent') {
            rows.push({ k: L('sysworld.panel.state'), v: stateLabel(inst, payload.state || payload.status), tone: toneForState(payload.state || payload.status) });
            rows.push({ k: L('sysworld.panel.type'), v: fmtVal(inst, payload.specialist || payload.type) });
            if (payload.model) rows.push({ k: L('sysworld.panel.model'), v: fmtVal(inst, payload.model) });
            rows.push({ k: L('sysworld.panel.tokens'), v: fmtVal(inst, pickNum(payload.tokens, payload.tokens_used, payload.token_count)) });
            rows.push({ k: L('sysworld.panel.access_count'), v: fmtVal(inst, pickNum(payload.tool_calls, payload.toolCalls)) });
        } else if (kind === 'container') {
            rows.push({ k: L('sysworld.panel.image'), v: fmtVal(inst, payload.image) });
            rows.push({ k: L('sysworld.panel.state'), v: stateLabel(inst, payload.state), tone: toneForState(payload.state) });
            rows.push({ k: L('sysworld.panel.status'), v: fmtVal(inst, payload.status) });
            const ports = Array.isArray(payload.ports) ? payload.ports.join(', ') : payload.ports;
            if (ports) rows.push({ k: L('sysworld.panel.ports'), v: fmtVal(inst, ports) });
        } else if (kind === 'daemon') {
            rows.push({ k: L('sysworld.panel.status'), v: fmtVal(inst, payload.status || payload.state), tone: toneForState(payload.status || payload.state) });
            rows.push({ k: L('sysworld.panel.restarts'), v: fmtVal(inst, pickNum(payload.restarts, payload.restart_count)) });
        } else if (kind === 'tool') {
            rows.push({ k: 'ID', v: fmtVal(inst, ud.id || ud.label) });
            const calls = pickNum(ud.count, payload.count, payload.calls);
            rows.push({ k: L('sysworld.panel.access_count'), v: fmtVal(inst, calls) });
            // Rank bar: this tool's calls relative to the busiest tool.
            const list = Array.isArray(inst.data.tools) ? inst.data.tools : [];
            let maxCalls = 0;
            let rank = 0;
            list.forEach(item => {
                const c = item ? pickNum(item.count, item.calls) : undefined;
                if (typeof c === 'number') {
                    if (c > maxCalls) maxCalls = c;
                    if ((item.name || item.tool) === (ud.id || ud.label)) rank = c;
                }
            });
            if (calls != null && maxCalls > 0) {
                rows.push({ k: L('sysworld.panel.rank'), v: Math.round((rank || calls) / maxCalls * 100) + '%', bar: (rank || calls) / maxCalls });
            }
        } else if (kind === 'cron') {
            rows.push({ k: L('sysworld.panel.schedule'), v: fmtVal(inst, payload.expr || payload.schedule || payload.expression) });
            const cronRel = relTime(inst, payload.next_run || payload.nextRun);
            rows.push({ k: L('sysworld.panel.next_run'), v: cronRel || fmtVal(inst, payload.next_run || payload.nextRun) });
        } else {
            rows.push({ k: 'ID', v: fmtVal(inst, ud.id) });
        }

        try { inst.hud.showPanel(title, rows, meta); } catch (_) {}
        try {
            if (inst.hud.showSelLabel) inst.hud.showSelLabel({ name: title, kind: meta.kind, accent: meta.accent });
        } catch (_) {}
    }

    // Camera-follow: while a focused object drifts (satellites, drones), the
    // controls target tracks its live world position and the camera is
    // translated by the same delta, so the user's orbit angle and zoom are
    // preserved. Rotation/zoom keep working around the moving target; pan is
    // disabled only while an actual moving object is being chased (it would
    // fight the tracking). Engages after the focus flight hands control back,
    // with the catch-up delta clamped so the first frames never jump.
    function updateFollowTarget(inst, dt) {
        const f = inst.follow;
        if (!f || !f.mesh) return;
        if (!f.mesh.parent || !inst.stage || !inst.stage.controls || !inst.stage.camera) {
            clearFollow(inst);
            return;
        }
        const controls = inst.stage.controls;
        if (f.pending) {
            if (controls.enabled) f.pending = false;
            return;
        }
        if (!inst._followVec) inst._followVec = new inst.THREE.Vector3();
        const target = inst._followVec;
        try { f.mesh.getWorldPosition(target); } catch (_) { return; }
        if (!inst._followDelta) inst._followDelta = new inst.THREE.Vector3();
        const delta = inst._followDelta;
        delta.copy(target).sub(controls.target);
        if (delta.lengthSq() < 1e-8) return;
        // Clamp the chase speed: satellites move far slower than this, so only
        // rare residuals (e.g. interrupted flights) glide softly into place.
        const maxStep = Math.max(0.05, 40 * (dt > 0 ? dt : 0.016));
        if (delta.lengthSq() > maxStep * maxStep) delta.setLength(maxStep);
        controls.target.add(delta);
        inst.stage.camera.position.add(delta);
        if (!f.moved) {
            f.moved = true;
            controls.enablePan = false;
        }
    }

    // Projects the focused object's world position to HUD pixels and pins
    // the selection label above it. Runs per frame; one scratch vector.
    function updateSelLabel(inst) {
        const hud = inst.hud;
        if (!hud || typeof hud.positionSelLabel !== 'function') return;
        const f = inst.focused;
        if (!f || !f.mesh) return;
        if (!f.mesh.parent) {
            // The tracked mesh was rebuilt (data refresh): drop the anchor.
            inst.focused = null;
            try { hud.hideSelLabel(); } catch (_) {}
            return;
        }
        if (!inst._selVec) inst._selVec = new inst.THREE.Vector3();
        const v = inst._selVec;
        try { f.mesh.getWorldPosition(v); } catch (_) { return; }
        v.y += (f.radius || 1) + 1.1;
        if (!inst.stage || !inst.stage.camera) return;
        v.project(inst.stage.camera);
        if (v.z > 1 || v.z < -1) {
            // Behind the camera: keep state, just slide the chip away.
            try { hud.positionSelLabel(-9999, -9999); } catch (_) {}
            return;
        }
        const w = inst.canvasHost.clientWidth || 1;
        const h = inst.canvasHost.clientHeight || 1;
        let x = (v.x * 0.5 + 0.5) * w;
        let y = (-v.y * 0.5 + 0.5) * h;
        // Clamp into view so the chip doubles as an edge indicator when the
        // selected object drifts off-screen instead of rendering at negative
        // coordinates.
        const marginX = Math.min(150, Math.max(40, w * 0.18));
        x = x < marginX ? marginX : (x > w - marginX ? w - marginX : x);
        y = y < 46 ? 46 : (y > h - 24 ? h - 24 : y);
        try { hud.positionSelLabel(x, y); } catch (_) {}
    }

    // ── RAF loop (the only one; stage.update renders, keep it last) ──────────

    // Soft ambient traffic between world zones so the universe always feels alive.
    function tickAmbientFx(inst, dt) {
        if (!inst.fx || inst.effectsEnabled === false) return;
        const THREE = inst.THREE;
        const L = (window.SysWorld && window.SysWorld.LAYOUT) || {};
        const P = (window.SysWorld && window.SysWorld.PALETTE) || {};
        inst._ambientCd = (inst._ambientCd || 1.2) - dt;
        if (inst._ambientCd > 0) return;
        inst._ambientCd = (1.4 + Math.random() * 2.8) * (inst._ambientRateMul || 1);

        const origin = new THREE.Vector3(0, 0, 0);
        const graph = L.graphCenter
            ? new THREE.Vector3(L.graphCenter.x, L.graphCenter.y, L.graphCenter.z)
            : new THREE.Vector3(0, 34, -120);
        const mission = new THREE.Vector3(
            Math.cos(inst.elapsed * 0.2) * (L.missionRingRadius || 84),
            2,
            Math.sin(inst.elapsed * 0.2) * (L.missionRingRadius || 84)
        );
        const orbit = new THREE.Vector3(
            Math.cos(inst.elapsed * 0.35) * (L.orbitOuter || 64),
            4 + Math.sin(inst.elapsed * 0.5) * 3,
            Math.sin(inst.elapsed * 0.35) * (L.orbitOuter || 64)
        );
        const infra = new THREE.Vector3(
            (Math.random() * 2 - 1) * 40,
            L.infraY != null ? L.infraY : -34,
            (Math.random() * 2 - 1) * 40
        );
        const points = [origin, graph, mission, orbit, infra];
        const a = points[Math.floor(Math.random() * points.length)];
        let b = points[Math.floor(Math.random() * points.length)];
        if (b === a) b = points[(points.indexOf(a) + 1) % points.length];
        const colors = [P.core || 0x59d4ff, P.mission || 0xffd54f, P.memory || 0x4dd0e1, P.agent || 0x80deea, P.tool || 0xfff176];
        const color = colors[Math.floor(Math.random() * colors.length)];
        const roll = Math.random();
        try {
            if (roll < 0.45 && inst.fx.comet) inst.fx.comet(a, b, color, { duration: 1.1 + Math.random() * 0.9, size: 2.4 });
            else if (roll < 0.8 && inst.fx.beam) inst.fx.beam(a, b, color, { duration: 0.55 + Math.random() * 0.5 });
            else if (inst.fx.sparkle) inst.fx.sparkle(a, color, 12 + Math.floor(Math.random() * 12));
        } catch (_) {}
    }

    function startLoop(inst) {
        let last = performance.now();
        const frame = now => {
            if (inst.disposed) return;
            inst.rafId = requestAnimationFrame(frame);
            if (inst.paused) {
                last = now;
                return;
            }
            let dt = (now - last) / 1000;
            last = now;
            if (!(dt > 0)) dt = 0;
            if (dt > MAX_DT) dt = MAX_DT;
            inst.elapsed += dt;
            const e = inst.elapsed;
            try { tickAmbientFx(inst, dt); } catch (_) {}
            try { if (inst.fx && inst.fx.update) inst.fx.update(dt, e); } catch (_) {}
            try { if (inst.core && inst.core.update) inst.core.update(dt, e); } catch (_) {}
            try { if (inst.orbit && inst.orbit.update) inst.orbit.update(dt, e); } catch (_) {}
            try { if (inst.graph && inst.graph.update) inst.graph.update(dt, e); } catch (_) {}
            try { if (inst.fleet && inst.fleet.update) inst.fleet.update(dt, e); } catch (_) {}
            try { updateFollowTarget(inst, dt); } catch (_) {}
            try { updateSelLabel(inst); } catch (_) {}
            // Cinematic idle drift: after 45s without interaction and with no
            // active selection, the camera slowly orbits on its own.
            try {
                const controls = inst.stage && inst.stage.controls;
                if (controls) {
                    const wantSpin = !inst.paused && !inst.focused && (Date.now() - inst.lastActivity) > 45000;
                    if (controls.autoRotate !== wantSpin) controls.autoRotate = wantSpin;
                    if (wantSpin && controls.autoRotateSpeed !== 0.35) controls.autoRotateSpeed = 0.35;
                }
            } catch (_) {}
            try { if (inst.stage && inst.stage.update) inst.stage.update(dt, e); } catch (_) {}
        };
        inst.rafId = requestAnimationFrame(frame);
    }

    // Pause on hidden tab or when the window scrolls out of view.
    function watchVisibility(inst) {
        const apply = () => {
            const paused = inst.docHidden || !inst.inView;
            if (paused === inst.paused) return;
            inst.paused = paused;
            try { if (inst.stage && inst.stage.setPaused) inst.stage.setPaused(paused); } catch (_) {}
        };
        const onVisibility = () => {
            inst.docHidden = !!document.hidden;
            apply();
        };
        document.addEventListener('visibilitychange', onVisibility);
        inst.disposers.push(() => document.removeEventListener('visibilitychange', onVisibility));
        if (typeof IntersectionObserver !== 'undefined') {
            const io = new IntersectionObserver(entries => {
                inst.inView = !!(entries && entries[0] && entries[0].isIntersecting);
                apply();
            }, { threshold: 0.01 });
            try { io.observe(inst.root); } catch (_) {}
            inst.intersectionObserver = io;
        }
    }

    // ── Dispose ──────────────────────────────────────────────────────────────

    function dispose(windowId) {
        const inst = instances.get(windowId);
        if (!inst) return;
        inst.disposed = true;

        if (inst.fallback) {
            try { if (inst.root) inst.root.remove(); } catch (_) {}
            instances.delete(windowId);
            return;
        }

        try {
            (inst.timers || []).forEach(id => {
                try { clearInterval(id); } catch (_) {}
                try { clearTimeout(id); } catch (_) {}
            });
            inst.timers = [];
        } catch (_) {}
        try {
            Object.keys(inst.debouncers || {}).forEach(key => {
                try { clearTimeout(inst.debouncers[key]); } catch (_) {}
            });
            inst.debouncers = {};
        } catch (_) {}
        try {
            if (inst.aborts) inst.aborts.forEach(ctl => { try { ctl.abort(); } catch (_) {} });
            if (inst.aborts) inst.aborts.clear();
        } catch (_) {}
        try {
            const sse = window.AuraSSE;
            if (sse && typeof sse.off === 'function') {
                Object.keys(inst.sseHandlers || {}).forEach(type => {
                    try { sse.off(type, inst.sseHandlers[type]); } catch (_) {}
                });
            }
            inst.sseHandlers = {};
        } catch (_) {}
        try { if (inst.rafId) cancelAnimationFrame(inst.rafId); } catch (_) {}
        try { if (inst.intersectionObserver) inst.intersectionObserver.disconnect(); } catch (_) {}
        try {
            (inst.disposers || []).forEach(fn => { try { fn(); } catch (_) {} });
            inst.disposers = [];
        } catch (_) {}
        // Module teardown order: hud → fleet → graph → orbit → core → stage → fx.
        ['hud', 'fleet', 'graph', 'orbit', 'core', 'stage', 'fx'].forEach(key => {
            const mod = inst[key];
            if (mod && typeof mod.dispose === 'function') {
                try { mod.dispose(); } catch (_) {}
            }
            inst[key] = null;
        });
        try { if (inst.root) inst.root.remove(); } catch (_) {}
        instances.delete(windowId);
    }

    window.SysWorldApp = { render, dispose };
})();
