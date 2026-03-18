# Fritz!Box Integration - Implementierungsplan

**Gerät:** AVM Fritz!Box (DSL, Cable, Fiber)  
**Schnittstellen:** TR-064 (SOAP), AHA-HTTP, Lua-Login  
**Ziel:** Vollständige Geräteverwaltung mit optionalen Feature-Gruppen

---

## 1. Übersicht & Architektur

```
┌─────────────────────────────────────────────────────────────────┐
│                     Fritz!Box Integration                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐ │
│  │   System    │  │   Network   │  │      Telephony          │ │
│  │  (ReadOnly) │  │ (Read/Write)│  │    (Read/Write)         │ │
│  ├─────────────┤  ├─────────────┤  ├─────────────────────────┤ │
│  │ • Info      │  │ • WLAN      │  │ • Anruflisten           │ │
│  │ • Uptime    │  │ • Gast-WLAN │  │ • Anrufbeantworter      │ │
│  │ • Reboot    │  │ • DECT      │  │ • Telefonbücher         │ │
│  │ • Log       │  │ • Mesh      │  │ • Rufumleitungen        │ │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘ │
│                                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐ │
│  │  Smart Home │  │   Storage   │  │    TV/Media (Cable)     │ │
│  │(Read/Write) │  │ (Read/Write)│  │      (ReadOnly)         │ │
│  ├─────────────┤  ├─────────────┤  ├─────────────────────────┤ │
│  │ • Geräte    │  │ • NAS       │  │ • DVB-C Sender          │ │
│  │ • Templates │  │ • FTP       │  │ • Stream-URLs           │ │
│  │ • Schalter  │  │ • Freigaben │  │ • EPG (optional)        │ │
│  │ • Heizung   │  │ • Links     │  │                         │ │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘ │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 2. Konfiguration

```yaml
fritzbox:
  enabled: false
  
  # Verbindung
  connection:
    host: "fritz.box"           # oder IP: 192.168.178.1
    port: 49000                 # TR-064 Port (default: 49000)
    https: true                 # TR-064 über HTTPS
    timeout: 10                 # Sekunden
    
    # Anmeldung (Vault empfohlen!)
    username: ""                # FRITZ!Box Benutzername
    password: ""                # Wird aus Vault gelesen falls leer
    
  # Feature-Gruppen (alle optional deaktivierbar)
  features:
    # ─────────────────────────────────────────────────────────────
    # GRUPPE 1: System-Informationen (immer lesbar)
    # ─────────────────────────────────────────────────────────────
    system:
      enabled: true
      readonly: true            # Keine Schreiboperationen möglich
      
      sub_features:
        device_info: true       # Modell, Version, Seriennummer
        uptime: true            # Betriebszeit
        log: true               # System-Log
        temperatures: true      # Hardware-Temperaturen (falls verfügbar)
    
    # ─────────────────────────────────────────────────────────────
    # GRUPPE 2: Netzwerk (Schreibzugriffe möglich)
    # ─────────────────────────────────────────────────────────────
    network:
      enabled: true
      readonly: false           # ⚠️ Kann auf false gesetzt werden
      
      sub_features:
        wlan: true              # 2.4GHz & 5GHz Einstellungen
        guest_wlan: true        # Gast-WLAN konfigurieren
        dect: true              # DECT-Basisstation
        mesh: true              # FRITZ!Mesh
        hosts: true             # Netzwerkgeräte
        wake_on_lan: true       # Geräte aufwecken
        port_forwarding: true   # Portfreigaben
        internet_access: true   # Internet-Zugriff pro Gerät
    
    # ─────────────────────────────────────────────────────────────
    # GRUPPE 3: Telefonie (Schreibzugriffe möglich)
    # ─────────────────────────────────────────────────────────────
    telephony:
      enabled: true
      readonly: false           # ⚠️ Kann auf false gesetzt werden
      
      sub_features:
        call_lists: true        # Anruflisten (ein-/ausgehend/verpasst)
        call_monitor: false     # Live-Anruf-Monitoring (TCP 1012)
        tam: true               # Anrufbeantworter (TAM)
        phonebooks: true        # Telefonbücher
        call_deflection: true   # Rufumleitungen
        call_blocking: true     # Anrufsperren
        click_to_dial: false    # Anrufe initiieren
    
    # ─────────────────────────────────────────────────────────────
    # GRUPPE 4: Smart Home (Schreibzugriffe möglich)
    # ─────────────────────────────────────────────────────────────
    smart_home:
      enabled: true
      readonly: false           # ⚠️ Kann auf false gesetzt werden
      
      sub_features:
        devices: true           # Geräte-Status
        switches: true          # Steckdosen schalten
        heating: true           # Heizungssteuerung
        blinds: true            # Rollladensteuerung
        lamps: true             # Lichtsteuerung
        templates: true         # Vorlagen anwenden
        routines: true          # Routinen (neu in FRITZ!OS 8+)
    
    # ─────────────────────────────────────────────────────────────
    # GRUPPE 5: Storage/NAS (Schreibzugriffe möglich)
    # ─────────────────────────────────────────────────────────────
    storage:
      enabled: true
      readonly: false           # ⚠️ Kann auf false gesetzt werden
      
      sub_features:
        nas: true               # NAS-Freigaben (SMB/FTP)
        ftp: true               # FTP-Server-Einstellungen
        file_shares: true       # Datei-Freigaben erstellen/löschen
        usb_devices: true       # USB-Geräte-Info
        media_server: true      # UPnP AV Mediaserver
    
    # ─────────────────────────────────────────────────────────────
    # GRUPPE 6: TV/Media (nur Cable-Modelle, ReadOnly)
    # ─────────────────────────────────────────────────────────────
    tv:
      enabled: true
      readonly: true            # Keine Schreiboperationen möglich
      cable_only: true          # Hinweis: Nur für DVB-C Modelle
      
      sub_features:
        channel_list: true      # Senderliste abrufen
        stream_urls: true       # Streaming-URLs generieren
        epg: false              # EPG-Daten (experimentell)

  # Automatisierung
  polling:
    enabled: true
    interval_seconds: 60        # Wie oft aktualisiert werden soll
    
    # Pro-Gruppe Polling
    groups:
      system: 300               # System selten ändern
      network: 60
      telephony: 30             # Anrufe oft prüfen
      smart_home: 30
      storage: 300
      tv: 3600                  # Senderliste selten
