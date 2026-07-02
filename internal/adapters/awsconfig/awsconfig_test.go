package awsconfig

import (
	"context"
	"strings"
	"testing"
)

func TestCLIListsProfilesFromAWSCLI(t *testing.T) {
	runner := &fakeRunner{out: []byte("group8\n\ndefault\nhmrdkn-dev1\n")}
	cli := NewCLI(runner)
	profiles, err := cli.ListProfiles(context.Background())
	if err != nil {
		t.Fatalf("list profiles: %v", err)
	}
	want := []string{"group8", "default", "hmrdkn-dev1"}
	if strings.Join(profiles, ",") != strings.Join(want, ",") {
		t.Fatalf("profiles = %#v, want %#v", profiles, want)
	}
	if runner.outputExecutable != "aws" || strings.Join(runner.outputArgs, " ") != "configure list-profiles" {
		t.Fatalf("command = %s %#v", runner.outputExecutable, runner.outputArgs)
	}
}

func TestCLILoginRunsSSOLoginForProfile(t *testing.T) {
	runner := &fakeRunner{}
	cli := NewCLI(runner)
	if err := cli.Login(context.Background(), "hmrdkn-dev1"); err != nil {
		t.Fatalf("login: %v", err)
	}
	want := "sso login --profile hmrdkn-dev1"
	if runner.interactiveExecutable != "aws" || strings.Join(runner.interactiveArgs, " ") != want {
		t.Fatalf("command = %s %#v", runner.interactiveExecutable, runner.interactiveArgs)
	}
}

func TestCLILoginRejectsBlankProfile(t *testing.T) {
	err := NewCLI(&fakeRunner{}).Login(context.Background(), " ")
	if err == nil || !strings.Contains(err.Error(), "profile is required") {
		t.Fatalf("err = %v", err)
	}
}

type fakeRunner struct {
	out                   []byte
	err                   error
	outputExecutable      string
	outputArgs            []string
	interactiveExecutable string
	interactiveArgs       []string
}

func (f *fakeRunner) RunOutput(_ context.Context, executable string, args []string) ([]byte, error) {
	f.outputExecutable = executable
	f.outputArgs = append([]string(nil), args...)
	return f.out, f.err
}

func (f *fakeRunner) RunInteractive(_ context.Context, executable string, args []string) error {
	f.interactiveExecutable = executable
	f.interactiveArgs = append([]string(nil), args...)
	return f.err
}
