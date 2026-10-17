import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';
const images = [], notices = [];
const element = () => ({ append() {}, setAttribute() {}, addEventListener() {}, remove() {}, className: '', textContent: '' });
const sandbox = vm.createContext({ window: { BUILD_VERSION: 'test-version' }, Math, Set, Map,
    localStorage: { setItem() {} }, document: { createElement: element },
    Image: class { constructor() { images.push(this); } set src(value) { this.url = value; } }
});
for (const name of ['constants', 'content', 'campaign', 'entities-rewards', 'entities-weapons', 'asteroids', 'planets']) {
    vm.runInContext(readFileSync(new URL(`../ui/js/desktop/apps/galaxa-${name}.js`, import.meta.url), 'utf8'), sandbox, { filename: name });
}
const GC = sandbox.window.GalaxaCore;
const enemy = (x = 100, y = 100, hp = 1, extra = {}) => ({ x, y, hp, maxHp: hp, st: 'FORM', type: 'bee', ...extra });
function context() {
    const c = Object.create(GC);
    Object.assign(c, { settings: { mode: 'classic', diff: 'normal', reducedMotion: false }, state: { disposed: false },
        G: { stage: 6, simTime: 0, runGeneration: 1, st: 'PLAYING', p: { x: 270, y: 650, alive: true, inv: 0 },
            enemies: [], playerMines: [], bul: [], ebul: [], exp: [], asteroids: [], score: 0, kills: 0, combo: 0,
            credits: 0, killCount: 0, stageKills: 0, weaponXP: 0, weaponLv: 1, scorePopups: [], combatText: [] },
        SFX: new Proxy({}, { get: () => () => {} }), maybeSmolder: () => false,
        addScore(n) { c.G.score += n; }, registerKill() { c.G.kills++; }, dropPU() { c.drops = (c.drops || 0) + 1; },
        unlockAchievement() {}, grantWeaponLevel() {}, checkWeaponEvo() {}, boom() {},
        isMiniBossStage: () => false, getShipHitbox: () => ({ x: c.G.p.x - 5, y: c.G.p.y - 5, w: 10, h: 10 }),
        killP() { c.G.p.inv = 1000; c.contacts = (c.contacts || 0) + 1; }, t: key => key, wrapEl: { appendChild: n => notices.push(n) }
    });
    GC.createCampaign(c); GC.createExpansionContent(c); GC.createEntitiesWeapons(c); GC.createEnemyRewards(c); GC.createAsteroids(c);
    c.seedRun(112233); return c;
}
{
    const c = context(), e = enemy(); c.G.enemies = [e];
    c.pushPlayerMine({ x: 100, y: 100, vy: 0 });
    c.updatePlayerMines(449, 0); assert.equal(e.hp, 1, 'mine must not arm early');
    c.updatePlayerMines(1, 0); assert.equal(e.st, 'DEAD'); assert.equal(c.G.playerMines.length, 0);
    assert.equal(c.G.kills, 1); assert.equal(c.drops, 1); assert.equal(c.G.killCount, 1);
    c.finishEnemyKill(e); c.areaDamage(100, 100, 72, 1); assert.equal(c.G.kills, 1, 'terminal kill is idempotent');
    for (let i = 0; i < 30; i++) c.pushPlayerMine({ x: 10, y: 10, vy: 0 });
    assert.equal(c.G.playerMines.length, 12); c.updatePlayerMines(9000, 0); assert.equal(c.G.playerMines.length, 0);
}
{
    const c = context(), e = enemy(), boss = enemy(110, 100, 20, { type: 'miniboss', sectorBoss: 'nebula', bossPhase: 3 });
    c.G.enemies = [e, boss]; c.G.ebul = [{ x: 110, y: 100 }, { x: 300, y: 300 }];
    c.pushPlayerMine({ x: 100, y: 100, vy: 0, weapon: 'emp_mine_layer' }); c.updatePlayerMines(450, 0);
    assert.equal(e.hp, 1); assert.equal(e.empUntil, 1500); assert.equal(boss.empUntil, undefined); assert.equal(boss.hp, 20);
    assert.equal(c.G.ebul.length, 1); assert.equal(c.G.ebul[0].x, 300);
}
{
    const c = context(); c.G.enemies = [enemy()];
    c.pushPlayerMine({ x: 100, y: 100, vy: 0, weapon: 'cluster_mine_layer' }); c.updatePlayerMines(450, 0);
    assert.equal(c.G.bul.length, 6); assert.ok(c.G.bul.every(b => b.mineShard && !b.rocket && b.life === 600)); assert.equal(c.G.kills, 1);
}
{
    const c = context(); c.G.enemies = [enemy(100), enemy(200), enemy(300), enemy(110, -20), enemy(110, 110, 1, { invulnerable: true })];
    c.G.activePU = { type: 'swarm_launcher' }; c.fireRocketSalvo(false);
    assert.equal(c.G.bul.length, 3); assert.equal(new Set(c.G.bul.map(b => b.target)).size, 3);
    assert.ok(c.G.bul.every(b => b.dmg === 1 && b.life === 6000));
    const before = c.G.enemies.map(e => e.hp); c.rocketImpact(c.G.bul[0], c.G.bul[0].target);
    assert.deepEqual(c.G.enemies.map(e => e.hp), before, 'swarm has no splash');
    const b = c.G.bul[0]; b.target.st = 'DEAD'; assert.equal(c.isSeekerTarget(b.target), false);
    assert.equal(c.isSeekerTarget(enemy(100, 100, 1, { mSkipDraw: true })), false, 'invisible aliens cannot be acquired');
    assert.ok(c.findNearestEnemy(b.x, b.y));
    c.G.mirrorActive = true; c.modesIsMirrorPermanent = () => true; c.mirrorDuplicateBullets(0);
    assert.equal(c.G.bul.length, 6); assert.equal(c.G.bul[3].weapon, 'swarm_launcher'); assert.equal(c.G.bul[3].dmg, 1);
    c.modesMirrorGhostDamageMult = () => 0; c.random = () => 1; assert.equal(c.shouldApplyPlayerHit(c.G.bul[3]), false);
}
{
    const c = context(), direct = enemy(100, 100, 10), splash = enemy(130, 100, 2), protectedBoss = enemy(140, 100, 100, { sectorBoss: 'void', bossPhase: 3, invulnerable: true });
    c.G.enemies = [direct, splash, protectedBoss]; c.G.activePU = { type: 'seeker_torpedo' }; c.fireRocketSalvo(false);
    const b = c.G.bul[0]; assert.equal(b.dmg, 4); assert.equal(b.speed, 210); b.x = b.y = 100;
    c.damageEnemy(direct, b.dmg); c.rocketImpact(b, direct);
    assert.equal(direct.hp, 6); assert.equal(splash.hp, 1); assert.equal(protectedBoss.hp, 100);
}
{
    const c = context(); c.startAsteroids(); const a = c.spawnAsteroid();
    assert.ok(a.y + a.r < 0); assert.ok(a.warning >= 700); assert.ok(Math.abs(a.x - c.G.asteroidLane) >= 80 + a.r);
    a.warning = 0; a.y = 100; a.size = 2; a.hp = 6; a.r = 31;
    assert.equal(c.damageAsteroid(a, 6), true); assert.equal(c.G.score, 100);
    const fragments = c.G.asteroids.filter(o => o.fragment); assert.equal(fragments.length, 2);
    fragments.forEach(f => c.damageAsteroid(f, 99)); assert.equal(c.G.score, 100); assert.equal(c.G.asteroids.filter(o => o.hp > 0).length, 0);
    c.G.asteroids = []; for (let i = 0; i < 60; i++) c.spawnAsteroid(); assert.ok(c.G.asteroids.length <= 20);
    const large = c.G.asteroids.filter(a => a.size === 2);
    for (const a of large) { a.warning = 0; const before = c.G.asteroids.length; c.damageAsteroid(a, a.hp); assert.equal(c.G.asteroids.length, before + 1); assert.ok(c.G.asteroids.length <= 20, 'fragment reservation respects pool cap'); }
    for (const a of c.G.asteroids) { a.warning = 0; a.y = 100; }
    const first = c.G.asteroids[0]; assert.equal(c.hitAsteroidEnemyShot({ x: first.x, y: first.y, w: 3, h: 3 }), true);
    first.hp = 10;
    const shot = { x: first.x, y: first.y, w: 3, h: 3, dmg: 1, pierce: true };
    c.hitAsteroidShot(shot); const hitCount = c.G.stageAccuracyHits; c.hitAsteroidShot(shot);
    assert.equal(c.G.stageAccuracyHits, hitCount, 'piercing hit counted only once'); assert.equal(first.hp, 9);
    c.G.stage = 5; c.startAsteroids(); assert.equal(c.spawnAsteroid(), null);
    c.G.stage = 6; c.G.chal = true; c.startAsteroids(); assert.equal(c.spawnAsteroid(), null);
    c.G.chal = false; c.G.encounterBoss = {}; c.startAsteroids(); assert.equal(c.spawnAsteroid(), null);
}
{
    const c = context(); c.G.stage = 1;
    assert.equal(c.expansionAlien('bee', 2, 1), null); assert.equal(c.expansionAlien('bee', 2, 2).id, 'nebula-ray');
    assert.equal(c.expansionAlien('bee', 5, 2).id, 'spore-hunter'); assert.equal(c.expansionAlien('boss', 2, 2), null);
    c.settings.mode = 'gauntlet';
    for (let stage = 1; stage <= 12; stage++) { c.G.stage = stage; assert.equal(c.expansionSector(), GC.SECTORS[Math.floor((stage - 1) / 2)].id); assert.ok(c.expansionAlien('bee', 2, 1)); }
}
{
    const c = context(); c.G.stage = 1; c.spriteAssets = { manifest: { sheets: {}, animations: {} } }; GC.ART_VERSION = 'test-version';
    for (const id of ['ocean', 'terran']) {
        for (const part of ['a', 'b', 'c']) c.spriteAssets.manifest.sheets[id + part] = { file: id + part + '.png', width: 1024, height: 1024, optional: 'planet' };
        c.spriteAssets.manifest.animations['planet.' + id] = { frames: Array.from({ length: 12 }, () => [0, 0, 512, 512]), ms: 2000,
            frameSheets: Array.from({ length: 12 }, (_, i) => id + ['a', 'b', 'c'][Math.floor(i / 4)]) };
    }
    GC.createPlanets(c); c.updatePlanets(0); assert.equal(c.planetCache.size, 1);
    let slot = [...c.planetCache.values()][0]; assert.ok(slot.image.url.endsWith('?v=test-version'));
    slot.image.naturalWidth = slot.image.naturalHeight = 1024; slot.image.onload(); assert.equal(slot.ready, true);
    for (let i = 0; i < 700; i++) { c.updatePlanets(0.05); for (const slot of c.planetCache.values()) if (!slot.ready) { slot.image.naturalWidth = slot.image.naturalHeight = 1024; slot.image.onload(); } assert.ok(c.planetCache.size <= 2); }
    c.G.stage = 2; c.updatePlanets(0.01); assert.ok(c.planetCache.size <= 2, 'transition keeps the two-sheet cap');
    const delayed = [...c.planetCache.values()].find(s => !s.ready); assert.ok(delayed); const onload = delayed.image.onload;
    c.G.runGeneration++; c.resetPlanets(); onload(); assert.equal(c.planetCache.size, 0); assert.equal(delayed.ready, false, 'late result ignored after reset');
    c.updatePlanets(0); slot = [...c.planetCache.values()][0]; slot.image.onerror(); assert.equal(slot.failed, true); assert.ok(notices.length);
    c.state.disposed = true; c.disposePlanets(); assert.equal(c.planetCache.size, 0);
}
console.log('Galaxa expansion: mine arming/caps/expiry, EMP, shards, swarm targets, torpedo, Mirror, once-only rewards, asteroid fragmentation/corridor, sector slots and planet cache/lifecycle passed.');
