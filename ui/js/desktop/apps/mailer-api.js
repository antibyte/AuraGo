(function () {
    'use strict';

    const root = '/api/desktop/mailer';
    const id = value => encodeURIComponent(String(value || ''));

    function create(api) {
        const request = (path, options) => api(root + path, options);
        const json = (method, value, signal) => ({
            method,
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(value || {}),
            signal
        });
        const collection = name => request('/' + name);
        return {
            request,
            state: () => request('/state'),
            settings: enabled => request('/settings', json('POST', { enabled })),
            threads: params => {
                const query = Object.assign({}, params);
                const signal = query.signal;
                delete query.signal;
                return request('/threads?' + new URLSearchParams(query).toString(), { signal });
            },
            thread: threadId => request('/threads/' + id(threadId)),
            read: (threadId, read) => request('/threads/' + id(threadId) + '/read', json('POST', { read })),
            action: (threadId, action) => {
                const payload = Object.assign({}, action || {});
                payload.kind = payload.kind || payload.action;
                payload.destination = payload.destination || payload.destination_folder_id || payload.folder_id || '';
                payload.label = payload.label || payload.label_id || '';
                return request('/threads/' + id(threadId) + '/actions', json('POST', payload));
            },
            snooze: (messageId, until) => request('/messages/' + id(messageId) + '/snooze', json('POST', { until })),
            unsnooze: messageId => request('/messages/' + id(messageId) + '/unsnooze', json('POST', {})),
            drafts: () => collection('drafts'),
            draft: draftId => request('/drafts/' + id(draftId)),
            saveDraft: draft => request('/drafts' + (draft.id ? '/' + id(draft.id) : ''), json(draft.id ? 'PUT' : 'POST', draft)),
            deleteDraft: draft => request('/drafts/' + id(draft.id) + '?revision=' + encodeURIComponent(draft.revision || 0), { method: 'DELETE' }),
            upload: (draftId, revision, file, progress) => new Promise((resolve, reject) => {
                const xhr = new XMLHttpRequest();
                xhr.open('POST', root + '/drafts/' + id(draftId) + '/attachments?revision=' + encodeURIComponent(revision || 0));
                xhr.withCredentials = true;
                xhr.upload.onprogress = event => { if (event.lengthComputable && progress) progress(event.loaded / event.total); };
                xhr.onerror = () => reject(new Error('network'));
                xhr.onabort = () => reject(new Error('aborted'));
                xhr.onload = () => {
                    let body = {};
                    try { body = JSON.parse(xhr.responseText || '{}'); } catch (_) {}
                    if (xhr.status < 200 || xhr.status >= 300) {
                        const error = new Error(body.error || body.message || 'upload_failed');
                        error.status = xhr.status;
                        error.body = body;
                        reject(error);
                        return;
                    }
                    resolve(body);
                };
                const form = new FormData();
                form.append('file', file, file.name);
                xhr.send(form);
            }),
            removeAttachment: (draftId, attachmentId, revision) => request('/drafts/' + id(draftId) + '/attachments/' + id(attachmentId) + '?revision=' + encodeURIComponent(revision || 0), { method: 'DELETE' }),
            copyMessageAttachment: (draftId, revision, messageId, attachmentId) => request('/drafts/' + id(draftId) + '/attachments/from-message?revision=' + encodeURIComponent(revision || 0), json('POST', { message_id: messageId, attachment_id: attachmentId })),
            outbox: params => {
                const query = new URLSearchParams(params || {});
                const search = query.toString();
                return request('/outbox' + (search ? '?' + search : ''));
            },
            enqueue: payload => request('/outbox', json('POST', payload)),
            cancelSend: sendId => request('/outbox/' + id(sendId), { method: 'DELETE' }),
            retrySend: sendId => request('/outbox/' + id(sendId) + '/retry', json('POST', {})),
            accounts: () => collection('accounts'),
            connect: sourceId => request('/accounts', json('POST', { source_id: sourceId })),
            disconnect: accountId => request('/accounts/' + id(accountId), { method: 'DELETE' }),
            accountSettings: (accountId, settings) => request('/accounts/' + id(accountId) + '/settings', json('PUT', settings)),
            createFolder: (accountId, name, parentId) => request('/accounts/' + id(accountId) + '/folders', json('POST', { name, parent_id: parentId || '' })),
            createAccount: payload => request('/accounts/create', json('POST', payload)),
            startOAuth: payload => request('/oauth/start', json('POST', payload)),
            testAccount: (accountId, direction) => request('/accounts/test', json('POST', {
                account_id: accountId,
                receive: direction === 'receive',
                send: direction === 'send'
            })),
            templates: () => collection('templates'),
            signatures: () => collection('signatures'),
            saveTemplate: item => request('/' + (item.kind === 'signature' ? 'signatures' : 'templates') + (item.id ? '/' + id(item.id) : ''), json(item.id ? 'PUT' : 'POST', item)),
            deleteTemplate: item => request('/' + (item.kind === 'signature' ? 'signatures' : 'templates') + '/' + id(item.id) + '?revision=' + encodeURIComponent(item.revision || 0), { method: 'DELETE' }),
            rules: () => collection('rules'),
            ruleRuns: messageId => request('/messages/' + id(messageId) + '/rule-runs'),
            retryRuleRun: (messageId, ruleId) => request('/messages/' + id(messageId) + '/rule-runs/' + id(ruleId) + '/retry', json('POST', {})),
            saveRule: rule => request('/rules' + (rule.id ? '/' + id(rule.id) : ''), json(rule.id ? 'PUT' : 'POST', rule)),
            deleteRule: rule => request('/rules/' + id(rule.id) + '?revision=' + encodeURIComponent(rule.revision || 0), { method: 'DELETE' }),
            previewRule: (rule, messageIds) => request('/rules/preview', json('POST', { rule, message_ids: messageIds })),
            applyRule: (ruleId, messageIds, revision) => request('/rules/' + id(ruleId) + '/apply', json('POST', { message_ids: messageIds, revision })),
            assist: payload => {
                const body = Object.assign({}, payload);
                const signal = body.signal;
                delete body.signal;
                return request('/assist', json('POST', body, signal));
            },
            attachmentURL: (messageId, attachmentId) => root + '/messages/' + id(messageId) + '/attachments/' + id(attachmentId),
            emlURL: messageId => root + '/messages/' + id(messageId) + '/eml'
        };
    }

    window.MailerAPI = { create };
})();
