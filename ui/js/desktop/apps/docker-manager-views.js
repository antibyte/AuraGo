/* Docker Manager views: HTML builders for the rail, the banners, the container table, the empty states and the detail
   drawer. A classic script (window.DockerManagerViews) with no DOM work at load. Every function takes a snapshot and
   returns a string; docker-manager.js inserts it, keeps the focus and routes the events through data-action hooks.
   User data (names, images, labels, log lines) always goes through esc(). Colour comes from data-tone and
   data-host-color attributes, never from inline styles. */
(function () {
    'use strict';

    const GLYPHS = {
        play: '<path d="M8 5l11 7-11 7z"/>',
        stop: '<rect x="6" y="6" width="12" height="12" rx="2"/>',
        restart: '<path d="M20 12a8 8 0 1 1-2.4-5.7"/><path d="M20 4v6h-6"/>',
        pause: '<path d="M9 6v12M15 6v12"/>',
        kill: '<path d="M13 3L5 14h6l-1 7 8-11h-6z"/>',
        remove: '<path d="M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3"/>',
        logs: '<path d="M6 3h9l4 4v14H6z"/><path d="M14 3v5h5M9 13h7M9 17h7"/>',
        inspect: '<circle cx="12" cy="12" r="9"/><path d="M12 11v6M12 7.5v.01"/>',
        more: '<circle cx="5" cy="12" r="1.5" fill="currentColor" stroke="none"/><circle cx="12" cy="12" r="1.5" fill="currentColor" stroke="none"/><circle cx="19" cy="12" r="1.5" fill="currentColor" stroke="none"/>',
        lock: '<rect x="5" y="11" width="14" height="9" rx="2"/><path d="M8 11V8a4 4 0 0 1 8 0v3"/>',
        plus: '<path d="M12 5v14M5 12h14"/>',
        chevron: '<path d="M9 6l6 6-6 6"/>',
        refresh: '<path d="M20 12a8 8 0 1 1-2.4-5.7"/><path d="M20 4v6h-6"/>',
        close: '<path d="M6 6l12 12M18 6L6 18"/>',
        copy: '<rect x="9" y="9" width="11" height="11" rx="2"/><path d="M5 15V6a2 2 0 0 1 2-2h9"/>',
        edit: '<path d="M4 20l1-4L16 5l3 3L8 19z"/>',
        server: '<rect x="3" y="4" width="18" height="6" rx="2"/><rect x="3" y="14" width="18" height="6" rx="2"/><path d="M7 7h.01M7 17h.01"/>',
        containers: '<rect x="6" y="3" width="14" height="5" rx="1"/><rect x="3" y="9.5" width="14" height="5" rx="1"/><rect x="7" y="16" width="14" height="5" rx="1"/>',
        search: '<circle cx="11" cy="11" r="6.5"/><path d="M16 16l4 4"/>',
        warning: '<path d="M12 3l10 18H2z"/><path d="M12 10v5M12 18v.01"/>',
        check: '<path d="M5 12l5 5 9-10"/>',
        download: '<path d="M12 3v12M7 10l5 5 5-5M4 20h16"/>',
        stacks: '<path d="M12 3l9 5-9 5-9-5z"/><path d="M3 12.5l9 5 9-5M3 17l9 5 9-5"/>',
        deploy: '<path d="M12 16V4M7 9l5-5 5 5M4 20h16"/>',
        link: '<path d="M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1"/>',
        folder: '<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>',
        command: '<path d="M9 6a3 3 0 1 0-3 3h12a3 3 0 1 0-3-3v12a3 3 0 1 0 3-3H6a3 3 0 1 0 3 3z"/>',
        file: '<path d="M6 3h9l4 4v14H6z"/><path d="M14 3v5h5"/>'
    };
    const ACTION_GLYPH = { start: 'play', unpause: 'play', stop: 'stop', restart: 'restart', pause: 'pause', kill: 'kill', remove: 'remove' };
    const ERROR_KEYS = ['read_only', 'container_not_found', 'invalid_host', 'network', 'timeout', 'forbidden', 'host_not_prepared', 'api_too_old', 'stacks_unavailable',
        'stack_name_invalid', 'stack_not_found', 'stack_exists', 'operation_in_progress', 'file_too_large', 'runner_image_missing', 'stacks_dir_unavailable', 'convert_failed', 'revision_conflict',
        'invalid_operation', 'operation_not_found', 'docker_error', 'host_not_found', 'invalid_action', 'internal_error', 'bad_request'];
    // Codes whose own sentence is followed by the server's message, because the message names the cause.
    const DETAIL_ERROR_CODES = ['invalid_host', 'stacks_dir_unavailable', 'convert_failed', 'docker_error', 'bad_request'];
    // The labels of AuraGo's container protection ("owner" of the 409 answer) that are no AuraGo owner name, and the
    // docker_manager.protected_<kind> sentence of each.
    const PROTECTED_LABEL_KINDS = { self: 'self', 'docker-endpoint': 'endpoint', 'shared-network': 'network', unverified: 'unverified', docker_manager: 'helper' };
    const PORTS_SHOWN = 2;
    const TABS = ['overview', 'logs', 'inspect'];
    // Terminal colour codes in container logs would show as stray characters.
    const ANSI = /\x1b\[[0-9;?]*[ -/]*[@-~]/g;

    function create(deps) {
        const { t, esc, model, lang, uid } = deps;
        const glyph = (name, cls) => '<svg class="dm-glyph' + (cls ? ' ' + cls : '') + '" viewBox="0 0 24 24" aria-hidden="true" focusable="false">' + (GLYPHS[name] || '') + '</svg>';
        const dot = tone => '<span class="dm-dot" data-tone="' + esc(tone) + '" aria-hidden="true"></span>';
        const chip = (name, color) => '<span class="dm-chip" data-host-color="' + esc(color) + '"><span class="dm-chip-dot" aria-hidden="true"></span>' + esc(name) + '</span>';
        const keyOf = row => row.key;

        function statusLabel(status) {
            return status.id === 'exited' ? t('status_exited_code', { code: status.code }) : t('status_' + status.id);
        }
        function actionLabel(action) { return action === 'remove' ? t('action_remove') : t('action_' + action); }
        function reasonText(reason) { return t('reason_' + model.reasonCode(reason)); }
        function protectedText(container) {
            const reason = model.protectedReason(container);
            if (!reason) return '';
            return reason.kind === 'owner' ? t('protected_owner', { owner: reason.owner }) : t('protected_' + reason.kind);
        }
        // The same sentences for the `owner` of a 409 container_protected_confirmation_required answer: the server names the most
        // specific reason by a label (self, docker-endpoint, shared-network, unverified) or by the AuraGo owner of the container.
        function protectedLabelText(label) {
            const kind = PROTECTED_LABEL_KINDS[label];
            if (kind) return t('protected_' + kind);
            return label ? t('protected_owner', { owner: label }) : '';
        }
        // A user-facing sentence for a failure. Codes without their own text show the generic sentence plus the server's message,
        // because that message names the cause (an engine error, a validation problem).
        function errorText(info) {
            const e = info || {}, data = e.data || {};
            if (e.code === 'host_unreachable') return reasonText(e.reason);
            const known = ERROR_KEYS.includes(e.code), base = known ? t('error_' + e.code, { version: String(data.api_version || ''), command: String(data.command || '') }) : t('error_unknown');
            return (!known || DETAIL_ERROR_CODES.includes(e.code)) && e.message ? base + ' ' + e.message : base;
        }
        function stripAnsi(value) { return String(value || '').replace(ANSI, ''); }

        // ---- rail --------------------------------------------------------------------------------------------------

        function hostLine(host, entry, health) {
            if (health.state === 'error') return '<span class="dm-host-sub" data-tone="' + health.tone + '">' + esc(t('host_error')) + '</span>';
            if (!entry || !entry.loaded) return '<span class="dm-host-sub">' + esc(health.state === 'ok' ? t('loading') : t('host_unknown')) + '</span>';
            const counts = model.hostCounts(host.id, { [host.id]: entry });
            return '<span class="dm-host-sub">' + esc(t('count_running_total', counts)) + '</span>';
        }

        function rail(snap) {
            const s = snap.state, scope = s.scope, totals = model.totalCounts(snap.hosts, s.byHost);
            const option = (id, color, name, titleText, line, tone, word) => {
                const selected = scope === id;
                return '<button type="button" role="option" class="dm-host-btn" data-action="pick-host" data-host-id="' + esc(id) + '" data-focus-id="host:' + esc(id) + '" aria-selected="' + selected
                    + '" tabindex="' + (selected ? '0' : '-1') + '" title="' + esc(titleText) + '"><span class="dm-swatch" data-host-color="' + esc(color) + '" aria-hidden="true"></span>'
                    + '<span class="dm-host-text"><span class="dm-host-name">' + esc(name) + '</span>' + line + '</span>' + (tone ? dot(tone) + '<span class="dm-sr-only">' + esc(word) + '</span>' : '') + '</button>';
            };
            const allLine = '<span class="dm-host-sub">' + esc(t('count_running_total', totals)) + '</span>';
            const items = [option('all', 'all', t('all_hosts'), t('all_hosts'), allLine, '', '')];
            for (const host of snap.hosts) {
                const entry = s.byHost[host.id], health = model.hostHealth(host, entry);
                items.push(option(host.id, model.colorOf(host.color), host.name, host.name + ' · ' + model.hostAddress(host), hostLine(host, entry, health), health.tone, t('host_' + health.state)));
            }
            const navButton = (section, glyphName, label, count) => '<button type="button" class="dm-nav-btn" data-action="pick-section" data-section="' + section + '" data-focus-id="nav:' + section + '"'
                + (snap.section === section ? ' aria-current="page"' : '') + '>' + glyph(glyphName) + '<span class="dm-nav-label">' + esc(label) + '</span><span class="dm-count">' + count + '</span></button>';
            return '<div class="dm-rail-section"><h2 class="dm-rail-title" id="' + uid + '-hosts">' + esc(t('hosts_heading')) + '</h2>'
                + '<div class="dm-hosts" role="listbox" aria-labelledby="' + uid + '-hosts" data-host-list>' + items.join('') + '</div></div>'
                + '<nav class="dm-nav" aria-label="' + esc(t('nav_label')) + '">' + navButton('stacks', 'stacks', t('nav_stacks'), snap.stackCounts.total) + navButton('containers', 'containers', t('nav_containers'), totals.total) + '</nav>'
                + '<div class="dm-rail-foot"><button type="button" class="dm-link-btn" data-action="manage-hosts" data-focus-id="manage">' + glyph('server') + '<span>' + esc(t('manage_hosts')) + '</span></button></div>';
        }

        // ---- banners (one per unreachable host in scope) ----------------------------------------------------------------

        function banners(snap) {
            const out = [];
            for (const host of snap.scopeHosts) {
                const entry = snap.state.byHost[host.id], health = model.hostHealth(host, entry);
                if (health.state !== 'error') continue;
                const stale = entry && entry.containers && entry.containers.length > 0;
                const id = uid + '-banner-' + host.id;
                out.push('<section class="dm-banner" data-tone="' + health.tone + '" aria-labelledby="' + esc(id) + '">'
                    + '<span class="dm-banner-icon">' + glyph('warning') + '</span><div class="dm-banner-body"><h3 id="' + esc(id) + '">' + esc(t('banner_title', { host: host.name })) + '</h3>'
                    + '<p>' + esc(reasonText(health.reason)) + (stale ? ' ' + esc(t('stale_notice')) : '') + '</p>'
                    + (health.message ? '<details class="dm-details"><summary>' + esc(t('banner_details')) + '</summary><pre class="dm-pre">' + esc(health.message) + '</pre></details>' : '') + '</div>'
                    + '<div class="dm-banner-actions"><button type="button" class="dm-btn" data-action="retry-host" data-host-id="' + esc(host.id) + '" data-focus-id="retry:' + esc(host.id) + '">' + glyph('refresh') + '<span>' + esc(t('banner_retry')) + '</span></button>'
                    + (snap.canEditHosts && !host.builtin ? '<button type="button" class="dm-btn" data-action="edit-host" data-host-id="' + esc(host.id) + '" data-focus-id="edit:' + esc(host.id) + '">' + glyph('edit') + '<span>' + esc(t('host_edit_title')) + '</span></button>' : '')
                    + '</div></section>');
            }
            return out.join('');
        }

        // ---- table -------------------------------------------------------------------------------------------------------

        function iconButton(action, extra, label, glyphName, o) {
            const key = o.key;
            return '<button type="button" class="dm-icon-btn' + (o.danger ? ' is-danger' : '') + '" data-action="' + action + '"' + extra + ' data-row-key="' + esc(key) + '" data-row-btn data-focus-id="' + o.focus
                + '" aria-label="' + esc(t('row_action_label', { action: label, name: o.name })) + '" title="' + esc(label) + '" tabindex="-1"' + (o.busy ? ' disabled aria-busy="true"' : '') + '>' + glyph(glyphName) + '</button>';
        }

        function portsCell(container) {
            const items = model.portItems(container.ports);
            if (!items.length) return '<span class="dm-muted" aria-hidden="true">—</span><span class="dm-sr-only">' + esc(t('ov_none')) + '</span>';
            const shown = items.slice(0, PORTS_SHOWN).map(item => '<span class="dm-port">' + esc(item.label) + '</span>').join('');
            const rest = items.slice(PORTS_SHOWN);
            return shown + (rest.length ? '<span class="dm-port is-more" title="' + esc(rest.map(item => item.label).join(', ')) + '">+' + rest.length + '</span>' : '');
        }

        function row(snap, r) {
            const c = r.c, status = model.statusOf(c), name = model.containerName(c), id = model.containerId(c);
            const actions = model.actionsFor(c, !snap.canMutate), busy = snap.state.busy.has(r.hostId + '/' + id);
            const open = !!(snap.state.detail && snap.state.detail.key === r.key);
            const sub = [c.compose_service, c.id].filter(Boolean).join(' · ');
            const lock = model.isProtected(c) ? '<span class="dm-lock" role="img" aria-label="' + esc(protectedText(c)) + '" title="' + esc(protectedText(c)) + '">' + glyph('lock') + '</span>' : '';
            // Empty slots keep the four buttons of every row in the same columns.
            const slot = '<span class="dm-icon-slot" aria-hidden="true"></span>', buttons = [];
            buttons.push(actions.toggle
                ? iconButton('act', ' data-act="' + actions.toggle + '"', actionLabel(actions.toggle), ACTION_GLYPH[actions.toggle], { key: r.key, name, busy, focus: 'act:' + esc(r.key) + ':toggle' })
                : slot);
            buttons.push(!actions.restart ? slot : iconButton('act', ' data-act="restart"', actionLabel('restart'), 'restart', { key: r.key, name, busy, focus: 'act:' + esc(r.key) + ':restart' }));
            buttons.push(iconButton('logs', '', t('action_logs'), 'logs', { key: r.key, name, busy: false, focus: 'act:' + esc(r.key) + ':logs' }));
            buttons.push(iconButton('more', '', t('action_more'), 'more', { key: r.key, name, busy: false, focus: 'act:' + esc(r.key) + ':more' }));
            return '<tr class="dm-row" role="row" data-nav-key="' + esc(r.key) + '" data-row-key="' + esc(r.key) + '" data-focus-id="row:' + esc(r.key) + '" data-tone="' + status.tone + '" aria-level="'
                + (snap.headers ? 2 : 1) + '" aria-selected="' + open + '" aria-busy="' + busy + '" tabindex="-1">'
                + '<td role="gridcell" class="dm-cell-status">' + dot(status.tone) + '<span class="dm-status"><span class="dm-status-text">' + esc(statusLabel(status)) + '</span><span class="dm-status-sub">' + esc(c.status) + '</span></span></td>'
                + '<td role="gridcell" class="dm-cell-name"><span class="dm-name-line"><span class="dm-name">' + esc(name) + '</span>' + lock + (snap.multi ? chip(r.hostName, r.hostColor) : '') + '</span><span class="dm-sub">' + esc(sub) + '</span></td>'
                + '<td role="gridcell" class="dm-cell-image"><span class="dm-image" title="' + esc(c.image) + '">' + esc(model.shortImage(c.image)) + '</span></td>'
                + '<td role="gridcell" class="dm-cell-ports">' + portsCell(c) + '</td>'
                + '<td role="gridcell" class="dm-cell-actions"><div class="dm-actions" role="group" aria-label="' + esc(t('row_actions_label', { name })) + '">' + buttons.join('') + '</div></td></tr>';
        }

        function groupRow(snap, group, collapsed) {
            const name = group.kind === 'project' ? group.project : t('group_single');
            const counts = group.counts;
            const text = t('count_running_total', { running: counts.running, total: counts.total }) + (counts.problem ? ' · ' + t('group_problems', { count: counts.problem }) : '');
            return '<tr class="dm-group-row" role="row" data-nav-key="g:' + esc(group.key) + '" data-group-key="' + esc(group.key) + '" data-focus-id="group:' + esc(group.key) + '" data-action="toggle-group" aria-level="1" aria-expanded="' + !collapsed + '" tabindex="-1">'
                + '<td role="gridcell" colspan="5"><span class="dm-group-main">' + glyph('chevron', 'dm-chevron') + (group.kind === 'project' ? dot(group.tone) : '') + '<strong class="dm-group-name">' + esc(name) + '</strong>'
                + (snap.multi && group.kind === 'project' ? chip(group.hostName, group.hostColor) : '') + '<span class="dm-group-count">' + esc(text) + '</span></span></td></tr>';
        }

        function tbody(snap) {
            const parts = [];
            for (const group of snap.groups) {
                const collapsed = snap.headers && snap.collapsed.has(group.key);
                if (snap.headers) parts.push(groupRow(snap, group, collapsed));
                if (!collapsed) for (const r of group.rows) parts.push(row(snap, r));
            }
            return parts.join('');
        }

        // ---- empty states ------------------------------------------------------------------------------------------------

        function panel(glyphName, title, body, button) {
            return '<div class="dm-empty-inner">' + glyph(glyphName, 'dm-empty-glyph') + '<h3>' + esc(title) + '</h3><p>' + esc(body) + '</p>' + (button || '') + '</div>';
        }
        const actionButton = (action, label, primary) => '<button type="button" class="dm-btn' + (primary ? ' is-primary' : '') + '" data-action="' + action + '" data-focus-id="' + action + '">' + esc(label) + '</button>';

        // What replaces both sections while Docker is switched off or the host list has not loaded (yet); '' otherwise.
        function gate(snap) {
            const s = snap.state;
            if (s.disabled) return panel('server', t('disabled_title'), t('disabled_body'), actionButton('open-settings', t('open_settings'), true));
            if (!s.hostsLoaded) {
                if (s.hostsFailed) return panel('warning', t('load_error_title'), errorText(s.hostsFailed), actionButton('refresh', t('banner_retry'), true));
                return '<div class="dm-empty-inner is-loading" role="status"><span class="dm-spinner" aria-hidden="true"></span><p>' + esc(t('loading')) + '</p></div>';
            }
            return '';
        }

        // '' when the table has rows. Unreachable hosts are explained by their banners, so an all-unreachable scope shows no extra message.
        function empty(snap) {
            const s = snap.state, gated = gate(snap);
            if (gated) return gated;
            if (snap.visible.length) return '';
            if (snap.rows.length === 0) {
                if (snap.scopeHosts.some(host => !s.byHost[host.id])) return '<div class="dm-empty-inner is-loading" role="status"><span class="dm-spinner" aria-hidden="true"></span><p>' + esc(t('loading')) + '</p></div>';
                if (snap.scopeHosts.every(host => !(s.byHost[host.id] && s.byHost[host.id].loaded))) return '';
                return panel('containers', t('empty_title'), snap.multi ? t('empty_all_body') : t('empty_body'));
            }
            return panel('search', t('no_match_title'), t('no_match_body'), actionButton('clear-filters', t('clear_filters'), false));
        }

        // ---- line diff (the revision-conflict dialog) -------------------------------------------------------------------

        // rows come from stackModel.lineDiff; unchanged runs collapse into one "N unchanged lines" row.
        function diffHtml(rows) {
            const body = deps.stackModel.collapseDiff(rows, 2).map(row => {
                if (row.type === 'skip') return '<div class="dm-diff-skip" role="row">' + esc(t('diff_skipped', { count: row.count })) + '</div>';
                const mark = row.type === 'add' ? '+' : row.type === 'del' ? '-' : '';
                const word = row.type === 'add' ? t('diff_added') : row.type === 'del' ? t('diff_removed') : '';
                return '<div class="dm-diff-row" role="row" data-diff="' + row.type + '"><span class="dm-diff-no" aria-hidden="true">' + (row.oldNo || '') + '</span><span class="dm-diff-no" aria-hidden="true">' + (row.newNo || '') + '</span>'
                    + '<span class="dm-diff-mark" aria-hidden="true">' + mark + '</span>' + (word ? '<span class="dm-sr-only">' + esc(word) + '</span>' : '') + '<code class="dm-diff-text">' + esc(row.text) + '</code></div>';
            }).join('');
            return '<div class="dm-diff" role="table" aria-label="' + esc(t('diff_label')) + '" tabindex="0">' + body + '</div>';
        }

        // ---- detail drawer -----------------------------------------------------------------------------------------------

        function fact(label, html) { return '<div class="dm-fact"><dt>' + esc(label) + '</dt><dd>' + html + '</dd></div>'; }
        const none = () => '<span class="dm-muted">' + esc(t('ov_none')) + '</span>';
        const list = items => (items.length ? '<ul class="dm-fact-list">' + items.map(item => '<li>' + esc(item) + '</li>').join('') + '</ul>' : none());

        function overview(r) {
            const c = r.c, status = model.statusOf(c), note = protectedText(c);
            const facts = [
                fact(t('ov_state'), dot(status.tone) + '<span class="dm-fact-state">' + esc(statusLabel(status)) + '</span> <span class="dm-muted">' + esc(c.status) + '</span>'),
                fact(t('ov_host'), chip(r.hostName, r.hostColor)),
                fact(t('col_image'), esc(c.image)),
                fact(t('ov_id'), esc(c.id)),
                fact(t('ov_created'), esc(model.formatCreated(c.created, lang)))
            ];
            if (c.compose_project) facts.push(fact(t('ov_project'), esc(c.compose_project)));
            if (c.compose_service) facts.push(fact(t('ov_service'), esc(c.compose_service)));
            if (c.compose_working_dir) facts.push(fact(t('ov_workdir'), esc(c.compose_working_dir)));
            facts.push(fact(t('col_ports'), list(model.portItems(c.ports).map(item => item.label))));
            facts.push(fact(t('ov_networks'), list(Array.isArray(c.networks) ? c.networks : [])));
            facts.push(fact(t('ov_mounts'), list((Array.isArray(c.mounts) ? c.mounts : []).map(model.mountLabel))));
            return (note ? '<p class="dm-note">' + glyph('lock') + '<span>' + esc(note) + '</span></p>' : '') + '<dl class="dm-facts">' + facts.join('') + '</dl>';
        }

        function copyButton(action) {
            return '<button type="button" class="dm-btn" data-action="' + action + '" data-focus-id="' + action + '">' + glyph('copy') + '<span>' + esc(t('copy')) + '</span></button>';
        }

        function logsPanel(detail) {
            const logs = detail.logs;
            const tails = deps.logTails.map(n => '<option value="' + n + '"' + (n === logs.tail ? ' selected' : '') + '>' + n + '</option>').join('');
            const tools = '<div class="dm-panel-tools"><label class="dm-inline"><span>' + esc(t('logs_tail')) + '</span><select data-log-tail data-focus-id="log-tail">' + tails + '</select></label>'
                + '<button type="button" class="dm-btn" data-action="logs-reload" data-focus-id="logs-reload">' + glyph('refresh') + '<span>' + esc(t('logs_reload')) + '</span></button>' + copyButton('copy-logs') + '</div>';
            let body;
            if (logs.status === 'error') body = '<p class="dm-error" role="alert">' + esc(t('logs_failed')) + ' ' + esc(errorText(logs.error)) + '</p>';
            else if (logs.status === 'ok' || (logs.status === 'loading' && logs.text)) {
                body = logs.text
                    ? '<pre class="dm-log' + (logs.status === 'loading' ? ' is-loading' : '') + '" tabindex="0" data-log aria-label="' + esc(t('tab_logs')) + '">' + esc(stripAnsi(logs.text)) + '</pre>'
                    : '<p class="dm-muted">' + esc(t('logs_empty')) + '</p>';
            } else body = '<p class="dm-muted" role="status">' + esc(t('logs_loading')) + '</p>';
            return tools + body;
        }

        function inspectPanel(detail) {
            const inspect = detail.inspect;
            let body;
            if (inspect.status === 'error') body = '<p class="dm-error" role="alert">' + esc(t('inspect_failed')) + ' ' + esc(errorText(inspect.error)) + '</p>';
            else if (inspect.status === 'ok') body = '<pre class="dm-log dm-code" tabindex="0" data-inspect aria-label="' + esc(t('tab_inspect')) + '">' + esc(JSON.stringify(inspect.data, null, 2)) + '</pre>';
            else body = '<p class="dm-muted" role="status">' + esc(t('inspect_loading')) + '</p>';
            return '<div class="dm-panel-tools">' + copyButton('copy-inspect') + '</div>' + body;
        }

        function drawerActions(snap, r) {
            const actions = model.actionsFor(r.c, !snap.canMutate), name = model.containerName(r.c);
            if (!actions.toggle && !actions.restart && !actions.menu.length && !actions.remove) return '';
            const busy = snap.state.busy.has(r.hostId + '/' + model.containerId(r.c));
            const button = (action, act, label, glyphName, danger) => '<button type="button" class="dm-btn' + (danger ? ' is-danger' : '') + '" data-action="' + action + '"' + (act ? ' data-act="' + act + '"' : '')
                + ' data-row-key="' + esc(r.key) + '" data-focus-id="d:' + (act || action) + '"' + (busy ? ' disabled aria-busy="true"' : '') + ' aria-label="' + esc(t('row_action_label', { action: label, name })) + '">' + glyph(glyphName) + '<span>' + esc(label) + '</span></button>';
            const out = [];
            if (actions.toggle) out.push(button('act', actions.toggle, actionLabel(actions.toggle), ACTION_GLYPH[actions.toggle], false));
            if (actions.restart) out.push(button('act', 'restart', actionLabel('restart'), 'restart', false));
            for (const action of actions.menu) out.push(button('act', action, actionLabel(action), ACTION_GLYPH[action], action === 'kill'));
            if (actions.remove) out.push(button('remove', '', actionLabel('remove'), 'remove', true));
            return '<div class="dm-drawer-actions">' + out.join('') + '</div>';
        }

        function drawer(snap, r) {
            const detail = snap.state.detail, c = r.c, status = model.statusOf(c), name = model.containerName(c);
            const tabs = TABS.map(tab => '<button type="button" role="tab" class="dm-tab" id="' + uid + '-tab-' + tab + '" data-action="tab" data-tab="' + tab + '" data-focus-id="tab:' + tab + '" aria-selected="' + (detail.tab === tab)
                + '" aria-controls="' + uid + '-tabpanel" tabindex="' + (detail.tab === tab ? '0' : '-1') + '">' + esc(t('tab_' + tab)) + '</button>').join('');
            let body;
            if (detail.tab === 'logs') body = logsPanel(detail);
            else if (detail.tab === 'inspect') body = inspectPanel(detail);
            else body = overview(r);
            return '<header class="dm-drawer-head"><div class="dm-drawer-title">' + dot(status.tone) + '<h2 id="' + uid + '-drawer-title">' + esc(name) + '</h2></div>'
                + '<button type="button" class="dm-icon-btn" data-action="close-detail" data-focus-id="close-detail" aria-label="' + esc(t('drawer_close')) + '" title="' + esc(t('drawer_close')) + '">' + glyph('close') + '</button></header>'
                + '<p class="dm-drawer-sub">' + esc(statusLabel(status)) + ' · ' + esc(model.shortImage(c.image)) + '</p>'
                + drawerActions(snap, r)
                + '<div class="dm-tabs" role="tablist" aria-label="' + esc(t('tabs_label')) + '">' + tabs + '</div>'
                + '<div class="dm-drawer-body" role="tabpanel" id="' + uid + '-tabpanel" aria-labelledby="' + uid + '-tab-' + detail.tab + '" data-tab-body="' + detail.tab + '">' + body + '</div>';
        }

        return { glyph, dot, chip, statusLabel, actionLabel, reasonText, protectedText, protectedLabelText, errorText, stripAnsi, rail, banners, tbody, gate, empty, drawer, diffHtml, keyOf, TABS };
    }

    window.DockerManagerViews = { create, TABS, ACTION_GLYPH };
})();
