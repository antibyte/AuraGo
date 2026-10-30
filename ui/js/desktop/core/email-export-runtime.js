    // Only first-party app windows can prepare mail. Files never enter session storage.
    const desktopEmailBindings = new Map();
    const desktopEmailLimit = 25 * 1024 * 1024;

    function emailExportError(key, values) {
        return new Error(t('desktop.email_' + key, values));
    }

    function emailSendAccounts(body) {
        return (body.accounts || []).map(account => Object.assign({}, account, body.account_settings?.[account.id] || {}))
            .filter(account => !account.disabled && !account.read_only && (account.capabilities || []).includes('send'));
    }

    async function emailExportState(signal) {
        const body = await api('/api/desktop/mailer/state', { signal });
        body.accounts = (body.accounts || []).map(account => Object.assign({}, account, body.account_settings?.[account.id] || {}));
        if (desktopReadonly() || body.readonly) throw emailExportError('readonly');
        if (!body.enabled) throw Object.assign(emailExportError('disabled'), { mailSetup: true });
        if (!emailSendAccounts(body).length) throw Object.assign(emailExportError('no_account'), { mailSetup: true });
        return body;
    }

    async function desktopEmailExport(windowId, options) {
        const win = state.windows.get(windowId);
        if (!win || !isBuiltinApp(win.appId) || win.appId === 'tresor') return false;
        options = options || {};
        if (win.emailExportBusy || options.disabled) return false;
        const controller = new AbortController();
        const signal = controller.signal;
        const abort = () => controller.abort();
        const check = () => {
            if (signal.aborted || !state.windows.has(windowId) || options.isCurrent?.() === false) {
                controller.abort();
                throw new DOMException('closed', 'AbortError');
            }
        };
        win.emailExportBusy = controller;
        options.signal?.addEventListener('abort', abort, { once: true });
        if (options.signal?.aborted) abort();
        const closeAbort = () => controller.abort();
        registerWindowCleanup(windowId, closeAbort);
        const policyChange = () => { if (desktopReadonly()) abort(); };
        document.addEventListener('aurago:desktop-policy', policyChange);
        try {
            check();
            await emailExportState(signal);
            check();
            const formats = (options.formats || []).map(item => typeof item === 'string' ? { id: item, label: item.toUpperCase() } : item);
            if (!formats.length) throw emailExportError('empty');
            const format = formats.length === 1 ? formats[0].id : await modalDialog({
                title: t('desktop.email_export'), selectLabel: t('desktop.email_format'), signal, host:options.dialogHost,
                select: formats.map(item => ({ value: item.id, label: item.label || item.id }))
            });
            if (!format) return false;
            check();
            const files = await options.createFiles(format, signal);
            check();
            if (files === false || files == null) return false;
            if (!Array.isArray(files) || !files.length || files.some(file => !(file instanceof File))) throw emailExportError('empty');
            const body = await emailExportState(signal);
            check();
            const limits = body.attachment_limits || {};
            const maxFile = Math.min(Number(limits.max_file_bytes) || desktopEmailLimit, desktopEmailLimit);
            const maxTotal = Math.min(Number(limits.max_message_bytes) || desktopEmailLimit, desktopEmailLimit);
            const oversized = files.find(file => file.size > maxFile);
            if (oversized) throw emailExportError('file_too_large', {name:oversized.name,size:maxFile / 1024 / 1024});
            if (files.reduce((sum, file) => sum + file.size, 0) > maxTotal) {
                throw emailExportError('too_large', { size: Math.floor(maxTotal / 1024 / 1024) });
            }
            let preferred = '';
            try { preferred = JSON.parse(localStorage.getItem('aurago.desktop.mailer.prefs') || '{}').accountId || ''; } catch (_) {}
            const accounts = emailSendAccounts(body);
            const accountId = (accounts.find(account => account.id === preferred) || accounts[0]).id;
            // The composer consumes and removes this one-shot launch field before its first upload.
            openApp('mailer', { forceNew: true, compose: true, state: body, accountId,
                draft: { subject: String(options.subject || files[0].name).replace(/[\r\n]/g, ' ').slice(0, 998) }, exportFiles: files });
            options.onHandoff?.();
            return true;
        } catch (error) {
            if (error?.name === 'AbortError' || signal.aborted) return false;
            if (error.mailSetup) {
                const setup = await modalDialog({ title: t('desktop.email_export'), message: error.message, signal, host:options.dialogHost,
                    choices: [{ value: 'setup', label: t('desktop.email_settings') }] });
                if (setup === 'setup' && !signal.aborted) {
                    openApp('mailer', { forceNew: true, showSettings: true });
                    options.onSetup?.();
                }
            } else showDesktopNotification({ title: t('desktop.email_export'), message: error.message || t('desktop.email_failed') });
            return false;
        } finally {
            options.signal?.removeEventListener('abort', abort);
            document.removeEventListener('aurago:desktop-policy', policyChange);
            if (win.emailExportBusy === controller) delete win.emailExportBusy;
            const cleanups = state.windowCleanups.get(windowId);
            if (cleanups) state.windowCleanups.set(windowId, cleanups.filter(cleanup => cleanup !== closeAbort));
        }
    }

    async function emailFileFromURL(url, filename, signal) {
        const target = new URL(url, location.origin);
        const allowed = /^\/(?:api\/desktop\/|api\/openscad\/|files\/)/.test(target.pathname)
            || /^\/api\/game-maker\/projects\/[^/]+\/export$/.test(target.pathname);
        if (target.origin !== location.origin || target.username || target.password || !allowed) throw emailExportError('failed');
        const response = await fetch(target.href, { credentials: 'same-origin', redirect: 'error', signal });
        if (!response.ok) throw emailExportError('failed');
        if (Number(response.headers.get('Content-Length')) > desktopEmailLimit) {
            await response.body?.cancel();
            throw emailExportError('too_large', { size: 25 });
        }
        const chunks = [];
        let size = 0;
        const reader = response.body?.getReader();
        if (!reader) throw emailExportError('failed');
        try {
            while (true) {
                const item = await reader.read();
                if (item.done) break;
                size += item.value.byteLength;
                if (size > desktopEmailLimit) {
                    await reader.cancel();
                    throw emailExportError('too_large', { size: 25 });
                }
                chunks.push(item.value);
            }
        } finally { reader.releaseLock(); }
        return new File(chunks, filename || 'attachment', { type: response.headers.get('Content-Type') || 'application/octet-stream' });
    }

    function attachEmailExport(windowId, toolbar, getOptions, scope) {
        const host = scope || contentEl(windowId);
        if (!host) return;
        desktopEmailBindings.get(host)?.dispose();
        const button = document.createElement('button');
        button.type = 'button';
        button.className = 'vd-button vd-email-export-button';
        button.dataset.emailExport = '';
        button.textContent = t('desktop.email_export');
        let disposed = false, scheduled = false, working = false, activeKey;
        const options = () => { try { return getOptions() || {}; } catch (_) { return {}; } };
        const refresh = () => {
            scheduled = false;
            if (disposed) return;
            const opts = options();
            const win = state.windows.get(windowId);
            if (working && win?.emailExportBusy && opts.key !== activeKey) win.emailExportBusy.abort();
            const target = typeof toolbar === 'function' ? toolbar() : host.querySelector(toolbar);
            if (target && button.parentNode !== target) target.appendChild(button);
            const disabled = desktopReadonly() || opts.disabled || !opts.formats?.length;
            button.disabled = !!(disabled || win?.emailExportBusy);
            const label = t(win?.emailExportBusy ? 'desktop.email_preparing' : 'desktop.email_export');
            if (button.textContent !== label) button.textContent = label;
            button.title = disabled ? opts.disabledReason || t(desktopReadonly() ? 'desktop.email_readonly' : 'desktop.email_empty') : t('desktop.email_export');
        };
        const schedule = () => { if (!scheduled) { scheduled = true; queueMicrotask(refresh); } };
        button.addEventListener('click', async () => {
            const opts = options();
            activeKey = opts.key;
            working = true;
            const task = desktopEmailExport(windowId, Object.assign({}, opts, { isCurrent: () => !disposed && options().key === activeKey }));
            refresh();
            await task;
            working = false;
            refresh();
        });
        // App redraws replace toolbars and result selections without replacing their window.
        const observer = new MutationObserver(schedule);
        observer.observe(host, { childList: true, subtree: true });
        host.addEventListener('input', schedule);
        host.addEventListener('change', schedule);
        host.addEventListener('click', schedule);
        document.addEventListener('aurago:desktop-policy', schedule);
        const binding = { refresh, dispose() {
            disposed = true;
            observer.disconnect();
            host.removeEventListener('input', schedule);
            host.removeEventListener('change', schedule);
            host.removeEventListener('click', schedule);
            document.removeEventListener('aurago:desktop-policy', schedule);
            button.remove();
            if (working) state.windows.get(windowId)?.emailExportBusy?.abort();
            if (desktopEmailBindings.get(host) === binding) desktopEmailBindings.delete(host);
        } };
        desktopEmailBindings.set(host, binding);
        registerWindowCleanup(windowId, () => binding.dispose());
        refresh();
        return binding;
    }

    async function emailFilesFromURLs(entries, signal) {
        const files = [];
        let total = 0;
        for (const entry of entries) {
            const file = await emailFileFromURL(entry.url, entry.name, signal);
            total += file.size;
            if (total > desktopEmailLimit) throw emailExportError('too_large', {size:25});
            files.push(file);
        }
        return files;
    }

    function rememberEmailAccount(accountId) {
        try {
            const prefs = JSON.parse(localStorage.getItem('aurago.desktop.mailer.prefs') || '{}');
            localStorage.setItem('aurago.desktop.mailer.prefs', JSON.stringify(Object.assign({}, prefs, {accountId})));
        } catch (_) {}
    }

    window.AuraDesktopEmail = { open: desktopEmailExport, attach: attachEmailExport, file: emailFileFromURL, files: emailFilesFromURLs, rememberAccount: rememberEmailAccount };
