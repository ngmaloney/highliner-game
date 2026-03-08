package main

import (
	"math"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestMain(m *testing.M) {
	// Force TrueColor so lipgloss emits ANSI codes in tests (non-TTY environment)
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

// TestScratchTicketOdds verifies each prize tier is within 20% of expected probability.
// Uses 500k iterations to reduce statistical noise on rare tiers.
func TestScratchTicketOdds(t *testing.T) {
	const n = 500000
	counts := map[int]int{}
	for i := 0; i < n; i++ {
		w := scratchTicket()
		counts[w]++
	}

	type tierCheck struct {
		prize    int
		expected float64 // expected probability
		tol      float64 // tolerance fraction (e.g. 0.20 = ±20%)
	}

	// Cumulative thresholds from scratchTicket():
	// r < 0.001 → 500 (prob 0.001) — rare; use wider tolerance
	// r < 0.005 → 200 (prob 0.004)
	// r < 0.022 → 100 (prob 0.017)
	// r < 0.055 → 50  (prob 0.033)
	// r < 0.122 → 40  (prob 0.067)
	// r < 0.322 → 20  (prob 0.200)
	// else      → 0   (prob 0.678)
	tiers := []tierCheck{
		{500, 0.001, 0.40}, // very rare; wider tolerance
		{200, 0.004, 0.30},
		{100, 0.017, 0.20},
		{50, 0.033, 0.20},
		{40, 0.067, 0.20},
		{20, 0.200, 0.20},
		{0, 0.678, 0.20},
	}

	for _, tc := range tiers {
		got := float64(counts[tc.prize]) / float64(n)
		diff := math.Abs(got-tc.expected) / tc.expected
		if diff > tc.tol {
			t.Errorf("prize $%d: expected ~%.4f got %.4f (%.1f%% off, >%.0f%% tolerance)",
				tc.prize, tc.expected, got, diff*100, tc.tol*100)
		}
	}
}

// TestGroundfishPrice verifies correct price per species
func TestGroundfishPrice(t *testing.T) {
	cases := []struct {
		name  string
		price float64
	}{
		{"Monkfish", 4.00},
		{"Cusk/Hake", 2.75},
		{"Black Sea Bass", 3.50},
		{"Halibut", 18.00},
		{"Unknown", 3.00},
		{"", 3.00},
	}
	for _, tc := range cases {
		got := groundfishPrice(tc.name)
		if got != tc.price {
			t.Errorf("groundfishPrice(%q) = %.2f, want %.2f", tc.name, got, tc.price)
		}
	}
}

// TestMoneyStr verifies formatting edge cases
func TestMoneyStr(t *testing.T) {
	cases := []struct {
		v    float64
		want string
	}{
		{0, "$0.00"},
		{100, "$100.00"},
		{-50, "-$50.00"},
		{1234.56, "$1234.56"},
		{-0.01, "-$0.01"},
	}
	for _, tc := range cases {
		got := moneyStr(tc.v)
		if got != tc.want {
			t.Errorf("moneyStr(%.2f) = %q, want %q", tc.v, got, tc.want)
		}
	}
}

// TestHealthStr verifies threshold coloring via ANSI RGB escape sequences.
// lipgloss renders #00FF00 as "38;2;0;255;0", #FF8C00 as "38;2;255;140;0", #FF3333 as "38;2;255;51;51".
func TestHealthStr(t *testing.T) {
	// ≥60 → green (#00FF00) → ANSI 38;2;0;255;0
	// ≥30 and <60 → orange (#FF8C00) → ANSI 38;2;255;140;0
	// <30 → red (#FF3333) → ANSI 38;2;255;51;51
	cases := []struct {
		h        float64
		ansiCode string
		desc     string
	}{
		{100, "38;2;0;255;0", "100% should be green"},
		{60, "38;2;0;255;0", "60% should be green"},
		{59, "38;2;255;140;0", "59% should be orange"},
		{30, "38;2;255;140;0", "30% should be orange"},
		{29, "38;2;255;51;51", "29% should be red"},
		{0, "38;2;255;51;51", "0% should be red"},
	}
	for _, tc := range cases {
		got := healthStr(tc.h)
		if !strings.Contains(got, tc.ansiCode) {
			t.Errorf("healthStr(%.0f): expected ANSI code %q in output %q — %s",
				tc.h, tc.ansiCode, got, tc.desc)
		}
	}
}

// TestSimulateHaul verifies basic invariants of the haul simulation
func TestSimulateHaul(t *testing.T) {
	gs := newGame()
	gs.Bait = 100
	gs.Fuel = 40

	zone := Zones[0] // Zone A — nearshore

	weather := Weather{
		Type:     WeatherClear,
		CanFish:  true,
		Modifier: 1.0,
		HullDamage: 0.5,
	}

	for i := 0; i < 50; i++ {
		result := simulateHaul(gs, zone, weather)

		if result.CatchLbs < 0 {
			t.Errorf("iteration %d: CatchLbs = %.2f, must be >= 0", i, result.CatchLbs)
		}
		if result.FuelUsed <= 0 {
			t.Errorf("iteration %d: FuelUsed = %d, must be > 0", i, result.FuelUsed)
		}

		// Grade lbs should sum to approximately CatchLbs
		if result.CatchLbs > 0 && len(result.Grades) > 0 {
			sum := 0.0
			for _, g := range result.Grades {
				sum += g.Lbs
			}
			// Allow for grades < 0.5 lbs being dropped; sum should be <= CatchLbs
			if sum > result.CatchLbs*1.01 {
				t.Errorf("iteration %d: grade sum %.2f > CatchLbs %.2f",
					i, sum, result.CatchLbs)
			}
		}
	}
}

// TestRollWeather verifies all rolled WeatherTypes are valid
func TestRollWeather(t *testing.T) {
	valid := map[WeatherType]bool{
		WeatherClear: true,
		WeatherFog:   true,
		WeatherSCA:   true,
		WeatherGale:  true,
	}
	for i := 0; i < 1000; i++ {
		w := rollWeather()
		if !valid[w.Type] {
			t.Errorf("iteration %d: invalid WeatherType %q", i, w.Type)
		}
	}
}
