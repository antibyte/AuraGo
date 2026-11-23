    // Source Control panel of Code Studio: staged and unstaged groups, commit, repository init
    // and the branch menu. History, the commit tab and the blame gutter live in git-history.js.

    function gitBranchIconMarkup(size) {
        const pixels = Number(size) || 14;
        return `<svg viewBox="0 0 16 16" width="${pixels}" height="${pixels}" fill="currentColor" aria-hidden="true" focusable="false"><path d="M11.75 2.5a.75.75 0 100 1.5.75.75 0 000-1.5zm-2.25.75a2.25 2.25 0 113 2.122V6.5a2 2 0 01-2 2H7.5v1.128a2.251 2.251 0 11-1.5 0V5.372a2.25 2.25 0 111.5 0v1.878h3a.5.5 0 00.5-.5v-1.128A2.251 2.251 0 019.5 3.25zM4.25 3.5a.75.75 0 100 1.5.75.75 0 000-1.5zM4.25 12a.75.75 0 100 1.5.75.75 0 000-1.5z"/></svg>`;
    }

    function gitChangeIsStaged(change) {
        return change.staged === true || change.staged === 'true';
    }

    // One status entry with the porcelain index (X) and worktree (Y) characters. Entries
    // without them (older servers, test fixtures) are derived from status and staged.
    function normalizeGitChange(raw) {
        const change = raw || {};
        const status = String(change.status || '');
        let index = typeof change.index === 'string' && change.index.length === 1 ? change.index : '';
        let worktree = typeof change.worktree === 'string' && change.worktree.length === 1 ? change.worktree : '';
        if (!index || !worktree) {
            if (status === '??') {
                index = '?';
                worktree = '?';
            } else if (gitChangeIsStaged(change)) {
                index = status.charAt(0) || 'M';
                worktree = ' ';
            } else {
                index = ' ';
                worktree = status.charAt(0) || 'M';
            }
        }
        return { path: String(change.path || ''), status, index, worktree };
    }

    // A file that is staged and changed again in the work tree is listed in both groups.
    function gitChangeGroups() {
        const staged = [];
        const changes = [];
        (state.gitChanges || []).forEach(raw => {
            const change = normalizeGitChange(raw);
            if (!change.path) return;
            if (change.index !== ' ' && change.index !== '?') staged.push(change);
            if (change.worktree !== ' ') changes.push(change);
        });
        return { staged, changes };
    }

    function gitChangeUntracked(change) {
        return change.index === '?' && change.worktree === '?';
    }

    function gitStatusCode(change, staged) {
        if (gitChangeUntracked(change)) return 'U';
        return staged ? change.index : change.worktree;
    }

    function gitStatusClass(code) {
        if (code === 'U' || code === '??') return 'untracked';
        if (code === 'M') return 'modified';
        if (code === 'A') return 'added';
        if (code === 'D') return 'deleted';
        return 'renamed';
    }

    function gitRowButton(attribute, value, label, glyph) {
        const valueHtml = value === null ? '' : `="${esc(value)}"`;
        return `<button type="button" class="cs-git-row-action" ${attribute}${valueHtml} data-git-write title="${esc(label)}" aria-label="${esc(label)}">${esc(glyph)}</button>`;
    }

    function gitChangeRowHtml(change, staged) {
        const code = gitStatusCode(change, staged);
        const trimmed = change.path.replace(/\/+$/, '');
        const slash = trimmed.lastIndexOf('/');
        const name = trimmed.slice(slash + 1) + (trimmed.length < change.path.length ? '/' : '');
        const dir = slash > 0 ? trimmed.slice(0, slash) : '';
        const actions = staged
            ? gitRowButton('data-git-unstage', change.path, tr('codeStudio.gitUnstage', 'Unstage Changes'), '−')
            : gitRowButton('data-git-discard', change.path, tr('codeStudio.discardChanges', 'Discard Changes'), '↶')
                + gitRowButton('data-git-stage', change.path, tr('codeStudio.gitStage', 'Stage Changes'), '+');
        return `<div class="cs-git-change-row">
                <button type="button" class="cs-git-change" data-git-file="${esc(change.path)}" data-git-staged="${staged ? 'true' : 'false'}" title="${esc(change.path)}">
                    <span class="cs-git-status ${gitStatusClass(code)}">${esc(code)}</span>
                    <span class="cs-git-filename">${esc(name)}</span>
                    ${dir ? `<span class="cs-git-dir">${esc(dir)}</span>` : ''}
                </button>
                <span class="cs-git-row-actions">${actions}</span>
            </div>`;
    }

    function gitGroupHtml(kind, title, changes, staged) {
        const bulk = staged
            ? gitRowButton('data-git-unstage-all', null, tr('codeStudio.gitUnstageAll', 'Unstage All Changes'), '−')
            : gitRowButton('data-git-stage-all', null, tr('codeStudio.gitStageAll', 'Stage All Changes'), '+');
        const rows = changes.length
            ? changes.map(change => gitChangeRowHtml(change, staged)).join('')
            : `<div class="cs-git-empty">${esc(tr('codeStudio.noChanges', 'No changes'))}</div>`;
        return `<div class="cs-git-section" data-git-section="${kind}">
                <div class="cs-git-section-head"><span class="cs-git-section-title">${esc(title)}</span><span class="cs-git-count">${changes.length}</span>${changes.length ? `<span class="cs-git-section-actions">${bulk}</span>` : ''}</div>
                <div class="cs-git-changes">${rows}</div>
            </div>`;
    }

    function renderGitPanel() {
        const panel = shellPart('[data-git-panel]');
        if (!panel) return;
        if (!state.gitVisible) {
            panel.hidden = true;
            return;
        }
        panel.hidden = false;
        // A refresh re-renders the panel: the commit message keeps focus and caret, the list its scroll position.
        const focused = document.activeElement;
        const keepFocus = !!focused && panel.contains(focused) && focused.matches('[data-git-commit-msg]');
        const caret = keepFocus ? [focused.selectionStart, focused.selectionEnd] : null;
        const scrollTop = panel.scrollTop;
        const head = `<div class="cs-git-head">
            <strong>${esc(tr('codeStudio.gitPanel', 'Source Control'))}</strong>
            <button type="button" class="cs-icon-button" data-git-refresh title="${esc(tr('codeStudio.refresh', 'Refresh'))}">${iconMarkup('refresh', 'R', 'cs-icon-button-icon', 14)}</button>
            <button type="button" class="cs-icon-button" data-git-close title="${esc(tr('codeStudio.closeTab', 'Close'))}">${iconMarkup('x', 'X', 'cs-icon-button-icon', 14)}</button>
        </div>`;
        if (state.gitIsRepo === false) {
            panel.innerHTML = `${head}
                <div class="cs-git-init">
                    <p>${esc(tr('codeStudio.noRepo', 'No git repository'))}</p>
                    <button type="button" class="cs-button primary" data-git-init data-git-write>${esc(tr('codeStudio.gitInitRepo', 'Initialize Repository'))}</button>
                </div>`;
        } else {
            const groups = gitChangeGroups();
            const branchHtml = state.gitIsRepo === true
                ? `<button type="button" class="cs-git-branch" data-git-branch title="${esc(tr('codeStudio.gitBranchMenu', 'Switch, create or delete branches'))}">${gitBranchIconMarkup(14)}<span>${esc(state.gitBranch || tr('codeStudio.gitDetached', 'Detached HEAD'))}</span></button>`
                : '';
            const identityHtml = state.gitIdentityPrompt ? `<form class="cs-git-identity" data-git-identity>
                    <p>${esc(tr('codeStudio.gitIdentityHint', 'Git needs your name and email for this repository.'))}</p>
                    <input name="name" value="${esc(state.gitIdentityName || '')}" placeholder="${esc(tr('codeStudio.identityName', 'Your name'))}" autocomplete="name" maxlength="200">
                    <input name="email" type="email" value="${esc(state.gitIdentityEmail || '')}" placeholder="${esc(tr('codeStudio.identityEmail', 'Your email'))}" autocomplete="email" maxlength="200">
                    <button type="submit" class="cs-button">${esc(tr('codeStudio.saveIdentity', 'Save identity'))}</button>
                </form>` : '';
            panel.innerHTML = `${head}
                ${branchHtml}
                <div class="cs-git-commit-area">
                    <textarea class="cs-git-commit-msg" data-git-commit-msg placeholder="${esc(tr('codeStudio.commitMessage', 'Commit message...'))}" rows="3">${esc(state.gitCommitMessage || '')}</textarea>
                    <div class="cs-git-commit-actions">
                        <button type="button" class="cs-button primary" data-git-commit>${esc(tr('codeStudio.commit', 'Commit'))}</button>
                    </div>
                    ${identityHtml}
                </div>
                ${groups.staged.length ? gitGroupHtml('staged', tr('codeStudio.gitStagedChanges', 'Staged Changes'), groups.staged, true) : ''}
                ${gitGroupHtml('changes', tr('codeStudio.changes', 'Changes'), groups.changes, false)}
                ${state.gitIsRepo === true ? gitHistorySectionHtml() : ''}`;
        }
        wireGitPanel(panel);
        panel.scrollTop = scrollTop;
        const message = panel.querySelector('[data-git-commit-msg]');
        if (keepFocus && message) {
            message.focus();
            message.setSelectionRange(caret[0], caret[1]);
        }
        updatePolicyControls();
    }

    function wireGitPanel(panel) {
        const onClick = (selector, handler) => {
            const element = panel.querySelector(selector);
            if (element) element.addEventListener('click', bind(handler));
        };
        onClick('[data-git-close]', toggleGitPanel);
        onClick('[data-git-refresh]', refreshGitStatus);
        onClick('[data-git-init]', initGitRepository);
        onClick('[data-git-commit]', commitGitChanges);
        onClick('[data-git-stage-all]', () => stageGitPaths(gitChangeGroups().changes.map(change => change.path)));
        onClick('[data-git-unstage-all]', () => unstageGitPaths(gitChangeGroups().staged.map(change => change.path)));
        const branch = panel.querySelector('[data-git-branch]');
        if (branch) branch.addEventListener('click', bind(() => {
            const rect = branch.getBoundingClientRect();
            showGitBranchMenu(rect.left, rect.bottom + 2);
        }));
        const messageBox = panel.querySelector('[data-git-commit-msg]');
        if (messageBox) messageBox.addEventListener('input', bind(() => {
            state.gitCommitMessage = messageBox.value;
        }));
        const identityForm = panel.querySelector('[data-git-identity]');
        if (identityForm) identityForm.addEventListener('submit', bind(event => {
            event.preventDefault();
            saveGitIdentity(new FormData(event.currentTarget));
        }));
        panel.querySelectorAll('[data-git-file]').forEach(btn => {
            btn.addEventListener('click', bind(() => openGitDiff(btn.dataset.gitFile, btn.dataset.gitStaged === 'true')));
        });
        panel.querySelectorAll('[data-git-stage]').forEach(btn => {
            btn.addEventListener('click', bind(() => stageGitPaths([btn.dataset.gitStage])));
        });
        panel.querySelectorAll('[data-git-unstage]').forEach(btn => {
            btn.addEventListener('click', bind(() => unstageGitPaths([btn.dataset.gitUnstage])));
        });
        panel.querySelectorAll('[data-git-discard]').forEach(btn => {
            btn.addEventListener('click', bind(() => discardGitChange(btn.dataset.gitDiscard)));
        });
        wireGitHistorySection(panel);
    }

    function toggleGitPanel() {
        state.gitVisible = !state.gitVisible;
        const root = ensureShellRoot();
        if (root) root.dataset.git = state.gitVisible ? 'visible' : 'hidden';
        if (state.gitVisible) refreshGitStatus();
        renderActivityBar();
        renderWindowMenus();
        scheduleTerminalFit();
    }

    function scheduleGitRefresh() {
        const instance = state;
        clearTimeout(instance.gitRefreshTimer);
        instance.gitRefreshTimer = setTimeout(bindInstance(instance, () => {
            instance.gitRefreshTimer = null;
            refreshGitStatus();
        }), 200);
    }

    async function refreshGitStatus() {
        const target = state;
        if (!isLiveInstance(target)) return;
        const request = target.gitRequest = Symbol();
        try {
            const result = await apiClient.gitStatus();
            if (!isLiveInstance(target) || target.gitRequest !== request) return;
            runWithInstance(target, () => {
                state.gitIsRepo = result.is_repo !== false;
                state.gitBranch = result.branch || '';
                state.gitChanges = Array.isArray(result.changes) ? result.changes : [];
                state.gitLog = Array.isArray(result.log) ? result.log : [];
                renderGitPanel();
                renderActivityBar();
                renderStatus();
                if (!state.gitIsRepo) return;
                refreshGitHistory();
                // A refresh follows every save, stage, commit and checkout: blame is read again
                // for each tab that shows it and has no unsaved edits.
                state.openTabs.forEach(tab => {
                    if (tab.blameOn && !tab.modified) refreshGitBlame(tab);
                });
            });
        } catch (err) {
            if (isLiveInstance(target) && target.gitRequest === request) {
                runWithInstance(target, () => {
                    state.gitIsRepo = null;
                    state.gitBranch = '';
                    state.gitChanges = [];
                    state.gitLog = [];
                    renderGitPanel();
                    renderStatus();
                });
            }
        }
    }

    async function openGitDiff(filePath, staged = false) {
        const target = state;
        if (!isLiveInstance(target)) return;
        let tab = target.openTabs.find(item => item.kind === 'diff' && item.gitPath === filePath);
        if (!tab) {
            tab = { kind: 'diff', gitPath: filePath, path: desktopToCodeStudioPath(filePath), content: '', modified: false, staged };
            target.openTabs.push(tab);
        }
        tab.staged = staged;
        tab.content = '';
        tab.loading = true;
        activateTab(target.openTabs.indexOf(tab));
        const request = tab.diffRequest = Symbol();
        try {
            const result = await apiClient.gitDiff(filePath, staged);
            if (!isLiveInstance(target) || !target.openTabs.includes(tab) || tab.diffRequest !== request) return;
            runWithInstance(target, () => {
                tab.content = result.diff || '';
                tab.loading = false;
                if (activeTab() === tab) renderEditor();
            });
        } catch (err) {
            if (!isLiveInstance(target) || !target.openTabs.includes(tab) || tab.diffRequest !== request) return;
            runWithInstance(target, () => {
                tab.loading = false;
                tab.content = err.message || String(err);
                if (activeTab() === tab) renderEditor();
            });
        }
    }

    // The diff renderer of the diff tab and of the commit tab.
    function diffLinesHtml(text) {
        return String(text || '').split('\n').map(line => {
            const style = line.startsWith('+') && !line.startsWith('+++') ? 'added' : line.startsWith('-') && !line.startsWith('---') ? 'removed' : line.startsWith('@@') ? 'context' : '';
            return `<span class="cs-diff-line ${style}">${esc(line)}</span>`;
        }).join('');
    }

    function renderGitDiffTab(editor, tab) {
        editor.innerHTML = `<div class="cs-diff-view">
            <div class="cs-diff-view-head">
                <span class="cs-diff-view-title"><strong>${esc(baseName(tab.path))}</strong><span class="cs-diff-badge">${esc(tr('codeStudio.gitDiff', 'Git Diff'))} · ${esc(tr('codeStudio.readOnly', 'Read-only'))}</span></span>
                <span class="cs-diff-view-spacer"></span>
                <select data-diff-source aria-label="${esc(tr('codeStudio.gitDiff', 'Git Diff'))}"><option value="worktree"${tab.staged ? '' : ' selected'}>${esc(tr('codeStudio.diffWorktree', 'Working tree'))}</option><option value="index"${tab.staged ? ' selected' : ''}>${esc(tr('codeStudio.diffIndex', 'Index'))}</option></select>
                <button type="button" class="cs-icon-button" data-diff-close aria-label="${esc(tr('desktop.close', 'Close'))}">×</button>
            </div>
            <div class="cs-diff-content">${tab.loading ? esc(tr('codeStudio.starting', 'Starting...')) : tab.content ? diffLinesHtml(tab.content) : esc(tr('codeStudio.noChanges', 'No changes'))}</div>
        </div>`;
        editor.querySelector('[data-diff-close]').addEventListener('click', bind(() => closeTab(state.openTabs.indexOf(tab))));
        editor.querySelector('[data-diff-source]').addEventListener('change', bind(event => openGitDiff(tab.gitPath, event.target.value === 'index')));
    }

    async function commitGitChanges() {
        const target = state;
        if (!canWrite(target)) return;
        const msgInput = shellPart('[data-git-commit-msg]');
        const message = (msgInput ? msgInput.value : state.gitCommitMessage || '').trim();
        state.gitCommitMessage = message;
        if (!message) {
            renderStatus(tr('codeStudio.commitMessage', 'Commit message...'));
            return;
        }
        const groups = gitChangeGroups();
        if (!groups.staged.length && !groups.changes.length) {
            flashStatus(tr('codeStudio.noChanges', 'No changes'));
            return;
        }
        // Commit takes the index as it is. Only with nothing staged does it offer to stage everything first.
        let addAll = false;
        if (!groups.staged.length) {
            const confirmed = await confirmValue(tr('codeStudio.gitStageAllAndCommit', 'Stage all changes and commit?'), { confirmLabel: tr('codeStudio.commit', 'Commit'), confirmIcon: 'check-square' });
            if (!confirmed || !canWrite(target)) return;
            addAll = true;
        }
        try {
            const result = await apiClient.gitCommit(message, { addAll });
            if (!canWrite(target)) return;
            runWithInstance(target, () => {
                flashStatus(tr('codeStudio.committed', 'Committed') + ': ' + (result.hash || '').slice(0, 7), 4000);
                state.gitCommitMessage = '';
                state.gitIdentityPrompt = false;
                const box = shellPart('[data-git-commit-msg]');
                if (box) box.value = '';
                refreshGitStatus();
            });
        } catch (err) {
            if (!canWrite(target)) return;
            runWithInstance(target, () => {
                if (err && err.status === 409 && err.message === 'git_identity_required') {
                    state.gitIdentityPrompt = true;
                    renderGitPanel();
                    return;
                }
                showGitError(err);
            });
        }
    }

    async function saveGitIdentity(formData) {
        const target = state;
        if (!canWrite(target)) return;
        const name = String(formData.get('name') || '').trim();
        const email = String(formData.get('email') || '').trim();
        state.gitIdentityName = name;
        state.gitIdentityEmail = email;
        if (!name || !email) {
            renderStatus(tr('codeStudio.gitIdentityHint', 'Git needs your name and email for this repository.'));
            return;
        }
        try {
            await apiClient.gitIdentity(name, email);
            if (!canWrite(target)) return;
            runWithInstance(target, () => {
                state.gitIdentityPrompt = false;
                renderGitPanel();
                renderStatus(tr('codeStudio.saveIdentity', 'Save identity'));
            });
        } catch (err) {
            if (isLiveInstance(target)) runWithInstance(target, () => renderStatus(err.message || String(err)));
        }
    }

    async function runGitPathAction(action, paths) {
        const target = state;
        const list = (paths || []).filter(Boolean);
        if (!canWrite(target) || !list.length) return false;
        const request = action === 'stage' ? apiClient.gitStage : action === 'unstage' ? apiClient.gitUnstage : apiClient.gitDiscard;
        try {
            await request(list);
        } catch (err) {
            if (isLiveInstance(target)) runWithInstance(target, () => {
                showGitError(err);
                refreshGitStatus();
            });
            return false;
        }
        if (!isLiveInstance(target)) return false;
        runWithInstance(target, () => {
            refreshGitStatus();
            if (action === 'discard') refreshGitWorkspaceView();
        });
        return true;
    }

    function stageGitPaths(paths) {
        return runGitPathAction('stage', paths);
    }

    function unstageGitPaths(paths) {
        return runGitPathAction('unstage', paths);
    }

    async function discardGitChange(path) {
        const target = state;
        if (!canWrite(target) || !path) return;
        const change = gitChangeGroups().changes.find(item => item.path === path);
        const message = change && gitChangeUntracked(change)
            ? tr('codeStudio.gitDiscardUntrackedConfirm', 'The untracked file {{name}} will be deleted. This cannot be undone.', { name: path })
            : tr('codeStudio.gitDiscardConfirm', 'Discard the changes in {{name}}? This cannot be undone.', { name: path });
        const confirmed = await confirmValue(message, { confirmLabel: tr('codeStudio.discard', 'Discard'), confirmIcon: 'x' });
        if (!confirmed || !canWrite(target)) return;
        await runWithInstance(target, () => runGitPathAction('discard', [path]));
    }

    async function initGitRepository() {
        const target = state;
        if (!canWrite(target)) return;
        try {
            await apiClient.gitInit();
        } catch (err) {
            if (isLiveInstance(target)) runWithInstance(target, () => {
                showGitError(err);
                refreshGitStatus();
            });
            return;
        }
        if (!isLiveInstance(target)) return;
        runWithInstance(target, () => {
            flashStatus(tr('codeStudio.gitInitialized', 'Initialized an empty Git repository'), 4000);
            refreshGitStatus();
        });
    }

    // Git's own message is shown verbatim: in full in the Output panel, its first line in the status bar.
    function showGitError(err) {
        const message = String(err && err.message ? err.message : err || '').trim();
        if (!message) return;
        appendOutput(message, 'git');
        setActivePanel('output');
        flashStatus(message.split('\n')[0], 8000);
    }

    // After a checkout or a discard the files on disk changed. Clean tabs read their file
    // again; reloadCleanTabFromDisk leaves tabs with unsaved edits alone. The tree is listed again.
    async function refreshGitWorkspaceView() {
        const target = state;
        if (!isLiveInstance(target)) return;
        target.openTabs.forEach(tab => { reloadCleanTabFromDisk(target, tab); });
        await runAsyncStep(target, () => refreshFiles(target.currentPath));
        for (const dir of [...target.expandedDirs]) {
            if (!isLiveInstance(target)) return;
            if (dir === target.currentPath) continue;
            try {
                const result = await apiClient.files(dir);
                if (!isLiveInstance(target)) return;
                target.treeCache[dir] = sortTreeEntries(result.files || []);
            } catch (_) {
                // The folder does not exist on this branch.
                delete target.treeCache[dir];
                target.expandedDirs.delete(dir);
            }
        }
        if (isLiveInstance(target)) runWithInstance(target, renderSidebar);
    }

    async function showGitBranchMenu(x, y) {
        const target = state;
        if (!isLiveInstance(target)) return;
        let result;
        try {
            result = await apiClient.gitBranch();
        } catch (err) {
            if (isLiveInstance(target)) runWithInstance(target, () => showGitError(err));
            return;
        }
        if (!isLiveInstance(target)) return;
        runWithInstance(target, () => {
            const current = String(result && result.current || '');
            const local = (result && Array.isArray(result.branches) ? result.branches : [])
                .map(name => String(name || '').trim())
                .filter(name => name && !name.startsWith('remotes/') && !name.startsWith('('));
            const writable = canWrite();
            const items = local.map((name, index) => ({
                id: 'git-branch-switch-' + index, label: name, icon: 'network', checked: name === current,
                disabled: !writable || name === current, action: bind(() => changeGitBranch(name, 'switch'))
            }));
            if (items.length) items.push({ separator: true });
            items.push({ id: 'git-branch-create', label: tr('codeStudio.gitCreateBranch', 'Create new branch...'), icon: 'file-plus', disabled: !writable, action: bind(createGitBranch) });
            // The current branch is never offered for deletion.
            const removable = local.filter(name => name !== current);
            if (removable.length) items.push({ separator: true });
            removable.forEach((name, index) => items.push({
                id: 'git-branch-delete-' + index, label: tr('codeStudio.gitDeleteBranch', 'Delete branch {{name}}', { name }), icon: 'trash',
                disabled: !writable, action: bind(() => deleteGitBranch(name))
            }));
            showStudioContextMenu(x, y, items);
        });
    }

    async function changeGitBranch(name, action) {
        const target = state;
        if (!canWrite(target) || !name) return false;
        try {
            const result = await apiClient.gitBranch(name, action);
            if (!isLiveInstance(target)) return false;
            runWithInstance(target, () => {
                const message = action === 'delete'
                    ? tr('codeStudio.gitBranchDeleted', 'Deleted branch {{name}}', { name })
                    : tr('codeStudio.gitSwitchedBranch', 'Switched to {{name}}', { name: result && result.current || name });
                flashStatus(message, 4000);
                refreshGitStatus();
                if (action !== 'delete') refreshGitWorkspaceView();
            });
            return true;
        } catch (err) {
            // Checkout errors such as "local changes would be overwritten" are shown verbatim.
            if (isLiveInstance(target)) runWithInstance(target, () => showGitError(err));
            return false;
        }
    }

    async function createGitBranch() {
        const target = state;
        if (!canWrite(target)) return;
        const name = String(await promptValue(tr('codeStudio.gitNewBranchName', 'New branch name'), '') || '').trim();
        if (!name || !canWrite(target)) return;
        await runWithInstance(target, () => changeGitBranch(name, 'create'));
    }

    async function deleteGitBranch(name) {
        const target = state;
        if (!canWrite(target) || !name || name === target.gitBranch) return;
        const confirmed = await confirmValue(tr('codeStudio.gitDeleteBranchConfirm', 'Delete branch {{name}}? Git refuses to delete branches that are not merged.', { name }));
        if (!confirmed || !canWrite(target)) return;
        await runWithInstance(target, () => changeGitBranch(name, 'delete'));
    }
