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

    function lockReason(spec) {
        const state = window.AuraConfigState;
        if (!state) return '';
        if (spec.requiresSaved && state && state.isDirty()) {
            return text('config.precision.action_save_first', 'Save your changes before testing.');
        }
        if (spec.requiresSaved && spec.container && controlSnapshot(spec.container) !== spec.containerSnapshot) {
            return text('config.precision.action_save_first', 'Save your changes before testing.');
        }
        const missing = (spec.requiredPaths || []).find(path => !hasValue(state ? state.get(path, { saved: true }) : undefined));
        if (missing) {
            return text('config.precision.action_missing_fields', 'Complete and save the required fields first.');
        }
        const missingSelector = (spec.requiredSelectors || []).find(selector => !selectorHasValue(selector));
        const missingSelectorGroup = (spec.requiredAnySelectors || []).find(group => !group.some(selectorHasValue));
        if (missingSelector || missingSelectorGroup) {
            return text('config.precision.action_missing_fields', 'Complete and save the required fields first.');
        }
        const missingCredential = (spec.credentialPaths || []).find(path => !hasValue(state.get(path, { saved: true })));
        if (missingCredential) {
            return text('config.precision.action_missing_credential', 'Store the required credential in the vault first.');
        }
        if (typeof spec.credentialReady === 'function' && !spec.credentialReady()) {
            return text('config.precision.action_missing_credential', 'Store the required credential in the vault first.');
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
        element.setAttribute('aria-describedby', reason.id);
        reason.textContent = message;
        reason.hidden = !message;
        element.dataset.configActionLocked = message ? 'true' : 'false';
    }

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

        element.dataset.configActionBusy = 'true';
        element.disabled = true;
        element.setAttribute('aria-busy', 'true');
        try {
            await spec.run();
            return true;
        } finally {
            element.dataset.configActionBusy = 'false';
            element.disabled = false;
            element.setAttribute('aria-busy', 'false');
            apply(spec);
        }
    }

    document.addEventListener('cfg:statechange', refresh);
    document.addEventListener('cfg:section-rendered', event => autoEnhanceTestActions(event.detail && event.detail.root));
    document.addEventListener('click', event => {
        const element = event.target.closest && event.target.closest(TEST_ACTION_SELECTOR);
        if (!element) return;
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
