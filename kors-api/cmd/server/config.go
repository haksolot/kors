package main

import (
	"os"
	"strconv"
)

type Config struct {
	Port                 string
	DatabaseURL          string
	NatsURL              string
	MinioURL             string
	MinioAccessKey       string
	MinioSecretKey       string
	MinioBucket          string
	ComplexityLimit      int
	GraphQLIntrospection bool
	JWKSEndpoint         string
	AppEnv               string
	OTLPEndpoint         string
	OTLPInsecure         bool
}

func loadConfig() Config {
	cl, _ := strconv.Atoi(getEnv("GRAPHQL_COMPLEXITY_LIMIT", "1000"))
	return Config{
		Port:                 getEnv("PORT", "8080"),
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		NatsURL:              getEnv("NATS_URL", "nats://localhost:4222"),
		MinioURL:             os.Getenv("MINIO_URL"),
		MinioAccessKey:       os.Getenv("MINIO_ACCESS_KEY"),
		MinioSecretKey:       os.Getenv("MINIO_SECRET_KEY"),
		MinioBucket:          getEnv("MINIO_BUCKET", "kors-files"),
		ComplexityLimit:      cl,
		GraphQLIntrospection: os.Getenv("GRAPHQL_INTROSPECTION") == "true",
		JWKSEndpoint:         getEnv("JWKS_ENDPOINT", "http://kors-sso:8080/realms/kors/protocol/openid-connect/certs"),
		AppEnv:               getEnv("APP_ENV", "production"),
		OTLPEndpoint:         os.Getenv("OTLP_ENDPOINT"),
		OTLPInsecure:         os.Getenv("OTLP_INSECURE") == "true",
	}
}
