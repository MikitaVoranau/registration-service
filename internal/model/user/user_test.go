package user_test

import (
	"registration-service/internal/model/user"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserModel(t *testing.T) {
	t.Run("User struct fields", func(t *testing.T) {
		user := user.User{
			ID:       1,
			Username: "testuser",
			Email:    "test@example.com",
			Password: "hashedpassword",
		}

		assert.Equal(t, uint64(1), user.ID)
		assert.Equal(t, "testuser", user.Username)
		assert.Equal(t, "test@example.com", user.Email)
		assert.Equal(t, "hashedpassword", user.Password)
	})
}
