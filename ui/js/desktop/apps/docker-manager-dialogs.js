/* Docker Manager dialogs: one native <dialog> (provided by docker-manager.js) that shows a stack of views: the host
   manager list, the add/edit form with "Test connection", the import-from-nest list and the confirmations (remove a
   container, act on a protected one, delete a host) and, for stacks, the destructive confirmations (delete a stack, down
   with volumes: they list what disappears), the revision-conflict dialog with a line diff, and the prepare-host question.
   Escape and Back pop one view; the close button and a click on the backdrop close all. All four go through requestClose():
   a view that holds unsaved input asks first, inline in its own form, so the form is never repainted and what was typed
   stays. A classic script (window.DockerManagerDialogs) with no DOM work at load. Secrets typed into the form live only in
   the form fields: they are sent with Save or Test and never rendered back. */
(function () {
    'use strict';

    const PROBLEM_INPUT = { name: 'name', address: 'address', port: 'port', sshUser: 'ssh_user', sshSecret: 'ssh_password', tlsCert: 'tls_cert', stacksDir: 'stacks_dir', socketPath: 'host_socket_path' };
    const SERVER_FIELD_INPUT = { name: 'name', address: 'address', port: 'port', ssh_user: 'ssh_user', ssh_secret: 'ssh_password', stacks_dir: 'stacks_dir', host_socket_path: 'host_socket_path', kind: 'kind' };
    const RUNNING_STATES = ['running', 'paused', 'restarting'];

    function create(app) {
        const { t, dt, esc, model, stackModel: sm, store, root, views, toast, announce, uid } = app;
        const dialog = root.querySelector('[data-dialog]'), content = root.querySelector('[data-dialog-content]');
        const stack = [];
        const testing = new Set();
        let opener = null, disposed = false, asking = false;
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
        // A closed <dialog> keeps its DOM. Whatever was typed into the host form (SSH password, PEM key) must not stay in the
        // hidden page, so the content goes away whenever the stack is empty.
        function clearContent() { content.replaceChildren(); }
        function pop(result) {
            const view = stack.pop();
            view?.onClose?.(result);
            if (stack.length) { paint(); return; }
            if (dialog.open) dialog.close();
            clearContent();
        }
        function closeAll() {
            while (stack.length) stack.pop().onClose?.(false);
            if (dialog.open) dialog.close();
            clearContent();
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

        dialog.addEventListener('cancel', event => { event.preventDefault(); requestClose(false, true); });
        // Escape is answered here as well: a second Escape without a click in between is not cancelable, so the browser would
        // close the dialog whatever the cancel handler says, together with what was typed. Preventing the keydown stops that.
        dialog.addEventListener('keydown', event => {
            if (event.key !== 'Escape' || event.defaultPrevented || event.isComposing) return;
            event.preventDefault();
            requestClose(false, true);
        });
        dialog.addEventListener('close', () => {
            while (stack.length) stack.pop().onClose?.(false);
            clearContent();
            const target = opener;
            opener = null;
            app.restoreFocus(target);
        });
        // The one door for Escape, Back, the close button and the backdrop (all: the whole dialog, else the top view). A view with
        // unsaved input (view.dirty()) asks first, in its own markup (view.ask(leave), view.keepEditing()), so its DOM stays as it
        // is; a second Escape answers that question with "keep editing". Only a dirty view that is not on top falls back to a
        // confirmation view pushed above it.
        function requestClose(all, viaEscape) {
            const view = top();
            if (!view) return;
            if (viaEscape && view.keepEditing && view.keepEditing()) return;
            const leave = () => { if (all) closeAll(); else pop(false); };
            const holdsInput = candidate => !!candidate.dirty && candidate.dirty();
            if (!(all ? stack : [view]).some(holdsInput)) { leave(); return; }
            if (holdsInput(view) && view.ask) { view.ask(leave); return; }
            if (asking) return;
            asking = true;
            void confirm({ title: t('discard_title'), message: t('discard_dialog_body'), confirmLabel: t('discard_confirm'), danger: true }).then(discard => {
                asking = false;
                if (discard && !disposed) leave();
            });
        }
        dialog.addEventListener('click', event => {
            if (event.target.closest('[data-dialog-dismiss]') || event.target === dialog) requestClose(true);
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
        // `info` is the server's 409 container_protected_confirmation_required answer when there is one. Its owner says what is
        // protected and gets the same translated sentence as a lock the client already knew about; the server's message is
        // not shown (it is English and ends with the instruction to repeat the request with confirm=protected).
        function confirmProtectedAction(row, action, info) {
            const name = model.containerName(row.c), label = views.actionLabel(action), answer = info || {};
            const note = views.protectedLabelText(answer.owner) || views.protectedText(row.c);
            return confirm({
                title: t('protected_confirm_title'), message: t('protected_confirm_body', { name, action: label }),
                note, confirmLabel: label, danger: true
            });
        }

        // ---- stacks: destructive confirmations, conflicts, prepare ------------------------------------------------------

        function volumesHtml(data) {
            if (data.status === 'loading') return '<p class="dm-muted" role="status">' + esc(t('volumes_loading')) + '</p>';
            if (data.status === 'error') return '<p class="dm-error" role="alert">' + esc(t('volumes_failed')) + ' ' + esc(views.errorText(data.error)) + '</p>';
            if (!data.list.length) return '<p class="dm-muted">' + esc(t('volumes_none')) + '</p>';
            return '<ul class="dm-lose-list">' + data.list.map(volume => '<li><strong>' + esc(volume.name) + '</strong> <span class="dm-muted">'
                + esc([volume.driver, volume.size >= 0 ? sm.formatBytes(volume.size) : ''].filter(Boolean).join(' · ')) + '</span></li>').join('') + '</ul>';
        }

        // Shared by delete and down -v: loads the named volumes of the stack (GET .../volumes) and keeps them in data.
        function loadVolumes(info, data, view) {
            data.status = 'loading';
            view.update();
            store.loadVolumes(info.hostId, info.name).then(list => { data.status = 'ok'; data.list = list; view.update(); },
                error => { data.status = 'error'; data.error = error.dm; view.update(); });
        }

        // Delete a stack: names the folder and the volumes, offers to delete the volumes too, and needs the stack name typed.
        // Resolves {volumes: boolean} or null.
        function confirmDelete(info) {
            return new Promise(resolve => {
                let settled = false;
                const settle = value => { if (!settled) { settled = true; resolve(value); } };
                const data = { status: 'loading', list: [], error: null }, choice = { volumes: false };
                let closed = false;
                const view = {
                    title: t('delete_title', { name: info.name }),
                    html: () => '<p class="dm-dialog-copy">' + esc(t('delete_body', { name: info.name, host: info.hostName })) + '</p>'
                        + '<ul class="dm-lose-list"><li>' + views.glyph('folder') + ' <span>' + esc(t('delete_folder', { path: info.dir })) + '</span></li></ul>'
                        + '<h3 class="dm-lose-title">' + esc(t('volumes_title')) + '</h3><div data-volumes>' + volumesHtml(data) + '</div>'
                        + '<label class="dm-check"><input type="checkbox" name="volumes" data-focus-id="delete:volumes"><span>' + esc(t('delete_volumes_too')) + '</span></label>'
                        + '<p class="dm-hint" data-volumes-note>' + esc(t('delete_volumes_stay')) + '</p>'
                        + '<p class="dm-note">' + views.glyph('lock') + '<span>' + esc(t('delete_images_stay')) + '</span></p>'
                        + '<div class="dm-field"><label for="' + fieldId('confirm-name') + '">' + esc(t('delete_type', { name: info.name })) + '</label><input id="' + fieldId('confirm-name') + '" name="confirm-name" autocomplete="off" spellcheck="false" autocapitalize="off" data-autofocus></div>'
                        + actions('', button('no', dt('desktop.cancel')) + button('yes', t('delete_confirm'), { danger: true, disabled: true })),
                    update() {
                        // A late volume list must not reach a dialog that is closed or covered, nor the one that replaced it.
                        if (closed || top() !== view) return;
                        refreshPart(view, '[data-volumes]', volumesHtml(data));
                        const note = content.querySelector('[data-volumes-note]');
                        if (note) note.textContent = choice.volumes ? t('delete_volumes_go') : t('delete_volumes_stay');
                    },
                    bind(c) {
                        const yes = c.querySelector('[data-dact="yes"]'), typed = c.querySelector('[name="confirm-name"]'), box = c.querySelector('[name="volumes"]');
                        yes.removeAttribute('aria-busy');
                        typed.addEventListener('input', () => { yes.disabled = typed.value.trim() !== info.name; });
                        box.addEventListener('change', () => { choice.volumes = box.checked; view.update(); });
                        c.querySelector('[data-dact="no"]').addEventListener('click', () => { settle(null); pop(false); });
                        yes.addEventListener('click', () => { settle({ volumes: choice.volumes }); pop(true); });
                    },
                    onClose() { closed = true; settle(null); }
                };
                push(view);
                loadVolumes(info, data, view);
            });
        }

        // Down with volumes: lists the volumes that are deleted with the containers. Resolves true or false.
        function confirmDownVolumes(info) {
            return new Promise(resolve => {
                let settled = false;
                const settle = value => { if (!settled) { settled = true; resolve(value); } };
                const data = { status: 'loading', list: [], error: null };
                let closed = false;
                const view = {
                    title: t('down_volumes_title', { name: info.name }),
                    html: () => '<p class="dm-dialog-copy">' + esc(t('down_volumes_body', { name: info.name, host: info.hostName })) + '</p>'
                        + '<h3 class="dm-lose-title">' + esc(t('volumes_title')) + '</h3><div data-volumes>' + volumesHtml(data) + '</div>'
                        + '<p class="dm-note">' + views.glyph('lock') + '<span>' + esc(t('down_volumes_stay', { path: info.dir })) + '</span></p>'
                        + actions('', button('no', dt('desktop.cancel'), { auto: true }) + button('retry', t('banner_retry'), { glyph: 'refresh' }) + button('yes', t('down_volumes_confirm'), { danger: true, disabled: true })),
                    update() {
                        // The confirm button belongs to whatever dialog is open: only this dialog, while it is on top, may switch it.
                        if (closed || top() !== view) return;
                        refreshPart(view, '[data-volumes]', volumesHtml(data));
                        const yes = content.querySelector('[data-dact="yes"]'), retry = content.querySelector('[data-dact="retry"]');
                        if (yes) { yes.disabled = data.status !== 'ok'; yes.removeAttribute('aria-busy'); }
                        if (retry) retry.hidden = data.status !== 'error';
                    },
                    bind(c) {
                        c.querySelector('[data-dact="no"]').addEventListener('click', () => { settle(false); pop(false); });
                        c.querySelector('[data-dact="yes"]').addEventListener('click', () => { settle(true); pop(true); });
                        c.querySelector('[data-dact="retry"]').addEventListener('click', () => loadVolumes(info, data, view));
                        view.update();
                    },
                    onClose() { closed = true; settle(false); }
                };
                push(view);
                loadVolumes(info, data, view);
            });
        }

        // The file changed on the host since it was loaded: a line diff from the host version to yours. Resolves 'mine'
        // (save over it), 'theirs' (take the host version) or null (keep editing).
        function confirmConflict(info) {
            return new Promise(resolve => {
                let settled = false;
                const settle = value => { if (!settled) { settled = true; resolve(value); } };
                const rows = sm.lineDiff(info.theirs, info.mine), same = !sm.diffChanged(rows);
                push({
                    title: t('conflict_title', { file: info.name }), wide: true,
                    html: () => '<p class="dm-dialog-copy">' + esc(t('conflict_body', { file: info.name })) + '</p>'
                        + (same ? '<p class="dm-muted">' + esc(t('conflict_same')) + '</p>' : '<p class="dm-hint">' + esc(t('conflict_legend')) + '</p>' + views.diffHtml(rows))
                        + actions(button('no', t('conflict_keep'), { auto: true }), button('theirs', t('conflict_take_host')) + button('mine', t('conflict_overwrite'), { danger: true })),
                    bind(c) {
                        c.querySelector('[data-dact="no"]').addEventListener('click', () => { settle(null); pop(false); });
                        c.querySelector('[data-dact="theirs"]').addEventListener('click', () => { settle('theirs'); pop(true); });
                        c.querySelector('[data-dact="mine"]').addEventListener('click', () => { settle('mine'); pop(true); });
                    },
                    onClose() { settle(null); }
                });
            });
        }

        // Preparing a host pulls the helper image (about 64 MB) and creates the IO container. Resolves true or false.
        function confirmPrepare(info) {
            return confirm({ title: t('prepare_title'), message: t('prepare_body', { image: info.image }) + ' ' + t('prepare_question', { host: info.hostName }), confirmLabel: t('prepare_button') });
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
                // The live region says why they are asked for again when the address, port or user of an SSH host changed.
                + '<div class="dm-hint dm-secret-again" role="status" aria-live="polite" data-secret-again></div>'
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
                // Closing a form that holds input asks here, inside the form, so nothing is repainted and nothing typed is lost.
                + '<div class="dm-discard-ask" role="alert" data-discard-ask hidden><p class="dm-dialog-copy">' + esc(t('discard_dialog_body')) + '</p>'
                + actions('', button('keep', t('conflict_keep')) + button('discard', t('discard_confirm'), { danger: true })) + '</div>'
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
            if (el) {
                // A field inside a closed <details> cannot take the focus: open its section first.
                const closed = el.closest('details:not([open])');
                if (closed) closed.open = true;
                el.setAttribute('aria-invalid', 'true');
                el.focus({ preventScroll: false });
            }
        }

        // Shows what the chosen connection needs. ui.replacing is true once the admin pressed Replace on stored credentials.
        function syncKind(c, base, ui) {
            const f = c.querySelector('[data-host-form]'), kind = f.elements.kind.value;
            for (const el of f.querySelectorAll('[data-for]')) el.hidden = !el.dataset.for.split(' ').includes(kind);
            f.elements.port.placeholder = String(model.defaultPort(kind));
            const form = readForm(c, base), stored = model.storedSecretUsable(form), tucked = stored && !ui.replacing;
            for (const el of f.querySelectorAll('[data-secret]')) el.hidden = el.hidden || tucked;
            for (const el of f.querySelectorAll('[data-stored-row]')) el.hidden = el.hidden || !tucked;
            // "Leave the fields empty to keep them" is true only while the server can still use the stored secret.
            const keep = f.querySelector('#' + fieldId('ssh_password') + '-hint');
            if (keep) keep.textContent = stored ? t('f_secret_stored') : '';
            const again = f.querySelector('[data-secret-again]'), why = model.sshSecretNeedsReentry(form) ? t('f_secret_again') : '';
            if (again.textContent !== why) again.textContent = why;
        }

        // The secret inputs are tucked away behind Replace while credentials are stored, and the folder and socket inputs sit in
        // the collapsed Advanced section: a problem on one of them opens its hiding place, so it can be seen and focused.
        function revealField(c, base, ui, inputName) {
            if (!ui.replacing && ['ssh_password', 'ssh_key', 'tls_ca', 'tls_cert', 'tls_key'].includes(inputName)) { ui.replacing = true; syncKind(c, base, ui); }
            if (['stacks_dir', 'host_socket_path'].includes(inputName)) { const advanced = c.querySelector('.dm-advanced'); if (advanced) advanced.open = true; }
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
            if (problem) { revealField(c, base, ui, PROBLEM_INPUT[problem.field]); showProblem(c, PROBLEM_INPUT[problem.field], t('v_' + problem.code)); return; }
            showProblem(c, '', '');
            btn.disabled = true;
            btn.setAttribute('aria-busy', 'true');
            out.innerHTML = '<span class="dm-muted">' + views.glyph('refresh', 'dm-working') + ' ' + esc(t('f_testing')) + '</span>';
            try {
                const status = await store.testDraft(model.hostPayload(form), form.id);
                if (out.isConnected) out.innerHTML = resultHtml(status);
            } catch (error) {
                if (!out.isConnected) return;
                const info = error.dm || {};
                if (info.code === 'invalid_host') {
                    // The server could not test the draft at all (a rejected field, credentials missing): same as a rejected save.
                    const input = serverInput(info, form.kind);
                    out.innerHTML = '';
                    revealField(c, base, ui, input);
                    showProblem(c, input, views.errorText(info));
                } else out.innerHTML = '<span class="dm-result is-bad">' + views.dot('bad') + '<span>' + esc(views.errorText(info)) + '</span></span>';
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
            if (problem) { revealField(c, base, ui, PROBLEM_INPUT[problem.field]); showProblem(c, PROBLEM_INPUT[problem.field], t('v_' + problem.code)); return; }
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
                revealField(c, base, ui, input);
                showProblem(c, input, views.errorText(info));
            }
        }

        function openHostForm(host) {
            if (!store.canEditHosts()) { toast(t('readonly_chip'), 'error'); return null; }
            const base = host ? model.hostToForm(host) : model.emptyHostForm(store.state.hosts);
            let form = null, initial = '', question = null, leave = null, lastField = null;
            const signature = () => JSON.stringify([...new FormData(form).entries()]);
            const hideQuestion = () => { if (question) question.hidden = true; leave = null; };
            const view = {
                title: host ? t('host_edit_title') : t('host_add'),
                html: () => formHtml(base),
                dirty: () => !!form && form.isConnected && signature() !== initial,
                // Shows the discard question inside the form; go() runs when the admin chooses to discard.
                ask(go) {
                    if (!question) return;
                    leave = go;
                    question.hidden = false;
                    question.querySelector('[data-dact="keep"]').focus({ preventScroll: true });
                },
                // True when the question was showing: it goes away, the form stays as typed and gets its focus back.
                keepEditing() {
                    if (!question || question.hidden) return false;
                    hideQuestion();
                    (lastField && lastField.isConnected ? lastField : form.elements.name).focus({ preventScroll: true });
                    return true;
                },
                bind(c) {
                    const f = c.querySelector('[data-host-form]'), ui = { replacing: false };
                    form = f;
                    question = c.querySelector('[data-discard-ask]');
                    syncKind(c, base, ui);
                    initial = signature();
                    f.addEventListener('focusin', event => { if (!question.contains(event.target)) lastField = event.target; });
                    f.addEventListener('input', () => syncKind(c, base, ui));
                    f.addEventListener('change', () => syncKind(c, base, ui));
                    f.addEventListener('submit', event => { event.preventDefault(); hideQuestion(); void saveForm(c, base, ui); });
                    f.querySelector('[data-dact="back"]').addEventListener('click', () => requestClose(false));
                    question.querySelector('[data-dact="keep"]').addEventListener('click', () => { view.keepEditing(); });
                    question.querySelector('[data-dact="discard"]').addEventListener('click', () => { const go = leave; hideQuestion(); if (go) go(); });
                    f.querySelector('[data-dact="test-form"]').addEventListener('click', () => void testForm(c, base, ui));
                    f.querySelector('[data-dact="replace"]').addEventListener('click', () => {
                        ui.replacing = true;
                        syncKind(c, base, ui);
                        const first = f.querySelector('[data-secret]:not([hidden]) input, [data-secret]:not([hidden]) textarea');
                        if (first) first.focus();
                    });
                }
            };
            return push(view);
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

        return { openHosts, openHostForm, openNests, confirm, confirmRemove, confirmProtectedAction, confirmDelete, confirmDownVolumes, confirmConflict, confirmPrepare, closeAll, dispose, isOpen: () => dialog.open };
    }

    window.DockerManagerDialogs = { create };
})();
