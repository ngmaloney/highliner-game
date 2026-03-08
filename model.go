package main

import (
	"time"
	"fmt"
	"math"
	"math/rand"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Screen IDs ──────────────────────────────────────────────────────────────

type Screen int

const (
	ScreenLog Screen = iota
	ScreenMaintenance
	ScreenDock
	ScreenMarket
)

// ─── Styles ───────────────────────────────────────────────────────────────────

var (
	// CGA/EGA palette approximations
	colorBlack         = lipgloss.Color("0")
	colorGreen         = lipgloss.Color("2")
	colorBrightGreen   = lipgloss.Color("10")
	colorCyan          = lipgloss.Color("6")
	colorBrightCyan    = lipgloss.Color("14")
	colorAmber         = lipgloss.Color("3")  // dark yellow / amber
	colorBrightAmber   = lipgloss.Color("#FFB300") // amber
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

// ─── Model ────────────────────────────────────────────────────────────────────

type model struct {
	gs           *GameState
	screen       Screen
	phase        Phase
	weather      Weather
	selectedZone int
	currentZone  Zone
	haul         *HaulResult
	logLines     []string
	pendingLines      []string     // queued lines for animated haul
	postEventLines    []string     // lines to resume after event decision
	activeEvent       *RandomEvent // current decision event, nil if none
	eventHaulResult   *HaulResult  // stored so event consequences can modify catch
	viewport     viewport.Model
	altVP        viewport.Model // for maintenance/dock/market screens
	width        int
	height       int
	confirmBuy   string // for market confirmations
	buyAmount    int
	marketCursor int
	saveMsg      string
	subtitle     string
	initialized  bool
}

type tickMsg struct{}

func newModel(gs *GameState) model {
	vp := viewport.New(80, 20)
	altVP := viewport.New(80, 20)
	m := model{
		gs:          gs,
		screen:      ScreenLog,
		phase:       PhaseMorning,
		viewport:    vp,
		altVP:       altVP,
		subtitle:    randomSubtitle(),
		initialized: false,
	}
	m.weather = rollWeather()
	return m
}

func (m *model) initLog() {
	m.startMorning()
}

var styleDefault = lipgloss.NewStyle().Foreground(colorBrightWhite)

// ─── Random Events ────────────────────────────────────────────────────────────

type EventType int

const (
	EventNone EventType = iota
	EventBerriedHen
	EventSquareGrouper
	EventJonahCrabs
	EventBoatDistress
	EventGhostTrap
	EventStormComing
)

type RandomEvent struct {
	Type    EventType
	Time    string // e.g. "1045"
	Desc    string // log line shown before decision
	KeyA    string // key to press
	LabelA  string // label for option A
	KeyB    string // key to press
	LabelB  string // label for option B
}

// ─── Animated Haul ────────────────────────────────────────────────────────────

type haulTickMsg struct{}

func tickHaul() tea.Cmd {
	return tea.Tick(280*time.Millisecond, func(t time.Time) tea.Msg {
		return haulTickMsg{}
	})
}

func (m *model) addLog(s string) {
	// If string has no ANSI codes, force bright white so terminal default doesn't dim it
	if !strings.Contains(s, "\x1b[") && s != "" {
		s = styleDefault.Render(s)
	}
	m.logLines = append(m.logLines, s)
}

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

func (m *model) addLogStyled(style lipgloss.Style, s string) {
	m.logLines = append(m.logLines, style.Render(s))
}

func (m *model) queueLog(s string) {
	if !strings.Contains(s, "\x1b[") && s != "" {
		s = styleDefault.Render(s)
	}
	m.pendingLines = append(m.pendingLines, s)
}

func (m *model) queueLogStyled(style lipgloss.Style, s string) {
	m.pendingLines = append(m.pendingLines, style.Render(s))
}

func (m model) Init() tea.Cmd {
	return nil
}

// ─── Update ──────────────────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case haulTickMsg:
		if len(m.pendingLines) > 0 {
			m.addLog(m.pendingLines[0])
			m.pendingLines = m.pendingLines[1:]
			m.syncViewport()
			if len(m.pendingLines) > 0 {
				return m, tickHaul()
			}
			// pendingLines empty — check if we're pausing for an event
			if m.activeEvent != nil {
				m.phase = PhaseDecision
				m.showEventPrompt()
				return m, nil
			}
			// Animation fully done — sell and go to evening
			m.doSell()
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 2
		m.viewport.Height = m.height - 8
		m.altVP.Width = msg.Width
		m.altVP.Height = m.height - 6
		if !m.initialized {
			m.initialized = true
			m.initLog()
		}
		m.syncViewport()
		m.syncAltViewport()

	case tea.KeyMsg:
		switch msg.String() {

		case "ctrl+c", "q":
			if m.phase == PhaseGameOver {
				return m, tea.Quit
			}
			saveGame(m.gs)
			return m, tea.Quit

		// Tab navigation — disabled during zone select (numbers 1-7 pick zones)
		case "1", "/":
			if m.phase != PhaseZoneSelect && m.phase != PhaseEvening && m.phase != PhaseDecision {
				m.screen = ScreenLog
				m.confirmBuy = ""
				m.syncViewport()
			} else {
				return m.handlePhaseKey(msg.String())
			}
		case "2", "m":
			if m.phase != PhaseZoneSelect && m.phase != PhaseEvening && m.phase != PhaseDecision {
				m.screen = ScreenMaintenance
				m.confirmBuy = ""
				m.syncAltViewport()
			} else {
				return m.handlePhaseKey(msg.String())
			}
		case "3", "d":
			if m.phase != PhaseZoneSelect && m.phase != PhaseEvening && m.phase != PhaseDecision {
				m.screen = ScreenDock
				m.confirmBuy = ""
				m.syncAltViewport()
			} else {
				return m.handlePhaseKey(msg.String())
			}
		case "4", "w":
			if m.phase != PhaseZoneSelect && m.phase != PhaseEvening && m.phase != PhaseDecision {
				m.screen = ScreenMarket
				m.confirmBuy = ""
				m.marketCursor = 0
				m.syncAltViewport()
			} else {
				return m.handlePhaseKey(msg.String())
			}

		default:
			// Fix: scroll alt viewport with j/k/up/down on non-interactive screens
			if m.screen == ScreenMaintenance || m.screen == ScreenDock {
				switch msg.String() {
				case "up", "k":
					m.altVP.LineUp(1)
					return m, nil
				case "down", "j":
					m.altVP.LineDown(1)
					return m, nil
				case "pgup":
					m.altVP.HalfViewUp()
					return m, nil
				case "pgdn":
					m.altVP.HalfViewDown()
					return m, nil
				case "b":
					if m.screen == ScreenDock {
						if m.gs.HasCrabPermit {
							m.confirmBuy = "Already licensed for crab."
						} else if m.gs.Money < 500 {
							m.confirmBuy = fmt.Sprintf("Need $500 — short by %s", moneyStr(500-m.gs.Money))
						} else {
							m.gs.Money -= 500
							m.gs.HasCrabPermit = true
							m.confirmBuy = "Crab permit issued. Jonah and rock crabs are yours to keep."
						}
						m.syncAltViewport()
						return m, nil
					}
				case "g":
					if m.screen == ScreenDock {
						if m.gs.HasGroundfishPermit {
							m.confirmBuy = "Already licensed for groundfish."
						} else if m.gs.Money < 1500 {
							m.confirmBuy = fmt.Sprintf("Need $1,500 — short by %s", moneyStr(1500-m.gs.Money))
						} else {
							m.gs.Money -= 1500
							m.gs.HasGroundfishPermit = true
							m.confirmBuy = "Groundfish permit issued. Monkfish and sea bass are yours to keep."
						}
						m.syncAltViewport()
						return m, nil
					}
				}
			}
			return m.handlePhaseKey(msg.String())
		}
	}

	// Viewport scrolling on log screen
	if m.screen == ScreenLog {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) handlePhaseKey(key string) (model, tea.Cmd) {
	// Market input takes priority — consume enter/space/arrows before phase handler sees them
	if m.screen == ScreenMarket {
		switch key {
		case "up", "k":
			if m.marketCursor > 0 {
				m.marketCursor--
				m.syncAltViewport()
			}
			return m, nil
		case "down", "j":
			if m.marketCursor < 12 { // 13 items: 4 supplies + 3 repairs + 6 equipment (0-12)
				m.marketCursor++
				m.syncAltViewport()
			}
			return m, nil
		case "enter", " ":
			m.doBuy()
			m.syncAltViewport()
			return m, nil
		}
	}

	switch m.phase {

	case PhaseMorning:
		switch key {
		case "enter", " ", "f":
			if m.weather.CanFish {
				m.phase = PhaseZoneSelect
				m.selectedZone = 0
				m.screen = ScreenLog
				m.addLog("")
				m.addLog(m.logDivider(styleTitle, "ZONE SELECT"))
				m.addLog("Choose your fishing grounds for today:")
				m.addLog("")
				for i, z := range Zones {
					if m.zoneBlocked(i) {
						reason := m.zoneBlockReason(i)
						m.addLog(styleDim(fmt.Sprintf("  [%d] %s (%s) — %s  %s", i+1, z.ID, z.Name, z.Description, reason)))
					} else {
						m.addLog(lipgloss.NewStyle().Foreground(colorBrightWhite).Render(fmt.Sprintf("  [%d] %s (%s) — %s", i+1, z.ID, z.Name, z.Description)))
					}
				}
				m.addLog("")
				m.addLog("Press 1-7 to select zone")
				m.syncViewport()
			} else {
				m.addLog("")
				m.addLogStyled(styleLogInfo, "⚓ Staying in port.")
				m.chargeDockFee()
				m.phase = PhaseSell
				m.doSell()
			}
		case "s":
			m.addLog("")
			m.addLogStyled(styleLogInfo, "Staying in port today.")
			m.chargeDockFee()
			m.phase = PhaseSell
			m.doSell()
		}

	case PhaseZoneSelect:
		for i := range Zones {
			if key == fmt.Sprintf("%d", i+1) {
				if m.zoneBlocked(i) {
					return m, nil // silently ignore — zone select screen shows the block
				}
				m.selectedZone = i
				m.phase = PhaseHauling
				m.screen = ScreenLog
				m.doHaul()
				return m, tickHaul()
			}
		}
		switch key {
		case "up", "k":
			if m.selectedZone > 0 {
				m.selectedZone--
			}
		case "down", "j":
			if m.selectedZone < len(Zones)-1 {
				m.selectedZone++
			}
		case "enter":
			if !m.zoneBlocked(m.selectedZone) {
				m.phase = PhaseHauling
				m.screen = ScreenLog
				m.doHaul()
				return m, tickHaul()
			}
		}

	case PhaseHauling:
		if key == " " && len(m.pendingLines) > 0 {
			for _, line := range m.pendingLines {
				m.addLog(line)
			}
			m.pendingLines = nil
			m.syncViewport()
			m.doSell()
			return m, nil
		}

	case PhaseDecision:
		if m.activeEvent != nil {
			m.resolveEvent(key)
		}

	case PhaseSell:
		// auto-transitions to PhaseEvening — no input needed

	case PhaseEvening:
		switch key {
		case "1": // Allen's Coffee Brandy
			if m.gs.Money >= 12 {
				m.gs.Money -= 12
				m.gs.DaysWithBooze++
				m.addLog("")
				if m.gs.DaysWithBooze >= 3 {
					m.gs.Hungover = true
					m.addLogStyled(styleLogDanger, "  You close the bar down. Again.")
					m.addLogStyled(styleLogDanger, "  Tomorrow's gonna be rough.")
				} else {
					m.addLogStyled(styleLogInfo, "  Few nips of Allen's. Sleep like a rock.")
				}
				m.doNextDay()
			} else {
				m.addLog(styleWarn.Render("  Not enough cash for a bottle."))
				m.syncViewport()
			}
		case "2": // Six-Pack of Natty
			if m.gs.Money >= 9 {
				m.gs.Money -= 9
				if m.gs.DaysWithBooze < 2 {
					m.gs.DaysWithBooze++
				}
				m.addLog("")
				m.addLogStyled(styleLogInfo, "  Crack a few on the float. Not a bad evening.")
				m.doNextDay()
			} else {
				m.addLog(styleWarn.Render("  Can't even afford a six-pack of Natty. Rough week."))
				m.syncViewport()
			}
		case "3": // Scratch ticket
			if m.gs.Money >= 5 {
				m.gs.Money -= 5
				winnings := scratchTicket()
				m.addLog("")
				if winnings == 0 {
					m.addLog("  Scratch ticket: loser. Threw it in the harbor.")
				} else if winnings >= 500 {
					m.gs.Money += float64(winnings)
					m.addLogStyled(styleLogGreen, fmt.Sprintf("  🎰 JACKPOT! Scratch ticket pays $%d!", winnings))
				} else {
					m.gs.Money += float64(winnings)
					m.addLogStyled(styleLogGreen, fmt.Sprintf("  Scratch ticket winner: +$%d", winnings))
				}
				m.doNextDay()
			} else {
				m.addLog(styleWarn.Render("  Can't spare $5 for a ticket."))
				m.syncViewport()
			}
		case "enter", " ", "n", "4": // early night
			m.gs.DaysWithBooze = 0
			m.doNextDay()
		}

	case PhaseGameOver:
		// q handled globally
	}



	m.syncViewport()
	return m, nil
}

func scratchTicket() int {
	r := rand.Float64()
	switch {
	case r < 0.001: return 500  // 0.1% jackpot
	case r < 0.021: return 100  // 2%
	case r < 0.101: return 25   // 8%
	case r < 0.351: return 10   // 25%
	default:        return 0    // 65% loser
	}
}

func (m *model) doEvening() {
	m.addLog("")
	m.addLog(m.logDivider(styleLogInfo, "EVENING"))
	m.addLog("")
	m.addLog(lipgloss.NewStyle().Foreground(colorBrightWhite).Render("  End of day. What are you doing tonight?"))
	m.addLog("")
	m.addLog(lipgloss.NewStyle().Foreground(colorBrightWhite).Render(fmt.Sprintf("  [1] Allen's Coffee Brandy   $12   %s", styleDim("3 nights running = can't fish tomorrow"))))
	m.addLog(lipgloss.NewStyle().Foreground(colorBrightWhite).Render(fmt.Sprintf("  [2] Six-Pack of Natty       $9    %s", styleDim("Mild. Won't wreck you."))))
	m.addLog(lipgloss.NewStyle().Foreground(colorBrightWhite).Render(fmt.Sprintf("  [3] Scratch Ticket          $5    %s", styleDim("Probably a loser. Probably."))))
	m.addLog(lipgloss.NewStyle().Foreground(colorBrightWhite).Render(fmt.Sprintf("  [4] Early night             free  %s", styleDim("Up before dawn. Full day tomorrow."))))
	m.addLog("")
	m.addLog(fmt.Sprintf("  Cash: %s", moneyStr(m.gs.Money)))
	m.syncViewport()
}

func (m *model) doNextDay() {
	m.gs.Day++
	m.weather = rollWeather()
	m.haul = nil
	m.screen = ScreenLog
	m.addLog("")
	m.addLog(m.logDivider(styleTitle, fmt.Sprintf("DAY %d", m.gs.Day)))
	// Auto-fill tank overnight
	boat := BoatModels[m.gs.BoatName]
	needed := boat.FuelCap - m.gs.Fuel
	if needed > 0 {
		cost := float64(needed) * m.gs.DieselPrice
		m.gs.Money -= cost
		m.gs.Fuel = boat.FuelCap
		m.addLogStyled(styleLogExpense, fmt.Sprintf("  ⛽ Filled tank overnight: %d gal @ $%.2f/gal (-$%.2f)", needed, m.gs.DieselPrice, cost))
	}
	if m.gs.Hungover {
		m.gs.Hungover = false
		m.gs.DaysWithBooze = 0
		m.phase = PhaseMorning
		m.addLog("")
		m.addLogStyled(styleLogDanger, "  You wake up face-down on the bait table.")
		m.addLogStyled(styleLogDanger, "  Can't make it out today. Day wasted.")
		m.chargeDockFee()
		m.addLog("")
		m.addLogStyled(styleKey, "  [ENTER/S] Skip to co-op   [W] Wharf   [M] Maintenance")
	} else {
		m.phase = PhaseMorning
		m.gs.DaysWithBooze = 0
		m.startMorning()
	}
	saveGame(m.gs)
	m.saveMsg = "✓ saved"
}

func (m *model) startMorning() {
	m.saveMsg = ""
	m.gs.DailyPrices = RollDailyPrices()
	m.gs.DieselPrice = RollDieselPrice()
	m.gs.BaitPrice = RollBaitPrice()
	m.addLog("")
	m.addLog(m.logDivider(styleLogInfo, "MORNING BRIEFING"))
	m.addLog("")

	w := m.weather
	weatherStyle := styleGood
	if w.Type == WeatherFog {
		weatherStyle = styleWarn
	} else if w.Type == WeatherSCA || w.Type == WeatherGale {
		weatherStyle = styleDanger
	}
	m.addLogStyled(styleLogInfo, "  NOAA MARINE FORECAST — EASTERN MAINE COASTAL WATERS")
	if !w.CanFish {
		m.addLogStyled(weatherStyle, fmt.Sprintf("  ⚠ %s — Stay in port.", w.Type))
	} else {
		m.addLogStyled(weatherStyle, fmt.Sprintf("  %s", w.Type))
	}
	m.addLogStyled(weatherStyle, fmt.Sprintf("  %s", w.NOAAForecast()))
	m.addLog("")

	m.addLogStyled(styleLabel, fmt.Sprintf("Vessel: %s — %s", m.gs.VesselName, m.gs.BoatName))
	m.addLog(fmt.Sprintf("  Engine: %s  |  Zincs: %s  |  Hydraulics: %s",
		healthStr(m.gs.Engine),
		healthStr(m.gs.Zincs),
		healthStr(m.gs.Hydraulics)))

	alerts := m.getAlerts()
	if len(alerts) > 0 {
		m.addLog("")
		m.addLog(m.logDivider(styleLogWarn, "MAINTENANCE ALERTS"))
		for _, a := range alerts {
			m.addLogStyled(styleLogWarn, fmt.Sprintf("  ⚠ %s", a))
		}
		m.addLogStyled(styleLogInfo, "  Press [m] to go to Maintenance screen")
	}

	m.addLog("")
	// Market intel
	m.addLog(m.logDivider(styleLogInfo, "MARKET"))
	p := m.gs.DailyPrices
	m.addLogStyled(styleLogInfo, fmt.Sprintf("  Chix $%.2f   Quarters $%.2f   Selects $%.2f   Jumbos $%.2f   Supers $%.2f   Culls $%.2f",
		p[0], p[1], p[2], p[3], p[4], p[5]))
	m.addLog(fmt.Sprintf("  Diesel $%.2f/gal   Herring bait $%.2f/lb",
		m.gs.DieselPrice, m.gs.BaitPrice))
	m.addLog("")



	if m.gs.Money < -10000 {
		m.phase = PhaseGameOver
		m.addLog("")
		m.addLog(m.logRule(styleLogDanger))
		m.addLogStyled(styleLogDanger, "  GAME OVER — The bank took the boat.")
		m.addLogStyled(styleLogDanger, fmt.Sprintf("  You fished %d days and caught %.0f lbs total.", m.gs.Day, m.gs.TotalCatch))
		m.addLogStyled(styleLogDanger, "  Press Q to quit.")
		m.addLog(m.logRule(styleLogDanger))
		return
	}

	if m.weather.CanFish && m.gs.Bait <= 0 {
		m.addLog("")
		m.addLogStyled(styleLogWarn, "  ⚠ Out of bait! Buy some at the wharf [w].")
	}
	if m.weather.CanFish && m.gs.Fuel < 5 {
		m.addLog("")
		m.addLogStyled(styleLogWarn, "  ⚠ Low fuel! Refuel at the wharf [w].")
	}

	m.addLog("")
	if m.weather.CanFish {
		m.addLog("")
		m.addLog(lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Bold(true).Render("  ▶ ENTER to fish") + "   " + styleDim("[S] Stay in port   [W] Wharf   [M] Maintenance"))
	} else {
		m.addLog("")
		m.addLog(styleWarn.Render("  ▶ ENTER to wait out weather") + "   " + styleDim("[W] Wharf   [M] Maintenance"))
	}
	m.syncViewport()
}

func (m *model) getAlerts() []string {
	var alerts []string
	if m.gs.Engine < 25 {
		alerts = append(alerts, fmt.Sprintf("Engine critically low (%.0f%%) — risk of breakdown at sea", m.gs.Engine))
	} else if m.gs.Engine < 50 {
		alerts = append(alerts, fmt.Sprintf("Engine needs service (%.0f%%)", m.gs.Engine))
	}
	if m.gs.Zincs < 20 {
		alerts = append(alerts, fmt.Sprintf("Zincs nearly depleted (%.0f%%) — hull corrosion accelerating", m.gs.Zincs))
	} else if m.gs.Zincs < 40 {
		alerts = append(alerts, fmt.Sprintf("Replace zincs soon (%.0f%%)", m.gs.Zincs))
	}
	if m.gs.Hydraulics < 25 {
		alerts = append(alerts, fmt.Sprintf("Hydraulics critical (%.0f%%) — hauler may fail", m.gs.Hydraulics))
	} else if m.gs.Hydraulics < 50 {
		alerts = append(alerts, fmt.Sprintf("Hydraulics degraded (%.0f%%)", m.gs.Hydraulics))
	}
	return alerts
}

// buildDebriefLines builds the end-of-day summary lines into the given queue
func (m *model) buildDebriefLines(result HaulResult, queue *[]string) {
	addQ := func(s string) {
		if !strings.Contains(s, "\x1b[") && s != "" {
			s = styleDefault.Render(s)
		}
		*queue = append(*queue, s)
	}
	addQS := func(style lipgloss.Style, s string) {
		*queue = append(*queue, style.Render(s))
	}

	addQ("")
	addQS(styleTitle, m.logDivider(styleTitle, "END OF DAY"))
	addQ("")
	addQS(styleLogInfo, "  CATCH")

	totalGross := 0.0
	for _, g := range result.Grades {
		gross := g.Lbs * g.Price
		totalGross += gross
		addQ(fmt.Sprintf("    %-10s %5.1f lbs  @ $%.2f/lb  = %s", g.Name, g.Lbs, g.Price, moneyStr(gross)))
	}
	addQ(fmt.Sprintf("    %-10s %5.1f lbs%s%s", "TOTAL", result.CatchLbs, strings.Repeat(" ", 16), moneyStr(totalGross)))

	// Bycatch section
	bycatchTotal := 0.0
	if result.JonahCrabLbs > 0 || result.RockCrabLbs > 0 || result.GroundfishLbs > 0 {
		addQ("")
		addQS(styleLogInfo, "  BYCATCH")
		if result.JonahCrabLbs > 0 {
			price := 0.75
			gross := result.JonahCrabLbs * price
			bycatchTotal += gross
			addQ(fmt.Sprintf("    %-14s %5.1f lbs  @ $%.2f/lb  = %s", "Jonah Crab", result.JonahCrabLbs, price, moneyStr(gross)))
		}
		if result.RockCrabLbs > 0 {
			price := 0.35
			gross := result.RockCrabLbs * price
			bycatchTotal += gross
			addQ(fmt.Sprintf("    %-14s %5.1f lbs  @ $%.2f/lb  = %s", "Rock Crab", result.RockCrabLbs, price, moneyStr(gross)))
		}
		if result.GroundfishLbs > 0 {
			price := groundfishPrice(result.GroundfishName)
			gross := result.GroundfishLbs * price
			bycatchTotal += gross
			if result.GroundfishName == "Halibut" {
				addQS(styleLogGreen, fmt.Sprintf("    %-14s %5.1f lbs  @ $%.2f/lb  = %s  ★", result.GroundfishName, result.GroundfishLbs, price, moneyStr(gross)))
			} else {
				addQ(fmt.Sprintf("    %-14s %5.1f lbs  @ $%.2f/lb  = %s", result.GroundfishName, result.GroundfishLbs, price, moneyStr(gross)))
			}
		}
		totalGross += bycatchTotal
	}
	addQ("")

	fuelCost := float64(result.FuelUsed) * m.gs.DieselPrice
	baitCost := float64(result.BaitUsed) * m.gs.BaitPrice
	addQS(styleLogInfo, "  EXPENSES")
	addQS(styleLogExpense, fmt.Sprintf("    Fuel   %d gal × $%.2f  -%s", result.FuelUsed, m.gs.DieselPrice, moneyStr(fuelCost)))
	addQS(styleLogExpense, fmt.Sprintf("    Bait   %d lbs × $%.2f  -%s", result.BaitUsed, m.gs.BaitPrice, moneyStr(baitCost)))
	addQ("")

	tripNet := totalGross - fuelCost - baitCost
	if tripNet >= 0 {
		addQS(styleLogGreen, fmt.Sprintf("  Trip net: %s", moneyStr(tripNet)))
	} else {
		addQS(styleLogRed, fmt.Sprintf("  Trip net: -%s (in the hole)", moneyStr(-tripNet)))
	}
	addQ("")

	zone := m.currentZone
	deckEntry := DeckLog(m.gs, zone, result)
	addQS(styleLogInfo, "  FROM THE DECK")
	addQS(styleLogDeck, fmt.Sprintf("  \"%s\"", deckEntry))
	addQ("")
}

func (m *model) doHaul() {
	zone := Zones[m.selectedZone]
	m.currentZone = zone
	result := simulateHaul(m.gs, zone, m.weather)
	m.haul = &result
	m.eventHaulResult = &result

	// Apply state changes immediately
	m.gs.Engine = math.Max(0, m.gs.Engine-result.EngineDmg)
	m.gs.Zincs = math.Max(0, m.gs.Zincs-result.ZincsDmg)
	m.gs.Hydraulics = math.Max(0, m.gs.Hydraulics-result.HydraulicsDmg)
	m.gs.Fuel = max(0, m.gs.Fuel-result.FuelUsed)
	m.gs.Bait = max(0, m.gs.Bait-result.BaitUsed)
	m.gs.Freezer += result.CatchLbs
	m.gs.TotalCatch += result.CatchLbs
	if result.TrapsLost > 0 {
		m.gs.Traps = max(0, m.gs.Traps-result.TrapsLost)
	}

	// Roll for a random event
	m.activeEvent = RollRandomEvent(m.gs, m.weather)

	// Immediate header
	m.addLog("")
	m.addLog(m.logDivider(styleTitle, "HAULING"))
	m.addLog(fmt.Sprintf("  Zone: %s — %s  |  Traps: %d  |  Gear: %.0f%%",
		zone.ID, zone.Name, m.gs.Traps, (m.gs.Engine+m.gs.Zincs+m.gs.Hydraulics)/3.0))
	m.addLog("")
	m.addLog(styleDim("  [SPACE] skip"))
	m.syncViewport()

	// Build pre-event queue (everything up to and including the event timestamp)
	m.pendingLines = nil
	m.queueLog("0600 — Left the dock, steaming to grounds...")
	m.queueLog(fmt.Sprintf("0730 — First buoy in sight. Zone %s.", zone.ID))

	if result.EngineDmg > 2.5 {
		m.queueLogStyled(styleLogWarn, "0815 — Engine running rough, losing a few RPM.")
	}
	if result.HydraulicsDmg > 2.0 {
		m.queueLogStyled(styleLogWarn, "0920 — Hauler hesitating on deep sets. Hydraulic pressure dropping.")
	}
	if result.ZincsDmg > 1.2 {
		m.queueLog("1010 — Mental note: zincs need checking this week.")
	}
	if m.gs.BoatName == "Eastern 22" && m.weather.Type == WeatherSCA {
		m.queueLogStyled(styleLogDanger, "1045 — ⚠ Eastern 22 taking a beating in these swells.")
		hullDmg := result.HullDmg * 2.5
		m.gs.Engine = math.Max(0, m.gs.Engine-hullDmg)
	}
	// Bycatch log lines
	if result.JonahCrabLbs > 0 {
		m.queueLog(fmt.Sprintf("1050 — Jonah crabs in the traps. Keeping %.0f lbs.", result.JonahCrabLbs))
	} else if !m.gs.HasCrabPermit && rand.Float64() < 0.25 {
		m.queueLog("1050 — Pulled some Jonah crabs. No crab permit — back they go.")
	}
	if result.GroundfishLbs > 0 {
		m.queueLogStyled(styleLogGreen, fmt.Sprintf("1055 — %s in the trap! %.0f lbs. Keeping it.", result.GroundfishName, result.GroundfishLbs))
	} else if !m.gs.HasGroundfishPermit && rand.Float64() < 0.12 {
		fish := []string{"monkfish", "cusk", "halibut"}[rand.Intn(3)]
		m.queueLogStyled(styleLogWarn, fmt.Sprintf("1055 — Pulled a %s. No groundfish permit — back it goes.", fish))
	}
	if result.TrapsLost > 0 {
		trapCost := float64(result.TrapsLost) * BoatModels[m.gs.BoatName].TrapCost
		if result.TrapsLost == 1 {
			m.queueLogStyled(styleLogWarn, fmt.Sprintf("1050 — Lost a trap out there. Gone. ($%.0f replacement)", trapCost))
		} else {
			m.queueLogStyled(styleLogDanger, fmt.Sprintf("1050 — Lost %d traps. Lines cut or swept. ($%.0f to replace)", result.TrapsLost, trapCost))
		}
	}

	// If event fires before the midday check, queue its description line
	if m.activeEvent != nil && (m.activeEvent.Time < "1100") {
		m.queueLogStyled(styleLogWarn, m.activeEvent.Desc)
	}

	if result.CatchLbs > 200 {
		m.queueLogStyled(styleGood, fmt.Sprintf("1100 — Killing it out here. %.0f lbs and counting.", result.CatchLbs*0.6))
	} else if result.CatchLbs > 80 {
		m.queueLog(fmt.Sprintf("1100 — Steady haul. %.0f lbs so far.", result.CatchLbs*0.6))
	} else {
		m.queueLogStyled(styleLogWarn, "1100 — Slim pickings. Traps running light.")
	}

	// If event fires after midday, queue its description line
	if m.activeEvent != nil && (m.activeEvent.Time >= "1100") {
		m.queueLogStyled(styleLogWarn, m.activeEvent.Desc)
	}

	// Build post-event queue (rest of day + debrief) stored separately
	m.postEventLines = nil
	if m.activeEvent == nil {
		// No event — everything goes in pendingLines
		m.queueLog(fmt.Sprintf("1430 — Last trap aboard. %.0f lbs total.", result.CatchLbs))
		m.queueLog("1600 — Back at the dock.")
		m.buildDebriefLines(result, &m.pendingLines)
	} else {
		// Event fires — post-event lines go in postEventLines
		postAdd := func(s string) {
			if !strings.Contains(s, "\x1b[") && s != "" {
				s = styleDefault.Render(s)
			}
			m.postEventLines = append(m.postEventLines, s)
		}
		postAdd(fmt.Sprintf("1430 — Last trap aboard. %.0f lbs total.", result.CatchLbs))
		postAdd("1600 — Back at the dock.")
		m.buildDebriefLines(result, &m.postEventLines)
	}
}

func (m *model) showEventPrompt() {
	if m.activeEvent == nil {
		return
	}
	m.addLog("")
	m.addLog(styleLogWarn.Bold(true).Render("  ── DECISION ──────────────────────────"))
	m.addLog(fmt.Sprintf("  %s", m.activeEvent.LabelA))
	m.addLog(fmt.Sprintf("  %s", m.activeEvent.LabelB))
	m.syncViewport()
}

func (m *model) resolveEvent(key string) {
	ev := m.activeEvent
	if ev == nil {
		return
	}
	result := m.eventHaulResult

	m.addLog("")
	switch ev.Type {
	case EventBerriedHen:
		if key == ev.KeyA { // keep
			lbs := 3.5
			m.gs.Freezer += lbs
			m.gs.TotalCatch += lbs
			if result != nil {
				result.CatchLbs += lbs
			}
			m.addLogStyled(styleLogGreen, "  You toss her in the tank. +3.5 lbs.")
			if rand.Float64() < 0.20 {
				fine := 1000.0
				m.gs.Money -= fine
				m.addLogStyled(styleLogDanger, "  Marine Patrol was watching. $1,000 fine.")
			}
		} else {
			m.addLog("  She hits the water swimming. Good fisherman.")
		}

	case EventSquareGrouper:
		if key == ev.KeyA { // keep
			m.gs.Money += 2500
			m.addLogStyled(styleLogGreen, "  $2,500 in the bilge. You didn't see anything.")
			if rand.Float64() < 0.15 {
				fine := 5000.0
				m.gs.Money -= fine
				m.addLogStyled(styleLogDanger, "  DEA boarded you at the dock. $5,000 fine. One day lost.")
				m.gs.Hungover = true // use Hungover to skip next day
			}
		} else {
			m.addLog("  CG thanks you on the radio. You feel okay about it.")
			if rand.Float64() < 0.40 {
				m.addLogStyled(styleLogGreen, "  Co-op comp'd your fuel today. Good karma.")
				m.gs.Money += float64(result.FuelUsed) * m.gs.DieselPrice
			}
		}

	case EventJonahCrabs:
		if key == ev.KeyA { // keep
			bonus := 40.0 + rand.Float64()*50.0
			m.gs.Money += bonus
			m.addLogStyled(styleLogGreen, fmt.Sprintf("  Jonah crabs brought in %s at the co-op.", moneyStr(bonus)))
		} else {
			m.addLog("  Tossed 'em back. Traps run cleaner without the crap.")
		}

	case EventGhostTrap:
		if key == ev.KeyA { // haul it
			lbs := 10.0 + rand.Float64()*20.0
			m.gs.Freezer += lbs
			m.gs.TotalCatch += lbs
			if result != nil {
				result.CatchLbs += lbs
			}
			m.addLogStyled(styleLogGreen, fmt.Sprintf("  Ghost trap had %.0f lbs in it. Yours now.", lbs))
		} else {
			m.addLog("  Left it. Not your gear, not your problem.")
		}

	case EventStormComing:
		if key == ev.KeyA { // push through
			m.addLog("  You push through. Storm hits hard on the run home.")
			dmg := 5.0 + rand.Float64()*10.0
			m.gs.Engine = math.Max(0, m.gs.Engine-dmg)
			m.gs.Hydraulics = math.Max(0, m.gs.Hydraulics-dmg*0.5)
			m.addLogStyled(styleLogWarn, fmt.Sprintf("  Took a beating: engine -%.0f%%, hydraulics -%.0f%%", dmg, dmg*0.5))
		} else { // head in
			// Lose ~40% of catch
			lost := m.gs.Freezer * 0.4
			m.gs.Freezer -= lost
			m.gs.TotalCatch -= lost
			if result != nil {
				result.CatchLbs -= lost
				if result.CatchLbs < 0 {
					result.CatchLbs = 0
				}
			}
			m.addLog("  You head in. Smart call. Storm was nasty.")
			m.addLogStyled(styleLogWarn, fmt.Sprintf("  Left %.0f lbs in the water. Made it home safe.", lost))
		}

	case EventBoatDistress:
		if key == ev.KeyA { // help
			m.addLog("  You haul ass to their position. They're taking on water fast.")
			roll := rand.Float64()
			switch {
			case roll < 0.40:
				lbs := 20.0 + rand.Float64()*15.0
				m.gs.Freezer += lbs
				m.gs.TotalCatch += lbs
				m.addLogStyled(styleLogGreen, fmt.Sprintf("  Captain tosses you a crate. %.0f lbs of select. 'Take it.'", lbs))
			case roll < 0.65:
				m.addLog("  Old timer hands you a bottle of Allen's. 'Buy you one sometime, kid.'")
				m.gs.DaysWithBooze = 0 // free night, no cost
			case roll < 0.80:
				m.addLog("  He pulls a scratch ticket from his wallet. 'Least I can do.'")
				winnings := scratchTicket()
				if winnings > 0 {
					m.gs.Money += float64(winnings)
					m.addLogStyled(styleLogGreen, fmt.Sprintf("  Scratch ticket: +$%d!", winnings))
				} else {
					m.addLog("  Scratch ticket: loser. Still felt good helping.")
				}
			case roll < 0.90:
				bonus := 0.50
				m.addLogStyled(styleLogGreen, fmt.Sprintf("  He radioed his co-op contact. You're getting +$%.2f/lb on today's catch.", bonus))
				m.gs.Money += result.CatchLbs * bonus
			default:
				m.addLog("  A nod and a wave. Good fisherman karma.")
			}
		} else {
			m.addLog("  You keep hauling. Someone else will get it.")
			if rand.Float64() < 0.20 {
				m.addLog(styleDim("  Heard about it at the co-op. Nobody said anything but you felt it."))
			}
		}
	}

	// Clear event, move post-event lines to pending, resume animation
	m.activeEvent = nil
	m.eventHaulResult = nil
	m.pendingLines = append(m.pendingLines, m.postEventLines...)
	m.postEventLines = nil
	m.phase = PhaseHauling
	m.addLog("")
	m.syncViewport()
}

func (m *model) chargeDockFee() {
	fee := 20.0
	m.gs.Money -= fee
	m.addLogStyled(styleLogExpense, fmt.Sprintf("  ⚓ Slip/mooring fee: -$%.0f", fee))
}

func (m *model) doSell() {
	if m.haul == nil && m.gs.Freezer == 0 {
		m.addLog("")
		m.addLog(m.logDivider(styleLogInfo, "CO-OP"))
		m.addLog("  Nothing to sell today.")
	} else {
		catchToSell := m.gs.Freezer
		// Use grade breakdown from today's haul if available; otherwise flat rate
		revenue := 0.0
		m.addLog("")
		m.addLog(m.logDivider(styleTitle, "CO-OP SALE"))
		if m.haul != nil && len(m.haul.Grades) > 0 {
			for _, g := range m.haul.Grades {
				m.addLog(fmt.Sprintf("  %-10s %.1f lbs @ $%.2f/lb", g.Name, g.Lbs, g.Price))
				revenue += g.Lbs * g.Price
			}
		} else {
			// Fallback: flat rate for freezer hold
			flatPrice := 5.75
			revenue = catchToSell * flatPrice
			m.addLog(fmt.Sprintf("  %.1f lbs @ $%.2f/lb (market avg)", catchToSell, flatPrice))
		}
		m.addLogStyled(styleLogRevenue, fmt.Sprintf("  Revenue: %s", moneyStr(revenue)))

		crewCut := m.gs.CrewWage
		bankPayment := 0.0
		if m.gs.BankLoan > 0 {
			bankPayment = math.Min(m.gs.BankLoan, revenue*0.1)
			m.gs.BankLoan -= bankPayment
		}

		net := revenue - crewCut - bankPayment

		if crewCut > 0 {
			m.addLogStyled(styleLogExpense, fmt.Sprintf("  Crew share: -%s", moneyStr(crewCut)))
		}
		if bankPayment > 0 {
			m.addLogStyled(styleLogExpense, fmt.Sprintf("  Bank payment: -%s", moneyStr(bankPayment)))
		}
		m.addLogStyled(styleLogRevenue, fmt.Sprintf("  Net: %s", moneyStr(net)))

		// Add bycatch revenue
		if m.haul != nil {
			jonahRev := m.haul.JonahCrabLbs * 0.75
			rockRev := m.haul.RockCrabLbs * 0.35
			groundfishRev := m.haul.GroundfishLbs * groundfishPrice(m.haul.GroundfishName)
			bycatchRev := jonahRev + rockRev + groundfishRev
			net += bycatchRev
			revenue += bycatchRev
		}
		m.gs.Money += net
		m.gs.Freezer = 0
		m.gs.TotalRevenue += revenue
	}

	m.addLog("")
	m.addLogStyled(styleLabel, fmt.Sprintf("  Bank balance: %s", moneyStr(m.gs.Money)))
	if m.gs.BankLoan > 0 {
		m.addLogStyled(styleLogExpense, fmt.Sprintf("  Outstanding loan: %s", moneyStr(m.gs.BankLoan)))
	}
	m.addLog("")
	saveGame(m.gs)
	m.syncViewport()
	m.phase = PhaseEvening
	m.doEvening()
}

func (m *model) doBuy() {
	items := []struct {
		name  string
		cost  float64
		apply func()
	}{
		{"Bait (50 lbs)", 30.00, func() {
			cap := BoatModels[m.gs.BoatName].BaitCap
			if m.gs.Bait >= cap { m.confirmBuy = "Bait storage full!"; return }
			add := min(50, cap-m.gs.Bait)
			m.gs.Bait += add
			m.confirmBuy = fmt.Sprintf("Loaded %d lbs herring", add)
		}},
		{"Bait (200 lbs)", 110.00, func() {
			cap := BoatModels[m.gs.BoatName].BaitCap
			if m.gs.Bait >= cap { m.confirmBuy = "Bait storage full!"; return }
			add := min(200, cap-m.gs.Bait)
			cost := float64(add) * m.gs.BaitPrice
			if m.gs.Money < cost { m.confirmBuy = fmt.Sprintf("Need %s", moneyStr(cost)); return }
			m.gs.Money -= cost
			m.gs.Bait += add
			m.confirmBuy = fmt.Sprintf("Loaded %d lbs herring for %s", add, moneyStr(cost))
		}},
		{"Fuel", 0, func() {
			boat := BoatModels[m.gs.BoatName]
			needed := boat.FuelCap - m.gs.Fuel
			if needed <= 0 {
				m.confirmBuy = "Tank is already full."
				return
			}
			cost := float64(needed) * m.gs.DieselPrice
			if m.gs.Money >= cost {
				m.gs.Money -= cost
				m.gs.Fuel = boat.FuelCap
				m.confirmBuy = fmt.Sprintf("Filled %d gal for %s", needed, moneyStr(cost))
			} else {
				// Can afford partial — fill what they can pay for
				canAfford := int(m.gs.Money / m.gs.DieselPrice)
				if canAfford > 0 {
					m.gs.Money -= float64(canAfford) * m.gs.DieselPrice
					m.gs.Fuel += canAfford
					m.confirmBuy = fmt.Sprintf("Partial fill: %d gal for %s (short on cash)", canAfford, moneyStr(float64(canAfford)*m.gs.DieselPrice))
				} else {
					m.confirmBuy = "Not enough cash for even a gallon."
				}
			}
		}},
		{"Add Traps", 0, func() {
			boat := BoatModels[m.gs.BoatName]
			increment, costPer := trapIncrement(boat.MaxTraps)
			if m.gs.Traps >= boat.MaxTraps {
				m.confirmBuy = "At max trap capacity for this vessel!"
				return
			}
			add := increment
			if m.gs.Traps+add > boat.MaxTraps {
				add = boat.MaxTraps - m.gs.Traps
			}
			cost := float64(add) * costPer
			if m.gs.Money >= cost {
				m.gs.Money -= cost
				m.gs.Traps += add
				m.confirmBuy = fmt.Sprintf("Added %d traps for %s", add, moneyStr(cost))
			} else {
				m.confirmBuy = fmt.Sprintf("Need %s — short by %s", moneyStr(cost), moneyStr(cost-m.gs.Money))
			}
		}},
		{"Repair Engine (to 100%)", 0, func() {
			if m.gs.Engine >= 100 {
				m.confirmBuy = "Engine is already in top shape!"
				return
			}
			cost := (100 - m.gs.Engine) * 12.0
			if m.gs.Money >= cost {
				m.gs.Money -= cost
				m.gs.Engine = 100
				m.confirmBuy = fmt.Sprintf("Engine overhauled for %s", moneyStr(cost))
			} else {
				m.confirmBuy = fmt.Sprintf("Need %s — short by %s", moneyStr(cost), moneyStr(cost-m.gs.Money))
			}
		}},
		{"Replace Zincs", 0, func() {
			if m.gs.Zincs >= 100 {
				m.confirmBuy = "Zincs are brand new!"
				return
			}
			cost := (100 - m.gs.Zincs) * 3.0
			if m.gs.Money >= cost {
				m.gs.Money -= cost
				m.gs.Zincs = 100
				m.confirmBuy = fmt.Sprintf("Zincs replaced for %s", moneyStr(cost))
			} else {
				m.confirmBuy = fmt.Sprintf("Need %s — short by %s", moneyStr(cost), moneyStr(cost-m.gs.Money))
			}
		}},
		{"Repair Hydraulics (to 100%)", 0, func() {
			if m.gs.Hydraulics >= 100 {
				m.confirmBuy = "Hydraulics are solid!"
				return
			}
			cost := (100 - m.gs.Hydraulics) * 8.0
			if m.gs.Money >= cost {
				m.gs.Money -= cost
				m.gs.Hydraulics = 100
				m.confirmBuy = fmt.Sprintf("Hydraulics rebuilt for %s", moneyStr(cost))
			} else {
				m.confirmBuy = fmt.Sprintf("Need %s — short by %s", moneyStr(cost), moneyStr(cost-m.gs.Money))
			}
		}},
		// Equipment (indices 7-11)
		{"Radar", 2500, func() {
			if m.gs.HasRadar { m.confirmBuy = "Already installed."; return }
			if m.gs.Money < 1200 { m.confirmBuy = fmt.Sprintf("Need %s — short by %s", moneyStr(1200), moneyStr(1200-m.gs.Money)); return }
			m.gs.Money -= 1200
			m.gs.HasRadar = true
			m.confirmBuy = "Radar installed. Fish in fog."
		}},
		{"GPS/Chartplotter", 1500, func() {
			if m.gs.HasGPS { m.confirmBuy = "Already installed."; return }
			if m.gs.Money < 800 { m.confirmBuy = fmt.Sprintf("Need %s — short by %s", moneyStr(800), moneyStr(800-m.gs.Money)); return }
			m.gs.Money -= 800
			m.gs.HasGPS = true
			m.confirmBuy = "GPS installed. Zones F and G unlocked."
		}},
		{"VHF Radio", 500, func() {
			if m.gs.HasVHF { m.confirmBuy = "Already installed."; return }
			if m.gs.Money < 250 { m.confirmBuy = fmt.Sprintf("Need %s — short by %s", moneyStr(250), moneyStr(250-m.gs.Money)); return }
			m.gs.Money -= 250
			m.gs.HasVHF = true
			m.confirmBuy = "VHF installed. You can hear channel 16 now."
		}},
		{"Upgraded Hauler", 2000, func() {
			if m.gs.HasUpgHauler { m.confirmBuy = "Already installed."; return }
			if m.gs.Money < 600 { m.confirmBuy = fmt.Sprintf("Need %s — short by %s", moneyStr(600), moneyStr(600-m.gs.Money)); return }
			m.gs.Money -= 600
			m.gs.HasUpgHauler = true
			m.confirmBuy = "Hauler upgraded. Hydraulics will thank you."
		}},
		{"Depth Sounder", 2500, func() {
			if m.gs.HasDepthSound { m.confirmBuy = "Already installed."; return }
			if m.gs.Money < 400 { m.confirmBuy = fmt.Sprintf("Need %s — short by %s", moneyStr(400), moneyStr(400-m.gs.Money)); return }
			m.gs.Money -= 400
			m.gs.HasDepthSound = true
			m.confirmBuy = "Depth sounder installed. You can read the bottom now."
		}},
		{"Exhaust Heat Exchanger", 5000, func() {
			if m.gs.HasExhaustHX { m.confirmBuy = "Already installed."; return }
			if m.gs.Money < 3000 { m.confirmBuy = fmt.Sprintf("Need %s — short by %s", moneyStr(3000), moneyStr(3000-m.gs.Money)); return }
			m.gs.Money -= 3000
			m.gs.HasExhaustHX = true
			m.confirmBuy = "Heat exchanger installed. Engine'll run cooler and last longer."
		}},
	}

	if m.marketCursor >= len(items) {
		return
	}

	item := items[m.marketCursor]
	if item.cost > 0 {
		if m.gs.Money >= item.cost {
			m.gs.Money -= item.cost
			item.apply()
			m.confirmBuy = fmt.Sprintf("Bought: %s", item.name)
		} else {
			m.confirmBuy = fmt.Sprintf("Not enough cash! Need %s", moneyStr(item.cost))
		}
	} else {
		item.apply()
	}
	saveGame(m.gs)
}

// Fix #2: removed styleLog.Render() wrapper — each line is already pre-styled
func (m *model) syncViewport() {
	content := strings.Join(m.logLines, "\n")
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
}

// syncAltViewport re-renders the current non-log screen into the alt viewport
func (m *model) syncAltViewport() {
	var content string
	switch m.screen {
	case ScreenMaintenance:
		content = m.viewMaintenanceContent()
	case ScreenDock:
		content = m.viewDockContent()
	case ScreenMarket:
		content = m.viewMarketContent()
	}
	m.altVP.SetContent(content)
}

// ─── View ─────────────────────────────────────────────────────────────────────

func (m model) View() string {
	if m.width == 0 {
		return "Loading HIGHLINER..."
	}

	var b strings.Builder

	// Fix #6: full-width header using lipgloss, no manual gap math hacks
	left := fmt.Sprintf(" 🦞 HIGHLINER  —  %s ", m.subtitle)
	right := fmt.Sprintf(" Day %d | %s (%s) | $%.0f ", m.gs.Day, m.gs.VesselName, m.gs.BoatName, m.gs.Money)
	gapW := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gapW < 0 {
		gapW = 0
	}
	headerLine := left + strings.Repeat(" ", gapW) + right
	header := styleHeader.Width(m.width).Render(headerLine)
	b.WriteString(header)
	b.WriteString("\n")

	// Tab bar
	tabs := []struct {
		label  string
		screen Screen
	}{
		{"[1/] LOG", ScreenLog},
		{"[2/M] MAINT", ScreenMaintenance},
		{"[3/D] DOCK", ScreenDock},
		{"[4/W] WHARF", ScreenMarket},
	}
	for _, t := range tabs {
		if t.screen == m.screen {
			b.WriteString(styleTabActive.Render(t.label))
		} else {
			b.WriteString(styleTab.Render(t.label))
		}
		b.WriteString(" ")
	}
	b.WriteString("\n")
	// Persistent stats bar — always visible on every screen
	boat := BoatModels[m.gs.BoatName]
	statsBar := fmt.Sprintf("  Cash: %s   Fuel: %d/%d gal   Bait: %d/%d lbs   Traps: %d/%d",
		moneyStr(m.gs.Money),
		m.gs.Fuel, boat.FuelCap,
		m.gs.Bait, boat.BaitCap,
		m.gs.Traps, boat.MaxTraps)
	b.WriteString(lipgloss.NewStyle().Foreground(colorBrightWhite).Background(lipgloss.Color("235")).Width(m.width).Render(statsBar))
	b.WriteString("\n")
	// Fix #7: consistent separator — full-width cyan rule
	b.WriteString(lipgloss.NewStyle().Foreground(colorCyan).Render(strings.Repeat("─", m.width)))
	b.WriteString("\n")

	// Fix #5: viewport dimensions are set in WindowSizeMsg, not here
	switch m.screen {
	case ScreenLog:
		b.WriteString(m.viewport.View())
	case ScreenMaintenance, ScreenDock, ScreenMarket:
		// Fix #8: alt screens use a real viewport — content scrolls
		b.WriteString(m.altVP.View())
	}

	// Fix #3: status bar fills terminal width
	b.WriteString("\n")

	weatherColor := colorBrightGreen
	if m.weather.Type == WeatherFog {
		weatherColor = colorBrightAmber
	} else if m.weather.Type == WeatherSCA || m.weather.Type == WeatherGale {
		weatherColor = colorBrightRed
	}

	sbStyle := lipgloss.NewStyle().Background(colorCyan).Foreground(colorBlack)
	wStyle := lipgloss.NewStyle().Background(colorCyan).Foreground(weatherColor).Bold(true)

	weatherSummary := fmt.Sprintf("%s  %dkt", m.weather.Type, m.weather.WindKts)
	if m.weather.GustKts > 0 {
		weatherSummary += fmt.Sprintf("G%d", m.weather.GustKts)
	}
	weatherSummary += fmt.Sprintf("  Seas %d-%dft", m.weather.SeasFt, m.weather.SeasFtHigh)
	leftBar := sbStyle.Render(" Weather: ") +
		wStyle.Render(weatherSummary) +
		sbStyle.Render(fmt.Sprintf("  Phase: %s", phaseName(m.phase)))
	if m.saveMsg != "" {
		leftBar += lipgloss.NewStyle().Background(colorCyan).Foreground(colorBrightGreen).Render("  " + m.saveMsg)
	}

	scrollHint := ""
	if m.screen == ScreenLog {
		scrollHint = sbStyle.Render(" [↑↓/PgUp/PgDn] scroll ") 
	}
	rightBar := scrollHint + sbStyle.Render(" [Q] Save+Quit ")

	leftW := lipgloss.Width(leftBar)
	rightW := lipgloss.Width(rightBar)
	padW := m.width - leftW - rightW
	if padW < 0 {
		padW = 0
	}

	b.WriteString(leftBar)
	b.WriteString(sbStyle.Render(strings.Repeat(" ", padW)))
	b.WriteString(rightBar)

	return styleBase.Render(b.String())
}

// ─── Screen Content Builders ─────────────────────────────────────────────────

// sectionHeader renders a full-width DOS-style section title bar
func sectionHeader(title string, width int) string {
	if width < 4 {
		width = 80
	}
	// ══ TITLE ══════════...
	pre := "══ " + title + " "
	pad := width - len(pre) - 2 // -2 for the trailing ══ minimum
	if pad < 0 {
		pad = 0
	}
	line := pre + strings.Repeat("═", pad+2)
	return "\n" + styleTitle.Render(line) + "\n\n"
}

// Fix #1: viewMaintenanceContent replaces hardcoded ASCII box with lipgloss border
func (m model) viewMaintenanceContent() string {
	var b strings.Builder

	b.WriteString(sectionHeader("BOAT MAINTENANCE DIAGNOSTICS", m.width))

	boat := BoatModels[m.gs.BoatName]
	b.WriteString(fmt.Sprintf("  %s  %s\n",
		styleLabel.Render("Vessel:"),
		styleValue.Render(fmt.Sprintf("%s — %s (%d ft)", m.gs.VesselName, m.gs.BoatName, boat.Length))))
	b.WriteString("\n")

	components := []struct {
		name   string
		health float64
		repair string
	}{
		{"Engine", m.gs.Engine, fmt.Sprintf("$%.0f to repair", (100-m.gs.Engine)*12.0)},
		{"Zincs", m.gs.Zincs, fmt.Sprintf("$%.0f to replace", (100-m.gs.Zincs)*3.0)},
		{"Hydraulics", m.gs.Hydraulics, fmt.Sprintf("$%.0f to repair", (100-m.gs.Hydraulics)*8.0)},
	}

	for _, c := range components {
		bar := healthBar(c.health, 30)
		status := healthStatus(c.health)
		var hs lipgloss.Style
		if c.health >= 60 {
			hs = styleGood
		} else if c.health >= 30 {
			hs = styleWarn
		} else {
			hs = styleDanger
		}
		b.WriteString(fmt.Sprintf("  %-14s %s %s  %s\n",
			styleLabel.Render(c.name+":"),
			bar,
			hs.Render(fmt.Sprintf("%5.1f%%", c.health)),
			hs.Render(status)))
		b.WriteString(fmt.Sprintf("  %-14s %s\n\n", "", styleDim(c.repair)))
	}

	b.WriteString("\n")
	b.WriteString(subHeader("REPAIR TIPS", m.width))
	b.WriteString("  Engine below 25% = risk of breakdown at sea\n")
	b.WriteString("  Zincs below 20% = hull corrodes faster\n")
	b.WriteString("  Hydraulics below 25% = hauler may fail mid-haul\n")
	b.WriteString("\n")
	b.WriteString(styleKey.Render("  Press [W] to go to Wharf for repairs"))
	b.WriteString("\n")
	b.WriteString(styleDim("  [↑↓/JK] Scroll"))

	return b.String()
}

// label pads a plain string to width BEFORE applying style, so ANSI codes don't break column alignment
func label(s string, width int) string {
	return styleLabel.Render(fmt.Sprintf("%-*s", width, s))
}

// trapIncrement returns (how many traps to add, cost per trap) based on boat capacity
// zoneBlocked returns true if the current boat can't safely reach this zone
func (m *model) zoneBlockReason(zoneIdx int) string {
	boat := BoatModels[m.gs.BoatName]
	if m.weather.Type == WeatherFog && !m.gs.HasRadar && zoneIdx > 0 {
		return "✗ fog — no radar"
	}
	if !m.gs.HasGPS && Zones[zoneIdx].SteamHours > 8.0 {
		return "✗ need GPS/chartplotter"
	}
	if boat.MaxSteamHrs > 0 && Zones[zoneIdx].SteamHours > boat.MaxSteamHrs {
		return "✗ too far offshore for this vessel"
	}
	return ""
}

func (m *model) zoneBlocked(zoneIdx int) bool {
	return m.zoneBlockReason(zoneIdx) != ""
}

func trapIncrement(maxTraps int) (int, float64) {
	switch {
	case maxTraps <= 40: // Eastern 22
		return 5, 175.0 // $175/trap — commercial wire trap
	case maxTraps <= 400: // Calvin Beal, Duffy
		return 25, 165.0 // $165/trap — bulk order discount
	default: // Young Bros, Wesmac
		return 50, 150.0 // $150/trap — volume pricing
	}
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

func (m model) viewDockContent() string {
	var b strings.Builder
	boat := BoatModels[m.gs.BoatName]

	b.WriteString(sectionHeader("DOCK MANAGEMENT", m.width))

	vesselDisplay := m.gs.VesselName
	if vesselDisplay == "" {
		vesselDisplay = m.gs.BoatName
	} else {
		vesselDisplay = fmt.Sprintf("%s — %s (%d ft)", m.gs.VesselName, m.gs.BoatName, boat.Length)
	}
	b.WriteString(fmt.Sprintf("  %s  %s\n\n", styleLabel.Render("Vessel:"), styleValue.Render(vesselDisplay)))

		b.WriteString(subHeader("GEAR & SUPPLIES", m.width))
	b.WriteString(fmt.Sprintf("  %s %s / %d max\n", label("Traps:", 12), styleValue.Render(fmt.Sprintf("%d", m.gs.Traps)), boat.MaxTraps))
	b.WriteString(fmt.Sprintf("  %s %s / %d lbs cap\n", label("Bait:", 12), styleValue.Render(fmt.Sprintf("%d", m.gs.Bait)), boat.BaitCap))
	b.WriteString(fmt.Sprintf("  %s %s / %d gal cap\n", label("Fuel:", 12), styleValue.Render(fmt.Sprintf("%d gal", m.gs.Fuel)), boat.FuelCap))
	b.WriteString(fmt.Sprintf("  %s %s lbs\n\n", label("Hold:", 12), styleValue.Render(fmt.Sprintf("%.1f", m.gs.Freezer))))

	b.WriteString(subHeader("FINANCES", m.width))
	b.WriteString(fmt.Sprintf("  %s %s\n", label("Cash:", 12), moneyStyled(m.gs.Money)))
	if m.gs.BankLoan > 0 {
		b.WriteString(fmt.Sprintf("  %s %s\n", label("Loan:", 12), styleDanger.Render(moneyStr(m.gs.BankLoan))))
	}
	b.WriteString(fmt.Sprintf("  %s %d days\n", label("Days out:", 12), m.gs.Day))
	b.WriteString(fmt.Sprintf("  %s %.0f lbs\n", label("Total catch:", 12), m.gs.TotalCatch))
	b.WriteString(fmt.Sprintf("  %s %s\n\n", label("Revenue:", 12), styleValue.Render(moneyStr(m.gs.TotalRevenue))))

	b.WriteString(subHeader("LICENSES", m.width))
	crabStatus := styleDim("not licensed — Jonah/rock crabs are throwbacks")
	if m.gs.HasCrabPermit {
		crabStatus = styleGood.Render("✓ licensed")
	}
	groundfishStatus := styleDim("not licensed — monkfish/sea bass are throwbacks")
	if m.gs.HasGroundfishPermit {
		groundfishStatus = styleGood.Render("✓ licensed")
	}
	b.WriteString(fmt.Sprintf("  %-22s %s\n", styleLabel.Render("Crab Permit ($500):"), crabStatus))
	b.WriteString(fmt.Sprintf("  %-22s %s\n\n", styleLabel.Render("Groundfish Permit ($1,500):"), groundfishStatus))
	if !m.gs.HasCrabPermit || !m.gs.HasGroundfishPermit {
		b.WriteString(styleDim("  Buy licenses at the harbormaster's office: [B]uy Crab / [G]roundfish\n\n"))
	}
	if m.confirmBuy != "" && m.screen == ScreenDock {
		b.WriteString(styleLogRevenue.Render(fmt.Sprintf("  ✓ %s", m.confirmBuy)) + "\n\n")
	}

	b.WriteString(subHeader("FLEET PROGRESSION", m.width))
	fleetOrder := []string{"Eastern 22", "Calvin Beal 34", "Duffy 35", "Young Bros 40", "Wesmac 46"}
	for _, name := range fleetOrder {
		bm := BoatModels[name]
		if name == m.gs.BoatName {
			b.WriteString(fmt.Sprintf("  ► %s  %d traps  %d ft\n",
				styleGood.Render(fmt.Sprintf("%-16s", name)), bm.MaxTraps, bm.Length))
		} else if bm.Cost <= int(m.gs.Money) {
			b.WriteString(fmt.Sprintf("    %s  %d traps  %s\n",
				styleValue.Render(fmt.Sprintf("%-16s", name)), bm.MaxTraps, styleGood.Render(fmt.Sprintf("$%d — can afford!", bm.Cost))))
		} else {
			b.WriteString(fmt.Sprintf("    %s  %d traps  %s\n",
				styleDim(fmt.Sprintf("%-16s", name)), bm.MaxTraps, styleDim(fmt.Sprintf("$%d", bm.Cost))))
		}
	}
	b.WriteString("\n")
	b.WriteString(styleDim("  [↑↓/JK] Scroll  [W] Go to Wharf"))
	b.WriteString("\n")

	return b.String()
}

// Fix #1: viewMarketContent replaces hardcoded ASCII box with lipgloss border
func (m model) viewMarketContent() string {
	var b strings.Builder

	b.WriteString(sectionHeader("WHARF MARKET & REPAIRS", m.width))

	b.WriteString(fmt.Sprintf("  Cash: %s   Fuel: %d/%d gal   Bait: %d/%d lbs\n",
		moneyStyled(m.gs.Money),
		m.gs.Fuel, BoatModels[m.gs.BoatName].FuelCap,
		m.gs.Bait, BoatModels[m.gs.BoatName].BaitCap))

	// Today's co-op dock prices
	p := m.gs.DailyPrices
	gradeNames := [6]string{"Chix", "Qtr", "Select", "Jumbo", "Super", "Cull"}
	b.WriteString("  ")
	for i, name := range gradeNames {
		b.WriteString(fmt.Sprintf("%s $%.2f  ", styleLabel.Render(name), p[i]))
	}
	b.WriteString("\n\n")

	if m.confirmBuy != "" {
		b.WriteString(styleLogRevenue.Render(fmt.Sprintf("  ✓ %s", m.confirmBuy)) + "\n\n")
	}

	type wharfItem struct {
		name string
		cost string
		desc string
	}

	// Supplies section
	supplyItems := []wharfItem{
		{"Bait (50 lbs)", fmt.Sprintf("$%.0f", 50*m.gs.BaitPrice), fmt.Sprintf("Herring @ $%.2f/lb — roughly one day on 20 traps", m.gs.BaitPrice)},
		{"Bait (200 lbs)", fmt.Sprintf("$%.0f", 200*m.gs.BaitPrice), fmt.Sprintf("Bulk herring @ $%.2f/lb — 3-4 days supply", m.gs.BaitPrice)},
		{"Fuel", func() string {
			needed := BoatModels[m.gs.BoatName].FuelCap - m.gs.Fuel
			if needed <= 0 { return "full" }
			return fmt.Sprintf("$%.0f", float64(needed)*m.gs.DieselPrice)
		}(), fmt.Sprintf("Diesel $%.2f/gal — need %d gal to top off", m.gs.DieselPrice, BoatModels[m.gs.BoatName].FuelCap-m.gs.Fuel)},
		{"Add Traps", func() string {
			inc, cpt := trapIncrement(BoatModels[m.gs.BoatName].MaxTraps)
			return fmt.Sprintf("$%.0f", float64(inc)*cpt)
		}(), func() string {
			boat := BoatModels[m.gs.BoatName]
			inc, cpt := trapIncrement(boat.MaxTraps)
			return fmt.Sprintf("+%d traps @ $%.0f/ea — %d/%d on boat", inc, cpt, m.gs.Traps, boat.MaxTraps)
		}()},
	}

	// Repairs section — show actual current cost
	engineCost := (100 - m.gs.Engine) * 12.0
	zincsCost  := (100 - m.gs.Zincs) * 3.0
	hydCost    := (100 - m.gs.Hydraulics) * 8.0
	repairItems := []wharfItem{
		{"Repair Engine", func() string {
			if m.gs.Engine >= 100 { return "good" }
			return fmt.Sprintf("$%.0f", engineCost)
		}(), fmt.Sprintf("%.0f%% → 100%%  |  %s", m.gs.Engine, healthStr(m.gs.Engine))},
		{"Replace Zincs", func() string {
			if m.gs.Zincs >= 100 { return "good" }
			return fmt.Sprintf("$%.0f", zincsCost)
		}(), fmt.Sprintf("%.0f%% → 100%%  |  %s", m.gs.Zincs, healthStr(m.gs.Zincs))},
		{"Repair Hydraulics", func() string {
			if m.gs.Hydraulics >= 100 { return "good" }
			return fmt.Sprintf("$%.0f", hydCost)
		}(), fmt.Sprintf("%.0f%% → 100%%  |  %s", m.gs.Hydraulics, healthStr(m.gs.Hydraulics))},
	}

			// Equipment items
	equipItems := []wharfItem{
		{"Radar", func() string {
			if m.gs.HasRadar { return "owned" }
			return "$2,500"
		}(), func() string {
			if m.gs.HasRadar { return "✓ fish all zones in fog" }
			return "fish all zones in fog"
		}()},
		{"GPS/Chartplotter", func() string {
			if m.gs.HasGPS { return "owned" }
			return "$1,500"
		}(), func() string {
			if m.gs.HasGPS { return "✓ unlocks zones F and G" }
			return "unlocks zones F and G"
		}()},
		{"VHF Radio", func() string {
			if m.gs.HasVHF { return "owned" }
			return "$500"
		}(), func() string {
			if m.gs.HasVHF { return "✓ weather forecast + distress events" }
			return "weather forecast + distress events"
		}()},
		{"Upgraded Hauler", func() string {
			if m.gs.HasUpgHauler { return "owned" }
			return "$2,000"
		}(), func() string {
			if m.gs.HasUpgHauler { return "✓ slower hydraulic wear" }
			return "slower hydraulic wear"
		}()},
		{"Depth Sounder", func() string {
			if m.gs.HasDepthSound { return "owned" }
			return "$2,500"
		}(), func() string {
			if m.gs.HasDepthSound { return "✓ full efficiency in deep zones" }
			return "full efficiency in deep zones (D-G)"
		}()},
		{"Exhaust Heat Exchanger", func() string {
			if m.gs.HasExhaustHX { return "owned" }
			return "$5,000"
		}(), func() string {
			if m.gs.HasExhaustHX { return "✓ engine runs cooler, less wear" }
			return "reduces engine wear per haul"
		}()},
	}

	allItems := append(supplyItems, repairItems...)
	allItems = append(allItems, equipItems...)

	renderItems := func(items []wharfItem, offset int) {
		for i, item := range items {
			idx := offset + i
			var lineStyle lipgloss.Style
			cursor := "  "
			if idx == m.marketCursor {
				cursor = "► "
				lineStyle = styleSelected
			} else {
				lineStyle = styleValue
			}
			line := fmt.Sprintf("%-24s %-16s %s", item.name, item.cost, item.desc)
			b.WriteString(fmt.Sprintf("  %s%s\n", cursor, lineStyle.Render(line)))
		}
	}

	_ = allItems // used by doBuy via m.marketCursor

	b.WriteString(subHeader("SUPPLIES", m.width))
	renderItems(supplyItems, 0)
	b.WriteString("\n")
	b.WriteString(subHeader("REPAIRS", m.width))
	renderItems(repairItems, len(supplyItems))
	b.WriteString("\n")
	b.WriteString(subHeader("EQUIPMENT", m.width))
	renderItems(equipItems, len(supplyItems)+len(repairItems))

	b.WriteString("\n")
	b.WriteString(styleKey.Render("  [↑↓/JK] Navigate   [ENTER] Buy   [1-4] Switch tabs"))

	b.WriteString("\n\n")
	b.WriteString(subHeader("BOAT UPGRADES (not yet implemented)", m.width))
	fleetOrder := []string{"Calvin Beal 34", "Duffy 35", "Young Bros 40", "Wesmac 46"}
	for _, name := range fleetOrder {
		bm := BoatModels[name]
		avail := m.gs.Money >= float64(bm.Cost)
		style := styleDanger
		if avail {
			style = styleGood
		}
		b.WriteString(fmt.Sprintf("  %s  $%-10d  %d traps  %s\n",
			style.Render(fmt.Sprintf("%-16s", name)),
			bm.Cost,
			bm.MaxTraps,
			styleDim(fmt.Sprintf("%d ft", bm.Length))))
	}

	return b.String()
}

// Keep old names as thin wrappers so any future callers don't break
func (m model) viewMaintenance() string { return m.viewMaintenanceContent() }
func (m model) viewDock() string        { return m.viewDockContent() }
func (m model) viewMarket() string      { return m.viewMarketContent() }

// ─── Helpers ─────────────────────────────────────────────────────────────────

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
