package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ─── Boat Definitions ────────────────────────────────────────────────────────

type BoatModel struct {
	Name         string
	Length       int
	MaxTraps     int
	HullRisk     float64 // multiplier for damage in rough weather (higher = more risk)
	FuelCap      int     // gallons, diesel tank
	FuelBurnRate float64 // gal/hr at cruise (diesel inboard)
	BaitCap      int     // lbs of herring the boat can carry (fish totes on deck)
	MaxSteamHrs  float64 // max round-trip steam hours (range limit); 0 = unlimited
	Cost         int
	TrapCost     float64 // replacement cost per trap ($)
	BaseHaul     float64 // lbs per trap per haul (realistic Maine avg ~1.5-2.5 lbs/trap/day)
}

var BoatModels = map[string]BoatModel{
	// Eastern 22: Yanmar/Volvo diesel inboard, ~2.5 gal/hr, 40-gal tank
	// Fishes nearshore to mid-range; outer zones are a stretch
	"Eastern 22": {
		Name: "Eastern 22", Length: 22, MaxTraps: 40,
		HullRisk: 2.5, FuelCap: 40, FuelBurnRate: 2.5, BaitCap: 75, MaxSteamHrs: 6.5, Cost: 0, TrapCost: 175, BaseHaul: 2.0,
	},
	// Crowley Beal 28: step up from the Eastern 22; popular first real boat in Maine
	"Crowley Beal 28": {
		Name: "Crowley Beal 28", Length: 28, MaxTraps: 100,
		HullRisk: 1.8, FuelCap: 70, FuelBurnRate: 3.2, BaitCap: 150, MaxSteamHrs: 7.5, Cost: 25000, TrapCost: 170, BaseHaul: 2.1,
	},
	"Calvin Beal 34": {
		Name: "Calvin Beal 34", Length: 34, MaxTraps: 200,
		HullRisk: 1.2, FuelCap: 120, FuelBurnRate: 4.5, BaitCap: 500, Cost: 45000, TrapCost: 165, BaseHaul: 2.2,
	},
	"Duffy 35": {
		Name: "Duffy 35", Length: 35, MaxTraps: 400,
		HullRisk: 1.1, FuelCap: 130, FuelBurnRate: 4.8, BaitCap: 600, Cost: 85000, TrapCost: 155, BaseHaul: 2.3,
	},
	"Young Bros 40": {
		Name: "Young Bros 40", Length: 40, MaxTraps: 600,
		HullRisk: 0.8, FuelCap: 200, FuelBurnRate: 9.0, BaitCap: 900, Cost: 120000, TrapCost: 150, BaseHaul: 2.5,
	},
	"Wesmac 46": {
		Name: "Wesmac 46", Length: 46, MaxTraps: 800,
		HullRisk: 0.5, FuelCap: 300, FuelBurnRate: 14.0, BaitCap: 1400, Cost: 280000, TrapCost: 150, BaseHaul: 2.7,
	},
}

// ─── Weather ─────────────────────────────────────────────────────────────────

type WeatherType string

const (
	WeatherClear WeatherType = "Clear"
	WeatherFog   WeatherType = "Fog"
	WeatherSCA   WeatherType = "Small Craft Advisory"
	WeatherGale  WeatherType = "Gale Warning"
)

type Weather struct {
	Type       WeatherType
	CanFish    bool
	Modifier   float64 // catch modifier
	HullDamage float64 // base hull damage %
	// NOAA forecast fields
	WindDir    string
	WindKts    int
	GustKts    int
	SeasFt     int
	SeasFtHigh int
	VisNM      string // e.g. "1 NM or less", "3 to 5 NM", "unrestricted"
	Fog        bool
	Precip     string // e.g. "", "A slight chance of showers.", "Rain likely."
}

// NOAAForecast returns a formatted NOAA-style marine forecast string.
func (w Weather) NOAAForecast() string {
	var parts []string

	// Wind line
	var windLine string
	if w.WindKts <= 8 {
		windLine = fmt.Sprintf("%s winds around %d kt.", w.WindDir, w.WindKts)
	} else if w.GustKts > 0 {
		windLine = fmt.Sprintf("%s winds %d to %d kt with gusts up to %d kt.", w.WindDir, w.WindKts, w.WindKts+3, w.GustKts)
	} else {
		windLine = fmt.Sprintf("%s winds %d to %d kt.", w.WindDir, w.WindKts, w.WindKts+3)
	}
	parts = append(parts, windLine)

	// Seas
	parts = append(parts, fmt.Sprintf("Seas %d to %d ft.", w.SeasFt, w.SeasFtHigh))

	// Wave detail (two wave trains, realistic directions/periods)
	waveDir1 := w.WindDir
	waveDir2 := waveTrainDir(w.WindDir)
	wavePeriod1 := 4 + rand.Intn(4) // 4-7 sec wind waves
	wavePeriod2 := 8 + rand.Intn(5) // 8-12 sec swell
	waveHt1 := w.SeasFt
	waveHt2 := max(1, w.SeasFt-1)
	parts = append(parts, fmt.Sprintf("Wave Detail: %s %d ft at %d seconds and %s %d ft at %d seconds.",
		waveDir1, waveHt1, wavePeriod1, waveDir2, waveHt2, wavePeriod2))

	// Fog
	if w.Fog {
		fogPhrases := []string{
			"Areas of fog.",
			"Widespread fog.",
			"Areas of dense fog.",
			"Patchy dense fog.",
		}
		parts = append(parts, fogPhrases[rand.Intn(len(fogPhrases))])
	}

	// Precip
	if w.Precip != "" {
		parts = append(parts, w.Precip)
	}

	// Visibility
	if w.VisNM != "unrestricted" {
		parts = append(parts, fmt.Sprintf("Vsby %s.", w.VisNM))
	}

	return strings.Join(parts, " ")
}

// waveTrainDir returns a secondary swell direction offset from primary wind direction.
func waveTrainDir(primary string) string {
	dirs := []string{"N", "NE", "E", "SE", "S", "SW", "W", "NW"}
	for i, d := range dirs {
		if d == primary {
			// offset 1-3 steps clockwise or counterclockwise
			offset := 1 + rand.Intn(3)
			return dirs[(i+offset)%len(dirs)]
		}
	}
	return "E"
}

var windDirs = []string{"N", "NE", "E", "SE", "S", "SW", "W", "NW"}

var weatherWeights = []int{55, 25, 15, 5} // % probability each

