package sshconfig

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hamardikan/lazyss/internal/app"
	"github.com/hamardikan/lazyss/internal/domain"
)

func TestInventoryReadsSSHConfigWithoutWriting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	input := "Host prod\n  HostName prod.example.com\n  User ubuntu\n  Port 2222\n  IdentityFile ~/.ssh/id_ed25519\n"
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	provider := NewInventory(path)
	machines, status, err := provider.ListMachines(context.Background(), app.InventoryQuery{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != input {
		t.Fatalf("ssh config was mutated")
	}
	if status.Status != domain.ProviderHealthy || len(machines) != 1 {
		t.Fatalf("status/machines = %#v %#v", status, machines)
	}
	m := machines[0]
	if m.Name != "prod" || m.Address != "prod.example.com" || m.User != "ubuntu" || m.Port != 2222 {
		t.Fatalf("machine mapping = %#v", m)
	}
}
