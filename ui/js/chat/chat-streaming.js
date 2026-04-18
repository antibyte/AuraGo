// AuraGo Chat — SSE / EventSource streaming & connection handling

/* ── SSE (Server-Sent Events) ── */
const agentStatusDiv = document.getElementById('agentStatusContainer');
const agentStatusText = document.getElementById('agentStatusText');

/* ── Floating action icons ── */
const TOOL_ICONS = {
    execute_shell: '\ud83d\udda5\ufe0f', execute_python: '\ud83d\udc0d', execute_sandbox: '\ud83d\udce6',
    filesystem: '\ud83d\udcc1', system_metrics: '\ud83d\udcca', process_management: '\u2699\ufe0f',
    follow_up: '\ud83d\udd04', analyze_image: '\ud83d\udd0d', transcribe_audio: '\ud83c\udfa4',
    send_image: '\ud83d\uddbc\ufe0f', execute_skill: '\ud83c\udfaf', list_skills: '\ud83d\udcdc',
    save_tool: '\ud83d\udcbe', remote_execution: '\ud83c\udf10', api_request: '\ud83d\udd17',
    manage_memory: '\ud83e\udde0', query_memory: '\ud83e\udde0', memory_reflect: '\ud83d\udcad',
    cheatsheet: '\ud83d\udccb', knowledge_graph: '\ud83d\udd78\ufe0f', secrets_vault: '\ud83d\udd10',
    cron_scheduler: '\u23f0', manage_notes: '\ud83d\udcdd', manage_journal: '\ud83d\udcd3',
    manage_missions: '\ud83c\udfaf', query_inventory: '\ud83d\udccb', register_device: '\ud83d\udcf1',
    home_assistant: '\ud83c\udfe0', meshcentral: '\ud83d\udda7', wake_on_lan: '\u26a1',
    docker: '\ud83d\udc33', co_agent: '\ud83e\udd16', homepage: '\ud83c\udf0d', homepage_registry: '\ud83d\udcda',
    call_webhook: '\ud83e\ude9d', manage_outgoing_webhooks: '\ud83e\ude9d', manage_webhooks: '\ud83e\ude9d',
    netlify: '\ud83d\ude80', manage_updates: '\ud83d\udd04', execute_sudo: '\ud83d\udee1\ufe0f',
    proxmox: '\ud83d\udda5\ufe0f', ollama: '\ud83e\udd99', tailscale: '\ud83d\udd12',
    cloudflare_tunnel: '\u2601\ufe0f', fetch_email: '\ud83d\udce7', send_email: '\ud83d\udce7',
    list_email_accounts: '\ud83d\udce7', firewall: '\ud83e\uddf1', ansible: '\ud83d\udd27',
    invasion_control: '\ud83e\udd5a', github: '\ud83d\udc19', generate_image: '\ud83c\udfa8',
    mqtt_publish: '\ud83d\udce1', mqtt_subscribe: '\ud83d\udce1', mqtt_unsubscribe: '\ud83d\udce1',
    mqtt_get_messages: '\ud83d\udce1', mcp_call: '\ud83d\udd0c', adguard: '\ud83d\udee1\ufe0f',
    google_workspace: '\ud83d\udcca', remote_control: '\ud83c\udfae', media_registry: '\ud83c\udfac',
    thinking: '\ud83d\udca1', coding: '\ud83d\udcbb', co_agent_spawn: '\ud83e\udd16',
    _default: '\u2728'
};

function spawnFloatingIcon(toolName) {
    const pill = agentStatusDiv.querySelector('.status-pill');
    if (!pill || agentStatusDiv.classList.contains('is-hidden')) return;
    const now = Date.now();
    const key = '_lastIcon_' + toolName;
    if (spawnFloatingIcon[key] && now - spawnFloatingIcon[key] < 800) return;
    spawnFloatingIcon[key] = now;
    const icon = document.createElement('span');
    icon.className = 'floating-icon';
    icon.textContent = TOOL_ICONS[toolName] || TOOL_ICONS._default;
    const pillW = pill.offsetWidth;
    const randomX = Math.random() * Math.max(pillW - 16, 20);
    icon.style.left = randomX + 'px';
    pill.appendChild(icon);
    icon.addEventListener('animationend', () => icon.remove());
}

function setConnectionState(state) {
    const pills = ['connectionPill', 'connectionPill-m'].map(id => document.getElementById(id)).filter(Boolean);
    pills.forEach(pill => {
        pill.classList.remove('pill-active', 'pill-disconnected', 'pill-reconnecting');
        if (state === 'connected') {
            pill.classList.add('pill-active');
            pill.textContent = t('chat.agent_connected');
        } else if (state === 'reconnecting') {
            pill.classList.add('pill-reconnecting');
            pill.textContent = t('chat.agent_reconnecting');
        } else {
            pill.classList.add('pill-disconnected');
            pill.textContent = t('chat.agent_disconnected');
        }
    });
}

