package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopQuickConnectUsesProtocolNeutralCopyAndFilters(t *testing.T) {
	t.Parallel()

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.Join("lang", "desktop", lang+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if strings.EqualFold(strings.TrimSpace(values["desktop.qc_title"]), "SSH") {
			t.Fatalf("%s keeps SSH-only Quick Connect title", path)
		}
		if strings.Contains(strings.ToLower(values["desktop.qc_no_devices"]), "ssh server") ||
			strings.Contains(strings.ToLower(values["desktop.qc_no_devices"]), "ssh-server") {
			t.Fatalf("%s keeps SSH-only Quick Connect empty state: %q", path, values["desktop.qc_no_devices"])
		}
	}

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	body := jsFunctionBodyInWindowMenuTest(t, mainText, "function renderQuickConnect(id, context)")
	for _, marker := range []string{
		`data-qc-filter="all"`,
		`data-qc-filter="ssh"`,
		`data-qc-filter="vnc"`,
		"let activeProtocolFilter = 'all'",
		"function protocolMatchesFilter(device)",
	} {
		if !strings.Contains(body, marker) {
			t.Fatalf("quick connect missing protocol filter marker %q", marker)
		}
	}
}

func TestDesktopQuickConnectOnlyListsRemoteDevicesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	for _, theme := range []string{"standard", "fruity"} {
		t.Run(theme, func(t *testing.T) {
			page := newQuickConnectShellPage(t, browser, theme, 1200, 800)
			defer page.Close()
			result := page.MustEval(`async()=>{
                serialShell.cleanup();
                const devices=[
                    {id:'ssh-lab',name:'SSH lab',type:'router',protocol:'ssh',quick_connect:true,ip_address:'192.0.2.10',port:2222},
                    {id:'vnc-lab',name:'VNC lab',type:'desktop',protocol:'vnc',quick_connect:true,ip_address:'192.0.2.11',port:5901},
                    ...[5500,1910,8008,8060,8009].map(port=>({id:'legacy-'+port,name:'Legacy discovery '+port,type:'server',protocol:'ssh',quick_connect:false,ip_address:'192.0.2.14',port})),
                    {id:'unchecked',name:'Unchecked SSH flag',type:'server',protocol:'ssh',ip_address:'192.0.2.15',port:22},
                    ...['generic','server','linux','vm',''].map(type=>({id:'cast-'+(type||'untyped'),name:'Cast '+(type||'untyped'),type,protocol:'none',ip_address:'192.0.2.12',port:8009})),
                    {id:'http-server',name:'HTTP server',type:'server',protocol:'http',ip_address:'192.0.2.13',port:80}
                ];
                const api=window.api;
                window.api=(url,options)=>url==='/api/desktop/integrations/devices?quick_connect=1'?Promise.resolve({devices}):api(url,options);
                serialShell.app=window.__renderQuickConnect('qc');
                const ids=()=>Array.from(document.querySelectorAll('[data-device-id]')).map(node=>node.dataset.deviceId).sort().join(',');
                const filter=value=>document.querySelector('[data-qc-filter="'+value+'"]').click();
                await serialShell.wait(()=>document.querySelector('[data-device-id="ssh-lab"]')&&document.querySelector('[data-serial-profile]'));
                const all=ids();
                filter('ssh');const ssh=ids();
                filter('vnc');const vnc=ids();
                filter('all');
                const search=document.querySelector('.vd-qc-search input');
                search.value='Cast';search.dispatchEvent(new Event('input'));
                const searchHidesUnconnectable=ids()==='';
                search.value='';search.dispatchEvent(new Event('input'));
                filter('serial');
                const serialProfiles=!document.querySelector('[data-serial-list]').hidden&&!!document.querySelector('[data-serial-profile]')&&document.querySelector('[data-device-list]').hidden;
                filter('all');
                applyQuickConnectLaunchContext('qc',{device_id:'cast-generic'});
                await serialShell.wait(()=>document.querySelector('[data-qc-target] button'));
                const targeted=ids();
                const targetDisabled=document.querySelector('[data-qc-target] button').disabled;
                document.querySelector('[data-device-id="cast-generic"]').click();
                applyQuickConnectLaunchContext('qc',{device_id:'legacy-8009'});
                await serialShell.wait(()=>document.querySelector('[data-device-id="legacy-8009"]'));
                const legacyTargetDisabled=document.querySelector('[data-qc-target] button').disabled;
                document.querySelector('[data-device-id="legacy-8009"]').click();
                const noConnection=serialShell.sockets.length===0&&serialShell.rfbs.length===0;
                serialShell.cleanup();
                return {all,ssh,vnc,searchHidesUnconnectable,serialProfiles,targeted,targetDisabled,legacyTargetDisabled,noConnection,errors:window.__shellErrors};
            }`)
			for key, want := range map[string]string{
				"all":      "__aurago-host__,ssh-lab,vnc-lab",
				"ssh":      "__aurago-host__,ssh-lab",
				"vnc":      "vnc-lab",
				"targeted": "__aurago-host__,cast-generic,ssh-lab,vnc-lab",
			} {
				if got := result.Get(key).Str(); got != want {
					t.Errorf("%s devices = %q, want %q", key, got, want)
				}
			}
			for _, key := range []string{"searchHidesUnconnectable", "serialProfiles", "targetDisabled", "legacyTargetDisabled", "noConnection"} {
				if !result.Get(key).Bool() {
					t.Errorf("%s failed: %s", key, result.JSON("", ""))
				}
			}
			if len(result.Get("errors").Arr()) != 0 {
				t.Errorf("browser errors: %s", result.Get("errors").JSON("", ""))
			}
		})
	}
}

