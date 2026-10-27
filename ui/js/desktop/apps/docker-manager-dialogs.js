/* Docker Manager dialogs: one native <dialog> (provided by docker-manager.js) that shows a stack of views: the host
   manager list, the add/edit form with "Test connection", the import-from-nest list and the confirmations (remove a
   container, act on a protected one, delete a host). Escape or Back pops one view; the close button and a click on the
   backdrop close all. A classic script (window.DockerManagerDialogs) with no DOM work at load. Secrets typed into the
   form live only in the form fields: they are sent with Save or Test and never rendered back. */
(function () {
    'use strict';

    const PROBLEM_INPUT = { name: 'name', address: 'address', port: 'port', sshUser: 'ssh_user', sshSecret: 'ssh_password', tlsCert: 'tls_cert', stacksDir: 'stacks_dir', socketPath: 'host_socket_path' };
    const SERVER_FIELD_INPUT = { name: 'name', address: 'address', port: 'port', ssh_user: 'ssh_user', ssh_secret: 'ssh_password', stacks_dir: 'stacks_dir', host_socket_path: 'host_socket_path', kind: 'kind' };
    const RUNNING_STATES = ['running', 'paused', 'restarting'];

    function create(app) {
        const { t, dt, esc, model, store, root, views, toast, announce, uid } = app;
        const dialog = root.querySelector('[data-dialog]'), content = root.querySelector('[data-dialog-content]');
        const stack = [];
        const testing = new Set();
        let opener = null, disposed = false;
        const fieldId = name => uid + '-f-' + name;

        // ---- the view stack --------------------------------------------------------------------------------------

        const top = () => stack[stack.length - 1];

        function paint() {
            const view = top();
            if (!view) return;
            content.innerHTML = '<h2 class="dm-dialog-title" id="' + uid + '-dialog-title">' + esc(view.title) + '</h2><div class="dm-view" data-view>' + view.html() + '</div>';
            dialog.classList.toggle('is-wide', !!view.wide);
            // Listeners go on the per-paint view element, which the next paint replaces together with them.
            const body = content.querySelector('[data-view]');
            view.bind?.(body);
            const first = body.querySelector('[data-autofocus]') || body.querySelector('input:not([type="hidden"]):not([type="radio"]), select, textarea, button');
            first?.focus({ preventScroll: true });
        }
        function push(view) {
            if (disposed) return null;
            stack.push(view);
            if (!dialog.open) { opener = document.activeElement; dialog.showModal(); }
            paint();
            return view;
        }
        function pop(result) {
            const view = stack.pop();
            view?.onClose?.(result);
            if (stack.length) paint(); else if (dialog.open) dialog.close();
        }
        function closeAll() {
            while (stack.length) stack.pop().onClose?.(false);
            if (dialog.open) dialog.close();
        }
        // Repaints a part of the top view in place and keeps the focus on the same control.
        function refreshPart(view, selector, html) {
            if (top() !== view) return;
            const part = content.querySelector(selector);
            if (!part) return;
            const focusId = document.activeElement?.dataset?.focusId;
            part.innerHTML = html;
            if (focusId) content.querySelector('[data-focus-id="' + CSS.escape(focusId) + '"]')?.focus({ preventScroll: true });
        }

        dialog.addEventListener('cancel', event => { event.preventDefault(); if (stack.length) pop(false); });
        dialog.addEventListener('close', () => {
            while (stack.length) stack.pop().onClose?.(false);
            const target = opener;
            opener = null;
            app.restoreFocus(target);
        });
        dialog.addEventListener('click', event => {
            if (event.target === dialog || event.target.closest('[data-dialog-dismiss]')) closeAll();
        });

        const actions = (left, right) => '<div class="dm-dialog-actions">' + (left || '') + '<span class="dm-spacer"></span>' + (right || '') + '</div>';
        const button = (act, label, o) => '<button type="button" class="dm-btn' + (o && o.primary ? ' is-primary' : '') + (o && o.danger ? ' is-danger-solid' : '') + '" data-dact="' + act + '"'
            + (o && o.id ? ' data-host-id="' + esc(o.id) + '"' : '') + (o && o.focusId ? ' data-focus-id="' + esc(o.focusId) + '"' : '') + (o && o.auto ? ' data-autofocus' : '') + (o && o.disabled ? ' disabled aria-busy="true"' : '') + '>'
            + (o && o.glyph ? views.glyph(o.glyph) : '') + '<span>' + esc(label) + '</span></button>';

        // ---- confirmations ---------------------------------------------------------------------------------------

        // Resolves true on the confirm button, false on Cancel, Escape, the close button or the backdrop. Cancel has the focus.
        function confirm(o) {
            return new Promise(resolve => {
                let settled = false;
                const settle = value => { if (!settled) { settled = true; resolve(value); } };
                push({
                    title: o.title,
                    html: () => '<p class="dm-dialog-copy">' + esc(o.message) + '</p>'
                        + (o.note ? '<p class="dm-note">' + views.glyph('lock') + '<span>' + esc(o.note) + '</span></p>' : '')
                        + actions('', button('no', dt('desktop.cancel'), { auto: true }) + button('yes', o.confirmLabel, { primary: !o.danger, danger: !!o.danger })),
                    bind(c) {
                        c.querySelector('[data-dact="no"]').addEventListener('click', () => { settle(false); pop(false); });
                        c.querySelector('[data-dact="yes"]').addEventListener('click', () => { settle(true); pop(true); });
                    },
                    onClose() { settle(false); }
                });
            });
        }

        function confirmRemove(row) {
            const c = row.c, name = model.containerName(c);
            const running = RUNNING_STATES.includes(String(c.state).toLowerCase());
            return confirm({
                title: t('remove_title'), message: running ? t('remove_body_running', { name, host: row.hostName }) : t('remove_body', { name, host: row.hostName }),
                note: views.protectedText(c), confirmLabel: t('action_remove'), danger: true
            });
        }
        // `info` is the server's 409 container_protected_confirmation_required answer when there is one: its message and owner
        // say what is protected; without it the client's own knowledge of the container explains the lock.
        function confirmProtectedAction(row, action, info) {
            const name = model.containerName(row.c), label = views.actionLabel(action), answer = info || {};
            const note = answer.message || (answer.owner ? t('protected_owner', { owner: answer.owner }) : '') || views.protectedText(row.c);
            return confirm({
                title: t('protected_confirm_title'), message: t('protected_confirm_body', { name, action: label }),
                note, confirmLabel: label, danger: true
            });
        }

        // ---- host manager --------------------------------------------------------------------------------------------

        function badge(text, tone) { return '<span class="dm-badge"' + (tone ? ' data-tone="' + tone + '"' : '') + '>' + esc(text) + '</span>'; }

        function hostRow(host, canEdit) {
            const entry = store.state.byHost[host.id], health = model.hostHealth(host, entry);
            const status = host.status || {};
            const word = health.state === 'error' ? t('host_error') + ': ' + views.reasonText(health.reason) : t('host_' + health.state);
            const meta = [t('kind_' + host.kind), model.hostAddress(host)];
            if (status.engine_version) meta.push(t('host_engine', { version: status.engine_version }));
            const badges = (host.builtin ? badge(t('host_builtin')) : '') + (host.plaintext ? badge(t('host_plain_badge'), 'warn') : '') + (host.secret_set ? badge(t('host_secret_set')) : '');
            const busy = testing.has(host.id);
            return '<li class="dm-hostrow" data-host-id="' + esc(host.id) + '"><span class="dm-swatch" data-host-color="' + esc(model.colorOf(host.color)) + '" aria-hidden="true"></span>'
                + '<div class="dm-hostrow-main"><div class="dm-hostrow-name"><strong>' + esc(host.name) + '</strong>' + badges + '</div>'
                + '<div class="dm-hostrow-sub">' + esc(meta.join(' · ')) + '</div>'
                + '<div class="dm-hostrow-status">' + views.dot(health.tone) + '<span' + (health.message ? ' title="' + esc(health.message) + '"' : '') + '>' + esc(word) + '</span></div></div>'
                + '<div class="dm-hostrow-actions">' + button('test', busy ? t('f_testing') : t('host_test'), { id: host.id, focusId: 'test:' + host.id, disabled: busy })
                + (canEdit && !host.builtin ? button('edit', t('host_edit'), { id: host.id, focusId: 'edit:' + host.id }) + button('delete', t('host_delete'), { id: host.id, focusId: 'delete:' + host.id }) : '') + '</div></li>';
        }

        function openHosts() {
            let unsubscribe = null;
            const canEdit = () => store.canEditHosts();
            const listHtml = () => store.state.hosts.map(host => hostRow(host, canEdit())).join('');
            const view = {
                title: t('rail_label'), wide: true,
                html: () => '<p class="dm-dialog-copy">' + esc(t('hosts_intro')) + '</p><ul class="dm-hostlist" data-hostlist>' + listHtml() + '</ul>'
                    + actions(canEdit() ? button('nests', t('host_import'), { glyph: 'download' }) : '', (canEdit() ? button('add', t('host_add'), { primary: true, glyph: 'plus', auto: true }) : '') + button('close', dt('desktop.close'))),
                update() { refreshPart(view, '[data-hostlist]', listHtml()); },
                bind(c) {
                    unsubscribe?.();
                    unsubscribe = store.subscribe(() => view.update());
                    c.addEventListener('click', event => {
                        const el = event.target.closest('[data-dact]');
                        if (!el) return;
                        const id = el.dataset.hostId, host = store.state.hosts.find(item => item.id === id);
                        switch (el.dataset.dact) {
                            case 'close': closeAll(); break;
                            case 'add': openHostForm(null); break;
                            case 'nests': openNests(); break;
                            case 'edit': if (host) openHostForm(host); break;
                            case 'test': if (host) void testRow(host, view); break;
                            case 'delete': if (host) void deleteRow(host); break;
                            default: break;
                        }
                    });
                },
                onClose() { unsubscribe?.(); unsubscribe = null; }
            };
            return push(view);
        }

        async function testRow(host, view) {
            testing.add(host.id);
            view.update();
            try {
                const updated = await store.testHost(host.id);
                void store.reloadContainers(host.id);
                announce(updated.status && updated.status.state === 'ok' ? t('f_test_ok', { version: updated.status.engine_version || '', api: updated.status.api_version || '', os: updated.status.os || '', arch: updated.status.arch || '' }) : t('host_error') + ': ' + host.name);
            } catch (error) {
                toast(views.errorText(error.dm), 'error');
            } finally {
                testing.delete(host.id);
                view.update();
            }
        }

        async function deleteRow(host) {
            const ok = await confirm({ title: t('host_delete_title'), message: t('host_delete_body', { name: host.name }), confirmLabel: t('host_delete'), danger: true });
            if (!ok || disposed) return;
            try {
                await store.deleteHost(host.id);
                toast(t('host_deleted', { name: host.name }), 'success');
                announce(t('host_deleted', { name: host.name }));
            } catch (error) {
                toast(views.errorText(error.dm), 'error');
            }
        }

        // ---- add / edit form ----------------------------------------------------------------------------------------

        function formHtml(form) {
            const editing = !!form.id, stored = model.storedSecretUsable(form);
            const input = (name, value, attrs, hint) => '<input id="' + fieldId(name) + '" name="' + name + '" value="' + esc(value) + '"' + (/autocomplete=/.test(attrs || '') ? '' : ' autocomplete="off"') + ' spellcheck="false" autocapitalize="off"' + (hint ? ' aria-describedby="' + fieldId(name) + '-hint"' : '') + ' ' + (attrs || '') + '>';
            const area = (name, placeholder, hint) => '<textarea id="' + fieldId(name) + '" name="' + name + '" rows="3" autocomplete="off" spellcheck="false" autocapitalize="off" placeholder="' + esc(placeholder || '') + '"' + (hint ? ' aria-describedby="' + fieldId(name) + '-hint"' : '') + '></textarea>';
            const field = (name, label, control, hint, cls, forKinds) => '<div class="dm-field' + (cls ? ' ' + cls : '') + '"' + (forKinds ? ' data-for="' + forKinds + '"' : '') + '><label for="' + fieldId(name) + '">' + esc(label) + '</label>' + control
                + (hint ? '<small class="dm-hint" id="' + fieldId(name) + '-hint">' + esc(hint) + '</small>' : '') + '</div>';
            const kinds = model.HOST_KINDS.map(kind => '<option value="' + kind + '"' + (kind === form.kind ? ' selected' : '') + '>' + esc(t('kind_' + kind)) + '</option>').join('');
            const colors = model.HOST_COLORS.map(color => '<label class="dm-swatch-opt"><input type="radio" name="color" value="' + color + '"' + (color === form.color ? ' checked' : '') + ' class="dm-sr-only"><span class="dm-swatch is-pick" data-host-color="' + color + '"></span><span class="dm-sr-only">' + esc(t('color_' + color)) + '</span></label>').join('');
            const keepHint = stored ? t('f_secret_stored') : '';
            return '<form class="dm-form" data-host-form novalidate>'
                + field('name', t('col_name'), input('name', form.name, 'maxlength="64" required data-autofocus'))
                + '<div class="dm-field-row">' + field('kind', t('f_kind'), '<select id="' + fieldId('kind') + '" name="kind">' + kinds + '</select>') + field('port', t('f_port'), input('port', form.port, 'inputmode="numeric" maxlength="5" data-port'), '', 'is-narrow') + '</div>'
                + field('address', t('f_address'), input('address', form.address, 'required', true), t('f_address_hint'))
                + '<div class="dm-warn" role="note" data-for="tcp">' + views.glyph('warning') + '<span>' + esc(t('warn_plain_tcp')) + '</span></div>'
                + field('ssh_user', t('f_ssh_user'), input('ssh_user', form.sshUser, ''), '', '', 'ssh')
                // A host with stored credentials shows "stored" and a Replace action; the secret fields appear only after Replace.
                + '<div class="dm-stored" data-for="ssh tls mtls" data-stored-row hidden>' + views.dot('ok') + '<span>' + esc(t('host_secret_set')) + '</span><button type="button" class="dm-btn" data-dact="replace" data-focus-id="replace-secret"><span>' + esc(t('f_replace')) + '</span></button></div>'
                + '<div class="dm-group" data-for="ssh" data-secret>'
                + field('ssh_password', t('f_ssh_password'), input('ssh_password', '', 'type="password" autocomplete="new-password"', !!keepHint), keepHint)
                + field('ssh_key', t('f_ssh_key'), area('ssh_key', '')) + '</div>'
                + '<div class="dm-group" data-for="tls mtls" data-secret>' + field('tls_ca', t('f_tls_ca'), area('tls_ca', '', true), t('f_tls_hint'))
                + '<div data-for="mtls">' + field('tls_cert', t('f_tls_cert'), area('tls_cert', '')) + field('tls_key', t('f_tls_key'), area('tls_key', '')) + '</div>'
                + (stored ? '<small class="dm-hint">' + esc(t('f_secret_stored')) + '</small>' : '') + '</div>'
                + '<fieldset class="dm-colors"><legend>' + esc(t('f_color')) + '</legend><div class="dm-color-row">' + colors + '</div></fieldset>'
                + '<details class="dm-advanced"' + (form.stacksDir !== model.DEFAULT_STACKS_DIR || form.socketPath !== model.DEFAULT_SOCKET ? ' open' : '') + '><summary>' + esc(t('f_advanced')) + '</summary><p class="dm-hint">' + esc(t('f_advanced_hint')) + '</p>'
                + field('stacks_dir', t('f_stacks_dir'), input('stacks_dir', form.stacksDir, '')) + field('host_socket_path', t('f_socket'), input('host_socket_path', form.socketPath, '')) + '</details>'
                + (editing && form.secretSet ? '<label class="dm-check" data-for="ssh tls mtls"><input type="checkbox" name="clear_secret"' + (form.clearSecret ? ' checked' : '') + '><span>' + esc(t('f_clear_secret')) + '</span></label>' : '')
                + '<div class="dm-test-row"><button type="button" class="dm-btn" data-dact="test-form" data-focus-id="test-form">' + views.glyph('check') + '<span>' + esc(t('f_test')) + '</span></button><span class="dm-hint">' + esc(t('f_test_hint')) + '</span></div>'
                + '<div class="dm-test-result" role="status" aria-live="polite" data-test-result></div>'
                + '<p class="dm-form-error" role="alert" data-form-error hidden></p>'
                + actions(button('back', dt('desktop.back')), '<button type="submit" class="dm-btn is-primary" data-focus-id="save-form"><span>' + esc(dt('desktop.save')) + '</span></button>') + '</form>';
        }

        function readForm(c, base) {
            const f = c.querySelector('[data-host-form]'), v = name => (f.elements[name] ? f.elements[name].value : '');
            const key = v('ssh_key').trim();
            const checked = f.querySelector('input[name="color"]:checked');
            return Object.assign({}, base, {
                name: v('name'), kind: v('kind'), address: v('address'), port: v('port'), sshUser: v('ssh_user'), sshSecret: key || v('ssh_password'),
                tlsCA: v('tls_ca'), tlsCert: v('tls_cert'), tlsKey: v('tls_key'), stacksDir: v('stacks_dir'), socketPath: v('host_socket_path'),
                color: checked ? checked.value : base.color, clearSecret: !!(f.elements.clear_secret && f.elements.clear_secret.checked)
            });
        }

        function showProblem(c, inputName, message) {
            for (const el of c.querySelectorAll('[aria-invalid]')) el.removeAttribute('aria-invalid');
            const error = c.querySelector('[data-form-error]');
            error.textContent = message;
            error.hidden = !message;
            const el = inputName ? c.querySelector('[name="' + inputName + '"]') : null;
            if (el) { el.setAttribute('aria-invalid', 'true'); el.focus({ preventScroll: false }); }
        }

        // Shows what the chosen connection needs. ui.replacing is true once the admin pressed Replace on stored credentials.
        function syncKind(c, base, ui) {
            const f = c.querySelector('[data-host-form]'), kind = f.elements.kind.value;
            for (const el of f.querySelectorAll('[data-for]')) el.hidden = !el.dataset.for.split(' ').includes(kind);
            f.elements.port.placeholder = String(model.defaultPort(kind));
            const form = readForm(c, base), stored = model.storedSecretUsable(form), tucked = stored && !ui.replacing;
            for (const el of f.querySelectorAll('[data-secret]')) el.hidden = el.hidden || tucked;
            for (const el of f.querySelectorAll('[data-stored-row]')) el.hidden = el.hidden || !tucked;
        }

        // The secret inputs are tucked away behind Replace while credentials are stored; a problem on one of them opens them.
        function revealSecrets(c, base, ui, inputName) {
            if (!ui.replacing && ['ssh_password', 'ssh_key', 'tls_ca', 'tls_cert', 'tls_key'].includes(inputName)) { ui.replacing = true; syncKind(c, base, ui); }
        }

        function resultHtml(status) {
            const s = model.normalizeStatus(status);
            if (s.state === 'ok') {
                const text = t('f_test_ok', { version: s.engine_version || '?', api: s.api_version || '?', os: s.os || '?', arch: s.arch || '?' });
                return '<span class="dm-result is-ok">' + views.dot('ok') + '<span>' + esc(text) + '</span></span>';
            }
            return '<span class="dm-result is-bad">' + views.dot('bad') + '<span>' + esc(t('f_test_failed', { reason: views.reasonText(s.code) })) + '</span></span>'
                + (s.message ? '<details class="dm-details"><summary>' + esc(t('banner_details')) + '</summary><pre class="dm-pre">' + esc(s.message) + '</pre></details>' : '');
        }

        // A test of an unreachable host can take 30 s. Only the Test button is busy; the rest of the form stays usable.
        async function testForm(c, base, ui) {
            const form = readForm(c, base), out = c.querySelector('[data-test-result]'), btn = c.querySelector('[data-dact="test-form"]');
            const problem = model.validateHostForm(form, store.state.hosts);
            if (problem) { revealSecrets(c, base, ui, PROBLEM_INPUT[problem.field]); showProblem(c, PROBLEM_INPUT[problem.field], t('v_' + problem.code)); return; }
            showProblem(c, '', '');
            btn.disabled = true;
            btn.setAttribute('aria-busy', 'true');
            out.innerHTML = '<span class="dm-muted">' + views.glyph('refresh', 'dm-working') + ' ' + esc(t('f_testing')) + '</span>';
            try {
                const status = await store.testDraft(model.hostPayload(form), form.id);
                if (out.isConnected) out.innerHTML = resultHtml(status);
            } catch (error) {
                if (out.isConnected) out.innerHTML = '<span class="dm-result is-bad">' + views.dot('bad') + '<span>' + esc(views.errorText(error.dm)) + '</span></span>';
            } finally {
                btn.disabled = false;
                btn.removeAttribute('aria-busy');
            }
        }

        // The server's field names for a rejected host; ssh_secret and tls are errors on the secret inputs.
        function serverInput(info, kind) {
            if (info.field === 'tls') return kind === 'mtls' ? 'tls_cert' : 'tls_ca';
            return SERVER_FIELD_INPUT[info.field] || '';
        }

        async function saveForm(c, base, ui) {
            const form = readForm(c, base), submit = c.querySelector('[type="submit"]');
            const problem = model.validateHostForm(form, store.state.hosts);
            if (problem) { revealSecrets(c, base, ui, PROBLEM_INPUT[problem.field]); showProblem(c, PROBLEM_INPUT[problem.field], t('v_' + problem.code)); return; }
            showProblem(c, '', '');
            submit.disabled = true;
            submit.setAttribute('aria-busy', 'true');
            try {
                const saved = await store.saveHost(form.id, model.hostPayload(form));
                pop(true);
                toast(t('host_saved', { name: saved.name }), 'success');
                announce(t('host_saved', { name: saved.name }));
            } catch (error) {
                submit.disabled = false;
                submit.removeAttribute('aria-busy');
                const info = error.dm || {};
                const input = info.code === 'invalid_host' ? serverInput(info, form.kind) : '';
                revealSecrets(c, base, ui, input);
                showProblem(c, input, views.errorText(info));
            }
        }

        function openHostForm(host) {
            if (!store.canEditHosts()) { toast(t('readonly_chip'), 'error'); return null; }
            const base = host ? model.hostToForm(host) : model.emptyHostForm(store.state.hosts);
            return push({
                title: host ? t('host_edit_title') : t('host_add'),
                html: () => formHtml(base),
                bind(c) {
                    const f = c.querySelector('[data-host-form]'), ui = { replacing: false };
                    syncKind(c, base, ui);
                    f.addEventListener('input', () => syncKind(c, base, ui));
                    f.addEventListener('change', () => syncKind(c, base, ui));
                    f.addEventListener('submit', event => { event.preventDefault(); void saveForm(c, base, ui); });
                    f.querySelector('[data-dact="back"]').addEventListener('click', () => pop(false));
                    f.querySelector('[data-dact="test-form"]').addEventListener('click', () => void testForm(c, base, ui));
                    f.querySelector('[data-dact="replace"]').addEventListener('click', () => {
                        ui.replacing = true;
                        syncKind(c, base, ui);
                        const first = f.querySelector('[data-secret]:not([hidden]) input, [data-secret]:not([hidden]) textarea');
                        if (first) first.focus();
                    });
                }
            });
        }

        // ---- import from nest -----------------------------------------------------------------------------------------

        function openNests() {
            const data = { status: 'loading', nests: [], error: null, importing: '' };
            const rowHtml = nest => {
                const method = model.NEST_METHODS.includes(nest.deploy_method) ? t('nest_method_' + nest.deploy_method) : String(nest.deploy_method || '');
                const meta = [method, nest.host ? nest.host + (nest.port ? ':' + nest.port : '') : '', nest.docker_tls ? t('kind_' + (nest.docker_tls === 'mtls' ? 'mtls' : 'tls')) : ''].filter(Boolean);
                const busy = data.importing === nest.id;
                return '<li class="dm-hostrow"><div class="dm-hostrow-main"><div class="dm-hostrow-name"><strong>' + esc(nest.name) + '</strong></div><div class="dm-hostrow-sub">' + esc(meta.join(' · ')) + '</div></div>'
                    + '<div class="dm-hostrow-actions">' + button('import', busy ? t('nest_importing') : t('nest_import'), { id: nest.id, focusId: 'import:' + nest.id, disabled: !!data.importing }) + '</div></li>';
            };
            const bodyHtml = () => {
                if (data.status === 'loading') return '<p class="dm-muted" role="status">' + esc(t('loading')) + '</p>';
                if (data.status === 'error') return '<p class="dm-error" role="alert">' + esc(views.errorText(data.error)) + '</p>';
                if (!data.nests.length) return '<p class="dm-muted">' + esc(t('nest_empty')) + '</p>';
                return '<ul class="dm-hostlist">' + data.nests.map(rowHtml).join('') + '</ul>';
            };
            const view = {
                title: t('nest_title'), wide: true,
                html: () => '<p class="dm-dialog-copy">' + esc(t('nest_intro')) + '</p><div data-nestlist>' + bodyHtml() + '</div>' + actions(button('back', dt('desktop.back'), { auto: true }), ''),
                update() { refreshPart(view, '[data-nestlist]', bodyHtml()); },
                bind(c) {
                    c.addEventListener('click', event => {
                        const el = event.target.closest('[data-dact]');
                        if (!el) return;
                        if (el.dataset.dact === 'back') pop(false);
                        if (el.dataset.dact === 'import' && !data.importing) void importNest(el.dataset.hostId);
                    });
                }
            };
            async function importNest(id) {
                data.importing = id;
                view.update();
                try {
                    const host = await store.importNest(id);
                    pop(true);
                    toast(t('nest_imported', { name: host.name }), 'success');
                    announce(t('nest_imported', { name: host.name }));
                } catch (error) {
                    data.importing = '';
                    data.status = 'error';
                    data.error = error.dm;
                    view.update();
                }
            }
            push(view);
            store.loadNests().then(nests => { data.status = 'ok'; data.nests = nests; view.update(); },
                error => { data.status = 'error'; data.error = error.dm; view.update(); });
            return view;
        }

        function dispose() {
            if (disposed) return;
            closeAll();
            disposed = true;
        }

        return { openHosts, openHostForm, openNests, confirm, confirmRemove, confirmProtectedAction, closeAll, dispose, isOpen: () => dialog.open };
    }

    window.DockerManagerDialogs = { create };
})();
