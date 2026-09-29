    function renderAgentPanel() {
        const panel = shellPart('[data-agent-panel]');
        if (!panel) return;
        if (!state.agentVisible) {
            panel.innerHTML = '';
            return;
        }
        const messages = state.agentMessages.length ? state.agentMessages.map(message => {
            const roleClass = message.role === 'user' ? 'user' : 'agent';
            const content = message.role === 'user' ? esc(message.text) : renderMarkdown(message.text);
            return `<div class="cs-agent-message ${roleClass}"><div class="cs-md-content">${content}</div></div>`;
        }).join('') : `<div class="cs-agent-message agent"><div class="cs-md-content">${esc(tr('desktop.chat_welcome', 'Ask me to create apps, widgets, or files for this desktop.'))}</div></div>`;

        const quickActions = `<div class="cs-quick-actions">
            <button type="button" class="cs-quick-action" data-code-action="explain">${esc(tr('codeStudio.explain', 'Explain'))}</button>
            <button type="button" class="cs-quick-action" data-code-action="comments">${esc(tr('codeStudio.generateComments', 'Comments'))}</button>
            <button type="button" class="cs-quick-action" data-code-action="tests">${esc(tr('codeStudio.generateTests', 'Tests'))}</button>
            <button type="button" class="cs-quick-action" data-code-action="refactor">${esc(tr('codeStudio.refactor', 'Refactor'))}</button>
        </div>`;

        const pending = state.pendingSuggestion;
        const suggestion = pending ? `<div class="code-studio-diff">
            <div class="cs-diff-head">
                <strong>${esc(tr('codeStudio.applyChanges', 'Apply Changes'))}</strong>
                <button type="button" class="cs-icon-button primary" data-agent-apply${suggestionCanApply(pending) ? '' : ' disabled'} title="${esc(tr('codeStudio.applyChanges', 'Apply Changes'))}" aria-label="${esc(tr('codeStudio.applyChanges', 'Apply Changes'))}">${iconMarkup('check-square', 'Y', 'cs-icon-button-icon', 15)}</button>
                <button type="button" class="cs-icon-button" data-agent-copy title="${esc(tr('desktop.copy'))}" aria-label="${esc(tr('desktop.copy'))}">${iconMarkup('copy', 'C', 'cs-icon-button-icon', 15)}</button>
                <button type="button" class="cs-icon-button" data-agent-discard title="${esc(tr('codeStudio.discardChanges', 'Discard Changes'))}" aria-label="${esc(tr('codeStudio.discardChanges', 'Discard Changes'))}">${iconMarkup('x', 'X', 'cs-icon-button-icon', 15)}</button>
            </div>
            <p data-agent-suggestion-note>${esc(suggestionNote(pending))}</p>
            <pre>${esc(pending.text)}</pre>
        </div>` : '';

        const typingIndicator = state.agentBusy ? `<div class="cs-agent-typing"><span class="cs-typing-dot"></span><span class="cs-typing-dot"></span><span class="cs-typing-dot"></span></div>` : '';

        panel.innerHTML = `<div class="cs-agent-head">
            <strong>${esc(tr('codeStudio.agentChat', 'Agent Chat'))}</strong>
            <button type="button" class="cs-icon-button" data-agent-close title="${esc(tr('desktop.close', 'Close'))}">${iconMarkup('x', 'X', 'cs-icon-button-icon', 16)}</button>
        </div>
        ${quickActions}
        <div class="cs-agent-log">${messages}${typingIndicator}</div>
        ${suggestion}
        <form class="cs-agent-form" data-agent-form>
            <input name="message" autocomplete="off" spellcheck="false" inputmode="text" enterkeyhint="send" placeholder="${esc(tr('desktop.chat_placeholder', 'Ask the agent...'))}">
            ${state.agentBusy
                ? `<button type="button" class="cs-agent-stop" data-agent-stop>${esc(tr('codeStudio.stop', 'Stop'))}</button>`
                : `<button type="submit" class="cs-button primary">${buttonIcon('chat', 'S')}<span>${esc(tr('desktop.send', 'Send'))}</span></button>`
            }
        </form>`;
        panel.querySelector('[data-agent-close]').addEventListener('click', bind(toggleAgentPanel));
        panel.querySelectorAll('[data-code-action]').forEach(btn => {
            btn.addEventListener('click', bind(() => runCodeAction(btn.dataset.codeAction)));
        });
        panel.querySelector('[data-agent-form]').addEventListener('submit', bind(event => {
            event.preventDefault();
            const input = event.currentTarget.elements.message;
            const message = input.value.trim();
            if (!message) return;
            input.value = '';
            sendAgentMessage(message);
        }));
        const stopBtn = panel.querySelector('[data-agent-stop]');
        if (stopBtn) stopBtn.addEventListener('click', bind(() => {
            if (state.agentAbortController) {
                state.agentAbortController.abort();
                state.agentAbortController = null;
            }
            const last = state.agentMessages.at(-1);
            if (last && last.role === 'agent') last.text = tr('codeStudio.stopped', 'Stopped');
            state.agentBusy = false;
            renderAgentPanel();
        }));
        const apply = panel.querySelector('[data-agent-apply]');
        if (apply) apply.addEventListener('click', bind(applyAgentSuggestion));
        const copy = panel.querySelector('[data-agent-copy]');
        if (copy) copy.addEventListener('click', bind(() => copyTextToClipboard(pending.text)));
        const discard = panel.querySelector('[data-agent-discard]');
        if (discard) discard.addEventListener('click', bind(() => {
            state.pendingSuggestion = null;
            renderAgentPanel();
        }));
        panel.querySelectorAll('.cs-md-code-copy').forEach(btn => {
            btn.addEventListener('click', bind(() => {
                const code = btn.closest('pre')?.querySelector('code');
                if (code) {
                    navigator.clipboard.writeText(code.textContent).then(() => {
                        btn.textContent = tr('desktop.copied');
                        setTimeout(() => { btn.textContent = tr('desktop.copy'); }, 1500);
                    }).catch(() => {});
                }
            }));
        });
        const log = panel.querySelector('.cs-agent-log');
        if (log) log.scrollTop = log.scrollHeight;
        if (!state.agentBusy) {
            const input = panel.querySelector('input[name="message"]');
            if (input && document.activeElement && panel.contains(document.activeElement)) input.focus();
        }
    }

    function renderMarkdown(text) {
        if (!text) return '';
        const blocks = [];
        const inline = [];
        const marker = '\u0000';
        let source = String(text).replace(/\r\n?/g, '\n');
        source = source.replace(/```([\w+#.-]*)[ \t]*\n([\s\S]*?)```/g, (_, lang, code) => {
            const language = String(lang || '').toLowerCase();
            const index = blocks.length;
            blocks.push(`<pre${language ? ` data-lang="${esc(language)}"` : ''}><code class="language-${esc(language || 'text')}">${esc(code.replace(/\n$/, ''))}</code><button type="button" class="cs-md-code-copy">${esc(tr('desktop.copy'))}</button></pre>`);
            return `\n${marker}BLOCK${index}${marker}\n`;
        });
        source = source.replace(/`([^`\n]+)`/g, (_, code) => {
            inline.push(`<code>${esc(code)}</code>`);
            return `${marker}INLINE${inline.length - 1}${marker}`;
        });
        let html = esc(source);
        html = html.replace(/^### (.+)$/gm, '<h3>$1</h3>');
        html = html.replace(/^## (.+)$/gm, '<h2>$1</h2>');
        html = html.replace(/^# (.+)$/gm, '<h1>$1</h1>');
        html = html.replace(/\*\*([^*\n]+)\*\*/g, '<strong>$1</strong>');
        html = html.replace(/(^|[^*\w])\*([^*\n]+)\*(?!\w)/g, '$1<em>$2</em>');
        html = html.replace(/^&gt; (.+)$/gm, '<blockquote>$1</blockquote>');
        html = html.replace(/^---$/gm, '<hr>');
        html = html.replace(/^[\-\*] (.+)$/gm, '<li data-list="ul">$1</li>');
        html = html.replace(/^\d+\. (.+)$/gm, '<li data-list="ol">$1</li>');
        html = html.replace(/((?:<li data-list="ul">.*<\/li>\n?)+)/g, '<ul>$1</ul>');
        html = html.replace(/((?:<li data-list="ol">.*<\/li>\n?)+)/g, '<ol>$1</ol>');
        html = html.replace(/ data-list="(?:ul|ol)"/g, '');
        html = html.replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, (_, label, href) => `<a href="${sanitizeMarkdownHref(href)}" target="_blank" rel="noopener">${label}</a>`);
        html = html.replace(/^(?!<[a-z/]|\u0000)((?!<).+)$/gm, '<p>$1</p>');
        html = html.replace(/<p>\s*<\/p>/g, '');
        html = html.replace(/\u0000BLOCK(\d+)\u0000/g, (_, index) => blocks[Number(index)] || '');
        html = html.replace(/\u0000INLINE(\d+)\u0000/g, (_, index) => inline[Number(index)] || '');
        return html;
    }

    function sanitizeMarkdownHref(rawHref) {
        const href = String(rawHref || '').trim();
        try {
            const parsed = new URL(href, window.location.origin);
            const protocol = parsed.protocol;
            if (protocol === 'http:' || protocol === 'https:' || protocol === 'mailto:') {
                return href;
            }
        } catch (_) {}
        return '#';
    }

    function toggleAgentPanel() {
        state.agentVisible = !state.agentVisible;
        ensureShellRoot().dataset.agent = state.agentVisible ? 'visible' : 'hidden';
        renderAgentPanel();
        renderActivityBar();
        renderWindowMenus();
        scheduleTerminalFit();
        if (state.agentVisible) {
            const input = shellPart('[data-agent-panel] input[name="message"]');
            if (input) input.focus();
        }
    }

    async function sendAgentMessage(message, intent) {
        const target = state;
        if (!isLiveInstance(target)) return;
        if (state.agentBusy) return;
        const tab = activeTab();
        const selection = codeStudioSelection();
        const source = {
            windowId: target.windowId, tab, path: tab && tab.path,
            revision: tab && (tab.revision || 0), intent,
            from: selection.text ? selection.from : 0,
            to: selection.text ? selection.to : editorValue(tab).length
        };
        const controller = new AbortController();
        const reply = { role: 'agent', text: tr('desktop.thinking', 'Working...') };
        let context;
        runWithInstance(target, () => {
            state.agentVisible = true;
            ensureShellRoot().dataset.agent = 'visible';
            state.agentMessages.push({ role: 'user', text: message });
            state.agentMessages.push(reply);
            state.agentBusy = true;
            state.agentAbortController = controller;
            state.pendingSuggestion = null;
            context = codeStudioAgentContext();
            renderAgentPanel();
            renderActivityBar();
        });
        try {
            const response = await api('/api/desktop/chat', {
                method: 'POST',
                body: JSON.stringify({ message, context }),
                signal: controller.signal
            });
            if (!isLiveInstance(target) || target.agentAbortController !== controller || controller.signal.aborted) return;
            const answer = response.answer || tr('desktop.done', 'Done');
            runWithInstance(target, () => {
                reply.text = answer;
                const suggestion = extractFirstCodeBlock(answer);
                if (suggestion) state.pendingSuggestion = {
                    ...source, text: suggestion,
                    canReplace: ['refactor', 'comments'].includes(intent) && (answer.match(/```/g) || []).length === 2
                };
            });
        } catch (err) {
            if (isLiveInstance(target) && target.agentAbortController === controller && err.name !== 'AbortError') {
                runWithInstance(target, () => {
                    reply.text = err.message || String(err);
                });
            }
        } finally {
            if (isLiveInstance(target) && target.agentAbortController === controller) {
                runWithInstance(target, () => {
                    state.agentBusy = false;
                    state.agentAbortController = null;
                    renderAgentPanel();
                });
            }
        }
    }

    function runCodeAction(action) {
        const tab = activeTab();
        if (!tab) return;
        const selection = codeStudioSelection();
        const target = selection.text ? 'selected code' : 'current file';
        const prompts = {
            explain: `Explain the ${target} in ${tab.path}.`,
            comments: `Generate clear comments for the ${target} in ${tab.path}. Return only the modified code when you change code.`,
            tests: `Generate useful tests for ${tab.path}. Return code blocks for new or changed files.`,
            refactor: `Refactor the ${target} in ${tab.path}. Return only the modified code.`
        };
        sendAgentMessage(prompts[action] || prompts.explain, action);
    }

    function codeStudioAgentContext() {
        const tab = activeTab();
        const cursor = codeStudioCursor();
        const selection = codeStudioSelection();
        const content = tab ? editorValue(tab) : '';
        return {
            source: 'code-studio',
            current_file: tab ? tab.path : '',
            current_language: tab ? tab.language : '',
            current_content: selection.text ? '' : content,
            cursor_line: cursor.line,
            cursor_column: cursor.column,
            selected_text: selection.text,
            open_files: state.openTabs.map(item => item.path)
        };
    }

    function extractFirstCodeBlock(text) {
        const match = String(text || '').match(/```[a-zA-Z0-9_-]*\n([\s\S]*?)```/);
        return match ? match[1].trimEnd() : '';
    }

    function suggestionCanApply(suggestion) {
        return suggestion && suggestion.canReplace && suggestion.windowId === state.windowId &&
            state.openTabs.includes(suggestion.tab) && suggestion.tab.path === suggestion.path &&
            !suggestion.tab.pathMutation && (suggestion.tab.revision || 0) === suggestion.revision;
    }

    function suggestionNote(suggestion) {
        if (!suggestion.canReplace) return tr('codeStudio.suggestionCopyOnly');
        return suggestionCanApply(suggestion) ? suggestion.path : tr('codeStudio.suggestionStale');
    }

    function updateSuggestionStatus() {
        const suggestion = state.pendingSuggestion;
        if (!suggestion) return;
        const apply = state.root.querySelector('[data-agent-apply]');
        const note = state.root.querySelector('[data-agent-suggestion-note]');
        if (apply) apply.disabled = !suggestionCanApply(suggestion);
        if (note) note.textContent = suggestionNote(suggestion);
    }

    function applyAgentSuggestion() {
        const suggestion = state.pendingSuggestion;
        if (!suggestion) return;
        if (!suggestionCanApply(suggestion)) {
            updateSuggestionStatus();
            return;
        }
        const tab = suggestion.tab;
        if (activeTab() !== tab) activateTab(state.openTabs.indexOf(tab));
        if (tab.view && tab.view.state && tab.view.state.doc) {
            tab.view.dispatch({ changes: { from: suggestion.from, to: suggestion.to, insert: suggestion.text }, userEvent: 'input' });
        } else if (tab.view && tab.view.textarea) {
            tab.view.setValue(tab.content.slice(0, suggestion.from) + suggestion.text + tab.content.slice(suggestion.to));
        }
        state.pendingSuggestion = null;
        renderTabs();
        renderStatus();
        renderAgentPanel();
    }

    function showCodeActionMenu(x, y) {
        const tab = activeTab();
        const hasSelection = !!codeStudioSelection().text;
        const items = [];
        if (tab) {
            items.push({ id: 'save', label: tr('codeStudio.save', 'Save'), icon: 'save', shortcut: 'Ctrl+S', disabled: !tab.modified, action: bind(() => saveCurrentFile()) });
            items.push({ id: 'run', label: tr('codeStudio.run', 'Run'), icon: 'run', shortcut: 'F5', action: bind(() => runCurrentFile()) });
            items.push({ id: 'copy-path', label: tr('codeStudio.copyPath', 'Copy path'), icon: 'copy', action: bind(() => copyTextToClipboard(tab.path)) });
            items.push({ separator: true });
        }
        items.push({ id: 'explain', label: tr('codeStudio.explain', 'Explain'), icon: 'info', disabled: !tab, action: bind(() => runCodeAction('explain')) });
        items.push({ id: 'comments', label: tr('codeStudio.generateComments', 'Generate Comments'), icon: 'notes', disabled: !tab, action: bind(() => runCodeAction('comments')) });
        items.push({ id: 'tests', label: tr('codeStudio.generateTests', 'Generate Tests'), icon: 'check-square', disabled: !tab, action: bind(() => runCodeAction('tests')) });
        items.push({ id: 'refactor', label: tr('codeStudio.refactor', 'Refactor'), icon: 'tools', disabled: !tab, action: bind(() => runCodeAction('refactor')) });
        if (hasSelection) {
            items.push({ separator: true });
            items.push({ id: 'ask-agent', label: tr('codeStudio.agentChat', 'Agent Chat'), icon: 'chat', action: bind(() => { if (!state.agentVisible) toggleAgentPanel(); }) });
        }
        showStudioContextMenu(x, y, items);
    }
