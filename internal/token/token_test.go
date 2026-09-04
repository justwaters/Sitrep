package token

import (
	"testing"
	"time"
)

func TestCreateConsume(t *testing.T) {
	s := NewStore()
	tok, err := s.Create(0)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tok.Value == "" {
		t.Fatal("expected non-empty token value")
	}

	if _, err := s.Consume(tok.Value); err != nil {
		t.Fatalf("first Consume: %v", err)
	}

	if _, err := s.Consume(tok.Value); err != ErrUsed {
		t.Fatalf("second Consume: got %v, want ErrUsed", err)
	}
}

func TestConsumeUnknown(t *testing.T) {
	s := NewStore()
	if _, err := s.Consume("sitrep_doesnotexist"); err != ErrNotFound {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestConsumeExpired(t *testing.T) {
	s := NewStore()
	tok, err := s.Create(time.Millisecond)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if _, err := s.Consume(tok.Value); err != ErrExpired {
		t.Fatalf("got %v, want ErrExpired", err)
	}
}

func TestCreateDefaultTTL(t *testing.T) {
	s := NewStore()
	before := time.Now()
	tok, err := s.Create(0)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	wantExpiry := before.Add(DefaultTTL)
	if tok.ExpiresAt.Before(wantExpiry.Add(-time.Second)) || tok.ExpiresAt.After(wantExpiry.Add(time.Second)) {
		t.Errorf("ExpiresAt = %v, want close to %v", tok.ExpiresAt, wantExpiry)
	}
}
