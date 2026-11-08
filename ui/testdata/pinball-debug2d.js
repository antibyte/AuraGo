// AuraPinball 2D debug view: canvas renderer + keyboard/mouse interaction for the physics world. y is up.
// Keys: Shift L/R flippers, Space plunger (hold), N/M/U nudge left/right/up, B ball at cursor, drag = throw a ball,
// A serve a ball via the plunger, P pause, . single step, 1-9 time scale, R reset, G grid, I ids, L lamps, V velocity.
import { FONT_STACK } from '/js/desktop/apps/pinball/config.js';
import { lampPolygon } from '/js/desktop/apps/pinball/lights/shapes.js';
import { BALL_R } from '/js/desktop/apps/pinball/physics/constants.js';

const X0 = -0.2575, FW = 0.515, FH = 1.07;
const MATERIAL_COLOR = { wood: '#c8924f', metal: '#c3ccd6', plastic: '#4fd0f0', rubber: '#ff5470', flipper: '#ffd24a', target: '#7dff9a', ball: '#ffffff' };
const SCALES = [1, 0.5, 0.25, 0.1, 0.05, 2, 3, 4, 8];
const THROW_GAIN = 9; // m/s of ball speed per metre of drag

const hColor = h => `hsl(${Math.round(200 + Math.max(0, Math.min(0.12, h || 0)) / 0.12 * 140)} 90% 62%)`;
const itemsOf = list => Array.isArray(list) ? list : (list && typeof list === 'object' ? Object.values(list) : []);
const isPt = p => Array.isArray(p) && p.length >= 2 && Number.isFinite(p[0]) && Number.isFinite(p[1]);

