package server

import (
	"net/http"
	"strings"
	"testing"

	"aurago/internal/dockerstacks"
)

// TestDockerManagerLiveReadsStayAllowedInReadOnlyMode pins the docker.readonly scope of the live
// routes: the event stream, its subscriptions, the log follows and the disk usage are reads, the
// terminal is refused.
func TestDockerManagerLiveReadsStayAllowedInReadOnlyMode(t *testing.T) {
	f := newDMLiveFixture(t)
	f.readOnly = true
	f.stacks.listing = dockerstacks.Listing{Stacks: []dockerstacks.StackSummary{{Name: "shop", Dir: "/opt/stacks/shop"}}}

	if hello := f.open(http.MethodGet, "/stream", nil).nextEvent(); hello.event != "hello" {
		t.Fatalf("stream = %s %s", hello.event, hello.data)
	}
	put := f.open(http.MethodPut, "/stream/subscriptions", strings.NewReader(`{"stream_id":"`+f.live.client.id+`","hosts":["local"]}`))
	if body := put.body(); put.resp.StatusCode != http.StatusOK {
		t.Fatalf("subscriptions = %d %v", put.resp.StatusCode, body)
	}
	for _, path := range []string{
		"/hosts/local/containers/web1/logs?follow=1",
		"/hosts/local/stacks/shop/logs?follow=1",
		"/hosts/local/projects/shop/logs?follow=1",
	} {
		if end := f.open(http.MethodGet, path, nil).nextEvent(); end.event != "end" {
			t.Fatalf("%s = %s %s, want a stream that ends", path, end.event, end.data)
		}
	}
	if body := f.open(http.MethodGet, "/hosts/local/df", nil).body(); body["status"] != "ok" {
		t.Fatalf("df = %v", body)
	}
	f.open(http.MethodGet, "/hosts/local/containers/web1/terminal", nil).expectError(http.StatusForbidden, "read_only")
}
