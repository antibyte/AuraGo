/* Metadata-only mail delivery accounting. Explicit resolution never sends mail. */
(function () {
    'use strict';
    window.mountMailReceiptsCard = function (root) {
        if (!root || !root.isConnected) return;
        const lifetime = new AbortController();
        const labels = key => t('config.mail_receipts.' + key);
        const element = (tag, text, className) => {
            const node = document.createElement(tag);
            if (text !== undefined) node.textContent = text;
            if (className) node.className = className;
            return node;
        };
        const stop = () => lifetime.abort();
        window.addEventListener('cfg:section-leave', stop, { once: true, signal: lifetime.signal });
        window.addEventListener('pagehide', stop, { once: true, signal: lifetime.signal });
        root.append(element('h3', labels('title')), element('p', labels('help')));
        const occupancy = element('p');
        const controls = element('div', undefined, 'cfg-action-row');
        const filterLabel = element('label', labels('channel') + ' ');
        const filter = element('select', undefined, 'field-select');
        filter.append(new Option(labels('all'), ''));
        const channelNames = { ntfy: 'ntfy', pushover: 'Pushover', telegram: 'Telegram', whatsapp: 'WhatsApp', discord: 'Discord', push: 'Web Push', telnyx: 'Telnyx', cyd: 'CYD', legacy: labels('legacy') };
        Object.entries(channelNames).forEach(([id, name]) => filter.append(new Option(name, id)));
        filterLabel.append(filter);
        const refresh = element('button', labels('refresh'), 'btn btn-secondary');
        refresh.type = 'button';
        controls.append(filterLabel, refresh);
        const status = element('p');
        status.setAttribute('role', 'status');
        status.setAttribute('aria-live', 'polite');
        const scroll = element('div');
        scroll.style.overflowX = 'auto';
        const table = element('table', undefined, 'cfg-table');
        const head = element('thead');
        const heading = element('tr');
        ['channel', 'time', 'status', 'actions'].forEach(key => {
            const cell = element('th', labels(key));
            cell.scope = 'col';
            heading.append(cell);
        });
        head.append(heading);
        const body = element('tbody');
        table.append(head, body);
        scroll.append(table);
        const more = element('button', labels('more'), 'btn btn-secondary');
        more.type = 'button';
        more.hidden = true;
        root.append(occupancy, controls, status, scroll, more);
        let busy = false;
        let cursor = '';
        const connected = () => root.isConnected && !lifetime.signal.aborted;
        const lock = value => {
            busy = value;
            root.setAttribute('aria-busy', String(value));
            root.querySelectorAll('button,select').forEach(node => { node.disabled = value || node.dataset.active === 'true'; });
        };
        const timestamp = raw => {
            const date = new Date(raw);
            return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString();
        };
        const resolve = async (receipt, resolution) => {
            if (busy || receipt.status === 'active') return;
            const confirmed = await showConfirm(labels(resolution === 'sent' ? 'confirm_sent' : 'confirm_not_sent'));
            if (!confirmed || !connected() || busy) return;
            lock(true);
            status.textContent = labels('loading');
            let uncertain = false;
            try {
                const response = await fetch('/api/mail-notifications/receipts/' + encodeURIComponent(receipt.id) + '/resolve', {
                    method: 'POST', headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ revision: receipt.revision, resolution }), signal: lifetime.signal
                });
                uncertain = !response.ok;
            } catch (error) {
                if (!connected()) return;
                uncertain = true;
            } finally {
                if (connected()) lock(false);
            }
            // A lost reply is reconciled by reading; never replay the mutation.
            await load(false, uncertain ? labels('reread') : labels('resolved'));
        };
        const row = receipt => {
            const tr = element('tr');
            tr.append(element('td', channelNames[receipt.channel] || labels('legacy')), element('td', timestamp(receipt.attempted_at)));
            const state = element('td', labels(receipt.status === 'active' ? 'active' : receipt.status === 'sent' ? 'sent' : 'unknown'));
            if (receipt.status === 'sent' && receipt.expires_at) state.append(element('small', ' · ' + labels('expires') + ' ' + timestamp(receipt.expires_at)));
            const actions = element('td');
            ['sent', 'not_sent'].forEach(resolution => {
                const button = element('button', labels(resolution === 'sent' ? 'mark_sent' : 'mark_not_sent'), 'btn btn-secondary');
                button.type = 'button';
                button.dataset.active = String(receipt.status === 'active');
                button.disabled = receipt.status === 'active';
                button.addEventListener('click', () => resolve(receipt, resolution));
                actions.append(button);
            });
            tr.append(state, actions);
            return tr;
        };
        async function load(append, notice) {
            if (busy || !connected()) return;
            lock(true);
            status.textContent = labels('loading');
            try {
                const query = new URLSearchParams({ limit: '50' });
                if (filter.value) query.set('channel', filter.value);
                if (append && cursor) query.set('cursor', cursor);
                const response = await fetch('/api/mail-notifications/receipts?' + query, { cache: 'no-store', signal: lifetime.signal });
                if (!response.ok) throw new Error('receipt_read_failed');
                const page = await response.json();
                if (!connected()) return;
                if (!append) body.replaceChildren();
                (page.receipts || []).forEach(receipt => body.append(row(receipt)));
                occupancy.textContent = (page.channels || []).map(usage => (channelNames[usage.channel] || labels('legacy')) + ': ' + Number(usage.count) + (usage.limit ? '/' + Number(usage.limit) : '')).join(' · ');
                cursor = page.next_cursor || '';
                more.hidden = !cursor;
                status.textContent = notice || (body.children.length ? '' : labels('empty'));
            } catch (error) {
                if (connected()) status.textContent = labels('error');
            } finally {
                if (connected()) lock(false);
            }
        }
        refresh.addEventListener('click', () => load(false));
        filter.addEventListener('change', () => load(false));
        more.addEventListener('click', () => load(true));
        void load(false);
    };
})();
