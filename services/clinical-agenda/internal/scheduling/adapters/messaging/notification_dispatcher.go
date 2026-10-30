package messaging

import (
	"context"
	"sync"

	"github.com/sigo-dental/clinical-agenda/internal/scheduling/domain"
)

type NotificationDispatcher struct {
	mu         sync.RWMutex
	dispatched map[string]domain.ScheduledReminder
}

func NewNotificationDispatcher() *NotificationDispatcher {
	return &NotificationDispatcher{
		dispatched: make(map[string]domain.ScheduledReminder),
	}
}

func (d *NotificationDispatcher) EnqueueAndDispatch(_ context.Context, job domain.ScheduledReminder) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, exists := d.dispatched[job.IdempotencyKey]; exists {
		return nil
	}
	job.Status = "DISPATCHED"
	d.dispatched[job.IdempotencyKey] = job
	return nil
}

func (d *NotificationDispatcher) CountDispatched() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.dispatched)
}
