package postgres

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
)

type Config struct {
	Host     string `env:"POSTGRES_HOST" env-default:"localhost"`
	Port     uint16 `env:"POSTGRES_PORT" env-default:"5433"`
	Username string `env:"POSTGRES_USER" env-default:"users"`
	Password string `env:"POSTGRES_PASSWORD" env-default:"2529"`
	Database string `env:"POSTGRES_DB"   env-default:"users"`
}

func New(config Config) (*pgx.Conn, error) {
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
		config.Username,
		config.Password,
		config.Host,
		config.Port,
		config.Database,
	)
	conn, err := pgx.Connect(context.Background(), connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	return conn, nil
}
