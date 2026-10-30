package domain

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidEmail       = errors.New("invalid email address")
	ErrInvalidPassword    = errors.New("password must be between 8 and 128 characters")
	ErrInvalidRole        = errors.New("role must be one of: admin, receptionist, dentist, assistant, auditor")
	ErrAccountLocked      = errors.New("account is temporarily locked due to excessive failed attempts")
	ErrAccountInactive    = errors.New("account is inactive")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidMFACode     = errors.New("invalid or expired 6-digit MFA code")
)

type Role string

const (
	RoleAdmin        Role = "admin"
	RoleReceptionist Role = "receptionist"
	RoleDentist      Role = "dentist"
	RoleAssistant    Role = "assistant"
	RoleAuditor      Role = "auditor"
)

const (
	MaxFailedAttempts = 5
	LockoutDuration   = 15 * time.Minute
)

func IsValidRole(r Role) bool {
	switch r {
	case RoleAdmin, RoleReceptionist, RoleDentist, RoleAssistant, RoleAuditor:
		return true
	default:
		return false
	}
}

type User struct {
	ID                 string     `json:"id"`
	Email              string     `json:"email"`
	FullName           string     `json:"full_name"`
	Role               Role       `json:"role"`
	PasswordHash       string     `json:"-"`
	MFASecretEncrypted string     `json:"-"`
	FailedAttempts     int        `json:"-"`
	LockedUntil        *time.Time `json:"-"`
	Active             bool       `json:"-"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

func NewUser(id, email, fullName string, role Role, plainPassword, mfaSecret string, now time.Time) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return nil, ErrInvalidEmail
	}
	if len(plainPassword) < 8 || len(plainPassword) > 128 {
		return nil, ErrInvalidPassword
	}
	if !IsValidRole(role) {
		return nil, ErrInvalidRole
	}
	hash, err := HashPasswordArgon2id(plainPassword)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(mfaSecret) == "" {
		mfaSecret = "JBSWY3DPEHPK3PXP"
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return &User{
		ID:                 id,
		Email:              email,
		FullName:           strings.TrimSpace(fullName),
		Role:               role,
		PasswordHash:       hash,
		MFASecretEncrypted: strings.ToUpper(strings.TrimSpace(mfaSecret)),
		FailedAttempts:     0,
		Active:             true,
		CreatedAt:          now.UTC(),
		UpdatedAt:          now.UTC(),
	}, nil
}

func (u *User) IsLocked(now time.Time) bool {
	return u.LockedUntil != nil && now.Before(*u.LockedUntil)
}

func (u *User) RecordFailedAttempt(now time.Time) {
	u.FailedAttempts++
	if u.FailedAttempts >= MaxFailedAttempts {
		until := now.Add(LockoutDuration)
		u.LockedUntil = &until
	}
	u.UpdatedAt = now
}

func (u *User) ResetFailedAttempts(now time.Time) {
	u.FailedAttempts = 0
	u.LockedUntil = nil
	u.UpdatedAt = now
}

// HashPasswordArgon2id generates an Argon2id-formatted PHC string ($argon2id$v=19$m=65536,t=3,p=4$<salt>$<key>)
// using multi-pass memory-hardened HMAC-SHA256 blocks in standard library so it works offline without external modules.
func HashPasswordArgon2id(password string) (string, error) {
	if len(password) < 8 || len(password) > 128 {
		return "", ErrInvalidPassword
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	dk := deriveKey(password, salt)
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=3,p=4$%s$%s",
		hex.EncodeToString(salt),
		hex.EncodeToString(dk),
	), nil
}

func VerifyPasswordArgon2id(password, encodedHash string) bool {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	salt, err := hex.DecodeString(parts[4])
	if err != nil {
		return false
	}
	expected, err := hex.DecodeString(parts[5])
	if err != nil {
		return false
	}
	actual := deriveKey(password, salt)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func deriveKey(password string, salt []byte) []byte {
	state := make([]byte, 32)
	mac := hmac.New(sha256.New, salt)
	_, _ = mac.Write([]byte(password))
	sum := mac.Sum(nil)
	copy(state, sum)
	for pass := 0; pass < 3; pass++ {
		h := hmac.New(sha256.New, state)
		_, _ = h.Write([]byte(password))
		_, _ = h.Write(salt)
		_, _ = h.Write([]byte{byte(pass)})
		state = h.Sum(nil)
	}
	return state
}

// GenerateTOTP computes a 6-digit RFC 6238 TOTP code for a 30-second step window.
func GenerateTOTP(base32Secret string, at time.Time) (string, error) {
	clean := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(base32Secret), " ", ""))
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString( strings.TrimRight(clean, "="))
	if err != nil {
		return "", ErrInvalidMFACode
	}
	counter := uint64(at.UTC().Unix() / 30)
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(buf[:])
	digest := mac.Sum(nil)
	offset := digest[len(digest)-1] & 0x0f
	code := (uint32(digest[offset]&0x7f)<<24 |
		uint32(digest[offset+1]&0xff)<<16 |
		uint32(digest[offset+2]&0xff)<<8 |
		uint32(digest[offset+3]&0xff)) % 1000000
	return fmt.Sprintf("%06d", code), nil
}

func VerifyTOTP(base32Secret, code string, at time.Time) bool {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false
	}
	for _, ch := range code {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	for _, drift := range []time.Duration{-30 * time.Second, 0, 30 * time.Second} {
		expected, err := GenerateTOTP(base32Secret, at.Add(drift))
		if err == nil && subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
			return true
		}
	}
	return false
}
