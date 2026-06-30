package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

type MachineID string

type ProviderKind string

const (
	ProviderSSH ProviderKind = "ssh"
	ProviderAWS ProviderKind = "aws"
)

type AccessMethod string

const (
	AccessSSH         AccessMethod = "ssh"
	AccessAWSSSMShell AccessMethod = "aws-ssm-shell"
	AccessSSHOverSSM  AccessMethod = "ssh-over-ssm"
)

type HealthStatus string

const (
	HealthUnknown HealthStatus = "unknown"
	HealthUp      HealthStatus = "up"
	HealthDown    HealthStatus = "down"
)

type ProviderState string

const (
	ProviderHealthy  ProviderState = "healthy"
	ProviderDegraded ProviderState = "degraded"
)

type Scope struct {
	Account string `json:"account,omitempty"`
	Region  string `json:"region,omitempty"`
	Profile string `json:"profile,omitempty"`
}

type Machine struct {
	ID              MachineID           `json:"id"`
	Name            string              `json:"name"`
	Provider        ProviderKind        `json:"provider"`
	NativeID        string              `json:"native_id,omitempty"`
	Address         string              `json:"address,omitempty"`
	User            string              `json:"user,omitempty"`
	Port            int                 `json:"port,omitempty"`
	Platform        string              `json:"platform,omitempty"`
	State           string              `json:"state,omitempty"`
	Scope           Scope               `json:"scope"`
	Tags            []string            `json:"tags,omitempty"`
	ProviderTags    map[string]string   `json:"provider_tags,omitempty"`
	Methods         []AccessMethod      `json:"methods"`
	SelectedMethod  AccessMethod        `json:"selected_method,omitempty"`
	PreferredMethod AccessMethod        `json:"preferred_method,omitempty"`
	Health          HealthObservation   `json:"health"`
	Pinned          bool                `json:"pinned"`
	Note            string              `json:"note,omitempty"`
	LastCheckedAt   time.Time           `json:"last_checked_at,omitempty"`
	LastConnectedAt time.Time           `json:"last_connected_at,omitempty"`
	ConnectionCount int                 `json:"connection_count,omitempty"`
	HealthHistory   []HealthObservation `json:"health_history,omitempty"`
	SessionHistory  []SessionEvent      `json:"session_history,omitempty"`
	Metadata        map[string]string   `json:"metadata,omitempty"`
}

type MachineOverlay struct {
	MachineID       MachineID           `json:"machine_id"`
	Pinned          bool                `json:"pinned,omitempty"`
	Tags            []string            `json:"tags,omitempty"`
	Note            string              `json:"note,omitempty"`
	PreferredMethod AccessMethod        `json:"preferred_method,omitempty"`
	LastCheckedAt   time.Time           `json:"last_checked_at,omitempty"`
	LastConnectedAt time.Time           `json:"last_connected_at,omitempty"`
	ConnectionCount int                 `json:"connection_count,omitempty"`
	LastHealth      HealthObservation   `json:"last_health,omitempty"`
	HealthHistory   []HealthObservation `json:"health_history,omitempty"`
	SessionHistory  []SessionEvent      `json:"session_history,omitempty"`
}

type HealthObservation struct {
	MachineID MachineID      `json:"machine_id"`
	Method    AccessMethod   `json:"method"`
	Status    HealthStatus   `json:"status"`
	Label     string         `json:"label"`
	Latency   *time.Duration `json:"latency,omitempty"`
	CheckedAt time.Time      `json:"checked_at"`
	Error     string         `json:"error,omitempty"`
}

type SessionEvent struct {
	MachineID MachineID    `json:"machine_id"`
	Method    AccessMethod `json:"method"`
	StartedAt time.Time    `json:"started_at"`
	EndedAt   time.Time    `json:"ended_at"`
	Success   bool         `json:"success"`
	ExitCode  int          `json:"exit_code,omitempty"`
	Error     string       `json:"error,omitempty"`
	Command   []string     `json:"command,omitempty"`
}

type ProviderStatus struct {
	Name    string        `json:"name"`
	Status  ProviderState `json:"status"`
	Message string        `json:"message,omitempty"`
}

func NewSSHMachineID(configPath, alias string) MachineID {
	sum := sha256.Sum256([]byte(configPath))
	return MachineID("ssh:" + hex.EncodeToString(sum[:])[:12] + ":" + alias)
}

func NewAWSSSMMachineID(account, region, nativeID string) MachineID {
	return MachineID("aws:ssm:" + account + ":" + region + ":" + nativeID)
}

func NewHealthObservation(id MachineID, method AccessMethod, status HealthStatus, label string, latency time.Duration, checkedAt time.Time) HealthObservation {
	if checkedAt.IsZero() {
		checkedAt = time.Now()
	}
	obs := HealthObservation{MachineID: id, Method: method, Status: status, Label: label, CheckedAt: checkedAt}
	if latency > 0 {
		obs.Latency = &latency
	}
	return obs
}

func (m *Machine) ApplyOverlay(overlay MachineOverlay) {
	if overlay.MachineID == "" || overlay.MachineID != m.ID {
		return
	}
	m.Pinned = overlay.Pinned
	m.Tags = append([]string(nil), overlay.Tags...)
	m.Note = overlay.Note
	m.PreferredMethod = overlay.PreferredMethod
	m.LastCheckedAt = overlay.LastCheckedAt
	m.LastConnectedAt = overlay.LastConnectedAt
	m.ConnectionCount = overlay.ConnectionCount
	m.HealthHistory = append([]HealthObservation(nil), overlay.HealthHistory...)
	m.SessionHistory = append([]SessionEvent(nil), overlay.SessionHistory...)
	if overlay.LastHealth.Label != "" || !overlay.LastHealth.CheckedAt.IsZero() {
		m.Health = overlay.LastHealth
	}
	if m.SelectedMethod == "" {
		m.SelectedMethod = m.DefaultMethod()
	}
}

func (m Machine) DefaultMethod() AccessMethod {
	if containsMethod(m.Methods, m.SelectedMethod) {
		return m.SelectedMethod
	}
	if containsMethod(m.Methods, m.PreferredMethod) {
		return m.PreferredMethod
	}
	if len(m.Methods) > 0 {
		return m.Methods[0]
	}
	return ""
}

func containsMethod(methods []AccessMethod, method AccessMethod) bool {
	if method == "" {
		return false
	}
	for _, m := range methods {
		if m == method {
			return true
		}
	}
	return false
}

func SortMachines(items []Machine) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.Pinned != b.Pinned {
			return a.Pinned
		}
		if (a.Health.Status == HealthUp) != (b.Health.Status == HealthUp) {
			return a.Health.Status == HealthUp
		}
		if a.LastConnectedAt.IsZero() != b.LastConnectedAt.IsZero() {
			return !a.LastConnectedAt.IsZero()
		}
		if !a.LastConnectedAt.Equal(b.LastConnectedAt) {
			return a.LastConnectedAt.After(b.LastConnectedAt)
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
}
