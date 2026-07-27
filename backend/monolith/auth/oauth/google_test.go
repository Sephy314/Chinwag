package oauth_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Sephy314/chinwag/backend/monolith/auth/domain"
	"github.com/Sephy314/chinwag/backend/monolith/auth/mocked"
	"github.com/Sephy314/chinwag/backend/monolith/auth/oauth"
	"github.com/Sephy314/chinwag/backend/monolith/auth/service"
	"github.com/Sephy314/chinwag/backend/monolith/shared/logger"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newSigningKey(t *testing.T) *domain.SigningKey {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return &domain.SigningKey{
		Kid:        "test-kid",
		PublicKey:  &priv.PublicKey,
		PrivateKey: priv,
	}
}

func setupHandler(t *testing.T) (*oauth.GoogleOAuthHandler, *mocked.UserRepo, *mocked.JwkService, *mocked.RefreshTokenService) {
	t.Helper()
	userRepo := &mocked.UserRepo{}
	jwkSvc := &mocked.JwkService{}
	refreshSvc := &mocked.RefreshTokenService{}

	svc := &service.UserService{
		Repo:           userRepo,
		JwkService:     jwkSvc,
		RefreshService: refreshSvc,
	}

	cfg := &oauth.GoogleConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost:8080/auth/google/callback",
	}

	h := oauth.NewGoogleOAuthHandler(cfg, svc, jwkSvc, refreshSvc, "http://localhost:3000", logger.New())
	return h, userRepo, jwkSvc, refreshSvc
}

func TestHandleRedirect_Success(t *testing.T) {
	h, _, _, _ := setupHandler(t)

	rec := echotest.ContextConfig{}.ServeWithHandler(t, h.HandleRedirect)

	require.Equal(t, http.StatusTemporaryRedirect, rec.Code)

	loc := rec.Header().Get("Location")
	require.Contains(t, loc, "accounts.google.com")
	require.Contains(t, loc, "client_id=test-client-id")
	require.Contains(t, loc, "redirect_uri=")
	require.Contains(t, loc, "response_type=code")
	require.Contains(t, loc, "scope=")
	require.Contains(t, loc, "state=")
}

func TestHandleCallback_ErrorParam(t *testing.T) {
	h, _, _, _ := setupHandler(t)

	rec := echotest.ContextConfig{
		QueryValues: url.Values{
			"error": {"access_denied"},
		},
	}.ServeWithHandler(t, h.HandleCallback)

	require.Equal(t, http.StatusTemporaryRedirect, rec.Code)
	loc := rec.Header().Get("Location")
	require.Equal(t, "http://localhost:3000/login?error=oauth_denied", loc)
}

func TestHandleCallback_MissingCode(t *testing.T) {
	h, _, _, _ := setupHandler(t)

	rec := echotest.ContextConfig{
		QueryValues: url.Values{
			"state": {"some-state"},
		},
	}.ServeWithHandler(t, h.HandleCallback)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleCallback_MissingState(t *testing.T) {
	h, _, _, _ := setupHandler(t)

	rec := echotest.ContextConfig{
		QueryValues: url.Values{
			"code": {"some-code"},
		},
	}.ServeWithHandler(t, h.HandleCallback)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleCallback_TokenExchangeFailure(t *testing.T) {
	h, _, _, _ := setupHandler(t)

	failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error": "invalid_grant"}`)
	}))
	defer failServer.Close()

	oauth.SetGoogleTokenURL(failServer.URL)
	oauth.SetGoogleUserURL("http://should-not-reach")
	defer oauth.ResetGoogleURLs()

	rec := echotest.ContextConfig{
		QueryValues: url.Values{
			"code":  {"bad-code"},
			"state": {"some-state"},
		},
	}.ServeWithHandler(t, h.HandleCallback)

	require.Equal(t, http.StatusTemporaryRedirect, rec.Code)
	loc := rec.Header().Get("Location")
	require.Equal(t, "http://localhost:3000/login?error=oauth_exchange_failed", loc)
}