func TestDesktopQuickConnectVNCSessionToolbarMarkers(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	body := jsFunctionBodyInWindowMenuTest(t, mainText, "function renderQuickConnect(id, context)")
	for _, marker := range []string{
		"function renderVNCToolbar(",
		"function setVNCStatus(",
		`class="vd-qc-vnc-toolbar"`,
		`data-vnc-scale="fit"`,
		`data-vnc-scale="native"`,
		`data-vnc-action="view-only"`,
		`data-vnc-action="ctrl-alt-del"`,
		"rfb.sendCtrlAltDel",
		"desktop.qc_vnc_status_connected",
		"desktop.qc_vnc_status_error",
	} {
		if !strings.Contains(body, marker) {
			t.Fatalf("quick connect missing VNC toolbar marker %q", marker)
		}
	}

	css := readAllDesktopAppCSS(t)
	for _, marker := range []string{
		".vd-qc-vnc-toolbar",
		".vd-qc-vnc-status",
		".vd-qc-vnc-toolbar .vd-qc-btn",
		"min-height: 44px",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("quick connect css missing VNC toolbar marker %q", marker)
		}
	}
}

func TestDesktopQuickConnectVNCErrorCodeCopyMarkers(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	body := jsFunctionBodyInWindowMenuTest(t, mainText, "function renderQuickConnect(id, context)")
	for _, marker := range []string{
		"function vncErrorText(",
		"desktop.qc_vnc_error_device_not_found",
		"desktop.qc_vnc_error_protocol_mismatch",
		"desktop.qc_vnc_error_credential_unavailable",
		"desktop.qc_vnc_error_dial_failed",
		"desktop.qc_vnc_error_auth_failed",
		"desktop.qc_vnc_error_init_failed",
	} {
		if !strings.Contains(body, marker) {
			t.Fatalf("quick connect missing VNC error mapping marker %q", marker)
		}
	}
}

