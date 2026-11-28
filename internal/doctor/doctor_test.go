package doctor

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hmrdkn-labs/lazyss/internal/domain"
)

func TestDoctorReportsMissingBinariesAndCredentials(t *testing.T) {
	d := Doctor{
		LookPath: func(_ string) (string, error) { return "", errors.New("missing") },
		Identity: func(context.Context) error { return errors.New("expired credentials") },
		Region:   "",
	}
	checks := d.Run(context.Background())
	if len(checks) < 4 {
		t.Fatalf("checks = %#v", checks)
	}
	for _, c := range checks {
		if c.OK {
			t.Fatalf("expected failures, got %#v", checks)
		}
	}
}

func TestDoctorCloudCLIsMissingAreOptionalUnlessRequired(t *testing.T) {
	missing := func(string) (string, error) { return "", errors.New("missing") }
	called := false
	status := func(context.Context) domain.ProviderStatus { called = true; return domain.ProviderStatus{} }
	checks := Doctor{LookPath: missing, GCP: status, Azure: status, RequireAzure: true}.Run(context.Background())
	byName := map[string]Check{}
	for _, c := range checks {
		byName[c.Name] = c
	}
	g, a := byName["gcloud"], byName["azure"]
	if g.OK || !g.Optional || !strings.Contains(g.Detail, "not installed; optional, needed for --source gcp") {
		t.Fatalf("gcloud = %#v", g)
	}
	if a.OK || a.Optional || !strings.Contains(a.Detail, "--source azure") {
		t.Fatalf("azure = %#v", a)
	}
	if called {
		t.Fatalf("status must not run when CLI is missing")
	}
}

func TestDoctorCloudStatusRedactsSubscriptionID(t *testing.T) {
	found := func(string) (string, error) { return "/bin/x", nil }
	d := Doctor{
		LookPath: found,
		GCP: func(context.Context) domain.ProviderStatus {
			return domain.ProviderStatus{Status: domain.ProviderDegraded, Message: "gcloud not logged in"}
		},
		Azure: func(context.Context) domain.ProviderStatus {
			return domain.ProviderStatus{Status: domain.ProviderHealthy, Message: "az logged in: subscription Prod (11111111-2222-3333-4444-555555555555); ssh extension installed"}
		},
	}
	for _, c := range d.Run(context.Background()) {
		switch c.Name {
		case "azure":
			if !c.OK || strings.Contains(c.Detail, "1111") || !strings.Contains(c.Detail, "Prod") {
				t.Fatalf("azure = %#v", c)
			}
		case "gcloud":
			if c.OK || !c.Optional || c.Fix == "" {
				t.Fatalf("gcloud = %#v", c)
			}
		}
	}
}

func TestDoctorAzureMissingSSHExtensionFixInstallsExtension(t *testing.T) {
	found := func(string) (string, error) { return "/bin/az", nil }
	azure := func(context.Context) domain.ProviderStatus {
		return domain.ProviderStatus{Status: domain.ProviderDegraded, Message: "az logged in: subscription Prod (11111111-2222-3333-4444-555555555555); az ssh extension missing: run az extension add --name ssh"}
	}
	checks := Doctor{LookPath: found, Azure: azure}.Run(context.Background())
	var a Check
	for _, c := range checks {
		if c.Name == "azure" {
			a = c
		}
	}
	if a.OK || a.Fix != "az extension add --name ssh" {
		t.Fatalf("azure = %#v", a)
	}
}
