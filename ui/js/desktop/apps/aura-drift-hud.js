/* Aura Drift - in-race HUD, minimap and countdown on a dedicated overlay canvas. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;

    AD.createHud = function (canvas, labels) {
        const c = canvas.getContext('2d');
        let width = 960, height = 540, dpr = 1;
        let message = '', messageTime = 0;

        function resize(w, h, ratio) {
            width = Math.max(320, Math.round(w));
            height = Math.max(240, Math.round(h));
            dpr = clamp(ratio || 1, 1, 2);
            canvas.width = Math.round(width * dpr);
            canvas.height = Math.round(height * dpr);
            canvas.style.width = width + 'px';
            canvas.style.height = height + 'px';
            c.setTransform(dpr, 0, 0, dpr, 0, 0);
        }

        function panel(x, y, w, h) {
            c.fillStyle = 'rgba(10,14,22,0.62)';
            c.strokeStyle = 'rgba(160,190,220,0.16)';
            c.lineWidth = 1;
            c.beginPath();
            c.roundRect ? c.roundRect(x, y, w, h, 10) : c.rect(x, y, w, h);
            c.fill(); c.stroke();
        }

        function text(value, x, y, size, colour, align, weight) {
            c.font = (weight || 500) + ' ' + size + 'px system-ui, sans-serif';
            c.fillStyle = colour || '#eef4ff';
            c.textAlign = align || 'left';
            c.textBaseline = 'alphabetic';
            c.fillText(value, x, y);
        }

        function minimap(race, x, y, size) {
            const track = race.track;
            const b = track.bounds;
            const spanX = Math.max(1, b.maxX - b.minX), spanY = Math.max(1, b.maxY - b.minY);
            const scale = Math.min((size - 24) / spanX, (size - 24) / spanY);
            const cx = x + size / 2, cy = y + size / 2;
            const mx = (b.minX + b.maxX) / 2, my = (b.minY + b.maxY) / 2;
            const toMap = (px, py) => [cx + (px - mx) * scale, cy + (py - my) * scale];
            c.beginPath();
            for (let i = 0; i < track.samples.length; i += 3) {
                const [sx, sy] = toMap(track.samples[i].x, track.samples[i].y);
                if (i === 0) c.moveTo(sx, sy); else c.lineTo(sx, sy);
            }
            c.closePath();
            c.strokeStyle = 'rgba(190,215,240,0.35)';
            c.lineWidth = Math.max(2, track.width * scale * 0.55);
            c.stroke();
            c.strokeStyle = 'rgba(230,245,255,0.75)'; c.lineWidth = 1; c.stroke();
            for (let i = 1; i < race.cars.length; i++) {
                const [sx, sy] = toMap(race.cars[i].v.x, race.cars[i].v.y);
                c.fillStyle = race.cars[i].v.car.colour;
                c.beginPath(); c.arc(sx, sy, 2.6, 0, Math.PI * 2); c.fill();
            }
            const [px, py] = toMap(race.cars[0].v.x, race.cars[0].v.y);
            c.fillStyle = '#ffffff';
            c.beginPath(); c.arc(px, py, 3.6, 0, Math.PI * 2); c.fill();
            c.strokeStyle = '#ff5a4e'; c.lineWidth = 1.5;
            c.beginPath(); c.arc(px, py, 6, 0, Math.PI * 2); c.stroke();
        }

        function bar(x, y, w, h, value, colour) {
            c.fillStyle = 'rgba(255,255,255,0.10)';
            c.fillRect(x, y, w, h);
            c.fillStyle = colour;
            c.fillRect(x, y, w * clamp(value, 0, 1), h);
        }

        return {
            resize,
            showMessage(value, seconds) { message = value; messageTime = seconds || 1.6; },
            render(race, dt, settings) {
                c.clearRect(0, 0, width, height);
                const player = race.cars[0];
                const v = player.v;
                const compact = width < 660;
                const pad = 14;

                panel(pad, pad, compact ? 178 : 214, 92);
                text(labels.lap + ' ' + Math.min(player.lap + 1, race.laps) + '/' + race.laps, pad + 12, pad + 26, 15, '#eef4ff');
                text(labels.position + ' ' + AD.playerPosition(race) + '/' + race.cars.length, pad + 12, pad + 50, 13, '#a9c4df');
                text(AD.formatTime(race.time), pad + 12, pad + 72, 14, '#eef4ff');
                text(labels.bestLap + ' ' + (isFinite(player.bestLap) ? AD.formatTime(player.bestLap) : '--:--.---'),
                    pad + 12, pad + 88, 11, '#8fa9c4');

                const mapSize = compact ? 122 : 158;
                panel(width - mapSize - pad, pad, mapSize, mapSize);
                minimap(race, width - mapSize - pad, pad, mapSize);

                const dialW = compact ? 168 : 216;
                panel(pad, height - dialW * 0.52 - pad, dialW, dialW * 0.52);
                text(String(Math.round(AD.vehicleSpeedKmh(v))), pad + 12, height - 34, compact ? 34 : 42, '#ffffff');
                text('km/h', pad + (compact ? 92 : 118), height - 36, 13, '#9fb8d4');
                text('G' + v.gear, pad + (compact ? 128 : 166), height - 36, 15, '#ffcf6b');
                bar(pad + 12, height - 24, dialW - 24, 5, v.rpm / 7200, v.rpm > 6300 ? '#ff8a5c' : '#5eead4');
                bar(pad + 12, height - 15, dialW - 24, 3, 1 - v.damage / 100, '#7fb2ff');

                const weaponW = compact ? 150 : 186;
                panel(width - weaponW - pad, height - 76 - pad, weaponW, 76);
                const def = player.weapon ? AD.WEAPONS[player.weapon] : null;
                text(labels.weapon, width - weaponW - pad + 12, height - 58 - pad, 11, '#8fa9c4');
                text(def ? def.name : '—', width - weaponW - pad + 12, height - 34 - pad, 17, def ? def.colour : '#9fb8d4');
                if (def) {
                    text('x' + player.ammo, width - pad - 12, height - 34 - pad, 14, '#eef4ff', 'right');
                    bar(width - weaponW - pad + 12, height - 24 - pad, weaponW - 24, 4,
                        1 - player.cooldown / def.cooldown, '#5eead4');
                }
                if (v.shield > 0) text('◈', width - weaponW - pad - 18, height - 34 - pad, 18, '#5eead4', 'center');

                if (race.state === 'countdown') {
                    const n = Math.ceil(race.countdown);
                    const label = n > 0 ? String(n) : labels.ready;
                    const pulse = settings.reducedMotion ? 0 : Math.sin(race.countdown * 9) * 0.08;
                    c.save();
                    c.globalAlpha = 0.9;
                    text(label, width / 2, height / 2 + 18, 62 + pulse * 40, '#ffffff', 'center');
                    c.restore();
                }
                if (messageTime > 0) {
                    messageTime -= dt;
                    c.globalAlpha = clamp(messageTime / 0.4, 0, 1);
                    text(message, width / 2, height * 0.28, 22, '#ffe6a8', 'center');
                    c.globalAlpha = 1;
                }
                if (race.state === 'paused') {
                    c.fillStyle = 'rgba(6,10,18,0.55)';
                    c.fillRect(0, 0, width, height);
                    text(labels.pause, width / 2, height / 2, 34, '#ffffff', 'center');
                }
            },
            dispose() { message = ''; messageTime = 0; }
        };
    };
})();
