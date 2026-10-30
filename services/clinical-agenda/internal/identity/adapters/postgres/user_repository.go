package postgres

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"

	"github.com/sigo-dental/clinical-agenda/internal/identity/application"
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

// ListUsers returns the staff accounts matching the filter, ordered by
// creation date (descending) with the id as tiebreaker so that pagination
// never duplicates nor skips an account between pages.
func (r *UserRepository) ListUsers(_ context.Context, filter application.UserFilter) ([]*domain.User, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := strings.ToLower(strings.TrimSpace(filter.Query))
	matched := make([]*domain.User, 0, len(r.byID))
	for _, u := range r.byID {
		if filter.Role != "" && u.Role != filter.Role {
			continue
		}
		if query != "" &&
			!strings.Contains(strings.ToLower(u.Email), query) &&
			!strings.Contains(strings.ToLower(u.FullName), query) {
			continue
		}
		copied := *u
		matched = append(matched, &copied)
	}

	sort.Slice(matched, func(i, j int) bool {
		if !matched[i].CreatedAt.Equal(matched[j].CreatedAt) {
			return matched[i].CreatedAt.After(matched[j].CreatedAt)
		}
		return matched[i].ID < matched[j].ID
	})

	total := len(matched)
	limit := filter.Limit
	if limit <= 0 {
		limit = application.DefaultListLimit
	} else if limit > application.MaxListLimit {
		limit = application.MaxListLimit
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return []*domain.User{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return matched[offset:end], total, nil
}
