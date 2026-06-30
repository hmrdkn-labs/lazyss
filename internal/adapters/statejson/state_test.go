package statejson

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hamardikan/lazyss/internal/domain"
)

func TestStoreWritesAtomicStateWith0600AndCappedHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store := New(path, 2)
	ctx := context.Background()
	id := domain.MachineID("ssh:a:prod")
	if err := store.SaveOverlay(ctx, domain.MachineOverlay{MachineID: id, Pinned: true, Note: "watch"}); err != nil {
		t.Fatalf("save overlay: %v", err)
	}
	for i := 0; i < 3; i++ {
		err := store.RecordHealth(ctx, domain.NewHealthObservation(id, domain.AccessSSH, domain.HealthUp, "tcp host:22", time.Millisecond, time.Now()))
		if err != nil {
			t.Fatalf("record health: %v", err)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 0600", got)
	}
	overlay, err := store.LoadOverlay(ctx, id)
	if err != nil {
		t.Fatalf("load overlay: %v", err)
	}
	if !overlay.Pinned || overlay.Note != "watch" || len(overlay.HealthHistory) != 2 {
		t.Fatalf("overlay/history = %#v", overlay)
	}
}

func TestStoreReportsCorruptJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("{nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := New(path, 20).LoadOverlay(context.Background(), "ssh:a")
	if err == nil {
		t.Fatalf("expected corrupt json error")
	}
}
