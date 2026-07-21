// Package onvif implements the small, security-bounded ONVIF subset used by
// AuraGo's network camera setup flow.
package onvif

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/security"
)

const (
	maxDiscoveryDevices = 128
	maxSetupSessions    = 32
	sessionTTL          = 5 * time.Minute
	maxSOAPResponse     = 1 << 20
	maxDiscoveryPacket  = 64 << 10
)

// Candidate is the deliberately limited discovery view returned to browsers.
type Candidate struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Model string `json:"model,omitempty"`
	IP    string `json:"ip"`
	Port  int    `json:"port"`
}

// Profile describes one selectable camera media profile.
type Profile struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Codec     string `json:"codec,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	FrameRate int    `json:"frame_rate,omitempty"`
}

// ProfileRequest selects a discovered candidate or a manually entered local endpoint.
type ProfileRequest struct {
	CandidateID string
	Address     string
	Username    string
	Password    string
}

// ProfileResult is safe to return to the setup client.
type ProfileResult struct {
	SetupToken string    `json:"setup_token"`
	Name       string    `json:"name,omitempty"`
	Model      string    `json:"model,omitempty"`
	Profiles   []Profile `json:"profiles"`
}

type discoveredDevice struct {
	endpoint *url.URL
	name     string
	model    string
	expires  time.Time
}

type setupSession struct {
	endpoint *url.URL
	username string
	password string
	profiles map[string]Profile
	expires  time.Time
	reserved string
}

// SetupReservation holds a setup token while its source is being published.
// Commit consumes the token; Release makes it available for another attempt.
type SetupReservation struct {
	service *Service
	token   string
	lease   string
	source  string
	once    sync.Once
}

// Source returns the vault-only source represented by this reservation.
func (r *SetupReservation) Source() string {
	if r == nil {
		return ""
	}
	return r.source
}

// Commit permanently consumes the reserved setup token.
func (r *SetupReservation) Commit() {
	if r == nil {
		return
	}
	r.once.Do(func() { r.service.finishReservation(r.token, r.lease, true) })
}

// Release returns an uncommitted setup token to the setup session.
func (r *SetupReservation) Release() {
	if r == nil {
		return
	}
	r.once.Do(func() { r.service.finishReservation(r.token, r.lease, false) })
}

// DiscoverFunc allows the UDP transport to be replaced in focused tests.
type DiscoverFunc func(context.Context) ([]DiscoveredDevice, error)

// DiscoveredDevice is the normalized internal result of WS-Discovery.
type DiscoveredDevice struct {
	Endpoint *url.URL
	Name     string
	Model    string
}

// Service owns short-lived discovery candidates and credential-bearing setup sessions.
type Service struct {
	mu          sync.Mutex
	broadcastOK bool
	now         func() time.Time
	discover    DiscoverFunc
	httpClient  *http.Client
	candidates  map[string]discoveredDevice
	setups      map[string]setupSession
}

// NewService creates an ONVIF setup service. Automatic discovery is capability gated.
func NewService(broadcastOK bool) *Service {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Camera setup is deliberately private-network scoped. Never send
	// credential-bearing ONVIF traffic through an environment proxy.
	transport.Proxy = nil
	return &Service{
		broadcastOK: broadcastOK,
		now:         time.Now,
		discover:    DiscoverNetwork,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   20 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		candidates: make(map[string]discoveredDevice),
		setups:     make(map[string]setupSession),
	}
}

// SetDiscoverFunc replaces network discovery for deterministic tests.
func (s *Service) SetDiscoverFunc(fn DiscoverFunc) {
	if s == nil || fn == nil {
		return
	}
	s.mu.Lock()
	s.discover = fn
	s.mu.Unlock()
}

// SetHTTPClient replaces the bounded SOAP client for deterministic tests.
func (s *Service) SetHTTPClient(client *http.Client) {
	if s == nil || client == nil {
		return
	}
	s.mu.Lock()
	s.httpClient = client
	s.mu.Unlock()
}

// DiscoveryAvailable explains whether WS-Discovery may be attempted.
func (s *Service) DiscoveryAvailable() (bool, string) {
	if s == nil {
		return false, "ONVIF discovery is unavailable"
	}
	if !s.broadcastOK {
		return false, "Automatic discovery requires local broadcast access; use a manual local address or stream URL"
	}
	return true, ""
}

// Discover performs one bounded WS-Discovery round and stores opaque candidates.
func (s *Service) Discover(ctx context.Context) ([]Candidate, error) {
	available, reason := s.DiscoveryAvailable()
	if !available {
		return nil, errors.New(reason)
	}
	s.mu.Lock()
	discover := s.discover
	s.mu.Unlock()
	devices, err := discover(ctx)
	if err != nil {
		return nil, fmt.Errorf("ONVIF discovery failed: %w", err)
	}
	now := s.now()
	result := make([]Candidate, 0, min(len(devices), maxDiscoveryDevices))
	seen := make(map[string]struct{}, len(devices))
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(now)
	for _, device := range devices {
		if len(result) >= maxDiscoveryDevices {
			break
		}
		endpoint, err := normalizeLocalEndpoint(device.Endpoint)
		if err != nil {
			continue
		}
		endpointKey := endpoint.String()
		if _, ok := seen[endpointKey]; ok {
			continue
		}
		seen[endpointKey] = struct{}{}
		id, err := randomToken()
		if err != nil {
			return nil, fmt.Errorf("create discovery candidate: %w", err)
		}
		s.makeCandidateRoomLocked()
		name := sanitizeLabel(device.Name, 96)
		if name == "" {
			name = "ONVIF camera"
		}
		model := sanitizeLabel(device.Model, 96)
		s.candidates[id] = discoveredDevice{endpoint: endpoint, name: name, model: model, expires: now.Add(sessionTTL)}
		port := endpointPort(endpoint)
		result = append(result, Candidate{ID: id, Name: name, Model: model, IP: endpoint.Hostname(), Port: port})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Name == result[j].Name {
			return result[i].IP < result[j].IP
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// Profiles queries the minimal device/media SOAP surface and stores credentials only in memory.
func (s *Service) Profiles(ctx context.Context, request ProfileRequest) (ProfileResult, error) {
	if s == nil {
		return ProfileResult{}, fmt.Errorf("ONVIF setup is unavailable")
	}
	username := strings.TrimSpace(request.Username)
	password := request.Password
	security.RegisterSensitive(username)
	security.RegisterSensitive(password)
	now := s.now()
	s.mu.Lock()
	s.pruneLocked(now)
	client := s.httpClient
	device, found := s.candidates[strings.TrimSpace(request.CandidateID)]
	s.mu.Unlock()
	var endpoint *url.URL
	name := ""
	model := ""
	if strings.TrimSpace(request.CandidateID) != "" {
		if !found {
			return ProfileResult{}, fmt.Errorf("discovery candidate is missing or expired")
		}
		endpoint = cloneURL(device.endpoint)
		name, model = device.name, device.model
	} else {
		parsed, err := parseManualAddress(request.Address)
		if err != nil {
			return ProfileResult{}, err
		}
		endpoint = parsed
	}
	if client == nil {
		return ProfileResult{}, fmt.Errorf("ONVIF HTTP client is unavailable")
	}
	deviceInfo, err := queryDeviceInformation(ctx, client, endpoint, username, password)
	if err == nil {
		if name == "" {
			name = deviceInfo.Name
		}
		if model == "" {
			model = deviceInfo.Model
		}
	}
	mediaEndpoint, err := queryMediaEndpoint(ctx, client, endpoint, username, password)
	if err != nil {
		return ProfileResult{}, err
	}
	profiles, err := queryProfiles(ctx, client, mediaEndpoint, username, password)
	if err != nil {
		return ProfileResult{}, err
	}
	if len(profiles) == 0 {
		return ProfileResult{}, fmt.Errorf("camera returned no usable ONVIF media profiles")
	}
	token, err := randomToken()
	if err != nil {
		return ProfileResult{}, fmt.Errorf("create setup token: %w", err)
	}
	profileMap := make(map[string]Profile, len(profiles))
	for _, profile := range profiles {
		profileMap[profile.ID] = profile
	}
	s.mu.Lock()
	s.pruneLocked(now)
	if !s.makeSetupRoomLocked() {
		s.mu.Unlock()
		return ProfileResult{}, fmt.Errorf("too many active camera setup sessions")
	}
	expires := now.Add(sessionTTL)
	s.setups[token] = setupSession{
		endpoint: cloneURL(endpoint), username: username, password: password,
		profiles: profileMap, expires: expires,
	}
	s.mu.Unlock()
	time.AfterFunc(sessionTTL, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if session, ok := s.setups[token]; ok && session.expires.Equal(expires) && !s.now().Before(expires) {
			delete(s.setups, token)
		}
	})
	return ProfileResult{SetupToken: token, Name: sanitizeLabel(name, 96), Model: sanitizeLabel(model, 96), Profiles: profiles}, nil
}

// Reserve validates and exclusively leases a setup token while its source is
// being published. The caller must Commit or Release the reservation.
func (s *Service) Reserve(token, profileID string) (*SetupReservation, error) {
	if s == nil {
		return nil, fmt.Errorf("ONVIF setup is unavailable")
	}
	token = strings.TrimSpace(token)
	profileID = strings.TrimSpace(profileID)
	lease, err := randomToken()
	if err != nil {
		return nil, fmt.Errorf("reserve setup token: %w", err)
	}
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(now)
	session, ok := s.setups[token]
	if !ok {
		return nil, fmt.Errorf("setup token is missing, expired, or already used")
	}
	if _, ok := session.profiles[profileID]; !ok {
		return nil, fmt.Errorf("selected ONVIF profile is unavailable")
	}
	if session.reserved != "" {
		return nil, fmt.Errorf("setup token is already in use")
	}
	session.reserved = lease
	s.setups[token] = session
	source := &url.URL{Scheme: "onvif", Host: session.endpoint.Host, Path: session.endpoint.Path}
	if session.username != "" || session.password != "" {
		source.User = url.UserPassword(session.username, session.password)
	}
	query := source.Query()
	query.Set("subtype", profileID)
	source.RawQuery = query.Encode()
	value := source.String()
	security.RegisterSensitive(value)
	return &SetupReservation{service: s, token: token, lease: lease, source: value}, nil
}

func (s *Service) finishReservation(token, lease string, commit bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(s.now())
	session, ok := s.setups[token]
	if !ok || session.reserved != lease {
		return
	}
	if commit {
		delete(s.setups, token)
		return
	}
	session.reserved = ""
	s.setups[token] = session
}

func (s *Service) pruneLocked(now time.Time) {
	for id, item := range s.candidates {
		if !now.Before(item.expires) {
			delete(s.candidates, id)
		}
	}
	for id, item := range s.setups {
		if !now.Before(item.expires) {
			delete(s.setups, id)
		}
	}
}

func (s *Service) makeCandidateRoomLocked() {
	for len(s.candidates) >= maxDiscoveryDevices {
		var oldestID string
		var oldest time.Time
		for id, item := range s.candidates {
			if oldestID == "" || item.expires.Before(oldest) {
				oldestID, oldest = id, item.expires
			}
		}
		if oldestID == "" {
			return
		}
		delete(s.candidates, oldestID)
	}
}

func (s *Service) makeSetupRoomLocked() bool {
	for len(s.setups) >= maxSetupSessions {
		var oldestID string
		var oldest time.Time
		for id, item := range s.setups {
			if item.reserved != "" {
				continue
			}
			if oldestID == "" || item.expires.Before(oldest) {
				oldestID, oldest = id, item.expires
			}
		}
		if oldestID == "" {
			return false
		}
		delete(s.setups, oldestID)
	}
	return true
}

// DiscoverNetwork sends one probe per suitable private IPv4 interface.
func DiscoverNetwork(ctx context.Context) ([]DiscoveredDevice, error) {
	deadline := time.Now().Add(5 * time.Second)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	addresses, err := privateInterfaceAddresses()
	if err != nil {
		return nil, err
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("no suitable private network interface")
	}
	type result struct {
		devices []DiscoveredDevice
		err     error
	}
	results := make(chan result, len(addresses))
	var wg sync.WaitGroup
	for _, address := range addresses {
		address := append(net.IP(nil), address...)
		wg.Add(1)
		go func() {
			defer wg.Done()
			devices, discoverErr := discoverOnInterface(ctx, address, deadline)
			results <- result{devices: devices, err: discoverErr}
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	unique := make(map[string]DiscoveredDevice)
	var firstErr error
	for item := range results {
		if item.err != nil && firstErr == nil && !errors.Is(item.err, context.DeadlineExceeded) && !errors.Is(item.err, context.Canceled) {
			firstErr = item.err
		}
		for _, device := range item.devices {
			if len(unique) >= maxDiscoveryDevices {
				break
			}
			unique[device.Endpoint.String()] = device
		}
	}
	out := make([]DiscoveredDevice, 0, len(unique))
	for _, device := range unique {
		out = append(out, device)
	}
	if len(out) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return out, nil
}

func discoverOnInterface(ctx context.Context, localIP net.IP, deadline time.Time) ([]DiscoveredDevice, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: localIP})
	if err != nil {
		return nil, fmt.Errorf("open discovery socket: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(deadline)
	messageID, err := randomToken()
	if err != nil {
		return nil, err
	}
	probe := `<?xml version="1.0" encoding="UTF-8"?><e:Envelope xmlns:e="http://www.w3.org/2003/05/soap-envelope" xmlns:w="http://schemas.xmlsoap.org/ws/2004/08/addressing" xmlns:d="http://schemas.xmlsoap.org/ws/2005/04/discovery" xmlns:dn="http://www.onvif.org/ver10/network/wsdl"><e:Header><w:MessageID>urn:uuid:` + messageID + `</w:MessageID><w:To e:mustUnderstand="true">urn:schemas-xmlsoap-org:ws:2005:04:discovery</w:To><w:Action e:mustUnderstand="true">http://schemas.xmlsoap.org/ws/2005/04/discovery/Probe</w:Action></e:Header><e:Body><d:Probe><d:Types>dn:NetworkVideoTransmitter</d:Types></d:Probe></e:Body></e:Envelope>`
	if _, err := conn.WriteToUDP([]byte(probe), &net.UDPAddr{IP: net.ParseIP("239.255.255.250"), Port: 3702}); err != nil {
		return nil, fmt.Errorf("send discovery probe: %w", err)
	}
	result := make([]DiscoveredDevice, 0)
	buffer := make([]byte, maxDiscoveryPacket)
	for len(result) < maxDiscoveryDevices {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		n, sender, readErr := conn.ReadFromUDP(buffer)
		if readErr != nil {
			if networkErr, ok := readErr.(net.Error); ok && networkErr.Timeout() {
				break
			}
			return result, fmt.Errorf("read discovery response: %w", readErr)
		}
		result = append(result, parseDiscoveryResponse(buffer[:n], sender.IP)...)
	}
	return result, nil
}

