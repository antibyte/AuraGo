package fritzbox

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// TR-064 exports can advertise the router's LAN address while the configured
// connection uses a hostname, TLS endpoint or forwarded port. Only the known
// export path and opaque query belong to the resource; its authority must never
// select where AuraGo sends a request or credentials.
func TestFritzRouterGeneratedListsUseConfiguredOrigin(t *testing.T) {
	for _, tc := range []struct {
		name, action, key, path, xml string
		fetch                        func(*Client) error
	}{
		{"calls", "GetCallList", "NewCallListURL", "/calllist.lua", `<root><Call><Name>Fixture</Name></Call></root>`, func(c *Client) error {
			entries, err := c.GetCallList()
			if err == nil && (len(entries) != 1 || entries[0].Name != "Fixture") {
				return fmt.Errorf("unexpected call list: %v", entries)
			}
			return err
		}},
		{"phonebook", "GetPhonebook", "NewPhonebookURL", "/phonebook.lua", `<phonebooks><phonebook><contact><person><realName>Fixture</realName></person></contact></phonebook></phonebooks>`, func(c *Client) error {
			entries, err := c.GetPhonebookEntries(0)
			if err == nil && (len(entries) != 1 || entries[0].Name != "Fixture") {
				return fmt.Errorf("unexpected phonebook: %v", entries)
			}
			return err
		}},
		{"messages", "GetMessageList", "NewURL", "/tamcalllist.lua", `<Root><Message><Name>Fixture</Name></Message></Root>`, func(c *Client) error {
			entries, err := c.GetTAMList(0)
			if err == nil && (len(entries) != 1 || entries[0].Name != "Fixture") {
				return fmt.Errorf("unexpected message list: %v", entries)
			}
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var downloads atomic.Int32
			router := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasPrefix(r.URL.Path, "/upnp/control/"):
					fmt.Fprintf(w, `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:%sResponse xmlns:u="urn:fixture"><%s>http://192.0.2.1:49000%s?sid=export-fixture&amp;timestamp=123</%s></u:%sResponse></s:Body></s:Envelope>`, tc.action, tc.key, tc.path, tc.key, tc.action)
				case r.URL.Path == tc.path:
					if r.URL.Query().Get("sid") != "export-fixture" || r.URL.Query().Get("timestamp") != "123" {
						t.Error("export query was lost")
					}
					downloads.Add(1)
					fmt.Fprint(w, tc.xml)
				case r.URL.Path == "/login_sid.lua":
					http.Error(w, "SID unavailable in fixture", http.StatusForbidden)
				default:
					t.Errorf("unexpected router request: %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer router.Close()
			cfg := fritzConfigForEndpoint(t, router.URL)
			cfg.FritzBox.HTTPS = true
			cfg.FritzBox.InsecureSkipVerify = true // The fixture has a self-signed certificate.
			client, err := NewClient(cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			if err := tc.fetch(client); err != nil {
				t.Fatalf("configured router export rejected: %v", err)
			}
			if downloads.Load() != 1 {
				t.Fatalf("export must be read once at the configured router, got %d", downloads.Load())
			}
		})
	}
}

func TestFritzListURLRebindingIsLimitedToKnownExports(t *testing.T) {
	c := &Client{tr: &TR064Client{baseURL: "https://router.example:49443"}, webURL: "https://router.example"}
	for _, tc := range []struct {
		name, raw, want string
	}{
		{"relative", "/calllist.lua?sid=fixture", "https://router.example:49443/calllist.lua?sid=fixture"},
		{"trusted web origin", "https://router.example/calllist.lua?sid=fixture", "https://router.example/calllist.lua?sid=fixture"},
		{"advertised LAN origin", "http://192.0.2.1:49000/calllist.lua?sid=a%2Bb&days=2", "https://router.example:49443/calllist.lua?sid=a%2Bb&days=2"},
		{"foreign authority never used", "https://foreign.invalid/calllist.lua?sid=fixture", "https://router.example:49443/calllist.lua?sid=fixture"},
		{"wrong export", "https://foreign.invalid/phonebook.lua", ""},
		{"unknown path", "https://foreign.invalid/other", ""},
		{"encoded path", "https://foreign.invalid/%63alllist.lua", ""},
		{"traversal", "https://foreign.invalid/../calllist.lua", ""},
		{"protocol relative", "//foreign.invalid/calllist.lua", ""},
		{"userinfo", "https://user:fixture@foreign.invalid/calllist.lua", ""},
		{"fragment", "https://foreign.invalid/calllist.lua#fragment", ""},
		{"other scheme", "ftp://foreign.invalid/calllist.lua", ""},
		{"opaque", "https:calllist.lua", ""},
		{"empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := c.routerListURL(tc.raw, "/calllist.lua", c.tr.baseURL)
			if tc.want == "" {
				if err == nil {
					t.Fatal("invalid export accepted")
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %q, error %v, want %q", got, err, tc.want)
			}
		})
	}
}

func TestFritzReboundListStillRejectsForeignRedirect(t *testing.T) {
	var foreignRequests atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		foreignRequests.Add(1)
		fmt.Fprint(w, `<root/>`)
	}))
	defer foreign.Close()
	router := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, foreign.URL+"/calllist.lua", http.StatusFound)
	}))
	defer router.Close()
	c, err := NewClient(fritzConfigForEndpoint(t, router.URL))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.fetchCallListXML("http://192.0.2.1:49000/calllist.lua?sid=export-fixture"); err == nil {
		t.Fatal("redirect from rebound list accepted")
	}
	if foreignRequests.Load() != 0 {
		t.Fatal("foreign redirect received a request")
	}
}
