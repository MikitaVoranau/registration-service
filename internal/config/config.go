package config

import (
	"errors"
	"github.com/ilyakaznacheev/cleanenv"
	"registration-service/internal/MinIO"
	"registration-service/pkg/database/postgres"
	"registration-service/pkg/database/redis"
)

type AuthConfig struct {
	GRPCPort  string `env:"GRPC_AUTH_PORT" env-default:"50051"`
	JWTSecret string `env:"JWT_TOKEN"`
	Postgres  postgres.Config
	Redis     redis.Config
}

type FileConfig struct {
	GRPCPort        string `env:"GRPC_FILE_PORT" env-default:"50052"`
	AuthServiceAddr string `env:"AUTH_SERVICE_ADDR" env-default:"localhost:50053"`
	Postgres        postgres.Config
	MinIO           MinIO.Config
}

func LoadAuthConfig() (*AuthConfig, error) {
	var cfg AuthConfig
	if err := cleanenv.ReadConfig("./.env", &cfg); err != nil {
		return nil, errors.New("cannot read Auth Config")
	}
	return &cfg, nil
}

func LoadFileConfig() (*FileConfig, error) {
	var cfg FileConfig
	if err := cleanenv.ReadConfig(".env", &cfg); err != nil {
		return nil, errors.New("cannot read File Config")
	}
	return &cfg, nil
}
