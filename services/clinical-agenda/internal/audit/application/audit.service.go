package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sigo-dental/clinical-agenda/internal/audit/domain"
)

var (
	ErrAuditForbidden = errors.New("only admin and auditor roles can query audit logs")
)

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

type randomIDGenerator struct{}

func (randomIDGenerator) NewID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}

type AuditService struct {
	repo  AuditRepository
	clock Clock
	ids   IDGenerator
}

func NewAuditService(repo AuditRepository, clock Clock, ids IDGenerator) *AuditService {
	if clock == nil {
		clock = systemClock{}
	}
	if ids == nil {
		ids = randomIDGenerator{}
	}
	return &AuditService{
		repo:  repo,
		clock: clock,
		ids:   ids,
	}
}

func (s *AuditService) Record(ctx context.Context, input RecordInput) (*domain.AuditEntry, error) {
	entry, err := domain.NewAuditEntry(
		s.ids.NewID(),
		input.ActorID,
		input.ActorRole,
		input.Action,
		input.ResourceType,
		input.ResourceID,
		input.SourceIP,
		s.clock.Now(),
		input.Metadata,
	)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Append(ctx, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

func (s *AuditService) ListLogs(ctx context.Context, actorRole string, filter AuditFilter) ([]*domain.AuditEntry, int, error) {
	role := strings.ToLower(strings.TrimSpace(actorRole))
	if role != "admin" && role != "auditor" {
		return nil, 0, ErrAuditForbidden
	}
	if filter.Limit <= 0 {
		filter.Limit = 50
	} else if filter.Limit > 200 {
		filter.Limit = 200
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return s.repo.List(ctx, filter)
}
