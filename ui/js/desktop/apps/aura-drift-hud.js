/* Aura Drift - in-race HUD, style meter, minimap and countdown on an overlay canvas. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;

    AD.createHud = function (canvas, labels) {
        const c = canvas.getContext('2d');
        let width = 960, height = 540, dpr = 1;
        let message = '', messageTime = 0;
        const popups = [];

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
            const g = c.createLinearGradient(x, y, x, y + h);
            g.addColorStop(0, 'rgba(16,22,34,0.72)');
            g.addColorStop(1, 'rgba(10,14,22,0.62)');
            c.fillStyle = g;
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

        function bar(x, y, w, h, value, colour) {
            c.fillStyle = 'rgba(255,255,255,0.10)';
            c.fillRect(x, y, w, h);
            c.fillStyle = colour;
            c.fillRect(x, y, w * clamp(value, 0, 1), h);
        }

        /* An arc gauge reads the engine faster than a straight bar: the eye
           follows the sweep of the needle. */
        function rpmArc(x, y, r, value, danger) {
            const start = Math.PI * 0.75;
            const end = Math.PI * 2.25;
            c.lineWidth = Math.max(3, r * 0.16);
            c.lineCap = 'round';
            c.strokeStyle = 'rgba(255,255,255,0.10)';
            c.beginPath();
            c.arc(x, y, r, start, end);
            c.stroke();
            c.strokeStyle = danger ? '#ff8a5c' : '#5eead4';
            c.beginPath();
            c.arc(x, y, r, start, start + (end - start) * clamp(value, 0, 1));
            c.stroke();
            /* Redline marker. */
            c.strokeStyle = 'rgba(255,90,78,0.75)';
            c.lineWidth = Math.max(1.5, r * 0.06);
            c.beginPath();
            c.arc(x, y, r, start + (end - start) * 0.87, end);
            c.stroke();
            c.lineCap = 'butt';
        }

        function sectorColour(index, time, best) {
            if (!isFinite(time)) return '#5c7085';
            if (!isFinite(best)) return '#7dd3fc';
            if (time <= best) return '#c084fc';
            if (time <= best * 1.02) return '#7dd3fc';
            return '#ff9d7a';
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
            c.strokeStyle = 'rgba(24,32,46,0.9)';
            c.lineWidth = Math.max(3, track.width * scale * 0.8);
            c.stroke();
            /* Sector colouring: three arcs in three colours so the driver can
               see which part of the lap is holding them back. */
            const sectors = track.sectors || [];
            const length = track.length || 1;
            const palette = ['rgba(125,211,252,0.6)', 'rgba(255,209,102,0.6)', 'rgba(192,132,252,0.6)'];
            for (let i = 0; i < track.samples.length; i++) {
                const s = track.samples[i];
                const t = s.s / length;
                let idx = 0;
                while (idx < sectors.length && t >= sectors[idx] / length) idx++;
                if (i % 2) continue;
                const [sx, sy] = toMap(s.x, s.y);
                c.fillStyle = palette[idx % palette.length];
                c.fillRect(sx - 1, sy - 1, 2, 2);
            }
            c.strokeStyle = 'rgba(190,215,240,0.55)';
            c.lineWidth = Math.max(2, track.width * scale * 0.5);
            c.stroke();
            /* Start line tick marks the loop origin. */
            const [s0x, s0y] = toMap(track.samples[0].x, track.samples[0].y);
            c.fillStyle = 'rgba(240,244,248,0.85)';
            c.fillRect(s0x - 1.5, s0y - 1.5, 3, 3);
            for (let i = 1; i < race.cars.length; i++) {
                const [sx, sy] = toMap(race.cars[i].v.x, race.cars[i].v.y);
                c.fillStyle = race.cars[i].v.car.colour;
                c.beginPath(); c.arc(sx, sy, 2.8, 0, Math.PI * 2); c.fill();
                c.strokeStyle = 'rgba(8,10,16,0.6)';
                c.lineWidth = 1;
                c.stroke();
            }
            if (race.ghost && race.ghost.active) {
                const [gx, gy] = toMap(race.ghost.v.x, race.ghost.v.y);
                c.strokeStyle = 'rgba(159,214,255,0.8)'; c.lineWidth = 1.2;
                c.beginPath(); c.arc(gx, gy, 3, 0, Math.PI * 2); c.stroke();
            }
            const player = race.cars[0].v;
            const [px, py] = toMap(player.x, player.y);
            c.save();
            c.translate(px, py); c.rotate(player.psi);
            c.fillStyle = '#ffffff';
            c.beginPath();
            c.moveTo(5, 0); c.lineTo(-3.4, 3.2); c.lineTo(-3.4, -3.2);
            c.closePath(); c.fill();
            c.restore();
            c.strokeStyle = '#ff5a4e'; c.lineWidth = 1.5;
            c.beginPath(); c.arc(px, py, 6, 0, Math.PI * 2); c.stroke();
        }

        function startLights(race) {
            const total = 5;
            const lit = clamp(Math.ceil(race.countdown), 0, total);
            const spacing = 34;
            const startX = width / 2 - (total - 1) * spacing / 2;
            for (let i = 0; i < total; i++) {
                const on = i < lit;
                const go = race.countdown <= 0.6;
                c.beginPath();
                c.arc(startX + i * spacing, height / 2 - 44, 12, 0, Math.PI * 2);
                c.fillStyle = on ? (go ? '#37e07c' : '#ff4d4d') : 'rgba(255,255,255,0.10)';
                c.fill();
                c.strokeStyle = 'rgba(230,240,255,0.35)'; c.lineWidth = 2; c.stroke();
            }
        }

        /* Style meter: pending points, the live multiplier and the time left
           to link the next drift into the chain. */
        function styleMeter(race, x, y, w) {
            const style = race.cars[0].style;
            if (!style) return 0;
            const pending = style.raw * style.multiplier;
            const active = style.drifting || style.timer > 0;
            const h = 58;
            panel(x, y, w, h);
            text(labels.style, x + 12, y + 18, 11, '#8fa9c4');
            const total = style.banked + pending;
            text(String(Math.round(total)), x + w - 12, y + 22, 19, active ? '#ffffff' : '#cfe0f2', 'right', 700);
            if (style.multiplier > 1) {
                text('x' + style.multiplier.toFixed(1), x + 12, y + 22, 16, '#ffd166', 'left', 700);
            }
            /* Chain timer: the bar drains while the link window is open. */
            const link = clamp(style.timer / (AD.STYLE ? AD.STYLE.comboWindow : 1.5), 0, 1);
            bar(x + 12, y + 28, w - 24, 6, active ? 1 : 0, style.multiplier > 2 ? '#ffd166' : '#5eead4');
            if (active && link > 0) {
                c.fillStyle = 'rgba(255,255,255,0.5)';
                c.fillRect(x + 12 + (w - 24) * link - 2, y + 28, 3, 6);
            }
            if (style.nearMisses > 0) {
                text(labels.combo + ' ' + style.nearMisses, x + w - 12, y + 46, 10, '#7dd3fc', 'right');
            }
            return h;
        }

        function pushPopup(textValue, colour) {
            popups.push({ text: textValue, colour: colour || '#ffffff', life: 1.4, max: 1.4 });
            if (popups.length > 6) popups.shift();
        }

        function drawPopups(dt) {
            for (let i = popups.length - 1; i >= 0; i--) {
                const p = popups[i];
                p.life -= dt;
                if (p.life <= 0) { popups.splice(i, 1); continue; }
                const t = 1 - p.life / p.max;
                c.globalAlpha = clamp(p.life / 0.5, 0, 1);
                const y = height * 0.42 - t * 46;
                c.font = '700 22px system-ui, sans-serif';
                c.textAlign = 'center';
                c.textBaseline = 'alphabetic';
                c.fillStyle = 'rgba(8,10,16,0.6)';
                c.fillText(p.text, width / 2 + 1, y + 1);
                c.fillStyle = p.colour;
                c.fillText(p.text, width / 2, y);
            }
            c.globalAlpha = 1;
            c.textAlign = 'left';
        }

        return {
            resize,
            showMessage(value, seconds) { message = value; messageTime = seconds || 1.6; },
            popup(value, colour) { pushPopup(value, colour); },
            render(race, dt, settings) {
                c.clearRect(0, 0, width, height);
                const player = race.cars[0];
                const v = player.v;
                const compact = width < 660;
                const pad = 14;

                const infoW = compact ? 186 : 226;
                panel(pad, pad, infoW, 108);
                text(labels.lap + ' ' + Math.min(player.lap + 1, race.laps) + '/' + race.laps, pad + 12, pad + 26, 15, '#eef4ff');
                text(labels.position + ' ' + AD.playerPosition(race) + '/' + race.cars.length, pad + 12, pad + 48, 13, '#a9c4df');
                text(AD.formatTime(race.time), pad + 12, pad + 70, 14, '#eef4ff');
                text(labels.bestLap + ' ' + (isFinite(player.bestLap) ? AD.formatTime(player.bestLap) : '--:--.---'),
                    pad + 12, pad + 86, 11, '#8fa9c4');

                /* Sector splits: the three purple/green/amber boxes. */
                const bests = player.bestSectors || [];
                const sectorW = (infoW - 24) / 3;
                for (let i = 0; i < 3; i++) {
                    const x = pad + 12 + i * sectorW;
                    const value = bests[i];
                    c.fillStyle = 'rgba(255,255,255,0.06)';
                    c.fillRect(x, pad + 94, sectorW - 4, 12);
                    c.fillStyle = sectorColour(i, value, Infinity);
                    c.font = '600 9px system-ui, sans-serif';
                    c.textAlign = 'center';
                    c.fillText('S' + (i + 1), x + (sectorW - 4) / 2, pad + 103);
                    c.textAlign = 'left';
                    if (isFinite(value)) {
                        c.fillStyle = sectorColour(i, value, bests[i]);
                        c.font = '600 10px system-ui, sans-serif';
                        c.textAlign = 'center';
                        c.fillText(AD.formatTime(value).slice(3), x + (sectorW - 4) / 2, pad + 103);
                        c.textAlign = 'left';
                    }
                }

                /* Live tower: order, colour chips and gaps in metres. */
                const towerH = 24 + race.cars.length * 18;
                panel(pad, pad + 118, infoW, towerH);
                const order = (race.standings && race.standings.length ? race.standings : race.cars.slice());
                for (let i = 0; i < order.length; i++) {
                    const entry = order[i];
                    const y = pad + 118 + 22 + i * 18;
                    c.fillStyle = entry.v.car.colour;
                    c.beginPath(); c.arc(pad + 20, y - 4, 4, 0, Math.PI * 2); c.fill();
                    const gap = i === 0 ? '' : '  +' + Math.max(0, Math.round(order[0].progress - entry.progress)) + 'm';
                    text('P' + (i + 1) + (entry.isPlayer ? ' •' : '') + gap,
                        pad + 30, y, 12, entry.isPlayer ? '#ffffff' : '#a9c4df');
                }

                const mapSize = compact ? 122 : 158;
                panel(width - mapSize - pad, pad, mapSize, mapSize);
                minimap(race, width - mapSize - pad, pad, mapSize);
                /* Condition readout under the map: the weather is part of the
                   race, so it belongs on the glass. */
                if (race.conditions) {
                    const kind = race.conditions.state.kind;
                    const intensity = Math.round(race.conditions.state.intensity * 100);
                    text(labels.conditions + ' ' + kind + ' ' + intensity + '%',
                        width - mapSize - pad + 10, pad + mapSize + 14, 10, '#8fa9c4');
                }

                const dialW = compact ? 172 : 224;
                const dialH = dialW * 0.62;
                const dialX = pad, dialY = height - dialH - pad;
                panel(dialX, dialY, dialW, dialH);
                const arcR = Math.min(dialH * 0.42, 34);
                const arcX = dialX + arcR + 14, arcY = dialY + arcR + 16;
                rpmArc(arcX, arcY, arcR, v.rpm / 7200, v.rpm > 6300);
                text(String(Math.max(0, Math.round(AD.vehicleSpeedKmh(v)))), dialX + 12, dialY + 46, compact ? 34 : 42, '#ffffff');
                text('km/h', dialX + (compact ? 96 : 124), dialY + 48, 13, '#9fb8d4');
                text(v.gear === 0 ? 'R' : 'G' + v.gear, dialX + dialW - 14, dialY + 48, 15, '#ffcf6b', 'right');
                bar(dialX + 12, dialY + 56, dialW - 24, 4, 1 - v.damage / 100, '#7fb2ff');
                /* Nitro resource: filled by drifting, spent by boost. */
                const drifting = !!(player.style && player.style.drifting);
                text(labels.nitro + (drifting ? '  +DRIFT' : ''), dialX + 12, dialY + 76, 10, drifting ? '#7dd3fc' : '#8fa9c4');
                bar(dialX + 12, dialY + 80, dialW - 24, 5, v.nitro === undefined ? 0 : v.nitro, v.nitro > 0.98 ? '#ffd166' : '#7dd3fc');
                if (v.slipstream > 0.25) {
                    text('» SLIP', dialX + dialW - 12, dialY + 76, 11, '#ffd166', 'right');
                }

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

                /* Style meter sits above the weapon panel so it never covers
                   the driving line. */
                const meterW = compact ? 172 : 224;
                styleMeter(race, width - meterW - pad, height - 76 - pad - 70, meterW);

                if (race.state === 'countdown') {
                    if (!settings.reducedMotion) startLights(race);
                    const n = Math.ceil(race.countdown);
                    const label = n > 0 ? String(n) : labels.ready;
                    const pulse = settings.reducedMotion ? 0 : Math.sin(race.countdown * 9) * 0.08;
                    c.save();
                    c.globalAlpha = 0.9;
                    text(label, width / 2, height / 2 + 34, 62 + pulse * 40, '#ffffff', 'center');
                    c.restore();
                }
                if (messageTime > 0) {
                    messageTime -= dt;
                    c.globalAlpha = clamp(messageTime / 0.4, 0, 1);
                    text(message, width / 2, height * 0.28, 22, '#ffe6a8', 'center');
                    c.globalAlpha = 1;
                }
                drawPopups(dt);
                if (race.state === 'paused') {
                    c.fillStyle = 'rgba(6,10,18,0.55)';
                    c.fillRect(0, 0, width, height);
                    text(labels.pause, width / 2, height / 2, 34, '#ffffff', 'center');
                }
            },
            dispose() { message = ''; messageTime = 0; popups.length = 0; }
        };
    };
})();
