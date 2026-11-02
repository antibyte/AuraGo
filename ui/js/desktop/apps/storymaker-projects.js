// Storymaker app project list: the start screen. A new project asks for a title and a format; the list shows every project
// of the workspace (newest first) with open and delete (a confirmation through the desktop dialog). The titles are
// untrusted text and reach the page through textContent only. Classic script, publishes S.createProjects.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};
    const ASPECTS = [['16:9', 'storymaker.aspect_16_9'], ['9:16', 'storymaker.aspect_9_16'], ['1:1', 'storymaker.aspect_1_1']];

    // inst: {t, readonly, createProject(spec), openProject(id), deleteProject(project), confirm(title, message)}
    function createProjects(inst) {
        const { el, icon } = S;
        const t = inst.t;
        let projects = null;
        let aspect = '16:9';
        let busy = false;
        let message = '';

        const title = el('input', { class: 'stm-input', type: 'text', id: inst.domId('new-title'), maxlength: '120', required: true, autocomplete: 'off', placeholder: t('storymaker.projects.new_title_placeholder'), dataset: { stm: 'new-title' } });
        const errorLine = el('p', { class: 'stm-form-error', role: 'alert', hidden: true, dataset: { stm: 'projects-error' } });
        const aspectButtons = ASPECTS.map(([value, key]) => el('button', {
            type: 'button', class: 'stm-chip', dataset: { stm: 'aspect', aspect: value }, ariaPressed: String(value === aspect),
            onClick: () => { aspect = value; syncAspect(); }
        }, t(key)));
        const create = el('button', { type: 'submit', class: 'stm-btn stm-btn-primary', dataset: { stm: 'create-project' } }, icon('plus'), el('span', { text: t('storymaker.projects.create') }));
        const form = el('form', { class: 'stm-new', onSubmit: event => { event.preventDefault(); submit(); } },
            el('label', { class: 'stm-field', for: inst.domId('new-title') }, el('span', { class: 'stm-label', text: t('storymaker.projects.new_title_label') }), title),
            el('div', { class: 'stm-field' }, el('span', { class: 'stm-label', text: t('storymaker.projects.new_format') }), el('div', { class: 'stm-chips', role: 'group', ariaLabel: t('storymaker.projects.new_format') }, aspectButtons)),
            el('div', { class: 'stm-actions' }, create),
            errorLine);
        const list = el('ul', { class: 'stm-project-list', dataset: { stm: 'project-list' } });
        const empty = el('div', { class: 'stm-empty', hidden: true, dataset: { stm: 'projects-empty' } },
            icon('film', 'stm-empty-icon'),
            el('h3', { text: t('storymaker.projects.empty_title') }),
            el('p', { text: t('storymaker.projects.empty_text') }));
        const loading = el('p', { class: 'stm-muted', role: 'status', text: t('storymaker.loading') });
        const root = el('section', { class: 'stm-projects', ariaLabel: t('storymaker.projects.title') },
            el('header', { class: 'stm-projects-head' },
                el('h2', { text: t('storymaker.projects.title') }),
                el('p', { class: 'stm-muted', text: t('storymaker.projects.intro') })),
            form, loading, empty, list);

        function syncAspect() {
            aspectButtons.forEach(button => button.setAttribute('aria-pressed', String(button.dataset.aspect === aspect)));
        }

        function syncDisabled() {
            const off = inst.readonly || busy;
            title.disabled = inst.readonly;
            aspectButtons.forEach(button => { button.disabled = off; });
            create.disabled = off;
            create.title = inst.readonly ? t('storymaker.banner.readonly') : '';
            list.querySelectorAll('[data-stm="delete-project"]').forEach(button => { button.disabled = off; });
        }

        function setMessage(text) {
            message = text || '';
            errorLine.textContent = message;
            errorLine.hidden = !message;
        }

        async function submit() {
            const value = title.value.trim();
            if (!value) { setMessage(t('storymaker.projects.title_required')); title.focus(); return; }
            busy = true;
            setMessage('');
            syncDisabled();
            try {
                await inst.createProject({ title: value, aspect });
            } catch (err) {
                setMessage(S.errors.describe(t, err));
            } finally {
                busy = false;
                syncDisabled();
            }
        }

        function card(project) {
            const meta = [project.aspect, project.fps ? project.fps + ' fps' : '', S.format.dateTime(project.updated_at)].filter(Boolean).join(' · ');
            const open = el('button', { type: 'button', class: 'stm-project-open', dataset: { stm: 'open-project', id: project.id }, onClick: () => inst.openProject(project.id) },
                el('span', { class: 'stm-project-title', text: project.title || project.id }),
                el('span', { class: 'stm-project-meta', text: meta }));
            const remove = el('button', {
                type: 'button', class: 'stm-btn stm-btn-quiet stm-btn-icon', dataset: { stm: 'delete-project', id: project.id },
                ariaLabel: t('storymaker.projects.delete_label', { title: project.title || project.id }), title: t('storymaker.delete'),
                onClick: async () => {
                    const ok = await inst.confirm(t('storymaker.projects.delete_title'), t('storymaker.projects.delete_text', { title: project.title || project.id }));
                    if (!ok) return;
                    busy = true;
                    syncDisabled();
                    try {
                        await inst.deleteProject(project);
                    } catch (err) {
                        setMessage(S.errors.describe(t, err));
                    } finally {
                        busy = false;
                        syncDisabled();
                    }
                }
            }, icon('trash'));
            return el('li', { class: 'stm-project' }, open, remove);
        }

        function paint() {
            S.clear(list);
            loading.hidden = projects !== null;
            const sorted = (projects || []).slice().sort((a, b) => String(b.updated_at).localeCompare(String(a.updated_at)));
            sorted.forEach(project => list.appendChild(card(project)));
            empty.hidden = projects === null || sorted.length > 0;
            syncDisabled();
        }

        paint();
        return {
            el: root,
            setProjects(next) { projects = Array.isArray(next) ? next : []; paint(); },
            setError(text) { setMessage(text); },
            refresh() { syncDisabled(); },
            focus() { title.focus(); },
            count: () => (projects ? projects.length : 0)
        };
    }

    S.createProjects = createProjects;
})();
