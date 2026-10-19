import { colorValue, formatToPar, powerHex } from './minigolf-shared.js';
import { createMinimap } from './minigolf-minimap.js';

const formatters = new Map();
function numberFormat(lang, options) {
    const key = (lang || '') + JSON.stringify(options);
    if (!formatters.has(key)) {
        let format;
        try { format = new Intl.NumberFormat(lang || undefined, options); } catch (_) { format = new Intl.NumberFormat('en', options); }
        formatters.set(key, format);
    }
    return formatters.get(key);
}

// "54 %" and "≈ 11 m" in the page language; distance parts joined by no-break spaces.
export function formatPower(strength, meters, lang) {
    const percent = numberFormat(lang, { style: 'percent', maximumFractionDigits: 0 }).format(Math.round(Math.max(0, Math.min(1, strength)) * 100) / 100);
    let distance = '';
    if (Number.isFinite(meters) && meters >= 0) {
        const rounded = meters < 1 ? Math.round(meters * 10) / 10 : meters < 10 ? Math.round(meters * 2) / 2 : Math.round(meters);
        distance = '≈\xa0' + numberFormat(lang, { maximumFractionDigits: 1 }).format(rounded) + '\xa0m';
    }
    return { percent, distance, text: distance ? percent + ' · ' + distance : percent };
}

// Top-left of the meter: 56 px right and 64 px above the ball, mirrored at the
// stage edges and clamped; bottom centre when the ball is not visible.
export function placeMeter(ball, width, height, size) {
    const half = size / 2;
    if (!ball?.visible) return { x: Math.round(width / 2 - half), y: Math.round(height - size - 12) };
    let cx = ball.x + 56, cy = ball.y - 64;
    if (cx + half > width) cx = ball.x - 56;
    if (cy - half < 0) cy = ball.y + 64;
    cx = Math.max(half, Math.min(width - half, cx)); cy = Math.max(half, Math.min(height - half, cy));
    return { x: Math.round(cx - half), y: Math.round(cy - half) };
}

// key: [kind, seconds] (spec 4.5)
export const TOASTS = Object.freeze({ sunk: ['success', 3], new_best: ['success', 6], water: ['warning', 3.5], out: ['warning', 3.5], limit: ['warning', 5], quality_changed: ['info', 4], audio_error: ['warning', 6], storage_error: ['warning', 8], progress_reset: ['info', 8] });
const MAX_TOASTS = 3;
const MAX_TICK_MS = 120;

