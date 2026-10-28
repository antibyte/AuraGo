/* Docker Manager groups: pure helpers that keep long container lists short, no DOM and no network. A classic script
   (window.DockerManagerGroups) so that scripts/test-docker-manager-ui.mjs can run it in a Node vm. A host with thousands of
   containers (a CI runner leaves one stopped container per job) used to make the table, and the single containers of the Stacks
   list, thousands of rows.
   - entries(): the stopped single containers (no compose project, not running, paused or restarting) of one image become one group
     entry once there are MIN_GROUP of them; running containers and every other container stay rows, always. Plain rows come first,
     then the image groups by image name.
   - plan(): what of a section is rendered. A section and an opened group show at most PAGE entries; the rest is "N hidden" and a
     button adds PAGE more (showMore).
   - state(): the open/closed and limit state the app keeps. An image group is closed until opened; while a search or a state filter is
     active every group with matches is open (its entries are the matches) until the admin closes it, and that choice ends with the
     next change of the search or filter. */
(function () {
    'use strict';

    const MIN_GROUP = 5;
    const PAGE = 200;
    const ACTIVE = ['running', 'paused', 'restarting'];

    const text = value => (value == null ? '' : String(value));
    const lower = value => text(value).toLowerCase();
    const array = value => (Array.isArray(value) ? value : []);
    const asRows = rows => array(rows).map(row => ({ type: 'row', row }));

    // A container the image grouping may take. Running, paused and restarting containers are never grouped, nor is a protected one
    // (the lock must stay visible on its own row) or a helper container of the Stacks feature.
    function groupable(container) {
        const c = container || {};
        return !text(c.compose_project) && !ACTIVE.includes(lower(c.state)) && !window.DockerManagerModel.isProtected(c);
    }
    const imageKey = image => 'i:' + text(image);

    // rows: the container rows of one section in display order. options.image switches the image grouping on (the single containers).
    // Returns [{type: 'row', row} ..., {type: 'image', key, image, rows, counts: {total, problem}, newest}]; newest is the largest
    // `created` (seconds) of the group.
    function entries(rows, options) {
        const list = array(rows), o = options || {};
        if (!o.image) return asRows(list);
        const model = window.DockerManagerModel, min = o.min > 1 ? o.min : MIN_GROUP, buckets = new Map();
        for (const row of list) {
            if (!row || !groupable(row.c)) continue;
            const key = imageKey(row.c.image);
            let bucket = buckets.get(key);
            if (!bucket) { bucket = { type: 'image', key, image: text(row.c.image), rows: [] }; buckets.set(key, bucket); }
            bucket.rows.push(row);
        }
        const groups = [...buckets.values()].filter(bucket => bucket.rows.length >= min);
        if (!groups.length) return asRows(list);
        const taken = new Set();
        for (const group of groups) {
            let problem = 0, newest = 0;
            for (const row of group.rows) {
                taken.add(row);
                if (model.problemOf(row.c)) problem += 1;
                newest = Math.max(newest, Number(row.c.created) || 0);
            }
            group.counts = { total: group.rows.length, problem };
            group.newest = newest;
        }
        groups.sort((a, b) => model.compareText(model.shortImage(a.image), model.shortImage(b.image)) || model.compareText(a.image, b.image));
        return asRows(list.filter(row => !taken.has(row))).concat(groups);
    }

    const moreOf = (key, hidden) => (hidden > 0 ? { key, hidden, count: Math.min(PAGE, hidden) } : null);

    // The rendered part of a plain list (stacks, external projects): {items, more}. more = {key, hidden, count} or null.
    function page(list, key, limit) {
        const all = array(list), cap = limit(key);
        return { items: all.slice(0, cap), more: moreOf(key, all.length - cap) };
    }

    // The rendered part of a section made of entries(). view = {limit(key), open(key)} (state().view). An opened image group shows its
    // first `limit` members and a `more` of its own; a closed one none. Returns {items, more}: items are rows or
    // {type: 'image', group, open, members, more}.
    function plan(list, section, view) {
        const shown = page(list, section, view.limit), items = [];
        for (const entry of shown.items) {
            if (entry.type !== 'image') { items.push(entry); continue; }
            const open = !!view.open(entry.key), members = open ? page(entry.rows, entry.key, view.limit) : { items: [], more: null };
            items.push({ type: 'image', group: entry, open, members: members.items, more: members.more });
        }
        return { items, more: shown.more };
    }

    // The state the app keeps. expanded: image groups the admin opened; closed: groups the admin closed while a search or filter opens
    // them all; limits: key -> how many entries a section or group shows; narrowKey: what the search and filter were when closed and
    // limits were last reset.
    function state() { return { expanded: new Set(), closed: new Set(), limits: new Map(), narrowKey: '' }; }

    // Call before every paint with the current search/filter. A change of it ends the admin's closings and the "show more" steps.
    function sync(s, narrowKey) {
        if (s.narrowKey === narrowKey) return;
        s.narrowKey = narrowKey;
        s.closed.clear();
        s.limits.clear();
    }
    function view(s, narrowed) {
        return {
            narrowed: !!narrowed,
            open: key => (narrowed ? !s.closed.has(key) : s.expanded.has(key)),
            limit: key => s.limits.get(key) || PAGE
        };
    }
    function toggle(s, narrowed, key) {
        const set = narrowed ? s.closed : s.expanded;
        if (set.has(key)) set.delete(key); else set.add(key);
    }
    function showMore(s, key) { s.limits.set(key, (s.limits.get(key) || PAGE) + PAGE); }

    window.DockerManagerGroups = { MIN_GROUP, PAGE, groupable, imageKey, entries, page, plan, state, sync, view, toggle, showMore };
})();
