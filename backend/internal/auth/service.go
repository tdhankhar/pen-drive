package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/oklog/ulid/v2"
	"golang.org/x/crypto/bcrypt"

	"github.com/abhishek/pen-drive/backend/internal/config"
	"github.com/abhishek/pen-drive/backend/internal/storage"
	"github.com/abhishek/pen-drive/backend/internal/users"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid token")
)

type Service struct {
	db        *sql.DB
	users     *users.Repository
	storage   *storage.Client
	jwtConfig config.JWTConfig
	now       func() time.Time
}

type TokenPair struct {
	AccessToken           string    `json:"access_token"`
	AccessTokenExpiresAt  time.Time `json:"access_token_expires_at"`
	RefreshToken          string    `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at"`
}

type AuthenticatedUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type Claims struct {
	jwt.RegisteredClaims
	Email string `json:"email"`
}

func NewService(db *sql.DB, userRepo *users.Repository, storageClient *storage.Client, jwtCfg config.JWTConfig) *Service {
	return &Service{
		db:        db,
		users:     userRepo,
		storage:   storageClient,
		jwtConfig: jwtCfg,
		now:       time.Now,
	}
}

func (s *Service) Signup(ctx context.Context, email, password string) (AuthenticatedUser, TokenPair, error) {
	email = normalizeEmail(email)
	if err := validateCredentials(email, password); err != nil {
		return AuthenticatedUser{}, TokenPair{}, err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return AuthenticatedUser{}, TokenPair{}, fmt.Errorf("hash password: %w", err)
	}

	user := users.User{
		ID:           strings.ToLower(ulid.Make().String()),
		Email:        email,
		PasswordHash: string(passwordHash),
	}

	if err := s.users.Create(ctx, user); err != nil {
		return AuthenticatedUser{}, TokenPair{}, err
	}

	if err := s.storage.CreateBucket(ctx, user.ID); err != nil {
		_ = s.users.Delete(ctx, user.ID)
		return AuthenticatedUser{}, TokenPair{}, fmt.Errorf("create user bucket: %w", err)
	}

	tokens, err := s.issueTokenPair(ctx, user)
	if err != nil {
		_ = s.users.Delete(ctx, user.ID)
		_ = s.storage.DeleteBucket(ctx, user.ID)
		return AuthenticatedUser{}, TokenPair{}, err
	}

	return AuthenticatedUser{ID: user.ID, Email: user.Email}, tokens, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (AuthenticatedUser, TokenPair, error) {
	user, err := s.users.FindByEmail(ctx, normalizeEmail(email))
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return AuthenticatedUser{}, TokenPair{}, ErrInvalidCredentials
		}
		return AuthenticatedUser{}, TokenPair{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return AuthenticatedUser{}, TokenPair{}, ErrInvalidCredentials
	}

	tokens, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return AuthenticatedUser{}, TokenPair{}, err
	}

	return AuthenticatedUser{ID: user.ID, Email: user.Email}, tokens, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (AuthenticatedUser, TokenPair, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return AuthenticatedUser{}, TokenPair{}, ErrInvalidToken
	}

	row, err := s.lookupRefreshToken(ctx, refreshToken)
	if err != nil {
		return AuthenticatedUser{}, TokenPair{}, err
	}

	if row.RevokedAt.Valid || row.ExpiresAt.Before(s.now().UTC()) {
		return AuthenticatedUser{}, TokenPair{}, ErrInvalidToken
	}

	if err := s.revokeRefreshToken(ctx, row.ID); err != nil {
		return AuthenticatedUser{}, TokenPair{}, err
	}

	user, err := s.users.FindByID(ctx, row.UserID)
	if err != nil {
		return AuthenticatedUser{}, TokenPair{}, err
	}

	tokens, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return AuthenticatedUser{}, TokenPair{}, err
	}

	return AuthenticatedUser{ID: user.ID, Email: user.Email}, tokens, nil
}

func (s *Service) ParseAccessToken(token string) (Claims, error) {
	claims := Claims{}
	parsed, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(s.jwtConfig.Secret), nil
	}, jwt.WithAudience(s.jwtConfig.Audience), jwt.WithIssuer(s.jwtConfig.Issuer))
	if err != nil {
		return Claims{}, ErrInvalidToken
	}

	if !parsed.Valid {
		return Claims{}, ErrInvalidToken
	}

	return claims, nil
}

func (s *Service) issueTokenPair(ctx context.Context, user users.User) (TokenPair, error) {
	accessTTL, _ := s.jwtConfig.AccessTTL()
	refreshTTL, _ := s.jwtConfig.RefreshTTL()

	now := s.now().UTC()
	accessExpiresAt := now.Add(accessTTL)
	refreshExpiresAt := now.Add(refreshTTL)

	accessClaims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			Issuer:    s.jwtConfig.Issuer,
			Audience:  []string{s.jwtConfig.Audience},
			ExpiresAt: jwt.NewNumericDate(accessExpiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
		Email: user.Email,
	}

	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(s.jwtConfig.Secret))
	if err != nil {
		return TokenPair{}, fmt.Errorf("sign access token: %w", err)
	}

	refreshToken, err := newRefreshToken()
	if err != nil {
		return TokenPair{}, fmt.Errorf("generate refresh token: %w", err)
	}

	if err := s.storeRefreshToken(ctx, user.ID, refreshToken, refreshExpiresAt); err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:           accessToken,
		AccessTokenExpiresAt:  accessExpiresAt,
		RefreshToken:          refreshToken,
		RefreshTokenExpiresAt: refreshExpiresAt,
	}, nil
}

type refreshTokenRow struct {
	ID        string
	UserID    string
	ExpiresAt time.Time
	RevokedAt sql.NullTime
}

func (s *Service) storeRefreshToken(ctx context.Context, userID, token string, expiresAt time.Time) error {
	tokenID := strings.ToLower(ulid.Make().String())
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at) VALUES ($1, $2, $3, $4)`,
		tokenID,
		userID,
		hashToken(token),
		expiresAt,
	)
	if err != nil {
		return fmt.Errorf("store refresh token: %w", err)
	}

	return nil
}

func (s *Service) lookupRefreshToken(ctx context.Context, token string) (refreshTokenRow, error) {
	var row refreshTokenRow
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, user_id, expires_at, revoked_at FROM refresh_tokens WHERE token_hash = $1`,
		hashToken(token),
	).Scan(&row.ID, &row.UserID, &row.ExpiresAt, &row.RevokedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return refreshTokenRow{}, ErrInvalidToken
		}
		return refreshTokenRow{}, fmt.Errorf("lookup refresh token: %w", err)
	}

	return row, nil
}

func (s *Service) revokeRefreshToken(ctx context.Context, tokenID string) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE refresh_tokens SET revoked_at = NOW() WHERE id = $1`, tokenID); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}

	return nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validateCredentials(email, password string) error {
	if email == "" || !strings.Contains(email, "@") {
		return fmt.Errorf("email is invalid")
	}

	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	return nil
}

func newRefreshToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
