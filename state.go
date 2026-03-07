package main

import (
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"path/filepath"
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
		HullRisk: 2.5, FuelCap: 40, FuelBurnRate: 2.5, BaitCap: 250, MaxSteamHrs: 6.5, Cost: 0, TrapCost: 175, BaseHaul: 2.0,
	},
	"Calvin Beal 34": {
		Name: "Calvin Beal 34", Length: 34, MaxTraps: 300,
		HullRisk: 1.2, FuelCap: 120, FuelBurnRate: 4.5, BaitCap: 500, Cost: 45000, TrapCost: 165, BaseHaul: 2.2,
	},
	"Duffy 35": {
		Name: "Duffy 35", Length: 35, MaxTraps: 400,
		HullRisk: 1.1, FuelCap: 130, FuelBurnRate: 4.8, BaitCap: 600, Cost: 50000, TrapCost: 155, BaseHaul: 2.3,
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
	Type        WeatherType
	Description string
	CanFish     bool
	Modifier    float64 // catch modifier
	HullDamage  float64 // base hull damage %
}

var WeatherTable = []Weather{
	{WeatherClear, "Flat calm, perfect day to haul", true, 1.0, 0.5},
	{WeatherFog, "Thick fog bank, radar mandatory", true, 0.85, 1.0},
	{WeatherSCA, "Winds 25-38 kts, heavy chop", false, 0.5, 3.0},
	{WeatherGale, "Gale-force winds, stay in port", false, 0.0, 0.0},
}

var weatherWeights = []int{55, 25, 15, 5} // % probability each

func rollWeather() Weather {
	r := rand.Intn(100)
	acc := 0
	for i, w := range weatherWeights {
		acc += w
		if r < acc {
			return WeatherTable[i]
		}
	}
	return WeatherTable[0]
}

// ─── Zones ───────────────────────────────────────────────────────────────────

type Zone struct {
	ID          string
	Name        string
	Multiplier  float64
	SteamHours  float64 // round-trip steam + hauling idle time (hrs); fuel = SteamHours * boat.FuelBurnRate
	Description string
}

var Zones = []Zone{
	// SteamHours = round-trip transit + hauling idle; multiplied by boat burn rate for fuel used
	{"A", "Nearshore Ledges", 0.75, 2.0, "Close in, well-picked, easy steam"},
	{"B", "Eastern Bay", 0.90, 3.0, "Mid-range, decent grounds"},
	{"C", "The Mudhole", 1.10, 4.0, "Deep soft bottom, good keepers"},
	{"D", "Green Island Shoals", 1.00, 3.5, "Classic zone, reliable but crowded"},
	{"E", "Outer Ledges", 1.30, 6.0, "Far out, big lobster if you can get there"},
	{"F", "The Rip", 1.45, 8.0, "Rough crossing, premium grounds"},
	{"G", "Deep Water Drop-off", 1.25, 10.0, "Long steam, cold water giants"},
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
	Hungover      bool `json:"hungover"`        // can't fish next day
	LastWeather string     `json:"last_weather"`
	TotalCatch  float64    `json:"total_catch_lbs"`
	TotalRevenue float64    `json:"total_revenue"`
	// Daily market prices by grade [chix, quarters, selects, jumbos, supers, culls]
	DailyPrices  [6]float64 `json:"daily_prices"`
	DieselPrice  float64    `json:"diesel_price"`  // $/gal, marine diesel
	BaitPrice    float64    `json:"bait_price"`    // $/lb, fresh herring spot price

	// Equipment
	HasRadar      bool `json:"has_radar"`       // fog: unlocks all zones
	HasGPS        bool `json:"has_gps"`         // unlocks zones F/G
	HasVHF        bool `json:"has_vhf"`         // weather forecast + distress events
	HasUpgHauler  bool `json:"has_upg_hauler"`  // slower hydraulic wear
	HasDepthSound bool `json:"has_depth_sound"` // full catch rate in deep zones (D-G)
	HasExhaustHX  bool `json:"has_exhaust_hx"`  // heat exchanger: reduces engine wear
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
}

