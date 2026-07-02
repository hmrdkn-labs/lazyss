package brand

import (
	"strings"
	"testing"
)

func TestLogoUsesStableASCIIBranding(t *testing.T) {
	logo := Logo("")
	for _, want := range []string{
		"Lazy Secure Shell",
		"SSH + SSM cockpit",
	} {
		if !strings.Contains(logo, want) {
			t.Fatalf("logo missing %q:\n%s", want, logo)
		}
	}
	for _, r := range logo {
		if r == '\n' || r == '\t' {
			continue
		}
		if r < 32 || r > 126 {
			t.Fatalf("logo contains non-ASCII rune %q", r)
		}
	}
}

func TestLogoBuildsCustomASCIIBanner(t *testing.T) {
	logo := Logo("Ops Access")
	for _, want := range []string{
		"+------------+",
		"| Ops Access |",
	} {
		if !strings.Contains(logo, want) {
			t.Fatalf("custom logo missing %q:\n%s", want, logo)
		}
	}
}

func TestVersionReportUsesBinaryAndProductName(t *testing.T) {
	got := VersionReport("v0.1.0-12-gabcdef")
	for _, want := range []string{
		"Lazy Secure Shell",
		"binary: lazyss",
		"version: v0.1.0-12-gabcdef",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("version report missing %q:\n%s", want, got)
		}
	}
}