type discoveryEnvelope struct {
	Matches []struct {
		XAddrs string `xml:"XAddrs"`
		Scopes string `xml:"Scopes"`
	} `xml:"Body>ProbeMatches>ProbeMatch"`
}

func parseDiscoveryResponse(data []byte, sender net.IP) []DiscoveredDevice {
	if !isLocalUnicastIP(sender) {
		return nil
	}
	var envelope discoveryEnvelope
	if err := xml.Unmarshal(data, &envelope); err != nil {
		return nil
	}
	result := make([]DiscoveredDevice, 0, len(envelope.Matches))
	for _, match := range envelope.Matches {
		name, model := discoveryLabels(match.Scopes)
		for _, rawAddress := range strings.Fields(match.XAddrs) {
			endpoint, err := url.Parse(rawAddress)
			if err != nil || endpoint == nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
				continue
			}
			port := endpoint.Port()
			if port == "" {
				endpoint.Host = sender.String()
			} else {
				endpoint.Host = net.JoinHostPort(sender.String(), port)
			}
			normalized, err := normalizeLocalEndpoint(endpoint)
			if err == nil {
				result = append(result, DiscoveredDevice{Endpoint: normalized, Name: name, Model: model})
			}
		}
	}
	return result
}

func discoveryLabels(scopes string) (string, string) {
	var name, model string
	for _, scope := range strings.Fields(scopes) {
		decoded, err := url.PathUnescape(scope)
		if err != nil {
			decoded = scope
		}
		lower := strings.ToLower(decoded)
		switch {
		case strings.Contains(lower, "/name/"):
			name = decoded[strings.LastIndex(decoded, "/")+1:]
		case strings.Contains(lower, "/hardware/"):
			model = decoded[strings.LastIndex(decoded, "/")+1:]
		}
	}
	return sanitizeLabel(name, 96), sanitizeLabel(model, 96)
}

