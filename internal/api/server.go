package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
)

// Server is the manager's local JSON query API, bound strictly to a
// loopback address.
type Server struct {
	httpServer *http.Server
}

// NewServer builds the API server. It refuses to construct if addr does
// not resolve to a loopback address, so a config typo can never
// accidentally expose the unauthenticated local API beyond this machine.
func NewServer(addr string, handler http.Handler) (*Server, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("parse api_listen_addr %q: %w", addr, err)
	}
	if !isLoopbackHost(host) {
		return nil, fmt.Errorf("api_listen_addr %q must be a loopback address (127.0.0.1, ::1, or localhost)", addr)
	}
	return &Server{httpServer: &http.Server{Addr: addr, Handler: handler}}, nil
}

func isLoopbackHost(host string) bool {
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return false
	}
	for _, ip := range ips {
		if !ip.IsLoopback() {
			return false
		}
	}
	return true
}

// ListenAndServe blocks serving plain HTTP (no TLS — loopback-only, no
// client cert needed) until the server is shut down.
func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
