/* Aura Drift - menu shell: garage rail with tabs (mode, car, track, settings, controls),
   loading view, pause, results and touch controls. The cards live in aura-drift-ui-cards.js.
   Every visible string goes through the translator; a missing key falls back to a readable
   word instead of the raw key. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;

    function el(tag, className, text) {
        const node = document.createElement(tag);
        if (className) node.className = className;
        if (text !== undefined && text !== null) node.textContent = String(text);
        return node;
    }

    function button(label, className, onClick) {
        const b = el('button', className, label);
        b.type = 'button';
        if (onClick) b.addEventListener('click', onClick);
        return b;
    }

    /* "aura_drift.settings.tab_mode" -> "Mode": only used when a key has no translation yet. */
    function humanize(key) {
        const parts = String(key).split('.');
        let last = parts.pop();
        if (/^(name|desc|tag|quirk|blurb|hint)$/.test(last) && parts.length) last = parts.pop();
        last = last.replace(/^tab_/, '').replace(/_/g, ' ');
        return last.charAt(0).toUpperCase() + last.slice(1);
    }

    function tr(t) {
        return function (key, fallback) {
            const value = t(key);
            if (typeof value === 'string' && value && value !== key) return value;
            return fallback === undefined ? humanize(key) : fallback;
        };
    }

    AD.uiKit = { el: el, button: button, humanize: humanize, tr: tr };

    const TABS = ['mode', 'car', 'track', 'settings', 'controls'];
    const CAMERAS = ['chase', 'high', 'drone', 'bumper'];
    /* Key caps are universal; "+" and "/" are separators between caps. */
    const CONTROLS = [
        ['steer', '← → / A D'], ['throttle', '↑ / W'], ['brake_reverse', '↓ / S'],
        ['handbrake', 'Shift'], ['drift_turbo', 'Shift'], ['nitro', 'E / Ctrl'], ['fire', 'Space'],
        ['air_roll', 'A D'], ['air_flip', 'W S'], ['air_spin', 'Shift + A D'],
        ['camera', 'C'], ['horn', 'H'], ['rescue', 'T'], ['pause', 'P / Esc'], ['restart', 'R']
    ];

    AD.createUI = function (root, labels, callbacks, settings, t) {
        callbacks = callbacks || {};
        const L = tr(typeof t === 'function' ? t : (key => key));
        const C = AD.uiCards || {
            modeCard: (m, i, ctx) => button(ctx.modeName(m.id), 'agdrift-card agdrift-mode-card'),
            carCard: car => button(car.name, 'agdrift-card agdrift-car-card'),
            trackCard: track => button(track.name, 'agdrift-card agdrift-track-card'),
            liverySwatch: (car, i) => button(String(i + 1), 'agdrift-swatch'),
            roving: () => () => {}
        };
        const sound = kind => { if (typeof callbacks.onUi === 'function') callbacks.onUi(kind); };
        const modeNames = { quick: labels.quickRace, time_trial: labels.timeTrial, cup: labels.cup, daily: labels.daily };
        const modeName = id => modeNames[id] || L('aura_drift.mode.' + id + '.name');
        const ctx = { L: L, modeName: modeName };
        const car0 = AD.carById(settings.car);
        const state = {
            tab: 'mode', mode: 'quick', car: car0.id, livery: Math.max(0, Math.min(3, settings.livery | 0)),
            track: AD.TRACKS[0].id, laps: AD.TRACKS[0].laps, rivals: 5
        };
        let bests = {};
        const syncs = [];
        const touchState = { left: 0, right: 0, throttle: 0, brake: 0, fire: 0, handbrake: 0, boost: 0 };

        const overlay = el('div', 'agdrift-overlay');
        const menu = el('div', 'agdrift-panel agdrift-menu');
        const pause = el('div', 'agdrift-panel agdrift-pause');
        const results = el('div', 'agdrift-panel agdrift-results');
        const loading = el('div', 'agdrift-panel agdrift-loading');
        const touch = el('div', 'agdrift-touch');
        /* Pointer-only affordances: keyboard users drive with the keys listed under Controls. */
        touch.setAttribute('aria-hidden', 'true');

        /* ---- menu rail ------------------------------------------------- */
        const rail = el('div', 'agdrift-rail');
        rail.appendChild(el('p', 'agdrift-sr-only', labels.controlsHint));
        const head = el('header', 'agdrift-head');
        const words = String(labels.title || 'Aura Drift').split(' ');
        const title = el('h1', 'agdrift-title');
        title.appendChild(el('span', 'agdrift-title-a', words.shift()));
        if (words.length) title.appendChild(el('span', 'agdrift-title-b', words.join(' ')));
        head.appendChild(title);
        head.appendChild(el('p', 'agdrift-subtitle', labels.subtitle));
        rail.appendChild(head);

        const tabsEl = el('div', 'agdrift-tabs');
        tabsEl.setAttribute('role', 'tablist');
        const tabButtons = TABS.map(id => {
            const b = button(L('aura_drift.settings.tab_' + id), 'agdrift-tab', () => showTab(id));
            b.setAttribute('role', 'tab');
            b.title = b.textContent;
            b.dataset.tab = id;
            tabsEl.appendChild(b);
            return b;
        });
        syncs.push(C.roving(tabsEl, '.agdrift-tab', true));
        rail.appendChild(tabsEl);

        const pagesEl = el('div', 'agdrift-pages');
        const pages = {};
        TABS.forEach(id => {
            const page = el('section', 'agdrift-page');
            page.setAttribute('role', 'tabpanel');
            page.dataset.page = id;
            page.setAttribute('aria-label', L('aura_drift.settings.tab_' + id));
            pages[id] = page;
            pagesEl.appendChild(page);
        });
        rail.appendChild(pagesEl);

        function group(className, label) {
            const g = el('div', className);
            g.setAttribute('role', 'group');
            g.setAttribute('aria-label', label);
            return g;
        }
        function row(labelText, control, className) {
            const r = el('label', 'agdrift-row' + (className ? ' ' + className : ''));
            r.appendChild(el('span', 'agdrift-row-label', labelText));
            r.appendChild(control);
            return r;
        }

        /* mode page */
        const modeGrid = group('agdrift-grid agdrift-modes', L('aura_drift.settings.tab_mode'));
        const modeCards = AD.MODES.map((m, i) => {
            const card = C.modeCard(m, i, ctx);
            card.addEventListener('click', () => { state.mode = m.id; sound('move'); refresh(); });
            modeGrid.appendChild(card);
            return card;
        });
        syncs.push(C.roving(modeGrid, '.agdrift-mode-card', true));
        pages.mode.appendChild(modeGrid);
        const rivalsRow = el('div', 'agdrift-row agdrift-rivals');
        rivalsRow.appendChild(el('span', 'agdrift-row-label', L('aura_drift.rivals')));
        const rivalsGroup = group('agdrift-seg', L('aura_drift.rivals'));
        const rivalButtons = [1, 2, 3, 4, 5].map(n => {
            const b = button(String(n), 'agdrift-seg-btn', () => { state.rivals = n; sound('move'); refresh(); });
            rivalsGroup.appendChild(b);
            return b;
        });
        syncs.push(C.roving(rivalsGroup, '.agdrift-seg-btn', true));
        rivalsRow.appendChild(rivalsGroup);
        pages.mode.appendChild(rivalsRow);

        /* car page */
        const carGrid = group('agdrift-grid agdrift-cars', L('aura_drift.settings.tab_car'));
        const carCards = AD.CARS.map(car => {
            const card = C.carCard(car, ctx);
            card.addEventListener('click', () => { state.car = car.id; sound('move'); refresh(); showcase(); });
            carGrid.appendChild(card);
            return card;
        });
        syncs.push(C.roving(carGrid, '.agdrift-car-card', true));
        pages.car.appendChild(carGrid);
        const liveryRow = el('div', 'agdrift-row agdrift-liveries');
        liveryRow.appendChild(el('span', 'agdrift-row-label', L('aura_drift.settings.livery')));
        const liveryGroup = group('agdrift-seg agdrift-swatches', L('aura_drift.settings.livery'));
        const swatches = [0, 1, 2, 3].map(i => {
            const b = C.liverySwatch(car0, i, L('aura_drift.settings.livery') + ' ' + (i + 1));
            b.addEventListener('click', () => { state.livery = i; sound('move'); refresh(); showcase(); });
            liveryGroup.appendChild(b);
            return b;
        });
        syncs.push(C.roving(liveryGroup, '.agdrift-swatch', true));
        liveryRow.appendChild(liveryGroup);
        pages.car.appendChild(liveryRow);

        /* track page */
        const trackGrid = group('agdrift-grid agdrift-tracks', labels.track);
        const trackCards = AD.TRACKS.map(track => {
            const card = C.trackCard(track, ctx);
            card.addEventListener('click', () => {
                state.track = track.id;
                state.laps = track.laps;
                sound('move');
                refresh();
                showcase();
            });
            trackGrid.appendChild(card);
            return card;
        });
        syncs.push(C.roving(trackGrid, '.agdrift-track-card', true));
        pages.track.appendChild(trackGrid);
        const lapsSelect = el('select', 'agdrift-select agdrift-laps');
        for (const n of [1, 2, 3, 4, 5, 6]) {
            const o = el('option', null, String(n));
            o.value = String(n);
            lapsSelect.appendChild(o);
        }
        lapsSelect.addEventListener('change', () => { state.laps = Number(lapsSelect.value); });
        const lapsRow = row(labels.laps, lapsSelect, 'agdrift-laps-row');
        pages.track.appendChild(lapsRow);

        /* settings page */
        function addOption(select, value, text, selected) {
            const o = el('option', null, text);
            o.value = value;
            if (selected) o.selected = true;
            select.appendChild(o);
        }
        function changed() { if (typeof callbacks.onSettings === 'function') callbacks.onSettings(settings); }
        const qualitySelect = el('select', 'agdrift-select');
        [['auto', labels.auto], ['low', labels.low], ['medium', labels.medium], ['high', labels.high], ['ultra', labels.ultra]]
            .forEach(([value, text]) => addOption(qualitySelect, value, text, value === settings.quality));
        qualitySelect.addEventListener('change', () => { settings.quality = qualitySelect.value; changed(); });
        pages.settings.appendChild(row(labels.quality, qualitySelect));

        const cameraSelect = el('select', 'agdrift-select');
        CAMERAS.forEach(id => addOption(cameraSelect, id, L('aura_drift.settings.camera_' + id), id === settings.camera));
        cameraSelect.addEventListener('change', () => { settings.camera = cameraSelect.value; changed(); });
        pages.settings.appendChild(row(L('aura_drift.settings.camera'), cameraSelect));

        const volume = el('input', 'agdrift-range');
        volume.type = 'range'; volume.min = '0'; volume.max = '100';
        volume.value = String(Math.round(settings.volume * 100));
        const paintVolume = () => volume.style.setProperty('--p', volume.value + '%');
        paintVolume();
        volume.addEventListener('input', () => { paintVolume(); settings.volume = Number(volume.value) / 100; changed(); });
        pages.settings.appendChild(row(labels.sound, volume));

        /* The score has a trim of its own: the sound slider is the whole mix, this one is the
         * music alone. It starts where the score sits under the race and may go up to full. */
        const musicTrim = typeof settings.musicVolume === 'number'
            ? settings.musicVolume
            : (typeof AD.defaultSettings === 'function' ? AD.defaultSettings().musicVolume : 0.9);
        const music = el('input', 'agdrift-range');
        music.type = 'range'; music.min = '0'; music.max = '100';
        music.value = String(Math.round(musicTrim * 100));
        const paintMusic = () => music.style.setProperty('--p', music.value + '%');
        paintMusic();
        music.addEventListener('input', () => { paintMusic(); settings.musicVolume = Number(music.value) / 100; changed(); });
        pages.settings.appendChild(row(labels.music, music));

        function toggle(labelText, initial, onChange) {
            const r = el('label', 'agdrift-check');
            const input = el('input');
            input.type = 'checkbox'; input.checked = !!initial;
            input.addEventListener('change', () => onChange(input.checked));
            r.appendChild(input);
            const sw = el('span', 'agdrift-switch');
            sw.setAttribute('aria-hidden', 'true');
            r.appendChild(sw);
            r.appendChild(el('span', 'agdrift-row-label', labelText));
            pages.settings.appendChild(r);
        }
        toggle(labels.muted, settings.muted, v => { settings.muted = v; changed(); });
        toggle(labels.weaponsOff, !settings.weapons, v => { settings.weapons = !v; changed(); });
        toggle(labels.reducedMotion, settings.reducedMotion, v => { settings.reducedMotion = v; changed(); });
        toggle(labels.showGhost, settings.showGhost, v => { settings.showGhost = v; changed(); });
        /* The voice commentary (Underdog Woof) speaks the banter lines. */
        toggle(labels.voice, settings.voice !== false, v => { settings.voice = v; changed(); });
        pages.settings.appendChild(el('p', 'agdrift-note', L('aura_drift.voice_note')));
        /* The licence of the model asks for the credit with a link to its home. */
        const voiceCredit = el('p', 'agdrift-note');
        const creditLink = el('a', null, L('aura_drift.voice_credit'));
        const creditUrl = L('aura_drift.voice_credit_url');
        if (creditUrl && creditUrl.indexOf('underdog.ai') >= 0) {
            creditLink.href = creditUrl;
            creditLink.target = '_blank';
            creditLink.rel = 'noopener noreferrer';
        }
        voiceCredit.appendChild(creditLink);
        pages.settings.appendChild(voiceCredit);

        const resetLabel = L('aura_drift.settings.reset_bests');
        const resetBtn = button(resetLabel, 'agdrift-secondary agdrift-reset');
        resetBtn.disabled = !!callbacks.readonly;
        let armed = 0;
        resetBtn.addEventListener('click', () => {
            if (!armed) {
                resetBtn.textContent = resetLabel + ' ?';
                resetBtn.classList.add('is-armed');
                armed = window.setTimeout(disarm, 3500);
                return;
            }
            disarm();
            if (typeof callbacks.onResetBests === 'function') callbacks.onResetBests();
        });
        function disarm() {
            window.clearTimeout(armed);
            armed = 0;
            resetBtn.textContent = resetLabel;
            resetBtn.classList.remove('is-armed');
        }
        pages.settings.appendChild(resetBtn);
        const fallbackNote = el('p', 'agdrift-note', L('aura_drift.settings.fallback_note'));
        fallbackNote.hidden = true;
        pages.settings.appendChild(fallbackNote);
        const credits = L('aura_drift.settings.credits', '');
        if (credits) pages.settings.appendChild(el('p', 'agdrift-note', credits));

        /* controls page */
        const table = el('div', 'agdrift-controls');
        table.setAttribute('role', 'table');
        for (const [id, keys] of CONTROLS) {
            const r = el('div', 'agdrift-control');
            r.setAttribute('role', 'row');
            const caps = el('span', 'agdrift-caps');
            caps.setAttribute('role', 'cell');
            keys.split(' ').forEach(token => {
                caps.appendChild(el('span', token === '/' || token === '+' ? 'agdrift-cap-sep' : 'agdrift-cap', token));
            });
            const what = el('span', 'agdrift-control-what', L('aura_drift.controls.' + id));
            what.setAttribute('role', 'cell');
            r.append(caps, what);
            table.appendChild(r);
        }
        pages.controls.appendChild(table);

        /* footer: summary, start, catch-up disclosure (always visible) */
        const foot = el('footer', 'agdrift-foot');
        const summary = el('p', 'agdrift-summary');
        foot.appendChild(summary);
        const startBtn = button(labels.start, 'agdrift-primary', () => {
            sound('start');
            const rules = AD.modeRules(state.mode);
            const config = {
                mode: state.mode, trackId: state.track, laps: state.laps, carId: state.car, livery: state.livery
            };
            if (rules.opponents > 0) config.opponents = Math.max(1, Math.min(5, state.rivals));
            settings.car = state.car;
            settings.livery = state.livery;
            callbacks.onStart(config);
        });
        foot.appendChild(startBtn);
        foot.appendChild(el('p', 'agdrift-hint agdrift-disclosure', labels.catchup));
        rail.appendChild(foot);

        /* stage: caption over the live garage */
        const stage = el('div', 'agdrift-stage');
        const spot = el('div', 'agdrift-spot');
        const spotTitle = el('p', 'agdrift-spot-title');
        const spotSub = el('p', 'agdrift-spot-sub');
        const spotBody = el('p', 'agdrift-spot-body');
        spot.append(spotTitle, spotSub, spotBody);
        stage.appendChild(spot);
        menu.append(rail, stage);

        /* ---- state -> DOM ------------------------------------------------ */
        const carOf = () => AD.carById(state.car);
        const trackOf = () => AD.trackById(state.track);
        const biomeOf = () => (AD.BIOMES && AD.BIOMES[trackOf().biome]) || { accent: '#9fd3ff' };

        function showcase() {
            if (typeof callbacks.onShowcase === 'function') {
                callbacks.onShowcase({ carId: state.car, livery: state.livery, biomeId: trackOf().biome });
            }
        }

        function bestText(track) {
            const drift = bests['drift:' + track.id];
            if (state.mode === 'drift_attack') {
                return drift && isFinite(drift.score) ? labels.record + ' ' + Math.round(drift.score).toLocaleString() : '';
            }
            const best = bests[track.id];
            return best && isFinite(best.time) ? labels.record + ' ' + AD.formatTime(best.time) : '';
        }

        function press(node, on) {
            node.setAttribute('aria-pressed', on ? 'true' : 'false');
            node.classList.toggle('is-active', !!on);
        }

        function refresh() {
            const rules = AD.modeRules(state.mode);
            const freeTrack = state.mode !== 'cup' && state.mode !== 'daily';
            modeCards.forEach(card => press(card, card.dataset.mode === state.mode));
            rivalsRow.hidden = !(rules.opponents > 0);
            rivalButtons.forEach((b, i) => press(b, i + 1 === state.rivals));
            carCards.forEach(card => press(card, card.dataset.car === state.car));
            const car = carOf();
            swatches.forEach((b, i) => { b.style.setProperty('--sw', car.liveries[i] || car.colour); press(b, i === state.livery); });
            trackCards.forEach(card => {
                card.disabled = !freeTrack;
                press(card, card.dataset.track === state.track);
                const track = AD.trackById(card.dataset.track);
                card._best.textContent = bestText(track);
            });
            lapsSelect.value = String(state.laps);
            lapsSelect.disabled = state.mode !== 'quick';
            lapsRow.classList.toggle('is-off', lapsSelect.disabled);
            tabButtons.forEach(b => {
                const on = b.dataset.tab === state.tab;
                b.setAttribute('aria-selected', on ? 'true' : 'false');
                b.classList.toggle('is-active', on);
            });
            TABS.forEach(id => { pages[id].hidden = id !== state.tab; });
            summary.textContent = [modeName(state.mode), car.name, state.mode === 'daily' ? labels.daily : trackOf().name].join('  ·  ');
            menu.style.setProperty('--acc', biomeOf().accent);
            menu.style.setProperty('--car', car.colour);
            caption();
            syncs.forEach(sync => sync());
        }

        function caption() {
            let head = '', sub = '', body = '';
            if (state.tab === 'car') {
                const car = carOf();
                head = car.name;
                sub = L('aura_drift.car.' + car.id + '.tag', '');
                body = L('aura_drift.car.' + car.id + '.quirk', '');
            } else if (state.tab === 'track') {
                const track = trackOf();
                head = track.name;
                sub = L('aura_drift.biome.' + track.biome);
                body = L('aura_drift.track.' + track.id + '.blurb', '');
            } else if (state.tab === 'mode') {
                head = modeName(state.mode);
                body = L('aura_drift.mode.' + state.mode + '.desc', '');
            }
            spotTitle.textContent = head;
            spotSub.textContent = sub;
            spotBody.textContent = body;
            spot.classList.toggle('is-empty', !head);
        }

        function showTab(id) {
            if (state.tab === id) return;
            state.tab = id;
            sound('move');
            refresh();
        }

        /* ---- pause ------------------------------------------------------ */
        pause.setAttribute('role', 'dialog');
        pause.setAttribute('aria-label', labels.pause);
        pause.appendChild(el('div', 'agdrift-kicker', 'AURA DRIFT'));
        pause.appendChild(el('h2', 'agdrift-heading', labels.pause));
        const resumeBtn = button(labels.resume, 'agdrift-primary', () => { sound('confirm'); callbacks.onResume(); });
        pause.appendChild(resumeBtn);
        pause.appendChild(button(labels.restart, 'agdrift-secondary', () => { sound('confirm'); callbacks.onRestart(); }));
        pause.appendChild(button(labels.menu, 'agdrift-secondary', () => { sound('back'); callbacks.onQuit(); }));

        /* ---- loading ---------------------------------------------------- */
        loading.setAttribute('role', 'status');
        const loadTitle = el('p', 'agdrift-load-title', labels.loading);
        const bar = el('div', 'agdrift-progress');
        bar.setAttribute('role', 'progressbar');
        bar.setAttribute('aria-valuemin', '0');
        bar.setAttribute('aria-valuemax', '100');
        bar.setAttribute('aria-label', L('aura_drift.settings.loading_assets'));
        const barFill = el('i');
        bar.appendChild(barFill);
        const loadText = el('p', 'agdrift-load-text', L('aura_drift.settings.loading_assets'));
        const cancelBtn = button(labels.menu, 'agdrift-secondary', () => { sound('back'); if (callbacks.onCancel) callbacks.onCancel(); });
        loading.append(loadTitle, bar, loadText, cancelBtn);

        /* ---- results ---------------------------------------------------- */
        results.setAttribute('role', 'dialog');
        results.setAttribute('aria-label', labels.results);
        const resultBody = el('div', 'agdrift-result-body');
        const resultButtons = el('div', 'agdrift-result-buttons');
        results.append(resultBody, resultButtons);

        function line(key, value) {
            const text = t(key);
            return typeof text === 'string' && text && text !== key ? AD.fmt(text, { value: value, n: value }) : humanize(key) + ': ' + value;
        }

        function medalClass(position) { return position === 1 ? 'is-gold' : (position === 2 ? 'is-silver' : (position === 3 ? 'is-bronze' : '')); }

        function buildResults(result, info) {
            resultBody.replaceChildren();
            resultButtons.replaceChildren();
            const mode = result.mode || 'quick';
            const tally = result.tally || {};
            const hero = el('div', 'agdrift-res-hero');
            hero.appendChild(el('div', 'agdrift-res-kicker', modeName(mode) + '  ·  ' + (info.track || '')));
            const big = el('div', 'agdrift-res-big');
            if (mode === 'drift_attack') {
                big.classList.add('is-score');
                big.appendChild(el('span', 'agdrift-res-pos', Math.round(result.score || 0).toLocaleString()));
                const medal = result.medal || 'none';
                big.appendChild(el('span', 'agdrift-res-medal is-' + medal, L('aura_drift.medal.' + medal)));
            } else if (mode === 'time_trial') {
                big.classList.add('is-score');
                big.appendChild(el('span', 'agdrift-res-pos', AD.formatTime(result.time)));
            } else {
                const medalCls = medalClass(result.position);
                if (medalCls) big.classList.add(medalCls);
                big.appendChild(el('span', 'agdrift-res-pos', String(result.position || '-')));
                big.appendChild(el('span', 'agdrift-res-of', '/' + (result.field || info.field || '')));
            }
            hero.appendChild(big);
            const titleText = result.title ? L('aura_drift.title.' + result.title) : '';
            if (titleText) hero.appendChild(el('h2', 'agdrift-res-title', titleText));
            if (info.record) hero.appendChild(el('p', 'agdrift-record', result.mode === 'drift_attack' && labels.newRecordScore ? labels.newRecordScore : labels.newRecord));
            resultBody.appendChild(hero);

            const rows = [];
            if (mode !== 'drift_attack') rows.push([labels.finish, AD.formatTime(result.time)]);
            if (isFinite(result.bestLap) && AD.modeRules(mode).laps !== 99) rows.push([labels.bestLap, AD.formatTime(result.bestLap)]);
            if (result.style) {
                if (mode !== 'drift_attack') rows.push([L('aura_drift.result.score'), Math.round(result.score || 0).toLocaleString()]);
                rows.push([labels.bestCombo, 'x' + (result.style.maxMultiplier || 1) + ' · ' + (result.style.drifts || 0) + ' ' + labels.combo]);
            }
            if (mode === 'cup' && info.cupTotal !== undefined) rows.push([L('aura_drift.result.cup_total'), String(info.cupTotal) + (result.points ? '  (+' + result.points + ')' : '')]);
            if ((mode === 'elimination' || mode === 'hot_potato')) {
                if (result.eliminatedAt !== null && result.eliminatedAt !== undefined) {
                    rows.push([line('aura_drift.result.eliminated_at', AD.formatTime(result.eliminatedAt).slice(0, 8)), '']);
                } else {
                    rows.push([L('aura_drift.result.survived'), '']);
                }
            }
            const grid = el('div', 'agdrift-res-grid');
            for (const [label, value] of rows) {
                const r = el('div', 'agdrift-result-row');
                r.appendChild(el('span', null, label));
                if (value) r.appendChild(el('strong', null, value));
                grid.appendChild(r);
            }
            resultBody.appendChild(grid);

            const counts = result.tricks && result.tricks.counts ? result.tricks.counts : {};
            const ids = Object.keys(counts).filter(id => counts[id] > 0).sort((a, b) => counts[b] - counts[a]).slice(0, 6);
            if (ids.length) {
                const chips = el('div', 'agdrift-chips');
                chips.setAttribute('aria-label', L('aura_drift.result.tricks'));
                ids.forEach(id => chips.appendChild(el('span', 'agdrift-chip', L('aura_drift.trick.' + id) + ' ×' + counts[id])));
                resultBody.appendChild(chips);
            }
            const tallyLines = [];
            if (tally.airTime >= 1) tallyLines.push(line('aura_drift.result.air_time', Math.round(tally.airTime)));
            if (tally.animalHits > 0) tallyLines.push(line('aura_drift.result.animals', tally.animalHits));
            if (tally.smashed > 0) tallyLines.push(line('aura_drift.result.smashed', tally.smashed));
            if (tallyLines.length) {
                const list = el('ul', 'agdrift-tally');
                tallyLines.forEach(text => list.appendChild(el('li', null, text)));
                resultBody.appendChild(list);
            }

            const again = button(labels.restart, 'agdrift-primary', () => { sound('confirm'); callbacks.onRestart(); });
            resultButtons.appendChild(info.hasNext ? button(labels.nextTrack, 'agdrift-primary', () => { sound('confirm'); callbacks.onNext(); }) : again);
            if (info.hasNext) resultButtons.appendChild(button(labels.restart, 'agdrift-secondary', () => { sound('confirm'); callbacks.onRestart(); }));
            resultButtons.appendChild(button(labels.menu, 'agdrift-secondary', () => { sound('back'); callbacks.onQuit(); }));
        }

        /* ---- touch (8 buttons, 56 px) --------------------------------- */
        function touchButton(label, key, className, tap) {
            const b = button(label, 'agdrift-touch-btn ' + className);
            const set = value => { if (key) touchState[key] = value; b.classList.toggle('is-down', !!value); };
            b.addEventListener('pointerdown', e => {
                e.preventDefault();
                set(1);
                if (tap && typeof callbacks[tap] === 'function') callbacks[tap]();
                if (key) b.setPointerCapture(e.pointerId);
            });
            const up = e => { e.preventDefault(); set(0); };
            b.addEventListener('pointerup', up);
            b.addEventListener('pointercancel', () => set(0));
            b.addEventListener('lostpointercapture', () => set(0));
            return b;
        }
        const leftPad = el('div', 'agdrift-touch-pad agdrift-touch-steer');
        leftPad.appendChild(touchButton('◀', 'left', 'agdrift-left'));
        leftPad.appendChild(touchButton('▶', 'right', 'agdrift-right'));
        const rightPad = el('div', 'agdrift-touch-pad agdrift-touch-act');
        rightPad.appendChild(touchButton('◉', null, 'agdrift-cam', 'onCamera'));
        rightPad.appendChild(touchButton('⚡', 'boost', 'agdrift-boost'));
        rightPad.appendChild(touchButton('✦', 'fire', 'agdrift-fire'));
        rightPad.appendChild(touchButton('↺', 'handbrake', 'agdrift-hand'));
        rightPad.appendChild(touchButton('▼', 'brake', 'agdrift-brake'));
        rightPad.appendChild(touchButton('▲', 'throttle', 'agdrift-gas'));
        touch.append(leftPad, rightPad);

        overlay.append(menu, loading, pause, results);
        root.appendChild(touch);
        root.appendChild(overlay);

        /* ---- view switching --------------------------------------------- */
        function show(node, side) {
            menu.hidden = node !== menu;
            loading.hidden = node !== loading;
            pause.hidden = node !== pause;
            results.hidden = node !== results;
            overlay.hidden = false;
            touch.hidden = true;
            /* The rail views push the 3D scene to the right so the car stays visible. */
            root.classList.toggle('is-rail', !!side);
            root.dataset.view = node === menu ? 'menu' : (node === loading ? 'loading' : (node === pause ? 'pause' : 'results'));
        }
        function focusFirst(node) {
            window.setTimeout(() => { try { node.focus({ preventScroll: true }); } catch (_) { /* detached */ } }, 30);
        }

        refresh();

        return {
            touch: touchState,
            showMenu(next) {
                bests = next || {};
                disarm();
                refresh();
                showcase();
                show(menu, true);
            },
            showLoading() { this.progress(0); show(loading, false); focusFirst(cancelBtn); },
            progress(p) {
                const pct = Math.max(0, Math.min(100, Math.round((isFinite(p) ? p : 0) * 100)));
                barFill.style.width = pct + '%';
                bar.setAttribute('aria-valuenow', String(pct));
                loadTitle.textContent = labels.loading + ' ' + pct + '%';
            },
            showPause() { show(pause, false); focusFirst(resumeBtn); },
            showResults(result, info) {
                buildResults(result, info || {});
                show(results, true);
                focusFirst(resultButtons.firstChild);
            },
            hide() { overlay.hidden = true; touch.hidden = false; root.classList.remove('is-rail'); root.dataset.view = 'play'; },
            setFallback(on) { fallbackNote.hidden = !on; },
            setBests(next) { bests = next || {}; refresh(); },
            destroy() { disarm(); overlay.remove(); touch.remove(); root.classList.remove('is-rail'); }
        };
    };
})();