```

---

## 3. TR-064 Services & Aktionen

### 3.1 System (DeviceInfo, DeviceConfig)

```go
// internal/fritzbox/system.go
package fritzbox

// DeviceInfoService
type DeviceInfoService struct {
    client *TR064Client
}

func (s *DeviceInfoService) GetInfo() (*DeviceInfo, error) {
    // Action: GetInfo
    // Service: urn:dslforum-org:service:DeviceInfo:1
    // Liefert: NewManufacturer, NewModelName, NewDescription, 
    //          NewSerialNumber, NewSoftwareVersion, etc.
}

func (s *DeviceInfoService) GetLog() ([]LogEntry, error) {
    // X_AVM-DE_GetDeviceLog (vendor-specific)
}

// DeviceConfigService
type DeviceConfigService struct {
    client *TR064Client
    readonly bool  // Respektiere readonly Einstellung!
}

func (s *DeviceConfigService) Reboot() error {
    if s.readonly {
        return fmt.Errorf("system group is readonly")
    }
    // Action: Reboot
}

func (s *DeviceConfigService) FactoryReset() error {
    if s.readonly {
        return fmt.Errorf("system group is readonly")
    }
    // Achtung: Dies löscht alle Einstellungen!
    // Action: FactoryReset
}
```

### 3.2 Telefonie (X_AVM-DE_OnTel, X_AVM-DE_TAM)

```go
// internal/fritzbox/telephony.go
package fritzbox

