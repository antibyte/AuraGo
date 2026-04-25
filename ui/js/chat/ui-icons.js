(() => {
    const ICON_VERSION = '20260425a';
    const ICON_BASE_PATH = '/img/chat-ui-icons';
    const DEFAULT_ICON_KEY = 'generic';

    const CHAT_UI_ICON_DEFINITIONS = [
        { key: 'theme-dark', label: 'Dark theme', sourceSlot: 0, aliases: ['dark'] },
        { key: 'code', label: 'Code', sourceSlot: 1, aliases: [] },
        { key: 'robot', label: 'Robot', sourceSlot: 2, aliases: ['logo', 'agent'] },
        { key: 'theme-black-matrix', label: 'Black Matrix theme', sourceSlot: 3, aliases: ['black-matrix'] },
        { key: 'play', label: 'Play', sourceSlot: 4, aliases: ['resume'] },
        { key: 'blocked', label: 'Blocked', sourceSlot: 5, aliases: ['lock'] },
        { key: 'stop', label: 'Stop', sourceSlot: 6, aliases: [] },
        { key: 'new-chat', label: 'New conversation', sourceSlot: 7, aliases: ['new', 'plus'] },
        { key: 'clipboard', label: 'Clipboard', sourceSlot: 8, aliases: ['copy', 'cheatsheet-copy'] },
        { key: 'complete', label: 'Complete', sourceSlot: 9, aliases: ['success', 'check', 'copied'] },
        { key: 'settings', label: 'Settings', sourceSlot: 10, aliases: ['gear', 'tool'] },
        { key: 'pause', label: 'Pause', sourceSlot: 11, aliases: [] },
        { key: 'bot', label: 'Assistant', sourceSlot: 12, aliases: ['assistant'] },
        { key: 'folder', label: 'Folder', sourceSlot: 13, aliases: ['drop-folder'] },
        { key: 'document', label: 'Document', sourceSlot: 14, aliases: ['doc', 'file-document'] },
        { key: 'text-file', label: 'Text file', sourceSlot: 15, aliases: ['txt', 'text'] },
        { key: 'search', label: 'Search', sourceSlot: 16, aliases: [] },
        { key: 'edit-document', label: 'Editable document', sourceSlot: 17, aliases: ['docx', 'doc'] },
        { key: 'generic', label: 'Generic icon', sourceSlot: 18, aliases: ['file', 'unknown'] },
        { key: 'archive', label: 'Archive', sourceSlot: 19, aliases: ['zip'] },
        { key: 'json', label: 'JSON', sourceSlot: 20, aliases: [] },
        { key: 'csv', label: 'CSV', sourceSlot: 21, aliases: ['yaml', 'table'] },
        { key: 'xml', label: 'XML', sourceSlot: 22, aliases: [] },
        { key: 'markdown', label: 'Markdown', sourceSlot: 23, aliases: ['md'] },
        { key: 'clear', label: 'Clear', sourceSlot: 24, aliases: ['reset'] },
        { key: 'mood-analytical', label: 'Analytical mood', sourceSlot: 25, aliases: ['analytical'] },
        { key: 'in-progress', label: 'In progress', sourceSlot: 26, aliases: ['progress', 'working'] },
        { key: 'activity', label: 'Activity', sourceSlot: 27, aliases: [] },
        { key: 'task', label: 'Task', sourceSlot: 28, aliases: [] },
        { key: 'angry', label: 'Angry feedback', sourceSlot: 29, aliases: [] },
        { key: 'retry', label: 'Retry', sourceSlot: 30, aliases: [] },
        { key: 'negative', label: 'Negative feedback', sourceSlot: 31, aliases: ['thumbs-down'] },
        { key: 'network', label: 'Network', sourceSlot: 32, aliases: [] },
        { key: 'target', label: 'Target', sourceSlot: 33, aliases: ['focused', 'current-task'] },
        { key: 'theme-ocean', label: 'Ocean theme', sourceSlot: 34, aliases: ['ocean'] },
        { key: 'warning', label: 'Warning', sourceSlot: 35, aliases: ['shield', 'cautious'] },
        { key: 'error', label: 'Error', sourceSlot: 36, aliases: ['failed'] },
        { key: 'theme-light', label: 'Light theme', sourceSlot: 37, aliases: ['light'] },
        { key: 'upload', label: 'Upload', sourceSlot: 38, aliases: ['uploading'] },
        { key: 'theme-cyberwar', label: 'Cyberwar theme', sourceSlot: 39, aliases: ['cyberwar'] },
        { key: 'vault', label: 'Vault', sourceSlot: 40, aliases: [] },
        { key: 'theme-sandstorm', label: 'Sandstorm theme', sourceSlot: 41, aliases: ['sandstorm'] },
        { key: 'database', label: 'Database', sourceSlot: 42, aliases: [] },
        { key: 'theme-retro-crt', label: 'Retro CRT theme', sourceSlot: 43, aliases: ['retro-crt'] },
        { key: 'storage', label: 'Storage', sourceSlot: 44, aliases: [] },
        { key: 'positive', label: 'Positive feedback', sourceSlot: 45, aliases: ['thumbs-up'] },
        { key: 'theme-papyrus', label: 'Papyrus theme', sourceSlot: 46, aliases: ['papyrus'] },
        { key: 'theme-lollipop', label: 'Lollipop theme', sourceSlot: 47, aliases: ['lollipop'] },
        { key: 'crying', label: 'Crying feedback', sourceSlot: 48, aliases: [] },
        { key: 'mobile', label: 'Mobile', sourceSlot: 49, aliases: [] },
        { key: 'link', label: 'Link', sourceSlot: 50, aliases: [] },
        { key: 'cloud', label: 'Cloud', sourceSlot: 51, aliases: [] },
        { key: 'mood-cautious', label: 'Cautious mood', sourceSlot: 52, aliases: ['cautious'] },
        { key: 'pending', label: 'Pending', sourceSlot: 53, aliases: ['todo', 'waiting'] },
        { key: 'theme-dark-sun', label: 'Dark Sun theme', sourceSlot: 54, aliases: ['dark-sun'] },
        { key: 'web', label: 'Web', sourceSlot: 55, aliases: ['html', 'htm'] },
        { key: 'download', label: 'Download', sourceSlot: 56, aliases: [] },
        { key: 'performance', label: 'Performance', sourceSlot: 57, aliases: [] },
        { key: 'expand', label: 'Expand', sourceSlot: 58, aliases: ['fullscreen'] },
        { key: 'send', label: 'Send', sourceSlot: 59, aliases: ['submit'] },
        { key: 'mood-curious', label: 'Curious mood', sourceSlot: 60, aliases: ['curious'] },
        { key: 'info', label: 'Information', sourceSlot: 61, aliases: [] },
        { key: 'source', label: 'Source', sourceSlot: 62, aliases: ['view-source'] },
        { key: 'diagram', label: 'Diagram', sourceSlot: 63, aliases: ['mermaid'] },
        { key: 'chevron-down', label: 'Expand menu', sourceSlot: 64, aliases: ['chevron'] },
        { key: 'cloud-drive', label: 'Cloud drive', sourceSlot: 65, aliases: [] },
        { key: 'attach', label: 'Attachment', sourceSlot: 66, aliases: ['paperclip', 'file-attach'] },
        { key: 'close', label: 'Close', sourceSlot: 67, aliases: ['cancel', 'remove', 'delete'] },
        { key: 'speaker-muted', label: 'Speaker muted', sourceSlot: 68, aliases: ['muted'] },
        { key: 'spreadsheet', label: 'Spreadsheet', sourceSlot: 69, aliases: ['xlsx', 'xls'] },
        { key: 'mail', label: 'Mail', sourceSlot: 70, aliases: [] },
        { key: 'conversation', label: 'Conversation', sourceSlot: 71, aliases: ['chat', 'sessions'] },
        { key: 'feedback', label: 'Feedback', sourceSlot: 72, aliases: [] },
        { key: 'microphone-wave', label: 'Voice wave', sourceSlot: 73, aliases: [] },
        { key: 'bell', label: 'Notifications', sourceSlot: 74, aliases: ['notification'] },
        { key: 'skipped', label: 'Skipped', sourceSlot: 75, aliases: [] },
        { key: 'speaker', label: 'Speaker', sourceSlot: 76, aliases: ['sound'] },
        { key: 'mood-playful', label: 'Playful mood', sourceSlot: 77, aliases: ['playful'] },
        { key: 'laughing', label: 'Laughing feedback', sourceSlot: 78, aliases: [] },
        { key: 'image', label: 'Image', sourceSlot: 79, aliases: ['picture'] },
        { key: 'video', label: 'Video', sourceSlot: 80, aliases: [] },
        { key: 'voice', label: 'Voice input', sourceSlot: 81, aliases: ['microphone', 'mic'] },
        { key: 'document-stack', label: 'Document stack', sourceSlot: 82, aliases: ['ppt'] },
        { key: 'amazed', label: 'Amazed feedback', sourceSlot: 83, aliases: [] },
        { key: 'recorder', label: 'Recorder', sourceSlot: 84, aliases: [] },
        { key: 'audio-wave', label: 'Audio wave', sourceSlot: 85, aliases: [] },
        { key: 'mood-creative', label: 'Creative mood', sourceSlot: 86, aliases: ['creative'] },
        { key: 'presentation', label: 'Presentation', sourceSlot: 87, aliases: ['pptx'] },
        { key: 'audio', label: 'Audio', sourceSlot: 88, aliases: ['music'] },
        { key: 'cheatsheet', label: 'Cheat sheet', sourceSlot: 89, aliases: [] },
        { key: 'pdf', label: 'PDF', sourceSlot: 90, aliases: [] },
        { key: 'notes', label: 'Notes', sourceSlot: 91, aliases: [] },
        { key: 'mood-focused', label: 'Focused mood', sourceSlot: 92, aliases: [] },
        { key: 'user', label: 'User', sourceSlot: 93, aliases: ['human'] },
        { key: 'mood-brain', label: 'Mood', sourceSlot: 94, aliases: ['brain', 'mood'] },
        { key: 'plan', label: 'Plan', sourceSlot: 95, aliases: [] },
        { key: 'journal', label: 'Journal', sourceSlot: 96, aliases: [] },
        { key: 'credit-card', label: 'Credits', sourceSlot: 97, aliases: ['credits', 'payment'] },
        { key: 'scroll-down', label: 'Scroll down', sourceSlot: 98, aliases: ['down'] },
        { key: 'more', label: 'More options', sourceSlot: 99, aliases: ['menu', 'ellipsis'] },
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
