async () => {
    const c = game, g = c.G, assert = (ok, message) => { if (!ok) throw Error(message); };
    cancelAnimationFrame(c.rafId); c.GalagaMusic.stop(); c.settings.mute = true; g.muted = true;
    const all = c.spriteAssets.manifest.expansion.motifs;
    assert(all.length === 50, 'expansion must contain exactly 50 motifs');
    const seen = new Set();
    for (const mode of ['classic', 'mirror', 'daily', 'endless', 'hyperdrive', 'gauntlet', 'boss_rush']) {
        c.settings.mode = mode; c.resetRun(921);
        const count = mode === 'gauntlet' ? 12 : mode === 'boss_rush' ? 6 : 30;
        for (let stage = 1; stage <= count; stage++) {
            g.stage = stage; c.startStage();
            if (c.isCampaign() && !g.chal && !g.encounterBoss) c.spawnCampaignWave();
            for (const e of g.enemies) if (e.artId) { seen.add(e.artId); assert(c.spriteAssets.manifest.animations[c.enemySpriteFor(e).sp.key], 'missing alien state'); }
            if (g.chal || g.bonusStage || g.encounterBoss || mode === 'boss_rush') assert(!g.asteroidEncounter, 'asteroid in excluded encounter');
            assert(g.enemies.every(e => !e.artId || GalaxaCore.EXPANSION_ALIENS[c.expansionSector()].some(a => a.id === e.artId && a.role === e.type)), 'alien role identity mismatch');
        }
    }
    assert(seen.size === 12, 'not all twelve aliens occur in real formations');
    let dailySignature;
    for (const fps of [30, 60, 144]) {
        c.settings.mode = 'daily'; c.resetRun(20261017); g.stage = 7; c.startStage(); g.st = 'PLAYING'; g.p.inv = 1e8; g.kb.f = true;
        c.collectPU({ type: 'swarm_launcher', x: g.p.x, y: g.p.y });
        c.scheduleGame(() => c.collectPU({ type: 'cluster_mine_layer', x: g.p.x, y: g.p.y }), 4500);
        for (let i = 0; i < fps * 10; i++) { c.stepFrame(1 / fps); c.renderFrame(); }
        const signature = JSON.stringify({ score: g.score, time: g.simTime, enemies: g.enemies.map(e => [e.type, e.artId, e.st, e.hp, e.x, e.y]),
            bullets: g.bul.map(b => [b.x, b.y, b.weapon]), mines: g.playerMines, asteroids: g.asteroids });
        if (dailySignature && signature !== dailySignature) {
            const old = JSON.parse(dailySignature), next = JSON.parse(signature);
            for (const key of Object.keys(next)) assert(JSON.stringify(old[key]) === JSON.stringify(next[key]), 'Daily expansion differs at ' + fps + ' FPS in ' + key + ': ' + JSON.stringify([old[key], next[key]]));
        } else dailySignature = signature;
    }
    c.settings.mode = 'classic';
    const weapons = ['mine_layer', 'emp_mine_layer', 'cluster_mine_layer', 'rocket_launcher', 'swarm_launcher', 'seeker_torpedo'];
    for (const type of weapons) {
        c.resetRun(123); c.startStage(); g.st = 'PLAYING'; g.p.inv = 1e6; g.enemies = [];
        c.collectPU({ type, x: g.p.x, y: g.p.y });
        assert(g.activePU.type === type, 'pickup failed ' + type);
        const def = c.EXPANSION_WEAPONS[type]; assert(g.puTimer === (def.mine ? 10000 : 12000), 'duration ' + type);
        if (def.mine) {
            c.dropPlayerMines(1 / 60); assert(g.playerMines.length === 1 && g.playerMines[0].armT === 450, 'automatic mine ' + type);
            c.renderFrame();
        } else {
            const a = c.newEnemy('bee', 140, 100, 0, 10), b = c.newEnemy('bee', 300, 100, 1, 10);
            Object.assign(a, { x: 140, y: 100, st: 'FORM' }); Object.assign(b, { x: 300, y: 100, st: 'FORM' }); g.enemies = [a, b];
            c.fire(2000); assert(g.bul.length === def.count, 'rocket count ' + type);
            const before = g.bul.length; c.fire(2000 + def.interval - 1); assert(g.bul.length === before, 'early rocket cooldown');
            c.fire(2000 + def.interval); assert(g.bul.length === before + def.count, 'rocket interval ' + type);
            for (const bullet of g.bul) bullet.target = a;
            a.invulnerable = true; c.updateBul(1 / 60);
            assert(g.bul.every(bullet => bullet.target === b), 'reacquire vulnerable target ' + type);
            g.enemies = []; c.updateBul(6); assert(!g.bul.some(bullet => bullet.homing), 'orphan seeker expiry');
        }
        if (!c.PU_UPGRADE[type]) { g.puTimer = 10; c.collectPU({ type, x: g.p.x, y: g.p.y }); assert(g.activePU.type === type && g.puTimer === c.PU_DUR[type], 'repeat refresh ' + type); }
    }
    c.resetRun(15); g.stage = 7; c.startStage(); g.st = 'STAGE_INTRO';
    const normal = c.newEnemy('bee', 100, 100, 0, 10), slowed = c.newEnemy('bee', 100, 100, 0, 10);
    for (const e of [normal, slowed]) Object.assign(e, { st: 'FORM', x: 100, y: 100, dTmr: 1e6 });
    slowed.empUntil = 1500; g.enemies = [normal, slowed]; g.fTmr = 0; c.updateE(0.1);
    assert(Math.abs((slowed.x - 100) / (normal.x - 100) - 0.5) < 0.001, 'EMP does not halve formation movement');
    c.resetRun(15); g.stage = 7; c.startStage(); g.st = 'PLAYING'; g.p.inv = 1e6;
    for (let i = 0; i < 120; i++) c.stepFrame(1 / 60);
    assert(g.asteroids.length > 0, 'asteroid encounter did not spawn');
    const rock = g.asteroids[0]; rock.warning = 0; rock.x = rock.y = 100;
    g.ebul = [{ x: 100, y: 100, w: 3, h: 3, vx: 0, vy: 1, kind: 'mine', fuse: 0 }];
    c.updateBul(1 / 60); assert(g.ebul.length === 0, 'blocked enemy mine released fragments');
    const before = g.asteroids[0].y; g.st = 'PAUSED'; c.stepFrame(0.1); assert(g.asteroids[0].y === before, 'pause moved asteroid');
    g.st = 'PLAYING'; c.renderFrame();
    const localImages = { ...c.spriteAssets.images };
    await Promise.all(Object.entries(c.spriteAssets.manifest.sheets).filter(([, s]) => s.optional).map(([id, s]) => new Promise((resolve, reject) => {
        const image = new Image(); image.onload = () => { localImages[id] = image; resolve(); }; image.onerror = reject;
        image.src = '/img/galaxa/' + s.file + '?v=' + GalaxaCore.ART_VERSION;
    })));
    const gallery = document.createElement('section'); gallery.id = 'expansion-gallery'; gallery.style.cssText = 'background:#071020;color:#dbeeff;padding:12px;display:grid;gap:18px;justify-items:center';
    const canvases = [];
    for (const kind of ['asteroid', 'alien', 'weapon', 'planet', 'effect']) {
        const motifs = all.filter(m => m.kind === kind), canvas = document.createElement('canvas');
        canvas.width = 960; canvas.height = Math.ceil(motifs.length / 3) * 220 + 40; canvas.dataset.kind = kind; canvas.style.cssText = 'max-width:100%;height:auto';
        gallery.appendChild(canvas); canvases.push({ canvas, motifs, kind });
    }
    function drawGallery(time) {
        for (const { canvas, motifs, kind } of canvases) {
            const d = canvas.getContext('2d'); d.fillStyle = '#071020'; d.fillRect(0, 0, canvas.width, canvas.height);
            d.font = 'bold 20px monospace'; d.fillStyle = '#e9ce9a'; d.fillText(kind.toUpperCase(), 12, 26);
            motifs.forEach((m, i) => {
                const x = i % 3 * 320, y = Math.floor(i / 3) * 220 + 40;
                const key = m.animations[0], a = c.spriteAssets.manifest.animations[key];
                const frame = Math.floor(time / (kind === 'planet' ? 180 : a.ms)) % a.frames.length, f = a.frames[frame];
                const image = localImages[a.frameSheets ? a.frameSheets[frame] : a.sheet], anchor = a.sourceAnchors[frame], size = kind === 'planet' ? 175 : kind === 'asteroid' ? 72 : kind === 'alien' ? 66 : 64, ratio = size / a.sourceSize;
                d.imageSmoothingEnabled = kind === 'planet'; d.drawImage(image, ...f, x + 160 - anchor[0] * ratio, y + 100 - anchor[1] * ratio, f[2] * ratio, f[3] * ratio);
                d.fillStyle = '#dbeeff'; d.font = '13px monospace'; d.fillText(String(m.number).padStart(2, '0') + ' ' + m.id, x + 12, y + 199, 300);
            });
        }
    }
    drawGallery(500); document.body.appendChild(gallery);
    window.expansionGallery = { canvases, draw: drawGallery, count: all.length };
    window.expansionGalleryImages = Object.fromEntries(canvases.map(({canvas,kind}) => [kind, canvas.toDataURL('image/png').split(',')[1]]));
    for (const { motifs, kind } of canvases) {
        const canvas = document.createElement('canvas'); canvas.width = 1280; canvas.height = motifs.length * 105;
        const d = canvas.getContext('2d'); d.fillStyle = '#071020'; d.fillRect(0, 0, canvas.width, canvas.height);
        motifs.forEach((m, row) => {
            d.fillStyle = '#dbeeff'; d.font = '13px monospace'; d.fillText(m.id, 8, row * 105 + 16);
            let column = 0;
            for (const key of m.animations) {
                const a = c.spriteAssets.manifest.animations[key];
                a.frames.forEach((f, frame) => {
                    const anchor = a.sourceAnchors[frame], ratio = 68 / a.sourceSize;
                    d.imageSmoothingEnabled = kind === 'planet';
                    d.drawImage(localImages[a.frameSheets ? a.frameSheets[frame] : a.sheet], ...f, column++ * 80 + 40 - anchor[0] * ratio, row * 105 + 61 - anchor[1] * ratio, f[2] * ratio, f[3] * ratio);
                });
            }
        });
        window.expansionGalleryImages[kind + '-frames'] = canvas.toDataURL('image/png').split(',')[1];
    }
    assert(!errors.length, errors.join('\n'));
    return { motifs: all.length, aliensSeen: seen.size, weapons: weapons.length, planetSheets: c.planetCache.size, errors };
}
