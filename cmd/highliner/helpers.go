package main

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─── Subtitles ────────────────────────────────────────────────────────────────

var subtitles = []string{
	"Haul. Sell. Survive.",
	"Pots, Diesel & Debt",
	"The Gulf Don't Care",
	"Salt, Diesel & Stubbornness",
	"Pull or Perish",
	"Traps Don't Haul Themselves.",
	"Dead Reckoning",
	"The Bite is On",
	"Bait. Set. Haul.",
	"You Either Highlinin or You Lyin",
}

func randomSubtitle() string {
	return subtitles[rand.Intn(len(subtitles))]
}

// ─── Health ───────────────────────────────────────────────────────────────────

func healthBar(h float64, width int) string {
	filled := int(h / 100.0 * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	empty := width - filled

	var style lipgloss.Style
	if h >= 70 {
		style = styleGood
	} else if h >= 40 {
		style = styleWarn
	} else {
		style = styleDanger
	}

	bar := "[" + strings.Repeat("█", filled) + strings.Repeat("░", empty) + "]"
	return style.Render(bar)
}

func healthStr(h float64) string {
	s := fmt.Sprintf("%.0f%%", h)
	if h >= 60 {
		return styleGood.Render(s)
	} else if h >= 30 {
		return styleWarn.Render(s)
	}
	return styleDanger.Render(s)
}

func healthStatus(h float64) string {
	if h >= 80 {
		return "GOOD"
	} else if h >= 60 {
		return "OK"
	} else if h >= 40 {
		return "WARN"
	} else if h >= 20 {
		return "POOR"
	}
	return "CRITICAL"
}

// ─── Money ────────────────────────────────────────────────────────────────────

func groundfishPrice(name string) float64 {
	switch name {
	case "Monkfish":
		return 4.00
	case "Cusk/Hake":
		return 2.75
	case "Black Sea Bass":
		return 3.50
	case "Halibut":
		return 18.00
	}
	return 3.00
}

func moneyStr(v float64) string {
	if v < 0 {
		return fmt.Sprintf("-$%.2f", -v)
	}
	return fmt.Sprintf("$%.2f", v)
}

func moneyStyled(v float64) string {
	s := moneyStr(v)
	if v < 0 {
		return styleDanger.Render(s)
	} else if v < 100 {
		return styleWarn.Render(s)
	}
	return styleGood.Render(s)
}

func styleDim(s string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("246")).Render(s) // readable grey
}

// ─── Phase Name ───────────────────────────────────────────────────────────────

func phaseName(p Phase) string {
	switch p {
	case PhaseMorning:
		return "MORNING PREP"
	case PhaseZoneSelect:
		return "ZONE SELECT"
	case PhaseHauling:
		return "HAULING"
	case PhaseDecision:
		return "DECISION"
	case PhaseSell:
		return "CO-OP SALE"
	case PhaseEvening:
		return "EVENING"
	case PhaseGameOver:
		return "GAME OVER"
	}
	return "???"
}

// ─── Math helpers ─────────────────────────────────────────────────────────────

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ─── Screen Content Helpers ───────────────────────────────────────────────────

// sectionHeader renders a full-width DOS-style section title bar
func sectionHeader(title string, width int) string {
	if width < 4 {
		width = 80
	}
	pre := "══ " + title + " "
	pad := width - len(pre) - 2
	if pad < 0 {
		pad = 0
	}
	line := pre + strings.Repeat("═", pad+2)
	return "\n" + styleTitle.Render(line) + "\n\n"
}

func subHeader(title string, width int) string {
	if width < 20 {
		width = 80
	}
	pre := "  ── " + title + " "
	pad := width - len([]rune(pre)) - 2
	if pad < 0 {
		pad = 0
	}
	line := pre + strings.Repeat("─", pad)
	return styleLogInfo.Render(line) + "\n"
}

// label pads a plain string to width BEFORE applying style, so ANSI codes don't break column alignment
func label(s string, width int) string {
	return styleLabel.Render(fmt.Sprintf("%-*s", width, s))
}

// ─── Log Helpers (methods on model) ──────────────────────────────────────────

// logDivider returns a full-width ── TITLE ────... string for the log
func (m *model) logDivider(style lipgloss.Style, title string) string {
	w := m.width
	if w < 20 {
		w = 80
	}
	pre := "── " + title + " "
	pad := w - len(pre) - 1
	if pad < 0 {
		pad = 0
	}
	return style.Render(pre + strings.Repeat("─", pad))
}

// logRule returns a full-width ═══...═══ line
func (m *model) logRule(style lipgloss.Style) string {
	w := m.width
	if w < 20 {
		w = 80
	}
	return style.Render(strings.Repeat("═", w))
}
