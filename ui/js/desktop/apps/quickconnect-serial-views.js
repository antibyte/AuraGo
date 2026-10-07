    // Quick Connect serial markup: session shell, profile list, profile form and
    // host port options. Pure HTML builders bundled inside the Desktop shell IIFE
    // (shared esc/iconMarkup); quickconnect-serial.js owns state and wiring.
    const QuickConnectSerialViews = (() => {
        const { BAUD_PRESETS, DEFAULT_OPTIONS, normalizeOptions } = QuickConnectSerialModel;

        function shellMarkup(tr) {
            return `<section class="vd-qc-serial" data-qc-serial-app>
                <header class="vd-qc-serial-header">
                    <div class="vd-qc-serial-heading"><strong>${esc(tr('desktop.qc_serial_title'))}</strong><span data-serial-status data-state="ready">${esc(tr('desktop.qc_serial_idle'))}</span></div>
                    <div class="vd-qc-serial-actions">
                        <button class="vd-qc-btn vd-qc-btn-sm" type="button" data-serial-disconnect disabled>${iconMarkup('x', 'X', 'vd-qc-btn-icon', 13)}<span>${esc(tr('desktop.qc_serial_disconnect'))}</span></button>
                    </div>
                </header>
                <div class="vd-qc-serial-layout">
                    <section class="vd-qc-serial-config" data-serial-editor></section>
                    <section class="vd-qc-serial-console" aria-label="${esc(tr('desktop.qc_serial_title'))}">
                        <div class="vd-qc-serial-toolbar" role="group" aria-label="${esc(tr('desktop.qc_serial_settings'))}">
                            <label>${esc(tr('desktop.qc_serial_receive'))}<select data-serial-rx-mode><option value="text">${esc(tr('desktop.qc_serial_receive_text'))}</option><option value="hex">${esc(tr('desktop.qc_serial_receive_hex'))}</option></select></label>
                            <label class="vd-qc-serial-toggle"><input type="checkbox" data-serial-dtr><span>${esc(tr('desktop.qc_serial_dtr'))}</span></label>
                            <label class="vd-qc-serial-toggle"><input type="checkbox" data-serial-rts><span>${esc(tr('desktop.qc_serial_rts'))}</span></label>
                            <button class="vd-qc-btn vd-qc-btn-sm" type="button" data-serial-break disabled>${esc(tr('desktop.qc_serial_break'))}</button>
                            <button class="vd-qc-btn vd-qc-btn-sm" type="button" data-serial-clear>${esc(tr('desktop.qc_serial_clear'))}</button>
                            <button class="vd-qc-btn vd-qc-btn-sm" type="button" data-serial-export>${esc(tr('desktop.qc_serial_export'))}</button>
                        </div>
                        <div class="vd-qc-serial-terminal" data-serial-terminal></div>
                        <div class="vd-qc-serial-capture-note" data-serial-capture-note hidden></div>
                        <form class="vd-qc-serial-send" data-serial-form>
                            <select data-serial-send-mode aria-label="${esc(tr('desktop.qc_serial_send'))}"><option value="text">${esc(tr('desktop.qc_serial_send_text'))}</option><option value="hex">${esc(tr('desktop.qc_serial_send_hex'))}</option></select>
                            <input type="text" data-serial-input autocomplete="off" spellcheck="false" placeholder="${esc(tr('desktop.qc_serial_input_placeholder'))}" disabled>
                            <select data-serial-line-ending aria-label="${esc(tr('desktop.qc_serial_line_ending'))}">
                                <option value="none">${esc(tr('desktop.qc_serial_no_ending'))}</option><option value="cr" selected>${esc(tr('desktop.qc_serial_carriage_return'))}</option><option value="lf">${esc(tr('desktop.qc_serial_line_feed'))}</option><option value="crlf">${esc(tr('desktop.qc_serial_crlf'))}</option>
                            </select>
                            <button class="vd-qc-btn vd-qc-btn-primary" type="submit" data-serial-send disabled>${iconMarkup('send', 'S', 'vd-qc-btn-icon', 13)}<span>${esc(tr('desktop.qc_serial_send'))}</span></button>
                        </form>
                    </section>
                </div>
            </section>`;
        }

        // isConnectable and diagnose apply the controller's live grants, port
        // availability and browser capability checks to one profile.
        function profileListMarkup({ profiles, selectedId, loading, readonly, isConnectable, diagnose, tr }) {
            return `<div class="vd-qc-serial-list-head"><span>${esc(tr('desktop.qc_serial_select_profile'))}</span><button class="vd-qc-btn vd-qc-btn-sm" type="button" data-serial-create ${readonly ? 'disabled' : ''}>${iconMarkup('plus', '+', 'vd-qc-btn-icon', 13)}<span>${esc(tr('desktop.qc_serial_create_profile'))}</span></button></div>` +
                (loading ? `<div class="vd-empty">${esc(tr('desktop.loading'))}</div>` : !profiles.length ? `<div class="vd-empty">${esc(tr('desktop.qc_serial_no_profiles'))}</div>` : profiles.map(profile => {
                    const detail = profile.source === 'host' ? profile.port : tr('desktop.qc_serial_browser');
                    const canConnect = isConnectable(profile);
                    const diagnostic = diagnose(profile);
                    return `<article class="vd-qc-serial-profile${profile.id === selectedId ? ' active' : ''}">
                        <button class="vd-qc-serial-profile-select" type="button" data-serial-profile="${esc(profile.id)}" aria-pressed="${profile.id === selectedId ? 'true' : 'false'}">
                            <strong>${esc(profile.name)}</strong><span><span class="vd-qc-badge vd-qc-serial-source" data-source="${esc(profile.source)}">${esc(profile.source === 'host' ? tr('desktop.qc_serial_host') : tr('desktop.qc_serial_browser'))}</span><span class="vd-qc-serial-profile-detail">${esc(detail || tr('desktop.qc_serial_choose_port'))}</span></span>
                            ${diagnostic ? `<small class="vd-qc-serial-diagnostic" role="status">${esc(diagnostic)}</small>` : ''}
                        </button>
                        <div class="vd-qc-serial-profile-actions">
                            <button class="vd-qc-btn vd-qc-btn-sm vd-qc-btn-primary" type="button" data-serial-connect="${esc(profile.id)}" ${canConnect ? '' : 'disabled'}>${esc(tr('desktop.qc_serial_connect'))}</button>
                            <button class="vd-qc-btn vd-qc-btn-sm" type="button" data-serial-delete="${esc(profile.id)}" ${readonly ? 'disabled' : ''} aria-label="${esc(tr('desktop.qc_serial_delete_profile'))}">${iconMarkup('trash', 'X', 'vd-qc-btn-icon', 13)}</button>
                        </div>
                    </article>`;
                }).join(''));
        }

        // Keeps a saved port that is currently missing visible as unavailable.
        function portOptionsMarkup(selectedPort, ports, portsError, tr) {
            const current = String(selectedPort || '');
            let output = `<option value="">${esc(tr('desktop.qc_serial_choose_port'))}</option>`;
            if (current && !ports.some(port => port.name === current)) output += `<option value="${esc(current)}" selected disabled>${esc(current)} — ${esc(tr('desktop.qc_serial_unavailable_port'))}</option>`;
            if (portsError) return output;
            if (!ports.length) output += `<option value="" disabled>${esc(tr('desktop.qc_serial_no_ports'))}</option>`;
            return output + ports.map(port => `<option value="${esc(port.name)}" ${port.name === current ? 'selected' : ''} ${port.busy ? 'disabled' : ''}>${esc(port.name)}${port.busy ? ` — ${esc(tr('desktop.qc_serial_port_busy'))}` : ''}</option>`).join('');
        }

        // Renders the editor for an existing profile, or a new-profile form when
        // current is undefined.
        function profileFormMarkup({ current, readonly, ports, portsError, tr }) {
            const profile = current || { id: '', name: '', source: 'browser', port: '', options: { ...DEFAULT_OPTIONS } };
            const source = profile.source;
            const optsValue = normalizeOptions(profile.options, source);
            const baudValue = BAUD_PRESETS.includes(optsValue.baud_rate) ? String(optsValue.baud_rate) : 'custom';
            const hostOptions = portOptionsMarkup(profile.port, ports, portsError, tr);
            return `<form class="vd-qc-serial-profile-form" data-serial-profile-form>
                <h3>${esc(current ? current.name : tr('desktop.qc_serial_create_profile'))}</h3>
                <label>${esc(tr('desktop.qc_serial_profile_name'))}<input name="name" type="text" maxlength="80" required value="${esc(profile.name)}" ${readonly ? 'disabled' : ''}></label>
                <label>${esc(tr('desktop.qc_serial_source'))}<select name="source" ${readonly ? 'disabled' : ''}><option value="browser" ${source === 'browser' ? 'selected' : ''}>${esc(tr('desktop.qc_serial_browser'))}</option><option value="host" ${source === 'host' ? 'selected' : ''}>${esc(tr('desktop.qc_serial_host'))}</option></select></label>
                <div data-serial-browser-fields ${source === 'browser' ? '' : 'hidden'}>
                    <p class="vd-qc-serial-help">${esc(tr('desktop.qc_serial_choose_port'))}</p>
                    <div class="vd-qc-serial-pair"><label>USB VID<input name="usb_vendor_id" type="text" inputmode="text" maxlength="6" placeholder="10C4" value="${profile.usb_vendor_id !== undefined ? profile.usb_vendor_id.toString(16).toUpperCase().padStart(4, '0') : ''}" ${readonly ? 'disabled' : ''}></label><label>USB PID<input name="usb_product_id" type="text" inputmode="text" maxlength="6" placeholder="EA60" value="${profile.usb_product_id !== undefined ? profile.usb_product_id.toString(16).toUpperCase().padStart(4, '0') : ''}" ${readonly ? 'disabled' : ''}></label></div>
                </div>
                <div data-serial-host-fields ${source === 'host' ? '' : 'hidden'}>
                    <label>${esc(tr('desktop.qc_serial_port'))}<select name="port" ${readonly ? 'disabled' : ''}>${hostOptions}</select></label>
                    <button class="vd-qc-btn vd-qc-btn-sm" type="button" data-serial-refresh-ports ${readonly ? 'disabled' : ''}>${iconMarkup('refresh', 'R', 'vd-qc-btn-icon', 12)}<span>${esc(tr('desktop.retry'))}</span></button>
                </div>
                <div class="vd-qc-serial-inline-status" data-serial-port-error role="status" ${portsError && source === 'host' ? '' : 'hidden'}>${portsError && source === 'host' ? esc(tr('desktop.load_failed')) : ''}</div>
                <details class="vd-qc-serial-options" open><summary>${esc(tr('desktop.qc_serial_settings'))}</summary>
                    <div class="vd-qc-serial-pair"><label>${esc(tr('desktop.qc_serial_baud_rate'))}<select name="baud_preset">${BAUD_PRESETS.map(rate => `<option value="${rate}" ${baudValue === String(rate) ? 'selected' : ''}>${rate}</option>`).join('')}<option value="custom" ${baudValue === 'custom' ? 'selected' : ''}>${esc(tr('desktop.qc_serial_custom'))}</option></select></label><label data-custom-baud ${baudValue === 'custom' ? '' : 'hidden'}>${esc(tr('desktop.qc_serial_custom'))}<input name="baud_custom" type="number" min="1" max="4000000" value="${optsValue.baud_rate}" ${readonly ? 'disabled' : ''}></label></div>
                    <div class="vd-qc-serial-pair"><label>${esc(tr('desktop.qc_serial_data_bits'))}<select name="data_bits"><option value="7" ${optsValue.data_bits === 7 ? 'selected' : ''}>7</option><option value="8" ${optsValue.data_bits === 8 ? 'selected' : ''}>8</option></select></label><label>${esc(tr('desktop.qc_serial_stop_bits'))}<select name="stop_bits"><option value="1" ${optsValue.stop_bits === 1 ? 'selected' : ''}>1</option><option value="2" ${optsValue.stop_bits === 2 ? 'selected' : ''}>2</option></select></label></div>
                    <div class="vd-qc-serial-pair"><label>${esc(tr('desktop.qc_serial_parity'))}<select name="parity"><option value="none" ${optsValue.parity === 'none' ? 'selected' : ''}>${esc(tr('desktop.qc_serial_none'))}</option><option value="even" ${optsValue.parity === 'even' ? 'selected' : ''}>${esc(tr('desktop.qc_serial_even'))}</option><option value="odd" ${optsValue.parity === 'odd' ? 'selected' : ''}>${esc(tr('desktop.qc_serial_odd'))}</option></select></label><label>${esc(tr('desktop.qc_serial_flow_control'))}<select name="flow_control"><option value="none">${esc(tr('desktop.qc_serial_none'))}</option><option value="hardware" ${optsValue.flow_control === 'hardware' ? 'selected' : ''} ${source === 'host' ? 'disabled' : ''}>${esc(tr('desktop.qc_serial_hardware'))}${source === 'host' ? ` — ${esc(tr('desktop.qc_serial_hardware_host_unsupported'))}` : ''}</option></select></label></div>
                    <div class="vd-qc-serial-pair"><label class="vd-qc-serial-check"><input name="dtr" type="checkbox" ${optsValue.dtr ? 'checked' : ''} ${readonly ? 'disabled' : ''}><span>${esc(tr('desktop.qc_serial_dtr'))}</span></label><label class="vd-qc-serial-check"><input name="rts" type="checkbox" ${optsValue.rts ? 'checked' : ''} ${readonly || (source === 'browser' && optsValue.flow_control === 'hardware') ? 'disabled' : ''}><span>${esc(tr('desktop.qc_serial_rts'))}</span></label></div>
                    <label class="vd-qc-serial-check"><input name="local_echo" type="checkbox" ${optsValue.local_echo ? 'checked' : ''}><span>${esc(tr('desktop.qc_serial_local_echo'))}</span></label>
                    <label>${esc(tr('desktop.qc_serial_line_ending'))}<select name="line_ending"><option value="none" ${optsValue.line_ending === 'none' ? 'selected' : ''}>${esc(tr('desktop.qc_serial_no_ending'))}</option><option value="cr" ${optsValue.line_ending === 'cr' ? 'selected' : ''}>${esc(tr('desktop.qc_serial_carriage_return'))}</option><option value="lf" ${optsValue.line_ending === 'lf' ? 'selected' : ''}>${esc(tr('desktop.qc_serial_line_feed'))}</option><option value="crlf" ${optsValue.line_ending === 'crlf' ? 'selected' : ''}>${esc(tr('desktop.qc_serial_crlf'))}</option></select></label>
                </details>
                <div class="vd-qc-serial-form-actions"><button class="vd-qc-btn vd-qc-btn-primary" type="submit" data-serial-save ${readonly ? 'disabled' : ''}>${iconMarkup('save', 'S', 'vd-qc-btn-icon', 13)}<span>${esc(tr('desktop.qc_serial_save_profile'))}</span></button></div>
                <div data-serial-form-status role="status" aria-live="polite"></div>
            </form>`;
        }

        return { shellMarkup, profileListMarkup, portOptionsMarkup, profileFormMarkup };
    })();