func privateInterfaceAddresses() ([]net.IP, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("list network interfaces: %w", err)
	}
	result := make([]net.IP, 0)
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err == nil && ip.To4() != nil && isLocalUnicastIP(ip) {
				result = append(result, ip.To4())
			}
		}
	}
	return result, nil
}

type deviceInformation struct {
	Name  string
	Model string
}

func queryDeviceInformation(ctx context.Context, client *http.Client, endpoint *url.URL, username, password string) (deviceInformation, error) {
	body := `<tds:GetDeviceInformation xmlns:tds="http://www.onvif.org/ver10/device/wsdl"/>`
	data, err := soapRequest(ctx, client, endpoint, username, password, "http://www.onvif.org/ver10/device/wsdl/GetDeviceInformation", body)
	if err != nil {
		return deviceInformation{}, err
	}
	var response struct {
		Manufacturer string `xml:"Body>GetDeviceInformationResponse>Manufacturer"`
		Model        string `xml:"Body>GetDeviceInformationResponse>Model"`
	}
	if err := xml.Unmarshal(data, &response); err != nil {
		return deviceInformation{}, fmt.Errorf("decode ONVIF device information")
	}
	return deviceInformation{Name: sanitizeLabel(response.Manufacturer, 96), Model: sanitizeLabel(response.Model, 96)}, nil
}

