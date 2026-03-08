package main

import (
	"fmt"
	"math"
	"math/rand"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m model) handlePhaseKey(key string) (model, tea.Cmd) {
	// Market input takes priority
	if m.screen == ScreenMarket {
		switch key {
		case "up", "k":
			if m.marketCursor > 0 {
				m.marketCursor--
				m.syncAltViewport()
			}
			return m, nil
		case "down", "j":
			if m.marketCursor < 20 {
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
					crabTag := ""
					if m.gs.HotCrabZone == z.ID {
						if m.gs.HasCrabPermit {
							crabTag = styleLogGreen.Render("  🦀 crabs running")
						} else {
							crabTag = styleLogWarn.Render("  🦀 crabby today")
						}
					}
					if m.zoneBlocked(i) {
						reason := m.zoneBlockReason(i)
						m.addLog(styleDim(fmt.Sprintf("  [%d] %s (%s) — %s  %s", i+1, z.ID, z.Name, z.Description, reason)))
					} else {
						m.addLog(lipgloss.NewStyle().Foreground(colorBrightWhite).Render(fmt.Sprintf("  [%d] %s (%s) — %s", i+1, z.ID, z.Name, z.Description)) + crabTag)
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
					return m, nil
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
			if len(m.pendingLines) > 0 {
				return m, tickHaul()
			}
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
			if m.gs.Money >= 20 {
				m.gs.Money -= 20
				winnings := scratchTicket()
				m.addLog("")
				if winnings == 0 {
					m.addLog("  Scratch ticket: loser. Threw it in the harbor.")
				} else if winnings >= 500 {
					m.gs.Money += float64(winnings)
					m.addLogStyled(styleLogGreen, fmt.Sprintf("  🎰 JACKPOT! Scratch ticket pays $%d!", winnings))
				} else if winnings == 20 {
					m.addLogStyled(styleLogGreen, "  Scratch ticket: break-even. At least you didn't lose.")
					m.gs.Money += float64(winnings)
				} else {
					m.gs.Money += float64(winnings)
					m.addLogStyled(styleLogGreen, fmt.Sprintf("  Scratch ticket winner: +$%d", winnings))
				}
				m.doNextDay()
			} else {
				m.addLog(styleWarn.Render("  Can't spare $20 for a ticket."))
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
	case r < 0.001:
		return 500 // 1 in 1000 — rare
	case r < 0.005:
		return 200 // 1 in 250 — hard to find
	case r < 0.022:
		return 100 // 1 in 59 — uncommon
	case r < 0.055:
		return 50 // 1 in 30 — regular
	case r < 0.122:
		return 40 // 1 in 15 — common
	case r < 0.322:
		return 20 // 1 in 5 — break-even
	default:
		return 0 // ~68% loser
	}
}

func (m *model) doEvening() {
	m.addLog("")
	m.addLog(m.logDivider(styleLogInfo, "EVENING"))
	m.addLog("")
	m.addLog(lipgloss.NewStyle().Foreground(colorBrightWhite).Render("  End of day. What are you doing tonight?"))
	m.addLog("")
	m.addLog(lipgloss.NewStyle().Foreground(colorBrightWhite).Render(fmt.Sprintf("  [1] Allen's Coffee Brandy   $12   %s", styleDim("Cut loose with some trailer juice!"))))
	m.addLog(lipgloss.NewStyle().Foreground(colorBrightWhite).Render(fmt.Sprintf("  [2] Six-Pack of Natty       $9    %s", styleDim("A couple of natty's won't hurt ya none!"))))
	m.addLog(lipgloss.NewStyle().Foreground(colorBrightWhite).Render(fmt.Sprintf("  [3] Scratch Ticket          $20   %s", styleDim("Probably a loser. Probably."))))
	m.addLog(lipgloss.NewStyle().Foreground(colorBrightWhite).Render(fmt.Sprintf("  [4] Early night             free  %s", styleDim("Up before dawn. Full day tomorrow."))))
	m.addLog("")
	m.addLog(fmt.Sprintf("  Cash: %s", moneyStr(m.gs.Money)))
	m.syncViewport()
}

func (m *model) doNextDay() {
	m.gs.Day++
	m.weather = rollWeather()
	m.gs.HasSternman = false
	m.gs.SternmanSkilled = false
	m.gs.FlatlanderBonus = false
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
	} else if m.gs.DayLost {
		m.gs.DayLost = false
		m.phase = PhaseMorning
		m.addLog("")
		m.addLogStyled(styleLogDanger, "  Spent the night in the Knox County lockup.")
		m.addLogStyled(styleLogDanger, "  Someone bailed you out this morning. $5,000 fine. Day wasted.")
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
	m.gs.HotCrabZone = RollHotCrabZone()

	// Deferred vandalism — 30% already set the flag; now resolve it
	if m.gs.PendingVandalism {
		m.gs.PendingVandalism = false
		dmg := 8.0 + rand.Float64()*12.0 // 8–20% engine damage
		m.gs.Engine = math.Max(0, m.gs.Engine-dmg)
		flavors := []string{
			"Came down to the dock this morning. Someone got into your engine bay.",
			"Hood on the engine was ajar. Wasn't like that last night.",
			"Found a wrench on the deck that ain't yours. Check your engine.",
		}
		m.addLog("")
		m.addLogStyled(styleLogDanger, fmt.Sprintf("  ⚠ %s", flavors[rand.Intn(len(flavors))]))
		m.addLogStyled(styleLogDanger, fmt.Sprintf("  Engine took %.0f%% damage overnight. That wasn't weather.", dmg))
	}

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
	m.addLog(m.logDivider(styleLogInfo, "CHANNEL 68"))
	for _, line := range RollMorningGossip(m.gs, m.weather) {
		m.addLogStyled(styleLogDeck, fmt.Sprintf("  %s", line))
	}

	m.addLog("")
	m.addLog(m.logDivider(styleLogInfo, "MARKET"))
	p := m.gs.DailyPrices
	m.addLogStyled(styleLogInfo, fmt.Sprintf("  Chix $%.2f   Qtrs $%.2f   Halves $%.2f   Selects $%.2f   Deuces $%.2f   Jumbos $%.2f   Culls $%.2f",
		p[0], p[1], p[2], p[3], p[4], p[5], p[6]))
	m.addLog(fmt.Sprintf("  Diesel $%.2f/gal   Herring bait $%.2f/lb",
		m.gs.DieselPrice, m.gs.BaitPrice))

	// Dock gossip about crab zones
	if m.gs.HotCrabZone != "" {
		gossip := []string{
			"Someone at the co-op said Zone %s is loaded with Jonah today.",
			"Heard on the radio — Zone %s running heavy crab this morning.",
			"Guy at the fuel dock said his buddy pulled nothing but crab out of Zone %s yesterday.",
			"Word around the wharf: Zone %s is crabby as hell right now.",
			"Old timer mentioned Zone %s has been full of Jonah the last couple days.",
		}
		line := gossip[rand.Intn(len(gossip))]
		m.addLog("")
		m.addLog(styleDim(fmt.Sprintf("  "+line, m.gs.HotCrabZone)))
	}
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
			price := 0.90
			gross := result.JonahCrabLbs * price
			bycatchTotal += gross
			addQ(fmt.Sprintf("    %-14s %5.1f lbs  @ $%.2f/lb  = %s", "Jonah Crab", result.JonahCrabLbs, price, moneyStr(gross)))
		}
		if result.RockCrabLbs > 0 {
			price := 0.50
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

	// Thrown-back bycatch
	if result.ThrownCrabLbs > 0 || result.ThrownGroundfishLbs > 0 {
		addQ("")
		addQ(styleDim("  THROWN BACK (no permit)"))
		if result.ThrownCrabLbs > 0 {
			missed := result.ThrownCrabLbs * 0.75
			addQ(styleDim(fmt.Sprintf("    %-14s %5.1f lbs  ~$0.90/lb  = %s — crab permit req.", "Jonah/Rock Crab", result.ThrownCrabLbs, moneyStr(missed))))
		}
		if result.ThrownGroundfishLbs > 0 {
			price := groundfishPrice(result.ThrownGroundfishName)
			missed := result.ThrownGroundfishLbs * price
			addQ(styleDim(fmt.Sprintf("    %-14s %5.1f lbs  ~$%.2f/lb  = %s — groundfish permit req.", result.ThrownGroundfishName, result.ThrownGroundfishLbs, price, moneyStr(missed))))
		}
	}
	addQ("")

	fuelCost := float64(result.FuelUsed) * m.gs.DieselPrice
	baitCost := float64(result.BaitUsed) * m.gs.BaitPrice
	sternmanCost := 0.0
	if m.gs.HasSternman {
		if m.gs.SternmanSkilled {
			sternmanCost = 150.0
		} else {
			sternmanCost = 60.0
		}
	}
	addQS(styleLogInfo, "  EXPENSES")
	addQS(styleLogExpense, fmt.Sprintf("    Fuel   %d gal × $%.2f  -%s", result.FuelUsed, m.gs.DieselPrice, moneyStr(fuelCost)))
	addQS(styleLogExpense, fmt.Sprintf("    Bait   %d lbs × $%.2f  -%s", result.BaitUsed, m.gs.BaitPrice, moneyStr(baitCost)))
	if sternmanCost > 0 {
		label := "Greenhand"
		if m.gs.SternmanSkilled {
			label = "Exp. hand"
		}
		addQS(styleLogExpense, fmt.Sprintf("    %-9s day rate   -%s", label, moneyStr(sternmanCost)))
	}
	addQ("")

	tripNet := totalGross - fuelCost - baitCost - sternmanCost
	if tripNet >= 0 {
		addQS(styleLogGreen, fmt.Sprintf("  Trip net: %s", moneyStr(tripNet)))
	} else {
		addQS(styleLogRed, fmt.Sprintf("  Trip net: -%s (in the hole)", moneyStr(-tripNet)))
	}
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

	// Build pre-event queue
	m.pendingLines = nil
	if m.gs.HasSternman {
		if m.gs.SternmanSkilled {
			m.queueLog("0530 — Your experienced hand's already rigging gear when you get to the dock.")
		} else {
			m.queueLog("0600 — Your greenhand shows up right on time. Seems eager enough.")
		}
	}
	m.queueLog("0600 — Left the dock, steaming to grounds...")

	// Breakdown check
	breakdownChance := 0.0
	if m.gs.Engine < 15 {
		breakdownChance = 0.45
	} else if m.gs.Engine < 25 {
		breakdownChance = 0.25
	} else if m.gs.Engine < 40 {
		breakdownChance = 0.08
	}
	if breakdownChance > 0 && rand.Float64() < breakdownChance {
		m.queueLogStyled(styleLogDanger, "0640 — Engine quit. Dead in the water.")
		if m.gs.HasVHF && rand.Float64() < 0.50 {
			m.queueLogStyled(styleLogGreen, "0645 — Got on channel 16. Dirty Ernie heard you — he'll tow you in for a six-pack.")
			m.queueLog("1100 — Back at the dock. No catch today.")
			m.gs.Money -= 9
		} else {
			if m.gs.HasVHF {
				m.queueLogStyled(styleLogWarn, "0645 — Put out a call on channel 16. No answer. Sea Tow's on the way.")
			} else {
				m.queueLogStyled(styleLogWarn, "0645 — No radio. Cell signal's weak out here. Finally got through to Sea Tow.")
			}
			m.queueLogStyled(styleLogExpense, "1030 — Sea Tow dragged you in. $300.")
			m.gs.Money -= 300
		}
		// Zero out the haul
		m.gs.Freezer -= result.CatchLbs
		m.gs.TotalCatch -= result.CatchLbs
		result.CatchLbs = 0
		result.Revenue = 0
		result.Grades = nil
		result.JonahCrabLbs = 0
		result.RockCrabLbs = 0
		result.GroundfishLbs = 0
		m.haul = &result
		m.queueLog("1200 — Engine in the shop. She'll need work before tomorrow.")
		m.activeEvent = nil
		return
	}

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
		m.queueLog(fmt.Sprintf("1050 — Jonah crabs loaded in the traps. Keeping %.0f lbs.", result.JonahCrabLbs))
		if result.CrabCrowded {
			m.queueLogStyled(styleLogWarn, "       Traps were packed. Lobster catch took a hit. Bait burning fast.")
		}
	} else if result.RockCrabLbs > 0 {
		m.queueLog(fmt.Sprintf("1050 — Rock crabs in the traps. Keeping %.0f lbs.", result.RockCrabLbs))
		if result.CrabCrowded {
			m.queueLogStyled(styleLogWarn, "       Crabs crowded out the lobster. Tore through the bait too.")
		}
	} else if !m.gs.HasCrabPermit && rand.Float64() < 0.40 {
		m.queueLog("1050 — Crabs loaded in traps. No permit — back they go.")
		if result.CrabCrowded {
			m.queueLogStyled(styleLogWarn, "       Traps full of crab. Light on lobster, bait gone.")
		}
	}
	if result.GroundfishLbs > 0 {
		m.queueLogStyled(styleLogGreen, fmt.Sprintf("1055 — %s in the trap! %.0f lbs. Keeping it.", result.GroundfishName, result.GroundfishLbs))
	} else if !m.gs.HasGroundfishPermit && rand.Float64() < 0.12 {
		fish := []string{"monkfish", "cusk", "halibut"}[rand.Intn(3)]
		m.queueLogStyled(styleLogWarn, fmt.Sprintf("1055 — Pulled a %s. No groundfish permit — back it goes.", fish))
	}
	if result.SternmanMishap {
		mishaps := []string{
			"0915 — Your greenhand dropped a full crate over the side.",
			"1005 — Greenhand tangled the banding. Lost time sorting it out.",
			"1020 — Kid grabbed the wrong buoy line. Pulled someone else's gear halfway up.",
		}
		m.queueLogStyled(styleLogWarn, fmt.Sprintf("%s %.0f lbs back in the water.", mishaps[rand.Intn(len(mishaps))], result.SternmanMishapLbs))
	}
	if result.TrapsLost > 0 {
		trapCost := float64(result.TrapsLost) * BoatModels[m.gs.BoatName].TrapCost
		if result.TrapsLost == 1 {
			m.queueLogStyled(styleLogWarn, fmt.Sprintf("1050 — Lost a trap out there. Gone. ($%.0f replacement)", trapCost))
		} else {
			m.queueLogStyled(styleLogDanger, fmt.Sprintf("1050 — Lost %d traps. Lines cut or swept. ($%.0f to replace)", result.TrapsLost, trapCost))
		}
	}

	// Pot thief — zone-based chance; crowded nearshore zones get higher odds
	// Zone A/B: 6%; C/D: 3%; E+: 1%
	if m.gs.Traps > 1 {
		potThiefChance := 0.01
		switch zone.ID {
		case "A", "B":
			potThiefChance = 0.06
		case "C", "D":
			potThiefChance = 0.03
		}
		if rand.Float64() < potThiefChance {
			m.gs.Traps--
			trapCost := BoatModels[m.gs.BoatName].TrapCost
			flavors := []string{
				"Buoy's gone too. Clipped clean.",
				"Warp cut right at the eye. That wasn't an accident.",
				"Asked around the dock. Nobody saw nothin'.",
				"Probably that guy with the green truck. Can't prove it.",
			}
			flavor := flavors[rand.Intn(len(flavors))]
			m.queueLogStyled(styleLogDanger, "1100 — Some friggin dubbah must of stole your pot!")
			m.queueLogStyled(styleLogDanger, fmt.Sprintf("       %s ($%.0f to replace)", flavor, trapCost))
		}
	}

	// Build post-event queue
	m.postEventLines = nil

	midday := func(queue *[]string) {
		addTo := func(s string) {
			if !strings.Contains(s, "\x1b[") && s != "" {
				s = styleDefault.Render(s)
			}
			*queue = append(*queue, s)
		}
		addToStyled := func(style lipgloss.Style, s string) {
			addTo(style.Render(s))
		}
		if result.CatchLbs > 200 {
			addToStyled(styleGood, fmt.Sprintf("1100 — Killing it out here. %.0f lbs and counting.", result.CatchLbs*0.6))
		} else if result.CatchLbs > 80 {
			addTo(fmt.Sprintf("1100 — Steady haul. %.0f lbs so far.", result.CatchLbs*0.6))
		} else {
			addToStyled(styleLogWarn, "1100 — Slim pickings. Traps running light.")
		}
	}

	if m.activeEvent == nil {
		midday(&m.pendingLines)
		m.queueLog(fmt.Sprintf("1430 — Last trap aboard. %.0f lbs total.", result.CatchLbs))
		m.queueLog("1600 — Back at the dock.")
		m.buildDebriefLines(result, &m.pendingLines)
	} else {
		m.queueLogStyled(styleLogWarn, m.activeEvent.Desc)
		postAdd := func(s string) {
			if !strings.Contains(s, "\x1b[") && s != "" {
				s = styleDefault.Render(s)
			}
			m.postEventLines = append(m.postEventLines, s)
		}
		postAddStyled := func(style lipgloss.Style, s string) { postAdd(style.Render(s)) }
		_ = postAddStyled
		midday(&m.postEventLines)
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
	if key != ev.KeyA && key != ev.KeyB {
		return
	}
	result := m.eventHaulResult

	m.addLog("")
	switch ev.Type {
	case EventBerriedHen:
		if key == ev.KeyA {
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
		if key == ev.KeyA {
			m.gs.Money += 2500
			m.addLogStyled(styleLogGreen, "  $2,500 in the bilge. You didn't see anything.")
			if rand.Float64() < 0.15 {
				fine := 5000.0
				m.gs.Money -= fine
				m.addLogStyled(styleLogDanger, "  Coast Guard was waiting at the dock. $5,000 fine and a night in Knox County.")
				m.gs.DayLost = true
			}
		} else {
			m.addLog("  CG thanks you on the radio. You feel okay about it.")
			if rand.Float64() < 0.40 {
				m.addLogStyled(styleLogGreen, "  Co-op comp'd your fuel today. Good karma.")
				m.gs.Money += float64(result.FuelUsed) * m.gs.DieselPrice
			}
		}

	case EventJonahCrabs:
		if key == ev.KeyA {
			bonus := 40.0 + rand.Float64()*50.0
			m.gs.Money += bonus
			m.addLogStyled(styleLogGreen, fmt.Sprintf("  Jonah crabs brought in %s at the co-op.", moneyStr(bonus)))
		} else {
			m.addLog("  Tossed 'em back. Traps run cleaner without the crap.")
		}

	case EventGhostTrap:
		if key == ev.KeyA {
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
		if key == ev.KeyA {
			m.addLog("  You push through. Storm hits hard on the run home.")
			dmg := 5.0 + rand.Float64()*10.0
			m.gs.Engine = math.Max(0, m.gs.Engine-dmg)
			m.gs.Hydraulics = math.Max(0, m.gs.Hydraulics-dmg*0.5)
			m.addLogStyled(styleLogWarn, fmt.Sprintf("  Took a beating: engine -%.0f%%, hydraulics -%.0f%%", dmg, dmg*0.5))
		} else {
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
		if key == ev.KeyA {
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
				m.gs.DaysWithBooze = 0
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

	case EventNeighborTrap:
		if key == ev.KeyA {
			bonus := 5.0 + rand.Float64()*12.0
			m.gs.Freezer += bonus
			m.gs.TotalCatch += bonus
			m.addLogStyled(styleLogGreen, fmt.Sprintf("  Hauled it. %.0f lbs of lobster. Tossed the trap back over the side.", bonus))
			if rand.Float64() < 0.30 {
				m.gs.PendingVandalism = true
				m.addLog(styleDim("  Someone was watching from the ridge. Could be nothing."))
			}
		} else {
			m.addLog("  You untangle the warp and drop it back. Not your gear, not your problem.")
		}

	case EventHotSet:
		if key == ev.KeyA {
			bonus := 18.0 + rand.Float64()*22.0
			m.gs.Freezer += bonus
			m.gs.TotalCatch += bonus
			m.addLogStyled(styleLogGreen, fmt.Sprintf("  Pulled every last one. %.0f extra lbs. Good set.", bonus))
		} else {
			bonus := 10.0 + rand.Float64()*10.0
			m.gs.Freezer += bonus
			m.gs.TotalCatch += bonus
			m.addLogStyled(styleLogGreen, fmt.Sprintf("  Sorted careful. %.0f lbs of keepers, clean and legal.", bonus))
		}

	case EventFlatlander:
		if key == ev.KeyA {
			// Mark a price multiplier for today's sell — store on GameState
			m.gs.FlatlanderBonus = true
			m.addLogStyled(styleLogGreen, "  You radio back. They want everything you've got. 20% over market today.")
		} else {
			m.addLog(styleDim("  You keep hauling. Their problem, not yours."))
		}

	case EventOldTimer:
		if key == ev.KeyA {
			bonus := m.gs.Freezer * 0.10
			if bonus < 5 { bonus = 5 }
			m.gs.Freezer += bonus
			m.gs.TotalCatch += bonus
			m.addLogStyled(styleLogGreen, fmt.Sprintf("  You work the east side. Old Donnie was right. %.0f extra lbs.", bonus))
		} else {
			m.addLog(styleDim("  You stick to your spots. Donnie's probably half-asleep anyway."))
		}

	case EventSunkTrap:
		if key == ev.KeyA {
			recovered := 1 + rand.Intn(3)
			bonus := 8.0 + rand.Float64()*15.0
			m.gs.Traps += recovered
			boat := BoatModels[m.gs.BoatName]
			if m.gs.Traps > boat.MaxTraps {
				m.gs.Traps = boat.MaxTraps
			}
			m.gs.Freezer += bonus
			m.gs.TotalCatch += bonus
			m.addLogStyled(styleLogGreen, fmt.Sprintf("  Grappled up %d traps, still serviceable. %.0f lbs of lobster inside. Good find.", recovered, bonus))
		} else {
			m.addLog(styleDim("  You leave it. Not your gear, not your problem."))
		}

	case EventGrayMarketHalibut:
		if key == ev.KeyA {
			cashBonus := 130.0 + rand.Float64()*40.0
			m.gs.Money += cashBonus
			m.addLogStyled(styleLogGreen, fmt.Sprintf("  You wrap it in burlap and slide it under the console. $%.0f cash when you get back.", cashBonus))
			if rand.Float64() < 0.08 {
				m.addLogStyled(styleLogDanger, "  Coast Guard was at the dock when you came in. They saw the fish.")
				fine := 400.0 + rand.Float64()*200.0
				m.gs.Money -= fine
				m.addLogStyled(styleLogDanger, fmt.Sprintf("  $%.0f fine. Not worth it.", fine))
			}
		} else {
			m.addLog(styleDim("  You slide it back over the rail. Regulations are regulations."))
		}

	case EventSealRaid:
		if key == ev.KeyA {
			if rand.Float64() < 0.50 {
				lost := 3.0 + rand.Float64()*6.0
				m.gs.Freezer = math.Max(0, m.gs.Freezer-lost)
				m.addLogStyled(styleLogWarn, fmt.Sprintf("  Seal backed off eventually. Lost maybe %.0f lbs before it went.", lost))
			} else {
				m.addLogStyled(styleLogGreen, "  Scared it off. Gear looks clean. Lucky.")
			}
		} else {
			lost := 8.0 + rand.Float64()*12.0
			m.gs.Freezer = math.Max(0, m.gs.Freezer-lost)
			m.addLogStyled(styleLogWarn, fmt.Sprintf("  Seal worked the whole string. Picked off %.0f lbs before you finished.", lost))
		}

	case EventCGCheck:
		if key == ev.KeyA || key == ev.KeyB {
			hasMissingPermit := false
			var issues []string
			// Check if they're in crab territory without permit
			if !m.gs.HasCrabPermit && m.gs.HotCrabZone != "" {
				hasMissingPermit = true
				issues = append(issues, "no crab permit")
			}
			if hasMissingPermit {
				fine := 300.0 + rand.Float64()*200.0
				m.gs.Money -= fine
				m.addLogStyled(styleLogDanger, fmt.Sprintf("  Boarding officer found issues: %s. $%.0f fine.", strings.Join(issues, ", "), fine))
			} else {
				m.addLogStyled(styleLogGreen, "  Papers in order. They wave you off. Back to hauling.")
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
		revenue := 0.0
		m.addLog("")
		m.addLog(m.logDivider(styleTitle, "CO-OP SALE"))
		if m.gs.FlatlanderBonus {
			m.addLogStyled(styleLogGreen, "  ★ Flatlander wedding premium — 20% over market today")
		}
		if m.haul != nil && len(m.haul.Grades) > 0 {
			for _, g := range m.haul.Grades {
				price := g.Price
				if m.gs.FlatlanderBonus {
					price *= 1.20
				}
				m.addLog(fmt.Sprintf("  %-10s %.1f lbs @ $%.2f/lb", g.Name, g.Lbs, price))
				revenue += g.Lbs * price
			}
		} else {
			flatPrice := 5.75
			if m.gs.FlatlanderBonus {
				flatPrice *= 1.20
			}
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
		// CREW
		{"Hire Greenhand", 60, func() {
			if m.gs.HasSternman {
				m.confirmBuy = "Already have crew for today."
				return
			}
			if m.gs.Money < 60 {
				m.confirmBuy = fmt.Sprintf("Need $60 — short by %s", moneyStr(60-m.gs.Money))
				return
			}
			m.gs.Money -= 60
			m.gs.HasSternman = true
			m.gs.SternmanSkilled = false
			m.confirmBuy = "Greenhand hired. He's waiting at the dock."
		}},
		{"Hire Experienced Hand", 150, func() {
			if m.gs.HasSternman {
				m.confirmBuy = "Already have crew for today."
				return
			}
			if m.gs.Money < 150 {
				m.confirmBuy = fmt.Sprintf("Need $150 — short by %s", moneyStr(150-m.gs.Money))
				return
			}
			m.gs.Money -= 150
			m.gs.HasSternman = true
			m.gs.SternmanSkilled = true
			m.confirmBuy = "Experienced hand hired. He knows what he's doing."
		}},
		// SUPPLIES
		{"Bait (50 lbs)", 30.00, func() {
			cap := BoatModels[m.gs.BoatName].BaitCap
			if m.gs.Bait >= cap {
				m.confirmBuy = "Bait storage full!"
				return
			}
			add := min(50, cap-m.gs.Bait)
			m.gs.Bait += add
			m.confirmBuy = fmt.Sprintf("Loaded %d lbs herring", add)
		}},
		{"Bait (200 lbs)", 110.00, func() {
			cap := BoatModels[m.gs.BoatName].BaitCap
			if m.gs.Bait >= cap {
				m.confirmBuy = "Bait storage full!"
				return
			}
			add := min(200, cap-m.gs.Bait)
			cost := float64(add) * m.gs.BaitPrice
			if m.gs.Money < cost {
				m.confirmBuy = fmt.Sprintf("Need %s", moneyStr(cost))
				return
			}
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
		// Equipment (indices 7-12)
		{"Radar", 2500, func() {
			if m.gs.HasRadar {
				m.confirmBuy = "Already installed."
				return
			}
			if m.gs.Money < 1200 {
				m.confirmBuy = fmt.Sprintf("Need %s — short by %s", moneyStr(1200), moneyStr(1200-m.gs.Money))
				return
			}
			m.gs.Money -= 1200
			m.gs.HasRadar = true
			m.confirmBuy = "Radar installed. Fish in fog."
		}},
		{"GPS/Chartplotter", 1500, func() {
			if m.gs.HasGPS {
				m.confirmBuy = "Already installed."
				return
			}
			if m.gs.Money < 800 {
				m.confirmBuy = fmt.Sprintf("Need %s — short by %s", moneyStr(800), moneyStr(800-m.gs.Money))
				return
			}
			m.gs.Money -= 800
			m.gs.HasGPS = true
			m.confirmBuy = "GPS installed. Zones F and G unlocked."
		}},
		{"VHF Radio", 500, func() {
			if m.gs.HasVHF {
				m.confirmBuy = "Already installed."
				return
			}
			if m.gs.Money < 250 {
				m.confirmBuy = fmt.Sprintf("Need %s — short by %s", moneyStr(250), moneyStr(250-m.gs.Money))
				return
			}
			m.gs.Money -= 250
			m.gs.HasVHF = true
			m.confirmBuy = "VHF installed. You can hear channel 16 now."
		}},
		{"Upgraded Hauler", 2000, func() {
			if m.gs.HasUpgHauler {
				m.confirmBuy = "Already installed."
				return
			}
			if m.gs.Money < 600 {
				m.confirmBuy = fmt.Sprintf("Need %s — short by %s", moneyStr(600), moneyStr(600-m.gs.Money))
				return
			}
			m.gs.Money -= 600
			m.gs.HasUpgHauler = true
			m.confirmBuy = "Hauler upgraded. Hydraulics will thank you."
		}},
		{"Depth Sounder", 2500, func() {
			if m.gs.HasDepthSound {
				m.confirmBuy = "Already installed."
				return
			}
			if m.gs.Money < 400 {
				m.confirmBuy = fmt.Sprintf("Need %s — short by %s", moneyStr(400), moneyStr(400-m.gs.Money))
				return
			}
			m.gs.Money -= 400
			m.gs.HasDepthSound = true
			m.confirmBuy = "Depth sounder installed. You can read the bottom now."
		}},
		{"Exhaust Heat Exchanger", 5000, func() {
			if m.gs.HasExhaustHX {
				m.confirmBuy = "Already installed."
				return
			}
			if m.gs.Money < 5000 {
				m.confirmBuy = fmt.Sprintf("Need %s — short by %s", moneyStr(5000), moneyStr(5000-m.gs.Money))
				return
			}
			m.gs.Money -= 5000
			m.gs.HasExhaustHX = true
			m.confirmBuy = "Heat exchanger installed. Engine'll run cooler and last longer."
		}},
		{"Deck Lights", 1500, func() {
			if m.gs.HasDeckLights {
				m.confirmBuy = "Already installed."
				return
			}
			if m.gs.Money < 1500 {
				m.confirmBuy = fmt.Sprintf("Need %s — short by %s", moneyStr(1500), moneyStr(1500-m.gs.Money))
				return
			}
			m.gs.Money -= 1500
			m.gs.HasDeckLights = true
			m.confirmBuy = "Spreader lights installed. You're leaving the dock at 0500 from now on."
		}},
		{"Crab Permit", 1500, func() {
			if m.gs.HasCrabPermit {
				m.confirmBuy = "Already licensed."
				return
			}
			if m.gs.Money < 1500 {
				m.confirmBuy = fmt.Sprintf("Need $1,500 — short by %s", moneyStr(1500-m.gs.Money))
				return
			}
			m.gs.Money -= 1500
			m.gs.HasCrabPermit = true
			m.confirmBuy = "Crab permit issued. Jonah and rock crabs are yours to keep."
		}},
		{"Groundfish Permit", 1500, func() {
			boat := BoatModels[m.gs.BoatName]
			if m.gs.HasGroundfishPermit {
				m.confirmBuy = "Already licensed."
				return
			}
			if boat.Length < 34 {
				m.confirmBuy = "NOAA requires a vessel 34 ft or larger for a groundfish permit."
				return
			}
			if m.gs.Money < 1500 {
				m.confirmBuy = fmt.Sprintf("Need $1,500 — short by %s", moneyStr(1500-m.gs.Money))
				return
			}
			m.gs.Money -= 1500
			m.gs.HasGroundfishPermit = true
			m.confirmBuy = "Groundfish permit issued. Monkfish, cusk, and halibut are yours to keep."
		}},
	}

	// Boat upgrades — append dynamically based on fleet order
	fleetOrder := []string{"Crowley Beal 28", "Calvin Beal 34", "Duffy 35", "Young Bros 40", "Wesmac 46"}
	for _, name := range fleetOrder {
		name := name // capture
		bm := BoatModels[name]
		items = append(items, struct {
			name  string
			cost  float64
			apply func()
		}{name, 0, func() {
			current := BoatModels[m.gs.BoatName]
			if bm.Length <= current.Length {
				m.confirmBuy = fmt.Sprintf("You're already on a %s or better.", name)
				return
			}
			if m.gs.Money < float64(bm.Cost) {
				m.confirmBuy = fmt.Sprintf("Need %s — short by %s", moneyStr(float64(bm.Cost)), moneyStr(float64(bm.Cost)-m.gs.Money))
				return
			}
			m.gs.Money -= float64(bm.Cost)
			m.gs.BoatName = name

			// New vessel — reset equipment, start with 50% trap cap, fresh health
			m.gs.Traps = bm.MaxTraps / 2
			m.gs.Fuel = min(m.gs.Fuel, bm.FuelCap)
			m.gs.Bait = min(m.gs.Bait, bm.BaitCap)
			m.gs.Engine = 100.0
			m.gs.Zincs = 100.0
			m.gs.Hydraulics = 100.0
			m.gs.HasRadar = false
			m.gs.HasGPS = false
			m.gs.HasVHF = false
			m.gs.HasUpgHauler = false
			m.gs.HasDepthSound = false
			m.gs.HasExhaustHX = false
			m.gs.HasDeckLights = false
			m.confirmBuy = fmt.Sprintf("She's yours. Welcome aboard the %s. Gear up at the Wharf.", name)
		}})
	}

	if m.marketCursor >= len(items) {
		return
	}

	item := items[m.marketCursor]
	item.apply()
	saveGame(m.gs)
}
