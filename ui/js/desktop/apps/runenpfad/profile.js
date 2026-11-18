/* Runenpfad persistent profile: schema, clamping, workshop upgrades and hero stats. DOM-free. */
(function (root) {
    'use strict';

    const STORAGE_KEY = 'aurago.runenpfad.v1';
    const SAVE_VERSION = 1;

    const FACTIONS = ['coven', 'circle', 'kobolds'];
    const STORY_ITEMS = { glow_mushroom: 9, sun_shard: 9, gear_part: 9, hex_charm: 1, lucky_amulet: 3, discount_token: 3, rune_attune: 2 };
    const QUEST_IDS = ['witch_mushrooms', 'seer_shards', 'alchemist_gears'];
    const FLAG_RE = /^[a-z][a-z0-9_]{0,15}\.[a-z0-9_]{1,23}$/;

    function levelsApi() {
        return root.RunenpfadLevels;
    }

    function createDefaultProfile() {
        return {
            v: SAVE_VERSION,
            hero: 'knight',
            sound: true,
            music: true,
            gfx: 'auto',
            unlockedLevel: 1,
            endlessBestFloor: 0,
            coins: 25,
            totalScore: 0,
            upgrades: {
                vitality: 1,
                might: 1,
                boots: 1,
                alchemy: 1
            },
            starsByLevel: {},
            bestTimeByLevel: {},
            rep: { coven: 0, circle: 0, kobolds: 0 },
            npcFlags: {},
            inventory: {},
            quests: { active: [], done: [] }
        };
    }

    function clampInt(val, min, max, fallback) {
        const n = Number(val);
        if (!Number.isFinite(n)) return fallback;
        return Math.min(max, Math.max(min, Math.floor(n)));
    }

    function isPlainObject(v) {
        return !!v && typeof v === 'object' && !Array.isArray(v);
    }

    function importStory(raw) {
        const rep = {};
        FACTIONS.forEach(function (f) {
            rep[f] = clampInt(isPlainObject(raw.rep) ? raw.rep[f] : 0, -5, 5, 0);
        });
        const npcFlags = {};
        if (isPlainObject(raw.npcFlags)) {
            Object.keys(raw.npcFlags).filter(function (k) { return FLAG_RE.test(k); }).sort().forEach(function (k) {
                const n = clampInt(raw.npcFlags[k], 0, 99, 0);
                if (n > 0 && Object.keys(npcFlags).length < 64) npcFlags[k] = n;
            });
        }
        const inventory = {};
        if (isPlainObject(raw.inventory)) {
            Object.keys(STORY_ITEMS).forEach(function (id) {
                if (!Object.prototype.hasOwnProperty.call(raw.inventory, id)) return;
                const n = clampInt(raw.inventory[id], 0, STORY_ITEMS[id], 0);
                if (n > 0) inventory[id] = n;
            });
        }
        const quests = { active: [], done: [] };
        if (isPlainObject(raw.quests)) {
            const list = function (v, cap, skip) {
                const out = [];
                if (Array.isArray(v)) {
                    v.forEach(function (q) {
                        if (QUEST_IDS.indexOf(q) >= 0 && out.indexOf(q) < 0 && skip.indexOf(q) < 0 && out.length < cap) out.push(q);
                    });
                }
                return out;
            };
            quests.done = list(raw.quests.done, 16, []);
            quests.active = list(raw.quests.active, 8, quests.done);
        }
        return { rep: rep, npcFlags: npcFlags, inventory: inventory, quests: quests };
    }

    function importProfile(raw) {
        if (!raw || typeof raw !== 'object' || Array.isArray(raw) || raw.v !== SAVE_VERSION) {
            return null;
        }
        const levels = levelsApi();
        const def = createDefaultProfile();
        const hero = raw.hero === 'ranger' ? 'ranger' : 'knight';
        const upgrades = {};
        levels.UPGRADE_DEFS.forEach(function (track) {
            upgrades[track.id] = clampInt(raw.upgrades && raw.upgrades[track.id], 1, track.maxLevel, 1);
        });
        const starsByLevel = {};
        if (raw.starsByLevel && typeof raw.starsByLevel === 'object' && !Array.isArray(raw.starsByLevel)) {
            Object.keys(raw.starsByLevel).forEach(function (k) {
                const lvl = Number(k);
                if (Number.isInteger(lvl) && lvl >= 1 && lvl <= levels.CAMPAIGN_LEVELS.length) {
                    starsByLevel[lvl] = clampInt(raw.starsByLevel[k], 0, 3, 0);
                }
            });
        }
        const bestTimeByLevel = {};
        if (raw.bestTimeByLevel && typeof raw.bestTimeByLevel === 'object' && !Array.isArray(raw.bestTimeByLevel)) {
            Object.keys(raw.bestTimeByLevel).forEach(function (k) {
                const lvl = Number(k);
                const t = Number(raw.bestTimeByLevel[k]);
                if (Number.isInteger(lvl) && lvl >= 1 && lvl <= levels.CAMPAIGN_LEVELS.length && Number.isFinite(t) && t > 0) {
                    bestTimeByLevel[lvl] = Math.round(t * 10) / 10;
                }
            });
        }
        const story = importStory(raw);
        return {
            v: SAVE_VERSION,
            hero: hero,
            sound: raw.sound !== false,
            music: raw.music !== false,
            gfx: raw.gfx === 'canvas' ? 'canvas' : 'auto',
            unlockedLevel: clampInt(raw.unlockedLevel, 1, levels.CAMPAIGN_LEVELS.length, def.unlockedLevel),
            endlessBestFloor: clampInt(raw.endlessBestFloor, 0, 9999, 0),
            coins: clampInt(raw.coins, 0, 9999999, 0),
            totalScore: clampInt(raw.totalScore, 0, 99999999, 0),
            upgrades: upgrades,
            starsByLevel: starsByLevel,
            bestTimeByLevel: bestTimeByLevel,
            rep: story.rep,
            npcFlags: story.npcFlags,
            inventory: story.inventory,
            quests: story.quests
        };
    }

    function exportProfile(profile) {
        return importProfile(profile) || createDefaultProfile();
    }

    function hasDiscount(profile) {
        return !!(profile && profile.inventory && profile.inventory.discount_token > 0);
    }

    function upgradeCost(trackId, currentLevel, profile) {
        const track = levelsApi().UPGRADE_DEFS.find(function (u) { return u.id === trackId; });
        if (!track || currentLevel >= track.maxLevel) return null;
        const cost = track.baseCost + (currentLevel - 1) * track.stepCost;
        return hasDiscount(profile) ? Math.ceil(cost * 0.7) : cost;
    }

    function buyUpgrade(profile, trackId) {
        const clean = exportProfile(profile);
        const track = levelsApi().UPGRADE_DEFS.find(function (u) { return u.id === trackId; });
        if (!track) return { ok: false, reason: 'unknown', profile: clean, cost: 0 };
        const lvl = clean.upgrades[trackId] || 1;
        if (lvl >= track.maxLevel) return { ok: false, reason: 'max', profile: clean, cost: 0 };
        const cost = upgradeCost(trackId, lvl, clean);
        if (clean.coins < cost) return { ok: false, reason: 'coins', profile: clean, cost: cost };
        clean.coins -= cost;
        if (hasDiscount(clean)) {
            clean.inventory.discount_token -= 1;
            if (clean.inventory.discount_token <= 0) delete clean.inventory.discount_token;
        }
        clean.upgrades[trackId] = lvl + 1;
        return { ok: true, profile: clean, cost: cost };
    }

    function totalStars(profile) {
        if (!profile || !profile.starsByLevel) return 0;
        let sum = 0;
        Object.keys(profile.starsByLevel).forEach(function (k) {
            sum += Number(profile.starsByLevel[k]) || 0;
        });
        return sum;
    }

    function heroStats(heroId, profile) {
        const levels = levelsApi();
        const def = levels.HERO_DEFS[heroId] || levels.HERO_DEFS.knight;
        const upg = (profile && profile.upgrades) || {};
        const vit = clampInt(upg.vitality, 1, 5, 1);
        const might = clampInt(upg.might, 1, 5, 1);
        const boots = clampInt(upg.boots, 1, 5, 1);
        const alchemy = clampInt(upg.alchemy, 1, 5, 1);
        return {
            def: def,
            maxHp: def.baseHp + (vit - 1),
            damage: def.baseDamage + Math.floor(might / 2),
            speed: def.speed + (boots - 1) * 14,
            jumpVel: def.jumpVel + (boots - 1) * 16,
            maxAirJumps: def.maxAirJumps + (boots >= 4 ? 1 : 0),
            potions: 1 + Math.floor(alchemy / 2),
            magnetRadius: 28 + (alchemy - 1) * 18
        };
    }

    root.RunenpfadProfile = {
        STORAGE_KEY: STORAGE_KEY,
        SAVE_VERSION: SAVE_VERSION,
        FACTIONS: FACTIONS,
        STORY_ITEMS: STORY_ITEMS,
        QUEST_IDS: QUEST_IDS,
        FLAG_RE: FLAG_RE,
        createDefaultProfile: createDefaultProfile,
        importProfile: importProfile,
        exportProfile: exportProfile,
        upgradeCost: upgradeCost,
        buyUpgrade: buyUpgrade,
        totalStars: totalStars,
        heroStats: heroStats
    };
})(typeof window !== 'undefined' ? window : globalThis);
