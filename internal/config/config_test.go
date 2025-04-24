// internal/config/config_test.go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"registration-service/internal/config"

	"github.com/stretchr/testify/assert"
)

func TestNew_Success(t *testing.T) {
	// Создаём временную папку и в ней структуру config/local.env
	td := t.TempDir()
	cfgDir := filepath.Join(td, "config")
	if err := os.Mkdir(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Записываем тестовый local.env
	envContent := `POSTGRES_HOST=localhost
POSTGRES_PORT=5433
POSTGRES_USER=users
POSTGRES_PASSWORD=2529
POSTGRES_DB=users

JWT_TOKEN=very_very_secret_key

GRPC_SERVER_PORT=50051

REDIS_HOST=localhost
REDIS_PORT=6380
REDIS_PASSWORD=
REDIS_DB=0
`
	if err := os.WriteFile(filepath.Join(cfgDir, "local.env"), []byte(envContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Переключаем рабочую директорию на td, чтобы config.New() увидел ./config/local.env
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	if err := os.Chdir(td); err != nil {
		t.Fatal(err)
	}

	// Вызываем
	cfg, err := config.New()
	assert.NoError(t, err)

	// Проверяем поля Postgres
	assert.Equal(t, "localhost", cfg.Postgres.Host)
	assert.Equal(t, uint16(5433), cfg.Postgres.Port)
	assert.Equal(t, "users", cfg.Postgres.Username)
	assert.Equal(t, "2529", cfg.Postgres.Password)
	assert.Equal(t, "users", cfg.Postgres.Database)

	// Проверяем JWT и GRPC порт
	assert.Equal(t, "very_very_secret_key", cfg.JWTSecret)
	assert.Equal(t, "50051", cfg.GRPCPort)

	// Проверяем Redis — Port теперь строка
	assert.Equal(t, "localhost", cfg.Redis.Host)
	assert.Equal(t, "6380", cfg.Redis.Port)
	assert.Equal(t, "", cfg.Redis.Password)
	assert.Equal(t, 0, cfg.Redis.Db)
}

func TestNew_FileNotFound(t *testing.T) {
	// Пустая временная папка без config/local.env
	td := t.TempDir()
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	if err := os.Chdir(td); err != nil {
		t.Fatal(err)
	}

	_, err := config.New()
	assert.Error(t, err)
}
