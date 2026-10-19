const FINISHED = Object.freeze({ step: null, finished: true });
const AIMING = ['aim', 'power', 'release'];
const KEYS = { aim: 'tutorial_aim_keys', power: 'tutorial_power_keys', release: 'tutorial_release_keys' };
const to = step => ({ step, finished: false });
export const tutorialStart = () => to('aim');

// Events: drag {strength}, turn, strength {strength}, cancel, shot, resolved, panel, confirm, skip.
// An unchanged state comes back as the same object, so callers compare by identity.
export function tutorialNext(state, event) {
    const step = state?.step;
    if (!step || !event) return state;
    switch (event.type) {
        case 'skip': return FINISHED;
        case 'drag': return step === 'aim' ? to('power') : step === 'power' && event.strength >= 0.2 ? to('release') : state;
        case 'turn': return step === 'aim' ? to('power') : state;
        case 'strength': return step === 'power' && event.strength >= 0.2 ? to('release') : state;
        case 'shot': return step === 'done' ? FINISHED : AIMING.includes(step) ? to('watch') : state;
        case 'resolved': return step === 'watch' ? to('done') : state;
        case 'panel': case 'confirm': return step === 'done' ? FINISHED : state;
        default: return state;
    }
}

// What the coach shows: text and key hint (translation keys) plus where the bubble sits, or null.
export function tutorialView(state, { panel = false, camera = false, canAim = false, ballVisible = true } = {}) {
    const step = state?.step;
    if (!step || panel) return null;
    if (step === 'watch') return { step, text: 'tutorial_watch', keys: null, anchor: 'top' };
    if (step === 'done') return { step, text: 'tutorial_done', keys: null, anchor: 'help' };
    if (!canAim) return null;
    const anchor = ballVisible ? 'ball' : 'top';
    return camera ? { step, text: 'tutorial_camera', keys: null, anchor } : { step, text: 'tutorial_' + step, keys: KEYS[step], anchor };
}

const MARGIN = 8, BALL_GAP = 36, METER_GAP = 120;

// One non-modal bubble over the stage. The DOM changes only when the step, text or anchor changes;
// the position follows the ball on every call. A bubble button that held the focus hands it back.
export function createCoach(container, { tr, esc, onFocusLost }) {
    container.setAttribute('role', 'group');
    container.setAttribute('aria-label', tr('tutorial'));
    container.innerHTML = `<div class="mg-coach-bubble" data-coach-bubble data-anchor="ball">
        <span class="mg-coach-demo" data-coach-demo aria-hidden="true"><i></i></span>
        <p class="mg-coach-text" data-coach-text role="status" aria-live="polite"></p>
        <p class="mg-coach-keys" data-coach-keys></p>
        <div class="mg-coach-actions"><button class="mg-primary" data-action="tutorial-done" hidden>${esc(tr('got_it'))}</button><button data-action="tutorial-skip">${esc(tr('tutorial_skip'))}</button></div>
    </div>`;
    const $ = selector => container.querySelector(selector);
    const bubble = $('[data-coach-bubble]'), text = $('[data-coach-text]'), keys = $('[data-coach-keys]'), demo = $('[data-coach-demo]'), done = $('[data-action="tutorial-done"]');
    let shown = '', placed = '';

    function place(view, ball) {
        const width = container.clientWidth, height = container.clientHeight, w = bubble.offsetWidth, h = bubble.offsetHeight;
        const clampX = x => Math.max(MARGIN, Math.min(width - w - MARGIN, x)), clampY = y => Math.max(MARGIN, Math.min(height - h - MARGIN, y));
        let x, y, side = 'none', target = width / 2;
        if (view.anchor === 'ball') {
            // Aiming steps keep clear of the power meter that opens up to the upper right of the ball.
            const lift = view.step === 'aim' ? BALL_GAP : METER_GAP;
            side = ball.y - h - lift >= MARGIN ? 'above' : 'below';
            x = clampX(ball.x - w / 2); y = clampY(side === 'above' ? ball.y - h - lift : ball.y + lift); target = ball.x;
        } else if (view.anchor === 'help') {
            const stage = container.getBoundingClientRect(), help = container.closest('.mg-app')?.querySelector('.mg-topbar [data-action="help"]')?.getBoundingClientRect();
            side = 'below'; target = help ? help.left + help.width / 2 - stage.left : width - 40;
            x = clampX(target - w / 2); y = help ? Math.max(MARGIN, help.bottom - stage.top + 10) : MARGIN;
        } else {
            // Top centre; on narrow stages the top holds the HUD stack, so the bubble sits at the bottom.
            x = clampX((width - w) / 2); y = width <= 560 ? clampY(height - h - 14) : 14;
        }
        const next = `${side}|${Math.round(x)}|${Math.round(y)}|${Math.round(target - x)}`;
        if (next === placed) return;
        placed = next;
        bubble.dataset.side = side;
        bubble.style.setProperty('--mg-arrow', Math.max(16, Math.min(w - 16, Math.round(target - x))) + 'px');
        bubble.style.transform = `translate(${Math.round(x)}px, ${Math.round(y)}px)`;
    }

    return {
        // view: result of tutorialView() or null; ball: scene.screenPosition().
        render(view, ball) {
            const hadFocus = container.contains(document.activeElement);
            if (!view) {
                if (!container.hidden) container.hidden = true;
                shown = ''; placed = '';
            } else {
                const signature = view.step + '|' + view.text + '|' + view.anchor;
                if (signature !== shown) {
                    shown = signature; placed = '';
                    bubble.dataset.anchor = view.anchor;
                    text.textContent = tr(view.text);
                    keys.textContent = view.keys ? tr(view.keys) : ''; keys.hidden = !view.keys;
                    demo.hidden = view.step !== 'aim';
                    done.hidden = view.step !== 'done';
                }
                if (container.hidden) container.hidden = false;
                place(view, ball || { visible: false });
            }
            const active = document.activeElement;
            if (hadFocus && (!container.contains(active) || active.closest('[hidden]'))) onFocusLost?.();
        },
        dispose() { container.replaceChildren(); container.hidden = true; }
    };
}
