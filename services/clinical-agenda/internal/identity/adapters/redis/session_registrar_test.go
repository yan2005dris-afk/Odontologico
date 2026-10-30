package redis_test

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/sigo-dental/clinical-agenda/internal/identity/adapters/redis"
)

// fakeRedis answers one RESP command with the given reply and records what it
// received.
type fakeRedis struct {
	listener net.Listener
	received chan string
	reply    string
}

func startFakeRedis(t *testing.T, reply string) *fakeRedis {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start fake redis: %v", err)
	}
	fake := &fakeRedis{listener: listener, received: make(chan string, 1), reply: reply}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)

		// RESP array header: *<count>
		header, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		command := header
		count := 0
		if _, scanErr := fmt.Sscanf(strings.TrimSpace(header), "*%d", &count); scanErr != nil {
			return
		}
		for i := 0; i < count; i++ {
			sizeLine, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			command += sizeLine
			size := 0
			if _, scanErr := fmt.Sscanf(strings.TrimSpace(sizeLine), "$%d", &size); scanErr != nil {
				return
			}
			payload := make([]byte, size+2)
			if _, err := reader.Read(payload); err != nil {
				return
			}
			command += string(payload)
		}

		fake.received <- command
		_, _ = conn.Write([]byte(fake.reply))
		// The connection stays open until the test tears the listener down.
		_, _ = reader.ReadString('\n')
	}()

	t.Cleanup(func() { _ = listener.Close() })
	return fake
}

func TestIdleSessionRegistrar_PublishesTheIdleMarker(t *testing.T) {
	fake := startFakeRedis(t, "+OK\r\n")
	registrar := redis.NewIdleSessionRegistrar(fake.listener.Addr().String(), time.Second)

	if err := registrar.RegisterIdleSession(context.Background(), "sess-123", 900*time.Second); err != nil {
		t.Fatalf("expected the marker to be published, got %v", err)
	}

	command := <-fake.received
	if !strings.Contains(command, "SET") {
		t.Fatalf("expected a SET command, got %q", command)
	}
	if !strings.Contains(command, redis.SessionKey("sess-123")) {
		t.Fatalf("expected the canonical session key, got %q", command)
	}
	if !strings.Contains(command, "EX") || !strings.Contains(command, "900") {
		t.Fatalf("expected the idle TTL of 900 seconds, got %q", command)
	}
}

func TestIdleSessionRegistrar_RejectsAnUnconfiguredAdapter(t *testing.T) {
	registrar := redis.NewIdleSessionRegistrar("", time.Second)
	if err := registrar.RegisterIdleSession(context.Background(), "sess-123", 900*time.Second); err == nil {
		t.Fatalf("expected an error when the adapter has no address")
	}
}

func TestIdleSessionRegistrar_RejectsAnEmptySessionID(t *testing.T) {
	fake := startFakeRedis(t, "+OK\r\n")
	registrar := redis.NewIdleSessionRegistrar(fake.listener.Addr().String(), time.Second)

	if err := registrar.RegisterIdleSession(context.Background(), "  ", 900*time.Second); err == nil {
		t.Fatalf("expected an error for an empty session id")
	}
}
