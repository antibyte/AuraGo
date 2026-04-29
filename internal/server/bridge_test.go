package server

import (
	"net"
	"sync/atomic"
	"testing"
)

func TestCloseTCPBridgeClosesListenerOnce(t *testing.T) {
	ln := &countingListener{}
	s := &Server{}
	s.setTCPBridgeListener(ln)

	s.CloseTCPBridge()
	s.CloseTCPBridge()

	if got := ln.closeCount.Load(); got != 1 {
		t.Fatalf("close count = %d, want 1", got)
	}
	if s.bridgeListener != nil {
		t.Fatalf("bridge listener was not cleared")
	}
}

type countingListener struct {
	closeCount atomic.Int32
}

func (l *countingListener) Accept() (net.Conn, error) { return nil, net.ErrClosed }
func (l *countingListener) Close() error {
	l.closeCount.Add(1)
	return nil
}
func (l *countingListener) Addr() net.Addr { return dummyAddr("bridge-test") }

type dummyAddr string

func (a dummyAddr) Network() string { return "test" }
func (a dummyAddr) String() string  { return string(a) }
