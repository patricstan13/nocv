package main

import (
	"bytes"
	"errors"
	"net"
	"strings"
	"testing"

	"nocv/graph"
)

func TestListenForExplorerUsesLoopbackAndEphemeralPort(t *testing.T) {
	listener, err := listenForExplorer()
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address type = %T, want *net.TCPAddr", listener.Addr())
	}
	if !address.IP.IsLoopback() || address.IP.String() != "127.0.0.1" {
		t.Fatalf("listener IP = %s, want 127.0.0.1", address.IP)
	}
	if address.Port == 0 {
		t.Fatal("listener retained port 0, want an assigned ephemeral port")
	}
}

func TestServeListenerPrintsURLAndEntersHTTPServer(t *testing.T) {
	listener := &stoppedListener{address: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 43123}}
	var output bytes.Buffer
	err := serveListener(&output, graph.New(), listener)
	if err == nil || !strings.Contains(err.Error(), "test listener stopped") {
		t.Fatalf("serveListener() error = %v", err)
	}
	if output.String() != "NOCV visual explorer:\nhttp://127.0.0.1:43123\n" {
		t.Fatalf("output = %q", output.String())
	}
	if !listener.closed {
		t.Fatal("listener was not closed")
	}
}

type stoppedListener struct {
	address net.Addr
	closed  bool
}

func (l *stoppedListener) Accept() (net.Conn, error) {
	return nil, errors.New("test listener stopped")
}

func (l *stoppedListener) Close() error {
	l.closed = true
	return nil
}

func (l *stoppedListener) Addr() net.Addr {
	return l.address
}