func rollWeather() Weather {
	r := rand.Intn(100)
	wType := WeatherClear
	switch {
	case r < 55:
		wType = WeatherClear
	case r < 80:
		wType = WeatherFog
	case r < 95:
		wType = WeatherSCA
	default:
		wType = WeatherGale
	}

	dir := windDirs[rand.Intn(len(windDirs))]

	switch wType {
	case WeatherClear:
		windKts := 5 + rand.Intn(11) // 5–15 kt
		gustKts := 0
		if windKts > 10 {
			gustKts = windKts + 3 + rand.Intn(5)
		}
		seasLow := 1 + rand.Intn(2)    // 1–2 ft
		seasHigh := seasLow + rand.Intn(2) // +0-1
		precip := ""
		if rand.Float64() < 0.10 {
			precip = "A slight chance of showers."
		}
		return Weather{
			Type: WeatherClear, CanFish: true, Modifier: 1.0, HullDamage: 0.5,
			WindDir: dir, WindKts: windKts, GustKts: gustKts,
			SeasFt: seasLow, SeasFtHigh: seasHigh,
			VisNM: "unrestricted", Fog: false, Precip: precip,
		}

	case WeatherFog:
		windKts := 5 + rand.Intn(11) // 5–15 kt, fog common in light winds
		gustKts := 0
		seasLow := 1 + rand.Intn(3)
		seasHigh := seasLow + 1 + rand.Intn(2)
		return Weather{
			Type: WeatherFog, CanFish: true, Modifier: 0.85, HullDamage: 1.0,
			WindDir: dir, WindKts: windKts, GustKts: gustKts,
			SeasFt: seasLow, SeasFtHigh: seasHigh,
			VisNM: "1 NM or less", Fog: true, Precip: "",
		}

	case WeatherSCA:
		windKts := 25 + rand.Intn(14) // 25–38 kt
		gustKts := windKts + 5 + rand.Intn(8)
		seasLow := 5 + rand.Intn(4)
		seasHigh := seasLow + 2 + rand.Intn(3)
		precip := ""
		if rand.Float64() < 0.40 {
			precip = "Rain likely."
		} else if rand.Float64() < 0.30 {
			precip = "A chance of showers."
		}
		return Weather{
			Type: WeatherSCA, CanFish: false, Modifier: 0.5, HullDamage: 3.0,
			WindDir: dir, WindKts: windKts, GustKts: gustKts,
			SeasFt: seasLow, SeasFtHigh: seasHigh,
			VisNM: "3 to 5 NM", Fog: false, Precip: precip,
		}

	default: // WeatherGale
		windKts := 39 + rand.Intn(25) // 39–63 kt
		gustKts := windKts + 8 + rand.Intn(12)
		seasLow := 10 + rand.Intn(6)
		seasHigh := seasLow + 3 + rand.Intn(5)
		return Weather{
			Type: WeatherGale, CanFish: false, Modifier: 0.0, HullDamage: 0.0,
			WindDir: dir, WindKts: windKts, GustKts: gustKts,
			SeasFt: seasLow, SeasFtHigh: seasHigh,
			VisNM: "1 NM or less", Fog: rand.Float64() < 0.3, Precip: "Heavy rain.",
		}
	}
}

// ─── Zones ───────────────────────────────────────────────────────────────────

type Zone struct {
	ID          string
	Name        string
	Multiplier  float64
	SteamHours  float64  // round-trip steam + hauling idle time (hrs); fuel = SteamHours * boat.FuelBurnRate
	Description string
	Crowding    float64  // 0.0–1.0; drives trap loss from gear conflicts + pot thief odds
}

var Zones = []Zone{
	// SteamHours = round-trip transit + hauling idle; multiplied by boat burn rate for fuel used
	// Crowding: nearshore zones heavily trafficked; offshore increasingly solitary
	{"A", "Nearshore Ledges", 0.75, 2.0, "Close in, well-picked, easy steam", 0.90},
	{"B", "Eastern Bay", 0.90, 3.0, "Mid-range, decent grounds", 0.70},
	{"C", "The Mudhole", 1.10, 4.0, "Deep soft bottom, good keepers", 0.50},
	{"D", "Green Island Shoals", 1.00, 3.5, "Classic zone, reliable but crowded", 0.60},
	{"E", "Outer Ledges", 1.30, 6.0, "Far out, big lobster if you can get there", 0.25},
	{"F", "The Rip", 1.45, 8.0, "Rough crossing, premium grounds", 0.15},
	{"G", "Deep Water Drop-off", 1.25, 10.0, "Long steam, cold water giants", 0.10},
}

// ─── Components ──────────────────────────────────────────────────────────────

type Component struct {
	Name         string
	Health       float64 // 0-100
	DailyWear    float64 // % degradation per fishing day
	RepairCostPt float64 // $ per health point to repair
}

// ─── Game State ──────────────────────────────────────────────────────────────

type Phase int

const (
	PhaseMorning Phase = iota
	PhaseZoneSelect
	PhaseHauling
	PhaseDecision // mid-haul random event waiting for player input
	PhaseSell
	PhaseEvening // vices: booze, scratch tickets
	PhaseGameOver
)

var vesselNames = []string{
	"Ella Faye", "The Bearded Clam", "Low Tide Lady", "Dirty Oarlocks",
	"No Sleep Till Stonington", "Herring Bone", "Buoy Oh Buoy",
	"Sternly Worded", "The Lobster Gospel", "Deadrise",
	"Piss & Vinegar", "Widow Maker", "Cold Haul", "Fog Bound",
	"Salt & Circumstance", "Hard Aground", "Two Pot Screamer",
	"Miss Conduct", "The Daily Grind", "Gut Pile",
	"Reel Therapy", "Knot Working", "Bait & Switch",
	"The Pintail", "Coffee & Diesel", "Barely Legal",
	"Deep Pockets", "Dead Reckoning", "The Green Buoy",
}

func randomVesselName() string {
	return vesselNames[rand.Intn(len(vesselNames))]
}

