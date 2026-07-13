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

	"github.com/hmrdkn-labs/lazyss/internal/adapters/awsconfig"
	"github.com/hmrdkn-labs/lazyss/internal/adapters/awsssm"
	"github.com/hmrdkn-labs/lazyss/internal/adapters/sshconfig"
	"github.com/hmrdkn-labs/lazyss/internal/adapters/statejson"
	"github.com/hmrdkn-labs/lazyss/internal/app"
	"github.com/hmrdkn-labs/lazyss/internal/brand"
	"github.com/hmrdkn-labs/lazyss/internal/doctor"
	"github.com/hmrdkn-labs/lazyss/internal/domain"
	"github.com/hmrdkn-labs/lazyss/internal/ports"
	"github.com/hmrdkn-labs/lazyss/internal/tui"
)

var version = "dev"

type cliConfig struct {
	Source         string
	SSHConfig      string
	AWSProfile     string
	AWSRegion      string
	CleanupHosts   []string
	CleanupWrite   bool
	CleanupCheck   bool
	CleanupTimeout time.Duration
	LogoText       string
	Version        bool
}

func main() {
	cfg, command, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if cfg.Version {
		fmt.Println(brand.ShortVersion(version))
		return
	}
	switch command {
	case "version":
		fmt.Print(brand.VersionReport(version))
	case "logo":
		fmt.Print(brand.Logo(cfg.LogoText))
	case "doctor":
		os.Exit(runDoctor(cfg))
	case "ssh cleanup":
		os.Exit(runSSHCleanup(cfg))
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
	remaining, err := parseFlagSegment(&cfg, args)
	if err != nil {
		return cfg, "", err
	}
	command := ""
	if len(remaining) > 0 {
		command = remaining[0]
		commandArgs := remaining[1:]
		if command == "ssh" {
			if len(commandArgs) == 0 {
				return cfg, "", fmt.Errorf("expected ssh subcommand")
			}
			if commandArgs[0] != "cleanup" {
				return cfg, "", fmt.Errorf("unknown ssh subcommand %q", commandArgs[0])
			}
			command = "ssh cleanup"
			commandArgs = commandArgs[1:]
		}
		trailing, err := parseFlagSegment(&cfg, commandArgs)
		if err != nil {
			return cfg, "", err
		}
		if len(trailing) > 0 {
			return cfg, "", fmt.Errorf("unexpected argument %q", trailing[0])
		}
	}
	switch cfg.Source {
	case "all", "ssh", "aws":
	default:
		return cfg, "", fmt.Errorf("invalid --source %q", cfg.Source)
	}
	return cfg, command, nil
}

type stringListFlag []string

func (s *stringListFlag) String() string {
	return fmt.Sprint([]string(*s))
}

func (s *stringListFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func parseFlagSegment(cfg *cliConfig, args []string) ([]string, error) {
	fs := flag.NewFlagSet("lazyss", flag.ContinueOnError)
	fs.StringVar(&cfg.Source, "source", cfg.Source, "inventory source: all, ssh, aws")
	fs.StringVar(&cfg.SSHConfig, "ssh-config", cfg.SSHConfig, "SSH config path")
	fs.StringVar(&cfg.AWSProfile, "aws-profile", cfg.AWSProfile, "AWS profile")
	fs.StringVar(&cfg.AWSRegion, "aws-region", cfg.AWSRegion, "AWS region")
	fs.Var((*stringListFlag)(&cfg.CleanupHosts), "host", "SSH host to remove with ssh cleanup --write; repeatable")
	fs.BoolVar(&cfg.CleanupWrite, "write", cfg.CleanupWrite, "write SSH cleanup changes")
	fs.BoolVar(&cfg.CleanupCheck, "check", cfg.CleanupCheck, "check SSH host TCP reachability in cleanup report")
	fs.DurationVar(&cfg.CleanupTimeout, "timeout", cfg.CleanupTimeout, "SSH cleanup reachability timeout")
	fs.StringVar(&cfg.LogoText, "text", cfg.LogoText, "custom text for lazyss logo")
	fs.BoolVar(&cfg.Version, "version", cfg.Version, "print version")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return fs.Args(), nil
}

func runTUI(cfg cliConfig) int {
	statePath, err := statejson.DefaultPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "state path:", err)
		return 1
	}
	store := statejson.New(statePath, 20)
	prefs, err := store.LoadPreferences(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "preferences:", err)
	}
	effectiveCfg := resolveAWSConfig(cfg, prefs)
	providers := buildProviders(effectiveCfg)
	awsCLI := awsconfig.NewCLI(nil)
	connectors := []ports.Connector{sshconfig.NewConnector(nil, nil), awsssm.NewConnector(nil)}
	checkers := []ports.HealthChecker{
		sshconfig.NewChecker(nil, nil, 3*time.Second),
		awsssm.Checker{},
	}
	var cleanup *app.CleanupService
	if effectiveCfg.SSHConfig != "" {
		cleanup = &app.CleanupService{Planner: sshconfig.NewCleaner(effectiveCfg.SSHConfig)}
	}
	runtime := &tui.Runtime{
		Inventory:   &app.InventoryService{Providers: providers, Store: store},
		Connect:     &app.ConnectService{Connectors: connectors, Store: store},
		Health:      &app.HealthService{Checkers: checkers, Store: store, MaxConcurrent: 8},
		Cleanup:     cleanup,
		Query:       app.InventoryQuery{Source: effectiveCfg.Source},
		Copy:        clipboard.WriteAll,
		Preferences: store,
		AWSProfiles: awsCLI,
		AWSLogin:    awsCLI,
		AWSProfile:  effectiveCfg.AWSProfile,
		AWSRegion:   effectiveCfg.AWSRegion,
		SetAWSProfile: func(_ context.Context, profile string) (*app.InventoryService, error) {
			nextCfg := effectiveCfg
			nextCfg.AWSProfile = profile
			effectiveCfg.AWSProfile = profile
			return &app.InventoryService{Providers: buildProviders(nextCfg), Store: store}, nil
		},
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

func resolveAWSConfig(cfg cliConfig, prefs domain.OperatorPreferences) cliConfig {
	if cfg.AWSProfile == "" {
		cfg.AWSProfile = prefs.AWSProfile
	}
	if cfg.AWSRegion == "" {
		cfg.AWSRegion = prefs.AWSRegion
	}
	return cfg
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

func runSSHCleanup(cfg cliConfig) int {
	if cfg.SSHConfig == "" {
		fmt.Fprintln(os.Stderr, "ssh cleanup: --ssh-config is required")
		return 2
	}
	svc := app.CleanupService{Planner: sshconfig.NewCleaner(cfg.SSHConfig)}
	if cfg.CleanupWrite {
		result, err := svc.Apply(ports.CleanupApplyOptions{Hosts: cfg.CleanupHosts})
		if err != nil {
			fmt.Fprintln(os.Stderr, "ssh cleanup:", err)
			return 1
		}
		fmt.Println("lazyss ssh cleanup")
		fmt.Println("mode: write")
		fmt.Println("backup:", result.BackupPath)
		if len(result.RemovedHosts) == 0 {
			fmt.Println("removed: none")
			return 0
		}
		for _, host := range result.RemovedHosts {
			fmt.Println("removed:", host)
		}
		return 0
	}

	plan, err := svc.Plan(ports.CleanupOptions{Check: cfg.CleanupCheck, Timeout: cfg.CleanupTimeout})
	if err != nil {
		fmt.Fprintln(os.Stderr, "ssh cleanup:", err)
		return 1
	}
	fmt.Println("lazyss ssh cleanup")
	fmt.Println("mode: dry-run")
	if cfg.CleanupCheck {
		fmt.Println("check: tcp")
	}
	for _, item := range plan.Items {
		line := fmt.Sprintf("  %-22s %-16s %-18s %s", item.Host, item.Action, item.Reason, cleanupTarget(item))
		if item.Protected {
			line += " protected"
		}
		if cfg.CleanupCheck {
			line += " check=" + nonempty(item.Check, "unchecked")
			if item.CheckErr != "" {
				line += " error=" + item.CheckErr
			}
		}
		fmt.Println(line)
	}
	fmt.Println("write: lazyss ssh cleanup --host <name> --write")
	return 0
}

func cleanupTarget(item ports.CleanupItem) string {
	if item.HostName == "" {
		return "-"
	}
	return fmt.Sprintf("%s@%s:%d", nonempty(item.User, "-"), item.HostName, item.Port)
}

func nonempty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

type failingProvider struct {
	name string
	err  error
}

func (f failingProvider) ProviderName() string { return f.name }

func (f failingProvider) ListMachines(context.Context, app.InventoryQuery) ([]domain.Machine, domain.ProviderStatus, error) {
	return nil, domain.ProviderStatus{Name: f.name, Status: domain.ProviderDegraded, Message: f.err.Error()}, f.err
}
