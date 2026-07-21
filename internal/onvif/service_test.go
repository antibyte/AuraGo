package onvif

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDiscoveryCapabilityGateAndSenderAddressNormalization(t *testing.T) {
	disabled := NewService(false)
	if _, err := disabled.Discover(context.Background()); err == nil {
		t.Fatal("Discover unexpectedly ignored the broadcast capability gate")
	}

	packet := []byte(`<?xml version="1.0"?><Envelope xmlns="http://www.w3.org/2003/05/soap-envelope"><Body><ProbeMatches xmlns="http://schemas.xmlsoap.org/ws/2005/04/discovery"><ProbeMatch><Scopes>onvif://www.onvif.org/name/Front%20Door onvif://www.onvif.org/hardware/Cam%20X</Scopes><XAddrs>http://203.0.113.77:8080/onvif/device_service</XAddrs></ProbeMatch></ProbeMatches></Body></Envelope>`)
	devices := parseDiscoveryResponse(packet, net.ParseIP("192.168.10.25"))
	if len(devices) != 1 {
		t.Fatalf("devices = %#v, want one", devices)
	}
	if got := devices[0].Endpoint.String(); got != "http://192.168.10.25:8080/onvif/device_service" {
		t.Fatalf("normalized endpoint = %q", got)
	}
	if devices[0].Name != "Front Door" || devices[0].Model != "Cam X" {
		t.Fatalf("unexpected labels: %#v", devices[0])
	}
	if got := parseDiscoveryResponse(packet, net.ParseIP("8.8.8.8")); len(got) != 0 {
		t.Fatalf("public response sender unexpectedly accepted: %#v", got)
	}
}

func TestProfilesUseUsernameTokenAndSetupTokenIsSingleUse(t *testing.T) {
	service := NewService(true)
	endpoint, _ := url.Parse("http://192.168.20.30/onvif/device_service")
	service.SetDiscoverFunc(func(context.Context) ([]DiscoveredDevice, error) {
		return []DiscoveredDevice{{Endpoint: endpoint, Name: "Entry", Model: "TestCam"}}, nil
	})
	candidates, err := service.Discover(context.Background())
	if err != nil || len(candidates) != 1 {
		t.Fatalf("Discover = %#v, %v", candidates, err)
	}

	transport := &soapRoundTripper{}
	service.SetHTTPClient(&http.Client{Transport: transport, Timeout: time.Second})
	result, err := service.Profiles(context.Background(), ProfileRequest{
		CandidateID: candidates[0].ID, Username: "camera-user", Password: "camera-password",
	})
	if err != nil {
		t.Fatalf("Profiles: %v", err)
	}
	if len(result.Profiles) != 1 || result.Profiles[0].Codec != "H264" || result.Profiles[0].Width != 1920 {
		t.Fatalf("unexpected profiles: %#v", result.Profiles)
	}
	transport.mu.Lock()
	requests := append([]string(nil), transport.requests...)
	transport.mu.Unlock()
	if len(requests) != 3 {
		t.Fatalf("SOAP request count = %d, want 3", len(requests))
	}
	for _, request := range requests {
		if !strings.Contains(request, "PasswordDigest") || !strings.Contains(request, "camera-user") {
			t.Fatalf("WS-Security UsernameToken missing: %s", request)
		}
		if strings.Contains(request, "camera-password") {
			t.Fatal("plain password appeared in SOAP request")
		}
	}
	source, err := service.Consume(result.SetupToken, result.Profiles[0].ID)
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if !strings.HasPrefix(source, "onvif://camera-user:camera-password@192.168.20.30/") || !strings.Contains(source, "subtype=profile-main") {
		t.Fatalf("unexpected vault source shape: %q", source)
	}
	if _, err := service.Consume(result.SetupToken, result.Profiles[0].ID); err == nil {
		t.Fatal("setup token replay unexpectedly succeeded")
	}
}

func TestProfilesRejectCrossHostMediaServiceAndOversizedSOAP(t *testing.T) {
	service := NewService(true)
	endpoint, _ := url.Parse("http://192.168.30.40/onvif/device_service")
	service.SetDiscoverFunc(func(context.Context) ([]DiscoveredDevice, error) {
		return []DiscoveredDevice{{Endpoint: endpoint}}, nil
	})
	candidates, err := service.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	service.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(request.Body)
		response := `<Envelope><Body><GetDeviceInformationResponse><Manufacturer>A</Manufacturer><Model>B</Model></GetDeviceInformationResponse></Body></Envelope>`
		if strings.Contains(string(body), "GetCapabilities") {
			response = `<Envelope><Body><GetCapabilitiesResponse><Capabilities><Media><XAddr>http://192.168.30.99/onvif/media</XAddr></Media></Capabilities></GetCapabilitiesResponse></Body></Envelope>`
		}
		return soapResponse(response), nil
	})})
	if _, err := service.Profiles(context.Background(), ProfileRequest{CandidateID: candidates[0].ID}); err == nil || !strings.Contains(err.Error(), "different host") {
		t.Fatalf("cross-host media service error = %v", err)
	}

	service.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return soapResponse(strings.Repeat("x", maxSOAPResponse+1)), nil
	})})
	if _, err := service.Profiles(context.Background(), ProfileRequest{CandidateID: candidates[0].ID}); err == nil || !strings.Contains(err.Error(), "1 MiB") {
		t.Fatalf("oversized SOAP error = %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type soapRoundTripper struct {
	mu       sync.Mutex
	requests []string
}

func (transport *soapRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(request.Body)
	text := string(body)
	transport.mu.Lock()
	transport.requests = append(transport.requests, text)
	transport.mu.Unlock()
	response := `<Envelope><Body><GetDeviceInformationResponse><Manufacturer>Acme</Manufacturer><Model>TestCam</Model></GetDeviceInformationResponse></Body></Envelope>`
	if strings.Contains(text, "GetCapabilities") {
		response = `<Envelope><Body><GetCapabilitiesResponse><Capabilities><Media><XAddr>http://192.168.20.30/onvif/media_service</XAddr></Media></Capabilities></GetCapabilitiesResponse></Body></Envelope>`
	}
	if strings.Contains(text, "GetProfiles") {
		response = `<Envelope><Body><GetProfilesResponse><Profiles token="profile-main"><Name>Main stream</Name><VideoEncoderConfiguration><Encoding>H264</Encoding><Resolution><Width>1920</Width><Height>1080</Height></Resolution><RateControl><FrameRateLimit>25</FrameRateLimit></RateControl></VideoEncoderConfiguration></Profiles></GetProfilesResponse></Body></Envelope>`
	}
	return soapResponse(response), nil
}

func soapResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
