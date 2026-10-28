package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Account metadata is public configuration. Credentials and generated source
// URLs are stored separately in the Vault, never in this structure.
type Go2RTCTuyaAccount struct {
	ID       string `yaml:"id" json:"id"`
	Name     string `yaml:"name" json:"name"`
	Method   string `yaml:"method" json:"method"`
	Region   string `yaml:"region" json:"region"`
	Revision string `yaml:"revision" json:"revision"`
}

type Go2RTCTuyaBinding struct {
	AccountID      string `yaml:"account_id" json:"account_id"`
	MainResolution string `yaml:"main_resolution" json:"main_resolution"`
	SubResolution  string `yaml:"sub_resolution,omitempty" json:"sub_resolution"`
}

type Go2RTCTuyaCredentials struct {
	Email        string `json:"email,omitempty"`
	Password     string `json:"password,omitempty"`
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	UID          string `json:"uid,omitempty"`
}

// The Smart hosts are the exact v1.9.14 region table; Cloud hosts are Tuya's
// documented data centers. Never accept a hostname supplied by a caller.
var go2RTCTuyaSmartHosts = map[string]string{
	"eu-central": "protect-eu.ismartlife.me", "eu-east": "protect-we.ismartlife.me",
	"us-west": "protect-us.ismartlife.me", "us-east": "protect-ue.ismartlife.me",
	"china": "protect.ismartlife.me", "india": "protect-in.ismartlife.me",
}
var go2RTCTuyaCloudHosts = map[string]string{
	"eu-central": "openapi.tuyaeu.com", "eu-west": "openapi-weaz.tuyaeu.com",
	"us-west": "openapi.tuyaus.com", "us-east": "openapi-ueaz.tuyaus.com",
	"china": "openapi.tuyacn.com", "india": "openapi.tuyain.com", "singapore": "openapi-sg.iotbing.com",
}

func Go2RTCTuyaHost(method, region string) string {
	if method == "smart" {
		return go2RTCTuyaSmartHosts[region]
	}
	if method == "cloud" {
		return go2RTCTuyaCloudHosts[region]
	}
	return ""
}

func Go2RTCTuyaAccountKey(id string) string {
	if Go2RTCStreamSourceVaultKey(id) == "" || id != strings.ToLower(strings.TrimSpace(id)) {
		return ""
	}
	return "go2rtc_tuya_account_" + id
}

func ValidateGo2RTCTuya(c Go2RTCConfig) error {
	seen := map[string]bool{}
	for _, a := range c.TuyaAccounts {
		if Go2RTCTuyaAccountKey(a.ID) == "" || seen[a.ID] || strings.TrimSpace(a.Name) == "" || len(a.Name) > 192 || strings.ContainsAny(a.Name, "\x00\r\n") || Go2RTCTuyaHost(a.Method, a.Region) == "" {
			return fmt.Errorf("invalid or duplicate Tuya account metadata")
		}
		seen[a.ID] = true
	}
	for _, s := range c.Streams {
		if b := s.Tuya; b != nil {
			if !seen[b.AccountID] {
				return fmt.Errorf("Tuya account is missing for camera %q", s.ID)
			}
			if (b.MainResolution != "hd" && b.MainResolution != "sd") || (b.SubResolution != "" && b.SubResolution != "sd" && b.SubResolution != "hd") || b.MainResolution == b.SubResolution {
				return fmt.Errorf("invalid Tuya quality selection for camera %q", s.ID)
			}
			if (s.Transport != "" && s.Transport != "inherit") || s.SnapshotMode == "camera" {
				return fmt.Errorf("Tuya uses its own transport and stream snapshots")
			}
		}
	}
	return nil
}

func ValidateGo2RTCTuyaCredentials(method string, c Go2RTCTuyaCredentials) error {
	fields := []string{c.Email, c.Password, c.ClientID, c.ClientSecret, c.UID}
	for _, v := range fields {
		if len(v) > 4096 || strings.ContainsAny(v, "\x00\r\n") {
			return fmt.Errorf("invalid Tuya credential")
		}
	}
	if method == "smart" && c.Email != "" && c.Password != "" && c.ClientID == "" && c.ClientSecret == "" && c.UID == "" {
		return nil
	}
	if method == "cloud" && c.ClientID != "" && c.ClientSecret != "" && c.UID != "" && c.Email == "" && c.Password == "" {
		return nil
	}
	return fmt.Errorf("complete credentials for the selected Tuya method are required")
}

func Go2RTCTuyaSource(a Go2RTCTuyaAccount, c Go2RTCTuyaCredentials, device, resolution string) (string, error) {
	if err := ValidateGo2RTCTuyaCredentials(a.Method, c); err != nil {
		return "", err
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`).MatchString(device) || (resolution != "hd" && resolution != "sd") || Go2RTCTuyaHost(a.Method, a.Region) == "" {
		return "", fmt.Errorf("invalid Tuya device, region or quality")
	}
	q := url.Values{"device_id": {device}, "resolution": {resolution}}
	if a.Method == "smart" {
		q.Set("email", c.Email)
		q.Set("password", c.Password)
	} else {
		q.Set("client_id", c.ClientID)
		q.Set("client_secret", c.ClientSecret)
		q.Set("uid", c.UID)
	}
	return (&url.URL{Scheme: "tuya", Host: Go2RTCTuyaHost(a.Method, a.Region), RawQuery: q.Encode()}).String(), nil
}

func ValidateGo2RTCTuyaSource(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "tuya" || u.User != nil || u.Path != "" || u.Fragment != "" || u.Port() != "" || u.Opaque != "" {
		return fmt.Errorf("invalid Tuya source")
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return fmt.Errorf("invalid Tuya query")
	}
	for key, values := range q {
		if len(values) != 1 || !strings.Contains("|device_id|resolution|email|password|client_id|client_secret|uid|", "|"+key+"|") {
			return fmt.Errorf("unsupported Tuya parameter")
		}
	}
	c := Go2RTCTuyaCredentials{Email: q.Get("email"), Password: q.Get("password"), ClientID: q.Get("client_id"), ClientSecret: q.Get("client_secret"), UID: q.Get("uid")}
	method := "cloud"
	hosts := go2RTCTuyaCloudHosts
	if _, ok := q["email"]; ok {
		method = "smart"
		hosts = go2RTCTuyaSmartHosts
	}
	for _, field := range []string{"email", "password", "client_id", "client_secret", "uid"} {
		_, present := q[field]
		wanted := (method == "smart" && (field == "email" || field == "password")) || (method == "cloud" && field != "email" && field != "password")
		if present != wanted {
			return fmt.Errorf("incomplete or mixed Tuya credentials")
		}
	}
	for region, host := range hosts {
		if u.Host == host {
			resolution := q.Get("resolution")
			if resolution == "" {
				resolution = "hd"
			}
			_, err := Go2RTCTuyaSource(Go2RTCTuyaAccount{Method: method, Region: region}, c, q.Get("device_id"), resolution)
			return err
		}
	}
	return fmt.Errorf("unsupported Tuya host")
}
