package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─── View ─────────────────────────────────────────────────────────────────────

func (m model) View() string {
	if m.width == 0 {
		return "Loading HIGHLINER..."
	}

	var b strings.Builder

	// Full-width header
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
		{"[5/G] GROUNDS", ScreenChart},
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

	// Persistent stats bar
	boat := BoatModels[m.gs.BoatName]
	bgStyle := lipgloss.NewStyle().Background(lipgloss.Color("235"))
	statLabel := bgStyle.Foreground(colorBrightWhite).Render
	statVal := func(val, warn, danger string, isWarn, isDanger bool) string {
		color := colorBrightWhite
		if isDanger {
			color = lipgloss.Color("#FF3333")
		} else if isWarn {
			color = lipgloss.Color("#FF8C00")
		}
		return bgStyle.Foreground(color).Bold(isDanger || isWarn).Render(val) +
			bgStyle.Foreground(colorBrightWhite).Render(warn+danger)
	}

	fuelPct := float64(m.gs.Fuel) / float64(boat.FuelCap)
	baitPct := float64(m.gs.Bait) / float64(boat.BaitCap)
	trapPct := float64(m.gs.Traps) / float64(boat.MaxTraps)

	fuelStr := statVal(
		fmt.Sprintf("%d/%d gal", m.gs.Fuel, boat.FuelCap), "", "",
		fuelPct < 0.30, fuelPct < 0.15)
	baitStr := statVal(
		fmt.Sprintf("%d/%d lbs", m.gs.Bait, boat.BaitCap), "", "",
		baitPct < 0.30, m.gs.Bait == 0)
	trapStr := statVal(
		fmt.Sprintf("%d/%d", m.gs.Traps, boat.MaxTraps), "", "",
		trapPct < 0.40, trapPct < 0.20)

	statsBar := lipgloss.JoinHorizontal(lipgloss.Top,
		bgStyle.Render("  "),
		statLabel("Cash: "), bgStyle.Foreground(colorBrightWhite).Render(moneyStr(m.gs.Money)),
		statLabel("   Fuel: "), fuelStr,
		statLabel("   Bait: "), baitStr,
		statLabel("   Traps: "), trapStr,
		bgStyle.Foreground(colorBrightWhite).Render("  "),
	)
	b.WriteString(lipgloss.NewStyle().Background(lipgloss.Color("235")).Width(m.width).Render(statsBar))
	b.WriteString("\n")

	// Full-width cyan separator
	b.WriteString(lipgloss.NewStyle().Foreground(colorCyan).Render(strings.Repeat("─", m.width)))
	b.WriteString("\n")

	// Main content area
	switch m.screen {
	case ScreenLog:
		b.WriteString(m.viewport.View())
	case ScreenMaintenance, ScreenDock, ScreenMarket, ScreenChart:
		b.WriteString(m.altVP.View())
	}

	b.WriteString("\n")

	// Status bar
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
			if needed <= 0 {
				return "full"
			}
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

	// Repairs section
	engineCost := (100 - m.gs.Engine) * 12.0
	zincsCost := (100 - m.gs.Zincs) * 3.0
	hydCost := (100 - m.gs.Hydraulics) * 8.0
	repairItems := []wharfItem{
		{"Repair Engine", func() string {
			if m.gs.Engine >= 100 {
				return "good"
			}
			return fmt.Sprintf("$%.0f", engineCost)
		}(), fmt.Sprintf("%.0f%% → 100%%  |  %s", m.gs.Engine, healthStr(m.gs.Engine))},
		{"Replace Zincs", func() string {
			if m.gs.Zincs >= 100 {
				return "good"
			}
			return fmt.Sprintf("$%.0f", zincsCost)
		}(), fmt.Sprintf("%.0f%% → 100%%  |  %s", m.gs.Zincs, healthStr(m.gs.Zincs))},
		{"Repair Hydraulics", func() string {
			if m.gs.Hydraulics >= 100 {
				return "good"
			}
			return fmt.Sprintf("$%.0f", hydCost)
		}(), fmt.Sprintf("%.0f%% → 100%%  |  %s", m.gs.Hydraulics, healthStr(m.gs.Hydraulics))},
	}

	// Equipment items
	equipItems := []wharfItem{
		{"Radar", func() string {
			if m.gs.HasRadar {
				return "owned"
			}
			return "$2,500"
		}(), func() string {
			if m.gs.HasRadar {
				return "✓ fish all zones in fog"
			}
			return "fish all zones in fog"
		}()},
		{"GPS/Chartplotter", func() string {
			if m.gs.HasGPS {
				return "owned"
			}
			return "$1,500"
		}(), func() string {
			if m.gs.HasGPS {
				return "✓ unlocks zones F and G"
			}
			return "unlocks zones F and G"
		}()},
		{"VHF Radio", func() string {
			if m.gs.HasVHF {
				return "owned"
			}
			return "$500"
		}(), func() string {
			if m.gs.HasVHF {
				return "✓ distress events + free tow chance"
			}
			return "distress events + free tow from Dirty Ernie"
		}()},
		{"Upgraded Hauler", func() string {
			if m.gs.HasUpgHauler {
				return "owned"
			}
			return "$2,000"
		}(), func() string {
			if m.gs.HasUpgHauler {
				return "✓ slower hydraulic wear"
			}
			return "slower hydraulic wear"
		}()},
		{"Depth Sounder", func() string {
			if m.gs.HasDepthSound {
				return "owned"
			}
			return "$2,500"
		}(), func() string {
			if m.gs.HasDepthSound {
				return "✓ full efficiency in deep zones"
			}
			return "full efficiency in deep zones (D-G)"
		}()},
		{"Exhaust Heat Exchanger", func() string {
			if m.gs.HasExhaustHX {
				return "owned"
			}
			return "$5,000"
		}(), func() string {
			if m.gs.HasExhaustHX {
				return "✓ engine runs cooler, less wear"
			}
			return "reduces engine wear per haul"
		}()},
	}

	permitItems := []wharfItem{
		{"Crab Permit", func() string {
			if m.gs.HasCrabPermit {
				return "licensed"
			}
			return "$1,500"
		}(), func() string {
			if m.gs.HasCrabPermit {
				return "✓ keep Jonah + rock crab"
			}
			return "keep and sell Jonah + rock crab"
		}()},
		{"Groundfish Permit", func() string {
			if m.gs.HasGroundfishPermit {
				return "licensed"
			}
			boat := BoatModels[m.gs.BoatName]
			if boat.Length < 34 {
				return styleDanger.Render("34ft+ only")
			}
			return "$1,500"
		}(), func() string {
			if m.gs.HasGroundfishPermit {
				return "✓ keep monkfish, cusk, halibut"
			}
			boat := BoatModels[m.gs.BoatName]
			if boat.Length < 34 {
				return styleWarn.Render("upgrade vessel to 34ft+ to unlock")
			}
			return "keep and sell monkfish, cusk, halibut"
		}()},
	}

	allItems := append(supplyItems, repairItems...)
	allItems = append(allItems, equipItems...)
	allItems = append(allItems, permitItems...)

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
	b.WriteString(subHeader("PERMITS", m.width))
	renderItems(permitItems, len(supplyItems)+len(repairItems)+len(equipItems))

	b.WriteString("\n")
	b.WriteString(styleKey.Render("  [↑↓/JK] Navigate   [ENTER] Buy   [1-4] Switch tabs"))

	b.WriteString("\n\n")
	b.WriteString(subHeader("BOAT UPGRADES", m.width))
	fleetOrder := []string{"Crowley Beal 28", "Calvin Beal 34", "Duffy 35", "Young Bros 40", "Wesmac 46"}
	// boat upgrade items start at index 13 (after 4 supply, 3 repair, 6 equip, 2 permit)
	boatBaseIdx := len(supplyItems) + len(repairItems) + len(equipItems) + len(permitItems)
	currentBoat := BoatModels[m.gs.BoatName]
	for i, name := range fleetOrder {
		bm := BoatModels[name]
		idx := boatBaseIdx + i
		cursor := "  "
		if m.marketCursor == idx {
			cursor = styleSelected.Render("►")
			cursor += " "
		} else {
			cursor = "  "
		}

		var costStr, descStr string
		if bm.Length <= currentBoat.Length {
			costStr = styleDim("owned")
			descStr = styleDim("current vessel or smaller")
		} else if m.gs.Money < float64(bm.Cost) {
			costStr = styleDanger.Render(moneyStr(float64(bm.Cost)))
			descStr = styleDim(fmt.Sprintf("need %s more", moneyStr(float64(bm.Cost)-m.gs.Money)))
		} else {
			costStr = styleGood.Render(moneyStr(float64(bm.Cost)))
			descStr = styleDim(fmt.Sprintf("%d traps  %d ft", bm.MaxTraps, bm.Length))
		}

		b.WriteString(fmt.Sprintf("%s%-22s  %-12s  %s\n",
			cursor,
			name,
			costStr,
			descStr))
	}

	return b.String()
}