// OnTelService - Telefonbuch & Anruflisten
type OnTelService struct {
    client *TR064Client
    readonly bool
}

// GetCallList ruft die Anrufliste ab
func (s *OnTelService) GetCallList() (*CallList, error) {
    // Action: X_AVM-DE_GetCallList
    // Liefert URL zu XML-Datei mit Anrufen
    // Format: date, type (1=in, 2=missed, 3=out, 10=rejected),
    //         localNumber, remoteNumber, duration
}

// Phonebook verwaltung
func (s *OnTelService) GetPhonebook(id int) (*Phonebook, error) {
    // Action: X_AVM-DE_GetPhonebook
}

func (s *OnTelService) SetPhonebookEntry(id int, entry PhonebookEntry) error {
    if s.readonly {
        return fmt.Errorf("telephony group is readonly")
    }
    // Action: X_AVM-DE_SetPhonebookEntry
}

// Rufumleitungen
func (s *OnTelService) GetCallDeflections() ([]CallDeflection, error) {
    // Action: X_AVM-DE_GetCallDeflections
}

func (s *OnTelService) SetCallDeflection(id int, enabled bool) error {
    if s.readonly {
        return fmt.Errorf("telephony group is readonly")
    }
    // Action: X_AVM-DE_SetCallDeflectionEnable
}

// TAMService - Anrufbeantworter
type TAMService struct {
    client *TR064Client
    readonly bool
}

// GetInfo ruft TAM-Status ab
func (s *TAMService) GetInfo(index int) (*TAMInfo, error) {
    // Action: X_AVM-DE_GetInfo
    // Liefert: NewEnable, NewName, NewTamListURL (Aufzeichnungen)
}

// GetMessageList ruft Liste der Nachrichten ab
func (s *TAMService) GetMessageList(index int) ([]TAMMessage, error) {
    // Action: X_AVM-DE_GetMessageList
    // Liefert URL zu XML mit Nachrichten
    // Format: Date, Name, Number, Duration, Path (URL zum Audio)
}

// SetEnable aktiviert/deaktiviert TAM
func (s *TAMService) SetEnable(index int, enabled bool) error {
    if s.readonly {
        return fmt.Errorf("telephony group is readonly")
    }
    // Action: X_AVM-DE_SetEnable
}

// DeleteMessage löscht eine Nachricht
func (s *TAMService) DeleteMessage(index int, msgPath string) error {
    if s.readonly {
        return fmt.Errorf("telephony group is readonly")
    }
    // Action: X_AVM-DE_DeleteMessage
}

// TAMMessage repräsentiert eine Anrufbeantworter-Nachricht
type TAMMessage struct {
    Index    int       `xml:"Index"`
    Name     string    `xml:"Name"`
    Number   string    `xml:"Number"`
    Date     time.Time `xml:"Date"`
    Duration int       `xml:"Duration"` // Sekunden
    Path     string    `xml:"Path"`     // URL zum Audio
    New      bool      `xml:"New"`      // Ungehört
}

// DownloadAudio lädt die Audio-Datei herunter
func (m *TAMMessage) DownloadAudio(client *http.Client) ([]byte, error) {
    resp, err := client.Get(m.Path)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    return io.ReadAll(resp.Body)
}
```

### 3.3 TV/Media (X_AVM-DE_Media) - Cable only

```go
// internal/fritzbox/tv.go
package fritzbox

// MediaService - DVB-C TV Sender (nur Cable-Modelle)
type MediaService struct {
    client *TR064Client
}

// GetChannelList ruft die Senderliste ab
func (s *MediaService) GetChannelList() (*ChannelList, error) {
    // Action: X_AVM-DE_GetChannelList
    // Service: urn:dslforum-org:service:X_AVM-DE_Media:1
    // 
    // Liefert XML mit:
    // - Channel ID
    // - Channel Name
    // - Channel URL (RTSP-Stream)
    // - Channel Type (TV/Radio)
    // - Quality (SD/HD)
}

