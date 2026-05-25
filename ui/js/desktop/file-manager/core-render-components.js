    // Reusable UI/Render helper functions for File Manager

    function t(key, fallback, vars) {
        if (fallback && typeof fallback === 'object' && !Array.isArray(fallback)) {
            vars = fallback;
            fallback = '';
        }
        if (fm.callbacks && typeof fm.callbacks.t === 'function') {
            const translated = fm.callbacks.t(key, vars || {});
            if (translated && translated !== key) return translated;
        }
        let text = fallback || key;
        Object.entries(vars || {}).forEach(([name, value]) => {
            text = text.replaceAll('{{' + name + '}}', String(value));
            text = text.replaceAll('{' + name + '}', String(value));
        });
        return text;
    }

    function esc(value) {
        if (fm.callbacks && typeof fm.callbacks.esc === 'function') {
            return fm.callbacks.esc(value);
        }
        return String(value == null ? '' : value)
            .replaceAll('&', '&amp;')
            .replaceAll('<', '&lt;')
            .replaceAll('>', '&gt;')
            .replaceAll('"', '&quot;')
            .replaceAll("'", '&#39;');
    }

    function api(url, options) {
        if (fm.callbacks && typeof fm.callbacks.api === 'function') {
            return fm.callbacks.api(url, options);
        }
        const opts = Object.assign({ credentials: 'same-origin', headers: { 'Content-Type': 'application/json' } }, options || {});
        if (opts.body instanceof FormData) delete opts.headers;
        return fetch(url, opts).then(async res => {
            if (!res.ok) {
                let message = res.statusText;
                try { const body = await res.json(); message = body.error || body.message || message; } catch (_) { message = await res.text() || message; }
                throw new Error(message);
            }
            return res.json();
        });
    }

    function isReadonly() {
        return !!(fm.callbacks && fm.callbacks.readonly);
    }

    function maxFileSize() {
        const value = Number(fm.callbacks && fm.callbacks.maxFileSize);
        return Number.isFinite(value) && value > 0 ? value : 0;
    }

    function isTouchLikePointer(event) {
        if (event && (event.pointerType === 'touch' || event.pointerType === 'pen')) return true;
        if (window.matchMedia && window.matchMedia('(hover: none) and (pointer: coarse)').matches) return true;
        return !!(window.matchMedia && window.matchMedia('(max-width: 820px)').matches);
    }

    function wireLongPress(element, callback, options) {
        options = options || {};
        const threshold = Number(options.threshold || 600);
        const feedbackDelay = Number(options.feedbackDelay || 300);
        const moveTolerance = Number(options.moveTolerance || 10);
        let timer = 0;
        let feedbackTimer = 0;
        let startX = 0;
        let startY = 0;
        let pointerId = null;
        let triggered = false;
        let suppressClick = false;

        function clearTimers() {
            if (timer) window.clearTimeout(timer);
            if (feedbackTimer) window.clearTimeout(feedbackTimer);
            timer = 0;
            feedbackTimer = 0;
        }

        function clearPress() {
            clearTimers();
            element.classList.remove('vd-long-press-active');
            pointerId = null;
            triggered = false;
        }

        element.addEventListener('pointerdown', event => {
            if (event.button !== 0 || !isTouchLikePointer(event)) return;
            clearTimers();
            startX = event.clientX;
            startY = event.clientY;
            pointerId = event.pointerId;
            triggered = false;
            feedbackTimer = window.setTimeout(() => {
                element.classList.add('vd-long-press-active');
            }, feedbackDelay);
            timer = window.setTimeout(() => {
                triggered = true;
                suppressClick = true;
                element.classList.add('vd-long-press-active');
                event.preventDefault();
                event.stopPropagation();
                callback(event);
            }, threshold);
        });

        element.addEventListener('pointermove', event => {
            if (!timer || pointerId !== event.pointerId) return;
            if (Math.abs(event.clientX - startX) > moveTolerance || Math.abs(event.clientY - startY) > moveTolerance) {
                clearPress();
            }
        });

        element.addEventListener('pointerup', event => {
            if (pointerId !== event.pointerId) return;
            if (triggered) {
                event.preventDefault();
                event.stopPropagation();
            }
            clearPress();
        });
        element.addEventListener('pointercancel', clearPress);
        element.addEventListener('click', event => {
            if (!suppressClick) return;
            suppressClick = false;
            event.preventDefault();
            event.stopPropagation();
        }, true);
    }

    function iconMarkup(key, fallback, className, size) {
        if (fm.callbacks && typeof fm.callbacks.iconMarkup === 'function') {
            return fm.callbacks.iconMarkup(key, fallback, className, size);
        }
        const pixels = Number(size || 16) || 16;
        return `<span class="${esc(className || '')}" style="font-size:${pixels}px">${esc(fallback || key || '')}</span>`;
    }

    // Natively configured file type icons mapping
    function iconForFile(file) {
        if (fm.callbacks && typeof fm.callbacks.iconForFile === 'function') {
            return fm.callbacks.iconForFile(file);
        }
        const ext = String(file.name || '').split('.').pop().toLowerCase();
        const map = {
            go: 'file-go', js: 'file-js', ts: 'file-js', mjs: 'file-js', jsx: 'file-js', tsx: 'file-js',
            py: 'file-py', rs: 'file-rs', json: 'file-json', yaml: 'file-yaml', yml: 'file-yaml',
            md: 'file-md', html: 'file-html', htm: 'file-html', css: 'file-css', scss: 'file-css', sass: 'file-css',
            png: 'file-image', jpg: 'file-image', jpeg: 'file-image', gif: 'file-image', svg: 'file-image', webp: 'file-image',
            mp4: 'file-video', mkv: 'file-video', avi: 'file-video', mov: 'file-video',
            mp3: 'file-audio', wav: 'file-audio', flac: 'file-audio', ogg: 'file-audio',
            zip: 'file-archive', tar: 'file-archive', gz: 'file-archive', rar: 'file-archive', '7z': 'file-archive',
            pdf: 'file-pdf', doc: 'file-doc', docx: 'file-doc', xls: 'file-xls', xlsx: 'file-xls', ppt: 'file-ppt', pptx: 'file-ppt',
            txt: 'file-text', log: 'file-text', csv: 'file-csv', sql: 'file-sql', dockerfile: 'file-docker',
            sh: 'file-shell', bash: 'file-shell', ps1: 'file-shell', zsh: 'file-shell',
            c: 'file-c', cpp: 'file-cpp', h: 'file-c', hpp: 'file-cpp', cs: 'file-csharp', java: 'file-java', kt: 'file-kotlin',
            php: 'file-php', rb: 'file-ruby', swift: 'file-swift',
        };
        return map[ext] || 'file';
    }

    function fileExt(name) {
        const parts = String(name || '').split('.');
        return parts.length > 1 ? parts.pop().toLowerCase() : '';
    }

    function isPreviewableImage(file) {
        if (!file || file.type !== 'file') return false;
        const mime = String(file.mime_type || '').toLowerCase();
        if (mime && PREVIEW_IMAGE_MIMES.has(mime)) return true;
        return PREVIEW_IMAGE_EXTS.has(fileExt(file.name));
    }

    function previewURL(file) {
        return '/api/desktop/preview?path=' + encodeURIComponent(file.path || '');
    }

    function thumbnailMarkup(file, iconKey, fallback, mode) {
        const icon = iconMarkup(iconKey, fallback, 'fm-thumb-fallback-icon', mode === 'grid' ? 38 : 18);
        if (!isPreviewableImage(file)) return icon;
        return `<span class="fm-thumb fm-thumb-${esc(mode || 'grid')}" aria-hidden="true">
            <img src="${esc(previewURL(file))}" loading="lazy" decoding="async" alt="">
            <span class="fm-thumb-fallback">${icon}</span>
        </span>`;
    }

    function iconForDirectory(name) {
        if (fm.callbacks && typeof fm.callbacks.iconForDirectory === 'function') {
            return fm.callbacks.iconForDirectory(name);
        }
        const lower = String(name || '').toLowerCase();
        if (lower === 'desktop') return 'folder-desktop';
        if (lower === 'documents') return 'folder-documents';
        if (lower === 'downloads') return 'folder-downloads';
        if (lower === 'pictures' || lower === 'images') return 'folder-pictures';
        if (lower === 'music' || lower === 'audio') return 'folder-music';
        if (lower === 'videos' || lower === 'movies') return 'folder-videos';
        if (lower === 'src' || lower === 'source') return 'folder-src';
        if (lower === 'dist' || lower === 'build' || lower === 'out') return 'folder-build';
        if (lower === 'node_modules') return 'folder-npm';
        if (lower === '.git') return 'folder-git';
        if (lower === 'config' || lower === '.config') return 'folder-config';
        if (lower === 'public') return 'folder-public';
        if (lower === 'assets') return 'folder-assets';
        if (lower === 'templates' || lower === 'views') return 'folder-templates';
        if (lower === 'scripts' || lower === 'bin') return 'folder-scripts';
        if (lower === 'test' || lower === 'tests') return 'folder-tests';
        if (lower === '.github') return 'folder-github';
        if (lower === 'workflows') return 'folder-workflows';
        if (lower === 'ui' || lower === 'www' || lower === 'web') return 'folder-ui';
        if (lower === 'internal') return 'folder-internal';
        if (lower === 'cmd') return 'folder-cmd';
        if (lower === 'api') return 'folder-api';
        if (lower === 'pkg') return 'folder-pkg';
        if (lower === 'data') return 'folder-data';
        if (lower === 'db' || lower === 'database' || lower === 'migrations') return 'folder-db';
        if (lower === 'deploy' || lower === 'deployment') return 'folder-deploy';
        if (lower === 'docs' || lower === 'documentation') return 'folder-docs';
        if (lower === 'reports') return 'folder-reports';
        if (lower === 'tools') return 'folder-tools';
        if (lower === 'lib' || lower === 'libs' || lower === 'vendor') return 'folder-lib';
        if (lower === 'agent_workspace' || lower === 'workspace') return 'folder-workspace';
        if (lower === 'logs') return 'folder-logs';
        if (lower === 'secrets' || lower === 'vault') return 'folder-secrets';
        if (lower === 'media') return 'folder-media';
        if (lower === 'backups') return 'folder-backups';
        if (lower === 'tmp' || lower === 'temp') return 'folder-temp';
        return 'folder';
    }

    function contextIconGlyph(icon) {
        const map = {
            'check-square': '\u2713',
            clipboard: '\u2398',
            copy: '\u2398',
            download: '\u2193',
            edit: '\u270e',
            'file-plus': '+',
            'folder-open': '\u25a1',
            'folder-plus': '+',
            info: 'i',
            refresh: '\u21bb',
            scissors: '\u2702',
            sort: '\u2195',
            trash: '\u1f5d1',
            eye: '\ud83d\udc41',
            'eye-off': '\ud83d\udc41\u0338',
            chat: '\ud83d\udcac',
            agent: '\ud83e\udd16',
            terminal: '\u203a',
            archive: '\ud83d\udcc7',
            star: '\u2605',
            link: '\ud83d\udd17'
        };
        return map[icon] || '';
    }
