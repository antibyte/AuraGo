    /* Fritz!Box widget: read-only router overview with four swipeable pages
       (connection, traffic, devices, telephony). Data comes from the
       admin-scoped GET /api/desktop/fritzbox/overview endpoint; the widget
       polls the fast connection section every 5 s while visible and the slow
       sections once a minute. Throughput history is a client-side ring buffer
       seeded by the FRITZ!OS online monitor (20 x 5 s per response), so the
       chart is filled immediately and grows to 15 minutes. Router-provided
       text (host names, SSIDs, caller names) is rendered via textContent only. */
    const FRITZ_WIDGET_POLL_MS = 5000;
    const FRITZ_WIDGET_SLOW_MS = 60000;
    const FRITZ_WIDGET_SYSTEM_MS = 300000;
    const FRITZ_WIDGET_PAGE_REFRESH_MS = 20000;
    const FRITZ_WIDGET_HISTORY_MAX = 180; // 15 minutes at 5 s
    const FRITZ_WIDGET_HISTORY_MIN_SPAN_MS = 100000;
    const FRITZ_WIDGET_HISTORY_SPAN_MS = 15 * 60 * 1000;
    const FRITZ_WIDGET_PAGE_KEY = 'aurago.desktop.fritzbox.page';
    const FRITZ_WIDGET_PAGES = ['connection', 'traffic', 'devices', 'telephony'];
    const FRITZ_WIDGET_GLYPHS = {
        down: '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M8 2v9.2l3.6-3.6 1.4 1.4L8 15 3 9l1.4-1.4L8 11.2V2z"/></svg>',
        up: '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M8 14V4.8L4.4 8.4 3 7l5-6 5 6-1.4 1.4L8 4.8V14z"/></svg>',
        lan: '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M2 3h12v6H9v2h3v2H4v-2h3V9H2V3zm2 2v2h8V5H4z"/></svg>',
        wlan: '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M8 13.5a1.5 1.5 0 1 1 0-3 1.5 1.5 0 0 1 0 3zM4.5 8.9l1.4 1.4a3 3 0 0 1 4.2 0l1.4-1.4a5 5 0 0 0-7 0zM1.7 6.1l1.4 1.4a7 7 0 0 1 9.8 0l1.4-1.4a9 9 0 0 0-12.6 0z"/></svg>',
        incoming: '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M13 3l-6.6 6.6V6H4.6v6h6V10.2H7.4L14 3.6z"/></svg>',
        outgoing: '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M3 13l6.6-6.6V10h1.8V4h-6v1.8h3.6L2.4 12.4z"/></svg>',
        missed: '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M4.2 3l3.8 3.8L11.8 3 13 4.2 9.2 8 13 11.8 11.8 13 8 9.2 4.2 13 3 11.8 6.8 8 3 4.2z"/></svg>',
        phone: '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M3.6 2h2.9l1.2 3.2-1.6 1.2a7.4 7.4 0 0 0 3.5 3.5l1.2-1.6L14 9.5v2.9A1.6 1.6 0 0 1 12.4 14 10.4 10.4 0 0 1 2 3.6 1.6 1.6 0 0 1 3.6 2z"/></svg>'
    };

    function fritzWidgetPagesFor(capabilities) {
        const caps = capabilities || {};
        return FRITZ_WIDGET_PAGES.filter(page => {
            if (page === 'connection' || page === 'traffic') return !!caps.connection;
            if (page === 'devices') return !!caps.devices;
            return !!caps.telephony;
        });
    }

    function fritzWidgetShell(label, uid) {
        const glyph = key => `<span class="vd-fritz-glyph">${FRITZ_WIDGET_GLYPHS[key]}</span>`;
        const fact = (key, valueKey, extra) => `<div class="vd-fritz-fact"><dt>${esc(label(key))}</dt><dd><span data-fritz="${valueKey}">–</span>${extra || ''}</dd></div>`;
        const copyButton = key => `<button type="button" class="vd-fritz-copy" data-fritz="copy-${key}" aria-label="${esc(label('copy_ip'))}" title="${esc(label('copy_ip'))}" hidden>${esc(t('desktop.copy'))}</button>`;
        return `<div class="vd-fritz" data-fritz="root" tabindex="0" role="region" aria-roledescription="carousel" aria-label="${esc(label('title'))}" id="${uid}">
            <div class="vd-fritz-head">
                <span class="vd-fritz-dot is-unknown" data-fritz="dot" aria-hidden="true"></span>
                <span class="vd-fritz-title" data-fritz="model">${esc(label('title'))}</span>
                <span class="vd-fritz-page-title" data-fritz="page-title" aria-live="polite"></span>
                <span class="vd-fritz-updated" data-fritz="updated"></span>
            </div>
            <div class="vd-fritz-viewport" data-fritz="viewport">
                <div class="vd-fritz-track" data-fritz="track">
                    <section class="vd-fritz-page" data-fritz-page="connection" aria-label="${esc(label('page_connection'))}">
                        <div class="vd-fritz-status"><span class="vd-fritz-status-text" data-fritz="status-text">–</span><span class="vd-fritz-status-since" data-fritz="status-since"></span></div>
                        <div class="vd-fritz-kpis">
                            <div class="vd-fritz-kpi is-down">
                                <span class="vd-fritz-kpi-label">${glyph('down')}${esc(label('download'))}</span>
                                <span class="vd-fritz-kpi-value"><b data-fritz="down-number">–</b><small data-fritz="down-unit"></small></span>
                                <div class="vd-fritz-kpi-spark" data-fritz="down-spark"></div>
                            </div>
                            <div class="vd-fritz-kpi is-up">
                                <span class="vd-fritz-kpi-label">${glyph('up')}${esc(label('upload'))}</span>
                                <span class="vd-fritz-kpi-value"><b data-fritz="up-number">–</b><small data-fritz="up-unit"></small></span>
                                <div class="vd-fritz-kpi-spark" data-fritz="up-spark"></div>
                            </div>
                        </div>
                        <div class="vd-fritz-gauges">
                            <div class="vd-fritz-gauge is-down"><div class="vd-fritz-gauge-arc" data-fritz="down-gauge"></div><b data-fritz="down-pct">–</b><span data-fritz="down-max"></span></div>
                            <div class="vd-fritz-gauge is-up"><div class="vd-fritz-gauge-arc" data-fritz="up-gauge"></div><b data-fritz="up-pct">–</b><span data-fritz="up-max"></span></div>
                        </div>
                        <dl class="vd-fritz-facts">
                            ${fact('external_ip', 'ipv4', copyButton('ipv4'))}
                            ${fact('external_ipv6', 'ipv6', copyButton('ipv6'))}
                            ${fact('access_type', 'access')}
                            ${fact('box_uptime', 'box-uptime')}
                        </dl>
                    </section>
                    <section class="vd-fritz-page" data-fritz-page="traffic" aria-label="${esc(label('page_traffic'))}">
                        <div class="vd-fritz-chart" data-fritz="chart-host">
                            <div class="vd-fritz-chart-canvas" data-fritz="chart"></div>
                            <div class="vd-fritz-chart-cursor" data-fritz="chart-cursor" hidden></div>
                            <div class="vd-fritz-chart-tip" data-fritz="chart-tip" hidden><span data-fritz="tip-time"></span><span class="is-down" data-fritz="tip-down"></span><span class="is-up" data-fritz="tip-up"></span></div>
                        </div>
                        <div class="vd-fritz-legend">
                            <span class="vd-fritz-legend-item is-down"><i></i>${esc(label('download'))} <b data-fritz="legend-down">–</b></span>
                            <span class="vd-fritz-legend-item is-up"><i></i>${esc(label('upload'))} <b data-fritz="legend-up">–</b></span>
                            <span class="vd-fritz-legend-span" data-fritz="legend-span"></span>
                        </div>
                        <dl class="vd-fritz-facts is-grid">
                            ${fact('received', 'total-down')}
                            ${fact('sent', 'total-up')}
                            ${fact('peak_down', 'peak-down')}
                            ${fact('peak_up', 'peak-up')}
                        </dl>
                    </section>
                    <section class="vd-fritz-page" data-fritz-page="devices" aria-label="${esc(label('page_devices'))}">
                        <div class="vd-fritz-devices-head">
                            <div class="vd-fritz-ring" data-fritz="ring"></div>
                            <div class="vd-fritz-devices-summary">
                                <span class="vd-fritz-devices-count"><b data-fritz="dev-active">–</b> ${esc(label('devices_active'))}</span>
                                <span class="vd-fritz-devices-total" data-fritz="dev-total"></span>
                                <div class="vd-fritz-chips">
                                    <span class="vd-fritz-chip is-lan">${glyph('lan')}${esc(label('lan'))} <b data-fritz="dev-lan">–</b></span>
                                    <span class="vd-fritz-chip is-wlan">${glyph('wlan')}${esc(label('wlan'))} <b data-fritz="dev-wlan">–</b></span>
                                </div>
                            </div>
                        </div>
                        <div class="vd-fritz-wlans" data-fritz="wlans"></div>
                        <ul class="vd-fritz-list" data-fritz="hosts"></ul>
                        <div class="vd-fritz-empty" data-fritz="hosts-empty" hidden>${esc(label('no_devices'))}</div>
                    </section>
                    <section class="vd-fritz-page" data-fritz-page="telephony" aria-label="${esc(label('page_telephony'))}">
                        <div class="vd-fritz-badges">
                            <div class="vd-fritz-badge is-missed">${glyph('missed')}<b data-fritz="missed">–</b><span>${esc(label('missed_today'))}</span></div>
                            <div class="vd-fritz-badge is-tam" data-fritz="tam-badge">${glyph('phone')}<b data-fritz="tam">–</b><span>${esc(label('tam_new'))}</span></div>
                        </div>
                        <ul class="vd-fritz-list is-calls" data-fritz="calls"></ul>
                        <div class="vd-fritz-empty" data-fritz="calls-empty" hidden>${esc(label('no_calls'))}</div>
                    </section>
                </div>
                <button type="button" class="vd-fritz-arrow is-prev" data-fritz="prev" aria-label="${esc(label('prev_page'))}">&#8249;</button>
                <button type="button" class="vd-fritz-arrow is-next" data-fritz="next" aria-label="${esc(label('next_page'))}">&#8250;</button>
            </div>
            <div class="vd-fritz-dots" data-fritz="dots" role="tablist"></div>
            <div class="vd-fritz-banner" data-fritz="banner" hidden><span data-fritz="banner-text"></span><button type="button" data-fritz="retry">${esc(t('desktop.retry'))}</button></div>
            <div class="vd-fritz-empty is-notice" data-fritz="notice" hidden></div>
            <div class="vd-fritz-skeleton" data-fritz="skeleton" aria-hidden="true"><i></i><i></i><i></i><i></i></div>
        </div>`;
    }

    function renderFritzBoxWidget(container) {
        const label = (key, vars) => t('desktop.widget_fritzbox_' + key, vars);
        const uid = 'fritz-' + Math.random().toString(36).slice(2, 8);
        container.innerHTML = fritzWidgetShell(label, uid);
        const refs = Object.fromEntries([...container.querySelectorAll('[data-fritz]')].map(el => [el.dataset.fritz, el]));
        const pageEls = Object.fromEntries([...container.querySelectorAll('[data-fritz-page]')].map(el => [el.dataset.fritzPage, el]));
        const state = {
            disposed: false, controller: null, timer: null, pages: [], page: 0, capabilities: null,
            history: [], connection: null, devices: null, telephony: null, system: null,
            fetchedAt: { connection: 0, devices: 0, telephony: 0, system: 0 }, hasData: false,
            chart: null, compact: false, copyTimer: null, drag: null
        };
        const lang = () => document.documentElement.lang || undefined;
        const clockFormat = new Intl.DateTimeFormat(lang(), { hour: '2-digit', minute: '2-digit' });
        const tipFormat = new Intl.DateTimeFormat(lang(), { hour: '2-digit', minute: '2-digit', second: '2-digit' });

        /* ---------- pager ---------- */
        function pageTitle(page) { return label('page_' + page); }

        function setPages(pages) {
            if (pages.join(',') === state.pages.join(',') && state.hasData) return;
            state.pages = pages;
            for (const page of FRITZ_WIDGET_PAGES) {
                pageEls[page].hidden = !pages.includes(page);
            }
            refs.dots.innerHTML = '';
            pages.forEach((page, index) => {
                const dot = document.createElement('button');
                dot.type = 'button';
                dot.className = 'vd-fritz-dotbtn';
                dot.setAttribute('role', 'tab');
                dot.setAttribute('aria-label', label('page_of', { current: index + 1, total: pages.length }) + ' · ' + pageTitle(page));
                dot.dataset.fritzDot = String(index);
                dot.addEventListener('click', () => setPage(index, true));
                refs.dots.appendChild(dot);
            });
            refs.dots.hidden = pages.length < 2;
            refs.prev.hidden = refs.next.hidden = pages.length < 2;
            refs.notice.hidden = pages.length > 0;
            if (!pages.length) refs.notice.textContent = label('no_sections');
            let stored = '';
            try { stored = localStorage.getItem(FRITZ_WIDGET_PAGE_KEY) || ''; } catch (_) { stored = ''; }
            setPage(Math.max(0, pages.indexOf(stored)), false);
        }

        function setPage(index, persist) {
            if (!state.pages.length) return;
            const next = Math.max(0, Math.min(state.pages.length - 1, index));
            state.page = next;
            refs.track.style.transform = `translateX(${-next * 100}%)`;
            refs.track.classList.remove('is-dragging');
            [...refs.dots.children].forEach((dot, i) => {
                dot.classList.toggle('is-active', i === next);
                dot.setAttribute('aria-selected', i === next ? 'true' : 'false');
            });
            state.pages.forEach((page, i) => {
                pageEls[page].classList.toggle('is-active', i === next);
                pageEls[page].setAttribute('aria-hidden', i === next ? 'false' : 'true');
            });
            refs.prev.disabled = next === 0;
            refs.next.disabled = next === state.pages.length - 1;
            refs['page-title'].textContent = pageTitle(state.pages[next]);
            if (persist) {
                try { localStorage.setItem(FRITZ_WIDGET_PAGE_KEY, state.pages[next]); } catch (_) { /* storage unavailable */ }
                const current = state.pages[next];
                if ((current === 'devices' || current === 'telephony') && Date.now() - state.fetchedAt[current] > FRITZ_WIDGET_PAGE_REFRESH_MS) refresh([current]);
            }
            if (state.pages[next] === 'traffic') renderChart();
        }

        /* ---------- formatting helpers ---------- */
        function setBits(numberEl, unitEl, bps) {
            const parts = fritzSplitBits(bps);
            numberEl.textContent = parts.number;
            unitEl.textContent = parts.unit;
        }

        function accessLabel(type) {
            const known = ['dsl', 'cable', 'fiber', 'ethernet', 'mobile'];
            return label('access_' + (known.includes(type) ? type : 'other'));
        }

        /* ---------- connection page ---------- */
        function renderConnection() {
            const c = state.connection;
            if (!c) return;
            refs.dot.className = 'vd-fritz-dot ' + (c.online ? 'is-online' : (c.status === 'Connecting' ? 'is-connecting' : 'is-offline'));
            refs['status-text'].textContent = c.online ? label('online') : (c.status === 'Connecting' ? label('connecting') : label('offline'));
            refs['status-text'].title = c.online ? '' : String(c.last_error || '');
            refs['status-text'].classList.toggle('is-offline', !c.online && c.status !== 'Connecting');
            refs['status-since'].textContent = c.online && c.uptime_seconds > 0 ? label('connected_for', { duration: sysmonFormatUptime(c.uptime_seconds) }) : '';
            setBits(refs['down-number'], refs['down-unit'], c.down_bps);
            setBits(refs['up-number'], refs['up-unit'], c.up_bps);
            const recent = state.history.slice(-24);
            refs['down-spark'].innerHTML = fritzSparkSVG(recent.map(s => s.down), 100, 18, 'is-down');
            refs['up-spark'].innerHTML = fritzSparkSVG(recent.map(s => s.up), 100, 18, 'is-up');
            for (const dir of ['down', 'up']) {
                const max = Number(c['max_' + dir + '_bps']) || 0;
                const ratio = max > 0 ? (Number(c[dir + '_bps']) || 0) / max : 0;
                refs[dir + '-gauge'].innerHTML = fritzGaugeSVG(ratio, 72, 'is-' + dir);
                refs[dir + '-pct'].textContent = max > 0 ? fritzFormatPercent(ratio) : '–';
                refs[dir + '-max'].textContent = max > 0 ? label('of_max', { rate: fritzFormatBits(max) }) : label('utilization');
            }
            for (const key of ['ipv4', 'ipv6']) {
                const value = String(c['external_' + key] || '').trim();
                refs[key].textContent = value || '–';
                refs[key].title = value;
                refs['copy-' + key].hidden = !value || !(navigator.clipboard && navigator.clipboard.writeText);
            }
            refs.access.textContent = c.access_type ? accessLabel(c.access_type) : '–';
            refs['box-uptime'].textContent = state.system && state.system.uptime_seconds > 0 ? sysmonFormatUptime(state.system.uptime_seconds) : '–';
            refs['total-down'].textContent = c.total_received_bytes > 0 ? sysmonFormatBytes(c.total_received_bytes) : '–';
            refs['total-up'].textContent = c.total_sent_bytes > 0 ? sysmonFormatBytes(c.total_sent_bytes) : '–';
        }

        /* ---------- traffic page ---------- */
        function chartRange() {
            const samples = state.history;
            const end = samples.length ? samples[samples.length - 1].t : Date.now();
            const first = samples.length ? samples[0].t : end;
            const span = Math.max(FRITZ_WIDGET_HISTORY_MIN_SPAN_MS, Math.min(FRITZ_WIDGET_HISTORY_SPAN_MS, end - first));
            return { start: end - span, end };
        }

        function renderChart() {
            if (state.disposed || pageEls.traffic.hidden) return;
            const samples = state.history;
            const width = Math.max(120, refs.chart.clientWidth || 300);
            const height = state.compact ? 72 : 104;
            const range = chartRange();
            const result = fritzAreaChartSVG({ samples, width, height, range, idPrefix: uid, compact: state.compact, gapMs: 15000 });
            refs.chart.innerHTML = result.svg;
            state.chart = { result, width };
            const latest = samples.length ? samples[samples.length - 1] : null;
            refs['legend-down'].textContent = latest ? fritzFormatBits(latest.down) : '–';
            refs['legend-up'].textContent = latest ? fritzFormatBits(latest.up) : '–';
            const spanMinutes = Math.max(1, Math.round((range.end - range.start) / 60000));
            refs['legend-span'].textContent = label('window_label', { span: spanMinutes + ' min' });
            let peakDown = 0;
            let peakUp = 0;
            for (const sample of samples) {
                if (sample.t < range.start) continue;
                peakDown = Math.max(peakDown, sample.down);
                peakUp = Math.max(peakUp, sample.up);
            }
            refs['peak-down'].textContent = samples.length ? fritzFormatBits(peakDown) : '–';
            refs['peak-up'].textContent = samples.length ? fritzFormatBits(peakUp) : '–';
        }

        function showChartTip(event) {
            if (!state.chart || !state.history.length) return;
            const rect = refs.chart.getBoundingClientRect();
            const scale = rect.width ? state.chart.width / rect.width : 1;
            const x = (event.clientX - rect.left) * scale;
            const plotWidth = state.chart.result.plotWidth;
            if (x > plotWidth) { hideChartTip(); return; }
            const index = fritzChartIndexAt(state.history, state.chart.result.range, plotWidth, x);
            const sample = state.history[index];
            if (!sample) return;
            const px = ((sample.t - state.chart.result.range.start) / Math.max(1, state.chart.result.range.end - state.chart.result.range.start)) * (plotWidth / scale);
            refs['chart-cursor'].hidden = false;
            refs['chart-cursor'].style.left = `${Math.max(0, px)}px`;
            refs['chart-tip'].hidden = false;
            refs['tip-time'].textContent = tipFormat.format(new Date(sample.t));
            refs['tip-down'].textContent = fritzFormatBits(sample.down);
            refs['tip-up'].textContent = fritzFormatBits(sample.up);
            const tipWidth = refs['chart-tip'].offsetWidth || 120;
            refs['chart-tip'].style.left = `${Math.max(0, Math.min(rect.width - tipWidth, px - tipWidth / 2))}px`;
        }

        function hideChartTip() {
            refs['chart-cursor'].hidden = true;
            refs['chart-tip'].hidden = true;
        }

        /* ---------- devices page ---------- */
        function renderDevices() {
            const d = state.devices;
            if (!d) return;
            refs.ring.innerHTML = fritzRingSVG(d.active, d.total, 64);
            refs['dev-active'].textContent = String(d.active);
            refs['dev-total'].textContent = label('devices_total', { total: d.total });
            refs['dev-lan'].textContent = String(d.lan_active);
            refs['dev-wlan'].textContent = String(d.wlan_active);
            refs.wlans.innerHTML = '';
            for (const wlan of d.wlans || []) {
                const chip = document.createElement('span');
                chip.className = 'vd-fritz-wlan' + (wlan.enabled ? ' is-on' : ' is-off') + (wlan.guest ? ' is-guest' : '');
                const state_ = document.createElement('i');
                state_.setAttribute('aria-hidden', 'true');
                const name = document.createElement('b');
                name.textContent = wlan.ssid || label('wlan');
                const meta = document.createElement('small');
                meta.textContent = wlan.guest ? label('guest') : (wlan.band ? `${wlan.band} GHz` : '') + (wlan.enabled ? '' : (wlan.band ? ' · ' : '') + label('wlan_off'));
                chip.append(state_, name, meta);
                chip.title = wlan.ssid || '';
                refs.wlans.appendChild(chip);
            }
            const active = (d.hosts || []).filter(host => host.active);
            refs.hosts.innerHTML = '';
            for (const host of active) {
                const item = document.createElement('li');
                item.className = 'vd-fritz-host is-' + (host.interface || 'other');
                const glyph = document.createElement('span');
                glyph.className = 'vd-fritz-glyph';
                glyph.innerHTML = FRITZ_WIDGET_GLYPHS[host.interface === 'wlan' ? 'wlan' : 'lan'];
                const name = document.createElement('span');
                name.className = 'vd-fritz-host-name';
                name.textContent = host.name || host.ip || '';
                name.title = name.textContent;
                const ip = document.createElement('span');
                ip.className = 'vd-fritz-host-ip';
                ip.textContent = host.ip || '';
                item.append(glyph, name, ip);
                refs.hosts.appendChild(item);
            }
            const hidden = Math.max(0, d.active - active.length);
            if (hidden > 0) {
                const more = document.createElement('li');
                more.className = 'vd-fritz-host is-more';
                more.textContent = label('more_devices', { count: hidden });
                refs.hosts.appendChild(more);
            }
            refs['hosts-empty'].hidden = active.length > 0;
        }

        /* ---------- telephony page ---------- */
        function renderTelephony() {
            const tel = state.telephony;
            if (!tel) return;
            refs.missed.textContent = String(tel.missed_today);
            refs['tam-badge'].hidden = !tel.tam_available;
            refs.tam.textContent = String(tel.tam_new);
            refs['tam-badge'].classList.toggle('has-new', tel.tam_new > 0);
            refs.calls.innerHTML = '';
            const now = Date.now();
            for (const call of tel.calls || []) {
                const item = document.createElement('li');
                item.className = 'vd-fritz-call is-' + (call.type || 'unknown');
                const glyph = document.createElement('span');
                glyph.className = 'vd-fritz-glyph';
                glyph.innerHTML = FRITZ_WIDGET_GLYPHS[call.type === 'outgoing' || call.type === 'active' ? 'outgoing' : (call.type === 'missed' || call.type === 'rejected' ? 'missed' : 'incoming')];
                glyph.title = label('call_' + (['incoming', 'outgoing', 'missed', 'active', 'rejected'].includes(call.type) ? call.type : 'unknown'));
                const who = document.createElement('span');
                who.className = 'vd-fritz-call-who';
                who.textContent = call.name || call.number || label('unknown_caller');
                who.title = [call.name, call.number].filter(Boolean).join(' · ');
                const when = document.createElement('span');
                when.className = 'vd-fritz-call-when';
                when.textContent = fritzRelativeTime(call.timestamp, call.date, now);
                const duration = document.createElement('span');
                duration.className = 'vd-fritz-call-duration';
                duration.textContent = call.type === 'missed' || call.type === 'rejected' ? '' : fritzFormatCallDuration(call.duration);
                item.append(glyph, who, when, duration);
                refs.calls.appendChild(item);
            }
            refs['calls-empty'].hidden = (tel.calls || []).length > 0;
        }

        /* ---------- data flow ---------- */
        function mergeHistory(connection, ageMs) {
            const end = Date.now() - Math.max(0, ageMs || 0);
            const monitor = connection.monitor;
            if (monitor && Array.isArray(monitor.down_bps) && monitor.down_bps.length) {
                fritzMergeMonitorSamples(state.history, monitor.down_bps, monitor.up_bps || [], (monitor.interval_seconds || 5) * 1000, end, FRITZ_WIDGET_HISTORY_MAX);
            } else {
                fritzMergeMonitorSamples(state.history, [connection.down_bps || 0], [connection.up_bps || 0], FRITZ_WIDGET_POLL_MS, end, FRITZ_WIDGET_HISTORY_MAX);
            }
        }

        function applyOverview(payload) {
            const generated = Date.parse(payload.generated_at || '') || Date.now();
            const fetched = payload.fetched_at || {};
            const now = Date.now();
            state.capabilities = payload.capabilities || {};
            setPages(fritzWidgetPagesFor(state.capabilities));
            if (payload.system) {
                state.system = payload.system;
                state.fetchedAt.system = now;
                refs.model.textContent = payload.system.model || label('title');
                refs.model.title = payload.system.firmware ? `${label('firmware')} ${payload.system.firmware}` : '';
            }
            if (payload.connection) {
                state.connection = payload.connection;
                state.fetchedAt.connection = now;
                const age = fetched.connection ? Math.max(0, generated - Date.parse(fetched.connection)) : 0;
                mergeHistory(payload.connection, age);
                renderConnection();
                if (state.pages[state.page] === 'traffic') renderChart();
            }
            if (payload.devices) {
                state.devices = payload.devices;
                state.fetchedAt.devices = now;
                renderDevices();
            }
            if (payload.telephony) {
                state.telephony = payload.telephony;
                state.fetchedAt.telephony = now;
                renderTelephony();
            }
            const errors = payload.errors || {};
            const failing = Object.keys(errors);
            refs.root.classList.toggle('is-stale', failing.length > 0);
            if (failing.length) {
                refs['banner-text'].textContent = errors.connection === 'auth_failed' ? label('error_auth') : label('error') + (state.hasData || Object.keys(payload.stale || {}).length ? ' · ' + label('stale') : '');
                refs.banner.hidden = false;
                if (errors.connection) refs.dot.className = 'vd-fritz-dot is-stale';
            } else {
                refs.banner.hidden = true;
            }
            state.hasData = true;
            refs.root.classList.add('is-ready');
            refs.skeleton.hidden = true;
            refs.updated.textContent = t('desktop.system_info_updated', { time: clockFormat.format(new Date()) });
        }

        function dueSections(forced) {
            const now = Date.now();
            const caps = state.capabilities;
            const sections = new Set(forced || []);
            if (!caps || caps.connection) sections.add('connection');
            if (!caps || (caps.system && now - state.fetchedAt.system > FRITZ_WIDGET_SYSTEM_MS)) sections.add('system');
            for (const slow of ['devices', 'telephony']) {
                if (!caps || (caps[slow] && now - state.fetchedAt[slow] > FRITZ_WIDGET_SLOW_MS)) sections.add(slow);
            }
            return [...sections];
        }

        async function refresh(forced) {
            if (state.disposed || document.hidden || state.controller) return;
            const sections = dueSections(forced);
            if (!sections.length) return;
            const request = new AbortController();
            state.controller = request;
            try {
                const payload = await api('/api/desktop/fritzbox/overview?sections=' + encodeURIComponent(sections.join(',')), { signal: request.signal });
                if (state.disposed || request.signal.aborted) return;
                refs.root.classList.remove('is-disabled');
                applyOverview(payload);
            } catch (err) {
                if (state.disposed || request.signal.aborted) return;
                if (err && err.message === 'fritzbox_disabled') {
                    refs.root.classList.add('is-disabled', 'is-ready');
                    refs.skeleton.hidden = true;
                    refs.banner.hidden = true;
                    refs.notice.hidden = false;
                    refs.notice.textContent = label('disabled_hint');
                    refs.dots.hidden = refs.prev.hidden = refs.next.hidden = true;
                    refs.dot.className = 'vd-fritz-dot is-unknown';
                    refs['page-title'].textContent = '';
                    refs.updated.textContent = '';
                    state.hasData = false;
                    state.pages = [];
                    return;
                }
                refs.skeleton.hidden = true;
                refs.root.classList.add('is-ready', 'is-stale');
                refs['banner-text'].textContent = state.hasData ? label('error') + ' · ' + label('stale') : t('desktop.load_failed');
                refs.banner.hidden = false;
                refs.dot.className = 'vd-fritz-dot ' + (state.hasData ? 'is-stale' : 'is-unknown');
            } finally {
                if (state.controller === request) state.controller = null;
            }
        }

        function schedule() {
            clearTimeout(state.timer);
            // A disabled integration is re-checked slowly so enabling it in
            // Settings brings the widget back without re-adding it.
            const delay = refs.root.classList.contains('is-disabled') ? FRITZ_WIDGET_SLOW_MS : FRITZ_WIDGET_POLL_MS;
            state.timer = setTimeout(async () => {
                await refresh();
                if (!state.disposed && !document.hidden) schedule();
            }, delay);
        }

        /* ---------- interactions ---------- */
        refs.prev.addEventListener('click', () => setPage(state.page - 1, true));
        refs.next.addEventListener('click', () => setPage(state.page + 1, true));
        refs.retry.addEventListener('click', () => {
            if (state.controller) state.controller.abort();
            state.controller = null;
            refs.root.classList.remove('is-disabled');
            refs.notice.hidden = true;
            refresh(['connection', 'devices', 'telephony', 'system']);
        });
        refs.root.addEventListener('keydown', event => {
            if (event.target.closest('button, a, input')) return;
            if (event.key === 'ArrowRight') { event.preventDefault(); setPage(state.page + 1, true); }
            else if (event.key === 'ArrowLeft') { event.preventDefault(); setPage(state.page - 1, true); }
            else if (event.key === 'Home') { event.preventDefault(); setPage(0, true); }
            else if (event.key === 'End') { event.preventDefault(); setPage(state.pages.length - 1, true); }
        });
        for (const key of ['ipv4', 'ipv6']) {
            refs['copy-' + key].addEventListener('click', async event => {
                event.stopPropagation();
                const value = refs[key].textContent;
                if (!value || value === '–' || !navigator.clipboard) return;
                try {
                    await navigator.clipboard.writeText(value);
                    const button = refs['copy-' + key];
                    button.textContent = t('desktop.copied');
                    button.classList.add('is-done');
                    clearTimeout(state.copyTimer);
                    state.copyTimer = setTimeout(() => {
                        if (state.disposed) return;
                        button.textContent = t('desktop.copy');
                        button.classList.remove('is-done');
                    }, 1500);
                } catch (_) { /* clipboard denied */ }
            });
        }
        refs.viewport.addEventListener('pointerdown', event => {
            if (event.button !== 0 || event.target.closest('button')) return;
            state.drag = { id: event.pointerId, startX: event.clientX, startY: event.clientY, dx: 0, active: false };
        });
        refs.viewport.addEventListener('pointermove', event => {
            const drag = state.drag;
            if (!drag || drag.id !== event.pointerId) return;
            const dx = event.clientX - drag.startX;
            const dy = event.clientY - drag.startY;
            if (!drag.active) {
                if (Math.abs(dx) < 8 || Math.abs(dx) < Math.abs(dy)) return;
                drag.active = true;
                refs.track.classList.add('is-dragging');
                try { refs.viewport.setPointerCapture(event.pointerId); } catch (_) { /* unsupported */ }
            }
            drag.dx = dx;
            const width = refs.viewport.clientWidth || 1;
            const limited = Math.max(-width * 0.6, Math.min(width * 0.6, dx));
            refs.track.style.transform = `translateX(calc(${-state.page * 100}% + ${limited}px))`;
        });
        const endDrag = event => {
            const drag = state.drag;
            if (!drag || drag.id !== event.pointerId) return;
            state.drag = null;
            if (!drag.active) return;
            const width = refs.viewport.clientWidth || 1;
            if (drag.dx < -Math.min(48, width * 0.2)) setPage(state.page + 1, true);
            else if (drag.dx > Math.min(48, width * 0.2)) setPage(state.page - 1, true);
            else setPage(state.page, false);
        };
        refs.viewport.addEventListener('pointerup', endDrag);
        refs.viewport.addEventListener('pointercancel', endDrag);
        refs['chart-host'].addEventListener('pointermove', showChartTip);
        refs['chart-host'].addEventListener('pointerleave', hideChartTip);

        const onVisibility = () => {
            if (document.hidden) {
                clearTimeout(state.timer);
                if (state.controller) state.controller.abort();
                state.controller = null;
            } else {
                refresh().then(() => { if (!state.disposed && !document.hidden) schedule(); });
            }
        };
        document.addEventListener('visibilitychange', onVisibility);

        let resizeFrame = 0;
        const observer = typeof ResizeObserver === 'function' ? new ResizeObserver(entries => {
            const width = entries[0] && entries[0].contentRect ? entries[0].contentRect.width : refs.root.clientWidth;
            const compact = width > 0 && width < 280;
            if (compact !== state.compact) {
                state.compact = compact;
                refs.root.classList.toggle('is-compact', compact);
            }
            cancelAnimationFrame(resizeFrame);
            resizeFrame = requestAnimationFrame(() => { if (state.pages[state.page] === 'traffic') renderChart(); });
        }) : null;
        if (observer) observer.observe(refs.root);

        setPages([]);
        refs.notice.hidden = true;
        refs.skeleton.hidden = false;
        refresh().then(() => { if (!state.disposed && !document.hidden) schedule(); });

        registerWidgetCleanup(() => {
            state.disposed = true;
            clearTimeout(state.timer);
            clearTimeout(state.copyTimer);
            cancelAnimationFrame(resizeFrame);
            if (state.controller) state.controller.abort();
            document.removeEventListener('visibilitychange', onVisibility);
            if (observer) observer.disconnect();
        });
    }
