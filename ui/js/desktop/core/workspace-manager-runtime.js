(function () {
    'use strict';

    const MAX_WORKSPACES = 8;
    const MAX_NAME_LENGTH = 64;
    const MAX_SAFE_ID = Number.MAX_SAFE_INTEGER;
    const DEFAULT_LAYOUT = Object.freeze({
        version: 1,
        revision: 1,
        next_id: 4,
        workspaces: Object.freeze([{ id: '1', name: '' }, { id: '2', name: '' }, { id: '3', name: '' }]),
        removed_to: Object.freeze({})
    });
    const DEFAULT_SETTINGS = Object.freeze({
        'session.windows': '',
        'appearance.wallpaper_by_space': '{}',
        'appearance.wallpaper': 'groupshoot'
    });

    const listeners = new Set();
    let config = {
        api: null,
        readonly: () => false,
        t: key => key,
        mountDialog: null,
        onChange: () => {},
        onConflict: () => {}
    };
    let current = { layout: cloneLayout(DEFAULT_LAYOUT), settings: { ...DEFAULT_SETTINGS } };
    let mutationPromise = null;
    let dialog = null;
    let dialogUnmount = null;
    let launcherFocus = null;
    let announcement = '';

    function cloneLayout(layout) {
        return {
            version: layout.version,
            revision: layout.revision,
            next_id: layout.next_id,
            workspaces: layout.workspaces.map(workspace => ({ id: workspace.id, name: workspace.name })),
            removed_to: { ...layout.removed_to }
        };
    }

    function cloneSnapshot(snapshot) {
        return { layout: cloneLayout(snapshot.layout), settings: { ...snapshot.settings } };
    }

    function normalizeSnapshot(value) {
        if (!value || typeof value !== 'object') throw new TypeError('Workspace state must be an object');
        const layout = value.layout;
        if (!layout || layout.version !== 1 || !Number.isSafeInteger(layout.revision) || layout.revision < 1 ||
            !Number.isSafeInteger(layout.next_id) || layout.next_id < 1 || layout.next_id > MAX_SAFE_ID ||
            !Array.isArray(layout.workspaces) || layout.workspaces.length < 1 || layout.workspaces.length > MAX_WORKSPACES) {
            throw new TypeError('Workspace layout is invalid');
        }
        const seen = new Set();
        const active = new Set();
        let maxID = 0;
        const workspaces = layout.workspaces.map(item => {
            if (!item || typeof item.id !== 'string' || !/^[1-9]\d*$/.test(item.id)) throw new TypeError('Workspace id is invalid');
            const numeric = Number(item.id);
            if (!Number.isSafeInteger(numeric) || String(numeric) !== item.id || seen.has(item.id)) throw new TypeError('Workspace id is invalid');
            if (typeof item.name !== 'string' || item.name !== item.name.trim() || Array.from(item.name).length > MAX_NAME_LENGTH || /[\u0000-\u001f\u007f]/u.test(item.name)) {
                throw new TypeError('Workspace name is invalid');
            }
            seen.add(item.id);
            active.add(item.id);
            maxID = Math.max(maxID, numeric);
            return { id: item.id, name: item.name };
        });
        const removed = layout.removed_to == null ? {} : layout.removed_to;
        if (!removed || typeof removed !== 'object' || Array.isArray(removed)) throw new TypeError('Workspace redirects are invalid');
        const removedTo = {};
        for (const [removedID, target] of Object.entries(removed)) {
            if (!/^[1-9]\d*$/.test(removedID) || !Number.isSafeInteger(Number(removedID)) || String(Number(removedID)) !== removedID ||
                seen.has(removedID) || typeof target !== 'string' || !active.has(target)) throw new TypeError('Workspace redirect is invalid');
            maxID = Math.max(maxID, Number(removedID));
            removedTo[removedID] = target;
        }
        if (layout.next_id <= maxID) throw new TypeError('Workspace next id is invalid');
        const settings = {};
        const sourceSettings = value.settings && typeof value.settings === 'object' ? value.settings : {};
        for (const key of Object.keys(DEFAULT_SETTINGS)) {
            const setting = sourceSettings[key];
            settings[key] = typeof setting === 'string' ? setting : DEFAULT_SETTINGS[key];
        }
        return { layout: { version: 1, revision: layout.revision, next_id: layout.next_id, workspaces, removed_to: removedTo }, settings };
    }

    function configure(options = {}) {
        config = {
            api: typeof options.api === 'function' ? options.api : config.api,
            readonly: typeof options.readonly === 'function' ? options.readonly : config.readonly,
            t: typeof options.t === 'function' ? options.t : config.t,
            mountDialog: typeof options.mountDialog === 'function' ? options.mountDialog : config.mountDialog,
            onChange: typeof options.onChange === 'function' ? options.onChange : config.onChange,
            onConflict: typeof options.onConflict === 'function' ? options.onConflict : config.onConflict
        };
        return api;
    }

    function snapshot() {
        return cloneSnapshot(current);
    }

    function adopt(value, reason = 'adopt') {
        const incoming = normalizeSnapshot(value);
        if (incoming.layout.revision < current.layout.revision) return false;
        current = incoming;
        notify(reason);
        return true;
    }

    function subscribe(listener) {
        if (typeof listener !== 'function') throw new TypeError('Workspace subscriber must be a function');
        listeners.add(listener);
        return () => listeners.delete(listener);
    }

    function notify(reason) {
        const value = snapshot();
        try { config.onChange(value, reason); } catch (_) { /* UI reconciliation cannot invalidate an accepted server snapshot. */ }
        for (const listener of [...listeners]) {
            try { listener(snapshot(), reason); } catch (_) { /* One subscriber must not block the others. */ }
        }
        if (dialog && !dialog.hidden) renderDialog({ preserve: true });
    }

    function workspace(id) {
        const key = String(id || '');
        const found = current.layout.workspaces.find(item => item.id === key);
        return found ? { ...found } : null;
    }

    function resolveId(id) {
        let key = String(id || current.layout.workspaces[0]?.id || '1');
        const visited = new Set();
        while (!current.layout.workspaces.some(item => item.id === key)) {
            if (visited.has(key) || !Object.hasOwn(current.layout.removed_to, key)) return current.layout.workspaces[0]?.id || '1';
            visited.add(key);
            key = current.layout.removed_to[key];
        }
        return key;
    }

    function name(id) {
        const record = workspace(resolveId(id));
        return record?.name || String(config.t('desktop.space_n')).replace('{{n}}', record?.id || String(id));
    }

    function isReadonly() {
        try { return config.readonly() === true; } catch (_) { return true; }
    }

    async function load() {
        if (typeof config.api !== 'function') throw new Error('Workspace API is not configured');
        const result = await config.api('/api/desktop/workspaces', { method: 'GET' });
        const state = result && result.layout ? result : result?.state;
        if (!adopt(state, 'load')) throw new Error('Workspace state is older than the current state');
        return snapshot();
    }

    function add(nameValue = '') { return mutate({ operation: 'add', name: String(nameValue || '') }); }
    function rename(id, nameValue) { return mutate({ operation: 'rename', id: String(id || ''), name: String(nameValue || '') }); }
    function reorder(order) { return mutate({ operation: 'reorder', order: Array.isArray(order) ? order.map(String) : [] }); }
    function remove(id, moveTo) { return mutate({ operation: 'delete', id: String(id || ''), move_to: String(moveTo || '') }); }

    function mutate(command) {
        if (isReadonly()) return Promise.reject(new Error('desktop_readonly'));
        if (mutationPromise) return Promise.reject(new Error('workspace_mutation_busy'));
        if (typeof config.api !== 'function') return Promise.reject(new Error('workspace_api_unavailable'));
        const body = { ...command, revision: current.layout.revision };
        const pending = config.api('/api/desktop/workspaces', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body)
        }).then(result => {
            const state = result && result.layout ? result : result?.state;
            if (!adopt(state, 'mutation')) throw new Error('Workspace response is stale');
            return snapshot();
        }).catch(error => {
            if ((error?.body?.code === 'workspace_revision_conflict' || error?.body?.code === 'workspace_revision_required') && error.body.state) {
                adopt(error.body.state, 'conflict');
                try { config.onConflict(snapshot(), error.body); } catch (_) { /* Keep the original conflict visible to the caller. */ }
            }
            throw error;
        }).finally(() => {
            if (mutationPromise === pending) mutationPromise = null;
            if (dialog && !dialog.hidden) renderDialog({ preserve: true });
        });
        mutationPromise = pending;
        if (dialog && !dialog.hidden) renderDialog({ preserve: true });
        return pending;
    }

    function escapeHTML(value) {
        return String(value).replace(/[&<>"']/g, character => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[character]);
    }

    function text(key, variables) {
        const translated = String(config.t(key, variables));
        return translated === key ? key : translated;
    }

    function openManager() {
        if (!document.body) return false;
        if (dialog && !dialog.hidden) return true;
        launcherFocus = document.activeElement;
        if (!dialog) {
            dialog = document.createElement('div');
            dialog.id = 'vd-workspace-manager';
            dialog.className = 'vd-workspace-manager';
            dialog.hidden = true;
            dialog.addEventListener('click', event => {
                if (event.target === dialog || event.target.closest('[data-workspace-dismiss]')) closeManager();
            });
            dialog.addEventListener('keydown', event => {
                if (event.key === 'Escape') {
                    event.preventDefault();
                    event.stopPropagation();
                    closeManager();
                }
                if (event.key === 'Tab') trapDialogFocus(event);
                if (event.key === 'Tab') event.stopPropagation();
            });
            if (config.mountDialog) dialogUnmount = config.mountDialog(dialog);
            else document.body.appendChild(dialog);
        }
        dialog.hidden = false;
        renderDialog({ preserve: false });
        dialog.querySelector('button:not(:disabled),input:not(:disabled),select:not(:disabled)')?.focus();
        return true;
    }

    function closeManager() {
        if (!dialog) return;
        dialogUnmount?.();
        dialogUnmount = null;
        dialog.remove();
        dialog = null;
        const focus = launcherFocus;
        launcherFocus = null;
        if (focus?.isConnected) focus.focus();
    }

    function trapDialogFocus(event) {
        const focusable = [...dialog.querySelectorAll('button:not(:disabled),input:not(:disabled),select:not(:disabled),[href],[tabindex]:not([tabindex="-1"])')]
            .filter(element => !element.closest('[hidden]') && element.getAttribute('aria-hidden') !== 'true' &&
                (typeof element.getClientRects !== 'function' || element.getClientRects().length > 0));
        if (!focusable.length) return;
        const first = focusable[0];
        const last = focusable[focusable.length - 1];
        if (event.shiftKey && document.activeElement === first) {
            event.preventDefault();
            last.focus();
        } else if (!event.shiftKey && document.activeElement === last) {
            event.preventDefault();
            first.focus();
        }
    }

    function captureDialogState() {
        if (!dialog) return { values: {}, targets: {}, confirmations: new Set(), focus: null };
        const values = {};
        dialog.querySelectorAll('[data-workspace-rename]').forEach(input => { values[input.dataset.workspaceRename] = input.value; });
        const addInput = dialog.querySelector('[data-workspace-add-name]');
        if (addInput) values.$add = addInput.value;
        const targets = {};
        dialog.querySelectorAll('[data-workspace-target]').forEach(select => { targets[select.dataset.workspaceTarget] = select.value; });
        const confirmations = new Set([...dialog.querySelectorAll('[data-workspace-confirm]:not([hidden])')].map(node => node.dataset.workspaceConfirm));
        return { values, targets, confirmations, focus: focusDescriptor(document.activeElement) };
    }

    function focusDescriptor(element) {
        if (!element || !dialog?.contains(element)) return null;
        const attributes = [
            'data-workspace-add-name', 'data-workspace-rename', 'data-workspace-save',
            'data-workspace-up', 'data-workspace-down', 'data-workspace-delete',
            'data-workspace-confirm-delete', 'data-workspace-target', 'data-workspace-cancel-delete', 'data-workspace-dismiss'
        ];
        const attribute = attributes.find(name => element.hasAttribute(name));
        const row = element.closest('[data-workspace-row]');
        return {
            attribute: attribute || '',
            value: attribute ? element.getAttribute(attribute) : '',
            rowIndex: row ? [...dialog.querySelectorAll('[data-workspace-row]')].indexOf(row) : -1,
            selectionStart: typeof element.selectionStart === 'number' ? element.selectionStart : null,
            selectionEnd: typeof element.selectionEnd === 'number' ? element.selectionEnd : null
        };
    }

    function elementByData(attribute, value) {
        if (!dialog || !attribute) return null;
        return [...dialog.querySelectorAll(`[${attribute}]`)].find(element => element.getAttribute(attribute) === value) || null;
    }

    function restoreDialogFocus(descriptor) {
        if (!dialog || !descriptor) return;
        let element = descriptor.attribute ? elementByData(descriptor.attribute, descriptor.value) : null;
        if (!element && descriptor.rowIndex >= 0) {
            const rows = dialog.querySelectorAll('[data-workspace-row]');
            const row = rows[Math.min(descriptor.rowIndex, rows.length - 1)];
            element = row?.querySelector('input:not(:disabled),button:not(:disabled),select:not(:disabled)') || null;
        }
        if (!element) element = dialog.querySelector('button:not(:disabled),input:not(:disabled),select:not(:disabled)');
        element?.focus();
        if (element && descriptor.selectionStart !== null && typeof element.setSelectionRange === 'function') {
            try { element.setSelectionRange(descriptor.selectionStart, descriptor.selectionEnd); } catch (_) { /* Some input types do not expose a selection range. */ }
        }
    }

    function renderDialog(options = {}) {
        if (!dialog || dialog.hidden) return;
        const previous = options.preserve === false ? null : captureDialogState();
        const readonly = isReadonly();
        const busy = Boolean(mutationPromise);
        const records = current.layout.workspaces;
        const rows = records.map((item, index) => {
            const first = index === 0;
            const last = index === records.length - 1;
            const others = records.filter(other => other.id !== item.id);
            const displayName = item.name || name(item.id);
            const savedTarget = previous?.targets[item.id] || '';
            const targetSelect = `<option value="">${escapeHTML(text('desktop.workspaces_delete_select_target'))}</option>` +
                others.map(other => `<option value="${escapeHTML(other.id)}"${savedTarget === other.id ? ' selected' : ''}>${escapeHTML(name(other.id))}</option>`).join('');
            const deleteMessage = savedTarget
                ? text('desktop.workspaces_delete_confirm', { name: displayName, target: name(savedTarget) })
                : text('desktop.workspaces_delete_select_target');
            return `<li class="vd-workspace-manager-row" data-workspace-row="${escapeHTML(item.id)}">
                <label class="vd-workspace-manager-name"><span>${escapeHTML(text('desktop.workspaces_name_label'))}</span><input type="text" aria-label="${escapeHTML(text('desktop.workspaces_name_label') + ': ' + displayName)}" maxlength="${MAX_NAME_LENGTH}" data-workspace-rename="${escapeHTML(item.id)}" value="${escapeHTML(previous?.values[item.id] ?? item.name)}" ${readonly || busy ? 'disabled' : ''}></label>
                <div class="vd-workspace-manager-actions">
                    <button type="button" data-workspace-save="${escapeHTML(item.id)}" aria-label="${escapeHTML(text('desktop.rename') + ': ' + displayName)}" ${readonly || busy ? 'disabled' : ''}>${escapeHTML(text('desktop.rename'))}</button>
                    <button type="button" data-workspace-up="${escapeHTML(item.id)}" aria-label="${escapeHTML(text('desktop.workspaces_move_up') + ': ' + displayName)}" ${readonly || busy || first ? 'disabled' : ''}>↑</button>
                    <button type="button" data-workspace-down="${escapeHTML(item.id)}" aria-label="${escapeHTML(text('desktop.workspaces_move_down') + ': ' + displayName)}" ${readonly || busy || last ? 'disabled' : ''}>↓</button>
                    ${records.length > 1 ? `<button type="button" class="vd-workspace-manager-delete" data-workspace-delete="${escapeHTML(item.id)}" aria-label="${escapeHTML(text('desktop.delete') + ': ' + displayName)}" ${readonly || busy ? 'disabled' : ''}>${escapeHTML(text('desktop.delete'))}</button>` : ''}
                </div>
                <div class="vd-workspace-manager-delete-confirm" data-workspace-confirm="${escapeHTML(item.id)}"${previous?.confirmations.has(item.id) ? '' : ' hidden'}>
                    <p data-workspace-delete-message="${escapeHTML(item.id)}">${escapeHTML(deleteMessage)}</p>
                    <label><span>${escapeHTML(text('desktop.workspaces_move_windows_to'))}</span><select data-workspace-target="${escapeHTML(item.id)}" ${readonly || busy ? 'disabled' : ''}>${targetSelect}</select></label>
                    <button type="button" data-workspace-confirm-delete="${escapeHTML(item.id)}" aria-label="${escapeHTML(text('desktop.delete') + ': ' + displayName)}" ${readonly || busy ? 'disabled' : ''}>${escapeHTML(text('desktop.delete'))}</button>
                    <button type="button" data-workspace-cancel-delete="${escapeHTML(item.id)}" ${busy ? 'disabled' : ''}>${escapeHTML(text('desktop.cancel'))}</button>
                </div>
            </li>`;
        }).join('');
        const canAdd = records.length < MAX_WORKSPACES;
        dialog.innerHTML = `<div class="vd-workspace-manager-backdrop" data-workspace-dismiss></div>
            <section class="vd-workspace-manager-panel" role="dialog" aria-modal="true" aria-labelledby="vd-workspace-manager-title" aria-describedby="vd-workspace-manager-description">
                <header class="vd-workspace-manager-header"><div><h2 id="vd-workspace-manager-title">${escapeHTML(text('desktop.workspaces_manage'))}</h2><p id="vd-workspace-manager-description">${escapeHTML(text('desktop.workspaces_description'))}</p></div><button type="button" data-workspace-dismiss aria-label="${escapeHTML(text('desktop.close'))}">×</button></header>
                ${readonly ? `<p class="vd-workspace-manager-readonly" role="status">${escapeHTML(text('desktop.file_dialog_readonly'))}</p>` : ''}
                <ol class="vd-workspace-manager-list">${rows}</ol>
                <form class="vd-workspace-manager-add"><label><span>${escapeHTML(text('desktop.workspaces_name_label'))}</span><input type="text" maxlength="${MAX_NAME_LENGTH}" data-workspace-add-name value="${escapeHTML(previous?.values.$add || '')}" ${readonly || busy || !canAdd ? 'disabled' : ''}></label><button type="submit" ${readonly || busy || !canAdd ? 'disabled' : ''}>${escapeHTML(text('desktop.workspaces_add'))}</button></form>
                ${records.length <= 1 ? `<p class="vd-workspace-manager-limit" role="status">${escapeHTML(text('desktop.workspaces_minimum'))}</p>` : ''}
                ${records.length >= MAX_WORKSPACES ? `<p class="vd-workspace-manager-limit" role="status">${escapeHTML(text('desktop.workspaces_maximum'))}</p>` : ''}
                <p class="vd-workspace-manager-status" role="status" aria-live="polite">${escapeHTML(announcement || (busy ? text('desktop.saving') : ''))}</p>
                <footer><button type="button" data-workspace-dismiss>${escapeHTML(text('desktop.close'))}</button></footer>
            </section>`;
        wireDialogEvents();
        restoreDialogFocus(previous?.focus || null);
    }

    function wireDialogEvents() {
        dialog.querySelector('.vd-workspace-manager-add')?.addEventListener('submit', event => {
            event.preventDefault();
            const input = dialog.querySelector('[data-workspace-add-name]');
            const value = input?.value.trim() || '';
            if (current.layout.workspaces.length >= MAX_WORKSPACES) {
                announcement = text('desktop.workspaces_maximum');
                dialog.querySelector('.vd-workspace-manager-status').textContent = announcement;
                return;
            }
            runUIAction(() => add(value), input, { clearAdd: true });
        });
        dialog.querySelectorAll('[data-workspace-save]').forEach(button => button.addEventListener('click', () => {
            const id = button.dataset.workspaceSave;
            const input = elementByData('data-workspace-rename', id);
            if (!input) return;
            runUIAction(() => rename(id, input.value), button, { committedNameID: id });
        }));
        dialog.querySelectorAll('[data-workspace-up],[data-workspace-down]').forEach(button => button.addEventListener('click', () => {
            const order = current.layout.workspaces.map(item => item.id);
            const index = order.indexOf(button.dataset.workspaceUp || button.dataset.workspaceDown);
            const delta = button.hasAttribute('data-workspace-up') ? -1 : 1;
            if (index < 0 || index + delta < 0 || index + delta >= order.length) return;
            [order[index], order[index + delta]] = [order[index + delta], order[index]];
            runUIAction(() => reorder(order), button);
        }));
        dialog.querySelectorAll('[data-workspace-delete]').forEach(button => button.addEventListener('click', () => {
            const confirm = elementByData('data-workspace-confirm', button.dataset.workspaceDelete);
            if (confirm) confirm.hidden = false;
            confirm?.querySelector('select')?.focus();
        }));
        dialog.querySelectorAll('[data-workspace-cancel-delete]').forEach(button => button.addEventListener('click', () => {
            const confirm = elementByData('data-workspace-confirm', button.dataset.workspaceCancelDelete);
            if (confirm) confirm.hidden = true;
        }));
        dialog.querySelectorAll('[data-workspace-confirm-delete]').forEach(button => button.addEventListener('click', () => {
            const id = button.dataset.workspaceConfirmDelete;
            const target = elementByData('data-workspace-target', id)?.value;
            if (!target || target === id) {
                announcement = text('desktop.workspaces_invalid');
                dialog.querySelector('.vd-workspace-manager-status').textContent = announcement;
                return;
            }
            runUIAction(() => remove(id, target), button);
        }));
        dialog.querySelectorAll('[data-workspace-target]').forEach(select => select.addEventListener('change', () => {
            const paragraph = elementByData('data-workspace-delete-message', select.dataset.workspaceTarget);
            const item = workspace(select.dataset.workspaceTarget);
            if (paragraph) paragraph.textContent = select.value
                ? text('desktop.workspaces_delete_confirm', { name: item?.name || name(select.dataset.workspaceTarget), target: name(select.value) })
                : text('desktop.workspaces_delete_select_target');
        }));
    }

    async function runUIAction(action, focusTarget, options = {}) {
        const focus = focusDescriptor(focusTarget || document.activeElement);
        announcement = '';
        try {
            await action();
            announcement = text('desktop.workspaces_saved');
            if (options.clearAdd) {
                const input = dialog?.querySelector('[data-workspace-add-name]');
                if (input) input.value = '';
            }
            if (options.committedNameID) {
                const input = elementByData('data-workspace-rename', options.committedNameID);
                const saved = workspace(options.committedNameID);
                if (input && saved) input.value = saved.name;
            }
        } catch (error) {
            const code = error?.body?.code || error?.message || '';
            if (code === 'workspace_limit') announcement = text('desktop.workspaces_limit');
            else if (code === 'workspace_not_found' || code === 'workspace_invalid') announcement = text('desktop.workspaces_invalid');
            else if (code === 'desktop_readonly') announcement = text('desktop.file_dialog_readonly');
            else if (code === 'workspace_revision_conflict' || code === 'workspace_revision_required') announcement = text('desktop.workspaces_conflict');
            else announcement = text('desktop.workspaces_invalid');
        }
        renderDialog({ preserve: true });
        restoreDialogFocus(focus);
    }

    const api = {
        configure, snapshot, adopt, subscribe, workspace, name, resolveId, isReadonly,
        load, add, rename, reorder, remove, open: openManager, close: closeManager, openManager, closeManager
    };
    window.AuraDesktopWorkspaceManager = api;
})();
