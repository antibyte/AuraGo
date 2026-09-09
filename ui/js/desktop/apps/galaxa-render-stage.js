(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};
    GC.createRenderStage = function (ctx) {
        function label(value, x, y, size, color, align) {
            const c = ctx.c; c.textAlign = align || 'center'; c.fillStyle = color || '#e2ebf5';
            c.font = 'bold ' + size + 'px "Share Tech Mono", monospace'; c.fillText(value, x, y, ctx.W - 48);
        }
        function panel(x, y, w, h, selected) {
            const c = ctx.c; c.fillStyle = selected ? '#182c47' : '#0c1629'; c.fillRect(x, y, w, h);
            c.fillStyle = selected ? '#e1be7d' : '#32465f';
            c.fillRect(x, y, w, 2); c.fillRect(x, y + h - 2, w, 2); c.fillRect(x, y, 2, h); c.fillRect(x + w - 2, y, 2, h);
            if (selected) { c.fillRect(x + 5, y + 5, 3, 3); c.fillRect(x + w - 8, y + h - 8, 3, 3); }
        }
        function renderTitle() {
            label('GALAXA', ctx.W / 2 + 2, 99, 54, '#283e5c'); label('GALAXA', ctx.W / 2, 95, 54, '#d5e8f7');
            label('D E L U X E', ctx.W / 2, 126, 20, '#e1be7d');
            label(ctx.t('galaxa.ship_select'), ctx.W / 2, 165, 11, '#809bb6');
            const ships = Object.keys(ctx.SHIP_TYPES);
            for (let i = 0; i < ships.length; i++) {
                const x = 30 + i * 122, selected = ships[i] === ctx.settings.ship;
                panel(x, 180, 114, 122, selected);
                ctx.drawAnimation(ctx.c, 'player.' + ships[i] + '.idle', x + 57, 230, ctx.G.animTime, 72);
                label(ctx.t('galaxa.' + ships[i]), x + 57, 284, 12, selected ? '#ead2a1' : '#aec2d6');
            }
            label(ctx.t('galaxa.ship_hint'), ctx.W / 2, 324, 11, '#809bb6');
            label(ctx.t('galaxa.mode_label') + ' · ' + ctx.getModeLabel(), ctx.W / 2, 370, 14);
            panel(132, 397, 276, 49, true); label(ctx.t('galaxa.insert_coin'), ctx.W / 2, 428, 18, '#f2d495');
            label(ctx.t('galaxa.high_score') + '  ' + String(ctx.G.hi).padStart(8, '0'), ctx.W / 2, 486, 15, '#84afc9');
            const rows = ctx.G.hiScores.slice(0, 4);
            rows.forEach((row, i) => label((i + 1) + '  ' + row.name + '  ' + row.score + '  · ' + row.stage, ctx.W / 2, 513 + i * 19, 12, '#afbdd0'));
            label(ctx.t('galaxa.play_hint'), ctx.W / 2, 626, 11, '#afbdd0');
            label(ctx.t('galaxa.menu_hint'), ctx.W / 2, 649, 11, '#809bb6');
            label(ctx.t('galaxa.action_parry') + ' X  ·  ' + ctx.t('galaxa.action_super') + ' C', ctx.W / 2, 676, 11, '#e1be7d');
        }
        function renderStageIntro() {
            const plan = ctx.stagePlan();
            panel(42, 294, ctx.W - 84, 102, true);
            label(ctx.t('galaxa.stage') + ' ' + ctx.G.stage, ctx.W / 2, 325, 24, '#e1be7d');
            label(ctx.t('galaxa.sector_' + GC.SECTORS[plan.sector].id), ctx.W / 2, 354, 18);
            label(ctx.t('galaxa.stage_' + plan.kind), ctx.W / 2, 379, 12, '#91b4cd');
        }
        function renderPause() {
            ctx.c.fillStyle = 'rgba(5,10,23,0.85)'; ctx.c.fillRect(0, 0, ctx.W, ctx.H);
            label(ctx.t('galaxa.paused'), ctx.W / 2, 265, 28, '#e1be7d');
            label(ctx.t('galaxa.score') + ' ' + ctx.G.score, ctx.W / 2, 299, 16);
            ['resume', 'restart', 'quit'].forEach((item, i) => {
                panel(125, 329 + i * 53, 290, 42, i === ctx.G.pauseSel);
                label(ctx.t('galaxa.' + item), ctx.W / 2, 357 + i * 53, 16, i === ctx.G.pauseSel ? '#e1be7d' : '#a1b7cb');
            });
        }
        const settingKeys = { sound: 'sound', difficulty: 'difficulty', volume: 'volume', musicVolume: 'music_volume', sfxVolume: 'sfx_volume',
            reducedMotion: 'reduced_motion', ship: 'ship_select', mode: 'mode_label', crt: 'crt_effect', particles: 'particle_density',
            shake: 'shake_intensity', riskIt: 'risk_it', adaptiveMusic: 'adaptive_music', quit: 'quit' };
        function value(item) {
            const v = ctx.settings[item.key];
            if (item.type === 'toggle') return ctx.t('galaxa.' + ((item.key === 'mute' ? !v : v) ? 'on' : 'off'));
            if (item.type === 'slider') return v + '%';
            if (item.key === 'shake') return Math.round(v * 100) + '%';
            if (item.type === 'action') return '';
            if (item.key === 'mode') return ctx.getModeLabel();
            return ctx.t('galaxa.' + v);
        }
        function renderSettings() {
            ctx.c.fillStyle = 'rgba(5,10,23,0.96)'; ctx.c.fillRect(0, 0, ctx.W, ctx.H);
            label(ctx.t('galaxa.settings'), ctx.W / 2, 58, 26, '#e1be7d');
            ctx.SETTINGS_ITEMS.forEach((item, i) => {
                const y = 83 + i * 37, selected = i === ctx.G.settingsSel;
                if (selected) panel(28, y, ctx.W - 56, 34, true);
                label(ctx.t('galaxa.' + settingKeys[item.id]), 42, y + 22, 12, selected ? '#f1d79f' : '#a9bed1', 'left');
                label(value(item), ctx.W - 42, y + 22, 12, '#b6d1e2', 'right');
            });
            label(ctx.t('galaxa.settings_hint'), ctx.W / 2, 667, 11, '#8fa5bb');
        }
        Object.assign(ctx, { renderTitle, renderStageIntro, renderPause, renderSettings });
    };
})();
