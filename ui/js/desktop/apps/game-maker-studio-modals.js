(function () {
    'use strict';

    // Skills and revisions modals for Game Maker Studio. The modal framework
    // (showModal, busy state, errors, confirm) is injected by the main module.

    const skillStatuses = ['ready', 'installed', 'updated', 'verified', 'disabled', 'missing', 'hash_mismatch'];

    function skillStatusLabel(state, status) {
        if (skillStatuses.includes(status)) return state.context.t('game_maker.skill_status_' + status);
        return status;
    }

    function showSkillsModal(state, helpers) {
        const { esc, t } = state.context;
        const skills = (state.capabilities && state.capabilities.skills) || [];
        helpers.showModal(state, `<section class="gm-modal gm-skills-modal">
            <header><div><span>${esc(t('game_maker.curated'))}</span><h2>${esc(t('game_maker.skills_title'))}</h2></div>
                <button type="button" data-modal-close aria-label="${esc(t('game_maker.close'))}">×</button></header>
            <div class="gm-skill-list">${skills.map(skill => `<article>
                <div><strong>${esc(skill.name)}</strong><span class="gm-skill-status is-${esc(skill.status)}">${esc(skillStatusLabel(state, skill.status))}</span></div>
                <p>${esc(skill.description)}</p>
                <details class="gm-skill-details"><summary>${esc(t('game_maker.details'))}</summary>
                    <dl><dt>${esc(t('game_maker.source'))}</dt><dd>${esc(skill.source)}</dd>
                        <dt>${esc(t('game_maker.commit'))}</dt><dd>${esc(skill.commit)}</dd>
                        <dt>${esc(t('game_maker.license'))}</dt><dd>${esc(skill.license)}</dd></dl>
                </details>
            </article>`).join('')}</div>
            <footer><button type="button" data-modal-close>${esc(t('game_maker.close'))}</button></footer>
        </section>`);
    }

    function formatRevisionTime(iso) {
        const stamp = Date.parse(iso || '');
        if (!stamp) return '';
        return ' · ' + new Date(stamp).toLocaleString([], { dateStyle: 'short', timeStyle: 'short' });
    }

    async function showRevisionsModal(state, helpers) {
        if (!state.project) return;
        const { esc, t } = state.context;
        try {
            const body = await state.api.revisions(state.project.id);
            if (state.disposed) return;
            helpers.showModal(state, `<section class="gm-modal gm-revisions-modal">
                <header><div><span>${esc(t('game_maker.history'))}</span><h2>${esc(t('game_maker.revisions_title'))}</h2></div>
                    <button type="button" data-modal-close aria-label="${esc(t('game_maker.close'))}">×</button></header>
                <div class="gm-revision-list">${(body.revisions || []).map(revision => `<article>
                    <div><strong>v${revision.number}</strong><span>${esc(revision.source)}${esc(formatRevisionTime(revision.created_at))}</span></div>
                    <p>${esc(revision.summary)}</p><small>${revision.file_count} ${esc(t('game_maker.files'))}</small>
                    <button type="button" data-restore="${revision.number}"
                        ${revision.number === state.project.current_revision ? 'disabled' : ''}>${esc(t('game_maker.restore'))}</button>
                </article>`).join('') || `<div class="gm-library-empty">${esc(t('game_maker.no_revisions'))}</div>`}</div>
                <footer><button type="button" data-modal-close>${esc(t('game_maker.close'))}</button></footer>
            </section>`, layer => {
                layer.querySelectorAll('[data-restore]').forEach(button => button.addEventListener('click', async () => {
                    const confirmed = await helpers.confirmAction(state, t('game_maker.restore_title'), t('game_maker.restore_confirm'));
                    if (!confirmed) return;
                    try {
                        helpers.setModalBusy(layer, true);
                        await state.api.restore(state.project.id, Number(button.dataset.restore));
                        helpers.closeModal(state);
                        await state.reloadProjectRecord();
                        await state.refreshPreview();
                    } catch (error) {
                        helpers.setModalBusy(layer, false);
                        helpers.modalError(layer, error.message || String(error));
                    }
                }));
            });
        } catch (error) {
            state.fail(error);
        }
    }

    window.GameMakerStudioModals = { showSkillsModal, showRevisionsModal };
})();