func queryMediaEndpoint(ctx context.Context, client *http.Client, endpoint *url.URL, username, password string) (*url.URL, error) {
	body := `<tds:GetCapabilities xmlns:tds="http://www.onvif.org/ver10/device/wsdl"><tds:Category>Media</tds:Category></tds:GetCapabilities>`
	data, err := soapRequest(ctx, client, endpoint, username, password, "http://www.onvif.org/ver10/device/wsdl/GetCapabilities", body)
	if err != nil {
		return nil, err
	}
	var response struct {
		XAddr string `xml:"Body>GetCapabilitiesResponse>Capabilities>Media>XAddr"`
	}
	if err := xml.Unmarshal(data, &response); err != nil || strings.TrimSpace(response.XAddr) == "" {
		return nil, fmt.Errorf("camera returned no ONVIF media service")
	}
	mediaURL, err := url.Parse(strings.TrimSpace(response.XAddr))
	if err != nil || mediaURL == nil || (mediaURL.Scheme != "http" && mediaURL.Scheme != "https") {
		return nil, fmt.Errorf("camera returned an invalid ONVIF media service")
	}
	mediaIP := net.ParseIP(mediaURL.Hostname())
	deviceIP := net.ParseIP(endpoint.Hostname())
	if mediaIP == nil || deviceIP == nil || !mediaIP.Equal(deviceIP) {
		return nil, fmt.Errorf("camera media service attempted to use a different host")
	}
	return normalizeLocalEndpoint(mediaURL)
}

