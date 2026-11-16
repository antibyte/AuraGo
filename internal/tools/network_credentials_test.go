package tools

import (
	"aurago/internal/security"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestCredentialAPIClientsRejectCrossOriginRedirects(t *testing.T) {
	hits := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++; io.WriteString(w, "leaked") }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	for name, client := range map[string]*http.Client{"brave": braveHTTPClient, "cloudflare": cfHTTPClient, "vercel": vercelHTTPClient, "notifications": notifyHTTPClient, "virustotal": virustotalHTTPClient, "adguard": adguardClientFactory()} {
		t.Run(name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodPost, source.URL, strings.NewReader("private body"))
			req.Header.Set("X-Api-Key", "fixture-secret")
			resp, err := client.Do(req)
			if resp != nil {
				resp.Body.Close()
			}
			if err == nil && resp.StatusCode < 300 {
				t.Error("redirect succeeded")
			}
		})
	}
	if hits != 0 {
		t.Fatalf("foreign origin received %d requests", hits)
	}
}

func TestIntegrationCredentialsRegisteredBeforeRequestAndScrubbedFromErrors(t *testing.T) {
	for _, name := range []string{"github", "tailscale", "koofr", "adguard", "vercel", "virustotal", "brave"} {
		t.Run(name, func(t *testing.T) {
			secret := "fixture-" + name + "-credential+with space"
			transport := publicationTransportFunc(func(req *http.Request) (*http.Response, error) {
				if security.Scrub(secret) == secret {
					t.Fatal("credential was not registered before network use")
				}
				body := secret + " " + url.QueryEscape(secret)
				return &http.Response{StatusCode: 400, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			fixture := &http.Client{Transport: transport}
			var output string
			var err error
			switch name {
			case "github":
				old := githubHTTPClient
				githubHTTPClient = fixture
				defer func() { githubHTTPClient = old }()
				raw, _, _, callErr := githubRequest(context.Background(), GitHubConfig{Token: secret}, http.MethodGet, "/user", nil)
				output = string(raw)
				err = callErr
			case "tailscale":
				old := tailscaleHTTPClient
				tailscaleHTTPClient = fixture
				defer func() { tailscaleHTTPClient = old }()
				raw, _, callErr := tailscaleRequest(TailscaleConfig{APIKey: secret}, http.MethodGet, "/api/v2/device/123", "")
				output = string(raw)
				err = callErr
			case "koofr":
				old := koofrHTTPClient
				koofrHTTPClient = fixture
				defer func() { koofrHTTPClient = old }()
				_, err = doKoofrRequest(http.MethodGet, "https://app.koofr.net/api/v2/mounts", "user", secret, "", nil)
			case "adguard":
				old := getAdGuardClient()
				adguardClient = fixture
				defer func() { adguardClient = old }()
				raw, _, callErr := adguardRequest(AdGuardConfig{URL: "http://adguard.test", Password: secret}, http.MethodGet, "/control/status", "")
				output = string(raw)
				err = callErr
			case "vercel":
				old := vercelHTTPClient
				vercelHTTPClient = fixture
				defer func() { vercelHTTPClient = old }()
				raw, _, callErr := vercelRequest(VercelConfig{Token: secret}, http.MethodGet, "/v9/projects", nil)
				output = string(raw)
				err = callErr
			case "virustotal":
				old := virustotalHTTPClient
				virustotalHTTPClient = fixture
				defer func() { virustotalHTTPClient = old }()
				_, err = virustotalDoRequest(secret, http.MethodGet, "https://www.virustotal.com/api/v3/files/fixture", nil, "")
			case "brave":
				old := braveHTTPClient
				braveHTTPClient = fixture
				defer func() { braveHTTPClient = old }()
				_, err = SearchBrave(context.Background(), secret, BraveSearchOptions{Query: "fixture"})
			}
			if err != nil {
				output += " " + err.Error()
			}
			if strings.Contains(output, secret) || strings.Contains(output, url.QueryEscape(secret)) {
				t.Fatal("provider error exposed a credential")
			}
		})
	}
}

func TestVirusTotalUploadURLBoundary(t *testing.T) {
	for _, raw := range []string{"https://www.virustotal.com/_ah/upload/id", "https://www.virustotal.com:443/upload?ticket=example"} {
		if err := validateVirusTotalUploadURL(raw); err != nil {
			t.Error(err)
		}
	}
	for _, raw := range []string{"http://www.virustotal.com/upload", "https://evil.test/upload", "https://www.virustotal.com.evil.test/", "https://www.virustotal.com:444/upload", "https://secret@www.virustotal.com/upload", "https://www.virustotal.com/upload#fragment", "//www.virustotal.com/upload"} {
		if err := validateVirusTotalUploadURL(raw); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
}

func TestVirusTotalRejectsUploadTargetBeforeReadingFile(t *testing.T) {
	previousClient, previousURL := virustotalHTTPClient, virustotalBaseURL
	defer func() { virustotalHTTPClient = previousClient; virustotalBaseURL = previousURL }()
	requests := 0
	virustotalBaseURL = "https://www.virustotal.com/api/v3"
	virustotalHTTPClient = &http.Client{Transport: publicationTransportFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"data":"https://evil.test/upload"}`))}, nil
	})}
	file, err := os.CreateTemp(t.TempDir(), "upload")
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	_, err = virustotalUploadOpenFile("fixture-virustotal-key", "fixture.bin", file, 33<<20, context.Background())
	if err == nil || !strings.Contains(err.Error(), "unsafe upload URL") {
		t.Fatalf("error=%v", err)
	}
	if requests != 1 {
		t.Fatalf("requests=%d", requests)
	}
}
