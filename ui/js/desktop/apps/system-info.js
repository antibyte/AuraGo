(function () {
    'use strict';

    const instances = new Map();
    const maxHistory = 60;

    function escapeHtml(value) {
        return String(value == null ? '' : value).replace(/[&<>'"]/g, ch => ({
            '&': '&amp;',
            '<': '&lt;',
            '>': '&gt;',
            "'": '&#39;',
            '"': '&quot;'
        }[ch]));
    }

    function t(context, key, vars) {
        return context && typeof context.t === 'function' ? context.t(key, vars) : key;
    }

    function cssVar(name, fallback) {
        const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
        return value || fallback;
    }

    function formatBytes(value) {
        const n = Number(value || 0);
        if (!Number.isFinite(n) || n <= 0) return '0 B';
        const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
        let size = n;
        let unit = 0;
        while (size >= 1024 && unit < units.length - 1) {
            size /= 1024;
            unit += 1;
        }
        return `${size.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`;
    }

    function formatUptime(seconds) {
        let total = Math.max(0, Math.floor(Number(seconds || 0)));
        const days = Math.floor(total / 86400);
        total -= days * 86400;
        const hours = Math.floor(total / 3600);
        total -= hours * 3600;
        const minutes = Math.floor(total / 60);
        if (days > 0) return `${days}d ${hours}h ${minutes}m`;
        if (hours > 0) return `${hours}h ${minutes}m`;
        return `${minutes}m`;
    }

    function pct(value) {
        const n = Number(value || 0);
        if (!Number.isFinite(n)) return 0;
        return Math.max(0, Math.min(100, n));
    }

    function createGauge(canvas, color) {
        if (!window.Chart || !canvas) return null;
        return new Chart(canvas, {
            type: 'doughnut',
            data: {
                labels: ['Used', 'Free'],
                datasets: [{
                    data: [0, 100],
                    backgroundColor: [color, 'rgba(255,255,255,0.08)'],
                    borderWidth: 0,
                    hoverOffset: 0
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                cutout: '76%',
                rotation: -90,
                circumference: 180,
                animation: { duration: 260 },
                plugins: {
                    legend: { display: false },
                    tooltip: { enabled: false }
                }
            }
        });
    }

    function createHistoryChart(canvas) {
        if (!window.Chart || !canvas) return null;
        const accent = cssVar('--vd-accent', '#27c7a6');
        const coral = cssVar('--vd-coral', '#ff8066');
        const amber = cssVar('--vd-amber', '#f2b84b');
        const muted = cssVar('--vd-muted', '#a7b0c0');
        return new Chart(canvas, {
            type: 'line',
            data: {
                labels: [],
                datasets: [
                    { label: 'CPU', data: [], borderColor: accent, backgroundColor: 'rgba(39,199,166,0.12)', borderWidth: 2, pointRadius: 0, tension: 0.34, fill: true },
                    { label: 'Memory', data: [], borderColor: coral, backgroundColor: 'rgba(255,128,102,0.08)', borderWidth: 2, pointRadius: 0, tension: 0.34 },
                    { label: 'Disk', data: [], borderColor: amber, backgroundColor: 'rgba(242,184,75,0.08)', borderWidth: 2, pointRadius: 0, tension: 0.34 }
                ]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                animation: { duration: 220 },
                interaction: { mode: 'index', intersect: false },
                scales: {
                    x: { grid: { color: 'rgba(255,255,255,0.06)' }, ticks: { color: muted, maxTicksLimit: 6 } },
                    y: { min: 0, max: 100, grid: { color: 'rgba(255,255,255,0.07)' }, ticks: { color: muted, callback: value => `${value}%` } }
                },
                plugins: {
                    legend: { labels: { color: muted, boxWidth: 10, usePointStyle: true } },
                    tooltip: { callbacks: { label: ctx => `${ctx.dataset.label}: ${Number(ctx.parsed.y || 0).toFixed(1)}%` } }
                }
            }
        });
    }

    function render(host, windowId, context) {
        if (!host) return;
        dispose(windowId);

        const title = t(context, 'desktop.system_info_title');
        host.innerHTML = `
            <div class="vd-sysinfo">
                <section class="vd-sysinfo-hero">
                    <div>
                        <span class="vd-sysinfo-kicker">${escapeHtml(t(context, 'desktop.system_info_live'))}</span>
                        <h2>${escapeHtml(title)}</h2>
                    </div>
                    <span class="vd-sysinfo-status" data-role="status">${escapeHtml(t(context, 'desktop.loading'))}</span>
                </section>
                <section class="vd-sysinfo-gauges">
                    ${gaugeMarkup('cpu', t(context, 'desktop.system_info_cpu'))}
                    ${gaugeMarkup('memory', t(context, 'desktop.system_info_memory'))}
                    ${gaugeMarkup('disk', t(context, 'desktop.system_info_disk'))}
                </section>
                <section class="vd-sysinfo-history">
                    <div class="vd-sysinfo-section-title">${escapeHtml(t(context, 'desktop.system_info_history'))}</div>
                    <div class="vd-sysinfo-chart-wrap"><canvas data-role="history"></canvas></div>
                </section>
                <section class="vd-sysinfo-details" data-role="details"></section>
            </div>`;

        const accent = cssVar('--vd-accent', '#27c7a6');
        const coral = cssVar('--vd-coral', '#ff8066');
        const amber = cssVar('--vd-amber', '#f2b84b');
        const instance = {
            host,
            context,
            history: [],
            lastMetrics: null,
            uptimeBase: null,
            uptimeAt: null,
            gauges: {
                cpu: createGauge(host.querySelector('[data-gauge="cpu"]'), accent),
                memory: createGauge(host.querySelector('[data-gauge="memory"]'), coral),
                disk: createGauge(host.querySelector('[data-gauge="disk"]'), amber)
            },
            historyChart: createHistoryChart(host.querySelector('[data-role="history"]')),
            handler: null,
            tickTimer: null
        };

        instance.handler = metrics => updateFromMetrics(instance, metrics);
        instances.set(windowId, instance);

        fetchMetrics(instance);

        if (window.AuraSSE && typeof window.AuraSSE.on === 'function') {
            window.AuraSSE.on('system_metrics', instance.handler);
        }

        instance.tickTimer = setInterval(() => tickUptime(instance), 1000);
    }

    function fetchMetrics(instance) {
        fetch('/api/dashboard/system', { credentials: 'same-origin', cache: 'no-store' })
            .then(r => r && r.ok ? r.json() : null)
            .then(data => {
                if (data) updateFromMetrics(instance, data);
            })
            .catch(() => {});
    }

    function tickUptime(instance) {
        if (!instance.uptimeBase || !instance.host.isConnected) return;
        const elapsed = (Date.now() - instance.uptimeAt) / 1000;
        const uptimeSeconds = instance.uptimeBase + elapsed;
        const uptimeEl = instance.host.querySelector('[data-detail="uptime"]');
        if (uptimeEl) uptimeEl.textContent = formatUptime(uptimeSeconds);
    }

    function gaugeMarkup(key, label) {
        return `<article class="vd-sysinfo-gauge vd-sysinfo-gauge-${key}">
            <div class="vd-sysinfo-gauge-canvas"><canvas data-gauge="${key}"></canvas></div>
            <div class="vd-sysinfo-gauge-value" data-value="${key}">--%</div>
            <div class="vd-sysinfo-gauge-label">${escapeHtml(label)}</div>
        </article>`;
    }

    function setStatus(instance, text) {
        const el = instance.host.querySelector('[data-role="status"]');
        if (el) el.textContent = text;
    }

    function updateGauge(instance, key, value) {
        const n = pct(value);
        const valueEl = instance.host.querySelector(`[data-value="${key}"]`);
        if (valueEl) valueEl.textContent = `${n.toFixed(1)}%`;
        const chart = instance.gauges[key];
        if (chart) {
            chart.data.datasets[0].data = [n, Math.max(0, 100 - n)];
            chart.update('none');
        }
    }

    function updateFromMetrics(instance, metrics) {
        if (!metrics) return;
        instance.lastMetrics = metrics;
        instance.uptimeBase = Number(metrics.uptime_seconds || 0);
        instance.uptimeAt = Date.now();

        const cpu = pct(metrics.cpu && metrics.cpu.usage_percent);
        const memory = pct(metrics.memory && metrics.memory.used_percent);
        const disk = pct(metrics.disk && metrics.disk.used_percent);

        updateGauge(instance, 'cpu', cpu);
        updateGauge(instance, 'memory', memory);
        updateGauge(instance, 'disk', disk);

        const now = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
        instance.history.push({ label: now, cpu, memory, disk });
        if (instance.history.length > maxHistory) instance.history.shift();
        updateHistory(instance);
        updateDetails(instance, metrics);
        setStatus(instance, t(instance.context, 'desktop.system_info_updated', { time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) }));
    }

    function updateHistory(instance) {
        const chart = instance.historyChart;
        if (!chart) return;
        chart.data.labels = instance.history.map(point => point.label);
        chart.data.datasets[0].data = instance.history.map(point => point.cpu);
        chart.data.datasets[1].data = instance.history.map(point => point.memory);
        chart.data.datasets[2].data = instance.history.map(point => point.disk);
        chart.update('none');
    }

    function updateDetails(instance, metrics) {
        const cpu = metrics.cpu || {};
        const memory = metrics.memory || {};
        const disk = metrics.disk || {};
        const network = metrics.network || {};
        const detailHost = instance.host.querySelector('[data-role="details"]');
        if (!detailHost) return;
        const uptimeSeconds = instance.uptimeBase + (Date.now() - instance.uptimeAt) / 1000;
        const details = [
            ['cpu_model', t(instance.context, 'desktop.system_info_cpu_model'), cpu.model_name || '-'],
            ['cores', t(instance.context, 'desktop.system_info_cores'), cpu.cores || '-'],
            ['memory', t(instance.context, 'desktop.system_info_memory'), `${formatBytes(memory.used)} / ${formatBytes(memory.total)}`],
            ['disk', t(instance.context, 'desktop.system_info_disk'), `${formatBytes(disk.used)} / ${formatBytes(disk.total)}`],
            ['network', t(instance.context, 'desktop.system_info_network'), `${formatBytes(network.bytes_sent)} up / ${formatBytes(network.bytes_recv)} down`],
            ['uptime', t(instance.context, 'desktop.system_info_uptime'), formatUptime(uptimeSeconds)]
        ];
        detailHost.innerHTML = details.map(([id, label, value]) => `<article class="vd-sysinfo-detail">
            <span>${escapeHtml(label)}</span>
            <strong data-detail="${id}">${escapeHtml(value)}</strong>
        </article>`).join('');
    }

    function dispose(windowId) {
        const instance = instances.get(windowId);
        if (!instance) return;
        if (instance.tickTimer) {
            clearInterval(instance.tickTimer);
            instance.tickTimer = null;
        }
        if (window.AuraSSE && typeof window.AuraSSE.off === 'function' && instance.handler) {
            window.AuraSSE.off('system_metrics', instance.handler);
        }
        Object.values(instance.gauges || {}).forEach(chart => {
            if (chart && typeof chart.destroy === 'function') chart.destroy();
        });
        if (instance.historyChart && typeof instance.historyChart.destroy === 'function') instance.historyChart.destroy();
        instances.delete(windowId);
    }

    window.SystemInfoApp = { render, dispose };
}());
