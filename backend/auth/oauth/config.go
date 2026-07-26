package oauth

import "os"

type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

func LoadGoogleConfig() *GoogleConfig {
	return &GoogleConfig{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
	}
}

func (c *GoogleConfig) IsValid() bool {
	return c.ClientID != "" && c.ClientSecret != "" && c.RedirectURL != ""
}
