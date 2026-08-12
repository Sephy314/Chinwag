package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Sephy314/chinwag/backend/services/auth/conn"
	"github.com/Sephy314/chinwag/backend/services/auth/handler"
	authmigrations "github.com/Sephy314/chinwag/backend/services/auth/migrations"
	"github.com/Sephy314/chinwag/backend/services/auth/oauth"
	"github.com/Sephy314/chinwag/backend/services/auth/repo"
	"github.com/Sephy314/chinwag/backend/services/auth/router"
	"github.com/Sephy314/chinwag/backend/services/auth/scheduler"
	"github.com/Sephy314/chinwag/backend/services/auth/service"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/cache"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/logger"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v5"
)

func main() {
	_, src, _, _ := runtime.Caller(0)
	envPath := filepath.Join(filepath.Dir(src), ".env")
	_ = godotenv.Load(envPath)

	slogLog := slog.New(logger.NewHandler())
	log := logger.NewWith(slogLog)
	cfg := LoadConfig()

	if err := authmigrations.RunAll(cfg.DBUrl, log); err != nil {
		log.Fatal("failed to run migrations", "error", err)
	}

	conns, err := conn.NewConnection(&conn.ConnectionConfig{
		DBUrl:         cfg.DBUrl,
		RedisAddr:     cfg.RedisAddr,
		RedisPassword: cfg.RedisPassword,
		Log:           slogLog,
	})
	if err != nil {
		log.Fatal("failed to connect to database", "error", err)
	}
	defer conns.DB.Close()

	cacheRedis := cache.NewRedisCache(conns.Rds)

	userRepo := repo.NewUserRepository(conns.DB)
	jwksRepo := repo.NewJwtRepository(conns.DB)
	auditRepo := repo.NewAuditRepo(conns.DB)
	unitOfWork := repo.NewSQLUnitOfWork(conns.DB)

	jwksService := service.NewJwksService(jwksRepo, log)
	refreshTokenService := service.NewRefreshTokenService(cacheRedis, "refresh:", time.Hour*24*14)
	dpopService := service.NewDPoPService(cacheRedis)
	userService := service.NewUserService(userRepo, jwksService, refreshTokenService, log, unitOfWork)
	auditService := service.NewAuditService(auditRepo)
	jwtService := service.NewJwtService(refreshTokenService, jwksService)

	keyRotationScheduler := scheduler.NewKeyRotationScheduler(jwksService, scheduler.NextMidnight(), log)
	go keyRotationScheduler.Start(context.Background())

	refreshTokenHandler := handler.NewRefreshHandler(refreshTokenService, jwtService, cacheRedis, dpopService, userService, log)
	userHandler := handler.NewUserHandler(userService, log, dpopService)
	jwksHandler := handler.NewJwksHandler(jwksService)

	adminUserHandler := handler.NewAdminUserHandler(userService, refreshTokenService, auditService, log)
	adminSessionHandler := handler.NewAdminSessionHandler(refreshTokenService, auditService, log)
	adminAuditHandler := handler.NewAdminAuditHandler(auditService, log)

	r := router.NewRouter(userHandler, jwksHandler, refreshTokenHandler, jwksService,
		adminUserHandler, adminSessionHandler, adminAuditHandler, log)

	googleCfg := oauth.LoadGoogleConfig()
	r.Setup(&router.RouterConfig{
		Port:               cfg.Port,
		FrontendURL:        cfg.FrontendURL,
		GoogleOAuthEnabled: googleCfg.IsValid(),
		GoogleConfig:       googleCfg,
		Cache:              cacheRedis,
		DPoPValidator:      dpopService.Validator(),
	})
	r.SetupInternal()

	startInternalMTLS(r.InternalEcho, cfg, log)

	log.Info("auth service starting", "port", cfg.Port)

	if err := r.Echo.Start("0.0.0.0:" + cfg.Port); err != nil {
		log.Fatal("auth service failed to start", "error", err)
	}
}

// startInternalMTLS serves the internal endpoints behind mutual TLS: the server
// presents its own certificate and REQUIRES a client certificate signed by the
// configured CA. This authenticates service-to-service callers (room/chat write
// audit events) without a shared secret. It fails closed — if TLS material is
// not configured, the internal listener is simply not started.
//
// buildInternalTLSConfig loads the server key pair and the CA used to verify
// client certificates, requiring and verifying a client cert on every
// connection (mutual TLS). Split out for testability.
func buildInternalTLSConfig(certFile, keyFile, caFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

func startInternalMTLS(e *echo.Echo, cfg *Config, log logger.Logger) {
	if cfg.InternalTLSCert == "" || cfg.InternalTLSKey == "" || cfg.InternalTLSCA == "" {
		log.Warn("internal mTLS not configured (INTERNAL_TLS_CERT/KEY/CA) — internal listener disabled")
		return
	}

	tlsCfg, err := buildInternalTLSConfig(cfg.InternalTLSCert, cfg.InternalTLSKey, cfg.InternalTLSCA)
	if err != nil {
		log.Fatal("failed to configure internal mTLS", "error", err)
	}

	ln, err := net.Listen("tcp", "0.0.0.0:"+cfg.InternalPort)
	if err != nil {
		log.Fatal("failed to listen on internal port", "port", cfg.InternalPort, "error", err)
	}

	srv := &http.Server{
		Handler:   e,
		TLSConfig: tlsCfg,
	}
	log.Info("internal mTLS listener starting", "port", cfg.InternalPort)

	go func() {
		if err := srv.Serve(tls.NewListener(ln, tlsCfg)); err != nil && err != http.ErrServerClosed {
			log.Error("internal mTLS listener failed", "error", err)
		}
	}()
}
