package config

import (
	"fmt"
	"github.com/ilyakaznacheev/cleanenv"
	"registration-service/pkg/database/postgres"
	"registration-service/pkg/database/redis"
)

type Config struct {
	Postgres  postgres.Config
	Redis     redis.RedisConfig
	GRPCPort  string `env:"GRPC_SERVER_PORT" env-default:"50051"`
	JWTSecret string `env:"JWT_TOKEN"`
}

func New() (*Config, error) {
	var cfg Config
	if err := cleanenv.ReadConfig("./config/local.env", &cfg); err != nil {
		return nil, fmt.Errorf("error reading config: %v", err)
	}
	return &cfg, nil
}
