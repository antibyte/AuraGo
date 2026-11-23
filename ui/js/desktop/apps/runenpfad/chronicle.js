/* Runenpfad chronicle of Edda: the ten journal pages (loot ids page_01 ... page_10) and the observatory chapter state.
 * DOM-free (global `RunenpfadChronicle`): no document, no window, no canvas, no random numbers, no clock.
 * Pages are plain loot ids in profile.loot; the chapter lives in the npcFlags `obs.open` and `obs.done`.
 * Texts live in the locale files (`runenpfad.page.<n>.title`, `runenpfad.place.<id>`, `runenpfad.notice.page_found`). */
(function (root) {
    'use strict';

    const LOOT_CAP = 48;
    const TOTAL = 10;
    const NOTICE = 'runenpfad.notice.page_found';

    // Place and campaign level where each page lies (contract section 3).
    const WHERE = [['hermit_hut', 1], ['beast_den', 3], ['kobold_den', 6], ['crystal_grotto', 9], ['nomad_tent', 11],
        ['sun_tomb', 14], ['summit_hut', 16], ['ice_cave', 19], ['brass_lab', 22], ['boiler_room', 24]];

    const PAGES = WHERE.map(function (w, i) {
        const n = i + 1;
        return { n: n, loot: lootId(n), placeId: w[0], level: w[1], titleKey: 'runenpfad.page.' + n + '.title' };
    });

    function lootId(n) { return 'page_' + (n < 10 ? '0' : '') + n; }

    function lootOf(profile) {
        return profile && Array.isArray(profile.loot) ? profile.loot : [];
    }

    function validPage(n) { return Number.isInteger(n) && n >= 1 && n <= TOTAL; }

    function has(profile, n) {
        return validPage(n) && lootOf(profile).indexOf(lootId(n)) >= 0;
    }

    function count(profile) {
        let c = 0;
        for (let n = 1; n <= TOTAL; n++) if (has(profile, n)) c += 1;
        return c;
    }

    // Returns true when the page was new. Endless mode never finds pages; a full loot list (48) refuses a new page.
    function give(sim, n) {
        if (!sim || !sim.profile || sim.mode === 'endless' || !validPage(n)) return false;
        const pr = sim.profile;
        if (has(pr, n)) return false;
        const list = pr.loot || (pr.loot = []);
        if (list.length >= LOOT_CAP) return false;
        list.push(lootId(n));
        list.sort();
        sim.storyDirty = true;
        const c = count(pr);
        if (root.RunenpfadPhysics) root.RunenpfadPhysics.pushEvent(sim, 'in_page', { page: n, first: true });
        sim.notice = NOTICE;
        const p = sim.player;
        if (p && Array.isArray(sim.floatingTexts)) {
            sim.floatingTexts.push({ x: p.x, y: p.y - 64, text: 'PAGE', key: NOTICE, suffix: ' ' + c + '/' + TOTAL, color: '#ffe9a8', timer: 1.4 });
        }
        return true;
    }

    // The ten pages with `have`; `t` (optional translator) fills `title` and `place` with the translated texts.
    function list(profile, t) {
        const tr = typeof t === 'function' ? t : function (k) { return k; };
        return PAGES.map(function (pg) {
            const placeKey = 'runenpfad.place.' + pg.placeId;
            return {
                n: pg.n, loot: pg.loot, placeId: pg.placeId, level: pg.level, titleKey: pg.titleKey, placeKey: placeKey,
                title: tr(pg.titleKey), place: tr(placeKey), have: has(profile, pg.n)
            };
        });
    }

    function flagOf(profile, key) {
        const f = profile && profile.npcFlags;
        return f && f[key] > 0 ? f[key] : 0;
    }

    function chapter(profile) {
        if (flagOf(profile, 'obs.done')) return 'done';
        return flagOf(profile, 'obs.open') ? 'open' : 'closed';
    }

    root.RunenpfadChronicle = {
        TOTAL: TOTAL,
        PAGES: PAGES,
        lootId: lootId,
        has: has,
        count: count,
        give: give,
        list: list,
        chapter: chapter
    };
})(typeof window !== 'undefined' ? window : globalThis);
