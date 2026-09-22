/* Shared read-only personality indicators and the explicit short-term reset. */
(() => {
    'use strict';
    const label = (key) => t('personality_dynamics.' + key);
    const node = (tag, text, className) => {
        const el = document.createElement(tag);
        if (text) el.textContent = text;
        if (className) el.className = className;
        return el;
    };

    function render(root, data, onReset) {
        if (!root) return;
        const dynamics = data && data.dynamics;
        const revision = Number(dynamics && dynamics.revision);
        if (Number.isFinite(revision) && revision < (root._personalityRevision || 0)) return;
        if (root._personalityResetting) return;
        if (Number.isFinite(revision)) root._personalityRevision = revision;
        root.classList.add('personality-dynamics');
        root.replaceChildren(node('h3', label('title')));
        if (!data || !data.enabled || !dynamics) {
            root.append(node('p', label(data && !data.enabled ? 'disabled' : 'unavailable')));
            return;
        }
        const meters = node('div', '', 'personality-dynamics-meters');
        for (const key of ['load', 'familiarity', 'friction']) {
            const raw = Number(dynamics[key]);
            const value = Number.isFinite(raw) ? Math.min(1, Math.max(0, raw)) : 0;
            const row = node('label', '', 'personality-dynamics-meter');
            const heading = node('span', label(key));
            heading.append(node('strong', ' ' + Math.round(value * 100) + '%'));
            const meter = node('meter');
            meter.min = 0; meter.max = 1; meter.value = value;
            meter.setAttribute('aria-label', label(key));
            meter.dataset.dynamic = key;
            row.append(heading, meter);
            meters.append(row);
        }
        const trend = ['steady', 'recovering', 'strained'].includes(dynamics.trend) ? dynamics.trend : 'steady';
        root.append(meters, node('p', label('trend') + ': ' + label(trend)), node('p', label('hint'), 'personality-dynamics-hint'));
        const button = node('button', label('reset'), 'btn btn-secondary');
        button.type = 'button'; button.dataset.action = 'reset-personality-dynamics';
        const status = node('span', '', 'personality-dynamics-status');
        status.setAttribute('role', 'status');
        button.addEventListener('click', async () => {
            if (root._personalityResetting) return;
            root._personalityResetting = true;
            button.disabled = true;
            try {
                const response = await fetch('/api/personality/dynamics/reset', {method: 'POST', credentials: 'same-origin'});
                if (!response.ok) throw new Error('reset failed');
                const updated = await response.json();
                root._personalityResetting = false;
                render(root, updated, onReset);
                if (onReset) onReset(updated);
                const result = root.querySelector('[role="status"]');
                if (result) result.textContent = label('reset_done');
            } catch (_) {
                status.textContent = label('unavailable');
            } finally {
                root._personalityResetting = false;
                button.disabled = false;
            }
        });
        root.append(button, status);
    }

    async function load(root) {
        if (!root) return;
        root.textContent = label('loading');
        try {
            const response = await fetch('/api/personality/state', {credentials: 'same-origin', cache: 'no-store'});
            if (!response.ok) throw new Error('state unavailable');
            const data = await response.json();
            if (root.isConnected) render(root, data);
        } catch (_) {
            if (root.isConnected) root.textContent = label('unavailable');
        }
    }
    window.AuraPersonalityDynamics = {render, load};
})();