// Channel repräsentiert einen TV-Sender
type Channel struct {
    ID       string `xml:"id"`
    Name     string `xml:"name"`
    Type     string `xml:"type"`     // " television" oder "radio"
    Quality  string `xml:"quality"`  // "SD" oder "HD"
    StreamURL string `xml:"url"`     // rtsp://... oder http://...
    LogoURL  string `xml:"logo"`     // Optional
}

// GetStreamURL generiert Streaming-URL für einen Sender
func (s *MediaService) GetStreamURL(channelID string) (string, error) {
    // Format: rtsp://fritz.box:554/?avm=1&freq=xxx&... 
    // oder HTTP-Stream falls verfügbar
}

// ExportM3U exportiert Senderliste als M3U-Playlist
func (s *MediaService) ExportM3U() (string, error) {
    channels, err := s.GetChannelList()
    if err != nil {
        return "", err
    }
    
    var buf strings.Builder
    buf.WriteString("#EXTM3U\n")
    
    for _, ch := range channels.Channels {
        buf.WriteString(fmt.Sprintf("#EXTINF:-1,%s\n%s\n", 
            ch.Name, ch.StreamURL))
    }
    
    return buf.String(), nil
}
```

### 3.4 Smart Home (X_AVM-DE_Homeauto)

```go
// internal/fritzbox/smarthome.go
package fritzbox

// HomeAutoService
type HomeAutoService struct {
    client *TR064Client
    readonly bool
}

// GetInfo ruft alle Smart Home Geräte ab
func (s *HomeAutoService) GetInfo() ([]SmartDevice, error) {
    // Action: X_AVM-DE_GetInfo
    // Liefert: Liste aller angelernten Geräte
}

// SmartDevice repräsentiert ein Smart Home Gerät
type SmartDevice struct {
    AIN       string  // Eindeutige ID
    Name      string
    Type      DeviceType  // Switch, Thermostat, Lamp, Blind, etc.
    Present   bool        // Online-Status
    
    // Für Schalter
    SwitchState *bool    // nil wenn nicht unterstützt
    
    // Für Thermostate
    Temperature      *float64 // Aktuelle Temperatur
    TargetTemp       *float64 // Soll-Temperatur
    ComfortTemp      *float64
    LoweringTemp     *float64
    BatteryLow       *bool
    
    // Für Lampen (neuere FRITZ!OS)
    Level            *int     // 0-100
    ColorTemperature *int     // Kelvin
    Hue              *int     
    Saturation       *int
}

// SetSwitch schaltet eine Steckdose
func (s *HomeAutoService) SetSwitch(ain string, on bool) error {
    if s.readonly {
        return fmt.Errorf("smart_home group is readonly")
    }
    // Action: X_AVM-DE_SetSwitch
}

// SetThermostatTemp setzt Soll-Temperatur
func (s *HomeAutoService) SetThermostatTemp(ain string, temp float64) error {
    if s.readonly {
        return fmt.Errorf("smart_home group is readonly")
    }
    // Action: X_AVM-DE_SetTemperature
}

// ApplyTemplate wendet eine Vorlage an
func (s *HomeAutoService) ApplyTemplate(templateID int) error {
    if s.readonly {
        return fmt.Errorf("smart_home group is readonly")
    }
    // Action: X_AVM-DE_ApplyTemplate
}
```

---

## 4. Authentifizierung

### 4.1 Session-ID Login (PBKDF2)

```go
// internal/fritzbox/auth.go
package fritzbox

import (
    "crypto/md5"
    "crypto/sha256"
    "encoding/hex"
    "encoding/xml"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "strings"
)

