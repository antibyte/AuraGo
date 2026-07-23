package sipphone

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/voice"

	"github.com/emiago/diago/audio"
	"github.com/pion/webrtc/v4"
	pionmedia "github.com/pion/webrtc/v4/pkg/media"
)

func TestBrowserMediaPCMUAudioLoopback(t *testing.T) {
	cfg := browserMediaTestConfig()
	service, err := NewBrowserMediaService(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	client, localTrack, remoteTrack, connected := newBrowserMediaTestClient(t)
	defer client.Close()
	offer := completeBrowserOffer(t, client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session, err := service.CreateSession(ctx, "owner", "client-12345678", offer)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: session.AnswerSDP}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-connected:
	case <-ctx.Done():
		t.Fatal("WebRTC client did not connect")
	}

	peer, err := service.ClaimSession("owner", "client-12345678", session.ID)
	if err != nil {
		t.Fatal(err)
	}
	bridge := voice.NewBridge(4)
	defer bridge.Close()
	if err := peer.Attach(ctx, "call-1", bridge); err != nil {
		t.Fatal(err)
	}
	defer peer.Detach("call-1")

	browserLinear := make([]byte, browserPCMFrameSamples*2)
	for i := 0; i < browserPCMFrameSamples; i++ {
		binary.LittleEndian.PutUint16(browserLinear[i*2:i*2+2], uint16(int16(900)))
	}
	browserPCMU := make([]byte, browserPCMFrameSamples)
	if _, err := audio.EncodeUlawTo(browserPCMU, browserLinear); err != nil {
		t.Fatal(err)
	}
	if err := localTrack.WriteSample(pionmedia.Sample{Data: browserPCMU, Duration: 20 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	fromBrowser, err := bridge.NextSend(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if fromBrowser.SampleRate != 8000 || len(fromBrowser.Samples) != browserPCMFrameSamples {
		t.Fatalf("browser PCM = %d Hz/%d samples", fromBrowser.SampleRate, len(fromBrowser.Samples))
	}

	toBrowser := make([]int16, browserPCMFrameSamples)
	for i := range toBrowser {
		toBrowser[i] = -700
	}
	if err := bridge.PushReceive(voice.PCMFrame{Samples: toBrowser, SampleRate: 8000}); err != nil {
		t.Fatal(err)
	}
	var inboundTrack *webrtc.TrackRemote
	select {
	case inboundTrack = <-remoteTrack:
	case <-ctx.Done():
		t.Fatal("browser did not receive the server audio track")
	}
	packet, _, err := inboundTrack.ReadRTP()
	if err != nil {
		t.Fatal(err)
	}
	if len(packet.Payload) != browserPCMFrameSamples {
		t.Fatalf("PCMU payload = %d bytes, want %d", len(packet.Payload), browserPCMFrameSamples)
	}
}

func TestBrowserMediaDiscardsAudioBeforeAttach(t *testing.T) {
	service, err := NewBrowserMediaService(browserMediaTestConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	client, localTrack, _, connected := newBrowserMediaTestClient(t)
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session, err := service.CreateSession(ctx, "owner", "client-12345678", completeBrowserOffer(t, client))
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: session.AnswerSDP}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-connected:
	case <-ctx.Done():
		t.Fatal("WebRTC client did not connect")
	}

	preAttachPCMU := browserPCMUFrame(t, 1200)
	for i := 0; i < 3; i++ {
		if err := localTrack.WriteSample(pionmedia.Sample{Data: preAttachPCMU, Duration: 20 * time.Millisecond}); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(100 * time.Millisecond)

	peer, err := service.ClaimSession("owner", "client-12345678", session.ID)
	if err != nil {
		t.Fatal(err)
	}
	bridge := voice.NewBridge(4)
	defer bridge.Close()
	if err := peer.Attach(ctx, "call-1", bridge); err != nil {
		t.Fatal(err)
	}
	defer peer.Detach("call-1")

	noBufferedAudio, noBufferedAudioCancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer noBufferedAudioCancel()
	if _, err := bridge.NextSend(noBufferedAudio); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("pre-attach browser audio reached SIP bridge: %v", err)
	}

	if err := localTrack.WriteSample(pionmedia.Sample{Data: browserPCMUFrame(t, 700), Duration: 20 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.NextSend(ctx); err != nil {
		t.Fatalf("post-attach browser audio did not reach SIP bridge: %v", err)
	}
}

func TestBrowserMediaMatchesRuntimeConfiguration(t *testing.T) {
	cfg := browserMediaTestConfig()
	service, err := NewBrowserMediaService(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	if !service.MatchesConfig(cfg) {
		t.Fatal("unchanged browser media configuration did not match runtime")
	}
	changedPort := cfg
	changedPort.BrowserMedia.UDPPort++
	if service.MatchesConfig(changedPort) {
		t.Fatal("changed browser media port matched stale runtime")
	}
	changedIP := cfg
	changedIP.BrowserMedia.AdvertisedIP = "192.0.2.10"
	if service.MatchesConfig(changedIP) {
		t.Fatal("changed advertised IP matched stale runtime")
	}
	disabled := cfg
	disabled.BrowserMedia.Enabled = false
	if service.MatchesConfig(disabled) {
		t.Fatal("disabled persisted configuration matched enabled runtime")
	}
}

func TestBrowserMediaSessionOwnershipAndTTL(t *testing.T) {
	cfg := browserMediaTestConfig()
	service, err := NewBrowserMediaService(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	service.sessionTTL = 40 * time.Millisecond

	client, _, _, _ := newBrowserMediaTestClient(t)
	defer client.Close()
	session, err := service.CreateSession(context.Background(), "owner-a", "client-12345678", completeBrowserOffer(t, client))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ClaimSession("owner-b", "client-12345678", session.ID); !errors.Is(err, ErrBrowserSessionOwner) {
		t.Fatalf("foreign owner error = %v", err)
	}
	time.Sleep(80 * time.Millisecond)
	if _, err := service.ClaimSession("owner-a", "client-12345678", session.ID); !errors.Is(err, ErrBrowserSessionInvalid) {
		t.Fatalf("expired session error = %v", err)
	}
}

func browserPCMUFrame(t *testing.T, sample int16) []byte {
	t.Helper()
	linear := make([]byte, browserPCMFrameSamples*2)
	for i := 0; i < browserPCMFrameSamples; i++ {
		binary.LittleEndian.PutUint16(linear[i*2:i*2+2], uint16(sample))
	}
	encoded := make([]byte, browserPCMFrameSamples)
	if _, err := audio.EncodeUlawTo(encoded, linear); err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestBrowserMediaDisconnectGraceAndFailureCallback(t *testing.T) {
	failed := make(chan string, 1)
	peer := &browserMediaPeer{
		callID:          "call-1",
		disconnectGrace: 35 * time.Millisecond,
		onClosed: func(_ *browserMediaPeer, unexpected bool, callID string) {
			if unexpected {
				failed <- callID
			}
		},
	}
	peer.connectionStateChanged(webrtc.PeerConnectionStateDisconnected)
	time.Sleep(10 * time.Millisecond)
	peer.connectionStateChanged(webrtc.PeerConnectionStateConnected)
	select {
	case <-failed:
		t.Fatal("reconnected peer failed before grace elapsed")
	case <-time.After(50 * time.Millisecond):
	}

	peer.connectionStateChanged(webrtc.PeerConnectionStateDisconnected)
	select {
	case callID := <-failed:
		if callID != "call-1" {
			t.Fatalf("failed call ID = %q", callID)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("disconnected peer did not fail after grace")
	}
}

func browserMediaTestConfig() config.SIPConfig {
	cfg := validTestSIPConfig()
	cfg.BindHost = "127.0.0.1"
	cfg.BrowserMedia.Enabled = true
	cfg.BrowserMedia.BindHost = "127.0.0.1"
	cfg.BrowserMedia.UDPPort = 0
	return cfg
}

func newBrowserMediaTestClient(t *testing.T) (*webrtc.PeerConnection, *webrtc.TrackLocalStaticSample, <-chan *webrtc.TrackRemote, <-chan struct{}) {
	t.Helper()
	settings := webrtc.SettingEngine{}
	settings.SetIncludeLoopbackCandidate(true)
	client, err := webrtc.NewAPI(webrtc.WithSettingEngine(settings)).NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}
	localTrack, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypePCMU, ClockRate: 8000, Channels: 1},
		"microphone",
		"browser",
	)
	if err != nil {
		client.Close()
		t.Fatal(err)
	}
	sender, err := client.AddTrack(localTrack)
	if err != nil {
		client.Close()
		t.Fatal(err)
	}
	go drainBrowserRTCP(sender)
	remoteTrack := make(chan *webrtc.TrackRemote, 1)
	client.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		remoteTrack <- track
	})
	connected := make(chan struct{})
	client.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateConnected {
			select {
			case <-connected:
			default:
				close(connected)
			}
		}
	})
	return client, localTrack, remoteTrack, connected
}

func completeBrowserOffer(t *testing.T, client *webrtc.PeerConnection) string {
	t.Helper()
	gatherComplete := webrtc.GatheringCompletePromise(client)
	offer, err := client.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SetLocalDescription(offer); err != nil {
		t.Fatal(err)
	}
	select {
	case <-gatherComplete:
	case <-time.After(5 * time.Second):
		t.Fatal("client ICE gathering timed out")
	}
	return client.LocalDescription().SDP
}
