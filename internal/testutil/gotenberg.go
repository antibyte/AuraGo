package testutil

import "net/http"

// GotenbergDebugResponse follows the v8.38.0 /debug wire format: flags are strings.
const GotenbergDebugResponse = `{"version":"8.38.0","flags":{
"chromium-deny-private-ips":"true","libreoffice-deny-private-ips":"true",
"webhook-deny-private-ips":"true","api-download-from-deny-private-ips":"true",
"chromium-enable-environment-proxy":"false","libreoffice-enable-environment-proxy":"false",
"webhook-enable-environment-proxy":"false","api-download-from-enable-environment-proxy":"false",
"chromium-disable-web-security":"false","chromium-allow-file-access-from-files":"false",
"chromium-shared-browser-context":"false","chromium-ignore-certificate-errors":"false",
"chromium-allow-insecure-localhost":"false","chromium-allow-list":"[]",
"libreoffice-allow-list":"[]","webhook-allow-list":"[]","api-download-from-allow-list":"[]",
"chromium-proxy-server":"","chromium-host-resolver-rules":""}}`

func ServeGotenbergDebug(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet || r.URL.Path != "/debug" {
		return false
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(GotenbergDebugResponse))
	return true
}
