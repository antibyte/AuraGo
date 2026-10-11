// CYD Creator start screen (no project open): describe an app to the agent, start in the designer,
// import a ZIP (when the transfer module of plan 5 is loaded) and the recent projects. The
// new-project dialog asks for name, board (with its verified/community badge), theme, orientation
// and device language, plus the description when the agent builds the app.
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    const ORIENTATIONS = ['landscape', 'portrait', 'landscape_flipped', 'portrait_flipped'];
    const NAME_MAX = 64;
    const DESCRIPTION_MAX = 4000;
    const CET = 'CET-1CEST,M3.5.0,M10.5.0/3';
    const ZONES = {
        'Europe/Berlin': CET, 'Europe/Vienna': CET, 'Europe/Zurich': CET, 'Europe/Amsterdam': CET, 'Europe/Brussels': CET,
        'Europe/Paris': CET, 'Europe/Rome': CET, 'Europe/Madrid': CET, 'Europe/Stockholm': CET, 'Europe/Oslo': CET,
        'Europe/Copenhagen': CET, 'Europe/Prague': CET, 'Europe/Warsaw': CET, 'Europe/Luxembourg': CET,
        'Europe/London': 'GMT0BST,M3.5.0/1,M10.5.0', 'Europe/Dublin': 'IST-1GMT0,M10.5.0,M3.5.0/1', 'Europe/Lisbon': 'WET0WEST,M3.5.0/1,M10.5.0',
        'Europe/Athens': 'EET-2EEST,M3.5.0/3,M10.5.0/4', 'Europe/Helsinki': 'EET-2EEST,M3.5.0/3,M10.5.0/4',
        'America/New_York': 'EST5EDT,M3.2.0,M11.1.0', 'America/Chicago': 'CST6CDT,M3.2.0,M11.1.0',
        'America/Denver': 'MST7MDT,M3.2.0,M11.1.0', 'America/Los_Angeles': 'PST8PDT,M3.2.0,M11.1.0',
        'Asia/Tokyo': 'JST-9', 'Asia/Shanghai': 'CST-8', 'Asia/Kolkata': 'IST-5:30', 'UTC': 'UTC0', 'Etc/UTC': 'UTC0'
    };

    // posixTimezone maps the browser's IANA zone to the POSIX TZ string the device firmware needs.
    function posixTimezone(iana) {
        return ZONES[String(iana || '')] || 'UTC0';
    }

    function browserZone() {
        try { return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'; } catch (_) { return 'UTC'; }
    }

    function languageName(code) {
        try {
            const name = new Intl.DisplayNames([code], { type: 'language' }).of(code);
            return name ? name.charAt(0).toUpperCase() + name.slice(1) : code;
        } catch (_) {
            return code;
        }
    }

    function boardSize(board) {
        const d = board.display || {};
        return d.width && d.height ? d.width + '×' + d.height : '';
    }

    // themeName: the theme.* strings belong to plan 4b; until they exist the id reads "Dark sun".
    function themeName(t, id) {
        const words = String(id || '').split('_');
        words[0] = words[0].charAt(0).toUpperCase() + words[0].slice(1);
        return t.or('cyd_creator.theme.' + id, words.join(' '));
    }

    function create(inst) {
        const { el, bag } = P.util;
        const t = inst.t;
        const b = bag();
        const state = { projects: [], loading: false, disposed: false };

        function actionCard(action, titleKey, textKey) {
            return el('button', { type: 'button', class: 'cc-action-card', dataset: { ccAction: action } },
                el('span', { class: 'cc-action-icon', 'aria-hidden': 'true', text: action === 'new-agent' ? '✦' : (action === 'import' ? '⇪' : '▦') }),
                el('strong', { text: t(titleKey) }),
                el('span', { class: 'cc-action-text', text: t(textKey) }));
        }

        const agentCard = actionCard('new-agent', 'cyd_creator.home.new_agent', 'cyd_creator.home.new_agent_text');
        const designerCard = actionCard('new-designer', 'cyd_creator.home.new_designer', 'cyd_creator.home.new_designer_text');
        const importCard = actionCard('import', 'cyd_creator.home.import', 'cyd_creator.home.import_text');
        const toolchainHost = el('div', { class: 'cc-home-toolchain', dataset: { ccToolchain: '' } });
        const list = el('ul', { class: 'cc-project-list', dataset: { ccProjects: '' } });
        const listEmpty = el('p', { class: 'cc-empty', text: t('cyd_creator.home.no_projects') });
        const readonlyNote = el('p', { class: 'cc-banner', hidden: true, text: t('cyd_creator.app.readonly_hint') });
        const root = el('div', { class: 'cc-home', dataset: { ccHome: '' } },
            el('header', { class: 'cc-home-hero' },
                el('h2', { class: 'cc-home-title', text: t('cyd_creator.home.title') }),
                el('p', { class: 'cc-home-lead', text: t('cyd_creator.home.lead') })),
            readonlyNote,
            toolchainHost,
            el('div', { class: 'cc-home-actions' }, agentCard, designerCard, importCard),
            el('section', { class: 'cc-home-recent', 'aria-labelledby': inst.domId('home-recent') },
                el('h3', { id: inst.domId('home-recent'), text: t('cyd_creator.home.recent') }),
                listEmpty, list));

        function boardLabel(id) {
            const board = ((inst.caps && inst.caps.boards) || []).find(x => x.id === id);
            return board ? board.label : id;
        }

        function renderProjects() {
            listEmpty.hidden = state.projects.length > 0;
            list.replaceChildren(...state.projects.map(project => {
                const badges = [];
                if (project.locked_by_job) badges.push(el('span', { class: 'cc-tag', dataset: { tone: 'accent' }, text: t('cyd_creator.home.badge_agent') }));
                if (project.has_draft) badges.push(el('span', { class: 'cc-tag', dataset: { tone: 'warning' }, text: t('cyd_creator.home.badge_draft') }));
                if (project.last_build && project.last_build.status) {
                    badges.push(el('span', { class: 'cc-tag', dataset: { tone: project.last_build.status === 'succeeded' ? 'ok' : (project.last_build.status === 'failed' ? 'danger' : '') }, text: t('cyd_creator.build.status_' + project.last_build.status) }));
                }
                const open = el('button', { type: 'button', class: 'cc-project-open', dataset: { ccAction: 'open', ccProjectId: project.id } },
                    el('span', { class: 'cc-project-name', text: project.name }),
                    el('span', { class: 'cc-project-meta', text: [boardLabel(project.board), themeName(t, project.theme), P.format.time(project.updated_at)].filter(Boolean).join(' · ') }),
                    badges.length ? el('span', { class: 'cc-project-badges' }, badges) : null);
                open.addEventListener('click', () => inst.openProject(project.id));
                const del = el('button', {
                    type: 'button', class: 'cc-icon-btn', dataset: { ccAction: 'delete-project', ccProjectId: project.id },
                    disabled: inst.readonly || !!project.locked_by_job, 'aria-label': t('cyd_creator.home.delete_project', { name: project.name }), title: t('cyd_creator.home.delete'), text: '×'
                });
                del.addEventListener('click', () => deleteProject(project));
                return el('li', { class: 'cc-project-row' }, open, del);
            }));
        }

        function renderActions() {
            const ro = !!inst.readonly;
            readonlyNote.hidden = !ro;
            agentCard.disabled = ro;
            designerCard.disabled = ro;
            importCard.hidden = !(P.transfer && typeof P.transfer.importZip === 'function');
            importCard.disabled = ro;
        }

        async function refresh() {
            state.loading = true;
            renderActions();
            inst.renderToolchain(toolchainHost);
            try {
                const projects = await inst.api.listProjects();
                if (state.disposed) return;
                state.projects = projects.slice().sort((x, y) => String(y.updated_at || '').localeCompare(String(x.updated_at || '')));
                renderProjects();
            } catch (err) {
                if (!state.disposed) inst.notify('error', P.errors.message(t, err));
            } finally {
                state.loading = false;
            }
        }

        async function deleteProject(project) {
            const ok = await inst.confirm({ titleKey: 'cyd_creator.home.delete', bodyKey: 'cyd_creator.home.delete_confirm', vars: { name: project.name }, okKey: 'cyd_creator.home.delete', danger: true });
            if (!ok) return;
            try {
                await inst.api.deleteProject(project.id);
                await refresh();
            } catch (err) {
                inst.notify('error', P.errors.message(t, err));
            }
        }

        // ── new project dialog ──
        function field(labelKey, control, helpKey) {
            return el('label', { class: 'cc-form-field' }, el('span', { class: 'cc-form-label', text: t(labelKey) }), control,
                helpKey ? el('span', { class: 'cc-form-help', text: t(helpKey) }) : null);
        }

        function openNewProject(mode) {
            if (inst.readonly) return null;
            const caps = inst.caps || {};
            const boards = (caps.boards || []).slice().sort((x, y) => (x.status === 'verified' ? 0 : 1) - (y.status === 'verified' ? 0 : 1) || String(x.label).localeCompare(String(y.label)));
            const themes = caps.themes || [];
            const languages = (caps.languages && caps.languages.length ? caps.languages : ['en']).slice();
            const uiLang = String(t.lang() || 'en').slice(0, 2);
            const nameInput = el('input', { class: 'cc-input', type: 'text', maxlength: String(NAME_MAX), autofocus: true, dataset: { ccField: 'name' } });
            const descInput = el('textarea', { class: 'cc-input', rows: '5', maxlength: String(DESCRIPTION_MAX), placeholder: t('cyd_creator.home.description_placeholder'), dataset: { ccField: 'description' } });
            const boardGroup = el('div', { class: 'cc-board-list', role: 'radiogroup', 'aria-label': t('cyd_creator.home.board') }, boards.map((board, index) => {
                const radio = el('input', { type: 'radio', name: inst.domId('board'), value: board.id, checked: index === 0, dataset: { ccBoard: board.id } });
                return el('label', { class: 'cc-board-option' }, radio,
                    el('span', { class: 'cc-board-text' },
                        el('span', { class: 'cc-board-label', text: board.label }),
                        el('span', { class: 'cc-board-meta', text: [String(board.chip || '').toUpperCase(), boardSize(board), board.display && board.display.driver].filter(Boolean).join(' · ') })),
                    el('span', { class: 'cc-tag', dataset: { tone: board.status === 'verified' ? 'ok' : '' }, title: t.or('cyd_creator.home.board_hint_' + board.status, ''), text: t.or('cyd_creator.board.status_' + board.status, board.status) }));
            }));
            const themeGroup = el('div', { class: 'cc-theme-list', role: 'radiogroup', 'aria-label': t('cyd_creator.home.theme') }, themes.map((theme, index) => {
                const tokens = theme.tokens || {};
                const radio = el('input', { type: 'radio', name: inst.domId('theme'), value: theme.id, checked: index === 0, dataset: { ccTheme: theme.id } });
                return el('label', { class: 'cc-theme-option' }, radio,
                    el('span', { class: 'cc-theme-sample', 'aria-hidden': 'true', style: { '--sw-bg': tokens.bg || '#000', '--sw-surface': tokens.surface || '#333', '--sw-accent': tokens.accent || '#888', '--sw-text': tokens.text || '#fff' } },
                        el('span', { class: 'cc-theme-sample-bar' }), el('span', { class: 'cc-theme-sample-dot' })),
                    el('span', { text: themeName(t, theme.id) }));
            }));
            const orientationSelect = el('select', { class: 'cc-select', dataset: { ccField: 'orientation' } },
                ORIENTATIONS.map(o => el('option', { value: o, text: t('cyd_creator.orientation.' + o) })));
            const languageSelect = el('select', { class: 'cc-select', dataset: { ccField: 'language' } },
                languages.map(code => el('option', { value: code, selected: code === uiLang, text: languageName(code) })));
            const body = [
                field('cyd_creator.home.name', nameInput),
                mode === 'agent' ? field('cyd_creator.home.description', descInput, 'cyd_creator.home.description_help') : null,
                el('fieldset', { class: 'cc-form-group' }, el('legend', { text: t('cyd_creator.home.board') }), boardGroup,
                    el('p', { class: 'cc-form-help', text: t('cyd_creator.home.board_help') })),
                el('fieldset', { class: 'cc-form-group' }, el('legend', { text: t('cyd_creator.home.theme') }), themeGroup),
                el('div', { class: 'cc-form-row' }, field('cyd_creator.home.orientation', orientationSelect), field('cyd_creator.home.language', languageSelect, 'cyd_creator.home.language_help'))
            ];
            const d = P.dialog.open(inst.root, {
                title: t(mode === 'agent' ? 'cyd_creator.home.dialog_agent_title' : 'cyd_creator.home.dialog_designer_title'),
                className: 'cc-dialog--wide',
                body,
                actions: [{ id: 'cancel', label: t('cyd_creator.app.cancel') }, { id: 'create', label: t(mode === 'agent' ? 'cyd_creator.home.create_agent' : 'cyd_creator.home.create'), kind: 'primary' }],
                owner: b,
                async onAction(id, dialog) {
                    if (id !== 'create') { dialog.close(null); return; }
                    const name = nameInput.value.trim();
                    const description = descInput.value.trim();
                    const board = (boardGroup.querySelector('input:checked') || {}).value || '';
                    const theme = (themeGroup.querySelector('input:checked') || {}).value || '';
                    if (!name) { dialog.setError(t('cyd_creator.home.name_required')); nameInput.focus(); return; }
                    if (mode === 'agent' && !description) { dialog.setError(t('cyd_creator.home.description_required')); descInput.focus(); return; }
                    if (!board || !theme) { dialog.setError(t('cyd_creator.home.board_required')); return; }
                    dialog.setError('');
                    dialog.setBusy(true);
                    try {
                        const project = await inst.api.createProject({ name, board, theme, language: languageSelect.value, timezone: posixTimezone(browserZone()) });
                        dialog.close(project);
                        await inst.openProject(project.id, { orientation: orientationSelect.value, prompt: mode === 'agent' ? description : '' });
                    } catch (err) {
                        dialog.setBusy(false);
                        dialog.setError(P.errors.message(t, err));
                    }
                }
            });
            return d.promise;
        }

        async function importZip() {
            if (!P.transfer || typeof P.transfer.importZip !== 'function') return;
            try {
                const project = await P.transfer.importZip(inst);
                if (project && project.id) await inst.openProject(project.id);
            } catch (err) {
                inst.notify('error', P.errors.message(t, err));
            }
        }

        b.on(agentCard, 'click', () => openNewProject('agent'));
        b.on(designerCard, 'click', () => openNewProject('designer'));
        b.on(importCard, 'click', importZip);
        b.add(inst.bus.on('toolchain:changed', () => inst.renderToolchain(toolchainHost)));

        return {
            el: root,
            mount(host) { host.appendChild(root); return refresh(); },
            refresh,
            openNewProject,
            dispose() {
                state.disposed = true;
                b.dispose();
                root.remove();
            }
        };
    }

    P.home = { create, posixTimezone };
})();
