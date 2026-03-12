package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrNotFound = errors.New("user not found")

type User struct {
	ID           string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, user User) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)`,
		user.ID,
		strings.ToLower(strings.TrimSpace(user.Email)),
		user.PasswordHash,
	)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}

	return nil
}

func (r *Repository) Delete(ctx context.Context, userID string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	return nil
}

func (r *Repository) FindByEmail(ctx context.Context, email string) (User, error) {
	var user User
	err := r.db.QueryRowContext(
		ctx,
		`SELECT id, email, password_hash, created_at, updated_at FROM users WHERE LOWER(email) = LOWER($1)`,
		strings.TrimSpace(email),
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("find user by email: %w", err)
	}

	return user, nil
}

func (r *Repository) FindByID(ctx context.Context, userID string) (User, error) {
	var user User
	err := r.db.QueryRowContext(
		ctx,
		`SELECT id, email, password_hash, created_at, updated_at FROM users WHERE id = $1`,
		userID,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("find user by id: %w", err)
	}

	return user, nil
}
