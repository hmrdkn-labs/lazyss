package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	tuiAccent  = lipgloss.Color("#7DD3FC")
	tuiAccent2 = lipgloss.Color("#FBBF24")
	tuiDim     = lipgloss.Color("#8B949E")
	tuiFaint   = lipgloss.Color("#586069")
	tuiGood    = lipgloss.Color("#3FB950")
	tuiBad     = lipgloss.Color("#F85149")
	tuiInk     = lipgloss.Color("#0B1220")

	titleStyle       = lipgloss.NewStyle().Bold(true).Foreground(tuiInk).Background(tuiAccent).Padding(0, 1)
	metaStyle        = lipgloss.NewStyle().Foreground(tuiDim)
	panelStyle       = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(tuiFaint)
	panelActiveStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(tuiAccent)
	panelTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(tuiAccent)
	selectedStyle    = lipgloss.NewStyle().Bold(true).Foreground(tuiInk).Background(tuiAccent)
	goodStyle        = lipgloss.NewStyle().Foreground(tuiGood)
	badStyle         = lipgloss.NewStyle().Foreground(tuiBad)
	warnStyle        = lipgloss.NewStyle().Foreground(tuiAccent2)
	dimStyle         = lipgloss.NewStyle().Foreground(tuiDim)
	faintStyle       = lipgloss.NewStyle().Foreground(tuiFaint)
)

func displayFit(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width <= 3 {
		runes := []rune(s)
		if len(runes) > width {
			runes = runes[:width]
		}
		return string(runes)
	}
	out := ""
	for _, r := range s {
		next := out + string(r)
		if lipgloss.Width(next) > width-3 {
			break
		}
		out = next
	}
	return out + "..."
}

func displayPadRight(s string, width int) string {
	if width <= 0 {
		return ""
	}
	fit := displayFit(s, width)
	if gap := width - lipgloss.Width(fit); gap > 0 {
		return fit + strings.Repeat(" ", gap)
	}
	return fit
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
