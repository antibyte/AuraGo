async function changePersonality(newId, triggerSelect) {
    try {
        const res = await fetch('/api/personality', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id: newId })
        });
        if (res.ok) {
            const sel = document.getElementById('personality-select');
            if (sel) sel.value = newId;
            // Visual feedback: brief flash on the triggering select
            if (triggerSelect) {
                triggerSelect.style.borderColor = '#22c55e';
                triggerSelect.style.boxShadow = '0 0 6px rgba(34,197,94,0.4)';
                setTimeout(() => {
                    triggerSelect.style.borderColor = '';
                    triggerSelect.style.boxShadow = '';
                }, 800);
            }
            console.log("Personality updated to:", newId);
        } else {
            console.error("Failed to update personality:", res.status);
            if (triggerSelect) {
                triggerSelect.style.borderColor = '#ef4444';
                setTimeout(() => { triggerSelect.style.borderColor = ''; }, 800);
            }
        }
    } catch (err) {
        console.error("Error updating personality:", err);
    }
}

// Personality dropdown toggle (custom, theme-aware)
(function initPersonalityDropdown() {
    const btn = document.getElementById('personality-select');
    const dropdown = document.getElementById('personality-dropdown');
    const mobilePersonalityBtn = document.getElementById('personality-mobile-btn');
    if (!btn || !dropdown) return;

    function togglePersonalityDropdown(e) {
        e.stopPropagation();
        const isOpen = !dropdown.hidden;
        dropdown.hidden = isOpen;
        btn.setAttribute('aria-expanded', String(!isOpen));
        if (isOpen && typeof window._hidePersonalityPreview === 'function') {
            window._hidePersonalityPreview();
        }
    }

    bindHeaderActivation(btn, togglePersonalityDropdown);

    document.addEventListener('click', (e) => {
        const clickedMobilePersonality = mobilePersonalityBtn && mobilePersonalityBtn.contains(e.target);
        if (!btn.contains(e.target) && !dropdown.contains(e.target) && !clickedMobilePersonality) {
            dropdown.hidden = true;
            btn.setAttribute('aria-expanded', 'false');
            if (typeof window._hidePersonalityPreview === 'function') {
                window._hidePersonalityPreview();
            }
        }
    });

    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape' && !dropdown.hidden) {
            dropdown.hidden = true;
            btn.setAttribute('aria-expanded', 'false');
            if (typeof window._hidePersonalityPreview === 'function') {
                window._hidePersonalityPreview();
            }
            btn.focus();
        }
    });

    if (mobilePersonalityBtn) {
        bindHeaderActivation(mobilePersonalityBtn, togglePersonalityDropdown);
    }
})();

// Global handler called from chat-history.js when user picks a personality
window._selectPersonality = function(personalityId) {
    const btn = document.getElementById('personality-select');
    const dropdown = document.getElementById('personality-dropdown');
    const label = document.getElementById('personality-label');
    let selectedPreviewKey = 'custom';

    changePersonality(personalityId, btn);

    if (dropdown) dropdown.hidden = true;
    if (btn) btn.setAttribute('aria-expanded', 'false');
    if (typeof window._hidePersonalityPreview === 'function') {
        window._hidePersonalityPreview();
    }

    if (dropdown) {
        dropdown.querySelectorAll('.personality-option').forEach(opt => {
            const isActive = opt.dataset.value === personalityId;
            opt.classList.toggle('active', isActive);
            opt.setAttribute('aria-selected', String(isActive));
            if (isActive) selectedPreviewKey = opt.dataset.previewKey || 'custom';
        });
    }

    if (typeof window._setActivePersonaIconKey === 'function') {
        window._setActivePersonaIconKey(selectedPreviewKey);
    }

    if (label) {
        label.textContent = personalityId.charAt(0).toUpperCase() + personalityId.slice(1);
    }
};

