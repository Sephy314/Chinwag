package structs

type RefreshToken struct {
	Subject      string
	RefreshToken string
}

type TokenSet struct {
	AccessToken  string
	RefreshToken string
}
