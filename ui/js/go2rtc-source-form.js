// Guided go2rtc camera source forms shared by Network Cameras and the config page.
// The server catalog (/api/go2rtc/source-types) describes the fields; this module
// renders them, builds the URL in memory and never writes a secret back into markup.
(function () {
    'use strict';

    const prefix = 'common.go2rtc_source.';
    const escape = value => String(value == null ? '' : value).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
    const enc = encodeURIComponent;
    let catalog = null;

    function loadCatalog(fetchTypes) {
        if (!catalog) {
            catalog = Promise.resolve(fetchTypes()).then(result => Array.isArray(result && result.types) ? result.types : []);
            catalog.catch(() => { catalog = null; });
        }
        return catalog;
    }

    function newState(type) {
        return { type: type || '', values: {}, error: '' };
    }

    function offered(types) {
        return (types || []).filter(type => type.mode === 'form' || type.mode === 'url');
    }

    function find(types, id) {
        return offered(types).find(type => type.id === id) || null;
    }

    function current(state, field) {
        const has = Object.prototype.hasOwnProperty.call(state.values, field.name);
        return String(has ? state.values[field.name] : (field.default || '')).trim();
    }

    function base64(text) {
        let binary = '';
        new TextEncoder().encode(text).forEach(byte => { binary += String.fromCharCode(byte); });
        return btoa(binary);
    }

    function hostPart(value) {
        const colons = (value.match(/:/g) || []).length;
        if (colons >= 2) return /^[0-9A-Fa-f:.%]+$/.test(value) ? '[' + value + ']' : null;
        if (colons === 1) return null;
        return /^[A-Za-z0-9.-]+$/.test(value) ? value : null;
    }

    function buildURL(types, state) {
        const type = find(types, state && state.type);
        if (!type) return { error: 'choose_type' };
        if (type.mode === 'url') {
            const url = String(state.values.url || '').trim();
            return url ? { url } : { error: 'required', field: 'url' };
        }
        const parts = { scheme: type.schemes[0], user: '', password: '', host: '', port: '', path: '', query: [] };
        for (const field of type.fields || []) {
            let value = current(state, field);
            if (!value) {
                if (field.required) return { error: 'required', field: field.name };
                continue;
            }
            if (field.kind === 'port' && !(/^\d{1,5}$/.test(value) && Number(value) >= 1 && Number(value) <= 65535)) return { error: 'invalid_port', field: field.name };
            if (field.kind === 'number' && !/^\d{1,6}$/.test(value)) return { error: 'invalid_number', field: field.name };
            if (field.kind === 'select' && Array.isArray(field.options) && field.options.length && !field.options.includes(value)) return { error: 'invalid_choice', field: field.name };
            if (field.kind === 'host') {
                value = hostPart(value);
                if (!value) return { error: 'invalid_host', field: field.name };
            }
            if (field.encode === 'base64') value = base64(value);
            const placement = field.placement || '';
            if (placement === 'scheme') parts.scheme = value;
            else if (placement === 'user') parts.user = value;
            else if (placement === 'password') parts.password = value;
            else if (placement === 'host') parts.host = value;
            else if (placement === 'port') parts.port = value;
            else if (placement === 'path') {
                if (value.includes('#')) return { error: 'invalid_path', field: field.name };
                parts.path = value.startsWith('/') ? value : '/' + value;
            } else if (placement.startsWith('query:')) parts.query.push(enc(placement.slice(6)) + '=' + enc(value));
        }
        let userinfo = '';
        if (parts.user && parts.password) userinfo = enc(parts.user) + ':' + enc(parts.password) + '@';
        else if (parts.user) userinfo = enc(parts.user) + '@';
        else if (parts.password && type.password_only_userinfo) userinfo = enc(parts.password) + '@';
        else if (parts.password) return { error: 'required', field: 'username' };
        const query = parts.query.length ? '?' + parts.query.join('&') : '';
        if (!parts.host) return { url: parts.scheme + ':' + query };
        return { url: parts.scheme + '://' + userinfo + parts.host + (parts.port ? ':' + parts.port : '') + parts.path + query };
    }

    function errorText(t, result) {
        const field = result.field ? t(prefix + 'field_' + result.field) : '';
        return t(prefix + result.error, { field });
    }

    function fieldControl(field, state, t, c) {
        const label = t(prefix + 'field_' + field.name) + (field.required ? ' *' : '');
        const attrs = ' class="' + escape(c.input) + '" data-source-field="' + escape(field.name) + '"';
        let control;
        if (field.kind === 'select') {
            const selected = current(state, field);
            control = '<select' + attrs + '>' + (field.options || []).map(option => '<option value="' + escape(option) + '"' + (option === selected ? ' selected' : '') + '>' + escape(option === '' ? t(prefix + 'option_default') : option) + '</option>').join('') + '</select>';
        } else if (field.kind === 'secret') {
            control = '<input' + attrs + ' type="password" autocomplete="new-password" value="">';
        } else {
            const numeric = field.kind === 'port' || field.kind === 'number' ? ' inputmode="numeric"' : '';
            const placeholder = field.kind === 'host' ? ' placeholder="192.168.1.20"' : '';
            control = '<input' + attrs + ' type="text" autocomplete="off"' + numeric + placeholder + ' value="' + escape(current(state, field)) + '">';
        }
        return '<label class="' + escape(c.field) + '"><span class="' + escape(c.label) + '">' + escape(label) + '</span>' + control + '</label>';
    }

    function markup(types, state, options) {
        const t = options.t;
        const c = Object.assign({ field: '', label: '', input: '' }, options.classes || {});
        const type = find(types, state.type);
        const choices = '<option value="">' + escape(options.emptyLabel || t(prefix + 'choose_type')) + '</option>' +
            offered(types).map(item => '<option value="' + escape(item.id) + '"' + (type && item.id === type.id ? ' selected' : '') + '>' + escape(item.label) + '</option>').join('');
        let html = '<label class="' + escape(c.field) + '"><span class="' + escape(c.label) + '">' + escape(t(prefix + 'type')) + '</span><select class="' + escape(c.input) + '" data-source-type>' + choices + '</select></label>';
        if (type) {
            if (type.hint) html += '<p class="go2rtc-source-hint">' + escape(t(prefix + 'hint_' + type.id)) + '</p>';
            if (type.mode === 'url') {
                html += '<label class="' + escape(c.field) + '"><span class="' + escape(c.label) + '">' + escape(t(prefix + 'field_url')) + ' *</span><input class="' + escape(c.input) + '" data-source-field="url" type="password" autocomplete="off" value="" placeholder="' + escape(type.template || '') + '"></label>';
            } else {
                html += (type.fields || []).map(field => fieldControl(field, state, t, c)).join('');
            }
            if (type.docs_url) html += '<p class="go2rtc-source-docs"><a href="' + escape(type.docs_url) + '" target="_blank" rel="noopener noreferrer">' + escape(t(prefix + 'docs')) + '</a></p>';
        }
        html += '<p class="go2rtc-source-error" role="alert"' + (state.error ? '' : ' hidden') + '>' + escape(state.error || '') + '</p>';
        return '<div class="go2rtc-source-form" data-source-form>' + html + '</div>';
    }

    function bind(root, state, onChange) {
        const select = root.querySelector('[data-source-type]');
        if (select) select.addEventListener('change', () => {
            state.type = select.value;
            state.values = {};
            state.error = '';
            onChange(true);
        });
        root.querySelectorAll('[data-source-field]').forEach(input => input.addEventListener('input', () => {
            state.values[input.dataset.sourceField] = input.value;
            state.error = '';
            onChange(false);
        }));
    }

    function label(types, id) {
        const type = (types || []).find(item => item.id === id);
        return type ? type.label : String(id || '').toUpperCase();
    }

    window.AuraGoSourceForm = Object.freeze({ loadCatalog, newState, buildURL, errorText, markup, bind, label });
})();