/* ── Clear session ── */
document.getElementById('clear-btn').addEventListener('click', async () => {
    closeComposerPanel();
    const ok = await showConfirm(
        t('chat.confirm_clear_title'),
        t('chat.confirm_clear_msg')
    );
    if (ok) {
        try {
            const res = await fetch(buildClearUrl(), { method: 'DELETE' });
            if (res.ok) {
                chatContent.innerHTML = '';
                conversation = [];
                hideTodoPanel();
                appendMessage('assistant', t('chat.greeting'));
            }
        } catch (err) {
            console.error("Failed to clear session:", err);
        }
    }
});

/* ── Stop agent ── */
document.getElementById('stop-btn').addEventListener('click', async () => {
    closeComposerPanel();
    const ok = await showConfirm(
        t('chat.confirm_stop_title'),
        t('chat.confirm_stop_msg')
    );
    if (ok) {
        try {
            const sid = typeof getActiveSessionId === 'function' ? getActiveSessionId() : 'default';
            const res = await fetch('/api/admin/stop', {
                method: 'POST',
                headers: { 'X-Session-ID': sid }
            });
            if (res.ok) {
                await showAlert(
                    t('chat.alert_stopped_title'),
                    t('chat.alert_stopped_msg')
                );
            }
        } catch (err) {
            console.error("Failed to stop agent:", err);
        }
    }
});

/* ── File Upload ── */
const fileInput = document.getElementById('file-input');
const uploadBtn = document.getElementById('upload-btn');
const attachChip = document.getElementById('attachment-chip');
const attachName = document.getElementById('attachment-name');
const attachClear = document.getElementById('attachment-clear');
const attachmentsPanel = document.getElementById('attachments-panel');
let pendingAttachments = []; // [{ path, filename, localUrl, kind }]

function _fileExtension(name) {
    const normalized = String(name || '').trim().toLowerCase();
    const idx = normalized.lastIndexOf('.');
    if (idx <= -1 || idx === normalized.length - 1) return '';
    return normalized.slice(idx + 1);
}

function _normalizedAttachmentName(file) {
    const originalName = String(file && file.name ? file.name : '').trim();
    if (originalName) return originalName;

    const mime = String(file && file.type ? file.type : '').toLowerCase();
    const extMap = {
        'image/png': 'png',
        'image/jpeg': 'jpg',
        'image/gif': 'gif',
        'image/webp': 'webp',
        'image/bmp': 'bmp',
        'audio/mpeg': 'mp3',
        'audio/mp3': 'mp3',
        'audio/wav': 'wav',
        'audio/x-wav': 'wav',
        'audio/ogg': 'ogg',
        'audio/webm': 'webm',
        'audio/mp4': 'm4a',
        'application/pdf': 'pdf',
        'application/msword': 'doc',
        'application/vnd.openxmlformats-officedocument.wordprocessingml.document': 'docx',
        'text/plain': 'txt'
    };
    const ext = extMap[mime] || 'bin';

    if (mime.startsWith('image/')) return `pasted-image.${ext}`;
    if (mime.startsWith('audio/')) return `pasted-audio.${ext}`;
    return `pasted-file.${ext}`;
}

function _attachmentKindFromFile(file) {
    const t = (file && file.type) ? file.type.toLowerCase() : '';
    if (t.startsWith('image/')) return 'image';
    if (t.startsWith('audio/')) return 'audio';
    if (t.startsWith('video/')) return 'video';
    const ext = _fileExtension(_normalizedAttachmentName(file));
    if (['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp'].includes(ext)) return 'image';
    if (['mp3', 'wav', 'ogg', 'webm', 'm4a', 'aac', 'flac'].includes(ext)) return 'audio';
    if (['mp4', 'mov', 'avi', 'mkv', 'webm'].includes(ext)) return 'video';
    return 'file';
}

function _makeAttachmentLabel() {
    if (!pendingAttachments.length) return '';
    const first = pendingAttachments[0]?.filename || t('chat.file_sent');
    if (pendingAttachments.length === 1) return first;
    return `${first} (+${pendingAttachments.length - 1})`;
}

