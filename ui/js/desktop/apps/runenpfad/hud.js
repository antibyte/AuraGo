/* Runenpfad HUD: slim rune-stone top bar, pixel icons from the jump-and-run sheet, boss bar and key-cap helpers.
 * Builds DOM only (no game canvas); app.js owns the window, dialogs and input. */
(function (root) {
    'use strict';

    const ICON_ASSETS = {
        heart: 'heart', coin: 'coin', key: 'key', star: 'star', potion: 'potion',
        gem_red: 'red_gem', gem_blue: 'blue_gem', gem_green: 'green_gem'
    };
    const ICON_FALLBACK = {
        heart: '\u2665', coin: '\u25cf', key: '\u2020', star: '\u2605', potion: '\u2726',
        gem_red: '\u25c6', gem_blue: '\u25c6', gem_green: '\u25c6'
    };
    const GEM_KINDS = ['gem_red', 'gem_blue', 'gem_green'];
    const HINT_MS = 6000;

    function create(o) {
        const el = o.el;
        const t = o.t;
        const fill = o.fill;
        const Levels = o.Levels;
        const Sim = o.Sim;
        const icons = [];
        const crops = Object.create(null);
        let sheets = null;
        let hintTimer = 0;
        let heartCount = -1;
        let atlas = null;
        let runeKind = '';
        let runeShownAt = 0;

        function setText(node, value) {
            const text = String(value);
            if (node.__rpText !== text) {
                node.__rpText = text;
                node.textContent = text;
            }
        }

        // Tight bounding box of the visible pixels, so small sprites fill the icon slot.
        function cropFor(assetId) {
            if (crops[assetId] !== undefined) return crops[assetId];
            crops[assetId] = null;
            const sheet = sheets && sheets['jump-and-run'];
            const asset = sheet && sheet.pack.assetsById[assetId];
            const frame = asset && asset.frames.length ? sheet.pack.framesByIndex[asset.frames[0]] : null;
            if (!frame) return null;
            let box = { x: 0, y: 0, w: frame.w, h: frame.h, frame: frame };
            try {
                const probe = document.createElement('canvas');
                probe.width = frame.w;
                probe.height = frame.h;
                const pctx = probe.getContext('2d');
                pctx.drawImage(sheet.image, frame.x, frame.y, frame.w, frame.h, 0, 0, frame.w, frame.h);
                const data = pctx.getImageData(0, 0, frame.w, frame.h).data;
                let x0 = frame.w, y0 = frame.h, x1 = -1, y1 = -1;
                for (let y = 0; y < frame.h; y++) {
                    for (let x = 0; x < frame.w; x++) {
                        if (data[(y * frame.w + x) * 4 + 3] > 12) {
                            if (x < x0) x0 = x;
                            if (x > x1) x1 = x;
                            if (y < y0) y0 = y;
                            if (y > y1) y1 = y;
                        }
                    }
                }
                if (x1 >= x0 && y1 >= y0) box = { x: x0, y: y0, w: x1 - x0 + 1, h: y1 - y0 + 1, frame: frame };
            } catch (err) { /* keep the full frame */ }
            crops[assetId] = box;
            return box;
        }

        function paintIcon(node) {
            const box = cropFor(ICON_ASSETS[node.dataset.rpIcon]);
            if (!box) return;
            const px = Number(node.dataset.rpPx) || 16;
            const size = px * 2;
            const canvas = el('canvas', 'rp-icon-canvas');
            canvas.width = size;
            canvas.height = size;
            const cx = canvas.getContext('2d');
            if (!cx) return;
            const k = Math.min(size / box.w, size / box.h);
            const dw = Math.max(1, Math.round(box.w * k));
            const dh = Math.max(1, Math.round(box.h * k));
            cx.imageSmoothingEnabled = true;
            cx.drawImage(sheets['jump-and-run'].image, box.frame.x + box.x, box.frame.y + box.y, box.w, box.h,
                Math.round((size - dw) / 2), Math.round((size - dh) / 2), dw, dh);
            node.replaceChildren(canvas);
            node.classList.add('is-ready');
        }

        // A pixel icon; shows a text glyph until the sprite sheet is available.
        function icon(kind, px) {
            const node = el('span', 'rp-icon rp-icon-' + kind, ICON_FALLBACK[kind] || '?');
            node.dataset.rpIcon = kind;
            node.dataset.rpPx = String(px || 16);
            node.style.setProperty('--rp-icon', (px || 16) + 'px');
            node.setAttribute('aria-hidden', 'true');
            // Only icons made before the sheets arrive wait in the list; later ones (dialogs, cards) paint at once.
            if (sheets) paintIcon(node);
            else icons.push(node);
            return node;
        }

        // Seconds as m:ss (stage clock, best times, par time).
        function formatTime(seconds) {
            const s = Math.max(0, Math.floor(Number(seconds) || 0));
            return Math.floor(s / 60) + ':' + (s % 60 < 10 ? '0' : '') + (s % 60);
        }

        // Orb icon from the painted atlas (powerups column per kind); the CSS colour dot stays when the atlas is absent.
        function paintRuneIcon(kind) {
            runeIcon.className = 'rp-rune-icon is-' + kind;
            runeIcon.replaceChildren();
            const A = root.RunenpfadArt;
            if (!atlas || !A || !A.has(atlas, 'powerups.' + kind)) return;
            const canvas = el('canvas', 'rp-rune-canvas');
            canvas.width = 56;
            canvas.height = 56;
            const cx = canvas.getContext('2d');
            if (!cx) return;
            A.draw(cx, atlas, 'powerups.' + kind, 0, 28, 28, { height: 58 });
            runeIcon.replaceChildren(canvas);
            runeIcon.classList.add('is-ready');
        }

        function setArt(a) {
            atlas = a || null;
            if (runeKind) paintRuneIcon(runeKind);
        }

        function paintRune(p) {
            const r = p.rune && p.rune.timeLeft > 0 ? p.rune : null;
            runePlaque.hidden = !r;
            if (!r) { runeKind = ''; return; }
            const now = root.performance.now();
            if (r.kind !== runeKind) {
                runeKind = r.kind;
                runeShownAt = now;
                paintRuneIcon(r.kind);
                setText(runeDesc, t('runenpfad.rune_' + r.kind + '_desc'));
                runePlaque.title = t('runenpfad.rune_' + r.kind) + ': ' + t('runenpfad.rune_' + r.kind + '_desc');
            }
            setText(runeName, t('runenpfad.rune_' + r.kind) + (r.kind === 'frost' ? ' \u00d7' + (r.charges || 0) : ''));
            runePlaque.classList.toggle('is-intro', now - runeShownAt < 2500);
            const frac = r.duration > 0 ? Math.max(0, Math.min(1, r.timeLeft / r.duration)) : 0;
            runeFill.style.width = Math.round(frac * 100) + '%';
            runePlaque.classList.toggle('is-ending', r.timeLeft < 2 && Math.floor(now / 220) % 2 === 0);
        }

        function paintRitual(stage) {
            const list = stage.rituals;
            let r = null;
            if (list) for (let i = 0; i < list.length; i++) if (list[i].state === 'active' && list[i].timeLimit > 0) { r = list[i]; break; }
            ritualPlaque.hidden = !r;
            if (!r) return;
            const left = Math.max(0, Math.min(r.timeLimit, r.timeLimit - r.t)); // r.t counts up
            const frac = left / r.timeLimit;
            ritualPlaque.title = t('runenpfad.notice.ritual_hint');
            setText(ritualNum, left.toFixed(1) + 's');
            ritualFill.style.width = Math.round(frac * 100) + '%';
            ritualPlaque.classList.toggle('is-urgent', frac < 0.3 && Math.floor(root.performance.now() / 200) % 2 === 0);
        }

        function setSheets(s) {
            sheets = s || null;
            if (sheets) icons.forEach(paintIcon);
        }

        // Arrow symbols are too small in the pixel font, so they use the UI font.
        function keycap(text) {
            return el('kbd', 'rp-keycap' + (/^[\u2190-\u2193]$/.test(text) ? ' rp-keycap-sym' : ''), text);
        }

        // Button label with an optional key-cap hint.
        function setButton(btn, label, key) {
            btn.replaceChildren(el('span', 'rp-btn-label', label));
            if (key) btn.append(keycap(key));
        }

        function chip(kind, extraClass) {
            const box = el('span', 'rp-chip rp-chip-' + kind + (extraClass ? ' ' + extraClass : ''));
            const value = el('span', 'rp-num');
            box.append(icon(kind, 16), value);
            return { box: box, value: value };
        }

        function tablet(label, className, dataKey) {
            const b = el('button', 'rp-btn rp-tablet' + (className ? ' ' + className : ''), label);
            b.type = 'button';
            if (dataKey) b.dataset.rpBtn = dataKey;
            return b;
        }

        // ---- top bar ----
        const rootEl = el('div', 'rp-hud');
        const bar = el('div', 'rp-topbar');

        const left = el('div', 'rp-bar-left');
        const heartsRow = el('span', 'rp-hearts');
        heartsRow.setAttribute('role', 'img');
        const healBtn = el('button', 'rp-btn rp-tablet rp-heal');
        healBtn.type = 'button';
        const healCount = el('span', 'rp-num');
        healBtn.append(icon('potion', 16), healCount);
        left.append(heartsRow, healBtn);

        const brand = el('div', 'rp-brand');
        const stageTitle = el('span', 'rp-stage-title');
        const subTitle = el('span', 'rp-sub-title');
        brand.append(stageTitle, subTitle);

        const stats = el('div', 'rp-stats');
        const coins = chip('coin');
        const gemsChip = el('span', 'rp-chip rp-chip-gems');
        const gemIcons = GEM_KINDS.map(function (k) {
            const g = icon(k, 16);
            gemsChip.append(g);
            return g;
        });
        const keyChip = chip('key');
        const starsChip = chip('star');
        const timeChip = el('span', 'rp-chip rp-chip-time');
        const timeValue = el('span', 'rp-num');
        timeChip.append(timeValue);
        stats.append(coins.box, gemsChip, keyChip.box, starsChip.box, timeChip);

        const actions = el('div', 'rp-actions');
        const heroBtn = tablet('', '', 'hero');
        const levelsBtn = tablet('', '', 'levels');
        const shopBtn = tablet('', 'rp-btn-gold', 'shop');
        const soundBtn = tablet('', '', 'sound');
        soundBtn.setAttribute('aria-pressed', 'true');
        const pauseBtn = tablet('', 'rp-btn-primary', 'pause');
        actions.append(heroBtn, levelsBtn, shopBtn, soundBtn, pauseBtn);
        bar.append(left, brand, stats, actions);

        // ---- notice row: status line plus compact trial / rune / combo tags ----
        const noticeRow = el('div', 'rp-notice-row');
        const notice = el('p', 'rp-notice');
        notice.setAttribute('role', 'status');
        // Speech-bubble mark shown in front of the notice while a camp NPC is in talking range.
        const talkMark = el('span', 'rp-notice-icon');
        talkMark.hidden = true;
        talkMark.setAttribute('aria-hidden', 'true');
        const tags = el('div', 'rp-tags');
        const modTag = el('span', 'rp-tag rp-tag-mod');
        const runeTag = el('span', 'rp-tag rp-tag-rune');
        const comboTag = el('span', 'rp-tag rp-tag-combo');
        const pathTag = el('span', 'rp-tag rp-tag-rune rp-tag-path');
        [modTag, runeTag, comboTag, pathTag].forEach(function (tag) { tag.hidden = true; });
        // Active rune power-up plaque: orb icon, name, short effect text and a draining timer bar.
        const runePlaque = el('div', 'rp-rune-plaque');
        runePlaque.hidden = true;
        const runeIcon = el('span', 'rp-rune-icon');
        runeIcon.setAttribute('aria-hidden', 'true');
        const runeBody = el('span', 'rp-rune-body');
        const runeName = el('span', 'rp-rune-name');
        const runeDesc = el('span', 'rp-rune-desc');
        const runeBar = el('span', 'rp-rune-bar');
        const runeFill = el('span', 'rp-rune-fill');
        runeBar.append(runeFill);
        runeBody.append(runeName, runeDesc, runeBar);
        runePlaque.append(runeIcon, runeBody);
        // Ritual countdown plaque (ritual_chain puzzle): a flame mark, the seconds left and a draining bar.
        const ritualPlaque = el('div', 'rp-ritual-plaque');
        ritualPlaque.hidden = true;
        const ritualMark = el('span', 'rp-ritual-mark', '✦');
        ritualMark.setAttribute('aria-hidden', 'true');
        const ritualNum = el('span', 'rp-ritual-num rp-num');
        const ritualBar = el('span', 'rp-ritual-bar');
        const ritualFill = el('span', 'rp-ritual-fill');
        ritualBar.append(ritualFill);
        ritualPlaque.append(ritualMark, ritualNum, ritualBar);
        tags.append(runePlaque, ritualPlaque, pathTag, modTag, runeTag, comboTag);
        noticeRow.append(talkMark, notice, tags);

        // ---- boss bar ----
        const bossWrap = el('div', 'rp-boss-wrap');
        bossWrap.hidden = true;
        const bossLabel = el('div', 'rp-boss-label');
        const bossName = el('span', 'rp-boss-name');
        const bossHpText = el('span', 'rp-num');
        bossLabel.append(bossName, bossHpText);
        const bossBar = el('div', 'rp-boss-bar');
        const bossFill = el('div', 'rp-boss-fill');
        bossBar.append(bossFill);
        bossWrap.append(bossLabel, bossBar);

        const controlsHint = el('p', 'rp-controls-hint');
        rootEl.append(bar, noticeRow, bossWrap, controlsHint);

        const bossCard = el('div', 'rp-bosscard');
        const bossCardName = el('strong');
        const bossCardSub = el('span');
        bossCard.setAttribute('aria-hidden', 'true');
        bossCard.append(bossCardName, bossCardSub);

        function paintTalkMark(on) {
            talkMark.hidden = !on;
            if (!on || talkMark.firstChild || !atlas || !root.RunenpfadArt.has(atlas, 'ui-npc-icons.talk')) return;
            const canvas = el('canvas', 'rp-notice-canvas');
            canvas.width = 56;
            canvas.height = 56;
            const cx = canvas.getContext('2d');
            if (!cx) return;
            root.RunenpfadArt.draw(cx, atlas, 'ui-npc-icons.talk', 0, 28, 28, { height: 52 });
            talkMark.append(canvas);
        }

        function paintCopy() {
            setButton(levelsBtn, t('runenpfad.btn_levels'));
            setButton(shopBtn, t('runenpfad.btn_shop'));
            setButton(pauseBtn, t('runenpfad.btn_pause'), 'P');
            healBtn.title = t('runenpfad.hud_heal') + ' (Q)';
            healBtn.setAttribute('aria-label', t('runenpfad.hud_heal'));
            controlsHint.textContent = t('runenpfad.controls');
            coins.box.title = t('runenpfad.hud_coins');
            gemsChip.title = t('runenpfad.hud_gems');
            starsChip.box.title = t('runenpfad.hud_stars');
            timeChip.title = t('runenpfad.hud_time');
        }

        function showHint() {
            controlsHint.classList.add('is-shown');
            root.clearTimeout(hintTimer);
            hintTimer = root.setTimeout(function () { controlsHint.classList.remove('is-shown'); }, HINT_MS);
        }

        function buildHearts(max) {
            heartsRow.replaceChildren();
            for (let i = 0; i < max; i++) heartsRow.append(icon('heart', 16));
            heartCount = max;
        }

        function paint(sim, profile) {
            const stage = sim.stage;
            const p = sim.player;
            const Pl = root.RunenpfadPlaces;
            const place = sim.interior && Pl ? Pl.placeOf(sim) : null;
            if (place) {
                // Inside a building: the place name; the world name stays as the sub title.
                setText(stageTitle, t(place.nameKey));
                setText(subTitle, t(stage.world.nameKey));
            } else if (stage.hub) {
                setText(stageTitle, t(stage.nameKey));
                setText(subTitle, t(stage.world.nameKey));
            } else if (sim.mode === 'endless') {
                setText(stageTitle, t('runenpfad.endless_title'));
                setText(subTitle, fill('runenpfad.endless_stage', { floor: sim.endlessFloor }) + ' \u00b7 ' + t(stage.world.nameKey));
            } else {
                setText(stageTitle, fill('runenpfad.hud_level', { world: stage.worldIndex + 1, stage: stage.stageInWorld, name: t(stage.nameKey) }));
                setText(subTitle, t(stage.world.nameKey));
            }
            if (heartCount !== p.maxHp) buildHearts(p.maxHp);
            Array.prototype.forEach.call(heartsRow.children, function (h, i) { h.classList.toggle('is-empty', i >= p.hp); });
            heartsRow.classList.toggle('is-low', p.hp <= 1 && sim.state === 'playing');
            heartsRow.setAttribute('aria-label', '\u2665 ' + p.hp + '/' + p.maxHp);
            heartsRow.title = '\u2665 ' + p.hp + '/' + p.maxHp;
            setText(healCount, '\u00d7' + p.potions);
            healBtn.disabled = p.potions <= 0 || p.hp >= p.maxHp;
            setText(coins.value, sim.profile.coins);
            gemIcons.forEach(function (g, i) { g.classList.toggle('is-empty', i >= sim.gemCount); });
            gemsChip.setAttribute('aria-label', t('runenpfad.hud_gems') + ' ' + sim.gemCount + '/3');
            keyChip.box.classList.toggle('is-on', !!sim.hasKey);
            keyChip.box.title = sim.hasKey ? t('runenpfad.hud_key_on') : t('runenpfad.hud_key_off');
            setText(keyChip.value, sim.hasKey ? '\u2713' : '\u2013');
            setText(starsChip.value, Sim.totalStars(profile) + '/75');
            setText(timeValue, formatTime(sim.elapsed));

            const mod = stage.mod;
            modTag.hidden = !mod;
            if (mod) {
                setText(modTag, t('runenpfad.hud_trial') + ': ' + t(mod.nameKey));
                modTag.title = t(mod.descKey);
            }
            const power = sim.blessing && sim.blessing.runePower > 0;
            runeTag.hidden = !power;
            if (power) setText(runeTag, t('runenpfad.hud_rune') + ' ' + Math.ceil(sim.blessing.runePower) + 's');
            paintRune(p);
            paintRitual(stage);
            const path = stage.resonancePath;
            let hint = '';
            if (sim.state === 'playing' && !sim.interior && !sim.encounter) {
                if (path && !path.complete && !path.needsLeave && (path.nextRune > 0 || Math.abs(p.x - path.runes[0].x) <= 180)) {
                    hint = path.nextRune === 0 ? t('runenpfad.resonance_hint') :
                        fill('runenpfad.resonance_progress', { count: path.nextRune, total: path.runes.length, seconds: Math.ceil(path.timeLeft) });
                } else if (stage.skyRays && stage.skyRays.some(ray =>
                    !ray.ridden && p.y >= ray.y && p.y - ray.y < 200 &&
                    p.x > ray.x - 100 && p.x < ray.x + ray.w + 100)) {
                    hint = t('runenpfad.sky_ray_hint');
                }
                if (!hint) {
                    const foe = stage.enemies.find(e => !e.dead && (e.moth || e.toad) &&
                        Math.abs(e.x - p.x) < 380 && Math.abs(e.y - p.y) < 245);
                    if (foe) hint = t(foe.moth ? 'runenpfad.encounter_moth' : 'runenpfad.encounter_toad');
                    else if (stage.thornRollers?.some(r => Math.abs(r.x - p.x) < 190 && Math.abs(r.y - p.y) < 125)) hint = t('runenpfad.encounter_roller');
                    else if (stage.moonSteps?.some(s => s.index === 0 && Math.abs(s.routeStartX - p.x) < 170 && Math.abs(s.y - p.y) < 170)) hint = t('runenpfad.encounter_moon');
                }
            }
            pathTag.hidden = !hint;
            if (hint) setText(pathTag, hint);
            const combo = sim.combo;
            comboTag.hidden = !(combo && combo.count >= 2);
            if (!comboTag.hidden) setText(comboTag, fill('runenpfad.hud_combo', { n: String.fromCharCode(215) + (Math.round((combo.mul || 1) * 10) / 10) }));

            const intro = sim.bossIntro;
            const introOn = !!(intro && intro.timer > 0);
            if (introOn) {
                setText(bossCardName, t(intro.nameKey));
                setText(bossCardSub, t('runenpfad.boss_intro_sub'));
            }
            bossCard.classList.toggle('is-on', introOn);
            setText(notice, sim.notice ? t(sim.notice) : '');
            notice.classList.toggle('is-empty', !sim.notice);
            paintTalkMark(sim.notice === 'runenpfad.npc.ui.talk_hint');
            // The chosen hero (profile) can run ahead of the stage hero once a stage is finished.
            const heroId = profile && Levels.HERO_DEFS[profile.hero] ? profile.hero : sim.heroId;
            if (heroBtn.__rpHero !== heroId) {
                heroBtn.__rpHero = heroId;
                setButton(heroBtn, t('runenpfad.btn_hero') + ': ' + t(Levels.HERO_DEFS[heroId].nameKey));
            }

            const boss = stage.bossId ? stage.enemies.find(function (e) { return e.id === stage.bossId; }) : null;
            const bossOn = !!(boss && !boss.dead);
            bossWrap.hidden = !bossOn;
            if (bossOn) {
                setText(bossName, t(boss.def.bossNameKey || stage.nameKey));
                setText(bossHpText, boss.hp + ' / ' + boss.maxHp);
                bossFill.style.width = Math.max(0, Math.min(100, Math.round((boss.hp / boss.maxHp) * 100))) + '%';
            }
            return bossOn;
        }

        // Star row (earned stars lit, the rest dark): big on the win card, small (px) on the world map tiles.
        function paintStars(container, earned, px) {
            container.replaceChildren();
            for (let i = 0; i < 3; i++) {
                const s = icon('star', px || 40);
                if (!px) s.classList.add('rp-bigstar');
                s.classList.toggle('is-empty', i >= earned);
                container.append(s);
            }
            container.setAttribute('role', 'img');
            container.setAttribute('aria-label', '\u2605 ' + earned + '/3');
        }

        // Summary plaques: [{ label, value, icon?, tag? }]; the tag is a small highlight line (new best time).
        function paintPlaques(container, items) {
            container.replaceChildren();
            items.forEach(function (it) {
                const plaque = el('div', 'rp-plaque');
                const head = el('span', 'rp-plaque-label');
                if (it.icon) head.append(icon(it.icon, 14));
                head.append(el('span', '', it.label));
                plaque.append(head, el('span', 'rp-plaque-value rp-num', it.value));
                if (it.tag) plaque.append(el('span', 'rp-plaque-tag', it.tag));
                container.append(plaque);
            });
        }

        // Win card summary: star row, the three star goals (one small star each) and the result plaques.
        // The goals mirror the simulation's star rules: portal, all gems, par time or a flawless run.
        function paintWinSummary(starsEl, goalsEl, plaquesEl, sim) {
            paintStars(starsEl, sim.earnedStars);
            goalsEl.replaceChildren();
            [
                { label: t('runenpfad.win_goal_portal'), done: true },
                { label: t('runenpfad.win_goal_gems'), done: sim.gemCount >= 3 },
                { label: fill('runenpfad.win_goal_swift', { time: formatTime(sim.stage.parTime) }), done: sim.elapsed <= sim.stage.parTime || sim.damageTaken === 0 }
            ].forEach(function (g) {
                const row = el('li', 'rp-goal' + (g.done ? ' is-done' : ''));
                const star = icon('star', 14);
                star.classList.toggle('is-empty', !g.done);
                row.append(star, el('span', '', g.label));
                goalsEl.append(row);
            });
            paintPlaques(plaquesEl, [
                { label: t('runenpfad.hud_time'), value: formatTime(sim.elapsed), tag: sim.newBest ? t('runenpfad.win_new_best') : '' },
                { label: t('runenpfad.hud_gems'), value: sim.gemCount + '/3', icon: 'gem_blue' },
                { label: t('runenpfad.hud_coins'), value: '+' + sim.stageCoins, icon: 'coin' },
                { label: t('runenpfad.hud_score'), value: String(sim.stageScore) }
            ]);
            // Final stage: the epilogue line of the story and the coins it paid out.
            const epi = sim.epilogue;
            if (epi && epi.key) {
                const box = el('div', 'rp-plaque rp-epilogue');
                box.append(el('span', 'rp-epilogue-text', t(epi.key)));
                if (epi.coins > 0) {
                    const c = el('span', 'rp-plaque-value rp-num');
                    c.append(icon('coin', 14), el('span', '', '+' + epi.coins));
                    box.append(c);
                }
                plaquesEl.append(box);
            }
        }

        // Key legend of the pause dialog: key caps in front of each action.
        function tf(key, fallback) {
            const w = t(key);
            return w && w !== key ? w : fallback;
        }

        // `inside` adds the top-down controls of building interiors.
        function paintLegend(container, inside) {
            container.replaceChildren();
            container.setAttribute('aria-label', t('runenpfad.controls'));
            (inside ? [
                { keys: ['W', 'A', 'S', 'D'], label: tf('runenpfad.legend.in_move', 'Inside: walk') },
                { keys: ['J', 'Enter'], label: tf('runenpfad.legend.in_attack', 'Inside: attack') },
                { keys: ['Space', 'K'], label: tf('runenpfad.legend.in_dash', 'Inside: dash') },
                { keys: ['E'], label: tf('runenpfad.legend.in_talk', 'Inside: talk, open, rest') }
            ] : []).concat([
                { keys: ['A', 'D', '\u2190', '\u2192'], label: t('runenpfad.ctl_move') },
                { keys: ['W', 'Space'], label: t('runenpfad.touch_jump') },
                { keys: ['J', 'Enter'], label: t('runenpfad.touch_attack') },
                { keys: ['K', 'Shift'], label: t('runenpfad.touch_special') },
                { keys: ['E', '↑'], label: t('runenpfad.npc.ui.talk') },
                { keys: ['Q'], label: t('runenpfad.hud_heal') },
                { keys: ['R'], label: t('runenpfad.pause_restart') },
                { keys: ['↓'], label: tf('runenpfad.npc.duel.legend_block', 'Duel: block') },
                { keys: ['Shift'], label: tf('runenpfad.npc.duel.legend_roll', 'Duel: roll') },
                { keys: ['E'], label: tf('runenpfad.npc.duel.legend_special', 'Duel: rune special') }
            ]).forEach(function (r) {
                const keys = el('dt', 'rp-legend-keys');
                r.keys.forEach(function (k) { keys.append(keycap(k)); });
                container.append(keys, el('dd', 'rp-legend-label', r.label));
            });
        }

        // Upgrade level as a row of pips (filled up to the current level).
        function pips(level, max) {
            const row = el('span', 'rp-pips');
            for (let i = 1; i <= max; i++) row.append(el('i', i <= level ? 'is-on' : ''));
            row.setAttribute('role', 'img');
            row.setAttribute('aria-label', level + '/' + max);
            return row;
        }

        function dispose() {
            root.clearTimeout(hintTimer);
        }

        return {
            root: rootEl, bossCard: bossCard,
            heroBtn: heroBtn, levelsBtn: levelsBtn, shopBtn: shopBtn, soundBtn: soundBtn, pauseBtn: pauseBtn, healBtn: healBtn,
            icon: icon, keycap: keycap, setButton: setButton, setSheets: setSheets, setArt: setArt, setText: setText,
            paint: paint, paintCopy: paintCopy, paintStars: paintStars, paintPlaques: paintPlaques,
            paintWinSummary: paintWinSummary, paintLegend: paintLegend, pips: pips, formatTime: formatTime,
            showHint: showHint, dispose: dispose
        };
    }

    root.RunenpfadHud = { create: create };
})(typeof window !== 'undefined' ? window : globalThis);
