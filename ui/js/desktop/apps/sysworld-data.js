(function () {
    'use strict';
    const NS = window.SysWorld = window.SysWorld || {};
    const paths = {
        overview: ['/api/dashboard/overview', 10000],
        system: ['/api/dashboard/system', 30000],
        memory: ['/api/dashboard/memory', 30000],
        activity: ['/api/dashboard/activity', 15000],
        missions: ['/api/missions/v2', 15000],
        tools: ['/api/dashboard/tool-stats', 60000],
        containers: ['/api/containers', 30000],
        daemons: ['/api/daemons', 30000],
        nodes: ['/api/knowledge-graph/nodes?limit=300', 90000],
        edges: ['/api/knowledge-graph/edges?limit=500', 90000],
        budget: ['/api/budget', 30000],
        operations: ['/api/operational-issues?status=open&limit=100', 30000],
    };
    const subscribers = new Set(), sources = {}, inFlight = new Map(), events = [];
    let timer = 0, generation = 0, handlers = [], sequence = 0;
    const number = (...values) => values.find(v => typeof v === 'number' && Number.isFinite(v));
    const list = (value, key) => Array.isArray(value) ? value : Array.isArray(value?.[key]) ? value[key] : Array.isArray(value?.items) ? value.items : [];
    function normalizeSystemMetrics(p) {
        if (!p || typeof p !== 'object') return {};
        return {
            cpu: number(p.cpu?.usage_percent, p.cpu_percent, p.cpu),
            ram: number(p.memory?.used_percent, p.memory_percent, p.memory),
            disk: number(p.disk?.used_percent, p.disk_percent),
            uptime: number(p.uptime_seconds, p.uptime),
        };
    }
    function notify() {
        for (const callback of subscribers) callback({ sources, events });
    }
    async function refresh(key, force = false) {
        if (!subscribers.size || inFlight.has(key)) return;
        const [path, interval] = paths[key], source = sources[key];
        if (!force && source && Date.now() - source.attempt < interval) return;
        const current = generation, sourceRevision = source?.revision || 0, controller = new AbortController();
        inFlight.set(key, controller);
        sources[key] = { ...source, attempt: Date.now() };
        const timeout = setTimeout(() => controller.abort(), 10000);
        try {
            const response = await fetch(path, { credentials: 'same-origin', signal: controller.signal });
            if (!response.ok) throw Error('Unavailable');
            const data = await response.json();
            if (!data || typeof data !== 'object' || data.status === 'error') throw Error('Unavailable');
            if (current !== generation || (key === 'system' && (sources.system?.revision || 0) !== sourceRevision)) return;
            sources[key] = { data, at: Date.now(), attempt: Date.now(), failed: false, revision: sourceRevision + 1 };
        } catch (_) {
            if (current === generation && (sources[key]?.revision || 0) === sourceRevision) sources[key] = { ...sources[key], failed: true };
        } finally {
            clearTimeout(timeout);
            if (inFlight.get(key) === controller) inFlight.delete(key);
            if (current === generation) notify();
        }
    }
    function addEvent(kind, label, district) {
        // Only event metadata: never copy tool arguments, chat text or warning payloads.
        events.unshift({ id: ++sequence, kind, label: String(label || '').slice(0, 120), district, at: Date.now() });
        events.length = Math.min(events.length, 60); notify();
    }
    function start() {
        const reg = (type, fn) => {
            window.AuraSSE?.on(type, fn); handlers.push([type, fn]);
        };
        reg('system_metrics', payload => {
            if (!payload || typeof payload !== 'object') return;
            sources.system = { data: payload, at: Date.now(), attempt: Date.now(), failed: false, revision: (sources.system?.revision || 0) + 1 }; notify();
        });
        reg('agent_status', payload => {
            const busy = typeof payload?.busy === 'boolean' ? payload.busy : typeof payload?.is_busy === 'boolean' ? payload.is_busy : undefined;
            if (busy !== undefined && sources.overview?.data) {
                sources.overview.data.agent = { ...sources.overview.data.agent, busy }; notify();
            }
            void refresh('overview');
        });
        reg('mission_update', () => { addEvent('mission', '', 'missions'); void refresh('missions'); });
        reg('coagent_progress', () => { void refresh('activity'); });
        reg('memory_update', () => { addEvent('memory', '', 'memory'); void refresh('memory'); });
        reg('budget_update', () => { void refresh('budget'); });
        reg('system_warning', () => { addEvent('warning', '', 'operations'); void refresh('operations'); });
        reg('tool_call_preview', p => {
            const name = typeof p?.tool_name === 'string' ? p.tool_name : typeof p?.tool === 'string' ? p.tool : typeof p?.name === 'string' ? p.name : '';
            addEvent('tool', name, 'agent');
        });
        Object.keys(paths).forEach(k => void refresh(k, true));
        timer = setInterval(() => {
            if (!document.hidden) Object.keys(paths).forEach(k => void refresh(k));
            notify();
        }, 5000);
    }
    function subscribe(callback) {
        subscribers.add(callback);
        if (subscribers.size === 1) start(); else callback({ sources, events });
        return () => {
            subscribers.delete(callback);
            if (subscribers.size) return;
            generation++; clearInterval(timer); timer = 0;
            inFlight.forEach(c => c.abort()); inFlight.clear();
            handlers.forEach(([type, fn]) => window.AuraSSE?.off(type, fn)); handlers = [];
            Object.keys(sources).forEach(k => delete sources[k]); events.length = 0;
        };
    }
    function entities(snapshot, L) {
        const out = [], source = snapshot.sources;
        const get = key => source[key]?.data;
        const age = key => !source[key]?.at || source[key].failed || Date.now() - source[key].at > Math.max(45000, paths[key][1] * 2.5);
        const make = (id, district, kind, label, key, state, rows = [], payload = {}) => {
            const value = { id, district, kind, label: String(label || id), source: paths[key][0], at: source[key]?.at,
                stale: age(key), state: state || 'unknown', rows, payload };
            out.push(value); return value;
        };
        const row = (key, value, format) => ({ key, value, format });
        const overview = get('overview'), agent = overview?.agent, system = get('system'), metrics = normalizeSystemMetrics(system);
        make('agent', 'agent', 'core', L('sysworld.zone.core'), 'overview', typeof agent?.busy === 'boolean' ? (agent.busy ? 'running' : 'idle') : 'unknown', [
            row('sysworld.panel.model', agent?.model), row('sysworld.city.provider', agent?.provider),
            row('sysworld.city.personality', agent?.personality), row('sysworld.city.context', agent?.context_window, 'number'),
            row('sysworld.stats.budget', number(get('budget')?.spent, get('budget')?.spent_usd, get('budget')?.total_spent), 'money'),
        ], agent);
        make('infra', 'infra', 'infrastructure', L('sysworld.zone.infra'), 'system', system ? 'idle' : 'unknown', [
            row('sysworld.stats.cpu', metrics.cpu, 'percent'), row('sysworld.stats.ram', metrics.ram, 'percent'),
            row('sysworld.city.disk', metrics.disk, 'percent'), row('sysworld.stats.uptime', metrics.uptime, 'uptime'),
            row('sysworld.panel.model', system?.cpu?.model_name), row('sysworld.city.cores', system?.cpu?.cores, 'number'),
            row('sysworld.city.memory_used', system?.memory?.used, 'bytes'), row('sysworld.city.memory_total', system?.memory?.total, 'bytes'),
            row('sysworld.city.disk_free', system?.disk?.free, 'bytes'),
        ]);
        const integrations = overview?.integrations;
        make('integrations', 'integrations', 'integration', L('sysworld.zone.integrations'), 'overview', integrations ? 'idle' : 'unknown', [
            row('sysworld.panel.enabled', integrations ? Object.values(integrations).filter(v => v === true || v?.enabled === true).length : undefined, 'number'),
            row('sysworld.panel.status', L('sysworld.city.configured_hint')),
        ]);
        for (const [id, value] of Object.entries(integrations || {}).sort(([a],[b]) => a.localeCompare(b))) {
            const enabled = typeof value === 'boolean' ? value : value?.enabled;
            make('integration:' + id, 'integrations', 'integration', id.replaceAll('_',' '), 'overview',
                enabled === true ? 'configured' : enabled === false ? 'disabled' : 'unknown',
                [row('sysworld.panel.id', id), row('sysworld.panel.status', L('sysworld.city.configured_hint'))]);
        }
        const memory = get('memory');
        make('memory', 'memory', 'memory', L('sysworld.zone.memory'), 'memory', memory ? 'idle' : 'unknown', [
            row('sysworld.city.vectors', memory?.vectordb_entries, 'number'), row('sysworld.city.core_facts', memory?.core_memory_facts, 'number'),
            row('sysworld.city.journal', memory?.journal_entries, 'number'), row('sysworld.city.notes', memory?.notes_count, 'number'),
            row('sysworld.city.chat_messages', memory?.chat_messages, 'number'),
        ]);
        const nodes = list(get('nodes'), 'nodes'), edges = list(get('edges'), 'edges');
        make('graph', 'graph', 'kgnode', L('sysworld.zone.graph'), 'nodes', get('nodes') ? 'idle' : 'unknown', [
            row('sysworld.city.loaded_nodes', get('nodes') ? nodes.length : undefined, 'number'), row('sysworld.panel.relations', get('edges') ? edges.length : undefined, 'number'),
            row('sysworld.city.coverage', L('sysworld.city.graph_limit')),
        ]);
        for (const node of nodes) {
            if (node.id == null) continue;
            const related = edges.filter(e => String(e.source) === String(node.id) || String(e.target) === String(node.id));
            make('node:' + node.id, 'graph', 'kgnode', node.label || node.id, 'nodes', 'idle', [
                row('sysworld.panel.id', node.id), row('sysworld.panel.type', node.type),
                row('sysworld.panel.access_count', node.access_count, 'number'),
                row('sysworld.panel.relations', related.length, 'number'),
            ], { id: node.id, related });
        }
        const missions = list(get('missions'), 'missions'), activity = get('activity');
        make('missions', 'missions', 'mission', L('sysworld.zone.missions'), 'missions', overview?.missions?.running ? 'running' : get('missions') ? 'idle' : 'unknown', [
            row('sysworld.stats.missions', overview?.missions?.total ?? (get('missions') ? missions.length : undefined), 'number'),
            row('sysworld.state.running', overview?.missions?.running, 'number'), row('sysworld.state.queued', overview?.missions?.queued, 'number'),
        ]);
        const records = [
            ['mission', 'missions', 'missions', missions], ['coagent', 'missions', 'activity', list(activity, 'coagents')],
            ['cron', 'missions', 'activity', list(activity, 'cron_jobs')], ['container', 'infra', 'containers', list(get('containers'), 'containers')],
            ['daemon', 'infra', 'daemons', list(get('daemons'), 'daemons')], ['tool', 'agent', 'tools', list(get('tools'), 'top_tools')],
        ];
        for (const [kind, district, key, values] of records) for (const p of values) {
            const id = p.id ?? p.name ?? p.tool; if (id == null) continue;
            const name = p.name || p.title || (Array.isArray(p.names) ? p.names[0]?.replace(/^\//,'') : null) || id;
            const state = p.state || p.status || (p.enabled === false || p.disabled === true ? 'disabled' : 'unknown');
            const rate = number(p.success_rate, p.successRate);
            make(kind + ':' + id, district, kind, name, key, state, [
                row('sysworld.panel.id', id), row('sysworld.panel.model', p.model),
                row('sysworld.panel.image', p.image), row('sysworld.panel.schedule', p.schedule || p.cron || p.expr || p.cron_expr),
                row('sysworld.panel.next_run', p.next_run, 'date'), row('sysworld.panel.last_run', p.last_run, 'date'),
                row('sysworld.panel.success_rate', rate == null ? undefined : rate <= 1 ? rate*100 : rate, 'percent'),
                row('sysworld.panel.access_count', number(p.run_count,p.count,p.calls,p.tool_calls), 'number'),
                row('sysworld.panel.tokens', number(p.tokens,p.tokens_used), 'number'), row('sysworld.panel.restarts', p.restarts, 'number'),
            ].filter(r => r.value !== undefined), { id });
        }
        const issues = list(get('operations'), 'issues');
        make('operations', 'operations', 'operations', L('sysworld.city.operations'), 'operations',
            !get('operations') ? 'unknown' : issues.some(i => i.severity === 'error' || i.severity === 'high' || i.severity === 'critical') ? 'error' : 'idle',
            [row('sysworld.city.issues', get('operations') ? number(get('operations').total, issues.length) : undefined, 'number')]);
        for (const issue of issues) if (issue.id) make('issue:' + issue.id, 'operations', 'operations', issue.title || issue.summary || issue.category || issue.id,
            'operations', issue.severity === 'error' || issue.severity === 'critical' ? 'error' : 'waiting',
            [row('sysworld.panel.id',issue.id), row('sysworld.city.severity',issue.severity), row('sysworld.panel.access_count',issue.occurrences,'number')]);
        return out;
    }
    NS.data = { subscribe, refresh: () => Object.keys(paths).forEach(k => void refresh(k, true)), entities, normalizeSystemMetrics };
})();