// Heads-up display over the 3D stage. Every countdown follows the frame clock that
// the game feeds to tick(), so banners and toasts pause with the window.
export function createHud(elements, { tr, esc, icon }) {
    const { hud, toasts, announcer } = elements;
    hud.innerHTML = `<div class="mg-plaque" data-plaque><span class="mg-plaque-world" data-plaque-world></span><strong class="mg-plaque-hole" data-plaque-hole></strong><span class="mg-plaque-name" data-plaque-name></span></div>
        <ol class="mg-chips" data-chips aria-label="${esc(tr('players'))}"></ol>
        <div class="mg-map" data-map></div>
        <div class="mg-banner" data-banner hidden><strong data-banner-title></strong><span data-banner-lines></span></div>
        <div class="mg-meter" data-meter hidden aria-hidden="true"><svg viewBox="0 0 96 96"><circle class="mg-meter-track" cx="48" cy="48" r="40"/><circle class="mg-meter-arc" data-meter-arc cx="48" cy="48" r="40" pathLength="100" stroke-dasharray="0 100" transform="rotate(-90 48 48)"/></svg><strong class="mg-meter-percent" data-meter-percent></strong><span class="mg-meter-distance" data-meter-distance></span></div>
        <p class="mg-hint" data-hint hidden></p>`;
    const $ = selector => hud.querySelector(selector);
    const plaque = { world: $('[data-plaque-world]'), hole: $('[data-plaque-hole]'), name: $('[data-plaque-name]') };
    const mapElement = $('[data-map]');
    mapElement.innerHTML = `<button class="mg-map-toggle" data-action="map" aria-pressed="true" aria-label="${esc(tr('map'))}" title="${esc(tr('map'))}">${icon('map')}</button><div class="mg-map-frame" role="img" aria-label="${esc(tr('hole_map'))}"><canvas class="mg-map-lane"></canvas><canvas class="mg-map-live"></canvas></div>`;
    const map = createMinimap(mapElement);
    const chips = $('[data-chips]'), banner = $('[data-banner]'), bannerTitle = $('[data-banner-title]'), bannerLines = $('[data-banner-lines]');
    const meter = $('[data-meter]'), arc = $('[data-meter-arc]'), percent = $('[data-meter-percent]'), distance = $('[data-meter-distance]'), hint = $('[data-hint]');
    let clock = 0, bannerLeft = 0, bannerCount = 0, bannerText = null, hintText = '', meterAt = '';
    const live = []; // { key, el, left }

    function announce(text) { announcer.textContent = text; }
    function dismiss(entry) { entry.el.remove(); const index = live.indexOf(entry); if (index >= 0) live.splice(index, 1); }
    function onToastClick(event) { const entry = live.find(item => item.el === event.target.closest('.mg-toast')); if (entry) dismiss(entry); }
    toasts.addEventListener('click', onToastClick);

    function chip(round, course, player, index) {
        const single = round.players.length === 1, active = index === round.turnIndex && !player.done;
        const score = player.done ? icon('check') + esc(formatToPar(player.strokes - course.par, tr('par'))) : String(player.strokes);
        return `<li class="mg-chip" data-player="${index}"${active ? ' data-active' : ''}${player.done ? ` data-done title="${esc(tr('chip_done'))}"` : ''} style="--mg-player:${colorValue(player.color)}"><i class="mg-dot"></i><span class="mg-chip-name">${esc(single ? tr('strokes') : player.name)}</span><b class="mg-chip-score">${score}</b>${active && !single ? `<span class="mg-chip-turn">${esc(tr('chip_active'))}</span>` : ''}</li>`;
    }

    return {
        map,
        update(round, course) {
            plaque.world.textContent = tr(course.world);
            plaque.hole.textContent = tr('hole_par').replace('{hole}', course.number).replace('{par}', course.par);
            plaque.name.textContent = tr('course_' + course.number);
            chips.innerHTML = round.players.map((player, index) => chip(round, course, player, index)).join('');
        },
        // formatted: result of formatPower(); the arc follows the 3D ring colours.
        power(strength, formatted) {
            percent.textContent = formatted.percent;
            distance.textContent = formatted.distance;
            distance.hidden = !formatted.distance;
            arc.setAttribute('stroke-dasharray', Math.round(strength * 1000) / 10 + ' 100');
            arc.setAttribute('stroke', powerHex(strength));
        },
        // position: result of scene.screenPosition(), or null to hide.
        meter(position) {
            if (!position) { if (!meter.hidden) meter.hidden = true; return; }
            if (meter.hidden) meter.hidden = false;
            const at = placeMeter(position, hud.clientWidth, hud.clientHeight, meter.offsetWidth || 96);
            const next = `translate(${at.x}px, ${at.y}px)`;
            if (next !== meterAt) { meterAt = next; meter.style.transform = next; }
        },
        banner(title, lines, color, seconds) {
            const extra = (lines || []).filter(Boolean);
            banner.style.setProperty('--mg-player', color);
            bannerTitle.textContent = title;
            bannerLines.innerHTML = extra.map(text => `<span>${esc(text)}</span>`).join('');
            banner.hidden = false;
            banner.classList.remove('mg-banner-in');
            void banner.offsetWidth;
            banner.classList.add('mg-banner-in');
            bannerCount++; bannerText = [title, ...extra].join(' · '); bannerLeft = seconds * 1000;
            announce(bannerText);
        },
        toast(key) {
            const [kind, seconds] = TOASTS[key] || ['info', 4], text = tr(key);
            const known = live.find(item => item.key === key);
            if (known) dismiss(known);
            const el = document.createElement('div');
            el.className = 'mg-toast'; el.dataset.toast = key; el.dataset.kind = kind; el.textContent = text;
            toasts.append(el);
            live.push({ key, el, left: seconds * 1000 });
            while (live.length > MAX_TOASTS) dismiss(live[0]);
            announce(text);
        },
        hint(text) {
            if (text === hintText) return;
            hintText = text; hint.textContent = text; hint.hidden = !text;
        },
        announce,
        // now: the requestAnimationFrame timestamp of frame().
        tick(now) {
            const dt = clock ? Math.max(0, Math.min(MAX_TICK_MS, now - clock)) : 0;
            clock = now;
            if (!banner.hidden && (bannerLeft -= dt) <= 0) { banner.hidden = true; banner.classList.remove('mg-banner-in'); }
            for (const entry of live.slice()) if ((entry.left -= dt) <= 0) dismiss(entry);
        },
        inspect() { return { meter: !meter.hidden, bannerCount, banner: bannerText, toasts: live.map(item => item.key), map: map.inspect() }; },
        dispose() {
            map.dispose();
            toasts.removeEventListener('click', onToastClick);
            live.length = 0; toasts.replaceChildren(); hud.replaceChildren();
        }
    };
}
