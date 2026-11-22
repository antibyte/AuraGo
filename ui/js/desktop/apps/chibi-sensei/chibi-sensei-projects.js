// Chibi Sensei projects: the project list (cards, archived toggle, archive / restore / delete), the create dialog and the Goal step
// (recipe, task, labels, task text and the base-model cards). The Goal step keeps a draft and saves it explicitly with If-Match.
// Everything reaches the page through P.el as text. Classic script, publishes P.projects.
(function () {
    'use strict';

    const P = window.ChibiSenseiParts = window.ChibiSenseiParts || {};

    const RECIPE_DESC_KEYS = {
        assistant: 'chibi_sensei.recipe.assistant_desc',
        specialist: 'chibi_sensei.recipe.specialist_desc',
        free: 'chibi_sensei.recipe.free_desc'
    };
    const TASK_KIND_KEYS = {
        classify: 'chibi_sensei.task_kind.classify',
        extract: 'chibi_sensei.task_kind.extract',
        rewrite: 'chibi_sensei.task_kind.rewrite',
        to_json: 'chibi_sensei.task_kind.to_json'
    };
    const STATE_KEYS = {
        todo: 'chibi_sensei.readiness.todo',
        in_progress: 'chibi_sensei.readiness.in_progress',
        ready: 'chibi_sensei.readiness.ready',
        done: 'chibi_sensei.readiness.done',
        blocked: 'chibi_sensei.readiness.blocked'
    };
    const STEP_ORDER = ['goal', 'data', 'training', 'test', 'use'];
    const NAME_MAX = 120;
    const GOAL_MAX = 4000;

    function readonlyTitle(inst) { return inst.readonly ? inst.t('chibi_sensei.readonly_banner') : ''; }

    function applyWrite(inst, node) {
        node.disabled = inst.readonly;
        node.title = readonlyTitle(inst);
    }

    function profiles(inst) { return inst.meta && Array.isArray(inst.meta.profiles) ? inst.meta.profiles : []; }

    // Three recipe radio cards in a fieldset. Radios of one window stay apart from those of another through the surrounding form.
    function recipeCards(inst, dataKey, onPick) {
        const { el } = P;
        const t = inst.t;
        const inputs = {};
        const box = el('fieldset', { class: 'csn-recipe-cards csn-field' }, el('legend', { class: 'csn-label', text: t('chibi_sensei.create.recipe_label') }));
        P.RECIPES.forEach(id => {
            const input = el('input', { type: 'radio', name: 'recipe', value: id, dataset: { csn: dataKey }, onChange: () => { if (input.checked) onPick(id); } });
            inputs[id] = input;
            box.appendChild(el('label', { class: 'csn-recipe-card' }, input,
                el('span', { class: 'csn-recipe-name', text: t(P.recipeKey(id)) }),
                el('span', { class: 'csn-recipe-desc csn-muted', text: t(RECIPE_DESC_KEYS[id]) })));
        });
        return {
            el: box,
            set(value) { P.RECIPES.forEach(id => { inputs[id].checked = id === value; }); },
            setDisabled(flag) { P.RECIPES.forEach(id => { inputs[id].disabled = flag; }); },
            first() { return inputs[P.RECIPES[0]]; }
        };
    }

    // ---- create dialog ---------------------------------------------------------------------------------------------------------

    function createProjectDialog(inst) {
        const { el } = P;
        const t = inst.t;
        let node = null;

        function open() {
            if (inst.disposed || inst.readonly || node) return;
            const opener = document.activeElement;
            let recipe = 'assistant';
            let busy = false;
            const error = el('p', { class: 'csn-error', role: 'alert', dataset: { csn: 'create-error' } });
            error.hidden = true;
            const nameInput = el('input', { type: 'text', class: 'csn-input', maxlength: String(NAME_MAX), required: true, autocomplete: 'off', placeholder: t('chibi_sensei.create.name_placeholder'), dataset: { csn: 'create-name' } });
            const cards = recipeCards(inst, 'create-recipe', id => { recipe = id; });
            cards.set(recipe);
            const submit = el('button', { type: 'submit', class: 'csn-btn csn-btn-primary', dataset: { csn: 'create-submit' }, text: t('chibi_sensei.create.submit') });
            const cancel = el('button', { type: 'button', class: 'csn-btn csn-btn-quiet', dataset: { csn: 'create-cancel' }, text: t('chibi_sensei.create.cancel') });
            const form = el('form', { class: 'csn-dialog-form', novalidate: true },
                el('h3', { text: t('chibi_sensei.create.title') }),
                el('label', { class: 'csn-field' }, el('span', { class: 'csn-label', text: t('chibi_sensei.create.name_label') }), nameInput),
                cards.el, error,
                el('div', { class: 'csn-actions' }, submit, cancel));
            node = el('dialog', { class: 'csn-dialog', ariaLabel: t('chibi_sensei.create.title'), dataset: { csn: 'create-dialog' } }, form);
            const dialog = node;
            const close = () => { if (dialog.open) dialog.close(); };
            const showError = text => { error.textContent = text; error.hidden = !text; };
            cancel.addEventListener('click', close);
            dialog.addEventListener('close', () => {
                dialog.remove();
                if (node === dialog) node = null;
                if (opener && typeof opener.focus === 'function' && opener.isConnected) opener.focus();
            });
            form.addEventListener('submit', async event => {
                event.preventDefault();
                if (busy) return;
                const name = nameInput.value.trim();
                if (!name) { showError(t('chibi_sensei.create.name_required')); nameInput.focus(); return; }
                busy = true;
                submit.disabled = true;
                showError('');
                try {
                    const project = await inst.api.createProject({ name, recipe });
                    if (inst.disposed) return;
                    close();
                    await inst.openProject(project.id);
                } catch (err) {
                    if (inst.disposed || (err && err.aborted)) return;
                    showError(inst.handleError(err));
                } finally {
                    busy = false;
                    submit.disabled = false;
                }
            });
            (inst.root || inst.host || document.body).appendChild(dialog);
            dialog.showModal();
            nameInput.focus();
        }

        return { open };
    }

    // ---- project list ----------------------------------------------------------------------------------------------------------

    function stepsOf(project) { return project && project.steps && typeof project.steps === 'object' ? project.steps : {}; }

    function createList(inst) {
        const { el, icon } = P;
        const t = inst.t;
        const dialog = createProjectDialog(inst);
        let projects = [];
        let archived = false;
        let loaded = false;
        let seq = 0;
        let busyId = '';

        const error = el('p', { class: 'csn-error', role: 'alert', dataset: { csn: 'list-error' } });
        error.hidden = true;
        const newButton = el('button', { type: 'button', class: 'csn-btn csn-btn-primary', dataset: { csn: 'new-project' }, onClick: () => dialog.open() }, icon('plus'), el('span', { text: t('chibi_sensei.projects.new') }));
        const toggle = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'toggle-archived' }, ariaPressed: 'false', onClick: () => { archived = !archived; paintToggle(); refresh(); } });
        const grid = el('ul', { class: 'csn-project-grid', dataset: { csn: 'project-grid' } });
        const empty = el('div', { class: 'csn-empty', dataset: { csn: 'empty' } },
            el('h3', { text: t('chibi_sensei.projects.empty_title') }),
            el('p', { class: 'csn-muted', text: t('chibi_sensei.projects.empty_text') }));
        empty.hidden = true;
        const node = el('section', { class: 'csn-start-view', ariaLabel: t('chibi_sensei.projects.title'), dataset: { csn: 'projects' } },
            el('header', { class: 'csn-start-head' },
                el('div', { class: 'csn-start-copy' }, el('h2', { text: t('chibi_sensei.projects.title') }), el('p', { class: 'csn-muted', text: t('chibi_sensei.projects.intro') })),
                el('div', { class: 'csn-actions' }, toggle, newButton)),
            error, grid, empty);

        function paintToggle() {
            toggle.textContent = t(archived ? 'chibi_sensei.projects.show_active' : 'chibi_sensei.projects.show_archived');
            toggle.setAttribute('aria-pressed', archived ? 'true' : 'false');
        }

        function setError(text) {
            error.textContent = text || '';
            error.hidden = !text;
        }

        function modelOf(project) {
            const found = profiles(inst).find(p => p && p.id === project.profile_id);
            if (found) return found.display_name || found.id;
            return project.profile_id || t('chibi_sensei.projects.no_model');
        }

        function progress(project) {
            const steps = stepsOf(project);
            const done = STEP_ORDER.filter(id => steps[id] === 'done' || steps[id] === 'ready').length;
            const box = el('div', { class: 'csn-progress', role: 'img', ariaLabel: t('chibi_sensei.projects.progress_label', { done }) });
            STEP_ORDER.forEach(id => {
                const state = P.READINESS.indexOf(steps[id]) >= 0 ? steps[id] : 'todo';
                box.appendChild(el('span', { dataset: { state }, title: t(P.stepKey(id)) + ': ' + t(STATE_KEYS[state]) }));
            });
            return box;
        }

        function actionButton(label, key, iconName, onClick) {
            const button = el('button', { type: 'button', class: 'csn-btn csn-btn-quiet', dataset: { csn: key }, onClick }, icon(iconName), el('span', { text: label }));
            applyWrite(inst, button);
            if (busyId) button.disabled = true;
            return button;
        }

        function card(project) {
            const name = project.name || '';
            const open = el('button', { type: 'button', class: 'csn-project-open', dataset: { csn: 'project-card', id: project.id }, ariaLabel: t('chibi_sensei.projects.open_label', { name }), onClick: () => inst.openProject(project.id) },
                el('span', { class: 'csn-project-card-name', text: name }),
                el('span', { class: 'csn-project-card-meta csn-muted', text: t(P.recipeKey(project.recipe)) }),
                el('span', { class: 'csn-project-card-meta csn-muted', text: modelOf(project) }),
                el('span', { class: 'csn-project-card-meta csn-muted', text: t('chibi_sensei.projects.rows', { count: P.format.number(project.rows) }) }),
                progress(project));
            const actions = el('div', { class: 'csn-project-actions' });
            if (project.archived) {
                actions.appendChild(el('span', { class: 'csn-badge', text: t('chibi_sensei.projects.archived_badge') }));
                actions.appendChild(actionButton(t('chibi_sensei.projects.restore'), 'restore-project', 'undo', () => setArchived(project, false)));
                actions.appendChild(actionButton(t('chibi_sensei.projects.delete'), 'delete-project', 'trash', () => removeProject(project)));
            } else {
                actions.appendChild(actionButton(t('chibi_sensei.projects.archive'), 'archive-project', 'file', () => setArchived(project, true)));
            }
            return el('li', { class: 'csn-project-card', dataset: { archived: project.archived ? '1' : '0' } }, open, actions);
        }

        function paint() {
            P.clear(grid);
            const sorted = projects.slice().sort((a, b) => String(b.updated || '').localeCompare(String(a.updated || '')));
            sorted.forEach(project => grid.appendChild(card(project)));
            empty.hidden = !loaded || sorted.length > 0;
            applyWrite(inst, newButton);
        }

        function setProjects(list) {
            projects = Array.isArray(list) ? list.filter(p => p && typeof p.id === 'string') : [];
            loaded = true;
            paint();
        }

        async function refresh() {
            const mine = ++seq;
            setError('');
            try {
                const list = await inst.api.listProjects(archived);
                if (inst.disposed || mine !== seq) return;
                setProjects(list);
            } catch (err) {
                if (inst.disposed || mine !== seq || (err && err.aborted)) return;
                setError(inst.handleError(err));
            }
        }

        async function setArchived(project, flag) {
            if (inst.readonly || busyId) return;
            busyId = project.id;
            paint();
            try {
                await inst.api.archiveProject(project.id, flag);
                if (inst.disposed) return;
                inst.notify(t(flag ? 'chibi_sensei.projects.archived' : 'chibi_sensei.projects.restored', { name: project.name || '' }), 'info');
            } catch (err) {
                if (inst.disposed || (err && err.aborted)) return;
                setError(inst.handleError(err));
            } finally {
                busyId = '';
            }
            if (!inst.disposed) await refresh();
        }

        async function removeProject(project) {
            if (inst.readonly || busyId || typeof inst.ctx.modalDialog !== 'function') return;
            let value = null;
            try { value = await inst.ctx.modalDialog({ title: t('chibi_sensei.projects.delete_title'), message: t('chibi_sensei.projects.delete_text', { name: project.name || '' }), input: true, value: '' }); } catch (_) { value = null; }
            if (inst.disposed || typeof value !== 'string') return;
            if (value !== project.name) { setError(t('chibi_sensei.projects.delete_mismatch')); return; }
            busyId = project.id;
            paint();
            setError('');
            try {
                await inst.api.deleteProject(project.id, value);
                if (inst.disposed) return;
                inst.notify(t('chibi_sensei.projects.deleted', { name: project.name || '' }), 'info');
            } catch (err) {
                if (inst.disposed || (err && err.aborted)) return;
                setError(inst.handleError(err));
            } finally {
                busyId = '';
            }
            if (!inst.disposed) await refresh();
        }

        const off = inst.bus.on('readonly', () => paint());
        paintToggle();
        refresh();

        return {
            el: node, setProjects, setError, refresh,
            focus() { newButton.focus(); },
            dispose() { seq++; off(); }
        };
    }

    // ---- goal step -------------------------------------------------------------------------------------------------------------

    function parseLabels(text) {
        return String(text || '').split(',').map(s => s.trim()).filter(Boolean);
    }

    function draftOf(project) {
        const p = project || {};
        return { recipe: p.recipe || '', task_kind: p.task_kind || '', labels: Array.isArray(p.labels) ? p.labels.slice() : [], goal_text: p.goal_text || '', profile_id: p.profile_id || '' };
    }

    function createGoal(inst) {
        const { el } = P;
        const t = inst.t;
        let draft = draftOf(inst.project);
        let labelText = draft.labels.join(', ');
        let saving = false;
        let conflict = false;
        let issues = [];
        let errorText = '';
        let disposed = false;

        const message = el('p', { class: 'csn-error', role: 'alert', dataset: { csn: 'goal-error' } });
        message.hidden = true;
        const issueList = el('ul', { class: 'csn-issues' });
        const reload = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'goal-reload' }, text: t('chibi_sensei.goal.reload'), onClick: () => reloadProject() });
        const keepMine = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'goal-keep-mine' }, text: t('chibi_sensei.goal.keep_mine'), onClick: () => keepDraft() });
        const conflictBox = el('div', { class: 'csn-banner', role: 'alert', dataset: { csn: 'goal-conflict', tone: 'warn' } },
            el('span', { text: t('chibi_sensei.goal.conflict') }), el('span', { class: 'csn-actions' }, reload, keepMine));
        conflictBox.hidden = true;

        const cards = recipeCards(inst, 'goal-recipe', id => {
            draft.recipe = id;
            if (id === 'specialist') { if (!draft.task_kind) draft.task_kind = P.TASK_KINDS[0]; } else draft.task_kind = '';
            changed();
        });

        const taskSelect = el('select', { class: 'csn-select', dataset: { csn: 'goal-task-kind' }, onChange: () => { draft.task_kind = taskSelect.value; changed(); } });
        const kinds = (() => {
            const rec = inst.meta && Array.isArray(inst.meta.recipes) ? inst.meta.recipes.find(r => r && r.id === 'specialist') : null;
            const listed = rec && Array.isArray(rec.task_kinds) ? rec.task_kinds.filter(k => TASK_KIND_KEYS[k]) : [];
            return listed.length ? listed : P.TASK_KINDS;
        })();
        kinds.forEach(kind => taskSelect.appendChild(el('option', { value: kind, text: t(TASK_KIND_KEYS[kind]) })));
        const taskField = el('label', { class: 'csn-field', dataset: { csn: 'goal-task-field' } }, el('span', { class: 'csn-label', text: t('chibi_sensei.goal.task_kind_label') }), taskSelect);

        const labelsInput = el('input', { type: 'text', class: 'csn-input', dataset: { csn: 'goal-labels' }, autocomplete: 'off', onInput: () => { labelText = labelsInput.value; draft.labels = parseLabels(labelText); changed(); } });
        const labelsError = el('span', { class: 'csn-hint csn-error', text: t('chibi_sensei.goal.labels_required') });
        const labelsField = el('label', { class: 'csn-field', dataset: { csn: 'goal-labels-field' } },
            el('span', { class: 'csn-label', text: t('chibi_sensei.goal.labels_label') }), labelsInput,
            el('span', { class: 'csn-hint', text: t('chibi_sensei.goal.labels_hint') }), labelsError);

        const textArea = el('textarea', { class: 'csn-textarea', maxlength: String(GOAL_MAX), placeholder: t('chibi_sensei.goal.text_placeholder'), dataset: { csn: 'goal-text' }, onInput: () => { draft.goal_text = textArea.value; changed(); } });
        const textError = el('span', { class: 'csn-hint csn-error', text: t('chibi_sensei.goal.text_required') });
        const textField = el('label', { class: 'csn-field' }, el('span', { class: 'csn-label', text: t('chibi_sensei.goal.text_label') }), textArea, textError);

        // Profile cards: built once, the pressed state is painted.
        const profileButtons = new Map();
        const profileBox = el('div', { class: 'csn-profile-grid', role: 'group', ariaLabel: t('chibi_sensei.goal.profile_label') });
        const suggested = inst.project && inst.project.suggested_profile && typeof inst.project.suggested_profile === 'object' ? inst.project.suggested_profile : null;
        profiles(inst).forEach(profile => {
            if (!profile || typeof profile.id !== 'string') return;
            const lic = profile.license && typeof profile.license === 'object' ? profile.license : {};
            const body = [
                el('span', { class: 'csn-profile-name', text: profile.display_name || profile.id }),
                el('span', { class: 'csn-profile-line', text: t('chibi_sensei.goal.params', { size: P.format.number(profile.params_b) }) }),
                el('span', { class: 'csn-profile-line', text: t('chibi_sensei.goal.context', { tokens: P.format.number(profile.context_max) }) }),
                el('span', { class: 'csn-profile-line', text: t('chibi_sensei.goal.gguf_size', { mb: P.format.number(profile.gguf_estimated_mb) }) }),
                el('span', { class: 'csn-profile-line', text: t('chibi_sensei.goal.license', { name: lic.name || '' }) })
            ];
            if (lic.requires_consent) body.push(el('span', { class: 'csn-profile-line csn-muted', text: t('chibi_sensei.goal.license_consent') }));
            if (profile.status === 'experimental') {
                body.push(el('span', { class: 'csn-badge', text: t('chibi_sensei.experimental') }));
                body.push(el('span', { class: 'csn-profile-line csn-muted', text: t('chibi_sensei.goal.experimental_hint') }));
            }
            if (suggested && suggested.id === profile.id) {
                body.push(el('span', { class: 'csn-badge csn-badge-accent', text: t('chibi_sensei.goal.suggested') }));
                body.push(el('span', { class: 'csn-profile-line csn-muted', text: P.errors.knownKey(t, suggested.reason_key, 'chibi_sensei.suggest.generic') }));
            }
            const button = el('button', { type: 'button', class: 'csn-profile-card', ariaPressed: 'false', dataset: { csn: 'profile', id: profile.id }, onClick: () => { draft.profile_id = profile.id; changed(); } }, body);
            profileButtons.set(profile.id, button);
            profileBox.appendChild(button);
        });
        const profileField = el('div', { class: 'csn-field' }, el('span', { class: 'csn-label', text: t('chibi_sensei.goal.profile_label') }),
            profileButtons.size ? profileBox : el('p', { class: 'csn-muted', text: t('chibi_sensei.goal.no_profiles') }));
        const advanced = el('details', { class: 'csn-advanced' }, el('summary', { text: t('chibi_sensei.goal.advanced') }), el('p', { class: 'csn-muted', text: t('chibi_sensei.goal.advanced_text') }));

        const status = el('span', { class: 'csn-muted', dataset: { csn: 'goal-status' } });
        const saveButton = el('button', { type: 'submit', class: 'csn-btn csn-btn-primary', dataset: { csn: 'goal-save' }, text: t('chibi_sensei.goal.save') });
        const form = el('form', { class: 'csn-goal-form', novalidate: true, onSubmit: event => { event.preventDefault(); save(); } },
            cards.el, taskField, labelsField, textField, profileField, advanced,
            el('div', { class: 'csn-actions' }, saveButton, status));
        const node = el('section', { class: 'csn-goal', ariaLabel: t('chibi_sensei.goal.title'), dataset: { csn: 'goal' } },
            el('h2', { text: t('chibi_sensei.goal.title') }), conflictBox, form, message, issueList);

        function patchOf() {
            const base = draftOf(inst.project);
            const patch = {};
            ['recipe', 'task_kind', 'goal_text', 'profile_id'].forEach(key => { if (draft[key] !== base[key]) patch[key] = draft[key]; });
            if (JSON.stringify(draft.labels) !== JSON.stringify(base.labels)) patch.labels = draft.labels;
            return patch;
        }
        const isDirty = () => !!inst.project && Object.keys(patchOf()).length > 0;
        const isValid = () => P.steps ? P.steps.goalComplete(draft) : false;

        function setMessage(text, list) {
            errorText = text || '';
            issues = list || [];
            message.textContent = errorText;
            message.hidden = !errorText;
            P.clear(issueList);
            issues.forEach(issue => issueList.appendChild(el('li', { class: 'csn-detail', text: String(issue && (issue.field || issue.path || issue.code) || '') })));
            issueList.hidden = !issues.length;
        }

        function paint() {
            const ro = inst.readonly;
            cards.set(draft.recipe);
            cards.setDisabled(ro);
            taskField.hidden = draft.recipe !== 'specialist';
            taskSelect.value = draft.task_kind || kinds[0];
            taskSelect.disabled = ro;
            const classify = draft.recipe === 'specialist' && draft.task_kind === 'classify';
            labelsField.hidden = !classify;
            if (document.activeElement !== labelsInput) labelsInput.value = labelText;
            labelsInput.disabled = ro;
            labelsError.hidden = draft.labels.length >= 2;
            if (document.activeElement !== textArea) textArea.value = draft.goal_text;
            textArea.disabled = ro;
            textError.hidden = !!draft.goal_text.trim();
            profileButtons.forEach((button, id) => { button.setAttribute('aria-pressed', id === draft.profile_id ? 'true' : 'false'); button.disabled = ro; });
            const dirty = isDirty();
            saveButton.disabled = ro || saving || !dirty;
            saveButton.title = readonlyTitle(inst);
            status.textContent = dirty ? t('chibi_sensei.goal.unsaved') : '';
            conflictBox.hidden = !conflict;
            reload.disabled = keepMine.disabled = saving;
            keepMine.disabled = saving || inst.readonly;
        }

        function changed() {
            inst.goalDirty = isDirty();
            setMessage('', []);
            paint();
            inst.recompute();
        }

        function adopt(project) {
            inst.project = project;
            draft = draftOf(project);
            labelText = draft.labels.join(', ');
            inst.goalDirty = false;
            conflict = false;
        }

        async function reloadProject() {
            try {
                const fresh = await inst.api.getProject(inst.project.id);
                if (disposed || inst.disposed) return;
                adopt(fresh);
                inst.bus.emit('project', fresh);
                setMessage('', []);
                paint();
                inst.recompute();
            } catch (err) {
                if (disposed || inst.disposed || (err && err.aborted)) return;
                setMessage(inst.handleError(err), []);
            }
        }

        async function keepDraft() {
            try {
                const fresh = await inst.api.getProject(inst.project.id);
                if (disposed || inst.disposed) return;
                inst.project = fresh;
                conflict = false;
                await save();
            } catch (err) {
                if (disposed || inst.disposed || (err && err.aborted)) return;
                setMessage(inst.handleError(err), []);
            }
        }

        // Resolves true when nothing is left to save or the save went through; false when it failed (the message says why).
        async function save() {
            if (inst.readonly || !inst.project) return true;
            if (saving) return false;
            const patch = patchOf();
            if (!Object.keys(patch).length) return true;
            saving = true;
            setMessage('', []);
            paint();
            try {
                const updated = await inst.api.patchProject(inst.project.id, inst.project.rev, patch);
                if (disposed || inst.disposed) return true;
                adopt(updated);
                saving = false;
                inst.bus.emit('project', updated);
                paint();
                inst.recompute();
                inst.notify(t('chibi_sensei.goal.saved'), 'info');
                return true;
            } catch (err) {
                saving = false;
                if (disposed || inst.disposed || (err && err.aborted)) return false;
                const e = P.errors.normalize(err);
                if (e.status === 412) { conflict = true; setMessage('', []); }
                else setMessage(inst.handleError(err), e.status === 422 ? e.issues : []);
                paint();
                return false;
            }
        }

        const offs = [
            inst.bus.on('readonly', () => paint()),
            inst.bus.on('project', project => {
                if (disposed || saving || inst.goalDirty || !project) return;
                draft = draftOf(project);
                labelText = draft.labels.join(', ');
                paint();
            })
        ];
        paint();

        return {
            el: node, paint, isDirty, isValid, save,
            focus() { const first = cards.first(); if (first && !first.disabled) first.focus(); },
            dispose() { disposed = true; offs.forEach(off => off()); }
        };
    }

    P.projects = { createList, createProjectDialog, createGoal };
})();
