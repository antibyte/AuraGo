    // Quick Connect serial profile model: the versioned profile schema, option
    // normalization, hex codec, profile form drafts and server error-code keys.
    // Pure helpers bundled inside the Desktop shell IIFE before
    // quickconnect-serial.js, which owns the session controller.
    const QuickConnectSerialModel = (() => {
        const STORAGE_KEY = 'quick_connect.serial_profiles';
        const MAX_PROFILES = 64;
        const MAX_PROFILE_BYTES = 64 * 1024;
        const BAUD_PRESETS = [1200, 2400, 4800, 9600, 14400, 19200, 28800, 38400, 57600, 76800, 115200, 230400, 460800, 921600];
        const DEFAULT_OPTIONS = Object.freeze({
            baud_rate: 115200,
            data_bits: 8,
            stop_bits: 1,
            parity: 'none',
            flow_control: 'none',
            local_echo: false,
            line_ending: 'cr',
            dtr: false,
            rts: false
        });
        const ERROR_KEYS = Object.freeze({
            unauthorized: 'desktop.qc_serial_permission_disabled',
            serial_disabled: 'desktop.qc_serial_permission_disabled',
            serial_ports_unavailable: 'desktop.qc_serial_open_failed',
            invalid_open: 'desktop.qc_serial_open_failed',
            invalid_control: 'desktop.qc_serial_open_failed',
            invalid_options: 'desktop.qc_serial_open_failed',
            invalid_port: 'desktop.qc_serial_unavailable_port',
            port_not_found: 'desktop.qc_serial_unavailable_port',
            port_busy: 'desktop.qc_serial_port_busy',
            open_required: 'desktop.qc_serial_open_failed',
            serial_read_failed: 'desktop.qc_serial_connection_lost',
            serial_write_failed: 'desktop.qc_serial_connection_lost',
            serial_control_failed: 'desktop.qc_serial_open_failed',
            idle_timeout: 'desktop.qc_serial_disconnected',
            disconnected: 'desktop.qc_serial_connection_lost',
            open_failed: 'desktop.qc_serial_open_failed'
        });

        function parseHex(value) {
            const compact = String(value || '').replace(/\s+/g, '');
            if (!compact || compact.length % 2 || !/^[0-9a-f]+$/i.test(compact)) return null;
            const bytes = new Uint8Array(compact.length / 2);
            for (let i = 0; i < bytes.length; i++) bytes[i] = parseInt(compact.slice(i * 2, i * 2 + 2), 16);
            return bytes;
        }

        function formatHex(bytes) {
            return Array.from(bytes || [], byte => Number(byte).toString(16).padStart(2, '0').toUpperCase()).join(' ');
        }

        function normalizeOptions(value, source) {
            const input = value && typeof value === 'object' ? value : {};
            const baud = Number(input.baud_rate);
            return {
                baud_rate: Number.isInteger(baud) && baud >= 1 && baud <= 4000000 ? baud : DEFAULT_OPTIONS.baud_rate,
                data_bits: input.data_bits === 7 ? 7 : 8,
                stop_bits: input.stop_bits === 2 ? 2 : 1,
                parity: ['none', 'even', 'odd'].includes(input.parity) ? input.parity : 'none',
                flow_control: source === 'browser' && input.flow_control === 'hardware' ? 'hardware' : 'none',
                local_echo: input.local_echo === true,
                line_ending: ['none', 'cr', 'lf', 'crlf'].includes(input.line_ending) ? input.line_ending : 'cr',
                dtr: input.dtr === true,
                rts: input.rts === true
            };
        }

        function normalizeProfile(value) {
            if (!value || typeof value !== 'object') return null;
            const source = value.source === 'browser' || value.source === 'host' ? value.source : '';
            const name = String(value.name || '').trim();
            const id = String(value.id || '');
            const port = String(value.port || '').trim();
            const hasVendor = value.usb_vendor_id !== undefined && value.usb_vendor_id !== null;
            const hasProduct = value.usb_product_id !== undefined && value.usb_product_id !== null;
            const vendor = hasVendor ? Number(value.usb_vendor_id) : undefined;
            const product = hasProduct ? Number(value.usb_product_id) : undefined;
            const validUSBID = id => Number.isInteger(id) && id >= 0 && id <= 0xFFFF;
            if (!source || !/^[A-Za-z0-9_-]{1,64}$/.test(id) || !name || Array.from(name).length > 80 || /[\u0000-\u001F\u007F-\u009F]/.test(name) || new TextEncoder().encode(port).byteLength > 256 || /[\u0000-\u001F\u007F-\u009F]/.test(port)) return null;
            if ((hasVendor && !validUSBID(vendor)) || (hasProduct && !validUSBID(product)) || (hasProduct && !hasVendor)) return null;
            return {
                id,
                name,
                source,
                port,
                ...(hasVendor ? { usb_vendor_id: vendor } : {}),
                ...(hasProduct ? { usb_product_id: product } : {}),
                options: normalizeOptions(value.options, source)
            };
        }

        function parseProfiles(raw) {
            try {
                const stored = typeof raw === 'string' ? JSON.parse(raw) : raw;
                if (!stored || stored.version !== 1 || !Array.isArray(stored.profiles)) return [];
                const seen = new Set();
                return stored.profiles.slice(0, MAX_PROFILES).map(normalizeProfile).filter(profile => {
                    if (!profile || seen.has(profile.id)) return false;
                    seen.add(profile.id);
                    return true;
                });
            } catch (_) {
                return [];
            }
        }

        function parseHexID(value) {
            const text = String(value || '').trim().replace(/^0x/i, '');
            if (!text) return undefined;
            if (!/^[0-9a-f]{1,4}$/i.test(text)) return null;
            return parseInt(text, 16);
        }

        // Maps a server or transport error code to its translation key.
        function errorKey(code, fallback) {
            return ERROR_KEYS[code] || fallback || 'desktop.qc_serial_open_failed';
        }

        // Reads and validates the profile form. Keeps the selected profile ID or
        // creates one; throws an Error carrying the translated field label.
        function readProfileDraft(form, selectedId, tr) {
            const name = form.elements.name.value.trim();
            const source = form.elements.source.value;
            const baudRate = form.elements.baud_preset.value === 'custom' ? Number(form.elements.baud_custom.value) : Number(form.elements.baud_preset.value);
            const usbVendor = parseHexID(form.elements.usb_vendor_id.value);
            const usbProduct = parseHexID(form.elements.usb_product_id.value);
            if (!name || Array.from(name).length > 80 || /[\u0000-\u001F\u007F-\u009F]/.test(name)) throw new Error(tr('desktop.qc_serial_profile_name'));
            if (!['browser', 'host'].includes(source)) throw new Error(tr('desktop.qc_serial_source'));
            if (source === 'host' && !form.elements.port.value) throw new Error(tr('desktop.qc_serial_choose_port'));
            if (!Number.isInteger(baudRate) || baudRate < 1 || baudRate > 4000000) throw new Error(tr('desktop.qc_serial_baud_rate'));
            if (usbVendor === null || usbProduct === null || (usbProduct !== undefined && usbVendor === undefined)) throw new Error(tr('desktop.qc_serial_usb_filter_invalid'));
            const profile = {
                id: selectedId || (crypto.randomUUID ? crypto.randomUUID() : `serial-${Date.now()}-${Math.random().toString(16).slice(2)}`),
                name,
                source,
                port: source === 'host' ? form.elements.port.value : '',
                options: normalizeOptions({
                    baud_rate: baudRate,
                    data_bits: Number(form.elements.data_bits.value),
                    stop_bits: Number(form.elements.stop_bits.value),
                    parity: form.elements.parity.value,
                    flow_control: form.elements.flow_control.value,
                    local_echo: form.elements.local_echo.checked,
                    line_ending: form.elements.line_ending.value,
                    dtr: form.elements.dtr.checked,
                    rts: form.elements.rts.checked
                }, source)
            };
            if (usbVendor !== undefined) profile.usb_vendor_id = usbVendor;
            if (usbProduct !== undefined) profile.usb_product_id = usbProduct;
            return profile;
        }

        return { STORAGE_KEY, MAX_PROFILES, MAX_PROFILE_BYTES, BAUD_PRESETS, DEFAULT_OPTIONS, parseHex, formatHex, normalizeOptions, normalizeProfile, parseProfiles, errorKey, readProfileDraft };
    })();
