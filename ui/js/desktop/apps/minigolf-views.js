const esc = value => String(value ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
export const PLAYER_COLORS = ['#fff0c5', '#f6a378', '#89c9ef', '#c8a6ed'];

export function createView(host, tr) {
    const root = document.createElement('div'); root.className = 'mg-app';
    root.innerHTML = `<header class="mg-topbar"><div class="mg-brand"><img src="${esc(host.dataset.golfIcon)}" alt="" width="34" height="34"><strong>AuraGolf</strong></div><div class="mg-course-heading"><span data-world></span><strong data-course>${esc(tr('round_ready'))}</strong></div><nav><button data-action="scorecard">${esc(tr('scorecard'))}</button><button data-action="settings">${esc(tr('settings'))}</button><button data-action="pause">${esc(tr('pause'))}</button></nav></header>
        <div class="mg-stage"><div class="mg-canvas"></div><div class="mg-hole-badge"><span data-hole>${esc(tr('hole'))} 1</span><span data-par>${esc(tr('par'))} 2</span></div><div class="mg-turn" aria-live="polite"><span data-turn></span><strong data-strokes></strong></div><div class="mg-overlay" hidden></div><div class="mg-status" role="status" aria-live="polite"></div></div>
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
            <label class="mg-check"><input data-motion type="checkbox" ${settings.reducedMotion ? 'checked' : ''}>${esc(tr('motion'))}</label><div class="mg-panel-actions">${button('credits', 'credits')}${button('close', 'close', true)}</div>`, 'settings');
    }
    function help(tutorial = false) {
        showPanel(`<div class="mg-eyebrow">${esc(tr(tutorial ? 'tutorial' : 'help'))}</div><h2>${esc(tr('aim_hint'))}</h2><p>${esc(tr('tutorial_body'))}</p><p>${esc(tr('keyboard_hint'))}</p><p>${esc(tr('touch_hint'))}</p><p class="mg-fine">${esc(tr('score_hint'))}</p>${button('close', tutorial ? 'got_it' : 'close', true)}`, 'help');
    }
    function scorecard(round, courses, complete, best) {
        const allDone = round.players.every(p => p.done);
        const columns = round.courseIds.map(id => courses.find(c => c.id === id));
        showPanel(`<div class="mg-eyebrow">${esc(tr(complete ? 'finished' : 'scorecard'))}</div><h2>${esc(tr(complete ? 'finished' : 'scorecard'))}</h2><div class="mg-score-scroll"><table><thead><tr><th>${esc(tr('player_name'))}</th>${columns.map(c => `<th title="${esc(tr('course_' + c.number))}">${c.number}</th>`).join('')}<th>${esc(tr('total'))}</th></tr></thead><tbody>${round.players.map((p, index) => `<tr><th><i style="background:${PLAYER_COLORS[index]}"></i>${esc(p.name)}</th>${columns.map((_, i) => `<td>${p.scores[i] ?? (i === round.holeIndex && !complete ? p.strokes : '–')}</td>`).join('')}<td><strong>${p.scores.reduce((sum, s) => sum + s, 0) + p.strokes}</strong></td></tr>`).join('')}</tbody></table></div>
            ${best ? `<p>${esc(tr('best'))}: <strong>${best}</strong></p>` : ''}<div class="mg-panel-actions">${complete ? button('menu', 'play_again', true) : allDone ? button('next-hole', 'next_hole', true) : button('close', 'close', true)}</div>`, 'scorecard');
    }
    function hud(round, course, busy) {
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
    return {
        root, canvasHost: root.querySelector('.mg-canvas'), panel, menu, playerNames, settings, help, scorecard, hud, showPanel,
        notice(key) { status.textContent = key ? tr(key) : ''; },
        close() { panel.hidden = true; panel.replaceChildren(); if (restoreFocus?.isConnected) restoreFocus.focus({ preventScroll: true }); restoreFocus = null; },
        pause() { showPanel(`<h2>${esc(tr('paused'))}</h2><p>${esc(tr('leave_hint'))}</p><div class="mg-panel-actions">${button('close', 'continue', true)}${button('menu', 'menu')}</div>`, 'pause'); },
        error(key) { showPanel(`<h2>${esc(tr(key))}</h2>${button('retry', 'retry', true)}`, key); },
        power(value) { root.querySelector('[data-power]').value = Math.round(value * 100); root.querySelector('[data-power-value]').textContent = Math.round(value * 100) + '%'; },
        track(value) { const el = root.querySelector('[data-track]'); if (el) el.textContent = value?.title || '♪'; },
        credits(tracks) { showPanel(`<h2>${esc(tr('credits'))}</h2><p>${esc(tr('credits_body'))}</p><ul>${tracks.map(t => `<li>${esc(t.title)} · ${esc(t.author)} · CC0</li>`).join('')}</ul>${button('close', 'close', true)}`, 'credits'); }
    };
}
