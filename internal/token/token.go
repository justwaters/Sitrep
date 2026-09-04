// Package token implements the manager's one-time worker enrollment
// tokens: random bearer credentials, single-use, time-limited, held
// in-memory only (consistent with the manager's no-persistent-history
// design — a manager restart invalidates outstanding unused tokens, and
// the fix is simply generating a new one).
package token

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	// Prefix makes tokens recognizable in logs/shell history.
	Prefix = "sitrep_"
	// DefaultTTL is used when Create is called with ttl <= 0.
	DefaultTTL = 15 * time.Minute

	randomBytes = 32
)

var (
	ErrNotFound = errors.New("token not found")
	ErrExpired  = errors.New("token expired")
	ErrUsed     = errors.New("token already used")
)

// Token is a single enrollment credential.
type Token struct {
	Value     string
	CreatedAt time.Time
	ExpiresAt time.Time
	used      bool
}

// Store holds outstanding enrollment tokens.
type Store struct {
	mu     sync.Mutex
	tokens map[string]*Token
}

// NewStore returns an empty token store.
func NewStore() *Store {
	return &Store{tokens: make(map[string]*Token)}
}

// Create generates and stores a new single-use token with the given TTL
// (DefaultTTL if ttl <= 0).
func (s *Store) Create(ttl time.Duration) (*Token, error) {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	buf := make([]byte, randomBytes)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	now := time.Now()
	t := &Token{
		Value:     Prefix + hex.EncodeToString(buf),
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}

	s.mu.Lock()
	s.tokens[t.Value] = t
	s.mu.Unlock()
	return t, nil
}

// Consume atomically validates and marks a token used. A second call with
// the same value returns ErrUsed; an unknown value returns ErrNotFound; a
// value past its ExpiresAt returns ErrExpired.
func (s *Store) Consume(value string) (*Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tokens[value]
	if !ok {
		return nil, ErrNotFound
	}
	if t.used {
		return nil, ErrUsed
	}
	if time.Now().After(t.ExpiresAt) {
		return nil, ErrExpired
	}
	t.used = true
	return t, nil
}
