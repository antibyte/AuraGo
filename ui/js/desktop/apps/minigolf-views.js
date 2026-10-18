const esc = value => String(value ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
export const PLAYER_COLORS = ['#fff0c5', '#f6a378', '#89c9ef', '#c8a6ed'];

export function createView(host, tr) {
    const root = document.createElement('div'); root.className = 'mg-app';
    root.innerHTML = `<header class="mg-topbar"><div class="mg-brand"><img src="${esc(host.dataset.golfIcon)}" alt="" width="34" height="34"><strong>AuraGolf</strong></div><div class="mg-course-heading"><span data-world></span><strong data-course>${esc(tr('round_ready'))}</strong></div><nav><button data-action="scorecard">${esc(tr('scorecard'))}</button><button data-action="settings">${esc(tr('settings'))}</button><button data-action="pause">${esc(tr('pause'))}</button></nav></header>
        <div class="mg-stage"><div class="mg-canvas"></div><div class="mg-hole-badge"><span data-hole>${esc(tr('hole'))} 1</span><span data-par>${esc(tr('par'))} 2</span></div><div class="mg-minimap" data-minimap hidden aria-label="${esc(tr('hole_map'))}"></div><div class="mg-turn" aria-live="polite"><span data-turn></span><strong data-strokes></strong></div><div class="mg-overlay" hidden></div><div class="mg-status" role="status" aria-live="polite"></div></div>
        <footer class="mg-controls"><div class="mg-player-strip"></div><div class="mg-power"><label for="mg-power-${createView.serial = (createView.serial || 0) + 1}">${esc(tr('power'))} <output data-power-value>35%</output></label><input id="mg-power-${createView.serial}" data-power type="range" min="2" max="100" value="35"></div><button class="mg-primary" data-action="shoot">${esc(tr('shoot'))}</button><button data-action="focus">${esc(tr('focus'))}</button><button data-action="camera" aria-pressed="false">${esc(tr('camera'))}</button><button data-action="help">?</button></footer>`;
    root.querySelector('[data-action="help"]').setAttribute('aria-label', tr('help'));
    host.replaceChildren(root);
    const panel = root.querySelector('.mg-overlay'), status = root.querySelector('.mg-status');
    let restoreFocus = null;
    const button = (action, label, primary = false) => `<button ${primary ? 'class="mg-primary"' : ''} data-action="${action}">${esc(tr(label))}</button>`;
    function showPanel(html, label) {
        restoreFocus ||= document.activeElement;
        panel.innerHTML = `<section class="mg-panel" role="dialog" aria-modal="true" aria-label="${esc(tr(label))}">${html}</section>`;
        panel.hidden = false;
        panel.querySelector('button,input,select')?.focus({ preventScroll: true });
    }
    function menu(courses, saved, values = {}) {
        const modes = ['full', 'garden', 'coast', 'night', 'practice'];
        showPanel(`<div class="mg-eyebrow">${esc(tr('subtitle'))}</div><h1>AuraGolf<span>✳</span></h1><p class="mg-tagline">${esc(tr('tagline'))}</p>
            <div class="mg-world-swatches"><span class="garden"></span><span class="coast"></span><span class="night"></span></div>
            <label>${esc(tr('mode'))}<select data-mode>${modes.map(mode => `<option value="${mode}"${values.mode === mode ? ' selected' : ''}>${esc(tr(mode))}</option>`).join('')}</select></label>
            <label data-hole-select hidden>${esc(tr('hole'))}<select data-hole-id>${courses.map(c => `<option value="${esc(c.id)}">${c.number}. ${esc(tr('course_' + c.number))}</option>`).join('')}</select></label>
            <label>${esc(tr('players'))}<select data-players>${[1, 2, 3, 4].map(n => `<option value="${n}"${values.count === n ? ' selected' : ''}>${n}</option>`).join('')}</select></label><div data-names class="mg-names"></div>
            <div class="mg-panel-actions">${button('start', 'start', true)}${saved ? button('resume', 'resume') : ''}</div><p class="mg-fine">${esc(tr('saved'))}</p>`, 'title');
        playerNames(values.count || 1);
        root.querySelector('[data-hole-select]').hidden = values.mode !== 'practice';
    }
    function playerNames(count) {
        const holder = root.querySelector('[data-names]'); if (!holder) return;
        const previous = Array.from(holder.querySelectorAll('input')).map(input => input.value);
        holder.innerHTML = Array.from({ length: count }, (_, i) => `<label><span style="--player-color:${PLAYER_COLORS[i]}">${esc(tr('player').replace('{n}', i + 1))}</span><input data-player-name maxlength="40" value="${esc(previous[i] || tr('player').replace('{n}', i + 1))}" autocomplete="off"></label>`).join('');
    }
    function settings(settings, track) {
        showPanel(`<h2>${esc(tr('settings'))}</h2><label>${esc(tr('music'))}<input data-music type="range" min="0" max="100" value="${Math.round(settings.music * 100)}"></label><div class="mg-track"><span data-track>${esc(track?.title || '♪')}</span>${button('next-track', 'next_track')}${button('mute', settings.music === 0 ? 'unmute' : 'mute')}</div>
            <label>${esc(tr('effects'))}<input data-effects type="range" min="0" max="100" value="${Math.round(settings.effects * 100)}"></label><label>${esc(tr('quality'))}<select data-quality>${['auto', 'low', 'medium', 'high'].map(q => `<option value="${q}"${settings.quality === q ? ' selected' : ''}>${esc(tr(q))}</option>`).join('')}</select></label>
            <label class="mg-check"><input data-motion type="checkbox" ${settings.reducedMotion ? 'checked' : ''}>${esc(tr('motion'))}</label><label class="mg-check"><input data-follow type="checkbox" ${settings.follow !== false ? 'checked' : ''}>${esc(tr('follow_ball'))}</label><label class="mg-check"><input data-aim-camera type="checkbox" ${settings.aimCamera ? 'checked' : ''}>${esc(tr('aim_camera'))}</label><label class="mg-check"><input data-magnet type="checkbox" ${settings.magnet !== false ? 'checked' : ''}>${esc(tr('magnet'))}</label><div class="mg-panel-actions">${button('credits', 'credits')}${button('close', 'close', true)}</div>`, 'settings');
    }
    function help(tutorial = false) {
        showPanel(`<div class="mg-eyebrow">${esc(tr(tutorial ? 'tutorial' : 'help'))}</div><h2>${esc(tr('aim_hint'))}</h2><p>${esc(tr('tutorial_body'))}</p><p>${esc(tr('keyboard_hint'))}</p><p>${esc(tr('touch_hint'))}</p><p class="mg-fine">${esc(tr('score_hint'))}</p>${button('close', tutorial ? 'got_it' : 'close', true)}`, 'help');
    }
    function scorecard(round, courses, complete, best) {
        const allDone = round.players.every(p => p.done);
        const columns = round.courseIds.map(id => courses.find(c => c.id === id));
        showPanel(`<div class="mg-eyebrow">${esc(tr(complete ? 'finished' : 'scorecard'))}</div><h2>${esc(tr(complete ? 'finished' : 'scorecard'))}</h2><div class="mg-score-scroll"><table><thead><tr><th>${esc(tr('player_name'))}</th>${columns.map(c => `<th title="${esc(tr('course_' + c.number))}">${c.number}</th>`).join('')}<th>${esc(tr('total'))}</th></tr></thead><tbody>${round.players.map((p, index) => `<tr><th><i style="background:${PLAYER_COLORS[index]}"></i>${esc(p.name)}</th>${columns.map((c, i) => { const value = p.scores[i] ?? (i === round.holeIndex && !complete ? p.strokes : null); if (value == null) return '<td>–</td>'; const rel = value - c.par; const grade = rel <= -2 ? 'mg-eagle' : rel === -1 ? 'mg-birdie' : rel === 0 ? 'mg-par' : rel === 1 ? 'mg-bogey' : 'mg-over'; const term = rel <= -2 ? tr('eagle') : rel === -1 ? tr('birdie') : rel === 0 ? tr('par') : rel === 1 ? tr('bogey') : tr('over'); return `<td class="${grade}" title="${esc(term)}">${value}</td>`; }).join('')}<td><strong>${p.scores.reduce((sum, s) => sum + s, 0) + p.strokes}</strong></td></tr>`).join('')}</tbody></table></div>
            ${best ? `<p>${esc(tr('best'))}: <strong>${best}</strong></p>` : ''}<div class="mg-panel-actions">${complete ? button('menu', 'play_again', true) : allDone ? button('next-hole', 'next_hole', true) : button('close', 'close', true)}</div>`, 'scorecard');
    }
    function hud(round, course, busy) {
        holePlan(course);
        root.dataset.course = course.id; root.dataset.turn = round.turnIndex; root.dataset.playing = String(busy);
        root.querySelector('[data-world]').textContent = tr(course.world);
        root.querySelector('[data-course]').textContent = tr('course_' + course.number);
        root.querySelector('[data-hole]').textContent = `${tr('hole')} ${course.number}`;
        root.querySelector('[data-par]').textContent = `${tr('par')} ${course.par}`;
        const player = round.players[round.turnIndex];
        root.querySelector('[data-turn]').textContent = player.name + ' · ' + tr(busy ? 'rolling' : 'turn');
        root.querySelector('[data-strokes]').textContent = `${tr('strokes')} ${player.strokes}`;
        root.querySelector('.mg-player-strip').innerHTML = round.players.map((p, i) => `<span class="${i === round.turnIndex ? 'active' : ''}" style="--player-color:${PLAYER_COLORS[i]}"><i></i>${esc(p.name)} <b>${p.strokes}${p.done ? ' ✓' : ''}</b></span>`).join('');
        root.querySelector('[data-action="shoot"]').disabled = busy || round.players.every(p => p.done);
    }
    panel.addEventListener('keydown', event => {
        if (event.key !== 'Tab') return;
        const controls = Array.from(panel.querySelectorAll('button,input,select,a[href]')).filter(el => !el.disabled && el.getClientRects().length);
        if (!controls.length) return;
        const first = controls[0], last = controls.at(-1);
        if (event.shiftKey && document.activeElement === first) { last.focus(); event.preventDefault(); }
        else if (!event.shiftKey && document.activeElement === last) { first.focus(); event.preventDefault(); }
    });
    let mapKey = null, mapProject = null, mapDots = [];
    function holePlan(course) {
        const holder = root.querySelector('[data-minimap]');
        if (!holder) return;
        if (!course || !Array.isArray(course.bounds) || course.bounds.length < 2) {
            holder.hidden = true; mapKey = null; mapProject = null; mapDots = [];
            return;
        }
        if (mapKey === course.id) { holder.hidden = false; return; }
        mapKey = course.id; holder.hidden = false; mapDots = [];
        const xs = [course.bounds[0][0], course.bounds[1][0]].sort((a, b) => a - b);
        const zs = [course.bounds[0][2], course.bounds[1][2]].sort((a, b) => a - b);
        const span = Math.max(xs[1] - xs[0], zs[1] - zs[0]) || 1;
        const pad = 8, box = 84;
        const px = value => pad + (value - xs[0]) / span * box;
        const pz = value => pad + (value - zs[0]) / span * box;
        mapProject = { px, pz };
        const water = (course.water || []).map(volume => {
            const x1 = px(Math.min(volume[0][0], volume[1][0])), x2 = px(Math.max(volume[0][0], volume[1][0]));
            const z1 = pz(Math.min(volume[0][2], volume[1][2])), z2 = pz(Math.max(volume[0][2], volume[1][2]));
            return `<rect class="map-water" x="${x1.toFixed(2)}" y="${z1.toFixed(2)}" width="${Math.max(3, x2 - x1).toFixed(2)}" height="${Math.max(3, z2 - z1).toFixed(2)}" rx="2"></rect>`;
        }).join('');
        const field = `<rect class="map-field" x="${px(xs[0]).toFixed(2)}" y="${pz(zs[0]).toFixed(2)}" width="${((xs[1] - xs[0]) / span * box).toFixed(2)}" height="${((zs[1] - zs[0]) / span * box).toFixed(2)}" rx="8"></rect>`;
        const tee = course.tee || [0, 0, 0], cup = course.cup || [0, 0, 0];
        holder.innerHTML = `<svg viewBox="0 0 100 100" aria-hidden="true">${field}${water}<line class="map-path" x1="${px(tee[0]).toFixed(2)}" y1="${pz(tee[2]).toFixed(2)}" x2="${px(cup[0]).toFixed(2)}" y2="${pz(cup[2]).toFixed(2)}"></line><circle class="map-tee" cx="${px(tee[0]).toFixed(2)}" cy="${pz(tee[2]).toFixed(2)}" r="2.4"></circle><circle class="map-cup" cx="${px(cup[0]).toFixed(2)}" cy="${pz(cup[2]).toFixed(2)}" r="3"></circle><g data-map-balls></g></svg>`;
    }
    function mapBalls(positions, active) {
        const holder = root.querySelector('[data-map-balls]');
        if (!holder || !mapProject || !positions?.length) return;
        const list = positions.filter(position => position && position.length >= 3);
        if (mapDots.length !== list.length) {
            holder.innerHTML = list.map((_, index) => `<circle r="3.6" fill="${PLAYER_COLORS[index % PLAYER_COLORS.length]}" stroke="#0b1612" stroke-width="1"></circle>`).join('');
            mapDots = Array.from(holder.children);
        }
        list.forEach((position, index) => {
            const dot = mapDots[index]; if (!dot) return;
            dot.setAttribute('cx', mapProject.px(position[0]).toFixed(2));
            dot.setAttribute('cy', mapProject.pz(position[2]).toFixed(2));
            dot.setAttribute('r', index === active ? '4.6' : '3.6');
        });
    }
    return {
        root, canvasHost: root.querySelector('.mg-canvas'), panel, menu, playerNames, settings, help, scorecard, hud, showPanel, mapBalls,
        notice(key) { status.textContent = key ? tr(key) : ''; },
        close() { panel.hidden = true; panel.replaceChildren(); if (restoreFocus?.isConnected) restoreFocus.focus({ preventScroll: true }); restoreFocus = null; },
        pause() { showPanel(`<h2>${esc(tr('paused'))}</h2><p>${esc(tr('leave_hint'))}</p><div class="mg-panel-actions">${button('close', 'continue', true)}${button('menu', 'menu')}</div>`, 'pause'); },
        error(key) { showPanel(`<h2>${esc(tr(key))}</h2>${button('retry', 'retry', true)}`, key); },
        power(value, meters) {
            const percent = Math.round(value * 100);
            root.querySelector('[data-power]').value = percent;
            const distance = Number.isFinite(meters) ? (meters < 1 ? Math.round(meters * 10) / 10 : meters < 10 ? Math.round(meters * 2) / 2 : Math.round(meters)) : null;
            root.querySelector('[data-power-value]').textContent = percent + '%' + (distance === null ? '' : ' · ≈' + distance.toLocaleString(document.documentElement.lang || undefined) + ' m');
        },
        track(value) { const el = root.querySelector('[data-track]'); if (el) el.textContent = value?.title || '♪'; },
        credits(tracks) { showPanel(`<h2>${esc(tr('credits'))}</h2><p>${esc(tr('credits_body'))}</p><ul>${tracks.map(t => `<li>${esc(t.title)} · ${esc(t.author)} · CC0</li>`).join('')}</ul>${button('close', 'close', true)}`, 'credits'); }
    };
}
