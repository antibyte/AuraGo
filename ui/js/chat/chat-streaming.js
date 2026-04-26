// AuraGo Chat — SSE / EventSource streaming & connection handling

/* ── SSE (Server-Sent Events) ── */
const agentStatusDiv = document.getElementById('agentStatusContainer');
const agentStatusText = document.getElementById('agentStatusText');
const agentStatusIcon = document.getElementById('agentStatusIcon');
const chatRobotEffects = document.getElementById('chat-robot-effects');
const toolIconStack = document.getElementById('tool-icon-stack');
const TOOL_STACK_IDLE_MS = 10000;
const TOOL_STACK_FADE_MS = 1000;
let toolStackIdleTimer = null;
let toolStackFadeTimer = null;

/* ── Floating action icons ── */
function setStatusToolIcon(toolName) {
    if (!agentStatusIcon || !window.AuraToolIcons) return;
    if (!toolName) {
        agentStatusIcon.classList.add('is-hidden');
        agentStatusIcon.removeAttribute('data-tool-icon');
        return;
    }
    window.AuraToolIcons.applyIcon(agentStatusIcon, toolName);
    agentStatusIcon.classList.remove('is-hidden');
}

function scheduleToolStackFade() {
    if (!toolIconStack) return;
    clearTimeout(toolStackIdleTimer);
    clearTimeout(toolStackFadeTimer);
    toolStackIdleTimer = setTimeout(() => {
        toolIconStack.classList.add('is-fading');
        toolStackFadeTimer = setTimeout(() => {
            toolIconStack.replaceChildren();
            toolIconStack.classList.remove('is-fading', 'has-icons');
        }, TOOL_STACK_FADE_MS);
    }, TOOL_STACK_IDLE_MS);
}

function pushToolStackIcon(toolName) {
    if (!toolIconStack || !window.AuraToolIcons) return;
    clearTimeout(toolStackFadeTimer);
    toolIconStack.classList.remove('is-fading');
    const icon = window.AuraToolIcons.createIcon(toolName, 'tool-stack-icon');
    toolIconStack.replaceChildren(icon);
    toolIconStack.classList.add('has-icons');
    scheduleToolStackFade();
}

