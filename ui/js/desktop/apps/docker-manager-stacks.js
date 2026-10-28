/* Docker Manager stacks: HTML builders for the Stacks section, the stack list with its three groups and the host cards
   ("Prepare host", API too old, folder unavailable), and the stack detail pane (header, tabs, overview, the static shell
   the editors are mounted into). A classic script (window.DockerManagerStacks) with no DOM work at load. Every function
   takes a snapshot and returns a string; docker-manager.js inserts it and routes the events through data-action hooks.
   User data (names, paths, images, messages) always goes through esc(); colour comes from data-tone and data-host-color. */
(function () {
    'use strict';

    const PORTS_SHOWN = 3;
    const FILE_NAMES = { compose: 'compose.yaml', env: '.env' };

    function create(deps) {
        const { t, esc, model, stackModel, views, uid } = deps;
        const { glyph, dot, chip } = views;
        const sm = stackModel;

        const statusText = stack => t('stack_status_' + sm.stackStatusId(stack));
        const button = (action, label, o) => {
            const x = o || {};
            return '<button type="button" class="dm-btn' + (x.primary ? ' is-primary' : '') + (x.danger ? ' is-danger' : '') + '" data-action="' + action + '"' + (x.attrs || '')
                + ' data-focus-id="' + esc(x.focus || action) + '"' + (x.disabled ? ' disabled aria-busy="' + (x.busy ? 'true' : 'false') + '"' : '') + '>' + (x.glyph ? glyph(x.glyph) : '') + '<span>' + esc(label) + '</span></button>';
        };
        const iconButton = (action, label, glyphName, attrs, o) => {
            const x = o || {};
            return '<button type="button" class="dm-icon-btn' + (x.danger ? ' is-danger' : '') + '" data-action="' + action + '"' + (attrs || '') + ' data-focus-id="' + esc(x.focus || action)
                + '" aria-label="' + esc(label) + '" title="' + esc(label) + '"' + (x.disabled ? ' disabled' : '') + (x.busy ? ' aria-busy="true"' : '') + '>' + glyph(glyphName) + '</button>';
        };
        const opBadge = op => (op ? '<span class="dm-opbadge" role="status"><span class="dm-spinner is-small" aria-hidden="true"></span>' + esc(t('op_kind_' + op.kind)) + '</span>' : '');
        const spinner = () => '<div class="dm-empty-inner is-loading" role="status"><span class="dm-spinner" aria-hidden="true"></span><p>' + esc(t('loading')) + '</p></div>';

        function ageText(created, now) {
            const age = sm.age(created, now);
            return age ? t('stack_up', { age: t('age_' + age.unit, { count: age.n }) }) : '';
        }

        // Published ports in a list row: plain labels (the row is a button; the links live in the detail pane).
        function portChips(ports) {
            const seen = new Set(), labels = [];
            for (const port of (Array.isArray(ports) ? ports : [])) {
                const published = port && Number(port.public_port);
                if (published && !seen.has(published)) { seen.add(published); labels.push(published); }
            }
            labels.sort((a, b) => a - b);
            if (!labels.length) return '';
            const shown = labels.slice(0, PORTS_SHOWN).map(label => '<span class="dm-port">' + label + '</span>').join('');
            return '<span class="dm-srow-ports">' + shown + (labels.length > PORTS_SHOWN ? '<span class="dm-port is-more" title="' + esc(labels.slice(PORTS_SHOWN).join(', ')) + '">+' + (labels.length - PORTS_SHOWN) + '</span>' : '') + '</span>';
        }

        // ---- host cards --------------------------------------------------------------------------------------------------

        function helperFacts(helpers) {
            const h = helpers || {};
            const line = (ok, label) => '<li>' + dot(ok ? 'ok' : 'bad') + '<span>' + esc(label) + '</span></li>';
            return '<ul class="dm-facts-inline">' + line(!!h.runner_image_present, t(h.runner_image_present ? 'prepare_image_ok' : 'prepare_image_missing'))
                + line(h.io_container === 'ok', t(h.io_container === 'ok' ? 'prepare_io_ok' : h.io_container === 'mismatch' ? 'prepare_io_mismatch' : 'prepare_io_missing')) + '</ul>';
        }

        function hostCard(snap, host, state) {
            const entry = snap.state.stacks[host.id] || {}, error = entry.error || {}, data = error.data || {};
            const id = uid + '-hostcard-' + host.id;
            const head = chip(host.name, model.colorOf(host.color));
            const card = (tone, glyphName, title, body, extra, action) => '<section class="dm-hostcard" data-tone="' + tone + '" data-host-card="' + esc(host.id) + '" aria-labelledby="' + esc(id) + '"><span class="dm-banner-icon">' + glyph(glyphName) + '</span>'
                + '<div class="dm-hostcard-body"><h3 id="' + esc(id) + '">' + head + ' <span>' + esc(title) + '</span></h3><p>' + esc(body) + '</p>' + (extra || '') + '</div>' + (action || '') + '</section>';
            if (state === 'not_prepared') {
                const busy = !!snap.prepareActive[host.id];
                const action = snap.canMutate ? button('prepare-host', busy ? t('prepare_running') : t('prepare_button'), { primary: true, glyph: 'download', attrs: ' data-host-id="' + esc(host.id) + '"', focus: 'prepare:' + host.id, disabled: busy, busy }) : '';
                return card('warn', 'download', t('prepare_title'), t('prepare_body', { image: data.helpers && data.helpers.runner_image ? data.helpers.runner_image : 'docker:29.9.2-cli' }), helperFacts(data.helpers), action);
            }
            if (state === 'api_too_old') return card('warn', 'warning', t('api_old_title'), t('error_api_too_old', { version: String(data.api_version || '?'), command: '' }));
            if (state === 'unreachable') return card('bad', 'warning', t('host_error'), views.reasonText(error.reason), '', '');
            return card('warn', 'warning', t('stacks_unavailable_title'), views.errorText(error), '', '');
        }

        // ---- the list ----------------------------------------------------------------------------------------------------

        // The rows of a host whose last listing failed are what it showed before: greyed out.
        const staleAttr = (snap, hostId) => (snap.state.stacks[hostId] && snap.state.stacks[hostId].error ? ' data-stale="true"' : '');

        function stackRow(snap, row) {
            const s = Object.assign({ services: { total: 0, running: 0 }, ports: [] }, row.stack), tone = sm.stackTone(s), open = !!(snap.state.stack && snap.state.stack.key === row.key && !snap.state.detail);
            const op = s.operation && sm.isActiveState(s.operation.state) ? s.operation : null;
            const sub = [statusText(s), s.services.total ? t('count_running_total', { running: s.services.running, total: s.services.total }) : t('stack_no_services')].join(' · ');
            return '<li class="dm-srow" data-tone="' + tone + '"' + staleAttr(snap, row.hostId) + '><button type="button" class="dm-srow-main" data-action="open-stack" data-stack-key="' + esc(row.key) + '" data-snav="' + esc(row.key) + '" data-focus-id="srow:' + esc(row.key)
                + '" tabindex="-1"' + (open ? ' aria-current="true"' : '') + '>' + dot(tone) + '<span class="dm-srow-text"><span class="dm-name-line"><span class="dm-name">' + esc(s.name) + '</span>' + (snap.multi ? chip(row.hostName, model.colorOf(row.hostColor)) : '')
                + '</span><span class="dm-sub">' + esc(sub) + '</span></span>' + portChips(s.ports) + opBadge(op) + '</button></li>';
        }

        function externalRow(snap, row) {
            const p = Object.assign({ services: { total: 0, running: 0 }, ports: [] }, row.project), tone = sm.stackTone(p), busy = snap.state.busy.has(row.key);
            const sub = [statusText(p), t('count_running_total', { running: p.services.running, total: p.services.total }), p.working_dir].filter(Boolean).join(' · ');
            const attrs = ' data-host-id="' + esc(row.hostId) + '" data-project="' + esc(p.name) + '"';
            const actions = snap.canMutate ? '<div class="dm-srow-actions" role="group" aria-label="' + esc(t('row_actions_label', { name: p.name })) + '">'
                + iconButton('project-action', t('action_start') + ': ' + p.name, 'play', attrs + ' data-act="start"', { focus: 'xp:' + row.key + ':start', disabled: busy })
                + iconButton('project-action', t('action_stop') + ': ' + p.name, 'stop', attrs + ' data-act="stop"', { focus: 'xp:' + row.key + ':stop', disabled: busy })
                + iconButton('project-action', t('action_restart') + ': ' + p.name, 'restart', attrs + ' data-act="restart"', { focus: 'xp:' + row.key + ':restart', disabled: busy }) + '</div>' : '';
            return '<li class="dm-srow" data-tone="' + tone + '"' + staleAttr(snap, row.hostId) + '><button type="button" class="dm-srow-main" data-action="show-project" data-project="' + esc(p.name) + '" data-snav="' + esc(row.key) + '" data-focus-id="srow:' + esc(row.key) + '" tabindex="-1">'
                + dot(tone) + '<span class="dm-srow-text"><span class="dm-name-line"><span class="dm-name">' + esc(p.name) + '</span>' + (snap.multi ? chip(row.hostName, model.colorOf(row.hostColor)) : '')
                + '</span><span class="dm-sub" title="' + esc(p.working_dir) + '">' + esc(sub) + '</span></span>' + portChips(p.ports) + '</button>' + actions + '</li>';
        }

        function singleRow(snap, row) {
            const c = row.c, status = model.statusOf(c), name = model.containerName(c), lock = model.isProtected(c);
            const sub = [views.statusLabel(status), model.shortImage(c.image)].join(' · ');
            return '<li class="dm-srow" data-tone="' + status.tone + '"><button type="button" class="dm-srow-main" data-action="open-container" data-row-key="' + esc(row.key) + '" data-snav="c:' + esc(row.key) + '" data-focus-id="srow:c:' + esc(row.key) + '" tabindex="-1"'
                + (snap.state.detail && snap.state.detail.key === row.key ? ' aria-current="true"' : '') + '>' + dot(status.tone) + '<span class="dm-srow-text"><span class="dm-name-line"><span class="dm-name">' + esc(name) + '</span>'
                + (lock ? '<span class="dm-lock" role="img" aria-label="' + esc(views.protectedText(c)) + '" title="' + esc(views.protectedText(c)) + '">' + glyph('lock') + '</span>' : '') + (snap.multi ? chip(row.hostName, row.hostColor) : '')
                + '</span><span class="dm-sub">' + esc(sub) + '</span></span></button></li>';
        }

        function group(key, title, count, items) {
            const id = uid + '-g-' + key;
            return '<section class="dm-sgroup" data-sgroup="' + key + '" aria-labelledby="' + id + '"><h3 class="dm-sgroup-title" id="' + id + '">' + esc(title) + '<span class="dm-count">' + count + '</span></h3><ul class="dm-slist">' + items.join('') + '</ul></section>';
        }

        function ignoredNote(snap) {
            const names = [];
            for (const host of snap.scopeHosts) {
                const entry = snap.state.stacks[host.id], ignored = entry && entry.listing ? entry.listing.ignored : [];
                for (const item of ignored) names.push(snap.multi ? host.name + '/' + item.name : item.name);
            }
            return names.length ? '<p class="dm-hint dm-ignored">' + esc(t('ignored_note', { count: names.length, names: names.slice(0, 6).join(', ') })) + '</p>' : '';
        }

        // The whole list pane: host cards first, then the groups. '' means "nothing to show yet" (the loading state paints).
        function list(snap) {
            const cards = [];
            let loading = 0;
            for (const host of snap.scopeHosts) {
                const state = snap.hostStates[host.id];
                // The banner above the list already explains a host whose containers could not be read either.
                const banner = state === 'unreachable' && snap.state.byHost[host.id] && snap.state.byHost[host.id].error;
                if (state === 'loading') loading += 1;
                else if (state !== 'ok' && !banner) cards.push(hostCard(snap, host, state));
            }
            const g = snap.sgroups, shown = g.stacks.length + g.external.length + g.singles.length;
            const parts = cards.slice();
            if (g.stacks.length) parts.push(group('stacks', t('group_stacks'), g.stacks.length, g.stacks.map(row => stackRow(snap, row))));
            if (g.external.length) parts.push(group('external', t('group_external'), g.external.length, g.external.map(row => externalRow(snap, row))));
            if (g.singles.length) parts.push(group('singles', t('group_single'), g.singles.length, g.singles.map(row => singleRow(snap, row))));
            const listed = snap.scopeHosts.length > 0 && snap.scopeHosts.every(host => snap.hostStates[host.id] === 'ok');
            if (!shown && !cards.length) {
                if (loading || !snap.state.hostsLoaded) return spinner();
                if (snap.query) return '<div class="dm-empty-inner">' + glyph('search', 'dm-empty-glyph') + '<h3>' + esc(t('no_match_title')) + '</h3><p>' + esc(t('no_match_body')) + '</p>' + button('clear-filters', t('clear_filters'), {}) + '</div>';
                if (listed) return '<div class="dm-empty-inner">' + glyph('stacks', 'dm-empty-glyph') + '<h3>' + esc(t('empty_stacks_title')) + '</h3><p>' + esc(t('empty_stacks_body')) + '</p>'
                    + (snap.canMutate ? button('new-stack', t('new_stack'), { primary: true, glyph: 'plus' }) : '') + '</div>';
                return '';
            }
            if (loading && !shown) parts.push(spinner());
            parts.push(ignoredNote(snap));
            return parts.join('');
        }

        // ---- the detail pane ----------------------------------------------------------------------------------------------

        function serviceRow(snap, hostId, service, host) {
            const tone = sm.serviceTone(service), state = sm.serviceStateId(service);
            const exit = state === 'exited' && service.exit_code != null ? ' · ' + t('svc_exit', { code: service.exit_code }) : '';
            const health = service.health ? ' · ' + t('svc_health_' + (['healthy', 'unhealthy', 'starting'].includes(service.health) ? service.health : 'none')) : '';
            const links = sm.portLinks(service.ports, sm.linkAddress(host, snap.pageHost));
            const ports = links.length ? '<span class="dm-svc-ports">' + links.map(link => (link.url
                ? '<a class="dm-portlink" href="' + esc(link.url) + '" target="_blank" rel="noopener noreferrer" title="' + esc(link.url) + '">' + esc(link.label) + '</a>'
                : '<span class="dm-port" title="' + esc(t('port_not_linked')) + '">' + esc(link.label) + '</span>')).join('') + '</span>' : '';
            const open = service.container_id && snap.state.byHost[hostId] && snap.state.byHost[hostId].containers.some(c => c && c.id === service.container_id)
                ? iconButton('open-service-container', t('svc_open_container', { name: service.name }), 'inspect', ' data-container-id="' + esc(service.container_id) + '"', { focus: 'svc:' + service.name }) : '';
            return '<li class="dm-svc" data-tone="' + tone + '">' + dot(tone) + '<div class="dm-svc-text"><span class="dm-name">' + esc(service.name) + '</span><span class="dm-sub" title="' + esc(service.image) + '">'
                + esc([t('svc_state_' + state) + health + exit, model.shortImage(service.image)].filter(Boolean).join(' · ')) + '</span></div>' + ports + open + '</li>';
        }

        function overview(snap) {
            const stack = snap.state.stack, detail = stack && stack.detail;
            if (!detail) return '';
            const host = snap.hosts.find(item => item.id === stack.hostId) || {};
            const services = Array.isArray(detail.service_list) ? detail.service_list : [];
            const parts = [];
            if (detail.parse_error) parts.push('<p class="dm-note">' + glyph('warning') + '<span>' + esc(t('parse_error', { message: detail.parse_error })) + '</span></p>');
            if (!services.length) parts.push('<p class="dm-muted">' + esc(sm.stackStatusId(detail) === 'draft' ? t('overview_draft') : t('overview_no_services')) + '</p>');
            else parts.push('<ul class="dm-svclist" aria-label="' + esc(t('overview_services')) + '">' + services.map(service => serviceRow(snap, stack.hostId, service, host)).join('') + '</ul>');
            const facts = [
                '<div class="dm-fact"><dt>' + esc(t('ov_path')) + '</dt><dd>' + esc(detail.dir) + '</dd></div>',
                '<div class="dm-fact"><dt>' + esc(t('ov_files')) + '</dt><dd>' + esc(detail.compose_file || FILE_NAMES.compose) + (detail.has_env ? ' · .env' : '') + '</dd></div>'
            ];
            parts.push('<dl class="dm-facts">' + facts.join('') + '</dl>');
            return parts.join('');
        }

        // Header of the pane: name, host, path, age, the two icon buttons, the accent button and the more menu.
        function head(snap) {
            const stack = snap.state.stack;
            if (!stack) return '';
            const detail = stack.detail, summary = detail || { name: stack.name, status: 'stopped', services: { total: 0, running: 0 } };
            const status = sm.stackStatusId(summary), actions = sm.stackActions(summary), tone = sm.STACK_TONE[status];
            const host = snap.hosts.find(item => item.id === stack.hostId) || { id: stack.hostId, name: stack.hostId };
            const op = snap.stackOps[stack.key] || null, busy = !!op;
            const can = snap.canMutate && !!detail;
            const attrs = ' data-stack-key="' + esc(stack.key) + '"';
            const buttons = can ? iconButton('stack-op', t('action_restart') + ': ' + stack.name, 'restart', attrs + ' data-op="restart"', { focus: 'sd:restart', disabled: busy || !actions.restart })
                + iconButton('stack-op', t('action_stop') + ': ' + stack.name, 'stop', attrs + ' data-op="stop"', { focus: 'sd:stop', disabled: busy || !actions.stop })
                + button('stack-op', t('action_' + actions.primary), { primary: true, glyph: actions.primary === 'update' ? 'refresh' : 'deploy', attrs: attrs + ' data-op="' + actions.primary + '"', focus: 'sd:primary', disabled: busy, busy })
                + iconButton('stack-more', t('action_more'), 'more', attrs, { focus: 'sd:more' }) : '';
            const meta = [detail ? detail.dir : '', detail ? ageText(detail.created, snap.now) : ''].filter(Boolean).join(' · ');
            const tabs = sm.STACK_TABS.map(tab => '<button type="button" role="tab" class="dm-tab" id="' + uid + '-stab-' + tab + '" data-action="stack-tab" data-tab="' + tab + '" data-focus-id="stab:' + tab + '" aria-selected="' + (stack.tab === tab)
                + '" aria-controls="' + uid + '-spanel-' + tab + '" tabindex="' + (stack.tab === tab ? '0' : '-1') + '">' + esc(t('stab_' + tab)) + (snap.dirty[tab] ? '<span class="dm-dirty" aria-hidden="true">●</span><span class="dm-sr-only">' + esc(t('unsaved')) + '</span>' : '') + '</button>').join('');
            return '<header class="dm-sd-head"><div class="dm-sd-title">' + dot(busy ? 'info' : tone) + '<h2 id="' + uid + '-stack-title">' + esc(stack.name) + '</h2>' + chip(host.name, model.colorOf(host.color))
                + '<span class="dm-sd-status">' + esc(statusText(summary)) + '</span>' + opBadge(op && op.op) + '</div>'
                + '<div class="dm-sd-actions">' + buttons + iconButton('close-stack', t('drawer_close'), 'close', '', { focus: 'sd:close' }) + '</div>'
                + (meta ? '<p class="dm-sd-meta" title="' + esc(meta) + '">' + esc(meta) + '</p>' : '') + '</header>'
                + '<div class="dm-tabs" role="tablist" aria-label="' + esc(t('tabs_label')) + '">' + tabs + '</div>';
        }

        // The panels the editors live in. Rendered once per opened stack; docker-manager-editor.js fills the surfaces and
        // keeps the toolbars (Save, Save and deploy) in step with the drafts.
        function shell(stack) {
            const bar = file => '<div class="dm-ed-bar"><span class="dm-ed-file">' + esc(FILE_NAMES[file]) + '</span><span class="dm-ed-state" data-ed-state="' + file + '" role="status"></span><span class="dm-spacer"></span>'
                + '<button type="button" class="dm-btn" data-action="save-file" data-file="' + file + '" data-focus-id="save:' + file + '" disabled><span>' + esc(t('editor_save')) + '</span></button>'
                + '<button type="button" class="dm-btn is-primary" data-action="save-deploy" data-file="' + file + '" data-focus-id="savedeploy:' + file + '" disabled>' + glyph('deploy') + '<span>' + esc(t('editor_save_deploy')) + '</span></button></div>';
            const panel = (tab, inner, hidden) => '<div role="tabpanel" class="dm-sd-panel" id="' + uid + '-spanel-' + tab + '" aria-labelledby="' + uid + '-stab-' + tab + '" data-panel="' + tab + '"' + (hidden ? ' hidden' : '') + '>' + inner + '</div>';
            return panel('overview', '<div class="dm-sd-overview" data-stack-overview></div>', stack.tab !== 'overview')
                + panel('compose', bar('compose') + '<div class="dm-ed-surface" data-ed-surface="compose"></div>', stack.tab !== 'compose')
                + panel('env', '<p class="dm-hint dm-env-note" data-env-note></p>' + bar('env') + '<div class="dm-ed-surface" data-ed-surface="env"></div>', stack.tab !== 'env');
        }

        // Loading, failed or vanished stack: the pane says so instead of the shell.
        function problem(snap) {
            const stack = snap.state.stack;
            if (!stack || stack.status === 'ok') return '';
            const head2 = '<header class="dm-sd-head"><div class="dm-sd-title"><h2 id="' + uid + '-stack-title">' + esc(stack.name) + '</h2></div><div class="dm-sd-actions">' + iconButton('close-stack', t('drawer_close'), 'close', '', { focus: 'sd:close' }) + '</div></header>';
            if (stack.status === 'loading') return head2 + spinner();
            const gone = stack.error && stack.error.code === 'stack_not_found';
            return head2 + '<div class="dm-empty-inner"><p class="dm-error" role="alert">' + esc(gone ? t('stack_gone') : t('stack_load_failed') + ' ' + views.errorText(stack.error)) + '</p>'
                + (gone ? '' : button('retry-stack', t('banner_retry'), { glyph: 'refresh' })) + '</div>';
        }

        return { list, head, shell, overview, problem, statusText, ageText };
    }

    window.DockerManagerStacks = { create, FILE_NAMES };
})();
