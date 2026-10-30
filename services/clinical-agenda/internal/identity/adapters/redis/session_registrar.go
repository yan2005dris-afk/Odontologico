// Package redis provides the Redis adapter of the identity context: it
// publishes the idle-session marker of a new session in the shared session
// store so the edge gateway can enforce the 15-minute idle timeout (RNF-05).
package redis

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// sessionIdlePrefix is the canonical key prefix of the shared idle-session
// store; it must match the key the API Gateway refreshes (`session:idle:<id>`).
const sessionIdlePrefix = "session:idle:"

// SessionKey formats the canonical Redis idle session key.
func SessionKey(sessionID string) string {
	return sessionIdlePrefix + strings.TrimSpace(sessionID)
}

// IdleSessionRegistrar writes the idle-session marker over the Redis RESP
// protocol (`SET session:idle:<id> 1 EX <seconds>`).
type IdleSessionRegistrar struct {
	addr        string
	dialTimeout time.Duration
}

// NewIdleSessionRegistrar builds the adapter for the given host:port.
func NewIdleSessionRegistrar(addr string, dialTimeout time.Duration) *IdleSessionRegistrar {
	if dialTimeout <= 0 {
		dialTimeout = 2 * time.Second
	}
	return &IdleSessionRegistrar{addr: strings.TrimSpace(addr), dialTimeout: dialTimeout}
}

// RegisterIdleSession creates the marker with the idle TTL of the session.
func (r *IdleSessionRegistrar) RegisterIdleSession(ctx context.Context, sessionID string, ttl time.Duration) error {
	if r == nil || r.addr == "" {
		return fmt.Errorf("redis: idle session registrar is not configured")
	}
	trimmed := strings.TrimSpace(sessionID)
	if trimmed == "" {
		return fmt.Errorf("redis: session id is required")
	}
	seconds := int(ttl.Seconds())
	if seconds <= 0 {
		seconds = 900
	}

	dialer := net.Dialer{Timeout: r.dialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", r.addr)
	if err != nil {
		return fmt.Errorf("redis: dial %s: %w", r.addr, err)
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(r.dialTimeout))
	}

	key := SessionKey(trimmed)
	ttlSeconds := strconv.Itoa(seconds)
	cmd := fmt.Sprintf("*5\r\n$3\r\nSET\r\n$%d\r\n%s\r\n$1\r\n1\r\n$2\r\nEX\r\n$%d\r\n%s\r\n",
		len(key), key, len(ttlSeconds), ttlSeconds)
	if _, err := conn.Write([]byte(cmd)); err != nil {
		return fmt.Errorf("redis: write session marker: %w", err)
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("redis: read session marker reply: %w", err)
	}
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "+OK") {
		return fmt.Errorf("redis: unexpected reply for session marker: %s", line)
	}
	return nil
}
