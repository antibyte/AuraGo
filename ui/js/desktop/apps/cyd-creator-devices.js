// CYD Creator devices (plan 5): the OTA dialog lists the boards flashed with the AuraGo link (name,
// board, current build, online/last seen, IP, RSSI, last update error), renames them, sets "Update to
// build …" (the board pulls it with its next heartbeat) and removes them (revokes the token). It follows
// device_changed events live and refreshes the online state every 30 s. Read-only users see the list
// without actions. Also adds the device API wrappers to P.api (the flasher mints through them).
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    const REFRESH_MS = 30000;
    const enc = value => encodeURIComponent(String(value));

    Object.assign(P.api, {
        listDevices: projectId => P.api.request('GET', '/projects/' + enc(projectId) + '/devices').then(r => r.data.devices || []),
        mintDevice: (projectId, buildId) => P.api.request('POST', '/projects/' + enc(projectId) + '/devices', { json: { build_id: buildId } }).then(r => r.data),
        updateDevice: (deviceId, patch) => P.api.request('PATCH', '/devices/' + enc(deviceId), { json: patch }).then(r => r.data.device),
        revokeDevice: deviceId => P.api.request('DELETE', '/devices/' + enc(deviceId)).then(() => true)
    });

    // errorText translates a stored last_error: "rolled_back[: detail]" or "failed[: reason]". A
    // rollback adds the hint that re-picking the failed build is reset until the board restarts (the
    // server resets that target during the boot that rolled back).
    function errorText(t, value) {
        const text = String(value || '');
        if (!text) return '';
        const [state, ...rest] = text.split(': ');
        const detail = rest.join(': ');
        if (state === 'rolled_back') return t('cyd_creator.devices.error_rolled_back') + (detail ? ' (' + detail + ')' : '') + ' ' + t('cyd_creator.devices.rollback_hint');
        if (state === 'failed') return detail ? t('cyd_creator.devices.error_failed', { reason: detail }) : t('cyd_creator.devices.error_failed_plain');
        return text;
    }

    function open(inst) {
        const { el } = P.util;
        const t = inst.t;
        const ota = (inst.caps && inst.caps.ota) || {};
        let readonly = !!inst.readonly;
        const state = { devices: [], loading: true, error: '', editing: '', draft: '' };
        const builds = () => ((inst.dock && inst.dock.builds.list()) || [])
            .filter(b => b.status === 'succeeded' && (b.artifacts || []).some(a => a.name === 'firmware.bin'));
        const buildName = id => {
            const all = (inst.dock && inst.dock.builds.list()) || [];
            const index = all.findIndex(b => b.id === id);
            if (index < 0) return id ? t('cyd_creator.devices.build_unknown', { id }) : t('cyd_creator.devices.build_none');
            const b = all[index];
            return t('cyd_creator.devices.build_label', { n: all.length - index, time: P.format.time(b.finished_at || b.created_at) });
        };
        const boardLabel = id => (((inst.caps && inst.caps.boards) || []).find(b => b.id === id) || {}).label || id;

        const list = el('ul', { class: 'cc-device-list', dataset: { ccDeviceList: '' } });
        const banners = [];
        if (ota.enabled === false) banners.push(el('p', { class: 'cc-banner cc-banner--warn', dataset: { ccDevicesOta: 'off' }, text: t('cyd_creator.devices.ota_off') }));
        if (ota.public_url) banners.push(el('p', { class: 'cc-form-help', dataset: { ccDevicesUrl: '' }, text: t('cyd_creator.devices.url', { url: ota.public_url }) }));
        if (ota.insecure) banners.push(el('p', { class: 'cc-banner cc-banner--warn', role: 'alert', dataset: { ccDevicesInsecure: '' }, text: t('cyd_creator.devices.insecure') }));
        const readonlyBanner = el('p', { class: 'cc-banner', dataset: { ccDevicesReadonly: '' }, text: t('cyd_creator.devices.readonly') });
        if (readonly) banners.push(readonlyBanner);
        const errorBox = el('p', { class: 'cc-dialog-error', role: 'alert', hidden: true });
        const d = P.dialog.open(inst.root, {
            title: t('cyd_creator.devices.title'),
            className: 'cc-dialog--wide cc-devices-dialog',
            body: el('div', { class: 'cc-devices' }, banners, errorBox, list),
            actions: [{ id: 'close', label: t('cyd_creator.app.close') }],
            // The dialog belongs to the open project: switching projects or closing the window closes it.
            owner: inst.projectBag || inst.bag
        });

        function showError(err) {
            errorBox.hidden = !err;
            errorBox.textContent = err ? P.errors.message(t, err) : '';
        }

        function statusTag(device) {
            const key = device.status === 'pending' ? 'pending' : (device.online ? 'online' : 'offline');
            return el('span', { class: 'cc-tag', dataset: { tone: key === 'online' ? 'ok' : (key === 'pending' ? 'warning' : ''), ccDeviceState: key }, text: t('cyd_creator.devices.state_' + key) });
        }

        function facts(device) {
            const rows = [
                [t('cyd_creator.devices.board'), boardLabel(device.board)],
                [t('cyd_creator.devices.build'), buildName(device.build_id)],
                [t('cyd_creator.devices.last_seen'), device.last_seen ? P.format.time(device.last_seen) : t('cyd_creator.devices.never')]
            ];
            if (device.target_build_id && device.target_build_id !== device.build_id) rows.push([t('cyd_creator.devices.target'), buildName(device.target_build_id)]);
            if (device.ip) rows.push([t('cyd_creator.devices.ip'), device.ip]);
            if (device.rssi) rows.push([t('cyd_creator.devices.rssi'), t('cyd_creator.devices.rssi_value', { rssi: device.rssi })]);
            return el('dl', { class: 'cc-device-facts' }, rows.map(([k, v]) => [el('dt', { text: k }), el('dd', { text: v })]));
        }

        function actionsFor(device) {
            if (readonly) return null;
            const box = el('div', { class: 'cc-device-actions' });
            // Only builds for the device's board: the server refuses a target for another board.
            const select = el('select', { class: 'cc-select', dataset: { ccDeviceTarget: device.id }, 'aria-label': t('cyd_creator.devices.update_to') },
                builds().filter(b => !b.board || !device.board || b.board === device.board)
                    .map(b => el('option', { value: b.id, selected: b.id === (device.target_build_id || device.build_id), text: buildName(b.id) })));
            const update = el('button', { type: 'button', class: 'cc-btn cc-btn--small cc-btn--primary', dataset: { ccDeviceAction: 'update', ccDeviceId: device.id }, text: t('cyd_creator.devices.update') });
            const syncUpdate = () => { update.disabled = ota.enabled === false || !select.value || select.value === device.target_build_id; };
            select.addEventListener('change', syncUpdate);
            syncUpdate();
            update.addEventListener('click', async () => {
                update.disabled = true;
                try {
                    upsert(await P.api.updateDevice(device.id, { target_build_id: select.value }));
                    showError(null);
                } catch (err) { showError(err); syncUpdate(); }
            });
            const rename = el('button', { type: 'button', class: 'cc-btn cc-btn--small cc-btn--ghost', dataset: { ccDeviceAction: 'rename', ccDeviceId: device.id }, text: t('cyd_creator.devices.rename') });
            rename.addEventListener('click', () => { state.editing = device.id; state.draft = device.name; render(); });
            const remove = el('button', { type: 'button', class: 'cc-btn cc-btn--small cc-btn--danger', dataset: { ccDeviceAction: 'remove', ccDeviceId: device.id }, text: t('cyd_creator.devices.remove') });
            remove.addEventListener('click', async () => {
                const ok = await inst.confirm({ titleKey: 'cyd_creator.devices.remove_title', bodyKey: 'cyd_creator.devices.remove_body', vars: { name: device.name }, okKey: 'cyd_creator.devices.remove', danger: true });
                if (!ok || readonly) return;
                try {
                    await P.api.revokeDevice(device.id);
                    state.devices = state.devices.filter(x => x.id !== device.id);
                    render();
                    showError(null);
                } catch (err) { showError(err); }
            });
            box.append(el('label', { class: 'cc-device-update' }, el('span', { text: t('cyd_creator.devices.update_to') }), select), update, rename, remove);
            return box;
        }

        function nameRow(device) {
            if (readonly || state.editing !== device.id) return el('strong', { class: 'cc-device-name', dataset: { ccDeviceName: device.id }, text: device.name });
            // The draft survives the re-renders of live events and the 30 s refresh.
            const input = el('input', { class: 'cc-input', value: state.draft, maxlength: '64', dataset: { ccDeviceNameInput: device.id }, 'aria-label': t('cyd_creator.devices.name') });
            input.addEventListener('input', () => { state.draft = input.value; });
            const save = async () => {
                const name = input.value.trim();
                if (!name || name === device.name) { state.editing = ''; render(); return; }
                try {
                    state.editing = '';
                    upsert(await P.api.updateDevice(device.id, { name }));
                    showError(null);
                } catch (err) { showError(err); render(); }
            };
            input.addEventListener('keydown', event => {
                if (event.key === 'Enter') { event.preventDefault(); save(); }
            });
            setTimeout(() => input.focus(), 0);
            return el('span', { class: 'cc-device-rename' }, input,
                el('button', { type: 'button', class: 'cc-btn cc-btn--small', dataset: { ccDeviceAction: 'save-name' }, text: t('cyd_creator.devices.save'), onClick: save }));
        }

        function render() {
            if (state.loading) { list.replaceChildren(el('li', { class: 'cc-loading', text: t('cyd_creator.app.loading') })); return; }
            if (!state.devices.length) { list.replaceChildren(el('li', { class: 'cc-empty', dataset: { ccDevicesEmpty: '' }, text: t('cyd_creator.devices.empty') })); return; }
            list.replaceChildren(...state.devices.map(device => el('li', { class: 'cc-device', dataset: { ccDevice: device.id } },
                el('div', { class: 'cc-device-head' }, nameRow(device), statusTag(device)),
                facts(device),
                device.last_error ? el('p', { class: 'cc-device-error', role: 'status', dataset: { ccDeviceError: device.id }, text: errorText(t, device.last_error) }) : null,
                actionsFor(device))));
        }

        function upsert(device) {
            if (!device || device.project_id !== inst.projectId) return;
            if (device.status === 'revoked') state.devices = state.devices.filter(x => x.id !== device.id);
            else {
                const index = state.devices.findIndex(x => x.id === device.id);
                if (index >= 0) state.devices[index] = device; else state.devices.push(device);
            }
            render();
        }

        async function load() {
            if (d.closed || !d.el.isConnected) { stop(); return; }
            try {
                state.devices = await P.api.listDevices(inst.projectId);
                showError(null);
            } catch (err) {
                showError(err);
            } finally {
                state.loading = false;
                render();
            }
        }

        // A desktop that turns read-only (or back) while the dialog is open shows or hides the actions.
        function onReadonly() {
            readonly = !!inst.readonly;
            if (readonly) { state.editing = ''; if (!readonlyBanner.isConnected) errorBox.before(readonlyBanner); }
            else readonlyBanner.remove();
            render();
        }

        const offs = [inst.bus.on('device:changed', upsert), inst.bus.on('readonly:changed', onReadonly)];
        const timer = setInterval(load, REFRESH_MS);
        function stop() { offs.forEach(off => off()); clearInterval(timer); }
        d.promise.then(stop);
        render();
        load();
        return d;
    }

    P.devices = { open, errorText };
})();
