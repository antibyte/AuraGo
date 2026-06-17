(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};
    GC.createBackground = function (ctx) {
        const STAR_COLS = ['#ffffff', '#ffeecc', '#ccddff', '#ffcccc', '#ccffcc'];
        const STARS = [];
        for (let i = 0; i < 60; i++) STARS.push({ x: Math.random() * ctx.W, y: Math.random() * ctx.H, sp: 10 + Math.random() * 20, br: 0.15 + Math.random() * 0.3, sz: 1, layer: 0, col: STAR_COLS[Math.floor(Math.random() * STAR_COLS.length)] });
        for (let i = 0; i < 45; i++) STARS.push({ x: Math.random() * ctx.W, y: Math.random() * ctx.H, sp: 30 + Math.random() * 30, br: 0.3 + Math.random() * 0.4, sz: Math.random() > 0.7 ? 2 : 1, layer: 1, col: STAR_COLS[Math.floor(Math.random() * STAR_COLS.length)] });
        for (let i = 0; i < 30; i++) STARS.push({ x: Math.random() * ctx.W, y: Math.random() * ctx.H, sp: 60 + Math.random() * 60, br: 0.5 + Math.random() * 0.5, sz: 2, layer: 2, col: STAR_COLS[Math.floor(Math.random() * STAR_COLS.length)] });
        for (let i = 0; i < 15; i++) STARS.push({ x: Math.random() * ctx.W, y: Math.random() * ctx.H, sp: 100 + Math.random() * 80, br: 0.7 + Math.random() * 0.3, sz: 2, layer: 3, twinkle: Math.random() * 6.28, col: '#ffffff' });
        let shootingStars = [];
        let nebulaCv = null, nebulaColors = [];
        let bgPlanets = [], bgComets = [];

        function initBG() {
            bgPlanets = [];
            const themes = ['nebula', 'asteroid', 'blackhole', 'ringed', 'storm', 'crystal'];
            const theme = themes[(ctx.G.stage - 1) % themes.length];
            ctx.G.bgTheme = theme;
            if (theme === 'asteroid') {
                for (let i = 0; i < 12; i++) bgPlanets.push({ x: Math.random() * ctx.W, y: Math.random() * ctx.H, r: 3 + Math.random() * 10, sp: 6 + Math.random() * 14, col: ['#554433', '#665544', '#443322', '#776655'][Math.floor(Math.random()*4)], type: 'asteroid', rot: Math.random()*6.28 });
            } else if (theme === 'blackhole') {
                bgPlanets.push({ x: ctx.W / 2, y: ctx.H / 3, r: 35, sp: 0, col: '#110022', type: 'blackhole', rotSp: 0.02 });
                for (let i = 0; i < 16; i++) bgPlanets.push({ x: ctx.W / 2 + (Math.random() - 0.5) * 220, y: ctx.H / 3 + (Math.random() - 0.5) * 220, r: 1.5 + Math.random() * 3.5, sp: 12 + Math.random() * 22, col: ['#443366', '#553377', '#332255'][Math.floor(Math.random()*3)], type: 'debris', orbit: Math.random() * 6.28, orbitR: 50 + Math.random() * 90, orbitSp: 0.4 + Math.random() * 1.2 });
            } else if (theme === 'ringed') {
                for (let i = 0; i < 2; i++) bgPlanets.push({ x: Math.random() * ctx.W, y: Math.random() * ctx.H, r: 10 + Math.random() * 12, sp: 2 + Math.random() * 3, col: i === 0 ? '#334466' : '#664433', type: 'planet', atmoCol: i === 0 ? 'rgba(100,160,255,0.12)' : 'rgba(255,160,100,0.12)', ringCol: i === 0 ? 'rgba(180,200,255,0.08)' : 'rgba(255,200,150,0.08)', ringR: 1.6 + Math.random() * 0.4 });
            } else if (theme === 'storm') {
                for (let i = 0; i < 2; i++) bgPlanets.push({ x: Math.random() * ctx.W, y: Math.random() * ctx.H, r: 14 + Math.random() * 10, sp: 1 + Math.random() * 2, col: i === 0 ? '#443322' : '#2a3a2a', type: 'planet', atmoCol: i === 0 ? 'rgba(200,150,80,0.1)' : 'rgba(100,200,120,0.08)', storm: true, stormCol: i === 0 ? 'rgba(180,120,40,0.06)' : 'rgba(80,180,100,0.06)' });
                for (let i = 0; i < 5; i++) bgPlanets.push({ x: Math.random() * ctx.W, y: Math.random() * ctx.H, r: 2 + Math.random() * 4, sp: 20 + Math.random() * 30, col: '#554433', type: 'asteroid' });
            } else if (theme === 'crystal') {
                for (let i = 0; i < 6; i++) bgPlanets.push({ x: Math.random() * ctx.W, y: Math.random() * ctx.H, r: 3 + Math.random() * 6, sp: 4 + Math.random() * 8, col: ['#446688', '#664488', '#448866'][i % 3], type: 'crystal', crystalCol: ['#88ccff', '#ff88cc', '#88ffcc'][i % 3] });
            } else {
                for (let i = 0; i < 4; i++) bgPlanets.push({ x: Math.random() * ctx.W, y: Math.random() * ctx.H, r: 6 + Math.random() * 14, sp: 2 + Math.random() * 4, col: ['#224466', '#446622', '#662244', '#443366'][i % 4], type: 'planet', atmoCol: ['rgba(68,136,255,0.12)', 'rgba(136,255,68,0.08)', 'rgba(255,136,68,0.1)', 'rgba(180,100,255,0.08)'][i % 4] });
            }
            bgComets = [];
        }

        function mkNebula() {
            const cols = [
                ['#1a0033', '#0d1a2e', '#0a2218'], ['#2a0a1a', '#1a1a3e', '#0d2a22'],
                ['#3a1a0a', '#1a2a3a', '#0a3a2a'], ['#0a1a3a', '#2a0a2a', '#1a3a0a'],
                ['#1a2a1a', '#3a0a1a', '#0a0a3a'], ['#1a0a2a', '#0a2a1a', '#2a1a0a'],
                ['#0a2a2a', '#1a0a3a', '#2a2a0a']
            ];
            nebulaColors = cols[(ctx.G.stage - 1) % cols.length];
            nebulaCv = ensureNebulaCanvas();
            const nc = nebulaCv.getContext('2d');
            nc.clearRect(0, 0, ctx.W, ctx.H);
            for (let i = 0; i < 4; i++) {
                const cx = ctx.W * (0.15 + i * 0.22 + Math.random() * 0.1), cy = ctx.H * (0.2 + i * 0.18 + Math.random() * 0.1), r = 100 + i * 35 + Math.random() * 30;
                const gr = nc.createRadialGradient(cx, cy, 0, cx, cy, r);
                gr.addColorStop(0, nebulaColors[i % 3]); gr.addColorStop(0.6, nebulaColors[i % 3] + '66'); gr.addColorStop(1, 'transparent');
                nc.fillStyle = gr; nc.fillRect(0, 0, ctx.W, ctx.H);
            }
        }

        function ensureNebulaCanvas() {
            if (!nebulaCv) {
                nebulaCv = document.createElement('canvas');
            }
            if (nebulaCv.width !== ctx.W) nebulaCv.width = ctx.W;
            if (nebulaCv.height !== ctx.H) nebulaCv.height = ctx.H;
            return nebulaCv;
        }

        function updateBackground(dt) {
            ctx.dt = dt;
            const warp = ctx.G.warpT > 0 ? 10 : 1;
            for (const s of STARS) {
                const lm = s.layer === 0 ? 0.3 : s.layer === 1 ? 0.6 : s.layer === 2 ? 1 : 1.4;
                s.y += s.sp * dt * warp * lm;
                if (s.y > ctx.H) { s.y = 0; s.x = Math.random() * ctx.W; s.col = STAR_COLS[Math.floor(Math.random() * STAR_COLS.length)]; }
                if (s.layer === 3) s.twinkle += dt * 3;
            }
            if (Math.random() < 0.003 && warp <= 1) {
                shootingStars.push({ x: Math.random() * ctx.W, y: -5, vx: -40 - Math.random() * 80, vy: 120 + Math.random() * 100, life: 1.5, t: 0, col: '#ffffff' });
            }
            let sslen = 0;
            for (let i = 0; i < shootingStars.length; i++) {
                const ss = shootingStars[i];
                ss.prevX = ss.x; ss.prevY = ss.y;
                ss.x += ss.vx * dt; ss.y += ss.vy * dt; ss.t += dt;
                if (ss.t < ss.life && ss.y < ctx.H + 10 && ss.x > -50) shootingStars[sslen++] = ss;
            }
            shootingStars.length = sslen;
            for (const p of bgPlanets) {
                if (p.type === 'blackhole') p.rotSp = (p.rotSp || 0) + dt;
                else if (p.type === 'debris') p.orbit += p.orbitSp * dt;
                else if (p.type === 'asteroid') { p.y += p.sp * dt * warp * 0.3; if (p.y > ctx.H + p.r) { p.y = -p.r; p.x = Math.random() * ctx.W; } }
                else if (p.type === 'planet') { p.y += p.sp * dt * warp * 0.15; if (p.y > ctx.H + p.r * 2) { p.y = -p.r * 2; p.x = Math.random() * ctx.W; } }
                else if (p.type === 'crystal') { p.y += p.sp * dt * warp * 0.2; if (p.y > ctx.H + p.r * 2) { p.y = -p.r * 2; p.x = Math.random() * ctx.W; } }
            }
            if (Math.random() < 0.004) {
                bgComets.push({ x: Math.random() * ctx.W, y: 0, vx: -30 - Math.random() * 70, vy: 160 + Math.random() * 140, life: 600, t: 0, size: 2 + Math.random() * 2 });
            }
            let cmlen = 0;
            for (let i = 0; i < bgComets.length; i++) {
                const cm = bgComets[i];
                cm.prevX = cm.x; cm.prevY = cm.y;
                cm.x += cm.vx * dt; cm.y += cm.vy * dt; cm.t += dt * 1000;
                if (cm.t < cm.life && cm.y <= ctx.H) bgComets[cmlen++] = cm;
            }
            bgComets.length = cmlen;
            if (ctx.G.bgTheme === 'storm' && Math.random() < 0.005) { ctx.G.lightningT = 150; ctx.G.lightningX = Math.random() * ctx.W; }
            if (ctx.G.lightningT > 0) ctx.G.lightningT -= dt * 1000;
            if (Math.random() < 0.002) ctx.SFX.envAmbience(ctx.G.bgTheme);
        }

        function drawStars(cv) {
            const warp = ctx.G.warpT > 0 ? 10 : 1;
            for (const s of STARS) {
                let brightness = s.br * (0.6 + 0.4 * Math.sin(ctx.tick * 0.02 + s.x));
                if (s.layer === 3) brightness *= 0.5 + 0.5 * Math.sin(s.twinkle);
                const colBase = s.col || '#ffffff';
                const r = parseInt(colBase.slice(1, 3), 16), g = parseInt(colBase.slice(3, 5), 16), b = parseInt(colBase.slice(5, 7), 16);
                cv.fillStyle = 'rgba(' + r + ',' + g + ',' + b + ',' + brightness + ')';
                const stretch = warp > 1 && s.layer >= 2 ? s.sz + 4 : s.sz;
                cv.fillRect(s.x, s.y, s.sz, stretch);
                if (s.layer >= 2 && brightness > 0.6) {
                    cv.globalAlpha = brightness * 0.3;
                    cv.fillRect(s.x - 1, s.y - 1, s.sz + 2, stretch + 2);
                    cv.globalAlpha = 1;
                }
            }
            for (let i = 0; i < shootingStars.length; i++) {
                const ss = shootingStars[i];
                const alpha = Math.max(0, 1 - ss.t / ss.life);
                cv.globalAlpha = alpha;
                cv.strokeStyle = ss.col; cv.lineWidth = 1;
                cv.beginPath();
                cv.moveTo(ss.x, ss.y);
                cv.lineTo(ss.prevX != null ? ss.prevX : ss.x - ss.vx * 0.05, ss.prevY != null ? ss.prevY : ss.y - ss.vy * 0.05);
                cv.stroke();
            }
            cv.globalAlpha = 1;

            if (ctx.G.warpT > 0) {
                const warpAlpha = Math.min(1, ctx.G.warpT / 500);
                const cx = ctx.W / 2, cy = ctx.H / 2;
                cv.save();
                cv.globalAlpha = warpAlpha * 0.85;
                cv.strokeStyle = 'rgba(220,240,255,' + warpAlpha + ')';
                cv.lineWidth = 1; cv.beginPath();
                for (const s of STARS) {
                    if (s.layer < 2 || s.sz !== 1) continue;
                    const dx = s.x - cx, dy = s.y - cy;
                    const dist = Math.hypot(dx, dy);
                    if (dist < 5) continue;
                    const len = Math.min(40, dist * 0.3 + 10) * warpAlpha;
                    const nx = dx / dist, ny = dy / dist;
                    cv.moveTo(s.x - nx * len, s.y - ny * len);
                    cv.lineTo(s.x, s.y);
                }
                cv.stroke();
                cv.lineWidth = 2; cv.beginPath();
                for (const s of STARS) {
                    if (s.layer < 2 || s.sz !== 2) continue;
                    const dx = s.x - cx, dy = s.y - cy;
                    const dist = Math.hypot(dx, dy);
                    if (dist < 5) continue;
                    const len = Math.min(40, dist * 0.3 + 10) * warpAlpha;
                    const nx = dx / dist, ny = dy / dist;
                    cv.moveTo(s.x - nx * len, s.y - ny * len);
                    cv.lineTo(s.x, s.y);
                }
                cv.stroke();
                cv.restore();
            }
            drawBG(cv);
        }

        function drawBG(cv) {
            for (const p of bgPlanets) {
                if (p.type === 'blackhole') {
                    const rr = p.r + Math.sin(ctx.tick * 0.02) * 3;
                    cv.save(); cv.globalAlpha = 0.5;
                    const gr = ctx.cachedRadialGradient(cv, 'blackhole', p.x, p.y, 0, p.r + 8, [[0, '#000'], [0.4, '#110033'], [0.7, '#220044'], [1, 'transparent']]);
                    cv.fillStyle = gr; cv.fillRect(p.x - rr - 8, p.y - rr - 8, (rr + 8) * 2, (rr + 8) * 2);
                    cv.beginPath(); cv.arc(p.x, p.y, p.r * 0.6, 0, Math.PI * 2); cv.fillStyle = '#000'; cv.fill();
                    cv.restore();
                } else if (p.type === 'debris') {
                    p.orbit += p.orbitSp * ctx.dt; const dx = Math.cos(p.orbit) * p.orbitR, dy = Math.sin(p.orbit) * p.orbitR;
                    cv.fillStyle = p.col; cv.globalAlpha = 0.35;
                    cv.fillRect(Math.floor(p.x + dx), Math.floor(p.y + dy), p.r * 2, p.r * 2);
                    cv.globalAlpha = 1;
                } else if (p.type === 'asteroid') {
                    cv.save(); cv.globalAlpha = 0.3; cv.translate(p.x, p.y); if (p.rot) cv.rotate(p.rot);
                    cv.fillStyle = p.col;
                    cv.fillRect(Math.floor(-p.r / 2), Math.floor(-p.r / 2), p.r, p.r);
                    cv.restore();
                } else if (p.type === 'planet') {
                    cv.save(); cv.globalAlpha = 0.35;
                    cv.beginPath(); cv.arc(p.x, p.y, p.r, 0, Math.PI * 2); cv.fillStyle = p.col; cv.fill();
                    if (p.atmoCol) { cv.beginPath(); cv.arc(p.x, p.y, p.r + 5, 0, Math.PI * 2); cv.fillStyle = p.atmoCol; cv.fill(); }
                    if (p.ringCol && p.ringR) {
                        cv.strokeStyle = p.ringCol; cv.lineWidth = 2; cv.beginPath();
                        cv.ellipse(p.x, p.y, p.r * p.ringR, p.r * p.ringR * 0.25, ctx.tick * 0.005, 0, Math.PI * 2);
                        cv.stroke();
                    }
                    if (p.storm && p.stormCol) {
                        for (let si = 0; si < 3; si++) {
                            const sa = ctx.tick * 0.01 + si * 2.1;
                            cv.beginPath(); cv.arc(p.x + Math.cos(sa) * p.r * 0.5, p.y + Math.sin(sa) * p.r * 0.3, p.r * 0.25, 0, Math.PI * 2);
                            cv.fillStyle = p.stormCol; cv.fill();
                        }
                    }
                    cv.restore();
                } else if (p.type === 'crystal') {
                    cv.save(); cv.globalAlpha = 0.4; cv.translate(p.x, p.y); cv.rotate(ctx.tick * 0.01);
                    cv.fillStyle = p.col;
                    cv.beginPath();
                    for (let ci = 0; ci < 6; ci++) {
                        const ca = (ci / 6) * Math.PI * 2;
                        const cx2 = Math.cos(ca) * p.r, cy2 = Math.sin(ca) * p.r;
                        if (ci === 0) cv.moveTo(cx2, cy2); else cv.lineTo(cx2, cy2);
                    }
                    cv.closePath(); cv.fill();
                    if (p.crystalCol) {
                        cv.strokeStyle = p.crystalCol; cv.lineWidth = 1; cv.stroke();
                        cv.globalAlpha = 0.2;
                        cv.fillStyle = p.crystalCol; cv.fill();
                    }
                    cv.restore();
                }
            }
            for (let i = 0; i < bgComets.length; i++) {
                const cm = bgComets[i];
                const alpha = Math.max(0, 1 - cm.t / cm.life);
                cv.globalAlpha = alpha * 0.7;
                cv.strokeStyle = '#aaccff'; cv.lineWidth = cm.size; cv.beginPath();
                cv.moveTo(cm.x, cm.y);
                cv.lineTo(cm.prevX != null ? cm.prevX : cm.x - cm.vx * 0.05, cm.prevY != null ? cm.prevY : cm.y - cm.vy * 0.05);
                cv.stroke();
                cv.fillStyle = '#ddeeff';
                cv.fillRect(cm.x - 1, cm.y - 1, 2, 2);
            }
            if (ctx.G.lightningT > 0) {
                const la = ctx.G.lightningT / 150;
                cv.globalAlpha = la * 0.3;
                cv.fillStyle = '#ffffff';
                cv.fillRect(0, 0, ctx.W, ctx.H);
                cv.globalAlpha = la * 0.8;
                cv.strokeStyle = '#ffffff';
                cv.lineWidth = 2;
                cv.shadowBlur = 12;
                cv.shadowColor = '#88ccff';
                cv.beginPath();
                let lx = ctx.G.lightningX, ly = 0;
                cv.moveTo(lx, ly);
                while (ly < ctx.H) {
                    lx += (Math.random() - 0.5) * 40;
                    ly += 15 + Math.random() * 25;
                    cv.lineTo(lx, ly);
                }
                cv.stroke();
                cv.shadowBlur = 0;
            }
            cv.globalAlpha = 1;
        }

        function drawNebula(cv) {
            if (!nebulaCv) return;
            const pulse = 0.25 + 0.1 * Math.sin(ctx.G.fTmr * 0.3);
            cv.globalAlpha = pulse * (ctx.G.chal ? 1.3 : 1);
            const y0 = -((ctx.G.fTmr * 15) % ctx.H);
            cv.drawImage(nebulaCv, 0, y0); cv.drawImage(nebulaCv, 0, y0 + ctx.H);
            if (ctx.G.chal) {
                cv.globalAlpha = Math.max(0, 0.08 + 0.05 * Math.sin(ctx.G.fTmr * 1.5));
                cv.fillStyle = '#ff440015'; cv.fillRect(0, 0, ctx.W, ctx.H);
            }
            cv.globalAlpha = 1;
        }

        ctx.STARS = STARS;
        ctx.initBG = initBG;
        ctx.mkNebula = mkNebula;
        ctx.ensureNebulaCanvas = ensureNebulaCanvas;
        ctx.updateBackground = updateBackground;
        ctx.drawStars = drawStars;
        ctx.drawBG = drawBG;
        ctx.drawNebula = drawNebula;
    };
})();
