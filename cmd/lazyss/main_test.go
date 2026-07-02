package main

import (
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
	cfg := resolveAWSConfig(cliConfig{AWSProfile: "flagged", AWSRegion: "us-east-1"}, prefs)
	if cfg.AWSProfile != "flagged" || cfg.AWSRegion != "us-east-1" {
		t.Fatalf("cfg = %#v", cfg)
	}
}

func TestResolveAWSConfigUsesPersistedPreferencesWhenFlagsMissing(t *testing.T) {
	prefs := domain.OperatorPreferences{AWSProfile: "hmrdkn-dev1", AWSRegion: "ap-southeast-1"}
	cfg := resolveAWSConfig(cliConfig{}, prefs)
	if cfg.AWSProfile != "hmrdkn-dev1" || cfg.AWSRegion != "ap-southeast-1" {
		t.Fatalf("cfg = %#v", cfg)
	}
}
