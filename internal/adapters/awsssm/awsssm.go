// Package awsssm adapts AWS SSM and EC2 APIs to LazySS inventory, health, and
// connection ports.
package awsssm

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/hamardikan/lazyss/internal/app"
	"github.com/hamardikan/lazyss/internal/domain"
	"github.com/hamardikan/lazyss/internal/ports"
)

// SSMAPI is the subset of AWS SSM used by the inventory adapter.
type SSMAPI interface {
	DescribeInstanceInformation(ctx context.Context, in *ssm.DescribeInstanceInformationInput, optFns ...func(*ssm.Options)) (*ssm.DescribeInstanceInformationOutput, error)
}

// EC2API is the subset of AWS EC2 used by the inventory adapter.
type EC2API interface {
	DescribeInstances(ctx context.Context, in *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

// Inventory lists AWS SSM managed nodes enriched with EC2 metadata.
type Inventory struct {
	account string
	region  string
	profile string
	ssm     SSMAPI
	ec2     EC2API
}

// NewInventory creates an AWS inventory from explicit clients.
func NewInventory(account, region string, ssmClient SSMAPI, ec2Client EC2API) Inventory {
	return Inventory{account: account, region: region, ssm: ssmClient, ec2: ec2Client}
}

// NewInventoryWithProfile creates an AWS inventory with profile metadata.
func NewInventoryWithProfile(account, region, profile string, ssmClient SSMAPI, ec2Client EC2API) Inventory {
	return Inventory{account: account, region: region, profile: profile, ssm: ssmClient, ec2: ec2Client}
}

// LoadInventory loads AWS config and creates an inventory adapter.
func LoadInventory(ctx context.Context, profile, region string) (Inventory, error) {
	var opts []func(*config.LoadOptions) error
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return Inventory{}, err
	}
	account := ""
	identity, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err == nil {
		account = aws.ToString(identity.Account)
	}
	return NewInventoryWithProfile(account, cfg.Region, profile, ssm.NewFromConfig(cfg), ec2.NewFromConfig(cfg)), nil
}

// ProviderName returns the app-level AWS provider key.
func (i Inventory) ProviderName() string { return "aws" }

// ListMachines returns SSM managed nodes as provider-neutral machines.
func (i Inventory) ListMachines(ctx context.Context, q app.InventoryQuery) ([]domain.Machine, domain.ProviderStatus, error) {
	if i.ssm == nil || i.ec2 == nil {
		return nil, domain.ProviderStatus{Name: "aws", Status: domain.ProviderDegraded, Message: "aws clients not configured"}, errors.New("aws clients not configured")
	}
	ssmNodes, err := i.fetchSSM(ctx)
	if err != nil {
		return nil, domain.ProviderStatus{Name: "aws", Status: domain.ProviderDegraded, Message: err.Error()}, err
	}
	ec2Nodes, err := i.fetchEC2(ctx, q)
	if err != nil {
		return nil, domain.ProviderStatus{Name: "aws", Status: domain.ProviderDegraded, Message: err.Error()}, err
	}
	ec2ByID := map[string]ec2Node{}
	for _, e := range ec2Nodes {
		ec2ByID[e.id] = e
	}
	var out []domain.Machine
	for _, node := range ssmNodes {
		e := ec2ByID[node.id]
		name := e.name
		if name == "" {
			name = node.id
		}
		address := e.ip
		if address == "" {
			address = node.ip
		}
		status, label := awsHealth(node.ping, e.state)
		id := domain.NewAWSSSMMachineID(i.account, i.region, node.id)
		out = append(out, domain.Machine{
			ID:           id,
			Name:         name,
			Provider:     domain.ProviderAWS,
			NativeID:     node.id,
			Address:      address,
			Platform:     node.platform,
			State:        e.state,
			Scope:        domain.Scope{Account: i.account, Region: i.region, Profile: i.profile},
			ProviderTags: e.tags,
			Methods:      []domain.AccessMethod{domain.AccessAWSSSMShell},
			Health:       domain.NewHealthObservation(id, domain.AccessAWSSSMShell, status, label, 0, zeroTime()),
		})
	}
	domain.SortMachines(out)
	return out, domain.ProviderStatus{Name: "aws", Status: domain.ProviderHealthy}, nil
}

type ssmNode struct {
	id       string
	ping     string
	platform string
	ip       string
}

func (i Inventory) fetchSSM(ctx context.Context) ([]ssmNode, error) {
	var out []ssmNode
	var token *string
	for {
		resp, err := i.ssm.DescribeInstanceInformation(ctx, &ssm.DescribeInstanceInformationInput{NextToken: token})
		if err != nil {
			return nil, err
		}
		for _, info := range resp.InstanceInformationList {
			out = append(out, ssmNode{
				id:       aws.ToString(info.InstanceId),
				ping:     string(info.PingStatus),
				platform: string(info.PlatformType),
				ip:       aws.ToString(info.IPAddress),
			})
		}
		if resp.NextToken == nil || *resp.NextToken == "" {
			break
		}
		token = resp.NextToken
	}
	return out, nil
}

type ec2Node struct {
	id    string
	name  string
	ip    string
	state string
	tags  map[string]string
}

func (i Inventory) fetchEC2(ctx context.Context, q app.InventoryQuery) ([]ec2Node, error) {
	var out []ec2Node
	var token *string
	filters := buildEC2Filters(q)
	for {
		in := &ec2.DescribeInstancesInput{NextToken: token}
		if len(filters) > 0 {
			in.Filters = filters
		}
		resp, err := i.ec2.DescribeInstances(ctx, in)
		if err != nil {
			return nil, err
		}
		for _, reservation := range resp.Reservations {
			for _, inst := range reservation.Instances {
				out = append(out, fromEC2(inst))
			}
		}
		if resp.NextToken == nil || *resp.NextToken == "" {
			break
		}
		token = resp.NextToken
	}
	return out, nil
}

func buildEC2Filters(q app.InventoryQuery) []ec2types.Filter {
	var filters []ec2types.Filter
	keys := make([]string, 0, len(q.Tags))
	for key := range q.Tags {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		filters = append(filters, ec2types.Filter{Name: aws.String("tag:" + key), Values: []string{q.Tags[key]}})
	}
	if q.NamePrefix != "" {
		filters = append(filters, ec2types.Filter{Name: aws.String("tag:Name"), Values: []string{q.NamePrefix + "*"}})
	}
	return filters
}

func fromEC2(inst ec2types.Instance) ec2Node {
	tags := map[string]string{}
	for _, tag := range inst.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	ip := aws.ToString(inst.PrivateIpAddress)
	if ip == "" {
		ip = aws.ToString(inst.PublicIpAddress)
	}
	state := ""
	if inst.State != nil {
		state = string(inst.State.Name)
	}
	return ec2Node{id: aws.ToString(inst.InstanceId), name: tags["Name"], ip: ip, state: state, tags: tags}
}

func awsHealth(ping, state string) (domain.HealthStatus, string) {
	label := "ssm " + nonempty(ping, "Unknown")
	if state != "" {
		label += " ec2 " + strings.ToLower(state)
	}
	if strings.EqualFold(ping, string(ssmtypes.PingStatusOnline)) {
		return domain.HealthUp, label
	}
	return domain.HealthDown, label
}

func nonempty(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func zeroTime() (t time.Time) { return t }

// Runner runs AWS CLI commands for interactive SSM sessions.
type Runner interface {
	RunInteractive(ctx context.Context, executable string, args []string) error
	RunOutput(ctx context.Context, executable string, args []string) ([]byte, error)
}

// Connector builds and runs AWS SSM shell sessions.
type Connector struct {
	runner Runner
}

// NewConnector creates an AWS SSM connector.
func NewConnector(runner Runner) Connector {
	if runner == nil {
		runner = osRunner{}
	}
	return Connector{runner: runner}
}

// Supports reports whether this connector can launch the requested method.
func (c Connector) Supports(machine domain.Machine, method domain.AccessMethod) bool {
	return method == domain.AccessAWSSSMShell && (machine.Provider == domain.ProviderAWS || hasMethod(machine, method))
}

// BuildCommand builds an `aws ssm start-session` argv.
func (c Connector) BuildCommand(machine domain.Machine, method domain.AccessMethod, _ app.ConnectOptions) (ports.CommandSpec, error) {
	if !c.Supports(machine, method) {
		return ports.CommandSpec{}, errors.New("aws ssm connector does not support method")
	}
	target := machine.NativeID
	if target == "" {
		target = machine.Name
	}
	args := []string{"ssm", "start-session", "--target", target}
	if machine.Scope.Profile != "" {
		args = append(args, "--profile", machine.Scope.Profile)
	}
	if machine.Scope.Region != "" {
		args = append(args, "--region", machine.Scope.Region)
	}
	return ports.CommandSpec{Executable: "aws", Args: args}, nil
}

// RunInteractive runs the AWS SSM session command.
func (c Connector) RunInteractive(ctx context.Context, cmd ports.CommandSpec) (app.SessionResult, error) {
	return app.SessionResult{}, c.runner.RunInteractive(ctx, cmd.Executable, cmd.Args)
}

// Checker turns AWS inventory readiness into health observations.
type Checker struct{}

// Supports reports whether this checker can evaluate the requested method.
func (Checker) Supports(machine domain.Machine, method domain.AccessMethod) bool {
	return method == domain.AccessAWSSSMShell && machine.Provider == domain.ProviderAWS
}

// Check returns the latest AWS SSM health known from inventory.
func (Checker) Check(_ context.Context, machine domain.Machine, method domain.AccessMethod) domain.HealthObservation {
	obs := machine.Health
	if obs.MachineID == "" {
		obs = domain.NewHealthObservation(machine.ID, method, domain.HealthUnknown, "ssm not checked", 0, time.Now())
	}
	obs.CheckedAt = time.Now()
	return obs
}

type osRunner struct{}

func (osRunner) RunInteractive(ctx context.Context, executable string, args []string) error {
	cmd := execCommandContext(ctx, executable, args...)
	return cmd.Run()
}

func (osRunner) RunOutput(ctx context.Context, executable string, args []string) ([]byte, error) {
	return execCommandContext(ctx, executable, args...).Output()
}

func hasMethod(machine domain.Machine, method domain.AccessMethod) bool {
	for _, m := range machine.Methods {
		if m == method {
			return true
		}
	}
	return false
}
