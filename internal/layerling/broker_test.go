package layerling

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestLayerlingBrokerIsolationAndValidation(t *testing.T) {
	b := NewBroker()
	var write atomic.Bool
	write.Store(true)
	e := b.Register("owner", "first", func(w bool) bool { return !w || write.Load() })
	defer b.Remove(e)
	other := b.Register("other", "second", func(bool) bool { return true })
	defer b.Remove(other)
	ctx := WithOwner(context.Background(), b, "owner", func() bool { return true })
	list, err := Execute(ctx, "list_editors", "", json.RawMessage(`{}`))
	if err != nil || strings.Contains(string(list), other.ID) || !strings.Contains(string(list), e.ID) {
		t.Fatalf("isolation: %s %v", list, err)
	}
	for _, test := range []struct{ op, id, args string }{{"get_scene", other.ID, `{}`}, {"get_scene", "", `{}`}, {"create_shape", e.ID, `{"kind":"bogus"}`}, {"create_shape", e.ID, `{}`}, {"get_scene", e.ID, `{"unknown":true}`}, {"unknown", e.ID, `{}`}} {
		if _, err := Execute(ctx, test.op, test.id, json.RawMessage(test.args)); err == nil {
			t.Fatalf("accepted %+v", test)
		}
	}
	write.Store(false)
	if _, err := Execute(ctx, "create_shape", e.ID, json.RawMessage(`{"kind":"box"}`)); err == nil {
		t.Fatal("read access mutated editor")
	}
	go func() {
		c := <-e.Commands()
		e.Reply(Result{ID: c.ID, OK: true, Data: json.RawMessage(`{"shapeCount":2}`)})
	}()
	result, err := Execute(ctx, "get_scene", e.ID, json.RawMessage(`{}`))
	if err != nil || !strings.Contains(string(result), "shapeCount") {
		t.Fatalf("read: %s %v", result, err)
	}
	if _, err := Execute(context.Background(), "get_scene", e.ID, json.RawMessage(`{}`)); err == nil {
		t.Fatal("unowned request accepted")
	}
}
func TestLayerlingBrokerSequentialAndRevoked(t *testing.T) {
	b := NewBroker()
	var allowed atomic.Bool
	allowed.Store(true)
	e := b.Register("owner", "first", func(bool) bool { return allowed.Load() })
	defer b.Remove(e)
	ctx := WithOwner(context.Background(), b, "owner", func() bool { return true })
	done := make(chan error, 2)
	go func() {
		_, err := Execute(ctx, "save_project", e.ID, json.RawMessage(`{"path":"part.lyl"}`))
		done <- err
	}()
	first := <-e.Commands()
	if !b.AuthorizeFile("owner", e.ID, first.ID, "part.lyl", true) || b.AuthorizeFile("foreign", e.ID, first.ID, "part.lyl", true) || b.AuthorizeFile("owner", e.ID, first.ID, "other.lyl", true) || b.AuthorizeFile("owner", e.ID, first.ID, "part.lyl", false) {
		t.Fatal("file command binding failed")
	}
	go func() { _, err := Execute(ctx, "get_scene", e.ID, json.RawMessage(`{}`)); done <- err }()
	select {
	case <-e.Commands():
		t.Fatal("overlapping commands")
	case <-time.After(30 * time.Millisecond):
	}
	e.Reply(Result{ID: first.ID, OK: true, Data: json.RawMessage(`{}`)})
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	second := <-e.Commands()
	if second.ID == first.ID {
		t.Fatal("request id reused")
	}
	allowed.Store(false)
	if b.AuthorizeFile("owner", e.ID, second.ID, "part.lyl", true) {
		t.Fatal("revoked file command accepted")
	}
	if err := <-done; err == nil {
		t.Fatal("revoked command succeeded")
	}
	select {
	case <-e.Done():
	default:
		t.Fatal("uncertain connection not revoked")
	}
}
func TestLayerlingBrokerRejectsUnmatchedResults(t *testing.T) {
	b := NewBroker()
	e := b.Register("owner", "first", func(bool) bool { return true })
	go func() { <-e.Commands(); e.Reply(Result{ID: "wrong", OK: true, Data: json.RawMessage(`{}`)}) }()
	_, err := Execute(WithOwner(context.Background(), b, "owner", func() bool { return true }), "get_scene", e.ID, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("unmatched result accepted")
	}
	select {
	case <-e.Done():
	default:
		t.Fatal("connection not revoked")
	}
}

func TestLayerlingCanceledCommandIsNotReplayed(t *testing.T) {
	b := NewBroker()
	e := b.Register("owner", "window", func(bool) bool { return true })
	ctx, cancel := context.WithCancel(WithOwner(context.Background(), b, "owner", func() bool { return true }))
	done := make(chan error, 1)
	go func() { _, err := Execute(ctx, "create_shape", e.ID, json.RawMessage(`{"kind":"box"}`)); done <- err }()
	<-e.Commands()
	cancel()
	if err := <-done; err == nil || !strings.Contains(err.Error(), "uncertain") {
		t.Fatal("uncertain mutation not reported", err)
	}
	if _, err := Execute(WithOwner(context.Background(), b, "owner", func() bool { return true }), "get_scene", e.ID, json.RawMessage(`{}`)); err == nil {
		t.Fatal("canceled editor reused")
	}
	if err := b.PublishFile("owner", e.ID, "stale", "part.lyl", func() error { t.Fatal("revoked publication called"); return nil }); err == nil {
		t.Fatal("publication not rejected")
	}
}
