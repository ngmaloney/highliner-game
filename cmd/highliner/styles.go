package main

import "github.com/charmbracelet/lipgloss"

// ─── Styles ───────────────────────────────────────────────────────────────────

var (
	// CGA/EGA palette approximations
	colorBlack         = lipgloss.Color("0")
	colorGreen         = lipgloss.Color("2")
	colorBrightGreen   = lipgloss.Color("10")
	colorCyan          = lipgloss.Color("6")
	colorBrightCyan    = lipgloss.Color("14")
	colorAmber         = lipgloss.Color("3")        // dark yellow / amber
	colorBrightAmber   = lipgloss.Color("#FFB300")  // amber
	colorRed           = lipgloss.Color("1")
	colorBrightRed     = lipgloss.Color("#FF3333")
	colorWhite         = lipgloss.Color("7")
	colorBrightWhite   = lipgloss.Color("15")
	colorMagenta       = lipgloss.Color("5")
	colorBrightMagenta = lipgloss.Color("13")

	styleBase = lipgloss.NewStyle().
			Background(colorBlack).
			Foreground(colorBrightGreen)

	styleBorder = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(colorCyan).
			Background(colorBlack)

	styleHeader = lipgloss.NewStyle().
			Background(colorCyan).
			Foreground(colorBlack).
			Bold(true)

	styleTab = lipgloss.NewStyle().
			Background(colorBlack).
			Foreground(colorCyan).
			Padding(0, 1)

	styleTabActive = lipgloss.NewStyle().
			Background(colorCyan).
			Foreground(colorBlack).
			Bold(true).
			Padding(0, 1)

	styleLabel = lipgloss.NewStyle().
			Foreground(colorAmber).
			Bold(true)

	styleValue = lipgloss.NewStyle().
			Foreground(colorBrightWhite)

	styleGood = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00"))

	styleWarn = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF8C00")).Bold(true) // bright orange

	styleDanger = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF3333"))

	styleLog = lipgloss.NewStyle().
			Foreground(colorBrightGreen)

	styleLogInfo = lipgloss.NewStyle().
			Foreground(colorBrightCyan)

	styleLogWarn = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).Bold(true) // bright orange

	styleLogDanger = lipgloss.NewStyle().
			Foreground(colorBrightRed)

	styleLogRevenue = lipgloss.NewStyle().
			Foreground(colorBrightAmber).
			Bold(true)

	styleLogGreen = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")).
			Bold(true)

	styleLogRed = lipgloss.NewStyle().
			Foreground(colorBrightRed).
			Bold(true)

	styleLogExpense = lipgloss.NewStyle().
			Foreground(colorRed)

	styleLogGrade = lipgloss.NewStyle().
			Foreground(colorCyan)

	styleLogDeck = lipgloss.NewStyle().
			Foreground(colorAmber).
			Italic(true)

	styleTitle = lipgloss.NewStyle().
			Foreground(colorBrightCyan).
			Bold(true)

	styleSelected = lipgloss.NewStyle().
			Background(colorBrightGreen).
			Foreground(colorBlack).
			Bold(true)

	styleKey = lipgloss.NewStyle().
			Background(colorBlack).
			Foreground(colorAmber).
			Bold(true)

	styleStatusBar = lipgloss.NewStyle().
			Background(colorCyan).
			Foreground(colorBlack)
)
