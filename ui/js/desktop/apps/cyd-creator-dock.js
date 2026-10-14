// CYD Creator bottom dock: collapsible, resizable tab strip with the Build log (live build_log lines,
// build history, status, usage, cancel, download) and Problems (layout issues + build diagnostics;
// a click reveals the widget in the designer or opens the file at the line). Other plans add tabs
// through inst.dock.registerTab(id, {labelKey, mount(el) → dispose}).
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    const PREFS_KEY = 'aurago.cydcreator.dock.v1';
    const LOG_MAX_LINES = 5000;
    const MIN_HEIGHT = 120;
    const DEFAULT_HEIGHT = 220;
    // Until the user resizes the dock, it takes at most this share of the window in Design mode with
    // the designer (plan 4b), so a small window keeps the canvas.
    const DESIGN_SHARE = 0.3;
    const TAB_ORDER = { log: 0, monitor: 1, problems: 2 };
    const ACTIVE_BUILD = new Set(['queued', 'running']);

    function readPrefs() {
        try { return JSON.parse(localStorage.getItem(PREFS_KEY) || '{}') || {}; } catch (_) { return {}; }
    }
    function writePrefs(prefs) {
        try { localStorage.setItem(PREFS_KEY, JSON.stringify(prefs)); } catch (_) { /* storage may be blocked */ }
    }

    // Builds sort newest first like GET /builds (created_at DESC, id DESC). created_at is RFC 3339
    // UTC with milliseconds; Date.parse also copes with other precisions or offsets.
    function stamp(build) {
        const ms = Date.parse(build && build.created_at);
        return Number.isFinite(ms) ? ms : 0;
    }
    function sortBuilds(list) {
        return list.slice().sort((a, b) => (stamp(b) - stamp(a)) || String(b.id).localeCompare(String(a.id)));
    }
    // progress orders a build's states: queued < running < finished. A copy with less progress than
    // the known one is stale (a late POST answer, an older snapshot).
    function progress(build) {
        return build.status === 'queued' ? 0 : (build.status === 'running' ? 1 : 2);
    }
    // pruned: an unknown finished build older than the newest known one was deleted by the retention
    // (5 builds per project) while its events (5000 per project) still replay to a new window.
    function pruned(build, known) {
        if (ACTIVE_BUILD.has(build.status) || !known.length) return false;
        return stamp(build) < known.reduce((max, x) => Math.max(max, stamp(x)), 0);
    }

    function create(inst) {
        const { el, bag } = P.util;
        const t = inst.t;
        const b = bag();
        const saved = readPrefs();
        const prefs = Object.assign({ h: DEFAULT_HEIGHT, collapsed: false, tab: 'log' }, saved);
        // custom: the user chose the height (drag or keys, here or in an earlier window); prefs saved
        // before the flag existed count when they hold another height than the default.
        let custom = prefs.custom === true || (Number.isFinite(saved.h) && saved.h !== DEFAULT_HEIGHT);
        let shownHeight = prefs.h;
        // Restoring the saved tab while the dock is built must not expand a collapsed dock.
        const startCollapsed = !!prefs.collapsed;
        const tabs = new Map();
        const state = {
            current: '',
            // fallback: the shown tab stands in for a saved tab that is not registered (yet).
            fallback: false,
            builds: [],
            // early: finished builds that events reported before GET /builds answered (mergeBuilds).
            early: new Map(),
            // seen: builds that events reported while the newest load (loadToken) runs; null otherwise.
            seen: null,
            loadToken: 0,
            selected: '',
            lines: [],
            truncated: false,
            follow: true,
            logToken: 0,
            issues: [],
            diagnostics: [],
            buildsLoaded: false
        };

        const tabBar = el('div', { class: 'cydc-dock-tabs', role: 'tablist', 'aria-label': t('cyd_creator.dock.label') });
        const toggleBtn = el('button', { type: 'button', class: 'cydc-icon-btn cydc-dock-toggle', dataset: { ccDockToggle: '' } });
        const handle = el('div', { class: 'cydc-dock-handle', role: 'separator', tabindex: '0', 'aria-orientation': 'horizontal', 'aria-label': t('cyd_creator.dock.resize') });
        const body = el('div', { class: 'cydc-dock-body' });
        const root = el('section', { class: 'cydc-dock', 'aria-label': t('cyd_creator.dock.label') },
            handle,
            el('div', { class: 'cydc-dock-bar' }, tabBar, el('span', { class: 'cydc-dock-spacer' }), toggleBtn),
            body);

        // ── tabs ──
        function registerTab(id, spec) {
            if (tabs.has(id)) return;
            const s = spec || {};
            const order = Object.prototype.hasOwnProperty.call(TAB_ORDER, id) ? TAB_ORDER[id] : (s.order == null ? 10 : s.order);
            const label = el('span', { class: 'cydc-dock-tab-label', text: t(s.labelKey || ('cyd_creator.dock.' + id)) });
            const badge = el('span', { class: 'cydc-tag cydc-dock-badge', hidden: true });
            const button = el('button', {
                type: 'button', class: 'cydc-dock-tab', role: 'tab', id: inst.domId('dock-tab-' + id),
                'aria-selected': 'false', 'aria-controls': inst.domId('dock-panel-' + id), tabindex: '-1',
                dataset: { ccDockTab: id, order: String(order) }
            }, label, badge);
            const panel = el('div', { class: 'cydc-dock-panel', role: 'tabpanel', id: inst.domId('dock-panel-' + id), 'aria-labelledby': inst.domId('dock-tab-' + id), hidden: true, dataset: { ccDockPanel: id } });
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
            if (id === prefs.tab && (state.current === '' || state.fallback)) show(id, true);
        }

        // show(id) is a user's choice: it expands the dock and remembers the tab. show(id, true)
        // restores a tab while the dock is built or when a saved tab registers later: it neither
        // expands the dock nor overwrites the saved tab (a fallback to the log keeps plan 5's monitor).
        function show(id, restore) {
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
            if (restore) {
                state.fallback = id !== prefs.tab;
            } else {
                state.fallback = false;
                if (prefs.collapsed) setCollapsed(false);
                prefs.tab = id;
                writePrefs(prefs);
            }
            followTail();
        }

        // decorative badges (the running dot) are hidden from screen readers; counts are read.
        function setBadge(id, text, tone, decorative) {
            const entry = tabs.get(id);
            if (!entry) return;
            entry.badge.hidden = !text;
            entry.badge.textContent = text || '';
            entry.badge.dataset.tone = tone || '';
            if (decorative) entry.badge.setAttribute('aria-hidden', 'true');
            else entry.badge.removeAttribute('aria-hidden');
        }

        // followTail keeps the log at its end while "follow" is on. A hidden <pre> cannot scroll, so
        // showing the log tab or expanding the dock catches up.
        function followTail() {
            if (state.follow && state.current === 'log' && !prefs.collapsed) logPre.scrollTop = logPre.scrollHeight;
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
            followTail();
        }
        // setHeight shows h within MIN_HEIGHT and 70 % of the window; byUser stores it as the user's
        // height. The separator states the shown height.
        function setHeight(h, byUser) {
            const max = Math.max(160, Math.round((inst.root.clientHeight || 800) * 0.7));
            shownHeight = Math.round(P.util.clamp(h, MIN_HEIGHT, max));
            if (byUser) {
                custom = true;
                prefs.custom = true;
                prefs.h = shownHeight;
            }
            root.style.setProperty('--cc-dock-h', shownHeight + 'px');
            handle.setAttribute('aria-valuemin', String(MIN_HEIGHT));
            handle.setAttribute('aria-valuemax', String(max));
            handle.setAttribute('aria-valuenow', String(shownHeight));
        }
        // startHeight: the user's height; else the default, in Design mode with the designer at most
        // DESIGN_SHARE of the window. Re-applied on mode changes until the user resizes the dock.
        function startHeight() {
            if (custom || inst.mode === 'code' || !inst.designer) return prefs.h;
            return Math.min(prefs.h, Math.max(MIN_HEIGHT, Math.round((inst.root.clientHeight || 800) * DESIGN_SHARE)));
        }
        b.on(toggleBtn, 'click', () => setCollapsed(!prefs.collapsed));
        b.on(handle, 'pointerdown', event => {
            if (event.button !== 0) return;
            event.preventDefault();
            const startY = event.clientY;
            const startH = shownHeight;
            handle.setPointerCapture(event.pointerId);
            const move = e => setHeight(startH + (startY - e.clientY), true);
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
            setHeight(shownHeight + (event.key === 'ArrowUp' ? 24 : -24), true);
            writePrefs(prefs);
        });

        // ── build log ──
        const buildSelect = el('select', { class: 'cydc-select cydc-log-builds', 'aria-label': t('cyd_creator.dock.build_select'), dataset: { ccLogBuilds: '' } });
        const statusLine = el('span', { class: 'cydc-build-status', role: 'status', dataset: { ccBuildStatus: '' } });
        const usageLine = el('span', { class: 'cydc-build-usage', dataset: { ccBuildUsage: '' } });
        const cancelBtn = el('button', { type: 'button', class: 'cydc-btn cydc-btn--small', dataset: { ccLogAction: 'cancel' }, hidden: true, text: t('cyd_creator.build.cancel') });
        const downloadLink = el('a', { class: 'cydc-btn cydc-btn--small cydc-btn--ghost', dataset: { ccLogAction: 'download' }, hidden: true, download: '', text: t('cyd_creator.dock.download_log') });
        const followBox = el('input', { type: 'checkbox', checked: true, dataset: { ccLogFollow: '' } });
        const logPre = el('pre', { class: 'cydc-log-lines', role: 'log', 'aria-live': 'off', tabindex: '0', dataset: { ccLog: '' } });
        const logEmpty = el('p', { class: 'cydc-empty cydc-log-empty', text: t('cyd_creator.dock.no_build') });
        const logPanel = el('div', { class: 'cydc-log' },
            el('div', { class: 'cydc-log-head' }, buildSelect, statusLine, usageLine, el('span', { class: 'cydc-dock-spacer' }), cancelBtn, downloadLink,
                el('label', { class: 'cydc-checkline' }, followBox, el('span', { text: t('cyd_creator.dock.follow') }))),
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
            downloadLink.download = build.id + '-build.log';
            setBadge('log', ACTIVE_BUILD.has(build.status) ? '•' : '', build.status === 'failed' ? 'danger' : 'accent', true);
        }

        function lineClass(line) {
            if (/\berror\b|: error:|FAILED/i.test(line)) return 'is-error';
            if (/\bwarning\b|: warning:/i.test(line)) return 'is-warning';
            return '';
        }

        function lineNode(line) {
            return el('span', { class: 'cydc-log-line ' + lineClass(line), text: line + '\n' });
        }
        function noteNode() {
            return el('span', { class: 'cydc-log-line is-note', text: t('cyd_creator.dock.log_truncated') + '\n' });
        }

        // While state.truncated is set, the truncation note is the log's first node.
        function renderLog() {
            const nodes = state.lines.map(lineNode);
            if (state.truncated) nodes.unshift(noteNode());
            logPre.replaceChildren(...nodes);
            if (state.follow) logPre.scrollTop = logPre.scrollHeight;
        }

        // pushLines appends live lines. Past LOG_MAX_LINES the oldest line nodes are removed (the
        // note stays first) instead of rebuilding the whole ring.
        function pushLines(lines) {
            const list = (Array.isArray(lines) ? lines : []).map(String);
            if (!list.length) return;
            if (list.length >= LOG_MAX_LINES) {
                state.lines = list.slice(-LOG_MAX_LINES);
                state.truncated = true;
                renderLog();
                return;
            }
            state.lines.push(...list);
            const frag = document.createDocumentFragment();
            list.forEach(line => frag.appendChild(lineNode(line)));
            logPre.appendChild(frag);
            const overflow = state.lines.length - LOG_MAX_LINES;
            if (overflow > 0) {
                state.lines.splice(0, overflow);
                if (!state.truncated) {
                    state.truncated = true;
                    logPre.insertBefore(noteNode(), logPre.firstChild);
                }
                const note = logPre.firstChild;
                for (let i = 0; i < overflow && note.nextSibling; i++) note.nextSibling.remove();
            }
            if (state.follow) logPre.scrollTop = logPre.scrollHeight;
        }

        // loadLog reads the stored log of buildId. keep leaves the visible lines in place until the
        // answer arrives, and when it is empty (an interrupted build stores no log).
        async function loadLog(buildId, keep) {
            const token = ++state.logToken;
            if (!keep) {
                state.lines = [];
                state.truncated = false;
                renderLog();
            }
            if (!buildId) return;
            try {
                const text = await inst.api.buildLog(buildId);
                if (token !== state.logToken || b.disposed) return;
                if (keep && !text) return;
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

        // renderSelected repaints the selected build after its record changed (prev: the record
        // before, null for a new one). A build that just finished gets its stored log: live lines
        // are throttled (skip markers) and carry no offset, while the stored log is complete before
        // the final build_status (the artefacts are stored before finishBuild emits it).
        function renderSelected(prev) {
            const build = selectedBuild();
            renderBuildStatus();
            if (!build || ACTIVE_BUILD.has(build.status)) return;
            setDiagnostics(build.diagnostics || []);
            if (!prev || ACTIVE_BUILD.has(prev.status)) loadLog(build.id, true);
        }

        // mergeBuilds combines GET /builds with the builds events reported meanwhile (the request and
        // the stream race): a listed build takes the event copy when that one has progressed further;
        // an unlisted build stays when it is active or not older than the newest listed one.
        function mergeBuilds(list, reported) {
            const byId = new Map();
            (Array.isArray(list) ? list : []).forEach(build => { if (build && build.id) byId.set(build.id, build); });
            const listed = Array.from(byId.values());
            reported.forEach(build => {
                const known = byId.get(build.id);
                if (known ? progress(build) > progress(known) : !pruned(build, listed)) byId.set(build.id, build);
            });
            return sortBuilds(Array.from(byId.values()));
        }

        // loadBuilds reads GET /builds; only the newest load applies. The first load merges what
        // events reported before it (the window's builds and the held-aside finished ones). A later
        // load (reloadProject after a restored database or pruned events) replaces the window's
        // builds and merges only those events reported while it ran, so stale builds disappear.
        async function loadBuilds() {
            const token = ++state.loadToken;
            const reload = state.buildsLoaded;
            state.seen = new Map();
            try {
                const list = await inst.api.listBuilds(inst.projectId);
                if (b.disposed || token !== state.loadToken) return;
                const before = selectedBuild();
                const reported = reload ? Array.from(state.seen.values()) : state.builds.concat(Array.from(state.early.values()));
                state.builds = mergeBuilds(list, reported);
                state.seen = null;
                state.early.clear();
                state.buildsLoaded = true;
                // A build the window already shows (started or followed meanwhile) stays selected.
                if (before && selectedBuild()) {
                    renderBuildOptions();
                    renderSelected(before);
                } else {
                    select(state.builds.length ? state.builds[0].id : '');
                }
            } catch (err) {
                if (b.disposed || token !== state.loadToken) return;
                state.seen = null;
                inst.notify('error', P.errors.message(t, err));
            }
        }

        // upsert takes a Build from POST /builds or a build_status event. A new build becomes the
        // selected one while "follow" is on. A copy with less progress than the known one is stale.
        // An unknown finished build waits for GET /builds (mergeBuilds) and, once the list is
        // loaded, is ignored when it is older than the newest known build (pruned).
        function upsert(build) {
            if (!build || !build.id) return;
            const index = state.builds.findIndex(x => x.id === build.id);
            const prev = index >= 0 ? state.builds[index] : null;
            if (prev && progress(build) < progress(prev)) return;
            if (!prev && !ACTIVE_BUILD.has(build.status)) {
                if (!state.buildsLoaded) { state.early.set(build.id, build); return; }
                if (pruned(build, state.builds)) return;
            }
            if (state.seen) state.seen.set(build.id, build);
            if (prev) state.builds[index] = build;
            else state.builds.unshift(build);
            state.builds = sortBuilds(state.builds);
            if (!prev && (state.follow || !state.selected)) {
                state.selected = build.id;
                state.lines = [];
                state.truncated = false;
                state.logToken++;
                renderLog();
            }
            renderBuildOptions();
            if (build.id === state.selected) renderSelected(prev);
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
        const problemsHead = el('p', { class: 'cydc-problems-summary', dataset: { ccProblemsSummary: '' } });
        const problemsList = el('ul', { class: 'cydc-problems-list', dataset: { ccProblems: '' } });
        const problemsEmpty = el('p', { class: 'cydc-empty', text: t('cyd_creator.dock.no_problems') });
        const problemsPanel = el('div', { class: 'cydc-problems' }, problemsHead, problemsEmpty, problemsList);

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
                    type: 'button', class: 'cydc-problem', disabled: !row.open,
                    dataset: { severity: row.severity, source: row.source, ccProblem: '' }
                },
                el('span', { class: 'cydc-problem-icon', 'aria-hidden': 'true', text: row.severity === 'error' ? '✕' : (row.severity === 'warning' ? '!' : 'i') }),
                el('span', { class: 'cydc-problem-text', text: row.text }),
                row.generated ? el('span', { class: 'cydc-tag', title: t('cyd_creator.dock.generated_hint'), text: t('cyd_creator.dock.generated') }) : null,
                el('span', { class: 'cydc-problem-source', text: t('cyd_creator.dock.source_' + row.source) }),
                row.where ? el('span', { class: 'cydc-problem-where', text: row.where }) : null);
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
        if (!state.current) show(tabs.has(prefs.tab) ? prefs.tab : 'log', true);
        setCollapsed(startCollapsed);
        setHeight(startHeight());
        renderBuildStatus();
        renderProblems();

        b.add(inst.bus.on('build:status', build => upsert(build)));
        b.add(inst.bus.on('sse:build_log', data => {
            if (!data || data.build_id !== state.selected) return;
            pushLines(data.lines);
        }));
        b.add(inst.bus.on('layout:issues', issues => setIssues(issues)));
        // The app sets the mode once the designer is mounted (and on every switch).
        b.add(inst.bus.on('mode:changed', () => { if (!custom) setHeight(startHeight()); }));
        // Live lines of a running build are ephemeral: after every (re)connect, the first one too
        // (lines between the HTTP snapshot and the subscription), read its log again.
        b.add(inst.bus.on('sse:stream_ready', () => {
            const build = selectedBuild();
            if (build && ACTIVE_BUILD.has(build.status)) loadLog(build.id, true);
        }));

        const dock = {
            el: root,
            registerTab,
            show: id => show(id),
            current: () => state.current,
            setCollapsed,
            toggle: () => setCollapsed(!prefs.collapsed),
            problems: { set: setIssues, setDiagnostics, list: () => ({ issues: state.issues.slice(), diagnostics: state.diagnostics.slice() }) },
            builds: { load: loadBuilds, upsert, select, list: () => state.builds.slice(), selected: () => selectedBuild() },
            // trackBuild follows a build the user just started. Its build_status events may have
            // arrived before the POST answer: then the answer ("queued") must not overwrite them,
            // and select() selects the build when upsert did not.
            trackBuild(build) {
                if (!build || !build.id) return;
                state.follow = true;
                followBox.checked = true;
                if (!state.builds.some(x => x.id === build.id)) upsert(build);
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
