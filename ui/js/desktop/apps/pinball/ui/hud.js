// AuraPinball HUD: DMD dock, plunger meter, ball-save chip, initials hint, live region. DOM only.
function el(tag, text, className) { const node = document.createElement(tag); if (text !== undefined) node.textContent = text; if (className) node.className = className; return node; }

export function createHud(tr) {
    const root = el('div', undefined, 'pb-hud');
    const dmdHost = el('div', undefined, 'pb-dmd');
    dmdHost.setAttribute('aria-hidden', 'true');
    const plunger = el('div', undefined, 'pb-plunger');
    plunger.setAttribute('role', 'progressbar');
    plunger.setAttribute('aria-label', tr('plunger'));
    plunger.setAttribute('aria-valuemin', '0'); plunger.setAttribute('aria-valuemax', '100'); plunger.setAttribute('aria-valuenow', '0');
    const fill = el('span', undefined, 'pb-plunger-fill');
    plunger.append(fill); plunger.hidden = true;
    const chip = el('div', undefined, 'pb-ballsave');
    const chipLabel = el('span', tr('ball_saver')), chipTime = el('strong');
    chip.append(chipLabel, chipTime); chip.hidden = true;
    const hint = el('div', tr('initials_hint'), 'pb-initials-hint');
    hint.hidden = true;
    const live = el('div', undefined, 'pb-sr-only pb-live');
    live.setAttribute('role', 'status'); live.setAttribute('aria-live', 'polite'); live.setAttribute('aria-atomic', 'true');
    root.append(dmdHost, plunger, chip, hint, live);
    let lastChip = -1, lastPlunger = -1, plungerShown = false;
    return {
        root, dmdHost, live,
        setPlunger(value, visible) {
            const show = !!visible, v = Math.max(0, Math.min(1, Number.isFinite(value) ? value : 0));
            if (show !== plungerShown) { plungerShown = show; plunger.hidden = !show; }
            const pct = Math.round(v * 100);
            if (pct !== lastPlunger) { lastPlunger = pct; fill.style.transform = 'scaleY(' + v.toFixed(3) + ')'; plunger.setAttribute('aria-valuenow', String(pct)); plunger.dataset.full = String(v >= .98); }
        },
        setBallSave(seconds) {
            const s = Number.isFinite(seconds) && seconds > 0 ? Math.ceil(seconds) : 0;
            if (s === lastChip) return;
            lastChip = s; chip.hidden = s === 0; chipTime.textContent = s ? String(s) : '';
        },
        setInitialsHint(on) { hint.hidden = !on; },
        announce(text) { if (live.textContent !== text) live.textContent = text; },
        show(visible) { root.hidden = !visible; }
    };
}
