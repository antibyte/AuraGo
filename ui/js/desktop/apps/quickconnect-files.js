    // Quick Connect file access for RDP devices, bundled inside the Desktop shell IIFE
    // (scripts/build-ui-bundles.js). The Files tab reuses the /api/desktop/sftp/* endpoints
    // with the RDP device id; the server picks SFTP or SMB from the device's file_access
    // setting. This file owns the device-dialog section, the error-code mapping and the
    // SFTP host-key trust flow. Secrets never pass through here: the server reads them.
    window.QuickConnectFiles = (() => {
        const MODES = ['off', 'auto', 'sftp', 'smb'];
        const DEFAULT_SFTP_PORT = 22;
        const DEFAULT_SHARE = 'C$';
        const DEFAULT_BACKOFF_SECONDS = 30;

        function modeOf(fileAccess) {
            return fileAccess && MODES.includes(fileAccess.mode) ? fileAccess.mode : 'off';
        }

        // Existing RDP devices without a stored setting are off; new devices start on auto.
        function initialSettings(device) {
            const isRDP = !!device && device.protocol === 'rdp';
            const stored = isRDP ? (device.file_access || null) : null;
            if (!device) return { mode: 'auto', sftp_port: DEFAULT_SFTP_PORT, smb_share: DEFAULT_SHARE, credential_id: '' };
            if (!isRDP) return { mode: 'auto', sftp_port: DEFAULT_SFTP_PORT, smb_share: DEFAULT_SHARE, credential_id: '' };
            return {
                mode: modeOf(stored),
                sftp_port: Number(stored && stored.sftp_port) || DEFAULT_SFTP_PORT,
                smb_share: (stored && stored.smb_share) || DEFAULT_SHARE,
                credential_id: (stored && stored.credential_id) || ''
            };
        }

        function sectionMarkup(t, device, credentials) {
            const s = initialSettings(device);
            const modeOption = mode => `<option value="${mode}" ${s.mode === mode ? 'selected' : ''}>${esc(t('desktop.qc_file_access_' + mode))}</option>`;
            const credOptions = (Array.isArray(credentials) ? credentials : []).map(cred => {
                const label = cred.name + (cred.username ? ' (' + cred.username + ')' : '');
                return `<option value="${esc(cred.id)}" ${s.credential_id === cred.id ? 'selected' : ''}>${esc(label)}</option>`;
            }).join('');
            return `<div class="vd-qc-form-section vd-qc-file-access" data-qc-file-access hidden>
                <div class="vd-qc-form-title">${esc(t('desktop.qc_file_access'))}</div>
                <label class="vd-qc-label">${esc(t('desktop.qc_file_access'))}
                    <select class="vd-qc-input" name="file_access_mode">${MODES.map(modeOption).join('')}</select>
                </label>
                <div class="vd-qc-form-row">
                    <label class="vd-qc-label vd-qc-flex-1" data-qc-fa-field="sftp">${esc(t('desktop.qc_file_access_sftp_port'))}
                        <input class="vd-qc-input" type="number" name="file_access_sftp_port" value="${esc(String(s.sftp_port))}" min="1" max="65535">
                    </label>
                    <label class="vd-qc-label vd-qc-flex-1" data-qc-fa-field="smb">${esc(t('desktop.qc_file_access_smb_share'))}
                        <input class="vd-qc-input" type="text" name="file_access_smb_share" value="${esc(s.smb_share)}" maxlength="80" autocapitalize="off" spellcheck="false">
                    </label>
                </div>
                <label class="vd-qc-label" data-qc-fa-field="credential">${esc(t('desktop.qc_file_access_credential'))}
                    <select class="vd-qc-input" name="file_access_credential_id">
                        <option value="" ${s.credential_id ? '' : 'selected'}>${esc(t('desktop.qc_file_access_same_credential'))}</option>${credOptions}
                    </select>
                </label>
                <span class="vd-qc-hint">${esc(t('desktop.qc_file_access_hint'))}</span>
            </div>`;
        }

        // Shows the section only for RDP and only the fields the chosen mode uses.
        function wire(overlay, protocolSelect) {
            const section = overlay.querySelector('[data-qc-file-access]');
            if (!section) return;
            const modeSelect = section.querySelector('select[name="file_access_mode"]');
            const apply = () => {
                section.hidden = protocolSelect.value !== 'rdp';
                const mode = modeSelect.value;
                const show = {
                    sftp: mode === 'auto' || mode === 'sftp',
                    smb: mode === 'auto' || mode === 'smb',
                    credential: mode !== 'off'
                };
                section.querySelectorAll('[data-qc-fa-field]').forEach(field => {
                    field.hidden = !show[field.dataset.qcFaField];
                });
            };
            modeSelect.addEventListener('change', apply);
            protocolSelect.addEventListener('change', apply);
            apply();
        }

        // The file_access object for the device body, or null when the section is absent or hidden.
        function read(overlay) {
            const section = overlay.querySelector('[data-qc-file-access]');
            if (!section || section.hidden) return null;
            const mode = section.querySelector('select[name="file_access_mode"]').value;
            const port = parseInt(section.querySelector('input[name="file_access_sftp_port"]').value, 10);
            const share = section.querySelector('input[name="file_access_smb_share"]').value.trim();
            return {
                mode: MODES.includes(mode) ? mode : 'off',
                sftp_port: port >= 1 && port <= 65535 ? port : DEFAULT_SFTP_PORT,
                smb_share: share || DEFAULT_SHARE,
                credential_id: section.querySelector('select[name="file_access_credential_id"]').value || ''
            };
        }

        function isOffDevice(device) {
            return !!device && device.protocol === 'rdp' && modeOf(device.file_access) === 'off';
        }

        // Maps an api() error (or a hand-built { status, body, retryAfter }) to a message.
        function codeOf(err) {
            const body = err && err.body;
            return body && typeof body.code === 'string' ? body.code : '';
        }

        function messageFor(err, t) {
            const body = (err && err.body) || {};
            switch (codeOf(err)) {
                case 'file_access_admin_required': return t('desktop.qc_files_admin_required');
                case 'file_access_disabled':
                case 'file_access_off': return t('desktop.qc_files_off');
                case 'file_access_unavailable': return t('desktop.qc_files_unavailable');
                case 'host_key_changed': return t('desktop.qc_files_host_key_changed');
                case 'host_key_mismatch': return t('desktop.qc_files_host_key_mismatch');
                case 'host_key_not_pending': return t('desktop.qc_files_host_key_not_pending');
                case 'file_access_auth_backoff': {
                    const seconds = Number(err.retryAfter || body.retry_after) || DEFAULT_BACKOFF_SECONDS;
                    return t('desktop.qc_files_auth_backoff').replace('{seconds}', String(seconds));
                }
                default:
                    return body.error || (err && err.message) || t('desktop.qc_sftp_error');
            }
        }

        function isOffCode(err) {
            const code = codeOf(err);
            return code === 'file_access_disabled' || code === 'file_access_off';
        }

        // Error / off state markup. The edit button (data-action="files-edit-device") is
        // wired by the caller; it only appears for RDP devices that can be edited.
        function noticeMarkup(t, message, withEdit) {
            const edit = withEdit
                ? `<button class="vd-qc-btn vd-qc-btn-sm" type="button" data-action="files-edit-device">${iconMarkup('edit', 'E', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.qc_files_edit_device'))}</span></button>`
                : '';
            return `<div class="vd-qc-sftp-error vd-qc-files-notice"><span>${esc(message)}</span>${edit}</div>`;
        }

        // The trust POST is sent at most once per unknown_host_key response, with the fingerprint
        // of that response only. Any failure (mismatch, expired prompt, 403, 429) is shown and
        // never retried; the user reopens the Files tab to get a fresh prompt.
        // Resolves with { message, off } to display, or null when the host key was trusted
        // and ctx.reload() was called. ctx: { t, api, deviceId, confirm, reload, isCurrent }.
        async function handleListError(err, ctx) {
            const t = ctx.t;
            const body = (err && err.body) || {};
            if (codeOf(err) === 'unknown_host_key' && body.fingerprint) {
                const detail = t('desktop.qc_host_key_prompt_message')
                    .replace('{{host}}', () => String(body.host || ''))
                    .replace('{{key_type}}', () => String(body.key_type || ''))
                    .replace('{{fingerprint}}', () => String(body.fingerprint || ''));
                const accept = await ctx.confirm(t('desktop.qc_host_key_prompt_title'), detail, {
                    okLabel: t('desktop.qc_host_key_accept'),
                    okIcon: 'check-square',
                    okClass: 'vd-qc-btn-primary'
                });
                if (ctx.isCurrent && !ctx.isCurrent()) return null;
                if (!accept) return { message: t('desktop.qc_host_key_rejected'), off: false };
                try {
                    await ctx.api('/api/desktop/sftp/hostkey?device_id=' + encodeURIComponent(ctx.deviceId), {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ device_id: ctx.deviceId, fingerprint: body.fingerprint })
                    });
                } catch (trustErr) {
                    return { message: messageFor(trustErr, t), off: false };
                }
                if (ctx.isCurrent && !ctx.isCurrent()) return null;
                ctx.reload();
                return null;
            }
            return { message: messageFor(err, t), off: isOffCode(err) };
        }

        return { sectionMarkup, wire, read, isOffDevice, messageFor, noticeMarkup, handleListError };
    })();
