/* Docker Manager panes: the glue between the app and the things that live as long as a pane is open. A classic script
   (window.DockerManagerPanes) with no DOM work at load.
   - Follow panels: the log panel and the terminal panel of the container drawer, the stack pane and the external project pane are
     created the first time their tab is shown, kept (hidden) while the pane stays on the same container, stack or project, and shut
     down when it changes or closes. A log panel follows only while its tab is visible (the pane is not covered, the window is shown).
     A terminal panel keeps a connected shell open for as long as its pane exists (other tab, covered pane, minimised or hidden window)
     and ends it (end frame, socket closed) when the pane, the container or the window closes, or when read-only is switched on.
     A panel whose mount element was replaced is built again.
   - Load cells: [data-load="host/containerid"] placeholders (table rows, service lines) are filled in place with the CPU and
     memory sparklines of the live stream. Nothing here repaints a table; a stats message touches only the cells of its container.
   - visibleLoads: which of those cells are on screen, for the subscription of the stream (at most 50 containers per host). */
(function () {
    'use strict';

    const MAX_IDS = 50;

    function create(deps) {
        const { t, model, charts, logs, terminal, store, findRow, lang } = deps;
        const helpers = window.DockerManagerTerminal;
        const slots = new Map();

        function slotFor(name, key) {
            let slot = slots.get(name);
            if (slot && slot.key !== key) { release(name); slot = null; }
            if (!slot) { slot = { key, logs: null, terminal: null, logsEl: null, terminalEl: null }; slots.set(name, slot); }
            return slot;
        }
        function release(name) {
            const slot = slots.get(name);
            if (!slot) return;
            slot.logs?.dispose();
            slot.terminal?.dispose();
            slots.delete(name);
        }

        // Creates the panels of a slot on demand and activates only the one whose tab is `tab` ('' = the pane is not shown). Activating
        // makes a log panel follow; a terminal panel only fits and focuses, its shell stays open while it is inactive.
        function sync(name, key, tab, mounts, make, focus) {
            const slot = slotFor(name, key);
            if (slot.logs && slot.logsEl !== mounts.logs) { slot.logs.dispose(); slot.logs = null; }
            if (slot.terminal && slot.terminalEl !== mounts.terminal) { slot.terminal.dispose(); slot.terminal = null; }
            if (tab === 'logs' && !slot.logs && mounts.logs) { slot.logs = make.logs(mounts.logs); slot.logsEl = mounts.logs; }
            if (tab === 'terminal' && !slot.terminal && mounts.terminal) { slot.terminal = make.terminal(mounts.terminal, focus); slot.terminalEl = mounts.terminal; }
            slot.logs?.setActive(tab === 'logs');
            slot.terminal?.setActive(tab === 'terminal');
            if (tab === 'terminal' && focus) slot.terminal?.focus();
            return slot;
        }

        const makeLogs = (target, services) => el => logs.open(el, Object.assign({ services }, target));
        const makeTerminal = (hostId, targets) => (el, focus) => terminal.open(el, { hostId, targets, readonly: () => !store.canMutate(), autofocus: !!focus });

        // The container drawer. liveEl holds [data-live-panel="logs"|"terminal"]; detail is state.detail or null.
        function drawer(liveEl, detail, focus) {
            if (!detail) { release('drawer'); return; }
            const row = findRow(detail.key);
            if (!row) { release('drawer'); return; }
            const mounts = { logs: liveEl.querySelector('[data-live-panel="logs"]'), terminal: liveEl.querySelector('[data-live-panel="terminal"]') };
            if (mounts.logs) mounts.logs.hidden = detail.tab !== 'logs';
            if (mounts.terminal) mounts.terminal.hidden = detail.tab !== 'terminal';
            const target = { kind: 'container', hostId: detail.hostId, ref: detail.id, label: model.containerName(row.c) };
            const slot = sync('drawer', detail.key, detail.tab, mounts, {
                logs: makeLogs(target),
                terminal: makeTerminal(detail.hostId, () => { const current = findRow(detail.key); return current ? [helpers.targetOf(current.c)] : []; })
            }, focus);
            slot.terminal?.update();
        }

        // The stack pane. shellEl holds [data-mount="logs"|"terminal"]; `shown` is false while the drawer or the new-stack view covers it.
        function stack(shellEl, current, shown, focus) {
            if (!current || current.status !== 'ok' || !current.detail) { release('stack'); return; }
            const services = (Array.isArray(current.detail.service_list) ? current.detail.service_list : []).map(service => service.name).filter(Boolean);
            const mounts = { logs: shellEl.querySelector('[data-mount="logs"]'), terminal: shellEl.querySelector('[data-mount="terminal"]') };
            const containers = () => { const entry = store.state.byHost[current.hostId]; return entry ? entry.containers : []; };
            const slot = sync('stack', current.key, shown ? current.tab : '', mounts, {
                logs: makeLogs({ kind: 'stack', hostId: current.hostId, name: current.name, label: current.name }, services),
                terminal: makeTerminal(current.hostId, () => { const now = store.state.stack; return now && now.key === current.key && now.detail ? helpers.stackTargets(now.detail, containers()) : []; })
            }, focus);
            slot.logs?.update({ services });
            slot.terminal?.update();
        }

        // The external project pane (overview and logs).
        function project(shellEl, current, shown) {
            if (!current) { release('project'); return; }
            const mounts = { logs: shellEl.querySelector('[data-mount="logs"]'), terminal: null };
            sync('project', current.key, shown ? current.tab : '', mounts, { logs: makeLogs({ kind: 'project', hostId: current.hostId, name: current.name, label: current.name }), terminal: null }, false);
        }

        // ---- load cells --------------------------------------------------------------------------------------------------

        function fill(el, points) {
            const last = points[points.length - 1], sig = last ? last.t + ':' + points.length : '';
            if (el.dataset.sig === sig) return;
            el.dataset.sig = sig;
            el.innerHTML = last ? charts.loadCell(points, { lang }) : '';
            el.title = last ? t('stat_title', {
                cpu: charts.percent(last.cpu, lang), mem: charts.bytes(last.mem, lang), rx: charts.bytes(charts.rate(points, 'net_rx'), lang), tx: charts.bytes(charts.rate(points, 'net_tx'), lang)
            }) : '';
        }
        const splitKey = key => ({ host: key.slice(0, key.indexOf('/')), id: key.slice(key.indexOf('/') + 1) });

        // Fills the cells of the containers in `keys` (a Set of "host/id"), or every cell when `keys` is omitted.
        function patchLoads(rootEl, keys) {
            for (const el of rootEl.querySelectorAll('[data-load]')) {
                const key = el.dataset.load;
                if (keys && !keys.has(key)) continue;
                const part = splitKey(key);
                fill(el, store.liveRing(part.host, part.id));
            }
        }

        // entries = [{root, viewport}]: the containers whose load cells are rendered and inside the viewport, {host: [id]}, at most 50 per host.
        function visibleLoads(entries) {
            const out = {};
            for (const entry of entries) {
                if (!entry.root || !entry.viewport) continue;
                const box = entry.viewport.getBoundingClientRect();
                for (const el of entry.root.querySelectorAll('[data-load]')) {
                    if (!el.getClientRects().length) continue;
                    const rect = el.getBoundingClientRect();
                    if (rect.bottom < box.top || rect.top > box.bottom) continue;
                    const part = splitKey(el.dataset.load), ids = out[part.host] || (out[part.host] = []);
                    if (ids.length < MAX_IDS && !ids.includes(part.id)) ids.push(part.id);
                }
            }
            return out;
        }

        // ---- rows --------------------------------------------------------------------------------------------------------

        // Brings the children of `parent` (the rows of a table body) in line with items = [{key, html}] without rebuilding the rows that did
        // not change. `cache` is a Map the caller keeps: it remembers the markup and the element of every key. Only new and changed rows
        // are parsed (one template for all of them); a row that stays keeps its element, so the focus on it and its load cell stay, and
        // is moved only when the order changed. Returns whether the DOM changed.
        function patchRows(parent, items, cache) {
            const next = new Map(), fresh = [];
            for (const item of items) {
                const known = cache.get(item.key);
                if (known && known.html === item.html && known.el.parentNode === parent && !next.has(item.key)) next.set(item.key, known); else fresh.push(item);
            }
            if (fresh.length) {
                const template = document.createElement('template');
                template.innerHTML = fresh.map(item => item.html).join('');
                const parsed = template.content.children;
                if (parsed.length !== fresh.length) throw new Error('Docker Manager: rows did not parse one to one');
                fresh.forEach((item, index) => next.set(item.key, { html: item.html, el: parsed[index] }));
            }
            const keep = new Set();
            for (const entry of next.values()) keep.add(entry.el);
            let changed = fresh.length > 0, cursor = parent.firstElementChild;
            for (const item of items) {
                const el = next.get(item.key).el;
                // Elements that are no longer wanted (gone rows, the old element of a changed row) are dropped as the walk passes them.
                while (cursor && cursor !== el && !keep.has(cursor)) { const stale = cursor; cursor = cursor.nextElementSibling; stale.remove(); changed = true; }
                if (cursor === el) cursor = cursor.nextElementSibling;
                else { parent.insertBefore(el, cursor); changed = true; }
            }
            while (cursor) { const stale = cursor; cursor = cursor.nextElementSibling; stale.remove(); changed = true; }
            cache.clear();
            for (const [key, entry] of next) cache.set(key, entry);
            return changed;
        }

        function dispose() { for (const name of [...slots.keys()]) release(name); }

        return { drawer, stack, project, release, patchLoads, patchRows, visibleLoads, dispose };
    }

    window.DockerManagerPanes = { create, MAX_IDS };
})();
