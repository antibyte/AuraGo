(function () {
    'use strict';

    function revision(inst) {
        return JSON.stringify([
            Number(inst.editGeneration) || 0,
            Number(inst.uploadQueueCount) || 0,
            !!inst.attachmentBusy,
            !!inst.forwardCopying,
            !!inst.draftConflict,
            !!inst.sending
        ]);
    }

    function t(ctx, key) {
        return ctx && typeof ctx.t === 'function' ? ctx.t('mailer.' + key) : 'mailer.' + key;
    }

    async function beforeClose(inst, save) {
        clearTimeout(inst.saveTimer);
        await inst.uploadTask.catch(() => {});
        await inst.forwardCopyPromise;
        await inst.attachmentTask;
        if (inst.failedUploads.size) {
            const keep = await inst.ctx.confirmDialog(
                t(inst.ctx, 'upload_failed_title'),
                t(inst.ctx, 'upload_failed_close_confirm')
            );
            if (!keep) return false;
        }
        if (inst.readonly || inst.committed || inst.savedGeneration === inst.editGeneration) return true;
        try { await save(inst); return true; }
        catch (_) {
            const keep = await inst.ctx.confirmDialog(
                t(inst.ctx, 'save_failed_title'),
                t(inst.ctx, 'close_unsaved_confirm')
            );
            return !!keep;
        }
    }

    function register(ctx, windowId, inst, save) {
        ctx.registerSessionAction?.(windowId, {
            inspect: () => ({
                dirty: !inst.committed && (
                    inst.savedGeneration !== inst.editGeneration || inst.uploadQueueCount > 0 ||
                    !!inst.attachmentBusy || !!inst.forwardCopying || !!inst.draftConflict || !!inst.sending
                ),
                revision: revision(inst),
                label: String(inst.draft.subject || inst.draft.id || t(ctx, 'compose'))
            }),
            decide: snapshot => ctx.decideSessionAction?.({ title: snapshot.title, label: snapshot.label, dirty: snapshot.dirty }),
            save: async expected => {
                if (inst.disposed || inst.readonly || inst.committed || inst.sending || revision(inst) !== expected) return false;
                clearTimeout(inst.saveTimer);
                await inst.uploadTask.catch(() => {});
                await inst.forwardCopyPromise;
                await inst.attachmentTask;
                if (inst.failedUploads.size || revision(inst) !== expected) return false;
                await save(inst);
                return !inst.draftConflict && !inst.failedUploads.size &&
                    inst.savedGeneration === inst.editGeneration && revision(inst) === expected
                    ? { revision: expected }
                    : false;
            }
        });
    }

    window.MailerSessionActions = { beforeClose, register };
})();