func TestHandleCallback_UserInfoFailure(t *testing.T) {
	h, _, _, _ := setupHandler(t)

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"fake-token","token_type":"Bearer","expires_in":3600}`)
	}))
	defer tokenServer.Close()

	failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error": "internal"}`)
	}))
	defer failServer.Close()

	oauth.SetGoogleTokenURL(tokenServer.URL)
	oauth.SetGoogleUserURL(failServer.URL)
	defer oauth.ResetGoogleURLs()

	rec := echotest.ContextConfig{
		QueryValues: url.Values{
			"code":  {"valid-code"},
			"state": {"some-state"},
		},
	}.ServeWithHandler(t, h.HandleCallback)

	require.Equal(t, http.StatusTemporaryRedirect, rec.Code)
	loc := rec.Header().Get("Location")
	require.Equal(t, "http://localhost:3000/login?error=oauth_userinfo_failed", loc)
}

func TestHandleCallback_ExistingUser_Success(t *testing.T) {
	h, userRepo, jwkSvc, refreshSvc := setupHandler(t)

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"google-access-token","token_type":"Bearer","expires_in":3600}`)
	}))
	defer tokenServer.Close()

	userInfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"google-123","email":"user@example.com","name":"Test User","picture":""}`)
	}))
	defer userInfoServer.Close()

	oauth.SetGoogleTokenURL(tokenServer.URL)
	oauth.SetGoogleUserURL(userInfoServer.URL)
	defer oauth.ResetGoogleURLs()

	existingUser := &domain.User{
		Id:       "existing-uid",
		Name:     "Test User",
		Email:    "user@example.com",
		Provider: "google",
	}

	userRepo.On("GetUserByEmail", mock.Anything, "user@example.com").Return(existingUser, nil)

	sk := newSigningKey(t)
	jwkSvc.On("GetActiveKey", mock.Anything).Return(sk, nil)
	refreshSvc.On("InsertRefreshToken", mock.Anything, mock.Anything).Return(nil)

	rec := echotest.ContextConfig{
		QueryValues: url.Values{
			"code":  {"valid-code"},
			"state": {"some-state"},
		},
	}.ServeWithHandler(t, h.HandleCallback)

	require.Equal(t, http.StatusTemporaryRedirect, rec.Code)
	loc := rec.Header().Get("Location")
	require.Contains(t, loc, "http://localhost:3000/oauth/callback?token=")

	cookies := rec.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refresh" {
			refreshCookie = c
			break
		}
	}
	require.NotNil(t, refreshCookie)
	require.Equal(t, "/auth", refreshCookie.Path)
	require.True(t, refreshCookie.HttpOnly)

	userRepo.AssertExpectations(t)
	jwkSvc.AssertExpectations(t)
	refreshSvc.AssertExpectations(t)
}

func TestHandleCallback_NewUser_Success(t *testing.T) {
	h, userRepo, jwkSvc, refreshSvc := setupHandler(t)

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"google-token","token_type":"Bearer","expires_in":3600}`)
	}))
	defer tokenServer.Close()

	userInfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"google-new-456","email":"newuser@example.com","name":"New User","picture":""}`)
	}))
	defer userInfoServer.Close()

	oauth.SetGoogleTokenURL(tokenServer.URL)
	oauth.SetGoogleUserURL(userInfoServer.URL)
	defer oauth.ResetGoogleURLs()

	userRepo.On("GetUserByEmail", mock.Anything, "newuser@example.com").Return((*domain.User)(nil), fmt.Errorf("not found"))
	userRepo.On("CreateOAuthUser", mock.Anything, mock.AnythingOfType("domain.User")).Return(nil)

	sk := newSigningKey(t)
	jwkSvc.On("GetActiveKey", mock.Anything).Return(sk, nil)
	refreshSvc.On("InsertRefreshToken", mock.Anything, mock.Anything).Return(nil)

	rec := echotest.ContextConfig{
		QueryValues: url.Values{
			"code":  {"valid-code"},
			"state": {"some-state"},
		},
	}.ServeWithHandler(t, h.HandleCallback)

	require.Equal(t, http.StatusTemporaryRedirect, rec.Code)
	loc := rec.Header().Get("Location")
	require.Contains(t, loc, "http://localhost:3000/oauth/callback?token=")

	userRepo.AssertExpectations(t)
	jwkSvc.AssertExpectations(t)
	refreshSvc.AssertExpectations(t)

	userRepo.AssertCalled(t, "CreateOAuthUser", mock.Anything, mock.MatchedBy(func(u domain.User) bool {
		return u.Email == "newuser@example.com" && u.Provider == "google" && u.ProviderID != nil && *u.ProviderID == "google-new-456"
	}))
}

