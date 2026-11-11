// EasyDrag: move a typed-in credential into the vault (FD-13). The detail view's credential
// warning offers "move to the vault" when the server marks it movable, else "remove value".
// The value is read from the draft here (ED.credentials) and never sent: the server finds it
// again by node, location and path.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const NAME = /^[a-z0-9_]{1,40}$/;

    function sanitize(text) { return String(text || '').toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, ''); }

    // proposeName builds a secret name from the host and the key of a finding, at most 40 characters.
    function proposeName(node, finding) {
        let host = '';
        try { host = new URL(String((node.params || {}).url || '').trim()).hostname; } catch (err) { host = ''; }
        const key = finding.location === 'url_userinfo' ? 'basic' : finding.path;
        return [host, key].map(sanitize).filter(Boolean).join('_').slice(0, 40).replace(/_+$/, '') || 'secret';
    }

    function create(ed) {
        const { t, esc } = ed;
        const core = ED.core;
        let movedInSession = false;
        let busy = false;

        function find(nodeId, issue) {
            const node = ed.model.node(nodeId);
            return node && ED.credentials.findings(node).find(f => f.param === issue.param && f.location === issue.location && f.path === issue.path);
        }

        // places counts the findings the server moves together: the same value in the same kind of place.
        function places(finding) {
            return ed.model.doc.nodes.reduce((sum, node) => sum + ED.credentials.findings(node).filter(f => f.movable && f.location === finding.location && f.value === finding.value).length, 0);
        }

        // applyDoc takes the params of the returned document as one change, so one undo step covers it.
        function applyDoc(served) {
            return ed.model.change(t('easydrag.ui.credential_move'), rec => {
                served.nodes.forEach(n => {
                    const current = ed.model.node(n.id);
                    if (current && JSON.stringify(current.params) !== JSON.stringify(n.params)) rec.node(n.id).params = JSON.parse(JSON.stringify(n.params));
                });
            });
        }

        function notify(message, type) { ed.ctx.notify({ title: t('easydrag.ui.credential_move'), message, type }); }

        // move saves, asks the server and applies its answer. It resolves to true when the dialog may close.
        async function move(finding, nodeId, name, replace) {
            if (busy) return false;
            if (ed.flushPendingEdits && ed.flushPendingEdits() === false) return false;
            if (!(await ed.saver.flush())) { notify(t('easydrag.ui.publish_unsaved'), 'error'); return false; }
            const revision = ed.saver.revision;
            const ticket = ed.saver.beginExternal(revision, ed.model.version);
            if (!ticket) { notify(t('easydrag.ui.publish_unsaved'), 'error'); return false; }
            busy = true;
            let res;
            try {
                res = await ed.api.moveCredential(ed.flow.id, { revision, node_id: nodeId, location: finding.location, path: finding.path, secret_name: name, replace });
            } catch (err) {
                busy = false;
                const status = Number(err && err.status);
                if (core.errorCode(err) && status >= 400 && status < 500 && ![408, 429].includes(status)) ed.saver.releaseExternal(ticket);
                else await ed.saver.recoverExternal(ticket); // uncertain: the saver's conflict flow reloads the server draft
                if (core.errorCode(err) === 'FLOW_SECRET_EXISTS' && !replace) {
                    return (await ed.ctx.confirmDialog(t('easydrag.ui.credential_move'), t('easydrag.ui.credential_replace_confirm', { name }))) ? move(finding, nodeId, name, true) : false;
                }
                notify(core.errorText(t, err), 'error');
                return false;
            }
            busy = false;
            if (!ed.saver.acknowledgeExternal(ticket, res.draft_revision, () => applyDoc(res.doc), res.issues || [])) {
                await ed.saver.recoverExternal(ticket);
                return true;
            }
            ed.model.scrubHistory(finding.value);
            ed.issues = res.issues || [];
            ed.bus.emit('issues', ed.issues);
            movedInSession = true;
            notify(t('easydrag.ui.credential_moved'));
            return true;
        }

        function openMove(nodeId, issue) {
            const finding = find(nodeId, issue);
            if (!finding || busy) return;
            const node = ed.model.node(nodeId);
            const dialog = core.modal(ed.root, {
                title: t('easydrag.ui.credential_move'), closeLabel: t('easydrag.ui.close'),
                body: '<p>' + esc(t('easydrag.ui.credential_move_text', { count: places(finding) })) + '</p><label class="ed-label">' + esc(t('easydrag.ui.credential_move_name')) +
                    '<input class="ed-input" data-ed-credential-name maxlength="40" autocomplete="off" spellcheck="false"></label><p class="ed-error" role="alert" hidden></p>',
                actions: [{ id: 'cancel', label: t('easydrag.ui.cancel') }, { id: 'move', label: t('easydrag.ui.credential_move'), primary: true }],
                onAction: async (action, d) => {
                    if (action !== 'move') return true;
                    const name = d.body.querySelector('[data-ed-credential-name]').value.trim();
                    if (!NAME.test(name)) {
                        const error = d.body.querySelector('.ed-error');
                        error.textContent = t('easydrag.ui.secret_invalid');
                        error.hidden = false;
                        return false;
                    }
                    return move(finding, nodeId, name, false);
                }
            });
            dialog.body.querySelector('[data-ed-credential-name]').value = proposeName(node, finding);
        }

        // removeValue clears the typed value in the client (the warning offers it when moving is not possible).
        function removeValue(nodeId, issue) {
            const node = ed.model.node(nodeId);
            const finding = node && find(nodeId, issue);
            if (!finding) return;
            ed.model.setParam(nodeId, finding.param, ED.credentials.remove(node, finding)[finding.param]);
            ed.model.scrubHistory(finding.value);
        }

        // decorate puts the action on a credential warning (the .ed-field-issue box of the detail form).
        function decorate(box, issue, node) {
            if (!box) return;
            const old = box.querySelector('[data-ed-credential-action]');
            if (old) old.remove();
            if (!issue || issue.code !== 'PARAM_LITERAL_CREDENTIAL' || !node || ed.readonly || ed.runView) return;
            const kind = issue.movable ? 'move' : 'remove';
            const button = core.el('<button type="button" class="ed-btn ed-btn--small" data-ed-credential-action="' + kind + '">' + esc(t(kind === 'move' ? 'easydrag.ui.credential_move' : 'easydrag.ui.credential_remove')) + '</button>');
            button.addEventListener('click', () => (kind === 'move' ? openMove(node.id, issue) : removeValue(node.id, issue)));
            box.appendChild(button);
        }

        // afterPublish offers the guided purge once, after the first publish that follows a move.
        function afterPublish() {
            if (!movedInSession) return;
            movedInSession = false;
            ED.credentialsPurge && ED.credentialsPurge.offer(ed.flow.id);
        }

        return { decorate, move, afterPublish };
    }

    ED.credentialsMove = { create, proposeName };
})();