func TestDesktopQuickConnectVNCPreservesErrorOnDisconnectMarkers(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	body := jsFunctionBodyInWindowMenuTest(t, mainText, "function renderQuickConnect(id, context)")
	for _, marker := range []string{
		"let lastVNCError = null",
		"JSON.parse(reason)",
		"parsed.code",
		"parsed.message",
		"lastVNCError = message",
		"if (lastVNCError)",
		"setVNCStatus(sessionEl, 'error', lastVNCError)",
		"disconnectPlaceholderHTML('desktop.qc_vnc_connection_error', deviceId, true, lastVNCError)",
	} {
		if !strings.Contains(body, marker) {
			t.Fatalf("quick connect missing VNC persistent error marker %q", marker)
		}
	}
}

func TestDesktopQuickConnectConsoleContract(t *testing.T) {
	t.Parallel()

	consoleText := readDesktopAssetText(t, "js/desktop/apps/quickconnect-console.js")
	shellText := readDesktopAssetText(t, "js/desktop/apps/quickconnect-launchpad-chat.js")
	for _, marker := range []string{
		"data-qc-statusbar", "data-qc-status-label", "data-qc-status-proto", "data-qc-status-target",
		"data-qc-status-time", "data-qc-status-size", "data-qc-phosphor-toggle", "data-qc-phosphor-label",
		"'role', 'dialog'", "aria-modal", "aurago.quickConnect.phosphor", "function quickConnectTerminalTheme(",
	} {
		if !strings.Contains(consoleText, marker) {
			t.Errorf("quickconnect-console.js missing marker %q", marker)
		}
	}
	for _, marker := range []string{
		`aria-pressed="true"`, "data-conn=", "data-qc-add-server", "data-qc-clear-search", "data-qc-retry",
		`role="tablist"`, `role="tab"`, "data-qc-session-name", "data-qc-devices-head", "data-qc-device-count",
		"QuickConnectConsole.openDialog", "cursorStyle: 'block'",
		"QuickConnectConsole.cleanTerminalText(t('desktop.qc_connecting')", "QuickConnectConsole.cleanTerminalText(msg.message)",
	} {
		if !strings.Contains(shellText, marker) {
			t.Errorf("quickconnect-launchpad-chat.js missing marker %q", marker)
		}
	}
	if strings.Contains(shellText, "data-qc-new-serial") {
		t.Error("the sidebar no longer has a New serial connection button")
	}

	newKeys := []string{
		"section_targets", "status_ready", "status_connecting", "status_connected", "status_disconnected",
		"status_error", "no_results", "clear_search", "hint_navigate", "hint_search", "badge_no_credential",
		"phosphor_label", "phosphor_auto", "phosphor_green", "phosphor_amber", "phosphor_ice",
		"session_time", "terminal_size",
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var values map[string]string
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/"+lang+".json")), &values); err != nil {
			t.Fatalf("parse %s: %v", lang, err)
		}
		for _, key := range newKeys {
			if strings.TrimSpace(values["desktop.qc_"+key]) == "" {
				t.Errorf("%s: desktop.qc_%s is empty", lang, key)
			}
		}
		if !strings.Contains(values["desktop.qc_phosphor_label"], "{{name}}") {
			t.Errorf("%s: phosphor label lost its {{name}} placeholder", lang)
		}
	}
}

