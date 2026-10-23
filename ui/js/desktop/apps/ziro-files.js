(function (root) {
    'use strict';
    const MAX_BYTES = 200 * 1024 * 1024;
    const MAX_FILES = 2000;
    function relative(path) {
        if (typeof path !== 'string' || !path || path.length > 1024 || /[\\:\x00-\x1f]/.test(path)
            || path.split('/').some(part => !part || part === '.' || part === '..' || /[. ]$/.test(part))) {
            throw new Error('ziro.invalid_path');
        }
        return path;
    }
    function shared(path) {
        relative(path);
        if (!path.startsWith('Shared/')) throw new Error('ziro.shared_only');
        return path;
    }
    function filesChecked(files) {
        if (!Array.isArray(files) || !files.length || files.length > MAX_FILES) throw new Error('ziro.project_limit');
        let total = 0;
        const seen = new Set();
        return files.map(file => {
            const name = relative(file.name);
            const key = name.toLowerCase();
            if (seen.has(key)) throw new Error('ziro.invalid_path');
            seen.add(key);
            if (!(file.bytes instanceof Uint8Array)) throw new Error('ziro.invalid_project');
            total += file.bytes.byteLength;
            if (total > MAX_BYTES) throw new Error('ziro.project_limit');
            return { name, bytes: file.bytes };
        });
    }
    function equal(a, b) { return !!a && a.length === b.length && a.every((value, i) => value === b[i]); }
    class Project {
        constructor(request, signal) {
            this.request = request;
            this.signal = signal;
            this.folder = '';
            this.baseline = new Map();
        }
        async call(url, options = {}) {
            if (this.signal?.aborted) throw new DOMException('Aborted', 'AbortError');
            const response = await this.request(url, { credentials: 'same-origin', cache: 'no-store', ...options, signal: this.signal });
            if (!response.ok) {
                const body = await response.json().catch(() => ({}));
                const error = new Error(body.error || body.message || 'HTTP ' + response.status);
                error.status = response.status;
                error.body = body;
                throw error;
            }
            return response;
        }
        async open(path) {
            shared(path);
            if (!/\.kicad_pro$/i.test(path)) throw new Error('ziro.invalid_project');
            const folder = path.slice(0, path.lastIndexOf('/'));
            shared(folder);
            const entries = [];
            for (let offset = 0; ;) {
                const response = await this.call('/api/desktop/files?' + new URLSearchParams({ path: folder, recursive: 'true', limit: '500', offset: String(offset) }));
                const body = await response.json();
                const page = body.files || [];
                entries.push(...page);
                if (entries.length > MAX_FILES) throw new Error('ziro.project_limit');
                if (!body.has_more && !body.truncated) break;
                if (!page.length) throw new Error('ziro.invalid_project');
                offset += page.length;
            }
            let total = 0;
            const files = [], baseline = new Map();
            for (const entry of entries) {
                if (entry.type === 'directory') continue;
                if (!entry.path?.startsWith(folder + '/') || entry.type === 'symlink' || entry.is_symlink || (entry.mode && !entry.mode.startsWith('-'))) throw new Error('ziro.invalid_path');
                const name = relative(entry.path.slice(folder.length + 1));
                if (name.split('/').some(part => ['.git', '.history'].includes(part))) continue;
                total += Number(entry.size || 0);
                if (total > MAX_BYTES) throw new Error('ziro.project_limit');
                const response = await this.call('/api/desktop/download?path=' + encodeURIComponent(entry.path));
                const version = response.headers.get('ETag');
                if (!/^"[0-9a-f]{64}"$/.test(version || '')) throw new Error('ziro.version_missing');
                const bytes = new Uint8Array(await response.arrayBuffer());
                files.push({ name, bytes });
                baseline.set(name, { bytes: bytes.slice(), version });
            }
            filesChecked(files);
            const main = files.findIndex(file => folder + '/' + file.name === path);
            if (main < 0) throw new Error('ziro.invalid_project');
            if (main > 0) files.unshift(files.splice(main, 1)[0]);
            this.folder = folder;
            this.baseline = baseline;
            return files;
        }
        async mkdir(path) {
            shared(path);
            const response = await this.call('/api/desktop/directory', {
                method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ path })
            });
            return response.json();
        }
        async save(files, onConflict) {
            shared(this.folder);
            const checked = filesChecked(files);
            for (const file of checked) {
                const previous = this.baseline.get(file.name);
                if (previous && equal(previous.bytes, file.bytes)) continue;
                const path = this.folder + '/' + file.name;
                const dir = path.slice(0, path.lastIndexOf('/'));
                await this.mkdir(dir);
                let version = previous?.version;
                for (;;) {
                    const form = new FormData();
                    form.set('path', dir);
                    form.set('file', new Blob([file.bytes]), file.name.split('/').pop());
                    let response;
                    try {
                        response = await this.call('/api/desktop/upload', {
                            method: 'POST', headers: version ? { 'If-Match': version } : { 'If-None-Match': '*' }, body: form
                        });
                    } catch (error) {
                        if (error.status !== 412 || error.body?.code !== 'file_conflict') throw error;
                        const choice = await onConflict(path, error.body.conflict);
                        if (choice === 'copy') throw Object.assign(new Error('ziro.copy_project'), { copyProject: true });
                        if (choice !== 'replace' || !error.body.conflict?.version) throw new DOMException('Cancelled', 'AbortError');
                        version = error.body.conflict.version;
                        continue;
                    }
                    const body = await response.json();
                    if (body.path !== path || !body.version) throw new Error('ziro.version_missing');
                    this.baseline.set(file.name, { version: body.version, bytes: file.bytes.slice() });
                    break;
                }
            }
        }
    }
    root.AuraZiroFiles = { Project, relative, shared, filesChecked, MAX_BYTES, MAX_FILES };
})(globalThis);
