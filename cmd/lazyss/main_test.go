package main

import (
	"strings"
	"testing"

	"github.com/hmrdkn-labs/lazyss/internal/domain"
)

func TestParseArgs(t *testing.T) {
	cfg, cmd, err := parseArgs([]string{"--source", "ssh", "--ssh-config", "/tmp/config", "doctor"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cmd != "doctor" || cfg.Source != "ssh" || cfg.SSHConfig != "/tmp/config" {
		t.Fatalf("cfg=%#v cmd=%q", cfg, cmd)
	}
}

func TestParseArgsAcceptsFlagsAfterCommand(t *testing.T) {
	cfg, cmd, err := parseArgs([]string{"doctor", "--aws-profile", "prod", "--aws-region", "ap-southeast-1"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cmd != "doctor" || cfg.AWSProfile != "prod" || cfg.AWSRegion != "ap-southeast-1" {
		t.Fatalf("cfg=%#v cmd=%q", cfg, cmd)
	}
}

func TestParseArgsAcceptsSSHCleanupCommand(t *testing.T) {
	cfg, cmd, err := parseArgs([]string{"ssh", "cleanup", "--ssh-config", "/tmp/config", "--host", "stale", "--host", "old", "--write", "--check"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cmd != "ssh cleanup" || cfg.SSHConfig != "/tmp/config" || !cfg.CleanupWrite || !cfg.CleanupCheck {
		t.Fatalf("cfg=%#v cmd=%q", cfg, cmd)
	}
	if got := cfg.CleanupHosts; len(got) != 2 || got[0] != "stale" || got[1] != "old" {
		t.Fatalf("cleanup hosts = %#v", got)
	}
}

func TestParseArgsAcceptsLogoCommand(t *testing.T) {
	cfg, cmd, err := parseArgs([]string{"logo", "--text", "Ops Access"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cmd != "logo" || cfg.LogoText != "Ops Access" {
		t.Fatalf("cfg=%#v cmd=%q", cfg, cmd)
	}
}

func TestParseArgsAcceptsVersionCommand(t *testing.T) {
	_, cmd, err := parseArgs([]string{"version"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cmd != "version" {
		t.Fatalf("cmd = %q", cmd)
	}
}

func TestResolveAWSConfigUsesCLIOverrideBeforePreferences(t *testing.T) {
	prefs := domain.OperatorPreferences{AWSProfile: "persisted", AWSRegion: "ap-southeast-1"}
	cfg := resolveConfig(cliConfig{AWSProfile: "flagged", AWSRegion: "us-east-1"}, prefs)
	if cfg.AWSProfile != "flagged" || cfg.AWSRegion != "us-east-1" {
		t.Fatalf("cfg = %#v", cfg)
	}
}

func TestResolveAWSConfigUsesPersistedPreferencesWhenFlagsMissing(t *testing.T) {
	prefs := domain.OperatorPreferences{AWSProfile: "hmrdkn-dev1", AWSRegion: "ap-southeast-1"}
	cfg := resolveConfig(cliConfig{}, prefs)
	if cfg.AWSProfile != "hmrdkn-dev1" || cfg.AWSRegion != "ap-southeast-1" {
		t.Fatalf("cfg = %#v", cfg)
	}
}

func TestParseArgsAcceptsCloudSourcesAndFlags(t *testing.T) {
	for _, source := range []string{"gcp", "azure"} {
		cfg, _, err := parseArgs([]string{"--source", source, "--gcp-project", "proj-a", "--azure-subscription", "sub-1"})
		if err != nil {
			t.Fatalf("%s: %v", source, err)
		}
		if cfg.Source != source || cfg.GCPProject != "proj-a" || cfg.AzureSub != "sub-1" {
			t.Fatalf("cfg = %#v", cfg)
		}
	}
	if _, _, err := parseArgs([]string{"--source", "digitalocean"}); err == nil {
		t.Fatalf("invalid source should fail")
	}
}

func TestResolveConfigUsesCloudPreferencesBelowFlags(t *testing.T) {
	prefs := domain.OperatorPreferences{GCPProject: "pref-proj", AzureSubscription: "pref-sub"}
	cfg := resolveConfig(cliConfig{GCPProject: "flag-proj"}, prefs)
	if cfg.GCPProject != "flag-proj" || cfg.AzureSub != "pref-sub" {
		t.Fatalf("cfg = %#v", cfg)
	}
}

func TestBuildProvidersRegistersCloudProvidersWithoutCLIs(t *testing.T) {
	var names []string
	for _, p := range buildProviders(cliConfig{Source: "all", SSHConfig: "/nonexistent", AWSRegion: "us-east-1"}) {
		names = append(names, p.ProviderName())
	}
	got := strings.Join(names, ",")
	if !strings.HasSuffix(got, "gcp,azure") {
		t.Fatalf("providers = %s", got)
	}
	only := buildProviders(cliConfig{Source: "azure"})
	if len(only) != 1 || only[0].ProviderName() != "azure" {
		t.Fatalf("azure only = %#v", only)
	}
}
