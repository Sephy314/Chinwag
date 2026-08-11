package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/Sephy314/chinwag/backend/services/auth/domain"
	"github.com/Sephy314/chinwag/backend/services/auth/internal/jwt"
	"github.com/Sephy314/chinwag/backend/services/auth/service"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/cache"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/logger"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/response"
	"github.com/Sephy314/chinwag/backend/services/auth/structs"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

var (
	googleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"
	googleUserURL  = "https://www.googleapis.com/oauth2/v2/userinfo"
)

func SetGoogleTokenURL(url string) { googleTokenURL = url }
func SetGoogleUserURL(url string)  { googleUserURL = url }
func ResetGoogleURLs() {
	googleTokenURL = "https://oauth2.googleapis.com/token"
	googleUserURL = "https://www.googleapis.com/oauth2/v2/userinfo"
}

type googleUserInfo struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

type GoogleOAuthHandler struct {
	Config         *GoogleConfig
	UserService    *service.UserService
	JwkService     service.JwksServiceInterface
	RefreshService service.RefreshTokenServiceInterface
	Cache          cache.Cache
	FrontendURL    string
	log            logger.Logger
}

func NewGoogleOAuthHandler(
	cfg *GoogleConfig,
	userSvc *service.UserService,
	jwkSvc service.JwksServiceInterface,
	refreshSvc service.RefreshTokenServiceInterface,
	cache cache.Cache,
	frontendURL string,
	log logger.Logger,
) *GoogleOAuthHandler {
	return &GoogleOAuthHandler{
		Config:         cfg,
		UserService:    userSvc,
		JwkService:     jwkSvc,
		RefreshService: refreshSvc,
		Cache:          cache,
		FrontendURL:    frontendURL,
		log:            log,
	}
}

const oauthStateTTL = 10 * time.Minute

