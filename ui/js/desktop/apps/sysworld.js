(function () {
    'use strict';

    // system-world entry: app lifecycle, data polling, SSE wiring, pointer
    // interaction and the single RAF loop. The sibling sysworld-*.js modules
    // attach their factories to window.SysWorld and load before this file
    // (see ui/js/desktop/core/module-loader.js, app id 'system-world').

    const instances = new Map();

    const QUALITY_STORAGE_KEY = 'aurago.desktop.sysworld.quality';
    const QUALITY_SCALES = { low: 0.4, medium: 0.7, high: 1.0 };
    const MAX_DT = 0.05;
    const HOVER_THROTTLE_MS = 40;
    const CLICK_SLOP_PX = 5;
    const API_TIMEOUT_MS = 10000;
    const LOADING_FAILSAFE_MS = 15000;
    const HOVER_SCALE = 1.18;
    const FOCUS_DURATION = 1.2;

    const CATEGORY_KEYS = { communication: 1, smarthome: 1, infrastructure: 1, ai: 1, storage: 1, monitoring: 1, other: 1 };
    const STATE_KEYS = { running: 1, queued: 1, idle: 1, error: 1, done: 1, waiting: 1, exited: 1, paused: 1 };

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
            focusObject: null
        };
        instances.set(windowId, inst);
        inst.apiGet = path => apiGet(inst, path);
        inst.focusObject = mesh => focusObject(inst, mesh);
        // HUD callback hooks (behaviour itself lives in the HUD module).
        inst.hudHandlers = {
            onPanelClose: () => {
                inst.panelKind = null;
                inst.panelToken++;
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
            renderFallback(root, t);
            instances.set(windowId, { fallback: true, root, disposed: false });
            return;
        }
        // Keep the loading overlay on top of the HUD until first data arrives.
        root.appendChild(loading);
        try { inst.stage.introFlight(); } catch (_) {}

        wireInteraction(inst);
        wireSse(inst);
        startPolling(inst);
        startLoop(inst);
        watchVisibility(inst);
    }

    function renderFallback(root, t) {
        root.innerHTML = '';
        const fallback = document.createElement('div');
        fallback.className = 'sw-fallback';
        fallback.textContent = t('sysworld.no_webgl');
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

    function startPolling(inst) {
        poll(inst, 5000, () => fetchOverview(inst));
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
            try { if (inst.core.setMetrics) inst.core.setMetrics(payload); } catch (_) {}
            const patch = {};
            if (typeof payload.cpu === 'number') patch.cpu = payload.cpu;
            if (typeof payload.memory === 'number') patch.ram = payload.memory;
            if (typeof payload.uptime_seconds === 'number') patch.uptime = payload.uptime_seconds;
            if (Object.keys(patch).length) {
                try { inst.hud.setStats(patch); } catch (_) {}
            }
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
            inst.hovered = { mesh, scale: mesh.scale.clone() };
            try { mesh.scale.multiplyScalar(HOVER_SCALE); } catch (_) {}
        }
    }

    function tooltipHtml(inst, ud) {
        const esc = inst.ctx.esc;
        const label = esc(String(ud.label != null ? ud.label : (ud.id != null ? ud.id : '')));
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
            ? '<strong>' + label + '</strong><span>' + sub + '</span>'
            : '<strong>' + label + '</strong>';
    }

    function handleHover(inst, clientX, clientY) {
        if (inst.disposed || inst.paused) return;
        const mesh = pick(inst, clientX, clientY);
        setHovered(inst, mesh);
        if (!inst.hud) return;
        if (mesh && mesh.userData && mesh.userData.kind) {
            try { inst.hud.showTooltip(tooltipHtml(inst, mesh.userData), clientX, clientY); } catch (_) {}
        } else {
            try { inst.hud.hideTooltip(); } catch (_) {}
        }
        inst.canvasHost.style.cursor = mesh ? 'pointer' : '';
    }

    function clearFocus(inst) {
        inst.panelKind = null;
        inst.panelToken++;
        try { if (inst.hud) inst.hud.hidePanel(); } catch (_) {}
    }

    function wireInteraction(inst) {
        const host = inst.canvasHost;
        let lastHover = 0;
        let down = false;
        let downX = 0;
        let downY = 0;

        const onMove = ev => {
            const now = Date.now();
            if (now - lastHover < HOVER_THROTTLE_MS) return;
            lastHover = now;
            handleHover(inst, ev.clientX, ev.clientY);
        };
        const onDown = ev => {
            down = true;
            downX = ev.clientX;
            downY = ev.clientY;
        };
        // Click = pointerup near pointerdown (<5px travel); anything else was a drag.
        const onUp = ev => {
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
        const onLeave = () => {
            down = false;
            setHovered(inst, null);
            try { if (inst.hud) inst.hud.hideTooltip(); } catch (_) {}
            host.style.cursor = '';
        };
        const onKey = ev => {
            if (ev.key !== 'Escape') return;
            if (inst.hud && inst.hud.isPanelOpen && inst.hud.isPanelOpen()) {
                clearFocus(inst);
                ev.stopPropagation();
            }
        };

        host.addEventListener('pointermove', onMove);
        host.addEventListener('pointerdown', onDown);
        host.addEventListener('pointerup', onUp);
        host.addEventListener('pointerleave', onLeave);
        document.addEventListener('keydown', onKey, true);
        inst.disposers.push(() => {
            host.removeEventListener('pointermove', onMove);
            host.removeEventListener('pointerdown', onDown);
            host.removeEventListener('pointerup', onUp);
            host.removeEventListener('pointerleave', onLeave);
            document.removeEventListener('keydown', onKey, true);
        });
    }

    // ── Focus flight + info panel ────────────────────────────────────────────

    function focusObject(inst, mesh) {
        if (!mesh || inst.disposed) return;
        const THREE = inst.THREE;
        try {
            const target = new THREE.Vector3();
            mesh.getWorldPosition(target);
            let radius = 1;
            const geometry = mesh.geometry;
            if (geometry) {
                if (!geometry.boundingSphere && geometry.computeBoundingSphere) {
                    try { geometry.computeBoundingSphere(); } catch (_) {}
                }
                if (geometry.boundingSphere && isFinite(geometry.boundingSphere.radius)) {
                    const scale = new THREE.Vector3();
                    mesh.getWorldScale(scale);
                    radius = Math.max(0.5, geometry.boundingSphere.radius * Math.max(scale.x, scale.y, scale.z));
                }
            }
            // Approach from the current camera side: 2.5× object radius + 18 units out.
            const dist = radius * 2.5 + 18;
            const dir = new THREE.Vector3();
            if (inst.stage && inst.stage.camera) dir.copy(inst.stage.camera.position).sub(target);
            if (dir.lengthSq() < 1e-6) dir.set(0, 0.4, 1);
            dir.normalize();
            const camPos = target.clone().add(dir.multiplyScalar(dist));
            if (inst.stage && typeof inst.stage.flyTo === 'function') inst.stage.flyTo(camPos, target, FOCUS_DURATION);
        } catch (_) {}
        showInfoFor(inst, mesh, mesh.userData || {});
    }

    // Escaped plain value, or the dash placeholder.
    function fmtVal(inst, v) {
        if (v == null || v === '') return '–';
        return inst.ctx.esc(String(v));
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

        if (kind === 'integration') {
            rows.push({ k: L('sysworld.panel.status'), v: L(ud.enabled !== false ? 'sysworld.panel.enabled' : 'sysworld.panel.disabled') });
            rows.push({
                k: L('sysworld.panel.category'),
                v: CATEGORY_KEYS[ud.category] ? L('sysworld.cat.' + ud.category) : fmtVal(inst, ud.category)
            });
            rows.push({ k: 'ID', v: fmtVal(inst, ud.id) });
        } else if (kind === 'kgnode') {
            rows.push({ k: L('sysworld.panel.type'), v: fmtVal(inst, ud.type) });
            rows.push({ k: L('sysworld.panel.access_count'), v: fmtVal(inst, ud.accessCount) });
            // Protection state (enabled/disabled wording is the closest panel vocabulary).
            rows.push({ k: L('sysworld.panel.status'), v: L(ud.protected ? 'sysworld.panel.enabled' : 'sysworld.panel.disabled') });
            // Detail fetch: expand the node in the graph, then report relation count.
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
                if (inst.panelKind === 'kgnode' && inst.hud.isPanelOpen && inst.hud.isPanelOpen()) {
                    try { inst.hud.showPanel(title, rows); } catch (_) {}
                }
            });
        } else if (kind === 'mission') {
            rows.push({ k: L('sysworld.panel.status'), v: stateLabel(inst, payload.status || payload.state) });
            rows.push({ k: L('sysworld.panel.schedule'), v: fmtVal(inst, payload.schedule || payload.cron) });
            rows.push({ k: L('sysworld.panel.last_run'), v: fmtVal(inst, payload.last_run || payload.lastRun) });
            rows.push({ k: L('sysworld.panel.access_count'), v: fmtVal(inst, pickNum(payload.run_count, payload.runs, payload.runCount)) });
        } else if (kind === 'coagent') {
            rows.push({ k: L('sysworld.panel.state'), v: stateLabel(inst, payload.state || payload.status) });
            rows.push({ k: L('sysworld.panel.type'), v: fmtVal(inst, payload.specialist || payload.type) });
            rows.push({ k: L('sysworld.panel.tokens'), v: fmtVal(inst, pickNum(payload.tokens, payload.tokens_used, payload.token_count)) });
            rows.push({ k: L('sysworld.panel.access_count'), v: fmtVal(inst, pickNum(payload.tool_calls, payload.toolCalls)) });
        } else if (kind === 'container') {
            rows.push({ k: L('sysworld.panel.image'), v: fmtVal(inst, payload.image) });
            rows.push({ k: L('sysworld.panel.state'), v: stateLabel(inst, payload.state) });
            rows.push({ k: L('sysworld.panel.status'), v: fmtVal(inst, payload.status) });
        } else if (kind === 'daemon') {
            rows.push({ k: L('sysworld.panel.status'), v: fmtVal(inst, payload.status || payload.state) });
            rows.push({ k: L('sysworld.panel.restarts'), v: fmtVal(inst, pickNum(payload.restarts, payload.restart_count)) });
        } else if (kind === 'tool') {
            rows.push({ k: 'ID', v: fmtVal(inst, ud.id || ud.label) });
            rows.push({ k: L('sysworld.panel.access_count'), v: fmtVal(inst, pickNum(ud.count, payload.count, payload.calls)) });
        } else if (kind === 'cron') {
            rows.push({ k: L('sysworld.panel.schedule'), v: fmtVal(inst, payload.expr || payload.schedule || payload.expression) });
            rows.push({ k: L('sysworld.panel.next_run'), v: fmtVal(inst, payload.next_run || payload.nextRun) });
        } else {
            rows.push({ k: 'ID', v: fmtVal(inst, ud.id) });
        }

        try { inst.hud.showPanel(title, rows); } catch (_) {}
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
        inst._ambientCd = 1.4 + Math.random() * 2.8;

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
