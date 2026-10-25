package agent

// toolFamilyTermSet lists, for one tool family, the words and phrases that
// name it in a user message, in English and German. Terms are whole words
// after the search folding and stemming (see toolIntentText): one stem covers
// the singular, the plural and the usual verb forms, so "file" also matches
// "files" and "scannen" also matches "scanne". Folded spelling is fine ("ae",
// "oe", "ue", "ss"); the matcher folds the message the same way.
//
// heads are German nouns that also match at either end of a compound word
// (Konfigurationsdatei, Dateiname, Heimnetzwerk). They have at least five
// runes and are no English words, so short heads ("pfad", "bild") never match
// inside unrelated words.
type toolFamilyTermSet struct {
	family string
	terms  []string
	heads  []string
}

// toolFamilyTermSets are in the order that breaks a tie: with the same number
// of matched terms the earlier family wins.
var toolFamilyTermSets = []toolFamilyTermSet{
	{
		family: "deployment",
		terms: []string{
			"homepage", "landing page", "landingpage", "website", "web site", "webseite",
			"netlify", "vercel", "here now", "deploy", "deployment", "publish",
			"veroeffentlichen", "veroeffentlicht", "veroeffentlichung", "publizieren", "hochladen", "hochgeladen",
		},
	},
	{
		family: "files",
		terms: []string{
			"file", "filesystem", "file system", "folder", "directory", "yaml", "yml", "json", "xml", "toml", "csv", "pdf",
			"pfad",
		},
		heads: []string{"datei", "ordner", "verzeichnis"},
	},
	{
		family: "shell",
		terms: []string{
			"shell", "command", "commandline", "command line", "terminal", "script", "bash", "powershell", "sudo", "cli",
		},
		heads: []string{"befehl", "kommando", "konsole", "skript"},
	},
	{
		family: "coding",
		terms: []string{
			"python", "code", "coding", "program", "programmer", "sandbox", "debug", "refactor", "compile",
			"programm", "quellcode",
		},
		heads: []string{"programmier"},
	},
	{
		family: "memory",
		terms: []string{
			"memory", "remember", "note", "journal", "knowledge", "knowledge graph",
			"merken", "gemerkt", "tagebuch",
		},
		heads: []string{"gedaechtnis", "notiz"},
	},
	{
		family: "web",
		terms: []string{
			"web", "web search", "search", "scrape", "scraper", "crawl", "crawler", "internet", "browser", "browse",
			"url", "http", "https", "api", "endpoint", "form",
			"suche", "suchen", "recherche", "recherchieren", "websuche", "internetsuche", "formular",
		},
	},
	{
		family: "network",
		terms: []string{
			"network", "dns", "ping", "port", "scan", "scanner", "fritz", "fritzbox", "fritz box", "wake on lan", "wol",
			"lan", "wlan", "wifi", "ip", "router", "subnet", "traceroute", "whois", "mdns", "upnp", "firewall",
			"scannen", "portscan", "portscanner",
		},
		heads: []string{"netzwerk"},
	},
	{
		family: "infra",
		terms: []string{
			"docker", "dockerfile", "compose", "container", "proxmox", "frigate", "nvr", "tailscale", "ansible", "github",
			"remote", "mqtt", "sql", "jellyfin", "truenas", "server", "ssh", "vm", "virtual machine", "nas",
			"kubernetes", "meshcentral", "ollama", "adguard", "grafana", "home assistant", "homeassistant", "koofr", "onedrive",
			"fernzugriff", "fernwartung", "virtuelle maschine",
		},
	},
	{
		family: "communication",
		terms: []string{
			"email", "mail", "webhook", "sms", "call", "contact", "telegram", "discord", "telnyx", "phone", "inbox", "address book",
			"adressbuch", "posteingang",
		},
		heads: []string{"anruf", "kontakt", "telefon"},
	},
	{
		family: "automation",
		terms: []string{
			"schedule", "scheduler", "cron", "cronjob", "mission", "automation", "automate", "follow up", "followup",
			"reminder", "recurring", "daily", "weekly", "timer", "alarm",
			"planen", "erinnerung", "taeglich", "woechentlich", "regelmaessig", "wecker",
		},
		heads: []string{"zeitplan", "automatisier"},
	},
	{
		family: "media",
		terms: []string{
			"image", "audio", "speech", "media", "music", "video", "photo", "picture", "tts", "voice", "podcast", "song",
			"movie", "film", "animation", "sound", "drawing", "illustration", "transcribe",
			"bild", "bilder", "foto", "lied", "sprachausgabe", "vorlesen", "stimme", "zeichnung", "grafik",
		},
		heads: []string{"musik"},
	},
}

// toolFamilyTerms are the compiled tables, aligned with toolFamilyTermSets.
var toolFamilyTerms = func() []*toolIntentTerms {
	compiled := make([]*toolIntentTerms, len(toolFamilyTermSets))
	for i, set := range toolFamilyTermSets {
		compiled[i] = newToolIntentTerms(set.terms, set.heads)
	}
	return compiled
}()
