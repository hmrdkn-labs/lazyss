package domain

import (
	"strings"
	"testing"
	"time"
)

func TestStableMachineIDs(t *testing.T) {
	ssh := NewSSHMachineID("/Users/me/.ssh/config", "prod-web")
	if !strings.HasPrefix(string(ssh), "ssh:") || !strings.HasSuffix(string(ssh), ":prod-web") {
		t.Fatalf("unexpected ssh id: %s", ssh)
	}
	if ssh != NewSSHMachineID("/Users/me/.ssh/config", "prod-web") {
		t.Fatalf("ssh id not stable")
	}

	aws := NewAWSSSMMachineID("123456789012", "ap-southeast-1", "i-123")
	if got, want := string(aws), "aws:ssm:123456789012:ap-southeast-1:i-123"; got != want {
		t.Fatalf("aws id = %q, want %q", got, want)
	}
	if ssh == aws {
		t.Fatalf("ssh and aws ids must not dedupe")
	}
}

func TestApplyOverlayAndPreferredMethod(t *testing.T) {
	m := Machine{
		ID:      "ssh:abc:prod",
		Name:    "prod",
		Methods: []AccessMethod{AccessSSH, AccessAWSSSMShell},
	}
	overlay := MachineOverlay{
		MachineID:       m.ID,
		Pinned:          true,
		Hidden:          true,
		Tags:            []string{"prod", "web"},
		Note:            "critical",
		PreferredMethod: AccessAWSSSMShell,
	}
	m.ApplyOverlay(overlay)
	if !m.Pinned || !m.Hidden || m.Note != "critical" || m.DefaultMethod() != AccessAWSSSMShell {
		t.Fatalf("overlay not applied: %#v", m)
	}
}

func TestSortMachinesPinnedHealthyRecentName(t *testing.T) {
	now := time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC)
	items := []Machine{
		{ID: "ssh:c", Name: "zeta", LastConnectedAt: now.Add(-time.Hour)},
		{ID: "ssh:b", Name: "alpha", Health: HealthObservation{Status: HealthUnknown}},
		{ID: "ssh:a", Name: "beta", Pinned: true},
		{ID: "ssh:d", Name: "gamma", Health: HealthObservation{Status: HealthUp}},
	}
	SortMachines(items)
	got := []string{items[0].Name, items[1].Name, items[2].Name, items[3].Name}
	want := []string{"beta", "gamma", "zeta", "alpha"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sort order = %v, want %v", got, want)
		}
	}
}

func TestHealthLabelsAreMethodSpecific(t *testing.T) {
	ssh := NewHealthObservation("ssh:abc:prod", AccessSSH, HealthUp, "tcp prod.example:22", 25*time.Millisecond, time.Time{})
	if ssh.Label != "tcp prod.example:22" || ssh.Latency == nil {
		t.Fatalf("ssh health should preserve tcp label and latency: %#v", ssh)
	}
	ssm := NewHealthObservation("aws:ssm:1:r:i", AccessAWSSSMShell, HealthUp, "ssm Online ec2 running", 0, time.Time{})
	if ssm.Label != "ssm Online ec2 running" || ssm.Latency != nil {
		t.Fatalf("ssm health should preserve label without fake latency: %#v", ssm)
	}
}
