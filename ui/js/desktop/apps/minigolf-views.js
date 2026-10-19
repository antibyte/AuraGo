import { DEFAULT_COLORS, colorValue } from './minigolf-shared.js';
import { createHud, formatPower } from './minigolf-hud.js';

export const esc = value => String(value ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));

// Inline 24 px line icons; .mg-icon strokes them with currentColor.
const ICONS = {
    scorecard: '<rect x="4" y="3" width="16" height="18" rx="2"/><path d="M8 8h8M8 12h8M8 16h5"/>',
    settings: '<path d="M4 6h10M18 6h2M4 12h4M12 12h8M4 18h12"/><circle cx="16" cy="6" r="2"/><circle cx="10" cy="12" r="2"/><circle cx="18" cy="18" r="2"/>',
    help: '<circle cx="12" cy="12" r="9"/><path d="M9.5 9.5a2.5 2.5 0 0 1 4.9.7c0 1.7-2.4 2.1-2.4 3.8M12 17.2v.1"/>',
    pause: '<path d="M9 5v14M15 5v14"/>', play: '<path d="M7 5l12 7-12 7z"/>', close: '<path d="M6 6l12 12M18 6L6 18"/>',
    ball: '<circle cx="12" cy="12" r="8"/><path d="M9 10h.01M12 8h.01M15 10h.01M10.5 13h.01M13.5 13h.01"/>',
    camera: '<path d="M4 8h3l2-3h6l2 3h3v11H4z"/><circle cx="12" cy="13" r="3.5"/>',
    map: '<path d="M3 6l6-2 6 2 6-2v14l-6 2-6-2-6 2z"/><path d="M9 4v14M15 6v14"/>',
    back: '<path d="M15 5l-7 7 7 7"/>', check: '<path d="M5 12.5l4.5 4.5L19 7"/>',
    star: '<path d="M12 3l2.7 5.6 6.1.9-4.4 4.3 1 6.1L12 17l-5.4 2.9 1-6.1-4.4-4.3 6.1-.9z"/>',
    flower: '<circle cx="12" cy="9" r="2"/><circle cx="12" cy="5" r="2"/><circle cx="16" cy="9" r="2"/><circle cx="12" cy="13" r="2"/><circle cx="8" cy="9" r="2"/><path d="M12 15v6M12 18l3-2"/>',
    wave: '<path d="M3 15c3 0 3-3 6-3s3 3 6 3 3-3 6-3M3 19c3 0 3-3 6-3s3 3 6 3 3-3 6-3M7 9c1-3 4-5 8-4"/>',
    lantern: '<path d="M10 3h4M12 3v2M8 7h8l-1 10H9z"/><path d="M9 20h6M12 10v4"/>',
    ship: '<path d="M4 16h16l-2 4H6z"/><path d="M12 3v13M12 4l6 9h-6M12 6l-5 7h5"/>',
    temple: '<path d="M3 20h18M5 20v-3h14v3M7 17v-3h10v3M9 14v-3h6v3M11 11V8h2v3"/>',
    moon: '<path d="M15 4a8 8 0 1 0 5 13A7 7 0 0 1 15 4z"/><circle cx="18" cy="7" r="1.5"/>',
    keyboard: '<rect x="3" y="6" width="18" height="12" rx="2"/><path d="M7 10h.01M11 10h.01M15 10h.01M7 14h10"/>',
    touch: '<path d="M9 11V5a2 2 0 0 1 4 0v6M13 10a2 2 0 0 1 4 0v4a6 6 0 0 1-6 6h-1a5 5 0 0 1-4-2l-3-4a1.6 1.6 0 0 1 2.4-2L9 15"/>',
    trophy: '<path d="M8 4h8v5a4 4 0 0 1-8 0zM8 6H5a3 3 0 0 0 3 4M16 6h3a3 3 0 0 1-3 4M12 13v4M8 20h8"/>',
    sound: '<path d="M4 10v4h4l5 4V6L8 10z"/><path d="M16 9a4 4 0 0 1 0 6M18.5 6.5a7.5 7.5 0 0 1 0 11"/>',
    display: '<rect x="3" y="5" width="18" height="12" rx="2"/><path d="M8 21h8M12 17v4"/>'
};
export const icon = name => `<svg class="mg-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">${ICONS[name] || ICONS.star}</svg>`;

