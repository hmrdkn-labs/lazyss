package doctor

import (
	"context"
	"errors"
	"testing"
)

func TestDoctorReportsMissingBinariesAndCredentials(t *testing.T) {
	d := Doctor{
		LookPath: func(_ string) (string, error) { return "", errors.New("missing") },
		Identity: func(context.Context) error { return errors.New("expired credentials") },
		Region:   "",
	}
	checks := d.Run(context.Background())
	if len(checks) < 4 {
		t.Fatalf("checks = %#v", checks)
	}
	for _, c := range checks {
		if c.OK {
			t.Fatalf("expected failures, got %#v", checks)
		}
	}
}
