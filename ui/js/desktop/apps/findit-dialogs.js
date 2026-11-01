(function () {
    'use strict';

    const esc = value => String(value ?? '').replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[char]);
    const asArray = value => Array.isArray(value) ? value : [];
    function fieldMarkup(field, value, t, instanceId) {
        const key = String(field.key || 'field').replace(/[^a-zA-Z0-9_-]/g, '-');
        const id = `${instanceId}-field-${key}`, label = t(field.labelKey || field.label || field.key || 'field');
        const step = field.step ?? (field.type === 'number' ? 'any' : null);
        const common = `id="${id}" name="${esc(field.key)}" ${field.required ? 'required' : ''} ${field.disabled ? 'disabled' : ''} ${field.min != null ? `min="${esc(field.min)}"` : ''} ${field.max != null ? `max="${esc(field.max)}"` : ''} ${step != null ? `step="${esc(step)}"` : ''} ${field.placeholder ? `placeholder="${esc(t(field.placeholder))}"` : ''}`;
        let control;
        if (field.type === 'textarea' || field.type === 'csv') {
            control = `<textarea ${common} rows="${Number(field.rows) || (field.type === 'csv' ? 7 : 3)}">${esc(value ?? '')}</textarea>`;
        } else if (field.type === 'select') {
            const options = asArray(field.options).map(option => {
                const idValue = typeof option === 'object' ? String(option.value ?? option.id ?? '') : String(option);
                const optionLabel = typeof option === 'object' ? String(option.label || option.name || idValue) : String(option);
                return `<option value="${esc(idValue)}" ${String(value ?? '') === idValue ? 'selected' : ''} ${option?.disabled ? 'disabled' : ''}>${esc(optionLabel)}</option>`;
            }).join('');
            control = `<select ${common}>${field.allowEmpty ? `<option value="">${esc(t(field.emptyLabel || 'none'))}</option>` : ''}${options}</select>`;
        } else if (field.type === 'checkbox') {
            control = `<input type="checkbox" ${common} ${value ? 'checked' : ''}>`;
        } else if (field.type === 'file') {
            control = `<input type="file" ${common} accept="${esc(field.accept || '*/*')}" ${field.capture ? 'capture="environment"' : ''}>`;
        } else {
            const type = ['text', 'number', 'url', 'email', 'date', 'search'].includes(field.type) ? field.type : 'text';
            control = `<input type="${type}" ${common} value="${esc(value ?? '')}" ${field.inputmode ? `inputmode="${esc(field.inputmode)}"` : ''}>`;
        }
        return `<label class="fi-form-field ${field.wide ? 'is-wide' : ''} ${field.type === 'checkbox' ? 'is-check' : ''}" for="${id}"><span>${esc(label)}${field.unit ? ` (${esc(field.unit)})` : ''}${field.required ? '<b aria-hidden="true"> *</b>' : ''}</span>${control}${field.help ? `<small>${esc(t(field.help))}</small>` : ''}</label>`;
    }
    function fieldsMarkup(fields, values, t, instanceId, advancedOpen = false) {
        const basic = fields.filter(field => !field.advanced || field.required);
        const advanced = fields.filter(field => field.advanced && !field.required);
        return basic.map(field => fieldMarkup(field, values[field.key], t, instanceId)).join('') + (advanced.length
            ? `<details class="fi-form-advanced" data-advanced-fields ${advancedOpen ? 'open' : ''}><summary>${esc(t('technical_filters'))}</summary><div class="fi-form-grid">${advanced.map(field => fieldMarkup(field, values[field.key], t, instanceId)).join('')}</div></details>`
            : '');
    }
    function closeDialog(dialog) {
        if (!dialog.isConnected) return;
        if (dialog.open) dialog.close();
        dialog.remove();
    }
    function openForm(options) {
        const host = options.host || document.body, t = options.t || (key => key), restore = document.activeElement;
        const instanceId = 'fi-' + crypto.randomUUID(), titleId = instanceId + '-form-title';
        const dialog = document.createElement('dialog'); dialog.className = 'fi-dialog'; dialog.setAttribute('aria-labelledby', titleId);
        const hostWidth = host.clientWidth || host.getBoundingClientRect?.().width || 0;
        dialog.dataset.hostWidth = hostWidth > 0 && hostWidth <= 760 ? 'narrow' : 'wide';
        if (hostWidth > 0) dialog.style.setProperty('--fi-host-width', `${hostWidth}px`);
        let fields = asArray(options.fields), values = { ...(options.values || {}) };
        dialog.innerHTML = `<form method="dialog" class="fi-dialog-form"><header class="fi-dialog-head"><div><p class="fi-kicker">${esc(t(options.kicker || 'edit_details'))}</p><h2 id="${esc(titleId)}">${esc(t(options.title))}</h2>${options.description ? `<p>${esc(t(options.description))}</p>` : ''}</div><button type="button" data-close aria-label="${esc(t('close'))}">×</button></header><div class="fi-dialog-body"><div class="fi-form-grid" data-form-fields>${fieldsMarkup(fields, values, t, instanceId)}</div>${options.contentHTML || ''}<p class="fi-form-error" data-error role="alert" hidden></p></div><footer class="fi-dialog-foot"><button type="button" data-cancel>${esc(t('cancel'))}</button>${options.secondarySubmitLabel ? `<button type="submit" class="fi-tool-button" data-submit-action="next">${esc(t(options.secondarySubmitLabel))}</button>` : ''}<button type="submit" class="fi-primary" data-submit>${esc(t(options.submitLabel || 'save'))}</button></footer></form>`;
        host.append(dialog);
        const form = dialog.querySelector('form'), abort = new AbortController(), signal = abort.signal;
        let settled = false;
        const finish = result => {
            if (settled) return;
            settled = true; abort.abort(); closeDialog(dialog); options.signal?.removeEventListener('abort', cancel);
            if (restore?.isConnected) restore.focus({ preventScroll: true });
            options.onClose?.();
            resolve(result);
        };
        let resolve;
        const promise = new Promise(done => { resolve = done; });
        const cancel = () => finish(null);
        dialog.addEventListener('cancel', event => { event.preventDefault(); cancel(); }, { signal });
        dialog.addEventListener('close', cancel, { signal });
        dialog.querySelector('[data-close]').addEventListener('click', cancel, { signal });
        dialog.querySelector('[data-cancel]').addEventListener('click', cancel, { signal });
        if (typeof options.fieldsFor === 'function' && options.fieldsWatch) {
            dialog.addEventListener('change', event => {
                if (event.target.name !== options.fieldsWatch) return;
                for (const field of fields) {
                    const element = form.elements.namedItem(field.key);
                    if (element) values[field.key] = field.type === 'checkbox' ? element.checked : element.value;
                }
                const focusName = event.target.name, focusValue = event.target.value;
                const advancedOpen = !!dialog.querySelector('[data-advanced-fields]')?.open;
                const valuePatch = options.valuesFor?.(focusValue, { ...values });
                if (valuePatch && typeof valuePatch === 'object') values = { ...values, ...valuePatch };
                const nextFields = asArray(options.fieldsFor(focusValue, { ...values }));
                const available = new Set(nextFields.map(field => field.key));
                for (const key of Object.keys(values)) if (!available.has(key)) delete values[key];
                fields = nextFields;
                dialog.querySelector('[data-form-fields]').innerHTML = fieldsMarkup(fields, values, t, instanceId, advancedOpen);
                const replacement = form.elements.namedItem(focusName);
                if (replacement) replacement.value = focusValue;
            }, { signal });
        }
        if (typeof options.onFieldChange === 'function') {
            dialog.addEventListener('change', event => options.onFieldChange(event.target.name, event.target.value, { ...values }), { signal });
        }
        form.addEventListener('submit', event => {
            event.preventDefault();
            const errorNode = dialog.querySelector('[data-error]'); errorNode.hidden = true; errorNode.textContent = '';
            const result = {};
            for (const field of fields) {
                const element = form.elements.namedItem(field.key);
                if (!element) continue;
                let value;
                if (field.type === 'checkbox') value = element.checked;
                else if (field.type === 'file') value = element.files?.[0] || null;
                else value = element.value;
                if (field.type === 'number' && value !== '') value = Number(value);
                if (field.type === 'textarea' && field.valueType === 'lines') value = value.split(/\r?\n/).map(line => line.trim()).filter(Boolean);
                if (field.type === 'textarea' && field.valueType === 'csv') value = value.split(',').map(item => item.trim()).filter(Boolean);
                if (field.required && (value == null || (typeof value === 'string' && !value.trim()) || (Array.isArray(value) && !value.length))) {
                    errorNode.textContent = t('required_field'); errorNode.hidden = false; element.focus(); return;
                }
                if (field.type === 'number' && value !== '' && !Number.isFinite(value)) {
                    errorNode.textContent = t('invalid_number'); errorNode.hidden = false; element.focus(); return;
                }
                if (field.type === 'url' && value) {
                    try { const url = new URL(value); if (!['http:', 'https:'].includes(url.protocol)) throw new Error('scheme'); }
                    catch (_) { errorNode.textContent = t('invalid_url'); errorNode.hidden = false; element.focus(); return; }
                }
                result[field.key] = value;
            }
            let validation;
            try { validation = options.validate?.(result); }
            catch (_) { validation = { message: 'error_generic' }; }
            if (validation) {
                const message = typeof validation === 'string' ? validation : validation.message || 'error_generic';
                errorNode.textContent = t(message); errorNode.hidden = false;
                const invalid = validation.field ? form.elements.namedItem(validation.field) : null;
                invalid?.focus();
                return;
            }
            if (event.submitter?.dataset.submitAction === 'next') result.__action = 'next';
            finish(result);
        }, { signal });
        if (options.signal) {
            options.signal.addEventListener('abort', cancel, { once: true });
            if (options.signal.aborted) cancel();
        }
        if (!settled) { dialog.showModal(); dialog.querySelector('input:not([type=checkbox]),select,textarea')?.focus({ preventScroll: true }); }
        return promise;
    }
    function openConfirm(options) {
        return openForm({ ...options, fields: [], title: options.title, description: options.message, submitLabel: options.confirmLabel || 'confirm' }).then(value => value !== null);
    }
    function openInfo(options) {
        const host = options.host || document.body, t = options.t || (key => key), restore = document.activeElement;
        const titleId = 'fi-' + crypto.randomUUID() + '-info-title';
        const dialog = document.createElement('dialog'); dialog.className = 'fi-dialog fi-info-dialog'; dialog.setAttribute('aria-labelledby', titleId);
        const hostWidth = host.clientWidth || host.getBoundingClientRect?.().width || 0;
        dialog.dataset.hostWidth = hostWidth > 0 && hostWidth <= 760 ? 'narrow' : 'wide';
        if (hostWidth > 0) dialog.style.setProperty('--fi-host-width', `${hostWidth}px`);
        dialog.innerHTML = `<form method="dialog" class="fi-dialog-form"><header class="fi-dialog-head"><div><p class="fi-kicker">${esc(t(options.kicker || 'findit'))}</p><h2 id="${esc(titleId)}">${esc(t(options.title))}</h2></div><button type="button" data-close aria-label="${esc(t('close'))}">×</button></header><div class="fi-dialog-body">${options.html || `<p>${esc(t(options.message || ''))}</p>`}</div><footer class="fi-dialog-foot"><button type="button" class="fi-primary" data-close>${esc(t('close'))}</button></footer></form>`;
        host.append(dialog);
        let closed = false;
        const close = () => {
            if (closed) return;
            closed = true;
            closeDialog(dialog);
            options.signal?.removeEventListener('abort', close);
            if (restore?.isConnected) restore.focus({ preventScroll: true });
        };
        if (options.signal) {
            options.signal.addEventListener('abort', close, { once: true });
            if (options.signal.aborted) close();
        }
        if (!closed) dialog.showModal();
        dialog.addEventListener('cancel', event => { event.preventDefault(); close(); }, { once: true });
        dialog.querySelectorAll('[data-close]').forEach(button => button.addEventListener('click', close, { once: true }));
        return { element: dialog, close };
    }
    window.FindItDialogs = { openForm, openConfirm, openInfo };
})();
