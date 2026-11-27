/* Runenpfad NPC trade: offers, reputation pricing, purchase effects and the kobold fake rule. DOM-free. */
(function (root) {
    'use strict';

    const RUNES = ['fire', 'frost', 'storm', 'magnet'];
    const FAKE_CHANCE = 0.35;
    const FAKE_REP = -2;
    const HAGGLE = 0.85;
    const UI_KEYS = ['trade_title', 'buy', 'back', 'leave', 'sold_out', 'too_poor', 'rep', 'coins', 'fake', 'price_up', 'price_down', 'owned'];

    function N() { return root.RunenpfadNpcs; }
    function P() { return root.RunenpfadPhysics; }

    function itemKeys(id) {
        return { nameKey: 'runenpfad.npc.item.' + id + '.name', descKey: 'runenpfad.npc.item.' + id + '.desc' };
    }

    function offer(id, base, icon, kind, stock, apply) {
        const k = itemKeys(id);
        return { id: id, base: base, nameKey: k.nameKey, descKey: k.descKey, icon: icon, kind: kind, apply: apply, stock: stock };
    }

    function stored(id) {
        return function (sim) { N().addItem(sim, id, 1); };
    }

    function runeOffer(kind) {
        return offer('rune_' + kind, 20, 'rune_' + kind, 'stage', 1, function (sim) {
            const p = sim.player;
            root.RunenpfadRunes.grant(sim, kind, p.x, p.y - 30);
        });
    }

    const OFFERS = {
        potion: offer('potion', 12, 'potion', 'stage', 2, function (sim) { sim.player.potions += 1; }),
        rune_fire: runeOffer('fire'),
        rune_frost: runeOffer('frost'),
        rune_storm: runeOffer('storm'),
        rune_magnet: runeOffer('magnet'),
        meal: offer('meal', 8, 'heart', 'stage', 5, function (sim) {
            const p = sim.player;
            p.hp = p.maxHp;
            sim.floatingTexts.push({ x: p.x, y: p.y - 44, text: 'FULL', icon: 'heart', color: '#f87171', timer: 1 });
        }),
        map_charm: offer('map_charm', 15, 'map_charm', 'stage', 1, function (sim) { sim.mapCharm = true; }),
        lucky_amulet: offer('lucky_amulet', 25, 'lucky_amulet', 'item', 1, stored('lucky_amulet')),
        discount_token: offer('discount_token', 30, 'discount_token', 'item', 1, stored('discount_token')),
        rune_attune: offer('rune_attune', 35, 'rune_attune', 'item', 1, stored('rune_attune')),
        // Round 11: kitchen ingredients (Tilda sells them; crafts.js turns pairs into meals).
        turnip: offer('turnip', 4, 'heart', 'item', 3, stored('turnip')),
        ham: offer('ham', 6, 'heart', 'item', 3, stored('ham')),
        frostberry: offer('frostberry', 6, 'heart', 'item', 3, stored('frostberry')),
        honey: offer('honey', 5, 'heart', 'item', 3, stored('honey'))
    };

    // Reputation -5..+5 moves the price by +30 % .. -30 %.
    function price(base, rep) {
        return Math.max(1, Math.round(base * (1 - 0.06 * (Number(rep) || 0))));
    }

    // The merchant's one rune kind per stage, derived from the stage id (no rng draw).
    function seededRune(sim) {
        return RUNES[(((sim.stage.id || 1) * 7) + 3) % RUNES.length];
    }

    function offerIds(sim, npc) {
        const def = N().NPCS[npc.npcId];
        if (!def) return [];
        return def.offers.map(function (id) { return id === 'rune_any' ? 'rune_' + seededRune(sim) : id; }).filter(function (id) { return OFFERS[id]; });
    }

    function priceOf(sim, npc, offerId) {
        const o = OFFERS[offerId];
        if (!o) return 0;
        const faction = N().NPCS[npc.npcId].faction;
        const rep = faction ? N().getRep(sim, faction) : 0;
        if (npc.npcId === 'merchant' && N().hasFlag(sim, 'merchant.haggle')) {
            return Math.max(1, Math.round(o.base * (1 - 0.06 * rep) * HAGGLE));
        }
        return price(o.base, rep);
    }

    function cappedOut(sim, o) {
        if (o.kind === 'item') return N().itemCount(sim, o.id) >= root.RunenpfadProfile.STORY_ITEMS[o.id];
        return o.id === 'map_charm' && !!sim.mapCharm;
    }

    function status(sim, npc, o) {
        const left = o.stock - (npc.sold[o.id] || 0);
        const cost = priceOf(sim, npc, o.id);
        let reason = null;
        if (left <= 0) reason = 'stock';
        else if (cappedOut(sim, o)) reason = 'cap';
        else if (sim.profile.coins < cost) reason = 'coins';
        return { id: o.id, price: cost, base: o.base, stock: Math.max(0, left), canBuy: reason === null, reason: reason };
    }

    function offersFor(sim, npc) {
        return offerIds(sim, npc).map(function (id) { return status(sim, npc, OFFERS[id]); });
    }

    function buy(sim, npc, offerId) {
        if (offerIds(sim, npc).indexOf(offerId) < 0) return { ok: false, reason: 'unknown' };
        const o = OFFERS[offerId];
        const st = status(sim, npc, o);
        if (!st.canBuy) return { ok: false, reason: st.reason };
        const n = N();
        const faction = n.NPCS[npc.npcId].faction;
        // Only a kobold merchant who dislikes the hero draws the rng for a fake.
        const fake = npc.npcId === 'merchant' && n.getRep(sim, faction) <= FAKE_REP && P().rand(sim) < FAKE_CHANCE;
        n.addCoins(sim, -st.price);
        npc.sold[offerId] = (npc.sold[offerId] || 0) + 1;
        if (fake) {
            n.setFlag(sim, 'merchant.fake_sold', 1);
            n.setFlag(sim, 'merchant.fake_price', Math.min(99, st.price));
        } else {
            o.apply(sim, npc);
        }
        sim.storyDirty = true;
        P().pushEvent(sim, 'npc_trade', { npcId: npc.npcId, offerId: offerId, fake: fake });
        return { ok: true, fake: fake };
    }

    // The fake is revealed when the dialogue closes (one label per stage).
    function onClose(sim, npc) {
        if (npc.npcId !== 'merchant' || npc.fakeRevealed || !N().hasFlag(sim, 'merchant.fake_sold')) return;
        npc.fakeRevealed = true;
        sim.floatingTexts.push({ x: sim.player.x, y: sim.player.y - 64, text: 'FAKE!', key: 'runenpfad.npc.float_fake', color: '#fca5a5', timer: 1.4 });
    }

    // Confront the merchant: reputation -1, half the price back, the flags are cleared.
    function confront(sim, npc) {
        const n = N();
        if (!n.hasFlag(sim, 'merchant.fake_sold')) return { ok: false };
        const refund = Math.floor(n.getFlag(sim, 'merchant.fake_price') / 2);
        n.addCoins(sim, refund);
        n.addRep(sim, n.NPCS[npc.npcId].faction, -1);
        n.setFlag(sim, 'merchant.fake_sold', 0);
        n.setFlag(sim, 'merchant.fake_price', 0);
        return { ok: true, refund: refund };
    }

    // Called once per stage by npcs.resolve: lucky amulet, expiring haggle, map charm reset.
    function applyStageStart(sim) {
        const n = N();
        sim.mapCharm = false;
        n.setFlag(sim, 'merchant.haggle', 0);
        n.setFlag(sim, 'merchant.haggled', 0);
        if (n.itemCount(sim, 'lucky_amulet') > 0) {
            n.addItem(sim, 'lucky_amulet', -1);
            sim.stage.coinValue = (sim.stage.coinValue || 1) + 1;
        }
    }

    function textKeys() {
        const out = [];
        const ids = Object.keys(OFFERS).concat(['glow_mushroom', 'sun_shard', 'gear_part', 'hex_charm', 'bell_bronze', 'dragon_scale', 'desk_key', 'room_key', 'clan_charm']);
        ids.forEach(function (id) {
            const k = itemKeys(id);
            out.push(k.nameKey, k.descKey);
        });
        UI_KEYS.forEach(function (k) { out.push('runenpfad.npc.ui.' + k); });
        out.push('runenpfad.npc.float_fake');
        return out;
    }

    root.RunenpfadTrade = {
        OFFERS: OFFERS,
        price: price,
        priceOf: priceOf,
        offersFor: offersFor,
        buy: buy,
        confront: confront,
        onClose: onClose,
        applyStageStart: applyStageStart,
        textKeys: textKeys
    };
})(typeof window !== 'undefined' ? window : globalThis);