function _revokeAttachmentURLs() {
    pendingAttachments.forEach(a => {
        try { if (a.localUrl) URL.revokeObjectURL(a.localUrl); } catch (_e) { }
    });
}

function clearPendingAttachments() {
    _revokeAttachmentURLs();
    pendingAttachments = [];
    if (fileInput) fileInput.value = '';
    chatSetHidden(attachChip, true);
    chatSetHidden(attachmentsPanel, true);
    uploadBtn.classList.remove('has-file');
    if (attachmentsPanel) attachmentsPanel.innerHTML = '';
}

function renderPendingAttachments() {
    if (!pendingAttachments.length) {
        chatSetHidden(attachChip, true);
        chatSetHidden(attachmentsPanel, true);
        uploadBtn.classList.remove('has-file');
        if (attachmentsPanel) attachmentsPanel.innerHTML = '';
        return;
    }

    attachName.textContent = _makeAttachmentLabel();
    chatSetHidden(attachChip, false);
    uploadBtn.classList.add('has-file');

    if (!attachmentsPanel) return;
    chatSetHidden(attachmentsPanel, false);

    const itemsHTML = pendingAttachments.map((a, idx) => {
        const safeName = escapeHtml(a.filename || '');
        const safePath = escapeHtml(a.path || '');
        const removeTitle = escapeAttr(t('chat.attachment_clear_title'));
        let thumb = `<div class="attachment-thumb"></div>`;
        if (a.kind === 'image' && a.localUrl) {
            thumb = `<img class="attachment-thumb" src="${escapeAttr(a.localUrl)}" alt="${safeName}">`;
        } else if (a.kind === 'audio' && a.localUrl) {
            thumb = `<audio class="attachment-thumb" src="${escapeAttr(a.localUrl)}" controls></audio>`;
        } else if (a.kind === 'video' && a.localUrl) {
            thumb = `<video class="attachment-thumb" src="${escapeAttr(a.localUrl)}" controls muted></video>`;
        }
        return `
            <div class="attachment-item" data-idx="${idx}">
                ${thumb}
                <div class="attachment-meta">
                    <div class="attachment-filename" title="${safeName}">${safeName}</div>
                    <div class="attachment-path" title="${safePath}">${safePath}</div>
                </div>
                <button type="button" class="attachment-remove" data-idx="${idx}" title="${removeTitle}">${chatIconMarkup('close')}</button>
            </div>
        `;
    }).join('');

    attachmentsPanel.innerHTML = `<div class="attachments-grid">${itemsHTML}</div>`;
    attachmentsPanel.querySelectorAll('.attachment-remove').forEach(btn => {
        btn.addEventListener('click', () => {
            const idx = Number(btn.dataset.idx);
            const item = pendingAttachments[idx];
            if (item && item.localUrl) {
                try { URL.revokeObjectURL(item.localUrl); } catch (_e) { }
            }
            pendingAttachments = pendingAttachments.filter((_, i) => i !== idx);
            renderPendingAttachments();
        });
    });
}

uploadBtn.addEventListener('click', () => {
    closeComposerPanel();
    fileInput.click();
});

if (cheatsheetPickerBtn) {
    cheatsheetPickerBtn.addEventListener('click', () => {
        openCheatsheetPicker();
    });
}

if (cheatsheetPickerCancelBtn) {
    cheatsheetPickerCancelBtn.addEventListener('click', closeCheatsheetPicker);
}
if (cheatsheetPickerCloseXBtn) {
    cheatsheetPickerCloseXBtn.addEventListener('click', closeCheatsheetPicker);
}
if (cheatsheetPickerOverlay) {
    cheatsheetPickerOverlay.addEventListener('click', (event) => {
        if (event.target === cheatsheetPickerOverlay) {
            closeCheatsheetPicker();
        }
    });
}
if (cheatsheetPickerSendBtn) {
    cheatsheetPickerSendBtn.addEventListener('click', async () => {
        if (!selectedCheatsheetId) return;
        const selectedSheet = cheatsheetPickerItems.find((sheet) => sheet && sheet.id === selectedCheatsheetId);
        if (!selectedSheet) return;
        let fullSheet = selectedSheet;
        try {
            fullSheet = await loadSelectedCheatsheetForAgentMessage();
        } catch (_error) { }
        closeCheatsheetPicker();
        const messageForAgent = buildCheatsheetAgentMessage(fullSheet);
        const visibleMessage = `${t('chat.cheatsheet_picker_sent_prefix')} ${fullSheet.name || selectedSheet.name || t('chat.cheatsheet_picker_unnamed')}`;
        await handleOutgoingMessage(messageForAgent, visibleMessage);
    });
}

