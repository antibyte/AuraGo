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
    async function loadLegacy(root, after = 0) {
        if (!root) return;
        const copy = (key) => t('personality_legacy.' + key);
        root.replaceChildren(node('h3', copy('title')), node('p', copy('hint')));
        const status = node('p', copy('loading'));
        status.setAttribute('role', 'status');
        const list = node('div', '', 'personality-legacy-list');
        const reload = node('button', copy('reload'), 'btn btn-secondary');
        reload.type = 'button';
        reload.dataset.action = 'reload';
        reload.addEventListener('click', () => loadLegacy(root));
        root.append(status, list, reload);
        try {
            const response = await fetch('/api/personality/legacy?limit=20&after=' + after, {credentials: 'same-origin', cache: 'no-store'});
            if (!response.ok) throw new Error('legacy unavailable');
            const data = await response.json();
            if (!root.isConnected || !root.contains(list)) return;
            status.textContent = data.entries.length ? '' : copy('empty');
            for (const entry of data.entries) {
                const item = node('article', '', 'personality-legacy-entry');
                item.dataset.entryId = String(entry.id);
                const kind = ['profile','character','emotion','mood'].includes(entry.kind) ? copy('kind_' + entry.kind) : entry.kind;
                item.append(node('strong', '#' + entry.id + ' · ' + kind));
                item.append(node('pre', JSON.stringify(entry.payload, null, 2)));
                const actions = node('div', '', 'personality-legacy-actions');
                const protectedNote = !!(entry.payload && entry.payload.protected);
                for (const action of ['discard', 'adopt']) {
                    if (action === 'adopt' && !['profile', 'character'].includes(entry.kind)) continue;
                    const button = node('button', action === 'discard' ? t('common.btn_delete') : copy('adopt'), 'btn btn-secondary');
                    button.type = 'button'; button.dataset.action = action;
                    button.disabled = action === 'discard' && protectedNote;
                    if (button.disabled) button.title = copy('protected');
                    button.addEventListener('click', async () => {
                        const buttons = [...root.querySelectorAll('button')];
                        const disabled = buttons.map(b => b.disabled);
                        buttons.forEach(b => { b.disabled = true; });
                        status.textContent = copy('loading');
                        try {
                            const result = await fetch('/api/personality/legacy/review', {
                                method: 'POST', credentials: 'same-origin', headers: {'Content-Type': 'application/json'},
                                body: JSON.stringify({id: entry.id, fingerprint: entry.fingerprint, action})
                            });
                            if (!root.isConnected || !root.contains(item)) return;
                            if (!result.ok) { status.textContent = copy(result.status === 409 ? 'stale' : 'failed'); return; }
                            item.remove(); status.textContent = copy('done');
                        } catch (_) {
                            if (root.contains(status)) status.textContent = copy('failed');
                        } finally { buttons.forEach((b, i) => { b.disabled = disabled[i]; }); }
                    });
                    actions.append(button);
                }
                if (protectedNote) item.append(node('p', copy('protected')));
                item.append(actions); list.append(item);
            }
            if (data.has_more) {
                const next = node('button', copy('next'), 'btn btn-secondary');
                next.type = 'button'; next.addEventListener('click', () => loadLegacy(root, data.next_after)); root.append(next);
                next.dataset.action = 'next-page';
            }
        } catch (_) { if (root.contains(status)) status.textContent = copy('failed'); }
    }
    window.AuraPersonalityDynamics = {render, load, loadLegacy};
})();
