(function () {
    'use strict';

    const instances = new Map();
    const DEFAULT_BUFFER = 10000;
    const MIN_BUFFER = 500;
    const MAX_BUFFER = 50000;
    const DEFAULT_TAIL = 500;
    const ROW_H = 28;
    const OVERSCAN = 14;
    const BUFFER_KEY = 'aurago.desktop.log_viewer.buffer_lines';

    function t(ctx, key, vars) {
        return ctx && typeof ctx.t === 'function' ? ctx.t(key, vars) : key;
    }

    function esc(ctx, value) {
        if (ctx && typeof ctx.esc === 'function') return ctx.esc(value);
        return String(value == null ? '' : value).replace(/[&<>'"]/g, (ch) => ({
            '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;'
        }[ch]));
    }

    function api(ctx) {
        return ctx && typeof ctx.api === 'function' ? ctx.api : async function (url, options) {
            const resp = await fetch(url, Object.assign({ credentials: 'same-origin', cache: 'no-store' }, options || {}));
            const body = (resp.headers.get('content-type') || '').includes('application/json') ? await resp.json() : {};
            if (!resp.ok) throw new Error(body.error || body.message || ('HTTP ' + resp.status));
            return body;
        };
    }

    function readBufferSize() {
        try {
            const n = parseInt(localStorage.getItem(BUFFER_KEY) || '', 10);
            if (Number.isFinite(n)) return Math.max(MIN_BUFFER, Math.min(MAX_BUFFER, n));
        } catch (_) {}
        return DEFAULT_BUFFER;
    }

    function writeBufferSize(n) {
        try { localStorage.setItem(BUFFER_KEY, String(n)); } catch (_) {}
    }

    function formatSize(bytes) {
        const n = Number(bytes || 0);
        if (!Number.isFinite(n) || n < 1024) return Math.max(0, n) + ' B';
        if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KiB';
        return (n / (1024 * 1024)).toFixed(1) + ' MiB';
    }

    function formatTime(value) {
        const raw = String(value || '');
        if (!raw) return '';
        const date = new Date(raw);
        if (Number.isNaN(date.getTime())) return raw.length > 12 ? raw.slice(11, 19) : raw;
        return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
    }

    function levelClass(level) {
        const key = String(level || '').toLowerCase();
        if (key === 'debug' || key === 'info' || key === 'warn' || key === 'error') return key;
        return '';
    }

    function el(tag, className) {
        const node = document.createElement(tag);
        if (className) node.className = className;
        return node;
    }

    function button(label, attrs) {
        const node = el('button');
        node.type = 'button';
        node.textContent = label;
        Object.keys(attrs || {}).forEach((key) => node.setAttribute(key, attrs[key]));
        return node;
    }

    function createState(host, windowId, ctx) {
        const Filters = window.LogViewerFilters;
        const filters = Filters && typeof Filters.create === 'function' ? Filters.create() : null;
        if (filters) filters.restore(windowId);
        return {
            host,
            windowId,
            ctx,
            disposed: false,
            filters,
            files: [],
            logDir: '',
            file: '',
            records: [],
            selectedIndex: -1,
            eofOffset: 0,
            source: null,
            pollTimer: null,
            paused: false,
            pausedQueue: [],
            follow: true,
            wrap: false,
            wholeFile: false,
            pending: 0,
            bufferLimit: readBufferSize(),
            fontSize: 12,
            detailOpen: false,
            keyHandler: null,
            renderQueued: false
        };
    }

    function shellMarkup(state) {
        const ctx = state.ctx;
        return '' +
            '<div class="vd-logviewer is-detail-closed">' +
                '<header class="vd-logviewer-toolbar">' +
                    '<div><span class="vd-logviewer-kicker">' + esc(ctx, t(ctx, 'desktop.app_log_viewer')) + '</span></div>' +
                    '<input class="vd-logviewer-search" type="search" data-role="search" placeholder="' + esc(ctx, t(ctx, 'desktop.log_viewer_search')) + '" aria-label="' + esc(ctx, t(ctx, 'desktop.log_viewer_search')) + '">' +
                    '<label class="vd-logviewer-chip" title="' + esc(ctx, t(ctx, 'desktop.log_viewer_regex_help')) + '">' +
                        '<input type="checkbox" data-role="regex"> Rx' +
                    '</label>' +
                    '<label class="vd-logviewer-chip">' +
                        '<input type="checkbox" data-role="whole"> ' + esc(ctx, t(ctx, 'desktop.log_viewer_whole_file')) +
                    '</label>' +
                    '<div class="vd-logviewer-levels" data-role="levels"></div>' +
                    '<span class="vd-logviewer-live" data-role="live">' + esc(ctx, t(ctx, 'desktop.log_viewer_live')) + '</span>' +
                    '<button type="button" data-role="pause">' + esc(ctx, t(ctx, 'desktop.log_viewer_pause')) + '</button>' +
                    '<button type="button" data-role="clear">' + esc(ctx, t(ctx, 'desktop.log_viewer_clear')) + '</button>' +
                    '<button type="button" data-role="download">' + esc(ctx, t(ctx, 'desktop.log_viewer_download')) + '</button>' +
                    '<button type="button" data-role="autoscroll" aria-pressed="true">' + esc(ctx, t(ctx, 'desktop.log_viewer_autoscroll')) + '</button>' +
                    '<button type="button" data-role="wrap" aria-pressed="false">' + esc(ctx, t(ctx, 'desktop.log_viewer_wrap')) + '</button>' +
                    '<label class="vd-logviewer-chip">' + esc(ctx, t(ctx, 'desktop.log_viewer_font_size')) +
                        '<input class="vd-logviewer-font" type="range" min="11" max="18" value="12" data-role="font">' +
                    '</label>' +
                '</header>' +
                '<div class="vd-logviewer-body">' +
                    '<aside class="vd-logviewer-sidebar">' +
                        '<div class="vd-logviewer-pane-title">' + esc(ctx, t(ctx, 'desktop.log_viewer_files')) + '</div>' +
                        '<div class="vd-logviewer-files" data-role="files"></div>' +
                    '</aside>' +
                    '<section class="vd-logviewer-list">' +
                        '<div class="vd-logviewer-scroller" data-role="scroller">' +
                            '<div data-role="spacer-top"></div>' +
                            '<div data-role="rows"></div>' +
                            '<div data-role="spacer-bottom"></div>' +
                        '</div>' +
                        '<button type="button" class="vd-logviewer-jump" data-role="jump" hidden>' + esc(ctx, t(ctx, 'desktop.log_viewer_jump_latest')) + '</button>' +
                    '</section>' +
                    '<aside class="vd-logviewer-detail" data-role="detail">' +
                        '<div class="vd-logviewer-pane-title">' + esc(ctx, t(ctx, 'desktop.log_viewer_details')) + '</div>' +
                        '<div class="vd-logviewer-detail-body" data-role="detail-body"></div>' +
                        '<div class="vd-logviewer-detail-actions">' +
                            '<button type="button" data-role="copy">' + esc(ctx, t(ctx, 'desktop.log_viewer_copy_raw')) + '</button>' +
                        '</div>' +
                    '</aside>' +
                '</div>' +
            '</div>';
    }

    function root(state) {
        return state.host.querySelector('.vd-logviewer');
    }

    function q(state, role) {
        return state.host.querySelector('[data-role="' + role + '"]');
    }

    function setLiveLabel(state) {
        const live = q(state, 'live');
        const pause = q(state, 'pause');
        if (live) live.textContent = t(state.ctx, state.paused ? 'desktop.log_viewer_paused' : 'desktop.log_viewer_live');
        if (pause) pause.textContent = t(state.ctx, state.paused ? 'desktop.log_viewer_resume' : 'desktop.log_viewer_pause');
        const node = root(state);
        if (node) node.classList.toggle('is-paused', state.paused);
    }

    function renderLevels(state) {
        const host = q(state, 'levels');
        if (!host || !state.filters) return;
        host.textContent = '';
        const counts = { DEBUG: 0, INFO: 0, WARN: 0, ERROR: 0 };
        state.records.forEach((record) => {
            const level = String(record.level || 'INFO').toUpperCase();
            if (counts[level] != null) counts[level] += 1;
        });
        (state.filters.LEVELS || ['DEBUG', 'INFO', 'WARN', 'ERROR']).forEach((level) => {
            const chip = button('', { 'data-level': level });
            chip.className = 'vd-logviewer-chip' + (state.filters.levels[level] !== false ? ' is-on' : '');
            const key = 'desktop.log_viewer_level_' + level.toLowerCase();
            chip.appendChild(document.createTextNode(t(state.ctx, key)));
            const count = el('span', 'vd-logviewer-chip-count');
            count.textContent = String(counts[level] || 0);
            chip.appendChild(count);
            chip.addEventListener('click', () => {
                state.filters.toggleLevel(level);
                state.filters.persist(state.windowId);
                renderLevels(state);
                queueRender(state);
            });
            host.appendChild(chip);
        });
    }

    function renderFiles(state) {
        const host = q(state, 'files');
        if (!host) return;
        host.textContent = '';
        if (!state.files.length) {
            const empty = el('div', 'vd-logviewer-empty');
            empty.textContent = t(state.ctx, 'desktop.log_viewer_no_files', { dir: state.logDir || '' });
            host.appendChild(empty);
            return;
        }
        state.files.forEach((file) => {
            const btn = button('', { 'data-file': file.name });
            btn.className = 'vd-logviewer-file' + (file.name === state.file ? ' is-active' : '') + (file.name === state.file && !state.paused ? ' is-live' : '');
            const name = el('div', 'vd-logviewer-file-name');
            name.textContent = file.name;
            const meta = el('div', 'vd-logviewer-file-meta');
            meta.textContent = formatSize(file.size);
            btn.appendChild(name);
            btn.appendChild(meta);
            btn.addEventListener('click', () => selectFile(state, file.name));
            host.appendChild(btn);
        });
    }

    function visibleRecords(state) {
        return state.filters ? state.filters.apply(state.records) : state.records.slice();
    }

    function queueRender(state) {
        if (state.renderQueued || state.disposed) return;
        state.renderQueued = true;
        requestAnimationFrame(() => {
            state.renderQueued = false;
            if (!state.disposed) renderRows(state);
        });
    }

    function renderRows(state) {
        const scroller = q(state, 'scroller');
        const rows = q(state, 'rows');
        const top = q(state, 'spacer-top');
        const bottom = q(state, 'spacer-bottom');
        if (!scroller || !rows) return;
        const list = visibleRecords(state);
        if (!list.length) {
            rows.textContent = '';
            if (top) top.style.height = '0px';
            if (bottom) bottom.style.height = '0px';
            const empty = el('div', 'vd-logviewer-empty');
            empty.textContent = t(state.ctx, state.records.length ? 'desktop.log_viewer_empty' : 'desktop.log_viewer_select_file');
            rows.appendChild(empty);
            updateJump(state);
            return;
        }
        const rowH = state.wrap ? 48 : ROW_H;
        const scrollTop = scroller.scrollTop;
        const viewH = scroller.clientHeight || 320;
        let start = Math.max(0, Math.floor(scrollTop / rowH) - OVERSCAN);
        let end = Math.min(list.length, Math.ceil((scrollTop + viewH) / rowH) + OVERSCAN);
        if (state.wrap && list.length < 400) {
            start = 0;
            end = list.length;
        }
        if (top) top.style.height = (start * rowH) + 'px';
        if (bottom) bottom.style.height = Math.max(0, (list.length - end) * rowH) + 'px';
        rows.textContent = '';
        for (let i = start; i < end; i += 1) {
            rows.appendChild(renderRow(state, list[i], i));
        }
        updateJump(state);
    }

    function renderRow(state, record, index) {
        const row = el('button', 'vd-logviewer-row');
        row.type = 'button';
        if (index === state.selectedIndex) row.classList.add('is-selected');
        const no = el('span', 'vd-logviewer-row-no');
        no.textContent = String(record.line_no || index + 1);
        const time = el('span', 'vd-logviewer-row-time');
        time.textContent = formatTime(record.time);
        const msg = el('span', 'vd-logviewer-row-msg');
        const badge = el('span', 'vd-logviewer-level' + (levelClass(record.level) ? ' is-' + levelClass(record.level) : ''));
        badge.textContent = record.level || '—';
        msg.appendChild(badge);
        msg.appendChild(document.createTextNode(record.msg || record.raw || ''));
        row.appendChild(no);
        row.appendChild(time);
        row.appendChild(msg);
        row.addEventListener('click', () => selectRow(state, index, true));
        return row;
    }

    function updateJump(state) {
        const jump = q(state, 'jump');
        if (!jump) return;
        const hidden = state.follow || state.pending <= 0;
        jump.hidden = hidden;
        if (!hidden) {
            jump.textContent = t(state.ctx, 'desktop.log_viewer_jump_latest') + ' · ' + t(state.ctx, 'desktop.log_viewer_new_lines', { count: state.pending });
        }
    }

    function selectRow(state, index, openDetail) {
        state.selectedIndex = index;
        if (openDetail) {
            state.detailOpen = true;
            const node = root(state);
            if (node) node.classList.remove('is-detail-closed');
        }
        renderDetail(state);
        queueRender(state);
    }

    function closeDetail(state) {
        state.detailOpen = false;
        const node = root(state);
        if (node) node.classList.add('is-detail-closed');
    }

    function renderDetail(state) {
        const body = q(state, 'detail-body');
        if (!body) return;
        body.textContent = '';
        const list = visibleRecords(state);
        const record = list[state.selectedIndex];
        if (!record) {
            const empty = el('div', 'vd-logviewer-empty');
            empty.textContent = t(state.ctx, 'desktop.log_viewer_select_file');
            body.appendChild(empty);
            return;
        }
        const dl = el('dl', 'vd-logviewer-kv');
        addKV(dl, t(state.ctx, 'desktop.log_viewer_line', { n: record.line_no || 0 }), String(record.line_no || ''));
        addKV(dl, t(state.ctx, 'desktop.log_viewer_levels'), record.level || '—');
        addKV(dl, t(state.ctx, 'desktop.log_viewer_files'), record.file || state.file);
        if (record.time) addKV(dl, 'time', record.time);
        if (record.msg) addKV(dl, 'msg', record.msg);
        body.appendChild(dl);
        const attrs = record.attrs || {};
        const keys = Object.keys(attrs);
        if (keys.length) {
            const title = el('div', 'vd-logviewer-pane-title');
            title.textContent = t(state.ctx, 'desktop.log_viewer_attr');
            body.appendChild(title);
            const adl = el('dl', 'vd-logviewer-kv');
            keys.sort().forEach((key) => addKV(adl, key, attrs[key]));
            body.appendChild(adl);
        }
        const raw = el('pre', 'vd-logviewer-raw');
        raw.textContent = record.raw || '';
        body.appendChild(raw);
    }

    function addKV(dl, key, value) {
        const dt = el('dt');
        dt.textContent = key;
        const dd = el('dd');
        dd.textContent = value == null ? '' : String(value);
        dl.appendChild(dt);
        dl.appendChild(dd);
    }

    function pushRecords(state, incoming, fromLive) {
        const items = Array.isArray(incoming) ? incoming : [incoming];
        if (!items.length) return;
        if (state.paused && fromLive) {
            state.pausedQueue = state.pausedQueue.concat(items);
            if (state.pausedQueue.length > state.bufferLimit) {
                state.pausedQueue = state.pausedQueue.slice(-state.bufferLimit);
            }
            return;
        }
        state.records = state.records.concat(items);
        if (state.records.length > state.bufferLimit) {
            state.records = state.records.slice(-state.bufferLimit);
        }
        if (fromLive && !state.follow) state.pending += items.length;
        renderLevels(state);
        queueRender(state);
        if (state.follow) scrollToLatest(state);
    }

    function scrollToLatest(state) {
        const scroller = q(state, 'scroller');
        if (!scroller) return;
        state.follow = true;
        state.pending = 0;
        requestAnimationFrame(() => {
            scroller.scrollTop = scroller.scrollHeight;
            updateJump(state);
        });
    }

    function closeStream(state) {
        if (state.source) {
            try { state.source.close(); } catch (_) {}
            state.source = null;
        }
        if (state.pollTimer) {
            clearInterval(state.pollTimer);
            clearTimeout(state.pollTimer);
            state.pollTimer = null;
        }
    }

    function startStream(state) {
        closeStream(state);
        if (!state.file) return;
        const url = '/api/desktop/logs/stream?file=' + encodeURIComponent(state.file) + '&since=' + encodeURIComponent(String(state.eofOffset || 0));
        if (typeof EventSource !== 'function') {
            state.pollTimer = setInterval(() => pollTail(state), 2000);
            return;
        }
        const source = new EventSource(url);
        state.source = source;
        source.onmessage = (event) => {
            if (state.disposed || source !== state.source) return;
            handleStreamPayload(state, event.data);
        };
        source.onerror = () => {
            if (state.disposed || source !== state.source) return;
            showBanner(state, t(state.ctx, 'desktop.log_viewer_stream_error'));
            closeStream(state);
            if (!state.disposed) {
                state.pollTimer = setTimeout(() => {
                    if (!state.disposed) startStream(state);
                }, 1200);
            }
        };
    }

    function handleStreamPayload(state, raw) {
        let data;
        try { data = JSON.parse(raw); } catch (_) { return; }
        const type = data && data.type;
        const payload = data && data.payload;
        if (type === 'log_line') {
            if (payload && payload.raw != null) {
                if (payload.offset != null) state.eofOffset = Number(payload.offset) || state.eofOffset;
                pushRecords(state, payload, true);
            }
            return;
        }
        if (type === 'log_heartbeat' && payload && payload.offset != null) {
            state.eofOffset = Number(payload.offset) || state.eofOffset;
            return;
        }
        if (type === 'log_truncated') {
            showBanner(state, t(state.ctx, 'desktop.log_viewer_truncated'));
            state.records = [];
            state.eofOffset = 0;
            selectFile(state, state.file);
            return;
        }
        if (type === 'log_error' && payload && payload.error) {
            showBanner(state, String(payload.error));
        }
    }

    async function pollTail(state) {
        if (state.disposed || !state.file) return;
        try {
            const data = await api(state.ctx)('/api/desktop/logs/tail?file=' + encodeURIComponent(state.file) + '&lines=200&offset=' + encodeURIComponent(String(state.eofOffset || 0)));
            if (data && data.truncated) {
                showBanner(state, t(state.ctx, 'desktop.log_viewer_truncated'));
                state.records = [];
            }
            if (data && Array.isArray(data.lines) && data.lines.length) pushRecords(state, data.lines, true);
            if (data && data.eof_offset != null) state.eofOffset = data.eof_offset;
        } catch (_) {}
    }

    function showBanner(state, text) {
        const rows = q(state, 'rows');
        if (!rows || !text) return;
        let banner = rows.querySelector('.vd-logviewer-banner');
        if (!banner) {
            banner = el('div', 'vd-logviewer-banner');
            rows.insertBefore(banner, rows.firstChild);
        }
        banner.textContent = text;
    }

    async function loadFiles(state) {
        const data = await api(state.ctx)('/api/desktop/logs/files');
        if (state.disposed) return;
        state.files = (data && data.files) || [];
        state.logDir = (data && data.log_dir) || '';
        renderFiles(state);
        const preferred = state.files.find((file) => file.name === 'aurago.log') || state.files[0];
        if (preferred) await selectFile(state, preferred.name);
    }

    async function selectFile(state, name) {
        state.file = name;
        state.records = [];
        state.pausedQueue = [];
        state.selectedIndex = -1;
        state.pending = 0;
        state.follow = true;
        renderFiles(state);
        queueRender(state);
        const rows = q(state, 'rows');
        if (rows) {
            rows.textContent = '';
            const loading = el('div', 'vd-logviewer-empty');
            loading.textContent = t(state.ctx, 'desktop.log_viewer_loading');
            rows.appendChild(loading);
        }
        const data = await api(state.ctx)('/api/desktop/logs/tail?file=' + encodeURIComponent(name) + '&lines=' + DEFAULT_TAIL);
        if (state.disposed || state.file !== name) return;
        state.records = (data && data.lines) || [];
        state.eofOffset = data && data.eof_offset != null ? data.eof_offset : 0;
        renderLevels(state);
        queueRender(state);
        scrollToLatest(state);
        startStream(state);
    }

    async function searchWholeFile(state) {
        if (!state.file || !state.filters) return;
        const query = state.filters.query;
        const levels = Object.keys(state.filters.levels).filter((level) => state.filters.levels[level] !== false).join(',');
        const params = new URLSearchParams({ file: state.file, q: query, levels: levels, limit: '400' });
        const data = await api(state.ctx)('/api/desktop/logs/search?' + params.toString());
        const matches = ((data && data.matches) || []).map((item) => item.record || item);
        state.records = matches;
        renderLevels(state);
        queueRender(state);
    }

    function wire(state) {
        const search = q(state, 'search');
        const regex = q(state, 'regex');
        const whole = q(state, 'whole');
        const pause = q(state, 'pause');
        const clear = q(state, 'clear');
        const download = q(state, 'download');
        const autoscroll = q(state, 'autoscroll');
        const wrap = q(state, 'wrap');
        const font = q(state, 'font');
        const jump = q(state, 'jump');
        const copy = q(state, 'copy');
        const scroller = q(state, 'scroller');
        if (state.filters) {
            if (search) search.value = state.filters.query;
            if (regex) regex.checked = !!state.filters.regex;
        }
        if (state.ctx && state.ctx.readonly && download) download.hidden = true;
        if (search) {
            search.addEventListener('input', () => {
                if (!state.filters) return;
                state.filters.setQuery(search.value);
                state.filters.persist(state.windowId);
                if (state.wholeFile) searchWholeFile(state).catch(() => {});
                else queueRender(state);
            });
        }
        if (regex) {
            regex.addEventListener('change', () => {
                if (!state.filters) return;
                state.filters.setRegex(regex.checked);
                state.filters.persist(state.windowId);
                queueRender(state);
            });
        }
        if (whole) {
            whole.addEventListener('change', () => {
                state.wholeFile = !!whole.checked;
                if (state.wholeFile) searchWholeFile(state).catch(() => {});
                else selectFile(state, state.file).catch(() => {});
            });
        }
        if (pause) {
            pause.addEventListener('click', () => {
                state.paused = !state.paused;
                if (!state.paused && state.pausedQueue.length) {
                    pushRecords(state, state.pausedQueue, false);
                    state.pausedQueue = [];
                }
                setLiveLabel(state);
            });
        }
        if (clear) {
            clear.addEventListener('click', () => {
                state.records = [];
                state.pausedQueue = [];
                state.pending = 0;
                state.selectedIndex = -1;
                renderLevels(state);
                queueRender(state);
            });
        }
        if (download) {
            download.addEventListener('click', () => {
                if (!state.file || (state.ctx && state.ctx.readonly)) return;
                window.open('/api/desktop/logs/download?file=' + encodeURIComponent(state.file), '_blank', 'noopener');
            });
        }
        if (autoscroll) {
            autoscroll.addEventListener('click', () => {
                state.follow = !state.follow;
                autoscroll.setAttribute('aria-pressed', state.follow ? 'true' : 'false');
                if (state.follow) scrollToLatest(state);
            });
        }
        if (wrap) {
            wrap.addEventListener('click', () => {
                state.wrap = !state.wrap;
                wrap.setAttribute('aria-pressed', state.wrap ? 'true' : 'false');
                const node = root(state);
                if (node) node.classList.toggle('is-wrap', state.wrap);
                queueRender(state);
            });
        }
        if (font) {
            font.addEventListener('input', () => {
                state.fontSize = Number(font.value) || 12;
                const node = root(state);
                if (node) node.style.setProperty('--lv-font', state.fontSize + 'px');
            });
        }
        if (jump) jump.addEventListener('click', () => scrollToLatest(state));
        if (copy) {
            copy.addEventListener('click', () => {
                const list = visibleRecords(state);
                const record = list[state.selectedIndex];
                if (!record || !navigator.clipboard) return;
                navigator.clipboard.writeText(record.raw || '').catch(() => {});
            });
        }
        if (scroller) {
            scroller.addEventListener('scroll', () => {
                const nearBottom = scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight < 48;
                if (!nearBottom) state.follow = false;
                else {
                    state.follow = true;
                    state.pending = 0;
                }
                if (autoscroll) autoscroll.setAttribute('aria-pressed', state.follow ? 'true' : 'false');
                queueRender(state);
            });
        }
        state.keyHandler = (event) => onKey(state, event);
        document.addEventListener('keydown', state.keyHandler);
        setLiveLabel(state);
        renderLevels(state);
    }

    function onKey(state, event) {
        if (state.disposed) return;
        const node = root(state);
        if (!node || !node.isConnected) return;
        const target = event.target;
        const typing = target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA');
        if (event.key === 'Escape') {
            if (typing) {
                target.blur();
                return;
            }
            closeDetail(state);
            return;
        }
        if (typing) return;
        if (event.key === '/' || event.key === 'f') {
            event.preventDefault();
            const search = q(state, 'search');
            if (search) search.focus();
            return;
        }
        if (event.key === 'p') {
            q(state, 'pause') && q(state, 'pause').click();
            return;
        }
        if (event.key === 'c') {
            q(state, 'clear') && q(state, 'clear').click();
            return;
        }
        if (event.key === 'g') {
            const scroller = q(state, 'scroller');
            if (scroller) scroller.scrollTop = 0;
            return;
        }
        if (event.key === 'G') {
            scrollToLatest(state);
            return;
        }
        const list = visibleRecords(state);
        if (!list.length) return;
        if (event.key === 'j' || event.key === 'ArrowDown') {
            event.preventDefault();
            selectRow(state, Math.min(list.length - 1, state.selectedIndex + 1), state.detailOpen);
            return;
        }
        if (event.key === 'k' || event.key === 'ArrowUp') {
            event.preventDefault();
            selectRow(state, Math.max(0, (state.selectedIndex < 0 ? 0 : state.selectedIndex - 1)), state.detailOpen);
            return;
        }
        if (event.key === 'Enter') selectRow(state, Math.max(0, state.selectedIndex), true);
    }

    function render(host, windowId, context) {
        if (!host) return;
        dispose(windowId);
        const ctx = context || {};
        const state = createState(host, windowId, ctx);
        instances.set(windowId, state);
        host.innerHTML = shellMarkup(state);
        wire(state);
        loadFiles(state).catch((err) => {
            showBanner(state, err && err.message ? err.message : t(ctx, 'desktop.log_viewer_stream_error'));
        });
    }

    function dispose(windowId) {
        const state = instances.get(windowId);
        if (!state) return;
        state.disposed = true;
        closeStream(state);
        if (state.keyHandler) document.removeEventListener('keydown', state.keyHandler);
        state.records = [];
        state.pausedQueue = [];
        instances.delete(windowId);
    }

    window.LogViewerApp = { render, dispose };
}());
