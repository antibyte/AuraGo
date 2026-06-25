    function renderGitPanel() {
        const panel = shellPart('[data-git-panel]');
        if (!panel) return;
        if (!state.gitVisible) {
            panel.hidden = true;
            return;
        }
        panel.hidden = false;
        const branchHtml = state.gitBranch
            ? `<div class="cs-git-branch"><svg viewBox="0 0 16 16" width="14" height="14" fill="currentColor"><path d="M11.75 2.5a.75.75 0 100 1.5.75.75 0 000-1.5zm-2.25.75a2.25 2.25 0 113 2.122V6.5a2 2 0 01-2 2H7.5v1.128a2.251 2.251 0 11-1.5 0V5.372a2.25 2.25 0 111.5 0v1.878h3a.5.5 0 00.5-.5v-1.128A2.251 2.251 0 019.5 3.25zM4.25 3.5a.75.75 0 100 1.5.75.75 0 000-1.5zM4.25 12a.75.75 0 100 1.5.75.75 0 000-1.5z"/></svg><span>${esc(state.gitBranch)}</span></div>`
            : `<div class="cs-git-branch">${esc(tr('codeStudio.noRepo', 'No git repository'))}</div>`;

        const changesHtml = state.gitChanges.length
            ? state.gitChanges.map(change => {
                const statusClass = change.status === '??' ? 'untracked' : change.status === 'M' ? 'modified' : change.status === 'A' ? 'added' : change.status === 'D' ? 'deleted' : 'renamed';
                const statusLabel = change.status === '??' ? 'U' : change.status;
                return `<button type="button" class="cs-git-change" data-git-file="${esc(change.path)}" title="${esc(change.path)}">
                    <span class="cs-git-status ${statusClass}">${esc(statusLabel)}</span>
                    <span class="cs-git-filename">${esc(change.path.split('/').pop())}</span>
                </button>`;
            }).join('')
            : `<div class="cs-git-empty">${esc(tr('codeStudio.noChanges', 'No changes'))}</div>`;

        const logHtml = state.gitLog.length
            ? state.gitLog.slice(0, 10).map(entry => `<div class="cs-git-log-entry"><span class="cs-git-hash">${esc(entry.hash)}</span> <span>${esc(entry.message)}</span></div>`).join('')
            : '';

        panel.innerHTML = `<div class="cs-git-head">
            <strong>${esc(tr('codeStudio.gitPanel', 'Source Control'))}</strong>
            <button type="button" class="cs-icon-button" data-git-refresh title="${esc(tr('codeStudio.refresh', 'Refresh'))}">${iconMarkup('refresh', 'R', 'cs-icon-button-icon', 14)}</button>
            <button type="button" class="cs-icon-button" data-git-close title="${esc(tr('codeStudio.closeTab', 'Close'))}">${iconMarkup('x', 'X', 'cs-icon-button-icon', 14)}</button>
        </div>
        ${branchHtml}
        <div class="cs-git-section">
            <div class="cs-git-section-head">${esc(tr('codeStudio.changes', 'Changes'))} <span class="cs-git-count">${state.gitChanges.length}</span></div>
            <div class="cs-git-changes">${changesHtml}</div>
        </div>
        <div class="cs-git-commit-area">
            <textarea class="cs-git-commit-msg" data-git-commit-msg placeholder="${esc(tr('codeStudio.commitMessage', 'Commit message...'))}" rows="3"></textarea>
            <div class="cs-git-commit-actions">
                <button type="button" class="cs-button primary" data-git-commit>${esc(tr('codeStudio.commit', 'Commit'))}</button>
            </div>
        </div>
        ${logHtml ? `<div class="cs-git-section"><div class="cs-git-section-head">${esc(tr('codeStudio.gitLog', 'Recent Commits'))}</div><div class="cs-git-log">${logHtml}</div></div>` : ''}`;

        panel.querySelector('[data-git-close]').addEventListener('click', bind(toggleGitPanel));
        panel.querySelector('[data-git-refresh]').addEventListener('click', bind(refreshGitStatus));
        panel.querySelector('[data-git-commit]').addEventListener('click', bind(commitGitChanges));
        panel.querySelectorAll('[data-git-file]').forEach(btn => {
            btn.addEventListener('click', bind(() => openGitDiff(btn.dataset.gitFile)));
        });
    }

    function toggleGitPanel() {
        state.gitVisible = !state.gitVisible;
        const root = ensureShellRoot();
        if (root) root.dataset.git = state.gitVisible ? 'visible' : 'hidden';
        if (state.gitVisible) refreshGitStatus();
        renderActivityBar();
        renderWindowMenus();
    }

    async function refreshGitStatus() {
        const target = state;
        if (!isLiveInstance(target)) return;
        try {
            const result = await apiClient.gitStatus();
            if (!isLiveInstance(target)) return;
            runWithInstance(target, () => {
                state.gitBranch = result.branch || '';
                state.gitChanges = result.changes || [];
                state.gitLog = result.log || [];
                renderGitPanel();
                renderActivityBar();
            });
        } catch (err) {
            if (isLiveInstance(target)) {
                runWithInstance(target, () => {
                    state.gitBranch = '';
                    state.gitChanges = [];
                    state.gitLog = [];
                    renderGitPanel();
                });
            }
        }
    }

    async function openGitDiff(filePath) {
        const target = state;
        if (!isLiveInstance(target)) return;
        try {
            const result = await apiClient.gitDiff(filePath, false);
            if (!isLiveInstance(target)) return;
            runWithInstance(target, () => {
                const diffLines = (result.diff || '').split('\n');
                const diffHtml = diffLines.map(line => {
                    if (line.startsWith('+') && !line.startsWith('+++')) return `<span class="cs-diff-line added">${esc(line)}</span>`;
                    if (line.startsWith('-') && !line.startsWith('---')) return `<span class="cs-diff-line removed">${esc(line)}</span>`;
                    if (line.startsWith('@@')) return `<span class="cs-diff-line context">${esc(line)}</span>`;
                    return `<span class="cs-diff-line">${esc(line)}</span>`;
                }).join('');
                const editor = shellPart('[data-editor]');
                if (editor) {
                    editor.innerHTML = `<div class="cs-diff-view">
                        <div class="cs-diff-view-head">
                            <strong>${esc(filePath.split('/').pop())}</strong>
                            <span>${esc(tr('codeStudio.gitDiff', 'Git Diff'))}</span>
                        </div>
                        <div class="cs-diff-content">${diffHtml}</div>
                    </div>`;
                }
            });
        } catch (err) {
            if (isLiveInstance(target)) runWithInstance(target, () => renderStatus(err.message || String(err)));
        }
    }

    async function commitGitChanges() {
        const target = state;
        if (!isLiveInstance(target)) return;
        const msgInput = shellPart('[data-git-commit-msg]');
        const message = msgInput ? msgInput.value.trim() : '';
        if (!message) {
            renderStatus(tr('codeStudio.commitMessage', 'Commit message...'));
            return;
        }
        try {
            const result = await apiClient.gitCommit(message, true);
            if (!isLiveInstance(target)) return;
            runWithInstance(target, () => {
                renderStatus(tr('codeStudio.committed', 'Committed') + ': ' + (result.hash || '').slice(0, 7));
                if (msgInput) msgInput.value = '';
                refreshGitStatus();
            });
        } catch (err) {
            if (isLiveInstance(target)) runWithInstance(target, () => renderStatus(err.message || String(err)));
        }
    }
