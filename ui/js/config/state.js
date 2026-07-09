// Config draft/session state for the Precision Workspace.
(function () {
    'use strict';

    let savedConfig = {};
    let draftConfig = {};
    let activeSection = '';
    let rules = {};
    let boundRoot = null;
    const changed = new Set();
    const listeners = new Set();

    function clone(value) {
        if (typeof structuredClone === 'function') return structuredClone(value);
        return JSON.parse(JSON.stringify(value == null ? {} : value));
    }

    function parts(path) {
        return String(path || '').split('.').filter(Boolean);
    }

    function read(object, path) {
        return parts(path).reduce((current, part) => {
            if (current == null) return undefined;
            return current[part];
        }, object);
    }

    function write(object, path, value) {
        const keys = parts(path);
        if (!keys.length) return;
        let current = object;
        keys.slice(0, -1).forEach((key, index) => {
            const nextKey = keys[index + 1];
            if (current[key] == null || typeof current[key] !== 'object') {
                current[key] = /^\d+$/.test(nextKey) ? [] : {};
            }
            current = current[key];
        });
        current[keys[keys.length - 1]] = clone(value);
    }

    function same(left, right) {
        return JSON.stringify(left) === JSON.stringify(right);
    }

    function notify() {
        const detail = snapshot();
        listeners.forEach(listener => listener(detail));
        document.dispatchEvent(new CustomEvent('cfg:statechange', { detail: detail }));
    }

    function init(config) {
        savedConfig = clone(config || {});
        draftConfig = clone(config || {});
        changed.clear();
        notify();
        return snapshot();
    }

    function beginSection(section) {
        activeSection = String(section || '');
        notify();
    }

    function get(path, options) {
        const source = options && options.saved ? savedConfig : draftConfig;
        return read(source, path);
    }

    function set(path, value) {
        if (!path) return;
        const before = read(draftConfig, path);
        write(draftConfig, path, value);
        if (same(read(savedConfig, path), read(draftConfig, path))) changed.delete(path);
        else changed.add(path);
        if (!same(before, value)) notify();
    }

    function markSaved(path, value) {
        if (!path) return;
        write(savedConfig, path, value);
        write(draftConfig, path, value);
        changed.delete(path);
        notify();
    }

    function dirtyPaths() {
        return Array.from(changed).sort();
    }

    function isDirty() {
        return changed.size > 0;
    }

    function buildPatch() {
        const patch = {};
        dirtyPaths().forEach(path => write(patch, path, read(draftConfig, path)));
        return patch;
    }

    function controlValue(element) {
        if (element.classList.contains('toggle')) return element.classList.contains('on');
        if (element.type === 'radio') return element.checked ? element.value : read(draftConfig, element.dataset.path);
        if (element.type === 'number' || element.type === 'range') {
            if (element.value === '') return 0;
            return element.step && parseFloat(element.step) < 1 ? parseFloat(element.value) : parseInt(element.value, 10);
        }
        if (element.dataset.type === 'array') {
            if (String(element.value).trim().startsWith('[')) {
                try { return JSON.parse(element.value); } catch (_) { /* use comma parsing below */ }
            }
            return String(element.value).split(',').map(value => value.trim()).filter(Boolean);
        }
        if (element.dataset.type === 'array-lines') {
            return String(element.value).split('\n').map(value => value.trim()).filter(Boolean);
        }
        if (element.dataset.type === 'json') {
            try { return JSON.parse(element.value); } catch (_) { return element.value; }
        }
        return element.value;
    }

    function syncElement(element) {
        if (!element || !element.dataset || !element.dataset.path) return false;
        if (element.type === 'radio' && !element.checked) return false;
        const path = element.dataset.path;
        const value = controlValue(element);
        const before = read(draftConfig, path);
        write(draftConfig, path, value);
        if (same(read(savedConfig, path), value)) changed.delete(path);
        else changed.add(path);
        return !same(before, value);
    }

    function syncFromDOM(root) {
        const scope = root || boundRoot || document;
        let didChange = false;
        scope.querySelectorAll('[data-path]').forEach(element => {
            didChange = syncElement(element) || didChange;
        });
        if (didChange) notify();
        return didChange;
    }

    function restoreDOM(root) {
        const scope = root || boundRoot || document;
        scope.querySelectorAll('[data-path]').forEach(element => {
            const value = read(draftConfig, element.dataset.path);
            if (element.classList.contains('toggle')) {
                element.classList.toggle('on', value === true);
                element.setAttribute('aria-checked', value === true ? 'true' : 'false');
            } else if (element.type === 'radio') {
                element.checked = String(value) === element.value;
            } else if (element.dataset.type === 'array') {
                element.value = Array.isArray(value) ? value.join(', ') : (value == null ? '' : value);
            } else if (element.dataset.type === 'array-lines') {
                element.value = Array.isArray(value) ? value.join('\n') : (value == null ? '' : value);
            } else if (element.dataset.type === 'json') {
                element.value = typeof value === 'string' ? value : JSON.stringify(value == null ? {} : value, null, 2);
            } else {
                element.value = value == null ? '' : value;
            }
        });
    }

    function bind(root) {
        boundRoot = root || document;
        if (boundRoot.__auraConfigStateBound === true) return;
        boundRoot.__auraConfigStateBound = true;
        const syncEvent = event => {
            const element = event.target && event.target.closest ? event.target.closest('[data-path]') : null;
            if (!element) return;
            if (event.type === 'click' && !element.classList.contains('toggle')) return;
            if (syncElement(element)) notify();
        };
        boundRoot.addEventListener('input', syncEvent);
        boundRoot.addEventListener('change', syncEvent);
        boundRoot.addEventListener('click', syncEvent);
    }

    function setRules(nextRules) {
        rules = Object.assign({}, nextRules || {});
    }

    function validationMessage(path, code) {
        return { path: path, code: code, message: code };
    }

    function validate() {
        const errors = [];
        Object.keys(rules).forEach(path => {
            const rule = rules[path] || {};
            const value = read(draftConfig, path);
            const empty = value == null || String(value).trim() === '';
            if (rule.required && empty) {
                errors.push(validationMessage(path, 'required'));
                return;
            }
            if (empty) return;
            if (rule.type === 'number' && (typeof value !== 'number' || Number.isNaN(value))) {
                errors.push(validationMessage(path, 'number'));
                return;
            }
            if (rule.min != null && Number(value) < Number(rule.min)) errors.push(validationMessage(path, 'min'));
            if (rule.max != null && Number(value) > Number(rule.max)) errors.push(validationMessage(path, 'max'));
            if (rule.pattern && !(new RegExp(rule.pattern)).test(String(value))) errors.push(validationMessage(path, 'pattern'));
            if (Array.isArray(rule.options) && !rule.options.includes(value)) errors.push(validationMessage(path, 'option'));
            if (rule.type === 'url') {
                try {
                    const parsed = new URL(String(value));
                    if (!['http:', 'https:'].includes(parsed.protocol)) throw new Error('protocol');
                } catch (_) {
                    errors.push(validationMessage(path, 'url'));
                }
            }
        });
        return { valid: errors.length === 0, errors: errors };
    }

    function commit(config) {
        return init(config == null ? draftConfig : config);
    }

    function discard() {
        draftConfig = clone(savedConfig);
        changed.clear();
        restoreDOM();
        notify();
        return clone(draftConfig);
    }

    function subscribe(listener) {
        if (typeof listener !== 'function') return function () {};
        listeners.add(listener);
        return function () { listeners.delete(listener); };
    }

    function snapshot() {
        return {
            section: activeSection,
            dirty: isDirty(),
            dirtyPaths: dirtyPaths(),
            saved: clone(savedConfig),
            draft: clone(draftConfig)
        };
    }

    window.AuraConfigState = {
        init: init,
        beginSection: beginSection,
        get: get,
        set: set,
        markSaved: markSaved,
        dirtyPaths: dirtyPaths,
        isDirty: isDirty,
        buildPatch: buildPatch,
        validate: validate,
        setRules: setRules,
        commit: commit,
        discard: discard,
        bind: bind,
        syncFromDOM: syncFromDOM,
        subscribe: subscribe,
        snapshot: snapshot
    };
})();
