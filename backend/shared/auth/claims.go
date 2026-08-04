package auth

import (
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	jwt.RegisteredClaims
	Role string `json:"role"`
	CNF  *CNF   `json:"cnf,omitempty"`
}

// CNF carries the confirmation key (RFC 9449 section 3) that binds the access
// token to the DPoP proof key.
type CNF struct {
	Jkt string `json:"jkt,omitempty"`
}
