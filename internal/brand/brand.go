// Package brand centralizes product naming, ASCII identity, and version output.
package brand

import (
	"fmt"
	"strings"
)

const (
	// Name is the user-facing product name.
	Name = "Lazy Secure Shell"
	// Binary is the installed command name.
	Binary = "lazyss"
	// Tagline is the short product promise used in CLI/TUI surfaces.
	Tagline = "SSH + SSM cockpit"
)

var logoLines = []string{
	" _                     ____ ____",
	"| |    __ _ _____   _ / ___/ ___|",
	"| |   / _` |_  / | | |\\___ \\___ \\",
	"| |__| (_| |/ /| |_| | ___) |__) |",
	"|_____\\__,_/___|\\__, ||____/____/",
	"                |___/            ",
	Name,
	Tagline,
}

// Logo returns the default product logo, or a generated ASCII banner for text.
func Logo(text string) string {
	text = cleanText(text)
	if text != "" && !strings.EqualFold(text, Name) && !strings.EqualFold(text, Binary) {
		return Banner(text)
	}
	return strings.Join(LogoLines(), "\n") + "\n"
}

// LogoLines returns a defensive copy of the default product logo.
func LogoLines() []string {
	out := make([]string, len(logoLines))
	copy(out, logoLines)
	return out
}

// Banner creates a compact ASCII logo for custom labels.
func Banner(text string) string {
	text = cleanText(text)
	if text == "" {
		text = Name
	}
	border := "+" + strings.Repeat("-", len(text)+2) + "+"
	return fmt.Sprintf("%s\n| %s |\n%s\n", border, text, border)
}

// ShortVersion renders the script-friendly version line.
func ShortVersion(version string) string {
	return fmt.Sprintf("%s %s", Binary, normalizeVersion(version))
}

// VersionReport renders the operator-facing version report.
func VersionReport(version string) string {
	return fmt.Sprintf("%s\nbinary: %s\nversion: %s\n", Name, Binary, normalizeVersion(version))
}

func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "dev"
	}
	return version
}

func cleanText(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	var b strings.Builder
	for _, r := range text {
		if r >= 32 && r <= 126 {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}
