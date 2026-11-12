(function () {
    'use strict';
    const escapeHTML = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'}[c]));
    let serial = 0;
    // Field: {id, label, type: 'text'|'url'|'number'|'select'|'checkbox', value, required,
    // min, max, step, maxlength, placeholder, options: [{value, label}]}.
    function fieldMarkup(field) {
        const name = 'data-form-field="' + escapeHTML(field.id) + '"', label = '<span>' + escapeHTML(field.label) + '</span>';
        if (field.type === 'checkbox') return '<label class="vd-form-check"><input type="checkbox" ' + name + (field.value ? ' checked' : '') + '>' + label + '</label>';
        if (field.type === 'select') return '<label>' + label + '<select class="vd-modal-input" ' + name + '>' + (field.options || []).map(option =>
            '<option value="' + escapeHTML(option.value) + '"' + (String(option.value) === String(field.value ?? '') ? ' selected' : '') + '>' + escapeHTML(option.label) + '</option>').join('') + '</select></label>';
        const limits = ['min', 'max', 'step', 'maxlength', 'placeholder'].filter(key => field[key] !== undefined && field[key] !== null)
            .map(key => ' ' + key + '="' + escapeHTML(field[key]) + '"').join('');
        return '<label>' + label + '<input class="vd-modal-input" type="' + (['number', 'url'].includes(field.type) ? field.type : 'text') + '" ' + name +
            ' value="' + escapeHTML(field.value ?? '') + '"' + limits + (field.required ? ' required' : '') + ' autocomplete="off"></label>';
    }
    // A number answers as a Number, a checkbox as a boolean, everything else as a trimmed string.
    function read(field, el) {
        if (field.type === 'checkbox') return el.checked;
        if (field.type === 'number') return el.valueAsNumber;
        return el.value.trim();
    }
    // open({title, fields, submitLabel, cancelLabel, validate, signal}) resolves to {id: value}
    // or to null when the dialog is cancelled. validate(values) returns a message to refuse them.
    function open(options) {
        if (options.signal?.aborted) return Promise.resolve(null);
        const fields = options.fields || [], id = 'vd-form-dialog-' + (++serial);
        const node = document.createElement('dialog');
        node.className = 'vd-form-dialog';
        node.setAttribute('aria-labelledby', id);
        node.innerHTML = '<form method="dialog" novalidate><h2 id="' + id + '">' + escapeHTML(options.title) + '</h2>' + fields.map(fieldMarkup).join('') +
            '<p role="alert" data-form-error hidden></p><div class="vd-modal-actions">' +
            '<button type="button" class="vd-button" data-form-cancel>' + escapeHTML(options.cancelLabel) + '</button>' +
            '<button type="submit" class="vd-button vd-button-primary">' + escapeHTML(options.submitLabel) + '</button></div></form>';
        // The dialog lives on the body, so an app's own key handlers never see its keys.
        document.body.appendChild(node);
        return new Promise(resolve => {
            const form = node.querySelector('form'), error = node.querySelector('[data-form-error]');
            let answer = null;
            const finish = value => { answer = value; node.close(); };
            const onAbort = () => finish(null);
            node.addEventListener('close', () => { options.signal?.removeEventListener('abort', onAbort); node.remove(); resolve(answer); }, {once: true});
            node.querySelector('[data-form-cancel]').addEventListener('click', () => finish(null));
            form.addEventListener('submit', event => {
                event.preventDefault();
                if (!form.reportValidity()) return;
                const values = {};
                for (const field of fields) values[field.id] = read(field, form.querySelector('[data-form-field="' + CSS.escape(field.id) + '"]'));
                const message = options.validate?.(values) || '';
                error.textContent = message;
                error.hidden = !message;
                if (!message) finish(values);
            });
            options.signal?.addEventListener('abort', onAbort, {once: true});
            // showModal traps the focus, closes on Escape and returns the focus on close.
            node.showModal();
            const first = node.querySelector('[data-form-field]');
            first?.focus();
            first?.select?.();
        });
    }
    window.AuraFormDialog = {open};
})();
