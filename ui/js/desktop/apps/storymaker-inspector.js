// Storymaker app inspector and library. The inspector edits the selected scene: environment and time of day (from the
// catalog), narration, the entry transition, the duration (automatic or fixed) and the actors and props that stand in it,
// and it moves or deletes the scene. The library lists the people, props and environments of the catalog with search; a card
// puts an actor or a prop into the selected scene, or sets its environment. Every edit goes through inst.editScene, which
// works on a clone of the story and saves it; narration keeps typing local until the debounce fires and no refresh ever
// replaces a control that has the focus. Classic script, publishes S.createInspector and S.createLibrary.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};
    const THUMB_RE = /^thumbs\/[A-Za-z0-9._-]+\.svg$/;
    const SUPPORTED_TRANSITIONS = ['cut', 'fade', 'slide-left', 'wipe'];

    function option(value, label, selected) {
        return S.el('option', { value, selected: selected || null, text: label });
    }

    // inst: {t, bus, story, selected, catalog, status, readonly, editScene(id, fn, opts), select(i), moveScene(i, to), deleteScene(i), openCode(id)}
    function createInspector(inst) {
        const { el, icon } = S;
        const t = inst.t;
        const body = el('div', { class: 'stm-inspector-body' });
        const root = el('section', { class: 'stm-inspector', ariaLabel: t('storymaker.inspector.title'), dataset: { stm: 'inspector' } },
            el('h3', { class: 'stm-panel-title', text: t('storymaker.inspector.title') }), body);
        let built = { id: '', kind: '', locked: false };
        let controls = {};

        function currentScene() {
            return S.model.scenesOf(inst.story)[inst.selected] || null;
        }

        function names(kind) {
            return S.model.entriesOf(inst.catalog, kind);
        }

        function environmentSelect(scene) {
            const select = el('select', { class: 'stm-select', id: inst.domId('env'), dataset: { stm: 'environment' } });
            const current = scene.environment || '';
            const known = names('environment');
            known.forEach(entry => select.appendChild(option(entry.id, S.catalogName(t, entry), entry.id === current)));
            if (current && !known.some(entry => entry.id === current)) select.appendChild(option(current, current, true));
            select.addEventListener('change', () => inst.editScene(scene.id, draft => { draft.environment = select.value; }, { immediate: true }));
            return select;
        }

        function variantSelect(scene) {
            const current = scene.variant || 'day';
            const select = el('select', { class: 'stm-select', id: inst.domId('variant'), dataset: { stm: 'variant' } },
                option('day', t('storymaker.inspector.variant_day'), current === 'day'),
                option('night', t('storymaker.inspector.variant_night'), current === 'night'));
            if (current !== 'day' && current !== 'night') select.appendChild(option(current, t('storymaker.inspector.unsupported', { name: current }), true));
            select.addEventListener('change', () => inst.editScene(scene.id, draft => { draft.variant = select.value; }, { immediate: true }));
            return select;
        }

        function transitionSelect(scene, first) {
            const select = el('select', { class: 'stm-select', id: inst.domId('transition'), dataset: { stm: 'transition' } });
            const current = scene.transition || (first ? 'cut' : 'fade');
            const listed = names('transition').map(entry => ({ value: entry.name, label: S.catalogName(t, entry) }));
            const offered = listed.length ? listed : SUPPORTED_TRANSITIONS.map(name => ({ value: name, label: name }));
            offered.forEach(item => select.appendChild(option(item.value, item.label, item.value === current)));
            if (!offered.some(item => item.value === current)) select.appendChild(option(current, t('storymaker.inspector.unsupported', { name: current }), true));
            select.disabled = first;
            select.addEventListener('change', () => inst.editScene(scene.id, draft => { draft.transition = select.value; }, { immediate: true }));
            return select;
        }

        function durationControls(scene) {
            const state = S.model.durationOf(scene);
            const mode = el('select', { class: 'stm-select', id: inst.domId('duration-mode'), dataset: { stm: 'duration-mode' } },
                option('auto', t('storymaker.inspector.duration_auto'), state.auto),
                option('fixed', t('storymaker.inspector.duration_fixed'), !state.auto));
            const maxSeconds = inst.status && inst.status.limits && Number(inst.status.limits.max_duration_seconds) > 0 ? Number(inst.status.limits.max_duration_seconds) : 600;
            const seconds = el('input', { class: 'stm-input stm-input-narrow', type: 'number', min: '0.5', max: String(maxSeconds), step: '0.5', value: state.auto ? '5' : String(state.seconds), ariaLabel: t('storymaker.inspector.duration_seconds'), dataset: { stm: 'duration-seconds' } });
            seconds.disabled = state.auto;
            const apply = () => inst.editScene(scene.id, draft => { S.model.setDuration(draft, mode.value === 'auto', seconds.value); }, { immediate: true });
            mode.addEventListener('change', () => { seconds.disabled = mode.value === 'auto'; apply(); });
            seconds.addEventListener('change', apply);
            return { mode, seconds };
        }

        function narrationField(scene) {
            const max = inst.status && inst.status.limits && Number(inst.status.limits.max_narration_chars) > 0 ? Number(inst.status.limits.max_narration_chars) : 0;
            const area = el('textarea', { class: 'stm-textarea', id: inst.domId('narration'), rows: '6', maxlength: max || null, spellcheck: 'true', dataset: { stm: 'narration' }, placeholder: t('storymaker.inspector.narration_placeholder') });
            area.value = scene.narration || '';
            area.addEventListener('input', () => inst.editScene(scene.id, draft => { S.model.setSceneField(draft, 'narration', area.value); }, { immediate: false }));
            return area;
        }

        function field(labelKey, controlId, control, hint) {
            return el('div', { class: 'stm-field' },
                el('label', { class: 'stm-label', for: controlId, text: t(labelKey) }),
                control,
                hint ? el('p', { class: 'stm-hint', text: hint }) : null);
        }

        function itemsList(scene) {
            const list = el('ul', { class: 'stm-items', dataset: { stm: 'scene-items' } });
            const castName = id => {
                const member = (inst.story.cast || []).find(candidate => candidate && candidate.id === id);
                if (!member) return id;
                const entry = inst.catalog && inst.catalog.byId ? inst.catalog.byId[member.character] : null;
                return member.name || S.catalogName(t, entry) || id;
            };
            (Array.isArray(scene.actors) ? scene.actors : []).forEach((actor, k) => {
                list.appendChild(el('li', { class: 'stm-item', dataset: { kind: 'actor' } },
                    icon('user'), el('span', { class: 'stm-item-name', text: castName(actor.cast) }),
                    el('button', { type: 'button', class: 'stm-btn stm-btn-quiet stm-btn-icon', dataset: { stm: 'remove-actor', index: k }, ariaLabel: t('storymaker.inspector.remove_item', { name: castName(actor.cast) }), title: t('storymaker.inspector.remove'), disabled: inst.readonly || null,
                        onClick: () => inst.editScene(scene.id, draft => { if (Array.isArray(draft.actors)) { draft.actors.splice(k, 1); if (!draft.actors.length) delete draft.actors; } }, { immediate: true }) }, icon('close'))));
            });
            (Array.isArray(scene.props) ? scene.props : []).forEach((prop, k) => {
                const entry = inst.catalog && inst.catalog.byId ? inst.catalog.byId[prop.item] : null;
                const name = S.catalogName(t, entry) || prop.item;
                list.appendChild(el('li', { class: 'stm-item', dataset: { kind: 'prop' } },
                    icon('box'), el('span', { class: 'stm-item-name', text: name }),
                    el('button', { type: 'button', class: 'stm-btn stm-btn-quiet stm-btn-icon', dataset: { stm: 'remove-prop', index: k }, ariaLabel: t('storymaker.inspector.remove_item', { name }), title: t('storymaker.inspector.remove'), disabled: inst.readonly || null,
                        onClick: () => inst.editScene(scene.id, draft => { if (Array.isArray(draft.props)) { draft.props.splice(k, 1); if (!draft.props.length) delete draft.props; } }, { immediate: true }) }, icon('close'))));
            });
            if (!list.children.length) list.appendChild(el('li', { class: 'stm-muted', text: t('storymaker.inspector.no_items') }));
            return list;
        }

        function build(scene, index) {
            S.clear(body);
            controls = {};
            const total = S.model.scenesOf(inst.story).length;
            const isCode = scene.kind === 'code';
            const head = el('div', { class: 'stm-inspector-head' },
                el('div', { class: 'stm-inspector-title' },
                    el('strong', { text: t('storymaker.strip.scene_n', { n: index + 1 }) }),
                    el('span', { class: 'stm-muted', text: scene.id })),
                el('div', { class: 'stm-inspector-tools' },
                    el('button', { type: 'button', class: 'stm-btn stm-btn-quiet stm-btn-icon', dataset: { stm: 'scene-up' }, ariaLabel: t('storymaker.inspector.move_up'), title: t('storymaker.inspector.move_up'), disabled: inst.readonly || index === 0 || null, onClick: () => inst.moveScene(index, index - 1) }, icon('up')),
                    el('button', { type: 'button', class: 'stm-btn stm-btn-quiet stm-btn-icon', dataset: { stm: 'scene-down' }, ariaLabel: t('storymaker.inspector.move_down'), title: t('storymaker.inspector.move_down'), disabled: inst.readonly || index >= total - 1 || null, onClick: () => inst.moveScene(index, index + 1) }, icon('down')),
                    el('button', { type: 'button', class: 'stm-btn stm-btn-quiet stm-btn-icon', dataset: { stm: 'scene-delete' }, ariaLabel: t('storymaker.inspector.delete_scene'), title: t('storymaker.inspector.delete_scene'), disabled: inst.readonly || total <= 1 || null, onClick: () => inst.deleteScene(index) }, icon('trash'))));
            body.appendChild(head);
            if (isCode) {
                body.appendChild(el('div', { class: 'stm-note' }, icon('code'), el('span', { text: t('storymaker.inspector.code_scene_note') }),
                    el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'edit-code' }, onClick: () => inst.openCode(scene.id) }, el('span', { text: t('storymaker.inspector.edit_code') }))));
            } else {
                controls.environment = environmentSelect(scene);
                controls.variant = variantSelect(scene);
                body.appendChild(field('storymaker.inspector.environment', inst.domId('env'), controls.environment));
                body.appendChild(field('storymaker.inspector.variant', inst.domId('variant'), controls.variant));
            }
            controls.narration = narrationField(scene);
            body.appendChild(field('storymaker.inspector.narration', inst.domId('narration'), controls.narration, t('storymaker.inspector.narration_hint')));
            controls.transition = transitionSelect(scene, index === 0);
            body.appendChild(field('storymaker.inspector.transition', inst.domId('transition'), controls.transition, index === 0 ? t('storymaker.inspector.transition_first') : ''));
            const duration = durationControls(scene);
            controls.durationMode = duration.mode;
            controls.durationSeconds = duration.seconds;
            body.appendChild(el('div', { class: 'stm-field' },
                el('label', { class: 'stm-label', for: inst.domId('duration-mode'), text: t('storymaker.inspector.duration') }),
                el('div', { class: 'stm-row' }, duration.mode, duration.seconds, el('span', { class: 'stm-muted', text: t('storymaker.inspector.seconds_unit') }))));
            if (!isCode) {
                controls.items = itemsList(scene);
                body.appendChild(el('div', { class: 'stm-field' }, el('span', { class: 'stm-label', text: t('storymaker.inspector.cast_props') }), controls.items));
            }
            applyReadonly();
        }

        function applyReadonly() {
            ['environment', 'variant', 'narration'].forEach(name => { if (controls[name]) controls[name].disabled = inst.readonly; });
            if (controls.narration) controls.narration.readOnly = inst.readonly;
            if (controls.durationMode) controls.durationMode.disabled = inst.readonly;
            if (controls.durationSeconds) controls.durationSeconds.disabled = inst.readonly || controls.durationMode.value === 'auto';
        }

        // Same scene, same kind: only the values move, and never under the user's hands.
        function refresh(scene, index) {
            const set = (control, value) => { if (control && document.activeElement !== control && control.value !== value) control.value = value; };
            set(controls.environment, scene.environment || '');
            set(controls.variant, scene.variant || 'day');
            set(controls.narration, scene.narration || '');
            const duration = S.model.durationOf(scene);
            set(controls.durationMode, duration.auto ? 'auto' : 'fixed');
            if (controls.durationSeconds && document.activeElement !== controls.durationSeconds) {
                controls.durationSeconds.value = duration.auto ? controls.durationSeconds.value : String(duration.seconds);
                controls.durationSeconds.disabled = inst.readonly || duration.auto;
            }
            set(controls.transition, scene.transition || (index === 0 ? 'cut' : 'fade'));
            if (controls.items) {
                const next = itemsList(scene);
                controls.items.replaceWith(next);
                controls.items = next;
            }
            applyReadonly();
        }

        function paint() {
            const scene = currentScene();
            if (!scene) { S.clear(body); built = { id: '', kind: '' }; return; }
            const kind = scene.kind === 'code' ? 'code' : 'storyboard';
            const total = S.model.scenesOf(inst.story).length;
            const key = scene.id + '|' + kind + '|' + inst.selected + '|' + total + '|' + inst.readonly + '|' + (inst.catalog ? inst.catalog.version : '');
            if (built.key !== key) {
                const hadFocus = body.contains(document.activeElement);
                build(scene, inst.selected);
                built = { id: scene.id, kind, key };
                if (hadFocus && controls.narration) controls.narration.focus();
            } else {
                refresh(scene, inst.selected);
            }
        }

        const off = [inst.bus.on('story', paint), inst.bus.on('selection', paint), inst.bus.on('readonly', paint)];
        paint();
        return { el: root, paint, dispose() { off.forEach(fn => fn()); } };
    }

    // inst: {t, bus, story, selected, catalog, readonly, addActor(entry), addProp(entry), setEnvironment(id), actorInScene(entry)}
    function createLibrary(inst) {
        const { el, icon } = S;
        const t = inst.t;
        const TABS = [['character', 'storymaker.library.people', 'user'], ['prop', 'storymaker.library.props', 'box'], ['environment', 'storymaker.library.environments', 'scenery']];
        let tab = 'character';
        let query = '';
        const search = el('input', { class: 'stm-input', type: 'search', id: inst.domId('library-search'), placeholder: t('storymaker.library.search'), ariaLabel: t('storymaker.library.search'), autocomplete: 'off', dataset: { stm: 'library-search' } });
        const tabs = el('div', { class: 'stm-tabs stm-tabs-small', role: 'tablist', ariaLabel: t('storymaker.library.title') },
            TABS.map(([kind, key, glyph]) => el('button', {
                type: 'button', class: 'stm-tab', role: 'tab', id: inst.domId('lib-' + kind), dataset: { stm: 'library-tab', kind }, ariaSelected: String(kind === tab),
                onClick: () => { tab = kind; paint(); }
            }, icon(glyph), el('span', { text: t(key) }))));
        const grid = el('ul', { class: 'stm-cards', dataset: { stm: 'library-grid' } });
        const empty = el('p', { class: 'stm-muted', hidden: true, text: t('storymaker.library.empty') });
        const root = el('section', { class: 'stm-library', ariaLabel: t('storymaker.library.title'), dataset: { stm: 'library' } },
            el('h3', { class: 'stm-panel-title', text: t('storymaker.library.title') }), tabs, search, grid, empty);

        function matches(entry) {
            if (!query) return true;
            const haystack = [S.catalogName(t, entry), entry.id, ...(Array.isArray(entry.tags) ? entry.tags : [])].join(' ').toLowerCase();
            return query.split(/\s+/).every(word => haystack.indexOf(word) >= 0);
        }

        function card(entry) {
            const scene = S.model.scenesOf(inst.story)[inst.selected];
            const name = S.catalogName(t, entry);
            const isCode = !scene || scene.kind === 'code';
            const present = entry.kind === 'character' ? inst.actorInScene(entry) : entry.kind === 'environment' ? !!scene && scene.environment === entry.id : false;
            const disabled = inst.readonly || isCode;
            const thumb = typeof entry.thumb === 'string' && THUMB_RE.test(entry.thumb) ? el('img', { class: 'stm-card-thumb', src: '/img/storymaker/' + entry.thumb, alt: '', width: '96', height: '96', loading: 'lazy', decoding: 'async' }) : icon('sparkle', 'stm-card-thumb');
            const action = entry.kind === 'environment' ? t('storymaker.library.use') : t('storymaker.library.add');
            return el('li', { class: 'stm-card-item' }, el('button', {
                type: 'button', class: 'stm-card', dataset: { stm: 'library-card', id: entry.id, kind: entry.kind, present: String(present) }, disabled: disabled || null,
                ariaLabel: action + ': ' + name, ariaPressed: entry.kind === 'prop' ? null : String(present),
                title: isCode ? t('storymaker.library.code_scene') : action,
                onClick: () => {
                    if (entry.kind === 'character') inst.addActor(entry);
                    else if (entry.kind === 'prop') inst.addProp(entry);
                    else inst.setEnvironment(entry.id);
                }
            }, thumb, el('span', { class: 'stm-card-name', text: name }), present ? el('span', { class: 'stm-card-mark' }, icon('check')) : null));
        }

        function paint() {
            tabs.querySelectorAll('[data-kind]').forEach(button => button.setAttribute('aria-selected', String(button.dataset.kind === tab)));
            S.clear(grid);
            const entries = S.model.entriesOf(inst.catalog, tab).filter(matches);
            entries.forEach(entry => grid.appendChild(card(entry)));
            empty.hidden = entries.length > 0;
        }

        search.addEventListener('input', () => { query = search.value.trim().toLowerCase(); paint(); });
        const off = [inst.bus.on('story', paint), inst.bus.on('selection', paint), inst.bus.on('readonly', paint), inst.bus.on('catalog', paint)];
        paint();
        return { el: root, paint, dispose() { off.forEach(fn => fn()); } };
    }

    S.createInspector = createInspector;
    S.createLibrary = createLibrary;
})();
