package main

import "os"

type Config struct {
	Port           string
	DBUrl          string
	AuthServiceURL string
	RoomServiceURL string
	JWKSURL        string
	FrontendURL    string
	NatsURL        string
}

func LoadConfig() *Config {
	port := os.Getenv("CHAT_QUERY_PORT")
	if port == "" {
		port = "8084"
	}

	dbUrl := os.Getenv("CHAT_QUERY_DB_URL")
	if dbUrl == "" {
		dbUrl = "postgres://sephy:ouilala0328@localhost:5432/chinwag_chat_projection?sslmode=disable"
	}

	authServiceURL := os.Getenv("AUTH_SERVICE_URL")
	if authServiceURL == "" {
		authServiceURL = "http://localhost:8081"
	}

	roomServiceURL := os.Getenv("ROOM_SERVICE_URL")
	if roomServiceURL == "" {
		roomServiceURL = "http://localhost:8082"
	}

	jwksURL := os.Getenv("AUTH_JWKS_URL")
	if jwksURL == "" {
		jwksURL = "http://localhost:8081/.well-known/jwks.json"
	}

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	return &Config{
		Port:           port,
		DBUrl:          dbUrl,
		AuthServiceURL: authServiceURL,
		RoomServiceURL: roomServiceURL,
		JWKSURL:        jwksURL,
		FrontendURL:    frontendURL,
		NatsURL:        natsURL,
	}
}
