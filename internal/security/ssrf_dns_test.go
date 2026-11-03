package security

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

func TestStrictPublicHTTPClientPinsDNSBeforeRebinding(t *testing.T) {
	server, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	var private atomic.Bool
	var queries atomic.Int32
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 2048)
		for {
			n, peer, err := server.ReadFrom(buf)
			if err != nil {
				return
			}
			var request dnsmessage.Message
			if request.Unpack(buf[:n]) != nil {
				continue
			}
			queries.Add(1)
			response := dnsmessage.Message{Header: dnsmessage.Header{ID: request.ID, Response: true, Authoritative: true}, Questions: request.Questions}
			for _, question := range request.Questions {
				if question.Type == dnsmessage.TypeA {
					address := [4]byte{8, 8, 8, 8}
					if private.Load() {
						address = [4]byte{127, 0, 0, 1}
					}
					response.Answers = append(response.Answers, dnsmessage.Resource{Header: dnsmessage.ResourceHeader{Name: question.Name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET}, Body: &dnsmessage.AResource{A: address}})
				}
			}
			wire, _ := response.Pack()
			_, _ = server.WriteTo(wire, peer)
		}
	}()
	old := net.DefaultResolver
	net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "udp", server.LocalAddr().String())
	}}
	defer func() { net.DefaultResolver = old; server.Close(); <-done }()
	client, err := NewStrictPublicHTTPClientForURLContext(context.Background(), "http://icons.test/icon.png", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.CloseIdleConnections()
	before := queries.Load()
	private.Store(true)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = client.Transport.(*http.Transport).DialContext(ctx, "tcp", "icons.test:80")
	var op *net.OpError
	if !errors.As(err, &op) || op.Addr == nil || op.Addr.String() != "8.8.8.8:80" {
		t.Fatalf("dial did not retain the validated public address: %v", err)
	}
	if queries.Load() != before {
		t.Fatal("transport resolved the hostname again after validation")
	}
	if _, err := NewStrictPublicHTTPClientForURLContext(context.Background(), "http://icons.test/redirect", time.Second); err == nil {
		t.Fatal("next hop accepted rebound private DNS")
	}
}
