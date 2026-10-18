(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    // OPTIMIZATION: Playing field slightly enlarged from 480x640 to 540x720
    // (+12.5% wider, +12.5% taller) for more room to maneuver, larger formations
    // and a less cramped feel. All game logic uses GC.W / GC.H via ctx inheritance,
    // so the change propagates automatically. Tuning notes:
    //   - FCOLS bumped 10->11 and ESP_X 48->50 so the formation fills the new
    //     width without leaving a wide empty band on the sides.
    //   - PLAYER_Y_MIN raised 380->400 so the player gets ~260px vertical range
    //     (was ~180px) — more dodging room, same lower safe zone.
    //   - PLAYER_Y_MAX kept at H-80 to preserve the bottom safe margin during
    //     cinematic super-zoom.
    GC.W = 540;
    GC.H = 720;
    GC.SECTORS = [
        { id: 'nebula', enemies: ['bee', 'butterfly', 'stalker'] },
        { id: 'asteroid', enemies: ['bomber', 'kamikaze', 'hunter', 'bee'] },
        { id: 'crystal', enemies: ['shield_bee', 'splitter', 'sniper', 'butterfly'] },
        { id: 'storm', enemies: ['spinner', 'carrier', 'lasher', 'hunter'] },
        { id: 'blackhole', enemies: ['weaver', 'stalker', 'splitter', 'bomber'] },
        { id: 'void', enemies: ['teleporter', 'carrier', 'sniper', 'kamikaze'] }
    ];
    GC.getStagePlan = function (stage, mode) {
        const n = Math.max(0, Math.floor(stage || 1) - 1);
        if (mode === 'boss_rush') return { sector: n % 6, step: 5, loop: Math.floor(n / 6) + 1, kind: 'boss' };
        return { sector: Math.floor(n % 30 / 5), step: n % 5 + 1, loop: Math.floor(n / 30) + 1,
            kind: ['intro', 'escort', 'bonus', 'elite', 'boss'][n % 5] };
    };
    GC.makeRunRandom = function (seed) {
        let s = (Math.floor(seed) >>> 0) % 2147483646 + 1;
        return function () { s = s * 16807 % 2147483647; return (s - 1) / 2147483646; };
    };
    GC.PLAYER_SPEED = 220;
    GC.PLAYER_Y_MIN = 400;
    // FIX: tightened from H-30 to H-60 so the 24x24 player sprite stays fully
    // visible during the cinematic super camera zoom (max camZoom 1.15x).
    // With the old value, sprite bottom (p.y+12 = 622) was drawn at canvas y
    // ~667 when zoomed in — 27 pixels below the canvas bottom (640) — so the
    // sprite appeared to "leave the field" by ~one player height during supers.
    // New value gives sprite bottom at 592, drawn at canvas y ~633 when zoomed
    // (8px clear of canvas bottom).
    GC.PLAYER_Y_MAX = GC.H - 80;
    GC.PLAYER_VERTICAL_SPEED_MULT = 0.85;
    GC.PB_SPEED = 500;
    GC.EB_SPEED = 260;
    GC.FCOLS = 11;
    GC.FROWS = 5;
    GC.ESP_X = 50;
    GC.ESP_Y = 43;
    GC.FTOP = 60;
    GC.DIVE_SPD = 180;
    GC.EXTRA_LIFE = 20000;
    GC.TITLE_IDLE = 15000;

    GC.PU_TYPES = ['rapid', 'spread', 'shield', 'bomb', 'speed', 'magnet', 'laser', 'multibomb', 'timeslow', 'pierce', 'homing', 'supernova', 'freeze', 'levelskip', 'ricochet', 'drone', 'blackhole_bomb', 'gravity_bomb', 'mirror', 'orbital_shield', 'chain_lightning', 'rocket_launcher', 'mine_layer', 'megabomb'];
    GC.PU_COL = { rapid: '#00ffcc', spread: '#ff6600', shield: '#4488ff', bomb: '#ff4444', speed: '#ffee00', magnet: '#ff44ff', laser: '#eeeeff', multibomb: '#cc2222', timeslow: '#aa44ff', pierce: '#88ffaa', homing: '#ff88aa', supernova: '#ffffff', freeze: '#88eeff', levelskip: '#ff88ff', ricochet: '#ffaa44', drone: '#44ffaa', blackhole_bomb: '#8844ff', gravity_bomb: '#cc66ff', mirror: '#88ddff', orbital_shield: '#44aaff', chain_lightning: '#aaddff', rocket_launcher: '#ff7722', mine_layer: '#ccaa44', megabomb: '#ff2200' };
    GC.PU_DUR = { rapid: 8000, spread: 10000, speed: 6000, magnet: 8000, laser: 5000, timeslow: 4000, pierce: 6000, homing: 0, freeze: 4000, ricochet: 8000, drone: 10000, mirror: 8000, orbital_shield: 8000, chain_lightning: 10000, rocket_launcher: 12000, mine_layer: 10000, mega_rocket: 12000, mega_mine_layer: 10000 };
    GC.PU_UPGRADE = { rapid: 'ultra_rapid', spread: 'mega_spread', speed: 'hyper_speed', magnet: 'super_magnet', laser: 'mega_laser', pierce: 'mega_pierce', ricochet: 'mega_ricochet', drone: 'dual_drone', rocket_launcher: 'mega_rocket', mine_layer: 'mega_mine_layer' };
    GC.PU_UPGRADE_COL = { ultra_rapid: '#00ffee', mega_spread: '#ff8800', hyper_speed: '#ffff44', super_magnet: '#ff88ff', mega_laser: '#ccddff', mega_pierce: '#aaffcc', mega_ricochet: '#ffcc66', dual_drone: '#66ffcc', mega_rocket: '#ff9944', mega_mine_layer: '#eedd66' };
    GC.PU_TRAIL_COL = { rapid: '0,255,204', ultra_rapid: '0,255,238', spread: '255,102,0', mega_spread: '255,136,0', shield: '68,136,255', speed: '255,238,0', hyper_speed: '255,255,68', magnet: '255,68,255', super_magnet: '255,136,255', laser: '180,200,255', mega_laser: '160,180,255', timeslow: '170,68,255', pierce: '136,255,170', mega_pierce: '170,255,204', homing: '255,136,170', freeze: '136,238,255', levelskip: '255,136,255', ricochet: '255,170,68', mega_ricochet: '255,204,102', drone: '68,255,170', dual_drone: '102,255,204', blackhole_bomb: '136,68,255', gravity_bomb: '204,102,255', mirror: '136,221,255', orbital_shield: '68,170,255', chain_lightning: '170,221,255', rocket_launcher: '255,120,40', mega_rocket: '255,150,60', mine_layer: '204,170,68', mega_mine_layer: '238,204,102' };
    GC.PU_RARITY = {
        common: ['rapid', 'spread', 'speed', 'pierce'],
        uncommon: ['shield', 'magnet', 'laser', 'ricochet', 'mirror', 'orbital_shield'],
        rare: ['homing', 'drone', 'timeslow', 'freeze', 'chain_lightning', 'rocket_launcher', 'mine_layer'],
        legendary: ['bomb', 'multibomb', 'supernova', 'blackhole_bomb', 'levelskip', 'gravity_bomb', 'megabomb']
    };
    GC.PU_RARITY_WEIGHT = { common: 50, uncommon: 30, rare: 15, legendary: 5 };
    GC.PLAYER_MINE_MAX = 12;
    GC.EXPANSION_WEAPONS = {
        mine_layer: { art: 'proximity-mine', mine: true, interval: 780, radius: 52, damage: 1 },
        mega_mine_layer: { art: 'proximity-mine', mine: true, interval: 520, count: 2, radius: 52, damage: 1 },
        emp_mine_layer: { art: 'emp-mine', mine: true, interval: 1000, radius: 72, damage: 0, emp: true },
        cluster_mine_layer: { art: 'cluster-mine', mine: true, interval: 1000, radius: 40, damage: 1, shards: 6 },
        rocket_launcher: { art: 'seeker-rocket', interval: 380, count: 1, damage: 1, radius: 50, speed: 575 },
        mega_rocket: { art: 'seeker-rocket', interval: 280, count: 2, damage: 1, radius: 50, speed: 575 },
        swarm_launcher: { art: 'swarm-rocket', interval: 850, count: 3, damage: 1, radius: 0, speed: 575 },
        seeker_torpedo: { art: 'seeker-torpedo', interval: 1200, count: 1, damage: 4, radius: 70, speed: 210 }
    };
    for (const type of ['emp_mine_layer', 'cluster_mine_layer', 'swarm_launcher', 'seeker_torpedo']) {
        GC.PU_TYPES.push(type); GC.PU_RARITY.rare.push(type);
        GC.PU_DUR[type] = GC.EXPANSION_WEAPONS[type].mine ? 10000 : 12000;
    }
    Object.assign(GC.PU_COL, { emp_mine_layer: '#66ddff', cluster_mine_layer: '#ffcc55', swarm_launcher: '#66ffaa', seeker_torpedo: '#ffaa66' });
    Object.assign(GC.PU_TRAIL_COL, { emp_mine_layer: '102,221,255', cluster_mine_layer: '255,204,85', swarm_launcher: '102,255,170', seeker_torpedo: '255,170,102' });
    GC.ASTEROID_LIMIT = 20;
    GC.ASTEROID_HP = [1, 3, 6];
    GC.ASTEROID_RADIUS = [12, 21, 31];
    GC.ASTEROID_WARNING_MS = 900;
    GC.MINE_ARM_MS = 450;
    GC.MINE_LIFE_MS = 9000;
    GC.SEEKER_LIFE_MS = 6000;
    GC.PLANET_SHEET_LIMIT = 2;
    GC.EXPANSION_ALIENS = {"nebula": [{"id": "nebula-ray", "role": "butterfly"}, {"id": "spore-hunter", "role": "stalker"}], "asteroid": [{"id": "ore-cracker", "role": "bomber"}, {"id": "armor-crab", "role": "shield_bee"}], "crystal": [{"id": "prism-falcon", "role": "sniper"}, {"id": "splinter-polyp", "role": "splitter"}], "storm": [{"id": "lightning-jelly", "role": "spinner"}, {"id": "storm-wasp", "role": "hunter"}], "blackhole": [{"id": "gravity-spider", "role": "weaver"}, {"id": "phase-isopod", "role": "teleporter"}], "void": [{"id": "void-runner", "role": "kamikaze"}, {"id": "swarm-carrier", "role": "carrier"}]};
    GC.ASTEROID_ART = [{"id": "crater-boulder", "effect": "rock-shatter"}, {"id": "basalt-monolith", "effect": "rock-shatter"}, {"id": "iron-meteor", "effect": "metal-sparks"}, {"id": "ore-geode", "effect": "metal-sparks"}, {"id": "ice-boulder", "effect": "ice-shatter"}, {"id": "frost-comet", "effect": "ice-shatter"}, {"id": "crystal-cluster", "effect": "crystal-shatter"}, {"id": "prism-shard", "effect": "crystal-shatter"}, {"id": "lava-boulder", "effect": "lava-ejecta"}, {"id": "slag-core", "effect": "lava-ejecta"}, {"id": "wreck-fragment", "effect": "metal-sparks"}, {"id": "hollow-asteroid", "effect": "rock-shatter"}];
    GC.PLANET_ART = {"nebula": ["ocean", "terran"], "asteroid": ["desert", "crater"], "crystal": ["ice", "crystal"], "storm": ["storm-gas", "ringed"], "blackhole": ["lava"], "void": ["night"]};

    GC.PU_SYNERGIES = {
        'rapid+pierce': { name: 'Phaser', col: '#00ffaa', desc: 'Double fire rate + pierce' },
        'spread+homing': { name: 'Swarm', col: '#ff8844', desc: 'Spread bullets curve toward targets' },
        'shield+magnet': { name: 'Aegis', col: '#88aaff', desc: 'Shield reflects bullets + pulls powerups' },
        'laser+timeslow': { name: 'Chrono-Beam', col: '#cc88ff', desc: 'Laser slows hit enemies' },
        'drone+ricochet': { name: 'Bouncer', col: '#66ffaa', desc: 'Drone bullets bounce off walls' }
        ,
        'rapid+speed': { name: 'BLITZ', col: '#ffff44', desc: '3x fire rate + 1.5x movement' },
        'pierce+laser': { name: 'ANNIHILATOR', col: '#ff44aa', desc: 'Laser pierces all + 50% damage' },
        'magnet+orbital_shield': { name: 'GRAVITY WELL', col: '#88ddff', desc: 'Shields attract powerups' }
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
        random_spread: function(e, count, speed, angle, ebSpd, random = Math.random) {
            const bullets = [];
            for (let i = 0; i < count; i++) {
                const a = Math.PI / 2 + (random() - 0.5) * angle;
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
        },
        // Fan that swings left/right from volley to volley, like a turret sweeping the field.
        fan_sweep: function (e, count, speed, arc, ebSpd) {
            e.sweepPh = ((e.sweepPh || 0) + 0.55) % (Math.PI * 2);
            const center = Math.PI / 2 + Math.sin(e.sweepPh) * arc * 0.5, bullets = [];
            for (let i = 0; i < count; i++) {
                const a = Math.max(0.15, Math.min(Math.PI - 0.15, center + (i - (count - 1) / 2) * (arc / Math.max(1, count - 1))));
                bullets.push({ x: e.x, y: e.y + 8, w: 2, h: 4, vx: Math.cos(a) * ebSpd * speed, vy: Math.sin(a) * ebSpd * speed, kind: 'spiral' });
            }
            return bullets;
        },
        // Two shots leave sideways and curve back toward the target column (steerX spring, see updateBul).
        pincer: function (e, speed, ebSpd, targetX, kind) {
            return [-1, 1].map(side => ({ x: e.x + side * 6, y: e.y + 8, w: 2, h: 5, vx: side * ebSpd * speed * 0.9, vy: ebSpd * speed * 0.55, steerX: targetX, steer: 4, kind: kind || 'hunter' }));
        },
        // Lobbed shells: they rise about half their speed value before gravity (ay) pulls them down into the player's lane.
        lob: function (e, count, speed, ebSpd, random) {
            const bullets = [];
            for (let i = 0; i < count; i++) {
                const sx = (i - (count - 1) / 2) * 0.35 + (random() - 0.5) * 0.2;
                bullets.push({ x: e.x, y: e.y + 8, w: 3, h: 3, vx: sx * ebSpd * speed, vy: -ebSpd * speed * 1.1, ay: ebSpd * speed * 1.2, kind: 'gravity' });
            }
            return bullets;
        },
        // A curtain of shots across a span, staggered in height so they arrive as a rolling wall.
        rain: function (e, count, speed, span, ebSpd, random) {
            const bullets = [];
            for (let i = 0; i < count; i++) {
                const fx = (count > 1 ? i / (count - 1) - 0.5 : 0) * span + (random() - 0.5) * 14;
                bullets.push({ x: e.x + fx, y: e.y + 8 - Math.abs(fx) * 0.25 - random() * 18, w: 2, h: 6, vx: 0, vy: ebSpd * speed, kind: 'ion' });
            }
            return bullets;
        },
        // Leads the target: aims where the ship will be when the shot arrives (two refinement passes).
        predictive: function (e, count, speed, ebSpd, target) {
            const vel = ebSpd * speed; let tx = target.x, ty = target.y;
            for (let k = 0; k < 2; k++) { const t = Math.hypot(tx - e.x, ty - e.y) / Math.max(1, vel); tx = target.x + (target.vx || 0) * t; ty = target.y + (target.vy || 0) * t; }
            const base = Math.atan2(ty - e.y, tx - e.x), bullets = [];
            for (let i = 0; i < count; i++) { const a = base + (i - (count - 1) / 2) * 0.12; bullets.push({ x: e.x, y: e.y + 8, w: 2, h: 5, vx: Math.cos(a) * vel, vy: Math.sin(a) * vel, kind: 'sniper' }); }
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
        collector: { name: 'Collector', desc: 'Collect all powerup types in a single run' },
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
        mutation_master: { name: 'Mutation Master', desc: 'Complete 5 mutated stages' },
        parry_master: { name: 'Parry Master', desc: 'Parry 50 enemy bullets in one run' },
        super_nova: { name: 'Nova Strike', desc: 'Activate your ship super 10 times total' },
        biome_explorer: { name: 'Explorer', desc: 'Reach every biome' },
        bonus_hunter: { name: 'Bonus Hunter', desc: 'Complete 5 bonus sub-stages' },
        gauntlet_clear: { name: 'Gauntlet Champion', desc: 'Clear all 12 Gauntlet waves' },
        hyper_survivor: { name: 'Hyper Survivor', desc: 'Reach stage 15 in Hyperdrive' },
        mirror_master: { name: 'Mirror Master', desc: 'Parry 25 bullets in Mirror mode' }
    };

    GC.SHIP_TYPES = {
        classic: { name: 'Classic', speedMult: 1, lifeMod: 0, hitboxMod: 0, invMult: 1, diveTargetMod: 1 },
        interceptor: { name: 'Interceptor', speedMult: 1.3, lifeMod: -1, hitboxMod: -3, invMult: 1, diveTargetMod: 1 },
        heavy: { name: 'Heavy', speedMult: 0.8, lifeMod: 1, hitboxMod: 3, invMult: 1, diveTargetMod: 1 },
        stealth: { name: 'Stealth', speedMult: 1, lifeMod: 0, hitboxMod: 0, invMult: 0.6, diveTargetMod: 0.5 }
    };

    GC.PTS = { bee: [50, 100], butterfly: [80, 160], boss: [400, 800], miniboss: [600, 1200], stalker: [120, 240], sniper: [100, 200], hunter: [200, 400], spinner: [90, 180], bomber: [110, 220], lasher: [80, 160], weaver: [130, 260], splitter: [140, 280], shield_bee: [70, 140], kamikaze: [150, 300], carrier: [200, 400], teleporter: [160, 320] };

    // NEW: Biome progression — one biome per 5 stages, each with own identity.
    GC.BIOMES = [
        { id: 'nebula', name: 'NEBULA', stages: [1, 2, 3, 4, 5], bgTheme: 'nebula', musicTheme: 'gameplay', hazardTheme: 'nebula', palette: ['#1a0033', '#4488ff', '#ffcc00'], enemyBonus: [], desc: 'Drifting gas clouds' },
        { id: 'asteroid', name: 'ASTEROID FIELD', stages: [6, 7, 8, 9, 10], bgTheme: 'asteroid', musicTheme: 'gameplay', hazardTheme: 'asteroid', palette: ['#3a2a1a', '#ff8844', '#aa6644'], enemyBonus: ['weaver', 'kamikaze'], desc: 'Dodging tumbling rocks' },
        { id: 'crystal', name: 'CRYSTAL CAVES', stages: [11, 12, 13, 14, 15], bgTheme: 'crystal', musicTheme: 'gameplay', hazardTheme: 'crystal', palette: ['#0a2a3a', '#88ccff', '#aaffee'], enemyBonus: ['splitter', 'shield_bee'], desc: 'Glittering crystal formations' },
        { id: 'storm', name: 'ION STORM', stages: [16, 17, 18, 19, 20], bgTheme: 'storm', musicTheme: 'deep_boss', hazardTheme: 'storm', palette: ['#1a1a2a', '#ffff44', '#88ccff'], enemyBonus: ['carrier', 'teleporter'], desc: 'Electromagnetic chaos' },
        { id: 'blackhole', name: 'BLACK HOLE', stages: [21, 22, 23, 24, 25], bgTheme: 'blackhole', musicTheme: 'deep_boss', hazardTheme: 'blackhole', palette: ['#0a0011', '#8844ff', '#cc66ff'], enemyBonus: ['weaver', 'splitter', 'kamikaze'], desc: 'Gravitational maelstrom' },
        { id: 'void', name: 'THE VOID', stages: [26, 27, 28, 29, 30], bgTheme: 'nebula', musicTheme: 'deep_boss', hazardTheme: 'nebula', palette: ['#000000', '#ffffff', '#ff44ff'], enemyBonus: ['teleporter', 'carrier', 'kamikaze', 'splitter'], desc: 'Beyond the known' }
    ];
    GC.getBiomeForStage = function (stage) { return GC.BIOMES[GC.getStagePlan(stage).sector]; };

    // NEW: Super / Overdrive definitions — unique per ship. Meter fills from kills+combo+parry.
    GC.SUPER_DEFS = {
        classic: { name: 'NOVA BARRAGE', desc: 'Screen-wide salvo', col: '#ffcc00', dur: 2000, meterMax: 100 },
        interceptor: { name: 'PHASE DASH', desc: 'i-frames + dash trail', col: '#00ffcc', dur: 1800, meterMax: 100 },
        heavy: { name: 'AEGIS CANNON', desc: 'Massive front beam', col: '#ff4444', dur: 2200, meterMax: 100 },
        stealth: { name: 'SHADOW CLONE', desc: '3 clones fire in sync', col: '#ff88ff', dur: 2500, meterMax: 100 }
    };
    GC.SUPER_METER_GAIN = { kill: 2, combo: 1, parry: 8, headshot: 4 };
    GC.SUPER_COST = 100; // full meter to activate

    GC.ENEMY_SPAWN_DURATION = 400;

    // Per-type movement visuals: pulse scale, dive trails, blink invisibility
    GC.ENEMY_MOTION_FX = {
        default: { pulse: { amp: 0.04, speed: 1.6 } },
        bee: { pulse: { amp: 0.09, speed: 2.4 } },
        butterfly: { pulse: { amp: 0.11, speed: 2.0 } },
        boss: { pulse: { amp: 0.05, speed: 1.2 }, diveScale: { min: 0.94, max: 1.08 } },
        miniboss: { pulse: { amp: 0.06, speed: 1.3 }, diveScale: { min: 0.94, max: 1.1 } },
        hunter: { pulse: { amp: 0.05, speed: 3 }, diveTrail: true, diveScale: { min: 0.9, max: 1.2 } },
        stalker: { blink: { on: 1500, off: 900 }, diveTrail: true },
        kamikaze: { pulse: { amp: 0.08, speed: 5.5 }, diveTrail: true, diveScale: { min: 0.86, max: 1.3 } },
        spinner: { pulse: { amp: 0.15, speed: 6.2 } },
        bomber: { pulse: { amp: 0.08, speed: 1.4 }, diveTrail: true },
        lasher: { pulse: { amp: 0.07, speed: 2.8 }, diveTrail: true },
        weaver: { pulse: { amp: 0.16, speed: 3.8 } },
        splitter: { pulse: { amp: 0.13, speed: 4.6 } },
        shield_bee: { pulse: { amp: 0.1, speed: 2.6 } },
        carrier: { pulse: { amp: 0.06, speed: 1.5 }, diveScale: { min: 0.95, max: 1.12 } },
        teleporter: { pulse: { amp: 0.08, speed: 3 }, blink: { on: 300, off: 2100 }, teleportBlink: true },
        sniper: { pulse: { amp: 0.04, speed: 1.2 }, blink: { on: 140, off: 3400, blinkAim: true } }
    };

    // Sprite heading (galaxa-enemy-motion.js): `sway` = idle breathing roll in formation,
    // `bank` = max lean into lateral motion while flying, `face` = nose follows the
    // velocity fully (homing types), `spin` = continuous rotation rad/s, `rate` = smoothing.
    GC.ENEMY_HEADING = {
        default: { sway: 0.06, swaySpeed: 1.4, bank: 0.42, rate: 7 },
        bee: { sway: 0.08, swaySpeed: 2.2, bank: 0.5, rate: 8 },
        butterfly: { sway: 0.1, swaySpeed: 1.8, bank: 0.55, rate: 7 },
        shield_bee: { sway: 0.07, swaySpeed: 2.0, bank: 0.45, rate: 8 },
        hunter: { sway: 0.04, swaySpeed: 1.2, bank: 0.3, rate: 9, face: true },
        stalker: { sway: 0.05, swaySpeed: 1.0, bank: 0.3, rate: 8, face: true },
        kamikaze: { sway: 0.03, swaySpeed: 2.6, bank: 0.3, rate: 11, face: true },
        spinner: { sway: 0, swaySpeed: 0, bank: 0, rate: 6, spin: 2.4 },
        teleporter: { sway: 0.12, swaySpeed: 3.0, bank: 0.4, rate: 7 },
        sniper: { sway: 0.02, swaySpeed: 0.8, bank: 0.25, rate: 6 },
        carrier: { sway: 0.03, swaySpeed: 0.9, bank: 0.2, rate: 5 },
        bomber: { sway: 0.05, swaySpeed: 1.1, bank: 0.35, rate: 6 },
        weaver: { sway: 0.09, swaySpeed: 2.4, bank: 0.5, rate: 8 },
        boss: { sway: 0.03, swaySpeed: 0.9, bank: 0.18, rate: 5 },
        miniboss: { sway: 0.03, swaySpeed: 1.0, bank: 0.2, rate: 5 }
    };

    // NEW: Enemy unpredictability — per-enemy behaviour quirks plus extra dive
    // path profiles. Gameplay decisions must go through ctx.random (seeded) so
    // runs stay reproducible per seed; visual-only randomness may use Math.random.
    // Quirks are never applied on easy difficulty and unlock by stage, so the
    // surprise curve grows with the player's skill and difficulty.
    GC.ENEMY_QUIRKS = [
        // Finte: dives a short way, then aborts and climbs back (never lethal by contact)
        { id: 'feint', stageMin: 3, weight: 3, types: ['bee', 'butterfly', 'sniper', 'spinner', 'bomber', 'lasher', 'weaver', 'splitter', 'shield_bee', 'carrier', 'teleporter'] },
        // Scheinangriff: long telegraph, then a laterally jittered shot
        { id: 'fakeout', stageMin: 4, weight: 3 },
        // Ausweichend: only joins the attack when the player is close
        { id: 'shy', stageMin: 5, weight: 2, types: ['bee', 'butterfly', 'shield_bee'] },
        // Salve: a normally harmless type returns fire once
        { id: 'volley', stageMin: 6, weight: 2, types: ['bee', 'shield_bee'] }
    ];
    GC.QUIRK_CHANCE = { easy: 0, normal: 0.2, hard: 0.36 }; // fraction of spawned enemies that receive a quirk
    GC.FEINT = { minMs: 700, maxMs: 1250, chance: 0.55, speedMult: 0.78 };
    GC.SQUAD_DIVE = { stageMin: 5, chance: 0.24, minSize: 3, maxSize: 5, staggerMs: 150 };
    GC.REVENGE_DIVE = { stageMin: 6, chance: 0.32, max: 2, radius: 88, telegraphMs: 180, rageMs: 1600, rageMult: 1.35 };
    // Occasionally two neighbouring enemies swap formation slots (with a puff),
    // so the formation never reads as a fully static wall.
    GC.FORMATION_SHUFFLE = { stageMin: 7, chance: 0.05 };
    // Dive path shapes; later stages unlock the wilder shapes. `weight` biases
    // the seeded pick inside GC.createEntitiesBehaviors pickDivePath().
    GC.DIVE_PATHS = {
        sine: { stageMin: 1, weight: 4 },
        zigzag: { stageMin: 3, weight: 3 },
        arc: { stageMin: 4, weight: 3 },
        bounce: { stageMin: 5, weight: 2 },
        weave: { stageMin: 6, weight: 2 },
        loop: { stageMin: 4, weight: 2 },
        strafe: { stageMin: 5, weight: 2 },
        hover: { stageMin: 6, weight: 2 }
    };
    // Rage aura + biome tint colours for enemy motion FX / auras (galaxa-enemy-fx.js).
    GC.ENEMY_RAGE_COL = '#ff4a2a';
    GC.ENEMY_BIOME_TINT = { nebula: '#88ccff', asteroid: '#ffaa66', crystal: '#aaffee', storm: '#ffe066', blackhole: '#c59bff', void: '#ff88ff' };

    // Smoldering crash dives (galaxa-enemy-fx.js): a hit enemy sometimes does
    // not explode at once but tumbles toward the player, trailing smoke and
    // embers, and detonates after a seeded random delay. Seeded per delay/count
    // so fixed-step runs stay deterministic; wobble phases are visual-only.
    GC.SMOLDER = {
        stageMin: 1, chance: 0.28, easyChance: 0.18, hardBonus: 0.12,
        minMs: 650, maxMs: 1450,          // detonation delay after ignition
        maxSimultaneous: 4,               // cap of active smolder divers
        fallBias: 0.52,                  // steering weight toward the player
        baseFall: 135,                   // downward drift px/s
        minSpin: 0.65,                   // gentle bank/roll minimum rad/s
        maxSpin: 2.35,                   // gentle tumble maximum rad/s
        contactRadius: 20,               // kills the player on touch
        // Distinct crash-dive archetypes (motion, rotation, smoke/spark profile, fuse).
        variants: [
            { id: 'glide', smoke: 1.5, sparks: 0.75, spinMult: 0.58, swayAmp: 22, swayFreq: 2.3, fallMult: 0.88, steerMult: 0.32, minMs: 650, maxMs: 1250, tint: '#9aa4b2', ringCol: '#ff9a44' },
            { id: 'spiral', smoke: 0.95, sparks: 1.35, spinMult: 1.0, swayAmp: 68, swayFreq: 6.2, fallMult: 1.0, steerMult: 0.45, minMs: 720, maxMs: 1380, tint: '#ff9a55', ringCol: '#ff7a33' },
            { id: 'sputter', smoke: 1.15, sparks: 1.5, spinMult: 0.72, swayAmp: 26, swayFreq: 3.6, fallMult: 0.94, steerMult: 0.4, sputterMs: 240, minMs: 780, maxMs: 1450, tint: '#99e6ff', ringCol: '#66d4ff' },
            { id: 'ember', smoke: 0.9, sparks: 1.3, spinMult: 0.85, swayAmp: 32, swayFreq: 4.0, fallMult: 1.12, steerMult: 0.78, minMs: 680, maxMs: 1320, tint: '#ff7733', ringCol: '#ff5522' }
        ]
    };

    // NEW: Parry tuning
    GC.PARRY_WINDOW = 120;     // ms active parry window
    GC.PARRY_COOLDOWN = 600;   // ms cooldown after window ends
    GC.PARRY_RADIUS = 38;      // px radius around player (scaled 1.33x for 32×32 player)

    // NEW: Explosion profiles per enemy type (flash size, shockwave strength, debris count, color set)
    GC.EXPLOSION_PROFILE = {
        bee: { flashR: 10, shockR: 30, debris: 6, smoke: 4, sparks: 10, cols: ['#ffcc00', '#ffaa00', '#ffee88'] },
        butterfly: { flashR: 10, shockR: 30, debris: 6, smoke: 4, sparks: 10, cols: ['#ff3366', '#ff6688', '#ff88aa'] },
        stalker: { flashR: 12, shockR: 36, debris: 7, smoke: 5, sparks: 12, cols: ['#6622aa', '#8844cc', '#aa66ee'] },
        sniper: { flashR: 10, shockR: 32, debris: 6, smoke: 4, sparks: 12, cols: ['#ffcc00', '#ffaa00', '#ffff44'] },
        hunter: { flashR: 14, shockR: 40, debris: 8, smoke: 6, sparks: 14, cols: ['#ff6600', '#ff8844', '#ffaa00'] },
        spinner: { flashR: 12, shockR: 36, debris: 7, smoke: 5, sparks: 14, cols: ['#00cccc', '#44ffff', '#88ffff'] },
        bomber: { flashR: 12, shockR: 38, debris: 7, smoke: 6, sparks: 12, cols: ['#aa44cc', '#cc66ff', '#ff44aa'] },
        lasher: { flashR: 10, shockR: 32, debris: 6, smoke: 4, sparks: 10, cols: ['#44ff88', '#00cc66', '#aaffcc'] },
        weaver: { flashR: 12, shockR: 34, debris: 7, smoke: 5, sparks: 12, cols: ['#ff8844', '#ffaa66', '#ffcc88'] },
        splitter: { flashR: 12, shockR: 34, debris: 8, smoke: 5, sparks: 12, cols: ['#88ff44', '#aaff66', '#ccff88'] },
        shield_bee: { flashR: 11, shockR: 32, debris: 6, smoke: 4, sparks: 11, cols: ['#ffcc00', '#ffdd44', '#ffee88'] },
        kamikaze: { flashR: 16, shockR: 44, debris: 9, smoke: 7, sparks: 16, cols: ['#ff2222', '#ff4444', '#ff6666'] },
        carrier: { flashR: 14, shockR: 42, debris: 9, smoke: 7, sparks: 14, cols: ['#cc88ff', '#ddaaff', '#eeccff'] },
        teleporter: { flashR: 13, shockR: 38, debris: 7, smoke: 5, sparks: 14, cols: ['#44ffff', '#66ffff', '#88ffff'] },
        boss: { flashR: 30, shockR: 80, debris: 16, smoke: 14, sparks: 28, cols: ['#ffcc00', '#ff8800', '#ff4444', '#fff'] },
        miniboss: { flashR: 26, shockR: 70, debris: 14, smoke: 12, sparks: 24, cols: ['#ffcc00', '#ff8800', '#ff4444'] }
    };

    // NEW: FX module tuning (galaxa-fx.js) — per-particle-setting caps so every
    // new effect scales with the PARTICLES setting (low/medium/high).
    GC.FX_CAPS = {
        low: { sparkCone: 4, sparkle: 8, glint: 3, ghost: 4, streak: 10, ring: 2, graze: 2, fireTrail: 4, confetti: 8, deathFlash: 2, muzzle: 2, respawn: 6, telegraph: 2, enemyMarks: 10, chain: 4, smolder: 2 },
        medium: { sparkCone: 7, sparkle: 14, glint: 5, ghost: 6, streak: 18, ring: 3, graze: 4, fireTrail: 8, confetti: 16, deathFlash: 3, muzzle: 3, respawn: 10, telegraph: 3, enemyMarks: 18, chain: 6, smolder: 4 },
        high: { sparkCone: 10, sparkle: 20, glint: 8, ghost: 8, streak: 26, ring: 3, graze: 6, fireTrail: 12, confetti: 28, deathFlash: 5, muzzle: 4, respawn: 14, telegraph: 4, enemyMarks: 28, chain: 8, smolder: 6 }
    };
    GC.FX_WARP_DUR = 1400;      // ms warp speed-line streaks stay active after a stage advance
    GC.FX_GHOST_INTERVAL = 90;  // ms between ship afterimage snapshots
    GC.FX_GHOST_LIFE = 260;     // ms afterimage fade time
    GC.FX_GHOST_COL = '#66ddff'; // afterimage tint while speed-boosted (parry uses white)
    GC.FX_GRAZE_RADIUS = 16;    // px — graze trigger distance from player center to bullet edge
    GC.FX_GRAZE_PART_COUNT = 3; // particles per graze tick
    GC.FX_GRAZE_COOLDOWN = 80;  // ms between graze triggers
    GC.FX_FIRE_TRAIL_COMBO = 5; // combo threshold for fire trail activation
    GC.FX_FIRE_TRAIL_INTERVAL = 45; // ms between fire trail drops
    GC.FX_FIRE_TRAIL_LIFE = 500; // ms fire trail particle lifetime
    GC.FX_MAGNET_PULL_COL = '#ff44ff'; // magnet pull-line color
    GC.FX_MAGNET_PULL_LINES = 6; // pull-lines drawn

    // NEW: Combat juice tuning (super-ready cue, last-life alarm, respawn
    // teleport ring, multi-kill cluster) — see galaxa-fx.js
    GC.FX_SUPER_READY_DUR = 900;    // ms gold pulse ring when the super meter fills
    GC.FX_LASTLIFE_INTERVAL = 1200; // ms heartbeat cadence on the last life
    GC.FX_MULTIKILL_WINDOW = 600;   // ms window for multi-kill clusters
    GC.FX_MULTIKILL_COUNT = 3;      // kills needed within the window
    GC.FX_MULTIKILL_HITSTOP = 60;   // ms hitstop applied by the multi-kill sting

    // NEW: Bonus sub-stage schedule (every 4 stages before boss)
    GC.BONUS_STAGE_EVERY = 4;
    GC.BONUS_STAGE_DURATION = 20000; // 20s

    // NEW: Stage archetypes
    GC.ARCHETYPES = {
        swarm_wave: {
            name: 'SWARM WAVE',
            duration: 30000,
            spawnRate: 0.08,
            enemyCount: 30,
            puDropRate: 0.5,
            hue: '#88ff44'
        },
        escort: {
            name: 'ESCORT',
            duration: 60000,
            vipHp: 5,
            enemyRate: 0.02,
            reward: 5000,
            hue: '#ffcc00'
        },
        asteroid_field: {
            name: 'ASTEROID FIELD',
            duration: 45000,
            asteroidRate: 0.05,
            reflectDamage: true,
            hue: '#aa6644'
        }
    };
    GC.ARCHETYPE_SCHEDULE = ['swarm_wave', null, null, 'escort', null, 'asteroid_field', null, null, 'swarm_wave', null, 'escort', null, 'asteroid_field', 'swarm_wave', null, 'escort', null, null, 'swarm_wave', null];

    GC.DEMO_DECISION_MS = 50;
    GC.DEMO_DODGE_LOOKAHEAD = 18;
    GC.DEMO_DODGE_RADIUS = 38;
    GC.DEMO_MENU_TAP_MS = 180;
    GC.DEMO_AIM_DEADZONE = 6;
})();