// Authenticator handhabt den Login
type Authenticator struct {
    host     string
    username string
    password string
    useTLS   bool
    
    sid      string
    client   *http.Client
}

// Login durchführen
func (a *Authenticator) Login() error {
    // 1. Challenge abrufen
    challenge, blockTime, isPBKDF2, err := a.getChallenge()
    if err != nil {
        return err
    }
    
    // 2. BlockTime abwarten
    if blockTime > 0 {
        time.Sleep(time.Duration(blockTime) * time.Second)
    }
    
    // 3. Response berechnen
    var response string
    if isPBKDF2 {
        response = a.calculatePBKDF2Response(challenge)
    } else {
        response = a.calculateMD5Response(challenge)
    }
    
    // 4. Login senden
    sid, err := a.sendLoginResponse(response)
    if err != nil {
        return err
    }
    
    if sid == "0000000000000000" {
        return fmt.Errorf("authentication failed: invalid credentials")
    }
    
    a.sid = sid
    return nil
}

// getChallenge ruft Login-Challenge ab
func (a *Authenticator) getChallenge() (challenge string, blockTime int, isPBKDF2 bool, err error) {
    url := fmt.Sprintf("%s/login_sid.lua?version=2", a.baseURL())
    
    resp, err := a.client.Get(url)
    if err != nil {
        return "", 0, false, err
    }
    defer resp.Body.Close()
    
    body, _ := io.ReadAll(resp.Body)
    
    // XML parsen
    var result struct {
        Challenge  string `xml:"Challenge"`
        BlockTime  int    `xml:"BlockTime"`
    }
    xml.Unmarshal(body, &result)
    
    isPBKDF2 = strings.HasPrefix(result.Challenge, "2$")
    return result.Challenge, result.BlockTime, isPBKDF2, nil
}

// calculatePBKDF2Response berechnet PBKDF2-Antwort (modern)
func (a *Authenticator) calculatePBKDF2Response(challenge string) string {
    // Format: 2$<iter1>$<salt1>$<iter2>$<salt2>
    parts := strings.Split(challenge, "$")
    if len(parts) != 5 {
        return ""
    }
    
    iter1, _ := strconv.Atoi(parts[1])
    salt1, _ := hex.DecodeString(parts[2])
    iter2, _ := strconv.Atoi(parts[3])
    salt2, _ := hex.DecodeString(parts[4])
    
    // Hash 1: PBKDF2 mit statischem Salt
    hash1 := pbkdf2.Key([]byte(a.password), salt1, iter1, 32, sha256.New)
    
    // Hash 2: PBKDF2 mit dynamischem Salt
    hash2 := pbkdf2.Key(hash1, salt2, iter2, 32, sha256.New)
    
    return fmt.Sprintf("%s$%s", parts[4], hex.EncodeToString(hash2))
}

// calculateMD5Response berechnet MD5-Antwort (legacy fallback)
func (a *Authenticator) calculateMD5Response(challenge string) string {
    // Format: challenge-password als UTF-16LE
    response := challenge + "-" + a.password
    utf16Data := utf16.Encode([]rune(response))
    
    var buf bytes.Buffer
    for _, v := range utf16Data {
        binary.Write(&buf, binary.LittleEndian, v)
    }
    
    hash := md5.Sum(buf.Bytes())
    return challenge + "-" + hex.EncodeToString(hash[:])
}

// Logout beendet die Session
func (a *Authenticator) Logout() error {
    url := fmt.Sprintf("%s/login_sid.lua?sid=%s&logout=1", a.baseURL(), a.sid)
    _, err := a.client.Get(url)
    a.sid = ""
    return err
}
```

---

## 5. TR-064 Client

```go
// internal/fritzbox/tr064.go
package fritzbox

