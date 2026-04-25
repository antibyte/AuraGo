(() => {
    const ICON_VERSION = '20260425b';
    const ICON_BASE_PATH = '/img/chat-ui-icons';
    const DEFAULT_ICON_KEY = 'generic';
    const CHAT_UI_ICON_STYLE_PRESET = 'ai-generated-activity-3d';

    const CHAT_UI_ICON_DEFINITIONS = [
        { key: 'theme-dark', label: 'Dark theme', shape: 'moon-stars', color: '#6d7cff', aliases: ['dark'] },
        { key: 'code', label: 'Code', shape: 'code-brackets', color: '#38bdf8', aliases: [] },
        { key: 'robot', label: 'Robot', shape: 'robot', color: '#22d3ee', aliases: ['logo', 'agent'] },
        { key: 'theme-black-matrix', label: 'Black Matrix theme', shape: 'matrix-grid', color: '#22c55e', aliases: ['black-matrix'] },
        { key: 'play', label: 'Play', shape: 'play', color: '#2dd4bf', aliases: ['resume'] },
        { key: 'blocked', label: 'Blocked', shape: 'lock', color: '#f97316', aliases: ['lock'] },
        { key: 'stop', label: 'Stop', shape: 'stop-square', color: '#ef4444', aliases: [] },
        { key: 'new-chat', label: 'New conversation', shape: 'plus-chat', color: '#22c55e', aliases: ['new', 'plus'] },
        { key: 'clipboard', label: 'Clipboard', shape: 'clipboard', color: '#a78bfa', aliases: ['copy', 'cheatsheet-copy'] },
        { key: 'complete', label: 'Complete', shape: 'check-circle', color: '#22c55e', aliases: ['success', 'check', 'copied'] },
        { key: 'settings', label: 'Settings', shape: 'gear', color: '#94a3b8', aliases: ['gear', 'tool'] },
        { key: 'pause', label: 'Pause', shape: 'pause', color: '#60a5fa', aliases: [] },
        { key: 'bot', label: 'Assistant', shape: 'bot-face', color: '#38bdf8', aliases: ['assistant'] },
        { key: 'folder', label: 'Folder', shape: 'folder', color: '#facc15', aliases: ['drop-folder'] },
        { key: 'document', label: 'Document', shape: 'file-text', color: '#93c5fd', aliases: ['doc', 'file-document'] },
        { key: 'text-file', label: 'Text file', shape: 'file-lines', color: '#cbd5e1', aliases: ['txt', 'text'] },
        { key: 'search', label: 'Search', shape: 'search', color: '#38bdf8', aliases: [] },
        { key: 'edit-document', label: 'Editable document', shape: 'file-pencil', color: '#818cf8', aliases: ['docx', 'doc'] },
        { key: 'generic', label: 'Generic icon', shape: 'spark-tool', color: '#64748b', aliases: ['file', 'unknown'] },
        { key: 'archive', label: 'Archive', shape: 'archive-box', color: '#f59e0b', aliases: ['zip'] },
        { key: 'json', label: 'JSON', shape: 'json-file', color: '#f97316', aliases: [] },
        { key: 'csv', label: 'CSV', shape: 'table-file', color: '#22c55e', aliases: ['yaml', 'table'] },
        { key: 'xml', label: 'XML', shape: 'xml-file', color: '#0ea5e9', aliases: [] },
        { key: 'markdown', label: 'Markdown', shape: 'markdown-file', color: '#6366f1', aliases: ['md'] },
        { key: 'clear', label: 'Clear', shape: 'eraser', color: '#f97316', aliases: ['reset'] },
        { key: 'mood-analytical', label: 'Analytical mood', shape: 'bar-chart', color: '#38bdf8', aliases: ['analytical'] },
        { key: 'in-progress', label: 'In progress', shape: 'spinner', color: '#38bdf8', aliases: ['progress', 'working'] },
        { key: 'activity', label: 'Activity', shape: 'trend-line', color: '#2dd4bf', aliases: [] },
        { key: 'task', label: 'Task', shape: 'task-list', color: '#a78bfa', aliases: [] },
        { key: 'angry', label: 'Angry feedback', shape: 'angry-face', color: '#ef4444', aliases: [] },
        { key: 'retry', label: 'Retry', shape: 'refresh', color: '#38bdf8', aliases: [] },
        { key: 'negative', label: 'Negative feedback', shape: 'thumb-down', color: '#f59e0b', aliases: ['thumbs-down'] },
        { key: 'network', label: 'Network', shape: 'network-nodes', color: '#14b8a6', aliases: [] },
        { key: 'target', label: 'Target', shape: 'target', color: '#f43f5e', aliases: ['focused', 'current-task'] },
        { key: 'theme-ocean', label: 'Ocean theme', shape: 'wave', color: '#0ea5e9', aliases: ['ocean'] },
        { key: 'warning', label: 'Warning', shape: 'shield-alert', color: '#f59e0b', aliases: ['shield', 'cautious'] },
        { key: 'error', label: 'Error', shape: 'x-circle', color: '#ef4444', aliases: ['failed'] },
        { key: 'theme-light', label: 'Light theme', shape: 'sun', color: '#facc15', aliases: ['light'] },
        { key: 'upload', label: 'Upload', shape: 'upload', color: '#22c55e', aliases: ['uploading'] },
        { key: 'theme-cyberwar', label: 'Cyberwar theme', shape: 'circuit-bolt', color: '#06b6d4', aliases: ['cyberwar'] },
        { key: 'vault', label: 'Vault', shape: 'vault', color: '#64748b', aliases: [] },
        { key: 'theme-sandstorm', label: 'Sandstorm theme', shape: 'sand-swirl', color: '#f59e0b', aliases: ['sandstorm'] },
        { key: 'database', label: 'Database', shape: 'database', color: '#60a5fa', aliases: [] },
        { key: 'theme-retro-crt', label: 'Retro CRT theme', shape: 'crt-screen', color: '#34d399', aliases: ['retro-crt'] },
        { key: 'storage', label: 'Storage', shape: 'storage-stack', color: '#94a3b8', aliases: [] },
        { key: 'positive', label: 'Positive feedback', shape: 'thumb-up', color: '#22c55e', aliases: ['thumbs-up'] },
        { key: 'theme-papyrus', label: 'Papyrus theme', shape: 'scroll-paper', color: '#d6a65d', aliases: ['papyrus'] },
        { key: 'theme-lollipop', label: 'Lollipop theme', shape: 'lollipop', color: '#fb7185', aliases: ['lollipop'] },
        { key: 'crying', label: 'Crying feedback', shape: 'cry-face', color: '#60a5fa', aliases: [] },
        { key: 'mobile', label: 'Mobile', shape: 'phone', color: '#818cf8', aliases: [] },
        { key: 'link', label: 'Link', shape: 'link', color: '#38bdf8', aliases: [] },
        { key: 'cloud', label: 'Cloud', shape: 'cloud', color: '#93c5fd', aliases: [] },
        { key: 'mood-cautious', label: 'Cautious mood', shape: 'shield', color: '#f59e0b', aliases: ['cautious'] },
        { key: 'pending', label: 'Pending', shape: 'clock', color: '#fbbf24', aliases: ['todo', 'waiting'] },
        { key: 'theme-dark-sun', label: 'Dark Sun theme', shape: 'eclipse', color: '#f97316', aliases: ['dark-sun'] },
        { key: 'web', label: 'Web', shape: 'globe', color: '#38bdf8', aliases: ['html', 'htm'] },
        { key: 'download', label: 'Download', shape: 'download', color: '#38bdf8', aliases: [] },
        { key: 'performance', label: 'Performance', shape: 'speedometer', color: '#22c55e', aliases: [] },
        { key: 'expand', label: 'Expand', shape: 'expand', color: '#a78bfa', aliases: ['fullscreen'] },
        { key: 'send', label: 'Send', shape: 'send', color: '#2dd4bf', aliases: ['submit'] },
        { key: 'mood-curious', label: 'Curious mood', shape: 'magnifier-star', color: '#38bdf8', aliases: ['curious'] },
        { key: 'info', label: 'Information', shape: 'info-circle', color: '#60a5fa', aliases: [] },
        { key: 'source', label: 'Source', shape: 'source-code', color: '#94a3b8', aliases: ['view-source'] },
        { key: 'diagram', label: 'Diagram', shape: 'diagram', color: '#a78bfa', aliases: ['mermaid'] },
        { key: 'chevron-down', label: 'Expand menu', shape: 'chevron-down', color: '#94a3b8', aliases: ['chevron'] },
        { key: 'cloud-drive', label: 'Cloud drive', shape: 'cloud-drive', color: '#38bdf8', aliases: [] },
        { key: 'attach', label: 'Attachment', shape: 'paperclip', color: '#c084fc', aliases: ['paperclip', 'file-attach'] },
        { key: 'close', label: 'Close', shape: 'close', color: '#ef4444', aliases: ['cancel', 'remove', 'delete'] },
        { key: 'speaker-muted', label: 'Speaker muted', shape: 'speaker-muted', color: '#94a3b8', aliases: ['muted'] },
        { key: 'spreadsheet', label: 'Spreadsheet', shape: 'spreadsheet-file', color: '#22c55e', aliases: ['xlsx', 'xls'] },
        { key: 'mail', label: 'Mail', shape: 'mail', color: '#38bdf8', aliases: [] },
        { key: 'conversation', label: 'Conversation', shape: 'chat-bubbles', color: '#2dd4bf', aliases: ['chat', 'sessions'] },
        { key: 'feedback', label: 'Feedback', shape: 'smile', color: '#fbbf24', aliases: [] },
        { key: 'microphone-wave', label: 'Voice wave', shape: 'microphone-wave', color: '#f472b6', aliases: [] },
        { key: 'bell', label: 'Notifications', shape: 'bell', color: '#facc15', aliases: ['notification'] },
        { key: 'skipped', label: 'Skipped', shape: 'skip-forward', color: '#94a3b8', aliases: [] },
        { key: 'speaker', label: 'Speaker', shape: 'speaker', color: '#22c55e', aliases: ['sound'] },
        { key: 'mood-playful', label: 'Playful mood', shape: 'gamepad', color: '#fb7185', aliases: ['playful'] },
        { key: 'laughing', label: 'Laughing feedback', shape: 'laugh-face', color: '#facc15', aliases: [] },
        { key: 'image', label: 'Image', shape: 'image-file', color: '#22c55e', aliases: ['picture'] },
        { key: 'video', label: 'Video', shape: 'video-file', color: '#f97316', aliases: [] },
        { key: 'voice', label: 'Voice input', shape: 'microphone', color: '#f472b6', aliases: ['microphone', 'mic'] },
        { key: 'document-stack', label: 'Document stack', shape: 'documents', color: '#93c5fd', aliases: ['ppt'] },
        { key: 'amazed', label: 'Amazed feedback', shape: 'amazed-face', color: '#a78bfa', aliases: [] },
        { key: 'recorder', label: 'Recorder', shape: 'record-dot', color: '#ef4444', aliases: [] },
        { key: 'audio-wave', label: 'Audio wave', shape: 'waveform', color: '#06b6d4', aliases: [] },
        { key: 'mood-creative', label: 'Creative mood', shape: 'palette', color: '#fb7185', aliases: ['creative'] },
        { key: 'presentation', label: 'Presentation', shape: 'presentation-file', color: '#f97316', aliases: ['pptx'] },
        { key: 'audio', label: 'Audio', shape: 'music-note', color: '#06b6d4', aliases: ['music'] },
        { key: 'cheatsheet', label: 'Cheat sheet', shape: 'cheatsheet', color: '#a78bfa', aliases: [] },
        { key: 'pdf', label: 'PDF', shape: 'pdf-file', color: '#ef4444', aliases: [] },
        { key: 'notes', label: 'Notes', shape: 'notebook', color: '#f59e0b', aliases: [] },
        { key: 'mood-focused', label: 'Focused mood', shape: 'focus-ring', color: '#f43f5e', aliases: [] },
        { key: 'user', label: 'User', shape: 'user', color: '#60a5fa', aliases: ['human'] },
        { key: 'mood-brain', label: 'Mood', shape: 'brain', color: '#a78bfa', aliases: ['brain', 'mood'] },
        { key: 'plan', label: 'Plan', shape: 'plan', color: '#818cf8', aliases: [] },
        { key: 'journal', label: 'Journal', shape: 'journal', color: '#f59e0b', aliases: [] },
        { key: 'credit-card', label: 'Credits', shape: 'credit-card', color: '#22c55e', aliases: ['credits', 'payment'] },
        { key: 'scroll-down', label: 'Scroll down', shape: 'arrow-down', color: '#2dd4bf', aliases: ['down'] },
        { key: 'more', label: 'More options', shape: 'ellipsis', color: '#94a3b8', aliases: ['menu', 'ellipsis'] },
    ];

    const definitionsByKey = new Map();
    const aliasesByKey = new Map();

    function normalizeIconName(iconName) {
        return String(iconName || DEFAULT_ICON_KEY)
            .trim()
            .toLowerCase()
            .replace(/[^a-z0-9]+/g, '-')
            .replace(/^-+|-+$/g, '') || DEFAULT_ICON_KEY;
    }

    function fileNameFor(definition) {
        return `${definition.key}.png`;
    }

    function getDefinition(iconName) {
        const normalized = normalizeIconName(iconName);
        const canonical = aliasesByKey.get(normalized) || normalized;
        return definitionsByKey.get(canonical) || definitionsByKey.get(DEFAULT_ICON_KEY);
    }

    function getIconUrl(iconName) {
        const definition = getDefinition(iconName);
        return `${ICON_BASE_PATH}/${fileNameFor(definition)}?v=${ICON_VERSION}`;
    }

    function applyIcon(el, iconName) {
        if (!el) return getDefinition(iconName);
        const definition = getDefinition(iconName || el.dataset.chatIcon);
        el.classList.add('chat-ui-icon');
        el.dataset.chatIcon = definition.key;
        el.setAttribute('aria-hidden', 'true');
        el.style.setProperty('--chat-ui-icon-url', `url('${getIconUrl(definition.key)}')`);
        return definition;
    }

    function createIcon(iconName, className = '') {
        const el = document.createElement('span');
        el.className = ['chat-ui-icon', className].filter(Boolean).join(' ');
        applyIcon(el, iconName);
        return el;
    }

    function escapeAttribute(value) {
        return String(value)
            .replace(/&/g, '&amp;')
            .replace(/"/g, '&quot;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;');
    }

    function chatUiIconMarkup(iconName, className = '') {
        const definition = getDefinition(iconName);
        const classes = ['chat-ui-icon', className].filter(Boolean).join(' ');
        const url = getIconUrl(definition.key);
        return `<span class="${escapeAttribute(classes)}" data-chat-icon="${escapeAttribute(definition.key)}" aria-hidden="true" style="--chat-ui-icon-url: url('${escapeAttribute(url)}')"></span>`;
    }

    function hydrate(root = document) {
        if (!root || !root.querySelectorAll) return;
        root.querySelectorAll('[data-chat-icon]').forEach((el) => applyIcon(el, el.dataset.chatIcon));
    }

    CHAT_UI_ICON_DEFINITIONS.forEach((definition) => {
        definitionsByKey.set(definition.key, definition);
        definition.aliases.forEach((alias) => aliasesByKey.set(normalizeIconName(alias), definition.key));
    });

    window.AuraChatIcons = {
        definitions: CHAT_UI_ICON_DEFINITIONS,
        stylePreset: CHAT_UI_ICON_STYLE_PRESET,
        normalizeIconName,
        getDefinition,
        getIconUrl,
        applyIcon,
        createIcon,
        markup: chatUiIconMarkup,
        hydrate,
    };
    window.chatUiIconMarkup = chatUiIconMarkup;

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => hydrate());
    } else {
        hydrate();
    }
})();
