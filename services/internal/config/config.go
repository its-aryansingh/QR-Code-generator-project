package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	AppEnv               string `env:"APP_ENV" envDefault:"local"`
	LogLevel             string `env:"LOG_LEVEL" envDefault:"info"`
	HTTPAddr             string `env:"HTTP_ADDR" envDefault:":8080"`
	DatabaseURL          string `env:"DATABASE_URL,required"`
	RedisURL             string `env:"REDIS_URL" envDefault:"redis://localhost:6379/0"`
	AppBaseURL           string `env:"APP_BASE_URL" envDefault:"http://localhost:3000"`
	APIPublicURL         string `env:"API_PUBLIC_URL" envDefault:"http://localhost:8080"`
	PlatformShortDomain  string `env:"PLATFORM_SHORT_DOMAIN" envDefault:"localhost:8090"`
	WebInternalURL       string `env:"WEB_INTERNAL_URL" envDefault:"http://localhost:3000"`
	RenderURL            string `env:"RENDER_URL" envDefault:"http://localhost:8081"`
	RenderSharedSecret   string `env:"RENDER_SHARED_SECRET" envDefault:"dev-render-secret"`
	JWTEd25519PrivateKey string `env:"JWT_ED25519_PRIVATE_KEY"`
	JWTKeyID             string `env:"JWT_KEY_ID" envDefault:"dev-key-1"`
	AppEncryptionKey     string `env:"APP_ENCRYPTION_KEY"`
	CookieSecure         bool   `env:"COOKIE_SECURE" envDefault:"false"`
	LRUSize              int    `env:"LRU_SIZE" envDefault:"100000"`
	LRUTTL               string `env:"LRU_TTL" envDefault:"30s"`
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}
