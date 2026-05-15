// Package jwt parses JWT payloads without verifying signatures. The signing
// secret lives on the school's server; we only ever read the public claims.
package jwt

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Claims struct {
	Iss string    // internal user ID (e.g. "4599")
	Exp time.Time // expiry
}

// Parse reads a JWT and returns its iss + exp claims. It does NOT verify
// the signature.
func Parse(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, fmt.Errorf("not a JWT (expected 3 parts, got %d)", len(parts))
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, fmt.Errorf("decode payload: %w", err)
	}
	var p struct {
		Iss string `json:"iss"`
		Exp int64  `json:"exp"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return Claims{}, fmt.Errorf("parse payload: %w", err)
	}
	if p.Exp == 0 {
		return Claims{}, fmt.Errorf("no exp claim")
	}
	return Claims{Iss: p.Iss, Exp: time.Unix(p.Exp, 0)}, nil
}
