// Package doctor reports local tool and cloud credential readiness.
package doctor

import (
	"context"
	"os/exec"
)

// Check is one doctor readiness result.
type Check struct {
	Name   string
	OK     bool
	Detail string
	Fix    string
}

// Doctor runs local preflight checks.
type Doctor struct {
	LookPath func(string) (string, error)
	Identity func(context.Context) error
	Region   string
}

// Run executes doctor checks and returns all results.
func (d Doctor) Run(ctx context.Context) []Check {
	lookPath := d.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	checks := []Check{
		binaryCheck(lookPath, "ssh", "install OpenSSH or ensure ssh is on PATH"),
		binaryCheck(lookPath, "aws", "install AWS CLI v2"),
		binaryCheck(lookPath, "session-manager-plugin", "install the AWS Session Manager plugin"),
	}
	if d.Region == "" {
		checks = append(checks, Check{Name: "AWS region", Detail: "no region resolved", Fix: "pass --aws-region or set AWS_REGION"})
	} else {
		checks = append(checks, Check{Name: "AWS region", OK: true, Detail: d.Region})
	}
	if d.Identity != nil {
		if err := d.Identity(ctx); err != nil {
			checks = append(checks, Check{Name: "AWS identity", Detail: err.Error(), Fix: "refresh credentials or run aws sso login"})
		} else {
			checks = append(checks, Check{Name: "AWS identity", OK: true, Detail: "resolved"})
		}
	}
	return checks
}

func binaryCheck(lookPath func(string) (string, error), name, fix string) Check {
	path, err := lookPath(name)
	if err != nil {
		return Check{Name: name, Detail: "not found on PATH", Fix: fix}
	}
	return Check{Name: name, OK: true, Detail: path}
}