export function mount2d(h) {
    const canvas = document.createElement('canvas');
    h.stage.appendChild(canvas);
    const ctx = canvas.getContext('2d');
    const layout = h.layout, world = h.world;
    const debug = world.debug();
    const show = { grid: false, ids: h.params.debug, lamps: true, vel: true, paths: true };
    const cursor = { x: 0, y: 0, down: null };
    const flash = new Map(); // device id -> remaining seconds highlighted after an event
    let W = 0, H = 0, s = 1, ox = 0, oy = 0, raf = 0, last = performance.now(), disposed = false, frameDebug = debug;

    const sx = x => ox + (x - X0) * s, sy = y => oy - y * s;
    const wx = px => (px - ox) / s + X0, wy = py => (oy - py) / s;

    function resize() {
        const dpr = window.devicePixelRatio || 1;
        W = h.stage.clientWidth; H = h.stage.clientHeight;
        canvas.width = Math.round(W * dpr); canvas.height = Math.round(H * dpr);
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        s = Math.min((W - 40) / FW, (H - 30) / FH);
        ox = (W - FW * s) / 2 + 0; oy = (H + FH * s) / 2;
    }

    function line(ax, ay, bx, by) { ctx.beginPath(); ctx.moveTo(sx(ax), sy(ay)); ctx.lineTo(sx(bx), sy(by)); ctx.stroke(); }
    function label(text, x, y, color = '#9fb0c4', size = 10) {
        ctx.font = `${size}px ${FONT_STACK}`; ctx.fillStyle = color; ctx.fillText(text, sx(x) + 3, sy(y) - 3);
    }

    function drawField() {
        ctx.fillStyle = '#0b0d12'; ctx.fillRect(0, 0, W, H);
        ctx.fillStyle = '#121a26'; ctx.fillRect(sx(X0), sy(FH), FW * s, FH * s);
        ctx.strokeStyle = '#2a3a52'; ctx.lineWidth = 1; ctx.strokeRect(sx(X0), sy(FH), FW * s, FH * s);
        if (show.grid) {
            ctx.strokeStyle = '#ffffff12'; ctx.fillStyle = '#6f819a'; ctx.font = `9px ${FONT_STACK}`;
            for (let x = -0.25; x <= 0.2501; x += 0.05) { line(x, 0, x, FH); ctx.fillText(x.toFixed(2), sx(x) + 2, sy(0) - 2); }
            for (let y = 0.1; y < FH; y += 0.1) { line(X0, y, X0 + FW, y); ctx.fillText(y.toFixed(1), sx(X0) + 2, sy(y) - 2); }
            ctx.strokeStyle = '#ffffff2a'; line(layout.meta.cx, 0, layout.meta.cx, FH);
            if (layout.drainY) { ctx.strokeStyle = '#ff4a4a66'; line(X0, layout.drainY, X0 + FW, layout.drainY); }
        }
    }

    function drawLamps() {
        if (!show.lamps) return;
        (layout.lamps || []).forEach((lamp, i) => {
            let pts; try { pts = lampPolygon(lamp, 16); } catch (_) { return; }
            ctx.beginPath(); pts.forEach((p, k) => k ? ctx.lineTo(sx(p[0]), sy(p[1])) : ctx.moveTo(sx(p[0]), sy(p[1]))); ctx.closePath();
            ctx.fillStyle = (lamp.color || '#ffd24a') + '55'; ctx.fill();
            ctx.strokeStyle = lamp.color || '#ffd24a'; ctx.lineWidth = 1; ctx.stroke();
            ctx.font = `8px ${FONT_STACK}`; ctx.fillStyle = '#e8eef7'; ctx.textAlign = 'center';
            ctx.fillText(String(i + 1), sx(lamp.x), sy(lamp.y) + 3); ctx.textAlign = 'start';
        });
    }

    function drawPaths() {
        if (!show.paths) return;
        const dbg = itemsOf(frameDebug.paths), defs = layout.paths || [];
        const list = dbg.length ? dbg : defs;
        for (const p of list) {
            const pts = (p.points || p.samples || []).filter(isPt);
            if (pts.length < 2) continue;
            ctx.lineWidth = 2;
            for (let i = 1; i < pts.length; i++) {
                ctx.strokeStyle = hColor(pts[i][2]); line(pts[i - 1][0], pts[i - 1][1], pts[i][0], pts[i][1]);
            }
            const def = defs.find(d => d.id === p.id) || p;
            if (def.entry && isPt(def.entry.a) && isPt(def.entry.b)) {
                ctx.strokeStyle = '#4dff88'; ctx.lineWidth = 3; line(def.entry.a[0], def.entry.a[1], def.entry.b[0], def.entry.b[1]);
            }
            const end = pts[pts.length - 1], prev = pts[pts.length - 2];
            ctx.strokeStyle = '#ff9d3a'; ctx.lineWidth = 3;
            const ang = Math.atan2(end[1] - prev[1], end[0] - prev[0]);
            line(end[0], end[1], end[0] + Math.cos(ang) * 0.02, end[1] + Math.sin(ang) * 0.02);
            if (show.ids) label(String(p.id), pts[0][0], pts[0][1], '#7fd0ff');
        }
    }

    function drawShapes() {
        ctx.lineCap = 'round';
        for (const g of frameDebug.segments || []) {
            ctx.strokeStyle = g.active === false ? '#55606e' : (MATERIAL_COLOR[g.material] || '#ccc');
            ctx.lineWidth = Math.max(1.5, 0.003 * s); ctx.setLineDash(g.active === false ? [4, 4] : []);
            line(g.ax, g.ay, g.bx, g.by);
        }
        ctx.setLineDash([]);
        for (const c of frameDebug.circles || []) {
            ctx.beginPath(); ctx.arc(sx(c.cx), sy(c.cy), Math.max(1.5, c.r * s), 0, Math.PI * 2);
            ctx.fillStyle = (MATERIAL_COLOR[c.material] || '#ccc') + (c.active === false ? '33' : '99'); ctx.fill();
            ctx.strokeStyle = MATERIAL_COLOR[c.material] || '#ccc'; ctx.lineWidth = 1; ctx.stroke();
        }
    }

    function drawFlippers(snap) {
        (frameDebug.flippers || []).forEach((f, i) => {
            const a = snap && snap.flippers.length > 2 * i ? snap.flippers[2 * i] : f.a;
            const phi = f.side > 0 ? a : Math.PI - a;
            const tx = f.x + Math.cos(phi) * f.length, ty = f.y + Math.sin(phi) * f.length;
            const nx = -Math.sin(phi), ny = Math.cos(phi);
            ctx.fillStyle = f.on ? '#ffe27a' : '#e0a82e'; ctx.strokeStyle = '#fff3c0'; ctx.lineWidth = 1;
            ctx.beginPath();
            ctx.moveTo(sx(f.x + nx * f.rBase), sy(f.y + ny * f.rBase)); ctx.lineTo(sx(tx + nx * f.rTip), sy(ty + ny * f.rTip));
            ctx.lineTo(sx(tx - nx * f.rTip), sy(ty - ny * f.rTip)); ctx.lineTo(sx(f.x - nx * f.rBase), sy(f.y - ny * f.rBase));
            ctx.closePath(); ctx.fill(); ctx.stroke();
            for (const [cx, cy, r] of [[f.x, f.y, f.rBase], [tx, ty, f.rTip]]) {
                ctx.beginPath(); ctx.arc(sx(cx), sy(cy), r * s, 0, Math.PI * 2); ctx.fill(); ctx.stroke();
            }
            if (show.ids) label(String(f.id), f.x, f.y + 0.02, '#ffd24a');
        });
    }

    // Devices: whatever world.debug().devices reports (objects with x,y[,r] or a,b), else the layout definitions.
    function drawDevices() {
        const dbg = itemsOf(frameDebug.devices);
        const list = dbg.length ? dbg : (layout.devices || []);
        for (const d of list) {
            if (!d || typeof d !== 'object') continue;
            const hot = flash.get(d.id) > 0;
            ctx.strokeStyle = hot ? '#ffffff' : '#ff9ae6'; ctx.fillStyle = hot ? '#ff9ae6aa' : '#ff9ae633'; ctx.lineWidth = hot ? 3 : 1.5;
            const lines = [];
            if (isPt(d.a) && isPt(d.b)) lines.push([d.a, d.b]);
            if (d.channel && isPt(d.channel.a) && isPt(d.channel.b)) lines.push([d.channel.a, d.channel.b]);
            for (const t of d.targets || []) if (Number.isFinite(t.x)) { ctx.fillRect(sx(t.x) - 3, sy(t.y) - 3, 6, 6); if (show.ids) label(String(t.id), t.x, t.y, '#ff9ae6'); }
            for (const [p, q] of lines) line(p[0], p[1], q[0], q[1]);
            const x = Number.isFinite(d.x) ? d.x : (lines.length ? (lines[0][0][0] + lines[0][1][0]) / 2 : null);
            const y = Number.isFinite(d.y) ? d.y : (lines.length ? (lines[0][0][1] + lines[0][1][1]) / 2 : null);
            if (x === null) continue;
            if (Number.isFinite(d.x) && !lines.length) {
                const r = Number.isFinite(d.r) ? d.r : 0.008;
                ctx.beginPath(); ctx.arc(sx(x), sy(y), Math.max(2, r * s), 0, Math.PI * 2); ctx.fill(); ctx.stroke();
            }
            if (show.ids) label(`${d.id || ''}${d.kind || d.type ? ' (' + (d.kind || d.type) + ')' : ''}`, x, y, '#ffb8ef');
        }
    }

    function drawBalls(snap) {
        for (const b of snap.balls) {
            if (b.hidden) continue;
            ctx.beginPath(); ctx.arc(sx(b.x), sy(b.y), Math.max(3, BALL_R * s), 0, Math.PI * 2);
            ctx.fillStyle = b.mode === 'path' ? '#b8e6ff' : '#ffffff'; ctx.fill();
            ctx.strokeStyle = '#2a7bff'; ctx.lineWidth = 1.5; ctx.stroke();
            if (show.vel) {
                ctx.strokeStyle = '#4dff88'; ctx.lineWidth = 2; line(b.x, b.y, b.x + b.vx * 0.05, b.y + b.vy * 0.05);
            }
            if (show.ids) label(`${b.id} ${b.mode} h${b.h.toFixed(3)} v${Math.hypot(b.vx, b.vy).toFixed(2)}`, b.x + 0.015, b.y + 0.015, '#fff');
        }
        if (cursor.down) {
            ctx.strokeStyle = '#ff9d3a'; ctx.lineWidth = 2; line(cursor.down.x, cursor.down.y, cursor.x, cursor.y);
        }
    }

    function drawHud(snap) {
        const st = world.stats(), lines = [
            `${h.params.table}  tick ${snap.tick}  x${h.clock.scale}${h.clock.paused ? '  PAUSED' : ''}  balls ${st.balls}  trough ${world.trough}  substeps ${st.substeps}`,
            `cursor ${cursor.x.toFixed(3)}, ${cursor.y.toFixed(3)}   plunger ${snap.plunger.toFixed(2)}   L${h.input.left ? '*' : '-'} R${h.input.right ? '*' : '-'}`,
            'Shift L/R flip  Space plunge  A serve  N/M/U nudge  B ball  drag throw  P pause  . step  1-9 speed  R reset  G grid  I ids  L lamps  V vel'
        ];
        ctx.font = `12px ${FONT_STACK}`; ctx.fillStyle = '#cfd8e3';
        lines.forEach((t, i) => ctx.fillText(t, 10, 18 + i * 15));
        const log = h.events.slice(-20);
        ctx.font = `11px ${FONT_STACK}`; ctx.textAlign = 'right';
        ctx.fillStyle = '#8aa0ba'; ctx.fillText('events (last 20)', W - 10, 18);
        log.forEach((e, i) => {
            ctx.fillStyle = '#d6e2f2';
            ctx.fillText(`${e.tick} ${e.type}:${e.id}${e.ball ? '#' + (e.ball.id ?? e.ball) : ''}${Number.isFinite(e.speed) ? ' ' + e.speed.toFixed(2) : ''}`, W - 10, 34 + i * 13);
        });
        ctx.textAlign = 'start';
    }

    function frame(now) {
        if (disposed) return;
        raf = requestAnimationFrame(frame);
        const dt = (now - last) / 1000; last = now;
        const before = h.events.length;
        const alpha = h.advance(dt);
        for (let i = before; i < h.events.length; i++) flash.set(h.events[i].id, 0.25);
        for (const [k, v] of flash) { if (v - dt <= 0) flash.delete(k); else flash.set(k, v - dt); }
        frameDebug = world.debug();
        const snap = h.renderSnapshot(alpha);
        drawField(); drawLamps(); drawPaths(); drawShapes(); drawDevices(); drawFlippers(snap); drawBalls(snap); drawHud(snap);
    }

    // ---- input ------------------------------------------------------------------------------------------------------
    function nudge(dx, dy) { world.command({ type: 'nudge', dx, dy }); }
    function onKey(e, down) {
        if (e.repeat && down) { e.preventDefault(); return; }
        if (e.code === 'ShiftLeft') h.input.left = down;
        else if (e.code === 'ShiftRight') h.input.right = down;
        else if (e.code === 'Space') { h.input.plunger = down ? 1 : 0; e.preventDefault(); }
        if (!down) return;
        switch (e.code) {
            case 'KeyN': nudge(-0.3, 0); break;
            case 'KeyM': nudge(0.3, 0); break;
            case 'KeyU': nudge(0, 0.3); break;
            case 'KeyB': h.placeBall(cursor.x, cursor.y); break;
            case 'KeyA': h.addBall(false); break;
            case 'KeyP': h.clock.paused = !h.clock.paused; break;
            case 'Period': if (h.clock.paused) h.step(1); break;
            case 'KeyR': h.reset(); break;
            case 'KeyG': show.grid = !show.grid; break;
            case 'KeyI': show.ids = !show.ids; break;
            case 'KeyL': show.lamps = !show.lamps; break;
            case 'KeyV': show.vel = !show.vel; break;
            default:
                if (/^Digit[1-9]$/.test(e.code)) h.clock.scale = SCALES[+e.code.slice(5) - 1];
        }
    }
    const kd = e => onKey(e, true), ku = e => onKey(e, false);
    function pos(e) {
        const r = canvas.getBoundingClientRect();
        cursor.x = wx(e.clientX - r.left); cursor.y = wy(e.clientY - r.top);
    }
    canvas.addEventListener('mousemove', pos);
    canvas.addEventListener('mousedown', e => { pos(e); cursor.down = { x: cursor.x, y: cursor.y }; });
    window.addEventListener('mouseup', () => {
        const d = cursor.down; cursor.down = null;
        if (d && Math.hypot(cursor.x - d.x, cursor.y - d.y) > 0.004) {
            h.placeBall(d.x, d.y, (cursor.x - d.x) * THROW_GAIN, (cursor.y - d.y) * THROW_GAIN);
        }
    });
    window.addEventListener('keydown', kd); window.addEventListener('keyup', ku);
    window.addEventListener('resize', resize);
    window.addEventListener('blur', () => { h.input.left = h.input.right = false; h.input.plunger = 0; });
    resize();
    raf = requestAnimationFrame(frame);
    return {
        dispose() {
            disposed = true; cancelAnimationFrame(raf);
            window.removeEventListener('keydown', kd); window.removeEventListener('keyup', ku); window.removeEventListener('resize', resize);
            canvas.remove();
        }
    };
}
