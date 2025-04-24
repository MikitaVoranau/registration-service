package redis

import (
	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Host     string `yaml:"host" env:"REDIS_HOST" env-default:"localhost"`
	Port     string `yaml:"port" env:"REDIS_PORT" env-default:"6380"`
	Password string `yaml:"password" env:"REDIS_PASSWORD" env-default:""`
	Db       int    `yaml:"db" env:"REDIS_DB" env-default:"0"`
}

func New(cfg RedisConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.Host + ":" + cfg.Port,
		Password: cfg.Password,
		DB:       cfg.Db,
	})
}
