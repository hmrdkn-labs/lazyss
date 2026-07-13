package sshconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hmrdkn-labs/lazyss/internal/ports"
)

func TestPlanCleanupClassifiesSCMForwardDuplicateAndMachines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	input := `
Host github-work
  HostName github.com
  User git
  IdentityFile ~/.ssh/id_ed25519

Host llm-api
  HostName workstation.example.com
  User hmrdkn
  LocalForward 4000 localhost:4000

Host ts-workstation
  HostName 100.77.140.101
  User hmrdkn

Host ts-workstation-name
  HostName 100.77.140.101
  User hmrdkn

Host prod
  HostName prod.example.com
  User ubuntu
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}

	plan, err := NewCleaner(path).PlanCleanup(ports.CleanupOptions{})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	got := map[string]ports.CleanupItem{}
	for _, item := range plan.Items {
		got[item.Host] = item
	}
	assertCleanupItem(t, got["github-work"], ports.CleanupKeep, "scm identity")
	assertCleanupItem(t, got["llm-api"], ports.CleanupHide, "port forward alias")
	assertCleanupItem(t, got["ts-workstation-name"], ports.CleanupDeleteCandidate, "duplicate target")
	assertCleanupItem(t, got["prod"], ports.CleanupKeep, "machine")
}

func TestApplyCleanupRemovesSelectedHostWithBackupAndKeepsProtectedHosts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	keyPath := filepath.Join(dir, "id_ed25519")
	if err := os.WriteFile(keyPath, []byte("private-key-placeholder"), 0o600); err != nil {
		t.Fatal(err)
	}
	input := `
Host github-work
  HostName github.com
  User git
  IdentityFile ` + keyPath + `

Host stale
  HostName stale.example.com
  User ubuntu
  IdentityFile ` + keyPath + `

Host keep
  HostName keep.example.com
  User ubuntu
`
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := NewCleaner(path).ApplyCleanup(ports.CleanupApplyOptions{
		Hosts: []string{"stale", "github-work"},
		Now:   time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatalf("expected protected host error")
	}
	if !strings.Contains(err.Error(), "protected") {
		t.Fatalf("error = %v", err)
	}
	unchanged, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(unchanged) != input {
		t.Fatalf("protected selection should not mutate config:\n%s", unchanged)
	}

	result, err := NewCleaner(path).ApplyCleanup(ports.CleanupApplyOptions{
		Hosts: []string{"stale"},
		Now:   time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("apply cleanup: %v", err)
	}
	if len(result.RemovedHosts) != 1 || result.RemovedHosts[0] != "stale" {
		t.Fatalf("removed hosts = %#v", result.RemovedHosts)
	}
	if result.BackupPath == "" {
		t.Fatalf("backup path missing")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(after), "Host stale") {
		t.Fatalf("stale host was not removed:\n%s", after)
	}
	if !strings.Contains(string(after), "Host github-work") || !strings.Contains(string(after), "Host keep") {
		t.Fatalf("protected or keep host removed:\n%s", after)
	}
	if _, err := os.Stat(keyPath); err != nil {
		t.Fatalf("key file should never be removed: %v", err)
	}
	backup, err := os.ReadFile(result.BackupPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != input {
		t.Fatalf("backup content changed")
	}
}

func assertCleanupItem(t *testing.T, item ports.CleanupItem, action ports.CleanupAction, reason string) {
	t.Helper()
	if item.Action != action || item.Reason != reason {
		t.Fatalf("%s action=%q reason=%q, want %q %q", item.Host, item.Action, item.Reason, action, reason)
	}
}
