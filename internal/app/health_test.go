package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hmrdkn-labs/lazyss/internal/domain"
)

func TestHealthChecksVisibleWithConcurrencyCap(t *testing.T) {
	store := newMemoryStore()
	checker := &recordingChecker{delay: 10 * time.Millisecond}
	svc := HealthService{Checkers: []HealthChecker{checker}, Store: store, MaxConcurrent: 2}
	machines := []domain.Machine{
		{ID: "ssh:a", Methods: []domain.AccessMethod{domain.AccessSSH}},
		{ID: "ssh:b", Methods: []domain.AccessMethod{domain.AccessSSH}},
		{ID: "ssh:c", Methods: []domain.AccessMethod{domain.AccessSSH}},
	}
	obs := svc.CheckVisible(context.Background(), machines, domain.AccessSSH)
	if len(obs) != 3 {
		t.Fatalf("observations = %d, want 3", len(obs))
	}
	if checker.maxSeen > 2 {
		t.Fatalf("max concurrency = %d, want <= 2", checker.maxSeen)
	}
}

type recordingChecker struct {
	mu      sync.Mutex
	delay   time.Duration
	current int
	maxSeen int
}

func (r *recordingChecker) Supports(domain.Machine, domain.AccessMethod) bool { return true }
func (r *recordingChecker) Check(_ context.Context, m domain.Machine, method domain.AccessMethod) domain.HealthObservation {
	r.mu.Lock()
	r.current++
	if r.current > r.maxSeen {
		r.maxSeen = r.current
	}
	r.mu.Unlock()
	time.Sleep(r.delay)
	r.mu.Lock()
	r.current--
	r.mu.Unlock()
	return domain.NewHealthObservation(m.ID, method, domain.HealthUp, "tcp host:22", time.Millisecond, time.Now())
}
