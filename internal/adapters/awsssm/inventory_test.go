package awsssm

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	smithy "github.com/aws/smithy-go"
	"github.com/hamardikan/lazyss/internal/app"
	"github.com/hamardikan/lazyss/internal/domain"
)

func TestInventoryPaginatesAndMapsSSMEC2Readiness(t *testing.T) {
	provider := NewInventory("123456789012", "ap-southeast-1", fakeSSM{
		pages: []*ssm.DescribeInstanceInformationOutput{
			{InstanceInformationList: []ssmtypes.InstanceInformation{{InstanceId: aws.String("i-1"), PingStatus: ssmtypes.PingStatusOnline}}, NextToken: aws.String("n")},
			{InstanceInformationList: []ssmtypes.InstanceInformation{{InstanceId: aws.String("i-2"), PingStatus: ssmtypes.PingStatusConnectionLost}}},
		},
	}, fakeEC2{out: &ec2.DescribeInstancesOutput{Reservations: []ec2types.Reservation{{Instances: []ec2types.Instance{
		{InstanceId: aws.String("i-1"), PrivateIpAddress: aws.String("10.0.0.1"), State: &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning}, Tags: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("web-1")}}},
		{InstanceId: aws.String("i-2"), State: &ec2types.InstanceState{Name: ec2types.InstanceStateNameStopped}, Tags: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("web-2")}}},
	}}}}})
	machines, status, err := provider.ListMachines(context.Background(), app.InventoryQuery{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if status.Status != domain.ProviderHealthy || len(machines) != 2 {
		t.Fatalf("status/machines = %#v %#v", status, machines)
	}
	if machines[0].Health.Label != "ssm Online ec2 running" || machines[1].Health.Status != domain.HealthDown {
		t.Fatalf("readiness mapping = %#v", machines)
	}
}

func TestConnectorBuildsStartSessionArgv(t *testing.T) {
	conn := NewConnector(fakeRunner{})
	m := domain.Machine{NativeID: "i-123", Methods: []domain.AccessMethod{domain.AccessAWSSSMShell}, Scope: domain.Scope{Profile: "prod", Region: "ap-southeast-1"}}
	cmd, err := conn.BuildCommand(m, domain.AccessAWSSSMShell, app.ConnectOptions{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	want := []string{"ssm", "start-session", "--target", "i-123", "--profile", "prod", "--region", "ap-southeast-1"}
	if cmd.Executable != "aws" || len(cmd.Args) != len(want) {
		t.Fatalf("cmd = %#v", cmd)
	}
	for i := range want {
		if cmd.Args[i] != want[i] {
			t.Fatalf("args = %#v, want %#v", cmd.Args, want)
		}
	}
}

func TestInventoryPassesEC2Filters(t *testing.T) {
	ec2Client := &capturingEC2{out: &ec2.DescribeInstancesOutput{}}
	provider := NewInventory("123456789012", "ap-southeast-1", fakeSSM{pages: []*ssm.DescribeInstanceInformationOutput{{}}}, ec2Client)
	_, _, err := provider.ListMachines(context.Background(), app.InventoryQuery{
		Tags:       map[string]string{"Env": "prod", "Team": "platform"},
		NamePrefix: "web-",
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	got := ec2Client.in.Filters
	if len(got) != 3 {
		t.Fatalf("filters = %#v", got)
	}
	if aws.ToString(got[0].Name) != "tag:Env" || got[0].Values[0] != "prod" {
		t.Fatalf("first filter = %#v", got[0])
	}
	if aws.ToString(got[2].Name) != "tag:Name" || got[2].Values[0] != "web-*" {
		t.Fatalf("name filter = %#v", got[2])
	}
}

func TestInventorySummarizesAWSAuthErrors(t *testing.T) {
	provider := NewInventory("123456789012", "ap-southeast-1", fakeSSM{
		err: &smithy.OperationError{
			ServiceID:     "SSM",
			OperationName: "DescribeInstanceInformation",
			Err: &smithy.GenericAPIError{
				Code:    "UnrecognizedClientException",
				Message: "The security token included in the request is invalid.",
			},
		},
	}, fakeEC2{out: &ec2.DescribeInstancesOutput{}})
	_, status, err := provider.ListMachines(context.Background(), app.InventoryQuery{})
	if err == nil {
		t.Fatalf("expected inventory error")
	}
	if status.Status != domain.ProviderDegraded {
		t.Fatalf("status = %#v", status)
	}
	if status.Message != "auth failed; refresh AWS credentials (UnrecognizedClientException)" {
		t.Fatalf("message = %q", status.Message)
	}
}

type fakeSSM struct {
	pages []*ssm.DescribeInstanceInformationOutput
	err   error
}

func (f fakeSSM) DescribeInstanceInformation(_ context.Context, in *ssm.DescribeInstanceInformationInput, _ ...func(*ssm.Options)) (*ssm.DescribeInstanceInformationOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	if in.NextToken == nil {
		return f.pages[0], nil
	}
	return f.pages[1], nil
}

type fakeEC2 struct{ out *ec2.DescribeInstancesOutput }

func (f fakeEC2) DescribeInstances(context.Context, *ec2.DescribeInstancesInput, ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return f.out, nil
}

type capturingEC2 struct {
	in  *ec2.DescribeInstancesInput
	out *ec2.DescribeInstancesOutput
}

func (c *capturingEC2) DescribeInstances(_ context.Context, in *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	c.in = in
	return c.out, nil
}
