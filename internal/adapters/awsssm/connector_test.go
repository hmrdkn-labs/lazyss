package awsssm

import "context"

type fakeRunner struct{ err error }

func (f fakeRunner) RunInteractive(context.Context, string, []string) error { return f.err }
func (f fakeRunner) RunOutput(context.Context, string, []string) ([]byte, error) {
	return nil, f.err
}
