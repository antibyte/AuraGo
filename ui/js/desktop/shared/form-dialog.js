(function () {
    'use strict';
    const escapeHTML = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'}[c]));
    let serial = 0;
    // Field: {id, label, type: 'text'|'url'|'number'|'select'|'checkbox'|'static'|'grid', value, required,
    // min, max, step, maxlength, placeholder, options: [{value, label}]}.
    //   static  a read-only row: label and value. It answers nothing.
    //   grid    rows x cols cells; pointing at a cell writes its row and column into the two number
    //           fields named by targets: [rowsId, colsId], and a click submits. It answers nothing.
    const silent = field => field.type === 'static' || field.type === 'grid';
    function fieldMarkup(field) {
        const name = 'data-form-field="' + escapeHTML(field.id) + '"', label = '<span>' + escapeHTML(field.label) + '</span>';
        if (field.type === 'static') return '<p class="vd-form-static">' + label + '<strong>' + escapeHTML(field.value ?? '') + '</strong></p>';
        if (field.type === 'grid') {
            let cells = '';
            for (let row = 1; row <= field.rows; row++) for (let col = 1; col <= field.cols; col++)
                cells += '<button type="button" tabindex="-1" data-form-cell="' + row + ',' + col + '" aria-label="' + row + ' × ' + col + '"></button>';
            return '<div class="vd-form-grid" data-form-grid="' + escapeHTML(field.id) + '" role="group" aria-label="' + escapeHTML(field.label) + '">' + cells + '</div>';
        }
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
    // open({title, fields, submitLabel, cancelLabel, extraLabel, validate, signal}) resolves to {id: value},
    // to {extra: true} when the extra button was pressed, or to null when the dialog is cancelled.
    // validate(values) returns a message to refuse them. Without cancelLabel there is no Cancel button.
    function open(options) {
        if (options.signal?.aborted) return Promise.resolve(null);
        const fields = options.fields || [], id = 'vd-form-dialog-' + (++serial);
        const node = document.createElement('dialog');
        node.className = 'vd-form-dialog';
        node.setAttribute('aria-labelledby', id);
        node.innerHTML = '<form method="dialog" novalidate><h2 id="' + id + '">' + escapeHTML(options.title) + '</h2>' + fields.map(fieldMarkup).join('') +
            '<p role="alert" data-form-error hidden></p><div class="vd-modal-actions">' +
            (options.extraLabel ? '<button type="button" class="vd-button" data-form-extra>' + escapeHTML(options.extraLabel) + '</button>' : '') +
            (options.cancelLabel ? '<button type="button" class="vd-button" data-form-cancel>' + escapeHTML(options.cancelLabel) + '</button>' : '') +
            '<button type="submit" class="vd-button vd-button-primary">' + escapeHTML(options.submitLabel) + '</button></div></form>';
        // The dialog lives on the body, so an app's own key handlers never see its keys.
        document.body.appendChild(node);
        // Keys stay in the dialog: the shell would run the active window's menu shortcuts otherwise.
        // Escape still closes it; that is the dialog's own cancel event, not a key handler.
        node.addEventListener('keydown', event => event.stopPropagation());
        return new Promise(resolve => {
            const form = node.querySelector('form'), error = node.querySelector('[data-form-error]');
            const input = fieldId => form.querySelector('[data-form-field="' + CSS.escape(fieldId) + '"]');
            let answer = null;
            const finish = value => { answer = value; node.close(); };
            const onAbort = () => finish(null);
            node.addEventListener('close', () => { options.signal?.removeEventListener('abort', onAbort); node.remove(); resolve(answer); }, {once: true});
            node.querySelector('[data-form-cancel]')?.addEventListener('click', () => finish(null));
            node.querySelector('[data-form-extra]')?.addEventListener('click', () => finish({extra: true}));
            for (const field of fields.filter(candidate => candidate.type === 'grid')) {
                const grid = form.querySelector('[data-form-grid="' + CSS.escape(field.id) + '"]'), [rows, cols] = field.targets.map(input);
                grid.style.setProperty('--vd-form-grid-cols', String(field.cols));
                const paint = () => {
                    for (const cell of grid.children) {
                        const [row, col] = cell.dataset.formCell.split(',').map(Number);
                        cell.toggleAttribute('data-on', row <= rows.valueAsNumber && col <= cols.valueAsNumber);
                    }
                };
                const point = event => {
                    const cell = event.target.closest('[data-form-cell]');
                    if (!cell) return false;
                    [rows.value, cols.value] = cell.dataset.formCell.split(',');
                    paint();
                    return true;
                };
                grid.addEventListener('pointerover', point);
                grid.addEventListener('click', event => { if (point(event)) form.requestSubmit(); });
                rows.addEventListener('input', paint);
                cols.addEventListener('input', paint);
                paint();
            }
            form.addEventListener('submit', event => {
                event.preventDefault();
                if (!form.reportValidity()) return;
                const values = {};
                for (const field of fields) if (!silent(field)) values[field.id] = read(field, input(field.id));
                const message = options.validate?.(values) || '';
                error.textContent = message;
                error.hidden = !message;
                if (!message) finish(values);
            });
            options.signal?.addEventListener('abort', onAbort, {once: true});
            // showModal traps the focus, closes on Escape and returns the focus on close.
            node.showModal();
            const first = node.querySelector('input[data-form-field],select[data-form-field]') || node.querySelector('[type=submit]');
            first.focus();
            first.select?.();
        });
    }
    window.AuraFormDialog = {open};
})();