func queryProfiles(ctx context.Context, client *http.Client, endpoint *url.URL, username, password string) ([]Profile, error) {
	body := `<trt:GetProfiles xmlns:trt="http://www.onvif.org/ver10/media/wsdl"/>`
	data, err := soapRequest(ctx, client, endpoint, username, password, "http://www.onvif.org/ver10/media/wsdl/GetProfiles", body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Profiles []struct {
			Token string `xml:"token,attr"`
			Name  string `xml:"Name"`
			Video struct {
				Encoding   string `xml:"Encoding"`
				Resolution struct {
					Width  int `xml:"Width"`
					Height int `xml:"Height"`
				} `xml:"Resolution"`
				Rate struct {
					FrameRate int `xml:"FrameRateLimit"`
				} `xml:"RateControl"`
			} `xml:"VideoEncoderConfiguration"`
		} `xml:"Body>GetProfilesResponse>Profiles"`
	}
	if err := xml.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("decode ONVIF media profiles")
	}
	profiles := make([]Profile, 0, len(response.Profiles))
	for _, raw := range response.Profiles {
		id := sanitizeProfileID(raw.Token)
		if id == "" {
			continue
		}
		name := sanitizeLabel(raw.Name, 96)
		if name == "" {
			name = id
		}
		profiles = append(profiles, Profile{
			ID: id, Name: name, Codec: sanitizeLabel(strings.ToUpper(raw.Video.Encoding), 24),
			Width: raw.Video.Resolution.Width, Height: raw.Video.Resolution.Height, FrameRate: raw.Video.Rate.FrameRate,
		})
	}
	return profiles, nil
}

