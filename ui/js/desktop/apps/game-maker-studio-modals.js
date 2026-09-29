(function () {
    'use strict';

    // Shared modal lifecycle, plus skills and revisions for Game Maker Studio.

    const skillStatuses = ['ready', 'installed', 'updated', 'verified', 'disabled', 'missing', 'hash_mismatch',
        'warning', 'dangerous', 'pending', 'error'];

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
        const projectID = state.project.id;
        const openedRevision = Number(state.project.current_revision) || 0;
        const { esc, t } = state.context;
        try {
            const body = await state.api.revisions(projectID);
            if (state.disposed || state.project?.id !== projectID) return;
            const restoreAllowed = Boolean(state.capabilities?.allow_edit);
            const readonlyNotice = t('game_maker.readonly_notice');
            helpers.showModal(state, `<section class="gm-modal gm-revisions-modal">
                <header><div><span>${esc(t('game_maker.history'))}</span><h2>${esc(t('game_maker.revisions_title'))}</h2></div>
                    <button type="button" data-modal-close aria-label="${esc(t('game_maker.close'))}">×</button></header>
                ${restoreAllowed ? '' : `<p class="gm-readonly-notice" role="status">${esc(readonlyNotice)}</p>`}
                <div class="gm-revision-list">${(body.revisions || []).map(revision => `<article>
                    <div><strong>v${revision.number}</strong><span>${esc(revision.source)}${esc(formatRevisionTime(revision.created_at))}</span></div>
                    <p>${esc(revision.summary)}</p><small>${revision.file_count} ${esc(t('game_maker.files'))}</small>
                    <button type="button" data-restore="${revision.number}"
                        ${revision.number === state.project.current_revision || !restoreAllowed ? 'disabled' : ''}
                        ${restoreAllowed ? '' : `title="${esc(readonlyNotice)}" aria-label="${esc(t('game_maker.restore'))}: ${esc(readonlyNotice)}"`}>${esc(t('game_maker.restore'))}</button>
                </article>`).join('') || `<div class="gm-library-empty">${esc(t('game_maker.no_revisions'))}</div>`}</div>
                <footer><button type="button" data-modal-close>${esc(t('game_maker.close'))}</button></footer>
            </section>`, layer => {
                layer.querySelectorAll('[data-restore]').forEach(button => button.addEventListener('click', async () => {
                    const revision = Number(button.dataset.restore);
                    if (!state.capabilities?.allow_edit || state.project?.id !== projectID
                        || Number(state.project.current_revision) !== openedRevision || revision === openedRevision) return;
                    const confirmed = await helpers.confirmAction(state, t('game_maker.restore_title'), t('game_maker.restore_confirm'));
                    if (!confirmed || state.project?.id !== projectID || Number(state.project.current_revision) !== openedRevision
                        || !state.capabilities?.allow_edit) return;
                    if (state.preparePreviewReplacement && !await state.preparePreviewReplacement()) return;
                    if (state.project?.id !== projectID || Number(state.project.current_revision) !== openedRevision) return;
                    let restored = false;
                    try {
                        helpers.setModalBusy(layer, true);
                        state.revisionMutationInProgress = projectID;
                        await state.api.restore(projectID, revision);
                        restored = true;
                        if (state.disposed || state.project?.id !== projectID) return;
                        helpers.closeModal(state);
                        if (!await state.reloadProjectRecord()) return;
                        await state.refreshPreview({ skipFlush: true });
                    } catch (error) {
                        if (restored) state.fail?.(error);
                        else {
                            helpers.setModalBusy(layer, false);
                            helpers.modalError(layer, error.message || String(error));
                        }
                    } finally {
                        if (state.revisionMutationInProgress === projectID) state.revisionMutationInProgress = null;
                    }
                }));
            });
        } catch (error) {
            state.fail(error);
        }
    }

    function showModal(state, html, mount) {
        if (state.assetBrowserCleanup) state.assetBrowserCleanup();
        const layer = state.container.querySelector('[data-gm-modal]');
        layer.hidden = false;
        layer.innerHTML = html;
        layer.querySelectorAll('[data-modal-close]').forEach(button =>
            button.addEventListener('click', () => closeModal(state)));
        layer.addEventListener('click', state.modalBackdrop = event => {
            if (event.target === layer) closeModal(state);
        }, { once: true });
        if (mount) mount(layer);
    }

    function closeModal(state) {
        if (state.assetBrowserCleanup) state.assetBrowserCleanup();
        const layer = state.container.querySelector('[data-gm-modal]');
        if (!layer) return;
        layer.hidden = true;
        layer.replaceChildren();
    }

    function setModalBusy(layer, busy) {
        layer.querySelectorAll('button,input,textarea,select').forEach(control => { control.disabled = busy; });
    }

    function modalError(layer, message) {
        const modal = layer.querySelector('.gm-modal');
        if (!modal) return;
        let error = layer.querySelector('.gm-modal-error');
        if (!error) {
            error = document.createElement('p');
            error.className = 'gm-modal-error';
            error.setAttribute('role', 'alert');
            // The pinned footer stays the last child; the message sits above it.
            const footer = [...modal.children].reverse().find(child => child.tagName === 'FOOTER');
            modal.insertBefore(error, footer || null);
        }
        error.textContent = message;
        if (error.scrollIntoView) error.scrollIntoView({ block: 'nearest' });
    }

    function confirmAction(state, title, message) {
        if (typeof state.context.confirmDialog === 'function') {
            return Promise.resolve(state.context.confirmDialog(title, message));
        }
        return Promise.resolve(false);
    }

    function mediaToggle(state, name, capability, label) {
        const { esc, t } = state.context;
        const available = Boolean(state.capabilities[capability]);
        return `<label class="gm-media-toggle ${available ? '' : 'is-disabled'}">
            <input type="checkbox" name="${name}" ${available ? 'checked' : 'disabled'}>
            <span><strong>${esc(t('game_maker.' + label))}</strong>
            <small>${esc(t(available ? 'game_maker.media_auto' : 'game_maker.media_unavailable'))}</small></span>
        </label>`;
    }

    window.GameMakerStudioModals = { showSkillsModal, showRevisionsModal, showModal, closeModal, setModalBusy, modalError, confirmAction, mediaToggle };
})();
