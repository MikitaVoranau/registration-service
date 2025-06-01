package userRepo

import (
	"context"
	"fmt"
	"registration-service/internal/model/user"

	"github.com/jackc/pgx/v5"
)

type UserRepo struct {
	conn *pgx.Conn
}

func New(conn *pgx.Conn) *UserRepo {
	return &UserRepo{conn: conn}
}

func (r *UserRepo) Create(ctx context.Context, username, email, passwordHash string) (uint32, error) {
	query := `INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3) RETURNING id`
	var userID uint32
	err := r.conn.QueryRow(ctx, query, username, email, passwordHash).Scan(&userID)
	if err != nil {
		return 0, fmt.Errorf("failed to insert user and retrieve id: %w", err)
	}
	return userID, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id uint32) (*user.User, error) {
	query := `SELECT id, username, email, password_hash FROM users WHERE id=$1`
	row := r.conn.QueryRow(ctx, query, id)

	var user user.User
	err := row.Scan(&user.ID, &user.Username, &user.Email, &user.Password)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return &user, nil
}

func (r *UserRepo) GetUserByEmail(ctx context.Context, email string) (*user.User, error) {
	query := `SELECT id, username, email, password_hash FROM users WHERE email=$1`
	row := r.conn.QueryRow(ctx, query, email)
	var user user.User
	err := row.Scan(&user.ID, &user.Username, &user.Email, &user.Password)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) GetByUsername(ctx context.Context, username string) ([]*user.User, error) {
	query := `SELECT id, username, email, password_hash FROM users WHERE username=$1`
	rows, err := r.conn.Query(ctx, query, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*user.User
	for rows.Next() {
		var user user.User
		if err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.Password); err != nil {
			return nil, err
		}
		users = append(users, &user)
	}
	return users, nil
}
