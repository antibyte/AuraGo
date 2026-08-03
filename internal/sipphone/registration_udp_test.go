package sipphone

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emiago/diago"
	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
)

func TestUDPRegistrarNegotiatedRefreshFailureAndUnregister(t *testing.T) {
	probe, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	registrarAddress := probe.LocalAddr().String()
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}

	quietLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	serverUA, err := sipgo.NewUA(
		sipgo.WithUserAgent("AuraGo-Test-Registrar"),
		sipgo.WithUserAgentTransportLayerOptions(sip.WithTransportLayerLogger(quietLogger)),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer serverUA.Close()
	registrar, err := sipgo.NewServer(serverUA)
	if err != nil {
		t.Fatal(err)
	}
	var authorized atomic.Int32
	initialRegistered := make(chan time.Time, 1)
	refreshReceived := make(chan time.Time, 1)
	unregisterReceived := make(chan struct{}, 1)
	handlerErrors := make(chan error, 4)
	registrar.OnRegister(func(request *sip.Request, transaction sip.ServerTransaction) {
		expires := request.GetHeader("Expires")
		contact := request.Contact()
		if (expires != nil && strings.TrimSpace(expires.Value()) == "0") || (contact != nil && strings.TrimSpace(contact.Value()) == "*") {
			if err := transaction.Respond(sip.NewResponseFromRequest(request, sip.StatusOK, "OK", nil)); err != nil {
				handlerErrors <- err
			}
			unregisterReceived <- struct{}{}
			return
		}
		if request.GetHeader("Authorization") == nil {
			response := sip.NewResponseFromRequest(request, sip.StatusUnauthorized, "Unauthorized", nil)
			response.AppendHeader(sip.NewHeader("WWW-Authenticate", `Digest realm="aurago-test", nonce="fixed-test-nonce", algorithm=MD5, qop="auth"`))
			if err := transaction.Respond(response); err != nil {
				handlerErrors <- err
			}
			return
		}
		switch authorized.Add(1) {
		case 1:
			response := sip.NewResponseFromRequest(request, sip.StatusOK, "OK", nil)
			expiry := sip.ExpiresHeader(2)
			response.AppendHeader(&expiry)
			if err := transaction.Respond(response); err != nil {
				handlerErrors <- err
			}
			initialRegistered <- time.Now()
		default:
			if err := transaction.Respond(sip.NewResponseFromRequest(request, sip.StatusForbidden, "Forbidden", nil)); err != nil {
				handlerErrors <- err
			}
			refreshReceived <- time.Now()
		}
	})

	serverCtx, stopServer := context.WithCancel(context.Background())
	ready := make(chan struct{})
	serverCtx = context.WithValue(serverCtx, sipgo.ListenReadyCtxKey, sipgo.ListenReadyCtxValue(ready))
	serverDone := make(chan error, 1)
	go func() { serverDone <- registrar.ListenAndServe(serverCtx, "udp", registrarAddress) }()
	select {
	case <-ready:
	case <-time.After(time.Second):
		stopServer()
		t.Fatal("UDP registrar did not start")
	}
	defer func() {
		stopServer()
		select {
		case err := <-serverDone:
			if err != nil && !errors.Is(err, net.ErrClosed) && !errors.Is(err, context.Canceled) {
				t.Errorf("stop UDP registrar: %v", err)
			}
		case <-time.After(time.Second):
			t.Error("UDP registrar did not stop")
		}
	}()

	_, port, err := net.SplitHostPort(registrarAddress)
	if err != nil {
		t.Fatal(err)
	}
	clientUA, err := sipgo.NewUA(
		sipgo.WithUserAgent("alice"), sipgo.WithUserAgentHostname("127.0.0.1"),
		sipgo.WithUserAgentTransportLayerOptions(sip.WithTransportLayerLogger(quietLogger)),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer clientUA.Close()
	endpoint := diago.NewDiago(clientUA,
		diago.WithLogger(quietLogger),
		diago.WithTransport(diago.Transport{Transport: "udp", BindHost: "127.0.0.1"}),
	)
	uri := sip.Uri{Scheme: "sip", User: "alice", Host: "127.0.0.1"}
	if _, err := fmt.Sscanf(port, "%d", &uri.Port); err != nil {
		t.Fatal(err)
	}
	tx, err := endpoint.RegisterTransaction(context.Background(), uri, diago.RegisterOptions{
		Username: "alice", Password: "secret", Expiry: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Register(context.Background()); err != nil {
		t.Fatal(err)
	}
	registeredAt := <-initialRegistered
	qualifyCtx, cancelQualify := context.WithTimeout(context.Background(), 3*time.Second)
	err = tx.QualifyLoop(qualifyCtx)
	cancelQualify()
	code, status := classifyRegistrationError(err)
	if code != "registration_failed_403" || status != sip.StatusForbidden {
		t.Fatalf("refresh error = %v, classified as %s/%d", err, code, status)
	}
	refreshedAt := <-refreshReceived
	refreshDelay := refreshedAt.Sub(registeredAt)
	if refreshDelay < 700*time.Millisecond || refreshDelay > 2*time.Second {
		t.Fatalf("negotiated refresh delay = %v, want approximately 75%% of two seconds", refreshDelay)
	}
	unregisterCtx, cancelUnregister := context.WithTimeout(context.Background(), time.Second)
	if err := tx.Unregister(unregisterCtx); err != nil {
		cancelUnregister()
		t.Fatal(err)
	}
	cancelUnregister()
	select {
	case <-unregisterReceived:
	case err := <-handlerErrors:
		t.Fatal(err)
	case <-time.After(time.Second):
		t.Fatal("final Expires: 0 unregister was not observed")
	}
}
