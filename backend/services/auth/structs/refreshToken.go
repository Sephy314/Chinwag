package structs

type RefreshToken struct {
	Subject      string
	RefreshToken string
	LineageID    string
	ParentHash   string
	Jkt          string
}

type RefreshTokenRecord struct {
	UserID     string
	LineageID  string
	ParentHash string
	Jkt        string
	Used       bool
	Revoked    bool
	CreatedAt  int64
}

type TokenSet struct {
	AccessToken  string
	RefreshToken string
	UserId       string
}
