(function () {
    'use strict';

    const SETTING_KEY = 'retronet.entries';
    const MAX_ENTRIES = 64;
    const MAX_BYTES = 64 * 1024;
    const NAME_MAX = 40;
    const DESCRIPTION_MAX = 80;
    const ID_LENGTH = 12;
    const ID_ALPHABET = 'abcdefghijklmnopqrstuvwxyz0123456789';
    const OWN_ID = /^own-[a-z0-9]{8,32}$/;
    const BLOCKED_PORTS = [25, 465, 587];
    const KINDS = ['bbs', 'world'];
    const CHARSETS = [['utf8', 'UTF-8'], ['cp437', 'CP437 (IBM PC)'], ['latin1', 'ISO 8859-1 (Latin-1)']];
    const CHARSET_VALUES = ['utf8', 'cp437', 'latin1'];
    // UX mirror of internal/retronet (entries.go); the server stays the authority.
    // hasInvisible: categories Cc, Cf, Co, Zl, Zp and U+FFFD, U+200D exempt; lone surrogates (Cs) reach Go as U+FFFD.
    const INVISIBLE = /[\p{Cc}\p{Co}\p{Cs}\p{Zl}\p{Zp}\ufffd]|(?!\u200d)\p{Cf}/u;
    // validName: at least one rune that is neither white space (Go unicode.IsSpace) nor U+200D.
    const VISIBLE = /[^\p{White_Space}\u200d]/u;
    const HOST_LABEL = /^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$/;
    const HEX_LABEL = /^0x[0-9a-f]*$/;
    const SSH_USER = /^[a-z0-9._-]{1,32}$/;
    const HOST_KEY = /^SHA256:[A-Za-z0-9+/]{43}$/;
    // internal/security nonPublicURLPrefixes (IsGlobalUnicast adds nothing beyond these); IPv4-mapped IPv6 is unwrapped first.
    const RESTRICTED_V4 = ['0.0.0.0/8', '10.0.0.0/8', '100.64.0.0/10', '127.0.0.0/8', '169.254.0.0/16', '172.16.0.0/12',
        '192.0.0.0/24', '192.0.2.0/24', '192.168.0.0/16', '198.18.0.0/15', '198.51.100.0/24', '203.0.113.0/24',
        '224.0.0.0/4', '240.0.0.0/4'];
    const RESTRICTED_V6 = ['::/128', '::1/128', '::/96', '64:ff9b::/96', '64:ff9b:1::/48', '2002::/16', '2001::/32',
        'fc00::/7', 'fe80::/10', 'ff00::/8', '2001:db8::/32'];
    const ERROR_HOST = 'desktop.terminal_retronet_error_host';
    const ERROR_PRIVATE = 'desktop.terminal_retronet_error_private_host';
    let dialogSeq = 0;

    // Interpolates {{name}} itself: t() is called without params, so `$` in server text is never a replacement pattern.
    function translator(t) {
        const lookup = typeof t === 'function' ? t : function (key) { return key; };
        return function (key, params) {
            let text = String(lookup(key));
            Object.keys(params || {}).forEach(function (name) {
                text = text.split('{{' + name + '}}').join(String(params[name]));
            });
            return text;
        };
    }

    // own- + 12 unbiased [a-z0-9] characters (rejection sampling below 252 = 7 * 36), unique among `taken`.
    function newId(taken) {
        let id = '';
        while (!id || taken.indexOf(id) >= 0) {
            const chars = [];
            const bytes = new Uint8Array(24);
            while (chars.length < ID_LENGTH) {
                window.crypto.getRandomValues(bytes);
                for (let i = 0; i < bytes.length && chars.length < ID_LENGTH; i += 1) {
                    if (bytes[i] < 252) chars.push(ID_ALPHABET[bytes[i] % ID_ALPHABET.length]);
                }
            }
            id = 'own-' + chars.join('');
        }
        return id;
    }

    // Exactly the stored keys (id, name, description, protocol, host, port, kind, charset, user, host_key);
    // directory-only fields (own, category, description_key) are dropped, empty optional ones left out.
    function storable(entry) {
        const source = entry || {};
        const protocol = source.protocol === 'ssh' ? 'ssh' : 'telnet';
        const out = { id: String(source.id || ''), name: String(source.name || '') };
        if (source.description) out.description = String(source.description);
        out.protocol = protocol;
        out.host = String(source.host || '');
        out.port = Number(source.port) || 0;
        if (protocol === 'telnet') {
            out.kind = KINDS.indexOf(source.kind) >= 0 ? source.kind : 'bbs';
            out.charset = CHARSET_VALUES.indexOf(source.charset) >= 0 ? source.charset : 'utf8';
        } else {
            out.user = String(source.user || '');
            if (source.host_key) out.host_key = String(source.host_key);
        }
        return out;
    }

    // Own entries only (catalog entries are never stored).
    function ownList(entries) {
        return (Array.isArray(entries) ? entries : []).filter(function (item) {
            return item && OWN_ID.test(String(item.id || ''));
        }).map(storable);
    }

    // Dotted decimal as net.ParseIP reads it: four parts 0-255 without leading zeros.
    function parseIPv4(text) {
        const parts = String(text).split('.');
        if (parts.length !== 4) return null;
        const out = [];
        for (let i = 0; i < 4; i += 1) {
            if (!/^(0|[1-9][0-9]{0,2})$/.test(parts[i]) || Number(parts[i]) > 255) return null;
            out.push(Number(parts[i]));
        }
        return out;
    }

    // Eight 16-bit groups: "::" at most once, an optional dotted IPv4 tail, no zone or brackets.
    function parseIPv6(text) {
        let source = String(text);
        const lastColon = source.lastIndexOf(':');
        if (lastColon < 0) return null;
        if (source.indexOf('.', lastColon) > lastColon) {
            const v4 = parseIPv4(source.slice(lastColon + 1));
            if (!v4) return null;
            source = source.slice(0, lastColon + 1) + ((v4[0] << 8) | v4[1]).toString(16) + ':' + ((v4[2] << 8) | v4[3]).toString(16);
        }
        const halves = source.split('::');
        if (halves.length > 2) return null;
        const groups = function (part) { return part === '' ? [] : part.split(':'); };
        const left = groups(halves[0]);
        const right = halves.length === 2 ? groups(halves[1]) : [];
        const all = left.concat(right);
        if (!all.every(function (group) { return /^[0-9a-f]{1,4}$/.test(group); })) return null;
        if (halves.length === 2 ? all.length > 7 : all.length !== 8) return null;
        const fill = [];
        for (let i = all.length; i < 8; i += 1) fill.push(0);
        return left.concat(fill, right).map(function (group) { return typeof group === 'number' ? group : parseInt(group, 16); });
    }

    // parts are octets (bits 8) or 16-bit groups (bits 16); cidr is written like the Go prefix list.
    function inPrefix(parts, bits, cidr) {
        const slash = cidr.indexOf('/');
        const base = bits === 8 ? parseIPv4(cidr.slice(0, slash)) : parseIPv6(cidr.slice(0, slash));
        let left = Number(cidr.slice(slash + 1));
        for (let i = 0; left > 0; i += 1) {
            const take = Math.min(bits, left);
            const mask = ((1 << take) - 1) << (bits - take);
            if ((parts[i] & mask) !== (base[i] & mask)) return false;
            left -= take;
        }
        return true;
    }

    function restrictedV4(octets) {
        return RESTRICTED_V4.some(function (cidr) { return inPrefix(octets, 8, cidr); });
    }

    function restrictedV6(groups) {
        const mapped = groups.slice(0, 5).every(function (group) { return group === 0; }) && groups[5] === 0xffff;
        if (mapped) return restrictedV4([groups[6] >> 8, groups[6] & 0xff, groups[7] >> 8, groups[7] & 0xff]);
        return RESTRICTED_V6.some(function (cidr) { return inPrefix(groups, 16, cidr); });
    }

    // validHost: a public IP literal or an RFC 1123 name (labels of 1-63 [a-z0-9-] without an outer hyphen,
    // at most 253 bytes, a last label neither all digits nor 0x hex). localhost names can never be dialed.
    function hostError(host) {
        if (!host || host.length > 253) return ERROR_HOST;
        const v4 = parseIPv4(host);
        if (v4) return restrictedV4(v4) ? ERROR_PRIVATE : '';
        if (host.indexOf(':') >= 0) {
            const v6 = parseIPv6(host);
            if (!v6) return ERROR_HOST;
            return restrictedV6(v6) ? ERROR_PRIVATE : '';
        }
        const labels = host.split('.');
        if (!labels.every(function (label) { return HOST_LABEL.test(label); })) return ERROR_HOST;
        const last = labels[labels.length - 1];
        if (/^[0-9]+$/.test(last) || HEX_LABEL.test(last)) return ERROR_HOST;
        if (host === 'localhost' || /\.localhost$/.test(host)) return ERROR_PRIVATE;
        return '';
    }

    function validName(name) {
        const length = Array.from(name).length;
        return length >= 1 && length <= NAME_MAX && VISIBLE.test(name) && !INVISIBLE.test(name);
    }

    function validDescription(text) {
        return Array.from(text).length <= DESCRIPTION_MAX && !INVISIBLE.test(text);
    }

    // The draft carries only the stored keys of its protocol: Telnet kind + charset (never user or host key),
    // SSH user (+ the pinned host key while protocol, host, port and user are unchanged).
    function readDraft(fields, entry, own) {
        const value = function (name) { return String(fields[name].value || ''); };
        const protocol = value('protocol') === 'ssh' ? 'ssh' : 'telnet';
        const portText = value('port').trim();
        const draft = {
            id: entry ? entry.id : newId(own.map(function (item) { return item.id; })),
            name: value('name').trim()
        };
        const description = value('description').trim();
        if (description) draft.description = description;
        draft.protocol = protocol;
        draft.host = value('host').trim().toLowerCase();
        draft.port = /^[0-9]{1,5}$/.test(portText) ? Number(portText) : NaN;
        if (protocol === 'telnet') {
            draft.kind = KINDS.indexOf(value('kind')) >= 0 ? value('kind') : 'bbs';
            draft.charset = CHARSET_VALUES.indexOf(value('charset')) >= 0 ? value('charset') : 'utf8';
        } else {
            draft.user = value('user').trim();
        }
        return draft;
    }

    // A pinned key stays valid only while protocol, host, port and user equal the stored entry's.
    function keepHostKey(draft, entry) {
        const unchanged = entry && draft.protocol === 'ssh' && entry.protocol === 'ssh' && entry.host_key && HOST_KEY.test(entry.host_key) &&
            String(entry.host).toLowerCase() === draft.host &&
            Number(entry.port) === draft.port && entry.user === draft.user;
        if (unchanged) draft.host_key = entry.host_key;
        return draft;
    }

    function localizedError(key) {
        const err = new Error('');
        err.key = key;
        return err;
    }

    // The own entries as stored right now: other windows may have saved since this dialog opened, and the
    // server pins SSH host keys on first contact. A reply without an entry list saves nothing.
    function loadStored(api) {
        return Promise.resolve().then(function () {
            return api('/api/desktop/retronet/directory');
        }).then(function (payload) {
            if (!payload || !Array.isArray(payload.entries)) throw new Error('');
            return ownList(payload.entries);
        });
    }

    // Applies this dialog's change to the stored list: a new entry is appended, an edit replaces the entry
    // by ID (keeping the stored host key while the SSH target is unchanged), a delete removes it by ID.
    function mergeChange(stored, change) {
        const ids = stored.map(function (item) { return item.id; });
        const index = change.id ? ids.indexOf(change.id) : -1;
        if (change.id && index < 0) throw localizedError('desktop.terminal_retronet_error_gone');
        const next = stored.slice();
        if (change.remove) {
            next.splice(index, 1);
            return { entries: next, saved: null };
        }
        const draft = Object.assign({}, change.draft);
        delete draft.host_key;
        if (index >= 0) {
            next[index] = keepHostKey(draft, stored[index]);
            return { entries: next, saved: next[index] };
        }
        if (ids.indexOf(draft.id) >= 0) draft.id = newId(ids);
        if (next.length >= MAX_ENTRIES) throw localizedError('desktop.terminal_retronet_error_limit');
        next.push(draft);
        return { entries: next, saved: draft };
    }

    function validate(draft) {
        if (!validName(draft.name)) return { key: 'desktop.terminal_retronet_error_name', field: 'name' };
        if (!validDescription(draft.description || '')) return { key: 'desktop.terminal_retronet_error_description', field: 'description' };
        const hostProblem = hostError(draft.host);
        if (hostProblem) return { key: hostProblem, field: 'host' };
        if (!Number.isInteger(draft.port) || draft.port < 1 || draft.port > 65535) {
            return { key: 'desktop.terminal_retronet_error_port', field: 'port' };
        }
        if (BLOCKED_PORTS.indexOf(draft.port) >= 0) return { key: 'desktop.terminal_retronet_error_mail_port', field: 'port' };
        if (draft.protocol === 'ssh' && !SSH_USER.test(draft.user || '')) {
            return { key: 'desktop.terminal_retronet_error_user', field: 'user' };
        }
        return null;
    }

    function saveEntries(api, entries) {
        const value = JSON.stringify({ version: 1, entries: entries });
        if (new TextEncoder().encode(value).length > MAX_BYTES) return Promise.reject(localizedError('desktop.terminal_retronet_error_limit'));
        return Promise.resolve().then(function () {
            return api('/api/desktop/settings', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ key: SETTING_KEY, value: value })
            });
        });
    }

    // Re-reads the stored entries, applies the change and stores the result; resolves { entries, saved }.
    function commit(api, change) {
        return loadStored(api).then(function (stored) {
            const result = mergeChange(stored, change);
            return saveEntries(api, result.entries).then(function () { return result; });
        });
    }

    // The server's error text (ctx.api throws body.error), else the generic message.
    function errorText(err, tr) {
        return err && err.message ? String(err.message) : tr('desktop.request_failed');
    }

    // Element with attributes; text is only ever set through textContent.
    function el(tag, attrs, text) {
        const node = document.createElement(tag);
        Object.keys(attrs || {}).forEach(function (name) { node.setAttribute(name, attrs[name]); });
        if (text != null) node.textContent = String(text);
        return node;
    }

    function append(parent, children) {
        children.forEach(function (child) { if (child) parent.appendChild(child); });
        return parent;
    }

    function textInput(name, value, attrs) {
        const input = el('input', Object.assign({ name: name, autocomplete: 'off', spellcheck: 'false' }, attrs || {}));
        input.value = value == null ? '' : String(value);
        return input;
    }

    function selectInput(name, options, value) {
        const select = el('select', { name: name });
        options.forEach(function (option) { select.appendChild(el('option', { value: option[0] }, option[1])); });
        select.value = value;
        return select;
    }

    // A labelled control; `only` shows it for one protocol.
    function fieldRow(name, label, control, only) {
        const row = el('label', { 'data-retronet-field': name });
        if (only) row.setAttribute('data-retronet-only', only);
        return append(row, [el('span', null, label), control]);
    }

    // Dialog skeleton: header (title + close), caller content, error line, footer (Cancel + action).
    function buildDialog(host, kind, title, content, action, tr) {
        const id = 'vd-retronet-dialog-' + (++dialogSeq);
        const dialog = document.createElement('dialog');
        dialog.className = 'vd-terminal-retronet-dialog';
        dialog.setAttribute('data-terminal-retronet-dialog', kind);
        dialog.setAttribute('aria-labelledby', id + '-title');
        const close = el('button', { type: 'button', 'data-retronet-close': '', 'aria-label': tr('desktop.close') }, '\u00d7');
        const cancel = el('button', { type: 'button', 'data-retronet-cancel': '' }, tr('desktop.cancel'));
        const error = el('p', { id: id + '-error', class: 'vd-terminal-retronet-error', role: 'alert', 'data-retronet-error': '' });
        error.hidden = true;
        const form = el('form', { novalidate: '' });
        append(form, [append(el('header'), [el('h2', { id: id + '-title' }, title), close])]);
        append(form, content);
        append(form, [error, append(el('footer'), [cancel, action])]);
        dialog.appendChild(form);
        host.appendChild(dialog);
        return { dialog: dialog, form: form, error: error, close: close, cancel: cancel, action: action };
    }

    function showError(parts, text, field) {
        parts.error.textContent = text;
        parts.error.hidden = false;
        if (field) {
            field.setAttribute('aria-invalid', 'true');
            field.setAttribute('aria-describedby', parts.error.getAttribute('id'));
            field.focus();
        }
    }

    function clearError(parts, fields) {
        parts.error.hidden = true;
        parts.error.textContent = '';
        Object.keys(fields || {}).forEach(function (name) {
            fields[name].removeAttribute('aria-invalid');
            fields[name].removeAttribute('aria-describedby');
        });
    }

    // Modal presentation; Escape, Cancel and the close button close it unless a save is pending. Focus returns to the opener.
    function present(parts, state, focusTarget) {
        const dialog = parts.dialog;
        const previous = document.activeElement;
        function dismiss() {
            if (state.busy) return;
            if (dialog.open) dialog.close();
            else dialog.remove();
        }
        parts.close.addEventListener('click', dismiss);
        parts.cancel.addEventListener('click', dismiss);
        dialog.addEventListener('cancel', function (event) {
            if (state.busy) event.preventDefault();
        });
        dialog.addEventListener('close', function () {
            dialog.remove();
            if (previous && previous.isConnected && typeof previous.focus === 'function') previous.focus();
        });
        dialog.showModal();
        if (focusTarget && typeof focusTarget.focus === 'function') focusTarget.focus();
    }

    // Create (entry null) or edit an own entry. `esc` (Contract E) is accepted but unused: no markup is built from text.
    function open(options) {
        const opts = options || {};
        const host = opts.host;
        if (!host || typeof opts.api !== 'function') return null;
        const tr = translator(opts.t);
        const entry = opts.entry ? storable(opts.entry) : null;
        const own = ownList(opts.ownEntries);
        const onSaved = typeof opts.onSaved === 'function' ? opts.onSaved : function () {};
        const kind = entry && entry.kind === 'world' ? 'world' : 'bbs';
        const charset = entry && entry.charset ? entry.charset : (kind === 'bbs' ? 'cp437' : 'utf8');
        const fields = {
            name: textInput('name', entry ? entry.name : '', { required: '' }),
            description: textInput('description', entry && entry.description ? entry.description : ''),
            protocol: selectInput('protocol', [['telnet', 'Telnet'], ['ssh', 'SSH']], entry ? entry.protocol : 'telnet'),
            kind: selectInput('kind', [['bbs', tr('desktop.terminal_retronet_type_bbs')], ['world', tr('desktop.terminal_retronet_type_world')]], kind),
            charset: selectInput('charset', CHARSETS, charset),
            host: textInput('host', entry ? entry.host : '', { maxlength: '253', inputmode: 'url', autocapitalize: 'none' }),
            port: textInput('port', entry ? String(entry.port) : '23', { type: 'number', min: '1', max: '65535', step: '1', inputmode: 'numeric' }),
            user: textInput('user', entry && entry.user ? entry.user : '', { maxlength: '32', autocapitalize: 'none' })
        };
        const grid = append(el('div', { class: 'vd-terminal-retronet-fields' }), [
            fieldRow('name', tr('desktop.terminal_retronet_field_name'), fields.name),
            fieldRow('description', tr('desktop.terminal_retronet_field_description'), fields.description),
            fieldRow('protocol', tr('desktop.terminal_retronet_field_protocol'), fields.protocol),
            fieldRow('kind', tr('desktop.terminal_retronet_field_type'), fields.kind, 'telnet'),
            fieldRow('charset', tr('desktop.terminal_retronet_field_charset'), fields.charset, 'telnet'),
            fieldRow('host', tr('desktop.terminal_retronet_field_host'), fields.host),
            fieldRow('port', tr('desktop.terminal_retronet_field_port'), fields.port),
            fieldRow('user', tr('desktop.terminal_retronet_field_user'), fields.user, 'ssh')
        ]);
        const save = el('button', { type: 'submit', class: 'vd-terminal-retronet-primary', 'data-retronet-save': '' }, tr('desktop.save'));
        const hint = el('p', { class: 'vd-terminal-retronet-hint' }, tr('desktop.terminal_retronet_form_hint'));
        const title = tr(entry ? 'desktop.terminal_retronet_edit_entry' : 'desktop.terminal_retronet_new_entry');
        const parts = buildDialog(host, entry ? 'edit' : 'new', title, [hint, grid], save, tr);
        const state = { busy: false };
        let charsetTouched = !!entry;

        function syncProtocol() {
            const shown = fields.protocol.value === 'ssh' ? 'ssh' : 'telnet';
            Array.prototype.forEach.call(grid.children, function (row) {
                const only = row.getAttribute('data-retronet-only');
                row.hidden = !!only && only !== shown;
            });
        }

        fields.protocol.addEventListener('change', function () {
            const ssh = fields.protocol.value === 'ssh';
            if (!fields.port.value || fields.port.value === (ssh ? '23' : '22')) fields.port.value = ssh ? '22' : '23';
            syncProtocol();
        });
        fields.kind.addEventListener('change', function () {
            if (!charsetTouched) fields.charset.value = fields.kind.value === 'bbs' ? 'cp437' : 'utf8';
        });
        fields.charset.addEventListener('change', function () { charsetTouched = true; });
        parts.form.addEventListener('submit', function (event) {
            event.preventDefault();
            if (state.busy) return;
            clearError(parts, fields);
            const draft = readDraft(fields, entry, own);
            const problem = validate(draft);
            if (problem) {
                showError(parts, tr(problem.key), fields[problem.field]);
                return;
            }
            if (!entry && own.length >= MAX_ENTRIES) {
                showError(parts, tr('desktop.terminal_retronet_error_limit'));
                return;
            }
            state.busy = true;
            save.disabled = true;
            commit(opts.api, { id: entry ? entry.id : '', draft: draft }).then(function (result) {
                state.busy = false;
                if (parts.dialog.open) parts.dialog.close();
                onSaved(result.entries, result.saved);
            }, function (err) {
                state.busy = false;
                save.disabled = false;
                if (err && err.key) showError(parts, tr(err.key));
                else showError(parts, tr('desktop.terminal_retronet_error_save', { message: errorText(err, tr) }));
            });
        });
        syncProtocol();
        present(parts, state, fields.name);
        return parts.dialog;
    }

    function confirmDelete(options) {
        const opts = options || {};
        const host = opts.host;
        if (!host || !opts.entry || typeof opts.api !== 'function') return null;
        const tr = translator(opts.t);
        const entry = storable(opts.entry);
        const own = ownList(opts.ownEntries);
        const onSaved = typeof opts.onSaved === 'function' ? opts.onSaved : function () {};
        const question = el('p', null, tr('desktop.terminal_retronet_delete_confirm', { name: entry.name }));
        const remove = el('button', { type: 'submit', class: 'vd-terminal-retronet-danger', 'data-retronet-delete': '' }, tr('desktop.delete'));
        const parts = buildDialog(host, 'delete', tr('desktop.terminal_retronet_delete_title'), [question], remove, tr);
        const state = { busy: false };
        parts.form.addEventListener('submit', function (event) {
            event.preventDefault();
            if (state.busy) return;
            state.busy = true;
            remove.disabled = true;
            clearError(parts);
            commit(opts.api, { id: entry.id, remove: true }).then(function (result) {
                state.busy = false;
                if (parts.dialog.open) parts.dialog.close();
                onSaved(result.entries, null);
            }, function (err) {
                state.busy = false;
                remove.disabled = false;
                if (err && err.key) showError(parts, tr(err.key));
                else showError(parts, tr('desktop.terminal_retronet_error_delete', { message: errorText(err, tr) }));
            });
        });
        present(parts, state, parts.cancel);
        return parts.dialog;
    }

    window.TerminalRetroNetEntries = { open: open, confirmDelete: confirmDelete };
})();
