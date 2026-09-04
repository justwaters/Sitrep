package transport

import (
	"crypto/tls"
	"net/http"
)

// NewEnrollServer builds and binds the bootstrap enrollment listener:
// server-side TLS authentication only, no client certificate requested or
// required (the worker has none yet — issuing one is exactly what
// enrollment does). It is a physically separate listener/port from the
// report server, so the report handler's mTLS requirement can never be
// bypassed by hitting this endpoint instead.
func NewEnrollServer(addr string, serverCert tls.Certificate, handler http.Handler) (*Server, error) {
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.NoClientCert,
		MinVersion:   tls.VersionTLS12,
	}
	return newTLSServer(addr, tlsConfig, handler)
}
