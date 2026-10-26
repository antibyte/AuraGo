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
                    {id:'ssh-lab',name:'SSH lab',type:'router',protocol:'ssh',ip_address:'192.0.2.10',port:2222},
                    {id:'vnc-lab',name:'VNC lab',type:'desktop',protocol:'vnc',ip_address:'192.0.2.11',port:5901},
                    ...['generic','server','linux','vm',''].map(type=>({id:'cast-'+(type||'untyped'),name:'Cast '+(type||'untyped'),type,protocol:'none',ip_address:'192.0.2.12',port:8009})),
                    {id:'http-server',name:'HTTP server',type:'server',protocol:'http',ip_address:'192.0.2.13',port:80}
                ];
                const api=window.api;
                window.api=(url,options)=>url==='/api/desktop/integrations/devices'?Promise.resolve({devices}):api(url,options);
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
                const noConnection=serialShell.sockets.length===0&&serialShell.rfbs.length===0;
                serialShell.cleanup();
                return {all,ssh,vnc,searchHidesUnconnectable,serialProfiles,targeted,targetDisabled,noConnection,errors:window.__shellErrors};
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
			for _, key := range []string{"searchHidesUnconnectable", "serialProfiles", "targetDisabled", "noConnection"} {
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