let sseReconnectTimer = null;
let _chatSSERegistered = false;

function connectSSE() {
    if (_chatSSERegistered) return;
    _chatSSERegistered = true;
    setConnectionState(window.AuraSSE.isConnected() ? 'connected' : 'reconnecting');

    window.AuraSSE.on('_open', function () {
        setConnectionState('connected');
        if (sseReconnectTimer) { clearTimeout(sseReconnectTimer); sseReconnectTimer = null; }
    });

    window.AuraSSE.on('_error', function (readyState) {
        if (readyState === EventSource.CLOSED) {
            setConnectionState('disconnected');
        } else {
            setConnectionState('reconnecting');
        }
        if (!sseReconnectTimer) {
            sseReconnectTimer = setTimeout(function () { sseReconnectTimer = null; }, 5000);
        }
    });

    window.AuraSSE.onLegacy(handleSSEMessage);

    let _streamingRow = null;
    let _streamingContent = '';
    let _thinkingContent = '';
    let _thinkingDiv = null;
    let _inThinkingBlock = false;
    window.AuraSSE.on('llm_stream_delta', function (payload) {
        if (!isCurrentSession(payload)) return;
        if (!payload || !payload.content) return;
        payload.content = payload.content.replace(/\u003cdone\s*\/?\u003e/gi, '');
        if (typeof stripLeakedToolMarkup === 'function') {
            payload.content = stripLeakedToolMarkup(payload.content);
        }
        if (!payload.content) return;
        const trimmed = payload.content.trimStart();
        if (trimmed.length > 0 && trimmed[0] === '{' &&
            (trimmed.includes('"tool_call"') || trimmed.includes('"tool_name"') ||
             trimmed.includes('"action"') || trimmed.includes('"command"') ||
             trimmed.includes('"operation"') || trimmed.includes('"arguments"'))) {
            return;
        }
        if (!_streamingRow) {
            _streamingRow = document.createElement('div');
            _streamingRow.className = 'msg-row bot';
            _streamingRow.innerHTML = '\u003cdiv class="avatar bot"\u003e\ud83e\udd16\u003c/div\u003e\u003cdiv class="bubble bot"\u003e\u003c/div\u003e';
            chatContent.appendChild(_streamingRow);
        }
        _streamingContent += payload.content;
        const bubble = _streamingRow.querySelector('.bubble');
        if (!bubble) return;
        if (_inThinkingBlock) {
        } else if (_thinkingContent) {
            const label = typeof t === 'function' ? t('chat.thinking_label') : 'Reasoning';
            const detailsHtml = '\u003cdetails class="thinking-block"\u003e\u003csummary\u003e\ud83e\udde0 ' + label + '\u003c/summary\u003e\u003cdiv class="thinking-content"\u003e' + escapeHtml(_thinkingContent) + '\u003c/div\u003e\u003c/details\u003e';
            bubble.innerHTML = detailsHtml + '\n\n' + escapeHtml(_streamingContent);
        } else {
            bubble.textContent = _streamingContent;
        }
        if (window.decorateEmojiGlyphs) {
            window.decorateEmojiGlyphs(bubble);
        }
        chatBox.scrollTop = chatBox.scrollHeight;
    });
    window.AuraSSE.on('llm_stream_done', function (payload) {
        if (!isCurrentSession(payload)) return;
        _streamingRow = null;
        _streamingContent = '';
        _thinkingContent = '';
        _thinkingDiv = null;
        _inThinkingBlock = false;
    });
    window.AuraSSE.on('thinking_block', function (payload) {
        if (!isCurrentSession(payload)) return;
        if (!payload || !payload.state) return;
        if (!_streamingRow) {
            _streamingRow = document.createElement('div');
            _streamingRow.className = 'msg-row bot';
            _streamingRow.innerHTML = '\u003cdiv class="avatar bot"\u003e\ud83e\udd16\u003c/div\u003e\u003cdiv class="bubble bot"\u003e\u003c/div\u003e';
            chatContent.appendChild(_streamingRow);
        }
        const bubble = _streamingRow.querySelector('.bubble');
        if (!bubble) return;
        if (payload.state === 'start') {
            _inThinkingBlock = true;
            _thinkingContent = '';
            const label = typeof t === 'function' ? t('chat.thinking_label') : 'Reasoning';
            const detailsHtml = '\u003cdetails class="thinking-block"\u003e\u003csummary\u003e\ud83e\udde0 ' + label + '\u003c/summary\u003e\u003cdiv class="thinking-content"\u003e\u003c/div\u003e\u003c/details\u003e';
            bubble.innerHTML = escapeHtml(_streamingContent) + detailsHtml;
            if (window.decorateEmojiGlyphs) {
                window.decorateEmojiGlyphs(bubble);
            }
            _thinkingDiv = bubble.querySelector('.thinking-content');
        } else if (payload.state === 'delta' && _thinkingDiv) {
            _thinkingContent += payload.content || '';
            _thinkingDiv.textContent = _thinkingContent;
            chatBox.scrollTop = chatBox.scrollHeight;
        } else if (payload.state === 'stop') {
            _inThinkingBlock = false;
            _thinkingDiv = null;
        }
    });

    window.AuraSSE.on('token_update', function (payload) {
        if (!isCurrentSession(payload)) return;
        if (!payload) return;
        const tokenEl = document.getElementById('tokenCounter');
        if (!tokenEl) return;
        const session = payload.session_total || 0;
        const est = payload.is_estimated ? ' ~' : '';
        tokenEl.textContent = t('chat.token_counter_format', { count: session.toLocaleString() + est });
    });
}