// isValidJkt reports whether s looks like a valid RFC 7638 SHA-256
// thumbprint (base64url, 43 characters). The jkt must always be present so
// OAuth-issued tokens are always bound to a DPoP key.
func isValidJkt(s string) bool {
	if len(s) != 43 {
		return false
	}
	for _, r := range s {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

const consumeStateScript = `
local v = redis.call('GET', KEYS[1])
if not v then
  return ''
end
redis.call('DEL', KEYS[1])
return v
`

func (h *GoogleOAuthHandler) HandleRedirect(c *echo.Context) error {
	jkt := c.QueryParam("jkt")

	if !isValidJkt(jkt) {
		h.log.Warn("oauth redirect rejected: missing invalid dpop key binding", "jkt_set", jkt != "")
		return c.Redirect(http.StatusTemporaryRedirect, h.FrontendURL+"/login?error=oauth_binding_required")
	}

	state := uuid.Must(uuid.NewV7()).String()
	if err := h.Cache.Set(c.Request().Context(), "oauth:state:"+state, jkt, oauthStateTTL); err != nil {
		h.log.Error("oauth: failed to store state binding", "error", err)
		return c.JSON(http.StatusInternalServerError, response.Error("failed to start oauth"))
	}

	h.log.Info("oauth redirect initiated", "state", state, "dpop_bound", true)

	params := url.Values{
		"client_id":     {h.Config.ClientID},
		"redirect_uri":  {h.Config.RedirectURL},
		"response_type": {"code"},
		"scope":         {"openid email profile"},
		"state":         {state},
		"access_type":   {"offline"},
		"prompt":        {"consent"},
	}

	return c.Redirect(http.StatusTemporaryRedirect, googleAuthURL+"?"+params.Encode())
}

func (h *GoogleOAuthHandler) HandleCallback(c *echo.Context) error {
	code := c.QueryParam("code")
	state := c.QueryParam("state")
	errParam := c.QueryParam("error")

	h.log.Info("oauth callback received", "state", state)

	if errParam != "" {
		h.log.Warn("oauth: user denied access", "error", errParam)
		return c.Redirect(http.StatusTemporaryRedirect, h.FrontendURL+"/login?error=oauth_denied")
	}

	if code == "" || state == "" {
		h.log.Warn("oauth: missing code or state", "code_empty", code == "", "state_empty", state == "")
		return c.JSON(http.StatusBadRequest, response.Error("missing code or state"))
	}

	tokenResp, err := h.exchangeCode(code)
	if err != nil {
		h.log.Error("oauth: token exchange failed", "error", err)
		return c.Redirect(http.StatusTemporaryRedirect, h.FrontendURL+"/login?error=oauth_exchange_failed")
	}

	userInfo, err := h.fetchUserInfo(tokenResp.AccessToken)
	if err != nil {
		h.log.Error("oauth: fetch user info failed", "error", err)
		return c.Redirect(http.StatusTemporaryRedirect, h.FrontendURL+"/login?error=oauth_userinfo_failed")
	}

	h.log.Info("oauth: user info fetched", "email", userInfo.Email)

	ctx := c.Request().Context()
	jkt := h.consumeState(ctx, state)

	if !isValidJkt(jkt) {
		h.log.Warn("oauth callback rejected: state binding missing or expired", "state", state)
		return c.Redirect(http.StatusTemporaryRedirect, h.FrontendURL+"/login?error=oauth_binding_lost")
	}

	tokens, err := h.findOrCreateUser(ctx, userInfo, jkt)
	if err != nil {
		h.log.Error("oauth: find or create user failed", "email", userInfo.Email, "error", err)
		return c.Redirect(http.StatusTemporaryRedirect, h.FrontendURL+"/login?error=oauth_user_failed")
	}

	c.SetCookie(&http.Cookie{
		Name:     "refresh",
		Value:    tokens.RefreshToken,
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(time.Hour * 24 * 7),
	})

	h.log.Info("oauth: login successful", "email", userInfo.Email, "user_id", tokens.UserId)
	redirectURL := fmt.Sprintf("%s/oauth/callback?token=%s", h.FrontendURL, tokens.AccessToken)
	return c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

type googleTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

func (h *GoogleOAuthHandler) exchangeCode(code string) (*googleTokenResponse, error) {
	data := url.Values{
		"code":          {code},
		"client_id":     {h.Config.ClientID},
		"client_secret": {h.Config.ClientSecret},
		"redirect_uri":  {h.Config.RedirectURL},
		"grant_type":    {"authorization_code"},
	}

	resp, err := http.PostForm(googleTokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp googleTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &tokenResp, nil
}

func (h *GoogleOAuthHandler) fetchUserInfo(accessToken string) (*googleUserInfo, error) {
	req, err := http.NewRequest("GET", googleUserURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo returned %d: %s", resp.StatusCode, string(body))
	}

	var userInfo googleUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}

func (h *GoogleOAuthHandler) consumeState(ctx context.Context, state string) string {
	if state == "" {
		return ""
	}

	res, err := h.Cache.Eval(ctx, consumeStateScript, []string{"oauth:state:" + state})
	if err != nil {
		h.log.Error("oauth: failed to consume state", "error", err)
		return ""
	}

	s, _ := res.(string)
	return s
}

func (h *GoogleOAuthHandler) findOrCreateUser(ctx context.Context, info *googleUserInfo, jkt string) (*structs.TokenSet, error) {
	user, err := h.UserService.GetUserByEmail(ctx, info.Email)
	if err != nil {
		user, err = h.createUserFromGoogle(ctx, info)
		if err != nil {
			return nil, err
		}
	}

	key, err := h.JwkService.GetActiveKey(ctx)
	if err != nil {
		return nil, err
	}

	accessToken, err := jwt.SignWithCNF(user.Id, string(user.Role), key.PrivateKey, key.Kid, jkt)
	if err != nil {
		return nil, err
	}

	refreshToken := uuid.Must(uuid.NewV7()).String()
	err = h.RefreshService.InsertRefreshToken(ctx, structs.RefreshToken{
		Subject:      user.Id,
		RefreshToken: refreshToken,
		Jkt:          jkt,
	})
	if err != nil {
		return nil, err
	}

	return &structs.TokenSet{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserId:       user.Id,
	}, nil
}

func (h *GoogleOAuthHandler) createUserFromGoogle(ctx context.Context, info *googleUserInfo) (*domain.User, error) {
	id := uuid.Must(uuid.NewV7()).String()
	providerID := info.ID

	newUser := &domain.User{
		Id:         id,
		Name:       info.Name,
		Email:      info.Email,
		Password:   "",
		Role:       domain.USER,
		Provider:   "google",
		ProviderID: &providerID,
	}

	err := h.UserService.CreateOAuthUser(ctx, *newUser)
	if err != nil {
		return nil, err
	}

	return newUser, nil
}
