// Package main wires the LazySS command-line entrypoint.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/hamardikan/lazyss/internal/adapters/awsssm"
	"github.com/hamardikan/lazyss/internal/adapters/sshconfig"
	"github.com/hamardikan/lazyss/internal/adapters/statejson"
	"github.com/hamardikan/lazyss/internal/app"
	"github.com/hamardikan/lazyss/internal/doctor"
	"github.com/hamardikan/lazyss/internal/domain"
	"github.com/hamardikan/lazyss/internal/ports"
	"github.com/hamardikan/lazyss/internal/tui"
)

var version = "dev"

type cliConfig struct {
	Source     string
	SSHConfig  string
	AWSProfile string
	AWSRegion  string
	Version    bool
}

func main() {
	cfg, command, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if cfg.Version {
		fmt.Println("lazyss", version)
		return
	}
	switch command {
	case "doctor":
		os.Exit(runDoctor(cfg))
	case "", "tui":
		os.Exit(runTUI(cfg))
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", command)
		os.Exit(2)
	}
}

func parseArgs(args []string) (cliConfig, string, error) {
	cfg := cliConfig{Source: "all"}
	home, err := os.UserHomeDir()
	if err == nil {
		cfg.SSHConfig = filepath.Join(home, ".ssh", "config")
	}
	fs := flag.NewFlagSet("lazyss", flag.ContinueOnError)
	fs.StringVar(&cfg.Source, "source", "all", "inventory source: all, ssh, aws")
	fs.StringVar(&cfg.SSHConfig, "ssh-config", cfg.SSHConfig, "SSH config path")
	fs.StringVar(&cfg.AWSProfile, "aws-profile", "", "AWS profile")
	fs.StringVar(&cfg.AWSRegion, "aws-region", "", "AWS region")
	fs.BoolVar(&cfg.Version, "version", false, "print version")
	if err := fs.Parse(args); err != nil {
		return cfg, "", err
	}
	switch cfg.Source {
	case "all", "ssh", "aws":
	default:
		return cfg, "", fmt.Errorf("invalid --source %q", cfg.Source)
	}
	command := ""
	if fs.NArg() > 0 {
		command = fs.Arg(0)
	}
	return cfg, command, nil
}

func runTUI(cfg cliConfig) int {
	statePath, err := statejson.DefaultPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "state path:", err)
		return 1
	}
	store := statejson.New(statePath, 20)
	providers := buildProviders(cfg)
	connectors := []ports.Connector{sshconfig.NewConnector(nil, nil), awsssm.NewConnector(nil)}
	checkers := []ports.HealthChecker{
		sshconfig.NewChecker(nil, nil, 3*time.Second),
		awsssm.Checker{},
	}
	runtime := &tui.Runtime{
		Inventory: &app.InventoryService{Providers: providers, Store: store},
		Connect:   &app.ConnectService{Connectors: connectors, Store: store},
		Health:    &app.HealthService{Checkers: checkers, Store: store, MaxConcurrent: 8},
		Query:     app.InventoryQuery{Source: cfg.Source},
		Copy:      clipboard.WriteAll,
	}
	if _, err := tea.NewProgram(tui.NewModel(runtime)).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tui:", err)
		return 1
	}
	return 0
}

func buildProviders(cfg cliConfig) []ports.InventoryProvider {
	var providers []ports.InventoryProvider
	if cfg.Source == "all" || cfg.Source == "ssh" {
		providers = append(providers, sshconfig.NewInventory(cfg.SSHConfig))
	}
	if cfg.Source == "all" || cfg.Source == "aws" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		inv, err := awsssm.LoadInventory(ctx, cfg.AWSProfile, cfg.AWSRegion)
		if err != nil {
			providers = append(providers, failingProvider{name: "aws", err: err})
		} else {
			providers = append(providers, inv)
		}
	}
	return providers
}

func runDoctor(cfg cliConfig) int {
	checks := doctor.Doctor{
		Region: cfg.AWSRegion,
		Identity: func(ctx context.Context) error {
			var opts []func(*config.LoadOptions) error
			if cfg.AWSProfile != "" {
				opts = append(opts, config.WithSharedConfigProfile(cfg.AWSProfile))
			}
			if cfg.AWSRegion != "" {
				opts = append(opts, config.WithRegion(cfg.AWSRegion))
			}
			awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
			if err != nil {
				return err
			}
			_, err = sts.NewFromConfig(awsCfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
			return err
		},
	}.Run(context.Background())

	fmt.Println("lazyss doctor")
	ok := true
	for _, check := range checks {
		mark := "ok"
		if !check.OK {
			mark = "fail"
			ok = false
		}
		fmt.Printf("  [%s] %s - %s\n", mark, check.Name, check.Detail)
		if !check.OK && check.Fix != "" {
			fmt.Printf("        fix: %s\n", check.Fix)
		}
	}
	if !ok {
		return 1
	}
	return 0
}

type failingProvider struct {
	name string
	err  error
}

func (f failingProvider) ProviderName() string { return f.name }

func (f failingProvider) ListMachines(context.Context, app.InventoryQuery) ([]domain.Machine, domain.ProviderStatus, error) {
	return nil, domain.ProviderStatus{Name: f.name, Status: domain.ProviderDegraded, Message: f.err.Error()}, f.err
}
