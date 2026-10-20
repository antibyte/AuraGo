package config

import (
	"net/url"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGo2RTCTuyaSources(t *testing.T) {
	for _, method := range []string{"smart", "cloud"} {
		c := Go2RTCTuyaCredentials{Email: "cam+test@example.org", Password: "pass&+#%ä"}
		if method == "cloud" {
			c = Go2RTCTuyaCredentials{ClientID: "client-id", ClientSecret: "key&+#%ä", UID: "uid123"}
		}
		source, err := Go2RTCTuyaSource(Go2RTCTuyaAccount{Method: method, Region: "eu-central"}, c, "device123", "hd")
		if err != nil || ValidateGo2RTCTuyaSource(source) != nil {
			t.Fatalf("%s source: %v", method, err)
		}
		u, _ := url.Parse(source)
		if method == "smart" && u.Query().Get("password") != c.Password {
			t.Fatal("password did not round trip")
		}
		for _, bad := range []string{source + "&exec=1", source + "&device_id=other", source + "#video=copy", strings.Replace(source, u.Host, "evil.example", 1), strings.Replace(source, u.Host, u.Host+":443", 1), strings.Replace(source, u.Host, u.Host+"/path", 1), source + "&" + map[string]string{"smart": "uid=", "cloud": "email="}[method]} {
			if ValidateGo2RTCTuyaSource(bad) == nil {
				t.Errorf("accepted invalid %s source", method)
			}
		}
	}
	data, err := yaml.Marshal(Go2RTCConfig{TuyaAccounts: []Go2RTCTuyaAccount{{ID: "tuya-home", Name: "Home", Method: "smart", Region: "eu-central"}}})
	if err != nil || strings.Contains(string(data), "password") || strings.Contains(string(data), "email") {
		t.Fatal("account YAML includes credentials")
	}
}
