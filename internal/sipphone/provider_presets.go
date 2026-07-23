package sipphone

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	"aurago/internal/config"
)

// SIPProviderField describes one non-technical value requested by the guided
// setup. Secret fields are descriptive only; their values travel separately
// and are never retained in the catalog.
type SIPProviderField struct {
	Key         string `json:"key"`
	LabelKey    string `json:"label_key"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Secret      bool   `json:"secret,omitempty"`
	Default     string `json:"default,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
}

// SIPProviderPreset is the public, secret-free setup catalog entry.
type SIPProviderPreset struct {
	ID                string             `json:"id"`
	Name              string             `json:"name"`
	Category          string             `json:"category"`
	Region            string             `json:"region"`
	DocumentationURL  string             `json:"documentation_url"`
	Notice            string             `json:"notice,omitempty"`
	Fields            []SIPProviderField `json:"fields"`
	registrar         string
	serverField       string
	usernameField     string
	authUsernameField string
	displayNameField  string
	outboundProxy     bool
	preferSRV         bool
	registerExpires   int
}

var sipProviderPresets = buildSIPProviderPresets()

// SIPProviderPresets returns a detached, deterministically ordered catalog.
func SIPProviderPresets() []SIPProviderPreset {
	result := make([]SIPProviderPreset, len(sipProviderPresets))
	for index, preset := range sipProviderPresets {
		result[index] = preset
		result[index].Fields = append([]SIPProviderField(nil), preset.Fields...)
	}
	return result
}

// ApplySIPProviderPreset builds a conservative registration-only
// configuration. Call permissions and their allowlists intentionally remain
// disabled until the administrator configures them explicitly.
func ApplySIPProviderPreset(presetID string, values map[string]string) (config.SIPConfig, error) {
	preset, ok := sipProviderPreset(presetID)
	if !ok {
		return config.SIPConfig{}, fmt.Errorf("unknown SIP provider preset")
	}
	allowed := make(map[string]SIPProviderField, len(preset.Fields))
	for _, field := range preset.Fields {
		allowed[field.Key] = field
	}
	clean := make(map[string]string, len(values))
	for key, value := range values {
		field, exists := allowed[key]
		if !exists || field.Secret {
			return config.SIPConfig{}, fmt.Errorf("unsupported SIP provider field %q", key)
		}
		value = strings.TrimSpace(value)
		if len(value) > 512 || strings.ContainsAny(value, "\r\n\x00") {
			return config.SIPConfig{}, fmt.Errorf("invalid value for SIP provider field %q", key)
		}
		clean[key] = value
	}
	for _, field := range preset.Fields {
		if field.Secret {
			continue
		}
		if clean[field.Key] == "" {
			clean[field.Key] = strings.TrimSpace(field.Default)
		}
		if field.Required && clean[field.Key] == "" {
			return config.SIPConfig{}, fmt.Errorf("required SIP provider field %q is missing", field.Key)
		}
	}

	registrar := preset.registrar
	if preset.serverField != "" {
		registrar = clean[preset.serverField]
	}
	registrar, domain, err := parseProviderServer(registrar)
	if err != nil {
		return config.SIPConfig{}, fmt.Errorf("invalid SIP provider server: %w", err)
	}
	username := clean[preset.usernameField]
	authUsername := username
	if preset.authUsernameField != "" && clean[preset.authUsernameField] != "" {
		authUsername = clean[preset.authUsernameField]
	}
	displayName := "AuraGo"
	if preset.displayNameField != "" && clean[preset.displayNameField] != "" {
		displayName = clean[preset.displayNameField]
	} else if preset.usernameField == "phone_number" {
		displayName = username
	}

	var result config.SIPConfig
	config.ApplySIPDefaults(&result)
	result.PresetID = preset.ID
	result.Enabled = true
	result.ReadOnly = true
	result.BindHost = "0.0.0.0"
	result.Registrar = registrar
	result.Domain = domain
	result.Username = username
	result.AuthUsername = authUsername
	result.DisplayName = displayName
	result.RegisterExpiresSeconds = preset.registerExpires
	result.PreferSRV = preset.preferSRV
	if result.RegisterExpiresSeconds == 0 {
		result.RegisterExpiresSeconds = config.DefaultSIPRegisterExpires
	}
	if preset.outboundProxy {
		result.OutboundProxy = registrar
	}
	result.Inbound.Route = "reject"
	result.Inbound.TrustedPeerCIDRs = nil
	result.Inbound.AllowedCallers = nil
	result.Outbound.AllowedDomains = nil
	result.Outbound.AllowedUsers = nil
	result.Outbound.AllowedE164Prefixes = nil
	result.Permissions.AnswerInbound = false
	result.Permissions.OriginateOutbound = false
	result.Permissions.SendDTMF = false
	result.Permissions.AgentHangup = true
	result.BrowserMedia.Enabled = false
	config.NormalizeSIPConfig(&result)
	if err := config.ValidateSIPConfig(result); err != nil {
		return config.SIPConfig{}, fmt.Errorf("invalid SIP provider values: %w", err)
	}
	return result, nil
}

