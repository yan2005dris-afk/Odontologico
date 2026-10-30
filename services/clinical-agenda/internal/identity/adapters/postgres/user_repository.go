package postgres

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/sigo-dental/clinical-agenda/internal/identity/domain"
)

const (
	InsertUserSQL    = `INSERT INTO users (id, email, full_name, role, password_hash, mfa_secret_encrypted, failed_attempts, locked_until, active, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	SelectByEmailSQL = `SELECT id, email, full_name, role, password_hash, mfa_secret_encrypted, failed_attempts, locked_until, active, created_at, updated_at FROM users WHERE email = $1`
	InsertSessionSQL = `INSERT INTO user_sessions (id, user_id, mfa_verified, last_activity_at, expires_at, revoked, source_ip, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
)

var ErrUserNotFound = errors.New("user not found")

type UserRepository struct {
	mu       sync.RWMutex
	byID     map[string]*domain.User
	byEmail  map[string]*domain.User
	sessions map[string]*domain.Session
	usedOTPs map[string]struct{}
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		byID:     make(map[string]*domain.User),
		byEmail:  make(map[string]*domain.User),
		sessions: make(map[string]*domain.Session),
		usedOTPs: make(map[string]struct{}),
	}
}

func (r *UserRepository) CreateUser(_ context.Context, user *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := *user
	r.byID[copied.ID] = &copied
	r.byEmail[strings.ToLower(copied.Email)] = &copied
	return nil
}

func (r *UserRepository) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.byEmail[strings.ToLower(strings.TrimSpace(email))]
	if !ok {
		return nil, ErrUserNotFound
	}
	copied := *u
	return &copied, nil
}

func (r *UserRepository) GetByID(_ context.Context, id string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.byID[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	copied := *u
	return &copied, nil
}

func (r *UserRepository) UpdateUser(_ context.Context, user *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := *user
	r.byID[copied.ID] = &copied
	r.byEmail[strings.ToLower(copied.Email)] = &copied
	return nil
}

func (r *UserRepository) SaveSession(_ context.Context, session *domain.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := *session
	r.sessions[copied.ID] = &copied
	return nil
}

func (r *UserRepository) GetSession(_ context.Context, sessionID string) (*domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sess, ok := r.sessions[sessionID]
	if !ok {
		return nil, ErrUserNotFound
	}
	copied := *sess
	return &copied, nil
}

func (r *UserRepository) MarkOTPUsed(_ context.Context, userID, code string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := userID + ":" + code
	if _, exists := r.usedOTPs[key]; exists {
		return false
	}
	r.usedOTPs[key] = struct{}{}
	return true
}
