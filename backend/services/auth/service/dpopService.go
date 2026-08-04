package service

import (
	"context"
	"net/http"
	"time"

	"github.com/Sephy314/chinwag/backend/shared/auth/dpop"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/cache"
	"github.com/google/uuid"
)

const dpopNonceTTL = 5 * time.Minute

// DPoPServiceInterface validates DPoP proofs and issues nonces.
type DPoPServiceInterface interface {
	Validate(ctx context.Context, r *http.Request) (*dpop.Proof, string, error)
	Validator() *dpop.Validator
}

// DPoPService delegates proof validation to the shared dpop.Validator so the
// token-issuing endpoints enforce exactly the same rules as the middleware.
type DPoPService struct {
	validator *dpop.Validator
}

func NewDPoPService(c cache.Cache) *DPoPService {
	return &DPoPService{
		validator: &dpop.Validator{
			NonceStore: c,
			IssueNonce: func(ctx context.Context) (string, error) {
				nonce := uuid.Must(uuid.NewV7()).String()
				if err := c.Set(ctx, "dpop:nonce:"+nonce, "1", dpopNonceTTL); err != nil {
					return "", err
				}
				return nonce, nil
			},
		},
	}
}

func (s *DPoPService) Validate(ctx context.Context, r *http.Request) (*dpop.Proof, string, error) {
	return s.validator.Validate(ctx, r)
}

func (s *DPoPService) Validator() *dpop.Validator {
	return s.validator
}