func parseProviderServer(raw string) (registrar string, domain string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, " \t\r\n\x00/@;?#\\") {
		return "", "", fmt.Errorf("enter a host name or IP address, optionally followed by a port")
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "sip:") || strings.HasPrefix(lower, "sips:") {
		return "", "", fmt.Errorf("enter the server without a sip: scheme")
	}

	if ip := net.ParseIP(strings.Trim(raw, "[]")); ip != nil {
		domain = ip.String()
		if strings.Contains(domain, ":") {
			return "[" + domain + "]", domain, nil
		}
		return domain, domain, nil
	}

	if host, port, splitErr := net.SplitHostPort(raw); splitErr == nil {
		host = strings.TrimSpace(strings.Trim(host, "[]"))
		portNumber, portErr := strconv.Atoi(port)
		if host == "" || portErr != nil || portNumber < 1 || portNumber > 65535 {
			return "", "", fmt.Errorf("the optional port must be between 1 and 65535")
		}
		if net.ParseIP(host) == nil && strings.Contains(host, ":") {
			return "", "", fmt.Errorf("the server IP address is invalid")
		}
		return net.JoinHostPort(host, strconv.Itoa(portNumber)), host, nil
	}
	if strings.Contains(raw, ":") {
		return "", "", fmt.Errorf("IPv6 addresses must be valid; ports require brackets around IPv6 addresses")
	}
	return raw, raw, nil
}

func sipProviderPreset(id string) (SIPProviderPreset, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	index := sort.Search(len(sipProviderPresets), func(index int) bool {
		return sipProviderPresets[index].ID >= id
	})
	if index >= len(sipProviderPresets) || sipProviderPresets[index].ID != id {
		return SIPProviderPreset{}, false
	}
	return sipProviderPresets[index], true
}