func soapRequest(ctx context.Context, client *http.Client, endpoint *url.URL, username, password, action, body string) ([]byte, error) {
	securityHeader, err := usernameToken(username, password)
	if err != nil {
		return nil, fmt.Errorf("create ONVIF authentication header: %w", err)
	}
	envelope := `<?xml version="1.0" encoding="UTF-8"?><s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope">` + securityHeader + `<s:Body>` + body + `</s:Body></s:Envelope>`
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), strings.NewReader(envelope))
	if err != nil {
		return nil, fmt.Errorf("create ONVIF request")
	}
	request.Header.Set("Content-Type", `application/soap+xml; charset=utf-8; action="`+action+`"`)
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("ONVIF camera request failed")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("ONVIF camera returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxSOAPResponse+1))
	if err != nil {
		return nil, fmt.Errorf("read ONVIF response")
	}
	if len(data) > maxSOAPResponse {
		return nil, fmt.Errorf("ONVIF response exceeds the 1 MiB limit")
	}
	return data, nil
}

func usernameToken(username, password string) (string, error) {
	if username == "" && password == "" {
		return "<s:Header/>", nil
	}
	nonce := make([]byte, 20)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	created := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	hash := sha1.New() // #nosec G505 -- WS-Security UsernameToken requires SHA-1 by specification.
	_, _ = hash.Write(nonce)
	_, _ = hash.Write([]byte(created))
	_, _ = hash.Write([]byte(password))
	digest := base64.StdEncoding.EncodeToString(hash.Sum(nil))
	var escapedUser bytes.Buffer
	_ = xml.EscapeText(&escapedUser, []byte(username))
	return `<s:Header><wsse:Security s:mustUnderstand="1" xmlns:wsse="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd" xmlns:wsu="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd"><wsse:UsernameToken><wsse:Username>` + escapedUser.String() + `</wsse:Username><wsse:Password Type="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0#PasswordDigest">` + digest + `</wsse:Password><wsse:Nonce EncodingType="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-soap-message-security-1.0#Base64Binary">` + base64.StdEncoding.EncodeToString(nonce) + `</wsse:Nonce><wsu:Created>` + created + `</wsu:Created></wsse:UsernameToken></wsse:Security></s:Header>`, nil
}

func parseManualAddress(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("a private camera IP or ONVIF service address is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	endpoint, err := url.Parse(raw)
	if err != nil || endpoint == nil || endpoint.Hostname() == "" {
		return nil, fmt.Errorf("invalid ONVIF service address")
	}
	if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return nil, fmt.Errorf("ONVIF service address must use HTTP or HTTPS")
	}
	if endpoint.Path == "" || endpoint.Path == "/" {
		endpoint.Path = "/onvif/device_service"
	}
	return normalizeLocalEndpoint(endpoint)
}

func normalizeLocalEndpoint(endpoint *url.URL) (*url.URL, error) {
	if endpoint == nil || endpoint.User != nil || endpoint.Hostname() == "" {
		return nil, fmt.Errorf("invalid local ONVIF endpoint")
	}
	if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return nil, fmt.Errorf("invalid ONVIF endpoint scheme")
	}
	ip := net.ParseIP(endpoint.Hostname())
	if !isLocalUnicastIP(ip) {
		return nil, fmt.Errorf("ONVIF endpoint must use a concrete private IP address")
	}
	if rawPort := endpoint.Port(); rawPort != "" {
		port, err := strconv.Atoi(rawPort)
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("invalid ONVIF endpoint port")
		}
	}
	clean := cloneURL(endpoint)
	clean.Fragment = ""
	clean.RawQuery = ""
	return clean, nil
}

func isLocalUnicastIP(ip net.IP) bool {
	return ip != nil && !ip.IsUnspecified() && !ip.IsLoopback() && !ip.IsMulticast() && (ip.IsPrivate() || ip.IsLinkLocalUnicast())
}

func endpointPort(endpoint *url.URL) int {
	if value, err := strconv.Atoi(endpoint.Port()); err == nil && value > 0 {
		return value
	}
	if endpoint.Scheme == "https" {
		return 443
	}
	return 80
}

func randomToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func sanitizeLabel(value string, limit int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	value = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, value)
	runes := []rune(value)
	if len(runes) > limit {
		value = string(runes[:limit])
	}
	return value
}

func sanitizeProfileID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 {
		return ""
	}
	for _, r := range value {
		if r < 0x21 || r == 0x7f || r == '&' || r == '?' || r == '#' {
			return ""
		}
	}
	return value
}

func cloneURL(value *url.URL) *url.URL {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
