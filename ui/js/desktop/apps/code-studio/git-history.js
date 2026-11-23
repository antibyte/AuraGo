    // Git history list, read-only commit tab and blame gutter of Code Studio. The Source Control
    // panel itself is in git.js; both fragments share the studio IIFE opened by core.js.

    let gitBlameMarkerType = null;
    let gitBlameMarkerBase = null;

    function gitRelativeDate(value) {
        const stamp = Date.parse(value || '');
        if (!Number.isFinite(stamp)) return '';
        const seconds = Math.round((stamp - Date.now()) / 1000);
        const units = [['year', 31536000], ['month', 2592000], ['week', 604800], ['day', 86400], ['hour', 3600], ['minute', 60]];
        try {
            const format = new Intl.RelativeTimeFormat(window.SYSTEM_LANG || document.documentElement.lang || undefined, { numeric: 'auto' });
            for (const [unit, size] of units) {
                if (Math.abs(seconds) >= size) return format.format(Math.round(seconds / size), unit);
            }
            return format.format(seconds, 'second');
        } catch (_) {
            return new Date(stamp).toLocaleDateString();
        }
    }

    function gitAbsoluteDate(value) {
        const stamp = Date.parse(value || '');
        return Number.isFinite(stamp) ? new Date(stamp).toLocaleString() : String(value || '');
    }

    function gitHistorySectionHtml() {
        const open = state.gitHistoryOpen !== false;
        const showAll = tr('codeStudio.gitHistoryShowAll', 'Show all commits');
        const filter = state.gitHistoryPath
            ? `<div class="cs-git-history-filter"><span title="${esc(state.gitHistoryPath)}">${esc(tr('codeStudio.gitHistoryFor', 'History of {{name}}', { name: baseName(state.gitHistoryPath) }))}</span><button type="button" data-git-history-clear title="${esc(showAll)}" aria-label="${esc(showAll)}">×</button></div>`
            : '';
        const entries = (state.gitHistory || []).map(entry => {
            const hash = String(entry.hash || '');
            const meta = [entry.author || '', gitRelativeDate(entry.date)].filter(Boolean).join(' · ');
            return `<button type="button" class="cs-git-history-entry" data-git-commit-hash="${esc(hash)}" title="${esc(entry.message || '')}">
                    <span class="cs-git-hash">${esc(entry.short || hash.slice(0, 7))}</span>
                    <span class="cs-git-history-subject">${esc(entry.message || '')}</span>
                    <span class="cs-git-history-meta">${esc(meta)}</span>
                </button>`;
        }).join('');
        const list = entries || `<div class="cs-git-empty">${esc(tr('codeStudio.gitNoCommits', 'No commits yet'))}</div>`;
        return `<div class="cs-git-section" data-git-section="history">
                <button type="button" class="cs-git-history-toggle" data-git-history-toggle aria-expanded="${open ? 'true' : 'false'}"><span class="cs-git-chevron" aria-hidden="true">▾</span><span>${esc(tr('codeStudio.gitHistory', 'History'))}</span></button>
                ${open ? filter + `<div class="cs-git-history-list">${list}</div>` : ''}
            </div>`;
    }

    function wireGitHistorySection(panel) {
        const toggle = panel.querySelector('[data-git-history-toggle]');
        if (toggle) toggle.addEventListener('click', bind(() => {
            state.gitHistoryOpen = state.gitHistoryOpen === false;
            renderGitPanel();
            if (state.gitHistoryOpen) refreshGitHistory();
        }));
        const clear = panel.querySelector('[data-git-history-clear]');
        if (clear) clear.addEventListener('click', bind(() => {
            state.gitHistoryPath = '';
            renderGitPanel();
            refreshGitHistory();
        }));
        panel.querySelectorAll('[data-git-commit-hash]').forEach(btn => {
            btn.addEventListener('click', bind(() => openGitCommit(btn.dataset.gitCommitHash)));
        });
    }

    async function refreshGitHistory() {
        const target = state;
        if (!isLiveInstance(target) || !target.gitVisible || target.gitHistoryOpen === false || target.gitIsRepo !== true) return;
        const request = target.gitHistoryRequest = Symbol();
        try {
            const result = await apiClient.gitLog(target.gitHistoryPath, 50);
            if (!isLiveInstance(target) || target.gitHistoryRequest !== request) return;
            runWithInstance(target, () => {
                state.gitHistory = Array.isArray(result.log) ? result.log : [];
                renderGitPanel();
            });
        } catch (_) {
            if (!isLiveInstance(target) || target.gitHistoryRequest !== request) return;
            runWithInstance(target, () => {
                state.gitHistory = [];
                renderGitPanel();
            });
        }
    }

    // Command "Git: File History": the History list shows the commits of the active file.
    function showGitFileHistory() {
        const tab = activeTab();
        if (!tab || isVirtualTab(tab)) return;
        state.gitHistoryPath = tab.path;
        state.gitHistoryOpen = true;
        if (!state.gitVisible) {
            toggleGitPanel();
            return;
        }
        renderGitPanel();
        refreshGitHistory();
    }

    async function openGitCommit(hash) {
        const target = state;
        const id = String(hash || '').trim();
        if (!isLiveInstance(target) || !/^[0-9a-fA-F]{4,40}$/.test(id) || /^0+$/.test(id)) return;
        let tab = target.openTabs.find(item => item.kind === 'commit' && item.hash === id);
        if (!tab) {
            tab = { kind: 'commit', hash: id, short: id.slice(0, 7), path: 'commit ' + id.slice(0, 7), content: '', modified: false, commit: null, loading: true, error: '' };
            target.openTabs.push(tab);
        }
        activateTab(target.openTabs.indexOf(tab));
        if (tab.commit) return;
        tab.loading = true;
        const request = tab.commitRequest = Symbol();
        try {
            const result = await apiClient.gitShow(id);
            if (!isLiveInstance(target) || !target.openTabs.includes(tab) || tab.commitRequest !== request) return;
            runWithInstance(target, () => {
                tab.commit = result;
                tab.loading = false;
                tab.short = result.short || tab.short;
                if (activeTab() === tab) renderEditor();
            });
        } catch (err) {
            if (!isLiveInstance(target) || !target.openTabs.includes(tab) || tab.commitRequest !== request) return;
            runWithInstance(target, () => {
                tab.loading = false;
                tab.error = err.message || String(err);
                if (activeTab() === tab) renderEditor();
            });
        }
    }

    function renderGitCommitTab(editor, tab) {
        const commit = tab.commit;
        const short = commit && commit.short || tab.short || String(tab.hash || '').slice(0, 7);
        let body;
        if (!commit) {
            body = `<div class="cs-diff-content">${esc(tab.loading ? tr('codeStudio.gitLoadingCommit', 'Loading commit...') : tab.error || '')}</div>`;
        } else {
            const files = Array.isArray(commit.files) ? commit.files : [];
            const filesHtml = files.map(file => `<div class="cs-git-commit-file"><span class="cs-git-status ${gitStatusClass(String(file.status || ''))}">${esc(file.status || '')}</span><span class="cs-git-filename">${esc(file.path || '')}</span></div>`).join('');
            body = `<div class="cs-git-commit-meta">
                    <div class="cs-git-commit-subject">${esc(commit.subject || '')}</div>
                    ${commit.body ? `<pre class="cs-git-commit-body">${esc(commit.body)}</pre>` : ''}
                    <div class="cs-git-commit-facts">
                        <span class="cs-git-hash">${esc(commit.hash || '')}</span>
                        <span>${esc(commit.author || '')}${commit.email ? ' &lt;' + esc(commit.email) + '&gt;' : ''}</span>
                        <span title="${esc(commit.date || '')}">${esc(gitAbsoluteDate(commit.date))}</span>
                    </div>
                    <div class="cs-git-commit-files">
                        <div class="cs-git-commit-files-head">${esc(tr('codeStudio.gitChangedFiles', 'Changed files ({{count}})', { count: files.length }))}</div>
                        ${filesHtml}
                    </div>
                </div>
                <div class="cs-diff-content">${commit.diff ? diffLinesHtml(commit.diff) : esc(tr('codeStudio.noChanges', 'No changes'))}${commit.truncated ? `<div class="cs-git-truncated">${esc(tr('codeStudio.gitDiffTruncated', 'The diff is too large and was cut off.'))}</div>` : ''}</div>`;
        }
        editor.innerHTML = `<div class="cs-diff-view" data-git-commit-view>
            <div class="cs-diff-view-head">
                <span class="cs-diff-view-title"><strong>${esc(short)}</strong><span class="cs-diff-badge">${esc(tr('codeStudio.commit', 'Commit'))} · ${esc(tr('codeStudio.readOnly', 'Read-only'))}</span></span>
                <span class="cs-diff-view-spacer"></span>
                <button type="button" class="cs-icon-button" data-diff-close aria-label="${esc(tr('desktop.close', 'Close'))}">×</button>
            </div>
            ${body}
        </div>`;
        editor.querySelector('[data-diff-close]').addEventListener('click', bind(() => closeTab(state.openTabs.indexOf(tab))));
    }

    function gitBlameUncommitted(entry) {
        return !entry || /^0+$/.test(String(entry.hash || ''));
    }

    // GutterMarker comes from the dynamically imported CodeMirror bundle, so the subclass is built on first use.
    function gitBlameMarkerClass(cm) {
        if (gitBlameMarkerType && gitBlameMarkerBase === cm.GutterMarker) return gitBlameMarkerType;
        gitBlameMarkerBase = cm.GutterMarker;
        gitBlameMarkerType = class GitBlameMarker extends cm.GutterMarker {
            constructor(entry, first) {
                super();
                this.entry = entry;
                this.first = first;
            }

            eq(other) {
                return other.entry === this.entry && other.first === this.first;
            }

            toDOM() {
                const element = document.createElement('div');
                const entry = this.entry;
                element.className = 'cs-blame-entry' + (this.first ? ' first' : '');
                const uncommitted = gitBlameUncommitted(entry);
                if (uncommitted) element.classList.add('uncommitted');
                element.title = uncommitted ? tr('codeStudio.gitNotCommitted', 'Not committed yet') : String(entry.summary || '');
                if (this.first) {
                    element.textContent = uncommitted
                        ? tr('codeStudio.gitNotCommitted', 'Not committed yet')
                        : [entry.author || '', gitRelativeDate(entry.date), entry.short || String(entry.hash || '').slice(0, 7)].filter(Boolean).join(' · ');
                }
                return element;
            }
        };
        return gitBlameMarkerType;
    }

    // Content of the per-view blame compartment: a gutter that shows author, relative date and
    // short hash on the first line of each run of lines from one commit; a click opens the commit.
    function gitBlameExtensions(cm, tab) {
        if (!cm || typeof cm.gutter !== 'function' || typeof cm.GutterMarker !== 'function') return [];
        if (!tab || !tab.blameOn || !Array.isArray(tab.blameLines)) return [];
        const Marker = gitBlameMarkerClass(cm);
        const byLine = [];
        tab.blameLines.forEach(entry => {
            if (entry && entry.line > 0) byLine[entry.line] = entry;
        });
        const markers = [];
        const markerAt = number => {
            const entry = byLine[number];
            if (!entry) return null;
            if (!markers[number]) {
                const previous = byLine[number - 1];
                markers[number] = new Marker(entry, !previous || previous.hash !== entry.hash);
            }
            return markers[number];
        };
        const instance = state;
        return [cm.gutter({
            class: 'cs-blame-gutter',
            lineMarker: (view, line) => markerAt(view.state.doc.lineAt(line.from).number),
            lineMarkerChange: update => update.docChanged,
            domEventHandlers: {
                click: bindInstance(instance, (view, line) => {
                    const entry = byLine[view.state.doc.lineAt(line.from).number];
                    if (!entry || gitBlameUncommitted(entry)) return false;
                    openGitCommit(entry.hash);
                    return true;
                })
            }
        })];
    }

    function applyGitBlame(tab) {
        const cm = state.cmModule;
        if (!cm || !tab) return;
        (tab.views || []).forEach(view => {
            if (!view || !view.csBlame) return;
            try { view.dispatch({ effects: view.csBlame.reconfigure(gitBlameExtensions(cm, tab)) }); } catch (_) {}
        });
    }

    // Command "Git: Toggle Blame" for the active editor.
    function toggleGitBlame() {
        const tab = activeTab();
        if (!tab || isVirtualTab(tab)) return undefined;
        tab.blameOn = !tab.blameOn;
        if (tab.blameOn) return refreshGitBlame(tab);
        tab.blameRequest = null;
        tab.blameLines = null;
        applyGitBlame(tab);
        return undefined;
    }

    async function refreshGitBlame(tab) {
        const target = state;
        if (!isLiveInstance(target) || !tab || !tab.blameOn || isVirtualTab(tab)) return;
        const path = tab.path;
        const request = tab.blameRequest = Symbol();
        try {
            const result = await apiClient.gitBlame(path);
            if (!isLiveInstance(target) || !target.openTabs.includes(tab) || tab.blameRequest !== request || tab.path !== path || !tab.blameOn) return;
            runWithInstance(target, () => {
                tab.blameLines = Array.isArray(result.lines) ? result.lines : [];
                applyGitBlame(tab);
            });
        } catch (err) {
            if (!isLiveInstance(target) || tab.blameRequest !== request) return;
            runWithInstance(target, () => {
                tab.blameOn = false;
                tab.blameLines = null;
                applyGitBlame(tab);
                showGitError(err);
            });
        }
    }
