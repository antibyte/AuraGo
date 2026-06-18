(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.createRelics = function (ctx) {
        const RELICS = {
            swift_start: {
                name: 'Swift Start', desc: 'Begin with Rapid Fire for stage 1', cost: 50, col: '#00ffcc', icon: '>>',
                apply: function(G) { G.activePU = { type: 'rapid', timer: 8000 }; G.puTimer = 8000; }
            },
            thick_skin: {
                name: 'Thick Skin', desc: '+1 starting life', cost: 80, col: '#ff4444', icon: '+',
                apply: function(G) { G.lives++; }
            },
            lucky_drop: {
                name: 'Lucky Drop', desc: '+10% powerup drop chance', cost: 60, col: '#ffcc00', icon: '?',
                passive: true, dropBonus: 0.1
            },
            combo_keeper: {
                name: 'Combo Keeper', desc: 'Combo timer +500ms', cost: 40, col: '#ff88aa', icon: 'C',
                passive: true, comboBonus: 500
            },
            scrap_collector: {
                name: 'Scrap Collector', desc: '+50% credit gain', cost: 45, col: '#ffaa44', icon: '$',
                passive: true, creditMult: 1.5
            }
        };

        const STORAGE_KEY = 'galaxa_relics';
        const LOADOUT_KEY = 'galaxa_relic_loadout';
        const MAX_ACTIVE = 3;

        function loadRelics() { try { return JSON.parse(localStorage.getItem(STORAGE_KEY) || '{}'); } catch (e) { return {}; } }
        function saveRelics(data) { try { localStorage.setItem(STORAGE_KEY, JSON.stringify(data)); } catch (e) {} }
        function loadLoadout() { try { return JSON.parse(localStorage.getItem(LOADOUT_KEY) || '[]'); } catch (e) { return []; } }
        function saveLoadout(loadout) { try { localStorage.setItem(LOADOUT_KEY, JSON.stringify(loadout)); } catch (e) {} }
        function getShards() { try { return parseInt(localStorage.getItem('galaxa_relic_shards') || '0'); } catch (e) { return 0; } }
        function addShards(amount) { const current = getShards(); try { localStorage.setItem('galaxa_relic_shards', String(current + amount)); } catch (e) {} return current + amount; }
        function spendShards(amount) { const current = getShards(); if (current < amount) return false; try { localStorage.setItem('galaxa_relic_shards', String(current - amount)); } catch (e) {} return true; }
        function earnShards(score, stage) { return addShards(Math.floor(stage * 2 + score / 5000)); }

        function buyRelic(id) {
            const relic = RELICS[id]; if (!relic) return false;
            const owned = loadRelics(); if (owned[id]) return false;
            if (!spendShards(relic.cost)) return false;
            owned[id] = true; saveRelics(owned); return true;
        }

        function setActiveLoadout(ids) {
            const owned = loadRelics();
            const valid = ids.filter(id => owned[id]).slice(0, MAX_ACTIVE);
            saveLoadout(valid); return valid;
        }

        function applyRelics(G) {
            const loadout = loadLoadout();
            for (const id of loadout) { const relic = RELICS[id]; if (relic && relic.apply) relic.apply(G); }
        }

        function getRelicBonuses() {
            const loadout = loadLoadout();
            let dropBonus = 0, comboBonus = 0, creditMult = 1;
            for (const id of loadout) {
                const relic = RELICS[id]; if (!relic) continue;
                if (relic.dropBonus) dropBonus += relic.dropBonus;
                if (relic.comboBonus) comboBonus += relic.comboBonus;
                if (relic.creditMult) creditMult *= relic.creditMult;
            }
            return { dropBonus, comboBonus, creditMult };
        }

        function renderRelics() {
            const shards = getShards();
            const owned = loadRelics();
            const loadout = loadLoadout();
            let h = '<div class="galaxa-overlay-box"><h2>' + ctx.t('galaxa.relics', 'RELICS') + '</h2>';
            h += '<p style="color:#ffcc00">' + ctx.t('galaxa.shards', 'Shards') + ': ' + shards + '</p>';
            h += '<p style="color:#888;font-size:10px">' + ctx.t('galaxa.relic_loadout', 'Active loadout') + ': ' + loadout.length + '/' + MAX_ACTIVE + '</p>';
            h += '<div class="galaxa-relic-grid">';
            for (const [id, relic] of Object.entries(RELICS)) {
                const isOwned = owned[id];
                const isActive = loadout.includes(id);
                const cls = isActive ? 'galaxa-relic active' : isOwned ? 'galaxa-relic owned' : 'galaxa-relic locked';
                h += '<div class="' + cls + '" data-relic="' + id + '" style="border-color:' + relic.col + '">';
                h += '<div class="galaxa-relic-icon" style="color:' + relic.col + '">' + relic.icon + '</div>';
                h += '<div class="galaxa-relic-name">' + relic.name + '</div>';
                h += '<div class="galaxa-relic-desc">' + relic.desc + '</div>';
                if (!isOwned) h += '<div class="galaxa-relic-cost">' + relic.cost + ' shards</div>';
                h += '</div>';
            }
            h += '</div>';
            h += '<p style="font-size:10px;color:#666;margin-top:8px">' + ctx.t('galaxa.relic_hint', 'Click owned relics to add/remove from loadout') + '</p>';
            h += '<p style="font-size:10px;color:#666">' + ctx.t('galaxa.relic_hint2', 'Earn shards by completing runs') + '</p>';
            h += '<div class="galaxa-relic-actions">';
            h += '<button class="galaxa-btn" data-action="back">' + ctx.t('galaxa.back', 'BACK') + '</button>';
            h += '</div></div>';
            ctx.overlayEl.innerHTML = h;
            ctx.overlayEl.classList.add('active');

            ctx.overlayEl.querySelectorAll('[data-relic]').forEach(el => {
                el.addEventListener('click', () => {
                    const rid = el.dataset.relic;
                    const o = loadRelics();
                    if (o[rid]) {
                        const lo = loadLoadout();
                        if (lo.includes(rid)) { saveLoadout(lo.filter(x => x !== rid)); }
                        else if (lo.length < MAX_ACTIVE) { lo.push(rid); saveLoadout(lo); }
                        if (ctx.SFX) ctx.SFX.puCollect(ctx.W / 2);
                    } else {
                        if (buyRelic(rid)) { if (ctx.SFX) ctx.SFX.relicActivate(); }
                    }
                    renderRelics();
                });
            });
            const backBtn = ctx.overlayEl.querySelector('[data-action="back"]');
            if (backBtn) backBtn.addEventListener('click', () => { ctx.overlayEl.classList.remove('active'); ctx.overlayEl.innerHTML = ''; });
        }

        ctx.RELICS = RELICS;
        ctx.relic_loadRelics = loadRelics;
        ctx.relic_saveRelics = saveRelics;
        ctx.relic_loadLoadout = loadLoadout;
        ctx.relic_saveLoadout = saveLoadout;
        ctx.relic_getShards = getShards;
        ctx.relic_addShards = addShards;
        ctx.relic_spendShards = spendShards;
        ctx.relic_earnShards = earnShards;
        ctx.relic_buyRelic = buyRelic;
        ctx.relic_setActiveLoadout = setActiveLoadout;
        ctx.relic_applyRelics = applyRelics;
        ctx.relic_getRelicBonuses = getRelicBonuses;
        ctx.renderRelics = renderRelics;
    };
})();
