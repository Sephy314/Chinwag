package main

import "os"

type Config struct {
	Port            string
	DBUrl           string
	RedisAddr       string
	RedisPassword   string
	FrontendURL     string
	InternalPort    string
	InternalTLSCert string
	InternalTLSKey  string
	InternalTLSCA   string
}

func LoadConfig() *Config {
	port := os.Getenv("AUTH_PORT")
	if port == "" {
		port = "8081"
	}

	dbUrl := os.Getenv("AUTH_DB_URL")
	if dbUrl == "" {
		dbUrl = "postgres://sephy:ouilala0328@localhost:5432/chinwag_auth?sslmode=disable"
	}

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	redisPassword := os.Getenv("REDIS_PW")

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	internalPort := os.Getenv("INTERNAL_PORT")
	if internalPort == "" {
		internalPort = "8085"
	}

	return &Config{
		Port:            port,
		DBUrl:           dbUrl,
		RedisAddr:       redisAddr,
		RedisPassword:   redisPassword,
		FrontendURL:     frontendURL,
		InternalPort:    internalPort,
		InternalTLSCert: os.Getenv("INTERNAL_TLS_CERT"),
		InternalTLSKey:  os.Getenv("INTERNAL_TLS_KEY"),
		InternalTLSCA:   os.Getenv("INTERNAL_TLS_CA"),
	}
}
