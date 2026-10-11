// CYD Creator bottom dock: collapsible, resizable tab strip with the Build log (live build_log lines,
// build history, status, usage, cancel, download) and Problems (layout issues + build diagnostics;
// a click reveals the widget in the designer or opens the file at the line). Other plans add tabs
// through inst.dock.registerTab(id, {labelKey, mount(el) → dispose}).
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    const PREFS_KEY = 'aurago.cydcreator.dock.v1';
    const LOG_MAX_LINES = 5000;
    const TAB_ORDER = { log: 0, monitor: 1, problems: 2 };
    const ACTIVE_BUILD = new Set(['queued', 'running']);

    function readPrefs() {
        try { return JSON.parse(localStorage.getItem(PREFS_KEY) || '{}') || {}; } catch (_) { return {}; }
    }
    function writePrefs(prefs) {
        try { localStorage.setItem(PREFS_KEY, JSON.stringify(prefs)); } catch (_) { /* storage may be blocked */ }
    }

    function sortBuilds(list) {
        return list.slice().sort((a, b) => String(b.created_at || '').localeCompare(String(a.created_at || '')));
    }

    function create(inst) {
        const { el, bag } = P.util;
        const t = inst.t;
        const b = bag();
        const prefs = Object.assign({ h: 220, collapsed: false, tab: 'log' }, readPrefs());
        const tabs = new Map();
        const state = {
            current: '',
            builds: [],
            selected: '',
            lines: [],
            truncated: false,
            follow: true,
            logToken: 0,
            issues: [],
            diagnostics: [],
            buildsLoaded: false
        };

        const tabBar = el('div', { class: 'cc-dock-tabs', role: 'tablist', 'aria-label': t('cyd_creator.dock.label') });
        const toggleBtn = el('button', { type: 'button', class: 'cc-icon-btn cc-dock-toggle', dataset: { ccDockToggle: '' } });
        const handle = el('div', { class: 'cc-dock-handle', role: 'separator', tabindex: '0', 'aria-orientation': 'horizontal', 'aria-label': t('cyd_creator.dock.resize') });
        const body = el('div', { class: 'cc-dock-body' });
        const root = el('section', { class: 'cc-dock', 'aria-label': t('cyd_creator.dock.label') },
            handle,
            el('div', { class: 'cc-dock-bar' }, tabBar, el('span', { class: 'cc-dock-spacer' }), toggleBtn),
            body);

        // ── tabs ──
        function registerTab(id, spec) {
            if (tabs.has(id)) return;
            const s = spec || {};
            const order = Object.prototype.hasOwnProperty.call(TAB_ORDER, id) ? TAB_ORDER[id] : (s.order == null ? 10 : s.order);
            const label = el('span', { class: 'cc-dock-tab-label', text: t(s.labelKey || ('cyd_creator.dock.' + id)) });
            const badge = el('span', { class: 'cc-tag cc-dock-badge', hidden: true });
            const button = el('button', {
                type: 'button', class: 'cc-dock-tab', role: 'tab', id: inst.domId('dock-tab-' + id),
                'aria-selected': 'false', 'aria-controls': inst.domId('dock-panel-' + id), tabindex: '-1',
                dataset: { ccDockTab: id, order: String(order) }
            }, label, badge);
            const panel = el('div', { class: 'cc-dock-panel', role: 'tabpanel', id: inst.domId('dock-panel-' + id), 'aria-labelledby': inst.domId('dock-tab-' + id), hidden: true, dataset: { ccDockPanel: id } });
            const entry = { id, order, button, panel, badge, mount: s.mount, dispose: null, mounted: false };
            tabs.set(id, entry);
            const after = Array.from(tabBar.children).find(node => Number(node.dataset.order) > order);
            tabBar.insertBefore(button, after || null);
            body.appendChild(panel);
            button.addEventListener('click', () => show(id));
            button.addEventListener('keydown', event => {
                if (event.key !== 'ArrowRight' && event.key !== 'ArrowLeft') return;
                const list = Array.from(tabBar.querySelectorAll('[role="tab"]'));
                const index = list.indexOf(button);
                const next = list[(index + (event.key === 'ArrowRight' ? 1 : -1) + list.length) % list.length];
                event.preventDefault();
                show(next.dataset.ccDockTab);
                next.focus();
            });
            if (state.current === '' && id === prefs.tab) show(id);
        }

        function show(id) {
            const entry = tabs.get(id);
            if (!entry) return;
            state.current = id;
            tabs.forEach(other => {
                const active = other.id === id;
                other.button.setAttribute('aria-selected', active ? 'true' : 'false');
                other.button.tabIndex = active ? 0 : -1;
                other.panel.hidden = !active;
            });
            if (!entry.mounted && typeof entry.mount === 'function') {
                entry.mounted = true;
                try {
                    const dispose = entry.mount(entry.panel);
                    if (typeof dispose === 'function') entry.dispose = dispose;
                } catch (err) {
                    console.error('cyd-creator dock tab failed to mount', id, err);
                }
            }
            if (prefs.collapsed) setCollapsed(false);
            prefs.tab = id;
            writePrefs(prefs);
        }

        function setBadge(id, text, tone) {
            const entry = tabs.get(id);
            if (!entry) return;
            entry.badge.hidden = !text;
            entry.badge.textContent = text || '';
            entry.badge.dataset.tone = tone || '';
        }

        // ── collapse and resize ──
        function setCollapsed(collapsed) {
            prefs.collapsed = !!collapsed;
            root.classList.toggle('is-collapsed', prefs.collapsed);
            body.hidden = prefs.collapsed;
            toggleBtn.setAttribute('aria-expanded', prefs.collapsed ? 'false' : 'true');
            toggleBtn.setAttribute('aria-label', t(prefs.collapsed ? 'cyd_creator.dock.expand' : 'cyd_creator.dock.collapse'));
            toggleBtn.title = toggleBtn.getAttribute('aria-label');
            toggleBtn.textContent = prefs.collapsed ? '▴' : '▾';
            writePrefs(prefs);
        }
        function setHeight(h) {
            const max = Math.max(160, Math.round((inst.root.clientHeight || 800) * 0.7));
            prefs.h = Math.round(P.util.clamp(h, 120, max));
            root.style.setProperty('--cc-dock-h', prefs.h + 'px');
            handle.setAttribute('aria-valuenow', String(prefs.h));
        }
        b.on(toggleBtn, 'click', () => setCollapsed(!prefs.collapsed));
        b.on(handle, 'pointerdown', event => {
            if (event.button !== 0) return;
            event.preventDefault();
            const startY = event.clientY;
            const startH = prefs.h;
            handle.setPointerCapture(event.pointerId);
            const move = e => setHeight(startH + (startY - e.clientY));
            const up = () => {
                handle.removeEventListener('pointermove', move);
                handle.removeEventListener('pointerup', up);
                handle.removeEventListener('pointercancel', up);
                writePrefs(prefs);
            };
            handle.addEventListener('pointermove', move);
            handle.addEventListener('pointerup', up);
            handle.addEventListener('pointercancel', up);
        });
        b.on(handle, 'keydown', event => {
            if (event.key !== 'ArrowUp' && event.key !== 'ArrowDown') return;
            event.preventDefault();
            setHeight(prefs.h + (event.key === 'ArrowUp' ? 24 : -24));
            writePrefs(prefs);
        });

        // ── build log ──
        const buildSelect = el('select', { class: 'cc-select cc-log-builds', 'aria-label': t('cyd_creator.dock.build_select'), dataset: { ccLogBuilds: '' } });
        const statusLine = el('span', { class: 'cc-build-status', role: 'status', dataset: { ccBuildStatus: '' } });
        const usageLine = el('span', { class: 'cc-build-usage', dataset: { ccBuildUsage: '' } });
        const cancelBtn = el('button', { type: 'button', class: 'cc-btn cc-btn--small', dataset: { ccLogAction: 'cancel' }, hidden: true, text: t('cyd_creator.build.cancel') });
        const downloadLink = el('a', { class: 'cc-btn cc-btn--small cc-btn--ghost', dataset: { ccLogAction: 'download' }, hidden: true, download: '', text: t('cyd_creator.dock.download_log') });
        const followBox = el('input', { type: 'checkbox', checked: true, dataset: { ccLogFollow: '' } });
        const logPre = el('pre', { class: 'cc-log-lines', role: 'log', 'aria-live': 'off', tabindex: '0', dataset: { ccLog: '' } });
        const logEmpty = el('p', { class: 'cc-empty cc-log-empty', text: t('cyd_creator.dock.no_build') });
        const logPanel = el('div', { class: 'cc-log' },
            el('div', { class: 'cc-log-head' }, buildSelect, statusLine, usageLine, el('span', { class: 'cc-dock-spacer' }), cancelBtn, downloadLink,
                el('label', { class: 'cc-checkline' }, followBox, el('span', { text: t('cyd_creator.dock.follow') }))),
            logEmpty, logPre);

        function selectedBuild() { return state.builds.find(x => x.id === state.selected) || null; }

        function renderBuildOptions() {
            buildSelect.replaceChildren(...state.builds.map((build, index) => el('option', {
                value: build.id,
                selected: build.id === state.selected,
                text: t('cyd_creator.dock.build_option', { n: state.builds.length - index, status: t('cyd_creator.build.status_' + build.status), time: P.format.time(build.created_at) })
            })));
            buildSelect.hidden = state.builds.length === 0;
        }

        function renderBuildStatus() {
            const build = selectedBuild();
            logEmpty.hidden = !!build;
            logPre.hidden = !build;
            if (!build) {
                statusLine.textContent = '';
                usageLine.textContent = '';
                cancelBtn.hidden = true;
                downloadLink.hidden = true;
                setBadge('log', '', '');
                return;
            }
            let text = t('cyd_creator.build.status_' + build.status);
            if (build.error_code) text += ' · ' + P.errors.message(t, { code: build.error_code });
            statusLine.textContent = text;
            statusLine.dataset.status = build.status;
            const u = build.usage || {};
            const flash = P.format.percent(u.flash_used, u.flash_total);
            const ram = P.format.percent(u.ram_used, u.ram_total);
            usageLine.textContent = flash || ram ? t('cyd_creator.build.usage', { flash: flash || '–', ram: ram || '–' }) : '';
            // Read-only viewers may cancel too: the server allows the cancel routes in read-only mode.
            cancelBtn.hidden = !ACTIVE_BUILD.has(build.status);
            downloadLink.hidden = false;
            downloadLink.href = inst.api.buildLogUrl(build.id);
            setBadge('log', ACTIVE_BUILD.has(build.status) ? '•' : '', build.status === 'failed' ? 'danger' : 'accent');
        }

        function lineClass(line) {
            if (/\berror\b|: error:|FAILED/i.test(line)) return 'is-error';
            if (/\bwarning\b|: warning:/i.test(line)) return 'is-warning';
            return '';
        }

        function renderLog() {
            const nodes = state.lines.map(line => el('span', { class: 'cc-log-line ' + lineClass(line), text: line + '\n' }));
            if (state.truncated) nodes.unshift(el('span', { class: 'cc-log-line is-note', text: t('cyd_creator.dock.log_truncated') + '\n' }));
            logPre.replaceChildren(...nodes);
            if (state.follow) logPre.scrollTop = logPre.scrollHeight;
        }

        function pushLines(lines) {
            const list = Array.isArray(lines) ? lines : [];
            if (!list.length) return;
            state.lines.push(...list.map(String));
            const overflow = state.lines.length - LOG_MAX_LINES;
            if (overflow > 0) {
                state.lines.splice(0, overflow);
                state.truncated = true;
                renderLog();
                return;
            }
            const frag = document.createDocumentFragment();
            list.forEach(line => frag.appendChild(el('span', { class: 'cc-log-line ' + lineClass(String(line)), text: String(line) + '\n' })));
            logPre.appendChild(frag);
            if (state.follow) logPre.scrollTop = logPre.scrollHeight;
        }

        async function loadLog(buildId) {
            const token = ++state.logToken;
            state.lines = [];
            state.truncated = false;
            renderLog();
            if (!buildId) return;
            try {
                const text = await inst.api.buildLog(buildId);
                if (token !== state.logToken || b.disposed) return;
                const lines = text.split(/\r?\n/);
                if (lines.length && lines[lines.length - 1] === '') lines.pop();
                state.truncated = lines.length > LOG_MAX_LINES;
                state.lines = lines.slice(-LOG_MAX_LINES);
                renderLog();
            } catch (err) {
                if (token !== state.logToken || b.disposed) return;
                if (err && err.status === 404) return;
                inst.notify('error', P.errors.message(t, err));
            }
        }

        function select(buildId) {
            state.selected = buildId || '';
            renderBuildOptions();
            renderBuildStatus();
            const build = selectedBuild();
            setDiagnostics(build ? (build.diagnostics || []) : []);
            loadLog(state.selected);
        }

        async function loadBuilds() {
            try {
                const list = await inst.api.listBuilds(inst.projectId);
                if (b.disposed) return;
                state.builds = sortBuilds(list);
                state.buildsLoaded = true;
                select(state.builds.length ? state.builds[0].id : '');
            } catch (err) {
                if (!b.disposed) inst.notify('error', P.errors.message(t, err));
            }
        }

        // upsert takes a Build from POST /builds or a build_status event. A new build becomes the
        // selected one while "follow" is on.
        function upsert(build) {
            if (!build || !build.id) return;
            const index = state.builds.findIndex(x => x.id === build.id);
            const isNew = index < 0;
            if (isNew) state.builds.unshift(build);
            else state.builds[index] = build;
            state.builds = sortBuilds(state.builds);
            if (isNew && (state.follow || !state.selected)) {
                state.selected = build.id;
                state.lines = [];
                state.truncated = false;
                state.logToken++;
                renderLog();
            }
            renderBuildOptions();
            if (build.id === state.selected) {
                renderBuildStatus();
                if (!ACTIVE_BUILD.has(build.status)) setDiagnostics(build.diagnostics || []);
            }
        }

        b.on(buildSelect, 'change', () => select(buildSelect.value));
        b.on(followBox, 'change', () => { state.follow = followBox.checked; if (state.follow) logPre.scrollTop = logPre.scrollHeight; });
        b.on(logPre, 'scroll', () => {
            const atEnd = logPre.scrollHeight - logPre.scrollTop - logPre.clientHeight < 24;
            if (!atEnd && state.follow) { state.follow = false; followBox.checked = false; }
        });
        b.on(cancelBtn, 'click', async () => {
            const build = selectedBuild();
            if (!build) return;
            cancelBtn.disabled = true;
            try { await inst.api.cancelBuild(build.id); } catch (err) { inst.notify('error', P.errors.message(t, err)); }
            cancelBtn.disabled = false;
        });

        // ── problems ──
        const problemsHead = el('p', { class: 'cc-problems-summary', dataset: { ccProblemsSummary: '' } });
        const problemsList = el('ul', { class: 'cc-problems-list', dataset: { ccProblems: '' } });
        const problemsEmpty = el('p', { class: 'cc-empty', text: t('cyd_creator.dock.no_problems') });
        const problemsPanel = el('div', { class: 'cc-problems' }, problemsHead, problemsEmpty, problemsList);

        function issueText(issue) {
            const key = 'cyd_creator.issue.' + issue.code;
            return t.has(key) ? t(key, { screen: issue.screen || '', widget: issue.widget || '' }) : String(issue.message || issue.code || '');
        }

        function renderProblems() {
            const rows = [];
            state.issues.forEach(issue => rows.push({
                severity: issue.severity === 'error' ? 'error' : 'warning',
                source: 'layout',
                text: issueText(issue),
                where: [issue.screen, issue.widget].filter(Boolean).join(' › '),
                open: () => inst.reveal({ screen: issue.screen || '', widget: issue.widget || '' }),
                generated: false
            }));
            state.diagnostics.forEach(d => rows.push({
                severity: d.severity === 'error' ? 'error' : (d.severity === 'warning' ? 'warning' : 'note'),
                source: 'build',
                text: String(d.message || ''),
                where: d.file ? d.file + ':' + (d.line || 0) + (d.column ? ':' + d.column : '') : '',
                open: d.file ? () => inst.openFile(d.file, d.line || 1) : null,
                generated: !!d.generated
            }));
            const rank = { error: 0, warning: 1, note: 2 };
            rows.sort((x, y) => rank[x.severity] - rank[y.severity]);
            const errors = rows.filter(r => r.severity === 'error').length;
            const warnings = rows.filter(r => r.severity === 'warning').length;
            problemsHead.textContent = rows.length ? t('cyd_creator.dock.problems_summary', { errors, warnings }) : '';
            problemsEmpty.hidden = rows.length > 0;
            problemsList.replaceChildren(...rows.map(row => {
                const button = el('button', {
                    type: 'button', class: 'cc-problem', disabled: !row.open,
                    dataset: { severity: row.severity, source: row.source, ccProblem: '' }
                },
                el('span', { class: 'cc-problem-icon', 'aria-hidden': 'true', text: row.severity === 'error' ? '✕' : (row.severity === 'warning' ? '!' : 'i') }),
                el('span', { class: 'cc-problem-text', text: row.text }),
                row.generated ? el('span', { class: 'cc-tag', title: t('cyd_creator.dock.generated_hint'), text: t('cyd_creator.dock.generated') }) : null,
                el('span', { class: 'cc-problem-source', text: t('cyd_creator.dock.source_' + row.source) }),
                row.where ? el('span', { class: 'cc-problem-where', text: row.where }) : null);
                if (row.open) button.addEventListener('click', row.open);
                return el('li', {}, button);
            }));
            setBadge('problems', errors + warnings ? String(errors + warnings) : '', errors ? 'danger' : 'warning');
        }

        function setIssues(list) {
            state.issues = Array.isArray(list) ? list.slice() : [];
            renderProblems();
        }
        function setDiagnostics(list) {
            state.diagnostics = Array.isArray(list) ? list.slice() : [];
            renderProblems();
            if (inst.code && typeof inst.code.setProblems === 'function') inst.code.setProblems(state.diagnostics);
        }

        // ── wiring ──
        registerTab('log', { labelKey: 'cyd_creator.dock.log', mount: panel => { panel.appendChild(logPanel); } });
        registerTab('problems', { labelKey: 'cyd_creator.dock.problems', mount: panel => { panel.appendChild(problemsPanel); } });
        if (!state.current) show(tabs.has(prefs.tab) ? prefs.tab : 'log');
        setCollapsed(prefs.collapsed);
        setHeight(prefs.h);
        renderBuildStatus();
        renderProblems();

        b.add(inst.bus.on('build:status', build => upsert(build)));
        b.add(inst.bus.on('sse:build_log', data => {
            if (!data || data.build_id !== state.selected) return;
            pushLines(data.lines);
        }));
        b.add(inst.bus.on('layout:issues', issues => setIssues(issues)));
        // After a reconnect the ephemeral lines of a running build are gone: read the log again.
        let streamSeen = false;
        b.add(inst.bus.on('sse:stream_ready', () => {
            const build = selectedBuild();
            if (streamSeen && build && ACTIVE_BUILD.has(build.status)) loadLog(build.id);
            streamSeen = true;
        }));

        const dock = {
            el: root,
            registerTab,
            show,
            current: () => state.current,
            setCollapsed,
            toggle: () => setCollapsed(!prefs.collapsed),
            problems: { set: setIssues, setDiagnostics, list: () => ({ issues: state.issues.slice(), diagnostics: state.diagnostics.slice() }) },
            builds: { load: loadBuilds, upsert, select, list: () => state.builds.slice(), selected: () => selectedBuild() },
            // trackBuild follows a build the user just started. Its build_status event may have
            // arrived before the POST answer; then upsert does not select it and select() does.
            trackBuild(build) {
                state.follow = true;
                followBox.checked = true;
                upsert(build);
                if (state.selected !== build.id) select(build.id);
                show('log');
            },
            dispose() {
                tabs.forEach(entry => {
                    if (entry.dispose) {
                        try { entry.dispose(); } catch (err) { console.error('cyd-creator dock tab dispose failed', entry.id, err); }
                    }
                });
                tabs.clear();
                b.dispose();
                root.remove();
            }
        };
        return dock;
    }

    P.dock = { create };
})();
