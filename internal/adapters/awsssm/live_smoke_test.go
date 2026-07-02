//go:build liveaws

package awsssm

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/hamardikan/lazyss/internal/app"
	"github.com/hamardikan/lazyss/internal/domain"
)

func TestLiveAWSInventoryWithProfile(t *testing.T) {
	profile := os.Getenv("LAZYSS_LIVE_AWS_PROFILE")
	if profile == "" {
		t.Skip("set LAZYSS_LIVE_AWS_PROFILE to run live AWS SSM smoke")
	}
	region := os.Getenv("LAZYSS_LIVE_AWS_REGION")

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	inventory, err := LoadInventory(ctx, profile, region)
	if err != nil {
		t.Fatalf("load AWS inventory with profile %q: %v", profile, err)
	}

	machines, status, err := inventory.ListMachines(ctx, app.InventoryQuery{Source: "aws"})
	if err != nil {
		t.Fatalf("list AWS SSM inventory with profile %q: status=%s message=%q err=%v", profile, status.Status, status.Message, err)
	}
	if status.Status != domain.ProviderHealthy {
		t.Fatalf("expected healthy AWS provider, got status=%s message=%q", status.Status, status.Message)
	}
	if len(machines) == 0 {
		t.Fatal("expected at least one AWS SSM managed node")
	}

	for _, machine := range machines {
		if machine.Provider != domain.ProviderAWS {
			t.Fatalf("expected AWS machine provider, got %q", machine.Provider)
		}
		if machine.Scope.Profile != profile {
			t.Fatalf("expected machine scope profile %q, got %q", profile, machine.Scope.Profile)
		}
		if machine.ID == "" || machine.NativeID == "" || machine.Name == "" {
			t.Fatal("expected AWS machine identity fields to be populated")
		}
	}
}
