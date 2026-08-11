package main

import (
	"context"
	"log/slog"
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

	refreshTokenHandler := handler.NewRefreshHandler(refreshTokenService, jwtService, cacheRedis, dpopService, log)
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

	log.Info("auth service starting", "port", cfg.Port)

	if err := r.Echo.Start("0.0.0.0:" + cfg.Port); err != nil {
		log.Fatal("auth service failed to start", "error", err)
	}
}
