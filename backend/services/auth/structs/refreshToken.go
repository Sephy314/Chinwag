package structs

// RefreshTokenClaims is the result of cryptographically validating a
// refresh-token JWT. It is derived purely from the JWT itself — no Redis state
// is consulted to produce it.
type RefreshTokenClaims struct {
	Subject   string
	JTI       string
	SID       string
	Jkt       string
	IssuedAt  int64
	ExpiresAt int64
}

// RotatedRefreshToken is the outcome of an atomic refresh rotation: the new
// signed refresh-token JWT plus its lineage identity.
type RotatedRefreshToken struct {
	NewToken string
	NewJTI   string
	SID      string
	UserID   string
}

// RefreshTokenRecord is a decoded Redis record keyed by a refresh token's jti.
type RefreshTokenRecord struct {
	UserID    string
	LineageID string
	Jkt       string
	Used      bool
	Revoked   bool
	CreatedAt int64
}

type TokenSet struct {
	AccessToken  string
	RefreshToken string
	UserId       string
}
