(function () {
    'use strict';

    const M = window.FindItModel, D = window.FindItDialogs;
    function create(c) {
        const { state, ctx, tr, canWrite, dialogOptions, draw, apiCommand, withBusy, showNotice, categoryName, icon } = c;
        const asList = M.asArray;
        const categoryOptions = () => asList(state.catalog?.categories).map(value => { const id = typeof value === 'object' ? value.id : value; return { value: id, label: tr('category_' + id, typeof value === 'object' ? value.name || value.label || id : String(value)) }; });
        const templateName = item => item?.built_in ? tr('template_' + item.id, item.name) : item?.name || '';
        const templateDescription = item => item?.built_in ? tr('template_desc_' + item.id, item.description || '') : item?.description || '';
        function renderTemplates() {
            const templates = asList(state.catalog?.templates), selected = templates.find(item => item.id === state.template?.id) || state.template;
            return `<div class="fi-page-toolbar"><div><p class="fi-eyebrow">${M.esc(tr('schema_library'))}</p><h1>${M.esc(tr('templates'))}</h1><p>${M.esc(tr('templates_intro'))}</p></div><div class="fi-page-actions">${canWrite() ? `<button type="button" class="fi-primary" data-action="new-template">${icon('plus', 16)}${M.esc(tr('create_template'))}</button>` : ''}</div></div><div class="fi-entity-layout ${state.mobilePanel === 'detail' ? 'fi-show-detail' : ''}"><section class="fi-entity-list fi-template-list">${templates.map(item => `<button type="button" data-select-template="${M.esc(item.id)}" class="fi-entity-row ${selected?.id === item.id ? 'is-selected' : ''}"><span class="fi-template-mark">${icon('settings', 17)}</span><span><b>${M.esc(templateName(item))}</b><small>${M.esc(categoryName(ctx, item.category))} · ${asList(item.fields).length} ${M.esc(tr('fields'))}</small></span><i>${item.built_in ? M.esc(tr('built_in')) : M.esc(tr('custom'))}</i></button>`).join('')}</section><section class="fi-entity-detail">${selected ? `<button type="button" class="fi-mobile-back" data-action="back-templates">${icon('back', 16)}${M.esc(tr('back_to_list'))}</button><div class="fi-template-hero"><span>${icon('settings', 25)}</span><div><p class="fi-eyebrow">${M.esc(selected.built_in ? tr('built_in_template') : tr('custom_template'))}</p><h2>${M.esc(templateName(selected))}</h2><p>${M.esc(templateDescription(selected))}</p></div></div><p class="fi-template-category">${M.esc(categoryName(ctx, selected.category))}</p><div class="fi-template-fields">${asList(selected.fields).map(field => `<div><span>${M.esc(selected.built_in ? tr('field_' + field.key, field.label || field.key) : field.label || field.key)}</span><code>${M.esc(field.key)}</code><small>${M.esc(tr('field_type_' + field.type, field.type))}${field.unit ? ` · ${M.esc(tr('unit_' + field.unit.toLowerCase().replace(/[^a-z0-9]+/g, '_'), field.unit))}` : ''}${field.required ? ` · ${M.esc(tr('required'))}` : ''}</small></div>`).join('') || `<p>${M.esc(tr('no_template_fields'))}</p>`}</div>${canWrite() && !selected.built_in ? `<div class="fi-detail-toolbar"><button type="button" class="fi-tool-button" data-action="edit-template">${icon('edit', 15)}${M.esc(tr('edit'))}</button><button type="button" class="fi-danger-button" data-action="archive-template">${M.esc(tr('archive_template'))}</button></div>` : ''}</section></div>` : ''}`;
        }
        async function templateForm(template = null) {
            if (!canWrite()) return;
            const options = dialogOptions({}), restore = document.activeElement;
            const draftKey = template?.id || 'new', previous = state.templateDraft?.id === draftKey ? state.templateDraft.data : null;
            const data = M.clone(previous || { name: template?.name || '', category: template?.category || categoryOptions()[0]?.value || 'other', description: template?.description || '', fields: template?.fields || [] });
            const existing = new Set(asList(template?.fields).map(field => field.key));
            const editor = document.createElement('dialog'), abort = new AbortController();
            editor.className = 'fi-dialog fi-template-editor';
            const titleId = 'fi-template-' + crypto.randomUUID();
            editor.setAttribute('aria-labelledby', titleId);
            editor.innerHTML = `<form class="fi-dialog-form"><header class="fi-dialog-head"><div><p class="fi-kicker">${M.esc(tr('schema_library'))}</p><h2 id="${titleId}">${M.esc(tr(template ? 'edit_template' : 'create_template'))}</h2><p>${M.esc(tr('template_visual_help'))}</p></div><button type="button" data-close aria-label="${M.esc(tr('close'))}">×</button></header><div class="fi-dialog-body"><div class="fi-form-grid"><label class="fi-form-field"><span>${M.esc(tr('template_name'))}</span><input name="name" required maxlength="256" value="${M.esc(data.name)}"></label><label class="fi-form-field"><span>${M.esc(tr('category'))}</span><select name="category">${categoryOptions().map(option => `<option value="${M.esc(option.value)}" ${option.value === data.category ? 'selected' : ''}>${M.esc(option.label)}</option>`).join('')}</select></label><label class="fi-form-field is-wide"><span>${M.esc(tr('description'))}</span><textarea name="description" rows="2">${M.esc(data.description)}</textarea></label></div><div class="fi-detail-toolbar"><h3>${M.esc(tr('template_fields'))}</h3><button type="button" class="fi-tool-button" data-add-field>${icon('plus', 16)} ${M.esc(tr('add_field'))}</button></div><div data-field-list></div><p class="fi-form-error" data-error role="alert" hidden></p></div><footer class="fi-dialog-foot"><button type="button" data-close>${M.esc(tr('cancel'))}</button><button type="submit" class="fi-primary">${M.esc(tr(template ? 'save' : 'create_template'))}</button></footer></form>`;
            options.host.append(editor);
            const form = editor.querySelector('form'), list = editor.querySelector('[data-field-list]');
            const types = ['text', 'number', 'boolean', 'date', 'enum'];
            let saving = false, closed = false;
            function read() {
                data.name = form.elements.name.value.trim(); data.category = form.elements.category.value; data.description = form.elements.description.value;
                data.fields = [...list.querySelectorAll('[data-field-row]')].map(row => {
                    const control = name => row.querySelector(`[data-prop="${name}"]`);
                    const field = { key: control('key').value.trim(), label: control('label').value.trim(), type: control('type').value, required: control('required').checked };
                    if (field.type === 'number') {
                        field.unit = control('unit').value.trim();
                        for (const bound of ['min', 'max']) if (control(bound).value !== '') field[bound] = Number(control(bound).value);
                    }
                    if (field.type === 'enum') field.options = control('options').value.split(/\r?\n/).map(value => value.trim()).filter(Boolean);
                    return field;
                });
                state.templateDraft = { id: draftKey, data: M.clone(data) };
            }
            function rows() {
                list.innerHTML = data.fields.map((field, index) => {
                    const locked = existing.has(field.key), disabled = locked ? 'disabled' : '';
                    return `<fieldset class="fi-template-field-row" data-field-row><legend>${M.esc(tr('field'))} ${index + 1}</legend><div class="fi-form-grid"><label class="fi-form-field"><span>${M.esc(tr('field_label'))}</span><input data-prop="label" required maxlength="128" value="${M.esc(field.label)}"></label><label class="fi-form-field"><span>${M.esc(tr('field_type'))}</span><select data-prop="type" ${disabled}>${types.map(type => `<option value="${type}" ${type === field.type ? 'selected' : ''}>${M.esc(tr('field_type_' + type))}</option>`).join('')}</select></label><label class="fi-form-field"><span>${M.esc(tr('field_key'))}</span><input data-prop="key" required pattern="[a-z][a-z0-9_]{0,63}" maxlength="64" ${disabled} value="${M.esc(field.key)}"><small>${M.esc(tr(locked ? 'field_key_locked' : 'field_key_help'))}</small></label><label class="fi-form-field is-check"><span>${M.esc(tr('required'))}</span><input type="checkbox" data-prop="required" ${field.required ? 'checked' : ''} ${disabled}></label><label class="fi-form-field" ${field.type === 'number' ? '' : 'hidden'}><span>${M.esc(tr('unit'))}</span><input data-prop="unit" maxlength="32" value="${M.esc(field.unit || '')}" ${disabled}></label><label class="fi-form-field" ${field.type === 'number' ? '' : 'hidden'}><span>${M.esc(tr('field_min'))}</span><input type="number" step="any" data-prop="min" value="${M.esc(field.min ?? '')}" ${disabled}></label><label class="fi-form-field" ${field.type === 'number' ? '' : 'hidden'}><span>${M.esc(tr('field_max'))}</span><input type="number" step="any" data-prop="max" value="${M.esc(field.max ?? '')}" ${disabled}></label><label class="fi-form-field is-wide" ${field.type === 'enum' ? '' : 'hidden'}><span>${M.esc(tr('field_options'))}</span><textarea data-prop="options" rows="3" ${field.type === 'enum' ? 'required' : ''}>${M.esc(asList(field.options).join('\n'))}</textarea><small>${M.esc(tr('field_options_help'))}</small></label></div><button type="button" class="fi-danger-button" data-remove-field="${index}" ${disabled}>${M.esc(tr('remove_field'))}</button></fieldset>`;
                }).join('');
                editor.querySelector('[data-add-field]').disabled = data.fields.length >= 128;
            }
            function close() {
                if (closed || saving) return;
                closed = true; read(); abort.abort(); options.signal?.removeEventListener('abort', dispose);
                editor.close(); editor.remove(); restore?.isConnected && restore.focus({ preventScroll: true });
            }
            function dispose() { saving = false; close(); }
            editor.addEventListener('cancel', event => { event.preventDefault(); close(); }, { signal: abort.signal });
            editor.querySelectorAll('[data-close]').forEach(button => button.addEventListener('click', close, { signal: abort.signal }));
            editor.addEventListener('input', read, { signal: abort.signal });
            editor.addEventListener('change', event => { read(); if (event.target.dataset.prop === 'type') rows(); }, { signal: abort.signal });
            editor.addEventListener('click', event => {
                const remove = event.target.closest('[data-remove-field]');
                if (remove) { read(); data.fields.splice(Number(remove.dataset.removeField), 1); rows(); editor.querySelector('[data-add-field]').focus(); }
            }, { signal: abort.signal });
            editor.querySelector('[data-add-field]').addEventListener('click', () => {
                read(); const keys = new Set(data.fields.map(field => field.key)); let index = data.fields.length + 1;
                while (keys.has('field_' + index)) index++;
                data.fields.push({ key: 'field_' + index, label: '', type: 'text', required: false }); rows();
                list.lastElementChild?.querySelector('[data-prop="label"]')?.focus();
            }, { signal: abort.signal });
            form.addEventListener('submit', async event => {
                event.preventDefault(); if (saving || !form.reportValidity()) return; read();
                const error = editor.querySelector('[data-error]'); error.hidden = true;
                const keys = new Set(data.fields.map(field => field.key));
                if (keys.size !== data.fields.length || data.fields.some(field => field.type === 'enum' && (!field.options.length || new Set(field.options).size !== field.options.length) || field.min != null && field.max != null && field.min > field.max)) {
                    error.textContent = tr('invalid_template_fields'); error.hidden = false; return;
                }
                const payload = M.clone(data);
                saving = true; form.querySelector('[type="submit"]').disabled = true;
                try {
                    const result = await apiCommand({ operation: template ? 'update' : 'create', entity: 'templates', id: template?.id, expected_version: template?.version, data: payload });
                    if (state.disposed) return;
                    const saved = result?.template || result;
                    state.catalog.templates = asList(state.catalog.templates).filter(value => value.id !== saved.id).concat(saved);
                    state.templates = asList(state.catalog.templates); state.template = saved; state.notice = tr(template ? 'template_saved' : 'template_created');
                    saving = false; close(); state.templateDraft = null; state.mobilePanel = 'detail'; draw();
                } catch (failure) {
                    error.textContent = tr(failure?.code === 'conflict' ? 'error_conflict' : 'error_invalid'); error.hidden = false;
                } finally { saving = false; if (editor.isConnected) form.querySelector('[type="submit"]').disabled = false; }
            }, { signal: abort.signal });
            options.signal?.addEventListener('abort', dispose, { once: true });
            rows(); editor.showModal(); form.elements.name.focus();
        }
        async function archiveTemplate() {
            const template = state.template; if (!template || template.built_in || !canWrite()) return;
            if (!await D.openConfirm(dialogOptions({ title: 'archive_template', confirmLabel: 'archive' }))) return;
            await withBusy(async () => {
                await apiCommand({ operation: 'archive', entity: 'templates', id: template.id, expected_version: template.version, data: {} });
                state.catalog.templates = asList(state.catalog.templates).filter(value => value.id !== template.id); state.template = null; state.notice = tr('template_archived'); draw();
            });
        }

        return { renderTemplates, templateForm, archiveTemplate };
    }
    window.FindItTemplates = { create };
})();
