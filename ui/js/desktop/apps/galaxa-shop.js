(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.SHOP_ITEMS = [
        { id: 'extra_life', name: 'Extra Life', desc: '+1 life', cost: 100, icon: '\u2764', apply: function(ctx) { ctx.G.lives++; } },
        { id: 'shield_hit', name: 'Shield Charge', desc: '+1 shield hit', cost: 80, icon: '\uD83D\uDEE1', apply: function(ctx) { ctx.G.shieldHits = Math.min(5, ctx.G.shieldHits + 1); } },
        { id: 'weapon_up', name: 'Weapon Boost', desc: 'Weapon level +1', cost: 300, icon: '\u2B50', apply: function(ctx) { ctx.G.weaponLv = Math.min(4, ctx.G.weaponLv + 1); }, maxBuy: 3 },
        { id: 'score_mult', name: 'Score Booster', desc: 'x1.5 score next stage', cost: 150, icon: '\u2716', apply: function(ctx) { ctx.G.scoreMult = Math.max(ctx.G.scoreMult, 1.5); } },
        { id: 'start_rapid', name: 'Rapid Fire', desc: 'Start with Rapid Fire', cost: 50, icon: '\uD83D\uDD25', puType: 'rapid' },
        { id: 'start_spread', name: 'Spread Shot', desc: 'Start with Spread Shot', cost: 60, icon: '\u2734', puType: 'spread' },
        { id: 'start_shield', name: 'Shield', desc: 'Start with Shield (3 hits)', cost: 80, icon: '\uD83D\uDEE1', puType: 'shield' },
        { id: 'start_pierce', name: 'Pierce', desc: 'Start with Pierce', cost: 70, icon: '\uD83C\uDFAF', puType: 'pierce' },
        { id: 'start_homing', name: 'Homing Missiles', desc: '5 homing missiles', cost: 120, icon: '\uD83D\uDE80', puType: 'homing' },
        { id: 'start_freeze', name: 'Freeze', desc: 'Start with Freeze', cost: 200, icon: '\u2744', puType: 'freeze' }
    ];

    GC.createShop = function (ctx) {
        let shopSel = 0;
        let shopItems = [];
        let shopBought = {};
        let shopVisits = 0;
        let purchasedPowerups = [];

        function openShop() {
            shopSel = 0;
            shopBought = {};
            purchasedPowerups = [];
            shopItems = GC.SHOP_ITEMS.filter(function(item) {
                if (item.id === 'weapon_up' && ctx.G.weaponLv >= 4) return false;
                return true;
            });
            ctx.G.shopOpen = true;
            ctx.G.st = 'SHOP';
            ctx.MusicEngine.play('shop');
            shopVisits++;
            if (shopVisits >= 10) ctx.unlockAchievement('shopaholic');
            ctx.SFX.coinInsert();
        }

        function closeShop() {
            ctx.G.shopOpen = false;
            ctx.startStage();
            for (const type of purchasedPowerups) ctx.collectPU({ type, x: ctx.G.p.x, y: ctx.G.p.y });
            purchasedPowerups = [];
        }

        function buyItem(item) {
            const bought = shopBought[item.id] || 0;
            if (item.maxBuy && bought >= item.maxBuy) return false;
            if (ctx.G.credits < item.cost) return false;
            ctx.G.credits -= item.cost;
            shopBought[item.id] = bought + 1;
            try { localStorage.setItem('galaxa_credits', String(ctx.G.credits)); } catch (e) {}
            if (item.apply) {
                item.apply(ctx);
                if (item.id === 'score_mult') ctx.G.nextScoreMult = 1.5;
            } else if (item.puType) {
                purchasedPowerups.push(item.puType);
            }
            ctx.SFX.shopBuy();
            if (ctx.fxPowerupSparkle) ctx.fxPowerupSparkle(ctx.W / 2, ctx.H / 2, '#ffcc00', 'uncommon');
            for (let i = 0; i < 10; i++) {
                const a = (i / 10) * Math.PI * 2;
                ctx.G.part.push({ x: ctx.W / 2, y: ctx.H / 2, vx: Math.cos(a) * 50, vy: Math.sin(a) * 50, life: 300, t: 0, col: '#ffcc00', size: 2, spark: true });
            }
            return true;
        }

        function updateShop() {
            if (ctx.G.inp.p && !ctx.G.inp.pp) { closeShop(); return; }
            const u = ctx.G.inp.u && !ctx.G.inp.up;
            const d = ctx.G.inp.d && !ctx.G.inp.dp;
            const f = ctx.G.inp.f && !ctx.G.inp.fp;
            if (u) shopSel = Math.max(0, shopSel - 1);
            if (d) shopSel = Math.min(shopItems.length, shopSel + 1);
            if (f) {
                if (shopSel >= shopItems.length) {
                    closeShop();
                    return;
                }
                buyItem(shopItems[shopSel]);
            }
        }

        function renderShop() {
            const c = ctx.c, W = ctx.W, H = ctx.H;
            c.fillStyle = '#071222ed'; c.fillRect(18, 22, W - 36, H - 44);
            function text(value, x, y, size, color, align) {
                c.textAlign = align || 'left'; c.fillStyle = color; c.font = size + 'px "Share Tech Mono", monospace';
                c.fillText(value, x, y, align === 'center' ? W - 60 : W - 160);
            }
            text(ctx.t('galaxa.shop'), W / 2, 60, 24, '#e7d2a6', 'center');
            text(ctx.t('galaxa.credits') + '  ' + ctx.G.credits, W / 2, 87, 14, '#8bbac4', 'center');
            const names = { extra_life: 'lives', shield_hit: 'shield', weapon_up: null, score_mult: 'score',
                start_rapid: 'rapid_fire', start_spread: 'spread_shot', start_shield: 'shield', start_pierce: 'pierce', start_homing: 'homing', start_freeze: 'freeze' };
            const symbols = { extra_life: '+1', shield_hit: '+1', weapon_up: 'W +1', score_mult: '×1.5' };
            for (let i = 0; i < shopItems.length; i++) {
                const item = shopItems[i], y = 104 + i * 44, maxed = item.maxBuy && (shopBought[item.id] || 0) >= item.maxBuy;
                c.fillStyle = i === shopSel ? '#233a53' : '#0e1d30'; c.fillRect(32, y, W - 64, 40);
                if (i === shopSel) { c.fillStyle = '#e7c286'; c.fillRect(32, y, 3, 40); }
                ctx.drawAnimation(c, 'pickup.' + (item.puType || (item.id === 'shield_hit' ? 'shield' : 'speed')), 54, y + 20, ctx.G.animTime, 26);
                const name = names[item.id] ? ctx.t('galaxa.' + names[item.id]) : '';
                text((symbols[item.id] || '') + ' ' + name, 78, y + 25, 13, '#cedfec');
                text(maxed ? ctx.t('galaxa.owned') : String(item.cost), W - 48, y + 25, 13, ctx.G.credits >= item.cost ? '#e7c286' : '#bd8080', 'right');
            }
            const y = 104 + shopItems.length * 44;
            c.fillStyle = shopSel === shopItems.length ? '#37516a' : '#152b42'; c.fillRect(90, y + 6, W - 180, 43);
            text(ctx.t('galaxa.leave_shop'), W / 2, y + 34, 16, '#e7c286', 'center');
            text(ctx.t('galaxa.shop_hint'), W / 2, H - 44, 11, '#90aac3', 'center');
        }

        ctx.shopClick = index => { shopSel = Math.max(0, Math.min(shopItems.length, index)); if (shopSel === shopItems.length) closeShop(); else buyItem(shopItems[shopSel]); };
        ctx.openShop = openShop;
        ctx.closeShop = closeShop;
        ctx.updateShop = updateShop;
        ctx.renderShop = renderShop;
        ctx.shopItemCount = function() { return shopItems.length; };
    };
})();