// TR064Client SOAP-Client für Fritz!Box
type TR064Client struct {
    host     string
    port     int
    username string
    password string
    useTLS   bool
    
    auth       *Authenticator
    httpClient *http.Client
    
    // Services
    DeviceInfo    *DeviceInfoService
    DeviceConfig  *DeviceConfigService
    WLANConfig    *WLANConfigService
    OnTel         *OnTelService
    TAM           *TAMService
    Media         *MediaService
    HomeAuto      *HomeAutoService
    Hosts         *HostsService
    Storage       *StorageService
}

// NewTR064Client erstellt neuen Client
func NewTR064Client(cfg FritzboxConfig) (*TR064Client, error) {
    client := &TR064Client{
        host:       cfg.Connection.Host,
        port:       cfg.Connection.Port,
        username:   cfg.Connection.Username,
        password:   cfg.Connection.Password,
        useTLS:     cfg.Connection.HTTPS,
        httpClient: &http.Client{Timeout: time.Duration(cfg.Connection.Timeout) * time.Second},
    }
    
    // Services initialisieren mit readonly-Flag
    client.DeviceInfo = &DeviceInfoService{client: client}
    client.DeviceConfig = &DeviceConfigService{
        client:   client,
        readonly: cfg.Features.System.Readonly,
    }
    client.OnTel = &OnTelService{
        client:   client,
        readonly: cfg.Features.Telephony.Readonly,
    }
    client.TAM = &TAMService{
        client:   client,
        readonly: cfg.Features.Telephony.Readonly,
    }
    client.HomeAuto = &HomeAutoService{
        client:   client,
        readonly: cfg.Features.SmartHome.Readonly,
    }
    // ... weitere Services
    
    // Authentifizieren
    if err := client.authenticate(); err != nil {
        return nil, err
    }
    
    return client, nil
}

