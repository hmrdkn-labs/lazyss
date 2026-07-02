package main

import (
	"testing"

	"github.com/hamardikan/lazyss/internal/domain"
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
