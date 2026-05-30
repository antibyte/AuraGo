(function () {
    'use strict';

    const DesktopChatRenderer = {
        seenSSEImages: new Set(),
        seenSSEVideos: new Set(),
        seenSSELiveStreams: new Set(),
        seenSSEAudios: new Set(),
        seenSSEDocuments: new Set(),
        _md: null,
        _lightbox: null,
        _audioQueue: [],
        _audioPlaying: false,
        _currentAudio: null,

        escapeHtml(str) {
            return window.AuraChatCore.escapeHtml(str);
        },

        escapeAttr(s) {
            return window.AuraChatCore.escapeAttr(s);
        },

        translate(key, fallback) {
            const translator = window.t || (() => '');
            const translated = translator(key);
            return translated && translated !== key ? translated : fallback;
        },

        normalizeChatTimestamp(timestamp) {
            return window.AuraChatCore.normalizeTimestamp(timestamp);
        },

        formatChatTimestamp(timestamp) {
            return window.AuraChatCore.formatTimestamp(timestamp);
        },

        appendTimestamp(chatLog, role, timestamp) {
            if (!chatLog) return null;
            const previous = chatLog.lastElementChild;
            if (previous && previous.classList && previous.classList.contains('vd-chat-entry-time')) {
                return previous;
            }
            const date = this.normalizeChatTimestamp(timestamp);
            const el = document.createElement('div');
            el.className = 'vd-chat-entry-time ' + (role === 'user' ? 'user' : 'agent');
            el.setAttribute('data-chat-timestamp', date.toISOString());
            el.textContent = this.formatChatTimestamp(date);
            chatLog.appendChild(el);
            return el;
        },

        getMarkdown() {
            if (this._md) return this._md;
            this._md = window.AuraChatCore.createMarkdownRenderer();
            return this._md;
        },

        stripLeakedToolMarkup(text) {
            return window.AuraChatCore.stripLeakedToolMarkup(text);
        },

        containsLeakedToolMarkup(text) {
            return window.AuraChatCore.containsLeakedToolMarkup(text);
        },

        sanitizeHTML(html) {
            return window.AuraChatCore.sanitizeRenderedHTML(html);
        },

        renderMarkdown(text) {
            const md = this.getMarkdown();
            if (!md) return this.escapeHtml(text);
            const preparedDisplay = window.AuraChatCore.prepareDisplayContent(text, false);
            let displayContent = preparedDisplay.displayContent;
            const isTechnical = preparedDisplay.isTechnical;
            if (isTechnical) {
                return '<pre>' + this.escapeHtml(displayContent) + '</pre>';
            }
            if (!displayContent) return '';
            if (this.seenSSEImages.size > 0) {
                displayContent = window.AuraChatCore.removeSeenMarkdownImages(displayContent, this.seenSSEImages);
                if (!displayContent) return '';
            }
            const prepared = window.AuraChatCore.prepareMarkdownContent(displayContent);
            const contentForRender = prepared.contentForRender;
            const thinkingBlocks = prepared.thinkingBlocks;
            let finalHTML = md.render(contentForRender);
            finalHTML = window.AuraChatCore.applyMarkdownLinkTargets(finalHTML);
            const renderThinkingBlock = (innerText) => {
                const innerHtml = md.render(innerText);
                const label = this.translate('chat.thinking_label', 'Reasoning');
                return '<details class="vd-thinking-block"><summary>' + label + '</summary><div class="vd-thinking-content">' + innerHtml + '</div></details>';
            };
            finalHTML = window.AuraChatCore.replaceThinkingPlaceholders(finalHTML, thinkingBlocks, renderThinkingBlock);
            finalHTML = this.sanitizeHTML(finalHTML);
            return finalHTML;
        },

        createBubble(role, innerHTML) {
            const bubble = document.createElement('div');
            bubble.className = 'vd-chat-bubble ' + role;
            if (typeof innerHTML === 'string') {
                bubble.innerHTML = innerHTML;
            }
            return bubble;
        },

        appendAvatar(chatLog, role, bubble, isContinuation) {
            if (role === 'user') {
                chatLog.appendChild(bubble);
                return;
            }
            const row = document.createElement('div');
            row.className = 'vd-chat-message-row agent';
            if (isContinuation) {
                const spacer = document.createElement('div');
                spacer.className = 'vd-chat-avatar-hidden';
                row.appendChild(spacer);
            } else {
                const avatar = document.createElement('div');
                avatar.className = 'vd-chat-avatar';
                if (window.AuraChatCore && typeof window.AuraChatCore.personaAvatarMarkup === 'function') {
                    avatar.innerHTML = window.AuraChatCore.personaAvatarMarkup('agent');
                } else {
                    avatar.textContent = '🤖';
                }
                row.appendChild(avatar);
            }
            row.appendChild(bubble);
            chatLog.appendChild(row);
        },

        appendRichBubble(chatLog, role, text, prevRole) {
            const bubble = this.createBubble(role, '');
            const isGroup = prevRole === role;

            if (isGroup) {
                bubble.dataset.roleGroup = 'continuation';
            }

            if (role === 'user') {
                bubble.textContent = text;
            } else {
                bubble.innerHTML = this.renderMarkdown(text);
                this.processImages(bubble);
                this.enhanceCodeBlocks(bubble);
                if (window.MermaidLoader) {
                    window.MermaidLoader.processBlocks(bubble);
                }
            }

            this.appendMessageActions(bubble, role);

            if (role === 'agent') {
                this.appendAvatar(chatLog, role, bubble, isGroup);
            } else {
                chatLog.appendChild(bubble);
            }

            const stamp = this.appendTimestamp(chatLog, role);
            (stamp || bubble).scrollIntoView({ block: 'end', behavior: 'smooth' });
            return bubble;
        },

        enhanceCodeBlocks(bubble) {
            if (!bubble) return;
            const codeBlocks = bubble.querySelectorAll('pre > code');
            codeBlocks.forEach(code => {
                const pre = code.parentElement;
                if (!pre || pre.querySelector('.vd-chat-code-block-header')) return;

                const langMatch = (code.className || '').match(/(?:language-|lang-)(\w[\w+-]*)/);
                const lang = langMatch ? langMatch[1] : '';

                const header = document.createElement('div');
                header.className = 'vd-chat-code-block-header';

                const langLabel = document.createElement('span');
                langLabel.className = 'vd-chat-code-lang';
                langLabel.textContent = lang || 'code';

                const copyBtn = document.createElement('button');
                copyBtn.className = 'vd-chat-code-copy';
                copyBtn.type = 'button';
                copyBtn.textContent = this.translate('desktop.copy', 'Copy');
                copyBtn.addEventListener('click', () => {
                    const text = code.textContent || '';
                    navigator.clipboard.writeText(text).then(() => {
                        copyBtn.textContent = this.translate('desktop.copied', 'Copied!');
                        copyBtn.classList.add('copied');
                        setTimeout(() => {
                            copyBtn.textContent = this.translate('desktop.copy', 'Copy');
                            copyBtn.classList.remove('copied');
                        }, 2000);
                    }).catch(() => {});
                });

                header.appendChild(langLabel);
                header.appendChild(copyBtn);
                pre.insertBefore(header, pre.firstChild);
            });
        },

        appendMessageActions(bubble, role) {
            const actions = document.createElement('div');
            actions.className = 'vd-chat-actions';

            const copyBtn = document.createElement('button');
            copyBtn.className = 'vd-chat-action-btn';
            copyBtn.type = 'button';
            copyBtn.title = this.translate('desktop.copy', 'Copy');
            copyBtn.setAttribute('aria-label', this.translate('desktop.copy', 'Copy'));
            copyBtn.innerHTML = '<span class="vd-chat-action-icon">📋</span>';
            copyBtn.addEventListener('click', (event) => {
                event.stopPropagation();
                const text = bubble.textContent || '';
                navigator.clipboard.writeText(text).then(() => {
                    copyBtn.querySelector('.vd-chat-action-icon').textContent = '✅';
                    setTimeout(() => {
                        copyBtn.querySelector('.vd-chat-action-icon').textContent = '📋';
                    }, 1500);
                }).catch(() => {});
            });
            actions.appendChild(copyBtn);

            if (role === 'user') {
                const editBtn = document.createElement('button');
                editBtn.className = 'vd-chat-action-btn';
                editBtn.type = 'button';
                editBtn.title = this.translate('desktop.edit', 'Edit');
                editBtn.setAttribute('aria-label', this.translate('desktop.edit', 'Edit'));
                editBtn.innerHTML = '<span class="vd-chat-action-icon">✏️</span>';
                editBtn.addEventListener('click', (event) => {
                    event.stopPropagation();
                    const chat = bubble.closest('.vd-chat');
                    if (!chat) return;
                    const input = chat.querySelector('.vd-chat-input');
                    if (input) {
                        input.value = bubble.textContent || '';
                        input.dispatchEvent(new Event('input', { bubbles: true }));
                        input.focus();
                    }
                });
                actions.appendChild(editBtn);
            }

            if (role === 'agent') {
                const retryBtn = document.createElement('button');
                retryBtn.className = 'vd-chat-action-btn';
                retryBtn.type = 'button';
                retryBtn.title = this.translate('desktop.retry', 'Retry');
                retryBtn.setAttribute('aria-label', this.translate('desktop.retry', 'Retry'));
                retryBtn.innerHTML = '<span class="vd-chat-action-icon">🔄</span>';
                retryBtn.addEventListener('click', (event) => {
                    event.stopPropagation();
                    const chat = bubble.closest('.vd-chat');
                    if (!chat) return;
                    const chatLog = chat.querySelector('.vd-chat-log');
                    if (!chatLog) return;
                    const userBubbles = chatLog.querySelectorAll('.vd-chat-bubble.user, .vd-chat-message-row.user .vd-chat-bubble');
                    const lastUser = userBubbles[userBubbles.length - 1];
                    if (lastUser) {
                        const host = chat.closest('[data-window-content]') || chat.parentElement;
                        if (host && window.AgentChatApp && typeof window.AgentChatApp.render === 'function') {
                            const text = lastUser.textContent || '';
                            if (text) {
                                const input = chat.querySelector('.vd-chat-input');
                                if (input) {
                                    input.value = text;
                                    input.dispatchEvent(new Event('input', { bubbles: true }));
                                }
                            }
                        }
                    }
                });
                actions.appendChild(retryBtn);
            }

            bubble.appendChild(actions);
        },

        processImages(container) {
            const images = container.querySelectorAll('img');
            images.forEach(img => {
                img.style.cursor = 'pointer';
                img.classList.add('vd-chat-zoomable');
                img.addEventListener('click', () => this.openLightbox(img.src, img.alt || ''));
            });
        },

        ensureLightbox() {
            if (this._lightbox) return this._lightbox;
            const lb = document.createElement('div');
            lb.className = 'vd-lightbox-overlay';
            lb.hidden = true;
            lb.innerHTML = '<div class="vd-lightbox-backdrop"></div><div class="vd-lightbox-content"><img class="vd-lightbox-img" src="" alt=""><button class="vd-lightbox-close" type="button">&times;</button></div>';
            document.body.appendChild(lb);
            lb.querySelector('.vd-lightbox-backdrop').addEventListener('click', () => this.closeLightbox());
            lb.querySelector('.vd-lightbox-close').addEventListener('click', () => this.closeLightbox());
            document.addEventListener('keydown', (e) => {
                if (e.key === 'Escape' && !lb.hidden) this.closeLightbox();
            });
            this._lightbox = lb;
            return lb;
        },

        openLightbox(src, alt) {
            const lb = this.ensureLightbox();
            const img = lb.querySelector('.vd-lightbox-img');
            img.src = src;
            img.alt = alt;
            lb.hidden = false;
        },

        closeLightbox() {
            if (this._lightbox) this._lightbox.hidden = true;
        },

        appendImageMessage(chatLog, imgData) {
            if (!imgData || !imgData.path || this.seenSSEImages.has(imgData.path)) return;
            this.seenSSEImages.add(imgData.path);
            const cap = this.escapeHtml(imgData.caption || '');
            const bubble = this.createBubble('agent', '');
            const img = document.createElement('img');
            img.className = 'vd-chat-zoomable';
            img.src = imgData.path;
            img.alt = cap;
            img.title = cap;
            img.loading = 'lazy';
            img.style.maxWidth = '100%';
            img.style.borderRadius = '8px';
            img.style.cursor = 'pointer';
            img.addEventListener('click', () => this.openLightbox(imgData.path, cap));
            bubble.appendChild(img);
            this.appendMessageActions(bubble, 'agent');
            this.appendAvatar(chatLog, 'agent', bubble);
            const stamp = this.appendTimestamp(chatLog, 'agent');
            (stamp || bubble).scrollIntoView({ block: 'end', behavior: 'smooth' });
        },

        appendVideoMessage(chatLog, videoData) {
            if (!videoData || !videoData.path || this.seenSSEVideos.has(videoData.path)) return;
            this.seenSSEVideos.add(videoData.path);
            const bubble = this.createBubble('agent', '');
            const video = document.createElement('video');
            video.controls = true;
            video.style.maxWidth = '100%';
            video.style.borderRadius = '8px';
            if (videoData.title) video.title = videoData.title;
            const source = document.createElement('source');
            source.src = videoData.path;
            source.type = String(videoData.mime_type || ((window.AuraChatCore && typeof window.AuraChatCore.videoMimeTypeForPath === 'function')
                ? window.AuraChatCore.videoMimeTypeForPath(videoData.path)
                : 'video/mp4'));
            video.appendChild(source);
            bubble.appendChild(video);
            this.appendMessageActions(bubble, 'agent');
            this.appendAvatar(chatLog, 'agent', bubble);
            const stamp = this.appendTimestamp(chatLog, 'agent');
            (stamp || bubble).scrollIntoView({ block: 'end', behavior: 'smooth' });
        },

        appendLiveStreamMessage(chatLog, streamData) {
            const key = streamData && (streamData.path || streamData.stream_url || streamData.message);
            if (!streamData || !key || this.seenSSELiveStreams.has(key)) return;
            this.seenSSELiveStreams.add(key);

            const bubble = this.createBubble('agent', '');
            const wrapper = document.createElement('div');
            wrapper.className = 'chat-video-wrapper chat-live-stream-wrapper';

            const title = String(streamData.title || 'Live stream').trim();
            if (title) {
                const titleEl = document.createElement('div');
                titleEl.className = 'chat-video-title';
                titleEl.textContent = title;
                wrapper.appendChild(titleEl);
            }

            if (streamData.path) {
                const img = document.createElement('img');
                img.className = 'chat-video-player chat-live-stream';
                img.src = streamData.path;
                img.alt = title || 'Live stream';
                img.style.maxWidth = '100%';
                img.style.borderRadius = '8px';
                wrapper.appendChild(img);
            } else {
                const msg = document.createElement('div');
                msg.className = 'chat-video-actions';
                msg.textContent = String(streamData.message || streamData.error || streamData.stream_url || '').trim();
                wrapper.appendChild(msg);
            }

            bubble.appendChild(wrapper);
            this.appendMessageActions(bubble, 'agent');
            this.appendAvatar(chatLog, 'agent', bubble);
            const stamp = this.appendTimestamp(chatLog, 'agent');
            (stamp || bubble).scrollIntoView({ block: 'end', behavior: 'smooth' });
        },

        _playNextAudio() {
            if (!this._audioQueue.length) {
                this._audioPlaying = false;
                this._currentAudio = null;
                return;
            }
            this._audioPlaying = true;
            const src = this._audioQueue.shift();
            const audio = new Audio(src);
            this._currentAudio = audio;
            let advanced = false;
            const next = () => {
                if (advanced) return;
                advanced = true;
                if (this._currentAudio === audio) this._currentAudio = null;
                this._playNextAudio();
            };
            audio.addEventListener('ended', next, { once: true });
            audio.addEventListener('error', next, { once: true });
            audio.play().catch(next);
        },

        enqueueAudioAutoPlay(src) {
            if (!src) return;
            this._audioQueue.push(src);
            if (!this._audioPlaying) this._playNextAudio();
        },

        appendAudioMessage(chatLog, audioData) {
            if (!audioData || !audioData.path || this.seenSSEAudios.has(audioData.path)) return;
            this.seenSSEAudios.add(audioData.path);
            this.enqueueAudioAutoPlay(audioData.path);
        },

        appendDocumentMessage(chatLog, docData) {
            if (!docData || !docData.path || this.seenSSEDocuments.has(docData.path)) return;
            this.seenSSEDocuments.add(docData.path);
            const title = this.escapeHtml(docData.title || docData.filename || this.translate('desktop.chat_document', 'Document'));
            const fmt = this.escapeHtml((docData.format || '').toUpperCase() || 'FILE');
            const downloadPath = docData.path;
            const card = document.createElement('div');
            card.className = 'vd-chat-document-card';
            card.innerHTML =
                '<div class="vd-chat-document-icon"><span class="vd-chat-doc-fmt">' + fmt + '</span></div>' +
                '<div class="vd-chat-document-info"><div class="vd-chat-document-title">' + title + '</div>' +
                '<div class="vd-chat-document-format">' + fmt + '</div></div>' +
                '<div class="vd-chat-document-actions">' +
                (downloadPath ? '<a href="' + this.escapeAttr(downloadPath) + '" download="' + this.escapeHtml(docData.filename || 'document') + '" title="' + this.escapeHtml(this.translate('desktop.media_download', 'Download')) + '">&#11015;</a>' : '') +
                '</div>';
            const bubble = this.createBubble('agent', '');
            bubble.appendChild(card);
            this.appendMessageActions(bubble, 'agent');
            this.appendAvatar(chatLog, 'agent', bubble);
            const stamp = this.appendTimestamp(chatLog, 'agent');
            (stamp || bubble).scrollIntoView({ block: 'end', behavior: 'smooth' });
        },

        createThinkingStatus() {
            const el = document.createElement('div');
            el.className = 'vd-chat-status';
            el.innerHTML = '<span class="vd-chat-status-dot"></span><span class="vd-chat-status-text">' + this.escapeHtml(this.translate('desktop.thinking', 'Working...')) + '</span>';
            return el;
        },

        updateStatus(statusEl, text) {
            if (!statusEl) return;
            const textEl = statusEl.querySelector('.vd-chat-status-text');
            if (textEl) textEl.textContent = text;
        },

        formatAgentActionStatus(data) {
            if (!data) return '';
            const event = data.event || data.type || '';
            const detail = String(data.detail || data.message || '').trim();
            if (event === 'thinking') {
                return detail || this.translate('chat.sse_thinking', this.translate('desktop.chat_thinking', 'Reasoning...'));
            }
            if (event === 'agent_action') {
                const payload = data.payload && typeof data.payload === 'object' ? data.payload : data;
                const toolName = String(payload.tool_name || payload.toolName || detail || 'tool').trim();
                const state = String(payload.state || '').toLowerCase();
                if (state === 'started') {
                    return this.translate('chat.sse_tool_start', this.translate('desktop.chat_using_tool', 'Using tool') + ': ') + toolName;
                }
                if (state === 'succeeded' || state === 'sanitized') {
                    return this.translate('chat.sse_tool_end', 'Tool completed: ') + toolName;
                }
                if (state === 'failed' || state === 'blocked' || state === 'cancelled') {
                    return this.translate('chat.sse_error_recovery', 'Script had an error. Fixing code...');
                }
                return '';
            }
            if (event === 'tool_start') {
                if (detail === 'co_agent' || detail === 'co_agents') return '';
                if (detail === 'list_skills') {
                    return this.translate('chat.sse_list_skills', 'Checking available skills...');
                }
                if (detail === 'execute_skill') {
                    return this.translate('chat.sse_execute_skill', 'Running skill: ') + detail;
                }
                return this.translate('chat.sse_tool_start', this.translate('desktop.chat_using_tool', 'Using tool') + ': ') + detail;
            }
            if (event === 'tool_end') {
                if (detail === 'co_agent' || detail === 'co_agents') return '';
                return this.translate('chat.sse_tool_end', 'Tool completed: ') + detail;
            }
            if (event === 'co_agent_spawn') {
                return this.translate('chat.sse_co_agent_spawn', 'Co-Agent started: ') + detail;
            }
            if (event === 'workflow_plan') {
                return this.translate('chat.sse_workflow_plan', 'Planning next steps');
            }
            if (event === 'coding') {
                return this.translate('chat.sse_coding', 'Writing and testing a script...');
            }
            if (event === 'error_recovery') {
                return this.translate('chat.sse_error_recovery', 'Script had an error. Fixing code...');
            }
            return '';
        },

        extractToolCallNarration(text) {
            let narration = String(text || '')
                .replace(/```json[\s\S]*?```/g, '')
                .replace(/`[^`]*`/g, '')
                .replace(/\{[\s\S]*"action"\s*:[\s\S]*/g, '')
                .replace(/\{[\s\S]*"tool"\s*:[\s\S]*/g, '')
                .replace(/\{[\s\S]*"tool_call"\s*:[\s\S]*/g, '')
                .replace(/\{[\s\S]*"tool_name"\s*:[\s\S]*/g, '')
                .replace(/\{[\s\S]*"parameters"\s*:[\s\S]*/g, '')
                .trim();
            narration = this.stripLeakedToolMarkup(narration);
            return narration && narration.split(/\s+/).filter(Boolean).length >= 6 ? narration : '';
        },

        createOpenInAppButton(appId, path) {
            const appNames = {
                writer: this.translate('desktop.app_writer', 'Writer'),
                sheets: this.translate('desktop.app_sheets', 'Sheets'),
                'code-studio': this.translate('desktop.app_code_studio', 'Code Studio')
            };
            const label = appNames[appId] || appId;
            const btn = document.createElement('button');
            btn.className = 'vd-chat-open-app-btn';
            btn.textContent = this.translate('desktop.chat_open_in', 'Open in') + ' ' + label;
            btn.dataset.appId = appId;
            if (path) btn.dataset.path = path;
            return btn;
        },

        resetDedupSets() {
            this.seenSSEImages.clear();
            this.seenSSEVideos.clear();
            this.seenSSELiveStreams.clear();
            this.seenSSEAudios.clear();
            this.seenSSEDocuments.clear();
        }
    };

    window.DesktopChatRenderer = DesktopChatRenderer;
})();