// CallAction führt SOAP-Action aus
func (c *TR064Client) CallAction(service, action string, args map[string]string) (map[string]string, error) {
    // SOAP Request bauen
    envelope := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" 
            s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:%s xmlns:u="urn:dslforum-org:service:%s">
      %s
    </u:%s>
  </s:Body>
</s:Envelope>`, action, service, buildArgs(args), action)
    
    url := fmt.Sprintf("%s:%d/upnp/control/%s", c.baseURL(), c.port, service)
    
    req, err := http.NewRequest("POST", url, strings.NewReader(envelope))
    if err != nil {
        return nil, err
    }
    
    req.Header.Set("Content-Type", "text/xml; charset=utf-8")
    req.Header.Set("SoapAction", fmt.Sprintf("urn:dslforum-org:service:%s#%s", service, action))
    
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    // Response parsen
    return parseSOAPResponse(resp.Body)
}
```

---

## 6. Tools & Integration

### 6.1 Tool-Definitionen

```go
// internal/tools/fritzbox.go
package tools

// FritzboxManager Tool-Registry
type FritzboxManager struct {
    client *fritzbox.TR064Client
    config fritzbox.FritzboxConfig
}

// Tool: fritzbox_get_call_list
func (m *FritzboxManager) GetCallList(args GetCallListArgs) (string, error) {
    if !m.config.Features.Telephony.Enabled {
        return "", fmt.Errorf("telephony feature disabled")
    }
    if !m.config.Features.Telephony.SubFeatures.CallLists {
        return "", fmt.Errorf("call_lists sub-feature disabled")
    }
    
    list, err := m.client.OnTel.GetCallList()
    if err != nil {
        return "", err
    }
    
    // Als JSON zurückgeben
    data, _ := json.MarshalIndent(list, "", "  ")
    return string(data), nil
}

// Tool: fritzbox_get_tam_messages
func (m *FritzboxManager) GetTAMMessages(args GetTAMArgs) (string, error) {
    if !m.config.Features.Telephony.Enabled {
        return "", fmt.Errorf("telephony feature disabled")
    }
    if !m.config.Features.Telephony.SubFeatures.TAM {
        return "", fmt.Errorf("tam sub-feature disabled")
    }
    
    messages, err := m.client.TAM.GetMessageList(args.Index)
    if err != nil {
        return "", err
    }
    
    data, _ := json.MarshalIndent(messages, "", "  ")
    return string(data), nil
}

// Tool: fritzbox_download_tam_message
func (m *FritzboxManager) DownloadTAMMessage(args DownloadTAMArgs) (string, error) {
    if !m.config.Features.Telephony.Enabled || !m.config.Features.Telephony.SubFeatures.TAM {
        return "", fmt.Errorf("tam feature disabled")
    }
    
    // Audio-Datei herunterladen
    audio, err := m.downloadTAMAudio(args.MessagePath)
    if err != nil {
        return "", err
    }
    
    // Speichern
    path := filepath.Join(args.SavePath, fmt.Sprintf("tam_msg_%d.wav", args.MessageIndex))
    if err := os.WriteFile(path, audio, 0644); err != nil {
        return "", err
    }
    
    return fmt.Sprintf("Nachricht gespeichert: %s (%d bytes)", path, len(audio)), nil
}

// Tool: fritzbox_get_tv_channels (Cable only)
func (m *FritzboxManager) GetTVChannels(args GetTVChannelsArgs) (string, error) {
    if !m.config.Features.TV.Enabled {
        return "", fmt.Errorf("tv feature disabled")
    }
    if !m.config.Features.TV.SubFeatures.ChannelList {
        return "", fmt.Errorf("channel_list sub-feature disabled")
    }
    
    channels, err := m.client.Media.GetChannelList()
    if err != nil {
        return "", err
    }
    
    // Als M3U oder JSON
    if args.Format == "m3u" {
        return channels.ExportM3U()
    }
    
    data, _ := json.MarshalIndent(channels, "", "  ")
    return string(data), nil
}

// Tool: fritzbox_reboot (respektiert readonly!)
func (m *FritzboxManager) Reboot(args RebootArgs) (string, error) {
    if !m.config.Features.System.Enabled {
        return "", fmt.Errorf("system feature disabled")
    }
    if m.config.Features.System.Readonly {
        return "", fmt.Errorf("system group is readonly - reboot not allowed")
    }
    
    if err := m.client.DeviceConfig.Reboot(); err != nil {
        return "", err
    }
    
    return "Fritz!Box wird neu gestartet...", nil
}

// Tool: fritzbox_set_wlan (respektiert readonly!)
func (m *FritzboxManager) SetWLAN(args SetWLANArgs) (string, error) {
    if !m.config.Features.Network.Enabled {
        return "", fmt.Errorf("network feature disabled")
    }
    if m.config.Features.Network.Readonly {
        return "", fmt.Errorf("network group is readonly")
    }
    if !m.config.Features.Network.SubFeatures.WLAN {
        return "", fmt.Errorf("wlan sub-feature disabled")
    }
    
    // ...
}
```

---

## 7. UI-Integration

```javascript
// ui/cfg/fritzbox.js
{
  "section": "fritzbox",
  "label": {
    "de": "Fritz!Box",
    "en": "Fritz!Box"
  },
  "fields": [
    {
      "key": "fritzbox.enabled",
      "type": "toggle",
      "label": "Integration aktivieren"
    },
    {
      "key": "fritzbox.connection.host",
      "type": "text",
      "label": "Host/IP",
      "default": "fritz.box"
    },
    {
      "key": "fritzbox.connection.username",
      "type": "text",
      "label": "Benutzername"
    },
    {
      "key": "fritzbox.connection.password",
      "type": "password",
      "label": "Passwort",
      "vault": true
    },
    
    // Feature-Gruppen mit Readonly-Toggle
    {
      "key": "fritzbox.features.system.enabled",
      "type": "toggle",
      "label": "System-Informationen"
    },
    {
      "key": "fritzbox.features.network.enabled",
      "type": "toggle",
      "label": "Netzwerk"
    },
    {
      "key": "fritzbox.features.network.readonly",
      "type": "toggle",
      "label": "Netzwerk: Nur lesen",
      "showIf": {"fritzbox.features.network.enabled": true}
    },
    {
      "key": "fritzbox.features.telephony.enabled",
      "type": "toggle",
      "label": "Telefonie"
    },
    {
      "key": "fritzbox.features.telephony.readonly",
      "type": "toggle",
      "label": "Telefonie: Nur lesen",
      "showIf": {"fritzbox.features.telephony.enabled": true}
    },
    {
      "key": "fritzbox.features.smart_home.enabled",
      "type": "toggle",
      "label": "Smart Home"
    },
    {
      "key": "fritzbox.features.smart_home.readonly",
      "type": "toggle",
      "label": "Smart Home: Nur lesen",
      "showIf": {"fritzbox.features.smart_home.enabled": true}
    },
    {
      "key": "fritzbox.features.storage.enabled",
      "type": "toggle",
      "label": "Storage/NAS"
    },
    {
      "key": "fritzbox.features.storage.readonly",
      "type": "toggle",
      "label": "Storage: Nur lesen",
      "showIf": {"fritzbox.features.storage.enabled": true}
    },
    {
      "key": "fritzbox.features.tv.enabled",
      "type": "toggle",
      "label": "TV/Media (nur Cable-Modelle)"
    }
  ]
}
```

---

## 8. Implementierungs-Roadmap

### Phase 1: Core & Auth (Woche 1)
- [ ] TR-064 SOAP Client
- [ ] PBKDF2/MD5 Authentifizierung
- [ ] Session-Management
- [ ] Config-Struktur

### Phase 2: System & Network (Woche 2)
- [ ] DeviceInfo Service
- [ ] DeviceConfig Service (mit readonly-Check)
- [ ] WLANConfig Service
- [ ] Hosts Service

### Phase 3: Telefonie (Woche 3)
- [ ] OnTel Service (Anruflisten, Telefonbücher)
- [ ] TAM Service (Anrufbeantworter)
- [ ] Audio-Download
- [ ] Call Monitor (optional)

### Phase 4: Smart Home & Storage (Woche 4)
- [ ] HomeAuto Service
- [ ] Storage Service
- [ ] Templates/Routinen

### Phase 5: TV/Media (Woche 5)
- [ ] Media Service (DVB-C)
- [ ] M3U-Export
- [ ] Stream-URL-Generierung

### Phase 6: Tools & UI (Woche 6)
- [ ] Tool-Integration
- [ ] Config UI
- [ ] Dashboard-Widgets
- [ ] Dokumentation

---

## 9. AVM API Dokumentation Referenzen

| Service | Dokumentation | Features |
|---------|---------------|----------|
| DeviceInfo | TR-064_Device_Info.pdf | Modell, Version, Seriennummer |
| DeviceConfig | TR-064_Device_Config.pdf | Reboot, Werksreset, Backup |
| WLANConfiguration | TR-064_WLAN_Configuration.pdf | WLAN, Gast-WLAN |
| X_AVM-DE_Dect | TR-064_DECT.pdf | DECT-Geräte |
| X_AVM-DE_OnTel | TR-064_Contact_SCPD.pdf | Telefonbuch, Anruflisten |
| X_AVM-DE_TAM | TR-064_TAM.pdf | Anrufbeantworter |
| X_AVM-DE_HomeAuto | TR-064_HomeAuto.pdf | Smart Home |
| X_AVM-DE_Media | TR-064_Media.pdf | DVB-C TV Sender |
| X_AVM-DE_Storage | TR-064_Storage.pdf | NAS/FTP |
| X_AVM-DE_Hosts | TR-064_Hosts.pdf | Netzwerkgeräte, WOL |

Alle Dokumente verfügbar unter: https://fritz.com/en/pages/interfaces