func buildSIPProviderPresets() []SIPProviderPreset {
	presets := []SIPProviderPreset{
		fritzBoxPreset(),
		fixedProvider("sipgate-de", "sipgate", "germany", "DE", "https://help.sipgate.de/hc/de/articles/4402620523665-Wie-konfiguriere-ich-mein-VoIP-Telefon-mit-sipgate-", "sipgate.de", 600, true, "device_password"),
		fixedProvider("easybell-voip", "easybell Anschluss", "germany", "DE", "https://www.easybell.de/hilfe/telefon-konfiguration/allgemein/allgemeine-anleitung-zur-voip-konfiguration/", "voip.easybell.de", 300, false, "device_password"),
		fixedProvider("easybell-cloud-pbx", "easybell Cloud Telefonanlage", "germany", "DE", "https://www.easybell.de/hilfe/telefon-konfiguration/snom-ip-telefon/snom-anleitungen/snom-ip-telefon-manuell-als-endgeraet-in-der-cloud-telefonanlage-einrichten/", "pbx.easybell.de", 300, false, "device_password"),
		telekomPreset(),
		routerProvider("vodafone-de-router", "Vodafone (Router/PBX)", "germany", "DE", "https://www.vodafone.de/hilfe/router.html"),
		routerProvider("o2-de-router", "O2 (Router/PBX)", "germany", "DE", "https://www.o2online.de/service/router/"),
		routerProvider("1und1-router", "1&1 (Router/PBX)", "germany", "DE", "https://hilfe-center.1und1.de/"),
		fixedProvider("telnyx", "Telnyx", "global", "Global", "https://sip.telnyx.com/", "sip.telnyx.com", 300, true, "e164"),
		withSRV(fixedProvider("callcentric", "Callcentric", "north_america", "US/CA", "https://www.callcentric.com/support/device/other", "sip.callcentric.net", 1800, true, "device_password")),
		serverProvider("voip-ms", "VoIP.ms", "north_america", "Global", "https://wiki.voip.ms/article/Servers", true, "account_server"),
		serverProvider("zadarma", "Zadarma", "global", "Global", "https://zadarma.com/en/services/calls/sip-trunk/", true, "account_server"),
		serverProvider("peoplefone", "peoplefone", "europe", "EU/CH", "https://support.peoplefone.com/en-che/peoplefone-sip-trunk/", true, "account_server"),
		serverProvider("dus-net", "dus.net", "germany", "DE", "https://www.dus.net/hilfe/", true, "account_server"),
		serverProvider("fonial", "fonial", "germany", "DE", "https://hilfe.fonial.de/", true, "account_server"),
		serverProvider("placetel", "Placetel", "germany", "DE", "https://help.placetel.de/", true, "account_server"),
		serverProvider("nfon", "NFON", "germany", "DE/EU", "https://www.nfon.com/de/service/documentation", true, "account_server"),
		serverProvider("deutsche-telefon", "Deutsche Telefon Standard", "germany", "DE", "https://help.deutsche-telefon.de/", true, "account_server"),
		serverProvider("pascom-cloud", "pascom Cloud", "germany", "DE/EU", "https://www.pascom.net/doc/", true, "account_server"),
		serverProvider("sipcall", "sipcall", "europe", "CH/DE/AT", "https://www.sipcall.ch/de-ch/support/", true, "account_server"),
		serverProvider("netvoip", "netvoip", "europe", "CH", "https://www.netvoip.ch/support/", true, "account_server"),
		serverProvider("fonira", "fonira", "europe", "AT", "https://fonira.at/hilfe/", true, "account_server"),
		serverProvider("voipfone", "Voipfone", "europe", "UK", "https://www.voipfone.co.uk/help", true, "account_server"),
		serverProvider("gradwell", "Gradwell", "europe", "UK", "https://help.gradwell.com/", true, "account_server"),
		serverProvider("aa-voice", "Andrews & Arnold Voice", "europe", "UK", "https://support.aa.net.uk/VoIP", true, "account_server"),
		serverProvider("ovhcloud-voip", "OVHcloud VoIP", "europe", "FR/EU", "https://help.ovhcloud.com/csm/en-voip-sip-configuration?id=kb_article_view&sysparm_article=KB0050344", true, "account_server"),
		serverProvider("keyyo", "Keyyo", "europe", "FR", "https://assistance.keyyo.com/", true, "account_server"),
		serverProvider("ippi", "ippi", "europe", "FR", "https://www.ippi.com/en/support/", true, "account_server"),
		serverProvider("cheapconnect", "CheapConnect", "europe", "NL", "https://www.cheapconnect.net/klantenservice/", true, "account_server"),
		serverProvider("voipgrid", "VoIPGRID", "europe", "NL/BE", "https://help.voipgrid.nl/", true, "account_server"),
		serverProvider("weepee", "WeePee", "europe", "BE", "https://www.weepee.io/support/", true, "account_server"),
		serverProvider("3starsnet", "3StarsNet", "europe", "BE", "https://support.3starsnet.com/", true, "account_server"),
		serverProvider("messagenet", "Messagenet", "europe", "IT", "https://help.messagenet.com/", true, "account_server"),
		serverProvider("olimontel", "OlimonTel", "europe", "IT", "https://www.olimontel.it/assistenza/", true, "account_server"),
		serverProvider("cellip", "Cellip", "europe", "Nordics", "https://support.cellip.com/", true, "account_server"),
		serverProvider("dstny", "Dstny", "europe", "EU", "https://support.dstny.com/", true, "account_server"),
		routerProvider("swisscom-router", "Swisscom (Router/PBX)", "europe", "CH", "https://www.swisscom.ch/de/privatkunden/hilfe/geraet/internet-router.html"),
		routerProvider("sunrise-router", "Sunrise (Router/PBX)", "europe", "CH", "https://www.sunrise.ch/de/support/internet/connect-box"),
		routerProvider("salt-router", "Salt (Router/PBX)", "europe", "CH", "https://www.salt.ch/de/help/home"),
		routerProvider("orange-router", "Orange (Router/PBX)", "europe", "EU", "https://assistance.orange.fr/"),
		routerProvider("proximus-router", "Proximus (Router/PBX)", "europe", "BE", "https://www.proximus.be/support/"),
		routerProvider("kpn-router", "KPN (Router/PBX)", "europe", "NL", "https://www.kpn.com/service/internet/wifi-en-modems"),
		routerProvider("bt-router", "BT (Router/PBX)", "europe", "UK", "https://www.bt.com/help/broadband"),
		routerProvider("virgin-media-router", "Virgin Media (Router/PBX)", "europe", "UK", "https://www.virginmedia.com/help"),
		routerProvider("tim-router", "TIM (Router/PBX)", "europe", "IT", "https://www.tim.it/assistenza"),
		routerProvider("movistar-router", "Movistar (Router/PBX)", "europe", "ES", "https://www.movistar.es/atencion-cliente/"),
		routerProvider("free-fr-router", "Free (Router/PBX)", "europe", "FR", "https://assistance.free.fr/"),
		routerProvider("sfr-router", "SFR (Router/PBX)", "europe", "FR", "https://assistance.sfr.fr/"),
		routerProvider("bouygues-router", "Bouygues Telecom (Router/PBX)", "europe", "FR", "https://www.bouyguestelecom.fr/assistance"),
		serverProvider("onsip", "OnSIP", "north_america", "US", "https://support.onsip.com/", true, "account_server"),
		serverProvider("anveo", "Anveo", "north_america", "Global", "https://www.anveo.com/faq.asp", true, "account_server"),
		serverProvider("localphone", "Localphone", "global", "Global", "https://www.localphone.com/help", true, "account_server"),
		serverProvider("linphone", "Linphone SIP Service", "global", "Global", "https://www.linphone.org/en/linphone-free-sip-service/", true, "account_server"),
		serverProvider("sip2sip", "SIP2SIP", "global", "Global", "https://sip2sip.info/", true, "account_server"),
		serverProvider("iptel", "iptel.org", "global", "Global", "https://www.iptel.org/service", true, "account_server"),
		serverProvider("crazytel", "Crazytel", "global", "AU/NZ", "https://help.crazytel.com.au/", true, "account_server"),
		serverProvider("maxotel", "MaxoTel", "global", "AU", "https://knowledgebase.maxo.com.au/", true, "account_server"),
		localPBXPreset("generic-pbx", "Andere Telefonanlage oder SIP-Anbieter", "https://www.rfc-editor.org/rfc/rfc3261"),
		localPBXPreset("speedport-pbx", "Speedport / lokaler SIP-Registrar", "https://www.telekom.de/hilfe/geraete/router/speedport"),
		localPBXPreset("asterisk-freepbx", "Asterisk / FreePBX", "https://docs.asterisk.org/"),
		localPBXPreset("freeswitch-fusionpbx", "FreeSWITCH / FusionPBX", "https://developer.signalwire.com/freeswitch/"),
		localPBXPreset("3cx", "3CX", "https://www.3cx.com/docs/"),
		localPBXPreset("starface", "STARFACE", "https://knowledge.starface.de/"),
		localPBXPreset("auerswald", "Auerswald", "https://www.auerswald.de/de/service"),
		localPBXPreset("innovaphone", "innovaphone", "https://wiki.innovaphone.com/"),
		localPBXPreset("unify", "Unify OpenScape", "https://www.unify.com/"),
		localPBXPreset("yeastar", "Yeastar", "https://help.yeastar.com/"),
		localPBXPreset("grandstream-ucm", "Grandstream UCM", "https://documentation.grandstream.com/"),
		localPBXPreset("cisco-cucm", "Cisco Unified Communications Manager", "https://www.cisco.com/c/en/us/support/unified-communications/unified-communications-manager-callmanager/series.html"),
		localPBXPreset("mitel", "Mitel", "https://www.mitel.com/document-center"),
		localPBXPreset("wildix", "Wildix", "https://confluence.wildix.com/"),
		localPBXPreset("avaya", "Avaya", "https://support.avaya.com/"),
	}
	sort.Slice(presets, func(i, j int) bool { return presets[i].ID < presets[j].ID })
	return presets
}