type GameState struct {
	Day        int         `json:"day"`
	Money      float64     `json:"money"`
	BoatName   string      `json:"boat_name"`
	VesselName string      `json:"vessel_name"`
	Traps      int         `json:"traps"`
	Bait       int         `json:"bait"`  // lbs
	Fuel       int         `json:"fuel"`  // gallons
	Engine     float64     `json:"engine_health"`
	Zincs      float64     `json:"zincs_health"`
	Hydraulics float64     `json:"hydraulics_health"`
	Freezer    float64     `json:"freezer_lbs"` // lbs of lobster in hold
	BankLoan     float64 `json:"bank_loan"`
	CrewWage     float64 `json:"crew_wage_daily"` // $0 for solo Eastern 22
	DaysWithBooze int  `json:"days_with_booze"` // consecutive nights drinking
	Hungover      bool `json:"hungover"`        // can't fish next day (booze)
	DayLost       bool `json:"day_lost"`        // can't fish next day (other)
	LastWeather string     `json:"last_weather"`
	TotalCatch  float64    `json:"total_catch_lbs"`
	TotalRevenue float64    `json:"total_revenue"`
	// Daily market prices by grade [chix, quarters, selects, jumbos, supers, culls]
	DailyPrices  [7]float64 `json:"daily_prices"`
	HotCrabZone  string     `json:"hot_crab_zone"`  // zone ID with heavy crab today, "" = none
	HotFishZone  string     `json:"hot_fish_zone"`  // zone running +40% today
	ColdFishZone string     `json:"cold_fish_zone"` // zone running -25% today
	DieselPrice  float64    `json:"diesel_price"`  // $/gal, marine diesel
	BaitPrice    float64    `json:"bait_price"`    // $/lb, fresh herring spot price

	// Equipment
	HasRadar      bool `json:"has_radar"`       // fog: unlocks all zones
	HasGPS        bool `json:"has_gps"`         // unlocks zones F/G
	HasVHF        bool `json:"has_vhf"`         // weather forecast + distress events
	HasUpgHauler  bool `json:"has_upg_hauler"`  // +30% yield, slower hydraulic wear
	HasLiveWell   bool `json:"has_live_well"`    // +10% sale price — lobsters arrive alive and lively
	HasBaitFreezer bool `json:"has_bait_freezer"` // 500 lb bait capacity
	HasGrapple          bool `json:"has_grapple"`           // recover lost gear from bottom
	TowedIn             bool `json:"towed_in"`              // towed home — engine fire, mechanical
	HasFireExtinguisher bool `json:"has_fire_extinguisher"` // one-time use — fight engine fire
	HasLifeRaft         bool `json:"has_life_raft"`         // one-time use — survive sinking
	HasDepthSound bool `json:"has_depth_sound"` // full catch rate in deep zones (D-G)
	HasExhaustHX  bool `json:"has_exhaust_hx"`  // heat exchanger: reduces engine wear
	HasDeckLights bool `json:"has_deck_lights"` // early departure, +10% catch

	// Licenses
	HasCrabPermit       bool `json:"has_crab_permit"`       // keep/sell Jonah + rock crab
	HasGroundfishPermit bool `json:"has_groundfish_permit"` // keep/sell monkfish + sea bass

	HasSternman      bool `json:"has_sternman"`       // hired for today only, resets each morning
	SternmanSkilled  bool `json:"sternman_skilled"`   // experienced vs greenhand
	FlatlanderBonus  bool `json:"flatlander_bonus"`   // 20% price bump today (resets each morning)

	PendingVandalism bool `json:"pending_vandalism"` // trap thief flagged — engine damage possible next morning
}

// EffectiveBaitCap returns the actual bait storage available — boat base or 500 with freezer upgrade.
func (gs *GameState) EffectiveBaitCap() int {
	base := BoatModels[gs.BoatName].BaitCap
	if gs.HasBaitFreezer && base < 500 {
		return 500
	}
	return base
}

func newGame() *GameState {
	return &GameState{
		Day:        1,
		Money:      500,
		BoatName:   "Eastern 22",
		VesselName: randomVesselName(),
		Traps:      20,
		Bait:       50,
		Fuel:       40, // full tank (Eastern 22 = 40 gal)
		Engine:     100,
		Zincs:      100,
		Hydraulics: 100,
		Freezer:    0,
		BankLoan:    0,
		CrewWage:    0,
		DieselPrice: RollDieselPrice(),
		BaitPrice:   RollBaitPrice(),
	}
}

func savePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".trap-sh", "save.json")
}