func TestHandleCallback_CreateOAuthUserFailure(t *testing.T) {
	h, userRepo, _, refreshSvc := setupHandler(t)

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"google-token","token_type":"Bearer","expires_in":3600}`)
	}))
	defer tokenServer.Close()

	userInfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"google-789","email":"fail@example.com","name":"Fail User","picture":""}`)
	}))
	defer userInfoServer.Close()

	oauth.SetGoogleTokenURL(tokenServer.URL)
	oauth.SetGoogleUserURL(userInfoServer.URL)
	defer oauth.ResetGoogleURLs()

	userRepo.On("GetUserByEmail", mock.Anything, "fail@example.com").Return((*domain.User)(nil), fmt.Errorf("not found"))
	userRepo.On("CreateOAuthUser", mock.Anything, mock.AnythingOfType("domain.User")).Return(fmt.Errorf("db error"))

	rec := echotest.ContextConfig{
		QueryValues: url.Values{
			"code":  {"valid-code"},
			"state": {"some-state"},
		},
	}.ServeWithHandler(t, h.HandleCallback)

	require.Equal(t, http.StatusTemporaryRedirect, rec.Code)
	loc := rec.Header().Get("Location")
	require.Equal(t, "http://localhost:3000/login?error=oauth_user_failed", loc)

	refreshSvc.AssertNotCalled(t, "InsertRefreshToken", mock.Anything, mock.Anything)
}

func TestHandleCallback_JwkServiceFailure(t *testing.T) {
	h, userRepo, jwkSvc, refreshSvc := setupHandler(t)

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"google-token","token_type":"Bearer","expires_in":3600}`)
	}))
	defer tokenServer.Close()

	userInfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"google-jwk-fail","email":"jwkfail@example.com","name":"JWK Fail","picture":""}`)
	}))
	defer userInfoServer.Close()

	oauth.SetGoogleTokenURL(tokenServer.URL)
	oauth.SetGoogleUserURL(userInfoServer.URL)
	defer oauth.ResetGoogleURLs()

	existingUser := &domain.User{
		Id:    "uid-jwk-fail",
		Name:  "JWK Fail",
		Email: "jwkfail@example.com",
	}

	userRepo.On("GetUserByEmail", mock.Anything, "jwkfail@example.com").Return(existingUser, nil)
	jwkSvc.On("GetActiveKey", mock.Anything).Return((*domain.SigningKey)(nil), fmt.Errorf("no active key"))

	rec := echotest.ContextConfig{
		QueryValues: url.Values{
			"code":  {"valid-code"},
			"state": {"some-state"},
		},
	}.ServeWithHandler(t, h.HandleCallback)

	require.Equal(t, http.StatusTemporaryRedirect, rec.Code)
	loc := rec.Header().Get("Location")
	require.Equal(t, "http://localhost:3000/login?error=oauth_user_failed", loc)

	refreshSvc.AssertNotCalled(t, "InsertRefreshToken", mock.Anything, mock.Anything)
}

func TestHandleCallback_RefreshTokenInsertFailure(t *testing.T) {
	h, userRepo, jwkSvc, refreshSvc := setupHandler(t)

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"google-token","token_type":"Bearer","expires_in":3600}`)
	}))
	defer tokenServer.Close()

	userInfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"google-refresh-fail","email":"refreshfail@example.com","name":"Refresh Fail","picture":""}`)
	}))
	defer userInfoServer.Close()

	oauth.SetGoogleTokenURL(tokenServer.URL)
	oauth.SetGoogleUserURL(userInfoServer.URL)
	defer oauth.ResetGoogleURLs()

	existingUser := &domain.User{
		Id:    "uid-refresh-fail",
		Name:  "Refresh Fail",
		Email: "refreshfail@example.com",
	}

	userRepo.On("GetUserByEmail", mock.Anything, "refreshfail@example.com").Return(existingUser, nil)

	sk := newSigningKey(t)
	jwkSvc.On("GetActiveKey", mock.Anything).Return(sk, nil)
	refreshSvc.On("InsertRefreshToken", mock.Anything, mock.Anything).Return(fmt.Errorf("redis error"))

	rec := echotest.ContextConfig{
		QueryValues: url.Values{
			"code":  {"valid-code"},
			"state": {"some-state"},
		},
	}.ServeWithHandler(t, h.HandleCallback)

	require.Equal(t, http.StatusTemporaryRedirect, rec.Code)
	loc := rec.Header().Get("Location")
	require.Equal(t, "http://localhost:3000/login?error=oauth_user_failed", loc)

	refreshSvc.AssertExpectations(t)
}
