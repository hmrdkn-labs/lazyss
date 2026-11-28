// Package doctor reports local tool and cloud credential readiness.
package doctor

import (
	"context"
	"os/exec"
	"regexp"
	"strings"

	"github.com/hmrdkn-labs/lazyss/internal/domain"
)

// Check is one doctor readiness result.
type Check struct {
	Name   string
	OK     bool
	Detail string
	Fix    string
	// Optional marks a failed check that must not change doctor's exit status.
	Optional bool
}

// Doctor runs local preflight checks.
type Doctor struct {
	LookPath func(string) (string, error)
	Identity func(context.Context) error
	Region   string

	// GCP and Azure report CLI login state; nil skips the section. They run only
	// when the CLI is on PATH. RequireGCP/RequireAzure are set when the operator
	// explicitly asked for that source, making a failure fatal.
	GCP          func(context.Context) domain.ProviderStatus
	Azure        func(context.Context) domain.ProviderStatus
	RequireGCP   bool
	RequireAzure bool
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
	if d.GCP != nil {
		checks = append(checks, cloudCheck(ctx, lookPath, cloudSpec{
			name: "gcloud", exe: "gcloud", source: "gcp", status: d.GCP, required: d.RequireGCP,
			installFix: "install the Google Cloud CLI", loginFix: "run gcloud auth login",
		}))
	}
	if d.Azure != nil {
		checks = append(checks, cloudCheck(ctx, lookPath, cloudSpec{
			name: "azure", exe: "az", source: "azure", status: d.Azure, required: d.RequireAzure,
			installFix: "install the Azure CLI", loginFix: "run az login",
			missing: "az ssh extension missing", missingFix: "az extension add --name ssh",
		}))
	}
	return checks
}

type cloudSpec struct {
	name, exe, source    string
	status               func(context.Context) domain.ProviderStatus
	required             bool
	installFix, loginFix string
	missing, missingFix  string
}

// guidPattern matches a parenthesised GUID so subscription and tenant ids never
// reach doctor output.
var guidPattern = regexp.MustCompile(`\s*\([0-9a-fA-F]{8}-[0-9a-fA-F-]{27}\)`)

func cloudCheck(ctx context.Context, lookPath func(string) (string, error), spec cloudSpec) Check {
	check := Check{Name: spec.name, Optional: !spec.required}
	if _, err := lookPath(spec.exe); err != nil {
		check.Detail = "not installed; optional, needed for --source " + spec.source
		check.Fix = spec.installFix
		return check
	}
	st := spec.status(ctx)
	msg := guidPattern.ReplaceAllString(strings.TrimSpace(st.Message), "")
	if st.Status == domain.ProviderHealthy {
		check.OK = true
		check.Detail = nonempty(msg, "ready")
		return check
	}
	check.Detail = nonempty(msg, "not ready")
	check.Fix = spec.loginFix
	// Azure reports a missing ssh extension in its message; that needs the extension, not login.
	if spec.missing != "" && strings.Contains(msg, spec.missing) {
		check.Fix = spec.missingFix
	}
	return check
}

func nonempty(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func binaryCheck(lookPath func(string) (string, error), name, fix string) Check {
	path, err := lookPath(name)
	if err != nil {
		return Check{Name: name, Detail: "not found on PATH", Fix: fix}
	}
	return Check{Name: name, OK: true, Detail: path}
}