attachClear.addEventListener('click', () => {
    clearPendingAttachments();
});

if (composerMoreBtn && composerPanel) {
    composerMoreBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        closeMoodFeedbackRow();
        toggleComposerPanel();
    });
    document.addEventListener('click', (e) => {
        if (composerPanel.classList.contains('is-hidden')) return;
        if (e.target.closest('#composer-panel') || e.target.closest('#composer-more-btn')) return;
        closeComposerPanel();
    });
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') {
            closeComposerPanel();
            closeMoodFeedbackRow();
            closeCheatsheetPicker();
        }
    });
}

/* ── Desktop: auto-open composer panel & handle resize ── */
if (composerPanel) {
    function applyDesktopComposerState() {
        if (isDesktopView()) {
            /* Ensure panel is visible on desktop */
            composerPanel.classList.remove('is-hidden');
        } else {
            /* On mobile, start with panel hidden unless already open */
            if (composerMoreBtn && !composerMoreBtn.classList.contains('is-open')) {
                composerPanel.classList.add('is-hidden');
            }
        }
    }
    /* Apply on load */
    applyDesktopComposerState();
    /* React to viewport changes (e.g. resize, orientation change) */
    _desktopMQ.addEventListener('change', applyDesktopComposerState);
}

if (feedbackToggleBtn && moodFeedbackRow) {
    feedbackToggleBtn.addEventListener('click', () => {
        const willOpen = moodFeedbackRow.classList.contains('is-hidden');
        chatSetHidden(moodFeedbackRow, !willOpen);
        closeComposerPanel();
    });
    document.addEventListener('click', (e) => {
        if (moodFeedbackRow.classList.contains('is-hidden')) return;
        if (e.target.closest('#mood-feedback-row') || e.target.closest('#feedback-toggle-btn')) return;
        closeMoodFeedbackRow();
    });
}

async function uploadSingleAttachment(file) {
    const formData = new FormData();
    formData.append('file', file, _normalizedAttachmentName(file));
    const res = await fetch('/api/upload', { method: 'POST', body: formData });
    if (!res.ok) throw new Error(await res.text());
    const data = await res.json();
    const kind = _attachmentKindFromFile(file);
    const localUrl = (kind !== 'file') ? URL.createObjectURL(file) : null;
    pendingAttachments.push({ path: data.path, filename: data.filename, localUrl, kind });
    renderPendingAttachments();
}

async function queueAttachmentUploads(files) {
    if (!files.length) return;
    uploadBtn.disabled = true;
    try {
        for (const file of files) {
            await uploadSingleAttachment(file);
        }
    } catch (err) {
        console.error('Upload error:', err);
        appendMessage('assistant', t('chat.upload_failed') + err.message);
    } finally {
        uploadBtn.disabled = false;
        fileInput.value = ''; // allow re-upload of same file(s)
    }
}

fileInput.addEventListener('change', async () => {
    const files = Array.from(fileInput.files || []);
    await queueAttachmentUploads(files);
});

userInput.addEventListener('paste', (event) => {
    const clipboard = event.clipboardData;
    if (!clipboard) return;

    const files = Array.from(clipboard.items || [])
        .filter((item) => item && item.kind === 'file')
        .map((item) => item.getAsFile())
        .filter(Boolean);

    if (!files.length) return;

    event.preventDefault();
    void queueAttachmentUploads(files);
});
