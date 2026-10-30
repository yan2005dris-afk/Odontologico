package redis

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

const sessionIdlePrefix = "session:idle:"

// SessionKey formats the canonical Redis idle session key ("session:idle:<session_id>") (S3, RNF-05).
func SessionKey(sessionID string) string {
	return sessionIdlePrefix + strings.TrimSpace(sessionID)
}

// InMemorySessionStore is a thread-safe SessionStorePort implementation supporting sliding TTL expiration.
type InMemorySessionStore struct {
	mu       sync.Mutex
	nowFn    func() time.Time
	sessions map[string]time.Time
}

// NewInMemorySessionStore creates an in-memory sliding TTL session store.
func NewInMemorySessionStore(nowFn func() time.Time) *InMemorySessionStore {
	if nowFn == nil {
		nowFn = time.Now
	}
	return &InMemorySessionStore{
		nowFn:    nowFn,
		sessions: make(map[string]time.Time),
	}
}

// RegisterSession seeds or updates a session with the given sliding idle TTL.
func (s *InMemorySessionStore) RegisterSession(sessionID string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := SessionKey(sessionID)
	s.sessions[key] = s.nowFn().Add(ttl)
}

// RevokeSession removes a session immediately.
func (s *InMemorySessionStore) RevokeSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, SessionKey(sessionID))
}

// TouchSession checks if session:idle:<sessionID> exists and has not expired; if active, renews its TTL (S3, RNF-05).
func (s *InMemorySessionStore) TouchSession(_ context.Context, sessionID string, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := SessionKey(sessionID)
	expiresAt, exists := s.sessions[key]
	if !exists {
		return false, nil
	}

	now := s.nowFn()
	if !now.Before(expiresAt) {
		delete(s.sessions, key)
		return false, nil
	}

	s.sessions[key] = now.Add(ttl)
	return true, nil
}

// RedisSessionStore implements SessionStorePort over the Redis RESP protocol (`EXPIRE session:idle:<id> 900`).
type RedisSessionStore struct {
	addr        string
	dialTimeout time.Duration
}

// NewRedisSessionStore constructs a Redis-backed session store for the given host:port.
func NewRedisSessionStore(addr string, dialTimeout time.Duration) *RedisSessionStore {
	if dialTimeout <= 0 {
		dialTimeout = 2 * time.Second
	}
	return &RedisSessionStore{
		addr:        addr,
		dialTimeout: dialTimeout,
	}
}

// TouchSession sends EXPIRE session:idle:<sessionID> <seconds> to Redis.
// Redis EXPIRE returns :1 if the timeout was refreshed on an existing key, or :0 if the key does not exist/expired.
func (r *RedisSessionStore) TouchSession(ctx context.Context, sessionID string, ttl time.Duration) (bool, error) {
	key := SessionKey(sessionID)
	seconds := int(ttl.Seconds())
	if seconds <= 0 {
		seconds = 900
	}

	dialer := net.Dialer{Timeout: r.dialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", r.addr)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(r.dialTimeout))
	}

	secStr := strconv.Itoa(seconds)
	cmd := fmt.Sprintf("*3\r\n$6\r\nEXPIRE\r\n$%d\r\n%s\r\n$%d\r\n%s\r\n", len(key), key, len(secStr), secStr)
	if _, err := conn.Write([]byte(cmd)); err != nil {
		return false, err
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, ":1") {
		return true, nil
	}
	if strings.HasPrefix(line, ":0") {
		return false, nil
	}
	return false, fmt.Errorf("unexpected redis response: %s", line)
}
