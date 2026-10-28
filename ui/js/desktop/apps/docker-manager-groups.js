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

    // The cap hides what comes last, so a list longer than one page is ranked first: running, paused and restarting containers and
    // those with a problem (exited with an error, dead, unhealthy) lead, then the image groups that hold a failed container, then the
    // other rows, then the other groups. The sort is stable (the name order stays inside a rank), so the cap only ever hides stopped
    // containers that are fine. A list that fits on a page is left in its order.
    function ranked(list) {
        if (list.length <= PAGE) return list;
        const model = window.DockerManagerModel;
        const rank = entry => {
            if (entry.type === 'image') return entry.counts.problem ? 1 : 3;
            const c = entry.row.c;
            return ACTIVE.includes(lower(c.state)) || model.problemOf(c) ? 0 : 2;
        };
        return list.map((entry, index) => ({ entry, index, rank: rank(entry) })).sort((a, b) => a.rank - b.rank || a.index - b.index).map(item => item.entry);
    }

    // rows: the container rows of one section in display order. options.image switches the image grouping on (the single containers).
    // Returns [{type: 'row', row} ..., {type: 'image', key, image, rows, counts: {total, problem}, newest}]; newest is the largest
    // `created` (seconds) of the group. Longer than a page, the list is ranked (see ranked()).
    function entries(rows, options) {
        const list = array(rows), o = options || {};
        if (!o.image) return ranked(asRows(list));
        const model = window.DockerManagerModel, min = o.min > 1 ? o.min : MIN_GROUP, buckets = new Map();
        for (const row of list) {
            if (!row || !groupable(row.c)) continue;
            const key = imageKey(row.c.image);
            let bucket = buckets.get(key);
            if (!bucket) { bucket = { type: 'image', key, image: text(row.c.image), rows: [] }; buckets.set(key, bucket); }
            bucket.rows.push(row);
        }
        const groups = [...buckets.values()].filter(bucket => bucket.rows.length >= min);
        if (!groups.length) return ranked(asRows(list));
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
        return ranked(asRows(list.filter(row => !taken.has(row))).concat(groups));
    }

    const moreOf = (key, hidden) => (hidden > 0 ? { key, hidden, count: Math.min(PAGE, hidden) } : null);

    // Whether a list item is, or holds, the container with this key: a row, an entry of a section (a row or an image group) or a bare row.
    const holdsKey = (item, key) => (item.type === 'image' ? item.rows.some(row => row.key === key) : (item.type === 'row' ? item.row : item).key === key);

    // The rendered part of a plain list (stacks, external projects): {items, more}. more = {key, hidden, count} or null. `pin` is the
    // key of the container whose drawer is open: when it sits behind the cap it is shown after the page (the cap never hides the row the
    // admin is working on), and the hidden count leaves it out.
    function page(list, key, limit, pin) {
        const all = array(list), cap = limit(key), items = all.slice(0, cap);
        let pinned = 0;
        if (pin) {
            for (let index = cap; index < all.length; index += 1) {
                if (holdsKey(all[index], pin)) { items.push(all[index]); pinned = 1; break; }
            }
        }
        return { items, more: moreOf(key, all.length - cap - pinned) };
    }

    // While a search or a state filter is active the groups with matches open on their own, but they share one page of rows (the
    // section's own rows included), so "Stopped" on 60 images of 50 containers is not 3,000 rows. The groups with the most matches
    // open first (equal sizes keep the name order) while the budget lasts; the rest stay closed with their match counts. The walk
    // looks at sizes only (a group costs at most one page), so opening, closing or "show more" never moves another group.
    function autoOpen(items) {
        const open = new Set();
        let used = items.length;
        for (const group of items.filter(entry => entry.type === 'image').sort((a, b) => b.rows.length - a.rows.length)) {
            if (used >= PAGE) break;
            open.add(group.key);
            used += Math.min(group.rows.length, PAGE);
        }
        return open;
    }

    // The rendered part of a section made of entries(). view = state().view(...). An opened image group shows its first `limit`
    // members and a `more` of its own; a closed one none. Returns {items, more}: items are rows or
    // {type: 'image', group, open, members, more}.
    function plan(list, section, view) {
        const shown = page(list, section, view.limit, view.pinned), items = [], auto = view.narrowed ? autoOpen(shown.items) : null;
        for (const entry of shown.items) {
            if (entry.type !== 'image') { items.push(entry); continue; }
            const open = !!view.isOpen(entry.key, auto), members = open ? page(entry.rows, entry.key, view.limit, view.pinned) : { items: [], more: null };
            items.push({ type: 'image', group: entry, open, members: members.items, more: members.more });
        }
        return { items, more: shown.more };
    }

    // The state the app keeps. expanded: image groups the admin opened; closed and opened: what the admin closed or opened while a
    // search or filter is active; limits: key -> how many entries a section or group shows; narrowKey: what the section, search and
    // filter were when closed, opened and limits were last reset.
    function state() { return { expanded: new Set(), closed: new Set(), opened: new Set(), limits: new Map(), narrowKey: '' }; }

    // Call before every paint with the current section/search/filter. A change of it ends the admin's closings and openings during a
    // search and the "show more" steps.
    function sync(s, narrowKey) {
        if (s.narrowKey === narrowKey) return;
        s.narrowKey = narrowKey;
        s.closed.clear();
        s.opened.clear();
        s.limits.clear();
    }
    // pinned: the key of the container whose drawer is open (its row is always on the page while its group is open).
    function view(s, narrowed, pinned) {
        return {
            narrowed: !!narrowed,
            pinned: pinned || '',
            limit: key => s.limits.get(key) || PAGE,
            // auto: the keys autoOpen chose (narrowed only); the admin's choice beats it either way
            isOpen: (key, auto) => (narrowed ? !s.closed.has(key) && (s.opened.has(key) || !!(auto && auto.has(key))) : s.expanded.has(key))
        };
    }
    // openNow: whether the group shows open at the moment of the click (the budget decides that while a search is active).
    function toggle(s, narrowed, key, openNow) {
        if (!narrowed) {
            if (s.expanded.has(key)) s.expanded.delete(key); else s.expanded.add(key);
        } else if (openNow) {
            s.opened.delete(key);
            s.closed.add(key);
        } else {
            s.closed.delete(key);
            s.opened.add(key);
        }
    }
    function showMore(s, key) { s.limits.set(key, (s.limits.get(key) || PAGE) + PAGE); }

    // ---- finding a container in what is on screen ---------------------------------------------------------------------------
    // The sections of the view on screen as [{key, entries}]: the groups of the table, or the single containers of the Stacks list.
    function layout(snap) {
        if (snap.section === 'stacks') {
            const g = snap.sgroups || {};
            return [{ key: 'sec:singles', entries: g.singleEntries || asRows(g.singles) }];
        }
        return array(snap.groups).map(group => ({ key: group.key, entries: group.entries || asRows(group.rows) }));
    }
    // {section, group}: the section that lists the container and the image group it sits in ('' for a plain row), or null.
    function locate(sections, rowKey) {
        for (const section of array(sections)) {
            for (const entry of section.entries) {
                if (holdsKey(entry, rowKey)) return { section: section.key, group: entry.type === 'image' ? entry.key : '' };
            }
        }
        return null;
    }
    const holds = (sections, groupKey, rowKey) => { const at = locate(sections, rowKey); return !!at && at.group === groupKey; };
    // Opens the image group that holds the container (by the admin's own say, so no budget closes it); false when it sits in none.
    function reveal(s, narrowed, sections, rowKey) {
        const at = locate(sections, rowKey);
        if (!at || !at.group) return false;
        if (narrowed) { s.closed.delete(at.group); s.opened.add(at.group); } else s.expanded.add(at.group);
        return true;
    }

    window.DockerManagerGroups = { MIN_GROUP, PAGE, groupable, imageKey, entries, page, plan, state, sync, view, toggle, showMore, layout, locate, holds, reveal };
})();
