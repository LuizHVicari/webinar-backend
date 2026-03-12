package config

import "github.com/caarlos0/env/v11"

type Config struct {
	Port            string `env:"PORT"              envDefault:"8080"`
	DatabaseURL     string `env:"DATABASE_URL"      required:"true"`
	RedisURL        string `env:"REDIS_URL"         required:"true"`
	KratosPublicURL string `env:"KRATOS_PUBLIC_URL" required:"true"`
	KratosAdminURL  string `env:"KRATOS_ADMIN_URL"  required:"true"`
	KetoReadURL     string `env:"KETO_READ_URL"     required:"true"`
	KetoWriteURL    string `env:"KETO_WRITE_URL"    required:"true"`
}

func Load() (Config, error) {
	var cfg Config
	return cfg, env.Parse(&cfg)
}
