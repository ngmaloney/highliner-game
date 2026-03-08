package main

import (
	"strings"
	"time"

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
	ScreenChart
)

// ─── Event Types ─────────────────────────────────────────────────────────────

type EventType int

const (
	EventNone EventType = iota
	EventBerriedHen
	EventSquareGrouper
	EventJonahCrabs
	EventBoatDistress
	EventGhostTrap
	EventStormComing
	EventNeighborTrap
	// Positive events
	EventHotSet
	EventFlatlander
	EventOldTimer
	EventSunkTrap
	EventGrayMarketHalibut
	// Negative events
	EventSealRaid
	EventCGCheck
	EventEngineTempHigh
	EventHumpback
	EventRivalBoat
	EventWardenQuestions
	// Positive events
	EventDoubleLoaded
	EventHelpNeighbor
)

type RandomEvent struct {
	Type   EventType
	Time   string // e.g. "1045"
	Desc   string // log line shown before decision
	KeyA   string // key to press
	LabelA string // label for option A
	KeyB   string // key to press
	LabelB string // label for option B
}

// ─── Model ────────────────────────────────────────────────────────────────────

type model struct {
	gs                *GameState
	screen            Screen
	phase             Phase
	weather           Weather
	selectedZone      int
	currentZone       Zone
	haul              *HaulResult
	logLines          []string
	pendingLines      []string     // queued lines for animated haul
	postEventLines    []string     // lines to resume after event decision
	activeEvent       *RandomEvent // current decision event, nil if none
	eventHaulResult   *HaulResult  // stored so event consequences can modify catch
	viewport          viewport.Model
	altVP             viewport.Model // for maintenance/dock/market screens
	width             int
	height            int
	confirmBuy        string // for market confirmations
	buyAmount         int
	marketCursor      int
	saveMsg           string
	subtitle          string
	initialized       bool
}

// tickMsg is reserved for future periodic ticks
type tickMsg struct{}

// ─── Animated Haul ────────────────────────────────────────────────────────────

type haulTickMsg struct{}

func tickHaul() tea.Cmd {
	return tea.Tick(280*time.Millisecond, func(t time.Time) tea.Msg {
		return haulTickMsg{}
	})
}

// styleDefault is used to ensure plain text is styled with bright white
var styleDefault = lipgloss.NewStyle().Foreground(colorBrightWhite)

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

func (m *model) addLog(s string) {
	if !strings.Contains(s, "\x1b[") && s != "" {
		s = styleDefault.Render(s)
	}
	m.logLines = append(m.logLines, s)
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

		case "1", "h":
			if m.phase != PhaseZoneSelect && m.phase != PhaseEvening && m.phase != PhaseDecision && m.phase != PhaseHauling {
				m.screen = ScreenLog
				m.confirmBuy = ""
				m.syncViewport()
			} else {
				return m.handlePhaseKey(msg.String())
			}
		case "2", "v":
			if m.phase != PhaseZoneSelect && m.phase != PhaseEvening && m.phase != PhaseDecision && m.phase != PhaseHauling {
				m.screen = ScreenDock
				m.confirmBuy = ""
				m.syncAltViewport()
			} else {
				return m.handlePhaseKey(msg.String())
			}
		case "3", "w":
			if m.phase != PhaseZoneSelect && m.phase != PhaseEvening && m.phase != PhaseDecision && m.phase != PhaseHauling {
				m.screen = ScreenMarket
				m.confirmBuy = ""
				m.marketCursor = 0
				m.syncAltViewport()
			} else {
				return m.handlePhaseKey(msg.String())
			}
		case "4", "g":
			if m.phase != PhaseZoneSelect && m.phase != PhaseEvening && m.phase != PhaseDecision && m.phase != PhaseHauling {
				m.screen = ScreenChart
				m.syncAltViewport()
			} else {
				return m.handlePhaseKey(msg.String())
			}

		default:
			// Scroll alt viewport on non-interactive screens
			if m.screen == ScreenDock {
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
							m.confirmBuy = "Need $500 — short by " + moneyStr(500-m.gs.Money)
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
						boat := BoatModels[m.gs.BoatName]
						if m.gs.HasGroundfishPermit {
							m.confirmBuy = "Already licensed for groundfish."
						} else if boat.Length < 34 {
							m.confirmBuy = "NOAA requires a vessel 34 ft or larger for a groundfish permit."
						} else if m.gs.Money < 1500 {
							m.confirmBuy = "Need $1,500 — short by " + moneyStr(1500-m.gs.Money)
						} else {
							m.gs.Money -= 1500
							m.gs.HasGroundfishPermit = true
							m.confirmBuy = "Groundfish permit issued. Monkfish, cusk, and halibut are yours to keep."
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
	if m.screen == ScreenLog && m.phase != PhaseDecision && m.phase != PhaseHauling && m.phase != PhaseEvening {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}