func TestDesktopQuickConnectConsoleBehaviourBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := newQuickConnectShellPage(t, browser, "standard", 1200, 800)
	defer page.Close()
	result := page.MustEval(`async()=>{
        const key=(target,value)=>target.dispatchEvent(new KeyboardEvent('keydown',{key:value,bubbles:true,cancelable:true}));
        const search=()=>document.querySelector('.vd-qc-search input');
        const root=()=>document.querySelector('.vd-quick-connect');
        await serialShell.wait(()=>document.querySelector('[data-device-id="ssh-lab"]'));
        const out={};
        // (a) slash focuses the search, ArrowDown reaches the first row
        document.querySelector('[data-device-id]').focus();
        key(document.activeElement,'/');
        out.slashFocusesSearch=document.activeElement===search();
        key(search(),'ArrowDown');
        out.arrowReachesRow=document.activeElement===document.querySelector('[data-device-id]');
        // (b) no results and the clear button
        search().value='zzz-none';search().dispatchEvent(new Event('input'));
        out.noResults=!!document.querySelector('[data-device-list] .vd-empty')&&document.querySelector('[data-qc-device-count]').textContent==='00';
        document.querySelector('[data-qc-clear-search]').click();
        out.clearWorks=search().value===''&&document.activeElement===search()&&!!document.querySelector('[data-device-id="ssh-lab"]');
        // (c) server dialog
        const add=document.querySelector('[data-qc-add-server]');
        add.focus();add.click();
        const dialog=document.querySelector('.vd-qc-modal-overlay .vd-qc-modal');
        out.dialogRole=!!dialog&&dialog.getAttribute('role')==='dialog'&&dialog.getAttribute('aria-modal')==='true'&&!!document.getElementById(dialog.getAttribute('aria-labelledby'))&&dialog.contains(document.activeElement);
        dialog.querySelector('[data-action="cancel"]').focus();
        key(document.activeElement,'/');
        out.slashStaysInDialog=dialog.contains(document.activeElement);
        key(document.activeElement,'Escape');
        out.escapeClosesAndRestoresFocus=!document.querySelector('.vd-qc-modal-overlay')&&document.activeElement===add;
        // (d) phosphor cycle survives a re-render
        const toggle=()=>document.querySelector('[data-qc-phosphor-toggle]');
        const seen=[root().dataset.qcPhosphor];
        toggle().click();seen.push(root().dataset.qcPhosphor);
        toggle().click();seen.push(root().dataset.qcPhosphor);
        serialShell.cleanup();
        serialShell.app=window.__renderQuickConnect('qc');
        await serialShell.wait(()=>document.querySelector('[data-device-id="ssh-lab"]'));
        out.phosphorCycle=seen.join(',')==='auto,green,amber';
        out.phosphorSurvives=root().dataset.qcPhosphor==='amber';
        toggle().click();toggle().click();
        out.phosphorBackToAuto=root().dataset.qcPhosphor==='auto';
        // (e) SSH row marking survives typing into the search field
        document.querySelector('[data-device-id="ssh-lab"]').click();
        await serialShell.wait(()=>serialShell.sockets.length===1);
        const row=()=>document.querySelector('[data-device-id="ssh-lab"]');
        out.connecting=row().classList.contains('active')&&row().dataset.conn==='connecting'&&document.querySelector('[data-qc-statusbar]').dataset.state==='connecting';
        await serialShell.sockets[0].onmessage({data:new Uint8Array([65]).buffer});
        search().value='ssh';search().dispatchEvent(new Event('input'));
        out.markSurvivesSearch=row().classList.contains('active')&&row().dataset.conn==='connected'&&document.querySelector('[data-qc-statusbar]').dataset.state==='connected';
        serialShell.cleanup();
        const dirty=['A',String.fromCharCode(27),'[2Jb',String.fromCharCode(7),String.fromCharCode(252),String.fromCharCode(0x9b),'X',String.fromCharCode(13,10)].join('');
        out.cleanText=QuickConnectConsole.cleanTerminalText(dirty)==='A[2Jb'+String.fromCharCode(252)+'X';
        out.errors=window.__shellErrors;
        return out;
    }`)
	for _, key := range []string{"slashFocusesSearch", "arrowReachesRow", "noResults", "clearWorks", "dialogRole", "slashStaysInDialog", "escapeClosesAndRestoresFocus", "phosphorCycle", "cleanText", "phosphorSurvives", "phosphorBackToAuto", "connecting", "markSurvivesSearch"} {
		if !result.Get(key).Bool() {
			t.Errorf("%s failed: %s", key, result.JSON("", ""))
		}
	}
	if len(result.Get("errors").Arr()) != 0 {
		t.Errorf("browser errors: %s", result.Get("errors").JSON("", ""))
	}
}