// Keep old names as thin wrappers so any future callers don't break
func (m model) viewMaintenance() string { return m.viewMaintenanceContent() }
func (m model) viewDock() string        { return m.viewDockContent() }
func (m model) viewMarket() string      { return m.viewMarketContent() }

func (m model) viewChartContent() string {
	var b strings.Builder
	boat := BoatModels[m.gs.BoatName]

	b.WriteString(subHeader("FISHING GROUNDS — EASTERN MAINE COASTAL WATERS", m.width))
	b.WriteString("\n")

	// ASCII depth/distance map — plain text with post-styled zone labels
	b.WriteString("  " + styleDim("◄── SHORE") + strings.Repeat(" ", 48) + styleDim("DEEP SEA ──►") + "\n\n")

	b.WriteString("  ")
	spacing := []int{0, 8, 8, 8, 8, 8, 8}
	for i, z := range Zones {
		if i > 0 {
			b.WriteString(strings.Repeat("~", spacing[i]-2))
		}
		var zs lipgloss.Style
		switch {
		case m.zoneBlocked(i):
			zs = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))
		case m.gs.HotCrabZone == z.ID:
			zs = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF8C00")).Bold(true)
		default:
			zs = lipgloss.NewStyle().Foreground(colorBrightWhite).Bold(true)
		}
		b.WriteString(zs.Render("[" + z.ID + "]"))
	}
	b.WriteString("\n  ")
	for _, d := range []string{"─────", "──────", "──────", "───────", "────────", "──────────", "────────────"} {
		b.WriteString(d)
	}
	b.WriteString("\n")
	b.WriteString(styleDim("  Sandy   Mud     Rocky   Shoals  Ledge   Mixed   Deep") + "\n\n")

	if m.gs.HotCrabZone != "" {
		b.WriteString("  " + lipgloss.NewStyle().Foreground(lipgloss.Color("#FF8C00")).Render(
			fmt.Sprintf("🦀 Zone %s running heavy crab today", m.gs.HotCrabZone)) + "\n\n")
	}

	// Stats table — all plain text, color applied to whole row after padding
	b.WriteString(subHeader("ZONE DETAILS", m.width))
	b.WriteString("\n")

	// column widths (plain chars): Z=1 Name=22 Steam=5 Fuel=7 Lobster=7 Crab=6 Fish=6 Notes
	hdr := fmt.Sprintf("  %-1s  %-22s  %-5s  %-7s  %-6s  %-10s  %-6s  %-6s  %-8s  %s",
		"Z", "Name", "Steam", "Fuel", "Lobster", "Crab", "Cusk", "Monk", "Halibut", "Notes")
	b.WriteString(styleLogInfo.Render(hdr) + "\n")
	b.WriteString("  " + styleDim(strings.Repeat("─", len(hdr)-2)) + "\n")

	for i, z := range Zones {
		fuelBurn := z.SteamHours * boat.FuelBurnRate

		// Catchable % estimates per zone
		mult := z.Multiplier * 100
		lobsterStr := "poor"
		switch {
		case mult >= 140:
			lobsterStr = "rich"
		case mult >= 120:
			lobsterStr = "great"
		case mult >= 100:
			lobsterStr = "good"
		case mult >= 85:
			lobsterStr = "fair"
		}

		crabFactor := 0.5 + (z.SteamHours/10.0)*0.8
		if m.gs.HotCrabZone == z.ID {
			crabFactor *= 1.6
		}
		jonahChance := math.Min(0.85, 0.30*crabFactor)
		rockChance  := math.Min(0.65, 0.18*crabFactor)
		crabPct     := int((jonahChance + rockChance*0.5) * 100) // weighted by frequency

		// Per-species groundfish odds — same formula as simulateHaul
		cuskStr, monkStr, haliStr := "-", "-", "-"
		if z.SteamHours >= 2.0 {
			sh := z.SteamHours
			cuskChance := math.Min(0.35, 0.06*sh)
			monkChance := math.Min(0.40, 0.04*sh)
			haliChance := math.Max(0, (sh-5.0)*0.025)
			cuskStr = fmt.Sprintf("~%.0f%%", cuskChance*100)
			monkStr = fmt.Sprintf("~%.0f%%", monkChance*100)
			if haliChance > 0 {
				haliStr = fmt.Sprintf("~%.0f%%", haliChance*100)
			}
		}

		crabStr := fmt.Sprintf("~%d%%", crabPct)
		if m.gs.HotCrabZone == z.ID {
			crabStr = fmt.Sprintf("~%d%% HOT", crabPct)
		}

		// Access notes (plain)
		notes := ""
		if m.zoneBlocked(i) {
			// Shorten block reasons to fit 120-char terminal
			reason := m.zoneBlockReason(i)
			if len(reason) > 22 {
				reason = reason[:22]
			}
			notes = reason
		} else if z.ID == "F" || z.ID == "G" {
			notes = "far offshore"
		} else if z.ID == "E" {
			notes = "long steam"
		}

		// Build the plain row, then color the whole thing
		plain := fmt.Sprintf("  %-1s  %-22s  %3.1fh   %4.1fgl  %-6s  %-10s  %-6s  %-6s  %-8s  %s",
			z.ID, z.Name, z.SteamHours, fuelBurn, lobsterStr, crabStr, cuskStr, monkStr, haliStr, notes)

		var rowColor lipgloss.Color
		switch {
		case m.zoneBlocked(i):
			rowColor = lipgloss.Color("#555555")
		case m.gs.HotCrabZone == z.ID:
			rowColor = lipgloss.Color("#FF8C00")
		default:
			rowColor = colorBrightWhite
		}
		b.WriteString(lipgloss.NewStyle().Foreground(rowColor).Render(plain) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(styleDim(fmt.Sprintf("  Vessel: %s  %.1f gal/hr  |  Tank: %d/%d gal  |  Range: %.1f hrs\n",
		m.gs.BoatName, boat.FuelBurnRate, m.gs.Fuel, boat.FuelCap, float64(m.gs.Fuel)/boat.FuelBurnRate)))
	b.WriteString(styleDim("  Crab/Fish % = chance per haul (requires permit to keep)") + "\n")

	return b.String()
}

// syncViewport updates the main log viewport
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
	case ScreenChart:
		content = m.viewChartContent()
	}
	m.altVP.SetContent(content)
}

// zoneBlockReason returns a human-readable reason why a zone is blocked, or ""
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

// trapIncrement returns (how many traps to add, cost per trap) based on boat capacity
func trapIncrement(maxTraps int) (int, float64) {
	switch {
	case maxTraps <= 40: // Eastern 22
		return 5, 175.0
	case maxTraps <= 400: // Calvin Beal, Duffy
		return 25, 165.0
	default: // Young Bros, Wesmac
		return 50, 150.0
	}
}
