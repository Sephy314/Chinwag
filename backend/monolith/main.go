// @title Chinwag API
// @version 1.0
// @description Chinwag Chat Application API
// @host localhost:8080
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
package main

import (
	"github.com/Sephy314/chinwag/backend/monolith/router"
	"github.com/Sephy314/chinwag/backend/monolith/shared/logger"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	log := logger.New()

	e, err := router.SetUpRouter(log)
	if err != nil {
		log.Fatal("failed to setup router", "error", err)
	}
	err = e.Start("0.0.0.0:8080")
	if err != nil {
		log.Fatal("server failed to start", "error", err)
	}
}

