/* Aura Drift - car art: pre-rendered silhouettes, wheels, lights and damage. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;

    /* Sprite footprint: 256 x 128 px maps to 4.6 x 2.3 world metres, so the
       aspect ratio matches every drivable corridor and one scale factor
       serves all four cars. */
    const SPRITE_W = 256;
    const SPRITE_H = 128;
    const CAR_LENGTH = 4.6;
    const CAR_WIDTH = 2.3;

    const sprites = new Map();

    function smoothPath(g, pts) {
        g.beginPath();
        const mid = (a, b) => [(a[0] + b[0]) / 2, (a[1] + b[1]) / 2];
        let prev = mid(pts[pts.length - 1], pts[0]);
        g.moveTo(prev[0], prev[1]);
        for (let i = 0; i < pts.length; i++) {
            const cur = pts[i];
            const next = mid(cur, pts[(i + 1) % pts.length]);
            g.quadraticCurveTo(cur[0], cur[1], next[0], next[1]);
        }
        g.closePath();
    }

    function shade(hex, amount) {
        const n = parseInt(hex.slice(1), 16);
        const r = clamp(((n >> 16) & 255) * (1 + amount), 0, 255) | 0;
        const g = clamp(((n >> 8) & 255) * (1 + amount), 0, 255) | 0;
        const b = clamp((n & 255) * (1 + amount), 0, 255) | 0;
        return 'rgb(' + r + ',' + g + ',' + b + ')';
    }

    /* --- silhouettes ------------------------------------------------------
       Nose points to +x. Each class reads differently at a glance: the
       prototype has a pointed nose and a huge wing, the GT a long bonnet,
       the muscle car broad hips, the rally hatch a short tail and roof scoop. */
    function outline(shape) {
        switch (shape) {
            case 'wedge':
                return [[124, -16], [110, -34], [72, -44], [8, -54], [-46, -58], [-92, -52],
                    [-116, -40], [-120, 0], [-116, 40], [-92, 52], [-46, 58], [8, 54],
                    [72, 44], [110, 34], [124, 16]];
            case 'muscle':
                return [[122, -22], [104, -42], [60, -50], [0, -54], [-56, -60], [-100, -56],
                    [-118, -42], [-120, 0], [-118, 42], [-100, 56], [-56, 60], [0, 54],
                    [60, 50], [104, 42], [122, 22]];
            case 'rally':
                return [[116, -26], [96, -46], [48, -54], [-16, -56], [-74, -54], [-108, -44],
                    [-116, -26], [-118, 0], [-116, 26], [-108, 44], [-74, 54], [-16, 56],
                    [48, 54], [96, 46], [116, 26]];
            default: /* gt */
                return [[120, -24], [100, -44], [56, -54], [-6, -56], [-64, -54], [-104, -46],
                    [-118, -30], [-120, 0], [-118, 30], [-104, 46], [-64, 54], [-6, 56],
                    [56, 54], [100, 44], [120, 24]];
        }
    }

    function paintCar(car, index) {
        const canvas = document.createElement('canvas');
        canvas.width = SPRITE_W; canvas.height = SPRITE_H;
        const g = canvas.getContext('2d');
        g.translate(SPRITE_W / 2, SPRITE_H / 2);

        const body = outline(car.shape || 'gt');
        const colour = car.colour;
        const accent = car.accent;

        /* Contact shadow baked under the body: keeps the sprite grounded even
           when the renderer draws its own soft shadow. */
        g.save();
        g.translate(4, 6);
        smoothPath(g, body);
        g.fillStyle = 'rgba(6,9,14,0.45)';
        g.fill();
        g.restore();

        /* Body shell with a cross-car gradient: bright shoulder, dark flank. */
        const shell = g.createLinearGradient(0, -60, 0, 60);
        shell.addColorStop(0, shade(colour, 0.34));
        shell.addColorStop(0.28, shade(colour, 0.12));
        shell.addColorStop(0.52, colour);
        shell.addColorStop(0.72, shade(colour, -0.26));
        shell.addColorStop(1, shade(colour, -0.5));
        smoothPath(g, body);
        g.fillStyle = shell;
        g.fill();

        /* Long specular sweep along the shoulder line. */
        g.save();
        smoothPath(g, body);
        g.clip();
        const sweep = g.createLinearGradient(-120, -50, 60, 20);
        sweep.addColorStop(0, 'rgba(255,255,255,0)');
        sweep.addColorStop(0.45, 'rgba(255,255,255,0.16)');
        sweep.addColorStop(0.62, 'rgba(255,255,255,0.05)');
        sweep.addColorStop(1, 'rgba(255,255,255,0)');
        g.fillStyle = sweep;
        g.fillRect(-130, -70, 260, 140);
        /* Panel break lines: bonnet, roof and boot read as separate panels. */
        g.strokeStyle = 'rgba(10,12,18,0.35)';
        g.lineWidth = 1.6;
        for (const x of [58, 18, -26, -70]) {
            g.beginPath();
            g.moveTo(x, -58);
            g.quadraticCurveTo(x + 3, 0, x, 58);
            g.stroke();
        }
        g.restore();

        /* Dark outline grounds the car on pale salt and sand. */
        smoothPath(g, body);
        g.strokeStyle = 'rgba(8,10,16,0.55)';
        g.lineWidth = 2.4;
        g.stroke();

        /* Cockpit: dark glass with a diagonal reflection that slides with
           the steering, so the car reads as a solid object with a window. */
        const glass = car.shape === 'rally'
            ? [[16, -30], [-6, -36], [-52, -34], [-62, 0], [-52, 34], [-6, 36], [16, 30]]
            : [[34, -28], [10, -36], [-44, -34], [-56, 0], [-44, 34], [10, 36], [34, 28]];
        smoothPath(g, glass);
        const glassFill = g.createLinearGradient(-60, -36, 20, 36);
        glassFill.addColorStop(0, 'rgba(26,38,52,0.95)');
        glassFill.addColorStop(0.55, 'rgba(14,22,32,0.95)');
        glassFill.addColorStop(1, 'rgba(30,44,60,0.95)');
        g.fillStyle = glassFill;
        g.fill();
        g.save();
        smoothPath(g, glass);
        g.clip();
        g.fillStyle = 'rgba(190,225,255,0.28)';
        g.beginPath();
        g.moveTo(-70, 44); g.lineTo(-18, -46); g.lineTo(6, -46); g.lineTo(-46, 44);
        g.closePath(); g.fill();
        g.fillStyle = 'rgba(255,255,255,0.10)';
        g.beginPath();
        g.moveTo(-30, 44); g.lineTo(16, -46); g.lineTo(26, -46); g.lineTo(-20, 44);
        g.closePath(); g.fill();
        g.restore();

        /* Racing stripes: a centre pair in the livery accent. */
        g.fillStyle = accent;
        g.globalAlpha = 0.82;
        g.fillRect(-118, -7, 236, 6);
        g.fillRect(-118, 1, 236, 6);
        g.globalAlpha = 1;

        /* Bonnet vents and a cooling intake: small dark slots. */
        g.fillStyle = 'rgba(12,14,20,0.55)';
        for (const y of [-26, -10, 10, 26]) {
            g.fillRect(62, y, 22, 5);
        }
        if (car.shape === 'rally') {
            g.fillStyle = 'rgba(12,14,20,0.5)';
            g.fillRect(-24, -12, 22, 24);
            g.fillStyle = accent;
            g.globalAlpha = 0.5;
            g.fillRect(-22, -10, 18, 20);
            g.globalAlpha = 1;
        }

        /* Rear wing for the prototype and the GT. */
        if (car.shape === 'wedge' || car.shape === 'gt') {
            g.fillStyle = 'rgba(14,16,22,0.92)';
            g.fillRect(-124, -56, 10, 112);
            g.fillStyle = shade(colour, -0.25);
            g.fillRect(-126, -50, 8, 100);
            g.fillStyle = accent;
            g.globalAlpha = 0.7;
            g.fillRect(-126, -50, 8, 4);
            g.fillRect(-126, 46, 8, 4);
            g.globalAlpha = 1;
        }
        if (car.shape === 'rally') {
            /* Roof spoiler instead of a wing. */
            g.fillStyle = 'rgba(16,18,24,0.9)';
            g.fillRect(-96, -44, 9, 88);
            g.fillStyle = shade(colour, -0.2);
            g.fillRect(-96, -42, 9, 84);
        }

        /* Front splitter and dive planes. */
        g.fillStyle = 'rgba(14,16,22,0.85)';
        g.fillRect(112, -40, 12, 80);
        g.fillStyle = 'rgba(255,255,255,0.08)';
        g.fillRect(112, -40, 3, 80);

        /* Mirrors. */
        g.fillStyle = shade(colour, -0.35);
        g.fillRect(28, -62, 14, 7);
        g.fillRect(28, 55, 14, 7);

        /* Headlights and tail lights. */
        g.fillStyle = 'rgba(255,244,214,0.95)';
        g.beginPath(); g.ellipse(112, -30, 10, 6, 0, 0, Math.PI * 2); g.fill();
        g.beginPath(); g.ellipse(112, 30, 10, 6, 0, 0, Math.PI * 2); g.fill();
        g.fillStyle = 'rgba(120,132,150,0.6)';
        g.beginPath(); g.ellipse(116, -30, 6, 4, 0, 0, Math.PI * 2); g.fill();
        g.beginPath(); g.ellipse(116, 30, 6, 4, 0, 0, Math.PI * 2); g.fill();
        g.fillStyle = 'rgba(168,32,26,0.92)';
        g.fillRect(-120, -46, 8, 20);
        g.fillRect(-120, 26, 8, 20);

        /* Exhausts. */
        g.fillStyle = 'rgba(24,26,32,0.9)';
        g.fillRect(-124, -18, 8, 10);
        g.fillRect(-124, 8, 8, 10);

        /* Race number roundel. */
        g.fillStyle = 'rgba(248,250,252,0.92)';
        g.beginPath(); g.arc(-70, 0, 15, 0, Math.PI * 2); g.fill();
        g.strokeStyle = 'rgba(12,14,20,0.5)';
        g.lineWidth = 1.5;
        g.stroke();
        g.fillStyle = '#12141a';
        g.font = 'bold 18px system-ui, sans-serif';
        g.textAlign = 'center';
        g.textBaseline = 'middle';
        g.fillText(String(index + 1), -70, 1);

        return canvas;
    }

    AD.carSprite = function (index) {
        const key = index % AD.CARS.length;
        let sprite = sprites.get(key);
        if (!sprite) {
            sprite = paintCar(AD.CARS[key], key);
            sprites.set(key, sprite);
        }
        return sprite;
    };

    function roundRect(g, x, y, w, h, r) {
        g.beginPath();
        g.moveTo(x + r, y);
        g.arcTo(x + w, y, x + w, y + h, r);
        g.arcTo(x + w, y + h, x, y + h, r);
        g.arcTo(x, y + h, x, y, r);
        g.arcTo(x, y, x + w, y, r);
        g.closePath();
    }

    /* Wheels live outside the sprite so the front pair can follow the
       steering angle and the rears can spin up under power. */
    function drawWheel(g, wx, wy, steer, len, wid, spin) {
        g.save();
        g.translate(wx, wy);
        g.rotate(steer);
        g.fillStyle = '#101317';
        roundRect(g, -len * 0.11, -wid * 0.085, len * 0.22, wid * 0.17, wid * 0.05);
        g.fill();
        g.fillStyle = 'rgba(190,205,225,0.22)';
        g.fillRect(-len * 0.015, -wid * 0.085, len * 0.03, wid * 0.17);
        /* Tyre shoulder catches the light. */
        g.fillStyle = 'rgba(255,255,255,0.09)';
        g.fillRect(-len * 0.11, -wid * 0.085, len * 0.22, wid * 0.03);
        if (spin) {
            g.fillStyle = 'rgba(255,190,120,' + (0.12 + spin * 0.22).toFixed(2) + ')';
            g.fillRect(-len * 0.02, -wid * 0.08, len * 0.04, wid * 0.16);
        }
        g.restore();
    }

    /* opts: { ghost, night, lighting, bloom, frame, reducedMotion } */
    AD.drawCar = function (g, car, sx, sy, scale, opts) {
        const v = car.v || car;
        const options = opts || {};
        if (sx < -110 || sx > (options.width || 99999) + 110 || sy < -110 || sy > (options.height || 99999) + 110) return;
        const ghost = !!options.ghost;
        const index = (v.car && v.carIndex !== undefined) ? v.carIndex % AD.CARS.length : 0;
        const sprite = AD.carSprite(index);
        const len = CAR_LENGTH * scale;
        const wid = CAR_WIDTH * scale;
        const factor = len / SPRITE_W;
        const speed = Math.max(0, v.vx || 0);

        g.save();
        g.translate(sx, sy);
        g.rotate(v.psi);
        if (ghost) g.globalAlpha = 0.34;

        /* Soft contact shadow offset away from the key light. */
        if (!ghost) {
            g.fillStyle = 'rgba(0,0,0,0.34)';
            g.beginPath();
            g.ellipse(-len * 0.06 + len * 0.05, len * 0.05, len * 0.55, wid * 0.56, 0, 0, Math.PI * 2);
            g.fill();
        }

        const steerAngle = ghost ? 0 : (v.steer || 0);
        const spin = ghost ? 0 : clamp((v.slip || 0) / 9, 0, 1);
        drawWheel(g, len * 0.30, -wid * 0.56, steerAngle, len, wid, 0);
        drawWheel(g, len * 0.30, wid * 0.56, steerAngle, len, wid, 0);
        drawWheel(g, -len * 0.31, -wid * 0.56, 0, len, wid, spin);
        drawWheel(g, -len * 0.31, wid * 0.56, 0, len, wid, spin);

        g.drawImage(sprite, -len / 2, -wid / 2, len, wid);

        if (!ghost) {
            /* Brake lights and their glow: the clearest read on intent. */
            const braking = v.brake > 0.05 || v.handbrake > 0.5;
            if (braking) {
                g.fillStyle = '#ff4436';
                g.fillRect(-len * 0.5, -wid * 0.38, len * 0.05, wid * 0.2);
                g.fillRect(-len * 0.5, wid * 0.18, len * 0.05, wid * 0.2);
                if (options.bloom) {
                    const glow = g.createRadialGradient(-len * 0.54, 0, 0, -len * 0.54, 0, len * 0.34);
                    glow.addColorStop(0, 'rgba(255,60,40,0.42)');
                    glow.addColorStop(1, 'rgba(255,60,40,0)');
                    g.fillStyle = glow;
                    g.fillRect(-len * 0.92, -wid * 0.75, len * 0.6, wid * 1.5);
                }
            }
            /* Headlight cone plus a pool on the asphalt ahead. */
            if (options.night && options.lighting) {
                for (const side of [-1, 1]) {
                    const hx = len * 0.46, hy = side * wid * 0.24;
                    const beam = g.createLinearGradient(hx, 0, hx + len * 4.6, 0);
                    beam.addColorStop(0, 'rgba(255,236,190,0.14)');
                    beam.addColorStop(1, 'rgba(255,236,190,0)');
                    g.fillStyle = beam;
                    g.beginPath();
                    g.moveTo(hx, hy - wid * 0.07);
                    g.lineTo(hx + len * 4.6, hy - wid * 0.8);
                    g.lineTo(hx + len * 4.6, hy + wid * 0.8);
                    g.lineTo(hx, hy + wid * 0.07);
                    g.closePath(); g.fill();
                    g.fillStyle = 'rgba(255,246,220,0.95)';
                    g.beginPath(); g.arc(hx, hy, wid * 0.07, 0, Math.PI * 2); g.fill();
                }
                const halo = g.createRadialGradient(len * 2.2, 0, 0, len * 2.2, 0, len * 2.1);
                halo.addColorStop(0, 'rgba(255,240,200,0.10)');
                halo.addColorStop(1, 'rgba(255,240,200,0)');
                g.fillStyle = halo;
                g.beginPath(); g.arc(len * 2.2, 0, len * 2.1, 0, Math.PI * 2); g.fill();
            }
            /* Nitro plume out of the exhausts. */
            if (v.boost) {
                const flicker = 0.8 + 0.2 * Math.abs(Math.sin((options.frame || 0) * 0.6));
                const flame = g.createLinearGradient(-len * 0.5, 0, -len * 1.25, 0);
                flame.addColorStop(0, 'rgba(150,235,255,' + (0.85 * flicker).toFixed(2) + ')');
                flame.addColorStop(0.45, 'rgba(90,190,255,' + (0.45 * flicker).toFixed(2) + ')');
                flame.addColorStop(1, 'rgba(60,140,255,0)');
                g.fillStyle = flame;
                g.beginPath();
                g.moveTo(-len * 0.48, -wid * 0.13);
                g.lineTo(-len * 1.25, 0);
                g.lineTo(-len * 0.48, wid * 0.13);
                g.closePath(); g.fill();
                g.fillStyle = 'rgba(240,253,255,0.9)';
                g.beginPath(); g.ellipse(-len * 0.52, 0, len * 0.08, wid * 0.12, 0, 0, Math.PI * 2); g.fill();
            }
            /* Shield bubble. */
            if (v.shield > 0) {
                const pulse = 0.65 + 0.35 * Math.sin((options.frame || 0) * 0.12);
                g.strokeStyle = 'rgba(94,234,212,' + (0.5 + 0.4 * pulse).toFixed(2) + ')';
                g.lineWidth = 2;
                g.beginPath();
                g.ellipse(0, 0, len * (0.6 + 0.03 * pulse), wid * (0.9 + 0.05 * pulse), 0, 0, Math.PI * 2);
                g.stroke();
            }
            /* EMP lock-on ring. */
            if (v.disabled > 0) {
                g.strokeStyle = 'rgba(56,189,248,0.85)';
                g.lineWidth = 1.5;
                g.setLineDash([4, 4]);
                g.lineDashOffset = -(options.frame || 0) * 0.7;
                g.beginPath(); g.arc(0, 0, len * 0.6, 0, Math.PI * 2); g.stroke();
                g.setLineDash([]);
            }
            /* Damage: dents, scrapes and a cracked panel. */
            const damage = v.damage || 0;
            if (damage > 18) {
                g.fillStyle = 'rgba(26,22,20,' + Math.min(0.5, damage / 190).toFixed(2) + ')';
                g.beginPath();
                g.ellipse(len * 0.18, wid * 0.16, wid * 0.22, wid * 0.16, 0.4, 0, Math.PI * 2);
                g.fill();
                g.strokeStyle = 'rgba(255,190,150,0.28)';
                g.lineWidth = 1.2;
                g.beginPath();
                g.moveTo(-len * 0.4, -wid * 0.3);
                g.lineTo(-len * 0.05, -wid * 0.12);
                g.stroke();
            }
            if (damage > 55) {
                g.fillStyle = 'rgba(30,26,24,0.5)';
                g.beginPath(); g.arc(len * 0.22, wid * 0.2, wid * 0.22, 0, Math.PI * 2); g.fill();
            }
            /* Player chevron: readable at a glance in a pack of four. */
            if (car.isPlayer) {
                g.fillStyle = 'rgba(255,255,255,0.92)';
                g.beginPath();
                g.moveTo(0, -wid * 1.02); g.lineTo(len * 0.2, -wid * 0.78); g.lineTo(-len * 0.2, -wid * 0.78);
                g.closePath(); g.fill();
                g.fillStyle = 'rgba(8,10,14,0.6)';
                g.beginPath();
                g.moveTo(0, -wid * 0.95); g.lineTo(len * 0.12, -wid * 0.81); g.lineTo(-len * 0.12, -wid * 0.81);
                g.closePath(); g.fill();
            }
        }
        g.restore();
    };

    AD.clearCarSprites = function () { sprites.clear(); };
})();
