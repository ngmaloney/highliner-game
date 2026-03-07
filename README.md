# HIGHLINER 🦞

A passive Maine lobstering strategy simulator — DOS-aesthetic TUI game built with Go + Bubble Tea.

```
╔═══════════════════════════════╗
║  HIGHLINER  ║
╚═══════════════════════════════╝
```

## Run It

```bash
cd /home/ngmaloney/.openclaw/workspace/trap-sh
go run .
```

Or run the pre-built binary:

```bash
./trap-sh
```

Go 1.22+ required. (Installed at `~/go/bin/go` on pinchy.)

## How to Play

You're a Maine lobsterman. Wake up, check the weather, haul your traps, sell at the co-op. Upgrade your boat. Don't go broke.

### Daily Loop

1. **Morning Prep** — Weather report, maintenance alerts, resource check
2. **Zone Select** — Pick your fishing grounds (A–G, varying yield/fuel cost)
3. **Haul** — Watch the day unfold in the log
4. **Sell** — Co-op takes your catch, pays market rate
5. **Next Day** — Game autosaves to `~/.trap-sh/save.json`

### Keyboard Navigation

| Key | Action |
|-----|--------|
| `1` or `/` | Main Log (scrolling day events) |
| `2` or `m` | Maintenance (boat diagnostics) |
| `3` or `d` | Dock (gear, hold, stats) |
| `4` or `w` | Wharf Market (bait, fuel, repairs) |
| `ENTER` / `SPACE` | Confirm / advance phase |
| `F` | Head out to fish (morning) |
| `S` | Stay in port / skip haul |
| `N` | Next day (after sell) |
| `↑↓` / `J K` | Navigate market list |
| `Q` | Save and quit |

### Starting Conditions

| Item | Value |
|------|-------|
| Vessel | Eastern 22 |
| Cash | $500 |
| Traps | 10 |
| Bait | 50 lbs |
| Fuel | 25 gal |
| Boat health | 50% (engine / zincs / hydraulics) |

### Fleet Progression

| Vessel | Traps | Notes |
|--------|-------|-------|
| Eastern 22 | 20 | Starter — high hull risk in SCA |
| Calvin Beal 34 | 60 | $45,000 — solid workhorse |
| Duffy 35 | 65 | $50,000 — slightly better hauls |
| Young Bros 40 | 100 | $120,000 — serious operation |
| Wesmac 46 | 150 | $280,000 — top of the fleet |

### Weather

| Condition | Fishing | Catch Modifier |
|-----------|---------|----------------|
| Clear | ✓ | 100% |
| Fog | ✓ | 85% |
| Small Craft Advisory | ✗ | — |
| Gale Warning | ✗ | — |

> ⚠ Eastern 22 + SCA = extra hull stress if you brave it.

### Haul Formula

```
catch_lbs = base_haul × traps × (gear_health/100) × zone_mult × weather_mod × luck(0.7–1.3)
```

**Gear health** = average of Engine + Zincs + Hydraulics.

### Component Degradation

| Component | Wear/Day | Repair Cost | Risk if Ignored |
|-----------|----------|-------------|-----------------|
| Engine | ~2.5% | $12/pt | Breakdown at sea |
| Zincs (anodes) | ~1.2% | $3/pt | Hull corrosion |
| Hydraulics | ~1.8% | $8/pt | Hauler failure |

### Save File

Game saves automatically each day-end to:

```
~/.trap-sh/save.json
```

Delete it to start a new game.

---

## Project Structure

```
trap-sh/
├── main.go      — Entry point, program init
├── state.go     — Game state, boats, weather, zones, haul calc
├── model.go     — Bubble Tea model, views, styles
├── go.mod
├── go.sum
└── README.md
```

## Tech Stack

- **Go** — game logic, save/load
- **[Bubble Tea](https://github.com/charmbracelet/bubbletea)** — TUI framework
- **[Lip Gloss](https://github.com/charmbracelet/lipgloss)** — CGA/EGA-style terminal colors
- **[Bubbles](https://github.com/charmbracelet/bubbles)** — viewport for log scrolling

Colors approximate the CGA 16-color palette: bright green, amber, cyan on black.

---

*"Haul more traps."*
