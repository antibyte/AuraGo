    /* System Monitor widget: live host metrics. Initial fetch of
       /api/dashboard/system, then live updates via the server-side
       'system_metrics' SSE broadcast (every 10s). All updates happen in
       place (text/attributes only) so re-renders never rebuild the DOM. */
    const SYSMON_HISTORY_LEN = 30; // 30 samples at 10s interval = 5 minutes

    function sysmonFormatBytes(value) {
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

    function sysmonFormatUptime(seconds) {
        let total = Math.max(0, Math.floor(Number(seconds || 0)));
        const days = Math.floor(total / 86400);
        total -= days * 86400;
        const hours = Math.floor(total / 3600);
        total -= hours * 3600;
        const minutes = Math.floor(total / 60);
        if (days > 0) return `${days}d ${hours}h`;
        if (hours > 0) return `${hours}h ${minutes}m`;
        return `${minutes}m`;
    }

    function sysmonClampPct(value) {
        const n = Number(value || 0);
        if (!Number.isFinite(n)) return 0;
        return Math.max(0, Math.min(100, n));
    }

    function renderSysmonWidget(container) {
        container.innerHTML = `<div class="vd-sysmon">
            <div class="vd-sysmon-title-row">
                <span class="vd-sysmon-title-icon">${iconMarkup('analytics', 'S', 'vd-sysmon-title-glyph', 15)}</span>
                <span class="vd-sysmon-title">${esc(t('desktop.widget_sysmon_title'))}</span>
                <span class="vd-sysmon-updated"></span>
            </div>
            <div class="vd-sysmon-metric">
                <div class="vd-sysmon-metric-head">
                    <span class="vd-sysmon-label">${esc(t('desktop.system_info_cpu'))}</span>
                    <span class="vd-sysmon-value" data-sysmon="cpu-value">–</span>
                </div>
                <div class="vd-sysmon-bar"><div class="vd-sysmon-bar-fill vd-sysmon-fill-cpu" data-sysmon="cpu-fill"></div></div>
                <div class="vd-sysmon-sub" data-sysmon="cpu-sub"></div>
            </div>
            <div class="vd-sysmon-metric">
                <div class="vd-sysmon-metric-head">
                    <span class="vd-sysmon-label">${esc(t('desktop.system_info_memory'))}</span>
                    <span class="vd-sysmon-value" data-sysmon="mem-value">–</span>
                </div>
                <div class="vd-sysmon-bar"><div class="vd-sysmon-bar-fill vd-sysmon-fill-mem" data-sysmon="mem-fill"></div></div>
                <div class="vd-sysmon-sub" data-sysmon="mem-sub"></div>
            </div>
            <div class="vd-sysmon-metric">
                <div class="vd-sysmon-metric-head">
                    <span class="vd-sysmon-label">${esc(t('desktop.system_info_disk'))}</span>
                    <span class="vd-sysmon-value" data-sysmon="disk-value">–</span>
                </div>
                <div class="vd-sysmon-bar"><div class="vd-sysmon-bar-fill vd-sysmon-fill-disk" data-sysmon="disk-fill"></div></div>
                <div class="vd-sysmon-sub" data-sysmon="disk-sub"></div>
            </div>
            <div class="vd-sysmon-spark">
                <svg class="vd-sysmon-spark-svg" viewBox="0 0 100 28" preserveAspectRatio="none" aria-hidden="true">
                    <path class="vd-sysmon-spark-area" data-sysmon="spark-area" d=""></path>
                    <path class="vd-sysmon-spark-line" data-sysmon="spark-line" d=""></path>
                </svg>
            </div>
            <div class="vd-sysmon-footer">
                <span class="vd-sysmon-net" data-sysmon="net-up"></span>
                <span class="vd-sysmon-net" data-sysmon="net-down"></span>
                <span class="vd-sysmon-uptime" data-sysmon="uptime"></span>
            </div>
        </div>`;

        const $ = key => container.querySelector(`[data-sysmon="${key}"]`);
        const refs = {
            root: container.querySelector('.vd-sysmon'),
            cpuValue: $('cpu-value'), cpuFill: $('cpu-fill'), cpuSub: $('cpu-sub'),
            memValue: $('mem-value'), memFill: $('mem-fill'), memSub: $('mem-sub'),
            diskValue: $('disk-value'), diskFill: $('disk-fill'), diskSub: $('disk-sub'),
            sparkLine: $('spark-line'), sparkArea: $('spark-area'),
            netUp: $('net-up'), netDown: $('net-down'),
            uptimeEl: $('uptime'), updatedEl: container.querySelector('.vd-sysmon-updated')
        };

        let disposed = false;
        let history = [];
        let lastNet = null;
        let processUptimeBase = null;
        let hostUptimeBase = null;

        function renderSpark() {
            if (!history.length) {
                refs.sparkLine.setAttribute('d', '');
                refs.sparkArea.setAttribute('d', '');
                return;
            }
            const step = 100 / (SYSMON_HISTORY_LEN - 1);
            const offset = SYSMON_HISTORY_LEN - history.length;
            const pts = history.map((v, i) => {
                const x = ((offset + i) * step).toFixed(2);
                const y = (27 - (v / 100) * 25).toFixed(2);
                return `${x},${y}`;
            });
            refs.sparkLine.setAttribute('d', 'M' + pts.join(' L'));
            refs.sparkArea.setAttribute('d', 'M' + pts.join(' L') + ' L100,28 L0,28 Z');
        }

        function renderUptime() {
            if (disposed) return;
            const now = Date.now();
            const parts = [];
            if (processUptimeBase) {
                const seconds = processUptimeBase.seconds + (now - processUptimeBase.at) / 1000;
                parts.push(`${t('desktop.system_info_uptime')} ${sysmonFormatUptime(seconds)}`);
            }
            if (hostUptimeBase) {
                const seconds = hostUptimeBase.seconds + (now - hostUptimeBase.at) / 1000;
                parts.push(`Host ${sysmonFormatUptime(seconds)}`);
            }
            refs.uptimeEl.textContent = parts.join(' · ');
        }

        function renderMetrics(data) {
            if (disposed || !data) return;
            const cpuPct = sysmonClampPct(data.cpu && data.cpu.usage_percent);
            const memPct = sysmonClampPct(data.memory && data.memory.used_percent);
            const diskPct = sysmonClampPct(data.disk && data.disk.used_percent);

            refs.cpuValue.textContent = `${Math.round(cpuPct)}%`;
            refs.cpuFill.style.width = `${cpuPct}%`;
            refs.cpuSub.textContent = data.cpu && data.cpu.cores
                ? `${data.cpu.cores} ${t('desktop.system_info_cores')}`
                : '';

            refs.memValue.textContent = `${Math.round(memPct)}%`;
            refs.memFill.style.width = `${memPct}%`;
            refs.memSub.textContent = `${sysmonFormatBytes(data.memory && data.memory.used)} / ${sysmonFormatBytes(data.memory && data.memory.total)}`;

            refs.diskValue.textContent = `${Math.round(diskPct)}%`;
            refs.diskFill.style.width = `${diskPct}%`;
            refs.diskSub.textContent = `${sysmonFormatBytes(data.disk && data.disk.used)} / ${sysmonFormatBytes(data.disk && data.disk.total)}`;

            history.push(cpuPct);
            if (history.length > SYSMON_HISTORY_LEN) history.shift();
            renderSpark();

            const now = Date.now();
            if (data.network) {
                if (lastNet) {
                    const dt = Math.max(1, (now - lastNet.time) / 1000);
                    const upRate = Math.max(0, (Number(data.network.bytes_sent || 0) - lastNet.sent) / dt);
                    const downRate = Math.max(0, (Number(data.network.bytes_recv || 0) - lastNet.recv) / dt);
                    refs.netUp.textContent = `\u2191 ${sysmonFormatBytes(upRate)}/s`;
                    refs.netDown.textContent = `\u2193 ${sysmonFormatBytes(downRate)}/s`;
                }
                lastNet = {
                    sent: Number(data.network.bytes_sent || 0),
                    recv: Number(data.network.bytes_recv || 0),
                    time: now
                };
            }

            processUptimeBase = { seconds: Number(data.uptime_seconds || 0), at: now };
            hostUptimeBase = typeof data.host_uptime_seconds === 'number'
                ? { seconds: data.host_uptime_seconds, at: now }
                : null;
            renderUptime();

            refs.updatedEl.textContent = t('desktop.system_info_updated', {
                time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
            });
            refs.root.classList.add('is-ready');
        }

        api('/api/dashboard/system')
            .then(data => renderMetrics(data))
            .catch(() => {
                if (disposed) return;
                refs.updatedEl.textContent = t('desktop.system_info_unavailable');
                refs.root.classList.add('is-ready');
            });

        const sseHandler = payload => renderMetrics(payload);
        if (window.AuraSSE && typeof window.AuraSSE.on === 'function') {
            window.AuraSSE.on('system_metrics', sseHandler);
        }
        const uptimeTimer = setInterval(renderUptime, 1000);

        registerWidgetCleanup(() => {
            disposed = true;
            if (window.AuraSSE && typeof window.AuraSSE.off === 'function') {
                window.AuraSSE.off('system_metrics', sseHandler);
            }
            clearInterval(uptimeTimer);
        });
    }
