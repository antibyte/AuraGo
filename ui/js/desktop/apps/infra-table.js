// Infra inventory table: the virtualized List view. infra.js inserts `el`,
// shows it in table view and calls render() on data, filter and selection
// events; scrolling re-renders the visible slice by itself.
(function () {
    'use strict';

    function create(app) {
        const { store, state, t, esc, model } = app;
        const holder = document.createElement('div');
        holder.innerHTML = `<section class="infra-table-view" data-table-view hidden aria-label="${esc(t('desktop.infra_inventory'))}"><div class="infra-table-head"><h2>${esc(t('desktop.infra_inventory'))}</h2><span data-table-total></span></div><div class="infra-table-scroll" data-table-scroll tabindex="0"><table><thead><tr><th>${esc(t('desktop.infra_name'))}</th><th>${esc(t('desktop.infra_type'))}</th><th>${esc(t('desktop.infra_hardware'))}</th><th>${esc(t('desktop.infra_address'))}</th><th>${esc(t('desktop.infra_source'))}</th><th>${esc(t('desktop.infra_status'))}</th><th>${esc(t('desktop.infra_column_power'))}</th></tr></thead><tbody data-table-body></tbody></table></div></section>`;
        const el = holder.firstElementChild;
        const scroll = el.querySelector('[data-table-scroll]'), body = el.querySelector('[data-table-body]'), total = el.querySelector('[data-table-total]');

        const rowMarkup = node => {
            const selected = state.selection.has(node.id);
            const powerKey = model.powerLabel(node.power);
            const power = powerKey ? `<span class="infra-power-pill" data-power="${esc(node.power)}">${esc(t(powerKey))}</span>` : '—';
            const nodeState = model.sourceState(node);
            const sources = (node.sources || []).map(source => t(`desktop.infra_source_${source.kind}`)).join(', ') || t('desktop.infra_manual');
            return `<tr data-row="${esc(node.id)}" class="${selected ? 'is-selected' : ''}"><td><button type="button" class="infra-table-name" data-select="${esc(node.id)}" aria-current="${selected ? 'true' : 'false'}">${app.icon(node.icon)}<span>${esc(node.name)}</span></button></td><td>${esc(app.labelForKind(node.kind))}</td><td>${esc([node.profile?.vendor, node.profile?.model].filter(Boolean).join(' · ') || '—')}</td><td>${esc(node.profile?.ip_address || node.profile?.mac_address || '—')}</td><td>${esc(sources)}</td><td><span class="infra-state-pill" data-state="${esc(nodeState)}"><i></i>${esc(app.statusLabel(nodeState))}</span></td><td>${power}</td></tr>`;
        };
        function render() {
            const items = state.nodes.filter(node => store.matches(node)).sort((a, b) => a.name.localeCompare(b.name));
            const slice = model.virtualSlice(items, scroll.scrollTop, scroll.clientHeight || 600);
            total.textContent = t('desktop.infra_asset_count').replace('{count}', items.length);
            body.innerHTML = `<tr class="infra-virtual-spacer"><td colspan="7" style="height:${slice.before}px"></td></tr>${slice.rows.map(rowMarkup).join('')}<tr class="infra-virtual-spacer"><td colspan="7" style="height:${slice.after}px"></td></tr>`;
        }
        // Selecting rebuilds the rows, so keyboard focus is put back on the new button.
        const onClick = event => {
            const button = event.target.closest('[data-select]');
            if (!button) return;
            const id = button.dataset.select;
            store.select([id]);
            render();
            [...body.querySelectorAll('[data-select]')].find(item => item.dataset.select === id)?.focus({ preventScroll: true });
        };
        body.addEventListener('click', onClick);
        scroll.addEventListener('scroll', render, { passive: true });
        function dispose() {
            body.removeEventListener('click', onClick);
            scroll.removeEventListener('scroll', render);
            el.remove();
        }

        return { el, render, dispose };
    }

    window.InfraAppTable = { create };
})();
