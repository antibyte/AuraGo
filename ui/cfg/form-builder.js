// Shared form builder for config section modules.
(function () {
    'use strict';
    let fieldID = 0;

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
        const tier = options.advanced ? ' data-tier="advanced"' : '';
        const id = options.id || 'cfg-form-' + (++fieldID);
        const helpID = id + '-help';
        const labelID = id + '-label';
        // Preserve integration-owned IDs and request bindings; add accessible names only.
        controlHTML = controlHTML.replace(/<(input|select|textarea)\b([^>]*)>/, (tag, name, attrs) => {
            if (!/\bid=/.test(attrs)) attrs += ' id="' + attr(id) + '"';
            attrs += ' aria-labelledby="' + attr(labelID) + '"';
            if (help) attrs += ' aria-describedby="' + attr(helpID) + '"';
            return '<' + name + attrs + '>';
        });
        return '<div class="field-group pw-field' + extraClass + '"' + tier + '>'
            + '<div class="field-label" id="' + attr(labelID) + '">' + html(labelText(options)) + '</div>'
            + controlHTML
            + (help ? '<div class="field-help" id="' + attr(helpID) + '">' + html(help) + '</div>' : '')
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

    function panel(options) {
        options = options || {};
        const title = options.title != null ? options.title : (options.titleKey ? t(options.titleKey) : '');
        const desc = options.desc != null ? options.desc : (options.descKey ? t(options.descKey) : '');
        const content = options.content != null ? options.content : (options.html || '');
        return '<section class="pw-panel cfg-topic' + (options.className ? ' ' + attr(options.className) : '') + '">'
            + (title ? '<div class="pw-panel-heading"><h2>' + html(title) + '</h2>' + (desc ? '<p>' + html(desc) + '</p>' : '') + '</div>' : '')
            + '<div class="pw-panel-body">' + content + '</div>'
            + '</section>';
    }

    function disclosure(options) {
        options = options || {};
        const title = options.title != null ? options.title : t(options.titleKey || 'config.precision.advanced_title');
        const desc = options.desc != null ? options.desc : (options.descKey ? t(options.descKey) : '');
        return '<details class="pw-disclosure' + (options.className ? ' ' + attr(options.className) : '') + '"' + (options.open ? ' open' : '') + '>'
            + '<summary><span><strong>' + html(title) + '</strong>' + (desc ? '<small>' + html(desc) + '</small>' : '') + '</span><span aria-hidden="true">+</span></summary>'
            + '<div class="pw-disclosure-body">' + (options.content || options.html || '') + '</div>'
            + '</details>';
    }

    function status(options) {
        options = options || {};
        const kind = options.kind ? ' is-' + attr(options.kind) : '';
        const message = options.text != null ? options.text : (options.textKey ? t(options.textKey) : '');
        return '<div class="pw-status' + kind + '" role="status" aria-live="polite">' + html(message) + '</div>';
    }

    function emptyState(options) {
        options = options || {};
        const title = options.title != null ? options.title : (options.titleKey ? t(options.titleKey) : '');
        const desc = options.desc != null ? options.desc : (options.descKey ? t(options.descKey) : '');
        return '<div class="pw-empty-state"><strong>' + html(title) + '</strong>' + (desc ? '<p>' + html(desc) + '</p>' : '') + (options.actionHTML || '') + '</div>';
    }

    function modal(options) {
        options = options || {};
        const title = options.title != null ? options.title : (options.titleKey ? t(options.titleKey) : '');
        const labelledBy = options.titleId || 'pw-modal-title';
        return '<div class="modal-overlay pw-modal-overlay' + (options.open ? ' open active' : '') + '">'
            + '<div class="modal-card pw-modal-card" role="dialog" aria-modal="true" aria-labelledby="' + attr(labelledBy) + '">'
            + '<h2 class="modal-title" id="' + attr(labelledBy) + '">' + html(title) + '</h2>'
            + '<div class="pw-modal-body">' + (options.content || options.html || '') + '</div>'
            + (options.actionsHTML ? '<div class="modal-actions">' + options.actionsHTML + '</div>' : '')
            + '</div></div>';
    }

    function actions(items) {
        const row = (items || []).map(item => item.html || '').join('');
        return '<div class="cfg-actions-row pw-action-row">' + row + '</div>';
    }

    function section(spec) {
        return renderSpec(spec);
    }

    function renderSpec(spec) {
        spec = spec || {};
        let out = '<div class="cfg-section active">';
        out += '<h1 class="section-header">' + html(spec.label || '') + '</h1>';
        if (spec.desc) out += '<div class="section-desc">' + html(spec.desc) + '</div>';
        if (spec.beforeHTML) out += spec.beforeHTML;
        let fields = '';
        (spec.fields || []).forEach(item => {
            fields += typeof item === 'string' ? item : field(item);
        });
        if (fields) out += panel({ titleKey: 'config.refresh.settings', content: fields, className: 'pw-section-panel' });
        (spec.groups || []).forEach(group => {
            out += panel(Object.assign({}, group, {
                content: (group.fields || []).map(item => typeof item === 'string' ? item : field(item)).join('') + (group.content || '')
            }));
        });
        if (spec.afterHTML) out += spec.afterHTML;
        out += '</div>';
        return out;
    }

    // Existing renderers declare topic boundaries with group headings. Materialize
    // those boundaries once, preserving nodes, IDs, listeners and conditional wrappers.
    // Field nesting is never used to decide whether something is a card.
    function layout(root, key) {
        if (!root || root.classList.contains('pw-overview')) return;
        const metadata = window.AuraConfigCatalog?.presentation?.[key] || {};
        const markers = '.cfg-group-title, .field-group-title, [data-config-topic-title]' + (metadata.markers ? ', ' + metadata.markers : '');
        const surfaces = '.cfg-topic, .cfg-object, [data-config-surface], .pw-panel';
        const apply = (selector, className) => {
            if (selector) root.querySelectorAll(selector).forEach(node => node.classList.add(className));
        };
        apply(metadata.topics, 'cfg-topic');
        apply(metadata.objects, 'cfg-object');
        apply(metadata.headings, 'cfg-topic-heading');
        apply(metadata.bodies, 'pw-panel-body');
        // These are existing, explicitly named shared topic/object components.
        apply('.cfg-card', 'cfg-topic');
        apply('.cfg-card-title', 'cfg-topic-heading');
        if (root.dataset.topicsReady) return;
        root.dataset.topicsReady = 'true';
        function create(title) {
            const template = document.createElement('template');
            template.innerHTML = panel({ title: title?.replace(/^[\p{Extended_Pictographic}\uFE0F\s]+/u, '') || t('config.refresh.settings') });
            return template.content.firstElementChild;
        }
        function arrange(parent) {
            let current = null;
            [...parent.children].forEach(node => {
                if (node.matches(markers)) {
                    current = create(node.textContent.trim());
                    node.before(current);
                    node.remove();
                    return;
                }
                if (node.matches(surfaces + ', .wh-tabs, .tabs, .cfg-error-state, .pw-empty-state, [role="dialog"], .modal-overlay')) {
                    current = null;
                    return;
                }
                const title = node.querySelector(':scope > .field-group-title, :scope > [data-config-topic-title]');
                if (title) {
                    node.classList.add('cfg-topic');
                    title.classList.add('cfg-topic-heading');
                    title.setAttribute('role', 'heading');
                    title.setAttribute('aria-level', '2');
                    current = null;
                    return;
                }
                if (node.matches('.section-header, .cfg-page-heading') || (!current && node.matches('.section-desc, .adg-status-banner, [role="status"]'))) {
                    current = null;
                    return;
                }
                // Runtime capability wrappers must remain around their fields.
                if (node.matches('.feature-unavailable-fields, .speech-lab-section')) {
                    arrange(node);
                    current = null;
                    return;
                }
                if (!current && node.matches('.field-group, .field-grid, .cfg-toggle-row, .cfg-toggle-row-highlight, [data-config-fields]')) {
                    current = create(parent.dataset.configIntro || t('config.refresh.settings'));
                    node.before(current);
                }
                if (current) current.querySelector('.pw-panel-body').append(node);
            });
        }
        arrange(root);
        if (metadata.flows) root.querySelectorAll(metadata.flows).forEach(arrange);
        root.querySelectorAll('.cfg-topic > .field-group-title').forEach(title => {
            title.classList.add('cfg-topic-heading');
            title.setAttribute('role', 'heading');
            title.setAttribute('aria-level', '2');
        });
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
        panel,
        disclosure,
        status,
        emptyState,
        modal,
        actions,
        renderSpec,
        layout
    };
})();
