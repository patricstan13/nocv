package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	"nocv/graph"
	"nocv/internal/webui"
)

func serve(out io.Writer, g *graph.Graph) error {
	listener, err := listenForExplorer()
	if err != nil {
		return err
	}
	return serveListener(out, g, listener)
}

func serveListener(out io.Writer, g *graph.Graph, listener net.Listener) error {
	defer listener.Close()

	if _, err := fmt.Fprintf(out, "NOCV visual explorer:\nhttp://%s\n", listener.Addr()); err != nil {
		return err
	}
	server := &http.Server{Handler: webui.Handler(g)}
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func listenForExplorer() (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:0")
}
