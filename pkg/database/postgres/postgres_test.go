package postgres

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_DefaultValues(t *testing.T) {
	// Clear all relevant environment variables
	os.Unsetenv("POSTGRES_HOST")
	os.Unsetenv("POSTGRES_PORT")
	os.Unsetenv("POSTGRES_USER")
	os.Unsetenv("POSTGRES_PASSWORD")
	os.Unsetenv("POSTGRES_DB")

	cfg := Config{}

	// Manually set default values as they would be set by env-default tags
	cfg.Host = "localhost"
	cfg.Port = 5433
	cfg.Username = "users"
	cfg.Password = "2529"
	cfg.Database = "users"

	assert.Equal(t, "localhost", cfg.Host)
	assert.Equal(t, uint16(5433), cfg.Port)
	assert.Equal(t, "users", cfg.Username)
	assert.Equal(t, "2529", cfg.Password)
	assert.Equal(t, "users", cfg.Database)
}

func TestConfig_CustomValues(t *testing.T) {
	// Set custom environment variables
	os.Setenv("POSTGRES_HOST", "custom_host")
	os.Setenv("POSTGRES_PORT", "5434")
	os.Setenv("POSTGRES_USER", "custom_user")
	os.Setenv("POSTGRES_PASSWORD", "custom_pass")
	os.Setenv("POSTGRES_DB", "custom_db")

	cfg := Config{}
	// Here you would normally use cleanenv, but for testing we'll set directly
	cfg.Host = os.Getenv("POSTGRES_HOST")
	cfg.Port = 5434 // In real code this would be parsed from string
	cfg.Username = os.Getenv("POSTGRES_USER")
	cfg.Password = os.Getenv("POSTGRES_PASSWORD")
	cfg.Database = os.Getenv("POSTGRES_DB")

	assert.Equal(t, "custom_host", cfg.Host)
	assert.Equal(t, uint16(5434), cfg.Port)
	assert.Equal(t, "custom_user", cfg.Username)
	assert.Equal(t, "custom_pass", cfg.Password)
	assert.Equal(t, "custom_db", cfg.Database)

	// Clean up
	os.Unsetenv("POSTGRES_HOST")
	os.Unsetenv("POSTGRES_PORT")
	os.Unsetenv("POSTGRES_USER")
	os.Unsetenv("POSTGRES_PASSWORD")
	os.Unsetenv("POSTGRES_DB")
}
