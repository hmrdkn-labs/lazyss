// Package awsconfig adapts safe AWS CLI configuration operations.
package awsconfig

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/hamardikan/lazyss/internal/ports"
)

// CLI lists AWS profiles and launches SSO login through the AWS CLI.
type CLI struct {
	runner ports.CommandRunner
}

// NewCLI creates an AWS CLI adapter.
func NewCLI(runner ports.CommandRunner) CLI {
	if runner == nil {
		runner = osRunner{}
	}
	return CLI{runner: runner}
}

// ListProfiles returns configured AWS profile names without reading credential
// material.
func (c CLI) ListProfiles(ctx context.Context) ([]string, error) {
	out, err := c.runner.RunOutput(ctx, "aws", []string{"configure", "list-profiles"})
	if err != nil {
		return nil, err
	}
	var profiles []string
	for _, line := range strings.Split(string(out), "\n") {
		profile := strings.TrimSpace(line)
		if profile != "" {
			profiles = append(profiles, profile)
		}
	}
	return profiles, nil
}

// Login opens AWS SSO login for one explicit profile.
func (c CLI) Login(ctx context.Context, profile string) error {
	profile = strings.TrimSpace(profile)
	if profile == "" {
		return errors.New("aws profile is required for sso login")
	}
	return c.runner.RunInteractive(ctx, "aws", []string{"sso", "login", "--profile", profile})
}

type osRunner struct{}

func (osRunner) RunOutput(ctx context.Context, executable string, args []string) ([]byte, error) {
	return exec.CommandContext(ctx, executable, args...).Output()
}

func (osRunner) RunInteractive(ctx context.Context, executable string, args []string) error {
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
