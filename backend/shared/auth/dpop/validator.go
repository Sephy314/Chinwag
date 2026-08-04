package dpop

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Validation timing parameters shared by every service that enforces DPoP.
const (
	DefaultProofMaxAge = 60 * time.Second
	DefaultFutureSkew  = 60 * time.Second
	DefaultJtiTTL      = 2 * time.Minute
)

// Error carries an RFC 9449 error code and the HTTP status for a rejected
// proof.
type Error struct {
	Code    string
	Status  int
	Message string
}

func (e *Error) Error() string { return e.Message }

func newError(code string, status int, msg string) *Error {
	return &Error{Code: code, Status: status, Message: msg}
}

// Predefined validation failures. Missing proofs/nonces are signalled with the
// use_dpop_nonce code so the client can retry with a fresh nonce (RFC 9449
// section 8.3).
var (
	ErrMissingProof = newError(ErrorUseNonce, http.StatusBadRequest, "DPoP proof is required")
	ErrMissingNonce = newError(ErrorUseNonce, http.StatusBadRequest, "missing DPoP nonce")
	ErrInvalidNonce = newError(ErrorUseNonce, http.StatusBadRequest, "invalid or expired DPoP nonce")
	ErrReplay       = newError(ErrorInvalidProof, http.StatusBadRequest, "DPoP proof replay detected")
)

// NonceStore provides single-use nonces and jti replay detection. Implemented
// by each service over its Redis client, keeping the shared package free of
// storage-library dependencies.
type NonceStore interface {
	// ConsumeNonce atomically deletes the single-use nonce and reports whether
	// it existed (false => missing, reused, or expired).
	ConsumeNonce(ctx context.Context, nonce string) (bool, error)
	// ReserveJti atomically records the proof jti for the given TTL and reports
	// whether it was a brand-new value (false => replay detected).
	ReserveJti(ctx context.Context, jti string, ttl time.Duration) (bool, error)
}

// Validator runs the complete RFC 9449 proof validation sequence shared by the
// protected-route middleware and the token-issuing endpoints, so behaviour is
// consistent everywhere.
type Validator struct {
	// NonceStore provides single-use nonces and jti replay detection. When nil,
	// nonce and replay checks are skipped (not for production).
	NonceStore NonceStore
	// IssueNonce creates and persists a fresh single-use nonce.
	IssueNonce func(ctx context.Context) (string, error)

	ProofMaxAge time.Duration
	FutureSkew  time.Duration
	JtiTTL      time.Duration
}

func (v *Validator) setDefaults() {
	if v.ProofMaxAge <= 0 {
		v.ProofMaxAge = DefaultProofMaxAge
	}
	if v.FutureSkew <= 0 {
		v.FutureSkew = DefaultFutureSkew
	}
	if v.JtiTTL <= 0 {
		v.JtiTTL = DefaultJtiTTL
	}
}

// Validate parses and validates the DPoP proof carried by r. On success it
// returns the proof and a fresh nonce to send back in the DPoP-Nonce response
// header. On failure it returns a *Error (along with a fresh nonce to challenge
// the client), or a plain error if the storage backend is unavailable.
func (v *Validator) Validate(ctx context.Context, r *http.Request) (*Proof, string, error) {
	v.setDefaults()

	issue := func() (string, error) {
		if v.IssueNonce == nil {
			return "", errors.New("no nonce issuer configured")
		}
		return v.IssueNonce(ctx)
	}
	issueOrEmpty := func() string {
		n, _ := issue()
		return n
	}

	raw := r.Header.Get(HeaderName)
	if raw == "" {
		return nil, issueOrEmpty(), ErrMissingProof
	}

	proof, err := ParseProof(raw)
	if err != nil {
		return nil, issueOrEmpty(), newError(ErrorInvalidProof, http.StatusBadRequest, "invalid DPoP proof: "+err.Error())
	}

	htu := RequestHTU(r)
	if err := proof.Validate(r.Method, htu, time.Now(), v.ProofMaxAge, v.FutureSkew); err != nil {
		return nil, issueOrEmpty(), newError(ErrorInvalidProof, http.StatusBadRequest, err.Error())
	}

	if err := proof.VerifySignature(); err != nil {
		return nil, issueOrEmpty(), newError(ErrorInvalidProof, http.StatusBadRequest, "invalid DPoP proof signature")
	}

	if v.NonceStore != nil {
		if proof.Claims.Nonce == "" {
			return nil, issueOrEmpty(), ErrMissingNonce
		}

		consumed, err := v.NonceStore.ConsumeNonce(ctx, proof.Claims.Nonce)
		if err != nil {
			return nil, "", err
		}
		if !consumed {
			return nil, issueOrEmpty(), ErrInvalidNonce
		}

		acquired, err := v.NonceStore.ReserveJti(ctx, proof.Claims.Jti, v.JtiTTL)
		if err != nil {
			return nil, "", err
		}
		if !acquired {
			return nil, issueOrEmpty(), ErrReplay
		}
	}

	nonce, err := issue()
	if err != nil {
		return nil, "", err
	}
	return proof, nonce, nil
}