// Package statejson persists LazySS local operator memory as atomic JSON.
package statejson

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/hmrdkn-labs/lazyss/internal/domain"
)

const defaultHistoryLimit = 20

// Store reads and writes local machine overlays and history.
type Store struct {
	path         string
	historyLimit int
	mu           sync.Mutex
}

type stateFile struct {
	Preferences domain.OperatorPreferences                 `json:"preferences,omitempty"`
	Overlays    map[domain.MachineID]domain.MachineOverlay `json:"overlays"`
}

// New creates a JSON state store.
func New(path string, historyLimit int) *Store {
	if historyLimit <= 0 {
		historyLimit = defaultHistoryLimit
	}
	return &Store{path: path, historyLimit: historyLimit}
}

// DefaultPath returns the default LazySS state path under the user config dir.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "lazyss", "state.json"), nil
}

// LoadOverlay returns local memory for one machine.
func (s *Store) LoadOverlay(_ context.Context, id domain.MachineID) (domain.MachineOverlay, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.load()
	if err != nil {
		return domain.MachineOverlay{}, err
	}
	overlay := st.Overlays[id]
	if overlay.MachineID == "" {
		overlay.MachineID = id
	}
	return overlay, nil
}

// SaveOverlay persists local memory for one machine.
func (s *Store) SaveOverlay(_ context.Context, overlay domain.MachineOverlay) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.load()
	if err != nil {
		return err
	}
	st.Overlays[overlay.MachineID] = overlay
	return s.save(st)
}

// LoadPreferences returns safe local cockpit preferences.
func (s *Store) LoadPreferences(_ context.Context) (domain.OperatorPreferences, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.load()
	if err != nil {
		return domain.OperatorPreferences{}, err
	}
	return st.Preferences, nil
}

// SavePreferences persists safe local cockpit preferences.
func (s *Store) SavePreferences(_ context.Context, prefs domain.OperatorPreferences) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.load()
	if err != nil {
		return err
	}
	st.Preferences = prefs
	return s.save(st)
}

// RecordHealth records the latest health observation and capped history.
func (s *Store) RecordHealth(_ context.Context, obs domain.HealthObservation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.load()
	if err != nil {
		return err
	}
	overlay := st.Overlays[obs.MachineID]
	overlay.MachineID = obs.MachineID
	overlay.LastHealth = obs
	overlay.LastCheckedAt = obs.CheckedAt
	overlay.HealthHistory = appendCapped(overlay.HealthHistory, obs, s.historyLimit)
	st.Overlays[obs.MachineID] = overlay
	return s.save(st)
}

// RecordSession records a connection attempt and capped history.
func (s *Store) RecordSession(_ context.Context, event domain.SessionEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.load()
	if err != nil {
		return err
	}
	overlay := st.Overlays[event.MachineID]
	overlay.MachineID = event.MachineID
	if event.Success {
		overlay.LastConnectedAt = event.EndedAt
		overlay.ConnectionCount++
	}
	overlay.SessionHistory = appendCapped(overlay.SessionHistory, event, s.historyLimit)
	st.Overlays[event.MachineID] = overlay
	return s.save(st)
}

func appendCapped[T any](items []T, item T, limit int) []T {
	items = append(items, item)
	if len(items) > limit {
		items = items[len(items)-limit:]
	}
	return items
}

func (s *Store) load() (stateFile, error) {
	st := stateFile{Overlays: map[domain.MachineID]domain.MachineOverlay{}}
	if s.path == "" {
		return st, nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return st, nil
		}
		return st, err
	}
	if len(data) == 0 {
		return st, nil
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return st, fmt.Errorf("parse state %s: %w", s.path, err)
	}
	if st.Overlays == nil {
		st.Overlays = map[domain.MachineID]domain.MachineOverlay{}
	}
	return st, nil
}

func (s *Store) save(st stateFile) error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".state-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return err
	}
	cleanup = false
	return os.Chmod(s.path, 0o600)
}
