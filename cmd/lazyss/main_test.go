package main

import "testing"

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
