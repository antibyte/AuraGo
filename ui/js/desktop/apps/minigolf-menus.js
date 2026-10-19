import { PALETTE, colorValue, fillText } from './minigolf-shared.js';

export const WORLD_ICONS = { garden: 'flower', coast: 'wave', night: 'lantern', pirate: 'ship', temple: 'temple', moon: 'moon' };

export function roundLabel(mode, holeId, courses, tr) {
    if (mode === 'full') return tr('full');
    if (mode !== 'practice') return tr(mode);
    const course = courses.find(item => item.id === holeId);
    return tr('training_named').replace('{hole}', () => course ? course.number + '. ' + tr('course_' + course.number) : '');
}

// Panels of the start flow, settings, help, pause, credits and errors. The view hands in
// its panel plumbing and helpers, so this module imports no other UI module.
export function createMenus({ showPanel, panel, tr, esc, icon, assetURL }) {
    const serial = createMenus.serial = (createMenus.serial || 0) + 1;
    const bestText = value => value ? fillText(tr('best_value'), { n: value }) : tr('no_best');
    const button = (action, text, primary = false) => `<button ${primary ? 'class="mg-primary" ' : ''}data-action="${action}">${esc(text)}</button>`;
    const stepHead = title => `<header class="mg-step-head"><button class="mg-back" data-action="back">${icon('back')}<span>${esc(tr('back'))}</span></button><h2>${esc(title)}</h2></header>`;
    const section = (cls, iconName, title, body) => `<section class="${cls}"><h3>${icon(iconName)}<span>${esc(title)}</span></h3>${body}</section>`;

    // A failed preview image falls back to the drawn placeholder underneath it.
    panel.addEventListener('error', event => { if (event.target instanceof HTMLImageElement && event.target.closest('.mg-preview')) event.target.remove(); }, true);

    function roundCard(cls, round, world, body) {
        return `<button class="mg-round-card ${cls}" data-round="${esc(round)}" data-world="${esc(world)}">${body}</button>`;
    }
    function home({ courses, worlds, best, saved }, focus) {
        const bests = best?.rounds || {}, par = list => list.reduce((sum, course) => sum + course.par, 0);
        const meta = list => fillText(tr('world_detail'), { n: list.length, par: par(list) });
        const worldCard = world => {
            const own = courses.filter(course => course.world === world.id);
            const image = world.preview ? `<img alt="" loading="lazy" decoding="async" src="${esc(assetURL('/3d/minigolf/' + world.preview))}">` : '';
            return roundCard('', world.id, world.id, `<span class="mg-preview"><span class="mg-island" aria-hidden="true"></span>${icon(WORLD_ICONS[world.id] || 'star')}${image}</span><span class="mg-round-title">${esc(tr(world.id))}</span><span class="mg-round-meta">${esc(meta(own))}</span><span class="mg-round-best">${esc(bestText(bests[world.id]))}</span>`);
        };
        const resume = saved ? `<article class="mg-resume-card"><div><h2>${esc(tr('resume'))}</h2><p>${esc(fillText(tr('resume_detail'), { hole: saved.hole, total: saved.total, players: saved.players }))}</p><p class="mg-muted">${esc(roundLabel(saved.mode, saved.holeId, courses, tr))}</p></div><button class="mg-primary" data-action="resume">${icon('play')}<span>${esc(tr('continue'))}</span></button></article>` : '';
        const full = roundCard('mg-round-full', 'full', 'full', `<span class="mg-round-stripes" aria-hidden="true">${worlds.map(world => `<i data-world="${esc(world.id)}"></i>`).join('')}</span><span class="mg-round-title">${esc(tr('full'))}</span><span class="mg-round-meta">${esc(meta(courses))}</span><span class="mg-round-best">${esc(bestText(bests.full))}</span>`);
        const practice = roundCard('mg-round-practice', 'practice', 'neutral', `<span class="mg-round-title">${esc(tr('training'))}</span><span class="mg-round-meta">${esc(tr('training_detail'))}</span>`);
        showPanel(`<header class="mg-menu-head"><h1 class="mg-title">AuraGolf</h1><p class="mg-tagline">${esc(tr('tagline'))}</p></header>${resume}<h2 class="mg-section-title">${esc(tr('choose_round'))}</h2><div class="mg-round-grid">${full}${worlds.map(worldCard).join('')}${practice}</div><p class="mg-fine">${esc(tr('saved'))}</p>`, 'title', focus);
    }
    function holes({ courses, worlds, best }, focus) {
        const bests = best?.holes || {};
        const tile = course => `<button class="mg-hole-tile" data-hole-id="${esc(course.id)}"><span class="mg-hole-number">${esc(course.number)}</span><span class="mg-hole-name">${esc(tr('course_' + course.number))}</span><span class="mg-hole-par">${esc(tr('par'))} ${esc(course.par)}</span><span class="mg-hole-best">${esc(bests[course.id] ? fillText(tr('best_value'), { n: bests[course.id] }) : '–')}</span></button>`;
        const group = world => `<section class="mg-hole-group" data-world="${esc(world.id)}"><h3 class="mg-group-title">${esc(tr(world.id))}</h3><div class="mg-hole-grid">${courses.filter(course => course.world === world.id).map(tile).join('')}</div></section>`;
        showPanel(`${stepHead(tr('pick_hole'))}${worlds.map(group).join('')}`, 'pick_hole', focus);
    }
    function players({ courses, setup }, focus) {
        const list = setup.players;
        const counts = [1, 2, 3, 4].map(n => `<label><input type="radio" name="mg-count-${serial}" data-player-count value="${n}"${list.length === n ? ' checked' : ''}><span>${n}</span></label>`).join('');
        const swatches = (player, index) => {
            const blocked = new Set(list.filter((_, other) => other !== index).map(entry => entry.color));
            return PALETTE.map(color => `<label class="mg-swatch" style="--mg-swatch:${esc(color.hex)}"><input type="radio" name="mg-color-${serial}-${index}" data-player-color value="${esc(color.id)}" aria-label="${esc(tr('color_' + color.id))}"${player.color === color.id ? ' checked' : ''}${blocked.has(color.id) ? ' disabled' : ''}><span aria-hidden="true"></span></label>`).join('');
        };
        const card = (player, index) => `<fieldset class="mg-player-card" data-player-index="${index}" style="--mg-player:${esc(colorValue(player.color))}"><legend>${esc(fillText(tr('player'), { n: index + 1 }))}</legend><div class="mg-player-fields"><fieldset class="mg-swatches"><legend class="mg-visually-hidden">${esc(tr('color'))}</legend>${swatches(player, index)}</fieldset><label class="mg-field"><span>${esc(tr('player_name'))}</span><input data-player-name maxlength="24" value="${esc(player.name)}" autocomplete="off" spellcheck="false"></label></div></fieldset>`;
        showPanel(`<header class="mg-step-head"><div><h2>${esc(tr('who_plays'))}</h2><p class="mg-muted">${esc(roundLabel(setup.mode, setup.holeId, courses, tr))}</p></div></header><fieldset class="mg-segment"><legend>${esc(tr('player_count'))}</legend>${counts}</fieldset><div class="mg-player-cards">${list.map(card).join('')}</div><div class="mg-panel-actions">${button('back', tr('back'))}${button('start', tr('got_it'), true)}</div>`, 'who_plays', focus);
    }

    function audioSettings(settings, track) {
        const slider = (hook, label, value) => `<label class="mg-field"><span>${esc(tr(label))}</span><input ${hook} type="range" min="0" max="100" value="${Math.round(value * 100)}"></label>`;
        return `${slider('data-music', 'music', settings.music)}<div class="mg-track"><span data-track>${esc(track?.title || '♪')}</span>${button('next-track', tr('next_track'))}${button('mute', tr(settings.music === 0 ? 'unmute' : 'mute'))}</div>${slider('data-effects', 'effects', settings.effects)}`;
    }
    function settings(values, track, focus) {
        const check = (hook, label, on) => `<label class="mg-check"><input ${hook} type="checkbox"${on ? ' checked' : ''}><span>${esc(tr(label))}</span></label>`;
        const quality = `<label class="mg-field"><span>${esc(tr('quality'))}</span><select data-quality>${['auto', 'low', 'medium', 'high'].map(q => `<option value="${q}"${values.quality === q ? ' selected' : ''}>${esc(tr(q))}</option>`).join('')}</select></label>`;
        showPanel(`<h2>${esc(tr('settings'))}</h2>${section('mg-settings-group', 'sound', tr('group_sound'), audioSettings(values, track))}${section('mg-settings-group', 'camera', tr('group_graphics'), quality)}${section('mg-settings-group', 'ball', tr('group_controls'), check('data-magnet', 'magnet', values.magnet !== false) + check('data-aim-camera', 'aim_camera', values.aimCamera) + check('data-follow', 'follow_ball', values.follow !== false))}${section('mg-settings-group', 'display', tr('group_display'), check('data-motion', 'motion', values.reducedMotion))}<div class="mg-panel-actions">${button('credits', tr('credits'))}${button('close', tr('close'), true)}</div>`, 'settings', focus);
    }
    function help({ inRound } = {}) {
        const row = (keys, text) => `<tr><th scope="row">${keys.map(key => `<kbd>${esc(key)}</kbd>`).join('')}</th><td>${esc(text)}</td></tr>`;
        const keys = `<table class="mg-keys"><tbody>${row(['←', '→'], tr('key_aim'))}${row(['↑', '↓'], tr('key_power'))}${row([tr('kbd_shift')], tr('key_fine'))}${row([tr('kbd_space')], tr('shoot'))}${row([tr('kbd_escape')], tr('key_escape'))}${row(['?'], tr('key_help'))}</tbody></table>`;
        const text = body => `<p>${esc(body)}</p>`;
        showPanel(`<h2>${esc(tr('help'))}</h2>${section('mg-help-section', 'ball', tr('help_aim_title'), text(tr('help_aim_body')))}${section('mg-help-section', 'keyboard', tr('help_keys_title'), keys)}${section('mg-help-section', 'touch', tr('help_touch_title'), text(tr('help_touch_body')))}${section('mg-help-section', 'trophy', tr('help_score_title'), text(tr('score_hint')))}<div class="mg-panel-actions">${inRound ? button('tutorial-again', tr('tutorial_again')) : ''}${button('close', tr('close'), true)}</div>`, 'help');
    }

    return {
        menu(state, focus) { (state.step === 'holes' ? holes : state.step === 'players' ? players : home)(state, focus); },
        audioSettings, settings, help,
        pause() { showPanel(`<h2>${esc(tr('pause'))}</h2><p>${esc(tr('paused'))}</p><p class="mg-muted">${esc(tr('leave_hint'))}</p><div class="mg-panel-actions">${button('close', tr('continue'), true)}${button('help', tr('help_short'))}${button('menu', tr('menu'))}</div>`, 'pause'); },
        credits(tracks) { showPanel(`<h2>${esc(tr('credits'))}</h2><p>${esc(tr('credits_body'))}</p><ul class="mg-credits">${tracks.map(item => `<li>${esc(item.title)} · ${esc(item.author)} · CC0</li>`).join('')}</ul><div class="mg-panel-actions">${button('close', tr('close'), true)}</div>`, 'credits'); },
        error(key) { showPanel(`<h2>${esc(tr(key))}</h2><div class="mg-panel-actions">${button('retry', tr('retry'), true)}</div>`, key); },
        // Moves colours between player cards without re-rendering, so typing and focus survive.
        syncPlayerColors(list) {
            panel.querySelectorAll('[data-player-index]').forEach(card => {
                const index = Number(card.dataset.playerIndex), blocked = new Set(list.filter((_, other) => other !== index).map(entry => entry.color));
                card.style.setProperty('--mg-player', colorValue(list[index]?.color));
                card.querySelectorAll('[data-player-color]').forEach(radio => { radio.checked = radio.value === list[index]?.color; radio.disabled = blocked.has(radio.value); });
            });
        }
    };
}
