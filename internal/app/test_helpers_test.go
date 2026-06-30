package app

import (
	"context"
	"sync"
	"time"

	"github.com/hamardikan/lazyss/internal/domain"
)

type memoryStore struct {
	mu       sync.Mutex
	overlays map[domain.MachineID]domain.MachineOverlay
	health   map[domain.MachineID][]domain.HealthObservation
	sessions map[domain.MachineID][]domain.SessionEvent
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		overlays: map[domain.MachineID]domain.MachineOverlay{},
		health:   map[domain.MachineID][]domain.HealthObservation{},
		sessions: map[domain.MachineID][]domain.SessionEvent{},
	}
}

func (m *memoryStore) LoadOverlay(_ context.Context, id domain.MachineID) (domain.MachineOverlay, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ov := m.overlays[id]
	if ov.MachineID == "" {
		ov.MachineID = id
	}
	return ov, nil
}
func (m *memoryStore) SaveOverlay(_ context.Context, overlay domain.MachineOverlay) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.overlays[overlay.MachineID] = overlay
	return nil
}
func (m *memoryStore) RecordHealth(_ context.Context, obs domain.HealthObservation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.health[obs.MachineID] = append(m.health[obs.MachineID], obs)
	ov := m.overlays[obs.MachineID]
	ov.MachineID = obs.MachineID
	ov.LastHealth = obs
	ov.LastCheckedAt = obs.CheckedAt
	m.overlays[obs.MachineID] = ov
	return nil
}
func (m *memoryStore) RecordSession(_ context.Context, event domain.SessionEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[event.MachineID] = append(m.sessions[event.MachineID], event)
	ov := m.overlays[event.MachineID]
	ov.MachineID = event.MachineID
	if event.Success {
		ov.LastConnectedAt = event.EndedAt
		ov.ConnectionCount++
	}
	m.overlays[event.MachineID] = ov
	return nil
}

type fixedClock struct{ t time.Time }

func (f fixedClock) Now() time.Time { return f.t }
