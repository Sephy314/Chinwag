package service

import (
	"context"
	"net/http"
	"time"

	"github.com/Sephy314/chinwag/backend/shared/auth/dpop"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/cache"
	"github.com/google/uuid"
)

const (
	// DPoPErrorUseNonce signals the client to retry with a fresh nonce (RFC 9449 section 8.3).
	DPoPErrorUseNonce = "use_dpop_nonce"
	// DPoPErrorInvalid signals a malformed or invalid proof.
	DPoPErrorInvalid = "invalid_dpop_proof"

	dpopNonceTTL   = 5 * time.Minute
	dpopProofMaxAge = 60 * time.Second
	dpopFutureSkew  = 60 * time.Second
	dpopJtiTTL      = 2 * time.Minute
)

// DPoPError is a typed DPoP validation failure carrying the RFC 9449 error code.
type DPoPError struct {
	Code    string
	Message string
}

func (e *DPoPError) Error() string { return e.Message }

// DPoPServiceInterface validates DPoP proofs and issues nonces.
type DPoPServiceInterface interface {
	Validate(ctx context.Context, r *http.Request) (*dpop.Proof, string, error)
	IssueNonce(ctx context.Context) (string, error)
}

type DPoPService struct {
	Cache cache.Cache
}

func NewDPoPService(c cache.Cache) *DPoPService {
	return &DPoPService{Cache: c}
}

const consumeNonceScript = `
local v = redis.call('GET', KEYS[1])
if not v then
  return 0
end
redis.call('DEL', KEYS[1])
return 1
`

// IssueNonce creates a single-use nonce and stores it in Redis.
func (s *DPoPService) IssueNonce(ctx context.Context) (string, error) {
	nonce := uuid.Must(uuid.NewV7()).String()
	if err := s.Cache.Set(ctx, "dpop:nonce:"+nonce, "1", dpopNonceTTL); err != nil {
		return "", err
	}
	return nonce, nil
}

// Validate parses and validates a DPoP proof from the request. On success it
// returns the proof and a fresh nonce for the DPoP-Nonce response header. On
// failure it returns a *DPoPError along with a fresh nonce to challenge the
// client.
func (s *DPoPService) Validate(ctx context.Context, r *http.Request) (*dpop.Proof, string, error) {
	raw := r.Header.Get(dpop.HeaderName)
	if raw == "" {
		n, err := s.IssueNonce(ctx)
		if err != nil {
			return nil, "", err
		}
		return nil, n, &DPoPError{Code: DPoPErrorUseNonce, Message: "DPoP proof is required"}
	}

	proof, err := dpop.ParseProof(raw)
	if err != nil {
		n, nerr := s.IssueNonce(ctx)
		if nerr != nil {
			return nil, "", nerr
		}
		return nil, n, &DPoPError{Code: DPoPErrorInvalid, Message: "invalid DPoP proof: " + err.Error()}
	}

	htu := dpop.RequestHTU(r)
	if err := proof.Validate(r.Method, htu, time.Now(), dpopProofMaxAge, dpopFutureSkew); err != nil {
		n, nerr := s.IssueNonce(ctx)
		if nerr != nil {
			return nil, "", nerr
		}
		return nil, n, &DPoPError{Code: DPoPErrorInvalid, Message: err.Error()}
	}

	if err := proof.VerifySignature(); err != nil {
		n, nerr := s.IssueNonce(ctx)
		if nerr != nil {
			return nil, "", nerr
		}
		return nil, n, &DPoPError{Code: DPoPErrorInvalid, Message: "invalid DPoP proof signature"}
	}

	if proof.Claims.Nonce == "" {
		n, err := s.IssueNonce(ctx)
		if err != nil {
			return nil, "", err
		}
		return nil, n, &DPoPError{Code: DPoPErrorUseNonce, Message: "missing DPoP nonce"}
	}

	consumed, err := s.consumeNonce(ctx, proof.Claims.Nonce)
	if err != nil {
		return nil, "", err
	}
	if !consumed {
		n, err := s.IssueNonce(ctx)
		if err != nil {
			return nil, "", err
		}
		return nil, n, &DPoPError{Code: DPoPErrorUseNonce, Message: "invalid or expired DPoP nonce"}
	}

	acquired, err := s.Cache.SetNX(ctx, "dpop:jti:"+proof.Claims.Jti, "1", dpopJtiTTL)
	if err != nil {
		return nil, "", err
	}
	if !acquired {
		n, nerr := s.IssueNonce(ctx)
		if nerr != nil {
			return nil, "", nerr
		}
		return nil, n, &DPoPError{Code: DPoPErrorInvalid, Message: "DPoP proof replay detected"}
	}

	n, err := s.IssueNonce(ctx)
	if err != nil {
		return nil, "", err
	}
	return proof, n, nil
}

func (s *DPoPService) consumeNonce(ctx context.Context, nonce string) (bool, error) {
	res, err := s.Cache.Eval(ctx, consumeNonceScript, []string{"dpop:nonce:" + nonce})
	if err != nil {
		return false, err
	}
	v, ok := res.(int64)
	if !ok {
		return false, nil
	}
	return v == 1, nil
}
