# HIGHLINER — Backlog

## Done (this session)
- [x] Super Jumbos grade (2.5+ lb, $13-16/lb, zones F/G only)
- [x] Zone range limits — Eastern 22 blocked from zones F+G (MaxSteamHrs=6.5)
- [x] Daily diesel + bait price variation
- [x] Bait cap per boat (fish totes: Eastern 22=250 lbs → Wesmac=1400 lbs)
- [x] Zero bait = zero catch; partial bait scales catch proportionally
- [x] Auto-fill tank overnight on day advance
- [x] Auto-save after every meaningful action
- [x] Trap prices corrected to $150-175 (commercial wire trap)
- [x] Starting 20 traps, 100% fuel/gear, full tank
- [x] Market price display on wharf screen (daily grade prices)
- [x] Zone select colors fixed (available = bright white, blocked = dark grey)

## In Progress
- [ ] Rename save dir `.trap-sh` → `.highliner`

## Gameplay
- [ ] **Sternman / mate hire** — daily wage (~$150-200/day) or catch share (10-15%); increases traps hauled per day; crew-specific flavor text; paid on bad weather days too
- [ ] **Seasonal lobster pricing** — summer glut ($4-5/lb), fall spike ($8-12+/lb); day/date drives season
- [ ] **Bait availability** — herring supply fluctuates; some days dock is short; price spikes
- [ ] **Maine trap tag limits** — license tier caps max traps; requires license upgrade to expand
- [ ] **Co-op membership** — annual fee, unlocks better pricing vs selling to dealer
- [ ] **State Marine Patrol** — random boarding event; fail = fine or license risk
- [ ] **Holding tank / live car** — store live lobsters when dock price is low; daily mortality ~1-2%/day; requires shore infrastructure to unlock
- [ ] **By-catch events** — shorts reduce sellable weight; egg-bearers must be V-notched and released; Cod triggers Marine Patrol risk
- [ ] **Vice system** — Allen's Coffee Brandy daily buff (from PRD); morale/stamina mechanic

## Shore Infrastructure (whole missing PRD pillar)
- [ ] **Dock footage** — purchase wharf space; unlocks overnight repairs, trap storage
- [ ] **Trap storage** — prevents gear rot on off-season traps
- [ ] **Workshop** — overnight repairs without losing a fishing day
- [ ] **Bait freezer** — buy bulk herring when cheap; store for shortage periods; prerequisite for big-boat operations

## Equipment Upgrades (bolt-ons)
- [ ] **Hydraulic hauler** — real purchasable upgrade; without it hand-hauling limits traps/day
- [ ] **Radar** — unlocks fishing in fog (currently blocked weather)
- [ ] **GPS/chartplotter** — small zone multiplier bonus
- [ ] **Insulated fish hold** — reduces spoilage, marginally better grade prices
- [ ] **Bait bags** — reduces bait consumption per trap

## Boat
- [ ] **Boat upgrade flow** — wire up purchase logic (currently "not yet implemented")
- [ ] **Vessel name carries over on upgrade** — or prompt to rename

## Calendar & Seasonality (Research Required)
- [ ] **Real calendar dates** — replace Day N with actual dates; start month selection (May = season opener)
- [ ] **Seasonal catch variation** — Maine DMR monthly data; summer = volume/low grade, fall = hard shell/premium
- [ ] **Seasonal pricing curves** — ties to holding tank strategy
- [ ] **Seasonal weather patterns** — winter gales, summer fog, fall squalls; probability table by month
- [ ] **Research**: Maine DMR landings reports (monthly catch rates), NOAA Gulf of Maine buoy data, co-op price sheets

## UI / Polish
- [ ] **More flavor text** — pool of ~25 repeats fast; need 80-100 entries across zone/weather/crew/gear contexts
- [ ] **Weather forecast** — show tomorrow's forecast at end of day
- [ ] **Trip log persistence** — last 7 days of catch/revenue on dock screen
- [ ] **Keyboard shortcut help** — `?` key

## Known Bugs / Debt
- [ ] `DeckLog()` uses `gs.Engine < 60` as fog proxy — replace with actual weather type on HaulResult
- [ ] Market cursor bound hardcoded — should derive from `len(items)-1`
- [ ] Boat upgrade prices need game-balance tuning (is $45k Calvin Beal right?)
- [ ] Hard/soft shell — OUT OF SCOPE (per gnimo 2026-03-07)
