package overlaytoken

import (
	"testing"
	"time"
)

var testSecret = []byte("test-overlay-secret-0123456789")

func TestSignVerify_RoundTrip(t *testing.T) {
	s := New(testSecret)
	now := time.Now() // exp must be in the future for Verify (real-clock) to pass
	tok, err := s.Sign("host:pod-a", "kid123", time.Hour, now)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	claims, err := s.Verify(tok)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Room != "host:pod-a" {
		t.Errorf("room = %q, want host:pod-a", claims.Room)
	}
	if claims.Scope != ScopeRead {
		t.Errorf("scope = %q, want read", claims.Scope)
	}
	if claims.Kid != "kid123" {
		t.Errorf("kid = %q, want kid123", claims.Kid)
	}
}

func TestVerify_Expired(t *testing.T) {
	s := New(testSecret)
	// Issued 2h ago with a 1h ttl → expired 1h ago relative to real now.
	past := time.Now().Add(-2 * time.Hour)
	tok, err := s.Sign("host:pod-a", "kid", time.Hour, past)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if _, err := s.Verify(tok); err == nil {
		t.Error("expired token should fail verification")
	}
}

func TestVerify_WrongSecret(t *testing.T) {
	tok, err := New(testSecret).Sign("host:pod-a", "kid", time.Hour, time.Now())
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if _, err := New([]byte("a-different-secret")).Verify(tok); err == nil {
		t.Error("token signed with a different secret should fail verification")
	}
}

func TestVerify_Tampered(t *testing.T) {
	s := New(testSecret)
	tok, _ := s.Sign("host:pod-a", "kid", time.Hour, time.Now())
	// Flip the last char of the signature.
	tampered := tok[:len(tok)-1]
	if tok[len(tok)-1] == 'a' {
		tampered += "b"
	} else {
		tampered += "a"
	}
	if _, err := s.Verify(tampered); err == nil {
		t.Error("tampered token should fail verification")
	}
}

func TestSignVerify_NoSecretFailsClosed(t *testing.T) {
	s := New(nil)
	if _, err := s.Sign("host:pod-a", "kid", time.Hour, time.Now()); err != ErrNoSecret {
		t.Errorf("Sign without secret = %v, want ErrNoSecret", err)
	}
	if _, err := s.Verify("whatever"); err != ErrNoSecret {
		t.Errorf("Verify without secret = %v, want ErrNoSecret", err)
	}
}