// RollDailyPrices generates co-op dock prices for the day
func RollDailyPrices() [6]float64 {
	shift := rand.Float64()*0.60 - 0.30 // ±$0.30 market swing
	return [6]float64{
		math.Max(4.00, 5.75+shift),   // Chix
		math.Max(5.00, 6.50+shift),   // Quarters
		math.Max(6.00, 7.75+shift),   // Selects
		math.Max(8.00, 10.50+shift),  // Jumbos
		math.Max(11.00, 14.00+shift), // Super Jumbos
		math.Max(2.50, 3.75+shift),   // Culls
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

// gradeDistribution returns fractions (must sum to 1.0) for
// [chix, quarters, selects, jumbos, super jumbos, culls]
// Super Jumbos (2.5+ lb) only appear meaningfully in zones F and G
func gradeDistribution(zone Zone) [6]float64 {
	switch zone.ID {
	case "A": // nearshore, heavily fished — mostly chix, no super jumbos
		return [6]float64{0.60, 0.20, 0.10, 0.02, 0.00, 0.08}
	case "B":
		return [6]float64{0.50, 0.25, 0.15, 0.03, 0.00, 0.07}
	case "C": // mudhole — soft bottom, decent mix
		return [6]float64{0.40, 0.27, 0.20, 0.05, 0.01, 0.07}
	case "D": // classic zone, crowded
		return [6]float64{0.47, 0.26, 0.16, 0.04, 0.00, 0.07}
	case "E": // outer ledges — better mix, occasional super jumbos
		return [6]float64{0.29, 0.27, 0.25, 0.10, 0.02, 0.07}
	case "F": // the rip — prime grounds, real chance at super jumbos
		return [6]float64{0.20, 0.24, 0.28, 0.16, 0.05, 0.07}
	case "G": // deep cold water — best grade, most super jumbos
		return [6]float64{0.13, 0.20, 0.28, 0.22, 0.10, 0.07}
	default:
		return [6]float64{0.44, 0.25, 0.18, 0.05, 0.01, 0.07}
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
	gradeNames := [6]string{"Chix", "Quarters", "Selects", "Jumbos", "Supers", "Culls"}
	dist := gradeDistribution(zone)

	var grades []GradeResult
	revenue := 0.0
	for i := 0; i < 6; i++ {
		lbs := catchLbs * dist[i]
		price := basePrices[i]
		if price < 0 { price = 0 }
		revenue += lbs * price
		if lbs >= 0.5 { // only show grades with meaningful weight
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
	baitPerTrap := 2.0 + rand.Float64()*1.0 // 2.0-3.0 lbs/trap
	baitUsed := int(float64(gs.Traps)*baitPerTrap + 0.5)
	if baitUsed > gs.Bait {
		baitUsed = gs.Bait
	}

	// Component wear: diesel engines are durable; zincs corrode from seawater
	// Engine: ~0.5-1% wear per day; heat exchanger reduces wear by 60%
	engineDmg := 0.5 + rand.Float64()*0.5
	if gs.HasExhaustHX {
		engineDmg *= 0.4
	}
	// Zincs: ~1-2% per day (saltwater exposure)
	zincsDmg := 1.0 + rand.Float64()*1.0
	// Hydraulics: ~0.4-0.9% per day
	hydDmg := 0.4 + rand.Float64()*0.5

	// Hull damage from weather
	hullDmg := weather.HullDamage * boat.HullRisk * (rand.Float64() * 0.5 + 0.5)

	// Trap loss — each trap has a base chance of being lost per haul
	trapLossRate := 0.02 // 2% per trap in normal conditions
	if weather.Type == WeatherSCA {
		trapLossRate = 0.05
	}
	// Deeper zones = rockier bottom, stronger current = more line loss
	trapLossRate += zone.SteamHours * 0.003
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
	}
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

// ─── Deck Log Flavor Text ─────────────────────────────────────────────────────

type flavorEntry struct {
	condition func(gs *GameState, zone Zone, result HaulResult) bool
	lines     []string
}

var flavorPool = []flavorEntry{
	// Zone A — nearshore, crowded
	{
		func(gs *GameState, z Zone, r HaulResult) bool { return z.ID == "A" },
		[]string{
			"Counted four other boats on the same ledge. Half of 'em were ours from last week.",
			"Tourist charter nearly ran over the string. Waved. They waved back. Nobody apologized.",
			"Zone A again. Picking up after everyone else. Someday we'll have enough to go farther out.",
			"The nearshore is fished hard this time of year. You can tell by the size of what's coming up.",
		},
	},
	// Zone G — deep water, long steam
	{
		func(gs *GameState, z Zone, r HaulResult) bool { return z.ID == "G" || z.ID == "F" },
		[]string{
			"Long steam. You eat your lunch before the first buoy. That's how you know it's a real trip.",
			"Cold out there. The kind of cold that gets into your gloves no matter what.",
			"Found the string fine. Hauled clean. The steam home felt twice as long.",
			"Nobody else out this far. That's either a good sign or a very bad one.",
		},
	},
	// Low gear health — maintenance needed
	{
		func(gs *GameState, z Zone, r HaulResult) bool {
			return (gs.Engine+gs.Zincs+gs.Hydraulics)/3.0 < 40
		},
		[]string{
			"Hauler stuttered on trap 7. Held breath all the way home.",
			"Engine's been running rough all week. Can hear it in the idle.",
			"The hydraulics squealed coming off the last string. Praying it holds another day.",
			"Need to get into the wharf before something gives. Not a question of if anymore.",
		},
	},
	// Fog
	{
		func(gs *GameState, z Zone, r HaulResult) bool { return gs.Engine < 60 }, // reuse as proxy; real impl checks weather
		[]string{},
	},
	// Good catch (top third of expected)
	{
		func(gs *GameState, z Zone, r HaulResult) bool { return r.CatchLbs > float64(gs.Traps)*2.0 },
		[]string{
			"Traps were stacked. The kind of day you don't talk about at the co-op.",
			"Good haul. Didn't jinx it by saying so until we were tied up.",
			"Selects running thick today. Priced right too.",
			"Should've had more bait. Could've stayed another string.",
		},
	},
	// Slim catch
	{
		func(gs *GameState, z Zone, r HaulResult) bool { return r.CatchLbs < float64(gs.Traps)*0.8 },
		[]string{
			"Light day. The bugs know something we don't.",
			"Happens. Didn't used to bother me. Now it does a little.",
			"Some guys at the dock had good numbers. Our zone just wasn't cooperating.",
			"Bait's expensive to haul empty traps. Something to think about.",
		},
	},
	// General / always available
	{
		func(gs *GameState, z Zone, r HaulResult) bool { return true },
		[]string{
			"Weather held. That's half the job right there.",
			"Tied up before dark. A good day by most measures.",
			"The water was reading right. Whether the traps agreed is another thing.",
			"Hands are done. Boat's put away. That's the job.",
			"Diesel prices are what they are. You just burn it.",
			"Saw a minke whale on the way in. Didn't tell anyone. Some things are just yours.",
			"Radio chatter was all about the same grounds. Makes you glad you went where you went.",
			"Didn't lose a single buoy today. That's worth noting.",
		},
	},
}

// RollRandomEvent returns a random mid-haul event, or nil (85% chance of none)
func RollRandomEvent(gs *GameState, weather Weather) *RandomEvent {
	if rand.Float64() > 0.15 {
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
			Type:   EventJonahCrabs,
			Time:   "1115",
			Desc:   "1115 — Traps are packed with Jonah crabs today. Legal to keep. You could bring these in.",
			KeyA:   "k", LabelA: "[K] Keep them (extra cash)",
			KeyB:   "t", LabelB: "[T] Toss back (not worth the hassle)",
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
	e := events[rand.Intn(len(events))]
	return &e
}

// DeckLog returns a contextual flavor text entry for the end-of-day summary
func DeckLog(gs *GameState, zone Zone, result HaulResult) string {
	// Collect all matching entries
	var candidates []string
	for _, e := range flavorPool {
		if e.condition(gs, zone, result) && len(e.lines) > 0 {
			candidates = append(candidates, e.lines...)
		}
	}
	if len(candidates) == 0 {
		return "Another day on the water."
	}
	return candidates[rand.Intn(len(candidates))]
}
