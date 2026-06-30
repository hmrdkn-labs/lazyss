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