func saveGame(gs *GameState) error {
	p := savePath()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(gs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

func loadGame() *GameState {
	p := savePath()
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var gs GameState
	if err := json.Unmarshal(data, &gs); err != nil {
		return nil
	}
	return &gs
}

// ─── Haul Calculation ────────────────────────────────────────────────────────

// GradeResult holds lbs and price for one size grade
type GradeResult struct {
	Name  string
	Lbs   float64
	Price float64 // $/lb
}

type HaulResult struct {
	CatchLbs      float64
	Revenue       float64
	Grades        []GradeResult // breakdown by size grade
	FuelUsed      int
	BaitUsed      int
	EngineDmg     float64
	ZincsDmg      float64
	HydraulicsDmg float64
	HullDmg       float64
	TrapsLost     int
	CrabCrowded      bool    // crabs reduced lobster catch this haul
	SternmanMishap   bool    // greenhand caused a mishap
	SternmanMishapLbs float64 // lbs lost to mishap
	// Bycatch (kept, with permit)
	JonahCrabLbs   float64
	RockCrabLbs    float64
	GroundfishName string
	GroundfishLbs  float64
	// Bycatch (thrown back, no permit)
	ThrownCrabLbs        float64
	ThrownGroundfishName string
	ThrownGroundfishLbs  float64
}

// RollDailyPrices generates co-op dock prices for the day
func RollDailyPrices() [7]float64 {
	shift := rand.Float64()*0.60 - 0.30 // ±$0.30 market swing
	return [7]float64{
		math.Max(4.00, 4.75+shift),   // Chix       1.0–1.2 lb
		math.Max(5.00, 5.75+shift),   // Quarters   1.25–1.4 lb
		math.Max(6.00, 6.75+shift),   // Halves     1.5–1.7 lb
		math.Max(7.00, 8.25+shift),   // Selects    1.75–2.4 lb
		math.Max(9.00, 10.50+shift),  // Deuces     2.5–2.9 lb
		math.Max(11.00, 13.00+shift), // Jumbos     3.0–4.0 lb
		math.Max(2.50, 3.50+shift),   // Culls
	}
}

// RollDieselPrice: marine diesel in Maine $4.00–$5.50/gal
func RollDieselPrice() float64 {
	return math.Round((4.00+rand.Float64()*1.50)*100) / 100
}

// RollBaitPrice: fresh herring spot price $0.40–$0.80/lb
func RollBaitPrice() float64 {
	return math.Round((0.40+rand.Float64()*0.40)*100) / 100
}

// RollHotCrabZone returns a zone ID that's running heavy on crab today.
// 65% chance any zone is hot; "" means no zone is particularly crabby.
func RollHotCrabZone() string {
	if rand.Float64() > 0.65 {
		return ""
	}
	zones := []string{"A", "B", "C", "D", "E", "F", "G"}
	return zones[rand.Intn(len(zones))]
}

// RollHotFishZone picks today's hot zone (+40%) and cold zone (-25%).
// Nearshore zones (A-D) are weighted 3x more likely to be hot — they're volatile.
// Hot and cold are always different zones.
func RollHotFishZone() (hot, cold string) {
	weights := []struct {
		id     string
		weight int
	}{
		{"A", 3}, {"B", 3}, {"C", 3}, {"D", 3}, {"E", 1}, {"F", 1}, {"G", 1},
	}
	pick := func(exclude string) string {
		pool := []string{}
		for _, w := range weights {
			if w.id == exclude {
				continue
			}
			for i := 0; i < w.weight; i++ {
				pool = append(pool, w.id)
			}
		}
		return pool[rand.Intn(len(pool))]
	}
	hot = pick("")
	cold = pick(hot)
	return hot, cold
}

// gradeDistribution returns fractions (must sum to 1.0) for
// [chix, quarters, halves, selects, deuces, jumbos, culls]
// Jumbos (3.0+ lb) only appear meaningfully in zones E-G
func gradeDistribution(zone Zone) [7]float64 {
	switch zone.ID {
	case "A": // nearshore, heavily fished — mostly chix and quarters
		return [7]float64{0.45, 0.25, 0.12, 0.07, 0.02, 0.01, 0.08}
	case "B":
		return [7]float64{0.38, 0.24, 0.14, 0.10, 0.04, 0.03, 0.07}
	case "C": // mudhole — soft bottom, decent mix
		return [7]float64{0.30, 0.24, 0.17, 0.13, 0.05, 0.04, 0.07}
	case "D": // classic zone, crowded
		return [7]float64{0.35, 0.25, 0.15, 0.12, 0.04, 0.02, 0.07}
	case "E": // outer ledges — better mix, jumbos start showing up
		return [7]float64{0.20, 0.22, 0.20, 0.17, 0.08, 0.06, 0.07}
	case "F": // the rip — prime grounds, real jumbos
		return [7]float64{0.13, 0.17, 0.20, 0.20, 0.13, 0.10, 0.07}
	case "G": // deep cold water — best grade, most jumbos
		return [7]float64{0.08, 0.13, 0.18, 0.22, 0.17, 0.15, 0.07}
	default:
		return [7]float64{0.33, 0.24, 0.16, 0.12, 0.05, 0.03, 0.07}
	}
}

func simulateHaul(gs *GameState, zone Zone, weather Weather) HaulResult {
	boat := BoatModels[gs.BoatName]

	// Gear health = avg of components
	gearHealth := (gs.Engine + gs.Zincs + gs.Hydraulics) / 3.0

	// Day-to-day luck factor: 0.75-1.25
	luck := 0.75 + rand.Float64()*0.5

	// No bait = no catch. Lobsters won't enter an unbaited trap.
	if gs.Bait == 0 {
		return HaulResult{CatchLbs: 0, Revenue: 0, Grades: nil,
			FuelUsed: int(zone.SteamHours*boat.FuelBurnRate+0.5),
			BaitUsed: 0,
			EngineDmg: 0.5 + rand.Float64()*0.5,
			ZincsDmg:  1.0 + rand.Float64()*1.0,
			HydraulicsDmg: 0.4 + rand.Float64()*0.5,
		}
	}

	// Catch: BaseHaul lbs/trap/day * traps * modifiers
	// BaseHaul is calibrated to real Maine averages (~1.5-2.7 lbs/trap/haul)
	catchLbs := boat.BaseHaul * float64(gs.Traps) *
		(gearHealth / 100.0) *
		zone.Multiplier *
		weather.Modifier *
		luck

	// Hydraulic hauler + davit — haul more pots, swing them aboard faster
	if gs.HasUpgHauler {
		catchLbs *= 1.30
	}

	// Daily hot/cold zone modifiers
	if gs.HotFishZone == zone.ID {
		catchLbs *= 1.40
	}
	if gs.ColdFishZone == zone.ID {
		catchLbs *= 0.75
	}

	if catchLbs < 0 {
		catchLbs = 0
	}

	// Bait shortage penalty: if you don't have enough bait, catch scales down proportionally
	baitNeeded := int(float64(gs.Traps)*2.5 + 0.5) // midpoint of 2-3 lbs/trap
	if gs.Bait < baitNeeded {
		catchLbs *= float64(gs.Bait) / float64(baitNeeded)
	}

	// Use today's rolled prices (set at morning by RollDailyPrices)
	basePrices := gs.DailyPrices
	// Live well keeps lobsters alive and lively — co-op pays a premium
	if gs.HasLiveWell {
		for i := range basePrices {
			basePrices[i] *= 1.10
		}
	}
	gradeNames := [7]string{"Chix", "Quarters", "Halves", "Selects", "Deuces", "Jumbos", "Culls"}
	dist := gradeDistribution(zone)

	// Minimum weight to have any lobsters of a given grade (one lobster minimum)
	// [chix, quarters, selects, jumbos, supers, culls]
	gradeMinLbs := [7]float64{1.0, 1.25, 1.5, 1.75, 2.5, 3.0, 1.0}

	// First pass: compute raw lbs per grade; zero out grades below minimum
	rawLbs := [7]float64{}
	spillover := 0.0
	for i := 0; i < 6; i++ {
		lbs := catchLbs * dist[i]
		if lbs > 0 && lbs < gradeMinLbs[i] {
			spillover += lbs // too few for even one lobster — add back to chix
			rawLbs[i] = 0
		} else {
			rawLbs[i] = lbs
		}
	}
	rawLbs[0] += spillover // redistribute sub-minimum grades into chix

	var grades []GradeResult
	revenue := 0.0
	for i := 0; i < 6; i++ {
		lbs := rawLbs[i]
		price := basePrices[i]
		if price < 0 { price = 0 }
		revenue += lbs * price
		if lbs >= gradeMinLbs[i] {
			grades = append(grades, GradeResult{Name: gradeNames[i], Lbs: lbs, Price: price})
		}
	}

	// Fuel: SteamHours * boat burn rate, ±10% variance
	fuelRaw := zone.SteamHours * boat.FuelBurnRate * (0.9 + rand.Float64()*0.2)
	fuelUsed := int(fuelRaw + 0.5) // round to nearest gallon
	if fuelUsed < 1 {
		fuelUsed = 1
	}
	if fuelUsed > gs.Fuel {
		fuelUsed = gs.Fuel
	}

	// Bait: ~2-3 lbs herring per trap per haul (commercial Maine standard)
	// baitUsed computed below after crab rolls (crabs affect bait consumption)

	// Component wear scales with steam hours — deeper zones = longer run = more wear
	// Zone A (~2.5 hrs) is baseline; Zone G (~10 hrs) is 4x the steam
	steamFactor := zone.SteamHours / 4.0 // normalize: 4 hrs = 1.0x

	// Engine: ~0.5-1% wear at baseline; heat exchanger reduces 60%
	engineDmg := (0.5 + rand.Float64()*0.5) * steamFactor
	if gs.HasExhaustHX {
		engineDmg *= 0.4
	}
	// Zincs: saltwater exposure scales with time on the water
	zincsDmg := (1.0 + rand.Float64()*1.0) * steamFactor
	// Hydraulics: hauler cycles — more traps and longer run means more pump hours
	hydDmg := (0.4 + rand.Float64()*0.5) * steamFactor
	if gs.HasSternman && gs.SternmanSkilled {
		hydDmg *= 0.8 // experienced hand handles the hauler properly
	}

	// Hull damage from weather
	hullDmg := weather.HullDamage * boat.HullRisk * (rand.Float64() * 0.5 + 0.5)

	// Trap loss — realistic Maine commercial rates
	// ~5-10% of traps lost annually over ~150 fishing days = 0.03-0.07% per trap per haul
	trapLossRate := 0.0005 // 0.05% per trap baseline (normal conditions)
	if weather.Type == WeatherSCA {
		trapLossRate = 0.002 // 0.2% per trap in rough seas (~4x normal)
	}
	// Deeper zones = rockier bottom, stronger current
	trapLossRate += zone.SteamHours * 0.00008
	// Crowded zones = gear conflicts with other boats, lines crossed, traps run over
	trapLossRate += zone.Crowding * 0.0004
	// Upgraded hauler = better line handling
	if gs.HasUpgHauler {
		trapLossRate *= 0.6
	}
	trapsLost := 0
	for i := 0; i < gs.Traps; i++ {
		if rand.Float64() < trapLossRate {
			trapsLost++
		}
	}

	// Bycatch
	var jonahLbs, rockLbs, groundfishLbs float64
	var groundfishName string

	traps := float64(gs.Traps)

	// Crab bycatch — presence and weight vary by zone depth
	// Nearshore (A/B): fewer crabs, shallower mud; deeper (C+): rocky bottom, heavier crab load
	crabFactor := 0.5 + (zone.SteamHours/10.0)*0.8 // 0.5x nearshore → 1.3x offshore
	if gs.HotCrabZone == zone.ID {
		crabFactor *= 1.6 // hot zone: crabs running heavy today
	}
	jonahChance := math.Min(0.85, 0.30*crabFactor)
	rockChance  := math.Min(0.65, 0.18*crabFactor)

	var rolledJonahLbs, rolledRockLbs float64
	if rand.Float64() < jonahChance {
		// 2–7 lbs/trap on an average day; good days push higher
		rolledJonahLbs = traps * (2.0 + rand.Float64()*5.0) * crabFactor
	}
	if rand.Float64() < rockChance {
		// 1–5 lbs/trap; rock crabs less common than Jonah
		rolledRockLbs = traps * (1.0 + rand.Float64()*4.0) * crabFactor
	}

	// Trap crowding: crabs eat bait and fill space, reducing lobster catch
	// Each trap holds ~20 lbs total; heavy crab load crowds out lobster
	const trapCapacityLbs = 20.0
	crabPerTrap := (rolledJonahLbs + rolledRockLbs) / math.Max(1, traps)
	crowding := math.Max(0.5, 1.0-(crabPerTrap/trapCapacityLbs)*0.8)
	catchLbs *= crowding

	// Deck lights — early departure, extra traps pulled in the dark hour
	if gs.HasDeckLights {
		catchLbs *= 1.10
	}

	// Sternman boost
	var sternmanMishap bool
	var sternmanMishapLbs float64
	if gs.HasSternman {
		if gs.SternmanSkilled {
			catchLbs *= 1.30
		} else {
			catchLbs *= 1.15
			// 10% chance greenhand causes a mishap — drops keepers overboard
			if rand.Float64() < 0.10 {
				sternmanMishap = true
				sternmanMishapLbs = 5.0 + rand.Float64()*10.0
				catchLbs = math.Max(0, catchLbs-sternmanMishapLbs)
			}
		}
	}

	// Bait: crabs eat bait aggressively — heavy crab load burns through herring faster
	baitPerTrap := 2.0 + rand.Float64()*1.0 // 2.0-3.0 lbs/trap baseline
	if crabPerTrap > 10.0 {
		baitPerTrap *= 1.5
	} else if crabPerTrap > 5.0 {
		baitPerTrap *= 1.25
	} else if crabPerTrap > 2.0 {
		baitPerTrap *= 1.1
	}
	baitUsed := int(float64(gs.Traps)*baitPerTrap + 0.5)
	if baitUsed > gs.Bait {
		baitUsed = gs.Bait
	}
	if gs.HasCrabPermit {
		jonahLbs = rolledJonahLbs
		rockLbs = rolledRockLbs
	}

	// Groundfish encounter — rates scale with zone depth/distance
	// Cusk/Hake: nearshore to mid; Monkfish: mid to deep; Halibut: offshore only
	var rolledGFName string
	var rolledGFLbs float64
	sh := zone.SteamHours
	if sh >= 2.0 {
		// Cusk/Hake: present nearshore, peak mid-range, tails off deep
		cuskChance := math.Min(0.35, 0.06*sh)
		// Monkfish: rare nearshore, peaks offshore
		monkChance := math.Min(0.40, 0.04*sh)
		// Halibut: offshore only — needs 6+ hrs steam to have any real shot
		haliChance := math.Max(0, (sh-5.0)*0.025) // 0% at Zone E (6h)=2.5%, Zone G (10h)=12.5%

		switch {
		case rand.Float64() < haliChance:
			rolledGFName = "Halibut"
			rolledGFLbs = 8.0 + rand.Float64()*22.0 // 8–30 lbs flat
		case rand.Float64() < monkChance:
			rolledGFName = "Monkfish"
			rolledGFLbs = traps * (0.01 + rand.Float64()*0.04) * (sh / 4.0)
		case rand.Float64() < cuskChance:
			rolledGFName = "Cusk/Hake"
			rolledGFLbs = traps * (0.02 + rand.Float64()*0.04)
		}
	}
	if gs.HasGroundfishPermit {
		groundfishName = rolledGFName
		groundfishLbs = rolledGFLbs
	}

	return HaulResult{
		CatchLbs:      catchLbs,
		Revenue:       revenue,
		Grades:        grades,
		FuelUsed:      fuelUsed,
		BaitUsed:      baitUsed,
		EngineDmg:     engineDmg,
		ZincsDmg:      zincsDmg,
		HydraulicsDmg: hydDmg,
		HullDmg:       hullDmg,
		TrapsLost:     trapsLost,
		CrabCrowded:       crowding < 0.85,
		SternmanMishap:    sternmanMishap,
		SternmanMishapLbs: sternmanMishapLbs,
		JonahCrabLbs:         jonahLbs,
		RockCrabLbs:          rockLbs,
		GroundfishName:       groundfishName,
		GroundfishLbs:        groundfishLbs,
		ThrownCrabLbs:        func() float64 { if !gs.HasCrabPermit { return rolledJonahLbs + rolledRockLbs }; return 0 }(),
		ThrownGroundfishName: func() string  { if !gs.HasGroundfishPermit { return rolledGFName }; return "" }(),
		ThrownGroundfishLbs:  func() float64 { if !gs.HasGroundfishPermit { return rolledGFLbs }; return 0 }(),
	}
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

// RollRandomEvent returns a random mid-haul event, or nil (70% chance of none)
func RollRandomEvent(gs *GameState, weather Weather) *RandomEvent {
	if rand.Float64() > 0.50 {
		return nil
	}
	events := []RandomEvent{
		{
			Type:   EventBerriedHen,
			Time:   "1045",
			Desc:   "1045 — Big mama came up. V-notch, eggs all over the swimmerets. She's a broodstock female.",
			KeyA:   "k", LabelA: "[K] Keep her (+$18, risk fine)",
			KeyB:   "t", LabelB: "[T] Throw her back (legal)",
		},
		{
			Type:   EventSquareGrouper,
			Time:   "0910",
			Desc:   "0910 — Something big tangled in the buoy line. Wrapped in plastic, waterlogged. You know what this is.",
			KeyA:   "k", LabelA: "[K] Haul it aboard (+$2,500)",
			KeyB:   "r", LabelB: "[R] Radio the Coast Guard",
		},

		{
			Type:   EventGhostTrap,
			Time:   "1200",
			Desc:   "1200 — Found a derelict trap on the bottom. No buoy, no tag. Full of lobsters.",
			KeyA:   "h", LabelA: "[H] Haul it up (gray area, extra lbs)",
			KeyB:   "l", LabelB: "[L] Leave it",
		},
		{
			Type:   EventStormComing,
			Time:   "1145",
			Desc:   "1145 — NOAA just issued a Gale Warning. Storm moving faster than forecast. You're an hour from the last set.",
			KeyA:   "p", LabelA: "[P] Push through and finish",
			KeyB:   "h", LabelB: "[H] Head in now (keep what you have)",
		},
	}
	// Boat in distress only if player has VHF
	if gs.HasVHF {
		events = append(events, RandomEvent{
			Type:   EventBoatDistress,
			Time:   "0955",
			Desc:   "0955 — Mayday on channel 16. Lobsterboat taking on water, 2 miles east.",
			KeyA:   "h", LabelA: "[H] Go help (lose 2 hrs fishing)",
			KeyB:   "i", LabelB: "[I] Keep hauling (someone else will get it)",
		})
	}
	// Neighbor's trap — always possible
	events = append(events, RandomEvent{
		Type:   EventNeighborTrap,
		Time:   "0850",
		Desc:   "0850 — Someone else's warp fouled in your line. Trap came up with it. Still loaded.",
		KeyA:   "h", LabelA: "[H] Haul it, keep the catch",
		KeyB:   "l", LabelB: "[L] Untangle and drop it back",
	})

	// ── Positive events ──────────────────────────────────────────────────────

	// Hot set — one trap way over-loaded
	events = append(events, RandomEvent{
		Type:   EventHotSet,
		Time:   "0930",
		Desc:   "0930 — One trap came up absolutely loaded. Stacked to the brim. You pull it slow.",
		KeyA:   "h", LabelA: "[H] Haul every last one",
		KeyB:   "s", LabelB: "[S] Sort and toss the shorts",
	})

	// Flatlander wedding price bump
	events = append(events, RandomEvent{
		Type:   EventFlatlander,
		Time:   "0845",
		Desc:   "0845 — Co-op called on the radio. Some flatlander's having a lobster bake wedding in Bar Harbor. Paying 20% over market on everything today.",
		KeyA:   "r", LabelA: "[R] Radio back — you'll bring the haul",
		KeyB:   "i", LabelB: "[I] Ignore it, sell normal",
	})

	// Old-timer tip
	events = append(events, RandomEvent{
		Type:   EventOldTimer,
		Time:   "0730",
		Desc:   "0730 — Old Donnie Beal crackles in on channel 68. Says he's been watching the temp break. Tells you to drop deep on the east side today.",
		KeyA:   "t", LabelA: "[T] Take his advice (+10% catch)",
		KeyB:   "i", LabelB: "[I] Stick to your usual spots",
	})

	// Sunken trap windfall
	events = append(events, RandomEvent{
		Type:   EventSunkTrap,
		Time:   "1015",
		Desc:   "1015 — Depth sounder lit up. Pile of gear on the bottom — traps from last season, still loaded. Could grapple them up.",
		KeyA:   "g", LabelA: "[G] Grapple them up (free traps + catch)",
		KeyB:   "l", LabelB: "[L] Leave it",
	})

	// Gray market halibut (only if no groundfish permit)
	if !gs.HasGroundfishPermit {
		events = append(events, RandomEvent{
			Type:   EventGrayMarketHalibut,
			Time:   "1050",
			Desc:   "1050 — Halibut in the trap. Fat one. No groundfish permit — you're not supposed to keep it.",
			KeyA:   "k", LabelA: "[K] Keep it, sell it quiet ($150 cash)",
			KeyB:   "t", LabelB: "[T] Throw it back",
		})
	}

	// ── Negative events ──────────────────────────────────────────────────────

	// Seal raid
	events = append(events, RandomEvent{
		Type:   EventSealRaid,
		Time:   "0920",
		Desc:   "0920 — Big gray seal working the gear ahead of you. Pulling lobsters right out of the traps as they come up.",
		KeyA:   "s", LabelA: "[S] Bang the hull, try to scare it off",
		KeyB:   "i", LabelB: "[I] Ignore it, keep hauling",
	})

	// Coast Guard permit check
	events = append(events, RandomEvent{
		Type:   EventCGCheck,
		Time:   "1100",
		Desc:   "1100 — Coast Guard vessel off the port bow. They're hailing you for a routine boarding.",
		KeyA:   "h", LabelA: "[H] Heave to and cooperate",
		KeyB:   "r", LabelB: "[R] Radio that you're hauling, ask for delay",
	})

	// Engine temp in the red
	events = append(events, RandomEvent{
		Type:   EventEngineTempHigh,
		Time:   "1030",
		Desc:   "1030 — Engine temp gauge climbing into the red. Could be the thermostat. Could be worse.",
		KeyA:   "p", LabelA: "[P] Push through and finish the string",
		KeyB:   "h", LabelB: "[H] Throttle back and head in now",
	})

	// Humpback in the zone
	events = append(events, RandomEvent{
		Type:   EventHumpback,
		Time:   "0950",
		Desc:   "0950 — Humpback working the zone. Big one. You can see the entanglement risk from here.",
		KeyA:   "h", LabelA: "[H] Haul around it (risk fine if gear tangles)",
		KeyB:   "p", LabelB: "[P] Pull early and move off",
	})

	// Rival boat on your grounds
	events = append(events, RandomEvent{
		Type:   EventRivalBoat,
		Time:   "0900",
		Desc:   "0900 — Danny Colby's been working right next to your string all week. That's your spot and he knows it.",
		KeyA:   "r", LabelA: "[R] Radio him — back off",
		KeyB:   "i", LabelB: "[I] Let it go",
	})

	// Warden asking about a trap
	events = append(events, RandomEvent{
		Type:   EventWardenQuestions,
		Time:   "1015",
		Desc:   "1015 — Marine Patrol alongside. Warden asking questions about a trap nearby — wrong buoy colors, might be unlicensed. Not yours, but you know whose it is.",
		KeyA:   "t", LabelA: "[T] Tell him whose it is",
		KeyB:   "p", LabelB: "[P] Play dumb",
	})

	// Double-loaded trap (no decision — pure flavor + bonus)
	events = append(events, RandomEvent{
		Type:   EventDoubleLoaded,
		Time:   "0935",
		Desc:   "0935 — One trap came up so full the bricks were barely holding. Bait must've been perfect. Pulled it slow.",
		KeyA:   "", LabelA: "",
		KeyB:   "", LabelB: "",
	})

	// Help a neighbor haul their string for a cut
	events = append(events, RandomEvent{
		Type:   EventHelpNeighbor,
		Time:   "0845",
		Desc:   "0845 — Billy Thurston on channel 68. Hauler seized up mid-string. Asking if anyone can help finish his last set. Offering a quarter share.",
		KeyA:   "h", LabelA: "[H] Go help (lose ~25% of your haul, get a cut)",
		KeyB:   "k", LabelB: "[K] Keep hauling your own gear",
	})

	events = append(events, RandomEvent{
		Type:   EventCoastGuardBoarding,
		Time:   "0930",
		Desc:   "0930 — Coast Guard Aids to Navigation boat off your stern. They're coming alongside. Routine safety inspection — extinguisher, life raft, flares. $500 per violation.",
		KeyA:   "k", LabelA: "[K] Let them aboard (you have no choice)",
	})

	events = append(events, RandomEvent{
		Type: EventEngineFire,
		Time: "1130",
		Desc: func() string {
			if gs.HasFireExtinguisher {
				return "1130 — Smoke coming out of the engine box. She's running hot and something caught. You've got a fire extinguisher aboard."
			}
			return "1130 — Smoke out of the engine box. Something caught. No extinguisher. The smoke is getting thick."
		}(),
		KeyA: func() string {
			if gs.HasFireExtinguisher {
				return "e"
			}
			return "a"
		}(),
		LabelA: func() string {
			if gs.HasFireExtinguisher {
				return "[E] Fight it — use the extinguisher"
			}
			return "[A] Abandon ship — nothing to fight it with"
		}(),
		KeyB: func() string {
			if gs.HasFireExtinguisher {
				return "a"
			}
			return ""
		}(),
		LabelB: func() string {
			if gs.HasFireExtinguisher {
				return "[A] Abandon ship"
			}
			return ""
		}(),
	})

	events = append(events, RandomEvent{
		Type: EventFoundOldGear,
		Time: "1015",
		Desc: func() string {
			if gs.HasGrapple {
				return "1015 — Buoy-less line snagged on your warp. Looks like it's been down there a while. Could be a full string of traps."
			}
			return "1015 — Buoy-less line off the starboard bow. Somebody lost their gear. No grapple, no way to drag for it."
		}(),
		KeyA: func() string {
			if gs.HasGrapple {
				return "g"
			}
			return ""
		}(),
		LabelA: func() string {
			if gs.HasGrapple {
				return "[G] Drag for it"
			}
			return ""
		}(),
		KeyB: "l", LabelB: "[L] Leave it",
	})

	e := events[rand.Intn(len(events))]
	return &e
}

// RollMorningGossip returns 1-2 lines of dock gossip for the morning briefing
func RollMorningGossip(gs *GameState, weather Weather) []string {
	// Contextual lines that reference actual game state
	var contextual []string
	if gs.HotCrabZone != "" {
		contextual = append(contextual,
			fmt.Sprintf("Jimmy at the co-op says Zone %s is all crabbed up. Said it like it was a bad thing. Man doesn't have a crab permit.", gs.HotCrabZone),
			fmt.Sprintf("Word around the dock is Zone %s is running heavy crab. Take it or leave it.", gs.HotCrabZone),
		)
	}
	if gs.HotFishZone != "" {
		contextual = append(contextual,
			fmt.Sprintf("Heard Zone %s is stacked this morning. Temperature break moved through last night. Worth a look.", gs.HotFishZone),
			fmt.Sprintf("Donnie Beal was out at 0400. Said Zone %s was loaded when he pulled his first string. Take it for what it's worth.", gs.HotFishZone),
			fmt.Sprintf("Co-op radio's been lighting up about Zone %s all morning. Tide's running right out there today.", gs.HotFishZone),
		)
	}
	if gs.ColdFishZone != "" {
		contextual = append(contextual,
			fmt.Sprintf("Zone %s's been quiet all week. Water temp dropped, lobsters moved. Save your fuel.", gs.ColdFishZone),
			fmt.Sprintf("Ricky Pease pulled his gear out of Zone %s yesterday. Said it wasn't worth the diesel. Man knows the water.", gs.ColdFishZone),
		)
	}
	if weather.Type == WeatherFog {
		contextual = append(contextual,
			"Thick out there this morning. Ronnie Thurston went out anyway. Ronnie Thurston also drives without his glasses. Connect the dots.",
			"Fog so thick you can't see the end of the dock. Pete Whitmore called it 'good visibility' and went out. Pete's wife looks nervous.",
		)
	}
	if weather.Type == WeatherSCA || weather.Type == WeatherGale {
		contextual = append(contextual,
			"Nobody's going out in this. Well. Crazy Eddie might. That's how he got the name.",
			"Coast Guard's been on the radio all morning. Stay in port and let 'em earn their pay.",
		)
	}
	if gs.Engine < 50 {
		contextual = append(contextual,
			"Leroy's engine seized up mid-string last week. Fixed it with wire and a prayer. Said it's 'good as new.' Check your oil.",
		)
	}

	// Static gossip pool — salty Maine humor
	static := []string{
		"Frankie Greenlaw bought a new boat. Three hundred thousand dollars. His wife left two days later. Cheaper to keep 'er, Frankie.",
		"Beautiful morning. Ruined by running into Dave Peasley at the fuel dock at 0430. Man talks like he's getting paid by the word.",
		"Waterfront restaurant in town's charging $38 for a lobster roll. Thirty-eight dollars. We pull 'em for six bucks a pound and some flatlander pays $38 for a sandwich.",
		"Co-op's new scale's been reading light. Mickey Ames weighed his boat dog on it — said 38 lbs. Dog's at least 50. We're getting robbed.",
		"State inspector came through Stonington yesterday checking V-notches. Couldn't tell a hen from a buoy. Sent him back to Augusta.",
		"New summer people put their kayak in the middle of the channel again. Tommy nearly ran 'em over. Said he tried to miss but couldn't decide which way they'd go.",
		"Heard Stevie Pomerleau's been 'fishing' Zone B all week. His wife says he's fishing. Co-op says his boat ain't moved. You do the math.",
		"Eddie from the fuel dock says diesel's going up next week. Eddie also said the Red Sox were gonna win the Series. Take that for what it's worth.",
		"Old Pete Whitmore showed up in brand new Grundéns. Still in the bag, creases and everything. Boys at the co-op said he looked like he bought 'em for a costume. He did not take it well.",
		"Ronnie Thurston's been bragging about pulling a 7-pounder. Nobody believes him. Man can barely pull his pants up straight.",
		"Jimmy at the co-op says Zone D was loaded yesterday. Jimmy also charges $4 for coffee. Man's judgment is suspect across the board.",
		"Fog rolled in on Ricky Pease out by the outer ledges. Found him going in circles an hour later. Second time this month. 'Bought a GPS,' he says. Ayuh.",
		"Marcy at the bait shed says herring's gonna be scarce next month. Course Marcy also named her cat 'Diesel' and feeds it tuna. Woman's a mystery.",
		"Selectman wants to put a hotel on the waterfront. Over my dead body. Over a lot of dead bodies, actually — that's where we keep our gear.",
		"Summer people keep waving at the lobster boats from their sailboats. Captain Danny started waving back with one finger. Progress.",
		"Heard Zone E was stacked last week. Also heard it's been picked clean since. Take your chances and your diesel.",
		"Bait's running high. Forty cents a pound more than last month. You're not fishing, you're feeding herring to the ocean.",
		"Leroy's hauler seized up mid-string Tuesday. Fixed it with a piece of wire and a prayer. Says it's good as new. Don't fish downwind of Leroy.",
		"Some college kid from UMaine's doing a 'study' on lobster migration. Been following boats around with a clipboard. We've been giving him bad data.",
		"Heard the DMR's sending out more wardens this month. Keep your V-notch throwbacks clean and your permits handy.",
		"Donnie Beal's been out since 0400 every day this week. Man's 74 years old. Makes the rest of us look bad. Intentionally, I think.",
		"Young kid from away bought a boat, painted it white, named it 'Sea Renity.' She sank at the mooring first night. Universe has a sense of humor.",
		"Tide's been running strong in Zone B all week. Lost two traps to that current this month. You've been warned and I've stopped warning.",
		"Danny Coombs got a stern camera. Says it's for 'safety.' His wife says she checks the footage every night. Different kind of safety.",
		"Price of lobster at the grocery store in Ellsworth is $24.99 a pound. We're getting $6. The math on that doesn't work in our favor.",
		"Heard there's a whale been working the outer ledges. Good news: lobster run away from whales. Bad news: so does your gear.",
		"Harold from the trap shop says wire mesh is backordered six weeks. Buy what you need now or you'll be knitting your own.",
		"Bobby Torrey got his moose permit. Taking two weeks off. Said it like he won the lottery. Might as well have.",
		"Kenny Leighton drew a moose permit third year in a row. Man puts in for every zone. Rest of us haven't seen a tag in ten years. Life ain't fair.",
		"Heard Wayne Alley's taking the week off — finally drew his moose permit after twelve years. Boat's just sitting at the mooring. Can't blame him.",
		"Shawn Conary says if this season holds he's getting two new sleds. Said the same thing last year. And the year before. Sleds are still '09s.",
		"Dale Eaton's been talking all summer about getting a new Polaris side-by-side for upta camp this fall. Dale also owes me forty bucks. I'll believe it when I see it.",
		"Terry Beal had three good weeks in a row and already bought a new sled. Didn't fix his hauler, didn't pay down his trap loan — bought a sled. That's lobstering.",
		"If the crab holds through September, Phil Robbins says he's finally getting that new camp upta Parlin Pond. Phil's been saying that since 2014.",
		"Ricky Gray told his wife if he has one more week like last week she's getting a new kitchen. His wife said she'd rather have a new sled. Woman knows what matters.",
		"Heard Daryl Sprague got pinched upta camp last weekend. Warden caught him with a doe and no tag. Rifle, freezer bags, the whole operation. IF&W don't play.",
		"Gary Hutchins thought he was slick — bagged some camp meat two weeks before season. Game warden was parked at the end of the road the whole time. Lost his license, his rifle, and his dignity. In that order.",
		"Asked Clyde how Zone C's been fishing lately. 'Hard tellin', not knowin',' he says. Helpful as always.",
		"Someone asked Donnie Beal if the price was gonna hold through October. 'Hard tellin', not knowin'.' Man's been fishing 40 years and that's his answer for everything.",
		"New guy on the dock rigged his own traps first season. Knots were something else. Ain't you cunning, kid.",
		"Warden come by checking licenses last Tuesday. Looked at Earl Coombs's setup and said 'nice rig.' Earl said 'finest kind.' Warden didn't know what that meant but he left anyway.",
		"Haul's been some good this week if the weather holds. Hard tellin' after that.",
		"Heard the co-op's getting a new scale. About time. Current one's been reading light all summer. Hard tellin' how much that's cost us.",
		"Gary Weed finally pulled that old Volvo and dropped in a John Deere. Boys at the co-op haven't let him live it down. 'You're a farmer now, Gary.'",
		"Ronnie Ames swears by his Cat. Says it'll outlast the boat, outlast him, outlast his kids. Probably right. Thing sounds like it's angry at the world.",
		"Heard Pete Conary's John Deere threw a belt mid-string out past the outer ledges. Pete says it's a fluke. Pete's Cat guys aren't surprised.",
		"Mike Robbins has been running a Cat since '98. Says he'll never switch. Also says he's never had a good day in Zone A. Man's loyal to his engines and his bad luck.",
		"Kid at the marina tried to tell Danny Thurston that John Deere and Cat are basically the same. Danny hasn't spoken to him since. It's been three weeks.",
		"Tommy Leighton put a John Deere in his new build. His father, who runs a Cat, has not visited the boat. Family dinners are tense.",
		"Bait shed's ripe this morning. People forget what real stink is. Grew up near the paper mill in Millinocket — that was stink. This is just Tuesday.",
		"Pulled a tautog this morning. Thing had more teeth than the fried dough line at Fryeburg Fair. Threw him back. Didn't trust him.",
	}

	// Prefer contextual 60% of the time when available, otherwise static
	if len(contextual) > 0 && rand.Float64() < 0.60 {
		return []string{contextual[rand.Intn(len(contextual))]}
	}
	return []string{static[rand.Intn(len(static))]}
}