func commonFields(includeServer, includeAuth bool) []SIPProviderField {
	fields := make([]SIPProviderField, 0, 5)
	if includeServer {
		fields = append(fields, SIPProviderField{Key: "server", LabelKey: "config.sip.wizard.server", Type: "text", Required: true, Placeholder: "sip.example.com"})
	}
	fields = append(fields, SIPProviderField{Key: "username", LabelKey: "config.sip.username", Type: "text", Required: true})
	if includeAuth {
		fields = append(fields, SIPProviderField{Key: "auth_username", LabelKey: "config.sip.auth_username", Type: "text"})
	}
	fields = append(fields,
		SIPProviderField{Key: "password", LabelKey: "config.sip.password", Type: "password", Required: true, Secret: true},
		SIPProviderField{Key: "display_name", LabelKey: "config.sip.display_name", Type: "text", Default: "AuraGo"},
	)
	return fields
}

func fixedProvider(id, name, category, region, docs, registrar string, expires int, proxy bool, notice string) SIPProviderPreset {
	return SIPProviderPreset{
		ID: id, Name: name, Category: category, Region: region, DocumentationURL: docs, Notice: notice,
		Fields: commonFields(false, false), registrar: registrar, usernameField: "username",
		displayNameField: "display_name", outboundProxy: proxy, registerExpires: expires,
	}
}

