(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.W = 480;
    GC.H = 640;
    GC.PLAYER_SPEED = 220;
    GC.PLAYER_Y_MIN = 380;
    GC.PLAYER_Y_MAX = GC.H - 30;
    GC.PLAYER_VERTICAL_SPEED_MULT = 0.85;
    GC.PB_SPEED = 500;
    GC.EB_SPEED = 260;
    GC.FCOLS = 10;
    GC.FROWS = 5;
    GC.ESP_X = 36;
    GC.ESP_Y = 32;
    GC.FTOP = 60;
    GC.DIVE_SPD = 180;
    GC.EXTRA_LIFE = 20000;
    GC.TITLE_IDLE = 15000;

    GC.PU_TYPES = ['rapid', 'spread', 'shield', 'bomb', 'speed', 'magnet', 'laser', 'multibomb', 'timeslow', 'pierce', 'homing', 'supernova', 'freeze', 'levelskip', 'ricochet', 'drone', 'blackhole_bomb', 'gravity_bomb', 'mirror', 'orbital_shield', 'chain_lightning'];
    GC.PU_COL = { rapid: '#00ffcc', spread: '#ff6600', shield: '#4488ff', bomb: '#ff4444', speed: '#ffee00', magnet: '#ff44ff', laser: '#eeeeff', multibomb: '#cc2222', timeslow: '#aa44ff', pierce: '#88ffaa', homing: '#ff88aa', supernova: '#ffffff', freeze: '#88eeff', levelskip: '#ff88ff', ricochet: '#ffaa44', drone: '#44ffaa', blackhole_bomb: '#8844ff', gravity_bomb: '#cc66ff', mirror: '#88ddff', orbital_shield: '#44aaff', chain_lightning: '#aaddff' };
    GC.PU_DUR = { rapid: 8000, spread: 10000, speed: 6000, magnet: 8000, laser: 5000, timeslow: 4000, pierce: 6000, homing: 0, freeze: 4000, ricochet: 8000, drone: 10000, mirror: 8000, orbital_shield: 8000, chain_lightning: 10000 };
    GC.PU_UPGRADE = { rapid: 'ultra_rapid', spread: 'mega_spread', speed: 'hyper_speed', magnet: 'super_magnet', laser: 'mega_laser', pierce: 'mega_pierce', ricochet: 'mega_ricochet', drone: 'dual_drone' };
    GC.PU_UPGRADE_COL = { ultra_rapid: '#00ffee', mega_spread: '#ff8800', hyper_speed: '#ffff44', super_magnet: '#ff88ff', mega_laser: '#ccddff', mega_pierce: '#aaffcc', mega_ricochet: '#ffcc66', dual_drone: '#66ffcc' };
    GC.PU_TRAIL_COL = { rapid: '0,255,204', ultra_rapid: '0,255,238', spread: '255,102,0', mega_spread: '255,136,0', shield: '68,136,255', speed: '255,238,0', hyper_speed: '255,255,68', magnet: '255,68,255', super_magnet: '255,136,255', laser: '180,200,255', mega_laser: '160,180,255', timeslow: '170,68,255', pierce: '136,255,170', mega_pierce: '170,255,204', homing: '255,136,170', freeze: '136,238,255', levelskip: '255,136,255', ricochet: '255,170,68', mega_ricochet: '255,204,102', drone: '68,255,170', dual_drone: '102,255,204', blackhole_bomb: '136,68,255', gravity_bomb: '204,102,255', mirror: '136,221,255', orbital_shield: '68,170,255', chain_lightning: '170,221,255' };
    GC.PU_RARITY = {
        common: ['rapid', 'spread', 'speed', 'pierce'],
        uncommon: ['shield', 'magnet', 'laser', 'ricochet', 'mirror', 'orbital_shield'],
        rare: ['homing', 'drone', 'timeslow', 'freeze', 'chain_lightning'],
        legendary: ['bomb', 'multibomb', 'supernova', 'blackhole_bomb', 'levelskip', 'gravity_bomb']
    };
    GC.PU_RARITY_WEIGHT = { common: 50, uncommon: 30, rare: 15, legendary: 5 };
    GC.PU_SYNERGIES = {
        'rapid+pierce': { name: 'Phaser', col: '#00ffaa', desc: 'Double fire rate + pierce' },
        'spread+homing': { name: 'Swarm', col: '#ff8844', desc: 'Spread bullets curve toward targets' },
        'shield+magnet': { name: 'Aegis', col: '#88aaff', desc: 'Shield reflects bullets + pulls powerups' },
        'laser+timeslow': { name: 'Chrono-Beam', col: '#cc88ff', desc: 'Laser slows hit enemies' },
        'drone+ricochet': { name: 'Bouncer', col: '#66ffaa', desc: 'Drone bullets bounce off walls' }
    };

    GC.COMBO_TIMEOUT = 2000;
    GC.COMBO_THRESH = [2, 3, 5, 8, 10, 15, 20];
    GC.COMBO_MULT = [1, 2, 4, 4, 8, 8, 16, 16];
    GC.COMBO_TEXT = ['', 'DOUBLE KILL', 'TRIPLE KILL', 'RAMPAGE', 'UNSTOPPABLE', 'GODLIKE', 'LEGENDARY', 'BEYOND'];

    GC.ATTACK_PATTERNS = {
        spiral: function(e, count, speed, spread, ebSpd) {
            const a0 = (e.shootPh || 0) + Math.PI / 3;
            e.shootPh = a0;
            const bullets = [];
            for (let i = 0; i < count; i++) {
                const a = a0 + i * (Math.PI * 2 / count);
                bullets.push({ x: e.x, y: e.y + 8, w: 2, h: 4, vx: Math.cos(a) * ebSpd * speed, vy: Math.sin(a) * ebSpd * speed, kind: 'spiral' });
            }
            return bullets;
        },
        circle: function(e, count, speed, ebSpd) {
            const bullets = [];
            for (let i = 0; i < count; i++) {
                const a = (i / count) * Math.PI * 2;
                bullets.push({ x: e.x, y: e.y + 8, w: 2, h: 4, vx: Math.cos(a) * ebSpd * speed, vy: Math.sin(a) * ebSpd * speed, kind: 'spiral' });
            }
            return bullets;
        },
        wave: function(e, count, speed, amplitude, ebSpd) {
            const bullets = [];
            for (let i = 0; i < count; i++) {
                const a = -Math.PI / 2 + (i - count / 2) * amplitude;
                bullets.push({ x: e.x, y: e.y + 8, w: 2, h: 4, vx: Math.cos(a) * ebSpd * speed * 0.3, vy: Math.sin(a) * ebSpd * speed, kind: 'spiral' });
            }
            return bullets;
        },
        aimed_burst: function(e, count, speed, delay, ebSpd, targetX, targetY) {
            const dx = targetX - e.x, dy = targetY - e.y, dist = Math.hypot(dx, dy) || 1;
            const baseA = Math.atan2(dy, dx);
            const bullets = [];
            for (let i = 0; i < count; i++) {
                const a = baseA + (i - count / 2) * 0.15;
                bullets.push({ x: e.x, y: e.y + 8, w: 2, h: 5, vx: Math.cos(a) * ebSpd * speed, vy: Math.sin(a) * ebSpd * speed, kind: 'hunter' });
            }
            return bullets;
        },
        random_spread: function(e, count, speed, angle, ebSpd) {
            const bullets = [];
            for (let i = 0; i < count; i++) {
                const a = -Math.PI / 2 + (Math.random() - 0.5) * angle;
                bullets.push({ x: e.x, y: e.y + 8, w: 2, h: 6, vx: Math.cos(a) * ebSpd * speed, vy: Math.sin(a) * ebSpd * speed });
            }
            return bullets;
        },
        wall: function(e, rows, cols, speed, ebSpd) {
            const bullets = [];
            for (let r = 0; r < rows; r++) {
                for (let c = 0; c < cols; c++) {
                    bullets.push({ x: e.x + (c - cols / 2) * 14, y: e.y + 8 + r * 10, w: 2, h: 4, vx: 0, vy: ebSpd * speed });
                }
            }
            return bullets;
        }
    };

    GC.STAGE_MODIFIERS = [
        { id: 'double_score', name: 'Double Score', desc: 'All points x2', col: '#ffcc00', apply: function(G) { G.scoreMult = 2; } },
        { id: 'glass_cannon', name: 'Glass Cannon', desc: 'Everyone has 1 HP', col: '#ff4444', apply: function(G) { G.glassCannon = true; } },
        { id: 'bullet_storm', name: 'Bullet Storm', desc: '2x enemy fire rate', col: '#ff8800', apply: function(G) { G.bulletStorm = true; } },
        { id: 'power_surge', name: 'Power Surge', desc: '3x powerup drops', col: '#44ff88', apply: function(G) { G.powerSurge = true; } },
        { id: 'darkness', name: 'Darkness', desc: 'Reduced visibility', col: '#444466', apply: function(G) { G.darkness = true; } },
        { id: 'turbo', name: 'Turbo', desc: '1.5x speed everything', col: '#44aaff', apply: function(G) { G.turbo = true; G.timeScale = 1.5; } },
        { id: 'mirror_field', name: 'Mirror Field', desc: '20% bullets split', col: '#88ddff', apply: function(G) { G.mirrorField = true; } },
        { id: 'gravity_well', name: 'Gravity Well', desc: 'Center pull affects all', col: '#cc66ff', apply: function(G) { G.gravityWell = true; } },
        { id: 'phasing', name: 'Phasing', desc: 'Enemies blink invulnerable', col: '#44ffff', apply: function(G) { G.phasing = true; } },
        { id: 'ricochet_world', name: 'Ricochet World', desc: 'Bullets bounce off walls', col: '#ffaa44', apply: function(G) { G.ricochetWorld = true; } }
    ];

    GC.NEW_ENEMY_TYPES = {
        weaver: { name: 'Weaver', stageMin: 7, hp: 1, pts: [130, 260], col: '#ff8844', desc: 'Moves in sine wave, shoots in direction' },
        splitter: { name: 'Splitter', stageMin: 8, hp: 2, pts: [140, 280], col: '#88ff44', desc: 'Splits into 2 mini enemies on death' },
        shield_bee: { name: 'Shield Bee', stageMin: 4, hp: 2, pts: [70, 140], col: '#ffcc00', desc: 'Bee with 1 extra shield HP' },
        kamikaze: { name: 'Kamikaze', stageMin: 6, hp: 1, pts: [150, 300], col: '#ff2222', desc: 'Charges at player, explodes on contact' },
        carrier: { name: 'Carrier', stageMin: 9, hp: 3, pts: [200, 400], col: '#cc88ff', desc: 'Releases 3 bees on death' },
        teleporter: { name: 'Teleporter', stageMin: 10, hp: 2, pts: [160, 320], col: '#44ffff', desc: 'Teleports every 2s to random position' }
    };

    GC.ACHIEVEMENTS = {
        first_blood: { name: 'First Blood', desc: 'Kill your first enemy' },
        combo_king: { name: 'Combo King', desc: 'Reach 15 combo' },
        untouchable: { name: 'Untouchable', desc: 'Complete a challenge stage without damage' },
        dual_wielder: { name: 'Dual Wielder', desc: 'Rescue a captured fighter' },
        survivor: { name: 'Survivor', desc: 'Reach stage 10' },
        legend: { name: 'Legend', desc: 'Reach stage 20' },
        perfectionist: { name: 'Perfectionist', desc: 'Get 3 perfect challenge stages' },
        power_collector: { name: 'Power Collector', desc: 'Collect all powerup types in one run' },
        speed_demon: { name: 'Speed Demon', desc: 'Complete a stage in under 30 seconds' },
        boss_slayer: { name: 'Boss Slayer', desc: 'Defeat 10 bosses total' },
        millionaire: { name: 'Millionaire', desc: 'Score 1,000,000 points' },
        no_powerups: { name: 'Purist', desc: 'Reach stage 10 without collecting any powerup' },
        one_life: { name: 'Iron Man', desc: 'Reach stage 5 starting with 1 life' },
        bullet_hell: { name: 'Bullet Hell', desc: 'Have 100+ enemy bullets on screen at once' },
        boss_rusher: { name: 'Boss Rusher', desc: 'Reach stage 10 in Boss Rush mode' },
        collector: { name: 'Collector', desc: 'Collect all 17 powerup types in a single run' },
        speedrun: { name: 'Speedrunner', desc: 'Complete stages 1-5 in under 2 minutes' },
        pacifist: { name: 'Pacifist', desc: 'Clear a stage without firing (drones/blackhole only)' },
        overcharge: { name: 'Overcharged', desc: 'Use Overcharge 5 times in one run' },
        shopaholic: { name: 'Shopaholic', desc: 'Visit the shop 10 times' },
        daily_warrior: { name: 'Daily Warrior', desc: 'Complete 7 daily challenges' },
        flawless_boss: { name: 'Flawless', desc: 'Defeat a boss without taking damage' },
        combo_god: { name: 'Combo God', desc: 'Reach 30 combo' },
        weapon_master: { name: 'Weapon Master', desc: 'Reach weapon level 4' },
        ship_collector: { name: 'Fleet Commander', desc: 'Reach stage 10 with all 4 ship types' },
        no_damage_run: { name: 'Untouchable II', desc: 'Complete 3 stages in a row without damage' },
        black_hole_master: { name: 'Singularity', desc: 'Kill 20 enemies with one Black Hole Bomb' },
        freeze_frame: { name: 'Cryo Master', desc: 'Kill 50 enemies while they are frozen' },
        ricochet_master: { name: 'Trick Shot', desc: 'Kill 30 enemies with ricochet bullets' },
        supernova_survivor: { name: 'Supernova Survivor', desc: 'Survive an enemy-triggered Supernova' },
        rage_survivor: { name: 'Rage Survivor', desc: 'Survive 10 rage-mode enemies' },
        chain_master: { name: 'Chain Master', desc: 'Hit 5 enemies with one chain lightning' },
        orbital_king: { name: 'Orbital King', desc: 'Block 20 bullets with orbital shields' },
        mutation_master: { name: 'Mutation Master', desc: 'Complete 5 mutated stages' }
    };

    GC.SHIP_TYPES = {
        classic: { name: 'Classic', speedMult: 1, lifeMod: 0, hitboxMod: 0, invMult: 1, diveTargetMod: 1 },
        interceptor: { name: 'Interceptor', speedMult: 1.3, lifeMod: -1, hitboxMod: -2, invMult: 1, diveTargetMod: 1 },
        heavy: { name: 'Heavy', speedMult: 0.8, lifeMod: 1, hitboxMod: 2, invMult: 1, diveTargetMod: 1 },
        stealth: { name: 'Stealth', speedMult: 1, lifeMod: 0, hitboxMod: 0, invMult: 0.6, diveTargetMod: 0.5 }
    };

    GC.PTS = { bee: [50, 100], butterfly: [80, 160], boss: [400, 800], miniboss: [600, 1200], stalker: [120, 240], sniper: [100, 200], hunter: [200, 400], spinner: [90, 180], bomber: [110, 220], lasher: [80, 160], weaver: [130, 260], splitter: [140, 280], shield_bee: [70, 140], kamikaze: [150, 300], carrier: [200, 400], teleporter: [160, 320] };
})();
