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

        function openShop() {
            shopSel = 0;
            shopBought = {};
            shopItems = GC.SHOP_ITEMS.filter(function(item) {
                if (item.id === 'weapon_up' && ctx.G.weaponLv >= 4) return false;
                return true;
            });
            ctx.G.shopOpen = true;
            ctx.G.st = 'SHOP';
            shopVisits++;
            if (shopVisits >= 10) ctx.unlockAchievement('shopaholic');
            ctx.SFX.coinInsert();
        }

        function closeShop() {
            ctx.G.shopOpen = false;
            ctx.G.st = 'STAGE_INTRO';
            ctx.G.introTmr = 1200;
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
            } else if (item.puType) {
                ctx.G.powerups.push({ x: ctx.G.p.x, y: ctx.G.p.y - 30, type: item.puType, t: 0 });
            }
            ctx.SFX.puCollect(ctx.W / 2);
            for (let i = 0; i < 10; i++) {
                const a = (i / 10) * Math.PI * 2;
                ctx.G.part.push({ x: ctx.W / 2, y: ctx.H / 2, vx: Math.cos(a) * 50, vy: Math.sin(a) * 50, life: 300, t: 0, col: '#ffcc00', size: 2, spark: true });
            }
            return true;
        }

        function updateShop() {
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
            ctx.c.fillStyle = 'rgba(0,0,0,0.88)';
            ctx.c.fillRect(0, 0, ctx.W, ctx.H);

            ctx.c.textAlign = 'center';
            ctx.c.fillStyle = '#ffcc00';
            ctx.c.font = 'bold 22px "Courier New",monospace';
            ctx.c.shadowBlur = 10;
            ctx.c.shadowColor = '#ffcc00';
            ctx.c.fillText('SHOP', ctx.W / 2, 50);
            ctx.c.shadowBlur = 0;

            ctx.c.fillStyle = '#44ff88';
            ctx.c.font = 'bold 14px "Courier New",monospace';
            ctx.c.fillText('CREDITS: ' + ctx.G.credits, ctx.W / 2, 80);

            const startY = 110;
            const itemH = 44;
            for (let i = 0; i < shopItems.length; i++) {
                const item = shopItems[i];
                const y = startY + i * itemH;
                const sel = i === shopSel;
                const bought = shopBought[item.id] || 0;
                const maxed = item.maxBuy && bought >= item.maxBuy;
                const canAfford = ctx.G.credits >= item.cost;

                if (sel) {
                    ctx.c.fillStyle = 'rgba(68,136,255,0.15)';
                    ctx.c.fillRect(20, y - 14, ctx.W - 40, itemH - 4);
                    ctx.c.strokeStyle = '#4488ff';
                    ctx.c.lineWidth = 1;
                    ctx.c.strokeRect(20, y - 14, ctx.W - 40, itemH - 4);
                }

                ctx.c.textAlign = 'left';
                ctx.c.fillStyle = sel ? '#ffcc00' : (canAfford && !maxed ? '#aaccee' : '#555');
                ctx.c.font = sel ? 'bold 12px "Courier New",monospace' : '11px "Courier New",monospace';
                ctx.c.fillText(item.icon + ' ' + item.name, 32, y);

                ctx.c.fillStyle = maxed ? '#448844' : (canAfford ? '#44ff88' : '#ff4444');
                ctx.c.font = '10px "Courier New",monospace';
                ctx.c.fillText(item.desc, 32, y + 14);

                ctx.c.textAlign = 'right';
                if (maxed) {
                    ctx.c.fillStyle = '#448844';
                    ctx.c.fillText('OWNED', ctx.W - 32, y + 4);
                } else {
                    ctx.c.fillStyle = canAfford ? '#ffcc00' : '#ff4444';
                    ctx.c.font = 'bold 11px "Courier New",monospace';
                    ctx.c.fillText(item.cost + ' CR', ctx.W - 32, y + 4);
                }
            }

            const leaveY = startY + shopItems.length * itemH;
            const selLeave = shopSel === shopItems.length;
            ctx.c.textAlign = 'center';
            ctx.c.fillStyle = selLeave ? '#ff4444' : '#888';
            ctx.c.font = selLeave ? 'bold 14px "Courier New",monospace' : '12px "Courier New",monospace';
            if (selLeave) { ctx.c.shadowBlur = 6; ctx.c.shadowColor = '#ff4444'; }
            ctx.c.fillText('LEAVE SHOP', ctx.W / 2, leaveY + 10);
            ctx.c.shadowBlur = 0;

            ctx.c.fillStyle = '#666';
            ctx.c.font = '10px "Courier New",monospace';
            ctx.c.fillText('\u2191\u2193 select  ENTER buy  ESC leave', ctx.W / 2, ctx.H - 30);
        }

        ctx.openShop = openShop;
        ctx.closeShop = closeShop;
        ctx.updateShop = updateShop;
        ctx.renderShop = renderShop;
        ctx.shopItemCount = function() { return shopItems.length; };
    };
})();
