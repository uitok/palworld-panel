package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"palpanel/internal/db"
	"palpanel/internal/id"
)

const (
	SessionCookieName = "palpanel_session"
	SessionLifetime   = 7 * 24 * time.Hour
	PasswordCost      = 12
)

var (
	usernamePattern           = regexp.MustCompile(`^[A-Za-z0-9._-]{3,32}$`)
	ErrInvalidLogin           = errors.New("invalid username or password")
	ErrInvalidCurrentPassword = errors.New("current password is incorrect")
	ErrUserDisabled           = errors.New("user is disabled")
	ErrInvalidUsername        = errors.New("username must be 3-32 letters, numbers, dots, underscores, or hyphens")
	ErrInvalidPassword        = errors.New("password must be between 12 and 128 characters")
	ErrInvalidKeyName         = errors.New("development key name must be between 1 and 64 characters")
)

type Credential string

const (
	CredentialSession Credential = "session"
	CredentialAPIKey  Credential = "api_key"
	CredentialLocal   Credential = "local"
)

type Identity struct {
	UserID     string
	Username   string
	Role       string
	Credential Credential
}

type Service struct {
	store *db.Store
	now   func() time.Time
}

type CreatedAPIKey struct {
	db.APIKey
	Token string `json:"token"`
}

func New(store *db.Store) *Service {
	return &Service{store: store, now: time.Now}
}

func (s *Service) Initialized(ctx context.Context) (bool, error) {
	count, err := s.store.UserCount(ctx)
	return count > 0, err
}

func (s *Service) Register(ctx context.Context, username, password string) (db.User, string, error) {
	username = strings.TrimSpace(username)
	if err := ValidateUsername(username); err != nil {
		return db.User{}, "", err
	}
	passwordHash, err := HashPassword(password)
	if err != nil {
		return db.User{}, "", err
	}
	user := db.User{ID: id.New("usr"), Username: username, PasswordHash: passwordHash, Role: "admin"}
	if err := s.store.CreateInitialUser(ctx, user); err != nil {
		return db.User{}, "", err
	}
	token, err := s.newSession(ctx, user.ID)
	return user, token, err
}

func (s *Service) Login(ctx context.Context, username, password string) (db.User, string, error) {
	user, err := s.store.GetUserByUsername(ctx, strings.TrimSpace(username))
	if errors.Is(err, sql.ErrNoRows) {
		return db.User{}, "", ErrInvalidLogin
	}
	if err != nil {
		return db.User{}, "", err
	}
	if user.Disabled {
		return db.User{}, "", ErrUserDisabled
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return db.User{}, "", ErrInvalidLogin
	}
	token, err := s.newSession(ctx, user.ID)
	return user, token, err
}

func (s *Service) AuthenticateSession(ctx context.Context, token string) (Identity, error) {
	if strings.TrimSpace(token) == "" {
		return Identity{}, sql.ErrNoRows
	}
	now := s.now().UTC()
	user, session, err := s.store.GetUserBySessionHash(ctx, TokenHash(token), now)
	if err != nil {
		return Identity{}, err
	}
	if lastSeen, parseErr := time.Parse(time.RFC3339Nano, session.LastSeenAt); parseErr != nil || now.Sub(lastSeen) >= 5*time.Minute {
		_ = s.store.TouchSession(ctx, session.ID, now)
	}
	return Identity{UserID: user.ID, Username: user.Username, Role: user.Role, Credential: CredentialSession}, nil
}

func (s *Service) AuthenticateAPIKey(ctx context.Context, token string) (Identity, error) {
	if !strings.HasPrefix(token, "ppk_") {
		return Identity{}, sql.ErrNoRows
	}
	user, key, err := s.store.GetUserByAPIKeyHash(ctx, TokenHash(token))
	if err != nil {
		return Identity{}, err
	}
	now := s.now().UTC()
	if key.LastUsedAt == "" {
		_ = s.store.TouchAPIKey(ctx, key.ID, now)
	} else if lastUsed, parseErr := time.Parse(time.RFC3339Nano, key.LastUsedAt); parseErr != nil || now.Sub(lastUsed) >= 5*time.Minute {
		_ = s.store.TouchAPIKey(ctx, key.ID, now)
	}
	return Identity{UserID: user.ID, Username: user.Username, Role: user.Role, Credential: CredentialAPIKey}, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	return s.store.DeleteSessionByHash(ctx, TokenHash(token))
}

func (s *Service) CreateAPIKey(ctx context.Context, userID, name string) (CreatedAPIKey, error) {
	name = strings.TrimSpace(name)
	if len(name) < 1 || len(name) > 64 {
		return CreatedAPIKey{}, ErrInvalidKeyName
	}
	raw, err := randomToken(32)
	if err != nil {
		return CreatedAPIKey{}, err
	}
	token := "ppk_" + raw
	prefix := token
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	key := db.APIKey{ID: id.New("key"), UserID: userID, Name: name, Prefix: prefix, TokenHash: TokenHash(token)}
	if err := s.store.CreateAPIKey(ctx, key); err != nil {
		return CreatedAPIKey{}, err
	}
	return CreatedAPIKey{APIKey: key, Token: token}, nil
}

func (s *Service) ListAPIKeys(ctx context.Context, userID string) ([]db.APIKey, error) {
	return s.store.ListAPIKeys(ctx, userID)
}

func (s *Service) RevokeAPIKey(ctx context.Context, userID, keyID string) error {
	return s.store.RevokeAPIKey(ctx, userID, keyID)
}

func (s *Service) ResetPassword(ctx context.Context, username, password string) error {
	if err := ValidateUsername(strings.TrimSpace(username)); err != nil {
		return err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	return s.store.ResetUserPassword(ctx, strings.TrimSpace(username), hash)
}

func (s *Service) ChangePassword(ctx context.Context, username, currentPassword, newPassword string) error {
	username = strings.TrimSpace(username)
	user, err := s.store.GetUserByUsername(ctx, username)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidCurrentPassword
	}
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)) != nil {
		return ErrInvalidCurrentPassword
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.store.ResetUserPassword(ctx, username, hash)
}

func ValidateUsername(username string) error {
	if !usernamePattern.MatchString(username) {
		return ErrInvalidUsername
	}
	return nil
}

func HashPassword(password string) (string, error) {
	if len(password) < 12 || len(password) > 128 {
		return "", ErrInvalidPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), PasswordCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func TokenHash(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

func (s *Service) newSession(ctx context.Context, userID string) (string, error) {
	token, err := randomToken(32)
	if err != nil {
		return "", err
	}
	now := s.now().UTC()
	session := db.Session{
		ID:         id.New("ses"),
		UserID:     userID,
		TokenHash:  TokenHash(token),
		ExpiresAt:  now.Add(SessionLifetime).Format(time.RFC3339Nano),
		LastSeenAt: now.Format(time.RFC3339Nano),
		CreatedAt:  now.Format(time.RFC3339Nano),
	}
	if err := s.store.CreateSession(ctx, session); err != nil {
		return "", err
	}
	return token, nil
}

func randomToken(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate secure token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
