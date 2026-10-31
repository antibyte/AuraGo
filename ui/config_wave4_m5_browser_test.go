package ui

// Wave 4 package M5: dialogs, delete buttons, list states and migrated sections of chromecast, devices,
// bluetooth, network_shares, remote_control, tailscale, go2rtc, mqtt, meshcore and the form-builder modules.

import "testing"

func TestConfigWave4M5DialogsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOriginWith(t, "en", true, nil) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card') && typeof cfgModal === 'function'`)
	page.MustSetViewport(1440, 900, 1, false)

	run := func(name, expr string) {
		t.Helper()
		got := page.MustEval(expr).String()
		t.Logf("%s => %s", name, got)
		if got != "ok" {
			t.Errorf("%s failed: %s", name, got)
		}
	}

	run("chromecast dialog and delete button", `async () => {
		const original = window.fetch;
		window.fetch = async url => String(url) === '/api/devices'
			? new Response(JSON.stringify([{id:'cc1', name:'Wohnzimmer', type:'chromecast', ip_address:'10.0.0.9', port:8009}]), {status: 200})
			: original(url);
		await selectSection('chromecast', {scrollBehavior:'auto'});
		await new Promise(r => setTimeout(r, 300));
		const row = document.querySelector('#cfgdev-tbody tr');
		if (!row) return 'no device row';
		const del = row.querySelector('.cfg-delete-btn');
		if (!del || !del.getAttribute('aria-label')) return 'no delete button with name';
		ccShowAddManual();
		const m = document.getElementById('cfgdev-device-dialog');
		if (!m || !m.classList.contains('open')) return 'dialog not open';
		if (!m.querySelector('label[for="cfgdev-field-name"]')) return 'name field unlabeled';
		document.getElementById('cfgdev-save-btn').click();
		await new Promise(r => setTimeout(r, 50));
		const err = document.getElementById('cfgdev-form-error');
		if (err.classList.contains('is-hidden') || err.getAttribute('role') !== 'alert') return 'no inline name error';
		closeModal('cfgdev-device-dialog');
		window.fetch = original;
		return document.getElementById('cfgdev-device-dialog') ? 'dialog not removed' : 'ok';
	}`)
	run("devices dialog", `async () => {
		const original = window.fetch;
		window.fetch = async url => String(url) === '/api/devices' ? new Response('[]', {status: 200}) : (String(url) === '/api/credentials' ? new Response('[]', {status: 200}) : original(url));
		// devices is not in the section menu; load and render it directly.
		await new Promise((resolve, reject) => { const s = document.createElement('script'); s.src = '/cfg/devices.js'; s.onload = resolve; s.onerror = reject; document.head.appendChild(s); });
		await renderDevicesSection({label: 'Devices', desc: ''});
		await new Promise(r => setTimeout(r, 200));
		if (!document.querySelector('#devices-state .pw-empty-state')) return 'no empty state';
		await devicesShowModal();
		const m = document.getElementById('device-dialog');
		if (!m || !m.classList.contains('open')) return 'dialog not open';
		if (!m.querySelector('label[for="device-field-ip"]') || !document.getElementById('device-form-error')) return 'missing label or error line';
		closeModal('device-dialog');
		window.fetch = original;
		return document.getElementById('device-dialog') ? 'dialog not removed' : 'ok';
	}`)
	run("bluetooth pin dialog", `async () => {
		configData.bluetooth = {enabled: true, readonly: false};
		await selectSection('bluetooth', {scrollBehavior:'auto'});
		resetDirtySnapshot();
		btOpenPINModal('AA:BB:CC:DD:EE:FF');
		const m = document.getElementById('bt-pin-dialog');
		if (!m || !m.classList.contains('open')) return 'dialog not open';
		if (document.getElementById('bt-pin-address').value !== 'AA:BB:CC:DD:EE:FF') return 'address lost';
		closeModal('bt-pin-dialog');
		return document.getElementById('bt-pin-dialog') || document.getElementById('bt-pin-value') ? 'pin lingers' : 'ok';
	}`)
	run("network shares dialog toggles", `async () => {
		configData.network_shares = {enabled: true, readonly: false, allow_create: true};
		await selectSection('network_shares', {scrollBehavior:'auto'});
		resetDirtySnapshot();
		nsPermissions = {smb_enabled: true, nfs_enabled: true, smb_allow_guest: true, allowed_principals: ['users'], allowed_clients: ['10.0.0.0/24']};
		nsRuntime = {usable: true, smb: {writable: true, backend: 'samba'}, nfs: {writable: true}};
		nsOpenShareModal('');
		const m = document.getElementById('ns-share-dialog');
		if (!m || !m.classList.contains('open')) return 'dialog not open';
		if (m.querySelector('input[type=checkbox]:not([data-cfg-choice])')) return 'native checkbox left';
		const ro = document.getElementById('ns-form-readonly');
		if (!ro.classList.contains('on')) return 'read-only should default on';
		ro.click();
		if (ro.classList.contains('on') || ro.getAttribute('aria-checked') !== 'false') return 'switch did not flip';
		if (isDirty) return 'dialog switch marked the config dirty';
		document.getElementById('ns-form-protocol').value = 'nfs'; nsRenderAccessFields();
		if (!m.querySelector('input.ns-client[data-cfg-choice]')) return 'client choice list missing marker';
		closeModal('ns-share-dialog');
		return document.getElementById('ns-share-dialog') ? 'dialog not removed' : 'ok';
	}`)
	run("remote control device switch", `async () => {
		const original = window.fetch;
		window.fetch = async (url, opts) => String(url) === '/api/remote/devices'
			? new Response(JSON.stringify([{id:'d1', name:'PC', os:'linux', arch:'amd64', status:'connected', is_connected:true, read_only:false, allowed_paths:[], effective_allowed_paths:[], capabilities:[], config_state:'confirmed'}]), {status: 200})
			: (String(url) === '/api/remote/platforms' ? new Response('[{"os":"linux","arch":"amd64","available":true}]', {status: 200}) : original(url, opts));
		configData.remote_control = {enabled: true};
		await selectSection('remote_control', {scrollBehavior:'auto'});
		await new Promise(r => setTimeout(r, 400));
		const sw = document.querySelector('[data-rc-readonly]');
		if (!sw || sw.getAttribute('role') !== 'switch') return 'no device switch';
		if (document.querySelector('#content input[type=checkbox]')) return 'native checkbox left';
		sw.click();
		if (!sw.classList.contains('on')) return 'switch did not flip';
		const del = document.querySelector('[data-rc-action="delete"]');
		if (!del || !del.classList.contains('btn-danger')) return 'delete is not a danger button';
		if (!document.querySelector('#rc-platform-list .rc-platform-btn')) return 'platform buttons missing';
		window.fetch = original;
		return 'ok';
	}`)
	run("tailscale delete button and icons", `async () => {
		configData.tailscale = {enabled: true, tsnet: {enabled: true}};
		const original = window.fetch;
		window.fetch = async url => String(url).startsWith('/api/tsnet/status')
			? new Response(JSON.stringify({running: true, serving_http: true, nodes: {main: {configured: true, health: 'ready'}}}), {status: 200})
			: (String(url).startsWith('/api/tailscale/status') ? new Response('{"status":"disabled"}', {status: 200}) : original(url));
		await selectSection('tailscale', {scrollBehavior:'auto'});
		await new Promise(r => setTimeout(r, 400));
		const del = document.getElementById('tsnet-node-delete-main');
		if (!del || !del.classList.contains('btn-danger')) return 'node key delete is not a danger button';
		if (/[\u{1F300}-\u{1FAFF}\u{2600}-\u{27BF}]/u.test(document.getElementById('content').textContent)) return 'emoji in section text';
		window.fetch = original;
		return 'ok';
	}`)
	run("go2rtc remove stream", `async () => {
		configData.go2rtc = {enabled: true, streams: [{id: 'cam', name: 'Cam', source: 'rtsp://x', source_configured: true}]};
		AuraConfigState.init(configData);
		await selectSection('go2rtc', {scrollBehavior:'auto'});
		await new Promise(r => setTimeout(r, 300));
		const remove = document.querySelector('.go2rtc-stream-card .cfg-delete-btn');
		if (!remove) return 'remove is not a delete button';
		return 'ok';
	}`)
	run("mqtt message list state", `async () => {
		configData.mqtt = {enabled: false};
		await selectSection('mqtt', {scrollBehavior:'auto'});
		if (!document.querySelector('#mqtt-messages-list .pw-empty-state')) return 'no empty state';
		if (/[\u{1F300}-\u{1FAFF}]/u.test(document.getElementById('content').textContent)) return 'emoji in section text';
		return 'ok';
	}`)
	run("form builder sections", `async () => {
		const original = window.fetch;
		window.fetch = async url => /\/api\/[a-z0-9-]+\/status/.test(String(url)) ? new Response('{"status":"disabled"}', {status: 200}) : original(url);
		const expectations = {
			truenas: ['truenas-apikey', 'truenas-test-btn', 'truenas-test-result', 'truenas.use_https'],
			jellyfin: ['jellyfin-apikey', 'jellyfin-test-btn', 'jellyfin-test-result', 'jellyfin.port'],
			uptime_kuma: ['uptime-kuma-api-key', 'uptime-kuma-test-btn', 'uptime-kuma-test-result', 'uptime_kuma.relay_instruction'],
			ldap: ['ldap-password', 'ldap-test-btn', 'ldap-test-result', 'ldap-transport'],
			telnyx: ['telnyx-api-key', 'telnyx-test-btn', 'telnyx-test-result', 'telnyx-allowed-numbers', 'telnyx.voice_language'],
			fritzbox: ['fb-password', 'fb-test-btn', 'fb-test-result', 'fb-feature-title-system', 'fb-poll-interval']
		};
		for (const [key, ids] of Object.entries(expectations)) {
			await selectSection(key, {scrollBehavior:'auto'});
			await new Promise(r => setTimeout(r, 250));
			for (const id of ids) {
				if (!document.getElementById(id) && !document.querySelector('[data-path="' + id + '"]')) { window.fetch = original; return key + ' misses ' + id; }
			}
			if (!document.querySelector('#content .pw-panel')) { window.fetch = original; return key + ' not on the form builder'; }
			if (document.querySelector('#content .toggle-wrap .toggle[data-path$=".enabled"]') === null && key !== 'telnyx') { window.fetch = original; return key + ' enable toggle missing'; }
		}
		window.fetch = original;
		return 'ok';
	}`)
}