function spawnFloatingIcon(toolName) {
    const effectHost = chatRobotEffects || (agentStatusDiv ? agentStatusDiv.querySelector('.status-pill') : null);
    if (!effectHost || !window.AuraToolIcons || !agentStatusDiv || agentStatusDiv.classList.contains('is-hidden')) return;
    const now = Date.now();
    const key = '_lastIcon_' + toolName;
    if (spawnFloatingIcon[key] && now - spawnFloatingIcon[key] < 800) return;
    spawnFloatingIcon[key] = now;
    const icon = window.AuraToolIcons.createIcon(toolName, 'floating-icon');
    const hostW = effectHost.offsetWidth || 72;
    const randomX = (hostW * (0.32 + Math.random() * 0.36));
    const drift = Math.round((Math.random() - 0.5) * Math.min(42, Math.max(18, hostW * 0.36)));
    const tilt = Math.random() * 24 - 12;
    icon.style.left = randomX + 'px';
    icon.style.setProperty('--tool-bubble-drift', drift + 'px');
    icon.style.setProperty('--tool-bubble-drift-mid', Math.round(drift * 0.42) + 'px');
    icon.style.setProperty('--tool-bubble-drift-end', Math.round(drift * 1.14) + 'px');
    icon.style.setProperty('--tool-bubble-tilt', tilt.toFixed(1) + 'deg');
    icon.style.setProperty('--tool-bubble-tilt-start', (-tilt * 0.6).toFixed(1) + 'deg');
    icon.style.setProperty('--tool-bubble-tilt-soft', (-tilt * 0.35).toFixed(1) + 'deg');
    icon.style.setProperty('--tool-bubble-tilt-mid', (tilt * 0.3).toFixed(1) + 'deg');
    icon.style.setProperty('--tool-bubble-tilt-pop', (tilt * 1.18).toFixed(1) + 'deg');
    icon.style.animationDelay = Math.round(Math.random() * 90) + 'ms';
    effectHost.appendChild(icon);
    pushToolStackIcon(toolName);
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
             trimmed.includes('"tool"') || trimmed.includes('"parameters"') ||
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
            setStatusToolIcon('thinking');
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
            setStatusToolIcon(data.detail);
            spawnFloatingIcon(data.detail);
        } else if (data.event === 'co_agent_spawn') {
            message = t('chat.sse_co_agent_spawn') + data.detail;
            setStatusToolIcon('co_agent_spawn');
            spawnFloatingIcon('co_agent_spawn');
        } else if (data.event === 'workflow_plan') {
            message = t('chat.sse_workflow_plan');
            setStatusToolIcon('manage_plan');
        } else if (data.event === 'tool_end') {
            if (data.detail === 'co_agent' || data.detail === 'co_agents') {
                return;
            }
            message = t('chat.sse_tool_end') + data.detail;
            setStatusToolIcon(data.detail);
        } else if (data.event === 'coding') {
            message = t('chat.sse_coding');
            setStatusToolIcon('coding');
            spawnFloatingIcon('coding');
        } else if (data.event === 'error_recovery') {
            message = t('chat.sse_error_recovery');
            setStatusToolIcon('generic_tool');
        } else if (data.event === 'tool_call') {
            if (debugMode) {
                appendToolOutput(data.detail, t('chat.tool_call_label'));
            }
            let thinkingText = (data.detail || '')
                .replace(/```json[\s\S]*?```/g, '')
                .replace(/`[^`]*`/g, '')
                .replace(/\{[\s\S]*"action"\s*:[\s\S]*/g, '')
                .replace(/\{[\s\S]*"tool"\s*:[\s\S]*/g, '')
                .replace(/\{[\s\S]*"tool_call"\s*:[\s\S]*/g, '')
                .replace(/\{[\s\S]*"tool_name"\s*:[\s\S]*/g, '')
                .replace(/\{[\s\S]*"parameters"\s*:[\s\S]*/g, '')
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
                        const botIcon = window.chatUiIconMarkup ? window.chatUiIconMarkup('bot') : '';
                        row.innerHTML = `<div class="avatar bot">${botIcon}</div><div class="bubble bot"></div>`;
                        row.querySelector('.bubble').appendChild(wrapper);
                        chatContent.appendChild(row);
                        chatBox.scrollTop = chatBox.scrollHeight;
                    }
                }
            } catch (e) { }
            return;
        } else if (data.event === 'video') {
            try {
                const videoData = JSON.parse(data.detail);
                if (videoData && videoData.path && !seenSSEVideos.has(videoData.path)) {
                    seenSSEVideos.add(videoData.path);
                    appendVideoMessage(videoData);
                }
            } catch (e) { }
            return;
        } else if (data.event === 'youtube_video') {
            try {
                const youtubeData = JSON.parse(data.detail);
                const key = youtubeData && (youtubeData.video_id || youtubeData.url || youtubeData.embed_url);
                if (key && !seenSSEYouTubeVideos.has(key)) {
                    seenSSEYouTubeVideos.add(key);
                    appendYouTubeMessage(youtubeData);
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
                    const openIcon = window.chatUiIconMarkup ? window.chatUiIconMarkup('search') : '';
                    const downloadIcon = window.chatUiIconMarkup ? window.chatUiIconMarkup('download') : '';
                    const openBtn = previewUrl
                        ? `<a href="${escapeHtml(previewUrl)}" target="_blank" rel="noopener noreferrer" title="Open">${openIcon}</a>`
                        : '';
                    const dlBtn = downloadPath
                        ? `<a href="${escapeHtml(downloadPath)}" download="${escapeHtml(docData.filename || 'document')}" title="Download">${downloadIcon}</a>`
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
                    const botIcon = window.chatUiIconMarkup ? window.chatUiIconMarkup('bot') : '';
                    row.innerHTML = `<div class="avatar bot">${botIcon}</div><div class="bubble bot"></div>`;
                    row.querySelector('.bubble').insertAdjacentHTML('beforeend', cardHTML);
                    chatContent.appendChild(row);
                    chatBox.scrollTop = chatBox.scrollHeight;
                }
            } catch (e) { }
            return;
        } else if (data.event === 'done') {
            _fetchConnectionLost = false;
            chatSetHidden(agentStatusDiv, true);
            setStatusToolIcon(null);
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
