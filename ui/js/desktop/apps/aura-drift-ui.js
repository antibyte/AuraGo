/* Aura Drift - menus, settings, pause, results and touch controls. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;

    function el(tag, className, text) {
        const node = document.createElement(tag);
        if (className) node.className = className;
        if (text !== undefined && text !== null) node.textContent = String(text);
        return node;
    }

    function addOption(select, value, label, selected) {
        const option = el('option', null, label);
        option.value = value;
        if (selected) option.selected = true;
        select.appendChild(option);
    }

    function button(label, className, onClick) {
        const b = el('button', className, label);
        b.type = 'button';
        b.addEventListener('click', onClick);
        return b;
    }

    AD.createUI = function (root, labels, callbacks, settings) {
        const overlay = el('div', 'agdrift-overlay');
        const menu = el('div', 'agdrift-panel agdrift-menu');
        const pause = el('div', 'agdrift-panel agdrift-pause');
        const results = el('div', 'agdrift-panel agdrift-results');
        const touch = el('div', 'agdrift-touch');
        /* Pointer-only affordances: keyboard users drive with arrow keys and get
           the translated control hint in the menu instead. */
        touch.setAttribute('aria-hidden', 'true');
        const srHint = el('p', 'agdrift-sr-only', labels.controlsHint);
        menu.appendChild(srHint);
        const touchState = { left: 0, right: 0, throttle: 0, brake: 0, fire: 0, handbrake: 0, boost: 0 };

        menu.appendChild(el('div', 'agdrift-kicker', 'AURA DRIFT'));
        menu.appendChild(el('h1', 'agdrift-title', labels.title));
        menu.appendChild(el('p', 'agdrift-subtitle', labels.subtitle));

        const modes = el('div', 'agdrift-modes');
        const modeIds = ['quick', 'time_trial', 'cup', 'daily'];
        const modeLabels = [labels.quickRace, labels.timeTrial, labels.cup, labels.daily];
        let selectedMode = 'quick';
        const modeButtons = [];
        modeIds.forEach((id, i) => {
            const b = button(modeLabels[i], 'agdrift-mode', () => {
                selectedMode = id;
                modeButtons.forEach((item, j) => item.classList.toggle('is-active', j === i));
                trackRow.style.opacity = (id === 'cup' || id === 'daily') ? '0.5' : '1';
                trackSelect.disabled = (id === 'cup' || id === 'daily');
            });
            if (i === 0) b.classList.add('is-active');
            modeButtons.push(b); modes.appendChild(b);
        });
        menu.appendChild(modes);

        const trackRow = el('label', 'agdrift-row');
        trackRow.appendChild(el('span', null, labels.track));
        const trackSelect = el('select', 'agdrift-select');
        for (const track of AD.TRACKS) addOption(trackSelect, track.id, track.name, track.id === AD.TRACKS[0].id);
        trackRow.appendChild(trackSelect);
        menu.appendChild(trackRow);

        const lapsRow = el('label', 'agdrift-row');
        lapsRow.appendChild(el('span', null, labels.laps));
        const lapsSelect = el('select', 'agdrift-select');
        for (const n of [1, 2, 3, 4, 5]) addOption(lapsSelect, String(n), String(n), n === 2);
        lapsRow.appendChild(lapsSelect);
        menu.appendChild(lapsRow);
        trackSelect.addEventListener('change', () => {
            lapsSelect.value = String(AD.trackById(trackSelect.value).laps);
        });

        const settingsBox = el('div', 'agdrift-settings');
        const qualityRow = el('label', 'agdrift-row');
        qualityRow.appendChild(el('span', null, labels.quality));
        const qualitySelect = el('select', 'agdrift-select');
        [['auto', labels.auto], ['low', labels.low], ['medium', labels.medium], ['high', labels.high], ['ultra', labels.ultra]]
            .forEach(([value, label]) => addOption(qualitySelect, value, label, value === settings.quality));
        qualityRow.appendChild(qualitySelect);
        settingsBox.appendChild(qualityRow);

        const volumeRow = el('label', 'agdrift-row');
        volumeRow.appendChild(el('span', null, labels.sound));
        const volume = el('input', 'agdrift-range');
        volume.type = 'range'; volume.min = '0'; volume.max = '100';
        volume.value = String(Math.round(settings.volume * 100));
        volumeRow.appendChild(volume);
        settingsBox.appendChild(volumeRow);

        function checkbox(labelText, initial, onChange) {
            const row = el('label', 'agdrift-check');
            const input = el('input');
            input.type = 'checkbox'; input.checked = !!initial;
            input.addEventListener('change', () => onChange(input.checked));
            row.appendChild(input);
            row.appendChild(el('span', null, labelText));
            settingsBox.appendChild(row);
            return input;
        }
        checkbox(labels.muted, settings.muted, value => { settings.muted = value; callbacks.onSettings(settings); });
        checkbox(labels.weaponsOff, !settings.weapons, value => { settings.weapons = !value; callbacks.onSettings(settings); });
        checkbox(labels.reducedMotion, settings.reducedMotion, value => { settings.reducedMotion = value; callbacks.onSettings(settings); });
        checkbox(labels.showGhost, settings.showGhost, value => { settings.showGhost = value; callbacks.onSettings(settings); });
        menu.appendChild(settingsBox);

        qualitySelect.addEventListener('change', () => { settings.quality = qualitySelect.value; callbacks.onSettings(settings); });
        volume.addEventListener('input', () => { settings.volume = Number(volume.value) / 100; callbacks.onSettings(settings); });

        menu.appendChild(el('p', 'agdrift-hint', labels.controlsHint));
        menu.appendChild(el('p', 'agdrift-hint agdrift-disclosure', labels.catchup));
        const bestLine = el('p', 'agdrift-best', '');
        menu.appendChild(bestLine);
        menu.appendChild(button(labels.start, 'agdrift-primary', () => {
            callbacks.onStart({
                mode: selectedMode,
                trackId: trackSelect.value,
                laps: Number(lapsSelect.value)
            });
        }));

        pause.appendChild(el('h2', null, labels.pause));
        pause.appendChild(button(labels.resume, 'agdrift-primary', () => callbacks.onResume()));
        pause.appendChild(button(labels.restart, 'agdrift-secondary', () => callbacks.onRestart()));
        pause.appendChild(button(labels.menu, 'agdrift-secondary', () => callbacks.onQuit()));

        results.appendChild(el('div', 'agdrift-kicker', 'AURA DRIFT'));
        results.appendChild(el('h2', null, labels.results));
        const resultBody = el('div', 'agdrift-result-body');
        results.appendChild(resultBody);
        const resultButtons = el('div', 'agdrift-result-buttons');
        results.appendChild(resultButtons);

        function touchButton(label, key, className) {
            const b = button(label, 'agdrift-touch-btn ' + className, () => {});
            const set = value => { touchState[key] = value; b.classList.toggle('is-down', !!value); };
            b.addEventListener('pointerdown', e => { e.preventDefault(); set(1); b.setPointerCapture(e.pointerId); });
            b.addEventListener('pointerup', e => { e.preventDefault(); set(0); });
            b.addEventListener('pointercancel', () => set(0));
            b.addEventListener('lostpointercapture', () => set(0));
            return b;
        }
        const leftPad = el('div', 'agdrift-touch-pad');
        leftPad.appendChild(touchButton('◀', 'left', 'agdrift-left'));
        leftPad.appendChild(touchButton('▶', 'right', 'agdrift-right'));
        const rightPad = el('div', 'agdrift-touch-pad');
        rightPad.appendChild(touchButton('▲', 'throttle', 'agdrift-gas'));
        rightPad.appendChild(touchButton('▼', 'brake', 'agdrift-brake'));
        rightPad.appendChild(touchButton('✦', 'fire', 'agdrift-fire'));
        rightPad.appendChild(touchButton('↺', 'handbrake', 'agdrift-hand'));
        touch.appendChild(leftPad); touch.appendChild(rightPad);

        overlay.appendChild(menu);
        overlay.appendChild(pause);
        overlay.appendChild(results);
        root.appendChild(overlay);
        root.appendChild(touch);

        function show(node) {
            menu.hidden = node !== menu;
            pause.hidden = node !== pause;
            results.hidden = node !== results;
            overlay.hidden = false;
            touch.hidden = !!node;
        }

        return {
            touch: touchState,
            showMenu(bests) {
                const keys = Object.keys(bests || {});
                bestLine.textContent = keys.length ? (labels.bestLap + ' · ' + keys.length + ' × ' + labels.record) : '';
                show(menu);
            },
            showPause() { show(pause); },
            showResults(result, info) {
                resultBody.replaceChildren();
                resultButtons.replaceChildren();
                const rows = [
                    [labels.finish, AD.formatTime(result.time)],
                    [labels.bestLap, isFinite(result.bestLap) ? AD.formatTime(result.bestLap) : '--:--.---'],
                    [labels.position, result.position + '/' + info.field],
                    [labels.track, info.track]
                ];
                for (const [label, value] of rows) {
                    const row = el('div', 'agdrift-result-row');
                    row.appendChild(el('span', null, label));
                    row.appendChild(el('strong', null, value));
                    resultBody.appendChild(row);
                }
                if (info.record) resultBody.appendChild(el('p', 'agdrift-record', labels.newRecord));
                resultButtons.appendChild(button(labels.restart, 'agdrift-primary', () => callbacks.onRestart()));
                if (info.hasNext) resultButtons.appendChild(button(labels.nextTrack, 'agdrift-secondary', () => callbacks.onNext()));
                resultButtons.appendChild(button(labels.menu, 'agdrift-secondary', () => callbacks.onQuit()));
                show(results);
            },
            hide() { overlay.hidden = true; touch.hidden = false; },
            setBests() { /* best line is rebuilt on showMenu */ },
            destroy() { overlay.remove(); touch.remove(); }
        };
    };
})();
