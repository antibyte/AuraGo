// Shared readiness and busy-state controller for config actions.
(function () {
    'use strict';

    const registrations = new Map();
    const automatic = new Map();
    const TEST_ACTION_SELECTOR = [
        'button[id*="test"]',
        'button[class*="test-btn"]',
        'button[onclick*="Test"]',
        'button[onclick*="testConnection"]'
    ].join(',');
    const MODAL_SELECTOR = '.modal-overlay, .sec-modal-overlay, .mcp-modal-overlay, .prov-modal-overlay, .sql-modal-overlay';

    function text(key, fallback) {
        if (typeof t === 'function') {
            const value = t(key);
            if (value && value !== key) return value;
        }
        return fallback;
    }

    function elementFor(spec) {
        return spec.element || document.getElementById(spec.elementId || spec.id);
    }

    function hasValue(value) {
        if (typeof cfgIsMaskedSecret === 'function' && cfgIsMaskedSecret(value)) return true;
        if (Array.isArray(value)) return value.length > 0;
        if (typeof value === 'boolean') return true;
        return value != null && String(value).trim() !== '';
    }

    function controlSnapshot(container) {
        if (!container) return '';
        return [...container.querySelectorAll('input, select, textarea')].map(element => {
            const value = element.type === 'checkbox' || element.type === 'radio' ? element.checked : element.value;
            return [element.id || element.name || element.type, value];
        }).map(entry => JSON.stringify(entry)).join('|');
    }

    function selectorHasValue(selector) {
        const element = document.querySelector(selector);
        if (!element) return false;
        if (element.type === 'checkbox' || element.type === 'radio') return element.checked;
        return hasValue(element.value);
    }

    // data-config-action-draft="true": the action sends the draft, so unsaved changes
    // do not lock it and the required values are read from the draft (a mask counts).
    // "false": the action needs the running service, so anything unsaved locks it.
    function lockReason(spec) {
        const state = window.AuraConfigState;
        if (!state) return '';
        const element = elementFor(spec);
        const draftMode = element && element.dataset ? element.dataset.configActionDraft : '';
        const useDraft = draftMode === 'true';
        const source = { saved: !useDraft };
        if (draftMode === 'false' && state.isDirty()) {
            return text('config.feedback.runtime_only', 'Checks the running service - save first.');
        }
        if (spec.requiresSaved && !useDraft && state && state.isDirty()) {
            return text('config.precision.action_save_first', 'Save your changes before testing.');
        }
        if (spec.requiresSaved && !useDraft && spec.container && controlSnapshot(spec.container) !== spec.containerSnapshot) {
            return text('config.precision.action_save_first', 'Save your changes before testing.');
        }
        // A nested array lists alternatives: any one of them is enough.
        const present = path => Array.isArray(path) ? path.some(present) : hasValue(state.get(path, source));
        const missing = (spec.requiredPaths || []).find(path => !present(path));
        if (missing) {
            return useDraft
                ? text('config.feedback.missing_fields', 'Fill in the required fields first.')
                : text('config.precision.action_missing_fields', 'Complete and save the required fields first.');
        }
        const missingSelector = (spec.requiredSelectors || []).find(selector => !selectorHasValue(selector));
        const missingSelectorGroup = (spec.requiredAnySelectors || []).find(group => !group.some(selectorHasValue));
        if (missingSelector || missingSelectorGroup) {
            return useDraft
                ? text('config.feedback.missing_fields', 'Fill in the required fields first.')
                : text('config.precision.action_missing_fields', 'Complete and save the required fields first.');
        }
        const missingCredential = (spec.credentialPaths || []).find(path => !present(path));
        if (missingCredential || (typeof spec.credentialReady === 'function' && !spec.credentialReady())) {
            return useDraft
                ? text('config.feedback.missing_credential', 'Enter the required credential first.')
                : text('config.precision.action_missing_credential', 'Store the required credential in the vault first.');
        }
        return '';
    }

    function reasonElement(spec, element) {
        const id = (element.id || spec.id) + '-reason';
        let reason = document.getElementById(id);
        if (!reason) {
            reason = document.createElement('span');
            reason.id = id;
            reason.className = 'pw-action-reason';
            reason.hidden = true;
            element.insertAdjacentElement('afterend', reason);
        }
        return reason;
    }

    function apply(spec) {
        const element = elementFor(spec);
        if (!element) return;
        const reason = reasonElement(spec, element);
        const message = lockReason(spec);
        element.setAttribute('aria-disabled', message ? 'true' : 'false');
        // Other descriptions (field help, result line) stay; only the reason id is added.
        const described = new Set((element.getAttribute('aria-describedby') || '').split(/\s+/).filter(Boolean));
        if (!described.has(reason.id)) {
            described.add(reason.id);
            element.setAttribute('aria-describedby', [...described].join(' '));
        }
        reason.textContent = message;
        reason.hidden = !message;
        element.dataset.configActionLocked = message ? 'true' : 'false';
    }

    // spec.result: options for cfgActionResult. run() then returns the fetch Response (or
    // a parsed body) and the outcome is written next to the button.
    function register(id, spec) {
        const normalized = Object.assign({ id: id, requiresSaved: false, requiredPaths: [] }, spec || {});
        registrations.set(id, normalized);
        const element = elementFor(normalized);
        if (element && element.dataset.configActionBound !== 'true') {
            element.dataset.configActionBound = 'true';
            element.addEventListener('click', event => {
                event.preventDefault();
                run(id);
            });
        }
        apply(normalized);
        return normalized;
    }

    function refresh() {
        registrations.forEach(apply);
        automatic.forEach(apply);
    }

    function automaticSpec(element) {
        const catalog = window.AuraConfigCatalog || {};
        const rules = (catalog.actionRules || {})[element.id] || {};
        const container = element.closest(MODAL_SELECTOR);
        return Object.assign({
            id: element.id || '',
            element,
            requiresSaved: true,
            requiredPaths: [],
            credentialPaths: [],
            requiredSelectors: [],
            requiredAnySelectors: [],
            container,
            containerSnapshot: controlSnapshot(container)
        }, rules);
    }

    function autoEnhanceTestActions(root) {
        const scope = root && root.querySelectorAll ? root : document;
        const elements = [];
        if (scope.matches && scope.matches(TEST_ACTION_SELECTOR)) elements.push(scope);
        scope.querySelectorAll(TEST_ACTION_SELECTOR).forEach(element => elements.push(element));
        elements.forEach(element => {
            if (automatic.has(element)) return;
            const spec = automaticSpec(element);
            automatic.set(element, spec);
            element.dataset.requiresSaved = 'true';
            apply(spec);
        });
    }

    async function run(id) {
        const spec = registrations.get(id);
        if (!spec) return false;
        const element = elementFor(spec);
        const reason = lockReason(spec);
        if (reason || !element || element.dataset.configActionBusy === 'true') {
            apply(spec);
            return false;
        }

        const feedback = window.AuraConfigFeedback;
        if (spec.result && feedback) {
            const outcome = await feedback.actionResult(element, spec.run, spec.result);
            apply(spec);
            return !outcome.skipped;
        }
        // One busy system for all actions: disabled, aria-busy and data-config-action-busy.
        if (feedback) feedback.busy(element, true);
        try {
            await spec.run();
            return true;
        } finally {
            if (feedback) feedback.busy(element, false);
            apply(spec);
        }
    }

    // Dangerous switches ask before they are turned on (never when turning off).
    function confirmLabel(element) {
        const labelled = (element.getAttribute('aria-labelledby') || '').split(/\s+/).filter(Boolean)
            .map(id => document.getElementById(id)).filter(Boolean).map(node => node.textContent.trim()).join(' ');
        if (labelled) return labelled;
        const wrapped = element.labels && element.labels[0] ? element.labels[0] : element.closest('label');
        if (wrapped && wrapped.textContent.trim()) return wrapped.textContent.trim();
        const row = element.closest('.cfg-toggle-row, .cfg-toggle-row-highlight, .cfg-toggle-row-compact');
        const rowLabel = row && row.querySelector('.cfg-toggle-label');
        if (rowLabel && rowLabel.textContent.trim()) return rowLabel.textContent.trim();
        const group = element.closest('.field-group');
        const fieldLabel = group && group.querySelector('.field-label');
        if (fieldLabel && fieldLabel.textContent.trim()) return fieldLabel.textContent.trim();
        return element.getAttribute('aria-label') || element.dataset.path || '';
    }

    function confirmCategory(element) {
        const registry = (window.AuraConfigCatalog || {}).confirmOnEnable || {};
        const attribute = element.getAttribute('data-cfg-confirm-enable');
        if (attribute !== null) return ['delete', 'destroy', 'host', 'write'].includes(attribute) ? attribute : 'destroy';
        return registry[element.dataset.path || ''] || '';
    }

    function guardEnable(event) {
        const element = event.target.closest && event.target.closest('.toggle[data-path], input[type="checkbox"][data-cfg-confirm-enable]');
        if (!element || element.dataset.cfgConfirmed === 'true') return;
        const category = confirmCategory(element);
        if (!category) return;
        // A checkbox already holds its new value while its click is dispatched; a toggle flips later.
        const turningOn = element.classList.contains('toggle') ? !element.classList.contains('on') : element.checked;
        if (!turningOn) return;
        if (element.disabled || element.getAttribute('aria-disabled') === 'true' || element.dataset.configActionLocked === 'true') return;
        if (typeof showConfirm !== 'function') return;
        // Also stops the inline toggleBool / gmCfgToggleValue handlers and the draft sync.
        event.preventDefault();
        event.stopImmediatePropagation();
        if (element.dataset.cfgConfirming === 'true') return;
        element.dataset.cfgConfirming = 'true';
        const label = confirmLabel(element);
        Promise.resolve(showConfirm(
            text('config.confirm_enable.title', 'Enable {label}?').replace('{label}', () => label),
            text('config.confirm_enable.' + category, '').replace('{label}', () => label)
        )).then(approved => {
            delete element.dataset.cfgConfirming;
            if (!approved) return;
            element.dataset.cfgConfirmed = 'true';
            try { element.click(); } finally { delete element.dataset.cfgConfirmed; }
            if (typeof element.focus === 'function') element.focus();
        }, () => { delete element.dataset.cfgConfirming; });
    }

    document.addEventListener('cfg:statechange', refresh);
    document.addEventListener('cfg:section-rendered', event => autoEnhanceTestActions(event.detail && event.detail.root));
    document.addEventListener('click', guardEnable, true);
    document.addEventListener('click', event => {
        const element = event.target.closest && event.target.closest(TEST_ACTION_SELECTOR);
        if (!element) return;
        // A running action ignores further clicks, even if a re-render re-enabled the button.
        if (element.dataset.configActionBusy === 'true' || element.getAttribute('aria-busy') === 'true') {
            event.preventDefault();
            event.stopImmediatePropagation();
            return;
        }
        autoEnhanceTestActions(element);
        const spec = automatic.get(element);
        if (!spec || !lockReason(spec)) return;
        event.preventDefault();
        event.stopImmediatePropagation();
        apply(spec);
        element.focus();
    }, true);

    const observer = new MutationObserver(records => {
        records.forEach(record => {
            record.addedNodes.forEach(node => {
                if (node.nodeType === Node.ELEMENT_NODE) autoEnhanceTestActions(node);
            });
            if (record.type === 'attributes' && record.target.matches(MODAL_SELECTOR) && !record.target.classList.contains('is-hidden')) {
                record.target.querySelectorAll(TEST_ACTION_SELECTOR).forEach(element => {
                    autoEnhanceTestActions(element);
                    const spec = automatic.get(element);
                    if (spec) {
                        spec.containerSnapshot = controlSnapshot(spec.container);
                        apply(spec);
                    }
                });
            }
        });
    });
    const observe = () => {
        const content = document.getElementById('content');
        if (document.body) observer.observe(document.body, { childList: true, subtree: true, attributes: true, attributeFilter: ['class'] });
        autoEnhanceTestActions(content || document);
    };
    if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', observe, { once: true });
    else observe();

    window.AuraConfigActions = {
        register: register,
        refresh: refresh,
        run: run,
        autoEnhanceTestActions: autoEnhanceTestActions
    };
})();
