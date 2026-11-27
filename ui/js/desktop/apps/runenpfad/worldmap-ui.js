/* Runenpfad world map dialog (global `RunenpfadWorldmapUi.create`): world tabs, level tiles, the endless tower card,
 * the village entry and the story chronicle. Builds DOM with createElement/textContent only; app.js owns the game.
 * Options: { el, t, fill, Levels, hud, getProfile, getSim, onStart(opts), onReplay(sceneId), view }. */
(function (root) {
    'use strict';

    const VIEW_ENDLESS = 5;
    const VIEW_CHRONICLE = 6;

    function create(o) {
        const el = o.el;
        const t = o.t;
        const fill = o.fill;
        const Levels = o.Levels;
        const hud = o.hud;
        const select = function () { if (typeof o.play === 'function') o.play('ui_select'); }; // enabled controls only
        let view = Math.max(0, Math.min(VIEW_CHRONICLE, Math.floor(Number(o.view) || 0)));
        let readPage = 0; // journal page open in the chronicle tab (0 = none)
        let readTablet = 0; // stone tablet open below the journal (0 = none)

        const dialog = el('dialog', 'rp-dialog rp-levels-dialog');
        const head = el('div', 'rp-dialog-head');
        const title = el('h2');
        const closeBtn = el('button', 'rp-btn');
        closeBtn.type = 'button';
        head.append(title, closeBtn);
        const villageRow = el('div', 'rp-village-row');
        const villageBtn = el('button', 'rp-btn rp-btn-gold');
        villageBtn.type = 'button';
        villageBtn.dataset.rpVillage = '1';
        villageRow.append(villageBtn);
        const tabs = el('div', 'rp-tabs');
        const grid = el('div', 'rp-level-grid');
        dialog.append(head, villageRow, tabs, grid);

        villageBtn.addEventListener('click', function () {
            select();
            dialog.close();
            o.onStart({ mode: 'campaign', levelId: 0 });
        });

        function cutscenes() { return root.RunenpfadCutscenes || null; }
        // Opened from the hall's chronicle while inside a building: the map stays read-only (no stage switching).
        function sceneLocked() { return !!(o.isLocked && o.isLocked()); }

        function addTab(index, label, extraText, extraClass) {
            const tab = el('button', 'rp-tab' + (view === index ? ' is-active' : '') + (extraClass || ''), label);
            tab.type = 'button';
            if (extraText) tab.append(el('span', 'rp-tab-stars', extraText));
            tab.addEventListener('click', function () {
                select();
                view = index;
                render();
                tabs.children[index].focus({ preventScroll: true });
            });
            tabs.append(tab);
        }

        function renderEndless(profile) {
            const card = el('div', 'rp-shop-item');
            card.append(el('div', 'rp-shop-title', t('runenpfad.endless_title')));
            card.append(el('p', 'rp-shop-desc', fill('runenpfad.endless_best', { floor: profile.endlessBestFloor })));
            const startBtn = el('button', 'rp-btn-primary', t('runenpfad.endless_start'));
            startBtn.type = 'button';
            startBtn.disabled = sceneLocked();
            startBtn.addEventListener('click', function () {
                select();
                dialog.close();
                o.onStart({ mode: 'endless', endlessFloor: 1 });
            });
            card.append(startBtn);
            grid.append(card);
        }

        // The story chronicle: a seen scene can be replayed, the others stay sealed.
        function renderChronicle(profile) {
            const C = cutscenes();
            const box = el('div', 'rp-chron');
            (C ? C.chronicle(profile) : []).forEach(function (entry, i) {
                const row = el('div', 'rp-chron-row' + (entry.seen ? ' is-seen' : ''));
                row.append(el('span', 'rp-chron-no', String(i + 1)));
                if (entry.seen) {
                    row.append(el('strong', 'rp-chron-title', t(entry.titleKey)));
                    const btn = el('button', 'rp-btn', t('runenpfad.cut.replay'));
                    btn.type = 'button';
                    btn.dataset.rpReplay = entry.id;
                    btn.addEventListener('click', function () { select(); o.onReplay(entry.id); });
                    row.append(btn);
                } else {
                    row.append(el('span', 'rp-chron-title is-locked', t('runenpfad.cut.locked')));
                }
                box.append(row);
            });
            grid.append(box);
            if (root.RunenpfadChronicle) renderJournal(profile);
            const Chr = root.RunenpfadChronicle;
            const Emb = root.RunenpfadEmber;
            if (Chr && Chr.tabletList && (Chr.tabletCount(profile) > 0 || (Emb && Emb.step(profile) >= 1))) renderTablets(profile);
        }

        // Edda's journal: ten rows (title and a read button, or where to look), the open page's text below them.
        function renderJournal(profile) {
            const Chr = root.RunenpfadChronicle;
            const pages = Chr.list(profile, t) || [];
            const box = el('div', 'rp-journal');
            const have = typeof Chr.count === 'function' ? Chr.count(profile) : pages.filter(function (p) { return p.have; }).length;
            const headRow = el('div', 'rp-journal-head');
            headRow.append(el('h3', 'rp-journal-title', t('runenpfad.cut.journal')), el('span', 'rp-journal-count rp-num', fill('runenpfad.cut.pages_count', { count: have })));
            box.append(headRow);
            pages.forEach(function (p, i) {
                const n = p.n || i + 1;
                const row = el('div', 'rp-chron-row rp-journal-row' + (p.have ? ' is-seen' : '') + (readPage === n ? ' is-open' : ''));
                row.dataset.rpPage = String(n);
                row.append(el('span', 'rp-chron-no', String(n)));
                if (p.have) {
                    row.append(el('strong', 'rp-chron-title', p.title || t(p.titleKey || 'runenpfad.page.' + n + '.title')));
                    const btn = el('button', 'rp-btn', t('runenpfad.cut.page_read'));
                    btn.type = 'button';
                    btn.dataset.rpReadPage = String(n);
                    btn.addEventListener('click', function () {
                        if (typeof o.play === 'function') o.play('ui_page');
                        readPage = readPage === n ? 0 : n;
                        render();
                        const again = grid.querySelector('[data-rp-read-page="' + n + '"]');
                        if (again) again.focus({ preventScroll: true });
                    });
                    row.append(btn);
                } else {
                    row.append(el('span', 'rp-chron-title is-locked', fill('runenpfad.cut.page_missing', { place: p.placeId ? t('runenpfad.place.' + p.placeId) : '' })));
                }
                box.append(row);
            });
            const open = pages.filter(function (p, i) { return p.have && (p.n || i + 1) === readPage; })[0];
            if (open) {
                const reader = el('article', 'rp-journal-reader');
                reader.append(el('h4', 'rp-journal-page', open.title || t(open.titleKey || 'runenpfad.page.' + readPage + '.title')));
                ['a', 'b'].forEach(function (part) { reader.append(el('p', 'rp-journal-text', t('runenpfad.npc.journal.p' + readPage + part))); });
                box.append(reader);
            }
            grid.append(box);
        }

        // The stone tablets of the ember chapter: six rows (title and a read button, or where to look), the open text below.
        function renderTablets(profile) {
            const Chr = root.RunenpfadChronicle;
            const tabs = Chr.tabletList(profile, t) || [];
            const box = el('div', 'rp-journal rp-tablets');
            const headRow = el('div', 'rp-journal-head');
            headRow.append(el('h3', 'rp-journal-title', t('runenpfad.tablet.title')), el('span', 'rp-journal-count rp-num', fill('runenpfad.tablet.count', { count: Chr.tabletCount(profile) })));
            box.append(headRow);
            tabs.forEach(function (tb) {
                const n = tb.n;
                const row = el('div', 'rp-chron-row rp-journal-row' + (tb.have ? ' is-seen' : '') + (readTablet === n ? ' is-open' : ''));
                row.dataset.rpTablet = String(n);
                row.append(el('span', 'rp-chron-no', String(n)));
                if (tb.have) {
                    row.append(el('strong', 'rp-chron-title', tb.title));
                    const btn = el('button', 'rp-btn', t('runenpfad.tablet.read'));
                    btn.type = 'button';
                    btn.dataset.rpReadTablet = String(n);
                    btn.addEventListener('click', function () {
                        if (typeof o.play === 'function') o.play('ui_page');
                        readTablet = readTablet === n ? 0 : n;
                        render();
                        const again = grid.querySelector('[data-rp-read-tablet="' + n + '"]');
                        if (again) again.focus({ preventScroll: true });
                    });
                    row.append(btn);
                } else {
                    row.append(el('span', 'rp-chron-title is-locked', fill('runenpfad.tablet.missing', { place: t(tb.placeKey) })));
                }
                box.append(row);
            });
            const open = tabs.filter(function (tb) { return tb.have && tb.n === readTablet; })[0];
            if (open) {
                const reader = el('article', 'rp-journal-reader');
                reader.append(el('h4', 'rp-journal-page', open.title), el('p', 'rp-journal-text', t('runenpfad.npc.lore.tab_' + open.n)));
                box.append(reader);
            }
            grid.append(box);
        }

        function renderTiles(profile) {
            const sim = o.getSim();
            const startIdx = view * 5;
            Levels.CAMPAIGN_LEVELS.slice(startIdx, startIdx + 5).forEach(function (lvl) {
                const locked = lvl.id > profile.unlockedLevel;
                const stars = profile.starsByLevel[lvl.id] || 0;
                const isCurrent = sim && sim.mode === 'campaign' && sim.levelId === lvl.id;
                const btn = el('button', 'rp-level-tile' + (isCurrent ? ' is-current' : '') + (lvl.boss ? ' is-boss' : ''));
                btn.type = 'button';
                btn.disabled = locked || sceneLocked();
                btn.dataset.rpLevelTile = String(lvl.id);
                btn.append(el('span', 'rp-level-no', (view + 1) + '-' + lvl.stageInWorld));
                btn.append(el('strong', '', t(lvl.nameKey)));
                if (lvl.boss) btn.append(el('span', 'rp-level-boss', t('runenpfad.boss_intro_sub')));
                const meta = el('span', 'rp-level-meta');
                if (locked) {
                    meta.textContent = t('runenpfad.level_locked');
                } else {
                    const best = profile.bestTimeByLevel[lvl.id];
                    const row = el('span', 'rp-level-stars');
                    hud.paintStars(row, stars, 14);
                    meta.append(row);
                    if (best) meta.append(el('span', 'rp-num', hud.formatTime(best)));
                }
                btn.append(meta);
                btn.addEventListener('click', function () {
                    select();
                    dialog.close();
                    o.onStart({ mode: 'campaign', levelId: lvl.id });
                });
                grid.append(btn);
            });
        }

        function render() {
            const profile = o.getProfile();
            villageBtn.textContent = t('runenpfad.village.enter');
            villageBtn.disabled = sceneLocked();
            villageRow.hidden = !root.RunenpfadVillage;
            tabs.replaceChildren();
            Levels.WORLDS.forEach(function (w, idx) {
                // Each world tab carries its star count; worlds that are still sealed stay dimmed.
                let stars = 0;
                for (let id = idx * 5 + 1; id <= idx * 5 + 5; id++) stars += profile.starsByLevel[id] || 0;
                const sealed = idx * 5 + 1 > profile.unlockedLevel;
                addTab(idx, (idx + 1) + '. ' + t(w.nameKey), sealed ? t('runenpfad.level_locked') : '★ ' + stars + '/15', sealed ? ' is-sealed' : '');
            });
            addTab(VIEW_ENDLESS, t('runenpfad.endless_title'), profile.endlessBestFloor > 0 ? fill('runenpfad.endless_stage', { floor: profile.endlessBestFloor }) : '');
            if (cutscenes()) addTab(VIEW_CHRONICLE, t('runenpfad.cut.chronicle'), '');

            grid.replaceChildren();
            if (view !== VIEW_CHRONICLE) { readPage = 0; readTablet = 0; }
            if (view === VIEW_ENDLESS) renderEndless(profile);
            else if (view === VIEW_CHRONICLE && cutscenes()) renderChronicle(profile);
            else renderTiles(profile);
        }

        function paintCopy() {
            title.textContent = t('runenpfad.levels_title');
            hud.setButton(closeBtn, t('runenpfad.close'), 'Esc');
        }

        function focusFirst() {
            const b = grid.querySelector('.is-current:not(:disabled)') || grid.querySelector('button:not(:disabled)') || tabs.querySelector('.is-active');
            if (b) b.focus({ preventScroll: true });
        }

        // Arrow keys walk the tabs and the enabled buttons of the grid.
        dialog.addEventListener('keydown', function (ev) {
            const dir = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -1, ArrowDown: 1 }[ev.key];
            if (!dir) return;
            const tabList = Array.from(tabs.children);
            const tiles = Array.from(grid.querySelectorAll('button:not(:disabled)'));
            const cur = root.document.activeElement;
            const horizontal = ev.key === 'ArrowLeft' || ev.key === 'ArrowRight';
            if (tabList.includes(cur)) {
                ev.preventDefault();
                if (horizontal) tabList[(tabList.indexOf(cur) + dir + tabList.length) % tabList.length].focus();
                else if (dir > 0 && tiles.length) tiles[0].focus();
            } else if (tiles.includes(cur)) {
                ev.preventDefault();
                const i = tiles.indexOf(cur);
                if (horizontal) tiles[(i + dir + tiles.length) % tiles.length].focus();
                else if (dir < 0 && i === 0) (tabs.querySelector('.is-active') || tabList[0]).focus();
                else tiles[Math.max(0, Math.min(tiles.length - 1, i + dir))].focus();
            }
        });

        return {
            dialog: dialog, closeBtn: closeBtn, render: render, paintCopy: paintCopy, focusFirst: focusFirst,
            setView: function (n) { view = Math.max(0, Math.min(VIEW_CHRONICLE, Math.floor(Number(n) || 0))); },
            VIEW_CHRONICLE: VIEW_CHRONICLE
        };
    }

    root.RunenpfadWorldmapUi = { create: create, VIEW_ENDLESS: VIEW_ENDLESS, VIEW_CHRONICLE: VIEW_CHRONICLE };
})(typeof window !== 'undefined' ? window : globalThis);
