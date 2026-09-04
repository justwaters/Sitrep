// Package api implements the manager's local JSON query API: a plain
// HTTP+JSON REST interface bound strictly to loopback so other software
// on the same machine can pull worker status data, with no client
// certificate required.
package api

import "github.com/justwaters/sitrep/internal/transport"

// WorkerSummary is the list-view shape for GET /v1/workers.
type WorkerSummary struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	EnrolledAt int64  `json:"enrolled_at"`
	LastSeen   int64  `json:"last_seen"`
}

// WorkerDetail is the shape for GET /v1/workers/{id}, adding the worker's
// most recent report.
type WorkerDetail struct {
	WorkerSummary
	LastReport *transport.Report `json:"last_report,omitempty"`
}

// ConfigResponse is the shape for GET /v1/config and the response to a
// successful PATCH /v1/config.
type ConfigResponse struct {
	Name            string   `json:"name"`
	ListenAddr      string   `json:"listen_addr"`
	APIListenAddr   string   `json:"api_listen_addr"`
	IntervalSeconds int      `json:"interval_seconds"`
	EnabledChecks   []string `json:"enabled_checks"`
}

// ConfigPatch is the request body for PATCH /v1/config. Fields left nil
// are left unchanged.
type ConfigPatch struct {
	IntervalSeconds *int      `json:"interval_seconds,omitempty"`
	EnabledChecks   *[]string `json:"enabled_checks,omitempty"`
}

// TokenCreateRequest is the request body for POST /v1/tokens.
type TokenCreateRequest struct {
	// TTLSeconds, if zero, falls back to token.DefaultTTL.
	TTLSeconds int `json:"ttl_seconds,omitempty"`
}

// TokenCreateResponse carries everything `sitrep manager token create`
// prints for the operator to hand to the worker wizard: the token, its
// expiry, where the worker should dial for enrollment, and the manager
// server certificate fingerprint the worker pins for that one bootstrap
// connection.
type TokenCreateResponse struct {
	Token                 string `json:"token"`
	ExpiresAt             int64  `json:"expires_at"`
	EnrollAddr            string `json:"enroll_addr"`
	ServerCertFingerprint string `json:"server_cert_fingerprint"`
	ManagerName           string `json:"manager_name"`
}
