package transport

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
)

// Server wraps an http.Server bound to a pre-created TLS listener. Binding
// eagerly (in the constructor, rather than inside Serve) lets callers —
// and tests binding to port 0 for an OS-assigned port — learn the actual
// bound address immediately via Addr(), before serving starts.
type Server struct {
	httpServer *http.Server
	listener   net.Listener
}

// Addr returns the address this server is bound to.
func (s *Server) Addr() net.Addr { return s.listener.Addr() }

// Serve blocks accepting and serving TLS connections until the server is
// shut down, at which point it returns http.ErrServerClosed.
func (s *Server) Serve() error {
	return s.httpServer.Serve(s.listener)
}

// Shutdown gracefully stops the server, waiting for in-flight requests to
// finish or ctx to be done.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// NewReportServer builds and binds the mTLS report listener: it requires
// and verifies a client certificate signed by clientCAs (the manager's
// own CA). This is the core security boundary between an enrolled worker
// and an anonymous connection — Go rejects the TLS handshake itself
// before any handler runs if no valid client cert is presented.
func NewReportServer(addr string, serverCert tls.Certificate, clientCAs *x509.CertPool, handler http.Handler) (*Server, error) {
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCAs,
		MinVersion:   tls.VersionTLS12,
	}
	return newTLSServer(addr, tlsConfig, handler)
}

func newTLSServer(addr string, tlsConfig *tls.Config, handler http.Handler) (*Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", addr, err)
	}
	return &Server{
		httpServer: &http.Server{Handler: handler, TLSConfig: tlsConfig},
		listener:   tls.NewListener(ln, tlsConfig),
	}, nil
}