func withSRV(preset SIPProviderPreset) SIPProviderPreset {
	preset.preferSRV = true
	return preset
}

func serverProvider(id, name, category, region, docs string, proxy bool, notice string) SIPProviderPreset {
	return SIPProviderPreset{
		ID: id, Name: name, Category: category, Region: region, DocumentationURL: docs, Notice: notice,
		Fields: commonFields(true, false), serverField: "server", usernameField: "username",
		displayNameField: "display_name", outboundProxy: proxy, registerExpires: 300,
	}
}

func localPBXPreset(id, name, docs string) SIPProviderPreset {
	preset := serverProvider(id, name, "pbx", "LAN/Cloud", docs, false, "pbx_credentials")
	preset.Fields = commonFields(true, true)
	preset.authUsernameField = "auth_username"
	return preset
}

func routerProvider(id, name, category, region, docs string) SIPProviderPreset {
	preset := localPBXPreset(id, name, docs)
	preset.Category = category
	preset.Region = region
	preset.Notice = "router_recommended"
	return preset
}

func fritzBoxPreset() SIPProviderPreset {
	fields := commonFields(true, false)
	fields[0].Default = "fritz.box"
	fields[0].Placeholder = "fritz.box"
	return SIPProviderPreset{
		ID: "fritzbox", Name: "FRITZ!Box", Category: "local", Region: "LAN",
		DocumentationURL: "https://avm.de/service/wissensdatenbank/dok/FRITZ-Box-7590/42_IP-Telefon-an-FRITZ-Box-anmelden-und-einrichten/",
		Notice:           "fritzbox_phone", Fields: fields, serverField: "server", usernameField: "username",
		displayNameField: "display_name", registerExpires: 300,
	}
}

func telekomPreset() SIPProviderPreset {
	return SIPProviderPreset{
		ID: "telekom-de", Name: "Deutsche Telekom", Category: "germany", Region: "DE",
		DocumentationURL: "https://www.telekom.de/hilfe/werksreset-speedport/werksreset-faq/einrichtung-sip-client",
		Notice:           "router_recommended",
		Fields: []SIPProviderField{
			{Key: "phone_number", LabelKey: "config.sip.wizard.phone_number", Type: "text", Required: true, Placeholder: "+491234567890"},
			{Key: "auth_username", LabelKey: "config.sip.auth_username", Type: "text", Required: true, Placeholder: "name@t-online.de"},
			{Key: "password", LabelKey: "config.sip.password", Type: "password", Required: true, Secret: true},
		},
		registrar: "tel.t-online.de", usernameField: "phone_number", authUsernameField: "auth_username",
		outboundProxy: true, preferSRV: true, registerExpires: 300,
	}
}
