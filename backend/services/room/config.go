package main

import "os"

type Config struct {
	Port          string
	DBUrl         string
	RedisAddr     string
	RedisPassword string
	AuthServiceURL string
	JWKSURL       string
	FrontendURL   string
}

func LoadConfig() *Config {
	port := os.Getenv("ROOM_PORT")
	if port == "" {
		port = "8082"
	}

	dbUrl := os.Getenv("ROOM_DB_URL")
	if dbUrl == "" {
		dbUrl = "postgres://sephy:ouilala0328@localhost:5432/chinwag_room?sslmode=disable"
	}

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	redisPassword := os.Getenv("REDIS_PW")

	authServiceURL := os.Getenv("AUTH_SERVICE_URL")
	if authServiceURL == "" {
		authServiceURL = "http://localhost:8081"
	}

	jwksURL := os.Getenv("AUTH_JWKS_URL")
	if jwksURL == "" {
		jwksURL = "http://localhost:8081/.well-known/jwks.json"
	}

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	return &Config{
		Port:           port,
		DBUrl:          dbUrl,
		RedisAddr:      redisAddr,
		RedisPassword:  redisPassword,
		AuthServiceURL: authServiceURL,
		JWKSURL:        jwksURL,
		FrontendURL:    frontendURL,
	}
}
