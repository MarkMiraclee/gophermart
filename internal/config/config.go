package config

import (
	"flag"
	"github.com/caarlos0/env/v6"
)

const (
	DefaultRunAddress           = "localhost:8080"
	DefaultDatabaseURI          = ""
	DefaultAccrualSystemAddress = ""
	DefaultJWTSecret            = "supersecretkey"
)

type Config struct {
	RunAddress           string `env:"RUN_ADDRESS"`
	DatabaseURI          string `env:"DATABASE_URI"`
	AccrualSystemAddress string `env:"ACCRUAL_SYSTEM_ADDRESS"`
	JWTSecret            string `env:"JWT_SECRET"`
}

func New() (*Config, error) {
	cfg := &Config{}

	flag.StringVar(&cfg.RunAddress, "a", DefaultRunAddress, "server address")
	flag.StringVar(&cfg.DatabaseURI, "d", DefaultDatabaseURI, "database URI")
	flag.StringVar(&cfg.AccrualSystemAddress, "r", DefaultAccrualSystemAddress, "accrual system address")
	flag.StringVar(&cfg.JWTSecret, "j", DefaultJWTSecret, "jwt secret key")
	flag.Parse()

	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
