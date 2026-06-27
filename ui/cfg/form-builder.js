// Shared form builder for config section modules.
(function () {
    'use strict';

    function html(value) {
        return typeof escapeHtml === 'function' ? escapeHtml(value == null ? '' : String(value)) : String(value == null ? '' : value);
    }

    function attr(value) {
        return typeof escapeAttr === 'function' ? escapeAttr(value == null ? '' : String(value)) : html(value).replace(/"/g, '&quot;');
    }

    function labelText(options) {
        return options.label != null ? options.label : t(options.labelKey || '');
    }

    function helpText(options) {
        if (options.help != null) return options.help;
        return options.helpKey ? t(options.helpKey) : '';
    }

    function fieldShell(options, controlHTML) {
        const help = helpText(options);
        const extraClass = options.groupClass ? ' ' + attr(options.groupClass) : '';
        return '<div class="field-group' + extraClass + '">'
            + '<div class="field-label">' + html(labelText(options)) + '</div>'
            + (help ? '<div class="field-help">' + html(help) + '</div>' : '')
            + controlHTML
            + '</div>';
    }

    function field(options) {
        options = options || {};
        const type = options.type || 'text';
        if (type === 'toggle') return toggle(options);
        if (type === 'select') return select(options);
        if (type === 'textarea') return textarea(options);
        if (type === 'password') return password(options);
        if (type === 'number') return number(options);
        const value = options.value != null ? options.value : '';
        const placeholder = options.placeholder != null ? options.placeholder : '';
        const id = options.id ? ' id="' + attr(options.id) + '"' : '';
        const css = options.className ? ' field-input ' + attr(options.className) : ' field-input';
        return fieldShell(options,
            '<input class="' + css.trim() + '" type="' + attr(type) + '"' + id
            + ' data-path="' + attr(options.path || '') + '" value="' + attr(value) + '" placeholder="' + attr(placeholder) + '">');
    }

    function toggle(options) {
        options = options || {};
        const on = options.value === true;
        return fieldShell(options,
            '<div class="toggle-wrap">'
            + '<div class="toggle' + (on ? ' on' : '') + '" data-path="' + attr(options.path || '') + '" onclick="toggleBool(this)"></div>'
            + '<span class="toggle-label">' + html(on ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>'
            + '</div>');
    }

    function select(options) {
        options = options || {};
        const selected = options.value != null ? String(options.value) : '';
        const rendered = (options.options || []).map(option => {
            const value = option.value != null ? String(option.value) : '';
            return '<option value="' + attr(value) + '"' + (value === selected ? ' selected' : '') + '>' + html(option.label) + '</option>';
        }).join('');
        const onChange = options.onchange ? ' onchange="' + attr(options.onchange) + '"' : '';
        return fieldShell(options,
            '<select class="field-select" data-path="' + attr(options.path || '') + '"' + onChange + '>' + rendered + '</select>');
    }

    function textarea(options) {
        options = options || {};
        const value = options.value != null ? options.value : '';
        const rows = options.rows || 4;
        return fieldShell(options,
            '<textarea class="field-input" data-path="' + attr(options.path || '') + '" rows="' + attr(rows) + '" placeholder="' + attr(options.placeholder || '') + '">' + html(value) + '</textarea>');
    }

    function password(options) {
        options = options || {};
        const value = typeof cfgSecretValue === 'function' ? cfgSecretValue(options.value) : (options.value || '');
        const placeholder = options.placeholder != null ? options.placeholder : (typeof cfgSecretPlaceholder === 'function' ? cfgSecretPlaceholder(options.value, '') : '');
        const id = options.id ? ' id="' + attr(options.id) + '"' : '';
        return fieldShell(options,
            '<div class="adg-password-row">'
            + '<div class="password-wrap cfg-password-input">'
            + '<input class="field-input adg-password-input" type="password"' + id + ' value="' + attr(value) + '" placeholder="' + attr(placeholder) + '" autocomplete="off">'
            + '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + (typeof EYE_OPEN_SVG !== 'undefined' ? EYE_OPEN_SVG : '') + '</button>'
            + '</div>'
            + (options.actionHTML || '')
            + '</div>');
    }

    function number(options) {
        options = Object.assign({}, options, { type: 'number' });
        let control = '<input class="field-input" type="number" data-path="' + attr(options.path || '') + '" value="' + attr(options.value != null ? options.value : '') + '"';
        if (options.min != null) control += ' min="' + attr(options.min) + '"';
        if (options.max != null) control += ' max="' + attr(options.max) + '"';
        if (options.step != null) control += ' step="' + attr(options.step) + '"';
        control += '>';
        return fieldShell(options, control);
    }

    function note(options) {
        options = options || {};
        const kind = options.kind ? ' is-' + attr(options.kind) : '';
        const text = options.text != null ? options.text : t(options.textKey || '');
        return '<div class="cfg-note' + kind + '">' + html(text) + '</div>';
    }

    function actions(items) {
        const row = (items || []).map(item => item.html || '').join('');
        return '<div class="field-group cfg-actions-row">' + row + '</div>';
    }

    function section(spec) {
        return renderSpec(spec);
    }

    function renderSpec(spec) {
        spec = spec || {};
        let out = '<div class="cfg-section active">';
        out += '<div class="section-header">' + html(spec.label || '') + '</div>';
        if (spec.desc) out += '<div class="section-desc">' + html(spec.desc) + '</div>';
        if (spec.beforeHTML) out += spec.beforeHTML;
        (spec.fields || []).forEach(item => {
            out += typeof item === 'string' ? item : field(item);
        });
        if (spec.afterHTML) out += spec.afterHTML;
        out += '</div>';
        return out;
    }

    window.AuraConfigForm = {
        section,
        field,
        toggle,
        select,
        textarea,
        password,
        number,
        note,
        actions,
        renderSpec
    };
})();
