/* Docker Manager new stack: the flow that creates a stack. A classic script (window.DockerManagerNewStack) with no DOM
   work at load. It fills the content area of the window (the list and the stack pane step aside) with
   - the name (checked like dockerstacks.ValidateName) and the host (fixed when the window is scoped to one host, a picker
     under "All hosts"),
   - two sources: Paste (the compose editor itself, plus an optional .env) and Convert docker run (POST /convert/docker-run
     fills the editor and lists what could not be converted),
   - a live preview next to the code from POST /lint (400 ms after the last keystroke): services, ports with the
     containers that already use them, storage (volumes to create, host folders), variables with "Create random value in
     .env", and the lint hints,
   - Save draft (create) and Deploy (create with deploy: true; the operation shows in the progress drawer). A deploy that could
     not start answers 201 with deploy_error and no operation: the stack stays a draft and an error toast says why.
   Files dropped on the window (compose.yaml, docker-compose.yml, .env) arrive through acceptFiles(). */
(function () {
    'use strict';

    const MAX_FILE_BYTES = 1024 * 1024;

    function create(deps) {
        const { t, esc, model, stackModel: sm, views, editor, store, dialogs, toast, announce, uid, el } = deps;
        const fieldId = name => uid + '-ns-' + name;
        const s = { open: false, tab: 'paste', hostId: '', locked: false, busy: false, lint: null, lintFailed: null, warnings: [], nameTouched: false, initial: '' };
        let compose = null, env = null, linter = null, ac = null;

        const q = selector => el.querySelector(selector);
        const hostName = id => (store.state.hosts.find(host => host.id === id) || {}).name || id;
        const takenNames = () => {
            const entry = store.state.stacks[s.hostId], listing = entry && entry.listing;
            return listing ? listing.stacks.map(stack => stack.name) : [];
        };
        const nameValue = () => (q('[name="name"]') ? q('[name="name"]').value.trim() : '');
        const composeText = () => (compose ? compose.getText() : '');
        const envText = () => (env ? env.getText() : '');
        const snapshot = () => JSON.stringify([nameValue(), composeText(), envText(), q('[name="command"]') ? q('[name="command"]').value : '']);
        const isOpen = () => s.open;
        const isDirty = () => s.open && snapshot() !== s.initial;

        // ---- preview -------------------------------------------------------------------------------------------------

        function portLine(port) {
            const published = port.host ? port.host + '→' : '';
            const label = (port.host_ip ? port.host_ip + ':' : '') + published + port.container + '/' + (port.protocol || 'tcp');
            return '<li><code>' + esc(label) + '</code>' + (port.conflict ? '<span class="dm-badge" data-tone="warn">' + esc(t('preview_port_conflict', { container: port.conflict })) + '</span>' : '') + '</li>';
        }

        function previewHtml() {
            if (s.lintFailed) return '<p class="dm-muted" role="status">' + esc(t('preview_failed')) + ' ' + esc(views.errorText(s.lintFailed.dm || s.lintFailed)) + '</p>';
            const result = s.lint;
            if (!result) return '<p class="dm-muted" role="status">' + esc(composeText().trim() ? t('preview_checking') : t('preview_empty')) + '</p>';
            const preview = result.preview || {}, services = Array.isArray(preview.services) ? preview.services : [], diagnostics = sm.sortDiagnostics(result.diagnostics);
            const out = [];
            if (result.ports_unchecked) out.push('<p class="dm-note">' + views.glyph('warning') + '<span>' + esc(t('preview_ports_unchecked')) + '</span></p>');
            out.push('<section class="dm-pv"><h3>' + esc(t('preview_services')) + '<span class="dm-count">' + services.length + '</span></h3>'
                + (services.length ? '<ul class="dm-pv-list">' + services.map(service => '<li><strong>' + esc(service.name) + '</strong> <span class="dm-muted">' + esc(service.image || (service.build ? t('preview_build') : '')) + '</span>'
                    + (service.ports && service.ports.length ? '<ul class="dm-pv-ports">' + service.ports.map(portLine).join('') + '</ul>' : '') + '</li>').join('') + '</ul>' : '<p class="dm-muted">' + esc(t('preview_no_services')) + '</p>') + '</section>');
            const groups = sm.storageGroups(preview), named = Array.isArray(preview.named_volumes) ? preview.named_volumes : [];
            const storage = [];
            if (named.length) storage.push('<li><span class="dm-pv-kind">' + esc(t('preview_kind_named')) + '</span> ' + named.map(name => '<code>' + esc(name) + '</code>').join(' ') + '</li>');
            for (const kind of ['bind', 'relative', 'anonymous', 'tmpfs']) {
                for (const volume of groups[kind]) storage.push('<li><span class="dm-pv-kind">' + esc(t('preview_kind_' + kind)) + '</span> <code>' + esc(volume.source ? volume.source + ' → ' : '') + esc(volume.target) + '</code>' + (volume.readOnly ? ' <span class="dm-muted">' + esc(t('preview_read_only')) + '</span>' : '') + '</li>');
            }
            out.push('<section class="dm-pv"><h3>' + esc(t('preview_storage')) + '</h3>' + (storage.length ? '<ul class="dm-pv-list">' + storage.join('') + '</ul>' : '<p class="dm-muted">' + esc(t('preview_no_storage')) + '</p>') + '</section>');
            const variables = Array.isArray(preview.variables) ? preview.variables : [], fixable = new Set(sm.randomFixes(diagnostics));
            if (variables.length) {
                out.push('<section class="dm-pv"><h3>' + esc(t('preview_variables')) + '</h3><ul class="dm-pv-list">' + variables.map(variable => {
                    const state = sm.envHas(envText(), variable.name) || variable.in_env ? 'set' : variable.has_default ? 'default' : variable.required ? 'required' : 'missing';
                    const fix = fixable.has(variable.name) && state === 'missing' ? ' <button type="button" class="dm-btn" data-ns="random" data-variable="' + esc(variable.name) + '"><span>' + esc(t('fix_env_random', { name: variable.name })) + '</span></button>' : '';
                    return '<li><code>' + esc(variable.name) + '</code> <span class="dm-badge"' + (state === 'missing' || state === 'required' ? ' data-tone="warn"' : '') + '>' + esc(t('preview_var_' + state)) + '</span>' + fix + '</li>';
                }).join('') + '</ul></section>');
            }
            if (diagnostics.length) {
                out.push('<section class="dm-pv"><h3>' + esc(t('preview_hints')) + '<span class="dm-count">' + diagnostics.length + '</span></h3><ul class="dm-pv-list">' + diagnostics.map(d => '<li data-severity="' + esc(sm.severityOf(d)) + '">'
                    + views.dot(sm.severityOf(d) === 'error' ? 'bad' : sm.severityOf(d) === 'warning' ? 'warn' : 'info') + '<span class="dm-sr-only">' + esc(t('lint_sev_' + sm.severityOf(d))) + '</span> '
                    + (d.line ? '<span class="dm-muted">' + esc(t('lint_line', { line: d.line })) + '</span> ' : '') + esc(sm.diagnosticText(d, t)) + '</li>').join('') + '</ul></section>');
            }
            return out.join('');
        }
        function paintPreview() {
            const target = q('[data-ns-preview]');
            if (target) target.innerHTML = previewHtml();
        }

        function distribute(result) {
            const split = sm.splitDiagnostics(result.diagnostics);
            const localized = list => list.map(d => Object.assign({}, d, { message: sm.diagnosticText(d, t) }));
            if (compose) compose.setDiagnostics(localized(split.compose));
            if (env) env.setDiagnostics(localized(split.env));
        }

        // ---- the form ------------------------------------------------------------------------------------------------

        function nameProblem() {
            const problem = sm.validateName(nameValue(), takenNames());
            const hint = q('[data-ns-name-hint]'), input = q('[name="name"]');
            if (!hint || !input) return problem;
            const touched = s.nameTouched || nameValue() !== '';
            hint.textContent = problem && touched ? t('sv_name_' + problem.code) : t('ns_name_hint');
            hint.dataset.tone = problem && touched ? 'bad' : '';
            if (problem && touched) input.setAttribute('aria-invalid', 'true'); else input.removeAttribute('aria-invalid');
            return problem;
        }

        function hostField() {
            const hosts = store.state.hosts;
            if (s.locked) return '<div class="dm-field"><span class="dm-label">' + esc(t('ns_host')) + '</span><div>' + views.chip(hostName(s.hostId), model.colorOf((hosts.find(host => host.id === s.hostId) || {}).color)) + '</div></div>';
            const options = hosts.map(host => {
                const state = sm.hostStacksState(store.state.stacks[host.id]);
                const note = state === 'not_prepared' ? ' (' + t('ns_host_unprepared') + ')' : state === 'api_too_old' ? ' (' + t('ns_host_old') + ')' : '';
                return '<option value="' + esc(host.id) + '"' + (host.id === s.hostId ? ' selected' : '') + '>' + esc(host.name + note) + '</option>';
            }).join('');
            return '<div class="dm-field"><label for="' + fieldId('host') + '">' + esc(t('ns_host')) + '</label><select id="' + fieldId('host') + '" name="host">' + options + '</select></div>';
        }

        function markup() {
            const tab = (id, label) => '<button type="button" role="tab" class="dm-tab" id="' + fieldId('tab-' + id) + '" data-ns-tab="' + id + '" data-focus-id="nstab:' + id + '" aria-selected="' + (s.tab === id) + '" aria-controls="' + fieldId('panel-' + id) + '" tabindex="' + (s.tab === id ? '0' : '-1') + '">' + esc(label) + '</button>';
            return '<div class="dm-ns"><header class="dm-ns-head"><h2 id="' + fieldId('title') + '">' + esc(t('ns_title')) + '</h2><span class="dm-spacer"></span>'
                + '<button type="button" class="dm-btn" data-ns="cancel" data-focus-id="ns:cancel">' + esc(t('ns_cancel')) + '</button></header>'
                + '<div class="dm-ns-body"><section class="dm-ns-main" aria-labelledby="' + fieldId('title') + '"><div class="dm-ns-fields">'
                + '<div class="dm-field"><label for="' + fieldId('name') + '">' + esc(t('ns_name')) + '</label><input id="' + fieldId('name') + '" name="name" maxlength="63" autocomplete="off" spellcheck="false" autocapitalize="off" aria-describedby="' + fieldId('name-hint') + '" data-autofocus>'
                + '<small class="dm-hint" id="' + fieldId('name-hint') + '" data-ns-name-hint>' + esc(t('ns_name_hint')) + '</small></div>' + hostField() + '</div>'
                + '<div class="dm-tabs" role="tablist" aria-label="' + esc(t('ns_source')) + '">' + tab('paste', t('ns_tab_paste')) + tab('convert', t('ns_tab_convert')) + '</div>'
                + '<div role="tabpanel" class="dm-ns-panel" id="' + fieldId('panel-paste') + '" aria-labelledby="' + fieldId('tab-paste') + '" data-ns-panel="paste"' + (s.tab === 'paste' ? '' : ' hidden') + '>'
                + '<p class="dm-hint">' + esc(t('ns_paste_hint')) + '</p><div data-ns-notes></div><div class="dm-ed-surface dm-ns-editor" data-ns-compose></div>'
                + '<details class="dm-advanced" data-ns-env><summary>' + esc(t('ns_env_optional')) + '</summary><div class="dm-ed-surface dm-ns-editor is-short" data-ns-envsurface></div></details></div>'
                + '<div role="tabpanel" class="dm-ns-panel" id="' + fieldId('panel-convert') + '" aria-labelledby="' + fieldId('tab-convert') + '" data-ns-panel="convert"' + (s.tab === 'convert' ? '' : ' hidden') + '>'
                + '<div class="dm-field"><label for="' + fieldId('command') + '">' + esc(t('ns_command')) + '</label><textarea id="' + fieldId('command') + '" name="command" rows="6" spellcheck="false" autocomplete="off" autocapitalize="off" placeholder="docker run -d --name web -p 8080:80 nginx:1.27"></textarea></div>'
                + '<div class="dm-ns-convert-actions"><button type="button" class="dm-btn is-primary" data-ns="convert" data-focus-id="ns:convert"><span>' + esc(t('ns_convert')) + '</span></button><span class="dm-hint" data-ns-convert-state role="status"></span></div></div>'
                + '</section><aside class="dm-ns-preview" aria-label="' + esc(t('preview_title')) + '"><h3 class="dm-ns-preview-title">' + esc(t('preview_title')) + '</h3><div data-ns-preview></div></aside></div>'
                + '<footer class="dm-ns-foot"><p class="dm-form-error" role="alert" data-ns-error hidden></p><span class="dm-spacer"></span>'
                + '<button type="button" class="dm-btn" data-ns="save" data-focus-id="ns:save"><span>' + esc(t('ns_save_draft')) + '</span></button>'
                + '<button type="button" class="dm-btn is-primary" data-ns="deploy" data-focus-id="ns:deploy">' + views.glyph('deploy') + '<span>' + esc(t('action_deploy')) + '</span></button></footer></div>';
        }

        function showError(message, inputName) {
            const box = q('[data-ns-error]');
            if (!box) return;
            box.textContent = message || '';
            box.hidden = !message;
            if (inputName) q('[name="' + inputName + '"]')?.focus();
        }

        function paintBusy() {
            for (const button of el.querySelectorAll('[data-ns="save"], [data-ns="deploy"]')) {
                button.disabled = s.busy || !deps.canMutate();
                if (s.busy) button.setAttribute('aria-busy', 'true'); else button.removeAttribute('aria-busy');
            }
        }

        function setTab(tab) {
            s.tab = tab;
            for (const button of el.querySelectorAll('[data-ns-tab]')) {
                const on = button.dataset.nsTab === tab;
                button.setAttribute('aria-selected', String(on));
                button.tabIndex = on ? 0 : -1;
            }
            for (const panel of el.querySelectorAll('[data-ns-panel]')) panel.hidden = panel.dataset.nsPanel !== tab;
            if (tab === 'paste') compose?.refresh();
        }

        function suggestName(value) {
            const input = q('[name="name"]');
            if (!input || input.value.trim() || s.nameTouched || !value) return;
            input.value = sm.sanitizeName(value);
            nameProblem();
        }

        function mountEditors() {
            compose = editor.mount(q('[data-ns-compose]'), {
                label: t('editor_compose_label'), text: '', language: 'yaml', readOnly: !deps.canMutate(),
                onChange: text => { s.lint = text.trim() ? s.lint : null; linter.schedule(); suggestName(sm.firstServiceName(text)); paintPreview(); },
                onSave: () => { void submit(false); }, onDeploy: () => { void submit(true); }, onFix: diagnostic => fix(diagnostic.fix && diagnostic.fix.variable)
            });
            const details = q('[data-ns-env]');
            const mountEnv = () => {
                if (env || !details.open) return;
                env = editor.mount(q('[data-ns-envsurface]'), {
                    label: t('editor_env_label'), text: s.pendingEnv || '', language: 'plain', wrap: true, readOnly: !deps.canMutate(),
                    onChange: () => { linter.schedule(); paintPreview(); }, onSave: () => { void submit(false); }, onDeploy: () => { void submit(true); }
                });
                s.pendingEnv = '';
            };
            details.addEventListener('toggle', mountEnv, { signal: ac.signal });
        }

        // Adds NAME=<random> to the .env (opening the section and creating the editor when needed).
        function fix(name) {
            if (!name || !deps.canMutate()) return;
            const details = q('[data-ns-env]');
            if (!env) {
                s.pendingEnv = sm.appendEnvVariable(s.pendingEnv || '', name, sm.randomValue(32, window.crypto));
                details.open = true;
                details.dispatchEvent(new Event('toggle'));
            } else env.setText(sm.appendEnvVariable(env.getText(), name, sm.randomValue(32, window.crypto)));
            linter.schedule();
            paintPreview();
            announce(t('fix_env_done', { name }));
        }

        // ---- actions -------------------------------------------------------------------------------------------------

        function warningsHtml() {
            if (!s.warnings.length) return '';
            return '<div class="dm-note">' + views.glyph('warning') + '<div><strong>' + esc(t('ns_convert_warnings')) + '</strong><ul class="dm-pv-list">'
                + s.warnings.map(w => '<li>' + esc(sm.CONVERT_WARNINGS.includes(w.code) ? t('convert_' + w.code, { flag: w.flag }) : w.message) + '</li>').join('') + '</ul></div></div>';
        }

        async function convert() {
            const command = q('[name="command"]').value.trim(), state = q('[data-ns-convert-state]');
            if (!command) { state.textContent = t('ns_command_empty'); q('[name="command"]').focus(); return; }
            state.textContent = t('ns_converting');
            try {
                const result = await store.convertDockerRun(command);
                if (!s.open) return;
                compose.setText(String(result.compose || ''));
                s.warnings = Array.isArray(result.warnings) ? result.warnings : [];
                q('[data-ns-notes]').innerHTML = warningsHtml();
                suggestName(result.service);
                state.textContent = '';
                setTab('paste');
                compose.focus();
                linter.schedule();
                announce(t('ns_converted'));
            } catch (error) {
                if (!s.open) return;
                state.textContent = views.errorText(error.dm);
            }
        }

        async function submit(deploy) {
            if (s.busy || !deps.canMutate()) return;
            showError('');
            const problem = nameProblem();
            if (problem) { nameProblem(); showError(t('sv_name_' + problem.code), 'name'); return; }
            if (!composeText().trim()) { showError(t('ns_compose_empty')); setTab('paste'); compose.focus(); return; }
            s.busy = true;
            paintBusy();
            const name = nameValue();
            try {
                const body = await store.createStack(s.hostId, { name, compose: composeText(), env: envText(), deploy });
                if (!s.open) return;
                s.busy = false;
                if (deploy && body && body.deploy_error) {
                    // The files are written, only the start failed: the stack opens as a draft and the reason is shown.
                    const failed = t('ns_deploy_failed', { name, reason: views.errorText(body.deploy_error) });
                    announce(failed);
                    toast(failed, 'error');
                } else {
                    announce(t(deploy ? 'ns_deployed' : 'ns_saved', { name }));
                    toast(t(deploy ? 'ns_deployed' : 'ns_saved', { name }), 'success');
                }
                close(true);
                if (deps.onCreated) deps.onCreated(s.hostId, name, body);
            } catch (error) {
                s.busy = false;
                if (!s.open) return;
                paintBusy();
                const info = error.dm || {};
                if (info.code === 'host_not_prepared' || info.code === 'runner_image_missing') {
                    const entry = store.state.stacks[s.hostId], helpers = entry && entry.error && entry.error.data && entry.error.data.helpers;
                    const data = info.data || {};
                    showError(views.errorText(info));
                    if (info.code === 'host_not_prepared' && await dialogs.confirmPrepare({ hostName: hostName(s.hostId), image: (data.helpers || helpers || {}).runner_image || 'docker:29.9.2-cli' })) void deps.onPrepare(s.hostId);
                } else if (info.code === 'stack_name_invalid' || info.code === 'stack_exists') { nameProblem(); showError(views.errorText(info), 'name'); }
                else showError(views.errorText(info));
            }
        }

        function open(options) {
            const o = options || {};
            if (s.open) {
                if (o.compose != null && compose) compose.setText(o.compose);
                return;
            }
            const hosts = store.state.hosts;
            s.open = true;
            s.tab = 'paste';
            s.locked = !!o.lockHost;
            s.hostId = o.hostId && hosts.some(host => host.id === o.hostId) ? o.hostId : (hosts.find(host => sm.hostStacksState(store.state.stacks[host.id]) === 'ok') || hosts[0] || {}).id || '';
            s.lint = null;
            s.lintFailed = null;
            s.warnings = [];
            s.busy = false;
            s.nameTouched = false;
            s.pendingEnv = '';
            ac = new AbortController();
            el.innerHTML = markup();
            el.hidden = false;
            linter = deps.createLinter(
                () => {
                    if (!composeText().trim()) return Promise.resolve({ diagnostics: [], preview: { services: [], named_volumes: [], variables: [] } });
                    return store.lint(s.hostId, { compose: composeText(), env: envText() }, 'newstack');
                },
                (result, error) => { s.lint = result; s.lintFailed = result ? null : error; if (result) distribute(result); paintPreview(); });
            mountEditors();
            const signal = ac.signal;
            el.addEventListener('input', event => {
                if (event.target.matches('[name="name"]')) { s.nameTouched = true; nameProblem(); }
            }, { signal });
            el.addEventListener('change', event => {
                if (event.target.matches('[name="host"]')) { s.hostId = event.target.value; nameProblem(); linter.schedule(); }
            }, { signal });
            el.addEventListener('click', event => {
                const tab = event.target.closest('[data-ns-tab]'), action = event.target.closest('[data-ns]');
                if (tab) { setTab(tab.dataset.nsTab); return; }
                if (!action) return;
                switch (action.dataset.ns) {
                    case 'cancel': void deps.requestClose(); break;
                    case 'convert': void convert(); break;
                    case 'save': void submit(false); break;
                    case 'deploy': void submit(true); break;
                    case 'random': fix(action.dataset.variable); break;
                    default: break;
                }
            }, { signal });
            el.addEventListener('keydown', event => {
                const tab = event.target.closest('[data-ns-tab]');
                if (!tab || !['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
                event.preventDefault();
                const next = tab.dataset.nsTab === 'paste' ? 'convert' : 'paste';
                setTab(next);
                q('[data-ns-tab="' + next + '"]').focus();
            }, { signal });
            nameProblem();
            paintPreview();
            paintBusy();
            if (o.compose != null) compose.setText(o.compose);
            if (o.env != null) { s.pendingEnv = o.env; q('[data-ns-env]').open = true; q('[data-ns-env]').dispatchEvent(new Event('toggle')); }
            if (o.name) { q('[name="name"]').value = sm.sanitizeName(o.name); nameProblem(); }
            s.initial = snapshot();
            if (o.compose != null || o.env != null) linter.schedule();
            q('[name="name"]').focus({ preventScroll: true });
        }

        function close(force) {
            if (!s.open) return;
            s.open = false;
            linter?.dispose();
            linter = null;
            ac?.abort();
            compose?.dispose();
            env?.dispose();
            compose = env = null;
            el.replaceChildren();
            el.hidden = true;
            if (!force && deps.onClosed) deps.onClosed();
        }

        // Files dropped on the window: the compose file and the .env go into the editors; anything else is ignored with a note.
        async function acceptFiles(files) {
            let taken = 0;
            for (const file of Array.from(files || [])) {
                const kind = sm.classifyDroppedFile(file.name);
                if (!kind) continue;
                if (file.size > MAX_FILE_BYTES) { toast(views.errorText({ code: 'file_too_large' }), 'error'); continue; }
                const content = await file.text();
                if (!s.open) return taken;
                if (kind === 'compose') { compose.setText(content); suggestName(sm.firstServiceName(content)); setTab('paste'); }
                else {
                    if (env) env.setText(content);
                    else { s.pendingEnv = content; q('[data-ns-env]').open = true; q('[data-ns-env]').dispatchEvent(new Event('toggle')); }
                }
                taken += 1;
            }
            if (taken) { linter.schedule(); announce(t('ns_dropped', { count: taken })); } else toast(t('ns_drop_ignored'), 'error');
            return taken;
        }

        return { open, close, isOpen, isDirty, acceptFiles, dispose: () => close(true) };
    }

    window.DockerManagerNewStack = { create, MAX_FILE_BYTES };
})();