setConnectionState('reconnecting');
connectSSE();

function handleSSEMessage(e) {
    try {
        const data = JSON.parse(e.data);
        let message = '';
        if (!data.event) return;
        if (data.session_id && data.session_id !== getActiveSessionId()) return;
        if (data.event === 'thinking' || data.event === 'tool_start' || data.event === 'co_agent_spawn' || data.event === 'coding') {
            chatSetHidden(agentStatusDiv, false);
        }
        if (data.event === 'thinking') {
            stopBtn.disabled = false;
            message = data.detail || t('chat.sse_thinking');
            spawnFloatingIcon('thinking');
        } else if (data.event === 'tool_start') {
            if (data.detail === 'execute_skill') {
                message = t('chat.sse_execute_skill') + data.detail;
            } else if (data.detail === 'list_skills') {
                message = t('chat.sse_list_skills');
            } else if (data.detail === 'co_agent' || data.detail === 'co_agents') {
                return;
            } else {
                message = t('chat.sse_tool_start') + data.detail;
            }
            spawnFloatingIcon(data.detail);
        } else if (data.event === 'co_agent_spawn') {
            message = t('chat.sse_co_agent_spawn') + data.detail;
            spawnFloatingIcon('co_agent_spawn');
        } else if (data.event === 'workflow_plan') {
            message = t('chat.sse_workflow_plan');
        } else if (data.event === 'tool_end') {
            if (data.detail === 'co_agent' || data.detail === 'co_agents') {
                return;
            }
            message = t('chat.sse_tool_end') + data.detail;
        } else if (data.event === 'coding') {
            message = t('chat.sse_coding');
            spawnFloatingIcon('coding');
        } else if (data.event === 'error_recovery') {
            message = t('chat.sse_error_recovery');
        } else if (data.event === 'tool_call') {
            if (debugMode) {
                appendToolOutput(data.detail, t('chat.tool_call_label'));
            }
            let thinkingText = (data.detail || '')
                .replace(/```json[\s\S]*?```/g, '')
                .replace(/`[^`]*`/g, '')
                .replace(/\{[\s\S]*"action"\s*:[\s\S]*/g, '')
                .replace(/\{[\s\S]*"tool_call"\s*:[\s\S]*/g, '')
                .replace(/\{[\s\S]*"tool_name"\s*:[\s\S]*/g, '')
                .trim();
            if (typeof stripLeakedToolMarkup === 'function') {
                thinkingText = stripLeakedToolMarkup(thinkingText);
            }
            if (thinkingText && thinkingText.split(/\s+/).filter(Boolean).length >= 6) {
                appendMessage('assistant', thinkingText);
            }
            return;
        } else if (data.event === 'tool_output') {
            if (debugMode) {
                appendToolOutput(data.detail, t('chat.tool_output_label'));
            }
            return;
        } else if (data.event === 'budget_update') {
            try {
                const b = JSON.parse(data.message || '{}');
                updateBudgetPills(b);
            } catch (_) { }
            return;
        } else if (data.event === 'budget_warning') {
            appendMessage('system', '\u26a0\ufe0f ' + (data.message || t('chat.budget_warning')));
            return;
        } else if (data.event === 'budget_blocked') {
            appendMessage('system', '\ud83d\udeab ' + (data.message || t('chat.budget_blocked')));
            return;
        } else if (data.event === 'todo_update') {
            updateTodoPanel(data.detail);
            return;
        } else if (data.event === 'plan_update') {
            try {
                const payload = JSON.parse(data.detail || '{}');
                updatePlanPanel(payload.plan || null);
            } catch (_) {
                updatePlanPanel(null);
            }
            return;
        } else if (data.event === 'image') {
            try {
                const imgData = JSON.parse(data.detail);
                if (imgData && imgData.path) {
                    seenSSEImages.add(imgData.path);
                    const cap = imgData.caption ? escapeHtml(imgData.caption) : '';
                    const safePath = escapeHtml(imgData.path);
                    const imgHTML = `
                                \u003cdiv class="msg-row bot"\u003e
                                    \u003cdiv class="avatar bot"\u003e\ud83e\udd16\u003c/div\u003e
                                    \u003cdiv class="bubble bot"\u003e\u003cimg class="chat-zoomable-image" src="${safePath}" alt="${cap}" title="${cap}" loading="lazy"\u003e\u003c/div\u003e
                                \u003c/div\u003e`;
                    chatContent.insertAdjacentHTML('beforeend', imgHTML);
                    chatBox.scrollTop = chatBox.scrollHeight;
                }
            } catch (e) { }
            return;
        } else if (data.event === 'audio') {
            try {
                const audioData = JSON.parse(data.detail);
                if (audioData && audioData.path && !seenSSEAudios.has(audioData.path)) {
                    seenSSEAudios.add(audioData.path);
                    if (speakerMode) {
                        enqueueAutoPlay(audioData.path);
                    } else {
                        const wrapper = document.createElement('div');
                        wrapper.className = 'chat-audio-wrapper';
                        if (audioData.title) {
                            const titleEl = document.createElement('div');
                            titleEl.className = 'chat-audio-title';
                            titleEl.textContent = audioData.title;
                            wrapper.appendChild(titleEl);
                        }
                        const player = new ChatAudioPlayer(audioData.path);
                        wrapper.appendChild(player.element);
                        const row = document.createElement('div');
                        row.className = 'msg-row bot';
                        row.innerHTML = '\u003cdiv class="avatar bot"\u003e\ud83e\udd16\u003c/div\u003e\u003cdiv class="bubble bot"\u003e\u003c/div\u003e';
                        row.querySelector('.bubble').appendChild(wrapper);
                        chatContent.appendChild(row);
                        chatBox.scrollTop = chatBox.scrollHeight;
                    }
                }
            } catch (e) { }
            return;
        } else if (data.event === 'document') {
            try {
                const docData = JSON.parse(data.detail);
                if (docData && docData.path && !seenSSEDocuments.has(docData.path)) {
                    seenSSEDocuments.add(docData.path);
                    const title = escapeHtml(docData.title || docData.filename || 'Document');
                    const fmt = escapeHtml((docData.format || '').toUpperCase() || 'FILE');
                    const docIcon = docFormatIcon(docData.format);
                    const previewUrl = isSafeHref(docData.preview_url, true) ? docData.preview_url : '';
                    const downloadPath = isSafeHref(docData.path, true) ? docData.path : '';
                    const openBtn = previewUrl
                        ? `\u003ca href="${escapeHtml(previewUrl)}" target="_blank" rel="noopener noreferrer" title="Open"\u003e\ud83d\udd0d\u003c/a\u003e`
                        : '';
                    const dlBtn = downloadPath
                        ? `\u003ca href="${escapeHtml(downloadPath)}" download="${escapeHtml(docData.filename || 'document')}" title="Download"\u003e\u2b07\u003c/a\u003e`
                        : '';
                    const cardHTML = `
                        \u003cdiv class="chat-document-card"\u003e
                            \u003cdiv class="chat-document-icon"\u003e${docIcon}\u003c/div\u003e
                            \u003cdiv class="chat-document-info"\u003e
                                \u003cdiv class="chat-document-title"\u003e${title}\u003c/div\u003e
                                \u003cdiv class="chat-document-format"\u003e${fmt}\u003c/div\u003e
                            \u003c/div\u003e
                            \u003cdiv class="chat-document-actions"\u003e${openBtn}${dlBtn}\u003c/div\u003e
                        \u003c/div\u003e`;
                    const row = document.createElement('div');
                    row.className = 'msg-row bot';
                    row.innerHTML = '\u003cdiv class="avatar bot"\u003e\ud83e\udd16\u003c/div\u003e\u003cdiv class="bubble bot"\u003e\u003c/div\u003e';
                    row.querySelector('.bubble').insertAdjacentHTML('beforeend', cardHTML);
                    chatContent.appendChild(row);
                    chatBox.scrollTop = chatBox.scrollHeight;
                }
            } catch (e) { }
            return;
        } else if (data.event === 'done') {
            _fetchConnectionLost = false;
            chatSetHidden(agentStatusDiv, true);
            stopBtn.disabled = true;
            hideTodoPanel();
            if (!_httpResponseRendered) {
                setTimeout(() => {
                    if (!_httpResponseRendered) {
                        tryRecoverFromHistory();
                    }
                }, 1500);
            }
            return;
        }
        if (message) {
            agentStatusText.textContent = message;
            chatSetHidden(agentStatusDiv, false);
        }
    } catch (err) { }
}
