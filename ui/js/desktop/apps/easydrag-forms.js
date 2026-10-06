// EasyDrag parameter forms: generated from NodeTypeInfo.params, with "fixed / from data"
// switches, dynamic options, visibility rules, inline issues and the live preview.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};

    function isTemplate(value) { return typeof value === 'string' && value.indexOf('{{') >= 0; }

    function visible(param, params) {
        const rule = param.visible_if;
        if (!rule) return true;
        const value = params[rule.param];
        return (rule.equals || []).map(String).includes(String(value == null ? '' : value));
    }

    // PLAINTEXT_VALUES lists generic tool params whose values can hold credentials (header
    // maps, LDAP attribute maps) but are saved in the flow as plain text: generic tools have
    // no secret picker, and the catalog drops only params named like credentials. The
    // catalog drops netlify and vercel env_value with their set_env operation today; they
    // stay listed so the hint is there if that operation comes back.
    const PLAINTEXT_VALUES = {
        'tool.manage_outgoing_webhooks': ['headers'],
        'tool.ldap': ['entry_attributes', 'changes'],
        'tool.netlify': ['env_value'],
        'tool.vercel': ['env_value']
    };

    function plaintextValue(type, name) {
        return Object.prototype.hasOwnProperty.call(PLAINTEXT_VALUES, type) && PLAINTEXT_VALUES[type].includes(name);
    }

    // render builds the form for one node. env:
    //   ed, node, info, upstream [{key, label, cat, fields}], roots {key: output}, issues [Issue]
    //   onChange(name, value), openFileDialog?
    // Returns {el, focus(param), insert(param, ref), refresh(node, roots), dispose()}.
    function render(env) {
        const core = ED.core;
        const F = ED.fields;
        const { t, esc } = env.ed;
        const readonly = !!(env.ed.readonly || env.ed.runView);
        const el = core.el('<div class="ed-form"></div>');
        const controls = new Map();
        const optionCache = new Map();
        const forced = new Set();
        let node = env.node;
        let roots = env.roots || {};

        function fieldEnv(name) {
            return {
                t, esc, readonly, root: env.ed.root, api: env.ed.api, upstream: env.upstream, roots,
                openFileDialog: env.openFileDialog,
                change: value => { env.onChange(name, value); }
            };
        }

        function previewFor(name) {
            const box = el.querySelector('.ed-field[data-param="' + CSS.escape(name) + '"] .ed-field-preview');
            if (!box) return;
            const value = node.params[name];
            if (!isTemplate(value) && !(value && typeof value === 'object')) { box.hidden = true; return; }
            box.hidden = false;
            if (!Object.keys(roots).length) { box.textContent = t('easydrag.ui.preview_no_data'); box.className = 'ed-field-preview is-muted'; return; }
            try {
                const result = ED.template.evaluate(value, roots);
                const shown = typeof result === 'string' ? (result.length > 140 ? result.slice(0, 139) + '…' : result) : '';
                box.className = 'ed-field-preview';
                box.innerHTML = '<span class="ed-preview-label">' + esc(t('easydrag.ui.preview_result')) + '</span> ' +
                    esc(ED.template.describe(result, t, core.fmt.number)) + (shown ? '<span class="ed-preview-text">' + esc(shown) + '</span>' : '');
            } catch (err) {
                box.className = 'ed-field-preview is-error';
                box.textContent = t('easydrag.ui.preview_failed', { reason: err.message });
            }
        }

        // loadOptions fills a select (or the datalist of a combo box) with the param's dynamic
        // options. Only an answer is cached: after a failed request the next build asks again.
        async function loadOptions(param, select, value) {
            const key = env.info.type + '|' + param.name;
            let answer = optionCache.get(key);
            if (!answer) {
                select.disabled = true;
                try {
                    const res = await env.ed.api.options(env.info.type, param.name);
                    answer = { options: res && Array.isArray(res.options) ? res.options : [], truncated: !!(res && res.truncated) };
                    optionCache.set(key, answer);
                } catch (err) {
                    answer = { options: [], truncated: false };
                    select.dataset.error = core.errorText(t, err);
                }
                select.disabled = readonly;
            }
            const options = answer.options;
            const list = select.tagName === 'DATALIST' ? select : null;
            if (list) {
                list.innerHTML = options.map(o => '<option value="' + esc(o.value) + '">' + esc(o.label + (o.hint ? ' · ' + o.hint : '')) + '</option>').join('');
                // Home Assistant entities are cut at 2000 (the only list the server cuts); the
                // combo box still takes any entity id typed in.
                const wrap = list.parentNode;
                if (answer.truncated && wrap && !wrap.querySelector('.ed-options-truncated')) {
                    wrap.appendChild(core.el('<p class="ed-hint ed-options-truncated">' + esc(t('easydrag.ui.options_truncated')) + '</p>'));
                }
                return;
            }
            const present = options.some(o => String(o.value) === String(value == null ? '' : value));
            select.innerHTML = (param.required ? '' : '<option value="">' + esc(t('easydrag.ui.option_none')) + '</option>') +
                options.map(o => '<option value="' + esc(o.value) + '"' + (String(o.value) === String(value == null ? '' : value) ? ' selected' : '') + '>' + esc(o.label) + '</option>').join('') +
                (!present && value ? '<option value="' + esc(value) + '" selected>' + esc(value) + '</option>' : '');
        }

        function templated(param, value) { return isTemplate(value) || forced.has(param.name); }

        function controlFor(param, value, id) {
            const fenv = fieldEnv(param.name);
            const set = v => fenv.change(v);
            const asTemplate = param.templatable && templated(param, value);
            switch (param.kind) {
                case 'textarea':
                    return F.templateField(fenv, value, { multiline: true, id }).el;
                case 'text':
                case 'cron':
                    return asTemplate || param.kind === 'text' ? F.templateField(fenv, value, { id }).el : plainInput(param, value, id, set);
                case 'number':
                    return templated(param, value) ? F.templateField(fenv, value, { id }).el : numberInput(value, id, set);
                case 'bool':
                    return F.toggle(fenv, value === undefined ? param.default : value, set, param.label);
                case 'segmented':
                    return F.segmented(fenv, param.options || [], value === undefined ? param.default : value, set);
                case 'select':
                    if (templated(param, value)) return F.templateField(fenv, value, { id }).el;
                    return selectControl(param, value, id, set);
                case 'multiselect':
                    return multiSelect(param, value, set);
                case 'file':
                    return F.fileField(fenv, value, set, id);
                case 'json':
                    return F.jsonEditor(fenv, value, set, id);
                case 'keyvalue':
                    return F.keyValue(fenv, value, set);
                case 'secret_ref':
                    return F.secretRef(fenv, value, set, id);
                case 'condition_group':
                    return F.conditionGroup(fenv, value, set);
                case 'cases':
                    return F.cases(fenv, value, set);
                case 'datetime':
                    return templated(param, value) ? F.templateField(fenv, value, { id }).el : F.dateTime(fenv, value, set, id);
                case 'fields':
                    return F.fieldList(fenv, param, value, set);
                case 'tags':
                    return F.tags(fenv, value, set);
                default:
                    return F.templateField(fenv, value, { id }).el;
            }
        }

        function plainInput(param, value, id, set) {
            const input = core.el('<input class="ed-input' + (param.kind === 'cron' ? ' ed-code' : '') + '" id="' + esc(id) + '" value="' + esc(value == null ? '' : value) + '"' + (readonly ? ' disabled' : '') + ' spellcheck="false">');
            input.addEventListener('change', () => set(input.value));
            return input;
        }

        function numberInput(value, id, set) {
            const input = core.el('<input type="number" inputmode="decimal" class="ed-input" id="' + esc(id) + '" value="' + esc(value == null ? '' : value) + '"' + (readonly ? ' disabled' : '') + '>');
            input.addEventListener('change', () => set(input.value === '' ? undefined : Number(input.value)));
            return input;
        }

        function selectControl(param, value, id, set) {
            if (param.options_source) {
                const free = param.options_source === 'ha_entities';
                if (free) {
                    const wrap = core.el('<div class="ed-combo"><input class="ed-input" id="' + esc(id) + '" list="' + esc(id) + '-list" value="' + esc(value == null ? '' : value) + '"' + (readonly ? ' disabled' : '') + ' spellcheck="false"><datalist id="' + esc(id) + '-list"></datalist></div>');
                    wrap.querySelector('input').addEventListener('change', (event) => set(event.target.value || undefined));
                    loadOptions(param, wrap.querySelector('datalist'), value);
                    return wrap;
                }
                const select = core.el('<select class="ed-input" id="' + esc(id) + '"' + (readonly ? ' disabled' : '') + '></select>');
                select.addEventListener('change', () => set(select.value === '' ? undefined : select.value));
                loadOptions(param, select, value);
                return select;
            }
            return ED.fields.selectBox(fieldEnv(param.name), (param.required ? [] : [{ value: '', label: t('easydrag.ui.option_none') }]).concat(param.options || []),
                value === undefined ? (param.default == null ? '' : param.default) : value, v => set(v === '' ? undefined : v), id);
        }

        function multiSelect(param, value, set) {
            const chosen = new Set(Array.isArray(value) ? value.map(String) : []);
            const wrap = core.el('<div class="ed-chips" role="group">' + (param.options || []).map(o =>
                '<button type="button" class="ed-chip-toggle" aria-pressed="' + chosen.has(String(o.value)) + '" data-value="' + esc(o.value) + '"' + (readonly ? ' disabled' : '') + '>' + esc(o.label) + '</button>').join('') + '</div>');
            wrap.addEventListener('click', (event) => {
                const b = event.target.closest('button');
                if (!b) return;
                if (chosen.has(b.dataset.value)) chosen.delete(b.dataset.value); else chosen.add(b.dataset.value);
                b.setAttribute('aria-pressed', String(chosen.has(b.dataset.value)));
                set((param.options || []).map(o => String(o.value)).filter(v => chosen.has(v)));
            });
            return wrap;
        }

        function issueFor(name) {
            return (env.issues || []).find(is => is.node_id === node.id && is.param && (is.param === name || is.param.startsWith(name + '[') || is.param.startsWith(name + '.')));
        }

        function build() {
            el.innerHTML = '';
            controls.clear();
            const params = (env.info && env.info.params) || [];
            if (!params.length) {
                el.innerHTML = '<p class="ed-form-empty">' + esc(t('easydrag.ui.form_no_params')) + '</p>';
                return;
            }
            params.forEach(param => {
                if (!visible(param, node.params)) return;
                const id = 'ed-p-' + env.ed.windowId + '-' + node.id + '-' + param.name;
                const value = node.params[param.name];
                const issue = issueFor(param.name);
                const canToggle = param.templatable && !['text', 'textarea', 'condition_group', 'cases', 'fields', 'keyvalue', 'file', 'json'].includes(param.kind) && !readonly;
                const field = core.el('<div class="ed-field' + (param.required ? ' is-required' : '') + (issue ? ' has-issue' : '') + '" data-param="' + esc(param.name) + '" data-kind="' + esc(param.kind) + '">' +
                    '<div class="ed-field-head"><label for="' + esc(id) + '">' + esc(param.label) + (param.required ? '<span class="ed-required" aria-hidden="true">*</span>' : '') + '</label>' +
                    (canToggle ? '<button type="button" class="ed-mode" data-ed-mode="' + esc(param.name) + '" aria-pressed="' + templated(param, value) + '" title="' + esc(t('easydrag.ui.mode_hint')) + '">' +
                        esc(templated(param, value) ? t('easydrag.ui.mode_data') : t('easydrag.ui.mode_fixed')) + '</button>' : '') +
                    (param.help ? '<span class="ed-help" tabindex="0" role="note" aria-label="' + esc(param.help) + '" title="' + esc(param.help) + '">' + core.icon('info') + '</span>' : '') +
                    '</div><div class="ed-field-control"></div>' +
                    (plaintextValue(env.info.type, param.name) ? '<p class="ed-hint ed-field-plaintext">' + esc(t('easydrag.ui.hint_plaintext_value')) + '</p>' : '') +
                    '<div class="ed-field-preview" hidden></div>' +
                    (issue ? '<div class="ed-field-issue">' + core.icon('alert') + '<span>' + esc(core.issueText(t, issue)) + '</span></div>' : '') + '</div>');
                const control = controlFor(param, value, id);
                field.querySelector('.ed-field-control').appendChild(control);
                if (param.sensitive_sink) field.classList.add('is-sink');
                controls.set(param.name, { field, control, param });
                el.appendChild(field);
                previewFor(param.name);
            });
        }

        el.addEventListener('click', (event) => {
            const mode = event.target.closest('[data-ed-mode]');
            if (!mode) return;
            const name = mode.dataset.edMode;
            const value = node.params[name];
            if (isTemplate(value) || forced.has(name)) {
                forced.delete(name);
                if (isTemplate(value)) env.onChange(name, undefined); else build();
                return;
            }
            forced.add(name);
            build();
            requestAnimationFrame(() => {
                const entry = controls.get(name);
                const view = entry && entry.field.querySelector('.ed-tpl-view');
                if (view) view.click();
            });
        });

        // Drop targets for fields dragged from the input tree.
        el.addEventListener('dragover', (event) => {
            const target = event.target.closest('.ed-field');
            if (!target || readonly || !event.dataTransfer.types.includes('application/x-easydrag-ref')) return;
            event.preventDefault();
            el.querySelectorAll('.is-drop').forEach(n => n.classList.remove('is-drop'));
            target.classList.add('is-drop');
        });
        el.addEventListener('dragleave', (event) => {
            const target = event.target.closest('.ed-field');
            if (target && !target.contains(event.relatedTarget)) target.classList.remove('is-drop');
        });
        el.addEventListener('drop', (event) => {
            const target = event.target.closest('.ed-field');
            if (!target) return;
            event.preventDefault();
            target.classList.remove('is-drop');
            const ref = event.dataTransfer.getData('application/x-easydrag-ref');
            if (!ref || readonly) return;
            // A param with several template fields (condition rows, key/value pairs, field
            // lists) takes the drop in the field under the pointer; insert picks the first one.
            const tpl = event.target.closest('.ed-tpl');
            if (tpl && tpl.edTemplate && target.contains(tpl)) tpl.edTemplate.insert(ref);
            else insert(target.dataset.param, ref);
        });

        // insert adds {{ref}} to a parameter (switching it to "from data" when needed). A
        // read-only form (read-only flow or run view) changes nothing.
        function insert(name, ref) {
            const entry = controls.get(name);
            if (!entry || readonly) return;
            const tpl = entry.field.querySelector('.ed-tpl');
            if (tpl && tpl.edTemplate) { tpl.edTemplate.insert(ref); return; }
            const value = node.params[name];
            const kind = entry.param.kind;
            if (['text', 'textarea', 'file'].includes(kind) || isTemplate(value)) {
                const current = typeof value === 'string' ? value : '';
                const sep = current && kind === 'textarea' ? ' ' : '';
                env.onChange(name, current.replace(/\{\{\}\}$/, '') + sep + '{{' + ref + '}}');
            } else if (entry.param.templatable) {
                env.onChange(name, '{{' + ref + '}}');
            }
        }

        build();

        return {
            el,
            insert,
            // refresh re-renders after the node changed outside the form (undo, other window).
            refresh(nextNode, nextRoots, issues) {
                const visibleBefore = Array.from(controls.keys()).join(',');
                node = nextNode;
                if (nextRoots) roots = nextRoots;
                if (issues) env.issues = issues;
                const visibleAfter = ((env.info && env.info.params) || []).filter(p => visible(p, node.params)).map(p => p.name).join(',');
                const focused = el.contains(document.activeElement);
                if (visibleBefore !== visibleAfter || !focused) build();
                else controls.forEach((_, name) => previewFor(name));
            },
            focus(name) {
                const entry = controls.get(name);
                if (!entry) return;
                entry.field.scrollIntoView({ block: 'center', behavior: 'smooth' });
                entry.field.classList.add('is-flash');
                setTimeout(() => entry.field.classList.remove('is-flash'), 1200);
                const target = entry.field.querySelector('.ed-tpl-view, input, select, textarea, button');
                if (target) target.focus();
            },
            dispose() { el.remove(); }
        };
    }

    ED.forms = { render, visible, isTemplate };
})();