export function createView(host, tr) {
    const serial = createView.serial = (createView.serial || 0) + 1;
    const root = document.createElement('div'); root.className = 'mg-app';
    root.dataset.world = 'neutral'; root.dataset.panel = 'loading'; root.dataset.reduced = 'false';
    const topButton = (action, iconName, key) => `<button class="mg-top-button" data-action="${action}" aria-label="${esc(tr(key))}" title="${esc(tr(key))}">${icon(iconName)}<span class="mg-label">${esc(tr(key))}</span></button>`;
    root.innerHTML = `<header class="mg-topbar">
            <div class="mg-brand"><img src="${esc(host.dataset.golfIcon)}" alt="" width="32" height="32"><strong class="mg-brand-name">AuraGolf</strong></div>
            <div class="mg-progress"><span class="mg-progress-label" data-progress-label>${esc(tr('subtitle'))}</span><ol class="mg-progress-track" data-progress-track aria-hidden="true"></ol></div>
            <nav class="mg-topbar-actions">${topButton('scorecard', 'scorecard', 'scorecard')}${topButton('settings', 'settings', 'settings')}${topButton('help', 'help', 'help_short')}${topButton('pause', 'pause', 'pause')}</nav>
        </header>
        <div class="mg-stage">
            <div class="mg-canvas"></div><div class="mg-hud" data-hud></div><div class="mg-coach" data-coach hidden></div>
            <div class="mg-loading" data-loading hidden><span class="mg-spinner" aria-hidden="true"></span><span>${esc(tr('loading'))}</span></div>
            <div class="mg-overlay" hidden></div><div class="mg-toasts" data-toasts aria-hidden="true"></div>
            <div class="mg-announcer" data-announcer role="status" aria-live="polite"></div>
        </div>
        <footer class="mg-controls">
            <div class="mg-power"><label for="mg-power-${serial}"><span>${esc(tr('power'))}</span><output data-power-value></output></label><input id="mg-power-${serial}" data-power type="range" min="2" max="100" value="35"></div>
            <button class="mg-primary" data-action="shoot">${icon('play')}<span class="mg-label">${esc(tr('shoot'))}</span></button>
            <button data-action="focus" aria-label="${esc(tr('focus'))}" title="${esc(tr('focus'))}">${icon('ball')}<span class="mg-label">${esc(tr('focus'))}</span></button>
            <button data-action="camera" aria-pressed="false" aria-label="${esc(tr('camera'))}" title="${esc(tr('camera'))}">${icon('camera')}<span class="mg-label">${esc(tr('camera'))}</span></button>
        </footer>`;
    host.replaceChildren(root);
    const $ = selector => root.querySelector(selector);
    const panel = $('.mg-overlay'), loadingEl = $('[data-loading]'), slider = $('[data-power]'), powerOut = $('[data-power-value]');
    const progressLabel = $('[data-progress-label]'), progressTrack = $('[data-progress-track]'), cameraButton = $('[data-action="camera"]');
    const hud = createHud({ hud: $('[data-hud]'), toasts: $('[data-toasts]'), announcer: $('[data-announcer]') }, { tr, esc, icon });
    let restoreFocus = null;
    const button = (action, label, primary = false) => `<button ${primary ? 'class="mg-primary"' : ''} data-action="${action}">${esc(tr(label))}</button>`;

    function showPanel(html, label, focus) {
        if (panel.hidden) restoreFocus = document.activeElement;
        panel.innerHTML = `<section class="mg-panel" role="dialog" aria-label="${esc(tr(label))}">${html}</section>`;
        panel.hidden = false;
        (focus && panel.querySelector(focus) || panel.querySelector('button:not(:disabled),input:not(:disabled),select:not(:disabled)'))?.focus({ preventScroll: true });
    }
    function progress(round) {
        if (!round) { progressLabel.textContent = tr('subtitle'); progressTrack.replaceChildren(); return; }
        const total = round.courseIds.length;
        progressLabel.textContent = tr('hole_progress').replace('{hole}', Math.min(round.holeIndex + 1, total)).replace('{total}', total);
        progressTrack.innerHTML = round.courseIds.map((_, index) => `<li data-state="${index < round.holeIndex ? 'done' : index === round.holeIndex ? 'current' : 'open'}"></li>`).join('');
    }
    function topbar(kind, hasRound) {
        const locked = kind === 'loading' || kind === 'error', idle = locked || kind === 'menu' || !hasRound;
        for (const [action, off] of [['scorecard', idle], ['pause', idle], ['settings', locked], ['help', locked]]) $(`.mg-topbar [data-action="${action}"]`).disabled = off;
    }
    // Tab stays on the open panel and the top bar.
    function trap(event) {
        if (event.key !== 'Tab' || panel.hidden) return;
        const controls = [...root.querySelectorAll('.mg-topbar button, .mg-overlay :is(button,input,select,textarea,summary,a[href])')].filter(el => !el.disabled && el.getClientRects().length);
        if (!controls.length) return;
        const first = controls[0], last = controls.at(-1);
        if (!controls.includes(document.activeElement)) { first.focus(); event.preventDefault(); }
        else if (event.shiftKey && document.activeElement === first) { last.focus(); event.preventDefault(); }
        else if (!event.shiftKey && document.activeElement === last) { first.focus(); event.preventDefault(); }
    }
    root.addEventListener('keydown', trap);

    // --- Transitional panels (replaced by minigolf-menus.js and minigolf-scorecard.js) ---
    function menu(courses, saved, values = {}) {
        const modes = ['full', 'garden', 'coast', 'night', 'practice'];
        showPanel(`<div class="mg-eyebrow">${esc(tr('subtitle'))}</div><h1>AuraGolf<span>✳</span></h1><p class="mg-tagline">${esc(tr('tagline'))}</p>
            <div class="mg-world-swatches"><span class="garden"></span><span class="coast"></span><span class="night"></span></div>
            <label>${esc(tr('mode'))}<select data-mode>${modes.map(mode => `<option value="${mode}"${values.mode === mode ? ' selected' : ''}>${esc(tr(mode))}</option>`).join('')}</select></label>
            <label data-hole-select hidden>${esc(tr('hole'))}<select data-hole-id>${courses.map(c => `<option value="${esc(c.id)}">${c.number}. ${esc(tr('course_' + c.number))}</option>`).join('')}</select></label>
            <label>${esc(tr('players'))}<select data-players>${[1, 2, 3, 4].map(n => `<option value="${n}"${values.count === n ? ' selected' : ''}>${n}</option>`).join('')}</select></label><div data-names class="mg-names"></div>
            <div class="mg-panel-actions">${button('start', 'start', true)}${saved ? button('resume', 'resume') : ''}</div><p class="mg-fine">${esc(tr('saved'))}</p>`, 'title');
        playerNames(values.count || 1);
        $('[data-hole-select]').hidden = values.mode !== 'practice';
    }
    function playerNames(count) {
        const holder = $('[data-names]'); if (!holder) return;
        const previous = Array.from(holder.querySelectorAll('input')).map(input => input.value);
        holder.innerHTML = Array.from({ length: count }, (_, i) => `<label><span style="--player-color:${colorValue(DEFAULT_COLORS[i])}">${esc(tr('player').replace('{n}', i + 1))}</span><input data-player-name maxlength="24" value="${esc(previous[i] || tr('player').replace('{n}', i + 1))}" autocomplete="off"></label>`).join('');
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
        showPanel(`<div class="mg-eyebrow">${esc(tr(complete ? 'finished' : 'scorecard'))}</div><h2>${esc(tr(complete ? 'finished' : 'scorecard'))}</h2><div class="mg-score-scroll"><table><thead><tr><th>${esc(tr('player_name'))}</th>${columns.map(c => `<th title="${esc(tr('course_' + c.number))}">${c.number}</th>`).join('')}<th>${esc(tr('total'))}</th></tr></thead><tbody>${round.players.map(p => `<tr><th><i style="background:${colorValue(p.color)}"></i>${esc(p.name)}</th>${columns.map((c, i) => { const value = p.scores[i] ?? (i === round.holeIndex && !complete ? p.strokes : null); if (value == null) return '<td>–</td>'; const rel = value - c.par; const grade = rel <= -2 ? 'mg-eagle' : rel === -1 ? 'mg-birdie' : rel === 0 ? 'mg-par' : rel === 1 ? 'mg-bogey' : 'mg-over'; const term = rel <= -2 ? tr('eagle') : rel === -1 ? tr('birdie') : rel === 0 ? tr('par') : rel === 1 ? tr('bogey') : tr('over'); return `<td class="${grade}" title="${esc(term)}">${value}</td>`; }).join('')}<td><strong>${p.scores.reduce((sum, s) => sum + s, 0) + p.strokes}</strong></td></tr>`).join('')}</tbody></table></div>
            ${best ? `<p>${esc(tr('best'))}: <strong>${best}</strong></p>` : ''}<div class="mg-panel-actions">${complete ? button('menu', 'play_again', true) : allDone ? button('next-hole', 'next_hole', true) : button('close', 'close', true)}</div>`, 'scorecard');
    }
    function pause() { showPanel(`<h2>${esc(tr('paused'))}</h2><p>${esc(tr('leave_hint'))}</p><div class="mg-panel-actions">${button('close', 'continue', true)}${button('menu', 'menu')}</div>`, 'pause'); }
    function error(key) { showPanel(`<h2>${esc(tr(key))}</h2>${button('retry', 'retry', true)}`, key); }
    function credits(tracks) { showPanel(`<h2>${esc(tr('credits'))}</h2><p>${esc(tr('credits_body'))}</p><ul>${tracks.map(t => `<li>${esc(t.title)} · ${esc(t.author)} · CC0</li>`).join('')}</ul>${button('close', 'close', true)}`, 'credits'); }

    return {
        root, canvasHost: $('.mg-canvas'), panel, hud, showPanel, progress, topbar,
        menu, playerNames, settings, help, scorecard, pause, error, credits,
        close() { panel.hidden = true; panel.replaceChildren(); if (restoreFocus?.isConnected && !restoreFocus.disabled) restoreFocus.focus({ preventScroll: true }); restoreFocus = null; },
        update(round, course, busy) {
            if (!round || !course) return;
            root.dataset.world = course.world || 'neutral'; root.dataset.course = course.id; root.dataset.turn = round.turnIndex; root.dataset.playing = String(busy);
            hud.update(round, course, busy); progress(round);
        },
        setCamera(pressed) {
            const text = tr(pressed ? 'aiming' : 'camera');
            cameraButton.setAttribute('aria-pressed', String(pressed)); cameraButton.setAttribute('aria-label', text); cameraButton.title = text;
            cameraButton.querySelector('.mg-label').textContent = text;
        },
        setReduced(on) { root.dataset.reduced = String(!!on); },
        loading(on) { loadingEl.hidden = !on; },
        power(strength, meters, lang) {
            const formatted = formatPower(strength, meters, lang);
            slider.value = Math.round(strength * 100); slider.setAttribute('aria-valuetext', formatted.text);
            powerOut.textContent = formatted.text;
            hud.power(strength, formatted);
        },
        track(value) { const el = $('[data-track]'); if (el) el.textContent = value?.title || '♪'; },
        dispose() { root.removeEventListener('keydown', trap); hud.dispose(); root.remove(); }
    };
}
