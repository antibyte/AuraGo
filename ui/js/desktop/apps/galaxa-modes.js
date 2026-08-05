(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.GAUNTLET_WAVES = [
        ['double_score'],
        ['glass_cannon', 'bullet_storm'],
        ['power_surge'],
        ['darkness', 'turbo'],
        ['mirror_field'],
        ['gravity_well', 'phasing'],
        ['ricochet_world'],
        ['bullet_storm', 'glass_cannon'],
        ['turbo', 'double_score'],
        ['darkness', 'mirror_field'],
        ['phasing', 'gravity_well'],
        ['bullet_storm', 'ricochet_world']
    ];

    GC.MODE_IDS = ['classic', 'endless', 'boss_rush', 'gauntlet', 'hyperdrive', 'mirror'];

    GC.createModes = function (ctx) {
        function modeId() { return ctx.settings.mode || 'classic'; }
        function isMode(id) { return modeId() === id; }

        function modById(id) {
            for (let i = 0; i < ctx.STAGE_MODIFIERS.length; i++) {
                if (ctx.STAGE_MODIFIERS[i].id === id) return ctx.STAGE_MODIFIERS[i];
            }
            return null;
        }

        function applyModifierIds(ids) {
            const applied = [];
            for (let i = 0; i < ids.length; i++) {
                const m = modById(ids[i]);
                if (m) { m.apply(ctx.G); applied.push(m); }
            }
            ctx.G.stageModifier = applied.length === 1 ? applied[0] : applied;
        }

        function resetModeFlags() {
            ctx.G.mirrorMode = false;
            ctx.G.gauntletWave = 0;
            ctx.G.gauntletComplete = false;
            ctx.G.hyperdriveBaseScale = 1;
        }

        function onRunStart() {
            resetModeFlags();
            if (isMode('gauntlet')) {
                ctx.G.lives = 3;
                ctx.G.gauntletWave = 1;
                ctx.G.stage = 1;
                ctx.G.contCnt = 0;
            }
            if (isMode('mirror')) {
                ctx.G.mirrorMode = true;
                ctx.G.mirrorActive = true;
            }
            if (isMode('hyperdrive')) {
                ctx.G.hyperdriveBaseScale = 1;
                ctx.G.timeScale = 1;
            }
        }

        function restoreTimeScale() {
            if (ctx.G.turbo) return 1.5;
            if (isMode('hyperdrive')) return ctx.G.hyperdriveBaseScale || 1;
            return 1;
        }

        function onStageStart() {
            if (isMode('gauntlet')) {
                const waveIdx = Math.min(ctx.G.stage - 1, GC.GAUNTLET_WAVES.length - 1);
                ctx.G.gauntletWave = ctx.G.stage;
                applyModifierIds(GC.GAUNTLET_WAVES[waveIdx]);
                if (ctx.SFX && ctx.SFX.gauntletWave) ctx.SFX.gauntletWave();
                ctx.G.scorePopups.push({
                    x: ctx.W / 2, y: 56,
                    text: ctx.t('galaxa.gauntlet_wave', 'GAUNTLET') + ' ' + ctx.G.gauntletWave + '/' + GC.GAUNTLET_WAVES.length,
                    t: 0, dur: 1800, col: '#ff8844', big: true
                });
            }
            if (isMode('hyperdrive')) {
                const scale = Math.min(2, 1 + (ctx.G.stage - 1) * 0.05);
                ctx.G.hyperdriveBaseScale = scale;
                ctx.G.timeScale = scale;
                const mods = ctx.STAGE_MODIFIERS;
                const mod = mods[(ctx.G.stage - 1) % mods.length];
                if (mod) { mod.apply(ctx.G); ctx.G.stageModifier = mod; }
                if (ctx.fxHyperTunnel) ctx.fxHyperTunnel();
                if (ctx.SFX && ctx.SFX.hyperShift) ctx.SFX.hyperShift();
            }
            if (isMode('mirror')) {
                ctx.G.mirrorMode = true;
                ctx.G.mirrorActive = true;
            }
        }

        function shouldOpenShop() {
            return !isMode('gauntlet');
        }

        function allowContinue() {
            return !isMode('gauntlet');
        }

        function getBaseMusicTheme(chal) {
            if (isMode('hyperdrive')) return 'hyperdrive';
            if (isMode('gauntlet')) return 'gauntlet';
            if (isMode('mirror')) return 'mirror';
            return chal ? 'challenge' : 'gameplay';
        }

        function onStageClearBeforeAdvance() {
            if (isMode('gauntlet') && ctx.G.stage >= GC.GAUNTLET_WAVES.length) {
                ctx.G.gauntletComplete = true;
                ctx.unlockAchievement('gauntlet_clear');
                if (ctx.fxScreenShatter) ctx.fxScreenShatter();
                if (ctx.SFX && ctx.SFX.screenShatter) ctx.SFX.screenShatter();
                ctx.G.scorePopups.push({
                    x: ctx.W / 2, y: ctx.H / 2 - 30,
                    text: ctx.t('galaxa.gauntlet_clear', 'GAUNTLET CLEAR!'),
                    t: 0, dur: 3500, col: '#ffcc00', big: true
                });
                return 'gauntlet_win';
            }
            if (isMode('hyperdrive') && ctx.G.stage >= 15) {
                ctx.unlockAchievement('hyper_survivor');
            }
            if (isMode('mirror') && ctx.G.parryCount >= 25) {
                ctx.unlockAchievement('mirror_master');
            }
            return null;
        }

        function getModeLabel() {
            return ctx.t('galaxa.mode_' + modeId(), modeId().replace(/_/g, ' ').toUpperCase());
        }

        function mirrorGhostDamageMult() {
            return isMode('mirror') ? 0.5 : 1;
        }

        function isMirrorPermanent() {
            return isMode('mirror') || ctx.G.mirrorMode;
        }

        ctx.modeId = modeId;
        ctx.isGameMode = isMode;
        ctx.modesOnRunStart = onRunStart;
        ctx.modesOnStageStart = onStageStart;
        ctx.modesShouldOpenShop = shouldOpenShop;
        ctx.modesAllowContinue = allowContinue;
        ctx.modesGetBaseMusicTheme = getBaseMusicTheme;
        ctx.modesOnStageClearBeforeAdvance = onStageClearBeforeAdvance;
        ctx.getModeLabel = getModeLabel;
        ctx.modesRestoreTimeScale = restoreTimeScale;
        ctx.modesMirrorGhostDamageMult = mirrorGhostDamageMult;
        ctx.modesIsMirrorPermanent = isMirrorPermanent;
    };
})();
