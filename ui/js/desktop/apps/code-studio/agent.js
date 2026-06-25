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

        const suggestion = state.pendingSuggestion ? `<div class="code-studio-diff">
            <div class="cs-diff-head">
                <strong>${esc(tr('codeStudio.applyChanges', 'Apply Changes'))}</strong>
                <button type="button" class="cs-button primary" data-agent-apply>${buttonIcon('check-square', 'Y')}<span>${esc(tr('codeStudio.applyChanges', 'Apply Changes'))}</span></button>
                <button type="button" class="cs-button" data-agent-discard>${buttonIcon('x', 'X')}<span>${esc(tr('codeStudio.discardChanges', 'Discard Changes'))}</span></button>
            </div>
            <pre>${esc(state.pendingSuggestion)}</pre>
        </div>` : '';

        const typingIndicator = state.agentBusy ? `<div class="cs-agent-typing"><span class="cs-typing-dot"></span><span class="cs-typing-dot"></span><span class="cs-typing-dot"></span></div>` : '';

        panel.innerHTML = `<div class="cs-agent-head">
            <strong>${esc(tr('codeStudio.agentChat', 'Agent Chat'))}</strong>
            <button type="button" class="cs-icon-button" data-agent-close title="${esc(tr('codeStudio.closeTab', 'Close tab'))}">${iconMarkup('x', 'X', 'cs-icon-button-icon', 16)}</button>
        </div>
        ${quickActions}
        <div class="cs-agent-log">${messages}${typingIndicator}</div>
        ${suggestion}
        <form class="cs-agent-form" data-agent-form>
            <input name="message" autocomplete="off" spellcheck="false" placeholder="${esc(tr('desktop.chat_placeholder', 'Ask the agent...'))}">
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
            state.agentBusy = false;
            renderAgentPanel();
        }));
        const apply = panel.querySelector('[data-agent-apply]');
        if (apply) apply.addEventListener('click', bind(applyAgentSuggestion));
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
                        btn.textContent = 'Copied!';
                        setTimeout(() => { btn.textContent = 'Copy'; }, 1500);
                    }).catch(() => {});
                }
            }));
        });
        const log = panel.querySelector('.cs-agent-log');
        if (log) log.scrollTop = log.scrollHeight;
    }

    function renderMarkdown(text) {
        if (!text) return '';
        let html = esc(text);
        html = html.replace(/```(\w*)\n([\s\S]*?)```/g, (_, lang, code) => {
            const langAttr = lang ? ` data-lang="${lang}"` : '';
            return `<pre${langAttr}><code class="language-${lang || 'text'}">${code}</code><button type="button" class="cs-md-code-copy">Copy</button></pre>`;
        });
        html = html.replace(/`([^`\n]+)`/g, '<code>$1</code>');
        html = html.replace(/^### (.+)$/gm, '<h3>$1</h3>');
        html = html.replace(/^## (.+)$/gm, '<h2>$1</h2>');
        html = html.replace(/^# (.+)$/gm, '<h1>$1</h1>');
        html = html.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
        html = html.replace(/\*([^*]+)\*/g, '<em>$1</em>');
        html = html.replace(/^&gt; (.+)$/gm, '<blockquote>$1</blockquote>');
        html = html.replace(/^---$/gm, '<hr>');
        html = html.replace(/^[\-\*] (.+)$/gm, '<li>$1</li>');
        html = html.replace(/((?:<li>.*<\/li>\n?)+)/g, '<ul>$1</ul>');
        html = html.replace(/^\d+\. (.+)$/gm, '<li>$1</li>');
        html = html.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');
        html = html.replace(/^(?!<[a-z/])((?!<).+)$/gm, '<p>$1</p>');
        html = html.replace(/<p>\s*<\/p>/g, '');
        return html;
    }

    function toggleAgentPanel() {
        state.agentVisible = !state.agentVisible;
        ensureShellRoot().dataset.agent = state.agentVisible ? 'visible' : 'hidden';
        renderAgentPanel();
        renderActivityBar();
        renderWindowMenus();
    }

    async function sendAgentMessage(message) {
        const target = state;
        if (!isLiveInstance(target)) return;
        if (state.agentBusy) return;
        let context;
        runWithInstance(target, () => {
            state.agentVisible = true;
            ensureShellRoot().dataset.agent = 'visible';
            state.agentMessages.push({ role: 'user', text: message });
            state.agentMessages.push({ role: 'agent', text: tr('desktop.thinking', 'Working...') });
            state.agentBusy = true;
            state.agentAbortController = new AbortController();
            context = codeStudioAgentContext();
            renderAgentPanel();
        });
        try {
            const response = await api('/api/desktop/chat', {
                method: 'POST',
                body: JSON.stringify({ message, context }),
                signal: state.agentAbortController && state.agentAbortController.signal
            });
            if (!isLiveInstance(target)) return;
            const answer = response.answer || tr('desktop.done', 'Done');
            runWithInstance(target, () => {
                state.agentMessages[state.agentMessages.length - 1] = { role: 'agent', text: answer };
                const suggestion = extractFirstCodeBlock(answer);
                if (suggestion) state.pendingSuggestion = suggestion;
            });
        } catch (err) {
            if (isLiveInstance(target) && err.name !== 'AbortError') {
                runWithInstance(target, () => {
                    state.agentMessages[state.agentMessages.length - 1] = { role: 'agent', text: err.message || String(err) };
                });
            }
        } finally {
            if (isLiveInstance(target)) {
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
        sendAgentMessage(prompts[action] || prompts.explain);
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

    function applyAgentSuggestion() {
        const tab = activeTab();
        if (!tab || !state.pendingSuggestion) return;
        if (tab.view && tab.view.state && tab.view.state.doc) {
            tab.view.dispatch({ changes: { from: 0, to: tab.view.state.doc.length, insert: state.pendingSuggestion } });
        } else if (tab.view && tab.view.textarea) {
            tab.view.setValue(state.pendingSuggestion);
        }
        tab.content = state.pendingSuggestion;
        tab.modified = true;
        state.pendingSuggestion = null;
        renderTabs();
        renderStatus();
        renderAgentPanel();
    }

    function showCodeActionMenu(x, y) {
        document.querySelectorAll('.cs-context-menu').forEach(menu => {
            if (typeof menu.__codeStudioCleanup === 'function') menu.__codeStudioCleanup();
            else menu.remove();
        });
        const instance = state;
        const menu = document.createElement('div');
        menu.className = 'cs-context-menu';
        menu.style.left = x + 'px';
        menu.style.top = y + 'px';
        menu.innerHTML = `
            <button type="button" data-code-action="explain">${buttonIcon('info', 'i')}<span>${esc(tr('codeStudio.explain', 'Explain'))}</span></button>
            <button type="button" data-code-action="comments">${buttonIcon('notes', 'N')}<span>${esc(tr('codeStudio.generateComments', 'Generate Comments'))}</span></button>
            <button type="button" data-code-action="tests">${buttonIcon('check-square', 'T')}<span>${esc(tr('codeStudio.generateTests', 'Generate Tests'))}</span></button>
            <button type="button" data-code-action="refactor">${buttonIcon('tools', 'R')}<span>${esc(tr('codeStudio.refactor', 'Refactor'))}</span></button>`;
        document.body.appendChild(menu);
        let boundClose = null;
        let menuClosed = false;
        let unregister = () => {};
        const cleanupMenu = () => {
            if (menuClosed) return;
            menuClosed = true;
            unregister();
            if (boundClose) document.removeEventListener('mousedown', boundClose);
            menu.remove();
        };
        menu.__codeStudioCleanup = cleanupMenu;
        runWithInstance(instance, () => {
            unregister = registerDisposer(cleanupMenu);
        });
        menu.querySelectorAll('[data-code-action]').forEach(btn => {
            btn.addEventListener('click', bind(() => {
                runCodeAction(btn.dataset.codeAction);
                cleanupMenu();
            }));
        });
        setTimeout(bind(() => {
            if (menuClosed) return;
            const close = event => {
                if (!menu.contains(event.target)) {
                    cleanupMenu();
                }
            };
            boundClose = bind(close);
            document.addEventListener('mousedown', boundClose);
        }), 0);
    }
